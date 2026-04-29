package stripe

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	stripe "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
	"github.com/google/uuid"
)

// WebhookHandler processes Stripe webhook events.
// Per D-15: handles checkout.session.completed to update subscription state.
type WebhookHandler struct {
	db     *sql.DB
	secret string
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(db *sql.DB, webhookSecret string) *WebhookHandler {
	return &WebhookHandler{
		db:     db,
		secret: webhookSecret,
	}
}

// HandleWebhook processes an incoming Stripe webhook request.
// Verifies the Stripe signature, checks idempotency, then routes to the appropriate event handler.
// All events return 200 to Stripe to prevent unnecessary retries, even on processing errors.
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 64KB (T-03-14: DoS mitigation)
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		// Body too large or read error — acknowledge to prevent retries
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	// Verify Stripe signature (T-03-09: Spoofing prevention)
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.secret)
	if err != nil {
		// Invalid signature — return 400 (Stripe will log but not retry)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signature"})
		return
	}

	// Idempotency check (T-03-10: replay detection)
	if h.isDuplicateEvent(event.ID) {
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	// Process event by type
	var processErr error
	switch event.Type {
	case "checkout.session.completed":
		processErr = h.handleCheckoutCompleted(event)
	case "invoice.payment_succeeded":
		// Log only — future use
		processErr = h.handleInvoicePaid(event)
	case "customer.subscription.deleted":
		processErr = h.handleSubscriptionDeleted(event)
	default:
		// Acknowledge unhandled events to prevent Stripe retries
	}

	// Record webhook receipt in audit log for idempotency
	h.recordWebhookEvent(event.ID, string(event.Type), processErr)

	// Always return 200 to Stripe — errors are logged internally
	writeJSON(w, http.StatusOK, map[string]bool{"received": true})
}

// handleCheckoutCompleted processes a checkout.session.completed event.
// Per D-15: extracts user_id and plan_id from checkout metadata, then creates or
// updates the subscription and assigns the plan to the user.
func (h *WebhookHandler) handleCheckoutCompleted(event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("unmarshal checkout session: %w", err)
	}

	userID := session.Metadata["user_id"]
	planID := session.Metadata["plan_id"]

	// T-03-11: Validate metadata — user_id and plan_id must be present
	if userID == "" || planID == "" {
		return fmt.Errorf("checkout.session.completed: missing user_id or plan_id in metadata (user_id=%q, plan_id=%q)", userID, planID)
	}

	// Validate plan exists
	var planExists bool
	err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM packages WHERE id = $1)`, planID).Scan(&planExists)
	if err != nil || !planExists {
		return fmt.Errorf("checkout.session.completed: plan %s not found", planID)
	}

	// Validate user exists
	var userExists bool
	err = h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&userExists)
	if err != nil || !userExists {
		return fmt.Errorf("checkout.session.completed: user %s not found", userID)
	}

	now := time.Now()
	periodEnd := now.Add(30 * 24 * time.Hour)

	// Check for existing active subscription for this user-plan combination
	var existingSubID string
	err = h.db.QueryRow(`
		SELECT id FROM subscriptions
		WHERE user_id = $1 AND status = 'active'
		LIMIT 1
	`, userID).Scan(&existingSubID)

	if err == sql.ErrNoRows {
		// No active subscription: create new one
		subID := uuid.New().String()
		_, err = h.db.Exec(`
			INSERT INTO subscriptions (id, user_id, package_id, status, started_at, current_period_start, current_period_end, created_at)
			VALUES ($1, $2, $3, 'active', $4, $4, $5, $4)
		`, subID, userID, planID, now, periodEnd)
		if err != nil {
			return fmt.Errorf("insert subscription: %w", err)
		}
	} else if err == nil {
		// Active subscription exists: ensure it reflects the paid plan
		_, err = h.db.Exec(`
			UPDATE subscriptions SET package_id = $1, status = 'active',
				current_period_start = $2, current_period_end = $3
			WHERE id = $4
		`, planID, now, periodEnd, existingSubID)
		if err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
	} else {
		return fmt.Errorf("query active subscription: %w", err)
	}

	// Update user's plan assignment
	_, err = h.db.Exec(`UPDATE users SET plan_id = $1 WHERE id = $2`, planID, userID)
	if err != nil {
		return fmt.Errorf("update user plan_id: %w", err)
	}

	// Insert audit log for the subscription update
	h.insertAuditLog(userID, "plan.checkout_completed", "subscription", planID, "checkout.session.completed")

	return nil
}

// handleInvoicePaid logs successful invoice payments (future use for usage billing).
func (h *WebhookHandler) handleInvoicePaid(event stripe.Event) error {
	// Log only — no action required currently
	return nil
}

// handleSubscriptionDeleted handles subscription cancellation.
// Updates subscription status and clears the user's plan assignment.
func (h *WebhookHandler) handleSubscriptionDeleted(event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return fmt.Errorf("unmarshal subscription: %w", err)
	}

	// Find user via customer metadata or subscriptions table
	userID := sub.Metadata["user_id"]
	if userID == "" {
		// No user_id in metadata — try lookup via customer
		if sub.Customer != nil {
			custID := sub.Customer.ID
			err := h.db.QueryRow(`SELECT id FROM users WHERE id = $1`, custID).Scan(&userID)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("lookup user by customer: %w", err)
			}
		}
	}

	if userID != "" {
		// Cancel active subscription
		_, err := h.db.Exec(`
			UPDATE subscriptions SET status = 'cancelled' WHERE user_id = $1 AND status = 'active'
		`, userID)
		if err != nil {
			return fmt.Errorf("cancel subscription: %w", err)
		}

		// Clear user's plan assignment
		_, err = h.db.Exec(`UPDATE users SET plan_id = NULL WHERE id = $1`, userID)
		if err != nil {
			return fmt.Errorf("clear user plan: %w", err)
		}

		h.insertAuditLog(userID, "plan.cancelled", "subscription", sub.ID, "customer.subscription.deleted")
	}

	return nil
}

// isDuplicateEvent checks if a webhook event has already been processed.
// Uses the audit_logs table with action='stripe.webhook' and resource_id = eventID.
func (h *WebhookHandler) isDuplicateEvent(eventID string) bool {
	var exists bool
	err := h.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM audit_logs WHERE action = 'stripe.webhook' AND resource_id = $1)
	`, eventID).Scan(&exists)
	if err != nil {
		return false // On error, process the event (safer to process twice than skip)
	}
	return exists
}

// recordWebhookEvent inserts a record into audit_logs for idempotency tracking.
func (h *WebhookHandler) recordWebhookEvent(eventID, eventType string, processErr error) {
	changes := eventType
	if processErr != nil {
		changes = eventType + ": error=" + processErr.Error()
	}

	_, _ = h.db.Exec(`
		INSERT INTO audit_logs (id, actor_id, actor_email, action, resource_type, resource_id, changes, created_at)
		VALUES ($1, 'system', 'stripe-webhook', 'stripe.webhook', 'webhook', $2, $3, $4)
	`, uuid.New().String(), eventID, changes, time.Now())
}

// insertAuditLog inserts a business audit log entry for subscription changes.
func (h *WebhookHandler) insertAuditLog(actorID, action, resourceType, resourceID, changes string) {
	_, _ = h.db.Exec(`
		INSERT INTO audit_logs (id, actor_id, actor_email, action, resource_type, resource_id, changes, created_at)
		VALUES ($1, $2, '', $3, $4, $5, $6, $7)
	`, uuid.New().String(), actorID, action, resourceType, resourceID, changes, time.Now())
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
