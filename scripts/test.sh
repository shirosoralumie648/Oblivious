#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
target="${1:-all}"

usage() {
  cat <<'EOF'
Usage: bash scripts/test.sh [all|web|server]
EOF
}

mkdir -p "$corepack_home"
mkdir -p "$go_cache" "$go_mod_cache"
export COREPACK_HOME="$corepack_home"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"

run_web_tests() {
  if [[ ! -d "$web_dir" ]]; then
    echo "[test] Skipping web tests: src/web not present."
    return
  fi

  echo "[test] Running web tests."
  pnpm --dir "$web_dir" test
}

run_server_tests() {
  if [[ ! -d "$server_dir" ]]; then
    echo "[test] Skipping server tests: src/server not present."
    return
  fi

  echo "[test] Running server unit tests."
  (cd "$server_dir" && go test ./internal/config ./internal/chat ./internal/knowledge ./internal/task ./internal/console)

  echo "[test] Running server integration tests."
  if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
    echo "[test] Skipping server integration tests: TEST_DATABASE_URL not set."
    return
  fi

  (cd "$server_dir" && go test ./internal/http)
}

case "$target" in
  all)
    run_web_tests
    run_server_tests
    ;;
  web)
    run_web_tests
    ;;
  server)
    run_server_tests
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
