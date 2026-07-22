#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
mode="--stage-a"
output_path=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --stage-a|--clean-head)
      mode="$1"
      shift
      ;;
    --output)
      [[ $# -ge 2 && -n "$2" ]] || {
        printf 'http_runtime_argument_invalid: output\n' >&2
        exit 2
      }
      output_path="$2"
      shift 2
      ;;
    *)
      printf 'http_runtime_argument_invalid: unsupported argument\n' >&2
      exit 2
      ;;
  esac
done

fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-http-runtime.XXXXXX")
checkout="$repo_root"
producer="$fixture_root/release-http-surface"
verifier="$fixture_root/release-contract"
producer_invocations=0

cleanup() {
  rm -rf -- "$fixture_root"
}
trap cleanup EXIT

fail() {
  printf 'http_runtime_contract_failed: %s\n' "$1" >&2
  exit 1
}

create_stage_a_checkout() {
  local relative
  local -a tracked_files
  checkout="$fixture_root/checkout"
  mkdir -p "$checkout"
  mapfile -d '' tracked_files < <(git -C "$repo_root" ls-files -z -- . ':(exclude)reference/**' ':(exclude).planning/**')
  (( ${#tracked_files[@]} > 0 )) || fail "tracked inventory empty"
  git -C "$repo_root" checkout-index --prefix="$checkout/" -- "${tracked_files[@]}"

  for relative in \
    src/server/cmd/release-http-surface/main.go \
    src/server/cmd/release-http-surface/main_test.go \
    src/server/internal/http/route_surface.go \
    src/server/internal/http/route_surface_test.go \
    scripts/verify-http-runtime-contract.sh \
    scripts/verify-http-runtime-contract-fixtures.sh; do
    [[ -f "$repo_root/$relative" ]] || fail "owned file missing"
    mkdir -p "$checkout/$(dirname "$relative")"
    cp "$repo_root/$relative" "$checkout/$relative"
  done

  git -C "$checkout" init -q
  git -C "$checkout" add -- .
  git -C "$checkout" -c user.name=http-runtime-gate -c user.email=http-runtime-gate.invalid commit -q -m snapshot
}

if [[ "$mode" == "--stage-a" ]]; then
  create_stage_a_checkout
else
  [[ -z "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]] || fail "clean-head worktree dirty"
fi

if [[ -z "$output_path" ]]; then
  output_path="$fixture_root/http-runtime.json"
fi

(
  cd "$checkout/src/server"
  go build -o "$producer" ./cmd/release-http-surface
  go build -o "$verifier" ./cmd/release-contract
)

producer_invocations=$((producer_invocations + 1))
env -u GITHUB_SHA "$producer" \
  --repo "$checkout" \
  --contract config/release/contract.v1.json \
  --schema config/release/contract.schema.json \
  --profile monolith \
  --manifest docs/api/route-surface-manifest.json \
  --output "$output_path"
[[ $producer_invocations -eq 1 ]] || fail "producer invocation count"
[[ -s "$output_path" ]] || fail "report missing"

"$verifier" verify-report --input "$output_path" >/dev/null
python3 - "$output_path" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1])
report = json.loads(path.read_text(encoding="utf-8"))
if report.get("schemaVersion") != "surface-report/v1":
    raise SystemExit("http_runtime_report_schema_invalid")
identity = report.get("releaseIdentity", {})
if identity.get("dirty") is not False or identity.get("evidenceClass") != "repository-local":
    raise SystemExit("http_runtime_identity_invalid")
surface = report.get("surfaceIdentity", {})
if surface.get("surface") != "http-runtime" or surface.get("canonicalSource") != "docs/api/openapi.yaml" or surface.get("consumer") != "runtime-route-registry":
    raise SystemExit("http_runtime_surface_invalid")
details = report.get("evidence", {}).get("details", {})
if details.get("operationCount") != 197 or details.get("mountedCount") != 197 or details.get("descriptorCount") != 197:
    raise SystemExit("http_runtime_owner_closure_invalid")
if not isinstance(details.get("mediaProbeCount"), int) or details["mediaProbeCount"] <= 0:
    raise SystemExit("http_runtime_media_probe_empty")
if details.get("parityResult") != "pass" or details.get("coreDigest") != details.get("runtimeDigest"):
    raise SystemExit("http_runtime_parity_invalid")
if report.get("drift") != {"missing": [], "extra": [], "incompatible": []}:
    raise SystemExit("http_runtime_drift_invalid")
if report.get("outcome") != {"result": "pass", "errorCodes": [], "skippedChecks": []}:
    raise SystemExit("http_runtime_outcome_invalid")
encoded = json.dumps(report, sort_keys=True)
for browser_field in ("requestEncoder", "responseDecoder", "eventSchema"):
    if browser_field in encoded:
        raise SystemExit("http_runtime_browser_field_present")
PY

printf '[http-runtime-contract] %s verified: owners=17 operations=197 producerInvocations=1\n' "${mode#--}"
