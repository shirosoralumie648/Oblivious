#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-relay-realtime-evidence.sh"
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
  echo "[collect-relay-realtime-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
proof_file="$tmpdir/relay-realtime-proof.json"
artifact_body="$artifact_dir/artifact-relay-realtime-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$proof_file" <<'JSON'
{
  "mode": "disabled_until_commercial_lifecycle",
  "productionPolicyDisabled": "pass",
  "authOriginPrebillAbortUsageBlockers": "pass",
  "summary": {
    "productionPolicyChecks": 1,
    "authOriginPrebillAbortUsageBlockerChecks": 5
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-relay-realtime-20260616 \
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
    if artifact["id"] == "artifact-relay-realtime-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

echo "[collect-relay-realtime-evidence-fixtures] generated Relay Realtime artifact body"

live_manifest="$tmpdir/manifest-live.json"
live_proof_file="$tmpdir/relay-realtime-proof-live.json"
cp "$manifest" "$live_manifest"
cat >"$live_proof_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "authPolicy": "pass",
  "originPolicy": "pass",
  "prebillSettlement": "pass",
  "abortSettlement": "pass",
  "usageLedger": "pass",
  "summary": {
    "totalRequests": 2,
    "authenticatedRequests": 2,
    "requestLinkedUsageRecords": 2,
    "priceSnapshotRecords": 2,
    "abortSettlementRecords": 1,
    "terminalUsageRecords": 2,
    "originPolicyChecks": 1
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-relay-realtime-20260616 \
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
manifest["relayRealtime"] = {
    "mode": "commercial_lifecycle_enabled",
    "productionPolicyEnabled": "pass",
    "authPolicy": "pass",
    "originPolicy": "pass",
    "prebillSettlement": "pass",
    "abortSettlement": "pass",
    "usageLedger": "pass",
    "evidenceRef": "artifact-relay-realtime-20260616",
}
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-realtime-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$live_manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$live_manifest" >/dev/null
echo "[collect-relay-realtime-evidence-fixtures] generated live Relay Realtime artifact body"

bad_summary_file="$tmpdir/relay-realtime-proof-bad-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "mode": "disabled_until_commercial_lifecycle",
  "productionPolicyDisabled": "pass",
  "authOriginPrebillAbortUsageBlockers": "pass",
  "summary": {
    "productionPolicyChecks": 1,
    "authOriginPrebillAbortUsageBlockerChecks": 4
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-relay-realtime-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-relay-realtime-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "incomplete Relay Realtime blocker summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.authOriginPrebillAbortUsageBlockerChecks must cover auth, origin, prebill, abort, and usage blockers" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad Relay Realtime summary failed without blocker diagnostic"
fi
echo "[collect-relay-realtime-evidence-fixtures] rejected incomplete Relay Realtime blocker summary"

bad_live_summary_file="$tmpdir/relay-realtime-proof-bad-live-summary.json"
cat >"$bad_live_summary_file" <<'JSON'
{
  "mode": "commercial_lifecycle_enabled",
  "productionPolicyEnabled": "pass",
  "authPolicy": "pass",
  "originPolicy": "pass",
  "prebillSettlement": "pass",
  "abortSettlement": "pass",
  "usageLedger": "pass",
  "summary": {
    "totalRequests": 2,
    "authenticatedRequests": 1,
    "requestLinkedUsageRecords": 2,
    "priceSnapshotRecords": 2,
    "abortSettlementRecords": 1,
    "terminalUsageRecords": 2,
    "originPolicyChecks": 1
  }
}
JSON

bad_live_summary_output="$tmpdir/bad-live-summary.out"
if bash "$collector" \
  --artifact-id artifact-relay-realtime-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_live_summary_file" \
  --output "$tmpdir/bad-relay-realtime-live-summary-artifact.json" >"$bad_live_summary_output" 2>&1; then
  cat "$bad_live_summary_output" >&2
  fail "incomplete live Relay Realtime summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.authenticatedRequests must equal summary.totalRequests" "$bad_live_summary_output"; then
  cat "$bad_live_summary_output" >&2
  fail "bad live Relay Realtime summary failed without authenticated request diagnostic"
fi
echo "[collect-relay-realtime-evidence-fixtures] rejected incomplete live Relay Realtime summary"
