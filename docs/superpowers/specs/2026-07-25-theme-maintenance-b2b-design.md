# 主题维护片(子项目 B2b)设计

日期:2026-07-25
状态:方向已与用户确认,spec 待审阅
范围:B1/B2a 落地后主题分发治理的维护类三件事——历史版本视图 + 版本级 kill switch、GitHub 更新检查(手动)、墓碑回收。目录审核(私有→catalog 晋升申请-审批流)与 starter 仓库推到 B2c;不在本文。
设计依据:`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`(§8.1.1、§10.B)、`docs/superpowers/specs/2026-07-24-theme-import-b1-design.md`(§4、§10)、`docs/superpowers/specs/2026-07-25-theme-governance-b2a-design.md`(§3)

## 1. 背景与原则

B2a 交付了作用于**当前版本**的 kill switch、owner 溯源、admin scope 分组。B2b 补齐三块维护能力,读路径/判定 SQL 都已就绪:

1. **版本级 kill switch 只到当前版本**:B2a 的 `PATCH /admin/themes/{id}` status 只能停用 `current_version_id`;一个被旧发布快照锁定、仍在供 CSS 的**非当前坏版本**没有管理入口。
2. **无更新检查**:github 来源主题存了 `source_url` 与当前 `source_ref`(sha),但没有比对 upstream 的入口;用户不知道自己装的主题有没有新版。
3. **墓碑永久占配额**:私有主题卸载时若被快照引用转墓碑(enabled=0),占配额直到快照不再引用,但没有回收路径。

**原则**:不新建子系统、不新增迁移;`internal/httpapi` 只做路由/DTO/序列化;沿用既有审计、限流、错误码惯例;更新检查复用 B1 的 netguard/GitHubClient。

## 2. 历史版本视图 + 版本级 kill switch

### 2.1 端点

- `GET /api/v1/admin/themes/{themeId}/versions` —— 列出该主题全部版本(不止当前),每行:`versionId`、`version`、`sourceRef`、`status`(active/disabled)、`createdAt`、`importedBy`(用户名,内置为空)、`isCurrent`(== themes.current_version_id)、`snapshotRefs`(引用该版本的 `published_snapshots` 计数)。管理员据此看清哪个旧版本还在供公开页。
- `PATCH /api/v1/admin/theme-versions/{versionId}` —— body `{status: active|disabled}`,精确停用/启用任意版本。

两者都在 `RequireAdmin` 路由组内。

### 2.2 守卫与语义

- **唯一守卫**:拒绝停用「`is_default=1` 主题的当前版本」——该版本 == 某默认主题的 `current_version_id` 时,`status=disabled` 返回 409 `DEFAULT_THEME_VERSION`。这与 B2a 的守卫、`AssertDefaultThemeUsable` 不变量一致。
- 停用**非当前**版本一律放行:锁定该版本的已发布快照公开页拿到 410(B1 既有读路径)→ `PublicShell` 回落基线令牌;不承诺即时全局撤销(§8.1.1)。
- 停用**非默认主题的当前版本**(与 B2a 现有能力等价)也放行——版本级端点是当前版本入口的超集。
- 未知 versionId → 404;幂等(重复停用不报错);审计沿用 `AuditRecord`(action `theme.version.disable`/`theme.version.enable`,target_type `theme_version`)。
- **boot-loop 免疫已就绪**:B2a 的 C1 修复让 `SyncBuiltin` 容忍被停用的内置**当前**版本;停用非当前内置版本时 `SyncBuiltin` 因内容哈希不同根本不会重新 upsert 它,天然安全。B2b 不需要再动 `SyncBuiltin`。

### 2.3 实现落点

- 新 store 方法:`ListThemeVersions(ctx, themeID) ([]ThemeVersion, error)`(LEFT JOIN users 取 importer 名 + 子查询算 snapshotRefs + 与 themes.current_version_id 比对 isCurrent);`SetVersionStatus(ctx, versionID, status, now, audit) error`(读该版本所属主题的 is_default 与其 current_version_id 判守卫,再 `UPDATE theme_versions SET status`)。放 `internal/admin`(治理属管理端)。
- httpapi:`GET .../versions`、`PATCH /theme-versions/{versionId}` 两个 handler + 错误映射(复用 B2a 的 `DEFAULT_THEME_VERSION` 409、`NOT_FOUND` 404)。
- **前端**:admin 主题卡增加「查看版本」展开,列出版本(sha 短号、状态、快照引用数、当前标记),每个非当前版本带停用/启用按钮;当前版本沿用卡片既有 B2a 停用控件(避免重复)。

## 3. GitHub 更新检查(手动,owner,仅 github 来源)

### 3.1 导出解析

`internal/themeimport/github.go`:现有 `resolveSHA`/`parseRepoURL` 未导出且 `resolveSHA` 只被 `FetchTarball` 调用(它会解析**并下载** tarball,对"只查有无新版"太重)。新增导出方法:

```
func (c *GitHubClient) ResolveHeadSHA(ctx context.Context, rawURL, ref string) (string, error)
```

内部 `parseRepoURL` + `resolveSHA`(github.com 走 api.github.com/commits;缺省 ref 用默认分支 HEAD),**不下载 tarball**。netguard 全防护、主机白名单、token 作用域(仅官方主机)全部复用 B1 既有实现,不放松。

### 3.2 端点与语义

- `POST /api/v1/me/themes/{themeId}/check-update` —— **owner 检查自己的私有主题**(themeID 必须 `scope=private AND owner_id=当前用户`,否则 404 不区分)。
- 返回 `{sourceType, hasUpdate, currentSha, latestSha}`:
  - `source_type='github'`:`ResolveHeadSHA(source_url, "")` 得 upstream HEAD sha,与当前版本 `source_ref` 比对;不等 → `hasUpdate:true`。
  - `source_type='upload'`:`{sourceType:'upload', hasUpdate:false}`(上传主题无 upstream,不报错)。
- **只检查、不升级**:升级仍走 B1 既有「重新拉取」(同 slug 重导入 `POST /me/themes/import` github 分支)。
- 上游不可达 → 502 `UPSTREAM_ERROR`(复用 B1 `ErrUpstream` 映射);仓库地址不合法 → 422。
- 限流:`AbuseProtection` 规则表加 `POST /api/v1/me/themes/{id}/check-update` 20 次/小时/IP(GitHub API 限额;与 import 10/h、validate 20/h 并列)。因路径含变量,用 `strings.HasPrefix("/api/v1/me/themes/") && strings.HasSuffix("/check-update")` 匹配(照 rate_limit.go 既有前后缀匹配惯例)。

### 3.3 实现落点

- `internal/themeimport/service.go`:`CheckUpdate(ctx, ownerID, themeID) (UpdateStatus, error)`——查该 owner 的私有主题(source_type/source_url + 当前版本 source_ref),github 则 `ResolveHeadSHA` 比对。
- httpapi:`POST /me/themes/{themeId}/check-update` handler(挂在受保护组,复用 themeimport handler)。
- **前端**:`/app/themes` 的「我的主题」卡片,`sourceType==='github'` 时加「检查更新」按钮 → 调端点 → `hasUpdate` 则提示「有新版本」并高亮既有「重新拉取」入口;无新版提示「已是最新」。

## 4. 墓碑回收(配额压力时自动)

- 墓碑 = `scope=private AND enabled=0` 的私有主题(B1 `UninstallPrivate` 对有快照引用的卸载产生)。它对 owner 不可见(eligibility 排除 enabled=0)却占配额。
- **回收时机**:`InstallPrivate` 在**因配额满即将拒绝之前**,先扫该 owner 的墓碑,对**其全部版本都无任何 `published_snapshots` 引用**的墓碑执行物理删除(复用 B1 `UninstallPrivate` 的引用计数 SQL `SELECT COUNT(*) FROM published_snapshots WHERE theme_version_id IN (SELECT id FROM theme_versions WHERE theme_id=?)` 与物理删分支:置空 current 指针 → 删版本 → 删行),再重新计配额。仍被引用的墓碑保留。
- 不加新端点、不加新 UI:回收在恰好需要时(配额压力)自动发生,owner 无感。
- **实现落点**:`internal/themes/install.go` 的 `InstallPrivate` 配额检查分支——`owned >= quota` 时先调新私有函数 `reclaimTombstones(ctx, tx, ownerID, now) (int, error)`(事务内,扫 + 物理删无引用墓碑),再重算 `COUNT(*)`;仍 `>= quota` 才返回 `ErrQuotaExceeded`。`reclaimTombstones` 与 `UninstallPrivate` 共享物理删逻辑(抽一个 `deleteThemeTx(ctx, tx, themeID, now)` 供两者复用,避免重复)。
- **测试**:造两个墓碑(一个有快照引用、一个无)+ 用满配额 → 再 InstallPrivate → 无引用墓碑被回收、配额腾出、安装成功;有引用墓碑保留;回收计数正确。

## 5. 契约

- `api/openapi.yaml`:
  - `GET /api/v1/admin/themes/{themeId}/versions`(200 `ThemeVersionsResponse`);`PATCH /api/v1/admin/theme-versions/{versionId}`(body status enum,200 `ThemeVersionResponse`,404/409);`POST /api/v1/me/themes/{themeId}/check-update`(200 `ThemeUpdateStatusResponse`,404/422/502)。
  - 新 schema:`ThemeVersion`(versionId/version/sourceRef/status/createdAt/importedBy/isCurrent/snapshotRefs)、`ThemeUpdateStatus`(sourceType/hasUpdate/currentSha/latestSha)。
  - 新错误码若有(本片沿用既有 `DEFAULT_THEME_VERSION`/`NOT_FOUND`/`UPSTREAM_ERROR`/`VALIDATION_FAILED`,预计无新增;若加须同步 `ErrorEnvelope.code` 枚举)。
- `tests/contract/`:版本列表 200、版本级 patch active↔disabled 200。停用版本后的 CSS 端点 410 **留 httpapi 单测**(与 B1/followups 一致,410 分支不进契约)。check-update:契约测试用真实二进制,对真实 github 会打网络,因此契约只覆盖 **upload 来源主题**断言 `hasUpdate:false`(无网络分支);github 分支的网络交互由 themeimport 单测用假 client 覆盖,不进契约。
- mock + `mock-contract.test.ts`:三个新端点。

## 6. 测试策略

- **Go 单测**:§2.3、§3.3、§4 列出的 store/service 断言;版本级 kill switch 的守卫与回落复用 eligibility/themes 测试范式;更新检查用 B1 的 `fakeResolver`/`roundTripFunc` 假 client 覆盖 has-update/no-update/upstream-error/upload-skip;墓碑回收覆盖有引用保留 + 无引用回收 + 配额腾出。
- **契约**:§5。
- **E2E**(`admin.spec.ts` + `user.spec.ts`,照既有范式):admin 展开版本列表并停用一个历史版本;owner 对私有主题点「检查更新」(mock 返回 hasUpdate)。E2E 走真实二进制,check-update 对真实 github 会打网络——E2E 只断 upload 主题或用无 upstream 分支,github 网络交互不进 E2E。
- 门槛:`make check`、`go test -race ./...`、`make build`、`make test-contract`、`make test-mock`、`make e2e`;UI 六态冒烟。

## 7. 交付与不在范围

- 分支 `feat/theme-maintenance-b2b`(自最新 main,含 B2a),单 PR。
- **不在范围(B2c / 独立)**:目录审核(私有→catalog 晋升申请-审批流)、starter 仓库、后台定时更新轮询、账号删除前置清理(仍卡在删用户功能不存在)、非 GitHub 布局的更新检查。
- 按 CLAUDE.md shipping path:PR → CI 绿 → auto-merge → 生产 CD(已恢复正常)。
