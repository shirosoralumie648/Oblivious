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
data["runId"] = "target-release-20260616"
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
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "ci://target-release/20260616/strict-verifier.log",
    "recordedAt" => "2026-06-16T01:00:00Z",
    "sha256" => "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  },
  {
    "id" => "artifact-deploy-20260616",
    "kind" => "deployment-log",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "ci://target-release/20260616/deploy.log",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-k8s-20260616",
    "kind" => "kubernetes-validation",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "ci://target-release/20260616/kubernetes.log",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-provider-stripe-20260616",
    "kind" => "provider-live-rail",
    "provider" => "stripe",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "provider://stripe/live/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-provider-alipay-20260616",
    "kind" => "provider-live-rail",
    "provider" => "alipay",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "provider://alipay/live/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-provider-wechatpay-20260616",
    "kind" => "provider-live-rail",
    "provider" => "wechatpay",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "provider://wechatpay/live/20260616",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-grpc-smoke-20260616",
    "kind" => "grpc-smoke-report",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "ci://target-release/20260616/grpc-smoke.json",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-secret-audit-20260616",
    "kind" => "secret-audit",
    "commit" => data["commit"],
    "runId" => data["runId"],
    "uri" => "ci://target-release/20260616/secret-audit.log",
    "recordedAt" => "2026-06-16T01:00:00Z"
  },
  {
    "id" => "artifact-workflow-telemetry-20260616",
    "kind" => "workflow-telemetry",
    "commit" => data["commit"],
    "runId" => data["runId"],
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
  "missing-run-lineage" \
  'data.delete("runId")' \
  "runId is required"

make_invalid_case \
  "dangling-strict-verifier-evidence-ref" \
  'data["strictVerifier"]["evidenceRef"] = "artifact-missing-20260616"' \
  "strictVerifier.evidenceRef must reference an artifact id listed in artifacts"

make_invalid_case \
  "mismatched-strict-verifier-artifact-kind" \
  'data["strictVerifier"]["evidenceRef"] = "artifact-provider-stripe-20260616"; data["providers"].find { |provider| provider["name"] == "stripe" }["evidenceRef"] = "artifact-strict-verifier-20260616"' \
  "strictVerifier.evidenceRef must reference artifact kind strict-verifier-log"

make_invalid_case \
  "mismatched-artifact-commit" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["commit"] = "0000000000000000000000000000000000000000"' \
  "artifacts[1].commit must match manifest commit"

make_invalid_case \
  "mismatched-artifact-run-id" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["runId"] = "target-release-20260615"' \
  "artifacts[1].runId must match manifest runId"

make_invalid_case \
  "placeholder-environment-base-url" \
  'data["environment"]["baseUrl"] = "TODO-target-base-url"' \
  "environment.baseUrl must reference a concrete target environment value, not a placeholder"

make_invalid_case \
  "invalid-environment-base-url" \
  'data["environment"]["baseUrl"] = "not-a-url"' \
  "environment.baseUrl must be an HTTP(S) URL"

make_invalid_case \
  "loopback-environment-base-url" \
  'data["environment"]["baseUrl"] = "http://localhost:3000"' \
  "environment.baseUrl must target a non-local target environment"

make_invalid_case \
  "abbreviated-loopback-environment-base-url" \
  'data["environment"]["baseUrl"] = "http://127.1:3000"' \
  "environment.baseUrl must target a non-local target environment"

make_invalid_case \
  "loopback-environment-trailing-dot" \
  'data["environment"]["baseUrl"] = "http://localhost.:3000"' \
  "environment.baseUrl must target a non-local target environment"

make_invalid_case \
  "ipv6-mapped-loopback-environment" \
  'data["environment"]["baseUrl"] = "http://[::ffff:127.0.0.1]:3000"' \
  "environment.baseUrl must target a non-local target environment"

make_invalid_case \
  "ipv6-unspecified-environment-base-url" \
  'data["environment"]["baseUrl"] = "http://[::]:3000"' \
  "environment.baseUrl must target a non-local target environment"

make_invalid_case \
  "zero-host-environment-base-url" \
  'data["environment"]["baseUrl"] = "http://0:3000"' \
  "environment.baseUrl must target a non-local target environment"

make_invalid_case \
  "credential-environment-base-url-userinfo" \
  'data["environment"]["baseUrl"] = "https://target-user:target-password@staging.oblivious.internal"' \
  "environment.baseUrl must not embed credentials in URI userinfo"

make_invalid_case \
  "secret-environment-base-url-query" \
  'data["environment"]["baseUrl"] = "https://staging.oblivious.internal?token=target-secret-token"' \
  "environment.baseUrl must not embed secret-like query or fragment parameters"

make_invalid_case \
  "password-environment-base-url-query" \
  'data["environment"]["baseUrl"] = "https://staging.oblivious.internal?password=target-secret-password"' \
  "environment.baseUrl must not embed secret-like query or fragment parameters"

make_invalid_case \
  "encoded-password-environment-base-url-query" \
  'data["environment"]["baseUrl"] = "https://staging.oblivious.internal?pass%77ord=target-secret-password"' \
  "environment.baseUrl must not embed secret-like query or fragment parameters"

make_invalid_case \
  "double-encoded-password-environment-base-url-query" \
  'data["environment"]["baseUrl"] = "https://staging.oblivious.internal?pass%2577ord=target-secret-password"' \
  "environment.baseUrl must not embed secret-like query or fragment parameters"

make_invalid_case \
  "secret-value-environment-base-url-query" \
  'data["environment"]["baseUrl"] = "https://staging.oblivious.internal?evidence=target-secret-token"' \
  "environment.baseUrl must not embed secret-like query or fragment parameters"

make_invalid_case \
  "secret-environment-base-url-fragment" \
  'data["environment"]["baseUrl"] = "https://staging.oblivious.internal#token=target-secret-token"' \
  "environment.baseUrl must not embed secret-like query or fragment parameters"

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
  "artifacts[1].uri must not embed secret-like query or fragment parameters"

make_invalid_case \
  "password-artifact-uri-query" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://ci.internal/runs/20260616/deploy.log?password=target-secret-password"' \
  "artifacts[1].uri must not embed secret-like query or fragment parameters"

make_invalid_case \
  "encoded-token-artifact-uri-query" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://ci.internal/runs/20260616/deploy.log?%74oken=target-secret-token"' \
  "artifacts[1].uri must not embed secret-like query or fragment parameters"

make_invalid_case \
  "double-encoded-token-artifact-uri-query" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://ci.internal/runs/20260616/deploy.log?t%256fken=target-secret-token"' \
  "artifacts[1].uri must not embed secret-like query or fragment parameters"

make_invalid_case \
  "secret-value-artifact-uri-query" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://ci.internal/runs/20260616/deploy.log?evidence=target-secret-token"' \
  "artifacts[1].uri must not embed secret-like query or fragment parameters"

make_invalid_case \
  "secret-artifact-uri-fragment" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://ci.internal/runs/20260616/deploy.log#token=target-secret-token"' \
  "artifacts[1].uri must not embed secret-like query or fragment parameters"

make_invalid_case \
  "credential-artifact-uri-userinfo" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "https://target-user:target-password@ci.internal/runs/20260616/deploy.log"' \
  "artifacts[1].uri must not embed credentials in URI userinfo"

make_invalid_case \
  "secret-audit-embedded-token" \
  'data["secretAudit"]["apiToken"] = "target-secret-token"' \
  "secretAudit.apiToken must not embed secret material"

make_invalid_case \
  "incomplete-secret-audit-scope" \
  'data["secretAudit"]["scope"] = ["runtime"]' \
  "secretAudit.scope must include kubernetes, providers, and runtime (missing: kubernetes, providers)"

make_invalid_case \
  "local-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "/tmp/target-release/deploy.log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

make_invalid_case \
  "file-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "file:///tmp/target-release/deploy.log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

make_invalid_case \
  "loopback-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "http://localhost:8080/target-release/deploy.log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

make_invalid_case \
  "abbreviated-loopback-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "http://127.1:8080/target-release/deploy.log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

make_invalid_case \
  "ipv6-mapped-loopback-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "http://[::ffff:127.0.0.1]:8080/target-release/deploy.log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

make_invalid_case \
  "zero-host-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "http://0:8080/target-release/deploy.log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

make_invalid_case \
  "inline-artifact-uri" \
  'data["artifacts"].find { |artifact| artifact["id"] == "artifact-deploy-20260616" }["uri"] = "data:text/plain,target-release-log"' \
  "artifacts[1].uri must reference a remote target artifact URI"

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
  "artifacts[9].id artifact-unused-20260616 must be referenced by a required evidenceRef"

make_invalid_case \
  "unused-artifact-masked-by-freeform-evidence-ref" \
  'data["artifacts"] << {"id" => "artifact-unused-20260616", "kind" => "supplemental-log", "commit" => data["commit"], "runId" => data["runId"], "uri" => "ci://target-release/20260616/unused.log", "recordedAt" => "2026-06-16T01:00:00Z"}; data["notes"] = {"evidenceRef" => "artifact-unused-20260616"}' \
  "artifacts[9].id artifact-unused-20260616 must be referenced by a required evidenceRef"

make_invalid_case \
  "missing-providers-collection" \
  'data.delete("providers")' \
  "providers must include at least one live provider evidence entry"

make_invalid_case \
  "missing-wechatpay-provider" \
  'data["providers"].reject! { |provider| provider["name"] == "wechatpay" }' \
  "providers must include live evidence for stripe, alipay, and wechatpay (missing: wechatpay)"

make_invalid_case \
  "unknown-provider-live-rail" \
  'data["providers"] << {"name" => "paypal", "mode" => "live", "checkout" => "pass", "refund" => "pass", "payout" => "pass", "reconciliation" => "pass", "evidenceRef" => "artifact-provider-paypal-20260616"}; data["artifacts"] << {"id" => "artifact-provider-paypal-20260616", "kind" => "provider-live-rail", "provider" => "paypal", "commit" => data["commit"], "runId" => data["runId"], "uri" => "provider://paypal/live/20260616", "recordedAt" => "2026-06-16T01:00:00Z"}' \
  "providers[3].name must be stripe, alipay, or wechatpay"

make_invalid_case \
  "swapped-provider-evidence-ref" \
  'stripe = data["providers"].find { |provider| provider["name"] == "stripe" }; alipay = data["providers"].find { |provider| provider["name"] == "alipay" }; stripe["evidenceRef"], alipay["evidenceRef"] = alipay["evidenceRef"], stripe["evidenceRef"]' \
  "providers[0].evidenceRef must reference provider-specific live evidence for stripe"

make_invalid_case \
  "missing-strict-k8s-flag" \
  'data["strictVerifier"]["command"] = data["strictVerifier"]["command"].split.reject { |token| token == "COMMERCIAL_COMPLETION_RUN_K8S=true" }.join(" ")' \
  "strictVerifier.command must include COMMERCIAL_COMPLETION_RUN_K8S=true"

make_invalid_case \
  "strict-command-mask-or-true" \
  'data["strictVerifier"]["command"] = data["strictVerifier"]["command"] + " || true"' \
  "strictVerifier.command must use the canonical strict verifier invocation"

make_invalid_case \
  "strict-command-env-quoted-skip-flags-as-args" \
  'data["strictVerifier"]["command"] = "env '\''COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true'\'' " + data["strictVerifier"]["command"] + " COMMERCIAL_COMPLETION_RUN_DEPLOY=true"' \
  "strictVerifier.command must use the canonical strict verifier invocation"

make_invalid_case \
  "quoted-env-skip-command" \
  'data["strictVerifier"]["command"] = "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS='\''true'\'' " + data["strictVerifier"]["command"]' \
  "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS"

make_invalid_case \
  "ansi-c-quoted-env-skip-command" \
  'data["strictVerifier"]["command"] = "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=$'\''true'\'' " + data["strictVerifier"]["command"]' \
  "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS"

make_invalid_case \
  "commit-mismatch-override-command" \
  'data["strictVerifier"]["command"] = "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true " + data["strictVerifier"]["command"]' \
  "strictVerifier.command must not enable OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH"

make_invalid_case \
  "ansi-c-quoted-commit-mismatch-command" \
  'data["strictVerifier"]["command"] = "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=$'\''true'\'' " + data["strictVerifier"]["command"]' \
  "strictVerifier.command must not enable OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH"

if OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true bash "$repo_root/scripts/verify-commercial-completion.sh" >"$tmpdir/commercial-commit-mismatch-override.out" 2>&1; then
  cat "$tmpdir/commercial-commit-mismatch-override.out" >&2
  fail "commercial-completion-commit-mismatch-override unexpectedly passed"
fi
if ! grep -Fq -- "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH cannot be true for strict final readiness" "$tmpdir/commercial-commit-mismatch-override.out"; then
  cat "$tmpdir/commercial-commit-mismatch-override.out" >&2
  fail "commercial-completion-commit-mismatch-override failed without expected pattern"
fi
echo "[target-release-evidence-fixtures] rejected commercial-completion-commit-mismatch-override"

make_invalid_case \
  "missing-strict-verifier-evidence-ref" \
  'data["strictVerifier"].delete("evidenceRef")' \
  "strictVerifier.evidenceRef is required"

make_invalid_case \
  "inverted-strict-verifier-window" \
  'data["strictVerifier"]["completedAt"] = "2026-06-15T23:59:00Z"' \
  "strictVerifier.completedAt must be at or after strictVerifier.startedAt"

make_invalid_case \
  "strict-verifier-artifact-outside-window" \
  'data["artifacts"].find { |artifact| artifact["id"] == data["strictVerifier"]["evidenceRef"] }["recordedAt"] = "2026-06-15T23:59:00Z"' \
  "strictVerifier.evidenceRef artifact recordedAt must be within strictVerifier.startedAt/completedAt"

make_invalid_case \
  "unfilled-kubernetes-secret-file-class" \
  'data["kubernetes"]["secretFileClass"] = "external-empty"' \
  "kubernetes.secretFileClass must be external-filled"

make_invalid_case \
  "invalid-workflow-telemetry-window" \
  'data["workflowTelemetry"]["window"] = "not-a-window"' \
  "workflowTelemetry.window must be an ISO-8601 start/end interval"

make_invalid_case \
  "inverted-workflow-telemetry-window" \
  'data["workflowTelemetry"]["window"] = "2026-06-16T01:00:00Z/2026-06-16T00:00:00Z"' \
  "workflowTelemetry.window end must be at or after start"

make_invalid_case \
  "workflow-telemetry-success-rate-above-one" \
  'data["workflowTelemetry"]["successRate"] = 1.01' \
  "workflowTelemetry.successRate must be between 0.99 and 1.0"

make_invalid_case \
  "missing-grpc-collection" \
  'data.delete("grpc")' \
  "grpc must be an array"

make_invalid_case \
  "unknown-grpc-service" \
  'data["grpc"] << {"service" => "admin", "address" => "admin:50066", "generatedClient" => "pass", "evidenceRef" => "artifact-grpc-smoke-20260616"}; data["grpcSmokeReport"]["results"] << {"service" => "admin", "address" => "admin:50066", "generatedClient" => "pass", "status" => "validation_response"}' \
  "grpc[3].service must be agent, workflow, or task"

make_invalid_case \
  "unknown-grpc-smoke-service" \
  'data["grpcSmokeReport"]["results"] << {"service" => "admin", "address" => "admin:50066", "generatedClient" => "pass", "status" => "validation_response"}' \
  "grpcSmokeReport.results[3].service must be agent, workflow, or task"

make_invalid_case \
  "loopback-grpc-address" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["address"] = "localhost:50063"; data["grpcSmokeReport"]["results"].find { |result| result["service"] == "agent" }["address"] = "localhost:50063"' \
  "grpc[0].address for agent must target a non-local service endpoint"

make_invalid_case \
  "abbreviated-loopback-grpc-address" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["address"] = "127.1:50063"; data["grpcSmokeReport"]["results"].find { |result| result["service"] == "agent" }["address"] = "127.1:50063"' \
  "grpc[0].address for agent must target a non-local service endpoint"

make_invalid_case \
  "ipv6-mapped-loopback-grpc-address" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["address"] = "[::ffff:127.0.0.1]:50063"; data["grpcSmokeReport"]["results"].find { |result| result["service"] == "agent" }["address"] = "[::ffff:127.0.0.1]:50063"' \
  "grpc[0].address for agent must target a non-local service endpoint"

make_invalid_case \
  "zero-host-grpc-address" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["address"] = "0:50063"; data["grpcSmokeReport"]["results"].find { |result| result["service"] == "agent" }["address"] = "0:50063"' \
  "grpc[0].address for agent must target a non-local service endpoint"

make_invalid_case \
  "url-shaped-grpc-address" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["address"] = "https://agent.target.internal:50063"; data["grpcSmokeReport"]["results"].find { |result| result["service"] == "agent" }["address"] = "https://agent.target.internal:50063"' \
  "grpc[0].address for agent must be a plain host:port endpoint"

make_invalid_case \
  "missing-grpc-smoke-report" \
  'data.delete("grpcSmokeReport")' \
  "grpcSmokeReport is required"

make_invalid_case \
  "invalid-grpc-smoke-timeout" \
  'data["grpcSmokeReport"]["timeout"] = "not-a-duration"' \
  "grpcSmokeReport.timeout must be a positive Go duration string"

make_invalid_case \
  "failed-grpc-smoke-result" \
  'data["grpcSmokeReport"]["results"].find { |result| result["service"] == "task" }["generatedClient"] = "fail"' \
  "grpcSmokeReport.results[2].generatedClient must be pass"

make_invalid_case \
  "workflow-grpc-smoke-status-mismatch" \
  'data["grpcSmokeReport"]["results"].find { |result| result["service"] == "workflow" }["status"] = "validation_error"' \
  "grpcSmokeReport.results[1].status for workflow must be validation_response"

make_invalid_case \
  "mismatched-grpc-smoke-evidence-ref" \
  'data["grpc"].find { |entry| entry["service"] == "agent" }["evidenceRef"] = "artifact-agent-only"' \
  "grpc agent evidenceRef must match grpcSmokeReport.evidenceRef"

make_invalid_case \
  "grpc-smoke-artifact-before-report" \
  'data["grpcSmokeReport"]["recordedAt"] = "2026-06-16T02:00:00Z"' \
  "grpcSmokeReport.evidenceRef artifact recordedAt must be at or after grpcSmokeReport.recordedAt"

make_invalid_case \
  "workflow-telemetry-artifact-before-window-end" \
  'data["workflowTelemetry"]["window"] = "2026-06-16T00:00:00Z/2026-06-16T02:00:00Z"' \
  "workflowTelemetry.evidenceRef artifact recordedAt must be at or after workflowTelemetry.window end"

echo "[target-release-evidence-fixtures] target release evidence verifier behavior is guarded."
