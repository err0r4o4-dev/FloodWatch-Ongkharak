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

Apply migration `000020_external_evidence` before deploying the matching API and worker. Providers are disabled by default. Set the same explicit allowlist in API and worker, for example `EXTERNAL_DATA_PROVIDERS=THAIWATER_WATER,THAIWATER_RAIN,TMD_CAP`. The canonical official data-source enabled flag also gates imported station readings. This task does not activate providers, apply application migrations or restart running services.

Supported additional codes are `THAIWATER_GATE`, `OPENMETEO_ECMWF`, `OPENMETEO_GFS`, `OPENMETEO_ICON`, `OPENMETEO_ENSEMBLE`, `GLOFAS` and `HII_WRF`. Free Open-Meteo integrations additionally require `EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL=true`; confirm eligibility, attribution and provider terms before enabling. This acknowledgment is not a commercial subscription or legal approval. No undocumented DDPM or ONWR endpoint is invented; these remain unconfigured pending an independently verified contract.

Read-only live check (no database/object writes):

```powershell
go -C apps/backend run ./cmd/external-data-check -providers THAIWATER_WATER,THAIWATER_RAIN,TMD_CAP
go -C apps/backend run ./cmd/external-data-check -providers OPENMETEO_ECMWF,OPENMETEO_GFS,OPENMETEO_ICON,OPENMETEO_ENSEMBLE,GLOFAS -non-commercial
```

## Semantics and known limits

- ThaiWater adapters query Nakhon Nayok, retaining station location/district so provincial evidence is not mislabeled as an Ongkharak measurement. Public frontend endpoints are undocumented and may change. Rainfall 1-hour and 24-hour totals remain separate. Only 24-hour rainfall is projected to canonical rainfall readings.
- Water level in metres MSL is elevation, not flood depth. Only explicit MSL water readings are projected. Imported readings remain UNVERIFIED. Water-level rules for MSL/LOCAL require an explicit station and matching datum; no threshold is automatically seeded. Old water evidence cannot trigger a new live warning.
- Deterministic and ensemble rainfall use the preceding-hour accumulation window ending at the provider timestamp and retain ensemble members. GloFAS is river discharge in cubic metres per second, not rainfall or local flood depth. Grid forecasts are trend evidence, not street-level safety certification.
- HII currently supplies metadata and LZW GeoTIFFs. Files decode, but rainfall unit, timezone and accumulation semantics remain unconfirmed. Three selected grid records intentionally have null values and UNKNOWN freshness until those meanings are verified.
- TMD CAP uses CAP 1.2 documents linked from the official RSS feed. Applicable geographic evidence is retained. Update/Cancel references reconcile stored prior alerts independently of feed order. Area-less updates inherit geography only from referenced documents in the same fetch; update-only or explicitly nonmatching updates retain lifecycle references with UNKNOWN applicability rather than inventing an Ongkharak warning. An empty applicable result means no matching evidence returned, not an all-clear. It is not a verified DDPM feed.

Failures retain earlier evidence and persist sanitized provider health. Evidence is versioned/idempotent; retries do not duplicate canonical readings. There is no automatic raw-evidence retention purge: provision private storage capacity and define a reviewed retention policy before long-running production use. Never make the MinIO evidence bucket public.

Migration rollback removes the external evidence tables and datum/period metadata. All water-level threshold rules are disabled before removing datum metadata, and pending explicit-datum water events are marked evaluated, preventing imported MSL values from activating legacy unscoped thresholds. Rule/alert/reading history is retained; external update outbox events are removed. Existing raw archive objects are not deleted automatically. Backup first: rollback is data-lossy and does not recreate evidence on reapplication. Review every water-level rule and datum before manually re-enabling it after rollback/reapplication.

## Live verification on October 4, 2026

Read-only dry-run fetched and validated all ten registered providers. ThaiWater returned 14 water records, 43 rainfall records and 22 gate records; deterministic models returned 72 hourly records each, ensemble 3,672 member-hour records, GloFAS seven daily records, HII three semantically unverified records and TMD zero geographically applicable records. These counts are a point-in-time observation, not permanent guarantees. The Ongkharak water sample was dated September 29, 2026 and gate examples July 17, 2023: both must remain stale. Dry-run sample freshness is deliberately UNKNOWN because the command does not represent a persisted live snapshot.
