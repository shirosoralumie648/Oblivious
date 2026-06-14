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
Usage: bash scripts/verify-commercial-db-evidence.sh [all|backend-journey|marketplace-money-movement|app-stateful-routes|tenant-membership-lifecycle|tenant-cross-surface|agent-runtime-memory|scheduled-task-runtime]

Runs narrow DB-backed commercial evidence without silently accepting skipped tests.

Profiles:
  all                          Run every DB-backed profile below in one
                               disposable/configured PostgreSQL session.
  backend-journey              Run TestCommercialHTTPJourney against PostgreSQL.
  marketplace-money-movement   Run focused Billing/Marketplace money movement
                               PostgreSQL lifecycle tests.
  app-stateful-routes          Run focused app state, tenant, CSRF, and
                               ownership PostgreSQL route tests.
  tenant-membership-lifecycle  Run focused Tenant SQL store and HTTP
                               membership/ownership lifecycle tests.
  tenant-cross-surface         Run focused cross-tenant app surface isolation
                               tests across Chat, Knowledge, Console, Agent,
                               Memory, MCP, Quota, and Marketplace.
  agent-runtime-memory         Run focused Agent runtime, approval, execution
                               mode, structured plan-step, and memory policy
                               PostgreSQL tests.
  scheduled-task-runtime       Run focused Scheduled Task SQL store, route, and
                               Workflow trigger sync PostgreSQL tests.

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

run_backend_journey_profile() {
  run_go_test_no_skips "backend commercial HTTP journey" "./internal/http" "^TestCommercialHTTPJourney$"
}

run_marketplace_money_movement_profile() {
  local marketplace_settlement_pattern
  local admin_money_movement_pattern

  marketplace_settlement_pattern="^TestSettlement(CreatePaidInstallCheckoutCreatesPendingOrderAndIntent|CreatePaidInstallCheckoutRecordsSelectedProvider|CreatePaidInstallCheckoutSQLUsesRequestedCurrency|ApplyPaidInstallCheckoutCompletedRecordsSelectedProviderLifecycle|MarkPayoutPendingDispatchesConfiguredProvider|MarkPayoutPaidUpdatesPayoutAndSettlementsOnce|ProviderPayoutPaidWebhookMatchesProviderPayoutIDOnce|ProviderPayoutFailedWebhookReleasesSettlementsOnce|MarkPayoutFailedReleasesSettlementsOnce|CreateDuePayoutsAggregatesAvailableSettlementsOnce|CreateDuePayoutsDispatchesConfiguredProvider|PublisherStatsIncludesSettlementAmounts)$"
  admin_money_movement_pattern="^Test(AdminBilling(SummaryIncludesMoneyMovementState|ListsExposeAllRequiredSurfaces|ListsApplyRecoveryFilters|SummaryAppliesFailedStatusFilter|MarksMarketplacePayoutPaid|MarksMarketplacePayoutFailedAndReleasesSettlements|CreateDueMarketplacePayoutsDispatchesConfiguredProvider|RecordsTopupRefundAndAdjustsQuota)|MarketplacePaidInstall(DoesNotInstallBeforeWebhook|CheckoutCreatorFailureMarksOrderFailed|UsesConfiguredProviderCheckoutCreator)|DomesticPaymentWebhookRouteAppliesMarketplace(InstallSettlementOnce|RefundOnce|PayoutPaidOnce|PayoutFailedOnce))$"
  run_go_test_no_skips "marketplace settlement money movement" "./internal/marketplace" "$marketplace_settlement_pattern"
  run_go_test_no_skips "admin billing and marketplace money movement routes" "./internal/http" "$admin_money_movement_pattern"
}

run_app_stateful_routes_profile() {
  local app_stateful_routes_pattern

  app_stateful_routes_pattern="^Test(ConsoleAPITokenCreateListAndRevoke|ConsoleUsageListsCurrentUserRecentRelayRequests|SelectOrganizationRequiresMembershipAndUpdatesSessionScope|OrganizationInvitationRevokeRejectsAcceptance|OrganizationSessionSecurityOnMembershipChanges|NotificationMutationRoutesEnforceOwnership|GetPreferencesReturnsUserInitializationState|UpdatePreferencesPersistsOnboardingState|ConversationAndMessageFlow|ConversationConfigFlow|RouteSurface(RequiresSessionForAppRoutes|RejectsCookieMutationWithoutCSRF))$"
  run_go_test_no_skips "app stateful route persistence and ownership" "./internal/http" "$app_stateful_routes_pattern"
}

run_tenant_membership_lifecycle_profile() {
  local tenant_store_pattern
  local tenant_http_pattern

  tenant_store_pattern="^TestSQLStore(OrganizationLifecycle|MembershipInvitationOwnershipLifecycle)$"
  tenant_http_pattern="^Test(RegisterCreatesDefaultOrganizationAndSessionScope|LoginResolvesDefaultOrganizationForLegacyUser|SelectOrganizationRequiresMembershipAndUpdatesSessionScope|OrganizationInvitationRevokeRejectsAcceptance|OrganizationSessionSecurityOnMembershipChanges|OrganizationMemberRoutesListTransferOwnershipAndRemove)$"
  run_go_test_no_skips "tenant SQL organization and membership lifecycle" "./internal/tenant" "$tenant_store_pattern"
  run_go_test_no_skips "tenant HTTP membership ownership lifecycle" "./internal/http" "$tenant_http_pattern"
}

run_tenant_cross_surface_profile() {
  local tenant_cross_surface_pattern

  tenant_cross_surface_pattern="^Test(CrossTenant(ChatScopeUsesActiveOrganization|KnowledgeScopeDeniesReadWriteAndAttach|ConsoleUsageUsesActiveOrganization|AgentScopeDeniesReadWriteAndConversation|MemoryScopeDeniesReadWrite|MCPScopeDeniesReadWriteAndConnect|QuotaScopeUsesActiveOrganization|MarketplacePublisherScopeUsesActiveOrganization)|MarketplacePublisherSettlementPreferencesUseActiveOrganization|AgentRunStatusEndpointsExposeTenantScopedRunDetail|AgentToolRunApprovalRejectRetryEndpointsAreTenantScoped)$"
  run_go_test_no_skips "tenant cross-surface app isolation" "./internal/http" "$tenant_cross_surface_pattern"
}

run_agent_runtime_memory_profile() {
  local agent_runtime_memory_pattern

  agent_runtime_memory_pattern="^(TestAgentRunStorePersistsRunLifecycle|TestAgentPlanStepStore(RoundTripsStepsInOrder|UpdatesStatusAndExecutionResult)|TestAgentSQLStorePersists(ApprovalConfigAndToolRiskLevels|DefaultExecutionModeConfig|LongTermMemoryWritePolicyConfig))$"
  run_go_test_no_skips "agent runtime and memory policy persistence" "./internal/agent" "$agent_runtime_memory_pattern"
}

run_scheduled_task_runtime_profile() {
  local scheduled_task_store_pattern
  local scheduled_task_routes_pattern

  scheduled_task_store_pattern="^TestSQLStore(CreatesAndListsScheduledTasksByOrganization|SyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks|GetsAndUpdatesScheduledTaskEnabledState|RecordsAndListsScheduledTaskRunsByOrganizationAndTask|ClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns|CompletesManualScheduledTaskRunWithoutAdvancingNextRun|CompletesScheduledTaskRunAndAdvancesTask)$"
  scheduled_task_routes_pattern="^(TestScheduledTasksRoute(CreatesAndListsTasks|ListsRunsForTaskWithinSessionOrganization)|TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks)$"
  run_go_test_no_skips "scheduled task SQL runtime persistence" "./internal/schedule" "$scheduled_task_store_pattern"
  run_go_test_no_skips "scheduled task route and workflow sync persistence" "./internal/http" "$scheduled_task_routes_pattern"
}

run_all_profiles() {
  run_backend_journey_profile
  run_marketplace_money_movement_profile
  run_app_stateful_routes_profile
  run_tenant_membership_lifecycle_profile
  run_tenant_cross_surface_profile
  run_agent_runtime_memory_profile
  run_scheduled_task_runtime_profile
}

if [[ -z "$database_url" ]]; then
  start_temporary_database
else
  echo "[commercial-db-evidence] using configured TEST_DATABASE_URL"
fi

case "$profile" in
  all)
    run_all_profiles
    ;;
  backend-journey)
    run_backend_journey_profile
    ;;
  marketplace-money-movement)
    run_marketplace_money_movement_profile
    ;;
  app-stateful-routes)
    run_app_stateful_routes_profile
    ;;
  tenant-membership-lifecycle)
    run_tenant_membership_lifecycle_profile
    ;;
  tenant-cross-surface)
    run_tenant_cross_surface_profile
    ;;
  agent-runtime-memory)
    run_agent_runtime_memory_profile
    ;;
  scheduled-task-runtime)
    run_scheduled_task_runtime_profile
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
