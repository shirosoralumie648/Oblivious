#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
python_bin="${PYTHON:-python}"
tmpdir=$(mktemp -d)

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[target-release-digests-fixtures] $*" >&2
  exit 1
}

fill_manifest() {
  "$python_bin" "$mutation_helper" --fill "$1"
}

write_artifact_bundle() {
  local manifest="$1"
  local output_dir="$2"

  mkdir -p "$output_dir"
  "$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$output_dir"
}

assert_digest_output() {
  local output="$1"

  "$python_bin" - "$output" <<'PY'
import json
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
data = json.loads(path.read_text(encoding="utf-8"))
sha_re = re.compile(r"^[0-9a-f]{64}$")
for field in ["targetEvidenceSha256", "artifactBundleSha256"]:
    value = data.get(field)
    if not isinstance(value, str) or not sha_re.match(value):
        raise SystemExit(f"{field} is not a SHA-256 digest")
    if value in ("a" * 64, "b" * 64, "0" * 64):
        raise SystemExit(f"{field} was not computed")
if data.get("schema") != "oblivious-target-release-digests-v1":
    raise SystemExit("unexpected digest schema")
if data.get("strictArtifactId") != "artifact-strict-verifier-20260616":
    raise SystemExit("strictArtifactId did not bind to the strict verifier artifact")
if data.get("artifactCount", 0) < 10:
    raise SystemExit("artifactCount did not cover the target release artifact set")
PY
}

compare_digest_fields() {
  local expected="$1"
  local actual="$2"
  local label="$3"

  "$python_bin" - "$expected" "$actual" "$label" <<'PY'
import json
import pathlib
import sys

expected = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
actual = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
label = sys.argv[3]
for field in ["targetEvidenceSha256", "artifactBundleSha256"]:
    if expected[field] != actual[field]:
        raise SystemExit(f"{label}: {field} changed unexpectedly")
PY
}

assert_changed_field() {
  local baseline="$1"
  local changed="$2"
  local field="$3"
  local label="$4"

  "$python_bin" - "$baseline" "$changed" "$field" "$label" <<'PY'
import json
import pathlib
import sys

baseline = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
changed = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
field = sys.argv[3]
label = sys.argv[4]
if baseline[field] == changed[field]:
    raise SystemExit(f"{label}: {field} did not change")
PY
}

assert_unchanged_field() {
  local baseline="$1"
  local changed="$2"
  local field="$3"
  local label="$4"

  "$python_bin" - "$baseline" "$changed" "$field" "$label" <<'PY'
import json
import pathlib
import sys

baseline = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
changed = json.loads(pathlib.Path(sys.argv[2]).read_text(encoding="utf-8"))
field = sys.argv[3]
label = sys.argv[4]
if baseline[field] != changed[field]:
    raise SystemExit(f"{label}: {field} changed unexpectedly")
PY
}

template_manifest="$tmpdir/template.json"
valid_manifest="$tmpdir/valid.json"
artifact_dir="$tmpdir/artifacts"
first_out="$tmpdir/digests-first.json"
write_out="$tmpdir/digests-write.json"
second_out="$tmpdir/digests-second.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$valid_manifest"
fill_manifest "$valid_manifest"
write_artifact_bundle "$valid_manifest" "$artifact_dir"

bash "$digest_tool" --manifest "$valid_manifest" --artifact-dir "$artifact_dir" > "$first_out"
assert_digest_output "$first_out"
echo "[target-release-digests-fixtures] generated canonical target release digests"

bash "$digest_tool" --manifest "$valid_manifest" --artifact-dir "$artifact_dir" --write > "$write_out"
assert_digest_output "$write_out"
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" "$valid_manifest" >/dev/null
echo "[target-release-digests-fixtures] wrote digest fields back to manifest and strict artifact body"

bash "$digest_tool" --manifest "$valid_manifest" --artifact-dir "$artifact_dir" > "$second_out"
compare_digest_fields "$first_out" "$second_out" "post-write recompute"
echo "[target-release-digests-fixtures] digest output is stable after circular fields are refreshed"

strict_mut_manifest="$tmpdir/strict-mutated.json"
strict_mut_artifact_dir="$tmpdir/artifacts-strict-mutated"
strict_mut_out="$tmpdir/digests-strict-mutated.json"
cp "$valid_manifest" "$strict_mut_manifest"
cp -R "$artifact_dir" "$strict_mut_artifact_dir"
"$python_bin" - "$strict_mut_manifest" "$strict_mut_artifact_dir/artifact-strict-verifier-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
body = json.loads(body_path.read_text(encoding="utf-8"))
manifest["strictVerifier"]["targetEvidenceSha256"] = "c" * 64
manifest["strictVerifier"]["artifactBundleSha256"] = "d" * 64
body["targetEvidenceSha256"] = "c" * 64
body["artifactBundleSha256"] = "d" * 64
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-strict-verifier-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
PY
bash "$digest_tool" --manifest "$strict_mut_manifest" --artifact-dir "$strict_mut_artifact_dir" > "$strict_mut_out"
compare_digest_fields "$first_out" "$strict_mut_out" "strict digest field normalization"
echo "[target-release-digests-fixtures] strict digest fields are normalized out of digest computation"

body_mut_manifest="$tmpdir/body-mutated.json"
body_mut_artifact_dir="$tmpdir/artifacts-body-mutated"
body_mut_out="$tmpdir/digests-body-mutated.json"
cp "$valid_manifest" "$body_mut_manifest"
cp -R "$artifact_dir" "$body_mut_artifact_dir"
"$python_bin" - "$body_mut_artifact_dir/artifact-workflow-telemetry-20260616.json" <<'PY'
import json
import pathlib
import sys

body_path = pathlib.Path(sys.argv[1])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["telemetry"]["successfulExecutions"] = body["telemetry"]["successfulExecutions"] - 1
body["telemetry"]["failedExecutions"] = body["telemetry"]["failedExecutions"] + 1
body_path.write_text(json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
PY
bash "$digest_tool" --manifest "$body_mut_manifest" --artifact-dir "$body_mut_artifact_dir" > "$body_mut_out"
assert_changed_field "$first_out" "$body_mut_out" "artifactBundleSha256" "non-strict artifact body mutation"
assert_changed_field "$first_out" "$body_mut_out" "targetEvidenceSha256" "non-strict artifact body mutation"
echo "[target-release-digests-fixtures] non-strict artifact body change updates artifact bundle digest"

manifest_mut="$tmpdir/manifest-mutated.json"
manifest_mut_out="$tmpdir/digests-manifest-mutated.json"
cp "$valid_manifest" "$manifest_mut"
"$python_bin" - "$manifest_mut" <<'PY'
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
manifest["environment"]["baseUrl"] = "https://target-alt.oblivious.internal"
manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
PY
bash "$digest_tool" --manifest "$manifest_mut" --artifact-dir "$artifact_dir" > "$manifest_mut_out"
assert_changed_field "$first_out" "$manifest_mut_out" "targetEvidenceSha256" "manifest environment mutation"
assert_unchanged_field "$first_out" "$manifest_mut_out" "artifactBundleSha256" "manifest environment mutation"
echo "[target-release-digests-fixtures] manifest environment change updates target evidence digest"

echo "[target-release-digests-fixtures] target release digest behavior is guarded."
