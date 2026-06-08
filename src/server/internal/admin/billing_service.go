package admin

import "context"

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
	return s.store.RecordTopupRefund(ctx, topupID, request)
}

func (s *Service) ListMarketplaceSettlements(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplaceSettlementInspection, int, error) {
	return s.store.ListMarketplaceSettlements(ctx, filter)
}

func (s *Service) ListMarketplacePayouts(ctx context.Context, filter BillingInspectionFilter) ([]*MarketplacePayoutInspection, int, error) {
	return s.store.ListMarketplacePayouts(ctx, filter)
}
