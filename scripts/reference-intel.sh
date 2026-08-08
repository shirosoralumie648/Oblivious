#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python3}"

cd "$repo_root"
exec "$python_bin" "$repo_root/scripts/reference_intel/pipeline.py" "$@"
