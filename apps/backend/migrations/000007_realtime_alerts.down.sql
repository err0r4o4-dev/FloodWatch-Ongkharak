DROP TRIGGER community_reports_event_outbox ON community_reports;
DROP TRIGGER road_conditions_event_outbox ON road_conditions;
DROP TRIGGER flood_events_event_outbox ON flood_events;
DROP TRIGGER water_readings_event_outbox ON water_readings;

DROP FUNCTION enqueue_report_verified();
DROP FUNCTION enqueue_road_status_changed();
DROP FUNCTION enqueue_flood_event_created();
DROP FUNCTION enqueue_water_level_updated();

DROP TABLE alerts;
DROP TABLE alert_rules;
DROP TABLE event_outbox;
