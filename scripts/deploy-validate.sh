#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_host_port="${OBLIVIOUS_SERVER_HOST_PORT:-8080}"
base_url="${BASE_URL:-http://127.0.0.1:${server_host_port}}"
keep_stack="${KEEP_STACK:-false}"
docker_up_timeout_seconds="${DEPLOY_VALIDATE_DOCKER_UP_TIMEOUT_SECONDS:-600}"

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

wait_for_compose_dependencies() {
  local attempt
  local attempts="${DEPLOY_VALIDATE_DEP_ATTEMPTS:-30}"
  local sleep_seconds="${DEPLOY_VALIDATE_DEP_SLEEP_SECONDS:-2}"

  for attempt in $(seq 1 "$attempts"); do
    if docker compose exec -T postgres pg_isready -h 127.0.0.1 -U oblivious -d oblivious >/dev/null 2>&1 &&
      [[ "$(docker compose exec -T redis redis-cli ping 2>/dev/null | tr -d '\r')" == "PONG" ]] &&
      docker compose exec -T qdrant bash -ec 'exec 3<>/dev/tcp/127.0.0.1/6333; printf "GET /healthz HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n" >&3; grep -q "200 OK" <&3' >/dev/null 2>&1 &&
      docker compose exec -T clickhouse sh -ec 'clickhouse-client --user "${CLICKHOUSE_USER:-oblivious}" --password "${CLICKHOUSE_PASSWORD:-oblivious}" --query "SELECT 1"' >/dev/null 2>&1; then
      echo "[deploy-validate] compose dependencies ready"
      return
    fi

    if [[ "$attempt" -lt "$attempts" ]]; then
      echo "[deploy-validate] waiting for compose dependencies ($attempt/$attempts)"
      sleep "$sleep_seconds"
    fi
  done

  echo "[deploy-validate] compose dependencies did not become healthy" >&2
  docker compose ps >&2 || true
  exit 4
}

run_compose_up() {
  local status
  local services=("$@")

  set +e
  if command -v timeout >/dev/null 2>&1; then
    timeout "$docker_up_timeout_seconds" docker compose up -d "${services[@]}"
    status=$?
  else
    docker compose up -d "${services[@]}"
    status=$?
  fi
  set -e

  if [[ "$status" -eq 124 ]]; then
    echo "[deploy-validate] docker compose up timed out after ${docker_up_timeout_seconds} while starting: ${services[*]}" >&2
    echo "[deploy-validate] pre-pull required images or configure registry access; set OBLIVIOUS_IMAGE_REGISTRY_PREFIX/OBLIVIOUS_POSTGRES_IMAGE for restricted networks" >&2
    echo "[deploy-validate] see docs/release/deployment-runtime-remediation.md" >&2
    exit 5
  fi

  if [[ "$status" -ne 0 ]]; then
    echo "[deploy-validate] docker compose up failed while starting: ${services[*]}" >&2
    echo "[deploy-validate] see docs/release/deployment-runtime-remediation.md" >&2
    exit "$status"
  fi
}

echo "[deploy-validate] rendering compose config"
docker compose config >/dev/null

echo "[deploy-validate] building images"
build_log=$(mktemp)
if ! docker compose build 2>&1 | tee "$build_log"; then
  if grep -qiE 'registry-1\.docker\.io|failed to resolve source metadata|proxy\.golang\.org|sum\.golang\.org|goproxy\.cn|goproxy\.io|mirrors\.aliyun\.com|github\.com/|i/o timeout|connection refused|connection reset by peer|TLS handshake timeout|connect: network is unreachable' "$build_log"; then
    echo "[deploy-validate] Docker image build could not reach required registry or module metadata" >&2
    echo "[deploy-validate] configure Docker daemon registry/proxy access or set OBLIVIOUS_IMAGE_REGISTRY_PREFIX/OBLIVIOUS_POSTGRES_IMAGE/OBLIVIOUS_GOPROXY, then rerun this script" >&2
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

echo "[deploy-validate] starting data services"
run_compose_up postgres redis qdrant clickhouse
wait_for_compose_dependencies

echo "[deploy-validate] applying migrations"
docker compose run --rm --no-deps oblivious-server /usr/local/bin/oblivious-migrate

echo "[deploy-validate] applying ClickHouse migrations"
docker compose run --rm clickhouse-init

echo "[deploy-validate] starting application stack"
run_compose_up oblivious-server oblivious-web

echo "[deploy-validate] running smoke against $base_url"
BASE_URL="$base_url" bash scripts/deploy-smoke.sh

echo "[deploy-validate] deployment validation ok"
