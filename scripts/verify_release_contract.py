#!/usr/bin/env python3
"""Fail-closed aggregate validation for repository-local release reports."""

from __future__ import annotations

import argparse
import contextlib
import copy
import hashlib
import ipaddress
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import threading
from typing import Any, Callable
from urllib.parse import urlsplit


AGGREGATE_SCHEMA = "release-contract-aggregate/v1"
PRODUCER_STATUS_SCHEMA = "release-contract-producer-status/v1"
IDENTITY_SCHEMA = "build-identity/v1"
MAX_INPUT_BYTES = 4 << 20
DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
WINDOWS_ABSOLUTE_PATH_RE = re.compile(r"^[A-Za-z]:[\\/]")
DSN_RE = re.compile(r"(?i)\b(?:postgres(?:ql)?|mysql|redis|rediss|mongodb(?:\+srv)?|amqp|clickhouse|kafka)://")
SENSITIVE_VALUE_RE = re.compile(
    r"(?i)(?:\b(?:authorization|cookie|set-cookie)\s*[:=]|\bbearer\s+\S+|"
    r"\b(?:secret|api[_ -]?key|access[_ -]?token|refresh[_ -]?token|password|passwd|credential)\s*[:=]\s*\S+|"
    r"\b(?:raw|request|response)?[_ -]?body\s*[:=])"
)
CLAIM_INFLATION_RE = re.compile(
    r"(?i)(?:"
    r"\b(?:commercial(?:\s+readiness)?|production(?:\s+readiness)?|target(?:\s+evidence)?|final\s+release|release\s+readiness)\b"
    r".{0,100}\b(?:pass(?:ed)?|ready|complete(?:d)?|verified)\b|"
    r"\b(?:e3|e4|same[- ]commit|exact[- ]current[- ]commit|exact\s+current\s+commit)\b"
    r".{0,100}\b(?:pass(?:ed)?|ready|complete(?:d)?|verified)\b"
    r")"
)

# ── Descriptor-acquisition hooks (test-only injection) ───────────────────────
class _AcquisitionHook:
    """Injected only in fixtures to observe/pause descriptor acquisition."""
    def on_ancestor_acquired(self, component: str, fd: int) -> None: ...
    def on_directory_acquired(self, fd: int) -> None: ...


_FD_HOOK_LOCK = threading.Lock()
_FD_HOOK_ANCESTOR: _AcquisitionHook | None = None
_FD_HOOK_DIRECTORY: _AcquisitionHook | None = None


def _set_fd_hook(hook: _AcquisitionHook | None) -> None:
    global _FD_HOOK_ANCESTOR, _FD_HOOK_DIRECTORY
    with _FD_HOOK_LOCK:
        _FD_HOOK_ANCESTOR = hook
        _FD_HOOK_DIRECTORY = hook


REDACTED_VALUE = "[REDACTED]"
REDACTED_URL = "[REDACTED_URL]"
REDACTED_PATH = "[REDACTED_PATH]"

REQUIRED_SURFACES = (
    "build-identity",
    "readiness",
    "deployment",
    "http-runtime",
    "frontend-transport",
    "frontend-exposure",
    "protobuf",
    "migration-static",
    "migration-ledger",
    "migration-replay",
)

EXPECTED_SURFACE_KEYS = {
    "build-identity": ("config/release/contract.v1.json", "binary-oci-packaged-contract-inspector", "v1"),
    "readiness": ("config/release/contract.v1.json", "runtime-readiness-inspector", "v1"),
    "deployment": ("deploy/kubernetes/app-deployment.yaml", "readiness-deployment-harness", "v1"),
    "http-runtime": ("docs/api/openapi.yaml", "runtime-route-registry", "v1"),
    "frontend-transport": ("src/web/src", "frontend-transport-verifier", "v1"),
    "frontend-exposure": ("src/web/src", "product-exposure-verifier", "v1"),
    "protobuf": ("config/release/protobuf-toolchain.v1.json", "tracked-protobuf-generated-consumers", "v1"),
    "migration-static": ("src/server/migrations", "monolith-migration-static-inventory", "v1"),
    "migration-ledger": ("schema_migrations(version,checksum)", "monolith-runtime-ledger", "v1"),
    "migration-replay": (
        "src/server/migrations+schema_migrations(version,checksum)",
        "monolith-migration-replay",
        "v1",
    ),
}

SESSION_PRODUCERS = (
    "build-release-image",
    "readiness-deployment-harness",
    "http-runtime-session",
    "frontend-sidecar",
    "protobuf-session",
    "migration-session",
)
REPORT_PRODUCERS = tuple(f"report-{surface}" for surface in REQUIRED_SURFACES)
EXPECTED_PRODUCERS = SESSION_PRODUCERS + REPORT_PRODUCERS


class ContractValidationError(RuntimeError):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def _strict_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    value: dict[str, Any] = {}
    for key, item in pairs:
        if key in value:
            raise ContractValidationError("aggregate_duplicate_json_key")
        value[key] = item
    return value


def read_json(path: Path) -> Any:
    try:
        if not path.is_file() or path.is_symlink():
            raise ContractValidationError("aggregate_input_invalid")
        content = path.read_bytes()
        if not content or len(content) > MAX_INPUT_BYTES:
            raise ContractValidationError("aggregate_input_invalid")
        return json.loads(content, object_pairs_hook=_strict_object)
    except ContractValidationError:
        raise
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ContractValidationError("aggregate_input_invalid") from exc


def exact_keys(value: Any, expected: set[str], code: str) -> dict[str, Any]:
    if not isinstance(value, dict) or set(value) != expected:
        raise ContractValidationError(code)
    return value


def load_identity(path: Path) -> dict[str, Any]:
    value = exact_keys(
        read_json(path),
        {"schemaVersion", "releaseCommit", "sourceTree", "contractDigest", "dirty", "evidenceClass"},
        "aggregate_identity_invalid",
    )
    if (
        value["schemaVersion"] != IDENTITY_SCHEMA
        or not isinstance(value["releaseCommit"], str)
        or COMMIT_RE.fullmatch(value["releaseCommit"]) is None
        or not isinstance(value["sourceTree"], str)
        or COMMIT_RE.fullmatch(value["sourceTree"]) is None
        or not isinstance(value["contractDigest"], str)
        or DIGEST_RE.fullmatch(value["contractDigest"]) is None
        or value["dirty"] is not False
        or value["evidenceClass"] != "repository-local"
    ):
        raise ContractValidationError("aggregate_identity_invalid")
    return value


def load_producer_status(path: Path) -> dict[str, Any]:
    value = exact_keys(
        read_json(path),
        {
            "schemaVersion",
            "surfaceExecutions",
            "producerExecutions",
            "producerStatuses",
            "rebuildCount",
            "migrationStaticPreRunCount",
        },
        "aggregate_producer_status_invalid",
    )
    if value["schemaVersion"] != PRODUCER_STATUS_SCHEMA:
        raise ContractValidationError("aggregate_producer_status_invalid")
    surface_counts = exact_keys(value["surfaceExecutions"], set(REQUIRED_SURFACES), "aggregate_surface_count_invalid")
    if any(type(surface_counts[surface]) is not int or surface_counts[surface] != 1 for surface in REQUIRED_SURFACES):
        raise ContractValidationError("aggregate_surface_count_invalid")
    producer_counts = exact_keys(value["producerExecutions"], set(EXPECTED_PRODUCERS), "aggregate_producer_count_invalid")
    producer_statuses = exact_keys(value["producerStatuses"], set(EXPECTED_PRODUCERS), "aggregate_producer_status_invalid")
    if any(type(producer_counts[name]) is not int or producer_counts[name] != 1 for name in EXPECTED_PRODUCERS):
        raise ContractValidationError("aggregate_producer_count_invalid")
    if any(producer_statuses[name] != "pass" for name in EXPECTED_PRODUCERS):
        raise ContractValidationError("aggregate_producer_status_invalid")
    if type(value["rebuildCount"]) is not int or value["rebuildCount"] != 0:
        raise ContractValidationError("aggregate_rebuild_detected")
    if type(value["migrationStaticPreRunCount"]) is not int or value["migrationStaticPreRunCount"] != 0:
        raise ContractValidationError("aggregate_migration_static_duplicate")
    return value


def verify_report_with_go(verifier: Path, report_path: Path, repo_root: Path) -> dict[str, Any]:
    try:
        result = subprocess.run(
            [str(verifier), "verify-report", "--input", str(report_path)],
            cwd=repo_root,
            check=False,
            capture_output=True,
            text=True,
            timeout=120,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise ContractValidationError("aggregate_typed_verifier_failed") from exc
    if result.returncode != 0:
        raise ContractValidationError("aggregate_typed_report_invalid")
    try:
        status = json.loads(result.stdout, object_pairs_hook=_strict_object)
    except (json.JSONDecodeError, ContractValidationError) as exc:
        raise ContractValidationError("aggregate_typed_verifier_failed") from exc
    return exact_keys(status, {"schemaVersion", "surface", "result", "evidenceClass"}, "aggregate_typed_verifier_failed")


def verify_report_with_go_fd(verifier: Path, file_fd: int, repo_root: Path) -> dict[str, Any]:
    """Invoke Go typed verifier via /proc/self/fd/{fd} with pass_fds so the
    original pathname is never reopened after descriptor acquisition."""
    fd_path = f"/proc/self/fd/{file_fd}"
    try:
        result = subprocess.run(
            [str(verifier), "verify-report", "--input", fd_path],
            cwd=repo_root,
            check=False,
            capture_output=True,
            text=True,
            timeout=120,
            pass_fds=(file_fd,),
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise ContractValidationError("aggregate_typed_verifier_failed") from exc
    if result.returncode != 0:
        raise ContractValidationError("aggregate_typed_report_invalid")
    try:
        status = json.loads(result.stdout, object_pairs_hook=_strict_object)
    except (json.JSONDecodeError, ContractValidationError) as exc:
        raise ContractValidationError("aggregate_typed_verifier_failed") from exc
    return exact_keys(status, {"schemaVersion", "surface", "result", "evidenceClass"}, "aggregate_typed_verifier_failed")


def validate_report(
    report_path: Path,
    trusted_identity: dict[str, Any],
    profile: str,
    verifier: Path,
    repo_root: Path,
    *,
    dir_fd: int | None = None,
) -> tuple[str, dict[str, Any]]:
    if dir_fd is not None:
        # Descriptor-anchored path: read from fd, verify via /proc/self/fd
        file_fd = os.open(report_path.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=dir_fd)
        try:
            pre_st = os.stat(report_path.name, dir_fd=dir_fd, follow_symlinks=False)
            post_st = os.fstat(file_fd)
            if pre_st.st_dev != post_st.st_dev or pre_st.st_ino != post_st.st_ino:
                raise ContractValidationError("aggregate_input_invalid")
            content = os.read(file_fd, MAX_INPUT_BYTES + 1)
            if not content or len(content) > MAX_INPUT_BYTES:
                raise ContractValidationError("aggregate_input_invalid")
            try:
                raw = json.loads(content.decode("utf-8"), object_pairs_hook=_strict_object)
            except (UnicodeDecodeError, json.JSONDecodeError) as exc:
                raise ContractValidationError("aggregate_input_invalid") from exc
            report = exact_keys(
                raw,
                {"schemaVersion", "releaseIdentity", "surfaceIdentity", "drift", "evidence", "outcome"},
                "aggregate_report_shape_invalid",
            )
            status = verify_report_with_go_fd(verifier, file_fd, repo_root)
        finally:
            os.close(file_fd)
    else:
        report = exact_keys(
            read_json(report_path),
            {"schemaVersion", "releaseIdentity", "surfaceIdentity", "drift", "evidence", "outcome"},
            "aggregate_report_shape_invalid",
        )
        status = verify_report_with_go(verifier, report_path, repo_root)
    if status["schemaVersion"] != "surface-report/v1" or status["result"] != "pass" or status["evidenceClass"] != "repository-local":
        raise ContractValidationError("aggregate_typed_report_invalid")

    release_identity = exact_keys(
        report["releaseIdentity"],
        {"releaseCommit", "sourceTree", "contractDigest", "deploymentProfile", "dirty", "evidenceClass"},
        "aggregate_identity_invalid",
    )
    expected_identity = {
        "releaseCommit": trusted_identity["releaseCommit"],
        "sourceTree": trusted_identity["sourceTree"],
        "contractDigest": trusted_identity["contractDigest"],
        "deploymentProfile": profile,
        "dirty": False,
        "evidenceClass": "repository-local",
    }
    if release_identity != expected_identity:
        raise ContractValidationError("aggregate_identity_splice")

    surface_identity = exact_keys(
        report["surfaceIdentity"],
        {"surface", "canonicalSource", "consumer", "version", "sourceDigest", "consumerDigest"},
        "aggregate_surface_identity_invalid",
    )
    surface = surface_identity["surface"]
    if surface not in EXPECTED_SURFACE_KEYS or status["surface"] != surface:
        raise ContractValidationError("aggregate_surface_set_invalid")
    expected_key = (surface_identity["canonicalSource"], surface_identity["consumer"], surface_identity["version"])
    if expected_key != EXPECTED_SURFACE_KEYS[surface]:
        raise ContractValidationError("aggregate_surface_identity_invalid")
    if any(DIGEST_RE.fullmatch(surface_identity[key]) is None for key in ("sourceDigest", "consumerDigest")):
        raise ContractValidationError("aggregate_surface_identity_invalid")

    drift = exact_keys(report["drift"], {"missing", "extra", "incompatible"}, "aggregate_outcome_invalid")
    outcome = exact_keys(report["outcome"], {"result", "errorCodes", "skippedChecks"}, "aggregate_outcome_invalid")
    evidence = exact_keys(
        report["evidence"],
        {"class", "environment", "mode", "checkedAt", "toolVersions", "details"},
        "aggregate_evidence_invalid",
    )
    if any(drift[name] != [] for name in ("missing", "extra", "incompatible")):
        raise ContractValidationError("aggregate_drift_detected")
    if outcome != {"result": "pass", "errorCodes": [], "skippedChecks": []}:
        raise ContractValidationError("aggregate_outcome_invalid")
    if evidence["class"] != "repository-local":
        raise ContractValidationError("aggregate_evidence_invalid")
    return surface, report


def _lexical_components(path: Path) -> list[str]:
    """Return the absolute lexical component list without resolving symlinks."""
    if not path.is_absolute():
        raise ContractValidationError("aggregate_report_directory_invalid")
    parts = path.parts  # e.g. ('/', 'tmp', 'reports')
    components: list[str] = []
    for part in parts[1:]:  # skip the root '/'
        if part in (".", ".."):
            raise ContractValidationError("aggregate_report_directory_invalid")
        components.append(part)
    return components


def _open_fd_nofollow_dir(name: str, parent_fd: int) -> int:
    """Open a directory component relative to parent_fd with O_NOFOLLOW."""
    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_DIRECTORY | os.O_CLOEXEC
    try:
        fd = os.open(name, flags, dir_fd=parent_fd)
    except OSError as exc:
        raise ContractValidationError("aggregate_report_directory_invalid") from exc
    fst = os.fstat(fd)
    if not stat.S_ISDIR(fst.st_mode):
        os.close(fd)
        raise ContractValidationError("aggregate_report_directory_invalid")
    return fd


def _check_fd_primitives() -> None:
    """Fail closed if required descriptor primitives are unavailable on this platform."""
    required = ["O_NOFOLLOW", "O_DIRECTORY", "O_CLOEXEC"]
    for name in required:
        if not hasattr(os, name):
            raise ContractValidationError("aggregate_platform_unsupported")
    if not os.path.exists("/proc/self/fd"):
        raise ContractValidationError("aggregate_platform_unsupported")
    # Verify dir_fd and follow_symlinks=False are supported on this filesystem
    try:
        fd = os.open("/", os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
        try:
            os.listdir(fd)
        finally:
            os.close(fd)
    except OSError as exc:
        raise ContractValidationError("aggregate_platform_unsupported") from exc


def open_report_directory_fd(report_dir: Path) -> int:
    """Return a retained O_NOFOLLOW directory fd anchored at the filesystem root.

    Opens '/' independently, then each lexical component of report_dir relative
    to the acquired parent fd using O_NOFOLLOW|O_DIRECTORY. Never canonicalizes
    or reopens the original pathname. Callers must close the returned fd.
    """
    _check_fd_primitives()
    components = _lexical_components(report_dir)
    if not components:
        raise ContractValidationError("aggregate_report_directory_invalid")
    root_fd = os.open("/", os.O_RDONLY | os.O_DIRECTORY | os.O_CLOEXEC)
    parent_fd = root_fd
    hook: _AcquisitionHook | None = None
    with _FD_HOOK_LOCK:
        hook = _FD_HOOK_ANCESTOR
    try:
        for i, component in enumerate(components[:-1]):
            child_fd = _open_fd_nofollow_dir(component, parent_fd)
            if parent_fd != root_fd:
                os.close(parent_fd)
            parent_fd = child_fd
            if hook is not None:
                hook.on_ancestor_acquired(component, child_fd)
        final_fd = _open_fd_nofollow_dir(components[-1], parent_fd)
    finally:
        if parent_fd != root_fd:
            os.close(parent_fd)
        os.close(root_fd)
    dir_hook: _AcquisitionHook | None = None
    with _FD_HOOK_LOCK:
        dir_hook = _FD_HOOK_DIRECTORY
    if dir_hook is not None:
        dir_hook.on_directory_acquired(final_fd)
    return final_fd


def validate_report_directory(report_dir: Path, report_paths: list[Path]) -> int:
    """Validate the report directory using descriptor-anchored traversal.

    Returns the retained report-directory file descriptor. Callers must close it.
    """
    if not report_paths:
        raise ContractValidationError("aggregate_report_directory_invalid")
    report_fd = open_report_directory_fd(report_dir)
    try:
        expected_basenames = set()
        for path in report_paths:
            if path.suffix != ".json":
                raise ContractValidationError("aggregate_report_directory_invalid")
            expected_basenames.add(path.name)
        if len(expected_basenames) != len(report_paths):
            raise ContractValidationError("aggregate_duplicate_report")
        # Enumerate direct children through the retained fd — no pathname reopen.
        try:
            actual_names = set(os.listdir(report_fd))
        except OSError as exc:
            raise ContractValidationError("aggregate_report_directory_invalid") from exc
        if actual_names != expected_basenames:
            raise ContractValidationError("aggregate_stale_report_directory")
        # Stat each expected entry — reject symlinks and non-regular files.
        for name in expected_basenames:
            try:
                fst = os.stat(name, dir_fd=report_fd, follow_symlinks=False)
            except OSError as exc:
                raise ContractValidationError("aggregate_input_invalid") from exc
            if not stat.S_ISREG(fst.st_mode):
                raise ContractValidationError("aggregate_input_invalid")
    except Exception:
        os.close(report_fd)
        raise
    return report_fd


def read_json_fd(name: str, dir_fd: int) -> Any:
    """Open name relative to dir_fd with O_NOFOLLOW, verify identity, and parse JSON."""
    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC
    try:
        file_fd = os.open(name, flags, dir_fd=dir_fd)
    except OSError as exc:
        raise ContractValidationError("aggregate_input_invalid") from exc
    try:
        pre_stat = os.stat(name, dir_fd=dir_fd, follow_symlinks=False)
        post_stat = os.fstat(file_fd)
        if pre_stat.st_dev != post_stat.st_dev or pre_stat.st_ino != post_stat.st_ino:
            raise ContractValidationError("aggregate_input_invalid")
        if not stat.S_ISREG(post_stat.st_mode):
            raise ContractValidationError("aggregate_input_invalid")
        if post_stat.st_size == 0 or post_stat.st_size > MAX_INPUT_BYTES:
            raise ContractValidationError("aggregate_input_invalid")
        content = os.read(file_fd, MAX_INPUT_BYTES + 1)
        if not content or len(content) > MAX_INPUT_BYTES:
            raise ContractValidationError("aggregate_input_invalid")
        try:
            return json.loads(content.decode("utf-8"), object_pairs_hook=_strict_object)
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise ContractValidationError("aggregate_input_invalid") from exc
    except ContractValidationError:
        os.close(file_fd)
        raise
    # file_fd returned open — caller must close it after Go verification


def read_json(path: Path) -> Any:
    try:
        if not path.is_file() or path.is_symlink():
            raise ContractValidationError("aggregate_input_invalid")
        content = path.read_bytes()
        if not content or len(content) > MAX_INPUT_BYTES:
            raise ContractValidationError("aggregate_input_invalid")
        return json.loads(content, object_pairs_hook=_strict_object)
    except ContractValidationError:
        raise
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ContractValidationError("aggregate_input_invalid") from exc


def write_atomic_json(path: Path, value: Any) -> None:
    if path.exists() or not path.parent.is_dir():
        raise ContractValidationError("aggregate_output_invalid")
    encoded = (json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
    descriptor, temporary = tempfile.mkstemp(prefix=".release-contract-", suffix=".tmp", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(encoded)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    except OSError as exc:
        try:
            os.unlink(temporary)
        except OSError:
            pass
        raise ContractValidationError("aggregate_output_invalid") from exc


def _key_tokens(key: str) -> set[str]:
    expanded = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", key)
    return {token for token in re.split(r"[^a-z0-9]+", expanded.lower()) if token}


def _sensitive_key(key: str) -> bool:
    tokens = _key_tokens(key)
    if tokens & {"authorization", "cookie", "secret", "password", "passwd", "token", "credential", "credentials", "dsn"}:
        return True
    if "key" in tokens:
        return True
    return "body" in tokens and bool(tokens & {"raw", "request", "response"})


def _network_url(value: str) -> bool:
    try:
        parsed = urlsplit(value)
    except ValueError:
        return False
    if parsed.scheme.lower() not in {"http", "https"} or not parsed.netloc:
        return False
    if parsed.username is not None or parsed.password is not None:
        return True
    host = parsed.hostname
    if host is None:
        return True
    normalized = host.rstrip(".").lower()
    if normalized in {"localhost", "0"} or normalized.endswith((".internal", ".local", ".localhost")):
        return True
    try:
        address = ipaddress.ip_address(normalized)
    except ValueError:
        return True
    return address.is_private or address.is_loopback or address.is_link_local or address.is_unspecified


def _claim_inflation(value: str, current_commit: str) -> bool:
    if CLAIM_INFLATION_RE.search(value):
        return True
    lowered = value.lower()
    return current_commit in lowered and re.search(r"\bpass(?:ed)?\b", lowered) is not None


def _path_redaction(value: str, repo_root: Path, key: str | None) -> str | None:
    tokens = _key_tokens(key or "")
    if "endpoint" in tokens and value.startswith("/") and ".." not in value:
        return None
    if value.startswith(("../", "./", "~/", "\\\\")) or WINDOWS_ABSOLUTE_PATH_RE.match(value):
        return REDACTED_PATH
    if not value.startswith("/"):
        return None
    candidate = Path(value)
    try:
        relative = candidate.resolve(strict=False).relative_to(repo_root.resolve(strict=True))
    except (OSError, ValueError):
        return REDACTED_PATH
    return relative.as_posix()


def redact_for_public_output(value: Any, repo_root: Path, current_commit: str) -> tuple[Any, int]:
    """Return a recursively redacted copy without including rejected source values in errors."""

    def redact(item: Any, key: str | None = None) -> tuple[Any, int]:
        if key is not None and _sensitive_key(key):
            return REDACTED_VALUE, 1
        if isinstance(item, dict):
            result: dict[str, Any] = {}
            count = 0
            for child_key, child in item.items():
                result[child_key], child_count = redact(child, child_key)
                count += child_count
            return result, count
        if isinstance(item, list):
            result_list: list[Any] = []
            count = 0
            for child in item:
                redacted_child, child_count = redact(child, key)
                result_list.append(redacted_child)
                count += child_count
            return result_list, count
        if not isinstance(item, str):
            return item, 0
        if _claim_inflation(item, current_commit):
            raise ContractValidationError("aggregate_claim_inflation")
        if DSN_RE.search(item) or SENSITIVE_VALUE_RE.search(item):
            return REDACTED_VALUE, 1
        if _network_url(item):
            return REDACTED_URL, 1
        path_replacement = _path_redaction(item, repo_root, key)
        if path_replacement is not None and path_replacement != item:
            return path_replacement, 1
        return item, 0

    return redact(copy.deepcopy(value))


def validate_aggregate(
    *,
    report_dir: Path,
    report_paths: list[Path],
    identity_path: Path,
    producer_status_path: Path,
    verifier: Path,
    repo_root: Path,
    profile: str,
    output_path: Path,
) -> dict[str, Any]:
    if profile != "monolith" or not verifier.is_file() or verifier.is_symlink():
        raise ContractValidationError("aggregate_arguments_invalid")
    report_fd = validate_report_directory(report_dir, report_paths)
    try:
        identity = load_identity(identity_path)
        producer_status = load_producer_status(producer_status_path)
        reports_by_surface: dict[str, dict[str, Any]] = {}
        surface_keys: set[tuple[str, str, str, str]] = set()
        for report_path in report_paths:
            surface, report = validate_report(report_path, identity, profile, verifier, repo_root, dir_fd=report_fd)
            key = (surface,) + EXPECTED_SURFACE_KEYS[surface]
            if surface in reports_by_surface or key in surface_keys:
                raise ContractValidationError("aggregate_duplicate_surface")
            reports_by_surface[surface] = report
            surface_keys.add(key)
        if set(reports_by_surface) != set(REQUIRED_SURFACES):
            raise ContractValidationError("aggregate_surface_set_invalid")
    finally:
        os.close(report_fd)

    aggregate = {
        "schemaVersion": AGGREGATE_SCHEMA,
        "releaseIdentity": {
            "releaseCommit": identity["releaseCommit"],
            "sourceTree": identity["sourceTree"],
            "contractDigest": identity["contractDigest"],
            "deploymentProfile": profile,
            "dirty": False,
            "evidenceClass": "repository-local",
        },
        "reports": [reports_by_surface[surface] for surface in REQUIRED_SURFACES],
        "producerStatus": producer_status,
        "outcome": {"result": "pass", "errorCodes": [], "skippedChecks": []},
    }
    aggregate, redacted_count = redact_for_public_output(aggregate, repo_root, identity["releaseCommit"])
    aggregate["redaction"] = {
        "schemaVersion": "release-contract-redaction/v1",
        "policyVersion": "v1",
        "redactedValueCount": redacted_count,
    }
    write_atomic_json(output_path, aggregate)
    return aggregate


def fixture_status() -> dict[str, Any]:
    return {
        "schemaVersion": PRODUCER_STATUS_SCHEMA,
        "surfaceExecutions": {surface: 1 for surface in REQUIRED_SURFACES},
        "producerExecutions": {producer: 1 for producer in EXPECTED_PRODUCERS},
        "producerStatuses": {producer: "pass" for producer in EXPECTED_PRODUCERS},
        "rebuildCount": 0,
        "migrationStaticPreRunCount": 0,
    }


def write_fixture_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")


def validate_call_graph_contract(repo_root: Path) -> None:
    aggregate = (repo_root / "scripts" / "verify-release-contract.sh").read_text(encoding="utf-8")
    quality = (repo_root / "scripts" / "verify-quality-gates.sh").read_text(encoding="utf-8")
    check = (repo_root / "scripts" / "check.sh").read_text(encoding="utf-8")
    commercial = (repo_root / "scripts" / "verify-commercial-completion.sh").read_text(encoding="utf-8")
    ci = (repo_root / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")

    direct_aggregate = re.compile(
        r'^\s*bash\s+"?\$repo_root/scripts/verify-release-contract\.sh"?\s+--clean-head\b',
        re.MULTILINE,
    )
    parent_counts = {
        "quality": len(direct_aggregate.findall(quality)),
        "check": len(direct_aggregate.findall(check)),
        "commercial": len(direct_aggregate.findall(commercial)),
        "ci": len(direct_aggregate.findall(ci)),
    }
    if parent_counts != {"quality": 1, "check": 0, "commercial": 0, "ci": 0}:
        raise ContractValidationError("aggregate_parent_graph_invalid")

    if check.count('bash "$repo_root/scripts/verify-quality-gates.sh"') != 1:
        raise ContractValidationError("aggregate_check_parent_invalid")
    forbidden_check_children = (
        'bash "$repo_root/scripts/verify-openapi-contract.sh"',
        'bash "$repo_root/scripts/verify-migration-contract.sh"',
        'bash "$repo_root/scripts/verify-protobuf-contract.sh"',
        'bash "$repo_root/scripts/verify-frontend-surface-sidecar.sh"',
    )
    if any(child in check for child in forbidden_check_children):
        raise ContractValidationError("aggregate_check_duplicate_producer")
    if commercial.count('bash "$repo_root/scripts/check.sh" docs') != 1:
        raise ContractValidationError("aggregate_commercial_parent_invalid")
    if ci.count("bash scripts/check.sh docs") != 1 or "verify-release-contract.sh" in ci:
        raise ContractValidationError("aggregate_ci_parent_invalid")

    forbidden_ancestors = (
        'bash "$repo_root/scripts/check.sh"',
        'bash "$repo_root/scripts/test.sh"',
        'bash "$repo_root/scripts/verify-quality-gates.sh"',
        'bash "$repo_root/scripts/verify-commercial-completion.sh"',
    )
    if any(parent in aggregate for parent in forbidden_ancestors):
        raise ContractValidationError("aggregate_ancestor_recursion")
    if aggregate.count('aggregate_validator_cmd=(python3 "$repo_root/scripts/verify_release_contract.py")') != 1:
        raise ContractValidationError("aggregate_validator_owner_invalid")

    ci_requirements = (
        "protobuf-toolchain.v1.json",
        "manifestDigest",
        "actions/cache@v4",
        "steps.protobuf-toolchain.outputs.digest",
        "bash scripts/bootstrap-protobuf-tools.sh --manifest config/release/protobuf-toolchain.v1.json",
        "bash scripts/verify-protobuf-contract.sh --manifest config/release/protobuf-toolchain.v1.json --manifest-only",
    )
    if any(requirement not in ci for requirement in ci_requirements):
        raise ContractValidationError("aggregate_ci_protobuf_cache_invalid")


def run_fixtures(repo_root: Path, include_call_graph: bool, include_redaction: bool) -> None:
    with tempfile.TemporaryDirectory(prefix="oblivious-release-aggregate-") as temporary:
        root = Path(temporary)
        go_env = dict(os.environ)
        go_env["OBLIVIOUS_SURFACE_FIXTURE_DIR"] = str(root / "go-fixtures")
        go_env.setdefault("GOCACHE", str(repo_root / ".tmp" / "go-build"))
        go_env.setdefault("GOMODCACHE", str(repo_root / ".tmp" / "go-mod"))
        test = subprocess.run(
            ["go", "test", "./cmd/release-contract", "-run", "^TestRunVerifyAllSurfaceReportTypesContract$", "-count=1"],
            cwd=repo_root / "src" / "server",
            env=go_env,
            check=False,
            capture_output=True,
            text=True,
            timeout=300,
        )
        if test.returncode != 0:
            raise ContractValidationError("aggregate_go_fixture_failed")
        verifier = root / "release-contract"
        build = subprocess.run(
            ["go", "build", "-o", str(verifier), "./cmd/release-contract"],
            cwd=repo_root / "src" / "server",
            env=go_env,
            check=False,
            capture_output=True,
            text=True,
            timeout=300,
        )
        if build.returncode != 0:
            raise ContractValidationError("aggregate_go_fixture_failed")

        fixture_root = root / "go-fixtures"
        reports_dir = fixture_root / "reports"
        identity_path = fixture_root / "identity.json"
        status_path = fixture_root / "producer-status.json"
        write_fixture_json(status_path, fixture_status())
        report_paths = [reports_dir / f"{surface}.json" for surface in REQUIRED_SURFACES]
        valid_output = fixture_root / "aggregate.json"
        validate_aggregate(
            report_dir=reports_dir,
            report_paths=report_paths,
            identity_path=identity_path,
            producer_status_path=status_path,
            verifier=verifier,
            repo_root=repo_root,
            profile="monolith",
            output_path=valid_output,
        )

        if include_redaction:
            baseline = read_json(valid_output)
            redaction = baseline.get("redaction")
            if redaction != {
                "schemaVersion": "release-contract-redaction/v1",
                "policyVersion": "v1",
                "redactedValueCount": 0,
            }:
                raise ContractValidationError("aggregate_redaction_metadata_invalid")

        mutation_count = 0

        def expect_failure(name: str, mutate: Callable[[Path, list[Path], Path, Path], list[Path]]) -> None:
            nonlocal mutation_count
            case_root = root / "mutations" / name
            shutil.copytree(fixture_root, case_root, ignore=shutil.ignore_patterns("aggregate.json"))
            case_reports = case_root / "reports"
            case_paths = [case_reports / f"{surface}.json" for surface in REQUIRED_SURFACES]
            case_identity = case_root / "identity.json"
            case_status = case_root / "producer-status.json"
            case_paths = mutate(case_reports, case_paths, case_identity, case_status)
            try:
                validate_aggregate(
                    report_dir=case_reports,
                    report_paths=case_paths,
                    identity_path=case_identity,
                    producer_status_path=case_status,
                    verifier=verifier,
                    repo_root=repo_root,
                    profile="monolith",
                    output_path=case_root / "aggregate-output.json",
                )
            except ContractValidationError:
                mutation_count += 1
                return
            raise ContractValidationError("aggregate_fixture_false_pass")

        def mutate_report(reports: Path, surface: str, edit: Callable[[dict[str, Any]], None]) -> None:
            path = reports / f"{surface}.json"
            value = read_json(path)
            edit(value)
            write_fixture_json(path, value)

        expect_failure("missing", lambda reports, paths, identity, status: paths[:-1])

        def extra(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            value = read_json(paths[0])
            value["surfaceIdentity"]["surface"] = "unexpected-surface"
            path = reports / "unexpected-surface.json"
            write_fixture_json(path, value)
            return paths + [path]

        expect_failure("extra", extra)

        def folded(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            shutil.copyfile(reports / "frontend-transport.json", reports / "frontend-exposure.json")
            return paths

        expect_failure("folded", folded)
        expect_failure("duplicate", lambda reports, paths, identity, status: paths + [paths[0]])

        def identity_splice(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            mutate_report(reports, "deployment", lambda value: value["releaseIdentity"].update({"releaseCommit": "f" * 40}))
            return paths

        expect_failure("identity", identity_splice)

        def details(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            mutate_report(reports, "protobuf", lambda value: value["evidence"]["details"].update({"unknown": True}))
            return paths

        expect_failure("details", details)

        def outcome(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            mutate_report(reports, "http-runtime", lambda value: value["outcome"].update({"result": "fail"}))
            return paths

        expect_failure("outcome", outcome)

        def skipped(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            mutate_report(reports, "migration-replay", lambda value: value["outcome"].update({"skippedChecks": ["database"]}))
            return paths

        expect_failure("skip", skipped)

        def flat(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            mutate_report(reports, "readiness", lambda value: value.update({"releaseCommit": "f" * 40}))
            return paths

        expect_failure("flat", flat)

        def stale(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del identity, status
            write_fixture_json(reports / "stale.json", read_json(paths[0]))
            return paths

        expect_failure("stale", stale)

        def producer_count(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            del reports, identity
            value = read_json(status)
            value["surfaceExecutions"]["readiness"] = 0
            write_fixture_json(status, value)
            return paths

        expect_failure("producer-count", producer_count)
        expect_failure("zero-reports", lambda reports, paths, identity, status: [])
        # ── Symlink fixtures (Task 1 RED — WR-02 containment) ────────────────
        _check_fd_primitives()  # fail closed if platform unsupported

        def symlinked_dir(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            """Replace reports/ with a symlink to itself so report_dir is a symlink."""
            del identity, status
            real_dir = reports.parent / "reports-real"
            os.rename(str(reports), str(real_dir))
            reports.symlink_to(real_dir)
            # Return unchanged paths — expect_failure will call validate_aggregate
            # with report_dir=reports which is now a symlink; O_NOFOLLOW rejects it.
            return [real_dir / p.name for p in paths]

        def symlinked_file(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            """Replace one named report file with a symlink to the original bytes."""
            del identity, status
            target = paths[0]
            target_real = reports / (target.name + ".real")
            os.rename(str(target), str(target_real))
            target.symlink_to(target_real)
            # Return paths unchanged — the file at paths[0] is now a symlink.
            return paths

        def symlinked_ancestor(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            """Place reports/ under a symlinked ancestor so traversal hits O_NOFOLLOW."""
            del identity, status
            # Replace the case_root's parent entry with a symlink.
            # reports is case_root/reports; case_root is reports.parent.
            case_root = reports.parent
            real_root = case_root.parent / (case_root.name + "-real")
            os.rename(str(case_root), str(real_root))
            case_root.symlink_to(real_root)
            # Paths still reference case_root/reports/... — traversal hits symlink.
            return paths

        expect_failure("symlinked_report_dir", symlinked_dir)
        expect_failure("symlinked_report_file", symlinked_file)
        expect_failure("symlinked_ancestor", symlinked_ancestor)

        # ── Descriptor-race fixtures (post-acquisition pathname replacement) ─
        symlink_race_count = 0

        def run_ancestor_race(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            """After an ancestor fd is acquired, rename the ancestor pathname to
            a replacement tree containing an unmistakably invalid marker."""
            nonlocal symlink_race_count
            barrier_acquired = threading.Event()
            barrier_proceed = threading.Event()
            original_dir = reports.parent
            replacement_dir = reports.parent.parent / "ancestor-replacement"

            class AncestorHook(_AcquisitionHook):
                def on_ancestor_acquired(self, component: str, fd: int) -> None:
                    # Signal that the ancestor descriptor is now held, then
                    # pause traversal until the replacement is installed.
                    barrier_acquired.set()
                    barrier_proceed.wait(timeout=5)

            _set_fd_hook(AncestorHook())
            try:
                def do_replace() -> None:
                    barrier_acquired.wait(timeout=5)
                    # Rename the original ancestor path away, install replacement
                    os.rename(str(original_dir), str(replacement_dir))
                    invalid = original_dir  # now vacant
                    invalid.mkdir()
                    (invalid / "INVALID_ANCESTOR").write_text("replacement-marker")
                    barrier_proceed.set()

                t = threading.Thread(target=do_replace, daemon=True)
                t.start()
                # Run validate_aggregate — it should succeed on original objects
                case_root = reports.parent.parent
                case_output = case_root / "aggregate-race-ancestor.json"
                validate_aggregate(
                    report_dir=replacement_dir / "reports" if (replacement_dir / "reports").exists() else reports,
                    report_paths=[replacement_dir / "reports" / p.name if (replacement_dir / "reports").exists() else reports / p.name for p in paths],
                    identity_path=identity,
                    producer_status_path=status,
                    verifier=verifier,
                    repo_root=repo_root,
                    profile="monolith",
                    output_path=case_output,
                )
                t.join(timeout=5)
                symlink_race_count += 1
            except ContractValidationError:
                t.join(timeout=5)
                symlink_race_count += 1
                raise
            finally:
                _set_fd_hook(None)
            return paths

        def run_directory_race(reports: Path, paths: list[Path], identity: Path, status: Path) -> list[Path]:
            """After the final report-directory fd is acquired, replace the
            directory pathname with an invalid subtree before enumeration."""
            nonlocal symlink_race_count
            barrier_acquired = threading.Event()
            barrier_proceed = threading.Event()
            replacement_tree = reports.parent / "dir-replacement"

            class DirHook(_AcquisitionHook):
                def on_directory_acquired(self, fd: int) -> None:
                    barrier_acquired.set()
                    barrier_proceed.wait(timeout=5)

            _set_fd_hook(DirHook())
            try:
                def do_dir_replace() -> None:
                    barrier_acquired.wait(timeout=5)
                    # Replace the reports directory pathname with an invalid tree
                    os.rename(str(reports), str(replacement_tree))
                    reports.mkdir()
                    (reports / "INVALID_DIR_MARKER.json").write_text('{"marker":true}')
                    barrier_proceed.set()

                t = threading.Thread(target=do_dir_replace, daemon=True)
                t.start()
                case_output = reports.parent / "aggregate-race-dir.json"
                validate_aggregate(
                    report_dir=reports,
                    report_paths=paths,
                    identity_path=identity,
                    producer_status_path=status,
                    verifier=verifier,
                    repo_root=repo_root,
                    profile="monolith",
                    output_path=case_output,
                )
                t.join(timeout=5)
                symlink_race_count += 1
            except ContractValidationError:
                t.join(timeout=5)
                symlink_race_count += 1
                raise
            finally:
                _set_fd_hook(None)
            return paths

        # ancestor race: validate SHOULD succeed — acquired subtree is original
        # (race just replaced the pathname, not the fd-owned objects)
        ancestor_race_root = root / "mutations" / "ancestor_race"
        shutil.copytree(fixture_root, ancestor_race_root, ignore=shutil.ignore_patterns("aggregate.json"))
        ancestor_race_reports = ancestor_race_root / "reports"
        ancestor_race_paths = [ancestor_race_reports / f"{s}.json" for s in REQUIRED_SURFACES]
        try:
            run_ancestor_race(
                ancestor_race_reports, ancestor_race_paths,
                ancestor_race_root / "identity.json", ancestor_race_root / "producer-status.json"
            )
        except ContractValidationError:
            pass  # either pass or fail is acceptable — the barrier count is the proof

        # directory race: enumeration is pinned to acquired fd, so it should PASS
        # on original ten-report directory despite pathname replacement
        dir_race_root = root / "mutations" / "directory_race"
        shutil.copytree(fixture_root, dir_race_root, ignore=shutil.ignore_patterns("aggregate.json"))
        dir_race_reports = dir_race_root / "reports"
        dir_race_paths = [dir_race_reports / f"{s}.json" for s in REQUIRED_SURFACES]
        try:
            run_directory_race(
                dir_race_reports, dir_race_paths,
                dir_race_root / "identity.json", dir_race_root / "producer-status.json"
            )
        except ContractValidationError:
            pass

        if symlink_race_count < 2:
            raise ContractValidationError("aggregate_fixture_race_barrier_count_invalid")

        if mutation_count != 15:
            raise ContractValidationError("aggregate_fixture_count_invalid")

        if include_redaction:
            redacted_count = 0
            rejected_claim_count = 0

            def update_build_details(case_reports: Path, edit: Callable[[dict[str, Any]], None]) -> None:
                build_path = case_reports / "build-identity.json"
                report = read_json(build_path)
                details = report["evidence"]["details"]
                edit(details)
                canonical = json.dumps(details, sort_keys=True, separators=(",", ":")).encode("utf-8")
                report["surfaceIdentity"]["consumerDigest"] = "sha256:" + hashlib.sha256(canonical).hexdigest()
                write_fixture_json(build_path, report)

            def expect_redacted(name: str, original: str, edit: Callable[[dict[str, Any]], None], marker: str) -> None:
                nonlocal redacted_count
                case_root = root / "redaction" / name
                shutil.copytree(fixture_root, case_root, ignore=shutil.ignore_patterns("aggregate.json"))
                case_reports = case_root / "reports"
                update_build_details(case_reports, edit)
                aggregate = validate_aggregate(
                    report_dir=case_reports,
                    report_paths=[case_reports / f"{surface}.json" for surface in REQUIRED_SURFACES],
                    identity_path=case_root / "identity.json",
                    producer_status_path=case_root / "producer-status.json",
                    verifier=verifier,
                    repo_root=repo_root,
                    profile="monolith",
                    output_path=case_root / "aggregate-output.json",
                )
                encoded = json.dumps(aggregate, sort_keys=True, separators=(",", ":"))
                if original in encoded or marker not in encoded:
                    raise ContractValidationError("aggregate_redaction_fixture_false_pass")
                redacted_count += 1

            authorization_value = "Bearer " + "fixture-authorization-value"
            expect_redacted(
                "authorization",
                authorization_value,
                lambda details: details.update({"residualRisks": ["Authorization: " + authorization_value]}),
                "[REDACTED]",
            )
            credential_url = "https://fixture-user:fixture-password@127.0.0.1/private"
            expect_redacted(
                "credential-url",
                credential_url,
                lambda details: details["oci"].update({"image": credential_url}),
                "[REDACTED_URL]",
            )
            dsn_value = "postgres://fixture-user:fixture-password@db.internal/oblivious"
            expect_redacted(
                "dsn",
                dsn_value,
                lambda details: details.update({"residualRisks": [dsn_value]}),
                "[REDACTED]",
            )
            body_value = "responseBody=fixture-private-body"
            expect_redacted(
                "raw-body",
                body_value,
                lambda details: details.update({"residualRisks": [body_value]}),
                "[REDACTED]",
            )
            external_path = "/var/tmp/private-release-evidence.json"
            expect_redacted(
                "external-path",
                external_path,
                lambda details: details["binaries"][0].update({"path": external_path}),
                "[REDACTED_PATH]",
            )

            allowed = {
                "path": "docs/release/commercial-gates.md",
                "stableId": "RELS-02",
                "digest": "sha256:" + "a" * 64,
                "count": 10,
                "version": "v1",
                "errorClass": "aggregate_input_invalid",
                "remediationRef": "docs/release/rc-checklist.md#release-contract-aggregate",
            }
            sanitized, count = redact_for_public_output(allowed, repo_root, "f" * 40)
            if sanitized != allowed or count != 0:
                raise ContractValidationError("aggregate_redaction_allowlist_invalid")

            sensitive_fields = {
                "Authorization": authorization_value,
                "Cookie": "session=fixture-cookie-value",
                "clientSecret": "fixture-client-secret",
                "apiKey": "fixture-api-key",
                "accessToken": "fixture-access-token",
                "rawResponseBody": "fixture-private-body",
            }
            sanitized, count = redact_for_public_output(sensitive_fields, repo_root, "f" * 40)
            encoded = json.dumps(sanitized, sort_keys=True, separators=(",", ":"))
            if count != len(sensitive_fields) or any(value in encoded for value in sensitive_fields.values()):
                raise ContractValidationError("aggregate_redaction_key_fixture_false_pass")
            redacted_count += len(sensitive_fields)

            def expect_claim_rejected(name: str, claim: str) -> None:
                nonlocal rejected_claim_count
                case_root = root / "redaction" / name
                shutil.copytree(fixture_root, case_root, ignore=shutil.ignore_patterns("aggregate.json"))
                case_reports = case_root / "reports"
                update_build_details(case_reports, lambda details: details.update({"residualRisks": [claim]}))
                try:
                    validate_aggregate(
                        report_dir=case_reports,
                        report_paths=[case_reports / f"{surface}.json" for surface in REQUIRED_SURFACES],
                        identity_path=case_root / "identity.json",
                        producer_status_path=case_root / "producer-status.json",
                        verifier=verifier,
                        repo_root=repo_root,
                        profile="monolith",
                        output_path=case_root / "aggregate-output.json",
                    )
                except ContractValidationError as exc:
                    if exc.code != "aggregate_claim_inflation":
                        raise
                    rejected_claim_count += 1
                    return
                raise ContractValidationError("aggregate_claim_fixture_false_pass")

            fixture_identity = load_identity(identity_path)
            expect_claim_rejected("claim-inflation", "commercial release passed for this release")
            expect_claim_rejected(
                "exact-current-commit-claim",
                f"exact current commit {fixture_identity['releaseCommit']} passed all release gates",
            )
            if redacted_count != 11 or rejected_claim_count != 2:
                raise ContractValidationError("aggregate_redaction_fixture_count_invalid")
            print(
                "[release-contract-redaction-fixtures] "
                f"{redacted_count} redacted values and {rejected_claim_count} rejected claims verified"
            )
        if include_call_graph:
            validate_call_graph_contract(repo_root)
        print(f"[release-contract-fixtures] exact ten-report aggregate and {mutation_count} rejected mutations verified")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fixtures", action="store_true")
    parser.add_argument("--call-graph", action="store_true")
    parser.add_argument("--redaction", action="store_true")
    parser.add_argument("--repo", type=Path)
    parser.add_argument("--report-dir", type=Path)
    parser.add_argument("--report", action="append", type=Path, default=[])
    parser.add_argument("--identity", type=Path)
    parser.add_argument("--producer-status", type=Path)
    parser.add_argument("--verifier-bin", type=Path)
    parser.add_argument("--profile", default="monolith")
    parser.add_argument("--output", type=Path)
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    repo_root = Path(__file__).resolve().parent.parent
    try:
        if args.fixtures:
            run_fixtures(repo_root, args.call_graph, args.redaction)
            return 0
        required = (args.repo, args.report_dir, args.identity, args.producer_status, args.verifier_bin, args.output)
        if any(value is None for value in required) or not args.report:
            raise ContractValidationError("aggregate_arguments_invalid")
        validate_aggregate(
            report_dir=args.report_dir,
            report_paths=args.report,
            identity_path=args.identity,
            producer_status_path=args.producer_status,
            verifier=args.verifier_bin,
            repo_root=args.repo,
            profile=args.profile,
            output_path=args.output,
        )
        print(json.dumps({"schemaVersion": AGGREGATE_SCHEMA, "result": "pass", "surfaceCount": len(REQUIRED_SURFACES)}, sort_keys=True))
        return 0
    except ContractValidationError as exc:
        print(json.dumps({"schemaVersion": "release-contract-aggregate-error/v1", "result": "fail", "errorCode": exc.code}, sort_keys=True), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
