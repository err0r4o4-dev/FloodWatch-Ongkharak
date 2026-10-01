---
name: react-typescript-frontend
description: Use when changing Flood Watch React components, routes, client state, typed API boundaries, or frontend tests.
---
# Purpose
Keep the React/Vite application feature-oriented and type-safe.
# Use When
Building UI behavior or consuming Go API contracts.
# Do Not Use When
Defining backend persistence, worker jobs, or server authorization.
# Required Rules
- Organize feature code by domain and shared primitives by reuse; avoid a global god component.
- Type API responses at the boundary and handle loading, unavailable, stale, and error states explicitly.
- Keep browser requests on the Go API origin; never call AI, Redis, DB, or MinIO directly.
- Test user-visible behavior with Vitest and React Testing Library, not internal implementation details.
# Workflow
Check contract, write a failing behavior test, implement minimally, run typecheck/test/build, inspect rendered states.
# Checklist
- [ ] No fabricated flood data presented as live.
- [ ] Route and keyboard behavior remain accessible.
# Related Skills
`rest-api-contract-design`, `flood-data-reliability`, `accessible-responsive-ui`.
