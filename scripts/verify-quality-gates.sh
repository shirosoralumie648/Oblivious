#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

assert_file_exists() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "[quality-gates] missing file: $path" >&2
    exit 1
  fi
}

assert_file_contains() {
  local path="$1"
  local pattern="$2"
  if ! rg -q --fixed-strings "$pattern" "$path"; then
    echo "[quality-gates] expected pattern '$pattern' in $path" >&2
    exit 1
  fi
}

assert_file_not_contains() {
  local path="$1"
  local pattern="$2"
  if rg -q --fixed-strings "$pattern" "$path"; then
    echo "[quality-gates] unexpected pattern '$pattern' in $path" >&2
    exit 1
  fi
}

workflow_file="$repo_root/.github/workflows/ci.yml"
workspace_file="$repo_root/pnpm-workspace.yaml"
gitignore_file="$repo_root/.gitignore"
package_file="$repo_root/package.json"
readme_file="$repo_root/README.md"
api_file="$repo_root/docs/API.md"
contracts_file="$repo_root/docs/architecture/current-system-contracts.md"
release_checklist_file="$repo_root/docs/release/rc-checklist.md"
commercial_gates_file="$repo_root/docs/release/commercial-gates.md"
relay_route_table_file="$repo_root/docs/release/relay-route-table.md"
deployment_remediation_file="$repo_root/docs/release/deployment-runtime-remediation.md"
dockerfile_server_file="$repo_root/Dockerfile.server"
dockerfile_web_file="$repo_root/Dockerfile.web"
dockerignore_file="$repo_root/.dockerignore"
compose_file="$repo_root/docker-compose.yml"
deploy_smoke_file="$repo_root/scripts/deploy-smoke.sh"
deploy_validate_file="$repo_root/scripts/deploy-validate.sh"
relay_security_file="$repo_root/scripts/verify-relay-security.sh"
k8s_namespace_file="$repo_root/deploy/kubernetes/namespace.yaml"
k8s_configmap_file="$repo_root/deploy/kubernetes/configmap.yaml"
k8s_secret_example_file="$repo_root/deploy/kubernetes/secret.example.yaml"
k8s_postgres_file="$repo_root/deploy/kubernetes/postgres.yaml"
k8s_redis_file="$repo_root/deploy/kubernetes/redis.yaml"
k8s_server_file="$repo_root/deploy/kubernetes/server.yaml"
k8s_web_file="$repo_root/deploy/kubernetes/web.yaml"
owner_matrix_file="$repo_root/docs/governance/owner-matrix.md"
weekly_status_file="$repo_root/docs/governance/weekly-status-template.md"
blocker_escalation_file="$repo_root/docs/governance/blocker-escalation.md"
check_script="$repo_root/scripts/check.sh"
test_script="$repo_root/scripts/test.sh"

assert_file_exists "$workflow_file"
assert_file_exists "$readme_file"
assert_file_exists "$api_file"
assert_file_exists "$contracts_file"
assert_file_exists "$release_checklist_file"
assert_file_exists "$commercial_gates_file"
assert_file_exists "$relay_route_table_file"
assert_file_exists "$deployment_remediation_file"
assert_file_exists "$dockerfile_server_file"
assert_file_exists "$dockerfile_web_file"
assert_file_exists "$dockerignore_file"
assert_file_exists "$compose_file"
assert_file_exists "$deploy_smoke_file"
assert_file_exists "$deploy_validate_file"
assert_file_exists "$relay_security_file"
assert_file_exists "$k8s_namespace_file"
assert_file_exists "$k8s_configmap_file"
assert_file_exists "$k8s_secret_example_file"
assert_file_exists "$k8s_postgres_file"
assert_file_exists "$k8s_redis_file"
assert_file_exists "$k8s_server_file"
assert_file_exists "$k8s_web_file"
assert_file_exists "$owner_matrix_file"
assert_file_exists "$weekly_status_file"
assert_file_exists "$blocker_escalation_file"

assert_file_not_contains "$workflow_file" "phase0-task1-contracts"
assert_file_contains "$workflow_file" "web:"
assert_file_contains "$workflow_file" "server:"
assert_file_contains "$workflow_file" "postgres:16"
assert_file_contains "$workflow_file" "TEST_DATABASE_URL"
assert_file_contains "$workflow_file" "OBLIVIOUS_REQUIRE_TEST_DATABASE"
assert_file_contains "$workflow_file" "bash scripts/check.sh"
assert_file_contains "$workflow_file" "bash scripts/check.sh relay-security"
assert_file_contains "$workflow_file" "bash scripts/test.sh"
assert_file_contains "$workflow_file" "bash scripts/test.sh server"

assert_file_contains "$package_file" '"dev": "bash scripts/dev.sh"'
assert_file_contains "$package_file" '"check": "bash scripts/check.sh"'
assert_file_contains "$package_file" '"test": "bash scripts/test.sh"'

assert_file_not_contains "$workspace_file" '"lobehub"'
assert_file_not_contains "$workspace_file" '"new-api"'
assert_file_contains "$gitignore_file" ".superpowers/"

assert_file_contains "$readme_file" "## Quick Start"
assert_file_contains "$readme_file" "bash scripts/check.sh"
assert_file_contains "$readme_file" "bash scripts/test.sh"
assert_file_contains "$readme_file" "docs/API.md"
assert_file_contains "$readme_file" "docs/release/rc-checklist.md"
assert_file_contains "$readme_file" "docs/release/commercial-gates.md"
assert_file_not_contains "$readme_file" ".worktrees/phase0-task1-contracts"

assert_file_contains "$api_file" "## Admin Endpoints"
assert_file_contains "$api_file" "## Marketplace Endpoints"
assert_file_contains "$api_file" "## Relay /v1 Endpoints"
assert_file_contains "$api_file" "/api/v1/admin/channels"
assert_file_contains "$api_file" "/api/v1/marketplace/search"
assert_file_contains "$api_file" "/v1/chat/completions"
assert_file_contains "$api_file" "docs/release/relay-route-table.md"

assert_file_contains "$contracts_file" "/api/v1/admin/stats"
assert_file_contains "$contracts_file" "/api/v1/marketplace/featured"
assert_file_contains "$contracts_file" "/marketplace/my-agents"
assert_file_contains "$contracts_file" "COREPACK_HOME=.tmp/corepack pnpm --dir src/web test:e2e"
assert_file_contains "$contracts_file" "lobehub/"
assert_file_contains "$contracts_file" "new-api/"

assert_file_contains "$release_checklist_file" "Web production build"
assert_file_contains "$release_checklist_file" "Docs and release assets"
assert_file_contains "$release_checklist_file" "Server release checks"
assert_file_contains "$release_checklist_file" "Documentation Evidence"
assert_file_contains "$release_checklist_file" "No P0/P1 defects open"
assert_file_contains "$release_checklist_file" "TEST_DATABASE_URL"
assert_file_contains "$release_checklist_file" "pnpm --dir src/web test:e2e"
assert_file_contains "$release_checklist_file" "Known accepted debt"
assert_file_contains "$release_checklist_file" "commercial-gates.md"
assert_file_contains "$release_checklist_file" "No live provider keys required"
assert_file_contains "$release_checklist_file" "docker compose build"
assert_file_contains "$release_checklist_file" "bash scripts/deploy-validate.sh"
assert_file_contains "$release_checklist_file" "bash scripts/deploy-smoke.sh"
assert_file_contains "$release_checklist_file" "kubectl apply -f deploy/kubernetes/"
assert_file_contains "$release_checklist_file" "deployment-runtime-remediation.md"
assert_file_not_contains "$release_checklist_file" ".worktrees/phase0-task1-contracts"

assert_file_contains "$commercial_gates_file" "Commercial Readiness Gates"
assert_file_contains "$commercial_gates_file" "Tenant And Identity Gate"
assert_file_contains "$commercial_gates_file" "Relay Authority Gate"
assert_file_contains "$commercial_gates_file" "Billing And Monetization Gate"
assert_file_contains "$commercial_gates_file" "Product Completeness Gate"
assert_file_contains "$commercial_gates_file" "Security Gate"
assert_file_contains "$commercial_gates_file" "Operations Gate"
assert_file_contains "$commercial_gates_file" "Verification Gate"
assert_file_contains "$commercial_gates_file" "v05"
assert_file_contains "$commercial_gates_file" "v06"
assert_file_contains "$commercial_gates_file" "v07"
assert_file_contains "$commercial_gates_file" "v08"
assert_file_contains "$commercial_gates_file" "docs/release/relay-route-table.md"
assert_file_contains "$commercial_gates_file" "scripts/verify-relay-security.sh"

assert_file_contains "$relay_route_table_file" "Relay Route Table"
assert_file_contains "$relay_route_table_file" "CommercialSupportedBilled"
assert_file_contains "$relay_route_table_file" "DisabledInProduction"
assert_file_contains "$relay_route_table_file" "endpoint_disabled_in_production"
assert_file_contains "$relay_route_table_file" "trusted_internal_identity"
assert_file_contains "$relay_route_table_file" "global_relay_token_bucket"
assert_file_contains "$relay_route_table_file" "relay_route_policy_decision"
assert_file_contains "$relay_route_table_file" "/v1/chat/completions"
assert_file_contains "$relay_route_table_file" "/v1/files"
assert_file_contains "$relay_route_table_file" "/v1/fine_tuning/jobs"
assert_file_contains "$relay_route_table_file" "/v1/threads/:id/runs"

assert_file_contains "$deployment_remediation_file" "bash scripts/deploy-validate.sh"
assert_file_contains "$deployment_remediation_file" "docker info"
assert_file_contains "$deployment_remediation_file" "kubectl apply -f deploy/kubernetes/"

assert_file_contains "$dockerfile_server_file" "src/server/cmd/server"
assert_file_contains "$dockerfile_server_file" "EXPOSE 8080"
assert_file_contains "$dockerfile_web_file" "pnpm --dir src/web build"
assert_file_contains "$dockerignore_file" "node_modules"
assert_file_contains "$dockerignore_file" ".worktrees"
assert_file_contains "$dockerignore_file" "new-api"
assert_file_contains "$compose_file" "oblivious-server"
assert_file_contains "$compose_file" "oblivious-web"
assert_file_contains "$compose_file" "postgres:16"
assert_file_contains "$compose_file" "redis:7"
assert_file_contains "$compose_file" "DATABASE_URL=postgres://oblivious:oblivious@postgres:5432/oblivious?sslmode=disable"
assert_file_not_contains "$compose_file" "sk-"
assert_file_contains "$deploy_smoke_file" 'BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"'
assert_file_contains "$deploy_smoke_file" "/healthz"
assert_file_contains "$deploy_smoke_file" "healthz ok"
assert_file_contains "$deploy_validate_file" "docker compose build"
assert_file_contains "$deploy_validate_file" "docker compose up -d"
assert_file_contains "$deploy_validate_file" "bash scripts/deploy-smoke.sh"
assert_file_contains "$deploy_validate_file" "docker daemon is not reachable"
assert_file_contains "$relay_security_file" "app services are Relay-only for provider calls"
assert_file_contains "$relay_security_file" "NewHTTPReplyGenerator"
assert_file_contains "$relay_security_file" "TenantIdentityRequired"
assert_file_contains "$k8s_namespace_file" "name: oblivious"
assert_file_contains "$k8s_configmap_file" "RELAY_ENABLED"
assert_file_contains "$k8s_secret_example_file" "REPLACE_ME"
assert_file_contains "$k8s_server_file" "/healthz"
assert_file_contains "$k8s_server_file" "oblivious-server:local"
assert_file_contains "$k8s_web_file" "oblivious-web:local"
for k8s_file in "$repo_root"/deploy/kubernetes/*.yaml; do
  assert_file_not_contains "$k8s_file" "sk-"
done

assert_file_contains "$owner_matrix_file" "| TL |"
assert_file_contains "$owner_matrix_file" "| FE |"
assert_file_contains "$owner_matrix_file" "| BE |"
assert_file_contains "$weekly_status_file" "Actual Owner:"
assert_file_contains "$weekly_status_file" "## Risks / Blockers"
assert_file_contains "$blocker_escalation_file" "| Severity | Definition | Response Window | Escalate To |"

assert_file_contains "$check_script" "scripts/verify-quality-gates.sh"
assert_file_contains "$check_script" "scripts/verify-relay-security.sh"
assert_file_contains "$check_script" "relay-security"
assert_file_contains "$check_script" 'workspace_file="$repo_root/pnpm-workspace.yaml"'
assert_file_contains "$check_script" 'Unexpected workspace member: lobehub'
assert_file_contains "$check_script" 'Unexpected workspace member: new-api'
assert_file_contains "$check_script" 'pnpm --dir "$web_dir" build'
assert_file_contains "$check_script" "docs/architecture/current-system-contracts.md"
assert_file_contains "$check_script" "config/.env.example"
assert_file_contains "$check_script" "go test ./... -count=1"

assert_file_contains "$test_script" "Running server unit tests."
assert_file_contains "$test_script" "Running server integration tests."
assert_file_contains "$test_script" "OBLIVIOUS_REQUIRE_TEST_DATABASE"
assert_file_contains "$test_script" "TEST_DATABASE_URL is required when OBLIVIOUS_REQUIRE_TEST_DATABASE=true."
assert_file_contains "$test_script" "Skipping server integration tests: TEST_DATABASE_URL not set."
assert_file_contains "$test_script" "go test ./... -count=1"
assert_file_contains "$test_script" "go test ./internal/http"

echo "[quality-gates] quality gate assets look complete."
