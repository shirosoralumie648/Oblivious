#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-provider-live-rail-evidence.sh"
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
  echo "[collect-provider-live-rail-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
provider_proof_file="$tmpdir/stripe-provider-live-rail.json"
artifact_body="$artifact_dir/artifact-provider-stripe-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$provider_proof_file" <<'JSON'
{
  "provider": "stripe",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "stripe_checkout_live_20260616",
    "refund": "stripe_refund_live_20260616",
    "payout": "stripe_payout_live_20260616",
    "reconciliation": "stripe_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 1
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-provider-stripe-20260616 \
  --provider stripe \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$provider_proof_file" \
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
    if artifact["id"] == "artifact-provider-stripe-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-provider-live-rail-evidence-fixtures] generated provider live rail artifact body"

bad_provider_file="$tmpdir/provider-live-rail-bad-provider.json"
cat >"$bad_provider_file" <<'JSON'
{
  "provider": "alipay",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "alipay_checkout_live_20260616",
    "refund": "alipay_refund_live_20260616",
    "payout": "alipay_payout_live_20260616",
    "reconciliation": "alipay_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 1
  }
}
JSON

bad_provider_output="$tmpdir/bad-provider.out"
if bash "$collector" \
  --artifact-id artifact-provider-stripe-20260616 \
  --provider stripe \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_provider_file" \
  --output "$tmpdir/bad-provider-live-rail-artifact.json" >"$bad_provider_output" 2>&1; then
  cat "$bad_provider_output" >&2
  fail "mismatched provider live rail proof unexpectedly passed"
fi
if ! grep -Fq -- "proof provider must match provider" "$bad_provider_output"; then
  cat "$bad_provider_output" >&2
  fail "bad provider live rail proof failed without provider diagnostic"
fi
echo "[collect-provider-live-rail-evidence-fixtures] rejected mismatched provider live rail proof"

bad_summary_file="$tmpdir/provider-live-rail-bad-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "provider": "stripe",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "stripe_checkout_live_20260616",
    "refund": "stripe_refund_live_20260616",
    "payout": "stripe_payout_live_20260616",
    "reconciliation": "stripe_reconciliation_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 0
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-provider-stripe-20260616 \
  --provider stripe \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-provider-live-rail-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete provider live rail summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.reconciliationChecks must be a positive integer" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad provider live rail summary failed without reconciliation diagnostic"
fi
echo "[collect-provider-live-rail-evidence-fixtures] rejected incomplete provider live rail summary"

bad_references_file="$tmpdir/provider-live-rail-bad-references.json"
cat >"$bad_references_file" <<'JSON'
{
  "provider": "stripe",
  "mode": "live",
  "providerEnvironment": "live",
  "checkout": "pass",
  "refund": "pass",
  "payout": "pass",
  "reconciliation": "pass",
  "references": {
    "checkout": "stripe_checkout_live_20260616",
    "refund": "stripe_refund_live_20260616",
    "payout": "stripe_payout_live_20260616"
  },
  "summary": {
    "checkoutAttempts": 1,
    "refundAttempts": 1,
    "payoutAttempts": 1,
    "reconciliationChecks": 1
  }
}
JSON

bad_references_output="$tmpdir/bad-references.out"
if bash "$collector" \
  --artifact-id artifact-provider-stripe-20260616 \
  --provider stripe \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_references_file" \
  --output "$tmpdir/bad-provider-live-rail-references-artifact.json" >"$bad_references_output" 2>&1; then
  cat "$bad_references_output" >&2
  fail "incomplete provider live rail references unexpectedly passed"
fi
if ! grep -Fq -- "references.reconciliation is required" "$bad_references_output"; then
  cat "$bad_references_output" >&2
  fail "bad provider live rail references failed without reconciliation reference diagnostic"
fi
echo "[collect-provider-live-rail-evidence-fixtures] rejected incomplete provider live rail references"
