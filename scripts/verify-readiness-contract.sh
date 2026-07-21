#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
stage=${1:-}
shift || true
output_dir=""
expected_head=""

fail() {
  local code="$1"
  printf '{"error":{"code":"%s"}}\n' "$code" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir) [[ $# -ge 2 ]] || fail invalid_arguments; output_dir="$2"; shift 2 ;;
    --expected-head) [[ $# -ge 2 ]] || fail invalid_arguments; expected_head="$2"; shift 2 ;;
    *) fail invalid_arguments ;;
  esac
done

[[ "$stage" == "stage-a" || "$stage" == "stage-b" ]] || fail invalid_stage
[[ -n "$output_dir" ]] || fail output_required

# The production descriptor exact join is a prerequisite for either deployment
# harness stage. Keep this selector anchored before Docker or identity work.
bash "$repo_root/scripts/run-go-tests-matched.sh" ./internal/http '^TestProductionEffectCoverageContract$'
bash "$repo_root/scripts/verify-readiness-deployment-contract.sh"
bash "$repo_root/scripts/run-go-tests-matched.sh" ./internal/surfacereport '^TestDeploymentSurfaceContract$'

if [[ "$stage" == "stage-a" ]]; then
  [[ -z "$expected_head" ]] || fail invalid_arguments
  fixture=$(mktemp -d)
  cleanup() {
    git -C "$repo_root" worktree remove --force "$fixture" >/dev/null 2>&1 || true
    rm -rf "$fixture"
  }
  trap cleanup EXIT
  git -C "$repo_root" worktree add --detach "$fixture" HEAD >/dev/null || fail fixture_worktree_failed
  bash "$fixture/scripts/verify-readiness-deployment-harness.sh" --mode standalone-build --repo-root "$fixture" --output-dir "$output_dir"
  jq -e '.result == "pass" and .evidenceClass == "repository-local" and .skippedChecks == []' "$output_dir/harness-result.json" >/dev/null || fail stage_a_failed
  echo "[readiness-contract] stage-a repository-local E2 passed"
  exit 0
fi

[[ "$expected_head" =~ ^[0-9a-f]{40}$ ]] || fail expected_head_required
actual_head=$(git -C "$repo_root" rev-parse 'HEAD^{commit}')
[[ "$actual_head" == "$expected_head" ]] || fail expected_head_mismatch
[[ -z "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]] || fail source_worktree_dirty
bash "$repo_root/scripts/verify-readiness-deployment-harness.sh" --mode standalone-build --repo-root "$repo_root" --output-dir "$output_dir"
jq -e --arg head "$expected_head" '.result == "pass" and .releaseCommit == $head and .evidenceClass == "repository-local" and .skippedChecks == []' "$output_dir/harness-result.json" >/dev/null || fail stage_b_failed
[[ "$(git -C "$repo_root" rev-parse 'HEAD^{commit}')" == "$expected_head" ]] || fail expected_head_changed
[[ -z "$(git -C "$repo_root" status --porcelain=v1 --untracked-files=all)" ]] || fail source_worktree_dirty
echo "[readiness-contract] stage-b exact clean HEAD repository-local E2 passed"
