# 部署指南

nav.ax 自身只监听明文 HTTP（默认 `:8080`），**生产环境必须挂在一个负责 TLS 终止的反向代理之后**。本文以官方域名 **`nav.ax`** 为例，给出可直接照抄的服务器选型、域名/DNS、反代配置、通配子域名与 TLS，以及进程守护方案。自托管者把 `nav.ax` 换成自己的域名即可。

## 1. 服务器选型

nav.ax 是**单个静态 Go 二进制 + 内嵌前端 + SQLite（WAL）**，纯 Go、无 CGO、无外部数据库/Redis/队列依赖，资源占用很低。

| 规格 | 建议 |
| --- | --- |
| 架构 | 单节点即可（SQLite 为单机存储，**不做横向扩容**；靠反代 + 备份保证可用性） |
| CPU / 内存 | 2 vCPU / 2 GB 起步，2–4 GB 更从容 |
| 磁盘 | 20–40 GB SSD（数据库 + 上传资源 + 备份都在数据目录，注意预留增长空间） |
| 系统 | Debian/Ubuntu LTS 等主流 Linux（x86_64 或 arm64 均可） |
| 网络 | 需具备**出站**访问（签名更新拉取、SMTP 发信）；对外仅暴露反代的 80/443 |

供应商无关，任意 VPS 均可：Hetzner Cloud、DigitalOcean、Vultr、Linode、AWS Lightsail，或阿里云 / 腾讯云轻量应用服务器等。选择离目标用户近、带宽稳定的机房即可。

> 备份即数据目录：定期备份 `NAVAX_DATA_DIR`（含 `navax.db`、`assets/`、`backups/`）。跨机灾备还需另行保管同一个 `NAVAX_MASTER_KEY`（见 §3）。

## 2. 域名与 DNS（先添加这一步）

在支持 `.ax`（奥兰群岛 ccTLD）的注册商处注册 / 管理 `nav.ax`，并在其 DNS 处**先添加以下记录**，指向服务器公网地址：

| 类型 | 主机 | 记录值 | 说明 |
| --- | --- | --- | --- |
| `A` | `nav.ax`（裸域名 / `@`） | 服务器 IPv4 | 站点主入口，必需 |
| `AAAA` | `nav.ax` | 服务器 IPv6 | 有 IPv6 时添加，可选 |
| `A` | `*.nav.ax`（通配） | 同一 IPv4 | 启用用户子域名时必需（见 §5） |
| `AAAA` | `*.nav.ax` | 同一 IPv6 | 可选 |

启用用户子域名后，签发 `*.nav.ax` 通配证书需要 DNS-01 质询，因此还要在 DNS 服务商处**创建一个 API Token** 交给反代（见 §5）。DNS 生效（含 TTL）后再进行后续步骤。

## 3. 生产环境必备配置

| 变量 | 生产取值 | 说明 |
| --- | --- | --- |
| `PUBLIC_BASE_URL` | `https://nav.ax`（不以 `/` 结尾） | 用于 Origin 校验、HSTS、Cookie Secure 判定、发布快照 canonical。必须是真实对外 HTTPS 地址。 |
| `INSTANCE_NAME` | `nav.ax` | 实例展示名称。 |
| `NAVAX_SECURE_COOKIES` | `true` | 会话 Cookie 加 `Secure`。挂 HTTPS 反代后务必开启。 |
| `NAVAX_TRUSTED_PROXIES` | 反代来源 CIDR/IP | **关键**：不配置的话，限流与访问统计会全部按反代 IP 归并——限流塌成一个全局桶（暴力破解防护失效、正常用户互相牵连），统计 UV 恒为 1。只有直连对端命中此列表时才会采信其 `X-Forwarded-For`。 |
| `ROOT_DOMAIN` | `nav.ax` | 启用子域名时填写；用户子域名形如 `name.nav.ax`。 |
| `NAVAX_MASTER_KEY` | `openssl rand -base64 32` | 加密第三方凭据。不配置会在数据目录自动生成 `master.key`。**备份归档（`.navbak`）刻意不包含 `master.key`**，避免密文与密钥同 archive 外泄；就地恢复沿用磁盘上现有的密钥，但**跨机器灾备必须自行保留同一个 `NAVAX_MASTER_KEY`**，否则恢复后第三方凭据无法解密（数据库其余内容仍可恢复）。生产环境建议显式配置本变量并按部署密钥妥善保管。 |
| `NAVAX_SETUP_TOKEN` | `openssl rand -hex 32` | 首次 `/setup` 初始化令牌。 |

反代必须**保留 `Host` 头**并**透传 `X-Forwarded-For`**，否则子域名路由与真实 IP 都会失效。

## 4. 反向代理示例

### Caddy（推荐，自动签发/续期证书）

单域名：

```caddyfile
nav.ax {
    reverse_proxy 127.0.0.1:8080
}
```

启用用户子域名（需要通配证书，见 §5）：

```caddyfile
nav.ax, *.nav.ax {
    tls {
        dns cloudflare {env.CF_API_TOKEN}   # 通配证书需 DNS-01 质询
    }
    reverse_proxy 127.0.0.1:8080
}
```

Caddy 默认已透传 `Host` 与 `X-Forwarded-For`。此时把本机/网桥地址加入信任列表即可，例如 `NAVAX_TRUSTED_PROXIES=127.0.0.1/32`。

### nginx

```nginx
# /etc/nginx/sites-available/navax
server {
    listen 443 ssl http2;
    server_name nav.ax *.nav.ax;    # 通配需 §5 的通配证书

    ssl_certificate     /etc/letsencrypt/live/nav.ax/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/nav.ax/privkey.pem;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;               # 保留 Host → 子域名路由
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
server {
    listen 80;
    server_name nav.ax *.nav.ax;
    return 301 https://$host$request_uri;
}
```

对应 `NAVAX_TRUSTED_PROXIES=127.0.0.1/32`。若 nginx 与应用分处不同主机，填 nginx 所在网段。

## 5. 通配子域名与通配 TLS

用户子域名靠数据库映射 + 请求 `Host` 解析，应用本身**不签发证书、不写 DNS**（`internal/subdomains` 有意不做 DNS 自动化）。运维需自备：

1. **通配 DNS**：已在 §2 为 `*.nav.ax` 添加 A/AAAA 记录指向服务器。
2. **通配 TLS 证书**：`*.nav.ax` 证书只能用 **DNS-01** 质询签发。
   - Caddy：如上用 `tls { dns <provider> ... }`（需对应 DNS 插件构建）。
   - certbot：`certbot certonly --dns-cloudflare -d nav.ax -d '*.nav.ax'`。
3. 反代 `server_name`/站点块同时覆盖裸域名 `nav.ax` 与 `*.nav.ax`。

未知的 `*.nav.ax` 主机若无对应已批准子域名，应用返回 404；裸域名与系统域名回落到系统首页。

## 6. Docker Compose

`docker compose up --build` 使用仓库内 `docker-compose.yml`（只读根文件系统、非 root、`cap_drop: ALL`）。生产建议：

- 在 `.env` 设置上表变量，尤其 `NAVAX_TRUSTED_PROXIES`（含 Docker 网桥网段，如 `172.16.0.0/12`）。
- 将 `NAVAX_IMAGE` 固定到具体版本/摘要而非 `:latest`。
- 反代可另起一个容器或跑在宿主机；容器自更新被**有意禁用**，升级请 `docker compose pull && up -d`。

## 7. 原生二进制 + systemd（重要）

原生二进制支持签名自更新，但**更新/恢复后进程只会优雅退出、不会自拉起**——必须由进程管理器负责重启。请使用 `Restart=always` 的 systemd 单元。

**视频背景依赖**：原生部署需本机安装 `ffmpeg`（及通常随包提供的 `ffprobe`），用于视频压缩与 poster 截帧。官方 Docker 镜像已内置；`deploy/install-navax.sh` 会在缺包时尝试通过 apt/dnf/apk 安装。未安装时上传视频会返回 503「服务器未安装 ffmpeg」。

```bash
# Debian / Ubuntu
sudo apt-get update && sudo apt-get install -y ffmpeg

# 校验（systemd 服务用户也须能在默认 PATH 找到）
which ffmpeg ffprobe
```

```ini
# /etc/systemd/system/navax.service
[Unit]
Description=nav.ax
After=network-online.target
Wants=network-online.target

[Service]
User=navax
WorkingDirectory=/opt/navax
EnvironmentFile=/opt/navax/.env
ExecStart=/opt/navax/bin/navax
Restart=always
RestartSec=2
# 与应用优雅停机窗口对齐（NAVAX_SHUTDOWN_TIMEOUT 默认 15s）
TimeoutStopSec=20

[Install]
WantedBy=multi-user.target
```

`systemctl enable --now navax`。这样管理员在后台点“应用更新”后，进程退出即被 systemd 用新二进制拉起。

## 8. 上线前检查清单

- [ ] 服务器就绪，`nav.ax` 与（可选）`*.nav.ax` 的 DNS 记录已添加并生效
- [ ] `PUBLIC_BASE_URL=https://nav.ax`，`NAVAX_SECURE_COOKIES=true`
- [ ] `NAVAX_TRUSTED_PROXIES` 覆盖反代来源，并验证统计 UV / 限流按真实 IP 生效
- [ ] `NAVAX_MASTER_KEY`、`NAVAX_SETUP_TOKEN` 由 `openssl rand` 显式提供并妥善备份
- [ ] 启用子域名时：通配 DNS + 通配 TLS 就绪，反代保留 `Host`
- [ ] 原生部署：systemd `Restart=always` 已配置
- [ ] 原生部署：已安装 `ffmpeg`（视频背景）；`which ffmpeg` 对运行用户可见
- [ ] 首次访问 `https://nav.ax/setup` 用 `NAVAX_SETUP_TOKEN` 完成初始化
- [ ] `/readyz` 返回 200，`/healthz` 存活
