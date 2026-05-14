package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strconv"
	"strings"

	"oblivious/server/internal/memory"
)

type memoryHandler struct {
	service *memory.Service
}

func newMemoryHandler(service *memory.Service) memoryHandler {
	return memoryHandler{service: service}
}

// POST /api/v1/app/memory/documents
func (h memoryHandler) addDocument(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req memory.AddDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
		return
	}

	doc, err := h.service.AddDocument(r.Context(), session, &req)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, doc)
}

// GET /api/v1/app/memory/documents/:id
func (h memoryHandler) getDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	doc, err := h.service.GetDocument(r.Context(), session, id)
	if err != nil {
		if err.Error() == "document not found" {
			writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "access denied" {
			writeError(w, stdhttp.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, doc)
}

// GET /api/v1/app/memory/documents
func (h memoryHandler) listDocuments(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	docs, err := h.service.ListDocuments(r.Context(), session, limit, offset)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, docs)
}

// PUT /api/v1/app/memory/documents/:id
func (h memoryHandler) updateDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req struct {
		Title   *string `json:"title,omitempty"`
		Content *string `json:"content,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	title := ""
	content := ""
	if req.Title != nil {
		title = *req.Title
	}
	if req.Content != nil {
		content = *req.Content
	}

	doc, err := h.service.UpdateDocument(r.Context(), session, id, title, content)
	if err != nil {
		if err.Error() == "document not found" {
			writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "access denied" {
			writeError(w, stdhttp.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, doc)
}

// DELETE /api/v1/app/memory/documents/:id
func (h memoryHandler) deleteDocument(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	err := h.service.DeleteDocument(r.Context(), session, id)
	if err != nil {
		if err.Error() == "document not found" {
			writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "access denied" {
			writeError(w, stdhttp.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/v1/app/memory/search
func (h memoryHandler) search(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req memory.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "query is required")
		return
	}

	results, err := h.service.Search(r.Context(), session, &req)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, results)
}

// GET /api/v1/app/memory/documents/:id/chunks
func (h memoryHandler) listChunks(w stdhttp.ResponseWriter, r *stdhttp.Request, documentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	chunks, err := h.service.ListChunks(r.Context(), session, documentID)
	if err != nil {
		if err.Error() == "document not found" {
			writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "access denied" {
			writeError(w, stdhttp.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, chunks)
}
