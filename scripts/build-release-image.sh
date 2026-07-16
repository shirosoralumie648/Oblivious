#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
authority_root="$source_root"
image_tag=""
contract_path="config/release/contract.v1.json"
schema_path="config/release/contract.schema.json"
output_dir="$source_root/.tmp/release-build"
fixture_mode=false
container_id=""

fail() {
  local code="$1"
  printf '{"error":{"code":"%s"}}\n' "$code" >&2
  exit 1
}

cleanup() {
  if [[ -n "$container_id" ]]; then
    docker rm -f "$container_id" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image-tag)
      [[ $# -ge 2 ]] || fail invalid_arguments
      image_tag="$2"
      shift 2
      ;;
    --contract)
      [[ $# -ge 2 ]] || fail invalid_arguments
      contract_path="$2"
      shift 2
      ;;
    --schema)
      [[ $# -ge 2 ]] || fail invalid_arguments
      schema_path="$2"
      shift 2
      ;;
    --output-dir)
      [[ $# -ge 2 ]] || fail invalid_arguments
      output_dir="$2"
      shift 2
      ;;
    --fixture-repo)
      [[ $# -ge 2 ]] || fail invalid_arguments
      authority_root="$2"
      fixture_mode=true
      shift 2
      ;;
    *)
      fail invalid_arguments
      ;;
  esac
done

[[ -n "$image_tag" && -d "$authority_root/.git" ]] || fail invalid_arguments
for command_name in git go docker jq; do
  command -v "$command_name" >/dev/null 2>&1 || fail build_tool_missing
done

authority_root=$(cd "$authority_root" && pwd -P)
mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd -P)
if [[ "$fixture_mode" != true && "$output_dir" != "$source_root/.tmp" && "$output_dir" != "$source_root/.tmp/"* ]]; then
  fail output_path_invalid
fi

if [[ -n "$(git -C "$authority_root" status --porcelain=v1 --untracked-files=all)" ]]; then
  fail source_worktree_dirty
fi

identity_json=""
identity_error="$output_dir/identity-error.json"
if ! identity_json=$(cd "$source_root/src/server" && go run ./cmd/release-contract identity \
  --repo "$authority_root" --contract "$contract_path" --schema "$schema_path" 2>"$identity_error"); then
  cat "$identity_error" >&2
  fail build_identity_mismatch
fi
rm -f "$identity_error"
printf '%s\n' "$identity_json" >"$output_dir/build-identity.json"

schema_version=$(jq -er '.schemaVersion' <<<"$identity_json") || fail build_identity_mismatch
release_commit=$(jq -er '.releaseCommit' <<<"$identity_json") || fail build_identity_mismatch
source_tree=$(jq -er '.sourceTree' <<<"$identity_json") || fail build_identity_mismatch
contract_digest=$(jq -er '.contractDigest' <<<"$identity_json") || fail build_identity_mismatch
dirty=$(jq -r '.dirty' <<<"$identity_json") || fail build_identity_mismatch
evidence_class=$(jq -er '.evidenceClass' <<<"$identity_json") || fail build_identity_mismatch
[[ "$schema_version" == "build-identity/v1" && "$release_commit" =~ ^[0-9a-f]{40}$ && "$source_tree" =~ ^[0-9a-f]{40}$ && "$contract_digest" =~ ^sha256:[0-9a-f]{64}$ && "$dirty" == "false" && "$evidence_class" == "repository-local" ]] || fail build_identity_mismatch

linker_flags="-s -w -X oblivious/server/internal/buildinfo.linkedReleaseCommit=$release_commit -X oblivious/server/internal/buildinfo.linkedSourceTree=$source_tree -X oblivious/server/internal/buildinfo.linkedContractDigest=$contract_digest -X oblivious/server/internal/buildinfo.linkedDirty=false -X oblivious/server/internal/buildinfo.linkedEvidenceClass=repository-local"
for binary in server migrate grpc-smoke; do
  if ! (cd "$source_root/src/server" && CGO_ENABLED=0 go build -trimpath -ldflags="$linker_flags" -o "$output_dir/oblivious-$binary" "./cmd/$binary"); then
    fail build_failed
  fi
done

if ! docker build --file "$source_root/Dockerfile.server" --tag "$image_tag" \
  --build-arg "RELEASE_COMMIT=$release_commit" \
  --build-arg "SOURCE_TREE=$source_tree" \
  --build-arg "CONTRACT_DIGEST=$contract_digest" \
  --build-arg "BUILD_DIRTY=false" \
  --build-arg "EVIDENCE_CLASS=repository-local" \
  "$source_root" >/dev/null; then
  fail build_failed
fi

check_identity() {
  local component="$1"
  local observed="$2"
  jq -e \
    --arg commit "$release_commit" \
    --arg tree "$source_tree" \
    --arg digest "$contract_digest" \
    '.schemaVersion == "build-identity/v1" and .releaseCommit == $commit and .sourceTree == $tree and .contractDigest == $digest and .dirty == false and .evidenceClass == "repository-local"' \
    <<<"$observed" >/dev/null || {
      printf '[build-release-image] identity mismatch: %s\n' "$component" >&2
      fail build_identity_mismatch
    }
}

for binary in server migrate grpc-smoke; do
  observed=""
  if ! observed=$(docker run --rm --entrypoint "/usr/local/bin/oblivious-$binary" "$image_tag" --inspect-build-identity); then
    fail build_identity_mismatch
  fi
  check_identity "$binary" "$observed"
done

labels=""
if ! labels=$(docker image inspect --format '{{json .Config.Labels}}' "$image_tag"); then
  fail build_identity_mismatch
fi
jq -e \
  --arg commit "$release_commit" \
  --arg tree "$source_tree" \
  --arg digest "$contract_digest" \
  '.["org.opencontainers.image.revision"] == $commit and .["io.oblivious.source-tree"] == $tree and .["io.oblivious.release-contract-digest"] == $digest and .["io.oblivious.build-identity-schema"] == "build-identity/v1" and .["io.oblivious.evidence-class"] == "repository-local"' \
  <<<"$labels" >/dev/null || fail build_identity_mismatch

if ! container_id=$(docker create "$image_tag"); then
  fail contract_digest_mismatch
fi
package_root="$output_dir/packaged"
rm -rf "$package_root"
mkdir -p "$package_root/config/release" "$package_root/scripts"
if ! docker cp "$container_id:/app/config/release/contract.v1.json" "$package_root/config/release/contract.v1.json" >/dev/null; then
  fail contract_digest_mismatch
fi
if ! docker cp "$container_id:/app/config/release/contract.schema.json" "$package_root/config/release/contract.schema.json" >/dev/null; then
  fail contract_digest_mismatch
fi
if ! docker cp "$container_id:/app/scripts/release-profile-operation.sh" "$package_root/scripts/release-profile-operation.sh" >/dev/null; then
  fail contract_digest_mismatch
fi
chmod +x "$package_root/scripts/release-profile-operation.sh"
docker rm -f "$container_id" >/dev/null
container_id=""

packaged_digest_json=""
if ! packaged_digest_json=$(cd "$source_root/src/server" && go run ./cmd/release-contract digest \
  --repo "$package_root" --contract config/release/contract.v1.json --schema config/release/contract.schema.json); then
  fail contract_digest_mismatch
fi
packaged_digest=$(jq -er '.contractDigest' <<<"$packaged_digest_json") || fail contract_digest_mismatch
[[ "$packaged_digest" == "$contract_digest" ]] || fail contract_digest_mismatch

printf '{"schemaVersion":"release-build-result/v1","result":"pass","evidenceClass":"repository-local","imageTag":"%s","releaseCommit":"%s","sourceTree":"%s","contractDigest":"%s","fixtureMode":%s}\n' \
  "$image_tag" "$release_commit" "$source_tree" "$contract_digest" "$fixture_mode"
