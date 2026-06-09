package admin

import (
	"strings"
	"testing"
)

func TestValidateTopupRefundRequestRejectsIncompleteOperatorEvidence(t *testing.T) {
	err := ValidateTopupRefundRequest(TopupRefundRequest{
		Provider:                "stripe",
		ProviderRefundID:        "re_1",
		ProviderPaymentIntentID: "",
		Amount:                  10,
		Currency:                "usd",
	})
	if err == nil {
		t.Fatal("expected missing stripe provider charge/payment intent evidence to be rejected")
	}
	if !strings.Contains(err.Error(), "providerChargeID or providerPaymentIntentID is required for stripe refunds") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
