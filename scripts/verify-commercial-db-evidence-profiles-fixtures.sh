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
