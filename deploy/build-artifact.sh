#!/usr/bin/env bash
# NoPanel artifact build — runs on the build server, cwd = source root.
# Contract: NPC_ARTIFACT_STAGING / NPC_VERSION / NPC_GIT_COMMIT_SHA injected by nopaneld.
# Output layout in staging: bin/navax + activate-artifact.sh + artifact-manifest.json
set -euo pipefail

STAGING="${NPC_ARTIFACT_STAGING:?NPC_ARTIFACT_STAGING is required}"
VERSION="${NPC_VERSION:-dev}"
COMMIT="${NPC_GIT_COMMIT_SHA:-unknown}"

# Alpha VPS runs at ~91% disk — refuse to build into a full disk.
avail_kb=$(df -Pk . | awk 'NR==2 {print $4}')
if [ "$avail_kb" -lt 1048576 ]; then
  echo "ERROR: <1GiB free on builder ($((avail_kb / 1024)) MiB) — clean /opt/navax-build first" >&2
  exit 1
fi

rm -rf "$STAGING"
mkdir -p "$STAGING/bin"

make build VERSION="$VERSION" COMMIT="$COMMIT"

cp bin/navax "$STAGING/bin/navax"
cp deploy/activate-artifact.sh "$STAGING/activate-artifact.sh"
chmod 0755 "$STAGING/activate-artifact.sh"

file_size() { stat -c %s "$1" 2>/dev/null || stat -f %z "$1"; }
file_sha() { sha256sum "$1" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$1" | awk '{print $1}'; }

sha_bin=$(file_sha "$STAGING/bin/navax")
sha_act=$(file_sha "$STAGING/activate-artifact.sh")

cat > "$STAGING/artifact-manifest.json" <<MANIFEST
{
  "schemaVersion": 1,
  "version": "${VERSION}",
  "gitCommitSha": "${COMMIT}",
  "builtAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "components": [
    { "name": "navax", "path": "bin/navax", "sha256": "${sha_bin}", "sizeBytes": $(file_size "$STAGING/bin/navax") },
    { "name": "activate-script", "path": "activate-artifact.sh", "sha256": "${sha_act}", "sizeBytes": $(file_size "$STAGING/activate-artifact.sh") }
  ]
}
MANIFEST

echo ">>> artifact staged: version=${VERSION} commit=${COMMIT} bin_sha256=${sha_bin}"
