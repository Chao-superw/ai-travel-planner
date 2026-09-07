# Core correctness review

Read-only review of domain/store/service/RPC and process entrypoints against the accepted design. No production files changed. Findings below are source-derived reproductions; the database lock-wait reproduction has not been executed by this reviewer. Line numbers refer to the compact source at review time.

## P1 — Changed replan boundary fees bypass the scope check

Location: `internal/service/worker.go:48,59-62`.

Every model replacement receives a fresh activity ID. `routes` finds the old route by the **new** `from.ID/to.ID`, and checks scope fee changes only when that exact old edge exists. Consequently any edge touching a replaced activity has `oldExists=false`; the protection does not cover the boundary it is intended to protect.

Reproduce with base sightseeing A → B → C and existing A/B and B/C transit fees of 200 cents. Select only B as editable; have the planner replace it with D inside the same time window; return feasible A/D and D/C routes costing 2,000 cents. Set a sufficiently generous total budget. Both changed boundary routes bypass `SCOPE_CONFLICT`, and the new version can succeed. Design sections 5.3 and 9 require boundary changes affecting outside costs to reject without expanding scope.

Compare boundary routes by the unchanged outside anchor and selected scope, not only exact edge IDs. Cover changed/inserted/deleted scoped activities and both incoming/outgoing edges, including known-to-unknown and unknown-to-known fee changes. Unchanged outside-to-outside edges should still preserve historical route evidence.

## P1 — Final save lease checks use the transaction's start time

Location: `internal/store/trips.go:8,10,14`.

PostgreSQL `now()` remains fixed for an entire transaction. Both the opening and final save gates use it. If saving blocks on a trip row lock after locking the job, lease expiry during that wait is invisible to the final gate. Renew/sweep cannot resolve this because Save holds the job row lock. A late save can publish a version after its lease expired.

Deterministic PostgreSQL reproduction: create a running edit job with a lease two seconds in the future and a later deadline. In another transaction lock the target trip row. Start `Save` with a context lasting at least ten seconds, wait three seconds, then release the trip lock. The current final gate still compares with the pre-wait transaction time and can commit `succeeded`. Expected: `LEASE_LOST`, unchanged current version, and no new trip version/items/routes.

Use actual database wall time (`clock_timestamp()`) for lease/deadline validity, especially the final success and conflict gates. Include the conflict terminal path in the audit: it commits without a fresh validity check at all.

## P2 — Missing city transport disappears from budget completeness

Location: `internal/domain/budget.go:8-11`, `internal/service/worker.go:14,59-64`, `internal/service/travel.go` GetBudgetSummary.

The plan explicitly warns that unlocated meal/hotel detours and daily first/last connections are unconfirmed, but no unknown transport cost represents them. `Budget` counts only explicit costs and returned sightseeing-to-sightseeing routes. When those amounts are known, it emits `complete=true`, allowing GetBudgetSummary to emit `within_budget=true` despite unpriced in-scope city transport.

Reproduce with a one-day transit itinerary containing one sightseeing activity with a curated known ticket fee and one unlocated meal. Hydrate assigns the meal a known 6,000-cent estimate. No sightseeing pair means zero routes. Budget has zero unknown items and is complete, even though all city transit is unpriced. The accepted budget scope includes city transport and forbids treating missing prices as zero.

Persist explicit unpriced transport allowances/items (with clear semantics so known route costs are not double counted), or otherwise propagate unresolved in-scope transport to budget completeness. Assert that the public budget omits within_budget while such costs remain unknown.

## Validation gaps worth adding during integration

The reviewed store tests currently cover session revocation/private job lookup and concurrent idempotency. Service tests cover hydration and contiguous route windows. These do not exercise the actual Save transaction failure paths.

- Inject an item/route insertion failure after the trip/current-version mutation and assert rollback across all tables and no success job.
- Run two Saves against the same base version: exactly one new version, the loser conflicted, and source_job_id tied only to the winner.
- Execute the lease-expiry-during-save scenario above, plus stale attempt ID and an already interrupted job.
- Test RetryPlanningJob via the service for identical retry idempotency and a failed edit whose base version became stale.
- Run replan through process with changed boundary IDs/fees, not only MergeRevision in isolation.
- Verify manual_edit reaches persistence while planner is unavailable, and a route failure preserves the prior version.

No direct cross-user read/write bypass was found in the reviewed service/store paths. New sessions are hashed at rest; replan/manual submissions load the owner-scoped trip; final CAS includes owner and base version. Save wraps trip/version/items/routes/success in one transaction, subject to the lease-time bug above. Model route/place/cost payloads are stripped or replaced before validation, and unchanged activities recover their server snapshot.

## Scoped re-review verdict after fixes

All three reported correctness findings are addressed in the current implementation. No new blocking finding was identified within these fixes.

- **Boundary fees:** `CheckBoundaryFees` derives incoming/outgoing edges from immutable outside activity IDs and compares the maps before saving. Fresh replacement IDs no longer evade the gate; added or removed boundary connections also produce a conflict. The helper is invoked after full route construction. `TestRevisionBoundaryFeeUsesProtectedAnchor` covers the original changed-ID/changed-fee bug and same-fee acceptance.
- **Late save:** opening and final successful Save gates now use `clock_timestamp()`. The conflicted transition also checks current lease/deadline and reads back the status, returning LEASE_LOST instead of committing a false conflict when the guard rejects it. Transaction rollback continues to cover all earlier mutations. The new PostgreSQL regression holds a job lock beyond lease expiry; the other regression exercises stale base-version rejection and an actual foreign-key insertion failure.
- **Budget:** each day represented by saved costs/routes gets an explicit unknown local transport allowance in the calculation. Valid persisted plans always include meal costs per day, so every trip day is represented. Unknown local transport now prevents complete/within_budget claims. Frontend integration documentation explicitly describes the unknown transfer allowance and omitted within_budget field, consistently with the implementation.

Verification boundary: this reviewer independently ran `go test ./internal/domain` successfully using a writable temporary build cache. A service-package rerun could not start because the default module cache was not writable in this review environment. The parent reports the new service and real-PostgreSQL regressions passed; this review inspected those test bodies but did not independently repeat the database run. `TestConcurrentVersionsAndAtomicRollback` currently tests sequential stale saves, not simultaneous Save contention; its name should not be cited as evidence of a concurrent execution stress test. Broader integration/failure checks above remain useful acceptance coverage, not newly discovered defects.

## Route repair feedback follow-up

Enhanced only `internal/domain/validate.go` RouteIssue and its domain test. Failed route feedback now includes date, both activity titles and IDs, from end/to start minutes, actual longest contiguous free seconds, route seconds, unchanged 300-second buffer and required total seconds. It explicitly explains that intervening meal/rest/hotel occupancy cannot count as travel slack or be bypassed by summing separated gaps. Repair guidance stays within the allowed edit scope.

RED: added `TestRouteIssueProvidesReadableRepairContext` first; it failed on all three meal/rest/hotel cases because the original feedback omitted date, titles, endpoints and free/required totals. GREEN: the same targeted test passes after the diagnostic-only change. It also verifies that a route exactly fitting the longest contiguous gap passes and exceeding it by one second fails. FreeWindowSeconds, buffer and validation threshold are unchanged. Command: `GOCACHE=/private/tmp/ai-travel-review-go-cache go test ./internal/domain -run TestRouteIssueProvidesReadableRepairContext -count=1`. No worker/provider files changed.

## Transit preference follow-up

Implemented the parent-directed policy adjustment only in worker.go/worker_test.go: transit first, then one real walking query only for an explicit MAP_NO_ROUTE API error. Accept the walking result only when both distance and duration are nonnegative and at most 1,500 meters/seconds. The returned walking route retains the map provider's actual mode, zero walking fare, provenance and query timestamp. No synthetic path is created. Other transit errors are returned without fallback; failed or excessive walking results become contextual MAP_NO_ROUTE errors. No-route messages contain date and place names with fixed application text, excluding raw provider messages/URLs/keys.

Both calls consume the existing 64-request counter. Cache keys retain the requested preference and stored routes retain actual mode. Warnings are derived after fresh/cache/baseline route selection, so fallback warnings survive Hydrate clearing warnings. Existing boundary cost and continuous free-window gates are unchanged.

TDD: `TestTransitShortWalkingFallback` first failed for the absent second call, absent contextual error and missing fallback behavior; then passed. Cases cover threshold-inclusive success, excessive distance/duration, negative values, quota/auth/unavailable errors with no fallback, the 64th request preventing a 65th fallback, two remaining slots succeeding, walking failure, explicit walking remaining unrestricted by the transit fallback limits, no-route redaction, actual mode/provider/fare, cached and unchanged baseline route reuse with warning/timestamp retention. Targeted final command passed: `GOCACHE=/tmp/ai-travel-build GOMODCACHE=/tmp/ai-travel-mod go test ./internal/service -run 'TestTransitShortWalkingFallback|TestRouteWindowCannotUseInterruptedTime|TestRevisionBoundaryFeeUsesProtectedAnchor' -count=1`. All providers in these tests are fakes; no live API calls were made. This is a documented implementation policy adjustment requested by the parent, not evidence of a separately confirmed user preference or a live fallback acceptance run.

## Walking fallback dependency-error correction

Supersedes the earlier statement that every failed walking lookup becomes MAP_NO_ROUTE. Only an explicit walking MAP_NO_ROUTE or an out-of-bounds short-walking result now produces a contextual 422 no-route error. Other walking API errors retain Code and Status with fixed, contextual, redacted text; MAP_CALL_LIMIT stays unchanged. Wrapped deadline/cancellation errors are normalized to their context sentinel so AsError produces TIMEOUT/504 or CANCELED/503 without leaking wrapper text. Unclassified walking dependency errors become a redacted MAP_UNAVAILABLE/503, never a definitive no-route result.

RED observed for the revised walking-failure expectation and five new unavailable/quota/auth/deadline/canceled cases. GREEN after the targeted correction. Tests assert retained status/code, context identity, no secret or URL leakage, exactly two calls, and confirmed walking-no-route behavior. Final targeted fallback tests passed; existing boundary and contiguous-time tests also passed during this fix. Only worker.go, worker_test.go and this report changed; no real API requests.

## Confirmed no-route enters existing repair feedback

The route orchestration layer now converts only MAP_NO_ROUTE from routeWithPreference into a contextual validation issue. It advances the previous sightseeing anchor to the failed leg's destination and continues checking the next adjacent leg, without inventing or appending a missing route. All dependency, quota and map-budget errors remain fatal. The existing process loop sees nonempty issues, uses its existing one repair opportunity and cannot save while an issue remains; no retries, scope changes or extra model rounds were added.

RED: updated no-route cases and `TestNoRouteCollectsRepairIssueAndContinuesAdjacentLegs` failed against immediate fatal returns. GREEN: targeted tests pass; the new three-point case verifies A→B no-route yields one issue, no fake A→B route, and an actual B→C route after exactly three provider calls. MAP_UNAVAILABLE still stops after one call. Final targeted validation also includes fallback classification, continuous free-window and boundary fee regressions. No live providers called.
