package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	publishingchannel "oblivious/server/internal/channel"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/observability"
)

const (
	channelHealthThreshold = 3
	channelDefaultLimit    = 20
	channelMaxLimit        = 100
)

var publishingChannelAlertSink observability.AlertSink
var publishingChannelAlertSinkMu sync.RWMutex
var publishingChannelRecoveryController *observability.RecoveryController
var publishingChannelRecoveryControllerMu sync.RWMutex

type publishingChannelStore interface {
	publishingchannel.Store
	publishingchannel.MessageLogStore
	publishingchannel.RetryWorkerStore
	CreateEvent(ctx context.Context, event notification.NotificationEvent) (*notification.Notification, error)
}

type channelHandler struct {
	store   publishingChannelStore
	service *publishingchannel.Service
}

type createChannelRequest struct {
	Type   publishingchannel.ChannelType   `json:"type"`
	Name   string                          `json:"name"`
	Config map[string]any                  `json:"config"`
	Status publishingchannel.ChannelStatus `json:"status"`
}

type channelStatusRequest struct {
	Status publishingchannel.ChannelStatus `json:"status"`
}

type sendChannelMessageRequest struct {
	Message channelMessageRequest `json:"message"`
}

type channelMessageRequest struct {
	ID             string                          `json:"id"`
	ConversationID string                          `json:"conversation_id"`
	Role           publishingchannel.Role          `json:"role"`
	Text           string                          `json:"text"`
	Content        []publishingchannel.ContentPart `json:"content"`
	Metadata       map[string]any                  `json:"metadata"`
	Timestamp      time.Time                       `json:"timestamp"`
}

type retryFailedChannelMessagesRequest struct {
	FallbackChannelID      string `json:"fallback_channel_id"`
	FallbackChannelIDCamel string `json:"fallbackChannelId"`
	Limit                  int    `json:"limit"`
}

func newChannelHandler(store publishingChannelStore, service *publishingchannel.Service) channelHandler {
	if service == nil {
		service = publishingchannel.NewService(publishingchannel.NewAdapterRegistry(nil))
	}
	return channelHandler{store: store, service: service}
}

func (h channelHandler) listChannels(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	configs, err := h.store.ListConfigs(r.Context(), session.OrganizationID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list channels failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, configs)
}

func (h channelHandler) getChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	config, ok := h.requireSessionChannel(w, r, channelID)
	if !ok {
		return
	}
	writeSuccess(w, stdhttp.StatusOK, config)
}

func (h channelHandler) createChannel(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var payload createChannelRequest
	if !decodeChannelJSON(w, r, &payload) {
		return
	}
	if payload.Status == "" {
		payload.Status = publishingchannel.ChannelStatusActive
	}
	created, err := h.store.CreateConfig(r.Context(), &publishingchannel.ChannelConfig{
		OrganizationID: session.OrganizationID,
		Type:           payload.Type,
		Name:           strings.TrimSpace(payload.Name),
		Config:         payload.Config,
		Status:         payload.Status,
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, created)
}

func (h channelHandler) updateChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var payload createChannelRequest
	if !decodeChannelJSON(w, r, &payload) {
		return
	}
	updated, err := h.store.UpdateConfig(r.Context(), session.OrganizationID, channelID, publishingchannel.ConfigUpdate{
		Type:   payload.Type,
		Name:   strings.TrimSpace(payload.Name),
		Config: payload.Config,
		Status: payload.Status,
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update channel failed")
		return
	}
	if updated == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, updated)
}

func (h channelHandler) deleteChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	h.setChannelStatus(w, r, channelID, publishingchannel.ChannelStatusDisabled)
}

func (h channelHandler) updateChannelStatus(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	var payload channelStatusRequest
	if !decodeChannelJSON(w, r, &payload) {
		return
	}
	h.setChannelStatus(w, r, channelID, payload.Status)
}

func (h channelHandler) testChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	config, ok := h.requireSessionChannel(w, r, channelID)
	if !ok {
		return
	}
	if config.Status == publishingchannel.ChannelStatusDisabled {
		writeError(w, stdhttp.StatusConflict, "channel_disabled", "channel is disabled")
		return
	}
	result, err := h.service.Test(r.Context(), *config)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h channelHandler) receiveWebhook(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	config, err := h.store.GetConfigByID(r.Context(), strings.TrimSpace(channelID))
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get channel failed")
		return
	}
	if config == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel not found")
		return
	}
	if config.Status == publishingchannel.ChannelStatusDisabled {
		writeError(w, stdhttp.StatusConflict, "channel_disabled", "channel is disabled")
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "read request body failed")
		return
	}
	if secret := channelSecret(config.Config); secret != "" {
		timestamp := r.Header.Get("X-Oblivious-Timestamp")
		signature := r.Header.Get("X-Oblivious-Signature")
		if !validWorkflowWebhookSignature(signature, secret, timestamp, raw) {
			writeError(w, stdhttp.StatusUnauthorized, "invalid_signature", "invalid webhook signature")
			return
		}
	}
	logEntry, err := h.service.Receive(r.Context(), publishingchannel.ReceiveRequest{
		ChannelID: config.ID,
		Type:      config.Type,
		Raw:       append(json.RawMessage(nil), raw...),
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	stored, err := h.store.RecordMessageLog(r.Context(), &logEntry)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record channel message failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, stored)
}

func (h channelHandler) listChannelMessages(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	if _, ok := h.requireSessionChannel(w, r, channelID); !ok {
		return
	}
	logs, err := h.store.ListMessageLogs(r.Context(), channelID, publishingchannel.ListMessageLogsInput{Limit: channelRequestLimit(r)})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list channel messages failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, logs)
}

func (h channelHandler) listFailedChannelMessages(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	if _, ok := h.requireSessionChannel(w, r, channelID); !ok {
		return
	}
	logs, err := h.store.ListFailedMessageLogs(r.Context(), channelID, publishingchannel.ListMessageLogsInput{Limit: channelRequestLimit(r)})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list failed channel messages failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, logs)
}

func (h channelHandler) retryFailedChannelMessages(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	if _, ok := h.requireSessionChannel(w, r, channelID); !ok {
		return
	}
	session, _ := sessionFromContext(r)
	var payload retryFailedChannelMessagesRequest
	if !decodeChannelJSON(w, r, &payload) {
		return
	}
	fallbackChannelID := strings.TrimSpace(firstNonEmpty(payload.FallbackChannelID, payload.FallbackChannelIDCamel))
	if fallbackChannelID != "" {
		fallback, err := h.store.GetConfig(r.Context(), session.OrganizationID, fallbackChannelID)
		if err != nil {
			writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get fallback channel failed")
			return
		}
		if fallback == nil {
			writeError(w, stdhttp.StatusNotFound, "fallback_channel_not_found", "fallback channel not found")
			return
		}
		if fallback.Status == publishingchannel.ChannelStatusDisabled {
			writeError(w, stdhttp.StatusConflict, "fallback_channel_disabled", "fallback channel is disabled")
			return
		}
	}
	result, err := h.service.ProcessDueRetryMessages(r.Context(), h.store, publishingchannel.ClaimDueRetryMessagesInput{
		ChannelID:         channelID,
		FallbackChannelID: fallbackChannelID,
		Limit:             payload.Limit,
		Now:               time.Now().UTC(),
	})
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "retry failed messages failed")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h channelHandler) sendChannelMessage(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) {
	config, ok := h.requireSessionChannel(w, r, channelID)
	if !ok {
		return
	}
	if config.Status == publishingchannel.ChannelStatusDisabled {
		writeError(w, stdhttp.StatusConflict, "channel_disabled", "channel is disabled")
		return
	}
	var payload sendChannelMessageRequest
	if !decodeChannelJSON(w, r, &payload) {
		return
	}
	message := payload.Message.toInternalMessage()
	logEntry, err := h.service.Send(r.Context(), publishingchannel.SendRequest{
		ChannelID: config.ID,
		Type:      config.Type,
		Config:    config.Config,
		Message:   message,
	})
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	stored, err := h.store.RecordMessageLog(r.Context(), &logEntry)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "record channel message failed")
		return
	}
	h.updateChannelHealthAfterSend(r, config, stored)
	writeSuccess(w, stdhttp.StatusOK, stored)
}

func (h channelHandler) setChannelStatus(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string, status publishingchannel.ChannelStatus) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	updated, err := h.store.UpdateConfigStatus(r.Context(), session.OrganizationID, channelID, status)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update channel status failed")
		return
	}
	if updated == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel not found")
		return
	}
	writeSuccess(w, stdhttp.StatusOK, updated)
}

func (h channelHandler) requireSessionChannel(w stdhttp.ResponseWriter, r *stdhttp.Request, channelID string) (*publishingchannel.ChannelConfig, bool) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return nil, false
	}
	config, err := h.store.GetConfig(r.Context(), session.OrganizationID, channelID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get channel failed")
		return nil, false
	}
	if config == nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", "channel not found")
		return nil, false
	}
	return config, true
}

func (h channelHandler) updateChannelHealthAfterSend(r *stdhttp.Request, config *publishingchannel.ChannelConfig, logEntry *publishingchannel.ChannelMessageLog) {
	if config == nil || logEntry == nil {
		return
	}
	if !logEntry.TransformSuccess || logEntry.Status == publishingchannel.MessageStatusRetryPending || logEntry.Status == publishingchannel.MessageStatusPermanentFailure {
		count, err := h.store.CountConsecutiveDeliveryFailures(r.Context(), config.ID, channelHealthThreshold)
		if err == nil && count >= channelHealthThreshold && config.Status != publishingchannel.ChannelStatusDegraded {
			if _, updateErr := h.store.UpdateConfigStatus(r.Context(), config.OrganizationID, config.ID, publishingchannel.ChannelStatusDegraded); updateErr == nil {
				h.notifyChannelDegraded(r, config, logEntry)
			}
		}
		return
	}
	if config.Status == publishingchannel.ChannelStatusDegraded {
		count, err := h.store.CountConsecutiveSuccessfulDeliveries(r.Context(), config.ID, channelHealthThreshold)
		if err == nil && count >= channelHealthThreshold {
			_, _ = h.store.UpdateConfigStatus(r.Context(), config.OrganizationID, config.ID, publishingchannel.ChannelStatusActive)
		}
	}
}

func (h channelHandler) notifyChannelDegraded(r *stdhttp.Request, config *publishingchannel.ChannelConfig, logEntry *publishingchannel.ChannelMessageLog) {
	session, ok := sessionFromContext(r)
	if !ok || h.store == nil {
		return
	}
	reason := strings.TrimSpace(logEntry.FailureReason)
	if reason == "" {
		reason = strings.TrimSpace(logEntry.TransformError)
	}
	_, _ = h.store.CreateEvent(r.Context(), notification.NotificationEvent{
		UserID:    session.User.ID,
		Type:      "warning",
		Category:  "system",
		Title:     "Publishing channel degraded",
		Message:   "Publishing channel delivery failed repeatedly.",
		ActionURL: "/workspace/publishing-channels",
		Metadata: map[string]any{
			"channelID":     config.ID,
			"failureReason": reason,
		},
	})
	routePublishingChannelDegradedAlert(r.Context(), config, logEntry, reason)
}

func setPublishingChannelAlertSinkForTest(sink observability.AlertSink) func() {
	return setPublishingChannelAlertSink(sink)
}

func setPublishingChannelAlertSink(sink observability.AlertSink) func() {
	publishingChannelAlertSinkMu.Lock()
	previous := publishingChannelAlertSink
	publishingChannelAlertSink = sink
	publishingChannelAlertSinkMu.Unlock()

	return func() {
		publishingChannelAlertSinkMu.Lock()
		publishingChannelAlertSink = previous
		publishingChannelAlertSinkMu.Unlock()
	}
}

func currentPublishingChannelAlertSink() observability.AlertSink {
	publishingChannelAlertSinkMu.RLock()
	defer publishingChannelAlertSinkMu.RUnlock()
	return publishingChannelAlertSink
}

func setPublishingChannelRecoveryControllerForTest(controller *observability.RecoveryController) func() {
	return setPublishingChannelRecoveryController(controller)
}

func setPublishingChannelRecoveryController(controller *observability.RecoveryController) func() {
	publishingChannelRecoveryControllerMu.Lock()
	previous := publishingChannelRecoveryController
	publishingChannelRecoveryController = controller
	publishingChannelRecoveryControllerMu.Unlock()

	return func() {
		publishingChannelRecoveryControllerMu.Lock()
		publishingChannelRecoveryController = previous
		publishingChannelRecoveryControllerMu.Unlock()
	}
}

func currentPublishingChannelRecoveryController() *observability.RecoveryController {
	publishingChannelRecoveryControllerMu.RLock()
	defer publishingChannelRecoveryControllerMu.RUnlock()
	return publishingChannelRecoveryController
}

func routePublishingChannelDegradedAlert(ctx context.Context, config *publishingchannel.ChannelConfig, logEntry *publishingchannel.ChannelMessageLog, reason string) {
	alertSink := currentPublishingChannelAlertSink()
	recoveryController := currentPublishingChannelRecoveryController()
	if alertSink == nil && recoveryController == nil {
		return
	}
	event := publishingChannelDegradedAlertEvent(config, logEntry, reason)
	if alertSink != nil {
		_ = alertSink.Notify(ctx, event)
	}
	if recoveryController != nil {
		_, _ = recoveryController.HandleAlert(ctx, event)
	}
}

func publishingChannelDegradedAlertEvent(config *publishingchannel.ChannelConfig, logEntry *publishingchannel.ChannelMessageLog, reason string) observability.AlertEvent {
	channelID := ""
	organizationID := ""
	channelType := ""
	channelName := ""
	if config != nil {
		channelID = config.ID
		organizationID = config.OrganizationID
		channelType = string(config.Type)
		channelName = config.Name
	}
	if reason == "" && logEntry != nil {
		reason = strings.TrimSpace(logEntry.FailureReason)
		if reason == "" {
			reason = strings.TrimSpace(logEntry.TransformError)
		}
	}
	if reason == "" {
		reason = "publishing channel delivery failed repeatedly"
	}
	occurredAt := time.Now().UTC()
	if logEntry != nil && !logEntry.CreatedAt.IsZero() {
		occurredAt = logEntry.CreatedAt.UTC()
	}
	return observability.AlertEvent{
		Key:        fmt.Sprintf("publishing_channel:%s:%s:degraded", organizationID, channelID),
		Severity:   observability.AlertSeverityWarning,
		Title:      "Publishing channel degraded",
		Message:    reason,
		Component:  "publishing_channel",
		OccurredAt: occurredAt,
		Fields: map[string]any{
			"organization_id": organizationID,
			"channel_id":      channelID,
			"channel_type":    channelType,
			"channel_name":    channelName,
			"failure_reason":  reason,
			"source":          "publishing_channel.health",
		},
	}
}

func decodeChannelJSON(w stdhttp.ResponseWriter, r *stdhttp.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "request body is required")
			return false
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return false
	}
	return true
}

func (m channelMessageRequest) toInternalMessage() publishingchannel.InternalMessage {
	content := append([]publishingchannel.ContentPart(nil), m.Content...)
	text := strings.TrimSpace(m.Text)
	if text != "" && !channelContentHasText(content) {
		content = append([]publishingchannel.ContentPart{{Type: publishingchannel.ContentTypeText, Text: text}}, content...)
	}
	return publishingchannel.InternalMessage{
		ID:             strings.TrimSpace(m.ID),
		ConversationID: strings.TrimSpace(m.ConversationID),
		Role:           m.Role,
		Content:        content,
		Metadata:       m.Metadata,
		Timestamp:      m.Timestamp,
	}
}

func channelContentHasText(content []publishingchannel.ContentPart) bool {
	for _, part := range content {
		if part.Type == publishingchannel.ContentTypeText && strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}

func channelRequestLimit(r *stdhttp.Request) int {
	limit, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if err != nil || limit <= 0 {
		return channelDefaultLimit
	}
	if limit > channelMaxLimit {
		return channelMaxLimit
	}
	return limit
}

func channelSecret(config map[string]any) string {
	if config == nil {
		return ""
	}
	for _, key := range []string{"secret", "webhook_secret", "webhookSecret"} {
		if value, ok := config[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
