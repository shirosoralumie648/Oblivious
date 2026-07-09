#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "kubernetes-validation"
PROOF_FIELDS = ["validation", "rollout", "failover"]
REFERENCE_FIELDS = PROOF_FIELDS
PLACEHOLDER_PATTERN = re.compile(r"TODO|TBD|placeholder|example|sample|fake", re.IGNORECASE)
EMBEDDED_SECRET_PATTERN = re.compile(
    r"(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)",
    re.IGNORECASE,
)


def fail(message):
    raise SystemExit(f"[collect-kubernetes-evidence] {message}")


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


def require_pass(proof, key):
    if proof.get(key) != "pass":
        fail(f"{key} must be pass")
    return "pass"


def require_secret_file_class(proof):
    if proof.get("secretFileClass") != "external-filled":
        fail("secretFileClass must be external-filled")
    return "external-filled"


def require_target_environment(proof):
    if proof.get("targetEnvironment") != "production":
        fail("targetEnvironment must be production")
    return "production"


def require_concrete_string(value, name):
    value = require_nonempty(value, name)
    if PLACEHOLDER_PATTERN.search(value):
        fail(f"{name} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        fail(f"{name} must not embed secret material")
    return value


def require_reference(references, key):
    value = references.get(key) if isinstance(references, dict) else None
    return require_concrete_string(value, f"references.{key}")


def build_references(proof):
    references = proof.get("references")
    if not isinstance(references, dict):
        fail("references must be a JSON object")
    return {field: require_reference(references, field) for field in REFERENCE_FIELDS}


def build_artifact(args):
    proof = target_evidence_source.read_proof(args, read_json, fail, PROOF_FIELDS + ["secretFileClass"])
    if proof.get("result", "pass") != "pass":
        fail("result must be pass")
    return {
        "artifactId": require_safe_artifact_id(args.artifact_id),
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": require_iso8601(args.recorded_at, "recorded-at"),
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "result": "pass",
        "targetEnvironment": require_target_environment(proof),
        "clusterRef": require_concrete_string(proof.get("clusterRef"), "clusterRef"),
        "namespace": require_concrete_string(proof.get("namespace"), "namespace"),
        "secretFileClass": require_secret_file_class(proof),
        "proofs": {field: require_pass(proof, field) for field in PROOF_FIELDS},
        "references": build_references(proof),
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
    print(f"[collect-kubernetes-evidence] wrote {output}")


if __name__ == "__main__":
    main()
