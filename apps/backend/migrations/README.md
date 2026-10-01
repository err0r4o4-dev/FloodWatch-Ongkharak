# Database migrations

The selected migration system is `golang-migrate/migrate` `v4.18.3`. Migrations use its versioned `NNNNNN_description.up.sql` and `.down.sql` convention. The root Compose `migrate` service owns execution and records applied versions in `schema_migrations`; no Go migration dependency is required.

Run migrations from the repository root after the PostgreSQL Compose service is ready:

```powershell
docker-compose --profile tools run --rm migrate
```

Migration 1 enables PostGIS; migration 2 creates auth tables; migration 3 creates the seven Phase 2 flood-domain tables and spatial/time indexes. Migration 4 adds finite-number checks to measurements and confidence after migration 3 was applied; an existing database containing non-finite values must be inspected and remediated before applying it. Down migration 4 removes only those checks. Down migration 3 drops only the new flood-domain tables (in dependency order); it deletes their data and must not be used against a populated environment without an approved backup plan. The migration 1 down intentionally leaves PostGIS installed because other spatial objects may depend on it.
