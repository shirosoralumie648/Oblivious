#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python}"

"$python_bin" "$repo_root/scripts/verify_deployment_operations_contract.py" "$repo_root"
