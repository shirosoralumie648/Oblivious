#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-deployment-evidence.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT
fail() { echo "[collect-deployment-evidence-fixtures] $*" >&2; exit 1; }

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
proof_file="$tmpdir/deployment-proof.json"
artifact_body="$artifact_dir/artifact-deploy-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$proof_file" <<'JSON'
{
  "result": "pass",
  "targetEnvironment": "production",
  "deployValidation": "pass",
  "backupRestore": "pass",
  "migrationReplay": "pass",
  "references": {
    "deployValidation": "deploy-validation-run-20260616",
    "backupRestore": "backup-restore-run-20260616",
    "migrationReplay": "migration-replay-run-20260616"
  }
}
JSON

bash "$collector" --artifact-id artifact-deploy-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$proof_file" --output "$artifact_body" >/dev/null
"$python_bin" - "$manifest" "$artifact_body" <<'PY'
import hashlib, json, pathlib, sys
manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body_bytes = body_path.read_bytes()
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-deploy-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-deployment-evidence-fixtures] generated deployment artifact body"

bad_file="$tmpdir/deployment-bad-backup.json"
cp "$proof_file" "$bad_file"
"$python_bin" - "$bad_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["backupRestore"] = "fail"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_output="$tmpdir/bad-backup.out"
if bash "$collector" --artifact-id artifact-deploy-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_file" --output "$tmpdir/bad-deploy.json" >"$bad_output" 2>&1; then
  cat "$bad_output" >&2
  fail "failed backup restore deployment proof unexpectedly passed"
fi
if ! grep -Fq -- "backupRestore must be pass" "$bad_output"; then
  cat "$bad_output" >&2
  fail "bad deployment proof failed without backup diagnostic"
fi
echo "[collect-deployment-evidence-fixtures] rejected failed backup restore deployment proof"

bad_reference_file="$tmpdir/deployment-bad-reference.json"
cp "$proof_file" "$bad_reference_file"
"$python_bin" - "$bad_reference_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["references"].pop("backupRestore", None)
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_reference_output="$tmpdir/bad-reference.out"
if bash "$collector" --artifact-id artifact-deploy-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_reference_file" --output "$tmpdir/bad-deploy-reference.json" >"$bad_reference_output" 2>&1; then
  cat "$bad_reference_output" >&2
  fail "deployment proof missing backup restore reference unexpectedly passed"
fi
if ! grep -Fq -- "references.backupRestore is required" "$bad_reference_output"; then
  cat "$bad_reference_output" >&2
  fail "bad deployment reference failed without backup reference diagnostic"
fi
echo "[collect-deployment-evidence-fixtures] rejected missing backup restore reference"
