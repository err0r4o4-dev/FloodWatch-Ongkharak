---
name: openapi-api-documentation
description: Use when creating or updating Flood Watch OpenAPI specifications, schemas, examples, security definitions, or Swagger-facing documentation.
---
# Purpose
Keep the OpenAPI contract accurate enough for clients, tests, and operators to rely on.
# Use When
Publishing API paths, request bodies, response schemas, authentication, or API examples.
# Do Not Use When
Changing implementation without a public contract change or documenting internal-only worker protocols.
# Required Rules
- Treat the OpenAPI document as the public contract; update it with every public API change.
- Define reusable schemas, required fields, formats, enums, errors, and security requirements precisely.
- Document provenance, timestamps, stale/unavailable states, and `UNKNOWN` values explicitly.
- Use examples only as illustrative static data; never present invented readings as live evidence.
# Workflow
Start from the intended HTTP contract, update schemas and paths, validate the document, and compare it with handler tests.
# Checklist
- [ ] Every response status has a schema or documented empty body.
- [ ] Security, error, and freshness semantics match implementation.
# Related Skills
`rest-api-contract-design`, `go-api-architecture`, `backend-security-testing`.
