#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
helper="$repo_root/scripts/run-go-tests-matched.sh"
fixture_dir=$(mktemp -d "$server_dir/run-go-tests-matched-fixture.XXXXXX")
fixture_package="./$(basename "$fixture_dir")"

cleanup() {
  rm -rf "$fixture_dir"
}
trap cleanup EXIT

cat >"$fixture_dir/matched_test.go" <<'EOF'
package matchedfixture

import "testing"

func TestPassing(t *testing.T) {}

func TestFailing(t *testing.T) {
	t.Fatal("intentional fixture failure")
}
EOF

pass_output=$(bash "$helper" "$fixture_package" '^TestPassing$')
grep -Fq '[go-test-match] matched 1 concrete test(s):' <<<"$pass_output"
grep -Fxq '  TestPassing' <<<"$pass_output"

set +e
zero_output=$(bash "$helper" "$fixture_package" '^TestDoesNotExist$' 2>&1)
zero_status=$?
set -e
if [[ $zero_status -eq 0 ]]; then
  echo "[fixture] zero-match selector unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'selector matched zero concrete tests' <<<"$zero_output"
if [[ $(grep -c '^ok[[:space:]]' <<<"$zero_output") -ne 1 ]]; then
  echo "[fixture] zero-match selector reached an unexpected second go test step" >&2
  exit 1
fi

set +e
package_output=$(bash "$helper" './run-go-tests-matched-package-does-not-exist' '^TestPassing$' 2>&1)
package_status=$?
set -e
if [[ $package_status -eq 0 ]]; then
  echo "[fixture] missing package unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'listing failed with exit status' <<<"$package_output"
grep -Eq 'does not exist|cannot find|not found' <<<"$package_output"

set +e
invalid_output=$(bash "$helper" "$fixture_package" '^Test([$' 2>&1)
invalid_status=$?
set -e
if [[ $invalid_status -eq 0 ]]; then
  echo "[fixture] invalid selector unexpectedly passed" >&2
  exit 1
fi
grep -Fq 'listing failed with exit status' <<<"$invalid_output"
grep -Eq 'error parsing regexp|invalid or unsupported Perl syntax' <<<"$invalid_output"

set +e
failure_output=$(bash "$helper" "$fixture_package" '^TestFailing$' 2>&1)
failure_status=$?
set -e
if [[ $failure_status -eq 0 ]]; then
  echo "[fixture] failing test status was masked" >&2
  exit 1
fi
grep -Fq 'intentional fixture failure' <<<"$failure_output"
grep -Fq -- '--- FAIL: TestFailing' <<<"$failure_output"

echo "[fixture] run-go-tests-matched fixtures passed"
