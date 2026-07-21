#!/usr/bin/env python3
"""Ruby-free OpenAPI contract verifier.

This script ports the checks that used to live as inline Ruby snippets in
scripts/verify-openapi-contract.sh. It intentionally stays data-driven where the
old verifier was data-driven, so the release gate still fails on the same broad
classes of OpenAPI drift: missing routes, envelope usage, CSRF/security surface,
route-surface parity, response/request schema refs, sensitive response fields,
and domain-specific required fields.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

from openapi_surface_fingerprint import (
    ProjectionError,
    generated_outputs,
    project_openapi,
    projection_differences,
    validate_manifest,
)

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised only in broken envs.
    print(
        "[openapi-contract] PyYAML is required: install python package 'yaml'",
        file=sys.stderr,
    )
    raise SystemExit(1) from exc


HTTP_METHODS = {"get", "post", "put", "patch", "delete"}
MUTATION_METHODS = {"post", "put", "patch", "delete"}


def dig(node: Any, *keys: Any) -> Any:
    current = node
    for key in keys:
        if isinstance(current, dict):
            current = current.get(key)
        elif isinstance(current, list) and isinstance(key, int):
            if key < 0 or key >= len(current):
                return None
            current = current[key]
        else:
            return None
    return current


def props(schema: Any) -> dict[str, Any]:
    return schema.get("properties", {}) if isinstance(schema, dict) else {}


def required(schema: Any) -> list[Any]:
    value = schema.get("required", []) if isinstance(schema, dict) else []
    return value if isinstance(value, list) else []


def enum_values(schema: Any, *keys: Any) -> list[Any]:
    value = dig(schema, *keys, "enum")
    return value if isinstance(value, list) else []


def includes_all(values: Any, expected: list[Any]) -> bool:
    return isinstance(values, list) and all(value in values for value in expected)


def ref_name(ref: str | None) -> str:
    return ref.rsplit("/", 1)[-1] if isinstance(ref, str) else ""


def inspect_value(value: Any) -> str:
    if value is None:
        return "nil"
    if isinstance(value, str):
        return repr(value)
    if isinstance(value, bool):
        return "true" if value else "false"
    return repr(value)


def resolve_ref(spec: dict[str, Any], ref: str) -> Any:
    if not ref.startswith("#/"):
        raise KeyError(ref)
    current: Any = spec
    for raw_part in ref[2:].split("/"):
        part = raw_part.replace("~1", "/").replace("~0", "~")
        current = current[part]
    return current


def operation(paths: dict[str, Any], path: str, method: str, missing: list[str]) -> dict[str, Any]:
    op = dig(paths, path, method)
    if not isinstance(op, dict):
        missing.append(f"{method.upper()} {path} must be documented")
        return {}
    return op


def response_data_ref(op: dict[str, Any], status: str) -> Any:
    all_of = dig(op, "responses", status, "content", "application/json", "schema", "allOf")
    if not isinstance(all_of, list):
        return None
    for entry in all_of:
        value = dig(entry, "properties", "data", "$ref")
        if value is not None:
            return value
    return None


def response_data_array_ref(op: dict[str, Any], status: str) -> Any:
    all_of = dig(op, "responses", status, "content", "application/json", "schema", "allOf")
    if not isinstance(all_of, list):
        return None
    for entry in all_of:
        value = dig(entry, "properties", "data", "items", "$ref")
        if value is not None:
            return value
    return None


def response_ref(op: dict[str, Any], status: str) -> Any:
    return dig(op, "responses", status, "content", "application/json", "schema", "$ref")


def request_body_ref(op: dict[str, Any], content_type: str = "application/json") -> Any:
    return dig(op, "requestBody", "content", content_type, "schema", "$ref")


def requires_cookie_and_csrf(op: dict[str, Any]) -> bool:
    security = op.get("security", [])
    return isinstance(security, list) and any(
        isinstance(entry, dict) and "cookieAuth" in entry and "csrfHeader" in entry for entry in security
    )


def requires_cookie_without_csrf(op: dict[str, Any]) -> bool:
    security = op.get("security", [])
    return isinstance(security, list) and any(
        isinstance(entry, dict) and "cookieAuth" in entry and "csrfHeader" not in entry for entry in security
    )


def requires_bearer(op: dict[str, Any], spec: dict[str, Any] | None = None) -> bool:
    security = op.get("security", spec.get("security", []) if spec else [])
    return isinstance(security, list) and any(
        isinstance(entry, dict) and "bearerAuth" in entry for entry in security
    )


def has_tags(op: dict[str, Any], *tags: str) -> bool:
    operation_tags = op.get("tags", [])
    return isinstance(operation_tags, list) and all(tag in operation_tags for tag in tags)


def fail(title: str, missing: list[str]) -> None:
    if not missing:
        return
    print(title, file=sys.stderr)
    for entry in missing:
        print(f"  - {entry}", file=sys.stderr)
    raise SystemExit(1)


def api_operations(spec: dict[str, Any]):
    for path, operations in spec.get("paths", {}).items():
        if not isinstance(path, str) or not path.startswith("/api/") or not isinstance(operations, dict):
            continue
        for method, op in operations.items():
            if method not in HTTP_METHODS or not isinstance(op, dict):
                continue
            yield path, method, op, operations


def one_of_requires(schema: Any, field: str) -> bool:
    return isinstance(schema, dict) and any(
        isinstance(entry, dict) and field in entry.get("required", []) for entry in schema.get("anyOf", [])
    )


def all_of_any_requires(schema: Any, field: str) -> bool:
    return isinstance(schema, dict) and any(one_of_requires(entry, field) for entry in schema.get("allOf", []))


def check_required_fields(
    schemas: dict[str, Any],
    schema_name: str,
    fields: list[str],
    missing: list[str],
    label: str | None = None,
) -> None:
    schema_required = required(schemas.get(schema_name, {}))
    label = label or schema_name
    for field in fields:
        if field not in schema_required:
            missing.append(f"{label} must require {field}")


def check_property_type(
    schemas: dict[str, Any],
    schema_name: str,
    fields: list[str],
    expected_type: str,
    missing: list[str],
    suffix: str | None = None,
) -> None:
    schema = schemas.get(schema_name, {})
    for field in fields:
        if dig(schema, "properties", field, "type") != expected_type:
            tail = suffix or f"as {expected_type}"
            missing.append(f"{schema_name}.{field} must be documented {tail}")


def check_expected_data_refs(
    paths: dict[str, Any],
    entries: dict[tuple[str, str, str], str],
    missing: list[str],
    *,
    tags: tuple[str, ...] = (),
    title: str = "data",
    read_cookie: bool = False,
) -> None:
    for (path, method, status), expected in entries.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
        for tag in tags:
            if not has_tags(op, tag):
                if len(tags) == 1:
                    missing.append(f"{method.upper()} {path} must be tagged {tag}")
                    break
            elif tag == tags[-1] and len(tags) > 1 and not has_tags(op, *tags):
                missing.append(f"{method.upper()} {path} must be tagged {' and '.join(tags)}")
        if read_cookie and method == "get" and not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")


def check_mutation_security(
    paths: dict[str, Any],
    entries: list[tuple[str, str]],
    missing: list[str],
    tag: str | None = None,
) -> None:
    for path, method in entries:
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if tag and not has_tags(op, tag):
            missing.append(f"{method.upper()} {path} must be tagged {tag}")


def check_read_security(
    paths: dict[str, Any],
    entries: list[tuple[str, str]],
    missing: list[str],
    tag: str | None = None,
) -> None:
    for path, method in entries:
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if tag and not has_tags(op, tag):
            missing.append(f"{method.upper()} {path} must be tagged {tag}")


def schema_refs_envelope(spec: dict[str, Any], schema: Any, seen: set[str] | None = None) -> bool:
    if seen is None:
        seen = set()
    if isinstance(schema, list):
        return any(schema_refs_envelope(spec, item, seen) for item in schema)
    if not isinstance(schema, dict):
        return False
    ref = schema.get("$ref")
    if ref:
        if ref == "#/components/schemas/Envelope":
            return True
        if ref in seen:
            return False
        seen.add(ref)
        return schema_refs_envelope(spec, resolve_ref(spec, ref), seen)
    return any(schema_refs_envelope(spec, value, set(seen)) for value in schema.values())


def response_refs_envelope(spec: dict[str, Any], response: Any, seen: set[str] | None = None) -> bool:
    if seen is None:
        seen = set()
    if isinstance(response, dict) and response.get("$ref"):
        ref = response["$ref"]
        if ref in seen:
            return False
        seen.add(ref)
        return response_refs_envelope(spec, resolve_ref(spec, ref), seen)
    if not isinstance(response, dict):
        return False
    json_content = dig(response, "content", "application/json")
    if not isinstance(json_content, dict):
        return True
    return schema_refs_envelope(spec, json_content.get("schema"))


def envelope_data_schema(schema: Any) -> Any:
    if not isinstance(schema, dict):
        return None
    all_of = schema.get("allOf", [])
    if not isinstance(all_of, list):
        return None
    for entry in all_of:
        value = dig(entry, "properties", "data")
        if value is not None:
            return value
    return None


def require_api_json_responses_use_envelope(spec: dict[str, Any]) -> None:
    raw_control_plane_operations = {
        ("get", "/api/v1/admin/readiness"),
        ("get", "/api/v1/app/readiness/capabilities"),
    }
    missing: list[str] = []
    for path, method, op, _operations in api_operations(spec):
        if path.startswith("/api/v1/relay/") or path.startswith("/api/v1/gateway/proxy/"):
            continue
        if (method, path) in raw_control_plane_operations:
            continue
        for status, response in op.get("responses", {}).items():
            if not response_refs_envelope(spec, response):
                missing.append(f"{method.upper()} {path} {status}")
    fail("[openapi-contract] /api JSON responses must reference #/components/schemas/Envelope:", missing)


def require_api_success_data_uses_named_schema(spec: dict[str, Any]) -> None:
    missing: list[str] = []
    for path, method, op, _operations in api_operations(spec):
        if path.startswith("/api/v1/relay/") or path.startswith("/api/v1/gateway/proxy/"):
            continue
        for status, response in op.get("responses", {}).items():
            if not str(status).startswith("2"):
                continue
            if isinstance(response, dict) and response.get("$ref"):
                response = resolve_ref(spec, response["$ref"])
            json_content = dig(response, "content", "application/json")
            if not isinstance(json_content, dict):
                continue
            data = envelope_data_schema(json_content.get("schema"))
            if not isinstance(data, dict):
                continue
            if data.get("$ref"):
                continue
            if data.get("type") == "array" and (dig(data, "items", "$ref") or dig(data, "items", "type")):
                continue
            missing.append(f"{method.upper()} {path} {status}")
    fail("[openapi-contract] /api 2xx JSON response data objects must use named component schemas:", missing)


def require_api_json_request_bodies_use_named_schemas(spec: dict[str, Any]) -> None:
    allowed_inline_bodies = {
        ("post", "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"): "public workflow webhook payload",
        ("post", "/api/v1/workflows/{workflowId}/webhook"): "session workflow webhook payload",
        ("post", "/api/v1/channels/webhook/{channelId}"): "public channel webhook payload",
        ("post", "/api/v1/billing/stripe/webhook"): "Stripe provider webhook payload",
    }
    missing: list[str] = []
    malformed: list[str] = []
    for path, method, op, _operations in api_operations(spec):
        schema = dig(op, "requestBody", "content", "application/json", "schema")
        if not isinstance(schema, dict) or schema.get("$ref"):
            continue
        reason = allowed_inline_bodies.get((method, path))
        if reason:
            if schema.get("type") != "object" or schema.get("additionalProperties") is not True:
                malformed.append(f"{method.upper()} {path} must remain an object with additionalProperties: true for {reason}")
            continue
        missing.append(f"{method.upper()} {path}")
    fail(
        "[openapi-contract] /api JSON request bodies must use named component schemas except approved webhook payloads:",
        missing + malformed,
    )


def require_api_security_surface_contract(spec: dict[str, Any]) -> None:
    public_mutations = {
        ("post", "/api/v1/auth/register"): "public auth registration",
        ("post", "/api/v1/auth/login"): "public auth login",
        ("post", "/api/v1/auth/password-reset/request"): "public password reset request",
        ("post", "/api/v1/auth/password-reset/confirm"): "public password reset confirmation",
        ("post", "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"): "public workflow webhook",
        ("post", "/api/v1/channels/webhook/{channelId}"): "public channel webhook",
        ("post", "/api/v1/billing/stripe/webhook"): "Stripe provider webhook",
        ("post", "/api/v1/billing/alipay/webhook"): "Alipay provider webhook",
        ("post", "/api/v1/billing/wechatpay/webhook"): "WeChat Pay provider webhook",
        ("post", "/api/v1/billing/marketplace-payout/webhook"): "marketplace payout provider webhook",
    }
    missing_security: list[str] = []
    missing_csrf: list[str] = []
    malformed_public: list[str] = []
    malformed_relay: list[str] = []
    for path, method, op, _operations in api_operations(spec):
        security = op.get("security")
        if security is None:
            missing_security.append(f"{method.upper()} {path}")
            continue
        if method not in MUTATION_METHODS:
            continue
        reason = public_mutations.get((method, path))
        if reason:
            if security != []:
                malformed_public.append(f"{method.upper()} {path} must declare security: [] for {reason}")
            continue
        if path.startswith("/api/v1/relay/"):
            if not requires_bearer(op):
                malformed_relay.append(f"{method.upper()} {path} must use bearerAuth for OpenAI-compatible Relay aliases")
            continue
        if not requires_cookie_and_csrf(op):
            missing_csrf.append(f"{method.upper()} {path}")
    messages = [f"{entry} must declare a security field" for entry in missing_security]
    messages += [f"{entry} must require cookieAuth plus csrfHeader" for entry in missing_csrf]
    messages += malformed_public + malformed_relay
    fail("[openapi-contract] /api security surface contract failed:", messages)


def require_api_path_parameter_contract(spec: dict[str, Any]) -> None:
    missing: list[str] = []
    malformed: list[str] = []
    extra: list[str] = []
    for path, method, op, operations in api_operations(spec):
        expected_names = re.findall(r"\{([^}]+)\}", path)
        shared_parameters = operations.get("parameters", []) if isinstance(operations, dict) else []
        parameters = []
        for parameter in list(shared_parameters) + list(op.get("parameters", [])):
            if isinstance(parameter, dict) and parameter.get("$ref"):
                parameter = resolve_ref(spec, parameter["$ref"])
            parameters.append(parameter)
        path_parameters = [
            parameter for parameter in parameters if isinstance(parameter, dict) and parameter.get("in") == "path"
        ]
        for name in expected_names:
            parameter = next((candidate for candidate in path_parameters if candidate.get("name") == name), None)
            if parameter is None:
                missing.append(f"{method.upper()} {path} missing path parameter {name}")
                continue
            if parameter.get("required") is not True:
                malformed.append(f"{method.upper()} {path} path parameter {name} must set required: true")
            schema = parameter.get("schema")
            if not isinstance(schema, dict) or not (schema.get("type") or schema.get("$ref")):
                malformed.append(f"{method.upper()} {path} path parameter {name} must declare a schema type or ref")
        for parameter in path_parameters:
            name = parameter.get("name")
            if name not in expected_names:
                extra.append(f"{method.upper()} {path} declares extra path parameter {name}")
    fail("[openapi-contract] /api path parameter contract failed:", missing + malformed + extra)


def require_api_operation_metadata_contract(spec: dict[str, Any]) -> None:
    missing_operation_id: list[str] = []
    duplicate_operation_id: dict[str, list[str]] = {}
    missing_tags: list[str] = []
    malformed_parameters: list[str] = []
    duplicate_parameters: list[str] = []
    for path, method, op, operations in api_operations(spec):
        operation_id = op.get("operationId")
        if not str(operation_id or "").strip():
            missing_operation_id.append(f"{method.upper()} {path}")
        else:
            duplicate_operation_id.setdefault(str(operation_id), []).append(f"{method.upper()} {path}")
        if not isinstance(op.get("tags"), list) or not op.get("tags"):
            missing_tags.append(f"{method.upper()} {path}")
        seen: set[tuple[str, str]] = set()
        shared_parameters = operations.get("parameters", []) if isinstance(operations, dict) else []
        for parameter in list(shared_parameters) + list(op.get("parameters", [])):
            if isinstance(parameter, dict) and parameter.get("$ref"):
                parameter = resolve_ref(spec, parameter["$ref"])
            if not isinstance(parameter, dict):
                malformed_parameters.append(f"{method.upper()} {path} has a non-object parameter")
                continue
            name = parameter.get("name")
            location = parameter.get("in")
            if not isinstance(name, str) or not name.strip() or location not in {"path", "query", "header", "cookie"}:
                malformed_parameters.append(
                    f"{method.upper()} {path} has malformed parameter name={inspect_value(name)} in={inspect_value(location)}"
                )
                continue
            key = (str(location), name)
            if key in seen:
                duplicate_parameters.append(f"{method.upper()} {path} declares duplicate parameter {location}:{name}")
            seen.add(key)
            schema = parameter.get("schema")
            if not isinstance(schema, dict) or not (schema.get("type") or schema.get("$ref")):
                malformed_parameters.append(
                    f"{method.upper()} {path} parameter {location}:{name} must declare a schema type or ref"
                )
    messages = [f"{entry} must declare operationId" for entry in missing_operation_id]
    for operation_id, entries in duplicate_operation_id.items():
        if len(entries) > 1:
            messages.append(f"operationId {operation_id} is duplicated by {', '.join(entries)}")
    messages += [f"{entry} must declare at least one tag" for entry in missing_tags]
    messages += malformed_parameters + duplicate_parameters
    fail("[openapi-contract] /api operation metadata contract failed:", messages)


def require_route_surface_manifest_contract(
    spec: dict[str, Any],
    manifest: dict[str, Any],
    contract: dict[str, Any],
    schema: dict[str, Any],
    artifact_paths: dict[str, Path],
) -> None:
    messages: list[str] = []
    try:
        validate_manifest(manifest, schema)
        projected = project_openapi(spec, contract)
        for difference in projection_differences(projected, manifest):
            messages.append(
                f"{difference['kind']} operationId={difference['operationId']} field={difference['field']}"
            )
        expected_outputs = generated_outputs(projected, contract)
        for label, path in artifact_paths.items():
            try:
                actual = path.read_bytes()
            except OSError:
                messages.append(f"missing artifact={label}")
                continue
            if actual != expected_outputs[label]:
                messages.append(f"stale artifact={label}")
    except ProjectionError as exc:
        messages.append(
            f"{exc.code} operationId={exc.operation_id} field={exc.field} count={exc.count}"
        )
    fail("[openapi-contract] route-surface fingerprint parity failed:", messages)


def manifest_route_samples(manifest: dict[str, Any]) -> dict[tuple[Any, Any], dict[str, Any]]:
    return {
        (route.get("method"), route.get("normalizedPath")): route
        for route in manifest.get("routeSamples", [])
        if isinstance(route, dict)
    }


RELEASE_EVIDENCE_PATHS = [
    "/api/v1/admin/release-evidence/rag-indexing",
    "/api/v1/admin/release-evidence/relay-realtime",
    "/api/v1/admin/release-evidence/relay-batch",
    "/api/v1/admin/release-evidence/marketplace-payout",
    "/api/v1/admin/release-evidence/marketplace-governance",
    "/api/v1/admin/release-evidence/provider-runtime-config",
    "/api/v1/admin/release-evidence/microservice-database",
]


WINDOWED_ADMIN_PROOF_PATHS = [
    "/api/v1/admin/observability/latency-slo-proof",
]


def require_release_evidence_contract(spec: dict[str, Any], manifest: dict[str, Any]) -> None:
    paths = spec.get("paths", {})
    schemas = dig(spec, "components", "schemas") or {}
    missing: list[str] = []

    scope_schema = schemas.get("ReleaseEvidenceScope", {})
    if not includes_all(required(scope_schema), ["from", "to"]):
        missing.append("ReleaseEvidenceScope must require from and to")
    for field in ["from", "to"]:
        if dig(scope_schema, "properties", field, "type") != "string" or dig(scope_schema, "properties", field, "format") != "date-time":
            missing.append(f"ReleaseEvidenceScope.{field} must be a date-time string")

    proof_schema = schemas.get("ReleaseEvidenceProof", {})
    if dig(proof_schema, "properties", "scope", "$ref") != "#/components/schemas/ReleaseEvidenceScope":
        missing.append("ReleaseEvidenceProof.scope must reference ReleaseEvidenceScope")
    if dig(proof_schema, "properties", "status", "type") != "string" or "not_ready" not in enum_values(proof_schema, "properties", "status"):
        missing.append("ReleaseEvidenceProof.status must document not_ready")
    if dig(proof_schema, "properties", "notReadyReason", "type") != "string":
        missing.append("ReleaseEvidenceProof.notReadyReason must be documented as a string")

    manifest_routes = manifest_route_samples(manifest)
    for path in RELEASE_EVIDENCE_PATHS:
        op = operation(paths, path, "get", missing)
        if not has_tags(op, "Admin", "ReleaseEvidence"):
            missing.append(f"GET {path} must be tagged Admin and ReleaseEvidence")
        if not requires_cookie_without_csrf(op):
            missing.append(f"GET {path} must require cookieAuth without csrfHeader")
        if response_ref(op, "200") != "#/components/schemas/ReleaseEvidenceProofEnvelope":
            missing.append(f"GET {path} 200 must reference ReleaseEvidenceProofEnvelope")
        if dig(op, "responses", "400", "$ref") != "#/components/responses/BadRequest":
            missing.append(f"GET {path} 400 must reference BadRequest")

        parameters = {
            parameter.get("name"): parameter
            for parameter in op.get("parameters", [])
            if isinstance(parameter, dict) and parameter.get("in") == "query"
        }
        for name in ["from", "to"]:
            parameter = parameters.get(name)
            if not isinstance(parameter, dict):
                missing.append(f"GET {path} must document optional {name} query parameter")
                continue
            if parameter.get("required") is True:
                missing.append(f"GET {path} {name} query parameter must be optional")
            if dig(parameter, "schema", "type") != "string" or dig(parameter, "schema", "format") != "date-time":
                missing.append(f"GET {path} {name} query parameter must be a date-time string")

        manifest_route = manifest_routes.get(("GET", path), {})
        sample_query = manifest_route.get("sampleQuery")
        if not isinstance(sample_query, dict) or not all(isinstance(sample_query.get(name), str) and sample_query.get(name) for name in ["from", "to"]):
            missing.append(f"route-surface manifest GET {path} must include sampleQuery.from and sampleQuery.to")

    fail("[openapi-contract] release-evidence contract is incomplete:", missing)


def require_windowed_admin_proof_contract(spec: dict[str, Any], manifest: dict[str, Any]) -> None:
    paths = spec.get("paths", {})
    manifest_routes = manifest_route_samples(manifest)
    missing: list[str] = []

    for path in WINDOWED_ADMIN_PROOF_PATHS:
        op = operation(paths, path, "get", missing)
        parameters = {
            parameter.get("name"): parameter
            for parameter in op.get("parameters", [])
            if isinstance(parameter, dict) and parameter.get("in") == "query"
        }
        for name in ["from", "to"]:
            parameter = parameters.get(name)
            if not isinstance(parameter, dict):
                missing.append(f"GET {path} must document required {name} query parameter")
                continue
            if parameter.get("required") is not True:
                missing.append(f"GET {path} {name} query parameter must be required")
            if dig(parameter, "schema", "type") != "string" or dig(parameter, "schema", "format") != "date-time":
                missing.append(f"GET {path} {name} query parameter must be a date-time string")

        manifest_route = manifest_routes.get(("GET", path), {})
        sample_query = manifest_route.get("sampleQuery")
        if not isinstance(sample_query, dict) or not all(isinstance(sample_query.get(name), str) and sample_query.get(name) for name in ["from", "to"]):
            missing.append(f"route-surface manifest GET {path} must include sampleQuery.from and sampleQuery.to")

    fail("[openapi-contract] windowed admin proof contract is incomplete:", missing)


def require_session_csrf_contract(spec: dict[str, Any]) -> None:
    paths = spec.get("paths", {})
    schemas = dig(spec, "components", "schemas") or {}
    schemes = dig(spec, "components", "securitySchemes") or {}
    missing: list[str] = []
    csrf_header = schemes.get("csrfHeader")
    if not (
        isinstance(csrf_header, dict)
        and csrf_header.get("type") == "apiKey"
        and csrf_header.get("in") == "header"
        and csrf_header.get("name") == "X-CSRF-Token"
    ):
        missing.append("components.securitySchemes.csrfHeader must document the X-CSRF-Token header")
    csrf_token = dig(schemas, "SessionResponse", "properties", "csrfToken")
    if not (isinstance(csrf_token, dict) and csrf_token.get("type") == "string" and "X-CSRF-Token" in csrf_token.get("description", "")):
        missing.append("components.schemas.SessionResponse.csrfToken must document reuse as X-CSRF-Token")
    public_auth_routes = {
        "/api/v1/auth/register": ("#/components/schemas/CredentialsRequest", "#/components/schemas/SessionResponse"),
        "/api/v1/auth/login": ("#/components/schemas/CredentialsRequest", "#/components/schemas/SessionResponse"),
        "/api/v1/auth/password-reset/request": ("#/components/schemas/PasswordResetRequest", "#/components/schemas/PasswordResetRequestResponse"),
        "/api/v1/auth/password-reset/confirm": ("#/components/schemas/PasswordResetConfirmRequest", "#/components/schemas/PasswordResetConfirmResponse"),
    }
    for path, (request_ref, response_ref_value) in public_auth_routes.items():
        op = dig(paths, path, "post") or {}
        if op.get("security") != []:
            missing.append(f"POST {path} must declare security: []")
        if request_body_ref(op) != request_ref:
            missing.append(f"POST {path} request body must reference {request_ref}")
        if response_data_ref(op, "200") != response_ref_value:
            missing.append(f"POST {path} 200 data must reference {response_ref_value}")
    cookie_only_get_routes = {
        "/api/v1/auth/me": ("Auth", "#/components/schemas/SessionResponse"),
        "/api/v1/app/me/preferences": ("Preferences", "#/components/schemas/Preferences"),
        "/api/v1/app/notifications": ("Notification", "#/components/schemas/Notification"),
        "/api/v1/app/notifications/unread-count": ("Notification", "#/components/schemas/NotificationUnreadCount"),
    }
    for path, (tag, response_ref_value) in cookie_only_get_routes.items():
        op = dig(paths, path, "get") or {}
        if not requires_cookie_without_csrf(op):
            missing.append(f"GET {path} must declare cookieAuth without csrfHeader")
        if not has_tags(op, tag):
            missing.append(f"GET {path} must be tagged {tag}")
        actual = response_data_array_ref(op, "200") if path == "/api/v1/app/notifications" else response_data_ref(op, "200")
        if actual != response_ref_value:
            missing.append(f"GET {path} 200 data must reference {response_ref_value}")
    if not ("email" in required(schemas.get("PasswordResetRequest", {})) and dig(schemas, "PasswordResetRequest", "properties", "email", "format") == "email"):
        missing.append("PasswordResetRequest must require email")
    if not ("token" in required(schemas.get("PasswordResetConfirmRequest", {})) and "password" in required(schemas.get("PasswordResetConfirmRequest", {}))):
        missing.append("PasswordResetConfirmRequest must require token and password")
    if not (
        dig(schemas, "PasswordResetRequestResponse", "properties", "requested", "type") == "boolean"
        and dig(schemas, "PasswordResetConfirmResponse", "properties", "reset", "type") == "boolean"
    ):
        missing.append("Password reset responses must document requested/reset booleans")
    if not (
        "loggedOut" in required(schemas.get("AuthLogoutResponse", {}))
        and dig(schemas, "AuthLogoutResponse", "properties", "loggedOut", "type") == "boolean"
    ):
        missing.append("AuthLogoutResponse must require loggedOut boolean")
    logout = dig(paths, "/api/v1/auth/logout", "post") or {}
    if not requires_cookie_and_csrf(logout):
        missing.append("POST /api/v1/auth/logout must require both cookieAuth and csrfHeader")
    if response_data_ref(logout, "200") != "#/components/schemas/AuthLogoutResponse":
        missing.append("POST /api/v1/auth/logout 200 data must reference AuthLogoutResponse")
    fail("[openapi-contract] session CSRF contract is incomplete:", missing)


def require_marketplace_contracts(spec: dict[str, Any]) -> None:
    paths = spec.get("paths", {})
    schemas = dig(spec, "components", "schemas") or {}
    require_marketplace_paid_install_contract(paths, schemas)
    require_marketplace_template_type_contract(paths, schemas)
    require_marketplace_surface_payload_contract(paths, schemas)
    require_marketplace_browse_payload_contract(paths, schemas)
    require_marketplace_private_read_auth_contract(paths)
    require_marketplace_public_read_contract(paths)
    require_marketplace_user_mutation_csrf_contract(paths)
    require_admin_marketplace_governance_csrf_contract(paths)
    require_admin_marketplace_review_csrf_contract(paths, schemas)


def require_marketplace_paid_install_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    if dig(schemas, "MarketplaceAgentDetailResponse", "properties", "paymentProviders", "items", "$ref") != "#/components/schemas/MarketplacePaymentProvider":
        missing.append("MarketplaceAgentDetailResponse.paymentProviders must reference MarketplacePaymentProvider")
    if not includes_all(enum_values(schemas.get("MarketplacePaymentProvider", {}), "properties", "name"), ["stripe", "alipay", "wechatpay"]):
        missing.append("MarketplacePaymentProvider.name must enumerate stripe, alipay, and wechatpay")
    install_request_refs = [entry.get("$ref") for entry in schemas.get("MarketplaceInstallRequest", {}).get("anyOf", []) if isinstance(entry, dict)]
    if not includes_all(install_request_refs, ["#/components/schemas/MarketplaceFreeInstallRequest", "#/components/schemas/MarketplacePaidInstallRequest"]):
        missing.append("MarketplaceInstallRequest must document free and paid Marketplace install request bodies")
    if dig(schemas, "MarketplaceFreeInstallRequest", "properties", "versionID", "type") != "string":
        missing.append("MarketplaceFreeInstallRequest.versionID must be documented")
    paid_request_provider_enum = enum_values(schemas.get("MarketplacePaidInstallRequest", {}), "properties", "provider")
    if "provider" not in required(schemas.get("MarketplacePaidInstallRequest", {})):
        missing.append("MarketplacePaidInstallRequest must require provider")
    if not includes_all(paid_request_provider_enum, ["stripe", "alipay", "wechatpay"]):
        missing.append("MarketplacePaidInstallRequest.provider must enumerate paid-install providers")
    install_refs = [entry.get("$ref") for entry in schemas.get("MarketplaceInstallResponse", {}).get("oneOf", []) if isinstance(entry, dict)]
    if not includes_all(install_refs, ["#/components/schemas/MarketplaceAgentInstall", "#/components/schemas/BillingCheckoutSession"]):
        missing.append("MarketplaceInstallResponse must cover free install records and paid checkout sessions")
    if response_data_ref(dig(paths, "/api/v1/marketplace/agents/{agentId}", "get") or {}, "200") != "#/components/schemas/MarketplaceAgentDetailResponse":
        missing.append("GET /api/v1/marketplace/agents/{agentId} must return MarketplaceAgentDetailResponse data")
    install = dig(paths, "/api/v1/marketplace/agents/{agentId}/install", "post") or {}
    if request_body_ref(install) != "#/components/schemas/MarketplaceInstallRequest":
        missing.append("POST /api/v1/marketplace/agents/{agentId}/install must document MarketplaceInstallRequest body")
    if response_data_ref(install, "201") != "#/components/schemas/MarketplaceInstallResponse":
        missing.append("POST /api/v1/marketplace/agents/{agentId}/install 201 must return MarketplaceInstallResponse data")
    for status in ["501", "502"]:
        if dig(install, "responses", status, "content", "application/json", "schema", "$ref") != "#/components/schemas/Envelope":
            missing.append(f"POST /api/v1/marketplace/agents/{{agentId}}/install {status} response must reference Envelope")
    fail("[openapi-contract] Marketplace paid-install contract is incomplete:", missing)


def require_marketplace_template_type_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    parameters = dig(paths, "/api/v1/marketplace/templates", "get", "parameters") or []
    list_enum = []
    if isinstance(parameters, list):
        list_enum = next((dig(param, "schema", "enum") for param in parameters if isinstance(param, dict) and param.get("name") == "type"), []) or []
    create_schema = dig(paths, "/api/v1/marketplace/templates", "post", "requestBody", "content", "application/json", "schema") or {}
    if isinstance(create_schema, dict) and create_schema.get("$ref"):
        create_schema = schemas.get(ref_name(create_schema.get("$ref")), {})
    create_enum = dig(create_schema, "properties", "type", "enum") or []
    for label, values in [
        ("GET /api/v1/marketplace/templates type query", list_enum),
        ("POST /api/v1/marketplace/templates type body", create_enum),
    ]:
        if not includes_all(values, ["agent", "workflow", "plugin"]):
            missing.append(f"{label} must enumerate agent, workflow, and plugin")
        if "bot" in values:
            missing.append(f"{label} must not expose legacy bot template type")
    fail("[openapi-contract] Marketplace template type contract is incomplete:", missing)


def require_marketplace_surface_payload_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/marketplace/agents", "post", "201"): "#/components/schemas/MarketplacePublishedAgent",
        ("/api/v1/marketplace/agents/{agentId}", "put", "200"): "#/components/schemas/MarketplacePublishedAgent",
        ("/api/v1/marketplace/agents/{agentId}", "delete", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/marketplace/agents/{agentId}/install", "delete", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/marketplace/agents/{agentId}/appeal", "post", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/marketplace/agents/{agentId}/abuse-reports", "post", "201"): "#/components/schemas/MarketplaceAbuseReport",
        ("/api/v1/marketplace/agents/{agentId}/reviews", "post", "201"): "#/components/schemas/MarketplaceAgentReview",
        ("/api/v1/marketplace/installs/{agentId}", "delete", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/marketplace/publisher/settlement-preferences", "get", "200"): "#/components/schemas/MarketplaceSettlementPreferences",
        ("/api/v1/marketplace/publisher/settlement-preferences", "put", "200"): "#/components/schemas/MarketplaceSettlementPreferences",
        ("/api/v1/marketplace/templates", "get", "200"): "#/components/schemas/MarketplaceTemplatesResponse",
        ("/api/v1/marketplace/templates", "post", "201"): "#/components/schemas/MarketplaceTemplate",
        ("/api/v1/marketplace/templates/{templateId}", "get", "200"): "#/components/schemas/MarketplaceTemplateDetailResponse",
        ("/api/v1/marketplace/templates/{templateId}/install", "post", "201"): "#/components/schemas/MarketplaceTemplateInstall",
        ("/api/v1/marketplace/agents/{agentId}/stats", "get", "200"): "#/components/schemas/MarketplaceAgentStats",
        ("/api/v1/marketplace/publisher/stats", "get", "200"): "#/components/schemas/MarketplacePublisherStats",
        ("/api/v1/admin/marketplace/agents/{agentId}/takedown", "post", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/admin/marketplace/agents/{agentId}/reinstate", "post", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/admin/marketplace/agents/{agentId}/reject-appeal", "post", "200"): "#/components/schemas/MarketplaceActionStatusResponse",
        ("/api/v1/admin/marketplace/abuse-reports", "get", "200"): "#/components/schemas/MarketplaceAbuseReportsResponse",
        ("/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve", "post", "200"): "#/components/schemas/MarketplaceAbuseReportStatusResponse",
        ("/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss", "post", "200"): "#/components/schemas/MarketplaceAbuseReportStatusResponse",
    }
    for (path, method, status), expected in expected_data_refs.items():
        if response_data_ref(dig(paths, path, method) or {}, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    expected_body_refs = {
        ("/api/v1/marketplace/agents", "post"): "#/components/schemas/MarketplaceAgentPublishRequest",
        ("/api/v1/marketplace/agents/{agentId}", "put"): "#/components/schemas/MarketplaceAgentPublishRequest",
        ("/api/v1/marketplace/agents/{agentId}/appeal", "post"): "#/components/schemas/MarketplaceGovernanceActionRequest",
        ("/api/v1/marketplace/agents/{agentId}/abuse-reports", "post"): "#/components/schemas/MarketplaceAbuseReportRequest",
        ("/api/v1/marketplace/agents/{agentId}/reviews", "post"): "#/components/schemas/MarketplaceReviewSubmitRequest",
        ("/api/v1/marketplace/publisher/settlement-preferences", "put"): "#/components/schemas/MarketplaceSettlementPreferencesRequest",
        ("/api/v1/marketplace/templates", "post"): "#/components/schemas/MarketplaceTemplateCreateRequest",
        ("/api/v1/admin/marketplace/agents/{agentId}/takedown", "post"): "#/components/schemas/MarketplaceGovernanceActionRequest",
        ("/api/v1/admin/marketplace/agents/{agentId}/reinstate", "post"): "#/components/schemas/MarketplaceGovernanceActionRequest",
        ("/api/v1/admin/marketplace/agents/{agentId}/reject-appeal", "post"): "#/components/schemas/MarketplaceGovernanceActionRequest",
        ("/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve", "post"): "#/components/schemas/MarketplaceAbuseReportResolutionRequest",
        ("/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss", "post"): "#/components/schemas/MarketplaceAbuseReportResolutionRequest",
    }
    for (path, method), expected in expected_body_refs.items():
        op = dig(paths, path, method) or {}
        if request_body_ref(op) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
        if dig(op, "requestBody", "required") is not True:
            missing.append(f"{method.upper()} {path} request body must be required")
    publish_request = schemas.get("MarketplaceAgentPublishRequest", {})
    check_required_fields(schemas, "MarketplaceAgentPublishRequest", ["name", "description", "categoryID", "tools", "pricingType", "version"], missing)
    check_property_type(schemas, "MarketplaceAgentPublishRequest", ["name", "description", "iconURL", "categoryID", "tools", "exampleConversations", "systemPrompt", "pricingType", "version", "changelog"], "string", missing)
    if dig(publish_request, "properties", "tags", "type") != "array" or dig(publish_request, "properties", "tags", "items", "type") != "string":
        missing.append("MarketplaceAgentPublishRequest.tags must be documented as string[]")
    if dig(publish_request, "properties", "pricingAmount", "type") != "number" or dig(publish_request, "properties", "pricingAmount", "format") != "double":
        missing.append("MarketplaceAgentPublishRequest.pricingAmount must be documented as double")
    if not includes_all(enum_values(publish_request, "properties", "visibility"), ["public", "private", "unlisted"]):
        missing.append("MarketplaceAgentPublishRequest.visibility must enumerate public, private, and unlisted")
    if not includes_all(enum_values(publish_request, "properties", "pricingType"), ["free", "one_time", "subscription"]):
        missing.append("MarketplaceAgentPublishRequest.pricingType must enumerate free, one_time, and subscription")
    if not includes_all(enum_values(schemas.get("MarketplaceActionStatusResponse", {}), "properties", "status"), ["deleted", "uninstalled", "appeal_pending", "takedown", "approved"]):
        missing.append("MarketplaceActionStatusResponse.status must enumerate Marketplace action statuses")
    if "reason" not in required(schemas.get("MarketplaceGovernanceActionRequest", {})):
        missing.append("MarketplaceGovernanceActionRequest must require reason")
    if dig(schemas, "MarketplaceGovernanceActionRequest", "properties", "reason", "type") != "string":
        missing.append("MarketplaceGovernanceActionRequest.reason must be documented as string")
    review_submit = schemas.get("MarketplaceReviewSubmitRequest", {})
    if "rating" not in required(review_submit):
        missing.append("MarketplaceReviewSubmitRequest must require rating")
    if not (
        dig(review_submit, "properties", "rating", "type") == "integer"
        and dig(review_submit, "properties", "rating", "minimum") == 1
        and dig(review_submit, "properties", "rating", "maximum") == 5
    ):
        missing.append("MarketplaceReviewSubmitRequest.rating must be documented as integer 1..5")
    if dig(review_submit, "properties", "body", "type") != "string":
        missing.append("MarketplaceReviewSubmitRequest.body must be documented as string")
    if dig(schemas, "MarketplaceTemplatesResponse", "properties", "templates", "items", "$ref") != "#/components/schemas/MarketplaceTemplate" or dig(schemas, "MarketplaceTemplatesResponse", "properties", "total", "type") != "integer":
        missing.append("MarketplaceTemplatesResponse must expose templates[] and total")
    if dig(schemas, "MarketplaceAbuseReportsResponse", "properties", "reports", "items", "$ref") != "#/components/schemas/MarketplaceAbuseReport" or dig(schemas, "MarketplaceAbuseReportsResponse", "properties", "total", "type") != "integer":
        missing.append("MarketplaceAbuseReportsResponse must expose reports[] and total")
    if not includes_all(enum_values(schemas.get("MarketplaceSettlementPreferences", {}), "properties", "cycle"), ["weekly", "monthly", "quarterly"]):
        missing.append("MarketplaceSettlementPreferences.cycle must enumerate weekly, monthly, and quarterly")
    check_property_type(schemas, "MarketplaceAgentStats", ["agentID", "agentName"], "string", missing)
    check_property_type(schemas, "MarketplaceAgentStats", ["installCount", "activeUsers", "apiCallCount"], "integer", missing)
    check_property_type(schemas, "MarketplacePublisherStats", ["totalAgents", "totalInstalls", "activeUsers", "totalAPICalls"], "integer", missing)
    check_property_type(schemas, "MarketplacePublisherStats", ["grossRevenue", "platformFees", "netRevenue", "refundedAmount", "pendingSettlementAmount", "availableAmount", "payoutPendingAmount", "paidOutAmount"], "number", missing)
    if dig(schemas, "MarketplacePublisherStats", "properties", "revenueTier", "$ref") != "#/components/schemas/MarketplaceRevenueTierDisclosure":
        missing.append("MarketplacePublisherStats.revenueTier must reference MarketplaceRevenueTierDisclosure")
    if dig(schemas, "MarketplacePublisherStats", "properties", "perAgentStats", "items", "$ref") != "#/components/schemas/MarketplaceAgentStats":
        missing.append("MarketplacePublisherStats.perAgentStats must expose MarketplaceAgentStats[]")
    check_property_type(schemas, "MarketplaceRevenueTierDisclosure", ["currentTier", "label"], "string", missing)
    check_property_type(schemas, "MarketplaceRevenueTierDisclosure", ["monthlySalesAmount", "platformFeeAmount", "publisherNetAmount", "platformFeePercent", "publisherSharePercent", "effectivePlatformFeePercent", "nextTierAt", "salesToNextTier", "estimatedPublisherNetAtNextTier", "estimatedPublisherNetIncreaseAtNextTier"], "number", missing)
    fail("[openapi-contract] Marketplace surface payload contract is incomplete:", missing)


def require_marketplace_browse_payload_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/marketplace/featured", "get", "200"): "#/components/schemas/MarketplaceAgentListResponse",
        ("/api/v1/marketplace/search", "get", "200"): "#/components/schemas/MarketplaceAgentListResponse",
        ("/api/v1/marketplace/agents", "get", "200"): "#/components/schemas/MarketplaceAgentListResponse",
        ("/api/v1/marketplace/my-agents", "get", "200"): "#/components/schemas/MarketplaceAgentListResponse",
        ("/api/v1/marketplace/curated", "get", "200"): "#/components/schemas/MarketplaceCuratedSectionsResponse",
        ("/api/v1/marketplace/categories", "get", "200"): "#/components/schemas/MarketplaceCategoriesResponse",
        ("/api/v1/marketplace/installs", "get", "200"): "#/components/schemas/MarketplaceInstallsResponse",
        ("/api/v1/marketplace/agents/{agentId}/reviews", "get", "200"): "#/components/schemas/MarketplaceReviewsResponse",
        ("/api/v1/marketplace/agents/{agentId}/versions", "get", "200"): "#/components/schemas/MarketplaceVersionsResponse",
    }
    for (path, method, status), expected in expected_data_refs.items():
        if response_data_ref(dig(paths, path, method) or {}, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    response_collections = {
        "MarketplaceAgentListResponse": ("agents", "#/components/schemas/MarketplacePublishedAgent"),
        "MarketplaceCategoriesResponse": ("categories", "#/components/schemas/MarketplaceCategory"),
        "MarketplaceInstallsResponse": ("installs", "#/components/schemas/MarketplaceAgentInstall"),
        "MarketplaceReviewsResponse": ("reviews", "#/components/schemas/MarketplaceAgentReview"),
        "MarketplaceVersionsResponse": ("versions", "#/components/schemas/MarketplaceAgentVersion"),
    }
    for schema_name, (collection, item_ref) in response_collections.items():
        if not (
            dig(schemas, schema_name, "properties", collection, "type") == "array"
            and dig(schemas, schema_name, "properties", collection, "items", "$ref") == item_ref
            and dig(schemas, schema_name, "properties", "total", "type") == "integer"
        ):
            missing.append(f"{schema_name} must expose {collection}[] as {item_ref} plus integer total")
    if dig(schemas, "MarketplacePublishedAgent", "properties", "recommendation", "$ref") != "#/components/schemas/MarketplaceRecommendationMetadata":
        missing.append("MarketplacePublishedAgent.recommendation must reference MarketplaceRecommendationMetadata")
    if not (
        dig(schemas, "MarketplaceRecommendationMetadata", "properties", "score", "type") == "number"
        and dig(schemas, "MarketplaceRecommendationMetadata", "properties", "score", "format") == "double"
        and dig(schemas, "MarketplaceRecommendationMetadata", "properties", "reason", "type") == "string"
    ):
        missing.append("MarketplaceRecommendationMetadata must document score double and reason string")
    for field in ["popular", "topRated", "recent"]:
        if not (
            dig(schemas, "MarketplaceCuratedSectionsResponse", "properties", field, "type") == "array"
            and dig(schemas, "MarketplaceCuratedSectionsResponse", "properties", field, "items", "$ref") == "#/components/schemas/MarketplacePublishedAgent"
        ):
            missing.append(f"MarketplaceCuratedSectionsResponse.{field} must expose MarketplacePublishedAgent[]")
    check_property_type(schemas, "MarketplaceCategory", ["id", "name", "slug"], "string", missing)
    if dig(schemas, "MarketplaceCategory", "properties", "displayOrder", "type") != "integer" or dig(schemas, "MarketplaceCategory", "properties", "agentCount", "type") != "integer":
        missing.append("MarketplaceCategory must document displayOrder and agentCount integers")
    if dig(schemas, "MarketplaceAgentReview", "properties", "rating", "type") != "integer" or dig(schemas, "MarketplaceAgentReview", "properties", "body", "type") != "string":
        missing.append("MarketplaceAgentReview must document rating integer and body string")
    fail("[openapi-contract] Marketplace browse payload contract is incomplete:", missing)


def require_marketplace_private_read_auth_contract(paths: dict[str, Any]) -> None:
    missing: list[str] = []
    for path in [
        "/api/v1/marketplace/agents/{agentId}/stats",
        "/api/v1/marketplace/my-agents",
        "/api/v1/marketplace/installs",
        "/api/v1/marketplace/publisher/stats",
        "/api/v1/marketplace/publisher/settlement-preferences",
    ]:
        op = operation(paths, path, "get", missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"GET {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Marketplace"):
            missing.append(f"GET {path} must be tagged Marketplace")
    fail("[openapi-contract] Marketplace private read auth contract is incomplete:", missing)


def require_marketplace_public_read_contract(paths: dict[str, Any]) -> None:
    missing: list[str] = []
    for path in [
        "/api/v1/marketplace/featured",
        "/api/v1/marketplace/curated",
        "/api/v1/marketplace/categories",
        "/api/v1/marketplace/search",
        "/api/v1/marketplace/agents",
        "/api/v1/marketplace/agents/{agentId}",
        "/api/v1/marketplace/agents/{agentId}/reviews",
        "/api/v1/marketplace/agents/{agentId}/versions",
        "/api/v1/marketplace/templates",
        "/api/v1/marketplace/templates/{templateId}",
    ]:
        op = operation(paths, path, "get", missing)
        if op.get("security") != []:
            missing.append(f"GET {path} must declare security: []")
        if not has_tags(op, "Marketplace"):
            missing.append(f"GET {path} must be tagged Marketplace")
    fail("[openapi-contract] Marketplace public read contract is incomplete:", missing)


def require_marketplace_user_mutation_csrf_contract(paths: dict[str, Any]) -> None:
    missing: list[str] = []
    check_mutation_security(paths, [
        ("/api/v1/marketplace/agents", "post"),
        ("/api/v1/marketplace/agents/{agentId}", "put"),
        ("/api/v1/marketplace/agents/{agentId}", "delete"),
        ("/api/v1/marketplace/agents/{agentId}/install", "post"),
        ("/api/v1/marketplace/agents/{agentId}/install", "delete"),
        ("/api/v1/marketplace/installs/{agentId}", "delete"),
        ("/api/v1/marketplace/agents/{agentId}/reviews", "post"),
        ("/api/v1/marketplace/agents/{agentId}/appeal", "post"),
        ("/api/v1/marketplace/agents/{agentId}/abuse-reports", "post"),
        ("/api/v1/marketplace/publisher/settlement-preferences", "put"),
        ("/api/v1/marketplace/templates", "post"),
        ("/api/v1/marketplace/templates/{templateId}/install", "post"),
    ], missing, "Marketplace")
    fail("[openapi-contract] Marketplace user mutation CSRF contract is incomplete:", missing)


def require_admin_marketplace_governance_csrf_contract(paths: dict[str, Any]) -> None:
    missing: list[str] = []
    for path, method in [
        ("/api/v1/admin/marketplace/agents/{agentId}/takedown", "post"),
        ("/api/v1/admin/marketplace/agents/{agentId}/reinstate", "post"),
        ("/api/v1/admin/marketplace/agents/{agentId}/reject-appeal", "post"),
        ("/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve", "post"),
        ("/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss", "post"),
    ]:
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Admin", "Marketplace"):
            missing.append(f"{method.upper()} {path} must be tagged Admin and Marketplace")
    abuse = operation(paths, "/api/v1/admin/marketplace/abuse-reports", "get", missing)
    if not requires_cookie_without_csrf(abuse):
        missing.append("GET /api/v1/admin/marketplace/abuse-reports must require cookieAuth without csrfHeader")
    if not has_tags(abuse, "Admin", "Marketplace"):
        missing.append("GET /api/v1/admin/marketplace/abuse-reports must be tagged Admin and Marketplace")
    fail("[openapi-contract] Admin Marketplace governance CSRF contract is incomplete:", missing)


def require_admin_marketplace_review_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    for path, method in [
        ("/api/v1/admin/reviews/sla/enforce", "post"),
        ("/api/v1/admin/reviews/{agentId}/approve", "post"),
        ("/api/v1/admin/reviews/{agentId}/claim", "post"),
        ("/api/v1/admin/reviews/{agentId}/reject", "post"),
        ("/api/v1/admin/reviews/{agentId}/needs-changes", "post"),
    ]:
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Admin", "Marketplace"):
            missing.append(f"{method.upper()} {path} must be tagged Admin and Marketplace")
    expected = {
        ("/api/v1/admin/reviews", "get"): "#/components/schemas/AdminReviewListResponse",
        ("/api/v1/admin/reviews/sla/enforce", "post"): "#/components/schemas/MarketplaceReviewSLAEnforcementResult",
        ("/api/v1/admin/reviews/{agentId}/approve", "post"): "#/components/schemas/MarketplaceReviewStatusResponse",
        ("/api/v1/admin/reviews/{agentId}/claim", "post"): "#/components/schemas/MarketplaceReviewStatusResponse",
        ("/api/v1/admin/reviews/{agentId}/reject", "post"): "#/components/schemas/MarketplaceReviewStatusResponse",
        ("/api/v1/admin/reviews/{agentId}/needs-changes", "post"): "#/components/schemas/MarketplaceReviewStatusResponse",
    }
    for (path, method), ref in expected.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, "200") != ref:
            missing.append(f"{method.upper()} {path} 200 data must reference {ref}")
        if method == "get" and not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
    for path, method in [("/api/v1/admin/reviews/{agentId}/reject", "post"), ("/api/v1/admin/reviews/{agentId}/needs-changes", "post")]:
        op = operation(paths, path, method, missing)
        expected_ref = "#/components/schemas/MarketplaceReviewDecisionRequest"
        if dig(op, "requestBody", "required") is not True or request_body_ref(op) != expected_ref:
            missing.append(f"{method.upper()} {path} request body must require {expected_ref}")
    review_props = props(schemas.get("AdminReviewListResponse", {}))
    if dig(review_props, "reviews", "type") != "array" or dig(review_props, "reviews", "items", "$ref") != "#/components/schemas/MarketplacePublishedAgent" or dig(review_props, "total", "type") != "integer":
        missing.append("AdminReviewListResponse must expose MarketplacePublishedAgent reviews[] plus integer total")
    if dig(props(schemas.get("MarketplacePublishedAgent", {})), "reviewerUserId", "type") != "string":
        missing.append("MarketplacePublishedAgent.reviewerUserId must be documented as string")
    if dig(props(schemas.get("MarketplaceReviewDecisionRequest", {})), "reason", "type") != "string":
        missing.append("MarketplaceReviewDecisionRequest.reason must be documented as string")
    status = props(schemas.get("MarketplaceReviewStatusResponse", {}))
    if dig(status, "status", "type") != "string" or sorted(dig(status, "status", "enum") or []) != ["approved", "claimed", "needs_changes", "rejected"]:
        missing.append("MarketplaceReviewStatusResponse.status must enumerate approved, claimed, rejected, and needs_changes")
    sla = props(schemas.get("MarketplaceReviewSLAEnforcementResult", {}))
    if dig(sla, "scanned", "type") != "integer" or dig(sla, "alerted", "type") != "integer":
        missing.append("MarketplaceReviewSLAEnforcementResult must expose integer scanned and alerted counts")
    fail("[openapi-contract] Admin Marketplace review CSRF/schema contract is incomplete:", missing)


def require_admin_channel_secret_response_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/admin/channel-providers", "get", "200"): "#/components/schemas/AdminChannelProviderListResponse",
        ("/api/v1/admin/models", "get", "200"): "#/components/schemas/AdminModelInventoryListResponse",
        ("/api/v1/admin/channels", "get", "200"): "#/components/schemas/AdminChannelListResponse",
        ("/api/v1/admin/channels", "post", "201"): "#/components/schemas/AdminChannel",
        ("/api/v1/admin/channels/{channelId}", "get", "200"): "#/components/schemas/AdminChannel",
        ("/api/v1/admin/channels/{channelId}", "put", "200"): "#/components/schemas/AdminChannel",
        ("/api/v1/admin/channels/{channelId}/test", "post", "200"): "#/components/schemas/AdminChannelTestResult",
        ("/api/v1/admin/channels/{channelId}/health", "get", "200"): "#/components/schemas/AdminChannelHealth",
        ("/api/v1/admin/channels/stats", "get", "200"): "#/components/schemas/AdminChannelRuntimeStatsResponse",
        ("/api/v1/admin/channels/{channelId}/sync-models", "post", "200"): "#/components/schemas/AdminChannelModelSyncResponse",
        ("/api/v1/admin/channels/{channelId}/model-updates/detect", "post", "200"): "#/components/schemas/AdminChannelModelUpdatePreview",
        ("/api/v1/admin/channels/{channelId}/model-updates/apply", "post", "200"): "#/components/schemas/AdminChannelModelUpdateApplyResponse",
        ("/api/v1/admin/channels/{channelId}/refresh-balance", "post", "200"): "#/components/schemas/AdminChannelBalanceRefreshResponse",
    }
    for (path, method, status), expected in expected_data_refs.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
        if not has_tags(op, "Admin", "Relay"):
            missing.append(f"{method.upper()} {path} must be tagged Admin and Relay")
    for path in ["/api/v1/admin/channel-providers", "/api/v1/admin/models", "/api/v1/admin/channels", "/api/v1/admin/channels/stats", "/api/v1/admin/channels/{channelId}", "/api/v1/admin/channels/{channelId}/health"]:
        if not requires_cookie_without_csrf(operation(paths, path, "get", missing)):
            missing.append(f"GET {path} must require cookieAuth without csrfHeader")
    for path in ["/api/v1/admin/channels", "/api/v1/admin/channels/{channelId}", "/api/v1/admin/channels/batch", "/api/v1/admin/channels/{channelId}/test", "/api/v1/admin/channels/{channelId}/sync-models", "/api/v1/admin/channels/{channelId}/model-updates/detect", "/api/v1/admin/channels/{channelId}/model-updates/apply", "/api/v1/admin/channels/{channelId}/refresh-balance"]:
        for method in [key for key in (paths.get(path, {}) if isinstance(paths.get(path), dict) else {}).keys() if key in {"post", "put", "delete"}]:
            if not requires_cookie_and_csrf(operation(paths, path, method, missing)):
                missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
    if request_body_ref(operation(paths, "/api/v1/admin/channels", "post", missing)) != "#/components/schemas/AdminChannelCreateRequest":
        missing.append("POST /api/v1/admin/channels request body must reference AdminChannelCreateRequest")
    if request_body_ref(operation(paths, "/api/v1/admin/channels/{channelId}", "put", missing)) != "#/components/schemas/AdminChannelUpdateRequest":
        missing.append("PUT /api/v1/admin/channels/{channelId} request body must reference AdminChannelUpdateRequest")
    channel_props = props(schemas.get("AdminChannel", {}))
    for field in ["apiKey", "api_key", "apiKeyEncrypted", "api_key_encrypted"]:
        if field in channel_props:
            missing.append("AdminChannel response schema must not expose API key fields")
            break
    for field in ["id", "name", "provider", "baseURL", "status"]:
        if dig(channel_props, field, "type") != "string":
            missing.append(f"AdminChannel.{field} must be documented as string")
    for field in ["models", "groups"]:
        if dig(channel_props, field, "type") != "array" or dig(channel_props, field, "items", "type") != "string":
            missing.append(f"AdminChannel.{field} must be documented as string[]")
    if dig(schemas, "AdminChannelListResponse", "properties", "channels", "items", "$ref") != "#/components/schemas/AdminChannel" or dig(schemas, "AdminChannelListResponse", "properties", "total", "type") != "integer":
        missing.append("AdminChannelListResponse must expose channels[] as AdminChannel plus integer total")
    for field in ["id", "displayName", "kind", "status", "defaultBaseURL"]:
        if dig(schemas, "AdminChannelProvider", "properties", field, "type") != "string":
            missing.append(f"AdminChannelProvider.{field} must be documented as string")
    if dig(schemas, "AdminChannelProvider", "properties", "configurable", "type") != "boolean":
        missing.append("AdminChannelProvider.configurable must be documented as boolean")
    if dig(schemas, "AdminChannelProviderListResponse", "properties", "providers", "items", "$ref") != "#/components/schemas/AdminChannelProvider":
        missing.append("AdminChannelProviderListResponse must expose providers[] as AdminChannelProvider")
    if dig(schemas, "AdminChannelCreateRequest", "properties", "apiKey", "type") != "string" or dig(schemas, "AdminChannelUpdateRequest", "properties", "apiKey", "type") != "string":
        missing.append("Admin channel create/update request schemas must document write-only apiKey input")
    if dig(schemas, "AdminChannelCreateRequest", "properties", "apiKey", "writeOnly") is not True or dig(schemas, "AdminChannelUpdateRequest", "properties", "apiKey", "writeOnly") is not True:
        missing.append("Admin channel apiKey request fields must be writeOnly")
    if dig(schemas, "AdminChannelModelSyncResponse", "properties", "channel", "$ref") != "#/components/schemas/AdminChannel" or dig(schemas, "AdminChannelModelSyncResponse", "properties", "testResult", "$ref") != "#/components/schemas/AdminChannelTestResult":
        missing.append("AdminChannelModelSyncResponse must expose channel and testResult")
    for field in ["currentModels", "upstreamModels", "added", "removed", "unchanged"]:
        if dig(schemas, "AdminChannelModelUpdatePreview", "properties", field, "type") != "array" or dig(schemas, "AdminChannelModelUpdatePreview", "properties", field, "items", "type") != "string":
            missing.append(f"AdminChannelModelUpdatePreview.{field} must be documented as string[]")
    if dig(schemas, "AdminChannelModelUpdatePreview", "properties", "testResult", "$ref") != "#/components/schemas/AdminChannelTestResult":
        missing.append("AdminChannelModelUpdatePreview.testResult must reference AdminChannelTestResult")
    if not includes_all(enum_values(schemas.get("AdminChannelModelUpdateApplyRequest", {}), "properties", "mode"), ["merge", "replace"]):
        missing.append("AdminChannelModelUpdateApplyRequest.mode must enumerate merge and replace")
    if not (
        dig(schemas, "AdminChannelModelUpdateApplyResponse", "properties", "channel", "$ref") == "#/components/schemas/AdminChannel"
        and dig(schemas, "AdminChannelModelUpdateApplyResponse", "properties", "preview", "$ref") == "#/components/schemas/AdminChannelModelUpdatePreview"
        and dig(schemas, "AdminChannelModelUpdateApplyResponse", "properties", "appliedModels", "items", "type") == "string"
    ):
        missing.append("AdminChannelModelUpdateApplyResponse must expose channel, preview, and appliedModels[]")
    if request_body_ref(operation(paths, "/api/v1/admin/channels/{channelId}/model-updates/apply", "post", missing)) != "#/components/schemas/AdminChannelModelUpdateApplyRequest":
        missing.append("POST /api/v1/admin/channels/{channelId}/model-updates/apply request body must reference AdminChannelModelUpdateApplyRequest")
    if not (
        dig(schemas, "AdminChannelBalanceRefreshResponse", "properties", "balance", "$ref") == "#/components/schemas/AdminChannelBalance"
        and dig(schemas, "AdminChannelBalanceRefreshResponse", "properties", "channelHealth", "$ref") == "#/components/schemas/AdminChannelHealthDetail"
        and dig(schemas, "AdminChannelBalanceRefreshResponse", "properties", "testResult", "$ref") == "#/components/schemas/AdminChannelTestResult"
    ):
        missing.append("AdminChannelBalanceRefreshResponse must expose balance, channelHealth, and testResult")
    if dig(schemas, "AdminChannelRuntimeStatsResponse", "properties", "stats", "items", "$ref") != "#/components/schemas/AdminChannelRuntimeStats":
        missing.append("AdminChannelRuntimeStatsResponse must expose stats[] as AdminChannelRuntimeStats")
    for field in ["channelID", "rpmCurrent", "tpmCurrent", "totalRequests", "successCount", "failureCount", "avgLatencyMs", "affinityConversationCount"]:
        if dig(schemas, "AdminChannelRuntimeStats", "properties", field) is None:
            missing.append(f"AdminChannelRuntimeStats.{field} must be documented")
    if dig(schemas, "AdminModelInventoryListResponse", "properties", "models", "items", "$ref") != "#/components/schemas/AdminModelInventoryEntry":
        missing.append("AdminModelInventoryListResponse must expose models[] as AdminModelInventoryEntry")
    if dig(schemas, "AdminModelInventoryListResponse", "properties", "total") is None:
        missing.append("AdminModelInventoryListResponse.total must be documented")
    for field in ["model", "providers", "groups", "channelCount", "enabledChannelCount", "disabledChannelCount", "requestCount", "totalCost", "totalChannelCost", "channels"]:
        if dig(schemas, "AdminModelInventoryEntry", "properties", field) is None:
            missing.append(f"AdminModelInventoryEntry.{field} must be documented")
    if dig(schemas, "AdminModelInventoryEntry", "properties", "channels", "items", "$ref") != "#/components/schemas/AdminModelInventoryChannel":
        missing.append("AdminModelInventoryEntry.channels must reference AdminModelInventoryChannel")
    for field in ["id", "name", "provider", "groups", "enabled", "priority", "estimatedCostPer1K", "costMultiplier"]:
        if dig(schemas, "AdminModelInventoryChannel", "properties", field) is None:
            missing.append(f"AdminModelInventoryChannel.{field} must be documented")
    fail("[openapi-contract] Admin channel secret response contract is incomplete:", missing)


def require_publishing_channel_secret_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/channels/{channelId}", "get", "200"): "#/components/schemas/ChannelConfig",
        ("/api/v1/channels", "post", "201"): "#/components/schemas/ChannelConfig",
        ("/api/v1/channels/{channelId}", "put", "200"): "#/components/schemas/ChannelConfig",
        ("/api/v1/channels/{channelId}", "delete", "200"): "#/components/schemas/ChannelConfig",
        ("/api/v1/channels/{channelId}/status", "patch", "200"): "#/components/schemas/ChannelConfig",
        ("/api/v1/channels/{channelId}/test", "post", "200"): "#/components/schemas/ChannelTestResult",
        ("/api/v1/channels/{channelId}/send", "post", "200"): "#/components/schemas/ChannelMessageLog",
        ("/api/v1/channels/{channelId}/retry-failed-messages", "post", "200"): "#/components/schemas/ChannelRetryProcessResult",
        ("/api/v1/channels/webhook/{channelId}", "post", "200"): "#/components/schemas/ChannelMessageLog",
    }
    for (path, method, status), expected in expected_data_refs.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
        if not has_tags(op, "Publishing"):
            missing.append(f"{method.upper()} {path} must be tagged Publishing")
    for path in ["/api/v1/channels", "/api/v1/channels/{channelId}/messages", "/api/v1/channels/{channelId}/failed-messages"]:
        op = operation(paths, path, "get", missing)
        expected = "#/components/schemas/ChannelConfig" if path == "/api/v1/channels" else "#/components/schemas/ChannelMessageLog"
        if response_data_array_ref(op, "200") != expected:
            missing.append(f"GET {path} 200 data must return the documented Publishing collection item schema")
        if not has_tags(op, "Publishing"):
            missing.append(f"GET {path} must be tagged Publishing")
    check_read_security(paths, [
        ("/api/v1/channels", "get"),
        ("/api/v1/channels/{channelId}", "get"),
        ("/api/v1/channels/{channelId}/messages", "get"),
        ("/api/v1/channels/{channelId}/failed-messages", "get"),
    ], missing)
    check_mutation_security(paths, [
        ("/api/v1/channels", "post"),
        ("/api/v1/channels/{channelId}", "put"),
        ("/api/v1/channels/{channelId}", "delete"),
        ("/api/v1/channels/{channelId}/status", "patch"),
        ("/api/v1/channels/{channelId}/test", "post"),
        ("/api/v1/channels/{channelId}/send", "post"),
        ("/api/v1/channels/{channelId}/retry-failed-messages", "post"),
    ], missing)
    expected_body_refs = {
        ("/api/v1/channels", "post"): "#/components/schemas/ChannelConfigRequest",
        ("/api/v1/channels/{channelId}", "put"): "#/components/schemas/ChannelConfigRequest",
        ("/api/v1/channels/{channelId}/status", "patch"): "#/components/schemas/ChannelStatusRequest",
        ("/api/v1/channels/{channelId}/send", "post"): "#/components/schemas/SendChannelMessageRequest",
        ("/api/v1/channels/{channelId}/retry-failed-messages", "post"): "#/components/schemas/RetryFailedChannelMessagesRequest",
    }
    for (path, method), expected in expected_body_refs.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    webhook = operation(paths, "/api/v1/channels/webhook/{channelId}", "post", missing)
    if webhook.get("security") != []:
        missing.append("POST /api/v1/channels/webhook/{channelId} must remain public with security: []")
    webhook_headers = [
        param.get("name") for param in webhook.get("parameters", []) if isinstance(param, dict) and param.get("in") == "header"
    ]
    for header in ["X-Oblivious-Timestamp", "X-Oblivious-Signature"]:
        if header not in webhook_headers:
            missing.append(f"POST /api/v1/channels/webhook/{{channelId}} must document {header}")
    config = dig(schemas, "ChannelConfig", "properties", "config") or {}
    if not (config.get("type") == "object" and config.get("additionalProperties") is True and "redacted" in config.get("description", "")):
        missing.append("ChannelConfig.config must document redacted response secrets")
    for field in props(schemas.get("ChannelConfig", {})):
        normalized = field.lower().replace("_", "").replace("-", "")
        if any(needle in normalized for needle in ["secret", "token", "apikey", "password"]):
            missing.append(f"ChannelConfig response schema must not expose {field} as a top-level credential field")
    request_config = dig(schemas, "ChannelConfigRequest", "properties", "config") or {}
    if not (
        request_config.get("type") == "object"
        and request_config.get("additionalProperties") is True
        and all(word in request_config.get("description", "") for word in ["secret", "token", "apiKey", "password"])
    ):
        missing.append("ChannelConfigRequest.config must document credential input and redacted-marker preservation")
    check_property_type(schemas, "ChannelTestResult", ["channel_id", "type", "message"], "string", missing)
    if not includes_all(enum_values(schemas.get("ChannelTestResult", {}), "properties", "status"), ["success", "failed"]):
        missing.append("ChannelTestResult.status must enumerate success and failed")
    fail("[openapi-contract] Publishing channel secret/CSRF contract is incomplete:", missing)


def require_admin_observability_provider_secret_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/admin/observability/alert-routing", "get", "200"): "#/components/schemas/AdminObservabilityAlertRoutingRules",
        ("/api/v1/admin/observability/alert-routing", "put", "200"): "#/components/schemas/AdminObservabilityAlertRoutingRules",
        ("/api/v1/admin/observability/alert-providers", "post", "201"): "#/components/schemas/AdminObservabilityAlertProvider",
        ("/api/v1/admin/observability/alert-providers/{providerId}", "put", "200"): "#/components/schemas/AdminObservabilityAlertProvider",
        ("/api/v1/admin/observability/alert-providers/{providerId}/test", "post", "200"): "#/components/schemas/AdminObservabilityAlertProviderTestResult",
    }
    for (path, method, status), expected in expected_data_refs.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
        if not has_tags(op, "Admin", "Observability"):
            missing.append(f"{method.upper()} {path} must be tagged Admin and Observability")
    list_op = operation(paths, "/api/v1/admin/observability/alert-providers", "get", missing)
    if response_data_array_ref(list_op, "200") != "#/components/schemas/AdminObservabilityAlertProvider":
        missing.append("GET /api/v1/admin/observability/alert-providers 200 data must return AdminObservabilityAlertProvider[]")
    if not has_tags(list_op, "Admin", "Observability"):
        missing.append("GET /api/v1/admin/observability/alert-providers must be tagged Admin and Observability")
    for path, method in [
        ("/api/v1/admin/observability/alert-routing", "get"),
        ("/api/v1/admin/observability/alert-providers", "get"),
        ("/api/v1/admin/observability/alerts", "get"),
        ("/api/v1/admin/observability/alerts/{alertKey}", "get"),
        ("/api/v1/admin/observability/alerts/{alertKey}/deliveries", "get"),
        ("/api/v1/admin/observability/recovery-actions", "get"),
    ]:
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Admin", "Observability"):
            missing.append(f"{method.upper()} {path} must be tagged Admin and Observability")
    for (path, method), expected in {
        ("/api/v1/admin/observability/alert-routing", "put"): "#/components/schemas/UpdateAdminObservabilityAlertRoutingRulesRequest",
        ("/api/v1/admin/observability/alert-providers", "post"): "#/components/schemas/AdminObservabilityAlertProviderRequest",
        ("/api/v1/admin/observability/alert-providers/{providerId}", "put"): "#/components/schemas/AdminObservabilityAlertProviderRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    check_mutation_security(paths, [
        ("/api/v1/admin/observability/alert-routing", "put"),
        ("/api/v1/admin/observability/alert-providers", "post"),
        ("/api/v1/admin/observability/alert-providers/{providerId}", "put"),
        ("/api/v1/admin/observability/alert-providers/{providerId}/test", "post"),
        ("/api/v1/admin/observability/alerts/{alertKey}/acknowledge", "post"),
        ("/api/v1/admin/observability/alerts/{alertKey}/resolve", "post"),
    ], missing)
    config = schemas.get("AdminObservabilityAlertProviderConfig", {})
    description = config.get("description", "") if isinstance(config, dict) else ""
    if not (
        config.get("type") == "object"
        and dig(config, "additionalProperties", "type") == "string"
        and all(word in description for word in ["password", "secret", "token", "webhook_url", "routing_key", "api_key", "private_key"])
        and "********" in description
        and "preserved" in description
    ):
        missing.append("AdminObservabilityAlertProviderConfig must document credential input, redaction, and redacted-marker preservation")
    provider_props = props(schemas.get("AdminObservabilityAlertProvider", {}))
    for field in ["id", "name", "createdAt", "updatedAt"]:
        if field not in provider_props:
            missing.append(f"AdminObservabilityAlertProvider.{field} must be documented")
    if not (
        dig(provider_props, "kind", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderKind"
        and dig(provider_props, "channel", "$ref") == "#/components/schemas/AdminObservabilityAlertDeliveryChannel"
        and dig(provider_props, "status", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderStatus"
        and dig(provider_props, "config", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderConfig"
    ):
        missing.append("AdminObservabilityAlertProvider must document kind/channel/status/config refs")
    if dig(schemas, "AdminObservabilityAlertProviderRequest", "properties", "config", "$ref") != "#/components/schemas/AdminObservabilityAlertProviderConfig":
        missing.append("AdminObservabilityAlertProviderRequest.config must reference AdminObservabilityAlertProviderConfig")
    fail("[openapi-contract] Admin Observability provider secret/CSRF contract is incomplete:", missing)


def require_mcp_auth_token_response_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    if response_data_array_ref(operation(paths, "/api/v1/app/mcp-servers", "get", missing), "200") != "#/components/schemas/McpServer":
        missing.append("GET /api/v1/app/mcp-servers 200 data must return McpServer[]")
    expected_data_refs = {
        ("/api/v1/app/mcp-servers", "post", "201"): "#/components/schemas/McpServer",
        ("/api/v1/app/mcp-servers/{serverId}", "get", "200"): "#/components/schemas/McpServer",
        ("/api/v1/app/mcp-servers/{serverId}", "delete", "200"): "#/components/schemas/McpActionStatus",
        ("/api/v1/app/mcp-servers/{serverId}/connect", "post", "200"): "#/components/schemas/McpServer",
        ("/api/v1/app/mcp-servers/{serverId}/disconnect", "post", "200"): "#/components/schemas/McpActionStatus",
        ("/api/v1/app/mcp-servers/{serverId}/status", "get", "200"): "#/components/schemas/McpActionStatus",
        ("/api/v1/app/mcp-servers/{serverId}/execute", "post", "200"): "#/components/schemas/McpToolResult",
    }
    for (path, method, status), expected in expected_data_refs.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
        if not has_tags(op, "MCP"):
            missing.append(f"{method.upper()} {path} must be tagged MCP")
    if response_data_array_ref(operation(paths, "/api/v1/app/mcp-local-servers", "get", missing), "200") != "#/components/schemas/McpLocalServer":
        missing.append("GET /api/v1/app/mcp-local-servers 200 data must return McpLocalServer[]")
    if response_data_array_ref(operation(paths, "/api/v1/app/mcp-servers/{serverId}/tools", "get", missing), "200") != "#/components/schemas/McpToolDefinition":
        missing.append("GET /api/v1/app/mcp-servers/{serverId}/tools 200 data must return McpToolDefinition[]")
    check_read_security(paths, [
        ("/api/v1/app/mcp-local-servers", "get"),
        ("/api/v1/app/mcp-servers", "get"),
        ("/api/v1/app/mcp-servers/{serverId}", "get"),
        ("/api/v1/app/mcp-servers/{serverId}/tools", "get"),
        ("/api/v1/app/mcp-servers/{serverId}/status", "get"),
    ], missing, "MCP")
    for (path, method), expected in {
        ("/api/v1/app/mcp-servers", "post"): "#/components/schemas/AddMcpServerRequest",
        ("/api/v1/app/mcp-servers/{serverId}/execute", "post"): "#/components/schemas/ExecuteMcpToolRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    check_mutation_security(paths, [
        ("/api/v1/app/mcp-servers", "post"),
        ("/api/v1/app/mcp-servers/{serverId}", "delete"),
        ("/api/v1/app/mcp-servers/{serverId}/connect", "post"),
        ("/api/v1/app/mcp-servers/{serverId}/disconnect", "post"),
        ("/api/v1/app/mcp-servers/{serverId}/execute", "post"),
    ], missing)
    server_props = props(schemas.get("McpServer", {}))
    if "authToken" in server_props or "auth_token" in server_props:
        missing.append("McpServer response schema must not expose authToken")
    if dig(server_props, "hasAuthToken", "type") != "boolean" or "raw token is not returned" not in str(dig(server_props, "hasAuthToken", "description") or ""):
        missing.append("McpServer.hasAuthToken must document raw-token redaction")
    for field in ["id", "organizationId", "userId", "name", "url", "status", "createdAt", "updatedAt"]:
        if field not in server_props:
            missing.append(f"McpServer.{field} must be documented")
    auth_token = dig(schemas, "AddMcpServerRequest", "properties", "authToken") or {}
    if not (
        auth_token.get("type") == "string"
        and auth_token.get("format") == "password"
        and auth_token.get("writeOnly") is True
        and "hasAuthToken" in auth_token.get("description", "")
    ):
        missing.append("AddMcpServerRequest.authToken must be password writeOnly input and point responses to hasAuthToken")
    fail("[openapi-contract] MCP auth-token response contract is incomplete:", missing)


def require_agent_run_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    mutation_paths = [
        ("/api/v1/agent/runs", "post"),
        ("/api/v1/agent/runs/{runId}/approve-tool", "post"),
        ("/api/v1/agent/runs/{runId}/reject-tool", "post"),
        ("/api/v1/agent/runs/{runId}/retry-tool", "post"),
        ("/api/v1/agent/runs/{runId}/continue-budget", "post"),
        ("/api/v1/agent/runs/{runId}/adjust-plan", "post"),
        ("/api/v1/agent/runs/{runId}/continue-plan", "post"),
        ("/api/v1/agent/runs/{runId}/approve-plan-step", "post"),
        ("/api/v1/agent/runs/{runId}/execute-plan-step", "post"),
        ("/api/v1/agent/runs/{runId}/skip-plan-step", "post"),
        ("/api/v1/agent/runs/{runId}/retry-plan-step", "post"),
        ("/api/v1/agent/runs/{runId}/update-plan-step", "patch"),
        ("/api/v1/agent/runs/{runId}/create-plan-step", "post"),
        ("/api/v1/agent/runs/{runId}/move-plan-step", "post"),
        ("/api/v1/agent/runs/{runId}/delete-plan-step", "post"),
        ("/api/v1/app/agents/tool-runs/{toolRunId}/approve", "post"),
        ("/api/v1/app/agents/tool-runs/{toolRunId}/reject", "post"),
        ("/api/v1/app/agents/tool-runs/{toolRunId}/retry", "post"),
    ]
    check_mutation_security(paths, mutation_paths, missing, "Agent")
    check_read_security(paths, [("/api/v1/agent/tools", "get"), ("/api/v1/agent/runs/{runId}", "get")], missing, "Agent")
    if response_data_array_ref(operation(paths, "/api/v1/agent/tools", "get", missing), "200") != "#/components/schemas/AgentToolDefinition":
        missing.append("GET /api/v1/agent/tools 200 data items must reference AgentToolDefinition")
    for (path, method), expected in {
        ("/api/v1/agent/runs", "post"): "#/components/schemas/AgentRunCreateRequest",
        ("/api/v1/agent/runs/{runId}/approve-tool", "post"): "#/components/schemas/AgentToolDecisionRequest",
        ("/api/v1/agent/runs/{runId}/reject-tool", "post"): "#/components/schemas/AgentToolDecisionRequest",
        ("/api/v1/agent/runs/{runId}/retry-tool", "post"): "#/components/schemas/AgentToolDecisionRequest",
        ("/api/v1/agent/runs/{runId}/continue-budget", "post"): "#/components/schemas/AgentRunContinueBudgetRequest",
        ("/api/v1/agent/runs/{runId}/adjust-plan", "post"): "#/components/schemas/AgentRunAdjustPlanRequest",
        ("/api/v1/agent/runs/{runId}/approve-plan-step", "post"): "#/components/schemas/AgentPlanStepDecisionRequest",
        ("/api/v1/agent/runs/{runId}/execute-plan-step", "post"): "#/components/schemas/AgentPlanStepDecisionRequest",
        ("/api/v1/agent/runs/{runId}/skip-plan-step", "post"): "#/components/schemas/AgentPlanStepDecisionRequest",
        ("/api/v1/agent/runs/{runId}/retry-plan-step", "post"): "#/components/schemas/AgentPlanStepDecisionRequest",
        ("/api/v1/agent/runs/{runId}/update-plan-step", "patch"): "#/components/schemas/AgentPlanStepUpdateRequest",
        ("/api/v1/agent/runs/{runId}/create-plan-step", "post"): "#/components/schemas/AgentPlanStepCreateRequest",
        ("/api/v1/agent/runs/{runId}/move-plan-step", "post"): "#/components/schemas/AgentPlanStepMoveRequest",
        ("/api/v1/agent/runs/{runId}/delete-plan-step", "post"): "#/components/schemas/AgentPlanStepDecisionRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    for path, method, status in [
        ("/api/v1/agent/runs", "post", "201"),
        ("/api/v1/agent/runs/{runId}", "get", "200"),
        ("/api/v1/agent/runs/{runId}/approve-tool", "post", "200"),
        ("/api/v1/agent/runs/{runId}/reject-tool", "post", "200"),
        ("/api/v1/agent/runs/{runId}/retry-tool", "post", "200"),
        ("/api/v1/agent/runs/{runId}/continue-budget", "post", "200"),
        ("/api/v1/agent/runs/{runId}/adjust-plan", "post", "200"),
        ("/api/v1/agent/runs/{runId}/continue-plan", "post", "200"),
        ("/api/v1/agent/runs/{runId}/approve-plan-step", "post", "200"),
        ("/api/v1/agent/runs/{runId}/execute-plan-step", "post", "200"),
        ("/api/v1/agent/runs/{runId}/skip-plan-step", "post", "200"),
        ("/api/v1/agent/runs/{runId}/retry-plan-step", "post", "200"),
        ("/api/v1/agent/runs/{runId}/update-plan-step", "patch", "200"),
        ("/api/v1/agent/runs/{runId}/create-plan-step", "post", "201"),
        ("/api/v1/agent/runs/{runId}/move-plan-step", "post", "200"),
        ("/api/v1/agent/runs/{runId}/delete-plan-step", "post", "200"),
    ]:
        if response_data_ref(operation(paths, path, method, missing), status) != "#/components/schemas/AgentRunResponse":
            missing.append(f"{method.upper()} {path} {status} data must reference #/components/schemas/AgentRunResponse")
    create = schemas.get("AgentRunCreateRequest", {})
    if not (
        all_of_any_requires(create, "agent_id")
        and all_of_any_requires(create, "agentId")
        and all_of_any_requires(create, "conversation_id")
        and all_of_any_requires(create, "conversationId")
        and all_of_any_requires(create, "input")
        and all_of_any_requires(create, "message")
        and "planning" in enum_values(create, "properties", "mode")
        and dig(create, "properties", "max_iterations", "maximum") == 100
        and dig(create, "properties", "maxIterations", "maximum") == 100
        and dig(create, "properties", "token_budget", "maximum") == 1000000
        and dig(create, "properties", "tokenBudget", "maximum") == 1000000
    ):
        missing.append("AgentRunCreateRequest must document snake/camel agent, conversation, input/message, mode, and execution-limit controls")
    continue_budget = schemas.get("AgentRunContinueBudgetRequest", {})
    if not (
        one_of_requires(continue_budget, "token_budget")
        and one_of_requires(continue_budget, "tokenBudget")
        and dig(continue_budget, "properties", "token_budget", "minimum") == 1000
        and dig(continue_budget, "properties", "token_budget", "maximum") == 1000000
        and dig(continue_budget, "properties", "tokenBudget", "minimum") == 1000
        and dig(continue_budget, "properties", "tokenBudget", "maximum") == 1000000
    ):
        missing.append("AgentRunContinueBudgetRequest must require snake/camel token budget aliases with bounded values")
    if not (
        "planning" in enum_values(schemas.get("AgentRun", {}), "properties", "mode")
        and "token_budget_exceeded" in enum_values(schemas.get("AgentRun", {}), "properties", "status")
        and dig(schemas, "AgentRunResponse", "properties", "run", "$ref") == "#/components/schemas/AgentRun"
        and dig(schemas, "AgentRunResponse", "properties", "toolRuns", "items", "$ref") == "#/components/schemas/AgentToolRun"
        and dig(schemas, "AgentRunResponse", "properties", "planSteps", "items", "$ref") == "#/components/schemas/AgentPlanStep"
        and dig(schemas, "AgentRunResponse", "properties", "messages", "items", "$ref") == "#/components/schemas/Message"
    ):
        missing.append("AgentRunResponse must expose run, toolRuns, planSteps, messages, and planning/token-budget status fields")
    if not (
        dig(schemas, "AgentToolDecisionRequest", "properties", "toolRunId", "type") == "string"
        and dig(schemas, "AgentToolDecisionRequest", "properties", "tool_run_id", "type") == "string"
        and dig(schemas, "AgentPlanStepDecisionRequest", "properties", "planStepId", "type") == "string"
        and dig(schemas, "AgentPlanStepDecisionRequest", "properties", "plan_step_id", "type") == "string"
        and sorted(enum_values(schemas.get("AgentPlanStepMoveRequest", {}), "properties", "direction")) == ["down", "up"]
    ):
        missing.append("Agent decision request schemas must document snake/camel identifiers and move direction enum")
    if not (
        dig(schemas, "AgentToolRun", "properties", "toolType", "type") == "string"
        and dig(schemas, "AgentToolRun", "properties", "serverId", "type") == "string"
        and dig(schemas, "AgentToolRun", "properties", "riskLevel", "type") == "string"
    ):
        missing.append("AgentToolRun must document toolType, serverId, and riskLevel metadata for custom/MCP tool evidence")
    if not (
        dig(schemas, "AgentPlanStep", "properties", "description", "type") == "string"
        and dig(schemas, "AgentPlanStep", "properties", "dependsOn", "items", "type") == "integer"
        and dig(schemas, "AgentPlanStepUpdateRequest", "properties", "description", "type") == "string"
        and dig(schemas, "AgentPlanStepUpdateRequest", "properties", "dependsOn", "items", "minimum") == 1
        and dig(schemas, "AgentPlanStepUpdateRequest", "properties", "depends_on", "items", "minimum") == 1
        and dig(schemas, "AgentPlanStepCreateRequest", "properties", "description", "type") == "string"
        and dig(schemas, "AgentPlanStepCreateRequest", "properties", "dependsOn", "items", "minimum") == 1
        and dig(schemas, "AgentPlanStepCreateRequest", "properties", "depends_on", "items", "minimum") == 1
    ):
        missing.append("Agent plan-step schemas must document structured description and dependsOn fields for response, update, and create requests")
    fail("[openapi-contract] Agent run mutation CSRF/schema contract is incomplete:", missing)


def require_workspace_agent_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    check_mutation_security(paths, [
        ("/api/v1/app/agents", "post"),
        ("/api/v1/app/agents/{agentId}", "put"),
        ("/api/v1/app/agents/{agentId}", "delete"),
        ("/api/v1/app/agents/{agentId}/conversations", "post"),
        ("/api/v1/app/agents/conversations/{conversationId}", "delete"),
        ("/api/v1/app/agents/conversations/{conversationId}/messages", "post"),
    ], missing, "Agent")
    read_responses = {
        ("/api/v1/app/agents", "get", "200"): ("#/components/schemas/AgentWorkspaceAgent", "array"),
        ("/api/v1/app/agents/{agentId}", "get", "200"): ("#/components/schemas/AgentWorkspaceAgent", "ref"),
        ("/api/v1/app/agents/{agentId}/tools", "get", "200"): ("#/components/schemas/AgentToolDefinition", "array"),
        ("/api/v1/app/agents/{agentId}/conversations", "get", "200"): ("#/components/schemas/AgentConversation", "array"),
        ("/api/v1/app/agents/conversations/{conversationId}", "get", "200"): ("#/components/schemas/AgentConversation", "ref"),
        ("/api/v1/app/agents/conversations/{conversationId}/messages", "get", "200"): ("#/components/schemas/AgentMessage", "array"),
        ("/api/v1/app/agents/conversations/{conversationId}/runs", "get", "200"): ("#/components/schemas/AgentRun", "array"),
        ("/api/v1/app/agents/runs/{runId}", "get", "200"): ("#/components/schemas/AgentRunDetail", "ref"),
    }
    for (path, method, status), (expected, shape) in read_responses.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Agent"):
            missing.append(f"{method.upper()} {path} must be tagged Agent")
        actual = response_data_array_ref(op, status) if shape == "array" else response_data_ref(op, status)
        if actual != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    for (path, method), expected in {
        ("/api/v1/app/agents", "post"): "#/components/schemas/CreateAgentRequest",
        ("/api/v1/app/agents/{agentId}", "put"): "#/components/schemas/UpdateAgentRequest",
        ("/api/v1/app/agents/conversations/{conversationId}/messages", "post"): "#/components/schemas/AgentSendMessageRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    for (path, method, status), expected in {
        ("/api/v1/app/agents", "post", "201"): "#/components/schemas/AgentWorkspaceAgent",
        ("/api/v1/app/agents/{agentId}", "put", "200"): "#/components/schemas/AgentWorkspaceAgent",
        ("/api/v1/app/agents/{agentId}", "delete", "200"): "#/components/schemas/AgentDeleteStatusResponse",
        ("/api/v1/app/agents/{agentId}/conversations", "post", "201"): "#/components/schemas/AgentConversation",
        ("/api/v1/app/agents/conversations/{conversationId}", "delete", "200"): "#/components/schemas/AgentDeleteStatusResponse",
        ("/api/v1/app/agents/conversations/{conversationId}/messages", "post", "200"): "#/components/schemas/AgentMessage",
    }.items():
        if response_data_ref(operation(paths, path, method, missing), status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    if dig(schemas, "AgentDeleteStatusResponse", "properties", "status", "type") != "string":
        missing.append("AgentDeleteStatusResponse must expose status string")
    if not (
        dig(schemas, "AgentSendMessageRequest", "properties", "content", "type") == "string"
        and "planning" in enum_values(schemas.get("AgentSendMessageRequest", {}), "properties", "mode")
        and dig(schemas, "AgentSendMessageRequest", "properties", "max_iterations", "type") == "integer"
        and dig(schemas, "AgentSendMessageRequest", "properties", "maxIterations", "type") == "integer"
        and dig(schemas, "AgentSendMessageRequest", "properties", "token_budget", "type") == "integer"
        and dig(schemas, "AgentSendMessageRequest", "properties", "tokenBudget", "type") == "integer"
    ):
        missing.append("AgentSendMessageRequest must document content, mode, and snake/camel budget controls")
    if not (
        dig(schemas, "CreateAgentRequest", "properties", "config", "$ref") == "#/components/schemas/AgentConfig"
        and dig(schemas, "UpdateAgentRequest", "properties", "config", "$ref") == "#/components/schemas/AgentConfig"
        and "planning" in enum_values(schemas.get("AgentConfig", {}), "properties", "defaultExecutionMode")
        and "llm_assisted" in enum_values(schemas.get("AgentConfig", {}), "properties", "longTermMemoryExtractionPolicy")
        and "memory_key_consolidate" in enum_values(schemas.get("AgentConfig", {}), "properties", "longTermMemoryUpdatePolicy")
        and "manual_only" in enum_values(schemas.get("AgentConfig", {}), "properties", "longTermMemoryWritePolicy")
        and dig(schemas, "AgentConfig", "properties", "modelRoutingRules", "items", "$ref") == "#/components/schemas/AgentModelRoutingRule"
        and dig(schemas, "AgentConfig", "properties", "skills", "items", "$ref") == "#/components/schemas/AgentSkill"
        and dig(schemas, "AgentConfig", "properties", "maxSkills", "type") == "integer"
    ):
        missing.append("Agent create/update request schemas must reference AgentConfig with execution, memory, model routing, and skill policies")
    if not (
        "targetModel" in required(schemas.get("AgentModelRoutingRule", {}))
        and dig(schemas, "AgentModelRoutingRule", "properties", "targetModel", "type") == "string"
        and dig(schemas, "AgentModelRoutingRule", "properties", "keywords", "items", "type") == "string"
        and "name" in required(schemas.get("AgentSkill", {}))
        and dig(schemas, "AgentSkill", "properties", "toolNames", "items", "type") == "string"
    ):
        missing.append("Agent runtime config schemas must document model routing rules and skill bundles")
    fail("[openapi-contract] Workspace Agent mutation CSRF/schema contract is incomplete:", missing)


def require_memory_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    for (path, method), tag in {
        ("/api/v1/app/memory/documents", "get"): "Memory",
        ("/api/v1/app/memory/documents/{documentId}", "get"): "Memory",
        ("/api/v1/app/memory/documents/{documentId}/chunks", "get"): "Memory",
        ("/api/v1/agent/memories", "get"): "Agent",
        ("/api/v1/agent/memories/export", "get"): "Agent",
    }.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, tag):
            missing.append(f"{method.upper()} {path} must be tagged {tag}")
    for (path, method), tag in {
        ("/api/v1/app/memory/documents", "post"): "Memory",
        ("/api/v1/app/memory/documents/{documentId}", "put"): "Memory",
        ("/api/v1/app/memory/documents/{documentId}", "delete"): "Memory",
        ("/api/v1/app/memory/search", "post"): "Memory",
        ("/api/v1/agent/memories", "post"): "Agent",
        ("/api/v1/agent/memories/import", "post"): "Agent",
        ("/api/v1/agent/memories/{memoryId}", "patch"): "Agent",
        ("/api/v1/agent/memories/{memoryId}", "delete"): "Agent",
    }.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, tag):
            missing.append(f"{method.upper()} {path} must be tagged {tag}")
    for (path, method), expected in {
        ("/api/v1/app/memory/documents", "post"): "#/components/schemas/MemoryDocumentRequest",
        ("/api/v1/app/memory/documents/{documentId}", "put"): "#/components/schemas/UpdateMemoryDocumentRequest",
        ("/api/v1/app/memory/search", "post"): "#/components/schemas/MemorySearchRequest",
        ("/api/v1/agent/memories", "post"): "#/components/schemas/AgentMemoryRequest",
        ("/api/v1/agent/memories/import", "post"): "#/components/schemas/AgentMemoryImportRequest",
        ("/api/v1/agent/memories/{memoryId}", "patch"): "#/components/schemas/AgentMemoryUpdateRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    for (path, method, status), expected in {
        ("/api/v1/app/memory/documents", "post", "201"): "#/components/schemas/MemoryDocument",
        ("/api/v1/app/memory/documents/{documentId}", "put", "200"): "#/components/schemas/MemoryDocument",
        ("/api/v1/app/memory/documents/{documentId}", "delete", "200"): "#/components/schemas/MemoryDeleteStatusResponse",
        ("/api/v1/agent/memories", "post", "201"): "#/components/schemas/AgentMemory",
        ("/api/v1/agent/memories/{memoryId}", "patch", "200"): "#/components/schemas/AgentMemory",
    }.items():
        if response_data_ref(operation(paths, path, method, missing), status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    checks = [
        ("/api/v1/app/memory/search", "post", "200", "#/components/schemas/MemorySearchResult", "array", "POST /api/v1/app/memory/search 200 data items must reference MemorySearchResult"),
        ("/api/v1/app/memory/documents", "get", "200", "#/components/schemas/MemoryDocument", "array", "GET /api/v1/app/memory/documents 200 data items must reference MemoryDocument"),
        ("/api/v1/app/memory/documents/{documentId}", "get", "200", "#/components/schemas/MemoryDocument", "ref", "GET /api/v1/app/memory/documents/{documentId} 200 data must reference MemoryDocument"),
        ("/api/v1/app/memory/documents/{documentId}/chunks", "get", "200", "#/components/schemas/MemoryChunk", "array", "GET /api/v1/app/memory/documents/{documentId}/chunks 200 data items must reference MemoryChunk"),
        ("/api/v1/agent/memories", "get", "200", "#/components/schemas/AgentMemory", "array", "GET /api/v1/agent/memories 200 data items must reference AgentMemory"),
        ("/api/v1/agent/memories/export", "get", "200", "#/components/schemas/AgentMemoryListResponse", "ref", "GET /api/v1/agent/memories/export 200 data must reference AgentMemoryListResponse"),
        ("/api/v1/agent/memories/import", "post", "201", "#/components/schemas/AgentMemory", "array", "POST /api/v1/agent/memories/import 201 data items must reference AgentMemory"),
    ]
    for path, method, status, expected, shape, message in checks:
        op = operation(paths, path, method, missing)
        actual = response_data_array_ref(op, status) if shape == "array" else response_data_ref(op, status)
        if actual != expected:
            missing.append(message)
    delete_op = operation(paths, "/api/v1/agent/memories/{memoryId}", "delete", missing)
    if dig(delete_op, "responses", "204", "description") is None:
        missing.append("DELETE /api/v1/agent/memories/{memoryId} must document 204 delete response")
    if dig(schemas, "MemoryDeleteStatusResponse", "properties", "status", "type") != "string":
        missing.append("MemoryDeleteStatusResponse must expose status string")
    if not (
        dig(schemas, "AgentMemoryUpdateRequest", "properties", "content", "type") == "string"
        and dig(schemas, "AgentMemoryUpdateRequest", "properties", "importance", "type") == "integer"
        and dig(schemas, "AgentMemoryUpdateRequest", "properties", "importance", "minimum") == 1
        and dig(schemas, "AgentMemoryUpdateRequest", "properties", "importance", "maximum") == 5
    ):
        missing.append("AgentMemoryUpdateRequest must document content and bounded importance")
    if not (
        dig(schemas, "MemorySearchRequest", "properties", "query", "type") == "string"
        and dig(schemas, "AgentMemoryRequest", "properties", "content", "type") == "string"
        and dig(schemas, "AgentMemoryRequest", "properties", "agentId", "type") == "string"
        and dig(schemas, "AgentMemoryRequest", "properties", "agent_id", "type") == "string"
        and dig(schemas, "AgentMemoryImportRequest", "properties", "memories", "items", "$ref") == "#/components/schemas/AgentMemoryRequest"
    ):
        missing.append("Memory and Agent memory request schemas must expose query/content and import item refs")
    fail("[openapi-contract] Memory mutation CSRF/schema contract is incomplete:", missing)


def require_billing_checkout_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    checkout = operation(paths, "/api/v1/billing/checkout", "post", missing)
    if not has_tags(checkout, "Billing"):
        missing.append("POST /api/v1/billing/checkout must be tagged Billing")
    if not requires_cookie_and_csrf(checkout):
        missing.append("POST /api/v1/billing/checkout must require cookieAuth and csrfHeader")
    if dig(checkout, "requestBody", "required") is not True or request_body_ref(checkout) != "#/components/schemas/BillingCheckoutRequest":
        missing.append("POST /api/v1/billing/checkout request body must require BillingCheckoutRequest")
    if response_data_ref(checkout, "201") != "#/components/schemas/BillingCheckoutSession":
        missing.append("POST /api/v1/billing/checkout 201 data must reference BillingCheckoutSession")
    for status in ["501", "502"]:
        if dig(checkout, "responses", status, "content", "application/json", "schema", "$ref") != "#/components/schemas/Envelope":
            missing.append(f"POST /api/v1/billing/checkout {status} response must reference Envelope")
    read_routes = {
        "/api/v1/console/billing": ("#/components/schemas/BillingSummary", "ref"),
        "/api/v1/console/usage": ("#/components/schemas/UsageSummary", "ref"),
        "/api/v1/console/access": ("#/components/schemas/AccessSummary", "ref"),
        "/api/v1/console/models": ("#/components/schemas/ModelSummary", "array"),
        "/api/v1/console/invoices": ("#/components/schemas/BillingInvoiceSummary", "array"),
        "/api/v1/console/api-tokens": ("#/components/schemas/RelayAPIToken", "array"),
        "/api/v1/console/api-tokens/{tokenId}/usage": ("#/components/schemas/ConsoleAPITokenUsageItem", "array"),
    }
    for path, (expected, shape) in read_routes.items():
        op = operation(paths, path, "get", missing)
        actual = response_data_array_ref(op, "200") if shape == "array" else response_data_ref(op, "200")
        if actual != expected:
            noun = "data items" if shape == "array" else "data"
            missing.append(f"GET {path} 200 {noun} must reference {expected.split('/')[-1] if path == '/api/v1/console/models' else expected}")
        if not requires_cookie_without_csrf(op):
            missing.append(f"GET {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Billing"):
            missing.append(f"GET {path} must be tagged Billing")
    model_summary = schemas.get("ModelSummary", {})
    if not (
        includes_all(required(model_summary), ["id", "label", "requests"])
        and dig(model_summary, "properties", "id", "type") == "string"
        and dig(model_summary, "properties", "label", "type") == "string"
        and dig(model_summary, "properties", "requests", "type") == "integer"
        and dig(model_summary, "properties", "requests", "minimum") == 0
    ):
        missing.append("ModelSummary must require id, label, and non-negative integer requests")
    if dig(schemas, "BillingSummary", "properties", "paymentProviders", "items", "$ref") != "#/components/schemas/BillingPaymentProvider":
        missing.append("BillingSummary.paymentProviders must reference BillingPaymentProvider")
    if not ("name" in required(schemas.get("BillingPaymentProvider", {})) and enum_values(schemas.get("BillingPaymentProvider", {}), "properties", "name") == ["stripe", "alipay", "wechatpay"]):
        missing.append("BillingPaymentProvider.name must require and enumerate stripe, alipay, and wechatpay")
    if dig(schemas, "BillingInvoiceSummary", "properties", "hostedInvoiceUrl", "format") != "uri" or dig(schemas, "BillingInvoiceSummary", "properties", "invoicePdf", "format") != "uri":
        missing.append("BillingInvoiceSummary must document hostedInvoiceUrl and invoicePdf URI fields")
    usage_props = props(schemas.get("UsageSummary", {}))
    if not (
        includes_all(required(schemas.get("UsageSummary", {})), ["period", "requests"])
        and dig(usage_props, "period", "type") == "string"
        and dig(usage_props, "requests", "type") == "integer"
        and dig(usage_props, "byModel", "items", "$ref") == "#/components/schemas/UsageDimensionSummary"
        and dig(usage_props, "byFeature", "items", "$ref") == "#/components/schemas/UsageDimensionSummary"
        and dig(usage_props, "byUser", "items", "$ref") == "#/components/schemas/UsageDimensionSummary"
        and dig(usage_props, "timeSeries", "items", "$ref") == "#/components/schemas/UsageTimeSeriesSummary"
        and dig(usage_props, "recent", "items", "$ref") == "#/components/schemas/ConsoleAPITokenUsageItem"
    ):
        missing.append("UsageSummary must document period, requests, usage dimensions, time series, and recent token usage")
    for schema_name, fields in {
        "UsageDimensionSummary": [("key", "string"), ("requestCount", "integer"), ("totalTokens", "integer"), ("totalCost", "number")],
        "UsageTimeSeriesSummary": [("bucket", "string"), ("requestCount", "integer"), ("totalTokens", "integer"), ("totalCost", "number")],
    }.items():
        schema_props = props(schemas.get(schema_name, {}))
        for field, expected_type in fields:
            if dig(schema_props, field, "type") != expected_type:
                missing.append(f"{schema_name} must document {', '.join(name for name, _type in fields[:-1])}, and {fields[-1][0]}")
                break
    access = schemas.get("AccessSummary", {})
    for field, expected_type in [
        ("defaultMode", "string"), ("modelStrategy", "string"), ("networkEnabledHint", "boolean"),
        ("onboardingCompleted", "boolean"), ("sessionExpiresAt", "string"), ("sessionId", "string"),
        ("userEmail", "string"), ("userId", "string"), ("workspaceId", "string"),
    ]:
        if field not in required(access) or dig(access, "properties", field, "type") != expected_type:
            missing.append(f"AccessSummary must require {field} as {expected_type}")
    token_props = props(schemas.get("RelayAPIToken", {}))
    if "rawToken" in token_props:
        missing.append("RelayAPIToken list schema must not expose rawToken")
    token_usage = props(schemas.get("ConsoleAPITokenUsageItem", {}))
    for field, expected_type in [
        ("apiTokenId", "string"), ("requestId", "string"), ("apiType", "string"), ("model", "string"),
        ("status", "string"), ("statusCode", "integer"), ("latencyMs", "integer"), ("cost", "number"),
        ("promptTokens", "integer"), ("completionTokens", "integer"), ("totalTokens", "integer"), ("createdAt", "string"),
    ]:
        if dig(token_usage, field, "type") != expected_type:
            missing.append(f"ConsoleAPITokenUsageItem must document {field} as {expected_type}")
    for field in ["provider", "channelId", "channel_id"]:
        if field in token_usage:
            missing.append(f"ConsoleAPITokenUsageItem must not expose {field}")
    fail("[openapi-contract] Billing checkout contract is incomplete:", missing)


def require_quota_topup_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    for (path, method), (shape, expected) in {
        ("/api/v1/app/quota", "get"): ("ref", "#/components/schemas/QuotaSnapshot"),
        ("/api/v1/app/packages", "get"): ("array", "#/components/schemas/PackageOption"),
    }.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Billing"):
            missing.append(f"{method.upper()} {path} must be tagged Billing")
        actual = response_data_array_ref(op, "200") if shape == "array" else response_data_ref(op, "200")
        if actual != expected:
            missing.append(f"{method.upper()} {path} 200 data must reference {expected}")
    op = operation(paths, "/api/v1/app/quota/topup", "post", missing)
    if not requires_cookie_and_csrf(op):
        missing.append("POST /api/v1/app/quota/topup must require cookieAuth and csrfHeader")
    if not has_tags(op, "Billing"):
        missing.append("POST /api/v1/app/quota/topup must be tagged Billing")
    if request_body_ref(op) != "#/components/schemas/QuotaTopupRequest":
        missing.append("POST /api/v1/app/quota/topup request body must reference QuotaTopupRequest")
    if dig(op, "responses", "402", "content", "application/json", "schema", "$ref") != "#/components/schemas/Envelope":
        missing.append("POST /api/v1/app/quota/topup 402 response must reference Envelope")
    if not (
        "amount" in required(schemas.get("QuotaTopupRequest", {}))
        and dig(schemas, "QuotaTopupRequest", "properties", "amount", "type") == "number"
        and dig(schemas, "QuotaTopupRequest", "properties", "amount", "exclusiveMinimum") == 0
    ):
        missing.append("QuotaTopupRequest must require a positive numeric amount")
    fail("[openapi-contract] Quota top-up CSRF/schema contract is incomplete:", missing)


def require_tenant_organization_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    for (path, method), expected in {
        ("/api/v1/app/organizations", "get"): "#/components/schemas/OrganizationMembershipListResponse",
        ("/api/v1/app/organizations/{organizationId}/members", "get"): "#/components/schemas/OrganizationMembersResponse",
    }.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Tenant"):
            missing.append(f"{method.upper()} {path} must be tagged Tenant")
        if response_data_ref(op, "200") != expected:
            missing.append(f"{method.upper()} {path} 200 data must reference {expected}")
    check_mutation_security(paths, [
        ("/api/v1/app/organizations/{organizationId}/select", "post"),
        ("/api/v1/app/organizations/{organizationId}/members/{userId}", "put"),
        ("/api/v1/app/organizations/{organizationId}/members/{userId}", "delete"),
        ("/api/v1/app/organizations/{organizationId}/invitations", "post"),
        ("/api/v1/app/organizations/{organizationId}/invitations/{invitationId}/revoke", "post"),
        ("/api/v1/app/organizations/{organizationId}/ownership-transfer", "post"),
        ("/api/v1/app/organization-invitations/{token}/accept", "post"),
    ], missing, "Tenant")
    for (path, method), expected in {
        ("/api/v1/app/organizations/{organizationId}/members/{userId}", "put"): "#/components/schemas/UpdateOrganizationMemberRoleRequest",
        ("/api/v1/app/organizations/{organizationId}/invitations", "post"): "#/components/schemas/InviteOrganizationMemberRequest",
        ("/api/v1/app/organizations/{organizationId}/ownership-transfer", "post"): "#/components/schemas/TransferOrganizationOwnershipRequest",
    }.items():
        op = operation(paths, path, method, missing)
        if dig(op, "requestBody", "required") is not True or request_body_ref(op) != expected:
            missing.append(f"{method.upper()} {path} request body must require {expected}")
    transfer = operation(paths, "/api/v1/app/organizations/{organizationId}/ownership-transfer", "post", missing)
    if response_data_ref(transfer, "200") != "#/components/schemas/OrganizationOwnershipTransferResponse":
        missing.append("POST /api/v1/app/organizations/{organizationId}/ownership-transfer 200 data must reference OrganizationOwnershipTransferResponse")
    if dig(schemas, "OrganizationMembershipListResponse", "properties", "memberships", "items", "$ref") != "#/components/schemas/OrganizationMembership":
        missing.append("OrganizationMembershipListResponse must expose memberships[]")
    if dig(schemas, "OrganizationMembersResponse", "properties", "members", "items", "$ref") != "#/components/schemas/OrganizationMembership":
        missing.append("OrganizationMembersResponse must expose members[]")
    if dig(schemas, "OrganizationOwnershipTransferResponse", "properties", "transferred", "type") != "boolean":
        missing.append("OrganizationOwnershipTransferResponse.transferred must be boolean")
    expected_roles = ["admin", "member"]
    if enum_values(schemas.get("OrganizationInvitation", {}), "properties", "role") != expected_roles:
        missing.append("OrganizationInvitation.role enum must match invitation runtime roles admin/member")
    if enum_values(schemas.get("InviteOrganizationMemberRequest", {}), "properties", "role") != expected_roles:
        missing.append("InviteOrganizationMemberRequest.role enum must match invitation runtime roles admin/member")
    fail("[openapi-contract] Tenant organization mutation CSRF contract is incomplete:", missing)


def require_workflow_execution_control_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    control_paths = [
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check", "post"),
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/decision", "post"),
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/pause", "post"),
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/resume", "post"),
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/cancel", "post"),
    ]
    for path, method in control_paths:
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Workflow"):
            missing.append(f"{method.upper()} {path} must be tagged Workflow")
        if response_ref(op, "200") != "#/components/schemas/WorkflowExecutionEnvelope":
            missing.append(f"{method.upper()} {path} 200 response must reference WorkflowExecutionEnvelope")
    for (path, method), expected in {
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check", "post"): "#/components/schemas/WorkflowResourceCheckRequest",
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/decision", "post"): "#/components/schemas/WorkflowFailureDecisionRequest",
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/resume", "post"): "#/components/schemas/WorkflowResumeExecutionRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    decision_schema = schemas.get("WorkflowFailureDecisionRequest")
    one_of = decision_schema.get("oneOf") if isinstance(decision_schema, dict) else None
    if not isinstance(one_of, list) or len(one_of) != 2:
        missing.append("WorkflowFailureDecisionRequest must model branch-specific nextNodeId requirements with oneOf")
    else:
        branch = next((entry for entry in one_of if dig(entry, "properties", "action", "enum") == ["branch"]), None)
        non_branch = next((entry for entry in one_of if dig(entry, "properties", "action", "enum") == ["retry", "retry_with_input", "edit_input_retry", "continue", "skip", "fail", "terminate"]), None)
        if not isinstance(branch, dict) or "nextNodeId" not in required(branch):
            missing.append("WorkflowFailureDecisionRequest.branch variant must require nextNodeId")
        if not isinstance(non_branch, dict) or "action" not in required(non_branch):
            missing.append("WorkflowFailureDecisionRequest non-branch variant must require action")
    fail("[openapi-contract] Workflow execution control CSRF contract is incomplete:", missing)


def require_workflow_management_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    workflow_mutations = [
        ("/api/v1/workflows", "post"),
        ("/api/v1/workflows/semantic-matches", "post"),
        ("/api/v1/workflows/conversation-matches", "post"),
        ("/api/v1/workflows/debug-retention/prune", "post"),
        ("/api/v1/workflows/{workflowId}", "put"),
        ("/api/v1/workflows/{workflowId}", "delete"),
        ("/api/v1/workflows/{workflowId}/execute", "post"),
        ("/api/v1/workflows/{workflowId}/webhook", "post"),
        ("/api/v1/workflows/{workflowId}/branches", "post"),
        ("/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "post"),
        ("/api/v1/workflows/{workflowId}/branches/{branchId}/merge", "post"),
        ("/api/v1/workflows/{workflowId}/rollback", "post"),
        ("/api/v1/workflows/{workflowId}/test-node", "post"),
    ]
    workflow_reads = {
        ("/api/v1/workflows", "get", "200"): "#/components/schemas/WorkflowDefinitionsEnvelope",
        ("/api/v1/workflows/{workflowId}", "get", "200"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}/versions", "get", "200"): "#/components/schemas/WorkflowDefinitionsEnvelope",
        ("/api/v1/workflows/{workflowId}/executions", "get", "200"): "#/components/schemas/WorkflowExecutionsEnvelope",
        ("/api/v1/workflows/{workflowId}/executions/{executionId}", "get", "200"): "#/components/schemas/WorkflowExecutionEnvelope",
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/debug-snapshot", "get", "200"): "#/components/schemas/WorkflowExecutionDebugSnapshotEnvelope",
        ("/api/v1/workflows/{workflowId}/executions/{executionId}/state-replay", "get", "200"): "#/components/schemas/WorkflowExecutionStateReplayEnvelope",
    }
    for (path, method, status), expected in workflow_reads.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Workflow"):
            missing.append(f"{method.upper()} {path} must be tagged Workflow")
        if response_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} response must reference {expected}")
    for path, method in workflow_mutations:
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Workflow"):
            missing.append(f"{method.upper()} {path} must be tagged Workflow")
    for (path, method), expected in {
        ("/api/v1/workflows", "post"): "#/components/schemas/CreateWorkflowRequest",
        ("/api/v1/workflows/semantic-matches", "post"): "#/components/schemas/WorkflowSemanticMatchRequest",
        ("/api/v1/workflows/conversation-matches", "post"): "#/components/schemas/WorkflowConversationMatchRequest",
        ("/api/v1/workflows/debug-retention/prune", "post"): "#/components/schemas/WorkflowExecutionDebugRetentionPruneRequest",
        ("/api/v1/workflows/{workflowId}", "put"): "#/components/schemas/UpdateWorkflowRequest",
        ("/api/v1/workflows/{workflowId}/execute", "post"): "#/components/schemas/ExecuteWorkflowRequest",
        ("/api/v1/workflows/{workflowId}/branches", "post"): "#/components/schemas/CreateWorkflowBranchRequest",
        ("/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "post"): "#/components/schemas/PublishWorkflowBranchRequest",
        ("/api/v1/workflows/{workflowId}/rollback", "post"): "#/components/schemas/RollbackWorkflowRequest",
        ("/api/v1/workflows/{workflowId}/test-node", "post"): "#/components/schemas/WorkflowNodeTestRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    for (path, method, status), expected in {
        ("/api/v1/workflows", "post", "201"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/semantic-matches", "post", "200"): "#/components/schemas/WorkflowSemanticMatchesEnvelope",
        ("/api/v1/workflows/conversation-matches", "post", "200"): "#/components/schemas/WorkflowConversationMatchesEnvelope",
        ("/api/v1/workflows/debug-retention/prune", "post", "200"): "#/components/schemas/WorkflowExecutionDebugRetentionPruneResultEnvelope",
        ("/api/v1/workflows/{workflowId}", "put", "200"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}", "delete", "200"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}/execute", "post", "201"): "#/components/schemas/WorkflowExecutionEnvelope",
        ("/api/v1/workflows/{workflowId}/webhook", "post", "201"): "#/components/schemas/WorkflowExecutionEnvelope",
        ("/api/v1/workflows/{workflowId}/branches", "post", "201"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "post", "201"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}/branches/{branchId}/merge", "post", "200"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}/rollback", "post", "200"): "#/components/schemas/WorkflowDefinitionEnvelope",
        ("/api/v1/workflows/{workflowId}/test-node", "post", "200"): "#/components/schemas/WorkflowNodeTestResultEnvelope",
    }.items():
        if response_ref(operation(paths, path, method, missing), status) != expected:
            missing.append(f"{method.upper()} {path} {status} response must reference {expected}")
    signed = operation(paths, "/api/v1/workflows/webhooks/{organizationId}/{workflowId}", "post", missing)
    if signed.get("security") != []:
        missing.append("POST /api/v1/workflows/webhooks/{organizationId}/{workflowId} must remain explicitly public with security: []")
    if dig(signed, "responses", "201", "content", "application/json", "schema", "$ref") != "#/components/schemas/WorkflowExecutionEnvelope":
        missing.append("POST /api/v1/workflows/webhooks/{organizationId}/{workflowId} 201 response must reference WorkflowExecutionEnvelope")
    if not (
        includes_all(required(schemas.get("CreateWorkflowRequest", {})), ["name", "definition"])
        and "version" in required(schemas.get("RollbackWorkflowRequest", {}))
        and includes_all(required(schemas.get("CreateWorkflowBranchRequest", {})), ["name", "version"])
    ):
        missing.append("Workflow create, rollback, and branch request schemas must preserve required fields")
    for schema_name, fields in {
        "WorkflowDefinition": ["id", "organizationId", "name", "status", "version", "definition", "createdAt", "updatedAt"],
        "WorkflowExecutionDebugSnapshot": ["executionId", "workflowId", "status", "variableSnapshot", "events", "stateReplay", "trace", "outputs", "performance", "logs"],
        "WorkflowExecutionDebugRetentionPruneRequest": ["before"],
        "WorkflowExecutionDebugRetentionPruneResult": ["traceEntriesDeleted", "variableSnapshotsDeleted"],
        "WorkflowExecutionEvent": ["id", "executionId", "organizationId", "eventType", "toStatus", "createdAt"],
        "WorkflowStateReplay": ["initialStatus", "finalStatus", "valid", "transitions"],
        "WorkflowStateReplayTransition": ["event", "fromStatus", "toStatus", "createdAt"],
        "WorkflowDebugVariableSnapshot": ["input", "context", "nodeOutputs"],
        "WorkflowDebugTraceEntry": ["nodeId", "nodeType", "status", "startedAt"],
        "WorkflowDebugPerformance": ["totalDurationMs", "nodeDurationsMs"],
        "WorkflowDebugLogEntry": ["level", "message", "timestamp"],
    }.items():
        check_required_fields(schemas, schema_name, fields, missing)
    fail("[openapi-contract] Workflow management CSRF/schema contract is incomplete:", missing)


def require_console_api_token_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    list_op = operation(paths, "/api/v1/console/api-tokens", "get", missing)
    create = operation(paths, "/api/v1/console/api-tokens", "post", missing)
    revoke = operation(paths, "/api/v1/console/api-tokens/{tokenId}", "delete", missing)
    usage = operation(paths, "/api/v1/console/api-tokens/{tokenId}/usage", "get", missing)
    for label, op in {"POST /api/v1/console/api-tokens": create, "DELETE /api/v1/console/api-tokens/{tokenId}": revoke}.items():
        if not requires_cookie_and_csrf(op):
            missing.append(f"{label} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Billing"):
            missing.append(f"{label} must be tagged Billing")
    for label, op in {"GET /api/v1/console/api-tokens": list_op, "GET /api/v1/console/api-tokens/{tokenId}/usage": usage}.items():
        if not requires_cookie_without_csrf(op):
            missing.append(f"{label} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Billing"):
            missing.append(f"{label} must be tagged Billing")
    if response_data_array_ref(list_op, "200") != "#/components/schemas/RelayAPIToken":
        missing.append("GET /api/v1/console/api-tokens 200 data items must reference RelayAPIToken")
    if response_data_array_ref(usage, "200") != "#/components/schemas/ConsoleAPITokenUsageItem":
        missing.append("GET /api/v1/console/api-tokens/{tokenId}/usage 200 data items must reference ConsoleAPITokenUsageItem")
    if dig(create, "requestBody", "required") is not True or request_body_ref(create) != "#/components/schemas/CreateRelayAPITokenRequest":
        missing.append("POST /api/v1/console/api-tokens request body must require CreateRelayAPITokenRequest")
    if response_data_ref(create, "201") != "#/components/schemas/CreatedRelayAPIToken":
        missing.append("POST /api/v1/console/api-tokens 201 data must reference CreatedRelayAPIToken")
    if response_data_ref(revoke, "200") != "#/components/schemas/RelayAPITokenRevokeResponse":
        missing.append("DELETE /api/v1/console/api-tokens/{tokenId} 200 data must reference RelayAPITokenRevokeResponse")
    revoke_schema = schemas.get("RelayAPITokenRevokeResponse", {})
    if not ("status" in required(revoke_schema) and dig(revoke_schema, "properties", "status", "type") == "string" and enum_values(revoke_schema, "properties", "status") == ["revoked"]):
        missing.append("RelayAPITokenRevokeResponse.status must be required and enumerate revoked")
    relay_props = props(schemas.get("RelayAPIToken", {}))
    for field in ["id", "name", "tokenPrefix", "status", "modelLimitsEnabled", "modelLimits", "usedQuota", "createdAt"]:
        if field not in relay_props:
            missing.append(f"RelayAPIToken.{field} must be documented for console list responses")
    for field in ["rawToken", "tokenHash", "token_hash"]:
        if field in relay_props:
            missing.append(f"RelayAPIToken list schema must not expose {field}")
    created_props = props(schemas.get("CreatedRelayAPIToken", {}))
    if "rawToken" not in created_props or dig(created_props, "token", "$ref") != "#/components/schemas/RelayAPIToken":
        missing.append("CreatedRelayAPIToken must expose one-time rawToken plus RelayAPIToken token")
    usage_props = props(schemas.get("ConsoleAPITokenUsageItem", {}))
    for field in ["id", "apiTokenId", "requestId", "apiType", "model", "status", "statusCode", "totalTokens", "createdAt"]:
        if field not in usage_props:
            missing.append(f"ConsoleAPITokenUsageItem.{field} must be documented for console usage responses")
    for field in ["rawToken", "tokenHash", "token_hash", "provider", "channelId", "channel_id"]:
        if field in usage_props:
            missing.append(f"ConsoleAPITokenUsageItem must not expose {field}")
    fail("[openapi-contract] Console API token CSRF contract is incomplete:", missing)


def require_admin_api_token_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    list_op = operation(paths, "/api/v1/admin/api-tokens", "get", missing)
    revoke = operation(paths, "/api/v1/admin/api-tokens/{tokenId}/revoke", "post", missing)
    for label, op in {"GET /api/v1/admin/api-tokens": list_op, "POST /api/v1/admin/api-tokens/{tokenId}/revoke": revoke}.items():
        if not has_tags(op, "Admin"):
            missing.append(f"{label} must be tagged Admin")
        if not has_tags(op, "Relay"):
            missing.append(f"{label} must be tagged Relay")
    if not requires_cookie_and_csrf(revoke):
        missing.append("POST /api/v1/admin/api-tokens/{tokenId}/revoke must require cookieAuth and csrfHeader")
    if not requires_cookie_without_csrf(list_op):
        missing.append("GET /api/v1/admin/api-tokens must require cookieAuth without csrfHeader")
    if response_data_ref(list_op, "200") != "#/components/schemas/AdminAPITokenListResponse":
        missing.append("GET /api/v1/admin/api-tokens 200 data must reference AdminAPITokenListResponse")
    if response_data_ref(revoke, "200") != "#/components/schemas/RelayAPITokenRevokeResponse":
        missing.append("POST /api/v1/admin/api-tokens/{tokenId}/revoke 200 data must reference RelayAPITokenRevokeResponse")
    parameter_names = [param.get("name") for param in list_op.get("parameters", []) if isinstance(param, dict)]
    for name in ["organizationID", "userID", "status", "userGroup", "search", "model", "limit", "offset"]:
        if name not in parameter_names:
            missing.append(f"GET /api/v1/admin/api-tokens must document {name} query parameter")
    if dig(schemas, "AdminAPITokenListResponse", "properties", "apiTokens", "items", "$ref") != "#/components/schemas/AdminAPIToken" or dig(schemas, "AdminAPITokenListResponse", "properties", "total", "type") != "integer":
        missing.append("AdminAPITokenListResponse must expose apiTokens[] plus integer total")
    admin_props = props(schemas.get("AdminAPIToken", {}))
    for field in ["id", "organizationId", "userId", "userEmail", "name", "tokenPrefix", "status", "modelLimitsEnabled", "modelLimits", "quotaLimit", "usedQuota", "requestCount", "totalCost", "createdAt"]:
        if field not in admin_props:
            missing.append(f"AdminAPIToken.{field} must be documented")
    relay_props = props(schemas.get("RelayAPIToken", {}))
    for field in ["rawToken", "tokenHash", "token_hash"]:
        if field in admin_props:
            missing.append(f"AdminAPIToken must not expose {field}")
        if field in relay_props:
            missing.append(f"RelayAPIToken list schema must not expose {field}")
    fail("[openapi-contract] Admin API token contract is incomplete:", missing)


def require_task_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    for (path, method), (shape, expected) in {
        ("/api/v1/app/tasks", "get"): ("array", "#/components/schemas/Task"),
        ("/api/v1/app/tasks/{taskId}", "get"): ("ref", "#/components/schemas/TaskDetail"),
    }.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Task"):
            missing.append(f"{method.upper()} {path} must be tagged Task")
        actual = response_data_array_ref(op, "200") if shape == "array" else response_data_ref(op, "200")
        if actual != expected:
            missing.append(f"{method.upper()} {path} 200 data must reference {expected}")
    expected_mutation_responses = {
        ("/api/v1/app/tasks", "post"): "#/components/schemas/Task",
        ("/api/v1/app/tasks/{taskId}/start", "post"): "#/components/schemas/TaskDetail",
        ("/api/v1/app/tasks/{taskId}/approve", "post"): "#/components/schemas/TaskDetail",
        ("/api/v1/app/tasks/{taskId}/pause", "post"): "#/components/schemas/TaskDetail",
        ("/api/v1/app/tasks/{taskId}/resume", "post"): "#/components/schemas/TaskDetail",
        ("/api/v1/app/tasks/{taskId}/cancel", "post"): "#/components/schemas/TaskDetail",
        ("/api/v1/app/tasks/{taskId}/budget", "post"): "#/components/schemas/TaskDetail",
    }
    for (path, method), expected in expected_mutation_responses.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Task"):
            missing.append(f"{method.upper()} {path} must be tagged Task")
        if response_data_ref(op, "200") != expected:
            missing.append(f"{method.upper()} {path} 200 data must reference {expected}")
    create = operation(paths, "/api/v1/app/tasks", "post", missing)
    if dig(create, "requestBody", "required") is not True or request_body_ref(create) != "#/components/schemas/CreateTaskRequest":
        missing.append("POST /api/v1/app/tasks request body must require CreateTaskRequest")
    budget = operation(paths, "/api/v1/app/tasks/{taskId}/budget", "post", missing)
    if dig(budget, "requestBody", "required") is not True or request_body_ref(budget) != "#/components/schemas/UpdateTaskBudgetRequest":
        missing.append("POST /api/v1/app/tasks/{taskId}/budget request body must require UpdateTaskBudgetRequest")
    create_request = schemas.get("CreateTaskRequest", {})
    if not includes_all(enum_values(create_request, "properties", "executionMode"), ["standard", "safe", "auto"]) or any(mode in enum_values(create_request, "properties", "executionMode") for mode in ["semi-auto", "manual"]):
        missing.append("CreateTaskRequest.executionMode must enumerate runtime modes standard, safe, and auto only")
    if not includes_all(enum_values(create_request, "properties", "authorizationScope"), ["knowledge_only", "workspace_tools", "full_access"]):
        missing.append("CreateTaskRequest.authorizationScope must enumerate runtime scopes")
    if dig(create_request, "properties", "goal", "minLength") != 1 or dig(create_request, "properties", "goal", "pattern") != "\\S":
        missing.append("CreateTaskRequest.goal must document the runtime non-blank goal requirement")
    task = schemas.get("Task", {})
    if not includes_all(enum_values(task, "properties", "executionMode"), ["standard", "safe", "auto"]):
        missing.append("Task.executionMode must enumerate standard, safe, and auto")
    if not includes_all(enum_values(task, "properties", "status"), ["draft", "running", "paused", "awaiting_confirmation", "completed", "cancelled"]):
        missing.append("Task.status must document runtime lifecycle statuses")
    if "workspace_tools" not in enum_values(task, "properties", "authorizationScope") or dig(task, "properties", "budgetConsumed", "type") != "integer":
        missing.append("Task must expose authorizationScope and budgetConsumed runtime fields")
    check_required_fields(schemas, "Task", ["id", "title", "goal", "status", "executionMode", "authorizationScope", "budgetLimit", "budgetConsumed", "createdAt", "updatedAt"], missing)
    check_required_fields(schemas, "TaskStep", ["id", "title", "status", "stepIndex", "createdAt", "updatedAt"], missing)
    check_required_fields(schemas, "TaskEvent", ["type", "message", "createdAt"], missing)
    check_required_fields(schemas, "TaskResultArtifact", ["label", "value"], missing)
    task_detail = schemas.get("TaskDetail", {})
    detail_refs = [entry.get("$ref") for entry in task_detail.get("allOf", []) if isinstance(entry, dict)]
    extension = next((entry for entry in task_detail.get("allOf", []) if isinstance(entry, dict) and isinstance(entry.get("properties"), dict)), {})
    detail_props = props(extension)
    if not (
        "#/components/schemas/Task" in detail_refs
        and dig(detail_props, "steps", "items", "$ref") == "#/components/schemas/TaskStep"
        and dig(detail_props, "events", "items", "$ref") == "#/components/schemas/TaskEvent"
        and dig(detail_props, "resultArtifacts", "items", "$ref") == "#/components/schemas/TaskResultArtifact"
        and dig(detail_props, "knowledgeBaseIds", "items", "type") == "string"
        and dig(detail_props, "toolAllowList", "items", "type") == "string"
        and dig(detail_props, "toolDenyList", "items", "type") == "string"
    ):
        missing.append("TaskDetail must extend Task and expose steps, events, resultArtifacts, knowledgeBaseIds, and tool rule arrays")
    for field in ["events", "knowledgeBaseIds", "resultArtifacts", "steps", "toolAllowList", "toolDenyList"]:
        if field not in required(extension):
            missing.append(f"TaskDetail must require {field}")
    fail("[openapi-contract] Task mutation CSRF contract is incomplete:", missing)


def require_notification_mutation_csrf_contract(paths: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_responses = {
        ("/api/v1/app/notifications", "post"): ("201", "#/components/schemas/Notification"),
        ("/api/v1/app/notifications/mark-all-read", "post"): ("200", "#/components/schemas/NotificationActionStatus"),
        ("/api/v1/app/notifications/{notificationId}", "patch"): ("200", "#/components/schemas/NotificationActionStatus"),
        ("/api/v1/app/notifications/{notificationId}", "delete"): ("200", "#/components/schemas/NotificationActionStatus"),
    }
    for (path, method), (status, expected) in expected_responses.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Notification"):
            missing.append(f"{method.upper()} {path} must be tagged Notification")
        if response_data_ref(op, status) != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    create = operation(paths, "/api/v1/app/notifications", "post", missing)
    if dig(create, "requestBody", "required") is not True or request_body_ref(create) != "#/components/schemas/CreateNotificationRequest":
        missing.append("POST /api/v1/app/notifications request body must require CreateNotificationRequest")
    fail("[openapi-contract] Notification mutation CSRF contract is incomplete:", missing)


def require_scheduled_task_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    if enum_values(schemas.get("ScheduledTask", {}), "properties", "targetType") != ["workflow", "agent"]:
        missing.append("ScheduledTask.targetType must enumerate workflow and agent")
    check_required_fields(schemas, "ScheduledTask", ["id", "organizationId", "name", "targetType", "targetId", "cronExpression", "enabled", "createdAt", "updatedAt"], missing)
    if enum_values(schemas.get("ScheduledTaskRun", {}), "properties", "status") != ["queued", "running", "completed", "failed", "cancelled"]:
        missing.append("ScheduledTaskRun.status must enumerate queued, running, completed, failed, and cancelled")
    check_required_fields(schemas, "ScheduledTaskRun", ["id", "organizationId", "scheduledTaskId", "status", "createdAt", "updatedAt"], missing)
    if not includes_all(required(schemas.get("CreateScheduledTaskRequest", {})), ["name", "targetType", "targetId", "cronExpression"]):
        missing.append("CreateScheduledTaskRequest must require name, targetType, targetId, and cronExpression")
    create_props = props(schemas.get("CreateScheduledTaskRequest", {}))
    if dig(create_props, "name", "minLength") != 1 or dig(create_props, "name", "pattern") != "\\S":
        missing.append("CreateScheduledTaskRequest.name must document non-blank task names")
    if dig(create_props, "targetId", "minLength") != 1 or dig(create_props, "targetId", "pattern") != "\\S":
        missing.append("CreateScheduledTaskRequest.targetId must document non-blank target IDs")
    if not (
        dig(create_props, "cronExpression", "minLength") == 1
        and dig(create_props, "cronExpression", "pattern") == "^\\s*(?:(?:\\S+\\s+){4}\\S+|@[A-Za-z]+(?:\\s+\\S+)?)\\s*$"
        and "five-field cron expression or supported descriptor" in str(dig(create_props, "cronExpression", "description") or "")
    ):
        missing.append("CreateScheduledTaskRequest.cronExpression must document non-blank five-field cron or descriptor shape")
    if "enabled" not in required(schemas.get("UpdateScheduledTaskStatusRequest", {})):
        missing.append("UpdateScheduledTaskStatusRequest must require enabled")
    list_op = operation(paths, "/api/v1/scheduled-tasks", "get", missing)
    if not requires_cookie_without_csrf(list_op):
        missing.append("GET /api/v1/scheduled-tasks must require cookieAuth without csrfHeader")
    if not has_tags(list_op, "ScheduledTask") or response_data_array_ref(list_op, "200") != "#/components/schemas/ScheduledTask":
        missing.append("GET /api/v1/scheduled-tasks must be tagged ScheduledTask and return ScheduledTask[] data")
    runs = operation(paths, "/api/v1/scheduled-tasks/{scheduledTaskId}/runs", "get", missing)
    if not requires_cookie_without_csrf(runs):
        missing.append("GET /api/v1/scheduled-tasks/{scheduledTaskId}/runs must require cookieAuth without csrfHeader")
    if not has_tags(runs, "ScheduledTask") or response_data_array_ref(runs, "200") != "#/components/schemas/ScheduledTaskRun":
        missing.append("GET /api/v1/scheduled-tasks/{scheduledTaskId}/runs must be tagged ScheduledTask and return ScheduledTaskRun[] data")
    expected_mutations = {
        ("/api/v1/scheduled-tasks", "post"): ("201", "#/components/schemas/CreateScheduledTaskRequest", "#/components/schemas/ScheduledTask"),
        ("/api/v1/scheduled-tasks/{scheduledTaskId}/status", "patch"): ("200", "#/components/schemas/UpdateScheduledTaskStatusRequest", "#/components/schemas/ScheduledTask"),
        ("/api/v1/scheduled-tasks/{scheduledTaskId}/run", "post"): ("202", None, "#/components/schemas/ScheduledTaskRun"),
    }
    for (path, method), (status, request_ref, response_ref_value) in expected_mutations.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "ScheduledTask"):
            missing.append(f"{method.upper()} {path} must be tagged ScheduledTask")
        if request_ref and (dig(op, "requestBody", "required") is not True or request_body_ref(op) != request_ref):
            missing.append(f"{method.upper()} {path} request body must require {request_ref}")
        if response_data_ref(op, status) != response_ref_value:
            missing.append(f"{method.upper()} {path} {status} data must reference {response_ref_value}")
    fail("[openapi-contract] Scheduled Task route/schema contract is incomplete:", missing)


def require_preferences_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    preferences = operation(paths, "/api/v1/app/me/preferences", "put", missing)
    get_preferences = operation(paths, "/api/v1/app/me/preferences", "get", missing)
    if response_data_ref(get_preferences, "200") != "#/components/schemas/Preferences":
        missing.append("GET /api/v1/app/me/preferences 200 data must reference Preferences")
    if not requires_cookie_and_csrf(preferences):
        missing.append("PUT /api/v1/app/me/preferences must require cookieAuth and csrfHeader")
    if not has_tags(preferences, "Preferences"):
        missing.append("PUT /api/v1/app/me/preferences must be tagged Preferences")
    if dig(preferences, "requestBody", "required") is not True or request_body_ref(preferences) != "#/components/schemas/UpdatePreferencesRequest":
        missing.append("PUT /api/v1/app/me/preferences request body must require UpdatePreferencesRequest")
    if response_data_ref(preferences, "200") != "#/components/schemas/Preferences":
        missing.append("PUT /api/v1/app/me/preferences 200 data must reference Preferences")
    expected_fields = {
        "defaultMode": "string",
        "modelStrategy": "string",
        "networkEnabledHint": "boolean",
        "onboardingCompleted": "boolean",
        "defaultAgentModel": "string",
        "sidebarCollapsed": "boolean",
        "notifications": "object",
    }
    for schema_name in ["Preferences", "UpdatePreferencesRequest"]:
        for field, expected_type in expected_fields.items():
            if dig(schemas, schema_name, "properties", field, "type") != expected_type:
                missing.append(f"{schema_name}.{field} must be documented as {expected_type}")
        if dig(schemas, schema_name, "properties", "notifications", "additionalProperties") is not True:
            missing.append(f"{schema_name}.notifications must allow object properties")
    fail("[openapi-contract] Preferences mutation CSRF contract is incomplete:", missing)


def require_chat_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_responses = {
        ("/api/v1/app/conversations", "post"): ("200", "#/components/schemas/Conversation", "ref"),
        ("/api/v1/app/conversations/{conversationId}", "put"): ("200", "#/components/schemas/Conversation", "ref"),
        ("/api/v1/app/conversations/{conversationId}", "delete"): ("200", "#/components/schemas/ConversationDeleteResponse", "ref"),
        ("/api/v1/app/conversations/{conversationId}/fork", "post"): ("200", "#/components/schemas/Conversation", "ref"),
        ("/api/v1/app/conversations/{conversationId}/messages", "post"): ("200", "#/components/schemas/Message", "array"),
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}", "put"): ("200", "#/components/schemas/Message", "ref"),
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}", "delete"): ("200", "#/components/schemas/MessageDeleteResponse", "ref"),
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}/bookmark", "post"): ("200", "#/components/schemas/Message", "ref"),
        ("/api/v1/app/conversations/{conversationId}/config", "put"): ("200", "#/components/schemas/ConversationConfig", "ref"),
        ("/api/v1/app/conversations/{conversationId}/convert-to-task", "post"): ("200", "#/components/schemas/TaskDraft", "ref"),
        ("/api/v1/app/conversations/{conversationId}/share", "post"): ("201", "#/components/schemas/ConversationShareResponse", "ref"),
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}/share", "post"): ("201", "#/components/schemas/MessageShareResponse", "ref"),
        ("/api/v1/conversations", "post"): ("200", "#/components/schemas/Conversation", "ref"),
        ("/api/v1/conversations/{conversationId}", "put"): ("200", "#/components/schemas/Conversation", "ref"),
        ("/api/v1/conversations/{conversationId}", "delete"): ("200", "#/components/schemas/ConversationDeleteResponse", "ref"),
        ("/api/v1/conversations/{conversationId}/fork", "post"): ("200", "#/components/schemas/Conversation", "ref"),
        ("/api/v1/conversations/{conversationId}/messages", "post"): ("200", "#/components/schemas/Message", "array"),
        ("/api/v1/conversations/{conversationId}/messages/{messageId}", "put"): ("200", "#/components/schemas/Message", "ref"),
        ("/api/v1/conversations/{conversationId}/messages/{messageId}", "delete"): ("200", "#/components/schemas/MessageDeleteResponse", "ref"),
        ("/api/v1/conversations/{conversationId}/messages/{messageId}/bookmark", "post"): ("200", "#/components/schemas/Message", "ref"),
        ("/api/v1/conversations/{conversationId}/messages/{messageId}/share", "post"): ("201", "#/components/schemas/MessageShareResponse", "ref"),
        ("/api/v1/app/personas", "post"): ("200", "#/components/schemas/Persona", "ref"),
        ("/api/v1/app/personas/{personaId}", "put"): ("200", "#/components/schemas/Persona", "ref"),
        ("/api/v1/app/personas/{personaId}", "delete"): ("200", "#/components/schemas/PersonaDeleteResponse", "ref"),
    }
    for (path, method), (status, expected, shape) in expected_responses.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if not has_tags(op, "Chat"):
            missing.append(f"{method.upper()} {path} must be tagged Chat")
        actual = response_data_array_ref(op, status) if shape == "array" else response_data_ref(op, status)
        if actual != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    for (path, method), expected in {
        ("/api/v1/app/conversations", "post"): "#/components/schemas/CreateConversationRequest",
        ("/api/v1/app/conversations/{conversationId}", "put"): "#/components/schemas/UpdateConversationRequest",
        ("/api/v1/app/conversations/{conversationId}/fork", "post"): "#/components/schemas/ForkConversationRequest",
        ("/api/v1/app/conversations/{conversationId}/messages", "post"): "#/components/schemas/SendMessageRequest",
        ("/api/v1/app/conversations/{conversationId}/messages/stream", "post"): "#/components/schemas/SendMessageRequest",
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}", "put"): "#/components/schemas/UpdateMessageRequest",
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}/bookmark", "post"): "#/components/schemas/BookmarkMessageRequest",
        ("/api/v1/app/conversations/{conversationId}/config", "put"): "#/components/schemas/UpdateConversationConfigRequest",
        ("/api/v1/app/conversations/{conversationId}/share", "post"): "#/components/schemas/CreateConversationShareRequest",
        ("/api/v1/app/conversations/{conversationId}/messages/{messageId}/share", "post"): "#/components/schemas/CreateMessageShareRequest",
        ("/api/v1/conversations", "post"): "#/components/schemas/CreateConversationRequest",
        ("/api/v1/conversations/{conversationId}", "put"): "#/components/schemas/UpdateConversationRequest",
        ("/api/v1/conversations/{conversationId}/fork", "post"): "#/components/schemas/ForkConversationRequest",
        ("/api/v1/conversations/{conversationId}/messages", "post"): "#/components/schemas/SendMessageRequest",
        ("/api/v1/conversations/{conversationId}/messages/{messageId}", "put"): "#/components/schemas/UpdateMessageRequest",
        ("/api/v1/conversations/{conversationId}/messages/{messageId}/bookmark", "post"): "#/components/schemas/BookmarkMessageRequest",
        ("/api/v1/conversations/{conversationId}/messages/{messageId}/share", "post"): "#/components/schemas/CreateMessageShareRequest",
        ("/api/v1/app/personas", "post"): "#/components/schemas/PersonaRequest",
        ("/api/v1/app/personas/{personaId}", "put"): "#/components/schemas/PersonaRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected:
            missing.append(f"{method.upper()} {path} request body must reference {expected}")
    read_routes = {
        ("/api/v1/conversations", "get"): ("#/components/schemas/Conversation", "array"),
        ("/api/v1/conversations/{conversationId}", "get"): ("#/components/schemas/Conversation", "ref"),
        ("/api/v1/conversations/{conversationId}/messages", "get"): ("#/components/schemas/Message", "array"),
        ("/api/v1/app/models", "get"): ("#/components/schemas/Model", "array"),
        ("/api/v1/app/conversations", "get"): ("#/components/schemas/Conversation", "array"),
        ("/api/v1/app/conversations/{conversationId}/messages", "get"): ("#/components/schemas/Message", "array"),
        ("/api/v1/app/personas", "get"): ("#/components/schemas/Persona", "array"),
        ("/api/v1/app/conversations/{conversationId}", "get"): ("#/components/schemas/Conversation", "ref"),
        ("/api/v1/app/personas/{personaId}", "get"): ("#/components/schemas/Persona", "ref"),
    }
    for (path, method), (expected, shape) in read_routes.items():
        op = operation(paths, path, method, missing)
        if not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
        if not has_tags(op, "Chat"):
            missing.append(f"{method.upper()} {path} must be tagged Chat")
        actual = response_data_array_ref(op, "200") if shape == "array" else response_data_ref(op, "200")
        if actual != expected:
            noun = "data array" if path == "/api/v1/app/personas" else "data"
            missing.append(f"{method.upper()} {path} 200 {noun} must reference {expected}")
    for (path, method), expected in {
        ("/api/v1/app/message-shares/{shareId}", "get"): "#/components/schemas/MessageShareDetailResponse",
        ("/api/v1/app/conversation-shares/{shareId}", "get"): "#/components/schemas/ConversationShareDetailResponse",
    }.items():
        op = operation(paths, path, method, missing)
        if op.get("security") != []:
            missing.append(f"{method.upper()} {path} must declare security: []")
        if not has_tags(op, "Chat"):
            missing.append(f"{method.upper()} {path} must be tagged Chat")
        if response_data_ref(op, "200") != expected:
            missing.append(f"{method.upper()} {path} 200 data must reference {expected}")
    stream = operation(paths, "/api/v1/app/conversations/{conversationId}/messages/stream", "post", missing)
    if not requires_cookie_and_csrf(stream):
        missing.append("POST /api/v1/app/conversations/{conversationId}/messages/stream must require cookieAuth and csrfHeader")
    if not has_tags(stream, "Chat"):
        missing.append("POST /api/v1/app/conversations/{conversationId}/messages/stream must be tagged Chat")
    if dig(stream, "responses", "200", "content", "text/event-stream", "schema", "type") != "string":
        missing.append("POST /api/v1/app/conversations/{conversationId}/messages/stream 200 response must document text/event-stream")
    export = operation(paths, "/api/v1/app/conversations/{conversationId}/export.md", "get", missing)
    if not requires_cookie_without_csrf(export):
        missing.append("GET /api/v1/app/conversations/{conversationId}/export.md must require cookieAuth without csrfHeader")
    if not has_tags(export, "Chat"):
        missing.append("GET /api/v1/app/conversations/{conversationId}/export.md must be tagged Chat")
    if dig(export, "responses", "200", "content", "text/markdown", "schema", "type") != "string":
        missing.append("GET /api/v1/app/conversations/{conversationId}/export.md 200 response must document text/markdown")
    config = operation(paths, "/api/v1/app/conversations/{conversationId}/config", "get", missing)
    if not requires_cookie_without_csrf(config):
        missing.append("GET /api/v1/app/conversations/{conversationId}/config must require cookieAuth without csrfHeader")
    if not has_tags(config, "Chat"):
        missing.append("GET /api/v1/app/conversations/{conversationId}/config must be tagged Chat")
    if response_data_ref(config, "200") != "#/components/schemas/ConversationConfig":
        missing.append("GET /api/v1/app/conversations/{conversationId}/config 200 data must reference ConversationConfig")
    if dig(schemas, "UpdateConversationConfigRequest", "properties", "personaId", "type") != "string":
        missing.append("UpdateConversationConfigRequest must document personaId")
    for schema_name in ["ConversationConfig", "TaskDraft"]:
        if schema_name not in schemas:
            missing.append(f"{schema_name} schema must be documented")
    if dig(schemas, "UpdateConversationRequest", "properties", "title", "type") != "string":
        missing.append("UpdateConversationRequest.title must be documented as string")
    if "deleted" not in enum_values(schemas.get("ConversationDeleteResponse", {}), "properties", "status"):
        missing.append("ConversationDeleteResponse.status must document deleted")
    fork = schemas.get("ForkConversationRequest", {})
    if not (
        "branchFromMessageId" in required(fork)
        and dig(fork, "properties", "branchFromMessageId", "type") == "string"
        and dig(fork, "properties", "messageId", "deprecated") is True
        and dig(fork, "properties", "sourceConversationId", "type") == "string"
    ):
        missing.append("ForkConversationRequest must require branchFromMessageId, document legacy messageId, and allow sourceConversationId")
    if dig(schemas, "Message", "properties", "bookmarked", "type") != "boolean":
        missing.append("Message.bookmarked must be documented as boolean")
    if not ("content" in required(schemas.get("UpdateMessageRequest", {})) and dig(schemas, "UpdateMessageRequest", "properties", "content", "type") == "string"):
        missing.append("UpdateMessageRequest.content must be required and documented as string")
    if dig(schemas, "BookmarkMessageRequest", "properties", "bookmarked", "type") != "boolean":
        missing.append("BookmarkMessageRequest.bookmarked must be documented as boolean")
    if "deleted" not in enum_values(schemas.get("MessageDeleteResponse", {}), "properties", "status"):
        missing.append("MessageDeleteResponse.status must document deleted")
    persona_props = props(schemas.get("Persona", {}))
    for field in ["id", "workspaceId", "name", "role", "style", "tone", "constraints", "openingMessage"]:
        if dig(persona_props, field, "type") != "string":
            missing.append(f"Persona.{field} must be documented as string")
    if dig(persona_props, "createdAt", "format") != "date-time":
        missing.append("Persona.createdAt must be documented as date-time")
    if dig(persona_props, "suggestedQuestions", "items", "type") != "string":
        missing.append("Persona.suggestedQuestions must be documented as string[]")
    if not ("name" in required(schemas.get("PersonaRequest", {})) and dig(schemas, "PersonaRequest", "properties", "name", "type") == "string"):
        missing.append("PersonaRequest.name must be required and documented as string")
    if dig(schemas, "PersonaRequest", "properties", "suggestedQuestions", "items", "type") != "string":
        missing.append("PersonaRequest.suggestedQuestions must be documented as string[]")
    if dig(schemas, "PersonaDeleteResponse", "properties", "status", "type") != "string":
        missing.append("PersonaDeleteResponse.status must be documented as string")
    if dig(schemas, "CreateMessageShareRequest", "properties", "expiresAt", "format") != "date-time":
        missing.append("CreateMessageShareRequest.expiresAt must be documented as date-time")
    if not (
        dig(schemas, "CreateConversationShareRequest", "properties", "startMessageId", "type") == "string"
        and dig(schemas, "CreateConversationShareRequest", "properties", "endMessageId", "type") == "string"
        and dig(schemas, "CreateConversationShareRequest", "properties", "expiresAt", "format") == "date-time"
    ):
        missing.append("CreateConversationShareRequest must document range fields and expiresAt")
    if dig(schemas, "MessageShareResponse", "properties", "url", "type") != "string" or dig(schemas, "ConversationShareResponse", "properties", "url", "type") != "string":
        missing.append("share response schemas must document url")
    if not (
        any(dig(entry, "properties", "message", "$ref") == "#/components/schemas/Message" for entry in schemas.get("MessageShareDetailResponse", {}).get("allOf", []))
        and any(dig(entry, "properties", "messages", "items", "$ref") == "#/components/schemas/Message" for entry in schemas.get("ConversationShareDetailResponse", {}).get("allOf", []))
    ):
        missing.append("share detail schemas must document message payloads")
    fail("[openapi-contract] Chat mutation CSRF contract is incomplete:", missing)


def require_knowledge_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    mutation_paths = [
        ("/api/v1/app/knowledge-bases", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}", "put"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}", "delete"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "put"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "delete"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "put"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve/debug", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "post"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "post"),
    ]
    check_mutation_security(paths, mutation_paths, missing, "Knowledge")
    check_read_security(paths, [
        ("/api/v1/app/knowledge-bases", "get"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}", "get"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "get"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions", "get"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks", "get"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "get"),
    ], missing, "Knowledge")
    for (path, method, content_type), expected in {
        ("/api/v1/app/knowledge-bases", "post", "application/json"): "#/components/schemas/CreateKnowledgeBaseRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}", "put", "application/json"): "#/components/schemas/CreateKnowledgeBaseRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "post", "application/json"): "#/components/schemas/CreateDocumentRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "post", "multipart/form-data"): "#/components/schemas/UploadKnowledgeDocumentRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "put", "application/json"): "#/components/schemas/CreateDocumentRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "put", "application/json"): "#/components/schemas/UpdateKnowledgeDocumentChunkRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "post", "application/json"): "#/components/schemas/SplitKnowledgeDocumentChunkRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "post", "application/json"): "#/components/schemas/MergeKnowledgeDocumentChunksRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "post", "application/json"): "#/components/schemas/RetrieveKnowledgeRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve/debug", "post", "application/json"): "#/components/schemas/RetrieveKnowledgeRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "post", "application/json"): "#/components/schemas/CreateKnowledgeRetrievalTestCaseRequest",
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "post", "application/json"): "#/components/schemas/KnowledgeRetrievalTestRunRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing), content_type) != expected:
            missing.append(f"{method.upper()} {path} {content_type} request body must reference {expected}")
    response_expectations = {
        ("/api/v1/app/knowledge-bases", "get", "200"): ("#/components/schemas/KnowledgeBase", "array"),
        ("/api/v1/app/knowledge-bases", "post", "200"): ("#/components/schemas/KnowledgeBase", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}", "get", "200"): ("#/components/schemas/KnowledgeBase", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}", "put", "200"): ("#/components/schemas/KnowledgeBase", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "get", "200"): ("#/components/schemas/Document", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "post", "200"): ("#/components/schemas/Document", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "post", "200"): ("#/components/schemas/Document", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "put", "200"): ("#/components/schemas/Document", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions", "get", "200"): ("#/components/schemas/KnowledgeDocumentVersion", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks", "get", "200"): ("#/components/schemas/KnowledgeDocumentChunk", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "put", "200"): ("#/components/schemas/KnowledgeDocumentChunk", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "post", "200"): ("#/components/schemas/KnowledgeDocumentChunk", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "post", "200"): ("#/components/schemas/KnowledgeDocumentChunk", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "post", "200"): ("#/components/schemas/KnowledgeRetrievalResult", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve/debug", "post", "200"): ("#/components/schemas/KnowledgeRetrievalDebugReport", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "get", "200"): ("#/components/schemas/KnowledgeRetrievalTestCase", "array"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "post", "201"): ("#/components/schemas/KnowledgeRetrievalTestCase", "ref"),
        ("/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "post", "200"): ("#/components/schemas/KnowledgeRetrievalTestRunReport", "ref"),
    }
    for (path, method, status), (expected, shape) in response_expectations.items():
        op = operation(paths, path, method, missing)
        actual = response_data_array_ref(op, status) if shape == "array" else response_data_ref(op, status)
        if actual != expected:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected}")
    aliases = {
        "/api/v1/knowledge-bases": "#/paths/~1api~1v1~1app~1knowledge-bases",
        "/api/v1/knowledge-bases/{knowledgeBaseId}": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/upload": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1upload",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1versions",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks~1{chunkId}",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks~1{chunkId}~1split",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks~1{chunkId}~1merge",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieve": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieve",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieve/debug": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieve~1debug",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieval-test-cases",
        "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run": "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieval-test-cases~1run",
    }
    for alias_path, expected_ref in aliases.items():
        if dig(paths, alias_path, "$ref") != expected_ref:
            missing.append(f"{alias_path} must reference {expected_ref}")
    root_delete = operation(paths, "/api/v1/documents/{documentId}", "delete", missing)
    if not requires_cookie_and_csrf(root_delete):
        missing.append("DELETE /api/v1/documents/{documentId} must require cookieAuth and csrfHeader")
    if not has_tags(root_delete, "Knowledge"):
        missing.append("DELETE /api/v1/documents/{documentId} must be tagged Knowledge")
    if not any(isinstance(param, dict) and param.get("name") == "documentId" and param.get("in") == "path" and param.get("required") is True for param in root_delete.get("parameters", [])):
        missing.append("DELETE /api/v1/documents/{documentId} must require documentId path parameter")
    if dig(root_delete, "responses", "204", "description") is None:
        missing.append("DELETE /api/v1/documents/{documentId} must document 204 deletion")
    if not (
        "hybrid_rerank" in enum_values(schemas.get("KnowledgeBase", {}), "properties", "retrievalMode")
        and dig(schemas, "CreateKnowledgeBaseRequest", "properties", "chunkSize", "type") == "integer"
        and dig(schemas, "CreateKnowledgeBaseRequest", "properties", "embeddingModel", "type") == "string"
        and dig(schemas, "CreateKnowledgeBaseRequest", "properties", "vectorWeight", "format") == "double"
    ):
        missing.append("KnowledgeBase and CreateKnowledgeBaseRequest must document retrieval/chunking config fields")
    if not (
        dig(schemas, "CreateDocumentRequest", "properties", "documentVersion", "type") == "string"
        and dig(schemas, "CreateDocumentRequest", "properties", "pageNumber", "type") == "integer"
        and dig(schemas, "CreateDocumentRequest", "properties", "sourceUrl", "type") == "string"
        and dig(schemas, "UploadKnowledgeDocumentRequest", "properties", "file", "format") == "binary"
    ):
        missing.append("Knowledge document create/upload schemas must document metadata and multipart file fields")
    if not (
        dig(schemas, "KnowledgeRetrievalResult", "properties", "documentId", "type") == "string"
        and dig(schemas, "KnowledgeRetrievalResult", "properties", "snippet", "type") == "string"
        and "hybrid_rerank" in enum_values(schemas.get("KnowledgeRetrievalResult", {}), "properties", "retrievalMode")
    ):
        missing.append("KnowledgeRetrievalResult schema must document result identity, snippet, and retrieval mode")
    if not (
        dig(schemas, "KnowledgeRetrievalDebugReport", "properties", "knowledgeBaseId", "type") == "string"
        and dig(schemas, "KnowledgeRetrievalDebugReport", "properties", "citationCoverage", "$ref") == "#/components/schemas/KnowledgeRetrievalCitationCoverage"
        and dig(schemas, "KnowledgeRetrievalDebugReport", "properties", "results", "items", "$ref") == "#/components/schemas/KnowledgeRetrievalResult"
        and dig(schemas, "KnowledgeRetrievalCitationCoverage", "properties", "resultsWithHighlights", "type") == "integer"
    ):
        missing.append("KnowledgeRetrievalDebugReport schema must document citation coverage and typed retrieval results")
    if not (
        dig(schemas, "CreateKnowledgeRetrievalTestCaseRequest", "properties", "expectedResult", "$ref") == "#/components/schemas/KnowledgeRetrievalResult"
        and dig(schemas, "KnowledgeRetrievalTestCase", "properties", "expectedResult", "$ref") == "#/components/schemas/KnowledgeRetrievalResult"
        and dig(schemas, "KnowledgeRetrievalTestRunReport", "properties", "results", "items", "$ref") == "#/components/schemas/KnowledgeRetrievalTestRunResult"
    ):
        missing.append("Knowledge retrieval test case schemas must reference typed retrieval results")
    fail("[openapi-contract] Knowledge mutation CSRF/schema contract is incomplete:", missing)


def require_admin_organization_mutation_csrf_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected = {
        ("/api/v1/admin/organizations", "get", "200"): "#/components/schemas/AdminOrganizationListResponse",
        ("/api/v1/admin/organizations", "post", "201"): "#/components/schemas/Organization",
        ("/api/v1/admin/organizations/{organizationId}", "get", "200"): "#/components/schemas/Organization",
        ("/api/v1/admin/organizations/{organizationId}", "put", "200"): "#/components/schemas/Organization",
        ("/api/v1/admin/organizations/{organizationId}/archive", "post", "200"): "#/components/schemas/Organization",
        ("/api/v1/admin/organizations/{organizationId}/members", "get", "200"): "#/components/schemas/AdminOrganizationMembersResponse",
    }
    for (path, method, status), expected_ref in expected.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected_ref:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected_ref}")
        if not has_tags(op, "Admin"):
            missing.append(f"{method.upper()} {path} must be tagged Admin")
        if method == "get" and not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
    check_mutation_security(paths, [
        ("/api/v1/admin/organizations", "post"),
        ("/api/v1/admin/organizations/{organizationId}", "put"),
        ("/api/v1/admin/organizations/{organizationId}/archive", "post"),
    ], missing)
    for (path, method), expected_ref in {
        ("/api/v1/admin/organizations", "post"): "#/components/schemas/CreateOrganizationRequest",
        ("/api/v1/admin/organizations/{organizationId}", "put"): "#/components/schemas/UpdateOrganizationRequest",
    }.items():
        op = operation(paths, path, method, missing)
        if dig(op, "requestBody", "required") is not True or request_body_ref(op) != expected_ref:
            missing.append(f"{method.upper()} {path} request body must require {expected_ref}")
    if dig(schemas, "AdminOrganizationListResponse", "properties", "organizations", "items", "$ref") != "#/components/schemas/Organization" or dig(schemas, "AdminOrganizationListResponse", "properties", "total", "type") != "integer":
        missing.append("AdminOrganizationListResponse must expose organizations[] plus integer total")
    if dig(schemas, "AdminOrganizationMembersResponse", "properties", "members", "items", "$ref") != "#/components/schemas/OrganizationMembership":
        missing.append("AdminOrganizationMembersResponse must expose members[]")
    fail("[openapi-contract] Admin organization mutation CSRF contract is incomplete:", missing)


def require_admin_core_management_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/admin/stats", "get", "200"): "#/components/schemas/AdminStats",
        ("/api/v1/admin/settings/relay-pricing", "get", "200"): "#/components/schemas/AdminRelayPricingSettings",
        ("/api/v1/admin/settings/relay-pricing", "put", "200"): "#/components/schemas/AdminRelayPricingSettings",
        ("/api/v1/admin/settings/usage-limits", "get", "200"): "#/components/schemas/AdminUsageLimitSettingsListResponse",
        ("/api/v1/admin/settings/usage-limits", "put", "200"): "#/components/schemas/AdminUsageLimitSettings",
        ("/api/v1/admin/routes", "get", "200"): "#/components/schemas/AdminRouteListResponse",
        ("/api/v1/admin/routes", "post", "201"): "#/components/schemas/AdminRoute",
        ("/api/v1/admin/routes/{routeId}", "get", "200"): "#/components/schemas/AdminRoute",
        ("/api/v1/admin/routes/{routeId}", "put", "200"): "#/components/schemas/AdminRoute",
        ("/api/v1/admin/routes/{routeId}", "delete", "200"): "#/components/schemas/AdminDeleteStatusResponse",
        ("/api/v1/admin/plans", "get", "200"): "#/components/schemas/AdminPlanListResponse",
        ("/api/v1/admin/plans", "post", "201"): "#/components/schemas/AdminPlan",
        ("/api/v1/admin/plans/{planId}", "get", "200"): "#/components/schemas/AdminPlan",
        ("/api/v1/admin/plans/{planId}", "put", "200"): "#/components/schemas/AdminPlan",
        ("/api/v1/admin/plans/{planId}", "delete", "200"): "#/components/schemas/AdminDeactivateStatusResponse",
        ("/api/v1/admin/users", "get", "200"): "#/components/schemas/AdminUserListResponse",
        ("/api/v1/admin/users/{userId}", "get", "200"): "#/components/schemas/AdminUser",
        ("/api/v1/admin/users/{userId}", "put", "200"): "#/components/schemas/AdminUser",
        ("/api/v1/admin/users/{userId}", "patch", "200"): "#/components/schemas/AdminUser",
        ("/api/v1/admin/users/{userId}", "delete", "200"): "#/components/schemas/AdminDeleteStatusResponse",
        ("/api/v1/admin/users/{userId}/disable", "post", "200"): "#/components/schemas/AdminUserStatusResponse",
        ("/api/v1/admin/users/{userId}/enable", "post", "200"): "#/components/schemas/AdminUserStatusResponse",
        ("/api/v1/admin/audit-logs", "get", "200"): "#/components/schemas/AdminAuditLogListResponse",
    }
    for (path, method, status), expected_ref in expected_data_refs.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, status) != expected_ref:
            missing.append(f"{method.upper()} {path} {status} data must reference {expected_ref}")
        if not has_tags(op, "Admin"):
            missing.append(f"{method.upper()} {path} must be tagged Admin")
        if method == "get" and not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
    for path in ["/api/v1/admin/settings/relay-pricing", "/api/v1/admin/settings/usage-limits", "/api/v1/admin/routes", "/api/v1/admin/routes/{routeId}", "/api/v1/admin/plans", "/api/v1/admin/plans/{planId}", "/api/v1/admin/users/{userId}", "/api/v1/admin/users/{userId}/disable", "/api/v1/admin/users/{userId}/enable"]:
        for method in [key for key in (paths.get(path, {}) if isinstance(paths.get(path), dict) else {}).keys() if key in {"post", "put", "patch", "delete"}]:
            if not requires_cookie_and_csrf(operation(paths, path, method, missing)):
                missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
    for (path, method), expected_ref in {
        ("/api/v1/admin/settings/relay-pricing", "put"): "#/components/schemas/AdminRelayPricingSettings",
        ("/api/v1/admin/settings/usage-limits", "put"): "#/components/schemas/AdminUsageLimitSettings",
        ("/api/v1/admin/routes", "post"): "#/components/schemas/AdminRouteCreateRequest",
        ("/api/v1/admin/routes/{routeId}", "put"): "#/components/schemas/AdminRouteUpdateRequest",
        ("/api/v1/admin/plans", "post"): "#/components/schemas/AdminPlanCreateRequest",
        ("/api/v1/admin/plans/{planId}", "put"): "#/components/schemas/AdminPlanUpdateRequest",
        ("/api/v1/admin/users/{userId}", "put"): "#/components/schemas/AdminUserUpdateRequest",
        ("/api/v1/admin/users/{userId}", "patch"): "#/components/schemas/AdminUserQuotaUpdateRequest",
    }.items():
        if request_body_ref(operation(paths, path, method, missing)) != expected_ref:
            missing.append(f"{method.upper()} {path} request body must reference {expected_ref}")
    for schema_name, fields in {
        "AdminStats": ["users", "quotas", "channelsTotal", "apiCalls24h"],
        "AdminRelayPricingSettings": ["modelMultipliers", "groupMultipliers"],
        "AdminUsageLimitSettings": ["organizationId", "userId", "quotaMode", "maxConcurrentRequests", "windowSeconds", "maxTokensPerWindow", "maxTokensPerRequest"],
        "AdminRoute": ["id", "model", "strategy", "channels", "createdAt"],
        "AdminPlan": ["id", "name", "quotaAmount", "tokenQuota", "maxTokensPerRequest", "isActive"],
        "AdminUser": ["id", "email", "role", "status", "createdAt"],
        "AdminAuditLogEntry": ["id", "actorID", "actorEmail", "action", "resourceType", "createdAt"],
    }.items():
        schema_props = props(schemas.get(schema_name, {}))
        for field in fields:
            if field not in schema_props:
                missing.append(f"{schema_name}.{field} must be documented")
    if dig(schemas, "AdminRelayPricingSettings", "properties", "modelMultipliers", "additionalProperties", "format") != "double" or dig(schemas, "AdminRelayPricingSettings", "properties", "groupMultipliers", "additionalProperties", "format") != "double":
        missing.append("AdminRelayPricingSettings must document model/group multiplier maps")
    if not (
        "user" in enum_values(schemas.get("AdminUsageLimitSettings", {}), "properties", "quotaMode")
        and dig(schemas, "AdminUsageLimitSettings", "properties", "maxTokensPerRequest", "type") == "integer"
        and dig(schemas, "AdminUsageLimitSettingsListResponse", "properties", "usageLimits", "items", "$ref") == "#/components/schemas/AdminUsageLimitSettings"
    ):
        missing.append("AdminUsageLimitSettings schemas must document scoped usage limits and request-token cap")
    if "balance" not in required(schemas.get("AdminUserQuotaUpdateRequest", {})) or dig(schemas, "AdminUserQuotaUpdateRequest", "properties", "balance", "minimum") != 0:
        missing.append("AdminUserQuotaUpdateRequest.balance must be required and non-negative")
    changes = dig(schemas, "AdminAuditLogEntry", "properties", "changes") or {}
    changes_description = changes.get("description", "")
    if not (
        changes.get("type") == "string"
        and all(word in changes_description.lower() for word in ["redacted", "credential"])
        and "apiKey" in changes_description
    ):
        missing.append("AdminAuditLogEntry.changes must document redacted credential fields including apiKey")
    for schema_name, (collection, item_ref) in {
        "AdminRouteListResponse": ("routes", "#/components/schemas/AdminRoute"),
        "AdminPlanListResponse": ("plans", "#/components/schemas/AdminPlan"),
        "AdminUserListResponse": ("users", "#/components/schemas/AdminUser"),
        "AdminAuditLogListResponse": ("entries", "#/components/schemas/AdminAuditLogEntry"),
    }.items():
        if dig(schemas, schema_name, "properties", collection, "items", "$ref") != item_ref or dig(schemas, schema_name, "properties", "total", "type") != "integer":
            missing.append(f"{schema_name} must expose {collection}[] plus integer total")
    fail("[openapi-contract] Admin core management contract is incomplete:", missing)


def require_admin_billing_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    expected_data_refs = {
        ("/api/v1/admin/billing/summary", "get"): "#/components/schemas/AdminBillingInspectionSummary",
        ("/api/v1/admin/billing/sessions", "get"): "#/components/schemas/AdminBillingSessionsResponse",
        ("/api/v1/admin/billing/payment-intents", "get"): "#/components/schemas/AdminPaymentIntentsResponse",
        ("/api/v1/admin/billing/webhook-events", "get"): "#/components/schemas/AdminWebhookEventsResponse",
        ("/api/v1/admin/billing/subscriptions", "get"): "#/components/schemas/AdminSubscriptionsResponse",
        ("/api/v1/admin/billing/topups", "get"): "#/components/schemas/AdminTopupsResponse",
        ("/api/v1/admin/billing/invoices", "get"): "#/components/schemas/AdminInvoicesResponse",
        ("/api/v1/admin/billing/refunds", "get"): "#/components/schemas/AdminRefundsResponse",
        ("/api/v1/admin/billing/settlements", "get"): "#/components/schemas/AdminMarketplaceSettlementsResponse",
        ("/api/v1/admin/billing/payouts", "get"): "#/components/schemas/AdminMarketplacePayoutsResponse",
        ("/api/v1/admin/billing/topups/{topupId}/refund", "post"): "#/components/schemas/AdminRefundInspection",
        ("/api/v1/admin/billing/payouts/create-due", "post"): "#/components/schemas/AdminMarketplacePayoutsResponse",
        ("/api/v1/admin/billing/payouts/{payoutId}/paid", "post"): "#/components/schemas/MarketplacePayout",
        ("/api/v1/admin/billing/payouts/{payoutId}/failed", "post"): "#/components/schemas/MarketplacePayout",
    }
    for (path, method), expected_ref in expected_data_refs.items():
        op = operation(paths, path, method, missing)
        if response_data_ref(op, "200") != expected_ref:
            missing.append(f"{method.upper()} {path} 200 data must reference {expected_ref}")
        if not has_tags(op, "Admin", "Billing"):
            missing.append(f"{method.upper()} {path} must be tagged Admin and Billing")
        if method == "get" and not requires_cookie_without_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth without csrfHeader")
    list_paths = [path for (path, method) in expected_data_refs if method == "get" and path != "/api/v1/admin/billing/summary"]
    for path in list_paths:
        names = [param.get("name") for param in dig(paths, path, "get", "parameters") or [] if isinstance(param, dict)]
        for name in ["organizationID", "organizationId", "userID", "userId", "status", "kind", "provider", "limit", "offset"]:
            if name not in names:
                missing.append(f"GET {path} must document {name} query filter")
    summary_names = [param.get("name") for param in dig(paths, "/api/v1/admin/billing/summary", "get", "parameters") or [] if isinstance(param, dict)]
    for name in ["organizationID", "organizationId", "userID", "userId", "status", "kind", "provider"]:
        if name not in summary_names:
            missing.append(f"GET /api/v1/admin/billing/summary must document {name} query filter")
    refund = operation(paths, "/api/v1/admin/billing/topups/{topupId}/refund", "post", missing)
    if request_body_ref(refund) != "#/components/schemas/AdminTopupRefundRequest":
        missing.append("POST /api/v1/admin/billing/topups/{topupId}/refund must document AdminTopupRefundRequest body")
    if not requires_cookie_and_csrf(refund):
        missing.append("POST /api/v1/admin/billing/topups/{topupId}/refund must require cookieAuth and csrfHeader")
    refund_schema = schemas.get("AdminTopupRefundRequest", {})
    check_required_fields(schemas, "AdminTopupRefundRequest", ["provider", "providerRefundID", "amount", "currency"], missing)
    alternatives = [entry.get("required") for entry in refund_schema.get("anyOf", []) if isinstance(entry, dict)]
    if not any("providerChargeID" in alt for alt in alternatives if isinstance(alt, list)) or not any("providerPaymentIntentID" in alt for alt in alternatives if isinstance(alt, list)):
        missing.append("AdminTopupRefundRequest must document providerChargeID or providerPaymentIntentID evidence alternatives")
    if not (float(dig(refund_schema, "properties", "amount", "minimum") or 0) > 0):
        missing.append("AdminTopupRefundRequest.amount must document a positive minimum")
    for path, method, body_ref, required_fields in [
        ("/api/v1/admin/billing/payouts/create-due", "post", None, []),
        ("/api/v1/admin/billing/payouts/{payoutId}/paid", "post", "#/components/schemas/AdminMarketplacePayoutPaidRequest", ["providerPayoutID"]),
        ("/api/v1/admin/billing/payouts/{payoutId}/failed", "post", "#/components/schemas/AdminMarketplacePayoutFailedRequest", ["providerPayoutID", "reason"]),
    ]:
        op = operation(paths, path, method, missing)
        if not requires_cookie_and_csrf(op):
            missing.append(f"{method.upper()} {path} must require cookieAuth and csrfHeader")
        if body_ref and request_body_ref(op) != body_ref:
            missing.append(f"{method.upper()} {path} must document {ref_name(body_ref)} body")
        if body_ref:
            check_required_fields(schemas, ref_name(body_ref), required_fields, missing)
    response_collections = {
        "AdminBillingSessionsResponse": ("sessions", "#/components/schemas/AdminBillingSessionInspection"),
        "AdminPaymentIntentsResponse": ("paymentIntents", "#/components/schemas/AdminPaymentIntentInspection"),
        "AdminWebhookEventsResponse": ("webhookEvents", "#/components/schemas/AdminWebhookEventInspection"),
        "AdminSubscriptionsResponse": ("subscriptions", "#/components/schemas/AdminSubscriptionInspection"),
        "AdminTopupsResponse": ("topups", "#/components/schemas/AdminTopupInspection"),
        "AdminInvoicesResponse": ("invoices", "#/components/schemas/AdminInvoiceInspection"),
        "AdminRefundsResponse": ("refunds", "#/components/schemas/AdminRefundInspection"),
        "AdminMarketplaceSettlementsResponse": ("settlements", "#/components/schemas/AdminMarketplaceSettlementInspection"),
        "AdminMarketplacePayoutsResponse": ("payouts", "#/components/schemas/AdminMarketplacePayoutInspection"),
    }
    for schema_name, (collection, item_ref) in response_collections.items():
        if not (
            dig(schemas, schema_name, "properties", collection, "type") == "array"
            and dig(schemas, schema_name, "properties", collection, "items", "$ref") == item_ref
            and dig(schemas, schema_name, "properties", "total", "type") == "integer"
        ):
            missing.append(f"{schema_name} must expose {collection}[] as {item_ref} plus integer total")
    usage_logs_op = operation(paths, "/api/v1/admin/usage-logs", "get", missing)
    if response_data_ref(usage_logs_op, "200") != "#/components/schemas/AdminUsageLogListResponse":
        missing.append("GET /api/v1/admin/usage-logs 200 data must reference AdminUsageLogListResponse")
    if not has_tags(usage_logs_op, "Admin", "Billing"):
        missing.append("GET /api/v1/admin/usage-logs must be tagged Admin and Billing")
    if not requires_cookie_without_csrf(usage_logs_op):
        missing.append("GET /api/v1/admin/usage-logs must require cookieAuth without csrfHeader")
    if not (
        dig(schemas, "AdminUsageLogListResponse", "properties", "usageLogs", "type") == "array"
        and dig(schemas, "AdminUsageLogListResponse", "properties", "usageLogs", "items", "$ref") == "#/components/schemas/AdminUsageLogEntry"
        and dig(schemas, "AdminUsageLogListResponse", "properties", "total", "type") == "integer"
    ):
        missing.append("AdminUsageLogListResponse must expose usageLogs[] as AdminUsageLogEntry plus integer total")
    if dig(schemas, "AdminUsageLogEntry", "properties", "requestLogEvidence", "$ref") != "#/components/schemas/AdminRequestLogEvidence":
        missing.append("AdminUsageLogEntry.requestLogEvidence must reference AdminRequestLogEvidence")
    check_required_fields(schemas, "AdminRequestLogEvidence", ["requestId", "requestLogId", "timestamp", "service", "endpoint", "method", "statusCode", "durationMs", "requestTokens", "responseTokens", "model", "costUsd"], missing)
    for field in ["requestId", "requestLogId", "service", "endpoint", "method", "model", "error", "traceId"]:
        if dig(schemas, "AdminRequestLogEvidence", "properties", field, "type") != "string":
            missing.append(f"AdminRequestLogEvidence.{field} must be documented as string")
    for field in ["statusCode", "durationMs", "requestTokens", "responseTokens"]:
        if dig(schemas, "AdminRequestLogEvidence", "properties", field, "type") != "integer":
            missing.append(f"AdminRequestLogEvidence.{field} must be documented as integer")
    if dig(schemas, "AdminRequestLogEvidence", "properties", "costUsd", "type") != "number":
        missing.append("AdminRequestLogEvidence.costUsd must be documented as number")
    if dig(schemas, "AdminRequestLogEvidence", "properties", "timestamp", "format") != "date-time":
        missing.append("AdminRequestLogEvidence.timestamp must be documented as date-time")
    if dig(schemas, "AdminRequestLogEvidence", "properties", "metadata", "type") != "object":
        missing.append("AdminRequestLogEvidence.metadata must be documented as object")
    check_required_fields(schemas, "AdminMarketplacePayoutsResponse", ["payouts", "total"], missing)
    check_required_fields(schemas, "AdminMarketplaceSettlementsResponse", ["settlements", "total"], missing)
    check_required_fields(schemas, "AdminMarketplacePayoutInspection", ["id", "publisherOrganizationId", "publisherUserId", "amount", "currency", "provider", "status", "createdAt", "updatedAt"], missing)
    check_required_fields(schemas, "AdminMarketplaceSettlementInspection", ["id", "orderId", "publisherOrganizationId", "publisherUserId", "agentId", "grossAmount", "platformFeeAmount", "publisherNetAmount", "refundedAmount", "status", "createdAt", "updatedAt"], missing)
    webhook_props = props(schemas.get("AdminWebhookEventInspection", {}))
    for field in ["payload", "rawPayload", "providerPayload"]:
        if field in webhook_props:
            missing.append("AdminWebhookEventInspection must not document raw provider payload fields")
    check_property_type(schemas, "AdminTopupInspection", ["provider", "providerPaymentIntentId", "currency"], "string", missing, "as string provider refund evidence")
    for field in ["billingSessions", "paymentIntents", "webhookEvents", "subscriptions", "topups", "invoices", "refunds", "settlements", "payouts"]:
        if dig(schemas, "AdminBillingInspectionSummary", "properties", field, "$ref") != "#/components/schemas/AdminBillingAmountSummary":
            missing.append(f"AdminBillingInspectionSummary.{field} must reference AdminBillingAmountSummary")
    if "MarketplacePayout" not in schemas:
        missing.append("MarketplacePayout schema must document the payout paid runtime response")
    fail("[openapi-contract] Admin Billing route/schema contract is incomplete:", missing)


def require_domestic_payment_webhook_payout_contract(paths: dict[str, Any], schemas: dict[str, Any]) -> None:
    missing: list[str] = []
    for path in ["/api/v1/billing/alipay/webhook", "/api/v1/billing/wechatpay/webhook"]:
        post = operation(paths, path, "post", missing)
        if post.get("security") != []:
            missing.append(f"POST {path} must declare security: []")
        headers = {param.get("name"): param for param in post.get("parameters", []) if isinstance(param, dict) and param.get("in") == "header"}
        for header in ["Oblivious-Payment-Timestamp", "Oblivious-Payment-Signature"]:
            param = headers.get(header)
            if not (isinstance(param, dict) and param.get("required") is True and dig(param, "schema", "type") == "string"):
                missing.append(f"POST {path} must document required {header} string header")
        if dig(post, "requestBody", "required") is not True:
            missing.append(f"POST {path} must require a request body")
        if request_body_ref(post) != "#/components/schemas/DomesticPaymentWebhookEvent":
            missing.append(f"POST {path} request body must reference DomesticPaymentWebhookEvent")
        if response_data_ref(post, "200") != "#/components/schemas/WebhookReceivedResponse":
            missing.append(f"POST {path} 200 data must reference WebhookReceivedResponse")
    event = schemas.get("DomesticPaymentWebhookEvent", {})
    for value in ["payout.paid", "payout.failed"]:
        if value not in enum_values(event, "properties", "type"):
            missing.append(f"DomesticPaymentWebhookEvent.type must enumerate {value}")
    for field in ["payout_id", "provider_payout_id", "status", "reason"]:
        if dig(event, "properties", field, "type") != "string":
            missing.append(f"DomesticPaymentWebhookEvent.{field} must be documented as a string")
    for field in ["id", "type"]:
        if field not in required(event):
            missing.append(f"DomesticPaymentWebhookEvent must require {field}")
    if dig(schemas, "WebhookReceivedResponse", "properties", "received", "type") != "boolean":
        missing.append("WebhookReceivedResponse.received must be documented as a boolean")
    fail("[openapi-contract] Domestic payment webhook payout contract is incomplete:", missing)


def require_relay_alias_bearer_contract(spec: dict[str, Any], paths: list[str]) -> None:
    missing: list[str] = []
    bearer = dig(spec, "components", "securitySchemes", "bearerAuth")
    if not (isinstance(bearer, dict) and bearer.get("type") == "http" and bearer.get("scheme") == "bearer"):
        missing.append("components.securitySchemes.bearerAuth must document Relay bearer tokens")
    for path in paths:
        operations = dig(spec, "paths", path) or {}
        for method, op in operations.items():
            if not isinstance(op, dict):
                continue
            if not has_tags(op, "Relay"):
                missing.append(f"{method.upper()} {path} must use the Relay tag")
            if not requires_bearer(op, spec):
                missing.append(f"{method.upper()} {path} must require bearerAuth")
    fail("[openapi-contract] Relay alias bearer contract is incomplete:", missing)


def require_websocket_contract(spec: dict[str, Any]) -> None:
    missing: list[str] = []
    op = dig(spec, "paths", "/api/v1/ws", "get") or {}
    if not op:
        missing.append("GET /api/v1/ws must be documented")
    if not has_tags(op, "Realtime"):
        missing.append("GET /api/v1/ws must be tagged Realtime")
    if not requires_cookie_without_csrf(op):
        missing.append("GET /api/v1/ws must require cookieAuth without csrfHeader")
    if dig(op, "responses", "101") is None:
        missing.append("GET /api/v1/ws must document 101 WebSocket upgrade")
    if dig(op, "responses", "401", "$ref") != "#/components/responses/Unauthorized":
        missing.append("GET /api/v1/ws 401 must reference Unauthorized")
    if dig(op, "responses", "405", "$ref") != "#/components/responses/MethodNotAllowed":
        missing.append("GET /api/v1/ws 405 must reference MethodNotAllowed")
    if dig(op, "x-websocket-client-message", "$ref") != "#/components/schemas/ChatRealtimeClientMessage":
        missing.append("GET /api/v1/ws must document ChatRealtimeClientMessage as the client frame")
    if dig(op, "x-websocket-server-message", "$ref") != "#/components/schemas/ChatRealtimeEvent":
        missing.append("GET /api/v1/ws must document ChatRealtimeEvent as the server frame")
    schemas = dig(spec, "components", "schemas") or {}
    for schema_name in [
        "ChatRealtimeClientMessage",
        "ChatRealtimeEvent",
        "ChatMessagesSyncedPayload",
        "ChatMessageUpdatedPayload",
        "ChatMessageDeletedPayload",
        "ChatTypingPayload",
    ]:
        if schema_name not in schemas:
            missing.append(f"components.schemas.{schema_name} must be documented for /api/v1/ws chat realtime")
    fail("[openapi-contract] WebSocket contract is incomplete:", missing)


RELAY_ALIAS_PATHS = [
    "/api/v1/relay/chat/completions",
    "/api/v1/relay/embeddings",
    "/api/v1/relay/responses",
    "/api/v1/relay/images/generations",
    "/api/v1/relay/images/edits",
    "/api/v1/relay/images/variations",
    "/api/v1/relay/audio/speech",
    "/api/v1/relay/audio/transcriptions",
    "/api/v1/relay/audio/translations",
    "/api/v1/relay/models",
]

REQUIRED_PATHS = RELAY_ALIAS_PATHS + [
    "/api/v1/agent/tools",
    "/api/v1/agent/runs",
    "/api/v1/agent/runs/{runId}",
    "/api/v1/agent/runs/{runId}/approve-tool",
    "/api/v1/agent/runs/{runId}/reject-tool",
    "/api/v1/agent/runs/{runId}/retry-tool",
    "/api/v1/agent/runs/{runId}/continue-budget",
    "/api/v1/agent/runs/{runId}/continue-plan",
    "/api/v1/agent/runs/{runId}/approve-plan-step",
    "/api/v1/agent/runs/{runId}/skip-plan-step",
    "/api/v1/agent/runs/{runId}/retry-plan-step",
    "/api/v1/agent/runs/{runId}/update-plan-step",
    "/api/v1/agent/runs/{runId}/create-plan-step",
    "/api/v1/agent/runs/{runId}/move-plan-step",
    "/api/v1/agent/runs/{runId}/delete-plan-step",
    "/api/v1/agent/runs/{runId}/execute-plan-step",
    "/api/v1/channels",
    "/api/v1/channels/{channelId}",
    "/api/v1/channels/{channelId}/status",
    "/api/v1/channels/{channelId}/test",
    "/api/v1/channels/{channelId}/send",
    "/api/v1/channels/{channelId}/messages",
    "/api/v1/channels/{channelId}/failed-messages",
    "/api/v1/channels/{channelId}/retry-failed-messages",
    "/api/v1/channels/webhook/{channelId}",
    "/api/v1/workflows",
    "/api/v1/workflows/semantic-matches",
    "/api/v1/workflows/conversation-matches",
    "/api/v1/workflows/webhooks/{organizationId}/{workflowId}",
    "/api/v1/workflows/{workflowId}",
    "/api/v1/workflows/{workflowId}/execute",
    "/api/v1/workflows/{workflowId}/webhook",
    "/api/v1/workflows/{workflowId}/versions",
    "/api/v1/workflows/{workflowId}/branches",
    "/api/v1/workflows/{workflowId}/branches/{branchId}/publish",
    "/api/v1/workflows/{workflowId}/branches/{branchId}/merge",
    "/api/v1/workflows/{workflowId}/rollback",
    "/api/v1/workflows/{workflowId}/test-node",
    "/api/v1/workflows/{workflowId}/executions",
    "/api/v1/workflows/{workflowId}/executions/{executionId}",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/debug-snapshot",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/state-replay",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/decision",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/pause",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/resume",
    "/api/v1/workflows/{workflowId}/executions/{executionId}/cancel",
    "/api/v1/admin/billing/summary",
    "/api/v1/admin/billing/sessions",
    "/api/v1/admin/billing/payment-intents",
    "/api/v1/admin/billing/webhook-events",
    "/api/v1/admin/billing/subscriptions",
    "/api/v1/admin/billing/topups",
    "/api/v1/admin/billing/invoices",
    "/api/v1/admin/billing/refunds",
    "/api/v1/admin/billing/settlements",
    "/api/v1/admin/billing/payouts",
    "/api/v1/admin/billing/topups/{topupId}/refund",
    "/api/v1/admin/billing/payouts/create-due",
    "/api/v1/admin/billing/payouts/{payoutId}/paid",
    "/api/v1/admin/billing/payouts/{payoutId}/failed",
    "/api/v1/admin/stats",
    "/api/v1/admin/api-tokens",
    "/api/v1/admin/api-tokens/{tokenId}/revoke",
    "/api/v1/admin/routes",
    "/api/v1/admin/routes/{routeId}",
    "/api/v1/admin/plans",
    "/api/v1/admin/plans/{planId}",
    "/api/v1/admin/users",
    "/api/v1/admin/users/{userId}",
    "/api/v1/admin/users/{userId}/disable",
    "/api/v1/admin/users/{userId}/enable",
    "/api/v1/admin/audit-logs",
    "/api/v1/admin/usage-logs",
    "/api/v1/admin/usage-analytics",
    "/api/v1/app/agents",
    "/api/v1/app/agents/{agentId}",
    "/api/v1/app/agents/{agentId}/tools",
    "/api/v1/app/agents/{agentId}/conversations",
    "/api/v1/app/agents/conversations/{conversationId}",
    "/api/v1/app/agents/conversations/{conversationId}/messages",
    "/api/v1/app/agents/conversations/{conversationId}/runs",
    "/api/v1/app/agents/runs/{runId}",
    "/api/v1/app/agents/tool-runs/{toolRunId}/approve",
    "/api/v1/app/agents/tool-runs/{toolRunId}/reject",
    "/api/v1/app/agents/tool-runs/{toolRunId}/retry",
    "/api/v1/app/memory/documents",
    "/api/v1/app/memory/documents/{documentId}",
    "/api/v1/app/memory/documents/{documentId}/chunks",
    "/api/v1/app/memory/search",
    "/api/v1/app/mcp-local-servers",
    "/api/v1/app/mcp-servers",
    "/api/v1/app/mcp-servers/{serverId}",
    "/api/v1/app/mcp-servers/{serverId}/connect",
    "/api/v1/app/mcp-servers/{serverId}/disconnect",
    "/api/v1/app/mcp-servers/{serverId}/tools",
    "/api/v1/app/mcp-servers/{serverId}/status",
    "/api/v1/app/mcp-servers/{serverId}/execute",
    "/api/v1/app/organizations",
    "/api/v1/app/organizations/{organizationId}/select",
    "/api/v1/app/organizations/{organizationId}/members",
    "/api/v1/app/organizations/{organizationId}/members/{userId}",
    "/api/v1/app/organizations/{organizationId}/invitations",
    "/api/v1/app/organizations/{organizationId}/invitations/{invitationId}/revoke",
    "/api/v1/app/organizations/{organizationId}/ownership-transfer",
    "/api/v1/app/organization-invitations/{token}/accept",
    "/api/v1/app/knowledge-bases",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases",
    "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run",
    "/api/v1/knowledge-bases",
    "/api/v1/knowledge-bases/{knowledgeBaseId}",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/upload",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieve",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases",
    "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run",
    "/api/v1/documents/{documentId}",
    "/api/v1/app/notifications",
    "/api/v1/app/notifications/unread-count",
    "/api/v1/app/notifications/mark-all-read",
    "/api/v1/app/notifications/{notificationId}",
    "/api/v1/app/quota",
    "/api/v1/app/packages",
    "/api/v1/app/quota/topup",
    "/api/v1/app/personas",
    "/api/v1/app/personas/{personaId}",
    "/api/v1/ws",
    "/api/v1/app/conversations/{conversationId}/share",
    "/api/v1/app/conversations/{conversationId}/messages/{messageId}/share",
    "/api/v1/app/message-shares/{shareId}",
    "/api/v1/app/conversation-shares/{shareId}",
    "/api/v1/scheduled-tasks",
    "/api/v1/scheduled-tasks/{scheduledTaskId}/runs",
    "/api/v1/scheduled-tasks/{scheduledTaskId}/status",
    "/api/v1/scheduled-tasks/{scheduledTaskId}/run",
    "/api/v1/console/usage",
    "/api/v1/console/access",
    "/api/v1/console/models",
    "/api/v1/console/billing",
    "/api/v1/console/invoices",
    "/api/v1/console/api-tokens",
    "/api/v1/console/api-tokens/{tokenId}",
    "/api/v1/console/api-tokens/{tokenId}/usage",
    "/api/v1/billing/checkout",
    "/api/v1/billing/stripe/webhook",
    "/api/v1/billing/alipay/webhook",
    "/api/v1/billing/wechatpay/webhook",
    "/api/v1/billing/marketplace-payout/webhook",
    "/api/v1/marketplace/featured",
    "/api/v1/marketplace/curated",
    "/api/v1/marketplace/categories",
    "/api/v1/marketplace/search",
    "/api/v1/marketplace/agents",
    "/api/v1/marketplace/agents/{agentId}",
    "/api/v1/marketplace/agents/{agentId}/install",
    "/api/v1/marketplace/agents/{agentId}/reviews",
    "/api/v1/marketplace/agents/{agentId}/appeal",
    "/api/v1/marketplace/agents/{agentId}/abuse-reports",
    "/api/v1/marketplace/agents/{agentId}/versions",
    "/api/v1/marketplace/agents/{agentId}/stats",
    "/api/v1/marketplace/my-agents",
    "/api/v1/marketplace/installs",
    "/api/v1/marketplace/installs/{agentId}",
    "/api/v1/marketplace/publisher/stats",
    "/api/v1/marketplace/publisher/settlement-preferences",
    "/api/v1/marketplace/templates",
    "/api/v1/marketplace/templates/{templateId}",
    "/api/v1/marketplace/templates/{templateId}/install",
    "/api/v1/admin/marketplace/agents/{agentId}/takedown",
    "/api/v1/admin/marketplace/agents/{agentId}/reinstate",
    "/api/v1/admin/marketplace/agents/{agentId}/reject-appeal",
    "/api/v1/admin/marketplace/abuse-reports",
    "/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve",
    "/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss",
    "/api/v1/admin/organizations",
    "/api/v1/admin/organizations/{organizationId}",
    "/api/v1/admin/organizations/{organizationId}/archive",
    "/api/v1/admin/organizations/{organizationId}/members",
    "/api/v1/admin/observability/alert-routing",
    "/api/v1/admin/observability/alert-providers",
    "/api/v1/admin/observability/alert-providers/{providerId}",
    "/api/v1/admin/observability/alert-providers/{providerId}/test",
    "/api/v1/admin/observability/alerts",
    "/api/v1/admin/observability/alerts/{alertKey}",
    "/api/v1/admin/observability/alerts/{alertKey}/acknowledge",
    "/api/v1/admin/observability/alerts/{alertKey}/resolve",
    "/api/v1/admin/observability/alerts/{alertKey}/deliveries",
    "/api/v1/admin/observability/recovery-actions",
    "/api/v1/admin/reviews",
    "/api/v1/admin/reviews/sla/enforce",
    "/api/v1/admin/reviews/{agentId}/approve",
    "/api/v1/admin/reviews/{agentId}/claim",
    "/api/v1/admin/reviews/{agentId}/reject",
    "/api/v1/admin/reviews/{agentId}/needs-changes",
]


def require_paths(spec: dict[str, Any]) -> None:
    paths = spec.get("paths", {})
    for path in REQUIRED_PATHS:
        if path not in paths:
            print(f"[openapi-contract] missing path: {path}", file=sys.stderr)
            raise SystemExit(1)


def require_public_security_empty(spec: dict[str, Any], path: str) -> None:
    post = dig(spec, "paths", path, "post")
    if not isinstance(post, dict) or post.get("security") != []:
        print(f"[openapi-contract] public POST {path} must declare security: []", file=sys.stderr)
        raise SystemExit(1)


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description="Verify OpenAPI release contract without Ruby")
    parser.add_argument("--openapi-file", default=str(repo_root / "docs" / "api" / "openapi.yaml"))
    parser.add_argument("--route-surface-manifest-file", default=str(repo_root / "docs" / "api" / "route-surface-manifest.json"))
    parser.add_argument("--release-contract-file", default=str(repo_root / "config" / "release" / "contract.v1.json"))
    parser.add_argument(
        "--route-surface-schema-file",
        default=str(repo_root / "docs" / "api" / "route-surface-manifest.schema.json"),
    )
    parser.add_argument(
        "--operation-contracts-file",
        default=str(repo_root / "src" / "web" / "src" / "generated" / "operation-contracts.generated.ts"),
    )
    parser.add_argument(
        "--release-projection-file",
        default=str(repo_root / "src" / "web" / "src" / "generated" / "release-projection.generated.ts"),
    )
    return parser.parse_args()


def load_yaml(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        data = yaml.safe_load(handle)
    if not isinstance(data, dict):
        print(f"[openapi-contract] {path} did not parse as an object", file=sys.stderr)
        raise SystemExit(1)
    return data


def load_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        data = json.load(handle)
    if not isinstance(data, dict):
        print(f"[openapi-contract] {path} did not parse as an object", file=sys.stderr)
        raise SystemExit(1)
    return data


def main() -> int:
    args = parse_args()
    spec = load_yaml(Path(args.openapi_file))
    manifest = load_json(Path(args.route_surface_manifest_file))
    contract = load_json(Path(args.release_contract_file))
    route_surface_schema = load_json(Path(args.route_surface_schema_file))
    paths = spec.get("paths", {})
    schemas = dig(spec, "components", "schemas") or {}

    require_paths(spec)
    for path in [
        "/api/v1/channels/webhook/{channelId}",
        "/api/v1/workflows/webhooks/{organizationId}/{workflowId}",
        "/api/v1/billing/stripe/webhook",
        "/api/v1/billing/alipay/webhook",
        "/api/v1/billing/wechatpay/webhook",
        "/api/v1/billing/marketplace-payout/webhook",
    ]:
        require_public_security_empty(spec, path)
    require_relay_alias_bearer_contract(spec, RELAY_ALIAS_PATHS)
    require_websocket_contract(spec)
    require_api_json_responses_use_envelope(spec)
    require_api_success_data_uses_named_schema(spec)
    require_api_json_request_bodies_use_named_schemas(spec)
    require_api_security_surface_contract(spec)
    require_api_path_parameter_contract(spec)
    require_api_operation_metadata_contract(spec)
    require_route_surface_manifest_contract(
        spec,
        manifest,
        contract,
        route_surface_schema,
        {
            "manifest": Path(args.route_surface_manifest_file),
            "typescript": Path(args.operation_contracts_file),
            "releaseProjection": Path(args.release_projection_file),
        },
    )
    require_release_evidence_contract(spec, manifest)
    require_windowed_admin_proof_contract(spec, manifest)
    require_session_csrf_contract(spec)
    require_marketplace_contracts(spec)
    require_publishing_channel_secret_csrf_contract(paths, schemas)
    require_admin_channel_secret_response_contract(paths, schemas)
    require_admin_observability_provider_secret_csrf_contract(paths, schemas)
    require_mcp_auth_token_response_contract(paths, schemas)
    require_workspace_agent_mutation_csrf_contract(paths, schemas)
    require_memory_mutation_csrf_contract(paths, schemas)
    require_agent_run_mutation_csrf_contract(paths, schemas)
    require_billing_checkout_contract(paths, schemas)
    require_quota_topup_csrf_contract(paths, schemas)
    require_tenant_organization_mutation_csrf_contract(paths, schemas)
    require_workflow_management_csrf_contract(paths, schemas)
    require_workflow_execution_control_csrf_contract(paths, schemas)
    require_console_api_token_csrf_contract(paths, schemas)
    require_admin_api_token_contract(paths, schemas)
    require_task_mutation_csrf_contract(paths, schemas)
    require_notification_mutation_csrf_contract(paths)
    require_scheduled_task_contract(paths, schemas)
    require_preferences_mutation_csrf_contract(paths, schemas)
    require_chat_mutation_csrf_contract(paths, schemas)
    require_knowledge_mutation_csrf_contract(paths, schemas)
    require_admin_organization_mutation_csrf_contract(paths, schemas)
    require_admin_core_management_contract(paths, schemas)
    require_admin_billing_contract(paths, schemas)
    require_domestic_payment_webhook_payout_contract(paths, schemas)

    print(
        "[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, "
        "Scheduled Task, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
