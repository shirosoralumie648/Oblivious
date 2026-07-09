#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
preflight="$repo_root/scripts/verify-commercial-preflight.mjs"
completion="$repo_root/scripts/verify-commercial-completion.sh"
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)
python_bin="${PYTHON:-python}"
inside_repo_root="$repo_root/.tmp/preflight-inside-repo-fixture"

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  rm -rf "$tmpdir"
  rm -rf "$inside_repo_root"
}
trap cleanup EXIT

fail() {
  echo "[commercial-preflight-fixtures] $*" >&2
  exit 1
}

manifest="$tmpdir/target-release-evidence.json"
artifact_dir="$tmpdir/artifacts"
corrupt_artifact_dir="$tmpdir/artifacts-corrupt"
semantic_corrupt_artifact_dir="$tmpdir/artifacts-semantic-corrupt"
secret_file="$tmpdir/secret.yaml"
valid_output="$tmpdir/preflight-valid.out"
corrupt_output="$tmpdir/preflight-corrupt.out"
target_valid_output="$tmpdir/preflight-target-valid.out"
target_corrupt_output="$tmpdir/preflight-target-corrupt.out"
missing_commit_manifest="$tmpdir/target-release-evidence-missing-commit.json"
missing_commit_output="$tmpdir/preflight-missing-commit.out"
semantic_corrupt_manifest="$tmpdir/target-release-evidence-semantic-corrupt.json"
semantic_corrupt_output="$tmpdir/preflight-semantic-corrupt.out"
completion_missing_commit_output="$tmpdir/completion-missing-commit.out"
completion_semantic_corrupt_output="$tmpdir/completion-semantic-corrupt.out"
completion_missing_flags_output="$tmpdir/completion-missing-final-flags.out"
completion_missing_required_inputs_output="$tmpdir/completion-missing-required-inputs.out"
completion_missing_secret_output="$tmpdir/completion-missing-k8s-secret.out"
inside_repo_manifest="$inside_repo_root/target-release-evidence.json"
inside_repo_artifact_dir="$inside_repo_root/artifacts"
inside_repo_manifest_output="$tmpdir/preflight-inside-repo-manifest.out"
inside_repo_artifact_output="$tmpdir/preflight-inside-repo-artifacts.out"

if TEST_DATABASE_URL="postgres://oblivious:oblivious@127.0.0.1:5432/oblivious?sslmode=disable" \
  bash "$completion" >"$completion_missing_flags_output" 2>&1; then
  cat "$completion_missing_flags_output" >&2
  fail "commercial completion unexpectedly accepted missing final gate flags"
fi
if ! grep -Fq -- "strict final readiness requires" "$completion_missing_flags_output"; then
  cat "$completion_missing_flags_output" >&2
  fail "commercial completion missing final gate flags did not fail fast with strict readiness message"
fi
if grep -Fq -- "START docs gate" "$completion_missing_flags_output"; then
  cat "$completion_missing_flags_output" >&2
  fail "commercial completion reached docs gate before rejecting missing final gate flags"
fi
echo "[commercial-preflight-fixtures] rejected missing final gate flags before heavy checks"

if COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
  COMMERCIAL_COMPLETION_RUN_K8S=true \
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
  bash "$completion" >"$completion_missing_required_inputs_output" 2>&1; then
  cat "$completion_missing_required_inputs_output" >&2
  fail "commercial completion unexpectedly accepted missing strict final input bundle"
fi
for expected in \
  "strict final readiness missing required inputs" \
  "TEST_DATABASE_URL is required for DB-backed Phase 30 commercial journey proof" \
  "OBLIVIOUS_K8S_SECRET_FILE is required when COMMERCIAL_COMPLETION_RUN_K8S=true" \
  "target live evidence manifest requires OBLIVIOUS_TARGET_EVIDENCE_FILE for strict final readiness" \
  "target live evidence manifest requires OBLIVIOUS_TARGET_ARTIFACT_DIR with downloaded artifact bodies for strict final readiness"; do
  if ! grep -Fq -- "$expected" "$completion_missing_required_inputs_output"; then
    cat "$completion_missing_required_inputs_output" >&2
    fail "commercial completion missing strict input bundle did not include: $expected"
  fi
done
if grep -Fq -- "START docs gate" "$completion_missing_required_inputs_output"; then
  cat "$completion_missing_required_inputs_output" >&2
  fail "commercial completion reached docs gate before rejecting missing strict final input bundle"
fi
echo "[commercial-preflight-fixtures] rejected missing strict final input bundle before heavy checks"

if TEST_DATABASE_URL="postgres://oblivious:oblivious@127.0.0.1:5432/oblivious?sslmode=disable" \
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
  COMMERCIAL_COMPLETION_RUN_K8S=true \
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
  bash "$completion" >"$completion_missing_secret_output" 2>&1; then
  cat "$completion_missing_secret_output" >&2
  fail "commercial completion unexpectedly accepted missing Kubernetes secret file"
fi
if ! grep -Fq -- "OBLIVIOUS_K8S_SECRET_FILE is required when COMMERCIAL_COMPLETION_RUN_K8S=true" "$completion_missing_secret_output"; then
  cat "$completion_missing_secret_output" >&2
  fail "commercial completion missing Kubernetes secret did not fail fast with expected message"
fi
if grep -Fq -- "START docs gate" "$completion_missing_secret_output"; then
  cat "$completion_missing_secret_output" >&2
  fail "commercial completion reached docs gate before rejecting missing Kubernetes secret"
fi
echo "[commercial-preflight-fixtures] rejected missing Kubernetes secret before heavy checks"

bash "$verifier" --print-template > "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"
bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
cp -R "$artifact_dir" "$corrupt_artifact_dir"
cp "$manifest" "$missing_commit_manifest"
cp "$manifest" "$semantic_corrupt_manifest"
cp -R "$artifact_dir" "$semantic_corrupt_artifact_dir"

cat >"$secret_file" <<'YAML'
apiVersion: v1
kind: Secret
metadata:
  name: oblivious-runtime
stringData:
  SESSION_SECRET: fixture-session-secret
YAML

run_preflight() {
  local output="$1"
  local bodies="$2"
  local evidence="${3:-$manifest}"

  TEST_DATABASE_URL="postgres://oblivious:oblivious@127.0.0.1:5432/oblivious?sslmode=disable" \
    COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
    COMMERCIAL_COMPLETION_RUN_K8S=true \
    COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
    COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
    OBLIVIOUS_K8S_SECRET_FILE="$secret_file" \
    OBLIVIOUS_TARGET_EVIDENCE_FILE="$evidence" \
    OBLIVIOUS_TARGET_ARTIFACT_DIR="$bodies" \
    node "$preflight" --local >"$output" 2>&1
}

run_target_preflight() {
  local output="$1"
  local bodies="$2"
  local evidence="${3:-$manifest}"

  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
    OBLIVIOUS_TARGET_EVIDENCE_FILE="$evidence" \
    OBLIVIOUS_TARGET_ARTIFACT_DIR="$bodies" \
    node "$preflight" --target-evidence-only >"$output" 2>&1
}

run_completion_target_preflight() {
  local output="$1"
  local evidence="$2"
  local bodies="$3"

  TEST_DATABASE_URL="postgres://oblivious:oblivious@127.0.0.1:5432/oblivious?sslmode=disable" \
    COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
    COMMERCIAL_COMPLETION_RUN_K8S=true \
    COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
    COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
    OBLIVIOUS_K8S_SECRET_FILE="$secret_file" \
    OBLIVIOUS_TARGET_EVIDENCE_FILE="$evidence" \
    OBLIVIOUS_TARGET_ARTIFACT_DIR="$bodies" \
    bash "$completion" >"$output" 2>&1
}

run_preflight "$valid_output" "$artifact_dir"
if ! grep -Fq -- "PASS target artifact body coverage" "$valid_output"; then
  cat "$valid_output" >&2
  fail "valid preflight fixture did not prove artifact body coverage"
fi
run_target_preflight "$target_valid_output" "$artifact_dir"
if ! grep -Fq -- "PASS target artifact body coverage" "$target_valid_output"; then
  cat "$target_valid_output" >&2
  fail "valid target-only preflight fixture did not prove artifact body coverage"
fi
if ! grep -Fq -- "PASS target evidence verifier" "$target_valid_output"; then
  cat "$target_valid_output" >&2
  fail "valid target-only preflight fixture did not run the full target evidence verifier"
fi
echo "[commercial-preflight-fixtures] accepted artifact body coverage"

mkdir -p "$inside_repo_root"
cp "$manifest" "$inside_repo_manifest"
if run_target_preflight "$inside_repo_manifest_output" "$artifact_dir" "$inside_repo_manifest"; then
  cat "$inside_repo_manifest_output" >&2
  fail "inside-repo manifest target-only preflight unexpectedly passed"
fi
if ! grep -Fq -- "strict final readiness requires an external, untracked target evidence manifest outside the repository" "$inside_repo_manifest_output"; then
  cat "$inside_repo_manifest_output" >&2
  fail "inside-repo manifest target-only preflight failed without external manifest diagnostic"
fi
echo "[commercial-preflight-fixtures] rejected inside-repo target evidence manifest"

cp -R "$artifact_dir" "$inside_repo_artifact_dir"
if run_target_preflight "$inside_repo_artifact_output" "$inside_repo_artifact_dir"; then
  cat "$inside_repo_artifact_output" >&2
  fail "inside-repo artifact body directory target-only preflight unexpectedly passed"
fi
if ! grep -Fq -- "strict final readiness requires an external, untracked artifact body directory outside the repository" "$inside_repo_artifact_output"; then
  cat "$inside_repo_artifact_output" >&2
  fail "inside-repo artifact body directory target-only preflight failed without external artifact directory diagnostic"
fi
echo "[commercial-preflight-fixtures] rejected inside-repo target artifact body directory"

"$python_bin" - "$missing_commit_manifest" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload.pop("commit", None)
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
if run_target_preflight "$missing_commit_output" "$artifact_dir" "$missing_commit_manifest"; then
  cat "$missing_commit_output" >&2
  fail "missing-commit target-only preflight unexpectedly passed"
fi
if ! grep -Fq -- "FAIL target evidence verifier" "$missing_commit_output" || ! grep -Fq -- "commit is required" "$missing_commit_output"; then
  cat "$missing_commit_output" >&2
  fail "missing-commit target-only preflight failed without target verifier diagnostic"
fi
if run_completion_target_preflight "$completion_missing_commit_output" "$missing_commit_manifest" "$artifact_dir"; then
  cat "$completion_missing_commit_output" >&2
  fail "commercial completion unexpectedly accepted missing target evidence commit"
fi
if ! grep -Fq -- "commit is required" "$completion_missing_commit_output"; then
  cat "$completion_missing_commit_output" >&2
  fail "commercial completion missing target commit failed without verifier diagnostic"
fi
if grep -Fq -- "START docs gate" "$completion_missing_commit_output"; then
  cat "$completion_missing_commit_output" >&2
  fail "commercial completion reached docs gate before rejecting missing target evidence commit"
fi
echo "[commercial-preflight-fixtures] rejected missing target evidence commit before heavy checks"

"$python_bin" - "$semantic_corrupt_manifest" "$semantic_corrupt_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("rawParserReplay", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
if run_target_preflight "$semantic_corrupt_output" "$semantic_corrupt_artifact_dir" "$semantic_corrupt_manifest"; then
  cat "$semantic_corrupt_output" >&2
  fail "semantic-corrupt target-only preflight unexpectedly passed"
fi
if ! grep -Fq -- "FAIL target evidence verifier" "$semantic_corrupt_output" || ! grep -Fq -- "body proofs.rawParserReplay must be pass" "$semantic_corrupt_output"; then
  cat "$semantic_corrupt_output" >&2
  fail "semantic-corrupt target-only preflight failed without body proof diagnostic"
fi
if run_completion_target_preflight "$completion_semantic_corrupt_output" "$semantic_corrupt_manifest" "$semantic_corrupt_artifact_dir"; then
  cat "$completion_semantic_corrupt_output" >&2
  fail "commercial completion unexpectedly accepted semantic-corrupt target artifact body"
fi
if ! grep -Fq -- "body proofs.rawParserReplay must be pass" "$completion_semantic_corrupt_output"; then
  cat "$completion_semantic_corrupt_output" >&2
  fail "commercial completion semantic-corrupt target artifact failed without verifier diagnostic"
fi
if grep -Fq -- "START docs gate" "$completion_semantic_corrupt_output"; then
  cat "$completion_semantic_corrupt_output" >&2
  fail "commercial completion reached docs gate before rejecting semantic-corrupt target artifact body"
fi
echo "[commercial-preflight-fixtures] rejected semantic target artifact body before heavy checks"

printf '\ncorrupted\n' >> "$corrupt_artifact_dir/artifact-strict-verifier-20260616.json"
run_preflight "$corrupt_output" "$corrupt_artifact_dir"
if ! grep -Fq -- "FAIL target artifact body coverage" "$corrupt_output"; then
  cat "$corrupt_output" >&2
  fail "corrupt preflight fixture did not fail artifact body coverage"
fi
if ! grep -Fq -- "sha256 mismatch" "$corrupt_output"; then
  cat "$corrupt_output" >&2
  fail "corrupt preflight fixture did not report sha256 mismatch"
fi
if run_target_preflight "$target_corrupt_output" "$corrupt_artifact_dir"; then
  cat "$target_corrupt_output" >&2
  fail "corrupt target-only preflight fixture unexpectedly passed"
fi
if ! grep -Fq -- "FAIL target artifact body coverage" "$target_corrupt_output"; then
  cat "$target_corrupt_output" >&2
  fail "corrupt target-only preflight fixture did not fail artifact body coverage"
fi
if ! grep -Fq -- "sha256 mismatch" "$target_corrupt_output"; then
  cat "$target_corrupt_output" >&2
  fail "corrupt target-only preflight fixture did not report sha256 mismatch"
fi
echo "[commercial-preflight-fixtures] rejected corrupt artifact body SHA"
