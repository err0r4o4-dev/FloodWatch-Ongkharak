CREATE TABLE road_segments (
    id UUID PRIMARY KEY,
    road_id UUID NOT NULL REFERENCES roads (id),
    name TEXT NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 200),
    start_label TEXT NOT NULL CHECK (btrim(start_label) <> '' AND char_length(start_label) <= 200),
    end_label TEXT NOT NULL CHECK (btrim(end_label) <> '' AND char_length(end_label) <= 200),
    geometry geometry(LineString, 4326) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    supersedes_id UUID REFERENCES road_segments (id),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT road_segments_geometry_valid CHECK (
        ST_NDims(geometry) = 2
        AND NOT ST_IsEmpty(geometry)
        AND ST_IsValid(geometry)
        AND ST_IsSimple(geometry)
        AND ST_NumPoints(geometry) >= 2
        AND ST_Length(geometry::geography) > 0
        AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180, -90, 180, 90, 4326))
        AND ST_StartPoint(geometry) IS DISTINCT FROM ST_EndPoint(geometry)
        AND ST_Length(geometry::geography) <= 1000
    )
);
CREATE INDEX road_segments_geometry_gist ON road_segments USING gist (geometry);
CREATE INDEX road_segments_geography_gist ON road_segments USING gist ((geometry::geography));
CREATE INDEX road_segments_active_created_idx ON road_segments (created_at DESC, id DESC) WHERE active;

CREATE FUNCTION enforce_road_segment_immutability() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.id IS DISTINCT FROM OLD.id
        OR NEW.road_id IS DISTINCT FROM OLD.road_id
        OR NEW.name IS DISTINCT FROM OLD.name
        OR NEW.start_label IS DISTINCT FROM OLD.start_label
        OR NEW.end_label IS DISTINCT FROM OLD.end_label
        OR NEW.geometry IS DISTINCT FROM OLD.geometry
        OR NEW.version IS DISTINCT FROM OLD.version
        OR NEW.supersedes_id IS DISTINCT FROM OLD.supersedes_id
        OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'road segment metadata is immutable';
    END IF;
    IF OLD.active = FALSE AND NEW.active = TRUE THEN
        RAISE EXCEPTION 'retired road segments cannot be reactivated';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER road_segments_immutable_metadata
BEFORE UPDATE ON road_segments
FOR EACH ROW EXECUTE FUNCTION enforce_road_segment_immutability();

ALTER TABLE community_reports
    ADD COLUMN road_segment_id UUID NULL REFERENCES road_segments (id),
    ADD COLUMN road_segment_assignment_method TEXT NOT NULL DEFAULT 'UNASSIGNED'
        CHECK (road_segment_assignment_method IN ('UNASSIGNED', 'USER_CONFIRMED', 'LEGACY_MATCH')),
    ADD COLUMN road_segment_assignment_pending BOOLEAN NOT NULL DEFAULT FALSE,
    ADD CONSTRAINT community_reports_road_segment_assignment_coherence CHECK (
        (road_segment_id IS NULL AND road_segment_assignment_method = 'UNASSIGNED')
        OR (road_segment_id IS NOT NULL AND road_segment_assignment_method IN ('USER_CONFIRMED', 'LEGACY_MATCH'))
    );

UPDATE community_reports SET road_segment_assignment_pending = TRUE;

CREATE INDEX community_reports_road_segment_idx ON community_reports (road_segment_id) WHERE road_segment_id IS NOT NULL;

CREATE TRIGGER road_segments_report_area_dirty
AFTER INSERT OR UPDATE OR DELETE ON road_segments
FOR EACH ROW EXECUTE FUNCTION mark_report_areas_dirty();
