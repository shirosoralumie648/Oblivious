#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
python_bin="${PYTHON:-python}"
mutation_helper="$repo_root/scripts/target_release_fixture_mutations.py"
digest_tool="$repo_root/scripts/compute-target-release-digests.sh"
tmpdir=$(mktemp -d)

if [[ -z "${PYTHON:-}" ]] && ! command -v "$python_bin" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  python_bin=python3
fi

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[target-release-evidence-fixtures] $*" >&2
  exit 1
}

fill_manifest() {
  "$python_bin" "$mutation_helper" --fill "$1"
}

mutate_json() {
  "$python_bin" "$mutation_helper" --mutate "$2" "$1"
}

write_artifact_bundle() {
  local manifest="$1"
  local output_dir="$2"

  mkdir -p "$output_dir"
  "$python_bin" "$mutation_helper" --write-artifacts "$manifest" "$output_dir"
}

expect_failure() {
  local label="$1"
  local path="$2"
  local expected_pattern="$3"
  local output="$tmpdir/${label//[^A-Za-z0-9_.-]/_}.out"

  if bash "$verifier" --manifest-only "$path" >"$output" 2>&1; then
    cat "$output" >&2
    fail "$label unexpectedly passed"
  fi
  if ! grep -Fq -- "$expected_pattern" "$output"; then
    cat "$output" >&2
    fail "$label failed without expected pattern: $expected_pattern"
  fi
  echo "[target-release-evidence-fixtures] rejected $label"
}

expect_artifact_bundle_failure() {
  local label="$1"
  local path="$2"
  local artifact_dir="$3"
  local expected_pattern="$4"
  local output="$tmpdir/${label//[^A-Za-z0-9_.-]/_}.out"

  if OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" "$path" >"$output" 2>&1; then
    cat "$output" >&2
    fail "$label unexpectedly passed"
  fi
  if ! grep -Fq -- "$expected_pattern" "$output"; then
    cat "$output" >&2
    fail "$label failed without expected pattern: $expected_pattern"
  fi
  echo "[target-release-evidence-fixtures] rejected $label"
}

make_invalid_case() {
  local label="$1"
  local expected_pattern="$2"
  local path="$tmpdir/$label.json"

  cp "$valid_manifest" "$path"
  mutate_json "$path" "$label"
  expect_failure "$label" "$path" "$expected_pattern"
}

template_manifest="$tmpdir/template.json"
valid_manifest="$tmpdir/valid.json"

bash "$verifier" --print-template > "$template_manifest"
expect_failure "generated-template-placeholders" "$template_manifest" "environment.name must reference a concrete target environment value, not a placeholder"

cp "$template_manifest" "$valid_manifest"
fill_manifest "$valid_manifest"
bash "$verifier" --manifest-only "$valid_manifest" >/dev/null
echo "[target-release-evidence-fixtures] accepted filled current-commit manifest"

missing_artifact_dir_output="$tmpdir/final-manifest-missing-artifact-dir.out"
if bash "$verifier" "$valid_manifest" >"$missing_artifact_dir_output" 2>&1; then
  cat "$missing_artifact_dir_output" >&2
  fail "final-manifest-missing-artifact-dir unexpectedly passed"
fi
if ! grep -Fq -- "OBLIVIOUS_TARGET_ARTIFACT_DIR is required for final target evidence validation" "$missing_artifact_dir_output"; then
  cat "$missing_artifact_dir_output" >&2
  fail "final-manifest-missing-artifact-dir failed without artifact dir diagnostic"
fi
echo "[target-release-evidence-fixtures] rejected final-manifest-missing-artifact-dir"

artifact_dir="$tmpdir/artifacts"
secret_file="$tmpdir/secret.yaml"
write_artifact_bundle "$valid_manifest" "$artifact_dir"
bash "$digest_tool" --manifest "$valid_manifest" --artifact-dir "$artifact_dir" --write >/dev/null
OBLIVIOUS_TARGET_ARTIFACT_DIR="$artifact_dir" bash "$verifier" "$valid_manifest" >/dev/null
echo "[target-release-evidence-fixtures] accepted filled manifest with downloaded artifact bundle"

bad_canonical_digest_manifest="$tmpdir/valid-bad-canonical-digest.json"
bad_canonical_digest_artifact_dir="$tmpdir/artifacts-bad-canonical-digest"
cp "$valid_manifest" "$bad_canonical_digest_manifest"
cp -R "$artifact_dir" "$bad_canonical_digest_artifact_dir"
"$python_bin" - "$bad_canonical_digest_manifest" "$bad_canonical_digest_artifact_dir/artifact-strict-verifier-20260616.json" <<'PY'
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
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-canonical-digest-mismatch" "$bad_canonical_digest_manifest" "$bad_canonical_digest_artifact_dir" "strictVerifier.targetEvidenceSha256 must match canonical target release digest"

bad_strict_digest_manifest="$tmpdir/valid-bad-strict-digest-body.json"
bad_strict_digest_artifact_dir="$tmpdir/artifacts-bad-strict-digest"
cp "$valid_manifest" "$bad_strict_digest_manifest"
write_artifact_bundle "$bad_strict_digest_manifest" "$bad_strict_digest_artifact_dir"
"$python_bin" - "$bad_strict_digest_manifest" "$bad_strict_digest_artifact_dir/artifact-strict-verifier-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["targetEvidenceSha256"] = "c" * 64
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-strict-verifier-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-strict-verifier-target-evidence-digest-mismatch" "$bad_strict_digest_manifest" "$bad_strict_digest_artifact_dir" "artifacts[0] body targetEvidenceSha256 must match strictVerifier.targetEvidenceSha256"

cat >"$secret_file" <<'YAML'
apiVersion: v1
kind: Secret
metadata:
  name: oblivious-runtime
stringData:
  SESSION_SECRET: fixture-session-secret
YAML

bad_artifact_dir="$tmpdir/artifacts-bad-marketplace-payout"
write_artifact_bundle "$valid_manifest" "$bad_artifact_dir"
"$python_bin" - "$bad_artifact_dir/artifact-marketplace-payouts-20260616.json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = json.loads(path.read_text(encoding="utf-8"))
payload["proofs"].pop("refundChargebackHandling", None)
path.write_text(json.dumps(payload, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-marketplace-payout-missing-refund-chargeback-proof" "$valid_manifest" "$bad_artifact_dir" "artifacts[13] body proofs.refundChargebackHandling must be pass"

bad_workflow_telemetry_manifest="$tmpdir/valid-bad-workflow-telemetry-body.json"
bad_workflow_telemetry_artifact_dir="$tmpdir/artifacts-bad-workflow-telemetry"
cp "$valid_manifest" "$bad_workflow_telemetry_manifest"
write_artifact_bundle "$bad_workflow_telemetry_manifest" "$bad_workflow_telemetry_artifact_dir"
"$python_bin" - "$bad_workflow_telemetry_manifest" "$bad_workflow_telemetry_artifact_dir/artifact-workflow-telemetry-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("telemetry", {})["successRate"] = 1.0
body["telemetry"]["totalExecutions"] = 100
body["telemetry"]["successfulExecutions"] = 100
body["telemetry"]["failedExecutions"] = 0
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-workflow-telemetry-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-workflow-telemetry-success-rate-mismatch" "$bad_workflow_telemetry_manifest" "$bad_workflow_telemetry_artifact_dir" "artifacts[8] body telemetry.successRate must match workflowTelemetry.successRate"

bad_workflow_telemetry_counts_manifest="$tmpdir/valid-bad-workflow-telemetry-counts-body.json"
bad_workflow_telemetry_counts_artifact_dir="$tmpdir/artifacts-bad-workflow-telemetry-counts"
cp "$valid_manifest" "$bad_workflow_telemetry_counts_manifest"
write_artifact_bundle "$bad_workflow_telemetry_counts_manifest" "$bad_workflow_telemetry_counts_artifact_dir"
"$python_bin" - "$bad_workflow_telemetry_counts_manifest" "$bad_workflow_telemetry_counts_artifact_dir/artifact-workflow-telemetry-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("telemetry", {})["totalExecutions"] = 101
body["telemetry"]["successfulExecutions"] = 100
body["telemetry"]["failedExecutions"] = 1
body["telemetry"]["successRate"] = 0.99
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-workflow-telemetry-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-workflow-telemetry-count-mismatch" "$bad_workflow_telemetry_counts_manifest" "$bad_workflow_telemetry_counts_artifact_dir" "artifacts[8] body telemetry.totalExecutions must match workflowTelemetry.totalExecutions"

bad_request_log_manifest="$tmpdir/valid-bad-request-log-body.json"
bad_request_log_artifact_dir="$tmpdir/artifacts-bad-request-log-observability"
cp "$valid_manifest" "$bad_request_log_manifest"
write_artifact_bundle "$bad_request_log_manifest" "$bad_request_log_artifact_dir"
"$python_bin" - "$bad_request_log_manifest" "$bad_request_log_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("requestUsageJoin", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-missing-usage-join-proof" "$bad_request_log_manifest" "$bad_request_log_artifact_dir" "artifacts[9] body proofs.requestUsageJoin must be pass"

bad_request_log_platform_body_manifest="$tmpdir/valid-bad-request-log-platform-body.json"
bad_request_log_platform_body_artifact_dir="$tmpdir/artifacts-bad-request-log-platform"
cp "$valid_manifest" "$bad_request_log_platform_body_manifest"
write_artifact_bundle "$bad_request_log_platform_body_manifest" "$bad_request_log_platform_body_artifact_dir"
"$python_bin" - "$bad_request_log_platform_body_manifest" "$bad_request_log_platform_body_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {})["clickHouseMigration"] = "fail"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-failed-clickhouse-platform-proof" "$bad_request_log_platform_body_manifest" "$bad_request_log_platform_body_artifact_dir" "artifacts[9] body proofs.clickHouseMigration must be pass"

bad_request_log_coverage_manifest="$tmpdir/valid-bad-request-log-coverage-body.json"
bad_request_log_coverage_artifact_dir="$tmpdir/artifacts-bad-request-log-coverage"
cp "$valid_manifest" "$bad_request_log_coverage_manifest"
write_artifact_bundle "$bad_request_log_coverage_manifest" "$bad_request_log_coverage_artifact_dir"
"$python_bin" - "$bad_request_log_coverage_manifest" "$bad_request_log_coverage_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("coverage", {})["missingRequestLogRecords"] = 1
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-missing-log-coverage" "$bad_request_log_coverage_manifest" "$bad_request_log_coverage_artifact_dir" "artifacts[9] body coverage.missingRequestLogRecords must equal 0"

bad_request_log_slo_manifest="$tmpdir/valid-bad-request-log-slo-body.json"
bad_request_log_slo_artifact_dir="$tmpdir/artifacts-bad-request-log-slo"
cp "$valid_manifest" "$bad_request_log_slo_manifest"
write_artifact_bundle "$bad_request_log_slo_manifest" "$bad_request_log_slo_artifact_dir"
"$python_bin" - "$bad_request_log_slo_manifest" "$bad_request_log_slo_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("slo", {}).setdefault("alertDelivery", {})["failedDeliveries"] = 1
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-failed-slo-delivery" "$bad_request_log_slo_manifest" "$bad_request_log_slo_artifact_dir" "artifacts[9] body slo.alertDelivery.failedDeliveries must equal 0"

bad_request_log_platform_source_manifest="$tmpdir/valid-bad-request-log-platform-source-body.json"
bad_request_log_platform_source_artifact_dir="$tmpdir/artifacts-bad-request-log-platform-source"
cp "$valid_manifest" "$bad_request_log_platform_source_manifest"
write_artifact_bundle "$bad_request_log_platform_source_manifest" "$bad_request_log_platform_source_artifact_dir"
"$python_bin" - "$bad_request_log_platform_source_manifest" "$bad_request_log_platform_source_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.pop("platformProofSource", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-missing-platform-proof-source" "$bad_request_log_platform_source_manifest" "$bad_request_log_platform_source_artifact_dir" "artifacts[9] body platformProofSource is required"

bad_request_log_platform_host_manifest="$tmpdir/valid-bad-request-log-platform-host-body.json"
bad_request_log_platform_host_artifact_dir="$tmpdir/artifacts-bad-request-log-platform-host"
cp "$valid_manifest" "$bad_request_log_platform_host_manifest"
write_artifact_bundle "$bad_request_log_platform_host_manifest" "$bad_request_log_platform_host_artifact_dir"
"$python_bin" - "$bad_request_log_platform_host_manifest" "$bad_request_log_platform_host_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("platformProofSource", {})["url"] = "https://other-target.oblivious.internal/release-evidence/artifact-request-log-observability-20260616-platform-proof.json"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-platform-source-host-mismatch" "$bad_request_log_platform_host_manifest" "$bad_request_log_platform_host_artifact_dir" "artifacts[9] body platformProofSource.url host must match environment.baseUrl host"

bad_request_log_slo_source_manifest="$tmpdir/valid-bad-request-log-slo-source-body.json"
bad_request_log_slo_source_artifact_dir="$tmpdir/artifacts-bad-request-log-slo-source"
cp "$valid_manifest" "$bad_request_log_slo_source_manifest"
write_artifact_bundle "$bad_request_log_slo_source_manifest" "$bad_request_log_slo_source_artifact_dir"
"$python_bin" - "$bad_request_log_slo_source_manifest" "$bad_request_log_slo_source_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.pop("sloProofSource", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-missing-slo-proof-source" "$bad_request_log_slo_source_manifest" "$bad_request_log_slo_source_artifact_dir" "artifacts[9] body sloProofSource is required"

bad_request_log_slo_host_manifest="$tmpdir/valid-bad-request-log-slo-host-body.json"
bad_request_log_slo_host_artifact_dir="$tmpdir/artifacts-bad-request-log-slo-host"
cp "$valid_manifest" "$bad_request_log_slo_host_manifest"
write_artifact_bundle "$bad_request_log_slo_host_manifest" "$bad_request_log_slo_host_artifact_dir"
"$python_bin" - "$bad_request_log_slo_host_manifest" "$bad_request_log_slo_host_artifact_dir/artifact-request-log-observability-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("sloProofSource", {})["url"] = "https://other-target.oblivious.internal/release-evidence/artifact-request-log-observability-20260616-latency-slo-proof.json"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-request-log-observability-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-request-log-slo-source-host-mismatch" "$bad_request_log_slo_host_manifest" "$bad_request_log_slo_host_artifact_dir" "artifacts[9] body sloProofSource.url host must match environment.baseUrl host"

bad_rag_manifest="$tmpdir/valid-bad-rag-body.json"
bad_rag_artifact_dir="$tmpdir/artifacts-bad-rag-indexing"
cp "$valid_manifest" "$bad_rag_manifest"
write_artifact_bundle "$bad_rag_manifest" "$bad_rag_artifact_dir"
"$python_bin" - "$bad_rag_manifest" "$bad_rag_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
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
expect_artifact_bundle_failure "artifact-bundle-rag-missing-raw-parser-replay-proof" "$bad_rag_manifest" "$bad_rag_artifact_dir" "artifacts[10] body proofs.rawParserReplay must be pass"

bad_rag_summary_manifest="$tmpdir/valid-bad-rag-summary-body.json"
bad_rag_summary_artifact_dir="$tmpdir/artifacts-bad-rag-indexing-summary"
cp "$valid_manifest" "$bad_rag_summary_manifest"
write_artifact_bundle "$bad_rag_summary_manifest" "$bad_rag_summary_artifact_dir"
"$python_bin" - "$bad_rag_summary_manifest" "$bad_rag_summary_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["drainedJobs"] = 1
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-incomplete-drain-summary" "$bad_rag_summary_manifest" "$bad_rag_summary_artifact_dir" "artifacts[10] body summary.drainedJobs must equal summary.queuedJobs"

bad_rag_worker_summary_manifest="$tmpdir/valid-bad-rag-worker-summary-body.json"
bad_rag_worker_summary_artifact_dir="$tmpdir/artifacts-bad-rag-worker-summary"
cp "$valid_manifest" "$bad_rag_worker_summary_manifest"
write_artifact_bundle "$bad_rag_worker_summary_manifest" "$bad_rag_worker_summary_artifact_dir"
"$python_bin" - "$bad_rag_worker_summary_manifest" "$bad_rag_worker_summary_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["workerCompletedJobs"] = 0
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-missing-worker-completion-summary" "$bad_rag_worker_summary_manifest" "$bad_rag_worker_summary_artifact_dir" "artifacts[10] body summary.workerCompletedJobs must be greater than zero"

bad_rag_raw_parser_count_manifest="$tmpdir/valid-bad-rag-raw-parser-count-body.json"
bad_rag_raw_parser_count_artifact_dir="$tmpdir/artifacts-bad-rag-raw-parser-count"
cp "$valid_manifest" "$bad_rag_raw_parser_count_manifest"
write_artifact_bundle "$bad_rag_raw_parser_count_manifest" "$bad_rag_raw_parser_count_artifact_dir"
"$python_bin" - "$bad_rag_raw_parser_count_manifest" "$bad_rag_raw_parser_count_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["rawParserReplayCount"] = 0
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-missing-raw-parser-count-summary" "$bad_rag_raw_parser_count_manifest" "$bad_rag_raw_parser_count_artifact_dir" "artifacts[10] body summary.rawParserReplayCount must be greater than zero"

bad_rag_worker_undercount_manifest="$tmpdir/valid-bad-rag-worker-undercount-body.json"
bad_rag_worker_undercount_artifact_dir="$tmpdir/artifacts-bad-rag-worker-undercount"
cp "$valid_manifest" "$bad_rag_worker_undercount_manifest"
write_artifact_bundle "$bad_rag_worker_undercount_manifest" "$bad_rag_worker_undercount_artifact_dir"
"$python_bin" - "$bad_rag_worker_undercount_manifest" "$bad_rag_worker_undercount_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
summary = body.setdefault("summary", {})
summary["queuedJobs"] = 3
summary["drainedJobs"] = 3
summary["workerCompletedJobs"] = 1
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-worker-completion-undercount" "$bad_rag_worker_undercount_manifest" "$bad_rag_worker_undercount_artifact_dir" "artifacts[10] body summary.workerCompletedJobs must equal summary.drainedJobs"

bad_collection_source_manifest="$tmpdir/valid-bad-collection-source-body.json"
bad_collection_source_artifact_dir="$tmpdir/artifacts-bad-collection-source"
cp "$valid_manifest" "$bad_collection_source_manifest"
write_artifact_bundle "$bad_collection_source_manifest" "$bad_collection_source_artifact_dir"
"$python_bin" - "$bad_collection_source_manifest" "$bad_collection_source_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.pop("collectionSource", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-missing-collection-source" "$bad_collection_source_manifest" "$bad_collection_source_artifact_dir" "artifacts[10] body collectionSource is required"

bad_collection_source_url_manifest="$tmpdir/valid-bad-collection-source-url-body.json"
bad_collection_source_url_artifact_dir="$tmpdir/artifacts-bad-collection-source-url"
cp "$valid_manifest" "$bad_collection_source_url_manifest"
write_artifact_bundle "$bad_collection_source_url_manifest" "$bad_collection_source_url_artifact_dir"
"$python_bin" - "$bad_collection_source_url_manifest" "$bad_collection_source_url_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "https://target.oblivious.internal/release-evidence/rag.json?token=secret"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-secret-collection-source-url" "$bad_collection_source_url_manifest" "$bad_collection_source_url_artifact_dir" "artifacts[10] body collectionSource.url must not embed secret-like query or fragment parameters"

bad_collection_source_host_manifest="$tmpdir/valid-bad-collection-source-host-body.json"
bad_collection_source_host_artifact_dir="$tmpdir/artifacts-bad-collection-source-host"
cp "$valid_manifest" "$bad_collection_source_host_manifest"
write_artifact_bundle "$bad_collection_source_host_manifest" "$bad_collection_source_host_artifact_dir"
"$python_bin" - "$bad_collection_source_host_manifest" "$bad_collection_source_host_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "https://other-target.oblivious.internal/release-evidence/artifact-rag-indexing-20260616.json"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-wrong-target-source-host" "$bad_collection_source_host_manifest" "$bad_collection_source_host_artifact_dir" "artifacts[10] body collectionSource.url host must match environment.baseUrl host"

bad_file_collection_source_manifest="$tmpdir/valid-bad-file-collection-source-body.json"
bad_file_collection_source_artifact_dir="$tmpdir/artifacts-bad-file-collection-source"
cp "$valid_manifest" "$bad_file_collection_source_manifest"
write_artifact_bundle "$bad_file_collection_source_manifest" "$bad_file_collection_source_artifact_dir"
"$python_bin" - "$bad_file_collection_source_manifest" "$bad_file_collection_source_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["collectionSource"] = {"type": "file", "collectedAt": "2026-06-16T01:00:00Z"}
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-file-collection-source" "$bad_file_collection_source_manifest" "$bad_file_collection_source_artifact_dir" "artifacts[10] body collectionSource.type must be target-url or target-api for final target evidence"

bad_local_collection_source_manifest="$tmpdir/valid-bad-local-collection-source-body.json"
bad_local_collection_source_artifact_dir="$tmpdir/artifacts-bad-local-collection-source"
cp "$valid_manifest" "$bad_local_collection_source_manifest"
write_artifact_bundle "$bad_local_collection_source_manifest" "$bad_local_collection_source_artifact_dir"
"$python_bin" - "$bad_local_collection_source_manifest" "$bad_local_collection_source_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "http://127.0.0.1:8080/internal/release/rag-indexing-proof.json"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-local-collection-source-url" "$bad_local_collection_source_manifest" "$bad_local_collection_source_artifact_dir" "artifacts[10] body collectionSource.url must target a non-local evidence endpoint"

bad_insecure_collection_source_manifest="$tmpdir/valid-bad-insecure-collection-source-body.json"
bad_insecure_collection_source_artifact_dir="$tmpdir/artifacts-bad-insecure-collection-source"
cp "$valid_manifest" "$bad_insecure_collection_source_manifest"
write_artifact_bundle "$bad_insecure_collection_source_manifest" "$bad_insecure_collection_source_artifact_dir"
"$python_bin" - "$bad_insecure_collection_source_manifest" "$bad_insecure_collection_source_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "http://release.oblivious.internal/internal/release/rag-indexing-proof.json"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-insecure-collection-source-url" "$bad_insecure_collection_source_manifest" "$bad_insecure_collection_source_artifact_dir" "artifacts[10] body collectionSource.url must use HTTPS for final target evidence"

bad_collection_source_time_manifest="$tmpdir/valid-bad-collection-source-time-body.json"
bad_collection_source_time_artifact_dir="$tmpdir/artifacts-bad-collection-source-time"
cp "$valid_manifest" "$bad_collection_source_time_manifest"
write_artifact_bundle "$bad_collection_source_time_manifest" "$bad_collection_source_time_artifact_dir"
"$python_bin" - "$bad_collection_source_time_manifest" "$bad_collection_source_time_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["collectedAt"] = "2026-06-16T00:59:59Z"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-collection-source-before-recorded-at" "$bad_collection_source_time_manifest" "$bad_collection_source_time_artifact_dir" "artifacts[10] body collectionSource.collectedAt must be at or after body recordedAt"

bad_collection_source_path_manifest="$tmpdir/valid-bad-collection-source-path-body.json"
bad_collection_source_path_artifact_dir="$tmpdir/artifacts-bad-collection-source-path"
cp "$valid_manifest" "$bad_collection_source_path_manifest"
write_artifact_bundle "$bad_collection_source_path_manifest" "$bad_collection_source_path_artifact_dir"
"$python_bin" - "$bad_collection_source_path_manifest" "$bad_collection_source_path_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "https://target.oblivious.internal/release-evidence/artifact-rag-indexing-20260616.json?from=2026-06-16T00%3A00%3A00Z&to=2026-06-16T01%3A00%3A00Z"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-collection-source-path-mismatch" "$bad_collection_source_path_manifest" "$bad_collection_source_path_artifact_dir" "artifacts[10] body collectionSource.url path must be /api/v1/admin/release-evidence/rag-indexing"

bad_collection_source_window_manifest="$tmpdir/valid-bad-collection-source-window-body.json"
bad_collection_source_window_artifact_dir="$tmpdir/artifacts-bad-collection-source-window"
cp "$valid_manifest" "$bad_collection_source_window_manifest"
write_artifact_bundle "$bad_collection_source_window_manifest" "$bad_collection_source_window_artifact_dir"
"$python_bin" - "$bad_collection_source_window_manifest" "$bad_collection_source_window_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "https://target.oblivious.internal/api/v1/admin/release-evidence/rag-indexing"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-collection-source-missing-window" "$bad_collection_source_window_manifest" "$bad_collection_source_window_artifact_dir" "artifacts[10] body collectionSource.url must include from and to release window query parameters"

bad_collection_source_family_manifest="$tmpdir/valid-bad-collection-source-family-body.json"
bad_collection_source_family_artifact_dir="$tmpdir/artifacts-bad-collection-source-family"
cp "$valid_manifest" "$bad_collection_source_family_manifest"
write_artifact_bundle "$bad_collection_source_family_manifest" "$bad_collection_source_family_artifact_dir"
"$python_bin" - "$bad_collection_source_family_manifest" "$bad_collection_source_family_artifact_dir/artifact-rag-indexing-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("collectionSource", {})["url"] = "https://target.oblivious.internal/release-evidence/artifact-marketplace-payouts-20260616.json"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-rag-indexing-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-rag-collection-source-url-artifact-id-mismatch" "$bad_collection_source_family_manifest" "$bad_collection_source_family_artifact_dir" "artifacts[10] body collectionSource.url must identify the same artifact or proof family"

bad_realtime_manifest="$tmpdir/valid-bad-relay-realtime-body.json"
bad_realtime_artifact_dir="$tmpdir/artifacts-bad-relay-realtime"
cp "$valid_manifest" "$bad_realtime_manifest"
write_artifact_bundle "$bad_realtime_manifest" "$bad_realtime_artifact_dir"
"$python_bin" - "$bad_realtime_manifest" "$bad_realtime_artifact_dir/artifact-relay-realtime-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("authPolicy", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-realtime-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-relay-realtime-missing-auth-policy-proof" "$bad_realtime_manifest" "$bad_realtime_artifact_dir" "artifacts[11] body proofs.authPolicy must be pass"

bad_realtime_summary_manifest="$tmpdir/valid-bad-relay-realtime-summary-body.json"
bad_realtime_summary_artifact_dir="$tmpdir/artifacts-bad-relay-realtime-summary"
cp "$valid_manifest" "$bad_realtime_summary_manifest"
write_artifact_bundle "$bad_realtime_summary_manifest" "$bad_realtime_summary_artifact_dir"
"$python_bin" - "$bad_realtime_summary_manifest" "$bad_realtime_summary_artifact_dir/artifact-relay-realtime-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["requestLinkedUsageRecords"] = 3
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-realtime-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-relay-realtime-request-usage-mismatch" "$bad_realtime_summary_manifest" "$bad_realtime_summary_artifact_dir" "artifacts[11] body summary.requestLinkedUsageRecords must equal summary.totalRequests"

bad_realtime_mode_manifest="$tmpdir/valid-bad-relay-realtime-mode-body.json"
bad_realtime_mode_artifact_dir="$tmpdir/artifacts-bad-relay-realtime-mode"
cp "$valid_manifest" "$bad_realtime_mode_manifest"
write_artifact_bundle "$bad_realtime_mode_manifest" "$bad_realtime_mode_artifact_dir"
"$python_bin" - "$bad_realtime_mode_manifest" "$bad_realtime_mode_artifact_dir/artifact-relay-realtime-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["mode"] = "disabled_until_commercial_lifecycle"
body["proofs"] = {
    "productionPolicyDisabled": "pass",
    "authOriginPrebillAbortUsageBlockers": "pass",
}
body["summary"] = {
    "productionPolicyChecks": 1,
    "authOriginPrebillAbortUsageBlockerChecks": 5,
}
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-realtime-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-relay-realtime-mode-mismatch" "$bad_realtime_mode_manifest" "$bad_realtime_mode_artifact_dir" "artifacts[11] body mode must match relayRealtime.mode"

bad_batch_manifest="$tmpdir/valid-bad-relay-batch-body.json"
bad_batch_artifact_dir="$tmpdir/artifacts-bad-relay-batch"
cp "$valid_manifest" "$bad_batch_manifest"
write_artifact_bundle "$bad_batch_manifest" "$bad_batch_artifact_dir"
"$python_bin" - "$bad_batch_manifest" "$bad_batch_artifact_dir/artifact-relay-batch-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("prebillReservation", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-batch-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-relay-batch-missing-prebill-proof" "$bad_batch_manifest" "$bad_batch_artifact_dir" "artifacts[12] body proofs.prebillReservation must be pass"

bad_batch_summary_manifest="$tmpdir/valid-bad-relay-batch-summary-body.json"
bad_batch_summary_artifact_dir="$tmpdir/artifacts-bad-relay-batch-summary"
cp "$valid_manifest" "$bad_batch_summary_manifest"
write_artifact_bundle "$bad_batch_summary_manifest" "$bad_batch_summary_artifact_dir"
"$python_bin" - "$bad_batch_summary_manifest" "$bad_batch_summary_artifact_dir/artifact-relay-batch-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["usageAuditRecords"] = 2
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-batch-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-relay-batch-incomplete-lifecycle-summary" "$bad_batch_summary_manifest" "$bad_batch_summary_artifact_dir" "artifacts[12] body summary.usageAuditRecords must cover summary.settlementRecords plus summary.refundRecords"

bad_batch_request_log_summary_manifest="$tmpdir/valid-bad-relay-batch-request-log-summary-body.json"
bad_batch_request_log_summary_artifact_dir="$tmpdir/artifacts-bad-relay-batch-request-log-summary"
cp "$valid_manifest" "$bad_batch_request_log_summary_manifest"
write_artifact_bundle "$bad_batch_request_log_summary_manifest" "$bad_batch_request_log_summary_artifact_dir"
"$python_bin" - "$bad_batch_request_log_summary_manifest" "$bad_batch_request_log_summary_artifact_dir/artifact-relay-batch-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["requestLogAuditRecords"] = 2
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-relay-batch-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-relay-batch-incomplete-request-log-summary" "$bad_batch_request_log_summary_manifest" "$bad_batch_request_log_summary_artifact_dir" "artifacts[12] body summary.requestLogAuditRecords must cover summary.settlementRecords plus summary.refundRecords"

bad_provider_live_manifest="$tmpdir/valid-bad-provider-live-body.json"
bad_provider_live_artifact_dir="$tmpdir/artifacts-bad-provider-live"
cp "$valid_manifest" "$bad_provider_live_manifest"
write_artifact_bundle "$bad_provider_live_manifest" "$bad_provider_live_artifact_dir"
"$python_bin" - "$bad_provider_live_manifest" "$bad_provider_live_artifact_dir/artifact-provider-stripe-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("refund", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-provider-stripe-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-provider-live-missing-refund-proof" "$bad_provider_live_manifest" "$bad_provider_live_artifact_dir" "artifacts[3] body proofs.refund must be pass"

bad_provider_live_summary_manifest="$tmpdir/valid-bad-provider-live-summary-body.json"
bad_provider_live_summary_artifact_dir="$tmpdir/artifacts-bad-provider-live-summary"
cp "$valid_manifest" "$bad_provider_live_summary_manifest"
write_artifact_bundle "$bad_provider_live_summary_manifest" "$bad_provider_live_summary_artifact_dir"
"$python_bin" - "$bad_provider_live_summary_manifest" "$bad_provider_live_summary_artifact_dir/artifact-provider-stripe-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["reconciliationChecks"] = 0
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-provider-stripe-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-provider-live-incomplete-reconciliation-summary" "$bad_provider_live_summary_manifest" "$bad_provider_live_summary_artifact_dir" "artifacts[3] body summary.reconciliationChecks must be greater than zero"

bad_provider_live_references_manifest="$tmpdir/valid-bad-provider-live-references-body.json"
bad_provider_live_references_artifact_dir="$tmpdir/artifacts-bad-provider-live-references"
cp "$valid_manifest" "$bad_provider_live_references_manifest"
write_artifact_bundle "$bad_provider_live_references_manifest" "$bad_provider_live_references_artifact_dir"
"$python_bin" - "$bad_provider_live_references_manifest" "$bad_provider_live_references_artifact_dir/artifact-provider-stripe-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("references", {}).pop("reconciliation", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-provider-stripe-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-provider-live-missing-reference" "$bad_provider_live_references_manifest" "$bad_provider_live_references_artifact_dir" "artifacts[3] body references.reconciliation is required"

bad_deployment_environment_manifest="$tmpdir/valid-bad-deployment-environment-body.json"
bad_deployment_environment_artifact_dir="$tmpdir/artifacts-bad-deployment-environment"
cp "$valid_manifest" "$bad_deployment_environment_manifest"
mutate_json "$bad_deployment_environment_manifest" "artifact-bundle-deployment-not-production"
write_artifact_bundle "$bad_deployment_environment_manifest" "$bad_deployment_environment_artifact_dir"
expect_artifact_bundle_failure "artifact-bundle-deployment-not-production" "$bad_deployment_environment_manifest" "$bad_deployment_environment_artifact_dir" "artifacts[1] body targetEnvironment must be production"

bad_deployment_references_manifest="$tmpdir/valid-bad-deployment-references-body.json"
bad_deployment_references_artifact_dir="$tmpdir/artifacts-bad-deployment-references"
cp "$valid_manifest" "$bad_deployment_references_manifest"
mutate_json "$bad_deployment_references_manifest" "artifact-bundle-deployment-missing-reference"
write_artifact_bundle "$bad_deployment_references_manifest" "$bad_deployment_references_artifact_dir"
expect_artifact_bundle_failure "artifact-bundle-deployment-missing-reference" "$bad_deployment_references_manifest" "$bad_deployment_references_artifact_dir" "artifacts[1] body references.backupRestore is required"

bad_kubernetes_cluster_manifest="$tmpdir/valid-bad-kubernetes-cluster-body.json"
bad_kubernetes_cluster_artifact_dir="$tmpdir/artifacts-bad-kubernetes-cluster"
cp "$valid_manifest" "$bad_kubernetes_cluster_manifest"
mutate_json "$bad_kubernetes_cluster_manifest" "artifact-bundle-kubernetes-missing-cluster-ref"
write_artifact_bundle "$bad_kubernetes_cluster_manifest" "$bad_kubernetes_cluster_artifact_dir"
expect_artifact_bundle_failure "artifact-bundle-kubernetes-missing-cluster-ref" "$bad_kubernetes_cluster_manifest" "$bad_kubernetes_cluster_artifact_dir" "artifacts[2] body clusterRef is required"

bad_kubernetes_references_manifest="$tmpdir/valid-bad-kubernetes-references-body.json"
bad_kubernetes_references_artifact_dir="$tmpdir/artifacts-bad-kubernetes-references"
cp "$valid_manifest" "$bad_kubernetes_references_manifest"
mutate_json "$bad_kubernetes_references_manifest" "artifact-bundle-kubernetes-missing-reference"
write_artifact_bundle "$bad_kubernetes_references_manifest" "$bad_kubernetes_references_artifact_dir"
expect_artifact_bundle_failure "artifact-bundle-kubernetes-missing-reference" "$bad_kubernetes_references_manifest" "$bad_kubernetes_references_artifact_dir" "artifacts[2] body references.failover is required"

bad_grpc_status_manifest="$tmpdir/valid-bad-grpc-smoke-status-body.json"
bad_grpc_status_artifact_dir="$tmpdir/artifacts-bad-grpc-smoke-status"
cp "$valid_manifest" "$bad_grpc_status_manifest"
write_artifact_bundle "$bad_grpc_status_manifest" "$bad_grpc_status_artifact_dir"
"$python_bin" - "$bad_grpc_status_manifest" "$bad_grpc_status_artifact_dir/artifact-grpc-smoke-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["results"][1]["status"] = "validation_error"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-grpc-smoke-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-grpc-smoke-body-status-mismatch" "$bad_grpc_status_manifest" "$bad_grpc_status_artifact_dir" "artifacts[6] body results[1].status for workflow must be validation_response"

bad_grpc_address_manifest="$tmpdir/valid-bad-grpc-smoke-address-body.json"
bad_grpc_address_artifact_dir="$tmpdir/artifacts-bad-grpc-smoke-address"
cp "$valid_manifest" "$bad_grpc_address_manifest"
write_artifact_bundle "$bad_grpc_address_manifest" "$bad_grpc_address_artifact_dir"
"$python_bin" - "$bad_grpc_address_manifest" "$bad_grpc_address_artifact_dir/artifact-grpc-smoke-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["results"][2]["address"] = "task.prod.oblivious.release.test:50064"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-grpc-smoke-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-grpc-smoke-body-address-mismatch" "$bad_grpc_address_manifest" "$bad_grpc_address_artifact_dir" "artifacts[6] body results[2].address for task must target port 50065"

bad_secret_body_manifest="$tmpdir/valid-bad-secret-audit-body.json"
bad_secret_body_artifact_dir="$tmpdir/artifacts-bad-secret-audit"
cp "$valid_manifest" "$bad_secret_body_manifest"
write_artifact_bundle "$bad_secret_body_manifest" "$bad_secret_body_artifact_dir"
"$python_bin" - "$bad_secret_body_manifest" "$bad_secret_body_artifact_dir/artifact-secret-audit-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["apiToken"] = "target-secret-token"
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-secret-audit-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-secret-audit-body-embedded-token" "$bad_secret_body_manifest" "$bad_secret_body_artifact_dir" "artifacts.7.body.apiToken must not embed secret material"

bad_secret_findings_manifest="$tmpdir/valid-bad-secret-audit-findings-body.json"
bad_secret_findings_artifact_dir="$tmpdir/artifacts-bad-secret-audit-findings"
cp "$valid_manifest" "$bad_secret_findings_manifest"
write_artifact_bundle "$bad_secret_findings_manifest" "$bad_secret_findings_artifact_dir"
"$python_bin" - "$bad_secret_findings_manifest" "$bad_secret_findings_artifact_dir/artifact-secret-audit-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["findings"] = [{"scope": "providers", "severity": "high"}]
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-secret-audit-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-secret-audit-body-findings" "$bad_secret_findings_manifest" "$bad_secret_findings_artifact_dir" "artifacts[7] body findings must be an empty array"

bad_payout_summary_manifest="$tmpdir/valid-bad-marketplace-payout-summary-body.json"
bad_payout_summary_artifact_dir="$tmpdir/artifacts-bad-marketplace-payout-summary"
cp "$valid_manifest" "$bad_payout_summary_manifest"
write_artifact_bundle "$bad_payout_summary_manifest" "$bad_payout_summary_artifact_dir"
"$python_bin" - "$bad_payout_summary_manifest" "$bad_payout_summary_artifact_dir/artifact-marketplace-payouts-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["refundChargebackCasesHandled"] = 0
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-marketplace-payouts-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-marketplace-payout-incomplete-refund-summary" "$bad_payout_summary_manifest" "$bad_payout_summary_artifact_dir" "artifacts[13] body summary.refundChargebackCasesHandled must equal summary.refundChargebackCases"

bad_payout_zero_refund_manifest="$tmpdir/valid-bad-marketplace-payout-zero-refund-body.json"
bad_payout_zero_refund_artifact_dir="$tmpdir/artifacts-bad-marketplace-payout-zero-refund"
cp "$valid_manifest" "$bad_payout_zero_refund_manifest"
write_artifact_bundle "$bad_payout_zero_refund_manifest" "$bad_payout_zero_refund_artifact_dir"
"$python_bin" - "$bad_payout_zero_refund_manifest" "$bad_payout_zero_refund_artifact_dir/artifact-marketplace-payouts-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
summary = body.setdefault("summary", {})
summary["refundChargebackCases"] = 0
summary["refundChargebackCasesHandled"] = 0
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-marketplace-payouts-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-marketplace-payout-zero-refund-chargeback-cases" "$bad_payout_zero_refund_manifest" "$bad_payout_zero_refund_artifact_dir" "artifacts[13] body summary.refundChargebackCases must be greater than zero"

bad_governance_summary_manifest="$tmpdir/valid-bad-marketplace-governance-summary-body.json"
bad_governance_summary_artifact_dir="$tmpdir/artifacts-bad-marketplace-governance-summary"
cp "$valid_manifest" "$bad_governance_summary_manifest"
write_artifact_bundle "$bad_governance_summary_manifest" "$bad_governance_summary_artifact_dir"
"$python_bin" - "$bad_governance_summary_manifest" "$bad_governance_summary_artifact_dir/artifact-marketplace-governance-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["appealDecisions"] = 1
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-marketplace-governance-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-marketplace-governance-incomplete-appeal-summary" "$bad_governance_summary_manifest" "$bad_governance_summary_artifact_dir" "artifacts[14] body summary.appealDecisions must equal summary.appealQueueItems"

bad_provider_runtime_manifest="$tmpdir/valid-bad-provider-runtime-body.json"
bad_provider_runtime_artifact_dir="$tmpdir/artifacts-bad-provider-runtime"
cp "$valid_manifest" "$bad_provider_runtime_manifest"
write_artifact_bundle "$bad_provider_runtime_manifest" "$bad_provider_runtime_artifact_dir"
"$python_bin" - "$bad_provider_runtime_manifest" "$bad_provider_runtime_artifact_dir/artifact-provider-runtime-config-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("webhookVerification", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-provider-runtime-config-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-provider-runtime-missing-webhook-verification-proof" "$bad_provider_runtime_manifest" "$bad_provider_runtime_artifact_dir" "artifacts[15] body proofs.webhookVerification must be pass"

bad_provider_summary_manifest="$tmpdir/valid-bad-provider-runtime-summary-body.json"
bad_provider_summary_artifact_dir="$tmpdir/artifacts-bad-provider-runtime-summary"
cp "$valid_manifest" "$bad_provider_summary_manifest"
write_artifact_bundle "$bad_provider_summary_manifest" "$bad_provider_summary_artifact_dir"
"$python_bin" - "$bad_provider_summary_manifest" "$bad_provider_summary_artifact_dir/artifact-provider-runtime-config-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["checkoutBaseUrlsChecked"] = 2
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-provider-runtime-config-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-provider-runtime-incomplete-checkout-summary" "$bad_provider_summary_manifest" "$bad_provider_summary_artifact_dir" "artifacts[15] body summary.checkoutBaseUrlsChecked must cover summary.providersConfigured"

bad_provider_details_manifest="$tmpdir/valid-bad-provider-runtime-details-body.json"
bad_provider_details_artifact_dir="$tmpdir/artifacts-bad-provider-runtime-details"
cp "$valid_manifest" "$bad_provider_details_manifest"
mutate_json "$bad_provider_details_manifest" "provider-runtime-config-artifact-missing-wechatpay-detail"
write_artifact_bundle "$bad_provider_details_manifest" "$bad_provider_details_artifact_dir"
expect_artifact_bundle_failure "artifact-bundle-provider-runtime-missing-wechatpay-detail" "$bad_provider_details_manifest" "$bad_provider_details_artifact_dir" "artifacts[15] body providers must include stripe, alipay, and wechatpay (missing: wechatpay)"

bad_microservice_db_details_manifest="$tmpdir/valid-bad-microservice-db-details-body.json"
bad_microservice_db_details_artifact_dir="$tmpdir/artifacts-bad-microservice-db-details"
cp "$valid_manifest" "$bad_microservice_db_details_manifest"
mutate_json "$bad_microservice_db_details_manifest" "microservice-database-artifact-missing-observability-detail"
write_artifact_bundle "$bad_microservice_db_details_manifest" "$bad_microservice_db_details_artifact_dir"
expect_artifact_bundle_failure "artifact-bundle-microservice-db-missing-observability-detail" "$bad_microservice_db_details_manifest" "$bad_microservice_db_details_artifact_dir" "artifacts[16] body services must include relay, chat, workflow, rag, agent, billing, marketplace, admin, channel, task, and observability (missing: observability)"

bad_microservice_db_manifest="$tmpdir/valid-bad-microservice-db-body.json"
bad_microservice_db_artifact_dir="$tmpdir/artifacts-bad-microservice-db"
cp "$valid_manifest" "$bad_microservice_db_manifest"
write_artifact_bundle "$bad_microservice_db_manifest" "$bad_microservice_db_artifact_dir"
"$python_bin" - "$bad_microservice_db_manifest" "$bad_microservice_db_artifact_dir/artifact-microservice-databases-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("proofs", {}).pop("migrationReadiness", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-microservice-databases-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-microservice-db-missing-migration-readiness-proof" "$bad_microservice_db_manifest" "$bad_microservice_db_artifact_dir" "artifacts[16] body proofs.migrationReadiness must be pass"

bad_microservice_db_summary_manifest="$tmpdir/valid-bad-microservice-db-summary-body.json"
bad_microservice_db_summary_artifact_dir="$tmpdir/artifacts-bad-microservice-db-summary"
cp "$valid_manifest" "$bad_microservice_db_summary_manifest"
write_artifact_bundle "$bad_microservice_db_summary_manifest" "$bad_microservice_db_summary_artifact_dir"
"$python_bin" - "$bad_microservice_db_summary_manifest" "$bad_microservice_db_summary_artifact_dir/artifact-microservice-databases-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body.setdefault("summary", {})["migrationReadinessChecks"] = 10
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-microservice-databases-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-microservice-db-incomplete-migration-readiness-summary" "$bad_microservice_db_summary_manifest" "$bad_microservice_db_summary_artifact_dir" "artifacts[16] body summary.migrationReadinessChecks must equal 11"

bad_microservice_db_mode_manifest="$tmpdir/valid-bad-microservice-db-mode-body.json"
bad_microservice_db_mode_artifact_dir="$tmpdir/artifacts-bad-microservice-db-mode"
cp "$valid_manifest" "$bad_microservice_db_mode_manifest"
write_artifact_bundle "$bad_microservice_db_mode_manifest" "$bad_microservice_db_mode_artifact_dir"
"$python_bin" - "$bad_microservice_db_mode_manifest" "$bad_microservice_db_mode_artifact_dir/artifact-microservice-databases-20260616.json" <<'PY'
import hashlib
import json
import pathlib
import sys

manifest_path = pathlib.Path(sys.argv[1])
body_path = pathlib.Path(sys.argv[2])
body = json.loads(body_path.read_text(encoding="utf-8"))
body["mode"] = "monolith"
body.pop("serviceUrlClass", None)
body_bytes = (json.dumps(body, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
body_path.write_bytes(body_bytes)
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
for artifact in manifest["artifacts"]:
    if artifact["id"] == "artifact-microservice-databases-20260616":
        artifact["sha256"] = hashlib.sha256(body_bytes).hexdigest()
        break
manifest_path.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
expect_artifact_bundle_failure "artifact-bundle-microservice-db-body-mode-mismatch" "$bad_microservice_db_mode_manifest" "$bad_microservice_db_mode_artifact_dir" "artifacts[16] body mode must be microservices"

make_invalid_case "missing-artifacts-index" "artifacts must include at least one target artifact entry"
make_invalid_case "missing-run-lineage" "runId is required"
make_invalid_case "dangling-strict-verifier-evidence-ref" "strictVerifier.evidenceRef must reference an artifact id listed in artifacts"
make_invalid_case "mismatched-strict-verifier-artifact-kind" "strictVerifier.evidenceRef must reference artifact kind strict-verifier-log"
make_invalid_case "mismatched-artifact-commit" "artifacts[1].commit must match manifest commit"
make_invalid_case "mismatched-artifact-run-id" "artifacts[1].runId must match manifest runId"
make_invalid_case "invalid-environment-class" "environment.class must be staging, preproduction, or production"
make_invalid_case "non-production-target-final" "environment.class must be production for final target evidence"
make_invalid_case "placeholder-environment-base-url" "environment.baseUrl must reference a concrete target environment value, not a placeholder"
make_invalid_case "invalid-environment-base-url" "environment.baseUrl must be an HTTP(S) URL"
make_invalid_case "loopback-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "abbreviated-loopback-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "octal-loopback-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "hex-loopback-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "fake-environment-base-url" "environment.baseUrl must reference a concrete target environment value, not a placeholder"
make_invalid_case "reserved-invalid-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "loopback-environment-trailing-dot" "environment.baseUrl must target a non-local target environment"
make_invalid_case "ipv6-mapped-loopback-environment" "environment.baseUrl must target a non-local target environment"
make_invalid_case "ipv6-unspecified-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "zero-host-environment-base-url" "environment.baseUrl must target a non-local target environment"
make_invalid_case "credential-environment-base-url-userinfo" "environment.baseUrl must not embed credentials in URI userinfo"
make_invalid_case "secret-environment-base-url-query" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "secret-name-only-environment-base-url-query" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "password-environment-base-url-query" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "encoded-password-environment-base-url-query" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "double-encoded-password-environment-base-url-query" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "secret-value-environment-base-url-query" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "secret-environment-base-url-fragment" "environment.baseUrl must not embed secret-like query or fragment parameters"
make_invalid_case "duplicate-artifact-id" "artifacts must not duplicate artifact-strict-verifier-20260616"
make_invalid_case "placeholder-artifact-id" "artifacts[0].id must reference a concrete target artifact, not a placeholder"
make_invalid_case "placeholder-artifact-kind" "artifacts[0].kind must describe a concrete target artifact, not a placeholder"
make_invalid_case "placeholder-artifact-uri" "artifacts[1].uri must reference a concrete target artifact, not a placeholder"
make_invalid_case "secret-artifact-uri-query" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "password-artifact-uri-query" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "encoded-token-artifact-uri-query" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "double-encoded-token-artifact-uri-query" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "secret-value-artifact-uri-query" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "secret-artifact-uri-fragment" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "secret-name-only-artifact-uri-fragment" "artifacts[1].uri must not embed secret-like query or fragment parameters"
make_invalid_case "credential-artifact-uri-userinfo" "artifacts[1].uri must not embed credentials in URI userinfo"
make_invalid_case "secret-audit-embedded-token" "secretAudit.apiToken must not embed secret material"
make_invalid_case "incomplete-secret-audit-scope" "secretAudit.scope must include kubernetes, providers, and runtime (missing: kubernetes, providers)"
make_invalid_case "local-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "file-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "loopback-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "abbreviated-loopback-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "octal-loopback-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "hex-loopback-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "ipv6-mapped-loopback-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "zero-host-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "fake-artifact-uri" "artifacts[1].uri must reference a concrete target artifact, not a placeholder"
make_invalid_case "reserved-invalid-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "inline-artifact-uri" "artifacts[1].uri must reference a remote target artifact URI"
make_invalid_case "invalid-artifact-recorded-at" "artifacts[0].recordedAt must be ISO-8601"
make_invalid_case "invalid-artifact-sha256" "artifacts[0].sha256 must be a 64-character hex digest"
make_invalid_case "missing-artifact-sha256" "artifacts[1].sha256 is required"
make_invalid_case "unused-artifact-id" "artifacts[17].id artifact-unused-20260616 must be referenced by a required evidenceRef"
make_invalid_case "unused-artifact-masked-by-freeform-evidence-ref" "artifacts[17].id artifact-unused-20260616 must be referenced by a required evidenceRef"
make_invalid_case "missing-providers-collection" "providers must include at least one live provider evidence entry"
make_invalid_case "missing-wechatpay-provider" "providers must include live evidence for stripe, alipay, and wechatpay (missing: wechatpay)"
make_invalid_case "unknown-provider-live-rail" "providers[3].name must be stripe, alipay, or wechatpay"
make_invalid_case "swapped-provider-evidence-ref" "providers[0].evidenceRef must reference provider-specific live evidence for stripe"
make_invalid_case "provider-live-rail-not-live-environment" "providers[0].providerEnvironment must be live"
make_invalid_case "missing-strict-k8s-flag" "strictVerifier.command must include COMMERCIAL_COMPLETION_RUN_K8S=true"
make_invalid_case "strict-command-mask-or-true" "strictVerifier.command must use the canonical strict verifier invocation"
make_invalid_case "strict-command-env-quoted-skip-flags-as-args" "strictVerifier.command must use the canonical strict verifier invocation"
make_invalid_case "quoted-env-skip-command" "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS"
make_invalid_case "ansi-c-quoted-env-skip-command" "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS"
make_invalid_case "commit-mismatch-override-command" "strictVerifier.command must not enable OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH"
make_invalid_case "ansi-c-quoted-commit-mismatch-command" "strictVerifier.command must not enable OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH"
make_invalid_case "missing-strict-target-evidence-sha" "strictVerifier.targetEvidenceSha256 is required"
make_invalid_case "invalid-strict-artifact-bundle-sha" "strictVerifier.artifactBundleSha256 must be a 64-character SHA-256 hex digest"

if OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true bash "$repo_root/scripts/verify-commercial-completion.sh" >"$tmpdir/commercial-commit-mismatch-override.out" 2>&1; then
  cat "$tmpdir/commercial-commit-mismatch-override.out" >&2
  fail "commercial-completion-commit-mismatch-override unexpectedly passed"
fi
if ! grep -Fq -- "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH cannot be true for strict final readiness" "$tmpdir/commercial-commit-mismatch-override.out"; then
  cat "$tmpdir/commercial-commit-mismatch-override.out" >&2
  fail "commercial-completion-commit-mismatch-override failed without expected pattern"
fi
echo "[target-release-evidence-fixtures] rejected commercial-completion-commit-mismatch-override"

if TEST_DATABASE_URL="postgres://oblivious:oblivious@127.0.0.1:5432/oblivious?sslmode=disable" \
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
  COMMERCIAL_COMPLETION_RUN_K8S=true \
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
  OBLIVIOUS_K8S_SECRET_FILE="$secret_file" \
  bash "$repo_root/scripts/verify-commercial-completion.sh" >"$tmpdir/commercial-missing-evidence-file.out" 2>&1; then
  cat "$tmpdir/commercial-missing-evidence-file.out" >&2
  fail "commercial-completion-missing-evidence-file unexpectedly passed"
fi
if ! grep -Fq -- "target live evidence manifest requires OBLIVIOUS_TARGET_EVIDENCE_FILE" "$tmpdir/commercial-missing-evidence-file.out"; then
  cat "$tmpdir/commercial-missing-evidence-file.out" >&2
  fail "commercial-completion-missing-evidence-file failed without expected pattern"
fi
echo "[target-release-evidence-fixtures] rejected commercial-completion-missing-evidence-file"

if TEST_DATABASE_URL="postgres://oblivious:oblivious@127.0.0.1:5432/oblivious?sslmode=disable" \
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true \
  COMMERCIAL_COMPLETION_RUN_K8S=true \
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true \
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true \
  OBLIVIOUS_K8S_SECRET_FILE="$secret_file" \
  OBLIVIOUS_TARGET_EVIDENCE_FILE="$valid_manifest" \
  bash "$repo_root/scripts/verify-commercial-completion.sh" >"$tmpdir/commercial-missing-artifact-dir.out" 2>&1; then
  cat "$tmpdir/commercial-missing-artifact-dir.out" >&2
  fail "commercial-completion-missing-artifact-dir unexpectedly passed"
fi
if ! grep -Fq -- "target live evidence manifest requires OBLIVIOUS_TARGET_ARTIFACT_DIR" "$tmpdir/commercial-missing-artifact-dir.out"; then
  cat "$tmpdir/commercial-missing-artifact-dir.out" >&2
  fail "commercial-completion-missing-artifact-dir failed without expected pattern"
fi
echo "[target-release-evidence-fixtures] rejected commercial-completion-missing-artifact-dir"

make_invalid_case "missing-strict-verifier-evidence-ref" "strictVerifier.evidenceRef is required"
make_invalid_case "inverted-strict-verifier-window" "strictVerifier.completedAt must be at or after strictVerifier.startedAt"
make_invalid_case "strict-verifier-artifact-outside-window" "strictVerifier.evidenceRef artifact recordedAt must be within strictVerifier.startedAt/completedAt"
make_invalid_case "unfilled-kubernetes-secret-file-class" "kubernetes.secretFileClass must be external-filled"
make_invalid_case "deployment-artifact-missing-backup-restore-proof" "deployment.evidenceRef artifact proofs.backupRestore must be pass"
make_invalid_case "kubernetes-artifact-missing-failover-proof" "kubernetes.evidenceRef artifact proofs.failover must be pass"
make_invalid_case "provider-live-rail-artifact-missing-payout-proof" "providers.0.evidenceRef artifact proofs.payout must be pass"
make_invalid_case "invalid-workflow-telemetry-window" "workflowTelemetry.window must be an ISO-8601 start/end interval"
make_invalid_case "inverted-workflow-telemetry-window" "workflowTelemetry.window end must be at or after start"
make_invalid_case "workflow-telemetry-success-rate-above-one" "workflowTelemetry.successRate must be between 0.99 and 1.0"
make_invalid_case "missing-workflow-telemetry-total-executions" "workflowTelemetry.totalExecutions must be greater than zero"
make_invalid_case "workflow-telemetry-count-mismatch" "workflowTelemetry.successfulExecutions plus workflowTelemetry.failedExecutions must equal workflowTelemetry.totalExecutions"
make_invalid_case "workflow-telemetry-success-rate-count-mismatch" "workflowTelemetry.successRate must equal successfulExecutions / totalExecutions"
make_invalid_case "missing-request-log-observability" "requestLogObservability is required"
make_invalid_case "request-log-observability-not-clickhouse" "requestLogObservability.backend must be clickhouse"
make_invalid_case "failed-clickhouse-migration-proof" "requestLogObservability.clickHouseMigration must be pass"
make_invalid_case "failed-request-log-usage-join-proof" "requestLogObservability.requestUsageJoin must be pass"
make_invalid_case "request-log-observability-artifact-missing-usage-join-proof" "requestLogObservability.evidenceRef artifact proofs.requestUsageJoin must be pass"
make_invalid_case "missing-latency-slo-trigger-proof" "requestLogObservability.latencySLOTrigger must be pass"
make_invalid_case "failed-latency-slo-alert-delivery-proof" "requestLogObservability.latencySLOAlertDelivery must be pass"
make_invalid_case "failed-latency-slo-recovery-action-proof" "requestLogObservability.latencySLORecoveryAction must be pass"
make_invalid_case "invalid-latency-slo-window" "requestLogObservability.latencySLOWindow must be an ISO-8601 start/end interval"
make_invalid_case "missing-latency-slo-alert-delivery" "requestLogObservability.alertDelivery is required"
make_invalid_case "failed-latency-slo-alert-delivery-count" "requestLogObservability.alertDelivery.failedDeliveries must equal 0"
make_invalid_case "missing-latency-slo-alert-channel" "requestLogObservability.alertDelivery.channels must be a non-empty array of strings"
make_invalid_case "sample-latency-slo-delivery-id" "requestLogObservability.alertDelivery.lastDeliveryId must be concrete"
make_invalid_case "fake-latency-slo-recovery-record-id" "requestLogObservability.recoveryAudit.lastRecordId must be concrete"
make_invalid_case "failed-latency-slo-recovery-audit-count" "requestLogObservability.recoveryAudit.failedActions must equal 0"
make_invalid_case "request-log-observability-kind-mismatch" "requestLogObservability.evidenceRef must reference artifact kind request-log-observability"
make_invalid_case "missing-rag-indexing-proof" "ragIndexing is required"
make_invalid_case "failed-rag-worker-deployment-proof" "ragIndexing.workerDeployment must be pass"
make_invalid_case "failed-rag-raw-parser-replay-proof" "ragIndexing.rawParserReplay must be pass"
make_invalid_case "failed-rag-stale-vector-filter-proof" "ragIndexing.staleVectorFilter must be pass"
make_invalid_case "rag-indexing-artifact-missing-raw-parser-proof" "ragIndexing.evidenceRef artifact proofs.rawParserReplay must be pass"
make_invalid_case "missing-relay-realtime-proof" "relayRealtime is required"
make_invalid_case "relay-realtime-enabled-without-lifecycle" "relayRealtime.mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled"
make_invalid_case "relay-realtime-disabled-final" "relayRealtime.mode must be commercial_lifecycle_enabled for final target evidence"
make_invalid_case "failed-relay-realtime-auth-policy-proof" "relayRealtime.authPolicy must be pass"
make_invalid_case "failed-relay-realtime-usage-ledger-proof" "relayRealtime.usageLedger must be pass"
make_invalid_case "relay-realtime-artifact-missing-auth-policy-proof" "relayRealtime.evidenceRef artifact proofs.authPolicy must be pass"
make_invalid_case "relay-realtime-kind-mismatch" "relayRealtime.evidenceRef must reference artifact kind relay-realtime-proof"
make_invalid_case "missing-relay-batch-proof" "relayBatch is required"
make_invalid_case "relay-batch-enabled-without-lifecycle" "relayBatch.mode must be disabled_until_commercial_lifecycle or commercial_lifecycle_enabled"
make_invalid_case "relay-batch-disabled-final" "relayBatch.mode must be commercial_lifecycle_enabled for final target evidence"
make_invalid_case "failed-relay-batch-prebill-proof" "relayBatch.prebillReservation must be pass"
make_invalid_case "failed-relay-batch-usage-audit-proof" "relayBatch.usageAudit must be pass"
make_invalid_case "relay-batch-artifact-missing-prebill-proof" "relayBatch.evidenceRef artifact proofs.prebillReservation must be pass"
make_invalid_case "relay-batch-kind-mismatch" "relayBatch.evidenceRef must reference artifact kind relay-batch-proof"
make_invalid_case "missing-marketplace-payout-proof" "marketplacePayouts is required"
make_invalid_case "marketplace-payout-provider-not-webhook" "marketplacePayouts.providerMode must be webhook"
make_invalid_case "failed-marketplace-payout-webhook-lifecycle" "marketplacePayouts.inboundWebhookLifecycle must be pass"
make_invalid_case "missing-marketplace-payout-refund-chargeback-proof" "marketplacePayouts.refundChargebackHandling must be pass"
make_invalid_case "marketplace-payout-artifact-missing-refund-chargeback-proof" "marketplacePayouts.evidenceRef artifact proofs.refundChargebackHandling must be pass"
make_invalid_case "marketplace-payout-kind-mismatch" "marketplacePayouts.evidenceRef must reference artifact kind marketplace-payout-proof"
make_invalid_case "missing-marketplace-governance-proof" "marketplaceGovernance is required"
make_invalid_case "failed-marketplace-governance-review-assignment" "marketplaceGovernance.reviewAssignment must be pass"
make_invalid_case "marketplace-governance-artifact-missing-review-sla-proof" "marketplaceGovernance.evidenceRef artifact proofs.reviewSLAEnforcement must be pass"
make_invalid_case "marketplace-governance-kind-mismatch" "marketplaceGovernance.evidenceRef must reference artifact kind marketplace-governance-proof"
make_invalid_case "missing-provider-runtime-config-proof" "providerRuntimeConfig is required"
make_invalid_case "failed-alipay-runtime-config-proof" "providerRuntimeConfig.alipay must be pass"
make_invalid_case "failed-provider-webhook-routes-proof" "providerRuntimeConfig.webhookRoutes must be pass"
make_invalid_case "provider-runtime-config-artifact-missing-webhook-verification-proof" "providerRuntimeConfig.evidenceRef artifact proofs.webhookVerification must be pass"
make_invalid_case "provider-runtime-config-kind-mismatch" "providerRuntimeConfig.evidenceRef must reference artifact kind provider-runtime-config"
make_invalid_case "missing-microservice-database-proof" "microserviceDatabases is required"
make_invalid_case "monolith-microservice-database-proof" "microserviceDatabases.mode must be microservices"
make_invalid_case "unfilled-microservice-database-url-class" "microserviceDatabases.serviceUrlClass must be external-filled"
make_invalid_case "failed-rag-microservice-database-proof" "microserviceDatabases.rag must be pass"
make_invalid_case "failed-agent-microservice-database-proof" "microserviceDatabases.agent must be pass"
make_invalid_case "microservice-database-artifact-missing-agent-proof" "microserviceDatabases.evidenceRef artifact proofs.agent must be pass"
make_invalid_case "microservice-database-kind-mismatch" "microserviceDatabases.evidenceRef must reference artifact kind microservice-database-proof"
make_invalid_case "missing-grpc-collection" "grpc must be an array"
make_invalid_case "unknown-grpc-service" "grpc[3].service must be agent, workflow, or task"
make_invalid_case "unknown-grpc-smoke-service" "grpcSmokeReport.results[3].service must be agent, workflow, or task"
make_invalid_case "loopback-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "abbreviated-loopback-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "octal-loopback-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "hex-loopback-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "ipv6-mapped-loopback-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "zero-host-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "fake-grpc-address" "grpc[0].address for agent must reference a concrete target service endpoint, not a placeholder"
make_invalid_case "reserved-invalid-grpc-address" "grpc[0].address for agent must target a non-local service endpoint"
make_invalid_case "url-shaped-grpc-address" "grpc[0].address for agent must be a plain host:port endpoint"
make_invalid_case "missing-grpc-smoke-report" "grpcSmokeReport is required"
make_invalid_case "invalid-grpc-smoke-timeout" "grpcSmokeReport.timeout must be a positive Go duration string"
make_invalid_case "failed-grpc-smoke-result" "grpcSmokeReport.results[2].generatedClient must be pass"
make_invalid_case "workflow-grpc-smoke-status-mismatch" "grpcSmokeReport.results[1].status for workflow must be validation_response"
make_invalid_case "mismatched-grpc-smoke-evidence-ref" "grpc agent evidenceRef must match grpcSmokeReport.evidenceRef"
make_invalid_case "grpc-smoke-artifact-before-report" "grpcSmokeReport.evidenceRef artifact recordedAt must be at or after grpcSmokeReport.recordedAt"
make_invalid_case "workflow-telemetry-artifact-before-window-end" "workflowTelemetry.evidenceRef artifact recordedAt must be at or after workflowTelemetry.window end"

echo "[target-release-evidence-fixtures] target release evidence verifier behavior is guarded."
