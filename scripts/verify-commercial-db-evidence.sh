#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
postgres_image="${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
profile="${1:-backend-journey}"
database_url="${TEST_DATABASE_URL:-}"
container_name=""
temporary_database="false"
output_files=()

usage() {
  cat <<'EOF'
Usage: bash scripts/verify-commercial-db-evidence.sh [backend-journey|marketplace-money-movement|app-stateful-routes|agent-runtime-memory]

Runs narrow DB-backed commercial evidence without silently accepting skipped tests.

Profiles:
  backend-journey              Run TestCommercialHTTPJourney against PostgreSQL.
  marketplace-money-movement   Run focused Billing/Marketplace money movement
                               PostgreSQL lifecycle tests.
  app-stateful-routes          Run focused app state, tenant, CSRF, and
                               ownership PostgreSQL route tests.
  agent-runtime-memory         Run focused Agent runtime, approval, execution
                               mode, and memory policy PostgreSQL tests.

Environment:
  TEST_DATABASE_URL        Optional PostgreSQL URL. If unset, a disposable pgvector
                           PostgreSQL container is started with Docker.
  OBLIVIOUS_POSTGRES_IMAGE Optional image for disposable PostgreSQL.
  COREPACK_HOME, GOCACHE, GOMODCACHE
                           Override local tool caches.
EOF
}

fail() {
  echo "[commercial-db-evidence] $*" >&2
  exit 1
}

cleanup() {
  local output_file
  for output_file in "${output_files[@]:-}"; do
    rm -f "$output_file" >/dev/null 2>&1 || true
  done
  if [[ -n "$container_name" ]]; then
    docker rm -f "$container_name" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ "$profile" == "--help" || "$profile" == "-h" ]]; then
  usage
  exit 0
fi

mkdir -p "$corepack_home" "$go_cache" "$go_mod_cache"

start_temporary_database() {
  local port

  command -v docker >/dev/null 2>&1 || fail "docker is required when TEST_DATABASE_URL is not set"
  docker info >/dev/null 2>&1 || fail "docker daemon is not reachable"

  container_name="oblivious-commercial-db-evidence-$$"
  echo "[commercial-db-evidence] starting disposable PostgreSQL with $postgres_image"
  docker run -d --rm \
    --name "$container_name" \
    -e POSTGRES_DB=oblivious \
    -e POSTGRES_USER=oblivious \
    -e POSTGRES_PASSWORD=oblivious \
    -p 127.0.0.1::5432 \
    "$postgres_image" >/dev/null

  port=$(docker port "$container_name" 5432/tcp | sed -E 's/.*:([0-9]+)$/\1/')
  [[ -n "$port" ]] || fail "could not resolve mapped PostgreSQL port"
  database_url="postgres://oblivious:oblivious@127.0.0.1:${port}/oblivious?sslmode=disable"
  temporary_database="true"

  for attempt in $(seq 1 60); do
    if docker exec "$container_name" pg_isready -U oblivious -d oblivious >/dev/null 2>&1; then
      return
    fi
    if [[ "$attempt" == "60" ]]; then
      fail "temporary PostgreSQL did not become ready"
    fi
    sleep 1
  done
}

run_go_test_no_skips() {
  local label="$1"
  local package="$2"
  local pattern="$3"
  local output_file
  local status

  output_file=$(mktemp)
  output_files+=("$output_file")

  echo "[commercial-db-evidence] START $label"
  set +e
  (
    cd "$server_dir"
    COREPACK_HOME="$corepack_home" \
      GOCACHE="$go_cache" \
      GOMODCACHE="$go_mod_cache" \
      TEST_DATABASE_URL="$database_url" \
      OBLIVIOUS_REQUIRE_TEST_DATABASE=true \
      go test -p 1 "$package" -run "$pattern" -count=1 -v
  ) 2>&1 | tee "$output_file"
  status=${PIPESTATUS[0]}
  set -e

  if grep -Fq -- "--- SKIP:" "$output_file"; then
    fail "$label skipped at least one test; skipped DB-backed evidence is not accepted"
  fi
  if ! grep -Fq -- "=== RUN" "$output_file"; then
    fail "$label did not run any tests; empty DB-backed evidence is not accepted"
  fi
  if [[ "$status" -ne 0 ]]; then
    fail "$label failed"
  fi
  echo "[commercial-db-evidence] PASS  $label"
}

if [[ -z "$database_url" ]]; then
  start_temporary_database
else
  echo "[commercial-db-evidence] using configured TEST_DATABASE_URL"
fi

case "$profile" in
  backend-journey)
    run_go_test_no_skips "backend commercial HTTP journey" "./internal/http" "^TestCommercialHTTPJourney$"
    ;;
  marketplace-money-movement)
    marketplace_settlement_pattern="^TestSettlement(CreatePaidInstallCheckoutCreatesPendingOrderAndIntent|CreatePaidInstallCheckoutRecordsSelectedProvider|CreatePaidInstallCheckoutSQLUsesRequestedCurrency|ApplyPaidInstallCheckoutCompletedRecordsSelectedProviderLifecycle|MarkPayoutPendingDispatchesConfiguredProvider|MarkPayoutPaidUpdatesPayoutAndSettlementsOnce|ProviderPayoutPaidWebhookMatchesProviderPayoutIDOnce|ProviderPayoutFailedWebhookReleasesSettlementsOnce|MarkPayoutFailedReleasesSettlementsOnce|CreateDuePayoutsAggregatesAvailableSettlementsOnce|CreateDuePayoutsDispatchesConfiguredProvider|PublisherStatsIncludesSettlementAmounts)$"
    admin_money_movement_pattern="^Test(AdminBilling(SummaryIncludesMoneyMovementState|ListsExposeAllRequiredSurfaces|ListsApplyRecoveryFilters|SummaryAppliesFailedStatusFilter|MarksMarketplacePayoutPaid|MarksMarketplacePayoutFailedAndReleasesSettlements|CreateDueMarketplacePayoutsDispatchesConfiguredProvider|RecordsTopupRefundAndAdjustsQuota)|MarketplacePaidInstall(DoesNotInstallBeforeWebhook|CheckoutCreatorFailureMarksOrderFailed|UsesConfiguredProviderCheckoutCreator)|DomesticPaymentWebhookRouteAppliesMarketplace(InstallSettlementOnce|RefundOnce|PayoutPaidOnce|PayoutFailedOnce))$"
    run_go_test_no_skips "marketplace settlement money movement" "./internal/marketplace" "$marketplace_settlement_pattern"
    run_go_test_no_skips "admin billing and marketplace money movement routes" "./internal/http" "$admin_money_movement_pattern"
    ;;
  app-stateful-routes)
    app_stateful_routes_pattern="^Test(ConsoleAPITokenCreateListAndRevoke|SelectOrganizationRequiresMembershipAndUpdatesSessionScope|OrganizationInvitationRevokeRejectsAcceptance|OrganizationSessionSecurityOnMembershipChanges|NotificationMutationRoutesEnforceOwnership|GetPreferencesReturnsUserInitializationState|UpdatePreferencesPersistsOnboardingState|ConversationAndMessageFlow|ConversationConfigFlow|RouteSurface(RequiresSessionForAppRoutes|RejectsCookieMutationWithoutCSRF))$"
    run_go_test_no_skips "app stateful route persistence and ownership" "./internal/http" "$app_stateful_routes_pattern"
    ;;
  agent-runtime-memory)
    agent_runtime_memory_pattern="^(TestAgentRunStorePersistsRunLifecycle|TestAgentSQLStorePersists(ApprovalConfigAndToolRiskLevels|DefaultExecutionModeConfig|LongTermMemoryWritePolicyConfig))$"
    run_go_test_no_skips "agent runtime and memory policy persistence" "./internal/agent" "$agent_runtime_memory_pattern"
    ;;
  *)
    usage >&2
    fail "unknown profile: $profile"
    ;;
esac

echo "[commercial-db-evidence] SUMMARY"
if [[ "$temporary_database" == "true" ]]; then
  echo "[commercial-db-evidence] database: disposable pgvector PostgreSQL"
else
  echo "[commercial-db-evidence] database: configured TEST_DATABASE_URL"
fi
echo "[commercial-db-evidence] skipped tests: none"
