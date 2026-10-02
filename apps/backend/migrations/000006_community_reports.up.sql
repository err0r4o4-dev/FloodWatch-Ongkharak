CREATE TABLE community_reports (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id),
    type TEXT NOT NULL CHECK (type IN ('FLOOD', 'ROAD', 'OTHER')),
    description TEXT NOT NULL DEFAULT '',
    location geometry(Point, 4326) NOT NULL,
    estimated_water_depth_m NUMERIC(10, 3),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'VERIFIED', 'REJECTED')),
    source_type TEXT NOT NULL DEFAULT 'COMMUNITY' CHECK (source_type = 'COMMUNITY'),
    verified_at TIMESTAMPTZ,
    verified_by UUID REFERENCES users (id),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users (id),
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT community_reports_location_valid CHECK (
        NOT ST_IsEmpty(location) AND ST_IsValid(location)
        AND ST_X(location) BETWEEN -180 AND 180 AND ST_Y(location) BETWEEN -90 AND 90
    ),
    CONSTRAINT community_reports_estimated_depth_valid CHECK (
        estimated_water_depth_m IS NULL OR (
            estimated_water_depth_m BETWEEN 0 AND 100
            AND estimated_water_depth_m > '-Infinity'::numeric
            AND estimated_water_depth_m < 'Infinity'::numeric
        )
    ),
    CONSTRAINT community_reports_verification_valid CHECK (
        (status = 'VERIFIED' AND verified_at IS NOT NULL AND verified_by IS NOT NULL AND reviewed_at = verified_at AND reviewed_by = verified_by AND rejection_reason IS NULL)
        OR (status = 'REJECTED' AND verified_at IS NULL AND verified_by IS NULL AND reviewed_at IS NOT NULL AND reviewed_by IS NOT NULL)
        OR (status = 'PENDING' AND verified_at IS NULL AND verified_by IS NULL AND reviewed_at IS NULL AND reviewed_by IS NULL AND rejection_reason IS NULL)
    ),
    CONSTRAINT community_reports_rejection_reason_valid CHECK (
        rejection_reason IS NULL OR char_length(rejection_reason) <= 500
    )
);
CREATE INDEX community_reports_location_gist ON community_reports USING gist (location);
CREATE INDEX community_reports_verified_created_idx ON community_reports (created_at DESC) WHERE status = 'VERIFIED';
CREATE INDEX community_reports_user_created_idx ON community_reports (user_id, created_at DESC);
CREATE INDEX community_reports_status_created_idx ON community_reports (status, created_at DESC);

CREATE TABLE community_report_media (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES community_reports (id) ON DELETE CASCADE,
    object_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    size_bytes INTEGER NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT community_report_media_object_key_valid CHECK (object_key ~ '^reports/[0-9a-f-]{36}/[0-9a-f-]{36}\.(jpg|png|webp)$')
);
CREATE INDEX community_report_media_report_created_idx ON community_report_media (report_id, created_at ASC);
