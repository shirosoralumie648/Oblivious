package marketplace

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPlatformFeeBPS = 2000
)

type SettlementService struct {
	store *SQLStore
}

func NewSettlementService(store *SQLStore) *SettlementService {
	return &SettlementService{store: store}
}

func (s *SettlementService) CreatePaidInstallCheckout(ctx context.Context, input PaidInstallCheckoutRequest) (*MarketplaceOrder, error) {
	if s == nil || s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("create paid install checkout: store is required")
	}
	if input.BuyerOrganizationID == "" || input.BuyerUserID == "" || input.AgentID == "" {
		return nil, fmt.Errorf("create paid install checkout: buyer organization, buyer user, and agent are required")
	}

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create paid install checkout: begin tx: %w", err)
	}
	defer tx.Rollback()

	var agent PublishedAgent
	var versionID sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT a.id, a.organization_id, a.owner_id, a.name, a.visibility, a.status,
		       a.pricing_type, a.pricing_amount,
		       COALESCE(NULLIF($2, ''), (
		       	SELECT av.id FROM agent_versions av
		       	WHERE av.agent_id = a.id AND av.status = 'approved'
		       	ORDER BY av.created_at DESC LIMIT 1
		       ))
		FROM published_agents a
		WHERE a.id = $1
		FOR UPDATE
	`, input.AgentID, input.VersionID).Scan(
		&agent.ID, &agent.OrganizationID, &agent.OwnerID, &agent.Name, &agent.Visibility, &agent.Status,
		&agent.PricingType, &agent.PricingAmount, &versionID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("create paid install checkout: agent not found")
		}
		return nil, fmt.Errorf("create paid install checkout: load agent: %w", err)
	}
	if agent.Status != "approved" || agent.Visibility != "public" {
		return nil, fmt.Errorf("create paid install checkout: only approved public agents can be installed")
	}
	if agent.PricingType == "free" || agent.PricingAmount <= 0 {
		return nil, fmt.Errorf("create paid install checkout: agent is not paid")
	}

	now := time.Now().UTC()
	orderID := uuid.New().String()
	paymentIntentID := uuid.New().String()
	gross := roundAmount(agent.PricingAmount)
	platformFee := roundAmount(gross * float64(defaultPlatformFeeBPS) / 10000)
	publisherNet := roundAmount(gross - platformFee)
	orderVersionID := ""
	if versionID.Valid {
		orderVersionID = versionID.String
	}

	metadata := map[string]string{
		"checkout_kind":             "marketplace_install",
		"marketplace_order_id":      orderID,
		"agent_id":                  agent.ID,
		"version_id":                orderVersionID,
		"publisher_user_id":         agent.OwnerID,
		"publisher_organization_id": agent.OrganizationID,
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("create paid install checkout: marshal metadata: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payment_intents (
			id, provider, organization_id, user_id, package_id, kind, amount,
			currency, status, metadata, created_at, updated_at
		)
		VALUES ($1, 'stripe', $2, $3, NULL, 'marketplace_install', $4, 'usd', 'pending', $5, $6, $6)
	`, paymentIntentID, input.BuyerOrganizationID, input.BuyerUserID, gross, encodedMetadata, now); err != nil {
		return nil, fmt.Errorf("create paid install checkout: insert payment intent: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO marketplace_orders (
			id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
			agent_id, version_id, payment_intent_id, gross_amount, platform_fee_amount,
			publisher_net_amount, refunded_amount, currency, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, 0, 'usd', 'pending_payment', $12, $12)
	`, orderID, input.BuyerOrganizationID, input.BuyerUserID, agent.OrganizationID, agent.OwnerID,
		agent.ID, orderVersionID, paymentIntentID, gross, platformFee, publisherNet, now)
	if err != nil {
		return nil, fmt.Errorf("create paid install checkout: insert order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("create paid install checkout: commit: %w", err)
	}
	return s.loadOrder(ctx, orderID)
}

func (s *SettlementService) ApplyPaidInstallCheckoutCompleted(ctx context.Context, input PaidInstallCheckoutCompleted) (*MarketplaceSettlement, error) {
	if input.EventID == "" || input.PaymentIntentID == "" {
		return nil, fmt.Errorf("apply paid install checkout: event id and payment intent id are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("apply paid install checkout: begin tx: %w", err)
	}
	defer tx.Rollback()

	order, err := s.loadOrderForUpdate(ctx, tx, input.OrderID, input.PaymentIntentID)
	if err != nil {
		return nil, err
	}
	transitionKey := fmt.Sprintf("stripe:%s:marketplace_checkout:%s", input.EventID, order.PaymentIntentID)
	inserted, err := insertMarketplaceLifecycleTransition(ctx, tx, transitionKey, input.EventID, "checkout.session.completed", order.BuyerOrganizationID, order.BuyerUserID, order.PaymentIntentID, "marketplace_order", order.ID, "paid")
	if err != nil {
		return nil, err
	}
	if !inserted {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("apply paid install checkout: commit duplicate: %w", err)
		}
		return s.loadSettlementByOrder(ctx, order.ID)
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE payment_intents
		SET status = 'completed',
		    provider_checkout_session_id = COALESCE(NULLIF($2, ''), provider_checkout_session_id),
		    provider_payment_intent_id = COALESCE(NULLIF($3, ''), provider_payment_intent_id),
		    updated_at = $4
		WHERE id = $1
	`, order.PaymentIntentID, input.ProviderCheckoutSessionID, input.ProviderPaymentIntentID, now); err != nil {
		return nil, fmt.Errorf("apply paid install checkout: complete payment intent: %w", err)
	}

	installID := uuid.New().String()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO agent_installs (id, agent_id, user_id, organization_id, version_id, installed_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		ON CONFLICT (organization_id, agent_id, user_id) DO NOTHING
	`, installID, order.AgentID, order.BuyerUserID, order.BuyerOrganizationID, order.VersionID, now)
	if err != nil {
		return nil, fmt.Errorf("apply paid install checkout: insert install: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("apply paid install checkout: install rows affected: %w", err)
	}
	if rows == 0 {
		if err := tx.QueryRowContext(ctx, `
			SELECT id FROM agent_installs
			WHERE organization_id = $1 AND agent_id = $2 AND user_id = $3
		`, order.BuyerOrganizationID, order.AgentID, order.BuyerUserID).Scan(&installID); err != nil {
			return nil, fmt.Errorf("apply paid install checkout: find existing install: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE published_agents SET install_count = install_count + 1 WHERE id = $1`, order.AgentID); err != nil {
		return nil, fmt.Errorf("apply paid install checkout: update install count: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_orders
		SET status = 'paid',
		    provider_checkout_session_id = COALESCE(NULLIF($2, ''), provider_checkout_session_id),
		    provider_payment_intent_id = COALESCE(NULLIF($3, ''), provider_payment_intent_id),
		    install_id = $4,
		    paid_at = COALESCE(paid_at, $5),
		    updated_at = $5
		WHERE id = $1
	`, order.ID, input.ProviderCheckoutSessionID, input.ProviderPaymentIntentID, installID, now); err != nil {
		return nil, fmt.Errorf("apply paid install checkout: mark order paid: %w", err)
	}

	settlementID := uuid.New().String()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_settlements (
			id, order_id, publisher_organization_id, publisher_user_id, agent_id,
			gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
			status, hold_until, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0, 'pending', $9, $10, $10)
		ON CONFLICT (order_id) DO NOTHING
	`, settlementID, order.ID, order.PublisherOrganizationID, order.PublisherUserID, order.AgentID,
		order.GrossAmount, order.PlatformFeeAmount, order.PublisherNetAmount, now.Add(7*24*time.Hour), now); err != nil {
		return nil, fmt.Errorf("apply paid install checkout: insert settlement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("apply paid install checkout: commit: %w", err)
	}
	return s.loadSettlementByOrder(ctx, order.ID)
}

func (s *SettlementService) ApplyMarketplaceRefund(ctx context.Context, input MarketplaceRefund) error {
	if input.EventID == "" || input.ProviderRefundID == "" || input.Amount <= 0 {
		return fmt.Errorf("apply marketplace refund: event id, refund id, and positive amount are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("apply marketplace refund: begin tx: %w", err)
	}
	defer tx.Rollback()

	order, err := s.loadOrderForRefund(ctx, tx, input.PaymentIntentID, input.ProviderPaymentIntentID)
	if err != nil {
		return err
	}
	transitionKey := fmt.Sprintf("stripe:%s:marketplace_refund:%s", input.EventID, input.ProviderRefundID)
	inserted, err := insertMarketplaceLifecycleTransition(ctx, tx, transitionKey, input.EventID, "refund.created", order.BuyerOrganizationID, order.BuyerUserID, order.PaymentIntentID, "marketplace_refund", input.ProviderRefundID, "succeeded")
	if err != nil {
		return err
	}
	if !inserted {
		return tx.Commit()
	}

	refundedTotal := order.RefundedAmount + input.Amount
	if refundedTotal > order.GrossAmount {
		refundedTotal = order.GrossAmount
	}
	orderStatus := "partially_refunded"
	settlementStatus := "partially_refunded"
	if refundedTotal >= order.GrossAmount {
		orderStatus = "refunded"
		settlementStatus = "reversed"
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE payment_intents
		SET status = $2, refunded_amount = $3, updated_at = $4
		WHERE id = $1
	`, order.PaymentIntentID, orderStatus, refundedTotal, now); err != nil {
		return fmt.Errorf("apply marketplace refund: update payment intent: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_orders
		SET status = $2, refunded_amount = $3, updated_at = $4
		WHERE id = $1
	`, order.ID, orderStatus, refundedTotal, now); err != nil {
		return fmt.Errorf("apply marketplace refund: update order: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_settlements
		SET status = $2, refunded_amount = $3, updated_at = $4
		WHERE order_id = $1
	`, order.ID, settlementStatus, refundedTotal, now); err != nil {
		return fmt.Errorf("apply marketplace refund: update settlement: %w", err)
	}
	return tx.Commit()
}

func (s *SettlementService) MarkSettlementPayoutPending(ctx context.Context, settlementID string, providerPayoutID string) (*MarketplacePayout, error) {
	if settlementID == "" {
		return nil, fmt.Errorf("mark payout pending: settlement id is required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mark payout pending: begin tx: %w", err)
	}
	defer tx.Rollback()

	var settlement MarketplaceSettlement
	if err := tx.QueryRowContext(ctx, `
		SELECT id, order_id, publisher_organization_id, publisher_user_id, agent_id,
		       gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
		       COALESCE(payout_id, ''), status, created_at, updated_at
		FROM marketplace_settlements
		WHERE id = $1
		FOR UPDATE
	`, settlementID).Scan(&settlement.ID, &settlement.OrderID, &settlement.PublisherOrganizationID, &settlement.PublisherUserID,
		&settlement.AgentID, &settlement.GrossAmount, &settlement.PlatformFeeAmount, &settlement.PublisherNetAmount,
		&settlement.RefundedAmount, &settlement.PayoutID, &settlement.Status, &settlement.CreatedAt, &settlement.UpdatedAt); err != nil {
		return nil, fmt.Errorf("mark payout pending: load settlement: %w", err)
	}

	now := time.Now().UTC()
	payoutID := uuid.New().String()
	amount := settlement.PublisherNetAmount - settlement.RefundedAmount
	if amount < 0 {
		amount = 0
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_payouts (
			id, publisher_organization_id, publisher_user_id, amount, currency,
			provider, provider_payout_id, status, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'usd', 'local', NULLIF($5, ''), 'payout_pending', '{}', $6, $6)
	`, payoutID, settlement.PublisherOrganizationID, settlement.PublisherUserID, amount, providerPayoutID, now); err != nil {
		return nil, fmt.Errorf("mark payout pending: insert payout: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_settlements
		SET payout_id = $2, status = 'payout_pending', updated_at = $3
		WHERE id = $1
	`, settlement.ID, payoutID, now); err != nil {
		return nil, fmt.Errorf("mark payout pending: update settlement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("mark payout pending: commit: %w", err)
	}
	return s.loadPayout(ctx, payoutID)
}

func (s *SettlementService) SetPaidInstallCheckoutSession(ctx context.Context, orderID string, paymentIntentID string, providerCheckoutSessionID string) error {
	if orderID == "" || paymentIntentID == "" || providerCheckoutSessionID == "" {
		return fmt.Errorf("set paid install checkout session: order, payment intent, and checkout session are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("set paid install checkout session: begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE marketplace_orders
		SET provider_checkout_session_id = $3, updated_at = $4
		WHERE id = $1 AND payment_intent_id = $2
	`, orderID, paymentIntentID, providerCheckoutSessionID, now)
	if err != nil {
		return fmt.Errorf("set paid install checkout session: update order: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set paid install checkout session: rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE payment_intents
		SET provider_checkout_session_id = $2,
		    metadata = metadata || jsonb_build_object('checkout_session_id', $2::text),
		    updated_at = $3
		WHERE id = $1
	`, paymentIntentID, providerCheckoutSessionID, now); err != nil {
		return fmt.Errorf("set paid install checkout session: update payment intent: %w", err)
	}
	return tx.Commit()
}

func (s *SettlementService) loadOrder(ctx context.Context, orderID string) (*MarketplaceOrder, error) {
	return scanMarketplaceOrder(s.store.db.QueryRowContext(ctx, `
		SELECT id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
		       agent_id, COALESCE(version_id, ''), payment_intent_id,
		       COALESCE(provider_checkout_session_id, ''), COALESCE(provider_payment_intent_id, ''),
		       COALESCE(install_id, ''), gross_amount, platform_fee_amount, publisher_net_amount,
		       refunded_amount, currency, status, created_at, updated_at
		FROM marketplace_orders WHERE id = $1
	`, orderID))
}

func (s *SettlementService) loadSettlementByOrder(ctx context.Context, orderID string) (*MarketplaceSettlement, error) {
	return scanMarketplaceSettlement(s.store.db.QueryRowContext(ctx, `
		SELECT id, order_id, publisher_organization_id, publisher_user_id, agent_id,
		       gross_amount, platform_fee_amount, publisher_net_amount, refunded_amount,
		       COALESCE(payout_id, ''), status, created_at, updated_at
		FROM marketplace_settlements WHERE order_id = $1
	`, orderID))
}

func (s *SettlementService) loadPayout(ctx context.Context, payoutID string) (*MarketplacePayout, error) {
	var payout MarketplacePayout
	if err := s.store.db.QueryRowContext(ctx, `
		SELECT id, publisher_organization_id, publisher_user_id, amount, currency,
		       provider, COALESCE(provider_payout_id, ''), status, created_at, updated_at
		FROM marketplace_payouts WHERE id = $1
	`, payoutID).Scan(&payout.ID, &payout.PublisherOrganizationID, &payout.PublisherUserID, &payout.Amount,
		&payout.Currency, &payout.Provider, &payout.ProviderPayoutID, &payout.Status, &payout.CreatedAt, &payout.UpdatedAt); err != nil {
		return nil, fmt.Errorf("load payout: %w", err)
	}
	return &payout, nil
}

func (s *SettlementService) loadOrderForUpdate(ctx context.Context, tx *sql.Tx, orderID string, paymentIntentID string) (*MarketplaceOrder, error) {
	query := `
		SELECT id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
		       agent_id, COALESCE(version_id, ''), payment_intent_id,
		       COALESCE(provider_checkout_session_id, ''), COALESCE(provider_payment_intent_id, ''),
		       COALESCE(install_id, ''), gross_amount, platform_fee_amount, publisher_net_amount,
		       refunded_amount, currency, status, created_at, updated_at
		FROM marketplace_orders
		WHERE payment_intent_id = $1 AND (NULLIF($2, '') IS NULL OR id = $2)
		FOR UPDATE
	`
	order, err := scanMarketplaceOrder(tx.QueryRowContext(ctx, query, paymentIntentID, orderID))
	if err != nil {
		return nil, fmt.Errorf("load marketplace order: %w", err)
	}
	return order, nil
}

func (s *SettlementService) loadOrderForRefund(ctx context.Context, tx *sql.Tx, paymentIntentID string, providerPaymentIntentID string) (*MarketplaceOrder, error) {
	query := `
		SELECT mo.id, mo.buyer_organization_id, mo.buyer_user_id, mo.publisher_organization_id, mo.publisher_user_id,
		       mo.agent_id, COALESCE(mo.version_id, ''), mo.payment_intent_id,
		       COALESCE(mo.provider_checkout_session_id, ''), COALESCE(mo.provider_payment_intent_id, ''),
		       COALESCE(mo.install_id, ''), mo.gross_amount, mo.platform_fee_amount, mo.publisher_net_amount,
		       mo.refunded_amount, mo.currency, mo.status, mo.created_at, mo.updated_at
		FROM marketplace_orders mo
		JOIN payment_intents pi ON pi.id = mo.payment_intent_id
		WHERE (NULLIF($1, '') IS NOT NULL AND mo.payment_intent_id = $1)
		   OR (NULLIF($2, '') IS NOT NULL AND pi.provider_payment_intent_id = $2)
		LIMIT 1
		FOR UPDATE OF mo
	`
	order, err := scanMarketplaceOrder(tx.QueryRowContext(ctx, query, paymentIntentID, providerPaymentIntentID))
	if err != nil {
		return nil, fmt.Errorf("load marketplace refund order: %w", err)
	}
	return order, nil
}

func scanMarketplaceOrder(row interface{ Scan(...interface{}) error }) (*MarketplaceOrder, error) {
	var order MarketplaceOrder
	if err := row.Scan(&order.ID, &order.BuyerOrganizationID, &order.BuyerUserID,
		&order.PublisherOrganizationID, &order.PublisherUserID, &order.AgentID, &order.VersionID,
		&order.PaymentIntentID, &order.ProviderCheckoutSessionID, &order.ProviderPaymentIntentID,
		&order.InstallID, &order.GrossAmount, &order.PlatformFeeAmount, &order.PublisherNetAmount,
		&order.RefundedAmount, &order.Currency, &order.Status, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return nil, err
	}
	return &order, nil
}

func scanMarketplaceSettlement(row interface{ Scan(...interface{}) error }) (*MarketplaceSettlement, error) {
	var settlement MarketplaceSettlement
	if err := row.Scan(&settlement.ID, &settlement.OrderID, &settlement.PublisherOrganizationID,
		&settlement.PublisherUserID, &settlement.AgentID, &settlement.GrossAmount,
		&settlement.PlatformFeeAmount, &settlement.PublisherNetAmount, &settlement.RefundedAmount,
		&settlement.PayoutID, &settlement.Status, &settlement.CreatedAt, &settlement.UpdatedAt); err != nil {
		return nil, err
	}
	return &settlement, nil
}

func insertMarketplaceLifecycleTransition(ctx context.Context, tx *sql.Tx, transitionKey, eventID, eventType, organizationID, userID, paymentIntentID, entityType, entityID, toState string) (bool, error) {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO billing_lifecycle_events (
			id, transition_key, provider, provider_event_id, event_type,
			organization_id, user_id, payment_intent_id, entity_type, entity_id,
			to_state, reason, payload, created_at
		)
		VALUES ($1, $2, 'stripe', $3, $4, $5, $6, $7, $8, $9, $10, $4, '{}', $11)
		ON CONFLICT (transition_key) DO NOTHING
	`, uuid.New().String(), transitionKey, eventID, eventType, organizationID, userID, paymentIntentID, entityType, entityID, toState, time.Now().UTC())
	if err != nil {
		return false, fmt.Errorf("insert marketplace lifecycle transition: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("marketplace lifecycle transition rows affected: %w", err)
	}
	return rows > 0, nil
}

func roundAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
}
