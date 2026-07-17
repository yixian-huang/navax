-- Allow OAuth registration to complete via email verification code.

CREATE TABLE auth_email_codes_v2 (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    email TEXT NOT NULL COLLATE NOCASE CHECK (length(email) BETWEEN 3 AND 254),
    purpose TEXT NOT NULL CHECK (purpose IN ('register', 'login', 'oauth_register')),
    code_hash BLOB NOT NULL CHECK (length(code_hash) >= 32),
    payload_json TEXT NOT NULL DEFAULT '{}' CHECK (length(payload_json) <= 4096),
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TEXT NOT NULL
);

INSERT INTO auth_email_codes_v2 (
    id, email, purpose, code_hash, payload_json, expires_at, consumed_at, attempts, created_at
)
SELECT id, email, purpose, code_hash, payload_json, expires_at, consumed_at, attempts, created_at
FROM auth_email_codes;

DROP TABLE auth_email_codes;
ALTER TABLE auth_email_codes_v2 RENAME TO auth_email_codes;

CREATE INDEX idx_auth_email_codes_lookup
    ON auth_email_codes(email, purpose, expires_at DESC)
    WHERE consumed_at IS NULL;
