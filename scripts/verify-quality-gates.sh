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

workflow_file="$repo_root/.github/workflows/ci.yml"
readme_file="$repo_root/README.md"
release_checklist_file="$repo_root/docs/release/rc-checklist.md"
check_script="$repo_root/scripts/check.sh"
test_script="$repo_root/scripts/test.sh"

assert_file_exists "$workflow_file"
assert_file_exists "$readme_file"
assert_file_exists "$release_checklist_file"

assert_file_contains "$workflow_file" "web:"
assert_file_contains "$workflow_file" "server:"
assert_file_contains "$workflow_file" "bash scripts/check.sh"
assert_file_contains "$workflow_file" "bash scripts/test.sh"

assert_file_contains "$readme_file" "## Quick Start"
assert_file_contains "$readme_file" "bash scripts/check.sh"
assert_file_contains "$readme_file" "bash scripts/test.sh"

assert_file_contains "$release_checklist_file" "Frontend build passes"
assert_file_contains "$release_checklist_file" "Core route smoke tests pass"
assert_file_contains "$release_checklist_file" "Server contract tests pass"
assert_file_contains "$release_checklist_file" "Runtime configuration matches docs"
assert_file_contains "$release_checklist_file" "No P0/P1 defects open"

assert_file_contains "$check_script" "scripts/verify-quality-gates.sh"
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
