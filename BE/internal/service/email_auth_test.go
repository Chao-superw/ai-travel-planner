package service

import (
	"ai-travel/internal/authn"
	"ai-travel/internal/domain"
	"ai-travel/internal/store"
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func authStore(t *testing.T) *store.Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	s, err := store.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err = s.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRegistrationRequiresVerifiedEmailProof(t *testing.T) {
	s := authStore(t)
	res := (&Travel{Store: s}).Handle(context.Background(), "Register", domain.Request{Username: "old_" + domain.ID()[:12], Password: "a-long-password-123"})
	if res.Error == nil || res.Error.Code != "INVALID_INPUT" {
		t.Fatal("username-only registration must not create an unverified account")
	}
}

// SMTP is the only external boundary replaced here; all authentication and
// persistence run against the real service and PostgreSQL.
type capturedMail struct{ email, code, purpose string }
type authMailbox struct {
	mu       sync.Mutex
	messages []capturedMail
	fail     bool
}

func (m *authMailbox) SendCode(_ context.Context, email, code, purpose string, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("private smtp provider detail")
	}
	m.messages = append(m.messages, capturedMail{email, code, purpose})
	return nil
}
func (m *authMailbox) last() capturedMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages[len(m.messages)-1]
}
func authTravel(t *testing.T) (*Travel, *authMailbox) {
	s := authStore(t)
	m := &authMailbox{}
	p := authn.DefaultPolicy([]byte(domain.ID() + domain.ID()))
	return &Travel{Store: s, Mailer: m, AuthPolicy: p}, m
}
func authCall(t *testing.T, tr *Travel, method string, r domain.Request) domain.Response {
	t.Helper()
	if r.ClientIP == "" {
		r.ClientIP = "127.0.0.1"
	}
	return tr.Handle(context.Background(), method, r)
}
func expectAuthCode(t *testing.T, r domain.Response, code string) {
	t.Helper()
	if code == "" {
		if r.Error != nil {
			t.Fatalf("unexpected error: %s", r.Error.Code)
		}
		return
	}
	if r.Error == nil || r.Error.Code != code {
		t.Fatalf("wanted %s, got %+v", code, r.Error)
	}
}
func issueRegistration(t *testing.T, tr *Travel, m *authMailbox, email string) domain.Request {
	t.Helper()
	r := authCall(t, tr, "RequestRegistrationCode", domain.Request{Email: email})
	expectAuthCode(t, r, "")
	if r.Challenge == nil {
		t.Fatal("missing challenge")
	}
	return domain.Request{Email: email, Password: "a-long-password-123", Code: m.last().code, ChallengeID: r.Challenge.ChallengeID}
}
func TestEmailRegistrationLoginAndProofReplay(t *testing.T) {
	tr, m := authTravel(t)
	email := "user_" + domain.ID() + "@example.com"
	proof := issueRegistration(t, tr, m, email)
	var before int
	tr.Store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE email_key=$1", email).Scan(&before)
	if before != 0 {
		t.Fatal("sending mail created a user")
	}
	registered := authCall(t, tr, "Register", proof)
	expectAuthCode(t, registered, "")
	if registered.User == nil || registered.User.Email != email || !registered.User.EmailVerified || registered.User.Role != "user" {
		t.Fatal("incorrect verified user")
	}
	expectAuthCode(t, authCall(t, tr, "Register", proof), "INVALID_VERIFICATION_CODE")
	login := authCall(t, tr, "Login", domain.Request{Email: email, Password: proof.Password})
	expectAuthCode(t, login, "")
	if login.Session == nil || login.Session.User.ID != registered.User.ID {
		t.Fatal("session identity changed")
	}
	me := authCall(t, tr, "GetCurrentUser", domain.Request{Token: login.Session.Token})
	expectAuthCode(t, me, "")
	expectAuthCode(t, authCall(t, tr, "Login", domain.Request{Email: email, Password: "incorrect-password"}), "INVALID_CREDENTIALS")
	expectAuthCode(t, authCall(t, tr, "Logout", domain.Request{Token: login.Session.Token}), "")
	expectAuthCode(t, authCall(t, tr, "GetCurrentUser", domain.Request{Token: login.Session.Token}), "UNAUTHENTICATED")
}
func TestWrongCodeAttemptsPersistAndCannotBeResetByReopen(t *testing.T) {
	tr, m := authTravel(t)
	email := "attempt_" + domain.ID() + "@example.com"
	proof := issueRegistration(t, tr, m, email)
	wrong := proof
	wrong.Code = "000000"
	if proof.Code == wrong.Code {
		wrong.Code = "111111"
	}
	for i := 0; i < 5; i++ {
		expectAuthCode(t, authCall(t, tr, "Register", wrong), "INVALID_VERIFICATION_CODE")
	}
	restarted := &Travel{Store: authStore(t), Mailer: m, AuthPolicy: tr.AuthPolicy}
	expectAuthCode(t, authCall(t, restarted, "Register", proof), "INVALID_VERIFICATION_CODE")
	var attempts int
	tr.Store.Pool.QueryRow(context.Background(), "SELECT attempts FROM auth_challenges WHERE id=$1", proof.ChallengeID).Scan(&attempts)
	if attempts != 5 {
		t.Fatalf("failed attempts rolled back: %d", attempts)
	}
}
func TestResendSupersedesOldCodeAndFailedSendKeepsExistingCode(t *testing.T) {
	tr, m := authTravel(t)
	email := "resend_" + domain.ID() + "@example.com"
	old := issueRegistration(t, tr, m, email)
	expectAuthCode(t, authCall(t, tr, "RequestRegistrationCode", domain.Request{Email: email}), "AUTH_RATE_LIMITED")
	age := func() {
		_, err := tr.Store.Pool.Exec(context.Background(), "UPDATE auth_challenges SET created_at=created_at-interval '61 seconds' WHERE email_key=$1", email)
		if err != nil {
			t.Fatal(err)
		}
	}
	age()
	fresh := issueRegistration(t, tr, m, email)
	expectAuthCode(t, authCall(t, tr, "Register", old), "INVALID_VERIFICATION_CODE")
	age()
	m.fail = true
	r := authCall(t, tr, "RequestRegistrationCode", domain.Request{Email: email})
	expectAuthCode(t, r, "MAIL_UNAVAILABLE")
	if strings.Contains(r.Error.Message, "private") {
		t.Fatal("provider details leaked")
	}
	expectAuthCode(t, authCall(t, tr, "Register", fresh), "")
}
func TestExpiredOrWrongEmailProofCannotRegister(t *testing.T) {
	tr, m := authTravel(t)
	email := "expired_" + domain.ID() + "@example.com"
	proof := issueRegistration(t, tr, m, email)
	other := proof
	other.Email = "other_" + email
	expectAuthCode(t, authCall(t, tr, "Register", other), "INVALID_VERIFICATION_CODE")
	_, err := tr.Store.Pool.Exec(context.Background(), "UPDATE auth_challenges SET expires_at=now()-interval '1 second' WHERE id=$1", proof.ChallengeID)
	if err != nil {
		t.Fatal(err)
	}
	expectAuthCode(t, authCall(t, tr, "Register", proof), "INVALID_VERIFICATION_CODE")
}
func TestConcurrentRegistrationCreatesExactlyOneUser(t *testing.T) {
	tr, m := authTravel(t)
	email := "concurrent_" + domain.ID() + "@example.com"
	proof := issueRegistration(t, tr, m, email)
	var wg sync.WaitGroup
	success := make(chan bool, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r := authCall(t, tr, "Register", proof); success <- r.Error == nil }()
	}
	wg.Wait()
	close(success)
	count := 0
	for ok := range success {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("successful registrations=%d", count)
	}
	var rows int
	tr.Store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE email_key=$1", email).Scan(&rows)
	if rows != 1 {
		t.Fatalf("duplicate accounts=%d", rows)
	}
}
func TestExistingEmailProofDoesNotReplacePassword(t *testing.T) {
	tr, m := authTravel(t)
	email := "existing_" + domain.ID() + "@example.com"
	proof := issueRegistration(t, tr, m, email)
	expectAuthCode(t, authCall(t, tr, "Register", proof), "")
	tr.Store.Pool.Exec(context.Background(), "UPDATE auth_challenges SET created_at=created_at-interval '61 seconds' WHERE email_key=$1", email)
	again := issueRegistration(t, tr, m, email)
	again.Password = "attacker-password-123"
	expectAuthCode(t, authCall(t, tr, "Register", again), "EMAIL_ALREADY_REGISTERED")
	expectAuthCode(t, authCall(t, tr, "Login", domain.Request{Email: email, Password: proof.Password}), "")
	expectAuthCode(t, authCall(t, tr, "Login", domain.Request{Email: email, Password: again.Password}), "INVALID_CREDENTIALS")
}
func TestLegacyBindingPreservesIdentityRoleAndOwnership(t *testing.T) {
	tr, m := authTravel(t)
	ctx := context.Background()
	id := domain.ID()
	name := "legacy_" + id
	pw := "legacy-password-123"
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	err := tr.Store.CreateUser(ctx, domain.User{ID: id, Username: name, Role: "admin"}, string(h))
	if err != nil {
		t.Fatal(err)
	}
	job, err := tr.Store.CreateJob(ctx, id, "generate", "legacy", domain.ID(), "hash", domain.JobInput{}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	login := authCall(t, tr, "LegacyLogin", domain.Request{Username: name, Password: pw})
	expectAuthCode(t, login, "")
	token := login.Session.Token
	expectAuthCode(t, authCall(t, tr, "ListTrips", domain.Request{Token: token}), "EMAIL_VERIFICATION_REQUIRED")
	email := "bound_" + id + "@example.com"
	issued := authCall(t, tr, "RequestEmailBindingCode", domain.Request{Token: token, Email: email})
	expectAuthCode(t, issued, "")
	proof := domain.Request{Token: token, Email: email, Code: m.last().code, ChallengeID: issued.Challenge.ChallengeID, Password: pw}
	expectAuthCode(t, authCall(t, tr, "Register", proof), "INVALID_VERIFICATION_CODE")
	bound := authCall(t, tr, "BindEmail", proof)
	expectAuthCode(t, bound, "")
	if bound.User.ID != id || bound.User.Role != "admin" {
		t.Fatal("binding replaced legacy user")
	}
	got := authCall(t, tr, "GetPlanningJob", domain.Request{Token: token, ID: job.ID})
	expectAuthCode(t, got, "")
	expectAuthCode(t, authCall(t, tr, "LegacyLogin", domain.Request{Username: name, Password: pw}), "INVALID_CREDENTIALS")
	expectAuthCode(t, authCall(t, tr, "Login", domain.Request{Email: email, Password: pw}), "")
}
func TestLoginAndSendLimitsArePersistentAcrossServiceInstances(t *testing.T) {
	tr, m := authTravel(t)
	tr.AuthPolicy.LoginAccountQuarter = 2
	tr.AuthPolicy.SendEmailHour = 1
	email := "limited_" + domain.ID() + "@example.com"
	proof := issueRegistration(t, tr, m, email)
	expectAuthCode(t, authCall(t, tr, "Register", proof), "")
	for i := 0; i < 2; i++ {
		expectAuthCode(t, authCall(t, tr, "Login", domain.Request{Email: email, Password: "bad-password"}), "INVALID_CREDENTIALS")
	}
	reopened := &Travel{Store: authStore(t), Mailer: m, AuthPolicy: tr.AuthPolicy}
	expectAuthCode(t, authCall(t, reopened, "Login", domain.Request{Email: email, Password: proof.Password}), "AUTH_RATE_LIMITED")
	tr.Store.Pool.Exec(context.Background(), "UPDATE auth_challenges SET created_at=created_at-interval '61 seconds' WHERE email_key=$1", email)
	expectAuthCode(t, authCall(t, reopened, "RequestRegistrationCode", domain.Request{Email: email}), "AUTH_RATE_LIMITED")
}

func TestBlockedAccountCannotDrainOtherAccountsGlobalLoginBudget(t *testing.T) {
	tr, _ := authTravel(t)
	tr.AuthPolicy.LoginAccountQuarter = 1
	tr.AuthPolicy.LoginGlobalQuarter = 2
	tr.AuthPolicy.LoginIPQuarter = 2
	first := domain.Request{Email: "first@example.test", Password: "incorrect-password"}
	expectAuthCode(t, authCall(t, tr, "Login", first), "INVALID_CREDENTIALS")
	for i := 0; i < 4; i++ {
		expectAuthCode(t, authCall(t, tr, "Login", first), "AUTH_RATE_LIMITED")
	}
	expectAuthCode(t, authCall(t, tr, "Login", domain.Request{Email: "other@example.test", Password: "incorrect-password"}), "INVALID_CREDENTIALS")
}

func TestBlockedSenderCannotDrainGlobalSendBudget(t *testing.T) {
	tr, m := authTravel(t)
	tr.AuthPolicy.SendEmailHour = 1
	tr.AuthPolicy.SendGlobalHour = 2
	tr.AuthPolicy.SendIPHour = 2
	email := "sender_" + domain.ID() + "@example.test"
	issueRegistration(t, tr, m, email)
	tr.Store.Pool.Exec(context.Background(), "UPDATE auth_challenges SET created_at=created_at-interval '61 seconds' WHERE email_key=$1", email)
	for i := 0; i < 3; i++ {
		expectAuthCode(t, authCall(t, tr, "RequestRegistrationCode", domain.Request{Email: email}), "AUTH_RATE_LIMITED")
	}
	proof := issueRegistration(t, tr, m, "different_"+domain.ID()+"@example.test")
	expectAuthCode(t, authCall(t, tr, "Register", proof), "")
}
func TestBindingProofCannotBeUsedByAnotherAccountOrBindTwoEmails(t *testing.T) {
	tr, m := authTravel(t)
	ctx := context.Background()
	create := func() string {
		id := domain.ID()
		name := "old_" + id
		h, _ := bcrypt.GenerateFromPassword([]byte("legacy-password-123"), bcrypt.DefaultCost)
		if e := tr.Store.CreateUser(ctx, domain.User{ID: id, Username: name, Role: "user"}, string(h)); e != nil {
			t.Fatal(e)
		}
		out := authCall(t, tr, "LegacyLogin", domain.Request{Username: name, Password: "legacy-password-123"})
		expectAuthCode(t, out, "")
		return out.Session.Token
	}
	a, b := create(), create()
	issue := func(email string) domain.Request {
		out := authCall(t, tr, "RequestEmailBindingCode", domain.Request{Token: a, Email: email})
		expectAuthCode(t, out, "")
		return domain.Request{Token: a, Email: email, Code: m.last().code, ChallengeID: out.Challenge.ChallengeID}
	}
	first := issue("first_" + domain.ID() + "@example.test")
	second := issue("second_" + domain.ID() + "@example.test")
	stolen := first
	stolen.Token = b
	expectAuthCode(t, authCall(t, tr, "BindEmail", stolen), "INVALID_VERIFICATION_CODE")
	results := make(chan domain.Response, 2)
	var wg sync.WaitGroup
	for _, proof := range []domain.Request{first, second} {
		wg.Add(1)
		go func(r domain.Request) { defer wg.Done(); results <- authCall(t, tr, "BindEmail", r) }(proof)
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for r := range results {
		if r.Error == nil {
			success++
		} else if r.Error.Code == "EMAIL_ALREADY_BOUND" {
			conflict++
		} else {
			t.Fatalf("unexpected binding error %s", r.Error.Code)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("bindings success=%d conflicts=%d", success, conflict)
	}
	me := authCall(t, tr, "GetCurrentUser", domain.Request{Token: b})
	expectAuthCode(t, me, "")
	if me.User.EmailVerified {
		t.Fatal("other account was bound")
	}
}
func TestUserWriteFailureDoesNotConsumeValidProof(t *testing.T) {
	tr, m := authTravel(t)
	ctx := context.Background()
	id := domain.ID()
	name := "collision_" + id
	if e := tr.Store.CreateUser(ctx, domain.User{ID: id, Username: name, Role: "user"}, "hash"); e != nil {
		t.Fatal(e)
	}
	email := "rollback_" + id + "@example.test"
	proof := issueRegistration(t, tr, m, email)
	p := store.EmailProof{ID: proof.ChallengeID, Email: email, Purpose: "register", MAC: authn.CodeMAC(tr.AuthPolicy.HMACKey, proof.ChallengeID, email, "register", "", proof.Code)}
	_, e := tr.Store.CompleteEmailProof(ctx, p, domain.User{ID: domain.ID(), Username: name}, "hash")
	if e == nil {
		t.Fatal("duplicate username write unexpectedly succeeded")
	}
	expectAuthCode(t, authCall(t, tr, "Register", proof), "")
}
