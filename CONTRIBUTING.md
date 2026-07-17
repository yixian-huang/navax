# 贡献指南

感谢你对 nav.ax 的兴趣。提交代码前请先读一遍本文——它能帮你的 PR 一次通过。

较大的功能改动请先开 issue 讨论方向,避免白做;小修小补直接提 PR 即可。
安全漏洞不要开公开 issue,请走 [SECURITY.md](SECURITY.md) 的私密报告渠道。

## 开发环境

- Go 1.25、Node.js 22、npm(Docker 可选)
- 全部命令在仓库根目录执行

```bash
make frontend  # npm ci + Vite 生产构建到 web/out
make check     # TypeScript、ESLint、mock 契约守卫、gofmt、go vet
make test      # Go 全量测试
make build     # 前端 + 内嵌 + 静态二进制 bin/navax
go run ./cmd/navax   # 本地运行(环境变量见 .env.example)
```

仅前端开发:`cd web && npm run dev`(Vite,端口 3000)。没有到 Go 后端的代理,
设 `VITE_ENABLE_API_MOCKS=true` 启用拦截 fetch 的 mock API(仅开发环境)。

## 合并门槛

每个 PR 必须通过:

- `make check`
- `go test -race ./...`
- `make build`

按改动类型追加:

- **接口契约变更**:同步更新 `api/openapi.yaml`(它是唯一契约来源),并通过
  `make test-contract`;mock 响应保持通过 `make test-mock`。
- **UI / 交互变更**:通过 `make e2e`(Playwright),并附浏览器冒烟结果——
  加载、空态、错误、移动端、键盘、暗色主题六种状态。PR 里附截图。
- **Bug 修复**:附带回归测试。
- **数据库变更**:只在 `migrations/` 追加新的顺序迁移文件,不修改已有迁移。

## 架构边界

这些是有意为之的约束,PR 引入以下内容会被拒绝:

- 不引入 ORM、DI 框架、事件总线、Redis、消息队列、PostgreSQL。
- `internal/httpapi/` 只负责路由、DTO、中间件与序列化;业务逻辑和事务边界
  放在 `internal/` 下的领域包里。
- 前端所有 HTTP 调用走 `web/src/api/`,不绕过 OpenAPI 契约;生产代码不得
  依赖 `web/src/mocks/`。
- 保持既有安全不变量(会话 Cookie 属性、Origin 校验、SSRF 防护、上传限制、
  令牌仅存哈希、限流),详见 [SECURITY.md](SECURITY.md) 的 Scope 一节。

## 代码风格

- Go:gofmt;包小而聚焦;导出名 `PascalCase`;表驱动测试命名
  `TestFeatureCondition`,与源码同目录 `*_test.go`;持久化和鉴权行为优先写
  SQLite 集成测试。
- 前端:TypeScript 函数组件;两空格缩进;组件文件 `PascalCase`;hooks 用
  `useXxx`;`@/` 别名指向 `web/src/`。React hooks、react-router 与
  `useTranslation` 由 auto-import 提供,不要手动 import。

## 提交与 PR

- Conventional Commits,英文主题,例如 `feat: add signed instance backups`。
- PR 描述需包含:用户可见的行为变化、关联 issue、迁移或配置变更、执行过的
  验证。

## 行为准则

参与本项目即表示同意 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。
