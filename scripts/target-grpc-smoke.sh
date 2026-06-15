#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

agent_addr="${AGENT_GRPC_ADDR:-${OBLIVIOUS_AGENT_GRPC_ADDR:-agent:50063}}"
workflow_addr="${WORKFLOW_GRPC_ADDR:-${OBLIVIOUS_WORKFLOW_GRPC_ADDR:-workflow:50064}}"
task_addr="${TASK_GRPC_ADDR:-${OBLIVIOUS_TASK_GRPC_ADDR:-task:50065}}"
timeout_value="${GRPC_SMOKE_TIMEOUT:-10s}"

cd "$repo_root/src/server"
go run ./cmd/grpc-smoke \
  --agent-addr "$agent_addr" \
  --workflow-addr "$workflow_addr" \
  --task-addr "$task_addr" \
  --timeout "$timeout_value"
