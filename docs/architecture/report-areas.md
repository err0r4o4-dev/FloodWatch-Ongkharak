# Community report areas and road-segment summaries

## Public boundary

Public report areas are currently road-segment summaries only. Each public area has
`areaType=ROAD_SEGMENT`, `radiusM=0` (deprecated compatibility), a WGS84
`LineString` geometry, `roadSegmentId` equal to the area ID, `segmentVersion >= 1`,
`segmentActive`, and the configured `startLabel`/`endLabel`. The midpoint remains
the GeoJSON `location`; the LineString is the spatial identity. Existing depth,
trend, freshness, counts, report IDs, `COMMUNITY` provenance, and UNKNOWN states
remain part of the response.

Legacy radius areas, memberships, and timelines remain in private storage for
compatibility and rollback inspection. They are not returned by the public area
list or detail APIs. The public list includes active segments with no reports and
returns UNKNOWN summaries rather than filtering by `totalReportCount`.

`GET /api/v1/report-areas` accepts bounded pagination and an optional WGS84 bbox;
the bbox intersects segment geometry, not only the midpoint. Detail can return an
archived segment version. Its timeline is recomputed from published reports
assigned to that exact immutable version; timelines never merge across versions.
`GET /api/v1/report-areas/{id}/reports` continues to expose the bounded original
published reports, including reports with no segment assignment. Derived summaries
never replace or delete original reports.

## Assignment and processing

Reports expose nullable `roadSegmentId` and
`roadSegmentAssignmentMethod` (`UNASSIGNED`, `USER_CONFIRMED`, or `LEGACY_MATCH`).
On input, a UUID confirms an active segment, `""` clears an assignment, and an
omitted value preserves an existing assignment only when coordinates are unchanged.
A coordinate change with an omitted value resets the report to UNASSIGNED; new
reports are UNASSIGNED unless the user explicitly confirms a segment. Assignment
validation uses the fixed 30 metre corridor and never changes report coordinates,
`observed_at`, or `updated_at` as a side effect.

Migration 25 marks existing reports `road_segment_assignment_pending=true` and
does not create a segment or fabricate coverage. The worker backfills only a
unique active match within 30 metres, records `LEGACY_MATCH`, and leaves zero or
multiple matches unresolved. It does not choose the nearest candidate when the
result is ambiguous. There are no fake seed segments; an empty segment catalog is
an UNKNOWN coverage state, not an official road status or a safety conclusion.

The rolling 60-minute summary compares the current window with the immediately
preceding non-overlapping 60-minute window. The existing 24-window timeline,
deduplication, publication filtering, depth bands, reporter trend, stale expiry,
conflict handling, and explicit missing-data UNKNOWN semantics are preserved.
Only published reports assigned to the exact segment version contribute to its
summary. Falling water or a low community depth is not a road-passability claim.

## Segment lifecycle

Administrators create segments through the road-segment API. A segment geometry is
a LineString no longer than 1000 metres along its existing road and within the
15-metre road corridor. Replacing a segment supplies `supersedesId`; the service
creates a new ID with the next version on the same road and archives the previous
version. Segment IDs and version history are immutable, so old report-area details
remain addressable without cross-version timeline merging.

Initially administrators must onboard segments before any road-segment coverage
appears. Segment definitions do not mutate official road conditions, which remain
a separate resource and provenance boundary.

## Migration and rollback

Apply migrations 24–26 before deploying matching API and worker binaries. This
documentation describes rollout intent only; it does not claim a live deployment,
application-database write, or service restart. Verification uses an isolated
synthetic PostgreSQL database.

Migration 26 adds report-area segment metadata while retaining legacy rows,
memberships, and timelines privately. Its down migration removes only derived
road-segment summaries; original reports remain unchanged. Before any down
migration, pause segment matching and report-area workers, stop report writes as
appropriate, drain compatible outbox work, and inspect populated immutable
segment history. The migration files must provide an explicit down-25 guard before
production rollback is considered; do not rely on a documentation warning alone.

## Verification status

Focused road-segment, report-assignment, report-area, and isolated PostgreSQL
checks are the relevant validation scope. Canonical YAML and embedded JSON must
normalize to the same OpenAPI document. No live provider data, official road
status, application database, or running service is used as evidence of coverage.
