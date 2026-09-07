# Final integration review

Current status (2026-09-07): all P2 findings recorded below were fixed and re-reviewed; no remaining P1/P2 was identified in the reviewed scope. Root's final whole-repository `go test -race ./... -count=1` against PostgreSQL `travel_test`, `go vet ./...`, native/Linux builds and 11-case fixture E2E all passed **after** the final v6 prompt update. Live v6 three-day and seven-day generation, and an explicit whole-day revision preserving the other dates, also passed. Detailed results and the narrower afternoon scope-conflict case are maintained in [verification.md](verification.md). The sections below preserve the finding/fix history; their earlier failures are not the current status.

Read-only review of gateway, service, store, domain, RPC, process configuration and the public contract. Earlier core/provider findings are excluded because they have separate fix/re-review owners. The following are source-derived reproductions; this reviewer did not interrupt the parent's running integration services or create production-code changes.

## P2 — Database failures are reported as invalid credentials

Location: `internal/store/auth.go:15-18,27-30`.

`Credentials` and `Auth` turn every query error into HTTP 401. A connection failure, closed pool, query cancellation or database outage therefore reports an incorrect password or expired session even when the supplied session remains valid. This affects every protected endpoint and encourages a frontend to discard a valid token during a transient dependency failure.

Reproduce deterministically by opening a store, closing its pool, and calling `Auth(ctx, validTokenHash)` or `Credentials(ctx, existingUsername)`: both return a 401 domain error. With a running API, the equivalent is a PostgreSQL connection failure during session lookup; `/api/v1/me` reports `UNAUTHENTICATED` rather than dependency failure.

Only `pgx.ErrNoRows` should produce an authentication rejection. Preserve context timeout/cancellation and map other database errors to the established dependency error, then test no-row and unavailable-pool cases separately.

## P2 — Canceled map requests consume future capacity indefinitely

Location: `internal/service/worker.go:78-96` (`RateMap.wait`).

The limiter advances `m.next` before waiting and leaves that reservation in place when a request times out or is canceled. Since searches and planning workers share this limiter, a short search burst leaves dead reservations in front of every subsequent map call, even after every original HTTP request has finished.

Reproduce with a fake instant MapProvider and `Interval=500ms`: start 200 `Search` calls with 10ms deadlines. After they have returned, submit one fresh call with a one-second deadline. It times out although no provider request is active, because the abandoned calls advanced the next slot about 100 seconds into the future. At HTTP scale, 400 simultaneous searches can leave roughly 200 seconds of reserved delay, exceeding an entire planning job's 180-second deadline. No per-user search quota prevents this path.

Use a cancellation-aware limiter/reservation API which rejects waits exceeding the caller's deadline and restores canceled reservations, or a bounded admission queue whose canceled entries are removed. A regression should assert that a fresh request can proceed shortly after a canceled burst and that aggregate provider QPS remains bounded.

## P2 — Native Kitex RPC timeouts map to 503 instead of 504

Location: `internal/gateway/gateway.go` (`invoke` error branch); `internal/rpc/travel.go` (`Call` timeout options).

`invoke` only recognizes `context.DeadlineExceeded` or an expired HTTP context. Travel RPC has a three-second timeout under a five-second HTTP context (seven under eight for searches). A Kitex `kerrors.ErrRPCTimeout` can therefore arrive while the HTTP context is still live and falls through to `503 DEPENDENCY_UNAVAILABLE`. The frontend contract explicitly distinguishes synchronous timeout as 504.

Deterministic gateway reproduction: have its fake Caller return `kerrors.ErrRPCTimeout` immediately from a valid `/api/v1/me` request. The current response is 503. A slow real RPC which triggers its client timeout before the outer deadline has the same classification path. Kitex v0.16.3 exposes `kerrors.IsTimeoutError`; its timeout errors need not wrap `context.DeadlineExceeded`.

Recognize Kitex timeout errors at the RPC adapter boundary or in the gateway and preserve the timeout category. Add a focused gateway test alongside its context-deadline test.

## Scope observations

No additional cross-user data-access bypass or missing mandatory HTTP endpoint was found. Routes are returned with saved trip details; a standalone route endpoint is not required by the accepted endpoint table. Model/service secrets are excluded from the model request DTO. The existing owner checks, version CAS, server-assigned model activity IDs, narrow manual-edit DTO, and immutable outside-scope activity merge were inspected. This review does not claim live provider or three-process acceptance; those belong to the parent's integration run.

## Final scoped re-review

All three findings above are resolved in the reviewed code. No remaining P1/P2 was found in these fixes.

- Authentication now emits 401 only for `pgx.ErrNoRows`; other query errors use `dbError`, which recognizes context deadlines as 504 and reports other database failures as 503. The closed-pool regression checks both session lookup and credentials lookup. Its real-PostgreSQL passing execution was reported by the parent; this reviewer inspected the test and implementation without repeating its database setup.
- `RateMap.wait` now advances capacity only when granting an immediately due slot. Waiting callers do not reserve future slots, so cancellation cannot leave the old backlog. The mutex still separates grants by the configured interval. The added canceled-request regression passes.
- Gateway error mapping now recognizes `kerrors.IsTimeoutError` while the outer HTTP context is still live. The focused regression supplies `kerrors.ErrRPCTimeout` and verifies sanitized HTTP 504.

Reviewer verification executed successfully:

```text
GOCACHE=/tmp/ai-travel-build GOMODCACHE=/tmp/ai-travel-mod go test ./internal/service ./internal/gateway -run 'TestCanceledMapRequestsDoNotReserveFutureCapacity|TestKitexRPCTimeoutMapsToGatewayTimeoutWhileContextIsActive' -count=1
ok ai-travel/internal/service
ok ai-travel/internal/gateway
```

## Final live-integration change review

Scope: only the `travel-planner-v2` prompt changes, `RouteIssue` diagnostic changes, and revised `scripts/live_smoke.py` assertions/report. Reviewed the complete source bundle `.local/review-final-live.txt` and the provider/core evidence reports; no repeated tests or production edits were performed.

### Spec compliance: changes needed — P2 prompt conflates time window and editable ID set

Location: `internal/providers/glm.go:20` and the additional revision sentence in `call`.

The new instruction says to replace every activity that should remain inside the editable scope and explicitly excludes locked activities. The actual contract also preserves **every activity absent from `editable_item_ids`**, including activities inside the date/time window that are not explicitly listed in `locked_item_ids`. `MergeRevision` removes the editable ID set, not all activities inside the time window. The prompt's complete-set instruction does not express that distinction.

Concrete case: the base has A at 09:00–10:00 and B at 11:00–12:00; the scope window is 09:00–13:00 with `editable_item_ids=[A]` and no explicit locks. Following the new instruction to return every desired activity inside the window can return replacement A' plus unchanged B. Travel already keeps B, assigns new model IDs, then merges the returned copy, producing overlapping/duplicate B and a failed revision. This is not a scope-protection bypass, but a prompt/merge contract conflict for valid partial selections.

Make the prompt explicit: only the listed editable IDs are removed; return their complete desired replacement set plus intended new activities within the window; never return or modify any base activity whose ID is not editable, regardless of whether it is explicitly locked. Full-day sight/meal/hotel requirements should be described as applying to the **merged** plan for revision requests. Extend the revision prompt test to cover these ID-set semantics rather than only matching the phrase `editable scope`.

The `RouteIssue` change is compliant: it reports the same maximum contiguous free interval and preserves the exact `duration + 300` threshold, including the exact-fit boundary. It does not loosen scope checks or the validator. Meal/rest/hotel occupancy guidance and geographic-nearness preference are consistent with the stated repair purpose.

### Quality: changes needed — P2 live check permits new out-of-scope activities

Location: `scripts/live_smoke.py:36-42`.

The new success checks verify the removed target ID, version increment, unchanged original outside activities, and at least one new museum. They never check the new replacement activities' date or time bounds. Thus they cannot establish that the successful edit stayed inside the requested scope.

Concrete false-positive fixture: return version + 1, preserve every original activity except the target, and add a new museum on a different day (or after `scope.end_minute`). All current assertions pass, `outside_activities_preserved=true` is emitted, and the script exits successfully. This reports preservation of the old activities correctly but would miss the precise out-of-scope addition regression that the live check is intended to catch.

Assert that every replacement activity has `date == scope.date`, `start_minute >= scope.start_minute`, and `end_minute <= scope.end_minute` before accepting the museum replacement. A small assertion-helper regression with an out-of-window museum should fail; no provider call is necessary for that negative fixture.

The revised script correctly makes non-successful revision terminal states fail the process, and its report excludes credentials, session tokens, full plans and model reasoning. The report's `provider_mode=real` remains an operator-selected label rather than independently verified provider provenance; the actual live-run environment/evidence must support that label. No additional finding is raised for that unchanged behavior.

### Live-integration fix re-review — final verdict

**Spec compliance: pass within the reviewed scope.** Prompt v3 now defines `editable_item_ids` as the exact removed set, protects all unselected base activities even inside the time window, and defines that window only as the legal replacement boundary. Generate coverage rules are separated from Revise replacement output and backend merge/full-plan validation. The candidate and `unsatisfied` examples both map to accepted DTO forms; `plan:null` correctly decodes as no plan object. The continuous-gap requirement still uses the unchanged 300-second buffer. The prior prompt P2 is resolved.

**Quality: pass within the reviewed scope.** Live smoke now checks every newly returned activity's exact scope date and inclusive time bounds, requires increasing windows, rejects surviving selected IDs, preserves every unselected activity, and requires at least one new museum activity. The earlier out-of-window museum false-positive case now fails before success is reported. Scope selection uses day two when available, otherwise day one, with a whole-day fallback only when the afternoon selection contains no sight. The prior smoke P2 is resolved. Invalid-output diagnostics use fixed bounded messages rather than echoing unknown field names or decoder text; inspected tests cover unsupported fields and invalid `unsatisfied` shape.

No remaining P1/P2 was identified in this fix wave. This was a single read-only source re-review of the current provider implementation/tests, live smoke script and provider report. Per the parent's instruction, the already-passing provider tests and Python AST check were not repeated. This verdict does not assert that the new v3 live-provider generation/revision run has succeeded; that acceptance evidence remains with the parent.

### V4 operation-mode follow-up

**Spec compliance: pass. Quality: pass within this small change.** Reviewed the current provider source, operation/prompt assertions, and the new provider-report evidence without repeating tests. No new P1/P2 found.

The request DTO's `operation` is derived solely from the Generate/Revise call path and cannot be supplied by model/request text. Generate now always appends the complete-trip instruction, including on repair calls with previous output and issues: return all dates and unchanged activities, never just the conflicting day or a patch. Revise retains the exact editable-ID replacement contract and the protection for unselected activities. These two branches agree with the existing orchestration behavior and do not introduce a new merge or scope rule. Guidance for absent route data explicitly leaves actual route queries/validation to the backend.

The inspected tests verify `operation=generate` and `operation=revise` in serialized model data and the whole-plan Generate wording, while preserving the scoped Revise assertions. Because repair uses the same Generate method, its prompt branch is identical. The provider report records the focused RED/GREEN cycle; this review does not treat it as evidence that v4 has already succeeded on a live multi-day model run. No validator, repair-count, or provider-call behavior change was found in this mode fix.

### V5 model-context projection follow-up

**Spec compliance: pass. Quality: pass within the requested projection and transport-option scope.** No new P1/P2 identified. This review inspected current source, projection tests and the provider report; the already-passing tests were not repeated.

Base and directly recognizable prior plans retain activity IDs, dates, windows, kinds, place IDs, titles, reasons and costs, plus route endpoint IDs, dates, mode, duration, distance and optional fees. Constraints, candidate Places, Scope, operation, instruction and validation issues remain present. These retain the information used to identify activities, honor the editable/locked sets, and repair timing/budget conflicts. Removing duplicated nested Place snapshots and route geometry does not alter persisted evidence or the backend's authority checks. The projection constructs new records and slices and does not write through retained fee pointers.

An ordinary wrapped model result (`outcome/plan/issues`) does not satisfy the plan-content check, so it remains bounded raw repair input rather than becoming an empty projected plan. Malformed or truncated text likewise follows the raw fallback. Projection only changes the next model request: it does not upgrade an invalid result to a candidate, drop validation issues, or bypass strict output/business validation. The synthetic payload reduction demonstrates that geometry was removed while the tested repair fields remain; it is not evidence of measured live token/latency improvement.

`--transport` accepts exactly `walking` and `transit`, defaults to `walking`, and feeds both submitted constraints and the result report from the same parsed value. A transit run therefore remains identifiable separately from walking, with unchanged success/revision assertions. Actual v5 live-provider completion remains a separate acceptance step.

### Transit polyline format and exact-fee follow-up

**Spec compliance: pass. Quality: pass within these adapter changes.** No new P1/P2 found. Reviewed the current adapter/tests, provider report, and supplied public-coordinate diagnostic sample without repeating the reported test run.

The transit decoder accepts direct strings, the observed nested `{ "polyline": "..." }` format, null and missing optional geometry. Wrong object shapes and non-string wrapped values fail decoding and remain dependency errors; they are not converted into fabricated routes. Segment iteration emits its walking geometry before the chosen first busline, and no longer concatenates parallel busline alternatives. Duration, distance and fee still come from the selected complete transit result, so the geometry fix introduces no duplicated walking time or alternative fare summation. Optional absent geometry does not erase the existing numeric route evidence.

`yuanCents` now requires decimal digits in the whole and optional fractional components, rejecting negative signs and malformed fractions before arithmetic. Its `(MaxInt64 - fraction) / 100` guard prevents both multiplication and final-addition overflow while keeping exact integer cents. Missing transit fees remain nil because conversion is invoked only for a nonempty fee string; known zero stays distinguishable from unknown. The inspected regressions cover the original negative-value failure, signed fraction, overflow, supported geometry shapes, first-alternative selection and unchanged total duration/fee.

The saved official diagnostic supplies evidence for the nested response format. This review does not extend that single-route evidence into a claim that the complete live trip-generation/revision chain has passed.

### Transit preference and execution-deadline review

Reviewed only the new bounded walking fallback, its tests, shared deadline constants/consumers, polling limit and associated policy documentation. No tests were repeated.

**Spec compliance: the documented strategy and deadline wiring are consistent.** Initial transit errors other than `MAP_NO_ROUTE` do not trigger walking. Both distance and duration must be nonnegative and at most 1,500, both external queries consume the existing 64-call budget, and successful fallback keeps actual route mode/fee/provider/query time. Fresh, cached and unchanged baseline routes all regain the warning. Boundary fee comparison still rejects a changed protected-boundary fare, including transit fare to zero walking fare; continuous-gap validation still checks the actual duration plus its buffer before save. Daily unknown transport accounting is not changed by a known zero walking leg.

The shared model/RPC/job limits are wired as 120/130/300 seconds. Claim derives its persisted deadline from the shared 300-second value. Queue expiry remains 300 seconds, leases 30 seconds and renewal 10 seconds; the live poll's 650 seconds covers queue plus execution plus sweep/poll margin. Two model calls can fit within the execution budget, but this is a cap rather than a guarantee that all map calls and work will finish. The database runtime verification remains the parent's separate run.

**Quality: one P2 requires correction — preserve fallback dependency failures.** Location: `internal/service/worker.go:440-445`, and the `walking-failure` case in `TestTransitShortWalkingFallback`.

After transit explicitly returns no route, the walking call maps every failure except `MAP_CALL_LIMIT` to `422 MAP_NO_ROUTE`. For example, transit returns `MAP_NO_ROUTE`, then walking returns `503 MAP_UNAVAILABLE` (or a quota/authentication/timeout failure). The job now tells the frontend there is no usable route even though walking availability was never determined. This loses the dependency error category needed to distinguish a temporary retryable failure from an itinerary constraint problem. The current test explicitly expects this misclassification.

Keep contextual `MAP_NO_ROUTE` for a walking no-route result or a successfully queried route outside the short-walk bounds. Preserve other walking error status/category with a fixed sanitized application message; do not expose arbitrary provider messages. Update the existing walking-failure expectation and cover a timeout category. This does not require another fallback, retry or scope change.

### Fallback error-classification and no-route repair re-review

**Spec compliance: pass. Quality: pass within these two corrections.** The preceding fallback-classification P2 is resolved; no new P1/P2 was found. Reviewed the current worker changes, focused tests and appended core-review evidence without repeating tests.

Walking fallback dependency errors now retain API status/code with application-generated messages. Wrapped deadline/cancellation errors become their context sentinels, preserving 504/503 classification without wrapper leakage. Only confirmed no-route outcomes and routes outside the allowed short-walk bounds become contextual no-route results; the call-limit error remains fatal.

The orchestration change converts only `MAP_NO_ROUTE` into repair feedback, advances the prior sightseeing anchor, and continues to the actual next adjacent leg. Missing routes are neither appended nor cached. The inspected A/B/C regression distinguishes a missing A→B from a real B→C and accounts for all three provider requests. Other dependency errors still abort. Nonempty issues continue to prevent Save and flow through the existing single repair opportunity; manual edits still fail validation without calling the model. Neither change increases the model-round limit, relaxes boundary fees, or authorizes saving an incomplete route set.

The parent reported the new focused regressions passing and is responsible for final validation of this revision. Earlier whole-repository race/vet results apply to the prior revision and are not represented here as validation of these last edits.

### V6 soft scheduling guidance and whole-day probe review

**Spec compliance: pass. Quality: pass within these prompt/probe changes.** No new P1/P2 found. Only the new prompt paragraphs, corresponding assertions and live-probe scope parameter/report were inspected; tests were not repeated.

The pace counts are explicitly soft targets beneath feasibility and the unchanged five-sight maximum. The 90-minute unknown-route gap is qualified by the allowed window and is explicitly insufficient, on its own, to reject a constrained revision. Available route evidence takes precedence with the existing duration-plus-300-second rule. Repair guidance favors fewer/closer editable sights and allowed time adjustments while preserving known minimum stays and scope boundaries. These instructions do not relax the business validator or authorize an expanded edit.

The whole-day probe selects a whole-day window explicitly, while the report records requested scope separately from actual scope/date/minutes, including an afternoon-to-day selection fallback. Existing replacement-window and unselected-activity assertions remain in place. A whole-day success should therefore be described as whole-day revision evidence, not as proof that a narrower afternoon boundary-fee conflict was resolved. The parent's v5 generation observations and correctly rejected afternoon edit remain distinct from pending v6 live acceptance.
