#!/usr/bin/env python3
import argparse
import json
import os
import re
import sys
from datetime import datetime


STRICT_VERIFIER_COMMAND = (
    "COMMERCIAL_COMPLETION_RUN_DEPLOY=true "
    "COMMERCIAL_COMPLETION_RUN_K8S=true "
    "COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true "
    "COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true "
    "bash scripts/verify-commercial-completion.sh"
)
ALLOWED_ENVIRONMENT_CLASSES = {"staging", "preproduction", "production"}
DISABLED_MODE = "disabled_until_commercial_lifecycle"
LIVE_MODE = "commercial_lifecycle_enabled"
DEPLOYMENT_PROOF_FIELDS = [
    "deployValidation",
    "backupRestore",
    "migrationReplay",
]
DEPLOYMENT_REFERENCE_FIELDS = DEPLOYMENT_PROOF_FIELDS
KUBERNETES_PROOF_FIELDS = [
    "validation",
    "rollout",
    "failover",
]
KUBERNETES_REFERENCE_FIELDS = KUBERNETES_PROOF_FIELDS
PROVIDER_LIVE_RAIL_PROOF_FIELDS = [
    "checkout",
    "refund",
    "payout",
    "reconciliation",
]
PROVIDER_LIVE_RAIL_REFERENCE_FIELDS = PROVIDER_LIVE_RAIL_PROOF_FIELDS
REFERENCE_PLACEHOLDER_PATTERN = re.compile(r"TODO|TBD|placeholder|example|sample|fake", re.IGNORECASE)
EMBEDDED_SECRET_PATTERN = re.compile(
    r"sk_(live|test)_[A-Za-z0-9]{12,}|rk_(live|test)_[A-Za-z0-9]{12,}|"
    r"AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|"
    r"gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}"
)
PAYMENT_PROVIDERS = ["stripe", "alipay", "wechatpay"]
SECRET_AUDIT_REQUIRED_SCOPES = ["kubernetes", "providers", "runtime"]
SECRET_AUDIT_SUMMARY_COUNT_FIELDS = [
    "totalRecordsScanned",
    "protectedRecords",
    "plaintextRecords",
    "invalidProtectedRecords",
    "rotationRequiredRecords",
]
REQUEST_LOG_PLATFORM_PROOF_FIELDS = [
    "clickHouseDeployment",
    "clickHouseMigration",
    "requestLogsTable",
    "ingestQuerySmoke",
]
REQUEST_LOG_SLO_PROOF_FIELDS = [
    "latencySLOTrigger",
    "latencySLOAlertDelivery",
    "latencySLORecoveryAction",
]
RAG_INDEXING_PROOF_FIELDS = [
    "durableQueueMigration",
    "workerDeployment",
    "enqueueDrainProbe",
    "rawParserReplay",
    "retrievalProbe",
    "staleVectorFilter",
]
RELAY_REALTIME_DISABLED_PROOF_FIELDS = [
    "productionPolicyDisabled",
    "authOriginPrebillAbortUsageBlockers",
]
RELAY_REALTIME_LIVE_PROOF_FIELDS = [
    "productionPolicyEnabled",
    "authPolicy",
    "originPolicy",
    "prebillSettlement",
    "abortSettlement",
    "usageLedger",
]
RELAY_BATCH_DISABLED_PROOF_FIELDS = [
    "productionPolicyDisabled",
    "prebillPollingSettlementRefundAuditUsageBlockers",
]
RELAY_BATCH_LIVE_PROOF_FIELDS = [
    "productionPolicyEnabled",
    "prebillReservation",
    "pollingCompletion",
    "settlement",
    "refund",
    "usageAudit",
]
MARKETPLACE_PAYOUT_PROOF_FIELDS = [
    "outboundDispatch",
    "inboundWebhookLifecycle",
    "settlementLedger",
    "reconciliation",
    "refundChargebackHandling",
]
MARKETPLACE_GOVERNANCE_PROOF_FIELDS = [
    "reviewQueue",
    "appealQueue",
    "appealDecisionLifecycle",
    "reviewAssignment",
    "reviewSLAEnforcement",
    "abuseReportLifecycle",
]
PROVIDER_RUNTIME_CONFIG_PROOF_FIELDS = [
    "stripe",
    "alipay",
    "wechatpay",
    "providerEnv",
    "checkoutBaseUrls",
    "webhookRoutes",
    "webhookVerification",
]
PROVIDER_RUNTIME_CONFIG_REQUIRED_PROVIDERS = ["stripe", "alipay", "wechatpay"]
MICROSERVICE_DATABASE_SERVICE_FIELDS = [
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
MICROSERVICE_DATABASE_PROOF_FIELDS = MICROSERVICE_DATABASE_SERVICE_FIELDS + [
    "migrationReadiness",
]


REQUIRED_ENV = [
    "OBLIVIOUS_TARGET_EVIDENCE_RUN_ID",
    "OBLIVIOUS_TARGET_ENVIRONMENT_NAME",
    "OBLIVIOUS_TARGET_ENVIRONMENT_CLASS",
    "OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL",
    "OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT",
    "OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT",
    "OBLIVIOUS_TARGET_STRICT_VERIFIER_URI",
    "OBLIVIOUS_TARGET_DEPLOYMENT_URI",
    "OBLIVIOUS_TARGET_KUBERNETES_URI",
    "OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI",
    "OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI",
    "OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI",
    "OBLIVIOUS_TARGET_GRPC_SMOKE_URI",
    "OBLIVIOUS_TARGET_SECRET_AUDIT_URI",
    "OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE",
    "OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW",
    "OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI",
    "OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_URI",
    "OBLIVIOUS_TARGET_RAG_INDEXING_URI",
    "OBLIVIOUS_TARGET_RELAY_REALTIME_URI",
    "OBLIVIOUS_TARGET_RELAY_BATCH_URI",
    "OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_URI",
    "OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI",
    "OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_URI",
    "OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_URI",
]


ARTIFACT_SPECS = [
    ("strict-verifier-log", "strict-verifier-log", "OBLIVIOUS_TARGET_STRICT_VERIFIER_URI"),
    ("deployment-log", "deployment-log", "OBLIVIOUS_TARGET_DEPLOYMENT_URI"),
    ("kubernetes-validation", "kubernetes-validation", "OBLIVIOUS_TARGET_KUBERNETES_URI"),
    ("stripe-provider-live-rail", "provider-live-rail", "OBLIVIOUS_TARGET_STRIPE_PROVIDER_URI", "stripe"),
    ("alipay-provider-live-rail", "provider-live-rail", "OBLIVIOUS_TARGET_ALIPAY_PROVIDER_URI", "alipay"),
    ("wechatpay-provider-live-rail", "provider-live-rail", "OBLIVIOUS_TARGET_WECHATPAY_PROVIDER_URI", "wechatpay"),
    ("grpc-smoke-report", "grpc-smoke-report", "OBLIVIOUS_TARGET_GRPC_SMOKE_URI"),
    ("secret-audit", "secret-audit", "OBLIVIOUS_TARGET_SECRET_AUDIT_URI"),
    ("workflow-telemetry", "workflow-telemetry", "OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_URI"),
    ("request-log-observability", "request-log-observability", "OBLIVIOUS_TARGET_REQUEST_LOG_OBSERVABILITY_URI"),
    ("rag-indexing-proof", "rag-indexing-proof", "OBLIVIOUS_TARGET_RAG_INDEXING_URI"),
    ("relay-realtime-proof", "relay-realtime-proof", "OBLIVIOUS_TARGET_RELAY_REALTIME_URI"),
    ("relay-batch-proof", "relay-batch-proof", "OBLIVIOUS_TARGET_RELAY_BATCH_URI"),
    ("marketplace-payout-proof", "marketplace-payout-proof", "OBLIVIOUS_TARGET_MARKETPLACE_PAYOUT_URI"),
    ("marketplace-governance-proof", "marketplace-governance-proof", "OBLIVIOUS_TARGET_MARKETPLACE_GOVERNANCE_URI"),
    ("provider-runtime-config", "provider-runtime-config", "OBLIVIOUS_TARGET_PROVIDER_RUNTIME_CONFIG_URI"),
    ("microservice-database-proof", "microservice-database-proof", "OBLIVIOUS_TARGET_MICROSERVICE_DATABASE_URI"),
]

REQUIRED_ARTIFACT_SHA_ENV = [spec[2].replace("_URI", "_SHA256") for spec in ARTIFACT_SPECS]


def require_env(name):
    value = os.environ.get(name, "").strip()
    if value == "":
        raise ValueError(f"{name} is required")
    return value


def require_sha256_value(value, label):
    if not isinstance(value, str) or value.strip() == "":
        raise ValueError(f"{label} is required")
    value = value.strip().lower()
    if not re.fullmatch(r"[0-9a-f]{64}", value):
        raise ValueError(f"{label} must be a 64-character SHA-256 hex digest")
    return value


def artifact_id(run_id, suffix):
    return f"{run_id}-{suffix}"


def artifact_recorded_at(kind, completed_at, smoke_recorded_at):
    specific_name = "OBLIVIOUS_TARGET_" + kind.upper().replace("-", "_") + "_RECORDED_AT"
    if os.environ.get(specific_name, "").strip():
        return os.environ[specific_name].strip()
    if kind == "grpc-smoke-report":
        return os.environ.get("OBLIVIOUS_TARGET_GRPC_SMOKE_RECORDED_AT", smoke_recorded_at).strip()
    return os.environ.get("OBLIVIOUS_TARGET_ARTIFACT_RECORDED_AT", completed_at).strip()


def load_grpc_smoke_report(path):
    with open(path, "r", encoding="utf-8") as handle:
        report = json.load(handle)
    if not isinstance(report, dict):
        raise ValueError("gRPC smoke report must be a JSON object")
    results = report.get("results")
    if not isinstance(results, list):
        raise ValueError("gRPC smoke report results must be an array")
    return report


def load_json_object(label, path):
    with open(path, "r", encoding="utf-8") as handle:
        payload = json.load(handle)
    if not isinstance(payload, dict):
        raise ValueError(f"{label} must be a JSON object")
    return payload


def require_iso8601_value(value, label):
    if not isinstance(value, str) or value.strip() == "":
        raise ValueError(f"{label} is required")
    normalized = value.strip()
    try:
        parsed = datetime.fromisoformat(normalized.replace("Z", "+00:00"))
    except ValueError as error:
        raise ValueError(f"{label} must be ISO-8601") from error
    return normalized, parsed


def require_iso8601_interval(value, label):
    if not isinstance(value, str) or value.strip() == "":
        raise ValueError(f"{label} is required")
    parts = [part.strip() for part in value.strip().split("/")]
    if len(parts) != 2 or any(part == "" for part in parts):
        raise ValueError(f"{label} must be an ISO-8601 start/end interval")
    try:
        starts_at = datetime.fromisoformat(parts[0].replace("Z", "+00:00"))
        ends_at = datetime.fromisoformat(parts[1].replace("Z", "+00:00"))
    except ValueError as error:
        raise ValueError(f"{label} must be an ISO-8601 start/end interval") from error
    if ends_at < starts_at:
        raise ValueError(f"{label} end must be at or after start")
    return value.strip()


def require_pass(payload, key, label):
    if payload.get(key) != "pass":
        raise ValueError(f"{label}.{key} must be pass")
    return "pass"


def require_object(value, label):
    if not isinstance(value, dict):
        raise ValueError(f"{label} is required")
    return value


def require_nonempty_string(value, label):
    if not isinstance(value, str) or value.strip() == "":
        raise ValueError(f"{label} is required")
    return value.strip()


def require_positive_count(payload, key, label):
    value = payload.get(key)
    if type(value) is not int or value <= 0:
        raise ValueError(f"{label}.{key} must be greater than zero")
    return value


def require_zero_count(payload, key, label):
    value = payload.get(key)
    if type(value) is not int or value != 0:
        raise ValueError(f"{label}.{key} must be zero")
    return value


def require_nonempty_string_array(payload, key, label):
    value = payload.get(key)
    if (
        not isinstance(value, list)
        or not value
        or any(not isinstance(item, str) or item.strip() == "" for item in value)
    ):
        raise ValueError(f"{label}.{key} must be a non-empty array of strings")
    return [item.strip() for item in value]


def build_request_log_slo_proof(slo):
    label = "request-log latency SLO proof"
    triggered_alerts = require_positive_count(slo, "triggeredAlerts", label)
    alert_delivery = require_object(slo.get("alertDelivery"), f"{label}.alertDelivery")
    configured_providers = require_positive_count(alert_delivery, "configuredProviders", f"{label}.alertDelivery")
    delivered_alerts = require_positive_count(alert_delivery, "deliveredAlerts", f"{label}.alertDelivery")
    require_zero_count(alert_delivery, "failedDeliveries", f"{label}.alertDelivery")
    if delivered_alerts < triggered_alerts:
        raise ValueError(f"{label}.alertDelivery.deliveredAlerts must be at least {label}.triggeredAlerts")
    recovery_audit = require_object(slo.get("recoveryAudit"), f"{label}.recoveryAudit")
    audit_records = require_positive_count(recovery_audit, "auditRecords", f"{label}.recoveryAudit")
    require_zero_count(recovery_audit, "failedActions", f"{label}.recoveryAudit")
    if audit_records < triggered_alerts:
        raise ValueError(f"{label}.recoveryAudit.auditRecords must be at least {label}.triggeredAlerts")
    return {
        "latencySLOTrigger": require_pass(slo, "latencySLOTrigger", label),
        "latencySLOAlertDelivery": require_pass(slo, "latencySLOAlertDelivery", label),
        "latencySLORecoveryAction": require_pass(slo, "latencySLORecoveryAction", label),
        "window": require_iso8601_interval(slo.get("window"), f"{label}.window"),
        "triggeredAlerts": triggered_alerts,
        "alertDelivery": {
            "configuredProviders": configured_providers,
            "deliveredAlerts": delivered_alerts,
            "failedDeliveries": 0,
            "channels": require_nonempty_string_array(alert_delivery, "channels", f"{label}.alertDelivery"),
            "lastDeliveryId": require_nonempty_string(alert_delivery.get("lastDeliveryId"), f"{label}.alertDelivery.lastDeliveryId"),
        },
        "recoveryAudit": {
            "auditRecords": audit_records,
            "failedActions": 0,
            "lastRecordId": require_nonempty_string(recovery_audit.get("lastRecordId"), f"{label}.recoveryAudit.lastRecordId"),
        },
    }


def require_count(payload, key):
    value = payload.get(key)
    if not isinstance(value, int) or value < 0:
        raise ValueError(f"request-log coverage.{key} must be a non-negative integer")
    return value


def build_request_usage_join_proof(coverage):
    checked = require_count(coverage, "checkedRecords")
    with_request_id = require_count(coverage, "usageRowsWithRequestId")
    missing_request_id = require_count(coverage, "usageRowsMissingRequestId")
    matched = require_count(coverage, "matchedRequestLogRecords")
    missing_logs = require_count(coverage, "missingRequestLogRecords")
    issues = coverage.get("issues", [])
    if not isinstance(issues, list):
        raise ValueError("request-log coverage.issues must be an array")
    if checked <= 0:
        raise ValueError("requestUsageJoin requires at least one checked usage record")
    if missing_request_id != 0:
        raise ValueError("requestUsageJoin requires zero usage rows missing request_id")
    if missing_logs != 0:
        raise ValueError("requestUsageJoin requires zero missing request-log records")
    if with_request_id != checked:
        raise ValueError("requestUsageJoin requires every checked usage row to carry request_id")
    if matched != with_request_id:
        raise ValueError("requestUsageJoin requires every request_id to join a request-log row")
    if issues:
        raise ValueError("requestUsageJoin requires an empty coverage issues list")
    return "pass"


def build_request_log_proofs(platform_proof_file, coverage_file, slo_file):
    platform = load_json_object("request-log platform proof", platform_proof_file)
    coverage = load_json_object("request-log coverage proof", coverage_file)
    slo = load_json_object("request-log latency SLO proof", slo_file)
    slo_proof = build_request_log_slo_proof(slo)
    return {
        "clickHouseDeployment": require_pass(platform, "clickHouseDeployment", "request-log platform proof"),
        "clickHouseMigration": require_pass(platform, "clickHouseMigration", "request-log platform proof"),
        "requestLogsTable": require_pass(platform, "requestLogsTable", "request-log platform proof"),
        "ingestQuerySmoke": require_pass(platform, "ingestQuerySmoke", "request-log platform proof"),
        "requestUsageJoin": build_request_usage_join_proof(coverage),
        "latencySLOTrigger": slo_proof["latencySLOTrigger"],
        "latencySLOAlertDelivery": slo_proof["latencySLOAlertDelivery"],
        "latencySLORecoveryAction": slo_proof["latencySLORecoveryAction"],
        "latencySLOWindow": slo_proof["window"],
        "latencySLOTriggeredAlerts": slo_proof["triggeredAlerts"],
        "alertDelivery": slo_proof["alertDelivery"],
        "recoveryAudit": slo_proof["recoveryAudit"],
    }


def require_rag_summary_count(summary, key):
    value = summary.get(key)
    if not isinstance(value, int) or value < 0:
        raise ValueError(f"RAG indexing summary.{key} must be a non-negative integer")
    return value


def require_positive_rag_summary_count(summary, key):
    value = require_rag_summary_count(summary, key)
    if value <= 0:
        raise ValueError(f"RAG indexing summary.{key} must be greater than zero")
    return value


def validate_rag_indexing_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("RAG indexing proof.summary must be a JSON object")
    queued_jobs = require_positive_rag_summary_count(summary, "queuedJobs")
    drained_jobs = require_rag_summary_count(summary, "drainedJobs")
    worker_completed_jobs = require_positive_rag_summary_count(summary, "workerCompletedJobs")
    require_positive_rag_summary_count(summary, "rawParserReplayCount")
    require_positive_rag_summary_count(summary, "retrievalProbeCount")
    require_positive_rag_summary_count(summary, "staleVectorRowsFiltered")
    if drained_jobs != queued_jobs:
        raise ValueError("RAG indexing summary.drainedJobs must equal summary.queuedJobs")
    if worker_completed_jobs != drained_jobs:
        raise ValueError("RAG indexing summary.workerCompletedJobs must equal summary.drainedJobs")


def build_rag_indexing_proofs(rag_proof_file):
    proof = load_json_object("RAG indexing proof", rag_proof_file)
    proofs = {
        field: require_pass(proof, field, "RAG indexing proof")
        for field in RAG_INDEXING_PROOF_FIELDS
    }
    validate_rag_indexing_summary(proof)
    return proofs


def build_strict_verifier_proof(strict_verifier_proof_file, current_commit, run_id):
    proof = load_json_object("strict verifier proof", strict_verifier_proof_file)
    command = proof.get("command")
    if not isinstance(command, str) or command.strip() == "":
        raise ValueError("strict verifier proof.command is required")
    if command.strip() != STRICT_VERIFIER_COMMAND:
        raise ValueError("strict verifier proof.command must use the canonical strict verifier invocation")
    proof_commit = proof.get("commit")
    if not isinstance(proof_commit, str) or proof_commit.strip() != current_commit:
        raise ValueError("strict verifier proof.commit must match --current-commit")
    proof_run_id = proof.get("runId")
    if not isinstance(proof_run_id, str) or proof_run_id.strip() != run_id:
        raise ValueError("strict verifier proof.runId must match OBLIVIOUS_TARGET_EVIDENCE_RUN_ID")
    result = require_pass(proof, "result", "strict verifier proof")
    if proof.get("skippedChecks") != []:
        raise ValueError("strict verifier proof.skippedChecks must be an empty array")
    started_at, started_dt = require_iso8601_value(proof.get("startedAt"), "strict verifier proof.startedAt")
    completed_at, completed_dt = require_iso8601_value(proof.get("completedAt"), "strict verifier proof.completedAt")
    if completed_dt < started_dt:
        raise ValueError("strict verifier proof.completedAt must be at or after strict verifier proof.startedAt")
    return {
        "command": command.strip(),
        "result": result,
        "skippedChecks": [],
        "startedAt": started_at,
        "completedAt": completed_at,
        "commit": proof_commit.strip(),
        "runId": proof_run_id.strip(),
        "targetEvidenceSha256": require_sha256_value(proof.get("targetEvidenceSha256"), "strict verifier proof.targetEvidenceSha256"),
        "artifactBundleSha256": require_sha256_value(proof.get("artifactBundleSha256"), "strict verifier proof.artifactBundleSha256"),
    }


def require_telemetry_count(telemetry, key, positive=False):
    value = telemetry.get(key)
    if type(value) is not int or value < 0:
        raise ValueError(f"workflow telemetry proof.telemetry.{key} must be a non-negative integer")
    if positive and value <= 0:
        raise ValueError(f"workflow telemetry proof.telemetry.{key} must be greater than zero")
    return value


def build_workflow_telemetry_proof(workflow_telemetry_proof_file):
    payload = load_json_object("workflow telemetry proof", workflow_telemetry_proof_file)
    if payload.get("result", "pass") != "pass":
        raise ValueError("workflow telemetry proof.result must be pass")
    telemetry = payload.get("telemetry") if isinstance(payload.get("telemetry"), dict) else payload
    if not isinstance(telemetry, dict):
        raise ValueError("workflow telemetry proof.telemetry must be a JSON object")
    success_rate = telemetry.get("successRate")
    if isinstance(success_rate, bool) or not isinstance(success_rate, (int, float)):
        raise ValueError("workflow telemetry proof.telemetry.successRate must be numeric")
    success_rate = float(success_rate)
    if success_rate < 0.99 or success_rate > 1.0:
        raise ValueError("workflow telemetry proof.telemetry.successRate must be between 0.99 and 1.0")
    window = require_iso8601_interval(telemetry.get("window"), "workflow telemetry proof.telemetry.window")
    total_executions = require_telemetry_count(telemetry, "totalExecutions", positive=True)
    successful_executions = require_telemetry_count(telemetry, "successfulExecutions")
    failed_executions = require_telemetry_count(telemetry, "failedExecutions")
    if successful_executions + failed_executions != total_executions:
        raise ValueError("workflow telemetry proof.telemetry.successfulExecutions plus telemetry.failedExecutions must equal telemetry.totalExecutions")
    if abs((successful_executions / total_executions) - success_rate) > 0.0005:
        raise ValueError("workflow telemetry proof.telemetry.successRate must equal telemetry.successfulExecutions / telemetry.totalExecutions")
    return {
        "result": "pass",
        "successRate": success_rate,
        "window": window,
        "totalExecutions": total_executions,
        "successfulExecutions": successful_executions,
        "failedExecutions": failed_executions,
    }


def build_secret_audit_proof(secret_audit_proof_file):
    proof = load_json_object("secret audit proof", secret_audit_proof_file)
    result = require_pass(proof, "result", "secret audit proof")
    checked_at, _ = require_iso8601_value(proof.get("checkedAt"), "secret audit proof.checkedAt")
    scope = proof.get("scope")
    if not isinstance(scope, list) or not scope or any(not isinstance(item, str) or item.strip() == "" for item in scope):
        raise ValueError("secret audit proof.scope must be a non-empty array of strings")
    normalized_scope = [item.strip().lower() for item in scope]
    missing_scopes = [item for item in SECRET_AUDIT_REQUIRED_SCOPES if item not in normalized_scope]
    if missing_scopes:
        raise ValueError("secret audit proof.scope must include kubernetes, providers, and runtime")
    findings = proof.get("findings")
    if findings != []:
        raise ValueError("secret audit proof.findings must be an empty array")
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("secret audit proof.summary must be a JSON object")
    output_summary = {}
    for field in SECRET_AUDIT_SUMMARY_COUNT_FIELDS:
        value = summary.get(field)
        if isinstance(value, bool) or not isinstance(value, int):
            raise ValueError(f"secret audit proof.summary.{field} must be an integer")
        if value < 0:
            raise ValueError(f"secret audit proof.summary.{field} must be non-negative")
        output_summary[field] = value
    if output_summary["totalRecordsScanned"] <= 0:
        raise ValueError("secret audit proof.summary.totalRecordsScanned must be greater than zero")
    for field in ["plaintextRecords", "invalidProtectedRecords", "rotationRequiredRecords"]:
        if output_summary[field] != 0:
            raise ValueError(f"secret audit proof.summary.{field} must be zero")
    if output_summary["protectedRecords"] > output_summary["totalRecordsScanned"]:
        raise ValueError("secret audit proof.summary.protectedRecords must not exceed secret audit proof.summary.totalRecordsScanned")
    output_scope = []
    seen_scope = set()
    for item in scope:
        stripped = item.strip()
        lowered = stripped.lower()
        if lowered not in seen_scope:
            seen_scope.add(lowered)
            output_scope.append(stripped)
    return {
        "result": result,
        "checkedAt": checked_at,
        "scope": output_scope,
        "findings": [],
        "summary": output_summary,
    }


def build_deployment_proofs(deployment_proof_file):
    proof = load_json_object("deployment proof", deployment_proof_file)
    if proof.get("result", "pass") != "pass":
        raise ValueError("deployment proof.result must be pass")
    require_target_environment(proof, "deployment proof")
    validate_reference_map(proof, DEPLOYMENT_REFERENCE_FIELDS, "deployment proof")
    return {
        field: require_pass(proof, field, "deployment proof")
        for field in DEPLOYMENT_PROOF_FIELDS
    }


def build_kubernetes_proof(kubernetes_proof_file):
    proof = load_json_object("kubernetes proof", kubernetes_proof_file)
    if proof.get("result", "pass") != "pass":
        raise ValueError("kubernetes proof.result must be pass")
    require_target_environment(proof, "kubernetes proof")
    validate_concrete_reference(proof.get("clusterRef"), "kubernetes proof.clusterRef")
    validate_concrete_reference(proof.get("namespace"), "kubernetes proof.namespace")
    validate_reference_map(proof, KUBERNETES_REFERENCE_FIELDS, "kubernetes proof")
    if proof.get("secretFileClass") != "external-filled":
        raise ValueError("kubernetes proof.secretFileClass must be external-filled")
    return {
        "secretFileClass": "external-filled",
        "proofs": {
            field: require_pass(proof, field, "kubernetes proof")
            for field in KUBERNETES_PROOF_FIELDS
        },
    }


def require_target_environment(proof, label):
    if proof.get("targetEnvironment") != "production":
        raise ValueError(f"{label}.targetEnvironment must be production")
    return "production"


def validate_concrete_reference(value, label):
    if not isinstance(value, str) or value.strip() == "":
        raise ValueError(f"{label} is required")
    if REFERENCE_PLACEHOLDER_PATTERN.search(value):
        raise ValueError(f"{label} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        raise ValueError(f"{label} must not embed secret material")
    return value.strip()


def validate_reference_map(proof, fields, label):
    references = proof.get("references")
    if not isinstance(references, dict):
        raise ValueError(f"{label}.references must be a JSON object")
    for field in fields:
        validate_concrete_reference(references.get(field), f"{label}.references.{field}")


def validate_provider_live_rail_summary(proof, provider_label):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError(f"{provider_label} provider live rail proof.summary must be a JSON object")
    for field in ["checkoutAttempts", "refundAttempts", "payoutAttempts", "reconciliationChecks"]:
        require_summary_count(summary, field, f"{provider_label} provider live rail proof", positive=True)


def validate_provider_live_rail_references(proof, provider_label):
    if proof.get("providerEnvironment") != "live":
        raise ValueError(f"{provider_label} provider live rail proof.providerEnvironment must be live")
    references = proof.get("references")
    if not isinstance(references, dict):
        raise ValueError(f"{provider_label} provider live rail proof.references must be a JSON object")
    for field in PROVIDER_LIVE_RAIL_REFERENCE_FIELDS:
        value = references.get(field)
        if not isinstance(value, str) or value.strip() == "":
            raise ValueError(f"{provider_label} provider live rail proof.references.{field} is required")
        if REFERENCE_PLACEHOLDER_PATTERN.search(value):
            raise ValueError(f"{provider_label} provider live rail proof.references.{field} must be concrete")
        if EMBEDDED_SECRET_PATTERN.search(value):
            raise ValueError(f"{provider_label} provider live rail proof.references.{field} must not embed secret material")


def build_provider_live_rail_proof(provider, proof_file):
    proof = load_json_object(f"{provider} provider live rail proof", proof_file)
    proof_provider = str(proof.get("provider", "")).strip().lower()
    if proof_provider != provider:
        raise ValueError(f"{provider} provider live rail proof.provider must match {provider}")
    if proof.get("mode") != "live":
        raise ValueError(f"{provider} provider live rail proof.mode must be live")
    proofs = {
        field: require_pass(proof, field, f"{provider} provider live rail proof")
        for field in PROVIDER_LIVE_RAIL_PROOF_FIELDS
    }
    validate_provider_live_rail_summary(proof, provider)
    validate_provider_live_rail_references(proof, provider)
    return {"mode": "live", "providerEnvironment": "live", "proofs": proofs}


def require_summary_count(summary, key, label, positive=False):
    value = summary.get(key)
    if not isinstance(value, int) or value < 0:
        raise ValueError(f"{label} summary.{key} must be a non-negative integer")
    if positive and value <= 0:
        raise ValueError(f"{label} summary.{key} must be greater than zero")
    return value


def require_mode(proof, label):
    mode = proof.get("mode")
    if mode not in (DISABLED_MODE, LIVE_MODE):
        raise ValueError(f"{label}.mode must be {DISABLED_MODE} or {LIVE_MODE}")
    return mode


def validate_relay_realtime_disabled_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("relay realtime proof.summary must be a JSON object")
    require_summary_count(summary, "productionPolicyChecks", "relay realtime proof", positive=True)
    blocker_checks = require_summary_count(summary, "authOriginPrebillAbortUsageBlockerChecks", "relay realtime proof", positive=True)
    if blocker_checks < 5:
        raise ValueError("relay realtime proof summary.authOriginPrebillAbortUsageBlockerChecks must cover auth, origin, prebill, abort, and usage blockers")


def validate_relay_realtime_live_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("relay realtime proof.summary must be a JSON object")
    total_requests = require_summary_count(summary, "totalRequests", "relay realtime proof", positive=True)
    authenticated_requests = require_summary_count(summary, "authenticatedRequests", "relay realtime proof", positive=True)
    request_linked_usage_records = require_summary_count(summary, "requestLinkedUsageRecords", "relay realtime proof", positive=True)
    price_snapshot_records = require_summary_count(summary, "priceSnapshotRecords", "relay realtime proof", positive=True)
    require_summary_count(summary, "abortSettlementRecords", "relay realtime proof", positive=True)
    terminal_usage_records = require_summary_count(summary, "terminalUsageRecords", "relay realtime proof", positive=True)
    require_summary_count(summary, "originPolicyChecks", "relay realtime proof", positive=True)
    if authenticated_requests != total_requests:
        raise ValueError("relay realtime proof summary.authenticatedRequests must equal summary.totalRequests")
    if request_linked_usage_records != total_requests:
        raise ValueError("relay realtime proof summary.requestLinkedUsageRecords must equal summary.totalRequests")
    if price_snapshot_records != total_requests:
        raise ValueError("relay realtime proof summary.priceSnapshotRecords must equal summary.totalRequests")
    if terminal_usage_records != total_requests:
        raise ValueError("relay realtime proof summary.terminalUsageRecords must equal summary.totalRequests")


def build_relay_realtime_proof(relay_realtime_proof_file):
    proof = load_json_object("relay realtime proof", relay_realtime_proof_file)
    mode = require_mode(proof, "relay realtime proof")
    if mode != LIVE_MODE:
        raise ValueError("relay realtime proof.mode must be commercial_lifecycle_enabled for final target evidence")
    fields = RELAY_REALTIME_LIVE_PROOF_FIELDS if mode == LIVE_MODE else RELAY_REALTIME_DISABLED_PROOF_FIELDS
    proofs = {
        field: require_pass(proof, field, "relay realtime proof")
        for field in fields
    }
    if mode == LIVE_MODE:
        validate_relay_realtime_live_summary(proof)
    else:
        validate_relay_realtime_disabled_summary(proof)
    return {"mode": mode, "proofs": proofs}


def validate_relay_batch_disabled_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("relay batch proof.summary must be a JSON object")
    require_summary_count(summary, "productionPolicyChecks", "relay batch proof", positive=True)
    blocker_checks = require_summary_count(summary, "prebillPollingSettlementRefundAuditUsageBlockerChecks", "relay batch proof", positive=True)
    if blocker_checks < 6:
        raise ValueError("relay batch proof summary.prebillPollingSettlementRefundAuditUsageBlockerChecks must cover prebill, polling, settlement, refund, audit, and usage blockers")


def validate_relay_batch_live_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("relay batch proof.summary must be a JSON object")
    require_summary_count(summary, "prebillReservations", "relay batch proof", positive=True)
    require_summary_count(summary, "pollingCompletions", "relay batch proof", positive=True)
    settlement_records = require_summary_count(summary, "settlementRecords", "relay batch proof", positive=True)
    refund_records = require_summary_count(summary, "refundRecords", "relay batch proof", positive=True)
    usage_audit_records = require_summary_count(summary, "usageAuditRecords", "relay batch proof", positive=True)
    request_log_audit_records = require_summary_count(summary, "requestLogAuditRecords", "relay batch proof", positive=True)
    terminal_failure_records = require_summary_count(summary, "terminalFailureRecords", "relay batch proof", positive=True)
    if usage_audit_records < settlement_records + refund_records:
        raise ValueError("relay batch proof summary.usageAuditRecords must cover summary.settlementRecords plus summary.refundRecords")
    if request_log_audit_records < settlement_records + refund_records:
        raise ValueError("relay batch proof summary.requestLogAuditRecords must cover summary.settlementRecords plus summary.refundRecords")
    if terminal_failure_records < refund_records:
        raise ValueError("relay batch proof summary.terminalFailureRecords must cover summary.refundRecords")


def build_relay_batch_proof(relay_batch_proof_file):
    proof = load_json_object("relay batch proof", relay_batch_proof_file)
    mode = require_mode(proof, "relay batch proof")
    if mode != LIVE_MODE:
        raise ValueError("relay batch proof.mode must be commercial_lifecycle_enabled for final target evidence")
    fields = RELAY_BATCH_LIVE_PROOF_FIELDS if mode == LIVE_MODE else RELAY_BATCH_DISABLED_PROOF_FIELDS
    proofs = {
        field: require_pass(proof, field, "relay batch proof")
        for field in fields
    }
    if mode == LIVE_MODE:
        validate_relay_batch_live_summary(proof)
    else:
        validate_relay_batch_disabled_summary(proof)
    return {"mode": mode, "proofs": proofs}


def validate_marketplace_payout_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("marketplace payout proof.summary must be a JSON object")
    outbound_dispatches = require_summary_count(summary, "outboundDispatches", "marketplace payout proof", positive=True)
    webhook_events = require_summary_count(summary, "webhookEvents", "marketplace payout proof", positive=True)
    settlement_ledger_entries = require_summary_count(summary, "settlementLedgerEntries", "marketplace payout proof", positive=True)
    reconciled_entries = require_summary_count(summary, "reconciledEntries", "marketplace payout proof")
    refund_chargeback_cases = require_summary_count(summary, "refundChargebackCases", "marketplace payout proof")
    refund_chargeback_cases_handled = require_summary_count(summary, "refundChargebackCasesHandled", "marketplace payout proof")
    if webhook_events < outbound_dispatches:
        raise ValueError("marketplace payout proof summary.webhookEvents must cover summary.outboundDispatches")
    if reconciled_entries != settlement_ledger_entries:
        raise ValueError("marketplace payout proof summary.reconciledEntries must equal summary.settlementLedgerEntries")
    if refund_chargeback_cases_handled != refund_chargeback_cases:
        raise ValueError("marketplace payout proof summary.refundChargebackCasesHandled must equal summary.refundChargebackCases")


def validate_marketplace_governance_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("marketplace governance proof.summary must be a JSON object")
    review_queue_items = require_summary_count(summary, "reviewQueueItems", "marketplace governance proof", positive=True)
    appeal_queue_items = require_summary_count(summary, "appealQueueItems", "marketplace governance proof", positive=True)
    appeal_decisions = require_summary_count(summary, "appealDecisions", "marketplace governance proof")
    review_assignments = require_summary_count(summary, "reviewAssignments", "marketplace governance proof", positive=True)
    sla_checks = require_summary_count(summary, "slaChecks", "marketplace governance proof", positive=True)
    require_summary_count(summary, "slaBreachesHandled", "marketplace governance proof")
    abuse_reports = require_summary_count(summary, "abuseReports", "marketplace governance proof", positive=True)
    abuse_reports_resolved = require_summary_count(summary, "abuseReportsResolved", "marketplace governance proof")
    if review_assignments < review_queue_items:
        raise ValueError("marketplace governance proof summary.reviewAssignments must cover summary.reviewQueueItems")
    if appeal_decisions != appeal_queue_items:
        raise ValueError("marketplace governance proof summary.appealDecisions must equal summary.appealQueueItems")
    if sla_checks < review_assignments:
        raise ValueError("marketplace governance proof summary.slaChecks must cover summary.reviewAssignments")
    if abuse_reports_resolved != abuse_reports:
        raise ValueError("marketplace governance proof summary.abuseReportsResolved must equal summary.abuseReports")


def build_marketplace_payout_proofs(payout_proof_file):
    proof = load_json_object("marketplace payout proof", payout_proof_file)
    proofs = {
        field: require_pass(proof, field, "marketplace payout proof")
        for field in MARKETPLACE_PAYOUT_PROOF_FIELDS
    }
    validate_marketplace_payout_summary(proof)
    return proofs


def build_marketplace_governance_proofs(governance_proof_file):
    proof = load_json_object("marketplace governance proof", governance_proof_file)
    proofs = {
        field: require_pass(proof, field, "marketplace governance proof")
        for field in MARKETPLACE_GOVERNANCE_PROOF_FIELDS
    }
    validate_marketplace_governance_summary(proof)
    return proofs


def validate_provider_runtime_config_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("provider runtime config proof.summary must be a JSON object")
    providers_configured = require_summary_count(summary, "providersConfigured", "provider runtime config proof", positive=True)
    provider_env_vars_checked = require_summary_count(summary, "providerEnvVarsChecked", "provider runtime config proof", positive=True)
    checkout_base_urls_checked = require_summary_count(summary, "checkoutBaseUrlsChecked", "provider runtime config proof", positive=True)
    webhook_routes_checked = require_summary_count(summary, "webhookRoutesChecked", "provider runtime config proof", positive=True)
    webhook_verification_checks = require_summary_count(summary, "webhookVerificationChecks", "provider runtime config proof", positive=True)
    if providers_configured < 3:
        raise ValueError("provider runtime config proof summary.providersConfigured must include Stripe, Alipay, and WeChat Pay")
    if provider_env_vars_checked < providers_configured:
        raise ValueError("provider runtime config proof summary.providerEnvVarsChecked must cover summary.providersConfigured")
    if checkout_base_urls_checked < providers_configured:
        raise ValueError("provider runtime config proof summary.checkoutBaseUrlsChecked must cover summary.providersConfigured")
    if webhook_routes_checked < providers_configured:
        raise ValueError("provider runtime config proof summary.webhookRoutesChecked must cover summary.providersConfigured")
    if webhook_verification_checks < providers_configured:
        raise ValueError("provider runtime config proof summary.webhookVerificationChecks must cover summary.providersConfigured")


def validate_provider_runtime_config_details(proof):
    providers = proof.get("providers")
    if not isinstance(providers, list) or not providers:
        raise ValueError("provider runtime config proof.providers must be a non-empty array")
    seen = set()
    for index, item in enumerate(providers):
        label = f"provider runtime config proof.providers[{index}]"
        if not isinstance(item, dict):
            raise ValueError(f"{label} must be a JSON object")
        name = validate_concrete_reference(item.get("name"), f"{label}.name").strip().lower()
        if name in seen:
            raise ValueError(f"provider runtime config proof.providers must not duplicate {name}")
        seen.add(name)
        if item.get("providerEnvironment") != "live":
            raise ValueError(f"{label}.providerEnvironment must be live")
        require_pass(item, "providerEnv", label)
        if item.get("checkoutBaseUrlClass") != "external-filled":
            raise ValueError(f"{label}.checkoutBaseUrlClass must be external-filled")
        require_pass(item, "webhookRoute", label)
        require_pass(item, "webhookVerification", label)
        validate_concrete_reference(item.get("evidenceId"), f"{label}.evidenceId")
    missing = [name for name in PROVIDER_RUNTIME_CONFIG_REQUIRED_PROVIDERS if name not in seen]
    if missing:
        raise ValueError("provider runtime config proof.providers must include stripe, alipay, and wechatpay (missing: " + ", ".join(missing) + ")")


def build_provider_runtime_config_proofs(provider_runtime_config_proof_file):
    proof = load_json_object("provider runtime config proof", provider_runtime_config_proof_file)
    proofs = {
        field: require_pass(proof, field, "provider runtime config proof")
        for field in PROVIDER_RUNTIME_CONFIG_PROOF_FIELDS
    }
    validate_provider_runtime_config_summary(proof)
    validate_provider_runtime_config_details(proof)
    return proofs


def validate_microservice_database_summary(proof):
    if proof.get("mode") != "microservices":
        raise ValueError("microservice database proof.mode must be microservices")
    if proof.get("serviceUrlClass") != "external-filled":
        raise ValueError("microservice database proof.serviceUrlClass must be external-filled")
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        raise ValueError("microservice database proof.summary must be a JSON object")
    services_checked = require_summary_count(summary, "servicesChecked", "microservice database proof", positive=True)
    external_urls_checked = require_summary_count(summary, "externalUrlsChecked", "microservice database proof", positive=True)
    migration_readiness_checks = require_summary_count(summary, "migrationReadinessChecks", "microservice database proof", positive=True)
    if services_checked != len(MICROSERVICE_DATABASE_SERVICE_FIELDS):
        raise ValueError(f"microservice database proof summary.servicesChecked must equal {len(MICROSERVICE_DATABASE_SERVICE_FIELDS)}")
    if external_urls_checked != services_checked:
        raise ValueError("microservice database proof summary.externalUrlsChecked must equal summary.servicesChecked")
    if migration_readiness_checks != services_checked:
        raise ValueError("microservice database proof summary.migrationReadinessChecks must equal summary.servicesChecked")


def validate_microservice_database_details(proof):
    services = proof.get("services")
    if not isinstance(services, list) or not services:
        raise ValueError("microservice database proof.services must be a non-empty array")
    seen = set()
    for index, item in enumerate(services):
        label = f"microservice database proof.services[{index}]"
        if not isinstance(item, dict):
            raise ValueError(f"{label} must be a JSON object")
        name = validate_concrete_reference(item.get("name"), f"{label}.name").strip().lower()
        if name in seen:
            raise ValueError(f"microservice database proof.services must not duplicate {name}")
        seen.add(name)
        if item.get("databaseUrlClass") != "external-filled":
            raise ValueError(f"{label}.databaseUrlClass must be external-filled")
        require_pass(item, "migrationReadiness", label)
        validate_concrete_reference(item.get("evidenceId"), f"{label}.evidenceId")
    missing = [name for name in MICROSERVICE_DATABASE_SERVICE_FIELDS if name not in seen]
    if missing:
        raise ValueError("microservice database proof.services must include relay, chat, workflow, rag, agent, billing, marketplace, admin, channel, task, and observability (missing: " + ", ".join(missing) + ")")


def build_microservice_database_proof(microservice_database_proof_file):
    proof = load_json_object("microservice database proof", microservice_database_proof_file)
    proofs = {
        field: require_pass(proof, field, "microservice database proof")
        for field in MICROSERVICE_DATABASE_PROOF_FIELDS
    }
    validate_microservice_database_summary(proof)
    validate_microservice_database_details(proof)
    return {
        "mode": "microservices",
        "serviceUrlClass": "external-filled",
        "proofs": proofs,
    }


def build_artifacts(
    run_id,
    commit,
    completed_at,
    grpc_recorded_at,
    deployment_proofs,
    kubernetes_proof,
    provider_live_rail_proofs,
    request_log_proofs,
    rag_indexing_proofs,
    relay_realtime_proof,
    relay_batch_proof,
    marketplace_payout_proofs,
    marketplace_governance_proofs,
    provider_runtime_config_proofs,
    microservice_database_proof,
):
    artifacts = []
    for spec in ARTIFACT_SPECS:
        suffix, kind, uri_env = spec[:3]
        provider = spec[3] if len(spec) > 3 else None
        artifact = {
            "id": artifact_id(run_id, suffix),
            "kind": kind,
            "commit": commit,
            "runId": run_id,
            "uri": require_env(uri_env),
            "recordedAt": artifact_recorded_at(kind, completed_at, grpc_recorded_at),
        }
        sha_env = uri_env.replace("_URI", "_SHA256")
        artifact["sha256"] = require_env(sha_env)
        if provider:
            artifact["provider"] = provider
            artifact["proofs"] = dict(provider_live_rail_proofs[provider]["proofs"])
        if kind == "deployment-log":
            artifact["proofs"] = dict(deployment_proofs)
        if kind == "kubernetes-validation":
            artifact["proofs"] = dict(kubernetes_proof["proofs"])
        if kind == "request-log-observability":
            artifact["proofs"] = {
                field: request_log_proofs[field]
                for field in REQUEST_LOG_PLATFORM_PROOF_FIELDS
                + ["requestUsageJoin"]
                + REQUEST_LOG_SLO_PROOF_FIELDS
            }
        if kind == "rag-indexing-proof":
            artifact["proofs"] = dict(rag_indexing_proofs)
        if kind == "relay-realtime-proof":
            artifact["proofs"] = dict(relay_realtime_proof["proofs"])
        if kind == "relay-batch-proof":
            artifact["proofs"] = dict(relay_batch_proof["proofs"])
        if kind == "marketplace-payout-proof":
            artifact["proofs"] = dict(marketplace_payout_proofs)
        elif kind == "marketplace-governance-proof":
            artifact["proofs"] = dict(marketplace_governance_proofs)
        if kind == "provider-runtime-config":
            artifact["proofs"] = dict(provider_runtime_config_proofs)
        if kind == "microservice-database-proof":
            artifact["proofs"] = dict(microservice_database_proof["proofs"])
        artifacts.append(artifact)
    return artifacts


def build_manifest(
    current_commit,
    grpc_smoke_file,
    strict_verifier_proof_file,
    deployment_proof_file,
    kubernetes_proof_file,
    stripe_provider_live_rail_proof_file,
    alipay_provider_live_rail_proof_file,
    wechatpay_provider_live_rail_proof_file,
    secret_audit_proof_file,
    workflow_telemetry_proof_file,
    request_log_platform_proof_file,
    request_log_coverage_file,
    request_log_slo_file,
    rag_proof_file,
    relay_realtime_proof_file,
    relay_batch_proof_file,
    marketplace_payout_proof_file,
    marketplace_governance_proof_file,
    provider_runtime_config_proof_file,
    microservice_database_proof_file,
):
    required_env = REQUIRED_ENV + REQUIRED_ARTIFACT_SHA_ENV
    missing = [name for name in required_env if os.environ.get(name, "").strip() == ""]
    if missing:
        raise ValueError("missing required environment variables: " + ", ".join(missing))

    grpc_report = load_grpc_smoke_report(grpc_smoke_file)
    run_id = require_env("OBLIVIOUS_TARGET_EVIDENCE_RUN_ID")
    strict_verifier_proof = build_strict_verifier_proof(strict_verifier_proof_file, current_commit, run_id)
    deployment_proofs = build_deployment_proofs(deployment_proof_file)
    kubernetes_proof = build_kubernetes_proof(kubernetes_proof_file)
    provider_live_rail_proofs = {
        "stripe": build_provider_live_rail_proof("stripe", stripe_provider_live_rail_proof_file),
        "alipay": build_provider_live_rail_proof("alipay", alipay_provider_live_rail_proof_file),
        "wechatpay": build_provider_live_rail_proof("wechatpay", wechatpay_provider_live_rail_proof_file),
    }
    secret_audit_proof = build_secret_audit_proof(secret_audit_proof_file)
    workflow_telemetry_proof = build_workflow_telemetry_proof(workflow_telemetry_proof_file)
    request_log_proofs = build_request_log_proofs(
        request_log_platform_proof_file,
        request_log_coverage_file,
        request_log_slo_file,
    )
    rag_indexing_proofs = build_rag_indexing_proofs(rag_proof_file)
    relay_realtime_proof = build_relay_realtime_proof(relay_realtime_proof_file)
    relay_batch_proof = build_relay_batch_proof(relay_batch_proof_file)
    marketplace_payout_proofs = build_marketplace_payout_proofs(marketplace_payout_proof_file)
    marketplace_governance_proofs = build_marketplace_governance_proofs(marketplace_governance_proof_file)
    provider_runtime_config_proofs = build_provider_runtime_config_proofs(provider_runtime_config_proof_file)
    microservice_database_proof = build_microservice_database_proof(microservice_database_proof_file)
    environment_class = require_env("OBLIVIOUS_TARGET_ENVIRONMENT_CLASS")
    if environment_class.strip().lower() not in ALLOWED_ENVIRONMENT_CLASSES:
        raise ValueError("OBLIVIOUS_TARGET_ENVIRONMENT_CLASS must be staging, preproduction, or production")
    if environment_class.strip().lower() != "production":
        raise ValueError("OBLIVIOUS_TARGET_ENVIRONMENT_CLASS must be production for final target evidence")
    started_at = require_env("OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT")
    completed_at = require_env("OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT")
    if started_at != strict_verifier_proof["startedAt"]:
        raise ValueError("OBLIVIOUS_TARGET_STRICT_VERIFIER_STARTED_AT must match strict verifier proof.startedAt")
    if completed_at != strict_verifier_proof["completedAt"]:
        raise ValueError("OBLIVIOUS_TARGET_STRICT_VERIFIER_COMPLETED_AT must match strict verifier proof.completedAt")
    grpc_recorded_at = str(grpc_report.get("recordedAt", "")).strip()
    if grpc_recorded_at == "":
        raise ValueError("gRPC smoke report recordedAt is required")

    grpc_ref = artifact_id(run_id, "grpc-smoke-report")
    grpc_entries = []
    for result in grpc_report["results"]:
        if not isinstance(result, dict):
            raise ValueError("gRPC smoke report results must contain objects")
        grpc_entries.append(
            {
                "service": result.get("service"),
                "address": result.get("address"),
                "generatedClient": result.get("generatedClient"),
                "evidenceRef": grpc_ref,
            }
        )

    success_rate_raw = require_env("OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE")
    try:
        success_rate = float(success_rate_raw)
    except ValueError as error:
        raise ValueError("OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE must be numeric") from error
    if abs(success_rate - workflow_telemetry_proof["successRate"]) > 0.000001:
        raise ValueError("OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_SUCCESS_RATE must match workflow telemetry proof.telemetry.successRate")
    workflow_window = require_env("OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW")
    if workflow_window != workflow_telemetry_proof["window"]:
        raise ValueError("OBLIVIOUS_TARGET_WORKFLOW_TELEMETRY_WINDOW must match workflow telemetry proof.telemetry.window")

    return {
        "schemaVersion": 1,
        "repository": "Oblivious",
        "commit": current_commit,
        "runId": run_id,
        "environment": {
            "name": require_env("OBLIVIOUS_TARGET_ENVIRONMENT_NAME"),
            "class": environment_class,
            "baseUrl": require_env("OBLIVIOUS_TARGET_ENVIRONMENT_BASE_URL"),
            "recordedAt": os.environ.get("OBLIVIOUS_TARGET_ENVIRONMENT_RECORDED_AT", completed_at).strip(),
        },
        "strictVerifier": {
            "command": strict_verifier_proof["command"],
            "result": strict_verifier_proof["result"],
            "skippedChecks": strict_verifier_proof["skippedChecks"],
            "startedAt": strict_verifier_proof["startedAt"],
            "completedAt": strict_verifier_proof["completedAt"],
            "targetEvidenceSha256": strict_verifier_proof["targetEvidenceSha256"],
            "artifactBundleSha256": strict_verifier_proof["artifactBundleSha256"],
            "evidenceRef": artifact_id(run_id, "strict-verifier-log"),
        },
        "deployment": {
            "deployValidation": deployment_proofs["deployValidation"],
            "backupRestore": deployment_proofs["backupRestore"],
            "migrationReplay": deployment_proofs["migrationReplay"],
            "evidenceRef": artifact_id(run_id, "deployment-log"),
        },
        "kubernetes": {
            "validation": kubernetes_proof["proofs"]["validation"],
            "rollout": kubernetes_proof["proofs"]["rollout"],
            "failover": kubernetes_proof["proofs"]["failover"],
            "secretFileClass": kubernetes_proof["secretFileClass"],
            "evidenceRef": artifact_id(run_id, "kubernetes-validation"),
        },
        "providers": [
            {"name": "stripe", "mode": provider_live_rail_proofs["stripe"]["mode"], "providerEnvironment": provider_live_rail_proofs["stripe"]["providerEnvironment"], **provider_live_rail_proofs["stripe"]["proofs"], "evidenceRef": artifact_id(run_id, "stripe-provider-live-rail")},
            {"name": "alipay", "mode": provider_live_rail_proofs["alipay"]["mode"], "providerEnvironment": provider_live_rail_proofs["alipay"]["providerEnvironment"], **provider_live_rail_proofs["alipay"]["proofs"], "evidenceRef": artifact_id(run_id, "alipay-provider-live-rail")},
            {"name": "wechatpay", "mode": provider_live_rail_proofs["wechatpay"]["mode"], "providerEnvironment": provider_live_rail_proofs["wechatpay"]["providerEnvironment"], **provider_live_rail_proofs["wechatpay"]["proofs"], "evidenceRef": artifact_id(run_id, "wechatpay-provider-live-rail")},
        ],
        "grpc": grpc_entries,
        "grpcSmokeReport": {
            "evidenceRef": grpc_ref,
            "recordedAt": grpc_recorded_at,
            "timeout": grpc_report.get("timeout"),
            "results": grpc_report["results"],
        },
        "secretAudit": {
            "result": secret_audit_proof["result"],
            "checkedAt": secret_audit_proof["checkedAt"],
            "scope": secret_audit_proof["scope"],
            "summary": secret_audit_proof["summary"],
            "evidenceRef": artifact_id(run_id, "secret-audit"),
        },
        "workflowTelemetry": {
            "result": workflow_telemetry_proof["result"],
            "successRate": workflow_telemetry_proof["successRate"],
            "window": workflow_telemetry_proof["window"],
            "totalExecutions": workflow_telemetry_proof["totalExecutions"],
            "successfulExecutions": workflow_telemetry_proof["successfulExecutions"],
            "failedExecutions": workflow_telemetry_proof["failedExecutions"],
            "evidenceRef": artifact_id(run_id, "workflow-telemetry"),
        },
        "requestLogObservability": {
            "backend": "clickhouse",
            "clickHouseDeployment": request_log_proofs["clickHouseDeployment"],
            "clickHouseMigration": request_log_proofs["clickHouseMigration"],
            "requestLogsTable": request_log_proofs["requestLogsTable"],
            "ingestQuerySmoke": request_log_proofs["ingestQuerySmoke"],
            "requestUsageJoin": request_log_proofs["requestUsageJoin"],
            "latencySLOTrigger": request_log_proofs["latencySLOTrigger"],
            "latencySLOAlertDelivery": request_log_proofs["latencySLOAlertDelivery"],
            "latencySLORecoveryAction": request_log_proofs["latencySLORecoveryAction"],
            "latencySLOWindow": request_log_proofs["latencySLOWindow"],
            "latencySLOTriggeredAlerts": request_log_proofs["latencySLOTriggeredAlerts"],
            "alertDelivery": request_log_proofs["alertDelivery"],
            "recoveryAudit": request_log_proofs["recoveryAudit"],
            "evidenceRef": artifact_id(run_id, "request-log-observability"),
        },
        "ragIndexing": {
            "durableQueueMigration": rag_indexing_proofs["durableQueueMigration"],
            "workerDeployment": rag_indexing_proofs["workerDeployment"],
            "enqueueDrainProbe": rag_indexing_proofs["enqueueDrainProbe"],
            "rawParserReplay": rag_indexing_proofs["rawParserReplay"],
            "retrievalProbe": rag_indexing_proofs["retrievalProbe"],
            "staleVectorFilter": rag_indexing_proofs["staleVectorFilter"],
            "evidenceRef": artifact_id(run_id, "rag-indexing-proof"),
        },
        "relayRealtime": {
            "mode": relay_realtime_proof["mode"],
            **relay_realtime_proof["proofs"],
            "evidenceRef": artifact_id(run_id, "relay-realtime-proof"),
        },
        "relayBatch": {
            "mode": relay_batch_proof["mode"],
            **relay_batch_proof["proofs"],
            "evidenceRef": artifact_id(run_id, "relay-batch-proof"),
        },
        "marketplacePayouts": {
            "providerMode": "webhook",
            "outboundDispatch": marketplace_payout_proofs["outboundDispatch"],
            "inboundWebhookLifecycle": marketplace_payout_proofs["inboundWebhookLifecycle"],
            "settlementLedger": marketplace_payout_proofs["settlementLedger"],
            "reconciliation": marketplace_payout_proofs["reconciliation"],
            "refundChargebackHandling": marketplace_payout_proofs["refundChargebackHandling"],
            "evidenceRef": artifact_id(run_id, "marketplace-payout-proof"),
        },
        "marketplaceGovernance": {
            "reviewQueue": marketplace_governance_proofs["reviewQueue"],
            "appealQueue": marketplace_governance_proofs["appealQueue"],
            "appealDecisionLifecycle": marketplace_governance_proofs["appealDecisionLifecycle"],
            "reviewAssignment": marketplace_governance_proofs["reviewAssignment"],
            "reviewSLAEnforcement": marketplace_governance_proofs["reviewSLAEnforcement"],
            "abuseReportLifecycle": marketplace_governance_proofs["abuseReportLifecycle"],
            "evidenceRef": artifact_id(run_id, "marketplace-governance-proof"),
        },
        "providerRuntimeConfig": {
            "stripe": provider_runtime_config_proofs["stripe"],
            "alipay": provider_runtime_config_proofs["alipay"],
            "wechatpay": provider_runtime_config_proofs["wechatpay"],
            "providerEnv": provider_runtime_config_proofs["providerEnv"],
            "checkoutBaseUrls": provider_runtime_config_proofs["checkoutBaseUrls"],
            "webhookRoutes": provider_runtime_config_proofs["webhookRoutes"],
            "webhookVerification": provider_runtime_config_proofs["webhookVerification"],
            "evidenceRef": artifact_id(run_id, "provider-runtime-config"),
        },
        "microserviceDatabases": {
            "mode": microservice_database_proof["mode"],
            "serviceUrlClass": microservice_database_proof["serviceUrlClass"],
            **microservice_database_proof["proofs"],
            "evidenceRef": artifact_id(run_id, "microservice-database-proof"),
        },
        "artifacts": build_artifacts(
            run_id,
            current_commit,
            completed_at,
            grpc_recorded_at,
            deployment_proofs,
            kubernetes_proof,
            provider_live_rail_proofs,
            request_log_proofs,
            rag_indexing_proofs,
            relay_realtime_proof,
            relay_batch_proof,
            marketplace_payout_proofs,
            marketplace_governance_proofs,
            provider_runtime_config_proofs,
            microservice_database_proof,
        ),
    }


def main():
    parser = argparse.ArgumentParser(description="Assemble a strict target release evidence manifest from concrete target-run inputs.")
    parser.add_argument("--current-commit", required=True)
    parser.add_argument("--grpc-smoke-file", required=True)
    parser.add_argument("--strict-verifier-proof-file", required=True)
    parser.add_argument("--deployment-proof-file", required=True)
    parser.add_argument("--kubernetes-proof-file", required=True)
    parser.add_argument("--stripe-provider-live-rail-proof-file", required=True)
    parser.add_argument("--alipay-provider-live-rail-proof-file", required=True)
    parser.add_argument("--wechatpay-provider-live-rail-proof-file", required=True)
    parser.add_argument("--secret-audit-proof-file", required=True)
    parser.add_argument("--workflow-telemetry-proof-file", required=True)
    parser.add_argument("--request-log-platform-proof-file", required=True)
    parser.add_argument("--request-log-coverage-file", required=True)
    parser.add_argument("--request-log-slo-file", required=True)
    parser.add_argument("--rag-proof-file", required=True)
    parser.add_argument("--relay-realtime-proof-file", required=True)
    parser.add_argument("--relay-batch-proof-file", required=True)
    parser.add_argument("--marketplace-payout-proof-file", required=True)
    parser.add_argument("--marketplace-governance-proof-file", required=True)
    parser.add_argument("--provider-runtime-config-proof-file", required=True)
    parser.add_argument("--microservice-database-proof-file", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    try:
        manifest = build_manifest(
            args.current_commit,
            args.grpc_smoke_file,
            args.strict_verifier_proof_file,
            args.deployment_proof_file,
            args.kubernetes_proof_file,
            args.stripe_provider_live_rail_proof_file,
            args.alipay_provider_live_rail_proof_file,
            args.wechatpay_provider_live_rail_proof_file,
            args.secret_audit_proof_file,
            args.workflow_telemetry_proof_file,
            args.request_log_platform_proof_file,
            args.request_log_coverage_file,
            args.request_log_slo_file,
            args.rag_proof_file,
            args.relay_realtime_proof_file,
            args.relay_batch_proof_file,
            args.marketplace_payout_proof_file,
            args.marketplace_governance_proof_file,
            args.provider_runtime_config_proof_file,
            args.microservice_database_proof_file,
        )
    except (OSError, ValueError, json.JSONDecodeError) as error:
        print(f"[assemble-target-release-evidence] {error}", file=sys.stderr)
        return 1

    with open(args.output, "w", encoding="utf-8") as handle:
        json.dump(manifest, handle, indent=2)
        handle.write("\n")
    print(f"[assemble-target-release-evidence] wrote {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
