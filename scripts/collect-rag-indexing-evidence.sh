#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/collect_rag_indexing_evidence.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: bash scripts/collect-rag-indexing-evidence.sh \
  --artifact-id artifact-rag-indexing-... \
  --commit <git-commit> \
  --run-id <target-release-run-id> \
  --recorded-at <ISO-8601 timestamp> \
  --proof-file /path/outside/git/rag-indexing-proof.json \
  --output /path/outside/git/artifacts/<artifact-id>.json

Or fetch the proof JSON from a protected target evidence URL:

OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN=... bash scripts/collect-rag-indexing-evidence.sh \
  --artifact-id artifact-rag-indexing-... \
  --commit <git-commit> \
  --run-id <target-release-run-id> \
  --recorded-at <ISO-8601 timestamp> \
  --proof-url https://target.example.com/api/v1/admin/release-evidence/rag-indexing \
  --output /path/outside/git/artifacts/<artifact-id>.json

Builds a rag-indexing-proof artifact body from target RAG indexing evidence.
The output is intended for OBLIVIOUS_TARGET_ARTIFACT_DIR validation by
verify-target-release-evidence.sh. Authentication must be supplied through
headers, environment, or cookie files; proof URLs must not embed credentials or
token/password/API-key style query parameters.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

"$python_bin" "$impl" "$@"
