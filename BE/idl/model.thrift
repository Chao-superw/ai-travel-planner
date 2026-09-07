// Generated from internal/domain/types.go by scripts/generate_idl.py
namespace go model

struct Constraints {
  1: string city
  2: string start_date
  3: string end_date
  4: i32 party_size
  5: i64 budget_cents
  6: string budget_scope
  7: i64 budget_total_cents
  8: list<string> interests
  9: string pace
  10: string transport
}

struct Place {
  1: string id
  2: string provider
  3: string provider_poi_id
  4: string name
  5: string city
  6: string city_code
  7: string ad_code
  8: string location
  9: string coordinate_system
  10: string address
  11: string type_code
  12: list<string> tags
  13: i32 duration_minutes
  14: optional i32 open_minute
  15: optional i32 close_minute
  16: optional i64 fee_cents
  17: string fee_source
  18: bool active
  19: string fetched_at
  20: i64 version
}

struct Cost {
  1: string category
  2: optional i64 amount_cents
  3: string unit
  4: string source
  5: bool estimated
}

struct Activity {
  1: string id
  2: string date
  3: i32 start_minute
  4: i32 end_minute
  5: string kind
  6: string place_id
  7: string title
  8: string reason
  9: optional Place place
  10: list<Cost> costs
}

struct Route {
  1: string from_item_id
  2: string to_item_id
  3: string date
  4: string mode
  5: i64 distance_m
  6: i64 duration_s
  7: optional i64 fee_cents
  8: string provider
  9: string queried_at
  10: string summary
  11: string polyline
  12: string from_location
  13: string to_location
}

struct Plan {
  1: string title
  2: string summary
  3: list<Activity> activities
  4: list<Route> routes
  5: list<string> warnings
}

struct Scope {
  1: string date
  2: i32 start_minute
  3: i32 end_minute
  4: list<string> editable_item_ids
  5: list<string> locked_item_ids
}

struct JobInput {
  1: Constraints constraints
  2: i64 expected_version
  3: optional Scope scope
  4: string instruction
  5: optional Plan plan
}

struct APIError {
  1: string code
  2: string message
  3: i32 status
  4: map<string,string> field_errors
  5: optional i32 retry_after
}

struct ModelRun {
  1: string provider_request_id
  2: string returned_model
  3: string prompt_version
  4: i64 duration_ms
  5: optional i64 prompt_tokens
  6: optional i64 completion_tokens
  7: string finish_reason
}

struct PlanningRequest {
  1: string service_token
  2: string job_id
  3: string attempt_id
  4: Constraints constraints
  5: list<Place> places
  6: optional Plan base
  7: optional Scope scope
  8: string instruction
  9: string previous_output
  10: list<string> validation_issues
}

struct PlanningResult {
  1: string outcome
  2: optional Plan plan
  3: list<string> issues
  4: string raw_output
  5: optional ModelRun model_run
  6: optional APIError error
}

struct User {
  1: string id
  2: string username
  3: string role
  4: string email
  5: bool email_verified
}

struct Session {
  1: string token
  2: string expires_at
  3: User user
}

struct Job {
  1: string id
  2: string kind
  3: string status
  4: string stage
  5: string trip_id
  6: i64 result_version
  7: string parent_job_id
  8: string created_at
  9: string updated_at
  10: optional APIError error
  11: list<ModelRun> model_runs
}

struct Trip {
  1: string id
  2: i64 version
  3: Constraints constraints
  4: Plan plan
  5: string created_at
}

struct AmountGroup {
  1: string key
  2: i64 known_cents
  3: i32 unknown_count
}

struct BudgetSummary {
  1: string trip_id
  2: i64 version
  3: i64 budget_total_cents
  4: i64 known_total_cents
  5: i32 unknown_count
  6: bool complete
  7: optional bool within_budget
  8: list<AmountGroup> by_day
  9: list<AmountGroup> by_category
}

struct Request {
  1: string token
  2: string request_id
  3: string username
  4: string password
  5: string id
  6: i64 version
  7: string city
  8: string keyword
  9: i32 page
  10: i32 page_size
  11: string idempotency_key
  12: optional JobInput input
  13: optional Place place
  14: string email
  15: string code
  16: string challenge_id
  17: string client_ip
}

struct Response {
  1: optional APIError error
  2: optional User user
  3: optional Session session
  4: optional Job job
  5: optional Trip trip
  6: list<Place> places
  7: list<Trip> trips
  8: optional BudgetSummary budget
  9: i32 page
  10: i32 page_size
  11: bool has_more
  12: optional EmailChallenge challenge
}

struct EmailChallenge {
  1: string challenge_id
  2: i32 expires_in
  3: i32 retry_after
}
