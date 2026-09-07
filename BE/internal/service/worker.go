package service

import (
	"ai-travel/internal/domain"
	"ai-travel/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"
)

func basicEqual(a, b domain.Activity) bool {
	a.Costs = nil
	a.Place = nil
	b.Costs = nil
	b.Place = nil
	return reflect.DeepEqual(a, b)
}
func Hydrate(c domain.Constraints, p domain.Plan, places []domain.Place, base *domain.Plan) domain.Plan {
	out := p
	out.Activities = append([]domain.Activity(nil), p.Activities...)
	out.Routes = []domain.Route{}
	out.Warnings = []string{"价格及营业时间可能需要确认；地图耗时为查询时估计。", "未定位的餐饮、住宿及每日首尾接驳路线待确认。"}
	known := map[string]domain.Place{}
	for _, x := range places {
		known[x.ID] = x
	}
	old := map[string]domain.Activity{}
	if base != nil {
		for _, a := range base.Activities {
			old[a.ID] = a
		}
	}
	for i, a := range out.Activities {
		if b, ok := old[a.ID]; ok && basicEqual(a, b) {
			out.Activities[i] = b
			continue
		}
		a.Place = nil
		a.Costs = []domain.Cost{}
		switch a.Kind {
		case "sightseeing":
			if p, ok := known[a.PlaceID]; ok {
				copy := p
				a.Place = &copy
				a.Costs = append(a.Costs, domain.Cost{Category: "ticket", AmountCents: p.FeeCents, Unit: "per_person", Source: p.FeeSource, Estimated: true})
			}
		case "meal":
			v := int64(6000)
			a.PlaceID = ""
			a.Costs = append(a.Costs, domain.Cost{Category: "food", AmountCents: &v, Unit: "per_person", Source: "应用估算：每人每餐60元，可作为预算参考", Estimated: true})
		case "hotel":
			v := int64((c.PartySize+1)/2) * 30000
			a.PlaceID = ""
			a.Costs = append(a.Costs, domain.Cost{Category: "hotel", AmountCents: &v, Unit: "group", Source: "应用估算：每两人一间，每间每晚300元", Estimated: true})
		case "rest":
			a.PlaceID = ""
		}
		out.Activities[i] = a
	}
	out.Activities = domain.Sorted(out.Activities)
	return out
}

// RateMap bounds aggregate external map request rate across searches and all workers.
type RateMap struct {
	Inner    MapProvider
	Interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func (m *RateMap) wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		m.mu.Lock()
		now := time.Now()
		if !now.Before(m.next) {
			interval := m.Interval
			if interval <= 0 {
				interval = 500 * time.Millisecond
			}
			m.next = now.Add(interval)
			m.mu.Unlock()
			return nil
		}
		delay := time.Until(m.next)
		m.mu.Unlock()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (m *RateMap) Search(ctx context.Context, c, k string, p, n int) ([]domain.Place, bool, error) {
	if e := m.wait(ctx); e != nil {
		return nil, false, e
	}
	return m.Inner.Search(ctx, c, k, p, n)
}
func (m *RateMap) Route(ctx context.Context, a, b domain.Place, mode string) (domain.Route, error) {
	if e := m.wait(ctx); e != nil {
		return domain.Route{}, e
	}
	return m.Inner.Route(ctx, a, b, mode)
}
func (t *Travel) StartWorkers(ctx context.Context, n int) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(n + 1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			if e := t.Store.Sweep(ctx); e != nil && ctx.Err() == nil {
				slog.Warn("job sweep failed")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					r, e := t.Store.Claim(ctx)
					if e != nil {
						if ctx.Err() == nil {
							slog.Warn("job claim failed")
						}
						continue
					}
					if r != nil {
						t.execute(ctx, *r)
					}
				}
			}
		}()
	}
	return wg
}
func (t *Travel) execute(parent context.Context, r store.Record) {
	ctx, cancel := context.WithDeadline(parent, r.Deadline)
	defer cancel()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		tick := time.NewTicker(10 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if e := t.Store.Renew(ctx, r); e != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-heartbeatDone }()
	defer func() {
		if recover() != nil {
			failctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
			defer stop()
			t.Store.Fail(failctx, r, "failed", domain.Err(500, "INTERNAL_ERROR", "规划执行异常"))
			slog.Error("job panic recovered", "job_id", r.Job.ID)
		}
	}()
	if e := t.process(ctx, r); e != nil {
		failctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		t.Store.Fail(failctx, r, "failed", AsError(e))
		slog.Warn("job failed", "job_id", r.Job.ID, "code", AsError(e).Code)
	}
}
func (t *Travel) process(ctx context.Context, r store.Record) error {
	places := []domain.Place{}
	seen := map[string]bool{}
	calls := 0
	runs := []domain.ModelRun{}
	add := func(ps []domain.Place) {
		for _, p := range ps {
			if !seen[p.ID] && len(places) < 50 {
				seen[p.ID] = true
				places = append(places, p)
			}
		}
	}
	var base *domain.Plan
	if r.Base != nil {
		base = &r.Base.Plan
		for _, a := range base.Activities {
			if a.Place != nil {
				add([]domain.Place{*a.Place})
			}
		}
	}
	stage := func(name string) error { return t.Store.Stage(ctx, r, name, places, runs) }
	if e := stage("searching_places"); e != nil {
		return e
	}
	if r.Job.Kind == "manual_edit" {
		for _, a := range r.Input.Plan.Activities {
			if a.Kind == "sightseeing" && !seen[a.PlaceID] {
				p, e := t.Store.Place(ctx, a.PlaceID)
				if e != nil {
					return e
				}
				add([]domain.Place{p})
			}
		}
	} else {
		queries := []string{"风景名胜", "博物馆"}
		if strings.Contains(r.Input.Instruction, "室内") || strings.Contains(r.Input.Instruction, "博物馆") {
			queries = []string{"博物馆", "风景名胜"}
		}
		for _, q := range queries {
			if len(places) >= 50 {
				break
			}
			calls++
			ps, _, e := t.Map.Search(ctx, r.Input.Constraints.City, q, 1, 25)
			if e != nil {
				return e
			}
			ps, e = t.Store.RememberPlaces(ctx, ps)
			if e != nil {
				return e
			}
			add(ps)
		}
	}
	if len(places) == 0 {
		return domain.Err(422, "INSUFFICIENT_PLACES", "没有查询到可用候选景点")
	}
	if e := stage("generating"); e != nil {
		return e
	}
	req := domain.PlanningRequest{JobID: r.Job.ID, AttemptID: r.AttemptID, Constraints: r.Input.Constraints, Places: places, Base: base, Scope: r.Input.Scope, Instruction: r.Input.Instruction}
	routeCache := map[string]domain.Route{}
	for round := 0; round < 2; round++ {
		var candidate domain.Plan
		if r.Job.Kind == "manual_edit" {
			candidate = *r.Input.Plan
		} else {
			var out domain.PlanningResult
			var e error
			if r.Job.Kind == "replan" {
				out, e = t.Planner.Revise(ctx, req)
			} else {
				out, e = t.Planner.Generate(ctx, req)
			}
			if out.ModelRun != nil {
				runs = append(runs, *out.ModelRun)
			}
			if se := stage("validating"); se != nil {
				return se
			}
			if e != nil {
				return e
			}
			if out.Error != nil {
				return out.Error
			}
			if out.Outcome == "unsatisfied" {
				return domain.Err(422, "CONSTRAINTS_UNSATISFIED", strings.Join(out.Issues, "; "))
			}
			if out.Outcome != "candidate" || out.Plan == nil {
				req.PreviousOutput = out.RawOutput
				req.ValidationIssues = out.Issues
				if round == 0 {
					if e := stage("repairing"); e != nil {
						return e
					}
					continue
				}
				return domain.Err(422, "INVALID_MODEL_OUTPUT", "模型修复后输出仍不符合格式")
			}
			candidate = *out.Plan
			for i := range candidate.Activities {
				candidate.Activities[i].ID = domain.ID()
			}
		}
		if r.Job.Kind == "replan" {
			merged, e := domain.MergeRevision(*base, *r.Input.Scope, candidate.Activities)
			if e != nil {
				req.ValidationIssues = []string{e.Error()}
				raw, _ := json.Marshal(candidate)
				req.PreviousOutput = string(raw)
				if round == 0 {
					continue
				}
				return e
			}
			candidate = merged
		}
		candidate = Hydrate(r.Input.Constraints, candidate, places, base)
		issues := domain.ValidatePlan(r.Input.Constraints, candidate, places)
		if len(issues) == 0 {
			if e := stage("querying_routes"); e != nil {
				return e
			}
			var e error
			candidate, issues, e = t.routes(ctx, r, candidate, routeCache, &calls)
			if e != nil {
				return e
			}
			issues = append(issues, domain.ValidatePlan(r.Input.Constraints, candidate, places)...)
		}
		if len(issues) == 0 && r.Job.Kind == "replan" {
			if e := CheckBoundaryFees(*base, *r.Input.Scope, candidate); e != nil {
				return e
			}
		}
		if len(issues) == 0 {
			if e := stage("saving"); e != nil {
				return e
			}
			return t.Store.Save(ctx, r, candidate)
		}
		if r.Job.Kind == "manual_edit" || round == 1 {
			return domain.Err(422, "PLAN_VALIDATION_FAILED", strings.Join(issues, "; "))
		}
		raw, _ := json.Marshal(candidate)
		req.PreviousOutput = string(raw)
		req.ValidationIssues = issues
		if e := stage("repairing"); e != nil {
			return e
		}
	}
	return domain.Err(422, "PLAN_VALIDATION_FAILED", "没有产生有效行程")
}
func (t *Travel) routes(ctx context.Context, r store.Record, p domain.Plan, cache map[string]domain.Route, calls *int) (domain.Plan, []string, error) {
	issues := []string{}
	var prev *domain.Activity
	oldItems := map[string]domain.Activity{}
	oldRoutes := map[string]domain.Route{}
	if r.Base != nil {
		for _, a := range r.Base.Plan.Activities {
			oldItems[a.ID] = a
		}
		for _, route := range r.Base.Plan.Routes {
			oldRoutes[route.FromItemID+"/"+route.ToItemID] = route
		}
	}
	for _, a := range domain.Sorted(p.Activities) {
		if a.Kind != "sightseeing" || a.Place == nil {
			continue
		}
		if prev == nil || prev.Date != a.Date {
			copy := a
			prev = &copy
			continue
		}
		from := *prev
		key := from.ID + "/" + a.ID
		route, oldExists := oldRoutes[key]
		unchanged := oldExists && domain.Unchanged(from, oldItems[from.ID]) && domain.Unchanged(a, oldItems[a.ID])
		if !unchanged {
			cacheKey := r.Input.Constraints.Transport + "/" + from.Place.Location + "/" + a.Place.Location
			cached, ok := cache[cacheKey]
			if ok {
				route = cached
			} else {
				var e error
				route, e = t.routeWithPreference(ctx, from, a, r.Input.Constraints.Transport, calls)
				if e != nil {
					var api *domain.APIError
					if errors.As(e, &api) && api.Code == "MAP_NO_ROUTE" {
						issues = append(issues, api.Message)
						copy := a
						prev = &copy
						continue
					}
					return p, nil, e
				}
				cache[cacheKey] = route
			}
		}
		route.FromItemID = from.ID
		route.ToItemID = a.ID
		route.Date = a.Date
		if r.Input.Constraints.Transport == "transit" && route.Mode == "walking" {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s：%s 到 %s 公交无方案，短程采用步行（%d 米，%d 秒）。", a.Date, from.Place.Name, a.Place.Name, route.DistanceM, route.DurationS))
		}
		if r.Job.Kind == "replan" && oldExists && !reflect.DeepEqual(oldRoutes[key].FeeCents, route.FeeCents) {
			return p, nil, domain.Err(422, "SCOPE_CONFLICT", "边界交通费用变化，需要扩大修改范围")
		}
		if issue := domain.RouteIssue(p, from, a, route); issue != "" {
			issues = append(issues, issue)
		}
		p.Routes = append(p.Routes, route)
		copy := a
		prev = &copy
	}
	if b := domain.Budget(p, r.Input.Constraints.PartySize); b.UnknownCount > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("有 %d 项费用待确认，预算统计仅包含已知费用。", b.UnknownCount))
	}
	return p, issues, nil
}

// Transit is preferred; only an explicit no-route response permits one bounded
// walking lookup. Both real requests consume the same per-job map budget.
func (t *Travel) routeWithPreference(ctx context.Context, from, to domain.Activity, preference string, calls *int) (domain.Route, error) {
	query := func(mode string) (domain.Route, error) {
		if *calls >= 64 {
			return domain.Route{}, domain.Err(429, "MAP_CALL_LIMIT", "单任务地图调用达到上限")
		}
		*calls++
		return t.Map.Route(ctx, *from.Place, *to.Place, mode)
	}
	noRoute := func(reason string) error {
		return domain.Err(422, "MAP_NO_ROUTE", fmt.Sprintf("%s：%s 到 %s 无可用路线；%s", from.Date, from.Place.Name, to.Place.Name, reason))
	}
	route, err := query(preference)
	if err == nil {
		return route, nil
	}
	var api *domain.APIError
	if !errors.As(err, &api) || api.Code != "MAP_NO_ROUTE" {
		return domain.Route{}, err
	}
	if preference != "transit" {
		return domain.Route{}, noRoute("地图未返回方案")
	}
	route, err = query("walking")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return domain.Route{}, context.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) {
			return domain.Route{}, context.Canceled
		}
		message := fmt.Sprintf("%s：%s 到 %s 公交无方案，步行查询因依赖故障未完成，请稍后重试", from.Date, from.Place.Name, to.Place.Name)
		if errors.As(err, &api) {
			switch api.Code {
			case "MAP_CALL_LIMIT":
				return domain.Route{}, err
			case "MAP_NO_ROUTE":
				return domain.Route{}, noRoute("公交及步行均未返回方案")
			default:
				return domain.Route{}, domain.Err(api.Status, api.Code, message)
			}
		}
		return domain.Route{}, domain.Err(503, "MAP_UNAVAILABLE", message)
	}
	if route.DistanceM < 0 || route.DistanceM > 1500 || route.DurationS < 0 || route.DurationS > 1500 {
		return domain.Route{}, noRoute("公交无方案，步行方案不满足距离至多 1500 米且耗时至多 1500 秒的短程限制")
	}
	return route, nil
}

// CheckBoundaryFees matches boundary legs by their immutable outside endpoint,
// because replacement activities deliberately receive new server-assigned IDs.
func CheckBoundaryFees(base domain.Plan, scope domain.Scope, updated domain.Plan) error {
	editable := map[string]bool{}
	for _, id := range scope.EditableItemIDs {
		editable[id] = true
	}
	protected := map[string]bool{}
	for _, a := range base.Activities {
		if !editable[a.ID] {
			protected[a.ID] = true
		}
	}
	boundary := func(routes []domain.Route) map[string]*int64 {
		m := map[string]*int64{}
		for _, r := range routes {
			from, to := protected[r.FromItemID], protected[r.ToItemID]
			if from && !to {
				m["out:"+r.FromItemID] = r.FeeCents
			}
			if !from && to {
				m["in:"+r.ToItemID] = r.FeeCents
			}
		}
		return m
	}
	before, after := boundary(base.Routes), boundary(updated.Routes)
	if !reflect.DeepEqual(before, after) {
		return domain.Err(422, "SCOPE_CONFLICT", "边界交通费用或衔接变化，需要扩大修改范围")
	}
	return nil
}
