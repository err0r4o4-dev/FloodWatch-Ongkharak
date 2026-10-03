ALTER TABLE users ADD COLUMN username TEXT;
ALTER TABLE users ADD COLUMN auth_version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD CONSTRAINT users_username_format CHECK (username IS NULL OR username ~ '^[a-z0-9_]{3,32}$');
ALTER TABLE users ADD CONSTRAINT users_username_unique UNIQUE (username);
ALTER TABLE users ADD CONSTRAINT users_auth_version_nonnegative CHECK (auth_version >= 0);

CREATE TABLE password_recovery_challenges (
    id UUID PRIMARY KEY,
    email_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(email_hash) = 32),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    auth_version BIGINT NOT NULL CHECK (auth_version >= 0),
    otp_hash BYTEA NOT NULL CHECK (octet_length(otp_hash) = 32),
    issued_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL CHECK (expires_at > issued_at),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    window_started_at TIMESTAMPTZ NOT NULL,
    request_count INTEGER NOT NULL CHECK (request_count BETWEEN 1 AND 5),
    verified_at TIMESTAMPTZ,
    reset_token_hash BYTEA UNIQUE CHECK (reset_token_hash IS NULL OR octet_length(reset_token_hash) = 32),
    reset_expires_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    CHECK ((verified_at IS NULL) = (reset_token_hash IS NULL)),
    CHECK ((verified_at IS NULL) = (reset_expires_at IS NULL)),
    CHECK (reset_expires_at IS NULL OR reset_expires_at > verified_at),
    CHECK (consumed_at IS NULL OR verified_at IS NOT NULL)
);

CREATE INDEX password_recovery_expiry_idx ON password_recovery_challenges(expires_at);
