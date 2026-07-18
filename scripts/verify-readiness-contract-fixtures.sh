#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
harness="$repo_root/scripts/verify-readiness-deployment-harness.sh"
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

fail() {
  echo "[readiness-contract-fixtures] $*" >&2
  exit 1
}

expect_failure() {
  local label="$1" pattern="$2"; shift 2
  local output="$tmpdir/$label.out"
  if "$@" >"$output" 2>&1; then
    cat "$output" >&2
    fail "$label unexpectedly passed"
  fi
  grep -Fq "$pattern" "$output" || { cat "$output" >&2; fail "$label missing error $pattern"; }
  echo "[readiness-contract-fixtures] rejected $label"
}

bash -n "$harness" "$repo_root/scripts/verify-readiness-contract.sh" "$repo_root/scripts/verify-readiness-deployment-contract.sh"
bash "$repo_root/scripts/verify-readiness-deployment-contract.sh"
bash "$repo_root/scripts/run-go-tests-matched.sh" ./internal/surfacereport '^TestDeploymentSurfaceContract$'

expect_failure invalid-mode invalid_mode bash "$harness" --mode caller --repo-root "$repo_root" --output-dir "$tmpdir/invalid-mode"
expect_failure missing-bundle artifact_bundle_required bash "$harness" --mode aggregate-consume --repo-root "$repo_root" --output-dir "$tmpdir/missing-bundle"

shim_dir="$tmpdir/shim"
mkdir -p "$shim_dir"
cat >"$shim_dir/docker" <<'SH'
#!/usr/bin/env bash
exit 1
SH
chmod +x "$shim_dir/docker"
expect_failure docker-unavailable docker_unavailable env PATH="$shim_dir:$PATH" bash "$harness" --mode standalone-build --repo-root "$repo_root" --output-dir "$tmpdir/docker-unavailable"

grep -Fq 'build_count=0' "$harness" || fail "aggregate build count is not initialized to zero"
grep -Fq 'mode" == "standalone-build' "$harness" || fail "builder is not isolated to standalone mode"
grep -Fq 'artifact_bundle_mismatch' "$harness" || fail "aggregate bundle mismatch is not fail-closed"
grep -Fq 'claim:"repository-local E2 only; no E3/E4, target, or commercial release claim"' "$harness" || fail "claim ceiling is missing"

echo "[readiness-contract-fixtures] fail-closed fixtures passed"
