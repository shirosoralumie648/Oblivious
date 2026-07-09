#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
collector="$repo_root/scripts/collect-kubernetes-evidence.sh"
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
fail() { echo "[collect-kubernetes-evidence-fixtures] $*" >&2; exit 1; }

template_manifest="$tmpdir/template.json"
manifest="$tmpdir/manifest.json"
artifact_dir="$tmpdir/artifacts"
proof_file="$tmpdir/kubernetes-proof.json"
artifact_body="$artifact_dir/artifact-k8s-20260616.json"

bash "$verifier" --print-template > "$template_manifest"
cp "$template_manifest" "$manifest"
"$python_bin" "$mutation_helper" --fill "$manifest"
"$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$artifact_dir"

cat >"$proof_file" <<'JSON'
{
  "result": "pass",
  "targetEnvironment": "production",
  "clusterRef": "prod-cluster-20260616",
  "namespace": "oblivious",
  "validation": "pass",
  "rollout": "pass",
  "failover": "pass",
  "secretFileClass": "external-filled",
  "references": {
    "validation": "k8s-validation-run-20260616",
    "rollout": "k8s-rollout-run-20260616",
    "failover": "k8s-failover-run-20260616"
  }
}
JSON

bash "$collector" --artifact-id artifact-k8s-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$proof_file" --output "$artifact_body" >/dev/null
"$python_bin" - "$manifest" "$artifact_body" <<'PY'
import hashlib, json, pathlib, sys
manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body_bytes = body_path.read_bytes()
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-k8s-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bash "$digest_tool" --manifest "$manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" --allow-file-collection-source "$manifest" >/dev/null
echo "[collect-kubernetes-evidence-fixtures] generated Kubernetes artifact body"

bad_secret_file="$tmpdir/kubernetes-bad-secret-class.json"
cp "$proof_file" "$bad_secret_file"
"$python_bin" - "$bad_secret_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["secretFileClass"] = "inline"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_secret_output="$tmpdir/bad-secret-class.out"
if bash "$collector" --artifact-id artifact-k8s-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_secret_file" --output "$tmpdir/bad-k8s-secret.json" >"$bad_secret_output" 2>&1; then
  cat "$bad_secret_output" >&2
  fail "unfilled Kubernetes secret class unexpectedly passed"
fi
if ! grep -Fq -- "secretFileClass must be external-filled" "$bad_secret_output"; then
  cat "$bad_secret_output" >&2
  fail "bad Kubernetes secret class failed without diagnostic"
fi
echo "[collect-kubernetes-evidence-fixtures] rejected unfilled Kubernetes secret class"

bad_failover_file="$tmpdir/kubernetes-bad-failover.json"
cp "$proof_file" "$bad_failover_file"
"$python_bin" - "$bad_failover_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["failover"] = "fail"
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_failover_output="$tmpdir/bad-failover.out"
if bash "$collector" --artifact-id artifact-k8s-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_failover_file" --output "$tmpdir/bad-k8s-failover.json" >"$bad_failover_output" 2>&1; then
  cat "$bad_failover_output" >&2
  fail "failed Kubernetes failover proof unexpectedly passed"
fi
if ! grep -Fq -- "failover must be pass" "$bad_failover_output"; then
  cat "$bad_failover_output" >&2
  fail "bad Kubernetes failover failed without diagnostic"
fi
echo "[collect-kubernetes-evidence-fixtures] rejected failed Kubernetes failover proof"

bad_cluster_file="$tmpdir/kubernetes-bad-cluster.json"
cp "$proof_file" "$bad_cluster_file"
"$python_bin" - "$bad_cluster_file" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload.pop("clusterRef", None)
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
bad_cluster_output="$tmpdir/bad-cluster.out"
if bash "$collector" --artifact-id artifact-k8s-20260616 --commit "$(git -C "$repo_root" rev-parse HEAD)" --run-id target-release-20260616 --recorded-at 2026-06-16T01:00:00Z --proof-file "$bad_cluster_file" --output "$tmpdir/bad-k8s-cluster.json" >"$bad_cluster_output" 2>&1; then
  cat "$bad_cluster_output" >&2
  fail "Kubernetes proof missing clusterRef unexpectedly passed"
fi
if ! grep -Fq -- "clusterRef is required" "$bad_cluster_output"; then
  cat "$bad_cluster_output" >&2
  fail "bad Kubernetes clusterRef failed without diagnostic"
fi
echo "[collect-kubernetes-evidence-fixtures] rejected missing Kubernetes clusterRef"
