ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_target_type_check;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_target_type_check CHECK (target_type IN ('USER', 'ALERT_RULE', 'DATA_SOURCE', 'COMMUNITY_REPORT', 'SYSTEM_SETTING', 'AUTH_SESSION', 'MANUAL_ANNOUNCEMENT'));

CREATE TABLE manual_announcements (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    body TEXT NOT NULL,
    advice TEXT NOT NULL DEFAULT '',
    area TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'WARNING', 'CRITICAL')),
    source_type TEXT NOT NULL CHECK (source_type = 'COMMUNITY'),
    publisher_type TEXT NOT NULL CHECK (publisher_type = 'ADMIN'),
    reference_kind TEXT NOT NULL CHECK (reference_kind = 'MEDIA'),
    observed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    references_json JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(references_json) = 'array'),
    status TEXT NOT NULL CHECK (status IN ('DRAFT', 'PUBLISHED', 'WITHDRAWN')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    withdrawal_reason TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL CHECK (revision > 0),
    CONSTRAINT manual_announcements_expiry_after_observation CHECK (expires_at IS NULL OR observed_at IS NULL OR expires_at > observed_at)
);

CREATE INDEX manual_announcements_public_idx ON manual_announcements (created_at DESC, id DESC) WHERE status IN ('PUBLISHED', 'WITHDRAWN');
CREATE INDEX manual_announcements_status_idx ON manual_announcements (status, expires_at, created_at DESC, id DESC);

CREATE TABLE manual_announcement_revisions (
    announcement_id UUID NOT NULL REFERENCES manual_announcements (id) ON DELETE RESTRICT,
    revision INTEGER NOT NULL CHECK (revision > 0),
    action TEXT NOT NULL CHECK (action IN ('CREATED', 'UPDATED', 'PUBLISHED', 'WITHDRAWN')),
    recorded_at TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    snapshot JSONB NOT NULL CHECK (jsonb_typeof(snapshot) = 'object'),
    PRIMARY KEY (announcement_id, revision)
);

CREATE INDEX manual_announcement_revisions_recorded_idx ON manual_announcement_revisions (announcement_id, recorded_at DESC, revision DESC);
