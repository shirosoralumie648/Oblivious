#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
mode=""
profile=""
image_tag=""
output_dir="$repo_root/.tmp/release-contract"
fixture_repo=""

fail() {
  local code="$1"
  printf '{"error":{"code":"%s"}}\n' "$code" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --stage-a | --clean-head)
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

if [[ "$mode" == "--stage-a" ]]; then
  [[ -z "$profile" && -z "$image_tag" ]] || fail invalid_arguments
  if [[ -n "$fixture_repo" && ! -d "$fixture_repo/.git" ]]; then
    fail invalid_arguments
  fi
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
      go run ./cmd/release-contract identity \
        --repo "$fixture_repo" \
        --contract config/release/contract.v1.json \
        --schema config/release/contract.schema.json
    ) >"$output_dir/fixture-identity.json"
  fi
  TMPDIR="$output_dir" bash "$repo_root/scripts/verify-release-contract-fixtures.sh"
  git -C "$repo_root" diff --check
  printf '{"schemaVersion":"release-contract-verification/v1","stage":"A","result":"pass","evidenceClass":"repository-local","targetEvidence":false}\n'
  exit 0
fi

[[ "$mode" == "--clean-head" ]] || fail invalid_arguments
[[ -z "$fixture_repo" ]] || fail invalid_arguments
[[ -n "$profile" ]] || fail profile_required
[[ "$profile" == "monolith" ]] || fail profile_not_committed

if [[ -n "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]]; then
  fail source_worktree_dirty
fi
command -v docker >/dev/null 2>&1 || fail build_tool_missing
docker info >/dev/null 2>&1 || fail docker_unavailable
command -v jq >/dev/null 2>&1 || fail build_tool_missing
command -v sha256sum >/dev/null 2>&1 || fail build_tool_missing

mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd -P)
if [[ "$output_dir" != "$repo_root/.tmp" && "$output_dir" != "$repo_root/.tmp/"* ]]; then
  fail output_path_invalid
fi

contract_path="config/release/contract.v1.json"
schema_path="config/release/contract.schema.json"
cli=(go run ./cmd/release-contract)

(
  cd "$repo_root/src/server"
  "${cli[@]}" validate --repo "$repo_root" --contract "$contract_path" --schema "$schema_path" >/dev/null
)

release_commit=$(git -C "$repo_root" rev-parse 'HEAD^{commit}')
source_tree=$(git -C "$repo_root" rev-parse 'HEAD^{tree}')
identity_json=$(
  cd "$repo_root/src/server"
  "${cli[@]}" identity --repo "$repo_root" --contract "$contract_path" --schema "$schema_path"
)
contract_digest=$(jq -er '.contractDigest' <<<"$identity_json") || fail build_identity_mismatch
jq -e --arg commit "$release_commit" --arg tree "$source_tree" \
  '.releaseCommit == $commit and .sourceTree == $tree and .dirty == false and .evidenceClass == "repository-local"' \
  <<<"$identity_json" >/dev/null || fail build_identity_mismatch

if [[ -z "$image_tag" ]]; then
  image_tag="oblivious-release:${release_commit}"
fi

env -u RELEASE_COMMIT -u SOURCE_TREE -u CONTRACT_DIGEST -u BUILD_DIRTY -u EVIDENCE_CLASS \
  bash "$repo_root/scripts/build-release-image.sh" \
  --image-tag "$image_tag" \
  --contract "$contract_path" \
  --schema "$schema_path" \
  --output-dir "$output_dir/build" >"$output_dir/build-result.json"

binary_digest() {
  local name="$1"
  local path="$output_dir/build/oblivious-$name"
  [[ -f "$path" ]] || fail build_identity_missing
  printf 'sha256:%s' "$(sha256sum "$path" | awk '{print $1}')"
}

grpc_digest=$(binary_digest grpc-smoke)
migrate_digest=$(binary_digest migrate)
server_digest=$(binary_digest server)
image_digest=$(docker image inspect --format '{{.Id}}' "$image_tag")
[[ "$image_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || fail build_identity_mismatch

inspection_path="$output_dir/build-inspection.json"
jq -n \
  --arg grpc "$grpc_digest" \
  --arg migrate "$migrate_digest" \
  --arg server "$server_digest" \
  --arg image "$image_tag" \
  --arg imageDigest "$image_digest" \
  --arg contract "$contract_digest" \
  '{
    binaries: [
      {name:"grpc-smoke",path:"/usr/local/bin/oblivious-grpc-smoke",digest:$grpc,matches:true},
      {name:"migrate",path:"/usr/local/bin/oblivious-migrate",digest:$migrate,matches:true},
      {name:"server",path:"/usr/local/bin/oblivious-server",digest:$server,matches:true}
    ],
    oci:{image:$image,digest:$imageDigest,matches:true},
    packagedContract:{path:"/app/config/release/contract.v1.json",digest:$contract,matches:true},
    residualRisks:["external target not inspected","supply chain attestations deferred"]
  }' >"$inspection_path"

report_path="$output_dir/build-identity-report.json"
report_result=$(
  cd "$repo_root/src/server"
  "${cli[@]}" report-build-identity \
    --repo "$repo_root" \
    --contract "$contract_path" \
    --schema "$schema_path" \
    --profile "$profile" \
    --inspection "$inspection_path" \
    --output "$report_path"
)
jq -e '.result == "pass" and .deploymentProfile == "monolith" and .evidenceClass == "repository-local"' <<<"$report_result" >/dev/null || fail surface_schema_invalid

verify_result=$(
  cd "$repo_root/src/server"
  "${cli[@]}" verify-report --input "$report_path"
)
jq -e '.result == "pass" and .surface == "build-identity" and .evidenceClass == "repository-local"' <<<"$verify_result" >/dev/null || fail surface_schema_invalid
jq -e '.outcome.result == "pass" and (.outcome.skippedChecks | length) == 0 and .releaseIdentity.deploymentProfile == "monolith"' "$report_path" >/dev/null || fail skipped_committed_check

jq -n \
  --arg commit "$release_commit" \
  --arg tree "$source_tree" \
  --arg digest "$contract_digest" \
  --arg profile "$profile" \
  --arg image "$image_tag" \
  --arg report "$report_path" \
  '{
    schemaVersion:"release-contract-verification/v1",stage:"B",result:"pass",
    evidenceClass:"repository-local",environment:"clean-head",
    releaseCommit:$commit,sourceTree:$tree,contractDigest:$digest,deploymentProfile:$profile,
    imageTag:$image,reportPath:$report,migrationState:"not-applicable",skippedChecks:[],
    residualRisks:["external target not inspected","supply chain attestations deferred"],
    claim:"RELS-01 foundation contribution only"
  }'
