package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"strings"
	"time"

	stripebilling "oblivious/server/internal/stripe"
)

const (
	domesticPaymentTimestampHeader = "Oblivious-Payment-Timestamp"
	domesticPaymentSignatureHeader = "Oblivious-Payment-Signature"
)

type domesticPaymentWebhookHandler struct {
	provider  string
	secret    string
	ledger    stripebilling.WebhookLedger
	lifecycle domesticPaymentLifecycle
}

type domesticPaymentWebhookEvent struct {
	ID                        string  `json:"id"`
	Type                      string  `json:"type"`
	OrganizationID            string  `json:"organization_id"`
	UserID                    string  `json:"user_id"`
	PaymentIntentID           string  `json:"payment_intent_id"`
	MarketplaceOrderID        string  `json:"marketplace_order_id"`
	PackageID                 string  `json:"plan_id"`
	Kind                      string  `json:"kind"`
	ProviderPaymentIntentID   string  `json:"provider_payment_intent_id"`
	ProviderSubscriptionID    string  `json:"provider_subscription_id"`
	ProviderCustomerID        string  `json:"provider_customer_id"`
	ProviderCheckoutSessionID string  `json:"provider_checkout_session_id"`
	Amount                    float64 `json:"amount"`
	Currency                  string  `json:"currency"`
}

type domesticPaymentLifecycleInput struct {
	Provider                  string
	EventID                   string
	OrganizationID            string
	UserID                    string
	PaymentIntentID           string
	MarketplaceOrderID        string
	PackageID                 string
	Kind                      string
	ProviderPaymentIntentID   string
	ProviderSubscriptionID    string
	ProviderCustomerID        string
	ProviderCheckoutSessionID string
	Amount                    float64
	Currency                  string
}

type domesticPaymentLifecycle interface {
	ApplyDomesticCheckoutPaid(ctx context.Context, input domesticPaymentLifecycleInput, payload []byte) error
}

type stripeDomesticPaymentLifecycleAdapter struct {
	service *stripebilling.LifecycleService
}

func (a stripeDomesticPaymentLifecycleAdapter) ApplyDomesticCheckoutPaid(ctx context.Context, input domesticPaymentLifecycleInput, payload []byte) error {
	if a.service == nil {
		return nil
	}
	return a.service.ApplyDomesticCheckoutPaid(ctx, stripebilling.DomesticCheckoutPaid{
		Provider:                  input.Provider,
		EventID:                   input.EventID,
		OrganizationID:            input.OrganizationID,
		UserID:                    input.UserID,
		PaymentIntentID:           input.PaymentIntentID,
		MarketplaceOrderID:        input.MarketplaceOrderID,
		PackageID:                 input.PackageID,
		Kind:                      input.Kind,
		ProviderPaymentIntentID:   input.ProviderPaymentIntentID,
		ProviderSubscriptionID:    input.ProviderSubscriptionID,
		ProviderCustomerID:        input.ProviderCustomerID,
		ProviderCheckoutSessionID: input.ProviderCheckoutSessionID,
		Amount:                    input.Amount,
		Currency:                  input.Currency,
	}, payload)
}

func newDomesticPaymentWebhookHandler(provider string, secret string, ledger stripebilling.WebhookLedger, lifecycle ...domesticPaymentLifecycle) domesticPaymentWebhookHandler {
	var lifecycleApplier domesticPaymentLifecycle
	if len(lifecycle) > 0 {
		lifecycleApplier = lifecycle[0]
	}
	return domesticPaymentWebhookHandler{
		provider:  strings.ToLower(strings.TrimSpace(provider)),
		secret:    strings.TrimSpace(secret),
		ledger:    ledger,
		lifecycle: lifecycleApplier,
	}
}

func (h domesticPaymentWebhookHandler) handle(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.ledger == nil || h.secret == "" || h.provider == "" {
		writeError(w, stdhttp.StatusNotImplemented, "provider_not_configured", "payment provider webhook is not configured")
		return
	}
	r.Body = stdhttp.MaxBytesReader(w, r.Body, 64<<10)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_payload", "invalid webhook payload")
		return
	}
	timestamp := strings.TrimSpace(r.Header.Get(domesticPaymentTimestampHeader))
	signature := strings.TrimSpace(r.Header.Get(domesticPaymentSignatureHeader))
	if timestamp == "" || signature == "" || !verifyDomesticPaymentWebhookSignature(h.secret, timestamp, payload, signature) {
		writeError(w, stdhttp.StatusBadRequest, "invalid_signature", "invalid webhook signature")
		return
	}

	var event domesticPaymentWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_payload", "invalid webhook payload")
		return
	}
	event.ID = strings.TrimSpace(event.ID)
	event.Type = strings.TrimSpace(event.Type)
	if event.ID == "" || event.Type == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_payload", "webhook id and type are required")
		return
	}

	now := time.Now().UTC()
	status := "processed"
	errorMessage := ""
	if strings.TrimSpace(event.OrganizationID) == "" || strings.TrimSpace(event.UserID) == "" || strings.TrimSpace(event.PaymentIntentID) == "" {
		status = "failed"
		errorMessage = "missing organization_id, user_id, or payment_intent_id"
	}
	recorded, err := h.ledger.RecordWebhookEvent(r.Context(), stripebilling.WebhookEvent{
		Provider:        h.provider,
		EventID:         event.ID,
		EventType:       event.Type,
		Status:          status,
		OrganizationID:  strings.TrimSpace(event.OrganizationID),
		UserID:          strings.TrimSpace(event.UserID),
		PaymentIntentID: strings.TrimSpace(event.PaymentIntentID),
		Payload:         append([]byte(nil), payload...),
		Error:           errorMessage,
		ReceivedAt:      now,
		ProcessedAt:     &now,
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "webhook_record_failed", "record payment webhook failed")
		return
	}
	if recorded && h.lifecycle != nil && status == "processed" && event.Type == "checkout.paid" {
		if err := h.lifecycle.ApplyDomesticCheckoutPaid(r.Context(), domesticPaymentLifecycleInput{
			Provider:                  h.provider,
			EventID:                   event.ID,
			OrganizationID:            strings.TrimSpace(event.OrganizationID),
			UserID:                    strings.TrimSpace(event.UserID),
			PaymentIntentID:           strings.TrimSpace(event.PaymentIntentID),
			MarketplaceOrderID:        strings.TrimSpace(event.MarketplaceOrderID),
			PackageID:                 strings.TrimSpace(event.PackageID),
			Kind:                      strings.TrimSpace(event.Kind),
			ProviderPaymentIntentID:   strings.TrimSpace(event.ProviderPaymentIntentID),
			ProviderSubscriptionID:    strings.TrimSpace(event.ProviderSubscriptionID),
			ProviderCustomerID:        strings.TrimSpace(event.ProviderCustomerID),
			ProviderCheckoutSessionID: strings.TrimSpace(event.ProviderCheckoutSessionID),
			Amount:                    event.Amount,
			Currency:                  strings.TrimSpace(event.Currency),
		}, payload); err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "webhook_lifecycle_failed", "apply payment webhook lifecycle failed")
			return
		}
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]bool{"received": true})
}

func verifyDomesticPaymentWebhookSignature(secret string, timestamp string, payload []byte, signature string) bool {
	if strings.TrimSpace(secret) == "" || strings.TrimSpace(timestamp) == "" || strings.TrimSpace(signature) == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(signature)))
}
