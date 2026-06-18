#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
target="$repo_root/scripts/verify-commercial-db-evidence.sh"

fail() {
  echo "[commercial-db-evidence-profiles] $*" >&2
  exit 1
}

profile_body_has_token() {
  local body="$1"
  local token="$2"

  [[ "$body" =~ (^|[^[:alnum:]_])${token}([^[:alnum:]_]|$) ]]
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

[[ "${usage_profiles[all]:-}" == "1" ]] || fail "usage profile list must include all"
[[ "${case_profiles[all]:-}" == "1" ]] || fail "case statement must include all"

for profile in "${required_profiles[@]}"; do
  [[ "${usage_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from usage list: $profile"
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from case statement: $profile"
  [[ "${all_profiles[$profile]:-}" == "1" ]] || fail "required profile is missing from run_all_profiles: $profile"
  grep -Eq "^  ${profile}([[:space:]]|$)" "$target" || fail "required profile is missing from help section: $profile"
done

for profile in "${!case_profiles[@]}"; do
  [[ "$profile" == "all" ]] && continue
  [[ "${usage_profiles[$profile]:-}" == "1" ]] || fail "usage profile list is missing $profile"
  [[ "${all_profiles[$profile]:-}" == "1" ]] || fail "run_all_profiles is missing $profile"
  grep -Eq "^  ${profile}([[:space:]]|$)" "$target" || fail "Profiles help section is missing $profile"
done

for profile in "${!usage_profiles[@]}"; do
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "usage profile list includes unknown profile $profile"
done

for profile in "${!all_profiles[@]}"; do
  [[ "${case_profiles[$profile]:-}" == "1" ]] || fail "run_all_profiles includes unknown profile $profile"
done

billing_checkout_topup_body=$(sed -n '/^run_billing_checkout_topup_http_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$billing_checkout_topup_body" "PersistsTenantPaymentIntent"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutPersistsTenantPaymentIntent"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "ExplicitStripeUsesExistingCheckout"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutExplicitStripeUsesExistingCheckout"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "UnconfiguredProvidersDoNotCreateArtifacts"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutUnconfiguredProvidersDoNotCreateArtifacts"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "CreatorFailureMarksTopupFailed"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutCreatorFailureMarksTopupFailed"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "UsesConfiguredProviderCheckoutCreator"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutUsesConfiguredProviderCheckoutCreator"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "UsesConfiguredDomesticProviderFromRouterConfig"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutUsesConfiguredDomesticProviderFromRouterConfig"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "TopupDoesNotCreditQuotaBeforeWebhook"; then
  fail "billing-checkout-topup-http must include TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "QuotaTopupEndpointNoLongerCreditsWithoutPayment"; then
  fail "billing-checkout-topup-http must include TestQuotaTopupEndpointNoLongerCreditsWithoutPayment"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "DomesticPaymentWebhookRoutesVerifySignatureAndRecordEvents"; then
  fail "billing-checkout-topup-http must include TestDomesticPaymentWebhookRoutesVerifySignatureAndRecordEvents"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "LifecycleOnce"; then
  fail "billing-checkout-topup-http must include TestDomesticPaymentWebhookRouteAppliesTopupLifecycleOnce"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "RefundOnce"; then
  fail "billing-checkout-topup-http must include TestDomesticPaymentWebhookRouteAppliesTopupRefundOnce"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "SubscriptionLifecycleOnce"; then
  fail "billing-checkout-topup-http must include DomesticPaymentWebhookRouteAppliesSubscriptionLifecycleOnce"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "RejectsInvalidSignature"; then
  fail "billing-checkout-topup-http must include TestStripeWebhookRouteRejectsInvalidSignature"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "RecordsSignedEventOnce"; then
  fail "billing-checkout-topup-http must include TestStripeWebhookRouteRecordsSignedEventOnce"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "AppliesCheckoutCompletedSubscriptionOnce"; then
  fail "billing-checkout-topup-http must include TestStripeWebhookRouteAppliesCheckoutCompletedSubscriptionOnce"
fi
if ! profile_body_has_token "$billing_checkout_topup_body" "RetriesLifecycleForRecordedDuplicateEvent"; then
  fail "billing-checkout-topup-http must include TestStripeWebhookRouteRetriesLifecycleForRecordedDuplicateEvent"
fi

billing_provider_lifecycle_body=$(sed -n '/^run_billing_provider_lifecycle_profile() {/,/^}/p' "$target")
if ! profile_body_has_token "$billing_provider_lifecycle_body" "TestApplyRefundUpdatesTopupOrderStatusAndRefundedAmount"; then
  fail "billing-provider-lifecycle must include TestApplyRefundUpdatesTopupOrderStatusAndRefundedAmount"
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
if [[ "$secret_response_safety_body" != *"ObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted"* ]]; then
  fail "secret-response-safety must include TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted"
fi
if [[ "$secret_response_safety_body" != *"PublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers"* ]]; then
  fail "secret-response-safety must include TestPublishingChannelHTTPRouteRedactsSQLStoreConfigSecretsAndPreservesMarkers"
fi
if [[ "$secret_response_safety_body" != *"AdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers"* ]]; then
  fail "secret-response-safety must include TestAdminRelayChannelHTTPRouteRedactsSQLStoreAPIKeysAndPreservesMarkers"
fi
if [[ "$secret_response_safety_body" != *"WorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers"* ]]; then
  fail "secret-response-safety must include TestWorkflowHTTPRouteRedactsSQLStoreSecretsAndPreservesMarkers"
fi
if [[ "$secret_response_safety_body" != *"SQLStoreProtectsAuthTokenWithPostgres"* ]]; then
  fail "secret-response-safety must include TestSQLStoreProtectsAuthTokenWithPostgres"
fi

observability_alert_recovery_body=$(sed -n '/^run_observability_alert_recovery_persistence_profile() {/,/^}/p' "$target")
if [[ "$observability_alert_recovery_body" != *"RoutingRuleStorePersistsRoutingRules"* ]]; then
  fail "observability-alert-recovery-persistence must include TestSQLAlertRoutingRuleStorePersistsRoutingRules"
fi
if [[ "$observability_alert_recovery_body" != *"PersistsAlertLifecycleAndEscalation"* ]]; then
  fail "observability-alert-recovery-persistence must include TestSQLAlertStateStorePersistsAlertLifecycleAndEscalation"
fi
if [[ "$observability_alert_recovery_body" != *"ListsAlertStatesWithFilters"* ]]; then
  fail "observability-alert-recovery-persistence must include TestSQLAlertStateStoreListsAlertStatesWithFilters"
fi
if [[ "$observability_alert_recovery_body" != *"PersistsNotificationThrottleAndRecoveryCooldown"* ]]; then
  fail "observability-alert-recovery-persistence must include TestSQLAlertStateStorePersistsNotificationThrottleAndRecoveryCooldown"
fi
if [[ "$observability_alert_recovery_body" != *"RecordsRepeatedDeliveryBatchesForSameAlert"* ]]; then
  fail "observability-alert-recovery-persistence must include TestSQLAlertStateStoreRecordsRepeatedDeliveryBatchesForSameAlert"
fi

scheduled_task_runtime_body=$(sed -n '/^run_scheduled_task_runtime_profile() {/,/^}/p' "$target")
if [[ "$scheduled_task_runtime_body" != *"ClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns"* ]]; then
  fail "scheduled-task-runtime must include TestSQLStoreClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns"
fi
if [[ "$scheduled_task_runtime_body" != *"FailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim"* ]]; then
  fail "scheduled-task-runtime must include TestSQLStoreFailsScheduledTaskRunAndAdvancesTaskToAvoidImmediateReclaim"
fi
if [[ "$scheduled_task_runtime_body" != *"ListsRunsForTaskWithinSessionOrganization"* ]]; then
  fail "scheduled-task-runtime must include TestScheduledTasksRouteListsRunsForTaskWithinSessionOrganization"
fi
if [[ "$scheduled_task_runtime_body" != *"DefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks"* ]]; then
  fail "scheduled-task-runtime must include TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks"
fi

quota_sql_isolation_body=$(sed -n '/^run_quota_sql_isolation_profile() {/,/^}/p' "$target")
if [[ "$quota_sql_isolation_body" != *"UsageLimitSettingsRoundTrip"* ]]; then
  fail "quota-sql-isolation must include TestSQLStoreUsageLimitSettingsRoundTrip"
fi
if [[ "$quota_sql_isolation_body" != *"UserQuotaModeUsesUserScopedBalance"* ]]; then
  fail "quota-sql-isolation must include TestSQLStoreUserQuotaModeUsesUserScopedBalance"
fi
if [[ "$quota_sql_isolation_body" != *"BillingSessionsAreOrganizationScoped"* ]]; then
  fail "quota-sql-isolation must include TestSQLStoreBillingSessionsAreOrganizationScoped"
fi
if [[ "$quota_sql_isolation_body" != *"TopupOrderMutationsRequireOrganizationScope"* ]]; then
  fail "quota-sql-isolation must include TestSQLStoreTopupOrderMutationsRequireOrganizationScope"
fi
if [[ "$quota_sql_isolation_body" != *"ResolveUsageLimitFallsBackToActiveSubscriptionRequestCap"* ]]; then
  fail "quota-sql-isolation must include TestSQLStoreResolveUsageLimitFallsBackToActiveSubscriptionRequestCap"
fi
if [[ "$quota_sql_isolation_body" != *"ListPackagesReturnsOnlyActivePublicHybridPlans"* ]]; then
  fail "quota-sql-isolation must include TestSQLStoreListPackagesReturnsOnlyActivePublicHybridPlans"
fi
if [[ "$quota_sql_isolation_body" != *"CrossTenantQuotaScopeUsesActiveOrganization"* ]]; then
  fail "quota-sql-isolation must include TestCrossTenantQuotaScopeUsesActiveOrganization"
fi
if [[ "$quota_sql_isolation_body" != *"AdminUsageLimitSettingsRoutePersistsWithPostgres"* ]]; then
  fail "quota-sql-isolation must include TestAdminUsageLimitSettingsRoutePersistsWithPostgres"
fi
if [[ "$quota_sql_isolation_body" != *"AdminUserQuotaRoutePersistsWithPostgres"* ]]; then
  fail "quota-sql-isolation must include TestAdminUserQuotaRoutePersistsWithPostgres"
fi
if [[ "$quota_sql_isolation_body" != *"BillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook"* ]]; then
  fail "quota-sql-isolation must include TestBillingCheckoutTopupDoesNotCreditQuotaBeforeWebhook"
fi
if [[ "$quota_sql_isolation_body" != *"QuotaTopupEndpointNoLongerCreditsWithoutPayment"* ]]; then
  fail "quota-sql-isolation must include TestQuotaTopupEndpointNoLongerCreditsWithoutPayment"
fi
if [[ "$quota_sql_isolation_body" != *"AdminBillingRecordsTopupRefundAndAdjustsQuota"* ]]; then
  fail "quota-sql-isolation must include TestAdminBillingRecordsTopupRefundAndAdjustsQuota"
fi
if [[ "$quota_sql_isolation_body" != *"CheckoutSessionCompletedFulfillsTopupOnce"* ]]; then
  fail "quota-sql-isolation must include TestLifecycleApplyCheckoutSessionCompletedFulfillsTopupOnce"
fi
if [[ "$quota_sql_isolation_body" != *"RefundRecordsRefundAndAdjustsTopup"* ]]; then
  fail "quota-sql-isolation must include TestLifecycleApplyRefundRecordsRefundAndAdjustsTopup"
fi

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
if [[ "$auth_security_persistence_body" != *"PasswordPolicyResetAndSessionRevocation"* ]]; then
  fail "auth-security-persistence must include TestPasswordPolicyResetAndSessionRevocation"
fi
if [[ "$auth_security_persistence_body" != *"PasswordResetTokenReplayExpiryAndUnknownEmailFailClosed"* ]]; then
  fail "auth-security-persistence must include TestPasswordResetTokenReplayExpiryAndUnknownEmailFailClosed"
fi
if [[ "$auth_security_persistence_body" != *"SQLRateLimiterPersistsBlocks"* ]]; then
  fail "auth-security-persistence must include TestSQLRateLimiterPersistsBlocks"
fi
if [[ "$auth_security_persistence_body" != *"RegisterLoginMeLogoutFlow"* ]]; then
  fail "auth-security-persistence must include TestRegisterLoginMeLogoutFlow"
fi
if [[ "$auth_security_persistence_body" != *"AuthRateLimitRejectsRepeatedFailedLogin"* ]]; then
  fail "auth-security-persistence must include TestAuthRateLimitRejectsRepeatedFailedLogin"
fi
if [[ "$auth_security_persistence_body" != *"PasswordResetRoutesConfirmAndRevokeSessions"* ]]; then
  fail "auth-security-persistence must include TestPasswordResetRoutesConfirmAndRevokeSessions"
fi
if [[ "$auth_security_persistence_body" != *"PasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv"* ]]; then
  fail "auth-security-persistence must include TestPasswordResetRequestDoesNotEnumerateEmailsOutsideTestEnv"
fi
if [[ "$auth_security_persistence_body" != *"RegisterStoresHashedPassword"* ]]; then
  fail "auth-security-persistence must include TestRegisterStoresHashedPassword"
fi
if [[ "$auth_security_persistence_body" != *"LoginAcceptsRawPasswordAgainstStoredHash"* ]]; then
  fail "auth-security-persistence must include TestLoginAcceptsRawPasswordAgainstStoredHash"
fi
if [[ "$auth_security_persistence_body" != *"MeRequiresSession"* ]]; then
  fail "auth-security-persistence must include TestMeRequiresSession"
fi
if [[ "$auth_security_persistence_body" != *"AuthResponsesExposeStableUserAndPreferenceContracts"* ]]; then
  fail "auth-security-persistence must include TestAuthResponsesExposeStableUserAndPreferenceContracts"
fi
if [[ "$auth_security_persistence_body" != *"SensitiveOrganizationActionsAreRateLimited"* ]]; then
  fail "auth-security-persistence must include TestSensitiveOrganizationActionsAreRateLimited"
fi

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
if [[ "$marketplace_template_routes_body" != *"MarketplaceTemplateRoutesCreateListDetailAndInstall"* ]]; then
  fail "marketplace-template-routes must include TestMarketplaceTemplateRoutesCreateListDetailAndInstall"
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
if [[ "$marketplace_money_movement_body" != *"StripeWebhookRouteAppliesMarketplaceInstallSettlementOnce"* ]]; then
  fail "marketplace-money-movement must include TestStripeWebhookRouteAppliesMarketplaceInstallSettlementOnce"
fi
if [[ "$marketplace_money_movement_body" != *"StripeRefundUpdatesMarketplaceSettlementOnce"* ]]; then
  fail "marketplace-money-movement must include TestStripeRefundUpdatesMarketplaceSettlementOnce"
fi
if [[ "$marketplace_money_movement_body" != *"AppliesSegmentedPlatformFees"* ]]; then
  fail "marketplace-money-movement must include TestSettlementAppliesSegmentedPlatformFees"
fi
if [[ "$marketplace_money_movement_body" != *"AppliesSpecSegmentedRevenueTiers"* ]]; then
  fail "marketplace-money-movement must include TestSettlementAppliesSpecSegmentedRevenueTiers"
fi
if [[ "$marketplace_money_movement_body" != *"MinimumSettlementBlocksSmallPayoutUntilCycleElapsed"* ]]; then
  fail "marketplace-money-movement must include TestSettlementMinimumSettlementBlocksSmallPayoutUntilCycleElapsed"
fi
if [[ "$marketplace_money_movement_body" != *"ApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce"* ]]; then
  fail "marketplace-money-movement must include TestSettlementApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce"
fi
if [[ "$marketplace_money_movement_body" != *"ApplyRefundAdjustsOrderAndSettlementOnce"* ]]; then
  fail "marketplace-money-movement must include TestSettlementApplyRefundAdjustsOrderAndSettlementOnce"
fi
if [[ "$marketplace_money_movement_body" != *"RevenueTierDisclosureUsesSegmentedFees"* ]]; then
  fail "marketplace-money-movement must include TestMarketplaceRevenueTierDisclosureUsesSegmentedFees"
fi
if [[ "$marketplace_money_movement_body" != *"PayoutStateIsLocalOnly"* ]]; then
  fail "marketplace-money-movement must include TestSettlementPayoutStateIsLocalOnly"
fi
if [[ "$marketplace_money_movement_body" != *"LifecycleTransitionKeyUsesSelectedProvider"* ]]; then
  fail "marketplace-money-movement must include TestMarketplaceLifecycleTransitionKeyUsesSelectedProvider"
fi
if [[ "$marketplace_money_movement_body" != *"PaymentIntentKindMigrationAllowsMarketplaceInstall"* ]]; then
  fail "marketplace-money-movement must include TestPaymentIntentKindMigrationAllowsMarketplaceInstall"
fi
if [[ "$marketplace_money_movement_body" != *"TopupSummaryQueryUsesPaymentIntentProviderFilter"* ]]; then
  fail "marketplace-money-movement must include TestTopupSummaryQueryUsesPaymentIntentProviderFilter"
fi
if [[ "$marketplace_money_movement_body" != *"MarketplaceSettlementQueriesUsePaymentIntentProviderFilter"* ]]; then
  fail "marketplace-money-movement must include TestMarketplaceSettlementQueriesUsePaymentIntentProviderFilter"
fi
if [[ "$marketplace_money_movement_body" != *"RecordTopupRefundUpdatesOrderStatusAndRefundedAmount"* ]]; then
  fail "marketplace-money-movement must include TestRecordTopupRefundUpdatesOrderStatusAndRefundedAmount"
fi
if [[ "$marketplace_money_movement_body" != *"RecordTopupRefundHandlerPassesDomesticPaymentIntentEvidence"* ]]; then
  fail "marketplace-money-movement must include TestAdminBillingRecordTopupRefundHandlerPassesDomesticPaymentIntentEvidence"
fi

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
if [[ "$workflow_sql_isolation_body" != *"TestWorkflowStorePersists(DefinitionsAndExecutions|VersionHistoryAndExecutionVersion)"* ]]; then
  fail "workflow-sql-isolation must include TestWorkflowStorePersistsDefinitionsAndExecutions and TestWorkflowStorePersistsVersionHistoryAndExecutionVersion"
fi
if [[ "$workflow_sql_isolation_body" != *"TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution"* ]]; then
  fail "workflow-sql-isolation must include TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution"
fi

echo "[commercial-db-evidence-profiles] commercial DB evidence profile list is synchronized."
