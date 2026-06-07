package channel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeRetryMessageStore struct {
	listInput  ClaimDueRetryMessagesInput
	listed     []*ChannelMessageLog
	listErr    error
	claimInput ClaimDueRetryMessagesInput
	claimed    []*ChannelMessageLog
	claimErr   error
}

func (f *fakeRetryMessageStore) ListDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	f.listInput = input
	return f.listed, f.listErr
}

func (f *fakeRetryMessageStore) ClaimDueRetryMessages(ctx context.Context, input ClaimDueRetryMessagesInput) ([]*ChannelMessageLog, error) {
	f.claimInput = input
	return f.claimed, f.claimErr
}

type fakeRetryWorkerStore struct {
	fakeRetryMessageStore
	configs      map[string]*ChannelConfig
	updated      []*ChannelMessageLog
	getConfigErr error
	updateErr    error
}

func (f *fakeRetryWorkerStore) GetConfigByID(ctx context.Context, id string) (*ChannelConfig, error) {
	if f.getConfigErr != nil {
		return nil, f.getConfigErr
	}
	return f.configs[id], nil
}

func (f *fakeRetryWorkerStore) UpdateRetryMessageLog(ctx context.Context, log *ChannelMessageLog) (*ChannelMessageLog, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	stored := *log
	f.updated = append(f.updated, &stored)
	return &stored, nil
}

type retryTestAdapter struct {
	channelType ChannelType
	deliverErr  error
	configs     []map[string]any
	messages    []InternalMessage
	payloads    []json.RawMessage
}

func (a *retryTestAdapter) Type() ChannelType {
	return a.channelType
}

func (a *retryTestAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	return InternalMessage{}, errors.New("inbound transform is not used by retry tests")
}

func (a *retryTestAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	a.messages = append(a.messages, message)
	return json.Marshal(map[string]any{
		"conversation_id": message.ConversationID,
		"text":            message.Content[0].Text,
	})
}

func (a *retryTestAdapter) DeliverOutbound(ctx context.Context, config map[string]any, raw json.RawMessage) error {
	a.configs = append(a.configs, config)
	a.payloads = append(a.payloads, append(json.RawMessage(nil), raw...))
	return a.deliverErr
}

func TestServiceReceiveTransformsWebhookInboundAndStoresRawAndTransformedMessage(t *testing.T) {
	adapter := NewWebhookAdapter()
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: adapter,
	}))
	raw := json.RawMessage(`{
		"id": "msg_1",
		"conversation_id": "conversation_1",
		"role": "user",
		"text": "hello from webhook",
		"metadata": {"source": "webhook"}
	}`)

	log, err := service.Receive(context.Background(), ReceiveRequest{
		ChannelID: "channel_1",
		Type:      ChannelTypeWebhook,
		Raw:       raw,
		Now:       time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
	})

	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if log.ChannelID != "channel_1" {
		t.Fatalf("expected channel_id to be preserved, got %q", log.ChannelID)
	}
	if log.Direction != DirectionInbound {
		t.Fatalf("expected inbound direction, got %q", log.Direction)
	}
	if string(log.RawMessage) != string(raw) {
		t.Fatalf("expected raw payload to be stored, got %s", log.RawMessage)
	}
	if !log.TransformSuccess {
		t.Fatalf("expected transform success, got error %q", log.TransformError)
	}
	if log.TransformedMessage.ID != "msg_1" ||
		log.TransformedMessage.ConversationID != "conversation_1" ||
		log.TransformedMessage.Role != RoleUser {
		t.Fatalf("unexpected transformed message: %+v", log.TransformedMessage)
	}
	if len(log.TransformedMessage.Content) != 1 ||
		log.TransformedMessage.Content[0].Type != ContentTypeText ||
		log.TransformedMessage.Content[0].Text != "hello from webhook" {
		t.Fatalf("unexpected transformed content: %+v", log.TransformedMessage.Content)
	}
	if log.TransformedMessage.Metadata["source"] != "webhook" {
		t.Fatalf("expected metadata source to survive transform, got %+v", log.TransformedMessage.Metadata)
	}
}

func TestServiceReceiveRecordsTransformFailureAndRawPayload(t *testing.T) {
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))
	raw := json.RawMessage(`{"conversation_id":"conversation_1","role":"invalid","text":"hello"}`)

	log, err := service.Receive(context.Background(), ReceiveRequest{
		ChannelID: "channel_1",
		Type:      ChannelTypeWebhook,
		Raw:       raw,
	})

	if err != nil {
		t.Fatalf("Receive should return a log, not an error, on transform failure: %v", err)
	}
	if log.ChannelID != "channel_1" {
		t.Fatalf("expected failed log to preserve channel_id, got %q", log.ChannelID)
	}
	if log.TransformSuccess {
		t.Fatal("expected transform_success=false")
	}
	if log.TransformError == "" || !strings.Contains(log.TransformError, "role") {
		t.Fatalf("expected role transform error, got %q", log.TransformError)
	}
	if string(log.RawMessage) != string(raw) {
		t.Fatalf("expected raw payload to be preserved, got %s", log.RawMessage)
	}
	if log.TransformedMessage.ID != "" {
		t.Fatalf("expected no transformed message on failure, got %+v", log.TransformedMessage)
	}
}

func TestServiceSendTransformsWebhookOutboundAndReturnsLog(t *testing.T) {
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))
	message := InternalMessage{
		ID:             "msg_out_1",
		ConversationID: "conversation_1",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "hello back"}},
		Metadata:       map[string]any{"source": "agent"},
		Timestamp:      time.Date(2026, 6, 4, 12, 5, 0, 0, time.UTC),
	}

	log, err := service.Send(context.Background(), SendRequest{
		ChannelID: "channel_1",
		Type:      ChannelTypeWebhook,
		Message:   message,
		Now:       time.Date(2026, 6, 4, 12, 6, 0, 0, time.UTC),
	})

	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if log.ChannelID != "channel_1" {
		t.Fatalf("expected channel_id to be preserved, got %q", log.ChannelID)
	}
	if log.Direction != DirectionOutbound {
		t.Fatalf("expected outbound direction, got %q", log.Direction)
	}
	if !log.TransformSuccess {
		t.Fatalf("expected transform success, got error %q", log.TransformError)
	}
	if log.TransformedMessage.ID != "msg_out_1" || log.TransformedMessage.Role != RoleAssistant {
		t.Fatalf("expected transformed message to be preserved, got %+v", log.TransformedMessage)
	}
	if !json.Valid(log.RawMessage) || !strings.Contains(string(log.RawMessage), `"text":"hello back"`) {
		t.Fatalf("expected outbound raw webhook payload, got %s", log.RawMessage)
	}
	if !log.CreatedAt.Equal(time.Date(2026, 6, 4, 12, 6, 0, 0, time.UTC)) {
		t.Fatalf("expected requested timestamp, got %s", log.CreatedAt)
	}
}

func TestServiceSendWebhookPostsConfiguredURLAndRecordsSuccess(t *testing.T) {
	received := make(chan string, 1)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type, got %q", r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read webhook body: %v", err)
		}
		received <- string(body)
		w.WriteHeader(stdhttp.StatusAccepted)
	}))
	defer server.Close()

	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))

	log, err := service.Send(context.Background(), SendRequest{
		ChannelID: "channel_1",
		Type:      ChannelTypeWebhook,
		Config:    map[string]any{"webhook_url": server.URL},
		Message: InternalMessage{
			ID:             "msg_out_1",
			ConversationID: "conversation_1",
			Role:           RoleAssistant,
			Content:        []ContentPart{{Type: ContentTypeText, Text: "hello webhook"}},
		},
	})

	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if log.Status != MessageStatusRecorded || !log.TransformSuccess || log.FailureReason != "" {
		t.Fatalf("expected recorded successful webhook delivery, got %+v", log)
	}
	select {
	case body := <-received:
		if !strings.Contains(body, `"conversation_id":"conversation_1"`) || !strings.Contains(body, `"text":"hello webhook"`) {
			t.Fatalf("expected transformed webhook payload to be posted, got %s", body)
		}
	default:
		t.Fatal("expected webhook endpoint to receive outbound POST")
	}
}

func TestServiceSendWebhookTimeoutRecordsDeliveryFailure(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(200 * time.Millisecond):
			w.WriteHeader(stdhttp.StatusOK)
		}
	}))
	defer server.Close()

	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	log, err := service.Send(ctx, SendRequest{
		ChannelID: "channel_1",
		Type:      ChannelTypeWebhook,
		Config:    map[string]any{"webhook_url": server.URL},
		Message: InternalMessage{
			ID:             "msg_out_timeout",
			ConversationID: "conversation_1",
			Role:           RoleAssistant,
			Content:        []ContentPart{{Type: ContentTypeText, Text: "timeout please"}},
		},
	})

	if err != nil {
		t.Fatalf("Send should return an auditable log, not an error, on timeout: %v", err)
	}
	if log.TransformSuccess {
		t.Fatalf("expected timeout to be recorded as transform_success=false, got %+v", log)
	}
	if log.FailureReason == "" || !strings.Contains(log.FailureReason, "timeout") {
		t.Fatalf("expected timeout failure reason, got %+v", log)
	}
	if !json.Valid(log.RawMessage) || !strings.Contains(string(log.RawMessage), `"text":"timeout please"`) {
		t.Fatalf("expected attempted webhook payload to be preserved, got %s", log.RawMessage)
	}
}

func TestServiceReceivesWeChatInboundWithDefaultAdapterRegistry(t *testing.T) {
	service := NewService(NewAdapterRegistry(nil))
	raw := json.RawMessage(`{
		"MsgId": "wechat_msg_1",
		"FromUserName": "user_openid",
		"MsgType": "text",
		"Content": "hello from wechat"
	}`)

	log, err := service.Receive(context.Background(), ReceiveRequest{
		ChannelID: "channel_wechat",
		Type:      ChannelType("wechat"),
		Raw:       raw,
	})

	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if !log.TransformSuccess {
		t.Fatalf("expected wechat transform success, got %q", log.TransformError)
	}
	if log.ConversationID != "user_openid" || log.TransformedMessage.ID != "wechat_msg_1" {
		t.Fatalf("unexpected transformed wechat message: %+v", log.TransformedMessage)
	}
	if len(log.TransformedMessage.Content) != 1 || log.TransformedMessage.Content[0].Text != "hello from wechat" {
		t.Fatalf("unexpected transformed wechat content: %+v", log.TransformedMessage.Content)
	}
}

func TestServiceSendsDiscordOutboundWithDefaultAdapterRegistry(t *testing.T) {
	service := NewService(NewAdapterRegistry(nil))

	log, err := service.Send(context.Background(), SendRequest{
		ChannelID: "channel_discord",
		Type:      ChannelType("discord"),
		Message: InternalMessage{
			ID:             "msg_out_discord",
			ConversationID: "discord_channel_1",
			Role:           RoleAssistant,
			Content: []ContentPart{
				{Type: ContentTypeText, Text: "discord reply"},
				{Type: ContentTypeCard, Metadata: map[string]any{"title": "Summary", "description": "sent"}},
			},
		},
	})

	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if !log.TransformSuccess {
		t.Fatalf("expected discord transform success, got %q", log.TransformError)
	}
	if !json.Valid(log.RawMessage) || !strings.Contains(string(log.RawMessage), `"content":"discord reply"`) || !strings.Contains(string(log.RawMessage), `"embeds"`) {
		t.Fatalf("expected discord outbound payload, got %s", log.RawMessage)
	}
}

func TestServiceSendRecordsTransformFailureWithAuditableRawPayload(t *testing.T) {
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))

	log, err := service.Send(context.Background(), SendRequest{
		ChannelID: "channel_1",
		Type:      ChannelTypeWebhook,
		Message: InternalMessage{
			ID:             "msg_invalid",
			ConversationID: "conversation_1",
			Role:           RoleAssistant,
		},
	})

	if err != nil {
		t.Fatalf("Send should return a log, not an error, on transform failure: %v", err)
	}
	if log.Direction != DirectionOutbound {
		t.Fatalf("expected outbound direction, got %q", log.Direction)
	}
	if log.TransformSuccess {
		t.Fatal("expected transform_success=false")
	}
	if log.TransformError == "" || !strings.Contains(log.TransformError, "content") {
		t.Fatalf("expected content transform error, got %q", log.TransformError)
	}
	if !json.Valid(log.RawMessage) || !strings.Contains(string(log.RawMessage), `"id":"msg_invalid"`) {
		t.Fatalf("expected auditable raw attempted message payload, got %s", log.RawMessage)
	}
	if log.TransformedMessage.ID != "msg_invalid" {
		t.Fatalf("expected attempted message to be preserved for audit, got %+v", log.TransformedMessage)
	}
}

func TestServiceMarkDeliveryFailedQueuesRetriesAndMarksPermanentFailure(t *testing.T) {
	service := NewService(NewAdapterRegistry(nil))
	now := time.Date(2026, 6, 4, 12, 30, 0, 0, time.UTC)
	base := ChannelMessageLog{
		ID:        "channel_message_retry_1",
		ChannelID: "channel_1",
		Direction: DirectionOutbound,
		Status:    MessageStatusRecorded,
	}

	first := service.MarkDeliveryFailed(base, "upstream 500", now)
	if first.Status != MessageStatusRetryPending {
		t.Fatalf("expected first failure to be retry_pending, got %q", first.Status)
	}
	if first.RetryCount != 1 || first.FailureReason != "upstream 500" {
		t.Fatalf("expected first failure count and reason, got count=%d reason=%q", first.RetryCount, first.FailureReason)
	}
	if first.NextRetryAt == nil || !first.NextRetryAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("expected first retry in 1 minute, got %+v", first.NextRetryAt)
	}

	fifth := service.MarkDeliveryFailed(ChannelMessageLog{
		ID:         "channel_message_retry_1",
		ChannelID:  "channel_1",
		Direction:  DirectionOutbound,
		Status:     MessageStatusRetryPending,
		RetryCount: 4,
	}, "still failing", now)
	if fifth.Status != MessageStatusRetryPending {
		t.Fatalf("expected fifth failure to remain retry_pending, got %q", fifth.Status)
	}
	if fifth.RetryCount != 5 || fifth.NextRetryAt == nil || !fifth.NextRetryAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expected fifth failure to schedule one hour retry, got count=%d next=%+v", fifth.RetryCount, fifth.NextRetryAt)
	}

	sixth := service.MarkDeliveryFailed(ChannelMessageLog{
		ID:         "channel_message_retry_1",
		ChannelID:  "channel_1",
		Direction:  DirectionOutbound,
		Status:     MessageStatusRetryPending,
		RetryCount: 5,
	}, "still failing", now)
	if sixth.Status != MessageStatusPermanentFailure {
		t.Fatalf("expected sixth failure to be permanent_failure, got %q", sixth.Status)
	}
	if sixth.RetryCount != 6 || sixth.NextRetryAt != nil {
		t.Fatalf("expected permanent failure without next retry, got count=%d next=%+v", sixth.RetryCount, sixth.NextRetryAt)
	}
}

func TestNextRetryDelayUsesPublishingFailureBackoff(t *testing.T) {
	cases := []struct {
		retryCount int
		want       time.Duration
	}{
		{retryCount: 1, want: time.Minute},
		{retryCount: 2, want: 5 * time.Minute},
		{retryCount: 3, want: 15 * time.Minute},
		{retryCount: 4, want: 30 * time.Minute},
		{retryCount: 5, want: time.Hour},
		{retryCount: 6, want: 0},
	}

	for _, tt := range cases {
		if got := NextRetryDelay(tt.retryCount); got != tt.want {
			t.Fatalf("NextRetryDelay(%d) = %s, want %s", tt.retryCount, got, tt.want)
		}
	}
}

func TestServiceListsAndClaimsDueRetryMessagesThroughStore(t *testing.T) {
	now := time.Date(2026, 6, 4, 16, 30, 0, 0, time.UTC)
	dueLog := &ChannelMessageLog{
		ID:            "channel_message_due_1",
		ChannelID:     "channel_1",
		Direction:     DirectionOutbound,
		Status:        MessageStatusRetryPending,
		RetryCount:    1,
		FailureReason: "temporary failure",
	}
	store := &fakeRetryMessageStore{
		listed:  []*ChannelMessageLog{dueLog},
		claimed: []*ChannelMessageLog{{ID: dueLog.ID, ChannelID: dueLog.ChannelID, Direction: DirectionOutbound, Status: MessageStatusSending}},
	}
	service := NewService(NewAdapterRegistry(nil))

	listed, err := service.ListDueRetryMessages(context.Background(), store, ClaimDueRetryMessagesInput{ChannelID: "channel_1", Now: now, Limit: 5})
	if err != nil {
		t.Fatalf("ListDueRetryMessages returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != dueLog.ID {
		t.Fatalf("unexpected listed retry messages: %+v", listed)
	}
	if store.listInput.ChannelID != "channel_1" || !store.listInput.Now.Equal(now) || store.listInput.Limit != 5 {
		t.Fatalf("expected list input to be forwarded, got %+v", store.listInput)
	}

	claimed, err := service.ClaimDueRetryMessages(context.Background(), store, ClaimDueRetryMessagesInput{ChannelID: "channel_1", Now: now, Limit: 5})
	if err != nil {
		t.Fatalf("ClaimDueRetryMessages returned error: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != dueLog.ID || claimed[0].Status != MessageStatusSending {
		t.Fatalf("unexpected claimed retry messages: %+v", claimed)
	}
	if store.claimInput.ChannelID != "channel_1" || !store.claimInput.Now.Equal(now) || store.claimInput.Limit != 5 {
		t.Fatalf("expected claim input to be forwarded, got %+v", store.claimInput)
	}
}

func TestServiceProcessDueRetryMessagesResendsClaimedMessagesAndRecordsSuccess(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	createdAt := now.Add(-10 * time.Minute)
	dueAt := now.Add(-time.Minute)
	channelType := ChannelType("retry_success")
	adapter := &retryTestAdapter{channelType: channelType}
	claimed := &ChannelMessageLog{
		ID:                 "channel_message_retry_success",
		ChannelID:          "channel_retry_success",
		ConversationID:     "conversation_retry",
		Direction:          DirectionOutbound,
		RawMessage:         json.RawMessage(`{"old":"payload"}`),
		TransformedMessage: InternalMessage{ID: "msg_retry_success", ConversationID: "conversation_retry", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "retry succeeds"}}},
		TransformSuccess:   false,
		TransformError:     "previous timeout",
		Status:             MessageStatusSending,
		RetryCount:         2,
		FailureReason:      "previous timeout",
		NextRetryAt:        &dueAt,
		CreatedAt:          createdAt,
	}
	store := &fakeRetryWorkerStore{
		fakeRetryMessageStore: fakeRetryMessageStore{claimed: []*ChannelMessageLog{claimed}},
		configs: map[string]*ChannelConfig{
			claimed.ChannelID: {ID: claimed.ChannelID, Type: channelType, Config: map[string]any{"webhook_url": "https://example.test/retry"}},
		},
	}
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{channelType: adapter}))

	result, err := service.ProcessDueRetryMessages(context.Background(), store, ClaimDueRetryMessagesInput{Now: now, Limit: 1})
	if err != nil {
		t.Fatalf("ProcessDueRetryMessages returned error: %v", err)
	}
	if result.Claimed != 1 || result.Succeeded != 1 || result.Failed != 0 || result.PermanentFailures != 0 {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if len(adapter.messages) != 1 || adapter.messages[0].ID != "msg_retry_success" {
		t.Fatalf("expected retry worker to resend transformed message through adapter, got %+v", adapter.messages)
	}
	if len(adapter.configs) != 1 || adapter.configs[0]["webhook_url"] != "https://example.test/retry" {
		t.Fatalf("expected retry worker to deliver with channel config, got %+v", adapter.configs)
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected one retry message update, got %+v", store.updated)
	}
	updated := store.updated[0]
	if updated.ID != claimed.ID || updated.Status != MessageStatusRecorded || !updated.TransformSuccess {
		t.Fatalf("expected claimed message to be marked successful, got %+v", updated)
	}
	if updated.RetryCount != 2 {
		t.Fatalf("expected retry count to preserve audit history, got %d", updated.RetryCount)
	}
	if updated.NextRetryAt != nil || updated.TransformError != "" || updated.FailureReason != "" {
		t.Fatalf("expected successful retry to clear retry fields, got %+v", updated)
	}
	if string(updated.RawMessage) == string(claimed.RawMessage) || !strings.Contains(string(updated.RawMessage), "retry succeeds") {
		t.Fatalf("expected successful retry to store fresh outbound payload, got %s", updated.RawMessage)
	}
}

func TestServiceProcessDueRetryMessagesRoutesClaimedMessageToFallbackChannel(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 15, 0, 0, time.UTC)
	primaryType := ChannelType("retry_primary")
	fallbackType := ChannelType("retry_fallback")
	primaryAdapter := &retryTestAdapter{channelType: primaryType}
	fallbackAdapter := &retryTestAdapter{channelType: fallbackType}
	claimed := &ChannelMessageLog{
		ID:                 "channel_message_retry_fallback",
		ChannelID:          "channel_primary",
		ConversationID:     "conversation_retry",
		Direction:          DirectionOutbound,
		RawMessage:         json.RawMessage(`{"old":"payload"}`),
		TransformedMessage: InternalMessage{ID: "msg_retry_fallback", ConversationID: "conversation_retry", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "send on fallback"}}},
		TransformSuccess:   false,
		TransformError:     "primary degraded",
		Status:             MessageStatusSending,
		RetryCount:         2,
		FailureReason:      "primary degraded",
		CreatedAt:          now.Add(-15 * time.Minute),
	}
	store := &fakeRetryWorkerStore{
		fakeRetryMessageStore: fakeRetryMessageStore{claimed: []*ChannelMessageLog{claimed}},
		configs: map[string]*ChannelConfig{
			"channel_primary":  {ID: "channel_primary", Type: primaryType, Config: map[string]any{"webhook_url": "https://primary.example/retry"}},
			"channel_fallback": {ID: "channel_fallback", Type: fallbackType, Config: map[string]any{"webhook_url": "https://fallback.example/retry"}},
		},
	}
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		primaryType:  primaryAdapter,
		fallbackType: fallbackAdapter,
	}))

	result, err := service.ProcessDueRetryMessages(context.Background(), store, ClaimDueRetryMessagesInput{
		FallbackChannelID: "channel_fallback",
		Now:               now,
		Limit:             1,
	})
	if err != nil {
		t.Fatalf("ProcessDueRetryMessages returned error: %v", err)
	}
	if result.Claimed != 1 || result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("unexpected fallback retry result: %+v", result)
	}
	if len(primaryAdapter.messages) != 0 {
		t.Fatalf("expected primary adapter not to resend fallback-routed message, got %+v", primaryAdapter.messages)
	}
	if len(fallbackAdapter.messages) != 1 || fallbackAdapter.messages[0].ID != "msg_retry_fallback" {
		t.Fatalf("expected fallback adapter to send claimed message, got %+v", fallbackAdapter.messages)
	}
	if len(fallbackAdapter.configs) != 1 || fallbackAdapter.configs[0]["webhook_url"] != "https://fallback.example/retry" {
		t.Fatalf("expected fallback channel config, got %+v", fallbackAdapter.configs)
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected one retry message update, got %+v", store.updated)
	}
	updated := store.updated[0]
	if updated.ChannelID != "channel_fallback" || updated.Status != MessageStatusRecorded || !updated.TransformSuccess {
		t.Fatalf("expected retry message to be recorded on fallback channel, got %+v", updated)
	}
	if updated.TransformError != "" || updated.FailureReason != "" || updated.NextRetryAt != nil {
		t.Fatalf("expected fallback success to clear previous failure metadata, got %+v", updated)
	}
}

func TestServiceProcessDueRetryMessagesBacksOffFailedResend(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 30, 0, 0, time.UTC)
	channelType := ChannelType("retry_failure")
	adapter := &retryTestAdapter{channelType: channelType, deliverErr: errors.New("upstream 503")}
	claimed := &ChannelMessageLog{
		ID:                 "channel_message_retry_failure",
		ChannelID:          "channel_retry_failure",
		ConversationID:     "conversation_retry",
		Direction:          DirectionOutbound,
		TransformedMessage: InternalMessage{ID: "msg_retry_failure", ConversationID: "conversation_retry", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "retry fails"}}},
		Status:             MessageStatusSending,
		RetryCount:         2,
		CreatedAt:          now.Add(-time.Hour),
	}
	store := &fakeRetryWorkerStore{
		fakeRetryMessageStore: fakeRetryMessageStore{claimed: []*ChannelMessageLog{claimed}},
		configs: map[string]*ChannelConfig{
			claimed.ChannelID: {ID: claimed.ChannelID, Type: channelType, Config: map[string]any{}},
		},
	}
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{channelType: adapter}))

	result, err := service.ProcessDueRetryMessages(context.Background(), store, ClaimDueRetryMessagesInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ProcessDueRetryMessages returned error: %v", err)
	}
	if result.Claimed != 1 || result.Succeeded != 0 || result.Failed != 1 || result.PermanentFailures != 0 {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected one retry message update, got %+v", store.updated)
	}
	updated := store.updated[0]
	if updated.Status != MessageStatusRetryPending || updated.RetryCount != 3 {
		t.Fatalf("expected failed retry to be queued with incremented count, got %+v", updated)
	}
	if updated.FailureReason != "upstream 503" || updated.TransformError != "upstream 503" {
		t.Fatalf("expected delivery failure reason to be preserved, got %+v", updated)
	}
	if updated.NextRetryAt == nil || !updated.NextRetryAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("expected third retry backoff at %s, got %+v", now.Add(15*time.Minute), updated.NextRetryAt)
	}
}

func TestServiceProcessDueRetryMessagesMarksPermanentFailureAtRetryLimit(t *testing.T) {
	now := time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC)
	channelType := ChannelType("retry_permanent")
	adapter := &retryTestAdapter{channelType: channelType, deliverErr: errors.New("still unavailable")}
	claimed := &ChannelMessageLog{
		ID:                 "channel_message_retry_permanent",
		ChannelID:          "channel_retry_permanent",
		Direction:          DirectionOutbound,
		TransformedMessage: InternalMessage{ID: "msg_retry_permanent", ConversationID: "conversation_retry", Role: RoleAssistant, Content: []ContentPart{{Type: ContentTypeText, Text: "retry permanent"}}},
		Status:             MessageStatusSending,
		RetryCount:         5,
		CreatedAt:          now.Add(-2 * time.Hour),
	}
	store := &fakeRetryWorkerStore{
		fakeRetryMessageStore: fakeRetryMessageStore{claimed: []*ChannelMessageLog{claimed}},
		configs: map[string]*ChannelConfig{
			claimed.ChannelID: {ID: claimed.ChannelID, Type: channelType, Config: map[string]any{}},
		},
	}
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{channelType: adapter}))

	result, err := service.ProcessDueRetryMessages(context.Background(), store, ClaimDueRetryMessagesInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ProcessDueRetryMessages returned error: %v", err)
	}
	if result.Claimed != 1 || result.Succeeded != 0 || result.Failed != 1 || result.PermanentFailures != 1 {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if len(store.updated) != 1 {
		t.Fatalf("expected one retry message update, got %+v", store.updated)
	}
	updated := store.updated[0]
	if updated.Status != MessageStatusPermanentFailure || updated.RetryCount != 6 {
		t.Fatalf("expected retry limit to mark permanent failure, got %+v", updated)
	}
	if updated.NextRetryAt != nil {
		t.Fatalf("expected permanent failure to clear next retry timestamp, got %+v", updated.NextRetryAt)
	}
}

func TestServiceRequiresRetryMessageStore(t *testing.T) {
	service := NewService(NewAdapterRegistry(nil))

	_, err := service.ListDueRetryMessages(context.Background(), nil, ClaimDueRetryMessagesInput{})
	if err == nil || !errors.Is(err, ErrRetryMessageStoreRequired) {
		t.Fatalf("expected ErrRetryMessageStoreRequired for nil list store, got %v", err)
	}

	_, err = service.ClaimDueRetryMessages(context.Background(), nil, ClaimDueRetryMessagesInput{})
	if err == nil || !errors.Is(err, ErrRetryMessageStoreRequired) {
		t.Fatalf("expected ErrRetryMessageStoreRequired for nil claim store, got %v", err)
	}

	_, err = service.ProcessDueRetryMessages(context.Background(), nil, ClaimDueRetryMessagesInput{})
	if err == nil || !errors.Is(err, ErrRetryMessageStoreRequired) {
		t.Fatalf("expected ErrRetryMessageStoreRequired for nil process store, got %v", err)
	}
}

func TestServiceTestReturnsDeterministicSuccessWhenAdapterExists(t *testing.T) {
	channelType := ChannelType("test_adapter")
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		channelType: &retryTestAdapter{channelType: channelType},
	}))

	result, err := service.Test(context.Background(), ChannelConfig{
		ID:     "channel_1",
		Type:   channelType,
		Status: ChannelStatusActive,
	})

	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	if result.ChannelID != "channel_1" || result.Type != string(channelType) || result.Status != "success" || result.Message == "" {
		t.Fatalf("unexpected deterministic test result: %+v", result)
	}
}

func TestServiceTestWebhookValidatesConfiguredURL(t *testing.T) {
	received := make(chan struct{}, 1)
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		received <- struct{}{}
		w.WriteHeader(stdhttp.StatusNoContent)
	}))
	defer server.Close()

	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))

	result, err := service.Test(context.Background(), ChannelConfig{
		ID:     "channel_1",
		Type:   ChannelTypeWebhook,
		Config: map[string]any{"webhook_url": server.URL},
		Status: ChannelStatusActive,
	})

	if err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("expected successful webhook test, got %+v", result)
	}
	select {
	case <-received:
	default:
		t.Fatal("expected webhook test to call configured webhook_url")
	}
}

func TestServiceTestWebhookFailsWithoutConfiguredURL(t *testing.T) {
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{
		ChannelTypeWebhook: NewWebhookAdapter(),
	}))

	result, err := service.Test(context.Background(), ChannelConfig{
		ID:     "channel_1",
		Type:   ChannelTypeWebhook,
		Config: map[string]any{"webhook_url": "   "},
		Status: ChannelStatusActive,
	})

	if err != nil {
		t.Fatalf("Test should return a failed result, not an error, for empty webhook_url: %v", err)
	}
	if result.Status != "failed" || result.Message == "" || !strings.Contains(result.Message, "webhook_url") {
		t.Fatalf("expected failed webhook_url test result, got %+v", result)
	}
}

func TestServiceTestReturnsErrorWhenAdapterMissing(t *testing.T) {
	service := NewService(NewAdapterRegistry(map[ChannelType]ChannelAdapter{}))

	_, err := service.Test(context.Background(), ChannelConfig{
		ID:   "channel_1",
		Type: ChannelTypeWebhook,
	})

	if err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("expected missing adapter error, got %v", err)
	}
}
