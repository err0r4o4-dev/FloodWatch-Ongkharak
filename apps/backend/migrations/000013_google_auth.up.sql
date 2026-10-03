ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

CREATE TABLE auth_identities (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider = 'GOOGLE'),
    provider_subject TEXT NOT NULL CHECK (provider_subject ~ '^[!-~]{1,255}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, provider_subject),
    UNIQUE (user_id, provider)
);

CREATE TABLE auth_google_transactions (
    state_hash BYTEA PRIMARY KEY CHECK (octet_length(state_hash) = 32),
    browser_hash BYTEA NOT NULL CHECK (octet_length(browser_hash) = 32),
    nonce TEXT NOT NULL CHECK (length(nonce) BETWEEN 1 AND 128),
    verifier TEXT NOT NULL CHECK (length(verifier) BETWEEN 1 AND 128),
    return_to TEXT NOT NULL CHECK (length(return_to) BETWEEN 1 AND 2048),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX auth_google_transactions_expiry_idx ON auth_google_transactions(expires_at);

CREATE TABLE auth_google_pending_links (
    token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash) = 32),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_subject TEXT NOT NULL CHECK (provider_subject ~ '^[!-~]{1,255}$'),
    return_to TEXT NOT NULL CHECK (length(return_to) BETWEEN 1 AND 2048),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX auth_google_pending_expiry_idx ON auth_google_pending_links(expires_at);
CREATE INDEX auth_google_pending_user_idx ON auth_google_pending_links(user_id);
