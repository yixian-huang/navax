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

访问 `http://localhost:8080/setup`，使用 `.env` 中的 `NAVAX_SETUP_TOKEN` 完成首次初始化。上线前必须把 `.env` 中的 `PUBLIC_BASE_URL` 改为真实 HTTPS 地址，并将 `NAVAX_SECURE_COOKIES` 设为 `true`。反向代理需保留原始 `Host`；启用个人子域名时，根域名在管理后台「系统设置 → 域名」中填写并开启，另需配置泛域名 DNS 与 TLS。

官方 `nav.ax` 实例中，4 个及以上字符的可用子域名会自动启用；1–3 个字符的稀缺短域名进入管理员审核。付费订阅尚未进入 v1，未来商业化主要围绕更高链接额度、短子域名和白标能力展开。

健康检查：`GET /healthz`；数据库就绪检查：`GET /readyz`；构建信息：`GET /api/v1/version`。

## 本地开发与构建

需要 Go 1.25、Node.js 22 和 npm。

```bash
make frontend  # 安装依赖并构建 web/out
make check     # TypeScript、ESLint、gofmt 与 go vet
make test      # Go 全量测试
make embed     # 将前端产物复制到 internal/webui/dist
make build     # 生成内嵌前端的 bin/navax 静态二进制
```

原生运行时需先导出环境变量：

```bash
set -a; . ./.env; set +a
NAVAX_DATA_DIR=./data ./bin/navax
```

## 配置

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `NAVAX_ADDR` | `:8080` | HTTP 监听地址 |
| `NAVAX_DATA_DIR` | `./data` | SQLite、上传、备份与密钥目录；容器内为 `/data` |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | 对外绝对地址，不带末尾 `/` |
| `INSTANCE_NAME` | `nav.ax` | 实例名称 |
| `NAVAX_SETUP_TOKEN` | 启动时随机生成 | 首次初始化令牌，至少 32 字符 |
| `NAVAX_MASTER_KEY` | 空 | 加密第三方凭据的 Base64 32 字节密钥；配置后不可随意更换 |
| `NAVAX_SECURE_COOKIES` | 随 HTTPS 自动开启 | 是否仅通过 HTTPS 发送 Session Cookie |
| `NAVAX_SESSION_TTL` | `720h` | Session 有效期 |
| `NAVAX_SHUTDOWN_TIMEOUT` | `15s` | 优雅停机期限 |
| `NAVAX_UPDATE_MANIFEST_URL` | 空 | 可选的签名更新清单 URL；官方取值见[更新](#更新) |
| `NAVAX_UPDATE_PUBLIC_KEY` | 空 | Base64 Ed25519 更新验签公钥；官方取值见[更新](#更新) |

## 数据、备份与恢复

持久数据全部位于 `NAVAX_DATA_DIR`。Compose 使用固定名称卷 `navax-data`；删除容器不会删除该卷。优先在管理后台创建、下载和恢复 `.navbak` 完整实例归档，其中包含 SQLite 快照、本地上传资源以及实例生成的密钥。恢复会使进程正常退出，Compose 的重启策略会在下一次启动时校验并原子应用归档。

做整卷离线备份时先停止服务，避免复制 WAL 中间状态：

```bash
docker compose stop navax
docker run --rm -v navax-data:/data:ro -v "$PWD":/backup alpine \
  tar czf /backup/navax-data-$(date +%F).tar.gz -C /data .
docker compose start navax
```

如果通过 `.env` 显式设置了 `NAVAX_MASTER_KEY`，它属于部署配置、不会写入归档，必须与备份一同保存在受控位置，否则已加密的第三方凭据无法解密。

## 更新

容器部署不会在容器内替换自身。先创建备份，再由 Compose 拉取并重建：

```bash
docker compose pull
docker compose up -d
docker compose ps
```

使用本地源码镜像时执行 `docker compose build --pull && docker compose up -d`。原生二进制可从 GitHub Release 下载对应平台文件，并用 `SHA256SUMS` 校验；配置签名更新清单后，也可在管理后台执行带备份与校验的原子更新。

发布工作流会在仓库 Secret `NAVAX_UPDATE_SIGNING_KEY_DER` 存在时生成 `update-manifest.json`。Secret 内容是 Ed25519 PKCS#8 DER 私钥的 Base64；实例侧的 `NAVAX_UPDATE_PUBLIC_KEY` 使用对应 32 字节原始公钥的 Base64。私钥只用于 GitHub Actions 签名，不应部署到实例。

### 启用后台一键更新（自托管）

官方发布的 `update-manifest.json` 使用官方私钥签名。自托管实例填入下面两项，即可在管理后台执行带备份与校验的原子更新：

```bash
NAVAX_UPDATE_MANIFEST_URL=https://github.com/yixian-huang/navax/releases/latest/download/update-manifest.json
NAVAX_UPDATE_PUBLIC_KEY=P0yCGX0jV+TAx/BfmY7tvGKFeRQmtjq/y9/pMl8ciDA=
```

生效条件：

- **需等首个正式版**。带连字符的 tag（如 `v0.1.0-rc.1`）会作为预发布上传，而 `latest/download` 只解析正式版，因此该 URL 在 `v0.1.0` 发布前返回 404。也可改为指向某个具体 tag 的资产地址。
- **仅原生二进制部署**支持自更新；容器部署会被拒绝，请改用 `docker compose pull`。
- 更新后进程**优雅退出但不自拉起**，必须使用 `Restart=always` 的 systemd 单元（见 [docs/deployment.md](docs/deployment.md) §7）。
- 校验失败、版本不高于当前版本，或清单签名不匹配时，更新会被拒绝并保留旧版本。

## 项目结构

- `cmd/navax/`：程序入口与构建信息
- `internal/`：业务、HTTP、SQLite、运维和内嵌 Web
- `migrations/`：启动时自动执行的 SQLite 迁移
- `web/`：React/Vite 前端
- `api/openapi.yaml`：HTTP API 契约（接口唯一契约来源）
- `docs/`：需求、架构与部署（中文，见下）
- `deploy/`：原生二进制安装与官方生产 CD 说明

## 文档

| 文档 | 内容 |
| --- | --- |
| [docs/requirements.md](docs/requirements.md) | 产品范围与验收（权威） |
| [docs/architecture.md](docs/architecture.md) | 模块边界、数据与安全不变量 |
| [docs/deployment.md](docs/deployment.md) | 自托管：DNS、TLS、反代、环境变量 |
| [deploy/README.md](deploy/README.md) | systemd 安装、升级、官方 CI→NoPanel CD |
| [docs/design-background-media-library.md](docs/design-background-media-library.md) | 背景媒体库专项设计 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 开发命令、合并门槛、架构边界 |

## 贡献

贡献流程、合并门槛与架构边界见 [CONTRIBUTING.md](CONTRIBUTING.md);参与项目请遵守 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。安全漏洞请按 [SECURITY.md](SECURITY.md) 通过私密渠道报告,不要公开提交 issue。

## 许可证与品牌

源代码以 [AGPL-3.0-only](LICENSE) 许可发布。按照 AGPL 第 13 条,若你以网络服务形式运行修改后的版本,必须向其用户提供对应源码——内置页脚的源码链接即用于满足这一要求,分发修改版时请保留等效入口。

"nav.ax" 名称与 logo 用于标识官方实例,不在代码许可范围内。部署修改版本请使用你自己的名称与标识,避免与官方实例混淆。
