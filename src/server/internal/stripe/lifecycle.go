package stripe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	stripeapi "github.com/stripe/stripe-go/v83"
)

// LifecycleService applies verified provider events to local billing state.
type LifecycleService struct {
	store LifecycleStore
}

// LifecycleStore persists idempotent billing lifecycle transitions.
type LifecycleStore interface {
	ApplyCheckoutCompleted(ctx context.Context, eventID string, input checkoutCompletedLifecycle, payload []byte) error
	ApplyInvoicePaid(ctx context.Context, eventID string, input invoiceLifecycle, payload []byte) error
	ApplyInvoicePaymentFailed(ctx context.Context, eventID string, input invoiceLifecycle, payload []byte) error
	ApplySubscriptionUpdated(ctx context.Context, eventID string, input subscriptionLifecycle, payload []byte) error
	ApplySubscriptionDeleted(ctx context.Context, eventID string, input subscriptionLifecycle, payload []byte) error
	ApplyRefund(ctx context.Context, eventID string, input refundLifecycle, payload []byte) error
}

func NewLifecycleService(store LifecycleStore) *LifecycleService {
	return &LifecycleService{store: store}
}

func (s *LifecycleService) ApplyStripeEvent(ctx context.Context, event stripeapi.Event, payload []byte) error {
	if s == nil || s.store == nil {
		return nil
	}
	switch string(event.Type) {
	case "checkout.session.completed":
		input, err := parseCheckoutCompleted(event)
		if err != nil {
			return err
		}
		return s.store.ApplyCheckoutCompleted(ctx, event.ID, input, payload)
	case "invoice.paid":
		input, err := parseInvoiceLifecycle(event)
		if err != nil {
			return err
		}
		return s.store.ApplyInvoicePaid(ctx, event.ID, input, payload)
	case "invoice.payment_failed":
		input, err := parseInvoiceLifecycle(event)
		if err != nil {
			return err
		}
		return s.store.ApplyInvoicePaymentFailed(ctx, event.ID, input, payload)
	case "customer.subscription.updated":
		input, err := parseSubscriptionLifecycle(event)
		if err != nil {
			return err
		}
		return s.store.ApplySubscriptionUpdated(ctx, event.ID, input, payload)
	case "customer.subscription.deleted":
		input, err := parseSubscriptionLifecycle(event)
		if err != nil {
			return err
		}
		return s.store.ApplySubscriptionDeleted(ctx, event.ID, input, payload)
	case "refund.created", "charge.refunded":
		input, err := parseRefundLifecycle(event)
		if err != nil {
			return err
		}
		return s.store.ApplyRefund(ctx, event.ID, input, payload)
	default:
		return nil
	}
}

type checkoutCompletedLifecycle struct {
	SessionID                 string
	OrganizationID            string
	UserID                    string
	PaymentIntentID           string
	PackageID                 string
	Kind                      string
	ProviderPaymentIntentID   string
	ProviderSubscriptionID    string
	ProviderCustomerID        string
	ProviderCheckoutSessionID string
	Amount                    float64
	Currency                  string
}

type checkoutSessionLifecyclePayload struct {
	ID                string            `json:"id"`
	ClientReferenceID string            `json:"client_reference_id"`
	Metadata          map[string]string `json:"metadata"`
	PaymentIntent     stripeID          `json:"payment_intent"`
	Subscription      stripeID          `json:"subscription"`
	Customer          stripeID          `json:"customer"`
	AmountTotal       int64             `json:"amount_total"`
	Currency          string            `json:"currency"`
}

type stripeID string

func (id *stripeID) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*id = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*id = stripeID(value)
		return nil
	}
	var object struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*id = stripeID(object.ID)
	return nil
}

func parseCheckoutCompleted(event stripeapi.Event) (checkoutCompletedLifecycle, error) {
	if event.Data == nil {
		return checkoutCompletedLifecycle{}, fmt.Errorf("checkout.session.completed: missing event data")
	}
	var session checkoutSessionLifecyclePayload
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return checkoutCompletedLifecycle{}, fmt.Errorf("unmarshal checkout session: %w", err)
	}
	metadata := session.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	input := checkoutCompletedLifecycle{
		SessionID:                 session.ID,
		OrganizationID:            metadata["organization_id"],
		UserID:                    metadata["user_id"],
		PaymentIntentID:           metadata["payment_intent_id"],
		PackageID:                 metadata["plan_id"],
		Kind:                      metadata["checkout_kind"],
		ProviderPaymentIntentID:   string(session.PaymentIntent),
		ProviderSubscriptionID:    string(session.Subscription),
		ProviderCustomerID:        string(session.Customer),
		ProviderCheckoutSessionID: session.ID,
		Amount:                    float64(session.AmountTotal) / 100,
		Currency:                  session.Currency,
	}
	if input.PaymentIntentID == "" {
		input.PaymentIntentID = session.ClientReferenceID
	}
	if input.Kind == "" {
		input.Kind = "subscription"
	}
	if input.Currency == "" {
		input.Currency = "usd"
	}
	if input.OrganizationID == "" || input.UserID == "" || input.PaymentIntentID == "" {
		return checkoutCompletedLifecycle{}, fmt.Errorf("checkout.session.completed: missing organization_id, user_id, or payment_intent_id")
	}
	if input.Kind == "subscription" && input.PackageID == "" {
		return checkoutCompletedLifecycle{}, fmt.Errorf("checkout.session.completed: missing plan_id for subscription checkout")
	}
	return input, nil
}

type invoiceLifecycle struct {
	ProviderInvoiceID       string
	ProviderSubscriptionID  string
	ProviderPaymentIntentID string
	ProviderCustomerID      string
	OrganizationID          string
	UserID                  string
	PaymentIntentID         string
	Status                  string
	AmountDue               float64
	AmountPaid              float64
	Currency                string
	HostedInvoiceURL        string
	InvoicePDF              string
}

type invoiceLifecyclePayload struct {
	ID               string            `json:"id"`
	Subscription     stripeID          `json:"subscription"`
	PaymentIntent    stripeID          `json:"payment_intent"`
	Customer         stripeID          `json:"customer"`
	Status           string            `json:"status"`
	AmountDue        int64             `json:"amount_due"`
	AmountPaid       int64             `json:"amount_paid"`
	Currency         string            `json:"currency"`
	HostedInvoiceURL string            `json:"hosted_invoice_url"`
	InvoicePDF       string            `json:"invoice_pdf"`
	Metadata         map[string]string `json:"metadata"`
}

func parseInvoiceLifecycle(event stripeapi.Event) (invoiceLifecycle, error) {
	if event.Data == nil {
		return invoiceLifecycle{}, fmt.Errorf("%s: missing event data", event.Type)
	}
	var invoice invoiceLifecyclePayload
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return invoiceLifecycle{}, fmt.Errorf("unmarshal invoice: %w", err)
	}
	metadata := invoice.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	status := invoice.Status
	if event.Type == "invoice.paid" {
		status = "paid"
	}
	if event.Type == "invoice.payment_failed" {
		status = "failed"
	}
	input := invoiceLifecycle{
		ProviderInvoiceID:       invoice.ID,
		ProviderSubscriptionID:  string(invoice.Subscription),
		ProviderPaymentIntentID: string(invoice.PaymentIntent),
		ProviderCustomerID:      string(invoice.Customer),
		OrganizationID:          metadata["organization_id"],
		UserID:                  metadata["user_id"],
		PaymentIntentID:         metadata["payment_intent_id"],
		Status:                  status,
		AmountDue:               centsToAmount(invoice.AmountDue),
		AmountPaid:              centsToAmount(invoice.AmountPaid),
		Currency:                invoice.Currency,
		HostedInvoiceURL:        invoice.HostedInvoiceURL,
		InvoicePDF:              invoice.InvoicePDF,
	}
	if input.Currency == "" {
		input.Currency = "usd"
	}
	if input.ProviderInvoiceID == "" || input.OrganizationID == "" || input.UserID == "" {
		return invoiceLifecycle{}, fmt.Errorf("%s: missing invoice id, organization_id, or user_id", event.Type)
	}
	return input, nil
}

type subscriptionLifecycle struct {
	ProviderSubscriptionID string
	ProviderCustomerID     string
	OrganizationID         string
	UserID                 string
	Status                 string
	CancelAtPeriodEnd      bool
}

type subscriptionLifecyclePayload struct {
	ID                string            `json:"id"`
	Customer          stripeID          `json:"customer"`
	Status            string            `json:"status"`
	CancelAtPeriodEnd bool              `json:"cancel_at_period_end"`
	Metadata          map[string]string `json:"metadata"`
}

func parseSubscriptionLifecycle(event stripeapi.Event) (subscriptionLifecycle, error) {
	if event.Data == nil {
		return subscriptionLifecycle{}, fmt.Errorf("%s: missing event data", event.Type)
	}
	var subscription subscriptionLifecyclePayload
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return subscriptionLifecycle{}, fmt.Errorf("unmarshal subscription: %w", err)
	}
	metadata := subscription.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	input := subscriptionLifecycle{
		ProviderSubscriptionID: subscription.ID,
		ProviderCustomerID:     string(subscription.Customer),
		OrganizationID:         metadata["organization_id"],
		UserID:                 metadata["user_id"],
		Status:                 normalizeSubscriptionStatus(subscription.Status),
		CancelAtPeriodEnd:      subscription.CancelAtPeriodEnd,
	}
	if event.Type == "customer.subscription.deleted" {
		input.Status = "cancelled"
	}
	if input.ProviderSubscriptionID == "" || input.OrganizationID == "" || input.UserID == "" {
		return subscriptionLifecycle{}, fmt.Errorf("%s: missing subscription id, organization_id, or user_id", event.Type)
	}
	return input, nil
}

type refundLifecycle struct {
	ProviderRefundID        string
	ProviderChargeID        string
	ProviderPaymentIntentID string
	OrganizationID          string
	UserID                  string
	PaymentIntentID         string
	Amount                  float64
	Currency                string
	Status                  string
	Reason                  string
}

type refundLifecyclePayload struct {
	ID            string            `json:"id"`
	Charge        stripeID          `json:"charge"`
	PaymentIntent stripeID          `json:"payment_intent"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	Status        string            `json:"status"`
	Reason        string            `json:"reason"`
	Metadata      map[string]string `json:"metadata"`
}

func parseRefundLifecycle(event stripeapi.Event) (refundLifecycle, error) {
	if event.Data == nil {
		return refundLifecycle{}, fmt.Errorf("%s: missing event data", event.Type)
	}
	var refund refundLifecyclePayload
	if err := json.Unmarshal(event.Data.Raw, &refund); err != nil {
		return refundLifecycle{}, fmt.Errorf("unmarshal refund: %w", err)
	}
	metadata := refund.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	status := refund.Status
	if status == "" {
		status = "succeeded"
	}
	input := refundLifecycle{
		ProviderRefundID:        refund.ID,
		ProviderChargeID:        string(refund.Charge),
		ProviderPaymentIntentID: string(refund.PaymentIntent),
		OrganizationID:          metadata["organization_id"],
		UserID:                  metadata["user_id"],
		PaymentIntentID:         metadata["payment_intent_id"],
		Amount:                  centsToAmount(refund.Amount),
		Currency:                refund.Currency,
		Status:                  status,
		Reason:                  refund.Reason,
	}
	if input.Currency == "" {
		input.Currency = "usd"
	}
	if input.ProviderRefundID == "" || input.OrganizationID == "" || input.UserID == "" {
		return refundLifecycle{}, fmt.Errorf("%s: missing refund id, organization_id, or user_id", event.Type)
	}
	return input, nil
}

// SQLLifecycleStore applies lifecycle transitions in PostgreSQL.
type SQLLifecycleStore struct {
	db *sql.DB
}

func NewSQLLifecycleStore(db *sql.DB) *SQLLifecycleStore {
	return &SQLLifecycleStore{db: db}
}

func (s *SQLLifecycleStore) ApplyCheckoutCompleted(ctx context.Context, eventID string, input checkoutCompletedLifecycle, payload []byte) error {
	transitionKey := fmt.Sprintf("stripe:%s:checkout:%s", eventID, input.PaymentIntentID)
	return s.withTransition(ctx, lifecycleTransition{
		Key:             transitionKey,
		ProviderEventID: eventID,
		EventType:       "checkout.session.completed",
		OrganizationID:  input.OrganizationID,
		UserID:          input.UserID,
		PaymentIntentID: input.PaymentIntentID,
		EntityType:      input.Kind,
		EntityID:        input.PaymentIntentID,
		ToState:         "completed",
		Reason:          "checkout.session.completed",
		Payload:         payload,
	}, func(tx *sql.Tx) error {
		var currentStatus, currentKind, packageID string
		var currentAmount float64
		if err := tx.QueryRowContext(ctx, `
			SELECT status, kind, COALESCE(package_id, ''), amount
			FROM payment_intents
			WHERE id = $1 AND organization_id = $2 AND user_id = $3
			FOR UPDATE
		`, input.PaymentIntentID, input.OrganizationID, input.UserID).Scan(&currentStatus, &currentKind, &packageID, &currentAmount); err != nil {
			return fmt.Errorf("load payment intent: %w", err)
		}
		if currentKind != input.Kind {
			return fmt.Errorf("checkout kind mismatch: intent=%s event=%s", currentKind, input.Kind)
		}
		if input.PackageID == "" {
			input.PackageID = packageID
		}
		if input.Amount == 0 {
			input.Amount = currentAmount
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE payment_intents
			SET status = 'completed',
			    provider_checkout_session_id = COALESCE(NULLIF($2, ''), provider_checkout_session_id),
			    provider_payment_intent_id = COALESCE(NULLIF($3, ''), provider_payment_intent_id),
			    provider_subscription_id = COALESCE(NULLIF($4, ''), provider_subscription_id),
			    updated_at = $5
			WHERE id = $1
		`, input.PaymentIntentID, input.ProviderCheckoutSessionID, input.ProviderPaymentIntentID, input.ProviderSubscriptionID, time.Now().UTC()); err != nil {
			return fmt.Errorf("complete payment intent: %w", err)
		}

		switch input.Kind {
		case "subscription":
			return s.applySubscriptionCheckout(ctx, tx, input)
		case "topup":
			return s.applyTopupCheckout(ctx, tx, input)
		default:
			return fmt.Errorf("unsupported checkout kind %q", input.Kind)
		}
	})
}

func (s *SQLLifecycleStore) ApplyInvoicePaid(ctx context.Context, eventID string, input invoiceLifecycle, payload []byte) error {
	return s.applyInvoice(ctx, eventID, input, payload, true)
}

func (s *SQLLifecycleStore) ApplyInvoicePaymentFailed(ctx context.Context, eventID string, input invoiceLifecycle, payload []byte) error {
	return s.applyInvoice(ctx, eventID, input, payload, false)
}

func (s *SQLLifecycleStore) applyInvoice(ctx context.Context, eventID string, input invoiceLifecycle, payload []byte, paid bool) error {
	transitionKey := fmt.Sprintf("stripe:%s:invoice:%s", eventID, input.ProviderInvoiceID)
	eventType := "invoice.payment_failed"
	toState := "failed"
	if paid {
		eventType = "invoice.paid"
		toState = "paid"
	}
	return s.withTransition(ctx, lifecycleTransition{
		Key:             transitionKey,
		ProviderEventID: eventID,
		EventType:       eventType,
		OrganizationID:  input.OrganizationID,
		UserID:          input.UserID,
		PaymentIntentID: input.PaymentIntentID,
		EntityType:      "invoice",
		EntityID:        input.ProviderInvoiceID,
		ToState:         toState,
		Reason:          eventType,
		Payload:         payload,
	}, func(tx *sql.Tx) error {
		subscriptionID, err := s.findSubscriptionID(ctx, tx, input.OrganizationID, input.UserID, input.ProviderSubscriptionID)
		if err != nil {
			return err
		}
		paymentIntentID := input.PaymentIntentID
		if paymentIntentID == "" && input.ProviderPaymentIntentID != "" {
			_ = tx.QueryRowContext(ctx, `
				SELECT id FROM payment_intents
				WHERE organization_id = $1 AND user_id = $2 AND provider_payment_intent_id = $3
				LIMIT 1
			`, input.OrganizationID, input.UserID, input.ProviderPaymentIntentID).Scan(&paymentIntentID)
		}
		if err := s.upsertInvoice(ctx, tx, input, subscriptionID, paymentIntentID, payload); err != nil {
			return err
		}
		now := time.Now().UTC()
		if paid {
			if subscriptionID != "" {
				if _, err := tx.ExecContext(ctx, `
					UPDATE subscriptions
					SET status = 'active',
					    package_id = COALESCE(next_plan_id, package_id),
					    next_plan_id = NULL,
					    provider_latest_invoice_id = $2,
					    provider_customer_id = COALESCE(NULLIF($3, ''), provider_customer_id),
					    failed_payment_at = NULL,
					    updated_at = $4
					WHERE id = $1
				`, subscriptionID, input.ProviderInvoiceID, input.ProviderCustomerID, now); err != nil {
					return fmt.Errorf("mark subscription invoice paid: %w", err)
				}
			}
			if paymentIntentID != "" {
				if _, err := tx.ExecContext(ctx, `
					UPDATE payment_intents
					SET status = 'completed',
					    provider_payment_intent_id = COALESCE(NULLIF($2, ''), provider_payment_intent_id),
					    provider_subscription_id = COALESCE(NULLIF($3, ''), provider_subscription_id),
					    provider_invoice_id = $4,
					    updated_at = $5
					WHERE id = $1
				`, paymentIntentID, input.ProviderPaymentIntentID, input.ProviderSubscriptionID, input.ProviderInvoiceID, now); err != nil {
					return fmt.Errorf("mark invoice payment intent complete: %w", err)
				}
			}
			return nil
		}
		if subscriptionID != "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE subscriptions
				SET status = 'past_due',
				    provider_latest_invoice_id = $2,
				    failed_payment_at = $3,
				    updated_at = $3
				WHERE id = $1
			`, subscriptionID, input.ProviderInvoiceID, now); err != nil {
				return fmt.Errorf("mark subscription payment failed: %w", err)
			}
		}
		if paymentIntentID != "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE payment_intents
				SET status = 'failed',
				    provider_payment_intent_id = COALESCE(NULLIF($2, ''), provider_payment_intent_id),
				    provider_invoice_id = $3,
				    updated_at = $4
				WHERE id = $1
			`, paymentIntentID, input.ProviderPaymentIntentID, input.ProviderInvoiceID, now); err != nil {
				return fmt.Errorf("mark invoice payment intent failed: %w", err)
			}
		}
		return nil
	})
}

func (s *SQLLifecycleStore) ApplySubscriptionUpdated(ctx context.Context, eventID string, input subscriptionLifecycle, payload []byte) error {
	return s.applySubscription(ctx, eventID, "customer.subscription.updated", input, payload)
}

func (s *SQLLifecycleStore) ApplySubscriptionDeleted(ctx context.Context, eventID string, input subscriptionLifecycle, payload []byte) error {
	input.Status = "cancelled"
	return s.applySubscription(ctx, eventID, "customer.subscription.deleted", input, payload)
}

func (s *SQLLifecycleStore) applySubscription(ctx context.Context, eventID string, eventType string, input subscriptionLifecycle, payload []byte) error {
	transitionKey := fmt.Sprintf("stripe:%s:subscription:%s", eventID, input.ProviderSubscriptionID)
	return s.withTransition(ctx, lifecycleTransition{
		Key:             transitionKey,
		ProviderEventID: eventID,
		EventType:       eventType,
		OrganizationID:  input.OrganizationID,
		UserID:          input.UserID,
		EntityType:      "subscription",
		EntityID:        input.ProviderSubscriptionID,
		ToState:         input.Status,
		Reason:          eventType,
		Payload:         payload,
	}, func(tx *sql.Tx) error {
		now := time.Now().UTC()
		result, err := tx.ExecContext(ctx, `
			UPDATE subscriptions
			SET status = COALESCE(NULLIF($4, ''), status),
			    provider_customer_id = COALESCE(NULLIF($5, ''), provider_customer_id),
			    cancel_at_period_end = $6,
			    updated_at = $7
			WHERE organization_id = $1 AND user_id = $2 AND provider_subscription_id = $3
		`, input.OrganizationID, input.UserID, input.ProviderSubscriptionID, input.Status, input.ProviderCustomerID, input.CancelAtPeriodEnd, now)
		if err != nil {
			return fmt.Errorf("update subscription lifecycle: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("subscription lifecycle rows affected: %w", err)
		}
		if rows == 0 {
			return sql.ErrNoRows
		}
		return nil
	})
}

func (s *SQLLifecycleStore) ApplyRefund(ctx context.Context, eventID string, input refundLifecycle, payload []byte) error {
	transitionKey := fmt.Sprintf("stripe:%s:refund:%s", eventID, input.ProviderRefundID)
	return s.withTransition(ctx, lifecycleTransition{
		Key:             transitionKey,
		ProviderEventID: eventID,
		EventType:       "refund.created",
		OrganizationID:  input.OrganizationID,
		UserID:          input.UserID,
		PaymentIntentID: input.PaymentIntentID,
		EntityType:      "refund",
		EntityID:        input.ProviderRefundID,
		ToState:         input.Status,
		Reason:          input.Reason,
		Payload:         payload,
	}, func(tx *sql.Tx) error {
		paymentIntentID := input.PaymentIntentID
		if paymentIntentID == "" && input.ProviderPaymentIntentID != "" {
			_ = tx.QueryRowContext(ctx, `
				SELECT id FROM payment_intents
				WHERE organization_id = $1 AND user_id = $2 AND provider_payment_intent_id = $3
				LIMIT 1
			`, input.OrganizationID, input.UserID, input.ProviderPaymentIntentID).Scan(&paymentIntentID)
		}
		if paymentIntentID == "" {
			return fmt.Errorf("refund requires local payment intent")
		}

		var intentKind, intentStatus string
		var intentAmount, priorRefunded float64
		if err := tx.QueryRowContext(ctx, `
			SELECT kind, status, amount, refunded_amount
			FROM payment_intents
			WHERE id = $1 AND organization_id = $2 AND user_id = $3
			FOR UPDATE
		`, paymentIntentID, input.OrganizationID, input.UserID).Scan(&intentKind, &intentStatus, &intentAmount, &priorRefunded); err != nil {
			return fmt.Errorf("load refund payment intent: %w", err)
		}
		_ = intentStatus

		topupOrderID := ""
		var topupPriorRefunded float64
		if intentKind == "topup" {
			if err := tx.QueryRowContext(ctx, `
				SELECT id, refunded_amount FROM topup_orders
				WHERE payment_intent_id = $1 AND organization_id = $2 AND user_id = $3
				FOR UPDATE
			`, paymentIntentID, input.OrganizationID, input.UserID).Scan(&topupOrderID, &topupPriorRefunded); err != nil {
				return fmt.Errorf("load refund topup order: %w", err)
			}
		}

		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO billing_refunds (
				id, provider, provider_refund_id, provider_charge_id, provider_payment_intent_id,
				organization_id, user_id, payment_intent_id, topup_order_id,
				amount, currency, status, reason, payload, created_at, updated_at
			)
			VALUES ($1, 'stripe', $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, NULLIF($8, ''), $9, $10, $11, NULLIF($12, ''), $13, $14, $14)
			ON CONFLICT (provider, provider_refund_id) DO UPDATE
			SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at, payload = EXCLUDED.payload
		`, uuid.New().String(), input.ProviderRefundID, input.ProviderChargeID, input.ProviderPaymentIntentID, input.OrganizationID, input.UserID, paymentIntentID, topupOrderID, input.Amount, input.Currency, input.Status, input.Reason, payloadOrEmpty(payload), now); err != nil {
			return fmt.Errorf("upsert refund: %w", err)
		}

		refundedTotal := priorRefunded + input.Amount
		if refundedTotal > intentAmount {
			refundedTotal = intentAmount
		}
		refundStatus := "partially_refunded"
		if refundedTotal >= intentAmount {
			refundStatus = "refunded"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE payment_intents
			SET status = $2,
			    refunded_amount = $3,
			    updated_at = $4
			WHERE id = $1
		`, paymentIntentID, refundStatus, refundedTotal, now); err != nil {
			return fmt.Errorf("update refunded payment intent: %w", err)
		}

		if topupOrderID != "" {
			topupRefundedTotal := topupPriorRefunded + input.Amount
			if topupRefundedTotal > intentAmount {
				topupRefundedTotal = intentAmount
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE topup_orders SET refunded_amount = $2 WHERE id = $1
			`, topupOrderID, topupRefundedTotal); err != nil {
				return fmt.Errorf("update topup refunded amount: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE quotas
				SET balance = balance - $2, updated_at = $3
				WHERE organization_id = $1
			`, input.OrganizationID, input.Amount, now); err != nil {
				return fmt.Errorf("reverse refunded topup quota: %w", err)
			}
		}
		return nil
	})
}

type lifecycleTransition struct {
	Key             string
	ProviderEventID string
	EventType       string
	OrganizationID  string
	UserID          string
	PaymentIntentID string
	EntityType      string
	EntityID        string
	FromState       string
	ToState         string
	Reason          string
	Payload         []byte
}

func (s *SQLLifecycleStore) withTransition(ctx context.Context, transition lifecycleTransition, apply func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin lifecycle transaction: %w", err)
	}
	defer tx.Rollback()

	payload := transition.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO billing_lifecycle_events (
			id, transition_key, provider, provider_event_id, event_type,
			organization_id, user_id, payment_intent_id, entity_type, entity_id,
			from_state, to_state, reason, payload, created_at
		)
		VALUES ($1, $2, 'stripe', $3, $4, $5, $6, NULLIF($7, ''), $8, NULLIF($9, ''), NULLIF($10, ''), $11, NULLIF($12, ''), $13, $14)
		ON CONFLICT (transition_key) DO NOTHING
	`, uuid.New().String(), transition.Key, transition.ProviderEventID, transition.EventType, transition.OrganizationID, transition.UserID, transition.PaymentIntentID, transition.EntityType, transition.EntityID, transition.FromState, transition.ToState, transition.Reason, payload, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("insert lifecycle transition: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("lifecycle transition rows affected: %w", err)
	}
	if rows == 0 {
		return nil
	}
	if err := apply(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit lifecycle transaction: %w", err)
	}
	return nil
}

func (s *SQLLifecycleStore) applySubscriptionCheckout(ctx context.Context, tx *sql.Tx, input checkoutCompletedLifecycle) error {
	now := time.Now().UTC()
	if input.PackageID == "" {
		return fmt.Errorf("subscription checkout requires package id")
	}
	var subscriptionID string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM subscriptions
		WHERE organization_id = $1 AND user_id = $2
		  AND (
		    NULLIF($3, '') IS NOT NULL AND provider_subscription_id = $3
		    OR (NULLIF($3, '') IS NULL AND status IN ('active', 'past_due'))
		  )
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, input.OrganizationID, input.UserID, input.ProviderSubscriptionID).Scan(&subscriptionID)
	if err == sql.ErrNoRows {
		subscriptionID = uuid.New().String()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO subscriptions (
				id, organization_id, user_id, package_id, status, started_at,
				current_period_start, current_period_end, provider_subscription_id,
				provider_customer_id, provider_checkout_session_id, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, 'active', $5, $5, NULL, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $5, $5)
		`, subscriptionID, input.OrganizationID, input.UserID, input.PackageID, now, input.ProviderSubscriptionID, input.ProviderCustomerID, input.ProviderCheckoutSessionID); err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("find subscription: %w", err)
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE subscriptions
			SET package_id = $2,
			    status = 'active',
			    provider_subscription_id = COALESCE(NULLIF($3, ''), provider_subscription_id),
			    provider_customer_id = COALESCE(NULLIF($4, ''), provider_customer_id),
			    provider_checkout_session_id = COALESCE(NULLIF($5, ''), provider_checkout_session_id),
			    failed_payment_at = NULL,
			    updated_at = $6
			WHERE id = $1
		`, subscriptionID, input.PackageID, input.ProviderSubscriptionID, input.ProviderCustomerID, input.ProviderCheckoutSessionID, now); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET plan_id = $1 WHERE id = $2`, input.PackageID, input.UserID); err != nil {
		return fmt.Errorf("update user plan: %w", err)
	}
	return nil
}

func (s *SQLLifecycleStore) applyTopupCheckout(ctx context.Context, tx *sql.Tx, input checkoutCompletedLifecycle) error {
	now := time.Now().UTC()
	var orderID, status string
	var amount float64
	if err := tx.QueryRowContext(ctx, `
		SELECT id, status, amount
		FROM topup_orders
		WHERE payment_intent_id = $1 AND organization_id = $2 AND user_id = $3
		FOR UPDATE
	`, input.PaymentIntentID, input.OrganizationID, input.UserID).Scan(&orderID, &status, &amount); err != nil {
		return fmt.Errorf("load topup order: %w", err)
	}
	if input.Amount == 0 {
		input.Amount = amount
	}
	if status != "paid" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE topup_orders
			SET status = 'paid', trade_no = NULLIF($2, ''), paid_at = $3,
			    provider_checkout_session_id = COALESCE(NULLIF($4, ''), provider_checkout_session_id)
			WHERE id = $1
		`, orderID, input.ProviderPaymentIntentID, now, input.ProviderCheckoutSessionID); err != nil {
			return fmt.Errorf("mark topup paid: %w", err)
		}
		if err := s.ensureQuota(ctx, tx, input.UserID, input.OrganizationID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE quotas
			SET balance = balance + $2, updated_at = $3
			WHERE organization_id = $1
		`, input.OrganizationID, amount, now); err != nil {
			return fmt.Errorf("credit topup quota: %w", err)
		}
	}
	return nil
}

func (s *SQLLifecycleStore) findSubscriptionID(ctx context.Context, tx *sql.Tx, organizationID, userID, providerSubscriptionID string) (string, error) {
	if providerSubscriptionID == "" {
		return "", nil
	}
	var subscriptionID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM subscriptions
		WHERE organization_id = $1 AND user_id = $2 AND provider_subscription_id = $3
		LIMIT 1
		FOR UPDATE
	`, organizationID, userID, providerSubscriptionID).Scan(&subscriptionID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find subscription: %w", err)
	}
	return subscriptionID, nil
}

func (s *SQLLifecycleStore) upsertInvoice(ctx context.Context, tx *sql.Tx, input invoiceLifecycle, subscriptionID string, paymentIntentID string, payload []byte) error {
	now := time.Now().UTC()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO billing_invoices (
			id, provider, provider_invoice_id, provider_subscription_id, provider_payment_intent_id,
			organization_id, user_id, subscription_id, payment_intent_id, status,
			amount_due, amount_paid, currency, hosted_invoice_url, invoice_pdf, payload, created_at, updated_at
		)
		VALUES ($1, 'stripe', $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, NULLIF($13, ''), NULLIF($14, ''), $15, $16, $16)
		ON CONFLICT (provider, provider_invoice_id) DO UPDATE
		SET status = EXCLUDED.status,
		    amount_due = EXCLUDED.amount_due,
		    amount_paid = EXCLUDED.amount_paid,
		    provider_subscription_id = EXCLUDED.provider_subscription_id,
		    provider_payment_intent_id = EXCLUDED.provider_payment_intent_id,
		    subscription_id = EXCLUDED.subscription_id,
		    payment_intent_id = EXCLUDED.payment_intent_id,
		    payload = EXCLUDED.payload,
		    updated_at = EXCLUDED.updated_at
	`, uuid.New().String(), input.ProviderInvoiceID, input.ProviderSubscriptionID, input.ProviderPaymentIntentID, input.OrganizationID, input.UserID, subscriptionID, paymentIntentID, input.Status, input.AmountDue, input.AmountPaid, input.Currency, input.HostedInvoiceURL, input.InvoicePDF, payloadOrEmpty(payload), now)
	if err != nil {
		return fmt.Errorf("upsert invoice: %w", err)
	}
	return nil
}

func payloadOrEmpty(payload []byte) []byte {
	if len(payload) == 0 {
		return []byte(`{}`)
	}
	return payload
}

func (s *SQLLifecycleStore) ensureQuota(ctx context.Context, tx *sql.Tx, userID, organizationID string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO quotas (id, organization_id, user_id, balance, used, created_at, updated_at)
		VALUES ($1, $2, $3, 0, 0, $4, $4)
		ON CONFLICT (organization_id) DO NOTHING
	`, "quota_"+uuid.New().String(), organizationID, userID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("ensure quota: %w", err)
	}
	return nil
}

func centsToAmount(cents int64) float64 {
	return float64(cents) / 100
}

func normalizeSubscriptionStatus(status string) string {
	switch status {
	case "", "active", "trialing":
		return "active"
	case "past_due", "unpaid", "incomplete", "incomplete_expired":
		return "past_due"
	case "canceled":
		return "cancelled"
	default:
		return status
	}
}
