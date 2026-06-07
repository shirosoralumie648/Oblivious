package http

import (
	"context"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/payment"
	stripebilling "oblivious/server/internal/stripe"
)

type marketplaceSettlementCheckoutService interface {
	CreatePaidInstallCheckout(ctx context.Context, input marketplace.PaidInstallCheckoutRequest) (*marketplace.MarketplaceOrder, error)
	SetPaidInstallCheckoutSession(ctx context.Context, orderID, paymentIntentID, providerCheckoutSessionID string) error
}

type marketplaceHandler struct {
	service           *marketplace.Service
	searchService     *marketplace.SearchService
	settlementService marketplaceSettlementCheckoutService
	governanceService *marketplace.GovernanceService
	checkoutCreators  map[string]stripebilling.CheckoutCreator
	checkoutConfig    stripebilling.CheckoutConfig
	providerRegistry  *payment.Registry
}

type marketplaceHandlerOption func(*marketplaceHandler)

type marketplacePaymentProviderResponse struct {
	Name string `json:"name"`
}

func newMarketplaceHandler(service *marketplace.Service, searchService *marketplace.SearchService, options ...marketplaceHandlerOption) marketplaceHandler {
	handler := marketplaceHandler{service: service, searchService: searchService}
	for _, option := range options {
		option(&handler)
	}
	return handler
}

func withMarketplaceCheckout(settlementService marketplaceSettlementCheckoutService, checkoutCreator stripebilling.CheckoutCreator, checkoutConfig stripebilling.CheckoutConfig, providerRegistry *payment.Registry, checkoutCreators map[string]stripebilling.CheckoutCreator) marketplaceHandlerOption {
	return func(handler *marketplaceHandler) {
		handler.settlementService = settlementService
		if providerRegistry == nil {
			providerRegistry = payment.DefaultRegistry()
		}
		creators := map[string]stripebilling.CheckoutCreator{}
		if checkoutCreator != nil {
			creators["stripe"] = checkoutCreator
		}
		for providerName, creator := range checkoutCreators {
			normalized := strings.ToLower(strings.TrimSpace(providerName))
			if normalized == "" || creator == nil {
				continue
			}
			creators[normalized] = creator
		}
		handler.checkoutCreators = creators
		handler.checkoutConfig = checkoutConfig
		handler.providerRegistry = providerRegistry
	}
}

func withMarketplaceGovernance(governanceService *marketplace.GovernanceService) marketplaceHandlerOption {
	return func(handler *marketplaceHandler) {
		handler.governanceService = governanceService
	}
}

func (h marketplaceHandler) listAgents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	h.searchAgents(w, r)
}

func (h marketplaceHandler) searchAgents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.searchService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace search is not configured")
		return
	}

	agents, total, err := h.searchService.SearchAgents(r.Context(), marketplaceSearchFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"agents": agents,
		"total":  total,
	})
}

func (h marketplaceHandler) getAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	agent, err := h.service.GetAgent(r.Context(), agentID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if agent == nil || agent.Status != "approved" || agent.Visibility != "public" {
		writeError(w, stdhttp.StatusNotFound, "not_found", "agent not found")
		return
	}

	versions, _ := h.service.ListAgentVersions(r.Context(), agentID)
	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"agent":            agent,
		"versions":         versions,
		"paymentProviders": h.availablePaymentProviders(),
	})
}

func (h marketplaceHandler) availablePaymentProviders() []marketplacePaymentProviderResponse {
	providerRegistry := h.providerRegistry
	if providerRegistry == nil {
		providerRegistry = payment.DefaultRegistry()
	}
	providers := providerRegistry.AvailableProviders()
	if len(providers) == 0 || len(h.checkoutCreators) == 0 {
		return []marketplacePaymentProviderResponse{}
	}
	available := make([]marketplacePaymentProviderResponse, 0, len(providers))
	for _, provider := range providers {
		if h.checkoutCreators[provider.Name] != nil {
			available = append(available, marketplacePaymentProviderResponse{Name: provider.Name})
		}
	}
	return available
}

func (h marketplaceHandler) publishAgent(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req marketplace.AgentPublishRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	agent, err := h.service.PublishAgent(r.Context(), session.User.ID, session.OrganizationID, session.User.Email, req, requestClientIP(r))
	if err != nil {
		if writeMarketplaceAutomatedReviewError(w, err) {
			return
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, agent)
}

func (h marketplaceHandler) updateAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req marketplace.AgentPublishRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	agent, err := h.service.UpdateAgent(r.Context(), session.User.ID, session.OrganizationID, session.User.Email, agentID, req, requestClientIP(r))
	if err != nil {
		if writeMarketplaceAutomatedReviewError(w, err) {
			return
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, agent)
}

func writeMarketplaceAutomatedReviewError(w stdhttp.ResponseWriter, err error) bool {
	var reviewErr *marketplace.AutomatedReviewError
	if !errors.As(err, &reviewErr) {
		return false
	}
	writeJSON(w, stdhttp.StatusBadRequest, Envelope{
		OK: false,
		Data: map[string]any{
			"automatedReview": reviewErr.Result,
		},
		Error: &ErrorPayload{
			Code:    "automated_review_rejected",
			Message: reviewErr.Error(),
		},
	})
	return true
}

func (h marketplaceHandler) deleteAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.DeleteAgent(r.Context(), session.User.ID, session.OrganizationID, agentID); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h marketplaceHandler) installAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	versionID := strings.TrimSpace(r.URL.Query().Get("versionID"))
	providerName := strings.TrimSpace(r.URL.Query().Get("provider"))
	if (versionID == "" || providerName == "") && r.Body != nil {
		var req struct {
			VersionID string `json:"versionID"`
			Provider  string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if versionID == "" {
				versionID = strings.TrimSpace(req.VersionID)
			}
			if providerName == "" {
				providerName = strings.TrimSpace(req.Provider)
			}
		}
	}

	agent, err := h.service.GetAgent(r.Context(), agentID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if agent == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "agent not found")
		return
	}
	if agent.PricingType != "free" && agent.PricingAmount > 0 {
		h.createPaidInstallCheckout(w, r, session.User.ID, session.OrganizationID, agent, versionID, providerName)
		return
	}

	install, err := h.service.InstallAgent(r.Context(), session.User.ID, session.OrganizationID, agentID, versionID)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, install)
}

func (h marketplaceHandler) createPaidInstallCheckout(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string, organizationID string, agent *marketplace.PublishedAgent, versionID string, providerName string) {
	providerRegistry := h.providerRegistry
	if providerRegistry == nil {
		providerRegistry = payment.DefaultRegistry()
	}
	provider, err := providerRegistry.Resolve(providerName)
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
	if h.settlementService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace paid checkout is not configured")
		return
	}
	order, err := h.settlementService.CreatePaidInstallCheckout(r.Context(), marketplace.PaidInstallCheckoutRequest{
		BuyerOrganizationID: organizationID,
		BuyerUserID:         userID,
		AgentID:             agent.ID,
		VersionID:           versionID,
		Provider:            provider.Name,
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	checkoutSession, err := checkoutCreator.CreateCheckoutSession(r.Context(), h.checkoutConfig, stripebilling.CheckoutSessionRequest{
		OrganizationID:          organizationID,
		UserID:                  userID,
		PaymentIntentID:         order.PaymentIntentID,
		PlanID:                  agent.ID,
		PlanName:                agent.Name,
		PlanPrice:               order.GrossAmount,
		Currency:                order.Currency,
		CheckoutKind:            "marketplace_install",
		MarketplaceOrderID:      order.ID,
		AgentID:                 order.AgentID,
		VersionID:               order.VersionID,
		PublisherUserID:         order.PublisherUserID,
		PublisherOrganizationID: order.PublisherOrganizationID,
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadGateway, "checkout_create_failed", "create checkout session failed")
		return
	}
	if err := h.settlementService.SetPaidInstallCheckoutSession(r.Context(), order.ID, order.PaymentIntentID, checkoutSession.ID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record marketplace checkout session failed")
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, billingCheckoutResponse{
		CheckoutSessionID: checkoutSession.ID,
		URL:               checkoutSession.URL,
	})
}

func (h marketplaceHandler) uninstallAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.UninstallAgent(r.Context(), session.User.ID, session.OrganizationID, agentID); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "uninstalled"})
}

func (h marketplaceHandler) listInstalledAgents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	installs, err := h.service.ListUserInstalls(r.Context(), session.User.ID, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"installs": installs,
		"total":    len(installs),
	})
}

func (h marketplaceHandler) listMyAgents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	limit := parseQueryInt(r, "limit", 20, 100)
	offset := parseQueryInt(r, "offset", 0, 0)
	agents, err := h.service.ListUserAgents(r.Context(), session.User.ID, session.OrganizationID, limit, offset)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"agents": agents,
		"total":  len(agents),
	})
}

func (h marketplaceHandler) getFeaturedAgents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.searchService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace search is not configured")
		return
	}

	agents, total, err := h.searchService.SearchAgents(r.Context(), marketplace.MarketplaceSearchFilter{
		Sort:  "recommended",
		Limit: 6,
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"agents": agents,
		"total":  total,
	})
}

func (h marketplaceHandler) getCuratedSections(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.searchService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace search is not configured")
		return
	}

	popular, _, err := h.searchService.SearchAgents(r.Context(), marketplace.MarketplaceSearchFilter{Sort: "popular", Limit: 6})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	topRated, _, err := h.searchService.SearchAgents(r.Context(), marketplace.MarketplaceSearchFilter{Sort: "rating", Limit: 6})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	recent, _, err := h.searchService.SearchAgents(r.Context(), marketplace.MarketplaceSearchFilter{Sort: "newest", Limit: 6})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"popular":  popular,
		"topRated": topRated,
		"recent":   recent,
	})
}

func (h marketplaceHandler) listCategories(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	categories, err := h.service.ListCategories(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"categories": categories,
		"total":      len(categories),
	})
}

func (h marketplaceHandler) listTemplates(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	templates, total, err := h.service.ListTemplates(r.Context(), marketplaceTemplateFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"templates": templates,
		"total":     total,
	})
}

func (h marketplaceHandler) createTemplate(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req marketplace.TemplateCreateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	template, err := h.service.CreateTemplate(r.Context(), session.OrganizationID, req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, template)
}

func (h marketplaceHandler) getTemplate(w stdhttp.ResponseWriter, r *stdhttp.Request, templateID string) {
	template, err := h.service.GetTemplate(r.Context(), templateID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if template == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "template not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"template": template})
}

func (h marketplaceHandler) installTemplate(w stdhttp.ResponseWriter, r *stdhttp.Request, templateID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	install, err := h.service.InstallTemplate(r.Context(), session.User.ID, session.OrganizationID, templateID)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, install)
}

func (h marketplaceHandler) listReviews(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	reviews, err := h.service.ListReviews(r.Context(), agentID, parseQueryInt(r, "limit", 20, 50), parseQueryInt(r, "offset", 0, 0))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"reviews": reviews,
		"total":   len(reviews),
	})
}

func (h marketplaceHandler) submitReview(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req marketplace.ReviewInput
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	req.AgentID = agentID

	review, err := h.service.SubmitReview(r.Context(), session.User.ID, session.OrganizationID, session.User.Name, req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, review)
}

func (h marketplaceHandler) takedownAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if h.governanceService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace governance is not configured")
		return
	}
	if err := h.governanceService.TakedownAgent(r.Context(), marketplace.GovernanceAction{
		ActorUserID:         session.User.ID,
		ActorOrganizationID: session.OrganizationID,
		AgentID:             agentID,
		Reason:              req.Reason,
	}); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "takedown"})
}

func (h marketplaceHandler) appealAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if h.governanceService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace governance is not configured")
		return
	}
	if err := h.governanceService.AppealAgent(r.Context(), marketplace.GovernanceAction{
		ActorUserID:         session.User.ID,
		ActorOrganizationID: session.OrganizationID,
		AgentID:             agentID,
		Reason:              req.Reason,
	}); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "appealed"})
}

func (h marketplaceHandler) reinstateAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if h.governanceService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace governance is not configured")
		return
	}
	if err := h.governanceService.ReinstateAgent(r.Context(), marketplace.GovernanceAction{
		ActorUserID:         session.User.ID,
		ActorOrganizationID: session.OrganizationID,
		AgentID:             agentID,
		Reason:              req.Reason,
	}); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "approved"})
}

func (h marketplaceHandler) reportAbuse(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Details string `json:"details"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if h.governanceService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace governance is not configured")
		return
	}
	report, err := h.governanceService.ReportAbuse(r.Context(), marketplace.AbuseReportRequest{
		ReporterOrganizationID: session.OrganizationID,
		ReporterUserID:         session.User.ID,
		AgentID:                agentID,
		Reason:                 req.Reason,
		Details:                req.Details,
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, report)
}

func (h marketplaceHandler) resolveAbuseReport(w stdhttp.ResponseWriter, r *stdhttp.Request, reportID string, status string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if h.governanceService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace governance is not configured")
		return
	}
	if err := h.governanceService.ResolveAbuseReport(r.Context(), marketplace.AbuseResolution{
		ReportID:       reportID,
		ReviewerUserID: session.User.ID,
		Status:         status,
		Resolution:     req.Resolution,
	}); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": status})
}

func (h marketplaceHandler) listAbuseReports(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.governanceService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "marketplace governance is not configured")
		return
	}
	reports, err := h.governanceService.ListAbuseReports(r.Context(), marketplace.AbuseReportFilter{
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:  parseQueryInt(r, "limit", 20, 100),
		Offset: parseQueryInt(r, "offset", 0, 0),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"reports": reports,
		"total":   len(reports),
	})
}

func (h marketplaceHandler) getAgentVersions(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	versions, err := h.service.ListAgentVersions(r.Context(), agentID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"versions": versions,
		"total":    len(versions),
	})
}

func (h marketplaceHandler) getPublisherStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	stats, err := h.service.GetPublisherStats(r.Context(), session.User.ID, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, stats)
}

func (h marketplaceHandler) getPublisherSettlementPreferences(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	preferences, err := h.service.GetPublisherSettlementPreferences(r.Context(), session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, preferences)
}

func (h marketplaceHandler) updatePublisherSettlementPreferences(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req struct {
		Cycle string `json:"cycle"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	preferences, err := h.service.UpdatePublisherSettlementPreferences(r.Context(), session.OrganizationID, req.Cycle)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, preferences)
}

func (h marketplaceHandler) getAgentStats(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	stats, err := h.service.GetAgentStats(r.Context(), agentID, session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, stats)
}

func marketplaceSearchFilter(r *stdhttp.Request) marketplace.MarketplaceSearchFilter {
	query := r.URL.Query()
	category := strings.TrimSpace(query.Get("categorySlug"))
	if category == "" {
		category = strings.TrimSpace(query.Get("category"))
	}

	sort := strings.TrimSpace(query.Get("sort"))
	switch sort {
	case "installs":
		sort = "popular"
	case "featured":
		sort = "recommended"
	case "relevance":
		sort = ""
	}

	return marketplace.MarketplaceSearchFilter{
		Query:        firstNonEmpty(query.Get("query"), query.Get("q")),
		CategorySlug: category,
		Tags:         splitQueryCSV(r, "tags"),
		MinRating:    parseQueryInt(r, "minRating", 0, 5),
		MaxRating:    parseQueryInt(r, "maxRating", 0, 5),
		PricingType:  strings.TrimSpace(query.Get("pricingType")),
		Sort:         sort,
		Limit:        parseQueryInt(r, "limit", 20, 50),
		Offset:       parseQueryInt(r, "offset", 0, 0),
	}
}

func marketplaceTemplateFilter(r *stdhttp.Request) marketplace.TemplateFilter {
	query := r.URL.Query()
	return marketplace.TemplateFilter{
		Query:    firstNonEmpty(query.Get("query"), query.Get("q")),
		Type:     strings.TrimSpace(query.Get("type")),
		Category: strings.TrimSpace(query.Get("category")),
		Tags:     splitQueryCSV(r, "tags"),
		Limit:    parseQueryInt(r, "limit", 20, 100),
		Offset:   parseQueryInt(r, "offset", 0, 0),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
