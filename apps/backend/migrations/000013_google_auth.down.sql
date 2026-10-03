DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM users WHERE password_hash IS NULL) THEN
        RAISE EXCEPTION 'Google-only accounts exist; explicitly provision local credentials before rollback';
    END IF;
END $$;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
DROP TABLE auth_google_pending_links;
DROP TABLE auth_google_transactions;
DROP TABLE auth_identities;
