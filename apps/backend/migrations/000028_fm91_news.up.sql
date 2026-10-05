ALTER TABLE external_data_sources
    DROP CONSTRAINT external_data_sources_kind_check,
    DROP CONSTRAINT external_data_sources_provenance_check,
    ADD CONSTRAINT external_data_sources_kind_check CHECK (kind IN ('OBSERVATION', 'FORECAST', 'OFFICIAL_ALERT', 'NEWS_REPORT')),
    ADD CONSTRAINT external_data_sources_provenance_check CHECK (provenance IN ('MODEL', 'OFFICIAL', 'MEDIA')),
    ADD CONSTRAINT external_data_sources_news_provenance_check CHECK (
        (kind = 'NEWS_REPORT' AND provenance = 'MEDIA')
        OR (kind IS DISTINCT FROM 'NEWS_REPORT' AND provenance IS DISTINCT FROM 'MEDIA')
    );

ALTER TABLE external_data_records
    DROP CONSTRAINT external_data_records_kind_check,
    DROP CONSTRAINT external_data_records_provenance_check,
    ADD CONSTRAINT external_data_records_kind_check CHECK (kind IN ('OBSERVATION', 'FORECAST', 'OFFICIAL_ALERT', 'NEWS_REPORT')),
    ADD CONSTRAINT external_data_records_provenance_check CHECK (provenance IN ('MODEL', 'OFFICIAL', 'MEDIA')),
    ADD CONSTRAINT external_data_records_news_provenance_check CHECK (
        (kind = 'NEWS_REPORT' AND provenance = 'MEDIA')
        OR (kind <> 'NEWS_REPORT' AND provenance <> 'MEDIA')
    );

ALTER TABLE event_outbox
    DROP CONSTRAINT event_outbox_source_type_check,
    ADD CONSTRAINT event_outbox_source_type_check CHECK (
        source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')
        OR (source_type IN ('MODEL', 'MEDIA') AND event_type = 'external_data.updated' AND resource_type = 'EXTERNAL_DATA')
    );

CREATE INDEX external_data_news_history_idx
    ON external_data_records (source_code, record_key, created_at, id)
    WHERE kind = 'NEWS_REPORT';

CREATE TABLE external_data_news_articles (
    source_code TEXT NOT NULL REFERENCES external_data_sources (code) ON DELETE CASCADE,
    article_id TEXT NOT NULL CHECK (article_id ~ '^[1-9][0-9]{0,18}$'),
    id UUID NOT NULL UNIQUE,
    current_record_id UUID NOT NULL REFERENCES external_data_records (id) ON DELETE CASCADE,
    published_at TIMESTAMPTZ,
    first_retrieved_at TIMESTAMPTZ NOT NULL,
    last_retrieved_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    history_total BIGINT NOT NULL CHECK (history_total >= 1),
    PRIMARY KEY (source_code, article_id),
    CONSTRAINT external_data_news_articles_time_order CHECK (
        first_retrieved_at <= updated_at AND updated_at <= last_retrieved_at
    )
);

CREATE INDEX external_data_news_articles_page_idx
    ON external_data_news_articles (source_code, (COALESCE(published_at, updated_at)) DESC, article_id);
