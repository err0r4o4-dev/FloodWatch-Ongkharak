# Flood Watch Ongkharak

ระบบติดตามและแจ้งเตือนสถานการณ์น้ำ อำเภอองครักษ์ จังหวัดนครนายก

Phase 3 adds a provider-neutral water ingestion worker, persistent run/source health tracking, and an opt-in synthetic fixture adapter to the canonical PostGIS flood domain. **No real external water provider is integrated and this is not a live flood-warning service.** A separate Open-Meteo model-weather panel for the Srinakharinwirot University Ongkharak campus reference point is available; it is not a local station reading or flood-safety evidence. Unavailable conditions must never be presented as safe. See `docs/architecture/weather.md` for configuration, attribution and non-commercial use limits.

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

## Thai and English

Select TH or EN to switch system text across the current public and administration routes. The browser remembers the selection; forms and map filters remain in place. Original reports and announcements are explicitly identified and are not automatically translated. Password guidance uses plain language while preserving the existing UTF-8 validation rule. Locale catalogs are separated under `apps/web/src/i18n/locales/{th,en}`. Existing authenticated Push subscriptions also carry a device language; apply migration 17 before deploying the updated API/worker. See `docs/architecture/localization.md` for behavior, maintenance, deployment, and verification boundaries.

## Prerequisites

Node.js 24, Corepack/pnpm 10, Go (version in `apps/backend/go.mod`), Python 3.12+, and Docker with the Compose plugin for the complete local stack. Local development credentials in `.env.example` are **not production secrets**.

## Start the stack

1. Copy `.env.example` to `.env` and replace its development passwords if the stack is reachable beyond your computer. Keep `POSTGRES_PASSWORD` consistent with the password inside `DATABASE_URL`.
2. Run `docker compose up --build -d`. Compose waits for Postgres, Redis, and MinIO health before the API; API readiness checks dependencies.
3. Open `http://localhost/` (Caddy), `http://localhost:8080/health`, `http://localhost:8080/ready`, and—only when enabled—`http://localhost:8080/docs` and `http://localhost:8080/openapi.json`.
4. Apply PostGIS, auth, flood-domain, and ingestion migrations with `docker compose --profile tools run --rm migrate` (or `docker-compose --profile tools run --rm migrate` if the standalone Compose binary is installed). This uses `golang-migrate` and its version-tracking table. Apply migrations before starting the worker or calling Phase 2 APIs.

## Access from the same local network

Postgres is available to local database clients at `127.0.0.1:5432`, using the credentials configured in `.env`; this port is not exposed to the LAN.

Caddy publishes the website on all IPv4 interfaces by default; direct web/API debugging ports remain localhost-only. Apply the entry-point change with `docker compose up -d --no-deps caddy`, then open `http://<server-LAN-IP>/` from another device (include `:<CADDY_HTTP_PORT>` when it is not 80). Use Windows `ipconfig` to find the active adapter's IPv4 address; localhost on another device points to that device, not the server.

Add the exact website origin, for example `http://192.168.0.101`, to the comma-separated `CORS_ALLOWED_ORIGINS` in your local `.env`, retaining localhost origins you use. Apply with `docker compose up -d --no-deps api`. This is needed for origin-checked requests such as admin login and report submission; do not use a wildcard. Apply required database migrations before enabling the matching API release.

Allow the Caddy TCP port through Windows Firewall only on a trusted Private network, scoped to the local subnet. Do not disable the firewall or enable router port forwarding. Replace development credentials before sharing. LAN HTTP is for trusted development, not production; secure-context features such as Web Push may be unavailable. Set `CADDY_BIND_ADDRESS=127.0.0.1` to restore localhost-only access, or use the server's LAN IPv4 address to avoid binding other interfaces.

## Frontend hot reload in Docker

Keep using `docker compose up --build` and open `http://localhost/` through Caddy (or the configured `CADDY_HTTP_PORT`). The development web service mounts `apps/web/src`, `apps/web/public`, `apps/web/index.html`, and `apps/web/vite.config.ts` read-only from your computer; dependencies remain inside the image. Saving frontend source or styles triggers Vite HMR without rebuilding, while HTML and public-file changes can trigger a full page reload. Docker enables file polling every 300 ms so Windows/Docker Desktop edits are detected.

If the stack is already running with the old configuration, apply the mounts once with `docker compose up --build -d --no-deps web`. Keep the browser open and save a frontend file to see the update. Rebuild after changing dependency manifests, lockfiles, Dockerfiles, or files outside the mounted paths. Go API/worker and Python code changes still require rebuilding their services. This applies only to the development Compose stack, not production deployment.

## Public web navigation

`/` is the Flood Watch landing page with a blue wave/grid hero, a code-native SVG map illustration, and four project goals. The map and weather card are explicitly labeled UI demonstrations: roads, waterways, areas, markers, risk colors, and the sample 28°C weather are not actual geographic, flood, or weather observations. The landing preview makes no map/weather API requests and loads no external map tiles; it has no simulated zoom controls or event-detail interactions. The real situation map at `/map` continues to use the existing APIs, geometry validation, and source-aware evidence; empty, failed, partial, or stale evidence is never treated as safe. The original overview remains at `/overview`, with its desktop sidebar and four-destination mobile navigation. Announcements and reports remain at `/announcements` and `/reports`; the application logo returns to `/`.

Public pages remain available without login. The hero's "เข้าสู่เมนู" action opens `/overview` directly, and the "ดูแผนที่" action and map-card link open `/map`. The public user menu edits a browser-local display name, defaulting to `Guest`, instead of offering login or registration. There is no extra guest-overview link, hero scroll-down arrow, or introduction anchor. The landing supports Thai/English switching, including all demonstration labels. The weather card has no provider connection, observation timestamp, or claim to show current Ongkharak weather. Administrator authorization and the real map's data behavior remain protected. The footer uses real internal destinations rather than unconfigured social/contact links.

Visitors can submit community reports without an account. Valid reports publish automatically as PENDING with an explicit unreviewed label; administrator review marks them VERIFIED, not official or safe. `/admin/reports` separates unreviewed, reviewed and removed queues. Administrators may remove pending or verified reports with a reason, hide their public detail/images, and restore them to unreviewed with retained private history. `/notifications` remains limited to published announcements and verified community reports, with browser-local read state and manual area filtering. Administrators enter `/admin` and sign in through `/admin/login`. Apply migrations through `000021_report_post_moderation` before deploying the matching API/worker/frontend. Existing pending submissions become public under the new policy: review sensitive historical content before deployment. See `docs/architecture/community-reports.md` and `docs/architecture/guest-public-flow.md` for capabilities, retained legacy data and deployment limits.

## Authentication

For token configuration, `.env` only needs `JWT_SECRET` (32-4096 bytes; use a fresh random value in production). The backend defaults to a 15-minute access token, a 7-day refresh token, and issuer `floodwatch-ongkharak-local` in development/test or `floodwatch-ongkharak` in production. `JWT_ISSUER`, `JWT_ACCESS_TTL`, and `REFRESH_TOKEN_TTL` remain optional overrides for direct backend runs and development Compose. Existing deployments with a custom issuer should keep the `JWT_ISSUER` override to avoid invalidating active access tokens.

Public registration is disabled; existing USER and ADMIN accounts are retained. GUEST is an unauthenticated visitor whose display name is not an account. Administrator login returns a short-lived bearer JWT and sets a host-only HttpOnly refresh cookie. Refresh rotates a server-side session, logout revokes its current token, and `/auth/me` requires a current access token. The retained `/api/v1/user/ping` accepts USER and ADMIN; `/api/v1/admin/ping` accepts ADMIN only. See `docs/architecture/authentication.md` for security semantics and limitations.

To create an administrator, set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` in your shell (never in the repository or `.env`), then run `docker compose --profile tools run --rm -e BOOTSTRAP_ADMIN_EMAIL -e BOOTSTRAP_ADMIN_PASSWORD admin-bootstrap`. This is explicit and idempotent, never an automatic startup seed. Sign in at `/admin`; `/docs/` on the same origin can exercise the retained session APIs and authorize with the returned bearer token. If you override the Caddy host port, include the corresponding exact origin in `CORS_ALLOWED_ORIGINS`. Production requires a fresh strong `JWT_SECRET`, HTTPS, `COOKIE_SECURE=true`, and restricted allowed origins.

## Flood domain API

Public, paginated GET routes list data sources, water stations and readings, flood events, risk areas, roads, and road condition history under `/api/v1/`. Spatial collections support an optional WGS84 `bbox=minLng,minLat,maxLng,maxLat`; geometry uses GeoJSON longitude/latitude order. Only ADMIN can POST a data source, assessed risk area, or condition for an existing road. USER receives 403 and GUEST 401. No live source is configured or seeded; tests use isolated synthetic fixtures and clean them up. See `docs/architecture/flood-domain.md` and `docs/api/openapi.yaml` for exact fields and freshness/UNKNOWN semantics. No route or risk value is a travel-safety guarantee.

The web dev server is also bound to localhost port 5173 and the API to localhost port 8080 for debugging. Postgres publishes only `127.0.0.1:5432`. Python AI, Redis, and MinIO have **no published host ports**; access them from the Compose network or with `docker compose exec`. Caddy exposes port 80 on `CADDY_BIND_ADDRESS`, not public HTTPS. Production must set strong credentials, TLS, restricted ingress, and `SWAGGER_ENABLED=false`; this Compose file is not a deployment template.

## Ingestion worker

External official/model evidence uses a separate opt-in flow: registered provider adapters → private raw archive → transactional evidence/health → public snapshot and SSE → bilingual overview panel. Providers remain disabled until `EXTERNAL_DATA_PROVIDERS` is explicitly configured in both API and worker. Apply migration 20 first; free Open-Meteo providers also require an eligibility acknowledgment. See `docs/architecture/external-data.md` for real GET checks, configuration, datum/freshness safeguards and limitations. The legacy synthetic fixture flow below remains separate.

The worker polls only source codes with server-registered adapters; no real provider is registered by default. To exercise the synthetic fixture in development, configure `INGESTION_FIXTURE_URL` as the origin of a controlled HTTP test server and `INGESTION_FIXTURE_SOURCE_CODE` as an exact `SYNTH_*` source code, then create an enabled `COMMUNITY` data source with that code. The server must serve `/observations` using the documented `SYNTHETIC_TEST_ONLY` fixture contract; never point this at a live provider. Production rejects fixture configuration. The worker does not seed sources or observations, and public API readiness does not depend on external provider availability. See `docs/architecture/ingestion.md` for run status, polling, retry, provenance, and cleanup boundaries.

If those host ports are busy, override `WEB_PORT`, `API_HOST_PORT`, and `CADDY_HTTP_PORT` without changing the Compose-internal `API_PORT=8080` and `AI_PORT=8001`; for example, PowerShell can set `$env:WEB_PORT='15173'; $env:API_HOST_PORT='18080'; $env:CADDY_HTTP_PORT='18081'` before Compose. This leaves other projects untouched. The UI development route is `/system-status`; `/health` and `/ready` are API routes.

MinIO uses a pinned Bitnami legacy development image because the formerly published official Docker Hub release could not be pulled. It is not a production image; review the image source and update to a supported distribution before deployment.

If Compose is unavailable, install web dependencies with `corepack pnpm install` then run `corepack pnpm dev`; use `go run ./cmd/api` and `go run ./cmd/worker` from `apps/backend`, and install `apps/ai-python/requirements-dev.txt` before running Uvicorn. For host-run Go, change Docker DNS names in `DATABASE_URL`, `REDIS_URL`, `MINIO_ENDPOINT`, and `AI_SERVICE_URL` to `localhost` and supply reachable infrastructure; `/health` works without it, `/ready` will correctly fail. The AI service is not a browser-facing API.

Vite also forwards `/api/*` requests unchanged to the Go API, so opening the dev-server port directly supports report submissions and other API calls without changing browser API URLs. Host-run Vite defaults to `http://127.0.0.1:8080`; export `VITE_API_PROXY_TARGET` before starting it if the API uses another port (PowerShell: `$env:VITE_API_PROXY_TARGET='http://127.0.0.1:18080'`). Compose sets the internal target to `http://api:8080`; after changing the web service environment, recreate only that service with `docker compose up -d --no-deps --force-recreate web`. Caddy routing and API authentication remain unchanged. If `localhost` and `127.0.0.1` behave differently, check IPv6 listeners: a host Vite process can occupy `::1` while Docker exposes Caddy on IPv4.

## Validation

From the repository root, run `corepack pnpm typecheck`, `corepack pnpm test`, and `corepack pnpm build`. From `apps/backend`, run `gofmt -l .`, `go vet ./...`, `go test ./...`, and `go build ./cmd/api ./cmd/worker ./cmd/bootstrap-admin`. From `apps/ai-python`, run `python -m pytest`. Finally run `docker compose config` and, with a working Docker daemon, `docker compose --profile tools run --rm migrate`, `docker compose --profile tools run --rm --build go-test`, and `docker compose up --build -d` to inspect service health. See `.github/workflows/foundation.yml` for repeatable CI checks.

## Contract and future work

The canonical `docs/api/openapi.yaml` and embedded Go specification document implemented authentication, flood-domain resources, reports, alerts/rules, realtime, notification history/Push, ADMIN management, AI evidence review, and operational metrics. A route-coverage test checks enabled feature routes against the contract; future tags do not imply live endpoints. `PUBLIC READ` / `AUTHENTICATED WRITE` governs the boundary, with ADMIN required for authoritative domain writes and evidence review. Do not treat application completeness as proof of production readiness or live disaster information.

## Existing-feature API coverage

- ADMIN dashboard: `GET /api/v1/admin/dashboard/stats` returns database-backed account/report counts and audit activity from the last 24 hours; active accounts are enabled accounts, not online sessions.
- Rules: ADMIN can list/get configured rules as well as create/update them; no thresholds are automatically seeded.
- Notifications: authenticated `GET /api/v1/notifications`, `GET /api/v1/notifications/unread-count`, `PATCH /api/v1/notifications/{id}/read`, and `POST /api/v1/notifications/read-all` are scoped to the account. Inbox history does not depend on browser Push permission or VAPID keys.
- The Go worker materializes new alerts into eligible enabled accounts' inboxes and durable delivery jobs independently of Redis. Push is bounded, retryable, and unavailable when unconfigured; historical alerts are not backfilled.
- ADMIN AI review uses `/api/v1/admin/ai-evidence`. The worker uses private media bytes, bounded leased jobs, and non-mock results. Python returns `503 MODEL_UNAVAILABLE` by default because no genuine model is bundled. Live providers, routing, and CCTV remain outside this feature.

Apply migrations 15 and 16 to the intended environment before deploying the updated API and worker. Migration rollback deletes the new inbox/delivery/evidence/job state; back up required data and do not roll back production casually. See `docs/architecture/api-coverage.md` and `docs/architecture/ai-boundary.md` for the approved boundaries and verification record.


## Google Login

Optional Google login supports new USER accounts and password-verified linking to existing accounts while retaining the normal JWT/refresh lifecycle. Configure the backend-only OAuth settings and migration using `docs/architecture/google-login.md`. Until configured, the Google action is unavailable and email/password login remains usable. Live Google consent and sign-in require a registered client; automated fixtures are not evidence of a live provider round trip.

## Admin Panel (Phase 8)

ADMIN users access `/admin` for centralized management:
- **User Management** (`/admin/users`): List/search/filter users, view activity stats, change roles (USER↔ADMIN), enable/disable accounts
- **System Settings** (`/admin/settings`): Configure rate limits, alert defaults, ingestion parameters, and feature flags with category grouping and JSON validation
- **Audit Logs** (`/admin/audit-logs`): Searchable history of admin actions, auth events, setting changes, and report reviews with full metadata and IP tracking

All admin actions are logged to `audit_logs` (migration 000010). Settings are stored in `system_settings` with read-only protection. The admin panel enforces ADMIN role via `RequireAdmin` component and backend middleware. Audit context captures actor, target, IP address, and user agent for security review.
