# Google Login Implementation Plan

**Goal:** Implement the approved Google sign-in, collision linking, and existing-session flow.
**Architecture:** Backend authorization-code OIDC with PostgreSQL single-use challenges and identities. Reuse Flood Watch sessions; React completes sign-in through refresh/me. All execution and final review are inline in the root session because the runtime rejected the allowed worker model; no alternate model was used.
**Tech Stack:** Go, PostgreSQL/pgx, maintained OIDC/OAuth libraries, React/TypeScript, Vitest.
**Spec:** `docs/superpowers/specs/2026-10-03-google-login-design.md` (approved by user on 2026-10-03).

## Global constraints

- Preserve all existing user changes; no commit, new branch, or destructive Git operations.
- Work in the requested workspace. Native subagent isolation may be used; do not create an alternate branch/worktree that drops uncommitted work.
- Implement only this feature. Keep Google secrets server-side, new accounts USER, disabled accounts blocked, and collisions password-verified.
- Challenge TTL five minutes; maximum five link-password failures; single-use browser binding in PostgreSQL.
- Test before implementation. Document unavailable real-provider/database/browser validation honestly.
- No inline comments, unrelated formatting, AI/worker changes, or phase advancement.

## Shared interfaces

Domain package `auth` defines `GoogleIdentity{Subject, Email string}` and `GoogleProvider` with `AuthorizationURL(state, nonce, verifier string) string` and `Verify(context.Context, code, verifier, nonce string) (GoogleIdentity, error)`.

`NewGoogleService(*Service, GoogleRepository, GoogleProvider) (*GoogleService, error)` provides `Start(ctx, returnTo) (GoogleStartResult, error)`, `Callback(ctx, state, browserToken, code) (GoogleResult, error)`, and `Link(ctx, pendingToken, password) (GoogleResult, error)`.

`GoogleStartResult` has `URL` and `BrowserToken`. `GoogleResult` has `Session *SessionResult`, `PendingToken`, and `ReturnTo`. Invalid state and canceled consent are distinguished; callback consumes valid state even when code is empty and returns its safe destination on cancellation.

Domain errors: `ErrGoogleState`, `ErrGoogleCanceled`, `ErrGoogleIdentity`, `ErrGoogleLink`, `ErrGoogleAttempts`, `ErrGoogleConflict`, `ErrGoogleProvider`. `ValidateReturnTo(string) (string,error)` enforces the internal destination policy.

Adapter `googleauth.New(ctx, googleauth.Config{ClientID, ClientSecret, RedirectURI})` returns a provider implementing that interface. Google configuration is optional; partial config fails startup.

Frontend: GET `/api/v1/auth/google/config` => `{enabled:boolean}`; GET start redirects; callback redirects to `/auth/google/link` or `/auth/complete?returnTo=...`, errors to `/login?googleError=CODE`; POST link body `{password:string}` => `{returnTo:string}` and refresh cookie. Google-only API helpers remain separate from existing AuthApi mocks.

## Task 1: Domain and persistence

Owner: root inline. Files: new `internal/auth/google*.go`, minimal `service.go` change, `internal/postgres/authrepo`, next migration and focused tests. Existing auth Repository interface stays compatible; use a separate GoogleRepository interface.

- [x] Add focused negative, replay, collision, disabled-account, destination, and nullable-password tests; run to see failures.
- [x] Implement domain service, safe session issuance, migration, transactional persistence/expiry/link-attempt limits.
- [x] Run focused Go tests and disposable-database integration cases, including HTTP refresh/me/logout and safe rollback.

## Task 2: Google provider

Owner: root inline. Files: new `internal/googleauth`, `go.mod`, `go.sum` only.

- [x] Verify primary documentation and add local signed-token/provider fixture tests; demonstrate failing checks.
- [x] Implement maintained-library verification, fixed endpoints, nonce/PKCE/email validation, bounded network behavior, and typed provider errors.
- [x] Run focused provider tests; fix a red/green JWKS-outage regression so provider failures are not invalid-identity errors.

## Task 3: React flow

Owner: root inline. Files: auth Google helpers/components/pages/tests, `AppRoutes.tsx` and the minimal AppShell auth-route exemption; preserve AuthPage user edits, no global/map files.

- [x] Add failing button/config/link/completion/redirect tests.
- [x] Implement typed helpers, optional-availability Google action, password link, and existing AuthProvider completion/error states.
- [x] Run focused Vitest/typecheck and isolated mock-only headless desktop/mobile/keyboard verification.

## Task 4: Integration and secure configuration

Owner: root. Files: config/tests, Google handler/tests, router, API main, OpenAPI, Compose/Caddy/env examples, architecture/deployment docs, tracker.

- [x] Add configuration, cookie/origin/redirect/error-mapping tests before wiring; cover router/OpenAPI and HTTP/PG lifecycle.
- [x] Wire optional config, independent OAuth cookies, and documented placeholders; validate Caddy callback-error log redaction in a separate container.
- [x] Run focused and broad checks, migration integration, rendered mobile/keyboard checks. Broad failures remain outside this feature as recorded in the tracker.
- [x] Review final diff inline for auth trust boundaries, concurrency, preserved edits and secrets; fix findings and reverify. Independent delegation was unavailable.

## Execution ledger

- 2026-10-03: User approved implementation. Plan interfaces reviewed: workers have disjoint writes; domain produces adapter/handler types, frontend consumes HTTP contracts. Root waits for integration only after completing handler/config tests. No commit/branch requested or created.
- Task status and exact validation outcomes are recorded in `TODO.md` before completion.
- Runtime rejected `cx/gpt-6-luna`; all implementation and review remained inline with no model fallback, commit, branch or production change.
- Disposable PostgreSQL passed migrations 1-13, identity concurrency/nullable scans/state replay/attempt limits/rollback refusal, and new-account/collision HTTP session lifecycle. Migration 13 down/up passed after test cleanup. Temporary containers were stopped without changing the existing app stack.
- Focused frontend auth tests: 16 passed. Focused auth TypeScript, API build, relevant Go vet, Caddy/Compose validation, and mock-only headless layouts/keyboard checks passed. Full frontend suite: 104 passed, 5 failed in pre-existing App/Reports tests. Full typecheck/build blocked by the existing missing FloodEvent export in AIEvidenceReview; broad Go checks blocked by existing AI type references and unused imports in admin/notification tests.
- Real Google consent/sign-in and production rollout remain unperformed because no live credentials were supplied. No application database migration or service restart was performed.
