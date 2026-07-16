#!/usr/bin/env bash
# nav.ax native-binary installer (systemd). Idempotent — safe to re-run.
#
# Prerequisites on the server (run as root):
#   1. Place the linux/amd64 binary at:   /opt/navax/bin/navax
#      (or pass its path as the first argument; it will be copied into place)
#   2. Run:                               bash install-navax.sh [path-to-binary]
#
# What it does:
#   - creates the `navax` system user and /opt/navax/{bin,data}
#   - installs the binary (0755, owned by navax)
#   - generates NAVAX_MASTER_KEY + NAVAX_SETUP_TOKEN on first run (kept across re-runs)
#   - writes /opt/navax/.env (0600) bound to 127.0.0.1:8090, behind a TLS reverse proxy
#   - installs + enables the systemd unit and health-checks /readyz
#
# It never overwrites an existing .env's secrets, so re-running will NOT rotate your
# master key (which would make previously-encrypted third-party credentials unreadable).
set -euo pipefail

APP_DIR=/opt/navax
BIN_DST="$APP_DIR/bin/navax"
ENV_FILE="$APP_DIR/.env"
UNIT=/etc/systemd/system/navax.service
BIND_ADDR=127.0.0.1:8090
PUBLIC_BASE_URL=https://nav.ax
ROOT_DOMAIN=nav.ax
INSTANCE_NAME=nav.ax

log() { printf '\033[1;36m[navax]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[navax] ERROR:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" = 0 ] || die "must run as root"

# 1. user + dirs
id navax >/dev/null 2>&1 || useradd --system --home-dir "$APP_DIR" --shell /usr/sbin/nologin navax
mkdir -p "$APP_DIR/bin" "$APP_DIR/data"

# 2. binary
SRC="${1:-$BIN_DST}"
if [ "$SRC" != "$BIN_DST" ]; then
  [ -f "$SRC" ] || die "binary not found at $SRC"
  install -m 0755 "$SRC" "$BIN_DST"
fi
[ -f "$BIN_DST" ] || die "no binary at $BIN_DST — copy the linux/amd64 build there first (or pass its path)"
chmod 0755 "$BIN_DST"
file "$BIN_DST" 2>/dev/null | grep -q "ELF 64-bit" || log "warning: $BIN_DST does not look like a linux ELF binary"

# 3. secrets — generated once, preserved on re-run
gen_secret() { # $1 = var name, $2 = generator command
  local name="$1" gen="$2" cur=""
  if [ -f "$ENV_FILE" ]; then cur="$(grep -E "^${name}=" "$ENV_FILE" | head -1 | cut -d= -f2- || true)"; fi
  if [ -n "$cur" ]; then printf '%s' "$cur"; else eval "$gen"; fi
}
MASTER_KEY="$(gen_secret NAVAX_MASTER_KEY 'openssl rand -base64 32')"
SETUP_TOKEN="$(gen_secret NAVAX_SETUP_TOKEN 'openssl rand -hex 32')"

# 4. .env (0600)
umask 077
cat > "$ENV_FILE" <<EOF
# nav.ax 生产配置（native binary + systemd）。由 install-navax.sh 生成。
PUBLIC_BASE_URL=$PUBLIC_BASE_URL
INSTANCE_NAME=$INSTANCE_NAME
ROOT_DOMAIN=$ROOT_DOMAIN

# 本机监听地址：只对反代（1Panel/openresty）暴露，不直接对外。
NAVAX_ADDR=$BIND_ADDR

# 挂 HTTPS 反代之后：会话 Cookie 加 Secure。
NAVAX_SECURE_COOKIES=true

# 可信反代来源（含本机与 Docker 网桥）。仅这些对端的 X-Forwarded-For 会被采信。
NAVAX_TRUSTED_PROXIES=127.0.0.1/32,172.16.0.0/12

# 数据目录（数据库、上传、备份）。
NAVAX_DATA_DIR=$APP_DIR/data

NAVAX_SESSION_TTL=720h
NAVAX_SHUTDOWN_TIMEOUT=15s

# 加密第三方凭据的主密钥。跨机灾备必须保留同一个值，请离线备份。
NAVAX_MASTER_KEY=$MASTER_KEY
# 首次 /setup 初始化令牌。
NAVAX_SETUP_TOKEN=$SETUP_TOKEN
EOF
chmod 0600 "$ENV_FILE"

# 5. ownership
chown -R navax:navax "$APP_DIR"

# 6. systemd unit
UNIT_SRC="$(cd "$(dirname "$0")" && pwd)/navax.service"
if [ -f "$UNIT_SRC" ]; then install -m 0644 "$UNIT_SRC" "$UNIT"; else die "navax.service not found next to installer"; fi
systemctl daemon-reload
systemctl enable navax >/dev/null 2>&1 || true
systemctl restart navax

# 7. health check
log "waiting for /readyz on http://$BIND_ADDR ..."
ok=0
for i in $(seq 1 30); do
  if curl -fsS "http://$BIND_ADDR/readyz" >/dev/null 2>&1; then ok=1; break; fi
  sleep 1
done
[ "$ok" = 1 ] || die "service did not become ready — check: journalctl -u navax -n 50"

log "service is up and healthy on $BIND_ADDR"
echo
echo "===================================================================="
echo " nav.ax 已作为 systemd 服务运行（127.0.0.1:8090）。"
echo " 下一步（NS 生效后）：在 1Panel 建反代站点 nav.ax -> 127.0.0.1:8090，"
echo " 签发裸域 + *.nav.ax 通配证书，然后访问 https://nav.ax/setup 初始化。"
echo
echo " 初始化令牌 NAVAX_SETUP_TOKEN：$SETUP_TOKEN"
echo " （NAVAX_MASTER_KEY 已写入 $ENV_FILE，请离线备份该文件。）"
echo "===================================================================="
