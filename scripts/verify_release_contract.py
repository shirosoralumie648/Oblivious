#!/usr/bin/env python3
"""Fail-closed aggregate validation for repository-local release reports."""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from typing import Any, Callable


AGGREGATE_SCHEMA = "release-contract-aggregate/v1"
PRODUCER_STATUS_SCHEMA = "release-contract-producer-status/v1"
IDENTITY_SCHEMA = "build-identity/v1"
MAX_INPUT_BYTES = 4 << 20
DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")

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


def validate_report(
    report_path: Path,
    trusted_identity: dict[str, Any],
    profile: str,
    verifier: Path,
    repo_root: Path,
) -> tuple[str, dict[str, Any]]:
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


def validate_report_directory(report_dir: Path, report_paths: list[Path]) -> None:
    try:
        resolved_dir = report_dir.resolve(strict=True)
    except OSError as exc:
        raise ContractValidationError("aggregate_report_directory_invalid") from exc
    if not resolved_dir.is_dir() or resolved_dir.is_symlink() or not report_paths:
        raise ContractValidationError("aggregate_report_directory_invalid")
    resolved_paths: list[Path] = []
    for path in report_paths:
        try:
            resolved = path.resolve(strict=True)
        except OSError as exc:
            raise ContractValidationError("aggregate_report_directory_invalid") from exc
        if resolved.parent != resolved_dir or resolved.suffix != ".json" or resolved.is_symlink():
            raise ContractValidationError("aggregate_report_directory_invalid")
        resolved_paths.append(resolved)
    if len(set(resolved_paths)) != len(resolved_paths):
        raise ContractValidationError("aggregate_duplicate_report")
    actual = {path.resolve() for path in resolved_dir.iterdir() if path.is_file()}
    if actual != set(resolved_paths):
        raise ContractValidationError("aggregate_stale_report_directory")


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
    validate_report_directory(report_dir, report_paths)
    identity = load_identity(identity_path)
    producer_status = load_producer_status(producer_status_path)
    reports_by_surface: dict[str, dict[str, Any]] = {}
    surface_keys: set[tuple[str, str, str, str]] = set()
    for report_path in report_paths:
        surface, report = validate_report(report_path, identity, profile, verifier, repo_root)
        key = (surface,) + EXPECTED_SURFACE_KEYS[surface]
        if surface in reports_by_surface or key in surface_keys:
            raise ContractValidationError("aggregate_duplicate_surface")
        reports_by_surface[surface] = report
        surface_keys.add(key)
    if set(reports_by_surface) != set(REQUIRED_SURFACES):
        raise ContractValidationError("aggregate_surface_set_invalid")

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


def run_fixtures(repo_root: Path, include_call_graph: bool, include_redaction: bool) -> None:
    del include_call_graph, include_redaction
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
        if mutation_count != 12:
            raise ContractValidationError("aggregate_fixture_count_invalid")
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
