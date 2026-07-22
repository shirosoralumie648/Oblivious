#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
mode=""
source_root=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --self-check|--stage-a)
      [[ -z "$mode" ]] || { printf 'frontend_sidecar_argument_invalid\n' >&2; exit 2; }
      mode="$1"
      shift
      ;;
    --root)
      [[ $# -ge 2 && -n "$2" ]] || { printf 'frontend_sidecar_argument_invalid\n' >&2; exit 2; }
      source_root=$(cd "$2" && pwd -P)
      shift 2
      ;;
    *)
      printf 'frontend_sidecar_argument_invalid\n' >&2
      exit 2
      ;;
  esac
done

[[ -n "$mode" ]] || { printf 'frontend_sidecar_argument_invalid: mode\n' >&2; exit 2; }
if [[ -z "$source_root" ]]; then
  if [[ "$mode" == "--self-check" ]]; then
    source_root="$repo_root/scripts/testdata/frontend-surface/production"
  else
    source_root="$repo_root/src/web/src"
  fi
fi

tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-frontend-sidecar.XXXXXX")
cleanup() { rm -rf -- "$tmp_root"; }
trap cleanup EXIT

if [[ "$mode" == "--self-check" ]]; then
  test_counts="$tmp_root/test-counts.json"
  FRONTEND_SIDECAR_TEST_COUNTS="$test_counts" node --test --test-reporter=tap "$repo_root/scripts/frontend_surface_sidecar.test.mjs" >"$tmp_root/tap.txt"
  python3 - "$tmp_root/tap.txt" "$test_counts" <<'PY'
import json
from pathlib import Path
import re
import sys

tap = Path(sys.argv[1]).read_text(encoding="utf-8")
counts = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
if not re.search(r"^# tests 6$", tap, re.MULTILINE):
    raise SystemExit("frontend_sidecar_tap_plan_invalid")
required = {
    "httpClient", "rawFetch", "swr", "multipartUpload", "sseStream", "eventSource", "websocket",
    "exposure", "generatedConsumer", "exclusion", "deterministic", "zeroInventory", "invalidConfig",
    "genericOnlyIdentity", "unresolvedDecoder", "malformedGenerated", "methodMismatch", "pathMismatch",
}
if set(counts) != required or any(not isinstance(value, int) or value <= 0 for value in counts.values()):
    raise SystemExit("frontend_sidecar_fixture_counts_invalid")
PY
fi

config="$source_root/tsconfig.json"
[[ -f "$config" ]] || config="$(dirname "$source_root")/tsconfig.json"
generated="$source_root/generated/client.generated.ts"
[[ -f "$generated" ]] || generated="$source_root/generated/operation-contracts.generated.ts"
[[ -f "$generated" ]] || generated=""
args=(--root "$source_root" --tsconfig "$config" --output "$tmp_root/first.json")
[[ -n "$generated" ]] && args+=(--generated-file "$generated")
node "$repo_root/scripts/frontend_surface_sidecar.mjs" "${args[@]}"
args+=(--output "$tmp_root/second.json")
node "$repo_root/scripts/frontend_surface_sidecar.mjs" "${args[@]}"
cmp -s "$tmp_root/first.json" "$tmp_root/second.json" || {
  printf 'frontend_sidecar_nondeterministic\n' >&2
  exit 1
}

python3 - "$tmp_root/first.json" "$source_root" <<'PY'
import json
from pathlib import Path
import re
import sys

report = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
root = Path(sys.argv[2]).as_posix()
if report.get("schemaVersion") != "frontend-surface-sidecar/v1":
    raise SystemExit("frontend_sidecar_schema_invalid")
scope = report.get("sourceScope", {})
if not isinstance(scope.get("filesScanned"), int) or scope["filesScanned"] <= 0:
    raise SystemExit("frontend_sidecar_files_empty")
if not re.fullmatch(r"sha256:[0-9a-f]{64}", scope.get("sourceDigest", "")):
    raise SystemExit("frontend_sidecar_source_digest_invalid")
if not report.get("operations") or not report.get("exposures") or not report.get("generatedConsumers"):
    raise SystemExit("frontend_sidecar_positive_inventory_required")
if report.get("unresolved") != []:
    raise SystemExit("frontend_sidecar_unresolved")
kinds = {entry.get("transport", {}).get("kind") for entry in report["operations"]}
required_kinds = {"http-client", "sse-stream", "swr", "websocket"}
if root.endswith("/scripts/testdata/frontend-surface/production"):
    required_kinds |= {"raw-fetch", "multipart-upload", "event-source"}
if not required_kinds.issubset(kinds):
    raise SystemExit("frontend_sidecar_taxonomy_incomplete")
if root.endswith("/src/web/src"):
    closure = scope.get("ownerClosure", {})
    if closure.get("expected") != 25 or closure.get("resolved") != 24 or closure.get("nonCallers") != 1:
        raise SystemExit("frontend_sidecar_owner_closure_invalid")
PY

printf '[frontend-sidecar] %s verified: files=%s operations=%s exposures=%s\n' \
  "${mode#--}" "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sourceScope"]["filesScanned"])' "$tmp_root/first.json")" \
  "$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["operations"]))' "$tmp_root/first.json")" \
  "$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["exposures"]))' "$tmp_root/first.json")"
