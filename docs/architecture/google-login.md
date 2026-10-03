# Google login

Google is an optional identity provider. Flood Watch owns all accounts, roles and sessions. Google authorization uses a backend redirect with state, nonce and S256 PKCE; the maintained Go OIDC verifier checks signed RS256 ID tokens, issuer, audience, expiry, nonce, issued time, subject, verified email and authorized party. No Google access or refresh token is kept.

## Account policy

- New Google accounts have USER role and a null local password; password login rejects them generically.
- Returning accounts are found by Google's stable subject, not email. Google email changes do not overwrite local email or transfer ownership.
- Matching an existing password account creates no session until the existing password is verified on `/auth/google/link`.
- Five wrong passwords exhaust a pending link. Linking is bound to its HttpOnly cookie and expires after five minutes. It cannot accept a caller-supplied account ID or email.
- Disabled accounts and invalid roles cannot obtain sessions. Existing permissions, report ownership, refresh rotation and logout continue to apply.

## Session flow

1. GET `/api/v1/auth/google/config` reports only feature availability.
2. GET `/api/v1/auth/google/start` creates PostgreSQL state and sets a host-only HttpOnly SameSite=Lax OAuth cookie before redirecting to Google.
3. GET `/api/v1/auth/google/callback` atomically consumes the matching state/browser transaction, exchanges the code, verifies identity, and resolves or creates the account.
4. Email collisions set a pending-link cookie instead of a refresh session. POST `/api/v1/auth/google/link` verifies the existing password and atomically links the identity.
5. Success sets the ordinary refresh cookie and navigates to `/auth/complete`; React restores the session with refresh/me and keeps its app access token only in memory.

The OAuth callback deliberately accepts Google's cross-site top-level navigation, guarded by browser-bound single-use state instead of the ordinary Origin check. Start rejects cross-site initiation; link POST requires an approved Origin. OAuth cookies use Lax independently of the existing configurable Lax/Strict refresh cookie. Production cookies require Secure.

Challenges are stored in PostgreSQL with hashed state/browser/pending handles, temporary nonce/verifier, validated internal destination, expiry and attempt count. Provider subject and per-account identity uniqueness, row locks, and per-subject transaction locks prevent duplicate accounts, identity transfer and replay. Expired challenges are deleted in bounded batches during issuance. Existing in-memory rate limits remain per replica/direct peer; retain trusted edge rate limits behind a proxy.

## Setup

Create a Google OAuth client of type **Web application** in Google Auth Platform. Configure its consent audience and test users as needed. Enable only `openid email`, and register the exact public callback; do not use the Vite-only host unless it also proxies `/api`.

All three backend settings must either be absent or valid:

```env
GOOGLE_OAUTH_CLIENT_ID=<web-client-id>
GOOGLE_OAUTH_CLIENT_SECRET=<web-client-secret>
GOOGLE_OAUTH_REDIRECT_URL=http://localhost/api/v1/auth/google/callback
```

The example uses the development Caddy origin. When the callback origin is in `CORS_ALLOWED_ORIGINS`, the backend uses it as the frontend origin. Otherwise, exactly one allowed frontend origin is required, with the same scheme and hostname as the callback; only the port may differ. This keeps host-only cookies usable without a separate frontend-origin setting or inference from request headers. Ambiguous frontend lists, different hostnames/schemes, invalid callback paths, and credentials/query/fragment are rejected at startup. Production requires HTTPS; only loopback development may use HTTP. Existing deployments must rename the previous Google environment variables to these three names when deploying the updated API.

For the local frontend on Caddy port 5173 with the callback directly on API port 8080:

```env
GOOGLE_OAUTH_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback
CORS_ALLOWED_ORIGINS=http://localhost:5173
```

Keep `CADDY_HTTP_PORT=5173`, `API_HOST_PORT=8080`, and the direct Vite `WEB_PORT` on a different available port (for example 15173). Open `http://localhost:5173/login`; frontend API requests still go through Caddy, while Google redirects to port 8080. Register `http://localhost:8080/api/v1/auth/google/callback` as the exact authorized redirect URI in Google Console. Do not add the API origin to the CORS list for this setup, because matching the callback origin selects it as the frontend destination. Switching to a single-origin setup remains supported by registering a callback on the frontend origin instead.

Configure real values only in your untracked local environment or deployment secret store. Never paste them into source, browser `VITE_*` settings, logs or tickets. Settings are passed only to the API; missing configuration disables Google without breaking password auth. Partial configuration fails startup without showing its values.

Apply migrations through `000013_google_auth` before deploying the changed API. Use the existing migration workflow documented in `apps/backend/migrations/README.md` (`docker compose --profile tools run --rm migrate`), then rebuild/recreate the development API with `docker compose up -d --build api` when ready. Reload the development proxy with `docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile` to apply log redaction. Production deployment must load the updated production proxy configuration as well. This implementation does not apply migrations or reload the running application's services automatically.

Migration 000013 introduces nullable local passwords, external identities, and temporary state/link tables. Its down migration refuses rollback while any Google-only accounts exist. Explicitly provision valid local credentials through a reviewed administrative procedure before rollback; never delete accounts or fabricate hashes merely to satisfy NOT NULL.

## Redirects, errors and operations

Destinations are bounded internal paths, defaulting to `/reports`; external URLs, backslashes, encoded ambiguity, control characters and auth loops are rejected. Redirect origins are configured, never inferred from request/proxy headers. Cancellation, state failure and provider outage are shown with safe codes, not raw provider errors. A failed frontend refresh/me never displays success.

OAuth responses use no-store/no-referrer. The API logger records paths only, not callback queries. Both Caddy configurations filter the default structured logger: remove callback code/state/error_description query values and Cookie/Authorization/Set-Cookie fields. Current configurations do not enable access logs; preserve equivalent redaction in any new access logger or external log pipeline. Never export callback codes/state or JWTs as log/metric labels. Production `/api/*` forwarding preserves the full Go path rather than stripping `/api`.

Automated tests use a local signed-token provider fixture and disposable PostgreSQL, never live credentials. A real Google round trip still requires a securely configured client and registered callback. Verify new/returning users, collisions, canceled consent, cookie attachment, refresh/me and logout before release. Browser/layout and full-suite failures must be reported separately from focused test results.
