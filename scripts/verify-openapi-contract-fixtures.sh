#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
python_bin="${PYTHON:-python3}"
suite="all"

case "${1:-}" in
  --projector-only) suite="projector" ;;
  --determinism) suite="determinism" ;;
  "") ;;
  *)
    echo "usage: $0 [--projector-only|--determinism]" >&2
    exit 2
    ;;
esac

exec "$python_bin" "$repo_root/scripts/openapi_surface_fingerprint.py" \
  --openapi "$repo_root/docs/api/openapi.yaml" \
  --contract "$repo_root/config/release/contract.v1.json" \
  --schema "$repo_root/docs/api/route-surface-manifest.schema.json" \
  --fixture-suite "$suite"
