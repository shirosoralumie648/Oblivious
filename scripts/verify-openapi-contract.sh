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

require_marketplace_paid_install_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    paths = spec.fetch("paths", {})
    missing = []

    detail = schemas["MarketplaceAgentDetailResponse"] || {}
    payment_provider = schemas["MarketplacePaymentProvider"] || {}
    install_request = schemas["MarketplaceInstallRequest"] || {}
    install_response = schemas["MarketplaceInstallResponse"] || {}

    unless detail.dig("properties", "paymentProviders", "items", "$ref") == "#/components/schemas/MarketplacePaymentProvider"
      missing << "MarketplaceAgentDetailResponse.paymentProviders must reference MarketplacePaymentProvider"
    end

    provider_enum = payment_provider.dig("properties", "name", "enum") || []
    unless ["stripe", "alipay", "wechatpay"].all? { |provider| provider_enum.include?(provider) }
      missing << "MarketplacePaymentProvider.name must enumerate stripe, alipay, and wechatpay"
    end

    unless install_request.dig("properties", "versionID", "type") == "string"
      missing << "MarketplaceInstallRequest.versionID must be documented"
    end

    request_provider_enum = install_request.dig("properties", "provider", "enum") || []
    unless ["stripe", "alipay", "wechatpay"].all? { |provider| request_provider_enum.include?(provider) }
      missing << "MarketplaceInstallRequest.provider must enumerate paid-install providers"
    end

    refs = install_response.fetch("oneOf", []).filter_map { |entry| entry["$ref"] }
    unless refs.include?("#/components/schemas/MarketplaceAgentInstall") && refs.include?("#/components/schemas/BillingCheckoutSession")
      missing << "MarketplaceInstallResponse must cover free install records and paid checkout sessions"
    end

    detail_data = paths.dig("/api/v1/marketplace/agents/{agentId}", "get", "responses", "200", "content", "application/json", "schema", "allOf")
    unless detail_data.is_a?(Array) && detail_data.any? { |entry| entry.dig("properties", "data", "$ref") == "#/components/schemas/MarketplaceAgentDetailResponse" }
      missing << "GET /api/v1/marketplace/agents/{agentId} must return MarketplaceAgentDetailResponse data"
    end

    install = paths.dig("/api/v1/marketplace/agents/{agentId}/install", "post") || {}
    unless install.dig("requestBody", "content", "application/json", "schema", "$ref") == "#/components/schemas/MarketplaceInstallRequest"
      missing << "POST /api/v1/marketplace/agents/{agentId}/install must document MarketplaceInstallRequest body"
    end

    install_data = install.dig("responses", "201", "content", "application/json", "schema", "allOf")
    unless install_data.is_a?(Array) && install_data.any? { |entry| entry.dig("properties", "data", "$ref") == "#/components/schemas/MarketplaceInstallResponse" }
      missing << "POST /api/v1/marketplace/agents/{agentId}/install 201 must return MarketplaceInstallResponse data"
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace paid-install contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_marketplace_template_type_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    paths = spec.fetch("paths", {})
    missing = []

    list_enum = paths.dig("/api/v1/marketplace/templates", "get", "parameters")&.
      find { |param| param["name"] == "type" }&.dig("schema", "enum") || []
    create_schema = paths.dig("/api/v1/marketplace/templates", "post", "requestBody", "content", "application/json", "schema") || {}
    create_ref = create_schema["$ref"]
    create_schema = schemas.fetch(create_ref.split("/").last, {}) if create_ref
    create_enum = create_schema.dig("properties", "type", "enum") || []

    [
      ["GET /api/v1/marketplace/templates type query", list_enum],
      ["POST /api/v1/marketplace/templates type body", create_enum],
    ].each do |label, enum_values|
      unless ["agent", "workflow", "plugin"].all? { |value| enum_values.include?(value) }
        missing << "#{label} must enumerate agent, workflow, and plugin"
      end
      if enum_values.include?("bot")
        missing << "#{label} must not expose legacy bot template type"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace template type contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_marketplace_surface_payload_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def response_data_ref(paths, path, method, status)
      paths.dig(path, method, "responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def request_body_ref(paths, path, method)
      paths.dig(path, method, "requestBody", "content", "application/json", "schema", "$ref")
    end

    def request_body_required?(paths, path, method)
      paths.dig(path, method, "requestBody", "required") == true
    end

    expected_data_refs = {
      ["/api/v1/marketplace/agents/{agentId}/abuse-reports", "post", "201"] => "#/components/schemas/MarketplaceAbuseReport",
      ["/api/v1/marketplace/publisher/settlement-preferences", "get", "200"] => "#/components/schemas/MarketplaceSettlementPreferences",
      ["/api/v1/marketplace/publisher/settlement-preferences", "put", "200"] => "#/components/schemas/MarketplaceSettlementPreferences",
      ["/api/v1/marketplace/templates", "get", "200"] => "#/components/schemas/MarketplaceTemplatesResponse",
      ["/api/v1/marketplace/templates", "post", "201"] => "#/components/schemas/MarketplaceTemplate",
      ["/api/v1/marketplace/templates/{templateId}", "get", "200"] => "#/components/schemas/MarketplaceTemplateDetailResponse",
      ["/api/v1/marketplace/templates/{templateId}/install", "post", "201"] => "#/components/schemas/MarketplaceTemplateInstall",
      ["/api/v1/admin/marketplace/abuse-reports", "get", "200"] => "#/components/schemas/MarketplaceAbuseReportsResponse",
      ["/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve", "post", "200"] => "#/components/schemas/MarketplaceAbuseReportStatusResponse",
      ["/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss", "post", "200"] => "#/components/schemas/MarketplaceAbuseReportStatusResponse",
    }

    expected_data_refs.each do |(path, method, status), expected|
      unless response_data_ref(paths, path, method, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    expected_body_refs = {
      ["/api/v1/marketplace/agents/{agentId}/abuse-reports", "post"] => "#/components/schemas/MarketplaceAbuseReportRequest",
      ["/api/v1/marketplace/publisher/settlement-preferences", "put"] => "#/components/schemas/MarketplaceSettlementPreferencesRequest",
      ["/api/v1/marketplace/templates", "post"] => "#/components/schemas/MarketplaceTemplateCreateRequest",
      ["/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve", "post"] => "#/components/schemas/MarketplaceAbuseReportResolutionRequest",
      ["/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss", "post"] => "#/components/schemas/MarketplaceAbuseReportResolutionRequest",
    }

    expected_body_refs.each do |(path, method), expected|
      unless request_body_ref(paths, path, method) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
      unless request_body_required?(paths, path, method)
        missing << "#{method.upcase} #{path} request body must be required"
      end
    end

    template_list = schemas["MarketplaceTemplatesResponse"] || {}
    unless template_list.dig("properties", "templates", "items", "$ref") == "#/components/schemas/MarketplaceTemplate" &&
        template_list.dig("properties", "total", "type") == "integer"
      missing << "MarketplaceTemplatesResponse must expose templates[] and total"
    end

    abuse_list = schemas["MarketplaceAbuseReportsResponse"] || {}
    unless abuse_list.dig("properties", "reports", "items", "$ref") == "#/components/schemas/MarketplaceAbuseReport" &&
        abuse_list.dig("properties", "total", "type") == "integer"
      missing << "MarketplaceAbuseReportsResponse must expose reports[] and total"
    end

    settlement_enum = schemas.dig("MarketplaceSettlementPreferences", "properties", "cycle", "enum") || []
    unless ["weekly", "monthly", "quarterly"].all? { |cycle| settlement_enum.include?(cycle) }
      missing << "MarketplaceSettlementPreferences.cycle must enumerate weekly, monthly, and quarterly"
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace surface payload contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_marketplace_browse_payload_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def response_data_ref(paths, path, method, status)
      paths.dig(path, method, "responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    expected_data_refs = {
      ["/api/v1/marketplace/featured", "get", "200"] => "#/components/schemas/MarketplaceAgentListResponse",
      ["/api/v1/marketplace/search", "get", "200"] => "#/components/schemas/MarketplaceAgentListResponse",
      ["/api/v1/marketplace/agents", "get", "200"] => "#/components/schemas/MarketplaceAgentListResponse",
      ["/api/v1/marketplace/my-agents", "get", "200"] => "#/components/schemas/MarketplaceAgentListResponse",
      ["/api/v1/marketplace/curated", "get", "200"] => "#/components/schemas/MarketplaceCuratedSectionsResponse",
      ["/api/v1/marketplace/categories", "get", "200"] => "#/components/schemas/MarketplaceCategoriesResponse",
      ["/api/v1/marketplace/installs", "get", "200"] => "#/components/schemas/MarketplaceInstallsResponse",
      ["/api/v1/marketplace/agents/{agentId}/reviews", "get", "200"] => "#/components/schemas/MarketplaceReviewsResponse",
      ["/api/v1/marketplace/agents/{agentId}/versions", "get", "200"] => "#/components/schemas/MarketplaceVersionsResponse",
    }

    expected_data_refs.each do |(path, method, status), expected|
      unless response_data_ref(paths, path, method, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    response_collections = {
      "MarketplaceAgentListResponse" => ["agents", "#/components/schemas/MarketplacePublishedAgent"],
      "MarketplaceCategoriesResponse" => ["categories", "#/components/schemas/MarketplaceCategory"],
      "MarketplaceInstallsResponse" => ["installs", "#/components/schemas/MarketplaceAgentInstall"],
      "MarketplaceReviewsResponse" => ["reviews", "#/components/schemas/MarketplaceAgentReview"],
      "MarketplaceVersionsResponse" => ["versions", "#/components/schemas/MarketplaceAgentVersion"],
    }
    response_collections.each do |schema_name, (collection, item_ref)|
      schema = schemas[schema_name] || {}
      unless schema.dig("properties", collection, "type") == "array" &&
          schema.dig("properties", collection, "items", "$ref") == item_ref &&
          schema.dig("properties", "total", "type") == "integer"
        missing << "#{schema_name} must expose #{collection}[] as #{item_ref} plus integer total"
      end
    end

    curated = schemas["MarketplaceCuratedSectionsResponse"] || {}
    ["popular", "topRated", "recent"].each do |property|
      unless curated.dig("properties", property, "type") == "array" &&
          curated.dig("properties", property, "items", "$ref") == "#/components/schemas/MarketplacePublishedAgent"
        missing << "MarketplaceCuratedSectionsResponse.#{property} must expose MarketplacePublishedAgent[]"
      end
    end

    category = schemas["MarketplaceCategory"] || {}
    ["id", "name", "slug"].each do |property|
      unless category.dig("properties", property, "type") == "string"
        missing << "MarketplaceCategory.#{property} must be documented as string"
      end
    end
    unless category.dig("properties", "displayOrder", "type") == "integer" &&
        category.dig("properties", "agentCount", "type") == "integer"
      missing << "MarketplaceCategory must document displayOrder and agentCount integers"
    end

    review = schemas["MarketplaceAgentReview"] || {}
    unless review.dig("properties", "rating", "type") == "integer" &&
        review.dig("properties", "body", "type") == "string"
      missing << "MarketplaceAgentReview must document rating integer and body string"
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace browse payload contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_channel_secret_response_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    expected_data_refs = {
      ["/api/v1/admin/channels", "get", "200"] => "#/components/schemas/AdminChannelListResponse",
      ["/api/v1/admin/channels", "post", "201"] => "#/components/schemas/AdminChannel",
      ["/api/v1/admin/channels/{channelId}", "get", "200"] => "#/components/schemas/AdminChannel",
      ["/api/v1/admin/channels/{channelId}", "put", "200"] => "#/components/schemas/AdminChannel",
      ["/api/v1/admin/channels/{channelId}/test", "post", "200"] => "#/components/schemas/AdminChannelTestResult",
      ["/api/v1/admin/channels/{channelId}/health", "get", "200"] => "#/components/schemas/AdminChannelHealth",
    }

    expected_data_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
      unless op.fetch("tags", []).include?("Admin") && op.fetch("tags", []).include?("Relay")
        missing << "#{method.upcase} #{path} must be tagged Admin and Relay"
      end
    end

    ["/api/v1/admin/channels", "/api/v1/admin/channels/{channelId}", "/api/v1/admin/channels/batch", "/api/v1/admin/channels/{channelId}/test"].each do |path|
      methods = paths.fetch(path, {}).keys.select { |method| ["post", "put", "delete"].include?(method) }
      methods.each do |method|
        op = operation(paths, path, method, missing)
        unless requires_cookie_and_csrf?(op)
          missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
        end
      end
    end

    create = operation(paths, "/api/v1/admin/channels", "post", missing)
    update = operation(paths, "/api/v1/admin/channels/{channelId}", "put", missing)
    unless request_body_ref(create) == "#/components/schemas/AdminChannelCreateRequest"
      missing << "POST /api/v1/admin/channels request body must reference AdminChannelCreateRequest"
    end
    unless request_body_ref(update) == "#/components/schemas/AdminChannelUpdateRequest"
      missing << "PUT /api/v1/admin/channels/{channelId} request body must reference AdminChannelUpdateRequest"
    end

    channel = schemas["AdminChannel"] || {}
    channel_properties = channel.fetch("properties", {})
    if channel_properties.key?("apiKey") || channel_properties.key?("api_key") || channel_properties.key?("apiKeyEncrypted") || channel_properties.key?("api_key_encrypted")
      missing << "AdminChannel response schema must not expose API key fields"
    end
    ["id", "name", "provider", "baseURL", "status"].each do |property|
      unless channel_properties.dig(property, "type") == "string"
        missing << "AdminChannel.#{property} must be documented as string"
      end
    end
    ["models", "groups"].each do |property|
      unless channel_properties.dig(property, "type") == "array" &&
          channel_properties.dig(property, "items", "type") == "string"
        missing << "AdminChannel.#{property} must be documented as string[]"
      end
    end

    list = schemas["AdminChannelListResponse"] || {}
    unless list.dig("properties", "channels", "items", "$ref") == "#/components/schemas/AdminChannel" &&
        list.dig("properties", "total", "type") == "integer"
      missing << "AdminChannelListResponse must expose channels[] as AdminChannel plus integer total"
    end

    create_req = schemas["AdminChannelCreateRequest"] || {}
    update_req = schemas["AdminChannelUpdateRequest"] || {}
    unless create_req.dig("properties", "apiKey", "type") == "string" &&
        update_req.dig("properties", "apiKey", "type") == "string"
      missing << "Admin channel create/update request schemas must document write-only apiKey input"
    end
    unless create_req.dig("properties", "apiKey", "writeOnly") == true &&
        update_req.dig("properties", "apiKey", "writeOnly") == true
      missing << "Admin channel apiKey request fields must be writeOnly"
    end

    unless missing.empty?
      warn "[openapi-contract] Admin channel secret response contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_billing_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    expected_data_refs = {
      ["/api/v1/admin/billing/summary", "get"] => "#/components/schemas/AdminBillingInspectionSummary",
      ["/api/v1/admin/billing/sessions", "get"] => "#/components/schemas/AdminBillingSessionsResponse",
      ["/api/v1/admin/billing/payment-intents", "get"] => "#/components/schemas/AdminPaymentIntentsResponse",
      ["/api/v1/admin/billing/webhook-events", "get"] => "#/components/schemas/AdminWebhookEventsResponse",
      ["/api/v1/admin/billing/subscriptions", "get"] => "#/components/schemas/AdminSubscriptionsResponse",
      ["/api/v1/admin/billing/topups", "get"] => "#/components/schemas/AdminTopupsResponse",
      ["/api/v1/admin/billing/invoices", "get"] => "#/components/schemas/AdminInvoicesResponse",
      ["/api/v1/admin/billing/refunds", "get"] => "#/components/schemas/AdminRefundsResponse",
      ["/api/v1/admin/billing/settlements", "get"] => "#/components/schemas/AdminMarketplaceSettlementsResponse",
      ["/api/v1/admin/billing/payouts", "get"] => "#/components/schemas/AdminMarketplacePayoutsResponse",
      ["/api/v1/admin/billing/topups/{topupId}/refund", "post"] => "#/components/schemas/AdminRefundInspection",
      ["/api/v1/admin/billing/payouts/{payoutId}/paid", "post"] => "#/components/schemas/MarketplacePayout",
    }

    expected_data_refs.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      actual = response_data_ref(op, "200")
      unless actual == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
      tags = op.fetch("tags", [])
      unless tags.include?("Admin") && tags.include?("Billing")
        missing << "#{method.upcase} #{path} must be tagged Admin and Billing"
      end
    end

    list_paths = expected_data_refs.keys.select { |path, method| method == "get" && path != "/api/v1/admin/billing/summary" }.map(&:first)
    list_paths.each do |path|
      names = (paths.dig(path, "get", "parameters") || []).map { |param| param["name"] }
      ["organizationID", "organizationId", "userID", "userId", "status", "kind", "provider", "limit", "offset"].each do |name|
        missing << "GET #{path} must document #{name} query filter" unless names.include?(name)
      end
    end

    summary_names = (paths.dig("/api/v1/admin/billing/summary", "get", "parameters") || []).map { |param| param["name"] }
    ["organizationID", "organizationId", "userID", "userId", "status", "kind", "provider"].each do |name|
      missing << "GET /api/v1/admin/billing/summary must document #{name} query filter" unless summary_names.include?(name)
    end

    refund = operation(paths, "/api/v1/admin/billing/topups/{topupId}/refund", "post", missing)
    unless request_body_ref(refund) == "#/components/schemas/AdminTopupRefundRequest"
      missing << "POST /api/v1/admin/billing/topups/{topupId}/refund must document AdminTopupRefundRequest body"
    end
    unless requires_cookie_and_csrf?(refund)
      missing << "POST /api/v1/admin/billing/topups/{topupId}/refund must require cookieAuth and csrfHeader"
    end

    paid = operation(paths, "/api/v1/admin/billing/payouts/{payoutId}/paid", "post", missing)
    unless request_body_ref(paid) == "#/components/schemas/AdminMarketplacePayoutPaidRequest"
      missing << "POST /api/v1/admin/billing/payouts/{payoutId}/paid must document AdminMarketplacePayoutPaidRequest body"
    end
    unless requires_cookie_and_csrf?(paid)
      missing << "POST /api/v1/admin/billing/payouts/{payoutId}/paid must require cookieAuth and csrfHeader"
    end

    response_collections = {
      "AdminBillingSessionsResponse" => ["sessions", "#/components/schemas/AdminBillingSessionInspection"],
      "AdminPaymentIntentsResponse" => ["paymentIntents", "#/components/schemas/AdminPaymentIntentInspection"],
      "AdminWebhookEventsResponse" => ["webhookEvents", "#/components/schemas/AdminWebhookEventInspection"],
      "AdminSubscriptionsResponse" => ["subscriptions", "#/components/schemas/AdminSubscriptionInspection"],
      "AdminTopupsResponse" => ["topups", "#/components/schemas/AdminTopupInspection"],
      "AdminInvoicesResponse" => ["invoices", "#/components/schemas/AdminInvoiceInspection"],
      "AdminRefundsResponse" => ["refunds", "#/components/schemas/AdminRefundInspection"],
      "AdminMarketplaceSettlementsResponse" => ["settlements", "#/components/schemas/AdminMarketplaceSettlementInspection"],
      "AdminMarketplacePayoutsResponse" => ["payouts", "#/components/schemas/AdminMarketplacePayoutInspection"],
    }
    response_collections.each do |schema_name, (collection, item_ref)|
      schema = schemas[schema_name] || {}
      unless schema.dig("properties", collection, "type") == "array" &&
          schema.dig("properties", collection, "items", "$ref") == item_ref &&
          schema.dig("properties", "total", "type") == "integer"
        missing << "#{schema_name} must expose #{collection}[] as #{item_ref} plus integer total"
      end
    end

    summary = schemas["AdminBillingInspectionSummary"] || {}
    ["billingSessions", "paymentIntents", "webhookEvents", "subscriptions", "topups", "invoices", "refunds", "settlements", "payouts"].each do |property|
      unless summary.dig("properties", property, "$ref") == "#/components/schemas/AdminBillingAmountSummary"
        missing << "AdminBillingInspectionSummary.#{property} must reference AdminBillingAmountSummary"
      end
    end

    unless schemas.key?("MarketplacePayout")
      missing << "MarketplacePayout schema must document the payout paid runtime response"
    end

    unless missing.empty?
      warn "[openapi-contract] Admin Billing route/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_domestic_payment_webhook_payout_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    [
      "/api/v1/billing/alipay/webhook",
      "/api/v1/billing/wechatpay/webhook",
    ].each do |path|
      post = operation(paths, path, "post", missing)
      unless post["security"] == []
        missing << "POST #{path} must declare security: []"
      end

      headers = (post["parameters"] || []).
        select { |param| param["in"] == "header" }.
        map { |param| [param["name"], param] }.
        to_h
      ["Oblivious-Payment-Timestamp", "Oblivious-Payment-Signature"].each do |header|
        param = headers[header]
        unless param && param["required"] == true && param.dig("schema", "type") == "string"
          missing << "POST #{path} must document required #{header} string header"
        end
      end

      unless post.dig("requestBody", "required") == true
        missing << "POST #{path} must require a request body"
      end
      unless post.dig("requestBody", "content", "application/json", "schema", "$ref") == "#/components/schemas/DomesticPaymentWebhookEvent"
        missing << "POST #{path} request body must reference DomesticPaymentWebhookEvent"
      end
      unless response_data_ref(post, "200") == "#/components/schemas/WebhookReceivedResponse"
        missing << "POST #{path} 200 data must reference WebhookReceivedResponse"
      end
    end

    event = schemas["DomesticPaymentWebhookEvent"] || {}
    event_type = event.dig("properties", "type", "enum") || []
    ["payout.paid", "payout.failed"].each do |value|
      unless event_type.include?(value)
        missing << "DomesticPaymentWebhookEvent.type must enumerate #{value}"
      end
    end

    ["payout_id", "provider_payout_id", "status", "reason"].each do |property|
      unless event.dig("properties", property, "type") == "string"
        missing << "DomesticPaymentWebhookEvent.#{property} must be documented as a string"
      end
    end

    required = event.fetch("required", [])
    ["id", "type"].each do |property|
      unless required.include?(property)
        missing << "DomesticPaymentWebhookEvent must require #{property}"
      end
    end

    unless schemas.dig("WebhookReceivedResponse", "properties", "received", "type") == "boolean"
      missing << "WebhookReceivedResponse.received must be documented as a boolean"
    end

    unless missing.empty?
      warn "[openapi-contract] Domestic payment webhook payout contract is incomplete:"
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
  "/api/v1/agent/runs/{runId}/continue-budget"
  "/api/v1/agent/runs/{runId}/approve-plan-step"
  "/api/v1/agent/runs/{runId}/skip-plan-step"
  "/api/v1/agent/runs/{runId}/retry-plan-step"
  "/api/v1/agent/runs/{runId}/update-plan-step"
  "/api/v1/agent/runs/{runId}/create-plan-step"
  "/api/v1/agent/runs/{runId}/move-plan-step"
  "/api/v1/agent/runs/{runId}/delete-plan-step"
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
  "/api/v1/admin/billing/summary"
  "/api/v1/admin/billing/sessions"
  "/api/v1/admin/billing/payment-intents"
  "/api/v1/admin/billing/webhook-events"
  "/api/v1/admin/billing/subscriptions"
  "/api/v1/admin/billing/topups"
  "/api/v1/admin/billing/invoices"
  "/api/v1/admin/billing/refunds"
  "/api/v1/admin/billing/settlements"
  "/api/v1/admin/billing/payouts"
  "/api/v1/admin/billing/topups/{topupId}/refund"
  "/api/v1/admin/billing/payouts/{payoutId}/paid"
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
require_marketplace_paid_install_contract
require_marketplace_template_type_contract
require_marketplace_surface_payload_contract
require_marketplace_browse_payload_contract
require_admin_channel_secret_response_contract
require_admin_billing_contract
require_domestic_payment_webhook_payout_contract

echo "[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented."
