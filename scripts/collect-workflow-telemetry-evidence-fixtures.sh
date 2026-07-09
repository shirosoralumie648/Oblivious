#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-workflow-telemetry-evidence.sh"
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
  echo "[collect-workflow-telemetry-evidence-fixtures] $*" >&2
  exit 1
}

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
telemetry_file="$tmpdir/workflow-telemetry.json"
artifact_body="$artifact_dir/artifact-workflow-telemetry-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$telemetry_file" <<'JSON'
{
  "successRate": 0.99,
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "totalExecutions": 100,
  "successfulExecutions": 99,
  "failedExecutions": 1
}
JSON

bash "$collector" \
  --artifact-id artifact-workflow-telemetry-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$telemetry_file" \
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
    if artifact["id"] == "artifact-workflow-telemetry-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY

bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-workflow-telemetry-evidence-fixtures] generated workflow telemetry artifact body"

bad_telemetry_file="$tmpdir/workflow-telemetry-bad-rate.json"
cat >"$bad_telemetry_file" <<'JSON'
{
  "successRate": 0.99,
  "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
  "totalExecutions": 100,
  "successfulExecutions": 98,
  "failedExecutions": 2
}
JSON

bad_rate_output="$tmpdir/bad-rate.out"
if bash "$collector" \
  --artifact-id artifact-workflow-telemetry-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_telemetry_file" \
  --output "$tmpdir/bad-workflow-telemetry-artifact.json" >"$bad_rate_output" 2>&1; then
  cat "$bad_rate_output" >&2
  fail "inconsistent workflow telemetry success rate unexpectedly passed"
fi
if ! grep -Fq -- "telemetry.successRate must equal telemetry.successfulExecutions / telemetry.totalExecutions" "$bad_rate_output"; then
  cat "$bad_rate_output" >&2
  fail "bad workflow telemetry failed without success rate diagnostic"
fi
echo "[collect-workflow-telemetry-evidence-fixtures] rejected inconsistent workflow telemetry success rate"

bad_window_file="$tmpdir/workflow-telemetry-bad-window.json"
cat >"$bad_window_file" <<'JSON'
{
  "successRate": 0.99,
  "window": "2026-06-16T01:00:00Z/2026-06-16T00:00:00Z",
  "totalExecutions": 100,
  "successfulExecutions": 99,
  "failedExecutions": 1
}
JSON

bad_window_output="$tmpdir/bad-window.out"
if bash "$collector" \
  --artifact-id artifact-workflow-telemetry-20260616 \
  --commit "$(git -C "$repo_root" rev-parse HEAD)" \
  --run-id target-release-20260616 \
  --recorded-at 2026-06-16T01:00:00Z \
  --proof-file "$bad_window_file" \
  --output "$tmpdir/bad-workflow-telemetry-window-artifact.json" >"$bad_window_output" 2>&1; then
  cat "$bad_window_output" >&2
  fail "inverted workflow telemetry window unexpectedly passed"
fi
if ! grep -Fq -- "telemetry.window end must be at or after start" "$bad_window_output"; then
  cat "$bad_window_output" >&2
  fail "bad workflow telemetry window failed without window diagnostic"
fi
echo "[collect-workflow-telemetry-evidence-fixtures] rejected inverted workflow telemetry window"
