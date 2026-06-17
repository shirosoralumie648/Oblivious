#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
target="$repo_root/scripts/verify-commercial-db-evidence.sh"

fail() {
  echo "[commercial-db-evidence-profiles] $*" >&2
  exit 1
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
if [[ "$relay_runtime_channel_isolation_body" != *"ConversationAffinityPersistsAndUpdatesChannel"* ]]; then
  fail "relay-runtime-channel-isolation must include TestRelayStoreConversationAffinityPersistsAndUpdatesChannel"
fi

echo "[commercial-db-evidence-profiles] commercial DB evidence profile list is synchronized."
