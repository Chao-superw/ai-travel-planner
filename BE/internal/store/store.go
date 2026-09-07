package store

import (
	"ai-travel/internal/domain"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, domain.Err(503, "DATABASE_UNAVAILABLE", "数据库连接配置无效")
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, domain.Err(503, "DATABASE_UNAVAILABLE", "数据库连接失败")
	}
	return &Store{p}, nil
}
func (s *Store) Close() { s.Pool.Close() }
func (s *Store) Migrate(ctx context.Context) error {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return dbError(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(704912)"); e != nil {
		return dbError(e)
	}
	// Record applied versions so restart never repeats DDL against live tables.
	if _, e = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); e != nil {
		return dbError(e)
	}
	const version = "20260907-email-auth-v1"
	var applied bool
	if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&applied); e != nil {
		return dbError(e)
	}
	if applied {
		return dbError(tx.Commit(ctx))
	}
	if _, e = tx.Exec(ctx, schema); e != nil {
		return dbError(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", version); e != nil {
		return dbError(e)
	}
	return dbError(tx.Commit(ctx))
}
func dbError(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return domain.Err(504, "TIMEOUT", "数据库操作超时")
	}
	var a *domain.APIError
	if errors.As(e, &a) {
		return a
	}
	var p *pgconn.PgError
	if errors.As(e, &p) && p.Code == "23505" {
		return domain.Err(409, "CONFLICT", "记录已存在或并发冲突")
	}
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Err(404, "NOT_FOUND", "资源不存在")
	}
	return domain.Err(503, "DATABASE_UNAVAILABLE", "数据库操作失败")
}
func bytes(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
func decode[T any](b []byte) (T, error) { var v T; e := json.Unmarshal(b, &v); return v, e }
