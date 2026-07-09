package marketplace

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WebhookPayoutProvider struct {
	endpointURL string
	secret      string
	client      *http.Client
}

type WebhookPayoutProviderOption func(*WebhookPayoutProvider)

func NewWebhookPayoutProvider(endpointURL, secret string, opts ...WebhookPayoutProviderOption) *WebhookPayoutProvider {
	provider := &WebhookPayoutProvider{
		endpointURL: strings.TrimSpace(endpointURL),
		secret:      strings.TrimSpace(secret),
		client:      &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(provider)
		}
	}
	return provider
}

func WithWebhookPayoutHTTPClient(client *http.Client) WebhookPayoutProviderOption {
	return func(provider *WebhookPayoutProvider) {
		if client != nil {
			provider.client = client
		}
	}
}

func (p *WebhookPayoutProvider) Name() string {
	return "webhook"
}

func (p *WebhookPayoutProvider) CreatePayout(ctx context.Context, request MarketplacePayoutDispatchRequest) (MarketplacePayoutDispatchResult, error) {
	if p == nil {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("webhook payout provider is not configured")
	}
	if err := validateWebhookPayoutEndpoint(p.endpointURL); err != nil {
		return MarketplacePayoutDispatchResult{}, err
	}
	if strings.TrimSpace(p.secret) == "" {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("webhook payout secret is required")
	}

	request.Currency = strings.ToLower(strings.TrimSpace(request.Currency))
	if request.Currency == "" {
		request.Currency = "usd"
	}
	request.Amount = roundAmount(request.Amount)
	payload := webhookPayoutRequest{
		PayoutID:                strings.TrimSpace(request.PayoutID),
		PublisherOrganizationID: strings.TrimSpace(request.PublisherOrganizationID),
		PublisherUserID:         strings.TrimSpace(request.PublisherUserID),
		Amount:                  request.Amount,
		Currency:                request.Currency,
		SettlementIDs:           normalizeWebhookPayoutSettlementIDs(request.SettlementIDs),
	}
	if payload.PayoutID == "" || payload.PublisherOrganizationID == "" || payload.PublisherUserID == "" || payload.Amount <= 0 {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("webhook payout request requires payout, publisher, and positive amount")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("marshal webhook payout request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpointURL, bytes.NewReader(body))
	if err != nil {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("create webhook payout request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("User-Agent", "oblivious-marketplace-payout/1.0")
	httpRequest.Header.Set("X-Oblivious-Payout-ID", payload.PayoutID)
	httpRequest.Header.Set("X-Oblivious-Payout-Signature", signWebhookPayoutBody(body, p.secret))

	httpClient := p.client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := httpClient.Do(httpRequest)
	if err != nil {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("dispatch webhook payout: %w", err)
	}
	defer response.Body.Close()

	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("read webhook payout response: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("webhook payout returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var responsePayload webhookPayoutResponse
	if err := json.Unmarshal(responseBody, &responsePayload); err != nil {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("decode webhook payout response: %w", err)
	}
	providerPayoutID := strings.TrimSpace(responsePayload.ProviderPayoutID)
	if providerPayoutID == "" {
		providerPayoutID = strings.TrimSpace(responsePayload.ProviderPayoutIDSnake)
	}
	if providerPayoutID == "" {
		return MarketplacePayoutDispatchResult{}, fmt.Errorf("webhook payout response provider payout id is required")
	}
	return MarketplacePayoutDispatchResult{ProviderPayoutID: providerPayoutID}, nil
}

type webhookPayoutRequest struct {
	PayoutID                string   `json:"payoutID"`
	PublisherOrganizationID string   `json:"publisherOrganizationID"`
	PublisherUserID         string   `json:"publisherUserID"`
	Amount                  float64  `json:"amount"`
	Currency                string   `json:"currency"`
	SettlementIDs           []string `json:"settlementIDs"`
}

type webhookPayoutResponse struct {
	ProviderPayoutID      string `json:"providerPayoutID"`
	ProviderPayoutIDSnake string `json:"provider_payout_id"`
}

func normalizeWebhookPayoutSettlementIDs(settlementIDs []string) []string {
	normalized := make([]string, 0, len(settlementIDs))
	for _, settlementID := range settlementIDs {
		settlementID = strings.TrimSpace(settlementID)
		if settlementID != "" {
			normalized = append(normalized, settlementID)
		}
	}
	return normalized
}

func signWebhookPayoutBody(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func validateWebhookPayoutEndpoint(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid webhook payout endpoint: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("webhook payout endpoint must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("webhook payout endpoint must include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("webhook payout endpoint must not include credentials")
	}
	return nil
}
