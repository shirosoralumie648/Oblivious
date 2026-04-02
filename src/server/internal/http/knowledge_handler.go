package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/knowledge"
)

type knowledgeHandler struct {
	service *knowledge.Service
}

type createKnowledgeBaseRequest struct {
	Name string `json:"name"`
}

func newKnowledgeHandler(service *knowledge.Service) knowledgeHandler {
	return knowledgeHandler{service: service}
}

func (h knowledgeHandler) listKnowledgeBases(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	bases, err := h.service.List(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge bases failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, bases)
}

func (h knowledgeHandler) getKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	base, err := h.service.Get(r.Context(), session, knowledgeBaseID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get knowledge base failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, base)
}

func (h knowledgeHandler) createKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	base, err := h.service.Create(r.Context(), session, name)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create knowledge base failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, base)
}
