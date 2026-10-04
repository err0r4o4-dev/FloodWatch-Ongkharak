DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM community_reports
        WHERE user_id IS NULL OR guest_upload_token_hash IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot roll back guest reports while guest rows exist';
    END IF;
END $$;

ALTER TABLE community_reports
    DROP CONSTRAINT community_reports_guest_upload_capability_valid,
    DROP CONSTRAINT community_reports_display_name_valid,
    DROP COLUMN guest_upload_token_expires_at,
    DROP COLUMN guest_upload_token_hash,
    DROP COLUMN display_name,
    ALTER COLUMN user_id SET NOT NULL;
