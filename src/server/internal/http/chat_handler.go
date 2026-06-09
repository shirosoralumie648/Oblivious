package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"strconv"
	"strings"

	"oblivious/server/internal/chat"
)

type chatHandler struct {
	service *chat.Service
}

type createConversationRequest struct {
	Title string `json:"title"`
}

type forkConversationRequest struct {
	SourceConversationID string `json:"sourceConversationId"`
	BranchFromMessageID  string `json:"branchFromMessageId"`
	Title                string `json:"title"`
}

type sendMessageOverridesRequest struct {
	ModelID              *string  `json:"modelId"`
	SystemPromptOverride *string  `json:"systemPromptOverride"`
	Temperature          *float64 `json:"temperature"`
	MaxOutputTokens      *int     `json:"maxOutputTokens"`
	ToolsEnabled         *bool    `json:"toolsEnabled"`
	PersonaID            *string  `json:"personaId"`
}

type sendMessageRequest struct {
	Content   string                       `json:"content"`
	Overrides *sendMessageOverridesRequest `json:"overrides"`
}

type updateConversationConfigRequest struct {
	ModelID              string   `json:"modelId"`
	KnowledgeBaseIDs     []string `json:"knowledgeBaseIds"`
	PersonaID            string   `json:"personaId"`
	SystemPromptOverride string   `json:"systemPromptOverride"`
	Temperature          float64  `json:"temperature"`
	MaxOutputTokens      int      `json:"maxOutputTokens"`
	ToolsEnabled         bool     `json:"toolsEnabled"`
}

type createPersonaRequest struct {
	Name               string   `json:"name"`
	Role               string   `json:"role"`
	Style              string   `json:"style"`
	Tone               string   `json:"tone"`
	Constraints        string   `json:"constraints"`
	OpeningMessage     string   `json:"openingMessage"`
	SuggestedQuestions []string `json:"suggestedQuestions"`
}

type searchMessagesRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type createAttachmentRequest struct {
	FileName    string `json:"fileName"`
	FileType    string `json:"fileType"`
	FileSize    int64  `json:"fileSize"`
	URL         string `json:"url"`
	Description string `json:"description"`
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

func (h chatHandler) forkConversation(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	h.forkConversationFromSource(w, r, "")
}

func (h chatHandler) forkConversationFromSource(w stdhttp.ResponseWriter, r *stdhttp.Request, sourceConversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload forkConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(sourceConversationID) == "" {
		sourceConversationID = payload.SourceConversationID
	}
	if strings.TrimSpace(sourceConversationID) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "sourceConversationId is required")
		return
	}
	if strings.TrimSpace(payload.BranchFromMessageID) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "branchFromMessageId is required")
		return
	}

	conversation, err := h.service.ForkConversation(
		r.Context(),
		session,
		strings.TrimSpace(sourceConversationID),
		strings.TrimSpace(payload.BranchFromMessageID),
		strings.TrimSpace(payload.Title),
	)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "fork conversation failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, conversation)
}

func (h chatHandler) listConversationBranches(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	branches, err := h.service.ListConversationBranches(r.Context(), session, conversationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list branches failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, branches)
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
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "conversation not found")
			return
		}
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
		payload.KnowledgeBaseIDs,
		strings.TrimSpace(payload.PersonaID),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "conversation not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update conversation config failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, config)
}

func (h chatHandler) convertConversationToTask(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	draft, err := h.service.ConvertConversationToTask(r.Context(), session, conversationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "convert conversation to task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, draft)
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

	messages, err := h.service.SendMessage(
		r.Context(),
		session,
		conversationID,
		strings.TrimSpace(payload.Content),
		toMessageOverrides(payload.Overrides),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "conversation not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "send message failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, messages)
}

func toMessageOverrides(payload *sendMessageOverridesRequest) *chat.MessageOverrides {
	if payload == nil {
		return nil
	}

	return &chat.MessageOverrides{
		ModelID:              payload.ModelID,
		SystemPromptOverride: payload.SystemPromptOverride,
		Temperature:          payload.Temperature,
		MaxOutputTokens:      payload.MaxOutputTokens,
		ToolsEnabled:         payload.ToolsEnabled,
		PersonaID:            payload.PersonaID,
	}
}

// --- Persona handlers ---

func (h chatHandler) createPersona(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createPersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "name is required")
		return
	}

	persona := chat.Persona{
		Name:               strings.TrimSpace(payload.Name),
		Role:               strings.TrimSpace(payload.Role),
		Style:              strings.TrimSpace(payload.Style),
		Tone:               strings.TrimSpace(payload.Tone),
		Constraints:        strings.TrimSpace(payload.Constraints),
		OpeningMessage:     strings.TrimSpace(payload.OpeningMessage),
		SuggestedQuestions: payload.SuggestedQuestions,
	}

	created, err := h.service.CreatePersona(r.Context(), session, persona)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create persona failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, created)
}

func (h chatHandler) getPersona(w stdhttp.ResponseWriter, r *stdhttp.Request, personaID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	persona, err := h.service.GetPersona(r.Context(), session, personaID)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "persona not found")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, persona)
}

func (h chatHandler) listPersonas(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	personas, err := h.service.ListPersonas(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list personas failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, personas)
}

func (h chatHandler) updatePersona(w stdhttp.ResponseWriter, r *stdhttp.Request, personaID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createPersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	persona := chat.Persona{
		Name:               strings.TrimSpace(payload.Name),
		Role:               strings.TrimSpace(payload.Role),
		Style:              strings.TrimSpace(payload.Style),
		Tone:               strings.TrimSpace(payload.Tone),
		Constraints:        strings.TrimSpace(payload.Constraints),
		OpeningMessage:     strings.TrimSpace(payload.OpeningMessage),
		SuggestedQuestions: payload.SuggestedQuestions,
	}

	updated, err := h.service.UpdatePersona(r.Context(), session, personaID, persona)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "persona not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update persona failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, updated)
}

func (h chatHandler) deletePersona(w stdhttp.ResponseWriter, r *stdhttp.Request, personaID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.DeletePersona(r.Context(), session, personaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "persona not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "delete persona failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

// --- Search handlers ---

func (h chatHandler) searchMessages(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "query parameter 'q' is required")
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results, err := h.service.SearchMessages(r.Context(), session, query, limit)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "search messages failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, results)
}

// --- Attachment handlers ---

func (h chatHandler) addMessageAttachment(w stdhttp.ResponseWriter, r *stdhttp.Request, messageID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createAttachmentRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(payload.FileName) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "fileName is required")
		return
	}
	if strings.TrimSpace(payload.URL) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "url is required")
		return
	}

	attachment := chat.MessageAttachment{
		MessageID:   messageID,
		FileName:    strings.TrimSpace(payload.FileName),
		FileType:    strings.TrimSpace(payload.FileType),
		FileSize:    payload.FileSize,
		URL:         strings.TrimSpace(payload.URL),
		Description: strings.TrimSpace(payload.Description),
	}

	created, err := h.service.AddMessageAttachment(r.Context(), session, attachment)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "add attachment failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, created)
}

func (h chatHandler) listMessageAttachments(w stdhttp.ResponseWriter, r *stdhttp.Request, messageID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	attachments, err := h.service.ListMessageAttachments(r.Context(), session, messageID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list attachments failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, attachments)
}
