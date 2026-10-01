# System overview

```mermaid
flowchart LR
  Browser[React PWA foundation] -->|same-origin HTTP| Caddy
  Caddy --> Web[Web dev server]
  Caddy --> API[Go Chi API]
  Worker[Go worker] --> DB[(PostgreSQL / PostGIS)]
  API --> DB
  API --> Redis[(Redis)]
  Worker --> Redis
  API --> MinIO[(MinIO)]
  Worker --> AI[Internal FastAPI]
```

`apps/backend` is **one Go module** with separate `cmd/api` and `cmd/worker` executables and shared `internal` code. The worker currently boots without ingestion jobs. Go handlers should depend on services, services on repository interfaces only when used, and infrastructure on explicit configuration. Domain concepts must not depend on Chi, HTTP, or vendor types. Python remains internal and separate. React never calls AI, database, Redis, or object storage directly.

The Caddy development proxy sends `/api/*`, `/events/*`, `/docs`, `/openapi.json`, `/health`, and `/ready` to Go and the rest to Vite. `/api/*` and `/events/*` are reserved routes, not implemented features. Only localhost web, API, and proxy ports are published in development. Production architecture requires TLS and private internal networks; no deployment is provided in Phase 0.
