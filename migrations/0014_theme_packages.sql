-- Theme packages: turn `themes` into a package table and add immutable,
-- content-addressed versions plus their assets.
--
-- Two invariants cannot be expressed as constraints on an existing SQLite
-- table (no ADD CONSTRAINT, no retrofitted CHECK or FOREIGN KEY), so they are
-- enforced by triggers below: the scope/owner_id pairing and the validity of
-- themes.current_version_id.

ALTER TABLE themes ADD COLUMN slug TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN scope TEXT NOT NULL DEFAULT 'catalog'
    CHECK (scope IN ('catalog', 'private'));
ALTER TABLE themes ADD COLUMN owner_id TEXT REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE themes ADD COLUMN source_type TEXT NOT NULL DEFAULT 'builtin'
    CHECK (source_type IN ('builtin', 'github', 'upload'));
ALTER TABLE themes ADD COLUMN source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN current_version_id TEXT;
ALTER TABLE themes ADD COLUMN spec_version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE theme_versions (
    id TEXT PRIMARY KEY,
    -- RESTRICT rather than CASCADE: published snapshots reference the compiled
    -- artefact by version id, so deleting a package must never silently strip
    -- the styling off a live public page. Deletion paths have to clear the
    -- references first.
    theme_id TEXT NOT NULL REFERENCES themes(id) ON DELETE RESTRICT,
    version TEXT NOT NULL,
    source_ref TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL,
    compiled_css BLOB NOT NULL,
    content_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    imported_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    UNIQUE (theme_id, content_hash)
);

CREATE TABLE theme_assets (
    id TEXT PRIMARY KEY,
    theme_version_id TEXT NOT NULL REFERENCES theme_versions(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    mime TEXT NOT NULL,
    bytes INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    data BLOB NOT NULL,
    UNIQUE (theme_version_id, path)
);

-- The snapshot -> version reference must be a real, queryable foreign key
-- instead of living only inside payload_json: otherwise DELETE FROM
-- theme_versions can pull the stylesheet out from under a published page, and
-- the RESTRICT on themes only guards package deletion.
-- Nullable: NULL means a snapshot published before this migration, which falls
-- back to the default theme at read time.
ALTER TABLE published_snapshots ADD COLUMN theme_version_id TEXT
    REFERENCES theme_versions(id) ON DELETE RESTRICT;

CREATE INDEX idx_published_snapshots_theme_version
    ON published_snapshots(theme_version_id) WHERE theme_version_id IS NOT NULL;

-- Backfill slugs for existing rows (including the themes 0013 disabled) before
-- creating the unique indexes; otherwise every empty-string slug collides.
UPDATE themes SET slug = id WHERE slug = '';

CREATE UNIQUE INDEX idx_themes_catalog_slug ON themes(slug) WHERE scope = 'catalog';
CREATE UNIQUE INDEX idx_themes_private_slug ON themes(owner_id, slug) WHERE scope = 'private';

-- scope and owner_id must agree. Without this, a private theme with a NULL
-- owner_id would slip past idx_themes_private_slug entirely, because NULLs
-- never compare equal inside a unique index.
CREATE TRIGGER themes_scope_owner_insert BEFORE INSERT ON themes
BEGIN
    SELECT RAISE(ABORT, 'catalog theme must have null owner_id; private theme must have owner_id')
    WHERE NOT ((NEW.scope = 'catalog' AND NEW.owner_id IS NULL)
            OR (NEW.scope = 'private' AND NEW.owner_id IS NOT NULL));
END;

CREATE TRIGGER themes_scope_owner_update BEFORE UPDATE ON themes
BEGIN
    SELECT RAISE(ABORT, 'catalog theme must have null owner_id; private theme must have owner_id')
    WHERE NOT ((NEW.scope = 'catalog' AND NEW.owner_id IS NULL)
            OR (NEW.scope = 'private' AND NEW.owner_id IS NOT NULL));
END;

-- current_version_id cannot be a real foreign key on an existing table, and
-- application-level checks do not cover writes that bypass the service layer.
-- Make it a database-level invariant instead: the pointer must exist, belong to
-- this very theme, and be active.
CREATE TRIGGER themes_current_version_valid BEFORE UPDATE OF current_version_id ON themes
WHEN NEW.current_version_id IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'current_version_id must reference an active version of this theme')
    WHERE NOT EXISTS (
        SELECT 1 FROM theme_versions
        WHERE id = NEW.current_version_id AND theme_id = NEW.id AND status = 'active'
    );
END;

-- The UPDATE trigger alone leaves a hole: a fresh themes row can be INSERTed
-- with a pointer to a nonexistent version, or to another theme's version, and
-- never pass through UPDATE at all. Verified by probe before adding this.
-- Bypassing the service layer is exactly what these triggers exist to catch,
-- so both write paths need the same guard.
CREATE TRIGGER themes_current_version_valid_insert BEFORE INSERT ON themes
WHEN NEW.current_version_id IS NOT NULL
BEGIN
    SELECT RAISE(ABORT, 'current_version_id must reference an active version of this theme')
    WHERE NOT EXISTS (
        SELECT 1 FROM theme_versions
        WHERE id = NEW.current_version_id AND theme_id = NEW.id AND status = 'active'
    );
END;

-- The published_snapshots foreign key only protects versions that have already
-- been published; a theme's current version may not be referenced by any
-- snapshot yet. Guard that window explicitly.
CREATE TRIGGER theme_versions_current_guard BEFORE DELETE ON theme_versions
BEGIN
    SELECT RAISE(ABORT, 'cannot delete a theme current version')
    WHERE EXISTS (SELECT 1 FROM themes WHERE current_version_id = OLD.id);
END;
