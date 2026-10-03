# Flood Watch Ongkharak

ระบบติดตามและแจ้งเตือนสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก

Phase 3 adds a provider-neutral ingestion worker, persistent run/source health tracking, and an opt-in synthetic fixture adapter to the canonical PostGIS flood domain. **No real external provider is integrated and this is not a live flood-warning service.** No frontend map, CCTV feeds, alerts, or AI models are implemented. Unavailable conditions must never be presented as safe.

## Layout

| Path | Responsibility |
| --- | --- |
| `apps/web` | React 19 / TypeScript / Vite app shell |
| `apps/backend` | One Go module: Chi API and worker commands, shared internal code |
| `apps/ai-python` | Internal-only FastAPI health service |
| `docs/api/openapi.yaml` | OpenAPI 3.1 source contract |
| `infra/caddy` | Local same-origin reverse proxy |
| `.agents/skills` | Project task-specific guidance; select only relevant skills |

See `docs/architecture/system-overview.md` and `TODO.md` for boundaries and next phases.

## Prerequisites

Node.js 24, Corepack/pnpm 10, Go (version in `apps/backend/go.mod`), Python 3.12+, and Docker with the Compose plugin for the complete local stack. Local development credentials in `.env.example` are **not production secrets**.

## Start the stack

1. Copy `.env.example` to `.env` and replace its development passwords if the stack is reachable beyond your computer. Keep `POSTGRES_PASSWORD` consistent with the password inside `DATABASE_URL`.
2. Run `docker compose up --build -d`. Compose waits for Postgres, Redis, and MinIO health before the API; API readiness checks dependencies.
3. Open `http://localhost/` (Caddy), `http://localhost:8080/health`, `http://localhost:8080/ready`, and—only when enabled—`http://localhost:8080/docs` and `http://localhost:8080/openapi.json`.
4. Apply PostGIS, auth, flood-domain, and ingestion migrations with `docker compose --profile tools run --rm migrate` (or `docker-compose --profile tools run --rm migrate` if the standalone Compose binary is installed). This uses `golang-migrate` and its version-tracking table. Apply migrations before starting the worker or calling Phase 2 APIs.

## Frontend hot reload in Docker

Keep using `docker compose up --build` and open `http://localhost/` through Caddy (or the configured `CADDY_HTTP_PORT`). The development web service mounts `apps/web/src`, `apps/web/public`, `apps/web/index.html`, and `apps/web/vite.config.ts` read-only from your computer; dependencies remain inside the image. Saving frontend source or styles triggers Vite HMR without rebuilding, while HTML and public-file changes can trigger a full page reload. Docker enables file polling every 300 ms so Windows/Docker Desktop edits are detected.

If the stack is already running with the old configuration, apply the mounts once with `docker compose up --build -d --no-deps web`. Keep the browser open and save a frontend file to see the update. Rebuild after changing dependency manifests, lockfiles, Dockerfiles, or files outside the mounted paths. Go API/worker and Python code changes still require rebuilding their services. This applies only to the development Compose stack, not production deployment.

## Authentication

For token configuration, `.env` only needs `JWT_SECRET` (32-4096 bytes; use a fresh random value in production). The backend defaults to a 15-minute access token, a 7-day refresh token, and issuer `floodwatch-ongkharak-local` in development/test or `floodwatch-ongkharak` in production. `JWT_ISSUER`, `JWT_ACCESS_TTL`, and `REFRESH_TOKEN_TTL` remain optional overrides for direct backend runs and development Compose. Existing deployments with a custom issuer should keep the `JWT_ISSUER` override to avoid invalidating active access tokens.

Public registration creates USER accounts only; GUEST is an unauthenticated visitor. Login returns a short-lived bearer JWT and sets a host-only HttpOnly refresh cookie. Refresh rotates a server-side session, logout revokes its current token, and `/auth/me` requires a current access token. `/api/v1/user/ping` accepts USER and ADMIN; `/api/v1/admin/ping` accepts ADMIN only. See `docs/architecture/authentication.md` for security semantics and limitations.

To create an administrator, set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` in your shell (never in the repository or `.env`), then run `docker compose --profile tools run --rm -e BOOTSTRAP_ADMIN_EMAIL -e BOOTSTRAP_ADMIN_PASSWORD admin-bootstrap`. This is explicit and idempotent, never an automatic startup seed. Use `/docs/` on the same origin to register/login, authorize with the returned bearer token, refresh via the browser's cookie, and exercise the USER/ADMIN demonstration routes. If you override the Caddy host port, include the corresponding exact origin in `CORS_ALLOWED_ORIGINS`. Production requires a fresh strong `JWT_SECRET`, HTTPS, `COOKIE_SECURE=true`, and restricted allowed origins.

## Flood domain API

Public, paginated GET routes list data sources, water stations and readings, flood events, risk areas, roads, and road condition history under `/api/v1/`. Spatial collections support an optional WGS84 `bbox=minLng,minLat,maxLng,maxLat`; geometry uses GeoJSON longitude/latitude order. Only ADMIN can POST a data source, assessed risk area, or condition for an existing road. USER receives 403 and GUEST 401. No live source is configured or seeded; tests use isolated synthetic fixtures and clean them up. See `docs/architecture/flood-domain.md` and `docs/api/openapi.yaml` for exact fields and freshness/UNKNOWN semantics. No route or risk value is a travel-safety guarantee.

The web dev server is also bound to localhost port 5173 and the API to localhost port 8080 for debugging. Python AI, Postgres, Redis, and MinIO have **no published host ports**; access them from the Compose network or with `docker compose exec`. Caddy exposes localhost port 80, not public HTTPS. Production must set strong credentials, TLS, restricted ingress, and `SWAGGER_ENABLED=false`; this Compose file is not a deployment template.

## Ingestion worker

The worker polls only source codes with server-registered adapters; no real provider is registered by default. To exercise the synthetic fixture in development, configure `INGESTION_FIXTURE_URL` as the origin of a controlled HTTP test server and `INGESTION_FIXTURE_SOURCE_CODE` as an exact `SYNTH_*` source code, then create an enabled `COMMUNITY` data source with that code. The server must serve `/observations` using the documented `SYNTHETIC_TEST_ONLY` fixture contract; never point this at a live provider. Production rejects fixture configuration. The worker does not seed sources or observations, and public API readiness does not depend on external provider availability. See `docs/architecture/ingestion.md` for run status, polling, retry, provenance, and cleanup boundaries.

If those host ports are busy, override `WEB_PORT`, `API_HOST_PORT`, and `CADDY_HTTP_PORT` without changing the Compose-internal `API_PORT=8080` and `AI_PORT=8001`; for example, PowerShell can set `$env:WEB_PORT='15173'; $env:API_HOST_PORT='18080'; $env:CADDY_HTTP_PORT='18081'` before Compose. This leaves other projects untouched. The UI development route is `/system-status`; `/health` and `/ready` are API routes.

MinIO uses a pinned Bitnami legacy development image because the formerly published official Docker Hub release could not be pulled. It is not a production image; review the image source and update to a supported distribution before deployment.

If Compose is unavailable, install web dependencies with `corepack pnpm install` then run `corepack pnpm dev`; use `go run ./cmd/api` and `go run ./cmd/worker` from `apps/backend`, and install `apps/ai-python/requirements-dev.txt` before running Uvicorn. For host-run Go, change Docker DNS names in `DATABASE_URL`, `REDIS_URL`, `MINIO_ENDPOINT`, and `AI_SERVICE_URL` to `localhost` and supply reachable infrastructure; `/health` works without it, `/ready` will correctly fail. The AI service is not a browser-facing API.

## Validation

From the repository root, run `corepack pnpm typecheck`, `corepack pnpm test`, and `corepack pnpm build`. From `apps/backend`, run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go build ./cmd/api ./cmd/worker ./cmd/bootstrap-admin`. From `apps/ai-python`, run `python -m pytest`. Finally run `docker compose config` and, with a working Docker daemon, `docker compose --profile tools run --rm migrate`, `docker compose --profile tools run --rm --build go-test`, and `docker compose up --build -d` to inspect service health. See `.github/workflows/foundation.yml` for repeatable CI checks.

## Contract and future work

The OpenAPI contract documents health, Phase 1 authentication, and Phase 2 public read/ADMIN write routes. Future tags do not imply other live endpoints. `PUBLIC READ` / `AUTHENTICATED WRITE` governs the implemented boundary; authoritative flood-domain writes require ADMIN. Do not expose this foundation as a production disaster-information service.


## Google Login

Optional Google login supports new USER accounts and password-verified linking to existing accounts while retaining the normal JWT/refresh lifecycle. Configure the backend-only OAuth settings and migration using `docs/architecture/google-login.md`. Until configured, the Google action is unavailable and email/password login remains usable. Live Google consent and sign-in require a registered client; automated fixtures are not evidence of a live provider round trip.

## Admin Panel (Phase 8)

ADMIN users access `/admin` for centralized management:
- **User Management** (`/admin/users`): List/search/filter users, view activity stats, change roles (USER↔ADMIN), enable/disable accounts
- **System Settings** (`/admin/settings`): Configure rate limits, alert defaults, ingestion parameters, and feature flags with category grouping and JSON validation
- **Audit Logs** (`/admin/audit-logs`): Searchable history of admin actions, auth events, setting changes, and report reviews with full metadata and IP tracking

All admin actions are logged to `audit_logs` (migration 000010). Settings are stored in `system_settings` with read-only protection. The admin panel enforces ADMIN role via `RequireAdmin` component and backend middleware. Audit context captures actor, target, IP address, and user agent for security review.
