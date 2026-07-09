#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "marketplace-governance-proof"
PASS_FIELDS = [
    "reviewQueue",
    "appealQueue",
    "appealDecisionLifecycle",
    "reviewAssignment",
    "reviewSLAEnforcement",
    "abuseReportLifecycle",
]
PASS_FIELD_ERRORS = {
    "reviewQueue": "reviewQueue must be pass",
    "appealQueue": "appealQueue must be pass",
    "appealDecisionLifecycle": "appealDecisionLifecycle must be pass",
    "reviewAssignment": "reviewAssignment must be pass",
    "reviewSLAEnforcement": "reviewSLAEnforcement must be pass",
    "abuseReportLifecycle": "abuseReportLifecycle must be pass",
}


def fail(message):
    raise SystemExit(f"[collect-marketplace-governance-evidence] {message}")


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


def build_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        fail("summary must be a JSON object")
    review_queue_items = require_count(summary, "reviewQueueItems", positive=True)
    appeal_queue_items = require_count(summary, "appealQueueItems", positive=True)
    appeal_decisions = require_count(summary, "appealDecisions")
    review_assignments = require_count(summary, "reviewAssignments", positive=True)
    sla_checks = require_count(summary, "slaChecks", positive=True)
    sla_breaches_handled = require_count(summary, "slaBreachesHandled")
    abuse_reports = require_count(summary, "abuseReports", positive=True)
    abuse_reports_resolved = require_count(summary, "abuseReportsResolved")
    if review_assignments < review_queue_items:
        fail("summary.reviewAssignments must cover summary.reviewQueueItems")
    if appeal_decisions != appeal_queue_items:
        fail("summary.appealDecisions must equal summary.appealQueueItems")
    if sla_checks < review_assignments:
        fail("summary.slaChecks must cover summary.reviewAssignments")
    if abuse_reports_resolved != abuse_reports:
        fail("summary.abuseReportsResolved must equal summary.abuseReports")
    return {
        "reviewQueueItems": review_queue_items,
        "appealQueueItems": appeal_queue_items,
        "appealDecisions": appeal_decisions,
        "reviewAssignments": review_assignments,
        "slaChecks": sla_checks,
        "slaBreachesHandled": sla_breaches_handled,
        "abuseReports": abuse_reports,
        "abuseReportsResolved": abuse_reports_resolved,
    }


def build_artifact(args):
    proof = target_evidence_source.read_proof(args, read_json, fail, PASS_FIELDS)
    artifact_id = require_safe_artifact_id(args.artifact_id)
    recorded_at = require_iso8601(args.recorded_at, "recorded-at")
    proofs = {field: require_pass(proof, field) for field in PASS_FIELDS}
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": recorded_at,
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "proofs": proofs,
        "summary": build_summary(proof),
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
    print(f"[collect-marketplace-governance-evidence] wrote {output}")


if __name__ == "__main__":
    main()
