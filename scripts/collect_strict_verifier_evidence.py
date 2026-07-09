#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
import shlex
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "strict-verifier-log"
STRICT_VERIFIER_REQUIRED_ENV = [
    "COMMERCIAL_COMPLETION_RUN_DEPLOY=true",
    "COMMERCIAL_COMPLETION_RUN_K8S=true",
    "COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true",
    "COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true",
]
STRICT_VERIFIER_COMMAND_TAIL = ["bash", "scripts/verify-commercial-completion.sh"]
STRICT_VERIFIER_FORBIDDEN_ENV = {
    "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS",
    "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH",
}


def fail(message):
    raise SystemExit(f"[collect-strict-verifier-evidence] {message}")


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


def require_sha256(value, name):
    value = require_nonempty(value, name)
    if not re.fullmatch(r"[0-9a-fA-F]{64}", value):
        fail(f"{name} must be a 64-character SHA-256 hex digest")
    return value.lower()


def require_canonical_command(command):
    command = require_nonempty(command, "command")
    try:
        tokens = shlex.split(command, posix=True)
    except ValueError:
        fail("command must use the canonical strict verifier invocation")
    seen_env = {}
    for token in tokens:
        if "=" not in token:
            continue
        key, value = token.split("=", 1)
        if key in STRICT_VERIFIER_FORBIDDEN_ENV:
            fail(f"command must not enable {key}")
        seen_env[key] = value
    if tokens[-2:] != STRICT_VERIFIER_COMMAND_TAIL:
        fail("command must run scripts/verify-commercial-completion.sh")
    for required_flag in STRICT_VERIFIER_REQUIRED_ENV:
        key, value = required_flag.split("=", 1)
        if seen_env.get(key) != value:
            fail(f"command must include {required_flag}")
    expected_tokens = STRICT_VERIFIER_REQUIRED_ENV + STRICT_VERIFIER_COMMAND_TAIL
    if len(tokens) != len(expected_tokens) or sorted(tokens[:-2]) != sorted(STRICT_VERIFIER_REQUIRED_ENV):
        fail("command must use the canonical strict verifier invocation")
    return command


def require_empty_skips(value):
    if value != []:
        fail("skippedChecks must be an empty array")
    return []


def build_artifact(args):
    proof = target_evidence_source.read_proof(
        args,
        read_json,
        fail,
        [
            "command",
            "result",
            "skippedChecks",
            "startedAt",
            "completedAt",
            "commit",
            "runId",
            "targetEvidenceSha256",
            "artifactBundleSha256",
        ],
    )
    if proof.get("result") != "pass":
        fail("result must be pass")
    commit = require_nonempty(args.commit, "commit")
    run_id = require_nonempty(args.run_id, "run-id")
    if require_nonempty(proof.get("commit"), "proof.commit") != commit:
        fail("proof.commit must match --commit")
    if require_nonempty(proof.get("runId"), "proof.runId") != run_id:
        fail("proof.runId must match --run-id")
    started_at = require_iso8601(proof.get("startedAt"), "startedAt")
    completed_at = require_iso8601(proof.get("completedAt"), "completedAt")
    if datetime.fromisoformat(completed_at.replace("Z", "+00:00")) < datetime.fromisoformat(started_at.replace("Z", "+00:00")):
        fail("completedAt must be at or after startedAt")
    return {
        "artifactId": require_safe_artifact_id(args.artifact_id),
        "kind": ARTIFACT_KIND,
        "commit": commit,
        "runId": run_id,
        "recordedAt": require_iso8601(args.recorded_at, "recorded-at"),
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "result": "pass",
        "command": require_canonical_command(proof.get("command")),
        "skippedChecks": require_empty_skips(proof.get("skippedChecks")),
        "startedAt": started_at,
        "completedAt": completed_at,
        "targetEvidenceSha256": require_sha256(proof.get("targetEvidenceSha256"), "proof.targetEvidenceSha256"),
        "artifactBundleSha256": require_sha256(proof.get("artifactBundleSha256"), "proof.artifactBundleSha256"),
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
    print(f"[collect-strict-verifier-evidence] wrote {output}")


if __name__ == "__main__":
    main()
