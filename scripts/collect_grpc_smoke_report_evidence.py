#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "grpc-smoke-report"
SERVICES = ("agent", "workflow", "task")
GO_DURATION_PATTERN = re.compile(r"^(?:(?:\d+(?:\.\d+)?|\.\d+)(?:ns|us|ms|s|m|h))+$")
EXPECTED_STATUS = {
    "agent": "validation_error",
    "workflow": "validation_response",
    "task": "validation_response",
}


def fail(message):
    raise SystemExit(f"[collect-grpc-smoke-report-evidence] {message}")


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


def require_timeout(value):
    value = require_nonempty(value, "timeout")
    if not GO_DURATION_PATTERN.match(value):
        fail("timeout must be a positive Go duration string")
    numbers = re.findall(r"(?:\d+(?:\.\d+)?|\.\d+)(?=ns|us|ms|s|m|h)", value)
    if not any(float(number) > 0 for number in numbers):
        fail("timeout must be a positive Go duration string")
    return value


def build_results(proof):
    results = proof.get("results")
    if not isinstance(results, list):
        fail("results must be an array")
    by_service = {}
    for index, result in enumerate(results):
        if not isinstance(result, dict):
            fail(f"results[{index}] must be an object")
        service = require_nonempty(result.get("service"), f"results[{index}].service")
        if service not in SERVICES:
            fail(f"results[{index}].service must be agent, workflow, or task")
        if service in by_service:
            fail(f"results must not duplicate {service} service results")
        address = require_nonempty(result.get("address"), f"results[{index}].address")
        if result.get("generatedClient") != "pass":
            fail(f"results[{index}].generatedClient must be pass")
        status = require_nonempty(result.get("status"), f"results[{index}].status")
        expected_status = EXPECTED_STATUS[service]
        if status != expected_status:
            fail(f"results[{index}].status for {service} must be {expected_status}")
        by_service[service] = {
            "service": service,
            "address": address,
            "generatedClient": "pass",
            "status": status,
        }
    missing = [service for service in SERVICES if service not in by_service]
    if missing:
        fail(f"results must include agent, workflow, and task smoke results (missing: {', '.join(missing)})")
    return [by_service[service] for service in SERVICES]


def build_artifact(args):
    proof = target_evidence_source.read_proof(args, read_json, fail, ["recordedAt", "timeout", "results"])
    artifact_id = require_safe_artifact_id(args.artifact_id)
    recorded_at = require_iso8601(args.recorded_at, "recorded-at")
    smoke_recorded_at = require_iso8601(proof.get("recordedAt"), "proof.recordedAt")
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": recorded_at,
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "result": "pass",
        "smokeReportRecordedAt": smoke_recorded_at,
        "timeout": require_timeout(proof.get("timeout")),
        "results": build_results(proof),
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
    print(f"[collect-grpc-smoke-report-evidence] wrote {output}")


if __name__ == "__main__":
    main()
