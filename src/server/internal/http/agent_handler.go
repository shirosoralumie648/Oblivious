package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/chat"
)

type agentHandler struct {
	service *agent.Service
}

func newAgentHandler(service *agent.Service) agentHandler {
	return agentHandler{service: service}
}

// POST /api/v1/app/agents
func (h agentHandler) createAgent(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agent.CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	ag, err := h.service.CreateAgent(r.Context(), session, &req)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, ag)
}

// GET /api/v1/app/agents/:id
func (h agentHandler) getAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	ag, err := h.service.GetAgent(r.Context(), session, id)
	if err != nil {
		if err.Error() == "agent not found" {
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

	writeSuccess(w, stdhttp.StatusOK, ag)
}

// GET /api/v1/app/agents
func (h agentHandler) listAgents(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	agents, err := h.service.ListAgents(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, agents)
}

// PUT /api/v1/app/agents/:id
func (h agentHandler) updateAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agent.UpdateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	ag, err := h.service.UpdateAgent(r.Context(), session, id, &req)
	if err != nil {
		if err.Error() == "agent not found" {
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

	writeSuccess(w, stdhttp.StatusOK, ag)
}

// DELETE /api/v1/app/agents/:id
func (h agentHandler) deleteAgent(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	err := h.service.DeleteAgent(r.Context(), session, id)
	if err != nil {
		if err.Error() == "agent not found" {
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

// POST /api/v1/app/agents/:id/conversations
func (h agentHandler) createConversation(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	conv, err := h.service.CreateConversation(r.Context(), session, agentID)
	if err != nil {
		if err.Error() == "agent not found" {
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

	writeSuccess(w, stdhttp.StatusCreated, conv)
}

// GET /api/v1/app/agents/:id/conversations
func (h agentHandler) listConversations(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	convs, err := h.service.ListConversations(r.Context(), session, agentID)
	if err != nil {
		if err.Error() == "agent not found" {
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

	writeSuccess(w, stdhttp.StatusOK, convs)
}

// GET /api/v1/app/agents/conversations/:id
func (h agentHandler) getConversation(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	conv, err := h.service.GetConversation(r.Context(), session, id)
	if err != nil {
		if err.Error() == "conversation not found" {
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

	writeSuccess(w, stdhttp.StatusOK, conv)
}

// DELETE /api/v1/app/agents/conversations/:id
func (h agentHandler) deleteConversation(w stdhttp.ResponseWriter, r *stdhttp.Request, id string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	err := h.service.DeleteConversation(r.Context(), session, id)
	if err != nil {
		if err.Error() == "conversation not found" {
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

type agentSendMessageRequest struct {
	Content string `json:"content"`
}

// POST /api/v1/app/agents/conversations/:id/messages
func (h agentHandler) sendMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentSendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
		return
	}

	msg, err := h.service.SendMessage(chat.WithRelayRequestMetadata(r.Context(), chat.RelayRequestMetadata{
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		WorkspaceID:    session.WorkspaceID,
		RequestID:      requestIDFromContext(r.Context()),
	}), session, conversationID, content)
	if err != nil {
		if err.Error() == "conversation not found" || err.Error() == "agent not found" {
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

	writeSuccess(w, stdhttp.StatusOK, msg)
}

// GET /api/v1/app/agents/conversations/:id/messages
func (h agentHandler) listMessages(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	messages, err := h.service.ListMessages(r.Context(), session, conversationID)
	if err != nil {
		if err.Error() == "conversation not found" {
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

	writeSuccess(w, stdhttp.StatusOK, messages)
}

// GET /api/v1/app/agents/:id/tools
func (h agentHandler) listAvailableTools(w stdhttp.ResponseWriter, r *stdhttp.Request, agentID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tools, err := h.service.ListAvailableTools(r.Context(), session, agentID)
	if err != nil {
		if err.Error() == "agent not found" {
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

	writeSuccess(w, stdhttp.StatusOK, tools)
}
