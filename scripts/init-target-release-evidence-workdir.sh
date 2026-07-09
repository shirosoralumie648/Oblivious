#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
workdir="${OBLIVIOUS_TARGET_RELEASE_WORKDIR:-}"
force=false

usage() {
  cat <<'EOF'
Usage: bash scripts/init-target-release-evidence-workdir.sh --workdir /path/outside/git/oblivious-release [--force]

Creates an external target release evidence workspace with raw/, artifacts/,
and logs/ subdirectories plus operator templates for the final no-skip
commercial release evidence flow.

This script does not create target evidence, proof JSON, manifests, artifact
bodies, secrets, or digests. It only creates a safe external workspace and
copyable collection checklist so the real target proof can be gathered
consistently.

Required:
  --workdir, or OBLIVIOUS_TARGET_RELEASE_WORKDIR
    Absolute or relative path outside this repository.

Optional:
  --force
    Allow writing templates into an existing external workspace.
EOF
}

fail() {
  echo "[init-target-release-evidence-workdir] $*" >&2
  exit 1
}

resolved_path() {
  local input_path="$1"
  local parent
  local base

  parent=$(dirname "$input_path")
  base=$(basename "$input_path")
  mkdir -p "$parent"
  (cd "$parent" && printf '%s/%s\n' "$(pwd -P)" "$base")
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --workdir)
      workdir="${2:-}"
      shift 2
      ;;
    --force)
      force=true
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

if [[ -z "$workdir" ]]; then
  fail "--workdir or OBLIVIOUS_TARGET_RELEASE_WORKDIR is required"
fi

repo_realpath=$(cd "$repo_root" && pwd -P)
workdir_realpath=$(resolved_path "$workdir")

case "$workdir_realpath" in
  "$repo_realpath"|"$repo_realpath"/*)
    fail "target release evidence workdir must be outside the repository: $workdir_realpath"
    ;;
esac

if [[ -e "$workdir_realpath" && "$force" != "true" ]]; then
  if [[ -n "$(find "$workdir_realpath" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
    fail "workdir already exists and is not empty; pass --force to add templates: $workdir_realpath"
  fi
fi

mkdir -p "$workdir_realpath/raw" "$workdir_realpath/artifacts" "$workdir_realpath/logs"

cat > "$workdir_realpath/.env.example" <<EOF
# Target release evidence workspace for Oblivious.
# Copy to .env and replace every value with real target values before use.
# Do not commit this workspace or put it inside the repository.

export OBLIVIOUS_TARGET_RELEASE_WORKDIR="$workdir_realpath"
export OBLIVIOUS_TARGET_EVIDENCE_RUN_ID="release-YYYYMMDD-HHMMSS"
export OBLIVIOUS_TARGET_ENVIRONMENT_NAME="production"
export OBLIVIOUS_TARGET_ENVIRONMENT_CLASS="production"
export OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL="https://target.example.com"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT="YYYY-MM-DDTHH:MM:SSZ"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT="YYYY-MM-DDTHH:MM:SSZ"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE="0.99"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW="YYYY-MM-DDTHH:MM:SSZ/YYYY-MM-DDTHH:MM:SSZ"
export OBLIVIOUS_TARGET_EVIDENCE_FROM="YYYY-MM-DDTHH:MM:SSZ"
export OBLIVIOUS_TARGET_EVIDENCE_TO="YYYY-MM-DDTHH:MM:SSZ"

export AGENT_GRPC_ADDR="agent.target.example.com:50063"
export WORKFLOW_GRPC_ADDR="workflow.target.example.com:50064"
export TASK_GRPC_ADDR="task.target.example.com:50065"

export OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN="replace-with-secret-outside-git"

export OBLIVIOUS_TARGET_STRICT_VERIFIER_URI="https://target.example.com/internal/release/strict-verifier.json"
export OBLIVIOUS_TARGET_DEPLOYMENT_URI="https://target.example.com/internal/release/deployment-proof.json"
export OBLIVIOUS_TARGET_KUBERNETES_URI="https://target.example.com/internal/release/kubernetes-proof.json"
export OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI="https://target.example.com/internal/release/stripe-provider-live-rail.json"
export OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI="https://target.example.com/internal/release/alipay-provider-live-rail.json"
export OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI="https://target.example.com/internal/release/wechatpay-provider-live-rail.json"
export OBLIVIOUS_TARGET_GRPC_SMOKE_URI="https://target.example.com/internal/release/grpc-smoke.json"
export OBLIVIOUS_TARGET_SECRET_AUDIT_URI="https://target.example.com/internal/release/secret-audit.json"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI="https://target.example.com/internal/release/workflow-telemetry.json"
export OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_URI="https://target.example.com/internal/release/clickhouse-request-log-platform-proof.json"
export OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_URI="https://target.example.com/internal/release/request-log-observability.json"
export OBLIVIOUS_TARGET_RAG_INDEXING_URI="https://target.example.com/internal/release/rag-indexing-proof.json"
export OBLIVIOUS_TARGET_RELAY_REALTIME_URI="https://target.example.com/internal/release/relay-realtime-proof.json"
export OBLIVIOUS_TARGET_RELAY_BATCH_URI="https://target.example.com/internal/release/relay-batch-proof.json"
export OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_URI="https://target.example.com/internal/release/marketplace-payout-proof.json"
export OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI="https://target.example.com/internal/release/marketplace-governance-proof.json"
export OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_URI="https://target.example.com/internal/release/provider-runtime-config-proof.json"
export OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_URI="https://target.example.com/internal/release/microservice-database-proof.json"

# Fill these after collecting target artifact bodies, or run the aggregate
# collector plus compute-target-release-digests.sh --write to refresh them.
export OBLIVIOUS_TARGET_STRICT_VERIFIER_SHA256=""
export OBLIVIOUS_TARGET_DEPLOYMENT_SHA256=""
export OBLIVIOUS_TARGET_KUBERNETES_SHA256=""
export OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256=""
export OBLIVIOUS_TARGET_ALIPAY_PROVIDER_SHA256=""
export OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_SHA256=""
export OBLIVIOUS_TARGET_GRPC_SMOKE_SHA256=""
export OBLIVIOUS_TARGET_SECRET_AUDIT_SHA256=""
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SHA256=""
export OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_SHA256=""
export OBLIVIOUS_TARGET_RAG_INDEXING_SHA256=""
export OBLIVIOUS_TARGET_RELAY_REALTIME_SHA256=""
export OBLIVIOUS_TARGET_RELAY_BATCH_SHA256=""
export OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_SHA256=""
export OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_SHA256=""
export OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_SHA256=""
export OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_SHA256=""

export OBLIVIOUS_TARGET_EVIDENCE_FILE="$workdir_realpath/target-release-evidence.json"
export OBLIVIOUS_TARGET_ARTIFACT_DIR="$workdir_realpath/artifacts"
EOF

cat > "$workdir_realpath/collect-target-evidence.todo.md" <<'EOF'
# Target Release Evidence Checklist

This workspace is for real target evidence only. Keep it outside git and do
not replace target proof with fixtures, examples, localhost output, or checked-in
files.

## 1. Raw Target Proof

- [ ] Capture gRPC generated-client smoke:

```bash
AGENT_GRPC_ADDR="$AGENT_GRPC_ADDR" \
WORKFLOW_GRPC_ADDR="$WORKFLOW_GRPC_ADDR" \
TASK_GRPC_ADDR="$TASK_GRPC_ADDR" \
bash scripts/target-grpc-smoke.sh > "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/grpc-smoke.json"
```

- [ ] Store these raw proof files under `raw/`:

```text
strict-verifier.json
deployment-proof.json
kubernetes-proof.json
stripe-provider-live-rail.json
alipay-provider-live-rail.json
wechatpay-provider-live-rail.json
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
```

## 2. Assemble Manifest

```bash
bash scripts/assemble-target-release-evidence.sh \
  --grpc-smoke-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/grpc-smoke.json" \
  --strict-verifier-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/strict-verifier.json" \
  --deployment-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/deployment-proof.json" \
  --kubernetes-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/kubernetes-proof.json" \
  --stripe-provider-live-rail-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/stripe-provider-live-rail.json" \
  --alipay-provider-live-rail-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/alipay-provider-live-rail.json" \
  --wechatpay-provider-live-rail-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/wechatpay-provider-live-rail.json" \
  --secret-audit-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/secret-audit.json" \
  --workflow-telemetry-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/workflow-telemetry.json" \
  --request-log-platform-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/clickhouse-request-log-platform-proof.json" \
  --request-log-coverage-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/usage-request-log-coverage.json" \
  --request-log-slo-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/latency-slo-proof.json" \
  --rag-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/rag-indexing-proof.json" \
  --relay-realtime-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/relay-realtime-proof.json" \
  --relay-batch-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/relay-batch-proof.json" \
  --marketplace-payout-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/marketplace-payout-proof.json" \
  --marketplace-governance-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/marketplace-governance-proof.json" \
  --provider-runtime-config-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/provider-runtime-config-proof.json" \
  --microservice-database-proof-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/microservice-database-proof.json" \
  --output "$OBLIVIOUS_TARGET_EVIDENCE_FILE" \
  --validate
```

## 3. Collect Artifact Bodies

Use target URLs or protected Admin APIs for final evidence. File collection is
only for local fallback inspection and is not final commercial readiness proof.

```bash
bash scripts/collect-target-release-artifacts.sh \
  --manifest "$OBLIVIOUS_TARGET_EVIDENCE_FILE" \
  --artifact-dir "$OBLIVIOUS_TARGET_ARTIFACT_DIR" \
  --bearer-token-env OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN \
  --strict-verifier-url "$OBLIVIOUS_TARGET_STRICT_VERIFIER_URI" \
  --deployment-proof-url "$OBLIVIOUS_TARGET_DEPLOYMENT_URI" \
  --kubernetes-proof-url "$OBLIVIOUS_TARGET_KUBERNETES_URI" \
  --workflow-telemetry-url "$OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI" \
  --request-log-platform-proof-url "$OBLIVIOUS_TARGET_REQUEST_LOG_PLATFORM_PROOF_URI" \
  --target-base-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL" \
  --coverage-query "from=$OBLIVIOUS_TARGET_EVIDENCE_FROM" \
  --coverage-query "to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --slo-file "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/raw/latency-slo-proof.json" \
  --rag-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/rag-indexing?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --relay-realtime-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/relay-realtime?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --relay-batch-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/relay-batch?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --marketplace-payout-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/marketplace-payout?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --marketplace-governance-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/marketplace-governance?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --provider-runtime-config-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/provider-runtime-config?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO" \
  --stripe-provider-live-rail-url "$OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI" \
  --alipay-provider-live-rail-url "$OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI" \
  --wechatpay-provider-live-rail-url "$OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI" \
  --grpc-smoke-url "$OBLIVIOUS_TARGET_GRPC_SMOKE_URI" \
  --secret-audit-url "$OBLIVIOUS_TARGET_SECRET_AUDIT_URI" \
  --microservice-database-proof-url "$OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL/api/v1/admin/release-evidence/microservice-database?from=$OBLIVIOUS_TARGET_EVIDENCE_FROM&to=$OBLIVIOUS_TARGET_EVIDENCE_TO"
```

## 4. Digest And Final Verification

```bash
bash scripts/compute-target-release-digests.sh \
  --manifest "$OBLIVIOUS_TARGET_EVIDENCE_FILE" \
  --artifact-dir "$OBLIVIOUS_TARGET_ARTIFACT_DIR" \
  --write \
  --output "$OBLIVIOUS_TARGET_RELEASE_WORKDIR/target-release-digests.json"

OBLIVIOUS_TARGET_ARTIFACT_DIR="$OBLIVIOUS_TARGET_ARTIFACT_DIR" \
bash scripts/verify-target-release-evidence.sh "$OBLIVIOUS_TARGET_EVIDENCE_FILE"
```

Then run the no-skip `scripts/verify-commercial-completion.sh` command from
`docs/release/rc-checklist.md` with the same external manifest and artifact dir.
EOF

cat > "$workdir_realpath/README.md" <<EOF
# Oblivious Target Release Evidence Workspace

This directory was initialized by:

\`\`\`bash
bash scripts/init-target-release-evidence-workdir.sh --workdir "$workdir_realpath"
\`\`\`

Directory layout:

- \`raw/\`: raw target proof JSON captured from protected target systems.
- \`artifacts/\`: downloaded target artifact bodies named \`<artifact-id>.json\`.
- \`logs/\`: terminal logs for the final strict commercial verifier and related commands.
- \`.env.example\`: environment template with placeholders that must be replaced.
- \`collect-target-evidence.todo.md\`: ordered collection checklist.

This workspace is intentionally outside git. It is not evidence by itself; final
commercial readiness still requires real target proof, digest refresh, and a
no-skip \`scripts/verify-commercial-completion.sh\` pass.
EOF

echo "[init-target-release-evidence-workdir] initialized $workdir_realpath"
echo "[init-target-release-evidence-workdir] next: copy $workdir_realpath/.env.example to $workdir_realpath/.env and replace placeholders with target values"
