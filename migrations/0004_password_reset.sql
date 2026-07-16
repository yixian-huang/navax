CREATE TABLE password_reset_tokens (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) >= 32),
    expires_at TEXT NOT NULL,
    used_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_password_reset_active ON password_reset_tokens(user_id) WHERE used_at IS NULL;
