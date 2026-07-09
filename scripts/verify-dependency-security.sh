#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
web_dir="$repo_root/src/web"
npm_registry="${OBLIVIOUS_SECURITY_NPM_REGISTRY:-https://registry.npmjs.org}"
npm_audit_level="${OBLIVIOUS_SECURITY_AUDIT_LEVEL:-moderate}"
go_toolchain="${OBLIVIOUS_SECURITY_GOTOOLCHAIN:-go1.26.5}"
govulncheck_version="${OBLIVIOUS_SECURITY_GOVULNCHECK_VERSION:-v1.3.0}"

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

run_pnpm_audit() {
  local repo_path

  if command_works pnpm; then
    pnpm --dir "$repo_root" audit --registry="$npm_registry" --audit-level="$npm_audit_level"
    return
  fi
  if command_works corepack; then
    corepack pnpm --dir "$repo_root" audit --registry="$npm_registry" --audit-level="$npm_audit_level"
    return
  fi
  if command -v cmd.exe >/dev/null 2>&1 && [[ -f "/mnt/c/Program Files/nodejs/corepack.cmd" ]]; then
    repo_path=$(windows_path "$repo_root")
    cmd.exe /c "C:\\Program Files\\nodejs\\corepack.cmd" pnpm --dir "$repo_path" audit --registry="$npm_registry" --audit-level="$npm_audit_level"
    return
  fi

  echo "[dependency-security] pnpm or corepack is required." >&2
  return 127
}

run_npm_audit() {
  local prefix="$1"
  local npm_path
  local prefix_path

  if command_works npm; then
    npm_path="$(command -v npm)"
    prefix_path="$prefix"
    case "$npm_path" in
      /mnt/*|/c/*)
        prefix_path=$(windows_path "$prefix")
        ;;
    esac
    npm --prefix "$prefix_path" audit --audit-level="$npm_audit_level" --registry="$npm_registry"
    return
  fi
  if command -v cmd.exe >/dev/null 2>&1 && [[ -f "/mnt/c/Program Files/nodejs/npm.cmd" ]]; then
    prefix_path=$(windows_path "$prefix")
    cmd.exe /c "C:\\Program Files\\nodejs\\npm.cmd" --prefix "$prefix_path" audit --audit-level="$npm_audit_level" --registry="$npm_registry"
    return
  fi

  echo "[dependency-security] npm is required." >&2
  return 127
}

has_auditable_npm_dependencies() {
  local prefix="$1"
  local package_json="$prefix/package.json"

  [[ -f "$prefix/package-lock.json" && -f "$package_json" ]] || return 1
  grep -Eq '"(dependencies|devDependencies|optionalDependencies|peerDependencies)"[[:space:]]*:' "$package_json"
}

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

run_with_retries() {
  local label="$1"
  shift
  local attempt
  local max_attempts="${OBLIVIOUS_SECURITY_AUDIT_RETRIES:-3}"
  local delay_seconds="${OBLIVIOUS_SECURITY_AUDIT_RETRY_DELAY_SECONDS:-5}"

  for attempt in $(seq 1 "$max_attempts"); do
    if "$@"; then
      return 0
    fi
    if [[ "$attempt" == "$max_attempts" ]]; then
      echo "[dependency-security] $label failed after $attempt attempt(s)." >&2
      return 1
    fi
    echo "[dependency-security] $label failed on attempt $attempt; retrying in ${delay_seconds}s." >&2
    sleep "$delay_seconds"
  done
}

echo "[dependency-security] Running pnpm audit against $npm_registry."
run_with_retries "pnpm audit" run_pnpm_audit

if has_auditable_npm_dependencies "$repo_root"; then
  echo "[dependency-security] Running npm audit for root package-lock against $npm_registry."
  run_with_retries "root npm audit" run_npm_audit "$repo_root"
else
  echo "[dependency-security] Skipping root npm audit: no npm package-lock dependency graph."
fi

if has_auditable_npm_dependencies "$web_dir"; then
  echo "[dependency-security] Running npm audit for src/web package-lock against $npm_registry."
  run_with_retries "src/web npm audit" run_npm_audit "$web_dir"
else
  echo "[dependency-security] Skipping src/web npm audit: no npm package-lock dependency graph."
fi

if [[ ! -d "$server_dir" ]]; then
  echo "[dependency-security] Skipping govulncheck: src/server not present."
  exit 0
fi

echo "[dependency-security] Running govulncheck $govulncheck_version with $go_toolchain."
(
  cd "$server_dir"
  GOTOOLCHAIN="$go_toolchain" go run "golang.org/x/vuln/cmd/govulncheck@$govulncheck_version" ./...
)

echo "[dependency-security] Dependency security checks passed."
