-- Email one-time codes (register / passwordless login) and OAuth identities.

CREATE TABLE auth_email_codes (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    email TEXT NOT NULL COLLATE NOCASE CHECK (length(email) BETWEEN 3 AND 254),
    purpose TEXT NOT NULL CHECK (purpose IN ('register', 'login')),
    code_hash BLOB NOT NULL CHECK (length(code_hash) >= 32),
    payload_json TEXT NOT NULL DEFAULT '{}' CHECK (length(payload_json) <= 4096),
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TEXT NOT NULL
);

CREATE INDEX idx_auth_email_codes_lookup
    ON auth_email_codes(email, purpose, expires_at DESC)
    WHERE consumed_at IS NULL;

CREATE TABLE oauth_identities (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    provider TEXT NOT NULL CHECK (provider IN ('google', 'github')),
    subject TEXT NOT NULL CHECK (length(subject) BETWEEN 1 AND 255),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email TEXT NOT NULL DEFAULT '' COLLATE NOCASE CHECK (length(email) <= 254),
    created_at TEXT NOT NULL,
    UNIQUE (provider, subject)
);

CREATE INDEX idx_oauth_identities_user ON oauth_identities(user_id);

CREATE TABLE oauth_states (
    state TEXT PRIMARY KEY CHECK (length(state) BETWEEN 16 AND 128),
    provider TEXT NOT NULL CHECK (provider IN ('google', 'github')),
    invitation_token TEXT NOT NULL DEFAULT '' CHECK (length(invitation_token) <= 512),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

-- Expand provider kind CHECK to allow OAuth apps (SQLite cannot ALTER CHECK in place).
CREATE TABLE provider_configs_v2 (
    kind TEXT PRIMARY KEY CHECK (kind IN ('smtp', 'storage', 'dns', 'oauth_google', 'oauth_github')),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    settings_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(settings_json)),
    secrets_ciphertext BLOB,
    secret_nonce BLOB,
    updated_at TEXT,
    CHECK ((secrets_ciphertext IS NULL AND secret_nonce IS NULL) OR
           (secrets_ciphertext IS NOT NULL AND secret_nonce IS NOT NULL))
);
INSERT INTO provider_configs_v2 (kind, enabled, settings_json, secrets_ciphertext, secret_nonce, updated_at)
SELECT kind, enabled, settings_json, secrets_ciphertext, secret_nonce, updated_at FROM provider_configs;
DROP TABLE provider_configs;
ALTER TABLE provider_configs_v2 RENAME TO provider_configs;

INSERT INTO provider_configs (kind) VALUES ('oauth_google'), ('oauth_github');
