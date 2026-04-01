#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

required_files=(
  "package.json"
  "pnpm-workspace.yaml"
  ".gitignore"
  "config/.env.example"
  "scripts/dev.sh"
  "scripts/test.sh"
  "scripts/check.sh"
)

missing=0
for file in "${required_files[@]}"; do
  if [[ ! -f "$ROOT_DIR/$file" ]]; then
    printf 'Missing required file: %s\n' "$file"
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

printf 'Workspace smoke test passed.\n'
