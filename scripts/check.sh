#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[check] Running workspace smoke test"
"$ROOT_DIR/scripts/test.sh"

echo "[check] Validating shell script syntax"
bash -n "$ROOT_DIR/scripts/dev.sh"
bash -n "$ROOT_DIR/scripts/test.sh"
bash -n "$ROOT_DIR/scripts/check.sh"

echo "[check] All workspace checks passed"
