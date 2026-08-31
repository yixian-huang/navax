# nav.ax

中文 | [English](README.en.md)

nav.ax 是一个 Go + React 构建的个性化导航站，面向个人、受邀用户和自托管场景。单个 Go 进程提供 REST API、公开导航页、管理界面、SQLite 存储与内嵌前端资源。

## Docker Compose 快速开始

需要 Docker 24+ 与 Compose v2。

```bash
cp .env.example .env
# 生产环境建议写入两个独立随机值
sed -i.bak "s/^NAVAX_SETUP_TOKEN=$/NAVAX_SETUP_TOKEN=$(openssl rand -hex 32)/" .env
sed -i.bak "s|^NAVAX_MASTER_KEY=$|NAVAX_MASTER_KEY=$(openssl rand -base64 32)|" .env
docker compose up -d --build
docker compose logs -f navax
```

访问 `http://localhost:8080/setup`，使用 `.env` 中的 `NAVAX_SETUP_TOKEN` 完成首次初始化。上线前把 `PUBLIC_BASE_URL` 改为真实 HTTPS 地址，并将 `NAVAX_SECURE_COOKIES` 设为 `true`。

环境变量、反代、备份、更新与 systemd 见 [docs/deployment.md](docs/deployment.md)；官方生产 CD 见 [deploy/README.md](deploy/README.md)。

健康检查：`GET /healthz` · 数据库就绪：`GET /readyz` · 构建信息：`GET /api/v1/version`。

## 本地开发与构建

需要 Go 1.25、Node.js 22 和 npm。

```bash
make check     # TypeScript、ESLint、mock 契约、gofmt、go vet
make test      # Go 全量测试
make build     # 前端 + 内嵌 + bin/navax
go run ./cmd/navax
```

仅前端：`cd web && npm run dev`（设 `VITE_ENABLE_API_MOCKS=true`）。合并门槛与架构边界见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 编写主题

主题是服务端校验入库的数据包，不是可执行代码。作者契约：[docs/theme-api.md](docs/theme-api.md)；最小示例：[examples/theme-starter](examples/theme-starter)。

## 文档

索引见 [docs/README.md](docs/README.md)。产品范围 [docs/requirements.md](docs/requirements.md)，架构 [docs/architecture.md](docs/architecture.md)。

## 贡献与许可

贡献请遵守 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。安全漏洞按 [SECURITY.md](SECURITY.md) 私密报告。源代码以 [AGPL-3.0-only](LICENSE) 许可；以网络服务运行修改版时须向用户提供对应源码（保留页脚源码链接）。「nav.ax」名称与 logo 标识官方实例，不在代码许可范围内。
