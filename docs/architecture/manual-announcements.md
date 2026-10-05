# Administrator-written media announcements

`/admin/announcements` provides draft creation, source references with individual reported times, preview, explicit publication, updates, withdrawal with a reason, status filters and retained revision history. System labels support Thai and English; authored text is never automatically translated. Date inputs use Gregorian years and explicit Asia/Bangkok conversion independent of the browser timezone.

Manual announcements remain COMMUNITY provenance with ADMIN publisher and MEDIA references. Administrator publication does not promote a newspaper report to OFFICIAL evidence. The existing official TMD panel and rule-generated alerts remain separate. No Push, email, realtime alert-created event, map status or road-condition write occurs.

Drafts may be incomplete. Publication and published updates require title, summary, body, area, reported situation time, future expiry, and at least one named public HTTP(S) source with its own reported time. Missing or future source observations, expired evidence and withdrawal yield UNKNOWN, never SAFE. Source time, manual recording time, publication time and update time remain distinct. Public cards recompute expiry while open, expose source citations, preserve archive labels and hide historical body/advice behind details. Public data is periodically re-read; failed or stalled reads show unavailable rather than retaining a current-looking warning.

Only authenticated ADMIN users can write. Requests have bounded JSON, pagination, read/write rate limits and five-second service deadlines. Mutations require the expected revision; stale requests return 409. Changes, full immutable revision snapshots and audit entries commit in one transaction. Drafts and private history are excluded from public reads. Source URLs are links only; the backend does not fetch them. No live flood facts or news URLs are seeded.

## Deployment

Apply `apps/backend/migrations/000024_manual_announcements.up.sql` after the existing migrations before deploying the matching API and web application. This work does not apply migrations, restart running services or publish the user's supplied news. Use the normal deployment procedure and back up retained data first. The down migration refuses rollback while manual-announcement audit records exist; do not delete audit history to bypass this safeguard. Rolling back the tables otherwise removes retained announcements and revisions.

API contracts are in `docs/api/openapi.yaml`: public `GET /api/v1/manual-announcements`, administrative list/detail/create/update and explicit publish/withdraw endpoints. All manual reads use no-store caching. No physical delete endpoint is provided.
