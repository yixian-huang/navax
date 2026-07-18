-- Personal/system navigation: draft-level enable/hide for categories and sites.
-- Disabled items remain in draft tables (and count toward quotas) but are
-- omitted from published snapshots and previews.

ALTER TABLE categories ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1));
ALTER TABLE sites ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1));
