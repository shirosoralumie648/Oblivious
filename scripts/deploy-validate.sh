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
build_log=$(mktemp)
if ! docker compose build 2>&1 | tee "$build_log"; then
  if grep -qiE 'registry-1\.docker\.io|failed to resolve source metadata|proxy\.golang\.org|sum\.golang\.org|goproxy\.cn|goproxy\.io|mirrors\.aliyun\.com|github\.com/|i/o timeout|connection refused|connection reset by peer|TLS handshake timeout|connect: network is unreachable' "$build_log"; then
    echo "[deploy-validate] Docker image build could not reach required registry or module metadata" >&2
    echo "[deploy-validate] configure Docker daemon registry/proxy access or set OBLIVIOUS_IMAGE_REGISTRY_PREFIX/OBLIVIOUS_GOPROXY, then rerun this script" >&2
    echo "[deploy-validate] see docs/release/deployment-runtime-remediation.md" >&2
  fi

  if grep -qi 'docker-credential-desktop' "$build_log"; then
    echo "[deploy-validate] Docker client references docker-credential-desktop, but the helper is unavailable" >&2
    echo "[deploy-validate] remove or replace the stale credsStore entry in ~/.docker/config.json" >&2
  fi

  rm -f "$build_log"
  exit 3
fi
rm -f "$build_log"

echo "[deploy-validate] starting stack"
docker compose up -d

echo "[deploy-validate] running smoke against $base_url"
BASE_URL="$base_url" bash scripts/deploy-smoke.sh

echo "[deploy-validate] deployment validation ok"
