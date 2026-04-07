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
Usage: bash scripts/check.sh [all|docs|web|server]
EOF
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
    rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
    rg -q --fixed-strings "$var_name" "$contracts_file"
  done

  for var_name in "${backend_vars[@]}"; do
    rg -q --fixed-strings "$var_name" "$repo_root/config/.env.example"
    rg -q --fixed-strings "$var_name" "$contracts_file"
    rg -q --fixed-strings "$var_name" "$repo_root/src/server/internal/config/config.go"
  done

  rg -q --fixed-strings "bash scripts/check.sh" "$contracts_file"
  rg -q --fixed-strings "bash scripts/test.sh" "$contracts_file"

  echo "[check] Verifying mainline workspace boundary."
  rg -q --fixed-strings "packages:" "$workspace_file"
  rg -q --fixed-strings "  - src/web" "$workspace_file"

  if rg -q --fixed-strings "lobehub" "$workspace_file"; then
    echo "[check] Unexpected workspace member: lobehub" >&2
    exit 1
  fi

  if rg -q --fixed-strings "new-api" "$workspace_file"; then
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

  echo "[check] Running server unit checks."
  (cd "$server_dir" && go test ./internal/config ./internal/chat ./internal/knowledge ./internal/task ./internal/console)
}

case "$target" in
  all)
    run_docs_checks
    run_web_checks
    run_server_checks
    ;;
  docs)
    run_docs_checks
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
