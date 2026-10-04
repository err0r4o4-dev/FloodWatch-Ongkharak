ALTER TABLE community_reports
    ALTER COLUMN user_id DROP NOT NULL,
    ADD COLUMN display_name TEXT NOT NULL DEFAULT 'Guest',
    ADD COLUMN guest_upload_token_hash BYTEA,
    ADD COLUMN guest_upload_token_expires_at TIMESTAMPTZ;

ALTER TABLE community_reports
    ADD CONSTRAINT community_reports_display_name_valid CHECK (
        btrim(display_name) <> '' AND char_length(display_name) <= 80
    ),
    ADD CONSTRAINT community_reports_guest_upload_capability_valid CHECK (
        (user_id IS NOT NULL OR guest_upload_token_hash IS NOT NULL)
        AND
        (guest_upload_token_hash IS NULL) = (guest_upload_token_expires_at IS NULL)
        AND (guest_upload_token_hash IS NULL OR octet_length(guest_upload_token_hash) = 32)
        AND (guest_upload_token_expires_at IS NULL OR guest_upload_token_expires_at > created_at)
    );
