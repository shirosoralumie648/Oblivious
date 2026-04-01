#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"

mkdir -p "$corepack_home"
export COREPACK_HOME="$corepack_home"

ran_any=false

if [[ -d "$web_dir" ]]; then
  echo "[check] Running web checks."
  pnpm --dir "$web_dir" check
  ran_any=true
else
  echo "[check] Skipping web checks: src/web not present."
fi

if [[ -d "$server_dir" ]]; then
  echo "[check] Running server checks."
  (cd "$server_dir" && go test ./...)
  ran_any=true
else
  echo "[check] Skipping server checks: src/server not present."
fi

if [[ "$ran_any" == false ]]; then
  echo "[check] Nothing to run yet."
fi
