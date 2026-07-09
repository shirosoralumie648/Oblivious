#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/collect_deployment_evidence.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  cat <<'EOF'
Usage: bash scripts/collect-deployment-evidence.sh \
  --artifact-id artifact-deploy-... \
  --commit <git-commit> \
  --run-id <target-release-run-id> \
  --recorded-at <ISO-8601 timestamp> \
  --proof-file /path/outside/git/deployment-proof.json \
  --output /path/outside/git/artifacts/<artifact-id>.json
EOF
  exit 0
fi

"$python_bin" "$impl" "$@"
