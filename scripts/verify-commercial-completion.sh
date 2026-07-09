#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
go_toolchain="${COMMERCIAL_COMPLETION_GOTOOLCHAIN:-go1.26.5}"
postgres_image="${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
playwright_browsers_path="${PLAYWRIGHT_BROWSERS_PATH:-$repo_root/.tmp/ms-playwright}"
windows_corepack_node="/mnt/c/Program Files/nodejs/node.exe"
windows_corepack_script="C:\\Program Files\\nodejs\\node_modules\\corepack\\dist\\corepack.js"
db_server_suite_container_name=""

usage() {
  cat <<'EOF'
Usage: bash scripts/verify-commercial-completion.sh [--help]

Runs the strict Phase 30 commercial completion verifier.

Required for strict final readiness:
  TEST_DATABASE_URL
    PostgreSQL test database URL. Must support pgvector for Knowledge RAG tests
    and the serial DB-backed full Go suite.
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true
    Run bash scripts/deploy-validate.sh as part of the final verifier.
  COMMERCIAL_COMPLETION_RUN_K8S=true
    Run bash scripts/k8s-validate.sh as part of the final verifier.
    Requires kubectl, a reachable Kubernetes context, and OBLIVIOUS_K8S_SECRET_FILE.
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true
    Run bash scripts/backup-restore-smoke.sh as part of the final verifier.
  COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true
    Run bash scripts/verify-target-release-evidence.sh as part of the final verifier.
    Requires OBLIVIOUS_TARGET_EVIDENCE_FILE pointing at an external JSON evidence manifest
    for live provider rails, deployed gRPC reachability, target secret audit, failover,
    production workflow telemetry, ClickHouse request-log observability, durable RAG
    indexing, Marketplace payout lifecycle, provider runtime configuration, and
    external-filled microservice database proof.
  OBLIVIOUS_TARGET_ARTIFACT_DIR
    Directory outside git containing downloaded target artifact bodies named <artifact-id>.json.
    The strict final verifier validates these bodies against the manifest SHA-256,
    lineage metadata, and required proof fields.

Optional:
  COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true
    Allow deploy, Kubernetes, and backup/restore checks to be skipped for local partial evidence.
    A run with this flag is not final commercial readiness evidence.
  PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
    Existing Chromium/Chrome executable for Playwright when browser cache is unavailable.
  COREPACK_HOME, GOCACHE, GOMODCACHE
    Override local tool caches.
  COMMERCIAL_COMPLETION_GOTOOLCHAIN
    Override the Go toolchain used by the final verifier. Defaults to go1.26.5.
  BACKUP_SMOKE_SOURCE_DATABASE_URL, BACKUP_SMOKE_RESTORE_DATABASE_URL
    External disposable databases for backup/restore smoke.
  COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL
    Optional separate PostgreSQL URL for the DB-backed full Go suite. If unset,
    the verifier starts a disposable pgvector PostgreSQL container so the full
    suite is isolated from DB evidence profile fixtures.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

mkdir -p "$corepack_home" "$go_cache" "$go_mod_cache"
mkdir -p "$playwright_browsers_path"
export COREPACK_HOME="$corepack_home"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"
export GOTOOLCHAIN="$go_toolchain"
export PLAYWRIGHT_BROWSERS_PATH="$playwright_browsers_path"

allow_env_skips="${COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS:-false}"
run_deploy="${COMMERCIAL_COMPLETION_RUN_DEPLOY:-false}"
run_k8s="${COMMERCIAL_COMPLETION_RUN_K8S:-false}"
run_backup_restore="${COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE:-false}"
run_target_evidence="${COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE:-false}"
skipped_checks=()

fail() {
  echo "[commercial-completion] $*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$db_server_suite_container_name" ]]; then
    docker rm -f "$db_server_suite_container_name" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

command_works() {
  local command_name="$1"

  command -v "$command_name" >/dev/null 2>&1 && "$command_name" --version >/dev/null 2>&1
}

docker_daemon_reachable() {
  command -v docker >/dev/null 2>&1 || return 1
  if command -v timeout >/dev/null 2>&1; then
    timeout 5 docker info >/dev/null 2>&1
  else
    docker info >/dev/null 2>&1
  fi
}

windows_path() {
  local path="$1"

  if command -v wslpath >/dev/null 2>&1; then
    wslpath -w "$path"
    return
  fi
  printf '%s\n' "$path"
}

windows_pnpm_args() {
  local convert_next=false
  local arg

  for arg in "$@"; do
    if [[ "$convert_next" == "true" ]]; then
      windows_path "$arg"
      convert_next=false
      continue
    fi

    printf '%s\n' "$arg"
    if [[ "$arg" == "--dir" || "$arg" == "-C" ]]; then
      convert_next=true
    fi
  done
}

if command_works pnpm; then
  :
elif command_works corepack; then
  pnpm() {
    corepack pnpm "$@"
  }
elif [[ -x "$windows_corepack_node" ]]; then
  pnpm() {
    local args=()
    local arg

    while IFS= read -r arg; do
      args+=("$arg")
    done < <(windows_pnpm_args "$@")

    "$windows_corepack_node" "$windows_corepack_script" pnpm "${args[@]}"
  }
else
  fail "pnpm or corepack is required for web verification"
fi

if [[ -z "${PLAYWRIGHT_WEB_SERVER_PACKAGE_RUNNER:-}" && -x "$windows_corepack_node" ]]; then
  export PLAYWRIGHT_WEB_SERVER_PACKAGE_RUNNER="\"C:\\Program Files\\nodejs\\node.exe\" \"${windows_corepack_script}\" pnpm"
fi

ensure_go_on_path() {
  local candidate
  local go_path=""
  local tool_bin_dir="$repo_root/.tmp/tool-bin"

  if command -v go >/dev/null 2>&1; then
    return
  fi

  for candidate in \
    "/mnt/c/Program Files/Go/bin/go" \
    "/mnt/c/Program Files/Go/bin/go.exe" \
    "/c/Program Files/Go/bin/go" \
    "/c/Program Files/Go/bin/go.exe" \
    "/usr/local/go/bin/go"; do
    if [[ -x "$candidate" ]]; then
      go_path="$candidate"
      break
    fi
  done

  if [[ -z "$go_path" ]]; then
    return
  fi

  mkdir -p "$tool_bin_dir"
  printf '#!/usr/bin/env sh\nexport WSLENV="${WSLENV:+$WSLENV:}TEST_DATABASE_URL:DATABASE_URL:OBLIVIOUS_REQUIRE_TEST_DATABASE:GOTOOLCHAIN:GOPROXY:GOSUMDB:GOPRIVATE:GONOSUMDB:GOFLAGS:HTTP_PROXY:HTTPS_PROXY:NO_PROXY"\nexec "%s" "$@"\n' "$go_path" > "$tool_bin_dir/go"
  chmod +x "$tool_bin_dir/go"
  export PATH="$tool_bin_dir:$PATH"
}

ensure_go_on_path

run_step() {
  local label="$1"
  shift

  echo "[commercial-completion] START $label"
  "$@"
  echo "[commercial-completion] PASS  $label"
}

commercial_preflight_node() {
  if command -v node >/dev/null 2>&1 && node --version >/dev/null 2>&1; then
    printf '%s\n' node
    return
  fi
  if [[ -x "$windows_corepack_node" ]]; then
    printf '%s\n' "$windows_corepack_node"
    return
  fi
  fail "node is required for commercial preflight"
}

run_commercial_preflight() {
  local node_bin

  node_bin="$(commercial_preflight_node)"
  "$node_bin" "$repo_root/scripts/verify-commercial-preflight.mjs" --target-evidence-only
}

resolved_file_path() {
  local input_path="$1"

  (cd "$(dirname "$input_path")" && printf '%s/%s\n' "$(pwd -P)" "$(basename "$input_path")")
}

require_k8s_secret_preflight() {
  local secret_file="${OBLIVIOUS_K8S_SECRET_FILE:-}"
  local secret_realpath
  local example_realpath
  local repo_realpath

  if [[ -z "$secret_file" ]]; then
    fail "OBLIVIOUS_K8S_SECRET_FILE is required when COMMERCIAL_COMPLETION_RUN_K8S=true"
  fi
  if [[ ! -f "$secret_file" ]]; then
    fail "Kubernetes secret file does not exist: $secret_file"
  fi

  secret_realpath="$(resolved_file_path "$secret_file")"
  example_realpath="$(resolved_file_path "$repo_root/deploy/kubernetes/secret.example.yaml")"
  repo_realpath="$(cd "$repo_root" && pwd -P)"
  if [[ "$secret_realpath" == "$example_realpath" ]]; then
    fail "refusing deploy/kubernetes/secret.example.yaml as runtime proof"
  fi
  case "$secret_realpath" in
    "$repo_realpath"|"$repo_realpath"/*)
      fail "strict final readiness requires OBLIVIOUS_K8S_SECRET_FILE outside the repository"
      ;;
  esac
  if grep -Eq "REPLACE_ME|CHANGE_ME|change-me-in-production" "$secret_file"; then
    fail "Kubernetes secret file still contains placeholder values"
  fi
}

require_strict_final_flags() {
  local missing=()

  [[ "$run_deploy" == "true" ]] || missing+=("COMMERCIAL_COMPLETION_RUN_DEPLOY=true")
  [[ "$run_k8s" == "true" ]] || missing+=("COMMERCIAL_COMPLETION_RUN_K8S=true")
  [[ "$run_backup_restore" == "true" ]] || missing+=("COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true")
  [[ "$run_target_evidence" == "true" ]] || missing+=("COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE=true")

  if [[ "${#missing[@]}" -eq 0 ]]; then
    return
  fi

  fail "strict final readiness requires ${missing[*]}; use COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true only for non-final local evidence"
}

require_strict_final_prerequisites() {
  local missing=()
  local message

  if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
    missing+=("TEST_DATABASE_URL is required for DB-backed Phase 30 commercial journey proof")
  fi
  if [[ "$run_k8s" == "true" && -z "${OBLIVIOUS_K8S_SECRET_FILE:-}" ]]; then
    missing+=("OBLIVIOUS_K8S_SECRET_FILE is required when COMMERCIAL_COMPLETION_RUN_K8S=true")
  fi
  if [[ "$run_target_evidence" == "true" && -z "${OBLIVIOUS_TARGET_EVIDENCE_FILE:-}" ]]; then
    missing+=("target live evidence manifest requires OBLIVIOUS_TARGET_EVIDENCE_FILE for strict final readiness")
  fi
  if [[ "$run_target_evidence" == "true" && -z "${OBLIVIOUS_TARGET_ARTIFACT_DIR:-}" ]]; then
    missing+=("target live evidence manifest requires OBLIVIOUS_TARGET_ARTIFACT_DIR with downloaded artifact bodies for strict final readiness")
  fi

  if [[ "${#missing[@]}" -eq 0 ]]; then
    return
  fi

  message="${missing[0]}"
  for item in "${missing[@]:1}"; do
    message="$message; $item"
  done
  fail "strict final readiness missing required inputs: $message"
}

require_db_server_suite_prerequisite() {
  if [[ -n "${COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL:-}" ]]; then
    return
  fi
  if docker_daemon_reachable; then
    return
  fi

  fail "COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL or a reachable docker daemon is required for isolated DB-backed server suite"
}

skip_or_fail() {
  local label="$1"
  local env_name="$2"

  if [[ "$allow_env_skips" == "true" ]]; then
    skipped_checks+=("$label")
    echo "[commercial-completion] SKIP  $label ($env_name is not true)"
    return
  fi

  fail "$label requires $env_name=true, or COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true for non-final local evidence"
}

start_db_server_suite_database() {
  local port

  command -v docker >/dev/null 2>&1 || fail "docker is required when COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL is not set"
  docker_daemon_reachable || fail "docker daemon is not reachable"

  db_server_suite_container_name="oblivious-commercial-server-suite-$$"
  echo "[commercial-completion] starting isolated DB-backed server suite PostgreSQL with $postgres_image" >&2
  docker run -d --rm \
    --name "$db_server_suite_container_name" \
    -e POSTGRES_DB=oblivious \
    -e POSTGRES_USER=oblivious \
    -e POSTGRES_PASSWORD=oblivious \
    -p 127.0.0.1::5432 \
    "$postgres_image" >/dev/null

  port=$(docker port "$db_server_suite_container_name" 5432/tcp | sed -E 's/.*:([0-9]+)$/\1/')
  [[ -n "$port" ]] || fail "could not resolve mapped PostgreSQL port for DB-backed server suite"

	for attempt in $(seq 1 60); do
	  if docker exec "$db_server_suite_container_name" pg_isready -U oblivious -d oblivious >/dev/null 2>&1; then
	    db_server_suite_database_url="postgres://oblivious:oblivious@127.0.0.1:${port}/oblivious?sslmode=disable"
	    return
	  fi
	  sleep 1
	done

  fail "isolated DB-backed server suite PostgreSQL did not become ready"
}

if [[ "${OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH:-false}" == "true" ]]; then
  fail "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH cannot be true for strict final readiness"
fi
if [[ "$allow_env_skips" != "true" ]]; then
  require_strict_final_flags
  require_strict_final_prerequisites
fi
if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  fail "TEST_DATABASE_URL is required for DB-backed Phase 30 commercial journey proof"
fi
if [[ "$run_k8s" == "true" ]]; then
  require_k8s_secret_preflight
fi
if [[ "$run_target_evidence" == "true" ]]; then
  if [[ -z "${OBLIVIOUS_TARGET_EVIDENCE_FILE:-}" ]]; then
    fail "target live evidence manifest requires OBLIVIOUS_TARGET_EVIDENCE_FILE for strict final readiness"
  fi
  if [[ ! -f "${OBLIVIOUS_TARGET_EVIDENCE_FILE:-}" ]]; then
    fail "target live evidence manifest file does not exist: ${OBLIVIOUS_TARGET_EVIDENCE_FILE:-}"
  fi
  if [[ -z "${OBLIVIOUS_TARGET_ARTIFACT_DIR:-}" ]]; then
    fail "target live evidence manifest requires OBLIVIOUS_TARGET_ARTIFACT_DIR with downloaded artifact bodies for strict final readiness"
  fi
  if [[ ! -d "${OBLIVIOUS_TARGET_ARTIFACT_DIR:-}" ]]; then
    fail "target live evidence artifact directory does not exist: ${OBLIVIOUS_TARGET_ARTIFACT_DIR:-}"
  fi
  run_step "target evidence preflight" run_commercial_preflight
fi
require_db_server_suite_prerequisite

run_step "docs gate" bash "$repo_root/scripts/check.sh" docs
run_step "Relay security gate" bash "$repo_root/scripts/check.sh" relay-security
run_step "dependency security gate" bash "$repo_root/scripts/check.sh" security
run_step "web TypeScript gate" pnpm --dir "$web_dir" exec tsc --noEmit
run_step "server Go suite" \
  env -u TEST_DATABASE_URL bash -c 'cd "$1" && go test -p 1 ./... -count=1' bash "$server_dir"
run_step "commercial frontend focused suites" \
  pnpm --dir "$web_dir" test -- ChatPage SoloPage KnowledgePage MarketplacePage AdminHomePage AdminBillingPage AdminReviewsPage --runInBand
if [[ -z "${PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH:-}" ]]; then
  run_step "Playwright Chromium browser" \
    pnpm --dir "$web_dir" exec playwright install chromium
fi
run_step "browser commercial journey" \
  pnpm --dir "$web_dir" test:e2e --grep "commercial journey"
run_step "DB-backed commercial evidence profiles" \
  env TEST_DATABASE_URL="$TEST_DATABASE_URL" bash "$repo_root/scripts/verify-commercial-db-evidence.sh" all
db_server_suite_database_url="${COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL:-}"
if [[ -z "$db_server_suite_database_url" ]]; then
	start_db_server_suite_database
fi
run_step "DB-backed server Go suite" \
  env TEST_DATABASE_URL="$db_server_suite_database_url" bash -c 'cd "$1" && go test -p 1 ./... -count=1' bash "$server_dir"

if [[ "$run_deploy" == "true" ]]; then
  run_step "deployment validation" bash "$repo_root/scripts/deploy-validate.sh"
else
  skip_or_fail "deployment validation" "COMMERCIAL_COMPLETION_RUN_DEPLOY"
fi

if [[ "$run_k8s" == "true" ]]; then
  run_step "Kubernetes validation" bash "$repo_root/scripts/k8s-validate.sh"
else
  skip_or_fail "Kubernetes validation" "COMMERCIAL_COMPLETION_RUN_K8S"
fi

if [[ "$run_backup_restore" == "true" ]]; then
  run_step "backup and restore smoke" bash "$repo_root/scripts/backup-restore-smoke.sh"
else
  skip_or_fail "backup and restore smoke" "COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE"
fi

if [[ "$run_target_evidence" == "true" ]]; then
  run_step "target live evidence manifest" bash "$repo_root/scripts/verify-target-release-evidence.sh"
else
  skip_or_fail "target live evidence manifest" "COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE"
fi

run_step "diff hygiene" git -C "$repo_root" diff --check

echo "[commercial-completion] SUMMARY"
echo "[commercial-completion] TEST_DATABASE_URL class: configured"
echo "[commercial-completion] Go toolchain: $go_toolchain"
if [[ -n "${COMMERCIAL_COMPLETION_DB_SERVER_SUITE_DATABASE_URL:-}" ]]; then
  echo "[commercial-completion] DB-backed server suite database: configured separate URL"
else
  echo "[commercial-completion] DB-backed server suite database: isolated disposable pgvector PostgreSQL"
fi
if [[ -n "${PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH:-}" ]]; then
  echo "[commercial-completion] Playwright executable: $PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH"
else
  echo "[commercial-completion] Playwright executable: default browser cache"
fi

if [[ "${#skipped_checks[@]}" -gt 0 ]]; then
  printf '[commercial-completion] skipped checks: %s\n' "${skipped_checks[*]}"
  echo "[commercial-completion] RESULT: partial local evidence only; not final commercial readiness"
else
  echo "[commercial-completion] skipped checks: none"
  echo "[commercial-completion] RESULT: strict verifier passed"
fi
