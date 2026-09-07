package service

import (
	"ai-travel/internal/domain"
	"ai-travel/internal/store"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type routeFallbackMap struct {
	respond    func(string) (domain.Route, error)
	modes      []string
	transitErr error
	walkingErr error
	walking    domain.Route
}

func (m *routeFallbackMap) Search(context.Context, string, string, int, int) ([]domain.Place, bool, error) {
	panic("unexpected search")
}
func (m *routeFallbackMap) Route(_ context.Context, _, _ domain.Place, mode string) (domain.Route, error) {
	m.modes = append(m.modes, mode)
	if m.respond != nil {
		return m.respond(mode)
	}
	if mode == "transit" {
		return domain.Route{}, m.transitErr
	}
	return m.walking, m.walkingErr
}
func TestTransitShortWalkingFallback(t *testing.T) {
	zero := int64(0)
	for _, tc := range []struct {
		name                                      string
		distance, duration                        int64
		startCalls                                int
		preference, firstCode, walkCode, wantCode string
		wantCalls                                 int
	}{
		{"short", 1500, 1500, 0, "transit", "MAP_NO_ROUTE", "", "", 2},
		{"far", 1501, 100, 0, "transit", "MAP_NO_ROUTE", "", "MAP_NO_ROUTE", 2},
		{"slow", 100, 1501, 0, "transit", "MAP_NO_ROUTE", "", "MAP_NO_ROUTE", 2},
		{"negative-distance", -1, 100, 0, "transit", "MAP_NO_ROUTE", "", "MAP_NO_ROUTE", 2},
		{"negative-duration", 100, -1, 0, "transit", "MAP_NO_ROUTE", "", "MAP_NO_ROUTE", 2},
		{"quota", 100, 100, 0, "transit", "MAP_RATE_LIMITED", "", "MAP_RATE_LIMITED", 1},
		{"unavailable", 100, 100, 0, "transit", "MAP_UNAVAILABLE", "", "MAP_UNAVAILABLE", 1},
		{"auth", 100, 100, 0, "transit", "MAP_AUTH_FAILED", "", "MAP_AUTH_FAILED", 1},
		{"second-call-budget", 100, 100, 63, "transit", "MAP_NO_ROUTE", "", "MAP_CALL_LIMIT", 1},
		{"two-slots", 100, 100, 62, "transit", "MAP_NO_ROUTE", "", "", 2},
		{"walking-failure", 100, 100, 0, "transit", "MAP_NO_ROUTE", "MAP_UNAVAILABLE", "MAP_UNAVAILABLE", 2},
		{"walking-confirmed-no-route", 100, 100, 0, "transit", "MAP_NO_ROUTE", "MAP_NO_ROUTE", "MAP_NO_ROUTE", 2},
		{"explicit-walking", 2000, 1600, 0, "walking", "", "", "", 1},
		{"walking-no-route", 100, 100, 0, "walking", "", "MAP_NO_ROUTE", "MAP_NO_ROUTE", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &routeFallbackMap{walking: domain.Route{Mode: "walking", DistanceM: tc.distance, DurationS: tc.duration, FeeCents: &zero, Provider: "amap", QueriedAt: "original-query"}}
			if tc.firstCode != "" {
				m.transitErr = domain.Err(422, tc.firstCode, "private https://key.invalid?key=SECRET")
			}
			if tc.walkCode != "" {
				m.walkingErr = domain.Err(503, tc.walkCode, "private https://key.invalid?key=SECRET")
			}
			p := domain.Plan{Activities: []domain.Activity{{ID: "a", Title: "西湖", Date: "2026-10-01", Kind: "sightseeing", EndMinute: 600, Place: &domain.Place{Name: "西湖", Location: "1,1"}}, {ID: "b", Title: "博物馆", Date: "2026-10-01", Kind: "sightseeing", StartMinute: 660, Place: &domain.Place{Name: "博物馆", Location: "2,2"}}}}
			r := store.Record{Input: domain.JobInput{Constraints: domain.Constraints{Transport: tc.preference}}}
			calls := tc.startCalls
			cache := map[string]domain.Route{}
			out, issues, err := (&Travel{Map: m}).routes(context.Background(), r, p, cache, &calls)
			if len(m.modes) != tc.wantCalls || calls != tc.startCalls+tc.wantCalls {
				t.Fatalf("calls modes=%v count=%d", m.modes, calls)
			}
			if tc.wantCode != "" {
				if tc.wantCode == "MAP_NO_ROUTE" {
					if err != nil || len(issues) != 1 || len(out.Routes) != 0 {
						t.Fatalf("no route must become repair issue without fake route: %v %v %+v", err, issues, out.Routes)
					}
					for _, v := range []string{"西湖", "博物馆", "2026-10-01"} {
						if !strings.Contains(issues[0], v) {
							t.Errorf("missing context %s: %s", v, issues[0])
						}
					}
					if strings.Contains(issues[0], "SECRET") || strings.Contains(issues[0], "https:") {
						t.Fatal("provider private details leaked")
					}
					return
				}
				var api *domain.APIError
				if !errors.As(err, &api) || api.Code != tc.wantCode {
					t.Fatalf("want %s got %v", tc.wantCode, err)
				}
				return
			}
			if err != nil || len(issues) > 0 {
				t.Fatalf("route failed %v %v", err, issues)
			}
			if len(out.Routes) != 1 || out.Routes[0].Mode != "walking" || out.Routes[0].Provider != "amap" || out.Routes[0].FeeCents == nil || *out.Routes[0].FeeCents != 0 {
				t.Fatalf("actual route lost: %+v", out.Routes)
			}
			warning := strings.Join(out.Warnings, " ")
			if (tc.preference == "transit") != strings.Contains(warning, "公交无方案，短程采用步行") {
				t.Fatalf("wrong fallback warning: %s", warning)
			}
			cached, _, err := (&Travel{Map: m}).routes(context.Background(), r, p, cache, &calls)
			if err != nil || len(m.modes) != tc.wantCalls {
				t.Fatal("fallback route not cached by preference", err, m.modes)
			}
			if tc.preference == "transit" && !strings.Contains(strings.Join(cached.Warnings, " "), "公交无方案，短程采用步行") {
				t.Fatal("cached fallback warning lost")
			}
			r.Base = &domain.Trip{Plan: out}
			reused, _, err := (&Travel{Map: m}).routes(context.Background(), r, p, map[string]domain.Route{}, &calls)
			if err != nil || len(m.modes) != tc.wantCalls || reused.Routes[0].QueriedAt != "original-query" {
				t.Fatal("unchanged baseline route not reused", err, m.modes)
			}
			if tc.preference == "transit" && !strings.Contains(strings.Join(reused.Warnings, " "), "公交无方案，短程采用步行") {
				t.Fatal("baseline fallback warning lost")
			}
		})
	}
}

func TestNoRouteCollectsRepairIssueAndContinuesAdjacentLegs(t *testing.T) {
	m := &routeFallbackMap{}
	m.respond = func(mode string) (domain.Route, error) {
		if len(m.modes) <= 2 {
			return domain.Route{}, domain.Err(422, "MAP_NO_ROUTE", "no route")
		}
		return domain.Route{Mode: "transit", Provider: "amap", DurationS: 60}, nil
	}
	p := domain.Plan{Activities: []domain.Activity{
		{ID: "a", Date: "2026-10-01", Kind: "sightseeing", StartMinute: 540, EndMinute: 600, Place: &domain.Place{Name: "西湖", Location: "1,1"}},
		{ID: "b", Date: "2026-10-01", Kind: "sightseeing", StartMinute: 660, EndMinute: 720, Place: &domain.Place{Name: "博物馆", Location: "2,2"}},
		{ID: "c", Date: "2026-10-01", Kind: "sightseeing", StartMinute: 780, EndMinute: 840, Place: &domain.Place{Name: "公园", Location: "3,3"}},
	}}
	r := store.Record{Input: domain.JobInput{Constraints: domain.Constraints{Transport: "transit"}}}
	calls := 0
	out, issues, err := (&Travel{Map: m}).routes(context.Background(), r, p, map[string]domain.Route{}, &calls)
	if err != nil || len(issues) != 1 || !strings.Contains(issues[0], "西湖 到 博物馆") {
		t.Fatalf("missing repair issue: %v %v", err, issues)
	}
	if calls != 3 || len(out.Routes) != 1 || out.Routes[0].FromItemID != "b" || out.Routes[0].ToItemID != "c" {
		t.Fatalf("wrong continuation/fake route: calls=%d routes=%+v", calls, out.Routes)
	}
	m = &routeFallbackMap{transitErr: domain.Err(503, "MAP_UNAVAILABLE", "unavailable")}
	calls = 0
	_, issues, err = (&Travel{Map: m}).routes(context.Background(), r, p, map[string]domain.Route{}, &calls)
	var api *domain.APIError
	if !errors.As(err, &api) || api.Code != "MAP_UNAVAILABLE" || len(issues) != 0 || calls != 1 {
		t.Fatalf("dependency error became repairable: %v %v %d", err, issues, calls)
	}
}

func TestWalkingFallbackPreservesDependencyErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cause  error
		status int32
		code   string
	}{
		{"unavailable", domain.Err(503, "MAP_UNAVAILABLE", "https://key.invalid?key=SECRET"), 503, "MAP_UNAVAILABLE"},
		{"quota", domain.Err(429, "MAP_QUOTA_EXCEEDED", "https://key.invalid?key=SECRET"), 429, "MAP_QUOTA_EXCEEDED"},
		{"auth", domain.Err(401, "MAP_AUTH_FAILED", "https://key.invalid?key=SECRET"), 401, "MAP_AUTH_FAILED"},
		{"deadline", fmt.Errorf("SECRET: %w", context.DeadlineExceeded), 504, "TIMEOUT"},
		{"canceled", fmt.Errorf("SECRET: %w", context.Canceled), 503, "CANCELED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &routeFallbackMap{transitErr: domain.Err(422, "MAP_NO_ROUTE", "no route"), walkingErr: tc.cause}
			from := domain.Activity{Date: "2026-10-01", Place: &domain.Place{Name: "西湖"}}
			to := domain.Activity{Place: &domain.Place{Name: "博物馆"}}
			calls := 0
			_, err := (&Travel{Map: m}).routeWithPreference(context.Background(), from, to, "transit", &calls)
			api := AsError(err)
			if api.Code != tc.code || api.Status != tc.status {
				t.Fatalf("classification lost: %+v", api)
			}
			if err == nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "https:") {
				t.Fatalf("unredacted error: %v", err)
			}
			for _, sentinel := range []error{context.DeadlineExceeded, context.Canceled} {
				if errors.Is(tc.cause, sentinel) && !errors.Is(err, sentinel) {
					t.Fatalf("context type lost: %v", err)
				}
			}
			if calls != 2 || len(m.modes) != 2 {
				t.Fatalf("unexpected retries: %v", m.modes)
			}
		})
	}
}

func TestHydrateDropsClientCostsAndRetainsUnknownTicket(t *testing.T) {
	zero := int64(0)
	c := domain.Constraints{PartySize: 2}
	places := []domain.Place{{ID: "amap:p", Active: true, Name: "Museum"}}
	p := domain.Plan{Activities: []domain.Activity{{ID: "a", Kind: "sightseeing", PlaceID: "amap:p", Costs: []domain.Cost{{Category: "ticket", AmountCents: &zero}}}}}
	out := Hydrate(c, p, places, nil)
	if len(out.Activities[0].Costs) != 1 || out.Activities[0].Costs[0].AmountCents != nil {
		t.Fatal("unverified free ticket accepted")
	}
	if out.Activities[0].Place == nil {
		t.Fatal("missing server snapshot")
	}
}
func TestHydratePreservesUntouchedActivity(t *testing.T) {
	p := domain.Plan{Activities: []domain.Activity{{ID: "a", Date: "2026-10-01", Kind: "meal", Title: "Lunch", StartMinute: 720, EndMinute: 780, Costs: []domain.Cost{{Category: "food", Unit: "group", Source: "historic"}}}}}
	input := p
	input.Activities = append([]domain.Activity(nil), p.Activities...)
	input.Activities[0].Costs = nil
	got := Hydrate(domain.Constraints{PartySize: 1}, input, nil, &p)
	if got.Activities[0].Costs[0].Source != "historic" {
		t.Fatal("history overwritten")
	}
}
func TestRouteWindowCannotUseInterruptedTime(t *testing.T) {
	p := domain.Plan{Activities: []domain.Activity{{ID: "meal", Date: "2026-10-01", StartMinute: 620, EndMinute: 640, Kind: "meal"}}}
	a := domain.Activity{ID: "a", Date: "2026-10-01", EndMinute: 600}
	b := domain.Activity{ID: "b", Date: "2026-10-01", StartMinute: 660}
	if domain.RouteIssue(p, a, b, domain.Route{DurationS: 1500}) == "" {
		t.Fatal("non-contiguous transport accepted")
	}
}
func TestRevisionBoundaryFeeUsesProtectedAnchor(t *testing.T) {
	oldfee, newfee := int64(100), int64(200)
	base := domain.Plan{Activities: []domain.Activity{{ID: "outside"}, {ID: "old-edit"}}, Routes: []domain.Route{{FromItemID: "outside", ToItemID: "old-edit", FeeCents: &oldfee}}}
	updated := domain.Plan{Activities: []domain.Activity{{ID: "outside"}, {ID: "new-edit"}}, Routes: []domain.Route{{FromItemID: "outside", ToItemID: "new-edit", FeeCents: &newfee}}}
	scope := domain.Scope{EditableItemIDs: []string{"old-edit"}}
	if CheckBoundaryFees(base, scope, updated) == nil {
		t.Fatal("replacement IDs bypassed protected boundary fee")
	}
	updated.Routes[0].FeeCents = &oldfee
	if e := CheckBoundaryFees(base, scope, updated); e != nil {
		t.Fatal(e)
	}
}

func TestCanceledMapRequestsDoNotReserveFutureCapacity(t *testing.T) {
	m := &RateMap{Interval: 20 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 10; i++ {
		m.wait(ctx)
	}
	live, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	if e := m.wait(live); e != nil {
		t.Fatal("canceled requests consumed future slots", e)
	}
}
