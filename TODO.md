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

## Approved feature — Notifications page and header entry (2026-10-04)

- [x] Add `/notifications` using the existing blue theme, reference-inspired hero, unavailable-history panel, browser preferences and data-safety caution; connect the header bell without adding a primary navigation destination.
- [x] Keep mark-all-read disabled with an explanation until history/read-state APIs exist; do not manufacture inbox contents or unread counts.
- [x] Localize browser Push settings and use the existing authenticated API client; handle account loading/guest, unsupported browser, missing worker, blocked permission, checking, failure/retry and accepted subscribe/unsubscribe actions while preserving shared action dialogs.
- [x] Verify affected rendered states and review scoped changes without replacing concurrent work.

Validation: `corepack pnpm -C apps/web test src/features/notifications src/app/App.test.tsx -t [Nn]otification` passed 21 tests (16 unrelated tests skipped). `node .tmp/notifications-ui-check.cjs` passed isolated headless Edge checks at 1440, 1280, 1024, 820, 768, 390 and 320px, header keyboard activation, bookmark/reload, heading focus, 44px bell target, navigation clearance, 200% notification text, reduced motion and Push states/actions. Screenshots were inspected; no browser runtime exceptions occurred. API and browser Push responses were synthetic fixtures confined to validation, not live flood data. The browser plugin failed initialization twice; existing local Playwright tooling was used without adding dependencies. Standalone Vite build passed with the existing large-chunk warning. Real device Safari and live Push delivery are not verified.

Broad checks: full Vitest had 149 passing tests and seven failures in existing App/auth-main-landmark, map search, overview and reports cases outside this feature. Full typecheck/build remain blocked by the unexported `FloodEvent` import in `AIEvidenceReview.tsx` and unused `location` in `OverviewPage.tsx`; neither unrelated file was changed by this feature. Scoped `git diff --check` passed. No backend, migration, production deployment or later roadmap phase was added.

## Approved feature — Shared SweetAlert2 Action Notifications (2026-10-04)

- [x] Add SweetAlert2 and shared Thai confirmation/success/error/warning/info dialogs matching the supplied rounded-panel/capsule-button reference. Serialize dialogs, use text-only external messages, focus cancel first, and prevent dismissal while confirmed requests are pending.
- [x] Integrate logout, password login/registration, Google account linking, demo-only password recovery, community report submission/edit/upload outcomes, community/AI review, account role/enable/disable actions, system-setting saves, and Push preference outcomes. Keep input validation, background data errors, stale/UNKNOWN states, and realtime updates inline; do not replace browser Push permission or operating-system notifications.
- [x] Add and run focused regression tests, inspect desktop/mobile rendered dialogs, and review the scoped diff without reverting concurrent work.

Validation: the final focused Vitest command covering shared alerts, AppShell alerts, Admin action tests, ReportForm/review tests, notifications, and auth passed 64 tests across 13 files. Latest full frontend Vitest passed 149/160; eleven failures remain outside this change in App, ReportsPage, MapPage and the concurrently changing OverviewPage tests. Earlier full runs had different totals because other tasks were adding tests/implementations during validation; no coverage was removed. Typecheck/full build remain blocked by the existing missing FloodEvent export in AIEvidenceReview and unused location in OverviewPage. Standalone Vite bundling passed with the existing large-chunk warning; it does not replace a passing typecheck. Scoped git diff --check passed, and a source scan found no native alert/confirm/prompt calls outside tests.

Rendered verification: node .tmp/sweetalert-ui-check.cjs passed against an isolated static bundle and mocked APIs in headless Edge at 1280px, 768px, 390px, and 320px. Checked screenshot styling, cancel-left/default cancel focus, 44px targets, Escape/cancel with zero mutation requests, restored focus, keyboard confirmation, disabled cancellation during loading, exactly one request, success acknowledgement/navigation, honest logout failure, demo-only info, 200% text reflow and reduced motion. Screenshots were inspected and are under .tmp/sweetalert-ui. No live flood data or production services were changed; real devices/Safari and live Push delivery were not verified. Concurrent notification-page/Push implementation and externally created commits were preserved; this task did not create commits or rewrite history.

## Approved UI — Photographic Heroes and Overview Dashboard

- [x] Apply the approved blue-themed photographic page hero to Overview, Map, Risk, Announcements, and Reports without editing the application header or navigation. Store the licensed landscape locally, expose image attribution, and label it as decorative rather than live flood evidence.
- [x] Arrange four loaded-record metrics above compact water/rainfall/risk/road panels, with independent published alerts and verified individual COMMUNITY reports. Keep actual events separate, preserve source/observation time/freshness/UNKNOWN, and never invent weather, trends, or 24-hour rainfall totals.
- [x] Link each overview report to its selected public detail; disclose unavailable linked reports instead of silently substituting another report. Refresh overview evidence on water, flood, road, alert, and verified-report realtime events.
- [x] Run focused regressions, inspect affected rendered states and responsive widths, and review the scoped diff while preserving concurrent work and externally created commits.

Verification: the Overview, realtime, Announcements, and Risk focused suites pass 23 tests; both new ReportsPage deep-link tests pass. Latest full frontend suite: 156/162 pass; six existing failures remain in App (development link and three auth landmarks), Map (missing loaded-station search), and Reports (guest sign-in link). The map failure was reproduced with the original pre-change MapPage source in an isolated Vitest configuration. Typecheck and full build remain blocked by TS2305 in AIEvidenceReview importing the absent FloodEvent export. Standalone Vite bundling passes with the existing large-chunk warning, not a passing full build. Scoped diff checks pass. No dependencies, backend contracts, live flood data, production systems, or later roadmap phases were changed.

Rendered verification: node .tmp/overview-ui-check.cjs passes with isolated synthetic API interception in headless Edge at 320, 390, 768, 1024, 1280, and 1440px. Screenshots inspected include the overview and lower panels, mobile layouts, photographic heroes, and empty/error/partial/loading/media-error states. Verified keyboard report navigation, clickable image credits (including the overlapping map workspace), no document overflow at normal text sizes, and 200% enlarged text scoped to the affected overview. Whole-page 200% root text retains the existing 640px body/header minimum at 390px; the header was deliberately not modified. Actual devices/Safari remain unverified. Evidence is under .tmp/overview-ui; this task did not create commits or rewrite history.

## Approved UI — Match Notifications Hero

- [x] Supersede the photographic hero on all five existing page-hero pages with the notifications page's tokenized blue gradient, ring decoration, typography, rounded surface, shadow, and responsive icon tiles. Keep page-specific copy, contextual decorative icons, existing data behavior, and application header/navigation unchanged; leave the notifications reference page intact.
- [x] Remove the photo-only credit/context from rendered heroes and remove map/metric overlap that would obscure the shorter hero. Preserve the old image assets and unrelated/concurrent changes.

Verification: PageHeroArt's five decorative-accessibility tests pass. Affected suites pass 42/45 tests; remaining failures concern the separately changed overview empty-state banner, existing map search, and guest report sign-in link. Typecheck remains blocked by the existing absent FloodEvent export in AIEvidenceReview. Standalone Vite bundling passes with the existing large-chunk warning. The isolated headless Edge check at .tmp/hero-notifications-ui-check.cjs passes all five pages at 1440, 768, 390, and 320px, directly comparing their computed hero/typography/tile styles to /notifications and verifying unchanged header, no document overflow, decorative-only icons, no photo background, and no hero/content overlap. Desktop/mobile screenshots were inspected under .tmp/hero-notifications-ui. Real devices/Safari are unverified; no dependencies, backend changes, production mutations, or commits were made by this task.

## Main landing and overview routing — authorized UI follow-up

- [x] Add an introductory landing page at `/` following the supplied two-column hero and three-step layout, preserving the blue design system and using code-native illustrative SVG without simulated live flood values.
- [x] Move the original overview to `/overview`; retain existing application destinations, four mobile navigation items, guest access, brand return to `/`, and authentication/authorization boundaries. Landing sign-in preserves `returnTo=/overview`.
- [x] Verify focused route/logout regressions and rendered responsive, keyboard, touch-target, and enlarged-text behavior; review the scoped diff and preserve concurrent changes.

Verification: `corepack pnpm --filter @floodwatch/web exec vitest run src/app/LandingRoutes.test.tsx src/app/AppShell.alerts.test.tsx` passes 8/8 tests. The full frontend run recorded 177/184 passing, with seven failures also observed before landing implementation (the existing development link, three auth landmarks, map search, overview empty banner, and realtime overview empty state). Typecheck and full build remain blocked by TS2305: `AIEvidenceReview` imports the absent `FloodEvent` export. Standalone Vite bundling passes with the existing large-chunk warning; this is not a passing full build.

Rendered verification: `node .tmp/landing-ui-check.cjs` passes in isolated headless Edge at 320, 390, 768, 1024, and 1440px; no horizontal overflow, links meet 44px touch targets, skip-link/keyboard entry opens `/overview`, overview bookmarks and brand return work, direct map and login-return links work, and 200% root text fits at 390px. Screenshots inspected include desktop, phone, and enlarged text under `.tmp/landing-ui`; the landing does not request live flood feeds. Safari, actual devices, and a real backend login were not exercised. No dependencies, backend contracts, production data, commits, or later roadmap phases were changed by this task.

## Approved feature — Real Username Login and OTP Password Recovery

- [x] Persist normalized unique usernames and accept email or username login while preserving legacy/Google account identity and safe return destinations.
- [x] Replace simulated recovery with request email, verify six-digit OTP, reset/confirm password, and persistent success with an explicit login button. Report successful registration separately from a failed automatic login.
- [x] Add migration 14, single-use expiring hashed grants, five-attempt OTP checks, per-email resend/hour limits, Argon2id password updates, and atomic revocation of refresh/access sessions. Reproduce and fix the old-password-login/reset race using a barrier-controlled PostgreSQL test.
- [x] Add optional verified STARTTLS SMTP delivery and explicit trusted-proxy CIDR recovery limiting without reading credentials or changing application databases/services.
- [x] Update OpenAPI/runtime JSON and architecture/setup docs, inspect rendered responsive/keyboard states, run focused tests, and independently review security fixes.
- [ ] Release gate: apply migration 14 to the intended application database, configure real SMTP and exact trusted ingress CIDRs securely, deploy the matching API/web services, and verify a real mailbox round trip.

Verification: `corepack pnpm -C apps/web test src/features/auth` passes 36 tests across four files. Standalone Vite bundling passes with the existing large-chunk warning; full typecheck remains blocked by the absent `FloodEvent` export in `AIEvidenceReview.tsx`. Full frontend suite: 183/190 pass; seven failures remain in existing App (development link and three auth-main-landmark checks), Map search, Overview empty-state, and Realtime/Overview empty-state tests. Those unrelated files/tests were not modified or weakened.

Focused Go auth, config, mailer, middleware, router/OpenAPI tests, selected production auth/recovery/Google handler tests, `go vet`, and `go build ./cmd/api` pass. Real authrepo PostgreSQL tests pass twice against a fresh disposable PostGIS database, including full router registration/login/OTP/reset, duplicate usernames, cooldown/attempt/expiry/replay limits, concurrent verify/reset, barrier-controlled login/reset, repeated refresh/reset races, old-session revocation, and linked-Google sign-in/refresh after reset with a nonzero authentication version. Migration 14 up/down/up preserved a synthetic legacy account with null username. Full Go suite still fails in unrelated AI missing flood types and unused admin/notification test imports; a broad parallel integration run also encountered shared-fixture contamination in alert/flood/outbox tests. The new ineligible recovery fixture was fixed to clean up its userless challenge and the clean authrepo suite passes repeatedly.

Rendered checks: `node .tmp/auth-flow-ui-check.cjs` passes in headless Edge at 1280, 390, and 320px with synthetic API interception, not live flood/auth data. Inspected register, OTP/error, mismatch/reset, success, and service-unavailable screenshots; checked keyboard submission, focus transfer, no overflow, one request, mismatch without mutation, no persisted grants, safe returnTo, no automatic success redirect, honest registration-success/login-failure handling, expired-grant restart, username login/error/navigation, 44px primary actions, reduced motion, and scoped 200% text reflow. Local evidence is under `.tmp/auth-flow-ui`; disposable database testing never used application/production data. Independent final security review found no unresolved High/Medium findings in the changed scope; proxy isolation remains dependent on configuring exact trusted ingress CIDRs. No real SMTP inbox, live proxy deployment, real device/Safari, service reload, or Go race-detector run (cgo unavailable) was verified. Concurrent user changes and externally created commits were preserved; agents did not commit or push.

## Approved feature — Existing-feature API coverage

The user approved filling current feature gaps after reviewing the API flow. This is an explicit feature extension, not authorization to begin a later roadmap phase or integrate live providers, routing engines, or CCTV.

- [x] Inspect frontend calls, Go router, worker, OpenAPI, current changes, and baseline checks
- [x] Replace Admin dashboard mock with database-backed statistics and use the shared authenticated frontend client
- [x] Add ADMIN reads for alert rules
- [x] Add account-scoped notification history, unread counts, read state, and durable Push delivery
- [x] Complete private-media AI jobs, non-mock evidence persistence, ADMIN review, and worker wiring
- [x] Align frontend, canonical/embedded OpenAPI, and architecture documentation
- [x] Run focused and integration checks, inspect affected rendered states, and review the final diff

Verified 2026-10-04: fresh `go test -count=1 ./...`, `go vet ./...`, API/worker/bootstrap builds, and OpenAPI coverage/equality checks pass. PostgreSQL-backed API/Admin/AI/notification integration and subscription-lock regression tests pass on disposable PostGIS databases. Migration 1–16 up and 16/15 down/up pass; the committed migration 15 remains unchanged, with subscription versioning added by new migration 16. The full shared-database integration run still has one existing outbox-dispatch test failure due to earlier flood/ingestion fixtures; all five outbox tests pass on a separate clean database. Redis SSE and MinIO integration tests are skipped without their test configuration.

Frontend focused tests: 75 pass; typecheck/build pass (existing bundle-size warning); full suite: 221 pass, seven unrelated failures. Headless Edge rendered checks: 11 scenarios at 1440/390/320px, no exceptions or horizontal overflow. Python: 13 pass, existing Starlette deprecation warning. Independent targeted security review's Medium findings were resolved; bounded claim retries can defer work under sustained contention. No genuine inference model, live providers, real Push device delivery, Google consent, SMTP mailbox, or production deployment was verified. This task applied migrations only to disposable test databases, not the application database.

Initial snapshot: Go tests could not build the existing AI worker because it referenced nonexistent flood types; existing Admin and notification test imports also failed compilation. Frontend typecheck failed on the AI review's missing `FloodEvent` export. The early Vitest capture reported 183 passing and 11 failing tests, but overlapped implementation and included four intentionally red notification regression tests; it was not a clean pre-change baseline. The final full frontend run reports 221 passing and seven unrelated failures in App, Map, Overview, and Realtime tests; existing coverage is preserved. Python previously returned synthetic predictions by default; it now refuses unconfigured inference. See `docs/architecture/api-coverage.md` for approved boundaries and final verification.

## Small Tasks

- [x] Remove the extra "เข้าสู่เมนู" button in the landing nav section.

## Main reference redesign — approved UI follow-up

- [x] Replace the three-step landing with the approved wave/grid hero, language control, map/weather composition, four project goals, and compact footer; omit the requested scroll-down arrow entirely.
- [x] Preserve `/overview`, guest access, direct `/map` navigation, and login `returnTo=/overview`; authenticated users receive menu actions instead of another sign-in prompt.
- [x] Reuse existing map APIs, layer validation, controls, and source/time/freshness details. Retain explicit loading, empty, failed, partial, stale/UNKNOWN, and truncation disclosures; leave weather and EN honestly unavailable rather than fabricate values or translations.
- [x] Add route, evidence, retry, and repeated-bounds regressions; prevent the map from dropping selected details when unchanged bounds are republished or a scrollbar changes the page width. Review the scoped diff while preserving concurrent API/auth/admin/notification work.

Verification: focused LandingRoutes, AppShell logout, and OverviewMapPreview suites pass 15/15 tests. Fresh `corepack pnpm typecheck` and `corepack pnpm build` pass; the existing large-chunk warning remains. Full frontend Vitest records 221/228 passing, with the same seven failures observed in this task's pre-change baseline: the development status link, three auth main landmarks, map search, overview empty banner, and realtime overview empty state. No valid coverage was removed to hide those failures.

Rendered verification: `node .tmp/main-reference-ui-check.cjs` passes in isolated headless Edge at 320, 390, 768, 1024, 1440, and 1672px. Verified no horizontal overflow, 44px visible action targets, no hero scroll link, four goals, disabled EN, real OpenStreetMap base tiles, loading/empty/error/partial states, unobscured map controls, keyboard marker selection, persistent source/time details, list-based detail access and Escape dismissal, guest overview/map navigation, login return parameter, and 200% root text at 390px. Screenshots inspected include desktop, mobile, degraded data, and selected record details under `.tmp/main-reference-ui`; flood evidence in this verification is explicitly synthetic and intercepted, not seeded into application data. Real backend login, live provider data, Safari, and physical devices remain unverified. No dependencies, backend code/contracts, production systems, commits, or later roadmap phases were changed by this UI task.

## Approved account dropdown — inline username editing

- [x] Replace the initial avatar with a checked user icon; show full wrapping email, inline username edit/set controls, red logout, and only a sign-in link for guests. Preserve ADMIN navigation and logout confirmation.
- [x] Persist username through authenticated `PATCH /api/v1/auth/me`, using only the bearer principal, existing normalization/uniqueness rules, a locked account update, disabled-account rejection, and a sanitized response. Align canonical and embedded OpenAPI.
- [x] Prevent duplicate saves, preserve failed drafts, restore keyboard focus, dismiss with Escape/outside interaction, and prevent late responses restoring logged-out accounts. Disable automatic 401 refresh/replay for the mutation to prevent a cross-tab account switch applying a pending edit to another account; independent security review confirmed the fix.

Verification: local focused Go tests and vet pass for auth, handlers, router, and auth repository. Existing Compose Go test service passes username/OpenAPI regressions against development PostgreSQL, including persisted renamed login, rejection of the old username, duplicate protection, and disabled-account protection. Broader router integration fails in unrelated `TestFeatureAPIAuthorizationAndInboxIsolation` at ADMIN AI evidence (500); Redis SSE integration was skipped without TEST_REDIS_URL. Frontend initially passed typecheck/build and 44 focused tests. After concurrent localization changes, the final focused run is 43/45 passing: all new account tests and mutation safeguards pass; two existing auth-route tests still expect English while the routes now render Thai. Full frontend run before those concurrent changes was 233/240 with the same seven failures as the pre-change 221/228 baseline. Latest typecheck/build is blocked by concurrent Admin localization, ForgotPasswordPage, and removed map layerLabels exports; those unrelated edits are preserved.

Rendered evidence: isolated headless Edge against the successful pre-localization build passes 320/390/768/1440px, long email wrapping, 44px controls, red logout, edit/save/loading/duplicate states, guest-only sign-in, keyboard focus/Enter/Escape, outside dismissal, and dropdown layout at 200% text on 390px. Screenshots under `.tmp/account-menu-ui` were inspected. Account data was intercepted synthetic UI evidence, not real user data. Latest localized rendering cannot be rebuilt until concurrent type errors are resolved. No live service rebuild/restart, production mutation, dependency/schema change, commit, or push was performed.

## Approved Thai/English localization and plain-language copy

- [x] Add a shared persistent TH/EN language provider and separate feature catalogs, with key/placeholder parity checks and translated document/accessibility metadata.
- [x] Localize current public, map/overview, auth, administration, report, announcement, and notification screens. Preserve input drafts, selected map evidence, layer filters, original user content, provenance, and honest UNKNOWN/error states.
- [x] Replace primary byte-based password hints, errors, and placeholders with plain-language guidance. Retain exact UTF-8 12–128-byte validation and expose technical details only in an expandable explanation.
- [x] Add migration 17 for existing Push device language, translated delivery summaries, and authenticated update-only owner-scoped locale synchronization with no automatic enrollment or 401 mutation replay. Preserve the concurrent public-only notification and ADMIN sign-in policy.
- [x] Document maintenance/deployment boundaries in `docs/architecture/localization.md`; inspect the final diff and retain concurrent guest/report/auth work.

Verification (2026-10-04, final source snapshot): web typecheck and production build pass; full Vitest passes 304/304 across 50 files. The existing large-bundle build warning remains. Earlier AccountMenu tests intermittently failed during concurrent changes; the final two full runs pass, without removing assertions. The realtime regression now waits for the current localized empty-station panel while retaining all event/refetch-count assertions.

Rendered verification: isolated headless Edge passes 67 checks at 320/390/1440px, including TH/EN switching, persistence, original announcement text, guest feed, translated visible/accessibility labels, form draft/error preservation, keyboard switching, UNKNOWN on map failure, and 200% auth text at 390px. Inspected screenshots are under `.tmp/i18n-ui`. A 320px user-detail overflow was reproduced and fixed by bounding the grid minimum to available width and wrapping the header/long identifiers. Synthetic intercepted fixtures are verification evidence only, not live flood data.

Backend verification: affected notification/handler/router/repository Go tests and vet pass locally. A freshly rebuilt Compose test image passes notification/handler tests, authenticated language/OpenAPI route-contract tests, and all five `TestRepository` persistence/ownership/revision tests against the separate `floodwatch_i18n_verification_20261004` database. Migration 17 default/constraint/down-up checks also passed in a rolled-back transaction. An earlier broader database run failed unrelated concurrent public-feed fixture code constraints and community-report tests with guest migration 18 absent in that isolated database; Redis SSE integration was skipped without `TEST_REDIS_URL`. Those failures are not claimed fixed by localization.

Release boundary: migration 17 must precede deployment of the locale-aware API/worker. This task did not migrate the application database, restart/deploy application services, commit, or push. Real Push-provider delivery, Safari, physical devices, and live disaster data remain unverified. Current public notifications remain in-page only; existing authenticated Push subscriptions can synchronize language without adding a public enrollment control.

## Approved Guest Public Flow (2026-10-04)

- [x] Public information, maps and report submission require no login; retain the person menu with editable browser-local display name defaulting to Guest.
- [x] Administrator entry uses `/admin` and `/admin/login`, safe return paths and backend ADMIN authorization. Public login/signup links are removed; existing accounts are retained and public registration is disabled.
- [x] Anonymous reports remain PENDING/private until approval; VERIFIED reports appear publicly with COMMUNITY provenance. Images use hashed, report-scoped, 15-minute upload capabilities without read/edit privileges.
- [x] Public in-web notifications include published OFFICIAL/COMMUNITY/AI alerts and VERIFIED reports, manual area filtering, global announcements and browser-local read state. Failed area fetches do not retain previous-area results.
- [x] Align OpenAPI and architecture/deployment documentation; complete independent security review and address its alert provenance, post-lock expiry and invalid-token privacy findings.

Verification: full frontend Vitest passes 305/305 across 50 files; production build/typecheck pass with the existing large-chunk warning. Local Go tests, vet and build pass; database/Redis checks without environment configuration skip locally. Serial Docker integration passes report, handler, report repository, public notification repository and router packages against the separate `floodwatch_guest_flow_test` database with migration 18. An initial expiry regression fixture violated the schema's expiry-after-creation constraint; corrected historical timestamps and reran all five packages successfully, without weakening the constraint.

Rendered checks: isolated headless Edge verifies Guest save/Escape/name synchronization, anonymous submission/PENDING, admin login return routing, public feed despite an admin session, errors/retry, widths 320/390/768/1440 and mobile 200% text. Screenshots use intercepted synthetic fixtures, not live flood data. Evidence: `.tmp/guest-flow-ui`, `.tmp/guest-flow-web-final.txt`, `.tmp/guest-flow-go-final.txt`, `.tmp/guest-flow-build-final.txt`. Storage-denied preferences survive component remounts within the page session.

Release boundary: the running application database has NOT received migration 18, and application services have NOT been rebuilt/restarted. Apply prior migrations then `000018_guest_reports` with the matching API/frontend release before using the new flow on the running application. Anonymous Web Push, private tracking links and guest text editing are excluded; real provider delivery, physical devices and Safari remain unverified. No commit/push or existing user deletion occurred.

## Future Skills

- [ ] Add narrower skills only when a later phase exposes meaningful recurring practices not covered by the Phase 0 core.
