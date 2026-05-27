#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

mapfile -t app_files < <(
  {
    rg --files "$repo_root/src/server/internal/agent" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/chat" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/console" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/http" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/knowledge" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/mcp" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/memory" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/task" -g '*.go' -g '!*_test.go'
    rg --files "$repo_root/src/server/internal/usage" -g '*.go' -g '!*_test.go'
  } | sort -u
)

if ((${#app_files[@]} == 0)); then
  echo "[relay-security] no app service files found" >&2
  exit 1
fi

deny_regex='api\.openai\.com|api\.anthropic\.com|generativelanguage\.googleapis\.com|api\.deepseek\.com|dashscope\.aliyuncs\.com|ark\.cn-beijing\.volces\.com|openrouter\.ai/api|g\.baseURL\+"/chat/completions"|NewHTTPReplyGenerator\(cfg\.LLMBaseURL|NewHTTPReplyGenerator\(.*LLMBaseURL'

violations=$(rg -n --pcre2 "$deny_regex" "${app_files[@]}" || true)
if [[ -n "$violations" ]]; then
  echo "[relay-security] app service direct-provider bypass patterns found:" >&2
  echo "$violations" >&2
  exit 1
fi

rg -q --fixed-strings "NewRelayGateway" "$repo_root/src/server/internal/http/router.go"
rg -q --fixed-strings "NewRelayEmbedder" "$repo_root/src/server/internal/http/router.go"
rg -q --fixed-strings "scripts/verify-relay-security.sh" "$repo_root/scripts/check.sh"
rg -q --fixed-strings "TenantIdentityRequired" "$repo_root/src/server/internal/relay/handler/policy.go"
rg -q --fixed-strings "RateLimitPolicy" "$repo_root/src/server/internal/relay/handler/policy.go"
rg -q --fixed-strings "AuditPolicy" "$repo_root/src/server/internal/relay/handler/policy.go"

echo "[relay-security] app services are Relay-only for provider calls."
