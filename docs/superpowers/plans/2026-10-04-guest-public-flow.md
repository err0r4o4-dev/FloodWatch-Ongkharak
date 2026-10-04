# Guest Public Flow Implementation Plan

Goal: implement the approved public Guest flow while retaining authenticated administrator authorization and unpublished-report privacy.

Scope: Guest display-name preference; admin-only UI entry; anonymous pending reports with scoped upload capabilities; public notification feed and area filtering; API contracts, focused security tests and rendered checks. Do not delete existing users or introduce anonymous Push/tracking tokens.

- [x] Inspect current identity, report publication, image access and notification storage boundaries; preserve existing working changes.
- [x] Add nullable report ownership, display-name snapshot and hashed expiring upload capabilities; test validation, wrong token, expiry, pending privacy and concurrent review/upload.
- [x] Remove public login/signup entry points, implement Guest name editing and `/admin/login` return routing, and move report review to `/admin/reports`.
- [x] Expose a bounded public notifications feed from published alerts and verified reports, local read state and manual area filtering; keep legacy inbox isolated.
- [x] Align canonical/embedded OpenAPI, document deployment limits and apply migration only to an isolated test database for integration verification.
- [x] Run affected frontend tests, typecheck/build, Go checks and isolated rendered scenarios; review final diff and record exact evidence in TODO.md.
