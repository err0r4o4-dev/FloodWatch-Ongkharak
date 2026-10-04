DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM event_outbox WHERE event_type = 'report.updated' AND published_at IS NULL) THEN
        RAISE EXCEPTION 'Quiesce report writes and drain report.updated events with a compatible dispatcher before rolling back API binaries or migration 21';
    END IF;
END;
$$;

DROP TRIGGER community_reports_public_event_outbox ON community_reports;
DROP FUNCTION enqueue_report_updated();
DROP INDEX community_reports_public_created_idx;
DROP TABLE community_report_reviews;
