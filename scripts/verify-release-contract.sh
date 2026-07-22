#!/usr/bin/env bash
set -euo pipefail

script_repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
repo_root="$script_repo_root"
mode=""
profile=""
image_tag=""
output_dir=""
fixture_repo=""
release_cli=""

readonly contract_path="config/release/contract.v1.json"
readonly schema_path="config/release/contract.schema.json"
readonly -a required_surfaces=(
  build-identity readiness deployment http-runtime frontend-transport
  frontend-exposure protobuf migration-static migration-ledger migration-replay
)
readonly -a session_producers=(
  build-release-image readiness-deployment-harness http-runtime-session
  frontend-sidecar protobuf-session migration-session
)
readonly -a report_producers=(
  report-build-identity report-readiness report-deployment report-http-runtime
  report-frontend-transport report-frontend-exposure report-protobuf
  report-migration-static report-migration-ledger report-migration-replay
)

declare -A producer_counts=()
declare -A producer_statuses=()
declare -A surface_counts=()

fail() {
  local code="$1"
  printf '{"error":{"code":"%s"}}\n' "$code" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --stage-a | --clean-head | --fixtures)
      [[ -z "$mode" ]] || fail invalid_arguments
      mode="$1"
      shift
      ;;
    --profile)
      [[ $# -ge 2 ]] || fail invalid_arguments
      profile="$2"
      shift 2
      ;;
    --image-tag)
      [[ $# -ge 2 ]] || fail invalid_arguments
      image_tag="$2"
      shift 2
      ;;
    --output-dir)
      [[ $# -ge 2 ]] || fail invalid_arguments
      output_dir="$2"
      shift 2
      ;;
    --fixture-repo)
      [[ $# -ge 2 ]] || fail invalid_arguments
      fixture_repo="$2"
      shift 2
      ;;
    *)
      fail invalid_arguments
      ;;
  esac
done

[[ -n "$mode" ]] || fail invalid_arguments

configure_commands() {
  build_script="$repo_root/scripts/build-release-image.sh"
  readiness_script="$repo_root/scripts/verify-readiness-deployment-harness.sh"
  http_script="$repo_root/scripts/verify-http-runtime-contract.sh"
  protobuf_script="$repo_root/scripts/verify-protobuf-contract.sh"
  migration_script="$repo_root/scripts/verify-migration-replay.sh"
  frontend_extractor_cmd=(node "$repo_root/scripts/frontend_surface_sidecar.mjs")
  frontend_transport_cmd=(python3 "$repo_root/scripts/verify_frontend_surface.py" transport)
  frontend_exposure_cmd=(python3 "$repo_root/scripts/verify_frontend_surface.py" exposure)
  aggregate_validator_cmd=(python3 "$repo_root/scripts/verify_release_contract.py")
}

reset_observed_counts() {
  producer_counts=()
  producer_statuses=()
  surface_counts=()
}

invoke_producer() {
  local producer="$1"
  shift
  if [[ "${producer_counts[$producer]:-0}" -ne 0 ]]; then
    fail aggregate_producer_duplicate
  fi
  producer_counts[$producer]=1
  producer_statuses[$producer]="running"
  if "$@"; then
    producer_statuses[$producer]="pass"
    return 0
  fi
  producer_statuses[$producer]="fail"
  return 1
}

register_surface() {
  local surface="$1"
  local report_path="$2"
  [[ -s "$report_path" ]] || fail aggregate_report_missing
  if [[ "${surface_counts[$surface]:-0}" -ne 0 ]]; then
    fail aggregate_surface_duplicate
  fi
  surface_counts[$surface]=1
}

register_group_report() {
  local surface="$1"
  local source_path="$2"
  local destination_path="$3"
  local producer="report-$surface"
  [[ -s "$source_path" ]] || fail aggregate_report_missing
  if [[ "${producer_counts[$producer]:-0}" -ne 0 ]]; then
    fail aggregate_producer_duplicate
  fi
  cp -- "$source_path" "$destination_path"
  producer_counts[$producer]=1
  producer_statuses[$producer]="pass"
  register_surface "$surface" "$destination_path"
}

prepare_new_output_dir() {
  mkdir -p "$repo_root/.tmp"
  if [[ -z "$output_dir" ]]; then
    output_dir=$(mktemp -d "$repo_root/.tmp/release-contract.XXXXXX") || fail output_path_invalid
  else
    if [[ "$output_dir" != /* ]]; then
      output_dir="$repo_root/$output_dir"
    fi
    local parent
    parent=$(dirname "$output_dir")
    mkdir -p "$parent" || fail output_path_unwritable
    parent=$(cd "$parent" && pwd -P) || fail output_path_invalid
    output_dir="$parent/$(basename "$output_dir")"
    [[ ! -e "$output_dir" ]] || fail output_not_new
    mkdir "$output_dir" || fail output_path_unwritable
  fi
  output_dir=$(cd "$output_dir" && pwd -P) || fail output_path_invalid
  if [[ "$output_dir" != "$repo_root/.tmp/"* ]]; then
    fail output_path_invalid
  fi
}

build_release_cli() {
  if [[ -n "$release_cli" ]]; then
    [[ -x "$release_cli" ]] || fail release_cli_missing
    return
  fi
  mkdir -p "$output_dir/bin" "${GOCACHE:-$repo_root/.tmp/go-build}" "${GOMODCACHE:-$repo_root/.tmp/go-mod}"
  release_cli="$output_dir/bin/release-contract"
  (
    cd "$repo_root/src/server"
    GOCACHE="${GOCACHE:-$repo_root/.tmp/go-build}" GOMODCACHE="${GOMODCACHE:-$repo_root/.tmp/go-mod}" \
      go build -o "$release_cli" ./cmd/release-contract
  ) || fail release_cli_build_failed
}

write_artifact_bundle() {
  local identity_path="$1"
  local inspection_path="$2"
  local bundle_path="$3"
  local image_digest="$4"
  local bundle_base="$bundle_path.base"
  local bundle_digest
  jq -n \
    --slurpfile identity "$identity_path" \
    --arg imageTag "$image_tag" \
    --arg imageDigest "$image_digest" \
    --arg inspection "$(basename "$inspection_path")" \
    '{
      schemaVersion:"release-artifact-bundle/v1",
      releaseIdentity:($identity[0] + {deploymentProfile:"monolith"}),
      image:{tag:$imageTag,digest:$imageDigest},
      inspection:{buildIdentity:"build/build-identity.json",buildResult:"build-result.json",details:$inspection},
      bundleDigest:""
    }' >"$bundle_base" || fail artifact_bundle_invalid
  bundle_digest="sha256:$(jq -cS 'del(.bundleDigest)' "$bundle_base" | sha256sum | awk '{print $1}')"
  jq --arg digest "$bundle_digest" '.bundleDigest = $digest' "$bundle_base" >"$bundle_path.tmp" || fail artifact_bundle_invalid
  mv -f -- "$bundle_path.tmp" "$bundle_path"
  rm -f -- "$bundle_base"
  jq -e --arg digest "$bundle_digest" \
    '.schemaVersion == "release-artifact-bundle/v1" and .bundleDigest == $digest and .releaseIdentity.dirty == false' \
    "$bundle_path" >/dev/null || fail artifact_bundle_invalid
}

write_frontend_runtime_observations() {
  local identity_path="$1"
  local app_projection="$2"
  local server_catalog="$3"
  python3 - "$repo_root/$contract_path" "$identity_path" "$app_projection" "$server_catalog" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

contract_path, identity_path, app_path, catalog_path = map(Path, sys.argv[1:])
contract = json.loads(contract_path.read_text(encoding="utf-8"))
trusted = json.loads(identity_path.read_text(encoding="utf-8"))
identity = {
    "sourceTree": trusted["sourceTree"],
    "contractDigest": trusted["contractDigest"],
    "deploymentProfile": "monolith",
}
capabilities = []
for capability in sorted(contract["capabilities"], key=lambda row: row["id"]):
    disposition = capability["commitment"]
    if disposition == "excluded":
        continue
    enabled = disposition == "committed"
    capabilities.append({
        "capabilityId": capability["id"],
        "disposition": disposition,
        "availability": "enabled" if enabled else "disabled",
        "enabled": enabled,
    })
generation = 1
digest_payload = {"identity": identity, "generation": generation, "capabilities": capabilities}
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
    "capabilities": capabilities,
}
profile = next(
    row for row in contract["profiles"]
    if row["id"] == "monolith" and row["commitment"] == "committed"
)
bindings = {row["id"]: row for row in contract["catalogBindings"]}
server_catalog = {
    "schemaVersion": "frontend-server-catalog-observation/v1",
    "releaseIdentity": identity,
    "subjects": sorted((bindings[item] for item in profile["catalogBindingIds"]), key=lambda row: row["id"]),
}
app_path.write_text(json.dumps(app_projection, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
catalog_path.write_text(json.dumps(server_catalog, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
PY
}

assert_exact_report_directory() {
  local reports_dir="$1"
  local actual expected
  actual=$(find "$reports_dir" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort)
  expected=$(printf '%s.json\n' "${required_surfaces[@]}" | LC_ALL=C sort)
  [[ "$actual" == "$expected" ]] || fail aggregate_stale_report_directory
}

write_producer_status() {
  local status_path="$1"
  local rebuild_count="$2"
  local events_path="$output_dir/producer-events.tsv"
  local surfaces_path="$output_dir/surface-events.tsv"
  local producer surface
  : >"$events_path"
  : >"$surfaces_path"
  for producer in "${session_producers[@]}" "${report_producers[@]}"; do
    printf '%s\t%s\t%s\n' "$producer" "${producer_counts[$producer]:-0}" "${producer_statuses[$producer]:-missing}" >>"$events_path"
  done
  for surface in "${required_surfaces[@]}"; do
    printf '%s\t%s\n' "$surface" "${surface_counts[$surface]:-0}" >>"$surfaces_path"
  done
  python3 - "$events_path" "$surfaces_path" "$status_path" "$rebuild_count" <<'PY'
import json
from pathlib import Path
import sys

events_path, surfaces_path, output_path = map(Path, sys.argv[1:4])
rebuild_count = int(sys.argv[4])
producer_counts = {}
producer_statuses = {}
for line in events_path.read_text(encoding="utf-8").splitlines():
    name, count, status = line.split("\t")
    producer_counts[name] = int(count)
    producer_statuses[name] = status
surface_counts = {}
for line in surfaces_path.read_text(encoding="utf-8").splitlines():
    name, count = line.split("\t")
    surface_counts[name] = int(count)
value = {
    "schemaVersion": "release-contract-producer-status/v1",
    "surfaceExecutions": surface_counts,
    "producerExecutions": producer_counts,
    "producerStatuses": producer_statuses,
    "rebuildCount": rebuild_count,
    "migrationStaticPreRunCount": 0,
}
temporary = output_path.with_suffix(output_path.suffix + ".tmp")
temporary.write_text(json.dumps(value, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
temporary.replace(output_path)
PY
}

run_release_session() {
  reset_observed_counts
  [[ "$profile" == "monolith" ]] || fail profile_not_committed
  [[ -z "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]] || fail source_worktree_dirty
  for command_name in git go docker jq sha256sum node python3; do
    command -v "$command_name" >/dev/null 2>&1 || fail build_tool_missing
  done
  docker info >/dev/null 2>&1 || fail docker_unavailable
  prepare_new_output_dir
  build_release_cli

  local reports_dir="$output_dir/reports"
  local identity_path="$output_dir/trusted-identity.json"
  local inspection_path="$output_dir/build-inspection.json"
  local bundle_path="$output_dir/artifact-bundle.json"
  local producer_status_path="$output_dir/producer-status.json"
  local aggregate_path="$output_dir/release-contract-aggregate.json"
  local release_commit source_tree contract_digest identity_json image_digest
  mkdir "$reports_dir"

  "$release_cli" validate --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" >/dev/null || fail contract_invalid
  identity_json=$("$release_cli" identity --repo "$repo_root" --contract "$contract_path" --schema "$schema_path") || fail build_identity_mismatch
  release_commit=$(jq -er '.releaseCommit' <<<"$identity_json") || fail build_identity_mismatch
  source_tree=$(jq -er '.sourceTree' <<<"$identity_json") || fail build_identity_mismatch
  contract_digest=$(jq -er '.contractDigest' <<<"$identity_json") || fail build_identity_mismatch
  jq -e --arg commit "$(git -C "$repo_root" rev-parse 'HEAD^{commit}')" --arg tree "$(git -C "$repo_root" rev-parse 'HEAD^{tree}')" \
    '.schemaVersion == "build-identity/v1" and .releaseCommit == $commit and .sourceTree == $tree and
     (.contractDigest | startswith("sha256:"))' <<<"$identity_json" >/dev/null || fail build_identity_mismatch
  jq -e '.dirty == false and .evidenceClass == "repository-local"' <<<"$identity_json" >/dev/null || fail build_identity_mismatch
  printf '%s\n' "$identity_json" >"$identity_path"
  [[ -n "$image_tag" ]] || image_tag="oblivious-release:${release_commit}"

  invoke_producer build-release-image \
    env -u RELEASE_COMMIT -u SOURCE_TREE -u CONTRACT_DIGEST -u BUILD_DIRTY -u EVIDENCE_CLASS \
    bash "$build_script" --image-tag "$image_tag" --contract "$contract_path" --schema "$schema_path" \
      --output-dir "$output_dir/build" >"$output_dir/build-result.json" || fail image_build_failed
  image_digest=$(docker image inspect --format '{{.Id}}' "$image_tag" 2>/dev/null) || fail image_missing
  [[ "$image_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || fail image_digest_mismatch

  local grpc_digest migrate_digest server_digest
  grpc_digest="sha256:$(sha256sum "$output_dir/build/oblivious-grpc-smoke" | awk '{print $1}')"
  migrate_digest="sha256:$(sha256sum "$output_dir/build/oblivious-migrate" | awk '{print $1}')"
  server_digest="sha256:$(sha256sum "$output_dir/build/oblivious-server" | awk '{print $1}')"
  jq -n \
    --arg grpc "$grpc_digest" --arg migrate "$migrate_digest" --arg server "$server_digest" \
    --arg image "$image_tag" --arg imageDigest "$image_digest" --arg contract "$contract_digest" \
    '{
      binaries:[
        {name:"grpc-smoke",path:"/usr/local/bin/oblivious-grpc-smoke",digest:$grpc,matches:true},
        {name:"migrate",path:"/usr/local/bin/oblivious-migrate",digest:$migrate,matches:true},
        {name:"server",path:"/usr/local/bin/oblivious-server",digest:$server,matches:true}
      ],
      oci:{image:$image,digest:$imageDigest,matches:true},
      packagedContract:{path:"/app/config/release/contract.v1.json",digest:$contract,matches:true},
      residualRisks:["external target not inspected","supply chain attestations deferred"]
    }' >"$inspection_path"
  write_artifact_bundle "$identity_path" "$inspection_path" "$bundle_path" "$image_digest"

  local build_report="$reports_dir/build-identity.json"
  invoke_producer report-build-identity "$release_cli" report-build-identity \
    --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" --profile monolith \
    --inspection "$inspection_path" --output "$build_report" >"$output_dir/build-report-command.json" || fail build_identity_report_failed
  register_surface build-identity "$build_report"

  local readiness_dir="$output_dir/readiness-deployment"
  invoke_producer readiness-deployment-harness bash "$readiness_script" \
    --mode aggregate-consume --repo-root "$repo_root" --output-dir "$readiness_dir" \
    --artifact-bundle "$bundle_path" --image-tag "$image_tag" --image-digest "$image_digest" \
    >"$output_dir/readiness-deployment-command.log" || fail readiness_deployment_failed
  local rebuild_count
  rebuild_count=$(jq -er '.buildInvocationCount' "$readiness_dir/harness-result.json") || fail readiness_deployment_failed
  [[ "$rebuild_count" -eq 0 ]] || fail aggregate_rebuild_detected
  jq -e --arg tag "$image_tag" --arg digest "$image_digest" \
    '.mode == "aggregate-consume" and .imageTag == $tag and .imageDigest == $digest and
     .result == "pass" and (.skippedChecks | length) == 0' "$readiness_dir/harness-result.json" >/dev/null || fail artifact_bundle_mismatch
  register_group_report readiness "$readiness_dir/readiness-report.json" "$reports_dir/readiness.json"
  register_group_report deployment "$readiness_dir/deployment-report.json" "$reports_dir/deployment.json"

  local http_report="$reports_dir/http-runtime.json"
  invoke_producer http-runtime-session bash "$http_script" --clean-head --output "$http_report" \
    >"$output_dir/http-runtime-command.log" || fail http_runtime_failed
  producer_counts[report-http-runtime]=1
  producer_statuses[report-http-runtime]="pass"
  register_surface http-runtime "$http_report"

  local sidecar="$output_dir/frontend-sidecar.json"
  local generated="$repo_root/src/web/src/generated/operation-contracts.generated.ts"
  invoke_producer frontend-sidecar "${frontend_extractor_cmd[@]}" \
    --root "$repo_root/src/web/src" --tsconfig "$repo_root/src/web/tsconfig.json" \
    --generated-file "$generated" --output "$sidecar" >"$output_dir/frontend-sidecar-command.log" || fail frontend_sidecar_failed
  [[ -s "$sidecar" ]] || fail frontend_sidecar_failed

  local transport_observation="$output_dir/frontend-transport-observation.json"
  local exposure_observation="$output_dir/frontend-exposure-observation.json"
  local app_projection="$output_dir/frontend-app-projection.json"
  local server_catalog="$output_dir/frontend-server-catalog.json"
  write_frontend_runtime_observations "$identity_path" "$app_projection" "$server_catalog" || fail frontend_exposure_failed
  "${frontend_transport_cmd[@]}" --sidecar "$sidecar" --manifest "$repo_root/docs/api/route-surface-manifest.json" \
    --output "$transport_observation" >"$output_dir/frontend-transport-projection.log" || fail frontend_transport_failed
  "${frontend_exposure_cmd[@]}" --sidecar "$sidecar" --contract "$repo_root/$contract_path" \
    --app-projection "$app_projection" --server-catalog "$server_catalog" --output "$exposure_observation" \
    >"$output_dir/frontend-exposure-projection.log" || fail frontend_exposure_failed
  local transport_digest exposure_digest
  transport_digest=$(jq -er '.sidecarDigest' "$transport_observation") || fail frontend_transport_failed
  exposure_digest=$(jq -er '.sidecarDigest' "$exposure_observation") || fail frontend_exposure_failed
  [[ "$transport_digest" == "$exposure_digest" ]] || fail frontend_sidecar_digest_splice

  local transport_report="$reports_dir/frontend-transport.json"
  local exposure_report="$reports_dir/frontend-exposure.json"
  invoke_producer report-frontend-transport "$release_cli" report-frontend-transport \
    --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" --profile monolith \
    --observation "$transport_observation" --sidecar-digest "$transport_digest" --output "$transport_report" \
    >"$output_dir/frontend-transport-report-command.json" || fail frontend_transport_report_failed
  register_surface frontend-transport "$transport_report"
  invoke_producer report-frontend-exposure "$release_cli" report-frontend-exposure \
    --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" --profile monolith \
    --observation "$exposure_observation" --sidecar-digest "$exposure_digest" --output "$exposure_report" \
    >"$output_dir/frontend-exposure-report-command.json" || fail frontend_exposure_report_failed
  register_surface frontend-exposure "$exposure_report"

  local protobuf_observation="$output_dir/protobuf-observation.json"
  invoke_producer protobuf-session bash "$protobuf_script" --observation-out "$protobuf_observation" \
    >"$output_dir/protobuf-command.log" || fail protobuf_failed
  local protobuf_report="$reports_dir/protobuf.json"
  invoke_producer report-protobuf "$release_cli" report-protobuf \
    --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" --profile monolith \
    --observation "$protobuf_observation" --output "$protobuf_report" \
    >"$output_dir/protobuf-report-command.json" || fail protobuf_report_failed
  register_surface protobuf "$protobuf_report"

  local migration_dir="$output_dir/migration-session"
  invoke_producer migration-session bash "$migration_script" session --output-dir "$migration_dir" \
    >"$output_dir/migration-session-command.log" || fail migration_session_failed
  register_group_report migration-static "$migration_dir/migration-static.json" "$reports_dir/migration-static.json"
  register_group_report migration-ledger "$migration_dir/migration-ledger.json" "$reports_dir/migration-ledger.json"
  register_group_report migration-replay "$migration_dir/migration-replay.json" "$reports_dir/migration-replay.json"

  assert_exact_report_directory "$reports_dir"
  write_producer_status "$producer_status_path" "$rebuild_count"
  local -a report_arguments=()
  local surface
  for surface in "${required_surfaces[@]}"; do
    report_arguments+=(--report "$reports_dir/$surface.json")
  done
  "${aggregate_validator_cmd[@]}" \
    --repo "$repo_root" --report-dir "$reports_dir" "${report_arguments[@]}" \
    --identity "$identity_path" --producer-status "$producer_status_path" \
    --verifier-bin "$release_cli" --profile monolith --output "$aggregate_path" \
    >"$output_dir/aggregate-validator-command.json" || fail aggregate_validation_failed
  [[ -s "$aggregate_path" ]] || fail aggregate_output_missing

  local relative_output="${output_dir#$repo_root/}"
  jq -n --arg output "$relative_output" --arg commit "$release_commit" --arg tree "$source_tree" \
    --arg contract "$contract_digest" --arg imageDigest "$image_digest" \
    '{schemaVersion:"release-contract-verification/v1",stage:"B",result:"pass",evidenceClass:"repository-local",
      environment:"clean-head",releaseCommit:$commit,sourceTree:$tree,contractDigest:$contract,
      deploymentProfile:"monolith",imageDigest:$imageDigest,outputDirectory:$output,
      migrationState:"fresh-first-apply-second-no-op",surfaceCount:10,skippedChecks:[],
      residualRisks:["external target not inspected","target Kubernetes and commercial rails deferred"],
      claim:"RELS-02 repository-local E1/E2 contribution only"}'
}

run_stage_a() {
  [[ -z "$profile" && -z "$image_tag" ]] || fail invalid_arguments
  if [[ -n "$fixture_repo" ]]; then
    git -C "$fixture_repo" rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail invalid_arguments
  fi
  [[ -n "$output_dir" ]] || output_dir="$repo_root/.tmp/release-contract-stage-a"
  mkdir -p "$output_dir"
  output_dir=$(cd "$output_dir" && pwd -P)
  if [[ "$output_dir" == "$repo_root" || ( "$output_dir" == "$repo_root/"* && "$output_dir" != "$repo_root/.tmp" && "$output_dir" != "$repo_root/.tmp/"* ) ]]; then
    fail output_path_invalid
  fi
  if [[ -n "$fixture_repo" ]]; then
    fixture_repo=$(cd "$fixture_repo" && pwd -P)
    [[ -z "$(git -C "$fixture_repo" status --porcelain=v1 --untracked-files=all)" ]] || fail source_worktree_dirty
    (
      cd "$repo_root/src/server"
      go run ./cmd/release-contract identity --repo "$fixture_repo" --contract "$contract_path" --schema "$schema_path"
    ) >"$output_dir/fixture-identity.json"
  fi
  bash "$repo_root/scripts/verify-release-contract-fixtures.sh"
  git -C "$repo_root" diff --check
  printf '{"schemaVersion":"release-contract-verification/v1","stage":"A","result":"pass","evidenceClass":"repository-local","targetEvidence":false}\n'
}

write_fixture_shims() {
  local fixture_root="$1"
  local fixture_checkout="$2"
  mkdir -p "$fixture_root/bin" "$fixture_checkout/scripts" "$fixture_checkout/bin" \
    "$fixture_checkout/config/release" "$fixture_checkout/src/web/src/generated" "$fixture_checkout/docs/api"
  printf '%s\n' '{"capabilities":[],"profiles":[{"id":"monolith","commitment":"committed","catalogBindingIds":[]}],"catalogBindings":[]}' >"$fixture_checkout/config/release/contract.v1.json"
  printf '{}\n' >"$fixture_checkout/config/release/contract.schema.json"
  printf '{}\n' >"$fixture_checkout/docs/api/route-surface-manifest.json"
  printf '{}\n' >"$fixture_checkout/src/web/tsconfig.json"
  printf 'export {};\n' >"$fixture_checkout/src/web/src/generated/operation-contracts.generated.ts"
  printf '.tmp/\n' >"$fixture_checkout/.gitignore"

  cat >"$fixture_root/bin/docker" <<'SHIM'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  info) exit 0 ;;
  image)
    [[ "${2:-}" == "inspect" ]] || exit 1
    printf 'sha256:%064d\n' 1
    ;;
  *) exit 0 ;;
esac
SHIM

  cat >"$fixture_checkout/bin/release-contract" <<'SHIM'
#!/usr/bin/env bash
set -euo pipefail
command_name="${1:-}"
shift || true
option() {
  local wanted="$1"
  shift
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "$wanted" ]]; then printf '%s' "$2"; return 0; fi
    shift
  done
  return 1
}
log() { printf '%s\n' "$1" >>"$OBLIVIOUS_FIXTURE_CALL_LOG"; }
case "$command_name" in
  validate) printf '{"result":"pass"}\n' ;;
  identity)
    repo=$(option --repo "$@")
    commit=$(git -C "$repo" rev-parse 'HEAD^{commit}')
    tree=$(git -C "$repo" rev-parse 'HEAD^{tree}')
    printf '{"schemaVersion":"build-identity/v1","releaseCommit":"%s","sourceTree":"%s","contractDigest":"sha256:%064d","dirty":false,"evidenceClass":"repository-local"}\n' "$commit" "$tree" 2
    ;;
  verify-report) printf '{"schemaVersion":"surface-report/v1","surface":"fixture","result":"pass","evidenceClass":"repository-local"}\n' ;;
  report-*)
    log "$command_name"
    output=$(option --output "$@")
    surface="${command_name#report-}"
    printf '{"schemaVersion":"surface-report/v1","surface":"%s"}\n' "$surface" >"$output"
    printf '{"result":"pass","surface":"%s"}\n' "$surface"
    ;;
  *) exit 2 ;;
esac
SHIM

  cat >"$fixture_root/producer-shim" <<'SHIM'
#!/usr/bin/env bash
set -euo pipefail
id=$(basename "$0")
option() {
  local wanted="$1"
  shift
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "$wanted" ]]; then printf '%s' "$2"; return 0; fi
    shift
  done
  return 1
}
log() { printf '%s\n' "$1" >>"$OBLIVIOUS_FIXTURE_CALL_LOG"; }
log "$id"
case "$id" in
  build-release-image.sh)
    [[ "${OBLIVIOUS_FIXTURE_MUTATION:-}" != "producer-failure" ]] || exit 19
    output=$(option --output-dir "$@")
    mkdir -p "$output"
    printf binary >"$output/oblivious-grpc-smoke"
    printf binary >"$output/oblivious-migrate"
    printf binary >"$output/oblivious-server"
    printf '{}\n' >"$output/build-identity.json"
    printf '{"result":"pass"}\n'
    ;;
  verify-readiness-deployment-harness.sh)
    output=$(option --output-dir "$@")
    tag=$(option --image-tag "$@")
    digest=$(option --image-digest "$@")
    bundle=$(option --artifact-bundle "$@")
    mkdir -p "$output"
    [[ "${OBLIVIOUS_FIXTURE_MUTATION:-}" != "bundle-mismatch" ]] || exit 20
    log report-readiness
    log report-deployment
    printf '{"surface":"readiness"}\n' >"$output/readiness-report.json"
    printf '{"surface":"deployment"}\n' >"$output/deployment-report.json"
    count=0
    [[ "${OBLIVIOUS_FIXTURE_MUTATION:-}" != "rebuild" ]] || count=1
    printf '{"mode":"aggregate-consume","result":"pass","imageTag":"%s","imageDigest":"%s","buildInvocationCount":%s,"skippedChecks":[]}\n' "$tag" "$digest" "$count" >"$output/harness-result.json"
    [[ -s "$bundle" ]]
    ;;
  verify-http-runtime-contract.sh)
    log report-http-runtime
    output=$(option --output "$@")
    [[ "${OBLIVIOUS_FIXTURE_MUTATION:-}" == "missing-report" ]] || printf '{"surface":"http-runtime"}\n' >"$output"
    ;;
  frontend_surface_sidecar.mjs)
    output=$(option --output "$@")
    printf '{"schemaVersion":"frontend-surface-sidecar/v1"}\n' >"$output"
    [[ "${OBLIVIOUS_FIXTURE_MUTATION:-}" != "duplicate" ]] || log frontend_surface_sidecar.mjs
    ;;
  frontend-transport-projection)
    output=$(option --output "$@")
    printf '{"sidecarDigest":"sha256:%064d"}\n' 3 >"$output"
    ;;
  frontend-exposure-projection)
    output=$(option --output "$@")
    printf '{"sidecarDigest":"sha256:%064d"}\n' 3 >"$output"
    ;;
  verify-protobuf-contract.sh)
    output=$(option --observation-out "$@")
    printf '{}\n' >"$output"
    ;;
  verify-migration-replay.sh)
    output=$(option --output-dir "$@")
    mkdir -p "$output"
    log report-migration-static
    log report-migration-ledger
    log report-migration-replay
    printf '{"surface":"migration-static"}\n' >"$output/migration-static.json"
    printf '{"surface":"migration-ledger"}\n' >"$output/migration-ledger.json"
    printf '{"surface":"migration-replay"}\n' >"$output/migration-replay.json"
    [[ "${OBLIVIOUS_FIXTURE_MUTATION:-}" != "cleanup-failure" ]] || exit 21
    ;;
  verify-release-contract-aggregate)
    output=$(option --output "$@")
    printf '{"schemaVersion":"release-contract-aggregate/v1"}\n' >"$output"
    ;;
  *) exit 2 ;;
esac
SHIM

  chmod +x "$fixture_root/bin/docker" "$fixture_checkout/bin/release-contract" "$fixture_root/producer-shim"
  local name
  for name in build-release-image.sh verify-readiness-deployment-harness.sh verify-http-runtime-contract.sh \
    frontend_surface_sidecar.mjs frontend-transport-projection frontend-exposure-projection \
    verify-protobuf-contract.sh verify-migration-replay.sh verify-release-contract-aggregate; do
    ln -s "$fixture_root/producer-shim" "$fixture_checkout/scripts/$name"
  done
}

assert_fixture_call_graph() {
  local log_path="$1"
  python3 - "$log_path" <<'PY'
from collections import Counter
from pathlib import Path
import sys

observed = Counter(Path(sys.argv[1]).read_text(encoding="utf-8").splitlines())
expected = {
    "build-release-image.sh",
    "report-build-identity",
    "verify-readiness-deployment-harness.sh",
    "report-readiness",
    "report-deployment",
    "verify-http-runtime-contract.sh",
    "report-http-runtime",
    "frontend_surface_sidecar.mjs",
    "frontend-transport-projection",
    "frontend-exposure-projection",
    "report-frontend-transport",
    "report-frontend-exposure",
    "verify-protobuf-contract.sh",
    "report-protobuf",
    "verify-migration-replay.sh",
    "report-migration-static",
    "report-migration-ledger",
    "report-migration-replay",
    "verify-release-contract-aggregate",
}
if set(observed) != expected or any(observed[name] != 1 for name in expected):
    raise SystemExit("release_call_graph_invalid: " + repr(dict(sorted(observed.items()))))
PY
}

run_fixture_session_case() (
  set -euo pipefail
  local fixture_checkout="$1"
  local fixture_root="$2"
  local mutation="$3"
  local case_name="${mutation:-baseline}"
  repo_root="$fixture_checkout"
  profile="monolith"
  image_tag="oblivious-fixture:immutable"
  output_dir="$fixture_checkout/.tmp/$case_name"
  release_cli="$fixture_checkout/bin/release-contract"
  export PATH="$fixture_root/bin:$PATH"
  export OBLIVIOUS_FIXTURE_CALL_LOG="$fixture_root/$case_name.calls"
  export OBLIVIOUS_FIXTURE_MUTATION="$mutation"
  : >"$OBLIVIOUS_FIXTURE_CALL_LOG"
  configure_commands
  frontend_extractor_cmd=("$fixture_checkout/scripts/frontend_surface_sidecar.mjs")
  frontend_transport_cmd=("$fixture_checkout/scripts/frontend-transport-projection")
  frontend_exposure_cmd=("$fixture_checkout/scripts/frontend-exposure-projection")
  aggregate_validator_cmd=("$fixture_checkout/scripts/verify-release-contract-aggregate")

  if [[ "$mutation" == "stale-output" ]]; then
    mkdir -p "$output_dir"
    printf stale >"$output_dir/stale.json"
  elif [[ "$mutation" == "unwritable-output" ]]; then
    printf blocked >"$fixture_checkout/.tmp/blocked-parent"
    output_dir="$fixture_checkout/.tmp/blocked-parent/output"
  fi

  run_release_session >"$fixture_root/$case_name.result.json"
  assert_fixture_call_graph "$OBLIVIOUS_FIXTURE_CALL_LOG"
)

run_call_graph_fixtures() {
  [[ -z "$profile" && -z "$image_tag" && -z "$output_dir" && -z "$fixture_repo" ]] || fail invalid_arguments
  local fixture_root fixture_checkout mutation rejected
  fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-release-call-graph.XXXXXX")
  fixture_checkout="$fixture_root/repository"
  mkdir -p "$fixture_checkout"
  trap 'rm -rf -- "$fixture_root"' RETURN
  write_fixture_shims "$fixture_root" "$fixture_checkout"
  git -C "$fixture_checkout" init -q
  git -C "$fixture_checkout" add -- .
  git -C "$fixture_checkout" -c user.name=release-call-graph -c user.email=release-call-graph.invalid commit -q -m snapshot

  run_fixture_session_case "$fixture_checkout" "$fixture_root" ""
  rejected=0
  for mutation in duplicate producer-failure missing-report rebuild bundle-mismatch cleanup-failure stale-output unwritable-output; do
    if run_fixture_session_case "$fixture_checkout" "$fixture_root" "$mutation" >/dev/null 2>&1; then
      fail call_graph_fixture_false_pass
    fi
    rejected=$((rejected + 1))
  done
  [[ "$rejected" -eq 8 ]] || fail call_graph_fixture_count_invalid
  printf '[release-contract-call-graph] Stage A fixed DAG and %s rejected mutations verified\n' "$rejected"
}

case "$mode" in
  --fixtures)
    run_call_graph_fixtures
    ;;
  --stage-a)
    run_stage_a
    ;;
  --clean-head)
    [[ -z "$fixture_repo" ]] || fail invalid_arguments
    [[ -n "$profile" ]] || fail profile_required
    configure_commands
    run_release_session
    ;;
  *)
    fail invalid_arguments
    ;;
esac
