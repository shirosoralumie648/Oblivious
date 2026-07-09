#!/usr/bin/env python3
import argparse
import json
import os
import pathlib
import re
import sys
from datetime import datetime
from urllib import parse, request

import target_evidence_source


ARTIFACT_KIND = "request-log-observability"
PLATFORM_PROOF_FIELDS = [
    "clickHouseDeployment",
    "clickHouseMigration",
    "requestLogsTable",
    "ingestQuerySmoke",
]
PASS_FIELDS = [
    "clickHouseDeployment",
    "clickHouseMigration",
    "requestLogsTable",
    "ingestQuerySmoke",
    "requestUsageJoin",
    "latencySLOTrigger",
    "latencySLOAlertDelivery",
    "latencySLORecoveryAction",
]
TARGET_COVERAGE_PATH = "/api/v1/admin/billing/reconciliation/usage-request-logs"
SECRET_QUERY_RE = re.compile(r"(token|password|passwd|pass|secret|api[_-]?key|signature|session)", re.IGNORECASE)
PLACEHOLDER_PATTERN = re.compile(r"TODO|TBD|placeholder|example|sample|fake", re.IGNORECASE)
EMBEDDED_SECRET_PATTERN = re.compile(
    r"(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|credential|private[_-]?key)",
    re.IGNORECASE,
)


def fail(message):
    raise SystemExit(f"[collect-request-log-observability-evidence] {message}")


def read_json(path_label, path):
    try:
        with open(path, "r", encoding="utf-8") as handle:
            payload = json.load(handle)
    except FileNotFoundError:
        fail(f"{path_label} file is required: {path}")
    except json.JSONDecodeError as error:
        fail(f"{path_label} must be valid JSON: {error}")
    if not isinstance(payload, dict):
        fail(f"{path_label} must be a JSON object")
    return payload


def require_url(value, name):
    return target_evidence_source.require_url(value, name, fail)


def build_coverage_url(args):
    if args.coverage_url:
        return require_url(args.coverage_url, "coverage-url")
    base_url = require_url(args.target_base_url, "target-base-url").rstrip("/")
    query_pairs = []
    for raw in args.coverage_query:
        if "=" not in raw:
            fail("coverage-query entries must be key=value")
        key, value = raw.split("=", 1)
        key = key.strip()
        if not key:
            fail("coverage-query key is required")
        if SECRET_QUERY_RE.search(parse.unquote_plus(key)):
            fail("coverage-query must not include secret-like query parameters")
        query_pairs.append((key, value))
    query = parse.urlencode(query_pairs)
    return base_url + TARGET_COVERAGE_PATH + (("?" + query) if query else "")


def read_secret_file(path_label, path):
    try:
        return pathlib.Path(path).read_text(encoding="utf-8").strip()
    except FileNotFoundError:
        fail(f"{path_label} file is required: {path}")


def request_headers(args):
    headers = {"Accept": "application/json"}
    bearer_token = ""
    if args.bearer_token_file:
        bearer_token = read_secret_file("bearer-token", args.bearer_token_file)
    elif args.bearer_token_env:
        bearer_token = os.environ.get(args.bearer_token_env, "").strip()
    if bearer_token:
        headers["Authorization"] = f"Bearer {bearer_token}"
    if args.cookie_file:
        cookie_value = read_secret_file("cookie", args.cookie_file)
        if cookie_value:
            headers["Cookie"] = cookie_value
    return headers


def read_json_url(label, url, args):
    if args.timeout_seconds <= 0:
        fail("timeout-seconds must be positive")
    req = request.Request(url, headers=request_headers(args), method="GET")
    try:
        with request.urlopen(req, timeout=args.timeout_seconds) as response:
            status = getattr(response, "status", response.getcode())
            body = response.read(2 * 1024 * 1024 + 1)
    except Exception as error:
        fail(f"{label} fetch failed: {error}")
    if status < 200 or status >= 300:
        fail(f"{label} fetch returned HTTP {status}")
    if len(body) > 2 * 1024 * 1024:
        fail(f"{label} response exceeded 2MiB")
    try:
        payload = json.loads(body.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        fail(f"{label} response must be JSON: {error}")
    if not isinstance(payload, dict):
        fail(f"{label} response must be a JSON object")
    data = payload.get("data")
    if isinstance(data, dict) and "checkedRecords" in data:
        return data
    return payload


def read_coverage(args):
    if args.coverage_file:
        return read_json("coverage", args.coverage_file)
    return read_json_url("coverage", build_coverage_url(args), args)


def read_platform_proof(args):
    if args.platform_proof_file:
        return read_json("platform-proof", args.platform_proof_file)
    platform_proof_url = require_url(args.platform_proof_url, "platform-proof-url")
    return target_evidence_source.read_json_url(
        "platform-proof",
        platform_proof_url,
        args,
        fail,
        PLATFORM_PROOF_FIELDS,
    )


def read_slo(args):
    if args.slo_file:
        return read_json("slo", args.slo_file)
    slo_url = require_url(args.slo_url, "slo-url")
    return target_evidence_source.read_json_url(
        "slo",
        slo_url,
        args,
        fail,
        ("latencySLOTrigger", "triggeredAlerts"),
    )


def build_collection_source(args, recorded_at):
    collected_at = target_evidence_source.require_collected_at(recorded_at, fail)
    if args.coverage_file:
        return {"type": "file", "collectedAt": collected_at}
    if args.coverage_url:
        return {
            "type": "target-url",
            "url": target_evidence_source.sanitized_url(args.coverage_url, "coverage-url", fail),
            "collectedAt": collected_at,
        }
    return {
        "type": "target-api",
        "url": build_coverage_url(args),
        "collectedAt": collected_at,
    }


def build_platform_proof_source(args, recorded_at):
    collected_at = target_evidence_source.require_collected_at(recorded_at, fail)
    if args.platform_proof_file:
        return {"type": "file", "collectedAt": collected_at}
    return {
        "type": "target-url",
        "url": target_evidence_source.sanitized_url(args.platform_proof_url, "platform-proof-url", fail),
        "collectedAt": collected_at,
    }


def build_slo_proof_source(args, recorded_at):
    collected_at = target_evidence_source.require_collected_at(recorded_at, fail)
    if args.slo_file:
        return {"type": "file", "collectedAt": collected_at}
    return {
        "type": "target-url",
        "url": target_evidence_source.sanitized_url(args.slo_url, "slo-url", fail),
        "collectedAt": collected_at,
    }


def require_nonempty(value, name):
    if not isinstance(value, str) or value.strip() == "":
        fail(f"{name} is required")
    return value.strip()


def require_concrete_id(value, name):
    value = require_nonempty(value, name)
    if PLACEHOLDER_PATTERN.search(value):
        fail(f"{name} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        fail(f"{name} must not embed secret material")
    return value


def require_iso8601(value, name):
    value = require_nonempty(value, name)
    try:
        datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        fail(f"{name} must be ISO-8601")
    return value


def require_iso8601_interval(value, name):
    value = require_nonempty(value, name)
    parts = [part.strip() for part in value.split("/")]
    if len(parts) != 2 or any(part == "" for part in parts):
        fail(f"{name} must be an ISO-8601 start/end interval")
    try:
        starts_at = datetime.fromisoformat(parts[0].replace("Z", "+00:00"))
        ends_at = datetime.fromisoformat(parts[1].replace("Z", "+00:00"))
    except ValueError:
        fail(f"{name} must be an ISO-8601 start/end interval")
    if ends_at < starts_at:
        fail(f"{name} end must be at or after start")
    return value


def require_safe_artifact_id(value):
    value = require_nonempty(value, "artifact-id")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", value):
        fail("artifact-id must use only letters, numbers, dot, underscore, and dash")
    return value


def require_count(payload, key):
    value = payload.get(key)
    if not isinstance(value, int) or value < 0:
        fail(f"coverage.{key} must be a non-negative integer")
    return value


def build_request_usage_join_proof(coverage):
    checked = require_count(coverage, "checkedRecords")
    with_request_id = require_count(coverage, "usageRowsWithRequestId")
    missing_request_id = require_count(coverage, "usageRowsMissingRequestId")
    matched = require_count(coverage, "matchedRequestLogRecords")
    missing_logs = require_count(coverage, "missingRequestLogRecords")
    issues = coverage.get("issues", [])
    if not isinstance(issues, list):
        fail("coverage.issues must be an array")
    if checked <= 0:
        fail("requestUsageJoin requires at least one checked usage record")
    if missing_request_id != 0:
        fail("requestUsageJoin requires zero usage rows missing request_id")
    if missing_logs != 0:
        fail("requestUsageJoin requires zero missing request-log records")
    if with_request_id != checked:
        fail("requestUsageJoin requires every checked usage row to carry request_id")
    if matched != with_request_id:
        fail("requestUsageJoin requires every request_id to join a request-log row")
    if issues:
        fail("requestUsageJoin requires an empty coverage issues list")
    return "pass"


def require_slo_pass(payload, key):
    if payload.get(key) != "pass":
        fail(f"{key} must be pass")
    return "pass"


def require_object(value, name):
    if not isinstance(value, dict):
        fail(f"{name} is required")
    return value


def require_positive_int(payload, key, name):
    value = payload.get(key)
    if type(value) is not int or value <= 0:
        fail(f"{name}.{key} must be greater than zero")
    return value


def require_zero_int(payload, key, name):
    value = payload.get(key)
    if type(value) is not int or value != 0:
        fail(f"{name}.{key} must be zero")
    return value


def require_string_array(payload, key, name):
    value = payload.get(key)
    if (
        not isinstance(value, list)
        or not value
        or any(not isinstance(item, str) or item.strip() == "" for item in value)
    ):
        fail(f"{name}.{key} must be a non-empty array of strings")
    return [item.strip() for item in value]


def build_slo_proof(slo):
    triggered_alerts = require_positive_int(slo, "triggeredAlerts", "slo")
    alert_delivery = require_object(slo.get("alertDelivery"), "slo.alertDelivery")
    configured_providers = require_positive_int(alert_delivery, "configuredProviders", "slo.alertDelivery")
    delivered_alerts = require_positive_int(alert_delivery, "deliveredAlerts", "slo.alertDelivery")
    require_zero_int(alert_delivery, "failedDeliveries", "slo.alertDelivery")
    if delivered_alerts < triggered_alerts:
        fail("slo.alertDelivery.deliveredAlerts must be at least slo.triggeredAlerts")
    recovery_audit = require_object(slo.get("recoveryAudit"), "slo.recoveryAudit")
    audit_records = require_positive_int(recovery_audit, "auditRecords", "slo.recoveryAudit")
    require_zero_int(recovery_audit, "failedActions", "slo.recoveryAudit")
    if audit_records < triggered_alerts:
        fail("slo.recoveryAudit.auditRecords must be at least slo.triggeredAlerts")
    return {
        "latencySLOTrigger": require_slo_pass(slo, "latencySLOTrigger"),
        "latencySLOAlertDelivery": require_slo_pass(slo, "latencySLOAlertDelivery"),
        "latencySLORecoveryAction": require_slo_pass(slo, "latencySLORecoveryAction"),
        "window": require_iso8601_interval(slo.get("window"), "slo.window"),
        "triggeredAlerts": triggered_alerts,
        "alertDelivery": {
            "configuredProviders": configured_providers,
            "deliveredAlerts": delivered_alerts,
            "failedDeliveries": 0,
            "channels": require_string_array(alert_delivery, "channels", "slo.alertDelivery"),
            "lastDeliveryId": require_concrete_id(alert_delivery.get("lastDeliveryId"), "slo.alertDelivery.lastDeliveryId"),
        },
        "recoveryAudit": {
            "auditRecords": audit_records,
            "failedActions": 0,
            "lastRecordId": require_concrete_id(recovery_audit.get("lastRecordId"), "slo.recoveryAudit.lastRecordId"),
        },
    }


def require_platform_pass(payload, key):
    if payload.get(key) != "pass":
        fail(f"platform-proof.{key} must be pass")
    return "pass"


def build_artifact(args):
    artifact_id = require_safe_artifact_id(args.artifact_id)
    recorded_at = require_iso8601(args.recorded_at, "recorded-at")
    collection_source = build_collection_source(args, recorded_at)
    platform_proof_source = build_platform_proof_source(args, recorded_at)
    slo_proof_source = build_slo_proof_source(args, recorded_at)
    coverage = read_coverage(args)
    platform_proof = read_platform_proof(args)
    slo = read_slo(args)
    slo_proof = build_slo_proof(slo)

    proofs = {
        "clickHouseDeployment": require_platform_pass(platform_proof, "clickHouseDeployment"),
        "clickHouseMigration": require_platform_pass(platform_proof, "clickHouseMigration"),
        "requestLogsTable": require_platform_pass(platform_proof, "requestLogsTable"),
        "ingestQuerySmoke": require_platform_pass(platform_proof, "ingestQuerySmoke"),
        "requestUsageJoin": build_request_usage_join_proof(coverage),
        "latencySLOTrigger": slo_proof["latencySLOTrigger"],
        "latencySLOAlertDelivery": slo_proof["latencySLOAlertDelivery"],
        "latencySLORecoveryAction": slo_proof["latencySLORecoveryAction"],
    }
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": recorded_at,
        "collectionSource": collection_source,
        "platformProofSource": platform_proof_source,
        "sloProofSource": slo_proof_source,
        "proofs": proofs,
        "coverage": {
            "checkedRecords": coverage["checkedRecords"],
            "usageRowsWithRequestId": coverage["usageRowsWithRequestId"],
            "usageRowsMissingRequestId": coverage["usageRowsMissingRequestId"],
            "matchedRequestLogRecords": coverage["matchedRequestLogRecords"],
            "missingRequestLogRecords": coverage["missingRequestLogRecords"],
            "issues": coverage.get("issues", []),
        },
        "slo": {
            "window": slo_proof["window"],
            "triggeredAlerts": slo_proof["triggeredAlerts"],
            "alertDelivery": slo_proof["alertDelivery"],
            "recoveryAudit": slo_proof["recoveryAudit"],
        },
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifact-id", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--recorded-at", required=True)
    coverage_source = parser.add_mutually_exclusive_group(required=True)
    coverage_source.add_argument("--coverage-file")
    coverage_source.add_argument("--coverage-url")
    coverage_source.add_argument("--target-base-url")
    platform_proof_source = parser.add_mutually_exclusive_group(required=True)
    platform_proof_source.add_argument("--platform-proof-file")
    platform_proof_source.add_argument("--platform-proof-url")
    parser.add_argument("--coverage-query", action="append", default=[])
    parser.add_argument("--bearer-token-env", default="OBLIVIOUS_TARGET_ADMIN_BEARER_TOKEN")
    parser.add_argument("--bearer-token-file")
    parser.add_argument("--cookie-file")
    parser.add_argument("--timeout-seconds", type=float, default=10.0)
    slo_source = parser.add_mutually_exclusive_group(required=True)
    slo_source.add_argument("--slo-file")
    slo_source.add_argument("--slo-url")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    artifact = build_artifact(args)
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(artifact, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
    print(f"[collect-request-log-observability-evidence] wrote {output}")


if __name__ == "__main__":
    main()
