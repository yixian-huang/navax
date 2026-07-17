#!/usr/bin/env bash
# Local stub test for activate-artifact.sh. Stubs install/systemctl/curl via PATH.
# Usage: bash deploy/test-activate-artifact.sh   (expects all scenarios to pass)
set -euo pipefail
cd "$(dirname "$0")/.."

# Fresh sandbox: incoming artifact dir, releases dir, "live" app-bin dir, stub bin dir.
# Stubs default to all-success; systemctl/curl behavior is switched per-invocation
# by SYSTEMCTL_MODE / CURL_MODE (passed explicitly to run_activate, not inherited).
setup_sandbox() {
  local tmp; tmp=$(mktemp -d)
  mkdir -p "$tmp/incoming/bin" "$tmp/releases" "$tmp/appbin" "$tmp/stub"

  cat > "$tmp/stub/install" <<'EOS'
#!/usr/bin/env bash
# strip -o/-g/-m flag pairs, then: install <src> <dst>
args=(); while [ $# -gt 0 ]; do case "$1" in -o|-g|-m) shift 2;; *) args+=("$1"); shift;; esac; done
cp -f "${args[0]}" "${args[1]}"
EOS

  # SYSTEMCTL_MODE: ok (always succeed) | fail (always fail) |
  # fail-first (fail the 1st call, succeed every call after — models "new binary
  # won't start, but rollback restart of the previous binary works").
  cat > "$tmp/stub/systemctl" <<'EOS'
#!/usr/bin/env bash
echo "systemctl $*" >> "${STUB_LOG:?}"
case "${SYSTEMCTL_MODE:-ok}" in
  fail) exit 1 ;;
  fail-first)
    n=0
    [ -f "${STUB_COUNT_FILE:?}" ] && n=$(cat "$STUB_COUNT_FILE")
    n=$((n + 1))
    echo "$n" > "$STUB_COUNT_FILE"
    [ "$n" -eq 1 ] && exit 1 || exit 0
    ;;
  *) exit 0 ;;
esac
EOS

  cat > "$tmp/stub/curl" <<'EOS'
#!/usr/bin/env bash
echo "curl $*" >> "${STUB_LOG:?}"
[ "${CURL_MODE:-ok}" = "ok" ] && exit 0 || exit 22
EOS

  chmod +x "$tmp/stub/"*
  echo "$tmp"
}

# Runs activate-artifact.sh in the sandbox with APP_BIN redirected under tmp/.
# $1=tmp  $2=curl_mode(ok|fail)  $3=systemctl_mode(ok|fail|fail-first)
# Echoes the script's exit code; stdout/stderr land in tmp/stdout.log|stderr.log.
run_activate() {
  local tmp="$1" curl_mode="$2" systemctl_mode="$3"
  sed "s|^APP_BIN=.*|APP_BIN=$tmp/appbin/navax|" deploy/activate-artifact.sh > "$tmp/patched.sh"
  local rc=0
  STUB_LOG="$tmp/stub.log" STUB_COUNT_FILE="$tmp/stub.count" PATH="$tmp/stub:$PATH" \
    CURL_MODE="$curl_mode" SYSTEMCTL_MODE="$systemctl_mode" \
    NPC_ARTIFACT_INCOMING="$tmp/incoming" NPC_VERSION="test/branch-abcd1234" \
    NPC_RELEASE_ROOT="$tmp/releases" NAVAX_PROBE_INTERVAL=0 \
    bash "$tmp/patched.sh" > "$tmp/stdout.log" 2> "$tmp/stderr.log" || rc=$?
  echo "$rc"
}

restart_count() { grep -c "systemctl restart navax.service" "$1/stub.log" 2>/dev/null || true; }

fail() { echo "FAIL($1): $2"; exit 1; }

# Scenario: full success path — swap, restart, probe all succeed.
scenario_ok() {
  local tmp; tmp=$(setup_sandbox)
  printf 'NEW_BINARY_ok' > "$tmp/incoming/bin/navax"
  printf 'OLD_BINARY' > "$tmp/appbin/navax"

  local rc; rc=$(run_activate "$tmp" ok ok)
  [ "$rc" -eq 0 ] || fail ok "exit=$rc expected=0"
  grep -q NEW_BINARY "$tmp/appbin/navax" || fail ok "app binary not swapped"
  [ -d "$tmp/releases/test-branch-abcd1234" ] || fail ok "version dir not sanitized"
  [ "$(restart_count "$tmp")" -eq 1 ] || fail ok "expected exactly 1 systemctl restart, got $(restart_count "$tmp")"

  echo "PASS(ok)"
  rm -rf "$tmp"
}

# Scenario: readyz probe never turns healthy — must roll back to previous and exit 1.
scenario_probe_fail() {
  local tmp; tmp=$(setup_sandbox)
  printf 'NEW_BINARY_fail' > "$tmp/incoming/bin/navax"
  printf 'OLD_BINARY' > "$tmp/appbin/navax"

  local rc; rc=$(run_activate "$tmp" fail ok)
  [ "$rc" -eq 1 ] || fail probe-fail "exit=$rc expected=1"
  grep -q OLD_BINARY "$tmp/appbin/navax" || fail probe-fail "rollback did not restore old binary"

  echo "PASS(probe-fail)"
  rm -rf "$tmp"
}

# Scenario: systemctl restart fails right after the NEW binary is swapped in
# (service won't come up), but the rollback restart of the previous binary
# succeeds. Must still be treated as an activation failure: roll back and
# exit non-zero — this is the case bare `swap_and_restart` statements used to
# skip straight past (script would abort under set -e before ever reaching
# the rollback block).
scenario_restart_fail_then_rollback_ok() {
  local tmp; tmp=$(setup_sandbox)
  printf 'NEW_BINARY_restartfail' > "$tmp/incoming/bin/navax"
  printf 'OLD_BINARY' > "$tmp/appbin/navax"

  local rc; rc=$(run_activate "$tmp" ok fail-first)
  [ "$rc" -eq 1 ] || fail restart-fail "exit=$rc expected=1"
  grep -q OLD_BINARY "$tmp/appbin/navax" || fail restart-fail "old binary not restored after failed restart"
  [ "$(restart_count "$tmp")" -ge 2 ] || fail restart-fail "expected >=2 systemctl restart calls, got $(restart_count "$tmp")"

  echo "PASS(restart-fail-then-rollback-ok)"
  rm -rf "$tmp"
}

# Scenario: first install — no existing app binary to snapshot as "previous".
# Success path must still work, and no previous/navax should ever be created
# (nothing to roll back to, and nothing should attempt to).
scenario_first_install() {
  local tmp; tmp=$(setup_sandbox)
  printf 'NEW_BINARY_first' > "$tmp/incoming/bin/navax"
  # deliberately no $tmp/appbin/navax

  local rc; rc=$(run_activate "$tmp" ok ok)
  [ "$rc" -eq 0 ] || fail first-install "exit=$rc expected=0"
  grep -q NEW_BINARY "$tmp/appbin/navax" || fail first-install "app binary not installed"
  [ -f "$tmp/releases/previous/navax" ] && fail first-install "previous/navax should not exist on first install"
  [ "$(restart_count "$tmp")" -eq 1 ] || fail first-install "rollback should not have been attempted"

  echo "PASS(first-install)"
  rm -rf "$tmp"
}

# Scenario: incoming artifact has no binary — must exit non-zero before ever
# touching install/systemctl/curl or the live app binary.
scenario_missing_new_binary() {
  local tmp; tmp=$(setup_sandbox)
  # deliberately no $tmp/incoming/bin/navax
  printf 'OLD_BINARY' > "$tmp/appbin/navax"

  local rc; rc=$(run_activate "$tmp" ok ok)
  [ "$rc" -eq 1 ] || fail missing-new-binary "exit=$rc expected=1"
  grep -q OLD_BINARY "$tmp/appbin/navax" || fail missing-new-binary "app binary should be untouched"
  [ -s "$tmp/stub.log" ] && fail missing-new-binary "install/systemctl/curl should never have been invoked"

  echo "PASS(missing-new-binary)"
  rm -rf "$tmp"
}

scenario_ok
scenario_probe_fail
scenario_restart_fail_then_rollback_ok
scenario_first_install
scenario_missing_new_binary
echo "ALL PASS"
