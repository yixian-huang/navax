-- Tech-feeling theme packages (frontend CSS lives in web/src/themes/packages).
-- Also seed slate-dark which was registered in the SPA but missing from catalog.

INSERT INTO themes (id, name, version, author, description, mode, preview, enabled, is_default, created_at, updated_at)
VALUES
    ('slate-dark', 'Slate Dark', '1.0.0', 'nav.ax', '墨色底，纸白文字，苔绿点缀。与 Slate 同一套直角与实线描边语言的暗色版。', 'dark', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('cyber', 'Cyber', '1.0.0', 'nav.ax', '近黑底上的电青与品红。细霓虹描边与扫描线，像未来都市的 HUD 面板。', 'dark', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('orbit', 'Orbit', '1.0.0', 'nav.ax', '靛紫虚空与电蓝轨迹。像深空任务控制台：冷静、精密、略带科幻。', 'dark', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('terminal', 'Terminal', '1.0.0', 'nav.ax', 'CRT 黑底与磷光绿。等宽字感与锐利直角，像深夜里亮着的系统控制台。', 'dark', '', 1, 0, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
ON CONFLICT(id) DO NOTHING;
