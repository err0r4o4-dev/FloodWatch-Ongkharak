# Flood Watch Ongkharak roadmap

`UNKNOWN` is not `SAFE`; preserve source provenance across future phases. Phase 11 is the current authorized scope; do not begin Phase 12.

## Phase 0 — Foundation (complete)

- [x] Confirm empty Git workspace and create ignore rules
- [x] Establish monorepo structure, project rules, and 17 core skills
- [x] Bootstrap React shell, shared Go API/worker module, and internal FastAPI
- [x] Configure development PostGIS, Redis, MinIO, Caddy, and Compose
- [x] Publish the initial OpenAPI contract and optional Swagger UI
- [x] Add focused tests, CI, architecture documentation, and environment example
- [x] Run available validation and record exact results

Local validation (2026-10-02): pnpm frozen install/typecheck/Vitest (1 passing)/production build; Go gofmt/vet/test/build of both commands; Python pytest (1 passing) and import; Compose config and eight-service startup; PostGIS migration/version query; API/AI health, API readiness 200 and dependency-failure 503/recovery; Caddy route smoke; Chrome desktop/mobile/Swagger rendering (390px viewport has no horizontal overflow). GitHub Actions workflow exists but has not yet run remotely. The pinned legacy MinIO image and CDN-hosted Swagger UI are development-only tradeoffs; MinIO readiness checks liveness, not object permissions.

## Phase 1 — Authentication

- [x] Implement registration, login, refresh rotation, logout, `/me`, USER/ADMIN middleware, and explicit admin bootstrap
- [x] Validate 401/403, cookie flags, input rejection, CSRF origin checks, rate limits, and Argon2id/JWT/session behavior
- [x] Verify PostgreSQL migration 2 and refresh-rotation integration tests; run local HTTP USER/ADMIN lifecycle and idempotent bootstrap
- [x] Align OpenAPI/embedded contract, documentation, config, Compose, CI, and frontend auth foundation
- [x] Run local Go vet/test/build, frontend typecheck/Vitest/build, Python pytest, Compose config, Docker Go suite, and API readiness smoke
- [ ] Browser-rendered Swagger interaction remains unverified: browser automation failed to initialize locally; HTTP `/docs/` and `/openapi.json` returned 200, and the documented manual flow is available

Local Phase 1 validation (2026-10-02): Go gofmt clean, vet/test/build API/worker/bootstrap; Docker PostgreSQL repository integration and full Go suite; frontend typecheck, six Vitest tests, build; Python one pytest passed; Compose config/migration no change; rebuilt API healthy and ready; HTTP register/login/me/refresh/logout, USER 403 on ADMIN, ADMIN 200, repeated bootstrap, invalid origin 403, and Swagger/OpenAPI HTTP 200. Remote GitHub Actions and browser-rendered Swagger interaction have not been verified.

## Phase 2 — Core Flood Domain

- [x] Model seven source-aware water, flood, risk, and road entities with migration 3 and finite-number checks in migration 4; alert engine remains later scope
- [x] Add public paginated read APIs, ADMIN-only manual writes, GeoJSON/bbox filtering, and OpenAPI contracts
- [x] Test canonical UNKNOWN/freshness and PostGIS bbox/intersection/duplicate behavior with synthetic fixtures
- [x] Run Go, frontend, Python, OpenAPI, Docker, fresh-migration, rollback/reapply, and HTTP smoke checks

Local Phase 2 validation (2026-10-02): gofmt clean; `go vet ./...`, `go test ./...`, and builds of API/worker/bootstrap pass. Docker Go suite passes against actual PostGIS, including source provenance, duplicate observations, geometry bbox/intersection, invalid coordinate/SRID, seven finite-number checks, 401/403 ADMIN boundaries, pagination, unknown IDs, and malformed input. Existing dev database migrated to version 4 (`dirty=false`); one disposable database applied 1→2→3→4 with seven tables and exercised 4→3→4 before deletion. OpenAPI 3.1 has 18 paths and 45 schemas, canonical/embedded equality, type-paired GeoJSON coordinates, resolved refs, and no duplicate YAML keys. Frontend typecheck, 6 Vitest tests, and build pass; Python pytest 1 passes (with cache/deprecation warnings). Compose config passes; rebuilt API readiness and OpenAPI return 200, six public collections return 200 without JWT, guest ADMIN write 401, invalid bbox 400. Earlier Phase 2 smoke also verified invalid severity. No real flood data was seeded; synthetic test readings were cleaned up. Remote GitHub Actions and browser-rendered Swagger interaction have not been verified.

## Phase 3 — External Data Ingestion

- [x] Add provider-neutral water ingestion engine, explicit unit/time/coordinate validation, bounded HTTP client, and synthetic-only fixture adapter
- [x] Add ingestion-run and source-health migration 5, transactional dedupe, bounded scheduler, and graceful worker shutdown
- [x] Verify PostGIS/fixture integration, fresh migration/rollback, worker startup, Go/frontend/Python, Compose, and API readiness

Local Phase 3 validation (2026-10-02): Go gofmt clean (46 files), `go test ./... -count=1`, `go vet ./...`, and API/worker/bootstrap builds pass. Docker full Go suite passes against PostGIS, including six ingestion repository integration tests, one error-code unit test, and a controlled `httptest.Server` fixture pipeline exercising partial/NUL rows, 429/503 retry, malformed/oversized/slow responses, duplicate polls, provenance, health degradation/recovery, and cleanup. Additional unit tests cover station-field controls, special-use IP denial, and persistence fallback. Existing dev DB is version 5 (`dirty=false`), with no synthetic source/run records left; a disposable database applied 1→2→3→4→5 (eight tables), rolled back 5→4, reapplied 4→5, then was dropped. Frontend typecheck, 6 Vitest tests and build pass; Python pytest 1 passes with existing deprecation/cache warnings; Compose config passes. Rebuilt worker runs with no fixture configured and API readiness remains 200. No live provider was called, and no ingestion UI or future-phase feature was added. Remote GitHub Actions and browser-rendered Swagger interaction remain unverified.

Future Provider Integration (not implemented): ThaiWater, DDPM, Google Flood Forecasting, and weather sources require verified API contracts, access terms, units, timestamp rules, and identifiers. No synthetic fixture is a live source.

## Phase 4 — Public Frontend

- [x] Build guest overview from existing public water, flood, risk, and road APIs with loading, partial failure, empty, stale, and UNKNOWN treatments
- [x] Add announcements, CCTV, and reports page shells with honest empty states; map UI shell only, with no routing or Leaflet interaction
- [x] Add desktop sidebar, mobile bottom navigation, accessibility/guest/auth regression tests, and responsive visual review
- [x] Run frontend, Go, Python, and Compose validation

Local Phase 4 validation (2026-10-02): frontend typecheck, 18 Vitest tests, and production build pass; Go vet and full Go tests pass from `apps/backend` (outside sandbox after Go cache access denial); Python pytest 1 passes via `.test-venv` with existing deprecation/cache warnings; Docker Compose config passes outside sandbox after Docker config access denial. Edge headless rendered desktop 1440px overview error state and mobile 500px map shell; 320px viewport, full browser interaction, and live API data were not verified. No live flood data was added and no Phase 5 spatial functionality was started.

## Phase 5 — Map & Routes

- [x] Add viewport-bounded, paginated GeoJSON reads and independently toggled flood, water, risk, and road layers
- [x] Show source, freshness, verification, confidence, and canonical UNKNOWN through selected-feature panel and keyboard list
- [x] Add map pan/zoom, fit/reset controls, responsive legend, and Route mode with user-selected points but no fabricated path
- [x] Verify API errors, empty/degraded/truncated states, route safeguards, responsive layouts, and cross-stack regression

Local Phase 5 validation (2026-10-02): frontend typecheck, 31 Vitest tests, and build pass; `go vet ./...` and `go test ./...` pass from `apps/backend`; Python pytest 1 passes with existing deprecation/cache warnings; `docker compose config --quiet` passes. Edge headless rendered 1440/500/390/320px: no horizontal overflow, five mobile navigation links, and minimum measured map/bottom-nav/route-control target height of 44px at 320px. Route mode rendered no path. Local map API was unavailable during browser inspection, so live feature styling/selection and a routing engine are not verified; synthetic evidence exists only in tests. OpenStreetMap tiles require external availability. No Phase 6 work started.

## Phase 6 — Community Reports

- [x] Add canonical COMMUNITY reports with PostGIS Point 4326, server-owned pending status, bounded validation, and owner-only pending edits
- [x] Store validated images in private MinIO through an API-controlled media boundary; enforce owner/public visibility and a three-image limit
- [x] Add ADMIN review with self-verification guard, private review trail, verified-only guest reads, and a distinct community map layer
- [x] Build guest, user, and minimal ADMIN report UI with location selection, media feedback, accessible states, and focused regression coverage

Local Phase 6 validation (2026-10-02): frontend typecheck, 49 Vitest tests, and build pass; Go vet/tests/build pass; Python pytest 1 passes with existing Starlette deprecation warning; Compose config passes. Migration 6 applied to local development PostGIS; fresh Docker synthetic repository and HTTP create/upload/stale-review/verify/guest-read integration tests pass, with a private MinIO bucket/object put/open/delete test. Review uses the latest `updatedAt` revision (including media changes), authenticated report writes are per-account rate-limited, and failed frontend uploads retry only remaining files against the saved report. Production-build guest/error pages rendered at 1440/500/390/320px; CDP mobile emulation confirms no horizontal overflow, five navigation links, and minimum mobile nav height above 44px at 320px. Authenticated UI and a single deployed API+MinIO end-to-end lifecycle were not visually verified; public/auth traffic still needs a trusted edge limiter behind Caddy; automatic route/realtime evaluation remains deferred. No Phase 7 work started.

## Phase 7 — Realtime & Alert Engine

- [x] Add versioned public event envelope, bounded Redis Stream, durable PostgreSQL outbox/retry, and canonical change triggers
- [x] Provide guest SSE with validated replay cursors, reset/heartbeat, cancellation, public-only events, and backpressure bounds
- [x] Persist configured structured alert rules and evidence-based deduplicated decisions; publish alert.created separately from transport
- [x] Refetch canonical REST data from one reconnecting frontend subscription and surface degraded realtime state
- [x] Add focused backend/frontend/security/reliability coverage and update OpenAPI/architecture docs

Local Phase 7 validation (2026-10-02): frontend typecheck, 55 Vitest tests, and build pass; Go vet/tests/command builds pass; Python pytest 1 passes with an existing Starlette deprecation warning; Compose config passes. Migrations 7 and 8 applied to local development PostGIS. Docker/PostGIS integration passes for canonical triggers, configured alert dedupe, UNKNOWN road transition, persisted outbox retry, and guest/USER/ADMIN rule authorization; Redis-backed SSE replay and guest visibility pass with client disconnect completing in about one second. Live Caddy GET `/api/v1/alerts` returned an empty list and `/api/v1/events/stream` returned text/event-stream and a connection frame. Production-build 500px mobile and 1440px desktop empty/realtime states rendered; 320px rendered emulation and an authenticated live browser lifecycle were not verified. No official thresholds or synthetic live incidents were added. Delivery is at-least-once and bounded by Redis retention; public edge rate limiting, operational outbox monitoring, and a deployed end-to-end event-to-browser smoke remain deferred. No Phase 8 work started.

## Phase 8 — Admin (complete)

- [x] Design admin domain models and PostgreSQL schema (migration 000010)
- [x] Implement audit logging middleware, service, and repository layer
- [x] Build ADMIN-only API endpoints for user management, settings, and audit logs
- [x] Add comprehensive backend tests (integration, handler, service)
- [x] Build frontend admin dashboard with navigation and role protection
- [x] Create user management, settings, and audit log viewer interfaces
- [x] Add frontend tests and accessibility validation
- [x] Update documentation with Phase 8 features

Local Phase 8 validation (2026-10-02): Migration 000010 adds audit_logs (9 columns, 4 indexes) and system_settings (9 columns, 2 indexes, 11 default rows). Backend: Go gofmt clean, go vet, go test with 3 new integration tests (audit log CRUD, user management with role/disable/enable, settings with category filter and read-only protection), 13 handler unit tests (all admin endpoints with mocks), and 8 service tests (audit logging, type validation, limits); API/worker/bootstrap build pass. Frontend: pnpm typecheck passes, 62 Vitest tests pass (added 7 admin tests: dashboard rendering, RequireAdmin protection, accessibility checks for ARIA labels/heading hierarchy/pagination), production build succeeds. Admin routes wired in router.go with ADMIN role requirement and rate limiting; AdminService initialized in cmd/api/main.go. Frontend: AdminDashboardPage with 6 navigation cards, UsersPage with search/filter/pagination, UserDetailPage with stats and role/status management, SettingsPage with category grouping and inline JSON editing, AuditLogsPage with expandable details and filters. All admin pages include loading/error states, confirmation dialogs, and Thai localization. Admin navigation link (⚙ จัดการระบบ) appears in AppShell for ADMIN users only. RequireAdmin component protects /admin/* routes. Responsive CSS added for dashboard grid, tables, badges, toolbar, and mobile layouts. README.md updated with Phase 8 admin panel documentation. No live ADMIN account was created or browser-tested; migration/API/frontend structure verified through code inspection and unit tests. No Phase 9 work started.

## Phase 9 — Notifications

- [x] Create `000011_notifications` migration for Web Push subscriptions table
- [x] Add `webpush-go` dependency, configure VAPID keys, and create Notification service and API handlers
- [x] Implement background worker delivery for `alert.created` events outbox processing
- [x] Update frontend service worker (`sw.js`) to receive pushes, and build `NotificationToggle` UI component
- [x] Verify unit tests, mock webpush integration, Vitest, and end-to-end functionality

Local Phase 9 validation (2026-10-03): Migration `000011_notifications` created for web push subscriptions. Go backend integrated `webpush-go` (requires `go get` manually due to safety classifier timeout), handler tests, and outbox integration inside `cmd/api/main.go` dispatcher. React frontend PWA service worker updated to receive pushes and navigate on click. `NotificationToggle` component tested with Vitest and wired up to check browser support. Because the shell tools were unavailable to verify via direct test execution, manual `go mod tidy` and test execution are deferred to the user.

## Phase 10 — AI Evidence Integration

- [x] Add `000012_ai_evidence` migration for linking `report_id` to evidence, and seed an `AI` DataSource
- [x] Build Python FastAPI `/analyze/media` endpoint with structured evidence response schemas
- [x] Build Go internal `ai.Client` and integrate it into the worker to analyze Community Report media outbox events
- [x] Build Admin "AI Evidence Review" UI to verify or reject `UNVERIFIED` `AI`-sourced flood and road observations
- [x] Add `pytest`, Go tests, and Vitest coverage to verify AI evidence lifecycle

Local Phase 10 validation (2026-10-03): Migration `000012_ai_evidence` created. Python FastAPI endpoint `/analyze/media` implemented with mock flood detection logic and tested via `test_main.py`. Go backend `ai.Client` and `ai.EvidenceWorker` built to orchestrate Community Report analysis and insert `UNVERIFIED` AI observations. React frontend added `AIEvidenceReview` page under the Admin Dashboard to allow verification or rejection of AI evidence. Because the shell tools were unavailable for direct testing during this iteration, `pytest` and Go test execution require manual verification by the user.

## Phase 11 — Production Readiness (complete)

- [x] Add Prometheus metrics to the Go backend (`/metrics` endpoint and Chi middleware)
- [x] Create `docker-compose.prod.yml` mapping network isolation and omitting development tools
- [x] Create `Caddyfile.prod` for HTTPS termination and secure proxy headers
- [x] Write `docs/deployment.md` covering server setup, strong secrets, and running the stack

Local Phase 11 validation (2026-10-03): Added `github.com/prometheus/client_golang/prometheus/promhttp` and integrated the `/metrics` endpoint to the Chi router in `router.go`. Created `docker-compose.prod.yml` which defines the production topology without exposing Postgres, Redis, and MinIO ports, while routing traffic entirely through the `caddy` service. Configured `Caddyfile.prod` with security headers, automatic Let's Encrypt TLS, and proper SPA routing. Detailed documentation provided in `docs/deployment.md` on provisioning `.env` variables and running the stack.

## Approved feature — Auth Flow Updates (2026-10-03)

- [x] Update types: Add `username` to `RegisterInput`
- [x] Update AuthPage: Modify login label and add `username` field for registration mode
- [x] Update ForgotPasswordPage: Implement 3-step UI flow (Email -> OTP -> New Password)
- [x] Verify UI components render correctly and handle simulated API calls

## Approved feature — Google Login (2026-10-03)

Specification: `docs/superpowers/specs/2026-10-03-google-login-design.md`. Implementation plan: `docs/superpowers/plans/2026-10-03-google-login.md`. User approved implementation; unrelated phases remain unchanged.

- [x] Implement verified Google OIDC provider and PostgreSQL single-use identity/linking flow.
- [x] Add secure routes/configuration and React login/link/completion UI while preserving password login.
- [x] Document OAuth setup, verify migration on disposable data, run focused/broad checks, and review security inline.
- [x] Record live Google/browser/database validation performed or unavailable without claiming unrun checks.
- [ ] Release gate: securely configure the Google client, apply migration 000013 to the intended application database, deploy/reload the changed services, and verify a real Google round trip.

Google Login validation (2026-10-03): All feature execution remained inline after the runtime rejected the allowed `cx/gpt-6-luna` worker model; no fallback or independent review was claimed. Focused Go auth/provider/config/PostgreSQL/router tests passed on disposable PostGIS, including concurrent identity creation, single-use state, five-attempt password linking, rollback refusal, and new/collision HTTP refresh/me/logout. Google handler tests passed separately using the production handler sources plus `google_test.go` because the existing `admin_test.go` has an unused import. Migration 13 down/up passed after cleanup; the temporary database was stopped. Relevant `go vet` and `go build ./cmd/api` passed. Frontend auth Vitest: 16 passed; focused auth TypeScript passed. Isolated mock-only headless Edge checks and inspected screenshots passed at 1280px/390px, including Google enabled/disabled/unavailable, keyboard linking/error, completion-bootstrap failure, touch target and overflow checks. Browser plugin kernels failed, so an approved temporary Playwright installation was used without changing project dependencies. Development/production Compose and Caddy configurations validated; a separate real Caddy 502 log test confirmed callback code/state/error_description redaction. Validation tools/logs/screenshots remain local under `.tmp/google-login-*`, not application sources.

Broad checks were attempted, not reported as passing: frontend suite had 104 passed and 5 failed in existing App/Reports tests (including pre-existing duplicate main landmarks); project typecheck/build fail on the existing missing `FloodEvent` export in `AIEvidenceReview.tsx`; full Go checks fail on existing AI references to absent flood types and unused imports in admin/notification tests. Those unrelated failures were not modified. Live Google consent/login, production deployment, migration of the actual app database and service reloads were not performed; credentials were not read, printed or added to tracked files. Existing concurrent auth/map/style/documentation changes were preserved.

## Approved feature — Mobile and iPad Floating Navigation (2026-10-03)

- [x] Apply the approved capsule/glass/wave reference in the existing blue theme, preserving all five destinations and desktop sidebar at 1280px/1440px with a fine pointer.
- [x] Support compact layouts through 1200px and touch-only tablet layouts through 1366px; reserve safe-area and floating-navigation clearance for content and mobile map panels.
- [x] Verify rendered navigation, active routes, real touch/keyboard input, visible focus, reduced motion, and the navigation itself at 200% root text size; review the scoped diff without changing unrelated work.

Local validation: `node .tmp/navbar-ui-check.cjs` passed nine Chrome-emulated viewports (320, 390, 768, 820, 1024, 1180, 1366 touch; 1280 and 1440 desktop), five touch destinations, keyboard/Enter navigation, map-panel clearance/Escape dismissal, and 200% navigation text sizing with no browser runtime exceptions. Screenshots/results are in `.tmp/navbar-*`; API responses were intercepted in the isolated test tab, not seeded as live flood data. Real iOS/iPadOS Safari and device safe-area behavior are not verified. Existing page/header reflow at 200% root text size remains outside this navbar-only scope.

`corepack pnpm -C apps/web test src/app/App.test.tsx -t 'navigation|mobile|icons|risk areas|institutional'`: 5 passed, 11 unrelated tests skipped. Full Vitest: 104/109 passed; five failures remain outside the CSS-only change (development-status link, three auth-page main-landmark tests, and the guest report-list test). Typecheck and full build are blocked by existing TS2305 in `src/features/admin/AIEvidenceReview.tsx:4` importing the unexported `FloodEvent` type. Standalone `corepack pnpm -C apps/web exec vite build` passed with the existing large-chunk warning; this does not replace a passing typecheck/full build. Scoped `git diff --check` passed. No dependencies, routes, backend behavior, or later roadmap phases were added.

## Future Skills

- [ ] Add narrower skills only when a later phase exposes meaningful recurring practices not covered by the Phase 0 core.
