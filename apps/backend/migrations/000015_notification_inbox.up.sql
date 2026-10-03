CREATE TABLE notification_materialization_state (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    cutoff_at TIMESTAMPTZ NOT NULL
);

INSERT INTO notification_materialization_state (id, cutoff_at) VALUES (TRUE, CURRENT_TIMESTAMP);

CREATE TABLE notification_alert_materializations (
    alert_id UUID PRIMARY KEY REFERENCES alerts (id),
    materialized_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notification_inbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    alert_id UUID NOT NULL REFERENCES alerts (id),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    message TEXT NOT NULL CHECK (btrim(message) <> ''),
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'WARNING', 'CRITICAL')),
    source_type TEXT NOT NULL CHECK (source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')),
    alert_triggered_at TIMESTAMPTZ NOT NULL,
    alert_created_at TIMESTAMPTZ NOT NULL,
    read_at TIMESTAMPTZ,
    CONSTRAINT notification_inbox_user_alert_unique UNIQUE (user_id, alert_id)
);

CREATE INDEX notification_inbox_user_created_idx ON notification_inbox (user_id, alert_created_at DESC, id DESC);
CREATE INDEX notification_inbox_unread_idx ON notification_inbox (user_id, alert_created_at DESC, id DESC)
    WHERE read_at IS NULL;

CREATE TABLE notification_delivery_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES notification_subscriptions (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    alert_id UUID NOT NULL REFERENCES alerts (id),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_until TIMESTAMPTZ,
    lease_token UUID,
    delivered_at TIMESTAMPTZ,
    CONSTRAINT notification_delivery_job_inbox_fk FOREIGN KEY (user_id, alert_id)
        REFERENCES notification_inbox (user_id, alert_id) ON DELETE CASCADE,
    CONSTRAINT notification_delivery_job_subscription_alert_unique UNIQUE (subscription_id, alert_id),
    CONSTRAINT notification_delivery_job_delivery_state CHECK (
        (locked_until IS NULL) = (lease_token IS NULL)
        AND (delivered_at IS NULL OR (locked_until IS NULL AND lease_token IS NULL))
    )
);

CREATE INDEX notification_delivery_jobs_pending_idx ON notification_delivery_jobs (next_attempt_at, id)
    WHERE delivered_at IS NULL AND attempts < 5;
