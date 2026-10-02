DROP TRIGGER IF EXISTS notification_subscriptions_updated_at ON notification_subscriptions;
DROP FUNCTION IF EXISTS update_notification_subscriptions_updated_at();
DROP INDEX IF EXISTS notification_subscriptions_user_idx;
DROP TABLE IF EXISTS notification_subscriptions;
