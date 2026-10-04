# System overview

```mermaid
flowchart LR
  Browser[React public frontend] -->|same-origin HTTP| Caddy
  Caddy --> Web[Web dev server]
  Caddy --> API[Go Chi API]
  Worker[Go worker] --> DB[(PostgreSQL / PostGIS)]
  API --> DB
  API --> Redis[(Redis)]
  Worker --> Redis
  API --> MinIO[(MinIO)]
  Worker --> AI[Internal FastAPI]
```

`apps/backend` is **one Go module** with separate `cmd/api` and `cmd/worker` executables and shared `internal` code. The worker schedules explicitly registered ingestion adapters; no live external adapter is registered in Phase 3. Go handlers should depend on services, services on repository interfaces only when used, and infrastructure on explicit configuration. Domain concepts must not depend on Chi, HTTP, or vendor types. Python remains internal and separate. React never calls AI, database, Redis, or object storage directly.

The Caddy development proxy sends `/api/*`, `/events/*`, `/docs`, `/openapi.json`, `/health`, and `/ready` to Go and the rest to Vite. Go exposes authentication, source-aware flood-domain resources, community reports, alerts, account notification history, and ADMIN operations under `/api/v1`; realtime uses `/api/v1/events/stream`. Only localhost web, API, and proxy ports are published in development. Production requires TLS, private internal networks, and operational access controls; a configuration file is not proof of a deployed production system.

The current React frontend exposes a static landing page and guest routes for overview, announcements, map, and verified reports. Overview reads bounded public flood-event, risk-area, road, and water-station collections and bounded latest readings without authentication. Announcements consume persisted alerts. Map layers show source and freshness, with no routing engine or fabricated route. There is no current CCTV page route. Authenticated notification history is separate from browser Push subscription settings. ADMIN dashboard counts come from PostgreSQL; private AI evidence requires ADMIN review. Partial failures and unknown/stale data never become reassuring defaults. See `map.md`, `ai-boundary.md`, and `api-coverage.md`.
