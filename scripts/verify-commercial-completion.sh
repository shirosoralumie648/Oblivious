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
    and production workflow telemetry.

Optional:
  COMMERCIAL_COMPLETION_ALLOW_ENV_SKIPS=true
    Allow deploy, Kubernetes, and backup/restore checks to be skipped for local partial evidence.
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
run_k8s="${COMMERCIAL_COMPLETION_RUN_K8S:-false}"
run_backup_restore="${COMMERCIAL_COMPLETION_RUN_BACKUP_RESTORE:-false}"
run_target_evidence="${COMMERCIAL_COMPLETION_RUN_TARGET_EVIDENCE:-false}"
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

if [[ "${OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH:-false}" == "true" ]]; then
  fail "OBLIVIOUS_TARGET_EVIDENCE_ALLOW_COMMIT_MISMATCH cannot be true for strict final readiness"
fi
if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  fail "TEST_DATABASE_URL is required for DB-backed Phase 30 commercial journey proof"
fi

run_step "docs gate" bash "$repo_root/scripts/check.sh" docs
run_step "Relay security gate" bash "$repo_root/scripts/check.sh" relay-security
run_step "dependency security gate" bash "$repo_root/scripts/check.sh" security
run_step "web TypeScript gate" pnpm --dir "$web_dir" exec tsc --noEmit
run_step "server Go suite" \
  bash -c 'cd "$1" && go test ./... -count=1' bash "$server_dir"
run_step "commercial frontend focused suites" \
  pnpm --dir "$web_dir" test -- ChatPage SoloPage KnowledgePage MarketplacePage AdminHomePage AdminBillingPage AdminReviewsPage --runInBand
run_step "browser commercial journey" \
  pnpm --dir "$web_dir" test:e2e --grep "commercial journey"
run_step "DB-backed commercial evidence profiles" \
  env TEST_DATABASE_URL="$TEST_DATABASE_URL" bash "$repo_root/scripts/verify-commercial-db-evidence.sh" all
run_step "DB-backed server Go suite" \
  env TEST_DATABASE_URL="$TEST_DATABASE_URL" bash -c 'cd "$1" && go test -p 1 ./... -count=1' bash "$server_dir"

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
