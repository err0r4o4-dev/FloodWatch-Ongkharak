-- Audit logs for administrative and security-sensitive actions
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action TEXT NOT NULL CHECK (btrim(action) <> ''),
    actor_id UUID REFERENCES users (id) ON DELETE SET NULL,
    actor_email TEXT NOT NULL CHECK (btrim(actor_email) <> ''),
    actor_role TEXT NOT NULL CHECK (actor_role IN ('USER', 'ADMIN', 'SYSTEM')),
    target_type TEXT NOT NULL CHECK (target_type IN ('USER', 'ALERT_RULE', 'DATA_SOURCE', 'COMMUNITY_REPORT', 'SYSTEM_SETTING', 'AUTH_SESSION')),
    target_id TEXT NOT NULL CHECK (btrim(target_id) <> ''),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX audit_logs_created_idx ON audit_logs (created_at DESC, id DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, created_at DESC) WHERE actor_id IS NOT NULL;
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id, created_at DESC);
CREATE INDEX audit_logs_action_idx ON audit_logs (action, created_at DESC);

-- System settings for configurable operational parameters
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY CHECK (btrim(key) <> ''),
    value JSONB NOT NULL CHECK (jsonb_typeof(value) IN ('string', 'number', 'boolean', 'object', 'array')),
    value_type TEXT NOT NULL CHECK (value_type IN ('string', 'number', 'boolean', 'json')),
    description TEXT NOT NULL CHECK (btrim(description) <> ''),
    category TEXT NOT NULL CHECK (category IN ('rate_limit', 'alert', 'ingestion', 'feature', 'security')),
    editable BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX system_settings_category_idx ON system_settings (category, key);
CREATE INDEX system_settings_updated_idx ON system_settings (updated_at DESC);

-- Insert default system settings
INSERT INTO system_settings (key, value, value_type, description, category, editable) VALUES
    ('rate_limit.auth.register_per_ip_per_hour', '5', 'number', 'Maximum registration attempts per IP per hour', 'rate_limit', TRUE),
    ('rate_limit.auth.login_per_account_per_minute', '10', 'number', 'Maximum login attempts per account per minute', 'rate_limit', TRUE),
    ('rate_limit.report.create_per_user_per_day', '10', 'number', 'Maximum reports a user can create per day', 'rate_limit', TRUE),
    ('alert.default_severity', '"WATCH"', 'string', 'Default severity for new alert rules', 'alert', TRUE),
    ('alert.max_rules_per_admin', '50', 'number', 'Maximum alert rules an admin can create', 'alert', TRUE),
    ('ingestion.worker_poll_interval_seconds', '300', 'number', 'Interval between ingestion polls in seconds', 'ingestion', TRUE),
    ('ingestion.max_concurrent_sources', '5', 'number', 'Maximum concurrent data source ingestions', 'ingestion', TRUE),
    ('feature.community_reports_enabled', 'true', 'boolean', 'Enable community report submission', 'feature', TRUE),
    ('feature.realtime_alerts_enabled', 'true', 'boolean', 'Enable realtime alert processing', 'feature', TRUE),
    ('security.session_max_age_hours', '168', 'number', 'Maximum session age in hours (7 days)', 'security', TRUE),
    ('security.require_email_verification', 'false', 'boolean', 'Require email verification for new accounts', 'security', FALSE);

-- Trigger to update system_settings.updated_at
CREATE FUNCTION update_system_settings_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER system_settings_updated_at BEFORE UPDATE ON system_settings
    FOR EACH ROW EXECUTE FUNCTION update_system_settings_updated_at();
