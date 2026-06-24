package admin

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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

func TestMarketplaceSettlementQueriesUsePaymentIntentProviderFilter(t *testing.T) {
	filter := normalizeBillingFilter(BillingInspectionFilter{
		OrganizationID: " org_1 ",
		UserID:         " user_1 ",
		Status:         " payout_pending ",
		Kind:           " marketplace_install ",
		Provider:       " stripe ",
		Limit:          25,
		Offset:         5,
	})
	summarySQL, summaryArgs := marketplaceSettlementSummaryQuery(filter)
	countSQL, countArgs := marketplaceSettlementCountQuery(filter)
	listSQL, listArgs := marketplaceSettlementListQuery(filter)
	queries := []struct {
		name string
		sql  string
		args []any
	}{
		{"summary", summarySQL, summaryArgs},
		{"count", countSQL, countArgs},
		{"list", listSQL, listArgs},
	}
	for _, tt := range queries {
		t.Run(tt.name, func(t *testing.T) {
			requiredFragments := []string{
				"FROM marketplace_settlements",
				"JOIN marketplace_orders ON marketplace_orders.id = marketplace_settlements.order_id",
				"JOIN payment_intents ON payment_intents.id = marketplace_orders.payment_intent_id",
				"marketplace_settlements.publisher_organization_id = $1",
				"marketplace_settlements.publisher_user_id = $2",
				"marketplace_settlements.status = $3",
				"payment_intents.kind = $4",
				"payment_intents.provider = $5",
			}
			for _, fragment := range requiredFragments {
				if !strings.Contains(tt.sql, fragment) {
					t.Fatalf("expected %s settlement query to contain %q, got %s", tt.name, fragment, tt.sql)
				}
			}
			if tt.name == "list" {
				if !strings.Contains(tt.sql, "ORDER BY marketplace_settlements.created_at DESC") ||
					!strings.Contains(tt.sql, "LIMIT $6 OFFSET $7") {
					t.Fatalf("expected list settlement query to order and paginate after filters, got %s", tt.sql)
				}
				if len(tt.args) != 7 || tt.args[5] != 25 || tt.args[6] != 5 {
					t.Fatalf("expected list args to include limit/offset after filter args, got %#v", tt.args)
				}
			} else if len(tt.args) != 5 {
				t.Fatalf("expected %s args to include only filter args, got %#v", tt.name, tt.args)
			}
			want := []any{"org_1", "user_1", "payout_pending", "marketplace_install", "stripe"}
			for i, value := range want {
				if tt.args[i] != value {
					t.Fatalf("expected %s arg %d to be %q, got %#v", tt.name, i, value, tt.args)
				}
			}
		})
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

func TestRecordTopupRefundRejectsConflictingProviderRefundEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, organization_id, user_id, COALESCE(payment_intent_id, ''), COALESCE(provider_checkout_session_id, ''),
		       amount, money, status, COALESCE(trade_no, ''), refunded_amount, paid_at, created_at
		FROM topup_orders
		WHERE id = $1
		FOR UPDATE
	`)).WithArgs("topup_1").WillReturnRows(sqlmock.NewRows([]string{
		"id", "organization_id", "user_id", "payment_intent_id", "provider_checkout_session_id",
		"amount", "money", "status", "trade_no", "refunded_amount", "paid_at", "created_at",
	}).AddRow("topup_1", "org_1", "user_1", "pi_1", "cs_1", 25.0, 25.0, "partially_refunded", "trade_1", 10.0, time.Now(), time.Now()))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT kind, status, amount, currency, refunded_amount, COALESCE(provider_payment_intent_id, '')
		FROM payment_intents
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
		FOR UPDATE
	`)).WithArgs("pi_1", "org_1", "user_1").WillReturnRows(sqlmock.NewRows([]string{
		"kind", "status", "amount", "currency", "refunded_amount", "provider_payment_intent_id",
	}).AddRow("topup", "partially_refunded", 25.0, "usd", 10.0, "pi_provider_1"))
	mock.ExpectExec("INSERT INTO billing_lifecycle_events").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, provider, provider_refund_id, COALESCE(provider_charge_id, ''), COALESCE(provider_payment_intent_id, ''),
		       organization_id, user_id, payment_intent_id, topup_order_id, amount, currency, status, COALESCE(reason, ''),
		       created_at, updated_at
		FROM billing_refunds
		WHERE provider = $1 AND provider_refund_id = $2
	`)).WithArgs("stripe", "reused_refund_1").WillReturnRows(sqlmock.NewRows([]string{
		"id", "provider", "provider_refund_id", "provider_charge_id", "provider_payment_intent_id",
		"organization_id", "user_id", "payment_intent_id", "topup_order_id", "amount", "currency", "status", "reason",
		"created_at", "updated_at",
	}).AddRow("refund_1", "stripe", "reused_refund_1", "ch_1", "pi_provider_1", "org_1", "user_1", "pi_1", "topup_1", 10.0, "usd", "succeeded", "duplicate charge", time.Now(), time.Now()))
	mock.ExpectRollback()

	_, err = NewSQLStore(db).RecordTopupRefund(context.Background(), "topup_1", TopupRefundRequest{
		Provider:                "stripe",
		ProviderRefundID:        "reused_refund_1",
		ProviderChargeID:        "ch_1",
		ProviderPaymentIntentID: "pi_provider_1",
		Amount:                  11,
		Currency:                "usd",
		Reason:                  "duplicate charge",
	})
	if !errors.Is(err, ErrTopupRefundIdempotencyConflict) {
		t.Fatalf("expected refund idempotency conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRecordTopupRefundReusesExistingRefundWithSameEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, organization_id, user_id, COALESCE(payment_intent_id, ''), COALESCE(provider_checkout_session_id, ''),
		       amount, money, status, COALESCE(trade_no, ''), refunded_amount, paid_at, created_at
		FROM topup_orders
		WHERE id = $1
		FOR UPDATE
	`)).WithArgs("topup_1").WillReturnRows(sqlmock.NewRows([]string{
		"id", "organization_id", "user_id", "payment_intent_id", "provider_checkout_session_id",
		"amount", "money", "status", "trade_no", "refunded_amount", "paid_at", "created_at",
	}).AddRow("topup_1", "org_1", "user_1", "pi_1", "cs_1", 25.0, 25.0, "partially_refunded", "trade_1", 10.0, time.Now(), time.Now()))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT kind, status, amount, currency, refunded_amount, COALESCE(provider_payment_intent_id, '')
		FROM payment_intents
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
		FOR UPDATE
	`)).WithArgs("pi_1", "org_1", "user_1").WillReturnRows(sqlmock.NewRows([]string{
		"kind", "status", "amount", "currency", "refunded_amount", "provider_payment_intent_id",
	}).AddRow("topup", "partially_refunded", 25.0, "usd", 10.0, "pi_provider_1"))
	mock.ExpectExec("INSERT INTO billing_lifecycle_events").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, provider, provider_refund_id, COALESCE(provider_charge_id, ''), COALESCE(provider_payment_intent_id, ''),
		       organization_id, user_id, payment_intent_id, topup_order_id, amount, currency, status, COALESCE(reason, ''),
		       created_at, updated_at
		FROM billing_refunds
		WHERE provider = $1 AND provider_refund_id = $2
	`)).WithArgs("stripe", "reused_refund_1").WillReturnRows(sqlmock.NewRows([]string{
		"id", "provider", "provider_refund_id", "provider_charge_id", "provider_payment_intent_id",
		"organization_id", "user_id", "payment_intent_id", "topup_order_id", "amount", "currency", "status", "reason",
		"created_at", "updated_at",
	}).AddRow("refund_1", "stripe", "reused_refund_1", "ch_1", "pi_provider_1", "org_1", "user_1", "pi_1", "topup_1", 11.0, "usd", "succeeded", "duplicate charge", time.Now(), time.Now()))
	mock.ExpectCommit()

	refund, err := NewSQLStore(db).RecordTopupRefund(context.Background(), "topup_1", TopupRefundRequest{
		Provider:                "stripe",
		ProviderRefundID:        "reused_refund_1",
		ProviderChargeID:        "ch_1",
		ProviderPaymentIntentID: "pi_provider_1",
		Amount:                  11,
		Currency:                "usd",
		Reason:                  "duplicate charge",
	})
	if err != nil {
		t.Fatalf("expected idempotent retry to reuse existing refund, got %v", err)
	}
	if refund == nil || refund.ID != "refund_1" || refund.Amount != 11 {
		t.Fatalf("expected existing refund row, got %+v", refund)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
