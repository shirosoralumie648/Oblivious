#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-provider-runtime-config-evidence.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[collect-provider-runtime-config-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
provider_config_file="$tmpdir/provider-runtime-config-proof.json"
artifact_body="$artifact_dir/artifact-provider-runtime-config-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$provider_config_file" <<'JSON'
{
  "stripe": "pass",
  "alipay": "pass",
  "wechatpay": "pass",
  "providerEnv": "pass",
  "checkoutBaseUrls": "pass",
  "webhookRoutes": "pass",
  "webhookVerification": "pass",
  "providers": [
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260616"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260616"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260616"}
  ],
  "summary": {
    "providersConfigured": 3,
    "providerEnvVarsChecked": 9,
    "checkoutBaseUrlsChecked": 3,
    "webhookRoutesChecked": 3,
    "webhookVerificationChecks": 3
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-provider-runtime-config-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$provider_config_file" \
  --output "$artifact_body" >/dev/null

"$python_bin" - "$manifest" "$artifact_body" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body_bytes = body_path.read_bytes()
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-provider-runtime-config-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-provider-runtime-config-evidence-fixtures] generated provider runtime config artifact body"

bad_proof_file="$tmpdir/provider-runtime-config-proof-bad.json"
cat >"$bad_proof_file" <<'JSON'
{
  "stripe": "pass",
  "alipay": "fail",
  "wechatpay": "pass",
  "providerEnv": "pass",
  "checkoutBaseUrls": "pass",
  "webhookRoutes": "pass",
  "webhookVerification": "pass",
  "providers": [
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260616"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260616"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260616"}
  ],
  "summary": {
    "providersConfigured": 3,
    "providerEnvVarsChecked": 9,
    "checkoutBaseUrlsChecked": 3,
    "webhookRoutesChecked": 3,
    "webhookVerificationChecks": 3
  }
}
JSON

bad_output="$tmpdir/bad-proof.out"
if bash "$collector" \
  --artifact-id artifact-provider-runtime-config-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_proof_file" \
  --output "$tmpdir/bad-provider-runtime-config-artifact.json" >"$bad_output" 2>&1; then
  cat "$bad_output" >&2
  fail "failed Alipay runtime config proof unexpectedly passed"
fi
if ! grep -Fq -- "alipay must be pass" "$bad_output"; then
  cat "$bad_output" >&2
  fail "bad provider config proof failed without alipay diagnostic"
fi
echo "[collect-provider-runtime-config-evidence-fixtures] rejected failed Alipay runtime config proof"

bad_summary_file="$tmpdir/provider-runtime-config-proof-bad-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "stripe": "pass",
  "alipay": "pass",
  "wechatpay": "pass",
  "providerEnv": "pass",
  "checkoutBaseUrls": "pass",
  "webhookRoutes": "pass",
  "webhookVerification": "pass",
  "providers": [
    {"name": "stripe", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_stripe_20260616"},
    {"name": "alipay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_alipay_20260616"},
    {"name": "wechatpay", "providerEnvironment": "live", "providerEnv": "pass", "checkoutBaseUrlClass": "external-filled", "webhookRoute": "pass", "webhookVerification": "pass", "evidenceId": "provider_runtime_config_wechatpay_20260616"}
  ],
  "summary": {
    "providersConfigured": 3,
    "providerEnvVarsChecked": 9,
    "checkoutBaseUrlsChecked": 2,
    "webhookRoutesChecked": 3,
    "webhookVerificationChecks": 3
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-provider-runtime-config-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-provider-runtime-config-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete provider checkout base URL summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.checkoutBaseUrlsChecked must cover summary.providersConfigured" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad provider config summary failed without checkout base URL diagnostic"
fi
echo "[collect-provider-runtime-config-evidence-fixtures] rejected incomplete checkout base URL summary"

bad_provider_details_file="$tmpdir/provider-runtime-config-proof-bad-provider-details.json"
cp "$provider_config_file" "$bad_provider_details_file"
"$python_bin" - "$bad_provider_details_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["providers"] = [item for item in payload["providers"] if item["name"] != "wechatpay"]
payload["summary"]["providersConfigured"] = 2
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_provider_details_output="$tmpdir/bad-provider-details.out"
if bash "$collector" \
  --artifact-id artifact-provider-runtime-config-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_provider_details_file" \
  --output "$tmpdir/bad-provider-runtime-config-details-artifact.json" >"$bad_provider_details_output" 2>&1; then
  cat "$bad_provider_details_output" >&2
  fail "provider runtime config missing WeChat Pay details unexpectedly passed"
fi
if ! grep -Fq -- "providers must include stripe, alipay, and wechatpay (missing: wechatpay)" "$bad_provider_details_output"; then
  cat "$bad_provider_details_output" >&2
  fail "bad provider details failed without WeChat Pay diagnostic"
fi
echo "[collect-provider-runtime-config-evidence-fixtures] rejected missing WeChat Pay provider details"
