# Backend foundation

`/health` reports API process liveness without checking external services. `/ready` returns `503` until PostgreSQL, Redis, and the MinIO liveness endpoint are all available. Phase 6 additionally checks/creates the configured private media bucket during API startup; the ongoing readiness probe still checks only the unauthenticated MinIO liveness endpoint and does not continuously validate bucket access or object permissions.

Phase 6 report images use a backend-owned S3-compatible storage interface. The API initializes the configured private MinIO bucket at startup, stores validated image bytes under generated object keys, and retains only metadata in PostgreSQL. Access to each image goes through the report visibility/ownership API; clients never receive storage credentials. CCTV snapshots and AI evidence are not implemented. HTTP handlers do not depend directly on a MinIO client.

In non-production environments, `SWAGGER_ENABLED=true` serves `/openapi.json` and `/docs/`. The Swagger UI loads the pinned `swagger-ui-dist@5.29.0` CDN assets, so development browsers need internet access. The OpenAPI runtime copy is checked against `docs/api/openapi.yaml` by `go test ./...`.
