package store

import (
	"ai-travel/internal/domain"
	"context"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

type AuthLimit struct {
	Key    string
	Max    int
	Window time.Duration
}

func rateError(seconds int32) *domain.APIError {
	if seconds < 1 {
		seconds = 1
	}
	e := domain.Err(429, "AUTH_RATE_LIMITED", "操作频繁，请稍后重试")
	e.RetryAfter = &seconds
	return e
}

// Every caller takes the same sorted lock order. Rejected requests roll back
// all buckets, so a blocked account cannot drain shared IP/global quotas.
func takeAuthLimits(ctx context.Context, tx pgx.Tx, limits []AuthLimit) error {
	limits = append([]AuthLimit(nil), limits...)
	sort.Slice(limits, func(i, j int) bool { return limits[i].Key < limits[j].Key })
	for _, l := range limits {
		if l.Max < 1 || l.Window < time.Second {
			return domain.Err(503, "AUTH_UNAVAILABLE", "认证配置不可用")
		}
		var attempts int
		var remaining int32
		e := tx.QueryRow(ctx, `INSERT INTO auth_rate_limits(key,attempts,resets_at) VALUES($1,1,clock_timestamp()+make_interval(secs=>$2))
   ON CONFLICT(key) DO UPDATE SET attempts=CASE WHEN auth_rate_limits.resets_at<=clock_timestamp() THEN 1 ELSE LEAST(auth_rate_limits.attempts+1,$3+1) END,
   resets_at=CASE WHEN auth_rate_limits.resets_at<=clock_timestamp() THEN clock_timestamp()+make_interval(secs=>$2) ELSE auth_rate_limits.resets_at END
   RETURNING attempts,GREATEST(1,ceil(extract(epoch FROM resets_at-clock_timestamp())))::integer`, l.Key, l.Window.Seconds(), l.Max).Scan(&attempts, &remaining)
		if e != nil {
			return dbError(e)
		}
		if attempts > l.Max {
			return rateError(remaining)
		}
	}
	return nil
}
func (s *Store) TakeAuthLimits(ctx context.Context, limits []AuthLimit) error {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return dbError(e)
	}
	defer tx.Rollback(ctx)
	result := takeAuthLimits(ctx, tx, limits)
	if result != nil {
		return result
	}
	if e = tx.Commit(ctx); e != nil {
		return dbError(e)
	}
	return result
}
func lockEmail(ctx context.Context, tx pgx.Tx, email string) error {
	_, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,40719))", email)
	return dbError(e)
}
