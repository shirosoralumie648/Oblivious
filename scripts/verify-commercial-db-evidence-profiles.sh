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
if [[ "$billing_checkout_topup_body" != *"SubscriptionLifecycleOnce"* ]]; then
  fail "billing-checkout-topup-http must include DomesticPaymentWebhookRouteAppliesSubscriptionLifecycleOnce"
fi

billing_provider_lifecycle_body=$(sed -n '/^run_billing_provider_lifecycle_profile() {/,/^}/p' "$target")
if [[ "$billing_provider_lifecycle_body" != *"ApplyRefundUpdatesTopupOrderStatusAndRefundedAmount"* ]]; then
  fail "billing-provider-lifecycle must include TestApplyRefundUpdatesTopupOrderStatusAndRefundedAmount"
fi
if [[ "$billing_provider_lifecycle_body" != *"AppliesDomesticRefundThroughRefundLifecycle"* ]]; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticRefundThroughRefundLifecycle"
fi
if [[ "$billing_provider_lifecycle_body" != *"AppliesDomesticSubscriptionDeletedThroughSubscriptionLifecycle"* ]]; then
  fail "billing-provider-lifecycle must include TestLifecycleAppliesDomesticSubscriptionDeletedThroughSubscriptionLifecycle"
fi

app_stateful_routes_body=$(sed -n '/^run_app_stateful_routes_profile() {/,/^}/p' "$target")
if [[ "$app_stateful_routes_body" != *"ConsoleUsageReflectsRecordedChatRequests"* ]]; then
  fail "app-stateful-routes must include TestConsoleUsageReflectsRecordedChatRequests"
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
if [[ "$agent_runtime_memory_body" != *"AgentToolRunStorePersistsToolLifecycle"* ]]; then
  fail "agent-runtime-memory must include TestAgentToolRunStorePersistsToolLifecycle"
fi
if [[ "$agent_runtime_memory_body" != *"AgentToolRunStorePersistsRiskLevel"* ]]; then
  fail "agent-runtime-memory must include TestAgentToolRunStorePersistsRiskLevel"
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
