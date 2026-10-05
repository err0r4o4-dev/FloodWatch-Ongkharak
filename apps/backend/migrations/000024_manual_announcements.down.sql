DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM audit_logs WHERE target_type = 'MANUAL_ANNOUNCEMENT') THEN
        RAISE EXCEPTION 'Cannot roll back migration 24 while manual announcement audit logs exist';
    END IF;
END;
$$;

DROP TABLE IF EXISTS manual_announcement_revisions;
DROP TABLE IF EXISTS manual_announcements;

ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_target_type_check;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_target_type_check CHECK (target_type IN ('USER', 'ALERT_RULE', 'DATA_SOURCE', 'COMMUNITY_REPORT', 'SYSTEM_SETTING', 'AUTH_SESSION'));
