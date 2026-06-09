package admin

import (
	"strings"
	"testing"
)

func TestTopupSummaryQueryUsesPaymentIntentProviderFilter(t *testing.T) {
	query, args := topupSummaryQuery(normalizeBillingFilter(BillingInspectionFilter{
		OrganizationID: " org_1 ",
		Status:         " paid ",
		Provider:       " stripe ",
	}))

	requiredFragments := []string{
		"FROM topup_orders",
		"LEFT JOIN payment_intents ON payment_intents.id = topup_orders.payment_intent_id",
		"topup_orders.organization_id = $1",
		"topup_orders.status = $2",
		"payment_intents.provider = $3",
		"CASE WHEN topup_orders.status = 'paid' THEN topup_orders.money ELSE 0 END",
		"SUM(topup_orders.refunded_amount)",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(query, fragment) {
			t.Fatalf("expected topup summary query to contain %q, got %s", fragment, query)
		}
	}
	if len(args) != 3 || args[0] != "org_1" || args[1] != "paid" || args[2] != "stripe" {
		t.Fatalf("expected normalized topup summary args [org_1 paid stripe], got %#v", args)
	}
}

func TestRecordTopupRefundUpdatesOrderStatusAndRefundedAmount(t *testing.T) {
	requiredFragments := []string{
		"UPDATE topup_orders",
		"SET status = $2, refunded_amount = $3",
		"WHERE id = $1",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(recordTopupRefundUpdateTopupOrderSQL, fragment) {
			t.Fatalf("expected topup refund update SQL to contain %q, got %s", fragment, recordTopupRefundUpdateTopupOrderSQL)
		}
	}
}
