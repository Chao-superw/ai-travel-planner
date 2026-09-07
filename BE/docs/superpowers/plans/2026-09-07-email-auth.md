# Email Authentication Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development for the independent SMTP task and inline TDD for the coupled account/RPC work.

**Goal:** Deliver verified email registration, email/password login, safe legacy migration, and frontend documentation.
**Architecture:** Existing Hertz → Kitex Travel → PostgreSQL, with a TLS SMTP adapter owned by Travel. Planner remains unchanged.
**Tech Stack:** Existing Go, pgx, bcrypt, net/smtp, Kitex and Hertz.
**Spec:** ../specs/2026-09-07-email-auth-design.md

## Global Constraints
- Work exclusively in BE; no commit, push, destructive deletion, or real email send.
- Existing Git repository has no initial commit; operate in the explicitly requested workspace, snapshot changed source under BE/.local/auth-before for review.
- SMTP secrets only in ignored BE/.env; never print them.
- Preserve current user IDs/trips and Thrift field IDs. No extra application processes or Redis.

## Task 1: SMTP adapter
- [x] Add internal/mailer TLS SMTP adapter with SendCode(context.Context,string,string,string,time.Time) error; LoadFromEnv() (*SMTP,error).
- [x] First run TLS protocol tests failing, then implement MIME, authenticated TLS, deadlines, injection protection and sanitized errors; no production test bypass.
- [x] Review source and test results before wiring into Travel.

## Task 2: Email account state machine
- [x] Add regression tests proving existing Register accepts unverified username accounts; confirm failure against desired email-only contract.
- [x] Add normalization/HMAC/policy helpers and PostgreSQL rate limits/challenges, atomic register/bind, and email credential reads.
- [x] Tests call Travel.Handle(RequestRegistrationCode/Register/Login/LegacyLogin/RequestEmailBindingCode/BindEmail), use real PG and injected mailbox only at SMTP boundary; assert persistent effects, concurrency, no password replacement and old ID preservation.
- [x] Extract auth dispatch from travel.go, preserve bcrypt/session and add legacy migration gate.

## Task 3: HTTP/RPC/runtime integration
- [x] Update gateway mapping tests first, add six endpoint DTOs, trusted peer IP and retry headers.
- [x] Append domain/Thrift fields and method entries; preserve existing wire IDs through the generator and test generation against reordered fixture structs.
- [x] Wire SMTP/policy environment validation in cmd/travel, update admin promotion, Compose, .env.example and local HMAC key.
- [x] Extend smoke with a local TLS SMTP fixture; run HTTP/RPC/PostgreSQL registration, session persistence and itinerary regression; cover binding ownership and concurrent binding with real PostgreSQL service tests.

## Task 4: Deliver and review
- [x] Update OpenAPI, frontend guide, README, examples and smoke scripts, with failure/retry/legacy semantics.
- [x] Run full tests on real PG, race, vet, build, generation stability and smoke; record actual evidence.
- [x] Obtain focused review, fix actionable defects, re-run relevant verification, report scope and remaining frontend integration.

## Decisions / progress
- Ruling: existing uncommitted workspace is the user-requested target; no worktree can be created without an unauthorized initial commit.
- Ruling: verification-based registration may reveal duplicate email only after valid code; all eligible send requests take the same SMTP path to avoid enumeration.

## Completion evidence
- Full `go test -race ./... -count=1` with the real test PostgreSQL passed; `go vet ./...` and `make build` passed.
- The generator's two regression tests passed; `make generate` changed none of 21 checked IDL/generated/RPC files.
- Final HTTP/Kitex/PostgreSQL/local TLS SMTP smoke passed 12 checks: `.local/smoke-12dcf185/report.json`.
- Focused independent review found a shared rate-budget drain; failing login regression reproduced it, transaction rollback fixed it, and login/send regressions now pass. Re-review found no remaining blocking issue.
- Migration lock inversion was observed against PostgreSQL. DDL ordering plus a version marker fixed repeated migration locking; old-schema preservation and restart-lock regressions pass.
- Current local API/Travel processes were refreshed. Readiness, invalid-email rejection on the new route and no-store were verified without sending mail; existing Planner and PostgreSQL were retained.
- Frontend integration, OpenAPI, examples, architecture and README updated. No FE edits, real email sends, commits or pushes.

## User-authorized scope extension
After backend completion the user explicitly requested adapting FE. The email login/verification registration pages and existing unverified-session binding gate are now integrated. The user then requested removing the legacy-login entry; the public login page exposes email login and registration only. FE passes 45 tests and production build. Focused review findings were reproduced and fixed, and re-review passed. No real email, commit or push was performed.
