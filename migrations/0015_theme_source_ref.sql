-- Persist the exact git ref (branch/tag name; empty string = default branch)
-- used at import time, separate from theme_versions.source_ref (the resolved
-- commit sha). Without this, CheckUpdate has no way to know which ref to
-- re-resolve and always compares against the default branch HEAD, producing
-- a false "has update" for every theme imported from a non-default branch
-- or a tag.
ALTER TABLE themes ADD COLUMN source_git_ref TEXT NOT NULL DEFAULT '';
