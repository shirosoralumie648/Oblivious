#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
sidecar=""
contract="$repo_root/config/release/contract.v1.json"
schema="$repo_root/config/release/contract.schema.json"
app_projection=""
server_catalog=""
output="$repo_root/.tmp/frontend-exposure-observation.json"
report_output="$repo_root/.tmp/frontend-exposure-report.json"
stage_a=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --sidecar) sidecar="${2:-}"; shift 2 ;;
    --contract) contract="${2:-}"; shift 2 ;;
    --schema) schema="${2:-}"; shift 2 ;;
    --app-projection) app_projection="${2:-}"; shift 2 ;;
    --server-catalog) server_catalog="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    --report-output) report_output="${2:-}"; shift 2 ;;
    --stage-a) stage_a=true; shift ;;
    *) printf 'frontend_exposure_argument_invalid: %s\n' "$1" >&2; exit 2 ;;
  esac
done

[[ -n "$sidecar" && -f "$sidecar" ]] || {
  printf 'frontend_exposure_argument_invalid: sidecar\n' >&2
  exit 2
}

canonical_contract="$repo_root/config/release/contract.v1.json"
canonical_schema="$repo_root/config/release/contract.schema.json"
if [[ "$stage_a" == true ]]; then
  if [[ "$(realpath -m "$contract")" != "$(realpath -m "$canonical_contract")" || "$(realpath -m "$schema")" != "$(realpath -m "$canonical_schema")" ]]; then
    printf 'frontend_exposure_argument_invalid: stage-a contract\n' >&2
    exit 2
  fi
  if [[ -n "$app_projection" || -n "$server_catalog" ]]; then
    printf 'frontend_exposure_argument_invalid: stage-a observation override\n' >&2
    exit 2
  fi

  tmp_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-frontend-exposure.XXXXXX")
  cleanup() { rm -rf -- "$tmp_root"; }
  trap cleanup EXIT
  app_projection="$tmp_root/app-projection.json"
  server_catalog="$tmp_root/server-catalog.json"
  identity_repo="$tmp_root/repository"
  git clone --quiet --no-hardlinks "$repo_root" "$identity_repo"

  source_tree=$(git -C "$repo_root" rev-parse 'HEAD^{tree}')
  [[ "$source_tree" =~ ^[0-9a-f]{40}$ ]] || {
    printf 'frontend_exposure_stage_a_identity_invalid: source-tree\n' >&2
    exit 1
  }
  [[ "$(git -C "$identity_repo" rev-parse 'HEAD^{tree}')" == "$source_tree" ]] || {
    printf 'frontend_exposure_stage_a_identity_invalid: snapshot-tree\n' >&2
    exit 1
  }
  digest_stderr="$tmp_root/digest.stderr"
  if ! digest_json=$(cd "$repo_root/src/server" && go run ./cmd/release-contract digest \
    --repo "$identity_repo" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json 2>"$digest_stderr"); then
    cat "$digest_stderr" >&2
    exit 1
  fi
  contract_digest=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("contractDigest", ""))' <<<"$digest_json")
  [[ "$contract_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || {
    printf 'frontend_exposure_stage_a_identity_invalid: contract-digest\n' >&2
    exit 1
  }

  python3 - "$contract" "$source_tree" "$contract_digest" "$app_projection" "$server_catalog" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

contract_path, source_tree, contract_digest, app_path, catalog_path = sys.argv[1:]
contract = json.loads(Path(contract_path).read_text(encoding="utf-8"))
identity = {
    "sourceTree": source_tree,
    "contractDigest": contract_digest,
    "deploymentProfile": "monolith",
}

capability_rows = []
for capability in sorted(contract["capabilities"], key=lambda row: row["id"]):
    disposition = capability["commitment"]
    if disposition == "excluded":
        continue
    enabled = disposition == "committed"
    capability_rows.append(
        {
            "capabilityId": capability["id"],
            "disposition": disposition,
            "availability": "enabled" if enabled else "disabled",
            "enabled": enabled,
        }
    )

generation = 1
digest_payload = {
    "identity": identity,
    "generation": generation,
    "capabilities": capability_rows,
}
projection_digest = "sha256:" + hashlib.sha256(
    json.dumps(digest_payload, ensure_ascii=False, separators=(",", ":")).encode()
).hexdigest()
app_projection = {
    "schemaVersion": "frontend-app-projection-observation/v1",
    "provenance": {
        "source": "authenticated-api",
        "provider": "ReleaseProjectionProvider",
        "operationId": "getAppReadinessCapabilities",
    },
    "releaseIdentity": identity,
    "generation": generation,
    "projectionDigest": projection_digest,
    "capabilities": capability_rows,
}

profile = next(
    row
    for row in contract["profiles"]
    if row["id"] == "monolith" and row["commitment"] == "committed"
)
binding_index = {row["id"]: row for row in contract["catalogBindings"]}
subjects = sorted(
    (binding_index[binding_id] for binding_id in profile["catalogBindingIds"]),
    key=lambda row: row["id"],
)
server_catalog = {
    "schemaVersion": "frontend-server-catalog-observation/v1",
    "releaseIdentity": identity,
    "subjects": subjects,
}

Path(app_path).write_text(json.dumps(app_projection, indent=2, sort_keys=True) + "\n", encoding="utf-8")
Path(catalog_path).write_text(json.dumps(server_catalog, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
else
  [[ -n "$app_projection" && -f "$app_projection" && -n "$server_catalog" && -f "$server_catalog" ]] || {
    printf 'frontend_exposure_argument_invalid: observations\n' >&2
    exit 2
  }
fi

python3 "$repo_root/scripts/verify_frontend_surface.py" exposure \
  --sidecar "$sidecar" \
  --contract "$contract" \
  --app-projection "$app_projection" \
  --server-catalog "$server_catalog" \
  --output "$output"

if [[ "$stage_a" == true ]]; then
  sidecar_digest=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["sidecarDigest"])' "$output")
  (
    cd "$repo_root/src/server"
    go run ./cmd/release-contract report-frontend-exposure \
      --repo "$identity_repo" \
      --contract config/release/contract.v1.json \
      --schema config/release/contract.schema.json \
      --profile monolith \
      --observation "$output" \
      --sidecar-digest "$sidecar_digest" \
      --output "$report_output"
  )
fi
