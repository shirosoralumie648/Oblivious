package http

import (
	stdhttp "net/http"

	"oblivious/server/internal/tenant"
)

type tenantHandler struct {
	service *tenant.Service
}

func newTenantHandler(service *tenant.Service) tenantHandler {
	return tenantHandler{service: service}
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
