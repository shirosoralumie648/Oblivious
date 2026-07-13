#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
workdir="${OBLIVIOUS_TARGET_RELEASE_WORKDIR:-}"
env_file=""
prepare_only=false
dry_run=false

usage() {
  cat <<'EOF'
Usage: bash scripts/run-target-release-evidence.sh \
  --workdir /path/outside/git/oblivious-release \
  [--env-file /path/outside/git/oblivious-release/.env] \
  [--prepare-only] [--dry-run]

Runs the strict target release evidence workflow from an external workdir:

  1. assemble the target manifest from existing raw proof
  2. collect target artifact bodies from target URLs/Admin APIs
  3. refresh canonical manifest and artifact bundle digests
  4. verify the manifest and downloaded artifact bodies
  5. run target-evidence-only commercial preflight
  6. run the final no-skip commercial verifier unless --prepare-only is set

This script does not create proof, fill placeholders, upload artifacts, or
relax any verifier. Every raw proof file, target URI, SHA-256, database URL,
Kubernetes secret file, and target credential must already be supplied through
the external workdir and env file.

Options:
  --workdir       External release evidence workdir created by
                  init-target-release-evidence-workdir.sh.
  --env-file      Environment file to source. Defaults to <workdir>/.env.
  --prepare-only  Stop after target evidence verification and preflight.
                  This is not final commercial readiness.
  --dry-run       Validate the external inputs and print the ordered steps
                  without executing evidence collection or verification.
EOF
}

fail() {
  echo "[run-target-release-evidence] $*" >&2
  exit 1
}

resolved_existing_path() {
  local input_path="$1"
  local parent
  local base

  parent=$(dirname "$input_path")
  base=$(basename "$input_path")
  [[ -d "$parent" ]] || fail "parent directory does not exist: $parent"
  (cd "$parent" && printf '%s/%s\n' "$(pwd -P)" "$base")
}

is_inside() {
  local child="$1"
  local parent="$2"

  [[ "$child" == "$parent" || "$child" == "$parent"/* ]]
}

require_env() {
  local name="$1"

  if [[ -z "${!name:-}" ]]; then
    fail "required environment is missing: $name"
  fi
}

require_sha_env() {
  local name="$1"
  local value="${!name:-}"

  if [[ ! "$value" =~ ^[0-9a-fA-F]{64}$ ]]; then
    fail "$name must be a 64-character SHA-256 value"
  fi
}

require_file() {
  local file="$1"

  [[ -f "$file" ]] || fail "required raw proof file is missing: $file"
}

run_logged() {
  local label="$1"
  local log_file="$2"
  shift 2

  if [[ "$dry_run" == "true" ]]; then
    echo "[run-target-release-evidence] DRY-RUN $label"
    return
  fi

  echo "[run-target-release-evidence] START $label"
  "$@" 2>&1 | tee "$log_file"
  echo "[run-target-release-evidence] PASS  $label"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --workdir)
      workdir="${2:-}"
      shift 2
      ;;
    --env-file)
      env_file="${2:-}"
      shift 2
      ;;
    --prepare-only)
      prepare_only=true
      shift
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

[[ -n "$workdir" ]] || fail "--workdir or OBLIVIOUS_TARGET_RELEASE_WORKDIR is required"
[[ -d "$workdir" ]] || fail "workdir does not exist: $workdir"

repo_realpath=$(cd "$repo_root" && pwd -P)
workdir_realpath=$(cd "$workdir" && pwd -P)
if is_inside "$workdir_realpath" "$repo_realpath"; then
  fail "target release evidence workdir must be outside the repository: $workdir_realpath"
fi

if [[ -z "$env_file" ]]; then
  env_file="$workdir_realpath/.env"
fi
env_file=$(resolved_existing_path "$env_file")
[[ -f "$env_file" ]] || fail "env file does not exist: $env_file"
if ! is_inside "$env_file" "$workdir_realpath"; then
  fail "env file must live inside the external release workdir"
fi

if grep -Eiq '^[[:space:]]*(export[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*=.*(TODO|TBD|REPLACE_ME|CHANGE_ME|target\.example\.com|YYYY-MM-DD|replace-with)' "$env_file"; then
  fail "env file still contains placeholder values"
fi

set -a
# shellcheck disable=SC1090
source "$env_file"
set +a

export OBLIVIOUS_TARGET_RELEASE_WORKDIR="$workdir_realpath"
export OBLIVIOUS_TARGET_EVIDENCE_FILE="${OBLIVIOUS_TARGET_EVIDENCE_FILE:-$workdir_realpath/target-release-evidence.json}"
export OBLIVIOUS_TARGET_ARTIFACT_DIR="${OBLIVIOUS_TARGET_ARTIFACT_DIR:-$workdir_realpath/artifacts}"

manifest=$(resolved_existing_path "$OBLIVIOUS_TARGET_EVIDENCE_FILE")
artifact_dir=$(resolved_existing_path "$OBLIVIOUS_TARGET_ARTIFACT_DIR")
if ! is_inside "$manifest" "$workdir_realpath"; then
  fail "OBLIVIOUS_TARGET_EVIDENCE_FILE must live inside the external release workdir"
fi
if ! is_inside "$artifact_dir" "$workdir_realpath"; then
  fail "OBLIVIOUS_TARGET_ARTIFACT_DIR must live inside the external release workdir"
fi

raw_dir="$workdir_realpath/raw"
logs_dir="$workdir_realpath/logs"
mkdir -p "$artifact_dir" "$logs_dir"

if [[ "${COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS:-false}" == "true" ]]; then
  fail "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true is not allowed"
fi
if [[ "${OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH:-false}" == "true" ]]; then
  fail "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true is not allowed"
fi

required_env=(
  OBLIVIOUS_TARGET_EVIDENCE_RUN_ID
  OBLIVIOUS_TARGET_ENVIRONMENT_NAME
  OBLIVIOUS_TARGET_ENVIRONMENT_CLASS
  OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL
  OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT
  OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT
  OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE
  OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW
  OBLIVIOUS_TARGET_EVIDENCE_FROM
  OBLIVIOUS_TARGET_EVIDENCE_TO
  OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN
  OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_URI
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
)
for name in "${required_env[@]}"; do
  require_env "$name"
done

sha_env=(
  OBLIVIOUS_TARGET_STRICT_VERIFIER_SHA256
  OBLIVIOUS_TARGET_DEPLOYMENT_SHA256
  OBLIVIOUS_TARGET_KUBERNETES_SHA256
  OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256
  OBLIVIOUS_TARGET_ALIPAY_PROVIDER_SHA256
  OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_SHA256
  OBLIVIOUS_TARGET_GRPC_SMOKE_SHA256
  OBLIVIOUS_TARGET_SECRET_AUDIT_SHA256
  OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SHA256
  OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_SHA256
  OBLIVIOUS_TARGET_RAG_INDEXING_SHA256
  OBLIVIOUS_TARGET_RELAY_REALTIME_SHA256
  OBLIVIOUS_TARGET_RELAY_BATCH_SHA256
  OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_SHA256
  OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_SHA256
  OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_SHA256
  OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_SHA256
)
for name in "${sha_env[@]}"; do
  require_sha_env "$name"
done

raw_files=(
  strict-verifier.json
  deployment-proof.json
  kubernetes-proof.json
  stripe-provider-live-rail.json
  alipay-provider-live-rail.json
  wechatpay-provider-live-rail.json
  grpc-smoke.json
  secret-audit.json
  workflow-telemetry.json
  clickhouse-request-log-platform-proof.json
  usage-request-log-coverage.json
  latency-slo-proof.json
  rag-indexing-proof.json
  relay-realtime-proof.json
  relay-batch-proof.json
  marketplace-payout-proof.json
  marketplace-governance-proof.json
  provider-runtime-config-proof.json
  microservice-database-proof.json
)
for name in "${raw_files[@]}"; do
  require_file "$raw_dir/$name"
done

if [[ "$prepare_only" != "true" ]]; then
  require_env TEST_DATABASE_URL
  require_env OBLIVIOUS_K8S_SECRET_FILE
  k8s_secret=$(resolved_existing_path "$OBLIVIOUS_K8S_SECRET_FILE")
  require_file "$k8s_secret"
  if is_inside "$k8s_secret" "$repo_realpath"; then
    fail "OBLIVIOUS_K8S_SECRET_FILE must be outside the repository"
  fi
fi

assemble_command=(
  bash "$repo_root/scripts/assemble-target-release-evidence.sh"
  --grpc-smoke-file "$raw_dir/grpc-smoke.json"
  --strict-verifier-proof-file "$raw_dir/strict-verifier.json"
  --deployment-proof-file "$raw_dir/deployment-proof.json"
  --kubernetes-proof-file "$raw_dir/kubernetes-proof.json"
  --stripe-provider-live-rail-proof-file "$raw_dir/stripe-provider-live-rail.json"
  --alipay-provider-live-rail-proof-file "$raw_dir/alipay-provider-live-rail.json"
  --wechatpay-provider-live-rail-proof-file "$raw_dir/wechatpay-provider-live-rail.json"
  --secret-audit-proof-file "$raw_dir/secret-audit.json"
  --workflow-telemetry-proof-file "$raw_dir/workflow-telemetry.json"
  --request-log-platform-proof-file "$raw_dir/clickhouse-request-log-platform-proof.json"
  --request-log-coverage-file "$raw_dir/usage-request-log-coverage.json"
  --request-log-slo-file "$raw_dir/latency-slo-proof.json"
  --rag-proof-file "$raw_dir/rag-indexing-proof.json"
  --relay-realtime-proof-file "$raw_dir/relay-realtime-proof.json"
  --relay-batch-proof-file "$raw_dir/relay-batch-proof.json"
  --marketplace-payout-proof-file "$raw_dir/marketplace-payout-proof.json"
  --marketplace-governance-proof-file "$raw_dir/marketplace-governance-proof.json"
  --provider-runtime-config-proof-file "$raw_dir/provider-runtime-config-proof.json"
  --microservice-database-proof-file "$raw_dir/microservice-database-proof.json"
  --output "$manifest"
  --validate
)

collect_command=(
  bash "$repo_root/scripts/collect-target-release-artifacts.sh"
  --manifest "$manifest"
  --artifact-dir "$artifact_dir"
  --bearer-token-env OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN
  --strict-verifier-url "$OBLIVIOUS_TARGET_STRICT_VERIFIER_URI"
  --deployment-proof-url "$OBLIVIOUS_TARGET_DEPLOYMENT_URI"
  --kubernetes-proof-url "$OBLIVIOUS_TARGET_KUBERNETES_URI"
  --workflow-telemetry-url "$OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI"
  --request-log-platform-proof-url "$OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_URI"
  --target-base-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL"
  --coverage-query "from=$OBLIVIOUS_TARGET_EVIDENCE_FROM"
  --coverage-query "to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --slo-file "$raw_dir/latency-slo-proof.json"
  --rag-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/rag-indexing?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --relay-realtime-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/relay-realtime?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --relay-batch-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/relay-batch?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --marketplace-payout-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/marketplace-payout?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --marketplace-governance-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/marketplace-governance?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --provider-runtime-config-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/provider-runtime-config?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
  --stripe-provider-live-rail-url "$OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI"
  --alipay-provider-live-rail-url "$OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI"
  --wechatpay-provider-live-rail-url "$OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI"
  --grpc-smoke-url "$OBLIVIOUS_TARGET_GRPC_SMOKE_URI"
  --secret-audit-url "$OBLIVIOUS_TARGET_SECRET_AUDIT_URI"
  --microservice-database-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/microservice-database?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
)

digest_command=(
  bash "$repo_root/scripts/compute-target-release-digests.sh"
  --manifest "$manifest"
  --artifact-dir "$artifact_dir"
  --write
  --output "$workdir_realpath/target-release-digests.json"
)

verify_command=(
  bash "$repo_root/scripts/verify-target-release-evidence.sh"
  "$manifest"
)

preflight_command=(
  node "$repo_root/scripts/verify-commercial-preflight.mjs"
  --target-evidence-only
  --json-output "$logs_dir/target-evidence-preflight.json"
)

run_logged "assemble target evidence manifest" "$logs_dir/01-assemble-target-evidence.log" "${assemble_command[@]}"
run_logged "collect target artifact bodies" "$logs_dir/02-collect-target-artifacts.log" "${collect_command[@]}"
run_logged "refresh target release digests" "$logs_dir/03-target-release-digests.log" "${digest_command[@]}"
run_logged "verify target evidence bundle" "$logs_dir/04-verify-target-evidence.log" env OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" "${verify_command[@]}"
run_logged "target evidence preflight" "$logs_dir/05-target-evidence-preflight.log" \
  env COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
  OBLIVIOUS_TARGET_EVIDENCE_FILE="$manifest" \
  OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" \
  "${preflight_command[@]}"

if [[ "$prepare_only" == "true" ]]; then
  echo "[run-target-release-evidence] prepare-only completed; final commercial verifier was not run"
  exit 0
fi

run_logged "final no-skip commercial verifier" "$logs_dir/06-commercial-completion.log" \
  env COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
  COMMERCIAL_COMPLETION_RUN_K8S=true \
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
  OBLIVIOUS_TARGET_EVIDENCE_FILE="$manifest" \
  OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" \
  OBLIVIOUS_K8S_SECRET_FILE="$k8s_secret" \
  TEST_DATABASE_URL="$TEST_DATABASE_URL" \
  bash "$repo_root/scripts/verify-commercial-completion.sh"

echo "[run-target-release-evidence] final target release evidence workflow passed"
