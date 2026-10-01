---
name: go-api-architecture
description: Use when adding Flood Watch Go API handlers, services, repositories, worker commands, or service boundaries.
---
# Purpose
Keep the Go API and workers small, explicit, and independently testable.
# Use When
Changing `apps/api`, shared Go packages, or worker command behavior.
# Do Not Use When
Changing React UI, FastAPI internals, or infrastructure-only configuration.
# Required Rules
- Keep HTTP handlers thin: decode, validate, call a service, and map typed errors.
- Keep API and worker commands separate; share domain packages rather than command logic.
- Pass context, dependencies, and transaction boundaries explicitly; never use request globals.
- Preserve provenance and freshness through service boundaries; absent data remains `UNKNOWN`.
# Workflow
Trace the request or job boundary, write a failing focused test, implement the smallest service change, then run Go tests and static checks.
# Checklist
- [ ] Input validation and error mapping are explicit.
- [ ] API and worker behavior remain independently runnable.
# Related Skills
`rest-api-contract-design`, `postgresql-postgis-backend`, `backend-security-testing`.
