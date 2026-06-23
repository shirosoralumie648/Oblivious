#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

usage() {
  cat <<'EOF'
Usage: OBLIVIOUS_TARGET_EVIDENCE_FILE=/path/outside/git/target-release-evidence.json bash scripts/verify-target-release-evidence.sh
       bash scripts/verify-target-release-evidence.sh /path/outside/git/target-release-evidence.json
       bash scripts/verify-target-release-evidence.sh --print-template > /path/outside/git/target-release-evidence.json

Validates target/live release evidence that cannot be proven by repository-local tests.
The evidence file must be JSON, must not contain secrets, and must refer to the current git HEAD.

Required JSON shape:
{
  "schemaVersion": 1,
  "repository": "Oblivious",
  "commit": "<full git sha>",
  "runId": "target-release-20260616",
  "environment": {
    "name": "staging",
    "class": "staging Kubernetes",
    "baseUrl": "https://oblivious.example.com",
    "recordedAt": "2026-06-16T00:00:00Z"
  },
  "strictVerifier": {
    "command": "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
    "result": "pass",
    "skippedChecks": [],
    "startedAt": "2026-06-16T00:00:00Z",
    "completedAt": "2026-06-16T01:00:00Z",
    "evidenceRef": "strict-verifier-log-or-artifact-id"
  },
  "deployment": {
    "deployValidation": "pass",
    "backupRestore": "pass",
    "migrationReplay": "pass",
    "evidenceRef": "release-log-or-artifact-id"
  },
  "kubernetes": {
    "validation": "pass",
    "rollout": "pass",
    "failover": "pass",
    "secretFileClass": "external-filled",
    "evidenceRef": "release-log-or-artifact-id"
  },
  "providers": [
    {
      "name": "stripe",
      "mode": "live",
      "checkout": "pass",
      "refund": "pass",
      "payout": "pass",
      "reconciliation": "pass",
      "evidenceRef": "stripe-provider-run-id"
    },
    {
      "name": "alipay",
      "mode": "live",
      "checkout": "pass",
      "refund": "pass",
      "payout": "pass",
      "reconciliation": "pass",
      "evidenceRef": "alipay-provider-run-id"
    },
    {
      "name": "wechatpay",
      "mode": "live",
      "checkout": "pass",
      "refund": "pass",
      "payout": "pass",
      "reconciliation": "pass",
      "evidenceRef": "wechatpay-provider-run-id"
    }
  ],
  "grpc": [
    {"service": "agent", "address": "agent:50063", "generatedClient": "pass", "evidenceRef": "grpc-smoke-log"},
    {"service": "workflow", "address": "workflow:50064", "generatedClient": "pass", "evidenceRef": "grpc-smoke-log"},
    {"service": "task", "address": "task:50065", "generatedClient": "pass", "evidenceRef": "grpc-smoke-log"}
  ],
  "grpcSmokeReport": {
    "evidenceRef": "grpc-smoke-log",
    "recordedAt": "2026-06-16T00:00:00Z",
    "timeout": "10s",
    "results": [
      {"service": "agent", "address": "agent:50063", "generatedClient": "pass", "status": "validation_error"},
      {"service": "workflow", "address": "workflow:50064", "generatedClient": "pass", "status": "validation_response"},
      {"service": "task", "address": "task:50065", "generatedClient": "pass", "status": "validation_response"}
    ]
  },
  "secretAudit": {
    "result": "pass",
    "scope": ["kubernetes", "providers", "runtime"],
    "evidenceRef": "secret-audit-log"
  },
  "workflowTelemetry": {
    "result": "pass",
    "successRate": 0.99,
    "window": "2026-06-16T00:00:00Z/2026-06-16T01:00:00Z",
    "evidenceRef": "telemetry-dashboard-or-export"
  },
  "artifacts": [
    {"id": "strict-verifier-log-or-artifact-id", "kind": "strict-verifier-log", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/strict-verifier", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "release-log-or-artifact-id", "kind": "deployment-log", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/deployment", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "stripe-provider-run-id", "kind": "provider-live-rail", "provider": "stripe", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/provider/stripe", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "alipay-provider-run-id", "kind": "provider-live-rail", "provider": "alipay", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/provider/alipay", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "wechatpay-provider-run-id", "kind": "provider-live-rail", "provider": "wechatpay", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/provider/wechatpay", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "grpc-smoke-log", "kind": "grpc-smoke-report", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/grpc-smoke", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "secret-audit-log", "kind": "secret-audit", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/secret-audit", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "telemetry-dashboard-or-export", "kind": "workflow-telemetry", "commit": "<full git sha>", "runId": "target-release-20260616", "uri": "ci://run/workflow-telemetry", "recordedAt": "2026-06-16T01:00:00Z"}
  ]
}

Optional:
  OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH=true
    Allow validating an evidence file for a different commit. This is never final readiness proof.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

current_commit=$(git -C "$repo_root" rev-parse HEAD)

if [[ "${1:-}" == "--print-template" ]]; then
  CURRENT_COMMIT="$current_commit" ruby <<'RUBY'
require "json"
require "time"
require "uri"

recorded_at = Time.now.utc

puts JSON.pretty_generate(
  {
    "schemaVersion" => 1,
    "repository" => "Oblivious",
    "commit" => ENV.fetch("CURRENT_COMMIT"),
    "runId" => "TODO-target-release-run-id",
    "environment" => {
      "name" => "TODO-target-environment-name",
      "class" => "TODO-target-environment-class",
      "baseUrl" => "TODO-target-base-url",
      "recordedAt" => recorded_at.iso8601
    },
    "strictVerifier" => {
      "command" => "COMMERCIAL_COMPLETION_RUN_DEPLOY=true COMMERCIAL_COMPLETION_RUN_K8S=true COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true bash scripts/verify-commercial-completion.sh",
      "result" => "pass",
      "skippedChecks" => [],
      "startedAt" => recorded_at.iso8601,
      "completedAt" => recorded_at.iso8601,
      "evidenceRef" => "TODO-strict-commercial-verifier-log"
    },
    "deployment" => {
      "deployValidation" => "pass",
      "backupRestore" => "pass",
      "migrationReplay" => "pass",
      "evidenceRef" => "TODO-release-log-or-artifact-id"
    },
    "kubernetes" => {
      "validation" => "pass",
      "rollout" => "pass",
      "failover" => "pass",
      "secretFileClass" => "external-filled",
      "evidenceRef" => "TODO-kubernetes-release-log-or-artifact-id"
    },
    "providers" => [
      {
        "name" => "stripe",
        "mode" => "live",
        "checkout" => "pass",
        "refund" => "pass",
        "payout" => "pass",
        "reconciliation" => "pass",
        "evidenceRef" => "TODO-stripe-provider-run-id"
      },
      {
        "name" => "alipay",
        "mode" => "live",
        "checkout" => "pass",
        "refund" => "pass",
        "payout" => "pass",
        "reconciliation" => "pass",
        "evidenceRef" => "TODO-alipay-provider-run-id"
      },
      {
        "name" => "wechatpay",
        "mode" => "live",
        "checkout" => "pass",
        "refund" => "pass",
        "payout" => "pass",
        "reconciliation" => "pass",
        "evidenceRef" => "TODO-wechatpay-provider-run-id"
      }
    ],
    "grpc" => [
      {"service" => "agent", "address" => "agent:50063", "generatedClient" => "pass", "evidenceRef" => "TODO-agent-grpc-smoke-log"},
      {"service" => "workflow", "address" => "workflow:50064", "generatedClient" => "pass", "evidenceRef" => "TODO-workflow-grpc-smoke-log"},
      {"service" => "task", "address" => "task:50065", "generatedClient" => "pass", "evidenceRef" => "TODO-task-grpc-smoke-log"}
    ],
    "grpcSmokeReport" => {
      "evidenceRef" => "TODO-target-grpc-smoke-report",
      "recordedAt" => Time.now.utc.iso8601,
      "timeout" => "10s",
      "results" => [
        {"service" => "agent", "address" => "agent:50063", "generatedClient" => "pass", "status" => "validation_error"},
        {"service" => "workflow", "address" => "workflow:50064", "generatedClient" => "pass", "status" => "validation_response"},
        {"service" => "task", "address" => "task:50065", "generatedClient" => "pass", "status" => "validation_response"}
      ]
    },
    "secretAudit" => {
      "result" => "pass",
      "scope" => ["kubernetes", "providers", "runtime"],
      "evidenceRef" => "TODO-secret-audit-log"
    },
    "workflowTelemetry" => {
      "result" => "pass",
      "successRate" => 0.99,
      "window" => "TODO-ISO8601-start/TODO-ISO8601-end",
      "evidenceRef" => "TODO-telemetry-dashboard-or-export"
    },
    "artifacts" => [
      {"id" => "TODO-strict-commercial-verifier-log", "kind" => "strict-verifier-log", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-strict-commercial-verifier-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-release-log-or-artifact-id", "kind" => "deployment-log", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-release-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-kubernetes-release-log-or-artifact-id", "kind" => "kubernetes-validation", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-kubernetes-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-stripe-provider-run-id", "kind" => "provider-live-rail", "provider" => "stripe", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-stripe-provider-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-alipay-provider-run-id", "kind" => "provider-live-rail", "provider" => "alipay", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-alipay-provider-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-wechatpay-provider-run-id", "kind" => "provider-live-rail", "provider" => "wechatpay", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-wechatpay-provider-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-target-grpc-smoke-report", "kind" => "grpc-smoke-report", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-target-grpc-smoke-report-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-secret-audit-log", "kind" => "secret-audit", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-secret-audit-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-telemetry-dashboard-or-export", "kind" => "workflow-telemetry", "commit" => ENV.fetch("CURRENT_COMMIT"), "runId" => "TODO-target-release-run-id", "uri" => "TODO-telemetry-dashboard-or-export-uri", "recordedAt" => recorded_at.iso8601}
    ]
  }
)
RUBY
  exit 0
fi

evidence_file="${1:-${OBLIVIOUS_TARGET_EVIDENCE_FILE:-}}"
if [[ -z "$evidence_file" ]]; then
  echo "[target-release-evidence] OBLIVIOUS_TARGET_EVIDENCE_FILE or file argument is required" >&2
  usage >&2
  exit 1
fi
if [[ ! -f "$evidence_file" ]]; then
  echo "[target-release-evidence] evidence file not found: $evidence_file" >&2
  exit 1
fi

allow_commit_mismatch="${OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH:-false}"

CURRENT_COMMIT="$current_commit" \
EVIDENCE_FILE="$evidence_file" \
ALLOW_COMMIT_MISMATCH="$allow_commit_mismatch" \
ruby <<'RUBY'
require "ipaddr"
require "json"
require "shellwords"
require "time"
require "uri"

def blank?(value)
  value.nil? || (value.respond_to?(:empty?) && value.empty?)
end

def dig_path(hash, path)
  path.reduce(hash) do |memo, key|
    case memo
    when Hash
      memo[key]
    when Array
      key.is_a?(Integer) ? memo[key] : nil
    else
      nil
    end
  end
end

def require_string(failures, data, path)
  value = dig_path(data, path)
  failures << "#{path.join(".")} is required" if !value.is_a?(String) || value.strip.empty?
  value
end

def require_pass(failures, data, path)
  value = dig_path(data, path)
  failures << "#{path.join(".")} must be pass" unless value == "pass"
end

def require_iso8601_interval(failures, data, path, interval_error:, ordering_error:)
  value = require_string(failures, data, path)
  return unless value.is_a?(String) && !value.strip.empty?

  parts = value.split("/", -1)
  if parts.length != 2 || parts.any? { |part| part.strip.empty? }
    failures << interval_error
    return
  end

  begin
    starts_at = Time.iso8601(parts.fetch(0).strip)
    ends_at = Time.iso8601(parts.fetch(1).strip)
  rescue ArgumentError
    failures << interval_error
    return
  end

  if ends_at < starts_at
    failures << ordering_error
  end
end

def parse_iso8601_safely(value)
  return nil unless value.is_a?(String)

  Time.iso8601(value)
rescue ArgumentError
  nil
end

def iso8601_interval_end(value)
  return nil unless value.is_a?(String)

  parts = value.split("/", -1)
  return nil unless parts.length == 2 && parts.none? { |part| part.strip.empty? }

  parse_iso8601_safely(parts.fetch(1).strip)
end

GO_DURATION_PATTERN = /\A(?:(?:\d+(?:\.\d+)?|\.\d+)(?:ns|us|ms|s|m|h))+\z/.freeze

def positive_go_duration_string?(value)
  return false unless value.is_a?(String)

  duration = value.strip
  return false if duration.empty?
  return false unless duration.match?(GO_DURATION_PATTERN)

  duration.scan(/(?:\d+(?:\.\d+)?|\.\d+)(?=ns|us|ms|s|m|h)/).any? { |number| number.to_f.positive? }
end

def local_target_host?(host)
  normalized = host.to_s.strip.downcase.sub(/\A\[(.*)\]\z/, '\1')
  normalized = normalized.sub(/\.+\z/, "")
  return false if normalized.empty?
  return true if normalized == "localhost" || normalized.end_with?(".localhost")

  begin
    ip = IPAddr.new(normalized)
    ip = ip.native if ip.respond_to?(:ipv4_mapped?) && ip.ipv4_mapped?
    ip.loopback? || ip.to_i.zero?
  rescue ArgumentError
    false
  end
end

STRICT_VERIFIER_REQUIRED_ENV = %w[
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true
  COMMERCIAL_COMPLETION_RUN_K8S=true
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true
].freeze
STRICT_VERIFIER_COMMAND_TAIL = ["bash", "scripts/verify-commercial-completion.sh"].freeze
STRICT_VERIFIER_FORBIDDEN_ENV = {
  "COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS" => "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS",
  "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH" => "strictVerifier.command must not enable OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH"
}.freeze

def validate_strict_verifier_command(failures, command)
  tokens = Shellwords.split(command)
  required_env = STRICT_VERIFIER_REQUIRED_ENV.to_h { |assignment| assignment.split("=", 2) }
  seen_env = {}
  command_tail = tokens.last(2)

  tokens.each do |token|
    key, value = token.split("=", 2)
    next unless value

    failures << STRICT_VERIFIER_FORBIDDEN_ENV.fetch(key) if STRICT_VERIFIER_FORBIDDEN_ENV.key?(key)
    seen_env[key] = value
  end

  failures << "strictVerifier.command must run scripts/verify-commercial-completion.sh" unless command_tail == STRICT_VERIFIER_COMMAND_TAIL

  STRICT_VERIFIER_REQUIRED_ENV.each do |required_flag|
    key, value = required_flag.split("=", 2)
    unless seen_env[key] == value
      failures << "strictVerifier.command must include #{required_flag}"
    end
  end

  expected_tokens = STRICT_VERIFIER_REQUIRED_ENV + STRICT_VERIFIER_COMMAND_TAIL
  if tokens.length != expected_tokens.length ||
      command_tail != STRICT_VERIFIER_COMMAND_TAIL ||
      (tokens[0...-2].sort != STRICT_VERIFIER_REQUIRED_ENV.sort)
    failures << "strictVerifier.command must use the canonical strict verifier invocation"
  end
rescue ArgumentError
  failures << "strictVerifier.command must use the canonical strict verifier invocation"
end

def require_http_url(failures, data, path, error, local_error: nil)
  value = require_string(failures, data, path)
  return unless value.is_a?(String) && !value.strip.empty?

  begin
    uri = URI.parse(value.strip)
  rescue URI::InvalidURIError
    failures << error
    return
  end

  unless %w[http https].include?(uri.scheme) && !blank?(uri.host)
    failures << error
    return
  end

  if local_error && local_target_host?(uri.host)
    failures << local_error
  end
end

def endpoint_host(value)
  raw = value.to_s.strip
  return nil if raw.empty?

  if raw.match?(/\A[a-z][a-z0-9+\-.]*:\/\//i)
    begin
      uri = URI.parse(raw)
      return uri.host unless blank?(uri.host)
    rescue URI::InvalidURIError
      return nil
    end
    raw = raw.sub(/\A[a-z][a-z0-9+\-.]*:\/+/i, "")
  end

  raw = raw.sub(/\A\/+/, "")
  if raw.start_with?("[")
    raw[/\A\[([^\]]+)\]/, 1]
  else
    raw.split(/[\/:]/, 2).first
  end
end

def local_endpoint?(value)
  host = endpoint_host(value)
  !blank?(host) && local_target_host?(host)
end

def parse_plain_grpc_address(value)
  return nil unless value.is_a?(String)

  raw = value.strip
  return nil if raw.empty?
  return nil if raw.match?(/\A[a-z][a-z0-9+\-.]*:\/\//i)
  return nil if raw.match?(/[\/?#]/)

  match = if raw.start_with?("[")
    raw.match(/\A\[([^\]]+)\]:(\d+)\z/)
  else
    raw.match(/\A([^:\s\[\]]+):(\d+)\z/)
  end
  return nil unless match

  {"host" => match[1], "port" => match[2]}
end

def placeholder?(value)
  value.is_a?(String) && value.match?(/TODO|TBD|placeholder|example|sample|\/path\/outside\/git|release-log-or-artifact-id|strict-verifier-log-or-artifact-id|strict-commercial-verifier-log|provider-run-id|grpc-smoke-log|secret-audit-log|telemetry-dashboard-or-export/i)
end

def secret_like_uri?(value)
  value.is_a?(String) && value.match?(/[?&#](?:[^=&#]*[_-])?(?:token|secret|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)=/i)
end

def userinfo_uri?(value)
  return false unless value.is_a?(String)

  begin
    uri = URI.parse(value.strip)
  rescue URI::InvalidURIError
    return false
  end

  uri.respond_to?(:userinfo) && !blank?(uri.userinfo)
end

def remote_artifact_uri?(value)
  return false unless value.is_a?(String)

  stripped = value.strip
  return false if stripped.empty? || stripped.start_with?("/")

  begin
    uri = URI.parse(stripped)
  rescue URI::InvalidURIError
    return false
  end

  return false if blank?(uri.scheme) || uri.scheme.downcase == "file"

  host = uri.respond_to?(:host) ? uri.host : nil
  return false if blank?(host)
  return false if local_target_host?(host)

  true
end

def require_evidence_ref(failures, data, path)
  value = require_string(failures, data, path)
  failures << "#{path.join(".")} must reference a concrete target artifact, not a placeholder" if placeholder?(value)
  value
end

def collect_skips(value, path = [], skips = [])
  case value
  when Hash
    value.each do |key, child|
      if key.to_s =~ /skip/i
        skips << [path + [key], child] unless child == [] || child == nil || child == false
      end
      collect_skips(child, path + [key], skips)
    end
  when Array
    value.each_with_index { |child, index| collect_skips(child, path + [index], skips) }
  end
  skips
end

def allowed_secret_metadata_path?(path)
  path == ["secretAudit"] || path == ["kubernetes", "secretFileClass"]
end

def collect_secret_material(value, path = [], findings = [])
  secret_key = path.last.to_s.match?(/secret|password|token|api[_-]?key|private[_-]?key|credential|kubeconfig/i)
  if secret_key && !allowed_secret_metadata_path?(path) && !blank?(value)
    findings << "#{path.join(".")} must not embed secret material"
  end

  if value.is_a?(String) && value.match?(/sk_(live|test)_[A-Za-z0-9]{12,}|rk_(live|test)_[A-Za-z0-9]{12,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|gh[pousr]_[A-Za-z0-9_]{20,}|xox[baprs]-[A-Za-z0-9-]{20,}/)
    findings << "#{path.join(".")} looks like an embedded secret value"
  end

  case value
  when Hash
    value.each { |key, child| collect_secret_material(child, path + [key], findings) }
  when Array
    value.each_with_index { |child, index| collect_secret_material(child, path + [index], findings) }
  end
  findings
end

def required_evidence_ref_path?(path)
  return true if path == ["strictVerifier", "evidenceRef"]
  return true if path == ["deployment", "evidenceRef"]
  return true if path == ["kubernetes", "evidenceRef"]
  return true if path == ["grpcSmokeReport", "evidenceRef"]
  return true if path == ["secretAudit", "evidenceRef"]
  return true if path == ["workflowTelemetry", "evidenceRef"]
  return true if path.length == 3 && path[0] == "providers" && path[1].is_a?(Integer) && path[2] == "evidenceRef"
  return true if path.length == 3 && path[0] == "grpc" && path[1].is_a?(Integer) && path[2] == "evidenceRef"

  false
end

def collect_required_evidence_refs(value, path = [], refs = [])
  case value
  when Hash
    value.each do |key, child|
      child_path = path + [key]
      refs << [child_path, child] if key == "evidenceRef" && required_evidence_ref_path?(child_path)
      collect_required_evidence_refs(child, child_path, refs)
    end
  when Array
    value.each_with_index { |child, index| collect_required_evidence_refs(child, path + [index], refs) }
  end
  refs
end

def require_artifact_kind(failures, data, artifact_ids, ref_path, expected_kind)
  ref = dig_path(data, ref_path)
  return unless ref.is_a?(String)
  return if placeholder?(ref)
  artifact = artifact_ids[ref]
  return unless artifact.is_a?(Hash)

  actual_kind = artifact["kind"]
  unless actual_kind == expected_kind
    failures << "#{ref_path.join(".")} must reference artifact kind #{expected_kind}"
  end
end

path = ENV.fetch("EVIDENCE_FILE")
current_commit = ENV.fetch("CURRENT_COMMIT")
allow_mismatch = ENV["ALLOW_COMMIT_MISMATCH"] == "true"
failures = []

begin
  data = JSON.parse(File.read(path))
rescue JSON::ParserError => e
  warn "[target-release-evidence] invalid JSON: #{e.message}"
  exit 1
end

unless data.is_a?(Hash)
  warn "[target-release-evidence] evidence root must be a JSON object"
  exit 1
end

failures << "schemaVersion must be 1" unless data["schemaVersion"] == 1
failures << "repository must be Oblivious" unless data["repository"] == "Oblivious"

commit = require_string(failures, data, ["commit"])
if commit && commit != current_commit && !allow_mismatch
  failures << "commit must match current HEAD #{current_commit}"
end
run_id = require_string(failures, data, ["runId"])
if run_id.is_a?(String) && placeholder?(run_id)
  failures << "runId must reference a concrete target evidence run, not a placeholder"
end

%w[name class baseUrl].each do |field|
  value = require_string(failures, data, ["environment", field])
  failures << "environment.#{field} must reference a concrete target environment value, not a placeholder" if placeholder?(value)
end
require_http_url(
  failures,
  data,
  ["environment", "baseUrl"],
  "environment.baseUrl must be an HTTP(S) URL",
  local_error: "environment.baseUrl must target a non-local target environment"
)
environment_base_url = dig_path(data, ["environment", "baseUrl"])
if environment_base_url.is_a?(String) && userinfo_uri?(environment_base_url)
  failures << "environment.baseUrl must not embed credentials in URI userinfo"
end
if environment_base_url.is_a?(String) && secret_like_uri?(environment_base_url)
  failures << "environment.baseUrl must not embed secret-like query or fragment parameters"
end
require_string(failures, data, ["environment", "recordedAt"])
recorded_at = dig_path(data, ["environment", "recordedAt"])
begin
  Time.iso8601(recorded_at) if recorded_at.is_a?(String)
rescue ArgumentError
  failures << "environment.recordedAt must be ISO-8601"
end

require_string(failures, data, ["strictVerifier", "command"])
require_pass(failures, data, ["strictVerifier", "result"])
command = dig_path(data, ["strictVerifier", "command"])
if command.is_a?(String)
  validate_strict_verifier_command(failures, command)
end
skipped_checks = dig_path(data, ["strictVerifier", "skippedChecks"])
failures << "strictVerifier.skippedChecks must be an empty array" unless skipped_checks == []
strict_started_at_raw = require_string(failures, data, ["strictVerifier", "startedAt"])
strict_completed_at_raw = require_string(failures, data, ["strictVerifier", "completedAt"])
strict_started_at = nil
strict_completed_at = nil
begin
  strict_started_at = Time.iso8601(strict_started_at_raw) if strict_started_at_raw.is_a?(String)
rescue ArgumentError
  failures << "strictVerifier.startedAt must be ISO-8601"
end
begin
  strict_completed_at = Time.iso8601(strict_completed_at_raw) if strict_completed_at_raw.is_a?(String)
rescue ArgumentError
  failures << "strictVerifier.completedAt must be ISO-8601"
end
if strict_started_at && strict_completed_at && strict_completed_at < strict_started_at
  failures << "strictVerifier.completedAt must be at or after strictVerifier.startedAt"
end
require_evidence_ref(failures, data, ["strictVerifier", "evidenceRef"])

%w[deployValidation backupRestore migrationReplay].each do |field|
  require_pass(failures, data, ["deployment", field])
end
require_evidence_ref(failures, data, ["deployment", "evidenceRef"])

%w[validation rollout failover].each do |field|
  require_pass(failures, data, ["kubernetes", field])
end
secret_file_class = require_string(failures, data, ["kubernetes", "secretFileClass"])
if secret_file_class.is_a?(String) && secret_file_class.strip != "external-filled"
  failures << "kubernetes.secretFileClass must be external-filled"
end
require_evidence_ref(failures, data, ["kubernetes", "evidenceRef"])

providers = data["providers"]
required_providers = %w[stripe alipay wechatpay]
if !providers.is_a?(Array) || providers.empty?
  failures << "providers must include at least one live provider evidence entry"
else
  providers_by_name = {}
  providers.each_with_index do |provider, index|
    unless provider.is_a?(Hash)
      failures << "providers[#{index}] must be an object"
      next
    end
    prefix = ["providers", index]
    name = require_string(failures, data, prefix + ["name"]).to_s.strip
    if !name.empty?
      if providers_by_name.key?(name)
        failures << "providers must not duplicate #{name} evidence"
      else
        providers_by_name[name] = provider
      end
    end
    failures << "providers[#{index}].mode must be live" unless provider["mode"] == "live"
    %w[checkout refund payout reconciliation].each do |field|
      failures << "providers[#{index}].#{field} must be pass" unless provider[field] == "pass"
    end
    failures << "providers[#{index}].evidenceRef is required" if blank?(provider["evidenceRef"])
    failures << "providers[#{index}].evidenceRef must reference a concrete target artifact, not a placeholder" if placeholder?(provider["evidenceRef"])
  end
  missing_providers = required_providers - providers_by_name.keys
  if missing_providers.any?
    failures << "providers must include live evidence for stripe, alipay, and wechatpay (missing: #{missing_providers.join(", ")})"
  end
end

grpc = data["grpc"]
required_services = %w[agent workflow task]
expected_grpc_ports = {"agent" => "50063", "workflow" => "50064", "task" => "50065"}
grpc_entries_by_service = {}
if !grpc.is_a?(Array)
  failures << "grpc must be an array"
else
  present = grpc.filter_map { |item| item["service"] if item.is_a?(Hash) }
  (required_services - present).each do |service|
    failures << "grpc must include #{service} service evidence"
  end
  grpc.each_with_index do |entry, index|
    unless entry.is_a?(Hash)
      failures << "grpc[#{index}] must be an object"
      next
    end
    service = entry["service"].to_s.strip
    address = entry["address"]
    failures << "grpc[#{index}].address is required" if blank?(address)
    parsed_address = parse_plain_grpc_address(address)
    if !blank?(address) && parsed_address.nil?
      failures << "grpc[#{index}].address for #{service} must be a plain host:port endpoint"
    end
    unless service.empty?
      if grpc_entries_by_service.key?(service)
        failures << "grpc must not duplicate #{service} service evidence"
      else
        grpc_entries_by_service[service] = entry
      end
    end
    expected_port = expected_grpc_ports[service]
    if parsed_address && expected_port && parsed_address["port"] != expected_port
      failures << "grpc[#{index}].address for #{service} must target port #{expected_port}"
    end
    if parsed_address && local_target_host?(parsed_address["host"])
      failures << "grpc[#{index}].address for #{service} must target a non-local service endpoint"
    end
    failures << "grpc[#{index}].generatedClient must be pass" unless entry["generatedClient"] == "pass"
    failures << "grpc[#{index}].evidenceRef is required" if blank?(entry["evidenceRef"])
    failures << "grpc[#{index}].evidenceRef must reference a concrete target artifact, not a placeholder" if placeholder?(entry["evidenceRef"])
  end
end

grpc_smoke_report = data["grpcSmokeReport"]
grpc_smoke_recorded_at = nil
if !grpc_smoke_report.is_a?(Hash)
  failures << "grpcSmokeReport is required"
else
  grpc_smoke_evidence_ref = require_evidence_ref(failures, data, ["grpcSmokeReport", "evidenceRef"])
  if grpc_smoke_evidence_ref.is_a?(String)
    grpc_entries_by_service.each do |service, entry|
      if entry["evidenceRef"].is_a?(String) && entry["evidenceRef"] != grpc_smoke_evidence_ref
        failures << "grpc #{service} evidenceRef must match grpcSmokeReport.evidenceRef"
      end
    end
  end
  smoke_recorded_at = require_string(failures, data, ["grpcSmokeReport", "recordedAt"])
  begin
    grpc_smoke_recorded_at = Time.iso8601(smoke_recorded_at) if smoke_recorded_at.is_a?(String)
  rescue ArgumentError
    failures << "grpcSmokeReport.recordedAt must be ISO-8601"
  end
  smoke_timeout = require_string(failures, data, ["grpcSmokeReport", "timeout"])
  if smoke_timeout.is_a?(String) && !smoke_timeout.strip.empty? && !positive_go_duration_string?(smoke_timeout)
    failures << "grpcSmokeReport.timeout must be a positive Go duration string"
  end
  smoke_results = grpc_smoke_report["results"]
  smoke_results_by_service = {}
  if !smoke_results.is_a?(Array)
    failures << "grpcSmokeReport.results must be an array"
  else
    smoke_results.each_with_index do |result, index|
      unless result.is_a?(Hash)
        failures << "grpcSmokeReport.results[#{index}] must be an object"
        next
      end
      service = result["service"].to_s.strip
      if service.empty?
        failures << "grpcSmokeReport.results[#{index}].service is required"
      elsif smoke_results_by_service.key?(service)
        failures << "grpcSmokeReport.results must not duplicate #{service} service results"
      else
        smoke_results_by_service[service] = result
      end
      result_address = result["address"]
      failures << "grpcSmokeReport.results[#{index}].address is required" if blank?(result_address)
      parsed_result_address = parse_plain_grpc_address(result_address)
      if !blank?(result_address) && parsed_result_address.nil?
        failures << "grpcSmokeReport.results[#{index}].address for #{service} must be a plain host:port endpoint"
      end
      manifest_entry = grpc_entries_by_service[service]
      if manifest_entry && result_address.to_s != manifest_entry["address"].to_s
        failures << "grpcSmokeReport.results[#{index}].address must match grpc #{service} address"
      end
      expected_port = expected_grpc_ports[service]
      if parsed_result_address && expected_port && parsed_result_address["port"] != expected_port
        failures << "grpcSmokeReport.results[#{index}].address for #{service} must target port #{expected_port}"
      end
      if parsed_result_address && local_target_host?(parsed_result_address["host"])
        failures << "grpcSmokeReport.results[#{index}].address for #{service} must target a non-local service endpoint"
      end
      failures << "grpcSmokeReport.results[#{index}].generatedClient must be pass" unless result["generatedClient"] == "pass"
      status = require_string(failures, data, ["grpcSmokeReport", "results", index, "status"])
      if status.is_a?(String) && !%w[validation_error validation_response].include?(status)
        failures << "grpcSmokeReport.results[#{index}].status must be validation_error or validation_response"
      end
    end
    missing_smoke_services = required_services - smoke_results_by_service.keys
    if missing_smoke_services.any?
      failures << "grpcSmokeReport.results must include agent, workflow, and task smoke results (missing: #{missing_smoke_services.join(", ")})"
    end
  end
end

require_pass(failures, data, ["secretAudit", "result"])
require_evidence_ref(failures, data, ["secretAudit", "evidenceRef"])
scope = dig_path(data, ["secretAudit", "scope"])
if !scope.is_a?(Array) || scope.empty? || scope.any? { |item| !item.is_a?(String) || item.strip.empty? }
  failures << "secretAudit.scope must be a non-empty array of strings"
else
  required_secret_audit_scopes = %w[kubernetes providers runtime]
  normalized_scope = scope.map { |item| item.strip.downcase }.uniq
  missing_secret_audit_scopes = required_secret_audit_scopes - normalized_scope
  if missing_secret_audit_scopes.any?
    failures << "secretAudit.scope must include kubernetes, providers, and runtime (missing: #{missing_secret_audit_scopes.join(", ")})"
  end
end

require_pass(failures, data, ["workflowTelemetry", "result"])
success_rate = dig_path(data, ["workflowTelemetry", "successRate"])
unless success_rate.is_a?(Numeric) && success_rate >= 0.99 && success_rate <= 1.0
  failures << "workflowTelemetry.successRate must be between 0.99 and 1.0"
end
require_iso8601_interval(
  failures,
  data,
  ["workflowTelemetry", "window"],
  interval_error: "workflowTelemetry.window must be an ISO-8601 start/end interval",
  ordering_error: "workflowTelemetry.window end must be at or after start"
)
workflow_telemetry_window_end = iso8601_interval_end(dig_path(data, ["workflowTelemetry", "window"]))
require_evidence_ref(failures, data, ["workflowTelemetry", "evidenceRef"])

artifacts = data["artifacts"]
artifact_ids = {}
artifact_indexes = {}
artifact_recorded_times = {}
if !artifacts.is_a?(Array) || artifacts.empty?
  failures << "artifacts must include at least one target artifact entry"
else
  artifacts.each_with_index do |artifact, index|
    unless artifact.is_a?(Hash)
      failures << "artifacts[#{index}] must be an object"
      next
    end
    id = require_string(failures, data, ["artifacts", index, "id"])
    if id.is_a?(String)
      failures << "artifacts[#{index}].id must reference a concrete target artifact, not a placeholder" if placeholder?(id)
      if artifact_ids.key?(id)
        failures << "artifacts must not duplicate #{id}"
      else
        artifact_ids[id] = artifact
        artifact_indexes[id] = index
      end
    end
    kind = require_string(failures, data, ["artifacts", index, "kind"])
    failures << "artifacts[#{index}].kind must describe a concrete target artifact, not a placeholder" if placeholder?(kind)
    artifact_commit = require_string(failures, data, ["artifacts", index, "commit"])
    if artifact_commit.is_a?(String)
      failures << "artifacts[#{index}].commit must reference a concrete release commit, not a placeholder" if placeholder?(artifact_commit)
      if commit.is_a?(String) && !placeholder?(artifact_commit) && artifact_commit != commit
        failures << "artifacts[#{index}].commit must match manifest commit"
      end
    end
    artifact_run_id = require_string(failures, data, ["artifacts", index, "runId"])
    if artifact_run_id.is_a?(String)
      failures << "artifacts[#{index}].runId must reference a concrete target evidence run, not a placeholder" if placeholder?(artifact_run_id)
      if run_id.is_a?(String) && !placeholder?(artifact_run_id) && artifact_run_id != run_id
        failures << "artifacts[#{index}].runId must match manifest runId"
      end
    end
    uri = require_string(failures, data, ["artifacts", index, "uri"])
    failures << "artifacts[#{index}].uri must reference a concrete target artifact, not a placeholder" if placeholder?(uri)
    failures << "artifacts[#{index}].uri must reference a remote target artifact URI" unless remote_artifact_uri?(uri)
    failures << "artifacts[#{index}].uri must not embed secret-like query or fragment parameters" if secret_like_uri?(uri)
    failures << "artifacts[#{index}].uri must not embed credentials in URI userinfo" if userinfo_uri?(uri)
    artifact_recorded_at = require_string(failures, data, ["artifacts", index, "recordedAt"])
    parsed_artifact_recorded_at = nil
    begin
      parsed_artifact_recorded_at = Time.iso8601(artifact_recorded_at) if artifact_recorded_at.is_a?(String)
    rescue ArgumentError
      failures << "artifacts[#{index}].recordedAt must be ISO-8601"
    end
    if id.is_a?(String) && !placeholder?(id) && parsed_artifact_recorded_at
      artifact_recorded_times[id] = parsed_artifact_recorded_at
    end
    sha256 = artifact["sha256"]
    if !sha256.nil? && (!sha256.is_a?(String) || !sha256.match?(/\A[0-9a-f]{64}\z/i))
      failures << "artifacts[#{index}].sha256 must be a 64-character hex digest when present"
    end
  end
end

referenced_artifact_ids = {}
collect_required_evidence_refs(data).each do |ref_path, value|
  next if !value.is_a?(String) || placeholder?(value)
  referenced_artifact_ids[value] = true
  unless artifact_ids.key?(value)
    failures << "#{ref_path.join(".")} must reference an artifact id listed in artifacts"
  end
end

artifact_ids.each_key do |id|
  next if placeholder?(id)
  next if referenced_artifact_ids.key?(id)
  failures << "artifacts[#{artifact_indexes.fetch(id)}].id #{id} must be referenced by a required evidenceRef"
end

require_artifact_kind(failures, data, artifact_ids, ["strictVerifier", "evidenceRef"], "strict-verifier-log")
strict_verifier_ref = dig_path(data, ["strictVerifier", "evidenceRef"])
if strict_started_at && strict_completed_at && strict_verifier_ref.is_a?(String) && !placeholder?(strict_verifier_ref)
  strict_verifier_artifact = artifact_ids[strict_verifier_ref]
  if strict_verifier_artifact.is_a?(Hash)
    strict_verifier_artifact_recorded_at = strict_verifier_artifact["recordedAt"]
    begin
      artifact_recorded_at = Time.iso8601(strict_verifier_artifact_recorded_at) if strict_verifier_artifact_recorded_at.is_a?(String)
      if artifact_recorded_at && (artifact_recorded_at < strict_started_at || artifact_recorded_at > strict_completed_at)
        failures << "strictVerifier.evidenceRef artifact recordedAt must be within strictVerifier.startedAt/completedAt"
      end
    rescue ArgumentError
      # The artifact loop already reports the ISO-8601 violation.
    end
  end
end
require_artifact_kind(failures, data, artifact_ids, ["deployment", "evidenceRef"], "deployment-log")
require_artifact_kind(failures, data, artifact_ids, ["kubernetes", "evidenceRef"], "kubernetes-validation")
provider_entries = providers.is_a?(Array) ? providers : []
provider_entries.each_with_index do |provider, index|
  require_artifact_kind(failures, data, artifact_ids, ["providers", index, "evidenceRef"], "provider-live-rail")
  next unless provider.is_a?(Hash)

  name = provider["name"]
  ref = provider["evidenceRef"]
  next unless name.is_a?(String) && ref.is_a?(String)
  next if placeholder?(name) || placeholder?(ref)

  artifact = artifact_ids[ref]
  next unless artifact.is_a?(Hash) && artifact["kind"] == "provider-live-rail"

  artifact_provider = artifact["provider"]
  if !artifact_provider.is_a?(String) || placeholder?(artifact_provider) || artifact_provider.strip != name.strip
    failures << "providers[#{index}].evidenceRef must reference provider-specific live evidence for #{name.strip}"
  end
end
grpc_entries = grpc.is_a?(Array) ? grpc : []
grpc_entries.each_with_index do |_entry, index|
  require_artifact_kind(failures, data, artifact_ids, ["grpc", index, "evidenceRef"], "grpc-smoke-report")
end
require_artifact_kind(failures, data, artifact_ids, ["grpcSmokeReport", "evidenceRef"], "grpc-smoke-report")
grpc_smoke_report_ref = dig_path(data, ["grpcSmokeReport", "evidenceRef"])
if grpc_smoke_recorded_at && grpc_smoke_report_ref.is_a?(String) && !placeholder?(grpc_smoke_report_ref)
  grpc_smoke_artifact_recorded_at = artifact_recorded_times[grpc_smoke_report_ref]
  if grpc_smoke_artifact_recorded_at && grpc_smoke_artifact_recorded_at < grpc_smoke_recorded_at
    failures << "grpcSmokeReport.evidenceRef artifact recordedAt must be at or after grpcSmokeReport.recordedAt"
  end
end
require_artifact_kind(failures, data, artifact_ids, ["secretAudit", "evidenceRef"], "secret-audit")
require_artifact_kind(failures, data, artifact_ids, ["workflowTelemetry", "evidenceRef"], "workflow-telemetry")
workflow_telemetry_ref = dig_path(data, ["workflowTelemetry", "evidenceRef"])
if workflow_telemetry_window_end && workflow_telemetry_ref.is_a?(String) && !placeholder?(workflow_telemetry_ref)
  workflow_telemetry_artifact_recorded_at = artifact_recorded_times[workflow_telemetry_ref]
  if workflow_telemetry_artifact_recorded_at && workflow_telemetry_artifact_recorded_at < workflow_telemetry_window_end
    failures << "workflowTelemetry.evidenceRef artifact recordedAt must be at or after workflowTelemetry.window end"
  end
end

collect_skips(data).each do |skip_path, value|
  failures << "#{skip_path.join(".")} must be empty/false for final target evidence; got #{value.inspect}"
end

collect_secret_material(data).each do |finding|
  failures << finding
end

if failures.any?
  warn "[target-release-evidence] FAIL"
  failures.each { |failure| warn "[target-release-evidence] - #{failure}" }
  exit 1
end

puts "[target-release-evidence] PASS target evidence manifest"
puts "[target-release-evidence] environment: #{data.dig("environment", "name")} (#{data.dig("environment", "class")})"
puts "[target-release-evidence] commit: #{data["commit"]}"
puts "[target-release-evidence] run: #{data["runId"]}"
puts "[target-release-evidence] providers: #{providers.map { |provider| provider["name"] }.join(", ")}"
puts "[target-release-evidence] grpc services: #{grpc.map { |entry| entry["service"] }.join(", ")}"
puts "[target-release-evidence] grpc smoke report: #{data.dig("grpcSmokeReport", "evidenceRef")}"
puts "[target-release-evidence] workflow success rate: #{success_rate}"
puts "[target-release-evidence] artifacts: #{artifact_ids.size}"
RUBY
