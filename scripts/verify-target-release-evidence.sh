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
    {"id": "strict-verifier-log-or-artifact-id", "kind": "strict-verifier-log", "uri": "ci://run/strict-verifier", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "release-log-or-artifact-id", "kind": "deployment-log", "uri": "ci://run/deployment", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "stripe-provider-run-id", "kind": "provider-live-rail", "provider": "stripe", "uri": "ci://run/provider/stripe", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "alipay-provider-run-id", "kind": "provider-live-rail", "provider": "alipay", "uri": "ci://run/provider/alipay", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "wechatpay-provider-run-id", "kind": "provider-live-rail", "provider": "wechatpay", "uri": "ci://run/provider/wechatpay", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "grpc-smoke-log", "kind": "grpc-smoke-report", "uri": "ci://run/grpc-smoke", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "secret-audit-log", "kind": "secret-audit", "uri": "ci://run/secret-audit", "recordedAt": "2026-06-16T01:00:00Z"},
    {"id": "telemetry-dashboard-or-export", "kind": "workflow-telemetry", "uri": "ci://run/workflow-telemetry", "recordedAt": "2026-06-16T01:00:00Z"}
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
      {"id" => "TODO-strict-commercial-verifier-log", "kind" => "strict-verifier-log", "uri" => "TODO-strict-commercial-verifier-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-release-log-or-artifact-id", "kind" => "deployment-log", "uri" => "TODO-release-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-kubernetes-release-log-or-artifact-id", "kind" => "kubernetes-validation", "uri" => "TODO-kubernetes-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-stripe-provider-run-id", "kind" => "provider-live-rail", "provider" => "stripe", "uri" => "TODO-stripe-provider-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-alipay-provider-run-id", "kind" => "provider-live-rail", "provider" => "alipay", "uri" => "TODO-alipay-provider-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-wechatpay-provider-run-id", "kind" => "provider-live-rail", "provider" => "wechatpay", "uri" => "TODO-wechatpay-provider-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-target-grpc-smoke-report", "kind" => "grpc-smoke-report", "uri" => "TODO-target-grpc-smoke-report-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-secret-audit-log", "kind" => "secret-audit", "uri" => "TODO-secret-audit-log-uri", "recordedAt" => recorded_at.iso8601},
      {"id" => "TODO-telemetry-dashboard-or-export", "kind" => "workflow-telemetry", "uri" => "TODO-telemetry-dashboard-or-export-uri", "recordedAt" => recorded_at.iso8601}
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
require "json"
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

def local_target_host?(host)
  normalized = host.to_s.downcase.sub(/\A\[(.*)\]\z/, '\1')
  normalized == "localhost" ||
    normalized.end_with?(".localhost") ||
    normalized == "0.0.0.0" ||
    normalized == "::1" ||
    normalized.start_with?("127.")
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

def placeholder?(value)
  value.is_a?(String) && value.match?(/TODO|TBD|placeholder|example|sample|\/path\/outside\/git|release-log-or-artifact-id|strict-verifier-log-or-artifact-id|strict-commercial-verifier-log|provider-run-id|grpc-smoke-log|secret-audit-log|telemetry-dashboard-or-export/i)
end

def secret_like_uri?(value)
  value.is_a?(String) && value.match?(/[?&#](?:[^=&#]*[_-])?(?:token|secret|signature|api[_-]?key|access[_-]?key|credential|kubeconfig|private[_-]?key)=/i)
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

def collect_evidence_refs(value, path = [], refs = [])
  case value
  when Hash
    value.each do |key, child|
      child_path = path + [key]
      refs << [child_path, child] if key == "evidenceRef" && path.first != "artifacts"
      collect_evidence_refs(child, child_path, refs)
    end
  when Array
    value.each_with_index { |child, index| collect_evidence_refs(child, path + [index], refs) }
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
  failures << "strictVerifier.command must run scripts/verify-commercial-completion.sh" unless command.include?("scripts/verify-commercial-completion.sh")
  if command.match?(%r{(?:^|[[:space:];&|])COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=(?:true|'true'|"true")(?=$|[[:space:];&|])})
    failures << "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS"
  end
  %w[
    COMMERCIAL_COMPLETION_RUN_DEPLOY=true
    COMMERCIAL_COMPLETION_RUN_K8S=true
    COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true
    COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true
  ].each do |required_flag|
    unless command.match?(/(?:^|\s)#{Regexp.escape(required_flag)}(?:\s|$)/)
      failures << "strictVerifier.command must include #{required_flag}"
    end
  end
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
    failures << "grpc[#{index}].address is required" if blank?(entry["address"])
    service = entry["service"].to_s.strip
    unless service.empty?
      if grpc_entries_by_service.key?(service)
        failures << "grpc must not duplicate #{service} service evidence"
      else
        grpc_entries_by_service[service] = entry
      end
    end
    expected_port = expected_grpc_ports[service]
    if expected_port && !entry["address"].to_s.end_with?(":#{expected_port}")
      failures << "grpc[#{index}].address for #{service} must target port #{expected_port}"
    end
    if local_endpoint?(entry["address"])
      failures << "grpc[#{index}].address for #{service} must target a non-local service endpoint"
    end
    failures << "grpc[#{index}].generatedClient must be pass" unless entry["generatedClient"] == "pass"
    failures << "grpc[#{index}].evidenceRef is required" if blank?(entry["evidenceRef"])
    failures << "grpc[#{index}].evidenceRef must reference a concrete target artifact, not a placeholder" if placeholder?(entry["evidenceRef"])
  end
end

grpc_smoke_report = data["grpcSmokeReport"]
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
    Time.iso8601(smoke_recorded_at) if smoke_recorded_at.is_a?(String)
  rescue ArgumentError
    failures << "grpcSmokeReport.recordedAt must be ISO-8601"
  end
  require_string(failures, data, ["grpcSmokeReport", "timeout"])
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
      failures << "grpcSmokeReport.results[#{index}].address is required" if blank?(result["address"])
      manifest_entry = grpc_entries_by_service[service]
      if manifest_entry && result["address"].to_s != manifest_entry["address"].to_s
        failures << "grpcSmokeReport.results[#{index}].address must match grpc #{service} address"
      end
      expected_port = expected_grpc_ports[service]
      if expected_port && !result["address"].to_s.end_with?(":#{expected_port}")
        failures << "grpcSmokeReport.results[#{index}].address for #{service} must target port #{expected_port}"
      end
      if local_endpoint?(result["address"])
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
unless success_rate.is_a?(Numeric) && success_rate >= 0.99
  failures << "workflowTelemetry.successRate must be >= 0.99"
end
require_iso8601_interval(
  failures,
  data,
  ["workflowTelemetry", "window"],
  interval_error: "workflowTelemetry.window must be an ISO-8601 start/end interval",
  ordering_error: "workflowTelemetry.window end must be at or after start"
)
require_evidence_ref(failures, data, ["workflowTelemetry", "evidenceRef"])

artifacts = data["artifacts"]
artifact_ids = {}
artifact_indexes = {}
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
    uri = require_string(failures, data, ["artifacts", index, "uri"])
    failures << "artifacts[#{index}].uri must reference a concrete target artifact, not a placeholder" if placeholder?(uri)
    failures << "artifacts[#{index}].uri must reference a remote target artifact URI" unless remote_artifact_uri?(uri)
    failures << "artifacts[#{index}].uri must not embed secret-like query parameters" if secret_like_uri?(uri)
    artifact_recorded_at = require_string(failures, data, ["artifacts", index, "recordedAt"])
    begin
      Time.iso8601(artifact_recorded_at) if artifact_recorded_at.is_a?(String)
    rescue ArgumentError
      failures << "artifacts[#{index}].recordedAt must be ISO-8601"
    end
    sha256 = artifact["sha256"]
    if !sha256.nil? && (!sha256.is_a?(String) || !sha256.match?(/\A[0-9a-f]{64}\z/i))
      failures << "artifacts[#{index}].sha256 must be a 64-character hex digest when present"
    end
  end
end

referenced_artifact_ids = {}
collect_evidence_refs(data).each do |ref_path, value|
  next if !value.is_a?(String) || placeholder?(value)
  referenced_artifact_ids[value] = true
  unless artifact_ids.key?(value)
    failures << "#{ref_path.join(".")} must reference an artifact id listed in artifacts"
  end
end

artifact_ids.each_key do |id|
  next if placeholder?(id)
  next if referenced_artifact_ids.key?(id)
  failures << "artifacts[#{artifact_indexes.fetch(id)}].id #{id} must be referenced by at least one evidenceRef"
end

require_artifact_kind(failures, data, artifact_ids, ["strictVerifier", "evidenceRef"], "strict-verifier-log")
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
require_artifact_kind(failures, data, artifact_ids, ["secretAudit", "evidenceRef"], "secret-audit")
require_artifact_kind(failures, data, artifact_ids, ["workflowTelemetry", "evidenceRef"], "workflow-telemetry")

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
puts "[target-release-evidence] providers: #{providers.map { |provider| provider["name"] }.join(", ")}"
puts "[target-release-evidence] grpc services: #{grpc.map { |entry| entry["service"] }.join(", ")}"
puts "[target-release-evidence] grpc smoke report: #{data.dig("grpcSmokeReport", "evidenceRef")}"
puts "[target-release-evidence] workflow success rate: #{success_rate}"
puts "[target-release-evidence] artifacts: #{artifact_ids.size}"
RUBY
