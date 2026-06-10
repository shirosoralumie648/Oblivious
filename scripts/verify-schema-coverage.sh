#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
postgres_dir="$repo_root/src/server/migrations"
clickhouse_dir="$repo_root/src/server/migrations/clickhouse"

assert_schema_evidence() {
  local family="$1"
  shift

  local missing=()
  for pattern in "$@"; do
    if ! rg -q -- "$pattern" "$postgres_dir" "$clickhouse_dir"; then
      missing+=("$pattern")
    fi
  done

  if (( ${#missing[@]} > 0 )); then
    echo "[schema-coverage] Part 3 core schema family '$family' is missing migration evidence:" >&2
    printf '  - %s\n' "${missing[@]}" >&2
    exit 1
  fi

  echo "[schema-coverage] Part 3 core schema family '$family' has migration evidence."
}

assert_schema_evidence "user and organization" \
  "CREATE TABLE IF NOT EXISTS organizations" \
  "CREATE TABLE IF NOT EXISTS users" \
  "CREATE TABLE IF NOT EXISTS organization_memberships"

assert_schema_evidence "chat" \
  "CREATE TABLE IF NOT EXISTS conversations" \
  "CREATE TABLE IF NOT EXISTS messages" \
  "CREATE TABLE IF NOT EXISTS personas" \
  "ADD COLUMN IF NOT EXISTS parent_id"

assert_schema_evidence "relay and gateway" \
  "CREATE TABLE IF NOT EXISTS channels" \
  "CREATE TABLE IF NOT EXISTS rate_limit_counters" \
  "CREATE TABLE IF NOT EXISTS relay_semantic_cache" \
  "CREATE TABLE IF NOT EXISTS relay_metrics"

assert_schema_evidence "workflow" \
  "CREATE TABLE IF NOT EXISTS workflows" \
  "CREATE TABLE IF NOT EXISTS workflow_executions" \
  "CREATE TABLE IF NOT EXISTS workflow_node_executions" \
  "CREATE TABLE IF NOT EXISTS workflow_versions"

assert_schema_evidence "knowledge and rag" \
  "CREATE TABLE IF NOT EXISTS knowledge_bases" \
  "CREATE TABLE IF NOT EXISTS knowledge_documents" \
  "CREATE TABLE IF NOT EXISTS knowledge_document_chunks" \
  "CREATE EXTENSION IF NOT EXISTS vector"

assert_schema_evidence "agent" \
  "CREATE TABLE IF NOT EXISTS agent_runs" \
  "CREATE TABLE IF NOT EXISTS agent_tool_runs" \
  "CREATE TABLE IF NOT EXISTS agent_memories" \
  "CREATE TABLE IF NOT EXISTS agent_plan_steps"

assert_schema_evidence "billing" \
  "CREATE TABLE IF NOT EXISTS subscriptions" \
  "CREATE TABLE IF NOT EXISTS payment_intents" \
  "CREATE TABLE IF NOT EXISTS quotas" \
  "CREATE TABLE IF NOT EXISTS concurrency_limits" \
  "CREATE TABLE IF NOT EXISTS token_rate_limits" \
  "CREATE TABLE IF NOT EXISTS billing_lifecycle_events" \
  "CREATE TABLE IF NOT EXISTS billing_invoices" \
  "CREATE TABLE IF NOT EXISTS billing_refunds"

assert_schema_evidence "marketplace" \
  "CREATE TABLE IF NOT EXISTS published_agents" \
  "CREATE TABLE IF NOT EXISTS agent_installs" \
  "CREATE TABLE IF NOT EXISTS agent_reviews" \
  "CREATE TABLE IF NOT EXISTS marketplace_orders" \
  "CREATE TABLE IF NOT EXISTS marketplace_settlements" \
  "CREATE TABLE IF NOT EXISTS marketplace_templates" \
  "ALTER COLUMN category_id SET NOT NULL" \
  "published_agents_category_id_fkey" \
  "FOREIGN KEY \(category_id\) REFERENCES categories\(id\)" \
  "idx_published_agents_category_id"

assert_schema_evidence "channel" \
  "CREATE TABLE IF NOT EXISTS channel_configs" \
  "CREATE TABLE IF NOT EXISTS channel_messages"

assert_schema_evidence "task" \
  "CREATE TABLE IF NOT EXISTS scheduled_tasks" \
  "CREATE TABLE IF NOT EXISTS task_executions"

assert_schema_evidence "observability" \
  "CREATE TABLE IF NOT EXISTS request_logs" \
  "CREATE TABLE IF NOT EXISTS observability_alert_states" \
  "CREATE TABLE IF NOT EXISTS observability_alert_delivery_attempts"

echo "[schema-coverage] Part 3 core schema family coverage verified across src/server/migrations and src/server/migrations/clickhouse."
