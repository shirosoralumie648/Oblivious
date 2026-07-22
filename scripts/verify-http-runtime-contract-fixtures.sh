#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
owner_closure_only=false
if [[ $# -gt 0 ]]; then
  if [[ $# -eq 1 && "$1" == "--owner-closure" ]]; then
    owner_closure_only=true
  else
    printf 'http_runtime_fixture_argument_invalid\n' >&2
    exit 2
  fi
fi

fixture_root=$(mktemp -d "${TMPDIR:-/tmp}/oblivious-http-runtime-fixtures.XXXXXX")
checkout="$fixture_root/checkout"
producer="$fixture_root/release-http-surface"
case_count=0
route_owner_count=0
operation_count=0
status_before=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)

cleanup() {
  rm -rf -- "$fixture_root"
}
trap cleanup EXIT

fail() {
  printf 'http_runtime_fixture_failed: %s\n' "$1" >&2
  exit 1
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    fail "$label unexpectedly passed"
  fi
  case_count=$((case_count + 1))
}

assert_route_owner_closure() {
  python3 - "$1" <<'PY'
from pathlib import Path
import re
import sys

root = Path(sys.argv[1])
owners = [
    "routes_auth.go",
    "routes_agent_memories.go",
    "routes_agent_runs.go",
    "routes_conversation_alias.go",
    "routes_gateway.go",
    "routes_preferences.go",
    "routes_task.go",
    "routes_schedule.go",
    "routes_chat.go",
    "routes_channel.go",
    "routes_console.go",
    "routes_knowledge.go",
    "routes_knowledge_alias.go",
    "routes_workflow.go",
    "routes_observability_alert.go",
    "routes_release_evidence.go",
]
source_dir = root / "src/server/internal/http"
if len(owners) != 16:
    raise SystemExit("http_runtime_route_owner_inventory_invalid")
for owner in owners:
    path = source_dir / owner
    if not path.is_file():
        raise SystemExit("http_runtime_route_owner_missing")
    source = path.read_text(encoding="utf-8")
    if re.search(r"\bmux\s*\.\s*Handle(?:Func)?\s*\(", source):
        raise SystemExit("http_runtime_direct_mount_bypass")
    if "RouteSurfaceRegistration" not in source and "routeSurfaceBinding" not in source:
        raise SystemExit("http_runtime_route_owner_untyped")
router = (source_dir / "router.go").read_text(encoding="utf-8")
if "RouteSurfaceRegistrarFactory" not in router:
    raise SystemExit("http_runtime_router_owner_missing")
print("17")
PY
}

create_checkout() {
  local relative
  local -a tracked_files
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
  git -C "$checkout" -c user.name=http-runtime-fixture -c user.email=http-runtime-fixture.invalid commit -q -m snapshot
}

build_producer() {
  local root="$1" output="$2"
  (
    cd "$root/src/server"
    go build -o "$output" ./cmd/release-http-surface
  )
}

invoke_producer() {
  local root="$1" binary="$2" output="$3"
  shift 3
  env -u GITHUB_SHA "$binary" \
    --repo "$root" \
    --contract config/release/contract.v1.json \
    --schema config/release/contract.schema.json \
    --profile monolith \
    --manifest docs/api/route-surface-manifest.json \
    --output "$output" "$@"
}

route_owner_count=$(assert_route_owner_closure "$repo_root")
[[ "$route_owner_count" == "17" ]] || fail "route owner count"

(
  cd "$repo_root"
  bash scripts/run-go-tests-matched.sh ./cmd/release-http-surface '^TestReleaseHTTPRuntimeSurfaceCommandContract$'
  bash scripts/run-go-tests-matched.sh ./internal/http '^TestRouteSurfaceGroupAContract$'
  bash scripts/run-go-tests-matched.sh ./internal/http '^TestRouteSurfaceGroupBContract$'
)
case_count=$((case_count + 3))

create_checkout
build_producer "$checkout" "$producer"
baseline_report="$fixture_root/baseline.json"
invoke_producer "$checkout" "$producer" "$baseline_report" >/dev/null
operation_count=$(python3 - "$baseline_report" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as handle:
    print(json.load(handle)["evidence"]["details"]["operationCount"])
PY
)
[[ "$operation_count" == "197" ]] || fail "operation count"

if [[ "$owner_closure_only" == true ]]; then
  status_after=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
  [[ "$status_after" == "$status_before" ]] || fail "worktree changed"
  printf '[http-runtime-fixtures] owner closure passed: routeOwners=%s operations=%s cases=%s\n' "$route_owner_count" "$operation_count" "$case_count"
  exit 0
fi

direct_mount_checkout="$fixture_root/direct-mount"
git clone -q "$checkout" "$direct_mount_checkout"
python3 - "$direct_mount_checkout/src/server/internal/http/routes_auth.go" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
path.write_text(path.read_text(encoding="utf-8") + "\nfunc fixtureDirectMount(mux interface{ Handle(string, any) }) { mux.Handle(\"/fixture\", nil) }\n", encoding="utf-8")
PY
expect_failure "direct mount bypass" assert_route_owner_closure "$direct_mount_checkout"

zero_owner_checkout="$fixture_root/zero-owner"
mkdir -p "$zero_owner_checkout"
expect_failure "zero owner count" assert_route_owner_closure "$zero_owner_checkout"

missing_checkout="$fixture_root/missing-descriptor"
git clone -q "$checkout" "$missing_checkout"
python3 - "$missing_checkout/src/server/internal/http/routes_auth.go" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text(encoding="utf-8")
needle = '\t\trouteSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/login", "login", "public", capability, false, "#/components/schemas/CredentialsRequest", "200", sessionResponse),\n'
if source.count(needle) != 1:
    raise SystemExit("fixture login operation selector invalid")
path.write_text(source.replace(needle, "", 1), encoding="utf-8")
PY
missing_binary="$fixture_root/release-http-surface-missing"
build_producer "$missing_checkout" "$missing_binary"
expect_failure "missing descriptor" invoke_producer "$missing_checkout" "$missing_binary" "$fixture_root/missing.json"

duplicate_checkout="$fixture_root/duplicate-descriptor"
git clone -q "$checkout" "$duplicate_checkout"
python3 - "$duplicate_checkout/src/server/internal/http/routes_auth.go" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
source = path.read_text(encoding="utf-8")
needle = '\t\trouteSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/auth/login", "login", "public", capability, false, "#/components/schemas/CredentialsRequest", "200", sessionResponse),\n'
if source.count(needle) != 1:
    raise SystemExit("fixture login operation selector invalid")
path.write_text(source.replace(needle, needle + needle, 1), encoding="utf-8")
PY
duplicate_binary="$fixture_root/release-http-surface-duplicate"
build_producer "$duplicate_checkout" "$duplicate_binary"
expect_failure "duplicate descriptor" invoke_producer "$duplicate_checkout" "$duplicate_binary" "$fixture_root/duplicate.json"

browser_checkout="$fixture_root/browser-field"
git clone -q "$checkout" "$browser_checkout"
python3 - "$browser_checkout/docs/api/route-surface-manifest.json" <<'PY'
import json
from pathlib import Path
import sys
path = Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
value["operations"][0]["responseDecoder"] = "browser"
path.write_text(json.dumps(value), encoding="utf-8")
PY
expect_failure "browser field" invoke_producer "$browser_checkout" "$producer" "$fixture_root/browser.json"

dirty_checkout="$fixture_root/dirty-identity"
git clone -q "$checkout" "$dirty_checkout"
python3 - "$dirty_checkout/README.md" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
path.write_text(path.read_text(encoding="utf-8") + "\nfixture dirty identity\n", encoding="utf-8")
PY
expect_failure "identity unavailable" invoke_producer "$dirty_checkout" "$producer" "$fixture_root/dirty.json"

expect_failure "identity splice" invoke_producer "$checkout" "$producer" "$fixture_root/identity.json" --release-commit ffffffffffffffffffffffffffffffffffffffff
expect_failure "committed skip" invoke_producer "$checkout" "$producer" "$fixture_root/skip.json" --skipped-checks runtime

output_directory="$fixture_root/output-directory"
mkdir -p "$output_directory"
expect_failure "atomic output" invoke_producer "$checkout" "$producer" "$output_directory"

bash "$checkout/scripts/verify-http-runtime-contract.sh" --clean-head --output "$fixture_root/clean-head.json" >/dev/null
case_count=$((case_count + 1))

(( case_count > 0 )) || fail "case count zero"
[[ "$route_owner_count" -gt 0 && "$operation_count" -gt 0 ]] || fail "discovery count zero"
status_after=$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)
[[ "$status_after" == "$status_before" ]] || fail "worktree changed"

printf '[http-runtime-fixtures] passed: routeOwners=%s operations=%s cases=%s\n' "$route_owner_count" "$operation_count" "$case_count"
