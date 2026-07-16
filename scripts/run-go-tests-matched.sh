#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"

usage() {
  cat <<'EOF' >&2
Usage: bash scripts/run-go-tests-matched.sh <package-pattern> <anchored-test-regex> [go-test-args...]
EOF
}

if [[ $# -lt 2 ]]; then
  usage
  exit 2
fi

package_pattern="$1"
test_regex="$2"
shift 2

if [[ "$test_regex" != ^* || "$test_regex" != *'$' ]]; then
  echo "[go-test-match] test regex must be anchored with ^ and \$: $test_regex" >&2
  exit 2
fi

if [[ ! -d "$server_dir" ]]; then
  echo "[go-test-match] Go module directory not found: $server_dir" >&2
  exit 1
fi

set +e
list_output=$(cd "$server_dir" && go test "$package_pattern" -list "$test_regex" "$@" 2>&1)
list_status=$?
set -e
printf '%s\n' "$list_output"

if [[ $list_status -ne 0 ]]; then
  echo "[go-test-match] listing failed with exit status $list_status" >&2
  exit "$list_status"
fi

mapfile -t matched_tests < <(printf '%s\n' "$list_output" | awk '/^Test[[:alnum:]_]+$/')
if [[ ${#matched_tests[@]} -eq 0 ]]; then
  echo "[go-test-match] selector matched zero concrete tests: $test_regex" >&2
  exit 1
fi

printf '[go-test-match] matched %d concrete test(s):\n' "${#matched_tests[@]}"
printf '  %s\n' "${matched_tests[@]}"

cd "$server_dir"
go test "$package_pattern" -run "$test_regex" -count=1 "$@"
