DROP INDEX IF EXISTS water_readings_report_idx;
DROP INDEX IF EXISTS road_conditions_report_idx;
DROP INDEX IF EXISTS flood_events_report_idx;

ALTER TABLE water_readings DROP COLUMN IF EXISTS report_id;
ALTER TABLE road_conditions DROP COLUMN IF EXISTS report_id;
ALTER TABLE flood_events DROP COLUMN IF EXISTS report_id;

DELETE FROM data_sources WHERE code = 'AI_VISION_V1';
