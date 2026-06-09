package http

import (
	"context"
	"encoding/json"
	"net"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/marketplace"
	"oblivious/server/internal/quota"
)

type adminQuotaSettingsService interface {
	ListUsageLimitSettings(ctx context.Context, organizationID string) ([]quota.UsageLimitSettings, error)
	SaveUsageLimitSettings(ctx context.Context, settings quota.UsageLimitSettings) (*quota.UsageLimitSettings, error)
}

type adminMarketplacePayoutService interface {
	MarkPayoutPaid(ctx context.Context, payoutID string, providerPayoutID string) (*marketplace.MarketplacePayout, error)
	MarkPayoutFailed(ctx context.Context, payoutID string, providerPayoutID string, reason string) (*marketplace.MarketplacePayout, error)
	CreateDuePayouts(ctx context.Context, now time.Time) ([]*marketplace.MarketplacePayout, error)
}

type adminReviewSLAEnforcer interface {
	EnforceReviewSLAs(ctx context.Context, options marketplace.ReviewSLAEnforcementOptions) (marketplace.ReviewSLAEnforcementResult, error)
}

type adminHandler struct {
	service          *admin.Service
	quotaService     adminQuotaSettingsService
	payoutService    adminMarketplacePayoutService
	reviewSLAService adminReviewSLAEnforcer
}

func newAdminHandler(service *admin.Service) adminHandler {
	return adminHandler{service: service}
}

func newAdminHandlerWithQuota(service *admin.Service, quotaService adminQuotaSettingsService) adminHandler {
	return adminHandler{service: service, quotaService: quotaService}
}

func newAdminHandlerWithPayouts(service *admin.Service, payoutService adminMarketplacePayoutService) adminHandler {
	return adminHandler{service: service, payoutService: payoutService}
}

func newAdminHandlerWithReviewSLA(service *admin.Service, reviewSLAService adminReviewSLAEnforcer) adminHandler {
	return adminHandler{service: service, reviewSLAService: reviewSLAService}
}

func newAdminHandlerWithPayoutsAndReviewSLA(service *admin.Service, payoutService adminMarketplacePayoutService, reviewSLAService adminReviewSLAEnforcer) adminHandler {
	return adminHandler{service: service, payoutService: payoutService, reviewSLAService: reviewSLAService}
}

func newAdminHandlerWithQuotaPayoutsAndReviewSLA(service *admin.Service, quotaService adminQuotaSettingsService, payoutService adminMarketplacePayoutService, reviewSLAService adminReviewSLAEnforcer) adminHandler {
	return adminHandler{service: service, quotaService: quotaService, payoutService: payoutService, reviewSLAService: reviewSLAService}
}

func newAdminHandlerWithQuotaAndPayouts(service *admin.Service, quotaService adminQuotaSettingsService, payoutService adminMarketplacePayoutService) adminHandler {
	return adminHandler{service: service, quotaService: quotaService, payoutService: payoutService}
}

func (h adminHandler) getStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	stats, err := h.service.GetSystemStats(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, stats)
}

func (h adminHandler) listChannels(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.ChannelFilter{
		Provider: r.URL.Query().Get("provider"),
		Status:   r.URL.Query().Get("status"),
		Search:   r.URL.Query().Get("search"),
		Limit:    parseQueryInt(r, "limit", 20, 100),
		Offset:   parseQueryInt(r, "offset", 0, 0),
	}

	channels, err := h.service.ListChannels(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"channels": channels,
		"total":    len(channels),
	})
}

func (h adminHandler) listChannelProviders(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	providers, err := h.service.ListChannelProviders(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"providers": providers,
	})
}

func (h adminHandler) getChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	channel, err := h.service.GetChannel(r.Context(), channelID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if channel == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, channel)
}

func (h adminHandler) createChannel(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.ChannelCreateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	channel, err := h.service.CreateChannel(r.Context(), session, req, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, channel)
}

func (h adminHandler) updateChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.ChannelUpdateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	channel, err := h.service.UpdateChannel(r.Context(), session, channelID, req, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, channel)
}

func (h adminHandler) deleteChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.DeleteChannel(r.Context(), session, channelID, r); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h adminHandler) testChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	result, err := h.service.TestChannel(r.Context(), channelID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h adminHandler) syncChannelModels(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	result, err := h.service.SyncChannelModels(r.Context(), session, channelID, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h adminHandler) detectChannelModelUpdates(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	result, err := h.service.DetectChannelModelUpdates(r.Context(), channelID)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h adminHandler) applyChannelModelUpdates(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.ChannelModelUpdateApplyRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	result, err := h.service.ApplyChannelModelUpdates(r.Context(), session, channelID, req, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h adminHandler) refreshChannelBalance(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	result, err := h.service.RefreshChannelBalance(r.Context(), session, channelID, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h adminHandler) getChannelHealth(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	health, err := h.service.GetChannelHealth(r.Context(), channelID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, health)
}

func (h adminHandler) listChannelRuntimeStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	stats, err := h.service.ListChannelRuntimeStats(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"stats": stats,
	})
}

func (h adminHandler) batchUpdateChannels(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req struct {
		IDs     []string `json:"ids"`
		Action  string   `json:"action"`
		Enabled *bool    `json:"enabled,omitempty"`
	}
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	action := strings.TrimSpace(req.Action)
	if action == "" && req.Enabled != nil {
		if *req.Enabled {
			action = "enable"
		} else {
			action = "disable"
		}
	}
	if len(req.IDs) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "ids are required")
		return
	}

	if err := h.service.BatchUpdateChannels(r.Context(), session, req.IDs, action, r); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "updated"})
}

func (h adminHandler) getRelayPricingSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	settings, err := h.service.GetRelayPricingSettings(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, settings)
}

func (h adminHandler) updateRelayPricingSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.RelayPricingSettings
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	settings, err := h.service.UpdateRelayPricingSettings(r.Context(), session, req, requestClientIP(r))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, settings)
}

func (h adminHandler) listUsageLimitSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	if h.quotaService == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "quota_unavailable", "quota settings service is unavailable")
		return
	}

	settings, err := h.quotaService.ListUsageLimitSettings(r.Context(), session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"usageLimits": settings})
}

func (h adminHandler) updateUsageLimitSettings(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	if h.quotaService == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "quota_unavailable", "quota settings service is unavailable")
		return
	}

	var req quota.UsageLimitSettings
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	req.OrganizationID = session.OrganizationID
	settings, err := h.quotaService.SaveUsageLimitSettings(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, settings)
}

func (h adminHandler) listUsageLogs(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.UsageLogFilter{
		OrganizationID: r.URL.Query().Get("organizationID"),
		UserID:         r.URL.Query().Get("userID"),
		APITokenID:     r.URL.Query().Get("apiTokenID"),
		RequestID:      r.URL.Query().Get("requestID"),
		APIType:        r.URL.Query().Get("apiType"),
		FeatureType:    r.URL.Query().Get("featureType"),
		QuotaMode:      r.URL.Query().Get("quotaMode"),
		Model:          r.URL.Query().Get("model"),
		ChannelID:      r.URL.Query().Get("channelID"),
		Provider:       r.URL.Query().Get("provider"),
		Status:         r.URL.Query().Get("status"),
		Limit:          parseQueryInt(r, "limit", 50, 100),
		Offset:         parseQueryInt(r, "offset", 0, 0),
	}

	logs, total, err := h.service.ListUsageLogs(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"usageLogs": logs,
		"total":     total,
	})
}

func (h adminHandler) getUsageAnalytics(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.UsageAnalyticsFilter{
		OrganizationID: r.URL.Query().Get("organizationID"),
		UserID:         r.URL.Query().Get("userID"),
		APIType:        r.URL.Query().Get("apiType"),
		FeatureType:    r.URL.Query().Get("featureType"),
		QuotaMode:      r.URL.Query().Get("quotaMode"),
		Model:          r.URL.Query().Get("model"),
		ChannelID:      r.URL.Query().Get("channelID"),
		Provider:       r.URL.Query().Get("provider"),
		Status:         r.URL.Query().Get("status"),
		Granularity:    r.URL.Query().Get("granularity"),
		Limit:          parseQueryInt(r, "limit", 10, 100),
	}
	if from, ok := parseQueryTime(r, "from"); ok {
		filter.From = from
	}
	if to, ok := parseQueryTime(r, "to"); ok {
		filter.To = to
	}
	analytics, err := h.service.GetUsageAnalytics(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, analytics)
}

func (h adminHandler) listAPITokens(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.APITokenFilter{
		OrganizationID: r.URL.Query().Get("organizationID"),
		UserID:         r.URL.Query().Get("userID"),
		Status:         r.URL.Query().Get("status"),
		UserGroup:      r.URL.Query().Get("userGroup"),
		Search:         r.URL.Query().Get("search"),
		Model:          r.URL.Query().Get("model"),
		Limit:          parseQueryInt(r, "limit", 50, 100),
		Offset:         parseQueryInt(r, "offset", 0, 0),
	}

	tokens, total, err := h.service.ListAPITokens(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"apiTokens": tokens,
		"total":     total,
	})
}

func (h adminHandler) revokeAPIToken(w stdhttp.ResponseWriter, r *stdhttp.Request, tokenID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.RevokeAPIToken(r.Context(), session, tokenID, requestClientIP(r)); err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "api token not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "revoked"})
}

func (h adminHandler) listModelInventory(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.ModelInventoryFilter{
		Provider: r.URL.Query().Get("provider"),
		Group:    r.URL.Query().Get("group"),
		Status:   r.URL.Query().Get("status"),
		Search:   r.URL.Query().Get("search"),
		Sort:     r.URL.Query().Get("sort"),
		Limit:    parseQueryInt(r, "limit", 50, 100),
		Offset:   parseQueryInt(r, "offset", 0, 0),
	}

	models, total, err := h.service.ListModelInventory(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"models": models,
		"total":  total,
	})
}

func (h adminHandler) listRoutes(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	routes, err := h.service.ListRoutes(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"routes": routes,
		"total":  len(routes),
	})
}

func (h adminHandler) getRoute(w stdhttp.ResponseWriter, r *stdhttp.Request, routeID string) {
	route, err := h.service.GetRoute(r.Context(), routeID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if route == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, route)
}

func (h adminHandler) createRoute(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.RouteCreateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	route, err := h.service.CreateRoute(r.Context(), session, req, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, route)
}

func (h adminHandler) updateRoute(w stdhttp.ResponseWriter, r *stdhttp.Request, routeID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.RouteUpdateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	route, err := h.service.UpdateRoute(r.Context(), session, routeID, req, r)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, route)
}

func (h adminHandler) deleteRoute(w stdhttp.ResponseWriter, r *stdhttp.Request, routeID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.DeleteRoute(r.Context(), session, routeID, r); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h adminHandler) listPlans(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.PlanFilter{
		Search: r.URL.Query().Get("search"),
		Limit:  parseQueryInt(r, "limit", 20, 100),
		Offset: parseQueryInt(r, "offset", 0, 0),
	}
	if isPublic, ok := parseOptionalBool(r, "isPublic"); ok {
		filter.IsPublic = &isPublic
	}
	switch r.URL.Query().Get("status") {
	case "active":
		active := true
		filter.IsActive = &active
	case "inactive":
		active := false
		filter.IsActive = &active
	}

	plans, err := h.service.ListPlans(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"plans": plans,
		"total": len(plans),
	})
}

func (h adminHandler) getPlan(w stdhttp.ResponseWriter, r *stdhttp.Request, planID string) {
	plan, err := h.service.GetPlan(r.Context(), planID)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, plan)
}

func (h adminHandler) createPlan(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.PlanCreateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	plan, err := h.service.CreatePlan(r.Context(), session, req, requestClientIP(r))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, plan)
}

func (h adminHandler) updatePlan(w stdhttp.ResponseWriter, r *stdhttp.Request, planID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.PlanUpdateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	plan, err := h.service.UpdatePlan(r.Context(), session, planID, req, requestClientIP(r))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, plan)
}

func (h adminHandler) deactivatePlan(w stdhttp.ResponseWriter, r *stdhttp.Request, planID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.DeactivatePlan(r.Context(), session, planID, requestClientIP(r)); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deactivated"})
}

func (h adminHandler) listUsers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := admin.UserListFilter{
		Search: r.URL.Query().Get("search"),
		Role:   r.URL.Query().Get("role"),
		PlanID: r.URL.Query().Get("planID"),
		Status: r.URL.Query().Get("status"),
		Sort:   r.URL.Query().Get("sort"),
		Limit:  limit,
		Offset: offset,
	}

	users, total, err := h.service.ListUsers(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]interface{}{
		"users": users,
		"total": total,
	})
}

func (h adminHandler) getUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	user, err := h.service.GetUserDetail(r.Context(), userID)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, user)
}

func (h adminHandler) updateUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req admin.UserUpdateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	user, err := h.service.UpdateUser(r.Context(), session.User.ID, session.User.Email, userID, req, requestClientIP(r))
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, user)
}

func (h adminHandler) disableUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.DisableUser(r.Context(), session.User.ID, session.User.Email, userID, requestClientIP(r)); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "disabled"})
}

func (h adminHandler) enableUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.EnableUser(r.Context(), session.User.ID, session.User.Email, userID, requestClientIP(r)); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "enabled"})
}

type updateUserQuotaRequest struct {
	Balance float64 `json:"balance"`
}

func (h adminHandler) updateUserQuota(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	var req updateUserQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	if err := h.service.UpdateUserQuota(r.Context(), userID, req.Balance); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	user, _ := h.service.GetUserDetail(r.Context(), userID)
	writeSuccess(w, stdhttp.StatusOK, user)
}

func (h adminHandler) deleteUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	if err := h.service.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h adminHandler) listAuditLogs(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := admin.AuditFilter{
		ActorID:        r.URL.Query().Get("actorID"),
		Action:         r.URL.Query().Get("action"),
		OrganizationID: firstNonEmpty(r.URL.Query().Get("organizationID"), r.URL.Query().Get("organizationId")),
		ResourceType:   r.URL.Query().Get("resourceType"),
		ResourceID:     r.URL.Query().Get("resourceID"),
		DateFrom:       r.URL.Query().Get("startDate"),
		DateTo:         r.URL.Query().Get("endDate"),
		Limit:          parseQueryInt(r, "limit", 50, 100),
		Offset:         parseQueryInt(r, "offset", 0, 0),
	}

	entries, total, err := h.service.ListAuditEntries(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"entries": entries,
		"total":   total,
	})
}

func (h adminHandler) listReviews(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "pending" && status != "pending_review" {
		writeSuccess(w, stdhttp.StatusOK, map[string]any{
			"reviews": []*struct{}{},
			"total":   0,
		})
		return
	}

	reviews, err := h.service.ListPendingReviews(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"reviews": reviews,
		"total":   len(reviews),
	})
}

func (h adminHandler) enforceReviewSLA(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.reviewSLAService == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "marketplace review SLA service is not configured")
		return
	}
	result, err := h.reviewSLAService.EnforceReviewSLAs(r.Context(), marketplace.ReviewSLAEnforcementOptions{
		Limit:  parseQueryInt(r, "limit", 100, 100),
		Offset: parseQueryInt(r, "offset", 0, 0),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h adminHandler) approveAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	if err := h.service.ApproveAgent(r.Context(), agentID); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	_ = h.service.LogAction(r.Context(), session.User.ID, session.User.Email, "agent.approve", "agent", agentID, "", requestClientIP(r))

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "approved"})
}

func (h adminHandler) rejectAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
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

	if err := h.service.RejectAgent(r.Context(), agentID, req.Reason); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	_ = h.service.LogAction(r.Context(), session.User.ID, session.User.Email, "agent.reject", "agent", agentID, req.Reason, requestClientIP(r))

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "rejected"})
}

func (h adminHandler) needsChangesAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
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

	if err := h.service.RequestAgentChanges(r.Context(), agentID, req.Reason); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	_ = h.service.LogAction(r.Context(), session.User.ID, session.User.Email, "agent.needs_changes", "agent", agentID, req.Reason, requestClientIP(r))

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "needs_changes"})
}

func (h adminHandler) billingFilter(r *stdhttp.Request) admin.BillingInspectionFilter {
	return admin.BillingInspectionFilter{
		OrganizationID: firstNonEmpty(r.URL.Query().Get("organizationID"), r.URL.Query().Get("organizationId")),
		UserID:         firstNonEmpty(r.URL.Query().Get("userID"), r.URL.Query().Get("userId")),
		Status:         r.URL.Query().Get("status"),
		Kind:           r.URL.Query().Get("kind"),
		Provider:       r.URL.Query().Get("provider"),
		Limit:          parseQueryInt(r, "limit", 50, 100),
		Offset:         parseQueryInt(r, "offset", 0, 0),
	}
}

func (h adminHandler) getBillingSummary(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	summary, err := h.service.GetBillingInspectionSummary(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, summary)
}

func (h adminHandler) listBillingSessions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListBillingSessions(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"sessions": items, "total": total})
}

func (h adminHandler) listPaymentIntents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListPaymentIntents(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"paymentIntents": items, "total": total})
}

func (h adminHandler) listWebhookEvents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListWebhookEvents(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"webhookEvents": items, "total": total})
}

func (h adminHandler) listSubscriptions(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListSubscriptions(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"subscriptions": items, "total": total})
}

func (h adminHandler) listTopups(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListTopups(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"topups": items, "total": total})
}

func (h adminHandler) listInvoices(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListInvoices(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"invoices": items, "total": total})
}

func (h adminHandler) listRefunds(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListRefunds(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"refunds": items, "total": total})
}

func (h adminHandler) recordTopupRefund(w stdhttp.ResponseWriter, r *stdhttp.Request, topupID string) {
	var request admin.TopupRefundRequest
	if !decodeRequestJSON(w, r, &request) {
		return
	}
	if err := admin.ValidateTopupRefundRequest(request); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	refund, err := h.service.RecordTopupRefund(r.Context(), topupID, request)
	if err != nil {
		if isNotFoundError(err) || strings.Contains(err.Error(), "not found") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "topup not found")
			return
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, refund)
}

func (h adminHandler) listMarketplaceSettlements(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListMarketplaceSettlements(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"settlements": items, "total": total})
}

func (h adminHandler) listMarketplacePayouts(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	items, total, err := h.service.ListMarketplacePayouts(r.Context(), h.billingFilter(r))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"payouts": items, "total": total})
}

func (h adminHandler) createDueMarketplacePayouts(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if h.payoutService == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "marketplace payout service is not configured")
		return
	}
	payouts, err := h.payoutService.CreateDuePayouts(r.Context(), time.Now().UTC())
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"payouts": payouts, "total": len(payouts)})
}

func (h adminHandler) markMarketplacePayoutPaid(w stdhttp.ResponseWriter, r *stdhttp.Request, payoutID string) {
	if h.payoutService == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "marketplace payout service is not configured")
		return
	}
	var request struct {
		ProviderPayoutID string `json:"providerPayoutID"`
	}
	if !decodeRequestJSON(w, r, &request) {
		return
	}
	providerPayoutID := strings.TrimSpace(request.ProviderPayoutID)
	if providerPayoutID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "providerPayoutID is required")
		return
	}
	payout, err := h.payoutService.MarkPayoutPaid(r.Context(), payoutID, providerPayoutID)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, payout)
}

func (h adminHandler) markMarketplacePayoutFailed(w stdhttp.ResponseWriter, r *stdhttp.Request, payoutID string) {
	if h.payoutService == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "marketplace payout service is not configured")
		return
	}
	var request struct {
		ProviderPayoutID string `json:"providerPayoutID"`
		Reason           string `json:"reason"`
	}
	if !decodeRequestJSON(w, r, &request) {
		return
	}
	providerPayoutID := strings.TrimSpace(request.ProviderPayoutID)
	if providerPayoutID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "providerPayoutID is required")
		return
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "reason is required")
		return
	}
	payout, err := h.payoutService.MarkPayoutFailed(r.Context(), payoutID, providerPayoutID, reason)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, payout)
}

func decodeRequestJSON(w stdhttp.ResponseWriter, r *stdhttp.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func sessionOrUnauthorized(w stdhttp.ResponseWriter, r *stdhttp.Request) (auth.Session, bool) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return auth.Session{}, false
	}
	return session, true
}

func parseQueryInt(r *stdhttp.Request, key string, defaultValue int, maxValue int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	if parsed < 0 {
		return defaultValue
	}
	if maxValue > 0 && parsed > maxValue {
		return maxValue
	}
	return parsed
}

func parseQueryTime(r *stdhttp.Request, key string) (time.Time, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func parseOptionalBool(r *stdhttp.Request, key string) (bool, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func splitQueryCSV(r *stdhttp.Request, key string) []string {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func requestClientIP(r *stdhttp.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
