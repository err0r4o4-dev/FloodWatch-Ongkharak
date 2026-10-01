CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    disabled_at TIMESTAMPTZ,
    CONSTRAINT users_email_normalized CHECK (email = lower(btrim(email)) AND email <> ''),
    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_role_check CHECK (role IN ('USER', 'ADMIN'))
);

CREATE TABLE refresh_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    rotated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    replaced_by UUID REFERENCES refresh_sessions (id),
    CONSTRAINT refresh_sessions_token_hash_unique UNIQUE (token_hash),
    CONSTRAINT refresh_sessions_token_hash_sha256 CHECK (octet_length(token_hash) = 32),
    CONSTRAINT refresh_sessions_expiry_after_issue CHECK (expires_at > issued_at),
    CONSTRAINT refresh_sessions_rotation_replacement CHECK ((rotated_at IS NULL) = (replaced_by IS NULL))
);

CREATE INDEX refresh_sessions_user_id_idx ON refresh_sessions (user_id);
CREATE INDEX refresh_sessions_active_expiry_idx ON refresh_sessions (expires_at)
    WHERE revoked_at IS NULL AND rotated_at IS NULL;
