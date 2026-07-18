#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
server_dir="$repo_root/src/server"
web_dir="$repo_root/src/web"
npm_registry="${OBLIVIOUS_SECURITY_NPM_REGISTRY:-https://registry.npmjs.org}"
npm_audit_level="${OBLIVIOUS_SECURITY_AUDIT_LEVEL:-moderate}"
go_toolchain="${OBLIVIOUS_SECURITY_GOTOOLCHAIN:-auto}"
govulncheck_version="${OBLIVIOUS_SECURITY_GOVULNCHECK_VERSION:-v1.3.0}"

echo "[dependency-security] Running pnpm audit against $npm_registry."
pnpm audit --registry="$npm_registry" --audit-level="$npm_audit_level"

echo "[dependency-security] Running npm audit for root package-lock against $npm_registry."
npm --prefix "$repo_root" audit --audit-level="$npm_audit_level" --registry="$npm_registry"

if [[ -f "$web_dir/package-lock.json" ]]; then
  echo "[dependency-security] Running npm audit for src/web package-lock against $npm_registry."
  npm --prefix "$web_dir" audit --audit-level="$npm_audit_level" --registry="$npm_registry"
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
