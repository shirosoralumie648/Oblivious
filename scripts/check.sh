#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
workspace_file="$repo_root/pnpm-workspace.yaml"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
target="${1:-all}"
node_bin=""

usage() {
  cat <<'EOF'
Usage: bash scripts/check.sh [all|docs|web|server|relay-security|security]
EOF
}

assert_contains() {
  local pattern="$1"
  local path="$2"
  grep -Fq -- "$pattern" "$path"
}

mkdir -p "$corepack_home"
mkdir -p "$go_cache" "$go_mod_cache"
export COREPACK_HOME="$corepack_home"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"

command_works() {
  local command_name="$1"

  command -v "$command_name" >/dev/null 2>&1 && "$command_name" --version >/dev/null 2>&1
}

windows_path() {
  local path="$1"

  if command -v wslpath >/dev/null 2>&1; then
    wslpath -w "$path"
    return
  fi
  printf '%s\n' "$path"
}

windows_pnpm_args() {
  local convert_next=false
  local arg

  for arg in "$@"; do
    if [[ "$convert_next" == "true" ]]; then
      windows_path "$arg"
      convert_next=false
      continue
    fi

    printf '%s\n' "$arg"
    if [[ "$arg" == "--dir" || "$arg" == "-C" ]]; then
      convert_next=true
    fi
  done
}

if command_works pnpm; then
  :
elif command_works corepack; then
  pnpm() {
    corepack pnpm "$@"
  }
elif command -v cmd.exe >/dev/null 2>&1 && [[ -f "/mnt/c/Program Files/nodejs/corepack.cmd" ]]; then
  pnpm() {
    local args=()
    local arg

    while IFS= read -r arg; do
      args+=("$arg")
    done < <(windows_pnpm_args "$@")

    cmd.exe /c "C:\\Program Files\\nodejs\\corepack.cmd" pnpm "${args[@]}"
  }
else
  echo "[check] pnpm or corepack is required for web checks." >&2
  exit 127
fi

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

resolve_python_bin() {
  local candidate
  local resolved

  if [[ -n "${PYTHON:-}" ]]; then
    return
  fi

  for candidate in \
    python \
    python3 \
    "/c/ProgramData/anaconda3/python" \
    "/c/ProgramData/anaconda3/python.exe" \
    "/mnt/c/ProgramData/anaconda3/python.exe"; do
    resolved="$(command -v "$candidate" 2>/dev/null || true)"
    if [[ -n "$resolved" ]] && "$resolved" --version >/dev/null 2>&1; then
      export PYTHON="$resolved"
      return
    fi
    if [[ -x "$candidate" ]] && "$candidate" --version >/dev/null 2>&1; then
      export PYTHON="$candidate"
      return
    fi
  done
}

resolve_python_bin

if ! command -v rg >/dev/null 2>&1; then
  for tool_bin_dir in \
    /mnt/c/Users/*/AppData/Local/Temp/codex-nativebin-wsl \
    /c/Users/*/AppData/Local/Temp/codex-nativebin-wsl; do
    if [[ -x "$tool_bin_dir/rg" || -x "$tool_bin_dir/rg.exe" ]]; then
      export PATH="$tool_bin_dir:$PATH"
      break
    fi
  done
fi

resolve_node_bin() {
  if [[ -n "$node_bin" ]]; then
    return
  fi

  if [[ -n "${NODE_BIN:-}" ]]; then
    node_bin="$NODE_BIN"
    return
  fi

  if command -v node >/dev/null 2>&1; then
    node_bin="$(command -v node)"
    return
  fi

  for candidate in \
    "/mnt/c/Program Files/nodejs/node.exe" \
    "/c/Program Files/nodejs/node.exe"; do
    if [[ -x "$candidate" ]]; then
      node_bin="$candidate"
      return
    fi
  done

  echo "[check] node is required for docs checks. Set NODE_BIN or install Node.js." >&2
  exit 127
}

run_node_script() {
  local script_path="$1"
  shift

  resolve_node_bin
  if [[ "$node_bin" == *.exe && "$(command -v wslpath || true)" != "" ]]; then
    script_path="$(wslpath -w "$script_path")"
  fi

  "$node_bin" "$script_path" "$@"
}

run_node_check() {
  local script_path="$1"

  resolve_node_bin
  if [[ "$node_bin" == *.exe && "$(command -v wslpath || true)" != "" ]]; then
    script_path="$(wslpath -w "$script_path")"
  fi

  "$node_bin" --check "$script_path"
}

run_docs_checks() {
  local contracts_file
  local frontend_vars
  local backend_vars

  echo "[check] Verifying release assets."
  bash "$repo_root/scripts/verify-quality-gates.sh"

  echo "[check] Verifying commercial verifier script syntax."
  run_node_check "$repo_root/scripts/verify-commercial-preflight.mjs"
  run_node_check "$repo_root/scripts/verify-commercial-local.mjs"

  echo "[check] Verifying commercial preflight behavior."
  bash "$repo_root/scripts/verify-commercial-preflight-fixtures.sh"

  echo "[check] Verifying target release evidence behavior."
  bash "$repo_root/scripts/verify-target-release-evidence-fixtures.sh"

  echo "[check] Verifying target release evidence assembler."
  bash "$repo_root/scripts/assemble-target-release-evidence-fixtures.sh"

  echo "[check] Verifying target release artifact collection."
  bash "$repo_root/scripts/collect-target-release-artifacts-fixtures.sh"

  echo "[check] Verifying target release digest computation."
  bash "$repo_root/scripts/compute-target-release-digests-fixtures.sh"

  echo "[check] Verifying strict verifier evidence collector."
  bash "$repo_root/scripts/collect-strict-verifier-evidence-fixtures.sh"

  echo "[check] Verifying deployment evidence collector."
  bash "$repo_root/scripts/collect-deployment-evidence-fixtures.sh"

  echo "[check] Verifying Kubernetes evidence collector."
  bash "$repo_root/scripts/collect-kubernetes-evidence-fixtures.sh"

  echo "[check] Verifying workflow telemetry evidence collector."
  bash "$repo_root/scripts/collect-workflow-telemetry-evidence-fixtures.sh"

  echo "[check] Verifying request-log observability evidence collector."
  bash "$repo_root/scripts/collect-request-log-observability-evidence-fixtures.sh"

  echo "[check] Verifying RAG indexing evidence collector."
  bash "$repo_root/scripts/collect-rag-indexing-evidence-fixtures.sh"

  echo "[check] Verifying Relay Realtime evidence collector."
  bash "$repo_root/scripts/collect-relay-realtime-evidence-fixtures.sh"

  echo "[check] Verifying Relay Batch evidence collector."
  bash "$repo_root/scripts/collect-relay-batch-evidence-fixtures.sh"

  echo "[check] Verifying marketplace payout evidence collector."
  bash "$repo_root/scripts/collect-marketplace-payout-evidence-fixtures.sh"

  echo "[check] Verifying marketplace governance evidence collector."
  bash "$repo_root/scripts/collect-marketplace-governance-evidence-fixtures.sh"

  echo "[check] Verifying provider runtime config evidence collector."
  bash "$repo_root/scripts/collect-provider-runtime-config-evidence-fixtures.sh"

  echo "[check] Verifying provider live rail evidence collector."
  bash "$repo_root/scripts/collect-provider-live-rail-evidence-fixtures.sh"

  echo "[check] Verifying gRPC smoke report evidence collector."
  bash "$repo_root/scripts/collect-grpc-smoke-report-evidence-fixtures.sh"

  echo "[check] Verifying secret audit evidence collector."
  bash "$repo_root/scripts/collect-secret-audit-evidence-fixtures.sh"

  echo "[check] Verifying microservice database evidence collector."
  bash "$repo_root/scripts/collect-microservice-database-evidence-fixtures.sh"

  echo "[check] Verifying observability dashboard."
  run_node_script "$repo_root/scripts/verify-observability-dashboard.mjs"

  echo "[check] Verifying Kubernetes recovery policy."
  bash "$repo_root/scripts/verify-k8s-recovery-policy.sh"

  echo "[check] Verifying deployment operations contract."
  bash "$repo_root/scripts/verify-deployment-operations-contract.sh"

  echo "[check] Verifying schema coverage."
  bash "$repo_root/scripts/verify-schema-coverage.sh"

  echo "[check] Verifying workflow success-rate evidence."
  bash "$repo_root/scripts/verify-workflow-success-rate-evidence.sh"

  echo "[check] Verifying commercial DB evidence profile list."
  bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh"
  bash "$repo_root/scripts/verify-commercial-db-evidence-profiles-fixtures.sh"

  echo "[check] Verifying fusion evidence pack."
  bash "$repo_root/scripts/verify-fusion-evidence-pack.sh"

  echo "[check] Verifying docs and env consistency."
  contracts_file="$repo_root/docs/architecture/current-system-contracts.md"
  frontend_vars=(
    WEB_PORT
    WEB_API_BASE_URL
  )
  backend_vars=(
    SERVER_PORT
    APP_ENV
    CORS_ALLOWED_ORIGINS
    DATABASE_URL
    SESSION_SECRET
    SESSION_COOKIE_NAME
    SESSION_COOKIE_SECURE
    LLM_BASE_URL
    LLM_API_KEY
    LLM_TIMEOUT_MS
    MODEL_DEFAULT_NAME
  )

  for var_name in "${frontend_vars[@]}"; do
    assert_contains "$var_name" "$repo_root/config/.env.example"
    assert_contains "$var_name" "$contracts_file"
  done

  for var_name in "${backend_vars[@]}"; do
    assert_contains "$var_name" "$repo_root/config/.env.example"
    assert_contains "$var_name" "$contracts_file"
    assert_contains "$var_name" "$repo_root/src/server/internal/config/config.go"
  done

  assert_contains "bash scripts/check.sh" "$contracts_file"
  assert_contains "bash scripts/test.sh" "$contracts_file"

  echo "[check] Verifying mainline workspace boundary."
  assert_contains "packages:" "$workspace_file"
  assert_contains "  - src/web" "$workspace_file"

  if grep -Fq -- "lobehub" "$workspace_file"; then
    echo "[check] Unexpected workspace member: lobehub" >&2
    exit 1
  fi

  if grep -Fq -- "new-api" "$workspace_file"; then
    echo "[check] Unexpected workspace member: new-api" >&2
    exit 1
  fi
}

run_web_checks() {
  if [[ ! -d "$web_dir" ]]; then
    echo "[check] Skipping web build: src/web not present."
    return
  fi

  echo "[check] Running web build."
  pnpm --dir "$web_dir" build
}

run_server_checks() {
  if [[ ! -d "$server_dir" ]]; then
    echo "[check] Skipping server compile checks: src/server not present."
    return
  fi

  echo "[check] Running server compile checks."
  (cd "$server_dir" && go test ./... -run '^$' -count=1)
}

run_relay_security_checks() {
  echo "[check] Verifying Relay security boundary."
  bash "$repo_root/scripts/verify-relay-security.sh"
}

run_dependency_security_checks() {
  echo "[check] Verifying dependency security."
  bash "$repo_root/scripts/verify-dependency-security.sh"
}

case "$target" in
  all)
    run_docs_checks
    run_relay_security_checks
    run_dependency_security_checks
    run_web_checks
    run_server_checks
    ;;
  docs)
    run_docs_checks
    ;;
  relay-security)
    run_relay_security_checks
    ;;
  security)
    run_dependency_security_checks
    ;;
  web)
    run_web_checks
    ;;
  server)
    run_server_checks
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
