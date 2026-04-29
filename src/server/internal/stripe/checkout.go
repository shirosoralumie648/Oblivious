package stripe

import (
	"context"
	"fmt"
	"os"

	stripe "github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/checkout/session"
)

// CheckoutConfig holds Stripe configuration for checkout sessions.
type CheckoutConfig struct {
	SecretKey     string // STRIPE_SECRET_KEY env var
	SuccessURL    string // Frontend success redirect URL
	CancelURL     string // Frontend cancel redirect URL
	WebhookSecret string // STRIPE_WEBHOOK_SECRET env var
}

// CheckoutSessionRequest holds the data needed to create a checkout session.
type CheckoutSessionRequest struct {
	UserID       string // ID of the subscribing user
	PlanID       string // ID of the plan being purchased
	PlanName     string // Display name of the plan
	PlanPrice    float64 // Plan price in USD (converted to cents)
	DurationDays *int    // If set, creates a recurring subscription
}

// NewCheckoutConfigFromEnv creates a CheckoutConfig from environment variables.
func NewCheckoutConfigFromEnv() CheckoutConfig {
	return CheckoutConfig{
		SecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		SuccessURL:    os.Getenv("STRIPE_SUCCESS_URL"),
		CancelURL:     os.Getenv("STRIPE_CANCEL_URL"),
		WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
	}
}

// CreateCheckoutSession creates a Stripe checkout session for the requested plan.
// Per D-15: metadata includes user_id and plan_id for webhook-driven subscription updates.
// Price is converted from dollars to cents (Stripe uses smallest currency unit).
// If DurationDays is set, creates a subscription-mode session with recurring billing.
func CreateCheckoutSession(ctx context.Context, cfg CheckoutConfig, req CheckoutSessionRequest) (*stripe.CheckoutSession, error) {
	stripe.Key = cfg.SecretKey

	unitAmount := int64(req.PlanPrice * 100) // dollars to cents

	mode := stripe.CheckoutSessionModePayment
	var recurring *stripe.CheckoutSessionLineItemPriceDataRecurringParams

	if req.DurationDays != nil && *req.DurationDays > 0 {
		mode = stripe.CheckoutSessionModeSubscription
		recurring = &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
			Interval: stripe.String("month"),
		}
	}

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: []*string{
			stripe.String("card"),
		},
		Mode: stripe.String(string(mode)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(req.PlanName),
					},
					UnitAmount: stripe.Int64(unitAmount),
					Recurring:  recurring,
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(cfg.SuccessURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(cfg.CancelURL),
		Metadata: map[string]string{
			"user_id": req.UserID,
			"plan_id": req.PlanID,
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("create checkout session: %w", err)
	}

	return sess, nil
}
