package store

import (
	"ai-travel/internal/domain"
	"context"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateUser(ctx context.Context, u domain.User, hash string) error {
	_, e := s.Pool.Exec(ctx, "INSERT INTO users(id,username,password_hash,role) VALUES($1,$2,$3,$4)", u.ID, u.Username, hash, u.Role)
	return dbError(e)
}
func (s *Store) Credentials(ctx context.Context, name string) (domain.User, string, error) {
	var u domain.User
	var h string
	e := s.Pool.QueryRow(ctx, "SELECT id,username,role,password_hash FROM users WHERE username=$1 AND active", name).Scan(&u.ID, &u.Username, &u.Role, &h)
	if e == pgx.ErrNoRows {
		return u, "", domain.Err(401, "INVALID_CREDENTIALS", "用户名或密码错误")
	}
	if e != nil {
		return u, "", dbError(e)
	}
	return u, h, nil
}
func (s *Store) CreateSession(ctx context.Context, hash, uid string) error {
	_, e := s.Pool.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '8 hours')", hash, uid)
	return dbError(e)
}
func (s *Store) Auth(ctx context.Context, hash string) (domain.User, error) {
	var u domain.User
	e := s.Pool.QueryRow(ctx, "SELECT u.id,u.username,u.role,COALESCE(u.email,''),u.email_verified_at IS NOT NULL FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.active", hash).Scan(&u.ID, &u.Username, &u.Role, &u.Email, &u.EmailVerified)
	if e == pgx.ErrNoRows {
		return u, domain.Err(401, "UNAUTHENTICATED", "请重新登录")
	}
	if e != nil {
		return u, dbError(e)
	}
	return u, nil
}
func (s *Store) Revoke(ctx context.Context, hash string) error {
	_, e := s.Pool.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE token_hash=$1", hash)
	return dbError(e)
}
