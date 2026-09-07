package store

import (
	"ai-travel/internal/domain"
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required for real PostgreSQL tests")
	}
	s, e := Open(context.Background(), url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	if e = s.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestSessionsRevokedAndPrivateJobs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := domain.User{ID: domain.ID(), Username: domain.ID(), Role: "user"}
	if e := s.CreateUser(ctx, u, "hash"); e != nil {
		t.Fatal(e)
	}
	if e := s.CreateSession(ctx, "session-"+u.ID, u.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Auth(ctx, "session-"+u.ID); e != nil {
		t.Fatal(e)
	}
	s.Revoke(ctx, "session-"+u.ID)
	if _, e := s.Auth(ctx, "session-"+u.ID); e == nil {
		t.Fatal("revoked session accepted")
	}
	j, e := s.CreateJob(ctx, u.ID, "generate", "new", "key", "hash", domain.JobInput{}, nil, "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.GetJob(ctx, "another-user", j.ID); e == nil {
		t.Fatal("private job leaked")
	}
}
func TestConcurrentIdempotency(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := domain.User{ID: domain.ID(), Username: domain.ID(), Role: "user"}
	s.CreateUser(ctx, u, "hash")
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := s.CreateJob(ctx, u.ID, "generate", "new", "same", "hash", domain.JobInput{}, nil, "")
			if e != nil {
				errs <- e
			} else {
				ids <- j.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("duplicate jobs")
		}
	}
	if _, e := s.CreateJob(ctx, u.ID, "generate", "new", "same", "different", domain.JobInput{}, nil, ""); e == nil {
		t.Fatal("payload conflict accepted")
	}
}

func runningJob(t *testing.T, s *Store, u domain.User, kind string, base *domain.Trip) Record {
	t.Helper()
	ctx := context.Background()
	j, e := s.CreateJob(ctx, u.ID, kind, domain.ID(), domain.ID(), "hash", domain.JobInput{}, base, "")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Pool.Exec(ctx, "UPDATE planning_jobs SET status='running',attempt_id='test-attempt',lease_expires_at=clock_timestamp()+interval '30 seconds',deadline_at=clock_timestamp()+interval '60 seconds' WHERE id=$1", j.ID)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Record(ctx, j.ID)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestLateSaveCannotCommitAfterLeaseExpires(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := domain.User{ID: domain.ID(), Username: domain.ID(), Role: "user"}
	s.CreateUser(ctx, u, "hash")
	r := runningJob(t, s, u, "generate", nil)
	s.Pool.Exec(ctx, "UPDATE planning_jobs SET lease_expires_at=clock_timestamp()+interval '200 milliseconds' WHERE id=$1", r.Job.ID)
	lock, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Rollback(ctx)
	if _, e = lock.Exec(ctx, "SELECT id FROM planning_jobs WHERE id=$1 FOR UPDATE", r.Job.ID); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- s.Save(ctx, r, domain.Plan{Activities: []domain.Activity{}, Routes: []domain.Route{}}) }()
	time.Sleep(350 * time.Millisecond)
	lock.Commit(ctx)
	if e := <-done; e == nil {
		t.Fatal("expired lease committed a new trip")
	}
	j, e := s.GetJob(ctx, u.ID, r.Job.ID)
	if e != nil {
		t.Fatal(e)
	}
	if j.Status == "succeeded" {
		t.Fatal("expired task succeeded")
	}
}
func TestConcurrentVersionsAndAtomicRollback(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := domain.User{ID: domain.ID(), Username: domain.ID(), Role: "user"}
	s.CreateUser(ctx, u, "hash")
	r := runningJob(t, s, u, "generate", nil)
	if e := s.Save(ctx, r, domain.Plan{Title: "original"}); e != nil {
		t.Fatal(e)
	}
	j, _ := s.GetJob(ctx, u.ID, r.Job.ID)
	base, e := s.GetTrip(ctx, u.ID, j.TripID, 0)
	if e != nil {
		t.Fatal(e)
	}
	edit := runningJob(t, s, u, "manual_edit", &base)
	ai := runningJob(t, s, u, "replan", &base)
	if e = s.Save(ctx, edit, domain.Plan{Title: "edited"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Save(ctx, ai, domain.Plan{Title: "stale"}); e != nil {
		t.Fatal(e)
	}
	aijob, _ := s.GetJob(ctx, u.ID, ai.Job.ID)
	current, _ := s.GetTrip(ctx, u.ID, j.TripID, 0)
	if aijob.Status != "conflicted" || current.Version != 2 || current.Plan.Title != "edited" {
		t.Fatal(aijob, current)
	}
	bad := runningJob(t, s, u, "generate", nil)
	if e = s.Save(ctx, bad, domain.Plan{Routes: []domain.Route{{FromItemID: "not-an-item", ToItemID: "missing"}}}); e == nil {
		t.Fatal("invalid FK accepted")
	}
	var n int
	s.Pool.QueryRow(ctx, "SELECT count(*) FROM trip_versions WHERE source_job_id=$1", bad.Job.ID).Scan(&n)
	if n != 0 {
		t.Fatal("partial version committed")
	}
}

func TestDatabaseOutageIsNotAuthenticationFailure(t *testing.T) {
	s := testStore(t)
	s.Close()
	_, e := s.Auth(context.Background(), "token-hash")
	a, ok := e.(*domain.APIError)
	if !ok || a.Status != 503 {
		t.Fatalf("DB outage reported as auth failure: %v", e)
	}
	_, _, e = s.Credentials(context.Background(), "existing-user")
	a, ok = e.(*domain.APIError)
	if !ok || a.Status != 503 {
		t.Fatalf("DB outage reported as bad password: %v", e)
	}
}

func TestAppliedMigrationDoesNotRelockLiveUserTable(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "LOCK TABLE users IN ACCESS SHARE MODE"); e != nil {
		t.Fatal(e)
	}
	deadline, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if e = s.Migrate(deadline); e != nil {
		t.Fatalf("already applied migration tried to alter a live table: %v", e)
	}
}

func TestOldSchemaMigrationPreservesUsersAndTripOwnership(t *testing.T) {
	original := testStore(t)
	ctx := context.Background()
	schema := "migration_" + strings.ReplaceAll(domain.ID(), "-", "")
	if _, e := original.Pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	cfg, e := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	s := &Store{Pool: pool}
	_, e = pool.Exec(ctx, `CREATE TABLE users(id text PRIMARY KEY,username text UNIQUE NOT NULL,password_hash text NOT NULL,role text NOT NULL CHECK(role IN ('user','admin')),active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE trips(id text PRIMARY KEY,owner_id text NOT NULL REFERENCES users(id),current_version bigint NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now());
 INSERT INTO users(id,username,password_hash,role) VALUES('old-user','old-admin','old-password-hash','admin');
 INSERT INTO trips(id,owner_id,current_version) VALUES('old-trip','old-user',3);`)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	var name, hash, role, owner string
	var missingEmail bool
	var version int
	if e = pool.QueryRow(ctx, "SELECT username,password_hash,role,email_key IS NULL FROM users WHERE id='old-user'").Scan(&name, &hash, &role, &missingEmail); e != nil {
		t.Fatal(e)
	}
	if name != "old-admin" || hash != "old-password-hash" || role != "admin" || !missingEmail {
		t.Fatal("legacy identity changed")
	}
	if e = pool.QueryRow(ctx, "SELECT owner_id,current_version FROM trips WHERE id='old-trip'").Scan(&owner, &version); e != nil {
		t.Fatal(e)
	}
	if owner != "old-user" || version != 3 {
		t.Fatal("legacy trip changed")
	}
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
}
