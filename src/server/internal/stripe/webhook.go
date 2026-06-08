package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	stripeapi "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/webhook"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
)

// WebhookHandler verifies Stripe webhook signatures and records provider events
// into the payment ledger before later lifecycle phases apply business effects.
type WebhookHandler struct {
	ledger    WebhookLedger
	secret    string
	lifecycle LifecycleApplier
}

type LifecycleApplier interface {
	ApplyStripeEvent(ctx context.Context, event stripeapi.Event, payload []byte) error
}

type webhookErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type webhookEnvelope struct {
	OK    bool                 `json:"ok"`
	Data  any                  `json:"data"`
	Error *webhookErrorPayload `json:"error"`
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(ledger WebhookLedger, webhookSecret string, lifecycle ...LifecycleApplier) *WebhookHandler {
	handler := &WebhookHandler{
		ledger: ledger,
		secret: webhookSecret,
	}
	if len(lifecycle) > 0 {
		handler.lifecycle = lifecycle[0]
	}
	return handler
}

// HandleWebhook processes an incoming Stripe webhook request.
// It must verify the Stripe signature against the raw request body before parsing.
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx, span := observability.StartSpan(r.Context(), "stripe.webhook")
	defer span.End()

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		metrics.RecordStripeWebhookFailure("invalid_payload")
		writeError(w, http.StatusBadRequest, "invalid_payload", "invalid payload")
		return
	}

	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), h.secret)
	if err != nil {
		metrics.RecordStripeWebhookFailure("invalid_signature")
		writeError(w, http.StatusBadRequest, "invalid_signature", "invalid signature")
		return
	}

	record := webhookEventRecord(event, payload)
	_, err = h.ledger.RecordWebhookEvent(ctx, record)
	if err != nil {
		metrics.RecordStripeWebhookEvent(string(event.Type), "record_failed")
		metrics.RecordStripeWebhookFailure("ledger_record_failed")
		writeError(w, http.StatusInternalServerError, "webhook_record_failed", "record webhook event failed")
		return
	}
	if h.lifecycle != nil {
		if err := h.lifecycle.ApplyStripeEvent(ctx, event, payload); err != nil {
			metrics.RecordStripeWebhookEvent(string(event.Type), "lifecycle_failed")
			metrics.RecordStripeWebhookFailure("lifecycle_apply_failed")
			writeError(w, http.StatusInternalServerError, "webhook_lifecycle_failed", "apply webhook lifecycle failed")
			return
		}
	}

	metrics.RecordStripeWebhookEvent(string(event.Type), record.Status)
	writeSuccess(w, http.StatusOK, map[string]bool{"received": true})
}

func webhookEventRecord(event stripeapi.Event, payload []byte) WebhookEvent {
	now := time.Now().UTC()
	record := WebhookEvent{
		Provider:    "stripe",
		EventID:     event.ID,
		EventType:   string(event.Type),
		Status:      "processed",
		Payload:     append([]byte(nil), payload...),
		ReceivedAt:  now,
		ProcessedAt: &now,
	}

	switch event.Type {
	case "checkout.session.completed":
		enrichCheckoutCompletedRecord(&record, event)
	}

	return record
}

func enrichCheckoutCompletedRecord(record *WebhookEvent, event stripeapi.Event) {
	var session stripeapi.CheckoutSession
	if event.Data == nil {
		record.Status = "failed"
		record.Error = "checkout.session.completed: missing event data"
		return
	}
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		record.Status = "failed"
		record.Error = fmt.Sprintf("unmarshal checkout session: %v", err)
		return
	}

	record.OrganizationID = session.Metadata["organization_id"]
	record.UserID = session.Metadata["user_id"]
	record.PaymentIntentID = session.Metadata["payment_intent_id"]

	checkoutKind := session.Metadata["checkout_kind"]
	planID := session.Metadata["plan_id"]
	planRequired := checkoutKind != "topup" && checkoutKind != "marketplace_install"
	if record.OrganizationID == "" || record.UserID == "" || (planRequired && planID == "") {
		record.Status = "failed"
		record.Error = fmt.Sprintf("checkout.session.completed: missing organization_id, user_id, or plan_id (organization_id=%q, user_id=%q, plan_id=%q)", record.OrganizationID, record.UserID, planID)
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeSuccess(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, webhookEnvelope{
		OK:    true,
		Data:  data,
		Error: nil,
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, webhookEnvelope{
		OK:   false,
		Data: nil,
		Error: &webhookErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}
