#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

agent_addr="${AGENT_GRPC_ADDR:-${OBLIVIOUS_AGENT_GRPC_ADDR:-agent:50063}}"
workflow_addr="${WORKFLOW_GRPC_ADDR:-${OBLIVIOUS_WORKFLOW_GRPC_ADDR:-workflow:50064}}"
task_addr="${TASK_GRPC_ADDR:-${OBLIVIOUS_TASK_GRPC_ADDR:-task:50065}}"
timeout_value="${GRPC_SMOKE_TIMEOUT:-10s}"

run_smoke() {
  "$@" \
    --agent-addr "$agent_addr" \
    --workflow-addr "$workflow_addr" \
    --task-addr "$task_addr" \
    --timeout "$timeout_value"
}

if [[ -n "${OBLIVIOUS_GRPC_SMOKE_BIN:-}" ]]; then
  run_smoke "$OBLIVIOUS_GRPC_SMOKE_BIN"
elif command -v oblivious-grpc-smoke >/dev/null 2>&1; then
  run_smoke oblivious-grpc-smoke
else
  cd "$repo_root/src/server"
  run_smoke go run ./cmd/grpc-smoke
fi
