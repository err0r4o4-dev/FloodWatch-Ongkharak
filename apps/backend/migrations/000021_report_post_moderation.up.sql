CREATE TABLE community_report_reviews (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES community_reports (id) ON DELETE CASCADE,
    action TEXT NOT NULL CHECK (action IN ('VERIFY', 'REMOVE', 'RESTORE')),
    previous_status TEXT NOT NULL CHECK (previous_status IN ('PENDING', 'VERIFIED', 'REJECTED')),
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'VERIFIED', 'REJECTED')),
    actor_id UUID NOT NULL REFERENCES users (id),
    reason TEXT NOT NULL DEFAULT '' CHECK (char_length(reason) <= 500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT community_report_reviews_transition_valid CHECK (
        (action = 'VERIFY' AND previous_status = 'PENDING' AND status = 'VERIFIED' AND reason = '')
        OR (action = 'REMOVE' AND previous_status IN ('PENDING', 'VERIFIED') AND status = 'REJECTED')
        OR (action = 'RESTORE' AND previous_status = 'REJECTED' AND status = 'PENDING' AND btrim(reason) <> '')
    )
);
CREATE INDEX community_report_reviews_report_created_idx ON community_report_reviews (report_id, created_at DESC, id DESC);
CREATE INDEX community_reports_public_created_idx ON community_reports (created_at DESC) WHERE status IN ('PENDING', 'VERIFIED');

INSERT INTO community_report_reviews (id, report_id, action, previous_status, status, actor_id, reason, created_at)
SELECT gen_random_uuid(), id, CASE status WHEN 'VERIFIED' THEN 'VERIFY' ELSE 'REMOVE' END,
    'PENDING', status, reviewed_by, COALESCE(rejection_reason, ''), reviewed_at
FROM community_reports WHERE status IN ('VERIFIED', 'REJECTED');

CREATE FUNCTION enqueue_report_updated() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.source_type = 'COMMUNITY' AND (TG_OP = 'INSERT' OR NEW.updated_at IS DISTINCT FROM OLD.updated_at) THEN
        INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, evaluated_at)
        VALUES (gen_random_uuid(), 'report.updated', NEW.updated_at, 'COMMUNITY', 'COMMUNITY_REPORT', NEW.id, CURRENT_TIMESTAMP);
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER community_reports_public_event_outbox AFTER INSERT OR UPDATE ON community_reports
FOR EACH ROW EXECUTE FUNCTION enqueue_report_updated();
