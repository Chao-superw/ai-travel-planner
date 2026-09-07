# Provider review findings

## P1 — Model output can populate backend-owned fields

**File:** `internal/providers/glm.go:24-27,120-126`

The advertised strict output schema is decoded directly into `domain.Plan`. That shared type contains `routes`, and each `domain.Activity` contains `place` and `costs`, so `DisallowUnknownFields` explicitly accepts all three fields. The adapter then returns them unchanged even though the task requires costs, place snapshots, and routes to be assigned only by the backend. A model response such as the following is accepted as `candidate` and exposes invented backend data to orchestration:

```json
{"outcome":"candidate","plan":{"title":"x","summary":"x","activities":[{"id":"a","date":"2026-09-08","start_minute":60,"end_minute":120,"kind":"sightseeing","place_id":"amap:p","title":"x","reason":"x","costs":[{"category":"ticket","amount_cents":1,"unit":"person","source":"model","estimated":false}]}],"routes":[{"from_item_id":"a","to_item_id":"b","distance_m":1}]},"issues":[]}
```

Use a provider-specific narrow response DTO containing only the documented fields, then map validated fields into `domain.Plan`.

## P1 — Candidate and revision semantic constraints are not validated

**File:** `internal/providers/glm.go:140-159`

`validOutput` checks only outcome, plan presence, unique nonempty IDs, increasing windows, and kind. It does not verify dates against the requested 1–7 day range, full day coverage, the five-sights-per-day cap, candidate `place_id` membership, hotel-night requirements, or revision scope/locked items. The existing `TestGLMGenerateRequestAndCandidate` demonstrates an exact repro: a multi-day request can receive `activities: []` and the adapter returns `candidate`. Likewise, a revision response can contain an activity outside `Scope.Date` or using a locked ID and is accepted. This lets untrusted model output bypass the contract before orchestration sees it. Validation needs the original `PlanningRequest` and whether the call is a revision.

## P2 — `NaN` coordinates pass validation

**File:** `internal/providers/amap.go:204-211`

`strconv.ParseFloat("NaN", 64)` succeeds, and every ordered comparison with NaN is false. Therefore `validLocation("NaN,NaN")` returns true. A search fixture with a matching attraction whose `location` is `"NaN,NaN"` is emitted as an active place, and `Route` sends the same invalid coordinates to Amap. Reject `math.IsNaN` and `math.IsInf` before applying bounds.

## P2 — Strict JSON parsing accepts trailing values

**File:** `internal/providers/glm.go:120-126`

Only one `Decoder.Decode` call is made. Consequently content such as
`{"outcome":"unsatisfied","issues":["none"]} {"extra":"value"}` is accepted as `unsatisfied`, despite the requirement for one strict JSON object. Decode once, then require a second decode to return `io.EOF`.

## P2 — Service token field is still sent to the model

**File:** `internal/providers/glm.go:55-56`

Setting `ServiceToken` to an empty string does not remove it because its JSON tag lacks `omitempty`; the user message contains `"service_token":""`. The brief requires the field to be removed, and the current test only asserts that the secret value is absent. Marshal a dedicated request DTO/map that has no service-token field and assert the key itself is absent.

## Verification

`GOCACHE=/tmp/ai-travel-build GOMODCACHE=/tmp/ai-travel-mod go test ./internal/providers` passes. The passing suite does not cover the acceptance paths above.

---

# Final re-review verdict

No remaining provider findings in the requested fix scope.

The narrow GLM DTO now rejects model-supplied `routes`, activity `costs`, and activity `place` fields before mapping to domain types. The decoder requires EOF after the first JSON value, and the request DTO omits the `service_token` key entirely. Focused tests cover each case.

Amap now rejects NaN and infinite coordinates. Search requests and result filtering admit scenic categories under `11` plus the explicitly supported museum category `140100`, while rejecting unrelated categories such as parking (`150900`); the mixed-category test covers the request value, accepted museum mapping/tags, and exclusion.

The earlier itinerary semantic-validation P1 is withdrawn: full itinerary and revision business validation belongs to the Travel/domain layer under design section 9, outside this provider adapter's responsibility.

Verification passed:

```text
GOCACHE=/tmp/ai-travel-build GOMODCACHE=/tmp/ai-travel-mod go test ./internal/providers
ok ai-travel/internal/providers
```
