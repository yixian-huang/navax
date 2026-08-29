#!/usr/bin/env bash
# Classify a NoPanel deployment status for the GitHub Actions wait loop.
# Prints one of: success | failure | pending
set -euo pipefail
status="${1:-}"
case "$status" in
  success|succeeded|healthy|done)
    echo success
    ;;
  failed|error|cancelled|canceled|rolled_back|skipped|rejected|*_failed)
    echo failure
    ;;
  *)
    echo pending
    ;;
esac
