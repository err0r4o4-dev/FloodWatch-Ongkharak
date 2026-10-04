DELETE FROM event_outbox WHERE event_type = 'external_data.updated';

ALTER TABLE event_outbox
    DROP CONSTRAINT event_outbox_source_type_check,
    ADD CONSTRAINT event_outbox_source_type_check CHECK (source_type IN ('OFFICIAL', 'COMMUNITY', 'AI'));

UPDATE event_outbox e
SET evaluated_at=CURRENT_TIMESTAMP
FROM water_readings r
WHERE e.event_type='water_level.updated' AND e.evaluated_at IS NULL
    AND e.payload->>'evidenceId'=r.id::text AND r.water_level_datum <> 'UNKNOWN';

UPDATE alert_rules SET enabled=FALSE WHERE rule_type='WATER_LEVEL_THRESHOLD';
ALTER TABLE alert_rules
    DROP CONSTRAINT IF EXISTS alert_rules_water_level_scope_valid,
    DROP COLUMN IF EXISTS water_station_id,
    DROP COLUMN IF EXISTS water_level_datum;

ALTER TABLE water_readings
    DROP COLUMN IF EXISTS rainfall_period_seconds,
    DROP COLUMN IF EXISTS water_level_datum;

ALTER TABLE external_data_sources
    DROP CONSTRAINT IF EXISTS external_data_sources_last_batch_fk;

DROP TABLE IF EXISTS external_data_record_raw_references;
DROP TABLE IF EXISTS external_data_raw_references;
DROP TABLE IF EXISTS external_data_batch_records;
DROP TABLE IF EXISTS external_data_records;
DROP TABLE IF EXISTS external_data_batches;
DROP TABLE IF EXISTS external_data_runs;
DROP TABLE IF EXISTS external_data_sources;
