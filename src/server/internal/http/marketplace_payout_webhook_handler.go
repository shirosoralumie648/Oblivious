package http

import (
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"strings"
	"time"

	"oblivious/server/internal/marketplace"
	stripebilling "oblivious/server/internal/stripe"
)

type marketplacePayoutLifecycle interface {
	ApplyProviderPayoutLifecycle(ctx context.Context, input marketplace.ProviderPayoutLifecycleEvent) (*marketplace.MarketplacePayout, error)
}

type marketplacePayoutWebhookHandler struct {
	provider  string
	secret    string
	ledger    stripebilling.WebhookLedger
	lifecycle marketplacePayoutLifecycle
}

func newMarketplacePayoutWebhookHandler(provider string, secret string, ledger stripebilling.WebhookLedger, lifecycle marketplacePayoutLifecycle) marketplacePayoutWebhookHandler {
	return marketplacePayoutWebhookHandler{
		provider:  strings.ToLower(strings.TrimSpace(provider)),
		secret:    strings.TrimSpace(secret),
		ledger:    ledger,
		lifecycle: lifecycle,
	}
}

func (h marketplacePayoutWebhookHandler) handle(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.ledger == nil || h.lifecycle == nil || h.secret == "" || h.provider == "" {
		writeError(w, stdhttp.StatusNotImplemented, "provider_not_configured", "marketplace payout webhook is not configured")
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
	if !isDomesticPayoutEventType(event.Type) {
		writeError(w, stdhttp.StatusBadRequest, "invalid_event_type", "marketplace payout webhook only accepts payout events")
		return
	}

	now := time.Now().UTC()
	processedAt := now
	status := "processed"
	errorMessage := ""
	if strings.TrimSpace(event.PayoutID) == "" && strings.TrimSpace(event.ProviderPayoutID) == "" {
		status = "failed"
		errorMessage = "missing payout_id or provider_payout_id"
	}
	recorded, err := h.ledger.RecordWebhookEvent(r.Context(), stripebilling.WebhookEvent{
		Provider:   h.provider,
		EventID:    event.ID,
		EventType:  event.Type,
		Status:     status,
		Payload:    append([]byte(nil), payload...),
		Error:      errorMessage,
		ReceivedAt: now,
		ProcessedAt: &processedAt,
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "webhook_record_failed", "record marketplace payout webhook failed")
		return
	}
	if recorded && status == "processed" {
		if _, err := h.lifecycle.ApplyProviderPayoutLifecycle(r.Context(), marketplace.ProviderPayoutLifecycleEvent{
			Provider:         h.provider,
			EventID:          event.ID,
			EventType:        event.Type,
			PayoutID:         strings.TrimSpace(event.PayoutID),
			ProviderPayoutID: strings.TrimSpace(event.ProviderPayoutID),
			Status:           strings.TrimSpace(event.Status),
			Reason:           strings.TrimSpace(event.Reason),
		}); err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "webhook_lifecycle_failed", "apply marketplace payout webhook lifecycle failed")
			return
		}
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]bool{"received": true})
}
