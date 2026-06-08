#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
go_cache="${GOCACHE:-/tmp/oblivious-go-cache}"
go_mod_cache="${GOMODCACHE:-/tmp/oblivious-gomod-cache}"

mkdir -p "$go_cache" "$go_mod_cache"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"

output=$(
  cd "$server_dir"
  go test ./internal/workflow -run TestServiceWorkflowSuccessRateEvidenceGate -count=1 -v
)

printf '%s\n' "$output"

if ! grep -Fq "workflow_success_rate_evidence executions=100 succeeded=100 failed=0 success_rate=1.0000 threshold=0.9900" <<<"$output"; then
  echo "[workflow-success-rate] missing deterministic 100/100 success-rate evidence line" >&2
  exit 1
fi
