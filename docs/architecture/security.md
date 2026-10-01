# Security baseline

Phase 0 is local development only. Internal Postgres, Redis, MinIO, and Python AI services have no host port mapping. API/web debug ports and Caddy bind to loopback. Production must terminate TLS, isolate networks, restrict Swagger, rotate strong secrets, use least-privilege database and object-store credentials, configure trusted origins, and add real authentication/authorization before write APIs.

`.env` and generated artifacts are ignored; `.env.example` contains deliberately weak development-only samples. Never log tokens or credentials. Future uploads need content/type/size validation and private buckets, and ingestion needs source validation and deduplication. Health and readiness should not expose internal connection details to anonymous users. Phase 0 does not claim production hardening.
