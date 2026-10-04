ALTER TABLE notification_subscriptions
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1;
ALTER TABLE notification_subscriptions
    ADD CONSTRAINT notification_subscriptions_version_positive CHECK (version > 0);

CREATE TABLE ai_analysis_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID NOT NULL REFERENCES community_reports (id) ON DELETE CASCADE,
    media_id UUID NOT NULL REFERENCES community_report_media (id) ON DELETE CASCADE,
    report_updated_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    location geometry(Point, 4326) NOT NULL,
    object_key TEXT NOT NULL,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    status TEXT NOT NULL DEFAULT 'QUEUED' CHECK (status IN ('QUEUED', 'PROCESSING', 'SUCCEEDED', 'FAILED')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    lease_token UUID NOT NULL DEFAULT gen_random_uuid(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    leased_until TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ai_analysis_jobs_location_valid CHECK (NOT ST_IsEmpty(location) AND ST_IsValid(location)),
    CONSTRAINT ai_analysis_jobs_attempts_bounded CHECK (attempts <= 3)
);
CREATE INDEX ai_analysis_jobs_claim_idx ON ai_analysis_jobs (status, available_at, created_at)
    WHERE status IN ('QUEUED', 'PROCESSING');
CREATE INDEX ai_analysis_jobs_report_idx ON ai_analysis_jobs (report_id, created_at);
CREATE UNIQUE INDEX ai_analysis_jobs_media_revision_uq ON ai_analysis_jobs (media_id, report_updated_at);

CREATE TABLE ai_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID NOT NULL REFERENCES community_reports (id) ON DELETE CASCADE,
    media_id UUID NOT NULL REFERENCES community_report_media (id) ON DELETE CASCADE,
    source_id UUID NOT NULL REFERENCES data_sources (id),
    model_version TEXT NOT NULL CHECK (btrim(model_version) <> '' AND char_length(model_version) BETWEEN 1 AND 100),
    flood JSONB,
    road JSONB,
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    report_updated_at TIMESTAMPTZ NOT NULL,
    location geometry(Point, 4326) NOT NULL,
    limitations JSONB NOT NULL CHECK (jsonb_typeof(limitations) = 'array' AND jsonb_array_length(limitations) BETWEEN 1 AND 10),
    status TEXT NOT NULL DEFAULT 'UNVERIFIED' CHECK (status IN ('UNVERIFIED', 'VERIFIED', 'REJECTED')),
    review_reason TEXT,
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ai_evidence_location_valid CHECK (NOT ST_IsEmpty(location) AND ST_IsValid(location)),
    CONSTRAINT ai_evidence_reason_bounded CHECK (review_reason IS NULL OR char_length(review_reason) BETWEEN 1 AND 500),
    CONSTRAINT ai_evidence_review_fields CHECK (
        (status = 'UNVERIFIED' AND reviewed_at IS NULL AND reviewed_by IS NULL AND review_reason IS NULL)
        OR (status IN ('VERIFIED', 'REJECTED') AND reviewed_at IS NOT NULL AND reviewed_by IS NOT NULL)
    )
);
CREATE INDEX ai_evidence_status_idx ON ai_evidence (status, received_at DESC, id DESC);
CREATE INDEX ai_evidence_report_idx ON ai_evidence (report_id, received_at DESC);
CREATE UNIQUE INDEX ai_evidence_media_revision_uq ON ai_evidence (media_id, report_updated_at);
