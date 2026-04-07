#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

assert_file_exists() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "[quality-gates] missing file: $path" >&2
    exit 1
  fi
}

assert_file_contains() {
  local path="$1"
  local pattern="$2"
  if ! rg -q --fixed-strings "$pattern" "$path"; then
    echo "[quality-gates] expected pattern '$pattern' in $path" >&2
    exit 1
  fi
}

assert_file_not_contains() {
  local path="$1"
  local pattern="$2"
  if rg -q --fixed-strings "$pattern" "$path"; then
    echo "[quality-gates] unexpected pattern '$pattern' in $path" >&2
    exit 1
  fi
}

workflow_file="$repo_root/.github/workflows/ci.yml"
workspace_file="$repo_root/pnpm-workspace.yaml"
gitignore_file="$repo_root/.gitignore"
package_file="$repo_root/package.json"
readme_file="$repo_root/README.md"
release_checklist_file="$repo_root/docs/release/rc-checklist.md"
owner_matrix_file="$repo_root/docs/governance/owner-matrix.md"
weekly_status_file="$repo_root/docs/governance/weekly-status-template.md"
blocker_escalation_file="$repo_root/docs/governance/blocker-escalation.md"
check_script="$repo_root/scripts/check.sh"
test_script="$repo_root/scripts/test.sh"

assert_file_exists "$workflow_file"
assert_file_exists "$readme_file"
assert_file_exists "$release_checklist_file"
assert_file_exists "$owner_matrix_file"
assert_file_exists "$weekly_status_file"
assert_file_exists "$blocker_escalation_file"

assert_file_not_contains "$workflow_file" "phase0-task1-contracts"
assert_file_contains "$workflow_file" "web:"
assert_file_contains "$workflow_file" "server:"
assert_file_contains "$workflow_file" "bash scripts/check.sh"
assert_file_contains "$workflow_file" "bash scripts/test.sh"

assert_file_contains "$package_file" '"dev": "bash scripts/dev.sh"'
assert_file_contains "$package_file" '"check": "bash scripts/check.sh"'
assert_file_contains "$package_file" '"test": "bash scripts/test.sh"'

assert_file_not_contains "$workspace_file" '"lobehub"'
assert_file_not_contains "$workspace_file" '"new-api"'
assert_file_contains "$gitignore_file" ".superpowers/"

assert_file_contains "$readme_file" "## Quick Start"
assert_file_contains "$readme_file" "bash scripts/check.sh"
assert_file_contains "$readme_file" "bash scripts/test.sh"
assert_file_not_contains "$readme_file" ".worktrees/phase0-task1-contracts"

assert_file_contains "$release_checklist_file" "Frontend build passes"
assert_file_contains "$release_checklist_file" "Core route smoke tests pass"
assert_file_contains "$release_checklist_file" "Server contract tests pass"
assert_file_contains "$release_checklist_file" "Runtime configuration matches docs"
assert_file_contains "$release_checklist_file" "No P0/P1 defects open"
assert_file_not_contains "$release_checklist_file" ".worktrees/phase0-task1-contracts"

assert_file_contains "$owner_matrix_file" "| TL |"
assert_file_contains "$owner_matrix_file" "| FE |"
assert_file_contains "$owner_matrix_file" "| BE |"
assert_file_contains "$weekly_status_file" "Actual Owner:"
assert_file_contains "$weekly_status_file" "## Risks / Blockers"
assert_file_contains "$blocker_escalation_file" "| Severity | Definition | Response Window | Escalate To |"

assert_file_contains "$check_script" "scripts/verify-quality-gates.sh"
assert_file_contains "$check_script" 'workspace_file="$repo_root/pnpm-workspace.yaml"'
assert_file_contains "$check_script" 'Unexpected workspace member: lobehub'
assert_file_contains "$check_script" 'Unexpected workspace member: new-api'
assert_file_contains "$check_script" 'pnpm --dir "$web_dir" build'
assert_file_contains "$check_script" "docs/architecture/current-system-contracts.md"
assert_file_contains "$check_script" "config/.env.example"
assert_file_contains "$check_script" "go test ./internal/config ./internal/chat ./internal/knowledge ./internal/task ./internal/console"

assert_file_contains "$test_script" "Running server unit tests."
assert_file_contains "$test_script" "Running server integration tests."
assert_file_contains "$test_script" "Skipping server integration tests: TEST_DATABASE_URL not set."
assert_file_contains "$test_script" "go test ./internal/config ./internal/chat ./internal/knowledge ./internal/task ./internal/console"
assert_file_contains "$test_script" "go test ./internal/http"

echo "[quality-gates] quality gate assets look complete."
