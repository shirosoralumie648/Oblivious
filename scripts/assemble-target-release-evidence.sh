#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"
impl="$repo_root/scripts/assemble_target_release_evidence.py"
grpc_smoke_file="${OBLIVIOUS_TARGET_GRPC_SMOKE_FILE:-}"
strict_verifier_proof_file="${OBLIVIOUS_TARGET_STRICT_VERIFIER_PROOF_FILE:-}"
deployment_proof_file="${OBLIVIOUS_TARGET_DEPLOYMENT_PROOF_FILE:-}"
kubernetes_proof_file="${OBLIVIOUS_TARGET_KUBERNETES_PROOF_FILE:-}"
stripe_provider_live_rail_proof_file="${OBLIVIOUS_TARGET_STRIPE_PROVIDER_LIVE_RAIL_PROOF_FILE:-}"
alipay_provider_live_rail_proof_file="${OBLIVIOUS_TARGET_ALIPAY_PROVIDER_LIVE_RAIL_PROOF_FILE:-}"
wechatpay_provider_live_rail_proof_file="${OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_LIVE_RAIL_PROOF_FILE:-}"
secret_audit_proof_file="${OBLIVIOUS_TARGET_SECRET_AUDIT_PROOF_FILE:-}"
workflow_telemetry_proof_file="${OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_PROOF_FILE:-}"
request_log_platform_proof_file="${OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_FILE:-}"
request_log_coverage_file="${OBLIVIOUS_TARGET_REQUEST_LOG_COVERAGE_FILE:-}"
request_log_slo_file="${OBLIVIOUS_TARGET_REQUEST_LOG_SLO_FILE:-}"
rag_proof_file="${OBLIVIOUS_TARGET_RAG_PROOF_FILE:-}"
relay_realtime_proof_file="${OBLIVIOUS_TARGET_RELAY_REALTIME_PROOF_FILE:-}"
relay_batch_proof_file="${OBLIVIOUS_TARGET_RELAY_BATCH_PROOF_FILE:-}"
marketplace_payout_proof_file="${OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_PROOF_FILE:-}"
marketplace_governance_proof_file="${OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_PROOF_FILE:-}"
provider_runtime_config_proof_file="${OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_PROOF_FILE:-}"
microservice_database_proof_file="${OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_PROOF_FILE:-}"
output_file="${OBLIVIOUS_TARGET_EVIDENCE_OUTPUT:-}"
artifact_dir="${OBLIVIOUS_TARGET_ARTIFACT_DIR:-}"
validate=false

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

usage() {
  cat <<'EOF'
Usage: bash scripts/assemble-target-release-evidence.sh \
  --grpc-smoke-file /path/outside/git/grpc-smoke.json \
  --strict-verifier-proof-file /path/outside/git/strict-verifier.json \
  --deployment-proof-file /path/outside/git/deployment-proof.json \
  --kubernetes-proof-file /path/outside/git/kubernetes-proof.json \
  --stripe-provider-live-rail-proof-file /path/outside/git/stripe-provider-live-rail.json \
  --alipay-provider-live-rail-proof-file /path/outside/git/alipay-provider-live-rail.json \
  --wechatpay-provider-live-rail-proof-file /path/outside/git/wechatpay-provider-live-rail.json \
  --secret-audit-proof-file /path/outside/git/secret-audit.json \
  --workflow-telemetry-proof-file /path/outside/git/workflow-telemetry.json \
  --request-log-platform-proof-file /path/outside/git/clickhouse-request-log-platform-proof.json \
  --request-log-coverage-file /path/outside/git/usage-request-log-coverage.json \
  --request-log-slo-file /path/outside/git/latency-slo-proof.json \
  --rag-proof-file /path/outside/git/rag-indexing-proof.json \
  --relay-realtime-proof-file /path/outside/git/relay-realtime-proof.json \
  --relay-batch-proof-file /path/outside/git/relay-batch-proof.json \
  --marketplace-payout-proof-file /path/outside/git/marketplace-payout-proof.json \
  --marketplace-governance-proof-file /path/outside/git/marketplace-governance-proof.json \
  --provider-runtime-config-proof-file /path/outside/git/provider-runtime-config-proof.json \
  --microservice-database-proof-file /path/outside/git/microservice-database-proof.json \
  --output /path/outside/git/target-release-evidence.json \
  [--artifact-dir /path/outside/git/downloaded-artifacts] \
  [--validate]

Assembles a target/live release evidence manifest from concrete target-run inputs.
This script does not create evidence or accept placeholders; it only wires target
artifact URIs, timestamps, telemetry, and gRPC smoke JSON into the strict
manifest schema.

Required environment:
  OBLIVIOUS_TARGET_EVIDENCE_RUN_ID
  OBLIVIOUS_TARGET_ENVIRONMENT_NAME
  OBLIVIOUS_TARGET_ENVIRONMENT_CLASS (staging, preproduction, or production)
  OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL
  OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT
  OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT
  OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE
  OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW

Required deployment/provider proof files, as arguments or environment:
  --strict-verifier-proof-file or OBLIVIOUS_TARGET_STRICT_VERIFIER_PROOF_FILE
  --deployment-proof-file or OBLIVIOUS_TARGET_DEPLOYMENT_PROOF_FILE
  --kubernetes-proof-file or OBLIVIOUS_TARGET_KUBERNETES_PROOF_FILE
  --stripe-provider-live-rail-proof-file or OBLIVIOUS_TARGET_STRIPE_PROVIDER_LIVE_RAIL_PROOF_FILE
  --alipay-provider-live-rail-proof-file or OBLIVIOUS_TARGET_ALIPAY_PROVIDER_LIVE_RAIL_PROOF_FILE
  --wechatpay-provider-live-rail-proof-file or OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_LIVE_RAIL_PROOF_FILE
  --secret-audit-proof-file or OBLIVIOUS_TARGET_SECRET_AUDIT_PROOF_FILE
  --workflow-telemetry-proof-file or OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_PROOF_FILE

Required request-log proof files, as arguments or environment:
  --request-log-platform-proof-file or OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_FILE
  --request-log-coverage-file or OBLIVIOUS_TARGET_REQUEST_LOG_COVERAGE_FILE
  --request-log-slo-file or OBLIVIOUS_TARGET_REQUEST_LOG_SLO_FILE

Required RAG proof file, as argument or environment:
  --rag-proof-file or OBLIVIOUS_TARGET_RAG_PROOF_FILE

Required Relay proof files, as arguments or environment:
  --relay-realtime-proof-file or OBLIVIOUS_TARGET_RELAY_REALTIME_PROOF_FILE
  --relay-batch-proof-file or OBLIVIOUS_TARGET_RELAY_BATCH_PROOF_FILE

Required Marketplace proof files, as arguments or environment:
  --marketplace-payout-proof-file or OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_PROOF_FILE
  --marketplace-governance-proof-file or OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_PROOF_FILE

Required provider/runtime proof files, as arguments or environment:
  --provider-runtime-config-proof-file or OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_PROOF_FILE
  --microservice-database-proof-file or OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_PROOF_FILE

Required artifact URI environment:
  OBLIVIOUS_TARGET_STRICT_VERIFIER_URI
  OBLIVIOUS_TARGET_DEPLOYMENT_URI
  OBLIVIOUS_TARGET_KUBERNETES_URI
  OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI
  OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI
  OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI
  OBLIVIOUS_TARGET_GRPC_SMOKE_URI
  OBLIVIOUS_TARGET_SECRET_AUDIT_URI
  OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI
  OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_URI
  OBLIVIOUS_TARGET_RAG_INDEXING_URI
  OBLIVIOUS_TARGET_RELAY_REALTIME_URI
  OBLIVIOUS_TARGET_RELAY_BATCH_URI
  OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_URI
  OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI
  OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_URI
  OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_URI

Every artifact URI environment above must also provide the matching SHA-256
environment with `_URI` replaced by `_SHA256`, for example:
  OBLIVIOUS_TARGET_STRICT_VERIFIER_SHA256

Optional strict artifact-body validation:
  --artifact-dir or OBLIVIOUS_TARGET_ARTIFACT_DIR
    When used with --validate, also validate downloaded artifact bodies named
    <artifact-id>.json against manifest SHA-256 values, required body fields,
    collection source contracts, and canonical target release digest fields.
    When --validate is used without --artifact-dir, the assembler performs only
    non-final manifest-level validation through verify-target-release-evidence.sh
    --manifest-only.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --grpc-smoke-file)
      grpc_smoke_file="${2:-}"
      shift 2
      ;;
    --strict-verifier-proof-file)
      strict_verifier_proof_file="${2:-}"
      shift 2
      ;;
    --deployment-proof-file)
      deployment_proof_file="${2:-}"
      shift 2
      ;;
    --kubernetes-proof-file)
      kubernetes_proof_file="${2:-}"
      shift 2
      ;;
    --stripe-provider-live-rail-proof-file)
      stripe_provider_live_rail_proof_file="${2:-}"
      shift 2
      ;;
    --alipay-provider-live-rail-proof-file)
      alipay_provider_live_rail_proof_file="${2:-}"
      shift 2
      ;;
    --wechatpay-provider-live-rail-proof-file)
      wechatpay_provider_live_rail_proof_file="${2:-}"
      shift 2
      ;;
    --secret-audit-proof-file)
      secret_audit_proof_file="${2:-}"
      shift 2
      ;;
    --workflow-telemetry-proof-file)
      workflow_telemetry_proof_file="${2:-}"
      shift 2
      ;;
    --request-log-platform-proof-file)
      request_log_platform_proof_file="${2:-}"
      shift 2
      ;;
    --request-log-coverage-file)
      request_log_coverage_file="${2:-}"
      shift 2
      ;;
    --request-log-slo-file)
      request_log_slo_file="${2:-}"
      shift 2
      ;;
    --rag-proof-file)
      rag_proof_file="${2:-}"
      shift 2
      ;;
    --relay-realtime-proof-file)
      relay_realtime_proof_file="${2:-}"
      shift 2
      ;;
    --relay-batch-proof-file)
      relay_batch_proof_file="${2:-}"
      shift 2
      ;;
    --marketplace-payout-proof-file)
      marketplace_payout_proof_file="${2:-}"
      shift 2
      ;;
    --marketplace-governance-proof-file)
      marketplace_governance_proof_file="${2:-}"
      shift 2
      ;;
    --provider-runtime-config-proof-file)
      provider_runtime_config_proof_file="${2:-}"
      shift 2
      ;;
    --microservice-database-proof-file)
      microservice_database_proof_file="${2:-}"
      shift 2
      ;;
    --output)
      output_file="${2:-}"
      shift 2
      ;;
    --artifact-dir)
      artifact_dir="${2:-}"
      shift 2
      ;;
    --validate)
      validate=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "[assemble-target-release-evidence] unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$grpc_smoke_file" ]]; then
  echo "[assemble-target-release-evidence] --grpc-smoke-file or OBLIVIOUS_TARGET_GRPC_SMOKE_FILE is required" >&2
  exit 1
fi
if [[ -z "$strict_verifier_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --strict-verifier-proof-file or OBLIVIOUS_TARGET_STRICT_VERIFIER_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$deployment_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --deployment-proof-file or OBLIVIOUS_TARGET_DEPLOYMENT_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$kubernetes_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --kubernetes-proof-file or OBLIVIOUS_TARGET_KUBERNETES_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$stripe_provider_live_rail_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --stripe-provider-live-rail-proof-file or OBLIVIOUS_TARGET_STRIPE_PROVIDER_LIVE_RAIL_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$alipay_provider_live_rail_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --alipay-provider-live-rail-proof-file or OBLIVIOUS_TARGET_ALIPAY_PROVIDER_LIVE_RAIL_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$wechatpay_provider_live_rail_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --wechatpay-provider-live-rail-proof-file or OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_LIVE_RAIL_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$secret_audit_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --secret-audit-proof-file or OBLIVIOUS_TARGET_SECRET_AUDIT_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$workflow_telemetry_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --workflow-telemetry-proof-file or OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$request_log_platform_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --request-log-platform-proof-file or OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$request_log_coverage_file" ]]; then
  echo "[assemble-target-release-evidence] --request-log-coverage-file or OBLIVIOUS_TARGET_REQUEST_LOG_COVERAGE_FILE is required" >&2
  exit 1
fi
if [[ -z "$request_log_slo_file" ]]; then
  echo "[assemble-target-release-evidence] --request-log-slo-file or OBLIVIOUS_TARGET_REQUEST_LOG_SLO_FILE is required" >&2
  exit 1
fi
if [[ -z "$rag_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --rag-proof-file or OBLIVIOUS_TARGET_RAG_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$relay_realtime_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --relay-realtime-proof-file or OBLIVIOUS_TARGET_RELAY_REALTIME_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$relay_batch_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --relay-batch-proof-file or OBLIVIOUS_TARGET_RELAY_BATCH_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$marketplace_payout_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --marketplace-payout-proof-file or OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$marketplace_governance_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --marketplace-governance-proof-file or OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$provider_runtime_config_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --provider-runtime-config-proof-file or OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$microservice_database_proof_file" ]]; then
  echo "[assemble-target-release-evidence] --microservice-database-proof-file or OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_PROOF_FILE is required" >&2
  exit 1
fi
if [[ -z "$output_file" ]]; then
  echo "[assemble-target-release-evidence] --output or OBLIVIOUS_TARGET_EVIDENCE_OUTPUT is required" >&2
  exit 1
fi

current_commit=$(git -C "$repo_root" rev-parse HEAD)
"$python_bin" "$impl" \
  --current-commit "$current_commit" \
  --grpc-smoke-file "$grpc_smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$request_log_platform_proof_file" \
  --request-log-coverage-file "$request_log_coverage_file" \
  --request-log-slo-file "$request_log_slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$marketplace_payout_proof_file" \
  --marketplace-governance-proof-file "$marketplace_governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$output_file"

if [[ "$validate" == "true" ]]; then
  if [[ -n "$artifact_dir" ]]; then
    OBLIVIOUS_TARGET_EVIDENCE_FILE="$output_file" OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$repo_root/scripts/verify-target-release-evidence.sh"
  else
    OBLIVIOUS_TARGET_EVIDENCE_FILE="$output_file" bash "$repo_root/scripts/verify-target-release-evidence.sh" --manifest-only
  fi
fi
