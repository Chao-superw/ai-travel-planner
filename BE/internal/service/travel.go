package service

import (
	"ai-travel/internal/authn"
	"ai-travel/internal/domain"
	"ai-travel/internal/store"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

type MapProvider interface {
	Search(context.Context, string, string, int, int) ([]domain.Place, bool, error)
	Route(context.Context, domain.Place, domain.Place, string) (domain.Route, error)
}
type Planner interface {
	Generate(context.Context, domain.PlanningRequest) (domain.PlanningResult, error)
	Revise(context.Context, domain.PlanningRequest) (domain.PlanningResult, error)
}
type Travel struct {
	Store      *store.Store
	Map        MapProvider
	Planner    Planner
	Mailer     MailSender
	AuthPolicy authn.Policy
}

func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func AsError(e error) *domain.APIError {
	var a *domain.APIError
	if errors.As(e, &a) {
		return a
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return domain.Err(504, "TIMEOUT", "操作超时")
	}
	if errors.Is(e, context.Canceled) {
		return domain.Err(503, "CANCELED", "操作已取消")
	}
	return domain.Err(500, "INTERNAL_ERROR", "内部处理失败")
}

func (t *Travel) Handle(ctx context.Context, method string, r domain.Request) domain.Response {
	res, e := t.handle(ctx, method, r)
	if e != nil {
		res.Error = AsError(e)
	}
	return res
}
func (t *Travel) handle(ctx context.Context, method string, r domain.Request) (res domain.Response, e error) {
	if method == "Health" {
		return res, t.Store.Pool.Ping(ctx)
	}
	switch method {
	case "RequestRegistrationCode":
		return t.issueEmailCode(ctx, r, "register", "")
	case "Register":
		return t.registerEmail(ctx, r)
	case "Login":
		return t.loginEmail(ctx, r, false)
	case "LegacyLogin":
		return t.loginEmail(ctx, r, true)
	}

	u, e := t.Store.Auth(ctx, Hash(r.Token))
	if e != nil {
		return res, e
	}
	switch method {
	case "RequestEmailBindingCode":
		if u.Email != "" || u.EmailVerified {
			return res, domain.Err(409, "EMAIL_ALREADY_BOUND", "当前账号已绑定邮箱")
		}
		return t.issueEmailCode(ctx, r, "bind", u.ID)
	case "BindEmail":
		return t.bindEmail(ctx, r, u)
	}
	if !u.EmailVerified && method != "GetCurrentUser" && method != "Logout" {
		return res, domain.Err(403, "EMAIL_VERIFICATION_REQUIRED", "请先验证并绑定邮箱")
	}

	switch method {
	case "Logout":
		e = t.Store.Revoke(ctx, Hash(r.Token))
	case "GetCurrentUser":
		res.User = &u
	case "SearchPlaces":
		if strings.TrimSpace(r.City) == "" || len([]rune(r.City)) > 40 || len([]rune(r.Keyword)) > 80 {
			return res, domain.Err(400, "INVALID_INPUT", "请输入有效城市和关键词")
		}
		page, size, e := pagination(r, 25)
		if e != nil {
			return res, e
		}
		ps, more, e := t.Map.Search(ctx, r.City, r.Keyword, page, size)
		if e != nil {
			return res, e
		}
		ps, e = t.Store.RememberPlaces(ctx, ps)
		res.Places = ps
		res.Page = int32(page)
		res.PageSize = int32(size)
		res.HasMore = more
		return res, e
	case "CreatePlace", "UpdatePlace":
		if u.Role != "admin" {
			return res, domain.Err(403, "FORBIDDEN", "仅管理员可维护景点资料")
		}
		if r.Place == nil {
			return res, domain.Err(400, "INVALID_INPUT", "缺少景点资料")
		}
		p := *r.Place
		if method == "UpdatePlace" {
			p.ID = r.ID
		}
		old, e := t.Store.Place(ctx, p.ID)
		if e != nil {
			return res, e
		}
		if p.DurationMinutes < 15 || p.DurationMinutes > 480 {
			return res, domain.Err(400, "INVALID_INPUT", "游玩时长应为 15 至 480 分钟")
		}
		if (p.OpenMinute == nil) != (p.CloseMinute == nil) || (p.OpenMinute != nil && (*p.OpenMinute < 0 || *p.CloseMinute > 1440 || *p.OpenMinute >= *p.CloseMinute)) {
			return res, domain.Err(400, "INVALID_INPUT", "营业窗口必须完整且有效")
		}
		if len(p.Tags) > 10 || len(p.FeeSource) > 500 || (p.FeeCents != nil && (*p.FeeCents < 0 || *p.FeeCents > 100000000 || p.FeeSource == "")) {
			return res, domain.Err(400, "INVALID_INPUT", "费用须非负并提供来源；标签最多 10 个")
		}
		old.Active = p.Active
		old.DurationMinutes = p.DurationMinutes
		old.Tags = p.Tags
		old.OpenMinute = p.OpenMinute
		old.CloseMinute = p.CloseMinute
		old.FeeCents = p.FeeCents
		old.FeeSource = p.FeeSource
		if method == "UpdatePlace" {
			old.Version = p.Version
		}
		saved, e := t.Store.Curate(ctx, old, method == "CreatePlace")
		res.Places = []domain.Place{saved}
		return res, e
	case "CreatePlanningJob", "CreateReplanningJob", "UpdateTrip":
		j, e := t.submit(ctx, u.ID, method, r, "")
		res.Job = &j
		return res, e
	case "GetPlanningJob":
		j, e := t.Store.GetJob(ctx, u.ID, r.ID)
		res.Job = &j
		return res, e
	case "RetryPlanningJob":
		j, e := t.Store.GetJob(ctx, u.ID, r.ID)
		if e != nil {
			return res, e
		}
		if j.Status != "failed" && j.Status != "interrupted" {
			return res, domain.Err(409, "JOB_NOT_RETRYABLE", "仅失败或中断任务可以重试；版本冲突需重新提交")
		}
		old, e := t.Store.Record(ctx, j.ID)
		if e != nil {
			return res, e
		}
		req := r
		req.Input = &old.Input
		req.ID = j.TripID
		m := map[string]string{"generate": "CreatePlanningJob", "replan": "CreateReplanningJob", "manual_edit": "UpdateTrip"}[j.Kind]
		retry, e := t.submit(ctx, u.ID, m, req, j.ID)
		res.Job = &retry
		return res, e
	case "GetTrip", "GetTripVersion", "GetBudgetSummary":
		version := int64(0)
		if method == "GetTripVersion" {
			version = r.Version
			if version < 1 {
				return res, domain.Err(400, "INVALID_INPUT", "版本号须为正整数")
			}
		}
		trip, e := t.Store.GetTrip(ctx, u.ID, r.ID, version)
		if e != nil {
			return res, e
		}
		if method == "GetBudgetSummary" {
			b := domain.Budget(trip.Plan, trip.Constraints.PartySize)
			b.TripID = trip.ID
			b.Version = trip.Version
			b.BudgetTotalCents = trip.Constraints.BudgetTotalCents
			if b.Complete {
				ok := b.KnownTotalCents <= b.BudgetTotalCents
				b.WithinBudget = &ok
			}
			res.Budget = &b
		} else {
			res.Trip = &trip
		}
	case "ListTrips", "ListTripVersions":
		page, size, e := pagination(r, 100)
		if e != nil {
			return res, e
		}
		id := ""
		if method == "ListTripVersions" {
			id = r.ID
		}
		res.Trips, res.HasMore, e = t.Store.ListTrips(ctx, u.ID, id, page, size)
		res.Page = int32(page)
		res.PageSize = int32(size)
		return res, e
	default:
		return res, domain.Err(404, "NOT_FOUND", "接口不存在")
	}
	return res, e
}
func pagination(r domain.Request, max int) (int, int, error) {
	p, s := int(r.Page), int(r.PageSize)
	if p == 0 {
		p = 1
	}
	if s == 0 {
		s = 20
	}
	if p < 1 || p > 10000 || s < 1 || s > max {
		return 0, 0, domain.Err(400, "INVALID_INPUT", "分页参数超出范围")
	}
	return p, s, nil
}
func (t *Travel) submit(ctx context.Context, owner, method string, r domain.Request, parent string) (domain.Job, error) {
	if len(r.IdempotencyKey) < 1 || len(r.IdempotencyKey) > 128 {
		return domain.Job{}, domain.Err(400, "IDEMPOTENCY_KEY_REQUIRED", "任务提交须提供 1 至 128 字节 Idempotency-Key")
	}
	if r.Input == nil {
		return domain.Job{}, domain.Err(400, "INVALID_INPUT", "缺少任务参数")
	}
	raw, _ := json.Marshal(r.Input)
	hash := Hash(string(raw))
	kind := map[string]string{"CreatePlanningJob": "generate", "CreateReplanningJob": "replan", "UpdateTrip": "manual_edit"}[method]
	op := kind + ":" + r.ID
	if parent != "" {
		op += "/retry/" + parent
	}
	if existing, e := t.Store.ExistingJob(ctx, owner, op, r.IdempotencyKey, hash); e != nil {
		return domain.Job{}, e
	} else if existing != nil {
		return *existing, nil
	}
	in := *r.Input
	var base *domain.Trip
	if kind == "generate" {
		if e := domain.ValidateConstraints(&in.Constraints); e != nil {
			return domain.Job{}, e
		}
		if in.Plan != nil || in.Scope != nil || in.ExpectedVersion != 0 {
			return domain.Job{}, domain.Err(400, "INVALID_INPUT", "创建任务不能包含已有行程或修改范围")
		}
	} else {
		trip, e := t.Store.GetTrip(ctx, owner, r.ID, 0)
		if e != nil {
			return domain.Job{}, e
		}
		if in.ExpectedVersion != trip.Version {
			return domain.Job{}, domain.Err(409, "VERSION_CONFLICT", "请携带当前行程版本")
		}
		base = &trip
		in.Constraints = trip.Constraints
		if kind == "replan" {
			if in.Scope == nil || len([]rune(in.Instruction)) < 1 || len([]rune(in.Instruction)) > 2000 {
				return domain.Job{}, domain.Err(400, "INVALID_SCOPE", "选择修改范围并填写 1 至 2000 字修改要求")
			}
			if e := domain.ValidateScope(trip.Plan, *in.Scope); e != nil {
				return domain.Job{}, e
			}
		} else if in.Plan == nil {
			return domain.Job{}, domain.Err(400, "INVALID_INPUT", "手动修改须提交完整 plan")
		}
	}
	return t.Store.CreateJob(ctx, owner, kind, op, r.IdempotencyKey, hash, in, base, parent)
}
