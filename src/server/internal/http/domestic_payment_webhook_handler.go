package http

import (
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
	provider string
	secret   string
	ledger   stripebilling.WebhookLedger
}

type domesticPaymentWebhookEvent struct {
	ID                      string `json:"id"`
	Type                    string `json:"type"`
	OrganizationID          string `json:"organization_id"`
	UserID                  string `json:"user_id"`
	PaymentIntentID         string `json:"payment_intent_id"`
	ProviderPaymentIntentID string `json:"provider_payment_intent_id"`
}

func newDomesticPaymentWebhookHandler(provider string, secret string, ledger stripebilling.WebhookLedger) domesticPaymentWebhookHandler {
	return domesticPaymentWebhookHandler{
		provider: strings.ToLower(strings.TrimSpace(provider)),
		secret:   strings.TrimSpace(secret),
		ledger:   ledger,
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
	if _, err := h.ledger.RecordWebhookEvent(r.Context(), stripebilling.WebhookEvent{
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
	}); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "webhook_record_failed", "record payment webhook failed")
		return
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
