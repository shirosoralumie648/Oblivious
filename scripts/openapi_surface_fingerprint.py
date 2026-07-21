#!/usr/bin/env python3
"""Project the authored OpenAPI surface into deterministic compare-only artifacts.

OpenAPI remains the HTTP schema authority.  This module validates explicit public
operation dispositions and capability references, then emits closed metadata
projections for runtime and frontend consumers.  It never derives runtime mounts.
"""

from __future__ import annotations

import argparse
import copy
import hashlib
import json
import os
import re
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable

try:
    import jsonschema
    import yaml
except ImportError as exc:  # pragma: no cover - broken environment only.
    print(f"[operation-surface] dependency_unavailable name={exc.name}", file=sys.stderr)
    raise SystemExit(1) from exc


HTTP_METHODS = ("delete", "get", "head", "options", "patch", "post", "put", "trace")
MANDATORY_PREFIXES = ("/api/", "/v1/")
DISPOSITION_EXTENSION = "x-oblivious-public-operation"
CAPABILITY_EXTENSION = "x-oblivious-capability-id"
TERMINAL_SUCCESS_EXTENSION = "x-oblivious-terminal-success-statuses"
OPERATION_ID_OVERRIDES_EXTENSION = "x-oblivious-operation-id-overrides"
SCHEMA_VERSION = "route-surface-manifest/v2"
SCOPE_VERSION = "public-operation-scope/v1"
FIXTURE_VERSION = "operation-surface-fixtures/v1"
SHA256_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
PATH_PARAMETER_RE = re.compile(r"\{([^{}]+)\}")


@dataclass(frozen=True)
class ProjectionError(Exception):
    code: str
    operation_id: str = "none"
    field: str = "none"
    count: int = 1

    def __str__(self) -> str:
        return (
            f"[operation-surface] {self.code} operationId={self.operation_id} "
            f"field={self.field} count={self.count}"
        )


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode("utf-8")


def canonical_digest(value: Any) -> str:
    return "sha256:" + hashlib.sha256(canonical_json(value)).hexdigest()


def pretty_json(value: Any) -> bytes:
    return (json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode("utf-8")


def load_yaml(path: Path) -> dict[str, Any]:
    try:
        value = yaml.safe_load(path.read_text(encoding="utf-8"))
    except (OSError, yaml.YAMLError) as exc:
        raise ProjectionError("openapi_invalid", field="document") from exc
    if not isinstance(value, dict):
        raise ProjectionError("openapi_invalid", field="document")
    return value


def load_json(path: Path, *, code: str = "json_invalid") -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ProjectionError(code, field="document") from exc
    if not isinstance(value, dict):
        raise ProjectionError(code, field="document")
    return value


def resolve_ref(document: dict[str, Any], ref: str, *, operation_id: str, field: str) -> Any:
    if not ref.startswith("#/"):
        raise ProjectionError("unresolved_ref", operation_id, field)
    current: Any = document
    try:
        for encoded in ref[2:].split("/"):
            key = encoded.replace("~1", "/").replace("~0", "~")
            current = current[key]
    except (KeyError, TypeError) as exc:
        raise ProjectionError("unresolved_ref", operation_id, field) from exc
    return current


def validate_nested_refs(document: dict[str, Any], value: Any, *, operation_id: str, field: str) -> None:
    if isinstance(value, list):
        for item in value:
            validate_nested_refs(document, item, operation_id=operation_id, field=field)
        return
    if not isinstance(value, dict):
        return
    ref = value.get("$ref")
    if ref is not None:
        if not isinstance(ref, str):
            raise ProjectionError("unresolved_ref", operation_id, field)
        resolve_ref(document, ref, operation_id=operation_id, field=field)
    for nested in value.values():
        validate_nested_refs(document, nested, operation_id=operation_id, field=field)


def normalize_path(path: str, *, operation_id: str = "none") -> str:
    if not path.startswith("/") or "//" in path:
        raise ProjectionError("path_invalid", operation_id, "normalizedPath")

    def normalize_parameter(match: re.Match[str]) -> str:
        name = match.group(1).strip()
        if not name or not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", name):
            raise ProjectionError("path_invalid", operation_id, "normalizedPath")
        return "{" + name + "}"

    normalized = PATH_PARAMETER_RE.sub(normalize_parameter, path)
    if normalized != "/" and normalized.endswith("/"):
        normalized = normalized[:-1]
    if "{" in normalized.replace("{" + "x" + "}", "") and normalized.count("{") != normalized.count("}"):
        raise ProjectionError("path_invalid", operation_id, "normalizedPath")
    return normalized


def iter_operations(spec: dict[str, Any]) -> Iterable[tuple[str, str, dict[str, Any]]]:
    paths = spec.get("paths")
    if not isinstance(paths, dict) or not paths:
        raise ProjectionError("operation_inventory_empty", field="paths", count=0)
    count = 0
    for raw_path, raw_item in paths.items():
        if not isinstance(raw_path, str) or not isinstance(raw_item, dict):
            raise ProjectionError("path_item_invalid", field="paths")
        item = raw_item
        operation_id_overrides: dict[str, Any] = {}
        if "$ref" in item:
            raw_overrides = item.get(OPERATION_ID_OVERRIDES_EXTENSION, {})
            if not isinstance(raw_overrides, dict):
                raise ProjectionError("operation_id_override_invalid", field=raw_path)
            operation_id_overrides = raw_overrides
            item = resolve_ref(spec, item["$ref"], operation_id="none", field="pathItem")
        if not isinstance(item, dict):
            raise ProjectionError("path_item_invalid", field="paths")
        unknown_overrides = set(operation_id_overrides) - {
            method for method in HTTP_METHODS if isinstance(item.get(method), dict)
        }
        if unknown_overrides:
            raise ProjectionError(
                "operation_id_override_invalid", field=raw_path, count=len(unknown_overrides)
            )
        for method in HTTP_METHODS:
            operation = item.get(method)
            if operation is None:
                continue
            if not isinstance(operation, dict):
                raise ProjectionError("operation_invalid", field=method)
            if method in operation_id_overrides:
                override = operation_id_overrides[method]
                if not isinstance(override, str) or not override.strip():
                    raise ProjectionError("operation_id_override_invalid", field=raw_path)
                operation = copy.deepcopy(operation)
                operation["operationId"] = override.strip()
            count += 1
            yield raw_path, method.upper(), operation
    if count == 0:
        raise ProjectionError("operation_inventory_empty", field="paths", count=0)


def security_kind(operation: dict[str, Any], spec: dict[str, Any], *, operation_id: str) -> tuple[str, bool]:
    security = operation.get("security", spec.get("security"))
    if security == []:
        return "public", False
    if not isinstance(security, list) or not security:
        raise ProjectionError("security_invalid", operation_id, "security")
    names = {
        name
        for alternative in security
        if isinstance(alternative, dict)
        for name in alternative
        if isinstance(name, str)
    }
    if "cookieAuth" in names and "csrfHeader" in names:
        return "cookie+csrf", True
    if "bearerAuth" in names:
        return "bearer", False
    if "cookieAuth" in names:
        return "cookie", False
    raise ProjectionError("security_invalid", operation_id, "security")


def schema_identity(spec: dict[str, Any], schema: Any, *, operation_id: str, field: str) -> dict[str, Any]:
    if schema is None:
        return {"kind": "none", "value": None}
    if not isinstance(schema, dict) or not schema:
        raise ProjectionError("schema_unrepresentable", operation_id, field)
    validate_nested_refs(spec, schema, operation_id=operation_id, field=field)
    if set(schema) == {"$ref"}:
        ref = schema["$ref"]
        if not isinstance(ref, str):
            raise ProjectionError("schema_unrepresentable", operation_id, field)
        return {"kind": "ref", "value": ref}
    return {"kind": "inline", "value": canonical_digest(schema)}


def resolve_component_object(
    spec: dict[str, Any], value: Any, *, operation_id: str, field: str
) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ProjectionError("component_invalid", operation_id, field)
    if "$ref" in value:
        value = resolve_ref(spec, value["$ref"], operation_id=operation_id, field=field)
    if not isinstance(value, dict):
        raise ProjectionError("component_invalid", operation_id, field)
    return value


def project_request(spec: dict[str, Any], operation: dict[str, Any], *, operation_id: str) -> dict[str, Any]:
    request_body = operation.get("requestBody")
    if request_body is None:
        return {"mediaType": None, "schemaIdentity": {"kind": "none", "value": None}}
    request_body = resolve_component_object(spec, request_body, operation_id=operation_id, field="request")
    content = request_body.get("content", {})
    if not isinstance(content, dict):
        raise ProjectionError("request_content_invalid", operation_id, "request.mediaType")
    if len(content) > 1:
        raise ProjectionError("request_content_ambiguous", operation_id, "request.mediaType", len(content))
    if not content:
        return {"mediaType": None, "schemaIdentity": {"kind": "none", "value": None}}
    media_type, media = next(iter(content.items()))
    if not isinstance(media_type, str) or not media_type.strip() or not isinstance(media, dict):
        raise ProjectionError("request_content_invalid", operation_id, "request.mediaType")
    return {
        "mediaType": media_type.lower(),
        "schemaIdentity": schema_identity(
            spec, media.get("schema"), operation_id=operation_id, field="request.schemaIdentity"
        ),
    }


def project_success_responses(
    spec: dict[str, Any], operation: dict[str, Any], *, operation_id: str
) -> list[dict[str, Any]]:
    responses = operation.get("responses")
    if not isinstance(responses, dict) or not responses:
        raise ProjectionError("responses_invalid", operation_id, "successResponses")
    success_statuses = {str(status) for status in responses if re.fullmatch(r"2[0-9][0-9]", str(status))}
    terminal = operation.get(TERMINAL_SUCCESS_EXTENSION, [])
    if not isinstance(terminal, list) or any(not isinstance(item, (str, int)) for item in terminal):
        raise ProjectionError("terminal_success_invalid", operation_id, "successResponses.status")
    for raw_status in terminal:
        status = str(raw_status)
        if status not in responses:
            raise ProjectionError("terminal_success_invalid", operation_id, "successResponses.status")
        success_statuses.add(status)
    if not success_statuses:
        raise ProjectionError("operation_has_no_success", operation_id, "successResponses", count=0)

    projected: list[dict[str, Any]] = []
    for status in sorted(success_statuses, key=lambda item: (len(item), item)):
        response = resolve_component_object(
            spec, responses[status], operation_id=operation_id, field=f"successResponses.{status}"
        )
        content = response.get("content", {})
        if not isinstance(content, dict):
            raise ProjectionError("response_content_invalid", operation_id, f"successResponses.{status}")
        if not content:
            projected.append(
                {
                    "status": status,
                    "mediaType": None,
                    "schemaIdentity": {"kind": "none", "value": None},
                }
            )
            continue
        for media_type, media in sorted(content.items()):
            if not isinstance(media_type, str) or not media_type.strip() or not isinstance(media, dict):
                raise ProjectionError(
                    "response_content_invalid", operation_id, f"successResponses.{status}.mediaType"
                )
            projected.append(
                {
                    "status": status,
                    "mediaType": media_type.lower(),
                    "schemaIdentity": schema_identity(
                        spec,
                        media.get("schema"),
                        operation_id=operation_id,
                        field=f"successResponses.{status}.{media_type}.schemaIdentity",
                    ),
                }
            )
    return sorted(
        projected,
        key=lambda item: (
            item["status"],
            item["mediaType"] or "",
            item["schemaIdentity"]["kind"],
            item["schemaIdentity"]["value"] or "",
        ),
    )


def _default_sample_path(path: str) -> str:
    def sample_value(match: re.Match[str]) -> str:
        name = match.group(1)
        stem = re.sub(r"(?<!^)(?=[A-Z])", "_", name).lower()
        if stem.endswith("_id"):
            stem = stem[:-3]
        return f"{stem}_1"

    return PATH_PARAMETER_RE.sub(sample_value, path)


def _default_query_value(parameter: dict[str, Any]) -> str:
    schema = parameter.get("schema", {})
    if not isinstance(schema, dict):
        return "value_1"
    if schema.get("format") == "date-time":
        return "1970-01-01T00:00:00Z"
    if schema.get("format") == "date":
        return "1970-01-01"
    if schema.get("type") in {"integer", "number"}:
        return "1"
    if schema.get("type") == "boolean":
        return "true"
    return f"{parameter.get('name', 'value')}_1"


def _sample_query(spec: dict[str, Any], operation: dict[str, Any], *, operation_id: str) -> dict[str, str] | None:
    result: dict[str, str] = {}
    parameters = operation.get("parameters", [])
    if not isinstance(parameters, list):
        raise ProjectionError("operation_parameters_invalid", operation_id, "parameters")
    for raw_parameter in parameters:
        parameter = resolve_component_object(
            spec, raw_parameter, operation_id=operation_id, field="parameters"
        )
        name = parameter.get("name")
        if parameter.get("in") != "query" or not isinstance(name, str):
            continue
        if parameter.get("required") is True or name in {"from", "to"}:
            result[name] = _default_query_value(parameter)
    return dict(sorted(result.items())) or None


def project_openapi(
    spec: dict[str, Any], contract: dict[str, Any]
) -> dict[str, Any]:
    capability_rows = contract.get("capabilities")
    if not isinstance(capability_rows, list) or not capability_rows:
        raise ProjectionError("capability_contract_invalid", field="capabilities")
    capabilities = {
        row.get("id")
        for row in capability_rows
        if isinstance(row, dict) and isinstance(row.get("id"), str)
    }
    if len(capabilities) != len(capability_rows):
        raise ProjectionError("capability_contract_invalid", field="capabilities")

    dispositions: list[dict[str, Any]] = []
    operations: list[dict[str, Any]] = []
    route_samples: list[dict[str, Any]] = []
    seen_keys: set[tuple[str, str]] = set()
    operation_ids: set[str] = set()

    for raw_path, method, operation in iter_operations(spec):
        operation_id = operation.get("operationId")
        if not isinstance(operation_id, str) or not operation_id.strip():
            raise ProjectionError("operation_id_empty", field="operationId")
        operation_id = operation_id.strip()
        path = normalize_path(raw_path, operation_id=operation_id)
        key = (method, path)
        if key in seen_keys:
            raise ProjectionError("operation_duplicate", operation_id, "method+normalizedPath")
        if operation_id in operation_ids:
            raise ProjectionError("operation_id_duplicate", operation_id, "operationId")
        seen_keys.add(key)
        operation_ids.add(operation_id)

        disposition = operation.get(DISPOSITION_EXTENSION)
        if not isinstance(disposition, dict) or set(disposition) != {"disposition", "reason"}:
            raise ProjectionError("disposition_missing", operation_id, DISPOSITION_EXTENSION)
        disposition_value = disposition.get("disposition")
        reason = disposition.get("reason")
        if disposition_value not in {"included", "excluded"} or not isinstance(reason, str) or not reason.strip():
            raise ProjectionError("disposition_invalid", operation_id, DISPOSITION_EXTENSION)
        mandatory = path.startswith(MANDATORY_PREFIXES)
        if mandatory and disposition_value != "included":
            raise ProjectionError("mandatory_operation_excluded", operation_id, DISPOSITION_EXTENSION)

        disposition_row = {
            "method": method,
            "normalizedPath": path,
            "disposition": disposition_value,
            "reason": reason.strip(),
        }
        dispositions.append(disposition_row)

        sample_row: dict[str, Any] = {
            "method": method,
            "normalizedPath": path,
            "samplePath": _default_sample_path(path),
            "tags": operation.get("tags", []),
        }
        sample_query = _sample_query(spec, operation, operation_id=operation_id)
        if sample_query:
            sample_row["sampleQuery"] = sample_query
        if not isinstance(sample_row["samplePath"], str) or "{" in sample_row["samplePath"]:
            raise ProjectionError("sample_path_invalid", operation_id, "samplePath")
        if not isinstance(sample_row["tags"], list) or not sample_row["tags"]:
            raise ProjectionError("operation_tags_empty", operation_id, "tags")
        route_samples.append(sample_row)

        if disposition_value == "excluded":
            continue
        capability_id = operation.get(CAPABILITY_EXTENSION)
        if not isinstance(capability_id, str) or capability_id not in capabilities:
            raise ProjectionError("unknown_capability", operation_id, CAPABILITY_EXTENSION)
        security, csrf = security_kind(operation, spec, operation_id=operation_id)
        operations.append(
            {
                "method": method,
                "normalizedPath": path,
                "operationId": operation_id,
                "security": security,
                "csrf": csrf,
                "capabilityId": capability_id,
                "request": project_request(spec, operation, operation_id=operation_id),
                "successResponses": project_success_responses(spec, operation, operation_id=operation_id),
            }
        )

    dispositions.sort(key=lambda item: (item["normalizedPath"], item["method"]))
    operations.sort(key=lambda item: (item["normalizedPath"], item["method"], item["operationId"]))
    route_samples.sort(key=lambda item: (item["normalizedPath"], item["method"]))
    if not operations:
        raise ProjectionError("included_operation_inventory_empty", field="operations", count=0)
    scope = {
        "schemaVersion": SCOPE_VERSION,
        "mandatoryPrefixes": list(MANDATORY_PREFIXES),
        "dispositions": dispositions,
    }
    projection_digest = canonical_digest({"scope": scope, "operations": operations})
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generatedFrom": "docs/api/openapi.yaml",
        "projectionDigest": projection_digest,
        "scope": scope,
        "operations": operations,
        "routeSamples": route_samples,
    }


def validate_manifest(manifest: dict[str, Any], schema: dict[str, Any]) -> None:
    try:
        jsonschema.Draft202012Validator(schema).validate(manifest)
    except jsonschema.ValidationError as exc:
        field = ".".join(str(part) for part in exc.absolute_path) or "document"
        raise ProjectionError("manifest_schema_invalid", field=field) from exc

    operations = manifest.get("operations", [])
    dispositions = manifest.get("scope", {}).get("dispositions", [])
    route_samples = manifest.get("routeSamples", [])
    operation_ids = [row.get("operationId") for row in operations]
    operation_keys = [(row.get("method"), row.get("normalizedPath")) for row in operations]
    disposition_keys = [(row.get("method"), row.get("normalizedPath")) for row in dispositions]
    sample_keys = [(row.get("method"), row.get("normalizedPath")) for row in route_samples]
    if len(set(operation_ids)) != len(operation_ids):
        raise ProjectionError("manifest_operation_id_duplicate", field="operationId")
    if len(set(operation_keys)) != len(operation_keys):
        raise ProjectionError("manifest_operation_duplicate", field="method+normalizedPath")
    if len(set(disposition_keys)) != len(disposition_keys):
        raise ProjectionError("manifest_disposition_duplicate", field="method+normalizedPath")
    if len(set(sample_keys)) != len(sample_keys):
        raise ProjectionError("manifest_sample_duplicate", field="method+normalizedPath")
    if set(sample_keys) != set(disposition_keys):
        raise ProjectionError(
            "manifest_sample_scope_mismatch",
            field="routeSamples",
            count=len(set(sample_keys).symmetric_difference(set(disposition_keys))),
        )
    included_keys = {
        (row["method"], row["normalizedPath"])
        for row in dispositions
        if row["disposition"] == "included"
    }
    if included_keys != set(operation_keys):
        raise ProjectionError(
            "manifest_scope_mismatch",
            field="scope.dispositions",
            count=len(included_keys.symmetric_difference(set(operation_keys))),
        )
    for operation in operations:
        response_keys = [
            (
                response["status"],
                response["mediaType"],
                canonical_json(response["schemaIdentity"]),
            )
            for response in operation["successResponses"]
        ]
        if len(set(response_keys)) != len(response_keys):
            raise ProjectionError(
                "manifest_success_response_duplicate",
                operation["operationId"],
                "successResponses",
            )
    digest = canonical_digest({"scope": manifest["scope"], "operations": operations})
    if manifest.get("projectionDigest") != digest:
        raise ProjectionError("manifest_digest_mismatch", field="projectionDigest")


def _flatten(value: Any, prefix: str = "") -> dict[str, Any]:
    if isinstance(value, dict):
        result: dict[str, Any] = {}
        for key in sorted(value):
            next_prefix = f"{prefix}.{key}" if prefix else str(key)
            result.update(_flatten(value[key], next_prefix))
        return result
    if isinstance(value, list):
        result = {}
        for index, item in enumerate(value):
            result.update(_flatten(item, f"{prefix}[{index}]"))
        if not value:
            result[prefix] = []
        return result
    return {prefix: value}


def projection_differences(expected: dict[str, Any], actual: dict[str, Any]) -> list[dict[str, str]]:
    differences: list[dict[str, str]] = []
    expected_operations = {row["operationId"]: row for row in expected.get("operations", [])}
    actual_operations = {row["operationId"]: row for row in actual.get("operations", [])}
    for operation_id in sorted(expected_operations.keys() - actual_operations.keys()):
        differences.append({"kind": "missing", "operationId": operation_id, "field": "operation"})
    for operation_id in sorted(actual_operations.keys() - expected_operations.keys()):
        differences.append({"kind": "extra", "operationId": operation_id, "field": "operation"})
    for operation_id in sorted(expected_operations.keys() & actual_operations.keys()):
        expected_flat = _flatten(expected_operations[operation_id])
        actual_flat = _flatten(actual_operations[operation_id])
        for field in sorted(set(expected_flat) | set(actual_flat)):
            if expected_flat.get(field) != actual_flat.get(field):
                differences.append({"kind": "incompatible", "operationId": operation_id, "field": field})

    def scope_key(row: dict[str, Any]) -> tuple[str, str]:
        return row["method"], row["normalizedPath"]

    expected_scope = {scope_key(row): row for row in expected.get("scope", {}).get("dispositions", [])}
    actual_scope = {scope_key(row): row for row in actual.get("scope", {}).get("dispositions", [])}
    for key in sorted(expected_scope.keys() - actual_scope.keys()):
        differences.append({"kind": "missing", "operationId": f"{key[0]} {key[1]}", "field": "disposition"})
    for key in sorted(actual_scope.keys() - expected_scope.keys()):
        differences.append({"kind": "extra", "operationId": f"{key[0]} {key[1]}", "field": "disposition"})
    for key in sorted(expected_scope.keys() & actual_scope.keys()):
        for field in ("disposition", "reason"):
            if expected_scope[key].get(field) != actual_scope[key].get(field):
                differences.append(
                    {"kind": "incompatible", "operationId": f"{key[0]} {key[1]}", "field": field}
                )
    return differences


def operation_contracts_typescript(manifest: dict[str, Any]) -> bytes:
    identities = {
        canonical_json(identity).decode("utf-8"): identity
        for operation in manifest["operations"]
        for identity in [
            operation["request"]["schemaIdentity"],
            *(response["schemaIdentity"] for response in operation["successResponses"]),
        ]
    }
    lines = [
        "// Generated by scripts/openapi_surface_fingerprint.py. Do not edit.",
        "",
        "export type SchemaIdentityV1 =",
        "  | { readonly kind: 'ref'; readonly value: string }",
        "  | { readonly kind: 'inline'; readonly value: `sha256:${string}` }",
        "  | { readonly kind: 'none'; readonly value: null };",
        "",
        "export type OperationContractMetadataV1 = {",
        "  readonly method: string;",
        "  readonly normalizedPath: string;",
        "  readonly operationId: string;",
        "  readonly security: 'public' | 'cookie' | 'cookie+csrf' | 'bearer';",
        "  readonly csrf: boolean;",
        "  readonly capabilityId: string;",
        "  readonly request: { readonly mediaType: string | null; readonly schemaIdentity: SchemaIdentityV1 };",
        "  readonly successResponses: readonly { readonly status: string; readonly mediaType: string | null; readonly schemaIdentity: SchemaIdentityV1 }[];",
        "};",
        "",
        "export type PublicOperationDispositionV1 = {",
        "  readonly method: string;",
        "  readonly normalizedPath: string;",
        "  readonly disposition: 'included' | 'excluded';",
        "  readonly reason: string;",
        "};",
        "",
        f"export const operationContractDigest = {json.dumps(manifest['projectionDigest'])} as const;",
        "",
        "export const publicOperationScope = "
        + json.dumps(manifest["scope"], ensure_ascii=False, indent=2, sort_keys=True)
        + " as const;",
        "",
        "export const schemaIdentities = "
        + json.dumps([identities[key] for key in sorted(identities)], ensure_ascii=False, indent=2, sort_keys=True)
        + " as const satisfies readonly SchemaIdentityV1[];",
        "",
        "export const operationContracts = "
        + json.dumps(manifest["operations"], ensure_ascii=False, indent=2, sort_keys=True)
        + " as const satisfies readonly OperationContractMetadataV1[];",
        "",
    ]
    exported_names: set[str] = set()
    for index, operation in enumerate(manifest["operations"]):
        raw_name = re.sub(r"[^A-Za-z0-9_$]", "_", operation["operationId"])
        if not re.match(r"^[A-Za-z_$]", raw_name):
            raw_name = "operation_" + raw_name
        export_name = raw_name + "OperationContract"
        if export_name in exported_names:
            raise ProjectionError(
                "typescript_export_duplicate", operation["operationId"], "operationId"
            )
        exported_names.add(export_name)
        lines.append(
            f"export const {export_name}: OperationContractMetadataV1 = operationContracts[{index}];"
        )
    lines.append("")
    return "\n".join(lines).encode("utf-8")


def release_projection_typescript(contract: dict[str, Any]) -> bytes:
    capabilities = {
        row["id"]: row
        for row in contract.get("capabilities", [])
        if isinstance(row, dict) and isinstance(row.get("id"), str)
    }
    capability_projection = []
    for capability_id, row in sorted(capabilities.items()):
        commitment = row.get("commitment")
        navigation = "visible" if commitment == "committed" else "conditional" if commitment == "conditional" else "hidden"
        projected = {
            "capabilityId": capability_id,
            "disposition": commitment,
            "navigationDisposition": navigation,
        }
        if row.get("reasonCode") is not None:
            projected["reasonCode"] = row["reasonCode"]
        capability_projection.append(projected)

    surfaces = []
    for row in sorted(contract.get("surfaceReferences", []), key=lambda item: item.get("id", "")):
        capability_ids = sorted(row.get("capabilityIds", []))
        commitments = {capabilities[item]["commitment"] for item in capability_ids if item in capabilities}
        disposition = "committed" if "committed" in commitments else "conditional" if "conditional" in commitments else "excluded"
        surfaces.append(
            {
                "surfaceId": row.get("id"),
                "canonicalSource": row.get("canonicalSource"),
                "consumer": row.get("consumer"),
                "capabilityIds": capability_ids,
                "disposition": disposition,
                "navigationDisposition": "visible" if disposition == "committed" else "conditional" if disposition == "conditional" else "hidden",
            }
        )
    payload = {"capabilities": capability_projection, "surfaces": surfaces}
    lines = [
        "// Generated by scripts/openapi_surface_fingerprint.py. Do not edit.",
        "",
        f"export const releaseProjectionDigest = {json.dumps(canonical_digest(payload))} as const;",
        "",
        "export const releaseCapabilityProjection = "
        + json.dumps(capability_projection, ensure_ascii=False, indent=2, sort_keys=True)
        + " as const;",
        "",
        "export const releaseSurfaceProjection = "
        + json.dumps(surfaces, ensure_ascii=False, indent=2, sort_keys=True)
        + " as const;",
        "",
    ]
    return "\n".join(lines).encode("utf-8")


def atomic_write(path: Path, content: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor, temp_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temp_name, path)
    finally:
        try:
            os.unlink(temp_name)
        except FileNotFoundError:
            pass


def generated_outputs(manifest: dict[str, Any], contract: dict[str, Any]) -> dict[str, bytes]:
    return {
        "manifest": pretty_json(manifest),
        "typescript": operation_contracts_typescript(manifest),
        "releaseProjection": release_projection_typescript(contract),
    }


def annotate_fixture_spec(spec: dict[str, Any]) -> None:
    # The authored document currently uses path-item aliases for compatibility
    # routes. Fixtures expand those aliases so each operation can carry the
    # explicit identity that the production projector requires.
    paths = spec.get("paths", {})
    for path, item in list(paths.items()):
        if isinstance(item, dict) and isinstance(item.get("$ref"), str):
            paths[path] = copy.deepcopy(
                resolve_ref(spec, item["$ref"], operation_id="none", field="pathItem")
            )
    seen_operation_ids: set[str] = set()
    for path, _method, operation in iter_operations(spec):
        operation_id = operation.get("operationId")
        if operation_id in seen_operation_ids:
            suffix = re.sub(r"[^A-Za-z0-9]+", "_", path).strip("_")
            operation["operationId"] = f"{operation_id}__{suffix}"
        seen_operation_ids.add(operation["operationId"])
        mandatory = path.startswith(MANDATORY_PREFIXES)
        operation[DISPOSITION_EXTENSION] = {
            "disposition": "included" if mandatory else "excluded",
            "reason": "mandatory_prefix" if mandatory else "fixture_operational_exclusion",
        }
        if mandatory:
            operation[CAPABILITY_EXTENSION] = "gateway.request_admission"
        statuses = {str(status) for status in operation.get("responses", {})}
        if not any(re.fullmatch(r"2[0-9][0-9]", status) for status in statuses):
            if operation.get("operationId") == "connectWorkspaceWebSocket" and "101" in statuses:
                operation[TERMINAL_SUCCESS_EXTENSION] = ["101"]
            elif operation.get("operationId") == "topupQuota" and "402" in statuses:
                operation[TERMINAL_SUCCESS_EXTENSION] = ["402"]


def _find_operation(spec: dict[str, Any], predicate: Callable[[str, str, dict[str, Any]], bool]) -> tuple[str, str, dict[str, Any]]:
    for path, method, operation in iter_operations(spec):
        if predicate(path, method, operation):
            return path, method, operation
    raise ProjectionError("fixture_selection_empty", field="operation", count=0)


def _write_yaml(path: Path, spec: dict[str, Any], *, sort_keys: bool = False) -> None:
    path.write_text(yaml.safe_dump(spec, allow_unicode=True, sort_keys=sort_keys), encoding="utf-8")


def _run_projector(script: Path, root: Path, mode: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [
            sys.executable,
            str(script),
            "--openapi",
            str(root / "openapi.yaml"),
            "--contract",
            str(root / "contract.json"),
            "--manifest",
            str(root / "manifest.json"),
            "--schema",
            str(root / "schema.json"),
            "--typescript",
            str(root / "operation-contracts.generated.ts"),
            "--release-projection",
            str(root / "release-projection.generated.ts"),
            mode,
        ],
        text=True,
        capture_output=True,
        check=False,
    )


def run_fixture_suite(args: argparse.Namespace, suite: str) -> dict[str, Any]:
    script = Path(__file__).resolve()
    original_spec = load_yaml(Path(args.openapi))
    annotate_fixture_spec(original_spec)
    counts: dict[str, int] = {}
    with tempfile.TemporaryDirectory(prefix="oblivious-operation-surface-") as temp:
        root = Path(temp)
        (root / "contract.json").write_bytes(Path(args.contract).read_bytes())
        (root / "schema.json").write_bytes(Path(args.schema).read_bytes())
        _write_yaml(root / "openapi.yaml", original_spec)
        initial = _run_projector(script, root, "--write")
        if initial.returncode != 0:
            raise ProjectionError("fixture_baseline_failed", field="write")
        baseline_files = {
            name: (root / name).read_bytes()
            for name in (
                "manifest.json",
                "operation-contracts.generated.ts",
                "release-projection.generated.ts",
            )
        }
        baseline_manifest = json.loads(baseline_files["manifest.json"])
        counts.update(
            {
                "apiOperations": sum(row["normalizedPath"].startswith("/api/") for row in baseline_manifest["operations"]),
                "v1Operations": sum(row["normalizedPath"].startswith("/v1/") for row in baseline_manifest["operations"]),
                "otherIncluded": sum(
                    row["disposition"] == "included"
                    and not row["normalizedPath"].startswith(MANDATORY_PREFIXES)
                    for row in baseline_manifest["scope"]["dispositions"]
                ),
                "otherExcluded": sum(
                    row["disposition"] == "excluded"
                    and not row["normalizedPath"].startswith(MANDATORY_PREFIXES)
                    for row in baseline_manifest["scope"]["dispositions"]
                ),
                "metadata": len(baseline_manifest["operations"]),
            }
        )
        if counts["apiOperations"] <= 0 or counts["otherExcluded"] <= 0 or counts["metadata"] <= 0:
            raise ProjectionError("fixture_count_zero", field="baseline", count=0)

        def expect_failure(name: str, mutate: Callable[[dict[str, Any]], None], code: str) -> None:
            candidate = copy.deepcopy(original_spec)
            mutate(candidate)
            _write_yaml(root / "openapi.yaml", candidate)
            result = _run_projector(script, root, "--check")
            if result.returncode == 0 or code not in result.stderr:
                raise ProjectionError("fixture_false_green", field=name, count=result.returncode)
            counts[name] = counts.get(name, 0) + 1

        request_selector = lambda _p, _m, op: isinstance(
            ((op.get("requestBody") or {}).get("content")), dict
        ) and bool((op.get("requestBody") or {}).get("content"))
        response_selector = lambda _p, _m, op: any(
            str(status).startswith("2") and isinstance(response, dict) and bool(response.get("content"))
            for status, response in (op.get("responses") or {}).items()
        )

        def mutate_operation(field_mutator: Callable[[str, str, dict[str, Any], dict[str, Any]], None]) -> Callable[[dict[str, Any]], None]:
            def mutate(spec: dict[str, Any]) -> None:
                path, method, operation = _find_operation(
                    spec,
                    lambda p, m, op: request_selector(p, m, op) and response_selector(p, m, op),
                )
                field_mutator(path, method, operation, spec)
            return mutate

        def mutate_secured_operation(spec: dict[str, Any]) -> None:
            path, method, operation = _find_operation(
                spec,
                lambda p, m, op: request_selector(p, m, op)
                and response_selector(p, m, op)
                and op.get("security") != [],
            )
            operation["security"] = []

        expect_failure(
            "methodMutations",
            mutate_operation(
                lambda path, method, operation, spec: (
                    spec["paths"][path].__setitem__("trace", operation),
                    spec["paths"][path].__delitem__(method.lower()),
                )
            ),
            "manifest_incompatible",
        )
        expect_failure(
            "pathMutations",
            mutate_operation(
                lambda path, method, operation, spec: (
                    spec["paths"].__setitem__(path + "/fixture-drift", {method.lower(): operation}),
                    spec["paths"][path].__delitem__(method.lower()),
                )
            ),
            "manifest_incompatible",
        )
        expect_failure(
            "operationIdMutations",
            mutate_operation(lambda _p, _m, operation, _s: operation.__setitem__("operationId", operation["operationId"] + "Drift")),
            "manifest_incompatible",
        )
        expect_failure(
            "securityMutations",
            mutate_secured_operation,
            "manifest_incompatible",
        )
        expect_failure(
            "csrfMutations",
            mutate_operation(
                lambda _p, _m, operation, _s: operation.__setitem__(
                    "security", [{"cookieAuth": [], "csrfHeader": []}]
                )
            ),
            "manifest_incompatible",
        )
        expect_failure(
            "capabilityMutations",
            mutate_operation(lambda _p, _m, operation, _s: operation.__setitem__(CAPABILITY_EXTENSION, "unknown.fixture")),
            "unknown_capability",
        )

        def mutate_request_media(_p: str, _m: str, operation: dict[str, Any], _s: dict[str, Any]) -> None:
            content = operation["requestBody"]["content"]
            media, value = next(iter(content.items()))
            del content[media]
            content["application/fixture+json"] = value

        expect_failure("requestMediaMutations", mutate_operation(mutate_request_media), "manifest_incompatible")
        expect_failure(
            "requestSchemaMutations",
            mutate_operation(
                lambda _p, _m, operation, _s: next(iter(operation["requestBody"]["content"].values())).__setitem__(
                    "schema", {"type": "string"}
                )
            ),
            "manifest_incompatible",
        )

        def first_success(operation: dict[str, Any]) -> tuple[str, dict[str, Any]]:
            return next(
                (str(status), response)
                for status, response in operation["responses"].items()
                if str(status).startswith("2") and isinstance(response, dict) and response.get("content")
            )

        def mutate_success_status(_p: str, _m: str, operation: dict[str, Any], _s: dict[str, Any]) -> None:
            status, response = first_success(operation)
            del operation["responses"][status]
            replacement = "299" if status != "299" else "298"
            operation["responses"][replacement] = response

        expect_failure("successStatusMutations", mutate_operation(mutate_success_status), "manifest_incompatible")

        def mutate_success_media(_p: str, _m: str, operation: dict[str, Any], _s: dict[str, Any]) -> None:
            _status, response = first_success(operation)
            media, value = next(iter(response["content"].items()))
            del response["content"][media]
            response["content"]["application/fixture+json"] = value

        expect_failure("successMediaMutations", mutate_operation(mutate_success_media), "manifest_incompatible")
        expect_failure(
            "successSchemaMutations",
            mutate_operation(
                lambda _p, _m, operation, _s: next(iter(first_success(operation)[1]["content"].values())).__setitem__(
                    "schema", {"type": "string"}
                )
            ),
            "manifest_incompatible",
        )
        expect_failure("emptyInventory", lambda spec: spec.__setitem__("paths", {}), "operation_inventory_empty")
        expect_failure(
            "duplicateOperationId",
            lambda spec: list(iter_operations(spec))[1][2].__setitem__(
                "operationId", list(iter_operations(spec))[0][2]["operationId"]
            ),
            "operation_id_duplicate",
        )
        expect_failure(
            "unresolvedRef",
            mutate_operation(
                lambda _p, _m, operation, _s: next(iter(operation["requestBody"]["content"].values())).__setitem__(
                    "schema", {"$ref": "#/components/schemas/FixtureMissing"}
                )
            ),
            "unresolved_ref",
        )
        expect_failure(
            "non2xxOnly",
            mutate_operation(lambda _p, _m, operation, _s: operation.__setitem__("responses", {"400": {"description": "fixture"}})),
            "operation_has_no_success",
        )
        expect_failure(
            "unrepresentableInlineSchema",
            mutate_operation(
                lambda _p, _m, operation, _s: next(iter(operation["requestBody"]["content"].values())).__setitem__(
                    "schema", {}
                )
            ),
            "schema_unrepresentable",
        )

        _write_yaml(root / "openapi.yaml", original_spec, sort_keys=True)
        equivalent = _run_projector(script, root, "--check")
        if equivalent.returncode != 0:
            raise ProjectionError("canonical_equivalence_failed", field="openapiMapOrder")
        counts["canonicalEquivalence"] = 1

        if suite in {"determinism", "all"}:
            _write_yaml(root / "openapi.yaml", original_spec)
            first = _run_projector(script, root, "--write")
            if first.returncode != 0:
                raise ProjectionError("determinism_write_failed", field="first")
            first_bytes = {name: (root / name).read_bytes() for name in baseline_files}
            second = _run_projector(script, root, "--write")
            if second.returncode != 0:
                raise ProjectionError("determinism_write_failed", field="second")
            for name, value in first_bytes.items():
                if (root / name).read_bytes() != value:
                    raise ProjectionError("determinism_mismatch", field=name)
            counts["deterministicGenerations"] = 2
            contract = json.loads((root / "contract.json").read_text(encoding="utf-8"))
            counts["releaseCapabilities"] = len(contract.get("capabilities", []))
            counts["releaseSurfaces"] = len(contract.get("surfaceReferences", []))
            if counts["releaseCapabilities"] <= 0 or counts["releaseSurfaces"] <= 0:
                raise ProjectionError("fixture_count_zero", field="releaseProjection", count=0)

        if suite == "all":
            _write_yaml(root / "openapi.yaml", original_spec)
            (root / "operation-contracts.generated.ts").write_text("// drift\n", encoding="utf-8")
            stale = _run_projector(script, root, "--check")
            if stale.returncode == 0 or "artifact_stale" not in stale.stderr:
                raise ProjectionError("fixture_false_green", field="typedExportDrift")
            counts["typedExportDrift"] = 1

            def expect_manifest_failure(
                name: str, mutate: Callable[[dict[str, Any]], None], code: str
            ) -> None:
                for filename, content in baseline_files.items():
                    (root / filename).write_bytes(content)
                candidate = json.loads(baseline_files["manifest.json"])
                mutate(candidate)
                candidate["projectionDigest"] = canonical_digest(
                    {"scope": candidate["scope"], "operations": candidate["operations"]}
                )
                (root / "manifest.json").write_bytes(pretty_json(candidate))
                result = _run_projector(script, root, "--check")
                if result.returncode == 0 or code not in result.stderr:
                    raise ProjectionError("fixture_false_green", field=name)
                counts[name] = 1

            def remove_operation(candidate: dict[str, Any]) -> None:
                removed = candidate["operations"].pop(0)
                key = (removed["method"], removed["normalizedPath"])
                candidate["scope"]["dispositions"] = [
                    row
                    for row in candidate["scope"]["dispositions"]
                    if (row["method"], row["normalizedPath"]) != key
                ]
                candidate["routeSamples"] = [
                    row
                    for row in candidate["routeSamples"]
                    if (row["method"], row["normalizedPath"]) != key
                ]

            def add_operation(candidate: dict[str, Any]) -> None:
                operation = copy.deepcopy(candidate["operations"][0])
                operation.update(
                    {
                        "method": "GET",
                        "normalizedPath": "/api/v1/fixture-extra",
                        "operationId": "fixtureExtraOperation",
                    }
                )
                candidate["operations"].append(operation)
                candidate["scope"]["dispositions"].append(
                    {
                        "method": "GET",
                        "normalizedPath": "/api/v1/fixture-extra",
                        "disposition": "included",
                        "reason": "fixture_extra",
                    }
                )
                candidate["routeSamples"].append(
                    {
                        "method": "GET",
                        "normalizedPath": "/api/v1/fixture-extra",
                        "samplePath": "/api/v1/fixture-extra",
                        "tags": ["Fixture"],
                    }
                )

            expect_manifest_failure("manifestMissingOperation", remove_operation, "manifest_incompatible")
            expect_manifest_failure("manifestExtraOperation", add_operation, "manifest_incompatible")
            expect_manifest_failure(
                "manifestDuplicateOperation",
                lambda candidate: candidate["operations"].append(
                    copy.deepcopy(candidate["operations"][0])
                ),
                "manifest_operation_id_duplicate",
            )
            expect_manifest_failure(
                "manifestCoreDrift",
                lambda candidate: candidate["operations"][0].__setitem__(
                    "capabilityId", "release.contract_reporting"
                ),
                "manifest_incompatible",
            )

        required_families = [
            "methodMutations",
            "pathMutations",
            "operationIdMutations",
            "securityMutations",
            "csrfMutations",
            "capabilityMutations",
            "requestMediaMutations",
            "requestSchemaMutations",
            "successStatusMutations",
            "successMediaMutations",
            "successSchemaMutations",
        ]
        if any(counts.get(name, 0) <= 0 for name in required_families):
            raise ProjectionError("fixture_count_zero", field="mutationFamilies", count=0)
    return {"schemaVersion": FIXTURE_VERSION, "suite": suite, "counts": dict(sorted(counts.items()))}


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--openapi", default=str(repo_root / "docs/api/openapi.yaml"))
    parser.add_argument("--contract", default=str(repo_root / "config/release/contract.v1.json"))
    parser.add_argument("--manifest", default=str(repo_root / "docs/api/route-surface-manifest.json"))
    parser.add_argument("--schema", default=str(repo_root / "docs/api/route-surface-manifest.schema.json"))
    parser.add_argument(
        "--typescript", default=str(repo_root / "src/web/src/generated/operation-contracts.generated.ts")
    )
    parser.add_argument(
        "--release-projection", default=str(repo_root / "src/web/src/generated/release-projection.generated.ts")
    )
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--scope", action="store_true")
    mode.add_argument("--project", action="store_true")
    mode.add_argument("--validate", action="store_true")
    mode.add_argument("--digest", action="store_true")
    mode.add_argument("--write", action="store_true")
    mode.add_argument("--check", action="store_true")
    mode.add_argument("--fixture-suite", choices=("projector", "determinism", "all"))
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.fixture_suite:
            print(json.dumps(run_fixture_suite(args, args.fixture_suite), sort_keys=True))
            return 0
        spec = load_yaml(Path(args.openapi))
        contract = load_json(Path(args.contract), code="capability_contract_invalid")
        manifest_path = Path(args.manifest)
        projected = project_openapi(spec, contract)
        schema = load_json(Path(args.schema), code="manifest_schema_invalid")
        validate_manifest(projected, schema)

        if args.scope:
            sys.stdout.buffer.write(pretty_json(projected["scope"]))
        elif args.digest:
            print(projected["projectionDigest"])
        elif args.validate:
            print(
                json.dumps(
                    {
                        "schemaVersion": "operation-surface-validation/v1",
                        "operations": len(projected["operations"]),
                        "dispositions": len(projected["scope"]["dispositions"]),
                        "projectionDigest": projected["projectionDigest"],
                        "evidenceClass": "E1",
                    },
                    sort_keys=True,
                )
            )
        elif args.write:
            outputs = generated_outputs(projected, contract)
            atomic_write(manifest_path, outputs["manifest"])
            atomic_write(Path(args.typescript), outputs["typescript"])
            atomic_write(Path(args.release_projection), outputs["releaseProjection"])
            print(
                json.dumps(
                    {
                        "schemaVersion": "operation-surface-write/v1",
                        "operations": len(projected["operations"]),
                        "dispositions": len(projected["scope"]["dispositions"]),
                        "projectionDigest": projected["projectionDigest"],
                    },
                    sort_keys=True,
                )
            )
        elif args.check:
            if not manifest_path.exists():
                raise ProjectionError("artifact_missing", field="manifest")
            prior_manifest = load_json(manifest_path, code="manifest_json_invalid")
            validate_manifest(prior_manifest, schema)
            differences = projection_differences(projected, prior_manifest)
            if differences:
                first = differences[0]
                raise ProjectionError(
                    "manifest_incompatible",
                    first["operationId"],
                    first["field"],
                    len(differences),
                )
            outputs = generated_outputs(projected, contract)
            for label, path in (
                ("manifest", manifest_path),
                ("typescript", Path(args.typescript)),
                ("releaseProjection", Path(args.release_projection)),
            ):
                try:
                    actual = path.read_bytes()
                except OSError as exc:
                    raise ProjectionError("artifact_missing", field=label) from exc
                if actual != outputs[label]:
                    raise ProjectionError("artifact_stale", field=label)
            print(
                json.dumps(
                    {
                        "schemaVersion": "operation-surface-check/v1",
                        "operations": len(projected["operations"]),
                        "dispositions": len(projected["scope"]["dispositions"]),
                        "projectionDigest": projected["projectionDigest"],
                        "evidenceClass": "E1",
                    },
                    sort_keys=True,
                )
            )
        else:
            sys.stdout.buffer.write(pretty_json(projected))
        return 0
    except ProjectionError as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
