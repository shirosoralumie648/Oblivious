#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "microservice-database-proof"
SERVICE_FIELDS = [
    "relay",
    "chat",
    "workflow",
    "rag",
    "agent",
    "billing",
    "marketplace",
    "admin",
    "channel",
    "task",
    "observability",
]
PASS_FIELDS = SERVICE_FIELDS + ["migrationReadiness"]
PASS_FIELD_ERRORS = {field: f"{field} must be pass" for field in PASS_FIELDS}
PLACEHOLDER_PATTERN = re.compile(r"TODO|TBD|placeholder|example|sample|fake", re.IGNORECASE)
EMBEDDED_SECRET_PATTERN = re.compile(
    r"(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|credential|private[_-]?key)",
    re.IGNORECASE,
)


def fail(message):
    raise SystemExit(f"[collect-microservice-database-evidence] {message}")


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


def require_detail_pass(payload, key, label):
    if payload.get(key) != "pass":
        fail(f"{label}.{key} must be pass")
    return "pass"


def require_concrete_string(value, name):
    value = require_nonempty(value, name)
    if PLACEHOLDER_PATTERN.search(value):
        fail(f"{name} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        fail(f"{name} must not embed secret material")
    return value


def require_count(payload, key, positive=False):
    value = payload.get(key)
    if not isinstance(value, int) or value < 0:
        fail(f"summary.{key} must be a non-negative integer")
    if positive and value <= 0:
        fail(f"summary.{key} must be greater than zero")
    return value


def require_mode(proof):
    if proof.get("mode") != "microservices":
        fail("mode must be microservices")
    if proof.get("serviceUrlClass") != "external-filled":
        fail("serviceUrlClass must be external-filled")


def build_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        fail("summary must be a JSON object")
    services_checked = require_count(summary, "servicesChecked", positive=True)
    external_urls_checked = require_count(summary, "externalUrlsChecked", positive=True)
    migration_readiness_checks = require_count(summary, "migrationReadinessChecks", positive=True)
    if services_checked != len(SERVICE_FIELDS):
        fail(f"summary.servicesChecked must equal {len(SERVICE_FIELDS)}")
    if external_urls_checked != services_checked:
        fail("summary.externalUrlsChecked must equal summary.servicesChecked")
    if migration_readiness_checks != services_checked:
        fail("summary.migrationReadinessChecks must equal summary.servicesChecked")
    return {
        "servicesChecked": services_checked,
        "externalUrlsChecked": external_urls_checked,
        "migrationReadinessChecks": migration_readiness_checks,
    }


def build_service_details(proof):
    services = proof.get("services")
    if not isinstance(services, list) or not services:
        fail("services must be a non-empty array")
    output = []
    seen = set()
    for index, item in enumerate(services):
        label = f"services[{index}]"
        if not isinstance(item, dict):
            fail(f"{label} must be a JSON object")
        name = require_concrete_string(item.get("name"), f"{label}.name").strip().lower()
        if name in seen:
            fail(f"services must not duplicate {name}")
        seen.add(name)
        if item.get("databaseUrlClass") != "external-filled":
            fail(f"{label}.databaseUrlClass must be external-filled")
        output.append(
            {
                "name": name,
                "databaseUrlClass": "external-filled",
                "migrationReadiness": require_detail_pass(item, "migrationReadiness", label),
                "evidenceId": require_concrete_string(item.get("evidenceId"), f"{label}.evidenceId"),
            }
        )
    missing = [name for name in SERVICE_FIELDS if name not in seen]
    if missing:
        fail("services must include relay, chat, workflow, rag, agent, billing, marketplace, admin, channel, task, and observability (missing: " + ", ".join(missing) + ")")
    return output


def build_artifact(args):
    proof = target_evidence_source.read_proof(args, read_json, fail, PASS_FIELDS)
    require_mode(proof)
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
        "mode": "microservices",
        "serviceUrlClass": "external-filled",
        "proofs": proofs,
        "services": build_service_details(proof),
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
    print(f"[collect-microservice-database-evidence] wrote {output}")


if __name__ == "__main__":
    main()
