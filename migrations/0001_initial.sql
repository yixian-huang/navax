CREATE TABLE users (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    username TEXT NOT NULL COLLATE NOCASE CHECK (length(username) BETWEEN 3 AND 32),
    email TEXT NOT NULL COLLATE NOCASE CHECK (length(email) BETWEEN 3 AND 254),
    password_hash TEXT NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '' CHECK (length(avatar_url) <= 2048),
    bio TEXT NOT NULL DEFAULT '' CHECK (length(bio) <= 300),
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (username),
    UNIQUE (email)
);

CREATE INDEX idx_users_status_created_at ON users(status, created_at DESC);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) >= 32),
    device TEXT NOT NULL DEFAULT '' CHECK (length(device) <= 300),
    approximate_location TEXT NOT NULL DEFAULT '' CHECK (length(approximate_location) <= 120),
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT
);

CREATE INDEX idx_sessions_user_active ON sessions(user_id, expires_at DESC) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_expiry ON sessions(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE invitations (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    token_hash BLOB NOT NULL UNIQUE CHECK (length(token_hash) >= 32),
    token_preview TEXT NOT NULL CHECK (length(token_preview) BETWEEN 1 AND 32),
    creator_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    email TEXT COLLATE NOCASE CHECK (email IS NULL OR length(email) BETWEEN 3 AND 254),
    max_uses INTEGER NOT NULL CHECK (max_uses BETWEEN 1 AND 100),
    used_count INTEGER NOT NULL DEFAULT 0 CHECK (used_count BETWEEN 0 AND max_uses),
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_invitations_active ON invitations(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_invitations_creator ON invitations(creator_id, created_at DESC);

CREATE TABLE invitation_redemptions (
    invitation_id TEXT NOT NULL REFERENCES invitations(id) ON DELETE RESTRICT,
    user_id TEXT NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    redeemed_at TEXT NOT NULL,
    PRIMARY KEY (invitation_id, user_id)
);

CREATE TABLE themes (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 1 AND 64),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    version TEXT NOT NULL CHECK (length(version) BETWEEN 1 AND 40),
    author TEXT NOT NULL CHECK (length(author) BETWEEN 1 AND 100),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    mode TEXT NOT NULL CHECK (mode IN ('light', 'dark', 'both')),
    preview TEXT NOT NULL DEFAULT '' CHECK (length(preview) <= 2048),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    is_default INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_themes_single_default ON themes(is_default) WHERE is_default = 1;

CREATE TABLE navigation_pages (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    kind TEXT NOT NULL CHECK (kind IN ('personal', 'system')),
    owner_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 100),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 300),
    draft_revision INTEGER NOT NULL DEFAULT 0 CHECK (draft_revision >= 0),
    settings_json TEXT NOT NULL CHECK (json_valid(settings_json)),
    draft_updated_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((kind = 'personal' AND owner_id IS NOT NULL) OR (kind = 'system' AND owner_id IS NULL))
);

CREATE UNIQUE INDEX idx_navigation_pages_personal_owner ON navigation_pages(owner_id) WHERE kind = 'personal';
CREATE UNIQUE INDEX idx_navigation_pages_single_system ON navigation_pages(kind) WHERE kind = 'system';

CREATE TABLE categories (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    page_id TEXT NOT NULL REFERENCES navigation_pages(id) ON DELETE CASCADE,
    name TEXT NOT NULL COLLATE NOCASE CHECK (length(name) BETWEEN 1 AND 60),
    icon TEXT NOT NULL DEFAULT '' CHECK (length(icon) <= 256),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    is_uncategorized INTEGER NOT NULL DEFAULT 0 CHECK (is_uncategorized IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, page_id),
    UNIQUE (page_id, name)
);

CREATE UNIQUE INDEX idx_categories_uncategorized ON categories(page_id) WHERE is_uncategorized = 1;
CREATE INDEX idx_categories_page_order ON categories(page_id, sort_order, id);

CREATE TABLE sites (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    page_id TEXT NOT NULL,
    category_id TEXT NOT NULL,
    title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 100),
    url TEXT NOT NULL CHECK (length(url) <= 2048 AND (url LIKE 'http://%' OR url LIKE 'https://%')),
    icon TEXT NOT NULL DEFAULT '' CHECK (length(icon) <= 2048),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 300),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (category_id, page_id) REFERENCES categories(id, page_id) ON DELETE CASCADE,
    UNIQUE (page_id, url)
);

CREATE INDEX idx_sites_category_order ON sites(category_id, sort_order, id);
CREATE INDEX idx_sites_page ON sites(page_id);

CREATE TABLE published_snapshots (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    page_id TEXT NOT NULL REFERENCES navigation_pages(id) ON DELETE CASCADE,
    draft_revision INTEGER NOT NULL CHECK (draft_revision >= 0),
    slug TEXT NOT NULL CHECK (length(slug) BETWEEN 1 AND 48),
    visibility TEXT NOT NULL CHECK (visibility IN ('unlisted', 'public')),
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    etag TEXT NOT NULL UNIQUE CHECK (length(etag) BETWEEN 3 AND 128),
    published_at TEXT NOT NULL
);

CREATE INDEX idx_published_snapshots_page_revision ON published_snapshots(page_id, draft_revision DESC, published_at DESC);

CREATE TABLE page_publications (
    page_id TEXT PRIMARY KEY REFERENCES navigation_pages(id) ON DELETE CASCADE,
    visibility TEXT NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'unlisted', 'public')),
    slug TEXT NOT NULL COLLATE NOCASE CHECK (length(slug) BETWEEN 1 AND 48),
    show_author INTEGER NOT NULL DEFAULT 1 CHECK (show_author IN (0, 1)),
    seo_title TEXT NOT NULL DEFAULT '' CHECK (length(seo_title) <= 70),
    seo_description TEXT NOT NULL DEFAULT '' CHECK (length(seo_description) <= 160),
    current_snapshot_id TEXT REFERENCES published_snapshots(id) ON DELETE SET NULL,
    featured INTEGER NOT NULL DEFAULT 0 CHECK (featured IN (0, 1)),
    tags_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(tags_json) AND json_type(tags_json) = 'array'),
    updated_at TEXT NOT NULL,
    UNIQUE (slug)
);

CREATE INDEX idx_page_publications_discover ON page_publications(visibility, featured DESC, updated_at DESC)
    WHERE visibility = 'public' AND current_snapshot_id IS NOT NULL;

CREATE TABLE subdomain_requests (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL COLLATE NOCASE CHECK (length(label) BETWEEN 3 AND 30),
    full_domain TEXT NOT NULL COLLATE NOCASE CHECK (length(full_domain) BETWEEN 3 AND 253),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'revoked')),
    reason TEXT NOT NULL DEFAULT '' CHECK (length(reason) <= 300),
    reviewer_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    applied_at TEXT NOT NULL,
    reviewed_at TEXT
);

CREATE UNIQUE INDEX idx_subdomain_active_user ON subdomain_requests(user_id) WHERE status IN ('pending', 'approved');
CREATE UNIQUE INDEX idx_subdomain_active_label ON subdomain_requests(label) WHERE status IN ('pending', 'approved');
CREATE INDEX idx_subdomain_status_time ON subdomain_requests(status, applied_at DESC);

CREATE TABLE directory_categories (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    name TEXT NOT NULL COLLATE NOCASE CHECK (length(name) BETWEEN 1 AND 60),
    icon TEXT NOT NULL DEFAULT '' CHECK (length(icon) <= 256),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (name)
);

CREATE INDEX idx_directory_categories_order ON directory_categories(enabled DESC, sort_order, id);

CREATE TABLE directory_sites (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    category_id TEXT NOT NULL REFERENCES directory_categories(id) ON DELETE CASCADE,
    title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 100),
    url TEXT NOT NULL CHECK (length(url) <= 2048 AND (url LIKE 'http://%' OR url LIKE 'https://%')),
    icon TEXT NOT NULL DEFAULT '' CHECK (length(icon) <= 2048),
    description TEXT NOT NULL DEFAULT '' CHECK (length(description) <= 300),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (category_id, url)
);

CREATE INDEX idx_directory_sites_category_order ON directory_sites(category_id, enabled DESC, sort_order, id);

CREATE TABLE assets (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('avatar', 'background', 'site-icon')),
    storage_driver TEXT NOT NULL CHECK (storage_driver IN ('local', 's3')),
    object_key TEXT NOT NULL UNIQUE CHECK (length(object_key) BETWEEN 1 AND 1024),
    url TEXT NOT NULL CHECK (length(url) BETWEEN 1 AND 2048),
    mime_type TEXT NOT NULL CHECK (length(mime_type) BETWEEN 1 AND 100),
    size_bytes INTEGER NOT NULL CHECK (size_bytes > 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    created_at TEXT NOT NULL
);

CREATE INDEX idx_assets_owner_time ON assets(owner_id, created_at DESC);

CREATE TABLE analytics_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    page_id TEXT NOT NULL REFERENCES navigation_pages(id) ON DELETE CASCADE,
    snapshot_id TEXT REFERENCES published_snapshots(id) ON DELETE SET NULL,
    site_id TEXT REFERENCES sites(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('page_view', 'site_click')),
    client_event_id TEXT CHECK (client_event_id IS NULL OR length(client_event_id) <= 128),
    visitor_hash BLOB NOT NULL CHECK (length(visitor_hash) >= 16),
    country_code TEXT NOT NULL DEFAULT '' CHECK (length(country_code) IN (0, 2)),
    device TEXT NOT NULL DEFAULT '' CHECK (length(device) <= 40),
    referrer_domain TEXT NOT NULL DEFAULT '' CHECK (length(referrer_domain) <= 253),
    occurred_date TEXT NOT NULL CHECK (length(occurred_date) = 10),
    occurred_at TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_analytics_client_event ON analytics_events(page_id, client_event_id)
    WHERE client_event_id IS NOT NULL;
CREATE INDEX idx_analytics_page_date ON analytics_events(page_id, occurred_date, event_type);
CREATE INDEX idx_analytics_site_time ON analytics_events(site_id, occurred_at DESC) WHERE site_id IS NOT NULL;
CREATE INDEX idx_analytics_retention ON analytics_events(occurred_at);

CREATE TABLE analytics_daily_visitors (
    page_id TEXT NOT NULL REFERENCES navigation_pages(id) ON DELETE CASCADE,
    occurred_date TEXT NOT NULL CHECK (length(occurred_date) = 10),
    visitor_hash BLOB NOT NULL CHECK (length(visitor_hash) >= 16),
    PRIMARY KEY (page_id, occurred_date, visitor_hash)
) WITHOUT ROWID;

CREATE TABLE link_check_results (
    site_id TEXT PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('reachable', 'unreachable', 'blocked', 'timeout')),
    http_status INTEGER CHECK (http_status IS NULL OR http_status BETWEEN 100 AND 599),
    latency_ms INTEGER CHECK (latency_ms IS NULL OR latency_ms >= 0),
    message TEXT NOT NULL DEFAULT '' CHECK (length(message) <= 500),
    checked_at TEXT NOT NULL
);

CREATE TABLE import_previews (
    token_hash BLOB PRIMARY KEY CHECK (length(token_hash) >= 32),
    page_id TEXT NOT NULL REFERENCES navigation_pages(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    format TEXT NOT NULL CHECK (format IN ('bookmarks-html', 'navax-json')),
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_import_previews_expiry ON import_previews(expires_at);

CREATE TABLE idempotency_records (
    scope TEXT NOT NULL CHECK (length(scope) BETWEEN 1 AND 100),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
    actor_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    request_hash BLOB NOT NULL CHECK (length(request_hash) >= 32),
    response_status INTEGER CHECK (response_status IS NULL OR response_status BETWEEN 100 AND 599),
    response_json TEXT CHECK (response_json IS NULL OR json_valid(response_json)),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (scope, idempotency_key)
) WITHOUT ROWID;

CREATE INDEX idx_idempotency_expiry ON idempotency_records(expires_at);

CREATE TABLE system_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    initialized INTEGER NOT NULL DEFAULT 0 CHECK (initialized IN (0, 1)),
    instance_name TEXT NOT NULL DEFAULT 'nav.ax' CHECK (length(instance_name) BETWEEN 1 AND 60),
    public_base_url TEXT NOT NULL DEFAULT '' CHECK (length(public_base_url) <= 2048),
    registration_mode TEXT NOT NULL DEFAULT 'closed' CHECK (registration_mode IN ('invite', 'closed')),
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

CREATE TABLE provider_configs (
    kind TEXT PRIMARY KEY CHECK (kind IN ('smtp', 'storage', 'dns')),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    settings_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(settings_json)),
    secrets_ciphertext BLOB,
    secret_nonce BLOB,
    updated_at TEXT,
    CHECK ((secrets_ciphertext IS NULL AND secret_nonce IS NULL) OR
           (secrets_ciphertext IS NOT NULL AND secret_nonce IS NOT NULL))
);

CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    actor_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    actor_name TEXT NOT NULL CHECK (length(actor_name) BETWEEN 1 AND 100),
    action TEXT NOT NULL CHECK (length(action) BETWEEN 1 AND 100),
    target_type TEXT NOT NULL CHECK (length(target_type) BETWEEN 1 AND 100),
    target_id TEXT NOT NULL DEFAULT '' CHECK (length(target_id) <= 128),
    detail_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(detail_json)),
    request_id TEXT NOT NULL DEFAULT '' CHECK (length(request_id) <= 128),
    created_at TEXT NOT NULL
);

CREATE INDEX idx_audit_logs_time ON audit_logs(created_at DESC, id);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_id, created_at DESC);

CREATE TABLE update_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    current_version TEXT NOT NULL DEFAULT 'dev',
    latest_version TEXT,
    deployment TEXT NOT NULL DEFAULT 'development' CHECK (deployment IN ('binary', 'container', 'development')),
    channel TEXT NOT NULL DEFAULT 'stable' CHECK (channel = 'stable'),
    auto_check INTEGER NOT NULL DEFAULT 1 CHECK (auto_check IN (0, 1)),
    auto_apply INTEGER NOT NULL DEFAULT 0 CHECK (auto_apply IN (0, 1)),
    maintenance_window TEXT,
    status TEXT NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'checking', 'available', 'downloading', 'applying', 'restart-required', 'failed')),
    release_notes TEXT NOT NULL DEFAULT '',
    manifest_json TEXT CHECK (manifest_json IS NULL OR json_valid(manifest_json)),
    checked_at TEXT,
    error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

CREATE TABLE update_history (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    from_version TEXT NOT NULL,
    to_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('started', 'succeeded', 'failed', 'rolled-back')),
    detail TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT
);

CREATE INDEX idx_update_history_time ON update_history(started_at DESC);

CREATE TABLE backups (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    reason TEXT NOT NULL CHECK (reason IN ('manual', 'pre-update', 'scheduled')),
    path TEXT NOT NULL UNIQUE CHECK (length(path) BETWEEN 1 AND 2048),
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_backups_time ON backups(created_at DESC);

CREATE TABLE restore_tokens (
    token_hash BLOB PRIMARY KEY CHECK (length(token_hash) >= 32),
    backup_id TEXT NOT NULL REFERENCES backups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    used_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_restore_tokens_expiry ON restore_tokens(expires_at) WHERE used_at IS NULL;

INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default, created_at, updated_at) VALUES
    ('slate', 'Slate', '1.0.0', 'nav.ax', '冷静中性，杂志般的编辑排版感。', 'both', '', 1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('kyoto', 'Kyoto', '1.0.0', 'nav.ax', '米纸奶白底，深林绿与赭石点缀。', 'both', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('noir', 'Noir', '1.0.0', 'nav.ax', '深黑画布上浮现金色与石榴红。', 'dark', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('terracotta', 'Terracotta', '1.0.0', 'nav.ax', '地中海与包豪斯碰撞的温暖陶土色。', 'both', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('sakura', 'Sakura', '1.0.0', 'nav.ax', '梦幻樱花粉与薄荷绿的轻盈组合。', 'light', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('mochi', 'Mochi', '1.0.0', 'nav.ax', '薰衣草紫与珊瑚橘的柔和主题。', 'light', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('pastelsky', 'Pastel Sky', '1.0.0', 'nav.ax', '晴空蓝与柠檬黄的轻快主题。', 'light', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('mono', 'Mono', '1.0.0', 'nav.ax', '高信息密度的纯粹黑白灰主题。', 'both', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

INSERT INTO navigation_pages (
    id, kind, owner_id, title, description, draft_revision, settings_json,
    draft_updated_at, created_at, updated_at
) VALUES (
    'page_system_root',
    'system',
    NULL,
    'nav.ax',
    '',
    0,
    '{"layout":{"template":"full","density":"comfortable","columns":4,"categoryStyle":"tabs"},"appearance":{"themeId":"slate","background":{"type":"none","value":"","opacity":1}},"search":{"defaultEngine":"google","showEngineSelector":true},"display":{"showClock":true,"showDate":true,"showGreeting":true},"preferences":{"locale":"zh-CN","timezone":"Asia/Shanghai","openLinksInNewTab":true}}',
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
);

INSERT INTO page_publications (page_id, visibility, slug, show_author, updated_at)
VALUES ('page_system_root', 'private', 'home', 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

INSERT INTO categories (id, page_id, name, icon, sort_order, is_uncategorized, created_at, updated_at)
VALUES (
    'category_system_uncategorized',
    'page_system_root',
    '未分类',
    '',
    0,
    1,
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
);

INSERT INTO system_settings (id, updated_at)
VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

INSERT INTO provider_configs (kind) VALUES ('smtp'), ('storage'), ('dns');

INSERT INTO update_state (id, updated_at)
VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
