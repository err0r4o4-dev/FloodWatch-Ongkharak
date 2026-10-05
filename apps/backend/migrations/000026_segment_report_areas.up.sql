ALTER TABLE report_areas
    DROP CONSTRAINT report_areas_radius_m_check,
    ADD COLUMN area_type TEXT NOT NULL DEFAULT 'LEGACY_RADIUS' CHECK (area_type IN ('LEGACY_RADIUS', 'ROAD_SEGMENT')),
    ADD COLUMN road_segment_id UUID REFERENCES road_segments (id),
    ADD COLUMN segment_version INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN segment_active BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN start_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN end_label TEXT NOT NULL DEFAULT '',
    ADD COLUMN geometry geometry(LineString, 4326),
    ADD CONSTRAINT report_areas_spatial_identity CHECK (
        (area_type = 'LEGACY_RADIUS' AND radius_m = 150 AND road_segment_id IS NULL AND segment_version = 0 AND geometry IS NULL)
        OR (area_type = 'ROAD_SEGMENT' AND radius_m = 0 AND road_segment_id IS NOT NULL AND road_segment_id = id AND segment_version > 0
            AND geometry IS NOT NULL AND NOT ST_IsEmpty(geometry) AND ST_IsValid(geometry)
            AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180, -90, 180, 90, 4326))
            AND btrim(start_label) <> '' AND btrim(end_label) <> '')
    );

CREATE UNIQUE INDEX report_areas_road_segment_idx ON report_areas (road_segment_id) WHERE road_segment_id IS NOT NULL;
CREATE INDEX report_areas_geometry_gist ON report_areas USING gist (geometry);

UPDATE report_area_refresh_state SET dirty_revision = dirty_revision + 1, updated_at = clock_timestamp() WHERE id = TRUE;
