#!/usr/bin/env python3
import argparse
import json
import pathlib
import re
from datetime import datetime

import target_evidence_source


ARTIFACT_KIND = "provider-runtime-config"
REQUIRED_PROVIDERS = ["stripe", "alipay", "wechatpay"]
PASS_FIELDS = [
    "stripe",
    "alipay",
    "wechatpay",
    "providerEnv",
    "checkoutBaseUrls",
    "webhookRoutes",
    "webhookVerification",
]
PASS_FIELD_ERRORS = {
    "stripe": "stripe must be pass",
    "alipay": "alipay must be pass",
    "wechatpay": "wechatpay must be pass",
    "providerEnv": "providerEnv must be pass",
    "checkoutBaseUrls": "checkoutBaseUrls must be pass",
    "webhookRoutes": "webhookRoutes must be pass",
    "webhookVerification": "webhookVerification must be pass",
}
PLACEHOLDER_PATTERN = re.compile(r"TODO|TBD|placeholder|example|sample|fake", re.IGNORECASE)
EMBEDDED_SECRET_PATTERN = re.compile(
    r"(?:token|secret|password|passwd|api[_-]?key|access[_-]?key|credential|private[_-]?key)",
    re.IGNORECASE,
)


def fail(message):
    raise SystemExit(f"[collect-provider-runtime-config-evidence] {message}")


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


def build_summary(proof):
    summary = proof.get("summary")
    if not isinstance(summary, dict):
        fail("summary must be a JSON object")
    providers_configured = require_count(summary, "providersConfigured", positive=True)
    provider_env_vars_checked = require_count(summary, "providerEnvVarsChecked", positive=True)
    checkout_base_urls_checked = require_count(summary, "checkoutBaseUrlsChecked", positive=True)
    webhook_routes_checked = require_count(summary, "webhookRoutesChecked", positive=True)
    webhook_verification_checks = require_count(summary, "webhookVerificationChecks", positive=True)
    if providers_configured < 3:
        fail("summary.providersConfigured must include Stripe, Alipay, and WeChat Pay")
    if provider_env_vars_checked < providers_configured:
        fail("summary.providerEnvVarsChecked must cover summary.providersConfigured")
    if checkout_base_urls_checked < providers_configured:
        fail("summary.checkoutBaseUrlsChecked must cover summary.providersConfigured")
    if webhook_routes_checked < providers_configured:
        fail("summary.webhookRoutesChecked must cover summary.providersConfigured")
    if webhook_verification_checks < providers_configured:
        fail("summary.webhookVerificationChecks must cover summary.providersConfigured")
    return {
        "providersConfigured": providers_configured,
        "providerEnvVarsChecked": provider_env_vars_checked,
        "checkoutBaseUrlsChecked": checkout_base_urls_checked,
        "webhookRoutesChecked": webhook_routes_checked,
        "webhookVerificationChecks": webhook_verification_checks,
    }


def build_provider_details(proof):
    providers = proof.get("providers")
    if not isinstance(providers, list) or not providers:
        fail("providers must be a non-empty array")
    output = []
    seen = set()
    for index, item in enumerate(providers):
        label = f"providers[{index}]"
        if not isinstance(item, dict):
            fail(f"{label} must be a JSON object")
        name = require_concrete_string(item.get("name"), f"{label}.name").strip().lower()
        if name in seen:
            fail(f"providers must not duplicate {name}")
        seen.add(name)
        if item.get("providerEnvironment") != "live":
            fail(f"{label}.providerEnvironment must be live")
        if item.get("checkoutBaseUrlClass") != "external-filled":
            fail(f"{label}.checkoutBaseUrlClass must be external-filled")
        output.append(
            {
                "name": name,
                "providerEnvironment": "live",
                "providerEnv": require_detail_pass(item, "providerEnv", label),
                "checkoutBaseUrlClass": "external-filled",
                "webhookRoute": require_detail_pass(item, "webhookRoute", label),
                "webhookVerification": require_detail_pass(item, "webhookVerification", label),
                "evidenceId": require_concrete_string(item.get("evidenceId"), f"{label}.evidenceId"),
            }
        )
    missing = [name for name in REQUIRED_PROVIDERS if name not in seen]
    if missing:
        fail("providers must include stripe, alipay, and wechatpay (missing: " + ", ".join(missing) + ")")
    return output


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
        "providers": build_provider_details(proof),
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
    print(f"[collect-provider-runtime-config-evidence] wrote {output}")


if __name__ == "__main__":
    main()
