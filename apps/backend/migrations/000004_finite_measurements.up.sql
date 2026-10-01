ALTER TABLE water_readings
    ADD CONSTRAINT water_readings_finite_measurements CHECK (
        (water_level_m IS NULL OR water_level_m > '-Infinity'::numeric AND water_level_m < 'Infinity'::numeric)
        AND (flow_rate_m3s IS NULL OR flow_rate_m3s > '-Infinity'::numeric AND flow_rate_m3s < 'Infinity'::numeric)
        AND (rainfall_mm IS NULL OR rainfall_mm > '-Infinity'::numeric AND rainfall_mm < 'Infinity'::numeric)
        AND (confidence IS NULL OR confidence > '-Infinity'::numeric AND confidence < 'Infinity'::numeric)
    );

ALTER TABLE flood_events
    ADD CONSTRAINT flood_events_finite_confidence CHECK (
        confidence IS NULL OR confidence > '-Infinity'::numeric AND confidence < 'Infinity'::numeric
    );

ALTER TABLE road_conditions
    ADD CONSTRAINT road_conditions_finite_measurements CHECK (
        (water_depth_m IS NULL OR water_depth_m > '-Infinity'::numeric AND water_depth_m < 'Infinity'::numeric)
        AND (confidence IS NULL OR confidence > '-Infinity'::numeric AND confidence < 'Infinity'::numeric)
    );
