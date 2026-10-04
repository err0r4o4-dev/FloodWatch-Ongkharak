ALTER TABLE community_report_media DROP COLUMN content_sha256;
DROP INDEX community_reports_observed_at_idx;
ALTER TABLE community_reports DROP COLUMN water_trend, DROP COLUMN observed_at;
