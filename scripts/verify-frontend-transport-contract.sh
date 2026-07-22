#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
sidecar=""
manifest="$repo_root/docs/api/route-surface-manifest.json"
output="$repo_root/.tmp/frontend-transport-observation.json"
stage_a=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sidecar) sidecar="${2:-}"; shift 2 ;;
    --manifest) manifest="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
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

exec python3 "$repo_root/scripts/verify_frontend_surface.py" transport \
  --sidecar "$sidecar" \
  --manifest "$manifest" \
  --output "$output"
