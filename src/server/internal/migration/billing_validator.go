package migration

import (
	"context"
	"database/sql"
	"fmt"
)

type BillingValidator struct {
	db *sql.DB
}

func NewBillingValidator(db *sql.DB) *BillingValidator {
	return &BillingValidator{db: db}
}

func (v *BillingValidator) Validate(ctx context.Context) error {
	if err := v.validateSubscriptions(ctx); err != nil {
		return fmt.Errorf("subscriptions: %w", err)
	}
	if err := v.validateInvoices(ctx); err != nil {
		return fmt.Errorf("invoices: %w", err)
	}
	if err := v.validatePayments(ctx); err != nil {
		return fmt.Errorf("payments: %w", err)
	}
	return nil
}

func (v *BillingValidator) validateSubscriptions(ctx context.Context) error {
	const query = `
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN user_id IS NULL THEN 1 END) as null_user,
			COUNT(CASE WHEN plan_id IS NULL THEN 1 END) as null_plan,
			COUNT(CASE WHEN status NOT IN ('active', 'cancelled', 'expired') THEN 1 END) as invalid_status
		FROM subscriptions`

	var total, nullUser, nullPlan, invalidStatus int
	if err := v.db.QueryRowContext(ctx, query).Scan(&total, &nullUser, &nullPlan, &invalidStatus); err != nil {
		return err
	}

	if nullUser > 0 {
		return fmt.Errorf("%d rows with null user_id", nullUser)
	}
	if nullPlan > 0 {
		return fmt.Errorf("%d rows with null plan_id", nullPlan)
	}
	if invalidStatus > 0 {
		return fmt.Errorf("%d rows with invalid status", invalidStatus)
	}

	return nil
}

func (v *BillingValidator) validateInvoices(ctx context.Context) error {
	const query = `
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN subscription_id IS NULL THEN 1 END) as null_subscription,
			COUNT(CASE WHEN amount < 0 THEN 1 END) as negative_amount,
			COUNT(CASE WHEN status NOT IN ('draft', 'paid', 'void', 'uncollectible') THEN 1 END) as invalid_status
		FROM invoices`

	var total, nullSubscription, negativeAmount, invalidStatus int
	if err := v.db.QueryRowContext(ctx, query).Scan(&total, &nullSubscription, &negativeAmount, &invalidStatus); err != nil {
		return err
	}

	if nullSubscription > 0 {
		return fmt.Errorf("%d rows with null subscription_id", nullSubscription)
	}
	if negativeAmount > 0 {
		return fmt.Errorf("%d rows with negative amount", negativeAmount)
	}
	if invalidStatus > 0 {
		return fmt.Errorf("%d rows with invalid status", invalidStatus)
	}

	return nil
}

func (v *BillingValidator) validatePayments(ctx context.Context) error {
	const query = `
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN invoice_id IS NULL THEN 1 END) as null_invoice,
			COUNT(CASE WHEN amount <= 0 THEN 1 END) as invalid_amount,
			COUNT(CASE WHEN status NOT IN ('succeeded', 'failed', 'pending') THEN 1 END) as invalid_status
		FROM payments`

	var total, nullInvoice, invalidAmount, invalidStatus int
	if err := v.db.QueryRowContext(ctx, query).Scan(&total, &nullInvoice, &invalidAmount, &invalidStatus); err != nil {
		return err
	}

	if nullInvoice > 0 {
		return fmt.Errorf("%d rows with null invoice_id", nullInvoice)
	}
	if invalidAmount > 0 {
		return fmt.Errorf("%d rows with invalid amount", invalidAmount)
	}
	if invalidStatus > 0 {
		return fmt.Errorf("%d rows with invalid status", invalidStatus)
	}

	return nil
}
