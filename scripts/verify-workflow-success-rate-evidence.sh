#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
go_cache="${GOCACHE:-/tmp/oblivious-go-cache}"
go_mod_cache="${GOMODCACHE:-/tmp/oblivious-gomod-cache}"

mkdir -p "$go_cache" "$go_mod_cache"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"

ensure_go_on_path() {
  local candidate
  local go_path=""
  local tool_bin_dir="$repo_root/.tmp/tool-bin"

  if command -v go >/dev/null 2>&1; then
    return
  fi

  for candidate in \
    "/mnt/c/Program Files/Go/bin/go" \
    "/mnt/c/Program Files/Go/bin/go.exe" \
    "/c/Program Files/Go/bin/go" \
    "/c/Program Files/Go/bin/go.exe" \
    "/usr/local/go/bin/go"; do
    if [[ -x "$candidate" ]]; then
      go_path="$candidate"
      break
    fi
  done

  if [[ -z "$go_path" ]]; then
    return
  fi

  mkdir -p "$tool_bin_dir"
  printf '#!/usr/bin/env sh\nexport WSLENV="${WSLENV:+$WSLENV:}TEST_DATABASE_URL:DATABASE_URL:OBLIVIOUS_REQUIRE_TEST_DATABASE:GOTOOLCHAIN:GOPROXY:GOSUMDB:GOPRIVATE:GONOSUMDB:GOFLAGS:HTTP_PROXY:HTTPS_PROXY:NO_PROXY"\nexec "%s" "$@"\n' "$go_path" > "$tool_bin_dir/go"
  chmod +x "$tool_bin_dir/go"
  export PATH="$tool_bin_dir:$PATH"
}

ensure_go_on_path

output=$(
  cd "$server_dir"
  go test ./internal/workflow -run TestServiceWorkflowSuccessRateEvidenceGate -count=1 -v
)

printf '%s\n' "$output"

if ! grep -Fq "workflow_success_rate_evidence executions=100 succeeded=100 failed=0 success_rate=1.0000 threshold=0.9900" <<<"$output"; then
  echo "[workflow-success-rate] missing deterministic 100/100 success-rate evidence line" >&2
  exit 1
fi
