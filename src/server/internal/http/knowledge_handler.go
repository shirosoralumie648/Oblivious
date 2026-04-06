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

type createKnowledgeDocumentRequest struct {
	Content string `json:"content"`
	Title   string `json:"title"`
}

type retrieveKnowledgeRequest struct {
	Query string `json:"query"`
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

func (h knowledgeHandler) updateKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
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

	base, err := h.service.Update(r.Context(), session, knowledgeBaseID, name)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update knowledge base failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, base)
}

func (h knowledgeHandler) deleteKnowledgeBase(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.Delete(r.Context(), session, knowledgeBaseID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "delete knowledge base failed")
		return
	}

	w.WriteHeader(stdhttp.StatusNoContent)
}

func (h knowledgeHandler) listKnowledgeDocuments(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	documents, err := h.service.ListDocuments(r.Context(), session, knowledgeBaseID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list knowledge documents failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, documents)
}

func (h knowledgeHandler) createKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	if title == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "title is required")
		return
	}

	document, err := h.service.CreateDocument(r.Context(), session, knowledgeBaseID, title, content)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create knowledge document failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, document)
}

func (h knowledgeHandler) retrieveKnowledge(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload retrieveKnowledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	query := strings.TrimSpace(payload.Query)
	if query == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "query is required")
		return
	}

	results, err := h.service.Retrieve(r.Context(), session, knowledgeBaseID, query)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "retrieve knowledge failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, results)
}

func (h knowledgeHandler) updateKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createKnowledgeDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	title := strings.TrimSpace(payload.Title)
	content := strings.TrimSpace(payload.Content)
	if title == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "title is required")
		return
	}

	document, err := h.service.UpdateDocument(r.Context(), session, knowledgeBaseID, documentID, title, content)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update knowledge document failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, document)
}

func (h knowledgeHandler) deleteKnowledgeDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, knowledgeBaseID, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.DeleteDocument(r.Context(), session, knowledgeBaseID, documentID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "delete knowledge document failed")
		return
	}

	w.WriteHeader(stdhttp.StatusNoContent)
}
