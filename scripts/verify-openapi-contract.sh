#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"

exec "$python_bin" "$repo_root/scripts/verify_openapi_contract.py" "$@"
