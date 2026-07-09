#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "provider-live-rail"
PROVIDERS = {"stripe", "alipay", "wechatpay"}
PASS_FIELDS = ["checkout", "refund", "payout", "reconciliation"]
SUMMARY_FIELDS = ["checkoutAttempts", "refundAttempts", "payoutAttempts", "reconciliationChecks"]
REFERENCE_FIELDS = PASS_FIELDS
PLACEHOLDER_PATTERN = re.compile(r"TODO|TBD|placeholder|example|sample|fake", re.IGNORECASE)
EMBEDDED_SECRET_PATTERN = re.compile(
    r"sk_(live|test)_[A-Za-z0-9]{12,}|rk_(live|test)_[A-Za-z0-9]{12,}|"
    r"AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|"
    r"gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}"
)


def fail(message):
    raise SystemExit(f"[collect-provider-live-rail-evidence] {message}")


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


def require_provider(value):
    provider = require_nonempty(value, "provider").lower()
    if provider not in PROVIDERS:
        fail("provider must be stripe, alipay, or wechatpay")
    return provider


def require_pass(payload, key):
    if payload.get(key) != "pass":
        fail(f"{key} must be pass")
    return "pass"


def require_count(payload, key):
    value = payload.get(key)
    if not isinstance(value, int) or value <= 0:
        fail(f"summary.{key} must be a positive integer")
    return value


def build_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        fail("summary must be a JSON object")
    return {field: require_count(summary, field) for field in SUMMARY_FIELDS}


def require_provider_environment(proof):
    value = proof.get("providerEnvironment")
    if value != "live":
        fail("providerEnvironment must be live")
    return "live"


def require_reference(references, key):
    value = references.get(key) if isinstance(references, dict) else None
    if not isinstance(value, str) or value.strip() == "":
        fail(f"references.{key} is required")
    value = value.strip()
    if PLACEHOLDER_PATTERN.search(value):
        fail(f"references.{key} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        fail(f"references.{key} must not embed secret material")
    return value


def build_references(proof):
    references = proof.get("references")
    if not isinstance(references, dict):
        fail("references must be a JSON object")
    return {field: require_reference(references, field) for field in REFERENCE_FIELDS}


def build_artifact(args):
    provider = require_provider(args.provider)
    proof = target_evidence_source.read_proof(args, read_json, fail, ["provider", "mode", *PASS_FIELDS])
    proof_provider = require_provider(proof.get("provider"))
    if proof_provider != provider:
        fail("proof provider must match provider")
    if proof.get("mode") != "live":
        fail("mode must be live")

    artifact_id = require_safe_artifact_id(args.artifact_id)
    recorded_at = require_iso8601(args.recorded_at, "recorded-at")
    proofs = {field: require_pass(proof, field) for field in PASS_FIELDS}
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "provider": provider,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": recorded_at,
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "mode": "live",
        "providerEnvironment": require_provider_environment(proof),
        "proofs": proofs,
        "references": build_references(proof),
        "summary": build_summary(proof),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--artifact-id", required=True)
    parser.add_argument("--provider", required=True)
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
    print(f"[collect-provider-live-rail-evidence] wrote {output}")


if __name__ == "__main__":
    main()
