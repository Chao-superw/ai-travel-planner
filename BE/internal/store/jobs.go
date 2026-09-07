package store

import (
	"ai-travel/internal/config"
	"ai-travel/internal/domain"
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"time"
)

const jobJSON = `public || jsonb_build_object('status',status,'stage',stage,'updated_at',updated_at,'trip_id',trip_id,'result_version',result_version,'error',error,'model_runs',model_runs)`

type Record struct {
	Job        domain.Job
	OwnerID    string
	Input      domain.JobInput
	Base       *domain.Trip
	AttemptID  string
	Deadline   time.Time
	Candidates []domain.Place
}

func (s *Store) ExistingJob(ctx context.Context, owner, op, key, hash string) (*domain.Job, error) {
	var b []byte
	var oldHash string
	e := s.Pool.QueryRow(ctx, "SELECT "+jobJSON+",request_hash FROM planning_jobs WHERE owner_id=$1 AND operation=$2 AND idem_key=$3", owner, op, key).Scan(&b, &oldHash)
	if e == pgx.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, dbError(e)
	}
	if hash != oldHash {
		return nil, domain.Err(409, "IDEMPOTENCY_CONFLICT", "同一幂等键对应不同请求")
	}
	j, e := decode[domain.Job](b)
	return &j, dbError(e)
}
func (s *Store) CreateJob(ctx context.Context, owner, kind, op, key, hash string, input domain.JobInput, base *domain.Trip, parent string) (domain.Job, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return domain.Job{}, dbError(e)
	}
	defer tx.Rollback(ctx)
	var uid string
	if e = tx.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", owner).Scan(&uid); e != nil {
		return domain.Job{}, dbError(e)
	}
	var raw []byte
	var old string
	e = tx.QueryRow(ctx, "SELECT "+jobJSON+",request_hash FROM planning_jobs WHERE owner_id=$1 AND operation=$2 AND idem_key=$3", owner, op, key).Scan(&raw, &old)
	if e == nil {
		if old != hash {
			return domain.Job{}, domain.Err(409, "IDEMPOTENCY_CONFLICT", "同一幂等键对应不同请求")
		}
		j, e := decode[domain.Job](raw)
		return j, dbError(e)
	}
	if e != pgx.ErrNoRows {
		return domain.Job{}, dbError(e)
	}
	var count int
	e = tx.QueryRow(ctx, "SELECT count(*) FROM planning_jobs WHERE owner_id=$1 AND status IN ('queued','running') AND ((kind='manual_edit')=($2='manual_edit'))", owner, kind).Scan(&count)
	if e != nil {
		return domain.Job{}, dbError(e)
	}
	if count > 0 {
		return domain.Job{}, domain.Err(429, "ACTIVE_JOB_LIMIT", "已有同类任务正在等待或执行")
	}
	j := domain.Job{ID: domain.ID(), Kind: kind, Status: "queued", Stage: "queued", ParentJobID: parent, CreatedAt: time.Now().UTC().Format(time.RFC3339), ModelRuns: []domain.ModelRun{}}
	j.UpdatedAt = j.CreatedAt
	if base != nil {
		j.TripID = base.ID
	}
	_, e = tx.Exec(ctx, "INSERT INTO planning_jobs(id,owner_id,kind,operation,idem_key,request_hash,input,base_snapshot,public,trip_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", j.ID, owner, kind, op, key, hash, bytes(input), bytes(base), bytes(j), j.TripID)
	if e != nil {
		return j, dbError(e)
	}
	return j, dbError(tx.Commit(ctx))
}
func (s *Store) GetJob(ctx context.Context, owner, id string) (domain.Job, error) {
	var b []byte
	e := s.Pool.QueryRow(ctx, "SELECT "+jobJSON+" FROM planning_jobs WHERE owner_id=$1 AND id=$2", owner, id).Scan(&b)
	if e != nil {
		return domain.Job{}, dbError(e)
	}
	j, e := decode[domain.Job](b)
	return j, dbError(e)
}
func (s *Store) Record(ctx context.Context, id string) (Record, error) {
	var r Record
	var a, b, c, d []byte
	e := s.Pool.QueryRow(ctx, "SELECT "+jobJSON+",owner_id,input,base_snapshot,attempt_id,COALESCE(deadline_at,now()),candidates FROM planning_jobs WHERE id=$1", id).Scan(&a, &r.OwnerID, &b, &c, &r.AttemptID, &r.Deadline, &d)
	if e != nil {
		return r, dbError(e)
	}
	for _, v := range []struct {
		b   []byte
		out any
	}{{a, &r.Job}, {b, &r.Input}, {c, &r.Base}, {d, &r.Candidates}} {
		if len(v.b) > 0 {
			if e = json.Unmarshal(v.b, v.out); e != nil {
				return r, dbError(e)
			}
		}
	}
	return r, nil
}
func (s *Store) Claim(ctx context.Context) (*Record, error) {
	var id string
	e := s.Pool.QueryRow(ctx, `WITH next AS (SELECT id FROM planning_jobs WHERE status='queued' AND created_at>now()-interval '5 minutes' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE planning_jobs j SET status='running',stage='loading',attempt_id=$1,lease_expires_at=now()+interval '30 seconds',deadline_at=now()+make_interval(secs => $2::double precision),updated_at=now() FROM next WHERE j.id=next.id RETURNING j.id`, domain.ID(), config.JobExecutionTimeout.Seconds()).Scan(&id)
	if e == pgx.ErrNoRows {
		return nil, nil
	}
	if e != nil {
		return nil, dbError(e)
	}
	r, e := s.Record(ctx, id)
	return &r, e
}
func (s *Store) Renew(ctx context.Context, r Record) error {
	tag, e := s.Pool.Exec(ctx, "UPDATE planning_jobs SET lease_expires_at=LEAST(now()+interval '30 seconds',deadline_at),updated_at=now() WHERE id=$1 AND attempt_id=$2 AND status='running' AND lease_expires_at>now() AND deadline_at>now()", r.Job.ID, r.AttemptID)
	if e != nil {
		return dbError(e)
	}
	if tag.RowsAffected() != 1 {
		return domain.Err(409, "LEASE_LOST", "任务租约已失效")
	}
	return nil
}
func (s *Store) Stage(ctx context.Context, r Record, stage string, places []domain.Place, runs []domain.ModelRun) error {
	tag, e := s.Pool.Exec(ctx, "UPDATE planning_jobs SET stage=$3,candidates=$4,model_runs=$5,updated_at=now() WHERE id=$1 AND attempt_id=$2 AND status='running' AND lease_expires_at>now() AND deadline_at>now()", r.Job.ID, r.AttemptID, stage, bytes(places), bytes(runs))
	if e != nil {
		return dbError(e)
	}
	if tag.RowsAffected() != 1 {
		return domain.Err(409, "LEASE_LOST", "任务租约已失效")
	}
	return nil
}
func (s *Store) Fail(ctx context.Context, r Record, status string, err *domain.APIError) error {
	_, e := s.Pool.Exec(ctx, "UPDATE planning_jobs SET status=$3,stage=$3,error=$4,updated_at=now() WHERE id=$1 AND attempt_id=$2 AND status='running' AND lease_expires_at>now() AND deadline_at>now()", r.Job.ID, r.AttemptID, status, bytes(err))
	return dbError(e)
}
func (s *Store) Sweep(ctx context.Context) error {
	_, e := s.Pool.Exec(ctx, `UPDATE planning_jobs SET status=CASE WHEN status='queued' THEN 'failed' ELSE 'interrupted' END, stage='expired',error='{"code":"JOB_EXPIRED","message":"任务等待超时或执行中断，请显式重试","status":504}',updated_at=now() WHERE (status='queued' AND created_at<=now()-interval '5 minutes') OR (status='running' AND (lease_expires_at<=now() OR deadline_at<=now()))`)
	return dbError(e)
}
