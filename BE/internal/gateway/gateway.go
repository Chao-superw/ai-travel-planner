package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"ai-travel/internal/domain"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

const maxBodyBytes = 1 << 20

type Caller interface {
	Call(context.Context, string, domain.Request) (domain.Response, error)
}

type handler struct{ caller Caller }

type placeInput struct {
	DurationMinutes int32    `json:"duration_minutes"`
	Tags            []string `json:"tags"`
	OpenMinute      *int32   `json:"open_minute"`
	CloseMinute     *int32   `json:"close_minute"`
	FeeCents        *int64   `json:"fee_cents"`
	FeeSource       string   `json:"fee_source"`
	Active          bool     `json:"active"`
}

type createPlaceInput struct {
	ID string `json:"id"`
	placeInput
}

type updatePlaceInput struct {
	Version int64 `json:"version"`
	placeInput
}

type constraintsInput struct {
	City        string   `json:"city"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	PartySize   int32    `json:"party_size"`
	BudgetCents int64    `json:"budget_cents"`
	BudgetScope string   `json:"budget_scope"`
	Interests   []string `json:"interests"`
	Pace        string   `json:"pace"`
	Transport   string   `json:"transport"`
}

func (v constraintsInput) domain() domain.Constraints {
	return domain.Constraints{City: v.City, StartDate: v.StartDate, EndDate: v.EndDate, PartySize: v.PartySize, BudgetCents: v.BudgetCents, BudgetScope: v.BudgetScope, Interests: v.Interests, Pace: v.Pace, Transport: v.Transport}
}

type activityInput struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	StartMinute int32  `json:"start_minute"`
	EndMinute   int32  `json:"end_minute"`
	Kind        string `json:"kind"`
	PlaceID     string `json:"place_id"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
}

type planInput struct {
	Title      string          `json:"title"`
	Summary    string          `json:"summary"`
	Activities []activityInput `json:"activities"`
}

func (v planInput) domain() domain.Plan {
	p := domain.Plan{Title: v.Title, Summary: v.Summary, Activities: make([]domain.Activity, len(v.Activities))}
	for i, a := range v.Activities {
		p.Activities[i] = domain.Activity{ID: a.ID, Date: a.Date, StartMinute: a.StartMinute, EndMinute: a.EndMinute, Kind: a.Kind, PlaceID: a.PlaceID, Title: a.Title, Reason: a.Reason}
	}
	return p
}

type scopeInput struct {
	Date            string   `json:"date"`
	StartMinute     int32    `json:"start_minute"`
	EndMinute       int32    `json:"end_minute"`
	EditableItemIDs []string `json:"editable_item_ids"`
	LockedItemIDs   []string `json:"locked_item_ids"`
}

func (v scopeInput) domain() domain.Scope {
	return domain.Scope{Date: v.Date, StartMinute: v.StartMinute, EndMinute: v.EndMinute, EditableItemIDs: v.EditableItemIDs, LockedItemIDs: v.LockedItemIDs}
}

func New(addr, origins string, caller Caller, openapiPath string) *server.Hertz {
	h := server.Default(server.WithHostPorts(addr))
	g := &handler{caller: caller}
	allowed := parseOrigins(origins)
	h.Use(authNoStore, cors(allowed))

	h.GET("/healthz", func(_ context.Context, c *app.RequestContext) { c.JSON(consts.StatusOK, utils.H{"status": "ok"}) })
	h.GET("/readyz", g.ready)
	h.GET("/openapi.yaml", serveOpenAPI(openapiPath))

	api := h.Group("/api/v1")
	api.POST("/auth/register", g.register)
	api.POST("/auth/login", g.login)
	api.POST("/auth/register/code", g.sendRegistrationCode)
	api.POST("/auth/legacy/login", g.legacyLogin)
	api.POST("/me/email/code", g.protected("RequestEmailBindingCode", 15*time.Second, g.sendBindingCode))
	api.POST("/me/email", g.protected("BindEmail", 5*time.Second, g.bindEmail))
	api.POST("/auth/logout", g.protected("Logout", 5*time.Second, g.logout))
	api.GET("/me", g.protected("GetCurrentUser", 5*time.Second, g.currentUser))
	api.GET("/places", g.protected("SearchPlaces", 8*time.Second, g.searchPlaces))
	api.POST("/admin/places", g.protected("CreatePlace", 8*time.Second, g.createPlace))
	api.PATCH("/admin/places/:id", g.protected("UpdatePlace", 8*time.Second, g.updatePlace))
	api.POST("/planning-jobs", g.protected("CreatePlanningJob", 5*time.Second, g.createPlanningJob))
	api.GET("/planning-jobs/:id", g.protected("GetPlanningJob", 5*time.Second, g.getJob))
	api.POST("/planning-jobs/:id/retries", g.protected("RetryPlanningJob", 5*time.Second, g.retryJob))
	api.GET("/trips", g.protected("ListTrips", 5*time.Second, g.listTrips))
	api.GET("/trips/:id", g.protected("GetTrip", 5*time.Second, g.getTrip))
	api.PATCH("/trips/:id", g.protected("UpdateTrip", 5*time.Second, g.updateTrip))
	api.POST("/trips/:id/replanning-jobs", g.protected("CreateReplanningJob", 5*time.Second, g.replan))
	api.GET("/trips/:id/versions", g.protected("ListTripVersions", 5*time.Second, g.listVersions))
	api.GET("/trips/:id/versions/:version", g.protected("GetTripVersion", 5*time.Second, g.getVersion))
	api.GET("/trips/:id/budget", g.protected("GetBudgetSummary", 5*time.Second, g.getBudget))
	return h
}

type endpoint func(context.Context, *app.RequestContext, string, string)

func authNoStore(ctx context.Context, c *app.RequestContext) {
	path := string(c.Path())
	if strings.HasPrefix(path, "/api/v1/auth/") || path == "/api/v1/me" || strings.HasPrefix(path, "/api/v1/me/") {
		c.Header("Cache-Control", "no-store")
	}
	c.Next(ctx)
}

func (g *handler) protected(method string, timeout time.Duration, next endpoint) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := domain.ID()
		token, ok := bearer(string(c.Request.Header.Peek("Authorization")))
		if !ok {
			writeError(c, requestID, domain.Err(401, "UNAUTHORIZED", "请提供有效的 Bearer 凭据"))
			return
		}
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		next(callCtx, c, requestID, token)
	}
}

func bearer(value string) (string, bool) {
	parts := strings.Fields(value)
	return func() (string, bool) {
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && parts[1] != "" {
			return parts[1], true
		}
		return "", false
	}()
}

func (g *handler) invoke(ctx context.Context, c *app.RequestContext, method, requestID string, req domain.Request, status int, data func(domain.Response) any) {
	req.RequestID = requestID
	res, err := g.caller.Call(ctx, method, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) || kerrors.IsTimeoutError(err) {
			writeError(c, requestID, domain.Err(504, "TIMEOUT", "操作超时"))
			return
		}
		writeError(c, requestID, domain.Err(503, "DEPENDENCY_UNAVAILABLE", "服务暂时不可用"))
		return
	}
	if res.Error != nil {
		writeError(c, requestID, res.Error)
		return
	}
	c.JSON(status, utils.H{"data": data(res), "request_id": requestID})
}

func (g *handler) logout(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.invoke(ctx, c, "Logout", rid, domain.Request{Token: token}, 200, func(domain.Response) any { return utils.H{"logged_out": true} })
}
func (g *handler) currentUser(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.invoke(ctx, c, "GetCurrentUser", rid, domain.Request{Token: token}, 200, func(r domain.Response) any { return r.User })
}

func (g *handler) searchPlaces(ctx context.Context, c *app.RequestContext, rid, token string) {
	p, s, err := pagination(c, 25)
	if err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	req := domain.Request{Token: token, City: c.Query("city"), Keyword: c.Query("keyword"), Page: p, PageSize: s}
	g.invoke(ctx, c, "SearchPlaces", rid, req, 200, func(r domain.Response) any {
		return utils.H{"items": r.Places, "page": r.Page, "page_size": r.PageSize, "has_more": r.HasMore}
	})
}

func (g *handler) createPlace(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in createPlaceInput
	if err := decode(c, &in); err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	g.savePlace(ctx, c, rid, token, "CreatePlace", in.ID, in.placeInput, 0)
}
func (g *handler) updatePlace(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in updatePlaceInput
	if err := decode(c, &in); err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	g.savePlace(ctx, c, rid, token, "UpdatePlace", c.Param("id"), in.placeInput, in.Version)
}
func (g *handler) savePlace(ctx context.Context, c *app.RequestContext, rid, token, method, id string, in placeInput, version int64) {
	p := &domain.Place{ID: id, DurationMinutes: in.DurationMinutes, Tags: in.Tags, OpenMinute: in.OpenMinute, CloseMinute: in.CloseMinute, FeeCents: in.FeeCents, FeeSource: in.FeeSource, Active: in.Active, Version: version}
	g.invoke(ctx, c, method, rid, domain.Request{Token: token, ID: id, Place: p}, 200, func(r domain.Response) any {
		if len(r.Places) == 0 {
			return nil
		}
		return r.Places[0]
	})
}

func (g *handler) createPlanningJob(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in struct {
		Constraints constraintsInput `json:"constraints"`
	}
	if err := decode(c, &in); err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	g.job(ctx, c, rid, "CreatePlanningJob", domain.Request{Token: token, IdempotencyKey: string(c.Request.Header.Peek("Idempotency-Key")), Input: &domain.JobInput{Constraints: in.Constraints.domain()}})
}
func (g *handler) getJob(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.invoke(ctx, c, "GetPlanningJob", rid, domain.Request{Token: token, ID: c.Param("id")}, 200, func(r domain.Response) any { return r.Job })
}
func (g *handler) retryJob(ctx context.Context, c *app.RequestContext, rid, token string) {
	if err := noBody(c); err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	g.job(ctx, c, rid, "RetryPlanningJob", domain.Request{Token: token, ID: c.Param("id"), IdempotencyKey: string(c.Request.Header.Peek("Idempotency-Key"))})
}
func (g *handler) job(ctx context.Context, c *app.RequestContext, rid, method string, req domain.Request) {
	if len(req.IdempotencyKey) < 1 || len(req.IdempotencyKey) > 128 {
		writeError(c, rid, domain.Err(400, "IDEMPOTENCY_KEY_REQUIRED", "请提供 1 至 128 字节 Idempotency-Key"))
		return
	}
	g.invoke(ctx, c, method, rid, req, 202, func(r domain.Response) any { return jobData(r.Job) })
}
func jobData(job *domain.Job) any {
	if job == nil {
		return nil
	}
	type accepted struct {
		*domain.Job
		StatusURL string `json:"status_url"`
	}
	return accepted{Job: job, StatusURL: "/api/v1/planning-jobs/" + job.ID}
}

func (g *handler) listTrips(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.list(ctx, c, rid, token, "ListTrips", "")
}
func (g *handler) listVersions(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.list(ctx, c, rid, token, "ListTripVersions", c.Param("id"))
}
func (g *handler) list(ctx context.Context, c *app.RequestContext, rid, token, method, id string) {
	p, s, err := pagination(c, 100)
	if err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	g.invoke(ctx, c, method, rid, domain.Request{Token: token, ID: id, Page: p, PageSize: s}, 200, func(r domain.Response) any {
		items := make([]tripSummary, len(r.Trips))
		for i := range r.Trips {
			items[i] = summarize(r.Trips[i])
		}
		return utils.H{"items": items, "page": r.Page, "page_size": r.PageSize, "has_more": r.HasMore}
	})
}

type tripSummary struct {
	ID          string             `json:"id"`
	Version     int64              `json:"version"`
	Constraints domain.Constraints `json:"constraints"`
	Title       string             `json:"title"`
	Summary     string             `json:"summary"`
	CreatedAt   string             `json:"created_at"`
}

func summarize(t domain.Trip) tripSummary {
	return tripSummary{ID: t.ID, Version: t.Version, Constraints: t.Constraints, Title: t.Plan.Title, Summary: t.Plan.Summary, CreatedAt: t.CreatedAt}
}
func (g *handler) getTrip(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.invoke(ctx, c, "GetTrip", rid, domain.Request{Token: token, ID: c.Param("id")}, 200, func(r domain.Response) any { return r.Trip })
}
func (g *handler) getBudget(ctx context.Context, c *app.RequestContext, rid, token string) {
	g.invoke(ctx, c, "GetBudgetSummary", rid, domain.Request{Token: token, ID: c.Param("id")}, 200, func(r domain.Response) any { return r.Budget })
}
func (g *handler) getVersion(ctx context.Context, c *app.RequestContext, rid, token string) {
	v, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil || v < 1 {
		writeBadRequest(c, rid, errors.New("invalid version"))
		return
	}
	g.invoke(ctx, c, "GetTripVersion", rid, domain.Request{Token: token, ID: c.Param("id"), Version: v}, 200, func(r domain.Response) any { return r.Trip })
}

func (g *handler) updateTrip(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in struct {
		ExpectedVersion int64     `json:"expected_version"`
		Plan            planInput `json:"plan"`
	}
	if err := decode(c, &in); err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	p := in.Plan.domain()
	g.job(ctx, c, rid, "UpdateTrip", domain.Request{Token: token, ID: c.Param("id"), IdempotencyKey: string(c.Request.Header.Peek("Idempotency-Key")), Input: &domain.JobInput{ExpectedVersion: in.ExpectedVersion, Plan: &p}})
}
func (g *handler) replan(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in struct {
		ExpectedVersion int64      `json:"expected_version"`
		Scope           scopeInput `json:"scope"`
		Instruction     string     `json:"instruction"`
	}
	if err := decode(c, &in); err != nil {
		writeBadRequest(c, rid, err)
		return
	}
	scope := in.Scope.domain()
	g.job(ctx, c, rid, "CreateReplanningJob", domain.Request{Token: token, ID: c.Param("id"), IdempotencyKey: string(c.Request.Header.Peek("Idempotency-Key")), Input: &domain.JobInput{ExpectedVersion: in.ExpectedVersion, Scope: &scope, Instruction: in.Instruction}})
}

func (g *handler) ready(ctx context.Context, c *app.RequestContext) {
	rid := domain.ID()
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	g.invoke(callCtx, c, "Health", rid, domain.Request{}, 200, func(domain.Response) any { return utils.H{"status": "ready"} })
}
func serveOpenAPI(path string) app.HandlerFunc {
	return func(_ context.Context, c *app.RequestContext) {
		if path == "" {
			c.Status(404)
			return
		}
		b, err := os.ReadFile(path)
		if err != nil {
			c.Status(404)
			return
		}
		c.Data(200, "application/yaml; charset=utf-8", b)
	}
}

func decode(c *app.RequestContext, dst any) error {
	b := c.Request.Body()
	if len(b) > maxBodyBytes {
		return errors.New("body too large")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}
func noBody(c *app.RequestContext) error {
	if len(bytes.TrimSpace(c.Request.Body())) != 0 {
		return errors.New("body must be empty")
	}
	return nil
}
func pagination(c *app.RequestContext, max int32) (int32, int32, error) {
	p, err := queryInt(c, "page", 1)
	if err != nil {
		return 0, 0, err
	}
	s, err := queryInt(c, "page_size", 20)
	if err != nil {
		return 0, 0, err
	}
	if p < 1 || p > 10000 || s < 1 || s > max {
		return 0, 0, errors.New("pagination out of range")
	}
	return p, s, nil
}
func queryInt(c *app.RequestContext, name string, fallback int32) (int32, error) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	return int32(n), err
}
func writeBadRequest(c *app.RequestContext, rid string, _ error) {
	writeError(c, rid, domain.Err(400, "INVALID_INPUT", "请求参数无效"))
}
func writeError(c *app.RequestContext, rid string, e *domain.APIError) {
	status := int(e.Status)
	if status < 400 || status > 599 {
		status = 500
	}
	payload := utils.H{"code": e.Code, "message": e.Message, "request_id": rid}
	if e.RetryAfter != nil {
		payload["retry_after"] = *e.RetryAfter
		c.Header("Retry-After", strconv.Itoa(int(*e.RetryAfter)))
	}
	if e.FieldErrors != nil {
		payload["field_errors"] = e.FieldErrors
	}
	c.JSON(status, payload)
}

func parseOrigins(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" && v != "*" {
			out[v] = struct{}{}
		}
	}
	return out
}
func cors(allowed map[string]struct{}) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.Request.Header.Peek("Origin"))
		_, ok := allowed[origin]
		if ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			c.Header("Access-Control-Expose-Headers", "Retry-After")
		}
		if string(c.Method()) == "OPTIONS" {
			if ok {
				c.Status(204)
			} else {
				c.Status(403)
			}
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}
