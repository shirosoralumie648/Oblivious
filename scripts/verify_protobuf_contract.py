#!/usr/bin/env python3
"""Validate the pinned protobuf toolchain and total tracked-file disposition."""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import subprocess
import sys
from typing import Any


SCHEMA_VERSION = "protobuf-toolchain/v1"
EXPECTED_VERSIONS = {
    "protoc": "25.1",
    "protoc-gen-go": "1.36.11",
    "protoc-gen-go-grpc": "1.6.2",
}
EXPECTED_HEADER_VERSION = "v4.25.1"
ALLOWED_PLUGINS = {"go", "go-grpc"}
HEX = set("0123456789abcdef")


class ContractError(RuntimeError):
    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


def fail(code: str, message: str) -> None:
    raise ContractError(code, message)


def canonical_bytes(manifest: dict[str, Any]) -> bytes:
    payload = copy.deepcopy(manifest)
    payload.pop("manifestDigest", None)
    return json.dumps(payload, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode()


def computed_manifest_digest(manifest: dict[str, Any]) -> str:
    return "sha256:" + hashlib.sha256(canonical_bytes(manifest)).hexdigest()


def require_exact_keys(value: dict[str, Any], expected: set[str], where: str) -> None:
    actual = set(value)
    if actual != expected:
        fail("protobuf_manifest_schema_invalid", f"{where} keys: expected {sorted(expected)}, got {sorted(actual)}")


def safe_relpath(value: Any, where: str, *, allow_dot: bool = False) -> str:
    if not isinstance(value, str) or not value:
        fail("protobuf_path_invalid", f"{where} must be a non-empty repository-relative path")
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts or "\\" in value or (value == "." and not allow_dot):
        fail("protobuf_path_invalid", f"{where} is unsafe: {value!r}")
    if value != "." and path.as_posix() != value:
        fail("protobuf_path_invalid", f"{where} is not normalized: {value!r}")
    return value


def git_inventory(repo: Path) -> tuple[set[str], set[str]]:
    result = subprocess.run(
        ["git", "-C", str(repo), "ls-files", "-z", "--", "*.proto", "*.pb.go", "*_grpc.pb.go"],
        check=True,
        stdout=subprocess.PIPE,
    )
    paths = {item.decode() for item in result.stdout.split(b"\0") if item and not item.decode().startswith("reference/")}
    sources = {path for path in paths if path.endswith(".proto")}
    outputs = {path for path in paths if path.endswith(".pb.go")}
    if not sources:
        fail("protobuf_source_inventory_empty", "git discovered zero tracked protobuf sources")
    if not outputs:
        fail("protobuf_output_inventory_empty", "git discovered zero tracked generated Go outputs")
    return sources, outputs


def validate_integrity(manifest: dict[str, Any]) -> None:
    if manifest.get("schemaVersion") != SCHEMA_VERSION:
        fail("protobuf_manifest_schema_invalid", "schemaVersion must be protobuf-toolchain/v1")
    digest = manifest.get("manifestDigest")
    if digest != computed_manifest_digest(manifest):
        fail("protobuf_manifest_digest_mismatch", "manifestDigest does not match canonical manifest bytes")
    if manifest.get("generatedHeaderVersion") != EXPECTED_HEADER_VERSION:
        fail("protobuf_generated_header_version_mismatch", "generatedHeaderVersion must be v4.25.1")
    tools = manifest.get("tools")
    if not isinstance(tools, dict) or set(tools) != set(EXPECTED_VERSIONS):
        fail("protobuf_tool_inventory_invalid", "tools must contain exactly the approved protobuf compiler and plugins")
    for name, version in EXPECTED_VERSIONS.items():
        tool = tools[name]
        if not isinstance(tool, dict) or tool.get("version") != version:
            fail("protobuf_tool_version_mismatch", f"{name} must be pinned to {version}")
    protoc = tools["protoc"]
    require_exact_keys(protoc, {"version", "versionOutput", "platforms"}, "tools.protoc")
    if protoc["versionOutput"] != "libprotoc 25.1":
        fail("protobuf_tool_version_mismatch", "unexpected protoc version output")
    platforms = protoc["platforms"]
    if not isinstance(platforms, dict) or set(platforms) != {"linux-amd64", "linux-arm64"}:
        fail("protobuf_tool_platform_invalid", "protoc platforms must be the closed Linux CI set")
    for platform, artifact in platforms.items():
        require_exact_keys(artifact, {"archive", "url", "sha256"}, f"tools.protoc.platforms.{platform}")
        checksum = artifact["sha256"]
        if not isinstance(checksum, str) or len(checksum) != 64 or set(checksum) - HEX:
            fail("protobuf_tool_checksum_invalid", f"invalid SHA-256 for {platform}")
        expected_prefix = "https://github.com/protocolbuffers/protobuf/releases/download/v25.1/"
        if artifact["url"] != expected_prefix + artifact["archive"]:
            fail("protobuf_tool_artifact_invalid", f"non-canonical protoc artifact for {platform}")
    for name in ("protoc-gen-go", "protoc-gen-go-grpc"):
        tool = tools[name]
        require_exact_keys(tool, {"version", "versionOutput", "module", "checksumModule", "moduleVersion", "moduleSum", "goModSum"}, f"tools.{name}")
        if tool["moduleVersion"] != "v" + tool["version"]:
            fail("protobuf_tool_version_mismatch", f"{name} module version mismatch")
        for field in ("moduleSum", "goModSum"):
            if not isinstance(tool[field], str) or not tool[field].startswith("h1:") or len(tool[field]) < 20:
                fail("protobuf_tool_checksum_invalid", f"{name} {field} is not a Go checksum")


def expected_header_source(source: str, include_root: str) -> str:
    root = PurePosixPath(include_root)
    path = PurePosixPath(source)
    if include_root == ".":
        return path.as_posix()
    try:
        return path.relative_to(root).as_posix()
    except ValueError:
        fail("protobuf_include_root_invalid", f"source {source} is outside include root {include_root}")
    raise AssertionError("unreachable")


def validate_manifest(manifest: dict[str, Any], repo: Path, inventories: tuple[set[str], set[str]] | None = None) -> dict[str, Any]:
    require_exact_keys(manifest, {"schemaVersion", "manifestDigest", "generatedHeaderVersion", "tools", "dispositions"}, "manifest")
    validate_integrity(manifest)
    sources, outputs = inventories or git_inventory(repo)
    dispositions = manifest.get("dispositions")
    if not isinstance(dispositions, list) or not dispositions:
        fail("protobuf_disposition_inventory_empty", "dispositions must be non-empty")
    seen_sources: set[str] = set()
    seen_outputs: set[str] = set()
    managed_count = 0
    source_only_count = 0
    generation_count = 0
    for index, disposition in enumerate(dispositions):
        where = f"dispositions[{index}]"
        if not isinstance(disposition, dict):
            fail("protobuf_manifest_schema_invalid", f"{where} must be an object")
        require_exact_keys(disposition, {"source", "mode", "includeRoot", "outputs", "consumers", "reason", "generations"}, where)
        source = safe_relpath(disposition["source"], where + ".source")
        include_root = safe_relpath(disposition["includeRoot"], where + ".includeRoot", allow_dot=True)
        if source in seen_sources:
            fail("protobuf_duplicate_source", f"source has multiple dispositions: {source}")
        seen_sources.add(source)
        if not (repo / source).is_file():
            fail("protobuf_source_missing", f"source does not exist: {source}")
        expected_header_source(source, include_root)
        consumer_values = disposition["consumers"]
        if not isinstance(consumer_values, list) or not consumer_values:
            fail("protobuf_consumer_missing", f"{source} has no concrete consumer")
        consumers: list[str] = []
        for consumer_index, consumer_value in enumerate(consumer_values):
            consumer = safe_relpath(consumer_value, f"{where}.consumers[{consumer_index}]")
            if not (repo / consumer).exists():
                fail("protobuf_consumer_missing", f"consumer does not exist: {consumer}")
            consumers.append(consumer)
        if len(consumers) != len(set(consumers)):
            fail("protobuf_duplicate_consumer", f"duplicate consumer for {source}")
        reason = disposition["reason"]
        if not isinstance(reason, str) or len(reason.strip()) < 12:
            fail("protobuf_reason_missing", f"{source} requires a concrete disposition reason")
        declared_outputs = disposition["outputs"]
        generations = disposition["generations"]
        if not isinstance(declared_outputs, list) or not isinstance(generations, list):
            fail("protobuf_manifest_schema_invalid", f"{source} outputs/generations must be arrays")
        local_outputs: list[str] = []
        for output_index, output_value in enumerate(declared_outputs):
            output = safe_relpath(output_value, f"{where}.outputs[{output_index}]")
            if not output.endswith(".pb.go"):
                fail("protobuf_output_path_invalid", f"generated output must end in .pb.go: {output}")
            if output in seen_outputs:
                fail("protobuf_duplicate_output", f"output has multiple dispositions: {output}")
            seen_outputs.add(output)
            local_outputs.append(output)
            if not (repo / output).is_file():
                fail("protobuf_output_missing", f"tracked output does not exist: {output}")
        mode = disposition["mode"]
        if mode == "source-only":
            source_only_count += 1
            if local_outputs or generations:
                fail("protobuf_source_only_has_outputs", f"source-only disposition has outputs: {source}")
            continue
        if mode != "managed":
            fail("protobuf_disposition_mode_invalid", f"unsupported mode for {source}: {mode!r}")
        managed_count += 1
        if not local_outputs:
            fail("protobuf_managed_outputs_empty", f"managed source has zero outputs: {source}")
        if not generations:
            fail("protobuf_generation_inventory_empty", f"managed source has zero generations: {source}")
        generated_outputs: list[str] = []
        generation_names: set[str] = set()
        for generation_index, generation in enumerate(generations):
            generation_count += 1
            generation_where = f"{where}.generations[{generation_index}]"
            require_exact_keys(generation, {"name", "includeRoot", "outputRoot", "plugins", "options", "outputs"}, generation_where)
            name = generation["name"]
            if not isinstance(name, str) or not name or name in generation_names:
                fail("protobuf_generation_name_invalid", f"invalid/duplicate generation name for {source}")
            generation_names.add(name)
            generation_include = safe_relpath(generation["includeRoot"], generation_where + ".includeRoot", allow_dot=True)
            safe_relpath(generation["outputRoot"], generation_where + ".outputRoot", allow_dot=True)
            header_source = expected_header_source(source, generation_include)
            plugins = generation["plugins"]
            options = generation["options"]
            generation_outputs = generation["outputs"]
            if not isinstance(plugins, list) or not plugins or len(plugins) != len(set(plugins)) or set(plugins) - ALLOWED_PLUGINS:
                fail("protobuf_plugin_inventory_invalid", f"invalid plugins for generation {name}")
            if not isinstance(options, dict) or set(options) != set(plugins):
                fail("protobuf_plugin_options_invalid", f"options must exactly match plugins for generation {name}")
            for plugin, values in options.items():
                if not isinstance(values, list) or not values or any(not isinstance(value, str) or not value for value in values):
                    fail("protobuf_plugin_options_invalid", f"empty/invalid options for {name}/{plugin}")
            if not isinstance(generation_outputs, list) or not generation_outputs:
                fail("protobuf_generation_outputs_empty", f"generation has zero outputs: {name}")
            for output_value in generation_outputs:
                output = safe_relpath(output_value, generation_where + ".outputs[]")
                generated_outputs.append(output)
                header = (repo / output).read_text(encoding="utf-8").splitlines()[:8]
                header_text = "\n".join(header)
                if f"protoc        {EXPECTED_HEADER_VERSION}" not in header_text and f"protoc             {EXPECTED_HEADER_VERSION}" not in header_text:
                    fail("protobuf_generated_header_version_mismatch", f"wrong protoc header in {output}")
                if f"source: {header_source}" not in header_text:
                    fail("protobuf_generated_source_mismatch", f"wrong generated source header in {output}; expected {header_source}")
                if output.endswith("_grpc.pb.go"):
                    if "go-grpc" not in plugins or "protoc-gen-go-grpc v1.6.2" not in header_text:
                        fail("protobuf_generated_plugin_version_mismatch", f"wrong gRPC plugin header in {output}")
                elif "go" not in plugins or "protoc-gen-go v1.36.11" not in header_text:
                    fail("protobuf_generated_plugin_version_mismatch", f"wrong Go plugin header in {output}")
        if sorted(generated_outputs) != sorted(local_outputs) or len(generated_outputs) != len(set(generated_outputs)):
            fail("protobuf_generation_output_mismatch", f"generation outputs do not exactly partition disposition outputs for {source}")
    if seen_sources != sources:
        missing = sorted(sources - seen_sources)
        extra = sorted(seen_sources - sources)
        fail("protobuf_source_disposition_mismatch", f"source dispositions differ from Git: missing={missing}, extra={extra}")
    if seen_outputs != outputs:
        missing = sorted(outputs - seen_outputs)
        extra = sorted(seen_outputs - outputs)
        fail("protobuf_output_disposition_mismatch", f"output dispositions differ from Git: missing={missing}, extra={extra}")
    if not managed_count or not source_only_count or not generation_count:
        fail("protobuf_disposition_inventory_empty", "managed/source-only/generation counts must all be positive")
    return {
        "schemaVersion": SCHEMA_VERSION,
        "manifestDigest": manifest["manifestDigest"],
        "toolVersions": EXPECTED_VERSIONS,
        "generatedHeaderVersion": EXPECTED_HEADER_VERSION,
        "sourceCount": len(sources),
        "outputCount": len(outputs),
        "dispositionCount": len(dispositions),
        "managedCount": managed_count,
        "sourceOnlyCount": source_only_count,
        "generationCount": generation_count,
    }


def refresh_digest(manifest: dict[str, Any]) -> None:
    manifest["manifestDigest"] = computed_manifest_digest(manifest)


def run_fixtures(base: dict[str, Any], repo: Path, inventories: tuple[set[str], set[str]]) -> dict[str, int]:
    cases: list[tuple[str, str, Any, tuple[set[str], set[str]] | None]] = []

    def add(name: str, code: str, mutate: Any, fixture_inventories: tuple[set[str], set[str]] | None = None) -> None:
        cases.append((name, code, mutate, fixture_inventories))

    add("wrong tool version", "protobuf_tool_version_mismatch", lambda value: value["tools"]["protoc"].update(version="25.2"))
    add("wrong artifact checksum", "protobuf_tool_checksum_invalid", lambda value: value["tools"]["protoc"]["platforms"]["linux-amd64"].update(sha256="bad"))
    add("duplicate source", "protobuf_duplicate_source", lambda value: value["dispositions"].append(copy.deepcopy(value["dispositions"][0])))
    add("duplicate output", "protobuf_duplicate_output", lambda value: value["dispositions"][3]["outputs"].append(value["dispositions"][0]["outputs"][0]))
    add("source-only consumer missing", "protobuf_consumer_missing", lambda value: value["dispositions"][2].update(consumers=[]))
    add("source-only reason missing", "protobuf_reason_missing", lambda value: value["dispositions"][2].update(reason=""))
    add("managed outputs empty", "protobuf_managed_outputs_empty", lambda value: value["dispositions"][1].update(outputs=[], generations=[]))
    add("generation outputs empty", "protobuf_generation_outputs_empty", lambda value: value["dispositions"][1]["generations"][0].update(outputs=[]))
    add("unsafe source path", "protobuf_path_invalid", lambda value: value["dispositions"][0].update(source="../agent.proto"))
    add("generated header version drift", "protobuf_generated_header_version_mismatch", lambda value: value.update(generatedHeaderVersion="v4.25.2"))
    add("manifest digest drift", "protobuf_manifest_digest_mismatch", lambda value: value.update(manifestDigest="sha256:" + "0" * 64))
    add("undispositioned source", "protobuf_source_disposition_mismatch", lambda value: value["dispositions"].pop(), inventories)
    add("undispositioned output", "protobuf_output_disposition_mismatch", lambda value: (value["dispositions"][0]["outputs"].pop(0), value["dispositions"][0]["generations"][0]["outputs"].pop(0)), inventories)
    add("empty source inventory", "protobuf_source_inventory_empty", lambda value: None, (set(), inventories[1]))
    add("empty output inventory", "protobuf_output_inventory_empty", lambda value: None, (inventories[0], set()))

    counts: dict[str, int] = {}
    for number, (name, expected_code, mutate, fixture_inventories) in enumerate(cases, start=1):
        fixture = copy.deepcopy(base)
        mutate(fixture)
        if expected_code != "protobuf_manifest_digest_mismatch":
            refresh_digest(fixture)
        try:
            selected_inventory = fixture_inventories or inventories
            if not selected_inventory[0]:
                fail("protobuf_source_inventory_empty", "fixture source inventory is empty")
            if not selected_inventory[1]:
                fail("protobuf_output_inventory_empty", "fixture output inventory is empty")
            validate_manifest(fixture, repo, selected_inventory)
        except ContractError as error:
            if error.code != expected_code:
                print(f"not ok {number} - {name} # expected {expected_code}, got {error.code}")
                raise
            counts[expected_code] = counts.get(expected_code, 0) + 1
            print(f"ok {number} - {name} [{error.code}]")
        else:
            fail("protobuf_fixture_false_green", f"fixture unexpectedly passed: {name}")
    if not cases or any(count <= 0 for count in counts.values()):
        fail("protobuf_fixture_count_zero", "every fixture mutation family must execute")
    return {"caseCount": len(cases), "mutationCount": sum(counts.values()), "mutationFamilyCount": len(counts)}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", default="config/release/protobuf-toolchain.v1.json")
    parser.add_argument("--fixtures", action="store_true")
    parser.add_argument("--manifest-only", action="store_true")
    parser.add_argument("--print-computed-digest", action="store_true")
    args = parser.parse_args()

    repo = Path(__file__).resolve().parent.parent
    manifest_path = (repo / args.manifest).resolve()
    try:
        manifest_path.relative_to(repo.resolve())
    except ValueError:
        print("protobuf_path_invalid: manifest must be inside the repository", file=sys.stderr)
        return 2
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        if args.print_computed_digest:
            print(computed_manifest_digest(manifest))
            return 0
        inventories = git_inventory(repo)
        summary = validate_manifest(manifest, repo, inventories)
        if args.fixtures:
            summary.update(run_fixtures(manifest, repo, inventories))
        summary["result"] = "pass"
        summary["mode"] = "manifest-only" if args.manifest_only else "contract"
        print("# summary " + json.dumps(summary, sort_keys=True, separators=(",", ":")))
        return 0
    except (OSError, json.JSONDecodeError, subprocess.CalledProcessError) as error:
        print(f"protobuf_contract_io_error: {error}", file=sys.stderr)
        return 2
    except ContractError as error:
        print(f"{error.code}: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
