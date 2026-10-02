CREATE TABLE notification_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE CHECK (btrim(endpoint) <> ''),
    p256dh TEXT NOT NULL CHECK (btrim(p256dh) <> ''),
    auth TEXT NOT NULL CHECK (btrim(auth) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX notification_subscriptions_user_idx ON notification_subscriptions(user_id);

-- Trigger to update updated_at
CREATE FUNCTION update_notification_subscriptions_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER notification_subscriptions_updated_at BEFORE UPDATE ON notification_subscriptions
    FOR EACH ROW EXECUTE FUNCTION update_notification_subscriptions_updated_at();
