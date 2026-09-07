package gateway

import (
	"ai-travel/internal/domain"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"strings"
	"testing"
)

func TestEmailRegistrationAcceptsVerificationFields(t *testing.T) {
	f := &fakeCaller{response: domain.Response{User: &domain.User{ID: "email-user", Role: "user"}}}
	status, _, _ := perform(t, f, "POST", "/api/v1/auth/register", `{"email":"user@example.com","password":"long-password-123","code":"123456","challenge_id":"challenge"}`)
	if status != 201 {
		t.Fatalf("email registration fields rejected: status=%d", status)
	}
}

func TestEmailLoginAndBindingMapNarrowDTOs(t *testing.T) {
	for _, tc := range []struct {
		path, method, body string
		protected          bool
	}{
		{"/api/v1/auth/login", "Login", `{"email":"user@example.com","password":"password8"}`, false},
		{"/api/v1/auth/register/code", "RequestRegistrationCode", `{"email":"user@example.com"}`, false},
		{"/api/v1/me/email/code", "RequestEmailBindingCode", `{"email":"user@example.com"}`, true},
		{"/api/v1/me/email", "BindEmail", `{"email":"user@example.com","code":"123456","challenge_id":"challenge"}`, true},
	} {
		f := &fakeCaller{response: domain.Response{Challenge: &domain.EmailChallenge{ChallengeID: "challenge", ExpiresIn: 600, RetryAfter: 60}}}
		headers := []ut.Header{{Key: "X-Forwarded-For", Value: "198.51.100.123"}}
		if tc.protected {
			headers = append(headers, authHeaders()...)
		}
		status, _, _ := perform(t, f, "POST", tc.path, tc.body, headers...)
		if status != 200 || len(f.calls) != 1 || f.calls[0].method != tc.method || f.calls[0].req.Email != "user@example.com" {
			t.Fatalf("bad mapping %s: %d %+v", tc.path, status, f.calls)
		}
		if f.calls[0].req.ClientIP == "198.51.100.123" {
			t.Fatal("untrusted forwarded IP accepted")
		}
	}
}
func TestEmailEndpointsRejectClientIdentityAndUsernameLogin(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/api/v1/auth/register/code", `{"email":"a@example.com","client_ip":"1.2.3.4"}`},
		{"/api/v1/auth/login", `{"username":"old-user","password":"password8"}`},
		{"/api/v1/auth/register", `{"email":"a@example.com","password":"password8","code":"123456","challenge_id":"id","role":"admin"}`},
	} {
		f := &fakeCaller{}
		status, _, _ := perform(t, f, "POST", tc.path, tc.body)
		if status != 400 || len(f.calls) != 0 {
			t.Fatalf("identity injection accepted: %s", tc.path)
		}
	}
	for _, path := range []string{"/api/v1/me/email/code", "/api/v1/me/email"} {
		f := &fakeCaller{}
		status, _, _ := perform(t, f, "POST", path, `{"email":"a@example.com"}`)
		if status != 401 {
			t.Fatal("binding is not authenticated")
		}
	}
}
func TestAuthRateLimitIncludesRetryAfterHeader(t *testing.T) {
	seconds := int32(60)
	f := &fakeCaller{response: domain.Response{Error: &domain.APIError{Code: "AUTH_RATE_LIMITED", Status: 429, Message: "稍后重试", RetryAfter: &seconds}}}
	h := New("127.0.0.1:0", "", f, "")
	body := `{"email":"a@example.com"}`
	r := ut.PerformRequest(h.Engine, "POST", "/api/v1/auth/register/code", &ut.Body{Body: strings.NewReader(body), Len: len(body)})
	if r.Code != 429 || r.Header().Get("Retry-After") != "60" {
		t.Fatalf("missing retry guidance: status=%d", r.Code)
	}
}

func TestAuthErrorsAreNotCacheable(t *testing.T) {
	f := &fakeCaller{}
	h := New("127.0.0.1:0", "", f, "")
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/api/v1/auth/register", `{"bad":true}`}, {"GET", "/api/v1/me", ""}, {"POST", "/api/v1/auth/logout", ""},
	} {
		r := ut.PerformRequest(h.Engine, tc.method, tc.path, &ut.Body{Body: strings.NewReader(tc.body), Len: len(tc.body)})
		if r.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("cacheable auth response: %s", tc.path)
		}
	}
}
