CREATE TABLE subdomain_requests_next (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL COLLATE NOCASE CHECK (length(label) BETWEEN 1 AND 30),
    full_domain TEXT NOT NULL COLLATE NOCASE CHECK (length(full_domain) BETWEEN 3 AND 253),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'revoked')),
    reason TEXT NOT NULL DEFAULT '' CHECK (length(reason) <= 300),
    reviewer_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    applied_at TEXT NOT NULL,
    reviewed_at TEXT
);

INSERT INTO subdomain_requests_next (
    id, user_id, label, full_domain, status, reason, reviewer_id, applied_at, reviewed_at
)
SELECT id, user_id, label, full_domain, status, reason, reviewer_id, applied_at, reviewed_at
FROM subdomain_requests;

DROP TABLE subdomain_requests;
ALTER TABLE subdomain_requests_next RENAME TO subdomain_requests;

CREATE UNIQUE INDEX idx_subdomain_active_user
    ON subdomain_requests(user_id) WHERE status IN ('pending', 'approved');
CREATE UNIQUE INDEX idx_subdomain_active_label
    ON subdomain_requests(label) WHERE status IN ('pending', 'approved');
CREATE INDEX idx_subdomain_status_time
    ON subdomain_requests(status, applied_at DESC);
