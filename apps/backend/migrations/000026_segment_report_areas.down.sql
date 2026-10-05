DELETE FROM report_areas WHERE area_type = 'ROAD_SEGMENT';

DROP INDEX report_areas_geometry_gist;
DROP INDEX report_areas_road_segment_idx;

ALTER TABLE report_areas
    DROP CONSTRAINT report_areas_spatial_identity,
    DROP COLUMN geometry,
    DROP COLUMN end_label,
    DROP COLUMN start_label,
    DROP COLUMN segment_active,
    DROP COLUMN segment_version,
    DROP COLUMN road_segment_id,
    DROP COLUMN area_type,
    ADD CONSTRAINT report_areas_radius_m_check CHECK (radius_m = 150);

UPDATE report_area_refresh_state SET dirty_revision = dirty_revision + 1, updated_at = clock_timestamp() WHERE id = TRUE;
