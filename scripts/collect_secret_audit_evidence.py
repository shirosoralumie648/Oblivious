#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "secret-audit"
REQUIRED_SCOPES = ["kubernetes", "providers", "runtime"]
SUMMARY_COUNT_FIELDS = [
    "totalRecordsScanned",
    "protectedRecords",
    "plaintextRecords",
    "invalidProtectedRecords",
    "rotationRequiredRecords",
]
EMBEDDED_SECRET_PATTERN = re.compile(
    r"sk_(live|test)_[A-Za-z0-9]{12,}|rk_(live|test)_[A-Za-z0-9]{12,}|"
    r"AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|"
    r"gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}"
)
SECRET_KEY_PATTERN = re.compile(r"secret|password|token|api[_-]?key|private[_-]?key|credential|kubeconfig", re.IGNORECASE)


def fail(message):
    raise SystemExit(f"[collect-secret-audit-evidence] {message}")


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


def blank(value):
    return value is None or (hasattr(value, "__len__") and len(value) == 0)


def path_label(path):
    return ".".join(str(part) for part in path)


def collect_secret_material(value, path=None, findings=None):
    if path is None:
        path = []
    if findings is None:
        findings = []
    if path and SECRET_KEY_PATTERN.search(str(path[-1])) and not blank(value):
        findings.append(f"{path_label(path)} must not embed secret material")
    if isinstance(value, str) and EMBEDDED_SECRET_PATTERN.search(value):
        findings.append(f"{path_label(path)} looks like an embedded secret value")
    if isinstance(value, dict):
        for key, child in value.items():
            collect_secret_material(child, path + [key], findings)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            collect_secret_material(child, path + [index], findings)
    return findings


def require_scope(proof):
    scope = proof.get("scope")
    if not isinstance(scope, list) or not scope or any(not isinstance(item, str) or item.strip() == "" for item in scope):
        fail("scope must be a non-empty array of strings")
    normalized = []
    output = []
    for item in scope:
        stripped = item.strip()
        lowered = stripped.lower()
        if lowered not in normalized:
            normalized.append(lowered)
            output.append(stripped)
    missing_scopes = [item for item in REQUIRED_SCOPES if item not in normalized]
    if missing_scopes:
        fail(f"scope must include kubernetes, providers, and runtime (missing: {', '.join(missing_scopes)})")
    return output


def require_findings(proof):
    findings = proof.get("findings")
    if not isinstance(findings, list):
        fail("findings must be an array")
    if findings:
        fail("findings must be an empty array")
    return []


def require_secret_audit_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        fail("summary must be a JSON object")
    output = {}
    for field in SUMMARY_COUNT_FIELDS:
        value = summary.get(field)
        if isinstance(value, bool) or not isinstance(value, int):
            fail(f"summary.{field} must be an integer")
        if value < 0:
            fail(f"summary.{field} must be non-negative")
        output[field] = value
    if output["totalRecordsScanned"] <= 0:
        fail("summary.totalRecordsScanned must be greater than zero")
    for field in ["plaintextRecords", "invalidProtectedRecords", "rotationRequiredRecords"]:
        if output[field] != 0:
            fail(f"summary.{field} must be zero")
    if output["protectedRecords"] > output["totalRecordsScanned"]:
        fail("summary.protectedRecords must not exceed summary.totalRecordsScanned")
    return output


def build_artifact(args):
    proof = target_evidence_source.read_proof(args, read_json, fail, ["result", "scope", "findings"])
    secret_findings = collect_secret_material(proof, ["proof"])
    if secret_findings:
        fail(secret_findings[0])
    if proof.get("result") != "pass":
        fail("result must be pass")
    artifact_id = require_safe_artifact_id(args.artifact_id)
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": require_iso8601(args.recorded_at, "recorded-at"),
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "result": "pass",
        "checkedAt": require_iso8601(proof.get("checkedAt"), "checkedAt"),
        "scope": require_scope(proof),
        "findings": require_findings(proof),
        "summary": require_secret_audit_summary(proof),
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
    print(f"[collect-secret-audit-evidence] wrote {output}")


if __name__ == "__main__":
    main()
