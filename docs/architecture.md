# nav.ax v1 技术架构

状态：`accepted`
日期：2026-07-16

## 1. 结论

后端采用 **Go 1.25 模块化单体**，以一个进程提供 REST API、公开页、静态前端、后台任务和运维端点。数据层使用 SQLite WAL，前端构建产物、数据库迁移和默认主题嵌入单一二进制；同时发布多架构容器镜像。

这套方案优先满足快速分发、低资源占用、自托管简单和公开读取性能。v1 不引入 Redis、队列、微服务或 PostgreSQL 兼容层。

## 2. 依赖选择

- HTTP：标准库 `net/http` + `github.com/go-chi/chi/v5`，中间件保持显式、可测试。
- 数据库：`database/sql` + `modernc.org/sqlite`，避免 CGO，便于 Linux amd64/arm64 交叉编译。
- 迁移：嵌入的顺序 SQL 迁移，由应用在启动时加进程锁后执行。
- 密码：`golang.org/x/crypto/argon2`；参数和哈希版本随记录保存。
- 日志：标准库 `log/slog` 输出结构化 JSON。
- 测试：标准库 `testing`、`httptest`；浏览器关键路径使用 Playwright。
- API：`api/openapi.yaml` 是唯一契约；Go DTO 与前端类型由契约约束并通过契约测试校验。

只在确有边界价值时增加依赖，不引入 ORM、依赖注入框架或通用事件总线。

## 3. 模块边界

```text
cmd/navax/             进程入口与版本信息
internal/app/          装配、启动、关闭和后台任务
internal/config/       环境变量、持久化设置和校验
internal/database/     SQLite、事务、迁移和备份
internal/httpapi/      路由、DTO、响应、认证/权限中间件
internal/auth/         用户、邀请、Session、密码
internal/navigation/   页面、分类、站点、排序、偏好，以及发布快照/公开投影/ETag
internal/catalog/      公开配置、主题列表、推荐目录、发现页
internal/assets/       图片上传：默认写入 `NAVAX_DATA_DIR/assets` 本地磁盘；管理员完整配置并启用 S3 后新上传可走对象存储，配置不完整或 S3 不可用时自动回退本地
internal/analytics/    匿名事件、聚合和保留策略
internal/admin/        运营管理和审计
internal/integrations/ SMTP、对象存储与 DNS 的配置/测试适配器（DNS 与 S3 写入为扩展预留）
internal/subdomains/   子域名申请、审核与 Host 解析数据
internal/maintenance/  更新检查、验签、备份和恢复
migrations/            只增不改的 SQL 迁移
web/                    React/Vite 前端
tests/                  跨模块集成与 E2E
```

HTTP 层只负责解析、校验、授权和序列化；业务模块拥有事务边界；数据库包不向上暴露 SQLite 专用对象。

## 4. 数据与一致性

核心表为 `users`、`sessions`、`invitations`、`navigation_pages`、`categories`、`sites`、`published_snapshots`、`subdomain_requests`、`themes`、公共目录、统计、资源、配置、审计、更新和备份记录。

- 启用 `foreign_keys`、WAL、`busy_timeout` 和短写事务。
- 分类/站点重排一次校验全部 ID 后一次提交，只增加一次草稿 revision。
- 发布在单事务中写不可变 JSON 快照并切换当前指针；公开请求从不读取草稿表。
- 子域名分配以长度策略判定：4 个及以上字符直接写入 `approved`，1–3 个字符写入 `pending` 并进入管理员审核；保留字与唯一约束始终先行。
- Session、邀请、恢复令牌只保存哈希；第三方秘密使用环境主密钥加密且永不回传。
- 完整 IP 不落库；访客 ID 使用按日轮换的 HMAC。

## 5. HTTP 与安全

同源 API 使用 Host-only、Secure、HttpOnly、SameSite=Lax Cookie。非安全方法在提供 `Origin`/`Referer` 时必须匹配 `PUBLIC_BASE_URL`（拦截浏览器 CSRF）；无 Origin 的机器客户端（curl/脚本）允许访问。滥用限流为**进程内内存**实现，部署模型为**单实例**；登录、邀请、事件、改密、恢复令牌和链接检查分别限流。公开快照返回 ETag 与 Cache-Control。

服务器抓取 URL 时，每次 DNS 解析和重定向都拒绝回环、私网、链路本地、保留地址及云元数据地址。上传限制 MIME、尺寸和大小，默认拒绝 SVG。

## 6. 部署和更新

默认数据目录包含 SQLite、上传、备份和更新暂存文件。进程提供 `/healthz`、`/readyz`、优雅停机和启动迁移。

原生二进制更新依次执行下载、SHA-256/Ed25519 验证、SQLite 在线备份、原子替换和重启；失败保留旧版本。容器模式只检查版本，不在容器内自替换。官方域名通过 `PUBLIC_BASE_URL`、`ROOT_DOMAIN` 和 `INSTANCE_NAME` 配置，自部署不硬编码 `nav.ax`。

## 7. 质量门槛

合并前必须通过 `go test ./...`、`go vet ./...`、前端 lint/type-check/build、OpenAPI 校验、API 集成测试和游客/用户/管理员关键 E2E。上线构建还需验证原生二进制与容器在空数据目录完成初始化、迁移、重启和持久化。
