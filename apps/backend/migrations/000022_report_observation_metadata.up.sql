ALTER TABLE community_reports
    ADD COLUMN observed_at TIMESTAMPTZ,
    ADD COLUMN water_trend TEXT NOT NULL DEFAULT 'UNKNOWN',
    ADD CONSTRAINT community_reports_water_trend_valid CHECK (water_trend IN ('UNKNOWN', 'RISING', 'STABLE', 'FALLING')),
    ADD CONSTRAINT community_reports_observed_at_valid CHECK (
        observed_at IS NULL OR (isfinite(observed_at) AND observed_at >= TIMESTAMPTZ '2000-01-01 00:00:00+00')
    );

CREATE INDEX community_reports_observed_at_idx ON community_reports (observed_at DESC)
    WHERE status IN ('PENDING', 'VERIFIED') AND observed_at IS NOT NULL;

ALTER TABLE community_report_media
    ADD COLUMN content_sha256 TEXT,
    ADD CONSTRAINT community_report_media_sha256_valid CHECK (content_sha256 IS NULL OR content_sha256 ~ '^[0-9a-f]{64}$');
