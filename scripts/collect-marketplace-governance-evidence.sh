#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/collect_marketplace_governance_evidence.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: bash scripts/collect-marketplace-governance-evidence.sh \
  --artifact-id artifact-marketplace-governance-... \
  --commit <git-commit> \
  --run-id <target-release-run-id> \
  --recorded-at <ISO-8601 timestamp> \
  --proof-file /path/outside/git/marketplace-governance-proof.json \
  --output /path/outside/git/artifacts/<artifact-id>.json

Or use --proof-url with OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN, --bearer-token-file,
or --cookie-file to fetch proof JSON from a protected target evidence URL.

Builds a marketplace-governance-proof artifact body from target governance evidence.
The output is intended for OBLIVIOUS_TARGET_ARTIFACT_DIR validation by
verify-target-release-evidence.sh. Proof URLs must not embed credentials or
token/password/API-key style query parameters.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

"$python_bin" "$impl" "$@"
