-- Private-theme → official-catalog promotion requests. Mirrors the
-- subdomain_requests apply-approve pattern (0002_short_subdomains.sql):
-- pending/approved/rejected/revoked state machine, CAS updates instead of
-- an explicit version column, one active (pending) request per theme.
--
-- owner_id is a snapshot of who applied: once approved, themes.owner_id is
-- set to NULL by the promotion itself (scope flips to 'catalog'), so this
-- column is the only place "who submitted this" survives.
--
-- version_id snapshots themes.current_version_id at submission time; the
-- approval transaction re-checks it still matches before promoting, so an
-- admin can never approve content different from what they reviewed.
CREATE TABLE theme_catalog_requests (
    id TEXT PRIMARY KEY,
    theme_id TEXT NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'revoked')),
    reason TEXT NOT NULL DEFAULT '' CHECK (length(reason) <= 300),
    version_id TEXT NOT NULL REFERENCES theme_versions(id),
    reviewer_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    applied_at TEXT NOT NULL,
    reviewed_at TEXT
);

CREATE UNIQUE INDEX idx_theme_catalog_active ON theme_catalog_requests(theme_id) WHERE status = 'pending';
CREATE INDEX idx_theme_catalog_status_time ON theme_catalog_requests(status, applied_at DESC);
