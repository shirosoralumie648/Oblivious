#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-target-release-artifacts.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"
evidence_server_pid=""
current_commit=$(git -C "$repo_root" rev-parse HEAD)

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  if [[ -n "$evidence_server_pid" ]]; then
    kill "$evidence_server_pid" >/dev/null 2>&1 || true
    wait "$evidence_server_pid" >/dev/null 2>&1 || true
  fi
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[collect-target-release-artifacts-fixtures] $*" >&2
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
        "evidenceId": f"microservice_database_{service}_20260616",
    }
    for service in services
]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
url_manifest="$tmpdir/manifest-url.json"
artifact_dir="$tmpdir/artifacts"
url_artifact_dir="$tmpdir/artifacts-url"
strict_verifier_file="$tmpdir/strict-verifier.json"
deployment_proof_file="$tmpdir/deployment-proof.json"
kubernetes_proof_file="$tmpdir/kubernetes-proof.json"
workflow_telemetry_file="$tmpdir/workflow-telemetry.json"
coverage_file="$tmpdir/usage-request-log-coverage.json"
request_log_platform_proof_file="$tmpdir/clickhouse-request-log-platform-proof.json"
slo_file="$tmpdir/latency-slo-proof.json"
rag_proof_file="$tmpdir/rag-indexing-proof.json"
relay_realtime_proof_file="$tmpdir/relay-realtime-proof.json"
relay_batch_proof_file="$tmpdir/relay-batch-proof.json"
payout_proof_file="$tmpdir/marketplace-payout-proof.json"
governance_proof_file="$tmpdir/marketplace-governance-proof.json"
provider_runtime_config_proof_file="$tmpdir/provider-runtime-config-proof.json"
stripe_provider_live_rail_file="$tmpdir/stripe-provider-live-rail.json"
alipay_provider_live_rail_file="$tmpdir/alipay-provider-live-rail.json"
wechatpay_provider_live_rail_file="$tmpdir/wechatpay-provider-live-rail.json"
grpc_smoke_file="$tmpdir/grpc-smoke.json"
secret_audit_file="$tmpdir/secret-audit.json"
microservice_database_proof_file="$tmpdir/microservice-database-proof.json"
evidence_server_script="$tmpdir/evidence_server.py"
evidence_server_port_file="$tmpdir/evidence-server-port"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$strict_verifier_file" <<JSON
{
  "artifactBundleSha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "command": "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
  "commit": "$current_commit",
  "result": "pass",
  "runId": "target-release-20260616",
  "skippedChecks": [],
  "startedAt": "2026-06-16T00:00:00Z",
  "completedAt": "2026-06-16T01:00:00Z",
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
    "deployValidation": "deploy-validation-run-20260616",
    "backupRestore": "backup-restore-run-20260616",
    "migrationReplay": "migration-replay-run-20260616"
  }
}
JSON
cat >"$kubernetes_proof_file" <<'JSON'
{
  "result": "pass",
  "targetEnvironment": "production",
  "clusterRef": "prod-cluster-20260616",
  "namespace": "oblivious",
  "validation": "pass",
  "rollout": "pass",
  "failover": "pass",
  "secretFileClass": "external-filled",
  "references": {
    "validation": "k8s-validation-run-20260616",
    "rollout": "k8s-rollout-run-20260616",
    "failover": "k8s-failover-run-20260616"
  }
}
JSON
cat >"$workflow_telemetry_file" <<'JSON'
{
  "successRate": 0.99,
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "totalExecutions": 100,
  "successfulExecutions": 99,
  "failedExecutions": 1
}
JSON
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
cat >"$request_log_platform_proof_file" <<'JSON'
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
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260616"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260616"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260616"}
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
cat >"$stripe_provider_live_rail_file" <<'JSON'
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
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 1
  }
}
JSON
cat >"$alipay_provider_live_rail_file" <<'JSON'
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
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 1
  }
}
JSON
cat >"$wechatpay_provider_live_rail_file" <<'JSON'
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
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 1
  }
}
JSON
cat >"$grpc_smoke_file" <<'JSON'
{
  "recordedAt": "2026-06-16T00:00:00Z",
  "timeout": "10s",
  "results": [
    {
      "service": "agent",
      "address": "agent.prod.oblivious.release.test:50063",
      "generatedClient": "pass",
      "status": "validation_error"
    },
    {
      "service": "workflow",
      "address": "workflow.prod.oblivious.release.test:50064",
      "generatedClient": "pass",
      "status": "validation_response"
    },
    {
      "service": "task",
      "address": "task.prod.oblivious.release.test:50065",
      "generatedClient": "pass",
      "status": "validation_response"
    }
  ]
}
JSON
cat >"$secret_audit_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-06-16T00:45:00Z",
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

bash "$collector" \
  --manifest "$manifest" \
  --artifact-dir "$artifact_dir" \
  --strict-verifier-file "$strict_verifier_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --workflow-telemetry-file "$workflow_telemetry_file" \
  --request-log-platform-proof-file "$request_log_platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --stripe-provider-live-rail-file "$stripe_provider_live_rail_file" \
  --alipay-provider-live-rail-file "$alipay_provider_live_rail_file" \
  --wechatpay-provider-live-rail-file "$wechatpay_provider_live_rail_file" \
  --grpc-smoke-file "$grpc_smoke_file" \
  --secret-audit-file "$secret_audit_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" >/dev/null

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-target-release-artifacts-fixtures] generated all target release artifact bodies"

cat >"$evidence_server_script" <<'PY'
import json
import pathlib
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

port_file = pathlib.Path(sys.argv[1])
files_by_path = {
    "/internal/release/strict-verifier.json": pathlib.Path(sys.argv[2]),
    "/internal/release/deployment-proof.json": pathlib.Path(sys.argv[3]),
    "/internal/release/kubernetes-proof.json": pathlib.Path(sys.argv[4]),
    "/internal/release/workflow-telemetry.json": pathlib.Path(sys.argv[5]),
    "/internal/release/clickhouse-request-log-platform-proof.json": pathlib.Path(sys.argv[6]),
    "/api/v1/admin/billing/reconciliation/usage-request-logs": pathlib.Path(sys.argv[7]),
    "/api/v1/admin/observability/latency-slo-proof": pathlib.Path(sys.argv[8]),
    "/internal/release/rag-indexing-proof.json": pathlib.Path(sys.argv[9]),
    "/api/v1/admin/release-evidence/rag-indexing": pathlib.Path(sys.argv[9]),
    "/internal/release/relay-realtime-proof.json": pathlib.Path(sys.argv[10]),
    "/api/v1/admin/release-evidence/relay-realtime": pathlib.Path(sys.argv[10]),
    "/internal/release/relay-batch-proof.json": pathlib.Path(sys.argv[11]),
    "/api/v1/admin/release-evidence/relay-batch": pathlib.Path(sys.argv[11]),
    "/internal/release/marketplace-payout-proof.json": pathlib.Path(sys.argv[12]),
    "/api/v1/admin/release-evidence/marketplace-payout": pathlib.Path(sys.argv[12]),
    "/internal/release/marketplace-governance-proof.json": pathlib.Path(sys.argv[13]),
    "/api/v1/admin/release-evidence/marketplace-governance": pathlib.Path(sys.argv[13]),
    "/internal/release/provider-runtime-config-proof.json": pathlib.Path(sys.argv[14]),
    "/api/v1/admin/release-evidence/provider-runtime-config": pathlib.Path(sys.argv[14]),
    "/internal/release/stripe-provider-live-rail.json": pathlib.Path(sys.argv[15]),
    "/internal/release/alipay-provider-live-rail.json": pathlib.Path(sys.argv[16]),
    "/internal/release/wechatpay-provider-live-rail.json": pathlib.Path(sys.argv[17]),
    "/internal/release/grpc-smoke.json": pathlib.Path(sys.argv[18]),
    "/internal/release/secret-audit.json": pathlib.Path(sys.argv[19]),
    "/internal/release/microservice-database-proof.json": pathlib.Path(sys.argv[20]),
    "/api/v1/admin/release-evidence/microservice-database": pathlib.Path(sys.argv[20]),
}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        source = files_by_path.get(parsed.path)
        if source is None:
            self.send_error(404)
            return
        if self.headers.get("Authorization") != "Bearer aggregate-fixture-token":
            self.send_error(401)
            return
        payload = {"data": json.loads(source.read_text(encoding="utf-8"))}
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_args):
        return


server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
port_file.write_text(str(server.server_port), encoding="utf-8")
server.serve_forever()
PY

"$python_bin" "$evidence_server_script" \
  "$evidence_server_port_file" \
  "$strict_verifier_file" \
  "$deployment_proof_file" \
  "$kubernetes_proof_file" \
  "$workflow_telemetry_file" \
  "$request_log_platform_proof_file" \
  "$coverage_file" \
  "$slo_file" \
  "$rag_proof_file" \
  "$relay_realtime_proof_file" \
  "$relay_batch_proof_file" \
  "$payout_proof_file" \
  "$governance_proof_file" \
  "$provider_runtime_config_proof_file" \
  "$stripe_provider_live_rail_file" \
  "$alipay_provider_live_rail_file" \
  "$wechatpay_provider_live_rail_file" \
  "$grpc_smoke_file" \
  "$secret_audit_file" \
  "$microservice_database_proof_file" &
evidence_server_pid=$!
for _ in $(seq 1 50); do
  if [[ -s "$evidence_server_port_file" ]]; then
    break
  fi
  sleep 0.1
done
if [[ ! -s "$evidence_server_port_file" ]]; then
  fail "target evidence fixture server did not start"
fi
evidence_server_port=$(cat "$evidence_server_port_file")

cp "$template_manifest" "$url_manifest"
"$python_bin" "$mutation_helper" --fill "$url_manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$url_manifest" "$url_artifact_dir"

OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN=aggregate-fixture-token bash "$collector" \
  --manifest "$url_manifest" \
  --artifact-dir "$url_artifact_dir" \
  --strict-verifier-url "http://127.0.0.1:${evidence_server_port}/internal/release/strict-verifier.json" \
  --deployment-proof-url "http://127.0.0.1:${evidence_server_port}/internal/release/deployment-proof.json" \
  --kubernetes-proof-url "http://127.0.0.1:${evidence_server_port}/internal/release/kubernetes-proof.json" \
  --workflow-telemetry-url "http://127.0.0.1:${evidence_server_port}/internal/release/workflow-telemetry.json" \
  --request-log-platform-proof-url "http://127.0.0.1:${evidence_server_port}/internal/release/clickhouse-request-log-platform-proof.json" \
  --target-base-url "http://127.0.0.1:${evidence_server_port}" \
  --coverage-query limit=100 \
  --slo-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/observability/latency-slo-proof?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --rag-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/rag-indexing?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --relay-realtime-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/relay-realtime?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --relay-batch-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/relay-batch?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --marketplace-payout-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/marketplace-payout?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --marketplace-governance-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/marketplace-governance?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --provider-runtime-config-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/provider-runtime-config?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" \
  --stripe-provider-live-rail-url "http://127.0.0.1:${evidence_server_port}/internal/release/stripe-provider-live-rail.json" \
  --alipay-provider-live-rail-url "http://127.0.0.1:${evidence_server_port}/internal/release/alipay-provider-live-rail.json" \
  --wechatpay-provider-live-rail-url "http://127.0.0.1:${evidence_server_port}/internal/release/wechatpay-provider-live-rail.json" \
  --grpc-smoke-url "http://127.0.0.1:${evidence_server_port}/internal/release/grpc-smoke.json" \
  --secret-audit-url "http://127.0.0.1:${evidence_server_port}/internal/release/secret-audit.json" \
  --microservice-database-proof-url "http://127.0.0.1:${evidence_server_port}/api/v1/admin/release-evidence/microservice-database?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z" >/dev/null

bash "$digest_tool" --manifest "$url_manifest" --artifact-dir "$url_artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$url_artifact_dir" bash "$verifier" --allow-local-collection-source "$url_manifest" >/dev/null
echo "[collect-target-release-artifacts-fixtures] generated all target release artifact bodies from URLs"

collector_file_args=(
  --manifest "$manifest"
  --artifact-dir "$artifact_dir"
  --strict-verifier-file "$strict_verifier_file"
  --deployment-proof-file "$deployment_proof_file"
  --kubernetes-proof-file "$kubernetes_proof_file"
  --workflow-telemetry-file "$workflow_telemetry_file"
  --request-log-platform-proof-file "$request_log_platform_proof_file"
  --coverage-file "$coverage_file"
  --slo-file "$slo_file"
  --rag-proof-file "$rag_proof_file"
  --relay-realtime-proof-file "$relay_realtime_proof_file"
  --relay-batch-proof-file "$relay_batch_proof_file"
  --marketplace-payout-proof-file "$payout_proof_file"
  --marketplace-governance-proof-file "$governance_proof_file"
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file"
  --stripe-provider-live-rail-file "$stripe_provider_live_rail_file"
  --alipay-provider-live-rail-file "$alipay_provider_live_rail_file"
  --wechatpay-provider-live-rail-file "$wechatpay_provider_live_rail_file"
  --grpc-smoke-file "$grpc_smoke_file"
  --secret-audit-file "$secret_audit_file"
  --microservice-database-proof-file "$microservice_database_proof_file"
)

run_collector_with_file_sources() {
  local output_file="$1"
  shift
  bash "$collector" "${collector_file_args[@]}" "$@" >"$output_file" 2>&1
}

run_collector_without_source() {
  local omitted_flag="$1"
  local output_file="$2"
  shift 2
  local args=()
  local index=0
  while [[ "$index" -lt "${#collector_file_args[@]}" ]]; do
    if [[ "${collector_file_args[$index]}" == "$omitted_flag" ]]; then
      index=$((index + 2))
      continue
    fi
    args+=("${collector_file_args[$index]}")
    index=$((index + 1))
  done
  bash "$collector" "${args[@]}" "$@" >"$output_file" 2>&1
}

source_choice_cases=(
  "strict-verifier|--strict-verifier-file|--strict-verifier-url|https://target.example.com/internal/release/strict-verifier.json"
  "deployment-proof|--deployment-proof-file|--deployment-proof-url|https://target.example.com/internal/release/deployment-proof.json"
  "kubernetes-proof|--kubernetes-proof-file|--kubernetes-proof-url|https://target.example.com/internal/release/kubernetes-proof.json"
  "workflow-telemetry|--workflow-telemetry-file|--workflow-telemetry-url|https://target.example.com/internal/release/workflow-telemetry.json"
  "latency-slo|--slo-file|--slo-url|https://target.example.com/api/v1/admin/observability/latency-slo-proof?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "rag-indexing|--rag-proof-file|--rag-proof-url|https://target.example.com/api/v1/admin/release-evidence/rag-indexing?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "relay-realtime|--relay-realtime-proof-file|--relay-realtime-proof-url|https://target.example.com/api/v1/admin/release-evidence/relay-realtime?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "relay-batch|--relay-batch-proof-file|--relay-batch-proof-url|https://target.example.com/api/v1/admin/release-evidence/relay-batch?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "marketplace-payout|--marketplace-payout-proof-file|--marketplace-payout-proof-url|https://target.example.com/api/v1/admin/release-evidence/marketplace-payout?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "marketplace-governance|--marketplace-governance-proof-file|--marketplace-governance-proof-url|https://target.example.com/api/v1/admin/release-evidence/marketplace-governance?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "provider-runtime-config|--provider-runtime-config-proof-file|--provider-runtime-config-proof-url|https://target.example.com/api/v1/admin/release-evidence/provider-runtime-config?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
  "stripe-provider-live-rail|--stripe-provider-live-rail-file|--stripe-provider-live-rail-url|https://target.example.com/internal/release/stripe-provider-live-rail.json"
  "alipay-provider-live-rail|--alipay-provider-live-rail-file|--alipay-provider-live-rail-url|https://target.example.com/internal/release/alipay-provider-live-rail.json"
  "wechatpay-provider-live-rail|--wechatpay-provider-live-rail-file|--wechatpay-provider-live-rail-url|https://target.example.com/internal/release/wechatpay-provider-live-rail.json"
  "grpc-smoke|--grpc-smoke-file|--grpc-smoke-url|https://target.example.com/internal/release/grpc-smoke.json"
  "secret-audit|--secret-audit-file|--secret-audit-url|https://target.example.com/internal/release/secret-audit.json"
  "microservice-database|--microservice-database-proof-file|--microservice-database-proof-url|https://target.example.com/api/v1/admin/release-evidence/microservice-database?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
)

for source_case in "${source_choice_cases[@]}"; do
  IFS="|" read -r source_label file_flag url_flag url_value <<<"$source_case"
  expected="exactly one of ${file_flag}, ${url_flag} is required"

  missing_source_output="$tmpdir/missing-${source_label}-source.out"
  if run_collector_without_source "$file_flag" "$missing_source_output"; then
    cat "$missing_source_output" >&2
    fail "missing ${source_label} source unexpectedly passed"
  fi
  if ! grep -Fq -- "$expected" "$missing_source_output"; then
    cat "$missing_source_output" >&2
    fail "missing ${source_label} source failed without expected diagnostic"
  fi
  echo "[collect-target-release-artifacts-fixtures] rejected missing ${source_label} source"

  double_source_output="$tmpdir/double-${source_label}-source.out"
  if run_collector_with_file_sources "$double_source_output" "$url_flag" "$url_value"; then
    cat "$double_source_output" >&2
    fail "double ${source_label} source unexpectedly passed"
  fi
  if ! grep -Fq -- "$expected" "$double_source_output"; then
    cat "$double_source_output" >&2
    fail "double ${source_label} source failed without expected diagnostic"
  fi
  echo "[collect-target-release-artifacts-fixtures] rejected ambiguous ${source_label} source"
done

missing_output="$tmpdir/missing-rag-proof.out"
if bash "$collector" \
  --manifest "$manifest" \
  --artifact-dir "$artifact_dir" \
  --strict-verifier-file "$strict_verifier_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --workflow-telemetry-file "$workflow_telemetry_file" \
  --request-log-platform-proof-file "$request_log_platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$slo_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --stripe-provider-live-rail-file "$stripe_provider_live_rail_file" \
  --alipay-provider-live-rail-file "$alipay_provider_live_rail_file" \
  --wechatpay-provider-live-rail-file "$wechatpay_provider_live_rail_file" \
  --grpc-smoke-file "$grpc_smoke_file" \
  --secret-audit-file "$secret_audit_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" >"$missing_output" 2>&1; then
  cat "$missing_output" >&2
  fail "missing RAG proof file unexpectedly passed"
fi
if ! grep -Fq -- "exactly one of --rag-proof-file, --rag-proof-url is required" "$missing_output"; then
  cat "$missing_output" >&2
  fail "missing RAG proof file failed without expected diagnostic"
fi
echo "[collect-target-release-artifacts-fixtures] rejected missing RAG proof file"

missing_request_log_platform_output="$tmpdir/missing-request-log-platform-proof.out"
if bash "$collector" \
  --manifest "$manifest" \
  --artifact-dir "$artifact_dir" \
  --strict-verifier-file "$strict_verifier_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --workflow-telemetry-file "$workflow_telemetry_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --stripe-provider-live-rail-file "$stripe_provider_live_rail_file" \
  --alipay-provider-live-rail-file "$alipay_provider_live_rail_file" \
  --wechatpay-provider-live-rail-file "$wechatpay_provider_live_rail_file" \
  --grpc-smoke-file "$grpc_smoke_file" \
  --secret-audit-file "$secret_audit_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" >"$missing_request_log_platform_output" 2>&1; then
  cat "$missing_request_log_platform_output" >&2
  fail "missing request-log platform proof source unexpectedly passed"
fi
if ! grep -Fq -- "exactly one of --request-log-platform-proof-file, --request-log-platform-proof-url is required" "$missing_request_log_platform_output"; then
  cat "$missing_request_log_platform_output" >&2
  fail "missing request-log platform proof source failed without expected diagnostic"
fi
echo "[collect-target-release-artifacts-fixtures] rejected missing request-log platform proof source"

double_rag_output="$tmpdir/double-rag-proof.out"
if bash "$collector" \
  --manifest "$manifest" \
  --artifact-dir "$artifact_dir" \
  --strict-verifier-file "$strict_verifier_file" \
  --deployment-proof-file "$deployment_proof_file" \
  --kubernetes-proof-file "$kubernetes_proof_file" \
  --workflow-telemetry-file "$workflow_telemetry_file" \
  --request-log-platform-proof-file "$request_log_platform_proof_file" \
  --coverage-file "$coverage_file" \
  --slo-file "$slo_file" \
  --rag-proof-file "$rag_proof_file" \
  --rag-proof-url "https://target.example.com/internal/release/rag-indexing-proof.json" \
  --relay-realtime-proof-file "$relay_realtime_proof_file" \
  --relay-batch-proof-file "$relay_batch_proof_file" \
  --marketplace-payout-proof-file "$payout_proof_file" \
  --marketplace-governance-proof-file "$governance_proof_file" \
  --provider-runtime-config-proof-file "$provider_runtime_config_proof_file" \
  --stripe-provider-live-rail-file "$stripe_provider_live_rail_file" \
  --alipay-provider-live-rail-file "$alipay_provider_live_rail_file" \
  --wechatpay-provider-live-rail-file "$wechatpay_provider_live_rail_file" \
  --grpc-smoke-file "$grpc_smoke_file" \
  --secret-audit-file "$secret_audit_file" \
  --microservice-database-proof-file "$microservice_database_proof_file" >"$double_rag_output" 2>&1; then
  cat "$double_rag_output" >&2
  fail "double RAG proof source unexpectedly passed"
fi
if ! grep -Fq -- "exactly one of --rag-proof-file, --rag-proof-url is required" "$double_rag_output"; then
  cat "$double_rag_output" >&2
  fail "double RAG proof source failed without expected diagnostic"
fi
echo "[collect-target-release-artifacts-fixtures] rejected ambiguous RAG proof source"
