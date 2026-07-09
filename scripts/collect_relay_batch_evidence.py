#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "relay-batch-proof"
DISABLED_MODE = "disabled_until_commercial_lifecycle"
LIVE_MODE = "commercial_lifecycle_enabled"
DISABLED_PASS_FIELDS = ["productionPolicyDisabled", "prebillPollingSettlementRefundAuditUsageBlockers"]
LIVE_PASS_FIELDS = [
    "productionPolicyEnabled",
    "prebillReservation",
    "pollingCompletion",
    "settlement",
    "refund",
    "usageAudit",
]
PASS_FIELD_ERRORS = {
    "productionPolicyDisabled": "productionPolicyDisabled must be pass",
    "prebillPollingSettlementRefundAuditUsageBlockers": "prebillPollingSettlementRefundAuditUsageBlockers must be pass",
    "productionPolicyEnabled": "productionPolicyEnabled must be pass",
    "prebillReservation": "prebillReservation must be pass",
    "pollingCompletion": "pollingCompletion must be pass",
    "settlement": "settlement must be pass",
    "refund": "refund must be pass",
    "usageAudit": "usageAudit must be pass",
}


def fail(message):
    raise SystemExit(f"[collect-relay-batch-evidence] {message}")


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


def require_nonempty(value, name):
    if not isinstance(value, str) or value.strip() == "":
        fail(f"{name} is required")
    return value.strip()


def require_iso8601(value, name):
    value = require_nonempty(value, name)
    try:
        datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        fail(f"{name} must be ISO-8601")
    return value


def require_safe_artifact_id(value):
    value = require_nonempty(value, "artifact-id")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", value):
        fail("artifact-id must use only letters, numbers, dot, underscore, and dash")
    return value


def require_pass(payload, key):
    if payload.get(key) != "pass":
        fail(PASS_FIELD_ERRORS[key])
    return "pass"


def require_count(payload, key, positive=False):
    value = payload.get(key)
    if not isinstance(value, int) or value < 0:
        fail(f"summary.{key} must be a non-negative integer")
    if positive and value <= 0:
        fail(f"summary.{key} must be greater than zero")
    return value


def require_mode(proof):
    mode = proof.get("mode")
    if mode not in (DISABLED_MODE, LIVE_MODE):
        fail("mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled")
    return mode


def build_disabled_summary(summary):
    production_policy_checks = require_count(summary, "productionPolicyChecks", positive=True)
    blocker_checks = require_count(summary, "prebillPollingSettlementRefundAuditUsageBlockerChecks", positive=True)
    if blocker_checks < 6:
        fail("summary.prebillPollingSettlementRefundAuditUsageBlockerChecks must cover prebill, polling, settlement, refund, audit, and usage blockers")
    return {
        "productionPolicyChecks": production_policy_checks,
        "prebillPollingSettlementRefundAuditUsageBlockerChecks": blocker_checks,
    }


def build_live_summary(summary):
    prebill_reservations = require_count(summary, "prebillReservations", positive=True)
    polling_completions = require_count(summary, "pollingCompletions", positive=True)
    settlement_records = require_count(summary, "settlementRecords", positive=True)
    refund_records = require_count(summary, "refundRecords", positive=True)
    usage_audit_records = require_count(summary, "usageAuditRecords", positive=True)
    request_log_audit_records = require_count(summary, "requestLogAuditRecords", positive=True)
    terminal_failure_records = require_count(summary, "terminalFailureRecords", positive=True)
    if usage_audit_records < settlement_records + refund_records:
        fail("summary.usageAuditRecords must cover summary.settlementRecords plus summary.refundRecords")
    if request_log_audit_records < settlement_records + refund_records:
        fail("summary.requestLogAuditRecords must cover summary.settlementRecords plus summary.refundRecords")
    if terminal_failure_records < refund_records:
        fail("summary.terminalFailureRecords must cover summary.refundRecords")
    return {
        "prebillReservations": prebill_reservations,
        "pollingCompletions": polling_completions,
        "settlementRecords": settlement_records,
        "refundRecords": refund_records,
        "usageAuditRecords": usage_audit_records,
        "requestLogAuditRecords": request_log_audit_records,
        "terminalFailureRecords": terminal_failure_records,
    }


def build_summary(proof, mode):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        fail("summary must be a JSON object")
    if mode == DISABLED_MODE:
        return build_disabled_summary(summary)
    return build_live_summary(summary)


def build_artifact(args):
    proof = target_evidence_source.read_proof(args, read_json, fail, ["mode", *DISABLED_PASS_FIELDS, *LIVE_PASS_FIELDS])
    mode = require_mode(proof)
    artifact_id = require_safe_artifact_id(args.artifact_id)
    recorded_at = require_iso8601(args.recorded_at, "recorded-at")
    pass_fields = LIVE_PASS_FIELDS if mode == LIVE_MODE else DISABLED_PASS_FIELDS
    proofs = {field: require_pass(proof, field) for field in pass_fields}
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": recorded_at,
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "mode": mode,
        "proofs": proofs,
        "summary": build_summary(proof, mode),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifact-id", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--recorded-at", required=True)
    target_evidence_source.add_proof_source_args(parser)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    artifact = build_artifact(args)
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(artifact, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
    print(f"[collect-relay-batch-evidence] wrote {output}")


if __name__ == "__main__":
    main()
