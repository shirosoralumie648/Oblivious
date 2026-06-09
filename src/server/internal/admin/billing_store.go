package admin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BillingInspectionStore defines Admin billing inspection and operator recovery operations.
type BillingInspectionStore interface {
	GetBillingInspectionSummary(ctx context.Context, filter BillingInspectionFilter) (*BillingInspectionSummary, error)
	ListBillingSessions(ctx context.Context, filter BillingInspectionFilter) ([]*BillingSessionInspection, int, error)
	ListPaymentIntents(ctx context.Context, filter BillingInspectionFilter) ([]*PaymentIntentInspection, int, error)
	ListWebhookEvents(ctx context.Context, filter BillingInspectionFilter) ([]*WebhookEventInspection, int, error)
	ListSubscriptions(ctx context.Context, filter BillingInspectionFilter) ([]*SubscriptionInspection, int, error)
	ListTopups(ctx context.Context, filter BillingInspectionFilter) ([]*TopupInspection, int, error)
	ListInvoices(ctx context.Context, filter BillingInspectionFilter) ([]*InvoiceInspection, int, error)
	ListRefunds(ctx context.Context, filter BillingInspectionFilter) ([]*RefundInspection, int, error)
	RecordTopupRefund(ctx context.Context, topupID string, request TopupRefundRequest) (*RefundInspection, error)
	ListMarketplaceSettlements(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplaceSettlementInspection, int, error)
	ListMarketplacePayouts(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplacePayoutInspection, int, error)
}

type billingColumnMap struct {
	OrganizationID string
	UserID         string
	Status         string
	Kind           string
	Provider       string
}

func normalizeBillingFilter(filter BillingInspectionFilter) BillingInspectionFilter {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	filter.OrganizationID = strings.TrimSpace(filter.OrganizationID)
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Kind = strings.TrimSpace(filter.Kind)
	filter.Provider = strings.TrimSpace(filter.Provider)
	return filter
}

func billingWhere(filter BillingInspectionFilter, columns billingColumnMap) (string, []any) {
	var conditions []string
	var args []any
	add := func(column, value string) {
		if column == "" || value == "" {
			return
		}
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	add(columns.OrganizationID, filter.OrganizationID)
	add(columns.UserID, filter.UserID)
	add(columns.Status, filter.Status)
	add(columns.Kind, filter.Kind)
	add(columns.Provider, filter.Provider)
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func countRows(ctx context.Context, db *sql.DB, table string, filter BillingInspectionFilter, columns billingColumnMap) (int, error) {
	where, args := billingWhere(filter, columns)
	var total int
	if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, table, where), args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if value.Valid {
		return &value.Time
	}
	return nil
}

func appendLimitOffset(args []any, filter BillingInspectionFilter) []any {
	args = append(args, filter.Limit, filter.Offset)
	return args
}

func topupSummaryQuery(filter BillingInspectionFilter) (string, []any) {
	columns := billingColumnMap{
		OrganizationID: "topup_orders.organization_id",
		UserID:         "topup_orders.user_id",
		Status:         "topup_orders.status",
		Provider:       "payment_intents.provider",
	}
	where, args := billingWhere(filter, columns)
	return `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN topup_orders.status = 'paid' THEN topup_orders.money ELSE 0 END), 0), COALESCE(SUM(topup_orders.refunded_amount), 0)
		FROM topup_orders
		LEFT JOIN payment_intents ON payment_intents.id = topup_orders.payment_intent_id ` + where, args
}

func (s *SQLStore) GetBillingInspectionSummary(ctx context.Context, filter BillingInspectionFilter) (*BillingInspectionSummary, error) {
	filter = normalizeBillingFilter(filter)
	summary := &BillingInspectionSummary{}

	where, args := billingWhere(filter, billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(pre_authorized_amt), 0), COALESCE(SUM(settled_amt), 0)
		FROM billing_sessions `+where, args...).Scan(&summary.BillingSessions.Count, &summary.BillingSessions.PreAuthorizedAmount, &summary.BillingSessions.SettledAmount); err != nil {
		return nil, fmt.Errorf("billing summary sessions: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Kind: "kind", Provider: "provider"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0), COALESCE(SUM(refunded_amount), 0)
		FROM payment_intents `+where, args...).Scan(&summary.PaymentIntents.Count, &summary.PaymentIntents.TotalAmount, &summary.PaymentIntents.RefundedAmount); err != nil {
		return nil, fmt.Errorf("billing summary payment intents: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Provider: "provider"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'failed')
		FROM stripe_webhook_events `+where, args...).Scan(&summary.WebhookEvents.Count, &summary.WebhookEvents.FailedCount); err != nil {
		return nil, fmt.Errorf("billing summary webhooks: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'active')
		FROM subscriptions `+where, args...).Scan(&summary.Subscriptions.Count, &summary.Subscriptions.ActiveCount); err != nil {
		return nil, fmt.Errorf("billing summary subscriptions: %w", err)
	}

	query, args := topupSummaryQuery(filter)
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&summary.Topups.Count, &summary.Topups.PaidAmount, &summary.Topups.RefundedAmount); err != nil {
		return nil, fmt.Errorf("billing summary topups: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Provider: "provider"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount_due), 0), COALESCE(SUM(amount_paid), 0)
		FROM billing_invoices `+where, args...).Scan(&summary.Invoices.Count, &summary.Invoices.AmountDue, &summary.Invoices.AmountPaid); err != nil {
		return nil, fmt.Errorf("billing summary invoices: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Provider: "provider"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM billing_refunds `+where, args...).Scan(&summary.Refunds.Count, &summary.Refunds.TotalAmount); err != nil {
		return nil, fmt.Errorf("billing summary refunds: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "publisher_organization_id", UserID: "publisher_user_id", Status: "status"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(gross_amount), 0), COALESCE(SUM(platform_fee_amount), 0),
		       COALESCE(SUM(publisher_net_amount), 0), COALESCE(SUM(refunded_amount), 0)
		FROM marketplace_settlements `+where, args...).Scan(&summary.Settlements.Count, &summary.Settlements.GrossAmount,
		&summary.Settlements.PlatformFeeAmount, &summary.Settlements.PublisherNetAmount, &summary.Settlements.RefundedAmount); err != nil {
		return nil, fmt.Errorf("billing summary settlements: %w", err)
	}

	where, args = billingWhere(filter, billingColumnMap{OrganizationID: "publisher_organization_id", UserID: "publisher_user_id", Status: "status", Provider: "provider"})
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM marketplace_payouts `+where, args...).Scan(&summary.Payouts.Count, &summary.Payouts.TotalAmount); err != nil {
		return nil, fmt.Errorf("billing summary payouts: %w", err)
	}

	return summary, nil
}

func (s *SQLStore) ListBillingSessions(ctx context.Context, filter BillingInspectionFilter) ([]*BillingSessionInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status"}
	total, err := countRows(ctx, s.db, "billing_sessions", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count billing sessions: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, COALESCE(channel_id, ''), COALESCE(model, ''), COALESCE(api_type, ''),
		       idempotency_key, pre_authorized_amt, settled_amt, status, created_at, settled_at
		FROM billing_sessions `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list billing sessions: %w", err)
	}
	defer rows.Close()

	var items []*BillingSessionInspection
	for rows.Next() {
		var item BillingSessionInspection
		var settledAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.ChannelID, &item.Model, &item.APIType,
			&item.IdempotencyKey, &item.PreAuthorizedAmount, &item.SettledAmount, &item.Status, &item.CreatedAt, &settledAt); err != nil {
			return nil, 0, fmt.Errorf("scan billing session: %w", err)
		}
		item.SettledAt = nullTimePtr(settledAt)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListPaymentIntents(ctx context.Context, filter BillingInspectionFilter) ([]*PaymentIntentInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Kind: "kind", Provider: "provider"}
	total, err := countRows(ctx, s.db, "payment_intents", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count payment intents: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, COALESCE(provider_checkout_session_id, ''), COALESCE(provider_payment_intent_id, ''),
		       organization_id, user_id, COALESCE(package_id, ''), kind, amount, currency, status, refunded_amount, created_at, updated_at
		FROM payment_intents `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list payment intents: %w", err)
	}
	defer rows.Close()

	var items []*PaymentIntentInspection
	for rows.Next() {
		var item PaymentIntentInspection
		if err := rows.Scan(&item.ID, &item.Provider, &item.ProviderCheckoutSessionID, &item.ProviderPaymentIntentID,
			&item.OrganizationID, &item.UserID, &item.PackageID, &item.Kind, &item.Amount, &item.Currency,
			&item.Status, &item.RefundedAmount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan payment intent: %w", err)
		}
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListWebhookEvents(ctx context.Context, filter BillingInspectionFilter) ([]*WebhookEventInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Provider: "provider"}
	total, err := countRows(ctx, s.db, "stripe_webhook_events", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count webhook events: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, event_id, event_type, status, organization_id, user_id, payment_intent_id,
		       COALESCE(error, ''), received_at, processed_at
		FROM stripe_webhook_events `+where+`
		ORDER BY received_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list webhook events: %w", err)
	}
	defer rows.Close()

	var items []*WebhookEventInspection
	for rows.Next() {
		var item WebhookEventInspection
		var organizationID, userID, paymentIntentID sql.NullString
		var processedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Provider, &item.EventID, &item.EventType, &item.Status, &organizationID,
			&userID, &paymentIntentID, &item.Error, &item.ReceivedAt, &processedAt); err != nil {
			return nil, 0, fmt.Errorf("scan webhook event: %w", err)
		}
		item.OrganizationID = nullStringValue(organizationID)
		item.UserID = nullStringValue(userID)
		item.PaymentIntentID = nullStringValue(paymentIntentID)
		item.ProcessedAt = nullTimePtr(processedAt)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListSubscriptions(ctx context.Context, filter BillingInspectionFilter) ([]*SubscriptionInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status"}
	total, err := countRows(ctx, s.db, "subscriptions", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count subscriptions: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, package_id, status, COALESCE(provider_subscription_id, ''),
		       COALESCE(provider_customer_id, ''), COALESCE(provider_checkout_session_id, ''),
		       COALESCE(provider_latest_invoice_id, ''), current_period_start, current_period_end,
		       cancel_at_period_end, failed_payment_at, created_at, updated_at
		FROM subscriptions `+where+`
		ORDER BY updated_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var items []*SubscriptionInspection
	for rows.Next() {
		var item SubscriptionInspection
		var currentPeriodEnd, failedPaymentAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.UserID, &item.PackageID, &item.Status,
			&item.ProviderSubscriptionID, &item.ProviderCustomerID, &item.ProviderCheckoutSessionID,
			&item.ProviderLatestInvoiceID, &item.CurrentPeriodStart, &currentPeriodEnd, &item.CancelAtPeriodEnd,
			&failedPaymentAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan subscription: %w", err)
		}
		item.CurrentPeriodEnd = nullTimePtr(currentPeriodEnd)
		item.FailedPaymentAt = nullTimePtr(failedPaymentAt)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListTopups(ctx context.Context, filter BillingInspectionFilter) ([]*TopupInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{
		OrganizationID: "topup_orders.organization_id",
		UserID:         "topup_orders.user_id",
		Status:         "topup_orders.status",
		Provider:       "payment_intents.provider",
	}
	where, args := billingWhere(filter, columns)
	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM topup_orders
		LEFT JOIN payment_intents ON payment_intents.id = topup_orders.payment_intent_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count topups: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT topup_orders.id, topup_orders.organization_id, topup_orders.user_id, topup_orders.payment_intent_id,
		       topup_orders.provider_checkout_session_id, COALESCE(payment_intents.provider, ''),
		       COALESCE(payment_intents.provider_payment_intent_id, ''), COALESCE(payment_intents.currency, ''),
		       topup_orders.amount, topup_orders.money, topup_orders.status, topup_orders.trade_no,
		       topup_orders.refunded_amount, topup_orders.paid_at, topup_orders.created_at
		FROM topup_orders
		LEFT JOIN payment_intents ON payment_intents.id = topup_orders.payment_intent_id `+where+`
		ORDER BY topup_orders.created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list topups: %w", err)
	}
	defer rows.Close()

	var items []*TopupInspection
	for rows.Next() {
		var item TopupInspection
		var paymentIntentID, providerCheckoutSessionID, provider, providerPaymentIntentID, currency, tradeNo sql.NullString
		var paidAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.UserID, &paymentIntentID, &providerCheckoutSessionID,
			&provider, &providerPaymentIntentID, &currency, &item.Amount, &item.Money, &item.Status, &tradeNo,
			&item.RefundedAmount, &paidAt, &item.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan topup: %w", err)
		}
		item.PaymentIntentID = nullStringValue(paymentIntentID)
		item.ProviderCheckoutSessionID = nullStringValue(providerCheckoutSessionID)
		item.Provider = nullStringValue(provider)
		item.ProviderPaymentIntentID = nullStringValue(providerPaymentIntentID)
		item.Currency = nullStringValue(currency)
		item.TradeNo = nullStringValue(tradeNo)
		item.PaidAt = nullTimePtr(paidAt)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListInvoices(ctx context.Context, filter BillingInspectionFilter) ([]*InvoiceInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Provider: "provider"}
	total, err := countRows(ctx, s.db, "billing_invoices", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, provider_invoice_id, COALESCE(provider_subscription_id, ''), COALESCE(provider_payment_intent_id, ''),
		       organization_id, user_id, subscription_id, payment_intent_id, status, amount_due, amount_paid, currency,
		       COALESCE(hosted_invoice_url, ''), COALESCE(invoice_pdf, ''), period_start, period_end, created_at, updated_at
		FROM billing_invoices `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	var items []*InvoiceInspection
	for rows.Next() {
		var item InvoiceInspection
		var subscriptionID, paymentIntentID sql.NullString
		var periodStart, periodEnd sql.NullTime
		if err := rows.Scan(&item.ID, &item.Provider, &item.ProviderInvoiceID, &item.ProviderSubscriptionID,
			&item.ProviderPaymentIntentID, &item.OrganizationID, &item.UserID, &subscriptionID, &paymentIntentID,
			&item.Status, &item.AmountDue, &item.AmountPaid, &item.Currency, &item.HostedInvoiceURL,
			&item.InvoicePDF, &periodStart, &periodEnd, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan invoice: %w", err)
		}
		item.SubscriptionID = nullStringValue(subscriptionID)
		item.PaymentIntentID = nullStringValue(paymentIntentID)
		item.PeriodStart = nullTimePtr(periodStart)
		item.PeriodEnd = nullTimePtr(periodEnd)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListRefunds(ctx context.Context, filter BillingInspectionFilter) ([]*RefundInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "organization_id", UserID: "user_id", Status: "status", Provider: "provider"}
	total, err := countRows(ctx, s.db, "billing_refunds", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count refunds: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, provider_refund_id, COALESCE(provider_charge_id, ''), COALESCE(provider_payment_intent_id, ''),
		       organization_id, user_id, payment_intent_id, topup_order_id, amount, currency, status, COALESCE(reason, ''),
		       created_at, updated_at
		FROM billing_refunds `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list refunds: %w", err)
	}
	defer rows.Close()

	var items []*RefundInspection
	for rows.Next() {
		var item RefundInspection
		var paymentIntentID, topupOrderID sql.NullString
		if err := rows.Scan(&item.ID, &item.Provider, &item.ProviderRefundID, &item.ProviderChargeID,
			&item.ProviderPaymentIntentID, &item.OrganizationID, &item.UserID, &paymentIntentID, &topupOrderID,
			&item.Amount, &item.Currency, &item.Status, &item.Reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan refund: %w", err)
		}
		item.PaymentIntentID = nullStringValue(paymentIntentID)
		item.TopupOrderID = nullStringValue(topupOrderID)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) RecordTopupRefund(ctx context.Context, topupID string, request TopupRefundRequest) (*RefundInspection, error) {
	topupID = strings.TrimSpace(topupID)
	if topupID == "" {
		return nil, fmt.Errorf("topup id is required")
	}
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		provider = "manual"
	}
	providerRefundID := strings.TrimSpace(request.ProviderRefundID)
	if providerRefundID == "" {
		return nil, fmt.Errorf("provider refund id is required")
	}
	if request.Amount <= 0 {
		return nil, fmt.Errorf("refund amount must be positive")
	}
	currency := strings.ToLower(strings.TrimSpace(request.Currency))
	status := strings.TrimSpace(request.Status)
	if status == "" {
		status = "succeeded"
	}
	reason := strings.TrimSpace(request.Reason)
	providerChargeID := strings.TrimSpace(request.ProviderChargeID)
	providerPaymentIntentID := strings.TrimSpace(request.ProviderPaymentIntentID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin topup refund: %w", err)
	}
	defer tx.Rollback()

	var topup TopupInspection
	var paidAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, COALESCE(payment_intent_id, ''), COALESCE(provider_checkout_session_id, ''),
		       amount, money, status, COALESCE(trade_no, ''), refunded_amount, paid_at, created_at
		FROM topup_orders
		WHERE id = $1
		FOR UPDATE
	`, topupID).Scan(&topup.ID, &topup.OrganizationID, &topup.UserID, &topup.PaymentIntentID, &topup.ProviderCheckoutSessionID,
		&topup.Amount, &topup.Money, &topup.Status, &topup.TradeNo, &topup.RefundedAmount, &paidAt, &topup.CreatedAt); err != nil {
		return nil, fmt.Errorf("load topup for refund: %w", err)
	}
	topup.PaidAt = nullTimePtr(paidAt)
	if topup.Status != "paid" && topup.Status != "partially_refunded" {
		return nil, fmt.Errorf("topup must be paid before refund")
	}
	if topup.PaymentIntentID == "" {
		return nil, fmt.Errorf("topup refund requires payment intent")
	}

	var intentKind, intentStatus, intentCurrency string
	var intentAmount, priorRefunded float64
	if err := tx.QueryRowContext(ctx, `
		SELECT kind, status, amount, currency, refunded_amount, COALESCE(provider_payment_intent_id, '')
		FROM payment_intents
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
		FOR UPDATE
	`, topup.PaymentIntentID, topup.OrganizationID, topup.UserID).Scan(
		&intentKind,
		&intentStatus,
		&intentAmount,
		&intentCurrency,
		&priorRefunded,
		&providerPaymentIntentID,
	); err != nil {
		return nil, fmt.Errorf("load topup payment intent: %w", err)
	}
	if intentKind != "topup" {
		return nil, fmt.Errorf("payment intent is not a topup")
	}
	if currency == "" {
		currency = strings.ToLower(intentCurrency)
	}
	if currency == "" {
		currency = "usd"
	}
	available := intentAmount - priorRefunded
	if available <= 0 {
		return nil, fmt.Errorf("topup is already fully refunded")
	}
	if request.Amount > available {
		return nil, fmt.Errorf("refund amount exceeds refundable balance")
	}

	now := time.Now().UTC()
	eventID := "admin_topup_refund:" + provider + ":" + providerRefundID
	transitionKey := provider + ":" + eventID + ":refund:" + providerRefundID
	result, err := tx.ExecContext(ctx, `
		INSERT INTO billing_lifecycle_events (
			id, transition_key, provider, provider_event_id, event_type,
			organization_id, user_id, payment_intent_id, entity_type, entity_id,
			from_state, to_state, reason, payload, created_at
		)
		VALUES ($1, $2, $3, $4, 'refund.created', $5, $6, $7, 'refund', $8, $9, $10, NULLIF($11, ''), $12, $13)
		ON CONFLICT (transition_key) DO NOTHING
	`, uuid.New().String(), transitionKey, provider, eventID, topup.OrganizationID, topup.UserID, topup.PaymentIntentID, providerRefundID, intentStatus, status, reason, []byte(`{}`), now)
	if err != nil {
		return nil, fmt.Errorf("insert topup refund lifecycle: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("topup refund lifecycle rows affected: %w", err)
	}
	if rows == 0 {
		refund, err := s.getRefundByProviderID(ctx, tx, provider, providerRefundID)
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit idempotent topup refund: %w", err)
		}
		return refund, nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO billing_refunds (
			id, provider, provider_refund_id, provider_charge_id, provider_payment_intent_id,
			organization_id, user_id, payment_intent_id, topup_order_id,
			amount, currency, status, reason, payload, created_at, updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''), $14, $15, $15)
	`, uuid.New().String(), provider, providerRefundID, providerChargeID, providerPaymentIntentID, topup.OrganizationID, topup.UserID,
		topup.PaymentIntentID, topup.ID, request.Amount, currency, status, reason, []byte(`{}`), now); err != nil {
		return nil, fmt.Errorf("insert topup refund: %w", err)
	}

	refundedTotal := priorRefunded + request.Amount
	if refundedTotal > intentAmount {
		refundedTotal = intentAmount
	}
	refundStatus := "partially_refunded"
	if refundedTotal >= intentAmount {
		refundStatus = "refunded"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE payment_intents
		SET status = $2, refunded_amount = $3, updated_at = $4
		WHERE id = $1
	`, topup.PaymentIntentID, refundStatus, refundedTotal, now); err != nil {
		return nil, fmt.Errorf("update topup refund payment intent: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE topup_orders
		SET refunded_amount = $2
		WHERE id = $1
	`, topup.ID, topup.RefundedAmount+request.Amount); err != nil {
		return nil, fmt.Errorf("update refunded topup: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE quotas
		SET balance = balance - $2, updated_at = $3
		WHERE organization_id = $1 AND scope = 'organization'
	`, topup.OrganizationID, request.Amount, now); err != nil {
		return nil, fmt.Errorf("reverse topup refund quota: %w", err)
	}

	refund, err := s.getRefundByProviderID(ctx, tx, provider, providerRefundID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit topup refund: %w", err)
	}
	return refund, nil
}

func (s *SQLStore) getRefundByProviderID(ctx context.Context, tx *sql.Tx, provider string, providerRefundID string) (*RefundInspection, error) {
	var item RefundInspection
	var paymentIntentID, topupOrderID sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT id, provider, provider_refund_id, COALESCE(provider_charge_id, ''), COALESCE(provider_payment_intent_id, ''),
		       organization_id, user_id, payment_intent_id, topup_order_id, amount, currency, status, COALESCE(reason, ''),
		       created_at, updated_at
		FROM billing_refunds
		WHERE provider = $1 AND provider_refund_id = $2
	`, provider, providerRefundID).Scan(&item.ID, &item.Provider, &item.ProviderRefundID, &item.ProviderChargeID,
		&item.ProviderPaymentIntentID, &item.OrganizationID, &item.UserID, &paymentIntentID, &topupOrderID,
		&item.Amount, &item.Currency, &item.Status, &item.Reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, fmt.Errorf("load topup refund record: %w", err)
	}
	item.PaymentIntentID = nullStringValue(paymentIntentID)
	item.TopupOrderID = nullStringValue(topupOrderID)
	return &item, nil
}

func (s *SQLStore) ListMarketplaceSettlements(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplaceSettlementInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "publisher_organization_id", UserID: "publisher_user_id", Status: "status"}
	total, err := countRows(ctx, s.db, "marketplace_settlements", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count marketplace settlements: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, order_id, publisher_organization_id, publisher_user_id, agent_id, gross_amount,
		       platform_fee_amount, publisher_net_amount, refunded_amount, payout_id, status, hold_until, created_at, updated_at
		FROM marketplace_settlements `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list marketplace settlements: %w", err)
	}
	defer rows.Close()

	var items []*MarketplaceSettlementInspection
	for rows.Next() {
		var item MarketplaceSettlementInspection
		var payoutID sql.NullString
		var holdUntil sql.NullTime
		if err := rows.Scan(&item.ID, &item.OrderID, &item.PublisherOrganizationID, &item.PublisherUserID,
			&item.AgentID, &item.GrossAmount, &item.PlatformFeeAmount, &item.PublisherNetAmount, &item.RefundedAmount,
			&payoutID, &item.Status, &holdUntil, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan marketplace settlement: %w", err)
		}
		item.PayoutID = nullStringValue(payoutID)
		item.HoldUntil = nullTimePtr(holdUntil)
		items = append(items, &item)
	}
	return items, total, rows.Err()
}

func (s *SQLStore) ListMarketplacePayouts(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplacePayoutInspection, int, error) {
	filter = normalizeBillingFilter(filter)
	columns := billingColumnMap{OrganizationID: "publisher_organization_id", UserID: "publisher_user_id", Status: "status", Provider: "provider"}
	total, err := countRows(ctx, s.db, "marketplace_payouts", filter, columns)
	if err != nil {
		return nil, 0, fmt.Errorf("count marketplace payouts: %w", err)
	}
	where, args := billingWhere(filter, columns)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, publisher_organization_id, publisher_user_id, amount, currency, provider,
		       COALESCE(provider_payout_id, ''), status, created_at, updated_at
		FROM marketplace_payouts `+where+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), appendLimitOffset(args, filter)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list marketplace payouts: %w", err)
	}
	defer rows.Close()

	var items []*MarketplacePayoutInspection
	for rows.Next() {
		var item MarketplacePayoutInspection
		if err := rows.Scan(&item.ID, &item.PublisherOrganizationID, &item.PublisherUserID, &item.Amount,
			&item.Currency, &item.Provider, &item.ProviderPayoutID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan marketplace payout: %w", err)
		}
		items = append(items, &item)
	}
	return items, total, rows.Err()
}
