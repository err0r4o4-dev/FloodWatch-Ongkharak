# Username login and password recovery

Migration 14 adds nullable usernames, account authentication versions, and password-recovery challenges. Registration requires email, username, and password. Usernames are trimmed and normalized to lowercase ASCII letters/digits/underscores of length 3–32 and are globally unique. Login accepts email or username through the existing `email` JSON field. Existing and Google-only users retain their current sign-in methods and no username is invented. Adding usernames to existing accounts requires a separately approved account-management change.

Registration and automatic login are separate requests: the UI explicitly reports an already-created account and supplies a login link if automatic login fails. The safe internal `returnTo` is preserved. Registration and reset retain the existing 12–128 UTF-8-byte password policy and Argon2id hashing.

## Recovery contract

`/forgot-password` uses email → OTP → new/confirm password → persistent success with a user-activated login button. There is no timeout simulation, automatic login, or automatic success redirect. Editing the email or restarting clears grants and passwords. Confirmation is checked locally and never sent to the API. Failures never produce success notifications. Grants remain in React memory only, never in URLs, logs, or browser storage.

- `POST /api/v1/auth/password/request-otp` accepts `{email}`. SMTP acceptance returns an opaque UUID `challengeId`, `expiresIn: 600`, `resendAfter: 60`, and a neutral notice, never an OTP or user identity. All valid submitted emails receive account-neutral mail, including unknown, disabled, and Google-only accounts. Ineligible accounts have no user-bound challenge and cannot verify or reset. This avoids an account-existence signal through SMTP delivery timing, at the cost of decoy mail; keep abuse limits enabled.
- Codes are six cryptographically random digits. PostgreSQL stores purpose-separated HMACs of the email and challenge ID plus OTP, keyed by the server secret. Challenges expire after ten minutes and allow five well-formed failed attempts. An accepted resend invalidates the old code, challenge ID, and grant. Changing `JWT_SECRET` invalidates outstanding OTPs.
- `POST /api/v1/auth/password/verify-otp` accepts `{challengeId,otp}`. One successful verification creates a 32-random-byte, five-minute reset-only grant, not a login token. PostgreSQL stores only its SHA-256 digest. Concurrent verification cannot create another grant.
- `POST /api/v1/auth/password/reset` accepts `{resetToken,password}`. A transaction rechecks eligibility, expiry, single use, and account version, updates the Argon2id hash, consumes the grant, revokes every refresh session, and increments `users.auth_version`. It clears the browser's old refresh cookie and returns an empty 204 without logging in. Session creation locks the account and verifies the password-proof version before inserting. Refresh and reset also acquire account locks before session locks, so late old-password logins cannot survive reset.

Requests are serialized per email in PostgreSQL and limited to one per 60 seconds and five per hour across replicas. Direct-client limits protect request (5/hour), verify (30/minute), and reset (10/minute). For a reverse proxy, configure `TRUSTED_PROXY_CIDRS` with only the actual ingress IP/subnet. Recovery accepts `X-Forwarded-For` only from these trusted peers, parses right-to-left, and ignores attacker-supplied prefixes/untrusted peers. The default empty setting uses the direct peer; clients behind an unconfigured proxy share its budget. Never broadly trust public/private ranges or arbitrary forwarded headers. Existing non-recovery limiters retain their documented direct-peer behavior.

Recovery enforces the existing origin policy, rejects unknown/trailing JSON fields, bounds request bodies, and uses `Cache-Control: no-store`. Diagnostic errors and SMTP responses are not exposed to clients. OpenAPI YAML is canonical and its embedded runtime JSON must match.

## SMTP setup

Set `SMTP_HOST`, `SMTP_PORT` (default 587), `SMTP_FROM` (plain email), optional paired `SMTP_USERNAME`/`SMTP_PASSWORD`, and `SMTP_REQUIRE_TLS` (default true). Delivery uses verified STARTTLS with TLS 1.2 or newer, context cancellation, and a ten-second connection deadline. Port 465 implicit TLS is not supported; use the provider's STARTTLS port. Only an explicitly configured loopback test SMTP server may use `SMTP_REQUIRE_TLS=false`; production rejects it. Never place real credentials in tracked files or expose test mailboxes publicly.

Unconfigured SMTP leaves login/registration available but OTP requests return `503 RECOVERY_UNAVAILABLE` before allocating a challenge. Delivery failure also returns 503, not simulated success; the challenge/request cap may already have changed, so retry may require waiting. SMTP acceptance does not prove inbox delivery or account eligibility.

## Deployment and rollback

Apply migration 14 before deploying the matching API and web bundle. Securely configure SMTP and trusted ingress CIDRs, reload the intended services, and verify a real mailbox round trip. Source implementation/testing does not migrate application/production data, deploy services, configure real credentials, or send real email automatically.

Rollback permanently removes usernames, authentication versions, and recovery grants. Use an approved backup procedure for populated databases and roll application code back together. Expired recovery rows should be purged under the approved database-retention procedure while preserving the active hourly rate-limit window.
