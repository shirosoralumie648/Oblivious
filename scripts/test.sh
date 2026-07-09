#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_dir="$repo_root/src/web"
server_dir="$repo_root/src/server"
corepack_home="${COREPACK_HOME:-$repo_root/.tmp/corepack}"
go_cache="${GOCACHE:-$repo_root/.tmp/go-build}"
go_mod_cache="${GOMODCACHE:-$repo_root/.tmp/go-mod}"
playwright_browsers_path="${PLAYWRIGHT_BROWSERS_PATH:-$repo_root/.tmp/ms-playwright}"
target="${1:-all}"
require_test_database="${OBLIVIOUS_REQUIRE_TEST_DATABASE:-false}"

usage() {
  cat <<'EOF'
Usage: bash scripts/test.sh [all|web|server|e2e]
EOF
}

mkdir -p "$corepack_home"
mkdir -p "$go_cache" "$go_mod_cache"
mkdir -p "$playwright_browsers_path"
export COREPACK_HOME="$corepack_home"
export GOCACHE="$go_cache"
export GOMODCACHE="$go_mod_cache"
export PLAYWRIGHT_BROWSERS_PATH="$playwright_browsers_path"

command_works() {
  local command_name="$1"

  command -v "$command_name" >/dev/null 2>&1 && "$command_name" --version >/dev/null 2>&1
}

windows_path() {
  local path="$1"

  if command -v wslpath >/dev/null 2>&1; then
    wslpath -w "$path"
    return
  fi
  printf '%s\n' "$path"
}

windows_pnpm_args() {
  local convert_next=false
  local arg

  for arg in "$@"; do
    if [[ "$convert_next" == "true" ]]; then
      windows_path "$arg"
      convert_next=false
      continue
    fi

    printf '%s\n' "$arg"
    if [[ "$arg" == "--dir" || "$arg" == "-C" ]]; then
      convert_next=true
    fi
  done
}

if command_works pnpm; then
  :
elif command_works corepack; then
  pnpm() {
    corepack pnpm "$@"
  }
elif command -v cmd.exe >/dev/null 2>&1 && [[ -f "/mnt/c/Program Files/nodejs/corepack.cmd" ]]; then
  pnpm() {
    local args=()
    local arg

    while IFS= read -r arg; do
      args+=("$arg")
    done < <(windows_pnpm_args "$@")

    cmd.exe /c "C:\\Program Files\\nodejs\\corepack.cmd" pnpm "${args[@]}"
  }
else
  echo "[test] pnpm or corepack is required for web tests." >&2
  exit 127
fi

ensure_go_on_path() {
  local candidate
  local go_path=""
  local tool_bin_dir="$repo_root/.tmp/tool-bin"

  if command -v go >/dev/null 2>&1; then
    return
  fi

  for candidate in \
    "/mnt/c/Program Files/Go/bin/go" \
    "/mnt/c/Program Files/Go/bin/go.exe" \
    "/c/Program Files/Go/bin/go" \
    "/c/Program Files/Go/bin/go.exe" \
    "/usr/local/go/bin/go"; do
    if [[ -x "$candidate" ]]; then
      go_path="$candidate"
      break
    fi
  done

  if [[ -z "$go_path" ]]; then
    return
  fi

  mkdir -p "$tool_bin_dir"
  printf '#!/usr/bin/env sh\nexport WSLENV="${WSLENV:+$WSLENV:}TEST_DATABASE_URL:DATABASE_URL:OBLIVIOUS_REQUIRE_TEST_DATABASE:GOTOOLCHAIN:GOPROXY:GOSUMDB:GOPRIVATE:GONOSUMDB:GOFLAGS:HTTP_PROXY:HTTPS_PROXY:NO_PROXY"\nexec "%s" "$@"\n' "$go_path" > "$tool_bin_dir/go"
  chmod +x "$tool_bin_dir/go"
  export PATH="$tool_bin_dir:$PATH"
}

ensure_go_on_path

run_web_tests() {
  if [[ ! -d "$web_dir" ]]; then
    echo "[test] Skipping web tests: src/web not present."
    return
  fi

  echo "[test] Running web tests."
  pnpm --dir "$web_dir" test
}

run_server_tests() {
  if [[ ! -d "$server_dir" ]]; then
    echo "[test] Skipping server tests: src/server not present."
    return
  fi

  if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
    if [[ "$require_test_database" == "true" ]]; then
      echo "[test] TEST_DATABASE_URL is required when OBLIVIOUS_REQUIRE_TEST_DATABASE=true." >&2
      exit 1
    fi

    echo "[test] Running server unit tests."
    (cd "$server_dir" && go test ./... -count=1)

    echo "[test] Skipping server integration tests: TEST_DATABASE_URL not set."
    return
  fi

  echo "[test] Running server database-backed tests serially."
  (cd "$server_dir" && go test -p 1 ./... -count=1)
}

run_e2e_tests() {
  if [[ ! -d "$web_dir" ]]; then
    echo "[test] Skipping browser E2E tests: src/web not present."
    return
  fi

  echo "[test] Running browser E2E tests."
  if [[ -z "${PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH:-}" ]]; then
    pnpm --dir "$web_dir" exec playwright install chromium
  fi
  pnpm --dir "$web_dir" test:e2e
}

case "$target" in
  all)
    run_web_tests
    run_server_tests
    run_e2e_tests
    ;;
  web)
    run_web_tests
    ;;
  server)
    run_server_tests
    ;;
  e2e)
    run_e2e_tests
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
