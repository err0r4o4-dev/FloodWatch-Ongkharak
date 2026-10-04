DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM event_outbox WHERE event_type = 'report-area.updated' AND published_at IS NULL) THEN
        RAISE EXCEPTION 'Quiesce report-area refreshes and drain report-area.updated events before rolling back migration 23';
    END IF;
END;
$$;

DROP TRIGGER community_report_media_report_area_dirty ON community_report_media;
DROP TRIGGER community_reports_report_area_dirty ON community_reports;
DROP FUNCTION mark_report_areas_dirty();
DROP TABLE report_area_refresh_state;
DROP TABLE report_area_timeline;
DROP TABLE report_area_members;
DROP TABLE report_areas;
