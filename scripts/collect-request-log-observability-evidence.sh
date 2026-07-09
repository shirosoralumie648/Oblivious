#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/collect_request_log_observability_evidence.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: bash scripts/collect-request-log-observability-evidence.sh \
  --artifact-id artifact-request-log-observability-... \
  --commit <git-commit> \
  --run-id <target-release-run-id> \
  --recorded-at <ISO-8601 timestamp> \
  --platform-proof-file /path/outside/git/clickhouse-request-log-platform-proof.json \
  --coverage-file /path/outside/git/usage-request-log-coverage.json \
  --slo-file /path/outside/git/latency-slo-proof.json \
  --output /path/outside/git/artifacts/<artifact-id>.json

Or fetch coverage directly from the target Admin API:

OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN=... bash scripts/collect-request-log-observability-evidence.sh \
  --artifact-id artifact-request-log-observability-... \
  --commit <git-commit> \
  --run-id <target-release-run-id> \
  --recorded-at <ISO-8601 timestamp> \
  --platform-proof-url https://target.example.com/internal/release/clickhouse-request-log-platform-proof.json \
  --target-base-url https://target.example.com \
  --coverage-query limit=100 \
  --slo-url "https://target.example.com/api/v1/admin/observability/latency-slo-proof?from=2026-06-16T00:00:00Z&to=2026-06-16T01:00:00Z" \
  --output /path/outside/git/artifacts/<artifact-id>.json

Builds a request-log-observability artifact body from target ClickHouse
platform proof, target Admin coverage evidence, and latency SLO evidence.
The SLO JSON can be read from --slo-file or fetched from --slo-url, and must include a
start/end window, triggered alert count, successful alert delivery details, and recovery audit details. The output is intended for
OBLIVIOUS_TARGET_ARTIFACT_DIR validation by verify-target-release-evidence.sh.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

"$python_bin" "$impl" "$@"
