#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-relay-batch-evidence.sh"
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
  echo "[collect-relay-batch-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
proof_file="$tmpdir/relay-batch-proof.json"
artifact_body="$artifact_dir/artifact-relay-batch-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$proof_file" <<'JSON'
{
  "mode": "disabled_until_commercial_lifecycle",
  "productionPolicyDisabled": "pass",
  "prebillPollingSettlementRefundAuditUsageBlockers": "pass",
  "summary": {
    "productionPolicyChecks": 1,
    "prebillPollingSettlementRefundAuditUsageBlockerChecks": 6
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-relay-batch-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$proof_file" \
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
    if artifact["id"] == "artifact-relay-batch-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

echo "[collect-relay-batch-evidence-fixtures] generated Relay Batch artifact body"

live_manifest="$tmpdir/manifest-live.json"
live_proof_file="$tmpdir/relay-batch-proof-live.json"
cp "$manifest" "$live_manifest"
cat >"$live_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 2,
    "pollingCompletions": 1,
    "settlementRecords": 1,
    "refundRecords": 1,
    "usageAuditRecords": 2,
    "requestLogAuditRecords": 2,
    "terminalFailureRecords": 1
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-relay-batch-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$live_proof_file" \
  --output "$artifact_body" >/dev/null

"$python_bin" - "$live_manifest" "$artifact_body" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body_bytes = body_path.read_bytes()
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
manifest["relayBatch"] = {
    "mode": "commercial_lifecycle_enabled",
    "productionPolicyEnabled": "pass",
    "prebillReservation": "pass",
    "pollingCompletion": "pass",
    "settlement": "pass",
    "refund": "pass",
    "usageAudit": "pass",
    "evidenceRef": "artifact-relay-batch-20260616",
}
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-batch-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$live_manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$live_manifest" >/dev/null
echo "[collect-relay-batch-evidence-fixtures] generated live Relay Batch artifact body"

bad_summary_file="$tmpdir/relay-batch-proof-bad-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "mode": "disabled_until_commercial_lifecycle",
  "productionPolicyDisabled": "pass",
  "prebillPollingSettlementRefundAuditUsageBlockers": "pass",
  "summary": {
    "productionPolicyChecks": 1,
    "prebillPollingSettlementRefundAuditUsageBlockerChecks": 5
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-relay-batch-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-relay-batch-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete Relay Batch blocker summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.prebillPollingSettlementRefundAuditUsageBlockerChecks must cover prebill, polling, settlement, refund, audit, and usage blockers" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad Relay Batch summary failed without blocker diagnostic"
fi
echo "[collect-relay-batch-evidence-fixtures] rejected incomplete Relay Batch blocker summary"

bad_live_summary_file="$tmpdir/relay-batch-proof-bad-live-summary.json"
cat >"$bad_live_summary_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 2,
    "pollingCompletions": 1,
    "settlementRecords": 1,
    "refundRecords": 1,
    "usageAuditRecords": 1,
    "requestLogAuditRecords": 2,
    "terminalFailureRecords": 1
  }
}
JSON

bad_live_summary_output="$tmpdir/bad-live-summary.out"
if bash "$collector" \
  --artifact-id artifact-relay-batch-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_live_summary_file" \
  --output "$tmpdir/bad-relay-batch-live-summary-artifact.json" >"$bad_live_summary_output" 2>&1; then
  cat "$bad_live_summary_output" >&2
  fail "incomplete live Relay Batch summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.usageAuditRecords must cover summary.settlementRecords plus summary.refundRecords" "$bad_live_summary_output"; then
  cat "$bad_live_summary_output" >&2
  fail "bad live Relay Batch summary failed without usage audit diagnostic"
fi
echo "[collect-relay-batch-evidence-fixtures] rejected incomplete live Relay Batch summary"

bad_request_log_summary_file="$tmpdir/relay-batch-proof-bad-request-log-summary.json"
cat >"$bad_request_log_summary_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "prebillReservation": "pass",
  "pollingCompletion": "pass",
  "settlement": "pass",
  "refund": "pass",
  "usageAudit": "pass",
  "summary": {
    "prebillReservations": 2,
    "pollingCompletions": 1,
    "settlementRecords": 1,
    "refundRecords": 1,
    "usageAuditRecords": 2,
    "requestLogAuditRecords": 1,
    "terminalFailureRecords": 1
  }
}
JSON

bad_request_log_summary_output="$tmpdir/bad-request-log-summary.out"
if bash "$collector" \
  --artifact-id artifact-relay-batch-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_request_log_summary_file" \
  --output "$tmpdir/bad-relay-batch-request-log-summary-artifact.json" >"$bad_request_log_summary_output" 2>&1; then
  cat "$bad_request_log_summary_output" >&2
  fail "incomplete live Relay Batch request-log summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.requestLogAuditRecords must cover summary.settlementRecords plus summary.refundRecords" "$bad_request_log_summary_output"; then
  cat "$bad_request_log_summary_output" >&2
  fail "bad live Relay Batch request-log summary failed without request-log audit diagnostic"
fi
echo "[collect-relay-batch-evidence-fixtures] rejected incomplete live Relay Batch request-log summary"
