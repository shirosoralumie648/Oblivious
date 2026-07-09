#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-marketplace-payout-evidence.sh"
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
  echo "[collect-marketplace-payout-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
payout_proof_file="$tmpdir/marketplace-payout-proof.json"
artifact_body="$artifact_dir/artifact-marketplace-payouts-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$payout_proof_file" <<'JSON'
{
  "outboundDispatch": "pass",
  "inboundWebhookLifecycle": "pass",
  "settlementLedger": "pass",
  "reconciliation": "pass",
  "refundChargebackHandling": "pass",
  "summary": {
    "outboundDispatches": 3,
    "webhookEvents": 3,
    "settlementLedgerEntries": 3,
    "reconciledEntries": 3,
    "refundChargebackCases": 1,
    "refundChargebackCasesHandled": 1
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-marketplace-payouts-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$payout_proof_file" \
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
    if artifact["id"] == "artifact-marketplace-payouts-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-marketplace-payout-evidence-fixtures] generated marketplace payout artifact body"

bad_proof_file="$tmpdir/marketplace-payout-proof-bad.json"
cat >"$bad_proof_file" <<'JSON'
{
  "outboundDispatch": "pass",
  "inboundWebhookLifecycle": "pass",
  "settlementLedger": "pass",
  "reconciliation": "pass",
  "refundChargebackHandling": "fail",
  "summary": {
    "outboundDispatches": 3,
    "webhookEvents": 3,
    "settlementLedgerEntries": 3,
    "reconciledEntries": 3,
    "refundChargebackCases": 1,
    "refundChargebackCasesHandled": 1
  }
}
JSON

bad_output="$tmpdir/bad-proof.out"
if bash "$collector" \
  --artifact-id artifact-marketplace-payouts-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_proof_file" \
  --output "$tmpdir/bad-payout-artifact.json" >"$bad_output" 2>&1; then
  cat "$bad_output" >&2
  fail "failed refund/chargeback proof unexpectedly passed"
fi
if ! grep -Fq -- "refundChargebackHandling must be pass" "$bad_output"; then
  cat "$bad_output" >&2
  fail "bad payout proof failed without refundChargebackHandling diagnostic"
fi
echo "[collect-marketplace-payout-evidence-fixtures] rejected failed refund chargeback proof"

bad_summary_file="$tmpdir/marketplace-payout-proof-bad-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "outboundDispatch": "pass",
  "inboundWebhookLifecycle": "pass",
  "settlementLedger": "pass",
  "reconciliation": "pass",
  "refundChargebackHandling": "pass",
  "summary": {
    "outboundDispatches": 3,
    "webhookEvents": 3,
    "settlementLedgerEntries": 3,
    "reconciledEntries": 2,
    "refundChargebackCases": 1,
    "refundChargebackCasesHandled": 1
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-marketplace-payouts-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-payout-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete payout reconciliation summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.reconciledEntries must equal summary.settlementLedgerEntries" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad payout summary failed without reconciliation diagnostic"
fi
echo "[collect-marketplace-payout-evidence-fixtures] rejected incomplete payout reconciliation summary"
