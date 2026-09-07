export interface User { id: string; username: string; role: 'user' | 'admin'; email: string; email_verified: boolean }
export interface EmailChallenge { challenge_id: string; expires_in: number; retry_after: number }
export interface Session { token: string; expires_at: string; user: User }

export interface ConstraintsInput {
  city: string
  start_date: string
  end_date: string
  party_size: number
  budget_cents: number
  budget_scope: 'per_person' | 'total'
  interests: string[]
  pace: string
  transport: 'walking' | 'transit'
}
export interface Constraints extends ConstraintsInput { budget_total_cents?: number }

export interface Cost { category: string; amount_cents: number | null; unit: string; source: string; estimated: boolean }
export interface Place { id: string; name: string; location: string; coordinate_system?: 'GCJ-02'; address?: string; fee_cents?: number | null }

// 地点库检索 / 管理员维护使用的完整地点结构
export interface PlaceFull {
  id: string
  provider: string
  provider_poi_id: string
  name: string
  city: string
  city_code: string
  ad_code: string
  location: string
  coordinate_system: string
  address: string
  type_code: string
  tags: string[]
  duration_minutes: number
  open_minute?: number | null
  close_minute?: number | null
  fee_cents?: number | null
  fee_source: string
  active: boolean
  fetched_at: string
  version: number
}

// GET /admin/places 精修入参（不含 open/close 时可整体置空）
export interface PlaceEditInput {
  duration_minutes: number
  tags: string[]
  open_minute: number | null
  close_minute: number | null
  fee_cents: number | null
  fee_source: string
  active: boolean
}
export interface CreatePlaceInput extends PlaceEditInput { id: string }
export interface UpdatePlaceInput extends PlaceEditInput { version: number }
export interface Activity {
  id: string; date: string; start_minute: number; end_minute: number
  kind: string; place_id: string; title: string; reason: string
  place?: Place; costs?: Cost[]
}
export interface Route {
  from_item_id: string; to_item_id: string; date: string
  mode: 'walking' | 'transit'
  distance_m: number; duration_s: number; fee_cents: number | null
  summary: string; polyline: string; from_location: string; to_location: string
}
export interface Plan { title: string; summary: string; activities: Activity[]; routes: Route[]; warnings: string[] }
export interface Trip { id: string; version: number; constraints: Constraints; plan: Plan; created_at: string }
export interface TripSummary { id: string; version: number; constraints: Constraints; title: string; summary: string; created_at: string }

// POST /trips/:id/replanning-jobs 的 scope，后端会做 ValidateScope 校验
export interface ReplanScope {
  date: string
  start_minute: number
  end_minute: number
  editable_item_ids: string[]
  locked_item_ids: string[]
}
export interface ReplanInput {
  expected_version: number
  scope: ReplanScope
  instruction: string
}

// PATCH /trips/:id 手动编辑：后端 planInput 仅接收这些活动字段（DisallowUnknownFields）
export interface ActivityEditInput {
  id: string
  date: string
  start_minute: number
  end_minute: number
  kind: string
  place_id: string
  title: string
  reason: string
}
export interface PlanEditInput {
  title: string
  summary: string
  activities: ActivityEditInput[]
}
export interface ManualEditInput {
  expected_version: number
  plan: PlanEditInput
}

export interface JobError { code: string; message: string; status: number; field_errors?: Record<string, string> }
export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'conflicted' | 'interrupted'
export interface Job {
  id: string; kind: 'generate' | 'replan' | 'manual_edit'; status: JobStatus; stage: string
  trip_id?: string; result_version?: number; created_at: string; updated_at: string; error?: JobError
}
export interface AcceptedJob extends Job { status_url: string }

export interface AmountGroup { key: string; known_cents: number; unknown_count: number }
export interface BudgetSummary {
  trip_id: string; version: number; budget_total_cents: number; known_total_cents: number
  unknown_count: number; complete: boolean; within_budget?: boolean
  by_day: AmountGroup[]; by_category: AmountGroup[]
}
export interface Paged<T> { items: T[]; page: number; page_size: number; has_more: boolean }
