#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
openapi_file="$repo_root/docs/api/openapi.yaml"

require_path() {
  local path="$1"
  if ! grep -Fq -- "  $path:" "$openapi_file"; then
    echo "[openapi-contract] missing path: $path" >&2
    exit 1
  fi
}

require_public_security_empty() {
  local path="$1"
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    path = ARGV.fetch(1)
    spec = YAML.load_file(file)
    post = spec.fetch("paths", {}).fetch(path, {}).fetch("post", nil)
    unless post && post["security"] == []
      warn "[openapi-contract] public POST #{path} must declare security: []"
      exit 1
    end
  ' "$openapi_file" "$path"
}

required_paths=(
  "/api/v1/agent/runs"
  "/api/v1/agent/runs/{runId}"
  "/api/v1/agent/runs/{runId}/approve-tool"
  "/api/v1/agent/runs/{runId}/reject-tool"
  "/api/v1/agent/runs/{runId}/retry-tool"
  "/api/v1/agent/runs/{runId}/approve-plan-step"
  "/api/v1/agent/runs/{runId}/execute-plan-step"
  "/api/v1/channels"
  "/api/v1/channels/{channelId}"
  "/api/v1/channels/{channelId}/status"
  "/api/v1/channels/{channelId}/test"
  "/api/v1/channels/{channelId}/send"
  "/api/v1/channels/{channelId}/messages"
  "/api/v1/channels/{channelId}/failed-messages"
  "/api/v1/channels/{channelId}/retry-failed-messages"
  "/api/v1/channels/webhook/{channelId}"
  "/api/v1/workflows"
  "/api/v1/workflows/semantic-matches"
  "/api/v1/workflows/conversation-matches"
  "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"
  "/api/v1/workflows/{workflowId}"
  "/api/v1/workflows/{workflowId}/execute"
  "/api/v1/workflows/{workflowId}/webhook"
  "/api/v1/workflows/{workflowId}/versions"
  "/api/v1/workflows/{workflowId}/branches"
  "/api/v1/workflows/{workflowId}/branches/{branchId}/publish"
  "/api/v1/workflows/{workflowId}/branches/{branchId}/merge"
  "/api/v1/workflows/{workflowId}/rollback"
  "/api/v1/workflows/{workflowId}/test-node"
  "/api/v1/workflows/{workflowId}/executions"
  "/api/v1/workflows/{workflowId}/executions/{executionId}"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/debug-snapshot"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/decision"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/pause"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/resume"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/cancel"
)

for path in "${required_paths[@]}"; do
  require_path "$path"
done

require_public_security_empty "/api/v1/channels/webhook/{channelId}"
require_public_security_empty "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"

echo "[openapi-contract] required Agent run, publishing channel, and Workflow paths are documented."
