# Google Login Design

Date: 2026-10-03

Status: Conversational flow approved. Written specification awaiting user review before implementation. No application code, configuration, or database has been changed for this feature.

## Goal and scope

Add Google login/registration through backend-mediated OpenID Connect authorization code, preserving email/password login, account IDs, report ownership, roles, and JWT/refresh sessions. Google verifies identity; Go controls eligibility, linking, authorization, and app sessions. React never receives Google secrets or stores Google tokens. Worker and Python AI service are outside scope.

New accounts are USER only. Email collisions never auto-link: verify the existing account's password. Existing Google identities sign into their original user IDs without changing email or role automatically.

Exclude One Tap, popup login, Google API access, offline Google refresh tokens, password reset, unlinking, merging, and unrelated redesign. Record this explicitly authorized feature separately in the tracker when implementation starts; do not begin unrelated phases.

## Inspected baseline

- `apps/web/src/features/auth/AuthPage.tsx` has a disabled Google button and uncommitted user edits to preserve.
- `AuthProvider.tsx` bootstraps with refresh then me; `api.ts` keeps access JWTs in memory. `apps/web/src/app/AppRoutes.tsx` needs completion/link routes.
- `apps/backend/internal/auth/service.go` handles eligibility and sessions. Reuse it rather than introduce a parallel session type.
- `internal/http/handler/auth.go` guards origins and uses host-only HttpOnly refresh cookies at `/api/v1/auth`. Refresh supports Lax/Strict; a separate OAuth cookie needs Lax for Google's cross-site callback GET.
- `internal/postgres/authrepo/repository.go` scans password hashes as strings. Migration `000002_auth` requires NOT NULL; add a new migration, never edit applied history. The highest inspected migration is `000012_ai_evidence`; recheck numbering before implementation.
- `TODO.md` marks Phase 11 complete; historical deferred checks do not prove current tests pass. Preserve other uncommitted edits, including README, AppShell, ForgotPasswordPage, ReportsPage, CSS, Vite, and Compose.

## User flow and endpoints

Paths are relative to `/api/v1`.

1. Login/register show a Google action. GET `/auth/google/config` returns only `{ "enabled": true/false }`. Absent configuration leaves password login usable with a clear Google-unavailable state.
2. Clicking Google navigates to GET `/auth/google/start?returnTo=...`. Validate same-origin initiation/internal destination (default `/reports`), create browser-bound temporary state, and redirect to Google with `openid email`, state, nonce, and S256 PKCE challenge. No unused profile/offline scopes.
3. Google returns to GET `/auth/google/callback`. Validate and atomically consume browser-bound state, exchange code with client secret/PKCE verifier, and verify ID token.
4. A linked subject loads its current account; disabled accounts/invalid roles receive no session.
5. An unlinked subject with unused verified email atomically creates/links a USER without a local password.
6. An unlinked subject matching an existing email creates a browser-bound pending link to that exact account, then redirects to `/auth/google/link`. No session yet.
7. POST `/auth/google/link` accepts only `{ "password": "..." }` JSON plus pending cookie. Derive target account/subject from pending state, not submitted IDs/email. Enforce origin, input bounds, expiry, rate limits, and at most five failed password attempts. Correct proof links the original user ID, sets the regular refresh cookie, and returns validated destination.
8. Success navigates to `/auth/complete?returnTo=...`. React waits for refresh/me bootstrap before redirecting. Bootstrap failure displays an actionable error, never false success.

Use existing JSON errors: generic 401 for wrong password or invalid/expired/consumed pending state; 429 for exhausted attempts; 403 for origin rejection; 409 for identity conflict; 502/503 for provider outage; generic 500 for storage/session failure. Configuration absence is explicit. Cancellation returns to login with a safe error code. Do not show raw provider errors or add public email lookup.

Callback deliberately receives cross-site navigation: do not apply the password handler's ordinary origin guard; require validated browser-bound state instead. Start rejects unsolicited cross-site initiation; link POST retains origin checks. OAuth responses use no-store/no-referrer headers. No token/password goes into URLs or route state.

## Persistence and concurrency

Add `auth_identities` with user foreign key, GOOGLE provider, opaque subject, and timestamp. Enforce unique provider/subject and one Google identity per account. Subject identifies the account; email is only a registration/collision attribute.

Make password_hash nullable and adapt scanners/authentication. Password registration still stores Argon2id hashes. Google-only password login fails generically with comparable dummy-hash work; never store fabricated passwords.

Persist OAuth/pending challenges in shared PostgreSQL, not process memory, for cross-replica replay protection. Store hashes of random browser handles/state, plus nonce, PKCE verifier, validated destination, purpose, expiry, and account/identity reference. Independent random values have at least 32 bytes of entropy where applicable. Expiry is five minutes. Never log temporary sensitive values.

Consume callback/link state and increment failed attempts atomically. Creation/linking uses transactions and database uniqueness; concurrent callbacks cannot duplicate/orphan users, move identities, or issue multiple sessions from one challenge. Recheck disabled status/role at final session issuance. Index expiry and perform bounded expired-record cleanup during issuance, without a new worker.

Down migration refuses restoring NOT NULL while Google-only users exist. Document deliberate rollback/account handling; never delete users, invent hashes, or silently remove their only login method.

## Security requirements

- Pin Google's issuer/provider endpoints from primary documentation. Requests cannot supply issuer, token/JWKS endpoint, or redirect origin.
- Prefer maintained OIDC/OAuth libraries over custom cryptography. New dependencies need a concrete purpose and pinned versions; verify current official documentation before implementation. Do not use public tokeninfo as a production verifier.
- Validate signature using Google keys, allowed algorithm, issuer, audience/client ID, expiry, nonce, subject, email format, and `email_verified = true`. Reject malformed or unexpected authorized-party claims. Unverified decoding is not authentication.
- Match Google subject before email. Changed email never transfers identity ownership or overwrites account email automatically. Password-email collisions require password proof; Google-only collisions fail safely, never auto-merge.
- Separate host-only HttpOnly OAuth/pending cookies from refresh cookies, with five-minute MaxAge and narrow auth paths. OAuth uses SameSite=Lax; production requires Secure. Clear temporary cookies on terminal failure/success/expiry without weakening refresh-cookie policy.
- Validate bounded internal `returnTo` server-side and client-side. Reject absolute/protocol-relative URLs, backslashes, controls, ambiguous encodings, and auth redirect loops. Derive redirect origins from validated configuration, never Host/forwarded headers/input.
- Never expose app JWTs/refresh tokens, Google ID/access tokens, secrets, verifier, or pending handles in URLs, browser storage, logs, analytics, audit details, or errors.
- Google's callback code/state necessarily arrives in its query string. Redact callback queries from app/proxy logging, avoid query-bearing metric labels and third-party callback resources, redirect immediately, and apply no-store/no-referrer.
- Bound provider HTTP timeouts/body sizes. Failed exchange/verification creates no account/session. Preserve server-side roles, account status, ownership, rotation, and logout checks. Document existing rate limiter deployment limits.

## Configuration and deployment

Add backend-only `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI`, and `AUTH_FRONTEND_ORIGIN`. All Google settings absent disables the feature without breaking existing deployments. Partial/invalid configuration fails startup without printing values.

Validate frontend origin against the exact existing allowlist; forbid credentials, paths, queries, and fragments. Production frontend/callback require HTTPS; HTTP is permitted only for loopback development. Callback uses `/api/v1/auth/google/callback` at public API ingress.

Document Google Web application client creation, consent audience/test users as applicable, and exact per-environment callback registration. Use the existing same-origin proxy/ingress; do not derive redirects from proxy headers or change topology.

Pass backend settings through development/production Compose using placeholders only. Add empty/example settings to `.env.example`; never read/print current `.env` values or put actual secrets in tracked files. Expose no Google secret through `VITE_*`.

## File responsibilities

- Backend `internal/auth`: account/identity/link rules and session reuse.
- A focused Google adapter: authorization URL, exchange, discovery/key handling, and validated identity behind a testable interface.
- `internal/postgres/authrepo` and new migration: nullable passwords, transactional identities, and expiring single-use challenges.
- `internal/http/handler`: thin Google handlers; router/config/`cmd/api` wire optional dependencies.
- Frontend `features/auth`: Google action, link/completion pages, typed APIs, destination validation, and tests. `AppRoutes.tsx` registers routes without unrelated changes.
- Authentication/deployment docs, README, OpenAPI, environment examples, Compose, and tracker: accurate contracts, setup, limits, and verification while preserving user edits.

## Acceptance and verification

1. New Google users receive USER accounts and normal app sessions without local passwords or tokens in URLs.
2. Returning users retain IDs, roles, reports, authorization, refresh, and logout behavior.
3. Email collisions cannot sign in/link/obtain sessions before correct password proof. Wrong, expired, replayed, cross-browser, or exhausted pending links fail safely.
4. Disabled accounts/invalid roles receive no sessions through either method.
5. Missing/wrong/expired/replayed state, invalid signature/nonce/issuer/audience/expiry/email, cancellation, provider outage, and database failures never produce success.
6. Concurrent callbacks/links preserve uniqueness, atomic creation, and single-use state.
7. Unsafe destinations are rejected; completion handles refresh/me errors without loops.
8. Password login/register, refresh rotation/replay, logout, protected/admin routes, and report ownership retain coverage.
9. Optional configuration preserves existing auth; partial configuration fails safely.
10. Google/link/completion loading/error/success states are labeled, keyboard accessible, and rendered at mobile/desktop widths using the existing design system.

Use focused failing-then-passing Go service/provider/handler/config tests and Vitest page/API/auth tests. Mock provider identity in domain tests and use a local HTTP provider fixture for exchange/token verification; automated tests require no live credentials.

Run migration/repository integration tests against disposable PostgreSQL via `TEST_DATABASE_URL`, including null scans, rollback refusal, concurrent uniqueness, expiry, and replay. Explicitly record skips if unavailable. Broaden to `go test ./...` from `apps/backend` and root `npm run typecheck`, `npm test`, and `npm run build`. Inspect rendered responsive/keyboard/error states; build success alone is insufficient. Review the diff for secrets, scope, and preserved edits.

Test real same-origin Google login only once the user configures credentials securely: callback/cookies, refresh/me, canceled consent, collision linking, and logout. If credentials/production access are absent, report live Google/production verification as unperformed.

## Next gate

Ask the user to review this written specification. After approval, create the implementation plan, record the feature in the tracker, and implement with focused tests. Do not claim Google login implemented or tested while this specification is the only change.
