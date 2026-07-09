#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-secret-audit-evidence.sh"
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
  echo "[collect-secret-audit-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
secret_audit_file="$tmpdir/secret-audit.json"
artifact_body="$artifact_dir/artifact-secret-audit-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$secret_audit_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-06-16T00:55:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 42,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 0
  }
}
JSON

bash "$collector" \
  --artifact-id artifact-secret-audit-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$secret_audit_file" \
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
    if artifact["id"] == "artifact-secret-audit-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-secret-audit-evidence-fixtures] generated secret audit artifact body"

bad_findings_file="$tmpdir/secret-audit-findings.json"
cat >"$bad_findings_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-06-16T00:55:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [{"scope": "providers", "severity": "high"}],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 42,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 0
  }
}
JSON

bad_findings_output="$tmpdir/bad-findings.out"
if bash "$collector" \
  --artifact-id artifact-secret-audit-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_findings_file" \
  --output "$tmpdir/bad-secret-audit-findings-artifact.json" >"$bad_findings_output" 2>&1; then
  cat "$bad_findings_output" >&2
  fail "secret audit findings unexpectedly passed"
fi
if ! grep -Fq -- "findings must be an empty array" "$bad_findings_output"; then
  cat "$bad_findings_output" >&2
  fail "bad secret audit findings failed without findings diagnostic"
fi
echo "[collect-secret-audit-evidence-fixtures] rejected secret audit findings"

bad_scope_file="$tmpdir/secret-audit-scope.json"
cat >"$bad_scope_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-06-16T00:55:00Z",
  "scope": ["runtime"],
  "findings": [],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 42,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 0
  }
}
JSON

bad_scope_output="$tmpdir/bad-scope.out"
if bash "$collector" \
  --artifact-id artifact-secret-audit-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_scope_file" \
  --output "$tmpdir/bad-secret-audit-scope-artifact.json" >"$bad_scope_output" 2>&1; then
  cat "$bad_scope_output" >&2
  fail "incomplete secret audit scope unexpectedly passed"
fi
if ! grep -Fq -- "scope must include kubernetes, providers, and runtime" "$bad_scope_output"; then
  cat "$bad_scope_output" >&2
  fail "bad secret audit scope failed without scope diagnostic"
fi
echo "[collect-secret-audit-evidence-fixtures] rejected incomplete secret audit scope"

bad_summary_file="$tmpdir/secret-audit-summary.json"
cat >"$bad_summary_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-06-16T00:55:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 41,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 1
  }
}
JSON

bad_summary_output="$tmpdir/bad-summary.out"
if bash "$collector" \
  --artifact-id artifact-secret-audit-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_summary_file" \
  --output "$tmpdir/bad-secret-audit-summary-artifact.json" >"$bad_summary_output" 2>&1; then
  cat "$bad_summary_output" >&2
  fail "rotation-required secret audit summary unexpectedly passed"
fi
if ! grep -Fq -- "summary.rotationRequiredRecords must be zero" "$bad_summary_output"; then
  cat "$bad_summary_output" >&2
  fail "bad secret audit summary failed without rotation diagnostic"
fi
echo "[collect-secret-audit-evidence-fixtures] rejected rotation-required secret audit summary"

bad_secret_file="$tmpdir/secret-audit-embedded-token.json"
cat >"$bad_secret_file" <<'JSON'
{
  "result": "pass",
  "checkedAt": "2026-06-16T00:55:00Z",
  "scope": ["kubernetes", "providers", "runtime"],
  "findings": [],
  "summary": {
    "totalRecordsScanned": 42,
    "protectedRecords": 42,
    "plaintextRecords": 0,
    "invalidProtectedRecords": 0,
    "rotationRequiredRecords": 0
  },
  "apiToken": "target-secret-token"
}
JSON

bad_secret_output="$tmpdir/bad-secret.out"
if bash "$collector" \
  --artifact-id artifact-secret-audit-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_secret_file" \
  --output "$tmpdir/bad-secret-audit-token-artifact.json" >"$bad_secret_output" 2>&1; then
  cat "$bad_secret_output" >&2
  fail "embedded secret audit token unexpectedly passed"
fi
if ! grep -Fq -- "proof.apiToken must not embed secret material" "$bad_secret_output"; then
  cat "$bad_secret_output" >&2
  fail "bad secret audit token failed without secret diagnostic"
fi
echo "[collect-secret-audit-evidence-fixtures] rejected embedded secret audit token"
