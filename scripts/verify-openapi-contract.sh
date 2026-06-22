#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
openapi_file="$repo_root/docs/api/openapi.yaml"
route_surface_manifest_file="$repo_root/docs/api/route-surface-manifest.json"

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
    spec = YAML.unsafe_load_file(file)
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
    spec = YAML.unsafe_load_file(file)

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

require_api_success_data_uses_named_schema() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)

    def resolve_ref(spec, ref)
      ref.sub(%r{\A#/}, "").split("/").reduce(spec) { |node, part| node.fetch(part) }
    end

    def resolve_response(spec, response)
      return response unless response.is_a?(Hash) && response["$ref"]

      resolve_ref(spec, response["$ref"])
    end

    def envelope_data_schema(schema)
      return nil unless schema.is_a?(Hash)

      schema.fetch("allOf", []).find { |entry| entry.dig("properties", "data") }&.
        dig("properties", "data")
    end

    missing = []
    spec.fetch("paths", {}).each do |path, operations|
      next unless path.start_with?("/api/")
      next if path.start_with?("/api/v1/relay/")

      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        operation.fetch("responses", {}).each do |status, response|
          next unless status.to_s.start_with?("2")

          json = resolve_response(spec, response).fetch("content", {}).fetch("application/json", nil)
          next unless json

          data = envelope_data_schema(json["schema"])
          next unless data.is_a?(Hash)
          next if data["$ref"]
          next if data["type"] == "array" && (data.dig("items", "$ref") || data.dig("items", "type"))

          missing << "#{method.upcase} #{path} #{status}"
        end
      end
    end

    unless missing.empty?
      warn "[openapi-contract] /api 2xx JSON response data objects must use named component schemas:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_api_json_request_bodies_use_named_schemas() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    allowed_inline_bodies = {
      ["post", "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"] => "public workflow webhook payload",
      ["post", "/api/v1/workflows/{workflowId}/webhook"] => "session workflow webhook payload",
      ["post", "/api/v1/channels/webhook/{channelId}"] => "public channel webhook payload",
      ["post", "/api/v1/billing/stripe/webhook"] => "Stripe provider webhook payload",
    }

    missing = []
    malformed_allowed = []
    spec.fetch("paths", {}).each do |path, operations|
      next unless path.start_with?("/api/")

      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        schema = operation.dig("requestBody", "content", "application/json", "schema")
        next unless schema
        next if schema["$ref"]

        key = [method, path]
        if allowed_inline_bodies.key?(key)
          unless schema["type"] == "object" && schema["additionalProperties"] == true
            malformed_allowed << "#{method.upcase} #{path} must remain an object with additionalProperties: true for #{allowed_inline_bodies.fetch(key)}"
          end
          next
        end

        missing << "#{method.upcase} #{path}"
      end
    end

    unless missing.empty? && malformed_allowed.empty?
      warn "[openapi-contract] /api JSON request bodies must use named component schemas except approved webhook payloads:"
      missing.each { |entry| warn "  - #{entry}" }
      malformed_allowed.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_api_security_surface_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    public_mutations = {
      ["post", "/api/v1/auth/register"] => "public auth registration",
      ["post", "/api/v1/auth/login"] => "public auth login",
      ["post", "/api/v1/auth/password-reset/request"] => "public password reset request",
      ["post", "/api/v1/auth/password-reset/confirm"] => "public password reset confirmation",
      ["post", "/api/v1/workflows/webhooks/{organizationId}/{workflowId}"] => "public workflow webhook",
      ["post", "/api/v1/channels/webhook/{channelId}"] => "public channel webhook",
      ["post", "/api/v1/billing/stripe/webhook"] => "Stripe provider webhook",
      ["post", "/api/v1/billing/alipay/webhook"] => "Alipay provider webhook",
      ["post", "/api/v1/billing/wechatpay/webhook"] => "WeChat Pay provider webhook",
    }

    missing_security = []
    missing_csrf = []
    malformed_public = []
    malformed_relay = []

    spec.fetch("paths", {}).each do |path, operations|
      next unless path.start_with?("/api/")

      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        security = operation["security"]
        if security.nil?
          missing_security << "#{method.upcase} #{path}"
          next
        end

        next unless %w[post put patch delete].include?(method)

        key = [method, path]
        if public_mutations.key?(key)
          unless security == []
            malformed_public << "#{method.upcase} #{path} must declare security: [] for #{public_mutations.fetch(key)}"
          end
          next
        end

        if path.start_with?("/api/v1/relay/")
          unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("bearerAuth") }
            malformed_relay << "#{method.upcase} #{path} must use bearerAuth for OpenAI-compatible Relay aliases"
          end
          next
        end

        unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
          missing_csrf << "#{method.upcase} #{path}"
        end
      end
    end

    unless missing_security.empty? && missing_csrf.empty? && malformed_public.empty? && malformed_relay.empty?
      warn "[openapi-contract] /api security surface contract failed:"
      missing_security.each { |entry| warn "  - #{entry} must declare a security field" }
      missing_csrf.each { |entry| warn "  - #{entry} must require cookieAuth plus csrfHeader" }
      malformed_public.each { |entry| warn "  - #{entry}" }
      malformed_relay.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_api_path_parameter_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)

    def resolve_ref(spec, ref)
      ref.sub(%r{\A#/}, "").split("/").reduce(spec) { |node, part| node.fetch(part) }
    end

    def resolve_parameter(spec, parameter)
      return parameter unless parameter.is_a?(Hash) && parameter["$ref"]

      resolve_ref(spec, parameter["$ref"])
    end

    missing = []
    malformed = []
    extra = []

    spec.fetch("paths", {}).each do |path, operations|
      next unless path.start_with?("/api/")

      expected_names = path.scan(/\{([^}]+)\}/).flatten
      shared_parameters = operations.fetch("parameters", [])

      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        parameters = (shared_parameters + operation.fetch("parameters", [])).map { |parameter| resolve_parameter(spec, parameter) }
        path_parameters = parameters.select { |parameter| parameter.is_a?(Hash) && parameter["in"] == "path" }

        expected_names.each do |name|
          parameter = path_parameters.find { |candidate| candidate["name"] == name }
          if parameter.nil?
            missing << "#{method.upcase} #{path} missing path parameter #{name}"
            next
          end

          unless parameter["required"] == true
            malformed << "#{method.upcase} #{path} path parameter #{name} must set required: true"
          end

          schema = parameter["schema"]
          unless schema.is_a?(Hash) && (schema["type"] || schema["$ref"])
            malformed << "#{method.upcase} #{path} path parameter #{name} must declare a schema type or ref"
          end
        end

        path_parameters.each do |parameter|
          name = parameter["name"]
          extra << "#{method.upcase} #{path} declares extra path parameter #{name}" unless expected_names.include?(name)
        end
      end
    end

    unless missing.empty? && malformed.empty? && extra.empty?
      warn "[openapi-contract] /api path parameter contract failed:"
      missing.each { |entry| warn "  - #{entry}" }
      malformed.each { |entry| warn "  - #{entry}" }
      extra.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_api_operation_metadata_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)

    def resolve_ref(spec, ref)
      ref.sub(%r{\A#/}, "").split("/").reduce(spec) { |node, part| node.fetch(part) }
    end

    def resolve_parameter(spec, parameter)
      return parameter unless parameter.is_a?(Hash) && parameter["$ref"]

      resolve_ref(spec, parameter["$ref"])
    end

    missing_operation_id = []
    duplicate_operation_id = Hash.new { |hash, key| hash[key] = [] }
    missing_tags = []
    malformed_parameters = []
    duplicate_parameters = []

    spec.fetch("paths", {}).each do |path, operations|
      next unless path.start_with?("/api/")

      shared_parameters = operations.fetch("parameters", [])

      operations.each do |method, operation|
        next unless operation.is_a?(Hash)

        operation_id = operation["operationId"]
        if operation_id.to_s.strip.empty?
          missing_operation_id << "#{method.upcase} #{path}"
        else
          duplicate_operation_id[operation_id] << "#{method.upcase} #{path}"
        end

        unless operation["tags"].is_a?(Array) && operation["tags"].any?
          missing_tags << "#{method.upcase} #{path}"
        end

        seen_parameters = {}
        (shared_parameters + operation.fetch("parameters", [])).each do |parameter|
          parameter = resolve_parameter(spec, parameter)
          unless parameter.is_a?(Hash)
            malformed_parameters << "#{method.upcase} #{path} has a non-object parameter"
            next
          end

          name = parameter["name"]
          location = parameter["in"]
          unless name.is_a?(String) && !name.strip.empty? && %w[path query header cookie].include?(location)
            malformed_parameters << "#{method.upcase} #{path} has malformed parameter name=#{name.inspect} in=#{location.inspect}"
            next
          end

          key = [location, name]
          if seen_parameters[key]
            duplicate_parameters << "#{method.upcase} #{path} declares duplicate parameter #{location}:#{name}"
          end
          seen_parameters[key] = true

          schema = parameter["schema"]
          unless schema.is_a?(Hash) && (schema["type"] || schema["$ref"])
            malformed_parameters << "#{method.upcase} #{path} parameter #{location}:#{name} must declare a schema type or ref"
          end
        end
      end
    end

    duplicate_operation_id.select! { |_operation_id, entries| entries.length > 1 }

    unless missing_operation_id.empty? && duplicate_operation_id.empty? && missing_tags.empty? && malformed_parameters.empty? && duplicate_parameters.empty?
      warn "[openapi-contract] /api operation metadata contract failed:"
      missing_operation_id.each { |entry| warn "  - #{entry} must declare operationId" }
      duplicate_operation_id.each do |operation_id, entries|
        warn "  - operationId #{operation_id} is duplicated by #{entries.join(", ")}"
      end
      missing_tags.each { |entry| warn "  - #{entry} must declare at least one tag" }
      malformed_parameters.each { |entry| warn "  - #{entry}" }
      duplicate_parameters.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_route_surface_manifest_contract() {
  ruby -rjson -ryaml -e '
    openapi_file = ARGV.fetch(0)
    manifest_file = ARGV.fetch(1)
    spec = YAML.unsafe_load_file(openapi_file)
    manifest = JSON.parse(File.read(manifest_file))

    def resolve_ref(spec, ref)
      ref.sub(%r{\A#/}, "").split("/").map { |part| part.gsub("~1", "/").gsub("~0", "~") }.
        reduce(spec) { |node, part| node.fetch(part) }
    end

    def resolve_path_item(spec, item)
      return item unless item.is_a?(Hash) && item["$ref"]

      resolve_ref(spec, item["$ref"])
    end

    def security_kind(operation, spec)
      security = operation.fetch("security", spec.fetch("security", nil))
      return "public" if security == []
      return "bearer" if security.is_a?(Array) && security.any? { |entry| entry.is_a?(Hash) && entry.key?("bearerAuth") }
      return "cookie+csrf" if security.is_a?(Array) && security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
      return "cookie" if security.is_a?(Array) && security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") }

      "unknown"
    end

    http_methods = %w[get post put patch delete]
    openapi_routes = {}
    spec.fetch("paths", {}).each do |path, raw_item|
      next unless path.start_with?("/api/")

      item = resolve_path_item(spec, raw_item)
      http_methods.each do |method|
        operation = item[method]
        next unless operation.is_a?(Hash)

        openapi_routes[[method.upcase, path]] = {
          "security" => security_kind(operation, spec),
          "csrf" => security_kind(operation, spec) == "cookie+csrf",
          "operationId" => operation["operationId"],
          "tags" => operation.fetch("tags", [])
        }
      end
    end

    manifest_routes = {}
    duplicate_manifest = []
    malformed_manifest = []
    manifest.fetch("routes", []).each do |route|
      method = route["method"]
      path = route["path"]
      sample_path = route["samplePath"]
      key = [method, path]

      unless method.is_a?(String) && http_methods.map(&:upcase).include?(method)
        malformed_manifest << "#{method.inspect} #{path.inspect} has an unsupported method"
      end
      unless path.is_a?(String) && path.start_with?("/api/")
        malformed_manifest << "#{method} #{path.inspect} path must start with /api/"
      end
      unless sample_path.is_a?(String) && sample_path.start_with?("/api/") && !sample_path.include?("{") && !sample_path.include?("}")
        malformed_manifest << "#{method} #{path} must provide a concrete /api/ samplePath"
      end
      unless %w[public cookie cookie+csrf bearer].include?(route["security"])
        malformed_manifest << "#{method} #{path} has unsupported security #{route["security"].inspect}"
      end
      unless route["csrf"] == (route["security"] == "cookie+csrf")
        malformed_manifest << "#{method} #{path} csrf must match cookie+csrf security"
      end

      duplicate_manifest << "#{method} #{path}" if manifest_routes.key?(key)
      manifest_routes[key] = route
    end

    missing_manifest = openapi_routes.keys - manifest_routes.keys
    stale_manifest = manifest_routes.keys - openapi_routes.keys
    contract_mismatch = []

    (openapi_routes.keys & manifest_routes.keys).each do |key|
      openapi_route = openapi_routes.fetch(key)
      manifest_route = manifest_routes.fetch(key)
      method, path = key
      if manifest_route["security"] != openapi_route.fetch("security")
        contract_mismatch << "#{method} #{path} security manifest=#{manifest_route["security"].inspect} openapi=#{openapi_route.fetch("security").inspect}"
      end
      if manifest_route["csrf"] != openapi_route.fetch("csrf")
        contract_mismatch << "#{method} #{path} csrf manifest=#{manifest_route["csrf"].inspect} openapi=#{openapi_route.fetch("csrf").inspect}"
      end
      if manifest_route["operationId"].to_s.strip.empty? || manifest_route["operationId"] != openapi_route.fetch("operationId")
        contract_mismatch << "#{method} #{path} operationId must match OpenAPI"
      end
      if !manifest_route["tags"].is_a?(Array) || manifest_route["tags"].empty? || manifest_route["tags"] != openapi_route.fetch("tags")
        contract_mismatch << "#{method} #{path} tags must match OpenAPI"
      end
    end

    unless missing_manifest.empty? && stale_manifest.empty? && duplicate_manifest.empty? && malformed_manifest.empty? && contract_mismatch.empty?
      warn "[openapi-contract] route-surface manifest parity failed:"
      missing_manifest.each { |method, path| warn "  - manifest missing #{method} #{path}" }
      stale_manifest.each { |method, path| warn "  - manifest has stale #{method} #{path}" }
      duplicate_manifest.each { |entry| warn "  - manifest duplicates #{entry}" }
      malformed_manifest.each { |entry| warn "  - #{entry}" }
      contract_mismatch.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file" "$route_surface_manifest_file"
}

require_session_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    public_auth_routes = {
      "/api/v1/auth/register" => ["#/components/schemas/CredentialsRequest", "#/components/schemas/SessionResponse"],
      "/api/v1/auth/login" => ["#/components/schemas/CredentialsRequest", "#/components/schemas/SessionResponse"],
      "/api/v1/auth/password-reset/request" => ["#/components/schemas/PasswordResetRequest", "#/components/schemas/PasswordResetRequestResponse"],
      "/api/v1/auth/password-reset/confirm" => ["#/components/schemas/PasswordResetConfirmRequest", "#/components/schemas/PasswordResetConfirmResponse"],
    }
    public_auth_routes.each do |path, (request_ref, response_ref)|
      operation = spec.fetch("paths", {}).fetch(path, {}).fetch("post", {})
      unless operation["security"] == []
        missing << "POST #{path} must declare security: []"
      end
      unless operation.dig("requestBody", "content", "application/json", "schema", "$ref") == request_ref
        missing << "POST #{path} request body must reference #{request_ref}"
      end
      unless response_data_ref(operation, "200") == response_ref
        missing << "POST #{path} 200 data must reference #{response_ref}"
      end
    end

    cookie_only_get_routes = {
      "/api/v1/auth/me" => ["Auth", "#/components/schemas/SessionResponse"],
      "/api/v1/app/me/preferences" => ["Preferences", "#/components/schemas/Preferences"],
      "/api/v1/app/notifications" => ["Notification", "#/components/schemas/Notification"],
      "/api/v1/app/notifications/unread-count" => ["Notification", "#/components/schemas/NotificationUnreadCount"],
    }
    cookie_only_get_routes.each do |path, (expected_tag, response_ref)|
      operation = spec.fetch("paths", {}).fetch(path, {}).fetch("get", {})
      unless operation.fetch("security", []).any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
        missing << "GET #{path} must declare cookieAuth without csrfHeader"
      end
      unless operation.fetch("tags", []).include?(expected_tag)
        missing << "GET #{path} must be tagged #{expected_tag}"
      end
      actual_response = if path == "/api/v1/app/notifications"
        operation.dig("responses", "200", "content", "application/json", "schema", "allOf")&.
          find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
          dig("properties", "data", "items", "$ref")
      else
        response_data_ref(operation, "200")
      end
      unless actual_response == response_ref
        missing << "GET #{path} 200 data must reference #{response_ref}"
      end
    end

    schemas = spec.fetch("components", {}).fetch("schemas", {})
    unless schemas.dig("PasswordResetRequest", "required")&.include?("email") &&
        schemas.dig("PasswordResetRequest", "properties", "email", "format") == "email"
      missing << "PasswordResetRequest must require email"
    end
    unless schemas.dig("PasswordResetConfirmRequest", "required")&.include?("token") &&
        schemas.dig("PasswordResetConfirmRequest", "required")&.include?("password")
      missing << "PasswordResetConfirmRequest must require token and password"
    end
    unless schemas.dig("PasswordResetRequestResponse", "properties", "requested", "type") == "boolean" &&
        schemas.dig("PasswordResetConfirmResponse", "properties", "reset", "type") == "boolean"
      missing << "Password reset responses must document requested/reset booleans"
    end
    unless schemas.dig("AuthLogoutResponse", "required")&.include?("loggedOut") &&
        schemas.dig("AuthLogoutResponse", "properties", "loggedOut", "type") == "boolean"
      missing << "AuthLogoutResponse must require loggedOut boolean"
    end

    logout = spec.fetch("paths", {}).fetch("/api/v1/auth/logout", {}).fetch("post", {})
    security = logout.fetch("security", spec.fetch("security", []))
    unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
      missing << "POST /api/v1/auth/logout must require both cookieAuth and csrfHeader"
    end
    unless response_data_ref(logout, "200") == "#/components/schemas/AuthLogoutResponse"
      missing << "POST /api/v1/auth/logout 200 data must reference AuthLogoutResponse"
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
    spec = YAML.unsafe_load_file(file)
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    paths = spec.fetch("paths", {})
    missing = []

    detail = schemas["MarketplaceAgentDetailResponse"] || {}
    payment_provider = schemas["MarketplacePaymentProvider"] || {}
    install_request = schemas["MarketplaceInstallRequest"] || {}
    free_install_request = schemas["MarketplaceFreeInstallRequest"] || {}
    paid_install_request = schemas["MarketplacePaidInstallRequest"] || {}
    install_response = schemas["MarketplaceInstallResponse"] || {}

    unless detail.dig("properties", "paymentProviders", "items", "$ref") == "#/components/schemas/MarketplacePaymentProvider"
      missing << "MarketplaceAgentDetailResponse.paymentProviders must reference MarketplacePaymentProvider"
    end

    provider_enum = payment_provider.dig("properties", "name", "enum") || []
    unless ["stripe", "alipay", "wechatpay"].all? { |provider| provider_enum.include?(provider) }
      missing << "MarketplacePaymentProvider.name must enumerate stripe, alipay, and wechatpay"
    end

    install_request_refs = install_request.fetch("anyOf", []).filter_map { |entry| entry["$ref"] }
    unless install_request_refs.include?("#/components/schemas/MarketplaceFreeInstallRequest") && install_request_refs.include?("#/components/schemas/MarketplacePaidInstallRequest")
      missing << "MarketplaceInstallRequest must document free and paid Marketplace install request bodies"
    end

    unless free_install_request.dig("properties", "versionID", "type") == "string"
      missing << "MarketplaceFreeInstallRequest.versionID must be documented"
    end
    paid_request_provider_enum = paid_install_request.dig("properties", "provider", "enum") || []
    unless paid_install_request.fetch("required", []).include?("provider")
      missing << "MarketplacePaidInstallRequest must require provider"
    end
    unless ["stripe", "alipay", "wechatpay"].all? { |provider| paid_request_provider_enum.include?(provider) }
      missing << "MarketplacePaidInstallRequest.provider must enumerate paid-install providers"
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
    ["501", "502"].each do |status|
      unless install.dig("responses", status, "content", "application/json", "schema", "$ref") == "#/components/schemas/Envelope"
        missing << "POST /api/v1/marketplace/agents/{agentId}/install #{status} response must reference Envelope"
      end
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
    spec = YAML.unsafe_load_file(file)
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
    spec = YAML.unsafe_load_file(file)
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
      ["/api/v1/marketplace/agents", "post", "201"] => "#/components/schemas/MarketplacePublishedAgent",
      ["/api/v1/marketplace/agents/{agentId}", "put", "200"] => "#/components/schemas/MarketplacePublishedAgent",
      ["/api/v1/marketplace/agents/{agentId}", "delete", "200"] => "#/components/schemas/MarketplaceActionStatusResponse",
      ["/api/v1/marketplace/agents/{agentId}/install", "delete", "200"] => "#/components/schemas/MarketplaceActionStatusResponse",
      ["/api/v1/marketplace/agents/{agentId}/appeal", "post", "200"] => "#/components/schemas/MarketplaceActionStatusResponse",
      ["/api/v1/marketplace/agents/{agentId}/abuse-reports", "post", "201"] => "#/components/schemas/MarketplaceAbuseReport",
      ["/api/v1/marketplace/agents/{agentId}/reviews", "post", "201"] => "#/components/schemas/MarketplaceAgentReview",
      ["/api/v1/marketplace/installs/{agentId}", "delete", "200"] => "#/components/schemas/MarketplaceActionStatusResponse",
      ["/api/v1/marketplace/publisher/settlement-preferences", "get", "200"] => "#/components/schemas/MarketplaceSettlementPreferences",
      ["/api/v1/marketplace/publisher/settlement-preferences", "put", "200"] => "#/components/schemas/MarketplaceSettlementPreferences",
      ["/api/v1/marketplace/templates", "get", "200"] => "#/components/schemas/MarketplaceTemplatesResponse",
      ["/api/v1/marketplace/templates", "post", "201"] => "#/components/schemas/MarketplaceTemplate",
      ["/api/v1/marketplace/templates/{templateId}", "get", "200"] => "#/components/schemas/MarketplaceTemplateDetailResponse",
      ["/api/v1/marketplace/templates/{templateId}/install", "post", "201"] => "#/components/schemas/MarketplaceTemplateInstall",
      ["/api/v1/marketplace/agents/{agentId}/stats", "get", "200"] => "#/components/schemas/MarketplaceAgentStats",
      ["/api/v1/marketplace/publisher/stats", "get", "200"] => "#/components/schemas/MarketplacePublisherStats",
      ["/api/v1/admin/marketplace/agents/{agentId}/takedown", "post", "200"] => "#/components/schemas/MarketplaceActionStatusResponse",
      ["/api/v1/admin/marketplace/agents/{agentId}/reinstate", "post", "200"] => "#/components/schemas/MarketplaceActionStatusResponse",
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
      ["/api/v1/marketplace/agents", "post"] => "#/components/schemas/MarketplaceAgentPublishRequest",
      ["/api/v1/marketplace/agents/{agentId}", "put"] => "#/components/schemas/MarketplaceAgentPublishRequest",
      ["/api/v1/marketplace/agents/{agentId}/appeal", "post"] => "#/components/schemas/MarketplaceGovernanceActionRequest",
      ["/api/v1/marketplace/agents/{agentId}/abuse-reports", "post"] => "#/components/schemas/MarketplaceAbuseReportRequest",
      ["/api/v1/marketplace/agents/{agentId}/reviews", "post"] => "#/components/schemas/MarketplaceReviewSubmitRequest",
      ["/api/v1/marketplace/publisher/settlement-preferences", "put"] => "#/components/schemas/MarketplaceSettlementPreferencesRequest",
      ["/api/v1/marketplace/templates", "post"] => "#/components/schemas/MarketplaceTemplateCreateRequest",
      ["/api/v1/admin/marketplace/agents/{agentId}/takedown", "post"] => "#/components/schemas/MarketplaceGovernanceActionRequest",
      ["/api/v1/admin/marketplace/agents/{agentId}/reinstate", "post"] => "#/components/schemas/MarketplaceGovernanceActionRequest",
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

    publish_request = schemas["MarketplaceAgentPublishRequest"] || {}
    required_publish_fields = publish_request.fetch("required", [])
    ["name", "description", "categoryID", "tools", "pricingType", "version"].each do |property|
      unless required_publish_fields.include?(property)
        missing << "MarketplaceAgentPublishRequest must require #{property}"
      end
    end
    ["name", "description", "iconURL", "categoryID", "tools", "exampleConversations", "systemPrompt", "pricingType", "version", "changelog"].each do |property|
      unless publish_request.dig("properties", property, "type") == "string"
        missing << "MarketplaceAgentPublishRequest.#{property} must be documented as string"
      end
    end
    unless publish_request.dig("properties", "tags", "type") == "array" &&
        publish_request.dig("properties", "tags", "items", "type") == "string"
      missing << "MarketplaceAgentPublishRequest.tags must be documented as string[]"
    end
    unless publish_request.dig("properties", "pricingAmount", "type") == "number" &&
        publish_request.dig("properties", "pricingAmount", "format") == "double"
      missing << "MarketplaceAgentPublishRequest.pricingAmount must be documented as double"
    end
    visibility_enum = publish_request.dig("properties", "visibility", "enum") || []
    unless ["public", "private", "unlisted"].all? { |value| visibility_enum.include?(value) }
      missing << "MarketplaceAgentPublishRequest.visibility must enumerate public, private, and unlisted"
    end
    pricing_enum = publish_request.dig("properties", "pricingType", "enum") || []
    unless ["free", "one_time", "subscription"].all? { |value| pricing_enum.include?(value) }
      missing << "MarketplaceAgentPublishRequest.pricingType must enumerate free, one_time, and subscription"
    end

    action_status_enum = schemas.dig("MarketplaceActionStatusResponse", "properties", "status", "enum") || []
    unless ["deleted", "uninstalled", "appealed", "takedown", "approved"].all? { |value| action_status_enum.include?(value) }
      missing << "MarketplaceActionStatusResponse.status must enumerate Marketplace action statuses"
    end

    governance_request = schemas["MarketplaceGovernanceActionRequest"] || {}
    unless governance_request.fetch("required", []).include?("reason")
      missing << "MarketplaceGovernanceActionRequest must require reason"
    end
    unless governance_request.dig("properties", "reason", "type") == "string"
      missing << "MarketplaceGovernanceActionRequest.reason must be documented as string"
    end

    review_submit = schemas["MarketplaceReviewSubmitRequest"] || {}
    unless review_submit.fetch("required", []).include?("rating")
      missing << "MarketplaceReviewSubmitRequest must require rating"
    end
    unless review_submit.dig("properties", "rating", "type") == "integer" &&
        review_submit.dig("properties", "rating", "minimum") == 1 &&
        review_submit.dig("properties", "rating", "maximum") == 5
      missing << "MarketplaceReviewSubmitRequest.rating must be documented as integer 1..5"
    end
    unless review_submit.dig("properties", "body", "type") == "string"
      missing << "MarketplaceReviewSubmitRequest.body must be documented as string"
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

    agent_stats = schemas["MarketplaceAgentStats"] || {}
    ["agentID", "agentName"].each do |property|
      unless agent_stats.dig("properties", property, "type") == "string"
        missing << "MarketplaceAgentStats.#{property} must be documented as string"
      end
    end
    ["installCount", "activeUsers", "apiCallCount"].each do |property|
      unless agent_stats.dig("properties", property, "type") == "integer"
        missing << "MarketplaceAgentStats.#{property} must be documented as integer"
      end
    end

    publisher_stats = schemas["MarketplacePublisherStats"] || {}
    ["totalAgents", "totalInstalls", "activeUsers", "totalAPICalls"].each do |property|
      unless publisher_stats.dig("properties", property, "type") == "integer"
        missing << "MarketplacePublisherStats.#{property} must be documented as integer"
      end
    end
    ["grossRevenue", "platformFees", "netRevenue", "refundedAmount", "pendingSettlementAmount", "availableAmount", "payoutPendingAmount", "paidOutAmount"].each do |property|
      unless publisher_stats.dig("properties", property, "type") == "number"
        missing << "MarketplacePublisherStats.#{property} must be documented as number"
      end
    end
    unless publisher_stats.dig("properties", "revenueTier", "$ref") == "#/components/schemas/MarketplaceRevenueTierDisclosure"
      missing << "MarketplacePublisherStats.revenueTier must reference MarketplaceRevenueTierDisclosure"
    end
    unless publisher_stats.dig("properties", "perAgentStats", "items", "$ref") == "#/components/schemas/MarketplaceAgentStats"
      missing << "MarketplacePublisherStats.perAgentStats must expose MarketplaceAgentStats[]"
    end

    revenue_tier = schemas["MarketplaceRevenueTierDisclosure"] || {}
    ["currentTier", "label"].each do |property|
      unless revenue_tier.dig("properties", property, "type") == "string"
        missing << "MarketplaceRevenueTierDisclosure.#{property} must be documented as string"
      end
    end
    ["monthlySalesAmount", "platformFeeAmount", "publisherNetAmount", "platformFeePercent", "publisherSharePercent", "effectivePlatformFeePercent", "nextTierAt", "salesToNextTier", "estimatedPublisherNetAtNextTier", "estimatedPublisherNetIncreaseAtNextTier"].each do |property|
      unless revenue_tier.dig("properties", property, "type") == "number"
        missing << "MarketplaceRevenueTierDisclosure.#{property} must be documented as number"
      end
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
    spec = YAML.unsafe_load_file(file)
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

    published_agent = schemas["MarketplacePublishedAgent"] || {}
    unless published_agent.dig("properties", "recommendation", "$ref") == "#/components/schemas/MarketplaceRecommendationMetadata"
      missing << "MarketplacePublishedAgent.recommendation must reference MarketplaceRecommendationMetadata"
    end
    recommendation = schemas["MarketplaceRecommendationMetadata"] || {}
    unless recommendation.dig("properties", "score", "type") == "number" &&
        recommendation.dig("properties", "score", "format") == "double" &&
        recommendation.dig("properties", "reason", "type") == "string"
      missing << "MarketplaceRecommendationMetadata must document score double and reason string"
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

require_marketplace_private_read_auth_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    [
      "/api/v1/marketplace/agents/{agentId}/stats",
      "/api/v1/marketplace/my-agents",
      "/api/v1/marketplace/installs",
      "/api/v1/marketplace/publisher/stats",
      "/api/v1/marketplace/publisher/settlement-preferences",
    ].each do |path|
      op = operation(paths, path, "get", missing)
      unless requires_cookie_without_csrf?(op)
        missing << "GET #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Marketplace")
        missing << "GET #{path} must be tagged Marketplace"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace private read auth contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_marketplace_public_read_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    [
      "/api/v1/marketplace/featured",
      "/api/v1/marketplace/curated",
      "/api/v1/marketplace/categories",
      "/api/v1/marketplace/search",
      "/api/v1/marketplace/agents",
      "/api/v1/marketplace/agents/{agentId}",
      "/api/v1/marketplace/agents/{agentId}/reviews",
      "/api/v1/marketplace/agents/{agentId}/versions",
      "/api/v1/marketplace/templates",
      "/api/v1/marketplace/templates/{templateId}",
    ].each do |path|
      op = operation(paths, path, "get", missing)
      unless op["security"] == []
        missing << "GET #{path} must declare security: []"
      end
      unless op.fetch("tags", []).include?("Marketplace")
        missing << "GET #{path} must be tagged Marketplace"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace public read contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_channel_secret_response_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    expected_data_refs = {
      ["/api/v1/admin/channel-providers", "get", "200"] => "#/components/schemas/AdminChannelProviderListResponse",
      ["/api/v1/admin/models", "get", "200"] => "#/components/schemas/AdminModelInventoryListResponse",
      ["/api/v1/admin/channels", "get", "200"] => "#/components/schemas/AdminChannelListResponse",
      ["/api/v1/admin/channels", "post", "201"] => "#/components/schemas/AdminChannel",
      ["/api/v1/admin/channels/{channelId}", "get", "200"] => "#/components/schemas/AdminChannel",
      ["/api/v1/admin/channels/{channelId}", "put", "200"] => "#/components/schemas/AdminChannel",
      ["/api/v1/admin/channels/{channelId}/test", "post", "200"] => "#/components/schemas/AdminChannelTestResult",
      ["/api/v1/admin/channels/{channelId}/health", "get", "200"] => "#/components/schemas/AdminChannelHealth",
      ["/api/v1/admin/channels/stats", "get", "200"] => "#/components/schemas/AdminChannelRuntimeStatsResponse",
      ["/api/v1/admin/channels/{channelId}/sync-models", "post", "200"] => "#/components/schemas/AdminChannelModelSyncResponse",
      ["/api/v1/admin/channels/{channelId}/model-updates/detect", "post", "200"] => "#/components/schemas/AdminChannelModelUpdatePreview",
      ["/api/v1/admin/channels/{channelId}/model-updates/apply", "post", "200"] => "#/components/schemas/AdminChannelModelUpdateApplyResponse",
      ["/api/v1/admin/channels/{channelId}/refresh-balance", "post", "200"] => "#/components/schemas/AdminChannelBalanceRefreshResponse",
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

    [
      "/api/v1/admin/channel-providers",
      "/api/v1/admin/models",
      "/api/v1/admin/channels",
      "/api/v1/admin/channels/stats",
      "/api/v1/admin/channels/{channelId}",
      "/api/v1/admin/channels/{channelId}/health",
    ].each do |path|
      op = operation(paths, path, "get", missing)
      unless requires_cookie_without_csrf?(op)
        missing << "GET #{path} must require cookieAuth without csrfHeader"
      end
    end

    [
      "/api/v1/admin/channels",
      "/api/v1/admin/channels/{channelId}",
      "/api/v1/admin/channels/batch",
      "/api/v1/admin/channels/{channelId}/test",
      "/api/v1/admin/channels/{channelId}/sync-models",
      "/api/v1/admin/channels/{channelId}/model-updates/detect",
      "/api/v1/admin/channels/{channelId}/model-updates/apply",
      "/api/v1/admin/channels/{channelId}/refresh-balance",
    ].each do |path|
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

    provider = schemas["AdminChannelProvider"] || {}
    provider_properties = provider.fetch("properties", {})
    ["id", "displayName", "kind", "status", "defaultBaseURL"].each do |property|
      unless provider_properties.dig(property, "type") == "string"
        missing << "AdminChannelProvider.#{property} must be documented as string"
      end
    end
    provider_list = schemas["AdminChannelProviderListResponse"] || {}
    unless provider_list.dig("properties", "providers", "items", "$ref") == "#/components/schemas/AdminChannelProvider"
      missing << "AdminChannelProviderListResponse must expose providers[] as AdminChannelProvider"
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

    sync_response = schemas["AdminChannelModelSyncResponse"] || {}
    unless sync_response.dig("properties", "channel", "$ref") == "#/components/schemas/AdminChannel" &&
        sync_response.dig("properties", "testResult", "$ref") == "#/components/schemas/AdminChannelTestResult"
      missing << "AdminChannelModelSyncResponse must expose channel and testResult"
    end

    preview = schemas["AdminChannelModelUpdatePreview"] || {}
    ["currentModels", "upstreamModels", "added", "removed", "unchanged"].each do |property|
      unless preview.dig("properties", property, "type") == "array" &&
          preview.dig("properties", property, "items", "type") == "string"
        missing << "AdminChannelModelUpdatePreview.#{property} must be documented as string[]"
      end
    end
    unless preview.dig("properties", "testResult", "$ref") == "#/components/schemas/AdminChannelTestResult"
      missing << "AdminChannelModelUpdatePreview.testResult must reference AdminChannelTestResult"
    end

    apply_request = schemas["AdminChannelModelUpdateApplyRequest"] || {}
    apply_enum = apply_request.dig("properties", "mode", "enum") || []
    unless ["merge", "replace"].all? { |mode| apply_enum.include?(mode) }
      missing << "AdminChannelModelUpdateApplyRequest.mode must enumerate merge and replace"
    end
    apply = schemas["AdminChannelModelUpdateApplyResponse"] || {}
    unless apply.dig("properties", "channel", "$ref") == "#/components/schemas/AdminChannel" &&
        apply.dig("properties", "preview", "$ref") == "#/components/schemas/AdminChannelModelUpdatePreview" &&
        apply.dig("properties", "appliedModels", "items", "type") == "string"
      missing << "AdminChannelModelUpdateApplyResponse must expose channel, preview, and appliedModels[]"
    end
    apply_op = operation(paths, "/api/v1/admin/channels/{channelId}/model-updates/apply", "post", missing)
    unless request_body_ref(apply_op) == "#/components/schemas/AdminChannelModelUpdateApplyRequest"
      missing << "POST /api/v1/admin/channels/{channelId}/model-updates/apply request body must reference AdminChannelModelUpdateApplyRequest"
    end

    refresh = schemas["AdminChannelBalanceRefreshResponse"] || {}
    unless refresh.dig("properties", "balance", "$ref") == "#/components/schemas/AdminChannelBalance" &&
        refresh.dig("properties", "channelHealth", "$ref") == "#/components/schemas/AdminChannelHealthDetail" &&
        refresh.dig("properties", "testResult", "$ref") == "#/components/schemas/AdminChannelTestResult"
      missing << "AdminChannelBalanceRefreshResponse must expose balance, channelHealth, and testResult"
    end

    runtime_stats_response = schemas["AdminChannelRuntimeStatsResponse"] || {}
    runtime_stats = schemas["AdminChannelRuntimeStats"] || {}
    unless runtime_stats_response.dig("properties", "stats", "items", "$ref") == "#/components/schemas/AdminChannelRuntimeStats"
      missing << "AdminChannelRuntimeStatsResponse must expose stats[] as AdminChannelRuntimeStats"
    end
    ["channelID", "rpmCurrent", "tpmCurrent", "totalRequests", "successCount", "failureCount", "avgLatencyMs", "affinityConversationCount"].each do |property|
      unless runtime_stats.dig("properties", property)
        missing << "AdminChannelRuntimeStats.#{property} must be documented"
      end
    end

    model_inventory_response = schemas["AdminModelInventoryListResponse"] || {}
    model_inventory_entry = schemas["AdminModelInventoryEntry"] || {}
    model_inventory_channel = schemas["AdminModelInventoryChannel"] || {}
    unless model_inventory_response.dig("properties", "models", "items", "$ref") == "#/components/schemas/AdminModelInventoryEntry"
      missing << "AdminModelInventoryListResponse must expose models[] as AdminModelInventoryEntry"
    end
    unless model_inventory_response.dig("properties", "total")
      missing << "AdminModelInventoryListResponse.total must be documented"
    end
    ["model", "providers", "groups", "channelCount", "enabledChannelCount", "disabledChannelCount", "requestCount", "totalCost", "totalChannelCost", "channels"].each do |property|
      unless model_inventory_entry.dig("properties", property)
        missing << "AdminModelInventoryEntry.#{property} must be documented"
      end
    end
    unless model_inventory_entry.dig("properties", "channels", "items", "$ref") == "#/components/schemas/AdminModelInventoryChannel"
      missing << "AdminModelInventoryEntry.channels must reference AdminModelInventoryChannel"
    end
    ["id", "name", "provider", "groups", "enabled", "priority", "estimatedCostPer1K", "costMultiplier"].each do |property|
      unless model_inventory_channel.dig("properties", property)
        missing << "AdminModelInventoryChannel.#{property} must be documented"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Admin channel secret response contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_publishing_channel_secret_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def response_data_array_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    expected_data_refs = {
      ["/api/v1/channels/{channelId}", "get", "200"] => "#/components/schemas/ChannelConfig",
      ["/api/v1/channels", "post", "201"] => "#/components/schemas/ChannelConfig",
      ["/api/v1/channels/{channelId}", "put", "200"] => "#/components/schemas/ChannelConfig",
      ["/api/v1/channels/{channelId}", "delete", "200"] => "#/components/schemas/ChannelConfig",
      ["/api/v1/channels/{channelId}/status", "patch", "200"] => "#/components/schemas/ChannelConfig",
      ["/api/v1/channels/{channelId}/test", "post", "200"] => "#/components/schemas/ChannelTestResult",
      ["/api/v1/channels/{channelId}/send", "post", "200"] => "#/components/schemas/ChannelMessageLog",
      ["/api/v1/channels/{channelId}/retry-failed-messages", "post", "200"] => "#/components/schemas/ChannelRetryProcessResult",
      ["/api/v1/channels/webhook/{channelId}", "post", "200"] => "#/components/schemas/ChannelMessageLog",
    }

    expected_data_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
      unless op.fetch("tags", []).include?("Publishing")
        missing << "#{method.upcase} #{path} must be tagged Publishing"
      end
    end

    ["/api/v1/channels", "/api/v1/channels/{channelId}/messages", "/api/v1/channels/{channelId}/failed-messages"].each do |path|
      op = operation(paths, path, "get", missing)
      unless response_data_array_ref(op, "200") == "#/components/schemas/#{path == "/api/v1/channels" ? "ChannelConfig" : "ChannelMessageLog"}"
        missing << "GET #{path} 200 data must return the documented Publishing collection item schema"
      end
      unless op.fetch("tags", []).include?("Publishing")
        missing << "GET #{path} must be tagged Publishing"
      end
    end

    [
      "/api/v1/channels",
      "/api/v1/channels/{channelId}",
      "/api/v1/channels/{channelId}/messages",
      "/api/v1/channels/{channelId}/failed-messages",
    ].each do |path|
      op = operation(paths, path, "get", missing)
      unless requires_cookie_without_csrf?(op)
        missing << "GET #{path} must require cookieAuth without csrfHeader"
      end
    end

    [
      ["/api/v1/channels", "post"],
      ["/api/v1/channels/{channelId}", "put"],
      ["/api/v1/channels/{channelId}", "delete"],
      ["/api/v1/channels/{channelId}/status", "patch"],
      ["/api/v1/channels/{channelId}/test", "post"],
      ["/api/v1/channels/{channelId}/send", "post"],
      ["/api/v1/channels/{channelId}/retry-failed-messages", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
    end

    {
      ["/api/v1/channels", "post"] => "#/components/schemas/ChannelConfigRequest",
      ["/api/v1/channels/{channelId}", "put"] => "#/components/schemas/ChannelConfigRequest",
      ["/api/v1/channels/{channelId}/status", "patch"] => "#/components/schemas/ChannelStatusRequest",
      ["/api/v1/channels/{channelId}/send", "post"] => "#/components/schemas/SendChannelMessageRequest",
      ["/api/v1/channels/{channelId}/retry-failed-messages", "post"] => "#/components/schemas/RetryFailedChannelMessagesRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    webhook = operation(paths, "/api/v1/channels/webhook/{channelId}", "post", missing)
    unless webhook["security"] == []
      missing << "POST /api/v1/channels/webhook/{channelId} must remain public with security: []"
    end
    webhook_headers = (webhook["parameters"] || []).select { |param| param["in"] == "header" }.map { |param| param["name"] }
    ["X-Oblivious-Timestamp", "X-Oblivious-Signature"].each do |header|
      missing << "POST /api/v1/channels/webhook/{channelId} must document #{header}" unless webhook_headers.include?(header)
    end

    channel = schemas["ChannelConfig"] || {}
    config = channel.dig("properties", "config") || {}
    unless config["type"] == "object" && config["additionalProperties"] == true &&
        config.fetch("description", "").include?("redacted")
      missing << "ChannelConfig.config must document redacted response secrets"
    end
    channel.fetch("properties", {}).each_key do |property|
      normalized = property.downcase.delete("_-")
      if ["secret", "token", "apikey", "password"].any? { |needle| normalized.include?(needle) }
        missing << "ChannelConfig response schema must not expose #{property} as a top-level credential field"
      end
    end

    request_config = schemas.dig("ChannelConfigRequest", "properties", "config") || {}
    unless request_config["type"] == "object" && request_config["additionalProperties"] == true &&
        ["secret", "token", "apiKey", "password"].all? { |word| request_config.fetch("description", "").include?(word) }
      missing << "ChannelConfigRequest.config must document credential input and redacted-marker preservation"
    end

    test_result = schemas["ChannelTestResult"] || {}
    ["channel_id", "type", "message"].each do |property|
      unless test_result.dig("properties", property, "type") == "string"
        missing << "ChannelTestResult.#{property} must be documented as string"
      end
    end
    status_enum = test_result.dig("properties", "status", "enum") || []
    unless ["success", "failed"].all? { |status| status_enum.include?(status) }
      missing << "ChannelTestResult.status must enumerate success and failed"
    end

    unless missing.empty?
      warn "[openapi-contract] Publishing channel secret/CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_observability_provider_secret_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def response_data_array_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    expected_data_refs = {
      ["/api/v1/admin/observability/alert-routing", "get", "200"] => "#/components/schemas/AdminObservabilityAlertRoutingRules",
      ["/api/v1/admin/observability/alert-routing", "put", "200"] => "#/components/schemas/AdminObservabilityAlertRoutingRules",
      ["/api/v1/admin/observability/alert-providers", "post", "201"] => "#/components/schemas/AdminObservabilityAlertProvider",
      ["/api/v1/admin/observability/alert-providers/{providerId}", "put", "200"] => "#/components/schemas/AdminObservabilityAlertProvider",
      ["/api/v1/admin/observability/alert-providers/{providerId}/test", "post", "200"] => "#/components/schemas/AdminObservabilityAlertProviderTestResult",
    }

    expected_data_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
      tags = op.fetch("tags", [])
      unless tags.include?("Admin") && tags.include?("Observability")
        missing << "#{method.upcase} #{path} must be tagged Admin and Observability"
      end
    end

    list = operation(paths, "/api/v1/admin/observability/alert-providers", "get", missing)
    unless response_data_array_ref(list, "200") == "#/components/schemas/AdminObservabilityAlertProvider"
      missing << "GET /api/v1/admin/observability/alert-providers 200 data must return AdminObservabilityAlertProvider[]"
    end
    unless list.fetch("tags", []).include?("Admin") && list.fetch("tags", []).include?("Observability")
      missing << "GET /api/v1/admin/observability/alert-providers must be tagged Admin and Observability"
    end

    [
      ["/api/v1/admin/observability/alert-routing", "get"],
      ["/api/v1/admin/observability/alert-providers", "get"],
      ["/api/v1/admin/observability/alerts", "get"],
      ["/api/v1/admin/observability/alerts/{alertKey}", "get"],
      ["/api/v1/admin/observability/alerts/{alertKey}/deliveries", "get"],
      ["/api/v1/admin/observability/recovery-actions", "get"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      tags = op.fetch("tags", [])
      unless tags.include?("Admin") && tags.include?("Observability")
        missing << "#{method.upcase} #{path} must be tagged Admin and Observability"
      end
    end

    {
      ["/api/v1/admin/observability/alert-routing", "put"] => "#/components/schemas/UpdateAdminObservabilityAlertRoutingRulesRequest",
      ["/api/v1/admin/observability/alert-providers", "post"] => "#/components/schemas/AdminObservabilityAlertProviderRequest",
      ["/api/v1/admin/observability/alert-providers/{providerId}", "put"] => "#/components/schemas/AdminObservabilityAlertProviderRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    [
      ["/api/v1/admin/observability/alert-routing", "put"],
      ["/api/v1/admin/observability/alert-providers", "post"],
      ["/api/v1/admin/observability/alert-providers/{providerId}", "put"],
      ["/api/v1/admin/observability/alert-providers/{providerId}/test", "post"],
      ["/api/v1/admin/observability/alerts/{alertKey}/acknowledge", "post"],
      ["/api/v1/admin/observability/alerts/{alertKey}/resolve", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
    end

    config = schemas["AdminObservabilityAlertProviderConfig"] || {}
    description = config.fetch("description", "")
    unless config["type"] == "object" &&
        config.dig("additionalProperties", "type") == "string" &&
        ["password", "secret", "token", "webhook_url", "routing_key", "api_key", "private_key"].all? { |word| description.include?(word) } &&
        description.include?("********") &&
        description.include?("preserved")
      missing << "AdminObservabilityAlertProviderConfig must document credential input, redaction, and redacted-marker preservation"
    end

    provider = schemas["AdminObservabilityAlertProvider"] || {}
    props = provider.fetch("properties", {})
    ["id", "name", "createdAt", "updatedAt"].each do |property|
      missing << "AdminObservabilityAlertProvider.#{property} must be documented" unless props.key?(property)
    end
    unless props.dig("kind", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderKind" &&
        props.dig("channel", "$ref") == "#/components/schemas/AdminObservabilityAlertDeliveryChannel" &&
        props.dig("status", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderStatus" &&
        props.dig("config", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderConfig"
      missing << "AdminObservabilityAlertProvider must document kind/channel/status/config refs"
    end

    request = schemas["AdminObservabilityAlertProviderRequest"] || {}
    unless request.dig("properties", "config", "$ref") == "#/components/schemas/AdminObservabilityAlertProviderConfig"
      missing << "AdminObservabilityAlertProviderRequest.config must reference AdminObservabilityAlertProviderConfig"
    end

    unless missing.empty?
      warn "[openapi-contract] Admin Observability provider secret/CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_mcp_auth_token_response_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def response_data_array_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    list = operation(paths, "/api/v1/app/mcp-servers", "get", missing)
    unless response_data_array_ref(list, "200") == "#/components/schemas/McpServer"
      missing << "GET /api/v1/app/mcp-servers 200 data must return McpServer[]"
    end

    expected_data_refs = {
      ["/api/v1/app/mcp-servers", "post", "201"] => "#/components/schemas/McpServer",
      ["/api/v1/app/mcp-servers/{serverId}", "get", "200"] => "#/components/schemas/McpServer",
      ["/api/v1/app/mcp-servers/{serverId}", "delete", "200"] => "#/components/schemas/McpActionStatus",
      ["/api/v1/app/mcp-servers/{serverId}/connect", "post", "200"] => "#/components/schemas/McpServer",
      ["/api/v1/app/mcp-servers/{serverId}/disconnect", "post", "200"] => "#/components/schemas/McpActionStatus",
      ["/api/v1/app/mcp-servers/{serverId}/status", "get", "200"] => "#/components/schemas/McpActionStatus",
      ["/api/v1/app/mcp-servers/{serverId}/execute", "post", "200"] => "#/components/schemas/McpToolResult",
    }

    expected_data_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
      unless op.fetch("tags", []).include?("MCP")
        missing << "#{method.upcase} #{path} must be tagged MCP"
      end
    end

    local = operation(paths, "/api/v1/app/mcp-local-servers", "get", missing)
    unless response_data_array_ref(local, "200") == "#/components/schemas/McpLocalServer"
      missing << "GET /api/v1/app/mcp-local-servers 200 data must return McpLocalServer[]"
    end
    tools = operation(paths, "/api/v1/app/mcp-servers/{serverId}/tools", "get", missing)
    unless response_data_array_ref(tools, "200") == "#/components/schemas/McpToolDefinition"
      missing << "GET /api/v1/app/mcp-servers/{serverId}/tools 200 data must return McpToolDefinition[]"
    end

    [
      ["/api/v1/app/mcp-local-servers", "get"],
      ["/api/v1/app/mcp-servers", "get"],
      ["/api/v1/app/mcp-servers/{serverId}", "get"],
      ["/api/v1/app/mcp-servers/{serverId}/tools", "get"],
      ["/api/v1/app/mcp-servers/{serverId}/status", "get"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("MCP")
        missing << "#{method.upcase} #{path} must be tagged MCP"
      end
    end

    {
      ["/api/v1/app/mcp-servers", "post"] => "#/components/schemas/AddMcpServerRequest",
      ["/api/v1/app/mcp-servers/{serverId}/execute", "post"] => "#/components/schemas/ExecuteMcpToolRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    [
      ["/api/v1/app/mcp-servers", "post"],
      ["/api/v1/app/mcp-servers/{serverId}", "delete"],
      ["/api/v1/app/mcp-servers/{serverId}/connect", "post"],
      ["/api/v1/app/mcp-servers/{serverId}/disconnect", "post"],
      ["/api/v1/app/mcp-servers/{serverId}/execute", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
    end

    server = schemas["McpServer"] || {}
    props = server.fetch("properties", {})
    if props.key?("authToken") || props.key?("auth_token")
      missing << "McpServer response schema must not expose authToken"
    end
    unless props.dig("hasAuthToken", "type") == "boolean" &&
        props.dig("hasAuthToken", "description").to_s.include?("raw token is not returned")
      missing << "McpServer.hasAuthToken must document raw-token redaction"
    end
    ["id", "organizationId", "userId", "name", "url", "status", "createdAt", "updatedAt"].each do |property|
      missing << "McpServer.#{property} must be documented" unless props.key?(property)
    end

    add = schemas["AddMcpServerRequest"] || {}
    auth_token = add.dig("properties", "authToken") || {}
    unless auth_token["type"] == "string" &&
        auth_token["format"] == "password" &&
        auth_token["writeOnly"] == true &&
        auth_token.fetch("description", "").include?("hasAuthToken")
      missing << "AddMcpServerRequest.authToken must be password writeOnly input and point responses to hasAuthToken"
    end

    unless missing.empty?
      warn "[openapi-contract] MCP auth-token response contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_marketplace_user_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    [
      ["/api/v1/marketplace/agents", "post"],
      ["/api/v1/marketplace/agents/{agentId}", "put"],
      ["/api/v1/marketplace/agents/{agentId}", "delete"],
      ["/api/v1/marketplace/agents/{agentId}/install", "post"],
      ["/api/v1/marketplace/agents/{agentId}/install", "delete"],
      ["/api/v1/marketplace/installs/{agentId}", "delete"],
      ["/api/v1/marketplace/agents/{agentId}/reviews", "post"],
      ["/api/v1/marketplace/agents/{agentId}/appeal", "post"],
      ["/api/v1/marketplace/agents/{agentId}/abuse-reports", "post"],
      ["/api/v1/marketplace/publisher/settlement-preferences", "put"],
      ["/api/v1/marketplace/templates", "post"],
      ["/api/v1/marketplace/templates/{templateId}/install", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Marketplace")
        missing << "#{method.upcase} #{path} must be tagged Marketplace"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Marketplace user mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_marketplace_governance_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    [
      ["/api/v1/admin/marketplace/agents/{agentId}/takedown", "post"],
      ["/api/v1/admin/marketplace/agents/{agentId}/reinstate", "post"],
      ["/api/v1/admin/marketplace/abuse-reports/{reportId}/resolve", "post"],
      ["/api/v1/admin/marketplace/abuse-reports/{reportId}/dismiss", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      tags = op.fetch("tags", [])
      unless tags.include?("Admin") && tags.include?("Marketplace")
        missing << "#{method.upcase} #{path} must be tagged Admin and Marketplace"
      end
    end

    abuse_reports = operation(paths, "/api/v1/admin/marketplace/abuse-reports", "get", missing)
    unless requires_cookie_without_csrf?(abuse_reports)
      missing << "GET /api/v1/admin/marketplace/abuse-reports must require cookieAuth without csrfHeader"
    end
    abuse_tags = abuse_reports.fetch("tags", [])
    unless abuse_tags.include?("Admin") && abuse_tags.include?("Marketplace")
      missing << "GET /api/v1/admin/marketplace/abuse-reports must be tagged Admin and Marketplace"
    end

    unless missing.empty?
      warn "[openapi-contract] Admin Marketplace governance CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_marketplace_review_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def schema_props(schema)
      schema.fetch("properties", {})
    end

    [
      ["/api/v1/admin/reviews/sla/enforce", "post"],
      ["/api/v1/admin/reviews/{agentId}/approve", "post"],
      ["/api/v1/admin/reviews/{agentId}/reject", "post"],
      ["/api/v1/admin/reviews/{agentId}/needs-changes", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      tags = op.fetch("tags", [])
      unless tags.include?("Admin") && tags.include?("Marketplace")
        missing << "#{method.upcase} #{path} must be tagged Admin and Marketplace"
      end
    end

    expected_response_refs = {
      ["/api/v1/admin/reviews", "get"] => "#/components/schemas/AdminReviewListResponse",
      ["/api/v1/admin/reviews/sla/enforce", "post"] => "#/components/schemas/MarketplaceReviewSLAEnforcementResult",
      ["/api/v1/admin/reviews/{agentId}/approve", "post"] => "#/components/schemas/MarketplaceReviewStatusResponse",
      ["/api/v1/admin/reviews/{agentId}/reject", "post"] => "#/components/schemas/MarketplaceReviewStatusResponse",
      ["/api/v1/admin/reviews/{agentId}/needs-changes", "post"] => "#/components/schemas/MarketplaceReviewStatusResponse",
    }
    expected_response_refs.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, "200") == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
      if method == "get" && !requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
    end

    {
      ["/api/v1/admin/reviews/{agentId}/reject", "post"] => "#/components/schemas/MarketplaceReviewDecisionRequest",
      ["/api/v1/admin/reviews/{agentId}/needs-changes", "post"] => "#/components/schemas/MarketplaceReviewDecisionRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless op.dig("requestBody", "required") == true && request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must require #{expected}"
      end
    end

    schemas = spec.fetch("components", {}).fetch("schemas", {})
    review_list = schema_props(schemas["AdminReviewListResponse"] || {})
    unless review_list.dig("reviews", "type") == "array" &&
        review_list.dig("reviews", "items", "$ref") == "#/components/schemas/MarketplacePublishedAgent" &&
        review_list.dig("total", "type") == "integer"
      missing << "AdminReviewListResponse must expose MarketplacePublishedAgent reviews[] plus integer total"
    end

    decision = schema_props(schemas["MarketplaceReviewDecisionRequest"] || {})
    unless decision.dig("reason", "type") == "string"
      missing << "MarketplaceReviewDecisionRequest.reason must be documented as string"
    end

    status = schema_props(schemas["MarketplaceReviewStatusResponse"] || {})
    unless status.dig("status", "type") == "string" &&
        status.dig("status", "enum")&.sort == ["approved", "needs_changes", "rejected"]
      missing << "MarketplaceReviewStatusResponse.status must enumerate approved, rejected, and needs_changes"
    end

    sla = schema_props(schemas["MarketplaceReviewSLAEnforcementResult"] || {})
    unless sla.dig("scanned", "type") == "integer" && sla.dig("alerted", "type") == "integer"
      missing << "MarketplaceReviewSLAEnforcementResult must expose integer scanned and alerted counts"
    end

    unless missing.empty?
      warn "[openapi-contract] Admin Marketplace review CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_agent_run_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    def any_of_requires?(schema, field)
      schema.fetch("anyOf", []).any? { |entry| entry.fetch("required", []).include?(field) }
    end

    def all_of_any_requires?(schema, field)
      schema.fetch("allOf", []).any? { |entry| any_of_requires?(entry, field) }
    end

    [
      ["/api/v1/agent/runs", "post"],
      ["/api/v1/agent/runs/{runId}/approve-tool", "post"],
      ["/api/v1/agent/runs/{runId}/reject-tool", "post"],
      ["/api/v1/agent/runs/{runId}/retry-tool", "post"],
      ["/api/v1/agent/runs/{runId}/continue-budget", "post"],
      ["/api/v1/agent/runs/{runId}/adjust-plan", "post"],
      ["/api/v1/agent/runs/{runId}/continue-plan", "post"],
      ["/api/v1/agent/runs/{runId}/approve-plan-step", "post"],
      ["/api/v1/agent/runs/{runId}/execute-plan-step", "post"],
      ["/api/v1/agent/runs/{runId}/skip-plan-step", "post"],
      ["/api/v1/agent/runs/{runId}/retry-plan-step", "post"],
      ["/api/v1/agent/runs/{runId}/update-plan-step", "patch"],
      ["/api/v1/agent/runs/{runId}/create-plan-step", "post"],
      ["/api/v1/agent/runs/{runId}/move-plan-step", "post"],
      ["/api/v1/agent/runs/{runId}/delete-plan-step", "post"],
      ["/api/v1/app/agents/tool-runs/{toolRunId}/approve", "post"],
      ["/api/v1/app/agents/tool-runs/{toolRunId}/reject", "post"],
      ["/api/v1/app/agents/tool-runs/{toolRunId}/retry", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Agent")
        missing << "#{method.upcase} #{path} must be tagged Agent"
      end
    end

    [
      ["/api/v1/agent/tools", "get"],
      ["/api/v1/agent/runs/{runId}", "get"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Agent")
        missing << "#{method.upcase} #{path} must be tagged Agent"
      end
    end

    tools_op = operation(paths, "/api/v1/agent/tools", "get", missing)
    unless response_array_item_ref(tools_op, "200") == "#/components/schemas/AgentToolDefinition"
      missing << "GET /api/v1/agent/tools 200 data items must reference AgentToolDefinition"
    end

    {
      ["/api/v1/agent/runs", "post"] => "#/components/schemas/AgentRunCreateRequest",
      ["/api/v1/agent/runs/{runId}/approve-tool", "post"] => "#/components/schemas/AgentToolDecisionRequest",
      ["/api/v1/agent/runs/{runId}/reject-tool", "post"] => "#/components/schemas/AgentToolDecisionRequest",
      ["/api/v1/agent/runs/{runId}/retry-tool", "post"] => "#/components/schemas/AgentToolDecisionRequest",
      ["/api/v1/agent/runs/{runId}/continue-budget", "post"] => "#/components/schemas/AgentRunContinueBudgetRequest",
      ["/api/v1/agent/runs/{runId}/adjust-plan", "post"] => "#/components/schemas/AgentRunAdjustPlanRequest",
      ["/api/v1/agent/runs/{runId}/approve-plan-step", "post"] => "#/components/schemas/AgentPlanStepDecisionRequest",
      ["/api/v1/agent/runs/{runId}/execute-plan-step", "post"] => "#/components/schemas/AgentPlanStepDecisionRequest",
      ["/api/v1/agent/runs/{runId}/skip-plan-step", "post"] => "#/components/schemas/AgentPlanStepDecisionRequest",
      ["/api/v1/agent/runs/{runId}/retry-plan-step", "post"] => "#/components/schemas/AgentPlanStepDecisionRequest",
      ["/api/v1/agent/runs/{runId}/update-plan-step", "patch"] => "#/components/schemas/AgentPlanStepUpdateRequest",
      ["/api/v1/agent/runs/{runId}/create-plan-step", "post"] => "#/components/schemas/AgentPlanStepCreateRequest",
      ["/api/v1/agent/runs/{runId}/move-plan-step", "post"] => "#/components/schemas/AgentPlanStepMoveRequest",
      ["/api/v1/agent/runs/{runId}/delete-plan-step", "post"] => "#/components/schemas/AgentPlanStepDecisionRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    {
      ["/api/v1/agent/runs", "post", "201"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}", "get", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/approve-tool", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/reject-tool", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/retry-tool", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/continue-budget", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/adjust-plan", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/continue-plan", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/approve-plan-step", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/execute-plan-step", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/skip-plan-step", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/retry-plan-step", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/update-plan-step", "patch", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/create-plan-step", "post", "201"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/move-plan-step", "post", "200"] => "#/components/schemas/AgentRunResponse",
      ["/api/v1/agent/runs/{runId}/delete-plan-step", "post", "200"] => "#/components/schemas/AgentRunResponse",
    }.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    create = schemas["AgentRunCreateRequest"] || {}
    unless all_of_any_requires?(create, "agent_id") &&
        all_of_any_requires?(create, "agentId") &&
        all_of_any_requires?(create, "conversation_id") &&
        all_of_any_requires?(create, "conversationId") &&
        all_of_any_requires?(create, "input") &&
        all_of_any_requires?(create, "message") &&
        create.dig("properties", "mode", "enum")&.include?("planning") &&
        create.dig("properties", "max_iterations", "maximum") == 100 &&
        create.dig("properties", "maxIterations", "maximum") == 100 &&
        create.dig("properties", "token_budget", "maximum") == 1000000 &&
        create.dig("properties", "tokenBudget", "maximum") == 1000000
      missing << "AgentRunCreateRequest must document snake/camel agent, conversation, input/message, mode, and execution-limit controls"
    end

    continue_budget = schemas["AgentRunContinueBudgetRequest"] || {}
    unless any_of_requires?(continue_budget, "token_budget") &&
        any_of_requires?(continue_budget, "tokenBudget") &&
        continue_budget.dig("properties", "token_budget", "minimum") == 1000 &&
        continue_budget.dig("properties", "token_budget", "maximum") == 1000000 &&
        continue_budget.dig("properties", "tokenBudget", "minimum") == 1000 &&
        continue_budget.dig("properties", "tokenBudget", "maximum") == 1000000
      missing << "AgentRunContinueBudgetRequest must require snake/camel token budget aliases with bounded values"
    end

    run = schemas["AgentRun"] || {}
    run_response = schemas["AgentRunResponse"] || {}
    unless run.dig("properties", "mode", "enum")&.include?("planning") &&
        run.dig("properties", "status", "enum")&.include?("token_budget_exceeded") &&
        run_response.dig("properties", "run", "$ref") == "#/components/schemas/AgentRun" &&
        run_response.dig("properties", "toolRuns", "items", "$ref") == "#/components/schemas/AgentToolRun" &&
        run_response.dig("properties", "planSteps", "items", "$ref") == "#/components/schemas/AgentPlanStep" &&
        run_response.dig("properties", "messages", "items", "$ref") == "#/components/schemas/Message"
      missing << "AgentRunResponse must expose run, toolRuns, planSteps, messages, and planning/token-budget status fields"
    end

    unless schemas.dig("AgentToolDecisionRequest", "properties", "toolRunId", "type") == "string" &&
        schemas.dig("AgentToolDecisionRequest", "properties", "tool_run_id", "type") == "string" &&
        schemas.dig("AgentPlanStepDecisionRequest", "properties", "planStepId", "type") == "string" &&
        schemas.dig("AgentPlanStepDecisionRequest", "properties", "plan_step_id", "type") == "string" &&
        schemas.dig("AgentPlanStepMoveRequest", "properties", "direction", "enum")&.sort == ["down", "up"]
      missing << "Agent decision request schemas must document snake/camel identifiers and move direction enum"
    end

    tool_run = schemas["AgentToolRun"] || {}
    unless tool_run.dig("properties", "toolType", "type") == "string" &&
        tool_run.dig("properties", "serverId", "type") == "string" &&
        tool_run.dig("properties", "riskLevel", "type") == "string"
      missing << "AgentToolRun must document toolType, serverId, and riskLevel metadata for custom/MCP tool evidence"
    end

    plan_step = schemas["AgentPlanStep"] || {}
    plan_step_update = schemas["AgentPlanStepUpdateRequest"] || {}
    plan_step_create = schemas["AgentPlanStepCreateRequest"] || {}
    unless plan_step.dig("properties", "description", "type") == "string" &&
        plan_step.dig("properties", "dependsOn", "items", "type") == "integer" &&
        plan_step_update.dig("properties", "description", "type") == "string" &&
        plan_step_update.dig("properties", "dependsOn", "items", "minimum") == 1 &&
        plan_step_update.dig("properties", "depends_on", "items", "minimum") == 1 &&
        plan_step_create.dig("properties", "description", "type") == "string" &&
        plan_step_create.dig("properties", "dependsOn", "items", "minimum") == 1 &&
        plan_step_create.dig("properties", "depends_on", "items", "minimum") == 1
      missing << "Agent plan-step schemas must document structured description and dependsOn fields for response, update, and create requests"
    end

    unless missing.empty?
      warn "[openapi-contract] Agent run mutation CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_workspace_agent_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_data_array_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    mutation_paths = [
      ["/api/v1/app/agents", "post"],
      ["/api/v1/app/agents/{agentId}", "put"],
      ["/api/v1/app/agents/{agentId}", "delete"],
      ["/api/v1/app/agents/{agentId}/conversations", "post"],
      ["/api/v1/app/agents/conversations/{conversationId}", "delete"],
      ["/api/v1/app/agents/conversations/{conversationId}/messages", "post"],
    ]

    mutation_paths.each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Agent")
        missing << "#{method.upcase} #{path} must be tagged Agent"
      end
    end

    {
      ["/api/v1/app/agents", "get", "200"] => ["#/components/schemas/AgentWorkspaceAgent", :array_ref],
      ["/api/v1/app/agents/{agentId}", "get", "200"] => ["#/components/schemas/AgentWorkspaceAgent", :ref],
      ["/api/v1/app/agents/{agentId}/tools", "get", "200"] => ["#/components/schemas/AgentToolDefinition", :array_ref],
      ["/api/v1/app/agents/{agentId}/conversations", "get", "200"] => ["#/components/schemas/AgentConversation", :array_ref],
      ["/api/v1/app/agents/conversations/{conversationId}", "get", "200"] => ["#/components/schemas/AgentConversation", :ref],
      ["/api/v1/app/agents/conversations/{conversationId}/messages", "get", "200"] => ["#/components/schemas/AgentMessage", :array_ref],
      ["/api/v1/app/agents/conversations/{conversationId}/runs", "get", "200"] => ["#/components/schemas/AgentRun", :array_ref],
      ["/api/v1/app/agents/runs/{runId}", "get", "200"] => ["#/components/schemas/AgentRunDetail", :ref],
    }.each do |(path, method, status), (expected, shape)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Agent")
        missing << "#{method.upcase} #{path} must be tagged Agent"
      end
      actual = shape == :array_ref ? response_data_array_ref(op, status) : response_data_ref(op, status)
      unless actual == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/agents", "post"] => "#/components/schemas/CreateAgentRequest",
      ["/api/v1/app/agents/{agentId}", "put"] => "#/components/schemas/UpdateAgentRequest",
      ["/api/v1/app/agents/conversations/{conversationId}/messages", "post"] => "#/components/schemas/AgentSendMessageRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/agents", "post", "201"] => "#/components/schemas/AgentWorkspaceAgent",
      ["/api/v1/app/agents/{agentId}", "put", "200"] => "#/components/schemas/AgentWorkspaceAgent",
      ["/api/v1/app/agents/{agentId}", "delete", "200"] => "#/components/schemas/AgentDeleteStatusResponse",
      ["/api/v1/app/agents/{agentId}/conversations", "post", "201"] => "#/components/schemas/AgentConversation",
      ["/api/v1/app/agents/conversations/{conversationId}", "delete", "200"] => "#/components/schemas/AgentDeleteStatusResponse",
      ["/api/v1/app/agents/conversations/{conversationId}/messages", "post", "200"] => "#/components/schemas/AgentMessage",
    }.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    unless schemas.dig("AgentDeleteStatusResponse", "properties", "status", "type") == "string"
      missing << "AgentDeleteStatusResponse must expose status string"
    end
    unless schemas.dig("AgentSendMessageRequest", "properties", "content", "type") == "string" &&
        schemas.dig("AgentSendMessageRequest", "properties", "mode", "enum")&.include?("planning") &&
        schemas.dig("AgentSendMessageRequest", "properties", "max_iterations", "type") == "integer" &&
        schemas.dig("AgentSendMessageRequest", "properties", "maxIterations", "type") == "integer" &&
        schemas.dig("AgentSendMessageRequest", "properties", "token_budget", "type") == "integer" &&
        schemas.dig("AgentSendMessageRequest", "properties", "tokenBudget", "type") == "integer"
      missing << "AgentSendMessageRequest must document content, mode, and snake/camel budget controls"
    end
    unless schemas.dig("CreateAgentRequest", "properties", "config", "$ref") == "#/components/schemas/AgentConfig" &&
        schemas.dig("UpdateAgentRequest", "properties", "config", "$ref") == "#/components/schemas/AgentConfig" &&
        schemas.dig("AgentConfig", "properties", "defaultExecutionMode", "enum")&.include?("planning") &&
        schemas.dig("AgentConfig", "properties", "longTermMemoryExtractionPolicy", "enum")&.include?("llm_assisted") &&
        schemas.dig("AgentConfig", "properties", "longTermMemoryUpdatePolicy", "enum")&.include?("memory_key_consolidate") &&
        schemas.dig("AgentConfig", "properties", "longTermMemoryWritePolicy", "enum")&.include?("manual_only") &&
        schemas.dig("AgentConfig", "properties", "modelRoutingRules", "items", "$ref") == "#/components/schemas/AgentModelRoutingRule" &&
        schemas.dig("AgentConfig", "properties", "skills", "items", "$ref") == "#/components/schemas/AgentSkill" &&
        schemas.dig("AgentConfig", "properties", "maxSkills", "type") == "integer"
      missing << "Agent create/update request schemas must reference AgentConfig with execution, memory, model routing, and skill policies"
    end
    unless schemas.dig("AgentModelRoutingRule", "required")&.include?("targetModel") &&
        schemas.dig("AgentModelRoutingRule", "properties", "targetModel", "type") == "string" &&
        schemas.dig("AgentModelRoutingRule", "properties", "keywords", "items", "type") == "string" &&
        schemas.dig("AgentSkill", "required")&.include?("name") &&
        schemas.dig("AgentSkill", "properties", "toolNames", "items", "type") == "string"
      missing << "Agent runtime config schemas must document model routing rules and skill bundles"
    end

    unless missing.empty?
      warn "[openapi-contract] Workspace Agent mutation CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_memory_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    def schema_props(schema)
      schema.fetch("properties", {})
    end

    {
      ["/api/v1/app/memory/documents", "get"] => "Memory",
      ["/api/v1/app/memory/documents/{documentId}", "get"] => "Memory",
      ["/api/v1/app/memory/documents/{documentId}/chunks", "get"] => "Memory",
      ["/api/v1/agent/memories", "get"] => "Agent",
      ["/api/v1/agent/memories/export", "get"] => "Agent",
    }.each do |(path, method), expected_tag|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?(expected_tag)
        missing << "#{method.upcase} #{path} must be tagged #{expected_tag}"
      end
    end

    {
      ["/api/v1/app/memory/documents", "post"] => "Memory",
      ["/api/v1/app/memory/documents/{documentId}", "put"] => "Memory",
      ["/api/v1/app/memory/documents/{documentId}", "delete"] => "Memory",
      ["/api/v1/app/memory/search", "post"] => "Memory",
      ["/api/v1/agent/memories", "post"] => "Agent",
      ["/api/v1/agent/memories/import", "post"] => "Agent",
      ["/api/v1/agent/memories/{memoryId}", "patch"] => "Agent",
      ["/api/v1/agent/memories/{memoryId}", "delete"] => "Agent",
    }.each do |(path, method), expected_tag|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?(expected_tag)
        missing << "#{method.upcase} #{path} must be tagged #{expected_tag}"
      end
    end

    {
      ["/api/v1/app/memory/documents", "post"] => "#/components/schemas/MemoryDocumentRequest",
      ["/api/v1/app/memory/documents/{documentId}", "put"] => "#/components/schemas/UpdateMemoryDocumentRequest",
      ["/api/v1/app/memory/search", "post"] => "#/components/schemas/MemorySearchRequest",
      ["/api/v1/agent/memories", "post"] => "#/components/schemas/AgentMemoryRequest",
      ["/api/v1/agent/memories/import", "post"] => "#/components/schemas/AgentMemoryImportRequest",
      ["/api/v1/agent/memories/{memoryId}", "patch"] => "#/components/schemas/AgentMemoryUpdateRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/memory/documents", "post", "201"] => "#/components/schemas/MemoryDocument",
      ["/api/v1/app/memory/documents/{documentId}", "put", "200"] => "#/components/schemas/MemoryDocument",
      ["/api/v1/app/memory/documents/{documentId}", "delete", "200"] => "#/components/schemas/MemoryDeleteStatusResponse",
      ["/api/v1/agent/memories", "post", "201"] => "#/components/schemas/AgentMemory",
      ["/api/v1/agent/memories/{memoryId}", "patch", "200"] => "#/components/schemas/AgentMemory",
    }.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    search_op = operation(paths, "/api/v1/app/memory/search", "post", missing)
    unless response_array_item_ref(search_op, "200") == "#/components/schemas/MemorySearchResult"
      missing << "POST /api/v1/app/memory/search 200 data items must reference MemorySearchResult"
    end

    list_memory_documents_op = operation(paths, "/api/v1/app/memory/documents", "get", missing)
    unless response_array_item_ref(list_memory_documents_op, "200") == "#/components/schemas/MemoryDocument"
      missing << "GET /api/v1/app/memory/documents 200 data items must reference MemoryDocument"
    end

    get_memory_document_op = operation(paths, "/api/v1/app/memory/documents/{documentId}", "get", missing)
    unless response_data_ref(get_memory_document_op, "200") == "#/components/schemas/MemoryDocument"
      missing << "GET /api/v1/app/memory/documents/{documentId} 200 data must reference MemoryDocument"
    end

    list_memory_document_chunks_op = operation(paths, "/api/v1/app/memory/documents/{documentId}/chunks", "get", missing)
    unless response_array_item_ref(list_memory_document_chunks_op, "200") == "#/components/schemas/MemoryChunk"
      missing << "GET /api/v1/app/memory/documents/{documentId}/chunks 200 data items must reference MemoryChunk"
    end

    list_agent_memories_op = operation(paths, "/api/v1/agent/memories", "get", missing)
    unless response_array_item_ref(list_agent_memories_op, "200") == "#/components/schemas/AgentMemory"
      missing << "GET /api/v1/agent/memories 200 data items must reference AgentMemory"
    end

    export_agent_memories_op = operation(paths, "/api/v1/agent/memories/export", "get", missing)
    unless response_data_ref(export_agent_memories_op, "200") == "#/components/schemas/AgentMemoryListResponse"
      missing << "GET /api/v1/agent/memories/export 200 data must reference AgentMemoryListResponse"
    end

    import_op = operation(paths, "/api/v1/agent/memories/import", "post", missing)
    unless response_array_item_ref(import_op, "201") == "#/components/schemas/AgentMemory"
      missing << "POST /api/v1/agent/memories/import 201 data items must reference AgentMemory"
    end

    delete_op = operation(paths, "/api/v1/agent/memories/{memoryId}", "delete", missing)
    unless delete_op.dig("responses", "204", "description")
      missing << "DELETE /api/v1/agent/memories/{memoryId} must document 204 delete response"
    end

    unless schemas.dig("MemoryDeleteStatusResponse", "properties", "status", "type") == "string"
      missing << "MemoryDeleteStatusResponse must expose status string"
    end
    unless schemas.dig("AgentMemoryUpdateRequest", "properties", "content", "type") == "string" &&
        schemas.dig("AgentMemoryUpdateRequest", "properties", "importance", "type") == "integer" &&
        schemas.dig("AgentMemoryUpdateRequest", "properties", "importance", "minimum") == 1 &&
        schemas.dig("AgentMemoryUpdateRequest", "properties", "importance", "maximum") == 5
      missing << "AgentMemoryUpdateRequest must document content and bounded importance"
    end
    unless schemas.dig("MemorySearchRequest", "properties", "query", "type") == "string" &&
        schemas.dig("AgentMemoryRequest", "properties", "content", "type") == "string" &&
        schemas.dig("AgentMemoryRequest", "properties", "agentId", "type") == "string" &&
        schemas.dig("AgentMemoryRequest", "properties", "agent_id", "type") == "string" &&
        schemas.dig("AgentMemoryImportRequest", "properties", "memories", "items", "$ref") == "#/components/schemas/AgentMemoryRequest"
      missing << "Memory and Agent memory request schemas must expose query/content and import item refs"
    end

    unless missing.empty?
      warn "[openapi-contract] Memory mutation CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_billing_checkout_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    def schema_props(schema)
      schema.fetch("properties", {})
    end

    checkout = paths.dig("/api/v1/billing/checkout", "post")
    unless checkout
      missing << "POST /api/v1/billing/checkout must be documented"
      checkout = {}
    end

    unless checkout.fetch("tags", []).include?("Billing")
      missing << "POST /api/v1/billing/checkout must be tagged Billing"
    end
    unless requires_cookie_and_csrf?(checkout)
      missing << "POST /api/v1/billing/checkout must require cookieAuth and csrfHeader"
    end
    unless checkout.dig("requestBody", "required") == true &&
        request_body_ref(checkout) == "#/components/schemas/BillingCheckoutRequest"
      missing << "POST /api/v1/billing/checkout request body must require BillingCheckoutRequest"
    end
    unless response_data_ref(checkout, "201") == "#/components/schemas/BillingCheckoutSession"
      missing << "POST /api/v1/billing/checkout 201 data must reference BillingCheckoutSession"
    end
    ["501", "502"].each do |status|
      unless checkout.dig("responses", status, "content", "application/json", "schema", "$ref") == "#/components/schemas/Envelope"
        missing << "POST /api/v1/billing/checkout #{status} response must reference Envelope"
      end
    end

    console_billing = paths.dig("/api/v1/console/billing", "get")
    unless console_billing
      missing << "GET /api/v1/console/billing must be documented"
      console_billing = {}
    end
    unless response_data_ref(console_billing, "200") == "#/components/schemas/BillingSummary"
      missing << "GET /api/v1/console/billing 200 data must reference BillingSummary"
    end

    console_usage = paths.dig("/api/v1/console/usage", "get")
    unless console_usage
      missing << "GET /api/v1/console/usage must be documented"
      console_usage = {}
    end
    unless response_data_ref(console_usage, "200") == "#/components/schemas/UsageSummary"
      missing << "GET /api/v1/console/usage 200 data must reference UsageSummary"
    end

    console_access = paths.dig("/api/v1/console/access", "get")
    unless console_access
      missing << "GET /api/v1/console/access must be documented"
      console_access = {}
    end
    unless response_data_ref(console_access, "200") == "#/components/schemas/AccessSummary"
      missing << "GET /api/v1/console/access 200 data must reference AccessSummary"
    end

    console_models = paths.dig("/api/v1/console/models", "get")
    unless console_models
      missing << "GET /api/v1/console/models must be documented"
      console_models = {}
    end
    unless response_array_item_ref(console_models, "200") == "#/components/schemas/ModelSummary"
      missing << "GET /api/v1/console/models 200 data items must reference ModelSummary"
    end

    console_invoices = paths.dig("/api/v1/console/invoices", "get")
    unless console_invoices
      missing << "GET /api/v1/console/invoices must be documented"
      console_invoices = {}
    end
    unless response_array_item_ref(console_invoices, "200") == "#/components/schemas/BillingInvoiceSummary"
      missing << "GET /api/v1/console/invoices 200 data items must reference BillingInvoiceSummary"
    end

    console_api_tokens = paths.dig("/api/v1/console/api-tokens", "get")
    unless console_api_tokens
      missing << "GET /api/v1/console/api-tokens must be documented"
      console_api_tokens = {}
    end
    unless response_array_item_ref(console_api_tokens, "200") == "#/components/schemas/RelayAPIToken"
      missing << "GET /api/v1/console/api-tokens 200 data items must reference RelayAPIToken"
    end

    console_api_token_usage = paths.dig("/api/v1/console/api-tokens/{tokenId}/usage", "get")
    unless console_api_token_usage
      missing << "GET /api/v1/console/api-tokens/{tokenId}/usage must be documented"
      console_api_token_usage = {}
    end
    unless response_array_item_ref(console_api_token_usage, "200") == "#/components/schemas/ConsoleAPITokenUsageItem"
      missing << "GET /api/v1/console/api-tokens/{tokenId}/usage 200 data items must reference ConsoleAPITokenUsageItem"
    end

    {
      "GET /api/v1/console/usage" => console_usage,
      "GET /api/v1/console/access" => console_access,
      "GET /api/v1/console/models" => console_models,
      "GET /api/v1/console/billing" => console_billing,
      "GET /api/v1/console/invoices" => console_invoices,
      "GET /api/v1/console/api-tokens" => console_api_tokens,
      "GET /api/v1/console/api-tokens/{tokenId}/usage" => console_api_token_usage,
    }.each do |route, op|
      unless requires_cookie_without_csrf?(op)
        missing << "#{route} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Billing")
        missing << "#{route} must be tagged Billing"
      end
    end

    model_summary = schemas["ModelSummary"] || {}
    unless model_summary.fetch("required", []).include?("id") &&
        model_summary.fetch("required", []).include?("label") &&
        model_summary.fetch("required", []).include?("requests") &&
        model_summary.dig("properties", "id", "type") == "string" &&
        model_summary.dig("properties", "label", "type") == "string" &&
        model_summary.dig("properties", "requests", "type") == "integer" &&
        model_summary.dig("properties", "requests", "minimum") == 0
      missing << "ModelSummary must require id, label, and non-negative integer requests"
    end

    billing_summary = schemas["BillingSummary"] || {}
    unless billing_summary.dig("properties", "paymentProviders", "items", "$ref") == "#/components/schemas/BillingPaymentProvider"
      missing << "BillingSummary.paymentProviders must reference BillingPaymentProvider"
    end
    provider = schemas["BillingPaymentProvider"] || {}
    unless provider.fetch("required", []).include?("name") &&
        provider.dig("properties", "name", "enum") == ["stripe", "alipay", "wechatpay"]
      missing << "BillingPaymentProvider.name must require and enumerate stripe, alipay, and wechatpay"
    end
    invoice = schemas["BillingInvoiceSummary"] || {}
    invoice_props = schema_props(invoice)
    unless invoice_props.dig("hostedInvoiceUrl", "type") == "string" &&
        invoice_props.dig("hostedInvoiceUrl", "format") == "uri" &&
        invoice_props.dig("invoicePdf", "type") == "string" &&
        invoice_props.dig("invoicePdf", "format") == "uri"
      missing << "BillingInvoiceSummary must document hostedInvoiceUrl and invoicePdf URI fields"
    end

    usage_summary = schemas["UsageSummary"] || {}
    usage_props = schema_props(usage_summary)
    unless usage_summary.fetch("required", []).include?("period") &&
        usage_summary.fetch("required", []).include?("requests") &&
        usage_props.dig("period", "type") == "string" &&
        usage_props.dig("requests", "type") == "integer" &&
        usage_props.dig("byModel", "items", "$ref") == "#/components/schemas/UsageDimensionSummary" &&
        usage_props.dig("byFeature", "items", "$ref") == "#/components/schemas/UsageDimensionSummary" &&
        usage_props.dig("byUser", "items", "$ref") == "#/components/schemas/UsageDimensionSummary" &&
        usage_props.dig("timeSeries", "items", "$ref") == "#/components/schemas/UsageTimeSeriesSummary" &&
        usage_props.dig("recent", "items", "$ref") == "#/components/schemas/ConsoleAPITokenUsageItem"
      missing << "UsageSummary must document period, requests, usage dimensions, time series, and recent token usage"
    end

    dimension = schema_props(schemas["UsageDimensionSummary"] || {})
    unless dimension.dig("key", "type") == "string" &&
        dimension.dig("requestCount", "type") == "integer" &&
        dimension.dig("totalTokens", "type") == "integer" &&
        dimension.dig("totalCost", "type") == "number"
      missing << "UsageDimensionSummary must document key, requestCount, totalTokens, and totalCost"
    end

    series = schema_props(schemas["UsageTimeSeriesSummary"] || {})
    unless series.dig("bucket", "type") == "string" &&
        series.dig("requestCount", "type") == "integer" &&
        series.dig("totalTokens", "type") == "integer" &&
        series.dig("totalCost", "type") == "number"
      missing << "UsageTimeSeriesSummary must document bucket, requestCount, totalTokens, and totalCost"
    end

    access = schemas["AccessSummary"] || {}
    access_props = schema_props(access)
    [
      ["defaultMode", "string"],
      ["modelStrategy", "string"],
      ["networkEnabledHint", "boolean"],
      ["onboardingCompleted", "boolean"],
      ["sessionExpiresAt", "string"],
      ["sessionId", "string"],
      ["userEmail", "string"],
      ["userId", "string"],
      ["workspaceId", "string"],
    ].each do |field, type|
      unless access.fetch("required", []).include?(field) && access_props.dig(field, "type") == type
        missing << "AccessSummary must require #{field} as #{type}"
      end
    end

    token = schemas["RelayAPIToken"] || {}
    if token.fetch("properties", {}).key?("rawToken")
      missing << "RelayAPIToken list schema must not expose rawToken"
    end
    token_usage = schema_props(schemas["ConsoleAPITokenUsageItem"] || {})
    [
      ["apiTokenId", "string"],
      ["requestId", "string"],
      ["apiType", "string"],
      ["model", "string"],
      ["status", "string"],
      ["statusCode", "integer"],
      ["latencyMs", "integer"],
      ["cost", "number"],
      ["promptTokens", "integer"],
      ["completionTokens", "integer"],
      ["totalTokens", "integer"],
      ["createdAt", "string"],
    ].each do |field, type|
      unless token_usage.dig(field, "type") == type
        missing << "ConsoleAPITokenUsageItem must document #{field} as #{type}"
      end
    end
    ["provider", "channelId", "channel_id"].each do |field|
      if token_usage.key?(field)
        missing << "ConsoleAPITokenUsageItem must not expose #{field}"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Billing checkout contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_quota_topup_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    missing = []

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    app_read_routes = {
      ["/api/v1/app/quota", "get"] => [:object, "#/components/schemas/QuotaSnapshot"],
      ["/api/v1/app/packages", "get"] => [:array, "#/components/schemas/PackageOption"],
    }
    app_read_routes.each do |(path, method), (shape, expected_response)|
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        op = {}
      end
      security = op.fetch("security", [])
      unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Billing")
        missing << "#{method.upcase} #{path} must be tagged Billing"
      end
      actual_response = shape == :array ? response_array_item_ref(op, "200") : response_data_ref(op, "200")
      unless actual_response == expected_response
        missing << "#{method.upcase} #{path} 200 data must reference #{expected_response}"
      end
    end

    operation = paths.dig("/api/v1/app/quota/topup", "post")
    unless operation
      missing << "POST /api/v1/app/quota/topup must be documented"
      operation = {}
    end

    security = operation.fetch("security", [])
    unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
      missing << "POST /api/v1/app/quota/topup must require cookieAuth and csrfHeader"
    end
    unless operation.fetch("tags", []).include?("Billing")
      missing << "POST /api/v1/app/quota/topup must be tagged Billing"
    end
    unless operation.dig("requestBody", "content", "application/json", "schema", "$ref") == "#/components/schemas/QuotaTopupRequest"
      missing << "POST /api/v1/app/quota/topup request body must reference QuotaTopupRequest"
    end
    unless operation.dig("responses", "402", "content", "application/json", "schema", "$ref") == "#/components/schemas/Envelope"
      missing << "POST /api/v1/app/quota/topup 402 response must reference Envelope"
    end
    unless schemas.dig("QuotaTopupRequest", "required")&.include?("amount") &&
        schemas.dig("QuotaTopupRequest", "properties", "amount", "type") == "number" &&
        schemas.dig("QuotaTopupRequest", "properties", "amount", "exclusiveMinimum") == 0
      missing << "QuotaTopupRequest must require a positive numeric amount"
    end

    unless missing.empty?
      warn "[openapi-contract] Quota top-up CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_tenant_organization_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    {
      ["/api/v1/app/organizations", "get"] => "#/components/schemas/OrganizationMembershipListResponse",
      ["/api/v1/app/organizations/{organizationId}/members", "get"] => "#/components/schemas/OrganizationMembersResponse",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Tenant")
        missing << "#{method.upcase} #{path} must be tagged Tenant"
      end
      unless response_data_ref(op, "200") == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
    end

    [
      ["/api/v1/app/organizations/{organizationId}/select", "post"],
      ["/api/v1/app/organizations/{organizationId}/members/{userId}", "put"],
      ["/api/v1/app/organizations/{organizationId}/members/{userId}", "delete"],
      ["/api/v1/app/organizations/{organizationId}/invitations", "post"],
      ["/api/v1/app/organizations/{organizationId}/invitations/{invitationId}/revoke", "post"],
      ["/api/v1/app/organizations/{organizationId}/ownership-transfer", "post"],
      ["/api/v1/app/organization-invitations/{token}/accept", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Tenant")
        missing << "#{method.upcase} #{path} must be tagged Tenant"
      end
    end

    {
      ["/api/v1/app/organizations/{organizationId}/members/{userId}", "put"] => "#/components/schemas/UpdateOrganizationMemberRoleRequest",
      ["/api/v1/app/organizations/{organizationId}/invitations", "post"] => "#/components/schemas/InviteOrganizationMemberRequest",
      ["/api/v1/app/organizations/{organizationId}/ownership-transfer", "post"] => "#/components/schemas/TransferOrganizationOwnershipRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless op.dig("requestBody", "required") == true && request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must require #{expected}"
      end
    end

    transfer = operation(paths, "/api/v1/app/organizations/{organizationId}/ownership-transfer", "post", missing)
    unless response_data_ref(transfer, "200") == "#/components/schemas/OrganizationOwnershipTransferResponse"
      missing << "POST /api/v1/app/organizations/{organizationId}/ownership-transfer 200 data must reference OrganizationOwnershipTransferResponse"
    end

    unless schemas.dig("OrganizationMembershipListResponse", "properties", "memberships", "items", "$ref") == "#/components/schemas/OrganizationMembership"
      missing << "OrganizationMembershipListResponse must expose memberships[]"
    end
    unless schemas.dig("OrganizationMembersResponse", "properties", "members", "items", "$ref") == "#/components/schemas/OrganizationMembership"
      missing << "OrganizationMembersResponse must expose members[]"
    end
    unless schemas.dig("OrganizationOwnershipTransferResponse", "properties", "transferred", "type") == "boolean"
      missing << "OrganizationOwnershipTransferResponse.transferred must be boolean"
    end

    unless missing.empty?
      warn "[openapi-contract] Tenant organization mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_workflow_execution_control_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "$ref")
    end

    [
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check", "post"],
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/decision", "post"],
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/pause", "post"],
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/resume", "post"],
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/cancel", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Workflow")
        missing << "#{method.upcase} #{path} must be tagged Workflow"
      end
      unless response_ref(op, "200") == "#/components/schemas/WorkflowExecutionEnvelope"
        missing << "#{method.upcase} #{path} 200 response must reference WorkflowExecutionEnvelope"
      end
    end

    {
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check", "post"] => "#/components/schemas/WorkflowResourceCheckRequest",
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/decision", "post"] => "#/components/schemas/WorkflowFailureDecisionRequest",
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/resume", "post"] => "#/components/schemas/WorkflowResumeExecutionRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Workflow execution control CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_workflow_management_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "$ref")
    end

    workflow_mutations = [
      ["/api/v1/workflows", "post"],
      ["/api/v1/workflows/semantic-matches", "post"],
      ["/api/v1/workflows/conversation-matches", "post"],
      ["/api/v1/workflows/{workflowId}", "put"],
      ["/api/v1/workflows/{workflowId}", "delete"],
      ["/api/v1/workflows/{workflowId}/execute", "post"],
      ["/api/v1/workflows/{workflowId}/webhook", "post"],
      ["/api/v1/workflows/{workflowId}/branches", "post"],
      ["/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "post"],
      ["/api/v1/workflows/{workflowId}/branches/{branchId}/merge", "post"],
      ["/api/v1/workflows/{workflowId}/rollback", "post"],
      ["/api/v1/workflows/{workflowId}/test-node", "post"],
    ]

    workflow_reads = {
      ["/api/v1/workflows", "get", "200"] => "#/components/schemas/WorkflowDefinitionsEnvelope",
      ["/api/v1/workflows/{workflowId}", "get", "200"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}/versions", "get", "200"] => "#/components/schemas/WorkflowDefinitionsEnvelope",
      ["/api/v1/workflows/{workflowId}/executions", "get", "200"] => "#/components/schemas/WorkflowExecutionsEnvelope",
      ["/api/v1/workflows/{workflowId}/executions/{executionId}", "get", "200"] => "#/components/schemas/WorkflowExecutionEnvelope",
      ["/api/v1/workflows/{workflowId}/executions/{executionId}/debug-snapshot", "get", "200"] => "#/components/schemas/WorkflowExecutionDebugSnapshotEnvelope",
    }

    workflow_reads.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Workflow")
        missing << "#{method.upcase} #{path} must be tagged Workflow"
      end
      unless response_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} response must reference #{expected}"
      end
    end

    workflow_mutations.each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Workflow")
        missing << "#{method.upcase} #{path} must be tagged Workflow"
      end
    end

    expected_request_refs = {
      ["/api/v1/workflows", "post"] => "#/components/schemas/CreateWorkflowRequest",
      ["/api/v1/workflows/semantic-matches", "post"] => "#/components/schemas/WorkflowSemanticMatchRequest",
      ["/api/v1/workflows/conversation-matches", "post"] => "#/components/schemas/WorkflowConversationMatchRequest",
      ["/api/v1/workflows/{workflowId}", "put"] => "#/components/schemas/UpdateWorkflowRequest",
      ["/api/v1/workflows/{workflowId}/execute", "post"] => "#/components/schemas/ExecuteWorkflowRequest",
      ["/api/v1/workflows/{workflowId}/branches", "post"] => "#/components/schemas/CreateWorkflowBranchRequest",
      ["/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "post"] => "#/components/schemas/PublishWorkflowBranchRequest",
      ["/api/v1/workflows/{workflowId}/rollback", "post"] => "#/components/schemas/RollbackWorkflowRequest",
      ["/api/v1/workflows/{workflowId}/test-node", "post"] => "#/components/schemas/WorkflowNodeTestRequest",
    }
    expected_request_refs.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    expected_response_refs = {
      ["/api/v1/workflows", "post", "201"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/semantic-matches", "post", "200"] => "#/components/schemas/WorkflowSemanticMatchesEnvelope",
      ["/api/v1/workflows/conversation-matches", "post", "200"] => "#/components/schemas/WorkflowConversationMatchesEnvelope",
      ["/api/v1/workflows/{workflowId}", "put", "200"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}", "delete", "200"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}/execute", "post", "201"] => "#/components/schemas/WorkflowExecutionEnvelope",
      ["/api/v1/workflows/{workflowId}/webhook", "post", "201"] => "#/components/schemas/WorkflowExecutionEnvelope",
      ["/api/v1/workflows/{workflowId}/branches", "post", "201"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "post", "201"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}/branches/{branchId}/merge", "post", "200"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}/rollback", "post", "200"] => "#/components/schemas/WorkflowDefinitionEnvelope",
      ["/api/v1/workflows/{workflowId}/test-node", "post", "200"] => "#/components/schemas/WorkflowNodeTestResultEnvelope",
    }
    expected_response_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} response must reference #{expected}"
      end
    end

    signed_webhook = operation(paths, "/api/v1/workflows/webhooks/{organizationId}/{workflowId}", "post", missing)
    unless signed_webhook["security"] == []
      missing << "POST /api/v1/workflows/webhooks/{organizationId}/{workflowId} must remain explicitly public with security: []"
    end
    unless signed_webhook.dig("responses", "201", "content", "application/json", "schema", "$ref") == "#/components/schemas/WorkflowExecutionEnvelope"
      missing << "POST /api/v1/workflows/webhooks/{organizationId}/{workflowId} 201 response must reference WorkflowExecutionEnvelope"
    end

    unless schemas.dig("CreateWorkflowRequest", "required")&.include?("name") &&
        schemas.dig("CreateWorkflowRequest", "required")&.include?("definition") &&
        schemas.dig("RollbackWorkflowRequest", "required")&.include?("version") &&
        schemas.dig("CreateWorkflowBranchRequest", "required")&.include?("name") &&
        schemas.dig("CreateWorkflowBranchRequest", "required")&.include?("version")
      missing << "Workflow create, rollback, and branch request schemas must preserve required fields"
    end

    unless missing.empty?
      warn "[openapi-contract] Workflow management CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_console_api_token_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    list = operation(paths, "/api/v1/console/api-tokens", "get", missing)
    create = operation(paths, "/api/v1/console/api-tokens", "post", missing)
    revoke = operation(paths, "/api/v1/console/api-tokens/{tokenId}", "delete", missing)
    usage = operation(paths, "/api/v1/console/api-tokens/{tokenId}/usage", "get", missing)

    {
      "POST /api/v1/console/api-tokens" => create,
      "DELETE /api/v1/console/api-tokens/{tokenId}" => revoke,
    }.each do |label, op|
      unless requires_cookie_and_csrf?(op)
        missing << "#{label} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Billing")
        missing << "#{label} must be tagged Billing"
      end
    end

    {
      "GET /api/v1/console/api-tokens" => list,
      "GET /api/v1/console/api-tokens/{tokenId}/usage" => usage,
    }.each do |label, op|
      unless requires_cookie_without_csrf?(op)
        missing << "#{label} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Billing")
        missing << "#{label} must be tagged Billing"
      end
    end

    unless response_array_item_ref(list, "200") == "#/components/schemas/RelayAPIToken"
      missing << "GET /api/v1/console/api-tokens 200 data items must reference RelayAPIToken"
    end
    unless response_array_item_ref(usage, "200") == "#/components/schemas/ConsoleAPITokenUsageItem"
      missing << "GET /api/v1/console/api-tokens/{tokenId}/usage 200 data items must reference ConsoleAPITokenUsageItem"
    end
    unless create.dig("requestBody", "required") == true &&
        request_body_ref(create) == "#/components/schemas/CreateRelayAPITokenRequest"
      missing << "POST /api/v1/console/api-tokens request body must require CreateRelayAPITokenRequest"
    end
    unless response_data_ref(create, "201") == "#/components/schemas/CreatedRelayAPIToken"
      missing << "POST /api/v1/console/api-tokens 201 data must reference CreatedRelayAPIToken"
    end
    unless response_data_ref(revoke, "200") == "#/components/schemas/RelayAPITokenRevokeResponse"
      missing << "DELETE /api/v1/console/api-tokens/{tokenId} 200 data must reference RelayAPITokenRevokeResponse"
    end

    revoke_schema = schemas["RelayAPITokenRevokeResponse"] || {}
    unless revoke_schema.fetch("required", []).include?("status") &&
        revoke_schema.dig("properties", "status", "type") == "string" &&
        revoke_schema.dig("properties", "status", "enum") == ["revoked"]
      missing << "RelayAPITokenRevokeResponse.status must be required and enumerate revoked"
    end

    relay_token_props = schemas.fetch("RelayAPIToken", {}).fetch("properties", {})
    ["id", "name", "tokenPrefix", "status", "modelLimitsEnabled", "modelLimits", "usedQuota", "createdAt"].each do |field|
      missing << "RelayAPIToken.#{field} must be documented for console list responses" unless relay_token_props.key?(field)
    end
    ["rawToken", "tokenHash", "token_hash"].each do |field|
      missing << "RelayAPIToken list schema must not expose #{field}" if relay_token_props.key?(field)
    end

    created_props = schemas.fetch("CreatedRelayAPIToken", {}).fetch("properties", {})
    unless created_props.key?("rawToken") && created_props.dig("token", "$ref") == "#/components/schemas/RelayAPIToken"
      missing << "CreatedRelayAPIToken must expose one-time rawToken plus RelayAPIToken token"
    end

    usage_props = schemas.fetch("ConsoleAPITokenUsageItem", {}).fetch("properties", {})
    ["id", "apiTokenId", "requestId", "apiType", "model", "status", "statusCode", "totalTokens", "createdAt"].each do |field|
      missing << "ConsoleAPITokenUsageItem.#{field} must be documented for console usage responses" unless usage_props.key?(field)
    end
    ["rawToken", "tokenHash", "token_hash"].each do |field|
      missing << "ConsoleAPITokenUsageItem must not expose #{field}" if usage_props.key?(field)
    end
    ["provider", "channelId", "channel_id"].each do |field|
      missing << "ConsoleAPITokenUsageItem must not expose #{field}" if usage_props.key?(field)
    end

    unless missing.empty?
      warn "[openapi-contract] Console API token CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_api_token_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    list = operation(paths, "/api/v1/admin/api-tokens", "get", missing)
    revoke = operation(paths, "/api/v1/admin/api-tokens/{tokenId}/revoke", "post", missing)

    {
      "GET /api/v1/admin/api-tokens" => list,
      "POST /api/v1/admin/api-tokens/{tokenId}/revoke" => revoke,
    }.each do |label, op|
      tags = op.fetch("tags", [])
      missing << "#{label} must be tagged Admin" unless tags.include?("Admin")
      missing << "#{label} must be tagged Relay" unless tags.include?("Relay")
    end

    unless requires_cookie_and_csrf?(revoke)
      missing << "POST /api/v1/admin/api-tokens/{tokenId}/revoke must require cookieAuth and csrfHeader"
    end
    unless requires_cookie_without_csrf?(list)
      missing << "GET /api/v1/admin/api-tokens must require cookieAuth without csrfHeader"
    end
    unless response_data_ref(list, "200") == "#/components/schemas/AdminAPITokenListResponse"
      missing << "GET /api/v1/admin/api-tokens 200 data must reference AdminAPITokenListResponse"
    end
    unless response_data_ref(revoke, "200") == "#/components/schemas/RelayAPITokenRevokeResponse"
      missing << "POST /api/v1/admin/api-tokens/{tokenId}/revoke 200 data must reference RelayAPITokenRevokeResponse"
    end

    parameter_names = list.fetch("parameters", []).map { |param| param["name"] }
    ["organizationID", "userID", "status", "userGroup", "search", "model", "limit", "offset"].each do |name|
      missing << "GET /api/v1/admin/api-tokens must document #{name} query parameter" unless parameter_names.include?(name)
    end

    list_schema = schemas["AdminAPITokenListResponse"] || {}
    unless list_schema.dig("properties", "apiTokens", "items", "$ref") == "#/components/schemas/AdminAPIToken" &&
        list_schema.dig("properties", "total", "type") == "integer"
      missing << "AdminAPITokenListResponse must expose apiTokens[] plus integer total"
    end

    admin_token_props = schemas.fetch("AdminAPIToken", {}).fetch("properties", {})
    ["id", "organizationId", "userId", "userEmail", "name", "tokenPrefix", "status", "modelLimitsEnabled", "modelLimits", "quotaLimit", "usedQuota", "requestCount", "totalCost", "createdAt"].each do |field|
      missing << "AdminAPIToken.#{field} must be documented" unless admin_token_props.key?(field)
    end
    ["rawToken", "tokenHash", "token_hash"].each do |field|
      missing << "AdminAPIToken must not expose #{field}" if admin_token_props.key?(field)
    end

    relay_token_props = schemas.fetch("RelayAPIToken", {}).fetch("properties", {})
    ["rawToken", "tokenHash", "token_hash"].each do |field|
      missing << "RelayAPIToken list schema must not expose #{field}" if relay_token_props.key?(field)
    end

    unless missing.empty?
      warn "[openapi-contract] Admin API token contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_task_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    expected_read_responses = {
      ["/api/v1/app/tasks", "get"] => [:array, "#/components/schemas/Task"],
      ["/api/v1/app/tasks/{taskId}", "get"] => [:object, "#/components/schemas/TaskDetail"],
    }

    expected_read_responses.each do |(path, method), (shape, expected_response)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Task")
        missing << "#{method.upcase} #{path} must be tagged Task"
      end
      actual_response = shape == :array ? response_array_item_ref(op, "200") : response_data_ref(op, "200")
      unless actual_response == expected_response
        missing << "#{method.upcase} #{path} 200 data must reference #{expected_response}"
      end
    end

    expected_mutation_responses = {
      ["/api/v1/app/tasks", "post"] => "#/components/schemas/Task",
      ["/api/v1/app/tasks/{taskId}/start", "post"] => "#/components/schemas/TaskDetail",
      ["/api/v1/app/tasks/{taskId}/approve", "post"] => "#/components/schemas/TaskDetail",
      ["/api/v1/app/tasks/{taskId}/pause", "post"] => "#/components/schemas/TaskDetail",
      ["/api/v1/app/tasks/{taskId}/resume", "post"] => "#/components/schemas/TaskDetail",
      ["/api/v1/app/tasks/{taskId}/cancel", "post"] => "#/components/schemas/TaskDetail",
      ["/api/v1/app/tasks/{taskId}/budget", "post"] => "#/components/schemas/TaskDetail",
    }

    expected_mutation_responses.each do |(path, method), expected_response|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Task")
        missing << "#{method.upcase} #{path} must be tagged Task"
      end
      unless response_data_ref(op, "200") == expected_response
        missing << "#{method.upcase} #{path} 200 data must reference #{expected_response}"
      end
    end

    create = operation(paths, "/api/v1/app/tasks", "post", missing)
    unless create.dig("requestBody", "required") == true &&
        request_body_ref(create) == "#/components/schemas/CreateTaskRequest"
      missing << "POST /api/v1/app/tasks request body must require CreateTaskRequest"
    end

    budget = operation(paths, "/api/v1/app/tasks/{taskId}/budget", "post", missing)
    unless budget.dig("requestBody", "required") == true &&
        request_body_ref(budget) == "#/components/schemas/UpdateTaskBudgetRequest"
      missing << "POST /api/v1/app/tasks/{taskId}/budget request body must require UpdateTaskBudgetRequest"
    end

    create_request = schemas["CreateTaskRequest"] || {}
    create_mode_enum = create_request.dig("properties", "executionMode", "enum") || []
    unless ["standard", "safe", "auto"].all? { |mode| create_mode_enum.include?(mode) } &&
        !["semi-auto", "manual"].any? { |mode| create_mode_enum.include?(mode) }
      missing << "CreateTaskRequest.executionMode must enumerate runtime modes standard, safe, and auto only"
    end
    create_scope_enum = create_request.dig("properties", "authorizationScope", "enum") || []
    unless ["knowledge_only", "workspace_tools", "full_access"].all? { |scope| create_scope_enum.include?(scope) }
      missing << "CreateTaskRequest.authorizationScope must enumerate runtime scopes"
    end

    task = schemas["Task"] || {}
    task_mode_enum = task.dig("properties", "executionMode", "enum") || []
    unless ["standard", "safe", "auto"].all? { |mode| task_mode_enum.include?(mode) }
      missing << "Task.executionMode must enumerate standard, safe, and auto"
    end
    task_status_enum = task.dig("properties", "status", "enum") || []
    unless ["draft", "running", "paused", "awaiting_confirmation", "completed", "cancelled"].all? { |status| task_status_enum.include?(status) }
      missing << "Task.status must document runtime lifecycle statuses"
    end
    unless task.dig("properties", "authorizationScope", "enum")&.include?("workspace_tools") &&
        task.dig("properties", "budgetConsumed", "type") == "integer"
      missing << "Task must expose authorizationScope and budgetConsumed runtime fields"
    end

    task_detail = schemas["TaskDetail"] || {}
    detail_refs = task_detail.fetch("allOf", []).filter_map { |entry| entry["$ref"] }
    detail_properties = task_detail.fetch("allOf", []).find { |entry| entry["properties"].is_a?(Hash) }&.fetch("properties", {}) || {}
    unless detail_refs.include?("#/components/schemas/Task") &&
        detail_properties.dig("steps", "items", "$ref") == "#/components/schemas/TaskStep" &&
        detail_properties.dig("events", "items", "$ref") == "#/components/schemas/TaskEvent" &&
        detail_properties.dig("resultArtifacts", "items", "$ref") == "#/components/schemas/TaskResultArtifact" &&
        detail_properties.dig("knowledgeBaseIds", "items", "type") == "string" &&
        detail_properties.dig("toolAllowList", "items", "type") == "string" &&
        detail_properties.dig("toolDenyList", "items", "type") == "string"
      missing << "TaskDetail must extend Task and expose steps, events, resultArtifacts, knowledgeBaseIds, and tool rule arrays"
    end

    unless missing.empty?
      warn "[openapi-contract] Task mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_notification_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    paths = spec.fetch("paths", {})
    missing = []

    def operation(paths, path, method, missing)
      op = paths.dig(path, method)
      unless op
        missing << "#{method.upcase} #{path} must be documented"
        return {}
      end
      op
    end

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    expected_responses = {
      ["/api/v1/app/notifications", "post"] => ["201", "#/components/schemas/Notification"],
      ["/api/v1/app/notifications/mark-all-read", "post"] => ["200", "#/components/schemas/NotificationActionStatus"],
      ["/api/v1/app/notifications/{notificationId}", "patch"] => ["200", "#/components/schemas/NotificationActionStatus"],
      ["/api/v1/app/notifications/{notificationId}", "delete"] => ["200", "#/components/schemas/NotificationActionStatus"],
    }

    expected_responses.each do |(path, method), (status, expected)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Notification")
        missing << "#{method.upcase} #{path} must be tagged Notification"
      end
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    create = operation(paths, "/api/v1/app/notifications", "post", missing)
    unless create.dig("requestBody", "required") == true &&
        request_body_ref(create) == "#/components/schemas/CreateNotificationRequest"
      missing << "POST /api/v1/app/notifications request body must require CreateNotificationRequest"
    end

    unless missing.empty?
      warn "[openapi-contract] Notification mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_scheduled_task_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_array_item_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    unless schemas.dig("ScheduledTask", "properties", "targetType", "enum") == ["workflow", "agent"]
      missing << "ScheduledTask.targetType must enumerate workflow and agent"
    end
    unless schemas.dig("ScheduledTaskRun", "properties", "status", "enum") == ["queued", "running", "completed", "failed", "cancelled"]
      missing << "ScheduledTaskRun.status must enumerate queued, running, completed, failed, and cancelled"
    end
    unless schemas.dig("CreateScheduledTaskRequest", "required")&.include?("name") &&
        schemas.dig("CreateScheduledTaskRequest", "required")&.include?("targetType") &&
        schemas.dig("CreateScheduledTaskRequest", "required")&.include?("targetId") &&
        schemas.dig("CreateScheduledTaskRequest", "required")&.include?("cronExpression")
      missing << "CreateScheduledTaskRequest must require name, targetType, targetId, and cronExpression"
    end
    unless schemas.dig("UpdateScheduledTaskStatusRequest", "required")&.include?("enabled")
      missing << "UpdateScheduledTaskStatusRequest must require enabled"
    end

    list = operation(paths, "/api/v1/scheduled-tasks", "get", missing)
    unless requires_cookie_without_csrf?(list)
      missing << "GET /api/v1/scheduled-tasks must require cookieAuth without csrfHeader"
    end
    unless list.fetch("tags", []).include?("ScheduledTask") &&
        response_array_item_ref(list, "200") == "#/components/schemas/ScheduledTask"
      missing << "GET /api/v1/scheduled-tasks must be tagged ScheduledTask and return ScheduledTask[] data"
    end

    runs = operation(paths, "/api/v1/scheduled-tasks/{scheduledTaskId}/runs", "get", missing)
    unless requires_cookie_without_csrf?(runs)
      missing << "GET /api/v1/scheduled-tasks/{scheduledTaskId}/runs must require cookieAuth without csrfHeader"
    end
    unless runs.fetch("tags", []).include?("ScheduledTask") &&
        response_array_item_ref(runs, "200") == "#/components/schemas/ScheduledTaskRun"
      missing << "GET /api/v1/scheduled-tasks/{scheduledTaskId}/runs must be tagged ScheduledTask and return ScheduledTaskRun[] data"
    end

    expected_mutations = {
      ["/api/v1/scheduled-tasks", "post"] => ["201", "#/components/schemas/CreateScheduledTaskRequest", "#/components/schemas/ScheduledTask"],
      ["/api/v1/scheduled-tasks/{scheduledTaskId}/status", "patch"] => ["200", "#/components/schemas/UpdateScheduledTaskStatusRequest", "#/components/schemas/ScheduledTask"],
      ["/api/v1/scheduled-tasks/{scheduledTaskId}/run", "post"] => ["202", nil, "#/components/schemas/ScheduledTaskRun"],
    }

    expected_mutations.each do |(path, method), (status, request_ref, response_ref)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("ScheduledTask")
        missing << "#{method.upcase} #{path} must be tagged ScheduledTask"
      end
      if request_ref
        unless op.dig("requestBody", "required") == true && request_body_ref(op) == request_ref
          missing << "#{method.upcase} #{path} request body must require #{request_ref}"
        end
      end
      unless response_data_ref(op, status) == response_ref
        missing << "#{method.upcase} #{path} #{status} data must reference #{response_ref}"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Scheduled Task route/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_preferences_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    preferences = operation(paths, "/api/v1/app/me/preferences", "put", missing)
    get_preferences = operation(paths, "/api/v1/app/me/preferences", "get", missing)
    unless response_data_ref(get_preferences, "200") == "#/components/schemas/Preferences"
      missing << "GET /api/v1/app/me/preferences 200 data must reference Preferences"
    end
    unless requires_cookie_and_csrf?(preferences)
      missing << "PUT /api/v1/app/me/preferences must require cookieAuth and csrfHeader"
    end
    unless preferences.fetch("tags", []).include?("Preferences")
      missing << "PUT /api/v1/app/me/preferences must be tagged Preferences"
    end
    unless preferences.dig("requestBody", "required") == true &&
        request_body_ref(preferences) == "#/components/schemas/UpdatePreferencesRequest"
      missing << "PUT /api/v1/app/me/preferences request body must require UpdatePreferencesRequest"
    end
    unless response_data_ref(preferences, "200") == "#/components/schemas/Preferences"
      missing << "PUT /api/v1/app/me/preferences 200 data must reference Preferences"
    end
    expected_fields = {
      "defaultMode" => "string",
      "modelStrategy" => "string",
      "networkEnabledHint" => "boolean",
      "onboardingCompleted" => "boolean",
      "defaultAgentModel" => "string",
      "sidebarCollapsed" => "boolean",
      "notifications" => "object"
    }
    ["Preferences", "UpdatePreferencesRequest"].each do |schema_name|
      properties = schemas.dig(schema_name, "properties") || {}
      expected_fields.each do |field, expected_type|
        unless properties.dig(field, "type") == expected_type
          missing << "#{schema_name}.#{field} must be documented as #{expected_type}"
        end
      end
      unless properties.dig("notifications", "additionalProperties") == true
        missing << "#{schema_name}.notifications must allow object properties"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Preferences mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_chat_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_data_array_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    expected_responses = {
      ["/api/v1/app/conversations", "post"] => ["200", "#/components/schemas/Conversation", :ref],
      ["/api/v1/app/conversations/{conversationId}", "put"] => ["200", "#/components/schemas/Conversation", :ref],
      ["/api/v1/app/conversations/{conversationId}", "delete"] => ["200", "#/components/schemas/ConversationDeleteResponse", :ref],
      ["/api/v1/app/conversations/{conversationId}/fork", "post"] => ["200", "#/components/schemas/Conversation", :ref],
      ["/api/v1/app/conversations/{conversationId}/messages", "post"] => ["200", "#/components/schemas/Message", :array_ref],
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}", "put"] => ["200", "#/components/schemas/Message", :ref],
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}", "delete"] => ["200", "#/components/schemas/MessageDeleteResponse", :ref],
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}/bookmark", "post"] => ["200", "#/components/schemas/Message", :ref],
      ["/api/v1/app/conversations/{conversationId}/config", "put"] => ["200", "#/components/schemas/ConversationConfig", :ref],
      ["/api/v1/app/conversations/{conversationId}/convert-to-task", "post"] => ["200", "#/components/schemas/TaskDraft", :ref],
      ["/api/v1/app/conversations/{conversationId}/share", "post"] => ["201", "#/components/schemas/ConversationShareResponse", :ref],
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}/share", "post"] => ["201", "#/components/schemas/MessageShareResponse", :ref],
      ["/api/v1/conversations", "post"] => ["200", "#/components/schemas/Conversation", :ref],
      ["/api/v1/conversations/{conversationId}", "put"] => ["200", "#/components/schemas/Conversation", :ref],
      ["/api/v1/conversations/{conversationId}", "delete"] => ["200", "#/components/schemas/ConversationDeleteResponse", :ref],
      ["/api/v1/conversations/{conversationId}/fork", "post"] => ["200", "#/components/schemas/Conversation", :ref],
      ["/api/v1/conversations/{conversationId}/messages", "post"] => ["200", "#/components/schemas/Message", :array_ref],
      ["/api/v1/conversations/{conversationId}/messages/{messageId}", "put"] => ["200", "#/components/schemas/Message", :ref],
      ["/api/v1/conversations/{conversationId}/messages/{messageId}", "delete"] => ["200", "#/components/schemas/MessageDeleteResponse", :ref],
      ["/api/v1/conversations/{conversationId}/messages/{messageId}/bookmark", "post"] => ["200", "#/components/schemas/Message", :ref],
      ["/api/v1/conversations/{conversationId}/messages/{messageId}/share", "post"] => ["201", "#/components/schemas/MessageShareResponse", :ref],
      ["/api/v1/app/personas", "post"] => ["200", "#/components/schemas/Persona", :ref],
      ["/api/v1/app/personas/{personaId}", "put"] => ["200", "#/components/schemas/Persona", :ref],
      ["/api/v1/app/personas/{personaId}", "delete"] => ["200", "#/components/schemas/PersonaDeleteResponse", :ref],
    }

    expected_responses.each do |(path, method), (status, expected, shape)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Chat")
        missing << "#{method.upcase} #{path} must be tagged Chat"
      end
      actual = shape == :array_ref ? response_data_array_ref(op, status) : response_data_ref(op, status)
      unless actual == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/conversations", "post"] => "#/components/schemas/CreateConversationRequest",
      ["/api/v1/app/conversations/{conversationId}", "put"] => "#/components/schemas/UpdateConversationRequest",
      ["/api/v1/app/conversations/{conversationId}/fork", "post"] => "#/components/schemas/ForkConversationRequest",
      ["/api/v1/app/conversations/{conversationId}/messages", "post"] => "#/components/schemas/SendMessageRequest",
      ["/api/v1/app/conversations/{conversationId}/messages/stream", "post"] => "#/components/schemas/SendMessageRequest",
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}", "put"] => "#/components/schemas/UpdateMessageRequest",
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}/bookmark", "post"] => "#/components/schemas/BookmarkMessageRequest",
      ["/api/v1/app/conversations/{conversationId}/config", "put"] => "#/components/schemas/UpdateConversationConfigRequest",
      ["/api/v1/app/conversations/{conversationId}/share", "post"] => "#/components/schemas/CreateConversationShareRequest",
      ["/api/v1/app/conversations/{conversationId}/messages/{messageId}/share", "post"] => "#/components/schemas/CreateMessageShareRequest",
      ["/api/v1/conversations", "post"] => "#/components/schemas/CreateConversationRequest",
      ["/api/v1/conversations/{conversationId}", "put"] => "#/components/schemas/UpdateConversationRequest",
      ["/api/v1/conversations/{conversationId}/fork", "post"] => "#/components/schemas/ForkConversationRequest",
      ["/api/v1/conversations/{conversationId}/messages", "post"] => "#/components/schemas/SendMessageRequest",
      ["/api/v1/conversations/{conversationId}/messages/{messageId}", "put"] => "#/components/schemas/UpdateMessageRequest",
      ["/api/v1/conversations/{conversationId}/messages/{messageId}/bookmark", "post"] => "#/components/schemas/BookmarkMessageRequest",
      ["/api/v1/conversations/{conversationId}/messages/{messageId}/share", "post"] => "#/components/schemas/CreateMessageShareRequest",
      ["/api/v1/app/personas", "post"] => "#/components/schemas/PersonaRequest",
      ["/api/v1/app/personas/{personaId}", "put"] => "#/components/schemas/PersonaRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    {
      ["/api/v1/conversations", "get"] => ["#/components/schemas/Conversation", :array_ref],
      ["/api/v1/conversations/{conversationId}", "get"] => ["#/components/schemas/Conversation", :ref],
      ["/api/v1/conversations/{conversationId}/messages", "get"] => ["#/components/schemas/Message", :array_ref],
    }.each do |(path, method), (expected, shape)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Chat")
        missing << "#{method.upcase} #{path} must be tagged Chat"
      end
      actual = shape == :array_ref ? response_data_array_ref(op, "200") : response_data_ref(op, "200")
      unless actual == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/models", "get"] => ["#/components/schemas/Model", :array_ref],
      ["/api/v1/app/conversations", "get"] => ["#/components/schemas/Conversation", :array_ref],
      ["/api/v1/app/conversations/{conversationId}/messages", "get"] => ["#/components/schemas/Message", :array_ref],
    }.each do |(path, method), (expected, shape)|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Chat")
        missing << "#{method.upcase} #{path} must be tagged Chat"
      end
      actual = shape == :array_ref ? response_data_array_ref(op, "200") : response_data_ref(op, "200")
      unless actual == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/personas", "get"] => "#/components/schemas/Persona",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Chat")
        missing << "#{method.upcase} #{path} must be tagged Chat"
      end
      unless response_data_array_ref(op, "200") == expected
        missing << "#{method.upcase} #{path} 200 data array must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/conversations/{conversationId}", "get"] => "#/components/schemas/Conversation",
      ["/api/v1/app/personas/{personaId}", "get"] => "#/components/schemas/Persona",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Chat")
        missing << "#{method.upcase} #{path} must be tagged Chat"
      end
      unless response_data_ref(op, "200") == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/message-shares/{shareId}", "get"] => "#/components/schemas/MessageShareDetailResponse",
      ["/api/v1/app/conversation-shares/{shareId}", "get"] => "#/components/schemas/ConversationShareDetailResponse",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless op["security"] == []
        missing << "#{method.upcase} #{path} must declare security: []"
      end
      unless op.fetch("tags", []).include?("Chat")
        missing << "#{method.upcase} #{path} must be tagged Chat"
      end
      unless response_data_ref(op, "200") == expected
        missing << "#{method.upcase} #{path} 200 data must reference #{expected}"
      end
    end

    stream = operation(paths, "/api/v1/app/conversations/{conversationId}/messages/stream", "post", missing)
    unless requires_cookie_and_csrf?(stream)
      missing << "POST /api/v1/app/conversations/{conversationId}/messages/stream must require cookieAuth and csrfHeader"
    end
    unless stream.fetch("tags", []).include?("Chat")
      missing << "POST /api/v1/app/conversations/{conversationId}/messages/stream must be tagged Chat"
    end
    unless stream.dig("responses", "200", "content", "text/event-stream", "schema", "type") == "string"
      missing << "POST /api/v1/app/conversations/{conversationId}/messages/stream 200 response must document text/event-stream"
    end

    export = operation(paths, "/api/v1/app/conversations/{conversationId}/export.md", "get", missing)
    unless requires_cookie_without_csrf?(export)
      missing << "GET /api/v1/app/conversations/{conversationId}/export.md must require cookieAuth without csrfHeader"
    end
    unless export.fetch("tags", []).include?("Chat")
      missing << "GET /api/v1/app/conversations/{conversationId}/export.md must be tagged Chat"
    end
    unless export.dig("responses", "200", "content", "text/markdown", "schema", "type") == "string"
      missing << "GET /api/v1/app/conversations/{conversationId}/export.md 200 response must document text/markdown"
    end

    config = operation(paths, "/api/v1/app/conversations/{conversationId}/config", "get", missing)
    unless requires_cookie_without_csrf?(config)
      missing << "GET /api/v1/app/conversations/{conversationId}/config must require cookieAuth without csrfHeader"
    end
    unless config.fetch("tags", []).include?("Chat")
      missing << "GET /api/v1/app/conversations/{conversationId}/config must be tagged Chat"
    end
    unless response_data_ref(config, "200") == "#/components/schemas/ConversationConfig"
      missing << "GET /api/v1/app/conversations/{conversationId}/config 200 data must reference ConversationConfig"
    end

    unless schemas.dig("UpdateConversationConfigRequest", "properties", "personaId", "type") == "string"
      missing << "UpdateConversationConfigRequest must document personaId"
    end
    unless schemas.key?("ConversationConfig")
      missing << "ConversationConfig schema must be documented"
    end
    unless schemas.key?("TaskDraft")
      missing << "TaskDraft schema must be documented"
    end
    unless schemas.dig("UpdateConversationRequest", "properties", "title", "type") == "string"
      missing << "UpdateConversationRequest.title must be documented as string"
    end
    unless schemas.dig("ConversationDeleteResponse", "properties", "status", "enum")&.include?("deleted")
      missing << "ConversationDeleteResponse.status must document deleted"
    end
    fork = schemas["ForkConversationRequest"] || {}
    unless fork.fetch("required", []).include?("branchFromMessageId") &&
        fork.dig("properties", "branchFromMessageId", "type") == "string" &&
        fork.dig("properties", "messageId", "deprecated") == true &&
        fork.dig("properties", "sourceConversationId", "type") == "string"
      missing << "ForkConversationRequest must require branchFromMessageId, document legacy messageId, and allow sourceConversationId"
    end
    unless schemas.dig("Message", "properties", "bookmarked", "type") == "boolean"
      missing << "Message.bookmarked must be documented as boolean"
    end
    update_message = schemas["UpdateMessageRequest"] || {}
    unless update_message.fetch("required", []).include?("content") && update_message.dig("properties", "content", "type") == "string"
      missing << "UpdateMessageRequest.content must be required and documented as string"
    end
    unless schemas.dig("BookmarkMessageRequest", "properties", "bookmarked", "type") == "boolean"
      missing << "BookmarkMessageRequest.bookmarked must be documented as boolean"
    end
    unless schemas.dig("MessageDeleteResponse", "properties", "status", "enum")&.include?("deleted")
      missing << "MessageDeleteResponse.status must document deleted"
    end
    persona = schemas["Persona"] || {}
    persona_props = persona.fetch("properties", {})
    ["id", "workspaceId", "name", "role", "style", "tone", "constraints", "openingMessage"].each do |property|
      unless persona_props.dig(property, "type") == "string"
        missing << "Persona.#{property} must be documented as string"
      end
    end
    unless persona_props.dig("createdAt", "format") == "date-time"
      missing << "Persona.createdAt must be documented as date-time"
    end
    unless persona_props.dig("suggestedQuestions", "items", "type") == "string"
      missing << "Persona.suggestedQuestions must be documented as string[]"
    end
    persona_request = schemas["PersonaRequest"] || {}
    unless persona_request.fetch("required", []).include?("name") && persona_request.dig("properties", "name", "type") == "string"
      missing << "PersonaRequest.name must be required and documented as string"
    end
    unless persona_request.dig("properties", "suggestedQuestions", "items", "type") == "string"
      missing << "PersonaRequest.suggestedQuestions must be documented as string[]"
    end
    unless schemas.dig("PersonaDeleteResponse", "properties", "status", "type") == "string"
      missing << "PersonaDeleteResponse.status must be documented as string"
    end
    unless schemas.dig("CreateMessageShareRequest", "properties", "expiresAt", "format") == "date-time"
      missing << "CreateMessageShareRequest.expiresAt must be documented as date-time"
    end
    unless schemas.dig("CreateConversationShareRequest", "properties", "startMessageId", "type") == "string" &&
        schemas.dig("CreateConversationShareRequest", "properties", "endMessageId", "type") == "string" &&
        schemas.dig("CreateConversationShareRequest", "properties", "expiresAt", "format") == "date-time"
      missing << "CreateConversationShareRequest must document range fields and expiresAt"
    end
    unless schemas.dig("MessageShareResponse", "properties", "url", "type") == "string" &&
        schemas.dig("ConversationShareResponse", "properties", "url", "type") == "string"
      missing << "share response schemas must document url"
    end
    unless schemas.dig("MessageShareDetailResponse", "allOf")&.any? { |entry| entry.dig("properties", "message", "$ref") == "#/components/schemas/Message" } &&
        schemas.dig("ConversationShareDetailResponse", "allOf")&.any? { |entry| entry.dig("properties", "messages", "items", "$ref") == "#/components/schemas/Message" }
      missing << "share detail schemas must document message payloads"
    end

    unless missing.empty?
      warn "[openapi-contract] Chat mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_knowledge_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation, content_type)
      operation.dig("requestBody", "content", content_type, "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    def response_data_array_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "items", "$ref") }&.
        dig("properties", "data", "items", "$ref")
    end

    mutation_paths = [
      ["/api/v1/app/knowledge-bases", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}", "put"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}", "delete"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "put"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "delete"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "put"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "post"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "post"],
    ]

    mutation_paths.each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
      unless op.fetch("tags", []).include?("Knowledge")
        missing << "#{method.upcase} #{path} must be tagged Knowledge"
      end
    end

    [
      ["/api/v1/app/knowledge-bases", "get"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}", "get"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "get"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions", "get"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks", "get"],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "get"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
      unless op.fetch("tags", []).include?("Knowledge")
        missing << "#{method.upcase} #{path} must be tagged Knowledge"
      end
    end

    {
      ["/api/v1/app/knowledge-bases", "post", "application/json"] => "#/components/schemas/CreateKnowledgeBaseRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}", "put", "application/json"] => "#/components/schemas/CreateKnowledgeBaseRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "post", "application/json"] => "#/components/schemas/CreateDocumentRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "post", "multipart/form-data"] => "#/components/schemas/UploadKnowledgeDocumentRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "put", "application/json"] => "#/components/schemas/CreateDocumentRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "put", "application/json"] => "#/components/schemas/UpdateKnowledgeDocumentChunkRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "post", "application/json"] => "#/components/schemas/SplitKnowledgeDocumentChunkRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "post", "application/json"] => "#/components/schemas/MergeKnowledgeDocumentChunksRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "post", "application/json"] => "#/components/schemas/RetrieveKnowledgeRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "post", "application/json"] => "#/components/schemas/CreateKnowledgeRetrievalTestCaseRequest",
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "post", "application/json"] => "#/components/schemas/KnowledgeRetrievalTestRunRequest",
    }.each do |(path, method, content_type), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op, content_type) == expected
        missing << "#{method.upcase} #{path} #{content_type} request body must reference #{expected}"
      end
    end

    {
      ["/api/v1/app/knowledge-bases", "get", "200"] => ["#/components/schemas/KnowledgeBase", :array_ref],
      ["/api/v1/app/knowledge-bases", "post", "200"] => ["#/components/schemas/KnowledgeBase", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}", "get", "200"] => ["#/components/schemas/KnowledgeBase", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}", "put", "200"] => ["#/components/schemas/KnowledgeBase", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "get", "200"] => ["#/components/schemas/Document", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents", "post", "200"] => ["#/components/schemas/Document", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload", "post", "200"] => ["#/components/schemas/Document", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}", "put", "200"] => ["#/components/schemas/Document", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions", "get", "200"] => ["#/components/schemas/KnowledgeDocumentVersion", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks", "get", "200"] => ["#/components/schemas/KnowledgeDocumentChunk", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}", "put", "200"] => ["#/components/schemas/KnowledgeDocumentChunk", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split", "post", "200"] => ["#/components/schemas/KnowledgeDocumentChunk", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge", "post", "200"] => ["#/components/schemas/KnowledgeDocumentChunk", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve", "post", "200"] => ["#/components/schemas/KnowledgeRetrievalResult", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "get", "200"] => ["#/components/schemas/KnowledgeRetrievalTestCase", :array_ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases", "post", "201"] => ["#/components/schemas/KnowledgeRetrievalTestCase", :ref],
      ["/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run", "post", "200"] => ["#/components/schemas/KnowledgeRetrievalTestRunReport", :ref],
	    }.each do |(path, method, status), (expected, shape)|
	      op = operation(paths, path, method, missing)
	      actual = shape == :array_ref ? response_data_array_ref(op, status) : response_data_ref(op, status)
	      unless actual == expected
	        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
	      end
	    end

	    {
	      "/api/v1/knowledge-bases" => "#/paths/~1api~1v1~1app~1knowledge-bases",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/upload" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1upload",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1versions",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks~1{chunkId}",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks~1{chunkId}~1split",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1documents~1{documentId}~1chunks~1{chunkId}~1merge",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieve" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieve",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieval-test-cases",
	      "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run" => "#/paths/~1api~1v1~1app~1knowledge-bases~1{knowledgeBaseId}~1retrieval-test-cases~1run",
	    }.each do |alias_path, expected_ref|
	      unless paths.dig(alias_path, "$ref") == expected_ref
	        missing << "#{alias_path} must reference #{expected_ref}"
	      end
	    end
	    root_document_delete = operation(paths, "/api/v1/documents/{documentId}", "delete", missing)
	    unless requires_cookie_and_csrf?(root_document_delete)
	      missing << "DELETE /api/v1/documents/{documentId} must require cookieAuth and csrfHeader"
	    end
	    unless root_document_delete.fetch("tags", []).include?("Knowledge")
	      missing << "DELETE /api/v1/documents/{documentId} must be tagged Knowledge"
	    end
	    unless root_document_delete.fetch("parameters", []).any? { |param| param["name"] == "documentId" && param["in"] == "path" && param["required"] == true }
	      missing << "DELETE /api/v1/documents/{documentId} must require documentId path parameter"
	    end
	    unless root_document_delete.dig("responses", "204", "description")
	      missing << "DELETE /api/v1/documents/{documentId} must document 204 deletion"
	    end

	    unless schemas.dig("KnowledgeBase", "properties", "retrievalMode", "enum")&.include?("hybrid_rerank") &&
        schemas.dig("CreateKnowledgeBaseRequest", "properties", "chunkSize", "type") == "integer" &&
        schemas.dig("CreateKnowledgeBaseRequest", "properties", "embeddingModel", "type") == "string" &&
        schemas.dig("CreateKnowledgeBaseRequest", "properties", "vectorWeight", "format") == "double"
      missing << "KnowledgeBase and CreateKnowledgeBaseRequest must document retrieval/chunking config fields"
    end
    unless schemas.dig("CreateDocumentRequest", "properties", "documentVersion", "type") == "string" &&
        schemas.dig("CreateDocumentRequest", "properties", "pageNumber", "type") == "integer" &&
        schemas.dig("CreateDocumentRequest", "properties", "sourceUrl", "type") == "string" &&
        schemas.dig("UploadKnowledgeDocumentRequest", "properties", "file", "format") == "binary"
      missing << "Knowledge document create/upload schemas must document metadata and multipart file fields"
    end
    unless schemas.dig("KnowledgeRetrievalResult", "properties", "documentId", "type") == "string" &&
        schemas.dig("KnowledgeRetrievalResult", "properties", "snippet", "type") == "string" &&
        schemas.dig("KnowledgeRetrievalResult", "properties", "retrievalMode", "enum")&.include?("hybrid_rerank")
      missing << "KnowledgeRetrievalResult schema must document result identity, snippet, and retrieval mode"
    end
    unless schemas.dig("CreateKnowledgeRetrievalTestCaseRequest", "properties", "expectedResult", "$ref") == "#/components/schemas/KnowledgeRetrievalResult" &&
        schemas.dig("KnowledgeRetrievalTestCase", "properties", "expectedResult", "$ref") == "#/components/schemas/KnowledgeRetrievalResult" &&
        schemas.dig("KnowledgeRetrievalTestRunReport", "properties", "results", "items", "$ref") == "#/components/schemas/KnowledgeRetrievalTestRunResult"
      missing << "Knowledge retrieval test case schemas must reference typed retrieval results"
    end

    unless missing.empty?
      warn "[openapi-contract] Knowledge mutation CSRF/schema contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_organization_mutation_csrf_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_and_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && entry.key?("csrfHeader") }
    end

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    def request_body_ref(operation)
      operation.dig("requestBody", "content", "application/json", "schema", "$ref")
    end

    def response_data_ref(operation, status)
      operation.dig("responses", status, "content", "application/json", "schema", "allOf")&.
        find { |entry| entry.dig("properties", "data", "$ref") }&.
        dig("properties", "data", "$ref")
    end

    expected_data_refs = {
      ["/api/v1/admin/organizations", "get", "200"] => "#/components/schemas/AdminOrganizationListResponse",
      ["/api/v1/admin/organizations", "post", "201"] => "#/components/schemas/Organization",
      ["/api/v1/admin/organizations/{organizationId}", "get", "200"] => "#/components/schemas/Organization",
      ["/api/v1/admin/organizations/{organizationId}", "put", "200"] => "#/components/schemas/Organization",
      ["/api/v1/admin/organizations/{organizationId}/archive", "post", "200"] => "#/components/schemas/Organization",
      ["/api/v1/admin/organizations/{organizationId}/members", "get", "200"] => "#/components/schemas/AdminOrganizationMembersResponse",
    }

    expected_data_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
      unless op.fetch("tags", []).include?("Admin")
        missing << "#{method.upcase} #{path} must be tagged Admin"
      end
      if method == "get" && !requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
    end

    [
      ["/api/v1/admin/organizations", "post"],
      ["/api/v1/admin/organizations/{organizationId}", "put"],
      ["/api/v1/admin/organizations/{organizationId}/archive", "post"],
    ].each do |path, method|
      op = operation(paths, path, method, missing)
      unless requires_cookie_and_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
      end
    end

    {
      ["/api/v1/admin/organizations", "post"] => "#/components/schemas/CreateOrganizationRequest",
      ["/api/v1/admin/organizations/{organizationId}", "put"] => "#/components/schemas/UpdateOrganizationRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless op.dig("requestBody", "required") == true && request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must require #{expected}"
      end
    end

    unless schemas.dig("AdminOrganizationListResponse", "properties", "organizations", "items", "$ref") == "#/components/schemas/Organization" &&
        schemas.dig("AdminOrganizationListResponse", "properties", "total", "type") == "integer"
      missing << "AdminOrganizationListResponse must expose organizations[] plus integer total"
    end
    unless schemas.dig("AdminOrganizationMembersResponse", "properties", "members", "items", "$ref") == "#/components/schemas/OrganizationMembership"
      missing << "AdminOrganizationMembersResponse must expose members[]"
    end

    unless missing.empty?
      warn "[openapi-contract] Admin organization mutation CSRF contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_core_management_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
    end

    expected_data_refs = {
      ["/api/v1/admin/stats", "get", "200"] => "#/components/schemas/AdminStats",
      ["/api/v1/admin/settings/relay-pricing", "get", "200"] => "#/components/schemas/AdminRelayPricingSettings",
      ["/api/v1/admin/settings/relay-pricing", "put", "200"] => "#/components/schemas/AdminRelayPricingSettings",
      ["/api/v1/admin/settings/usage-limits", "get", "200"] => "#/components/schemas/AdminUsageLimitSettingsListResponse",
      ["/api/v1/admin/settings/usage-limits", "put", "200"] => "#/components/schemas/AdminUsageLimitSettings",
      ["/api/v1/admin/routes", "get", "200"] => "#/components/schemas/AdminRouteListResponse",
      ["/api/v1/admin/routes", "post", "201"] => "#/components/schemas/AdminRoute",
      ["/api/v1/admin/routes/{routeId}", "get", "200"] => "#/components/schemas/AdminRoute",
      ["/api/v1/admin/routes/{routeId}", "put", "200"] => "#/components/schemas/AdminRoute",
      ["/api/v1/admin/routes/{routeId}", "delete", "200"] => "#/components/schemas/AdminDeleteStatusResponse",
      ["/api/v1/admin/plans", "get", "200"] => "#/components/schemas/AdminPlanListResponse",
      ["/api/v1/admin/plans", "post", "201"] => "#/components/schemas/AdminPlan",
      ["/api/v1/admin/plans/{planId}", "get", "200"] => "#/components/schemas/AdminPlan",
      ["/api/v1/admin/plans/{planId}", "put", "200"] => "#/components/schemas/AdminPlan",
      ["/api/v1/admin/plans/{planId}", "delete", "200"] => "#/components/schemas/AdminDeactivateStatusResponse",
      ["/api/v1/admin/users", "get", "200"] => "#/components/schemas/AdminUserListResponse",
      ["/api/v1/admin/users/{userId}", "get", "200"] => "#/components/schemas/AdminUser",
      ["/api/v1/admin/users/{userId}", "put", "200"] => "#/components/schemas/AdminUser",
      ["/api/v1/admin/users/{userId}", "patch", "200"] => "#/components/schemas/AdminUser",
      ["/api/v1/admin/users/{userId}", "delete", "200"] => "#/components/schemas/AdminDeleteStatusResponse",
      ["/api/v1/admin/users/{userId}/disable", "post", "200"] => "#/components/schemas/AdminUserStatusResponse",
      ["/api/v1/admin/users/{userId}/enable", "post", "200"] => "#/components/schemas/AdminUserStatusResponse",
      ["/api/v1/admin/audit-logs", "get", "200"] => "#/components/schemas/AdminAuditLogListResponse",
    }

    expected_data_refs.each do |(path, method, status), expected|
      op = operation(paths, path, method, missing)
      unless response_data_ref(op, status) == expected
        missing << "#{method.upcase} #{path} #{status} data must reference #{expected}"
      end
      unless op.fetch("tags", []).include?("Admin")
        missing << "#{method.upcase} #{path} must be tagged Admin"
      end
      if method == "get" && !requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
      end
    end

    ["/api/v1/admin/settings/relay-pricing", "/api/v1/admin/settings/usage-limits", "/api/v1/admin/routes", "/api/v1/admin/routes/{routeId}", "/api/v1/admin/plans", "/api/v1/admin/plans/{planId}", "/api/v1/admin/users/{userId}", "/api/v1/admin/users/{userId}/disable", "/api/v1/admin/users/{userId}/enable"].each do |path|
      methods = paths.fetch(path, {}).keys.select { |method| ["post", "put", "patch", "delete"].include?(method) }
      methods.each do |method|
        op = operation(paths, path, method, missing)
        unless requires_cookie_and_csrf?(op)
          missing << "#{method.upcase} #{path} must require cookieAuth and csrfHeader"
        end
      end
    end

    {
      ["/api/v1/admin/settings/relay-pricing", "put"] => "#/components/schemas/AdminRelayPricingSettings",
      ["/api/v1/admin/settings/usage-limits", "put"] => "#/components/schemas/AdminUsageLimitSettings",
      ["/api/v1/admin/routes", "post"] => "#/components/schemas/AdminRouteCreateRequest",
      ["/api/v1/admin/routes/{routeId}", "put"] => "#/components/schemas/AdminRouteUpdateRequest",
      ["/api/v1/admin/plans", "post"] => "#/components/schemas/AdminPlanCreateRequest",
      ["/api/v1/admin/plans/{planId}", "put"] => "#/components/schemas/AdminPlanUpdateRequest",
      ["/api/v1/admin/users/{userId}", "put"] => "#/components/schemas/AdminUserUpdateRequest",
      ["/api/v1/admin/users/{userId}", "patch"] => "#/components/schemas/AdminUserQuotaUpdateRequest",
    }.each do |(path, method), expected|
      op = operation(paths, path, method, missing)
      unless request_body_ref(op) == expected
        missing << "#{method.upcase} #{path} request body must reference #{expected}"
      end
    end

    {
      "AdminStats" => ["users", "quotas", "channelsTotal", "apiCalls24h"],
      "AdminRelayPricingSettings" => ["modelMultipliers", "groupMultipliers"],
      "AdminUsageLimitSettings" => ["organizationId", "userId", "quotaMode", "maxConcurrentRequests", "windowSeconds", "maxTokensPerWindow", "maxTokensPerRequest"],
      "AdminRoute" => ["id", "model", "strategy", "channels", "createdAt"],
      "AdminPlan" => ["id", "name", "quotaAmount", "tokenQuota", "maxTokensPerRequest", "isActive"],
      "AdminUser" => ["id", "email", "role", "status", "createdAt"],
      "AdminAuditLogEntry" => ["id", "actorID", "actorEmail", "action", "resourceType", "createdAt"],
    }.each do |schema_name, properties|
      props = schemas.fetch(schema_name, {}).fetch("properties", {})
      properties.each do |property|
        missing << "#{schema_name}.#{property} must be documented" unless props.key?(property)
      end
    end

    unless schemas.dig("AdminRelayPricingSettings", "properties", "modelMultipliers", "additionalProperties", "format") == "double" &&
        schemas.dig("AdminRelayPricingSettings", "properties", "groupMultipliers", "additionalProperties", "format") == "double"
      missing << "AdminRelayPricingSettings must document model/group multiplier maps"
    end
    unless schemas.dig("AdminUsageLimitSettings", "properties", "quotaMode", "enum")&.include?("user") &&
        schemas.dig("AdminUsageLimitSettings", "properties", "maxTokensPerRequest", "type") == "integer" &&
        schemas.dig("AdminUsageLimitSettingsListResponse", "properties", "usageLimits", "items", "$ref") == "#/components/schemas/AdminUsageLimitSettings"
      missing << "AdminUsageLimitSettings schemas must document scoped usage limits and request-token cap"
    end
    unless schemas.dig("AdminUserQuotaUpdateRequest", "required")&.include?("balance") &&
        schemas.dig("AdminUserQuotaUpdateRequest", "properties", "balance", "minimum") == 0
      missing << "AdminUserQuotaUpdateRequest.balance must be required and non-negative"
    end
    changes = schemas.dig("AdminAuditLogEntry", "properties", "changes") || {}
    changes_description = changes.fetch("description", "")
    normalized_changes_description = changes_description.downcase
    unless changes["type"] == "string" &&
        ["redacted", "credential"].all? { |word| normalized_changes_description.include?(word) } &&
        changes_description.include?("apiKey")
      missing << "AdminAuditLogEntry.changes must document redacted credential fields including apiKey"
    end

    {
      "AdminRouteListResponse" => ["routes", "#/components/schemas/AdminRoute"],
      "AdminPlanListResponse" => ["plans", "#/components/schemas/AdminPlan"],
      "AdminUserListResponse" => ["users", "#/components/schemas/AdminUser"],
      "AdminAuditLogListResponse" => ["entries", "#/components/schemas/AdminAuditLogEntry"],
    }.each do |schema_name, (collection_property, item_ref)|
      schema = schemas[schema_name] || {}
      unless schema.dig("properties", collection_property, "items", "$ref") == item_ref &&
          schema.dig("properties", "total", "type") == "integer"
        missing << "#{schema_name} must expose #{collection_property}[] plus integer total"
      end
    end

    unless missing.empty?
      warn "[openapi-contract] Admin core management contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
}

require_admin_billing_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
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

    def requires_cookie_without_csrf?(operation)
      security = operation.fetch("security", [])
      security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
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
      ["/api/v1/admin/billing/payouts/create-due", "post"] => "#/components/schemas/AdminMarketplacePayoutsResponse",
      ["/api/v1/admin/billing/payouts/{payoutId}/paid", "post"] => "#/components/schemas/MarketplacePayout",
      ["/api/v1/admin/billing/payouts/{payoutId}/failed", "post"] => "#/components/schemas/MarketplacePayout",
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
      if method == "get" && !requires_cookie_without_csrf?(op)
        missing << "#{method.upcase} #{path} must require cookieAuth without csrfHeader"
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
    refund_schema = schemas["AdminTopupRefundRequest"] || {}
    refund_required = refund_schema.fetch("required", [])
    ["provider", "providerRefundID", "amount", "currency"].each do |field|
      missing << "AdminTopupRefundRequest must require #{field}" unless refund_required.include?(field)
    end
    unless refund_schema.dig("properties", "amount", "minimum").to_f > 0
      missing << "AdminTopupRefundRequest.amount must document a positive minimum"
    end

    create_due = operation(paths, "/api/v1/admin/billing/payouts/create-due", "post", missing)
    unless requires_cookie_and_csrf?(create_due)
      missing << "POST /api/v1/admin/billing/payouts/create-due must require cookieAuth and csrfHeader"
    end

    paid = operation(paths, "/api/v1/admin/billing/payouts/{payoutId}/paid", "post", missing)
    unless request_body_ref(paid) == "#/components/schemas/AdminMarketplacePayoutPaidRequest"
      missing << "POST /api/v1/admin/billing/payouts/{payoutId}/paid must document AdminMarketplacePayoutPaidRequest body"
    end
    unless requires_cookie_and_csrf?(paid)
      missing << "POST /api/v1/admin/billing/payouts/{payoutId}/paid must require cookieAuth and csrfHeader"
    end
    paid_schema = schemas["AdminMarketplacePayoutPaidRequest"] || {}
    unless paid_schema.fetch("required", []).include?("providerPayoutID")
      missing << "AdminMarketplacePayoutPaidRequest must require providerPayoutID"
    end

    failed = operation(paths, "/api/v1/admin/billing/payouts/{payoutId}/failed", "post", missing)
    unless request_body_ref(failed) == "#/components/schemas/AdminMarketplacePayoutFailedRequest"
      missing << "POST /api/v1/admin/billing/payouts/{payoutId}/failed must document AdminMarketplacePayoutFailedRequest body"
    end
    unless requires_cookie_and_csrf?(failed)
      missing << "POST /api/v1/admin/billing/payouts/{payoutId}/failed must require cookieAuth and csrfHeader"
    end
    failed_schema = schemas["AdminMarketplacePayoutFailedRequest"] || {}
    ["providerPayoutID", "reason"].each do |field|
      missing << "AdminMarketplacePayoutFailedRequest must require #{field}" unless failed_schema.fetch("required", []).include?(field)
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

    webhook_event_schema = schemas["AdminWebhookEventInspection"] || {}
    webhook_event_properties = webhook_event_schema.fetch("properties", {})
    ["payload", "rawPayload", "providerPayload"].each do |property|
      if webhook_event_properties.key?(property)
        missing << "AdminWebhookEventInspection must not document raw provider payload fields"
      end
    end

    topup_schema = schemas["AdminTopupInspection"] || {}
    ["provider", "providerPaymentIntentId", "currency"].each do |property|
      unless topup_schema.dig("properties", property, "type") == "string"
        missing << "AdminTopupInspection must document #{property} as string provider refund evidence"
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
    spec = YAML.unsafe_load_file(file)
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
    spec = YAML.unsafe_load_file(file)
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

require_websocket_contract() {
  ruby -ryaml -e '
    file = ARGV.fetch(0)
    spec = YAML.unsafe_load_file(file)
    op = spec.fetch("paths", {}).fetch("/api/v1/ws", {}).fetch("get", {})
    missing = []

    missing << "GET /api/v1/ws must be documented" if op.empty?
    unless op.fetch("tags", []).include?("Realtime")
      missing << "GET /api/v1/ws must be tagged Realtime"
    end

    security = op.fetch("security", spec.fetch("security", []))
    unless security.any? { |entry| entry.is_a?(Hash) && entry.key?("cookieAuth") && !entry.key?("csrfHeader") }
      missing << "GET /api/v1/ws must require cookieAuth without csrfHeader"
    end
    unless op.dig("responses", "101")
      missing << "GET /api/v1/ws must document 101 WebSocket upgrade"
    end
    unless op.dig("responses", "401", "$ref") == "#/components/responses/Unauthorized"
      missing << "GET /api/v1/ws 401 must reference Unauthorized"
    end
    unless op.dig("responses", "405", "$ref") == "#/components/responses/MethodNotAllowed"
      missing << "GET /api/v1/ws 405 must reference MethodNotAllowed"
    end
    unless op.dig("x-websocket-client-message", "$ref") == "#/components/schemas/ChatRealtimeClientMessage"
      missing << "GET /api/v1/ws must document ChatRealtimeClientMessage as the client frame"
    end
    unless op.dig("x-websocket-server-message", "$ref") == "#/components/schemas/ChatRealtimeEvent"
      missing << "GET /api/v1/ws must document ChatRealtimeEvent as the server frame"
    end
    schemas = spec.fetch("components", {}).fetch("schemas", {})
    %w[
      ChatRealtimeClientMessage
      ChatRealtimeEvent
      ChatMessagesSyncedPayload
      ChatMessageUpdatedPayload
      ChatMessageDeletedPayload
      ChatTypingPayload
    ].each do |schema_name|
      missing << "components.schemas.#{schema_name} must be documented for /api/v1/ws chat realtime" unless schemas.key?(schema_name)
    end

    unless missing.empty?
      warn "[openapi-contract] WebSocket contract is incomplete:"
      missing.each { |entry| warn "  - #{entry}" }
      exit 1
    end
  ' "$openapi_file"
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
  "/api/v1/agent/runs/{runId}/continue-plan"
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
  "/api/v1/admin/billing/payouts/create-due"
  "/api/v1/admin/billing/payouts/{payoutId}/paid"
  "/api/v1/admin/billing/payouts/{payoutId}/failed"
  "/api/v1/admin/stats"
  "/api/v1/admin/api-tokens"
  "/api/v1/admin/api-tokens/{tokenId}/revoke"
  "/api/v1/admin/routes"
  "/api/v1/admin/routes/{routeId}"
  "/api/v1/admin/plans"
  "/api/v1/admin/plans/{planId}"
  "/api/v1/admin/users"
  "/api/v1/admin/users/{userId}"
  "/api/v1/admin/users/{userId}/disable"
  "/api/v1/admin/users/{userId}/enable"
  "/api/v1/admin/audit-logs"
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
  "/api/v1/app/knowledge-bases"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/upload"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieve"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases"
  "/api/v1/app/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run"
  "/api/v1/knowledge-bases"
  "/api/v1/knowledge-bases/{knowledgeBaseId}"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/upload"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/versions"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/split"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/documents/{documentId}/chunks/{chunkId}/merge"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieve"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases"
  "/api/v1/knowledge-bases/{knowledgeBaseId}/retrieval-test-cases/run"
  "/api/v1/documents/{documentId}"
  "/api/v1/app/notifications"
  "/api/v1/app/notifications/unread-count"
  "/api/v1/app/notifications/mark-all-read"
  "/api/v1/app/notifications/{notificationId}"
  "/api/v1/app/quota"
  "/api/v1/app/packages"
  "/api/v1/app/quota/topup"
  "/api/v1/app/personas"
  "/api/v1/app/personas/{personaId}"
  "/api/v1/ws"
  "/api/v1/app/conversations/{conversationId}/share"
  "/api/v1/app/conversations/{conversationId}/messages/{messageId}/share"
  "/api/v1/app/message-shares/{shareId}"
  "/api/v1/app/conversation-shares/{shareId}"
  "/api/v1/scheduled-tasks"
  "/api/v1/scheduled-tasks/{scheduledTaskId}/runs"
  "/api/v1/scheduled-tasks/{scheduledTaskId}/status"
  "/api/v1/scheduled-tasks/{scheduledTaskId}/run"
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
require_websocket_contract
require_api_json_responses_use_envelope
require_api_success_data_uses_named_schema
require_api_json_request_bodies_use_named_schemas
require_api_security_surface_contract
require_api_path_parameter_contract
require_api_operation_metadata_contract
require_route_surface_manifest_contract
require_session_csrf_contract
require_marketplace_paid_install_contract
require_marketplace_template_type_contract
require_marketplace_surface_payload_contract
require_marketplace_browse_payload_contract
require_marketplace_private_read_auth_contract
require_marketplace_public_read_contract
require_publishing_channel_secret_csrf_contract
require_admin_channel_secret_response_contract
require_admin_observability_provider_secret_csrf_contract
require_mcp_auth_token_response_contract
require_marketplace_user_mutation_csrf_contract
require_admin_marketplace_governance_csrf_contract
require_admin_marketplace_review_csrf_contract
require_workspace_agent_mutation_csrf_contract
require_memory_mutation_csrf_contract
require_agent_run_mutation_csrf_contract
require_billing_checkout_contract
require_quota_topup_csrf_contract
require_tenant_organization_mutation_csrf_contract
require_workflow_management_csrf_contract
require_workflow_execution_control_csrf_contract
require_console_api_token_csrf_contract
require_admin_api_token_contract
require_task_mutation_csrf_contract
require_notification_mutation_csrf_contract
require_scheduled_task_contract
require_preferences_mutation_csrf_contract
require_chat_mutation_csrf_contract
require_knowledge_mutation_csrf_contract
require_admin_organization_mutation_csrf_contract
require_admin_core_management_contract
require_admin_billing_contract
require_domestic_payment_webhook_payout_contract

echo "[openapi-contract] required Relay alias, Agent, Memory, MCP, Tenant, Notification, Scheduled Task, Observability, publishing channel, Workflow, Billing, and Marketplace paths are documented."
