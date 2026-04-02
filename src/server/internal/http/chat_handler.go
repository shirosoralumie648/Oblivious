package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/chat"
)

type chatHandler struct {
	service *chat.Service
}

type createConversationRequest struct {
	Title string `json:"title"`
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

type updateConversationConfigRequest struct {
	ModelID              string  `json:"modelId"`
	SystemPromptOverride string  `json:"systemPromptOverride"`
	Temperature          float64 `json:"temperature"`
	MaxOutputTokens      int     `json:"maxOutputTokens"`
	ToolsEnabled         bool    `json:"toolsEnabled"`
}

func newChatHandler(service *chat.Service) chatHandler {
	return chatHandler{service: service}
}

func (h chatHandler) createConversation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	conversation, err := h.service.CreateConversation(r.Context(), session, strings.TrimSpace(payload.Title))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create conversation failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, conversation)
}

func (h chatHandler) listConversations(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	conversations, err := h.service.ListConversations(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list conversations failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, conversations)
}

func (h chatHandler) listModels(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	writeSuccess(w, stdhttp.StatusOK, h.service.ListModels())
}

func (h chatHandler) listMessages(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	messages, err := h.service.ListMessages(r.Context(), session, conversationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list messages failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, messages)
}

func (h chatHandler) getConversationConfig(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	config, err := h.service.GetConversationConfig(r.Context(), session, conversationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get conversation config failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, config)
}

func (h chatHandler) updateConversationConfig(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload updateConversationConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	config, err := h.service.UpdateConversationConfig(
		r.Context(),
		session,
		conversationID,
		strings.TrimSpace(payload.ModelID),
		strings.TrimSpace(payload.SystemPromptOverride),
		payload.Temperature,
		payload.MaxOutputTokens,
		payload.ToolsEnabled,
	)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update conversation config failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, config)
}

func (h chatHandler) sendMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(payload.Content) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
		return
	}

	messages, err := h.service.SendMessage(r.Context(), session, conversationID, strings.TrimSpace(payload.Content))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "send message failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, messages)
}
