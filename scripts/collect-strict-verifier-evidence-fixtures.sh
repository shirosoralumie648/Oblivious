#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-strict-verifier-evidence.sh"
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
  echo "[collect-strict-verifier-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
proof_file="$tmpdir/strict-verifier.json"
artifact_body="$artifact_dir/artifact-strict-verifier-20260616.json"
current_commit=$(git -C "$repo_root" rev-parse HEAD)

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$proof_file" <<JSON
{
  "artifactBundleSha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "command": "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
  "commit": "$current_commit",
  "result": "pass",
  "runId": "target-release-20260616",
  "skippedChecks": [],
  "startedAt": "2026-06-16T00:00:00Z",
  "completedAt": "2026-06-16T01:00:00Z",
  "targetEvidenceSha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}
JSON

bash "$collector" \
  --artifact-id artifact-strict-verifier-20260616 \
  --commit "$current_commit" \
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
    if artifact["id"] == "artifact-strict-verifier-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-strict-verifier-evidence-fixtures] generated strict verifier artifact body"

bad_skips_file="$tmpdir/strict-verifier-skips.json"
cp "$proof_file" "$bad_skips_file"
"$python_bin" - "$bad_skips_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["skippedChecks"] = ["deploy"]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_skips_output="$tmpdir/bad-skips.out"
if bash "$collector" --artifact-id artifact-strict-verifier-20260616 --commit "$current_commit" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_skips_file" --output "$tmpdir/bad-strict-skips.json" >"$bad_skips_output" 2>&1; then
  cat "$bad_skips_output" >&2
  fail "strict verifier skipped checks unexpectedly passed"
fi
if ! grep -Fq -- "skippedChecks must be an empty array" "$bad_skips_output"; then
  cat "$bad_skips_output" >&2
  fail "bad strict verifier skips failed without skip diagnostic"
fi
echo "[collect-strict-verifier-evidence-fixtures] rejected strict verifier skipped checks"

bad_command_file="$tmpdir/strict-verifier-command.json"
cp "$proof_file" "$bad_command_file"
"$python_bin" - "$bad_command_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["command"] = "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true " + payload["command"]
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_command_output="$tmpdir/bad-command.out"
if bash "$collector" --artifact-id artifact-strict-verifier-20260616 --commit "$current_commit" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_command_file" --output "$tmpdir/bad-strict-command.json" >"$bad_command_output" 2>&1; then
  cat "$bad_command_output" >&2
  fail "strict verifier skip command unexpectedly passed"
fi
if ! grep -Fq -- "command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS" "$bad_command_output"; then
  cat "$bad_command_output" >&2
  fail "bad strict verifier command failed without command diagnostic"
fi
echo "[collect-strict-verifier-evidence-fixtures] rejected strict verifier skip command"

bad_commit_file="$tmpdir/strict-verifier-commit.json"
cp "$proof_file" "$bad_commit_file"
"$python_bin" - "$bad_commit_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["commit"] = "0" * 40
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_commit_output="$tmpdir/bad-commit.out"
if bash "$collector" --artifact-id artifact-strict-verifier-20260616 --commit "$current_commit" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_commit_file" --output "$tmpdir/bad-strict-commit.json" >"$bad_commit_output" 2>&1; then
  cat "$bad_commit_output" >&2
  fail "strict verifier proof commit mismatch unexpectedly passed"
fi
if ! grep -Fq -- "proof.commit must match --commit" "$bad_commit_output"; then
  cat "$bad_commit_output" >&2
  fail "bad strict verifier commit failed without commit diagnostic"
fi
echo "[collect-strict-verifier-evidence-fixtures] rejected strict verifier commit mismatch"

bad_run_file="$tmpdir/strict-verifier-run.json"
cp "$proof_file" "$bad_run_file"
"$python_bin" - "$bad_run_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["runId"] = "target-release-other"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_run_output="$tmpdir/bad-run.out"
if bash "$collector" --artifact-id artifact-strict-verifier-20260616 --commit "$current_commit" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_run_file" --output "$tmpdir/bad-strict-run.json" >"$bad_run_output" 2>&1; then
  cat "$bad_run_output" >&2
  fail "strict verifier proof run mismatch unexpectedly passed"
fi
if ! grep -Fq -- "proof.runId must match --run-id" "$bad_run_output"; then
  cat "$bad_run_output" >&2
  fail "bad strict verifier run failed without run diagnostic"
fi
echo "[collect-strict-verifier-evidence-fixtures] rejected strict verifier run mismatch"
