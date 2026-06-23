#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

tmp_dir=$(mktemp -d)
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

target_fixture="$tmp_dir/verify-commercial-db-evidence.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_fixture"
perl -0pi -e 's/TestQuotaObservabilityRecordsSettlementFailure/TestQuotaObservabilityRecordsSettlementFailureV2/g' "$target_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected mutated quota observability token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"quota-sql-isolation must include TestQuotaObservabilityRecordsSettlementFailure"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected quota observability rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_secret_fixture="$tmp_dir/verify-commercial-db-evidence-secret-response.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_secret_fixture"
perl -0pi -e 's/ObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted/ObservabilityAlertAdminRouteSQLProviderSecretsAreRedactedV2/g' "$target_secret_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_secret_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed secret-response token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"secret-response-safety must include TestObservabilityAlertAdminRouteSQLProviderSecretsAreRedacted"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected secret-response token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_marketplace_template_fixture="$tmp_dir/verify-commercial-db-evidence-marketplace-template.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_marketplace_template_fixture"
perl -0pi -e 's/MarketplaceTemplateRoutesCreateListDetailAndInstall/MarketplaceTemplateRoutesCreateListDetailAndInstallV2/g' "$target_marketplace_template_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_marketplace_template_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed marketplace-template token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"marketplace-template-routes must include TestMarketplaceTemplateRoutesCreateListDetailAndInstall"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected marketplace-template token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_billing_checkout_fixture="$tmp_dir/verify-commercial-db-evidence-billing-checkout.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_billing_checkout_fixture"
perl -0pi -e 's/TestBillingCheckoutRequiresSession/TestBillingCheckoutRequiresSessionV2/g' "$target_billing_checkout_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_billing_checkout_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed billing checkout token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"billing-checkout-topup-http must include TestBillingCheckoutRequiresSession"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected billing checkout token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_app_stateful_fixture="$tmp_dir/verify-commercial-db-evidence-app-stateful.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_app_stateful_fixture"
perl -0pi -e 's/TestConsoleAPITokenCreateListAndRevoke/TestLegacyConsoleAPITokenCreateListAndRevoke/g' "$target_app_stateful_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_app_stateful_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected prefixed app-stateful token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"app-stateful-routes must include TestConsoleAPITokenCreateListAndRevoke"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected app-stateful full-name rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_tenant_cross_surface_fixture="$tmp_dir/verify-commercial-db-evidence-tenant-cross-surface.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_tenant_cross_surface_fixture"
perl -0pi -e 's/TestCrossTenantChatScopeUsesActiveOrganization/TestLegacyCrossTenantChatScopeUsesActiveOrganization/g' "$target_tenant_cross_surface_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_tenant_cross_surface_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected prefixed tenant cross-surface token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"tenant-cross-surface must include TestCrossTenantChatScopeUsesActiveOrganization"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected tenant cross-surface full-name rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_marketplace_recommendation_fixture="$tmp_dir/verify-commercial-db-evidence-marketplace-recommendation.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_marketplace_recommendation_fixture"
perl -0pi -e 's/TestSearchAgentsRecommendedRanksContentMatchesOverGenericHotAgents/TestSearchAgentsRecommendedV2RanksContentMatchesOverGenericHotAgents/g' "$target_marketplace_recommendation_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_marketplace_recommendation_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected prefixed marketplace recommendation token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"marketplace-recommendation-search must include TestSearchAgentsRecommendedRanksContentMatchesOverGenericHotAgents"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected marketplace recommendation full-name rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_scheduled_task_fixture="$tmp_dir/verify-commercial-db-evidence-scheduled-task.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_scheduled_task_fixture"
perl -0pi -e 's/TestSQLStoreCreatesAndListsScheduledTasksByOrganization/TestSQLStoreV2CreatesAndListsScheduledTasksByOrganization/g' "$target_scheduled_task_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_scheduled_task_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected prefixed scheduled-task token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"scheduled-task-runtime must include TestSQLStoreCreatesAndListsScheduledTasksByOrganization"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected scheduled-task full-name rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_auth_security_fixture="$tmp_dir/verify-commercial-db-evidence-auth-security.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_auth_security_fixture"
perl -0pi -e 's/MeRequiresSession/MeRequiresSessionV2/g' "$target_auth_security_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_auth_security_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed auth security token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"auth-security-persistence must include TestMeRequiresSession"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected auth security token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_marketplace_money_fixture="$tmp_dir/verify-commercial-db-evidence-marketplace-money.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_marketplace_money_fixture"
perl -0pi -e 's/TestDomesticPaymentWebhookRouteAppliesMarketplaceInstallSettlementOnce/TestDomesticPaymentWebhookRouteAppliesMarketplaceV2InstallSettlementOnce/g' "$target_marketplace_money_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_marketplace_money_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected marketplace money-movement grouped prefix to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"marketplace-money-movement must include TestDomesticPaymentWebhookRouteAppliesMarketplaceInstallSettlementOnce"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected marketplace money-movement full-name rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_quota_fixture="$tmp_dir/verify-commercial-db-evidence-quota.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_quota_fixture"
perl -0pi -e 's/UsageLimitSettingsRoundTrip/UsageLimitSettingsRoundTripV2/g' "$target_quota_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_quota_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed quota token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"quota-sql-isolation must include TestSQLStoreUsageLimitSettingsRoundTrip"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected quota full-name rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_workflow_fixture="$tmp_dir/verify-commercial-db-evidence-workflow.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_workflow_fixture"
perl -0pi -e 's/TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution/TestCrossTenantWorkflowScopeDeniesReadWriteAndExecutionSuffix/g' "$target_workflow_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_workflow_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed workflow token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"workflow-sql-isolation must include TestCrossTenantWorkflowScopeDeniesReadWriteAndExecution"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected workflow token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_migration_fixture="$tmp_dir/verify-commercial-db-evidence-migration.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_migration_fixture"
perl -0pi -e 's/TestApplyMigrationsRecordsLedgerAndSkipsAppliedFiles/TestApplyMigrationsRecordsLedgerAndSkipsAppliedFilesV2/g' "$target_migration_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_migration_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed migration ledger token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"migration-ledger-backfills must include TestApplyMigrationsRecordsLedgerAndSkipsAppliedFiles"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected migration ledger token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_relay_file_mapping_fixture="$tmp_dir/verify-commercial-db-evidence-relay-file-mapping.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_relay_file_mapping_fixture"
perl -0pi -e 's/TestRelayStoreSaveFileMappingPersistsTenantOwnership/TestRelayStoreSaveFileMappingPersistsTenantOwnershipV2/g' "$target_relay_file_mapping_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_relay_file_mapping_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed relay file-mapping token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"relay-file-mapping-tenant-ownership must include TestRelayStoreSaveFileMappingPersistsTenantOwnership"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected relay file-mapping token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_publishing_channel_fixture="$tmp_dir/verify-commercial-db-evidence-publishing-channel.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_publishing_channel_fixture"
perl -0pi -e 's/TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation/TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolationV2/g' "$target_publishing_channel_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_publishing_channel_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed publishing-channel token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"publishing-channel-isolation must include TestPublishingChannelHTTPRouteEnforcesActiveOrganizationIsolation"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected publishing-channel token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_admin_relay_channel_fixture="$tmp_dir/verify-commercial-db-evidence-admin-relay-channel.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_admin_relay_channel_fixture"
perl -0pi -e 's/TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation/TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolationV2/g' "$target_admin_relay_channel_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_admin_relay_channel_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed admin-relay-channel token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"admin-relay-channel-isolation must include TestAdminRelayChannelHTTPRouteEnforcesActiveOrganizationIsolation"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected admin-relay-channel token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_admin_relay_read_fixture="$tmp_dir/verify-commercial-db-evidence-admin-relay-read.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_admin_relay_read_fixture"
perl -0pi -e 's/TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization/TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganizationV2/g' "$target_admin_relay_read_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_admin_relay_read_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected suffixed admin-relay-read token to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"admin-relay-read-isolation must include TestAdminRelayReadSurfacesScopeRuntimeStatsAndModelInventoryToActiveOrganization"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected admin-relay-read token-boundary rejection message" >&2
  echo "$output" >&2
  exit 1
fi

target_help_fixture="$tmp_dir/verify-commercial-db-evidence-help-only.sh"
cp "$repo_root/scripts/verify-commercial-db-evidence.sh" "$target_help_fixture"
perl -0pi -e 's/(  core-sql-persistence         Run focused Chat SQL sharing\/forking,\n                               Publishing channel SQL retry\/archive, and Relay\n                               semantic-cache SQL persistence tests\.\n)/$1  stale-help-only-profile    Run stale help-only profile tests.\n/s' "$target_help_fixture"

if output=$(COMMERCIAL_DB_EVIDENCE_TARGET="$target_help_fixture" bash "$repo_root/scripts/verify-commercial-db-evidence-profiles.sh" 2>&1); then
  echo "[commercial-db-evidence-profiles-fixtures] expected stale help-only profile to be rejected" >&2
  echo "$output" >&2
  exit 1
fi

if [[ "$output" != *"help section includes unknown profile stale-help-only-profile"* ]]; then
  echo "[commercial-db-evidence-profiles-fixtures] expected help-only rejection message" >&2
  echo "$output" >&2
  exit 1
fi

echo "[commercial-db-evidence-profiles-fixtures] commercial DB evidence profile fixture behavior is guarded."
