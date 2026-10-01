---
name: rest-api-contract-design
description: Use when defining or changing Flood Watch HTTP resources, request validation, responses, pagination, errors, or versioned API behavior.
---
# Purpose
Make public API contracts predictable, typed, and safe for uncertain flood information.
# Use When
Adding endpoints, request/response schemas, filters, status codes, or pagination.
# Do Not Use When
Implementing internal database queries or browser-only component state.
# Required Rules
- Model resources and actions clearly; use stable names, typed fields, and documented defaults.
- Validate requests at the boundary and return consistent machine-readable error responses.
- Use `401` for unauthenticated and `403` for unauthorized requests; never conceal authorization failures as success.
- Include provenance, observed time, and freshness where status data is exposed; never coerce missing data to `SAFE`.
# Workflow
Define the contract and edge cases, update OpenAPI, write contract tests, then implement matching handlers.
# Checklist
- [ ] Success, validation, authorization, and unavailable responses are specified.
- [ ] Pagination, filtering, and time semantics have bounded defaults.
# Related Skills
`openapi-api-documentation`, `go-api-architecture`, `flood-data-reliability`.
