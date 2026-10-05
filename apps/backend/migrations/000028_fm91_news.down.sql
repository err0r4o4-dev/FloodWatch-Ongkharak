DROP TABLE IF EXISTS external_data_news_articles;
DELETE FROM event_outbox WHERE source_type = 'MEDIA';
UPDATE external_data_sources SET last_successful_batch_id = NULL WHERE kind = 'NEWS_REPORT' OR provenance = 'MEDIA' OR code = 'FM91_NEWS';
DELETE FROM external_data_record_raw_references WHERE record_id IN (SELECT id FROM external_data_records WHERE kind = 'NEWS_REPORT');
DELETE FROM external_data_batch_records WHERE record_id IN (SELECT id FROM external_data_records WHERE kind = 'NEWS_REPORT');
DELETE FROM external_data_raw_references WHERE batch_id IN (SELECT b.id FROM external_data_batches b JOIN external_data_sources s ON s.code = b.source_code WHERE s.kind = 'NEWS_REPORT' OR s.provenance = 'MEDIA' OR s.code = 'FM91_NEWS');
DELETE FROM external_data_records WHERE kind = 'NEWS_REPORT';
DELETE FROM external_data_batches WHERE source_code IN (SELECT code FROM external_data_sources WHERE kind = 'NEWS_REPORT' OR provenance = 'MEDIA' OR code = 'FM91_NEWS');
DELETE FROM external_data_runs WHERE source_code IN (SELECT code FROM external_data_sources WHERE kind = 'NEWS_REPORT' OR provenance = 'MEDIA' OR code = 'FM91_NEWS');
DELETE FROM external_data_sources WHERE kind = 'NEWS_REPORT' OR provenance = 'MEDIA' OR code = 'FM91_NEWS';

DROP INDEX external_data_news_history_idx;

ALTER TABLE external_data_records
    DROP CONSTRAINT external_data_records_news_provenance_check,
    DROP CONSTRAINT external_data_records_kind_check,
    DROP CONSTRAINT external_data_records_provenance_check,
    ADD CONSTRAINT external_data_records_kind_check CHECK (kind IN ('OBSERVATION', 'FORECAST', 'OFFICIAL_ALERT')),
    ADD CONSTRAINT external_data_records_provenance_check CHECK (provenance IN ('MODEL', 'OFFICIAL'));

ALTER TABLE external_data_sources
    DROP CONSTRAINT external_data_sources_news_provenance_check,
    DROP CONSTRAINT external_data_sources_kind_check,
    DROP CONSTRAINT external_data_sources_provenance_check,
    ADD CONSTRAINT external_data_sources_kind_check CHECK (kind IN ('OBSERVATION', 'FORECAST', 'OFFICIAL_ALERT')),
    ADD CONSTRAINT external_data_sources_provenance_check CHECK (provenance IN ('MODEL', 'OFFICIAL'));

ALTER TABLE event_outbox
    DROP CONSTRAINT event_outbox_source_type_check,
    ADD CONSTRAINT event_outbox_source_type_check CHECK (
        source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')
        OR (source_type = 'MODEL' AND event_type = 'external_data.updated' AND resource_type = 'EXTERNAL_DATA')
    );
