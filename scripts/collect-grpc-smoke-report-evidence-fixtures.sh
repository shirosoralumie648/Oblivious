#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-grpc-smoke-report-evidence.sh"
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
  echo "[collect-grpc-smoke-report-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
grpc_smoke_file="$tmpdir/grpc-smoke.json"
artifact_body="$artifact_dir/artifact-grpc-smoke-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$grpc_smoke_file" <<'JSON'
{
  "recordedAt": "2026-06-16T00:59:00Z",
  "timeout": "10s",
  "results": [
    {
      "service": "agent",
      "address": "agent.prod.oblivious.release.test:50063",
      "generatedClient": "pass",
      "status": "validation_error"
    },
    {
      "service": "workflow",
      "address": "workflow.prod.oblivious.release.test:50064",
      "generatedClient": "pass",
      "status": "validation_response"
    },
    {
      "service": "task",
      "address": "task.prod.oblivious.release.test:50065",
      "generatedClient": "pass",
      "status": "validation_response"
    }
  ]
}
JSON

"$python_bin" - "$manifest" <<'PY'
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
manifest["grpcSmokeReport"]["recordedAt"] = "2026-06-16T00:59:00Z"
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$collector" \
  --artifact-id artifact-grpc-smoke-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$grpc_smoke_file" \
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
    if artifact["id"] == "artifact-grpc-smoke-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-grpc-smoke-report-evidence-fixtures] generated gRPC smoke report artifact body"

bad_generated_client_file="$tmpdir/grpc-smoke-bad-generated-client.json"
cp "$grpc_smoke_file" "$bad_generated_client_file"
"$python_bin" - "$bad_generated_client_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["results"][2]["generatedClient"] = "fail"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bad_generated_client_output="$tmpdir/bad-generated-client.out"
if bash "$collector" \
  --artifact-id artifact-grpc-smoke-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_generated_client_file" \
  --output "$tmpdir/bad-grpc-smoke-generated-client-artifact.json" >"$bad_generated_client_output" 2>&1; then
  cat "$bad_generated_client_output" >&2
  fail "failed generated-client gRPC smoke proof unexpectedly passed"
fi
if ! grep -Fq -- "results[2].generatedClient must be pass" "$bad_generated_client_output"; then
  cat "$bad_generated_client_output" >&2
  fail "bad generated-client gRPC smoke proof failed without generatedClient diagnostic"
fi
echo "[collect-grpc-smoke-report-evidence-fixtures] rejected failed generated-client gRPC smoke proof"

bad_status_file="$tmpdir/grpc-smoke-bad-status.json"
cp "$grpc_smoke_file" "$bad_status_file"
"$python_bin" - "$bad_status_file" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["results"][1]["status"] = "validation_error"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bad_status_output="$tmpdir/bad-status.out"
if bash "$collector" \
  --artifact-id artifact-grpc-smoke-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_status_file" \
  --output "$tmpdir/bad-grpc-smoke-status-artifact.json" >"$bad_status_output" 2>&1; then
  cat "$bad_status_output" >&2
  fail "workflow status mismatch gRPC smoke proof unexpectedly passed"
fi
if ! grep -Fq -- "results[1].status for workflow must be validation_response" "$bad_status_output"; then
  cat "$bad_status_output" >&2
  fail "bad workflow gRPC smoke proof failed without status diagnostic"
fi
echo "[collect-grpc-smoke-report-evidence-fixtures] rejected workflow status mismatch gRPC smoke proof"
