package marketplace

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
)

func TestSettlementCreatePaidInstallCheckoutCreatesPendingOrderAndIntent(t *testing.T) {
	database := settlementTestDB(t)
	service := NewSettlementService(NewSQLStore(database))

	insertSettlementUserOrg(t, database, "buyer_user", "buyer_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementAgent(t, database, "agent_paid", "publisher_user", "publisher_org", "one_time", 50)

	order, err := service.CreatePaidInstallCheckout(context.Background(), PaidInstallCheckoutRequest{
		BuyerOrganizationID: "buyer_org",
		BuyerUserID:         "buyer_user",
		AgentID:             "agent_paid",
		VersionID:           "version_agent_paid",
	})
	if err != nil {
		t.Fatalf("CreatePaidInstallCheckout returned error: %v", err)
	}

	if order.Status != "pending_payment" || order.GrossAmount != 50 || order.PlatformFeeAmount != 10 || order.PublisherNetAmount != 40 {
		t.Fatalf("unexpected pending order: %#v", order)
	}

	var installCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_paid'`).Scan(&installCount); err != nil {
		t.Fatalf("count installs: %v", err)
	}
	if installCount != 0 {
		t.Fatalf("paid checkout must not create install before webhook, got %d installs", installCount)
	}

	var intentKind, intentStatus string
	var intentAmount float64
	if err := database.QueryRow(`
		SELECT kind, status, amount
		FROM payment_intents
		WHERE id = $1 AND organization_id = 'buyer_org' AND user_id = 'buyer_user'
	`, order.PaymentIntentID).Scan(&intentKind, &intentStatus, &intentAmount); err != nil {
		t.Fatalf("query payment intent: %v", err)
	}
	if intentKind != "marketplace_install" || intentStatus != "pending" || intentAmount != 50 {
		t.Fatalf("expected pending marketplace payment intent, got kind=%s status=%s amount=%.2f", intentKind, intentStatus, intentAmount)
	}
}

func TestSettlementApplyPaidInstallCheckoutCompletedCreatesInstallAndSettlementOnce(t *testing.T) {
	database := settlementTestDB(t)
	service := NewSettlementService(NewSQLStore(database))

	insertSettlementUserOrg(t, database, "buyer_user", "buyer_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementAgent(t, database, "agent_paid", "publisher_user", "publisher_org", "one_time", 50)
	order, err := service.CreatePaidInstallCheckout(context.Background(), PaidInstallCheckoutRequest{
		BuyerOrganizationID: "buyer_org",
		BuyerUserID:         "buyer_user",
		AgentID:             "agent_paid",
		VersionID:           "version_agent_paid",
	})
	if err != nil {
		t.Fatalf("create paid checkout: %v", err)
	}

	completed := PaidInstallCheckoutCompleted{
		EventID:                   "evt_marketplace_checkout",
		OrderID:                   order.ID,
		PaymentIntentID:           order.PaymentIntentID,
		ProviderCheckoutSessionID: "cs_marketplace",
		ProviderPaymentIntentID:   "pi_marketplace",
	}
	for i := 0; i < 2; i++ {
		if _, err := service.ApplyPaidInstallCheckoutCompleted(context.Background(), completed); err != nil {
			t.Fatalf("apply checkout completed attempt %d: %v", i+1, err)
		}
	}

	var orderStatus string
	var installCount, settlementCount int
	if err := database.QueryRow(`SELECT status FROM marketplace_orders WHERE id = $1`, order.ID).Scan(&orderStatus); err != nil {
		t.Fatalf("query order status: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM agent_installs WHERE agent_id = 'agent_paid' AND organization_id = 'buyer_org'`).Scan(&installCount); err != nil {
		t.Fatalf("count installs: %v", err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM marketplace_settlements WHERE order_id = $1`, order.ID).Scan(&settlementCount); err != nil {
		t.Fatalf("count settlements: %v", err)
	}
	if orderStatus != "paid" || installCount != 1 || settlementCount != 1 {
		t.Fatalf("expected paid order with one install and one settlement, got status=%s installs=%d settlements=%d", orderStatus, installCount, settlementCount)
	}
}

func TestSettlementApplyRefundAdjustsOrderAndSettlementOnce(t *testing.T) {
	database := settlementTestDB(t)
	service := NewSettlementService(NewSQLStore(database))

	insertSettlementUserOrg(t, database, "buyer_user", "buyer_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementAgent(t, database, "agent_paid", "publisher_user", "publisher_org", "one_time", 50)
	order, err := service.CreatePaidInstallCheckout(context.Background(), PaidInstallCheckoutRequest{
		BuyerOrganizationID: "buyer_org",
		BuyerUserID:         "buyer_user",
		AgentID:             "agent_paid",
	})
	if err != nil {
		t.Fatalf("create paid checkout: %v", err)
	}
	if _, err := service.ApplyPaidInstallCheckoutCompleted(context.Background(), PaidInstallCheckoutCompleted{
		EventID:                   "evt_marketplace_checkout",
		OrderID:                   order.ID,
		PaymentIntentID:           order.PaymentIntentID,
		ProviderCheckoutSessionID: "cs_marketplace",
		ProviderPaymentIntentID:   "pi_marketplace",
	}); err != nil {
		t.Fatalf("apply checkout completed: %v", err)
	}

	refund := MarketplaceRefund{
		EventID:          "evt_marketplace_refund",
		ProviderRefundID: "re_marketplace",
		PaymentIntentID:  order.PaymentIntentID,
		Amount:           25,
		Currency:         "usd",
		Reason:           "requested_by_customer",
	}
	for i := 0; i < 2; i++ {
		if err := service.ApplyMarketplaceRefund(context.Background(), refund); err != nil {
			t.Fatalf("apply refund attempt %d: %v", i+1, err)
		}
	}

	var orderStatus, settlementStatus string
	var orderRefunded, settlementRefunded float64
	if err := database.QueryRow(`SELECT status, refunded_amount FROM marketplace_orders WHERE id = $1`, order.ID).Scan(&orderStatus, &orderRefunded); err != nil {
		t.Fatalf("query order refund: %v", err)
	}
	if err := database.QueryRow(`SELECT status, refunded_amount FROM marketplace_settlements WHERE order_id = $1`, order.ID).Scan(&settlementStatus, &settlementRefunded); err != nil {
		t.Fatalf("query settlement refund: %v", err)
	}
	if orderStatus != "partially_refunded" || orderRefunded != 25 || settlementStatus != "partially_refunded" || settlementRefunded != 25 {
		t.Fatalf("expected one partial refund, got order=%s %.2f settlement=%s %.2f", orderStatus, orderRefunded, settlementStatus, settlementRefunded)
	}
}

func TestSettlementPayoutStateIsLocalOnly(t *testing.T) {
	database := settlementTestDB(t)
	service := NewSettlementService(NewSQLStore(database))

	insertSettlementUserOrg(t, database, "buyer_user", "buyer_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementAgent(t, database, "agent_paid", "publisher_user", "publisher_org", "one_time", 50)
	order, err := service.CreatePaidInstallCheckout(context.Background(), PaidInstallCheckoutRequest{
		BuyerOrganizationID: "buyer_org",
		BuyerUserID:         "buyer_user",
		AgentID:             "agent_paid",
	})
	if err != nil {
		t.Fatalf("create paid checkout: %v", err)
	}
	settlement, err := service.ApplyPaidInstallCheckoutCompleted(context.Background(), PaidInstallCheckoutCompleted{
		EventID:                   "evt_marketplace_checkout",
		OrderID:                   order.ID,
		PaymentIntentID:           order.PaymentIntentID,
		ProviderCheckoutSessionID: "cs_marketplace",
		ProviderPaymentIntentID:   "pi_marketplace",
	})
	if err != nil {
		t.Fatalf("apply checkout completed: %v", err)
	}

	payout, err := service.MarkSettlementPayoutPending(context.Background(), settlement.ID, "manual-batch-1")
	if err != nil {
		t.Fatalf("MarkSettlementPayoutPending returned error: %v", err)
	}
	if payout.Provider != "local" || payout.ProviderPayoutID != "manual-batch-1" || payout.Status != "payout_pending" {
		t.Fatalf("expected local payout state only, got %#v", payout)
	}
}

func TestSettlementPublisherStatsIncludesSettlementAmounts(t *testing.T) {
	database := settlementTestDB(t)
	settlementService := NewSettlementService(NewSQLStore(database))
	publisherService := NewService(NewSQLStore(database), nil)

	insertSettlementUserOrg(t, database, "buyer_user", "buyer_org")
	insertSettlementUserOrg(t, database, "publisher_user", "publisher_org")
	insertSettlementAgent(t, database, "agent_paid", "publisher_user", "publisher_org", "one_time", 50)
	order, err := settlementService.CreatePaidInstallCheckout(context.Background(), PaidInstallCheckoutRequest{
		BuyerOrganizationID: "buyer_org",
		BuyerUserID:         "buyer_user",
		AgentID:             "agent_paid",
	})
	if err != nil {
		t.Fatalf("create paid checkout: %v", err)
	}
	settlement, err := settlementService.ApplyPaidInstallCheckoutCompleted(context.Background(), PaidInstallCheckoutCompleted{
		EventID:                   "evt_marketplace_checkout",
		OrderID:                   order.ID,
		PaymentIntentID:           order.PaymentIntentID,
		ProviderCheckoutSessionID: "cs_marketplace",
		ProviderPaymentIntentID:   "pi_marketplace",
	})
	if err != nil {
		t.Fatalf("apply checkout completed: %v", err)
	}
	if err := settlementService.ApplyMarketplaceRefund(context.Background(), MarketplaceRefund{
		EventID:          "evt_marketplace_refund",
		ProviderRefundID: "re_marketplace",
		PaymentIntentID:  order.PaymentIntentID,
		Amount:           10,
		Currency:         "usd",
		Reason:           "requested_by_customer",
	}); err != nil {
		t.Fatalf("apply marketplace refund: %v", err)
	}
	if _, err := settlementService.MarkSettlementPayoutPending(context.Background(), settlement.ID, "manual-batch-1"); err != nil {
		t.Fatalf("mark payout pending: %v", err)
	}

	stats, err := publisherService.GetPublisherStats(context.Background(), "publisher_user", "publisher_org")
	if err != nil {
		t.Fatalf("GetPublisherStats returned error: %v", err)
	}
	if stats.GrossRevenue != 50 || stats.PlatformFees != 10 || stats.NetRevenue != 40 || stats.RefundedAmount != 10 || stats.PayoutPendingAmount != 30 {
		t.Fatalf("unexpected financial stats: %#v", stats)
	}
}

func settlementTestDB(t *testing.T) *sql.DB {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE") == "true" {
			t.Fatal("TEST_DATABASE_URL is required when OBLIVIOUS_REQUIRE_TEST_DATABASE=true")
		}
		t.Skip("TEST_DATABASE_URL is required for marketplace settlement integration tests")
	}
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open settlement database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping settlement database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	if _, err := database.Exec(`SELECT pg_advisory_lock(104211)`); err != nil {
		t.Fatalf("lock settlement database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104211)`); err != nil {
			t.Fatalf("unlock settlement database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS marketplace_payouts CASCADE`,
		`DROP TABLE IF EXISTS marketplace_settlements CASCADE`,
		`DROP TABLE IF EXISTS marketplace_orders CASCADE`,
		`DROP TABLE IF EXISTS marketplace_governance_events CASCADE`,
		`DROP TABLE IF EXISTS marketplace_abuse_reports CASCADE`,
		`DROP TABLE IF EXISTS billing_lifecycle_events CASCADE`,
		`DROP TABLE IF EXISTS payment_intents CASCADE`,
		`DROP TABLE IF EXISTS agent_installs CASCADE`,
		`DROP TABLE IF EXISTS agent_versions CASCADE`,
		`DROP TABLE IF EXISTS published_agents CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', name TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE published_agents (id TEXT PRIMARY KEY, owner_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, name TEXT NOT NULL, description TEXT NOT NULL, icon_url TEXT, category_id TEXT, tags TEXT[] NOT NULL DEFAULT '{}', tools JSONB, example_conversations JSONB, system_prompt TEXT, visibility TEXT NOT NULL DEFAULT 'public', status TEXT NOT NULL DEFAULT 'approved', review_reason TEXT, pricing_type TEXT NOT NULL DEFAULT 'free', pricing_amount DECIMAL(10,2) DEFAULT 0, install_count INTEGER NOT NULL DEFAULT 0, rating_avg DECIMAL(3,2) DEFAULT 0, rating_count INTEGER NOT NULL DEFAULT 0, reviewed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_versions (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, version TEXT NOT NULL, changelog TEXT, metadata JSONB, status TEXT NOT NULL DEFAULT 'approved', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(agent_id, version))`,
		`CREATE TABLE agent_installs (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, version_id TEXT REFERENCES agent_versions(id), installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(organization_id, agent_id, user_id))`,
		`CREATE TABLE payment_intents (id TEXT PRIMARY KEY, provider TEXT NOT NULL DEFAULT 'stripe', provider_checkout_session_id TEXT UNIQUE, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, package_id TEXT, kind TEXT NOT NULL, amount DECIMAL(15,6) NOT NULL, currency TEXT NOT NULL DEFAULT 'usd', status TEXT NOT NULL DEFAULT 'pending', metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), provider_payment_intent_id TEXT, provider_subscription_id TEXT, provider_invoice_id TEXT, refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0)`,
		`CREATE TABLE billing_lifecycle_events (id TEXT PRIMARY KEY, transition_key TEXT NOT NULL UNIQUE, provider TEXT NOT NULL DEFAULT 'stripe', provider_event_id TEXT NOT NULL, event_type TEXT NOT NULL, organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, payment_intent_id TEXT REFERENCES payment_intents(id) ON DELETE SET NULL, entity_type TEXT NOT NULL, entity_id TEXT, from_state TEXT, to_state TEXT NOT NULL, reason TEXT, payload JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE marketplace_orders (id TEXT PRIMARY KEY, buyer_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, buyer_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, publisher_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, publisher_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, version_id TEXT REFERENCES agent_versions(id) ON DELETE SET NULL, payment_intent_id TEXT NOT NULL UNIQUE REFERENCES payment_intents(id) ON DELETE CASCADE, provider_checkout_session_id TEXT, provider_payment_intent_id TEXT, install_id TEXT REFERENCES agent_installs(id) ON DELETE SET NULL, gross_amount DECIMAL(15,6) NOT NULL, platform_fee_amount DECIMAL(15,6) NOT NULL, publisher_net_amount DECIMAL(15,6) NOT NULL, refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0, currency TEXT NOT NULL DEFAULT 'usd', status TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), paid_at TIMESTAMPTZ)`,
		`CREATE TABLE marketplace_payouts (id TEXT PRIMARY KEY, publisher_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, publisher_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, amount DECIMAL(15,6) NOT NULL, currency TEXT NOT NULL DEFAULT 'usd', provider TEXT NOT NULL DEFAULT 'local', provider_payout_id TEXT, status TEXT NOT NULL, metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE marketplace_settlements (id TEXT PRIMARY KEY, order_id TEXT NOT NULL UNIQUE REFERENCES marketplace_orders(id) ON DELETE CASCADE, publisher_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, publisher_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, gross_amount DECIMAL(15,6) NOT NULL, platform_fee_amount DECIMAL(15,6) NOT NULL, publisher_net_amount DECIMAL(15,6) NOT NULL, refunded_amount DECIMAL(15,6) NOT NULL DEFAULT 0, payout_id TEXT REFERENCES marketplace_payouts(id) ON DELETE SET NULL, status TEXT NOT NULL, hold_until TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE marketplace_governance_events (id TEXT PRIMARY KEY, actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, actor_organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, action TEXT NOT NULL, from_status TEXT, to_status TEXT, reason TEXT, metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE marketplace_abuse_reports (id TEXT PRIMARY KEY, reporter_organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE, reporter_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, agent_id TEXT NOT NULL REFERENCES published_agents(id) ON DELETE CASCADE, reason TEXT NOT NULL, details TEXT, status TEXT NOT NULL DEFAULT 'open', resolution TEXT, reviewer_user_id TEXT REFERENCES users(id) ON DELETE SET NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), resolved_at TIMESTAMPTZ)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare settlement database: %v", err)
		}
	}
	return database
}

func insertSettlementUserOrg(t *testing.T, database *sql.DB, userID, organizationID string) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO users (id, email, password_hash, role, created_at)
		VALUES ($1, $2, 'hash', 'user', NOW())
	`, userID, userID+"@example.com"); err != nil {
		t.Fatalf("insert settlement user: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO organizations (id, slug, name, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', '{}', NOW(), NOW())
	`, organizationID, organizationID, organizationID); err != nil {
		t.Fatalf("insert settlement organization: %v", err)
	}
}

func insertSettlementAgent(t *testing.T, database *sql.DB, agentID, ownerID, organizationID, pricingType string, pricingAmount float64) {
	t.Helper()
	if _, err := database.Exec(`
		INSERT INTO published_agents (
			id, owner_id, organization_id, name, description, tools, example_conversations,
			visibility, status, pricing_type, pricing_amount, install_count, rating_avg, rating_count, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'Paid Agent', 'A paid marketplace agent.', '{"tools":[{"name":"paid"}]}'::jsonb,
		        '[]'::jsonb, 'public', 'approved', $4, $5, 0, 0, 0, NOW(), NOW())
	`, agentID, ownerID, organizationID, pricingType, pricingAmount); err != nil {
		t.Fatalf("insert settlement agent: %v", err)
	}
	if _, err := database.Exec(`
		INSERT INTO agent_versions (id, agent_id, organization_id, version, changelog, metadata, status, created_at)
		VALUES ($1, $2, $3, '1.0.0', 'initial', '{}', 'approved', NOW())
	`, "version_"+agentID, agentID, organizationID); err != nil {
		t.Fatalf("insert settlement agent version: %v", err)
	}
}
