# Data trust and provenance

Canonical source types are `OFFICIAL`, `COMMUNITY`, and `AI`. Neither community nor AI evidence becomes official automatically. Future records should support `source`, `sourceType`, `observedAt`, `receivedAt`, `updatedAt`, `freshness`, `verificationStatus`, and `confidence` with appropriately nullable/unknown states. Do not infer safety from an absence of reports.

`UNKNOWN != SAFE`. Stale or unavailable data must be labelled unavailable, with its age and origin visible to users. Road statuses are domain values `PASSABLE`, `WATCH`, `SMALL_VEHICLE_NOT_RECOMMENDED`, `CLOSED`, and `UNKNOWN`, never UI color names. UI status colors must augment text and other accessible cues. No live data exists in Phase 0.
