#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
target="${COMMERCIAL_DB_EVIDENCE_TARGET:-$repo_root/scripts/verify-commercial-db-evidence.sh}"

fail() {
  echo "[commercial-db-evidence-profiles] $*" >&2
  exit 1
}

profile_body_has_token() {
  local body="$1"
  local token="$2"

  [[ "$body" =~ (^|[^[:alnum:]_])${token}([^[:alnum:]_]|$) ]]
}

require_profile_token() {
  local body="$1"
  local profile="$2"
  local token="$3"
  local test_name="$4"

  if ! profile_body_has_token "$body" "$token"; then
    fail "$profile must include $test_name"
  fi
}

profile_name_from_function() {
  local function_name="$1"
  function_name="${function_name#run_}"
  function_name="${function_name%_profile}"
  echo "${function_name//_/-}"
}

declare -A usage_profiles=()
declare -A case_profiles=()
declare -A all_profiles=()
declare -A help_profiles=()
required_profiles=(
  "migration-ledger-backfills"
)

usage_line=$(grep -E '^Usage: bash scripts/verify-commercial-db-evidence\.sh \[' "$target" | head -n 1 || true)
if [[ -z "$usage_line" ]]; then
  fail "usage line is missing the profile list"
fi

usage_list="${usage_line#*[}"
usage_list="${usage_list%]*}"
IFS='|' read -r -a usage_items <<< "$usage_list"
for profile in "${usage_items[@]}"; do
  [[ -n "$profile" ]] || fail "usage profile list contains an empty entry"
  usage_profiles["$profile"]=1
done

while IFS= read -r profile; do
  [[ -n "$profile" ]] || continue
  case_profiles["$profile"]=1
done < <(
  sed -n '/^case "\$profile" in/,/^esac/p' "$target" |
    sed -n 's/^  \([a-z0-9][a-z0-9-]*\))$/\1/p'
)

while IFS= read -r function_name; do
  [[ -n "$function_name" ]] || continue
  profile=$(profile_name_from_function "$function_name")
  all_profiles["$profile"]=1
done < <(
  awk '
    /^run_all_profiles\(\) \{/ { in_all = 1; next }
    in_all && /^}/ { in_all = 0 }
    in_all { print }
  ' "$target" | sed -n 's/^[[:space:]]*\(run_[a-z0-9_]*_profile\)$/\1/p'
)

while IFS= read -r profile; do
  [[ -n "$profile" ]] || continue
  help_profiles["$profile"]=1
done < <(
  sed -n '/^Profiles:/,/^Environment:/p' "$target" |
    sed -n 's/^  \([a-z0-9][a-z0-9-]*\)\([[:space:]].*\)\{0,1\}$/\1/p'
)

[[ "${usage_profiles[all]:-}" == "1" ]] || fail "usage profile list must include all"
[[ "${case_profiles[all]:-}" == "1" ]] || fail "case statement must include all"

for profile in "${required_profiles[@]}"; do
  [[ "${usage_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from usage list: $profile"
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from case statement: $profile"
  [[ "${all_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from run_all_profiles: $profile"
  [[ "${help_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from help section: $profile"
done

for profile in "${!case_profiles[@]}"; do
  [[ "$profile" == "all" ]] && continue
  [[ "${usage_profiles[$profile]:-}" == "1" ]] || fail "usage profile list is missing $profile"
  [[ "${all_profiles[$profile]:-}" == "1" ]] || fail "run_all_profiles is missing $profile"
  [[ "${help_profiles[$profile]:-}" == "1" ]] || fail "Profiles help section is missing $profile"
done

for profile in "${!usage_profiles[@]}"; do
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "usage profile list includes unknown profile $profile"
done

for profile in "${!all_profiles[@]}"; do
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "run_all_profiles includes unknown profile $profile"
done

for profile in "${!help_profiles[@]}"; do
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "help section includes unknown profile $profile"
done

backend_journey_body=$(sed -n '/^run_backend_journey_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$backend_journey_body" "TestCommercialHTTPJourney"; then
  fail "backend-journey must include TestCommercialHTTPJourney"
fi

billing_checkout_topup_body=$(sed -n '/^run_billing_checkout_topup_http_profile() {/,/^}/p' "$target")
billing_checkout_topup_required=(
  "TestBillingCheckoutRequiresSession|TestBillingCheckoutRequiresSession"
  "TestBillingCheckoutPersistsTenantPaymentIntent|TestBillingCheckoutPersistsTenantPaymentIntent"
  "TestBillingCheckoutExplicitStripeUsesExistingCheckout|TestBillingCheckoutExplicitStripeUsesExistingCheckout"
  "TestBillingCheckoutUnconfiguredProvidersDoNotCreateArtifacts|TestBillingCheckoutUnconfiguredProvidersDoNotCreateArtifacts"
  "TestBillingCheckoutCreatorFailureMarksTopupFailed|TestBillingCheckoutCreatorFailureMarksTopupFailed"
  "TestBillingCheckoutUsesConfiguredProviderCheckoutCreator|TestBillingCheckoutUsesConfiguredProviderCheckoutCreator"
  "TestBillingCheckoutUsesConfiguredDomesticProviderFromRouterConfig|TestBillingCheckoutUsesConfiguredDomesticProviderFromRouterConfig"
  "TestConsoleBillingPaymentProvidersExposeConfiguredCheckoutProviders|TestConsoleBillingPaymentProvidersExposeConfiguredCheckoutProviders"
  "TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook|TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook"
  "TestQuotaTopupEndpointNoLongerCreditsWithoutPayment|TestQuotaTopupEndpointNoLongerCreditsWithoutPayment"
  "TestDomesticPaymentWebhookRoutesVerifySignatureAndRecordEvents|TestDomesticPaymentWebhookRoutesVerifySignatureAndRecordEvents"
  "TestDomesticPaymentWebhookRouteAppliesTopupLifecycleOnce|TestDomesticPaymentWebhookRouteAppliesTopupLifecycleOnce"
  "TestDomesticPaymentWebhookRouteAppliesTopupRefundOnce|TestDomesticPaymentWebhookRouteAppliesTopupRefundOnce"
  "TestDomesticPaymentWebhookRouteAppliesSubscriptionLifecycleOnce|TestDomesticPaymentWebhookRouteAppliesSubscriptionLifecycleOnce"
  "TestStripeWebhookRouteRejectsInvalidSignature|TestStripeWebhookRouteRejectsInvalidSignature"
  "TestStripeWebhookRouteRecordsSignedEventOnce|TestStripeWebhookRouteRecordsSignedEventOnce"
  "TestStripeWebhookRouteAppliesCheckoutCompletedSubscriptionOnce|TestStripeWebhookRouteAppliesCheckoutCompletedSubscriptionOnce"
  "TestStripeWebhookRouteRetriesLifecycleForRecordedDuplicateEvent|TestStripeWebhookRouteRetriesLifecycleForRecordedDuplicateEvent"
)
for entry in "${billing_checkout_topup_required[@]}"; do
  IFS='|' read -r token test_name <<< "$entry"
  require_profile_token "$billing_checkout_topup_body" "billing-checkout-topup-http" "$token" "$test_name"
done

billing_provider_lifecycle_body=$(sed -n '/^run_billing_provider_lifecycle_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestApplyRefundUpdatesTopupOrderStatusAndRefundedAmount"; then
  fail "billing-provider-lifecycle must include TestApplyRefundUpdatesTopupOrderStatusAndRefundedAmount"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleObservabilityRecordsCheckoutCompleted"; then
  fail "billing-provider-lifecycle must include TestLifecycleObservabilityRecordsCheckoutCompleted"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleAppliesDomesticCheckoutPaidThroughCheckoutCompletion"; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticCheckoutPaidThroughCheckoutCompletion"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleAppliesDomesticMarketplaceInstallThroughSettlementApplier"; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticMarketplaceInstallThroughSettlementApplier"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleAppliesDomesticRefundThroughRefundLifecycle"; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticRefundThroughRefundLifecycle"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleAppliesDomesticMarketplaceRefundThroughSettlementApplier"; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticMarketplaceRefundThroughSettlementApplier"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleAppliesDomesticSubscriptionUpdatedThroughSubscriptionLifecycle"; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticSubscriptionUpdatedThroughSubscriptionLifecycle"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleAppliesDomesticSubscriptionDeletedThroughSubscriptionLifecycle"; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticSubscriptionDeletedThroughSubscriptionLifecycle"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleApplyCheckoutSessionCompletedCreatesSubscriptionOnce"; then
  fail "billing-provider-lifecycle must include TestLifecycleApplyCheckoutSessionCompletedCreatesSubscriptionOnce"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce"; then
  fail "billing-provider-lifecycle must include TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleApplyInvoicePaidAndPaymentFailedTransitions"; then
  fail "billing-provider-lifecycle must include TestLifecycleApplyInvoicePaidAndPaymentFailedTransitions"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleApplySubscriptionUpdatedAndDeletedTransitions"; then
  fail "billing-provider-lifecycle must include TestLifecycleApplySubscriptionUpdatedAndDeletedTransitions"
fi
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup"; then
  fail "billing-provider-lifecycle must include TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup"
fi

admin_usage_analytics_db_body=$(sed -n '/^run_admin_usage_analytics_db_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$admin_usage_analytics_db_body" "TestSQLStoreUsageDailyAggregatesPostgresRefreshAndAnalytics"; then
  fail "admin-usage-analytics-db must include TestSQLStoreUsageDailyAggregatesPostgresRefreshAndAnalytics"
fi
if ! profile_body_has_token "$admin_usage_analytics_db_body" "TestSQLStoreUsageAnalyticsRawRecordsFallsBackFromZeroTotalTokens"; then
  fail "admin-usage-analytics-db must include TestSQLStoreUsageAnalyticsRawRecordsFallsBackFromZeroTotalTokens"
fi
if ! profile_body_has_token "$admin_usage_analytics_db_body" "TestSQLStoreListUsageLogsFallsBackFromZeroTotalTokens"; then
  fail "admin-usage-analytics-db must include TestSQLStoreListUsageLogsFallsBackFromZeroTotalTokens"
fi

app_stateful_routes_body=$(sed -n '/^run_app_stateful_routes_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$app_stateful_routes_body" "ConsoleAPITokenCreateListAndRevoke"; then
  fail "app-stateful-routes must include TestConsoleAPITokenCreateListAndRevoke"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "ConsoleUsageReflectsRecordedChatRequests"; then
  fail "app-stateful-routes must include TestConsoleUsageReflectsRecordedChatRequests"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "ConsoleUsageListsCurrentUserRecentRelayRequests"; then
  fail "app-stateful-routes must include TestConsoleUsageListsCurrentUserRecentRelayRequests"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "SelectOrganizationRequiresMembershipAndUpdatesSessionScope"; then
  fail "app-stateful-routes must include TestSelectOrganizationRequiresMembershipAndUpdatesSessionScope"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "OrganizationInvitationRevokeRejectsAcceptance"; then
  fail "app-stateful-routes must include TestOrganizationInvitationRevokeRejectsAcceptance"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "OrganizationSessionSecurityOnMembershipChanges"; then
  fail "app-stateful-routes must include TestOrganizationSessionSecurityOnMembershipChanges"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "NotificationMutationRoutesEnforceOwnership"; then
  fail "app-stateful-routes must include TestNotificationMutationRoutesEnforceOwnership"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "GetPreferencesReturnsUserInitializationState"; then
  fail "app-stateful-routes must include TestGetPreferencesReturnsUserInitializationState"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "UpdatePreferencesPersistsOnboardingState"; then
  fail "app-stateful-routes must include TestUpdatePreferencesPersistsOnboardingState"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "ConversationAndMessageFlow"; then
  fail "app-stateful-routes must include TestConversationAndMessageFlow"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "ConversationConfigFlow"; then
  fail "app-stateful-routes must include TestConversationConfigFlow"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "TaskApproveRouteRequiresAwaitingConfirmationAndWorkspaceScope"; then
  fail "app-stateful-routes must include TestTaskApproveRouteRequiresAwaitingConfirmationAndWorkspaceScope"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "RequiresSessionForAppRoutes"; then
  fail "app-stateful-routes must include TestRouteSurfaceRequiresSessionForAppRoutes"
fi
if ! profile_body_has_token "$app_stateful_routes_body" "RejectsCookieMutationWithoutCSRF"; then
  fail "app-stateful-routes must include TestRouteSurfaceRejectsCookieMutationWithoutCSRF"
fi

tenant_membership_lifecycle_body=$(sed -n '/^run_tenant_membership_lifecycle_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestSQLStoreOrganizationLifecycle"; then
  fail "tenant-membership-lifecycle must include TestSQLStoreOrganizationLifecycle"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestSQLStoreMembershipInvitationOwnershipLifecycle"; then
  fail "tenant-membership-lifecycle must include TestSQLStoreMembershipInvitationOwnershipLifecycle"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestRegisterCreatesDefaultOrganizationAndSessionScope"; then
  fail "tenant-membership-lifecycle must include TestRegisterCreatesDefaultOrganizationAndSessionScope"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestLoginResolvesDefaultOrganizationForLegacyUser"; then
  fail "tenant-membership-lifecycle must include TestLoginResolvesDefaultOrganizationForLegacyUser"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestSelectOrganizationRequiresMembershipAndUpdatesSessionScope"; then
  fail "tenant-membership-lifecycle must include TestSelectOrganizationRequiresMembershipAndUpdatesSessionScope"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestOrganizationInvitationRevokeRejectsAcceptance"; then
  fail "tenant-membership-lifecycle must include TestOrganizationInvitationRevokeRejectsAcceptance"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestOrganizationSessionSecurityOnMembershipChanges"; then
  fail "tenant-membership-lifecycle must include TestOrganizationSessionSecurityOnMembershipChanges"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestOrganizationMemberRoutesListTransferOwnershipAndRemove"; then
  fail "tenant-membership-lifecycle must include TestOrganizationMemberRoutesListTransferOwnershipAndRemove"
fi
if ! profile_body_has_token "$tenant_membership_lifecycle_body" "TestAdminOrganizationRoutesPersistWithPostgres"; then
  fail "tenant-membership-lifecycle must include TestAdminOrganizationRoutesPersistWithPostgres"
fi

tenant_cross_surface_body=$(sed -n '/^run_tenant_cross_surface_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$tenant_cross_surface_body" "ChatScopeUsesActiveOrganization"; then
  fail "tenant-cross-surface must include TestCrossTenantChatScopeUsesActiveOrganization"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "KnowledgeScopeDeniesReadWriteAndAttach"; then
  fail "tenant-cross-surface must include TestCrossTenantKnowledgeScopeDeniesReadWriteAndAttach"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "ConsoleUsageUsesActiveOrganization"; then
  fail "tenant-cross-surface must include TestCrossTenantConsoleUsageUsesActiveOrganization"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "AgentScopeDeniesReadWriteAndConversation"; then
  fail "tenant-cross-surface must include TestCrossTenantAgentScopeDeniesReadWriteAndConversation"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "MemoryScopeDeniesReadWrite"; then
  fail "tenant-cross-surface must include TestCrossTenantMemoryScopeDeniesReadWrite"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "MCPScopeDeniesReadWriteAndConnect"; then
  fail "tenant-cross-surface must include TestCrossTenantMCPScopeDeniesReadWriteAndConnect"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "QuotaScopeUsesActiveOrganization"; then
  fail "tenant-cross-surface must include TestCrossTenantQuotaScopeUsesActiveOrganization"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "ScheduledTaskScope"; then
  fail "tenant-cross-surface must include TestCrossTenantScheduledTaskScope"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "MarketplacePublisherScopeUsesActiveOrganization"; then
  fail "tenant-cross-surface must include TestCrossTenantMarketplacePublisherScopeUsesActiveOrganization"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "MarketplacePublisherSettlementPreferencesUseActiveOrganization"; then
  fail "tenant-cross-surface must include TestMarketplacePublisherSettlementPreferencesUseActiveOrganization"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "AgentRunStatusEndpointsExposeTenantScopedRunDetail"; then
  fail "tenant-cross-surface must include TestAgentRunStatusEndpointsExposeTenantScopedRunDetail"
fi
if ! profile_body_has_token "$tenant_cross_surface_body" "AgentToolRunApprovalRejectRetryEndpointsAreTenantScoped"; then
  fail "tenant-cross-surface must include TestAgentToolRunApprovalRejectRetryEndpointsAreTenantScoped"
fi

secret_response_safety_body=$(sed -n '/^run_secret_response_safety_profile() {/,/^}/p' "$target")
secret_response_safety_required=(
  "ObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted|TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted"
  "PublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers|TestPublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers"
  "AdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers|TestAdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers"
  "WorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers|TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers"
  "TestSQLStoreProtectsAuthTokenWithPostgres|TestSQLStoreProtectsAuthTokenWithPostgres"
)
for entry in "${secret_response_safety_required[@]}"; do
  IFS='|' read -r token test_name <<< "$entry"
  require_profile_token "$secret_response_safety_body" "secret-response-safety" "$token" "$test_name"
done

observability_alert_recovery_body=$(sed -n '/^run_observability_alert_recovery_persistence_profile() {/,/^}/p' "$target")
observability_alert_recovery_required=(
  "RoutingRuleStorePersistsRoutingRules|TestSQLAlertRoutingRuleStorePersistsRoutingRules"
  "PersistsAlertLifecycleAndEscalation|TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation"
  "ListsAlertStatesWithFilters|TestSQLAlertStateStoreListsAlertStatesWithFilters"
  "PersistsNotificationThrottleAndRecoveryCooldown|TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown"
  "RecordsRepeatedDeliveryBatchesForSameAlert|TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert"
)
for requirement in "${observability_alert_recovery_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$observability_alert_recovery_body" "observability-alert-recovery-persistence" "$token" "$test_name"
done

scheduled_task_runtime_body=$(sed -n '/^run_scheduled_task_runtime_profile() {/,/^}/p' "$target")
scheduled_task_runtime_required=(
  "CreatesAndListsScheduledTasksByOrganization|TestSQLStoreCreatesAndListsScheduledTasksByOrganization"
  "SyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks|TestSQLStoreSyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks"
  "GetsAndUpdatesScheduledTaskEnabledState|TestSQLStoreGetsAndUpdatesScheduledTaskEnabledState"
  "RecordsAndListsScheduledTaskRunsByOrganizationAndTask|TestSQLStoreRecordsAndListsScheduledTaskRunsByOrganizationAndTask"
  "ClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns|TestSQLStoreClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns"
  "CompletesManualScheduledTaskRunWithoutAdvancingNextRun|TestSQLStoreCompletesManualScheduledTaskRunWithoutAdvancingNextRun"
  "CompletesScheduledTaskRunAndAdvancesTask|TestSQLStoreCompletesScheduledTaskRunAndAdvancesTask"
  "FailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim|TestSQLStoreFailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim"
  "CreatesAndListsTasks|TestScheduledTasksRouteCreatesAndListsTasks"
  "ListsRunsForTaskWithinSessionOrganization|TestScheduledTasksRouteListsRunsForTaskWithinSessionOrganization"
  "TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks|TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks"
)
for requirement in "${scheduled_task_runtime_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$scheduled_task_runtime_body" "scheduled-task-runtime" "$token" "$test_name"
done

quota_sql_isolation_body=$(sed -n '/^run_quota_sql_isolation_profile() {/,/^}/p' "$target")
quota_sql_isolation_required=(
  "TestSQLStoreUsageLimitSettingsRoundTrip|TestSQLStoreUsageLimitSettingsRoundTrip"
  "TestSQLStoreUserQuotaModeUsesUserScopedBalance|TestSQLStoreUserQuotaModeUsesUserScopedBalance"
  "TestSQLStoreBillingSessionsAreOrganizationScoped|TestSQLStoreBillingSessionsAreOrganizationScoped"
  "TestSQLStoreTopupOrderMutationsRequireOrganizationScope|TestSQLStoreTopupOrderMutationsRequireOrganizationScope"
  "TestSQLStoreResolveUsageLimitFallsBackToActiveSubscriptionRequestCap|TestSQLStoreResolveUsageLimitFallsBackToActiveSubscriptionRequestCap"
  "TestSQLStoreListPackagesReturnsOnlyActivePublicHybridPlans|TestSQLStoreListPackagesReturnsOnlyActivePublicHybridPlans"
  "TestQuotaObservabilityRecordsSettlementFailure|TestQuotaObservabilityRecordsSettlementFailure"
  "TestCrossTenantQuotaScopeUsesActiveOrganization|TestCrossTenantQuotaScopeUsesActiveOrganization"
  "TestAdminUsageLimitSettingsRoutePersistsWithPostgres|TestAdminUsageLimitSettingsRoutePersistsWithPostgres"
  "TestAdminUserQuotaRoutePersistsWithPostgres|TestAdminUserQuotaRoutePersistsWithPostgres"
  "TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook|TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook"
  "TestQuotaTopupEndpointNoLongerCreditsWithoutPayment|TestQuotaTopupEndpointNoLongerCreditsWithoutPayment"
  "TestAdminBillingRecordsTopupRefundAndAdjustsQuota|TestAdminBillingRecordsTopupRefundAndAdjustsQuota"
  "TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce|TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce"
  "TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup|TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup"
)
for requirement in "${quota_sql_isolation_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$quota_sql_isolation_body" "quota-sql-isolation" "$token" "$test_name"
done

core_sql_persistence_body=$(sed -n '/^run_core_sql_persistence_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreMessageShareExpiresAndReadsPublicPayload"; then
  fail "core-sql-persistence must include TestSQLStoreMessageShareExpiresAndReadsPublicPayload"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreConversationShareReturnsRequestedMessageRange"; then
  fail "core-sql-persistence must include TestSQLStoreConversationShareReturnsRequestedMessageRange"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreForkConversationCopiesScopedConversationData"; then
  fail "core-sql-persistence must include TestSQLStoreForkConversationCopiesScopedConversationData"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreConversationConfigPersistsPersonaID"; then
  fail "core-sql-persistence must include TestSQLStoreConversationConfigPersistsPersonaID"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreCreateAndListMessagePreservesAttachments"; then
  fail "core-sql-persistence must include TestSQLStoreCreateAndListMessagePreservesAttachments"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreCreateAndListMessagePreservesKnowledgeCitations"; then
  fail "core-sql-persistence must include TestSQLStoreCreateAndListMessagePreservesKnowledgeCitations"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreListPersonasScopesOrganizationAndOrdersByName"; then
  fail "core-sql-persistence must include TestSQLStoreListPersonasScopesOrganizationAndOrdersByName"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreForkConversationCopiesMessagesThroughBoundary"; then
  fail "core-sql-persistence must include TestSQLStoreForkConversationCopiesMessagesThroughBoundary"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreForkConversationCopiesMessageAttachments"; then
  fail "core-sql-persistence must include TestSQLStoreForkConversationCopiesMessageAttachments"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLStoreListConversationsMarksThreadsWithBookmarkedMessages"; then
  fail "core-sql-persistence must include TestSQLStoreListConversationsMarksThreadsWithBookmarkedMessages"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStorePersistsConfigsAndMessageLogs"; then
  fail "core-sql-persistence must include TestChannelSQLStorePersistsConfigsAndMessageLogs"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStoreCountsConsecutiveSuccessfulOutboundDeliveries"; then
  fail "core-sql-persistence must include TestChannelSQLStoreCountsConsecutiveSuccessfulOutboundDeliveries"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStoreListsAndClaimsDueRetryMessages"; then
  fail "core-sql-persistence must include TestChannelSQLStoreListsAndClaimsDueRetryMessages"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStoreListsAndClaimsDueRetryMessagesForSpecificChannel"; then
  fail "core-sql-persistence must include TestChannelSQLStoreListsAndClaimsDueRetryMessagesForSpecificChannel"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStoreForceClaimsFutureRetryMessagesForManualFailover"; then
  fail "core-sql-persistence must include TestChannelSQLStoreForceClaimsFutureRetryMessagesForManualFailover"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStoreArchivesExpiredMessageLogsWithoutDeletingRetryQueue"; then
  fail "core-sql-persistence must include TestChannelSQLStoreArchivesExpiredMessageLogsWithoutDeletingRetryQueue"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestChannelSQLStoreArchivesExpiredMessageLogsToObjectBeforeDeleting"; then
  fail "core-sql-persistence must include TestChannelSQLStoreArchivesExpiredMessageLogsToObjectBeforeDeleting"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLSemanticCacheStoreUsesPgvectorForSimilarityLookup"; then
  fail "core-sql-persistence must include TestSQLSemanticCacheStoreUsesPgvectorForSimilarityLookup"
fi
if ! profile_body_has_token "$core_sql_persistence_body" "TestSQLSemanticCacheStorePersistsEntriesAndHitCounts"; then
  fail "core-sql-persistence must include TestSQLSemanticCacheStorePersistsEntriesAndHitCounts"
fi

auth_security_persistence_body=$(sed -n '/^run_auth_security_persistence_profile() {/,/^}/p' "$target")
auth_security_persistence_required=(
  "TestPasswordPolicyResetAndSessionRevocation|TestPasswordPolicyResetAndSessionRevocation"
  "TestPasswordResetTokenReplayExpiryAndUnknownEmailFailClosed|TestPasswordResetTokenReplayExpiryAndUnknownEmailFailClosed"
  "TestSQLRateLimiterPersistsBlocks|TestSQLRateLimiterPersistsBlocks"
  "RegisterLoginMeLogoutFlow|TestRegisterLoginMeLogoutFlow"
  "AuthRateLimitRejectsRepeatedFailedLogin|TestAuthRateLimitRejectsRepeatedFailedLogin"
  "PasswordResetRoutesConfirmAndRevokeSessions|TestPasswordResetRoutesConfirmAndRevokeSessions"
  "PasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv|TestPasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv"
  "RegisterStoresHashedPassword|TestRegisterStoresHashedPassword"
  "LoginAcceptsRawPasswordAgainstStoredHash|TestLoginAcceptsRawPasswordAgainstStoredHash"
  "MeRequiresSession|TestMeRequiresSession"
  "AuthResponsesExposeStableUserAndPreferenceContracts|TestAuthResponsesExposeStableUserAndPreferenceContracts"
  "SensitiveOrganizationActionsAreRateLimited|TestSensitiveOrganizationActionsAreRateLimited"
)
for requirement in "${auth_security_persistence_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$auth_security_persistence_body" "auth-security-persistence" "$token" "$test_name"
done

migration_ledger_backfills_body=$(sed -n '/^run_migration_ledger_backfills_profile() {/,/^}/p' "$target")
if [[ "$migration_ledger_backfills_body" != *"RecordsLedgerAndSkipsAppliedFiles"* ]]; then
  fail "migration-ledger-backfills must include TestApplyMigrationsRecordsLedgerAndSkipsAppliedFiles"
fi
if [[ "$migration_ledger_backfills_body" != *"RejectsChecksumMismatch"* ]]; then
  fail "migration-ledger-backfills must include TestApplyMigrationsRejectsChecksumMismatch"
fi
if [[ "$migration_ledger_backfills_body" != *"BackfillsLegacyTenantScopeData"* ]]; then
  fail "migration-ledger-backfills must include TestApplyMigrationsBackfillsLegacyTenantScopeData"
fi
if [[ "$migration_ledger_backfills_body" != *"BackfillsMarketplaceCategoryIDs"* ]]; then
  fail "migration-ledger-backfills must include TestApplyMigrationsBackfillsMarketplaceCategoryIDs"
fi

relay_file_mapping_body=$(sed -n '/^run_relay_file_mapping_tenant_ownership_profile() {/,/^}/p' "$target")
if [[ "$relay_file_mapping_body" != *"SaveFileMappingPersistsTenantOwnership"* ]]; then
  fail "relay-file-mapping-tenant-ownership must include TestRelayStoreSaveFileMappingPersistsTenantOwnership"
fi
if [[ "$relay_file_mapping_body" != *"GetFileMappingRequiresTenantOwnership"* ]]; then
  fail "relay-file-mapping-tenant-ownership must include TestRelayStoreGetFileMappingRequiresTenantOwnership"
fi
if [[ "$relay_file_mapping_body" != *"ListFileMappingsRequiresTenantOwnership"* ]]; then
  fail "relay-file-mapping-tenant-ownership must include TestRelayStoreListFileMappingsRequiresTenantOwnership"
fi
if [[ "$relay_file_mapping_body" != *"NewRelayFilesSQLRelayStoreUploadGetTenantFailClosed"* ]]; then
  fail "relay-file-mapping-tenant-ownership must include TestNewRelayFilesSQLRelayStoreUploadGetTenantFailClosed"
fi

publishing_channel_isolation_body=$(sed -n '/^run_publishing_channel_isolation_profile() {/,/^}/p' "$target")
if [[ "$publishing_channel_isolation_body" != *"TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation"* ]]; then
  fail "publishing-channel-isolation must include TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation"
fi

admin_relay_channel_isolation_body=$(sed -n '/^run_admin_relay_channel_isolation_profile() {/,/^}/p' "$target")
if [[ "$admin_relay_channel_isolation_body" != *"TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation"* ]]; then
  fail "admin-relay-channel-isolation must include TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation"
fi

admin_relay_read_isolation_body=$(sed -n '/^run_admin_relay_read_isolation_profile() {/,/^}/p' "$target")
if [[ "$admin_relay_read_isolation_body" != *"TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization"* ]]; then
  fail "admin-relay-read-isolation must include TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization"
fi

marketplace_template_routes_body=$(sed -n '/^run_marketplace_template_routes_profile() {/,/^}/p' "$target")
marketplace_template_routes_required=(
  "TestMarketplaceTemplateRoutesCreateListDetailAndInstall|TestMarketplaceTemplateRoutesCreateListDetailAndInstall"
  "TestMarketplaceRouterRegistersTemplateAndPublisherPreferenceRoutes|TestMarketplaceRouterRegistersTemplateAndPublisherPreferenceRoutes"
)
for entry in "${marketplace_template_routes_required[@]}"; do
  IFS='|' read -r token test_name <<< "$entry"
  require_profile_token "$marketplace_template_routes_body" "marketplace-template-routes" "$token" "$test_name"
done

marketplace_governance_review_body=$(sed -n '/^run_marketplace_governance_review_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$marketplace_governance_review_body" "TestGovernanceTakedownPreventsNewInstallsAndPreservesHistory"; then
  fail "marketplace-governance-review must include TestGovernanceTakedownPreventsNewInstallsAndPreservesHistory"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestGovernanceAppealAndReinstateRecordEvents"; then
  fail "marketplace-governance-review must include TestGovernanceAppealAndReinstateRecordEvents"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestGovernanceAbuseReportLifecycle"; then
  fail "marketplace-governance-review must include TestGovernanceAbuseReportLifecycle"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestGovernanceListsOpenAbuseReportsForReviewQueue"; then
  fail "marketplace-governance-review must include TestGovernanceListsOpenAbuseReportsForReviewQueue"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestGovernanceAbuseReportNotifiesPublisher"; then
  fail "marketplace-governance-review must include TestGovernanceAbuseReportNotifiesPublisher"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestGovernanceRequestsPublisherChangesForPendingReview"; then
  fail "marketplace-governance-review must include TestGovernanceRequestsPublisherChangesForPendingReview"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestAutomatedReviewAllowsCleanAgentToWaitForManualReview"; then
  fail "marketplace-governance-review must include TestAutomatedReviewAllowsCleanAgentToWaitForManualReview"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestAutomatedReviewRejectsPromptInjectionAndSensitiveAPIFindings"; then
  fail "marketplace-governance-review must include TestAutomatedReviewRejectsPromptInjectionAndSensitiveAPIFindings"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestAdminReviewSLAEnforceRouteScansPendingReviewsAndAlerts"; then
  fail "marketplace-governance-review must include TestAdminReviewSLAEnforceRouteScansPendingReviewsAndAlerts"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestMarketplaceGovernanceTakedownAppealAndReinstate"; then
  fail "marketplace-governance-review must include TestMarketplaceGovernanceTakedownAppealAndReinstate"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestMarketplaceAbuseReportLifecycle"; then
  fail "marketplace-governance-review must include TestMarketplaceAbuseReportLifecycle"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestMarketplaceAdminReviewAuditCarriesAgentOrganization"; then
  fail "marketplace-governance-review must include TestMarketplaceAdminReviewAuditCarriesAgentOrganization"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestMarketplaceAdminReviewNeedsChangesRoute"; then
  fail "marketplace-governance-review must include TestMarketplaceAdminReviewNeedsChangesRoute"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestMarketplacePublishRunsAutomatedReviewGovernance"; then
  fail "marketplace-governance-review must include TestMarketplacePublishRunsAutomatedReviewGovernance"
fi
if ! profile_body_has_token "$marketplace_governance_review_body" "TestAdminMarketplaceListsOpenAbuseReports"; then
  fail "marketplace-governance-review must include TestAdminMarketplaceListsOpenAbuseReports"
fi

marketplace_recommendation_search_body=$(sed -n '/^run_marketplace_recommendation_search_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$marketplace_recommendation_search_body" "RanksContentMatchesOverGenericHotAgents"; then
  fail "marketplace-recommendation-search must include TestSearchAgentsRecommendedRanksContentMatchesOverGenericHotAgents"
fi
if ! profile_body_has_token "$marketplace_recommendation_search_body" "FallbackExplorationIsDeterministicAndNonEmpty"; then
  fail "marketplace-recommendation-search must include TestSearchAgentsRecommendedFallbackExplorationIsDeterministicAndNonEmpty"
fi
if ! profile_body_has_token "$marketplace_recommendation_search_body" "UsesRankingSignals"; then
  fail "marketplace-recommendation-search must include TestSearchAgentsRecommendedUsesRankingSignals"
fi
if ! profile_body_has_token "$marketplace_recommendation_search_body" "UsesCollaborativeFilteringForRequester"; then
  fail "marketplace-recommendation-search must include TestSearchAgentsRecommendedUsesCollaborativeFilteringForRequester"
fi
if ! profile_body_has_token "$marketplace_recommendation_search_body" "DemotesGovernanceWeightedAgents"; then
  fail "marketplace-recommendation-search must include TestSearchAgentsRecommendedDemotesGovernanceWeightedAgents"
fi

marketplace_money_movement_body=$(sed -n '/^run_marketplace_money_movement_profile() {/,/^}/p' "$target")
marketplace_admin_billing_sql_shape_body=$(sed -n '/admin_billing_sql_shape_pattern=/p' "$target")
marketplace_settlement_body=$(sed -n '/marketplace_settlement_pattern=/p' "$target")
marketplace_admin_money_movement_body=$(sed -n '/admin_money_movement_pattern=/p' "$target")

marketplace_admin_billing_sql_shape_required=(
  "TestTopupSummaryQueryUsesPaymentIntentProviderFilter|TestTopupSummaryQueryUsesPaymentIntentProviderFilter"
  "TestMarketplaceSettlementQueriesUsePaymentIntentProviderFilter|TestMarketplaceSettlementQueriesUsePaymentIntentProviderFilter"
  "TestRecordTopupRefundUpdatesOrderStatusAndRefundedAmount|TestRecordTopupRefundUpdatesOrderStatusAndRefundedAmount"
)
for requirement in "${marketplace_admin_billing_sql_shape_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$marketplace_admin_billing_sql_shape_body" "marketplace-money-movement" "$token" "$test_name"
done

marketplace_settlement_required=(
  "TestMarketplaceLifecycleTransitionKeyUsesSelectedProvider|TestMarketplaceLifecycleTransitionKeyUsesSelectedProvider"
  "TestMarketplaceRevenueTierDisclosureUsesSegmentedFees|TestMarketplaceRevenueTierDisclosureUsesSegmentedFees"
  "TestPaymentIntentKindMigrationAllowsMarketplaceInstall|TestPaymentIntentKindMigrationAllowsMarketplaceInstall"
  "TestSettlementCreatePaidInstallCheckoutCreatesPendingOrderAndIntent|TestSettlementCreatePaidInstallCheckoutCreatesPendingOrderAndIntent"
  "TestSettlementCreatePaidInstallCheckoutRecordsSelectedProvider|TestSettlementCreatePaidInstallCheckoutRecordsSelectedProvider"
  "TestSettlementCreatePaidInstallCheckoutSQLUsesRequestedCurrency|TestSettlementCreatePaidInstallCheckoutSQLUsesRequestedCurrency"
  "TestSettlementMarkPaidInstallCheckoutFailedMarksOrderAndIntent|TestSettlementMarkPaidInstallCheckoutFailedMarksOrderAndIntent"
  "TestSettlementAppliesSegmentedPlatformFees|TestSettlementAppliesSegmentedPlatformFees"
  "TestSettlementAppliesSpecSegmentedRevenueTiers|TestSettlementAppliesSpecSegmentedRevenueTiers"
  "TestSettlementMinimumSettlementBlocksSmallPayoutUntilCycleElapsed|TestSettlementMinimumSettlementBlocksSmallPayoutUntilCycleElapsed"
  "TestSettlementApplyPaidInstallCheckoutCompletedRecordsSelectedProviderLifecycle|TestSettlementApplyPaidInstallCheckoutCompletedRecordsSelectedProviderLifecycle"
  "TestSettlementApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce|TestSettlementApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce"
  "TestSettlementApplyRefundAdjustsOrderAndSettlementOnce|TestSettlementApplyRefundAdjustsOrderAndSettlementOnce"
  "TestSettlementBuyerUninstallPreservesPaidOrderAndSettlement|TestSettlementBuyerUninstallPreservesPaidOrderAndSettlement"
  "TestSettlementPublisherDeleteWithPaidOrderIsRejectedAndPreservesAudit|TestSettlementPublisherDeleteWithPaidOrderIsRejectedAndPreservesAudit"
  "TestSettlementAuditRetentionMigrationRejectsDirectAuditCascade|TestSettlementAuditRetentionMigrationRejectsDirectAuditCascade"
  "TestSettlementPayoutStateIsLocalOnly|TestSettlementPayoutStateIsLocalOnly"
  "TestSettlementMarkPayoutPendingDispatchesConfiguredProvider|TestSettlementMarkPayoutPendingDispatchesConfiguredProvider"
  "TestSettlementMarkPayoutPaidUpdatesPayoutAndSettlementsOnce|TestSettlementMarkPayoutPaidUpdatesPayoutAndSettlementsOnce"
  "TestSettlementProviderPayoutPaidWebhookMatchesProviderPayoutIDOnce|TestSettlementProviderPayoutPaidWebhookMatchesProviderPayoutIDOnce"
  "TestSettlementProviderPayoutFailedWebhookReleasesSettlementsOnce|TestSettlementProviderPayoutFailedWebhookReleasesSettlementsOnce"
  "TestSettlementMarkPayoutFailedReleasesSettlementsOnce|TestSettlementMarkPayoutFailedReleasesSettlementsOnce"
  "TestSettlementCreateDuePayoutsAggregatesAvailableSettlementsOnce|TestSettlementCreateDuePayoutsAggregatesAvailableSettlementsOnce"
  "TestSettlementCreateDuePayoutsDispatchesConfiguredProvider|TestSettlementCreateDuePayoutsDispatchesConfiguredProvider"
  "TestSettlementPublisherStatsIncludesSettlementAmounts|TestSettlementPublisherStatsIncludesSettlementAmounts"
)
for requirement in "${marketplace_settlement_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$marketplace_settlement_body" "marketplace-money-movement" "$token" "$test_name"
done

marketplace_admin_money_movement_required=(
  "TestAdminBillingSummaryIncludesMoneyMovementState|TestAdminBillingSummaryIncludesMoneyMovementState"
  "TestAdminBillingListsExposeAllRequiredSurfaces|TestAdminBillingListsExposeAllRequiredSurfaces"
  "TestAdminBillingListsApplyRecoveryFilters|TestAdminBillingListsApplyRecoveryFilters"
  "TestAdminBillingListHandlersPassBillingFiltersWithoutDatabase|TestAdminBillingListHandlersPassBillingFiltersWithoutDatabase"
  "TestAdminBillingSummaryAppliesFailedStatusFilter|TestAdminBillingSummaryAppliesFailedStatusFilter"
  "TestAdminBillingMarksMarketplacePayoutPaid|TestAdminBillingMarksMarketplacePayoutPaid"
  "TestAdminBillingMarkPayoutPaidHandlerRejectsMissingProviderPayoutID|TestAdminBillingMarkPayoutPaidHandlerRejectsMissingProviderPayoutID"
  "TestAdminBillingMarksMarketplacePayoutFailedAndReleasesSettlements|TestAdminBillingMarksMarketplacePayoutFailedAndReleasesSettlements"
  "TestAdminBillingMarkPayoutFailedHandlerRejectsMissingOperatorEvidence|TestAdminBillingMarkPayoutFailedHandlerRejectsMissingOperatorEvidence"
  "TestAdminBillingCreateDueMarketplacePayoutsDispatchesConfiguredProvider|TestAdminBillingCreateDueMarketplacePayoutsDispatchesConfiguredProvider"
  "TestAdminBillingRecordsTopupRefundAndAdjustsQuota|TestAdminBillingRecordsTopupRefundAndAdjustsQuota"
  "TestAdminBillingRecordTopupRefundHandlerPassesDomesticPaymentIntentEvidence|TestAdminBillingRecordTopupRefundHandlerPassesDomesticPaymentIntentEvidence"
  "TestAdminBillingRecordTopupRefundHandlerRejectsMissingOperatorEvidence|TestAdminBillingRecordTopupRefundHandlerRejectsMissingOperatorEvidence"
  "TestAdminBillingRejectsTopupRefundWithoutOperatorEvidenceAndPreservesLedger|TestAdminBillingRejectsTopupRefundWithoutOperatorEvidenceAndPreservesLedger"
  "TestAdminBillingWebhookEventsDoNotExposeRawPayload|TestAdminBillingWebhookEventsDoNotExposeRawPayload"
  "TestMarketplaceAgentDetailExposesConfiguredDomesticPaymentProviders|TestMarketplaceAgentDetailExposesConfiguredDomesticPaymentProviders"
  "TestMarketplacePaidInstallDoesNotInstallBeforeWebhook|TestMarketplacePaidInstallDoesNotInstallBeforeWebhook"
  "TestMarketplacePaidInstallCheckoutCreatorFailureMarksOrderFailed|TestMarketplacePaidInstallCheckoutCreatorFailureMarksOrderFailed"
  "TestMarketplacePaidInstallUsesConfiguredProviderCheckoutCreator|TestMarketplacePaidInstallUsesConfiguredProviderCheckoutCreator"
  "TestMarketplacePaidInstallCheckoutUsesSelectedProviderAndReturnsCheckoutSession|TestMarketplacePaidInstallCheckoutUsesSelectedProviderAndReturnsCheckoutSession"
  "TestMarketplacePaidInstallCheckoutRejectsMissingProviderBeforeSettlement|TestMarketplacePaidInstallCheckoutRejectsMissingProviderBeforeSettlement"
  "TestMarketplacePaidInstallCheckoutRejectsConfiguredProviderWithoutCheckoutCreatorBeforeSettlement|TestMarketplacePaidInstallCheckoutRejectsConfiguredProviderWithoutCheckoutCreatorBeforeSettlement"
  "TestMarketplacePaidInstallCheckoutRejectsUnsupportedProviderBeforeSettlement|TestMarketplacePaidInstallCheckoutRejectsUnsupportedProviderBeforeSettlement"
  "TestMarketplacePublisherStatsIncludesSettlementAmounts|TestMarketplacePublisherStatsIncludesSettlementAmounts"
  "TestBuildPaymentCheckoutProvidersEnablesDomesticHostedProviders|TestBuildPaymentCheckoutProvidersEnablesDomesticHostedProviders"
  "TestStripeWebhookRouteAppliesMarketplaceInstallSettlementOnce|TestStripeWebhookRouteAppliesMarketplaceInstallSettlementOnce"
  "TestStripeRefundUpdatesMarketplaceSettlementOnce|TestStripeRefundUpdatesMarketplaceSettlementOnce"
  "TestDomesticPaymentWebhookHandlerAppliesPayoutLifecycleOnce|TestDomesticPaymentWebhookHandlerAppliesPayoutLifecycleOnce"
  "TestDomesticPaymentWebhookRouteAppliesMarketplaceInstallSettlementOnce|TestDomesticPaymentWebhookRouteAppliesMarketplaceInstallSettlementOnce"
  "TestDomesticPaymentWebhookRouteAppliesMarketplaceRefundOnce|TestDomesticPaymentWebhookRouteAppliesMarketplaceRefundOnce"
  "TestDomesticPaymentWebhookRouteAppliesMarketplacePayoutPaidOnce|TestDomesticPaymentWebhookRouteAppliesMarketplacePayoutPaidOnce"
  "TestDomesticPaymentWebhookRouteAppliesMarketplacePayoutFailedOnce|TestDomesticPaymentWebhookRouteAppliesMarketplacePayoutFailedOnce"
)
for requirement in "${marketplace_admin_money_movement_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$marketplace_admin_money_movement_body" "marketplace-money-movement" "$token" "$test_name"
done

agent_runtime_memory_body=$(sed -n '/^run_agent_runtime_memory_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$agent_runtime_memory_body" "TestExecuteReActWithModelRouting"; then
  fail "agent-runtime-memory must include TestExecuteReActWithModelRouting"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestExecuteReActModelSwitching"; then
  fail "agent-runtime-memory must include TestExecuteReActModelSwitching"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestExecuteReActWithSkillSelection"; then
  fail "agent-runtime-memory must include TestExecuteReActWithSkillSelection"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestBuildToolsFromSkills"; then
  fail "agent-runtime-memory must include TestBuildToolsFromSkills"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestInjectSkillInstructions"; then
  fail "agent-runtime-memory must include TestInjectSkillInstructions"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestCallAgentToolRegistration"; then
  fail "agent-runtime-memory must include TestCallAgentToolRegistration"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestCallAgentToolRegistration_RecursionLimit"; then
  fail "agent-runtime-memory must include TestCallAgentToolRegistration_RecursionLimit"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestCallAgentTool_RecursionDepthGuard"; then
  fail "agent-runtime-memory must include TestCallAgentTool_RecursionDepthGuard"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_PrimarySuccess"; then
  fail "agent-runtime-memory must include TestWebsearchTool_PrimarySuccess"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_FallbackChain"; then
  fail "agent-runtime-memory must include TestWebsearchTool_FallbackChain"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_AllProvidersExhausted"; then
  fail "agent-runtime-memory must include TestWebsearchTool_AllProvidersExhausted"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_MissingProviderInMap"; then
  fail "agent-runtime-memory must include TestWebsearchTool_MissingProviderInMap"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_EmptyFallback"; then
  fail "agent-runtime-memory must include TestWebsearchTool_EmptyFallback"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_NoProviders"; then
  fail "agent-runtime-memory must include TestWebsearchTool_NoProviders"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_IntegrationWithConfig"; then
  fail "agent-runtime-memory must include TestWebsearchTool_IntegrationWithConfig"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_IntegrationWithMockProviders"; then
  fail "agent-runtime-memory must include TestWebsearchTool_IntegrationWithMockProviders"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_DefaultProvider"; then
  fail "agent-runtime-memory must include TestWebsearchTool_DefaultProvider"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestWebsearchTool_MultipleProvidersChain"; then
  fail "agent-runtime-memory must include TestWebsearchTool_MultipleProvidersChain"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentRunStorePersistsRunLifecycle"; then
  fail "agent-runtime-memory must include TestAgentRunStorePersistsRunLifecycle"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentToolRunStorePersistsToolLifecycle"; then
  fail "agent-runtime-memory must include TestAgentToolRunStorePersistsToolLifecycle"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentToolRunStorePersistsRiskLevel"; then
  fail "agent-runtime-memory must include TestAgentToolRunStorePersistsRiskLevel"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentPlanStepStoreRoundTripsStepsInOrder"; then
  fail "agent-runtime-memory must include TestAgentPlanStepStoreRoundTripsStepsInOrder"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentPlanStepStoreUpdatesStatusAndExecutionResult"; then
  fail "agent-runtime-memory must include TestAgentPlanStepStoreUpdatesStatusAndExecutionResult"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentSQLStorePersistsApprovalConfigAndToolRiskLevels"; then
  fail "agent-runtime-memory must include TestAgentSQLStorePersistsApprovalConfigAndToolRiskLevels"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentSQLStorePersistsDefaultExecutionModeConfig"; then
  fail "agent-runtime-memory must include TestAgentSQLStorePersistsDefaultExecutionModeConfig"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentSQLStorePersistsLongTermMemoryWritePolicyConfig"; then
  fail "agent-runtime-memory must include TestAgentSQLStorePersistsLongTermMemoryWritePolicyConfig"
fi
if ! profile_body_has_token "$agent_runtime_memory_body" "TestAgentMemoryStorePersistsAndFiltersMemories"; then
  fail "agent-runtime-memory must include TestAgentMemoryStorePersistsAndFiltersMemories"
fi

relay_runtime_channel_isolation_body=$(sed -n '/^run_relay_runtime_channel_isolation_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$relay_runtime_channel_isolation_body" "LoadBalancerSelectModelForOrganizationFiltersModelRouteAndFallback"; then
  fail "relay-runtime-channel-isolation must include TestLoadBalancerSelectModelForOrganizationFiltersModelRouteAndFallback"
fi
if ! profile_body_has_token "$relay_runtime_channel_isolation_body" "RouterRouteWithBillingUsesTrustedOrganizationForChannelSelectionAndAffinity"; then
  fail "relay-runtime-channel-isolation must include TestRouterRouteWithBillingUsesTrustedOrganizationForChannelSelectionAndAffinity"
fi
if ! profile_body_has_token "$relay_runtime_channel_isolation_body" "ConversationAffinityPersistsAndUpdatesChannel"; then
  fail "relay-runtime-channel-isolation must include TestRelayStoreConversationAffinityPersistsAndUpdatesChannel"
fi
if ! profile_body_has_token "$relay_runtime_channel_isolation_body" "LoadPoolPreservesChannelOrganizationScope"; then
  fail "relay-runtime-channel-isolation must include TestRelayStoreLoadPoolPreservesChannelOrganizationScope"
fi
if ! profile_body_has_token "$relay_runtime_channel_isolation_body" "ProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey"; then
  fail "relay-runtime-channel-isolation must include TestRelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey"
fi
if ! profile_body_has_token "$relay_runtime_channel_isolation_body" "TestModelsHandlerScopesModelsToTrustedOrganization"; then
  fail "relay-runtime-channel-isolation must include TestModelsHandlerScopesModelsToTrustedOrganization"
fi

workflow_sql_isolation_body=$(sed -n '/^run_workflow_sql_isolation_profile() {/,/^}/p' "$target")
workflow_sql_isolation_required=(
  "TestWorkflowStorePersistsDefinitionsAndExecutions|TestWorkflowStorePersistsDefinitionsAndExecutions"
  "TestWorkflowStorePersistsVersionHistoryAndExecutionVersion|TestWorkflowStorePersistsVersionHistoryAndExecutionVersion"
  "TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution|TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution"
)
for requirement in "${workflow_sql_isolation_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$workflow_sql_isolation_body" "workflow-sql-isolation" "$token" "$test_name"
done

echo "[commercial-db-evidence-profiles] commercial DB evidence profile list is synchronized."
