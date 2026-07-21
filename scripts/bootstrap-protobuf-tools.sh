#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="config/release/protobuf-toolchain.v1.json"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manifest)
      [[ $# -ge 2 ]] || { echo "protobuf_argument_invalid: --manifest requires a path" >&2; exit 2; }
      MANIFEST="$2"
      shift 2
      ;;
    *)
      echo "protobuf_argument_invalid: unsupported bootstrap argument: $1" >&2
      exit 2
      ;;
  esac
done

if [[ "$MANIFEST" = /* ]]; then
  MANIFEST_PATH="$MANIFEST"
else
  MANIFEST_PATH="$ROOT_DIR/$MANIFEST"
fi

python3 "$ROOT_DIR/scripts/verify_protobuf_contract.py" --manifest "$MANIFEST_PATH" --manifest-only >/dev/null

case "$(uname -s)-$(uname -m)" in
  Linux-x86_64) PLATFORM="linux-amd64" ;;
  Linux-aarch64|Linux-arm64) PLATFORM="linux-arm64" ;;
  *)
    echo "protobuf_tool_platform_unsupported: $(uname -s)-$(uname -m)" >&2
    exit 1
    ;;
esac

mapfile -t TOOL_VALUES < <(python3 - "$MANIFEST_PATH" "$PLATFORM" <<'PY'
import json
from pathlib import Path
import sys

manifest = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
platform = manifest["tools"]["protoc"]["platforms"][sys.argv[2]]
values = [
    manifest["manifestDigest"],
    platform["url"],
    platform["sha256"],
]
for name in ("protoc-gen-go", "protoc-gen-go-grpc"):
    tool = manifest["tools"][name]
    values.extend([
        name,
        tool["module"],
        tool["checksumModule"],
        tool["moduleVersion"],
        tool["moduleSum"],
        tool["goModSum"],
        tool["versionOutput"],
    ])
for value in values:
    if not isinstance(value, str) or not value or "\n" in value:
        raise SystemExit("protobuf_manifest_schema_invalid: bootstrap field is not a safe scalar")
    print(value)
PY
)

[[ ${#TOOL_VALUES[@]} -eq 17 ]] || { echo "protobuf_manifest_schema_invalid: bootstrap scalar count mismatch" >&2; exit 1; }
MANIFEST_DIGEST="${TOOL_VALUES[0]}"
PROTOC_URL="${TOOL_VALUES[1]}"
PROTOC_SHA256="${TOOL_VALUES[2]}"
TOOLS_ROOT="$ROOT_DIR/.tmp/protobuf-tools"
BIN_DIR="$TOOLS_ROOT/bin"
mkdir -p "$TOOLS_ROOT" || { echo "protobuf_tool_directory_unwritable: $TOOLS_ROOT" >&2; exit 1; }
[[ -w "$TOOLS_ROOT" ]] || { echo "protobuf_tool_directory_unwritable: $TOOLS_ROOT" >&2; exit 1; }

verify_versions() {
  local candidate="$1"
  [[ -x "$candidate/protoc" && ! -L "$candidate/protoc" ]] || return 1
  [[ -x "$candidate/protoc-gen-go" && ! -L "$candidate/protoc-gen-go" ]] || return 1
  [[ -x "$candidate/protoc-gen-go-grpc" && ! -L "$candidate/protoc-gen-go-grpc" ]] || return 1
  [[ "$($candidate/protoc --version 2>&1)" == "libprotoc 25.1" ]] || return 1
  [[ "$($candidate/protoc-gen-go --version 2>&1)" == "protoc-gen-go v1.36.11" ]] || return 1
  [[ "$($candidate/protoc-gen-go-grpc --version 2>&1)" == "protoc-gen-go-grpc 1.6.2" ]] || return 1
  [[ -f "$candidate/.protobuf-toolchain-digest" && ! -L "$candidate/.protobuf-toolchain-digest" ]] || return 1
  [[ "$(<"$candidate/.protobuf-toolchain-digest")" == "$MANIFEST_DIGEST" ]] || return 1
}

if verify_versions "$BIN_DIR"; then
  printf 'protobuf tools verified at %s (%s)\n' "$BIN_DIR" "$MANIFEST_DIGEST"
  exit 0
fi

STAGE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/oblivious-protobuf-tools.XXXXXX")"
NEW_DIR=""
OLD_DIR=""
cleanup() {
  [[ -z "$NEW_DIR" || ! -e "$NEW_DIR" ]] || rm -rf -- "$NEW_DIR"
  [[ -z "$OLD_DIR" || ! -e "$OLD_DIR" ]] || {
    if [[ ! -e "$BIN_DIR" ]]; then
      mv -- "$OLD_DIR" "$BIN_DIR" || true
    else
      rm -rf -- "$OLD_DIR"
    fi
  }
  chmod -R u+w "$STAGE_DIR" 2>/dev/null || true
  rm -rf -- "$STAGE_DIR"
}
trap cleanup EXIT

mkdir -p "$STAGE_DIR/bin" "$STAGE_DIR/download" "$STAGE_DIR/gomodcache" "$STAGE_DIR/gopath"
curl --proto '=https' --tlsv1.2 --http1.1 -fL \
  --retry 5 --retry-all-errors --connect-timeout 15 --max-time 240 \
  -o "$STAGE_DIR/download/protoc.zip" "$PROTOC_URL"
ACTUAL_PROTOC_SHA256="$(sha256sum "$STAGE_DIR/download/protoc.zip" | awk '{print $1}')"
[[ "$ACTUAL_PROTOC_SHA256" == "$PROTOC_SHA256" ]] || {
  echo "protobuf_tool_checksum_mismatch: protoc archive checksum differs" >&2
  exit 1
}
unzip -q "$STAGE_DIR/download/protoc.zip" 'bin/protoc' -d "$STAGE_DIR/unpack"
cp "$STAGE_DIR/unpack/bin/protoc" "$STAGE_DIR/bin/protoc"
chmod 0755 "$STAGE_DIR/bin/protoc"

install_go_tool() {
  local value_offset="$1"
  local name="${TOOL_VALUES[$value_offset]}"
  local module="${TOOL_VALUES[$((value_offset + 1))]}"
  local checksum_module="${TOOL_VALUES[$((value_offset + 2))]}"
  local module_version="${TOOL_VALUES[$((value_offset + 3))]}"
  local expected_sum="${TOOL_VALUES[$((value_offset + 4))]}"
  local expected_go_mod_sum="${TOOL_VALUES[$((value_offset + 5))]}"
  local expected_version="${TOOL_VALUES[$((value_offset + 6))]}"
  local download_json
  local actual_values
  local attempt

  download_json=""
  for attempt in 1 2 3; do
    if download_json="$(GOWORK=off GOMODCACHE="$STAGE_DIR/gomodcache" GOPATH="$STAGE_DIR/gopath" go mod download -json "$checksum_module@$module_version")"; then
      break
    fi
    echo "protobuf_tool_download_retry: $name attempt $attempt" >&2
    [[ "$attempt" -eq 3 ]] || sleep "$attempt"
  done
  [[ -n "$download_json" ]] || {
    echo "protobuf_tool_download_failed: $name" >&2
    return 1
  }
  actual_values="$(DOWNLOAD_JSON="$download_json" python3 - <<'PY'
import json
import os

value = json.loads(os.environ["DOWNLOAD_JSON"])
print(value.get("Sum", ""))
print(value.get("GoModSum", ""))
PY
)"
  [[ "$actual_values" == "$expected_sum"$'\n'"$expected_go_mod_sum" ]] || {
    echo "protobuf_tool_checksum_mismatch: $name module checksum differs" >&2
    return 1
  }
  GOWORK=off GOMODCACHE="$STAGE_DIR/gomodcache" GOPATH="$STAGE_DIR/gopath" GOBIN="$STAGE_DIR/bin" \
    go install "$module@$module_version"
  [[ -x "$STAGE_DIR/bin/$name" ]] || {
    echo "protobuf_tool_install_missing: $name" >&2
    return 1
  }
  [[ "$($STAGE_DIR/bin/$name --version 2>&1)" == "$expected_version" ]] || {
    echo "protobuf_tool_version_mismatch: $name" >&2
    return 1
  }
}

install_go_tool 3
install_go_tool 10
printf '%s\n' "$MANIFEST_DIGEST" > "$STAGE_DIR/bin/.protobuf-toolchain-digest"
verify_versions "$STAGE_DIR/bin" || {
  echo "protobuf_tool_version_mismatch: staged toolchain failed exact version checks" >&2
  exit 1
}

NEW_DIR="$TOOLS_ROOT/.bin.new.$$"
mv -- "$STAGE_DIR/bin" "$NEW_DIR"
if [[ -e "$BIN_DIR" ]]; then
  OLD_DIR="$TOOLS_ROOT/.bin.old.$$"
  mv -- "$BIN_DIR" "$OLD_DIR"
fi
if ! mv -- "$NEW_DIR" "$BIN_DIR"; then
  [[ -z "$OLD_DIR" ]] || mv -- "$OLD_DIR" "$BIN_DIR"
  echo "protobuf_tool_install_failed: atomic tool directory replacement failed" >&2
  exit 1
fi
NEW_DIR=""
if [[ -n "$OLD_DIR" ]]; then
  rm -rf -- "$OLD_DIR"
  OLD_DIR=""
fi
verify_versions "$BIN_DIR" || {
  echo "protobuf_tool_version_mismatch: installed toolchain failed exact version checks" >&2
  exit 1
}
printf 'protobuf tools installed at %s (%s)\n' "$BIN_DIR" "$MANIFEST_DIGEST"
