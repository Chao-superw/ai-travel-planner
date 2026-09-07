# HTTP Gateway and Frontend Contract Task

Root `BE/` (relative to the repository root). Implement only `internal/gateway/*.go`, `cmd/api/main.go`, `docs/openapi.yaml`, `docs/frontend-integration.md`, `docs/examples.http`. Do not edit shared domain/rpc/go.mod, secrets, git, or other files. No subagents, commits or deletion. Parent implements service/worker and RPC concurrently. Report docs/gateway-report.md. Use TDD. Authoritative domain types: internal/domain/types.go; backend methods: internal/service/travel.go. Kitex generated types already present; gateway MUST use domain through caller abstraction below, never DB/provider direct.

Exact parent-provided API (will exist shortly):
```
internal/rpc.NewTravelClient(addr string) (*TravelClient,error)
(*TravelClient).Call(context.Context,method string,r domain.Request) (domain.Response,error)
internal/config.Env(key,fallback string) string
```
Your API:
```
type Caller interface { Call(context.Context,string,domain.Request)(domain.Response,error) }
func New(addr,origins string,caller Caller,openapiPath string) *server.Hertz
```
Use Hertz v0.10.6. cmd/api reads API_ADDR (default127.0.0.1:8080), TRAVEL_ADDR(127.0.0.1:8888), CORS_ORIGINS(http://localhost:5173,http://127.0.0.1:5173), OPENAPI_PATH(docs/openapi.yaml). Calls New(...).Spin(). Config does not require keys here.

All public `/api/v1` endpoints and methods per design §5; map below. JSON strictly snake_case, unknown fields and trailing JSON rejected, max1MiB, safe request_id generated domain.ID() never trust unbounded external value. Bearer parsed, authentication forwarded on *every* protected endpoint; backend validates role/ownership. Gateway rejects missing Bearer before calling protected method. Credentials are never logged. HTTP response success `{data: ..., request_id: string}`, error `{code,message,request_id,field_errors?}` (no status member exposed). Separate framework/transport errors sanitized503 or timeout504, don't leak URL. Body/query failures400, jobcreate202, register201, all other successes200 including logout `{logged_out:true}`. Include job status_url when task accepted. Each API deadline5s except SearchPlaces/CreatePlace/UpdatePlace8s. CORS only configured exact origins (no wildcard default), Authorization/Content-Type/Idempotency-Key allowed, OPTIONS204 permitted only for configured origins. GET /healthz process liveness and GET /readyz call Travel Health; GET /openapi.yaml serve document with yaml content type; missing file404. Don't turn API calls into an HTTP file server.

Routes and request mapping:
POST /api/v1/auth/register -> Register body {username,password}; data User.
POST /api/v1/auth/login -> Login samebody; data Session.
POST /api/v1/auth/logout -> Logout no body; data logged_out.
GET /api/v1/me -> GetCurrentUser; data User.
GET /api/v1/places?city=杭州市&keyword=博物馆&page=1&page_size=20 -> SearchPlaces. Validate integer parsing, no negative/overflow (backend0 meansdefault but explicit0 queryreject), max25; data {items:Places,page,page_size,has_more}.
POST /api/v1/admin/places -> CreatePlace body domain.Place subset {id,duration_minutes,tags,open_minute?,close_minute?,fee_cents?,fee_source,active}. p.ID must come from prior search. Backend verifies identity via saved search. Return single place object (Response.Places[0]).
PATCH /api/v1/admin/places/:id -> UpdatePlace body same editable subset +version, pathauthoritative. ALL editable fields treated replacement; document this, unknown fee/window represented absent/null. Supplied provider identity/name/coords must be rejected in body (backend ignores anyway).
POST /api/v1/planning-jobs -> CreatePlanningJob body {constraints:domain.Constraints}. Recommended full example city杭州市,start_date2026-10-01,end_date2026-10-03,party_size2,budget_cents200000,budget_scopeper_person,interests文化/美食,pacebalanced,transporttransit. budget_total_cents server-owned reject external. Require Idempotency-Key1-128bytes.
GET /api/v1/planning-jobs/:id -> GetPlanningJob data Job.
POST /api/v1/planning-jobs/:id/retries -> RetryPlanningJob no body +Idempotency-Key dataJob202.
GET /api/v1/trips -> ListTrips pagination default20max100; data list summaries not full activities.
GET /api/v1/trips/:id -> GetTrip data Trip (constraints+plan with activities/routes).
PATCH /api/v1/trips/:id -> UpdateTrip body {expected_version,plan} +Idempotency-Key ->Job202. Backend ignores/recomputes costs/place snapshots/routes for changed activities, preserves unchanged. Use narrow input Activity id,date,start_minute,end_minute,kind,place_id,title,reason; backend-owned fields in plan excluded from input; read-only response snapshots must be omitted when sending updates. manual_edit no model.
POST /api/v1/trips/:id/replanning-jobs -> CreateReplanningJob body {expected_version,scope:{date,start_minute,end_minute,editable_item_ids,locked_item_ids},instruction} +Idempotency-Key -> Job202.
GET /api/v1/trips/:id/versions -> ListTripVersions paginated Trip summaries.
GET /api/v1/trips/:id/versions/:version -> GetTripVersion positive parsedint64, dataTrip.
GET /api/v1/trips/:id/budget -> GetBudgetSummary, dataBudgetSummary. Unknown price absent/null (omitempty); completefalse=>within_budget absent; knownTotal counts only known cents, per-person multiplied byparty exactlyonce; by_day/by_category charts frombackend.

Provide comprehensive valid OpenAPI3 yaml with all endpoints, schemas, request restrictions (additionalProperties false input), auth, headers, exact envelope, errors,status enums queued/running/succeeded/failed/conflicted/interrupted, pagination, post202. Documentation Chinese includes minimal startup integration URL, login bearer, creation/polling2s/finalstates, job success thenGETtrip, version conflictrefetch, scopeexamples(actualselectedIDs not inventedwork), budgetunits&unknown, GCJ02 lonlat, maproutequerytime notfutureguarantee, response shapes, HTTP examples. Never claim real tests passing without running. All IDs examples abstract strings, no actualcredentials. Tests use fake Caller onlyatRPC boundary to observe real gateway validation/mapping/CORS/error behavior; test cancellation budgetsifpractical. Run go test ./internal/gateway with GOCACHE=/tmp/ai-travel-build GOMODCACHE=/tmp/ai-travel-mod; don't mutate go.mod ifmissingdeps ask parent.
