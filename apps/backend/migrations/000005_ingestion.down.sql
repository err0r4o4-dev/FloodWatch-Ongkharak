DROP TABLE IF EXISTS ingestion_runs;

ALTER TABLE data_sources
    DROP CONSTRAINT IF EXISTS data_sources_health_time_order,
    DROP CONSTRAINT IF EXISTS data_sources_health_status_valid,
    DROP CONSTRAINT IF EXISTS data_sources_consecutive_failures_nonnegative,
    DROP COLUMN IF EXISTS health_status,
    DROP COLUMN IF EXISTS consecutive_failures,
    DROP COLUMN IF EXISTS last_attempt_at;
