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
)

for path in "${required_paths[@]}"; do
  require_path "$path"
done

echo "[openapi-contract] required Agent run and publishing channel paths are documented."
