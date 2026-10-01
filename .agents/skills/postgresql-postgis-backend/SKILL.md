---
name: postgresql-postgis-backend
description: Use when designing Flood Watch PostgreSQL or PostGIS schemas, migrations, spatial queries, indexes, repositories, or data-retention rules.
---
# Purpose
Store source-aware flood data with correct spatial semantics and reversible schema evolution.
# Use When
Adding tables, migrations, geospatial columns, SQL queries, indexes, or retention jobs.
# Do Not Use When
Changing API-only serialization, Redis transport, or client-side map rendering.
# Required Rules
- Use migrations for every schema change; make ordering, constraints, and rollback implications explicit.
- Store source provenance, observed time, ingestion time, and status separately from derived interpretations.
- Validate geometry, SRID, coordinate order, and spatial extent at write boundaries.
- Index proven query paths and parameterize SQL; never interpolate external input.
# Workflow
Model invariants, write migration and repository tests, inspect query plans where needed, then test upgrade and rollback paths.
# Checklist
- [ ] Constraints prevent invalid timestamps, provenance, and spatial data.
- [ ] Spatial queries declare SRID and use appropriate indexes.
# Related Skills
`go-api-architecture`, `flood-data-reliability`, `floodwatch-map-geospatial-ui`.
