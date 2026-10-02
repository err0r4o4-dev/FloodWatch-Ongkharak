-- Add an AI data source so that AI observations can be recorded
INSERT INTO data_sources (id, code, name, source_type, provider, enabled)
VALUES ('00000000-0000-4000-8000-000000000001', 'AI_VISION_V1', 'FloodVision v1', 'AI', 'Internal AI Service', TRUE)
ON CONFLICT (code) DO NOTHING;

ALTER TABLE flood_events ADD COLUMN report_id UUID REFERENCES community_reports (id) ON DELETE SET NULL;
ALTER TABLE road_conditions ADD COLUMN report_id UUID REFERENCES community_reports (id) ON DELETE SET NULL;
ALTER TABLE water_readings ADD COLUMN report_id UUID REFERENCES community_reports (id) ON DELETE SET NULL;

CREATE INDEX flood_events_report_idx ON flood_events(report_id) WHERE report_id IS NOT NULL;
CREATE INDEX road_conditions_report_idx ON road_conditions(report_id) WHERE report_id IS NOT NULL;
CREATE INDEX water_readings_report_idx ON water_readings(report_id) WHERE report_id IS NOT NULL;
