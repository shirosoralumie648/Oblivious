package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strconv"
	"strings"

	"oblivious/server/internal/agent"
)

type agentMemoriesHandler struct {
	service *agent.Service
}

func newAgentMemoriesHandler(service *agent.Service) agentMemoriesHandler {
	return agentMemoriesHandler{service: service}
}

type agentMemoryCreateRequest struct {
	AgentID      string         `json:"agent_id"`
	AgentIDCamel string         `json:"agentId"`
	Content      string         `json:"content"`
	Importance   int            `json:"importance"`
	Metadata     map[string]any `json:"metadata"`
	Title        string         `json:"title"`
	Type         string         `json:"type"`
}

type agentMemoryImportRequest struct {
	Memories []agentMemoryCreateRequest `json:"memories"`
}

type agentMemoryUpdateRequest struct {
	Content    *string `json:"content"`
	Importance *int    `json:"importance"`
}

func (h agentMemoriesHandler) createMemory(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if h.service == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "agent memory service is not configured")
		return
	}

	var req agentMemoryCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
		return
	}

	memory, err := h.service.CreateMemory(r.Context(), session, agent.CreateMemoryRequest{
		AgentID:    firstAgentRunNonEmpty(req.AgentID, req.AgentIDCamel),
		Type:       strings.TrimSpace(req.Type),
		Content:    content,
		Importance: req.Importance,
		Metadata:   req.Metadata,
	})
	if err != nil {
		writeAgentMemoryError(w, err)
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, memory)
}

func (h agentMemoriesHandler) importMemories(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if h.service == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "agent memory service is not configured")
		return
	}

	var req agentMemoryImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if len(req.Memories) == 0 {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "memories are required")
		return
	}
	if len(req.Memories) > 100 {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "memories import is limited to 100 entries")
		return
	}

	imported := make([]*agent.Memory, 0, len(req.Memories))
	for _, item := range req.Memories {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
			return
		}
		memory, err := h.service.CreateMemory(r.Context(), session, agent.CreateMemoryRequest{
			AgentID:    firstAgentRunNonEmpty(item.AgentID, item.AgentIDCamel),
			Type:       strings.TrimSpace(item.Type),
			Content:    content,
			Importance: item.Importance,
			Metadata:   item.Metadata,
		})
		if err != nil {
			writeAgentMemoryError(w, err)
			return
		}
		imported = append(imported, memory)
	}

	writeSuccess(w, stdhttp.StatusCreated, imported)
}

func (h agentMemoriesHandler) searchMemories(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if h.service == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "agent memory service is not configured")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit, _ = strconv.Atoi(r.URL.Query().Get("topK"))
	}
	if limit == 0 {
		limit, _ = strconv.Atoi(r.URL.Query().Get("top_k"))
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	memories, err := h.service.ListMemories(r.Context(), session, agent.ListMemoriesRequest{
		AgentID: firstAgentRunNonEmpty(r.URL.Query().Get("agentId"), r.URL.Query().Get("agent_id")),
		Type:    r.URL.Query().Get("type"),
		Query:   query,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		writeAgentMemoryError(w, err)
		return
	}

	writeSuccess(w, stdhttp.StatusOK, memories)
}

func (h agentMemoriesHandler) updateMemory(w stdhttp.ResponseWriter, r *stdhttp.Request, memoryID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if h.service == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "agent memory service is not configured")
		return
	}

	var req agentMemoryUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	memory, err := h.service.UpdateMemory(r.Context(), session, memoryID, agent.UpdateMemoryRequest{
		Content:    req.Content,
		Importance: req.Importance,
	})
	if err != nil {
		writeAgentMemoryError(w, err)
		return
	}

	writeSuccess(w, stdhttp.StatusOK, memory)
}

func (h agentMemoriesHandler) deleteMemory(w stdhttp.ResponseWriter, r *stdhttp.Request, memoryID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if h.service == nil {
		writeError(w, stdhttp.StatusServiceUnavailable, "service_unavailable", "agent memory service is not configured")
		return
	}

	if err := h.service.DeleteMemory(r.Context(), session, memoryID); err != nil {
		writeAgentMemoryError(w, err)
		return
	}

	w.WriteHeader(stdhttp.StatusNoContent)
}

func writeAgentMemoryError(w stdhttp.ResponseWriter, err error) {
	switch err.Error() {
	case "agent not found", "memory not found":
		writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
	case "access denied":
		writeError(w, stdhttp.StatusForbidden, "forbidden", err.Error())
	case "content is required", "importance must be between 1 and 5":
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
	default:
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
	}
}
