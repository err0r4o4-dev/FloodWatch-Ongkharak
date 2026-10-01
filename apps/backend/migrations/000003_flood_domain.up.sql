CREATE TABLE data_sources (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK (source_type IN ('OFFICIAL', 'COMMUNITY', 'AI')),
    provider TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_successful_fetch_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT data_sources_code_format CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{0,63}$'),
    CONSTRAINT data_sources_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT data_sources_provider_nonempty CHECK (btrim(provider) <> '')
);

CREATE TABLE water_stations (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES data_sources (id),
    external_id TEXT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    location geometry(Point, 4326) NOT NULL,
    district TEXT,
    province TEXT,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source_id, code),
    UNIQUE (source_id, external_id),
    CONSTRAINT water_stations_code_nonempty CHECK (btrim(code) <> ''),
    CONSTRAINT water_stations_name_nonempty CHECK (btrim(name) <> ''),
    CONSTRAINT water_stations_location_valid CHECK (NOT ST_IsEmpty(location) AND ST_IsValid(location)
        AND ST_X(location) BETWEEN -180 AND 180 AND ST_Y(location) BETWEEN -90 AND 90)
);
CREATE INDEX water_stations_location_gist ON water_stations USING gist (location);

CREATE TABLE water_readings (
    id UUID PRIMARY KEY,
    station_id UUID NOT NULL REFERENCES water_stations (id),
    source_id UUID NOT NULL REFERENCES data_sources (id),
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    water_level_m NUMERIC(12, 4),
    flow_rate_m3s NUMERIC(14, 4),
    rainfall_mm NUMERIC(12, 4),
    verification_status TEXT NOT NULL DEFAULT 'UNVERIFIED' CHECK (verification_status IN ('UNVERIFIED', 'VERIFIED', 'REJECTED')),
    confidence NUMERIC(4, 3) CHECK (confidence BETWEEN 0 AND 1),
    CONSTRAINT water_readings_observation_unique UNIQUE (station_id, source_id, observed_at),
    CONSTRAINT water_readings_has_measurement CHECK (water_level_m IS NOT NULL OR flow_rate_m3s IS NOT NULL OR rainfall_mm IS NOT NULL),
    CONSTRAINT water_readings_nonnegative_flows CHECK (flow_rate_m3s >= 0 AND rainfall_mm >= 0),
    CONSTRAINT water_readings_time_order CHECK (received_at >= observed_at)
);
CREATE INDEX water_readings_station_time_idx ON water_readings (station_id, observed_at DESC);

CREATE TABLE flood_events (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES data_sources (id),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    description TEXT NOT NULL DEFAULT '',
    geometry geometry(Geometry, 4326) NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('INFO', 'WATCH', 'WARNING', 'CRITICAL', 'UNKNOWN')),
    status TEXT NOT NULL CHECK (status IN ('ACTIVE', 'RESOLVED', 'UNKNOWN')),
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    starts_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    verification_status TEXT NOT NULL DEFAULT 'UNVERIFIED' CHECK (verification_status IN ('UNVERIFIED', 'VERIFIED', 'REJECTED')),
    confidence NUMERIC(4, 3) CHECK (confidence BETWEEN 0 AND 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT flood_events_geometry_valid CHECK (NOT ST_IsEmpty(geometry) AND ST_IsValid(geometry)
        AND GeometryType(geometry) IN ('POINT', 'POLYGON', 'MULTIPOLYGON')
        AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180, -90, 180, 90, 4326))),
    CONSTRAINT flood_events_time_order CHECK (received_at >= observed_at AND (ended_at IS NULL OR starts_at IS NULL OR ended_at >= starts_at))
);
CREATE INDEX flood_events_geometry_gist ON flood_events USING gist (geometry);
CREATE INDEX flood_events_observed_idx ON flood_events (observed_at DESC);

CREATE TABLE risk_areas (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES data_sources (id),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    description TEXT NOT NULL DEFAULT '',
    geometry geometry(MultiPolygon, 4326) NOT NULL,
    risk_level TEXT NOT NULL CHECK (risk_level IN ('LOW', 'MODERATE', 'HIGH', 'VERY_HIGH', 'UNKNOWN')),
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT risk_areas_geometry_valid CHECK (NOT ST_IsEmpty(geometry) AND ST_IsValid(geometry)
        AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180, -90, 180, 90, 4326))),
    CONSTRAINT risk_areas_time_order CHECK (valid_to IS NULL OR valid_from IS NULL OR valid_to >= valid_from)
);
CREATE INDEX risk_areas_geometry_gist ON risk_areas USING gist (geometry);

CREATE TABLE roads (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE CHECK (btrim(code) <> ''),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    geometry geometry(MultiLineString, 4326) NOT NULL,
    road_class TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT roads_geometry_valid CHECK (NOT ST_IsEmpty(geometry) AND ST_IsValid(geometry)
        AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180, -90, 180, 90, 4326)))
);
CREATE INDEX roads_geometry_gist ON roads USING gist (geometry);

CREATE TABLE road_conditions (
    id UUID PRIMARY KEY,
    road_id UUID NOT NULL REFERENCES roads (id),
    source_id UUID NOT NULL REFERENCES data_sources (id),
    status TEXT NOT NULL CHECK (status IN ('PASSABLE', 'WATCH', 'SMALL_VEHICLE_NOT_RECOMMENDED', 'CLOSED', 'UNKNOWN')),
    description TEXT NOT NULL DEFAULT '',
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    valid_until TIMESTAMPTZ,
    water_depth_m NUMERIC(10, 3) CHECK (water_depth_m >= 0),
    verification_status TEXT NOT NULL DEFAULT 'UNVERIFIED' CHECK (verification_status IN ('UNVERIFIED', 'VERIFIED', 'REJECTED')),
    confidence NUMERIC(4, 3) CHECK (confidence BETWEEN 0 AND 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT road_conditions_observation_unique UNIQUE (road_id, source_id, observed_at),
    CONSTRAINT road_conditions_time_order CHECK (received_at >= observed_at AND (valid_until IS NULL OR valid_until >= observed_at))
);
CREATE INDEX road_conditions_road_time_idx ON road_conditions (road_id, observed_at DESC);
