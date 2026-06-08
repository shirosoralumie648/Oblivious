#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
openapi_file="$repo_root/docs/api/openapi.yaml"

require_path() {
  local path="$1"
  if ! grep -Fq -- "  $path:" "$openapi_file"; then
    echo "[openapi-contract] missing path: $path" >&2
    exit 1
  fi
}

require_public_security_empty() {
  local path="$1"
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    path = ARGV.fetch(1)
    spec = YAML.load_file(file)
    post = spec.fetch("paths", {}).fetch(path, {}).fetch("post", nil)
    unless post && post["security"] == []
      warn "[openapi-contract] public POST #{path} must declare security: []"
      exit 1
    end
  ' "$openapi_file" "$path"
}

require_api_json_responses_use_envelope() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)

    def resolve_ref(spec, ref)
      ref.sub(%r{\A#/}, "").split("/").reduce(spec) { |node, part| node.fetch(part) }
    end

    def schema_refs_envelope?(spec, schema, seen = {})
      return false unless schema.is_a?(Hash) || schema.is_a?(Array)
      return schema.any? { |item| schema_refs_envelope?(spec, item, seen) } if schema.is_a?(Array)

      ref = schema["$ref"]
      if ref
        return true if ref == "#/components/schemas/Envelope"
        return false if seen[ref]

        seen[ref] = true
        return schema_refs_envelope?(spec, resolve_ref(spec, ref), seen)
      end

      schema.any? { |_key, value| schema_refs_envelope?(spec, value, seen.dup) }
    end

    def response_refs_envelope?(spec, response, seen = {})
      ref = response["$ref"] if response.is_a?(Hash)
      if ref
        return false if seen[ref]

        seen[ref] = true
        return response_refs_envelope?(spec, resolve_ref(spec, ref), seen)
      end

      json = response.fetch("content", {}).fetch("application/json", nil)
      return true unless json

      schema_refs_envelope?(spec, json["schema"])
    end

    missing = []
    spec.fetch("paths", {}).each do |path, operations|
      next unless path.start_with?("/api/")
      next if path.start_with?("/api/v1/relay/")

      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        operation.fetch("responses", {}).each do |status, response|
          next if response_refs_envelope?(spec, response)

          missing << "#{method.upcase} #{path} #{status}"
        end
      end
    end

    unless missing.empty?
      warn "[openapi-contract] /api JSON responses must reference #/components/schemas/Envelope:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_session_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    security_schemes = spec.fetch("components", {}).fetch("securitySchemes", {})
    csrf_header = security_schemes["csrfHeader"]
    missing = []

    unless csrf_header && csrf_header["type"] == "apiKey" && csrf_header["in"] == "header" && csrf_header["name"] == "X-CSRF-Token"
      missing << "components.securitySchemes.csrfHeader must document the X-CSRF-Token header"
    end

    session_response = spec.fetch("components", {}).fetch("schemas", {}).fetch("SessionResponse", {})
    csrf_token = session_response.fetch("properties", {})["csrfToken"]
    unless csrf_token && csrf_token["type"] == "string" && csrf_token.fetch("description", "").include?("X-CSRF-Token")
      missing << "components.schemas.SessionResponse.csrfToken must document reuse as X-CSRF-Token"
    end

    logout = spec.fetch("paths", {}).fetch("/api/v1/auth/logout", {}).fetch("post", {})
    security = logout.fetch("security", spec.fetch("security", []))
    unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
      missing << "POST /api/v1/auth/logout must require both cookieAuth and csrfHeader"
    end

    unless missing.empty?
      warn "[openapi-contract] session CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_relay_alias_bearer_contract() {
  ruby -ryaml -e '
    file = ARGV.shift
    paths = ARGV
    spec = YAML.load_file(file)
    schemes = spec.fetch("components", {}).fetch("securitySchemes", {})
    bearer = schemes["bearerAuth"]
    missing = []

    unless bearer && bearer["type"] == "http" && bearer["scheme"] == "bearer"
      missing << "components.securitySchemes.bearerAuth must document Relay bearer tokens"
    end

    paths.each do |path|
      operations = spec.fetch("paths", {}).fetch(path, {})
      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        tags = operation.fetch("tags", [])
        security = operation.fetch("security", spec.fetch("security", []))
        unless tags.include?("Relay")
          missing << "#{method.upcase} #{path} must use the Relay tag"
        end
        unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("bearerAuth") }
          missing << "#{method.upcase} #{path} must require bearerAuth"
        end
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Relay alias bearer contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file" "$@"
}

relay_alias_paths=(
  "/api/v1/relay/chat/completions"
  "/api/v1/relay/embeddings"
  "/api/v1/relay/responses"
  "/api/v1/relay/images/generations"
  "/api/v1/relay/images/edits"
  "/api/v1/relay/images/variations"
  "/api/v1/relay/audio/speech"
  "/api/v1/relay/audio/transcriptions"
  "/api/v1/relay/audio/translations"
  "/api/v1/relay/models"
)

required_paths=(
  "${relay_alias_paths[@]}"
  "/api/v1/agent/tools"
  "/api/v1/agent/runs"
  "/api/v1/agent/runs/{runId}"
  "/api/v1/agent/runs/{runId}/approve-tool"
  "/api/v1/agent/runs/{runId}/reject-tool"
  "/api/v1/agent/runs/{runId}/retry-tool"
  "/api/v1/agent/runs/{runId}/approve-plan-step"
  "/api/v1/agent/runs/{runId}/update-plan-step"
  "/api/v1/agent/runs/{runId}/move-plan-step"
  "/api/v1/agent/runs/{runId}/execute-plan-step"
  "/api/v1/channels"
  "/api/v1/channels/{channelId}"
  "/api/v1/channels/{channelId}/status"
  "/api/v1/channels/{channelId}/test"
  "/api/v1/channels/{channelId}/send"
  "/api/v1/channels/{channelId}/messages"
  "/api/v1/channels/{channelId}/failed-messages"
  "/api/v1/channels/{channelId}/retry-failed-messages"
  "/api/v1/channels/webhook/{channelId}"
  "/api/v1/workflows"
  "/api/v1/workflows/semantic-matches"
  "/api/v1/workflows/conversation-matches"
  "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"
  "/api/v1/workflows/{workflowId}"
  "/api/v1/workflows/{workflowId}/execute"
  "/api/v1/workflows/{workflowId}/webhook"
  "/api/v1/workflows/{workflowId}/versions"
  "/api/v1/workflows/{workflowId}/branches"
  "/api/v1/workflows/{workflowId}/branches/{branchId}/publish"
  "/api/v1/workflows/{workflowId}/branches/{branchId}/merge"
  "/api/v1/workflows/{workflowId}/rollback"
  "/api/v1/workflows/{workflowId}/test-node"
  "/api/v1/workflows/{workflowId}/executions"
  "/api/v1/workflows/{workflowId}/executions/{executionId}"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/debug-snapshot"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/decision"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/pause"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/resume"
  "/api/v1/workflows/{workflowId}/executions/{executionId}/cancel"
  "/api/v1/app/agents"
  "/api/v1/app/agents/{agentId}"
  "/api/v1/app/agents/{agentId}/tools"
  "/api/v1/app/agents/{agentId}/conversations"
  "/api/v1/app/agents/conversations/{conversationId}"
  "/api/v1/app/agents/conversations/{conversationId}/messages"
  "/api/v1/app/agents/conversations/{conversationId}/runs"
  "/api/v1/app/agents/runs/{runId}"
  "/api/v1/app/agents/tool-runs/{toolRunId}/approve"
  "/api/v1/app/agents/tool-runs/{toolRunId}/reject"
  "/api/v1/app/agents/tool-runs/{toolRunId}/retry"
  "/api/v1/app/memory/documents"
  "/api/v1/app/memory/documents/{documentId}"
  "/api/v1/app/memory/documents/{documentId}/chunks"
  "/api/v1/app/memory/search"
  "/api/v1/app/mcp-local-servers"
  "/api/v1/app/mcp-servers"
  "/api/v1/app/mcp-servers/{serverId}"
  "/api/v1/app/mcp-servers/{serverId}/connect"
  "/api/v1/app/mcp-servers/{serverId}/disconnect"
  "/api/v1/app/mcp-servers/{serverId}/tools"
  "/api/v1/app/mcp-servers/{serverId}/status"
  "/api/v1/app/mcp-servers/{serverId}/execute"
  "/api/v1/app/organizations"
  "/api/v1/app/organizations/{organizationId}/select"
  "/api/v1/app/organizations/{organizationId}/members"
  "/api/v1/app/organizations/{organizationId}/members/{userId}"
  "/api/v1/app/organizations/{organizationId}/invitations"
  "/api/v1/app/organizations/{organizationId}/invitations/{invitationId}/revoke"
  "/api/v1/app/organizations/{organizationId}/ownership-transfer"
  "/api/v1/app/organization-invitations/{token}/accept"
  "/api/v1/app/notifications"
  "/api/v1/app/notifications/unread-count"
  "/api/v1/app/notifications/mark-all-read"
  "/api/v1/app/notifications/{notificationId}"
  "/api/v1/app/quota"
  "/api/v1/app/packages"
  "/api/v1/app/quota/topup"
  "/api/v1/console/usage"
  "/api/v1/console/access"
  "/api/v1/console/models"
  "/api/v1/console/billing"
  "/api/v1/console/invoices"
  "/api/v1/console/api-tokens"
  "/api/v1/console/api-tokens/{tokenId}"
  "/api/v1/console/api-tokens/{tokenId}/usage"
  "/api/v1/billing/checkout"
  "/api/v1/billing/stripe/webhook"
  "/api/v1/billing/alipay/webhook"
  "/api/v1/billing/wechatpay/webhook"
  "/api/v1/marketplace/featured"
  "/api/v1/marketplace/curated"
  "/api/v1/marketplace/categories"
  "/api/v1/marketplace/search"
  "/api/v1/marketplace/agents"
  "/api/v1/marketplace/agents/{agentId}"
  "/api/v1/marketplace/agents/{agentId}/install"
  "/api/v1/marketplace/agents/{agentId}/reviews"
  "/api/v1/marketplace/agents/{agentId}/appeal"
  "/api/v1/marketplace/agents/{agentId}/abuse-reports"
  "/api/v1/marketplace/agents/{agentId}/versions"
  "/api/v1/marketplace/agents/{agentId}/stats"
  "/api/v1/marketplace/my-agents"
  "/api/v1/marketplace/installs"
  "/api/v1/marketplace/installs/{agentId}"
  "/api/v1/marketplace/publisher/stats"
  "/api/v1/marketplace/publisher/settlement-preferences"
  "/api/v1/marketplace/templates"
  "/api/v1/marketplace/templates/{templateId}"
  "/api/v1/marketplace/templates/{templateId}/install"
  "/api/v1/admin/marketplace/agents/{agentId}/takedown"
  "/api/v1/admin/marketplace/agents/{agentId}/reinstate"
  "/api/v1/admin/marketplace/abuse-reports"
  "/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve"
  "/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss"
  "/api/v1/admin/organizations"
  "/api/v1/admin/organizations/{organizationId}"
  "/api/v1/admin/organizations/{organizationId}/archive"
  "/api/v1/admin/organizations/{organizationId}/members"
  "/api/v1/admin/observability/alert-routing"
  "/api/v1/admin/observability/alert-providers"
  "/api/v1/admin/observability/alert-providers/{providerId}"
  "/api/v1/admin/observability/alert-providers/{providerId}/test"
  "/api/v1/admin/observability/alerts"
  "/api/v1/admin/observability/alerts/{alertKey}"
  "/api/v1/admin/observability/alerts/{alertKey}/acknowledge"
  "/api/v1/admin/observability/alerts/{alertKey}/resolve"
  "/api/v1/admin/observability/alerts/{alertKey}/deliveries"
  "/api/v1/admin/observability/recovery-actions"
  "/api/v1/admin/reviews"
  "/api/v1/admin/reviews/sla/enforce"
  "/api/v1/admin/reviews/{agentId}/approve"
  "/api/v1/admin/reviews/{agentId}/reject"
  "/api/v1/admin/reviews/{agentId}/needs-changes"
)

for path in "${required_paths[@]}"; do
  require_path "$path"
done

require_public_security_empty "/api/v1/channels/webhook/{channelId}"
require_public_security_empty "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"
require_public_security_empty "/api/v1/billing/stripe/webhook"
require_public_security_empty "/api/v1/billing/alipay/webhook"
require_public_security_empty "/api/v1/billing/wechatpay/webhook"
require_relay_alias_bearer_contract "${relay_alias_paths[@]}"
require_api_json_responses_use_envelope
require_session_csrf_contract

echo "[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented."
