package service

import (
	"ai-travel/internal/authn"
	"ai-travel/internal/domain"
	"ai-travel/internal/store"
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"net/netip"
	"time"
)

type MailSender interface {
	SendCode(context.Context, string, string, string, time.Time) error
}

var dummyPasswordHash = func() []byte {
	h, e := bcrypt.GenerateFromPassword([]byte("timing-only-dummy-password-123"), bcrypt.DefaultCost)
	if e != nil {
		panic(e)
	}
	return h
}()

func (t *Travel) authLimits(group, account, ip string, accountMax, ipMax, globalMax int, window time.Duration) []store.AuthLimit {
	if parsed, e := netip.ParseAddr(ip); e == nil {
		ip = parsed.Unmap().String()
	} else {
		ip = "unknown"
	}
	key := t.AuthPolicy.HMACKey
	return []store.AuthLimit{
		{Key: authn.RateKey(key, group+":0-global", "global"), Max: globalMax, Window: window},
		{Key: authn.RateKey(key, group+":1-ip", ip), Max: ipMax, Window: window},
		{Key: authn.RateKey(key, group+":2-account", account), Max: accountMax, Window: window},
	}
}
func (t *Travel) authConfigured() error {
	if len(t.AuthPolicy.HMACKey) < 32 {
		return domain.Err(503, "AUTH_UNAVAILABLE", "认证配置不可用")
	}
	return nil
}
func emailInput(raw string) (string, error) {
	v, e := authn.NormalizeEmail(raw)
	if e != nil {
		return "", domain.Err(400, "INVALID_INPUT", "请输入有效的邮箱地址")
	}
	return v, nil
}
func mailUnavailable() error {
	return domain.Err(503, "MAIL_UNAVAILABLE", "验证码邮件暂时无法发送，请稍后重试")
}

func (t *Travel) issueEmailCode(ctx context.Context, r domain.Request, purpose, userID string) (domain.Response, error) {
	var res domain.Response
	email, e := emailInput(r.Email)
	if e != nil {
		return res, e
	}
	if e = t.authConfigured(); e != nil {
		return res, e
	}
	if t.Mailer == nil {
		return res, mailUnavailable()
	}
	code, e := authn.GenerateCode()
	if e != nil {
		return res, domain.Err(503, "AUTH_UNAVAILABLE", "验证码暂不可用")
	}
	p := t.AuthPolicy
	c := store.AuthChallenge{ID: domain.ID(), Email: email, Purpose: purpose, UserID: userID, MaxAttempts: p.MaxAttempts}
	c.MAC = authn.CodeMAC(p.HMACKey, c.ID, email, purpose, userID, code)
	c, e = t.Store.ReserveAuthChallenge(ctx, c, p.CodeTTL, p.ResendInterval, t.authLimits("send", email, r.ClientIP, p.SendEmailHour, p.SendIPHour, p.SendGlobalHour, time.Hour))
	if e != nil {
		return res, e
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	sendErr := t.Mailer.SendCode(sendCtx, email, code, purpose, c.ExpiresAt)
	cancel()
	// Finishing the known delivery outcome must not depend on an HTTP client
	// remaining connected. It is bounded independently and contains no SMTP retry.
	finalizeCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer done()
	ready, finishErr := t.Store.CompleteAuthDelivery(finalizeCtx, c.ID, sendErr == nil)
	if sendErr != nil {
		return res, mailUnavailable()
	}
	if finishErr != nil {
		return res, finishErr
	}
	if !ready {
		return res, mailUnavailable()
	}
	seconds := int32(time.Until(c.ExpiresAt).Seconds())
	if seconds < 1 {
		return res, mailUnavailable()
	}
	res.Challenge = &domain.EmailChallenge{ChallengeID: c.ID, ExpiresIn: seconds, RetryAfter: int32(p.ResendInterval.Seconds())}
	return res, nil
}
func (t *Travel) registerEmail(ctx context.Context, r domain.Request) (domain.Response, error) {
	var res domain.Response
	email, e := emailInput(r.Email)
	if e != nil {
		return res, e
	}
	if len(r.Password) < 8 || len(r.Password) > 72 {
		return res, domain.Err(400, "INVALID_INPUT", "密码须为 8 至 72 字节")
	}
	proof, e := t.prepareEmailProof(ctx, r, email, "register", "")
	if e != nil {
		return res, e
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(r.Password), bcrypt.DefaultCost)
	if e != nil {
		return res, e
	}
	id := domain.ID()
	user := domain.User{ID: id, Username: "旅行者_" + id, Role: "user"}
	u, e := t.Store.CompleteEmailProof(ctx, proof, user, string(hash))
	if e != nil {
		return res, e
	}
	res.User = &u
	return res, nil
}
func (t *Travel) prepareEmailProof(ctx context.Context, r domain.Request, email, purpose, uid string) (store.EmailProof, error) {
	var proof store.EmailProof
	if e := t.authConfigured(); e != nil {
		return proof, e
	}
	p := t.AuthPolicy
	if e := t.Store.TakeAuthLimits(ctx, t.authLimits("verify", email, r.ClientIP, p.VerifyEmailHour, p.VerifyIPHour, p.VerifyGlobalHour, time.Hour)); e != nil {
		return proof, e
	}
	if !authn.ValidCode(r.Code) || len(r.ChallengeID) < 1 || len(r.ChallengeID) > 128 {
		return proof, domain.Err(400, "INVALID_VERIFICATION_CODE", "验证码无效或已过期，请检查或重新获取")
	}
	return store.EmailProof{ID: r.ChallengeID, Email: email, Purpose: purpose, UserID: uid, MAC: authn.CodeMAC(p.HMACKey, r.ChallengeID, email, purpose, uid, r.Code)}, nil
}
func (t *Travel) bindEmail(ctx context.Context, r domain.Request, u domain.User) (domain.Response, error) {
	var res domain.Response
	if u.Email != "" || u.EmailVerified {
		return res, domain.Err(409, "EMAIL_ALREADY_BOUND", "当前账号已绑定邮箱")
	}
	email, e := emailInput(r.Email)
	if e != nil {
		return res, e
	}
	proof, e := t.prepareEmailProof(ctx, r, email, "bind", u.ID)
	if e != nil {
		return res, e
	}
	bound, e := t.Store.CompleteEmailProof(ctx, proof, domain.User{}, "")
	if e != nil {
		return res, e
	}
	res.User = &bound
	return res, nil
}
func (t *Travel) loginEmail(ctx context.Context, r domain.Request, legacy bool) (domain.Response, error) {
	var res domain.Response
	if e := t.authConfigured(); e != nil {
		return res, e
	}
	account := r.Username
	message := "用户名或密码错误"
	valid := len(account) > 0 && len(account) <= 128
	if !legacy {
		message = "邮箱或密码错误"
		email, e := authn.NormalizeEmail(r.Email)
		valid = e == nil
		account = email
	}
	p := t.AuthPolicy
	if e := t.Store.TakeAuthLimits(ctx, t.authLimits("login", account, r.ClientIP, p.LoginAccountQuarter, p.LoginIPQuarter, p.LoginGlobalQuarter, 15*time.Minute)); e != nil {
		return res, e
	}
	if !valid || len(r.Password) > 72 || len(r.Password) < 8 {
		bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte("invalid-input"))
		return res, domain.Err(401, "INVALID_CREDENTIALS", message)
	}
	var u domain.User
	var hash string
	var e error
	if legacy {
		u, hash, e = t.Store.LegacyCredentials(ctx, account)
	} else {
		u, hash, e = t.Store.EmailCredentials(ctx, account)
	}
	if e != nil {
		var api *domain.APIError
		if !errors.As(e, &api) || api.Code != "INVALID_CREDENTIALS" {
			return res, e
		}
		bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(r.Password))
		return res, domain.Err(401, "INVALID_CREDENTIALS", message)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(r.Password)) != nil {
		return res, domain.Err(401, "INVALID_CREDENTIALS", message)
	}
	token := domain.ID() + domain.ID()
	if e = t.Store.CreateSession(ctx, Hash(token), u.ID); e != nil {
		return res, e
	}
	res.Session = &domain.Session{Token: token, ExpiresAt: time.Now().Add(8 * time.Hour).UTC().Format(time.RFC3339), User: u}
	return res, nil
}
