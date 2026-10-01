---
name: docker-environment-infrastructure
description: Use when changing Flood Watch Dockerfiles, Compose services, local environment variables, networks, volumes, reverse proxy, or service health checks.
---
# Purpose
Keep local infrastructure reproducible, least-privileged, and aligned with service boundaries.
# Use When
Adding or modifying Compose, Docker build stages, Caddy, PostGIS, Redis, MinIO, environment examples, or health checks.
# Do Not Use When
Changing application business logic without deployment or runtime configuration impact.
# Required Rules
- Keep React, Go API/worker, and internal FastAPI services separately buildable and network-scoped.
- Use `.env.example` for documented placeholders; never commit real credentials or production endpoints.
- Persist only required stateful services, use named volumes deliberately, and expose the minimum ports.
- Define health checks and dependency readiness without assuming a healthy container means usable data.
# Workflow
Trace service dependencies, update configuration and documentation together, validate Compose syntax, then start the smallest affected stack.
# Checklist
- [ ] Internal services are not publicly exposed by default.
- [ ] Configuration has safe defaults and clear required variables.
# Related Skills
`go-api-architecture`, `python-fastapi-ai-service`, `backend-security-testing`.
