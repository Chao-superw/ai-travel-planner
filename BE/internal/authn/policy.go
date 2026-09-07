package authn

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Policy struct {
	HMACKey                                                 []byte
	CodeTTL, ResendInterval                                 time.Duration
	MaxAttempts                                             int
	SendEmailHour, SendIPHour, SendGlobalHour               int
	VerifyEmailHour, VerifyIPHour, VerifyGlobalHour         int
	LoginAccountQuarter, LoginIPQuarter, LoginGlobalQuarter int
}

func DefaultPolicy(key []byte) Policy {
	return Policy{HMACKey: key, CodeTTL: 10 * time.Minute, ResendInterval: time.Minute, MaxAttempts: 5,
		SendEmailHour: 5, SendIPHour: 100, SendGlobalHour: 500,
		VerifyEmailHour: 30, VerifyIPHour: 300, VerifyGlobalHour: 3000,
		LoginAccountQuarter: 10, LoginIPQuarter: 100, LoginGlobalQuarter: 1000}
}

func LoadPolicy() (Policy, error) {
	p := DefaultPolicy([]byte(os.Getenv("AUTH_CODE_HMAC_KEY")))
	if len(p.HMACKey) < 32 || len(p.HMACKey) > 512 {
		return p, fmt.Errorf("AUTH_CODE_HMAC_KEY must contain 32 to 512 bytes")
	}
	for _, v := range []struct {
		name     string
		target   *int
		min, max int
	}{
		{"AUTH_CODE_MAX_ATTEMPTS", &p.MaxAttempts, 1, 10},
		{"AUTH_SEND_EMAIL_HOUR", &p.SendEmailHour, 1, 50}, {"AUTH_SEND_IP_HOUR", &p.SendIPHour, 1, 10000}, {"AUTH_SEND_GLOBAL_HOUR", &p.SendGlobalHour, 1, 100000},
		{"AUTH_VERIFY_EMAIL_HOUR", &p.VerifyEmailHour, 1, 100}, {"AUTH_VERIFY_IP_HOUR", &p.VerifyIPHour, 1, 10000}, {"AUTH_VERIFY_GLOBAL_HOUR", &p.VerifyGlobalHour, 1, 100000},
		{"AUTH_LOGIN_ACCOUNT_QUARTER", &p.LoginAccountQuarter, 1, 100}, {"AUTH_LOGIN_IP_QUARTER", &p.LoginIPQuarter, 1, 10000}, {"AUTH_LOGIN_GLOBAL_QUARTER", &p.LoginGlobalQuarter, 1, 100000},
	} {
		if raw := os.Getenv(v.name); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < v.min || n > v.max {
				return p, fmt.Errorf("invalid %s", v.name)
			}
			*v.target = n
		}
	}
	for _, v := range []struct {
		name     string
		target   *time.Duration
		min, max int
	}{
		{"AUTH_CODE_TTL_SECONDS", &p.CodeTTL, 60, 900}, {"AUTH_CODE_RESEND_SECONDS", &p.ResendInterval, 30, 300},
	} {
		if raw := os.Getenv(v.name); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < v.min || n > v.max {
				return p, fmt.Errorf("invalid %s", v.name)
			}
			*v.target = time.Duration(n) * time.Second
		}
	}
	return p, nil
}
