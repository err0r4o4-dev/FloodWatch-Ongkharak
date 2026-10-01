# Data trust and provenance

Canonical source types are `OFFICIAL`, `COMMUNITY`, and `AI`, constrained in `data_sources` and joined into public observations. Neither community nor AI evidence becomes official automatically. Water, flood, risk, and road responses carry the relevant source, observation/receipt time, freshness, verification, and nullable confidence without manufacturing absent values. Do not infer safety from an absence of reports. See `flood-domain.md` for exact freshness policy and raw/effective status separation.

`UNKNOWN != SAFE`. Stale or unavailable evidence becomes UNKNOWN for effective road, flood, and risk status, with raw evidence still available for audit. Road statuses are domain values `PASSABLE`, `WATCH`, `SMALL_VEHICLE_NOT_RECOMMENDED`, `CLOSED`, and `UNKNOWN`, never UI color names. UI status colors must augment text and other accessible cues. No live flood observations are supplied in Phase 2.
