#!/usr/bin/env bash
# NoPanel artifact activate — runs on the deploy server (root, via runner).
# Swap /opt/navax/bin/navax → systemctl restart → probe /readyz → roll back on failure.
set -euo pipefail

INCOMING="${NPC_ARTIFACT_INCOMING:?NPC_ARTIFACT_INCOMING is required}"
VERSION="${NPC_VERSION:-unknown}"
UNIT="${NPC_SYSTEMD_UNIT:-navax.service}"
RELEASE_ROOT="${NPC_RELEASE_ROOT:-/opt/navax/releases}"
PROBE_INTERVAL="${NAVAX_PROBE_INTERVAL:-3}"
APP_BIN=/opt/navax/bin/navax
READY_URL=http://127.0.0.1:8090/readyz

# NPC_VERSION is "<ref>-<sha8>" and ref may contain '/'; sanitize for dir names.
VERSION_SAFE=$(printf '%s' "$VERSION" | tr '/' '-')

NEW_BIN="$INCOMING/bin/navax"
[ -f "$NEW_BIN" ] || { echo "ERROR: new binary missing at $NEW_BIN" >&2; exit 1; }

mkdir -p "$RELEASE_ROOT/$VERSION_SAFE" "$RELEASE_ROOT/previous"
install -m 0755 "$NEW_BIN" "$RELEASE_ROOT/$VERSION_SAFE/navax"

# Preserve the currently-running binary as the rollback target (skip on first install).
if [ -f "$APP_BIN" ]; then
  cp -f "$APP_BIN" "$RELEASE_ROOT/previous/navax"
fi

probe_ready() {
  local i
  for i in $(seq 1 10); do
    sleep "$PROBE_INTERVAL"
    if curl -fsS -m 4 "$READY_URL" >/dev/null 2>&1; then return 0; fi
  done
  return 1
}

swap_and_restart() {
  install -o navax -g navax -m 0755 "$1" "$APP_BIN" || return 1
  systemctl restart "$UNIT" || return 1
}

# Any failure here (bad binary, missing user, restart failure) or a failed
# readyz probe must fall through to the rollback path below, never abort the
# script outright — hence the guarded `if ... && ...` instead of bare statements.
if swap_and_restart "$RELEASE_ROOT/$VERSION_SAFE/navax" && probe_ready; then
  # Keep the 5 most recent release dirs (never touch previous/).
  ls -1dt "$RELEASE_ROOT"/*/ 2>/dev/null | grep -v '/previous/$' | tail -n +6 | xargs -r rm -rf
  echo ">>> activate ok: version=$VERSION"
  exit 0
fi

echo "!!! activation failed for $VERSION — rolling back to previous binary" >&2
if [ -f "$RELEASE_ROOT/previous/navax" ]; then
  if swap_and_restart "$RELEASE_ROOT/previous/navax" && probe_ready; then
    echo "!!! rollback complete — previous version restored and healthy" >&2
  else
    echo "!!! ROLLBACK FAILED — manual intervention required (attempted restore to $APP_BIN; health unconfirmed)" >&2
  fi
else
  echo "!!! no previous binary recorded — nothing to roll back to" >&2
fi
exit 1
