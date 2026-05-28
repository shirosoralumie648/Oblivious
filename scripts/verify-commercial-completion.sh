#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"

usage() {
  cat <<'EOF'
Usage: bash scripts/verify-commercial-completion.sh [--help]

Runs the strict Phase 30 commercial completion verifier.

Required for strict final readiness:
  TEST_DATABASE_URL
    PostgreSQL test database URL. Must support pgvector for Knowledge RAG tests.
  COMMERCIAL_COMPLETION_RUN_DEPLOY=true
    Run bash scripts/deploy-validate.sh as part of the final verifier.
  COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE=true
    Run bash scripts/backup-restore-smoke.sh as part of the final verifier.

Optional:
  COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true
    Allow deploy and backup/restore checks to be skipped for local partial evidence.
    A run with this flag is not final commercial readiness evidence.
  PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
    Existing Chromium/Chrome executable for Playwright when browser cache is unavailable.
  COREPACK_HOME, GOCACHE, GOMODCACHE
    Override local tool caches.
  BACKUP_SMOKE_SOURCE_DATABASE_URL, BACKUP_SMOKE_RESTORE_DATABASE_URL
    External disposable databases for backup/restore smoke.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

mkdir -p "$corepack_home" "$go_cache" "$go_mod_cache"
export COREPACK_HOME="$corepack_home"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"

allow_env_skips="${COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS:-false}"
run_deploy="${COMMERCIAL_COMPLETION_RUN_DEPLOY:-false}"
run_backup_restore="${COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE:-false}"
skipped_checks=()

fail() {
  echo "[commercial-completion] $*" >&2
  exit 1
}

run_step() {
  local label="$1"
  shift

  echo "[commercial-completion] START $label"
  "$@"
  echo "[commercial-completion] PASS  $label"
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

if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  fail "TEST_DATABASE_URL is required for DB-backed Phase 30 commercial journey proof"
fi

run_step "docs gate" bash "$repo_root/scripts/check.sh" docs
run_step "Relay security gate" bash "$repo_root/scripts/check.sh" relay-security
run_step "commercial frontend focused suites" \
  pnpm --dir "$web_dir" test -- ChatPage SoloPage KnowledgePage MarketplacePage AdminHomePage AdminBillingPage AdminReviewsPage --runInBand
run_step "browser commercial journey" \
  pnpm --dir "$web_dir" test:e2e --grep "commercial journey"
run_step "backend DB commercial journey" \
  bash -c 'cd "$1" && TEST_DATABASE_URL="$2" OBLIVIOUS_REQUIRE_TEST_DATABASE=true go test ./internal/http -run TestCommercialHTTPJourney -count=1' _ "$server_dir" "$TEST_DATABASE_URL"

if [[ "$run_deploy" == "true" ]]; then
  run_step "deployment validation" bash "$repo_root/scripts/deploy-validate.sh"
else
  skip_or_fail "deployment validation" "COMMERCIAL_COMPLETION_RUN_DEPLOY"
fi

if [[ "$run_backup_restore" == "true" ]]; then
  run_step "backup and restore smoke" bash "$repo_root/scripts/backup-restore-smoke.sh"
else
  skip_or_fail "backup and restore smoke" "COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE"
fi

echo "[commercial-completion] SUMMARY"
echo "[commercial-completion] TEST_DATABASE_URL class: configured"
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
