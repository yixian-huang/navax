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

## 生产自动 CD（官方 nav.ax）

官方生产（VIP Cloud `/opt/navax` + systemd）经 **NoPanel artifact** 流水线发布：

1. **构建机**（当前 env `buildServerId` = **VIP Cloud**，与生产同机）：`bash deploy/build-artifact.sh` → 产物 `bin/navax` + `activate-artifact.sh`
2. **传输** 到激活目录（`distributionConfig.targetPath`，现为 `/opt/navax-build/incoming`）
3. **激活** `activate-artifact.sh`：换二进制 → `systemctl restart navax` → 探 `http://127.0.0.1:8090/readyz` → 失败回滚

> **为何不是 Alpha？** Alpha 上 `nopanel-probe` 默认 `PrivateTmp=true`，且 SFTP 上传的 `/tmp/npc-source-*.tar.gz` 属 root，probe 用户无法 `rm`。已在 Alpha 加 drop-in `PrivateTmp=false` 并 `chown` `/opt/navax-build`，但 root 拥有的 sticky `/tmp` 上传文件仍无法被 probe 删除，故生产 env 暂时把 **build server 设为 VIP Cloud（control-plane SSH / root）**。恢复 Alpha 构建前需 NoPanel 侧把源码包落到 probe 可写路径（或上传用户改为 `nopanel-probe`）。

### 触发方式

| 方式 | 何时 | 说明 |
|------|------|------|
| **GitHub Actions（默认）** | `main` 上 push 且 CI 三门禁（verify / e2e / container）全绿 | 见 `.github/workflows/ci.yml` 的 `deploy-production` job |
| **手动 CLI** | 任意时刻 | `npc deploy navax production --ref main --wait` |
| **Deploy hook** | 任意 git ref | `POST https://ops.nopanel.dev/api/v1/deploy-hooks/<project>/production` + `{"gitRef":"<sha|branch|tag>"}` |

### 仓库 Secret

在 GitHub → Settings → Secrets → Actions 配置：

- `NPC_API_KEY`：NoPanel API Key（需 `deployments:write`、`environments:read`、`projects:read`；与本机 `npc` 可用的 team key 对齐）。**401** 时先 `npc auth test` 确认本地 key，再 `gh secret set NPC_API_KEY` 覆盖。

未配置时 `deploy-production` 会失败并提示，避免静默跳过。

### 本地一键发布当前 main

```bash
npc deploy navax production --ref main --wait
```

构建机磁盘紧张时先清理 Docker build cache / `/opt/navax-build`，否则 `build-artifact.sh` 在可用空间 <1GiB 时会拒绝构建。VIP 上需有 `sudo`（activate 包装会调用）；`apt-get install -y sudo` 一次即可。
