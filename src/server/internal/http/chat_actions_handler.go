package http

import (
	"encoding/json"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	"oblivious/server/internal/chat"
)

type updateConversationRequest struct {
	Title string `json:"title"`
}

type updateMessageRequest struct {
	Content string `json:"content"`
}

type bookmarkMessageRequest struct {
	Bookmarked *bool `json:"bookmarked"`
}

type createShareRequest struct {
	StartMessageID string `json:"startMessageId"`
	EndMessageID   string `json:"endMessageId"`
	ExpiresAt      string `json:"expiresAt"`
}

func (h chatHandler) getConversation(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	conversation, err := h.service.GetConversation(r.Context(), session, conversationID)
	if err != nil {
		writeChatActionError(w, err, "get conversation failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, conversation)
}

func (h chatHandler) updateConversation(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var payload updateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	conversation, err := h.service.UpdateConversation(r.Context(), session, conversationID, strings.TrimSpace(payload.Title))
	if err != nil {
		writeChatActionError(w, err, "update conversation failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, conversation)
}

func (h chatHandler) deleteConversation(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if err := h.service.DeleteConversation(r.Context(), session, conversationID); err != nil {
		writeChatActionError(w, err, "delete conversation failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h chatHandler) updateMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID, messageID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var payload updateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(payload.Content) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "content is required")
		return
	}
	message, err := h.service.UpdateMessage(r.Context(), session, conversationID, messageID, strings.TrimSpace(payload.Content))
	if err != nil {
		writeChatActionError(w, err, "update message failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, message)
}

func (h chatHandler) deleteMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID, messageID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if err := h.service.DeleteMessage(r.Context(), session, conversationID, messageID); err != nil {
		writeChatActionError(w, err, "delete message failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}

func (h chatHandler) bookmarkMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID, messageID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	bookmarked := true
	var payload bookmarkMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err == nil && payload.Bookmarked != nil {
		bookmarked = *payload.Bookmarked
	}
	message, err := h.service.BookmarkMessage(r.Context(), session, conversationID, messageID, bookmarked)
	if err != nil {
		writeChatActionError(w, err, "bookmark message failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, message)
}

func (h chatHandler) createMessageShare(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID, messageID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var payload createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	expiresAt, ok := parseOptionalShareExpiration(w, payload.ExpiresAt)
	if !ok {
		return
	}
	share, err := h.service.CreateMessageShare(r.Context(), session, conversationID, messageID, expiresAt)
	if err != nil {
		writeChatActionError(w, err, "create message share failed")
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, share)
}

func (h chatHandler) getMessageShare(w stdhttp.ResponseWriter, r *stdhttp.Request, shareID string) {
	share, err := h.service.GetMessageShare(r.Context(), shareID, time.Now().UTC())
	if err != nil {
		writeChatActionError(w, err, "message share not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, share)
}

func (h chatHandler) createConversationShare(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var payload createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	expiresAt, ok := parseOptionalShareExpiration(w, payload.ExpiresAt)
	if !ok {
		return
	}
	share, err := h.service.CreateConversationShare(r.Context(), session, conversationID, chat.ConversationShareStoreOptions{
		StartMessageID: strings.TrimSpace(payload.StartMessageID),
		EndMessageID:   strings.TrimSpace(payload.EndMessageID),
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		writeChatActionError(w, err, "create conversation share failed")
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, share)
}

func (h chatHandler) getConversationShare(w stdhttp.ResponseWriter, r *stdhttp.Request, shareID string) {
	share, err := h.service.GetConversationShare(r.Context(), shareID, time.Now().UTC())
	if err != nil {
		writeChatActionError(w, err, "conversation share not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, share)
}

func (h chatHandler) exportConversationMarkdown(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	messages, err := h.service.ListMessages(r.Context(), session, conversationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "export conversation failed")
		return
	}
	var builder strings.Builder
	for _, message := range messages {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("## ")
		builder.WriteString(strings.Title(message.Role))
		builder.WriteString("\n\n")
		builder.WriteString(message.Content)
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, conversationID))
	w.WriteHeader(stdhttp.StatusOK)
	_, _ = w.Write([]byte(builder.String()))
}

func (h chatHandler) streamMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, conversationID string) {
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

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(stdhttp.StatusOK)
	flusher, _ := w.(stdhttp.Flusher)

	err := h.service.SendMessageStream(
		r.Context(),
		session,
		conversationID,
		strings.TrimSpace(payload.Content),
		toMessageOverrides(payload.Overrides),
		func(chunk string) error {
			if err := writeSSEFrame(w, "", chunk); err != nil {
				return err
			}
			if flusher != nil {
				flusher.Flush()
			}
			return nil
		},
	)
	if err != nil {
		_ = writeSSEFrame(w, "error", "send message failed")
		if flusher != nil {
			flusher.Flush()
		}
		return
	}
	_ = writeSSEFrame(w, "", "[DONE]")
	if flusher != nil {
		flusher.Flush()
	}
}

func writeSSEFrame(w stdhttp.ResponseWriter, event string, data string) error {
	if strings.TrimSpace(event) != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", strings.TrimSpace(event)); err != nil {
			return err
		}
	}
	normalized := strings.ReplaceAll(data, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}

func parseOptionalShareExpiration(w stdhttp.ResponseWriter, value string) (*time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "expiresAt must be RFC3339")
		return nil, false
	}
	parsed = parsed.UTC()
	return &parsed, true
}

func writeChatActionError(w stdhttp.ResponseWriter, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, chat.ErrUnsupportedChatAction):
		writeError(w, stdhttp.StatusNotImplemented, "not_implemented", "chat action is not supported")
	case errors.Is(err, chat.ErrMessageShareExpired):
		writeError(w, stdhttp.StatusGone, "share_expired", "share has expired")
	case isNotFoundError(err):
		writeError(w, stdhttp.StatusNotFound, "not_found", fallbackMessage)
	default:
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", fallbackMessage)
	}
}
