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
EXPOSURE_SCHEMA = "frontend-exposure-observation/v1"
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


def safe_exposure_source(source: Any) -> tuple[str, int, int]:
    source = require_object(source, "frontend_exposure_source_invalid")
    if set(source) != {"file", "line", "column"}:
        fail("frontend_exposure_source_invalid")
    file = source.get("file")
    line = source.get("line")
    column = source.get("column")
    if not isinstance(file, str) or not file:
        fail("frontend_exposure_source_invalid")
    path = PurePosixPath(file)
    if path.is_absolute() or ".." in path.parts or path.as_posix() != file or "\\" in file:
        fail("frontend_exposure_source_invalid")
    if not isinstance(line, int) or line <= 0 or not isinstance(column, int) or column <= 0:
        fail("frontend_exposure_source_invalid")
    return file, line, column


def project_release_contract(contract: dict[str, Any]) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
    if contract.get("schemaVersion") != "contract/v1" or contract.get("defaultProfile") != "monolith":
        fail("frontend_release_contract_invalid")
    raw_capabilities = require_list(contract.get("capabilities"), "frontend_release_contract_invalid", nonempty=True)
    capabilities: dict[str, dict[str, Any]] = {}
    projected_capabilities: list[dict[str, Any]] = []
    for raw in raw_capabilities:
        row = require_object(raw, "frontend_release_contract_invalid")
        capability_id = row.get("id")
        disposition = row.get("commitment")
        if (
            not isinstance(capability_id, str)
            or not capability_id
            or capability_id in capabilities
            or disposition not in {"committed", "conditional", "excluded"}
        ):
            fail("frontend_release_contract_invalid")
        capabilities[capability_id] = row
        projected = {
            "capabilityId": capability_id,
            "disposition": disposition,
            "navigationDisposition": "visible" if disposition == "committed" else "conditional" if disposition == "conditional" else "hidden",
        }
        if row.get("reasonCode") is not None:
            projected["reasonCode"] = row["reasonCode"]
        projected_capabilities.append(projected)
    projected_capabilities.sort(key=lambda row: row["capabilityId"])

    surfaces: list[dict[str, Any]] = []
    surface_ids: set[str] = set()
    for raw in require_list(contract.get("surfaceReferences"), "frontend_release_contract_invalid", nonempty=True):
        row = require_object(raw, "frontend_release_contract_invalid")
        surface_id = row.get("id")
        capability_ids = row.get("capabilityIds")
        if not isinstance(surface_id, str) or not surface_id or surface_id in surface_ids or not isinstance(capability_ids, list):
            fail("frontend_release_contract_invalid")
        if len(set(capability_ids)) != len(capability_ids) or any(item not in capabilities for item in capability_ids):
            fail("frontend_release_contract_invalid")
        surface_ids.add(surface_id)
        commitments = {capabilities[item]["commitment"] for item in capability_ids}
        disposition = "committed" if "committed" in commitments else "conditional" if "conditional" in commitments else "excluded"
        surfaces.append(
            {
                "surfaceId": surface_id,
                "canonicalSource": row.get("canonicalSource"),
                "consumer": row.get("consumer"),
                "capabilityIds": sorted(capability_ids),
                "disposition": disposition,
                "navigationDisposition": "visible" if disposition == "committed" else "conditional" if disposition == "conditional" else "hidden",
            }
        )
    surfaces.sort(key=lambda row: row["surfaceId"])
    return {"capabilities": projected_capabilities, "surfaces": surfaces}, capabilities


def validate_release_identity(value: Any) -> dict[str, str]:
    identity = require_object(value, "frontend_exposure_identity_invalid")
    if set(identity) != {"sourceTree", "contractDigest", "deploymentProfile"}:
        fail("frontend_exposure_identity_invalid")
    source_tree = identity.get("sourceTree")
    if (
        not isinstance(source_tree, str)
        or len(source_tree) != 40
        or any(character not in "0123456789abcdef" for character in source_tree)
        or not valid_digest(identity.get("contractDigest"))
        or identity.get("deploymentProfile") != "monolith"
    ):
        fail("frontend_exposure_identity_invalid")
    return identity  # type: ignore[return-value]


def runtime_projection_digest(identity: dict[str, str], generation: int, capabilities: list[dict[str, Any]]) -> str:
    payload = {
        "identity": {
            "sourceTree": identity["sourceTree"],
            "contractDigest": identity["contractDigest"],
            "deploymentProfile": identity["deploymentProfile"],
        },
        "generation": generation,
        "capabilities": capabilities,
    }
    return digest_bytes(json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode())


def validate_app_projection(
    value: dict[str, Any], capabilities: dict[str, dict[str, Any]]
) -> tuple[dict[str, str], dict[str, dict[str, Any]]]:
    if set(value) != {"schemaVersion", "provenance", "releaseIdentity", "generation", "projectionDigest", "capabilities"}:
        fail("frontend_app_projection_schema_invalid")
    if value.get("schemaVersion") != "frontend-app-projection-observation/v1":
        fail("frontend_app_projection_schema_invalid")
    provenance = require_object(value.get("provenance"), "frontend_app_projection_provenance_invalid")
    if provenance != {
        "source": "authenticated-api",
        "provider": "ReleaseProjectionProvider",
        "operationId": "getAppReadinessCapabilities",
    }:
        fail("frontend_app_projection_provenance_invalid")
    identity = validate_release_identity(value.get("releaseIdentity"))
    generation = value.get("generation")
    if not isinstance(generation, int) or isinstance(generation, bool) or generation <= 0:
        fail("frontend_app_projection_schema_invalid")
    rows = require_list(value.get("capabilities"), "frontend_app_projection_schema_invalid", nonempty=True)
    expected_ids = sorted(capability_id for capability_id, row in capabilities.items() if row["commitment"] != "excluded")
    index: dict[str, dict[str, Any]] = {}
    normalized: list[dict[str, Any]] = []
    for raw in rows:
        row = require_object(raw, "frontend_app_projection_schema_invalid")
        if set(row) != {"capabilityId", "disposition", "availability", "enabled"}:
            fail("frontend_app_projection_schema_invalid")
        capability_id = row.get("capabilityId")
        if not isinstance(capability_id, str) or capability_id in index or capability_id not in capabilities:
            fail("frontend_app_projection_capability_unknown")
        expected = capabilities[capability_id]["commitment"]
        if expected == "excluded":
            fail("frontend_excluded_capability_exposed")
        if row.get("disposition") != expected or row.get("availability") not in {"enabled", "disabled", "blocked"}:
            fail("frontend_app_projection_state_invalid")
        if not isinstance(row.get("enabled"), bool) or row["enabled"] != (row["availability"] == "enabled"):
            fail("frontend_app_projection_state_invalid")
        index[capability_id] = row
        normalized.append(
            {
                "capabilityId": capability_id,
                "disposition": row["disposition"],
                "availability": row["availability"],
                "enabled": row["enabled"],
            }
        )
    if [row["capabilityId"] for row in normalized] != expected_ids:
        fail("frontend_app_projection_inventory_mismatch")
    if value.get("projectionDigest") != runtime_projection_digest(identity, generation, normalized):
        fail("frontend_app_projection_digest_mismatch")
    return identity, index


def validate_catalog(
    value: dict[str, Any], contract: dict[str, Any], identity: dict[str, str], capabilities: dict[str, dict[str, Any]]
) -> list[dict[str, Any]]:
    if set(value) != {"schemaVersion", "releaseIdentity", "subjects"} or value.get("schemaVersion") != "frontend-server-catalog-observation/v1":
        fail("frontend_catalog_schema_invalid")
    if validate_release_identity(value.get("releaseIdentity")) != identity:
        fail("frontend_exposure_identity_splice")
    profiles = require_list(contract.get("profiles"), "frontend_release_contract_invalid", nonempty=True)
    profile = next((row for row in profiles if isinstance(row, dict) and row.get("id") == "monolith" and row.get("commitment") == "committed"), None)
    if profile is None or not isinstance(profile.get("catalogBindingIds"), list):
        fail("frontend_release_contract_invalid")
    bindings = require_list(contract.get("catalogBindings"), "frontend_release_contract_invalid", nonempty=True)
    binding_index = {
        row.get("id"): row
        for row in bindings
        if isinstance(row, dict) and isinstance(row.get("id"), str)
    }
    expected_ids = profile["catalogBindingIds"]
    if len(set(expected_ids)) != len(expected_ids) or any(item not in binding_index for item in expected_ids):
        fail("frontend_release_contract_invalid")
    expected = sorted((binding_index[item] for item in expected_ids), key=lambda row: row["id"])
    subjects = require_list(value.get("subjects"), "frontend_catalog_inventory_mismatch", nonempty=True)
    actual: list[dict[str, Any]] = []
    for raw in subjects:
        row = require_object(raw, "frontend_catalog_schema_invalid")
        if set(row) != {"id", "subjectKind", "subjectId", "runtimeClass", "capabilityId"}:
            fail("frontend_catalog_schema_invalid")
        if row.get("capabilityId") not in capabilities:
            fail("frontend_catalog_capability_unknown")
        actual.append(row)
    actual.sort(key=lambda row: str(row.get("id")))
    if actual != expected:
        fail("frontend_catalog_inventory_mismatch")
    return actual


def validate_dto_and_mutation_contracts(sidecar: dict[str, Any]) -> int:
    dtos = require_list(sidecar.get("dtoContracts"), "frontend_dto_contract_invalid", nonempty=True)
    dto_index: dict[str, dict[str, Any]] = {}
    expected_dto_names = {
        "AppCapabilityProjectionResponse",
        "ModelOption",
        "AgentToolDefinition",
        "UpdateConversationConfigRequest",
        "CreateAgentRequest",
        "UpdateAgentRequest",
        "AgentTool",
    }
    for raw in dtos:
        row = require_object(raw, "frontend_dto_contract_invalid")
        safe_exposure_source(row.get("source"))
        name = row.get("name")
        fields = row.get("fields")
        if not isinstance(name, str) or name in dto_index or not isinstance(fields, list) or fields != sorted(set(fields)):
            fail("frontend_dto_contract_invalid")
        dto_index[name] = row
    if set(dto_index) != expected_dto_names:
        fail("frontend_dto_contract_invalid")
    for name in ("ModelOption", "AgentToolDefinition"):
        if "capabilityId" not in dto_index[name]["fields"] or dto_index[name].get("role") != "catalog-response":
            fail("frontend_catalog_selector_identity_invalid")
    for name in ("UpdateConversationConfigRequest", "CreateAgentRequest", "UpdateAgentRequest", "AgentTool"):
        if "capabilityId" in dto_index[name]["fields"] or dto_index[name].get("role") != "mutation-request":
            fail("frontend_mutation_capability_identity")

    expected_mutations = {
        "chat-model-mutation": ("ModelOption", "UpdateConversationConfigRequest"),
        "agent-tool-catalog-projection": ("AgentToolDefinition", "AgentTool"),
        "agent-mutation": ("CreateAgentRequest|UpdateAgentRequest", "Record"),
    }
    mutations = require_list(sidecar.get("mutationContracts"), "frontend_mutation_contract_invalid", nonempty=True)
    mutation_index: dict[str, dict[str, Any]] = {}
    for raw in mutations:
        row = require_object(raw, "frontend_mutation_contract_invalid")
        safe_exposure_source(row.get("source"))
        mutation_id = row.get("id")
        fields = row.get("fields")
        if not isinstance(mutation_id, str) or mutation_id in mutation_index or not isinstance(fields, list) or not fields:
            fail("frontend_mutation_contract_invalid")
        mutation_index[mutation_id] = row
    if set(mutation_index) != set(expected_mutations):
        fail("frontend_mutation_contract_invalid")
    for mutation_id, (input_type, output_type) in expected_mutations.items():
        row = mutation_index[mutation_id]
        if (
            row.get("inputType") != input_type
            or row.get("outputType") != output_type
            or row.get("capabilityIdOmitted") is not True
            or "capabilityId" in row["fields"]
        ):
            fail("frontend_mutation_capability_identity")
    return len(mutations)


def validate_projection_provider(sidecar: dict[str, Any]) -> None:
    provider = require_object(sidecar.get("projectionProvider"), "frontend_projection_provider_invalid")
    safe_exposure_source(provider.get("source"))
    expected = {
        "component": "ReleaseProjectionProvider",
        "responseType": "AppCapabilityProjectionResponse",
        "operationId": "getAppReadinessCapabilities",
        "authSource": "useAppContext",
        "authenticatedStatus": "authenticated",
        "stateSource": "api.load",
        "props": ["children"],
    }
    if {key: provider.get(key) for key in expected} != expected or set(provider) != {"source", *expected}:
        fail("frontend_projection_provider_invalid")
    operations = require_list(sidecar.get("operations"), "frontend_operation_inventory_empty", nonempty=True)
    if sum(row.get("operation", {}).get("operationId") == "getAppReadinessCapabilities" for row in operations if isinstance(row, dict)) != 1:
        fail("frontend_projection_provider_operation_invalid")


def verify_exposure(
    sidecar_path: Path, contract_path: Path, app_projection_path: Path, server_catalog_path: Path
) -> dict[str, Any]:
    sidecar, sidecar_bytes = load_object(sidecar_path, "frontend_sidecar_invalid")
    contract, _ = load_object(contract_path, "frontend_release_contract_invalid")
    app_projection, _ = load_object(app_projection_path, "frontend_app_projection_schema_invalid")
    server_catalog, _ = load_object(server_catalog_path, "frontend_catalog_schema_invalid")
    generated_consumers = sidecar.get("generatedConsumers")
    if not isinstance(generated_consumers, int) or isinstance(generated_consumers, bool) or generated_consumers <= 0:
        fail("frontend_generated_consumer_mismatch")
    source_digest, config_digest = validate_sidecar_header(sidecar, generated_consumers)
    if require_list(sidecar.get("policyViolations"), "frontend_exposure_sidecar_invalid"):
        fail("frontend_client_capability_authority")
    projected, capabilities = project_release_contract(contract)
    release_projection = require_object(sidecar.get("releaseProjection"), "frontend_release_projection_mismatch")
    if set(release_projection) != {"digest", "capabilities", "surfaces"}:
        fail("frontend_release_projection_mismatch")
    if (
        release_projection.get("capabilities") != projected["capabilities"]
        or release_projection.get("surfaces") != projected["surfaces"]
        or release_projection.get("digest") != canonical_digest(projected)
    ):
        fail("frontend_release_projection_mismatch")
    identity, app_index = validate_app_projection(app_projection, capabilities)
    catalog = validate_catalog(server_catalog, contract, identity, capabilities)
    mutation_count = validate_dto_and_mutation_contracts(sidecar)
    validate_projection_provider(sidecar)

    exposures = require_list(sidecar.get("exposures"), "frontend_exposure_inventory_empty", nonempty=True)
    navigation_count = 0
    selector_count = 0
    availability_guards: set[tuple[str, str | None]] = set()
    selector_subjects: set[str] = set()
    navigation_rows: list[tuple[str, str, str]] = []
    for raw in exposures:
        row = require_object(raw, "frontend_exposure_sidecar_invalid")
        if set(row) != {"source", "kind", "surfaceKind", "productPath", "catalogSubject", "capabilityId", "capabilitySource"}:
            fail("frontend_exposure_sidecar_invalid")
        source = safe_exposure_source(row.get("source"))
        kind = row.get("kind")
        if kind in {"navigation", "link"}:
            capability_id = row.get("capabilityId")
            if not isinstance(row.get("productPath"), str) or not isinstance(capability_id, str) or capability_id not in capabilities:
                fail("frontend_navigation_capability_invalid")
            if capabilities[capability_id]["commitment"] == "excluded":
                fail("frontend_excluded_capability_exposed")
            navigation_rows.append((source[0], str(row.get("surfaceKind")), capability_id))
            navigation_count += 1
        elif kind == "selector":
            subject = row.get("catalogSubject")
            if subject not in {"ModelOption.capabilityId", "AgentToolDefinition.capabilityId"}:
                if isinstance(subject, str) and subject.startswith("Admin"):
                    fail("frontend_admin_inventory_exposed")
                fail("frontend_catalog_selector_identity_invalid")
            selector_subjects.add(subject)
            selector_count += 1
        elif kind == "availability-guard":
            availability_guards.add((source[0], row.get("catalogSubject")))
        else:
            fail("frontend_exposure_kind_unknown")
    if selector_subjects != {"ModelOption.capabilityId", "AgentToolDefinition.capabilityId"}:
        fail("frontend_catalog_selector_identity_invalid")
    if navigation_count == 0:
        fail("frontend_navigation_inventory_empty")
    provider_file = sidecar["projectionProvider"]["source"]["file"]
    for source_file, surface_kind, capability_id in navigation_rows:
        if capabilities[capability_id]["commitment"] != "conditional":
            continue
        guarded = any(file == source_file for file, _ in availability_guards)
        if surface_kind == "router":
            guarded = guarded or any(file == provider_file for file, _ in availability_guards)
        if not guarded:
            fail("frontend_conditional_exposure_unguarded")

    selectable = [
        row
        for row in catalog
        if row.get("subjectKind") in {"model", "tool"}
        and app_index.get(str(row.get("capabilityId")), {}).get("enabled") is True
        and capabilities[str(row.get("capabilityId"))]["commitment"] != "excluded"
    ]
    if not selectable:
        fail("frontend_catalog_selectable_inventory_empty")
    projection_digest = canonical_digest(
        {
            "generatedProjectionDigest": release_projection["digest"],
            "appProjectionDigest": app_projection["projectionDigest"],
            "catalogDigest": canonical_digest(catalog),
            "selectorSubjects": sorted(selector_subjects),
            "mutationContractCount": mutation_count,
            "selectableCatalogSubjectCount": len(selectable),
        }
    )
    return {
        "schemaVersion": EXPOSURE_SCHEMA,
        "sidecarDigest": digest_bytes(sidecar_bytes),
        "sourceDigest": source_digest,
        "configDigest": config_digest,
        "exposureCount": len(exposures),
        "catalogCount": len(catalog),
        "navigationCount": navigation_count,
        "generatedConsumerCount": generated_consumers,
        "projectionDigest": projection_digest,
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
    exposure = subparsers.add_parser("exposure")
    exposure.add_argument("--sidecar", required=True)
    exposure.add_argument("--contract", required=True)
    exposure.add_argument("--app-projection", required=True)
    exposure.add_argument("--server-catalog", required=True)
    exposure.add_argument("--output", required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "transport":
            result = verify_transport(Path(args.sidecar), Path(args.manifest))
        elif args.command == "exposure":
            result = verify_exposure(
                Path(args.sidecar),
                Path(args.contract),
                Path(args.app_projection),
                Path(args.server_catalog),
            )
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
