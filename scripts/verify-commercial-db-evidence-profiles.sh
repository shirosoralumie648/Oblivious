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

require_profile_text() {
  local body="$1"
  local profile="$2"
  local text="$3"
  local description="$4"

  if [[ "$body" != *"$text"* ]]; then
    fail "$profile must use exact $description"
  fi
}

is_exact_go_test_pattern() {
  local pattern="$1"
  local single_pattern='^\^Test[[:alnum:]_]+\$$'
  local grouped_pattern='^\^\(Test[[:alnum:]_]+(\|Test[[:alnum:]_]+)*\)\$$'

  [[ "$pattern" =~ $single_pattern || "$pattern" =~ $grouped_pattern ]]
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
require_profile_text "$backend_journey_body" "backend-journey" 'backend_journey_pattern="^TestCommercialHTTPJourney$"' "backend commercial HTTP journey test pattern"

while IFS='=' read -r pattern_name pattern_value; do
  [[ -n "$pattern_name" ]] || continue
  if ! is_exact_go_test_pattern "$pattern_value"; then
    fail "$pattern_name must use exact anchored full Go test names"
  fi
done < <(
  sed -n 's/^[[:space:]]*\([a-z0-9_]*_pattern\)="\(\^.*\)"$/\1=\2/p' "$target"
)

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
app_stateful_routes_required=(
  "TestConsoleAPITokenCreateListAndRevoke|TestConsoleAPITokenCreateListAndRevoke"
  "TestConsoleUsageReflectsRecordedChatRequests|TestConsoleUsageReflectsRecordedChatRequests"
  "TestConsoleUsageListsCurrentUserRecentRelayRequests|TestConsoleUsageListsCurrentUserRecentRelayRequests"
  "TestSelectOrganizationRequiresMembershipAndUpdatesSessionScope|TestSelectOrganizationRequiresMembershipAndUpdatesSessionScope"
  "TestOrganizationInvitationRevokeRejectsAcceptance|TestOrganizationInvitationRevokeRejectsAcceptance"
  "TestOrganizationSessionSecurityOnMembershipChanges|TestOrganizationSessionSecurityOnMembershipChanges"
  "TestNotificationMutationRoutesEnforceOwnership|TestNotificationMutationRoutesEnforceOwnership"
  "TestGetPreferencesReturnsUserInitializationState|TestGetPreferencesReturnsUserInitializationState"
  "TestUpdatePreferencesPersistsOnboardingState|TestUpdatePreferencesPersistsOnboardingState"
  "TestConversationAndMessageFlow|TestConversationAndMessageFlow"
  "TestConversationConfigFlow|TestConversationConfigFlow"
  "TestTaskApproveRouteRequiresAwaitingConfirmationAndWorkspaceScope|TestTaskApproveRouteRequiresAwaitingConfirmationAndWorkspaceScope"
  "TestRouteSurfaceRequiresSessionForAppRoutes|TestRouteSurfaceRequiresSessionForAppRoutes"
  "TestRouteSurfaceRejectsCookieMutationWithoutCSRF|TestRouteSurfaceRejectsCookieMutationWithoutCSRF"
)
for requirement in "${app_stateful_routes_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$app_stateful_routes_body" "app-stateful-routes" "$token" "$test_name"
done

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
tenant_cross_surface_required=(
  "TestCrossTenantChatScopeUsesActiveOrganization|TestCrossTenantChatScopeUsesActiveOrganization"
  "TestCrossTenantKnowledgeScopeDeniesReadWriteAndAttach|TestCrossTenantKnowledgeScopeDeniesReadWriteAndAttach"
  "TestCrossTenantConsoleUsageUsesActiveOrganization|TestCrossTenantConsoleUsageUsesActiveOrganization"
  "TestCrossTenantAgentScopeDeniesReadWriteAndConversation|TestCrossTenantAgentScopeDeniesReadWriteAndConversation"
  "TestCrossTenantMemoryScopeDeniesReadWrite|TestCrossTenantMemoryScopeDeniesReadWrite"
  "TestCrossTenantMCPScopeDeniesReadWriteAndConnect|TestCrossTenantMCPScopeDeniesReadWriteAndConnect"
  "TestCrossTenantQuotaScopeUsesActiveOrganization|TestCrossTenantQuotaScopeUsesActiveOrganization"
  "TestCrossTenantScheduledTaskScope|TestCrossTenantScheduledTaskScope"
  "TestCrossTenantMarketplacePublisherScopeUsesActiveOrganization|TestCrossTenantMarketplacePublisherScopeUsesActiveOrganization"
  "TestMarketplacePublisherSettlementPreferencesUseActiveOrganization|TestMarketplacePublisherSettlementPreferencesUseActiveOrganization"
  "TestAgentRunStatusEndpointsExposeTenantScopedRunDetail|TestAgentRunStatusEndpointsExposeTenantScopedRunDetail"
  "TestAgentToolRunApprovalRejectRetryEndpointsAreTenantScoped|TestAgentToolRunApprovalRejectRetryEndpointsAreTenantScoped"
)
for requirement in "${tenant_cross_surface_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$tenant_cross_surface_body" "tenant-cross-surface" "$token" "$test_name"
done

secret_response_safety_body=$(sed -n '/^run_secret_response_safety_profile() {/,/^}/p' "$target")
secret_response_safety_required=(
  "TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted|TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted"
  "TestPublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers|TestPublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers"
  "TestAdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers|TestAdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers"
  "TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers|TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers"
  "TestSQLStoreProtectsAuthTokenWithPostgres|TestSQLStoreProtectsAuthTokenWithPostgres"
)
for entry in "${secret_response_safety_required[@]}"; do
  IFS='|' read -r token test_name <<< "$entry"
  require_profile_token "$secret_response_safety_body" "secret-response-safety" "$token" "$test_name"
done

observability_alert_recovery_body=$(sed -n '/^run_observability_alert_recovery_persistence_profile() {/,/^}/p' "$target")
observability_alert_recovery_required=(
  "TestSQLAlertRoutingRuleStorePersistsRoutingRules|TestSQLAlertRoutingRuleStorePersistsRoutingRules"
  "TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation|TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation"
  "TestSQLAlertStateStoreListsAlertStatesWithFilters|TestSQLAlertStateStoreListsAlertStatesWithFilters"
  "TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown|TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown"
  "TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert|TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert"
)
for requirement in "${observability_alert_recovery_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$observability_alert_recovery_body" "observability-alert-recovery-persistence" "$token" "$test_name"
done

scheduled_task_runtime_body=$(sed -n '/^run_scheduled_task_runtime_profile() {/,/^}/p' "$target")
scheduled_task_runtime_required=(
  "TestSQLStoreCreatesAndListsScheduledTasksByOrganization|TestSQLStoreCreatesAndListsScheduledTasksByOrganization"
  "TestSQLStoreSyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks|TestSQLStoreSyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks"
  "TestSQLStoreGetsAndUpdatesScheduledTaskEnabledState|TestSQLStoreGetsAndUpdatesScheduledTaskEnabledState"
  "TestSQLStoreRecordsAndListsScheduledTaskRunsByOrganizationAndTask|TestSQLStoreRecordsAndListsScheduledTaskRunsByOrganizationAndTask"
  "TestSQLStoreClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns|TestSQLStoreClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns"
  "TestSQLStoreCompletesManualScheduledTaskRunWithoutAdvancingNextRun|TestSQLStoreCompletesManualScheduledTaskRunWithoutAdvancingNextRun"
  "TestSQLStoreCompletesScheduledTaskRunAndAdvancesTask|TestSQLStoreCompletesScheduledTaskRunAndAdvancesTask"
  "TestSQLStoreFailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim|TestSQLStoreFailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim"
  "TestScheduledTasksRouteCreatesAndListsTasks|TestScheduledTasksRouteCreatesAndListsTasks"
  "TestScheduledTasksRouteListsRunsForTaskWithinSessionOrganization|TestScheduledTasksRouteListsRunsForTaskWithinSessionOrganization"
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
  "TestRegisterLoginMeLogoutFlow|TestRegisterLoginMeLogoutFlow"
  "TestAuthRateLimitRejectsRepeatedFailedLogin|TestAuthRateLimitRejectsRepeatedFailedLogin"
  "TestPasswordResetRoutesConfirmAndRevokeSessions|TestPasswordResetRoutesConfirmAndRevokeSessions"
  "TestPasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv|TestPasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv"
  "TestRegisterStoresHashedPassword|TestRegisterStoresHashedPassword"
  "TestLoginAcceptsRawPasswordAgainstStoredHash|TestLoginAcceptsRawPasswordAgainstStoredHash"
  "TestMeRequiresSession|TestMeRequiresSession"
  "TestAuthResponsesExposeStableUserAndPreferenceContracts|TestAuthResponsesExposeStableUserAndPreferenceContracts"
  "TestSensitiveOrganizationActionsAreRateLimited|TestSensitiveOrganizationActionsAreRateLimited"
)
for requirement in "${auth_security_persistence_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$auth_security_persistence_body" "auth-security-persistence" "$token" "$test_name"
done
require_profile_text "$auth_security_persistence_body" "auth-security-persistence" 'auth_http_pattern="^(TestRegisterLoginMeLogoutFlow|TestAuthRateLimitRejectsRepeatedFailedLogin|TestPasswordResetRoutesConfirmAndRevokeSessions|TestPasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv|TestRegisterStoresHashedPassword|TestLoginAcceptsRawPasswordAgainstStoredHash|TestMeRequiresSession|TestAuthResponsesExposeStableUserAndPreferenceContracts|TestSensitiveOrganizationActionsAreRateLimited)$"' "auth HTTP test pattern"

migration_ledger_backfills_body=$(sed -n '/^run_migration_ledger_backfills_profile() {/,/^}/p' "$target")
migration_ledger_backfills_required=(
  "TestApplyMigrationsRecordsLedgerAndSkipsAppliedFiles|TestApplyMigrationsRecordsLedgerAndSkipsAppliedFiles"
  "TestApplyMigrationsRejectsChecksumMismatch|TestApplyMigrationsRejectsChecksumMismatch"
  "TestApplyMigrationsBackfillsLegacyTenantScopeData|TestApplyMigrationsBackfillsLegacyTenantScopeData"
  "TestApplyMigrationsBackfillsMarketplaceCategoryIDs|TestApplyMigrationsBackfillsMarketplaceCategoryIDs"
)
for requirement in "${migration_ledger_backfills_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$migration_ledger_backfills_body" "migration-ledger-backfills" "$token" "$test_name"
done

relay_file_mapping_body=$(sed -n '/^run_relay_file_mapping_tenant_ownership_profile() {/,/^}/p' "$target")
relay_file_mapping_required=(
  "TestRelayStoreSaveFileMappingPersistsTenantOwnership|TestRelayStoreSaveFileMappingPersistsTenantOwnership"
  "TestRelayStoreGetFileMappingRequiresTenantOwnership|TestRelayStoreGetFileMappingRequiresTenantOwnership"
  "TestRelayStoreListFileMappingsRequiresTenantOwnership|TestRelayStoreListFileMappingsRequiresTenantOwnership"
  "TestNewRelayFilesSQLRelayStoreUploadGetTenantFailClosed|TestNewRelayFilesSQLRelayStoreUploadGetTenantFailClosed"
)
for requirement in "${relay_file_mapping_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$relay_file_mapping_body" "relay-file-mapping-tenant-ownership" "$token" "$test_name"
done

publishing_channel_isolation_body=$(sed -n '/^run_publishing_channel_isolation_profile() {/,/^}/p' "$target")
require_profile_token "$publishing_channel_isolation_body" "publishing-channel-isolation" "TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation" "TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation"

admin_relay_channel_isolation_body=$(sed -n '/^run_admin_relay_channel_isolation_profile() {/,/^}/p' "$target")
require_profile_token "$admin_relay_channel_isolation_body" "admin-relay-channel-isolation" "TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation" "TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation"

admin_relay_read_isolation_body=$(sed -n '/^run_admin_relay_read_isolation_profile() {/,/^}/p' "$target")
require_profile_token "$admin_relay_read_isolation_body" "admin-relay-read-isolation" "TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization" "TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization"

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
marketplace_recommendation_search_required=(
  "TestSearchAgentsRecommendedRanksContentMatchesOverGenericHotAgents|TestSearchAgentsRecommendedRanksContentMatchesOverGenericHotAgents"
  "TestSearchAgentsRecommendedFallbackExplorationIsDeterministicAndNonEmpty|TestSearchAgentsRecommendedFallbackExplorationIsDeterministicAndNonEmpty"
  "TestSearchAgentsRecommendedUsesRankingSignals|TestSearchAgentsRecommendedUsesRankingSignals"
  "TestSearchAgentsRecommendedUsesCollaborativeFilteringForRequester|TestSearchAgentsRecommendedUsesCollaborativeFilteringForRequester"
  "TestSearchAgentsRecommendedDemotesGovernanceWeightedAgents|TestSearchAgentsRecommendedDemotesGovernanceWeightedAgents"
)
for requirement in "${marketplace_recommendation_search_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$marketplace_recommendation_search_body" "marketplace-recommendation-search" "$token" "$test_name"
done

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
relay_runtime_channel_isolation_required=(
  "TestLoadBalancerSelectModelForOrganizationFiltersModelRouteAndFallback|TestLoadBalancerSelectModelForOrganizationFiltersModelRouteAndFallback"
  "TestRouterRouteWithBillingUsesTrustedOrganizationForChannelSelectionAndAffinity|TestRouterRouteWithBillingUsesTrustedOrganizationForChannelSelectionAndAffinity"
  "TestRelayStoreConversationAffinityPersistsAndUpdatesChannel|TestRelayStoreConversationAffinityPersistsAndUpdatesChannel"
  "TestRelayStoreLoadPoolPreservesChannelOrganizationScope|TestRelayStoreLoadPoolPreservesChannelOrganizationScope"
  "TestRelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey|TestRelayStoreProtectsChannelAPIKeyAtRestAndHydratesRuntimeKey"
  "TestModelsHandlerScopesModelsToTrustedOrganization|TestModelsHandlerScopesModelsToTrustedOrganization"
)
for requirement in "${relay_runtime_channel_isolation_required[@]}"; do
  token="${requirement%%|*}"
  test_name="${requirement#*|}"
  require_profile_token "$relay_runtime_channel_isolation_body" "relay-runtime-channel-isolation" "$token" "$test_name"
done

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
