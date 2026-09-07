package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"ai-travel/internal/domain"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

type callRecord struct {
	method string
	req    domain.Request
}

type fakeCaller struct {
	response domain.Response
	err      error
	calls    []callRecord
}

func (f *fakeCaller) Call(_ context.Context, method string, req domain.Request) (domain.Response, error) {
	f.calls = append(f.calls, callRecord{method: method, req: req})
	return f.response, f.err
}

func perform(t *testing.T, f *fakeCaller, method, path, body string, headers ...ut.Header) (int, map[string]any, map[string]string) {
	t.Helper()
	h := New("127.0.0.1:0", "http://localhost:5173", f, "")
	var b *ut.Body
	if body != "" {
		b = &ut.Body{Body: bytes.NewBufferString(body), Len: len(body)}
	}
	r := ut.PerformRequest(h.Engine, method, path, b, headers...)
	var payload map[string]any
	if len(r.Body.Bytes()) != 0 {
		if err := json.Unmarshal(r.Body.Bytes(), &payload); err != nil {
			t.Fatalf("response is not JSON: %s (%v)", r.Body.Bytes(), err)
		}
	}
	gotHeaders := map[string]string{
		"content_type": r.Header().Get("Content-Type"),
		"allow_origin": r.Header().Get("Access-Control-Allow-Origin"),
	}
	return r.Code, payload, gotHeaders
}

func authHeaders(extra ...ut.Header) []ut.Header {
	return append([]ut.Header{{Key: "Authorization", Value: "Bearer secret-token"}, {Key: "Content-Type", Value: "application/json"}}, extra...)
}

func TestRegisterMapsBodyAndReturnsCreatedEnvelope(t *testing.T) {
	f := &fakeCaller{response: domain.Response{User: &domain.User{ID: "user-id", Username: "旅行者", Role: "user"}}}
	status, body, _ := perform(t, f, "POST", "/api/v1/auth/register", `{"email":"user@example.com","password":"password8","code":"123456","challenge_id":"challenge"}`, ut.Header{Key: "Content-Type", Value: "application/json"})
	if status != 201 || len(f.calls) != 1 || f.calls[0].method != "Register" || f.calls[0].req.Email != "user@example.com" || f.calls[0].req.Code != "123456" || f.calls[0].req.ChallengeID != "challenge" || f.calls[0].req.Password != "password8" {
		t.Fatalf("unexpected result: status=%d calls=%+v", status, f.calls)
	}
	if body["request_id"] == "" || body["data"].(map[string]any)["id"] != "user-id" {
		t.Fatalf("unexpected envelope: %#v", body)
	}
}

func TestProtectedRoutesRejectMissingBearerWithoutCallingBackend(t *testing.T) {
	paths := []struct{ method, path string }{
		{"POST", "/api/v1/auth/logout"}, {"GET", "/api/v1/me"}, {"GET", "/api/v1/places?city=x"},
		{"POST", "/api/v1/admin/places"}, {"PATCH", "/api/v1/admin/places/p1"}, {"POST", "/api/v1/planning-jobs"},
		{"GET", "/api/v1/planning-jobs/j1"}, {"POST", "/api/v1/planning-jobs/j1/retries"}, {"GET", "/api/v1/trips"},
		{"GET", "/api/v1/trips/t1"}, {"PATCH", "/api/v1/trips/t1"}, {"POST", "/api/v1/trips/t1/replanning-jobs"},
		{"GET", "/api/v1/trips/t1/versions"}, {"GET", "/api/v1/trips/t1/versions/1"}, {"GET", "/api/v1/trips/t1/budget"},
	}
	for _, tc := range paths {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			f := &fakeCaller{}
			status, body, _ := perform(t, f, tc.method, tc.path, "")
			if status != 401 || len(f.calls) != 0 || body["code"] != "UNAUTHORIZED" || body["request_id"] == "" {
				t.Fatalf("unexpected rejection: %d %#v calls=%d", status, body, len(f.calls))
			}
		})
	}
}

func TestSearchValidatesPaginationAndMapsResponse(t *testing.T) {
	f := &fakeCaller{response: domain.Response{Places: []domain.Place{{ID: "p1", Name: "博物馆"}}, Page: 2, PageSize: 25, HasMore: true}}
	status, body, _ := perform(t, f, "GET", "/api/v1/places?city=%E6%9D%AD%E5%B7%9E%E5%B8%82&keyword=%E5%8D%9A%E7%89%A9%E9%A6%86&page=2&page_size=25", "", authHeaders()...)
	data := body["data"].(map[string]any)
	if status != 200 || f.calls[0].method != "SearchPlaces" || f.calls[0].req.Page != 2 || f.calls[0].req.PageSize != 25 || data["has_more"] != true {
		t.Fatalf("unexpected search: %d %#v %+v", status, body, f.calls)
	}

	for _, query := range []string{"page=0", "page=-1", "page=2147483648", "page_size=26", "page_size=x"} {
		f := &fakeCaller{}
		status, _, _ := perform(t, f, "GET", "/api/v1/places?city=x&"+query, "", authHeaders()...)
		if status != 400 || len(f.calls) != 0 {
			t.Fatalf("query %q: status=%d calls=%d", query, status, len(f.calls))
		}
	}
}

func TestStrictJSONRejectsUnknownTrailingAndOversizedBodies(t *testing.T) {
	for name, body := range map[string]string{
		"unknown":  `{"username":"abc","password":"password8","role":"admin"}`,
		"trailing": `{"username":"abc","password":"password8"} {}`,
		"oversize": `{"username":"` + strings.Repeat("a", (1<<20)+1) + `","password":"password8"}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeCaller{}
			status, _, _ := perform(t, f, "POST", "/api/v1/auth/register", body, ut.Header{Key: "Content-Type", Value: "application/json"})
			if status != 400 || len(f.calls) != 0 {
				t.Fatalf("status=%d calls=%d", status, len(f.calls))
			}
		})
	}
}

func TestPlanningJobMapsConstraintsIdempotencyAndStatusURL(t *testing.T) {
	f := &fakeCaller{response: domain.Response{Job: &domain.Job{ID: "job-id", Status: "queued"}}}
	body := `{"constraints":{"city":"杭州市","start_date":"2026-10-01","end_date":"2026-10-03","party_size":2,"budget_cents":200000,"budget_scope":"per_person","interests":["文化","美食"],"pace":"balanced","transport":"transit"}}`
	status, got, _ := perform(t, f, "POST", "/api/v1/planning-jobs", body, authHeaders(ut.Header{Key: "Idempotency-Key", Value: "create-1"})...)
	data := got["data"].(map[string]any)
	if status != 202 || f.calls[0].method != "CreatePlanningJob" || f.calls[0].req.IdempotencyKey != "create-1" || f.calls[0].req.Input.Constraints.BudgetCents != 200000 || data["status_url"] != "/api/v1/planning-jobs/job-id" {
		t.Fatalf("unexpected create: %d %#v %+v", status, got, f.calls)
	}

	f = &fakeCaller{}
	bad := strings.Replace(body, `"budget_scope"`, `"budget_total_cents":400000,"budget_scope"`, 1)
	status, _, _ = perform(t, f, "POST", "/api/v1/planning-jobs", bad, authHeaders(ut.Header{Key: "Idempotency-Key", Value: "create-1"})...)
	if status != 400 || len(f.calls) != 0 {
		t.Fatalf("server-owned field accepted: status=%d", status)
	}
}

func TestUpdateTripUsesNarrowPlanInputAndPathID(t *testing.T) {
	f := &fakeCaller{response: domain.Response{Job: &domain.Job{ID: "job-id", Status: "queued"}}}
	body := `{"expected_version":3,"plan":{"title":"周末","summary":"摘要","activities":[{"id":"a1","date":"2026-10-01","start_minute":540,"end_minute":600,"kind":"place","place_id":"p1","title":"博物馆","reason":"文化"}]}}`
	status, _, _ := perform(t, f, "PATCH", "/api/v1/trips/trip-id", body, authHeaders(ut.Header{Key: "Idempotency-Key", Value: "edit-1"})...)
	if status != 202 || f.calls[0].method != "UpdateTrip" || f.calls[0].req.ID != "trip-id" || f.calls[0].req.Input.ExpectedVersion != 3 || len(f.calls[0].req.Input.Plan.Activities) != 1 {
		t.Fatalf("unexpected update: %d %+v", status, f.calls)
	}

	f = &fakeCaller{}
	readOnly := strings.Replace(body, `"reason":"文化"`, `"reason":"文化","costs":[]`, 1)
	status, _, _ = perform(t, f, "PATCH", "/api/v1/trips/trip-id", readOnly, authHeaders(ut.Header{Key: "Idempotency-Key", Value: "edit-1"})...)
	if status != 400 || len(f.calls) != 0 {
		t.Fatalf("read-only activity fields accepted")
	}
}

func TestBackendAndTransportErrorsAreSanitized(t *testing.T) {
	f := &fakeCaller{response: domain.Response{Error: &domain.APIError{Status: 409, Code: "VERSION_CONFLICT", Message: "请刷新", FieldErrors: map[string]string{"expected_version": "过期"}}}}
	status, body, _ := perform(t, f, "GET", "/api/v1/trips/t1", "", authHeaders()...)
	if status != 409 || body["code"] != "VERSION_CONFLICT" || body["status"] != nil || body["field_errors"] == nil {
		t.Fatalf("bad business error: %d %#v", status, body)
	}

	f = &fakeCaller{err: errors.New("dial tcp https://secret.example/path?key=credential")}
	status, body, _ = perform(t, f, "GET", "/api/v1/trips/t1", "", authHeaders()...)
	if status != 503 || body["code"] != "DEPENDENCY_UNAVAILABLE" || strings.Contains(body["message"].(string), "secret") {
		t.Fatalf("leaked transport error: %d %#v", status, body)
	}
}

func TestKitexRPCTimeoutMapsToGatewayTimeoutWhileContextIsActive(t *testing.T) {
	f := &fakeCaller{err: kerrors.ErrRPCTimeout}
	status, body, _ := perform(t, f, "GET", "/api/v1/trips/t1", "", authHeaders()...)
	if status != 504 || body["code"] != "TIMEOUT" || body["message"] != "操作超时" {
		t.Fatalf("Kitex timeout was not sanitized as 504: status=%d body=%#v", status, body)
	}
}

func TestDeadlineAndCORS(t *testing.T) {
	type deadlineCaller struct{ seen time.Duration }
	var seen time.Duration
	caller := CallerFunc(func(ctx context.Context, _ string, _ domain.Request) (domain.Response, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("missing deadline")
		}
		seen = time.Until(deadline)
		return domain.Response{Places: []domain.Place{}}, nil
	})
	h := New("127.0.0.1:0", "http://localhost:5173", caller, "")
	r := ut.PerformRequest(h.Engine, "GET", "/api/v1/places?city=x", nil, ut.Header{Key: "Authorization", Value: "Bearer t"}, ut.Header{Key: "Origin", Value: "http://localhost:5173"})
	if r.Code != 200 || seen < 7*time.Second || seen > 8*time.Second || r.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("deadline/cors: %v %q", seen, r.Header().Get("Access-Control-Allow-Origin"))
	}

	r = ut.PerformRequest(h.Engine, "OPTIONS", "/api/v1/trips", nil, ut.Header{Key: "Origin", Value: "https://evil.example"}, ut.Header{Key: "Access-Control-Request-Method", Value: "GET"})
	if r.Code == 204 || r.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unconfigured origin allowed: %d", r.Code)
	}
	r = ut.PerformRequest(h.Engine, "OPTIONS", "/api/v1/trips", nil, ut.Header{Key: "Origin", Value: "http://localhost:5173"}, ut.Header{Key: "Access-Control-Request-Method", Value: "GET"})
	if r.Code != 204 || r.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("configured preflight rejected: %d", r.Code)
	}
}

type CallerFunc func(context.Context, string, domain.Request) (domain.Response, error)

func (f CallerFunc) Call(ctx context.Context, method string, req domain.Request) (domain.Response, error) {
	return f(ctx, method, req)
}
