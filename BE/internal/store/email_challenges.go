package store

import (
	"ai-travel/internal/authn"
	"ai-travel/internal/domain"
	"context"
	"crypto/hmac"
	"github.com/jackc/pgx/v5"
	"time"
)

type AuthChallenge struct {
	ID, Email, Purpose, UserID, MAC string
	ExpiresAt                       time.Time
	MaxAttempts                     int
}
type EmailProof struct{ ID, Email, Purpose, UserID, MAC string }

func invalidCode() error {
	return domain.Err(400, "INVALID_VERIFICATION_CODE", "验证码无效或已过期，请检查或重新获取")
}

func (s *Store) ReserveAuthChallenge(ctx context.Context, c AuthChallenge, ttl, cooldown time.Duration, limits []AuthLimit) (AuthChallenge, error) {
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return c, dbError(e)
	}
	defer tx.Rollback(ctx)
	if e = lockEmail(ctx, tx, c.Email); e != nil {
		return c, e
	}
	var retry int32
	e = tx.QueryRow(ctx, `SELECT COALESCE(GREATEST(0,ceil(extract(epoch FROM max(created_at)+make_interval(secs=>$2)-clock_timestamp()))),0)::integer FROM auth_challenges WHERE email_key=$1`, c.Email, cooldown.Seconds()).Scan(&retry)
	if e != nil {
		return c, dbError(e)
	}
	if retry > 0 {
		return c, rateError(retry)
	}
	if e = takeAuthLimits(ctx, tx, limits); e != nil {
		return c, e
	}
	e = tx.QueryRow(ctx, `INSERT INTO auth_challenges(id,email_key,purpose,user_id,code_mac,status,expires_at,max_attempts)
 VALUES($1,$2,$3,$4,$5,'sending',clock_timestamp()+make_interval(secs=>$6),$7) RETURNING expires_at`, c.ID, c.Email, c.Purpose, c.UserID, c.MAC, ttl.Seconds(), c.MaxAttempts).Scan(&c.ExpiresAt)
	if e != nil {
		return c, dbError(e)
	}
	return c, dbError(tx.Commit(ctx))
}

// CompleteAuthDelivery never lets a late SMTP acknowledgement supersede a
// newer request. It runs after SMTP, outside the reservation transaction.
func (s *Store) CompleteAuthDelivery(ctx context.Context, id string, success bool) (bool, error) {
	var email string
	if e := s.Pool.QueryRow(ctx, "SELECT email_key FROM auth_challenges WHERE id=$1", id).Scan(&email); e != nil {
		return false, dbError(e)
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return false, dbError(e)
	}
	defer tx.Rollback(ctx)
	if e = lockEmail(ctx, tx, email); e != nil {
		return false, e
	}
	if !success {
		_, e = tx.Exec(ctx, "UPDATE auth_challenges SET status='failed' WHERE id=$1 AND status='sending'", id)
		if e != nil {
			return false, dbError(e)
		}
		return false, dbError(tx.Commit(ctx))
	}
	var eligible bool
	e = tx.QueryRow(ctx, `SELECT c.status='sending' AND c.expires_at>clock_timestamp() AND NOT EXISTS(SELECT 1 FROM auth_challenges n WHERE n.email_key=c.email_key AND n.created_at>c.created_at AND n.status IN ('sending','ready','consumed')) FROM auth_challenges c WHERE c.id=$1`, id).Scan(&eligible)
	if e != nil {
		return false, dbError(e)
	}
	if eligible {
		_, e = tx.Exec(ctx, "UPDATE auth_challenges SET status='superseded' WHERE email_key=$1 AND id<>$2 AND status='ready'", email, id)
		if e != nil {
			return false, dbError(e)
		}
		_, e = tx.Exec(ctx, "UPDATE auth_challenges SET status='ready' WHERE id=$1", id)
	} else {
		_, e = tx.Exec(ctx, "UPDATE auth_challenges SET status='failed' WHERE id=$1 AND status='sending'", id)
	}
	if e != nil {
		return false, dbError(e)
	}
	return eligible, dbError(tx.Commit(ctx))
}

func (s *Store) CompleteEmailProof(ctx context.Context, p EmailProof, newUser domain.User, passwordHash string) (domain.User, error) {
	var empty domain.User
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return empty, dbError(e)
	}
	defer tx.Rollback(ctx)
	if e = lockEmail(ctx, tx, p.Email); e != nil {
		return empty, e
	}
	var email, purpose, uid, mac, status string
	var attempts, max int
	var unexpired bool
	e = tx.QueryRow(ctx, `SELECT email_key,purpose,user_id,code_mac,status,attempts,max_attempts,expires_at>clock_timestamp() FROM auth_challenges WHERE id=$1 FOR UPDATE`, p.ID).Scan(&email, &purpose, &uid, &mac, &status, &attempts, &max, &unexpired)
	if e == pgx.ErrNoRows {
		return empty, invalidCode()
	}
	if e != nil {
		return empty, dbError(e)
	}
	if email != p.Email || purpose != p.Purpose || uid != p.UserID || status != "ready" || !unexpired || attempts >= max {
		return empty, invalidCode()
	}
	if !hmac.Equal([]byte(mac), []byte(p.MAC)) {
		_, e = tx.Exec(ctx, "UPDATE auth_challenges SET attempts=attempts+1,status=CASE WHEN attempts+1>=max_attempts THEN 'failed' ELSE status END WHERE id=$1", p.ID)
		if e != nil {
			return empty, dbError(e)
		}
		if e = tx.Commit(ctx); e != nil {
			return empty, dbError(e)
		}
		return empty, invalidCode()
	}
	var existing bool
	e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email_key=$1)", p.Email).Scan(&existing)
	if e != nil {
		return empty, dbError(e)
	}
	if existing {
		_, e = tx.Exec(ctx, "UPDATE auth_challenges SET status='consumed',consumed_at=clock_timestamp() WHERE id=$1", p.ID)
		if e != nil {
			return empty, dbError(e)
		}
		if e = tx.Commit(ctx); e != nil {
			return empty, dbError(e)
		}
		return empty, domain.Err(409, "EMAIL_ALREADY_REGISTERED", "该邮箱已注册，请使用已有账号登录")
	}
	var user domain.User
	if p.Purpose == "register" {
		user = newUser
		user.Email = p.Email
		user.EmailVerified = true
		user.Role = "user"
		_, e = tx.Exec(ctx, "INSERT INTO users(id,username,password_hash,role,email,email_key,email_verified_at) VALUES($1,$2,$3,'user',$4,$4,clock_timestamp())", user.ID, user.Username, passwordHash, p.Email)
	} else if p.Purpose == "bind" && p.UserID != "" {
		e = tx.QueryRow(ctx, "SELECT id,username,role,COALESCE(email,''),email_verified_at IS NOT NULL FROM users WHERE id=$1 AND active FOR UPDATE", p.UserID).Scan(&user.ID, &user.Username, &user.Role, &user.Email, &user.EmailVerified)
		if e != nil {
			return empty, dbError(e)
		}
		if user.Email != "" || user.EmailVerified {
			return empty, domain.Err(409, "EMAIL_ALREADY_BOUND", "当前账号已绑定邮箱")
		}
		_, e = tx.Exec(ctx, "UPDATE users SET email=$2,email_key=$2,email_verified_at=clock_timestamp() WHERE id=$1", p.UserID, p.Email)
		user.Email = p.Email
		user.EmailVerified = true
	} else {
		return empty, invalidCode()
	}
	if e != nil {
		return empty, dbError(e)
	}
	_, e = tx.Exec(ctx, "UPDATE auth_challenges SET status='consumed',consumed_at=clock_timestamp() WHERE id=$1", p.ID)
	if e != nil {
		return empty, dbError(e)
	}
	return user, dbError(tx.Commit(ctx))
}

func (s *Store) EmailCredentials(ctx context.Context, email string) (domain.User, string, error) {
	var u domain.User
	var hash string
	e := s.Pool.QueryRow(ctx, "SELECT id,username,role,email,email_verified_at IS NOT NULL,password_hash FROM users WHERE email_key=$1 AND active AND email_verified_at IS NOT NULL", email).Scan(&u.ID, &u.Username, &u.Role, &u.Email, &u.EmailVerified, &hash)
	if e == pgx.ErrNoRows {
		return u, "", domain.Err(401, "INVALID_CREDENTIALS", "邮箱或密码错误")
	}
	return u, hash, dbError(e)
}
func (s *Store) LegacyCredentials(ctx context.Context, name string) (domain.User, string, error) {
	var u domain.User
	var hash string
	e := s.Pool.QueryRow(ctx, "SELECT id,username,role,password_hash FROM users WHERE username=$1 AND active AND email_key IS NULL AND email_verified_at IS NULL", name).Scan(&u.ID, &u.Username, &u.Role, &hash)
	if e == pgx.ErrNoRows {
		return u, "", domain.Err(401, "INVALID_CREDENTIALS", "用户名或密码错误")
	}
	return u, hash, dbError(e)
}
func (s *Store) PromoteVerifiedUser(ctx context.Context, email string) error {
	normalized, e := authn.NormalizeEmail(email)
	if e != nil {
		return domain.Err(400, "INVALID_INPUT", "邮箱格式无效")
	}
	result, e := s.Pool.Exec(ctx, "UPDATE users SET role='admin' WHERE email_key=$1 AND email_verified_at IS NOT NULL AND active", normalized)
	if e != nil {
		return dbError(e)
	}
	if result.RowsAffected() != 1 {
		return domain.Err(404, "NOT_FOUND", "请先完成邮箱验证码注册")
	}
	return nil
}
