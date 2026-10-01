---
name: backend-security-testing
description: Use when testing Flood Watch backend authentication, authorization, validation, sessions, rate limits, tenant boundaries, or security-sensitive endpoints.
---
# Purpose
Verify backend trust boundaries with behavior tests instead of relying on UI restrictions or happy paths.
# Use When
Adding or changing protected endpoints, roles, tokens, input handling, uploads, admin actions, or external callbacks.
# Do Not Use When
Writing public layout tests or making non-security documentation edits.
# Required Rules
- Test unauthenticated, unauthorized, malformed, and cross-scope requests for every protected capability.
- Enforce authorization server-side at the resource/action boundary; never trust client-supplied roles or identifiers.
- Validate and bound all external input, including headers, pagination, URLs, media metadata, and webhook payloads.
- Keep secrets out of fixtures, logs, errors, and documentation; use safe test credentials only.
# Workflow
Map the trust boundary, write negative tests first, exercise real middleware and handlers, then inspect errors and audit signals.
# Checklist
- [ ] Tests distinguish `401`, `403`, validation, and not-found behavior.
- [ ] Logs and responses expose no credentials or internal details.
# Related Skills
`go-api-architecture`, `rest-api-contract-design`, `docker-environment-infrastructure`.
