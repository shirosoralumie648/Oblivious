#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
health_url="${BASE_URL%/}/healthz"
attempts="${DEPLOY_SMOKE_ATTEMPTS:-30}"
sleep_seconds="${DEPLOY_SMOKE_SLEEP_SECONDS:-2}"

request_healthz() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS "$health_url" >/dev/null
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -qO- "$health_url" >/dev/null
    return
  fi

  echo "[deploy-smoke] curl or wget is required" >&2
  return 127
}

for attempt in $(seq 1 "$attempts"); do
  if request_healthz; then
    echo "[deploy-smoke] healthz ok: $health_url"
    exit 0
  fi

  if [[ "$attempt" -lt "$attempts" ]]; then
    echo "[deploy-smoke] waiting for healthz ($attempt/$attempts): $health_url"
    sleep "$sleep_seconds"
  fi
done

echo "[deploy-smoke] healthz failed after $attempts attempts: $health_url" >&2
exit 1
