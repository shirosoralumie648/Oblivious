package http

import (
	"encoding/json"
	"net"
	stdhttp "net/http"
	"strconv"
	"strings"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/auth"
)

type adminHandler struct {
	service *admin.Service
}

func newAdminHandler(service *admin.Service) adminHandler {
	return adminHandler{service: service}
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

func (h adminHandler) getChannelHealth(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	health, err := h.service.GetChannelHealth(r.Context(), channelID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, health)
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
