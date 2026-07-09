#!/usr/bin/env python3
import argparse
import hashlib
import json
import pathlib
import subprocess
import sys


ARTIFACT_COLLECTORS = [
    {
        "kind": "strict-verifier-log",
        "impl": "collect_strict_verifier_evidence.py",
        "one_of": [
            ("--proof-file", "strict_verifier_file", "--strict-verifier-file"),
            ("--proof-url", "strict_verifier_url", "--strict-verifier-url"),
        ],
    },
    {
        "kind": "deployment-log",
        "impl": "collect_deployment_evidence.py",
        "one_of": [
            ("--proof-file", "deployment_proof_file", "--deployment-proof-file"),
            ("--proof-url", "deployment_proof_url", "--deployment-proof-url"),
        ],
    },
    {
        "kind": "kubernetes-validation",
        "impl": "collect_kubernetes_evidence.py",
        "one_of": [
            ("--proof-file", "kubernetes_proof_file", "--kubernetes-proof-file"),
            ("--proof-url", "kubernetes_proof_url", "--kubernetes-proof-url"),
        ],
    },
    {
        "kind": "workflow-telemetry",
        "impl": "collect_workflow_telemetry_evidence.py",
        "one_of": [
            ("--proof-file", "workflow_telemetry_file", "--workflow-telemetry-file"),
            ("--proof-url", "workflow_telemetry_url", "--workflow-telemetry-url"),
        ],
    },
    {
        "kind": "request-log-observability",
        "impl": "collect_request_log_observability_evidence.py",
        "one_of_groups": [
            [
                ("--platform-proof-file", "request_log_platform_proof_file", "--request-log-platform-proof-file"),
                ("--platform-proof-url", "request_log_platform_proof_url", "--request-log-platform-proof-url"),
            ],
            [
                ("--coverage-file", "coverage_file", "--coverage-file"),
                ("--coverage-url", "coverage_url", "--coverage-url"),
                ("--target-base-url", "target_base_url", "--target-base-url"),
            ],
            [
                ("--slo-file", "slo_file", "--slo-file"),
                ("--slo-url", "slo_url", "--slo-url"),
            ],
        ],
        "repeat_args": [
            ("--coverage-query", "coverage_query"),
        ],
    },
    {
        "kind": "rag-indexing-proof",
        "impl": "collect_rag_indexing_evidence.py",
        "one_of": [
            ("--proof-file", "rag_proof_file", "--rag-proof-file"),
            ("--proof-url", "rag_proof_url", "--rag-proof-url"),
        ],
    },
    {
        "kind": "relay-realtime-proof",
        "impl": "collect_relay_realtime_evidence.py",
        "one_of": [
            ("--proof-file", "relay_realtime_proof_file", "--relay-realtime-proof-file"),
            ("--proof-url", "relay_realtime_proof_url", "--relay-realtime-proof-url"),
        ],
    },
    {
        "kind": "relay-batch-proof",
        "impl": "collect_relay_batch_evidence.py",
        "one_of": [
            ("--proof-file", "relay_batch_proof_file", "--relay-batch-proof-file"),
            ("--proof-url", "relay_batch_proof_url", "--relay-batch-proof-url"),
        ],
    },
    {
        "kind": "marketplace-payout-proof",
        "impl": "collect_marketplace_payout_evidence.py",
        "one_of": [
            ("--proof-file", "marketplace_payout_proof_file", "--marketplace-payout-proof-file"),
            ("--proof-url", "marketplace_payout_proof_url", "--marketplace-payout-proof-url"),
        ],
    },
    {
        "kind": "marketplace-governance-proof",
        "impl": "collect_marketplace_governance_evidence.py",
        "one_of": [
            ("--proof-file", "marketplace_governance_proof_file", "--marketplace-governance-proof-file"),
            ("--proof-url", "marketplace_governance_proof_url", "--marketplace-governance-proof-url"),
        ],
    },
    {
        "kind": "provider-runtime-config",
        "impl": "collect_provider_runtime_config_evidence.py",
        "one_of": [
            ("--proof-file", "provider_runtime_config_proof_file", "--provider-runtime-config-proof-file"),
            ("--proof-url", "provider_runtime_config_proof_url", "--provider-runtime-config-proof-url"),
        ],
    },
    {
        "kind": "provider-live-rail",
        "provider": "stripe",
        "impl": "collect_provider_live_rail_evidence.py",
        "static_args": [("--provider", "stripe")],
        "one_of": [
            ("--proof-file", "stripe_provider_live_rail_file", "--stripe-provider-live-rail-file"),
            ("--proof-url", "stripe_provider_live_rail_url", "--stripe-provider-live-rail-url"),
        ],
    },
    {
        "kind": "provider-live-rail",
        "provider": "alipay",
        "impl": "collect_provider_live_rail_evidence.py",
        "static_args": [("--provider", "alipay")],
        "one_of": [
            ("--proof-file", "alipay_provider_live_rail_file", "--alipay-provider-live-rail-file"),
            ("--proof-url", "alipay_provider_live_rail_url", "--alipay-provider-live-rail-url"),
        ],
    },
    {
        "kind": "provider-live-rail",
        "provider": "wechatpay",
        "impl": "collect_provider_live_rail_evidence.py",
        "static_args": [("--provider", "wechatpay")],
        "one_of": [
            ("--proof-file", "wechatpay_provider_live_rail_file", "--wechatpay-provider-live-rail-file"),
            ("--proof-url", "wechatpay_provider_live_rail_url", "--wechatpay-provider-live-rail-url"),
        ],
    },
    {
        "kind": "grpc-smoke-report",
        "impl": "collect_grpc_smoke_report_evidence.py",
        "one_of": [
            ("--proof-file", "grpc_smoke_file", "--grpc-smoke-file"),
            ("--proof-url", "grpc_smoke_url", "--grpc-smoke-url"),
        ],
    },
    {
        "kind": "secret-audit",
        "impl": "collect_secret_audit_evidence.py",
        "one_of": [
            ("--proof-file", "secret_audit_file", "--secret-audit-file"),
            ("--proof-url", "secret_audit_url", "--secret-audit-url"),
        ],
    },
    {
        "kind": "microservice-database-proof",
        "impl": "collect_microservice_database_evidence.py",
        "one_of": [
            ("--proof-file", "microservice_database_proof_file", "--microservice-database-proof-file"),
            ("--proof-url", "microservice_database_proof_url", "--microservice-database-proof-url"),
        ],
    },
]


def fail(message):
    raise SystemExit(f"[collect-target-release-artifacts] {message}")


def require_path(args, attr, flag):
    value = getattr(args, attr)
    if not value:
        fail(f"{flag} is required")
    return str(pathlib.Path(value))


def require_one_of(args, choices):
    provided = []
    for collector_flag, attr, public_flag in choices:
        value = getattr(args, attr)
        if value:
            provided.append((collector_flag, str(value), public_flag))
    if len(provided) != 1:
        public_flags = ", ".join(choice[2] for choice in choices)
        fail(f"exactly one of {public_flags} is required")
    return [provided[0][0], provided[0][1]]


def append_optional_args(args, extra_args, choices):
    for collector_flag, attr in choices:
        value = getattr(args, attr)
        if isinstance(value, list):
            for item in value:
                extra_args.extend([collector_flag, str(item)])
        elif value is not None:
            extra_args.extend([collector_flag, str(value)])


def read_manifest(path):
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        fail(f"--manifest file is required: {path}")
    except json.JSONDecodeError as error:
        fail(f"--manifest must be valid JSON: {error}")
    if not isinstance(payload, dict):
        fail("--manifest must be a JSON object")
    artifacts = payload.get("artifacts")
    if not isinstance(artifacts, list):
        fail("--manifest must include artifacts[]")
    return payload


def find_artifact(manifest, kind, provider=None):
    matches = [
        item
        for item in manifest["artifacts"]
        if isinstance(item, dict)
        and item.get("kind") == kind
        and (provider is None or item.get("provider") == provider)
    ]
    if len(matches) != 1:
        label = f"{kind} provider {provider}" if provider else kind
        fail(f"manifest must include exactly one artifact kind {label}")
    artifact = matches[0]
    for field in ["id", "commit", "runId", "recordedAt"]:
        if not isinstance(artifact.get(field), str) or artifact[field].strip() == "":
            fail(f"artifact kind {kind} must include {field}")
    if provider is not None and artifact.get("provider") != provider:
        fail(f"artifact kind {kind} must include provider {provider}")
    return artifact


def run_collector(repo_root, impl_name, artifact, extra_args, output_path):
    command = [
        sys.executable,
        str(repo_root / "scripts" / impl_name),
        "--artifact-id",
        artifact["id"],
        "--commit",
        artifact["commit"],
        "--run-id",
        artifact["runId"],
        "--recorded-at",
        artifact["recordedAt"],
    ]
    command.extend(extra_args)
    command.extend(["--output", str(output_path)])
    subprocess.run(command, check=True, stdout=subprocess.DEVNULL)


def collect(args):
    repo_root = pathlib.Path(args.repo_root)
    manifest_path = pathlib.Path(require_path(args, "manifest", "--manifest"))
    artifact_dir = pathlib.Path(require_path(args, "artifact_dir", "--artifact-dir"))
    manifest = read_manifest(manifest_path)
    artifact_dir.mkdir(parents=True, exist_ok=True)

    for spec in ARTIFACT_COLLECTORS:
        artifact = find_artifact(manifest, spec["kind"], spec.get("provider"))
        output_path = artifact_dir / f"{artifact['id']}.json"
        extra_args = []
        for flag, value in spec.get("static_args", []):
            extra_args.extend([flag, value])
        for collector_flag, attr, public_flag in spec.get("args", []):
            extra_args.extend([collector_flag, require_path(args, attr, public_flag)])
        if "one_of" in spec:
            extra_args.extend(require_one_of(args, spec["one_of"]))
        for choices in spec.get("one_of_groups", []):
            extra_args.extend(require_one_of(args, choices))
        append_optional_args(args, extra_args, spec.get("repeat_args", []))
        append_optional_args(
            args,
            extra_args,
            [
                ("--bearer-token-env", "bearer_token_env"),
                ("--bearer-token-file", "bearer_token_file"),
                ("--cookie-file", "cookie_file"),
                ("--timeout-seconds", "timeout_seconds"),
            ],
        )
        run_collector(repo_root, spec["impl"], artifact, extra_args, output_path)
        artifact["sha256"] = hashlib.sha256(output_path.read_bytes()).hexdigest()

    manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--strict-verifier-file")
    parser.add_argument("--strict-verifier-url")
    parser.add_argument("--deployment-proof-file")
    parser.add_argument("--deployment-proof-url")
    parser.add_argument("--kubernetes-proof-file")
    parser.add_argument("--kubernetes-proof-url")
    parser.add_argument("--workflow-telemetry-file")
    parser.add_argument("--workflow-telemetry-url")
    parser.add_argument("--request-log-platform-proof-file")
    parser.add_argument("--request-log-platform-proof-url")
    parser.add_argument("--coverage-file")
    parser.add_argument("--coverage-url")
    parser.add_argument("--target-base-url")
    parser.add_argument("--coverage-query", action="append", default=[])
    parser.add_argument("--slo-file")
    parser.add_argument("--slo-url")
    parser.add_argument("--rag-proof-file")
    parser.add_argument("--rag-proof-url")
    parser.add_argument("--relay-realtime-proof-file")
    parser.add_argument("--relay-realtime-proof-url")
    parser.add_argument("--relay-batch-proof-file")
    parser.add_argument("--relay-batch-proof-url")
    parser.add_argument("--marketplace-payout-proof-file")
    parser.add_argument("--marketplace-payout-proof-url")
    parser.add_argument("--marketplace-governance-proof-file")
    parser.add_argument("--marketplace-governance-proof-url")
    parser.add_argument("--provider-runtime-config-proof-file")
    parser.add_argument("--provider-runtime-config-proof-url")
    parser.add_argument("--stripe-provider-live-rail-file")
    parser.add_argument("--stripe-provider-live-rail-url")
    parser.add_argument("--alipay-provider-live-rail-file")
    parser.add_argument("--alipay-provider-live-rail-url")
    parser.add_argument("--wechatpay-provider-live-rail-file")
    parser.add_argument("--wechatpay-provider-live-rail-url")
    parser.add_argument("--grpc-smoke-file")
    parser.add_argument("--grpc-smoke-url")
    parser.add_argument("--secret-audit-file")
    parser.add_argument("--secret-audit-url")
    parser.add_argument("--microservice-database-proof-file")
    parser.add_argument("--microservice-database-proof-url")
    parser.add_argument("--bearer-token-env")
    parser.add_argument("--bearer-token-file")
    parser.add_argument("--cookie-file")
    parser.add_argument("--timeout-seconds", type=float)
    args = parser.parse_args()
    try:
        collect(args)
    except subprocess.CalledProcessError as error:
        sys.exit(error.returncode)
    print(f"[collect-target-release-artifacts] wrote artifact bodies under {args.artifact_dir}")


if __name__ == "__main__":
    main()
