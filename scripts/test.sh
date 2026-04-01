#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"

mkdir -p "$corepack_home"
mkdir -p "$go_cache" "$go_mod_cache"
export COREPACK_HOME="$corepack_home"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"

ran_any=false

if [[ -d "$web_dir" ]]; then
  echo "[test] Running web tests."
  pnpm --dir "$web_dir" test
  ran_any=true
else
  echo "[test] Skipping web tests: src/web not present."
fi

if [[ -d "$server_dir" ]]; then
  echo "[test] Running server tests."
  (cd "$server_dir" && go test ./...)
  ran_any=true
else
  echo "[test] Skipping server tests: src/server not present."
fi

if [[ "$ran_any" == false ]]; then
  echo "[test] Nothing to run yet."
fi
