package http

import (
	"encoding/json"
	"fmt"
	stdhttp "net/http"

	"github.com/google/uuid"

	"oblivious/server/internal/quota"
	stripebilling "oblivious/server/internal/stripe"
)

type billingHandler struct {
	checkoutCreator stripebilling.CheckoutCreator
	checkoutConfig  stripebilling.CheckoutConfig
	paymentStore    stripebilling.PaymentIntentStore
	quotaService    *quota.Service
}

type billingCheckoutRequest struct {
	PackageID string `json:"packageId"`
	Kind      string `json:"kind"`
}

type billingCheckoutResponse struct {
	CheckoutSessionID string `json:"checkoutSessionId"`
	URL               string `json:"url"`
}

func newBillingHandler(checkoutCreator stripebilling.CheckoutCreator, checkoutConfig stripebilling.CheckoutConfig, paymentStore stripebilling.PaymentIntentStore, quotaService *quota.Service) billingHandler {
	return billingHandler{
		checkoutCreator: checkoutCreator,
		checkoutConfig:  checkoutConfig,
		paymentStore:    paymentStore,
		quotaService:    quotaService,
	}
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
	if req.PackageID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "packageId is required")
		return
	}
	kind := req.Kind
	if kind == "" {
		kind = "subscription"
	}
	if kind != "subscription" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "only subscription checkout is supported in phase 17")
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

	paymentIntentID := uuid.New().String()
	checkoutReq := stripebilling.CheckoutSessionRequest{
		OrganizationID:  session.OrganizationID,
		UserID:          session.User.ID,
		PaymentIntentID: paymentIntentID,
		PlanID:          pkg.ID,
		PlanName:        pkg.Name,
		PlanPrice:       pkg.Price,
		Currency:        "usd",
		CheckoutKind:    kind,
		DurationDays:    pkg.DurationDays,
	}
	durationDays := ""
	if pkg.DurationDays != nil {
		durationDays = fmt.Sprintf("%d", *pkg.DurationDays)
	}
	if _, err := h.paymentStore.CreatePaymentIntent(r.Context(), stripebilling.PaymentIntent{
		ID:             paymentIntentID,
		Provider:       "stripe",
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		PackageID:      pkg.ID,
		Kind:           kind,
		Amount:         pkg.Price,
		Currency:       "usd",
		Status:         "pending",
		Metadata: map[string]string{
			"package_name":  pkg.Name,
			"duration_days": durationDays,
		},
	}); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record payment intent failed")
		return
	}

	checkoutSession, err := h.checkoutCreator.CreateCheckoutSession(r.Context(), h.checkoutConfig, checkoutReq)
	if err != nil {
		writeError(w, stdhttp.StatusBadGateway, "checkout_create_failed", "create checkout session failed")
		return
	}

	if err := h.paymentStore.SetCheckoutSession(r.Context(), paymentIntentID, checkoutSession.ID, map[string]string{
		"checkout_session_id": checkoutSession.ID,
		"package_name":        pkg.Name,
		"duration_days":       durationDays,
	}); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record checkout session failed")
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, billingCheckoutResponse{
		CheckoutSessionID: checkoutSession.ID,
		URL:               checkoutSession.URL,
	})
}
