# Database migrations

The selected migration system is `golang-migrate/migrate` `v4.18.3`. Migrations use its versioned `NNNNNN_description.up.sql` and `.down.sql` convention. The root Compose `migrate` service owns execution and records applied versions in `schema_migrations`; no Go migration dependency is required.

Run migrations from the repository root after the PostgreSQL Compose service is ready:

```powershell
docker-compose --profile tools run --rm migrate
```

The down migration intentionally leaves PostGIS installed because later spatial objects may depend on it. No application schema is created in Phase 0.
