-- Background media library: instance presets (≤12) and per-user library (≤3).

CREATE TABLE background_media (
    id TEXT PRIMARY KEY CHECK (length(id) BETWEEN 8 AND 64),
    scope TEXT NOT NULL CHECK (scope IN ('instance', 'user')),
    owner_user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    media_kind TEXT NOT NULL CHECK (media_kind IN ('image', 'video')),
    mime_type TEXT NOT NULL CHECK (length(mime_type) BETWEEN 1 AND 100),
    url TEXT NOT NULL CHECK (length(url) BETWEEN 1 AND 2048),
    poster_url TEXT CHECK (poster_url IS NULL OR length(poster_url) BETWEEN 1 AND 2048),
    width INTEGER NOT NULL DEFAULT 0 CHECK (width >= 0),
    height INTEGER NOT NULL DEFAULT 0 CHECK (height >= 0),
    duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    size_bytes INTEGER NOT NULL CHECK (size_bytes > 0),
    sort_order INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    CHECK (
        (scope = 'instance' AND owner_user_id IS NULL)
        OR (scope = 'user' AND owner_user_id IS NOT NULL)
    )
);

CREATE INDEX idx_background_media_instance
    ON background_media(scope, enabled DESC, sort_order, id)
    WHERE scope = 'instance';

CREATE INDEX idx_background_media_user
    ON background_media(owner_user_id, created_at DESC)
    WHERE scope = 'user';
