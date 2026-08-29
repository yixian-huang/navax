# nav.ax 主题 starter

这份目录是一份能通过服务端校验器的最小主题包。复制它，改 `theme.json` 的 `id` / `name` / 令牌，再改 `theme.css`。

作者契约：[docs/theme-api.md](../../docs/theme-api.md)。

## 导入

本仓库根目录是应用，不是主题包，**不能**把 `yixian-huang/navax` 当作 GitHub 导入源。

- **zip**：把本目录打成 zip（`theme.json` 在包根或一层文件夹下均可），登录后到「主题设置 → 导入主题 → 上传 zip」。选文件后会先 dry-run 校验。
- **GitHub**：把本目录内容放到**你自己的仓库根目录**（根上要有 `theme.json`），再在导入对话框填仓库 URL。

```bash
cd examples/theme-starter
zip -r ../starter.zip theme.json theme.css README.md
```

本地校验（需已登录的 cookie）：

```bash
curl -sS -b cookie.txt -F file=@../starter.zip http://localhost:8080/api/v1/themes/validate
```

## 必改项

- `theme.json` 的 `id` 必须符合 `^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$`，且提交官方目录时不能用保留名（内置主题、`default` / `official` / `navax` / `starter` 等）。示例 id `starter` **只能私有安装**，不能晋升目录。
- `tier` 保持 `1`。宿主暂不接受声明式布局（tier 2）或 JS（tier 3）。
- 令牌至少包含 `font.heading/body/label/mono` 与 `color.background/foreground/primary/accent`。缺失的 radius / elevation 会回落基线。

## 许可

这些示例文件随 nav.ax 以 AGPL-3.0-only 分发。你自己的主题请在 `theme.json` 的 `license` 里写明你的许可证。
