CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE event_outbox (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL CHECK (btrim(event_type) <> ''),
    occurred_at TIMESTAMPTZ NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')),
    resource_type TEXT NOT NULL CHECK (btrim(resource_type) <> ''),
    resource_id UUID NOT NULL,
    payload_version INTEGER NOT NULL DEFAULT 1 CHECK (payload_version >= 1),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    published_at TIMESTAMPTZ,
    evaluated_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX event_outbox_unpublished_idx ON event_outbox (occurred_at, id) WHERE published_at IS NULL;
CREATE INDEX event_outbox_unevaluated_domain_idx ON event_outbox (occurred_at, id)
    WHERE evaluated_at IS NULL AND event_type IN ('water_level.updated', 'road.status.changed', 'flood_event.created');

CREATE TABLE alert_rules (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    rule_type TEXT NOT NULL CHECK (rule_type IN ('WATER_LEVEL_THRESHOLD', 'ROAD_CLOSED', 'FLOOD_CRITICAL')),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'WARNING', 'CRITICAL')),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    message TEXT NOT NULL CHECK (btrim(message) <> ''),
    water_level_threshold_m NUMERIC(12, 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT alert_rules_water_threshold_valid CHECK (
        (rule_type = 'WATER_LEVEL_THRESHOLD' AND water_level_threshold_m IS NOT NULL AND water_level_threshold_m >= 0 AND water_level_threshold_m > '-Infinity'::numeric AND water_level_threshold_m < 'Infinity'::numeric)
        OR (rule_type IN ('ROAD_CLOSED', 'FLOOD_CRITICAL') AND water_level_threshold_m IS NULL)
    )
);
CREATE INDEX alert_rules_enabled_idx ON alert_rules (created_at, id) WHERE enabled;

CREATE TABLE alerts (
    id UUID PRIMARY KEY,
    rule_id UUID NOT NULL REFERENCES alert_rules (id),
    rule_type TEXT NOT NULL CHECK (rule_type IN ('WATER_LEVEL_THRESHOLD', 'ROAD_CLOSED', 'FLOOD_CRITICAL')),
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'WARNING', 'CRITICAL')),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    message TEXT NOT NULL CHECK (btrim(message) <> ''),
    evidence_id UUID NOT NULL,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('WATER_STATION', 'ROAD', 'FLOOD_EVENT')),
    resource_id UUID NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')),
    triggered_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (rule_id, evidence_id)
);
CREATE INDEX alerts_triggered_idx ON alerts (triggered_at DESC, id DESC);
CREATE INDEX alerts_resource_created_idx ON alerts (resource_type, resource_id, created_at DESC);

CREATE FUNCTION enqueue_water_level_updated() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    event_source_type TEXT;
BEGIN
    SELECT source_type INTO event_source_type FROM data_sources WHERE id = NEW.source_id AND source_type IN ('OFFICIAL', 'COMMUNITY', 'AI');
    IF NOT FOUND THEN
        RETURN NEW;
    END IF;
    INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload)
    VALUES (gen_random_uuid(), 'water_level.updated', NEW.received_at, event_source_type, 'WATER_STATION', NEW.station_id, jsonb_build_object('evidenceId', NEW.id));
    RETURN NEW;
END;
$$;

CREATE FUNCTION enqueue_flood_event_created() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    event_source_type TEXT;
BEGIN
    SELECT source_type INTO event_source_type FROM data_sources WHERE id = NEW.source_id AND source_type IN ('OFFICIAL', 'COMMUNITY', 'AI');
    IF NOT FOUND THEN
        RETURN NEW;
    END IF;
    INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload)
    VALUES (gen_random_uuid(), 'flood_event.created', NEW.received_at, event_source_type, 'FLOOD_EVENT', NEW.id, jsonb_build_object('evidenceId', NEW.id));
    RETURN NEW;
END;
$$;

CREATE FUNCTION enqueue_road_status_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    event_source_type TEXT;
    current_condition_id UUID;
    previous_status TEXT;
BEGIN
    PERFORM 1 FROM roads WHERE id = NEW.road_id FOR UPDATE;
    SELECT source_type INTO event_source_type FROM data_sources WHERE id = NEW.source_id AND source_type IN ('OFFICIAL', 'COMMUNITY', 'AI');
    IF NOT FOUND OR NEW.verification_status <> 'VERIFIED' OR NEW.status = 'UNKNOWN' THEN
        RETURN NEW;
    END IF;
    SELECT id INTO current_condition_id FROM road_conditions WHERE road_id = NEW.road_id ORDER BY observed_at DESC, received_at DESC, id DESC LIMIT 1;
    IF current_condition_id <> NEW.id THEN
        RETURN NEW;
    END IF;
    SELECT status INTO previous_status FROM road_conditions WHERE road_id = NEW.road_id AND id <> NEW.id ORDER BY observed_at DESC, received_at DESC, id DESC LIMIT 1;
    IF previous_status IS NOT NULL AND previous_status = NEW.status THEN
        RETURN NEW;
    END IF;
    INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload)
    VALUES (gen_random_uuid(), 'road.status.changed', NEW.received_at, event_source_type, 'ROAD', NEW.road_id, jsonb_build_object('evidenceId', NEW.id));
    RETURN NEW;
END;
$$;

CREATE FUNCTION enqueue_report_verified() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status = 'PENDING' AND NEW.status = 'VERIFIED' AND NEW.source_type = 'COMMUNITY' THEN
        INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, evaluated_at)
        VALUES (gen_random_uuid(), 'report.verified', NEW.verified_at, 'COMMUNITY', 'COMMUNITY_REPORT', NEW.id, CURRENT_TIMESTAMP);
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER water_readings_event_outbox AFTER INSERT ON water_readings FOR EACH ROW EXECUTE FUNCTION enqueue_water_level_updated();
CREATE TRIGGER flood_events_event_outbox AFTER INSERT ON flood_events FOR EACH ROW EXECUTE FUNCTION enqueue_flood_event_created();
CREATE TRIGGER road_conditions_event_outbox AFTER INSERT ON road_conditions FOR EACH ROW EXECUTE FUNCTION enqueue_road_status_changed();
CREATE TRIGGER community_reports_event_outbox AFTER UPDATE OF status ON community_reports FOR EACH ROW EXECUTE FUNCTION enqueue_report_verified();
