# 目录审核(子项目 B2c)设计

日期:2026-07-25
状态:方向已与用户确认,spec 待审阅
范围:私有主题 → 提交官方目录 → 管理员审批 → scope 从 `private` 晋升为 `catalog` 的申请-审批流,照搬 `internal/subdomains` 的申请-审批范式。顺带两个路线图里点名的小项:①导入 ref 落库(修复更新检查恒对比默认分支 HEAD 的假阳性)②`openapi.yaml` 给 `PATCH /admin/theme-versions/{versionId}` 补 422。三者在同一分支/同一轮 SDD 里交付,但迁移文件互相独立、互不依赖。
设计依据:`docs/superpowers/specs/2026-07-23-theme-spec-v1-design.md`(§3、§7.1、§7.2、§8.1)、`docs/superpowers/specs/2026-07-24-theme-import-b1-design.md`、`docs/superpowers/specs/2026-07-25-theme-governance-b2a-design.md`、`docs/superpowers/specs/2026-07-25-theme-maintenance-b2b-design.md`(§5 行:"目录审核...推到 B2c")、`internal/subdomains`(申请-审批范式的直接参照)

## 1. 背景与原则

主题机制的 scope 只有两个值(`migrations/0014_theme_packages.sql`):`catalog`(全站可见,`owner_id IS NULL`)与 `private`(仅 owner 可见,`owner_id` 非空),两者由触发器 `themes_scope_owner_insert`/`_update` 强制联动,任何 UPDATE 必须同一条语句里把 `scope` 和 `owner_id` 一起改对,否则回滚。晋升的本质就是一条满足这个约束的 UPDATE——但谁能改、什么时候能改、改之前要校验什么,需要一套申请-审批流来治理,而不是直接开一个管理员可任意调用的 PATCH。

**原则**:
- 完整复用 `internal/subdomains` 已验证过的模式:独立请求表、`pending/approved/rejected/revoked` 状态机、CAS(`UPDATE ... WHERE id=? AND status=?` + `RowsAffected` 判定)代替显式版本号做乐观并发、审计沿用 `audit_logs`。
- 新包 `internal/themecatalog` 与已有的 `internal/catalog`(站点导航目录/发现页)是两个不相关的领域,包名故意都含"catalog"是因为都在复用 `themes.scope='catalog'` 这个既有词汇,设计与实现中需注意不要互相引用。
- 不引入"退回私有"(`approved → revoked`)——已确认下架用现有的 kill switch(`enabled` 开关 + B2b 版本级 status)即可覆盖,scope 晋升是单向操作,状态机更简单。
- catalog slug 全站唯一、private slug 按 owner 唯一(`idx_themes_catalog_slug`/`idx_themes_private_slug`),这是晋升流程里唯一真正棘手的冲突面,贯穿提交、审批两个时间点分别校验。
- **运营风险(遗留)**:catalog slug 是全站永久(晋升无回退)、由用户提交内容决定的命名空间,且没有保留字校验。未来新增内置主题前,必须先确认其目标 slug 尚未被某个用户晋升的主题占用,否则启动期的内置主题同步迁移会撞上 `idx_themes_catalog_slug` 唯一索引而失败。

## 2. 数据模型与状态机

新迁移 `migrations/0016_theme_catalog_requests.sql`:

```sql
CREATE TABLE theme_catalog_requests (
    id TEXT PRIMARY KEY,
    theme_id TEXT NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','revoked')),
    reason TEXT NOT NULL DEFAULT '',
    version_id TEXT NOT NULL REFERENCES theme_versions(id),
    reviewer_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    applied_at TEXT NOT NULL,
    reviewed_at TEXT
);
CREATE UNIQUE INDEX idx_theme_catalog_active ON theme_catalog_requests(theme_id) WHERE status = 'pending';
CREATE INDEX idx_theme_catalog_status_time ON theme_catalog_requests(status, applied_at DESC);
```

- `owner_id` 是提交时的申请人快照——晋升成功后 `themes.owner_id` 会被置 NULL,这一列是"谁申请的"这段历史唯一的留存位置,审计与前端展示都靠它。
- `version_id` 是提交时 `themes.current_version_id` 的快照,审批时用来确认版本没有在审核期间被换掉(见下方"锁定升级")。
- `revoked` 状态只用于 owner 自撤 pending 请求(`Cancel`),不用于已批准请求的回退。

**状态机**(`internal/themecatalog/service.go`,结构对齐 `internal/subdomains.Service`):

```go
type Actor struct{ ID, Username, Role, Status string }

func (s *Service) Request(ctx context.Context, actor Actor, themeID string) (Request, error)
func (s *Service) Cancel(ctx context.Context, actor Actor, themeID string) error
func (s *Service) Review(ctx context.Context, actor Actor, requestID, decision, reason string) (Request, error)
func (s *Service) List(ctx context.Context, actor Actor, status string, page, pageSize int) (Page[Request], error)
```

- **`Request`**(owner,`authorize` 要求 `actor.ID == 该主题 owner_id`):校验①主题存在、`scope='private'`、`enabled=1`、有 `current_version_id`;②`slug` 未与任何 `scope='catalog'` 主题冲突,冲突→ `ErrSlugConflict`(422);③无该主题的其它 pending 请求(唯一索引兜底,冲突→ `ErrConflict` 409)。写入时把 `current_version_id` 存进 `version_id`。
- **`Cancel`**(owner 自撤,对齐 `subdomains.CancelPending`):`UPDATE theme_catalog_requests SET status='revoked', reviewed_at=? WHERE theme_id=? AND owner_id=? AND status='pending'`,`RowsAffected==0` → `ErrNotFound`。
- **`Review`**(admin,`authorize` 要求 `actor.Role=='admin'`):CAS `UPDATE theme_catalog_requests SET status=?, reason=?, reviewer_id=?, reviewed_at=? WHERE id=? AND status='pending'`,`RowsAffected==0` → `ErrInvalidTransition`(409,已被处理或不存在)。
  - `decision='approve'`:同一事务内先重新校验①该请求的 `version_id` 仍等于 `themes.current_version_id`(理论上因"锁定升级"不会不一致,防御性保留,不一致 → `ErrVersionChanged` 409)②`slug` 仍无 catalog 冲突(审核期间被别的主题抢注 → `ErrSlugConflict` 409,需要管理员拒绝或 owner 撤回重提,不做自动改名)。两项都过,执行 `UPDATE themes SET scope='catalog', owner_id=NULL WHERE id=?`(满足触发器要求的单条语句同时改两列)。
  - `decision='reject'`:只落 `reason`,不碰 `themes` 表。后端不强制 `reason` 非空(对齐 `subdomains.Review` 只做 300 字上限校验,不禁止空值),「拒绝必填理由」是前端 UX 约束,不是 API 契约。
  - 每次决议在同一事务写 `audit_logs`(action `theme_catalog.apply`/`theme_catalog.approve`/`theme_catalog.reject`/`theme_catalog.revoke`,target_type `theme_catalog_request`),对齐 subdomains 惯例。
- **`List`**:管理员审核队列,支持按 `status` 过滤 + 分页,对齐 `GET /admin/subdomains` 的实现形状。

**锁定升级**(防止审核期间内容被偷换):`internal/themes/install.go` 的 `InstallPrivate` 升级分支(已存在 `themeID` 的路径)新增一次查询:
```sql
SELECT 1 FROM theme_catalog_requests WHERE theme_id = ? AND status = 'pending'
```
命中则返回新的 `ErrCatalogRequestPending`,`internal/themeimport.Service.install` 透传;httpapi 映射为 409。效果:只要有 pending 请求,重新导入(= 升级)GitHub/zip 来源都会被拦住,直到请求被撤回或审批完成——这保证管理员审核的内容就是最终晋升的内容。`internal/themes` 包直接查询 `theme_catalog_requests` 表(而不是反向依赖 `internal/themecatalog` 包)是刻意选择:这是模块化单体 + 单一 SQLite 库的既有惯例(`internal/admin` 也直接读写 `themes`/`theme_versions` 表,不经 `internal/themes.Store` 中转),避免引入新的包依赖方向。

## 3. HTTP API 与契约

**路由**(`internal/httpapi/themecatalog.go`,新文件):

| 方法 | 路径 | 角色 | 说明 |
|---|---|---|---|
| POST | `/api/v1/me/themes/{themeId}/catalog-request` | owner | 提交,201,body 为空 |
| DELETE | `/api/v1/me/themes/{themeId}/catalog-request` | owner | 撤回 pending,204 |
| GET | `/api/v1/admin/theme-catalog-requests?status=` | admin | 分页列表 |
| PATCH | `/api/v1/admin/theme-catalog-requests/{requestId}` | admin | body `{decision: 'approve'\|'reject', reason?}` |

不新增"查询单个请求状态"的 GET——`Theme` schema 直接加两个可空字段:`catalogRequestStatus?: 'pending'|'rejected'`、`catalogRequestReason?: string`(已批准的请求体现为 `scope:'catalog'`,不需要状态残留)。`internal/catalog.Service.Themes`(owner/公开读取共用的查询,`internal/catalog/service.go:114`)与 `internal/admin` 的 `GET /admin/themes` 各自的 SQL 都加一段 `LEFT JOIN theme_catalog_requests` 取最新一条 `status IN ('pending','rejected')` 的记录;owner 侧查询按 `owner_id = actorID` 限定(与既有 eligibility 谓词的匿名安全性一致,`actorID` 为空时天然不匹配),admin 侧不限定,能看到所有人的请求状态。

**错误映射**(`writeError` 新增 case):

| 场景 | HTTP | code |
|---|---|---|
| slug 与现有 catalog 冲突(提交或审批时) | 422 | VALIDATION_FAILED |
| 已有 pending 请求(重复提交/审批期间升级) | 409 | CONFLICT |
| 请求/主题不存在或非本人 | 404 | NOT_FOUND |
| 非 pending 状态被 review/cancel(竞态) | 409 | CONFLICT |
| 审批时 version 与快照不一致(防御性) | 409 | CONFLICT |

**openapi.yaml**:新增上述 4 个 path;新 schema `ThemeCatalogRequest`(id/themeId/ownerId/ownerName/status/reason/appliedAt/reviewedAt)与 `ThemeCatalogRequestStatus` enum(`pending/approved/rejected/revoked`);`Theme` schema 加 `catalogRequestStatus`/`catalogRequestReason` 两个可空字段。`tests/contract/` 加一段 pending→approve 全链路用例(提交→管理员列表可见→通过→`GET /themes` 公开可见,仿现有 subdomain 那段)。

## 4. 前端改动

**Owner 端**(`web/src/pages/app/themes/page.tsx`,私有主题卡片区域):
- 卡片新增「提交官方目录」按钮,仅当 `enabled && status==='active' && !catalogRequestStatus` 时可点;点击后乐观更新为 pending 态。
- pending 态:按钮替换为「审核中」徽标 +「撤回」链接;同时该卡片的「检查更新」/重新导入入口禁用并提示"审核期间不可升级"(直接消费后端 409 语义,不额外查询)。
- rejected 态:「已拒绝」徽标 + 展示 `catalogRequestReason` + 「重新提交」按钮。

**Admin 端**(新 section,结构参照 `web/src/pages/admin/operations/components/SubdomainsSection.tsx` 的审核队列 UI,放进 `web/src/pages/admin/themes/`):
- pending 队列:主题名/owner/slug/提交时间,可展开预览(复用既有主题预览能力)。
- 通过/拒绝两个操作,`reason` 均为可选审核说明(对齐 `SubdomainsSection.tsx` 既有交互,不强制填写)。

**API 层**:`web/src/api/themes.ts` 加 `submitCatalogRequest`/`cancelCatalogRequest`;新增 admin 侧 list/review 调用(并入 `web/src/api/admin.ts` 或独立文件均可,实现阶段定)。`web/src/api/mock-handlers.ts` 同步实现,过 `make test-mock`。

## 5. 顺带小项①:导入 ref 落库

**根因**:`internal/themeimport/service.go:121` 的 `CheckUpdate` 恒调用 `ResolveHeadSHA(ctx, sourceURL, "")`——空 ref 语义上代表"默认分支"。但 `ImportGitHub(ctx, ownerID, repoURL, ref)` 里用户传入的 `ref`(可能是 tag 或非默认分支)从未持久化,只有解析后的 commit sha 落进了 `theme_versions.source_ref`。结果:凡是从非默认分支/tag 导入的主题,更新检查永远拿默认分支 HEAD 去跟这个 tag/分支当时的 sha 比较,恒为"有更新"的假阳性。

**修复**(独立迁移 `migrations/0015_theme_source_ref.sql`,与 B2c 主功能解耦,便于单独 review/回滚):
- `themes` 加列 `source_git_ref TEXT NOT NULL DEFAULT ''`(空串=默认分支,与 `ResolveHeadSHA` 既有约定一致;放在 `themes` 而非 `theme_versions`,因为这是"该私有主题跟踪哪个 ref"的属性,与 `source_type`/`source_url` 同级)。
- `internal/themes.Store.InstallPrivate` 新增 `sourceGitRef` 参数,INSERT 与升级 UPDATE 分支都写这一列(跟随 `source_url` 的既有刷新时机)。
- `internal/themeimport.Service.ImportGitHub` 把入参 `ref` 一路透传到 `install`;`ImportZip` 传空串(upload 来源无 ref 概念)。
- `internal/themes.Store.PrivateThemeSource` 多返回一个 `gitRef` 字段;`CheckUpdate` 改用它调用 `ResolveHeadSHA(ctx, sourceURL, gitRef)`,不再硬编码空串。
- 不改 API 契约(`ref` 本来就是 `POST /me/themes/import` 的入参,只是现在真正被记住);老数据 `source_git_ref` 默认空串,等价于此前的实际行为(恒当默认分支处理),不需要回填脚本。

## 6. 顺带小项②:openapi 422 补全

**根因**:`internal/admin/service.go:459-465` 的 `SetThemeVersionStatus`,`status` 不在 `active`/`disabled` 之内时返回 `ErrInvalidInput`,经 `writeError` 映射为 422 `VALIDATION_FAILED`——这条路径生产代码里真实可达,但 `api/openapi.yaml` 的 `/api/v1/admin/theme-versions/{versionId}` PATCH 只声明了 `200/401/403/404/409`,缺 422。

**修复**:openapi 给该 path 加 `'422': { $ref: '#/components/responses/ErrorResponse' }`;`tests/contract/` 补一个用例(PATCH 传非法 `status` 值,断言 422 且响应体符合 `ErrorResponse` schema)。纯文档 + 测试补全,不改后端代码。

## 7. 测试策略

- **Go 单测**(表驱动,放各包 `*_test.go`):`internal/themecatalog` 状态机(pending/approve/reject/revoke 合法与非法转移、CAS 竞态、slug 冲突时机分提交/审批两处、version_id 快照校验);`internal/themes` 升级路径在 pending 时被拦截(`ErrCatalogRequestPending`);`internal/themeimport.CheckUpdate` 用非默认分支/tag 场景断言不再假阳性(§5)。
- **SQLite 集成测试**:晋升 UPDATE 下触发器(`scope`/`owner_id` 联动)行为;`idx_theme_catalog_active`/`idx_themes_catalog_slug` 冲突路径。
- **`tests/contract/`**:目录审核 pending→approve 全链路(§3);422 补全用例(§6)。
- **E2E**(`tests/e2e/`,admin/user spec):走一遍提交→审核队列→通过的 UI 路径,断言主题出现在公开目录里;审核期间升级按钮被禁用的提示。
- 回归门槛:`make check`、`go test -race ./...`、`make build`、`make test-contract`、`make test-mock`、`make e2e`;UI 六态冒烟(加载/空/错误/移动端/键盘/深色主题)。

## 8. 交付与不在范围

- 分支 `feat/theme-catalog-review-b2c`(自最新 main),单 PR,含 §2-§7 全部内容(核心流程 + 两个顺带小项,迁移文件互相独立但同分支交付)。
- **不在范围**:`approved → revoked`(退回私有,已确认用现有 kill switch/enabled 覆盖);starter 官方仓库;后台定时更新轮询;账号删除前置清理(仍卡在删用户功能不存在);非 GitHub 布局的更新检查;子项目 C(tier 2 声明式布局)。
- 按 CLAUDE.md shipping path:PR → CI 绿 → auto-merge → 生产 CD。
