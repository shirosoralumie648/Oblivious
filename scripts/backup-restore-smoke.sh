#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source_url="${BACKUP_SMOKE_SOURCE_DATABASE_URL:-}"
restore_url="${BACKUP_SMOKE_RESTORE_DATABASE_URL:-}"
postgres_image="${OBLIVIOUS_POSTGRES_IMAGE:-pgvector/pgvector:pg16}"
client_image="${PG_CLIENT_IMAGE:-$postgres_image}"
client_network="${PG_CLIENT_DOCKER_NETWORK:-host}"
run_id="phase22_$(date -u +%Y%m%d%H%M%S)_$$"
source_container=""
restore_container=""

fail() {
  echo "[backup-restore-smoke] $*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$source_container" ]]; then
    docker rm -f "$source_container" >/dev/null 2>&1 || true
  fi
  if [[ -n "$restore_container" ]]; then
    docker rm -f "$restore_container" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

require_tool() {
  local tool="$1"
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "[backup-restore-smoke] $tool is required" >&2
    exit 127
  fi
}

psql_client() {
  local database="$1"
  shift

  if command -v psql >/dev/null 2>&1; then
    psql -X -v ON_ERROR_STOP=1 "$database" "$@"
    return
  fi

  docker run --rm -i \
    --network "$client_network" \
    "$client_image" \
    psql -X -v ON_ERROR_STOP=1 "$database" "$@"
}

psql_scalar() {
  local database="$1"
  local query="$2"
  psql_client "$database" -Atc "$query"
}

wait_for_database() {
  local database="$1"
  local label="$2"
  local attempt

  for attempt in $(seq 1 60); do
    if psql_client "$database" -Atc "SELECT 1;" >/dev/null 2>&1; then
      echo "[backup-restore-smoke] database ready: $label"
      return
    fi
    sleep 1
  done

  fail "database did not become ready: $label"
}

start_database_container() {
  local name="$1"
  local port

  docker run -d --rm \
    --name "$name" \
    -e POSTGRES_DB=oblivious \
    -e POSTGRES_USER=oblivious \
    -e POSTGRES_PASSWORD=oblivious \
    -p 127.0.0.1::5432 \
    "$postgres_image" >/dev/null

  port=$(docker port "$name" 5432/tcp | sed -E 's/.*:([0-9]+)$/\1/')
  [[ -n "$port" ]] || fail "could not resolve mapped port for $name"
  printf 'postgres://oblivious:oblivious@127.0.0.1:%s/oblivious?sslmode=disable' "$port"
}

ensure_empty_database() {
  local database="$1"
  local count

  count=$(psql_scalar "$database" "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE';")
  if [[ "$count" != "0" && "${BACKUP_RESTORE_ALLOW_NON_EMPTY:-false}" != "true" ]]; then
    fail "restore target is not fresh: found $count public tables"
  fi
}

apply_migrations() {
  echo "[backup-restore-smoke] applying migrations to source"
  (
    cd "$repo_root/src/server"
    DATABASE_URL="$source_url" \
      SESSION_SECRET=phase22-smoke-secret \
      go run ./cmd/migrate
  )
}

seed_fixture() {
  echo "[backup-restore-smoke] seeding commercial tenant fixture"
  psql_client "$source_url" <<'SQL'
INSERT INTO users (id, email, password_hash, name, role, status)
VALUES
  ('phase22_owner', 'phase22-owner@example.test', 'hash', 'Phase 22 Owner', 'admin', 'active'),
  ('phase22_member', 'phase22-member@example.test', 'hash', 'Phase 22 Member', 'user', 'active'),
  ('phase22_buyer', 'phase22-buyer@example.test', 'hash', 'Phase 22 Buyer', 'user', 'active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO organizations (id, slug, name, status, metadata, created_by_user_id)
VALUES
  ('phase22_org_publisher', 'phase22-publisher', 'Phase 22 Publisher', 'active', '{"fixture":"phase22"}', 'phase22_owner'),
  ('phase22_org_buyer', 'phase22-buyer', 'Phase 22 Buyer', 'active', '{"fixture":"phase22"}', 'phase22_buyer')
ON CONFLICT (id) DO NOTHING;

INSERT INTO organization_memberships (id, organization_id, user_id, role, created_by_user_id)
VALUES
  ('phase22_membership_owner', 'phase22_org_publisher', 'phase22_owner', 'owner', 'phase22_owner'),
  ('phase22_membership_member', 'phase22_org_publisher', 'phase22_member', 'member', 'phase22_owner'),
  ('phase22_membership_buyer', 'phase22_org_buyer', 'phase22_buyer', 'owner', 'phase22_buyer')
ON CONFLICT (id) DO NOTHING;

INSERT INTO packages (id, name, description, quota_amount, price, duration_days, is_active, sort_order)
VALUES ('phase22_package', 'Phase 22 Commercial Plan', 'Backup restore smoke fixture', 1000.000000, 29.00, 30, true, 22)
ON CONFLICT (id) DO NOTHING;

INSERT INTO quotas (id, user_id, organization_id, balance, used)
VALUES
  ('phase22_quota_publisher', 'phase22_owner', 'phase22_org_publisher', 500.000000, 42.000000),
  ('phase22_quota_buyer', 'phase22_buyer', 'phase22_org_buyer', 300.000000, 12.000000)
ON CONFLICT (id) DO NOTHING;

INSERT INTO billing_sessions (id, user_id, organization_id, channel_id, model, api_type, idempotency_key, pre_authorized_amt, settled_amt, status, settled_at)
VALUES ('phase22_billing_session', 'phase22_owner', 'phase22_org_publisher', 'phase22_channel', 'gpt-4o-mini', 'chat.completions', 'phase22_idempotency', 1.230000, 1.000000, 'settled', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO payment_intents (id, provider, provider_checkout_session_id, organization_id, user_id, package_id, kind, amount, currency, status, metadata, provider_payment_intent_id, provider_subscription_id, provider_invoice_id, refunded_amount)
VALUES
  ('phase22_payment_subscription', 'stripe', 'phase22_cs_subscription', 'phase22_org_publisher', 'phase22_owner', 'phase22_package', 'subscription', 29.000000, 'usd', 'completed', '{"fixture":"phase22"}', 'phase22_pi_subscription', 'phase22_sub_provider', 'phase22_invoice_provider', 0),
  ('phase22_payment_topup', 'stripe', 'phase22_cs_topup', 'phase22_org_publisher', 'phase22_owner', 'phase22_package', 'topup', 10.000000, 'usd', 'partially_refunded', '{"fixture":"phase22"}', 'phase22_pi_topup', NULL, NULL, 2.000000),
  ('phase22_payment_marketplace', 'stripe', 'phase22_cs_marketplace', 'phase22_org_buyer', 'phase22_buyer', NULL, 'topup', 50.000000, 'usd', 'completed', '{"fixture":"phase22","marketplace_order_id":"phase22_marketplace_order"}', 'phase22_pi_marketplace', NULL, NULL, 0)
ON CONFLICT (id) DO NOTHING;

INSERT INTO subscriptions (id, user_id, organization_id, package_id, status, provider_subscription_id, provider_customer_id, provider_checkout_session_id, provider_latest_invoice_id, current_period_start, current_period_end)
VALUES ('phase22_subscription', 'phase22_owner', 'phase22_org_publisher', 'phase22_package', 'active', 'phase22_sub_provider', 'phase22_customer', 'phase22_cs_subscription', 'phase22_invoice_provider', NOW(), NOW() + INTERVAL '30 days')
ON CONFLICT (id) DO NOTHING;

INSERT INTO topup_orders (id, user_id, organization_id, amount, money, status, trade_no, paid_at, payment_intent_id, provider_checkout_session_id, refunded_amount)
VALUES ('phase22_topup', 'phase22_owner', 'phase22_org_publisher', 100.000000, 10.00, 'paid', 'phase22_trade', NOW(), 'phase22_payment_topup', 'phase22_cs_topup', 2.000000)
ON CONFLICT (id) DO NOTHING;

INSERT INTO stripe_webhook_events (id, provider, event_id, event_type, status, organization_id, user_id, payment_intent_id, payload, processed_at)
VALUES ('phase22_webhook', 'stripe', 'phase22_evt_subscription', 'checkout.session.completed', 'processed', 'phase22_org_publisher', 'phase22_owner', 'phase22_payment_subscription', '{"fixture":"phase22"}', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO billing_lifecycle_events (id, transition_key, provider, provider_event_id, event_type, organization_id, user_id, payment_intent_id, entity_type, entity_id, from_state, to_state, reason, payload)
VALUES ('phase22_lifecycle', 'phase22_transition', 'stripe', 'phase22_evt_subscription', 'checkout.session.completed', 'phase22_org_publisher', 'phase22_owner', 'phase22_payment_subscription', 'subscription', 'phase22_subscription', 'pending', 'active', 'phase22 smoke', '{"fixture":"phase22"}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO billing_invoices (id, provider, provider_invoice_id, provider_subscription_id, provider_payment_intent_id, organization_id, user_id, subscription_id, payment_intent_id, status, amount_due, amount_paid, currency, payload)
VALUES ('phase22_invoice', 'stripe', 'phase22_invoice_provider', 'phase22_sub_provider', 'phase22_pi_subscription', 'phase22_org_publisher', 'phase22_owner', 'phase22_subscription', 'phase22_payment_subscription', 'paid', 29.000000, 29.000000, 'usd', '{"fixture":"phase22"}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO billing_refunds (id, provider, provider_refund_id, provider_charge_id, provider_payment_intent_id, organization_id, user_id, payment_intent_id, topup_order_id, amount, currency, status, reason, payload)
VALUES ('phase22_refund', 'stripe', 'phase22_refund_provider', 'phase22_charge', 'phase22_pi_topup', 'phase22_org_publisher', 'phase22_owner', 'phase22_payment_topup', 'phase22_topup', 2.000000, 'usd', 'succeeded', 'requested_by_customer', '{"fixture":"phase22"}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO published_agents (id, owner_id, organization_id, name, description, category_id, tags, tools, example_conversations, system_prompt, visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, reviewed_at)
VALUES ('phase22_agent', 'phase22_owner', 'phase22_org_publisher', 'Phase 22 Agent', 'Backup restore marketplace fixture', 'cat_productivity', ARRAY['phase22'], '{"tools":["calculator"]}', '[]', 'You are a fixture.', 'public', 'approved', 'paid', 50.00, 1, 5.00, 1, NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status)
VALUES ('phase22_agent_version', 'phase22_agent', 'phase22_org_publisher', '1.0.0', 'Phase 22 fixture', '{"fixture":"phase22"}', 'approved')
ON CONFLICT (id) DO NOTHING;

INSERT INTO agent_installs (id, agent_id, organization_id, user_id, version_id)
VALUES ('phase22_install', 'phase22_agent', 'phase22_org_buyer', 'phase22_buyer', 'phase22_agent_version')
ON CONFLICT (id) DO NOTHING;

INSERT INTO marketplace_orders (id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id, agent_id, version_id, payment_intent_id, provider_checkout_session_id, provider_payment_intent_id, install_id, gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount, currency, status, paid_at)
VALUES ('phase22_marketplace_order', 'phase22_org_buyer', 'phase22_buyer', 'phase22_org_publisher', 'phase22_owner', 'phase22_agent', 'phase22_agent_version', 'phase22_payment_marketplace', 'phase22_cs_marketplace', 'phase22_pi_marketplace', 'phase22_install', 50.000000, 10.000000, 40.000000, 0, 'usd', 'paid', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO marketplace_payouts (id, publisher_organization_id, publisher_user_id, amount, currency, provider, provider_payout_id, status, metadata)
VALUES ('phase22_payout', 'phase22_org_publisher', 'phase22_owner', 40.000000, 'usd', 'local', 'phase22_payout_provider', 'pending', '{"fixture":"phase22"}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO marketplace_settlements (id, order_id, publisher_organization_id, publisher_user_id, agent_id, gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount, payout_id, status, hold_until)
VALUES ('phase22_settlement', 'phase22_marketplace_order', 'phase22_org_publisher', 'phase22_owner', 'phase22_agent', 50.000000, 10.000000, 40.000000, 0, 'phase22_payout', 'available', NOW() + INTERVAL '7 days')
ON CONFLICT (id) DO NOTHING;

INSERT INTO marketplace_governance_events (id, actor_user_id, actor_organization_id, agent_id, action, from_status, to_status, reason, metadata)
VALUES ('phase22_governance', 'phase22_owner', 'phase22_org_publisher', 'phase22_agent', 'approve', 'pending_review', 'approved', 'phase22 smoke', '{"fixture":"phase22"}')
ON CONFLICT (id) DO NOTHING;

INSERT INTO marketplace_abuse_reports (id, reporter_organization_id, reporter_user_id, agent_id, reason, details, status, resolution, reviewer_user_id, resolved_at)
VALUES ('phase22_abuse', 'phase22_org_buyer', 'phase22_buyer', 'phase22_agent', 'policy', 'Phase 22 smoke report', 'resolved', 'No issue', 'phase22_owner', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO audit_logs (id, actor_id, actor_email, action, resource_type, resource_id, organization_id, changes, ip_address, user_agent)
VALUES ('phase22_audit', 'phase22_owner', 'phase22-owner@example.test', 'phase22.backup_restore_smoke', 'organization', 'phase22_org_publisher', 'phase22_org_publisher', '{"fixture":"phase22"}', '127.0.0.1', 'phase22-smoke')
ON CONFLICT (id) DO NOTHING;
SQL
}

assert_count() {
  local label="$1"
  local query="$2"
  local expected="$3"
  local actual

  actual=$(psql_scalar "$restore_url" "$query")
  if [[ "$actual" != "$expected" ]]; then
    fail "$label mismatch: expected $expected got $actual"
  fi
  echo "[backup-restore-smoke] verified $label: $actual"
}

require_tool sha256sum
require_tool go

if [[ -z "$source_url" || -z "$restore_url" ]]; then
  require_tool docker
  source_container="oblivious-phase22-source-$$"
  restore_container="oblivious-phase22-restore-$$"
  echo "[backup-restore-smoke] starting disposable source database with $postgres_image"
  source_url=$(start_database_container "$source_container")
  echo "[backup-restore-smoke] starting disposable restore database with $postgres_image"
  restore_url=$(start_database_container "$restore_container")
fi

wait_for_database "$source_url" "source"
wait_for_database "$restore_url" "restore"
ensure_empty_database "$restore_url"
apply_migrations
seed_fixture

backup_dir=".tmp/backups/$run_id"
backup_basename="phase22-backup-restore-smoke"
BACKUP_DATABASE_URL="$source_url" BACKUP_DIR="$backup_dir" BACKUP_BASENAME="$backup_basename" PG_CLIENT_IMAGE="$client_image" PG_CLIENT_DOCKER_NETWORK="$client_network" bash "$repo_root/scripts/backup-postgres.sh"
backup_file="$repo_root/$backup_dir/$backup_basename.dump"
RESTORE_DATABASE_URL="$restore_url" BACKUP_FILE="$backup_file" PG_CLIENT_IMAGE="$client_image" PG_CLIENT_DOCKER_NETWORK="$client_network" bash "$repo_root/scripts/restore-postgres.sh"

expected_migrations=$(find "$repo_root/src/server/migrations" -maxdepth 1 -type f -name '*.sql' | wc -l | tr -d ' ')
assert_count "schema_migrations" "SELECT COUNT(*) FROM schema_migrations;" "$expected_migrations"
assert_count "users" "SELECT COUNT(*) FROM users WHERE id LIKE 'phase22_%';" "3"
assert_count "organizations" "SELECT COUNT(*) FROM organizations WHERE id LIKE 'phase22_%';" "2"
assert_count "memberships" "SELECT COUNT(*) FROM organization_memberships WHERE id LIKE 'phase22_%';" "3"
assert_count "quotas" "SELECT COUNT(*) FROM quotas WHERE id LIKE 'phase22_%';" "2"
assert_count "billing sessions" "SELECT COUNT(*) FROM billing_sessions WHERE id = 'phase22_billing_session';" "1"
assert_count "payment intents" "SELECT COUNT(*) FROM payment_intents WHERE id LIKE 'phase22_%';" "3"
assert_count "stripe webhook events" "SELECT COUNT(*) FROM stripe_webhook_events WHERE id = 'phase22_webhook';" "1"
assert_count "billing lifecycle events" "SELECT COUNT(*) FROM billing_lifecycle_events WHERE id = 'phase22_lifecycle';" "1"
assert_count "billing invoices" "SELECT COUNT(*) FROM billing_invoices WHERE id = 'phase22_invoice';" "1"
assert_count "billing refunds" "SELECT COUNT(*) FROM billing_refunds WHERE id = 'phase22_refund';" "1"
assert_count "published agents" "SELECT COUNT(*) FROM published_agents WHERE id = 'phase22_agent';" "1"
assert_count "marketplace orders" "SELECT COUNT(*) FROM marketplace_orders WHERE id = 'phase22_marketplace_order';" "1"
assert_count "marketplace settlements" "SELECT COUNT(*) FROM marketplace_settlements WHERE id = 'phase22_settlement';" "1"
assert_count "marketplace payouts" "SELECT COUNT(*) FROM marketplace_payouts WHERE id = 'phase22_payout';" "1"
assert_count "marketplace governance events" "SELECT COUNT(*) FROM marketplace_governance_events WHERE id = 'phase22_governance';" "1"
assert_count "marketplace abuse reports" "SELECT COUNT(*) FROM marketplace_abuse_reports WHERE id = 'phase22_abuse';" "1"
assert_count "audit logs" "SELECT COUNT(*) FROM audit_logs WHERE id = 'phase22_audit' AND organization_id = 'phase22_org_publisher';" "1"

echo "[backup-restore-smoke] backup/restore smoke ok"
