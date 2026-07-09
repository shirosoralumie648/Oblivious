#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/collect_target_release_artifacts.py"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: bash scripts/collect-target-release-artifacts.sh \
  --manifest /path/outside/git/target-release-evidence.json \
  --artifact-dir /path/outside/git/downloaded-artifacts \
  --strict-verifier-file /path/outside/git/strict-verifier.json \
  --deployment-proof-file /path/outside/git/deployment-proof.json \
  --kubernetes-proof-file /path/outside/git/kubernetes-proof.json \
  --workflow-telemetry-file /path/outside/git/workflow-telemetry.json \
  --request-log-platform-proof-file /path/outside/git/clickhouse-request-log-platform-proof.json \
  --coverage-file /path/outside/git/usage-request-log-coverage.json \
  --coverage-query limit=100 \
  --slo-file /path/outside/git/latency-slo-proof.json \
  --rag-proof-file /path/outside/git/rag-indexing-proof.json \
  --relay-realtime-proof-file /path/outside/git/relay-realtime-proof.json \
  --relay-batch-proof-file /path/outside/git/relay-batch-proof.json \
  --marketplace-payout-proof-file /path/outside/git/marketplace-payout-proof.json \
  --marketplace-governance-proof-file /path/outside/git/marketplace-governance-proof.json \
  --provider-runtime-config-proof-file /path/outside/git/provider-runtime-config-proof.json \
  --stripe-provider-live-rail-file /path/outside/git/stripe-provider-live-rail.json \
  --alipay-provider-live-rail-file /path/outside/git/alipay-provider-live-rail.json \
  --wechatpay-provider-live-rail-file /path/outside/git/wechatpay-provider-live-rail.json \
  --grpc-smoke-file /path/outside/git/grpc-smoke.json \
  --secret-audit-file /path/outside/git/secret-audit.json \
  --microservice-database-proof-file /path/outside/git/microservice-database-proof.json

Runs all target release artifact-body collectors for a filled target manifest,
writes each <artifact-id>.json body into --artifact-dir, and updates the
manifest artifact SHA-256 values to match the generated bodies.

Use exactly one of --workflow-telemetry-file or --workflow-telemetry-url for
workflow success telemetry. Use exactly one of --coverage-file, --coverage-url,
or --target-base-url for request-log coverage. Use exactly one of
--request-log-platform-proof-file or --request-log-platform-proof-url for
ClickHouse deployment/migration/table/ingest smoke proof. Use exactly one of
--slo-file or --slo-url for latency SLO trigger/delivery/recovery proof.
Use exactly one of each
--strict-verifier-file/--strict-verifier-url, --deployment-proof-file/--deployment-proof-url,
--kubernetes-proof-file/--kubernetes-proof-url, and --*-proof-file or
--*-proof-url pair for RAG, Relay, Marketplace, provider runtime config,
provider live rails, gRPC smoke, secret audit, and microservice database proofs. URL forms can use
--bearer-token-env, --bearer-token-file, --cookie-file, and --timeout-seconds,
and must not carry credentials or token/password/API-key style query
parameters.

URL proof examples include:
  --strict-verifier-url https://target.example.com/internal/release/strict-verifier.json
  --deployment-proof-url https://target.example.com/internal/release/deployment-proof.json
  --kubernetes-proof-url https://target.example.com/internal/release/kubernetes-proof.json
  --request-log-platform-proof-url https://target.example.com/internal/release/clickhouse-request-log-platform-proof.json
  --slo-url https://target.example.com/api/v1/admin/observability/latency-slo-proof?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --rag-proof-url https://target.example.com/api/v1/admin/release-evidence/rag-indexing?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --relay-realtime-proof-url https://target.example.com/api/v1/admin/release-evidence/relay-realtime?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --relay-batch-proof-url https://target.example.com/api/v1/admin/release-evidence/relay-batch?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --marketplace-payout-proof-url https://target.example.com/api/v1/admin/release-evidence/marketplace-payout?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --marketplace-governance-proof-url https://target.example.com/api/v1/admin/release-evidence/marketplace-governance?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --provider-runtime-config-proof-url https://target.example.com/api/v1/admin/release-evidence/provider-runtime-config?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
  --stripe-provider-live-rail-url https://target.example.com/internal/release/stripe-provider-live-rail.json
  --alipay-provider-live-rail-url https://target.example.com/internal/release/alipay-provider-live-rail.json
  --wechatpay-provider-live-rail-url https://target.example.com/internal/release/wechatpay-provider-live-rail.json
  --grpc-smoke-url https://target.example.com/internal/release/grpc-smoke.json
  --secret-audit-url https://target.example.com/internal/release/secret-audit.json
  --microservice-database-proof-url https://target.example.com/api/v1/admin/release-evidence/microservice-database?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

"$python_bin" "$impl" --repo-root "$repo_root" "$@"
