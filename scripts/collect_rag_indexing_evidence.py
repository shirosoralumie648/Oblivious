#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "rag-indexing-proof"
PASS_FIELDS = [
    "durableQueueMigration",
    "workerDeployment",
    "enqueueDrainProbe",
    "rawParserReplay",
    "retrievalProbe",
    "staleVectorFilter",
]
PASS_FIELD_ERRORS = {
    "durableQueueMigration": "durableQueueMigration must be pass",
    "workerDeployment": "workerDeployment must be pass",
    "enqueueDrainProbe": "enqueueDrainProbe must be pass",
    "rawParserReplay": "rawParserReplay must be pass",
    "retrievalProbe": "retrievalProbe must be pass",
    "staleVectorFilter": "staleVectorFilter must be pass",
}


def fail(message):
    raise SystemExit(f"[collect-rag-indexing-evidence] {message}")


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
    queued_jobs = require_count(summary, "queuedJobs", positive=True)
    drained_jobs = require_count(summary, "drainedJobs")
    worker_completed_jobs = require_count(summary, "workerCompletedJobs", positive=True)
    raw_parser_replay_count = require_count(summary, "rawParserReplayCount", positive=True)
    retrieval_probe_count = require_count(summary, "retrievalProbeCount", positive=True)
    stale_vector_rows_filtered = require_count(summary, "staleVectorRowsFiltered", positive=True)
    if drained_jobs != queued_jobs:
        fail("summary.drainedJobs must equal summary.queuedJobs")
    if worker_completed_jobs != drained_jobs:
        fail("summary.workerCompletedJobs must equal summary.drainedJobs")
    return {
        "queuedJobs": queued_jobs,
        "drainedJobs": drained_jobs,
        "workerCompletedJobs": worker_completed_jobs,
        "rawParserReplayCount": raw_parser_replay_count,
        "retrievalProbeCount": retrieval_probe_count,
        "staleVectorRowsFiltered": stale_vector_rows_filtered,
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
    print(f"[collect-rag-indexing-evidence] wrote {output}")


if __name__ == "__main__":
    main()
