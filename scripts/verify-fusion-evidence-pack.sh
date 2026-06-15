#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
pack_file="$repo_root/docs/release/fusion-spec-evidence-pack.md"
matrix_file="$repo_root/docs/reports/2026-06-07-fusion-spec-completion-matrix.md"

require_file() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "[fusion-evidence-pack] missing file: $path" >&2
    exit 1
  fi
}

require_contains() {
  local path="$1"
  local pattern="$2"
  if ! grep -Fq -- "$pattern" "$path"; then
    echo "[fusion-evidence-pack] expected '$pattern' in $path" >&2
    exit 1
  fi
}

require_file "$pack_file"
require_file "$matrix_file"

require_contains "$pack_file" "not a final completion claim"
require_contains "$pack_file" "docs/reports/2026-06-07-fusion-spec-completion-matrix.md"
require_contains "$pack_file" "## Requirement Evidence Index"
require_contains "$pack_file" "## Required Final Commands"
require_contains "$pack_file" "## Unresolved Risk List"
require_contains "$pack_file" "TEST_DATABASE_URL"
require_contains "$pack_file" "scripts/deploy-validate.sh"
require_contains "$pack_file" "scripts/k8s-validate.sh"
require_contains "$pack_file" "scripts/backup-restore-smoke.sh"
require_contains "$pack_file" "scripts/verify-target-release-evidence.sh"
require_contains "$pack_file" "--print-template"
require_contains "$pack_file" "scripts/verify-commercial-completion.sh"
require_contains "$pack_file" "scripts/verify-commercial-db-evidence.sh"
require_contains "$pack_file" "A skipped command is not successful proof"
require_contains "$pack_file" 'Keep rows marked `Partial`'
require_contains "$matrix_file" "Migration strategy and release readiness"
require_contains "$matrix_file" "Partial"

echo "[fusion-evidence-pack] fusion evidence pack is present and guarded."
