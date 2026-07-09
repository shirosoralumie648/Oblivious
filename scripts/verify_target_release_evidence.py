#!/usr/bin/env python3
import argparse
import hashlib
import ipaddress
import json
import os
import pathlib
import re
import shlex
import sys
from copy import deepcopy
from datetime import datetime, timezone
from urllib.parse import parse_qs, unquote_plus, urlsplit


STRICT_VERIFIER_REQUIRED_ENV = [
    "COMMERCIAL_COMPLETION_RUN_DEPLOY=true",
    "COMMERCIAL_COMPLETION_RUN_K8S=true",
    "COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true",
    "COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true",
]
STRICT_VERIFIER_COMMAND_TAIL = ["bash", "scripts/verify-commercial-completion.sh"]
STRICT_VERIFIER_FORBIDDEN_ENV = {
    "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS": "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS",
    "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH": "strictVerifier.command must not enable OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH",
}
ALLOWED_ENVIRONMENT_CLASSES = {"staging", "preproduction", "production"}
GO_DURATION_PATTERN = re.compile(r"^(?:(?:\d+(?:\.\d+)?|\.\d+)(?:ns|us|ms|s|m|h))+$")
PLACEHOLDER_PATTERN = re.compile(
    r"TODO|TBD|placeholder|example|sample|fake|/path/outside/git|release-log-or-artifact-id|"
    r"strict-verifier-log-or-artifact-id|strict-commercial-verifier-log|provider-run-id|"
    r"grpc-smoke-log|secret-audit-log|telemetry-dashboard-or-export",
    re.IGNORECASE,
)
SECRET_LIKE_URI_PARAMETER_NAME_PATTERN = re.compile(
    r"^(?:[^=&#]*[_-])?(?:token|secret|password|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)$",
    re.IGNORECASE,
)
SECRET_LIKE_URI_PARAMETER_VALUE_PATTERN = re.compile(
    r"(?:\b|[_-])(?:token|secret|password|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)(?:\b|[_-])",
    re.IGNORECASE,
)

EXPECTED_TARGET_SOURCE_PATHS = {
    ("request-log-observability", "collectionSource"): "/api/v1/admin/billing/reconciliation/usage-request-logs",
    ("request-log-observability", "sloProofSource"): "/api/v1/admin/observability/latency-slo-proof",
    ("rag-indexing-proof", "collectionSource"): "/api/v1/admin/release-evidence/rag-indexing",
    ("agent-sandbox-proof", "collectionSource"): "/api/v1/admin/release-evidence/agent-sandbox",
    ("relay-realtime-proof", "collectionSource"): "/api/v1/admin/release-evidence/relay-realtime",
    ("relay-batch-proof", "collectionSource"): "/api/v1/admin/release-evidence/relay-batch",
    ("marketplace-payout-proof", "collectionSource"): "/api/v1/admin/release-evidence/marketplace-payout",
    ("marketplace-governance-proof", "collectionSource"): "/api/v1/admin/release-evidence/marketplace-governance",
    ("provider-runtime-config", "collectionSource"): "/api/v1/admin/release-evidence/provider-runtime-config",
    ("microservice-database-proof", "collectionSource"): "/api/v1/admin/release-evidence/microservice-database",
}
WINDOWED_TARGET_SOURCE_KEYS = {
    ("request-log-observability", "sloProofSource"),
    ("rag-indexing-proof", "collectionSource"),
    ("agent-sandbox-proof", "collectionSource"),
    ("relay-realtime-proof", "collectionSource"),
    ("relay-batch-proof", "collectionSource"),
    ("marketplace-payout-proof", "collectionSource"),
    ("marketplace-governance-proof", "collectionSource"),
    ("provider-runtime-config", "collectionSource"),
    ("microservice-database-proof", "collectionSource"),
}
EMBEDDED_SECRET_PATTERN = re.compile(
    r"sk_(live|test)_[A-Za-z0-9]{12,}|rk_(live|test)_[A-Za-z0-9]{12,}|"
    r"AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|"
    r"gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}"
)
SECRET_KEY_PATTERN = re.compile(r"secret|password|token|api[_-]?key|private[_-]?key|credential|kubeconfig", re.IGNORECASE)
REQUIRED_GRPC_SERVICES = ["agent", "workflow", "task"]
EXPECTED_GRPC_PORTS = {"agent": "50063", "workflow": "50064", "task": "50065"}
EXPECTED_GRPC_SMOKE_STATUSES = {
    "agent": "validation_error",
    "workflow": "validation_response",
    "task": "validation_response",
}


def blank(value):
    return value is None or (hasattr(value, "__len__") and len(value) == 0)


def path_label(path):
    return ".".join(str(part) for part in path)


def dig_path(data, path):
    node = data
    for part in path:
        if isinstance(node, dict):
            node = node.get(part)
        elif isinstance(node, list) and isinstance(part, int) and 0 <= part < len(node):
            node = node[part]
        else:
            return None
    return node


def require_string(failures, data, path):
    value = dig_path(data, path)
    if not isinstance(value, str) or value.strip() == "":
        failures.append(f"{path_label(path)} is required")
    return value


def require_pass(failures, data, path):
    if dig_path(data, path) != "pass":
        failures.append(f"{path_label(path)} must be pass")


def parse_iso8601_safely(value):
    if not isinstance(value, str):
        return None
    try:
        return datetime.fromisoformat(value.strip().replace("Z", "+00:00"))
    except ValueError:
        return None


def require_iso8601_interval(failures, data, path, interval_error, ordering_error):
    value = require_string(failures, data, path)
    if not isinstance(value, str) or value.strip() == "":
        return
    parts = [part.strip() for part in value.split("/")]
    if len(parts) != 2 or any(part == "" for part in parts):
        failures.append(interval_error)
        return
    starts_at = parse_iso8601_safely(parts[0])
    ends_at = parse_iso8601_safely(parts[1])
    if starts_at is None or ends_at is None:
        failures.append(interval_error)
        return
    if ends_at < starts_at:
        failures.append(ordering_error)


def iso8601_interval_end(value):
    if not isinstance(value, str):
        return None
    parts = [part.strip() for part in value.split("/")]
    if len(parts) != 2 or any(part == "" for part in parts):
        return None
    return parse_iso8601_safely(parts[1])


def positive_go_duration_string(value):
    if not isinstance(value, str):
        return False
    duration = value.strip()
    if duration == "" or not GO_DURATION_PATTERN.match(duration):
        return False
    return any(float(match) > 0 for match in re.findall(r"(?:\d+(?:\.\d+)?|\.\d+)(?=ns|us|ms|s|m|h)", duration))


def parse_ipv4_numeric_component(value):
    component = str(value or "").strip().lower()
    if component == "":
        return None
    if component.startswith("0x"):
        if not re.fullmatch(r"0x[0-9a-f]+", component):
            return None
        return int(component, 16)
    if len(component) > 1 and component.startswith("0") and re.fullmatch(r"0[0-7]+", component):
        return int(component, 8)
    if re.fullmatch(r"\d+", component):
        return int(component, 10)
    return None


def ipv4_numeric_local_or_unspecified(host):
    parts = str(host or "").split(".")
    if len(parts) > 4 or any(part == "" for part in parts):
        return False
    numbers = []
    for part in parts:
        number = parse_ipv4_numeric_component(part)
        if number is None or number < 0:
            return False
        numbers.append(number)
    if not numbers:
        return False
    if numbers[0] in (0, 127):
        return True
    if len(numbers) != 1 or numbers[0] > 0xFFFFFFFF:
        return False
    first_octet = numbers[0] >> 24
    return first_octet in (0, 127)


def reserved_target_host(host):
    normalized = str(host or "").strip().lower()
    if normalized.startswith("[") and normalized.endswith("]"):
        normalized = normalized[1:-1]
    normalized = re.sub(r"\.+$", "", normalized)
    return normalized == "invalid" or normalized.endswith(".invalid")


def local_target_host(host):
    normalized = str(host or "").strip().lower()
    if normalized.startswith("[") and normalized.endswith("]"):
        normalized = normalized[1:-1]
    normalized = re.sub(r"\.+$", "", normalized)
    if normalized == "":
        return False
    if normalized == "0":
        return True
    if normalized == "localhost" or normalized.endswith(".localhost"):
        return True
    if ipv4_numeric_local_or_unspecified(normalized):
        return True
    try:
        ip = ipaddress.ip_address(normalized)
        if getattr(ip, "ipv4_mapped", None):
            ip = ip.ipv4_mapped
        return ip.is_loopback or int(ip) == 0
    except ValueError:
        return False


def non_target_host(host):
    return local_target_host(host) or reserved_target_host(host)


def validate_strict_verifier_command(failures, command):
    try:
        tokens = shlex.split(command, posix=True)
    except ValueError:
        failures.append("strictVerifier.command must use the canonical strict verifier invocation")
        return

    seen_env = {}
    for token in tokens:
        if "=" not in token:
            continue
        key, value = token.split("=", 1)
        if key in STRICT_VERIFIER_FORBIDDEN_ENV:
            failures.append(STRICT_VERIFIER_FORBIDDEN_ENV[key])
        seen_env[key] = value

    command_tail = tokens[-2:]
    if command_tail != STRICT_VERIFIER_COMMAND_TAIL:
        failures.append("strictVerifier.command must run scripts/verify-commercial-completion.sh")

    for required_flag in STRICT_VERIFIER_REQUIRED_ENV:
        key, value = required_flag.split("=", 1)
        if seen_env.get(key) != value:
            failures.append(f"strictVerifier.command must include {required_flag}")

    expected_tokens = STRICT_VERIFIER_REQUIRED_ENV + STRICT_VERIFIER_COMMAND_TAIL
    if (
        len(tokens) != len(expected_tokens)
        or command_tail != STRICT_VERIFIER_COMMAND_TAIL
        or sorted(tokens[:-2]) != sorted(STRICT_VERIFIER_REQUIRED_ENV)
    ):
        failures.append("strictVerifier.command must use the canonical strict verifier invocation")


def require_http_url(failures, data, path, error, local_error=None):
    value = require_string(failures, data, path)
    if not isinstance(value, str) or value.strip() == "":
        return
    parsed = urlsplit(value.strip())
    if parsed.scheme not in ("http", "https") or blank(parsed.hostname):
        failures.append(error)
        return
    if local_error and non_target_host(parsed.hostname):
        failures.append(local_error)


def endpoint_host(value):
    raw = str(value or "").strip()
    if raw == "":
        return None
    if re.match(r"^[a-z][a-z0-9+\-.]*://", raw, re.IGNORECASE):
        parsed = urlsplit(raw)
        if parsed.hostname:
            return parsed.hostname
        raw = re.sub(r"^[a-z][a-z0-9+\-.]*:/+", "", raw, flags=re.IGNORECASE)
    raw = re.sub(r"^/+", "", raw)
    if raw.startswith("["):
        match = re.match(r"^\[([^\]]+)\]", raw)
        return match.group(1) if match else None
    return re.split(r"[/:]", raw, maxsplit=1)[0]


def local_endpoint(value):
    host = endpoint_host(value)
    return not blank(host) and local_target_host(host)


def parse_plain_grpc_address(value):
    if not isinstance(value, str):
        return None
    raw = value.strip()
    if raw == "" or re.match(r"^[a-z][a-z0-9+\-.]*://", raw, re.IGNORECASE) or re.search(r"[/\?#]", raw):
        return None
    if raw.startswith("["):
        match = re.match(r"^\[([^\]]+)\]:(\d+)$", raw)
    else:
        match = re.match(r"^([^:\s\[\]]+):(\d+)$", raw)
    if not match:
        return None
    return {"host": match.group(1), "port": match.group(2)}


def placeholder(value):
    return isinstance(value, str) and PLACEHOLDER_PATTERN.search(value) is not None


def decoded_uri_component(component):
    decoded = str(component or "")
    while True:
        next_decoded = unquote_plus(decoded)
        if next_decoded == decoded:
            return decoded
        decoded = next_decoded


def secret_like_uri(value):
    if not isinstance(value, str):
        return False
    if re.search(
        r"[?&#](?:[^=&#]*[_-])?(?:token|secret|password|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)(?:=|[&#]|$)",
        value,
        re.IGNORECASE,
    ):
        return True
    for match in re.finditer(r"[?&#]([^=&#]+)(?:=([^&#]*))?", value):
        parameter_name = decoded_uri_component(match.group(1))
        parameter_value = decoded_uri_component(match.group(2)) if match.group(2) is not None else None
        if parameter_name and SECRET_LIKE_URI_PARAMETER_NAME_PATTERN.search(parameter_name):
            return True
        if parameter_value and SECRET_LIKE_URI_PARAMETER_VALUE_PATTERN.search(parameter_value):
            return True
    return False


def userinfo_uri(value):
    if not isinstance(value, str):
        return False
    parsed = urlsplit(value.strip())
    return parsed.username is not None or parsed.password is not None


def remote_artifact_uri(value):
    if not isinstance(value, str):
        return False
    stripped = value.strip()
    if stripped == "" or stripped.startswith("/"):
        return False
    parsed = urlsplit(stripped)
    if blank(parsed.scheme) or parsed.scheme.lower() == "file":
        return False
    if blank(parsed.hostname):
        return False
    if non_target_host(parsed.hostname):
        return False
    return True


def require_evidence_ref(failures, data, path):
    value = require_string(failures, data, path)
    if placeholder(value):
        failures.append(f"{path_label(path)} must reference a concrete target artifact, not a placeholder")
    return value


def collect_skips(value, path=None, skips=None):
    if path is None:
        path = []
    if skips is None:
        skips = []
    if isinstance(value, dict):
        for key, child in value.items():
            child_path = path + [key]
            if re.search(r"skip", str(key), re.IGNORECASE) and child not in ([], None, False):
                skips.append((child_path, child))
            collect_skips(child, child_path, skips)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            collect_skips(child, path + [index], skips)
    return skips


def allowed_secret_metadata_path(path):
    return path == ["secretAudit"] or path == ["kubernetes", "secretFileClass"]


def collect_secret_material(value, path=None, findings=None):
    if path is None:
        path = []
    if findings is None:
        findings = []
    secret_key = bool(path and SECRET_KEY_PATTERN.search(str(path[-1])))
    if secret_key and not allowed_secret_metadata_path(path) and not blank(value):
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


def required_evidence_ref_path(path):
    static_paths = [
        ["strictVerifier", "evidenceRef"],
        ["deployment", "evidenceRef"],
        ["kubernetes", "evidenceRef"],
        ["grpcSmokeReport", "evidenceRef"],
        ["secretAudit", "evidenceRef"],
        ["workflowTelemetry", "evidenceRef"],
        ["requestLogObservability", "evidenceRef"],
        ["ragIndexing", "evidenceRef"],
        ["agentSandbox", "evidenceRef"],
        ["relayRealtime", "evidenceRef"],
        ["relayBatch", "evidenceRef"],
        ["marketplacePayouts", "evidenceRef"],
        ["marketplaceGovernance", "evidenceRef"],
        ["providerRuntimeConfig", "evidenceRef"],
        ["microserviceDatabases", "evidenceRef"],
    ]
    return (
        path in static_paths
        or (len(path) == 3 and path[0] == "providers" and isinstance(path[1], int) and path[2] == "evidenceRef")
        or (len(path) == 3 and path[0] == "grpc" and isinstance(path[1], int) and path[2] == "evidenceRef")
    )


def collect_required_evidence_refs(value, path=None, refs=None):
    if path is None:
        path = []
    if refs is None:
        refs = []
    if isinstance(value, dict):
        for key, child in value.items():
            child_path = path + [key]
            if key == "evidenceRef" and required_evidence_ref_path(child_path):
                refs.append((child_path, child))
            collect_required_evidence_refs(child, child_path, refs)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            collect_required_evidence_refs(child, path + [index], refs)
    return refs


def require_artifact_kind(failures, data, artifact_ids, ref_path, expected_kind):
    ref = dig_path(data, ref_path)
    if not isinstance(ref, str) or placeholder(ref):
        return
    artifact = artifact_ids.get(ref)
    if not isinstance(artifact, dict):
        return
    if artifact.get("kind") != expected_kind:
        failures.append(f"{path_label(ref_path)} must reference artifact kind {expected_kind}")


def require_artifact_proofs(failures, data, artifact_ids, ref_path, required_proofs):
    ref = dig_path(data, ref_path)
    if not isinstance(ref, str) or placeholder(ref):
        return
    artifact = artifact_ids.get(ref)
    if not isinstance(artifact, dict):
        return
    proofs = artifact.get("proofs")
    for proof in required_proofs:
        if not isinstance(proofs, dict) or proofs.get(proof) != "pass":
            failures.append(f"{path_label(ref_path)} artifact proofs.{proof} must be pass")


def artifact_body_path(artifact_dir, artifact_id):
    if not isinstance(artifact_id, str) or artifact_id.strip() == "" or placeholder(artifact_id):
        return None
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", artifact_id):
        return None
    return pathlib.Path(artifact_dir) / f"{artifact_id}.json"


def validate_artifact_body_proofs(failures, artifact_body, artifact_index, required_proofs):
    proofs = artifact_body.get("proofs") if isinstance(artifact_body, dict) else None
    for proof in required_proofs:
        if not isinstance(proofs, dict) or proofs.get(proof) != "pass":
            failures.append(f"artifacts[{artifact_index}] body proofs.{proof} must be pass")


def validate_microservice_database_body_summary(failures, artifact_body, artifact_index):
    if artifact_body.get("mode") != "microservices":
        failures.append(f"artifacts[{artifact_index}] body mode must be microservices")
    if artifact_body.get("serviceUrlClass") != "external-filled":
        failures.append(f"artifacts[{artifact_index}] body serviceUrlClass must be external-filled")
    service_count = validate_microservice_database_details(failures, artifact_body, artifact_index)
    summary = artifact_body.get("summary") if isinstance(artifact_body, dict) else None
    if not isinstance(summary, dict):
        failures.append(f"artifacts[{artifact_index}] body summary is required")
        return
    expected_services = 11
    expected_counts = {
        "servicesChecked": (expected_services, "body summary.servicesChecked must equal 11"),
        "externalUrlsChecked": (expected_services, "body summary.externalUrlsChecked must equal 11"),
        "migrationReadinessChecks": (expected_services, "body summary.migrationReadinessChecks must equal 11"),
    }
    for key, (expected, message) in expected_counts.items():
        value = summary.get(key)
        if type(value) is not int or value != expected:
            failures.append(f"artifacts[{artifact_index}] {message}")
    if isinstance(service_count, int) and summary.get("servicesChecked") != service_count:
        failures.append(f"artifacts[{artifact_index}] body summary.servicesChecked must equal service detail count")


def validate_microservice_database_details(failures, artifact_body, artifact_index):
    services = artifact_body.get("services") if isinstance(artifact_body, dict) else None
    if not isinstance(services, list) or not services:
        failures.append(f"artifacts[{artifact_index}] body services must be a non-empty array")
        return None
    required = {"relay", "chat", "workflow", "rag", "agent", "billing", "marketplace", "admin", "channel", "task", "observability"}
    seen = set()
    for service_index, item in enumerate(services):
        label = f"artifacts[{artifact_index}] body services[{service_index}]"
        if not isinstance(item, dict):
            failures.append(f"{label} must be a JSON object")
            continue
        name = item.get("name")
        if not isinstance(name, str) or name.strip() == "":
            failures.append(f"{label}.name is required")
            continue
        normalized_name = name.strip().lower()
        if placeholder(name):
            failures.append(f"{label}.name must be concrete")
        if EMBEDDED_SECRET_PATTERN.search(name):
            failures.append(f"{label}.name must not embed secret material")
        if normalized_name in seen:
            failures.append(f"artifacts[{artifact_index}] body services must not duplicate {normalized_name}")
        seen.add(normalized_name)
        if item.get("databaseUrlClass") != "external-filled":
            failures.append(f"{label}.databaseUrlClass must be external-filled")
        if item.get("migrationReadiness") != "pass":
            failures.append(f"{label}.migrationReadiness must be pass")
        validate_concrete_reference(failures, item.get("evidenceId"), f"{label}.evidenceId")
    missing = [name for name in sorted(required) if name not in seen]
    if missing:
        failures.append(f"artifacts[{artifact_index}] body services must include relay, chat, workflow, rag, agent, billing, marketplace, admin, channel, task, and observability (missing: {', '.join(missing)})")
    return len(services)


def validate_rag_indexing_body_summary(failures, artifact_body, artifact_index):
    summary = artifact_body.get("summary") if isinstance(artifact_body, dict) else None
    if not isinstance(summary, dict):
        failures.append(f"artifacts[{artifact_index}] body summary is required")
        return
    queued_jobs = summary.get("queuedJobs")
    drained_jobs = summary.get("drainedJobs")
    worker_completed_jobs = summary.get("workerCompletedJobs")
    raw_parser_replay_count = summary.get("rawParserReplayCount")
    retrieval_probe_count = summary.get("retrievalProbeCount")
    stale_vector_rows_filtered = summary.get("staleVectorRowsFiltered")
    if type(queued_jobs) is not int or queued_jobs <= 0:
        failures.append(f"artifacts[{artifact_index}] body summary.queuedJobs must be greater than zero")
    if type(drained_jobs) is not int or drained_jobs != queued_jobs:
        failures.append(f"artifacts[{artifact_index}] body summary.drainedJobs must equal summary.queuedJobs")
    if type(worker_completed_jobs) is not int or worker_completed_jobs <= 0:
        failures.append(f"artifacts[{artifact_index}] body summary.workerCompletedJobs must be greater than zero")
    if type(worker_completed_jobs) is int and type(drained_jobs) is int and worker_completed_jobs != drained_jobs:
        failures.append(f"artifacts[{artifact_index}] body summary.workerCompletedJobs must equal summary.drainedJobs")
    if type(raw_parser_replay_count) is not int or raw_parser_replay_count <= 0:
        failures.append(f"artifacts[{artifact_index}] body summary.rawParserReplayCount must be greater than zero")
    if type(retrieval_probe_count) is not int or retrieval_probe_count <= 0:
        failures.append(f"artifacts[{artifact_index}] body summary.retrievalProbeCount must be greater than zero")
    if type(stale_vector_rows_filtered) is not int or stale_vector_rows_filtered <= 0:
        failures.append(f"artifacts[{artifact_index}] body summary.staleVectorRowsFiltered must be greater than zero")


def validate_agent_sandbox_body_summary(failures, artifact_body, artifact_index):
    summary = artifact_body.get("summary") if isinstance(artifact_body, dict) else None
    if not isinstance(summary, dict):
        failures.append(f"artifacts[{artifact_index}] body summary is required")
        return
    required_positive_counts = [
        "sandboxRuns",
        "contextJoinedRuns",
        "cancellationProbes",
        "retainedArtifactBodies",
        "retainedLogRecords",
        "deniedNetworkProbes",
    ]
    for key in required_positive_counts:
        value = summary.get(key)
        if type(value) is not int or value <= 0:
            failures.append(f"artifacts[{artifact_index}] body summary.{key} must be greater than zero")
    sandbox_runs = summary.get("sandboxRuns")
    context_joined_runs = summary.get("contextJoinedRuns")
    if type(sandbox_runs) is int and type(context_joined_runs) is int and context_joined_runs != sandbox_runs:
        failures.append(f"artifacts[{artifact_index}] body summary.contextJoinedRuns must equal summary.sandboxRuns")
    if summary.get("hostFilesystemEscapes") != 0:
        failures.append(f"artifacts[{artifact_index}] body summary.hostFilesystemEscapes must equal 0")
    if summary.get("networkAccessViolations") != 0:
        failures.append(f"artifacts[{artifact_index}] body summary.networkAccessViolations must equal 0")


def validate_request_log_observability_body_coverage(failures, artifact_body, artifact_index):
    coverage = artifact_body.get("coverage") if isinstance(artifact_body, dict) else None
    if not isinstance(coverage, dict):
        failures.append(f"artifacts[{artifact_index}] body coverage is required")
        return
    checked_records = coverage.get("checkedRecords")
    usage_rows_with_request_id = coverage.get("usageRowsWithRequestId")
    usage_rows_missing_request_id = coverage.get("usageRowsMissingRequestId")
    matched_request_log_records = coverage.get("matchedRequestLogRecords")
    missing_request_log_records = coverage.get("missingRequestLogRecords")
    issues = coverage.get("issues")
    if type(checked_records) is not int or checked_records <= 0:
        failures.append(f"artifacts[{artifact_index}] body coverage.checkedRecords must be greater than zero")
    if type(usage_rows_with_request_id) is not int or usage_rows_with_request_id != checked_records:
        failures.append(f"artifacts[{artifact_index}] body coverage.usageRowsWithRequestId must equal coverage.checkedRecords")
    if type(usage_rows_missing_request_id) is not int or usage_rows_missing_request_id != 0:
        failures.append(f"artifacts[{artifact_index}] body coverage.usageRowsMissingRequestId must equal 0")
    if type(matched_request_log_records) is not int or matched_request_log_records != usage_rows_with_request_id:
        failures.append(f"artifacts[{artifact_index}] body coverage.matchedRequestLogRecords must equal coverage.usageRowsWithRequestId")
    if type(missing_request_log_records) is not int or missing_request_log_records != 0:
        failures.append(f"artifacts[{artifact_index}] body coverage.missingRequestLogRecords must equal 0")
    if not isinstance(issues, list) or issues:
        failures.append(f"artifacts[{artifact_index}] body coverage.issues must be an empty array")


def validate_iso8601_interval_value(failures, value, label):
    if not isinstance(value, str) or value.strip() == "":
        failures.append(f"{label} is required")
        return
    parts = [part.strip() for part in value.split("/")]
    if len(parts) != 2 or any(part == "" for part in parts):
        failures.append(f"{label} must be an ISO-8601 start/end interval")
        return
    starts_at = parse_iso8601_safely(parts[0])
    ends_at = parse_iso8601_safely(parts[1])
    if starts_at is None or ends_at is None:
        failures.append(f"{label} must be an ISO-8601 start/end interval")
        return
    if ends_at < starts_at:
        failures.append(f"{label} end must be at or after start")


def positive_int_field(failures, payload, key, label):
    value = payload.get(key) if isinstance(payload, dict) else None
    if type(value) is not int or value <= 0:
        failures.append(f"{label}.{key} must be greater than zero")
    return value


def zero_int_field(failures, payload, key, label):
    value = payload.get(key) if isinstance(payload, dict) else None
    if type(value) is not int or value != 0:
        failures.append(f"{label}.{key} must equal 0")
    return value


def string_field(failures, payload, key, label):
    value = payload.get(key) if isinstance(payload, dict) else None
    if not isinstance(value, str) or value.strip() == "":
        failures.append(f"{label}.{key} is required")
    return value


def concrete_string_field(failures, payload, key, label):
    value = string_field(failures, payload, key, label)
    if not isinstance(value, str) or value.strip() == "":
        return value
    if placeholder(value):
        failures.append(f"{label}.{key} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        failures.append(f"{label}.{key} must not embed secret material")
    return value


def string_array_field(failures, payload, key, label):
    value = payload.get(key) if isinstance(payload, dict) else None
    if (
        not isinstance(value, list)
        or not value
        or any(not isinstance(item, str) or item.strip() == "" for item in value)
    ):
        failures.append(f"{label}.{key} must be a non-empty array of strings")
    return value


def validate_request_log_slo_details(
    failures,
    payload,
    label,
    window_key,
    triggered_alerts_key,
):
    if not isinstance(payload, dict):
        failures.append(f"{label} is required")
        return
    validate_iso8601_interval_value(failures, payload.get(window_key), f"{label}.{window_key}")
    triggered_alerts = positive_int_field(failures, payload, triggered_alerts_key, label)

    alert_delivery = payload.get("alertDelivery")
    if not isinstance(alert_delivery, dict):
        failures.append(f"{label}.alertDelivery is required")
    else:
        positive_int_field(failures, alert_delivery, "configuredProviders", f"{label}.alertDelivery")
        delivered_alerts = positive_int_field(failures, alert_delivery, "deliveredAlerts", f"{label}.alertDelivery")
        zero_int_field(failures, alert_delivery, "failedDeliveries", f"{label}.alertDelivery")
        string_array_field(failures, alert_delivery, "channels", f"{label}.alertDelivery")
        concrete_string_field(failures, alert_delivery, "lastDeliveryId", f"{label}.alertDelivery")
        if type(delivered_alerts) is int and type(triggered_alerts) is int and delivered_alerts < triggered_alerts:
            failures.append(f"{label}.alertDelivery.deliveredAlerts must be at least {label}.{triggered_alerts_key}")

    recovery_audit = payload.get("recoveryAudit")
    if not isinstance(recovery_audit, dict):
        failures.append(f"{label}.recoveryAudit is required")
    else:
        audit_records = positive_int_field(failures, recovery_audit, "auditRecords", f"{label}.recoveryAudit")
        zero_int_field(failures, recovery_audit, "failedActions", f"{label}.recoveryAudit")
        concrete_string_field(failures, recovery_audit, "lastRecordId", f"{label}.recoveryAudit")
        if type(audit_records) is int and type(triggered_alerts) is int and audit_records < triggered_alerts:
            failures.append(f"{label}.recoveryAudit.auditRecords must be at least {label}.{triggered_alerts_key}")


def validate_request_log_observability_body_slo(failures, artifact_body, artifact_index, manifest):
    slo = artifact_body.get("slo") if isinstance(artifact_body, dict) else None
    label = f"artifacts[{artifact_index}] body slo"
    validate_request_log_slo_details(failures, slo, label, "window", "triggeredAlerts")
    manifest_slo = manifest.get("requestLogObservability") if isinstance(manifest, dict) else None
    if not isinstance(slo, dict) or not isinstance(manifest_slo, dict):
        return
    expected_pairs = [
        ("window", manifest_slo.get("latencySLOWindow")),
        ("triggeredAlerts", manifest_slo.get("latencySLOTriggeredAlerts")),
        ("alertDelivery", manifest_slo.get("alertDelivery")),
        ("recoveryAudit", manifest_slo.get("recoveryAudit")),
    ]
    for key, expected in expected_pairs:
        if key in slo and expected is not None and slo.get(key) != expected:
            failures.append(f"{label}.{key} must match requestLogObservability.{key if key not in ('window', 'triggeredAlerts') else 'latencySLO' + key[0].upper() + key[1:]}")


def numeric_value(value):
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return None
    return float(value)


def sha256_value(value):
    return isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value.strip().lower()) is not None


def validate_workflow_telemetry_body(failures, artifact_body, artifact_index, manifest):
    if artifact_body.get("result") != "pass":
        failures.append(f"artifacts[{artifact_index}] body result must be pass")
    telemetry = artifact_body.get("telemetry") if isinstance(artifact_body, dict) else None
    if not isinstance(telemetry, dict):
        failures.append(f"artifacts[{artifact_index}] body telemetry is required")
        return

    manifest_telemetry = manifest.get("workflowTelemetry") if isinstance(manifest, dict) else {}
    manifest_success_rate = numeric_value(manifest_telemetry.get("successRate") if isinstance(manifest_telemetry, dict) else None)
    body_success_rate = numeric_value(telemetry.get("successRate"))
    if body_success_rate is None or body_success_rate < 0.99 or body_success_rate > 1.0:
        failures.append(f"artifacts[{artifact_index}] body telemetry.successRate must be between 0.99 and 1.0")
    elif manifest_success_rate is not None and abs(body_success_rate - manifest_success_rate) > 0.000001:
        failures.append(f"artifacts[{artifact_index}] body telemetry.successRate must match workflowTelemetry.successRate")

    body_window = telemetry.get("window")
    manifest_window = manifest_telemetry.get("window") if isinstance(manifest_telemetry, dict) else None
    if not isinstance(body_window, str) or iso8601_interval_end(body_window) is None:
        failures.append(f"artifacts[{artifact_index}] body telemetry.window must be an ISO-8601 start/end interval")
    elif isinstance(manifest_window, str) and body_window.strip() != manifest_window.strip():
        failures.append(f"artifacts[{artifact_index}] body telemetry.window must match workflowTelemetry.window")

    total_executions = telemetry.get("totalExecutions")
    successful_executions = telemetry.get("successfulExecutions")
    failed_executions = telemetry.get("failedExecutions")
    if type(total_executions) is not int or total_executions <= 0:
        failures.append(f"artifacts[{artifact_index}] body telemetry.totalExecutions must be greater than zero")
    if type(successful_executions) is not int or successful_executions < 0:
        failures.append(f"artifacts[{artifact_index}] body telemetry.successfulExecutions must be a non-negative integer")
    if type(failed_executions) is not int or failed_executions < 0:
        failures.append(f"artifacts[{artifact_index}] body telemetry.failedExecutions must be a non-negative integer")
    if (
        type(total_executions) is int
        and type(successful_executions) is int
        and type(failed_executions) is int
        and successful_executions + failed_executions != total_executions
    ):
        failures.append(f"artifacts[{artifact_index}] body telemetry.successfulExecutions plus telemetry.failedExecutions must equal telemetry.totalExecutions")
    if (
        body_success_rate is not None
        and type(total_executions) is int
        and total_executions > 0
        and type(successful_executions) is int
        and abs((successful_executions / total_executions) - body_success_rate) > 0.0005
    ):
        failures.append(f"artifacts[{artifact_index}] body telemetry.successRate must equal successfulExecutions / totalExecutions")
    if isinstance(manifest_telemetry, dict):
        for key in ["totalExecutions", "successfulExecutions", "failedExecutions"]:
            expected = manifest_telemetry.get(key)
            if type(expected) is int and telemetry.get(key) != expected:
                failures.append(f"artifacts[{artifact_index}] body telemetry.{key} must match workflowTelemetry.{key}")


def validate_strict_verifier_body(failures, artifact_body, artifact_index, manifest):
    if artifact_body.get("result") != "pass":
        failures.append(f"artifacts[{artifact_index}] body result must be pass")
    if artifact_body.get("skippedChecks") != []:
        failures.append(f"artifacts[{artifact_index}] body skippedChecks must be an empty array")
    manifest_strict = manifest.get("strictVerifier") if isinstance(manifest, dict) else {}
    if not isinstance(manifest_strict, dict):
        manifest_strict = {}
    for key in ["command", "startedAt", "completedAt", "targetEvidenceSha256", "artifactBundleSha256"]:
        body_value = artifact_body.get(key)
        manifest_value = manifest_strict.get(key)
        if body_value != manifest_value:
            failures.append(f"artifacts[{artifact_index}] body {key} must match strictVerifier.{key}")
    for key in ["targetEvidenceSha256", "artifactBundleSha256"]:
        if not sha256_value(artifact_body.get(key)):
            failures.append(f"artifacts[{artifact_index}] body {key} must be a 64-character SHA-256 hex digest")
    command = artifact_body.get("command")
    if isinstance(command, str):
        validate_strict_verifier_command(failures, command)
    for key in ["startedAt", "completedAt"]:
        value = artifact_body.get(key)
        if not isinstance(value, str) or parse_iso8601_safely(value) is None:
            failures.append(f"artifacts[{artifact_index}] body {key} must be ISO-8601")
    started_at = parse_iso8601_safely(artifact_body.get("startedAt"))
    completed_at = parse_iso8601_safely(artifact_body.get("completedAt"))
    if started_at and completed_at and completed_at < started_at:
        failures.append(f"artifacts[{artifact_index}] body completedAt must be at or after startedAt")


def validate_deployment_body(failures, artifact_body, artifact_index, manifest):
    if artifact_body.get("result") != "pass":
        failures.append(f"artifacts[{artifact_index}] body result must be pass")
    validate_target_environment(failures, artifact_body, artifact_index, manifest)
    validate_reference_map(
        failures,
        artifact_body,
        artifact_index,
        ["deployValidation", "backupRestore", "migrationReplay"],
    )
    deployment = manifest.get("deployment") if isinstance(manifest, dict) else {}
    if not isinstance(deployment, dict):
        deployment = {}
    proofs = artifact_body.get("proofs") if isinstance(artifact_body, dict) else None
    for key in ["deployValidation", "backupRestore", "migrationReplay"]:
        if deployment.get(key) == "pass" and isinstance(proofs, dict) and proofs.get(key) != deployment.get(key):
            failures.append(f"artifacts[{artifact_index}] body proofs.{key} must match deployment.{key}")


def validate_kubernetes_body(failures, artifact_body, artifact_index, manifest):
    if artifact_body.get("result") != "pass":
        failures.append(f"artifacts[{artifact_index}] body result must be pass")
    validate_target_environment(failures, artifact_body, artifact_index, manifest)
    validate_concrete_reference(failures, artifact_body.get("clusterRef"), f"artifacts[{artifact_index}] body clusterRef")
    validate_concrete_reference(failures, artifact_body.get("namespace"), f"artifacts[{artifact_index}] body namespace")
    validate_reference_map(
        failures,
        artifact_body,
        artifact_index,
        ["validation", "rollout", "failover"],
    )
    kubernetes = manifest.get("kubernetes") if isinstance(manifest, dict) else {}
    if not isinstance(kubernetes, dict):
        kubernetes = {}
    proofs = artifact_body.get("proofs") if isinstance(artifact_body, dict) else None
    for key in ["validation", "rollout", "failover"]:
        if kubernetes.get(key) == "pass" and isinstance(proofs, dict) and proofs.get(key) != kubernetes.get(key):
            failures.append(f"artifacts[{artifact_index}] body proofs.{key} must match kubernetes.{key}")
    if artifact_body.get("secretFileClass") != "external-filled":
        failures.append(f"artifacts[{artifact_index}] body secretFileClass must be external-filled")
    elif kubernetes.get("secretFileClass") == "external-filled" and artifact_body.get("secretFileClass") != kubernetes.get("secretFileClass"):
        failures.append(f"artifacts[{artifact_index}] body secretFileClass must match kubernetes.secretFileClass")


def validate_target_environment(failures, artifact_body, artifact_index, manifest):
    target_environment = artifact_body.get("targetEnvironment")
    if target_environment != "production":
        failures.append(f"artifacts[{artifact_index}] body targetEnvironment must be production")
    manifest_environment = manifest.get("environment") if isinstance(manifest, dict) else {}
    manifest_class = manifest_environment.get("class") if isinstance(manifest_environment, dict) else None
    if isinstance(manifest_class, str) and manifest_class.strip().lower() == "production" and target_environment != "production":
        failures.append(f"artifacts[{artifact_index}] body targetEnvironment must match environment.class")


def validate_concrete_reference(failures, value, label):
    if not isinstance(value, str) or value.strip() == "":
        failures.append(f"{label} is required")
        return
    if placeholder(value):
        failures.append(f"{label} must be concrete")
    if EMBEDDED_SECRET_PATTERN.search(value):
        failures.append(f"{label} must not embed secret material")


def validate_reference_map(failures, artifact_body, artifact_index, required_fields):
    references = artifact_body.get("references") if isinstance(artifact_body, dict) else None
    if not isinstance(references, dict):
        failures.append(f"artifacts[{artifact_index}] body references is required")
        return
    for field in required_fields:
        validate_concrete_reference(
            failures,
            references.get(field),
            f"artifacts[{artifact_index}] body references.{field}",
        )


def body_summary(failures, artifact_body, artifact_index):
    summary = artifact_body.get("summary") if isinstance(artifact_body, dict) else None
    if not isinstance(summary, dict):
        failures.append(f"artifacts[{artifact_index}] body summary is required")
        return None
    return summary


def summary_int(failures, summary, artifact_index, key, positive=False):
    value = summary.get(key) if isinstance(summary, dict) else None
    if type(value) is not int or value < 0:
        failures.append(f"artifacts[{artifact_index}] body summary.{key} must be a non-negative integer")
        return None
    if positive and value <= 0:
        failures.append(f"artifacts[{artifact_index}] body summary.{key} must be greater than zero")
    return value


def validate_relay_realtime_body_summary(failures, artifact_body, artifact_index):
    mode = artifact_body.get("mode")
    if mode not in ("disabled_until_commercial_lifecycle", "commercial_lifecycle_enabled"):
        failures.append(f"artifacts[{artifact_index}] body mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled")
        return
    if mode == "disabled_until_commercial_lifecycle":
        validate_artifact_body_proofs(
            failures,
            artifact_body,
            artifact_index,
            ["productionPolicyDisabled", "authOriginPrebillAbortUsageBlockers"],
        )
    else:
        validate_artifact_body_proofs(
            failures,
            artifact_body,
            artifact_index,
            [
                "productionPolicyEnabled",
                "authPolicy",
                "originPolicy",
                "prebillSettlement",
                "abortSettlement",
                "usageLedger",
            ],
        )
    summary = body_summary(failures, artifact_body, artifact_index)
    if summary is None:
        return
    if mode == "disabled_until_commercial_lifecycle":
        summary_int(failures, summary, artifact_index, "productionPolicyChecks", positive=True)
        blocker_checks = summary_int(failures, summary, artifact_index, "authOriginPrebillAbortUsageBlockerChecks", positive=True)
        if isinstance(blocker_checks, int) and blocker_checks < 5:
            failures.append(f"artifacts[{artifact_index}] body summary.authOriginPrebillAbortUsageBlockerChecks must cover auth, origin, prebill, abort, and usage blockers")
        return
    total_requests = summary_int(failures, summary, artifact_index, "totalRequests", positive=True)
    authenticated_requests = summary_int(failures, summary, artifact_index, "authenticatedRequests", positive=True)
    request_linked_usage_records = summary_int(failures, summary, artifact_index, "requestLinkedUsageRecords", positive=True)
    price_snapshot_records = summary_int(failures, summary, artifact_index, "priceSnapshotRecords", positive=True)
    summary_int(failures, summary, artifact_index, "abortSettlementRecords", positive=True)
    terminal_usage_records = summary_int(failures, summary, artifact_index, "terminalUsageRecords", positive=True)
    summary_int(failures, summary, artifact_index, "originPolicyChecks", positive=True)
    if isinstance(authenticated_requests, int) and isinstance(total_requests, int) and authenticated_requests != total_requests:
        failures.append(f"artifacts[{artifact_index}] body summary.authenticatedRequests must equal summary.totalRequests")
    if isinstance(request_linked_usage_records, int) and isinstance(total_requests, int) and request_linked_usage_records != total_requests:
        failures.append(f"artifacts[{artifact_index}] body summary.requestLinkedUsageRecords must equal summary.totalRequests")
    if isinstance(price_snapshot_records, int) and isinstance(total_requests, int) and price_snapshot_records != total_requests:
        failures.append(f"artifacts[{artifact_index}] body summary.priceSnapshotRecords must equal summary.totalRequests")
    if isinstance(terminal_usage_records, int) and isinstance(total_requests, int) and terminal_usage_records != total_requests:
        failures.append(f"artifacts[{artifact_index}] body summary.terminalUsageRecords must equal summary.totalRequests")


def validate_relay_batch_body_summary(failures, artifact_body, artifact_index):
    mode = artifact_body.get("mode")
    if mode not in ("disabled_until_commercial_lifecycle", "commercial_lifecycle_enabled"):
        failures.append(f"artifacts[{artifact_index}] body mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled")
        return
    if mode == "disabled_until_commercial_lifecycle":
        validate_artifact_body_proofs(
            failures,
            artifact_body,
            artifact_index,
            ["productionPolicyDisabled", "prebillPollingSettlementRefundAuditUsageBlockers"],
        )
    else:
        validate_artifact_body_proofs(
            failures,
            artifact_body,
            artifact_index,
            [
                "productionPolicyEnabled",
                "prebillReservation",
                "pollingCompletion",
                "settlement",
                "refund",
                "usageAudit",
            ],
        )
    summary = body_summary(failures, artifact_body, artifact_index)
    if summary is None:
        return
    if mode == "disabled_until_commercial_lifecycle":
        summary_int(failures, summary, artifact_index, "productionPolicyChecks", positive=True)
        blocker_checks = summary_int(failures, summary, artifact_index, "prebillPollingSettlementRefundAuditUsageBlockerChecks", positive=True)
        if isinstance(blocker_checks, int) and blocker_checks < 6:
            failures.append(f"artifacts[{artifact_index}] body summary.prebillPollingSettlementRefundAuditUsageBlockerChecks must cover prebill, polling, settlement, refund, audit, and usage blockers")
        return
    summary_int(failures, summary, artifact_index, "prebillReservations", positive=True)
    summary_int(failures, summary, artifact_index, "pollingCompletions", positive=True)
    settlement_records = summary_int(failures, summary, artifact_index, "settlementRecords", positive=True)
    refund_records = summary_int(failures, summary, artifact_index, "refundRecords", positive=True)
    usage_audit_records = summary_int(failures, summary, artifact_index, "usageAuditRecords", positive=True)
    request_log_audit_records = summary_int(failures, summary, artifact_index, "requestLogAuditRecords", positive=True)
    terminal_failure_records = summary_int(failures, summary, artifact_index, "terminalFailureRecords", positive=True)
    if (
        isinstance(usage_audit_records, int)
        and isinstance(settlement_records, int)
        and isinstance(refund_records, int)
        and usage_audit_records < settlement_records + refund_records
    ):
        failures.append(f"artifacts[{artifact_index}] body summary.usageAuditRecords must cover summary.settlementRecords plus summary.refundRecords")
    if (
        isinstance(request_log_audit_records, int)
        and isinstance(settlement_records, int)
        and isinstance(refund_records, int)
        and request_log_audit_records < settlement_records + refund_records
    ):
        failures.append(f"artifacts[{artifact_index}] body summary.requestLogAuditRecords must cover summary.settlementRecords plus summary.refundRecords")
    if isinstance(terminal_failure_records, int) and isinstance(refund_records, int) and terminal_failure_records < refund_records:
        failures.append(f"artifacts[{artifact_index}] body summary.terminalFailureRecords must cover summary.refundRecords")


def validate_marketplace_payout_body_summary(failures, artifact_body, artifact_index):
    summary = body_summary(failures, artifact_body, artifact_index)
    if summary is None:
        return
    outbound_dispatches = summary_int(failures, summary, artifact_index, "outboundDispatches", positive=True)
    webhook_events = summary_int(failures, summary, artifact_index, "webhookEvents", positive=True)
    settlement_ledger_entries = summary_int(failures, summary, artifact_index, "settlementLedgerEntries", positive=True)
    reconciled_entries = summary_int(failures, summary, artifact_index, "reconciledEntries")
    refund_chargeback_cases = summary_int(failures, summary, artifact_index, "refundChargebackCases", positive=True)
    refund_chargeback_cases_handled = summary_int(failures, summary, artifact_index, "refundChargebackCasesHandled")
    if isinstance(webhook_events, int) and isinstance(outbound_dispatches, int) and webhook_events < outbound_dispatches:
        failures.append(f"artifacts[{artifact_index}] body summary.webhookEvents must cover summary.outboundDispatches")
    if isinstance(reconciled_entries, int) and isinstance(settlement_ledger_entries, int) and reconciled_entries != settlement_ledger_entries:
        failures.append(f"artifacts[{artifact_index}] body summary.reconciledEntries must equal summary.settlementLedgerEntries")
    if isinstance(refund_chargeback_cases_handled, int) and isinstance(refund_chargeback_cases, int) and refund_chargeback_cases_handled != refund_chargeback_cases:
        failures.append(f"artifacts[{artifact_index}] body summary.refundChargebackCasesHandled must equal summary.refundChargebackCases")


def validate_marketplace_governance_body_summary(failures, artifact_body, artifact_index):
    summary = body_summary(failures, artifact_body, artifact_index)
    if summary is None:
        return
    review_queue_items = summary_int(failures, summary, artifact_index, "reviewQueueItems", positive=True)
    appeal_queue_items = summary_int(failures, summary, artifact_index, "appealQueueItems", positive=True)
    appeal_decisions = summary_int(failures, summary, artifact_index, "appealDecisions")
    review_assignments = summary_int(failures, summary, artifact_index, "reviewAssignments", positive=True)
    sla_checks = summary_int(failures, summary, artifact_index, "slaChecks", positive=True)
    summary_int(failures, summary, artifact_index, "slaBreachesHandled")
    abuse_reports = summary_int(failures, summary, artifact_index, "abuseReports", positive=True)
    abuse_reports_resolved = summary_int(failures, summary, artifact_index, "abuseReportsResolved")
    if isinstance(review_assignments, int) and isinstance(review_queue_items, int) and review_assignments < review_queue_items:
        failures.append(f"artifacts[{artifact_index}] body summary.reviewAssignments must cover summary.reviewQueueItems")
    if isinstance(appeal_decisions, int) and isinstance(appeal_queue_items, int) and appeal_decisions != appeal_queue_items:
        failures.append(f"artifacts[{artifact_index}] body summary.appealDecisions must equal summary.appealQueueItems")
    if isinstance(sla_checks, int) and isinstance(review_assignments, int) and sla_checks < review_assignments:
        failures.append(f"artifacts[{artifact_index}] body summary.slaChecks must cover summary.reviewAssignments")
    if isinstance(abuse_reports_resolved, int) and isinstance(abuse_reports, int) and abuse_reports_resolved != abuse_reports:
        failures.append(f"artifacts[{artifact_index}] body summary.abuseReportsResolved must equal summary.abuseReports")


def validate_provider_runtime_config_body_summary(failures, artifact_body, artifact_index):
    provider_count = validate_provider_runtime_config_details(failures, artifact_body, artifact_index)
    summary = body_summary(failures, artifact_body, artifact_index)
    if summary is None:
        return
    providers_configured = summary_int(failures, summary, artifact_index, "providersConfigured", positive=True)
    provider_env_vars_checked = summary_int(failures, summary, artifact_index, "providerEnvVarsChecked", positive=True)
    checkout_base_urls_checked = summary_int(failures, summary, artifact_index, "checkoutBaseUrlsChecked", positive=True)
    webhook_routes_checked = summary_int(failures, summary, artifact_index, "webhookRoutesChecked", positive=True)
    webhook_verification_checks = summary_int(failures, summary, artifact_index, "webhookVerificationChecks", positive=True)
    if isinstance(providers_configured, int) and providers_configured < 3:
        failures.append(f"artifacts[{artifact_index}] body summary.providersConfigured must include Stripe, Alipay, and WeChat Pay")
    if isinstance(provider_env_vars_checked, int) and isinstance(providers_configured, int) and provider_env_vars_checked < providers_configured:
        failures.append(f"artifacts[{artifact_index}] body summary.providerEnvVarsChecked must cover summary.providersConfigured")
    if isinstance(checkout_base_urls_checked, int) and isinstance(providers_configured, int) and checkout_base_urls_checked < providers_configured:
        failures.append(f"artifacts[{artifact_index}] body summary.checkoutBaseUrlsChecked must cover summary.providersConfigured")
    if isinstance(webhook_routes_checked, int) and isinstance(providers_configured, int) and webhook_routes_checked < providers_configured:
        failures.append(f"artifacts[{artifact_index}] body summary.webhookRoutesChecked must cover summary.providersConfigured")
    if isinstance(webhook_verification_checks, int) and isinstance(providers_configured, int) and webhook_verification_checks < providers_configured:
        failures.append(f"artifacts[{artifact_index}] body summary.webhookVerificationChecks must cover summary.providersConfigured")
    if isinstance(providers_configured, int) and isinstance(provider_count, int) and providers_configured != provider_count:
        failures.append(f"artifacts[{artifact_index}] body summary.providersConfigured must equal provider detail count")


def validate_provider_runtime_config_details(failures, artifact_body, artifact_index):
    providers = artifact_body.get("providers") if isinstance(artifact_body, dict) else None
    if not isinstance(providers, list) or not providers:
        failures.append(f"artifacts[{artifact_index}] body providers must be a non-empty array")
        return None
    required = {"stripe", "alipay", "wechatpay"}
    seen = set()
    for provider_index, item in enumerate(providers):
        label = f"artifacts[{artifact_index}] body providers[{provider_index}]"
        if not isinstance(item, dict):
            failures.append(f"{label} must be a JSON object")
            continue
        name = item.get("name")
        if not isinstance(name, str) or name.strip() == "":
            failures.append(f"{label}.name is required")
            continue
        normalized_name = name.strip().lower()
        if placeholder(name):
            failures.append(f"{label}.name must be concrete")
        if EMBEDDED_SECRET_PATTERN.search(name):
            failures.append(f"{label}.name must not embed secret material")
        if normalized_name in seen:
            failures.append(f"artifacts[{artifact_index}] body providers must not duplicate {normalized_name}")
        seen.add(normalized_name)
        if item.get("providerEnvironment") != "live":
            failures.append(f"{label}.providerEnvironment must be live")
        if item.get("providerEnv") != "pass":
            failures.append(f"{label}.providerEnv must be pass")
        if item.get("checkoutBaseUrlClass") != "external-filled":
            failures.append(f"{label}.checkoutBaseUrlClass must be external-filled")
        if item.get("webhookRoute") != "pass":
            failures.append(f"{label}.webhookRoute must be pass")
        if item.get("webhookVerification") != "pass":
            failures.append(f"{label}.webhookVerification must be pass")
        validate_concrete_reference(failures, item.get("evidenceId"), f"{label}.evidenceId")
    missing = [name for name in sorted(required) if name not in seen]
    if missing:
        failures.append(f"artifacts[{artifact_index}] body providers must include stripe, alipay, and wechatpay (missing: {', '.join(missing)})")
    return len(providers)


def validate_provider_live_rail_body(failures, artifact_body, artifact_index):
    if artifact_body.get("mode") != "live":
        failures.append(f"artifacts[{artifact_index}] body mode must be live")
    if artifact_body.get("providerEnvironment") != "live":
        failures.append(f"artifacts[{artifact_index}] body providerEnvironment must be live")
    validate_artifact_body_proofs(
        failures,
        artifact_body,
        artifact_index,
        ["checkout", "refund", "payout", "reconciliation"],
    )
    validate_provider_live_rail_references(failures, artifact_body, artifact_index)
    summary = body_summary(failures, artifact_body, artifact_index)
    if summary is None:
        return
    for key in ["checkoutAttempts", "refundAttempts", "payoutAttempts", "reconciliationChecks"]:
        summary_int(failures, summary, artifact_index, key, positive=True)


def validate_provider_live_rail_references(failures, artifact_body, artifact_index):
    references = artifact_body.get("references")
    if not isinstance(references, dict):
        failures.append(f"artifacts[{artifact_index}] body references is required")
        return
    for key in ["checkout", "refund", "payout", "reconciliation"]:
        value = references.get(key)
        if not isinstance(value, str) or value.strip() == "":
            failures.append(f"artifacts[{artifact_index}] body references.{key} is required")
            continue
        if placeholder(value):
            failures.append(f"artifacts[{artifact_index}] body references.{key} must be concrete")
        if EMBEDDED_SECRET_PATTERN.search(value):
            failures.append(f"artifacts[{artifact_index}] body references.{key} must not embed secret material")


def validate_secret_audit_body(failures, artifact_body, artifact_index):
    if artifact_body.get("result") != "pass":
        failures.append(f"artifacts[{artifact_index}] body result must be pass")
    scope = artifact_body.get("scope")
    if not isinstance(scope, list) or not scope or any(not isinstance(item, str) or item.strip() == "" for item in scope):
        failures.append(f"artifacts[{artifact_index}] body scope must be a non-empty array of strings")
    else:
        required_scopes = ["kubernetes", "providers", "runtime"]
        normalized_scope = [item.strip().lower() for item in scope]
        missing_scopes = [item for item in required_scopes if item not in normalized_scope]
        if missing_scopes:
            failures.append(f"artifacts[{artifact_index}] body scope must include kubernetes, providers, and runtime (missing: {', '.join(missing_scopes)})")
    findings = artifact_body.get("findings")
    if not isinstance(findings, list):
        failures.append(f"artifacts[{artifact_index}] body findings must be an array")
    elif findings:
        failures.append(f"artifacts[{artifact_index}] body findings must be an empty array")
    failures.extend(collect_secret_material(artifact_body, ["artifacts", artifact_index, "body"]))


def validate_grpc_smoke_report_body(failures, artifact_body, artifact_index, manifest):
    if artifact_body.get("result") != "pass":
        failures.append(f"artifacts[{artifact_index}] body result must be pass")

    manifest_smoke_report = manifest.get("grpcSmokeReport") if isinstance(manifest, dict) else {}
    if not isinstance(manifest_smoke_report, dict):
        manifest_smoke_report = {}
    manifest_grpc_results = manifest.get("grpc") if isinstance(manifest, dict) else []
    manifest_grpc_by_service = {
        item.get("service"): item
        for item in manifest_grpc_results
        if isinstance(item, dict) and isinstance(item.get("service"), str)
    } if isinstance(manifest_grpc_results, list) else {}
    manifest_smoke_results = manifest_smoke_report.get("results")
    manifest_smoke_by_service = {
        item.get("service"): item
        for item in manifest_smoke_results
        if isinstance(item, dict) and isinstance(item.get("service"), str)
    } if isinstance(manifest_smoke_results, list) else {}

    smoke_recorded_at = artifact_body.get("smokeReportRecordedAt")
    if not isinstance(smoke_recorded_at, str) or parse_iso8601_safely(smoke_recorded_at) is None:
        failures.append(f"artifacts[{artifact_index}] body smokeReportRecordedAt must be ISO-8601")
    elif isinstance(manifest_smoke_report.get("recordedAt"), str) and smoke_recorded_at.strip() != manifest_smoke_report["recordedAt"].strip():
        failures.append(f"artifacts[{artifact_index}] body smokeReportRecordedAt must match grpcSmokeReport.recordedAt")

    timeout = artifact_body.get("timeout")
    if not isinstance(timeout, str) or not positive_go_duration_string(timeout):
        failures.append(f"artifacts[{artifact_index}] body timeout must be a positive Go duration string")
    elif isinstance(manifest_smoke_report.get("timeout"), str) and timeout.strip() != manifest_smoke_report["timeout"].strip():
        failures.append(f"artifacts[{artifact_index}] body timeout must match grpcSmokeReport.timeout")

    results = artifact_body.get("results")
    if not isinstance(results, list):
        failures.append(f"artifacts[{artifact_index}] body results must be an array")
        return

    results_by_service = {}
    for index, result in enumerate(results):
        if not isinstance(result, dict):
            failures.append(f"artifacts[{artifact_index}] body results[{index}] must be an object")
            continue
        service = str(result.get("service") or "").strip()
        if service == "":
            failures.append(f"artifacts[{artifact_index}] body results[{index}].service is required")
        elif service not in REQUIRED_GRPC_SERVICES:
            failures.append(f"artifacts[{artifact_index}] body results[{index}].service must be agent, workflow, or task")
        elif service in results_by_service:
            failures.append(f"artifacts[{artifact_index}] body results must not duplicate {service} service results")
        else:
            results_by_service[service] = result

        address = result.get("address")
        if blank(address):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address is required")
        parsed_address = parse_plain_grpc_address(address)
        if not blank(address) and parsed_address is None:
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address for {service} must be a plain host:port endpoint")
        if placeholder(address):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address for {service} must reference a concrete target service endpoint, not a placeholder")
        expected_port = EXPECTED_GRPC_PORTS.get(service)
        if parsed_address and expected_port and parsed_address["port"] != expected_port:
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address for {service} must target port {expected_port}")
        if parsed_address and non_target_host(parsed_address["host"]):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address for {service} must target a non-local service endpoint")
        manifest_entry = manifest_grpc_by_service.get(service)
        if manifest_entry and str(address) != str(manifest_entry.get("address")):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address must match grpc {service} address")
        manifest_smoke_entry = manifest_smoke_by_service.get(service)
        if manifest_smoke_entry and str(address) != str(manifest_smoke_entry.get("address")):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].address must match grpcSmokeReport {service} address")

        if result.get("generatedClient") != "pass":
            failures.append(f"artifacts[{artifact_index}] body results[{index}].generatedClient must be pass")
        if manifest_entry and result.get("generatedClient") != manifest_entry.get("generatedClient"):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].generatedClient must match grpc {service} generatedClient")
        if manifest_smoke_entry and result.get("generatedClient") != manifest_smoke_entry.get("generatedClient"):
            failures.append(f"artifacts[{artifact_index}] body results[{index}].generatedClient must match grpcSmokeReport {service} generatedClient")

        status = result.get("status")
        if not isinstance(status, str) or status.strip() == "":
            failures.append(f"artifacts[{artifact_index}] body results[{index}].status is required")
        else:
            expected_status = EXPECTED_GRPC_SMOKE_STATUSES.get(service)
            if expected_status and status != expected_status:
                failures.append(f"artifacts[{artifact_index}] body results[{index}].status for {service} must be {expected_status}")
            if manifest_smoke_entry and status != manifest_smoke_entry.get("status"):
                failures.append(f"artifacts[{artifact_index}] body results[{index}].status must match grpcSmokeReport {service} status")

    missing_services = [service for service in REQUIRED_GRPC_SERVICES if service not in results_by_service]
    if missing_services:
        failures.append(f"artifacts[{artifact_index}] body results must include agent, workflow, and task smoke results (missing: {', '.join(missing_services)})")


def validate_target_source_window_query(failures, artifact_index, source_field, parsed_url):
    query = parse_qs(parsed_url.query, keep_blank_values=True)
    from_values = query.get("from", [])
    to_values = query.get("to", [])
    if len(from_values) != 1 or len(to_values) != 1:
        failures.append(f"artifacts[{artifact_index}] body {source_field}.url must include from and to release window query parameters")
        return
    from_dt = parse_iso8601_safely(from_values[0])
    to_dt = parse_iso8601_safely(to_values[0])
    if from_dt is None or to_dt is None:
        failures.append(f"artifacts[{artifact_index}] body {source_field}.url from and to query parameters must be ISO-8601")
        return
    if to_dt < from_dt:
        failures.append(f"artifacts[{artifact_index}] body {source_field}.url to query parameter must be at or after from")


def validate_artifact_body_collection_source(
    failures,
    artifact_body,
    artifact_index,
    allow_file_collection_source,
    allow_local_collection_source,
    target_base_hostname=None,
    source_field="collectionSource",
):
    source = artifact_body.get(source_field) if isinstance(artifact_body, dict) else None
    if not isinstance(source, dict):
        failures.append(f"artifacts[{artifact_index}] body {source_field} is required")
        return
    source_type = source.get("type")
    if source_type not in ("file", "target-url", "target-api"):
        failures.append(f"artifacts[{artifact_index}] body {source_field}.type must be file, target-url, or target-api")
    elif source_type == "file" and not allow_file_collection_source:
        failures.append(f"artifacts[{artifact_index}] body {source_field}.type must be target-url or target-api for final target evidence")
    collected_at = source.get("collectedAt")
    collected_at_dt = parse_iso8601_safely(collected_at)
    if not isinstance(collected_at, str) or collected_at_dt is None:
        failures.append(f"artifacts[{artifact_index}] body {source_field}.collectedAt must be ISO-8601")
    recorded_at_dt = parse_iso8601_safely(artifact_body.get("recordedAt"))
    if collected_at_dt is not None and recorded_at_dt is not None and collected_at_dt < recorded_at_dt:
        failures.append(f"artifacts[{artifact_index}] body {source_field}.collectedAt must be at or after body recordedAt")
    if source_field == "collectionSource" and artifact_body.get("kind") == "strict-verifier-log" and collected_at_dt is not None:
        started_at = parse_iso8601_safely(artifact_body.get("startedAt"))
        completed_at = parse_iso8601_safely(artifact_body.get("completedAt"))
        if started_at is not None and collected_at_dt < started_at:
            failures.append(f"artifacts[{artifact_index}] body {source_field}.collectedAt must fall within strict verifier run window")
        if completed_at is not None and collected_at_dt > completed_at:
            failures.append(f"artifacts[{artifact_index}] body {source_field}.collectedAt must fall within strict verifier run window")
    if source_type in ("target-url", "target-api"):
        url = source.get("url")
        if not isinstance(url, str) or url.strip() == "":
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url is required")
            return
        parsed = urlsplit(url.strip())
        local_fixture_url = bool(
            allow_local_collection_source
            and parsed.scheme in ("http", "https")
            and not blank(parsed.hostname)
            and local_target_host(parsed.hostname)
        )
        if parsed.scheme not in ("http", "https") or blank(parsed.hostname):
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url must be an HTTP(S) URL")
        else:
            if parsed.scheme != "https" and not local_fixture_url:
                failures.append(f"artifacts[{artifact_index}] body {source_field}.url must use HTTPS for final target evidence")
            if non_target_host(parsed.hostname) and not allow_local_collection_source:
                failures.append(f"artifacts[{artifact_index}] body {source_field}.url must target a non-local evidence endpoint")
            if (
                target_base_hostname
                and not local_fixture_url
                and isinstance(parsed.hostname, str)
                and parsed.hostname.lower() != target_base_hostname.lower()
            ):
                failures.append(f"artifacts[{artifact_index}] body {source_field}.url host must match environment.baseUrl host")
        if placeholder(url):
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url must be concrete")
        if secret_like_uri(url):
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url must not embed secret-like query or fragment parameters")
        if userinfo_uri(url):
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url must not embed credentials in URI userinfo")
        path_slug = unquote_plus(parsed.path or "").lower()
        artifact_id = str(artifact_body.get("artifactId") or "").lower()
        kind_slug = str(artifact_body.get("kind") or "").lower()
        expected_path = EXPECTED_TARGET_SOURCE_PATHS.get((kind_slug, source_field))
        if expected_path and parsed.path.rstrip("/") != expected_path:
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url path must be {expected_path}")
        if (kind_slug, source_field) in WINDOWED_TARGET_SOURCE_KEYS:
            validate_target_source_window_query(failures, artifact_index, source_field, parsed)
        kind_core = re.sub(r"-(?:proof|log|validation|report)$", "", kind_slug)
        provider_slug = str(artifact_body.get("provider") or "").lower()
        proof_family_aliases = {
            "request-log-observability": ["usage-request-logs", "request-log"],
        }
        allowed_tokens = [token for token in (artifact_id, kind_slug, kind_core, provider_slug) if token]
        allowed_tokens.extend(proof_family_aliases.get(kind_slug, []))
        if kind_slug == "request-log-observability" and source_field == "sloProofSource":
            allowed_tokens.extend(["latency-slo-proof", "observability"])
        if allowed_tokens and not any(token in path_slug for token in allowed_tokens):
            failures.append(f"artifacts[{artifact_index}] body {source_field}.url must identify the same artifact or proof family")


def validate_artifact_bodies(
    failures,
    data,
    artifact_ids,
    artifact_indexes,
    artifact_dir,
    allow_file_collection_source=False,
    allow_local_collection_source=False,
    target_base_hostname=None,
):
    if not artifact_dir:
        return
    root = pathlib.Path(artifact_dir)
    if not root.is_dir():
        failures.append("artifact body directory must exist")
        return
    required_body_proofs_by_kind = {
        "strict-verifier-log": [],
        "deployment-log": ["deployValidation", "backupRestore", "migrationReplay"],
        "kubernetes-validation": ["validation", "rollout", "failover"],
        "request-log-observability": [
            "clickHouseDeployment",
            "clickHouseMigration",
            "requestLogsTable",
            "ingestQuerySmoke",
            "requestUsageJoin",
            "latencySLOTrigger",
            "latencySLOAlertDelivery",
            "latencySLORecoveryAction",
        ],
        "rag-indexing-proof": [
            "durableQueueMigration",
            "workerDeployment",
            "enqueueDrainProbe",
            "rawParserReplay",
            "retrievalProbe",
            "staleVectorFilter",
        ],
        "agent-sandbox-proof": [
            "dockerRuntime",
            "containerSecurityPolicy",
            "networkIsolation",
            "resourceLimits",
            "executionContextJoin",
            "cancellation",
            "artifactLogRetention",
            "auditTrace",
        ],
        "relay-realtime-proof": [],
        "relay-batch-proof": [],
        "marketplace-payout-proof": ["outboundDispatch", "inboundWebhookLifecycle", "settlementLedger", "reconciliation", "refundChargebackHandling"],
        "marketplace-governance-proof": ["reviewQueue", "appealQueue", "appealDecisionLifecycle", "reviewAssignment", "reviewSLAEnforcement", "abuseReportLifecycle"],
        "provider-runtime-config": ["stripe", "alipay", "wechatpay", "providerEnv", "checkoutBaseUrls", "webhookRoutes", "webhookVerification"],
        "provider-live-rail": [],
        "grpc-smoke-report": [],
        "secret-audit": [],
        "microservice-database-proof": ["relay", "chat", "workflow", "rag", "agent", "billing", "marketplace", "admin", "channel", "task", "observability", "migrationReadiness"],
    }
    collection_source_required_kinds = set(required_body_proofs_by_kind)
    collection_source_required_kinds.add("workflow-telemetry")
    for artifact_id, artifact in artifact_ids.items():
        index = artifact_indexes[artifact_id]
        path = artifact_body_path(root, artifact_id)
        if path is None:
            failures.append(f"artifacts[{index}] body path must use a safe artifact id")
            continue
        if not path.is_file():
            failures.append(f"artifacts[{index}] body file is required")
            continue
        body_bytes = path.read_bytes()
        expected_sha256 = artifact.get("sha256")
        actual_sha256 = hashlib.sha256(body_bytes).hexdigest()
        if isinstance(expected_sha256, str) and expected_sha256.strip() and actual_sha256 != expected_sha256.lower():
            failures.append(f"artifacts[{index}] body sha256 must match manifest sha256")
        try:
            body = json.loads(body_bytes.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError):
            failures.append(f"artifacts[{index}] body must be valid JSON")
            continue
        if not isinstance(body, dict):
            failures.append(f"artifacts[{index}] body must be a JSON object")
            continue
        expected_pairs = {
            "artifactId": artifact_id,
            "kind": artifact.get("kind"),
            "commit": artifact.get("commit"),
            "runId": artifact.get("runId"),
            "recordedAt": artifact.get("recordedAt"),
        }
        if "provider" in artifact:
            expected_pairs["provider"] = artifact.get("provider")
        for key, expected in expected_pairs.items():
            if body.get(key) != expected:
                failures.append(f"artifacts[{index}] body {key} must match manifest")
        required_body_proofs = required_body_proofs_by_kind.get(artifact.get("kind"), [])
        if artifact.get("kind") in collection_source_required_kinds:
            validate_artifact_body_collection_source(
                failures,
                body,
                index,
                allow_file_collection_source,
                allow_local_collection_source,
                target_base_hostname,
            )
        validate_artifact_body_proofs(failures, body, index, required_body_proofs)
        if artifact.get("kind") == "strict-verifier-log":
            validate_strict_verifier_body(failures, body, index, data)
        if artifact.get("kind") == "deployment-log":
            validate_deployment_body(failures, body, index, data)
        if artifact.get("kind") == "kubernetes-validation":
            validate_kubernetes_body(failures, body, index, data)
        if artifact.get("kind") == "workflow-telemetry":
            validate_workflow_telemetry_body(failures, body, index, data)
        if artifact.get("kind") == "request-log-observability":
            validate_request_log_observability_body_coverage(failures, body, index)
            validate_request_log_observability_body_slo(failures, body, index, data)
            validate_artifact_body_collection_source(
                failures,
                body,
                index,
                allow_file_collection_source,
                allow_local_collection_source,
                target_base_hostname,
                "platformProofSource",
            )
            validate_artifact_body_collection_source(
                failures,
                body,
                index,
                allow_file_collection_source,
                allow_local_collection_source,
                target_base_hostname,
                "sloProofSource",
            )
        if artifact.get("kind") == "rag-indexing-proof":
            validate_rag_indexing_body_summary(failures, body, index)
        if artifact.get("kind") == "agent-sandbox-proof":
            validate_agent_sandbox_body_summary(failures, body, index)
        if artifact.get("kind") == "relay-realtime-proof":
            expected_mode = dig_path(data, ["relayRealtime", "mode"])
            if body.get("mode") != expected_mode:
                failures.append(f"artifacts[{index}] body mode must match relayRealtime.mode")
            if expected_mode == "disabled_until_commercial_lifecycle":
                validate_artifact_body_proofs(
                    failures,
                    body,
                    index,
                    ["productionPolicyDisabled", "authOriginPrebillAbortUsageBlockers"],
                )
            elif expected_mode == "commercial_lifecycle_enabled":
                validate_artifact_body_proofs(
                    failures,
                    body,
                    index,
                    ["productionPolicyEnabled", "authPolicy", "originPolicy", "prebillSettlement", "abortSettlement", "usageLedger"],
                )
            validate_relay_realtime_body_summary(failures, body, index)
        if artifact.get("kind") == "relay-batch-proof":
            expected_mode = dig_path(data, ["relayBatch", "mode"])
            if body.get("mode") != expected_mode:
                failures.append(f"artifacts[{index}] body mode must match relayBatch.mode")
            if expected_mode == "disabled_until_commercial_lifecycle":
                validate_artifact_body_proofs(
                    failures,
                    body,
                    index,
                    ["productionPolicyDisabled", "prebillPollingSettlementRefundAuditUsageBlockers"],
                )
            elif expected_mode == "commercial_lifecycle_enabled":
                validate_artifact_body_proofs(
                    failures,
                    body,
                    index,
                    ["productionPolicyEnabled", "prebillReservation", "pollingCompletion", "settlement", "refund", "usageAudit"],
                )
            validate_relay_batch_body_summary(failures, body, index)
        if artifact.get("kind") == "marketplace-payout-proof":
            validate_marketplace_payout_body_summary(failures, body, index)
        if artifact.get("kind") == "marketplace-governance-proof":
            validate_marketplace_governance_body_summary(failures, body, index)
        if artifact.get("kind") == "provider-runtime-config":
            validate_provider_runtime_config_body_summary(failures, body, index)
        if artifact.get("kind") == "provider-live-rail":
            validate_provider_live_rail_body(failures, body, index)
        if artifact.get("kind") == "grpc-smoke-report":
            validate_grpc_smoke_report_body(failures, body, index, data)
        if artifact.get("kind") == "secret-audit":
            validate_secret_audit_body(failures, body, index)
        if artifact.get("kind") == "microservice-database-proof":
            validate_microservice_database_body_summary(failures, body, index)

    validate_canonical_release_digests(failures, data, root)


def validate_canonical_release_digests(failures, data, artifact_dir):
    try:
        import target_release_digests

        computed = target_release_digests.compute(data, artifact_dir)
    except SystemExit as exc:
        failures.append(f"canonical target release digest computation failed: {exc}")
        return
    except Exception as exc:
        failures.append(f"canonical target release digest computation failed: {exc}")
        return

    strict = data.get("strictVerifier") if isinstance(data, dict) else {}
    if not isinstance(strict, dict):
        strict = {}
    if strict.get("targetEvidenceSha256") != computed["targetEvidenceSha256"]:
        failures.append("strictVerifier.targetEvidenceSha256 must match canonical target release digest")
    if strict.get("artifactBundleSha256") != computed["artifactBundleSha256"]:
        failures.append("strictVerifier.artifactBundleSha256 must match canonical artifact bundle digest")


def iso_now():
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def build_template(current_commit):
    recorded_at = iso_now()
    return {
        "schemaVersion": 1,
        "repository": "Oblivious",
        "commit": current_commit,
        "runId": "TODO-target-release-run-id",
        "environment": {
            "name": "TODO-target-environment-name",
            "class": "TODO-target-environment-class",
            "baseUrl": "TODO-target-base-url",
            "recordedAt": recorded_at,
        },
        "strictVerifier": {
            "command": "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
            "result": "pass",
            "skippedChecks": [],
            "startedAt": recorded_at,
            "completedAt": recorded_at,
            "targetEvidenceSha256": "TODO-target-evidence-sha256",
            "artifactBundleSha256": "TODO-artifact-bundle-sha256",
            "evidenceRef": "TODO-strict-commercial-verifier-log",
        },
        "deployment": {"deployValidation": "pass", "backupRestore": "pass", "migrationReplay": "pass", "evidenceRef": "TODO-release-log-or-artifact-id"},
        "kubernetes": {"validation": "pass", "rollout": "pass", "failover": "pass", "secretFileClass": "external-filled", "evidenceRef": "TODO-kubernetes-release-log-or-artifact-id"},
        "providers": [
            {"name": "stripe", "mode": "live", "providerEnvironment": "live", "checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass", "evidenceRef": "TODO-stripe-provider-run-id"},
            {"name": "alipay", "mode": "live", "providerEnvironment": "live", "checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass", "evidenceRef": "TODO-alipay-provider-run-id"},
            {"name": "wechatpay", "mode": "live", "providerEnvironment": "live", "checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass", "evidenceRef": "TODO-wechatpay-provider-run-id"},
        ],
        "grpc": [
            {"service": "agent", "address": "agent:50063", "generatedClient": "pass", "evidenceRef": "TODO-agent-grpc-smoke-log"},
            {"service": "workflow", "address": "workflow:50064", "generatedClient": "pass", "evidenceRef": "TODO-workflow-grpc-smoke-log"},
            {"service": "task", "address": "task:50065", "generatedClient": "pass", "evidenceRef": "TODO-task-grpc-smoke-log"},
        ],
        "grpcSmokeReport": {
            "evidenceRef": "TODO-target-grpc-smoke-report",
            "recordedAt": recorded_at,
            "timeout": "10s",
            "results": [
                {"service": "agent", "address": "agent:50063", "generatedClient": "pass", "status": "validation_error"},
                {"service": "workflow", "address": "workflow:50064", "generatedClient": "pass", "status": "validation_response"},
                {"service": "task", "address": "task:50065", "generatedClient": "pass", "status": "validation_response"},
            ],
        },
        "secretAudit": {"result": "pass", "scope": ["kubernetes", "providers", "runtime"], "evidenceRef": "TODO-secret-audit-log"},
        "workflowTelemetry": {
            "result": "pass",
            "successRate": 0.99,
            "window": "TODO-ISO8601-start/TODO-ISO8601-end",
            "totalExecutions": 100,
            "successfulExecutions": 99,
            "failedExecutions": 1,
            "evidenceRef": "TODO-telemetry-dashboard-or-export",
        },
        "requestLogObservability": {
            "backend": "clickhouse",
            "clickHouseDeployment": "pass",
            "clickHouseMigration": "pass",
            "requestLogsTable": "pass",
            "ingestQuerySmoke": "pass",
            "requestUsageJoin": "pass",
            "latencySLOTrigger": "pass",
            "latencySLOAlertDelivery": "pass",
            "latencySLORecoveryAction": "pass",
            "latencySLOWindow": "TODO-ISO8601-start/TODO-ISO8601-end",
            "latencySLOTriggeredAlerts": 1,
            "alertDelivery": {
                "configuredProviders": 1,
                "deliveredAlerts": 1,
                "failedDeliveries": 0,
                "channels": ["TODO-alert-channel"],
                "lastDeliveryId": "TODO-alert-delivery-id",
            },
            "recoveryAudit": {
                "auditRecords": 1,
                "failedActions": 0,
                "lastRecordId": "TODO-recovery-audit-id",
            },
            "evidenceRef": "TODO-clickhouse-request-log-proof",
        },
        "ragIndexing": {
            "durableQueueMigration": "pass",
            "workerDeployment": "pass",
            "enqueueDrainProbe": "pass",
            "rawParserReplay": "pass",
            "retrievalProbe": "pass",
            "staleVectorFilter": "pass",
            "evidenceRef": "TODO-rag-indexing-proof",
        },
        "relayRealtime": {
            "mode": "commercial_lifecycle_enabled",
            "productionPolicyEnabled": "pass",
            "authPolicy": "pass",
            "originPolicy": "pass",
            "prebillSettlement": "pass",
            "abortSettlement": "pass",
            "usageLedger": "pass",
            "evidenceRef": "TODO-relay-realtime-proof",
        },
        "relayBatch": {
            "mode": "commercial_lifecycle_enabled",
            "productionPolicyEnabled": "pass",
            "prebillReservation": "pass",
            "pollingCompletion": "pass",
            "settlement": "pass",
            "refund": "pass",
            "usageAudit": "pass",
            "evidenceRef": "TODO-relay-batch-proof",
        },
        "marketplacePayouts": {
            "providerMode": "webhook",
            "outboundDispatch": "pass",
            "inboundWebhookLifecycle": "pass",
            "settlementLedger": "pass",
            "reconciliation": "pass",
            "refundChargebackHandling": "pass",
            "evidenceRef": "TODO-marketplace-payout-proof",
        },
        "marketplaceGovernance": {
            "reviewQueue": "pass",
            "appealQueue": "pass",
            "appealDecisionLifecycle": "pass",
            "reviewAssignment": "pass",
            "reviewSLAEnforcement": "pass",
            "abuseReportLifecycle": "pass",
            "evidenceRef": "TODO-marketplace-governance-proof",
        },
        "providerRuntimeConfig": {"stripe": "pass", "alipay": "pass", "wechatpay": "pass", "providerEnv": "pass", "checkoutBaseUrls": "pass", "webhookRoutes": "pass", "webhookVerification": "pass", "evidenceRef": "TODO-provider-runtime-config-proof"},
        "microserviceDatabases": {
            "mode": "microservices",
            "serviceUrlClass": "external-filled",
            "relay": "pass",
            "chat": "pass",
            "workflow": "pass",
            "rag": "pass",
            "agent": "pass",
            "billing": "pass",
            "marketplace": "pass",
            "admin": "pass",
            "channel": "pass",
            "task": "pass",
            "observability": "pass",
            "migrationReadiness": "pass",
            "evidenceRef": "TODO-microservice-database-proof",
        },
        "artifacts": [
            {"id": "TODO-strict-commercial-verifier-log", "kind": "strict-verifier-log", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-strict-commercial-verifier-log-uri", "recordedAt": recorded_at},
            {"id": "TODO-release-log-or-artifact-id", "kind": "deployment-log", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-release-log-uri", "recordedAt": recorded_at, "proofs": {"deployValidation": "pass", "backupRestore": "pass", "migrationReplay": "pass"}},
            {"id": "TODO-kubernetes-release-log-or-artifact-id", "kind": "kubernetes-validation", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-kubernetes-log-uri", "recordedAt": recorded_at, "proofs": {"validation": "pass", "rollout": "pass", "failover": "pass"}},
            {"id": "TODO-stripe-provider-run-id", "kind": "provider-live-rail", "provider": "stripe", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-stripe-provider-log-uri", "recordedAt": recorded_at, "proofs": {"checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass"}},
            {"id": "TODO-alipay-provider-run-id", "kind": "provider-live-rail", "provider": "alipay", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-alipay-provider-log-uri", "recordedAt": recorded_at, "proofs": {"checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass"}},
            {"id": "TODO-wechatpay-provider-run-id", "kind": "provider-live-rail", "provider": "wechatpay", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-wechatpay-provider-log-uri", "recordedAt": recorded_at, "proofs": {"checkout": "pass", "refund": "pass", "payout": "pass", "reconciliation": "pass"}},
            {"id": "TODO-target-grpc-smoke-report", "kind": "grpc-smoke-report", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-target-grpc-smoke-report-uri", "recordedAt": recorded_at},
            {"id": "TODO-secret-audit-log", "kind": "secret-audit", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-secret-audit-log-uri", "recordedAt": recorded_at},
            {"id": "TODO-telemetry-dashboard-or-export", "kind": "workflow-telemetry", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-telemetry-dashboard-or-export-uri", "recordedAt": recorded_at},
            {"id": "TODO-clickhouse-request-log-proof", "kind": "request-log-observability", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-clickhouse-request-log-proof-uri", "recordedAt": recorded_at, "proofs": {"clickHouseDeployment": "pass", "clickHouseMigration": "pass", "requestLogsTable": "pass", "ingestQuerySmoke": "pass", "requestUsageJoin": "pass", "latencySLOTrigger": "pass", "latencySLOAlertDelivery": "pass", "latencySLORecoveryAction": "pass"}},
            {"id": "TODO-rag-indexing-proof", "kind": "rag-indexing-proof", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-rag-indexing-proof-uri", "recordedAt": recorded_at, "proofs": {"durableQueueMigration": "pass", "workerDeployment": "pass", "enqueueDrainProbe": "pass", "rawParserReplay": "pass", "retrievalProbe": "pass", "staleVectorFilter": "pass"}},
            {"id": "TODO-relay-realtime-proof", "kind": "relay-realtime-proof", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-relay-realtime-proof-uri", "recordedAt": recorded_at, "proofs": {"productionPolicyEnabled": "pass", "authPolicy": "pass", "originPolicy": "pass", "prebillSettlement": "pass", "abortSettlement": "pass", "usageLedger": "pass"}},
            {"id": "TODO-relay-batch-proof", "kind": "relay-batch-proof", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-relay-batch-proof-uri", "recordedAt": recorded_at, "proofs": {"productionPolicyEnabled": "pass", "prebillReservation": "pass", "pollingCompletion": "pass", "settlement": "pass", "refund": "pass", "usageAudit": "pass"}},
            {"id": "TODO-marketplace-payout-proof", "kind": "marketplace-payout-proof", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-marketplace-payout-proof-uri", "recordedAt": recorded_at, "proofs": {"outboundDispatch": "pass", "inboundWebhookLifecycle": "pass", "settlementLedger": "pass", "reconciliation": "pass", "refundChargebackHandling": "pass"}},
            {"id": "TODO-marketplace-governance-proof", "kind": "marketplace-governance-proof", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-marketplace-governance-proof-uri", "recordedAt": recorded_at, "proofs": {"reviewQueue": "pass", "appealQueue": "pass", "appealDecisionLifecycle": "pass", "reviewAssignment": "pass", "reviewSLAEnforcement": "pass", "abuseReportLifecycle": "pass"}},
            {"id": "TODO-provider-runtime-config-proof", "kind": "provider-runtime-config", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-provider-runtime-config-proof-uri", "recordedAt": recorded_at, "proofs": {"stripe": "pass", "alipay": "pass", "wechatpay": "pass", "providerEnv": "pass", "checkoutBaseUrls": "pass", "webhookRoutes": "pass", "webhookVerification": "pass"}},
            {"id": "TODO-microservice-database-proof", "kind": "microservice-database-proof", "commit": current_commit, "runId": "TODO-target-release-run-id", "uri": "TODO-microservice-database-proof-uri", "recordedAt": recorded_at, "proofs": {"relay": "pass", "chat": "pass", "workflow": "pass", "rag": "pass", "agent": "pass", "billing": "pass", "marketplace": "pass", "admin": "pass", "channel": "pass", "task": "pass", "observability": "pass", "migrationReadiness": "pass"}},
        ],
    }


def validate(
    data,
    current_commit,
    allow_mismatch,
    artifact_dir=None,
    allow_file_collection_source=False,
    allow_local_collection_source=False,
    allow_non_production_target=False,
    allow_disabled_commercial_lifecycle=False,
):
    failures = []
    if not isinstance(data, dict):
        return ["evidence root must be a JSON object"], {}, [], None

    failures.append("schemaVersion must be 1") if data.get("schemaVersion") != 1 else None
    failures.append("repository must be Oblivious") if data.get("repository") != "Oblivious" else None
    commit = require_string(failures, data, ["commit"])
    if commit and commit != current_commit and not allow_mismatch:
        failures.append(f"commit must match current HEAD {current_commit}")
    run_id = require_string(failures, data, ["runId"])
    if placeholder(run_id):
        failures.append("runId must reference a concrete target evidence run, not a placeholder")

    for field in ["name", "class", "baseUrl"]:
        value = require_string(failures, data, ["environment", field])
        if placeholder(value):
            failures.append(f"environment.{field} must reference a concrete target environment value, not a placeholder")
    environment_class = dig_path(data, ["environment", "class"])
    normalized_environment_class = environment_class.strip().lower() if isinstance(environment_class, str) else None
    if isinstance(environment_class, str) and normalized_environment_class not in ALLOWED_ENVIRONMENT_CLASSES:
        failures.append("environment.class must be staging, preproduction, or production")
    elif normalized_environment_class and normalized_environment_class != "production" and not allow_non_production_target:
        failures.append("environment.class must be production for final target evidence")
    require_http_url(
        failures,
        data,
        ["environment", "baseUrl"],
        "environment.baseUrl must be an HTTP(S) URL",
        local_error="environment.baseUrl must target a non-local target environment",
    )
    environment_base_url = dig_path(data, ["environment", "baseUrl"])
    target_base_hostname = None
    if isinstance(environment_base_url, str):
        target_base_hostname = urlsplit(environment_base_url.strip()).hostname
    if isinstance(environment_base_url, str) and userinfo_uri(environment_base_url):
        failures.append("environment.baseUrl must not embed credentials in URI userinfo")
    if isinstance(environment_base_url, str) and secret_like_uri(environment_base_url):
        failures.append("environment.baseUrl must not embed secret-like query or fragment parameters")
    recorded_at = require_string(failures, data, ["environment", "recordedAt"])
    if isinstance(recorded_at, str) and parse_iso8601_safely(recorded_at) is None:
        failures.append("environment.recordedAt must be ISO-8601")

    require_string(failures, data, ["strictVerifier", "command"])
    require_pass(failures, data, ["strictVerifier", "result"])
    command = dig_path(data, ["strictVerifier", "command"])
    if isinstance(command, str):
        validate_strict_verifier_command(failures, command)
    if dig_path(data, ["strictVerifier", "skippedChecks"]) != []:
        failures.append("strictVerifier.skippedChecks must be an empty array")
    strict_started_at_raw = require_string(failures, data, ["strictVerifier", "startedAt"])
    strict_completed_at_raw = require_string(failures, data, ["strictVerifier", "completedAt"])
    strict_started_at = parse_iso8601_safely(strict_started_at_raw)
    strict_completed_at = parse_iso8601_safely(strict_completed_at_raw)
    if isinstance(strict_started_at_raw, str) and strict_started_at is None:
        failures.append("strictVerifier.startedAt must be ISO-8601")
    if isinstance(strict_completed_at_raw, str) and strict_completed_at is None:
        failures.append("strictVerifier.completedAt must be ISO-8601")
    if strict_started_at and strict_completed_at and strict_completed_at < strict_started_at:
        failures.append("strictVerifier.completedAt must be at or after strictVerifier.startedAt")
    for field in ["targetEvidenceSha256", "artifactBundleSha256"]:
        value = require_string(failures, data, ["strictVerifier", field])
        if isinstance(value, str) and not sha256_value(value):
            failures.append(f"strictVerifier.{field} must be a 64-character SHA-256 hex digest")
    require_evidence_ref(failures, data, ["strictVerifier", "evidenceRef"])

    for field in ["deployValidation", "backupRestore", "migrationReplay"]:
        require_pass(failures, data, ["deployment", field])
    require_evidence_ref(failures, data, ["deployment", "evidenceRef"])

    for field in ["validation", "rollout", "failover"]:
        require_pass(failures, data, ["kubernetes", field])
    secret_file_class = require_string(failures, data, ["kubernetes", "secretFileClass"])
    if isinstance(secret_file_class, str) and secret_file_class.strip() != "external-filled":
        failures.append("kubernetes.secretFileClass must be external-filled")
    require_evidence_ref(failures, data, ["kubernetes", "evidenceRef"])

    providers = data.get("providers")
    required_providers = ["stripe", "alipay", "wechatpay"]
    providers_by_name = {}
    if not isinstance(providers, list) or not providers:
        failures.append("providers must include at least one live provider evidence entry")
    else:
        for index, provider in enumerate(providers):
            if not isinstance(provider, dict):
                failures.append(f"providers[{index}] must be an object")
                continue
            prefix = ["providers", index]
            name = str(require_string(failures, data, prefix + ["name"]) or "").strip()
            if name and name not in required_providers:
                failures.append(f"providers[{index}].name must be stripe, alipay, or wechatpay")
            if name:
                if name in providers_by_name:
                    failures.append(f"providers must not duplicate {name} evidence")
                else:
                    providers_by_name[name] = provider
            if provider.get("mode") != "live":
                failures.append(f"providers[{index}].mode must be live")
            if provider.get("providerEnvironment") != "live":
                failures.append(f"providers[{index}].providerEnvironment must be live")
            for field in ["checkout", "refund", "payout", "reconciliation"]:
                if provider.get(field) != "pass":
                    failures.append(f"providers[{index}].{field} must be pass")
            if blank(provider.get("evidenceRef")):
                failures.append(f"providers[{index}].evidenceRef is required")
            if placeholder(provider.get("evidenceRef")):
                failures.append(f"providers[{index}].evidenceRef must reference a concrete target artifact, not a placeholder")
        missing_providers = [name for name in required_providers if name not in providers_by_name]
        if missing_providers:
            failures.append(f"providers must include live evidence for stripe, alipay, and wechatpay (missing: {', '.join(missing_providers)})")

    grpc = data.get("grpc")
    required_services = ["agent", "workflow", "task"]
    expected_grpc_ports = {"agent": "50063", "workflow": "50064", "task": "50065"}
    expected_grpc_smoke_statuses = {"agent": "validation_error", "workflow": "validation_response", "task": "validation_response"}
    grpc_entries_by_service = {}
    if not isinstance(grpc, list):
        failures.append("grpc must be an array")
    else:
        present = [item.get("service") for item in grpc if isinstance(item, dict)]
        for service in [service for service in required_services if service not in present]:
            failures.append(f"grpc must include {service} service evidence")
        for index, entry in enumerate(grpc):
            if not isinstance(entry, dict):
                failures.append(f"grpc[{index}] must be an object")
                continue
            service = str(entry.get("service") or "").strip()
            if service and service not in required_services:
                failures.append(f"grpc[{index}].service must be agent, workflow, or task")
            address = entry.get("address")
            if blank(address):
                failures.append(f"grpc[{index}].address is required")
            parsed_address = parse_plain_grpc_address(address)
            if not blank(address) and parsed_address is None:
                failures.append(f"grpc[{index}].address for {service} must be a plain host:port endpoint")
            if placeholder(address):
                failures.append(f"grpc[{index}].address for {service} must reference a concrete target service endpoint, not a placeholder")
            if service:
                if service in grpc_entries_by_service:
                    failures.append(f"grpc must not duplicate {service} service evidence")
                else:
                    grpc_entries_by_service[service] = entry
            expected_port = expected_grpc_ports.get(service)
            if parsed_address and expected_port and parsed_address["port"] != expected_port:
                failures.append(f"grpc[{index}].address for {service} must target port {expected_port}")
            if parsed_address and non_target_host(parsed_address["host"]):
                failures.append(f"grpc[{index}].address for {service} must target a non-local service endpoint")
            if entry.get("generatedClient") != "pass":
                failures.append(f"grpc[{index}].generatedClient must be pass")
            if blank(entry.get("evidenceRef")):
                failures.append(f"grpc[{index}].evidenceRef is required")
            if placeholder(entry.get("evidenceRef")):
                failures.append(f"grpc[{index}].evidenceRef must reference a concrete target artifact, not a placeholder")

    grpc_smoke_report = data.get("grpcSmokeReport")
    grpc_smoke_recorded_at = None
    if not isinstance(grpc_smoke_report, dict):
        failures.append("grpcSmokeReport is required")
    else:
        grpc_smoke_evidence_ref = require_evidence_ref(failures, data, ["grpcSmokeReport", "evidenceRef"])
        if isinstance(grpc_smoke_evidence_ref, str):
            for service, entry in grpc_entries_by_service.items():
                if isinstance(entry.get("evidenceRef"), str) and entry.get("evidenceRef") != grpc_smoke_evidence_ref:
                    failures.append(f"grpc {service} evidenceRef must match grpcSmokeReport.evidenceRef")
        smoke_recorded_at = require_string(failures, data, ["grpcSmokeReport", "recordedAt"])
        grpc_smoke_recorded_at = parse_iso8601_safely(smoke_recorded_at)
        if isinstance(smoke_recorded_at, str) and grpc_smoke_recorded_at is None:
            failures.append("grpcSmokeReport.recordedAt must be ISO-8601")
        smoke_timeout = require_string(failures, data, ["grpcSmokeReport", "timeout"])
        if isinstance(smoke_timeout, str) and smoke_timeout.strip() and not positive_go_duration_string(smoke_timeout):
            failures.append("grpcSmokeReport.timeout must be a positive Go duration string")
        smoke_results = grpc_smoke_report.get("results")
        smoke_results_by_service = {}
        if not isinstance(smoke_results, list):
            failures.append("grpcSmokeReport.results must be an array")
        else:
            for index, result in enumerate(smoke_results):
                if not isinstance(result, dict):
                    failures.append(f"grpcSmokeReport.results[{index}] must be an object")
                    continue
                service = str(result.get("service") or "").strip()
                if service == "":
                    failures.append(f"grpcSmokeReport.results[{index}].service is required")
                elif service not in required_services:
                    failures.append(f"grpcSmokeReport.results[{index}].service must be agent, workflow, or task")
                elif service in smoke_results_by_service:
                    failures.append(f"grpcSmokeReport.results must not duplicate {service} service results")
                else:
                    smoke_results_by_service[service] = result
                result_address = result.get("address")
                if blank(result_address):
                    failures.append(f"grpcSmokeReport.results[{index}].address is required")
                parsed_result_address = parse_plain_grpc_address(result_address)
                if not blank(result_address) and parsed_result_address is None:
                    failures.append(f"grpcSmokeReport.results[{index}].address for {service} must be a plain host:port endpoint")
                if placeholder(result_address):
                    failures.append(f"grpcSmokeReport.results[{index}].address for {service} must reference a concrete target service endpoint, not a placeholder")
                manifest_entry = grpc_entries_by_service.get(service)
                if manifest_entry and str(result_address) != str(manifest_entry.get("address")):
                    failures.append(f"grpcSmokeReport.results[{index}].address must match grpc {service} address")
                expected_port = expected_grpc_ports.get(service)
                if parsed_result_address and expected_port and parsed_result_address["port"] != expected_port:
                    failures.append(f"grpcSmokeReport.results[{index}].address for {service} must target port {expected_port}")
                if parsed_result_address and non_target_host(parsed_result_address["host"]):
                    failures.append(f"grpcSmokeReport.results[{index}].address for {service} must target a non-local service endpoint")
                if result.get("generatedClient") != "pass":
                    failures.append(f"grpcSmokeReport.results[{index}].generatedClient must be pass")
                status = require_string(failures, data, ["grpcSmokeReport", "results", index, "status"])
                expected_status = expected_grpc_smoke_statuses.get(service)
                if isinstance(status, str) and expected_status and status != expected_status:
                    failures.append(f"grpcSmokeReport.results[{index}].status for {service} must be {expected_status}")
            missing_smoke_services = [service for service in required_services if service not in smoke_results_by_service]
            if missing_smoke_services:
                failures.append(f"grpcSmokeReport.results must include agent, workflow, and task smoke results (missing: {', '.join(missing_smoke_services)})")

    require_pass(failures, data, ["secretAudit", "result"])
    require_evidence_ref(failures, data, ["secretAudit", "evidenceRef"])
    scope = dig_path(data, ["secretAudit", "scope"])
    if not isinstance(scope, list) or not scope or any(not isinstance(item, str) or item.strip() == "" for item in scope):
        failures.append("secretAudit.scope must be a non-empty array of strings")
    else:
        required_secret_audit_scopes = ["kubernetes", "providers", "runtime"]
        normalized_scope = []
        for item in scope:
            normalized = item.strip().lower()
            if normalized not in normalized_scope:
                normalized_scope.append(normalized)
        missing_secret_audit_scopes = [item for item in required_secret_audit_scopes if item not in normalized_scope]
        if missing_secret_audit_scopes:
            failures.append(f"secretAudit.scope must include kubernetes, providers, and runtime (missing: {', '.join(missing_secret_audit_scopes)})")

    require_pass(failures, data, ["workflowTelemetry", "result"])
    success_rate = dig_path(data, ["workflowTelemetry", "successRate"])
    if isinstance(success_rate, bool) or not isinstance(success_rate, (int, float)) or success_rate < 0.99 or success_rate > 1.0:
        failures.append("workflowTelemetry.successRate must be between 0.99 and 1.0")
    total_executions = dig_path(data, ["workflowTelemetry", "totalExecutions"])
    successful_executions = dig_path(data, ["workflowTelemetry", "successfulExecutions"])
    failed_executions = dig_path(data, ["workflowTelemetry", "failedExecutions"])
    if type(total_executions) is not int or total_executions <= 0:
        failures.append("workflowTelemetry.totalExecutions must be greater than zero")
    if type(successful_executions) is not int or successful_executions < 0:
        failures.append("workflowTelemetry.successfulExecutions must be a non-negative integer")
    if type(failed_executions) is not int or failed_executions < 0:
        failures.append("workflowTelemetry.failedExecutions must be a non-negative integer")
    if (
        type(total_executions) is int
        and type(successful_executions) is int
        and type(failed_executions) is int
        and successful_executions + failed_executions != total_executions
    ):
        failures.append("workflowTelemetry.successfulExecutions plus workflowTelemetry.failedExecutions must equal workflowTelemetry.totalExecutions")
    if (
        isinstance(success_rate, (int, float))
        and not isinstance(success_rate, bool)
        and type(total_executions) is int
        and total_executions > 0
        and type(successful_executions) is int
        and abs((successful_executions / total_executions) - float(success_rate)) > 0.0005
    ):
        failures.append("workflowTelemetry.successRate must equal successfulExecutions / totalExecutions")
    require_iso8601_interval(
        failures,
        data,
        ["workflowTelemetry", "window"],
        interval_error="workflowTelemetry.window must be an ISO-8601 start/end interval",
        ordering_error="workflowTelemetry.window end must be at or after start",
    )
    workflow_telemetry_window_end = iso8601_interval_end(dig_path(data, ["workflowTelemetry", "window"]))
    require_evidence_ref(failures, data, ["workflowTelemetry", "evidenceRef"])

    if not isinstance(dig_path(data, ["requestLogObservability"]), dict):
        failures.append("requestLogObservability is required")
    else:
        backend = require_string(failures, data, ["requestLogObservability", "backend"])
        if isinstance(backend, str) and backend.strip() != "clickhouse":
            failures.append("requestLogObservability.backend must be clickhouse")
        for field in [
            "clickHouseDeployment",
            "clickHouseMigration",
            "requestLogsTable",
            "ingestQuerySmoke",
            "requestUsageJoin",
            "latencySLOTrigger",
            "latencySLOAlertDelivery",
            "latencySLORecoveryAction",
        ]:
            require_pass(failures, data, ["requestLogObservability", field])
        validate_request_log_slo_details(
            failures,
            dig_path(data, ["requestLogObservability"]),
            "requestLogObservability",
            "latencySLOWindow",
            "latencySLOTriggeredAlerts",
        )
        require_evidence_ref(failures, data, ["requestLogObservability", "evidenceRef"])

    if not isinstance(dig_path(data, ["ragIndexing"]), dict):
        failures.append("ragIndexing is required")
    else:
        for field in ["durableQueueMigration", "workerDeployment", "enqueueDrainProbe", "rawParserReplay", "retrievalProbe", "staleVectorFilter"]:
            require_pass(failures, data, ["ragIndexing", field])
        require_evidence_ref(failures, data, ["ragIndexing", "evidenceRef"])

    if not isinstance(dig_path(data, ["relayRealtime"]), dict):
        failures.append("relayRealtime is required")
    else:
        mode = require_string(failures, data, ["relayRealtime", "mode"])
        if isinstance(mode, str):
            mode = mode.strip()
            if mode == "disabled_until_commercial_lifecycle":
                if not allow_disabled_commercial_lifecycle:
                    failures.append("relayRealtime.mode must be commercial_lifecycle_enabled for final target evidence")
                for field in ["productionPolicyDisabled", "authOriginPrebillAbortUsageBlockers"]:
                    require_pass(failures, data, ["relayRealtime", field])
            elif mode == "commercial_lifecycle_enabled":
                for field in [
                    "productionPolicyEnabled",
                    "authPolicy",
                    "originPolicy",
                    "prebillSettlement",
                    "abortSettlement",
                    "usageLedger",
                ]:
                    require_pass(failures, data, ["relayRealtime", field])
            else:
                failures.append("relayRealtime.mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled")
        require_evidence_ref(failures, data, ["relayRealtime", "evidenceRef"])

    if not isinstance(dig_path(data, ["relayBatch"]), dict):
        failures.append("relayBatch is required")
    else:
        mode = require_string(failures, data, ["relayBatch", "mode"])
        if isinstance(mode, str):
            mode = mode.strip()
            if mode == "disabled_until_commercial_lifecycle":
                if not allow_disabled_commercial_lifecycle:
                    failures.append("relayBatch.mode must be commercial_lifecycle_enabled for final target evidence")
                for field in ["productionPolicyDisabled", "prebillPollingSettlementRefundAuditUsageBlockers"]:
                    require_pass(failures, data, ["relayBatch", field])
            elif mode == "commercial_lifecycle_enabled":
                for field in [
                    "productionPolicyEnabled",
                    "prebillReservation",
                    "pollingCompletion",
                    "settlement",
                    "refund",
                    "usageAudit",
                ]:
                    require_pass(failures, data, ["relayBatch", field])
            else:
                failures.append("relayBatch.mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled")
        require_evidence_ref(failures, data, ["relayBatch", "evidenceRef"])

    if not isinstance(dig_path(data, ["marketplacePayouts"]), dict):
        failures.append("marketplacePayouts is required")
    else:
        provider_mode = require_string(failures, data, ["marketplacePayouts", "providerMode"])
        if isinstance(provider_mode, str) and provider_mode.strip() != "webhook":
            failures.append("marketplacePayouts.providerMode must be webhook")
        for field in ["outboundDispatch", "inboundWebhookLifecycle", "settlementLedger", "reconciliation", "refundChargebackHandling"]:
            require_pass(failures, data, ["marketplacePayouts", field])
        require_evidence_ref(failures, data, ["marketplacePayouts", "evidenceRef"])

    if not isinstance(dig_path(data, ["marketplaceGovernance"]), dict):
        failures.append("marketplaceGovernance is required")
    else:
        for field in ["reviewQueue", "appealQueue", "appealDecisionLifecycle", "reviewAssignment", "reviewSLAEnforcement", "abuseReportLifecycle"]:
            require_pass(failures, data, ["marketplaceGovernance", field])
        require_evidence_ref(failures, data, ["marketplaceGovernance", "evidenceRef"])

    if not isinstance(dig_path(data, ["providerRuntimeConfig"]), dict):
        failures.append("providerRuntimeConfig is required")
    else:
        for field in ["stripe", "alipay", "wechatpay", "providerEnv", "checkoutBaseUrls", "webhookRoutes", "webhookVerification"]:
            require_pass(failures, data, ["providerRuntimeConfig", field])
        require_evidence_ref(failures, data, ["providerRuntimeConfig", "evidenceRef"])

    if not isinstance(dig_path(data, ["microserviceDatabases"]), dict):
        failures.append("microserviceDatabases is required")
    else:
        db_mode = require_string(failures, data, ["microserviceDatabases", "mode"])
        if isinstance(db_mode, str) and db_mode.strip() != "microservices":
            failures.append("microserviceDatabases.mode must be microservices")
        service_url_class = require_string(failures, data, ["microserviceDatabases", "serviceUrlClass"])
        if isinstance(service_url_class, str) and service_url_class.strip() != "external-filled":
            failures.append("microserviceDatabases.serviceUrlClass must be external-filled")
        for field in ["relay", "chat", "workflow", "rag", "agent", "billing", "marketplace", "admin", "channel", "task", "observability", "migrationReadiness"]:
            require_pass(failures, data, ["microserviceDatabases", field])
        require_evidence_ref(failures, data, ["microserviceDatabases", "evidenceRef"])

    artifacts = data.get("artifacts")
    artifact_ids = {}
    artifact_indexes = {}
    artifact_recorded_times = {}
    if not isinstance(artifacts, list) or not artifacts:
        failures.append("artifacts must include at least one target artifact entry")
    else:
        for index, artifact in enumerate(artifacts):
            if not isinstance(artifact, dict):
                failures.append(f"artifacts[{index}] must be an object")
                continue
            artifact_id = require_string(failures, data, ["artifacts", index, "id"])
            if isinstance(artifact_id, str):
                if placeholder(artifact_id):
                    failures.append(f"artifacts[{index}].id must reference a concrete target artifact, not a placeholder")
                if artifact_id in artifact_ids:
                    failures.append(f"artifacts must not duplicate {artifact_id}")
                else:
                    artifact_ids[artifact_id] = artifact
                    artifact_indexes[artifact_id] = index
            kind = require_string(failures, data, ["artifacts", index, "kind"])
            if placeholder(kind):
                failures.append(f"artifacts[{index}].kind must describe a concrete target artifact, not a placeholder")
            artifact_commit = require_string(failures, data, ["artifacts", index, "commit"])
            if isinstance(artifact_commit, str):
                if placeholder(artifact_commit):
                    failures.append(f"artifacts[{index}].commit must reference a concrete release commit, not a placeholder")
                if isinstance(commit, str) and not placeholder(artifact_commit) and artifact_commit != commit:
                    failures.append(f"artifacts[{index}].commit must match manifest commit")
            artifact_run_id = require_string(failures, data, ["artifacts", index, "runId"])
            if isinstance(artifact_run_id, str):
                if placeholder(artifact_run_id):
                    failures.append(f"artifacts[{index}].runId must reference a concrete target evidence run, not a placeholder")
                if isinstance(run_id, str) and not placeholder(artifact_run_id) and artifact_run_id != run_id:
                    failures.append(f"artifacts[{index}].runId must match manifest runId")
            uri = require_string(failures, data, ["artifacts", index, "uri"])
            if placeholder(uri):
                failures.append(f"artifacts[{index}].uri must reference a concrete target artifact, not a placeholder")
            if not remote_artifact_uri(uri):
                failures.append(f"artifacts[{index}].uri must reference a remote target artifact URI")
            if secret_like_uri(uri):
                failures.append(f"artifacts[{index}].uri must not embed secret-like query or fragment parameters")
            if userinfo_uri(uri):
                failures.append(f"artifacts[{index}].uri must not embed credentials in URI userinfo")
            artifact_recorded_at = require_string(failures, data, ["artifacts", index, "recordedAt"])
            parsed_artifact_recorded_at = parse_iso8601_safely(artifact_recorded_at)
            if isinstance(artifact_recorded_at, str) and parsed_artifact_recorded_at is None:
                failures.append(f"artifacts[{index}].recordedAt must be ISO-8601")
            if isinstance(artifact_id, str) and not placeholder(artifact_id) and parsed_artifact_recorded_at:
                artifact_recorded_times[artifact_id] = parsed_artifact_recorded_at
            sha256 = artifact.get("sha256")
            if not isinstance(sha256, str) or sha256.strip() == "":
                failures.append(f"artifacts[{index}].sha256 is required")
            elif not re.match(r"^[0-9a-f]{64}$", sha256, re.IGNORECASE):
                failures.append(f"artifacts[{index}].sha256 must be a 64-character hex digest")

    referenced_artifact_ids = set()
    for ref_path, value in collect_required_evidence_refs(data):
        if not isinstance(value, str) or placeholder(value):
            continue
        referenced_artifact_ids.add(value)
        if value not in artifact_ids:
            failures.append(f"{path_label(ref_path)} must reference an artifact id listed in artifacts")

    for artifact_id in artifact_ids:
        if placeholder(artifact_id) or artifact_id in referenced_artifact_ids:
            continue
        failures.append(f"artifacts[{artifact_indexes[artifact_id]}].id {artifact_id} must be referenced by a required evidenceRef")

    require_artifact_kind(failures, data, artifact_ids, ["strictVerifier", "evidenceRef"], "strict-verifier-log")
    strict_verifier_ref = dig_path(data, ["strictVerifier", "evidenceRef"])
    if strict_started_at and strict_completed_at and isinstance(strict_verifier_ref, str) and not placeholder(strict_verifier_ref):
        strict_verifier_artifact = artifact_ids.get(strict_verifier_ref)
        if isinstance(strict_verifier_artifact, dict):
            artifact_recorded_at = parse_iso8601_safely(strict_verifier_artifact.get("recordedAt"))
            if artifact_recorded_at and (artifact_recorded_at < strict_started_at or artifact_recorded_at > strict_completed_at):
                failures.append("strictVerifier.evidenceRef artifact recordedAt must be within strictVerifier.startedAt/completedAt")
    require_artifact_kind(failures, data, artifact_ids, ["deployment", "evidenceRef"], "deployment-log")
    require_artifact_kind(failures, data, artifact_ids, ["kubernetes", "evidenceRef"], "kubernetes-validation")
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["deployment", "evidenceRef"],
        ["deployValidation", "backupRestore", "migrationReplay"],
    )
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["kubernetes", "evidenceRef"],
        ["validation", "rollout", "failover"],
    )
    provider_entries = providers if isinstance(providers, list) else []
    for index, provider in enumerate(provider_entries):
        require_artifact_kind(failures, data, artifact_ids, ["providers", index, "evidenceRef"], "provider-live-rail")
        require_artifact_proofs(
            failures,
            data,
            artifact_ids,
            ["providers", index, "evidenceRef"],
            ["checkout", "refund", "payout", "reconciliation"],
        )
        if not isinstance(provider, dict):
            continue
        name = provider.get("name")
        ref = provider.get("evidenceRef")
        if not isinstance(name, str) or not isinstance(ref, str) or placeholder(name) or placeholder(ref):
            continue
        artifact = artifact_ids.get(ref)
        if isinstance(artifact, dict) and artifact.get("kind") == "provider-live-rail":
            artifact_provider = artifact.get("provider")
            if not isinstance(artifact_provider, str) or placeholder(artifact_provider) or artifact_provider.strip() != name.strip():
                failures.append(f"providers[{index}].evidenceRef must reference provider-specific live evidence for {name.strip()}")
    grpc_entries = grpc if isinstance(grpc, list) else []
    for index, _entry in enumerate(grpc_entries):
        require_artifact_kind(failures, data, artifact_ids, ["grpc", index, "evidenceRef"], "grpc-smoke-report")
    require_artifact_kind(failures, data, artifact_ids, ["grpcSmokeReport", "evidenceRef"], "grpc-smoke-report")
    grpc_smoke_report_ref = dig_path(data, ["grpcSmokeReport", "evidenceRef"])
    if grpc_smoke_recorded_at and isinstance(grpc_smoke_report_ref, str) and not placeholder(grpc_smoke_report_ref):
        grpc_smoke_artifact_recorded_at = artifact_recorded_times.get(grpc_smoke_report_ref)
        if grpc_smoke_artifact_recorded_at and grpc_smoke_artifact_recorded_at < grpc_smoke_recorded_at:
            failures.append("grpcSmokeReport.evidenceRef artifact recordedAt must be at or after grpcSmokeReport.recordedAt")
    require_artifact_kind(failures, data, artifact_ids, ["secretAudit", "evidenceRef"], "secret-audit")
    require_artifact_kind(failures, data, artifact_ids, ["workflowTelemetry", "evidenceRef"], "workflow-telemetry")
    require_artifact_kind(failures, data, artifact_ids, ["requestLogObservability", "evidenceRef"], "request-log-observability")
    require_artifact_kind(failures, data, artifact_ids, ["ragIndexing", "evidenceRef"], "rag-indexing-proof")
    require_artifact_kind(failures, data, artifact_ids, ["relayRealtime", "evidenceRef"], "relay-realtime-proof")
    require_artifact_kind(failures, data, artifact_ids, ["relayBatch", "evidenceRef"], "relay-batch-proof")
    require_artifact_kind(failures, data, artifact_ids, ["marketplacePayouts", "evidenceRef"], "marketplace-payout-proof")
    require_artifact_kind(failures, data, artifact_ids, ["marketplaceGovernance", "evidenceRef"], "marketplace-governance-proof")
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["requestLogObservability", "evidenceRef"],
        [
            "clickHouseDeployment",
            "clickHouseMigration",
            "requestLogsTable",
            "ingestQuerySmoke",
            "requestUsageJoin",
            "latencySLOTrigger",
            "latencySLOAlertDelivery",
            "latencySLORecoveryAction",
        ],
    )
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["ragIndexing", "evidenceRef"],
        [
            "durableQueueMigration",
            "workerDeployment",
            "enqueueDrainProbe",
            "rawParserReplay",
            "retrievalProbe",
            "staleVectorFilter",
        ],
    )
    relay_realtime_mode = dig_path(data, ["relayRealtime", "mode"])
    if relay_realtime_mode == "disabled_until_commercial_lifecycle":
        require_artifact_proofs(
            failures,
            data,
            artifact_ids,
            ["relayRealtime", "evidenceRef"],
            ["productionPolicyDisabled", "authOriginPrebillAbortUsageBlockers"],
        )
    elif relay_realtime_mode == "commercial_lifecycle_enabled":
        require_artifact_proofs(
            failures,
            data,
            artifact_ids,
            ["relayRealtime", "evidenceRef"],
            ["productionPolicyEnabled", "authPolicy", "originPolicy", "prebillSettlement", "abortSettlement", "usageLedger"],
        )
    relay_batch_mode = dig_path(data, ["relayBatch", "mode"])
    if relay_batch_mode == "disabled_until_commercial_lifecycle":
        require_artifact_proofs(
            failures,
            data,
            artifact_ids,
            ["relayBatch", "evidenceRef"],
            ["productionPolicyDisabled", "prebillPollingSettlementRefundAuditUsageBlockers"],
        )
    elif relay_batch_mode == "commercial_lifecycle_enabled":
        require_artifact_proofs(
            failures,
            data,
            artifact_ids,
            ["relayBatch", "evidenceRef"],
            ["productionPolicyEnabled", "prebillReservation", "pollingCompletion", "settlement", "refund", "usageAudit"],
        )
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["marketplacePayouts", "evidenceRef"],
        ["outboundDispatch", "inboundWebhookLifecycle", "settlementLedger", "reconciliation", "refundChargebackHandling"],
    )
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["marketplaceGovernance", "evidenceRef"],
        ["reviewQueue", "appealQueue", "appealDecisionLifecycle", "reviewAssignment", "reviewSLAEnforcement", "abuseReportLifecycle"],
    )
    require_artifact_kind(failures, data, artifact_ids, ["providerRuntimeConfig", "evidenceRef"], "provider-runtime-config")
    require_artifact_kind(failures, data, artifact_ids, ["microserviceDatabases", "evidenceRef"], "microservice-database-proof")
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["providerRuntimeConfig", "evidenceRef"],
        ["stripe", "alipay", "wechatpay", "providerEnv", "checkoutBaseUrls", "webhookRoutes", "webhookVerification"],
    )
    require_artifact_proofs(
        failures,
        data,
        artifact_ids,
        ["microserviceDatabases", "evidenceRef"],
        ["relay", "chat", "workflow", "rag", "agent", "billing", "marketplace", "admin", "channel", "task", "observability", "migrationReadiness"],
    )
    validate_artifact_bodies(
        failures,
        data,
        artifact_ids,
        artifact_indexes,
        artifact_dir,
        allow_file_collection_source,
        allow_local_collection_source,
        target_base_hostname,
    )
    workflow_telemetry_ref = dig_path(data, ["workflowTelemetry", "evidenceRef"])
    if workflow_telemetry_window_end and isinstance(workflow_telemetry_ref, str) and not placeholder(workflow_telemetry_ref):
        workflow_telemetry_artifact_recorded_at = artifact_recorded_times.get(workflow_telemetry_ref)
        if workflow_telemetry_artifact_recorded_at and workflow_telemetry_artifact_recorded_at < workflow_telemetry_window_end:
            failures.append("workflowTelemetry.evidenceRef artifact recordedAt must be at or after workflowTelemetry.window end")

    for skip_path, value in collect_skips(data):
        failures.append(f"{path_label(skip_path)} must be empty/false for final target evidence; got {repr(value)}")
    failures.extend(collect_secret_material(data))
    return failures, artifact_ids, providers if isinstance(providers, list) else [], grpc if isinstance(grpc, list) else [], success_rate


def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("--print-template", action="store_true")
    parser.add_argument("--current-commit", required=True)
    parser.add_argument("--allow-commit-mismatch", action="store_true")
    parser.add_argument("--allow-file-collection-source", action="store_true")
    parser.add_argument("--allow-local-collection-source", action="store_true")
    parser.add_argument("--allow-non-production-target", action="store_true")
    parser.add_argument("--allow-disabled-commercial-lifecycle", action="store_true")
    parser.add_argument("--artifact-dir")
    parser.add_argument("evidence_file", nargs="?")
    args = parser.parse_args()

    if args.print_template:
        print(json.dumps(build_template(args.current_commit), indent=2))
        return 0
    if not args.evidence_file:
        print("[target-release-evidence] evidence file is required", file=sys.stderr)
        return 1
    try:
        with open(args.evidence_file, "r", encoding="utf-8") as handle:
            data = json.load(handle)
    except json.JSONDecodeError as error:
        print(f"[target-release-evidence] invalid JSON: {error}", file=sys.stderr)
        return 1

    if not isinstance(data, dict):
        print("[target-release-evidence] evidence root must be a JSON object", file=sys.stderr)
        return 1

    failures, artifact_ids, providers, grpc, success_rate = validate(
        data,
        args.current_commit,
        args.allow_commit_mismatch,
        args.artifact_dir,
        args.allow_file_collection_source,
        args.allow_local_collection_source,
        args.allow_non_production_target,
        args.allow_disabled_commercial_lifecycle,
    )
    if failures:
        print("[target-release-evidence] FAIL", file=sys.stderr)
        for failure in failures:
            print(f"[target-release-evidence] - {failure}", file=sys.stderr)
        return 1

    print("[target-release-evidence] PASS target evidence manifest")
    print(f"[target-release-evidence] environment: {data.get('environment', {}).get('name')} ({data.get('environment', {}).get('class')})")
    print(f"[target-release-evidence] commit: {data.get('commit')}")
    print(f"[target-release-evidence] run: {data.get('runId')}")
    print(f"[target-release-evidence] providers: {', '.join(str(provider.get('name')) for provider in providers)}")
    print(f"[target-release-evidence] grpc services: {', '.join(str(entry.get('service')) for entry in grpc)}")
    print(f"[target-release-evidence] grpc smoke report: {dig_path(data, ['grpcSmokeReport', 'evidenceRef'])}")
    print(f"[target-release-evidence] workflow success rate: {success_rate}")
    print(f"[target-release-evidence] request log backend: {dig_path(data, ['requestLogObservability', 'backend'])}")
    print(f"[target-release-evidence] marketplace payout mode: {dig_path(data, ['marketplacePayouts', 'providerMode'])}")
    print(f"[target-release-evidence] marketplace governance evidence: {dig_path(data, ['marketplaceGovernance', 'evidenceRef'])}")
    print(f"[target-release-evidence] microservice database mode: {dig_path(data, ['microserviceDatabases', 'mode'])}")
    print(f"[target-release-evidence] artifacts: {len(artifact_ids)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
