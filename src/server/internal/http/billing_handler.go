package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"

	"github.com/google/uuid"

	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/payment"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/releasecontract"
	stripebilling "oblivious/server/internal/stripe"
)

type billingHandler struct {
	checkoutCreators map[string]stripebilling.CheckoutCreator
	checkoutConfig   stripebilling.CheckoutConfig
	paymentStore     stripebilling.PaymentIntentStore
	quotaService     *quota.Service
	providerRegistry *payment.Registry
	readiness        *billingFinancialReadiness
}

type billingFinancialReadiness struct {
	guard         releasecontract.Guard
	checkout      releasecontract.CapabilityID
	configuration error
}

func newBillingFinancialReadiness(financial marketplace.FinancialReadiness) (*billingFinancialReadiness, error) {
	return newCheckoutFinancialReadiness(financial, "http.billing.checkout", "http.billingHandler")
}

func newCheckoutFinancialReadiness(financial marketplace.FinancialReadiness, descriptorID, owner string) (*billingFinancialReadiness, error) {
	if financial.Guard == nil || financial.Effects == nil || !financial.Authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "http.billing"}
	}
	checkout, err := financial.Authorities.CapabilityBindings.Resolve(releasecontract.EffectBillingCheckout)
	if err != nil {
		return nil, err
	}
	if err := financial.Effects.Register(releasecontract.EffectDescriptor{
		ID: descriptorID, CapabilityID: string(checkout),
		Boundary: releasecontract.BoundaryFinancial, Owner: owner,
	}); err != nil {
		return nil, err
	}
	return &billingFinancialReadiness{guard: financial.Guard, checkout: checkout}, nil
}

func (r *billingFinancialReadiness) require(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if r.configuration != nil {
		return r.configuration
	}
	return r.guard.Require(ctx, string(r.checkout), releasecontract.BoundaryFinancial)
}

type billingCheckoutRequest struct {
	PackageID string  `json:"packageId"`
	Kind      string  `json:"kind"`
	Amount    float64 `json:"amount"`
	Provider  string  `json:"provider"`
}

type billingCheckoutResponse struct {
	CheckoutSessionID string `json:"checkoutSessionId"`
	URL               string `json:"url"`
}

func newBillingHandler(checkoutCreator stripebilling.CheckoutCreator, checkoutConfig stripebilling.CheckoutConfig, paymentStore stripebilling.PaymentIntentStore, quotaService *quota.Service, providerRegistry *payment.Registry, checkoutCreators map[string]stripebilling.CheckoutCreator) billingHandler {
	if providerRegistry == nil {
		providerRegistry = payment.DefaultRegistry()
	}
	if checkoutCreators == nil {
		checkoutCreators = map[string]stripebilling.CheckoutCreator{}
	}
	if checkoutCreator != nil && checkoutCreators["stripe"] == nil {
		checkoutCreators["stripe"] = checkoutCreator
	}
	return billingHandler{
		checkoutCreators: checkoutCreators,
		checkoutConfig:   checkoutConfig,
		paymentStore:     paymentStore,
		quotaService:     quotaService,
		providerRegistry: providerRegistry,
	}
}

// newBillingHandlerWithReadiness is the production constructor. The legacy
// constructor remains for isolated tests that do not exercise financial flow.
func newBillingHandlerWithReadiness(checkoutCreator stripebilling.CheckoutCreator, checkoutConfig stripebilling.CheckoutConfig, paymentStore stripebilling.PaymentIntentStore, quotaService *quota.Service, providerRegistry *payment.Registry, checkoutCreators map[string]stripebilling.CheckoutCreator, financial marketplace.FinancialReadiness) (billingHandler, error) {
	handler := newBillingHandler(checkoutCreator, checkoutConfig, paymentStore, quotaService, providerRegistry, checkoutCreators)
	readiness, err := newBillingFinancialReadiness(financial)
	if err != nil {
		return billingHandler{}, err
	}
	handler.readiness = readiness
	return handler, nil
}

func newBillingHandlerWithFinancialReadiness(checkoutCreator stripebilling.CheckoutCreator, checkoutConfig stripebilling.CheckoutConfig, paymentStore stripebilling.PaymentIntentStore, quotaService *quota.Service, providerRegistry *payment.Registry, checkoutCreators map[string]stripebilling.CheckoutCreator, financial marketplace.FinancialReadiness) (billingHandler, error) {
	return newBillingHandlerWithReadiness(checkoutCreator, checkoutConfig, paymentStore, quotaService, providerRegistry, checkoutCreators, financial)
}

func (h billingHandler) checkout(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if session.OrganizationID == "" {
		writeError(w, stdhttp.StatusBadRequest, "organization_required", "active organization is required")
		return
	}

	var req billingCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "subscription"
	}
	if kind != "subscription" && kind != "topup" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "kind must be subscription or topup")
		return
	}
	provider, err := h.providerRegistry.Resolve(req.Provider)
	if err != nil {
		var providerErr *payment.ProviderError
		if errors.As(err, &providerErr) {
			switch providerErr.Code {
			case payment.CodeProviderNotConfigured:
				writeError(w, stdhttp.StatusNotImplemented, providerErr.Code, "payment provider is not configured")
			case payment.CodeUnsupportedProvider:
				writeError(w, stdhttp.StatusBadRequest, providerErr.Code, "payment provider is not supported")
			default:
				writeError(w, stdhttp.StatusBadRequest, "invalid_provider", "payment provider is invalid")
			}
			return
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_provider", "payment provider is invalid")
		return
	}
	checkoutCreator := h.checkoutCreators[provider.Name]
	if checkoutCreator == nil {
		writeError(w, stdhttp.StatusNotImplemented, payment.CodeProviderNotConfigured, "payment provider checkout is not configured")
		return
	}
	if err := h.readiness.require(r.Context()); err != nil {
		writeBillingReadinessError(w, err)
		return
	}

	paymentIntentID := uuid.New().String()
	currency := provider.Currency
	if currency == "" {
		currency = "usd"
	}
	checkoutReq := stripebilling.CheckoutSessionRequest{
		OrganizationID:  session.OrganizationID,
		UserID:          session.User.ID,
		PaymentIntentID: paymentIntentID,
		Currency:        currency,
		CheckoutKind:    kind,
	}
	var intent stripebilling.PaymentIntent
	durationDays := ""

	if kind == "subscription" {
		if req.PackageID == "" {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "packageId is required")
			return
		}
		pkg, err := h.quotaService.GetPackage(r.Context(), req.PackageID)
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get package failed")
			return
		}
		if pkg == nil || !pkg.IsActive {
			writeError(w, stdhttp.StatusNotFound, "package_not_found", "package not found")
			return
		}
		checkoutReq.PlanID = pkg.ID
		checkoutReq.PlanName = pkg.Name
		checkoutReq.PlanPrice = pkg.Price
		checkoutReq.DurationDays = pkg.DurationDays
		if pkg.DurationDays != nil {
			durationDays = fmt.Sprintf("%d", *pkg.DurationDays)
		}
		intent = stripebilling.PaymentIntent{
			ID:             paymentIntentID,
			Provider:       provider.Name,
			OrganizationID: session.OrganizationID,
			UserID:         session.User.ID,
			PackageID:      pkg.ID,
			Kind:           kind,
			Amount:         pkg.Price,
			Currency:       currency,
			Status:         "pending",
			Metadata: map[string]string{
				"package_name":  pkg.Name,
				"duration_days": durationDays,
			},
		}
	} else {
		if req.Amount <= 0 {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "amount must be positive")
			return
		}
		checkoutReq.PlanName = "Quota top-up"
		checkoutReq.PlanPrice = req.Amount
		intent = stripebilling.PaymentIntent{
			ID:             paymentIntentID,
			Provider:       provider.Name,
			OrganizationID: session.OrganizationID,
			UserID:         session.User.ID,
			Kind:           kind,
			Amount:         req.Amount,
			Currency:       currency,
			Status:         "pending",
			Metadata: map[string]string{
				"topup_amount": fmt.Sprintf("%.6f", req.Amount),
			},
		}
	}

	if _, err := h.paymentStore.CreatePaymentIntent(r.Context(), intent); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record payment intent failed")
		return
	}
	if kind == "topup" {
		if _, err := h.quotaService.CreatePendingTopup(r.Context(), session.User.ID, session.OrganizationID, paymentIntentID, req.Amount, req.Amount); err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record topup order failed")
			return
		}
	}

	if err := h.readiness.require(r.Context()); err != nil {
		writeBillingReadinessError(w, err)
		return
	}
	checkoutSession, err := checkoutCreator.CreateCheckoutSession(r.Context(), h.checkoutConfig, checkoutReq)
	if err != nil {
		if failErr := h.markCheckoutCreationFailed(r.Context(), session.OrganizationID, paymentIntentID, kind, err.Error()); failErr != nil {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "mark checkout failed state failed")
			return
		}
		writeError(w, stdhttp.StatusBadGateway, "checkout_create_failed", "create checkout session failed")
		return
	}

	metadata := map[string]string{
		"checkout_session_id": checkoutSession.ID,
	}
	if kind == "subscription" {
		metadata["duration_days"] = durationDays
		metadata["package_name"] = intent.Metadata["package_name"]
	} else {
		metadata["topup_amount"] = intent.Metadata["topup_amount"]
	}
	if err := h.paymentStore.SetCheckoutSession(r.Context(), paymentIntentID, checkoutSession.ID, metadata); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record checkout session failed")
		return
	}
	if kind == "topup" {
		if err := h.quotaService.SetTopupCheckoutSession(r.Context(), session.OrganizationID, paymentIntentID, checkoutSession.ID); err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record topup checkout session failed")
			return
		}
	}

	writeSuccess(w, stdhttp.StatusCreated, billingCheckoutResponse{
		CheckoutSessionID: checkoutSession.ID,
		URL:               checkoutSession.URL,
	})
}

func writeBillingReadinessError(w stdhttp.ResponseWriter, err error) {
	var readinessErr *releasecontract.ReadinessError
	if errors.As(err, &readinessErr) {
		writeError(w, stdhttp.StatusServiceUnavailable, string(readinessErr.Code), "financial operation is not available")
		return
	}
	writeError(w, stdhttp.StatusServiceUnavailable, "readiness_unavailable", "financial operation is not available")
}

func (h billingHandler) markCheckoutCreationFailed(ctx context.Context, organizationID string, paymentIntentID string, kind string, reason string) error {
	if err := h.paymentStore.MarkPaymentIntentFailed(ctx, paymentIntentID, reason); err != nil {
		return err
	}
	if kind == "topup" {
		if err := h.quotaService.MarkTopupCheckoutFailed(ctx, organizationID, paymentIntentID); err != nil {
			return err
		}
	}
	return nil
}
