# Database migrations

Migration 28 adds `NEWS_REPORT` / `MEDIA` evidence for opt-in, automatically published
FM91 news. Apply it before the matching API/worker and configure `FM91_NEWS` in both
provider allowlists. It reuses immutable external evidence tables and adds an indexed
current-news projection for SQL pagination and stable revision clocks; it does not seed
news, source coordinates, station readings or alert rules. Down is data-lossy for
media news only: stop FM91 writers, back up retained evidence, then downgrade matching
binaries and migration together. Private archive objects remain. See
`docs/architecture/fm91-news.md` for bounds, freshness and rollout details.

## Current segment rollout

Migrations 25 and 26 add administrator-defined road segments, report assignment
metadata, and road-segment-derived public report areas. Apply migrations 24–26
before deploying matching API and worker binaries. Migration 27 adds auditable
OpenStreetMap network import provenance and nullable provenance links on roads.
It creates no road or flood seed data; its down migration refuses populated
import provenance. Apply it before using `cmd/import-road-segments`.

On 2026-10-05, isolated PostgreSQL checks and migration preservation checks passed
before the backed-up local development database advanced to version 27. The local
stack now contains 2,098 real OSM roads and 4,514 immutable segments within limited
coverage, while retaining all 8 original community reports. This is local rollout
evidence, not authorization or verification for a production deployment. See
`docs/architecture/road-segments.md` for the retained snapshot, exclusions, backup,
operating commands, and verification boundaries.

Migration 25 creates `road_segments` and nullable report assignment columns, then
marks only existing reports `road_segment_assignment_pending=true`. It creates no
segment seed data. The worker may set `LEGACY_MATCH` only for a unique active
segment within 30 metres; zero or multiple matches remain unresolved. It never
guesses the nearest segment, and it does not change report coordinates,
`observed_at`, or `updated_at`. New reports omitted from assignment remain
`UNASSIGNED` and are not worker-mapped automatically.

Migration 26 adds segment metadata to `report_areas` while retaining legacy-radius
rows, memberships, and timelines privately. The public API exposes only
`ROAD_SEGMENT` areas, including empty active segments with UNKNOWN summaries;
legacy-radius summaries are not public. Down 26 removes only derived segment
summaries and leaves original reports unchanged. Before down 26, pause matching
binaries/workers and report-area refreshes, stop writes as required, and inspect
pending work. Before down 25, inspect the populated immutable segment history and
require an explicit migration guard; the current SQL must not be treated as safe
for populated rollback until that guard is verified.

Road-segment onboarding starts empty. Administrators or a validated real-data
import create immutable versions with
LineString geometry no longer than 1000 metres along an existing road and within a
15-metre corridor. No segment definition changes official road status or creates
official flood evidence. Missing segments, missing reports, stale data, and
unresolved assignment are UNKNOWN, never SAFE.

See `docs/architecture/road-segments.md` and
`docs/architecture/report-areas.md` for the public contract, rolling-window
semantics, assignment rules, and verification boundaries.

Migrations 22 and 23 add explicit report observation time/reporter trend, private media byte digests, and derived community report areas with a durable refresh revision and timeline. Apply migrations 1–23 before deploying the matching API and worker. Existing observation times remain unknown rather than being backfilled from submission time. Migration 23 queues initial processing and adds identifier-only `report-area.updated` events; old dispatchers do not understand these. Before rollback, stop area refreshes and report writes and drain pending area events with a compatible dispatcher. Down 23 refuses to discard unpublished area events and removes only derived areas; down 22 permanently removes saved observation metadata/digests. See `docs/architecture/report-areas.md`; verification must use an isolated synthetic database, never a production rollback.

Migration 21 adds append-only report moderation history, its public-read index, and identifier-only `report.updated` outbox signals. Deploy matching API/worker and frontend only after migrations 1–21. Pending reports become public under the new read policy; review any sensitive historical pending submissions before deployment. New removal/restoration requires a reason and current revision; restore revokes the old guest upload capability. For rollback, stop all report writes and drain unpublished `report.updated` events with a compatible dispatcher before downgrading binaries; then run down while writes remain stopped. Old binaries reject that event type and otherwise block the outbox. Down refuses when unpublished new events remain, deletes moderation history but preserves report rows/media/current status, and requires an approved backup for populated deployments. See `docs/architecture/community-reports.md`.

Migration 17 adds a per-device Web Push language (`th` or `en`), defaulting existing subscriptions to Thai. Apply it before deploying the locale-aware API and worker. Roll back those binaries before the down migration, which removes saved Push language preferences. Browser UI language storage is independent. See `docs/architecture/localization.md` for the language flow and update-only subscription ownership boundary.

Migration 14 adds nullable, normalized unique usernames, account authentication versions, and expiring single-use password-recovery challenges. Existing accounts retain current sign-in methods; no username is invented. Deploy the updated API only after applying it. Down migration 14 permanently removes usernames and recovery metadata; populated installations need an approved backup and matching application rollback. See `docs/architecture/password-recovery.md` for setup and release gates.

Migration 13 adds Google identities and five-minute OAuth/pending-link challenges, and allows null local passwords for Google-only accounts. Its down migration refuses rollback while Google-only accounts exist; explicitly provision valid local credentials through a reviewed administrative procedure before rollback, never delete users or fabricate hashes. See `docs/architecture/google-login.md` for deployment ordering and account policy.

The selected migration system is `golang-migrate/migrate` `v4.18.3`. Migrations use its versioned `NNNNNN_description.up.sql` and `.down.sql` convention. The root Compose `migrate` service owns execution and records applied versions in `schema_migrations`; no Go migration dependency is required.

Migration 6 adds community reports and media metadata. Its PostGIS `Point` column is constrained to SRID 4326, valid non-empty longitude/latitude coordinates, and has a GiST index for bounding-box reads. Rolling migration 6 down permanently deletes report records and media metadata; it does not delete S3-compatible objects, which must be removed separately under an approved retention procedure.

Migration 7 adds structured alert rules, deduplicated alerts, public event outbox, and canonical-write triggers. No rules or official water thresholds are seeded. Down migration 7 destroys alert history and queued events and must not be used against populated environments without an approved backup and recovery plan.

Migration 8 refines the road event trigger to notify clients when the current effective status changes to or from `UNKNOWN`, including unverified evidence. It replaces the trigger function without rewriting prior records; its down migration restores the migration 7 behavior.

Migration 9 restores per-road serialization for concurrent road-condition inserts. It uses `FOR NO KEY UPDATE` to avoid conflicting with the foreign-key key-share locks held by concurrent inserts; its down migration restores the migration 8 function. It does not rewrite prior records.

Run migrations from the repository root after the PostgreSQL Compose service is ready:

```powershell
docker-compose --profile tools run --rm migrate
```

Migration 1 enables PostGIS; migration 2 creates auth tables; migration 3 creates the seven Phase 2 flood-domain tables and spatial/time indexes. Migration 4 adds finite-number checks to measurements and confidence after migration 3 was applied; an existing database containing non-finite values must be inspected and remediated before applying it. Migration 5 adds ingestion runs and source health fields. Down migration 5 removes those run records and health fields, so use it only with an approved backup plan for a populated environment. Down migration 4 removes only finite-number checks. Down migration 3 drops the flood-domain tables and their data (in dependency order). The migration 1 down intentionally leaves PostGIS installed because other spatial objects may depend on it.
