#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_entry=""
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"

mkdir -p "$corepack_home"
export COREPACK_HOME="$corepack_home"

has_web=false
has_server=false

if [[ -d "$web_dir" ]]; then
  has_web=true
fi

if [[ -d "$repo_root/src/server/cmd/server" ]]; then
  server_entry="$repo_root/src/server/cmd/server"
  has_server=true
elif [[ -d "$repo_root/src/server/cmd/api" ]]; then
  server_entry="$repo_root/src/server/cmd/api"
  has_server=true
fi

if [[ "$has_web" == false && "$has_server" == false ]]; then
  echo "[dev] Skipping: neither src/web nor src/server exists yet."
  exit 0
fi

if [[ "$has_web" == true && "$has_server" == false ]]; then
  echo "[dev] Starting web only (src/server not present yet)."
  exec pnpm --dir "$web_dir" dev
fi

if [[ "$has_web" == false && "$has_server" == true ]]; then
  echo "[dev] Starting server only (src/web not present yet)."
  exec go run "$server_entry"
fi

echo "[dev] Starting web and server."
pnpm --dir "$web_dir" dev &
web_pid=$!

cleanup() {
  if kill -0 "$web_pid" >/dev/null 2>&1; then
    kill "$web_pid" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT INT TERM
go run "$server_entry"
