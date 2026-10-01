# Backend foundation

`/health` reports API process liveness without checking external services. `/ready` returns `503` until PostgreSQL, Redis, and the MinIO liveness endpoint are all available. Phase 0 validates the MinIO endpoint, bucket, and credentials are configured, but its readiness probe checks only the unauthenticated MinIO liveness endpoint; it does not validate bucket access or object permissions.

Future report media, CCTV snapshots, and AI evidence use a backend-owned S3-compatible storage boundary. No storage interface is introduced until an object operation requires one, and HTTP handlers must not depend directly on a MinIO client.

In non-production environments, `SWAGGER_ENABLED=true` serves `/openapi.json` and `/docs/`. The Swagger UI loads the pinned `swagger-ui-dist@5.29.0` CDN assets, so development browsers need internet access. The OpenAPI runtime copy is checked against `docs/api/openapi.yaml` by `go test ./...`.
