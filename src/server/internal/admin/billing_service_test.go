package admin

import (
	"strings"
	"testing"
)

func TestValidateTopupRefundRequestRejectsIncompleteOperatorEvidence(t *testing.T) {
	cases := []struct {
		name    string
		request TopupRefundRequest
		want    string
	}{
		{
			name: "stripe charge or payment intent",
			request: TopupRefundRequest{
				Provider:                "stripe",
				ProviderRefundID:        "re_1",
				ProviderPaymentIntentID: "",
				Amount:                  10,
				Currency:                "usd",
			},
			want: "providerChargeID or providerPaymentIntentID is required for stripe refunds",
		},
		{
			name: "alipay payment intent",
			request: TopupRefundRequest{
				Provider:         "alipay",
				ProviderRefundID: "alipay_refund_1",
				Amount:           10,
				Currency:         "cny",
			},
			want: "providerPaymentIntentID is required for domestic provider refunds",
		},
		{
			name: "wechatpay payment intent",
			request: TopupRefundRequest{
				Provider:         "wechatpay",
				ProviderRefundID: "wechatpay_refund_1",
				Amount:           10,
				Currency:         "cny",
			},
			want: "providerPaymentIntentID is required for domestic provider refunds",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTopupRefundRequest(tt.request)
			if err == nil {
				t.Fatal("expected missing operator evidence to be rejected")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateTopupRefundRequestAcceptsDomesticOperatorEvidence(t *testing.T) {
	cases := []TopupRefundRequest{
		{
			Provider:                "alipay",
			ProviderRefundID:        "alipay_refund_1",
			ProviderPaymentIntentID: "alipay_payment_1",
			Amount:                  10,
			Currency:                "cny",
		},
		{
			Provider:                "wechatpay",
			ProviderRefundID:        "wechatpay_refund_1",
			ProviderPaymentIntentID: "wechatpay_payment_1",
			Amount:                  10,
			Currency:                "cny",
		},
	}

	for _, request := range cases {
		t.Run(request.Provider, func(t *testing.T) {
			if err := ValidateTopupRefundRequest(request); err != nil {
				t.Fatalf("expected domestic refund evidence to be accepted: %v", err)
			}
		})
	}
}
