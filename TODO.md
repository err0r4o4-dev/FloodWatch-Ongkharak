# Flood Watch Ongkharak roadmap

`UNKNOWN` is not `SAFE`; preserve source provenance across future phases. Phase 2 is the current authorized scope; do not begin Phase 3.

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

- [ ] Integrate vetted providers through fetch/validate/normalize/deduplicate/persist adapters

## Phase 4 — Public Frontend

- [ ] Build public information and alert experiences without fabricated readings

## Phase 5 — Map & Routes

- [ ] Add map layers, road statuses, and route-mode safeguards

## Phase 6 — Community Reports

- [ ] Add authenticated submissions, media, and verification workflow

## Phase 7 — Realtime & Alert Engine

- [ ] Implement Redis Streams, SSE, and alert evaluation

## Phase 8 — Admin

- [ ] Add authorized management and audit views

## Phase 9 — Notifications

- [ ] Add opted-in subscriptions and delivery

## Phase 10 — CCTV & AI

- [ ] Add camera metadata and reviewed AI evidence integration

## Phase 11 — Production Readiness

- [ ] Harden operations, secrets, observability, and deployment

## Future Skills

- [ ] Add narrower skills only when a later phase exposes meaningful recurring practices not covered by the Phase 0 core.
