# External evidence ingestion

## Flow

```text
Configured official/model providers
  -> bounded worker polling (two concurrent fetches, 90-second timeout)
  -> fetch, validate, normalize, preserve timestamp/unit/datum
  -> private content-addressed raw payload archive in MinIO
  -> transactional PostgreSQL evidence, health and outbox
  -> eligible official station readings only
  -> external_data.updated SSE
  -> GET /api/v1/external-data/ongkharak
  -> bilingual overview evidence panel
```

The public API reads stored snapshots, never calls providers on behalf of a visitor. Raw bodies, storage keys and raw reference metadata are not public. Forecast and gate evidence does not become a water-level measurement. MODEL is distinct from OFFICIAL, COMMUNITY and AI. Source fetch health and each record's freshness are evaluated separately; a successful GET does not make an old observation current. Missing, stale, malformed or semantically unconfirmed evidence is UNKNOWN, not SAFE. The panel refreshes every 60 seconds while visible; failed or 15-second stalled snapshot requests null retained values and mark evidence UNKNOWN rather than retaining current-looking badges indefinitely.

## Configuration and deployment boundary

### Ongkharak station scope

ThaiWater water, rain and gate adapters retain normalized station evidence only when the provider's district is `องครักษ์` and province is `นครนายก`, after trimming whitespace. Missing or contradictory labels are excluded; a station name mentioning Ongkharak or coordinates near the map center cannot establish district membership. The existing upstream requests still use `province_code=26`; no unverified district query parameter or administrative polygon is invented. Original provincial responses remain unchanged in the private raw archive for audit.

Worker validation also rejects out-of-scope normalized ThaiWater stations. The snapshot repository filters older provincial evidence before counting and applying the record limit, so `recordCount` covers only eligible local evidence. Canonical ThaiWater projection accepts only local stations; public station lists, details and readings exclude historical imported stations outside this scope. Pending water-level evidence from those stations cannot create a new threshold alert. Existing records, station/reading history and previously created alerts are not deleted or rewritten. An empty local result is missing evidence, not an all-clear; freshness rules remain unchanged.

This scope change is for external station evidence, not a new district-wide geofencing policy for community reports, manually entered resources or road/risk layers. Model weather/forecasts remain fixed-reference-point or nearby grid evidence, not measurements for every location in the district. TMD provincial warnings that cover Ongkharak remain relevant and preserve their original provincial codes/text; they are not relabeled district-specific. Applying the change requires deploying matching API and worker binaries, but no new migration or dependency. This change does not restart services or modify provider enablement.

Retained TMD evidence now also feeds the separate public official-announcement lifecycle endpoint and panels described in `official-announcements.md`. This preserves update/cancel/expiry history without creating app-generated alerts or sending Push/email.

Apply migration `000020_external_evidence` before deploying the matching API and worker. Providers are disabled by default. Set the same explicit allowlist in API and worker, for example `EXTERNAL_DATA_PROVIDERS=THAIWATER_WATER,THAIWATER_RAIN,TMD_CAP`. The canonical official data-source enabled flag also gates imported station readings. This task does not activate providers, apply application migrations or restart running services.

Supported additional codes are `THAIWATER_GATE`, `OPENMETEO_ECMWF`, `OPENMETEO_GFS`, `OPENMETEO_ICON`, `OPENMETEO_ENSEMBLE`, `GLOFAS` and `HII_WRF`. Free Open-Meteo integrations additionally require `EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL=true`; confirm eligibility, attribution and provider terms before enabling. This acknowledgment is not a commercial subscription or legal approval. No undocumented DDPM or ONWR endpoint is invented; these remain unconfigured pending an independently verified contract.

Read-only live check (no database/object writes):

```powershell
go -C apps/backend run ./cmd/external-data-check -providers THAIWATER_WATER,THAIWATER_RAIN,TMD_CAP
go -C apps/backend run ./cmd/external-data-check -providers OPENMETEO_ECMWF,OPENMETEO_GFS,OPENMETEO_ICON,OPENMETEO_ENSEMBLE,GLOFAS -non-commercial
```

## Semantics and known limits

### TMD polling and duplicate checks

The TMD client bundles the public GlobalSign GCC R6 AlphaSSL CA 2025 intermediate because the observed TMD TLS endpoint omitted it. Certificate source: `https://secure.globalsign.com/cacert/gsgccr6alphasslca2025.crt`; DER SHA-256: `A883559231F8388DAF35CE41C8101040AE8FD9B656434247B9475AF592CC08CA`. This supplements only the TMD client's chain, not the container or other providers' trust stores. Normal TLS verification stays enabled and a second verification requires a path to the original system roots, the configured hostname, server-auth usage and valid certificate dates. The bundled intermediate cannot act as the final trust anchor. No runtime certificate download or TLS bypass is used. The intermediate expires on 2027-05-21; recheck the provider chain when it rotates and remove or update this workaround as appropriate. The observed leaf expires on 2026-10-09; an expired leaf must still fail closed.

TLS regression tests use synthetic certificates and cover the omitted-intermediate success case, an untrusted root, wrong hostname, expired leaf and malformed certificate input. A GET-only live check on 2026-10-05 Bangkok time fetched the RSS plus four CAP documents successfully (five raw payloads, zero eligible current records); this does not imply an all-clear for flood risk.

When `TMD_CAP` is enabled, the worker polls once on startup, checks scheduling every minute and attempts TMD again when its ten-minute interval is due. The source cannot overlap within one service; the database also prevents two active runs for the same source. A failed attempt remains visible and waits until the next due attempt rather than causing an immediate retry loop. Individual provider HTTP retries remain bounded.

Repeated feed links are downloaded once per attempt. Different links carrying the same parsed CAP identifier/message type/content produce one evidence record while retaining both raw documents. Conflicting documents with the same identifier/message type in one feed fail the attempt instead of selecting an arbitrary version. Across successful polling attempts, unchanged normalized evidence reuses its existing database record; changed content retains a separate immutable version. Every successful attempt still has its own audit batch/raw links and `external_data.updated` snapshot invalidation. These invalidations refresh the separate official-announcement read model, not user notifications; Push/email delivery remains separate integration work.

Focused regressions cover the exact ten-minute boundary, outage recovery, overlapping polls, duplicate aliases, conflicting duplicates, repeated persisted CAP content and revised versions. On October 4, 2026, all twelve external-repository integration tests passed without skips against a fresh disposable PostGIS database with migrations 1–20. Two read-only live TMD checks also succeeded, each with five raw payloads and zero applicable records; the dedup assertions use explicitly synthetic alerts, not fabricated live warnings. Race-detector execution was unavailable because the current Go environment does not enable cgo. This follow-up does not modify `.env`, migrate the application database, restart services or send a notification.

- ThaiWater adapters query Nakhon Nayok then retain only confirmed Ongkharak station evidence as described above. Public frontend endpoints are undocumented and may change. Rainfall 1-hour and 24-hour totals remain separate. Only 24-hour rainfall is projected to canonical rainfall readings.
- Water level in metres MSL is elevation, not flood depth. Only explicit MSL water readings are projected. Imported readings remain UNVERIFIED. Water-level rules for MSL/LOCAL require an explicit station and matching datum; no threshold is automatically seeded. Old water evidence cannot trigger a new live warning.
- Deterministic and ensemble rainfall use the preceding-hour accumulation window ending at the provider timestamp and retain ensemble members. GloFAS is river discharge in cubic metres per second, not rainfall or local flood depth. Grid forecasts are trend evidence, not street-level safety certification.
- HII currently supplies metadata and LZW GeoTIFFs. Files decode, but rainfall unit, timezone and accumulation semantics remain unconfirmed. Three selected grid records intentionally have null values and UNKNOWN freshness until those meanings are verified.
- TMD CAP uses CAP 1.2 documents linked from the official RSS feed. Applicable geographic evidence is retained. Update/Cancel references reconcile stored prior alerts independently of feed order. Area-less updates inherit geography only from referenced documents in the same fetch; update-only or explicitly nonmatching updates retain lifecycle references with UNKNOWN applicability rather than inventing an Ongkharak warning. An empty applicable result means no matching evidence returned, not an all-clear. It is not a verified DDPM feed.

Failures retain earlier evidence and persist sanitized provider health. Evidence is versioned/idempotent; retries do not duplicate canonical readings. There is no automatic raw-evidence retention purge: provision private storage capacity and define a reviewed retention policy before long-running production use. Never make the MinIO evidence bucket public.

Migration rollback removes the external evidence tables and datum/period metadata. All water-level threshold rules are disabled before removing datum metadata, and pending explicit-datum water events are marked evaluated, preventing imported MSL values from activating legacy unscoped thresholds. Rule/alert/reading history is retained; external update outbox events are removed. Existing raw archive objects are not deleted automatically. Backup first: rollback is data-lossy and does not recreate evidence on reapplication. Review every water-level rule and datum before manually re-enabling it after rollback/reapplication.

## Live verification on October 4, 2026

The following dry-run counts predate the Ongkharak station filter and include provincial records; they are not the current local record counts.

Read-only dry-run fetched and validated all ten registered providers. ThaiWater returned 14 water records, 43 rainfall records and 22 gate records; deterministic models returned 72 hourly records each, ensemble 3,672 member-hour records, GloFAS seven daily records, HII three semantically unverified records and TMD zero geographically applicable records. These counts are a point-in-time observation, not permanent guarantees. The Ongkharak water sample was dated September 29, 2026 and gate examples July 17, 2023: both must remain stale. Dry-run sample freshness is deliberately UNKNOWN because the command does not represent a persisted live snapshot.

## Station-scope verification

Fresh full backend tests, vet and command builds pass. All 25 affected PostGIS integrations pass against separate fresh databases for external evidence (18), flood repository (1) and outbox (6), with migrations 1–21; the first shared-database attempt exposed cross-package leftover outbox fixtures, so no unrelated tests were weakened or changed. Regressions cover mixed/missing/contradictory geographic labels for all three ThaiWater adapters, private raw preservation, pre-limit local counts, no-local-evidence results, canonical projection, historical station/detail/read isolation and out-of-district alert-evidence exclusion. Canonical/embedded OpenAPI contracts match. Fixtures are synthetic; this verification does not claim live upstream success or deployment. No frontend code, provider settings, running services or application database were changed by this scope work.
