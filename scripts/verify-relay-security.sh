#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

collect_go_files() {
  local dir

  for dir in "$@"; do
    [[ -d "$dir" ]] || continue
    find "$dir" -type f -name '*.go' ! -name '*_test.go'
  done
}

assert_fixed_string() {
  local pattern="$1"
  local path="$2"

  grep -Fq -- "$pattern" "$path"
}

mapfile -t app_files < <(
  collect_go_files \
    "$repo_root/src/server/internal/agent" \
    "$repo_root/src/server/internal/chat" \
    "$repo_root/src/server/internal/console" \
    "$repo_root/src/server/internal/http" \
    "$repo_root/src/server/internal/knowledge" \
    "$repo_root/src/server/internal/mcp" \
    "$repo_root/src/server/internal/memory" \
    "$repo_root/src/server/internal/task" \
    "$repo_root/src/server/internal/usage" | sort -u
)

if ((${#app_files[@]} == 0)); then
  echo "[relay-security] no app service files found" >&2
  exit 1
fi

deny_regex='api\.openai\.com|api\.anthropic\.com|generativelanguage\.googleapis\.com|api\.deepseek\.com|dashscope\.aliyuncs\.com|ark\.cn-beijing\.volces\.com|openrouter\.ai/api|g\.baseURL\+"/chat/completions"|NewHTTPReplyGenerator\(cfg\.LLMBaseURL|NewHTTPReplyGenerator\(.*LLMBaseURL'

violations=$(grep -En "$deny_regex" "${app_files[@]}" || true)
if [[ -n "$violations" ]]; then
  echo "[relay-security] app service direct-provider bypass patterns found:" >&2
  echo "$violations" >&2
  exit 1
fi

assert_fixed_string "NewRelayGateway" "$repo_root/src/server/internal/http/router.go"
assert_fixed_string "NewRelayEmbedder" "$repo_root/src/server/internal/http/router.go"
assert_fixed_string "scripts/verify-relay-security.sh" "$repo_root/scripts/check.sh"
assert_fixed_string "TenantIdentityRequired" "$repo_root/src/server/internal/relay/handler/policy.go"
assert_fixed_string "RateLimitPolicy" "$repo_root/src/server/internal/relay/handler/policy.go"
assert_fixed_string "AuditPolicy" "$repo_root/src/server/internal/relay/handler/policy.go"

echo "[relay-security] app services are Relay-only for provider calls."
