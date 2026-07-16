# 部署指南

nav.ax 自身只监听明文 HTTP（默认 `:8080`），**生产环境必须挂在一个负责 TLS 终止的反向代理之后**。本文给出可直接照抄的反代配置、通配子域名与 TLS 方案、以及原生二进制的进程守护。

## 1. 生产环境必备配置

| 变量 | 生产取值 | 说明 |
| --- | --- | --- |
| `PUBLIC_BASE_URL` | `https://你的域名`（不以 `/` 结尾） | 用于 Origin 校验、HSTS、Cookie Secure 判定、发布快照 canonical。必须是真实对外 HTTPS 地址。 |
| `NAVAX_SECURE_COOKIES` | `true` | 会话 Cookie 加 `Secure`。挂 HTTPS 反代后务必开启。 |
| `NAVAX_TRUSTED_PROXIES` | 反代来源 CIDR/IP | **关键**：不配置的话，限流与访问统计会全部按反代 IP 归并——限流塌成一个全局桶（暴力破解防护失效、正常用户互相牵连），统计 UV 恒为 1。只有直连对端命中此列表时才会采信其 `X-Forwarded-For`。 |
| `ROOT_DOMAIN` | 你的根域名（启用子域名时） | 例如 `nav.ax`。用户子域名形如 `name.nav.ax`。 |
| `NAVAX_MASTER_KEY` | `openssl rand -base64 32` | 加密第三方凭据。不配置会在数据目录自动生成 `master.key`。**备份归档（`.navbak`）刻意不包含 `master.key`**，避免密文与密钥同archive外泄；就地恢复沿用磁盘上现有的密钥，但**跨机器灾备必须自行保留同一个 `NAVAX_MASTER_KEY`**，否则恢复后第三方凭据无法解密（数据库其余内容仍可恢复）。生产环境建议显式配置本变量并按部署密钥妥善保管。 |
| `NAVAX_SETUP_TOKEN` | `openssl rand -hex 32` | 首次 `/setup` 初始化令牌。 |

反代必须**保留 `Host` 头**并**透传 `X-Forwarded-For`**，否则子域名路由与真实 IP 都会失效。

## 2. 反向代理示例

### Caddy（推荐，自动签发/续期证书）

单域名：

```caddyfile
nav.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

启用用户子域名（需要通配证书，见 §3）：

```caddyfile
nav.example.com, *.nav.example.com {
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
    server_name nav.example.com *.nav.example.com;    # 通配需 §3 的通配证书

    ssl_certificate     /etc/letsencrypt/live/nav.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/nav.example.com/privkey.pem;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;               # 保留 Host → 子域名路由
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
    }
}
server {
    listen 80;
    server_name nav.example.com *.nav.example.com;
    return 301 https://$host$request_uri;
}
```

对应 `NAVAX_TRUSTED_PROXIES=127.0.0.1/32`。若 nginx 与应用分处不同主机，填 nginx 所在网段。

## 3. 通配子域名与通配 TLS

用户子域名靠数据库映射 + 请求 `Host` 解析，应用本身**不签发证书、不写 DNS**（`internal/subdomains` 有意不做 DNS 自动化）。运维需自备：

1. **通配 DNS**：为 `*.nav.example.com` 添加 A/AAAA 记录（或 CNAME）指向服务器。
2. **通配 TLS 证书**：`*.example.com` 证书只能用 **DNS-01** 质询签发。
   - Caddy：如上用 `tls { dns <provider> ... }`（需对应 DNS 插件构建）。
   - certbot：`certbot certonly --dns-cloudflare -d nav.example.com -d '*.nav.example.com'`。
3. 反代 `server_name`/站点块同时覆盖裸域名与 `*.根域名`。

未知的 `*.根域名` 主机若无对应已批准子域名，应用返回 404；裸域名与系统域名回落到系统首页。

## 4. Docker Compose

`docker compose up --build` 使用仓库内 `docker-compose.yml`（只读根文件系统、非 root、`cap_drop: ALL`）。生产建议：

- 在 `.env` 设置上表变量，尤其 `NAVAX_TRUSTED_PROXIES`（含 Docker 网桥网段，如 `172.16.0.0/12`）。
- 将 `NAVAX_IMAGE` 固定到具体版本/摘要而非 `:latest`。
- 反代可另起一个容器或跑在宿主机；容器自更新被**有意禁用**，升级请 `docker compose pull && up -d`。

## 5. 原生二进制 + systemd（重要）

原生二进制支持签名自更新，但**更新/恢复后进程只会优雅退出、不会自拉起**——必须由进程管理器负责重启。请使用 `Restart=always` 的 systemd 单元：

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

## 6. 上线前检查清单

- [ ] `PUBLIC_BASE_URL` 为真实 HTTPS 地址，`NAVAX_SECURE_COOKIES=true`
- [ ] `NAVAX_TRUSTED_PROXIES` 覆盖反代来源，并验证统计 UV / 限流按真实 IP 生效
- [ ] `NAVAX_MASTER_KEY`、`NAVAX_SETUP_TOKEN` 由 `openssl rand` 显式提供
- [ ] 启用子域名时：通配 DNS + 通配 TLS 就绪，反代保留 `Host`
- [ ] 原生部署：systemd `Restart=always` 已配置
- [ ] `/readyz` 返回 200，`/healthz` 存活
