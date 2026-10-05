DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM road_segments) THEN
        RAISE EXCEPTION 'Refusing to roll back migration 25 while road segments exist; export and review immutable segment data first';
    END IF;
    IF EXISTS (
        SELECT 1 FROM community_reports
        WHERE road_segment_id IS NOT NULL
            OR road_segment_assignment_method <> 'UNASSIGNED'
            OR road_segment_assignment_pending
    ) THEN
        RAISE EXCEPTION 'Refusing to roll back migration 25 while report segment assignment metadata exists; export and review assignment data first';
    END IF;
END;
$$;

DROP TRIGGER road_segments_report_area_dirty ON road_segments;
DROP TRIGGER road_segments_immutable_metadata ON road_segments;
DROP FUNCTION enforce_road_segment_immutability();
DROP INDEX IF EXISTS community_reports_road_segment_idx;
ALTER TABLE community_reports DROP CONSTRAINT community_reports_road_segment_assignment_coherence;
ALTER TABLE community_reports
    DROP COLUMN road_segment_assignment_pending,
    DROP COLUMN road_segment_assignment_method,
    DROP COLUMN road_segment_id;
DROP TABLE road_segments;
