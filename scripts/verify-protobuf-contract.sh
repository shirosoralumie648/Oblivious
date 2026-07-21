#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="config/release/protobuf-toolchain.v1.json"
MODE="contract"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manifest)
      [[ $# -ge 2 ]] || { echo "protobuf_argument_invalid: --manifest requires a path" >&2; exit 2; }
      MANIFEST="$2"
      shift 2
      ;;
    --manifest-only)
      MODE="manifest-only"
      shift
      ;;
    --fixtures)
      MODE="fixtures"
      shift
      ;;
    *)
      echo "protobuf_argument_invalid: unsupported argument: $1" >&2
      exit 2
      ;;
  esac
done

case "$MODE" in
  manifest-only)
    exec python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" --manifest "$MANIFEST" --manifest-only
    ;;
  fixtures)
    exec python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" --manifest "$MANIFEST" --fixtures
    ;;
  contract)
    exec python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" --manifest "$MANIFEST"
    ;;
esac
