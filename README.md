# Flood Watch Ongkharak

ระบบติดตามและแจ้งเตือนสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก

Phase 1 adds authentication to the development foundation, **not a live flood-warning service**. No observations, routes, CCTV feeds, alerts, or AI models are implemented. Unavailable conditions must never be presented as safe.

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
4. Apply PostGIS and auth migrations with `docker compose --profile tools run --rm migrate` (or `docker-compose --profile tools run --rm migrate` if the standalone Compose binary is installed). This uses `golang-migrate` and its version-tracking table.

## Authentication

Public registration creates USER accounts only; GUEST is an unauthenticated visitor. Login returns a short-lived bearer JWT and sets a host-only HttpOnly refresh cookie. Refresh rotates a server-side session, logout revokes its current token, and `/auth/me` requires a current access token. `/api/v1/user/ping` accepts USER and ADMIN; `/api/v1/admin/ping` accepts ADMIN only. See `docs/architecture/authentication.md` for security semantics and limitations.

To create an administrator, set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` in your shell (never in the repository or `.env`), then run `docker compose --profile tools run --rm -e BOOTSTRAP_ADMIN_EMAIL -e BOOTSTRAP_ADMIN_PASSWORD admin-bootstrap`. This is explicit and idempotent, never an automatic startup seed. Use `/docs/` on the same origin to register/login, authorize with the returned bearer token, refresh via the browser's cookie, and exercise the USER/ADMIN demonstration routes. If you override the Caddy host port, include the corresponding exact origin in `CORS_ALLOWED_ORIGINS`. Production requires a fresh strong `JWT_SECRET`, HTTPS, `COOKIE_SECURE=true`, and restricted allowed origins.

The web dev server is also bound to localhost port 5173 and the API to localhost port 8080 for debugging. Python AI, Postgres, Redis, and MinIO have **no published host ports**; access them from the Compose network or with `docker compose exec`. Caddy exposes localhost port 80, not public HTTPS. Production must set strong credentials, TLS, restricted ingress, and `SWAGGER_ENABLED=false`; this Compose file is not a deployment template.

If those host ports are busy, override `WEB_PORT`, `API_HOST_PORT`, and `CADDY_HTTP_PORT` without changing the Compose-internal `API_PORT=8080` and `AI_PORT=8001`; for example, PowerShell can set `$env:WEB_PORT='15173'; $env:API_HOST_PORT='18080'; $env:CADDY_HTTP_PORT='18081'` before Compose. This leaves other projects untouched. The UI development route is `/system-status`; `/health` and `/ready` are API routes.

MinIO uses a pinned Bitnami legacy development image because the formerly published official Docker Hub release could not be pulled. It is not a production image; review the image source and update to a supported distribution before deployment.

If Compose is unavailable, install web dependencies with `corepack pnpm install` then run `corepack pnpm dev`; use `go run ./cmd/api` and `go run ./cmd/worker` from `apps/backend`, and install `apps/ai-python/requirements-dev.txt` before running Uvicorn. For host-run Go, change Docker DNS names in `DATABASE_URL`, `REDIS_URL`, `MINIO_ENDPOINT`, and `AI_SERVICE_URL` to `localhost` and supply reachable infrastructure; `/health` works without it, `/ready` will correctly fail. The AI service is not a browser-facing API.

## Validation

From the repository root, run `corepack pnpm typecheck`, `corepack pnpm test`, and `corepack pnpm build`. From `apps/backend`, run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go build ./cmd/api ./cmd/worker ./cmd/bootstrap-admin`. From `apps/ai-python`, run `python -m pytest`. Finally run `docker compose config` and, with a working Docker daemon, `docker compose --profile tools run --rm migrate`, `docker compose --profile tools run --rm --build go-test`, and `docker compose up --build -d` to inspect service health. See `.github/workflows/foundation.yml` for repeatable CI checks.

## Contract and future work

The OpenAPI contract documents health and Phase 1 authentication routes; its future domain tags do not imply live data endpoints. `PUBLIC READ` / `AUTHENTICATED WRITE` is the policy for future domain routes, not permission to invent Phase 2 behavior. Do not expose this foundation as a production disaster-information service.
