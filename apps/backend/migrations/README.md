# Database migrations

The selected migration system is `golang-migrate/migrate` `v4.18.3`. Migrations use its versioned `NNNNNN_description.up.sql` and `.down.sql` convention. The root Compose `migrate` service owns execution and records applied versions in `schema_migrations`; no Go migration dependency is required.

Run migrations from the repository root after the PostgreSQL Compose service is ready:

```powershell
docker-compose --profile tools run --rm migrate
```

Migration 1 enables PostGIS; migration 2 creates auth tables; migration 3 creates the seven Phase 2 flood-domain tables and spatial/time indexes. Migration 4 adds finite-number checks to measurements and confidence after migration 3 was applied; an existing database containing non-finite values must be inspected and remediated before applying it. Migration 5 adds ingestion runs and source health fields. Down migration 5 removes those run records and health fields, so use it only with an approved backup plan for a populated environment. Down migration 4 removes only finite-number checks. Down migration 3 drops the flood-domain tables and their data (in dependency order). The migration 1 down intentionally leaves PostGIS installed because other spatial objects may depend on it.
