package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"

	"oblivious/server/internal/chat"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/ws"
)

type chatHandler struct {
	notifyConversation func(conversationID, eventType string, payload any)
	service            *chat.Service
	authorities        releasecontract.RuntimeAuthorities
}

type createConversationRequest struct {
	Title string `json:"title"`
}

type forkConversationRequest struct {
	SourceConversationID string `json:"sourceConversationId"`
	BranchFromMessageID  string `json:"branchFromMessageId"`
	MessageID            string `json:"messageId"`
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

type modelOptionResponse struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	CapabilityID string `json:"capabilityId"`
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
	return chatHandler{
		notifyConversation: ws.NotifyConversation,
		service:            service,
	}
}

// newReadinessChatHandler binds model mutation/response mapping to the
// startup-built catalog authority. The legacy constructor remains available
// for isolated tests that do not exercise runtime dispatch.
func newReadinessChatHandler(service *chat.Service, authorities releasecontract.RuntimeAuthorities) (chatHandler, error) {
	if service == nil || !authorities.Valid() {
		return chatHandler{}, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "http.chat"}
	}
	return chatHandler{notifyConversation: ws.NotifyConversation, service: service, authorities: authorities}, nil
}

func (h chatHandler) resolveModel(ctx context.Context, modelID string) (releasecontract.CapabilityID, error) {
	if !h.authorities.Valid() {
		return "", nil
	}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return "", &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "modelId"}
	}
	return h.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, releasecontract.CatalogSubject{
		Kind: releasecontract.CatalogSubjectModel, ID: modelID, Runtime: releasecontract.CatalogRuntimeServerModel,
	}, releasecontract.BoundaryHTTP)
}

func writeChatReadinessError(w stdhttp.ResponseWriter, err error) {
	var readinessErr *releasecontract.ReadinessError
	if errors.As(err, &readinessErr) {
		status := stdhttp.StatusServiceUnavailable
		if readinessErr.Code == releasecontract.CodeCapabilityUnknown {
			status = stdhttp.StatusBadRequest
		}
		writeError(w, status, string(readinessErr.Code), "model is not available")
		return
	}
	writeError(w, stdhttp.StatusServiceUnavailable, "readiness_unavailable", "model is not available")
}

func decodeStrictChatMutation(r *stdhttp.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func (h chatHandler) publishChatMessagesSynced(sessionUserID, conversationID string, messages []chat.Message) {
	if h.notifyConversation == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	h.notifyConversation(conversationID, "chat_messages_synced", map[string]any{
		"conversationId": conversationID,
		"messages":       messages,
		"userId":         strings.TrimSpace(sessionUserID),
	})
}

func (h chatHandler) publishChatMessageUpdated(sessionUserID, conversationID string, message chat.Message) {
	if h.notifyConversation == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	h.notifyConversation(conversationID, "chat_message_updated", map[string]any{
		"conversationId": conversationID,
		"message":        message,
		"messageId":      message.ID,
		"userId":         strings.TrimSpace(sessionUserID),
	})
}

func (h chatHandler) publishChatMessageDeleted(sessionUserID, conversationID, messageID string) {
	if h.notifyConversation == nil || strings.TrimSpace(conversationID) == "" || strings.TrimSpace(messageID) == "" {
		return
	}
	h.notifyConversation(conversationID, "chat_message_deleted", map[string]any{
		"conversationId": conversationID,
		"messageId":      strings.TrimSpace(messageID),
		"userId":         strings.TrimSpace(sessionUserID),
	})
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
	branchFromMessageID := strings.TrimSpace(payload.BranchFromMessageID)
	if branchFromMessageID == "" {
		branchFromMessageID = strings.TrimSpace(payload.MessageID)
	}
	if branchFromMessageID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "branchFromMessageId is required")
		return
	}

	conversation, err := h.service.ForkConversation(
		r.Context(),
		session,
		strings.TrimSpace(sourceConversationID),
		branchFromMessageID,
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

func (h chatHandler) listModels(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	models := h.service.ListModels()
	resolved := make([]modelOptionResponse, 0, len(models))
	for _, model := range models {
		capabilityID := releasecontract.CapabilityID("")
		if h.authorities.Valid() {
			var err error
			capabilityID, err = h.resolveModel(r.Context(), model.ID)
			if err != nil {
				// The catalog is the server-owned inventory. Unknown legacy aliases
				// are omitted rather than returning caller-selectable authority.
				continue
			}
		}
		resolved = append(resolved, modelOptionResponse{ID: model.ID, Label: model.Label, CapabilityID: string(capabilityID)})
	}
	writeSuccess(w, stdhttp.StatusOK, resolved)
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
	if err := decodeStrictChatMutation(r, &payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(payload.ModelID) != "" {
		if _, err := h.resolveModel(r.Context(), payload.ModelID); err != nil {
			writeChatReadinessError(w, err)
			return
		}
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
	if err := decodeStrictChatMutation(r, &payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if payload.Overrides != nil && payload.Overrides.ModelID != nil && strings.TrimSpace(*payload.Overrides.ModelID) != "" {
		if _, err := h.resolveModel(r.Context(), *payload.Overrides.ModelID); err != nil {
			writeChatReadinessError(w, err)
			return
		}
	}
	if strings.TrimSpace(payload.Content) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
		return
	}

	ctx := chat.WithRelayRequestMetadata(r.Context(), chat.RelayRequestMetadata{
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		WorkspaceID:    session.WorkspaceID,
		RequestID:      requestIDFromContext(r.Context()),
	})
	messages, err := h.service.SendMessage(
		ctx,
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

	h.publishChatMessagesSynced(session.User.ID, conversationID, messages)
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
