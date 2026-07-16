#!/usr/bin/env bash
set -euo pipefail

source_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
wrapper="$source_root/scripts/build-release-image.sh"
fixture_root=$(mktemp -d)
authority_root="$fixture_root/authority"
fake_bin="$fixture_root/bin"
docker_state="$fixture_root/docker-state"
real_go=$(command -v go)
mkdir -p "$authority_root/config/release" "$authority_root/scripts" "$fake_bin" "$docker_state"

cleanup() {
  rm -rf "$fixture_root"
}
trap cleanup EXIT

cp "$source_root/config/release/contract.v1.json" "$authority_root/config/release/contract.v1.json"
cp "$source_root/config/release/contract.schema.json" "$authority_root/config/release/contract.schema.json"
cp "$source_root/scripts/release-profile-operation.sh" "$authority_root/scripts/release-profile-operation.sh"
printf 'release build fixture\n' >"$authority_root/README.md"
git -C "$authority_root" init -q
git -C "$authority_root" config user.name "Oblivious Fixture"
git -C "$authority_root" config user.email "oblivious-fixture@example.invalid"
git -C "$authority_root" add .
git -C "$authority_root" commit -q -m fixture

cat >"$fake_bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "run" ]]; then
  exec "$REAL_GO" "$@"
fi
if [[ "${1:-}" != "build" ]]; then
  echo "unexpected go command: $*" >&2
  exit 90
fi
count=0
[[ -f "$FAKE_DOCKER_STATE/go-build-count" ]] && count=$(<"$FAKE_DOCKER_STATE/go-build-count")
printf '%s\n' "$((count + 1))" >"$FAKE_DOCKER_STATE/go-build-count"
output=""
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "-o" ]]; then
    output="$2"
    break
  fi
  shift
done
[[ -n "$output" ]] || exit 91
printf '#!/usr/bin/env sh\nexit 0\n' >"$output"
chmod +x "$output"
EOF
chmod +x "$fake_bin/go"

cat >"$fake_bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
state="$FAKE_DOCKER_STATE"
mutation="${BUILD_FIXTURE_MUTATION:-none}"
command_name="${1:-}"
shift || true
case "$command_name" in
  build)
    build_count=0
    [[ -f "$state/docker-build-count" ]] && build_count=$(<"$state/docker-build-count")
    printf '%s\n' "$((build_count + 1))" >"$state/docker-build-count"
    while [[ $# -gt 0 ]]; do
      if [[ "$1" == "--build-arg" ]]; then
        case "$2" in
          RELEASE_COMMIT=*) printf '%s' "${2#*=}" >"$state/commit" ;;
          SOURCE_TREE=*) printf '%s' "${2#*=}" >"$state/tree" ;;
          CONTRACT_DIGEST=*) printf '%s' "${2#*=}" >"$state/digest" ;;
        esac
        shift 2
      else
        shift
      fi
    done
    ;;
  run)
    entrypoint=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --rm) shift ;;
        --entrypoint) entrypoint="$2"; shift 2 ;;
        *) shift ;;
      esac
    done
    component=${entrypoint##*/oblivious-}
    commit=$(<"$state/commit")
    tree=$(<"$state/tree")
    digest=$(<"$state/digest")
    if [[ "$mutation" == "${component}_mismatch" ]]; then
      commit=ffffffffffffffffffffffffffffffffffffffff
    fi
    jq -nc --arg commit "$commit" --arg tree "$tree" --arg digest "$digest" \
      '{schemaVersion:"build-identity/v1",releaseCommit:$commit,sourceTree:$tree,contractDigest:$digest,dirty:false,evidenceClass:"repository-local"}'
    ;;
  image)
    [[ "${1:-}" == "inspect" ]] || exit 92
    commit=$(<"$state/commit")
    tree=$(<"$state/tree")
    digest=$(<"$state/digest")
    [[ "$mutation" == "oci_revision_mismatch" ]] && commit=ffffffffffffffffffffffffffffffffffffffff
    [[ "$mutation" == "oci_tree_mismatch" ]] && tree=ffffffffffffffffffffffffffffffffffffffff
    [[ "$mutation" == "oci_contract_mismatch" ]] && digest=sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
    jq -nc --arg commit "$commit" --arg tree "$tree" --arg digest "$digest" \
      '{"org.opencontainers.image.revision":$commit,"io.oblivious.source-tree":$tree,"io.oblivious.release-contract-digest":$digest,"io.oblivious.build-identity-schema":"build-identity/v1","io.oblivious.evidence-class":"repository-local"}'
    ;;
  create)
    printf 'fixture-container\n'
    ;;
  cp)
    source_path="$1"
    destination="$2"
    if [[ "$source_path" == *contract.v1.json ]]; then
      [[ "$mutation" == "packaged_contract_missing" ]] && exit 93
      cp "$FIXTURE_CONTRACT" "$destination"
      if [[ "$mutation" == "packaged_contract_mutation" ]]; then
        temporary="$destination.tmp"
        jq '.profiles[0].maxAgeSeconds += 1' "$destination" >"$temporary"
        mv "$temporary" "$destination"
      fi
    elif [[ "$source_path" == *contract.schema.json ]]; then
      cp "$FIXTURE_SCHEMA" "$destination"
    else
      cp "$FIXTURE_OPERATION" "$destination"
    fi
    ;;
  rm)
    ;;
  *)
    echo "unexpected docker command: $command_name $*" >&2
    exit 94
    ;;
esac
EOF
chmod +x "$fake_bin/docker"

export PATH="$fake_bin:$PATH"
export REAL_GO="$real_go"
export FAKE_DOCKER_STATE="$docker_state"
export FIXTURE_CONTRACT="$authority_root/config/release/contract.v1.json"
export FIXTURE_SCHEMA="$authority_root/config/release/contract.schema.json"
export FIXTURE_OPERATION="$authority_root/scripts/release-profile-operation.sh"

run_wrapper() {
  local name="$1"
  local mutation="$2"
  local output_dir="$fixture_root/output-$name"
  BUILD_FIXTURE_MUTATION="$mutation" bash "$wrapper" \
    --fixture-repo "$authority_root" \
    --image-tag "oblivious-fixture:$name" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json \
    --output-dir "$output_dir"
}

assert_failure() {
  local name="$1"
  local mutation="$2"
  local expected_code="$3"
  local output status
  set +e
  output=$(run_wrapper "$name" "$mutation" 2>&1)
  status=$?
  set -e
  if [[ $status -eq 0 ]]; then
    echo "[fixture] $name unexpectedly passed" >&2
    exit 1
  fi
  grep -Fq "\"code\":\"$expected_code\"" <<<"$output" || {
    echo "[fixture] $name missing code $expected_code: $output" >&2
    exit 1
  }
}

baseline=$(run_wrapper baseline none)
grep -Fq '"result":"pass"' <<<"$baseline"
grep -Fq '"fixtureMode":true' <<<"$baseline"

for dirty_case in tracked staged untracked; do
  printf '0\n' >"$docker_state/go-build-count"
  printf '0\n' >"$docker_state/docker-build-count"
  case "$dirty_case" in
    tracked) printf 'dirty\n' >>"$authority_root/README.md" ;;
    staged) printf 'staged\n' >>"$authority_root/README.md"; git -C "$authority_root" add README.md ;;
    untracked) printf 'untracked\n' >"$authority_root/untracked.txt" ;;
  esac
  assert_failure "dirty-$dirty_case" none source_worktree_dirty
  [[ "$(<"$docker_state/go-build-count")" == "0" ]] || {
    echo "[fixture] dirty $dirty_case reached build tool" >&2
    exit 1
  }
  [[ "$(<"$docker_state/docker-build-count")" == "0" ]] || {
    echo "[fixture] dirty $dirty_case reached Docker build" >&2
    exit 1
  }
  git -C "$authority_root" reset -q -- README.md
  git -C "$authority_root" restore README.md
  rm -f "$authority_root/untracked.txt"
done

for mutation in server_mismatch migrate_mismatch grpc-smoke_mismatch oci_revision_mismatch oci_tree_mismatch oci_contract_mismatch; do
  assert_failure "$mutation" "$mutation" build_identity_mismatch
done
assert_failure packaged_contract_mutation packaged_contract_mutation contract_digest_mismatch
assert_failure packaged_contract_missing packaged_contract_missing contract_digest_mismatch

echo "[fixture] release build identity fixtures passed"
