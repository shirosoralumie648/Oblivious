#!/usr/bin/env python3
import argparse
import hashlib
import json
import pathlib
from copy import deepcopy


def provider(data, name):
    return next(item for item in data["providers"] if item["name"] == name)


def artifact(data, artifact_id):
    return next(item for item in data["artifacts"] if item["id"] == artifact_id)


def grpc_entry(data, service):
    return next(item for item in data["grpc"] if item["service"] == service)


def grpc_result(data, service):
    return next(item for item in data["grpcSmokeReport"]["results"] if item["service"] == service)


def evidence_base_url(manifest):
    environment = (manifest or {}).get("environment", {}) if isinstance(manifest, dict) else {}
    base_url = environment.get("baseUrl")
    if not isinstance(base_url, str) or not base_url.strip().startswith(("http://", "https://")):
        return "https://target.oblivious.internal"
    return base_url.strip().rstrip("/")


def evidence_url(manifest, artifact_id, suffix=""):
    return f"{evidence_base_url(manifest)}/release-evidence/{artifact_id}{suffix}.json"


RELEASE_WINDOW_QUERY = "from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
ADMIN_RELEASE_EVIDENCE_PATH_BY_KIND = {
    "rag-indexing-proof": "/api/v1/admin/release-evidence/rag-indexing",
    "relay-realtime-proof": "/api/v1/admin/release-evidence/relay-realtime",
    "relay-batch-proof": "/api/v1/admin/release-evidence/relay-batch",
    "marketplace-payout-proof": "/api/v1/admin/release-evidence/marketplace-payout",
    "marketplace-governance-proof": "/api/v1/admin/release-evidence/marketplace-governance",
    "provider-runtime-config": "/api/v1/admin/release-evidence/provider-runtime-config",
    "microservice-database-proof": "/api/v1/admin/release-evidence/microservice-database",
}
MICROSERVICE_DATABASE_SERVICE_NAMES = [
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


def target_api_url(manifest, path, include_window=False):
    url = f"{evidence_base_url(manifest)}{path}"
    if include_window:
        url += f"?{RELEASE_WINDOW_QUERY}"
    return url


def artifact_collection_url(manifest, artifact):
    kind = artifact.get("kind")
    if kind == "request-log-observability":
        return target_api_url(manifest, "/api/v1/admin/billing/reconciliation/usage-request-logs") + "?limit=100"
    path = ADMIN_RELEASE_EVIDENCE_PATH_BY_KIND.get(kind)
    if path:
        return target_api_url(manifest, path, include_window=True)
    return evidence_url(manifest, artifact["id"])


def request_log_slo_url(manifest):
    return target_api_url(manifest, "/api/v1/admin/observability/latency-slo-proof", include_window=True)


def add_unused_artifact(data, include_lineage):
    entry = {
        "id": "artifact-unused-20260616",
        "kind": "supplemental-log",
        "uri": "ci://target-release/20260616/unused.log",
        "recordedAt": "2026-06-16T01:00:00Z",
    }
    if include_lineage:
        entry["commit"] = data["commit"]
        entry["runId"] = data["runId"]
    data["artifacts"].append(entry)


def artifact_body(artifact, manifest=None):
    body = {
        "artifactId": artifact["id"],
        "kind": artifact["kind"],
        "commit": artifact["commit"],
        "runId": artifact["runId"],
        "recordedAt": artifact["recordedAt"],
    }
    if "provider" in artifact:
        body["provider"] = artifact["provider"]
    if artifact["kind"] == "strict-verifier-log":
        strict_verifier = (manifest or {}).get("strictVerifier", {})
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["result"] = "pass"
        body["command"] = strict_verifier.get("command", "")
        body["skippedChecks"] = []
        body["startedAt"] = strict_verifier.get("startedAt", artifact["recordedAt"])
        body["completedAt"] = strict_verifier.get("completedAt", artifact["recordedAt"])
        body["targetEvidenceSha256"] = strict_verifier.get("targetEvidenceSha256", "a" * 64)
        body["artifactBundleSha256"] = strict_verifier.get("artifactBundleSha256", "b" * 64)
    if artifact["kind"] == "deployment-log":
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["result"] = "pass"
        body["targetEnvironment"] = "production"
        body["proofs"] = {
            "deployValidation": "pass",
            "backupRestore": "pass",
            "migrationReplay": "pass",
        }
        body["references"] = {
            "deployValidation": "deploy-validation-run-20260616",
            "backupRestore": "backup-restore-run-20260616",
            "migrationReplay": "migration-replay-run-20260616",
        }
    if artifact["kind"] == "kubernetes-validation":
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["result"] = "pass"
        body["targetEnvironment"] = "production"
        body["clusterRef"] = "prod-cluster-20260616"
        body["namespace"] = "oblivious"
        body["secretFileClass"] = "external-filled"
        body["proofs"] = {
            "validation": "pass",
            "rollout": "pass",
            "failover": "pass",
        }
        body["references"] = {
            "validation": "k8s-validation-run-20260616",
            "rollout": "k8s-rollout-run-20260616",
            "failover": "k8s-failover-run-20260616",
        }
    if "proofs" in artifact:
        body["proofs"] = artifact["proofs"]
        source_type = "target-api" if artifact["kind"] == "request-log-observability" else "target-url"
        body["collectionSource"] = {
            "type": source_type,
            "url": artifact_collection_url(manifest, artifact),
            "collectedAt": artifact["recordedAt"],
        }
    if artifact["kind"] == "workflow-telemetry":
        workflow_telemetry = (manifest or {}).get("workflowTelemetry", {})
        success_rate = workflow_telemetry.get("successRate", 0.99)
        total_executions = workflow_telemetry.get("totalExecutions")
        successful_executions = workflow_telemetry.get("successfulExecutions")
        failed_executions = workflow_telemetry.get("failedExecutions")
        if type(total_executions) is not int or type(successful_executions) is not int or type(failed_executions) is not int:
            if abs(float(success_rate) - 0.995) < 0.000001:
                total_executions = 200
                successful_executions = 199
            else:
                total_executions = 100
                successful_executions = int(round(float(success_rate) * total_executions))
            failed_executions = total_executions - successful_executions
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["result"] = "pass"
        body["telemetry"] = {
            "successRate": success_rate,
            "window": workflow_telemetry.get("window", "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z"),
            "totalExecutions": total_executions,
            "successfulExecutions": successful_executions,
            "failedExecutions": failed_executions,
        }
    if artifact["kind"] == "microservice-database-proof":
        body["mode"] = "microservices"
        body["serviceUrlClass"] = "external-filled"
        body["services"] = [
            {
                "name": service,
                "databaseUrlClass": "external-filled",
                "migrationReadiness": "pass",
                "evidenceId": f"microservice_database_{service}_20260616",
            }
            for service in MICROSERVICE_DATABASE_SERVICE_NAMES
        ]
        body["summary"] = {
            "servicesChecked": 11,
            "externalUrlsChecked": 11,
            "migrationReadinessChecks": 11,
        }
    if artifact["kind"] == "rag-indexing-proof":
        body["summary"] = {
            "queuedJobs": 2,
            "drainedJobs": 2,
            "workerCompletedJobs": 2,
            "rawParserReplayCount": 1,
            "retrievalProbeCount": 2,
            "staleVectorRowsFiltered": 1,
        }
    if artifact["kind"] == "request-log-observability":
        body["platformProofSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"], "-platform-proof"),
            "collectedAt": artifact["recordedAt"],
        }
        body["sloProofSource"] = {
            "type": "target-url",
            "url": request_log_slo_url(manifest),
            "collectedAt": artifact["recordedAt"],
        }
        body["coverage"] = {
            "checkedRecords": 4,
            "usageRowsWithRequestId": 4,
            "usageRowsMissingRequestId": 0,
            "matchedRequestLogRecords": 4,
            "missingRequestLogRecords": 0,
            "issues": [],
        }
        request_log = (manifest or {}).get("requestLogObservability", {})
        body["slo"] = {
            "window": request_log.get("latencySLOWindow"),
            "triggeredAlerts": request_log.get("latencySLOTriggeredAlerts"),
            "alertDelivery": request_log.get("alertDelivery"),
            "recoveryAudit": request_log.get("recoveryAudit"),
        }
    if artifact["kind"] == "provider-live-rail":
        provider_name = artifact.get("provider", "provider")
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["mode"] = "live"
        body["providerEnvironment"] = "live"
        body["proofs"] = {
            "checkout": "pass",
            "refund": "pass",
            "payout": "pass",
            "reconciliation": "pass",
        }
        body["references"] = {
            "checkout": f"{provider_name}_checkout_live_20260616",
            "refund": f"{provider_name}_refund_live_20260616",
            "payout": f"{provider_name}_payout_live_20260616",
            "reconciliation": f"{provider_name}_reconciliation_live_20260616",
        }
        body["summary"] = {
            "checkoutAttempts": 1,
            "refundAttempts": 1,
            "payoutAttempts": 1,
            "reconciliationChecks": 1,
        }
    if artifact["kind"] == "grpc-smoke-report":
        grpc_smoke_report = (manifest or {}).get("grpcSmokeReport", {})
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["result"] = "pass"
        body["smokeReportRecordedAt"] = grpc_smoke_report.get("recordedAt", "2026-06-16T00:00:00Z")
        body["timeout"] = grpc_smoke_report.get("timeout", "10s")
        body["results"] = deepcopy(grpc_smoke_report.get("results", []))
    if artifact["kind"] == "secret-audit":
        body["collectionSource"] = {
            "type": "target-url",
            "url": evidence_url(manifest, artifact["id"]),
            "collectedAt": artifact["recordedAt"],
        }
        body["result"] = "pass"
        body["scope"] = ["kubernetes", "providers", "runtime"]
        body["findings"] = []
    if artifact["kind"] == "relay-realtime-proof":
        relay_realtime = (manifest or {}).get("relayRealtime", {})
        body["mode"] = relay_realtime.get("mode", "commercial_lifecycle_enabled")
        if body["mode"] == "commercial_lifecycle_enabled":
            body["summary"] = {
                "totalRequests": 4,
                "authenticatedRequests": 4,
                "requestLinkedUsageRecords": 4,
                "priceSnapshotRecords": 4,
                "abortSettlementRecords": 1,
                "terminalUsageRecords": 4,
                "originPolicyChecks": 4,
            }
        else:
            body["summary"] = {
                "productionPolicyChecks": 1,
                "authOriginPrebillAbortUsageBlockerChecks": 5,
            }
    if artifact["kind"] == "relay-batch-proof":
        relay_batch = (manifest or {}).get("relayBatch", {})
        body["mode"] = relay_batch.get("mode", "commercial_lifecycle_enabled")
        if body["mode"] == "commercial_lifecycle_enabled":
            body["summary"] = {
                "prebillReservations": 3,
                "pollingCompletions": 3,
                "settlementRecords": 2,
                "refundRecords": 1,
                "usageAuditRecords": 3,
                "requestLogAuditRecords": 3,
                "terminalFailureRecords": 1,
            }
        else:
            body["summary"] = {
                "productionPolicyChecks": 1,
                "prebillPollingSettlementRefundAuditUsageBlockerChecks": 6,
            }
    if artifact["kind"] == "marketplace-payout-proof":
        body["summary"] = {
            "outboundDispatches": 3,
            "webhookEvents": 3,
            "settlementLedgerEntries": 3,
            "reconciledEntries": 3,
            "refundChargebackCases": 1,
            "refundChargebackCasesHandled": 1,
        }
    if artifact["kind"] == "marketplace-governance-proof":
        body["summary"] = {
            "reviewQueueItems": 4,
            "appealQueueItems": 2,
            "appealDecisions": 2,
            "reviewAssignments": 4,
            "slaChecks": 4,
            "slaBreachesHandled": 1,
            "abuseReports": 2,
            "abuseReportsResolved": 2,
        }
    if artifact["kind"] == "provider-runtime-config":
        body["providers"] = [
            {
                "name": "stripe",
                "providerEnvironment": "live",
                "providerEnv": "pass",
                "checkoutBaseUrlClass": "external-filled",
                "webhookRoute": "pass",
                "webhookVerification": "pass",
                "evidenceId": "provider_runtime_config_stripe_20260616",
            },
            {
                "name": "alipay",
                "providerEnvironment": "live",
                "providerEnv": "pass",
                "checkoutBaseUrlClass": "external-filled",
                "webhookRoute": "pass",
                "webhookVerification": "pass",
                "evidenceId": "provider_runtime_config_alipay_20260616",
            },
            {
                "name": "wechatpay",
                "providerEnvironment": "live",
                "providerEnv": "pass",
                "checkoutBaseUrlClass": "external-filled",
                "webhookRoute": "pass",
                "webhookVerification": "pass",
                "evidenceId": "provider_runtime_config_wechatpay_20260616",
            },
        ]
        body["summary"] = {
            "providersConfigured": 3,
            "providerEnvVarsChecked": 9,
            "checkoutBaseUrlsChecked": 3,
            "webhookRoutesChecked": 3,
            "webhookVerificationChecks": 3,
        }
    return body


def write_artifacts(manifest, output_dir):
    output = pathlib.Path(output_dir)
    output.mkdir(parents=True, exist_ok=True)
    for item in manifest["artifacts"]:
        path = output / f"{item['id']}.json"
        body = item.pop("_bodyOverride", None)
        if body is None:
            body = artifact_body(item, manifest)
        body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
        path.write_bytes(body_bytes)
        item["sha256"] = hashlib.sha256(body_bytes).hexdigest()


def fill_manifest(data):
    data["runId"] = "target-release-20260616"
    data["environment"]["name"] = "production-target"
    data["environment"]["class"] = "production"
    data["environment"]["baseUrl"] = "https://target.oblivious.internal"
    data["strictVerifier"]["evidenceRef"] = "artifact-strict-verifier-20260616"
    data["strictVerifier"]["startedAt"] = "2026-06-16T00:00:00Z"
    data["strictVerifier"]["completedAt"] = "2026-06-16T01:00:00Z"
    data["strictVerifier"]["targetEvidenceSha256"] = "a" * 64
    data["strictVerifier"]["artifactBundleSha256"] = "b" * 64
    data["deployment"]["evidenceRef"] = "artifact-deploy-20260616"
    data["kubernetes"]["evidenceRef"] = "artifact-k8s-20260616"
    for item in data["providers"]:
        item["providerEnvironment"] = "live"
        item["evidenceRef"] = f"artifact-provider-{item['name']}-20260616"
    grpc_ports = {"agent": "50063", "workflow": "50064", "task": "50065"}
    for item in data["grpc"]:
        service = item["service"]
        item["address"] = f"{service}.prod.oblivious.release.test:{grpc_ports[service]}"
        item["evidenceRef"] = "artifact-grpc-smoke-20260616"
    data["grpcSmokeReport"]["evidenceRef"] = "artifact-grpc-smoke-20260616"
    data["grpcSmokeReport"]["recordedAt"] = "2026-06-16T00:00:00Z"
    data["grpcSmokeReport"]["timeout"] = "10s"
    for item in data["grpcSmokeReport"]["results"]:
        service = item["service"]
        item["address"] = f"{service}.prod.oblivious.release.test:{grpc_ports[service]}"
        item["generatedClient"] = "pass"
    data["secretAudit"]["evidenceRef"] = "artifact-secret-audit-20260616"
    data["workflowTelemetry"]["window"] = "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z"
    data["workflowTelemetry"]["totalExecutions"] = 100
    data["workflowTelemetry"]["successfulExecutions"] = 99
    data["workflowTelemetry"]["failedExecutions"] = 1
    data["workflowTelemetry"]["evidenceRef"] = "artifact-workflow-telemetry-20260616"
    data["requestLogObservability"]["evidenceRef"] = "artifact-request-log-observability-20260616"
    data["requestLogObservability"]["latencySLOWindow"] = "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z"
    data["requestLogObservability"]["latencySLOTriggeredAlerts"] = 2
    data["requestLogObservability"]["alertDelivery"] = {
        "configuredProviders": 1,
        "deliveredAlerts": 2,
        "failedDeliveries": 0,
        "channels": ["pagerduty-primary"],
        "lastDeliveryId": "alert_delivery_20260616_0001",
    }
    data["requestLogObservability"]["recoveryAudit"] = {
        "auditRecords": 2,
        "failedActions": 0,
        "lastRecordId": "slo_recovery_audit_20260616_0001",
    }
    data["ragIndexing"]["evidenceRef"] = "artifact-rag-indexing-20260616"
    data["relayRealtime"] = {
        "mode": "commercial_lifecycle_enabled",
        "productionPolicyEnabled": "pass",
        "authPolicy": "pass",
        "originPolicy": "pass",
        "prebillSettlement": "pass",
        "abortSettlement": "pass",
        "usageLedger": "pass",
        "evidenceRef": "artifact-relay-realtime-20260616",
    }
    data["relayBatch"] = {
        "mode": "commercial_lifecycle_enabled",
        "productionPolicyEnabled": "pass",
        "prebillReservation": "pass",
        "pollingCompletion": "pass",
        "settlement": "pass",
        "refund": "pass",
        "usageAudit": "pass",
        "evidenceRef": "artifact-relay-batch-20260616",
    }
    data["marketplacePayouts"]["evidenceRef"] = "artifact-marketplace-payouts-20260616"
    data["marketplaceGovernance"]["evidenceRef"] = "artifact-marketplace-governance-20260616"
    data["providerRuntimeConfig"]["evidenceRef"] = "artifact-provider-runtime-config-20260616"
    data["microserviceDatabases"]["evidenceRef"] = "artifact-microservice-databases-20260616"
    data["artifacts"] = [
        {
            "id": "artifact-strict-verifier-20260616",
            "kind": "strict-verifier-log",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/strict-verifier.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
        },
        {"id": "artifact-deploy-20260616", "kind": "deployment-log", "commit": data["commit"], "runId": data["runId"], "uri": "ci://target-release/20260616/deploy.log", "recordedAt": "2026-06-16T01:00:00Z", "proofs": {"deployValidation": "pass", "backupRestore": "pass", "migrationReplay": "pass"}},
        {"id": "artifact-k8s-20260616", "kind": "kubernetes-validation", "commit": data["commit"], "runId": data["runId"], "uri": "ci://target-release/20260616/kubernetes.log", "recordedAt": "2026-06-16T01:00:00Z", "proofs": {"validation": "pass", "rollout": "pass", "failover": "pass"}},
        {"id": "artifact-provider-stripe-20260616", "kind": "provider-live-rail", "provider": "stripe", "commit": data["commit"], "runId": data["runId"], "uri": "provider://stripe/live/20260616", "recordedAt": "2026-06-16T01:00:00Z", "proofs": {"checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass"}},
        {"id": "artifact-provider-alipay-20260616", "kind": "provider-live-rail", "provider": "alipay", "commit": data["commit"], "runId": data["runId"], "uri": "provider://alipay/live/20260616", "recordedAt": "2026-06-16T01:00:00Z", "proofs": {"checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass"}},
        {"id": "artifact-provider-wechatpay-20260616", "kind": "provider-live-rail", "provider": "wechatpay", "commit": data["commit"], "runId": data["runId"], "uri": "provider://wechatpay/live/20260616", "recordedAt": "2026-06-16T01:00:00Z", "proofs": {"checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass"}},
        {"id": "artifact-grpc-smoke-20260616", "kind": "grpc-smoke-report", "commit": data["commit"], "runId": data["runId"], "uri": "ci://target-release/20260616/grpc-smoke.json", "recordedAt": "2026-06-16T01:00:00Z"},
        {"id": "artifact-secret-audit-20260616", "kind": "secret-audit", "commit": data["commit"], "runId": data["runId"], "uri": "ci://target-release/20260616/secret-audit.log", "recordedAt": "2026-06-16T01:00:00Z"},
        {"id": "artifact-workflow-telemetry-20260616", "kind": "workflow-telemetry", "commit": data["commit"], "runId": data["runId"], "uri": "observability://target/workflows/success-rate/20260616", "recordedAt": "2026-06-16T01:00:00Z"},
        {
            "id": "artifact-request-log-observability-20260616",
            "kind": "request-log-observability",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "observability://target/clickhouse/request-logs/20260616",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "clickHouseDeployment": "pass",
                "clickHouseMigration": "pass",
                "requestLogsTable": "pass",
                "ingestQuerySmoke": "pass",
                "requestUsageJoin": "pass",
                "latencySLOTrigger": "pass",
                "latencySLOAlertDelivery": "pass",
                "latencySLORecoveryAction": "pass",
            },
        },
        {
            "id": "artifact-rag-indexing-20260616",
            "kind": "rag-indexing-proof",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/rag-indexing.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "durableQueueMigration": "pass",
                "workerDeployment": "pass",
                "enqueueDrainProbe": "pass",
                "rawParserReplay": "pass",
                "retrievalProbe": "pass",
                "staleVectorFilter": "pass",
            },
        },
        {
            "id": "artifact-relay-realtime-20260616",
            "kind": "relay-realtime-proof",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/relay-realtime.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "productionPolicyEnabled": "pass",
                "authPolicy": "pass",
                "originPolicy": "pass",
                "prebillSettlement": "pass",
                "abortSettlement": "pass",
                "usageLedger": "pass",
            },
        },
        {
            "id": "artifact-relay-batch-20260616",
            "kind": "relay-batch-proof",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/relay-batch.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "productionPolicyEnabled": "pass",
                "prebillReservation": "pass",
                "pollingCompletion": "pass",
                "settlement": "pass",
                "refund": "pass",
                "usageAudit": "pass",
            },
        },
        {
            "id": "artifact-marketplace-payouts-20260616",
            "kind": "marketplace-payout-proof",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "provider://marketplace-payouts/webhook/20260616",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "outboundDispatch": "pass",
                "inboundWebhookLifecycle": "pass",
                "settlementLedger": "pass",
                "reconciliation": "pass",
                "refundChargebackHandling": "pass",
            },
        },
        {
            "id": "artifact-marketplace-governance-20260616",
            "kind": "marketplace-governance-proof",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/marketplace-governance.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "reviewQueue": "pass",
                "appealQueue": "pass",
                "appealDecisionLifecycle": "pass",
                "reviewAssignment": "pass",
                "reviewSLAEnforcement": "pass",
                "abuseReportLifecycle": "pass",
            },
        },
        {
            "id": "artifact-provider-runtime-config-20260616",
            "kind": "provider-runtime-config",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/provider-runtime-config.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
                "stripe": "pass",
                "alipay": "pass",
                "wechatpay": "pass",
                "providerEnv": "pass",
                "checkoutBaseUrls": "pass",
                "webhookRoutes": "pass",
                "webhookVerification": "pass",
            },
        },
        {
            "id": "artifact-microservice-databases-20260616",
            "kind": "microservice-database-proof",
            "commit": data["commit"],
            "runId": data["runId"],
            "uri": "ci://target-release/20260616/microservice-databases.log",
            "recordedAt": "2026-06-16T01:00:00Z",
            "proofs": {
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
            },
        },
    ]
    for index, item in enumerate(data["artifacts"], start=1):
        item.setdefault("sha256", f"{index:064x}")


def mutate(data, label):
    if label == "missing-artifacts-index":
        data.pop("artifacts", None)
    elif label == "missing-run-lineage":
        data.pop("runId", None)
    elif label == "dangling-strict-verifier-evidence-ref":
        data["strictVerifier"]["evidenceRef"] = "artifact-missing-20260616"
    elif label == "mismatched-strict-verifier-artifact-kind":
        data["strictVerifier"]["evidenceRef"] = "artifact-provider-stripe-20260616"
        provider(data, "stripe")["evidenceRef"] = "artifact-strict-verifier-20260616"
    elif label == "mismatched-artifact-commit":
        artifact(data, "artifact-deploy-20260616")["commit"] = "0000000000000000000000000000000000000000"
    elif label == "mismatched-artifact-run-id":
        artifact(data, "artifact-deploy-20260616")["runId"] = "target-release-20260615"
    elif label == "invalid-environment-class":
        data["environment"]["class"] = "developer laptop"
    elif label == "non-production-target-final":
        data["environment"]["class"] = "staging"
    elif label == "placeholder-environment-base-url":
        data["environment"]["baseUrl"] = "TODO-target-base-url"
    elif label == "invalid-environment-base-url":
        data["environment"]["baseUrl"] = "not-a-url"
    elif label == "loopback-environment-base-url":
        data["environment"]["baseUrl"] = "http://localhost:3000"
    elif label == "abbreviated-loopback-environment-base-url":
        data["environment"]["baseUrl"] = "http://127.1:3000"
    elif label == "octal-loopback-environment-base-url":
        data["environment"]["baseUrl"] = "http://0177.0.0.1:3000"
    elif label == "hex-loopback-environment-base-url":
        data["environment"]["baseUrl"] = "http://0x7f.0.0.1:3000"
    elif label == "fake-environment-base-url":
        data["environment"]["baseUrl"] = "https://fake-target.invalid"
    elif label == "reserved-invalid-environment-base-url":
        data["environment"]["baseUrl"] = "https://target.invalid"
    elif label == "loopback-environment-trailing-dot":
        data["environment"]["baseUrl"] = "http://localhost.:3000"
    elif label == "ipv6-mapped-loopback-environment":
        data["environment"]["baseUrl"] = "http://[::ffff:127.0.0.1]:3000"
    elif label == "ipv6-unspecified-environment-base-url":
        data["environment"]["baseUrl"] = "http://[::]:3000"
    elif label == "zero-host-environment-base-url":
        data["environment"]["baseUrl"] = "http://0:3000"
    elif label == "credential-environment-base-url-userinfo":
        data["environment"]["baseUrl"] = "https://target-user:target-password@staging.oblivious.internal"
    elif label == "secret-environment-base-url-query":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal?token=target-secret-token"
    elif label == "secret-name-only-environment-base-url-query":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal?token"
    elif label == "password-environment-base-url-query":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal?password=target-secret-password"
    elif label == "encoded-password-environment-base-url-query":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal?pass%77ord=target-secret-password"
    elif label == "double-encoded-password-environment-base-url-query":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal?pass%2577ord=target-secret-password"
    elif label == "secret-value-environment-base-url-query":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal?evidence=target-secret-token"
    elif label == "secret-environment-base-url-fragment":
        data["environment"]["baseUrl"] = "https://staging.oblivious.internal#token=target-secret-token"
    elif label == "duplicate-artifact-id":
        data["artifacts"].append(deepcopy(data["artifacts"][0]))
    elif label == "placeholder-artifact-id":
        data["artifacts"][0]["id"] = "TODO-strict-verifier-log"
    elif label == "placeholder-artifact-kind":
        data["artifacts"][0]["kind"] = "TODO-log-kind"
    elif label == "placeholder-artifact-uri":
        artifact(data, "artifact-deploy-20260616")["uri"] = "/path/outside/git/deploy.log"
    elif label in ("secret-artifact-uri-query", "password-artifact-uri-query", "encoded-token-artifact-uri-query", "double-encoded-token-artifact-uri-query", "secret-value-artifact-uri-query", "secret-artifact-uri-fragment", "secret-name-only-artifact-uri-fragment", "credential-artifact-uri-userinfo", "local-artifact-uri", "file-artifact-uri", "loopback-artifact-uri", "abbreviated-loopback-artifact-uri", "octal-loopback-artifact-uri", "hex-loopback-artifact-uri", "ipv6-mapped-loopback-artifact-uri", "zero-host-artifact-uri", "fake-artifact-uri", "reserved-invalid-artifact-uri", "inline-artifact-uri"):
        values = {
            "secret-artifact-uri-query": "https://ci.internal/runs/20260616/deploy.log?token=target-secret-token",
            "password-artifact-uri-query": "https://ci.internal/runs/20260616/deploy.log?password=target-secret-password",
            "encoded-token-artifact-uri-query": "https://ci.internal/runs/20260616/deploy.log?%74oken=target-secret-token",
            "double-encoded-token-artifact-uri-query": "https://ci.internal/runs/20260616/deploy.log?t%256fken=target-secret-token",
            "secret-value-artifact-uri-query": "https://ci.internal/runs/20260616/deploy.log?evidence=target-secret-token",
            "secret-artifact-uri-fragment": "https://ci.internal/runs/20260616/deploy.log#token=target-secret-token",
            "secret-name-only-artifact-uri-fragment": "https://ci.internal/runs/20260616/deploy.log#password",
            "credential-artifact-uri-userinfo": "https://target-user:target-password@ci.internal/runs/20260616/deploy.log",
            "local-artifact-uri": "/tmp/target-release/deploy.log",
            "file-artifact-uri": "file:///tmp/target-release/deploy.log",
            "loopback-artifact-uri": "http://localhost:8080/target-release/deploy.log",
            "abbreviated-loopback-artifact-uri": "http://127.1:8080/target-release/deploy.log",
            "octal-loopback-artifact-uri": "http://0177.0.0.1:8080/target-release/deploy.log",
            "hex-loopback-artifact-uri": "http://0x7f.0.0.1:8080/target-release/deploy.log",
            "ipv6-mapped-loopback-artifact-uri": "http://[::ffff:127.0.0.1]:8080/target-release/deploy.log",
            "zero-host-artifact-uri": "http://0:8080/target-release/deploy.log",
            "fake-artifact-uri": "https://fake-artifacts.invalid/target-release/deploy.log",
            "reserved-invalid-artifact-uri": "https://artifacts.invalid/target-release/deploy.log",
            "inline-artifact-uri": "data:text/plain,target-release-log",
        }
        artifact(data, "artifact-deploy-20260616")["uri"] = values[label]
    elif label == "secret-audit-embedded-token":
        data["secretAudit"]["apiToken"] = "target-secret-token"
    elif label == "incomplete-secret-audit-scope":
        data["secretAudit"]["scope"] = ["runtime"]
    elif label == "invalid-artifact-recorded-at":
        data["artifacts"][0]["recordedAt"] = "2026/06/16 01:00"
    elif label == "invalid-artifact-sha256":
        data["artifacts"][0]["sha256"] = "not-a-digest"
    elif label == "missing-artifact-sha256":
        artifact(data, "artifact-deploy-20260616").pop("sha256", None)
    elif label == "unused-artifact-id":
        add_unused_artifact(data, False)
    elif label == "unused-artifact-masked-by-freeform-evidence-ref":
        add_unused_artifact(data, True)
        data["notes"] = {"evidenceRef": "artifact-unused-20260616"}
    elif label == "missing-providers-collection":
        data.pop("providers", None)
    elif label == "missing-wechatpay-provider":
        data["providers"] = [item for item in data["providers"] if item["name"] != "wechatpay"]
    elif label == "unknown-provider-live-rail":
        data["providers"].append({"name": "paypal", "mode": "live", "providerEnvironment": "live", "checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass", "evidenceRef": "artifact-provider-paypal-20260616"})
        data["artifacts"].append({"id": "artifact-provider-paypal-20260616", "kind": "provider-live-rail", "provider": "paypal", "commit": data["commit"], "runId": data["runId"], "uri": "provider://paypal/live/20260616", "recordedAt": "2026-06-16T01:00:00Z"})
    elif label == "swapped-provider-evidence-ref":
        stripe = provider(data, "stripe")
        alipay = provider(data, "alipay")
        stripe["evidenceRef"], alipay["evidenceRef"] = alipay["evidenceRef"], stripe["evidenceRef"]
    elif label == "provider-live-rail-not-live-environment":
        provider(data, "stripe")["providerEnvironment"] = "sandbox"
    elif label == "missing-strict-k8s-flag":
        data["strictVerifier"]["command"] = " ".join(token for token in data["strictVerifier"]["command"].split() if token != "COMMERCIAL_COMPLETION_RUN_K8S=true")
    elif label == "strict-command-mask-or-true":
        data["strictVerifier"]["command"] += " || true"
    elif label == "strict-command-env-quoted-skip-flags-as-args":
        data["strictVerifier"]["command"] = "env 'COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true' " + data["strictVerifier"]["command"] + " COMMERCIAL_COMPLETION_RUN_DEPLOY=true"
    elif label == "quoted-env-skip-command":
        data["strictVerifier"]["command"] = "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS='true' " + data["strictVerifier"]["command"]
    elif label == "ansi-c-quoted-env-skip-command":
        data["strictVerifier"]["command"] = "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=$'true' " + data["strictVerifier"]["command"]
    elif label == "commit-mismatch-override-command":
        data["strictVerifier"]["command"] = "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true " + data["strictVerifier"]["command"]
    elif label == "ansi-c-quoted-commit-mismatch-command":
        data["strictVerifier"]["command"] = "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=$'true' " + data["strictVerifier"]["command"]
    elif label == "missing-strict-target-evidence-sha":
        data["strictVerifier"].pop("targetEvidenceSha256", None)
    elif label == "invalid-strict-artifact-bundle-sha":
        data["strictVerifier"]["artifactBundleSha256"] = "not-a-sha"
    elif label == "missing-strict-verifier-evidence-ref":
        data["strictVerifier"].pop("evidenceRef", None)
    elif label == "inverted-strict-verifier-window":
        data["strictVerifier"]["completedAt"] = "2026-06-15T23:59:00Z"
    elif label == "strict-verifier-artifact-outside-window":
        artifact(data, data["strictVerifier"]["evidenceRef"])["recordedAt"] = "2026-06-15T23:59:00Z"
    elif label == "unfilled-kubernetes-secret-file-class":
        data["kubernetes"]["secretFileClass"] = "external-empty"
    elif label == "deployment-artifact-missing-backup-restore-proof":
        artifact(data, "artifact-deploy-20260616").setdefault("proofs", {}).pop("backupRestore", None)
    elif label == "kubernetes-artifact-missing-failover-proof":
        artifact(data, "artifact-k8s-20260616").setdefault("proofs", {}).pop("failover", None)
    elif label == "artifact-bundle-deployment-not-production":
        body = artifact_body(artifact(data, "artifact-deploy-20260616"), data)
        body["targetEnvironment"] = "staging"
        artifact(data, "artifact-deploy-20260616")["_bodyOverride"] = body
    elif label == "artifact-bundle-deployment-missing-reference":
        body = artifact_body(artifact(data, "artifact-deploy-20260616"), data)
        body["references"].pop("backupRestore", None)
        artifact(data, "artifact-deploy-20260616")["_bodyOverride"] = body
    elif label == "artifact-bundle-kubernetes-missing-cluster-ref":
        body = artifact_body(artifact(data, "artifact-k8s-20260616"), data)
        body.pop("clusterRef", None)
        artifact(data, "artifact-k8s-20260616")["_bodyOverride"] = body
    elif label == "artifact-bundle-kubernetes-missing-reference":
        body = artifact_body(artifact(data, "artifact-k8s-20260616"), data)
        body["references"].pop("failover", None)
        artifact(data, "artifact-k8s-20260616")["_bodyOverride"] = body
    elif label == "provider-live-rail-artifact-missing-payout-proof":
        artifact(data, "artifact-provider-stripe-20260616").setdefault("proofs", {}).pop("payout", None)
    elif label == "invalid-workflow-telemetry-window":
        data["workflowTelemetry"]["window"] = "not-a-window"
    elif label == "inverted-workflow-telemetry-window":
        data["workflowTelemetry"]["window"] = "2026-06-16T01:00:00Z/2026-06-16T00:00:00Z"
    elif label == "workflow-telemetry-success-rate-above-one":
        data["workflowTelemetry"]["successRate"] = 1.01
    elif label == "missing-workflow-telemetry-total-executions":
        data["workflowTelemetry"].pop("totalExecutions", None)
    elif label == "workflow-telemetry-count-mismatch":
        data["workflowTelemetry"]["failedExecutions"] = 0
    elif label == "workflow-telemetry-success-rate-count-mismatch":
        data["workflowTelemetry"]["successfulExecutions"] = 98
        data["workflowTelemetry"]["failedExecutions"] = 2
    elif label == "missing-request-log-observability":
        data.pop("requestLogObservability", None)
    elif label == "request-log-observability-not-clickhouse":
        data["requestLogObservability"]["backend"] = "postgres"
    elif label == "failed-clickhouse-migration-proof":
        data["requestLogObservability"]["clickHouseMigration"] = "fail"
    elif label == "failed-request-log-usage-join-proof":
        data["requestLogObservability"]["requestUsageJoin"] = "fail"
    elif label == "request-log-observability-artifact-missing-usage-join-proof":
        artifact(data, "artifact-request-log-observability-20260616")["proofs"].pop("requestUsageJoin", None)
    elif label == "missing-latency-slo-trigger-proof":
        data["requestLogObservability"].pop("latencySLOTrigger", None)
    elif label == "failed-latency-slo-alert-delivery-proof":
        data["requestLogObservability"]["latencySLOAlertDelivery"] = "fail"
    elif label == "failed-latency-slo-recovery-action-proof":
        data["requestLogObservability"]["latencySLORecoveryAction"] = "fail"
    elif label == "invalid-latency-slo-window":
        data["requestLogObservability"]["latencySLOWindow"] = "not-a-window"
    elif label == "missing-latency-slo-alert-delivery":
        data["requestLogObservability"].pop("alertDelivery", None)
    elif label == "failed-latency-slo-alert-delivery-count":
        data["requestLogObservability"]["alertDelivery"]["failedDeliveries"] = 1
    elif label == "missing-latency-slo-alert-channel":
        data["requestLogObservability"]["alertDelivery"]["channels"] = []
    elif label == "sample-latency-slo-delivery-id":
        data["requestLogObservability"]["alertDelivery"]["lastDeliveryId"] = "sample-alert-id"
    elif label == "fake-latency-slo-recovery-record-id":
        data["requestLogObservability"]["recoveryAudit"]["lastRecordId"] = "fake-record-id"
    elif label == "failed-latency-slo-recovery-audit-count":
        data["requestLogObservability"]["recoveryAudit"]["failedActions"] = 1
    elif label == "request-log-observability-kind-mismatch":
        data["requestLogObservability"]["evidenceRef"] = "artifact-rag-indexing-20260616"
        data["ragIndexing"]["evidenceRef"] = "artifact-request-log-observability-20260616"
    elif label == "missing-rag-indexing-proof":
        data.pop("ragIndexing", None)
    elif label == "failed-rag-worker-deployment-proof":
        data["ragIndexing"]["workerDeployment"] = "fail"
    elif label == "failed-rag-raw-parser-replay-proof":
        data["ragIndexing"]["rawParserReplay"] = "fail"
    elif label == "failed-rag-stale-vector-filter-proof":
        data["ragIndexing"]["staleVectorFilter"] = "fail"
    elif label == "rag-indexing-artifact-missing-raw-parser-proof":
        artifact(data, "artifact-rag-indexing-20260616").setdefault("proofs", {}).pop("rawParserReplay", None)
    elif label == "missing-relay-realtime-proof":
        data.pop("relayRealtime", None)
    elif label == "relay-realtime-enabled-without-lifecycle":
        data["relayRealtime"]["mode"] = "enabled"
    elif label == "relay-realtime-disabled-final":
        data["relayRealtime"] = {
            "mode": "disabled_until_commercial_lifecycle",
            "productionPolicyDisabled": "pass",
            "authOriginPrebillAbortUsageBlockers": "pass",
            "evidenceRef": "artifact-relay-realtime-20260616",
        }
        artifact(data, "artifact-relay-realtime-20260616")["proofs"] = {
            "productionPolicyDisabled": "pass",
            "authOriginPrebillAbortUsageBlockers": "pass",
        }
    elif label == "failed-relay-realtime-auth-policy-proof":
        data["relayRealtime"]["authPolicy"] = "fail"
    elif label == "failed-relay-realtime-usage-ledger-proof":
        data["relayRealtime"]["usageLedger"] = "fail"
    elif label == "relay-realtime-artifact-missing-auth-policy-proof":
        artifact(data, "artifact-relay-realtime-20260616").setdefault("proofs", {}).pop("authPolicy", None)
    elif label == "relay-realtime-kind-mismatch":
        data["relayRealtime"]["evidenceRef"] = "artifact-rag-indexing-20260616"
        data["ragIndexing"]["evidenceRef"] = "artifact-relay-realtime-20260616"
    elif label == "missing-relay-batch-proof":
        data.pop("relayBatch", None)
    elif label == "relay-batch-enabled-without-lifecycle":
        data["relayBatch"]["mode"] = "enabled"
    elif label == "relay-batch-disabled-final":
        data["relayBatch"] = {
            "mode": "disabled_until_commercial_lifecycle",
            "productionPolicyDisabled": "pass",
            "prebillPollingSettlementRefundAuditUsageBlockers": "pass",
            "evidenceRef": "artifact-relay-batch-20260616",
        }
        artifact(data, "artifact-relay-batch-20260616")["proofs"] = {
            "productionPolicyDisabled": "pass",
            "prebillPollingSettlementRefundAuditUsageBlockers": "pass",
        }
    elif label == "failed-relay-batch-prebill-proof":
        data["relayBatch"]["prebillReservation"] = "fail"
    elif label == "failed-relay-batch-usage-audit-proof":
        data["relayBatch"]["usageAudit"] = "fail"
    elif label == "relay-batch-artifact-missing-prebill-proof":
        artifact(data, "artifact-relay-batch-20260616").setdefault("proofs", {}).pop("prebillReservation", None)
    elif label == "relay-batch-kind-mismatch":
        data["relayBatch"]["evidenceRef"] = "artifact-rag-indexing-20260616"
        data["ragIndexing"]["evidenceRef"] = "artifact-relay-batch-20260616"
    elif label == "missing-marketplace-payout-proof":
        data.pop("marketplacePayouts", None)
    elif label == "marketplace-payout-provider-not-webhook":
        data["marketplacePayouts"]["providerMode"] = "local"
    elif label == "failed-marketplace-payout-webhook-lifecycle":
        data["marketplacePayouts"]["inboundWebhookLifecycle"] = "fail"
    elif label == "missing-marketplace-payout-refund-chargeback-proof":
        data["marketplacePayouts"].pop("refundChargebackHandling", None)
    elif label == "marketplace-payout-artifact-missing-refund-chargeback-proof":
        artifact(data, "artifact-marketplace-payouts-20260616").setdefault("proofs", {}).pop("refundChargebackHandling", None)
    elif label == "marketplace-payout-kind-mismatch":
        data["marketplacePayouts"]["evidenceRef"] = "artifact-workflow-telemetry-20260616"
        data["workflowTelemetry"]["evidenceRef"] = "artifact-marketplace-payouts-20260616"
    elif label == "missing-marketplace-governance-proof":
        data.pop("marketplaceGovernance", None)
    elif label == "failed-marketplace-governance-review-assignment":
        data["marketplaceGovernance"]["reviewAssignment"] = "fail"
    elif label == "marketplace-governance-artifact-missing-review-sla-proof":
        artifact(data, "artifact-marketplace-governance-20260616").setdefault("proofs", {}).pop("reviewSLAEnforcement", None)
    elif label == "marketplace-governance-kind-mismatch":
        data["marketplaceGovernance"]["evidenceRef"] = "artifact-marketplace-payouts-20260616"
        data["marketplacePayouts"]["evidenceRef"] = "artifact-marketplace-governance-20260616"
    elif label == "missing-provider-runtime-config-proof":
        data.pop("providerRuntimeConfig", None)
    elif label == "failed-alipay-runtime-config-proof":
        data["providerRuntimeConfig"]["alipay"] = "fail"
    elif label == "failed-provider-webhook-routes-proof":
        data["providerRuntimeConfig"]["webhookRoutes"] = "fail"
    elif label == "provider-runtime-config-artifact-missing-webhook-verification-proof":
        artifact(data, "artifact-provider-runtime-config-20260616").setdefault("proofs", {}).pop("webhookVerification", None)
    elif label == "provider-runtime-config-artifact-missing-wechatpay-detail":
        body = artifact_body(artifact(data, "artifact-provider-runtime-config-20260616"), data)
        body["providers"] = [item for item in body["providers"] if item["name"] != "wechatpay"]
        body["summary"]["providersConfigured"] = 2
        artifact(data, "artifact-provider-runtime-config-20260616")["_bodyOverride"] = body
    elif label == "provider-runtime-config-kind-mismatch":
        data["providerRuntimeConfig"]["evidenceRef"] = "artifact-marketplace-payouts-20260616"
        data["marketplacePayouts"]["evidenceRef"] = "artifact-provider-runtime-config-20260616"
    elif label == "missing-microservice-database-proof":
        data.pop("microserviceDatabases", None)
    elif label == "monolith-microservice-database-proof":
        data["microserviceDatabases"]["mode"] = "monolith"
    elif label == "unfilled-microservice-database-url-class":
        data["microserviceDatabases"]["serviceUrlClass"] = "example"
    elif label == "failed-rag-microservice-database-proof":
        data["microserviceDatabases"]["rag"] = "fail"
    elif label == "failed-agent-microservice-database-proof":
        data["microserviceDatabases"]["agent"] = "fail"
    elif label == "microservice-database-artifact-missing-agent-proof":
        artifact(data, "artifact-microservice-databases-20260616").setdefault("proofs", {}).pop("agent", None)
    elif label == "microservice-database-artifact-missing-observability-detail":
        body = artifact_body(artifact(data, "artifact-microservice-databases-20260616"), data)
        body["services"] = [
            item for item in body["services"] if item["name"] != "observability"
        ]
        body["summary"]["servicesChecked"] = 10
        artifact(data, "artifact-microservice-databases-20260616")["_bodyOverride"] = body
    elif label == "microservice-database-kind-mismatch":
        data["microserviceDatabases"]["evidenceRef"] = "artifact-provider-runtime-config-20260616"
        data["providerRuntimeConfig"]["evidenceRef"] = "artifact-microservice-databases-20260616"
    elif label == "missing-grpc-collection":
        data.pop("grpc", None)
    elif label == "unknown-grpc-service":
        data["grpc"].append({"service": "admin", "address": "admin:50066", "generatedClient": "pass", "evidenceRef": "artifact-grpc-smoke-20260616"})
        data["grpcSmokeReport"]["results"].append({"service": "admin", "address": "admin:50066", "generatedClient": "pass", "status": "validation_response"})
    elif label == "unknown-grpc-smoke-service":
        data["grpcSmokeReport"]["results"].append({"service": "admin", "address": "admin:50066", "generatedClient": "pass", "status": "validation_response"})
    elif label in ("loopback-grpc-address", "abbreviated-loopback-grpc-address", "octal-loopback-grpc-address", "hex-loopback-grpc-address", "ipv6-mapped-loopback-grpc-address", "zero-host-grpc-address", "fake-grpc-address", "reserved-invalid-grpc-address", "url-shaped-grpc-address"):
        values = {
            "loopback-grpc-address": "localhost:50063",
            "abbreviated-loopback-grpc-address": "127.1:50063",
            "octal-loopback-grpc-address": "0177.0.0.1:50063",
            "hex-loopback-grpc-address": "0x7f.0.0.1:50063",
            "ipv6-mapped-loopback-grpc-address": "[::ffff:127.0.0.1]:50063",
            "zero-host-grpc-address": "0:50063",
            "fake-grpc-address": "fake-target.invalid:50063",
            "reserved-invalid-grpc-address": "target.invalid:50063",
            "url-shaped-grpc-address": "https://agent.target.internal:50063",
        }
        grpc_entry(data, "agent")["address"] = values[label]
        grpc_result(data, "agent")["address"] = values[label]
    elif label == "missing-grpc-smoke-report":
        data.pop("grpcSmokeReport", None)
    elif label == "invalid-grpc-smoke-timeout":
        data["grpcSmokeReport"]["timeout"] = "not-a-duration"
    elif label == "failed-grpc-smoke-result":
        grpc_result(data, "task")["generatedClient"] = "fail"
    elif label == "workflow-grpc-smoke-status-mismatch":
        grpc_result(data, "workflow")["status"] = "validation_error"
    elif label == "mismatched-grpc-smoke-evidence-ref":
        grpc_entry(data, "agent")["evidenceRef"] = "artifact-agent-only"
    elif label == "grpc-smoke-artifact-before-report":
        data["grpcSmokeReport"]["recordedAt"] = "2026-06-16T02:00:00Z"
    elif label == "workflow-telemetry-artifact-before-window-end":
        data["workflowTelemetry"]["window"] = "2026-06-16T00:00:00Z/2026-06-16T02:00:00Z"
    else:
        raise SystemExit(f"unknown target evidence fixture mutation: {label}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--fill", action="store_true")
    parser.add_argument("--mutate")
    parser.add_argument("--write-artifacts", action="store_true")
    parser.add_argument("path")
    parser.add_argument("output_dir", nargs="?")
    args = parser.parse_args()

    with open(args.path, "r", encoding="utf-8") as handle:
        data = json.load(handle)
    if args.fill:
        fill_manifest(data)
    elif args.mutate:
        mutate(data, args.mutate)
    elif args.write_artifacts:
        if not args.output_dir:
            raise SystemExit("--write-artifacts requires output_dir")
        write_artifacts(data, args.output_dir)
    else:
        raise SystemExit("--fill or --mutate is required")
    with open(args.path, "w", encoding="utf-8") as handle:
        json.dump(data, handle, indent=2)
        handle.write("\n")


if __name__ == "__main__":
    raise SystemExit(main())
