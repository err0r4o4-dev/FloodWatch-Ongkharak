# Community report areas and water trends

## Boundary

The situation map groups published community reports into stable report areas rather than aggregating only the records loaded in a viewport. Original reports, guarded media, publication policy, moderation, and owner permissions remain separate. Area summaries always have `COMMUNITY` provenance: they are not official observations, AI predictions, road-passability decisions, or proof of safety. No AI model, new provider, notification rule, or routing decision is introduced.

## Observation input

Migration 22 adds nullable `observed_at` and reporter-stated `water_trend`. The form asks when the incident was observed, separately from submission time, and offers rising, stable, falling, or unknown change. Unknown observation times remain null, including existing reports; the migration never backfills observation time from submission or edit time. Such reports remain available as originals but do not determine current depth or trend.

The API bounds known observation times to year 2000 or later and no more than 30 seconds ahead of server time for clock skew. A future observation is not eligible until its observation time has arrived. Omitted observation metadata on legacy edits preserves the existing value. Text and media edits advance the report revision without making old evidence fresh. Explicitly selecting “no flooding observed at this point” submits zero estimated depth; unspecified depth does not submit zero. A falling-water statement without a depth is not a low-depth measurement.

New media uploads receive a server-computed SHA-256 digest of the original bytes. The digest is internal, never a public ownership credential or media URL. Legacy media without a digest remains usable; no byte hash is fabricated.

## Spatial grouping

The initial radius is 150 metres from a fixed area anchor. A report selects the nearest eligible anchor, or creates a new one; existing anchors preserve area IDs. Membership cannot expand transitively from one report to the next. GeoJSON uses WGS84 `[longitude, latitude]`; the stored PostGIS anchor supports bounding-box reads.

This radius is an initial product parameter, not a GPS accuracy claim or a hydrological boundary. The first version has no reliable road-side or canal-barrier association, so proximity does not establish that reports are on the same road or water body. No place-name geocoding is invented. Area names are supplied community metadata and must be derived only from currently published members, with a generic fallback. Separate/merge administrator controls and barrier-aware grouping are not included in this version.

## Temporal aggregation

The current summary uses the rolling 60 minutes ending at computation time. Its comparison uses the immediately preceding, non-overlapping 60 minutes. Depth measurements are eligible only when the report is published, water-relevant (`FLOOD` or `ROAD`), and has a known observation time inside the relevant window. Other reports remain accessible as originals but cannot classify water depth.

Depth bands are:

| Band | Estimated depth |
| --- | --- |
| `LOW` | 0 to less than 10 cm |
| `MODERATE` | 10 to less than 30 cm |
| `HIGH` | 30 through 50 cm |
| `VERY_HIGH` | Greater than 50 cm |
| `UNKNOWN` | Missing, outdated, updating, or inconclusive evidence |

Only a strict majority of deduplicated depth samples establishes a band. One sample is explicitly provisional; a tie or lack of a majority remains unknown. A latest report does not automatically override multiple recent observations. The summary reports raw recent report count, deduplicated sample count, and total published member count; none means verified independent people.

Authenticated repeat observations are reduced to the latest per account per window. Exact normalized duplicate content and shared original-image hashes reduce repeated evidence; Guest display names are not identities. These mechanisms are not a guarantee against coordinated spam, changed wording, or re-encoded images.

Calculated trend compares known depth bands in adjacent windows. Reporter-stated trend is displayed separately. Missing or conflicting comparison evidence yields unknown calculated trend. The main pin uses the depth band; a separate falling indicator may be green while the main pin remains red for water deeper than 50 cm. Colour is accompanied by range/status text and direction symbols.

The area detail supplies 24 hourly windows, including explicit missing-data windows, and paginated original reports. Historical windows describe evidence within their own intervals, not present-day freshness or continuous measurements. All original reports remain stored beyond this initial 24-hour summary display. Removed evidence is excluded when rebuilding public summaries and timelines.

## Processing and public reads

Migration 23 adds stable area anchors, report memberships, derived timeline windows, and a durable refresh revision. Source report and media changes mark derived state dirty. The Go worker checks pending work every five seconds with a bounded attempt context, and periodically refreshes time-dependent summaries even without new reports. Transaction-scoped advisory locking serializes workers; deterministic source-row key-share locks precede membership foreign-key writes. The final revision compare-and-set occurs after those writes, avoiding inverted locks against report moderation/media transactions. A source revision change rolls back the candidate results for retry. Membership, summaries, timeline, processed revision, and identifier-only outbox events commit together. Events are emitted for changed public summaries/membership or a disappearing public area, not every unchanged or empty anchor on each periodic refresh.

Public reads fail closed while derived state is dirty, without exposing obsolete evidence identifiers, report-derived names, or counts. Area detail reads require a consistent database snapshot. Read-time expiry prevents a stopped worker from leaving an old `FRESH` depth indefinitely. The browser additionally refreshes and treats outdated/unavailable data as unknown. A failed job never creates a fabricated successful summary.

New public resources are:

- `GET /api/v1/report-areas`: bounded area list with optional `bbox`, `limit`, and `offset`.
- `GET /api/v1/report-areas/{id}`: current summary and 24-hour timeline.
- `GET /api/v1/report-areas/{id}/reports`: paginated canonical published originals, omitting private ownership and moderation fields.

The identifier-only `report-area.updated` signal uses `COMMUNITY`, resource type `REPORT_AREA`, and empty object payload. Clients refetch canonical REST; the event itself is not flood evidence. Existing `report.updated` and `report.verified` signals remain compatible. SSE failure does not turn absence into safety.

## Release and rollback

Apply migrations 1–23 before deploying the matching API and worker. Source edits alone do not migrate or restart the existing application. Migration 22 adds evidence metadata without rewriting historical observation times. Migration 23 initially queues a refresh for existing published reports; until processing completes, area results may be updating or empty rather than fabricated.

Before rolling back migration 23, stop area refreshes and report writes and drain unpublished `report-area.updated` events using a compatible dispatcher. The down migration refuses to discard pending events. It drops derived area data, not original reports or report media. Rolling back migration 22 removes saved observation metadata and media digests permanently; use an approved backup for populated deployments and matching application binaries. Never run these rollback operations against production as part of routine verification.

## Verification scope

Focused tests cover observation metadata, depth thresholds, adjacent windows, conflicting and missing evidence, deduplication, fixed-radius grouping, publication/removal lineage, public response privacy, API contracts, realtime invalidation, and affected React states. PostgreSQL integration uses `TEST_DATABASE_URL` pointing to an isolated synthetic database. Rendered verification must cover desktop/mobile, unknown/stale states, opening area details, keyboard dismissal, and the area-update form. Compilation alone does not establish flood accuracy or production readiness.

The initial worker rebuilds derived area information from retained source reports. Production-scale history and area-count performance must be measured before large deployments; incremental spatial processing and wider history controls are later work, not implied by this implementation.

Implementation verification used an isolated synthetic PostgreSQL database, not the running application database. Focused backend integration checks, Go vet/build, and 90 report-area/report/realtime frontend tests passed. Desktop and mobile rendered checks covered historical versus current depths, opening details, update navigation, and Escape dismissal. The broader affected frontend suite retains three pre-existing missing map-search assertions; standard TypeScript checking/build is blocked by unused `signingOut` and `handleLogout` symbols in `AppShell.tsx`. These unrelated failures were not hidden or weakened. Apply the migrations and restart matching API/worker binaries separately before live use.
