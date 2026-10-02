ALTER TABLE data_sources
    ADD COLUMN last_attempt_at TIMESTAMPTZ,
    ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN health_status TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD CONSTRAINT data_sources_consecutive_failures_nonnegative CHECK (consecutive_failures >= 0),
    ADD CONSTRAINT data_sources_health_status_valid CHECK (health_status IN ('HEALTHY', 'DEGRADED', 'UNAVAILABLE', 'UNKNOWN')),
    ADD CONSTRAINT data_sources_health_time_order CHECK (
        (last_attempt_at IS NULL OR last_successful_fetch_at IS NULL OR last_successful_fetch_at <= last_attempt_at)
        AND (last_attempt_at IS NULL OR last_failure_at IS NULL OR last_failure_at <= last_attempt_at)
    );

CREATE TABLE ingestion_runs (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES data_sources (id),
    status TEXT NOT NULL CHECK (status IN ('RUNNING', 'SUCCESS', 'PARTIAL', 'FAILED')),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    fetched_count INTEGER NOT NULL DEFAULT 0 CHECK (fetched_count >= 0),
    accepted_count INTEGER NOT NULL DEFAULT 0 CHECK (accepted_count >= 0),
    rejected_count INTEGER NOT NULL DEFAULT 0 CHECK (rejected_count >= 0),
    inserted_count INTEGER NOT NULL DEFAULT 0 CHECK (inserted_count >= 0),
    updated_count INTEGER NOT NULL DEFAULT 0 CHECK (updated_count >= 0),
    duplicate_count INTEGER NOT NULL DEFAULT 0 CHECK (duplicate_count >= 0),
    error_code TEXT NOT NULL DEFAULT '' CHECK (length(error_code) <= 64),
    error_message_summary TEXT NOT NULL DEFAULT '' CHECK (error_message_summary IN (
        '',
        'Provider fetch failed.',
        'Configured provider is unavailable.',
        'A previous run was interrupted.',
        'Some observations were rejected.',
        'Ingestion failed.'
    )),
    CONSTRAINT ingestion_runs_lifecycle_valid CHECK (
        (status = 'RUNNING' AND finished_at IS NULL
            AND fetched_count = 0 AND accepted_count = 0 AND rejected_count = 0
            AND inserted_count = 0 AND updated_count = 0 AND duplicate_count = 0
            AND error_code = '' AND error_message_summary = '')
        OR (status IN ('SUCCESS', 'PARTIAL', 'FAILED') AND finished_at IS NOT NULL AND finished_at >= started_at)
    )
);

CREATE UNIQUE INDEX ingestion_runs_source_started_uidx ON ingestion_runs (source_id, started_at);
CREATE UNIQUE INDEX ingestion_runs_one_running_per_source_uidx ON ingestion_runs (source_id) WHERE status = 'RUNNING';
CREATE INDEX ingestion_runs_source_recent_idx ON ingestion_runs (source_id, finished_at DESC) WHERE finished_at IS NOT NULL;
