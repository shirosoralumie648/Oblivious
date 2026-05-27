package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/marketplace"
)

type marketplaceHandler struct {
	service       *marketplace.Service
	searchService *marketplace.SearchService
}

func newMarketplaceHandler(service *marketplace.Service, searchService *marketplace.SearchService) marketplaceHandler {
	return marketplaceHandler{service: service, searchService: searchService}
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
		"agent":    agent,
		"versions": versions,
	})
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
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, agent)
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
	if versionID == "" && r.Body != nil {
		var req struct {
			VersionID string `json:"versionID"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			versionID = strings.TrimSpace(req.VersionID)
		}
	}

	install, err := h.service.InstallAgent(r.Context(), session.User.ID, session.OrganizationID, agentID, versionID)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, install)
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
