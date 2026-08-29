#!/usr/bin/env bash
# Local tests for classify-nopanel-status.sh.
# Usage: bash deploy/test-classify-nopanel-status.sh
set -euo pipefail
cd "$(dirname "$0")/.."

fail() { echo "FAIL($1): $2"; exit 1; }

classify() {
  bash deploy/classify-nopanel-status.sh "$1"
}

expect() {
  local status="$1" want="$2"
  local got
  got="$(classify "$status")"
  [ "$got" = "$want" ] || fail "$status" "got=$got want=$want"
}

# Terminal success — stop polling.
expect success success
expect succeeded success
expect healthy success
expect done success

# Known terminal failures, including the 2026-08-29 preflight hang.
expect failed failure
expect error failure
expect cancelled failure
expect canceled failure
expect rolled_back failure
expect rollback_failed failure
expect activation_failed failure
expect health_check_failed failure
expect preflight_failed failure

# Any other *_failed status must also be terminal, so the wait loop
# does not grow a new hole every time NoPanel adds a phase.
expect build_failed failure
expect transfer_failed failure

# In-flight — keep polling.
expect running pending
expect in_progress pending
expect queued pending
expect "" pending

# The wait loop in CI must actually call the classifier; otherwise a YAML
# revert brings back the 30-minute preflight hang while this test stays green.
grep -q 'deploy/classify-nopanel-status.sh' .github/workflows/ci.yml \
  || fail ci.yml "deploy-production wait loop does not call classify-nopanel-status.sh"

echo "ok classify-nopanel-status"
