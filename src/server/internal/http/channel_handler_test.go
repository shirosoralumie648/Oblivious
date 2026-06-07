package http

import (
	"context"
	"encoding/json"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/channel"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/observability"
)

type publishingChannelFakeStore struct {
	configs            map[string]*channel.ChannelConfig
	createdConfig      *channel.ChannelConfig
	lastLog            *channel.ChannelMessageLog
	logs               []*channel.ChannelMessageLog
	notifications      []*notification.Notification
	organizationID     string
	requestedID        string
	updatedStatus      channel.ChannelStatus
	updatedConfig      *channel.ChannelConfig
	claimInput         channel.ClaimDueRetryMessagesInput
	updatedRetryLogs   []*channel.ChannelMessageLog
	consecutiveLimit   int
	consecutiveChannel string
	successLimit       int
	successChannel     string
}

func newPublishingChannelTestHandler(store *publishingChannelFakeStore) channelHandler {
	if store.configs == nil {
		store.configs = map[string]*channel.ChannelConfig{}
	}
	service := channel.NewServiceWithOptions(
		channel.NewAdapterRegistry(nil),
		channel.WithChannelHealthNotifier(publishingChannelHealthNotifier),
	)
	return newChannelHandler(store, service)
}

func (s *publishingChannelFakeStore) CreateConfig(ctx context.Context, config *channel.ChannelConfig) (*channel.ChannelConfig, error) {
	s.createdConfig = cloneChannelConfig(config)
	created := cloneChannelConfig(config)
	if created.ID == "" {
		created.ID = "channel_created"
	}
	if created.Status == "" {
		created.Status = channel.ChannelStatusActive
	}
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	created.CreatedAt = now
	created.UpdatedAt = now
	s.configs[created.ID] = created
	return cloneChannelConfig(created), nil
}

func (s *publishingChannelFakeStore) GetConfig(ctx context.Context, organizationID, id string) (*channel.ChannelConfig, error) {
	s.organizationID = organizationID
	s.requestedID = id
	config := s.configs[id]
	if config == nil || config.OrganizationID != organizationID {
		return nil, nil
	}
	return cloneChannelConfig(config), nil
}

func (s *publishingChannelFakeStore) GetConfigByID(ctx context.Context, id string) (*channel.ChannelConfig, error) {
	s.requestedID = id
	return cloneChannelConfig(s.configs[id]), nil
}

func (s *publishingChannelFakeStore) ListConfigs(ctx context.Context, organizationID string) ([]*channel.ChannelConfig, error) {
	s.organizationID = organizationID
	configs := make([]*channel.ChannelConfig, 0, len(s.configs))
	for _, config := range s.configs {
		if config.OrganizationID == organizationID {
			configs = append(configs, cloneChannelConfig(config))
		}
	}
	return configs, nil
}

func (s *publishingChannelFakeStore) UpdateConfigStatus(ctx context.Context, organizationID, id string, status channel.ChannelStatus) (*channel.ChannelConfig, error) {
	s.organizationID = organizationID
	s.requestedID = id
	s.updatedStatus = status
	config := s.configs[id]
	if config == nil || config.OrganizationID != organizationID {
		return nil, nil
	}
	updated := cloneChannelConfig(config)
	updated.Status = status
	updated.UpdatedAt = time.Date(2026, 6, 4, 12, 30, 0, 0, time.UTC)
	s.configs[id] = updated
	return cloneChannelConfig(updated), nil
}

func (s *publishingChannelFakeStore) UpdateConfig(ctx context.Context, organizationID, id string, update channel.ConfigUpdate) (*channel.ChannelConfig, error) {
	s.organizationID = organizationID
	s.requestedID = id
	config := s.configs[id]
	if config == nil || config.OrganizationID != organizationID {
		return nil, nil
	}
	updated := cloneChannelConfig(config)
	updated.Name = update.Name
	updated.Type = update.Type
	updated.Config = update.Config
	updated.Status = update.Status
	updated.UpdatedAt = time.Date(2026, 6, 4, 12, 45, 0, 0, time.UTC)
	s.configs[id] = updated
	s.updatedConfig = cloneChannelConfig(updated)
	return cloneChannelConfig(updated), nil
}

func (s *publishingChannelFakeStore) RecordMessageLog(ctx context.Context, log *channel.ChannelMessageLog) (*channel.ChannelMessageLog, error) {
	s.lastLog = cloneChannelMessageLog(log)
	stored := cloneChannelMessageLog(log)
	stored.ID = "log_1"
	s.logs = append(s.logs, cloneChannelMessageLog(stored))
	return stored, nil
}

func (s *publishingChannelFakeStore) ListMessageLogs(ctx context.Context, channelID string, input channel.ListMessageLogsInput) ([]*channel.ChannelMessageLog, error) {
	logs := make([]*channel.ChannelMessageLog, 0, len(s.logs))
	for _, log := range s.logs {
		if log.ChannelID == channelID {
			logs = append(logs, cloneChannelMessageLog(log))
		}
	}
	return logs, nil
}

func (s *publishingChannelFakeStore) ListFailedMessageLogs(ctx context.Context, channelID string, input channel.ListMessageLogsInput) ([]*channel.ChannelMessageLog, error) {
	logs := make([]*channel.ChannelMessageLog, 0, len(s.logs))
	for _, log := range s.logs {
		if log.ChannelID == channelID &&
			log.Direction == channel.DirectionOutbound &&
			(log.Status == channel.MessageStatusRetryPending || log.Status == channel.MessageStatusPermanentFailure) {
			logs = append(logs, cloneChannelMessageLog(log))
		}
	}
	return logs, nil
}

func (s *publishingChannelFakeStore) ListDueRetryMessages(ctx context.Context, input channel.ClaimDueRetryMessagesInput) ([]*channel.ChannelMessageLog, error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	logs := make([]*channel.ChannelMessageLog, 0, len(s.logs))
	for _, log := range s.logs {
		if log.ChannelID == input.ChannelID &&
			log.Direction == channel.DirectionOutbound &&
			log.Status == channel.MessageStatusRetryPending &&
			log.NextRetryAt != nil &&
			!log.NextRetryAt.After(now) {
			logs = append(logs, cloneChannelMessageLog(log))
		}
	}
	return logs, nil
}

func (s *publishingChannelFakeStore) ClaimDueRetryMessages(ctx context.Context, input channel.ClaimDueRetryMessagesInput) ([]*channel.ChannelMessageLog, error) {
	s.claimInput = input
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	claimed := make([]*channel.ChannelMessageLog, 0, len(s.logs))
	limit := input.Limit
	if limit <= 0 {
		limit = len(s.logs)
	}
	for index, log := range s.logs {
		if len(claimed) >= limit {
			break
		}
		if log.ChannelID != input.ChannelID ||
			log.Direction != channel.DirectionOutbound ||
			log.Status != channel.MessageStatusRetryPending ||
			log.NextRetryAt == nil ||
			log.NextRetryAt.After(now) {
			continue
		}
		updated := cloneChannelMessageLog(log)
		updated.Status = channel.MessageStatusSending
		s.logs[index] = updated
		claimed = append(claimed, cloneChannelMessageLog(updated))
	}
	return claimed, nil
}

func (s *publishingChannelFakeStore) UpdateRetryMessageLog(ctx context.Context, log *channel.ChannelMessageLog) (*channel.ChannelMessageLog, error) {
	stored := cloneChannelMessageLog(log)
	s.updatedRetryLogs = append(s.updatedRetryLogs, cloneChannelMessageLog(stored))
	for index, existing := range s.logs {
		if existing.ID == stored.ID {
			s.logs[index] = cloneChannelMessageLog(stored)
			return stored, nil
		}
	}
	s.logs = append(s.logs, cloneChannelMessageLog(stored))
	return stored, nil
}

func (s *publishingChannelFakeStore) CountConsecutiveDeliveryFailures(ctx context.Context, channelID string, limit int) (int, error) {
	s.consecutiveChannel = channelID
	s.consecutiveLimit = limit
	count := 0
	for i := len(s.logs) - 1; i >= 0; i-- {
		log := s.logs[i]
		if log.ChannelID != channelID || log.Direction != channel.DirectionOutbound {
			continue
		}
		if log.Status == channel.MessageStatusRetryPending || log.Status == channel.MessageStatusPermanentFailure || !log.TransformSuccess {
			count++
			if count >= limit {
				break
			}
			continue
		}
		break
	}
	return count, nil
}

func (s *publishingChannelFakeStore) CountConsecutiveSuccessfulDeliveries(ctx context.Context, channelID string, limit int) (int, error) {
	s.successChannel = channelID
	s.successLimit = limit
	count := 0
	for i := len(s.logs) - 1; i >= 0; i-- {
		log := s.logs[i]
		if log.ChannelID != channelID || log.Direction != channel.DirectionOutbound {
			continue
		}
		if log.Status == channel.MessageStatusRecorded && log.TransformSuccess {
			count++
			if count >= limit {
				break
			}
			continue
		}
		break
	}
	return count, nil
}

func (s *publishingChannelFakeStore) CreateEvent(ctx context.Context, event notification.NotificationEvent) (*notification.Notification, error) {
	created := &notification.Notification{
		ID:        "notification_1",
		UserID:    event.UserID,
		Type:      event.Type,
		Category:  event.Category,
		Title:     event.Title,
		Message:   event.Message,
		ActionURL: event.ActionURL,
		Metadata:  event.Metadata,
		CreatedAt: time.Date(2026, 6, 4, 13, 0, 0, 0, time.UTC),
	}
	s.notifications = append(s.notifications, created)
	return created, nil
}

func TestPublishingChannelHandlerListsTenantConfigs(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
		"channel_2": {ID: "channel_2", OrganizationID: "org_other", Type: channel.ChannelTypeWebhook, Name: "Other", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.listChannels(recorder, publishingChannelRequest(stdhttp.MethodGet, "/api/v1/channels", "", "org_1"))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data []channel.ChannelConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != "channel_1" {
		t.Fatalf("expected tenant-scoped channel list, got %+v", response.Data)
	}
	if store.organizationID != "org_1" {
		t.Fatalf("expected organization scope org_1, got %q", store.organizationID)
	}
}

func TestPublishingChannelHandlerCreatesConfigForSessionOrganization(t *testing.T) {
	store := &publishingChannelFakeStore{}
	handler := newPublishingChannelTestHandler(store)

	body := `{"type":"webhook","name":"Support Webhook","config":{"secret":"test-secret"}}`
	recorder := httptest.NewRecorder()
	handler.createChannel(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels", body, "org_1"))

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.createdConfig == nil {
		t.Fatal("expected config to be created")
	}
	if store.createdConfig.OrganizationID != "org_1" || store.createdConfig.Type != channel.ChannelTypeWebhook || store.createdConfig.Name != "Support Webhook" {
		t.Fatalf("unexpected created config: %+v", store.createdConfig)
	}
	if store.createdConfig.Config["secret"] != "test-secret" {
		t.Fatalf("expected config payload to be preserved, got %+v", store.createdConfig.Config)
	}
}

func TestPublishingChannelHandlerCreatesSupportedPublishingAdapterTypes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		channelType channel.ChannelType
	}{
		{name: "feishu", channelType: channel.ChannelType("feishu")},
		{name: "wechat", channelType: channel.ChannelType("wechat")},
		{name: "discord", channelType: channel.ChannelType("discord")},
		{name: "slack", channelType: channel.ChannelType("slack")},
		{name: "telegram", channelType: channel.ChannelType("telegram")},
		{name: "web_embed", channelType: channel.ChannelType("web_embed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &publishingChannelFakeStore{}
			handler := newPublishingChannelTestHandler(store)

			body := `{"type":"` + string(tc.channelType) + `","name":"` + tc.name + ` Channel","config":{"enabled":true}}`
			recorder := httptest.NewRecorder()
			handler.createChannel(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels", body, "org_1"))

			if recorder.Code != stdhttp.StatusCreated {
				t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if store.createdConfig == nil || store.createdConfig.Type != tc.channelType {
				t.Fatalf("expected created %q config, got %+v", tc.channelType, store.createdConfig)
			}
		})
	}
}

func TestPublishingChannelHandlerTestsNewThinSliceAdapterTypes(t *testing.T) {
	for _, channelType := range []channel.ChannelType{
		channel.ChannelType("slack"),
		channel.ChannelType("telegram"),
		channel.ChannelType("web_embed"),
	} {
		t.Run(string(channelType), func(t *testing.T) {
			store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
				"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channelType, Name: "Thin Slice", Status: channel.ChannelStatusActive},
			}}
			handler := newPublishingChannelTestHandler(store)

			recorder := httptest.NewRecorder()
			handler.testChannel(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/test", "", "org_1"), "channel_1")

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			var response struct {
				Data struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Data.Type != string(channelType) || response.Data.Status != "success" {
				t.Fatalf("unexpected adapter test response: %+v", response.Data)
			}
		})
	}
}

func TestPublishingChannelHandlerGetsConfigByID(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.getChannel(recorder, publishingChannelRequest(stdhttp.MethodGet, "/api/v1/channels/channel_1", "", "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.requestedID != "channel_1" {
		t.Fatalf("expected channel_1 lookup, got %q", store.requestedID)
	}
	var response struct {
		Data channel.ChannelConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID != "channel_1" {
		t.Fatalf("expected channel_1, got %+v", response.Data)
	}
}

func TestPublishingChannelHandlerListsMessageLogsAndFailedRetryQueue(t *testing.T) {
	nextRetryAt := time.Date(2026, 6, 4, 12, 45, 0, 0, time.UTC)
	store := &publishingChannelFakeStore{
		configs: map[string]*channel.ChannelConfig{
			"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusDegraded},
			"channel_2": {ID: "channel_2", OrganizationID: "org_2", Type: channel.ChannelTypeWebhook, Name: "Other", Status: channel.ChannelStatusActive},
		},
		logs: []*channel.ChannelMessageLog{
			{
				ID:                 "channel_message_recent",
				ChannelID:          "channel_1",
				ConversationID:     "conversation_1",
				Direction:          channel.DirectionInbound,
				RawMessage:         json.RawMessage(`{"text":"hello from webhook"}`),
				TransformedMessage: channel.InternalMessage{ID: "msg_in_1", ConversationID: "conversation_1", Role: channel.RoleUser, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "hello from webhook"}}},
				TransformSuccess:   true,
				Status:             channel.MessageStatusRecorded,
				CreatedAt:          time.Date(2026, 6, 4, 12, 40, 0, 0, time.UTC),
			},
			{
				ID:                 "channel_message_failed",
				ChannelID:          "channel_1",
				ConversationID:     "conversation_1",
				Direction:          channel.DirectionOutbound,
				RawMessage:         json.RawMessage(`{"text":"retry me"}`),
				TransformedMessage: channel.InternalMessage{ID: "msg_out_1", ConversationID: "conversation_1", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "retry me"}}},
				TransformSuccess:   false,
				TransformError:     "delivery failed",
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         2,
				FailureReason:      "upstream 503",
				NextRetryAt:        &nextRetryAt,
				CreatedAt:          time.Date(2026, 6, 4, 12, 39, 0, 0, time.UTC),
			},
			{
				ID:               "other_channel_message",
				ChannelID:        "channel_2",
				Direction:        channel.DirectionOutbound,
				RawMessage:       json.RawMessage(`{"text":"hidden"}`),
				TransformSuccess: true,
				Status:           channel.MessageStatusRecorded,
				CreatedAt:        time.Date(2026, 6, 4, 12, 41, 0, 0, time.UTC),
			},
		},
	}
	handler := newPublishingChannelTestHandler(store)

	messages := httptest.NewRecorder()
	handler.listChannelMessages(messages, publishingChannelRequest(stdhttp.MethodGet, "/api/v1/channels/channel_1/messages", "", "org_1"), "channel_1")
	if messages.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", messages.Code, messages.Body.String())
	}
	var messagesResponse struct {
		Data []channel.ChannelMessageLog `json:"data"`
	}
	if err := json.Unmarshal(messages.Body.Bytes(), &messagesResponse); err != nil {
		t.Fatalf("decode message log response: %v", err)
	}
	if len(messagesResponse.Data) != 2 || messagesResponse.Data[0].ID != "channel_message_recent" || messagesResponse.Data[1].RawMessage == nil {
		t.Fatalf("expected channel-scoped message logs with raw payloads, got %+v", messagesResponse.Data)
	}

	failures := httptest.NewRecorder()
	handler.listFailedChannelMessages(failures, publishingChannelRequest(stdhttp.MethodGet, "/api/v1/channels/channel_1/failed-messages", "", "org_1"), "channel_1")
	if failures.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", failures.Code, failures.Body.String())
	}
	var failuresResponse struct {
		Data []channel.ChannelMessageLog `json:"data"`
	}
	if err := json.Unmarshal(failures.Body.Bytes(), &failuresResponse); err != nil {
		t.Fatalf("decode failed message response: %v", err)
	}
	if len(failuresResponse.Data) != 1 {
		t.Fatalf("expected one failed retry message, got %+v", failuresResponse.Data)
	}
	failed := failuresResponse.Data[0]
	if failed.ID != "channel_message_failed" ||
		failed.Direction != channel.DirectionOutbound ||
		failed.Status != channel.MessageStatusRetryPending ||
		failed.RetryCount != 2 ||
		failed.FailureReason != "upstream 503" ||
		failed.RawMessage == nil ||
		failed.NextRetryAt == nil {
		t.Fatalf("expected failed retry metadata to be visible, got %+v", failed)
	}
}

func TestPublishingChannelHandlerRetriesFailedMessagesThroughFallbackChannel(t *testing.T) {
	now := time.Now().UTC().Add(-time.Minute)
	store := &publishingChannelFakeStore{
		configs: map[string]*channel.ChannelConfig{
			"channel_primary": {
				ID:             "channel_primary",
				OrganizationID: "org_1",
				Type:           channel.ChannelTypeWebhook,
				Name:           "Primary",
				Config:         map[string]any{"webhook_url": "https://primary.example/retry"},
				Status:         channel.ChannelStatusDegraded,
			},
			"channel_fallback": {
				ID:             "channel_fallback",
				OrganizationID: "org_1",
				Type:           channel.ChannelTypeWebhook,
				Name:           "Fallback",
				Config:         map[string]any{"webhook_url": "https://fallback.example/retry"},
				Status:         channel.ChannelStatusActive,
			},
			"channel_other": {
				ID:             "channel_other",
				OrganizationID: "org_1",
				Type:           channel.ChannelTypeWebhook,
				Name:           "Other",
				Config:         map[string]any{"webhook_url": "https://other.example/retry"},
				Status:         channel.ChannelStatusActive,
			},
		},
		logs: []*channel.ChannelMessageLog{
			{
				ID:                 "channel_message_primary_due",
				ChannelID:          "channel_primary",
				ConversationID:     "conversation_1",
				Direction:          channel.DirectionOutbound,
				TransformedMessage: channel.InternalMessage{ID: "msg_primary_due", ConversationID: "conversation_1", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "retry on fallback"}}},
				TransformSuccess:   false,
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         2,
				FailureReason:      "primary degraded",
				NextRetryAt:        &now,
				CreatedAt:          now.Add(-time.Minute),
			},
			{
				ID:                 "channel_message_other_due",
				ChannelID:          "channel_other",
				Direction:          channel.DirectionOutbound,
				TransformedMessage: channel.InternalMessage{ID: "msg_other_due", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "do not claim"}}},
				TransformSuccess:   false,
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         1,
				NextRetryAt:        &now,
				CreatedAt:          now.Add(-2 * time.Minute),
			},
		},
	}
	handler := newPublishingChannelTestHandler(store)

	body := `{"fallback_channel_id":"channel_fallback","limit":10}`
	recorder := httptest.NewRecorder()
	handler.retryFailedChannelMessages(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_primary/retry-failed-messages", body, "org_1"), "channel_primary")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.claimInput.ChannelID != "channel_primary" || store.claimInput.FallbackChannelID != "channel_fallback" || store.claimInput.Limit != 10 {
		t.Fatalf("expected retry claim scoped to primary with fallback, got %+v", store.claimInput)
	}
	if len(store.updatedRetryLogs) != 1 {
		t.Fatalf("expected one updated retry log, got %+v", store.updatedRetryLogs)
	}
	if store.updatedRetryLogs[0].ID != "channel_message_primary_due" ||
		store.updatedRetryLogs[0].ChannelID != "channel_fallback" ||
		store.updatedRetryLogs[0].Status != channel.MessageStatusRetryPending {
		t.Fatalf("expected primary due message to be retried through fallback only, got %+v", store.updatedRetryLogs[0])
	}
	for _, log := range store.logs {
		if log.ID == "channel_message_other_due" && log.Status != channel.MessageStatusRetryPending {
			t.Fatalf("expected other channel due message to remain pending, got %+v", log)
		}
	}

	var response struct {
		Data channel.RetryProcessResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}
	if response.Data.Claimed != 1 || response.Data.Failed != 1 || response.Data.Succeeded != 0 {
		t.Fatalf("unexpected retry process result: %+v", response.Data)
	}
}

func TestPublishingChannelHandlerRetryFailedMessagesRoutesDegradedAlert(t *testing.T) {
	now := time.Now().UTC().Add(-time.Minute)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusServiceUnavailable)
	}))
	defer server.Close()
	store := &publishingChannelFakeStore{
		configs: map[string]*channel.ChannelConfig{
			"channel_primary": {
				ID:             "channel_primary",
				OrganizationID: "org_1",
				Type:           channel.ChannelTypeWebhook,
				Name:           "Primary",
				Config:         map[string]any{"webhook_url": server.URL},
				Status:         channel.ChannelStatusActive,
			},
		},
		logs: []*channel.ChannelMessageLog{
			{
				ID:                 "channel_message_primary_due",
				ChannelID:          "channel_primary",
				ConversationID:     "conversation_1",
				Direction:          channel.DirectionOutbound,
				TransformedMessage: channel.InternalMessage{ID: "msg_primary_due", ConversationID: "conversation_1", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "retry primary"}}},
				TransformSuccess:   false,
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         2,
				FailureReason:      "primary degraded",
				NextRetryAt:        &now,
				CreatedAt:          now.Add(-time.Minute),
			},
			{
				ID:                 "channel_message_previous_failure_1",
				ChannelID:          "channel_primary",
				ConversationID:     "conversation_1",
				Direction:          channel.DirectionOutbound,
				TransformedMessage: channel.InternalMessage{ID: "msg_previous_failure_1", ConversationID: "conversation_1", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "previous failure"}}},
				TransformSuccess:   false,
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         1,
				FailureReason:      "prior upstream 503",
				NextRetryAt:        nil,
				CreatedAt:          now.Add(-2 * time.Minute),
			},
			{
				ID:                 "channel_message_previous_failure_2",
				ChannelID:          "channel_primary",
				ConversationID:     "conversation_1",
				Direction:          channel.DirectionOutbound,
				TransformedMessage: channel.InternalMessage{ID: "msg_previous_failure_2", ConversationID: "conversation_1", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "previous failure"}}},
				TransformSuccess:   false,
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         1,
				FailureReason:      "prior upstream 503",
				NextRetryAt:        nil,
				CreatedAt:          now.Add(-3 * time.Minute),
			},
		},
	}
	alertStore := observability.NewInMemoryAlertStateStore()
	alertSink := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityWarning: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{alertSink},
		HistoryStore: alertStore,
	})
	restoreAlert := setPublishingChannelAlertSinkForTest(observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: alertStore,
		NotifySink: dispatcher,
	}))
	defer restoreAlert()
	restoreRecovery := setPublishingChannelRecoveryControllerForTest(observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: alertStore,
		Policies: []observability.RecoveryPolicy{
			{
				Name:       "record-channel-degraded",
				Severity:   observability.AlertSeverityWarning,
				Component:  "publishing_channel",
				ActionType: observability.RecoveryActionFailover,
			},
		},
	}))
	defer restoreRecovery()
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.retryFailedChannelMessages(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_primary/retry-failed-messages", `{"limit":10}`, "org_1"), "channel_primary")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.configs["channel_primary"].Status != channel.ChannelStatusDegraded {
		t.Fatalf("expected retry failure threshold to degrade primary channel, got %q", store.configs["channel_primary"].Status)
	}
	const alertKey = "publishing_channel:org_1:channel_primary:degraded"
	state, found, err := alertStore.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen {
		t.Fatalf("expected open retry endpoint degraded alert, found=%v state=%+v", found, state)
	}
	if len(alertSink.events) != 1 || alertSink.events[0].Key != alertKey || !strings.Contains(alertSink.events[0].Message, "503") {
		t.Fatalf("expected delivered retry endpoint degraded alert, got %+v", alertSink.events)
	}
	actions, err := alertStore.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].Type != observability.RecoveryActionFailover {
		t.Fatalf("expected retry endpoint recovery action, got %+v", actions)
	}
}

func TestPublishingChannelHandlerRetryFailedMessagesRejectsInvalidFallbackChannel(t *testing.T) {
	for _, tc := range []struct {
		name           string
		fallbackConfig *channel.ChannelConfig
		wantStatus     int
		wantCode       string
	}{
		{
			name:       "missing",
			wantStatus: stdhttp.StatusNotFound,
			wantCode:   "fallback_channel_not_found",
		},
		{
			name: "other organization",
			fallbackConfig: &channel.ChannelConfig{
				ID:             "channel_fallback",
				OrganizationID: "org_other",
				Type:           channel.ChannelTypeWebhook,
				Name:           "Other Org Fallback",
				Status:         channel.ChannelStatusActive,
			},
			wantStatus: stdhttp.StatusNotFound,
			wantCode:   "fallback_channel_not_found",
		},
		{
			name: "disabled",
			fallbackConfig: &channel.ChannelConfig{
				ID:             "channel_fallback",
				OrganizationID: "org_1",
				Type:           channel.ChannelTypeWebhook,
				Name:           "Disabled Fallback",
				Status:         channel.ChannelStatusDisabled,
			},
			wantStatus: stdhttp.StatusConflict,
			wantCode:   "fallback_channel_disabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configs := map[string]*channel.ChannelConfig{
				"channel_primary": {
					ID:             "channel_primary",
					OrganizationID: "org_1",
					Type:           channel.ChannelTypeWebhook,
					Name:           "Primary",
					Status:         channel.ChannelStatusDegraded,
				},
			}
			if tc.fallbackConfig != nil {
				configs[tc.fallbackConfig.ID] = tc.fallbackConfig
			}
			store := &publishingChannelFakeStore{configs: configs}
			handler := newPublishingChannelTestHandler(store)

			recorder := httptest.NewRecorder()
			handler.retryFailedChannelMessages(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_primary/retry-failed-messages", `{"fallback_channel_id":"channel_fallback"}`, "org_1"), "channel_primary")

			if recorder.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d with body %s", tc.wantStatus, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), tc.wantCode) {
				t.Fatalf("expected error code %q, got body %s", tc.wantCode, recorder.Body.String())
			}
			if store.claimInput.ChannelID != "" {
				t.Fatalf("expected invalid fallback not to claim retry queue, got %+v", store.claimInput)
			}
		})
	}
}

func TestPublishingChannelHandlerUpdatesStatus(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.updateChannelStatus(recorder, publishingChannelRequest(stdhttp.MethodPatch, "/api/v1/channels/channel_1/status", `{"status":"disabled"}`, "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.updatedStatus != channel.ChannelStatusDisabled {
		t.Fatalf("expected disabled status update, got %q", store.updatedStatus)
	}
	var response struct {
		Data channel.ChannelConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != channel.ChannelStatusDisabled {
		t.Fatalf("expected disabled response, got %+v", response.Data)
	}
}

func TestPublishingChannelHandlerUpdatesConfigWithinSessionOrganization(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Config: map[string]any{"old": true}, Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"type":"webhook","name":"Support Webhook","config":{"secret":"rotated"},"status":"degraded"}`
	recorder := httptest.NewRecorder()
	handler.updateChannel(recorder, publishingChannelRequest(stdhttp.MethodPut, "/api/v1/channels/channel_1", body, "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.organizationID != "org_1" || store.requestedID != "channel_1" {
		t.Fatalf("expected org-scoped update for channel_1, got org=%q id=%q", store.organizationID, store.requestedID)
	}
	if store.updatedConfig == nil || store.updatedConfig.Name != "Support Webhook" || store.updatedConfig.Status != channel.ChannelStatusDegraded {
		t.Fatalf("unexpected updated config: %+v", store.updatedConfig)
	}
	if store.updatedConfig.Config["secret"] != "rotated" {
		t.Fatalf("expected replacement config payload, got %+v", store.updatedConfig.Config)
	}
	var response struct {
		Data channel.ChannelConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Name != "Support Webhook" || response.Data.Status != channel.ChannelStatusDegraded {
		t.Fatalf("expected updated config response, got %+v", response.Data)
	}
}

func TestPublishingChannelHandlerDeleteSoftDisablesConfig(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.deleteChannel(recorder, publishingChannelRequest(stdhttp.MethodDelete, "/api/v1/channels/channel_1", "", "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.updatedStatus != channel.ChannelStatusDisabled {
		t.Fatalf("expected soft delete to set disabled, got %q", store.updatedStatus)
	}
	var response struct {
		Data channel.ChannelConfig `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != channel.ChannelStatusDisabled {
		t.Fatalf("expected disabled response, got %+v", response.Data)
	}
}

func TestPublishingChannelHandlerTestsActiveWebhookConfig(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(stdhttp.StatusNoContent)
	}))
	defer server.Close()

	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {
			ID:             "channel_1",
			OrganizationID: "org_1",
			Type:           channel.ChannelTypeWebhook,
			Name:           "Website",
			Config:         map[string]any{"webhook_url": server.URL},
			Status:         channel.ChannelStatusActive,
		},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.testChannel(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/test", "", "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			ChannelID string `json:"channel_id"`
			Status    string `json:"status"`
			Message   string `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ChannelID != "channel_1" || response.Data.Status != "success" || response.Data.Message == "" {
		t.Fatalf("unexpected test response: %+v", response.Data)
	}
}

func TestPublishingChannelHandlerTestsActiveFeishuConfig(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelType("feishu"), Name: "Feishu", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.testChannel(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/test", "", "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Type != "feishu" || response.Data.Status != "success" {
		t.Fatalf("unexpected feishu test response: %+v", response.Data)
	}
}

func TestPublishingChannelHandlerTestRejectsDisabledConfig(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusDisabled},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.testChannel(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/test", "", "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected 409 for disabled channel test, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestPublishingChannelHandlerReceivesWebhookThroughServiceAndRecordsLog(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"id":"msg_1","conversation_id":"conversation_1","role":"user","text":"hello from webhook","metadata":{"source":"test"}}`
	recorder := httptest.NewRecorder()
	handler.receiveWebhook(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/webhook/channel_1", strings.NewReader(body)), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog == nil {
		t.Fatal("expected webhook message log to be recorded")
	}
	if store.lastLog.ChannelID != "channel_1" || !store.lastLog.TransformSuccess || store.lastLog.TransformedMessage.ConversationID != "conversation_1" {
		t.Fatalf("unexpected webhook log: %+v", store.lastLog)
	}
	if string(store.lastLog.RawMessage) != body {
		t.Fatalf("expected raw body to be logged, got %s", store.lastLog.RawMessage)
	}
	var response struct {
		Data channel.ChannelMessageLog `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID != "log_1" || !response.Data.TransformSuccess {
		t.Fatalf("expected stored success log response, got %+v", response.Data)
	}
}

func TestPublishingChannelHandlerRejectsUnsignedWebhookWhenSecretConfigured(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive, Config: map[string]any{"secret": "top-secret"}},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.receiveWebhook(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/webhook/channel_1", strings.NewReader(`{"text":"blocked"}`)), "channel_1")

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401 for unsigned webhook, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog != nil {
		t.Fatalf("expected unsigned webhook not to be logged, got %+v", store.lastLog)
	}
}

func TestPublishingChannelHandlerAcceptsSignedWebhookWhenSecretConfigured(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive, Config: map[string]any{"secret": "top-secret"}},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"id":"msg_1","conversation_id":"conversation_1","role":"user","text":"hello from signed webhook"}`
	timestamp := time.Now().UTC().Format(time.RFC3339)
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/webhook/channel_1", strings.NewReader(body))
	request.Header.Set("X-Oblivious-Timestamp", timestamp)
	request.Header.Set("X-Oblivious-Signature", "sha256="+workflowWebhookSignature("top-secret", timestamp, body))

	recorder := httptest.NewRecorder()
	handler.receiveWebhook(recorder, request, "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog == nil || !store.lastLog.TransformSuccess {
		t.Fatalf("expected signed webhook to be logged successfully, got %+v", store.lastLog)
	}
}

func TestPublishingChannelHandlerReceivesWeChatJSONThroughServiceAndRecordsLog(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelType("wechat"), Name: "WeChat", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"MsgId":"wechat_msg_1","FromUserName":"user_openid","MsgType":"text","Content":"hello from wechat"}`
	recorder := httptest.NewRecorder()
	handler.receiveWebhook(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/webhook/channel_1", strings.NewReader(body)), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog == nil || !store.lastLog.TransformSuccess {
		t.Fatalf("expected wechat message log success, got %+v", store.lastLog)
	}
	if store.lastLog.TransformedMessage.ConversationID != "user_openid" ||
		len(store.lastLog.TransformedMessage.Content) != 1 ||
		store.lastLog.TransformedMessage.Content[0].Text != "hello from wechat" {
		t.Fatalf("unexpected wechat transformed message: %+v", store.lastLog.TransformedMessage)
	}
}

func TestPublishingChannelHandlerSendsOutboundMessageAndRecordsLog(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"message":{"id":"msg_out_1","conversation_id":"conversation_1","role":"assistant","text":"hello back","metadata":{"source":"agent"}}}`
	recorder := httptest.NewRecorder()
	handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", body, "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog == nil {
		t.Fatal("expected outbound message log to be recorded")
	}
	if store.lastLog.ChannelID != "channel_1" || store.lastLog.Direction != channel.DirectionOutbound || !store.lastLog.TransformSuccess {
		t.Fatalf("unexpected outbound log: %+v", store.lastLog)
	}
	if store.lastLog.TransformedMessage.ConversationID != "conversation_1" || store.lastLog.TransformedMessage.Content[0].Text != "hello back" {
		t.Fatalf("expected transformed outbound message content, got %+v", store.lastLog.TransformedMessage)
	}
	if !strings.Contains(string(store.lastLog.RawMessage), `"text":"hello back"`) {
		t.Fatalf("expected raw outbound payload to be logged, got %s", store.lastLog.RawMessage)
	}
	var response struct {
		Data channel.ChannelMessageLog `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID != "log_1" || response.Data.Direction != channel.DirectionOutbound {
		t.Fatalf("expected stored outbound log response, got %+v", response.Data)
	}
}

func TestPublishingChannelHandlerSendsWebhookOutboundToConfiguredURL(t *testing.T) {
	received := make(chan string, 1)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read webhook body: %v", err)
		}
		received <- string(body)
		w.WriteHeader(stdhttp.StatusOK)
	}))
	defer server.Close()

	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {
			ID:             "channel_1",
			OrganizationID: "org_1",
			Type:           channel.ChannelTypeWebhook,
			Name:           "Website",
			Config:         map[string]any{"webhook_url": server.URL},
			Status:         channel.ChannelStatusActive,
		},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"message":{"id":"msg_out_1","conversation_id":"conversation_1","role":"assistant","text":"hello webhook"}}`
	recorder := httptest.NewRecorder()
	handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", body, "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog == nil || store.lastLog.Status != channel.MessageStatusRecorded || !store.lastLog.TransformSuccess {
		t.Fatalf("expected recorded successful outbound log, got %+v", store.lastLog)
	}
	select {
	case posted := <-received:
		if !strings.Contains(posted, `"text":"hello webhook"`) {
			t.Fatalf("expected webhook post body to include transformed text, got %s", posted)
		}
	default:
		t.Fatal("expected configured webhook URL to receive outbound POST")
	}
}

func TestPublishingChannelHandlerSendWebhook5xxUsesRetryAndDegradedNotification(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusServiceUnavailable)
	}))
	defer server.Close()

	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {
			ID:             "channel_1",
			OrganizationID: "org_1",
			Type:           channel.ChannelTypeWebhook,
			Name:           "Website",
			Config:         map[string]any{"webhook_url": server.URL},
			Status:         channel.ChannelStatusActive,
		},
	}}
	handler := newPublishingChannelTestHandler(store)
	body := `{"message":{"id":"msg_out_1","conversation_id":"conversation_1","role":"assistant","text":"service unavailable"}}`

	for attempt := 1; attempt <= 3; attempt++ {
		recorder := httptest.NewRecorder()
		handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", body, "org_1"), "channel_1")

		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", attempt, recorder.Code, recorder.Body.String())
		}
		if store.lastLog == nil || store.lastLog.Status != channel.MessageStatusRetryPending || store.lastLog.TransformSuccess {
			t.Fatalf("attempt %d expected retry_pending failed delivery log, got %+v", attempt, store.lastLog)
		}
		if store.lastLog.FailureReason == "" || !strings.Contains(store.lastLog.FailureReason, "503") {
			t.Fatalf("attempt %d expected 503 failure reason, got %+v", attempt, store.lastLog)
		}
		if attempt < 3 && store.configs["channel_1"].Status != channel.ChannelStatusActive {
			t.Fatalf("attempt %d should not degrade channel before threshold, got %q", attempt, store.configs["channel_1"].Status)
		}
	}

	if store.configs["channel_1"].Status != channel.ChannelStatusDegraded || store.updatedStatus != channel.ChannelStatusDegraded {
		t.Fatalf("expected third consecutive webhook 5xx to mark degraded, config=%+v updated=%q", store.configs["channel_1"], store.updatedStatus)
	}
	if len(store.notifications) != 1 {
		t.Fatalf("expected degraded notification after third webhook 5xx, got %+v", store.notifications)
	}
	if !strings.Contains(store.notifications[0].Metadata["failureReason"].(string), "503") {
		t.Fatalf("expected notification failure reason to include 503, got %+v", store.notifications[0].Metadata)
	}
}

func TestPublishingChannelHandlerSendsDiscordOutboundMessageAndRecordsLog(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelType("discord"), Name: "Discord", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)

	body := `{"message":{"id":"msg_out_1","conversation_id":"discord_channel_1","role":"assistant","text":"hello discord","content":[{"type":"card","metadata":{"title":"Summary","description":"sent"}}]}}`
	recorder := httptest.NewRecorder()
	handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", body, "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog == nil || !store.lastLog.TransformSuccess || store.lastLog.Direction != channel.DirectionOutbound {
		t.Fatalf("unexpected discord outbound log: %+v", store.lastLog)
	}
	raw := string(store.lastLog.RawMessage)
	if !strings.Contains(raw, `"content":"hello discord"`) || !strings.Contains(raw, `"embeds"`) {
		t.Fatalf("expected discord raw outbound payload, got %s", store.lastLog.RawMessage)
	}
}

func TestPublishingChannelHandlerMarksChannelDegradedAndNotifiesAfterThreeConsecutiveSendFailures(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)
	body := `{"message":{"id":"msg_out_failure","conversation_id":"conversation_1","role":"assistant"}}`

	for attempt := 1; attempt <= 3; attempt++ {
		recorder := httptest.NewRecorder()
		handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", body, "org_1"), "channel_1")

		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", attempt, recorder.Code, recorder.Body.String())
		}
		if store.lastLog == nil || store.lastLog.TransformSuccess || !strings.Contains(store.lastLog.TransformError, "content is required") {
			t.Fatalf("attempt %d expected send transform failure log, got %+v", attempt, store.lastLog)
		}
		if attempt < 3 {
			if store.configs["channel_1"].Status != channel.ChannelStatusActive {
				t.Fatalf("attempt %d should not degrade channel, got %q", attempt, store.configs["channel_1"].Status)
			}
			if len(store.notifications) != 0 {
				t.Fatalf("attempt %d should not notify before threshold, got %+v", attempt, store.notifications)
			}
		}
	}

	if store.consecutiveChannel != "channel_1" || store.consecutiveLimit != 3 {
		t.Fatalf("expected consecutive failure check for channel_1 limit 3, got channel=%q limit=%d", store.consecutiveChannel, store.consecutiveLimit)
	}
	if store.configs["channel_1"].Status != channel.ChannelStatusDegraded || store.updatedStatus != channel.ChannelStatusDegraded {
		t.Fatalf("expected third consecutive failure to mark degraded, config=%+v updated=%q", store.configs["channel_1"], store.updatedStatus)
	}
	if len(store.notifications) != 1 {
		t.Fatalf("expected one operator notification after third failure, got %+v", store.notifications)
	}
	notification := store.notifications[0]
	if notification.UserID != "user_1" || notification.Type != "warning" || notification.Category != "system" {
		t.Fatalf("unexpected notification envelope: %+v", notification)
	}
	if notification.Metadata["channelID"] != "channel_1" || !strings.Contains(notification.Metadata["failureReason"].(string), "content is required") {
		t.Fatalf("expected channelID and failureReason notification metadata, got %+v", notification.Metadata)
	}
}

func TestPublishingChannelHandlerRoutesDegradedChannelToAlertDeliveryAndRecovery(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusActive},
	}}
	alertStore := observability.NewInMemoryAlertStateStore()
	alertSink := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityWarning: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{alertSink},
		HistoryStore: alertStore,
	})
	alertRouter := observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: alertStore,
		NotifySink: dispatcher,
	})
	recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: alertStore,
		Policies: []observability.RecoveryPolicy{
			{
				Name:       "record-channel-degraded",
				Severity:   observability.AlertSeverityWarning,
				Component:  "publishing_channel",
				ActionType: observability.RecoveryActionFailover,
			},
		},
	})
	restoreAlert := setPublishingChannelAlertSinkForTest(alertRouter)
	defer restoreAlert()
	restoreRecovery := setPublishingChannelRecoveryControllerForTest(recovery)
	defer restoreRecovery()

	handler := newPublishingChannelTestHandler(store)
	body := `{"message":{"id":"msg_out_failure","conversation_id":"conversation_1","role":"assistant"}}`

	for attempt := 1; attempt <= 3; attempt++ {
		recorder := httptest.NewRecorder()
		handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", body, "org_1"), "channel_1")
		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("attempt %d expected 200, got %d with body %s", attempt, recorder.Code, recorder.Body.String())
		}
	}

	const alertKey = "publishing_channel:org_1:channel_1:degraded"
	state, found, err := alertStore.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Component != "publishing_channel" {
		t.Fatalf("expected open publishing channel alert state, found=%v state=%+v", found, state)
	}
	if len(alertSink.events) != 1 || alertSink.events[0].Key != alertKey {
		t.Fatalf("expected one delivered channel degraded alert, got %+v", alertSink.events)
	}
	attempts, err := alertStore.ListDeliveryAttempts(context.Background(), observability.AlertDeliveryHistoryFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Channel != observability.AlertDeliveryChannelInApp || !attempts[0].Delivered {
		t.Fatalf("expected successful in-app delivery attempt, got %+v", attempts)
	}
	actions, err := alertStore.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].PolicyName != "record-channel-degraded" || actions[0].Type != observability.RecoveryActionFailover {
		t.Fatalf("expected recorded channel failover recovery action, got %+v", actions)
	}
}

func TestPublishingChannelHealthNotifierRoutesRetryWorkerDegradedEventToAlertDeliveryAndRecovery(t *testing.T) {
	alertStore := observability.NewInMemoryAlertStateStore()
	alertSink := &captureMiddlewareAlertSink{channel: observability.AlertDeliveryChannelInApp}
	dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
		Policy: observability.DeliveryPolicy{
			Routes: map[observability.AlertSeverity][]observability.AlertDeliveryChannel{
				observability.AlertSeverityWarning: {observability.AlertDeliveryChannelInApp},
			},
		},
		Sinks:        []observability.AlertDeliverySink{alertSink},
		HistoryStore: alertStore,
	})
	alertRouter := observability.NewAlertRouter(observability.AlertRouterOptions{
		StateStore: alertStore,
		NotifySink: dispatcher,
	})
	recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
		StateStore: alertStore,
		Policies: []observability.RecoveryPolicy{
			{
				Name:       "record-channel-degraded",
				Severity:   observability.AlertSeverityWarning,
				Component:  "publishing_channel",
				ActionType: observability.RecoveryActionFailover,
			},
		},
	})
	restoreAlert := setPublishingChannelAlertSinkForTest(alertRouter)
	defer restoreAlert()
	restoreRecovery := setPublishingChannelRecoveryControllerForTest(recovery)
	defer restoreRecovery()

	occurredAt := time.Date(2026, 6, 5, 12, 30, 0, 0, time.UTC)
	publishingChannelHealthNotifier(context.Background(), channel.ChannelHealthEvent{
		OrganizationID: "org_1",
		ChannelID:      "channel_retry_worker",
		ChannelName:    "Retry worker channel",
		ChannelType:    channel.ChannelTypeWebhook,
		Status:         channel.ChannelStatusDegraded,
		MessageLogID:   "channel_message_retry_worker",
		Reason:         "upstream 503",
		OccurredAt:     occurredAt,
	})

	const alertKey = "publishing_channel:org_1:channel_retry_worker:degraded"
	state, found, err := alertStore.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Component != "publishing_channel" {
		t.Fatalf("expected open retry worker publishing alert, found=%v state=%+v", found, state)
	}
	if !state.LastOccurredAt.Equal(occurredAt) {
		t.Fatalf("expected alert timestamp from retry worker event, got %s", state.LastOccurredAt)
	}
	if len(alertSink.events) != 1 || alertSink.events[0].Key != alertKey || alertSink.events[0].Message != "upstream 503" {
		t.Fatalf("expected one delivered retry worker channel degraded alert, got %+v", alertSink.events)
	}
	actions, err := alertStore.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].PolicyName != "record-channel-degraded" || actions[0].Type != observability.RecoveryActionFailover {
		t.Fatalf("expected retry worker channel failover recovery action, got %+v", actions)
	}
}

func TestPublishingChannelHandlerRecoversDegradedChannelAfterThreeConsecutiveSendSuccesses(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusDegraded},
	}}
	handler := newPublishingChannelTestHandler(store)
	successBody := `{"message":{"id":"msg_out_success","conversation_id":"conversation_1","role":"assistant","text":"hello back"}}`
	failureBody := `{"message":{"id":"msg_out_failure","conversation_id":"conversation_1","role":"assistant"}}`

	for attempt := 1; attempt <= 2; attempt++ {
		recorder := httptest.NewRecorder()
		handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", successBody, "org_1"), "channel_1")

		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("success attempt %d expected 200, got %d with body %s", attempt, recorder.Code, recorder.Body.String())
		}
		if store.configs["channel_1"].Status != channel.ChannelStatusDegraded {
			t.Fatalf("success attempt %d should not recover before threshold, got %q", attempt, store.configs["channel_1"].Status)
		}
	}

	recorder := httptest.NewRecorder()
	handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", failureBody, "org_1"), "channel_1")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("failure attempt expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.configs["channel_1"].Status != channel.ChannelStatusDegraded {
		t.Fatalf("failure should keep degraded status, got %q", store.configs["channel_1"].Status)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		recorder := httptest.NewRecorder()
		handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", successBody, "org_1"), "channel_1")

		if recorder.Code != stdhttp.StatusOK {
			t.Fatalf("post-failure success attempt %d expected 200, got %d with body %s", attempt, recorder.Code, recorder.Body.String())
		}
		if attempt < 3 && store.configs["channel_1"].Status != channel.ChannelStatusDegraded {
			t.Fatalf("post-failure success attempt %d should not recover before threshold, got %q", attempt, store.configs["channel_1"].Status)
		}
	}

	if store.successChannel != "channel_1" || store.successLimit != 3 {
		t.Fatalf("expected consecutive success check for channel_1 limit 3, got channel=%q limit=%d", store.successChannel, store.successLimit)
	}
	if store.configs["channel_1"].Status != channel.ChannelStatusActive || store.updatedStatus != channel.ChannelStatusActive {
		t.Fatalf("expected third consecutive success to recover active, config=%+v updated=%q", store.configs["channel_1"], store.updatedStatus)
	}
}

func TestPublishingChannelHandlerSendRejectsDisabledChannel(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusDisabled},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.sendChannelMessage(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", `{"message":{"conversation_id":"conversation_1","role":"assistant","text":"blocked"}}`, "org_1"), "channel_1")

	if recorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected 409 for disabled channel, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog != nil {
		t.Fatalf("expected disabled send not to be logged, got %+v", store.lastLog)
	}
}

func TestPublishingChannelHandlerRejectsDisabledWebhook(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Website", Status: channel.ChannelStatusDisabled},
	}}
	handler := newPublishingChannelTestHandler(store)

	recorder := httptest.NewRecorder()
	handler.receiveWebhook(recorder, httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/webhook/channel_1", strings.NewReader(`{"text":"blocked"}`)), "channel_1")

	if recorder.Code != stdhttp.StatusConflict {
		t.Fatalf("expected 409 for disabled channel, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastLog != nil {
		t.Fatalf("expected disabled webhook not to be logged, got %+v", store.lastLog)
	}
}

func publishingChannelRequest(method, path, body, organizationID string) *stdhttp.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), channelSessionContextKey, auth.Session{
		ID:             "session_1",
		OrganizationID: organizationID,
		User:           auth.User{ID: "user_1", Email: "user@example.com"},
		WorkspaceID:    "workspace_1",
	}))
	return request
}

func cloneChannelConfig(config *channel.ChannelConfig) *channel.ChannelConfig {
	if config == nil {
		return nil
	}
	clone := *config
	clone.Config = map[string]any{}
	for key, value := range config.Config {
		clone.Config[key] = value
	}
	return &clone
}

func cloneChannelMessageLog(log *channel.ChannelMessageLog) *channel.ChannelMessageLog {
	if log == nil {
		return nil
	}
	clone := *log
	clone.RawMessage = append(json.RawMessage(nil), log.RawMessage...)
	if log.TransformedMessage.Content != nil {
		clone.TransformedMessage.Content = append([]channel.ContentPart(nil), log.TransformedMessage.Content...)
	}
	if log.TransformedMessage.Metadata != nil {
		clone.TransformedMessage.Metadata = map[string]any{}
		for key, value := range log.TransformedMessage.Metadata {
			clone.TransformedMessage.Metadata[key] = value
		}
	}
	return &clone
}
