ALTER TABLE notification_subscriptions ADD COLUMN locale TEXT NOT NULL DEFAULT 'th' CHECK (locale IN ('th', 'en'));
