package marketplace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
)

const (
	defaultPlatformFeeBPS = 2000
)

// ErrMarketplacePayoutProviderRequired is returned when a payable marketplace
// settlement would need an external payout dispatch but no payout provider is
// configured.
var ErrMarketplacePayoutProviderRequired = errors.New("marketplace payout provider is required")

// MarketplaceFeeTierBasis selects the sales interval used for segmented platform fees.
type MarketplaceFeeTierBasis string

const (
	MarketplaceFeeTierBasisCurrentOrderAmount       MarketplaceFeeTierBasis = "current_order_amount"
	MarketplaceFeeTierBasisOrderAmount              MarketplaceFeeTierBasis = "order_amount"
	MarketplaceFeeTierBasisPublisherCumulativeSales MarketplaceFeeTierBasis = "publisher_cumulative_sales"
)

// MarketplacePlatformFeeTier configures platform fee basis points for sales at or above MinimumAmount.
type MarketplacePlatformFeeTier struct {
	MinimumAmount float64
	FeeBPS        int
}

type MarketplacePayoutDispatchRequest struct {
	PayoutID                string
	PublisherOrganizationID string
	PublisherUserID         string
	Amount                  float64
	Currency                string
	SettlementIDs           []string
}

type MarketplacePayoutDispatchResult struct {
	ProviderPayoutID string
}

type ProviderPayoutLifecycleEvent struct {
	Provider         string
	EventID          string
	EventType        string
	PayoutID         string
	ProviderPayoutID string
	Status           string
	Reason           string
}

type MarketplacePayoutProvider interface {
	Name() string
	CreatePayout(ctx context.Context, request MarketplacePayoutDispatchRequest) (MarketplacePayoutDispatchResult, error)
}

type SettlementServiceOption func(*SettlementService)

type SettlementService struct {
	store                   *SQLStore
	platformFeeTierBasis    MarketplaceFeeTierBasis
	platformFeeTiers        []MarketplacePlatformFeeTier
	minimumSettlementAmount float64
	minimumSettlementCycle  time.Duration
	payoutProvider          MarketplacePayoutProvider
}

func NewSettlementService(store *SQLStore, opts ...SettlementServiceOption) *SettlementService {
	service := &SettlementService{
		store:                store,
		platformFeeTierBasis: MarketplaceFeeTierBasisCurrentOrderAmount,
		platformFeeTiers: []MarketplacePlatformFeeTier{
			{MinimumAmount: 0, FeeBPS: defaultPlatformFeeBPS},
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	if len(service.platformFeeTiers) == 0 {
		service.platformFeeTiers = []MarketplacePlatformFeeTier{{MinimumAmount: 0, FeeBPS: defaultPlatformFeeBPS}}
	}
	return service
}

func WithMarketplacePlatformFeeTiers(basis MarketplaceFeeTierBasis, tiers []MarketplacePlatformFeeTier) SettlementServiceOption {
	return func(service *SettlementService) {
		service.platformFeeTierBasis = basis
		service.platformFeeTiers = normalizePlatformFeeTiers(tiers)
	}
}

func WithMarketplaceMinimumSettlement(amount float64, cycle time.Duration) SettlementServiceOption {
	return func(service *SettlementService) {
		service.minimumSettlementAmount = roundAmount(amount)
		service.minimumSettlementCycle = cycle
	}
}

func WithMarketplacePayoutProvider(provider MarketplacePayoutProvider) SettlementServiceOption {
	return func(service *SettlementService) {
		service.payoutProvider = provider
	}
}

type marketplaceOrderAmounts struct {
	GrossAmount        float64
	PlatformFeeAmount  float64
	PublisherNetAmount float64
}

func (s *SettlementService) CreatePaidInstallCheckout(ctx context.Context, input PaidInstallCheckoutRequest) (*MarketplaceOrder, error) {
	ctx, span := observability.StartSpan(ctx, "marketplace.paid_install_checkout")
	defer span.End()

	if s == nil || s.store == nil || s.store.db == nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: store is required")
	}
	if input.BuyerOrganizationID == "" || input.BuyerUserID == "" || input.AgentID == "" {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: buyer organization, buyer user, and agent are required")
	}

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
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
			metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
			return nil, fmt.Errorf("create paid install checkout: agent not found")
		}
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: load agent: %w", err)
	}
	if agent.Status != "approved" || agent.Visibility != "public" {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: only approved public agents can be installed")
	}
	if agent.PricingType == "free" || agent.PricingAmount <= 0 {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: agent is not paid")
	}

	now := time.Now().UTC()
	orderID := uuid.New().String()
	paymentIntentID := uuid.New().String()
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	if provider == "" {
		provider = "stripe"
	}
	currency := normalizeMarketplaceCurrency(input.Currency)
	publisherCumulativeSales := 0.0
	if s.platformFeeTierBasis == MarketplaceFeeTierBasisPublisherCumulativeSales {
		publisherCumulativeSales, err = s.publisherCumulativeSales(ctx, tx, agent.OrganizationID, agent.OwnerID)
		if err != nil {
			metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
			return nil, fmt.Errorf("create paid install checkout: calculate publisher cumulative sales: %w", err)
		}
	}
	amounts, err := s.calculateOrderAmounts(agent.PricingAmount, publisherCumulativeSales)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: calculate order amounts: %w", err)
	}
	gross := amounts.GrossAmount
	platformFee := amounts.PlatformFeeAmount
	publisherNet := amounts.PublisherNetAmount
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
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: marshal metadata: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
			INSERT INTO payment_intents (
				id, provider, organization_id, user_id, package_id, kind, amount,
				currency, status, metadata, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, NULL, 'marketplace_install', $5, $6, 'pending', $7, $8, $8)
		`, paymentIntentID, provider, input.BuyerOrganizationID, input.BuyerUserID, gross, currency, encodedMetadata, now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: insert payment intent: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO marketplace_orders (
			id, buyer_organization_id, buyer_user_id, publisher_organization_id, publisher_user_id,
				agent_id, version_id, payment_intent_id, gross_amount, platform_fee_amount,
				publisher_net_amount, refunded_amount, currency, status, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9, $10, $11, 0, $12, 'pending_payment', $13, $13)
		`, orderID, input.BuyerOrganizationID, input.BuyerUserID, agent.OrganizationID, agent.OwnerID,
		agent.ID, orderVersionID, paymentIntentID, gross, platformFee, publisherNet, currency, now)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: insert order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "failed")
		return nil, fmt.Errorf("create paid install checkout: commit: %w", err)
	}
	metrics.RecordMarketplaceSettlementEvent("paid_install_checkout", "pending_payment")
	return s.loadOrder(ctx, orderID)
}

func normalizeMarketplaceCurrency(currency string) string {
	currency = strings.ToLower(strings.TrimSpace(currency))
	if currency == "" {
		return "usd"
	}
	return currency
}

func (s *SettlementService) ApplyPaidInstallCheckoutCompleted(ctx context.Context, input PaidInstallCheckoutCompleted) (*MarketplaceSettlement, error) {
	ctx, span := observability.StartSpan(ctx, "marketplace.paid_install_completed")
	defer span.End()

	if input.EventID == "" || input.PaymentIntentID == "" {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: event id and payment intent id are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: begin tx: %w", err)
	}
	defer tx.Rollback()

	order, err := s.loadOrderForUpdate(ctx, tx, input.OrderID, input.PaymentIntentID)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, err
	}
	transitionKey := marketplaceLifecycleTransitionKey(order.Provider, input.EventID, "marketplace_checkout", order.PaymentIntentID)
	inserted, err := insertMarketplaceLifecycleTransition(ctx, tx, order.Provider, transitionKey, input.EventID, "checkout.session.completed", order.BuyerOrganizationID, order.BuyerUserID, order.PaymentIntentID, "marketplace_order", order.ID, "paid")
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, err
	}
	if !inserted {
		if err := tx.Commit(); err != nil {
			metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
			return nil, fmt.Errorf("apply paid install checkout: commit duplicate: %w", err)
		}
		metrics.RecordMarketplaceSettlementEvent("paid_install", "duplicate")
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
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: complete payment intent: %w", err)
	}

	installID := uuid.New().String()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO agent_installs (id, agent_id, user_id, organization_id, version_id, installed_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
		ON CONFLICT (organization_id, agent_id, user_id) DO NOTHING
	`, installID, order.AgentID, order.BuyerUserID, order.BuyerOrganizationID, order.VersionID, now)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: insert install: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: install rows affected: %w", err)
	}
	if rows == 0 {
		if err := tx.QueryRowContext(ctx, `
			SELECT id FROM agent_installs
			WHERE organization_id = $1 AND agent_id = $2 AND user_id = $3
		`, order.BuyerOrganizationID, order.AgentID, order.BuyerUserID).Scan(&installID); err != nil {
			metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
			return nil, fmt.Errorf("apply paid install checkout: find existing install: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE published_agents SET install_count = install_count + 1 WHERE id = $1`, order.AgentID); err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
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
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
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
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: insert settlement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("paid_install", "failed")
		return nil, fmt.Errorf("apply paid install checkout: commit: %w", err)
	}
	metrics.RecordMarketplaceSettlementEvent("paid_install", "paid")
	return s.loadSettlementByOrder(ctx, order.ID)
}

func (s *SettlementService) ApplyMarketplaceRefund(ctx context.Context, input MarketplaceRefund) error {
	ctx, span := observability.StartSpan(ctx, "marketplace.refund")
	defer span.End()

	if input.EventID == "" || input.ProviderRefundID == "" || input.Amount <= 0 {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return fmt.Errorf("apply marketplace refund: event id, refund id, and positive amount are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return fmt.Errorf("apply marketplace refund: begin tx: %w", err)
	}
	defer tx.Rollback()

	order, err := s.loadOrderForRefund(ctx, tx, input.PaymentIntentID, input.ProviderPaymentIntentID)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return err
	}
	transitionKey := marketplaceLifecycleTransitionKey(order.Provider, input.EventID, "marketplace_refund", input.ProviderRefundID)
	inserted, err := insertMarketplaceLifecycleTransition(ctx, tx, order.Provider, transitionKey, input.EventID, "refund.created", order.BuyerOrganizationID, order.BuyerUserID, order.PaymentIntentID, "marketplace_refund", input.ProviderRefundID, "succeeded")
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return err
	}
	if !inserted {
		metrics.RecordMarketplaceSettlementEvent("refund", "duplicate")
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
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return fmt.Errorf("apply marketplace refund: update payment intent: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_orders
		SET status = $2, refunded_amount = $3, updated_at = $4
		WHERE id = $1
	`, order.ID, orderStatus, refundedTotal, now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return fmt.Errorf("apply marketplace refund: update order: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_settlements
		SET status = $2, refunded_amount = $3, updated_at = $4
		WHERE order_id = $1
	`, order.ID, settlementStatus, refundedTotal, now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return fmt.Errorf("apply marketplace refund: update settlement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("refund", "failed")
		return err
	}
	metrics.RecordMarketplaceSettlementEvent("refund", settlementStatus)
	return nil
}

func (s *SettlementService) MarkSettlementPayoutPending(ctx context.Context, settlementID string, providerPayoutID string) (*MarketplacePayout, error) {
	ctx, span := observability.StartSpan(ctx, "marketplace.payout_pending")
	defer span.End()

	if settlementID == "" {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout pending: settlement id is required")
	}
	providerName, err := s.marketplacePayoutProviderName()
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout pending: %w", err)
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
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
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout pending: load settlement: %w", err)
	}

	now := time.Now().UTC()
	payoutID := uuid.New().String()
	amount, err := s.calculatePayoutAmount(settlement, now)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, err
	}
	providerPayoutID = strings.TrimSpace(providerPayoutID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO marketplace_payouts (
			id, publisher_organization_id, publisher_user_id, amount, currency,
			provider, provider_payout_id, status, metadata, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, 'usd', $5, NULL, 'payout_pending',
			jsonb_build_object(
				'dispatch_status', 'queued',
				'dispatch_queued_at', $7::timestamptz,
				'operator_provider_payout_id', NULLIF($6, '')
			),
			$7, $7
		)
	`, payoutID, settlement.PublisherOrganizationID, settlement.PublisherUserID, amount, providerName, providerPayoutID, now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout pending: insert payout: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_settlements
		SET payout_id = $2, status = 'payout_pending', updated_at = $3
		WHERE id = $1
	`, settlement.ID, payoutID, now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout pending: update settlement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout pending: commit: %w", err)
	}
	metrics.RecordMarketplaceSettlementEvent("payout", "payout_pending")
	payout, err := s.dispatchPersistedPayout(ctx, payoutID)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, err
	}
	return payout, nil
}

func (s *SettlementService) MarkPayoutPaid(ctx context.Context, payoutID string, providerPayoutID string) (*MarketplacePayout, error) {
	ctx, span := observability.StartSpan(ctx, "marketplace.payout_paid")
	defer span.End()

	if payoutID == "" {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: payout id is required")
	}
	if strings.TrimSpace(providerPayoutID) == "" {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: provider payout id is required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: begin tx: %w", err)
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM marketplace_payouts
		WHERE id = $1
		FOR UPDATE
	`, payoutID).Scan(&status); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: load payout: %w", err)
	}
	if status != "payout_pending" && status != "paid_out" {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: payout %s cannot transition from %s", payoutID, status)
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_payouts
		SET status = 'paid_out',
		    provider_payout_id = $2,
		    updated_at = $3
		WHERE id = $1
	`, payoutID, strings.TrimSpace(providerPayoutID), now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: update payout: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE marketplace_settlements
		SET status = 'paid_out',
		    updated_at = $2
		WHERE payout_id = $1
		  AND status IN ('payout_pending', 'paid_out')
	`, payoutID, now); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: update settlements: %w", err)
	}
	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout", "failed")
		return nil, fmt.Errorf("mark payout paid: commit: %w", err)
	}
	metrics.RecordMarketplaceSettlementEvent("payout", "paid_out")
	return s.loadPayout(ctx, payoutID)
}

func (s *SettlementService) MarkPayoutFailed(ctx context.Context, payoutID string, providerPayoutID string, reason string) (*MarketplacePayout, error) {
	payoutID = strings.TrimSpace(payoutID)
	providerPayoutID = strings.TrimSpace(providerPayoutID)
	reason = strings.TrimSpace(reason)
	if payoutID == "" {
		return nil, fmt.Errorf("mark payout failed: payout id is required")
	}
	if providerPayoutID == "" {
		return nil, fmt.Errorf("mark payout failed: provider payout id is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("mark payout failed: reason is required")
	}
	return s.ApplyProviderPayoutLifecycle(ctx, ProviderPayoutLifecycleEvent{
		EventID:          "admin:payout.failed:" + payoutID + ":" + providerPayoutID,
		EventType:        "payout.failed",
		PayoutID:         payoutID,
		ProviderPayoutID: providerPayoutID,
		Status:           "failed",
		Reason:           reason,
	})
}

func (s *SettlementService) ApplyProviderPayoutLifecycle(ctx context.Context, input ProviderPayoutLifecycleEvent) (*MarketplacePayout, error) {
	ctx, span := observability.StartSpan(ctx, "marketplace.provider_payout_lifecycle")
	defer span.End()

	eventType := strings.TrimSpace(input.EventType)
	provider := strings.ToLower(strings.TrimSpace(input.Provider))
	eventID := strings.TrimSpace(input.EventID)
	payoutID := strings.TrimSpace(input.PayoutID)
	providerPayoutID := strings.TrimSpace(input.ProviderPayoutID)
	reason := strings.TrimSpace(input.Reason)

	if eventID == "" || eventType == "" {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: event id and type are required")
	}
	if payoutID == "" && providerPayoutID == "" {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: payout id or provider payout id is required")
	}

	toState := ""
	switch eventType {
	case "payout.paid":
		toState = "paid_out"
	case "payout.failed":
		toState = "failed"
	default:
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: unsupported event type %s", eventType)
	}

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: begin tx: %w", err)
	}
	defer tx.Rollback()

	payout, err := loadPayoutForProviderLifecycle(ctx, tx, payoutID, providerPayoutID, provider)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, err
	}
	if provider == "" {
		provider = payout.Provider
	}
	if provider != "" && payout.Provider != "" && payout.Provider != provider {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: payout %s belongs to provider %s, got %s", payout.ID, payout.Provider, provider)
	}
	if providerPayoutID == "" {
		providerPayoutID = payout.ProviderPayoutID
	} else if payout.ProviderPayoutID != "" && payout.ProviderPayoutID != providerPayoutID {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: payout %s belongs to provider payout %s, got %s", payout.ID, payout.ProviderPayoutID, providerPayoutID)
	}

	transitionKey := marketplaceLifecycleTransitionKey(provider, eventID, "marketplace_payout", payout.ID)
	inserted, err := insertMarketplaceLifecycleTransition(ctx, tx, provider, transitionKey, eventID, eventType, payout.PublisherOrganizationID, payout.PublisherUserID, "", "marketplace_payout", payout.ID, toState)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, err
	}
	if !inserted {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "duplicate")
		if err := tx.Commit(); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: commit duplicate: %w", err)
		}
		return s.loadPayout(ctx, payout.ID)
	}

	now := time.Now().UTC()
	switch toState {
	case "paid_out":
		if payout.Status != "payout_pending" && payout.Status != "paid_out" {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: payout %s cannot transition from %s to paid_out", payout.ID, payout.Status)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE marketplace_payouts
			SET status = 'paid_out',
			    provider_payout_id = COALESCE(NULLIF($2, ''), provider_payout_id),
			    metadata = metadata || jsonb_build_object(
			        'provider_event_id', $3::text,
			        'provider_event_type', $4::text,
			        'provider_status', $5::text,
			        'provider_reason', NULLIF($6, ''),
			        'provider_processed_at', $7::timestamptz
			    ),
			    updated_at = $7
			WHERE id = $1
		`, payout.ID, providerPayoutID, eventID, eventType, strings.TrimSpace(input.Status), reason, now); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: update paid payout: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE marketplace_settlements
			SET status = 'paid_out',
			    updated_at = $2
			WHERE payout_id = $1
			  AND status IN ('payout_pending', 'paid_out')
		`, payout.ID, now); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: update paid settlements: %w", err)
		}
	case "failed":
		if payout.Status != "payout_pending" && payout.Status != "failed" {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: payout %s cannot transition from %s to failed", payout.ID, payout.Status)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE marketplace_payouts
			SET status = 'failed',
			    provider_payout_id = COALESCE(NULLIF($2, ''), provider_payout_id),
			    metadata = metadata || jsonb_build_object(
			        'provider_event_id', $3::text,
			        'provider_event_type', $4::text,
			        'provider_status', $5::text,
			        'provider_reason', NULLIF($6, ''),
			        'provider_processed_at', $7::timestamptz
			    ),
			    updated_at = $7
			WHERE id = $1
		`, payout.ID, providerPayoutID, eventID, eventType, strings.TrimSpace(input.Status), reason, now); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: update failed payout: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE marketplace_settlements
			SET payout_id = NULL,
			    status = 'available',
			    updated_at = $2
			WHERE payout_id = $1
			  AND status = 'payout_pending'
		`, payout.ID, now); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
			return nil, fmt.Errorf("apply provider payout lifecycle: release failed settlements: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_provider", "failed")
		return nil, fmt.Errorf("apply provider payout lifecycle: commit: %w", err)
	}
	metrics.RecordMarketplaceSettlementEvent("payout_provider", toState)
	return s.loadPayout(ctx, payout.ID)
}

func (s *SettlementService) CreateDuePayouts(ctx context.Context, now time.Time) ([]*MarketplacePayout, error) {
	ctx, span := observability.StartSpan(ctx, "marketplace.create_due_payouts")
	defer span.End()

	if s == nil || s.store == nil || s.store.db == nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: store is required")
	}
	providerName, err := s.marketplacePayoutProviderName()
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: %w", err)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	queuedPayoutIDs, err := s.listQueuedPayoutDispatches(ctx, providerName, 100)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, err
	}
	dispatchedQueuedPayouts, err := s.dispatchPersistedPayouts(ctx, queuedPayoutIDs)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, err
	}

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: begin tx: %w", err)
	}
	defer tx.Rollback()

	type payoutGroup struct {
		PublisherOrganizationID string
		PublisherUserID         string
		Currency                string
		Amount                  float64
		SettlementIDs           []string
		OldestSettlementAt      time.Time
	}
	type payoutGroupKey struct {
		PublisherOrganizationID string
		PublisherUserID         string
		Currency                string
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT ms.id, ms.publisher_organization_id, ms.publisher_user_id, mo.currency,
		       GREATEST(ms.publisher_net_amount - ms.refunded_amount, 0) AS amount,
		       ms.created_at
		FROM marketplace_settlements ms
		JOIN marketplace_orders mo ON mo.id = ms.order_id
		WHERE ms.status = 'available'
		  AND ms.payout_id IS NULL
		  AND (ms.hold_until IS NULL OR ms.hold_until <= $1)
		ORDER BY ms.publisher_organization_id, ms.publisher_user_id, mo.currency, ms.created_at, ms.id
		FOR UPDATE OF ms
	`, now)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: query due settlements: %w", err)
	}
	defer rows.Close()

	groupByKey := make(map[payoutGroupKey]*payoutGroup)
	var groupOrder []payoutGroupKey
	for rows.Next() {
		var settlementID, publisherOrganizationID, publisherUserID, currency string
		var amount float64
		var createdAt time.Time
		if err := rows.Scan(&settlementID, &publisherOrganizationID, &publisherUserID, &currency, &amount, &createdAt); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
			return nil, fmt.Errorf("create due payouts: scan due settlement: %w", err)
		}
		key := payoutGroupKey{
			PublisherOrganizationID: publisherOrganizationID,
			PublisherUserID:         publisherUserID,
			Currency:                currency,
		}
		group := groupByKey[key]
		if group == nil {
			group = &payoutGroup{
				PublisherOrganizationID: publisherOrganizationID,
				PublisherUserID:         publisherUserID,
				Currency:                key.Currency,
				OldestSettlementAt:      createdAt,
			}
			groupByKey[key] = group
			groupOrder = append(groupOrder, key)
		}
		group.Amount = roundAmount(group.Amount + amount)
		group.SettlementIDs = append(group.SettlementIDs, settlementID)
		if createdAt.Before(group.OldestSettlementAt) {
			group.OldestSettlementAt = createdAt
		}
	}
	if err := rows.Err(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: read due settlement groups: %w", err)
	}
	if err := rows.Close(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: close due settlement rows: %w", err)
	}

	payoutIDs := make([]string, 0, len(groupOrder))
	for _, key := range groupOrder {
		group := groupByKey[key]
		if group.Amount <= 0 || len(group.SettlementIDs) == 0 {
			continue
		}
		if s.minimumSettlementAmount > 0 && group.Amount < s.minimumSettlementAmount {
			if s.minimumSettlementCycle <= 0 || group.OldestSettlementAt.IsZero() || now.Before(group.OldestSettlementAt.Add(s.minimumSettlementCycle)) {
				continue
			}
		}

		payoutID := uuid.New().String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO marketplace_payouts (
				id, publisher_organization_id, publisher_user_id, amount, currency,
				provider, provider_payout_id, status, metadata, created_at, updated_at
			)
			VALUES (
				$1, $2, $3, $4, $5, $6, NULL, 'payout_pending',
				jsonb_build_object('dispatch_status', 'queued', 'dispatch_queued_at', $7::timestamptz),
				$7, $7
			)
		`, payoutID, group.PublisherOrganizationID, group.PublisherUserID, group.Amount, group.Currency, providerName, now); err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
			return nil, fmt.Errorf("create due payouts: insert payout: %w", err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE marketplace_settlements
			SET payout_id = $2, status = 'payout_pending', updated_at = $3
			WHERE id = ANY($1)
			  AND status = 'available'
			  AND payout_id IS NULL
		`, pq.Array(group.SettlementIDs), payoutID, now)
		if err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
			return nil, fmt.Errorf("create due payouts: update settlements: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
			return nil, fmt.Errorf("create due payouts: settlement rows affected: %w", err)
		}
		if rowsAffected == 0 {
			continue
		}
		payoutIDs = append(payoutIDs, payoutID)
	}
	if err := tx.Commit(); err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, fmt.Errorf("create due payouts: commit: %w", err)
	}

	dispatchedNewPayouts, err := s.dispatchPersistedPayouts(ctx, payoutIDs)
	if err != nil {
		metrics.RecordMarketplaceSettlementEvent("payout_batch", "failed")
		return nil, err
	}
	payouts := append(dispatchedQueuedPayouts, dispatchedNewPayouts...)
	metrics.RecordMarketplaceSettlementEvent("payout_batch", "payout_pending")
	return payouts, nil
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

func (s *SettlementService) MarkPaidInstallCheckoutFailed(ctx context.Context, orderID string, paymentIntentID string, reason string) error {
	if orderID == "" || paymentIntentID == "" {
		return fmt.Errorf("mark paid install checkout failed: order and payment intent are required")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mark paid install checkout failed: begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE marketplace_orders
		SET status = 'failed', updated_at = $3
		WHERE id = $1 AND payment_intent_id = $2 AND status = 'pending_payment'
	`, orderID, paymentIntentID, now)
	if err != nil {
		return fmt.Errorf("mark paid install checkout failed: update order: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark paid install checkout failed: order rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	metadata := map[string]string{}
	if reason != "" {
		metadata["checkout_failure_reason"] = reason
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("mark paid install checkout failed: marshal metadata: %w", err)
	}
	result, err = tx.ExecContext(ctx, `
		UPDATE payment_intents
		SET status = 'failed',
		    metadata = metadata || $2::jsonb,
		    updated_at = $3
		WHERE id = $1 AND status = 'pending'
	`, paymentIntentID, string(encodedMetadata), now)
	if err != nil {
		return fmt.Errorf("mark paid install checkout failed: update payment intent: %w", err)
	}
	rows, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark paid install checkout failed: payment intent rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *SettlementService) marketplacePayoutProviderName() (string, error) {
	if s == nil || s.payoutProvider == nil {
		return "", ErrMarketplacePayoutProviderRequired
	}
	providerName := strings.ToLower(strings.TrimSpace(s.payoutProvider.Name()))
	if providerName == "" {
		return "", fmt.Errorf("marketplace payout provider name is required")
	}
	return providerName, nil
}

func (s *SettlementService) listQueuedPayoutDispatches(ctx context.Context, providerName string, limit int) ([]string, error) {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName == "" {
		return nil, fmt.Errorf("list queued marketplace payout dispatches: provider name is required")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.store.db.QueryContext(ctx, `
		SELECT id
		FROM marketplace_payouts
		WHERE provider = $1
		  AND status = 'payout_pending'
		  AND COALESCE(provider_payout_id, '') = ''
		ORDER BY created_at ASC, id ASC
		LIMIT $2
	`, providerName, limit)
	if err != nil {
		return nil, fmt.Errorf("list queued marketplace payout dispatches: %w", err)
	}
	defer rows.Close()
	payoutIDs := []string{}
	for rows.Next() {
		var payoutID string
		if err := rows.Scan(&payoutID); err != nil {
			return nil, fmt.Errorf("list queued marketplace payout dispatches: scan: %w", err)
		}
		payoutIDs = append(payoutIDs, payoutID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list queued marketplace payout dispatches: read: %w", err)
	}
	return payoutIDs, nil
}

func (s *SettlementService) dispatchPersistedPayouts(ctx context.Context, payoutIDs []string) ([]*MarketplacePayout, error) {
	payouts := make([]*MarketplacePayout, 0, len(payoutIDs))
	for _, payoutID := range payoutIDs {
		payout, err := s.dispatchPersistedPayout(ctx, payoutID)
		if err != nil {
			return nil, err
		}
		payouts = append(payouts, payout)
	}
	return payouts, nil
}

func (s *SettlementService) dispatchPersistedPayout(ctx context.Context, payoutID string) (*MarketplacePayout, error) {
	payout, settlementIDs, err := s.loadPayoutDispatchIntent(ctx, payoutID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(payout.ProviderPayoutID) != "" {
		return payout, nil
	}
	providerName, dispatchResult, err := s.dispatchPayout(ctx, MarketplacePayoutDispatchRequest{
		PayoutID:                payout.ID,
		PublisherOrganizationID: payout.PublisherOrganizationID,
		PublisherUserID:         payout.PublisherUserID,
		Amount:                  payout.Amount,
		Currency:                payout.Currency,
		SettlementIDs:           settlementIDs,
	})
	if err != nil {
		if recordErr := s.recordPayoutDispatchFailure(ctx, payout.ID, err); recordErr != nil {
			return nil, recordErr
		}
		return nil, err
	}
	if err := s.recordPayoutDispatchSuccess(ctx, payout.ID, providerName, dispatchResult.ProviderPayoutID); err != nil {
		return nil, err
	}
	return s.loadPayout(ctx, payout.ID)
}

func (s *SettlementService) loadPayoutDispatchIntent(ctx context.Context, payoutID string) (*MarketplacePayout, []string, error) {
	payout, err := s.loadPayout(ctx, strings.TrimSpace(payoutID))
	if err != nil {
		return nil, nil, fmt.Errorf("load marketplace payout dispatch intent: %w", err)
	}
	rows, err := s.store.db.QueryContext(ctx, `
		SELECT id
		FROM marketplace_settlements
		WHERE payout_id = $1
		ORDER BY created_at ASC, id ASC
	`, payout.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("load marketplace payout dispatch intent: query settlements: %w", err)
	}
	defer rows.Close()
	settlementIDs := []string{}
	for rows.Next() {
		var settlementID string
		if err := rows.Scan(&settlementID); err != nil {
			return nil, nil, fmt.Errorf("load marketplace payout dispatch intent: scan settlement: %w", err)
		}
		settlementIDs = append(settlementIDs, settlementID)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("load marketplace payout dispatch intent: read settlements: %w", err)
	}
	return payout, settlementIDs, nil
}

func (s *SettlementService) recordPayoutDispatchSuccess(ctx context.Context, payoutID, providerName, providerPayoutID string) error {
	now := time.Now().UTC()
	result, err := s.store.db.ExecContext(ctx, `
		UPDATE marketplace_payouts
		SET provider = $2,
		    provider_payout_id = NULLIF($3, ''),
		    metadata = metadata || jsonb_build_object(
		        'dispatch_status', 'dispatched',
		        'provider_dispatched_at', $4::timestamptz
		    ),
		    updated_at = $4
		WHERE id = $1
		  AND status IN ('payout_pending', 'paid_out')
	`, payoutID, providerName, providerPayoutID, now)
	if err != nil {
		return fmt.Errorf("record marketplace payout dispatch success: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record marketplace payout dispatch success: rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SettlementService) recordPayoutDispatchFailure(ctx context.Context, payoutID string, dispatchErr error) error {
	reason := ""
	if dispatchErr != nil {
		reason = strings.TrimSpace(dispatchErr.Error())
	}
	now := time.Now().UTC()
	result, err := s.store.db.ExecContext(ctx, `
		UPDATE marketplace_payouts
		SET metadata = metadata || jsonb_build_object(
		        'dispatch_status', 'failed',
		        'dispatch_error', NULLIF($2, ''),
		        'provider_dispatch_failed_at', $3::timestamptz
		    ),
		    updated_at = $3
		WHERE id = $1
		  AND status = 'payout_pending'
	`, payoutID, reason, now)
	if err != nil {
		return fmt.Errorf("record marketplace payout dispatch failure: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("record marketplace payout dispatch failure: rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *SettlementService) dispatchPayout(ctx context.Context, request MarketplacePayoutDispatchRequest) (string, MarketplacePayoutDispatchResult, error) {
	if s == nil || s.payoutProvider == nil {
		return "", MarketplacePayoutDispatchResult{}, fmt.Errorf("dispatch marketplace payout: %w", ErrMarketplacePayoutProviderRequired)
	}
	providerName := strings.ToLower(strings.TrimSpace(s.payoutProvider.Name()))
	if providerName == "" {
		return "", MarketplacePayoutDispatchResult{}, fmt.Errorf("dispatch marketplace payout: provider name is required")
	}
	request.Currency = strings.ToLower(strings.TrimSpace(request.Currency))
	if request.Currency == "" {
		request.Currency = "usd"
	}
	request.Amount = roundAmount(request.Amount)
	result, err := s.payoutProvider.CreatePayout(ctx, request)
	if err != nil {
		return "", MarketplacePayoutDispatchResult{}, fmt.Errorf("dispatch marketplace payout via %s: %w", providerName, err)
	}
	result.ProviderPayoutID = strings.TrimSpace(result.ProviderPayoutID)
	if result.ProviderPayoutID == "" {
		return "", MarketplacePayoutDispatchResult{}, fmt.Errorf("dispatch marketplace payout via %s: provider payout id is required", providerName)
	}
	return providerName, result, nil
}

func (s *SettlementService) loadOrder(ctx context.Context, orderID string) (*MarketplaceOrder, error) {
	return scanMarketplaceOrder(s.store.db.QueryRowContext(ctx, `
		SELECT mo.id, mo.buyer_organization_id, mo.buyer_user_id, mo.publisher_organization_id, mo.publisher_user_id,
		       mo.agent_id, COALESCE(mo.version_id, ''), pi.provider, mo.payment_intent_id,
		       COALESCE(mo.provider_checkout_session_id, ''), COALESCE(mo.provider_payment_intent_id, ''),
		       COALESCE(mo.install_id, ''), mo.gross_amount, mo.platform_fee_amount, mo.publisher_net_amount,
		       mo.refunded_amount, mo.currency, mo.status, mo.created_at, mo.updated_at
		FROM marketplace_orders mo
		JOIN payment_intents pi ON pi.id = mo.payment_intent_id
		WHERE mo.id = $1
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

func loadPayoutForProviderLifecycle(ctx context.Context, tx *sql.Tx, payoutID, providerPayoutID, provider string) (*MarketplacePayout, error) {
	var payout MarketplacePayout
	if err := tx.QueryRowContext(ctx, `
		SELECT id, publisher_organization_id, publisher_user_id, amount, currency,
		       provider, COALESCE(provider_payout_id, ''), status, created_at, updated_at
		FROM marketplace_payouts
		WHERE (($1 <> '' AND id = $1)
		   OR ($2 <> '' AND provider_payout_id = $2 AND ($3 = '' OR provider = $3)))
		ORDER BY CASE WHEN id = $1 THEN 0 ELSE 1 END
		LIMIT 1
		FOR UPDATE
	`, payoutID, providerPayoutID, provider).Scan(&payout.ID, &payout.PublisherOrganizationID, &payout.PublisherUserID, &payout.Amount,
		&payout.Currency, &payout.Provider, &payout.ProviderPayoutID, &payout.Status, &payout.CreatedAt, &payout.UpdatedAt); err != nil {
		return nil, fmt.Errorf("apply provider payout lifecycle: load payout: %w", err)
	}
	return &payout, nil
}

func (s *SettlementService) loadOrderForUpdate(ctx context.Context, tx *sql.Tx, orderID string, paymentIntentID string) (*MarketplaceOrder, error) {
	query := `
		SELECT mo.id, mo.buyer_organization_id, mo.buyer_user_id, mo.publisher_organization_id, mo.publisher_user_id,
		       mo.agent_id, COALESCE(mo.version_id, ''), pi.provider, mo.payment_intent_id,
		       COALESCE(mo.provider_checkout_session_id, ''), COALESCE(mo.provider_payment_intent_id, ''),
		       COALESCE(mo.install_id, ''), mo.gross_amount, mo.platform_fee_amount, mo.publisher_net_amount,
		       mo.refunded_amount, mo.currency, mo.status, mo.created_at, mo.updated_at
		FROM marketplace_orders mo
		JOIN payment_intents pi ON pi.id = mo.payment_intent_id
		WHERE mo.payment_intent_id = $1 AND (NULLIF($2, '') IS NULL OR mo.id = $2)
		FOR UPDATE OF mo
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
		       mo.agent_id, COALESCE(mo.version_id, ''), pi.provider, mo.payment_intent_id,
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
		&order.Provider, &order.PaymentIntentID, &order.ProviderCheckoutSessionID, &order.ProviderPaymentIntentID,
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

func marketplaceLifecycleTransitionKey(provider, eventID, transitionType, entityID string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "stripe"
	}
	return fmt.Sprintf("%s:%s:%s:%s", provider, eventID, transitionType, entityID)
}

func insertMarketplaceLifecycleTransition(ctx context.Context, tx *sql.Tx, provider, transitionKey, eventID, eventType, organizationID, userID, paymentIntentID, entityType, entityID, toState string) (bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "stripe"
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO billing_lifecycle_events (
			id, transition_key, provider, provider_event_id, event_type,
			organization_id, user_id, payment_intent_id, entity_type, entity_id,
			to_state, reason, payload, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11, $5, '{}', $12)
		ON CONFLICT (transition_key) DO NOTHING
	`, uuid.New().String(), transitionKey, provider, eventID, eventType, organizationID, userID, paymentIntentID, entityType, entityID, toState, time.Now().UTC())
	if err != nil {
		return false, fmt.Errorf("insert marketplace lifecycle transition: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("marketplace lifecycle transition rows affected: %w", err)
	}
	return rows > 0, nil
}

func (s *SettlementService) calculateOrderAmounts(orderAmount float64, publisherCumulativeSales float64) (marketplaceOrderAmounts, error) {
	gross := roundAmount(orderAmount)
	if gross <= 0 {
		return marketplaceOrderAmounts{}, fmt.Errorf("order amount must be positive")
	}
	platformFee, err := s.calculateSegmentedPlatformFee(gross, publisherCumulativeSales)
	if err != nil {
		return marketplaceOrderAmounts{}, err
	}
	return marketplaceOrderAmounts{
		GrossAmount:        gross,
		PlatformFeeAmount:  platformFee,
		PublisherNetAmount: roundAmount(gross - platformFee),
	}, nil
}

func (s *SettlementService) calculateSegmentedPlatformFee(orderAmount float64, publisherCumulativeSales float64) (float64, error) {
	tiers := normalizePlatformFeeTiers(s.platformFeeTiers)
	if len(tiers) == 0 {
		tiers = []MarketplacePlatformFeeTier{{MinimumAmount: 0, FeeBPS: defaultPlatformFeeBPS}}
	}
	for _, tier := range tiers {
		if tier.MinimumAmount < 0 {
			return 0, fmt.Errorf("platform fee tier minimum amount must be non-negative")
		}
		if tier.FeeBPS < 0 || tier.FeeBPS > 10000 {
			return 0, fmt.Errorf("platform fee tier fee bps must be between 0 and 10000")
		}
	}

	startAmount := 0.0
	switch s.platformFeeTierBasis {
	case MarketplaceFeeTierBasisCurrentOrderAmount, MarketplaceFeeTierBasisOrderAmount, "":
		startAmount = 0
	case MarketplaceFeeTierBasisPublisherCumulativeSales:
		startAmount = roundAmount(math.Max(publisherCumulativeSales, 0))
	default:
		return 0, fmt.Errorf("unsupported platform fee tier basis %q", s.platformFeeTierBasis)
	}

	endAmount := roundAmount(startAmount + orderAmount)
	if endAmount <= startAmount {
		return 0, nil
	}
	total := 0.0
	for i, tier := range tiers {
		nextMinimum := math.Inf(1)
		if i+1 < len(tiers) {
			nextMinimum = tiers[i+1].MinimumAmount
		}
		segmentStart := math.Max(startAmount, tier.MinimumAmount)
		segmentEnd := math.Min(endAmount, nextMinimum)
		if segmentEnd <= segmentStart {
			continue
		}
		total += (segmentEnd - segmentStart) * float64(tier.FeeBPS) / 10000
	}
	return roundAmount(total), nil
}

func (s *SettlementService) publisherCumulativeSales(ctx context.Context, tx *sql.Tx, publisherOrganizationID string, publisherUserID string) (float64, error) {
	var amount float64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(gross_amount), 0)
		FROM marketplace_orders
		WHERE publisher_organization_id = $1
		  AND publisher_user_id = $2
		  AND status IN ('paid', 'partially_refunded', 'refunded')
	`, publisherOrganizationID, publisherUserID).Scan(&amount); err != nil {
		return 0, err
	}
	return roundAmount(amount), nil
}

func (s *SettlementService) calculatePayoutAmount(settlement MarketplaceSettlement, now time.Time) (float64, error) {
	amount := roundAmount(settlement.PublisherNetAmount - settlement.RefundedAmount)
	if amount < 0 {
		amount = 0
	}
	if s.minimumSettlementAmount <= 0 || amount >= s.minimumSettlementAmount {
		return amount, nil
	}
	if s.minimumSettlementCycle > 0 && !settlement.CreatedAt.IsZero() && !now.Before(settlement.CreatedAt.Add(s.minimumSettlementCycle)) {
		return amount, nil
	}
	return 0, fmt.Errorf("mark payout pending: minimum settlement amount %.2f not met for settlement %s (amount %.2f)", s.minimumSettlementAmount, settlement.ID, amount)
}

func normalizePlatformFeeTiers(tiers []MarketplacePlatformFeeTier) []MarketplacePlatformFeeTier {
	if len(tiers) == 0 {
		return nil
	}
	normalized := append([]MarketplacePlatformFeeTier(nil), tiers...)
	sort.SliceStable(normalized, func(i, j int) bool {
		return normalized[i].MinimumAmount < normalized[j].MinimumAmount
	})
	return normalized
}

func roundAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
}
