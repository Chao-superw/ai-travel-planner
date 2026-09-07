package domain

type Constraints struct {
	City             string   `json:"city"`
	StartDate        string   `json:"start_date"`
	EndDate          string   `json:"end_date"`
	PartySize        int32    `json:"party_size"`
	BudgetCents      int64    `json:"budget_cents"`
	BudgetScope      string   `json:"budget_scope"`
	BudgetTotalCents int64    `json:"budget_total_cents"`
	Interests        []string `json:"interests"`
	Pace             string   `json:"pace"`
	Transport        string   `json:"transport"`
}
type Place struct {
	ID               string   `json:"id"`
	Provider         string   `json:"provider"`
	ProviderPOIID    string   `json:"provider_poi_id"`
	Name             string   `json:"name"`
	City             string   `json:"city"`
	CityCode         string   `json:"city_code"`
	AdCode           string   `json:"ad_code"`
	Location         string   `json:"location"`
	CoordinateSystem string   `json:"coordinate_system"`
	Address          string   `json:"address"`
	TypeCode         string   `json:"type_code"`
	Tags             []string `json:"tags"`
	DurationMinutes  int32    `json:"duration_minutes"`
	OpenMinute       *int32   `json:"open_minute,omitempty"`
	CloseMinute      *int32   `json:"close_minute,omitempty"`
	FeeCents         *int64   `json:"fee_cents,omitempty"`
	FeeSource        string   `json:"fee_source"`
	Active           bool     `json:"active"`
	FetchedAt        string   `json:"fetched_at"`
	Version          int64    `json:"version"`
}
type Cost struct {
	Category    string `json:"category"`
	AmountCents *int64 `json:"amount_cents,omitempty"`
	Unit        string `json:"unit"`
	Source      string `json:"source"`
	Estimated   bool   `json:"estimated"`
}
type Activity struct {
	ID          string `json:"id"`
	Date        string `json:"date"`
	StartMinute int32  `json:"start_minute"`
	EndMinute   int32  `json:"end_minute"`
	Kind        string `json:"kind"`
	PlaceID     string `json:"place_id"`
	Title       string `json:"title"`
	Reason      string `json:"reason"`
	Place       *Place `json:"place,omitempty"`
	Costs       []Cost `json:"costs"`
}
type Route struct {
	FromItemID   string `json:"from_item_id"`
	ToItemID     string `json:"to_item_id"`
	Date         string `json:"date"`
	Mode         string `json:"mode"`
	DistanceM    int64  `json:"distance_m"`
	DurationS    int64  `json:"duration_s"`
	FeeCents     *int64 `json:"fee_cents,omitempty"`
	Provider     string `json:"provider"`
	QueriedAt    string `json:"queried_at"`
	Summary      string `json:"summary"`
	Polyline     string `json:"polyline"`
	FromLocation string `json:"from_location"`
	ToLocation   string `json:"to_location"`
}
type Plan struct {
	Title      string     `json:"title"`
	Summary    string     `json:"summary"`
	Activities []Activity `json:"activities"`
	Routes     []Route    `json:"routes"`
	Warnings   []string   `json:"warnings"`
}
type Scope struct {
	Date            string   `json:"date"`
	StartMinute     int32    `json:"start_minute"`
	EndMinute       int32    `json:"end_minute"`
	EditableItemIDs []string `json:"editable_item_ids"`
	LockedItemIDs   []string `json:"locked_item_ids"`
}
type JobInput struct {
	Constraints     Constraints `json:"constraints"`
	ExpectedVersion int64       `json:"expected_version"`
	Scope           *Scope      `json:"scope,omitempty"`
	Instruction     string      `json:"instruction"`
	Plan            *Plan       `json:"plan,omitempty"`
}
type APIError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Status      int32             `json:"status"`
	FieldErrors map[string]string `json:"field_errors,omitempty"`
	RetryAfter  *int32            `json:"retry_after,omitempty"`
}
type ModelRun struct {
	ProviderRequestID string `json:"provider_request_id"`
	ReturnedModel     string `json:"returned_model"`
	PromptVersion     string `json:"prompt_version"`
	DurationMS        int64  `json:"duration_ms"`
	PromptTokens      *int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens  *int64 `json:"completion_tokens,omitempty"`
	FinishReason      string `json:"finish_reason"`
}
type PlanningRequest struct {
	ServiceToken     string      `json:"service_token"`
	JobID            string      `json:"job_id"`
	AttemptID        string      `json:"attempt_id"`
	Constraints      Constraints `json:"constraints"`
	Places           []Place     `json:"places"`
	Base             *Plan       `json:"base,omitempty"`
	Scope            *Scope      `json:"scope,omitempty"`
	Instruction      string      `json:"instruction"`
	PreviousOutput   string      `json:"previous_output"`
	ValidationIssues []string    `json:"validation_issues"`
}
type PlanningResult struct {
	Outcome   string    `json:"outcome"`
	Plan      *Plan     `json:"plan,omitempty"`
	Issues    []string  `json:"issues"`
	RawOutput string    `json:"raw_output"`
	ModelRun  *ModelRun `json:"model_run,omitempty"`
	Error     *APIError `json:"error,omitempty"`
}
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}
type Session struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	User      User   `json:"user"`
}
type Job struct {
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	Stage         string     `json:"stage"`
	TripID        string     `json:"trip_id"`
	ResultVersion int64      `json:"result_version"`
	ParentJobID   string     `json:"parent_job_id"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
	Error         *APIError  `json:"error,omitempty"`
	ModelRuns     []ModelRun `json:"model_runs"`
}
type Trip struct {
	ID          string      `json:"id"`
	Version     int64       `json:"version"`
	Constraints Constraints `json:"constraints"`
	Plan        Plan        `json:"plan"`
	CreatedAt   string      `json:"created_at"`
}
type AmountGroup struct {
	Key          string `json:"key"`
	KnownCents   int64  `json:"known_cents"`
	UnknownCount int32  `json:"unknown_count"`
}
type BudgetSummary struct {
	TripID           string        `json:"trip_id"`
	Version          int64         `json:"version"`
	BudgetTotalCents int64         `json:"budget_total_cents"`
	KnownTotalCents  int64         `json:"known_total_cents"`
	UnknownCount     int32         `json:"unknown_count"`
	Complete         bool          `json:"complete"`
	WithinBudget     *bool         `json:"within_budget,omitempty"`
	ByDay            []AmountGroup `json:"by_day"`
	ByCategory       []AmountGroup `json:"by_category"`
}
type Request struct {
	Token          string    `json:"token"`
	RequestID      string    `json:"request_id"`
	Username       string    `json:"username"`
	Password       string    `json:"password"`
	ID             string    `json:"id"`
	Version        int64     `json:"version"`
	City           string    `json:"city"`
	Keyword        string    `json:"keyword"`
	Page           int32     `json:"page"`
	PageSize       int32     `json:"page_size"`
	IdempotencyKey string    `json:"idempotency_key"`
	Input          *JobInput `json:"input,omitempty"`
	Place          *Place    `json:"place,omitempty"`
	Email          string    `json:"email"`
	Code           string    `json:"code"`
	ChallengeID    string    `json:"challenge_id"`
	ClientIP       string    `json:"client_ip"`
}
type Response struct {
	Error     *APIError       `json:"error,omitempty"`
	User      *User           `json:"user,omitempty"`
	Session   *Session        `json:"session,omitempty"`
	Job       *Job            `json:"job,omitempty"`
	Trip      *Trip           `json:"trip,omitempty"`
	Places    []Place         `json:"places"`
	Trips     []Trip          `json:"trips"`
	Budget    *BudgetSummary  `json:"budget,omitempty"`
	Page      int32           `json:"page"`
	PageSize  int32           `json:"page_size"`
	HasMore   bool            `json:"has_more"`
	Challenge *EmailChallenge `json:"challenge,omitempty"`
}

type EmailChallenge struct {
	ChallengeID string `json:"challenge_id"`
	ExpiresIn   int32  `json:"expires_in"`
	RetryAfter  int32  `json:"retry_after"`
}
