#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
assembler="$repo_root/scripts/assemble-target-release-evidence.sh"
request_log_collector="$repo_root/scripts/collect-request-log-observability-evidence.sh"
rag_collector="$repo_root/scripts/collect-rag-indexing-evidence.sh"
relay_realtime_collector="$repo_root/scripts/collect-relay-realtime-evidence.sh"
relay_batch_collector="$repo_root/scripts/collect-relay-batch-evidence.sh"
payout_collector="$repo_root/scripts/collect-marketplace-payout-evidence.sh"
governance_collector="$repo_root/scripts/collect-marketplace-governance-evidence.sh"
provider_runtime_config_collector="$repo_root/scripts/collect-provider-runtime-config-evidence.sh"
microservice_database_collector="$repo_root/scripts/collect-microservice-database-evidence.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[assemble-target-release-evidence-fixtures] $*" >&2
  exit 1
}

add_microservice_services() {
  "$python_bin" - "$1" <<'PY'
import json
import pathlib
import sys

services = [
    "relay",
    "chat",
    "workflow",
    "rag",
    "agent",
    "billing",
    "marketplace",
    "admin",
    "channel",
    "task",
    "observability",
]
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["services"] = [
    {
        "name": service,
        "databaseUrlClass": "external-filled",
        "migrationReadiness": "pass",
        "evidenceId": f"microservice_database_{service}_20260701",
    }
    for service in services
]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

smoke_file="$tmpdir/grpc-smoke.json"
manifest_file="$tmpdir/target-release-evidence.json"
artifact_dir="$tmpdir/artifacts"
strict_verifier_proof_file="$tmpdir/strict-verifier.json"
deployment_proof_file="$tmpdir/deployment-proof.json"
kubernetes_proof_file="$tmpdir/kubernetes-proof.json"
current_commit=$(git -C "$repo_root" rev-parse HEAD)
stripe_provider_live_rail_proof_file="$tmpdir/stripe-provider-live-rail.json"
alipay_provider_live_rail_proof_file="$tmpdir/alipay-provider-live-rail.json"
wechatpay_provider_live_rail_proof_file="$tmpdir/wechatpay-provider-live-rail.json"
secret_audit_proof_file="$tmpdir/secret-audit.json"
workflow_telemetry_proof_file="$tmpdir/workflow-telemetry.json"
coverage_file="$tmpdir/usage-request-log-coverage.json"
platform_proof_file="$tmpdir/clickhouse-request-log-platform-proof.json"
slo_file="$tmpdir/latency-slo-proof.json"
rag_proof_file="$tmpdir/rag-indexing-proof.json"
relay_realtime_proof_file="$tmpdir/relay-realtime-proof.json"
relay_batch_proof_file="$tmpdir/relay-batch-proof.json"
payout_proof_file="$tmpdir/marketplace-payout-proof.json"
governance_proof_file="$tmpdir/marketplace-governance-proof.json"
provider_runtime_config_proof_file="$tmpdir/provider-runtime-config-proof.json"
microservice_database_proof_file="$tmpdir/microservice-database-proof.json"
cat >"$smoke_file" <<'JSON'
{
  "recordedAt": "2026-07-01T00:10:00Z",
  "timeout": "10s",
  "results": [
    {"service": "agent", "address": "agent.prod.oblivious.release.test:50063", "generatedClient": "pass", "status": "validation_error"},
    {"service": "workflow", "address": "workflow.prod.oblivious.release.test:50064", "generatedClient": "pass", "status": "validation_response"},
    {"service": "task", "address": "task.prod.oblivious.release.test:50065", "generatedClient": "pass", "status": "validation_response"}
  ]
}
JSON

cat >"$strict_verifier_proof_file" <<JSON
{
  "artifactBundleSha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "command": "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
  "commit": "$current_commit",
  "result": "pass",
  "runId": "target-run-20260701",
  "skippedChecks": [],
  "startedAt": "2026-07-01T00:00:00Z",
  "completedAt": "2026-07-01T00:30:00Z",
  "targetEvidenceSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
JSON
cat >"$deployment_proof_file" <<'JSON'
{
  "result": "pass",
  "targetEnvironment": "production",
  "deployValidation": "pass",
  "backupRestore": "pass",
  "migrationReplay": "pass",
  "references": {
    "deployValidation": "deploy-validation-run-20260701",
    "backupRestore": "backup-restore-run-20260701",
    "migrationReplay": "migration-replay-run-20260701"
  }
}
JSON
cat >"$kubernetes_proof_file" <<'JSON'
{
  "result": "pass",
  "targetEnvironment": "production",
  "clusterRef": "prod-cluster-20260701",
  "namespace": "oblivious",
  "validation": "pass",
  "rollout": "pass",
  "failover": "pass",
  "secretFileClass": "external-filled",
  "references": {
    "validation": "k8s-validation-run-20260701",
    "rollout": "k8s-rollout-run-20260701",
    "failover": "k8s-failover-run-20260701"
  }
}
JSON
cat >"$stripe_provider_live_rail_proof_file" <<'JSON'
{
  "provider": "stripe",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "stripe_checkout_live_20260616",
    "refund": "stripe_refund_live_20260616",
    "payout": "stripe_payout_live_20260616",
    "reconciliation": "stripe_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 2,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 2
  }
}
JSON
cat >"$alipay_provider_live_rail_proof_file" <<'JSON'
{
  "provider": "alipay",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "alipay_checkout_live_20260616",
    "refund": "alipay_refund_live_20260616",
    "payout": "alipay_payout_live_20260616",
    "reconciliation": "alipay_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 2,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 2
  }
}
JSON
cat >"$wechatpay_provider_live_rail_proof_file" <<'JSON'
{
  "provider": "wechatpay",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "wechatpay_checkout_live_20260616",
    "refund": "wechatpay_refund_live_20260616",
    "payout": "wechatpay_payout_live_20260616",
    "reconciliation": "wechatpay_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 2,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 2
  }
}
JSON
cat >"$secret_audit_proof_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-07-01T00:25:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 42,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 0
  }
}
JSON
cat >"$workflow_telemetry_proof_file" <<'JSON'
{
  "result": "pass",
  "telemetry": {
    "successRate": 0.995,
    "window": "2026-07-01T00:00:00Z/2026-07-01T00:20:00Z",
    "totalExecutions": 200,
    "successfulExecutions": 199,
    "failedExecutions": 1
  }
}
JSON

export OBLIVIOUS_TARGET_EVIDENCE_RUN_ID="target-run-20260701"
export OBLIVIOUS_TARGET_ENVIRONMENT_NAME="production-blue"
export OBLIVIOUS_TARGET_ENVIRONMENT_CLASS="production"
export OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL="https://app.oblivious.release.test"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT="2026-07-01T00:00:00Z"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT="2026-07-01T00:30:00Z"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE="0.995"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW="2026-07-01T00:00:00Z/2026-07-01T00:20:00Z"

artifact_base="https://release-artifacts.oblivious.release.test/target-run-20260701"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_URI="$artifact_base/strict-verifier.log"
export OBLIVIOUS_TARGET_DEPLOYMENT_URI="$artifact_base/deployment.log"
export OBLIVIOUS_TARGET_KUBERNETES_URI="$artifact_base/kubernetes.log"
export OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI="$artifact_base/stripe-provider-live-rail.json"
export OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI="$artifact_base/alipay-provider-live-rail.json"
export OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI="$artifact_base/wechatpay-provider-live-rail.json"
export OBLIVIOUS_TARGET_GRPC_SMOKE_URI="$artifact_base/grpc-smoke.json"
export OBLIVIOUS_TARGET_SECRET_AUDIT_URI="$artifact_base/secret-audit.json"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI="$artifact_base/workflow-telemetry.json"
export OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_URI="$artifact_base/request-log-observability.json"
export OBLIVIOUS_TARGET_RAG_INDEXING_URI="$artifact_base/rag-indexing-proof.json"
export OBLIVIOUS_TARGET_RELAY_REALTIME_URI="$artifact_base/relay-realtime-proof.json"
export OBLIVIOUS_TARGET_RELAY_BATCH_URI="$artifact_base/relay-batch-proof.json"
export OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_URI="$artifact_base/marketplace-payout-proof.json"
export OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI="$artifact_base/marketplace-governance-proof.json"
export OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_URI="$artifact_base/provider-runtime-config.json"
export OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_URI="$artifact_base/microservice-database-proof.json"
export OBLIVIOUS_TARGET_STRICT_VERIFIER_SHA256="1111111111111111111111111111111111111111111111111111111111111111"
export OBLIVIOUS_TARGET_DEPLOYMENT_SHA256="2222222222222222222222222222222222222222222222222222222222222222"
export OBLIVIOUS_TARGET_KUBERNETES_SHA256="3333333333333333333333333333333333333333333333333333333333333333"
export OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256="4444444444444444444444444444444444444444444444444444444444444444"
export OBLIVIOUS_TARGET_ALIPAY_PROVIDER_SHA256="5555555555555555555555555555555555555555555555555555555555555555"
export OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_SHA256="6666666666666666666666666666666666666666666666666666666666666666"
export OBLIVIOUS_TARGET_GRPC_SMOKE_SHA256="7777777777777777777777777777777777777777777777777777777777777777"
export OBLIVIOUS_TARGET_SECRET_AUDIT_SHA256="8888888888888888888888888888888888888888888888888888888888888888"
export OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SHA256="9999999999999999999999999999999999999999999999999999999999999999"
export OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_SHA256="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
export OBLIVIOUS_TARGET_RAG_INDEXING_SHA256="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
export OBLIVIOUS_TARGET_RELAY_REALTIME_SHA256="ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
export OBLIVIOUS_TARGET_RELAY_BATCH_SHA256="abababababababababababababababababababababababababababababababab"
export OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_SHA256="cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
export OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_SHA256="cdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcdcd"
export OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_SHA256="dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
export OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_SHA256="eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

cat >"$coverage_file" <<'JSON'
{
  "checkedRecords": 4,
  "usageRowsWithRequestId": 4,
  "usageRowsMissingRequestId": 0,
  "matchedRequestLogRecords": 4,
  "missingRequestLogRecords": 0,
  "issues": [],
  "limit": 100,
  "offset": 0
}
JSON
cat >"$platform_proof_file" <<'JSON'
{
  "clickHouseDeployment": "pass",
  "clickHouseMigration": "pass",
  "requestLogsTable": "pass",
  "ingestQuerySmoke": "pass"
}
JSON
cat >"$slo_file" <<'JSON'
{
  "latencySLOTrigger": "pass",
  "latencySLOAlertDelivery": "pass",
  "latencySLORecoveryAction": "pass",
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "triggeredAlerts": 2,
  "alertDelivery": {
    "configuredProviders": 1,
    "deliveredAlerts": 2,
    "failedDeliveries": 0,
    "channels": ["pagerduty-primary"],
    "lastDeliveryId": "alert_delivery_20260616_0001"
  },
  "recoveryAudit": {
    "auditRecords": 2,
    "failedActions": 0,
    "lastRecordId": "slo_recovery_audit_20260616_0001"
  }
}
JSON
cat >"$rag_proof_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 3,
    "drainedJobs": 3,
    "workerCompletedJobs": 3,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 2,
    "staleVectorRowsFiltered": 1
  }
}
JSON
cat >"$relay_realtime_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "authPolicy": "pass",
  "originPolicy": "pass",
  "prebillSettlement": "pass",
  "abortSettlement": "pass",
  "usageLedger": "pass",
  "summary": {
    "totalRequests": 4,
    "authenticatedRequests": 4,
    "requestLinkedUsageRecords": 4,
    "priceSnapshotRecords": 4,
    "abortSettlementRecords": 1,
    "terminalUsageRecords": 4,
    "originPolicyChecks": 4
  }
}
JSON
cat >"$relay_batch_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 3,
    "pollingCompletions": 3,
    "settlementRecords": 2,
    "refundRecords": 1,
    "usageAuditRecords": 3,
    "requestLogAuditRecords": 3,
    "terminalFailureRecords": 1
  }
}
JSON
cat >"$payout_proof_file" <<'JSON'
{
  "outboundDispatch": "pass",
  "inboundWebhookLifecycle": "pass",
  "settlementLedger": "pass",
  "reconciliation": "pass",
  "refundChargebackHandling": "pass",
  "summary": {
    "outboundDispatches": 3,
    "webhookEvents": 3,
    "settlementLedgerEntries": 3,
    "reconciledEntries": 3,
    "refundChargebackCases": 1,
    "refundChargebackCasesHandled": 1
  }
}
JSON
cat >"$governance_proof_file" <<'JSON'
{
  "reviewQueue": "pass",
  "appealQueue": "pass",
  "appealDecisionLifecycle": "pass",
  "reviewAssignment": "pass",
  "reviewSLAEnforcement": "pass",
  "abuseReportLifecycle": "pass",
  "summary": {
    "reviewQueueItems": 4,
    "appealQueueItems": 2,
    "appealDecisions": 2,
    "reviewAssignments": 4,
    "slaChecks": 4,
    "slaBreachesHandled": 1,
    "abuseReports": 2,
    "abuseReportsResolved": 2
  }
}
JSON
cat >"$provider_runtime_config_proof_file" <<'JSON'
{
  "stripe": "pass",
  "alipay": "pass",
  "wechatpay": "pass",
  "providerEnv": "pass",
  "checkoutBaseUrls": "pass",
  "webhookRoutes": "pass",
  "webhookVerification": "pass",
  "providers": [
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260701"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260701"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260701"}
  ],
  "summary": {
    "providersConfigured": 3,
    "providerEnvVarsChecked": 9,
    "checkoutBaseUrlsChecked": 3,
    "webhookRoutesChecked": 3,
    "webhookVerificationChecks": 3
  }
}
JSON
cat >"$microservice_database_proof_file" <<'JSON'
{
  "mode": "microservices",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 11,
    "externalUrlsChecked": 11,
    "migrationReadinessChecks": 11
  }
}
JSON
add_microservice_services "$microservice_database_proof_file"

assemble_target_evidence() {
  local target_output="$1"
  shift
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$target_output" \
    "$@"
}

assemble_target_evidence "$manifest_file" --validate >/dev/null
echo "[assemble-target-release-evidence-fixtures] assembled and validated target evidence manifest"

"$python_bin" "$mutation_helper" --write-artifacts "$manifest_file" "$artifact_dir"
target_url_digest_env_file="$tmpdir/target-url-digest-env.sh"
bash "$digest_tool" --manifest "$manifest_file" --artifact-dir "$artifact_dir" --write >/dev/null
"$python_bin" - "$manifest_file" "$strict_verifier_proof_file" "$target_url_digest_env_file" <<'PY'
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
strict_proof_path = pathlib.Path(sys.argv[2])
digest_env_path = pathlib.Path(sys.argv[3])

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
strict = manifest["strictVerifier"]
proof = json.loads(strict_proof_path.read_text(encoding="utf-8"))
proof["targetEvidenceSha256"] = strict["targetEvidenceSha256"]
proof["artifactBundleSha256"] = strict["artifactBundleSha256"]
strict_proof_path.write_text(json.dumps(proof, indent=2, sort_keys=True) + "\n", encoding="utf-8")

env_by_artifact = {
    ("strict-verifier-log", None): "OBLIVIOUS_TARGET_STRICT_VERIFIER_SHA256",
    ("deployment-log", None): "OBLIVIOUS_TARGET_DEPLOYMENT_SHA256",
    ("kubernetes-validation", None): "OBLIVIOUS_TARGET_KUBERNETES_SHA256",
    ("provider-live-rail", "stripe"): "OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256",
    ("provider-live-rail", "alipay"): "OBLIVIOUS_TARGET_ALIPAY_PROVIDER_SHA256",
    ("provider-live-rail", "wechatpay"): "OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_SHA256",
    ("grpc-smoke-report", None): "OBLIVIOUS_TARGET_GRPC_SMOKE_SHA256",
    ("secret-audit", None): "OBLIVIOUS_TARGET_SECRET_AUDIT_SHA256",
    ("workflow-telemetry", None): "OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SHA256",
    ("request-log-observability", None): "OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_SHA256",
    ("rag-indexing-proof", None): "OBLIVIOUS_TARGET_RAG_INDEXING_SHA256",
    ("relay-realtime-proof", None): "OBLIVIOUS_TARGET_RELAY_REALTIME_SHA256",
    ("relay-batch-proof", None): "OBLIVIOUS_TARGET_RELAY_BATCH_SHA256",
    ("marketplace-payout-proof", None): "OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_SHA256",
    ("marketplace-governance-proof", None): "OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_SHA256",
    ("provider-runtime-config", None): "OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_SHA256",
    ("microservice-database-proof", None): "OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_SHA256",
}
lines = []
for artifact in manifest["artifacts"]:
    env_name = env_by_artifact[(artifact["kind"], artifact.get("provider"))]
    lines.append(f"{env_name}={artifact['sha256']}\n")
digest_env_path.write_text("".join(lines), encoding="utf-8")
PY
while IFS='=' read -r digest_env_name digest_env_value; do
  export "$digest_env_name=$digest_env_value"
done <"$target_url_digest_env_file"
artifact_validated_manifest_file="$tmpdir/target-release-evidence-artifact-validated.json"
assemble_target_evidence "$artifact_validated_manifest_file" --artifact-dir "$artifact_dir" --validate >/dev/null
echo "[assemble-target-release-evidence-fixtures] assembler validated target artifact bodies"
bad_artifact_dir="$tmpdir/bad-artifacts"
mkdir -p "$bad_artifact_dir"
cp "$artifact_dir"/*.json "$bad_artifact_dir"/
printf '\n' >>"$bad_artifact_dir/target-run-20260701-rag-indexing-proof.json"
bad_artifact_output="$tmpdir/bad-artifact-dir.out"
if assemble_target_evidence "$tmpdir/bad-artifact-manifest.json" --artifact-dir "$bad_artifact_dir" --validate >"$bad_artifact_output" 2>&1; then
  cat "$bad_artifact_output" >&2
  fail "mismatched artifact body SHA-256 unexpectedly passed assembler validation"
fi
if ! grep -Fq -- "body sha256 must match manifest sha256" "$bad_artifact_output"; then
  cat "$bad_artifact_output" >&2
  fail "mismatched artifact body failed without naming body SHA-256 mismatch"
fi
echo "[assemble-target-release-evidence-fixtures] assembler rejected mismatched artifact body"
cat >"$relay_realtime_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "authPolicy": "pass",
  "originPolicy": "pass",
  "prebillSettlement": "pass",
  "abortSettlement": "pass",
  "usageLedger": "pass",
  "summary": {
    "totalRequests": 4,
    "authenticatedRequests": 4,
    "requestLinkedUsageRecords": 4,
    "priceSnapshotRecords": 4,
    "abortSettlementRecords": 1,
    "terminalUsageRecords": 4,
    "originPolicyChecks": 4
  }
}
JSON
cat >"$relay_batch_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 3,
    "pollingCompletions": 3,
    "settlementRecords": 2,
    "refundRecords": 1,
    "usageAuditRecords": 3,
    "requestLogAuditRecords": 3,
    "terminalFailureRecords": 1
  }
}
JSON
cat >"$payout_proof_file" <<'JSON'
{
  "outboundDispatch": "pass",
  "inboundWebhookLifecycle": "pass",
  "settlementLedger": "pass",
  "reconciliation": "pass",
  "refundChargebackHandling": "pass",
  "summary": {
    "outboundDispatches": 3,
    "webhookEvents": 3,
    "settlementLedgerEntries": 3,
    "reconciledEntries": 3,
    "refundChargebackCases": 1,
    "refundChargebackCasesHandled": 1
  }
}
JSON
cat >"$governance_proof_file" <<'JSON'
{
  "reviewQueue": "pass",
  "appealQueue": "pass",
  "appealDecisionLifecycle": "pass",
  "reviewAssignment": "pass",
  "reviewSLAEnforcement": "pass",
  "abuseReportLifecycle": "pass",
  "summary": {
    "reviewQueueItems": 4,
    "appealQueueItems": 2,
    "appealDecisions": 2,
    "reviewAssignments": 4,
    "slaChecks": 4,
    "slaBreachesHandled": 1,
    "abuseReports": 2,
    "abuseReportsResolved": 2
  }
}
JSON
cat >"$provider_runtime_config_proof_file" <<'JSON'
{
  "stripe": "pass",
  "alipay": "pass",
  "wechatpay": "pass",
  "providerEnv": "pass",
  "checkoutBaseUrls": "pass",
  "webhookRoutes": "pass",
  "webhookVerification": "pass",
  "providers": [
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260701"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260701"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260701"}
  ],
  "summary": {
    "providersConfigured": 3,
    "providerEnvVarsChecked": 9,
    "checkoutBaseUrlsChecked": 3,
    "webhookRoutesChecked": 3,
    "webhookVerificationChecks": 3
  }
}
JSON
cat >"$microservice_database_proof_file" <<'JSON'
{
  "mode": "microservices",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 11,
    "externalUrlsChecked": 11,
    "migrationReadinessChecks": 11
  }
}
JSON
add_microservice_services "$microservice_database_proof_file"
fixture_bash_bin="${BASH:-bash}"
if command -v cygpath >/dev/null 2>&1; then
  fixture_bash_bin="$(cygpath -w "$(command -v bash)")"
fi
export OBLIVIOUS_FIXTURE_BASH_BIN="$fixture_bash_bin"
"$python_bin" - "$manifest_file" "$artifact_dir" "$request_log_collector" "$platform_proof_file" "$coverage_file" "$slo_file" "$rag_collector" "$rag_proof_file" "$relay_realtime_collector" "$relay_realtime_proof_file" "$relay_batch_collector" "$relay_batch_proof_file" "$payout_collector" "$payout_proof_file" "$governance_collector" "$governance_proof_file" "$provider_runtime_config_collector" "$provider_runtime_config_proof_file" "$microservice_database_collector" "$microservice_database_proof_file" <<'PY'
import json
import os
import pathlib
import subprocess
import sys

def bash_path(value):
    text = str(value)
    if os.name != "nt":
        return text
    normalized = text.replace("\\", "/")
    if len(normalized) >= 3 and normalized[1] == ":" and normalized[2] == "/":
        return f"/{normalized[0].lower()}{normalized[2:]}"
    return normalized

bash_bin = os.environ.get("OBLIVIOUS_FIXTURE_BASH_BIN", "bash")

def run_collector(args):
    result = subprocess.run(
        args,
        text=True,
        encoding="utf-8",
        errors="replace",
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stdout)
        raise subprocess.CalledProcessError(result.returncode, args)

manifest_path = pathlib.Path(sys.argv[1])
artifact_dir = pathlib.Path(sys.argv[2])
request_log_collector = sys.argv[3]
platform_proof_file = sys.argv[4]
coverage_file = sys.argv[5]
slo_file = sys.argv[6]
rag_collector = sys.argv[7]
rag_proof_file = sys.argv[8]
relay_realtime_collector = sys.argv[9]
relay_realtime_proof_file = sys.argv[10]
relay_batch_collector = sys.argv[11]
relay_batch_proof_file = sys.argv[12]
payout_collector = sys.argv[13]
payout_proof_file = sys.argv[14]
governance_collector = sys.argv[15]
governance_proof_file = sys.argv[16]
provider_runtime_config_collector = sys.argv[17]
provider_runtime_config_proof_file = sys.argv[18]
microservice_database_collector = sys.argv[19]
microservice_database_proof_file = sys.argv[20]

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
request_log_artifact = next(
    item for item in manifest["artifacts"] if item["kind"] == "request-log-observability"
)
body_path = artifact_dir / f"{request_log_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(request_log_collector),
        "--artifact-id",
        request_log_artifact["id"],
        "--commit",
        request_log_artifact["commit"],
        "--run-id",
        request_log_artifact["runId"],
        "--recorded-at",
        request_log_artifact["recordedAt"],
        "--platform-proof-file",
        bash_path(platform_proof_file),
        "--coverage-file",
        bash_path(coverage_file),
        "--slo-file",
        bash_path(slo_file),
        "--output",
        bash_path(body_path),
    ]
)
rag_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "rag-indexing-proof")
rag_body_path = artifact_dir / f"{rag_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(rag_collector),
        "--artifact-id",
        rag_artifact["id"],
        "--commit",
        rag_artifact["commit"],
        "--run-id",
        rag_artifact["runId"],
        "--recorded-at",
        rag_artifact["recordedAt"],
        "--proof-file",
        bash_path(rag_proof_file),
        "--output",
        bash_path(rag_body_path),
    ]
)
relay_realtime_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "relay-realtime-proof")
relay_realtime_body_path = artifact_dir / f"{relay_realtime_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(relay_realtime_collector),
        "--artifact-id",
        relay_realtime_artifact["id"],
        "--commit",
        relay_realtime_artifact["commit"],
        "--run-id",
        relay_realtime_artifact["runId"],
        "--recorded-at",
        relay_realtime_artifact["recordedAt"],
        "--proof-file",
        bash_path(relay_realtime_proof_file),
        "--output",
        bash_path(relay_realtime_body_path),
    ]
)
relay_batch_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "relay-batch-proof")
relay_batch_body_path = artifact_dir / f"{relay_batch_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(relay_batch_collector),
        "--artifact-id",
        relay_batch_artifact["id"],
        "--commit",
        relay_batch_artifact["commit"],
        "--run-id",
        relay_batch_artifact["runId"],
        "--recorded-at",
        relay_batch_artifact["recordedAt"],
        "--proof-file",
        bash_path(relay_batch_proof_file),
        "--output",
        bash_path(relay_batch_body_path),
    ]
)
payout_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "marketplace-payout-proof")
payout_body_path = artifact_dir / f"{payout_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(payout_collector),
        "--artifact-id",
        payout_artifact["id"],
        "--commit",
        payout_artifact["commit"],
        "--run-id",
        payout_artifact["runId"],
        "--recorded-at",
        payout_artifact["recordedAt"],
        "--proof-file",
        bash_path(payout_proof_file),
        "--output",
        bash_path(payout_body_path),
    ]
)
governance_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "marketplace-governance-proof")
governance_body_path = artifact_dir / f"{governance_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(governance_collector),
        "--artifact-id",
        governance_artifact["id"],
        "--commit",
        governance_artifact["commit"],
        "--run-id",
        governance_artifact["runId"],
        "--recorded-at",
        governance_artifact["recordedAt"],
        "--proof-file",
        bash_path(governance_proof_file),
        "--output",
        bash_path(governance_body_path),
    ]
)
provider_runtime_config_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "provider-runtime-config")
provider_runtime_config_body_path = artifact_dir / f"{provider_runtime_config_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(provider_runtime_config_collector),
        "--artifact-id",
        provider_runtime_config_artifact["id"],
        "--commit",
        provider_runtime_config_artifact["commit"],
        "--run-id",
        provider_runtime_config_artifact["runId"],
        "--recorded-at",
        provider_runtime_config_artifact["recordedAt"],
        "--proof-file",
        bash_path(provider_runtime_config_proof_file),
        "--output",
        bash_path(provider_runtime_config_body_path),
    ]
)
microservice_database_artifact = next(item for item in manifest["artifacts"] if item["kind"] == "microservice-database-proof")
microservice_database_body_path = artifact_dir / f"{microservice_database_artifact['id']}.json"
run_collector(
    [
        bash_bin,
        bash_path(microservice_database_collector),
        "--artifact-id",
        microservice_database_artifact["id"],
        "--commit",
        microservice_database_artifact["commit"],
        "--run-id",
        microservice_database_artifact["runId"],
        "--recorded-at",
        microservice_database_artifact["recordedAt"],
        "--proof-file",
        bash_path(microservice_database_proof_file),
        "--output",
        bash_path(microservice_database_body_path),
    ]
)
PY
"$python_bin" - "$manifest_file" "$artifact_dir" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
artifact_dir = pathlib.Path(sys.argv[2])

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    body_path = artifact_dir / f"{artifact['id']}.json"
    artifact["sha256"] = hashlib.sha256(body_path.read_bytes()).hexdigest()
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bash "$digest_tool" --manifest "$manifest_file" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest_file" >/dev/null
echo "[assemble-target-release-evidence-fixtures] assembled and validated collector artifact bundle"

missing_output="$tmpdir/missing-env.out"
if (
  unset OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing.json" \
    --validate
) >"$missing_output" 2>&1; then
  cat "$missing_output" >&2
  fail "missing required artifact URI unexpectedly passed"
fi
if ! grep -Fq -- "OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI" "$missing_output"; then
  cat "$missing_output" >&2
  fail "missing required artifact URI failed without naming OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required artifact URI"

missing_sha_output="$tmpdir/missing-sha.out"
if (
  unset OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-sha.json" \
    --validate
) >"$missing_sha_output" 2>&1; then
  cat "$missing_sha_output" >&2
  fail "missing required artifact SHA-256 unexpectedly passed"
fi
if ! grep -Fq -- "OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256" "$missing_sha_output"; then
  cat "$missing_sha_output" >&2
  fail "missing required artifact SHA-256 failed without naming OBLIVIOUS_TARGET_STRIPE_PROVIDER_SHA256"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required artifact SHA-256"

missing_strict_verifier_output="$tmpdir/missing-strict-verifier-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-strict-verifier.json" \
    --validate
) >"$missing_strict_verifier_output" 2>&1; then
  cat "$missing_strict_verifier_output" >&2
  fail "missing required strict verifier proof unexpectedly passed"
fi
if ! grep -Fq -- "--strict-verifier-proof-file or OBLIVIOUS_TARGET_STRICT_VERIFIER_PROOF_FILE is required" "$missing_strict_verifier_output"; then
  cat "$missing_strict_verifier_output" >&2
  fail "missing strict verifier proof failed without naming --strict-verifier-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required strict verifier proof"

missing_deployment_output="$tmpdir/missing-deployment-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-deployment.json" \
    --validate
) >"$missing_deployment_output" 2>&1; then
  cat "$missing_deployment_output" >&2
  fail "missing required deployment proof unexpectedly passed"
fi
if ! grep -Fq -- "--deployment-proof-file or OBLIVIOUS_TARGET_DEPLOYMENT_PROOF_FILE is required" "$missing_deployment_output"; then
  cat "$missing_deployment_output" >&2
  fail "missing deployment proof failed without naming --deployment-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required deployment proof"

missing_kubernetes_output="$tmpdir/missing-kubernetes-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-kubernetes.json" \
    --validate
) >"$missing_kubernetes_output" 2>&1; then
  cat "$missing_kubernetes_output" >&2
  fail "missing required Kubernetes proof unexpectedly passed"
fi
if ! grep -Fq -- "--kubernetes-proof-file or OBLIVIOUS_TARGET_KUBERNETES_PROOF_FILE is required" "$missing_kubernetes_output"; then
  cat "$missing_kubernetes_output" >&2
  fail "missing Kubernetes proof failed without naming --kubernetes-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required Kubernetes proof"

missing_stripe_provider_output="$tmpdir/missing-stripe-provider-live-rail-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-stripe-provider-live-rail.json" \
    --validate
) >"$missing_stripe_provider_output" 2>&1; then
  cat "$missing_stripe_provider_output" >&2
  fail "missing required Stripe provider live rail proof unexpectedly passed"
fi
if ! grep -Fq -- "--stripe-provider-live-rail-proof-file or OBLIVIOUS_TARGET_STRIPE_PROVIDER_LIVE_RAIL_PROOF_FILE is required" "$missing_stripe_provider_output"; then
  cat "$missing_stripe_provider_output" >&2
  fail "missing Stripe provider live rail proof failed without naming --stripe-provider-live-rail-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required Stripe provider live rail proof"

missing_secret_audit_output="$tmpdir/missing-secret-audit-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-secret-audit.json" \
    --validate
) >"$missing_secret_audit_output" 2>&1; then
  cat "$missing_secret_audit_output" >&2
  fail "missing required secret audit proof unexpectedly passed"
fi
if ! grep -Fq -- "--secret-audit-proof-file or OBLIVIOUS_TARGET_SECRET_AUDIT_PROOF_FILE is required" "$missing_secret_audit_output"; then
  cat "$missing_secret_audit_output" >&2
  fail "missing secret audit proof failed without naming --secret-audit-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required secret audit proof"

missing_workflow_telemetry_output="$tmpdir/missing-workflow-telemetry-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-workflow-telemetry.json" \
    --validate
) >"$missing_workflow_telemetry_output" 2>&1; then
  cat "$missing_workflow_telemetry_output" >&2
  fail "missing required workflow telemetry proof unexpectedly passed"
fi
if ! grep -Fq -- "--workflow-telemetry-proof-file or OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_PROOF_FILE is required" "$missing_workflow_telemetry_output"; then
  cat "$missing_workflow_telemetry_output" >&2
  fail "missing workflow telemetry proof failed without naming --workflow-telemetry-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required workflow telemetry proof"

bad_strict_verifier_proof_file="$tmpdir/strict-verifier-bad.json"
cat >"$bad_strict_verifier_proof_file" <<JSON
{
  "artifactBundleSha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "command": "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
  "commit": "$current_commit",
  "result": "pass",
  "runId": "target-run-20260701",
  "skippedChecks": ["deployment"],
  "startedAt": "2026-07-01T00:00:00Z",
  "completedAt": "2026-07-01T00:30:00Z",
  "targetEvidenceSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
JSON
bad_strict_verifier_output="$tmpdir/bad-strict-verifier-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$bad_strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-strict-verifier.json" \
  --validate >"$bad_strict_verifier_output" 2>&1; then
  cat "$bad_strict_verifier_output" >&2
  fail "bad strict verifier proof unexpectedly passed"
fi
if ! grep -Fq -- "strict verifier proof.skippedChecks must be an empty array" "$bad_strict_verifier_output"; then
  cat "$bad_strict_verifier_output" >&2
  fail "bad strict verifier proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected invalid strict verifier proof"

bad_secret_audit_proof_file="$tmpdir/secret-audit-bad.json"
cat >"$bad_secret_audit_proof_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-07-01T00:25:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [{"kind": "embedded-token"}],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 42,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 0
  }
}
JSON
bad_secret_audit_output="$tmpdir/bad-secret-audit-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$bad_secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-secret-audit.json" \
  --validate >"$bad_secret_audit_output" 2>&1; then
  cat "$bad_secret_audit_output" >&2
  fail "bad secret audit proof unexpectedly passed"
fi
if ! grep -Fq -- "secret audit proof.findings must be an empty array" "$bad_secret_audit_output"; then
  cat "$bad_secret_audit_output" >&2
  fail "bad secret audit proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected invalid secret audit proof"

bad_secret_audit_summary_file="$tmpdir/secret-audit-bad-summary.json"
cat >"$bad_secret_audit_summary_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-07-01T00:25:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 41,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 1
  }
}
JSON
bad_secret_audit_summary_output="$tmpdir/bad-secret-audit-summary.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$bad_secret_audit_summary_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-secret-audit-summary.json" \
  --validate >"$bad_secret_audit_summary_output" 2>&1; then
  cat "$bad_secret_audit_summary_output" >&2
  fail "bad secret audit summary unexpectedly passed"
fi
if ! grep -Fq -- "secret audit proof.summary.rotationRequiredRecords must be zero" "$bad_secret_audit_summary_output"; then
  cat "$bad_secret_audit_summary_output" >&2
  fail "bad secret audit summary failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected invalid secret audit summary"

bad_workflow_telemetry_proof_file="$tmpdir/workflow-telemetry-bad.json"
cat >"$bad_workflow_telemetry_proof_file" <<'JSON'
{
  "result": "pass",
  "telemetry": {
    "successRate": 0.995,
    "window": "2026-07-01T00:00:00Z/2026-07-01T00:20:00Z",
    "totalExecutions": 200,
    "successfulExecutions": 198,
    "failedExecutions": 2
  }
}
JSON
bad_workflow_telemetry_output="$tmpdir/bad-workflow-telemetry-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$bad_workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-workflow-telemetry.json" \
  --validate >"$bad_workflow_telemetry_output" 2>&1; then
  cat "$bad_workflow_telemetry_output" >&2
  fail "bad workflow telemetry proof unexpectedly passed"
fi
if ! grep -Fq -- "workflow telemetry proof.telemetry.successRate must equal telemetry.successfulExecutions / telemetry.totalExecutions" "$bad_workflow_telemetry_output"; then
  cat "$bad_workflow_telemetry_output" >&2
  fail "bad workflow telemetry proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected invalid workflow telemetry proof"

bad_kubernetes_proof_file="$tmpdir/kubernetes-proof-bad.json"
cat >"$bad_kubernetes_proof_file" <<'JSON'
{
  "result": "pass",
  "targetEnvironment": "production",
  "clusterRef": "prod-cluster-20260701",
  "namespace": "oblivious",
  "validation": "pass",
  "rollout": "pass",
  "failover": "pass",
  "secretFileClass": "example",
  "references": {
    "validation": "k8s-validation-run-20260701",
    "rollout": "k8s-rollout-run-20260701",
    "failover": "k8s-failover-run-20260701"
  }
}
JSON
bad_kubernetes_output="$tmpdir/bad-kubernetes-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$bad_kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-kubernetes.json" \
  --validate >"$bad_kubernetes_output" 2>&1; then
  cat "$bad_kubernetes_output" >&2
  fail "bad Kubernetes proof unexpectedly passed"
fi
if ! grep -Fq -- "kubernetes proof.secretFileClass must be external-filled" "$bad_kubernetes_output"; then
  cat "$bad_kubernetes_output" >&2
  fail "bad Kubernetes proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected invalid Kubernetes proof"

bad_stripe_provider_proof_file="$tmpdir/stripe-provider-live-rail-bad.json"
cat >"$bad_stripe_provider_proof_file" <<'JSON'
{
  "provider": "alipay",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "alipay_checkout_live_20260616",
    "refund": "alipay_refund_live_20260616",
    "payout": "alipay_payout_live_20260616",
    "reconciliation": "alipay_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 2,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 2
  }
}
JSON
bad_stripe_provider_output="$tmpdir/bad-stripe-provider-live-rail.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$bad_stripe_provider_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-stripe-provider-live-rail.json" \
  --validate >"$bad_stripe_provider_output" 2>&1; then
  cat "$bad_stripe_provider_output" >&2
  fail "bad Stripe provider live rail proof unexpectedly passed"
fi
if ! grep -Fq -- "stripe provider live rail proof.provider must match stripe" "$bad_stripe_provider_output"; then
  cat "$bad_stripe_provider_output" >&2
  fail "bad Stripe provider live rail proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected provider live rail proof mismatch"

bad_stripe_provider_references_file="$tmpdir/stripe-provider-live-rail-bad-references.json"
cat >"$bad_stripe_provider_references_file" <<'JSON'
{
  "provider": "stripe",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "stripe_checkout_live_20260616",
    "refund": "stripe_refund_live_20260616",
    "payout": "stripe_payout_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 2,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 2
  }
}
JSON
bad_stripe_provider_references_output="$tmpdir/bad-stripe-provider-live-rail-references.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$bad_stripe_provider_references_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-stripe-provider-live-rail-references.json" \
  --validate >"$bad_stripe_provider_references_output" 2>&1; then
  cat "$bad_stripe_provider_references_output" >&2
  fail "bad Stripe provider live rail references unexpectedly passed"
fi
if ! grep -Fq -- "stripe provider live rail proof.references.reconciliation is required" "$bad_stripe_provider_references_output"; then
  cat "$bad_stripe_provider_references_output" >&2
  fail "bad Stripe provider live rail references failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected incomplete provider live rail references"

missing_slo_output="$tmpdir/missing-latency-slo.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-slo.json" \
    --validate
) >"$missing_slo_output" 2>&1; then
  cat "$missing_slo_output" >&2
  fail "missing required latency SLO proof unexpectedly passed"
fi
if ! grep -Fq -- "--request-log-slo-file or OBLIVIOUS_TARGET_REQUEST_LOG_SLO_FILE is required" "$missing_slo_output"; then
  cat "$missing_slo_output" >&2
  fail "missing latency SLO proof failed without naming --request-log-slo-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required latency SLO proof"

missing_rag_output="$tmpdir/missing-rag-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --output "$tmpdir/missing-rag.json" \
    --validate
) >"$missing_rag_output" 2>&1; then
  cat "$missing_rag_output" >&2
  fail "missing required RAG proof unexpectedly passed"
fi
if ! grep -Fq -- "--rag-proof-file or OBLIVIOUS_TARGET_RAG_PROOF_FILE is required" "$missing_rag_output"; then
  cat "$missing_rag_output" >&2
  fail "missing RAG proof failed without naming --rag-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required RAG proof"

missing_realtime_output="$tmpdir/missing-relay-realtime-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-relay-realtime.json" \
    --validate
) >"$missing_realtime_output" 2>&1; then
  cat "$missing_realtime_output" >&2
  fail "missing required Relay Realtime proof unexpectedly passed"
fi
if ! grep -Fq -- "--relay-realtime-proof-file or OBLIVIOUS_TARGET_RELAY_REALTIME_PROOF_FILE is required" "$missing_realtime_output"; then
  cat "$missing_realtime_output" >&2
  fail "missing Relay Realtime proof failed without naming --relay-realtime-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required Relay Realtime proof"

missing_batch_output="$tmpdir/missing-relay-batch-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-relay-batch.json" \
    --validate
) >"$missing_batch_output" 2>&1; then
  cat "$missing_batch_output" >&2
  fail "missing required Relay Batch proof unexpectedly passed"
fi
if ! grep -Fq -- "--relay-batch-proof-file or OBLIVIOUS_TARGET_RELAY_BATCH_PROOF_FILE is required" "$missing_batch_output"; then
  cat "$missing_batch_output" >&2
  fail "missing Relay Batch proof failed without naming --relay-batch-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required Relay Batch proof"

bad_realtime_proof_file="$tmpdir/relay-realtime-proof-bad.json"
cat >"$bad_realtime_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "authPolicy": "fail",
  "originPolicy": "pass",
  "prebillSettlement": "pass",
  "abortSettlement": "pass",
  "usageLedger": "pass",
  "summary": {
    "totalRequests": 4,
    "authenticatedRequests": 4,
    "requestLinkedUsageRecords": 4,
    "priceSnapshotRecords": 4,
    "abortSettlementRecords": 1,
    "terminalUsageRecords": 4,
    "originPolicyChecks": 4
  }
}
JSON
bad_realtime_output="$tmpdir/bad-relay-realtime-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$bad_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-relay-realtime.json" \
  --validate >"$bad_realtime_output" 2>&1; then
  cat "$bad_realtime_output" >&2
  fail "failed Relay Realtime lifecycle proof unexpectedly passed"
fi
if ! grep -Fq -- "relay realtime proof.authPolicy must be pass" "$bad_realtime_output"; then
  cat "$bad_realtime_output" >&2
  fail "failed Relay Realtime proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed Relay Realtime lifecycle proof"

bad_batch_summary_file="$tmpdir/relay-batch-proof-bad-summary.json"
cat >"$bad_batch_summary_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 3,
    "pollingCompletions": 3,
    "settlementRecords": 2,
    "refundRecords": 1,
    "usageAuditRecords": 2,
    "requestLogAuditRecords": 3,
    "terminalFailureRecords": 1
  }
}
JSON
bad_batch_summary_output="$tmpdir/bad-relay-batch-summary.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$bad_batch_summary_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-relay-batch-summary.json" \
  --validate >"$bad_batch_summary_output" 2>&1; then
  cat "$bad_batch_summary_output" >&2
  fail "incomplete Relay Batch lifecycle summary unexpectedly passed"
fi
if ! grep -Fq -- "relay batch proof summary.usageAuditRecords must cover summary.settlementRecords plus summary.refundRecords" "$bad_batch_summary_output"; then
  cat "$bad_batch_summary_output" >&2
  fail "bad Relay Batch summary failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected incomplete Relay Batch lifecycle summary"

bad_batch_request_log_summary_file="$tmpdir/relay-batch-proof-bad-request-log-summary.json"
cat >"$bad_batch_request_log_summary_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 3,
    "pollingCompletions": 3,
    "settlementRecords": 2,
    "refundRecords": 1,
    "usageAuditRecords": 3,
    "requestLogAuditRecords": 2,
    "terminalFailureRecords": 1
  }
}
JSON
bad_batch_request_log_summary_output="$tmpdir/bad-relay-batch-request-log-summary.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$bad_batch_request_log_summary_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-relay-batch-request-log-summary.json" \
  --validate >"$bad_batch_request_log_summary_output" 2>&1; then
  cat "$bad_batch_request_log_summary_output" >&2
  fail "incomplete Relay Batch request-log summary unexpectedly passed"
fi
if ! grep -Fq -- "relay batch proof summary.requestLogAuditRecords must cover summary.settlementRecords plus summary.refundRecords" "$bad_batch_request_log_summary_output"; then
  cat "$bad_batch_request_log_summary_output" >&2
  fail "bad Relay Batch request-log summary failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected incomplete Relay Batch request-log summary"

bad_rag_proof_file="$tmpdir/rag-indexing-proof-bad.json"
cat >"$bad_rag_proof_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "fail",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 3,
    "drainedJobs": 3,
    "workerCompletedJobs": 3,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 2,
    "staleVectorRowsFiltered": 1
  }
}
JSON
bad_rag_output="$tmpdir/bad-rag-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$bad_rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-rag.json" \
  --validate >"$bad_rag_output" 2>&1; then
  cat "$bad_rag_output" >&2
  fail "failed RAG raw parser replay proof unexpectedly passed"
fi
if ! grep -Fq -- "RAG indexing proof.rawParserReplay must be pass" "$bad_rag_output"; then
  cat "$bad_rag_output" >&2
  fail "failed RAG raw parser replay proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed RAG raw parser replay proof"

bad_rag_summary_file="$tmpdir/rag-indexing-proof-worker-mismatch.json"
cat >"$bad_rag_summary_file" <<'JSON'
{
  "durableQueueMigration": "pass",
  "workerDeployment": "pass",
  "enqueueDrainProbe": "pass",
  "rawParserReplay": "pass",
  "retrievalProbe": "pass",
  "staleVectorFilter": "pass",
  "summary": {
    "queuedJobs": 3,
    "drainedJobs": 3,
    "workerCompletedJobs": 2,
    "rawParserReplayCount": 1,
    "retrievalProbeCount": 2,
    "staleVectorRowsFiltered": 1
  }
}
JSON
bad_rag_summary_output="$tmpdir/bad-rag-summary.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$bad_rag_summary_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-rag-summary.json" \
  --validate >"$bad_rag_summary_output" 2>&1; then
  cat "$bad_rag_summary_output" >&2
  fail "RAG worker mismatch proof unexpectedly passed"
fi
if ! grep -Fq -- "RAG indexing summary.workerCompletedJobs must equal summary.drainedJobs" "$bad_rag_summary_output"; then
  cat "$bad_rag_summary_output" >&2
  fail "RAG worker mismatch proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected RAG worker mismatch summary"

missing_payout_output="$tmpdir/missing-marketplace-payout-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-marketplace-payout.json" \
    --validate
) >"$missing_payout_output" 2>&1; then
  cat "$missing_payout_output" >&2
  fail "missing required marketplace payout proof unexpectedly passed"
fi
if ! grep -Fq -- "--marketplace-payout-proof-file or OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_PROOF_FILE is required" "$missing_payout_output"; then
  cat "$missing_payout_output" >&2
  fail "missing marketplace payout proof failed without naming --marketplace-payout-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required marketplace payout proof"

missing_governance_output="$tmpdir/missing-marketplace-governance-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --output "$tmpdir/missing-marketplace-governance.json" \
    --validate
) >"$missing_governance_output" 2>&1; then
  cat "$missing_governance_output" >&2
  fail "missing required marketplace governance proof unexpectedly passed"
fi
if ! grep -Fq -- "--marketplace-governance-proof-file or OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_PROOF_FILE is required" "$missing_governance_output"; then
  cat "$missing_governance_output" >&2
  fail "missing marketplace governance proof failed without naming --marketplace-governance-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required marketplace governance proof"

bad_payout_proof_file="$tmpdir/marketplace-payout-proof-bad.json"
cat >"$bad_payout_proof_file" <<'JSON'
{
  "outboundDispatch": "pass",
  "inboundWebhookLifecycle": "pass",
  "settlementLedger": "pass",
  "reconciliation": "pass",
  "refundChargebackHandling": "fail",
  "summary": {
    "outboundDispatches": 3,
    "webhookEvents": 3,
    "settlementLedgerEntries": 3,
    "reconciledEntries": 3,
    "refundChargebackCases": 1,
    "refundChargebackCasesHandled": 1
  }
}
JSON
bad_payout_output="$tmpdir/bad-marketplace-payout-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$bad_payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-marketplace-payout.json" \
  --validate >"$bad_payout_output" 2>&1; then
  cat "$bad_payout_output" >&2
  fail "failed marketplace payout refund/chargeback proof unexpectedly passed"
fi
if ! grep -Fq -- "marketplace payout proof.refundChargebackHandling must be pass" "$bad_payout_output"; then
  cat "$bad_payout_output" >&2
  fail "failed marketplace payout proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed marketplace payout refund/chargeback proof"

bad_governance_summary_file="$tmpdir/marketplace-governance-proof-bad-summary.json"
cat >"$bad_governance_summary_file" <<'JSON'
{
  "reviewQueue": "pass",
  "appealQueue": "pass",
  "appealDecisionLifecycle": "pass",
  "reviewAssignment": "pass",
  "reviewSLAEnforcement": "pass",
  "abuseReportLifecycle": "pass",
  "summary": {
    "reviewQueueItems": 4,
    "appealQueueItems": 2,
    "appealDecisions": 1,
    "reviewAssignments": 4,
    "slaChecks": 4,
    "slaBreachesHandled": 1,
    "abuseReports": 2,
    "abuseReportsResolved": 2
  }
}
JSON
bad_governance_output="$tmpdir/bad-marketplace-governance-summary.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$bad_governance_summary_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-marketplace-governance.json" \
  --validate >"$bad_governance_output" 2>&1; then
  cat "$bad_governance_output" >&2
  fail "marketplace governance appeal summary mismatch unexpectedly passed"
fi
if ! grep -Fq -- "marketplace governance proof summary.appealDecisions must equal summary.appealQueueItems" "$bad_governance_output"; then
  cat "$bad_governance_output" >&2
  fail "bad marketplace governance summary failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected marketplace governance appeal summary mismatch"

missing_provider_runtime_output="$tmpdir/missing-provider-runtime-config-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/missing-provider-runtime-config.json" \
    --validate
) >"$missing_provider_runtime_output" 2>&1; then
  cat "$missing_provider_runtime_output" >&2
  fail "missing required provider runtime config proof unexpectedly passed"
fi
if ! grep -Fq -- "--provider-runtime-config-proof-file or OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_PROOF_FILE is required" "$missing_provider_runtime_output"; then
  cat "$missing_provider_runtime_output" >&2
  fail "missing provider runtime config proof failed without naming --provider-runtime-config-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required provider runtime config proof"

missing_microservice_database_output="$tmpdir/missing-microservice-database-proof.out"
if (
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
    --relay-realtime-proof-file "$relay_realtime_proof_file" \
    --relay-batch-proof-file "$relay_batch_proof_file" \
    --marketplace-payout-proof-file "$payout_proof_file" \
    --marketplace-governance-proof-file "$governance_proof_file" \
    --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
    --output "$tmpdir/missing-microservice-database.json" \
    --validate
) >"$missing_microservice_database_output" 2>&1; then
  cat "$missing_microservice_database_output" >&2
  fail "missing required microservice database proof unexpectedly passed"
fi
if ! grep -Fq -- "--microservice-database-proof-file or OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_PROOF_FILE is required" "$missing_microservice_database_output"; then
  cat "$missing_microservice_database_output" >&2
  fail "missing microservice database proof failed without naming --microservice-database-proof-file"
fi
echo "[assemble-target-release-evidence-fixtures] rejected missing required microservice database proof"

bad_provider_runtime_config_file="$tmpdir/provider-runtime-config-proof-bad.json"
cat >"$bad_provider_runtime_config_file" <<'JSON'
{
  "stripe": "pass",
  "alipay": "fail",
  "wechatpay": "pass",
  "providerEnv": "pass",
  "checkoutBaseUrls": "pass",
  "webhookRoutes": "pass",
  "webhookVerification": "pass",
  "providers": [
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260701"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260701"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260701"}
  ],
  "summary": {
    "providersConfigured": 3,
    "providerEnvVarsChecked": 9,
    "checkoutBaseUrlsChecked": 3,
    "webhookRoutesChecked": 3,
    "webhookVerificationChecks": 3
  }
}
JSON
bad_provider_runtime_output="$tmpdir/bad-provider-runtime-config-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$bad_provider_runtime_config_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-provider-runtime-config.json" \
  --validate >"$bad_provider_runtime_output" 2>&1; then
  cat "$bad_provider_runtime_output" >&2
  fail "failed provider runtime config proof unexpectedly passed"
fi
if ! grep -Fq -- "provider runtime config proof.alipay must be pass" "$bad_provider_runtime_output"; then
  cat "$bad_provider_runtime_output" >&2
  fail "failed provider runtime config proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed provider runtime config proof"

bad_microservice_database_file="$tmpdir/microservice-database-proof-bad-summary.json"
cat >"$bad_microservice_database_file" <<'JSON'
{
  "mode": "microservices",
  "serviceUrlClass": "external-filled",
  "relay": "pass",
  "chat": "pass",
  "workflow": "pass",
  "rag": "pass",
  "agent": "pass",
  "billing": "pass",
  "marketplace": "pass",
  "admin": "pass",
  "channel": "pass",
  "task": "pass",
  "observability": "pass",
  "migrationReadiness": "pass",
  "summary": {
    "servicesChecked": 10,
    "externalUrlsChecked": 10,
    "migrationReadinessChecks": 10
  }
}
JSON
add_microservice_services "$bad_microservice_database_file"
bad_microservice_database_output="$tmpdir/bad-microservice-database-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$bad_microservice_database_file" \
  --output "$tmpdir/bad-microservice-database.json" \
  --validate >"$bad_microservice_database_output" 2>&1; then
  cat "$bad_microservice_database_output" >&2
  fail "bad microservice database proof unexpectedly passed"
fi
if ! grep -Fq -- "microservice database proof summary.servicesChecked must equal 11" "$bad_microservice_database_output"; then
  cat "$bad_microservice_database_output" >&2
  fail "bad microservice database proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected incomplete microservice database proof"

bad_platform_proof_file="$tmpdir/clickhouse-request-log-platform-proof-bad.json"
cat >"$bad_platform_proof_file" <<'JSON'
{
  "clickHouseDeployment": "pass",
  "clickHouseMigration": "fail",
  "requestLogsTable": "pass",
  "ingestQuerySmoke": "pass"
}
JSON
bad_platform_output="$tmpdir/bad-request-log-platform-proof.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$bad_platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-platform.json" \
  --validate >"$bad_platform_output" 2>&1; then
  cat "$bad_platform_output" >&2
  fail "failed request-log platform proof unexpectedly passed"
fi
if ! grep -Fq -- "request-log platform proof.clickHouseMigration must be pass" "$bad_platform_output"; then
  cat "$bad_platform_output" >&2
  fail "failed request-log platform proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed request-log platform proof"

bad_coverage_file="$tmpdir/usage-request-log-coverage-bad.json"
cat >"$bad_coverage_file" <<'JSON'
{
  "checkedRecords": 4,
  "usageRowsWithRequestId": 4,
  "usageRowsMissingRequestId": 0,
  "matchedRequestLogRecords": 3,
  "missingRequestLogRecords": 1,
  "issues": [{"id": "usage_1", "requestId": "req_missing", "issue": "missing_request_log"}]
}
JSON
bad_coverage_output="$tmpdir/bad-request-log-coverage.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$bad_coverage_file" \
  --request-log-slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-coverage.json" \
  --validate >"$bad_coverage_output" 2>&1; then
  cat "$bad_coverage_output" >&2
  fail "failed request-log coverage proof unexpectedly passed"
fi
if ! grep -Fq -- "requestUsageJoin requires zero missing request-log records" "$bad_coverage_output"; then
  cat "$bad_coverage_output" >&2
  fail "failed request-log coverage proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed request-log coverage proof"

bad_slo_delivery_file="$tmpdir/latency-slo-proof-bad-delivery.json"
cat >"$bad_slo_delivery_file" <<'JSON'
{
  "latencySLOTrigger": "pass",
  "latencySLOAlertDelivery": "pass",
  "latencySLORecoveryAction": "pass",
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "triggeredAlerts": 2,
  "alertDelivery": {
    "configuredProviders": 1,
    "deliveredAlerts": 2,
    "failedDeliveries": 1,
    "channels": ["pagerduty-primary"],
    "lastDeliveryId": "alert_delivery_20260616_0001"
  },
  "recoveryAudit": {
    "auditRecords": 2,
    "failedActions": 0,
    "lastRecordId": "slo_recovery_audit_20260616_0001"
  }
}
JSON
bad_slo_delivery_output="$tmpdir/bad-request-log-slo-delivery.out"
if bash "$assembler" \
  --grpc-smoke-file "$smoke_file" \
  --strict-verifier-proof-file "$strict_verifier_proof_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
  --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
  --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
  --secret-audit-proof-file "$secret_audit_proof_file" \
  --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
  --request-log-platform-proof-file "$platform_proof_file" \
  --request-log-coverage-file "$coverage_file" \
  --request-log-slo-file "$bad_slo_delivery_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
  --output "$tmpdir/bad-slo-delivery.json" \
  --validate >"$bad_slo_delivery_output" 2>&1; then
  cat "$bad_slo_delivery_output" >&2
  fail "failed request-log SLO alert delivery proof unexpectedly passed"
fi
if ! grep -Fq -- "request-log latency SLO proof.alertDelivery.failedDeliveries must be zero" "$bad_slo_delivery_output"; then
  cat "$bad_slo_delivery_output" >&2
  fail "failed request-log SLO alert delivery proof failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected failed request-log SLO alert delivery proof"

invalid_class_output="$tmpdir/invalid-environment-class.out"
if (
  export OBLIVIOUS_TARGET_ENVIRONMENT_CLASS="developer laptop"
  bash "$assembler" \
    --grpc-smoke-file "$smoke_file" \
    --strict-verifier-proof-file "$strict_verifier_proof_file" \
    --deployment-proof-file "$deployment_proof_file" \
    --kubernetes-proof-file "$kubernetes_proof_file" \
    --stripe-provider-live-rail-proof-file "$stripe_provider_live_rail_proof_file" \
    --alipay-provider-live-rail-proof-file "$alipay_provider_live_rail_proof_file" \
    --wechatpay-provider-live-rail-proof-file "$wechatpay_provider_live_rail_proof_file" \
    --secret-audit-proof-file "$secret_audit_proof_file" \
    --workflow-telemetry-proof-file "$workflow_telemetry_proof_file" \
    --request-log-platform-proof-file "$platform_proof_file" \
    --request-log-coverage-file "$coverage_file" \
    --request-log-slo-file "$slo_file" \
    --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" \
    --output "$tmpdir/invalid-environment-class.json" \
    --validate
) >"$invalid_class_output" 2>&1; then
  cat "$invalid_class_output" >&2
  fail "invalid environment class unexpectedly passed"
fi
if ! grep -Fq -- "OBLIVIOUS_TARGET_ENVIRONMENT_CLASS must be staging, preproduction, or production" "$invalid_class_output"; then
  cat "$invalid_class_output" >&2
  fail "invalid environment class failed without expected diagnostic"
fi
echo "[assemble-target-release-evidence-fixtures] rejected invalid environment class"
