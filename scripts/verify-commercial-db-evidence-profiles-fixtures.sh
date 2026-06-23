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

echo "[commercial-db-evidence-profiles-fixtures] commercial DB evidence profile fixture behavior is guarded."
