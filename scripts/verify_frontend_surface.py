#!/usr/bin/env python3
"""Verify compiler-sidecar transport and product exposure against canonical JSON inputs."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import sys
import tempfile
from typing import Any


SIDECAR_SCHEMA = "frontend-surface-sidecar/v1"
MANIFEST_SCHEMA = "route-surface-manifest/v2"
TRANSPORT_SCHEMA = "frontend-transport-observation/v1"
SHA256_PREFIX = "sha256:"


class SurfaceError(RuntimeError):
    def __init__(self, code: str, detail: str = "") -> None:
        super().__init__(detail)
        self.code = code


def fail(code: str, detail: str = "") -> None:
    raise SurfaceError(code, detail)


def canonical_bytes(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode()


def digest_bytes(value: bytes) -> str:
    return SHA256_PREFIX + hashlib.sha256(value).hexdigest()


def canonical_digest(value: Any) -> str:
    return digest_bytes(canonical_bytes(value))


def valid_digest(value: Any) -> bool:
    if not isinstance(value, str) or not value.startswith(SHA256_PREFIX) or len(value) != 71:
        return False
    try:
        int(value[len(SHA256_PREFIX) :], 16)
    except ValueError:
        return False
    return value[len(SHA256_PREFIX) :] == value[len(SHA256_PREFIX) :].lower()


def load_object(path: Path, code: str) -> tuple[dict[str, Any], bytes]:
    try:
        raw = path.read_bytes()
        value = json.loads(raw)
    except (OSError, json.JSONDecodeError) as error:
        raise SurfaceError(code, str(path)) from error
    if not isinstance(value, dict):
        fail(code, str(path))
    return value, raw


def write_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    content = json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n"
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def require_object(value: Any, code: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        fail(code)
    return value


def require_list(value: Any, code: str, *, nonempty: bool = False) -> list[Any]:
    if not isinstance(value, list) or (nonempty and not value):
        fail(code)
    return value


def safe_source(source: Any) -> tuple[str, int, int]:
    source = require_object(source, "frontend_source_invalid")
    if set(source) != {"file", "line", "column", "symbol"}:
        fail("frontend_source_invalid")
    file = source.get("file")
    line = source.get("line")
    column = source.get("column")
    symbol = source.get("symbol")
    if not isinstance(file, str) or not file or not isinstance(symbol, str) or not symbol:
        fail("frontend_source_invalid")
    path = PurePosixPath(file)
    if path.is_absolute() or ".." in path.parts or path.as_posix() != file or "\\" in file:
        fail("frontend_source_invalid")
    if not isinstance(line, int) or line <= 0 or not isinstance(column, int) or column <= 0:
        fail("frontend_source_invalid")
    if any(part == "generated" for part in path.parts) or ".generated." in path.name:
        fail("frontend_generated_call_classified", file)
    return file, line, column


def media_base(value: Any) -> tuple[str, dict[str, str]] | None:
    if not isinstance(value, str) or not value.strip():
        return None
    parts = [part.strip() for part in value.split(";")]
    base = parts[0].lower()
    if "/" not in base:
        return None
    parameters: dict[str, str] = {}
    for part in parts[1:]:
        if "=" not in part:
            return None
        name, parameter_value = (item.strip().lower() for item in part.split("=", 1))
        parameter_value = parameter_value.strip('"')
        if not name or not parameter_value or name in parameters:
            return None
        parameters[name] = parameter_value
    return base, parameters


def json_media(value: Any) -> bool:
    parsed = media_base(value)
    return parsed is not None and (parsed[0] == "application/json" or parsed[0].endswith("+json"))


def validate_manifest(manifest: dict[str, Any]) -> tuple[dict[str, dict[str, Any]], dict[str, dict[str, Any]]]:
    required = {
        "schemaVersion",
        "generatedFrom",
        "projectionDigest",
        "browserEventDigest",
        "scope",
        "operations",
        "browserEvents",
        "routeSamples",
    }
    if set(manifest) != required or manifest.get("schemaVersion") != MANIFEST_SCHEMA:
        fail("frontend_manifest_schema_invalid")
    operations = require_list(
        manifest.get("operations"), "frontend_core_inventory_empty", nonempty=True
    )
    scope = require_object(manifest.get("scope"), "frontend_manifest_schema_invalid")
    if manifest.get("projectionDigest") != canonical_digest({"scope": scope, "operations": operations}):
        fail("frontend_manifest_digest_mismatch")
    browser_events = require_list(
        manifest.get("browserEvents"), "frontend_browser_event_inventory_empty", nonempty=True
    )
    if manifest.get("browserEventDigest") != canonical_digest(browser_events):
        fail("frontend_manifest_digest_mismatch")

    operation_index: dict[str, dict[str, Any]] = {}
    operation_keys: set[tuple[Any, Any]] = set()
    for raw_operation in operations:
        operation = require_object(raw_operation, "frontend_core_invalid")
        operation_id = operation.get("operationId")
        key = (operation.get("method"), operation.get("normalizedPath"))
        if not isinstance(operation_id, str) or not operation_id or operation_id in operation_index or key in operation_keys:
            fail("frontend_core_ambiguous", str(operation_id))
        operation_index[operation_id] = operation
        operation_keys.add(key)

    event_index: dict[str, dict[str, Any]] = {}
    for raw_event in browser_events:
        event = require_object(raw_event, "frontend_event_identity_invalid")
        operation_id = event.get("operationId")
        if operation_id not in operation_index or operation_id in event_index:
            fail("frontend_event_identity_invalid", str(operation_id))
        if event.get("transport") not in {"sse", "websocket"}:
            fail("frontend_event_identity_invalid", str(operation_id))
        events = require_list(event.get("events"), "frontend_event_identity_invalid", nonempty=True)
        for item in events:
            item = require_object(item, "frontend_event_identity_invalid")
            if set(item) != {"direction", "kind", "schemaIdentity"}:
                fail("frontend_event_identity_invalid", str(operation_id))
            if item.get("direction") not in {"client", "server"} or item.get("kind") not in {"message", "event"}:
                fail("frontend_event_identity_invalid", str(operation_id))
        event_index[operation_id] = event
    return operation_index, event_index


def validate_sidecar_header(sidecar: dict[str, Any], manifest_count: int) -> tuple[str, str]:
    if sidecar.get("schemaVersion") != SIDECAR_SCHEMA:
        fail("frontend_sidecar_schema_invalid")
    scope = require_object(sidecar.get("sourceScope"), "frontend_sidecar_schema_invalid")
    extractor = require_object(sidecar.get("extractor"), "frontend_sidecar_schema_invalid")
    if scope.get("generatedPolicy") != "consumer-only" or not isinstance(scope.get("filesScanned"), int) or scope["filesScanned"] <= 0:
        fail("frontend_sidecar_scope_invalid")
    source_digest = scope.get("sourceDigest")
    config_digest = extractor.get("configDigest")
    if not valid_digest(source_digest) or not valid_digest(config_digest):
        fail("frontend_sidecar_digest_invalid")
    unresolved = require_list(sidecar.get("unresolved"), "frontend_sidecar_schema_invalid")
    if unresolved:
        fail("frontend_sidecar_unresolved", str(len(unresolved)))
    generated_consumers = sidecar.get("generatedConsumers")
    if generated_consumers != manifest_count:
        fail("frontend_generated_consumer_mismatch", str(generated_consumers))
    return source_digest, config_digest


def validate_request(row: dict[str, Any], core: dict[str, Any]) -> str:
    request = require_object(row.get("request"), "frontend_request_invalid")
    encoder = require_object(row.get("requestEncoder"), "frontend_request_invalid")
    if set(encoder) != {"id", "mediaType", "schemaIdentity"}:
        fail("frontend_request_invalid")
    if request.get("mediaType") != encoder.get("mediaType"):
        fail("frontend_request_media_incompatible")
    if request.get("schemaIdentity") != encoder.get("schemaIdentity"):
        fail("frontend_request_schema_incompatible")
    if {"mediaType": encoder.get("mediaType"), "schemaIdentity": encoder.get("schemaIdentity")} != core.get("request"):
        fail("frontend_request_core_mismatch")
    encoder_id = encoder.get("id")
    media_type = encoder.get("mediaType")
    if encoder_id == "none":
        if media_type is not None or encoder.get("schemaIdentity") != {"kind": "none", "value": None}:
            fail("frontend_request_encoder_incompatible")
    elif encoder_id == "json":
        if not json_media(media_type):
            fail("frontend_request_encoder_incompatible")
    elif encoder_id == "form-data":
        if media_base(media_type) != ("multipart/form-data", {}):
            fail("frontend_request_encoder_incompatible")
    elif encoder_id == "raw":
        if media_base(media_type) is None:
            fail("frontend_request_encoder_incompatible")
    else:
        fail("frontend_request_encoder_incompatible")
    if request.get("encoder") != encoder_id or request.get("schemaRef") != encoder.get("schemaIdentity", {}).get("value"):
        fail("frontend_request_encoder_incompatible")
    return str(encoder_id)


def validate_response(row: dict[str, Any], core: dict[str, Any], transport_kind: str) -> str:
    response = require_object(row.get("response"), "frontend_response_invalid")
    decoder = require_object(row.get("responseDecoder"), "frontend_response_invalid")
    if set(decoder) != {"id", "status", "mediaType", "schemaIdentity"}:
        fail("frontend_response_invalid")
    if response.get("status") != decoder.get("status"):
        fail("frontend_response_status_incompatible")
    if response.get("mediaType") != decoder.get("mediaType"):
        fail("frontend_response_media_incompatible")
    if response.get("schemaIdentity") != decoder.get("schemaIdentity"):
        fail("frontend_response_schema_incompatible")
    if response.get("decoder") != decoder.get("id") or response.get("schemaRef") != decoder.get("schemaIdentity", {}).get("value"):
        fail("frontend_response_decoder_incompatible")
    candidates = [
        item
        for item in require_list(core.get("successResponses"), "frontend_core_invalid", nonempty=True)
        if item.get("status") == str(decoder.get("status"))
        and item.get("mediaType") == decoder.get("mediaType")
        and item.get("schemaIdentity") == decoder.get("schemaIdentity")
    ]
    if len(candidates) != 1:
        fail("frontend_response_core_mismatch")
    decoder_id = decoder.get("id")
    media_type = decoder.get("mediaType")
    parsed = media_base(media_type)
    if decoder_id == "json-envelope":
        if not json_media(media_type):
            fail("frontend_response_decoder_incompatible")
    elif decoder_id == "text":
        if parsed is None or not parsed[0].startswith("text/") or set(parsed[1]) - {"charset"} or parsed[1].get("charset", "utf-8") != "utf-8":
            fail("frontend_response_decoder_incompatible")
    elif decoder_id == "raw-response":
        if media_type is None and transport_kind != "websocket":
            fail("frontend_response_decoder_incompatible")
    elif decoder_id == "none":
        if decoder.get("status") != 204 or media_type is not None or decoder.get("schemaIdentity") != {"kind": "none", "value": None}:
            fail("frontend_response_decoder_incompatible")
    elif decoder_id == "event-source":
        if transport_kind != "event-source" or parsed is None or parsed[0] != "text/event-stream" or parsed[1]:
            fail("frontend_response_decoder_incompatible")
    else:
        fail("frontend_response_decoder_incompatible")
    return str(decoder_id)


def validate_events(
    row: dict[str, Any], event_index: dict[str, dict[str, Any]], operation_id: str, protocol: str, kind: str
) -> int:
    actual = require_list(row.get("events"), "frontend_event_identity_invalid")
    streaming = kind in {"sse-stream", "event-source", "websocket"}
    expected_row = event_index.get(operation_id)
    if not streaming:
        if actual:
            fail("frontend_event_identity_mismatch", operation_id)
        return 0
    if expected_row is None:
        fail("frontend_event_identity_missing", operation_id)
    expected_transport = "websocket" if protocol == "websocket" else "sse"
    if expected_row.get("transport") != expected_transport or actual != expected_row.get("events"):
        fail("frontend_event_identity_mismatch", operation_id)
    return len(actual)


def verify_transport(sidecar_path: Path, manifest_path: Path) -> dict[str, Any]:
    sidecar, sidecar_bytes = load_object(sidecar_path, "frontend_sidecar_invalid")
    manifest, _ = load_object(manifest_path, "frontend_manifest_invalid")
    operation_index, event_index = validate_manifest(manifest)
    source_digest, config_digest = validate_sidecar_header(sidecar, len(operation_index))
    operations = require_list(
        sidecar.get("operations"), "frontend_operation_inventory_empty", nonempty=True
    )
    seen_sources: set[tuple[str, int, int]] = set()
    taxonomy: list[dict[str, Any]] = []
    allowed_taxonomy = {
        ("http", "http-client"),
        ("http", "raw-fetch"),
        ("http", "swr"),
        ("http", "multipart-upload"),
        ("sse", "sse-stream"),
        ("sse", "event-source"),
        ("websocket", "websocket"),
    }
    for raw_row in operations:
        row = require_object(raw_row, "frontend_operation_invalid")
        source = safe_source(row.get("source"))
        if source in seen_sources:
            fail("frontend_source_classification_duplicate", source[0])
        seen_sources.add(source)
        contract = require_object(row.get("contract"), "frontend_core_invalid")
        operation = require_object(row.get("operation"), "frontend_core_invalid")
        operation_id = operation.get("operationId")
        core = operation_index.get(operation_id)
        if core is None:
            fail("frontend_core_missing", str(operation_id))
        if contract != operation or operation != core:
            fail("frontend_core_mismatch", str(operation_id))
        transport = require_object(row.get("transport"), "frontend_transport_invalid")
        protocol = transport.get("protocol")
        kind = transport.get("kind")
        if (protocol, kind) not in allowed_taxonomy:
            fail("frontend_transport_taxonomy_unknown", str(kind))
        if (
            transport.get("method") != core.get("method")
            or transport.get("pathTemplate") != core.get("normalizedPath")
            or row.get("source", {}).get("symbol") != operation_id
        ):
            fail("frontend_transport_core_mismatch", str(operation_id))
        encoder = validate_request(row, core)
        decoder = validate_response(row, core, str(kind))
        event_count = validate_events(row, event_index, str(operation_id), str(protocol), str(kind))
        taxonomy.append(
            {
                "protocol": protocol,
                "kind": kind,
                "encoder": encoder,
                "decoder": decoder,
                "eventCount": event_count,
            }
        )
    taxonomy.sort(key=canonical_bytes)
    count = len(operations)
    return {
        "schemaVersion": TRANSPORT_SCHEMA,
        "sidecarDigest": digest_bytes(sidecar_bytes),
        "sourceDigest": source_digest,
        "configDigest": config_digest,
        "operationCount": count,
        "coreCount": count,
        "compatibleCount": count,
        "taxonomyDigest": canonical_digest(taxonomy),
        "unresolvedCount": 0,
        "errorCodes": [],
        "skippedChecks": [],
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    transport = subparsers.add_parser("transport")
    transport.add_argument("--sidecar", required=True)
    transport.add_argument("--manifest", required=True)
    transport.add_argument("--output", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "transport":
            result = verify_transport(Path(args.sidecar), Path(args.manifest))
        else:  # pragma: no cover - argparse owns the command set.
            fail("frontend_command_invalid")
        write_json(Path(args.output), result)
        print(json.dumps({"schemaVersion": result["schemaVersion"], "result": "pass"}, sort_keys=True))
        return 0
    except SurfaceError as error:
        print(error.code, file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
