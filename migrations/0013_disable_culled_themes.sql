-- Disable culled first-party themes so admin catalog matches SPA packages.
-- Existing page settings may still reference old themeIds; SPA resolveThemeId maps them.

UPDATE themes
SET enabled = 0,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id IN ('kyoto', 'terracotta', 'mochi', 'pastelsky', 'mono', 'cyber');
