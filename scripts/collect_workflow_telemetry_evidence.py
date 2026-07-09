#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "workflow-telemetry"


def fail(message):
    raise SystemExit(f"[collect-workflow-telemetry-evidence] {message}")


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


def require_iso8601_window(value):
    value = require_nonempty(value, "telemetry.window")
    parts = [part.strip() for part in value.split("/")]
    if len(parts) != 2 or any(part == "" for part in parts):
        fail("telemetry.window must be an ISO-8601 start/end interval")
    try:
        starts_at = datetime.fromisoformat(parts[0].replace("Z", "+00:00"))
        ends_at = datetime.fromisoformat(parts[1].replace("Z", "+00:00"))
    except ValueError:
        fail("telemetry.window must be an ISO-8601 start/end interval")
    if ends_at < starts_at:
        fail("telemetry.window end must be at or after start")
    return value


def require_safe_artifact_id(value):
    value = require_nonempty(value, "artifact-id")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", value):
        fail("artifact-id must use only letters, numbers, dot, underscore, and dash")
    return value


def require_number(payload, key):
    value = payload.get(key)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        fail(f"telemetry.{key} must be numeric")
    return float(value)


def require_count(payload, key, positive=False):
    value = payload.get(key)
    if type(value) is not int or value < 0:
        fail(f"telemetry.{key} must be a non-negative integer")
    if positive and value <= 0:
        fail(f"telemetry.{key} must be greater than zero")
    return value


def build_telemetry(payload):
    telemetry = payload.get("telemetry") if isinstance(payload.get("telemetry"), dict) else payload
    success_rate = require_number(telemetry, "successRate")
    if success_rate < 0.99 or success_rate > 1.0:
        fail("telemetry.successRate must be between 0.99 and 1.0")
    window = require_iso8601_window(telemetry.get("window"))
    total_executions = require_count(telemetry, "totalExecutions", positive=True)
    successful_executions = require_count(telemetry, "successfulExecutions")
    failed_executions = require_count(telemetry, "failedExecutions")
    if successful_executions + failed_executions != total_executions:
        fail("telemetry.successfulExecutions plus telemetry.failedExecutions must equal telemetry.totalExecutions")
    computed_success_rate = successful_executions / total_executions
    if abs(computed_success_rate - success_rate) > 0.0005:
        fail("telemetry.successRate must equal telemetry.successfulExecutions / telemetry.totalExecutions")
    return {
        "successRate": success_rate,
        "window": window,
        "totalExecutions": total_executions,
        "successfulExecutions": successful_executions,
        "failedExecutions": failed_executions,
    }


def build_artifact(args):
    payload = target_evidence_source.read_proof(args, read_json, fail, ("successRate", "telemetry"))
    artifact_id = require_safe_artifact_id(args.artifact_id)
    recorded_at = require_iso8601(args.recorded_at, "recorded-at")
    return {
        "artifactId": artifact_id,
        "kind": ARTIFACT_KIND,
        "commit": require_nonempty(args.commit, "commit"),
        "runId": require_nonempty(args.run_id, "run-id"),
        "recordedAt": recorded_at,
        "collectionSource": target_evidence_source.proof_collection_source(args, fail),
        "result": "pass",
        "telemetry": build_telemetry(payload),
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
    print(f"[collect-workflow-telemetry-evidence] wrote {output}")


if __name__ == "__main__":
    main()
