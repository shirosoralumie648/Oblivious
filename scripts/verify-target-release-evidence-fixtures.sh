#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
verifier="$repo_root/scripts/verify-target-release-evidence.sh"
tmpdir=$(mktemp -d)

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

fail() {
  echo "[target-release-evidence-fixtures] $*" >&2
  exit 1
}

mutate_json() {
  local path="$1"
  local expression="$2"

  ruby -rjson -e '
path = ARGV.fetch(0)
expression = ARGV.fetch(1)
data = JSON.parse(File.read(path))
eval(expression)
File.write(path, JSON.pretty_generate(data))
' "$path" "$expression"
}

fill_manifest() {
  local path="$1"

  mutate_json "$path" '
data["environment"]["name"] = "staging-target"
data["environment"]["class"] = "staging Kubernetes"
data["environment"]["baseUrl"] = "https://staging.oblivious.internal"
data["strictVerifier"]["evidenceRef"] = "artifact-strict-verifier-20260616"
data["strictVerifier"]["startedAt"] = "2026-06-16T00:00:00Z"
data["strictVerifier"]["completedAt"] = "2026-06-16T01:00:00Z"
data["deployment"]["evidenceRef"] = "artifact-deploy-20260616"
data["kubernetes"]["evidenceRef"] = "artifact-k8s-20260616"
data["providers"].each { |provider| provider["evidenceRef"] = "artifact-provider-#{provider.fetch("name")}-20260616" }
data["grpc"].each { |entry| entry["evidenceRef"] = "artifact-grpc-smoke-20260616" }
data["grpcSmokeReport"]["evidenceRef"] = "artifact-grpc-smoke-20260616"
data["grpcSmokeReport"]["recordedAt"] = "2026-06-16T00:00:00Z"
data["secretAudit"]["evidenceRef"] = "artifact-secret-audit-20260616"
data["workflowTelemetry"]["window"] = "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z"
data["workflowTelemetry"]["evidenceRef"] = "artifact-workflow-telemetry-20260616"
data["artifacts"] = [
  {
    "id" => "artifact-strict-verifier-20260616",
    "kind" => "strict-verifier-log",
    "uri" => "ci://target-release/20260616/strict-verifier.log",
    "recordedAt" => "2026-06-16T01:00:00Z",
    "sha256" => "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  {
    "id" => "artifact-deploy-20260616",
    "kind" => "deployment-log",
    "uri" => "ci://target-release/20260616/deploy.log",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-k8s-20260616",
    "kind" => "kubernetes-validation",
    "uri" => "ci://target-release/20260616/kubernetes.log",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-provider-stripe-20260616",
    "kind" => "provider-live-rail",
    "uri" => "provider://stripe/live/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-provider-alipay-20260616",
    "kind" => "provider-live-rail",
    "uri" => "provider://alipay/live/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-provider-wechatpay-20260616",
    "kind" => "provider-live-rail",
    "uri" => "provider://wechatpay/live/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-grpc-smoke-20260616",
    "kind" => "grpc-smoke-report",
    "uri" => "ci://target-release/20260616/grpc-smoke.json",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-secret-audit-20260616",
    "kind" => "secret-audit",
    "uri" => "ci://target-release/20260616/secret-audit.log",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-workflow-telemetry-20260616",
    "kind" => "workflow-telemetry",
    "uri" => "observability://target/workflows/success-rate/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  }
]
'
}

expect_failure() {
  local label="$1"
  local path="$2"
  local expected_pattern="$3"
  local output="$tmpdir/${label//[^A-Za-z0-9_.-]/_}.out"

  if bash "$verifier" "$path" >"$output" 2>&1; then
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
  local expression="$2"
  local expected_pattern="$3"
  local path="$tmpdir/$label.json"

  cp "$valid_manifest" "$path"
  mutate_json "$path" "$expression"
  expect_failure "$label" "$path" "$expected_pattern"
}

template_manifest="$tmpdir/template.json"
valid_manifest="$tmpdir/valid.json"

bash "$verifier" --print-template > "$template_manifest"
expect_failure "generated-template-placeholders" "$template_manifest" "environment.name must reference a concrete target environment value, not a placeholder"

cp "$template_manifest" "$valid_manifest"
fill_manifest "$valid_manifest"
bash "$verifier" "$valid_manifest" >/dev/null
echo "[target-release-evidence-fixtures] accepted filled current-commit manifest"

make_invalid_case \
  "missing-artifacts-index" \
  'data.delete("artifacts")' \
  "artifacts must include at least one target artifact entry"

make_invalid_case \
  "dangling-strict-verifier-evidence-ref" \
  'data["strictVerifier"]["evidenceRef"] = "artifact-missing-20260616"' \
  "strictVerifier.evidenceRef must reference an artifact id listed in artifacts"

make_invalid_case \
  "mismatched-strict-verifier-artifact-kind" \
  'data["strictVerifier"]["evidenceRef"] = "artifact-provider-stripe-20260616"; data["providers"].find { |provider| provider["name"] == "stripe" }["evidenceRef"] = "artifact-strict-verifier-20260616"' \
  "strictVerifier.evidenceRef must reference artifact kind strict-verifier-log"

make_invalid_case \
  "placeholder-environment-base-url" \
  'data["environment"]["baseUrl"] = "TODO-target-base-url"' \
  "environment.baseUrl must reference a concrete target environment value, not a placeholder"

make_invalid_case \
  "duplicate-artifact-id" \
  'data["artifacts"] << data["artifacts"].first.dup' \
  "artifacts must not duplicate artifact-strict-verifier-20260616"

make_invalid_case \
  "placeholder-artifact-id" \
  'data["artifacts"].first["id"] = "TODO-strict-verifier-log"' \
  "artifacts[0].id must reference a concrete target artifact, not a placeholder"

make_invalid_case \
  "placeholder-artifact-kind" \
  'data["artifacts"].first["kind"] = "TODO-log-kind"' \
  "artifacts[0].kind must describe a concrete target artifact, not a placeholder"

make_invalid_case \
  "placeholder-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "/path/outside/git/deploy.log"' \
  "artifacts[1].uri must reference a concrete target artifact, not a placeholder"

make_invalid_case \
  "secret-artifact-uri-query" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://ci.internal/runs/20260616/deploy.log?token=target-secret-token"' \
  "artifacts[1].uri must not embed secret-like query parameters"

make_invalid_case \
  "invalid-artifact-recorded-at" \
  'data["artifacts"].first["recordedAt"] = "2026/06/16 01:00"' \
  "artifacts[0].recordedAt must be ISO-8601"

make_invalid_case \
  "invalid-artifact-sha256" \
  'data["artifacts"].first["sha256"] = "not-a-digest"' \
  "artifacts[0].sha256 must be a 64-character hex digest when present"

make_invalid_case \
  "unused-artifact-id" \
  'data["artifacts"] << {"id" => "artifact-unused-20260616", "kind" => "supplemental-log", "uri" => "ci://target-release/20260616/unused.log", "recordedAt" => "2026-06-16T01:00:00Z"}' \
  "artifacts[9].id artifact-unused-20260616 must be referenced by at least one evidenceRef"

make_invalid_case \
  "missing-wechatpay-provider" \
  'data["providers"].reject! { |provider| provider["name"] == "wechatpay" }' \
  "providers must include live evidence for stripe, alipay, and wechatpay (missing: wechatpay)"

make_invalid_case \
  "missing-strict-k8s-flag" \
  'data["strictVerifier"]["command"] = data["strictVerifier"]["command"].split.reject { |token| token == "COMMERCIAL_COMPLETION_RUN_K8S=true" }.join(" ")' \
  "strictVerifier.command must include COMMERCIAL_COMPLETION_RUN_K8S=true"

make_invalid_case \
  "missing-strict-verifier-evidence-ref" \
  'data["strictVerifier"].delete("evidenceRef")' \
  "strictVerifier.evidenceRef is required"

make_invalid_case \
  "inverted-strict-verifier-window" \
  'data["strictVerifier"]["completedAt"] = "2026-06-15T23:59:00Z"' \
  "strictVerifier.completedAt must be at or after strictVerifier.startedAt"

make_invalid_case \
  "invalid-workflow-telemetry-window" \
  'data["workflowTelemetry"]["window"] = "not-a-window"' \
  "workflowTelemetry.window must be an ISO-8601 start/end interval"

make_invalid_case \
  "inverted-workflow-telemetry-window" \
  'data["workflowTelemetry"]["window"] = "2026-06-16T01:00:00Z/2026-06-16T00:00:00Z"' \
  "workflowTelemetry.window end must be at or after start"

make_invalid_case \
  "missing-grpc-smoke-report" \
  'data.delete("grpcSmokeReport")' \
  "grpcSmokeReport is required"

make_invalid_case \
  "failed-grpc-smoke-result" \
  'data["grpcSmokeReport"]["results"].find { |result| result["service"] == "task" }["generatedClient"] = "fail"' \
  "grpcSmokeReport.results[2].generatedClient must be pass"

make_invalid_case \
  "mismatched-grpc-smoke-evidence-ref" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["evidenceRef"] = "artifact-agent-only"' \
  "grpc agent evidenceRef must match grpcSmokeReport.evidenceRef"

echo "[target-release-evidence-fixtures] target release evidence verifier behavior is guarded."
