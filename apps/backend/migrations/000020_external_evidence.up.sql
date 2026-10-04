CREATE TABLE external_data_sources (
    id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    code TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT CHECK (kind IN ('OBSERVATION', 'FORECAST', 'OFFICIAL_ALERT')),
    provenance TEXT CHECK (provenance IN ('MODEL', 'OFFICIAL')),
    url TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    last_error_code TEXT,
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    health_status TEXT NOT NULL DEFAULT 'UNKNOWN' CHECK (health_status IN ('HEALTHY', 'DEGRADED', 'UNAVAILABLE', 'UNKNOWN')),
    last_successful_batch_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT external_data_sources_code_format CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{0,63}$'),
    CONSTRAINT external_data_sources_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT external_data_sources_health_time_order CHECK (
        (last_success_at IS NULL OR last_attempt_at IS NULL OR last_success_at <= last_attempt_at)
        AND (last_failure_at IS NULL OR last_attempt_at IS NULL OR last_failure_at <= last_attempt_at)
    )
);

INSERT INTO data_sources (id, code, name, source_type, provider, enabled)
VALUES
    (gen_random_uuid(), 'THAIWATER_WATER', 'ThaiWater water observations', 'OFFICIAL', 'ThaiWater', TRUE),
    (gen_random_uuid(), 'THAIWATER_RAIN', 'ThaiWater rainfall observations', 'OFFICIAL', 'ThaiWater', TRUE)
ON CONFLICT (code) DO NOTHING;

ALTER TABLE water_readings
    ADD COLUMN water_level_datum TEXT NOT NULL DEFAULT 'UNKNOWN'
        CHECK (water_level_datum IN ('UNKNOWN', 'MSL', 'LOCAL')),
    ADD COLUMN rainfall_period_seconds INTEGER
        CHECK (rainfall_period_seconds IS NULL OR rainfall_period_seconds > 0);

ALTER TABLE alert_rules
    ADD COLUMN water_level_datum TEXT NOT NULL DEFAULT 'UNKNOWN'
        CHECK (water_level_datum IN ('UNKNOWN', 'MSL', 'LOCAL')),
    ADD COLUMN water_station_id UUID REFERENCES water_stations (id),
    ADD CONSTRAINT alert_rules_water_level_scope_valid CHECK (
        (rule_type = 'WATER_LEVEL_THRESHOLD' AND (
            (water_level_datum = 'UNKNOWN')
            OR (water_level_datum IN ('MSL', 'LOCAL') AND water_station_id IS NOT NULL)
        ))
        OR (rule_type <> 'WATER_LEVEL_THRESHOLD' AND water_level_datum = 'UNKNOWN' AND water_station_id IS NULL)
    );

CREATE TABLE external_data_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_code TEXT NOT NULL REFERENCES external_data_sources (code),
    status TEXT NOT NULL CHECK (status IN ('RUNNING', 'SUCCESS', 'FAILED')),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    error_code TEXT NOT NULL DEFAULT '' CHECK (char_length(error_code) <= 64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT external_data_runs_lifecycle_valid CHECK (
        (status = 'RUNNING' AND finished_at IS NULL AND error_code = '')
        OR (status IN ('SUCCESS', 'FAILED') AND finished_at IS NOT NULL AND finished_at >= started_at)
    )
);

CREATE UNIQUE INDEX external_data_runs_one_running_uidx
    ON external_data_runs (source_code) WHERE status = 'RUNNING';
CREATE INDEX external_data_runs_source_recent_idx
    ON external_data_runs (source_code, started_at DESC, id DESC);

CREATE TABLE external_data_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_code TEXT NOT NULL REFERENCES external_data_sources (code),
    run_id UUID NOT NULL UNIQUE REFERENCES external_data_runs (id),
    retrieved_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE external_data_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_code TEXT NOT NULL REFERENCES external_data_sources (code),
    record_key TEXT NOT NULL,
    content_hash CHAR(64) NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('OBSERVATION', 'FORECAST', 'OFFICIAL_ALERT')),
    provenance TEXT NOT NULL CHECK (provenance IN ('MODEL', 'OFFICIAL')),
    district TEXT,
    observed_at TIMESTAMPTZ,
    issued_at TIMESTAMPTZ,
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    model TEXT,
    member TEXT,
    alert_message_type TEXT,
    alert_identifier TEXT,
    freshness TEXT,
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    geometry geometry(Geometry, 4326),
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT external_data_records_key_nonempty CHECK (btrim(record_key) <> ''),
    CONSTRAINT external_data_records_hash_format CHECK (content_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT external_data_records_time_order CHECK (valid_to IS NULL OR valid_from IS NULL OR valid_to >= valid_from),
    CONSTRAINT external_data_records_coordinate_pair CHECK (
        (latitude IS NULL AND longitude IS NULL)
        OR (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180)
    ),
    CONSTRAINT external_data_records_geometry_valid CHECK (
        geometry IS NULL OR (NOT ST_IsEmpty(geometry) AND ST_IsValid(geometry)
            AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180, -90, 180, 90, 4326)))
    ),
    UNIQUE (source_code, record_key, content_hash)
);

CREATE INDEX external_data_records_district_idx
    ON external_data_records (source_code, district, observed_at DESC);
CREATE INDEX external_data_records_forecast_idx
    ON external_data_records (source_code, valid_from, member, model);
CREATE INDEX external_data_records_alert_idx
    ON external_data_records (source_code, valid_to, issued_at DESC, alert_message_type);

CREATE TABLE external_data_batch_records (
    batch_id UUID NOT NULL REFERENCES external_data_batches (id) ON DELETE CASCADE,
    record_id UUID NOT NULL REFERENCES external_data_records (id),
    PRIMARY KEY (batch_id, record_id)
);

CREATE INDEX external_data_batch_records_record_idx
    ON external_data_batch_records (record_id, batch_id);

CREATE TABLE external_data_raw_references (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES external_data_runs (id),
    batch_id UUID NOT NULL REFERENCES external_data_batches (id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    object_key TEXT NOT NULL DEFAULT '',
    sha256 CHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT external_data_raw_references_url_or_object_key CHECK (url <> '' OR object_key <> ''),
    CONSTRAINT external_data_raw_references_hash_format CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    UNIQUE (run_id, url, object_key, sha256)
);

CREATE TABLE external_data_record_raw_references (
    record_id UUID NOT NULL REFERENCES external_data_records (id),
    raw_reference_id UUID NOT NULL REFERENCES external_data_raw_references (id) ON DELETE CASCADE,
    PRIMARY KEY (record_id, raw_reference_id)
);

ALTER TABLE external_data_sources
    ADD CONSTRAINT external_data_sources_last_batch_fk
    FOREIGN KEY (last_successful_batch_id) REFERENCES external_data_batches (id);

CREATE INDEX external_data_raw_references_run_idx
    ON external_data_raw_references (run_id, id);

ALTER TABLE event_outbox
    DROP CONSTRAINT event_outbox_source_type_check,
    ADD CONSTRAINT event_outbox_source_type_check CHECK (
        source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')
        OR (source_type = 'MODEL' AND event_type = 'external_data.updated' AND resource_type = 'EXTERNAL_DATA')
    );
