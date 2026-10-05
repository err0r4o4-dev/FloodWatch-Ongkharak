# Road-segment contract and rollout

## API

`GET /api/v1/road-segments` returns a bounded active-only list with `limit`,
`offset`, and optional WGS84 `bbox=minLng,minLat,maxLng,maxLat`. Bbox filtering
intersects the stored LineString geometry, not merely its midpoint. The response
contains `{items, limit, offset}`; an empty list means no configured coverage.

`GET /api/v1/road-segments/candidates?longitude=...&latitude=...` returns segment
items with `distanceM`, `assignmentStatus` (`UNASSIGNED`, `SINGLE_CANDIDATE`, or
`AMBIGUOUS`), and `matchingDistanceM=30`. Candidates are active segments within
30 metres. Multiple matches are ambiguous; the API does not select a nearest
segment on behalf of a reporter.

`POST /api/v1/road-segments` is ADMIN-only and remains subject to the existing
bearer authentication, browser origin, and CSRF protections. Input requires
`roadId`, `name`, `startLabel`, `endLabel`, and a WGS84 GeoJSON LineString;
`supersedesId` is optional. Geometry must be no longer than 1000 metres along the
existing road and stay within its 15-metre corridor.

The response segment contains `id`, `roadId`, labels, LineString `geometry`, Point
midpoint `location`, `version`, nullable `supersedesId`, `active`, and `createdAt`.
Replacing an active segment creates a new immutable ID with the next version on
the same road and archives the superseded version. There are no fake seed records.

## Report and area relationship

Report input accepts an optional `roadSegmentId`: a UUID confirms the assignment,
`""` clears it, and omission preserves an existing assignment only when the
coordinates are unchanged. A coordinate change with omission resets the report;
new reports omitted from assignment are `UNASSIGNED`. Public reports expose the
nullable ID and `roadSegmentAssignmentMethod` (`UNASSIGNED`, `USER_CONFIRMED`, or
`LEGACY_MATCH`).

Public report areas use only `areaType=ROAD_SEGMENT`. `roadSegmentId` equals the
area ID; `segmentVersion`, `segmentActive`, labels, LineString geometry, and
deprecated `radiusM=0` identify the segment version. Empty active segments remain
visible with UNKNOWN evidence. Archived detail remains addressable, but timelines
use only published reports assigned to that exact version and never merge across
versions. Original reports remain available through the original reports API.

## Operations

Migration 25 marks existing reports pending and the worker assigns only a unique
active match within 30 metres as `LEGACY_MATCH`. It leaves ambiguous/unmatched
reports unresolved and never modifies coordinates or observation/update times.
Migration 26 keeps legacy-radius rows and derived history private while public
reads move to segment summaries. Down 26 removes derived segment summaries only;
original reports remain. Pause matching binaries/workers before rollback and
inspect the populated immutable history before down 25; the migration guard must
be verified before production rollback.

Initial onboarding has no segments until an ADMIN creates them or an operator
imports a validated real road-network snapshot. No missing data or coverage is
interpreted as SAFE, and segment definitions do not mutate official road status.

### Real OpenStreetMap onboarding

Migration 27 records import provenance and links imported parent roads to it.
`cmd/import-road-segments` reads a retained OSM JSON snapshot, checks WGS84 bounds,
uses actual road names or an explicit unnamed-way label, splits at shared OSM
junction nodes, and subdivides longer spans along their original geometry.
PostGIS validates every parent and segment. Replaying an identical snapshot is
idempotent; changed immutable extents require deliberate replacement, not an
automatic geometry overwrite. The importer never creates flood reports or road
condition observations.

The Compose `road-import` tools service receives the existing backend environment
without printing credentials and mounts `data/road-network` read-only. Dry-run is
the default; database writes require `--apply`. Capture its JSON stdout on the host
as the manifest; do not use `--manifest` against this read-only container mount.

Example dry-run from the repository root:

```powershell
docker compose --profile tools build road-import
docker compose --profile tools run --rm road-import --input data/road-network/ongkharak-osm-2026-10-05.json --bbox 100.98,14.02,101.08,14.14 --source-url "https://api.openstreetmap.org/api/0.6/map.json?bbox=100.98,14.02,101.08,14.14" --retrieved-at 2026-10-05T08:46:04Z --raw-snapshot-path data/road-network/ongkharak-osm-2026-10-05.json
```

OpenStreetMap contributors supply the geometry under ODbL-1.0. It is not official
flood or passability evidence. Keep the raw snapshot, SHA-256, retrieval timestamp,
bbox, and exclusion counts. The map and admin screen retain OSM attribution.

## Implementation status — 2026-10-05

Segment APIs, administrator definitions, explicit report assignment, immutable version
summaries, dashed map extents, and segment timelines are implemented. Focused frontend
verification now passes 42 tests across the map, segment admin/API, report form,
segment layer, and area details; earlier mocked-data checks also covered desktop
1440 × 1000 and mobile 390 × 844. Six focused backend packages pass with PostgreSQL
integrations in a fresh isolated database. OpenAPI consistency checks, Go vet,
API/worker/importer container builds, and the Vite bundle build pass.

The local development stack at `http://localhost:5173` now runs migrations 1–27
and updated API/worker binaries. Before rollout, the local database was backed up
to `data/backups/before-road-segments-2026-10-05.dump`. Migration and PostgreSQL
integration checks ran against an isolated synthetic database first, including
migration 26 round-trip preservation of original reports.

Real OSM snapshot `data/road-network/ongkharak-osm-2026-10-05.json` was retrieved
on 2026-10-05 at 08:46:04 UTC with SHA-256
`0b5bc3fa7dd00128de001e76ceb646cb12961db50933707275624764a669f969`.
Its bbox is `[100.98,14.02,101.08,14.14]`, limited local coverage rather than the
entire district. Dry-run normalized 2,099 roads / 4,515 segments. PostGIS accepted
2,098 roads / 4,514 segments and rejected one invalid parent; 360 non-motor-road
ways and 11 restricted service ways were excluded during normalization. The host
retains `.manifest.json`, `.apply.json`, and `.replay.json` beside the snapshot.
Replaying that snapshot with the final importer reused all 2,098 roads and 4,514
segments without adding duplicates. Renaming or resegmenting an imported way and
same-hash/different-bbox provenance conflicts are rejected and integration-tested.

Live checks confirm 4,514 segment summaries, 108,336 hourly timeline rows, all
8 original reports retained, and no invented road-condition observations.
Two existing reports matched uniquely; unmatched or ambiguous reports remain
unassigned. Empty evidence remains UNKNOWN / COMMUNITY. A full synthetic refresh
with this network completed in approximately 10 seconds, below the worker's
30-second attempt timeout. The regular fixture suite uses a separate empty test
database: its five-second concurrency checks and global-list assumptions are not
a network-load benchmark and failed when run against the populated workload DB.
The fresh fixture suite passes; no valid test assertions were weakened.

Actual desktop 1440 × 1000 and mobile 390 × 844 checks verify map selection opens
the details panel directly, 24 timeline windows load, the bounds-list button is
absent, and the report form offers explicit nearby segment selection. No test
report was submitted to the application database. Unauthenticated segment creation
returns HTTP 401. Admin creation is covered by focused automated tests; a real
authenticated administrator session was not exercised during rollout.

Standard frontend typechecking remains blocked by the existing unused `signingOut`
and `handleLogout` declarations in `AppShell.tsx`; the Vite bundle build is not a
substitute for that typecheck. Some existing React tests also emit act warnings.
