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
      "evidenceRef": "provider-run-id"
    },
    {
      "name": "alipay",
      "mode": "live",
      "checkout": "pass",
      "refund": "pass",
      "payout": "pass",
      "reconciliation": "pass",
      "evidenceRef": "provider-run-id"
    },
    {
      "name": "wechatpay",
      "mode": "live",
      "checkout": "pass",
      "refund": "pass",
      "payout": "pass",
      "reconciliation": "pass",
      "evidenceRef": "provider-run-id"
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
  }
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
    }
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

def placeholder?(value)
  value.is_a?(String) && value.match?(/TODO|TBD|placeholder|example|sample|release-log-or-artifact-id|strict-verifier-log-or-artifact-id|strict-commercial-verifier-log|provider-run-id|grpc-smoke-log|secret-audit-log|telemetry-dashboard-or-export/i)
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
  path.first == "secretAudit" || path == ["kubernetes", "secretFileClass"]
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

%w[name class baseUrl recordedAt].each do |field|
  require_string(failures, data, ["environment", field])
end
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
  failures << "strictVerifier.command must not enable COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS" if command.include?("COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true")
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
if secret_file_class.is_a?(String) && secret_file_class.match?(/example|placeholder|sample/i)
  failures << "kubernetes.secretFileClass must describe a filled external secret, not an example or placeholder"
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
end

require_pass(failures, data, ["workflowTelemetry", "result"])
success_rate = dig_path(data, ["workflowTelemetry", "successRate"])
unless success_rate.is_a?(Numeric) && success_rate >= 0.99
  failures << "workflowTelemetry.successRate must be >= 0.99"
end
require_string(failures, data, ["workflowTelemetry", "window"])
require_evidence_ref(failures, data, ["workflowTelemetry", "evidenceRef"])

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
RUBY
