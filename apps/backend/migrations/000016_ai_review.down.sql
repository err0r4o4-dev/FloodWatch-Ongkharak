DROP TABLE IF EXISTS ai_evidence;
DROP TABLE IF EXISTS ai_analysis_jobs;
ALTER TABLE notification_subscriptions DROP CONSTRAINT IF EXISTS notification_subscriptions_version_positive;
ALTER TABLE notification_subscriptions DROP COLUMN IF EXISTS version;
