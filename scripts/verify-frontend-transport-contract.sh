#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
sidecar=""
manifest="$repo_root/docs/api/route-surface-manifest.json"
output="$repo_root/.tmp/frontend-transport-observation.json"
report_output="$repo_root/.tmp/frontend-transport-report.json"
stage_a=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sidecar) sidecar="${2:-}"; shift 2 ;;
    --manifest) manifest="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    --report-output) report_output="${2:-}"; shift 2 ;;
    --stage-a) stage_a=true; shift ;;
    *) printf 'frontend_transport_argument_invalid: %s\n' "$1" >&2; exit 2 ;;
  esac
done

[[ -n "$sidecar" && -f "$sidecar" ]] || {
  printf 'frontend_transport_argument_invalid: sidecar\n' >&2
  exit 2
}
if [[ "$stage_a" == true && "$(realpath -m "$manifest")" != "$(realpath -m "$repo_root/docs/api/route-surface-manifest.json")" ]]; then
  printf 'frontend_transport_argument_invalid: stage-a manifest\n' >&2
  exit 2
fi

python3 "$repo_root/scripts/verify_frontend_surface.py" transport \
  --sidecar "$sidecar" \
  --manifest "$manifest" \
  --output "$output"

if [[ "$stage_a" == true ]]; then
  tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-frontend-transport-report.XXXXXX")
  cleanup() { rm -rf -- "$tmp_root"; }
  trap cleanup EXIT
  identity_repo="$tmp_root/repository"
  git clone --quiet --no-hardlinks "$repo_root" "$identity_repo"
  expected_tree=$(git -C "$repo_root" rev-parse 'HEAD^{tree}')
  actual_tree=$(git -C "$identity_repo" rev-parse 'HEAD^{tree}')
  [[ "$actual_tree" == "$expected_tree" ]] || {
    printf 'frontend_transport_stage_a_identity_invalid: source-tree\n' >&2
    exit 1
  }
  sidecar_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sidecarDigest"])' "$output")
  (
    cd "$repo_root/src/server"
    go run ./cmd/release-contract report-frontend-transport \
      --repo "$identity_repo" \
      --contract config/release/contract.v1.json \
      --schema config/release/contract.schema.json \
      --profile monolith \
      --observation "$output" \
      --sidecar-digest "$sidecar_digest" \
      --output "$report_output"
  )
fi
