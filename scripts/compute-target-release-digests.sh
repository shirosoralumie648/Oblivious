#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/target_release_digests.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: bash scripts/compute-target-release-digests.sh \
  --manifest /path/outside/git/target-release-evidence.json \
  --artifact-dir /path/outside/git/downloaded-artifacts \
  [--write] [--output /path/outside/git/target-release-digests.json]

Computes canonical target release digest fields after the manifest and every
artifact body have been collected. The digest convention breaks the strict
verifier self-reference by normalizing these circular fields to 64 zeroes while
hashing:

  strictVerifier.targetEvidenceSha256
  strictVerifier.artifactBundleSha256
  artifacts[<strict-verifier-log>].sha256

With --write, the tool writes targetEvidenceSha256 and artifactBundleSha256 back
to the manifest, refreshes the strict-verifier artifact body, and updates
artifact sha256 values from the downloaded body directory.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

"$python_bin" "$impl" "$@"
