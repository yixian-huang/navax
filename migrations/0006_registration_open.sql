-- Allow public (open) self-registration in addition to invite-only and closed.
-- SQLite cannot alter CHECK constraints in place; rebuild system_settings.

CREATE TABLE system_settings_new (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    initialized INTEGER NOT NULL DEFAULT 0 CHECK (initialized IN (0, 1)),
    instance_name TEXT NOT NULL DEFAULT 'nav.ax' CHECK (length(instance_name) BETWEEN 1 AND 60),
    public_base_url TEXT NOT NULL DEFAULT '' CHECK (length(public_base_url) <= 2048),
    registration_mode TEXT NOT NULL DEFAULT 'closed' CHECK (registration_mode IN ('invite', 'closed', 'open')),
    max_categories_per_page INTEGER NOT NULL DEFAULT 50 CHECK (max_categories_per_page BETWEEN 1 AND 500),
    max_sites_per_page INTEGER NOT NULL DEFAULT 1000 CHECK (max_sites_per_page BETWEEN 1 AND 10000),
    max_upload_bytes INTEGER NOT NULL DEFAULT 5242880 CHECK (max_upload_bytes BETWEEN 1024 AND 52428800),
    discover_enabled INTEGER NOT NULL DEFAULT 1 CHECK (discover_enabled IN (0, 1)),
    analytics_enabled INTEGER NOT NULL DEFAULT 1 CHECK (analytics_enabled IN (0, 1)),
    analytics_retention_days INTEGER NOT NULL DEFAULT 90 CHECK (analytics_retention_days BETWEEN 7 AND 365),
    root_domain TEXT CHECK (root_domain IS NULL OR length(root_domain) BETWEEN 1 AND 253),
    subdomains_enabled INTEGER NOT NULL DEFAULT 0 CHECK (subdomains_enabled IN (0, 1)),
    updated_at TEXT NOT NULL
);

INSERT INTO system_settings_new (
    id, initialized, instance_name, public_base_url, registration_mode,
    max_categories_per_page, max_sites_per_page, max_upload_bytes,
    discover_enabled, analytics_enabled, analytics_retention_days,
    root_domain, subdomains_enabled, updated_at
)
SELECT
    id, initialized, instance_name, public_base_url, registration_mode,
    max_categories_per_page, max_sites_per_page, max_upload_bytes,
    discover_enabled, analytics_enabled, analytics_retention_days,
    root_domain, subdomains_enabled, updated_at
FROM system_settings;

DROP TABLE system_settings;
ALTER TABLE system_settings_new RENAME TO system_settings;
