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

usage() {
  cat <<'EOF'
Usage: bash scripts/check.sh [all|docs|web|server|relay-security]
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

run_docs_checks() {
  local contracts_file
  local frontend_vars
  local backend_vars

  echo "[check] Verifying release assets."
  bash "$repo_root/scripts/verify-quality-gates.sh"

  echo "[check] Verifying observability dashboard."
  node "$repo_root/scripts/verify-observability-dashboard.mjs"

  echo "[check] Verifying Kubernetes recovery policy."
  bash "$repo_root/scripts/verify-k8s-recovery-policy.sh"

  echo "[check] Verifying OpenAPI contract coverage."
  bash "$repo_root/scripts/verify-openapi-contract.sh"

  echo "[check] Verifying migration contract coverage."
  bash "$repo_root/scripts/verify-migration-contract.sh"

  echo "[check] Verifying schema coverage."
  bash "$repo_root/scripts/verify-schema-coverage.sh"

  echo "[check] Verifying workflow success-rate evidence."
  bash "$repo_root/scripts/verify-workflow-success-rate-evidence.sh"

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
    echo "[check] Skipping server unit checks: src/server not present."
    return
  fi

  echo "[check] Running server release checks."
  (cd "$server_dir" && go test ./... -count=1)
}

run_relay_security_checks() {
  echo "[check] Verifying Relay security boundary."
  bash "$repo_root/scripts/verify-relay-security.sh"
}

case "$target" in
  all)
    run_docs_checks
    run_relay_security_checks
    run_web_checks
    run_server_checks
    ;;
  docs)
    run_docs_checks
    ;;
  relay-security)
    run_relay_security_checks
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
