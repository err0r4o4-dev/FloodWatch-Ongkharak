CREATE OR REPLACE FUNCTION enqueue_road_status_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    event_source_type TEXT;
    current_condition_id UUID;
    previous_effective_status TEXT;
    new_effective_status TEXT;
BEGIN
    PERFORM 1 FROM roads WHERE id = NEW.road_id FOR NO KEY UPDATE;
    SELECT source_type INTO event_source_type FROM data_sources WHERE id = NEW.source_id AND source_type IN ('OFFICIAL', 'COMMUNITY', 'AI');
    IF NOT FOUND THEN
        RETURN NEW;
    END IF;
    SELECT id INTO current_condition_id FROM road_conditions WHERE road_id = NEW.road_id ORDER BY observed_at DESC, received_at DESC, id DESC LIMIT 1;
    IF current_condition_id <> NEW.id THEN
        RETURN NEW;
    END IF;
    SELECT CASE WHEN verification_status = 'VERIFIED' THEN status ELSE 'UNKNOWN' END INTO previous_effective_status
    FROM road_conditions WHERE road_id = NEW.road_id AND id <> NEW.id
    ORDER BY observed_at DESC, received_at DESC, id DESC LIMIT 1;
    new_effective_status := CASE WHEN NEW.verification_status = 'VERIFIED' THEN NEW.status ELSE 'UNKNOWN' END;
    IF previous_effective_status IS NOT NULL AND previous_effective_status = new_effective_status THEN
        RETURN NEW;
    END IF;
    INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload)
    VALUES (gen_random_uuid(), 'road.status.changed', NEW.received_at, event_source_type, 'ROAD', NEW.road_id, jsonb_build_object('evidenceId', NEW.id));
    RETURN NEW;
END;
$$;
