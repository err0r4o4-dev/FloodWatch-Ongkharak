CREATE TABLE report_areas (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL DEFAULT 'Community report area' CHECK (btrim(name) <> ''),
    anchor geography(Point, 4326) NOT NULL,
    radius_m INTEGER NOT NULL DEFAULT 150 CHECK (radius_m = 150),
    source_type TEXT NOT NULL CHECK (source_type = 'COMMUNITY'),
    depth_band TEXT NOT NULL CHECK (depth_band IN ('UNKNOWN', 'LOW', 'MODERATE', 'HIGH', 'VERY_HIGH')),
    previous_depth_band TEXT NOT NULL CHECK (previous_depth_band IN ('UNKNOWN', 'LOW', 'MODERATE', 'HIGH', 'VERY_HIGH')),
    trend TEXT NOT NULL CHECK (trend IN ('UNKNOWN', 'RISING', 'STABLE', 'FALLING')),
    reported_trend TEXT NOT NULL CHECK (reported_trend IN ('UNKNOWN', 'RISING', 'STABLE', 'FALLING')),
    freshness TEXT NOT NULL CHECK (freshness IN ('FRESH', 'STALE', 'UNKNOWN')),
    summary_reason TEXT NOT NULL CHECK (summary_reason IN ('SUMMARIZED', 'PROVISIONAL', 'NO_RECENT_REPORTS', 'INSUFFICIENT_MEASUREMENTS', 'CONFLICTING_REPORTS', 'UPDATING')),
    latest_observed_at TIMESTAMPTZ,
    computed_at TIMESTAMPTZ NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    report_count INTEGER NOT NULL CHECK (report_count >= 0),
    sample_count INTEGER NOT NULL CHECK (sample_count >= 0),
    total_report_count INTEGER NOT NULL CHECK (total_report_count >= 0),
    report_ids UUID[] NOT NULL DEFAULT '{}',
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0)
);
CREATE INDEX report_areas_anchor_gist ON report_areas USING gist (anchor);
CREATE INDEX report_areas_public_computed_idx ON report_areas (computed_at DESC, id);

CREATE TABLE report_area_members (
    area_id UUID NOT NULL REFERENCES report_areas (id) ON DELETE CASCADE,
    report_id UUID NOT NULL REFERENCES community_reports (id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (area_id, report_id)
);
CREATE INDEX report_area_members_report_idx ON report_area_members (report_id, area_id);

CREATE TABLE report_area_timeline (
    area_id UUID NOT NULL REFERENCES report_areas (id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    depth_band TEXT NOT NULL CHECK (depth_band IN ('UNKNOWN', 'LOW', 'MODERATE', 'HIGH', 'VERY_HIGH')),
    previous_depth_band TEXT NOT NULL CHECK (previous_depth_band IN ('UNKNOWN', 'LOW', 'MODERATE', 'HIGH', 'VERY_HIGH')),
    trend TEXT NOT NULL CHECK (trend IN ('UNKNOWN', 'RISING', 'STABLE', 'FALLING')),
    reported_trend TEXT NOT NULL CHECK (reported_trend IN ('UNKNOWN', 'RISING', 'STABLE', 'FALLING')),
    freshness TEXT NOT NULL CHECK (freshness IN ('FRESH', 'STALE', 'UNKNOWN')),
    summary_reason TEXT NOT NULL CHECK (summary_reason IN ('SUMMARIZED', 'PROVISIONAL', 'NO_RECENT_REPORTS', 'INSUFFICIENT_MEASUREMENTS', 'CONFLICTING_REPORTS', 'UPDATING')),
    latest_observed_at TIMESTAMPTZ,
    report_count INTEGER NOT NULL CHECK (report_count >= 0),
    sample_count INTEGER NOT NULL CHECK (sample_count >= 0),
    evidence_ids UUID[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (area_id, window_start),
    CONSTRAINT report_area_timeline_window_valid CHECK (window_end > window_start)
);

CREATE TABLE report_area_refresh_state (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    dirty_revision BIGINT NOT NULL DEFAULT 0 CHECK (dirty_revision >= 0),
    processed_revision BIGINT NOT NULL DEFAULT 0 CHECK (processed_revision >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO report_area_refresh_state (id) VALUES (TRUE);

CREATE FUNCTION mark_report_areas_dirty() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE report_area_refresh_state
    SET dirty_revision = dirty_revision + 1, updated_at = clock_timestamp()
    WHERE id = TRUE;
    RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER community_reports_report_area_dirty
AFTER INSERT OR UPDATE ON community_reports
FOR EACH ROW EXECUTE FUNCTION mark_report_areas_dirty();

CREATE TRIGGER community_report_media_report_area_dirty
AFTER INSERT OR UPDATE OR DELETE ON community_report_media
FOR EACH ROW EXECUTE FUNCTION mark_report_areas_dirty();
