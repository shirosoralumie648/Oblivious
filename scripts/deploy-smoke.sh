#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
base_url="${BASE_URL%/}"
attempts="${DEPLOY_SMOKE_ATTEMPTS:-30}"
sleep_seconds="${DEPLOY_SMOKE_SLEEP_SECONDS:-2}"
last_body_file=""
last_status=""

cleanup() {
  if [[ -n "$last_body_file" && -f "$last_body_file" ]]; then
    rm -f "$last_body_file"
  fi
}
trap cleanup EXIT

fail() {
  echo "[deploy-smoke] $*" >&2
  if [[ -n "$last_body_file" && -f "$last_body_file" ]]; then
    sed 's/^/[deploy-smoke] response: /' "$last_body_file" >&2 || true
  fi
  exit 1
}

status_is_2xx() {
  [[ "$1" =~ ^2[0-9][0-9]$ ]]
}

request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local url="${base_url}${path}"
  local status_file

  cleanup
  last_body_file=$(mktemp)
  status_file=$(mktemp)

  if command -v curl >/dev/null 2>&1; then
    if [[ -n "$body" ]]; then
      last_status=$(curl -sS -o "$last_body_file" -w "%{http_code}" \
        -X "$method" \
        -H "Content-Type: application/json" \
        --data "$body" \
        "$url" 2>"$status_file" || true)
    else
      last_status=$(curl -sS -o "$last_body_file" -w "%{http_code}" \
        -X "$method" \
        "$url" 2>"$status_file" || true)
    fi
    cat "$status_file" >&2 || true
    rm -f "$status_file"
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    if [[ -n "$body" ]]; then
      wget -qO "$last_body_file" \
        --server-response \
        --method="$method" \
        --header="Content-Type: application/json" \
        --body-data="$body" \
        "$url" 2>"$status_file" || true
    else
      wget -qO "$last_body_file" \
        --server-response \
        --method="$method" \
        "$url" 2>"$status_file" || true
    fi
    last_status=$(awk '/^  HTTP\\// { code=$2 } END { print code }' "$status_file")
    cat "$status_file" >&2 || true
    rm -f "$status_file"
    return
  fi

  rm -f "$status_file"
  fail "curl or wget is required"
}

probe_healthz() {
  request GET /healthz
  status_is_2xx "$last_status"
}

wait_for_healthz() {
  local attempt
  for attempt in $(seq 1 "$attempts"); do
    if probe_healthz; then
      echo "[deploy-smoke] healthz ok: ${base_url}/healthz"
      return
    fi

    if [[ "$attempt" -lt "$attempts" ]]; then
      echo "[deploy-smoke] waiting for healthz ($attempt/$attempts): ${base_url}/healthz"
      sleep "$sleep_seconds"
    fi
  done

  fail "healthz failed after $attempts attempts: ${base_url}/healthz status=${last_status:-none}"
}

probe_metrics() {
  request GET /metrics
  if ! status_is_2xx "$last_status"; then
    fail "metrics failed: ${base_url}/metrics status=${last_status:-none}"
  fi
  if ! rg -q "# HELP|go_|promhttp_metric_handler" "$last_body_file"; then
    fail "metrics response did not look like Prometheus output"
  fi
  echo "[deploy-smoke] metrics ok: ${base_url}/metrics"
}

probe_app_route() {
  request GET /api/v1/auth/me
  case "$last_status" in
    200|401|403)
      echo "[deploy-smoke] app route ok: ${base_url}/api/v1/auth/me status=$last_status"
      ;;
    404)
      fail "app route is not mounted: ${base_url}/api/v1/auth/me status=404"
      ;;
    *)
      fail "app route returned unexpected status: ${base_url}/api/v1/auth/me status=${last_status:-none}"
      ;;
  esac
}

probe_relay_route() {
  local payload
  payload='{"model":"gpt-4o-mini","messages":[{"role":"user","content":"deployment smoke"}],"max_tokens":1}'
  request POST /v1/chat/completions "$payload"
  case "$last_status" in
    400|401|403)
      echo "[deploy-smoke] relay route ok: ${base_url}/v1/chat/completions status=$last_status"
      ;;
    404)
      fail "Relay route is not mounted: ${base_url}/v1/chat/completions status=404"
      ;;
    503)
      if rg -q "no_available_channel|no healthy channel available" "$last_body_file"; then
        echo "[deploy-smoke] relay route ok: ${base_url}/v1/chat/completions status=$last_status no_available_channel"
      else
        fail "Relay route reached an upstream/provider failure instead of local no-channel policy handling: status=$last_status"
      fi
      ;;
    502|504)
      fail "Relay route reached an upstream/provider failure instead of local policy handling: status=$last_status"
      ;;
    000|"")
      fail "Relay route did not return an HTTP response"
      ;;
    *)
      fail "Relay route returned unexpected status: ${base_url}/v1/chat/completions status=$last_status"
      ;;
  esac
}

wait_for_healthz
probe_metrics
probe_app_route
probe_relay_route

echo "[deploy-smoke] runtime smoke ok: ${base_url}"
