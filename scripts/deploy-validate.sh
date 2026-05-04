#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
base_url="${BASE_URL:-http://127.0.0.1:8080}"
keep_stack="${KEEP_STACK:-false}"

cd "$repo_root"

if ! command -v docker >/dev/null 2>&1; then
  echo "[deploy-validate] docker is required" >&2
  exit 127
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "[deploy-validate] docker compose is required" >&2
  exit 127
fi

if ! docker info >/dev/null 2>&1; then
  echo "[deploy-validate] docker daemon is not reachable for the current user/session" >&2
  echo "[deploy-validate] fix Docker daemon access, then rerun: bash scripts/deploy-validate.sh" >&2
  exit 2
fi

cleanup() {
  if [[ "$keep_stack" == "true" ]]; then
    echo "[deploy-validate] KEEP_STACK=true; leaving compose stack running"
    return
  fi

  docker compose down --remove-orphans
}
trap cleanup EXIT

echo "[deploy-validate] rendering compose config"
docker compose config >/dev/null

echo "[deploy-validate] building images"
docker compose build

echo "[deploy-validate] starting stack"
docker compose up -d

echo "[deploy-validate] running smoke against $base_url"
BASE_URL="$base_url" bash scripts/deploy-smoke.sh

echo "[deploy-validate] deployment validation ok"
