#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="config/release/protobuf-toolchain.v1.json"
MODE="contract"
OBSERVATION_OUT=""

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
    --fresh-checkout)
      MODE="fresh-checkout"
      shift
      ;;
    --stage-a)
      MODE="stage-a"
      shift
      ;;
    --observation-out)
      [[ $# -ge 2 ]] || { echo "protobuf_argument_invalid: --observation-out requires a path" >&2; exit 2; }
      OBSERVATION_OUT="$2"
      shift 2
      ;;
    *)
      echo "protobuf_argument_invalid: unsupported argument: $1" >&2
      exit 2
      ;;
  esac
done

run_current_checkout() {
  bash "$ROOT_DIR/scripts/bootstrap-protobuf-tools.sh" --manifest "$MANIFEST"
  local arguments=(
    --manifest "$MANIFEST"
    --tool-bin "$ROOT_DIR/.tmp/protobuf-tools/bin"
    --regenerate
    --regeneration-fixtures
  )
  if [[ -n "$OBSERVATION_OUT" ]]; then
    arguments+=(--observation-out "$OBSERVATION_OUT")
  fi
  python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" "${arguments[@]}"
}

run_fresh_checkout() {
  [[ "$MANIFEST" == "config/release/protobuf-toolchain.v1.json" ]] || {
    echo "protobuf_argument_invalid: fresh-checkout requires the canonical manifest" >&2
    exit 2
  }
  local fixture_root checkout allowed_bin relative destination command_path
  fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/oblivious-protobuf-fresh.XXXXXX")"
  checkout="$fixture_root/checkout"
  allowed_bin="$fixture_root/allowed-bin"
  mkdir -p "$checkout" "$allowed_bin"
  trap 'rm -rf -- "$fixture_root"' RETURN

  copy_relative() {
    relative="$1"
    destination="$checkout/$relative"
    mkdir -p "$(dirname "$destination")"
    cp "$ROOT_DIR/$relative" "$destination"
  }

  copy_relative .gitignore
  copy_relative config/release/protobuf-toolchain.v1.json
  copy_relative scripts/bootstrap-protobuf-tools.sh
  copy_relative scripts/verify-protobuf-contract.sh
  copy_relative scripts/verify_protobuf_contract.py
  copy_relative src/server/Makefile
  copy_relative src/server/pkg/metrics/client.go
  while IFS= read -r -d '' relative; do
    [[ "$relative" == reference/* ]] || copy_relative "$relative"
  done < <(git -C "$ROOT_DIR" ls-files -z -- '*.proto' '*.pb.go' '*_grpc.pb.go')

  git -C "$checkout" init -q
  git -C "$checkout" add -- .gitignore config scripts src/server/Makefile src/server/api/proto src/server/internal/grpc src/server/pkg api/proto
  git -C "$checkout" -c user.name=protobuf-gate -c user.email=protobuf-gate.invalid commit -q -m snapshot

  for command_path in bash python3 git curl unzip sha256sum go uname dirname mktemp mkdir cp chmod mv rm awk sleep; do
    command_path="$(type -P "$command_path")"
    [[ -n "$command_path" ]] || {
      echo "protobuf_fresh_dependency_missing: $command_path" >&2
      exit 1
    }
    ln -s "$command_path" "$allowed_bin/$(basename "$command_path")"
  done

  (
    cd "$checkout"
    PATH="$allowed_bin" bash scripts/bootstrap-protobuf-tools.sh --manifest config/release/protobuf-toolchain.v1.json
    PATH="$allowed_bin" python3 scripts/verify_protobuf_contract.py \
      --manifest config/release/protobuf-toolchain.v1.json \
      --tool-bin "$checkout/.tmp/protobuf-tools/bin" \
      --regenerate \
      --regeneration-fixtures
    git diff --exit-code -- '*.proto' '*.pb.go' '*_grpc.pb.go'
    [[ -z "$(git status --porcelain --untracked-files=no)" ]] || {
      echo "protobuf_fresh_checkout_mutated: tracked snapshot changed" >&2
      exit 1
    }
  )
}

case "$MODE" in
  manifest-only)
    exec python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" --manifest "$MANIFEST" --manifest-only
    ;;
  fixtures)
    exec python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" --manifest "$MANIFEST" --fixtures
    ;;
  fresh-checkout)
    run_fresh_checkout
    ;;
  stage-a|contract)
    run_current_checkout
    ;;
esac
