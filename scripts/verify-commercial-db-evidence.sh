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
Usage: bash scripts/verify-commercial-db-evidence.sh [all|backend-journey|marketplace-money-movement|marketplace-governance-review|marketplace-recommendation-search|marketplace-template-routes|billing-checkout-topup-http|billing-provider-lifecycle|admin-usage-analytics-db|app-stateful-routes|tenant-membership-lifecycle|tenant-cross-surface|secret-response-safety|agent-runtime-memory|scheduled-task-runtime|auth-security-persistence|migration-ledger-backfills|relay-file-mapping-tenant-ownership|relay-runtime-channel-isolation|workflow-sql-isolation|publishing-channel-isolation|admin-relay-channel-isolation|admin-relay-read-isolation|observability-alert-recovery-persistence|quota-sql-isolation|core-sql-persistence]

Runs narrow DB-backed commercial evidence without silently accepting skipped tests.

Profiles:
  all                          Run every DB-backed profile below in one
                               disposable/configured PostgreSQL session.
  backend-journey              Run TestCommercialHTTPJourney against PostgreSQL.
  marketplace-money-movement   Run focused Billing/Marketplace money movement
                               PostgreSQL, provider filter, and refund evidence
                               tests.
  marketplace-governance-review
                               Run focused Marketplace governance, automated
                               review, abuse-report, and review-SLA
                               PostgreSQL lifecycle tests.
  marketplace-recommendation-search
                               Run focused Marketplace recommended search,
                               ranking-signal, collaborative-filtering, and
                               exploration PostgreSQL tests.
  marketplace-template-routes  Run focused Marketplace template create, list,
                               detail, and install PostgreSQL route tests.
  billing-checkout-topup-http  Run focused Billing checkout, top-up, and
                               payment webhook PostgreSQL route tests.
  billing-provider-lifecycle   Run focused Stripe/shared and domestic checkout,
                               invoice, subscription, and refund lifecycle
                               PostgreSQL/bridge tests.
  admin-usage-analytics-db     Run focused Admin usage daily aggregate refresh,
                               analytics, and zero-token fallback PostgreSQL
                               tests.
  app-stateful-routes          Run focused app state, tenant, CSRF, and
                               ownership PostgreSQL route tests.
  tenant-membership-lifecycle  Run focused Tenant SQL store and HTTP
                               membership/ownership lifecycle tests.
  tenant-cross-surface         Run focused cross-tenant app surface isolation
                               tests across Chat, Knowledge, Console, Agent,
                               Memory, MCP, Quota, and Marketplace.
  secret-response-safety       Run focused DB-backed response redaction and
                               at-rest protection tests for persisted provider,
                               channel, workflow, and MCP auth-token secrets.
  agent-runtime-memory         Run focused Agent runtime, ReAct model routing,
                               skills, tool fallback, approval, execution mode,
                               structured plan-step, memory store, and memory
                               policy PostgreSQL tests.
  scheduled-task-runtime       Run focused Scheduled Task SQL store, route, and
                               Workflow trigger sync PostgreSQL tests.
  auth-security-persistence    Run focused Auth password policy, reset,
                               replay/expiry, session revocation, hash, and
                               rate-limit persistence PostgreSQL tests.
  migration-ledger-backfills   Run focused migration ledger, checksum, legacy
                               tenant-scope backfill, and Marketplace
                               category-ID backfill PostgreSQL tests.
  relay-file-mapping-tenant-ownership
                               Run focused Relay file-mapping upload, mapped
                               get/list passthrough, and tenant ownership
                               PostgreSQL tests.
  relay-runtime-channel-isolation
                               Run focused Relay runtime channel selection,
                               model discovery, conversation affinity, and
                               SQL pool-loading active-organization isolation
                               tests.
  workflow-sql-isolation       Run focused Workflow SQL store and HTTP router
                               active-organization isolation PostgreSQL tests.
  publishing-channel-isolation Run focused Publishing channel real-router
                               active-organization isolation PostgreSQL tests.
  admin-relay-channel-isolation
                               Run focused Admin Relay channel real-router
                               active-organization isolation PostgreSQL tests.
  admin-relay-read-isolation   Run focused Admin Relay runtime stats and model
                               inventory active-organization read isolation
                               PostgreSQL tests.
  observability-alert-recovery-persistence
                               Run focused Observability alert routing,
                               lifecycle, delivery history, notification
                               throttle, and recovery cooldown PostgreSQL
                               tests.
  quota-sql-isolation          Run focused Quota SQL store tenant isolation,
                               Admin user quota route persistence, and HTTP
                               active-organization isolation PostgreSQL tests.
  core-sql-persistence         Run focused Chat SQL sharing/forking,
                               Publishing channel SQL retry/archive, and Relay
                               semantic-cache SQL persistence tests.

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
  local admin_billing_sql_shape_pattern
  local marketplace_settlement_pattern
  local admin_money_movement_pattern

  admin_billing_sql_shape_pattern="^(TestTopupSummaryQueryUsesPaymentIntentProviderFilter|TestMarketplaceSettlementQueriesUsePaymentIntentProviderFilter|TestRecordTopupRefundUpdatesOrderStatusAndRefundedAmount)$"
  marketplace_settlement_pattern="^(TestMarketplace(LifecycleTransitionKeyUsesSelectedProvider|RevenueTierDisclosureUsesSegmentedFees)|TestPaymentIntentKindMigrationAllowsMarketplaceInstall|TestSettlement(CreatePaidInstallCheckoutCreatesPendingOrderAndIntent|CreatePaidInstallCheckoutRecordsSelectedProvider|CreatePaidInstallCheckoutSQLUsesRequestedCurrency|MarkPaidInstallCheckoutFailedMarksOrderAndIntent|AppliesSegmentedPlatformFees|AppliesSpecSegmentedRevenueTiers|MinimumSettlementBlocksSmallPayoutUntilCycleElapsed|ApplyPaidInstallCheckoutCompletedRecordsSelectedProviderLifecycle|ApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce|ApplyRefundAdjustsOrderAndSettlementOnce|BuyerUninstallPreservesPaidOrderAndSettlement|PublisherDeleteWithPaidOrderIsRejectedAndPreservesAudit|AuditRetentionMigrationRejectsDirectAuditCascade|PayoutStateIsLocalOnly|MarkPayoutPendingDispatchesConfiguredProvider|MarkPayoutPaidUpdatesPayoutAndSettlementsOnce|ProviderPayoutPaidWebhookMatchesProviderPayoutIDOnce|ProviderPayoutFailedWebhookReleasesSettlementsOnce|MarkPayoutFailedReleasesSettlementsOnce|CreateDuePayoutsAggregatesAvailableSettlementsOnce|CreateDuePayoutsDispatchesConfiguredProvider|PublisherStatsIncludesSettlementAmounts))$"
  admin_money_movement_pattern="^Test(AdminBilling(SummaryIncludesMoneyMovementState|ListsExposeAllRequiredSurfaces|ListsApplyRecoveryFilters|SummaryAppliesFailedStatusFilter|MarksMarketplacePayoutPaid|MarksMarketplacePayoutFailedAndReleasesSettlements|CreateDueMarketplacePayoutsDispatchesConfiguredProvider|RecordsTopupRefundAndAdjustsQuota|RecordTopupRefundHandlerPassesDomesticPaymentIntentEvidence|RejectsTopupRefundWithoutOperatorEvidenceAndPreservesLedger|WebhookEventsDoNotExposeRawPayload)|MarketplacePaidInstall(DoesNotInstallBeforeWebhook|CheckoutCreatorFailureMarksOrderFailed|UsesConfiguredProviderCheckoutCreator)|MarketplacePublisherStatsIncludesSettlementAmounts|StripeWebhookRouteAppliesMarketplaceInstallSettlementOnce|StripeRefundUpdatesMarketplaceSettlementOnce|DomesticPaymentWebhookRouteAppliesMarketplace(InstallSettlementOnce|RefundOnce|PayoutPaidOnce|PayoutFailedOnce))$"
  run_go_test_no_skips "admin billing money movement SQL shape" "./internal/admin" "$admin_billing_sql_shape_pattern"
  run_go_test_no_skips "marketplace settlement money movement" "./internal/marketplace" "$marketplace_settlement_pattern"
  run_go_test_no_skips "admin billing and marketplace money movement routes" "./internal/http" "$admin_money_movement_pattern"
}

run_marketplace_governance_review_profile() {
  local marketplace_governance_pattern
  local marketplace_http_pattern

  marketplace_governance_pattern="^Test(Governance(TakedownPreventsNewInstallsAndPreservesHistory|AppealAndReinstateRecordEvents|AbuseReportLifecycle|ListsOpenAbuseReportsForReviewQueue|AbuseReportNotifiesPublisher|RequestsPublisherChangesForPendingReview)|AutomatedReview(AllowsCleanAgentToWaitForManualReview|RejectsPromptInjectionAndSensitiveAPIFindings))$"
  marketplace_http_pattern="^Test(AdminReviewSLAEnforceRouteScansPendingReviewsAndAlerts|Marketplace(GovernanceTakedownAppealAndReinstate|AbuseReportLifecycle|AdminReviewNeedsChangesRoute|PublishRunsAutomatedReviewGovernance)|AdminMarketplaceListsOpenAbuseReports)$"
  run_go_test_no_skips "marketplace governance and automated review persistence" "./internal/marketplace" "$marketplace_governance_pattern"
  run_go_test_no_skips "marketplace governance and review HTTP routes" "./internal/http" "$marketplace_http_pattern"
}

run_marketplace_recommendation_search_profile() {
  local marketplace_recommendation_pattern

  marketplace_recommendation_pattern="^TestSearchAgentsRecommended(RanksContentMatchesOverGenericHotAgents|FallbackExplorationIsDeterministicAndNonEmpty|UsesRankingSignals|UsesCollaborativeFilteringForRequester|DemotesGovernanceWeightedAgents)$"
  run_go_test_no_skips "marketplace recommendation search persistence" "./internal/marketplace" "$marketplace_recommendation_pattern"
}

run_marketplace_template_routes_profile() {
  run_go_test_no_skips "marketplace template route persistence" "./internal/http" "^TestMarketplaceTemplateRoutesCreateListDetailAndInstall$"
}

run_billing_checkout_topup_http_profile() {
  local billing_checkout_topup_pattern

  billing_checkout_topup_pattern="^Test(BillingCheckout(RequiresSession|PersistsTenantPaymentIntent|ExplicitStripeUsesExistingCheckout|UnconfiguredProvidersDoNotCreateArtifacts|CreatorFailureMarksTopupFailed|UsesConfiguredProviderCheckoutCreator|UsesConfiguredDomesticProviderFromRouterConfig|TopupDoesNotCreditQuotaBeforeWebhook)|QuotaTopupEndpointNoLongerCreditsWithoutPayment|DomesticPaymentWebhookRoutesVerifySignatureAndRecordEvents|DomesticPaymentWebhookRouteApplies(Topup(LifecycleOnce|RefundOnce)|SubscriptionLifecycleOnce)|StripeWebhookRoute(RejectsInvalidSignature|RecordsSignedEventOnce|AppliesCheckoutCompletedSubscriptionOnce|RetriesLifecycleForRecordedDuplicateEvent))$"
  run_go_test_no_skips "billing checkout and top-up HTTP persistence" "./internal/http" "$billing_checkout_topup_pattern"
}

run_billing_provider_lifecycle_profile() {
  local billing_provider_lifecycle_pattern

  billing_provider_lifecycle_pattern="^(TestApplyRefundUpdatesTopupOrderStatusAndRefundedAmount|TestLifecycleAppliesDomesticCheckoutPaidThroughCheckoutCompletion|TestLifecycleAppliesDomesticMarketplaceInstallThroughSettlementApplier|TestLifecycleAppliesDomesticRefundThroughRefundLifecycle|TestLifecycleAppliesDomesticMarketplaceRefundThroughSettlementApplier|TestLifecycleAppliesDomesticSubscriptionUpdatedThroughSubscriptionLifecycle|TestLifecycleAppliesDomesticSubscriptionDeletedThroughSubscriptionLifecycle|TestLifecycleApplyCheckoutSessionCompletedCreatesSubscriptionOnce|TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce|TestLifecycleApplyInvoicePaidAndPaymentFailedTransitions|TestLifecycleApplySubscriptionUpdatedAndDeletedTransitions|TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup)$"
  run_go_test_no_skips "billing provider lifecycle" "./internal/stripe" "$billing_provider_lifecycle_pattern"
}

run_admin_usage_analytics_db_profile() {
  local admin_usage_analytics_pattern

  admin_usage_analytics_pattern="^TestSQLStore(UsageDailyAggregatesPostgresRefreshAndAnalytics|UsageAnalyticsRawRecordsFallsBackFromZeroTotalTokens|ListUsageLogsFallsBackFromZeroTotalTokens)$"
  run_go_test_no_skips "admin usage analytics daily aggregate persistence" "./internal/admin" "$admin_usage_analytics_pattern"
}

run_app_stateful_routes_profile() {
  local app_stateful_routes_pattern

  app_stateful_routes_pattern="^Test(ConsoleAPITokenCreateListAndRevoke|ConsoleUsageReflectsRecordedChatRequests|ConsoleUsageListsCurrentUserRecentRelayRequests|SelectOrganizationRequiresMembershipAndUpdatesSessionScope|OrganizationInvitationRevokeRejectsAcceptance|OrganizationSessionSecurityOnMembershipChanges|NotificationMutationRoutesEnforceOwnership|GetPreferencesReturnsUserInitializationState|UpdatePreferencesPersistsOnboardingState|ConversationAndMessageFlow|ConversationConfigFlow|TaskApproveRouteRequiresAwaitingConfirmationAndWorkspaceScope|RouteSurface(RequiresSessionForAppRoutes|RejectsCookieMutationWithoutCSRF))$"
  run_go_test_no_skips "app stateful route persistence and ownership" "./internal/http" "$app_stateful_routes_pattern"
}

run_tenant_membership_lifecycle_profile() {
  local tenant_store_pattern
  local tenant_http_pattern

  tenant_store_pattern="^TestSQLStore(OrganizationLifecycle|MembershipInvitationOwnershipLifecycle)$"
  tenant_http_pattern="^Test(RegisterCreatesDefaultOrganizationAndSessionScope|LoginResolvesDefaultOrganizationForLegacyUser|SelectOrganizationRequiresMembershipAndUpdatesSessionScope|OrganizationInvitationRevokeRejectsAcceptance|OrganizationSessionSecurityOnMembershipChanges|OrganizationMemberRoutesListTransferOwnershipAndRemove|AdminOrganizationRoutesPersistWithPostgres)$"
  run_go_test_no_skips "tenant SQL organization and membership lifecycle" "./internal/tenant" "$tenant_store_pattern"
  run_go_test_no_skips "tenant HTTP membership ownership lifecycle" "./internal/http" "$tenant_http_pattern"
}

run_tenant_cross_surface_profile() {
  local tenant_cross_surface_pattern

  tenant_cross_surface_pattern="^Test(CrossTenant(ChatScopeUsesActiveOrganization|KnowledgeScopeDeniesReadWriteAndAttach|ConsoleUsageUsesActiveOrganization|AgentScopeDeniesReadWriteAndConversation|MemoryScopeDeniesReadWrite|MCPScopeDeniesReadWriteAndConnect|QuotaScopeUsesActiveOrganization|ScheduledTaskScope|MarketplacePublisherScopeUsesActiveOrganization)|MarketplacePublisherSettlementPreferencesUseActiveOrganization|AgentRunStatusEndpointsExposeTenantScopedRunDetail|AgentToolRunApprovalRejectRetryEndpointsAreTenantScoped)$"
  run_go_test_no_skips "tenant cross-surface app isolation" "./internal/http" "$tenant_cross_surface_pattern"
}

run_secret_response_safety_profile() {
  local secret_response_safety_pattern

  secret_response_safety_pattern="^Test(ObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted|PublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers|AdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers|WorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers)$"
  run_go_test_no_skips "secret response safety" "./internal/http" "$secret_response_safety_pattern"
  run_go_test_no_skips "MCP auth token at-rest and response-listing safety" "./internal/mcp" "^TestSQLStoreProtectsAuthTokenWithPostgres$"
}

run_agent_runtime_memory_profile() {
  local agent_runtime_memory_pattern
  local agent_react_runtime_pattern
  local agent_tool_runtime_pattern

  agent_runtime_memory_pattern="^(TestAgentRunStorePersistsRunLifecycle|TestAgentToolRunStorePersistsToolLifecycle|TestAgentToolRunStorePersistsRiskLevel|TestAgentPlanStepStore(RoundTripsStepsInOrder|UpdatesStatusAndExecutionResult)|TestAgentSQLStorePersists(ApprovalConfigAndToolRiskLevels|DefaultExecutionModeConfig|LongTermMemoryWritePolicyConfig)|TestAgentMemoryStorePersistsAndFiltersMemories)$"
  agent_react_runtime_pattern="^(TestExecuteReActWithModelRouting|TestExecuteReActModelSwitching|TestExecuteReActWithSkillSelection|TestBuildToolsFromSkills|TestInjectSkillInstructions)$"
  agent_tool_runtime_pattern="^(TestCallAgentTool(Registration(_RecursionLimit)?|_RecursionDepthGuard)|TestWebsearchTool_(PrimarySuccess|FallbackChain|AllProvidersExhausted|MissingProviderInMap|EmptyFallback|NoProviders|IntegrationWithConfig|IntegrationWithMockProviders|DefaultProvider|MultipleProvidersChain))$"
  run_go_test_no_skips "agent ReAct model routing and skill runtime" "./internal/agent" "$agent_react_runtime_pattern"
  run_go_test_no_skips "agent tool delegation and websearch fallback runtime" "./internal/agent/tools" "$agent_tool_runtime_pattern"
  run_go_test_no_skips "agent runtime, memory store, and memory policy persistence" "./internal/agent" "$agent_runtime_memory_pattern"
}

run_scheduled_task_runtime_profile() {
  local scheduled_task_store_pattern
  local scheduled_task_routes_pattern

  scheduled_task_store_pattern="^TestSQLStore(CreatesAndListsScheduledTasksByOrganization|SyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks|GetsAndUpdatesScheduledTaskEnabledState|RecordsAndListsScheduledTaskRunsByOrganizationAndTask|ClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns|CompletesManualScheduledTaskRunWithoutAdvancingNextRun|CompletesScheduledTaskRunAndAdvancesTask|FailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim)$"
  scheduled_task_routes_pattern="^(TestScheduledTasksRoute(CreatesAndListsTasks|ListsRunsForTaskWithinSessionOrganization)|TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks)$"
  run_go_test_no_skips "scheduled task SQL runtime persistence" "./internal/schedule" "$scheduled_task_store_pattern"
  run_go_test_no_skips "scheduled task route and workflow sync persistence" "./internal/http" "$scheduled_task_routes_pattern"
}

run_auth_security_persistence_profile() {
  local auth_store_pattern
  local auth_http_pattern

  auth_store_pattern="^(TestPasswordPolicyResetAndSessionRevocation|TestPasswordResetTokenReplayExpiryAndUnknownEmailFailClosed|TestSQLRateLimiterPersistsBlocks)$"
  auth_http_pattern="^Test(RegisterLoginMeLogoutFlow|AuthRateLimitRejectsRepeatedFailedLogin|PasswordResetRoutesConfirmAndRevokeSessions|PasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv|RegisterStoresHashedPassword|LoginAcceptsRawPasswordAgainstStoredHash|MeRequiresSession|AuthResponsesExposeStableUserAndPreferenceContracts|SensitiveOrganizationActionsAreRateLimited)$"
  run_go_test_no_skips "auth password reset and rate-limit persistence" "./internal/auth" "$auth_store_pattern"
  run_go_test_no_skips "auth HTTP persistence and security routes" "./internal/http" "$auth_http_pattern"
}

run_migration_ledger_backfills_profile() {
  local migration_ledger_backfills_pattern

  migration_ledger_backfills_pattern="^TestApplyMigrations(RecordsLedgerAndSkipsAppliedFiles|RejectsChecksumMismatch|BackfillsLegacyTenantScopeData|BackfillsMarketplaceCategoryIDs)$"
  run_go_test_no_skips "migration ledger replay and backfill persistence" "./cmd/migrate" "$migration_ledger_backfills_pattern"
}

run_relay_file_mapping_tenant_ownership_profile() {
  local relay_file_mapping_pattern

  relay_file_mapping_pattern="^(TestRelayStore(SaveFileMappingPersistsTenantOwnership|GetFileMappingRequiresTenantOwnership|ListFileMappingsRequiresTenantOwnership)|TestNewRelayFilesSQLRelayStoreUploadGetTenantFailClosed)$"
  run_go_test_no_skips "relay file mapping tenant ownership" "./internal/relay" "$relay_file_mapping_pattern"
}

run_relay_runtime_channel_isolation_profile() {
  local relay_runtime_pattern

  relay_runtime_pattern="^Test(LoadBalancerSelectModelForOrganizationFiltersModelRouteAndFallback|RouterRouteWithBillingUsesTrustedOrganizationForChannelSelectionAndAffinity|RelayStore(ConversationAffinityPersistsAndUpdatesChannel|LoadPoolPreservesChannelOrganizationScope|ProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey))$"
  run_go_test_no_skips "relay runtime channel active-organization isolation" "./internal/relay" "$relay_runtime_pattern"
  run_go_test_no_skips "relay models active-organization isolation" "./internal/relay/handler" "^TestModelsHandlerScopesModelsToTrustedOrganization$"
}

run_workflow_sql_isolation_profile() {
  local workflow_store_pattern
  local workflow_http_pattern

  workflow_store_pattern="^TestWorkflowStorePersists(DefinitionsAndExecutions|VersionHistoryAndExecutionVersion)$"
  workflow_http_pattern="^TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution$"
  run_go_test_no_skips "workflow SQL store tenant isolation" "./internal/workflow" "$workflow_store_pattern"
  run_go_test_no_skips "workflow HTTP active-organization isolation" "./internal/http" "$workflow_http_pattern"
}

run_publishing_channel_isolation_profile() {
  run_go_test_no_skips "publishing channel HTTP active-organization isolation" "./internal/http" "^TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation$"
}

run_admin_relay_channel_isolation_profile() {
  run_go_test_no_skips "admin relay channel HTTP active-organization isolation" "./internal/http" "^TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation$"
}

run_admin_relay_read_isolation_profile() {
  run_go_test_no_skips "admin relay read-surface HTTP active-organization isolation" "./internal/http" "^TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization$"
}

run_observability_alert_recovery_persistence_profile() {
  local observability_alert_recovery_pattern

  observability_alert_recovery_pattern="^TestSQLAlert(RoutingRuleStorePersistsRoutingRules|StateStore(PersistsAlertLifecycleAndEscalation|ListsAlertStatesWithFilters|PersistsNotificationThrottleAndRecoveryCooldown|RecordsRepeatedDeliveryBatchesForSameAlert))$"
  run_go_test_no_skips "observability alert routing and recovery persistence" "./internal/observability" "$observability_alert_recovery_pattern"
}

run_quota_sql_isolation_profile() {
  local quota_store_pattern
  local quota_http_pattern
  local quota_stripe_lifecycle_pattern

  quota_store_pattern="^TestSQLStore(UsageLimitSettingsRoundTrip|UserQuotaModeUsesUserScopedBalance|BillingSessionsAreOrganizationScoped|TopupOrderMutationsRequireOrganizationScope|ResolveUsageLimitFallsBackToActiveSubscriptionRequestCap|ListPackagesReturnsOnlyActivePublicHybridPlans)$"
  quota_http_pattern="^Test(CrossTenantQuotaScopeUsesActiveOrganization|AdminUsageLimitSettingsRoutePersistsWithPostgres|AdminUserQuotaRoutePersistsWithPostgres|BillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook|QuotaTopupEndpointNoLongerCreditsWithoutPayment|AdminBillingRecordsTopupRefundAndAdjustsQuota)$"
  quota_stripe_lifecycle_pattern="^TestLifecycleApply(CheckoutSessionCompletedFulfillsTopupOnce|RefundRecordsRefundAndAdjustsTopup)$"
  run_go_test_no_skips "quota SQL tenant isolation" "./internal/quota" "$quota_store_pattern"
  run_go_test_no_skips "quota HTTP lifecycle and active-organization isolation" "./internal/http" "$quota_http_pattern"
  run_go_test_no_skips "quota provider lifecycle balance accounting" "./internal/stripe" "$quota_stripe_lifecycle_pattern"
}

run_core_sql_persistence_profile() {
  local chat_store_pattern
  local channel_store_pattern
  local relay_semantic_cache_pattern

  chat_store_pattern="^TestSQLStore(MessageShareExpiresAndReadsPublicPayload|ConversationShareReturnsRequestedMessageRange|ForkConversationCopiesScopedConversationData|ConversationConfigPersistsPersonaID|CreateAndListMessagePreservesAttachments|CreateAndListMessagePreservesKnowledgeCitations|ListPersonasScopesOrganizationAndOrdersByName|ForkConversationCopiesMessagesThroughBoundary|ForkConversationCopiesMessageAttachments|ListConversationsMarksThreadsWithBookmarkedMessages)$"
  channel_store_pattern="^TestChannelSQLStore(PersistsConfigsAndMessageLogs|CountsConsecutiveSuccessfulOutboundDeliveries|ListsAndClaimsDueRetryMessages|ListsAndClaimsDueRetryMessagesForSpecificChannel|ForceClaimsFutureRetryMessagesForManualFailover|ArchivesExpiredMessageLogsWithoutDeletingRetryQueue|ArchivesExpiredMessageLogsToObjectBeforeDeleting)$"
  relay_semantic_cache_pattern="^TestSQLSemanticCacheStore(UsesPgvectorForSimilarityLookup|PersistsEntriesAndHitCounts)$"
  run_go_test_no_skips "chat SQL sharing, config, attachment, citation, and fork persistence" "./internal/chat" "$chat_store_pattern"
  run_go_test_no_skips "publishing channel SQL config, retry, failover, and archive persistence" "./internal/channel" "$channel_store_pattern"
  run_go_test_no_skips "relay semantic cache SQL persistence" "./internal/relay/cache" "$relay_semantic_cache_pattern"
}

run_all_profiles() {
  run_backend_journey_profile
  run_marketplace_money_movement_profile
  run_marketplace_governance_review_profile
  run_marketplace_recommendation_search_profile
  run_marketplace_template_routes_profile
  run_billing_checkout_topup_http_profile
  run_billing_provider_lifecycle_profile
  run_admin_usage_analytics_db_profile
  run_app_stateful_routes_profile
  run_tenant_membership_lifecycle_profile
  run_tenant_cross_surface_profile
  run_secret_response_safety_profile
  run_agent_runtime_memory_profile
  run_scheduled_task_runtime_profile
  run_auth_security_persistence_profile
  run_migration_ledger_backfills_profile
  run_relay_file_mapping_tenant_ownership_profile
  run_relay_runtime_channel_isolation_profile
  run_workflow_sql_isolation_profile
  run_publishing_channel_isolation_profile
  run_admin_relay_channel_isolation_profile
  run_admin_relay_read_isolation_profile
  run_observability_alert_recovery_persistence_profile
  run_quota_sql_isolation_profile
  run_core_sql_persistence_profile
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
  marketplace-governance-review)
    run_marketplace_governance_review_profile
    ;;
  marketplace-recommendation-search)
    run_marketplace_recommendation_search_profile
    ;;
  marketplace-template-routes)
    run_marketplace_template_routes_profile
    ;;
  billing-checkout-topup-http)
    run_billing_checkout_topup_http_profile
    ;;
  billing-provider-lifecycle)
    run_billing_provider_lifecycle_profile
    ;;
  admin-usage-analytics-db)
    run_admin_usage_analytics_db_profile
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
  secret-response-safety)
    run_secret_response_safety_profile
    ;;
  agent-runtime-memory)
    run_agent_runtime_memory_profile
    ;;
  scheduled-task-runtime)
    run_scheduled_task_runtime_profile
    ;;
  auth-security-persistence)
    run_auth_security_persistence_profile
    ;;
  migration-ledger-backfills)
    run_migration_ledger_backfills_profile
    ;;
  relay-file-mapping-tenant-ownership)
    run_relay_file_mapping_tenant_ownership_profile
    ;;
  relay-runtime-channel-isolation)
    run_relay_runtime_channel_isolation_profile
    ;;
  workflow-sql-isolation)
    run_workflow_sql_isolation_profile
    ;;
  publishing-channel-isolation)
    run_publishing_channel_isolation_profile
    ;;
  admin-relay-channel-isolation)
    run_admin_relay_channel_isolation_profile
    ;;
  admin-relay-read-isolation)
    run_admin_relay_read_isolation_profile
    ;;
  observability-alert-recovery-persistence)
    run_observability_alert_recovery_persistence_profile
    ;;
  quota-sql-isolation)
    run_quota_sql_isolation_profile
    ;;
  core-sql-persistence)
    run_core_sql_persistence_profile
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
