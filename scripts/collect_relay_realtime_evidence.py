#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "relay-realtime-proof"
DISABLED_MODE = "disabled_until_commercial_lifecycle"
LIVE_MODE = "commercial_lifecycle_enabled"
DISABLED_PASS_FIELDS = ["productionPolicyDisabled", "authOriginPrebillAbortUsageBlockers"]
LIVE_PASS_FIELDS = [
    "productionPolicyEnabled",
    "authPolicy",
    "originPolicy",
    "prebillSettlement",
    "abortSettlement",
    "usageLedger",
]
PASS_FIELD_ERRORS = {
    "productionPolicyDisabled": "productionPolicyDisabled must be pass",
    "authOriginPrebillAbortUsageBlockers": "authOriginPrebillAbortUsageBlockers must be pass",
    "productionPolicyEnabled": "productionPolicyEnabled must be pass",
    "authPolicy": "authPolicy must be pass",
    "originPolicy": "originPolicy must be pass",
    "prebillSettlement": "prebillSettlement must be pass",
    "abortSettlement": "abortSettlement must be pass",
    "usageLedger": "usageLedger must be pass",
}


def fail(message):
    raise SystemExit(f"[collect-relay-realtime-evidence] {message}")


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
    blocker_checks = require_count(summary, "authOriginPrebillAbortUsageBlockerChecks", positive=True)
    if blocker_checks < 5:
        fail("summary.authOriginPrebillAbortUsageBlockerChecks must cover auth, origin, prebill, abort, and usage blockers")
    return {
        "productionPolicyChecks": production_policy_checks,
        "authOriginPrebillAbortUsageBlockerChecks": blocker_checks,
    }


def build_live_summary(summary):
    total_requests = require_count(summary, "totalRequests", positive=True)
    authenticated_requests = require_count(summary, "authenticatedRequests", positive=True)
    request_linked_usage_records = require_count(summary, "requestLinkedUsageRecords", positive=True)
    price_snapshot_records = require_count(summary, "priceSnapshotRecords", positive=True)
    abort_settlement_records = require_count(summary, "abortSettlementRecords", positive=True)
    terminal_usage_records = require_count(summary, "terminalUsageRecords", positive=True)
    origin_policy_checks = require_count(summary, "originPolicyChecks", positive=True)
    if authenticated_requests != total_requests:
        fail("summary.authenticatedRequests must equal summary.totalRequests")
    if request_linked_usage_records != total_requests:
        fail("summary.requestLinkedUsageRecords must equal summary.totalRequests")
    if price_snapshot_records != total_requests:
        fail("summary.priceSnapshotRecords must equal summary.totalRequests")
    if terminal_usage_records != total_requests:
        fail("summary.terminalUsageRecords must equal summary.totalRequests")
    return {
        "totalRequests": total_requests,
        "authenticatedRequests": authenticated_requests,
        "requestLinkedUsageRecords": request_linked_usage_records,
        "priceSnapshotRecords": price_snapshot_records,
        "abortSettlementRecords": abort_settlement_records,
        "terminalUsageRecords": terminal_usage_records,
        "originPolicyChecks": origin_policy_checks,
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
    print(f"[collect-relay-realtime-evidence] wrote {output}")


if __name__ == "__main__":
    main()
