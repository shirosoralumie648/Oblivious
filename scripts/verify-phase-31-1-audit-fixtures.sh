#!/usr/bin/env bash
set -euo pipefail

root=$(git rev-parse --show-toplevel)
phase_rel=.planning/phases/31.1-readiness-fail-closed
source_phase="$root/$phase_rel"
validator="$root/scripts/verify-phase-31-1-audit.py"
tmp=$(mktemp -d /var/tmp/oblivious-phase-31-1-audit-fixtures.XXXXXX)
trap 'rm -rf "$tmp"' EXIT

baseline="$tmp/baseline"
mkdir -p "$baseline"
for plan in $(seq 11 22); do
  cp "$source_phase/31.1-$plan-PLAN.md" "$baseline/"
done
cp "$source_phase/31.1-SECURITY.md" "$baseline/"
cp "$source_phase/31.1-VALIDATION.md" "$baseline/"

audited_head=${EXPECTED_IMPLEMENTATION_HEAD:-$(sed -n 's/^audited_implementation_head: //p' "$source_phase/31.1-SECURITY.md" | head -1)}
test "${#audited_head}" -eq 40

run_validator() {
  local fixture_dir=$1
  python "$validator" \
    --phase-dir "$fixture_dir" \
    --expected-implementation-head "$audited_head" \
    --security "$fixture_dir/31.1-SECURITY.md" \
    --validation "$fixture_dir/31.1-VALIDATION.md"
}

replace_first() {
  local file=$1
  local search=$2
  local replacement=$3
  SEARCH=$search REPLACEMENT=$replacement perl -0pi -e '
    my $at = index($_, $ENV{SEARCH});
    die "fixture token not found: $ENV{SEARCH}\n" if $at < 0;
    substr($_, $at, length($ENV{SEARCH}), $ENV{REPLACEMENT});
  ' "$file"
}

delete_matching_line() {
  local file=$1
  local literal=$2
  LITERAL=$literal perl -ni -e 'print unless index($_, $ENV{LITERAL}) >= 0' "$file"
}

duplicate_matching_line() {
  local file=$1
  local literal=$2
  LITERAL=$literal perl -ni -e 'print; print if index($_, $ENV{LITERAL}) >= 0' "$file"
}

append_line() {
  local file=$1
  local line=$2
  printf '\n%s\n' "$line" >>"$file"
}

case_count=0
expect_failure() {
  local name=$1
  local expected=$2
  shift 2
  local fixture_dir="$tmp/$name"
  local log="$tmp/$name.log"
  cp -a "$baseline" "$fixture_dir"
  "$@" "$fixture_dir"
  if run_validator "$fixture_dir" >"$log" 2>&1; then
    echo "[phase-31.1-audit-fixtures] mutation unexpectedly passed: $name" >&2
    exit 1
  fi
  if ! grep -Fq "$expected" "$log"; then
    echo "[phase-31.1-audit-fixtures] mutation failed for the wrong reason: $name" >&2
    cat "$log" >&2
    exit 1
  fi
  case_count=$((case_count + 1))
  echo "[phase-31.1-audit-fixtures] rejected $name"
}

security_missing() {
  delete_matching_line "$1/31.1-SECURITY.md" '| T-31.1-11-01 |'
}

security_extra() {
  local file="$1/31.1-SECURITY.md"
  local row='| T-31.1-10-99 | high | pre_closeout | stale boundary | bash scripts/check.sh docs | PASS | 9a9098c63792023467262946bae7ae94b6f39fc4 | mitigated |'
  replace_first "$file" '| T-31.1-11-01 |' "$row
| T-31.1-11-01 |"
}

security_duplicate() {
  duplicate_matching_line "$1/31.1-SECURITY.md" '| T-31.1-11-01 |'
}

security_blank_boundary() {
  local file="$1/31.1-SECURITY.md"
  replace_first "$file" '| T-31.1-11-01 | high | pre_closeout | Bootstrap generation transition |' '| T-31.1-11-01 | high | pre_closeout |  |'
}

security_blank_command() {
  local file="$1/31.1-SECURITY.md"
  replace_first "$file" "| bash scripts/run-go-tests-matched.sh ./internal/releasecontract '^TestReadinessManagerBootstrapConcurrencyContract$' -race | PASS |" '|  | PASS |'
}

security_blank_result() {
  local file="$1/31.1-SECURITY.md"
  replace_first "$file" '| T-31.1-11-01 | high | pre_closeout | Bootstrap generation transition | bash scripts/run-go-tests-matched.sh' '| T-31.1-11-01 | high | pre_closeout | Bootstrap generation transition | bash scripts/run-go-tests-matched.sh'
  replace_first "$file" "-race | PASS | $audited_head | mitigated |" "-race |  | $audited_head | mitigated |"
}

security_wrong_commit() {
  local file="$1/31.1-SECURITY.md"
  replace_first "$file" "-race | PASS | $audited_head | mitigated |" '-race | PASS | 0000000000000000000000000000000000000000 | mitigated |'
}

security_premature_observation() {
  local file="$1/31.1-SECURITY.md"
  replace_first "$file" '| T-31.1-20-01 | high | post_closeout_external |' '| T-31.1-20-01 | high | pre_closeout |'
}

validation_missing_task() {
  delete_matching_line "$1/31.1-VALIDATION.md" '| 31.1-11-01 |'
}

validation_duplicate_task() {
  duplicate_matching_line "$1/31.1-VALIDATION.md" '| 31.1-11-01 |'
}

validation_missing_evidence() {
  local file="$1/31.1-VALIDATION.md"
  replace_first "$file" 'TestStrictRouterAdminReadinessCompositionContract, TestModelCatalogMutationContract' 'TestStrictRouterAdminReadinessCompositionContract'
}

validation_extra_evidence() {
  local file="$1/31.1-VALIDATION.md"
  replace_first "$file" 'TestReadinessManagerBootstrapConcurrencyContract | PASS |' 'TestReadinessManagerBootstrapConcurrencyContract, ASSERT-UNEXPECTED-EVIDENCE | PASS |'
}

validation_duplicate_evidence_cell() {
  local file="$1/31.1-VALIDATION.md"
  replace_first "$file" 'TestReadinessManagerBootstrapConcurrencyContract | PASS |' 'TestReadinessManagerBootstrapConcurrencyContract, TestReadinessManagerBootstrapConcurrencyContract | PASS |'
}

validation_appended_token() {
  local file="$1/31.1-VALIDATION.md"
  replace_first "$file" 'TestStrictRouterAdminReadinessCompositionContract, TestModelCatalogMutationContract' 'TestStrictRouterAdminReadinessCompositionContract'
  append_line "$file" 'Appendix token: TestModelCatalogMutationContract'
}

validation_frontmatter() {
  local key=$1
  local old=$2
  local new=$3
  local fixture_dir=$4
  replace_first "$fixture_dir/31.1-VALIDATION.md" "$key: $old" "$key: $new"
}

validation_legacy_text() {
  local text=$1
  local fixture_dir=$2
  append_line "$fixture_dir/31.1-VALIDATION.md" "$text"
}

validation_legacy_plan_row() {
  append_line "$1/31.1-VALIDATION.md" '| 31.1-01-01 | 01 | 1 | TestLegacyGapTask | PASS | 9a9098c63792023467262946bae7ae94b6f39fc4 |'
}

echo '[phase-31.1-audit-fixtures] positive baseline'
run_validator "$baseline"

expect_failure security-missing 'security threat set mismatch' security_missing
expect_failure security-extra 'security threat set mismatch' security_extra
expect_failure security-duplicate 'duplicate security threat id' security_duplicate
expect_failure security-blank-boundary 'source boundary is blank or placeholder' security_blank_boundary
expect_failure security-blank-command 'evidence command is blank or placeholder' security_blank_command
expect_failure security-blank-result 'observed result is blank or placeholder' security_blank_result
expect_failure security-wrong-commit 'commit identity must equal audited implementation head' security_wrong_commit
expect_failure security-premature-observation 'post-closeout row has invalid evidence phase' security_premature_observation

expect_failure validation-missing-task 'validation task set mismatch' validation_missing_task
expect_failure validation-duplicate-task 'duplicate validation task id' validation_duplicate_task
expect_failure validation-missing-evidence 'validation evidence pair set mismatch' validation_missing_evidence
expect_failure validation-extra-evidence 'validation evidence pair set mismatch' validation_extra_evidence
expect_failure validation-duplicate-evidence-cell 'duplicate evidence id in validation cell' validation_duplicate_evidence_cell
expect_failure validation-appended-token 'expected evidence id appears outside designated table' validation_appended_token
expect_failure validation-target-plans-10 'frontmatter target_plans must equal 22' validation_frontmatter target_plans 22 10
expect_failure validation-target-plans-20 'frontmatter target_plans must equal 22' validation_frontmatter target_plans 22 20
expect_failure validation-gap-plans-10 'frontmatter gap_plans must equal 12' validation_frontmatter gap_plans 12 10
expect_failure validation-mapped-tasks-22 'frontmatter mapped_gap_tasks must equal 26' validation_frontmatter mapped_gap_tasks 26 22
expect_failure validation-pairs-35 'frontmatter mapped_gap_evidence_pairs must equal 43' validation_frontmatter mapped_gap_evidence_pairs 43 35
expect_failure validation-unique-ids-32 'frontmatter unique_gap_evidence_ids must equal 40' validation_frontmatter unique_gap_evidence_ids 40 32
expect_failure validation-wave-pending 'frontmatter wave_0_complete must equal true' validation_frontmatter wave_0_complete true false
expect_failure validation-legacy-28-tasks 'legacy validation token is forbidden: 28 executable tasks' validation_legacy_text '28 executable tasks'
expect_failure validation-legacy-28-of-28 'legacy validation token is forbidden: 28/28' validation_legacy_text '28/28'
expect_failure validation-pending-map 'legacy validation token is forbidden: pending implementation' validation_legacy_text 'pending implementation'
expect_failure validation-legacy-plan-row 'normalized task id appears outside designated table' validation_legacy_plan_row

test "$case_count" -eq 25
echo "[phase-31.1-audit-fixtures] pass: 1 positive and $case_count rejected mutations"
