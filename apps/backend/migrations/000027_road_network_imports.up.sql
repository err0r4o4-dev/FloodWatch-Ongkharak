CREATE TABLE road_network_imports (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider = 'OPENSTREETMAP'),
    source_url TEXT NOT NULL CHECK (source_url LIKE 'https://api.openstreetmap.org/%'),
    license TEXT NOT NULL CHECK (license = 'ODbL-1.0'),
    retrieved_at TIMESTAMPTZ NOT NULL,
    source_timestamp TIMESTAMPTZ,
    content_sha256 TEXT NOT NULL UNIQUE CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
    bbox JSONB NOT NULL CHECK (jsonb_typeof(bbox) = 'array' AND jsonb_array_length(bbox) = 4),
    road_count INTEGER NOT NULL CHECK (road_count >= 0),
    segment_count INTEGER NOT NULL CHECK (segment_count >= 0),
    raw_snapshot_path TEXT NOT NULL CHECK (btrim(raw_snapshot_path) <> '')
);

ALTER TABLE roads ADD COLUMN road_network_import_id UUID REFERENCES road_network_imports (id);
CREATE INDEX roads_network_import_idx ON roads (road_network_import_id) WHERE road_network_import_id IS NOT NULL;
