#!/usr/bin/env bash
# Local stub test for activate-artifact.sh. Stubs install/systemctl/curl via PATH.
# Usage: bash deploy/test-activate-artifact.sh   (expects both scenarios to pass)
set -euo pipefail
cd "$(dirname "$0")/.."

run_scenario() { # $1=curl_mode(ok|fail) $2=expected_exit
  local tmp; tmp=$(mktemp -d)
  mkdir -p "$tmp/incoming/bin" "$tmp/releases" "$tmp/appbin" "$tmp/stub"
  printf 'NEW_BINARY_%s' "$1" > "$tmp/incoming/bin/navax"
  printf 'OLD_BINARY' > "$tmp/appbin/navax"

  cat > "$tmp/stub/install" <<'EOS'
#!/usr/bin/env bash
# strip -o/-g/-m flag pairs, then: install <src> <dst>
args=(); while [ $# -gt 0 ]; do case "$1" in -o|-g|-m) shift 2;; *) args+=("$1"); shift;; esac; done
cp -f "${args[0]}" "${args[1]}"
EOS
  cat > "$tmp/stub/systemctl" <<'EOS'
#!/usr/bin/env bash
echo "systemctl $*" >> "${STUB_LOG:?}"
EOS
  cat > "$tmp/stub/curl" <<EOS
#!/usr/bin/env bash
echo "curl \$*" >> "\${STUB_LOG:?}"
[ "$1" = "ok" ] && exit 0 || exit 22
EOS
  chmod +x "$tmp/stub/"*

  local rc=0
  STUB_LOG="$tmp/stub.log" PATH="$tmp/stub:$PATH" \
    NPC_ARTIFACT_INCOMING="$tmp/incoming" NPC_VERSION="test/branch-abcd1234" \
    NPC_RELEASE_ROOT="$tmp/releases" NAVAX_PROBE_INTERVAL=0 \
    bash -c "APP_BIN_OVERRIDE=1; sed 's|^APP_BIN=.*|APP_BIN='"$tmp"'/appbin/navax|' deploy/activate-artifact.sh > $tmp/patched.sh; bash $tmp/patched.sh" || rc=$?

  [ "$rc" -eq "$2" ] || { echo "FAIL($1): exit=$rc expected=$2"; exit 1; }
  if [ "$1" = ok ]; then
    grep -q NEW_BINARY "$tmp/appbin/navax" || { echo "FAIL(ok): app binary not swapped"; exit 1; }
    [ -d "$tmp/releases/test-branch-abcd1234" ] || { echo "FAIL(ok): version dir not sanitized"; exit 1; }
  else
    grep -q OLD_BINARY "$tmp/appbin/navax" || { echo "FAIL(fail): rollback did not restore old binary"; exit 1; }
  fi
  echo "PASS($1)"
  rm -rf "$tmp"
}

run_scenario ok 0
run_scenario fail 1
echo "ALL PASS"
