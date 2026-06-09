package admin

import (
	"context"
	"fmt"
	"strings"
)

// GetBillingInspectionSummary returns aggregate money-movement evidence for Admin billing inspection.
func (s *Service) GetBillingInspectionSummary(ctx context.Context, filter BillingInspectionFilter) (*BillingInspectionSummary, error) {
	return s.store.GetBillingInspectionSummary(ctx, filter)
}

func (s *Service) ListBillingSessions(ctx context.Context, filter BillingInspectionFilter) ([]*BillingSessionInspection, int, error) {
	return s.store.ListBillingSessions(ctx, filter)
}

func (s *Service) ListPaymentIntents(ctx context.Context, filter BillingInspectionFilter) ([]*PaymentIntentInspection, int, error) {
	return s.store.ListPaymentIntents(ctx, filter)
}

func (s *Service) ListWebhookEvents(ctx context.Context, filter BillingInspectionFilter) ([]*WebhookEventInspection, int, error) {
	return s.store.ListWebhookEvents(ctx, filter)
}

func (s *Service) ListSubscriptions(ctx context.Context, filter BillingInspectionFilter) ([]*SubscriptionInspection, int, error) {
	return s.store.ListSubscriptions(ctx, filter)
}

func (s *Service) ListTopups(ctx context.Context, filter BillingInspectionFilter) ([]*TopupInspection, int, error) {
	return s.store.ListTopups(ctx, filter)
}

func (s *Service) ListInvoices(ctx context.Context, filter BillingInspectionFilter) ([]*InvoiceInspection, int, error) {
	return s.store.ListInvoices(ctx, filter)
}

func (s *Service) ListRefunds(ctx context.Context, filter BillingInspectionFilter) ([]*RefundInspection, int, error) {
	return s.store.ListRefunds(ctx, filter)
}

func (s *Service) RecordTopupRefund(ctx context.Context, topupID string, request TopupRefundRequest) (*RefundInspection, error) {
	if err := ValidateTopupRefundRequest(request); err != nil {
		return nil, err
	}
	return s.store.RecordTopupRefund(ctx, topupID, request)
}

func ValidateTopupRefundRequest(request TopupRefundRequest) error {
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider == "" {
		return fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(request.ProviderRefundID) == "" {
		return fmt.Errorf("providerRefundID is required")
	}
	if request.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if strings.TrimSpace(request.Currency) == "" {
		return fmt.Errorf("currency is required")
	}
	if provider == "stripe" && strings.TrimSpace(request.ProviderChargeID) == "" && strings.TrimSpace(request.ProviderPaymentIntentID) == "" {
		return fmt.Errorf("providerChargeID or providerPaymentIntentID is required for stripe refunds")
	}
	if (provider == "alipay" || provider == "wechatpay") && strings.TrimSpace(request.ProviderPaymentIntentID) == "" {
		return fmt.Errorf("providerPaymentIntentID is required for domestic provider refunds")
	}
	return nil
}

func (s *Service) ListMarketplaceSettlements(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplaceSettlementInspection, int, error) {
	return s.store.ListMarketplaceSettlements(ctx, filter)
}

func (s *Service) ListMarketplacePayouts(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplacePayoutInspection, int, error) {
	return s.store.ListMarketplacePayouts(ctx, filter)
}
