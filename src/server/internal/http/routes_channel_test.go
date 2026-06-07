package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/channel"
)

func TestRegisterPublishingChannelRoutesKeepsWebhookPublic(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Webhook", Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)
	mux := stdhttp.NewServeMux()
	auth := &recordingSessionMiddleware{}
	registerPublishingChannelRoutes(mux, auth, handler)

	body := `{"id":"msg_1","conversation_id":"conversation_1","role":"user","text":"hello"}`
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/webhook/channel_1", strings.NewReader(body))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected public webhook 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if auth.requestCalls != 0 {
		t.Fatalf("expected webhook route not to require session, got %d middleware request calls", auth.requestCalls)
	}
	if store.lastLog == nil || store.lastLog.ChannelID != "channel_1" {
		t.Fatalf("expected webhook log for channel_1, got %+v", store.lastLog)
	}
}

func TestRegisterPublishingChannelRoutesDispatchesUpdateDeleteAndTest(t *testing.T) {
	store := &publishingChannelFakeStore{configs: map[string]*channel.ChannelConfig{
		"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Webhook", Config: map[string]any{}, Status: channel.ChannelStatusActive},
	}}
	handler := newPublishingChannelTestHandler(store)
	mux := stdhttp.NewServeMux()
	auth := &recordingSessionMiddleware{}
	registerPublishingChannelRoutes(mux, auth, handler)

	update := httptest.NewRecorder()
	mux.ServeHTTP(update, publishingChannelRequest(stdhttp.MethodPut, "/api/v1/channels/channel_1", `{"type":"webhook","name":"Updated","config":{},"status":"active"}`, "org_1"))
	if update.Code != stdhttp.StatusOK {
		t.Fatalf("expected PUT dispatch 200, got %d with body %s", update.Code, update.Body.String())
	}
	if store.updatedConfig == nil || store.updatedConfig.Name != "Updated" {
		t.Fatalf("expected update route to modify channel, got %+v", store.updatedConfig)
	}

	test := httptest.NewRecorder()
	mux.ServeHTTP(test, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/test", "", "org_1"))
	if test.Code != stdhttp.StatusOK {
		t.Fatalf("expected test dispatch 200, got %d with body %s", test.Code, test.Body.String())
	}

	send := httptest.NewRecorder()
	mux.ServeHTTP(send, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/send", `{"message":{"conversation_id":"conversation_1","role":"assistant","text":"hello"}}`, "org_1"))
	if send.Code != stdhttp.StatusOK {
		t.Fatalf("expected send dispatch 200, got %d with body %s", send.Code, send.Body.String())
	}
	if store.lastLog == nil || store.lastLog.Direction != channel.DirectionOutbound {
		t.Fatalf("expected send route to record outbound log, got %+v", store.lastLog)
	}

	remove := httptest.NewRecorder()
	mux.ServeHTTP(remove, publishingChannelRequest(stdhttp.MethodDelete, "/api/v1/channels/channel_1", "", "org_1"))
	if remove.Code != stdhttp.StatusOK {
		t.Fatalf("expected DELETE dispatch 200, got %d with body %s", remove.Code, remove.Body.String())
	}
	if store.updatedStatus != channel.ChannelStatusDisabled {
		t.Fatalf("expected delete route to disable channel, got %q", store.updatedStatus)
	}
	if auth.requestCalls != 4 {
		t.Fatalf("expected session middleware for four authenticated routes, got %d", auth.requestCalls)
	}
}

func TestRegisterPublishingChannelRoutesDispatchesMessageLogAndFailureVisibility(t *testing.T) {
	nextRetryAt := time.Date(2026, 6, 4, 12, 45, 0, 0, time.UTC)
	store := &publishingChannelFakeStore{
		configs: map[string]*channel.ChannelConfig{
			"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Ops Webhook", Status: channel.ChannelStatusDegraded},
			"channel_2": {ID: "channel_2", OrganizationID: "org_other", Type: channel.ChannelTypeWebhook, Name: "Other", Status: channel.ChannelStatusActive},
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
				ID:               "other_org_message",
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
	mux := stdhttp.NewServeMux()
	auth := &recordingSessionMiddleware{}
	registerPublishingChannelRoutes(mux, auth, handler)

	messages := httptest.NewRecorder()
	mux.ServeHTTP(messages, publishingChannelRequest(stdhttp.MethodGet, "/api/v1/channels/channel_1/messages", "", "org_1"))
	if messages.Code != stdhttp.StatusOK {
		t.Fatalf("expected message log route 200, got %d with body %s", messages.Code, messages.Body.String())
	}
	var messagesResponse struct {
		Data []channel.ChannelMessageLog `json:"data"`
	}
	if err := json.Unmarshal(messages.Body.Bytes(), &messagesResponse); err != nil {
		t.Fatalf("decode message log response: %v", err)
	}
	if len(messagesResponse.Data) != 2 || messagesResponse.Data[0].ID != "channel_message_recent" || messagesResponse.Data[1].RawMessage == nil {
		t.Fatalf("expected recent channel-scoped message logs with raw payloads, got %+v", messagesResponse.Data)
	}

	failures := httptest.NewRecorder()
	mux.ServeHTTP(failures, publishingChannelRequest(stdhttp.MethodGet, "/api/v1/channels/channel_1/failed-messages", "", "org_1"))
	if failures.Code != stdhttp.StatusOK {
		t.Fatalf("expected failed message route 200, got %d with body %s", failures.Code, failures.Body.String())
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
		failed.NextRetryAt == nil {
		t.Fatalf("expected failed retry metadata to be visible, got %+v", failed)
	}
	if auth.requestCalls != 2 {
		t.Fatalf("expected session middleware for message visibility routes, got %d", auth.requestCalls)
	}
}

func TestRegisterPublishingChannelRoutesDispatchesFailedMessageRetry(t *testing.T) {
	now := time.Now().UTC().Add(-time.Minute)
	store := &publishingChannelFakeStore{
		configs: map[string]*channel.ChannelConfig{
			"channel_1": {ID: "channel_1", OrganizationID: "org_1", Type: channel.ChannelTypeWebhook, Name: "Ops Webhook", Status: channel.ChannelStatusDegraded},
		},
		logs: []*channel.ChannelMessageLog{
			{
				ID:                 "channel_message_failed",
				ChannelID:          "channel_1",
				Direction:          channel.DirectionOutbound,
				TransformedMessage: channel.InternalMessage{ID: "msg_out_1", Role: channel.RoleAssistant, Content: []channel.ContentPart{{Type: channel.ContentTypeText, Text: "retry me"}}},
				TransformSuccess:   false,
				Status:             channel.MessageStatusRetryPending,
				RetryCount:         1,
				NextRetryAt:        &now,
				CreatedAt:          now.Add(-time.Minute),
			},
		},
	}
	handler := newPublishingChannelTestHandler(store)
	mux := stdhttp.NewServeMux()
	auth := &recordingSessionMiddleware{}
	registerPublishingChannelRoutes(mux, auth, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, publishingChannelRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/retry-failed-messages", `{"limit":5}`, "org_1"))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected retry failed messages route 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.claimInput.ChannelID != "channel_1" || store.claimInput.Limit != 5 {
		t.Fatalf("expected route to dispatch channel-scoped retry, got %+v", store.claimInput)
	}
	if auth.requestCalls != 1 {
		t.Fatalf("expected session middleware for retry route, got %d", auth.requestCalls)
	}
}

type recordingSessionMiddleware struct {
	requestCalls int
}

func (m *recordingSessionMiddleware) requireSession(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		m.requestCalls++
		next.ServeHTTP(w, r)
	})
}
