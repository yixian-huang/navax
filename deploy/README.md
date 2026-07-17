# nav.ax 部署套件（native binary + systemd）

配合 `docs/deployment.md` §7 使用。目标：把交叉编译好的 Linux 二进制装成一个受 systemd 守护、只监听 `127.0.0.1:8090` 的服务，再由反向代理（1Panel / openresty / Caddy / nginx）终止 TLS。

## 文件

- `navax.service` — systemd 单元（`Restart=always`，最小可写路径加固到数据目录）。
- `install-navax.sh` — 幂等安装脚本：建用户/目录、装二进制、首次生成并保存 `NAVAX_MASTER_KEY` 与 `NAVAX_SETUP_TOKEN`、写 `/opt/navax/.env`（0600）、装并启用 systemd 单元、健康检查 `/readyz`。**重跑不会轮换主密钥**（避免既有加密凭据失效）。

## 交叉编译二进制（在开发机）

```bash
make embed   # 先构建并内嵌前端
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -ldflags="-s -w -X main.version=${VERSION} -X main.deployment=binary" \
  -o bin/navax-linux-amd64 ./cmd/navax
```

## 安装（在服务器，root）

1. 把 `bin/navax-linux-amd64` 传到服务器（scp / 1Panel 文件管理器上传均可），例如放到 `/root/navax`。
2. 把本目录（`navax.service` 与 `install-navax.sh`）一并放到服务器同一目录。
3. 运行：

```bash
bash install-navax.sh /root/navax
```

脚本会尝试安装 `ffmpeg`（视频背景压缩与 poster）。若自动安装失败，请手动：`apt-get install -y ffmpeg`（或发行版等价包）。

脚本结束会打印 `NAVAX_SETUP_TOKEN`；`NAVAX_MASTER_KEY` 已写入 `/opt/navax/.env`，**请离线备份该文件**（跨机灾备需保留同一个主密钥）。

## 之后（NS 生效后）

1. 反代：在 1Panel 建站点 `nav.ax` → 反向代理到 `http://127.0.0.1:8090`，保留 `Host`、透传 `X-Forwarded-For`。
2. 证书：裸域 `nav.ax` 走 HTTP-01；`*.nav.ax` 通配走 Cloudflare DNS-01（需一个 `Zone→DNS→Edit` 且限定到 nav.ax 的 API Token）。
3. 初始化：访问 `https://nav.ax/setup`，用上面的 `NAVAX_SETUP_TOKEN` 完成初始化。
4. 校验：`/readyz` 返回 200；确认统计 UV / 限流按真实 IP 生效（`NAVAX_TRUSTED_PROXIES` 已含本机与 Docker 网桥）。

## 升级

替换 `/opt/navax/bin/navax` 后 `systemctl restart navax`。原生二进制的签名自更新在退出后由 systemd（`Restart=always`）自动拉起。
