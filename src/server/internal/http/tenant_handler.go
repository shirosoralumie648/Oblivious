package http

import (
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/tenant"
)

type tenantHandler struct {
	authMiddleware authMiddleware
	authService    *auth.Service
	service        *tenant.Service
}

func newTenantHandler(service *tenant.Service, authService *auth.Service, authMiddleware authMiddleware) tenantHandler {
	return tenantHandler{service: service, authService: authService, authMiddleware: authMiddleware}
}

func (h tenantHandler) listOrganizations(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filter := tenant.OrganizationListFilter{
		Status: r.URL.Query().Get("status"),
		Search: r.URL.Query().Get("search"),
		Limit:  parseQueryInt(r, "limit", 20, 100),
		Offset: parseQueryInt(r, "offset", 0, 0),
	}

	organizations, total, err := h.service.ListOrganizations(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"organizations": organizations,
		"total":         total,
	})
}

func (h tenantHandler) createOrganization(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	var req tenant.CreateOrganizationRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	req.CreatedByUserID = &session.User.ID

	organization, err := h.service.CreateOrganization(r.Context(), req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, organization)
}

func (h tenantHandler) getOrganization(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	organization, err := h.service.GetOrganization(r.Context(), organizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if organization == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "organization not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, organization)
}

func (h tenantHandler) updateOrganization(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	var req tenant.OrganizationUpdateRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}

	organization, err := h.service.UpdateOrganization(r.Context(), organizationID, req)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if organization == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "organization not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, organization)
}

func (h tenantHandler) archiveOrganization(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	if _, ok := sessionOrUnauthorized(w, r); !ok {
		return
	}

	organization, err := h.service.ArchiveOrganization(r.Context(), organizationID)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if organization == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "organization not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, organization)
}

func (h tenantHandler) listMyOrganizations(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}

	memberships, err := h.service.ListMembershipsForUser(r.Context(), session.User.ID)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"memberships": memberships})
}

func (h tenantHandler) selectOrganization(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	scope, err := h.service.ResolveOrganizationScope(r.Context(), session.User.ID, organizationID)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	if h.authService == nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "auth service unavailable")
		return
	}
	updated, err := h.authService.SetSessionOrganization(r.Context(), session.ID, scope.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "select organization failed")
		return
	}
	h.authMiddleware.setSessionCookie(w, updated)
	writeSuccess(w, stdhttp.StatusOK, map[string]any{
		"organization": map[string]string{"id": scope.OrganizationID},
		"session":      map[string]string{"id": updated.ID},
	})
}

func (h tenantHandler) listOrganizationMembers(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	members, err := h.service.ListOrganizationMembers(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, organizationID)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]any{"members": members})
}

func (h tenantHandler) inviteMember(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req tenant.InviteMemberRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	invitation, err := h.service.InviteMember(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, organizationID, req)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, invitation)
}

func (h tenantHandler) acceptInvitation(w stdhttp.ResponseWriter, r *stdhttp.Request, token string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	membership, err := h.service.AcceptInvitation(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, token)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	if h.authService != nil {
		rotated, err := h.authService.RotateSession(r.Context(), session.ID)
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "rotate session failed")
			return
		}
		h.authMiddleware.setSessionCookie(w, rotated)
	}
	writeSuccess(w, stdhttp.StatusOK, membership)
}

func (h tenantHandler) revokeInvitation(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID, invitationID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	invitation, err := h.service.RevokeInvitation(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, organizationID, invitationID)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	if invitation == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "invitation not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, invitation)
}

func (h tenantHandler) updateMemberRole(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID, userID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req tenant.UpdateMemberRoleRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	membership, err := h.service.UpdateMemberRole(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, organizationID, userID, req)
	if err != nil {
		writeTenantError(w, err)
		return
	}
	if err := h.revokeUserSessions(r, membership.UserID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "revoke member sessions failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, membership)
}

func (h tenantHandler) removeMember(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID, userID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	if err := h.service.RemoveMember(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, organizationID, userID); err != nil {
		writeTenantError(w, err)
		return
	}
	if err := h.revokeUserSessions(r, userID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "revoke member sessions failed")
		return
	}
	w.WriteHeader(stdhttp.StatusNoContent)
}

func (h tenantHandler) transferOwnership(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string) {
	session, ok := sessionOrUnauthorized(w, r)
	if !ok {
		return
	}
	var req tenant.TransferOwnershipRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if err := h.service.TransferOwnership(r.Context(), tenant.Actor{
		UserID:    session.User.ID,
		Email:     session.User.Email,
		IPAddress: clientIP(r),
	}, organizationID, req); err != nil {
		writeTenantError(w, err)
		return
	}
	if err := h.revokeUserSessions(r, session.User.ID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "revoke owner sessions failed")
		return
	}
	if err := h.revokeUserSessions(r, req.NewOwnerUserID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "revoke new owner sessions failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]bool{"transferred": true})
}

func (h tenantHandler) revokeUserSessions(r *stdhttp.Request, userID string) error {
	if h.authService == nil {
		return nil
	}
	return h.authService.RevokeUserSessions(r.Context(), userID, "")
}

func writeTenantError(w stdhttp.ResponseWriter, err error) {
	message := err.Error()
	switch {
	case containsAny(message, "required", "insufficient", "only owners", "membership required"):
		writeError(w, stdhttp.StatusForbidden, "forbidden", message)
	case containsAny(message, "not found"):
		writeError(w, stdhttp.StatusNotFound, "not_found", message)
	case containsAny(message, "active owner", "already", "transfer ownership", "not pending", "revoked", "expired"):
		writeError(w, stdhttp.StatusConflict, "conflict", message)
	default:
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", message)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func clientIP(r *stdhttp.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return r.RemoteAddr
}
