package observability

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"oblivious/server/internal/notification"
)

type captureDeliverySink struct {
	channel AlertDeliveryChannel
	events  []AlertEvent
	err     error
}

func (s *captureDeliverySink) Channel() AlertDeliveryChannel {
	return s.channel
}

func (s *captureDeliverySink) Deliver(_ context.Context, event AlertEvent) error {
	s.events = append(s.events, event)
	return s.err
}

type alertNotificationStore struct {
	notifications []*notification.Notification
	err           error
}

type capturedSMTPMessage struct {
	from       string
	recipients []string
	data       string
}

func startFakeSMTPServer(t *testing.T) (string, func() []capturedSMTPMessage, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake SMTP server: %v", err)
	}
	var (
		mu       sync.Mutex
		messages []capturedSMTPMessage
	)
	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					t.Errorf("accept fake SMTP connection: %v", err)
					return
				}
			}
			go handleFakeSMTPConnection(t, conn, &mu, &messages)
		}
	}()
	snapshot := func() []capturedSMTPMessage {
		mu.Lock()
		defer mu.Unlock()
		copied := make([]capturedSMTPMessage, len(messages))
		copy(copied, messages)
		return copied
	}
	closeServer := func() {
		close(done)
		_ = listener.Close()
	}
	return listener.Addr().String(), snapshot, closeServer
}

func handleFakeSMTPConnection(t *testing.T, conn net.Conn, mu *sync.Mutex, messages *[]capturedSMTPMessage) {
	t.Helper()
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeLine := func(line string) {
		if _, err := fmt.Fprintf(writer, "%s\r\n", line); err != nil {
			t.Errorf("write fake SMTP response: %v", err)
			return
		}
		if err := writer.Flush(); err != nil {
			t.Errorf("flush fake SMTP response: %v", err)
		}
	}
	writeLine("220 localhost ESMTP")
	message := capturedSMTPMessage{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(command)
		switch {
		case strings.HasPrefix(upper, "EHLO ") || strings.HasPrefix(upper, "HELO "):
			writeLine("250-localhost")
			writeLine("250 AUTH PLAIN")
		case strings.HasPrefix(upper, "AUTH "):
			writeLine("235 2.7.0 authentication accepted")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			message.from = strings.Trim(strings.TrimPrefix(command, "MAIL FROM:"), "<> ")
			writeLine("250 OK")
		case strings.HasPrefix(upper, "RCPT TO:"):
			message.recipients = append(message.recipients, strings.Trim(strings.TrimPrefix(command, "RCPT TO:"), "<> "))
			writeLine("250 OK")
		case upper == "DATA":
			writeLine("354 End data with <CR><LF>.<CR><LF>")
			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					t.Errorf("read fake SMTP DATA: %v", err)
					return
				}
				if strings.TrimRight(dataLine, "\r\n") == "." {
					break
				}
				body.WriteString(dataLine)
			}
			message.data = body.String()
			mu.Lock()
			*messages = append(*messages, message)
			mu.Unlock()
			writeLine("250 OK")
		case upper == "QUIT":
			writeLine("221 Bye")
			return
		default:
			writeLine("250 OK")
		}
	}
}

func (s *alertNotificationStore) Create(_ context.Context, notification *notification.Notification) (*notification.Notification, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.notifications = append(s.notifications, notification)
	return notification, nil
}

func (s *alertNotificationStore) Get(_ context.Context, id string) (*notification.Notification, error) {
	for _, notification := range s.notifications {
		if notification.ID == id {
			return notification, nil
		}
	}
	return nil, nil
}

func (s *alertNotificationStore) List(_ context.Context, userID string, _ bool, _, _ int) ([]*notification.Notification, error) {
	notifications := []*notification.Notification{}
	for _, notification := range s.notifications {
		if notification.UserID == userID {
			notifications = append(notifications, notification)
		}
	}
	return notifications, nil
}

func (s *alertNotificationStore) MarkRead(context.Context, string) error {
	return nil
}

func (s *alertNotificationStore) MarkAllRead(context.Context, string) error {
	return nil
}

func (s *alertNotificationStore) Delete(context.Context, string) error {
	return nil
}

func (s *alertNotificationStore) GetUnreadCount(context.Context, string) (int, error) {
	return 0, nil
}

func TestDefaultAlertDeliveryPolicyRoutesBySeverity(t *testing.T) {
	policy := DefaultAlertDeliveryPolicy()

	cases := []struct {
		name     string
		severity AlertSeverity
		want     []AlertDeliveryChannel
	}{
		{name: "debug", severity: AlertSeverityDebug, want: nil},
		{name: "info", severity: AlertSeverityInfo, want: []AlertDeliveryChannel{
			AlertDeliveryChannelEmail,
		}},
		{name: "warning", severity: AlertSeverityWarning, want: []AlertDeliveryChannel{
			AlertDeliveryChannelEmail,
			AlertDeliveryChannelIM,
		}},
		{name: "critical", severity: AlertSeverityCritical, want: []AlertDeliveryChannel{
			AlertDeliveryChannelEmail,
			AlertDeliveryChannelIM,
			AlertDeliveryChannelSMS,
			AlertDeliveryChannelThirdParty,
			AlertDeliveryChannelPhone,
		}},
		{name: "unknown severity falls back to warning", severity: AlertSeverity("notice"), want: []AlertDeliveryChannel{
			AlertDeliveryChannelEmail,
			AlertDeliveryChannelIM,
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := policy.ChannelsForSeverity(tt.severity)
			if !sameDeliveryChannels(got, tt.want) {
				t.Fatalf("expected channels %v, got %v", tt.want, got)
			}
		})
	}
}

func TestAlertRoutingRuleStoreUpdatesDispatcherChannels(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertRoutingRuleStore(DefaultAlertRoutingRules())
	email := &captureDeliverySink{channel: AlertDeliveryChannelEmail}
	im := &captureDeliverySink{channel: AlertDeliveryChannelIM}
	sms := &captureDeliverySink{channel: AlertDeliveryChannelSMS}
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		RoutingRules: store,
		Sinks:        []AlertDeliverySink{email, im, sms},
	})

	before := dispatcher.Deliver(ctx, AlertEvent{
		Key:      "relay-backlog",
		Severity: AlertSeverityWarning,
		Title:    "Relay backlog",
	})
	if !sameDeliveryChannels(deliveryResultChannels(before), []AlertDeliveryChannel{
		AlertDeliveryChannelEmail,
		AlertDeliveryChannelIM,
	}) {
		t.Fatalf("expected default warning channels, got %+v", before)
	}

	if _, err := store.UpdateRoutingRules(ctx, AlertRoutingRules{
		AlertSeverityDebug:    nil,
		AlertSeverityInfo:     {AlertDeliveryChannelEmail},
		AlertSeverityWarning:  {AlertDeliveryChannelSMS},
		AlertSeverityCritical: {AlertDeliveryChannelEmail, AlertDeliveryChannelIM, AlertDeliveryChannelSMS, AlertDeliveryChannelThirdParty, AlertDeliveryChannelPhone},
	}); err != nil {
		t.Fatalf("update routing rules: %v", err)
	}

	after := dispatcher.Deliver(ctx, AlertEvent{
		Key:      "relay-backlog",
		Severity: AlertSeverityWarning,
		Title:    "Relay backlog",
	})
	if !sameDeliveryChannels(deliveryResultChannels(after), []AlertDeliveryChannel{AlertDeliveryChannelSMS}) {
		t.Fatalf("expected updated warning channel to be used by dispatcher, got %+v", after)
	}
	if len(sms.events) != 1 {
		t.Fatalf("expected SMS sink to receive updated warning alert once, got %d", len(sms.events))
	}
}

func TestAlertDeliveryDispatcherBatchesInfoEmailDigestForOneHour(t *testing.T) {
	ctx := context.Background()
	email := &captureDeliverySink{channel: AlertDeliveryChannelEmail}
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityInfo: {AlertDeliveryChannelEmail},
			},
		},
		Sinks: []AlertDeliverySink{email},
	})
	startedAt := time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC)

	firstResults := dispatcher.Deliver(ctx, AlertEvent{
		Key:        "service-started",
		Severity:   AlertSeverityInfo,
		Title:      "Service started",
		Message:    "api server started",
		Component:  "server",
		OccurredAt: startedAt,
	})
	secondResults := dispatcher.Deliver(ctx, AlertEvent{
		Key:        "job-finished",
		Severity:   AlertSeverityInfo,
		Title:      "Scheduled job finished",
		Message:    "daily sync completed",
		Component:  "scheduler",
		OccurredAt: startedAt.Add(30 * time.Minute),
	})

	if len(firstResults) != 0 || len(secondResults) != 0 || len(email.events) != 0 {
		t.Fatalf("expected info email alerts to be queued for digest, first=%+v second=%+v email=%+v", firstResults, secondResults, email.events)
	}
	if earlyResults := dispatcher.FlushInfoEmailDigests(ctx, startedAt.Add(59*time.Minute)); len(earlyResults) != 0 {
		t.Fatalf("expected digest flush before one hour to be skipped, got %+v", earlyResults)
	}

	dueResults := dispatcher.FlushInfoEmailDigests(ctx, startedAt.Add(time.Hour))

	if len(dueResults) != 1 || !dueResults[0].Delivered || dueResults[0].Channel != AlertDeliveryChannelEmail || dueResults[0].Err != nil {
		t.Fatalf("expected one delivered info digest email, got %+v", dueResults)
	}
	if len(email.events) != 1 {
		t.Fatalf("expected one digest email event, got %+v", email.events)
	}
	digest := email.events[0]
	if digest.Severity != AlertSeverityInfo || digest.Title != "Info alert digest (2 events)" {
		t.Fatalf("expected info digest title and severity, got %+v", digest)
	}
	if !strings.Contains(digest.Message, "Service started") || !strings.Contains(digest.Message, "Scheduled job finished") || !strings.Contains(digest.Message, "api server started") || !strings.Contains(digest.Message, "daily sync completed") {
		t.Fatalf("expected digest message to summarize queued info alerts, got %q", digest.Message)
	}
}

func TestAlertRoutingRulesRouteWarningToInAppNotifications(t *testing.T) {
	ctx := context.Background()
	stateStore := NewInMemoryAlertStateStore()
	routingStore := NewInMemoryAlertRoutingRuleStore(DefaultAlertRoutingRules())
	notificationStore := &alertNotificationStore{}
	notifier := notification.NewService(notificationStore)
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		RoutingRules: routingStore,
		Sinks: []AlertDeliverySink{
			NewInAppAlertDeliverySink(AlertInAppDeliverySinkOptions{
				Notifier:         notifier,
				RecipientUserIDs: []string{"admin_user"},
				ActionURL:        "/admin/alerts",
			}),
		},
		HistoryStore: stateStore,
	})
	router := NewAlertRouter(AlertRouterOptions{
		StateStore: stateStore,
		NotifySink: dispatcher,
	})

	if _, err := routingStore.UpdateRoutingRules(ctx, AlertRoutingRules{
		AlertSeverityWarning: {AlertDeliveryChannelInApp},
	}); err != nil {
		t.Fatalf("update routing rules with in-app warning delivery: %v", err)
	}

	if err := router.Route(ctx, AlertEvent{
		Key:        "relay-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Relay backlog",
		Message:    "queue depth is high",
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("route warning alert: %v", err)
	}

	if len(notificationStore.notifications) != 1 {
		t.Fatalf("expected one in-app notification, got %+v", notificationStore.notifications)
	}
	if notificationStore.notifications[0].UserID != "admin_user" {
		t.Fatalf("expected alert notification for admin_user, got %+v", notificationStore.notifications[0])
	}
	attempts, err := stateStore.ListDeliveryAttempts(ctx, AlertDeliveryHistoryFilter{AlertKey: "relay-backlog"})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Channel != AlertDeliveryChannelInApp || !attempts[0].Delivered {
		t.Fatalf("expected successful in-app delivery attempt, got %+v", attempts)
	}
}

func TestAlertDeliveryFanOutReportsFailuresWithoutBlockingOtherChannels(t *testing.T) {
	email := &captureDeliverySink{channel: AlertDeliveryChannelEmail}
	im := &captureDeliverySink{channel: AlertDeliveryChannelIM, err: errors.New("im webhook failed")}
	inApp := &captureDeliverySink{channel: AlertDeliveryChannelInApp}
	sms := &captureDeliverySink{channel: AlertDeliveryChannelSMS}
	thirdParty := &captureDeliverySink{channel: AlertDeliveryChannelThirdParty}
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DefaultAlertDeliveryPolicy(),
		Sinks:  []AlertDeliverySink{email, im, inApp, sms, thirdParty},
	})

	results := dispatcher.Deliver(context.Background(), AlertEvent{
		Key:      "queue-backlog",
		Severity: AlertSeverityWarning,
		Title:    "Queue backlog",
		Message:  "queue depth is high",
	})

	if len(results) != 2 {
		t.Fatalf("expected one result per warning channel, got %d: %+v", len(results), results)
	}
	if resultForChannel(t, results, AlertDeliveryChannelEmail).Err != nil {
		t.Fatalf("expected email delivery to succeed: %+v", results)
	}
	imResult := resultForChannel(t, results, AlertDeliveryChannelIM)
	if imResult.Err == nil {
		t.Fatalf("expected IM delivery failure to be captured: %+v", results)
	}
	if len(email.events) != 1 || len(im.events) != 1 {
		t.Fatalf("expected matching warning sinks to be called once, email=%d im=%d", len(email.events), len(im.events))
	}
	if len(inApp.events) != 0 || len(sms.events) != 0 || len(thirdParty.events) != 0 {
		t.Fatalf("expected non-warning sinks not to be called, in_app=%d sms=%d third_party=%d", len(inApp.events), len(sms.events), len(thirdParty.events))
	}
}

func TestAlertDeliveryFanOutReturnsMissingSinkResult(t *testing.T) {
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DefaultAlertDeliveryPolicy(),
		Sinks: []AlertDeliverySink{
			&captureDeliverySink{channel: AlertDeliveryChannelEmail},
			&captureDeliverySink{channel: AlertDeliveryChannelIM},
			&captureDeliverySink{channel: AlertDeliveryChannelInApp},
		},
	})

	results := dispatcher.Deliver(context.Background(), AlertEvent{
		Key:      "database-down",
		Severity: AlertSeverityCritical,
		Title:    "Database down",
		Message:  "primary database unavailable",
	})

	if len(results) != 5 {
		t.Fatalf("expected one result per critical channel, got %d: %+v", len(results), results)
	}
	smsResult := resultForChannel(t, results, AlertDeliveryChannelSMS)
	if smsResult.Err == nil {
		t.Fatalf("expected missing SMS sink to be reported: %+v", results)
	}
	thirdPartyResult := resultForChannel(t, results, AlertDeliveryChannelThirdParty)
	if thirdPartyResult.Err == nil {
		t.Fatalf("expected missing third-party sink to be reported: %+v", results)
	}
	phoneResult := resultForChannel(t, results, AlertDeliveryChannelPhone)
	if phoneResult.Err == nil {
		t.Fatalf("expected missing phone sink to be reported: %+v", results)
	}
}

func TestAlertDeliveryDispatcherRecordsDeliveryHistory(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	email := &captureDeliverySink{channel: AlertDeliveryChannelEmail}
	im := &captureDeliverySink{channel: AlertDeliveryChannelIM, err: errors.New("im webhook failed")}
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DefaultAlertDeliveryPolicy(),
		Sinks: []AlertDeliverySink{
			email,
			im,
			&captureDeliverySink{channel: AlertDeliveryChannelInApp},
		},
		HistoryStore: store,
	})
	event := AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Queue backlog",
		Message:    "queue depth is high",
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC),
	}

	results := dispatcher.Deliver(ctx, event)

	if len(results) != 2 {
		t.Fatalf("expected one result per warning channel, got %d: %+v", len(results), results)
	}
	history, err := store.ListDeliveryAttempts(ctx, AlertDeliveryHistoryFilter{AlertKey: "queue-backlog"})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected three delivery attempts, got %+v", history)
	}
	if history[0].Channel != AlertDeliveryChannelEmail || !history[0].Delivered || history[0].Error != "" {
		t.Fatalf("expected successful email attempt first, got %+v", history[0])
	}
	if history[1].Channel != AlertDeliveryChannelIM || history[1].Delivered || history[1].Error != "im webhook failed" {
		t.Fatalf("expected failed IM attempt second, got %+v", history[1])
	}
	if !history[0].AttemptedAt.Equal(event.OccurredAt) || history[0].Severity != AlertSeverityWarning || history[0].Component != "relay" {
		t.Fatalf("expected alert context on delivery attempt, got %+v", history[0])
	}
}

func TestAlertDeliveryDispatcherSuppressesRepeatedNotificationInsideWindow(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	email := &captureDeliverySink{channel: AlertDeliveryChannelEmail}
	im := &captureDeliverySink{channel: AlertDeliveryChannelIM}
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityWarning: {AlertDeliveryChannelEmail, AlertDeliveryChannelIM},
			},
		},
		Sinks:             []AlertDeliverySink{email, im},
		HistoryStore:      store,
		NotificationStore: store,
	})
	first := AlertEvent{
		Key:        "queue-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Queue backlog",
		Message:    "queue depth is high",
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC),
	}
	repeated := first
	repeated.OccurredAt = first.OccurredAt.Add(10 * time.Minute)

	firstResults := dispatcher.Deliver(ctx, first)
	repeatedResults := dispatcher.Deliver(ctx, repeated)

	if len(firstResults) != 2 {
		t.Fatalf("expected initial warning to fan out to both channels, got %+v", firstResults)
	}
	if len(repeatedResults) != 0 {
		t.Fatalf("expected repeated notification inside warning window to be suppressed, got %+v", repeatedResults)
	}
	if len(email.events) != 1 || len(im.events) != 1 {
		t.Fatalf("expected sinks to be called only for initial notification, email=%d im=%d", len(email.events), len(im.events))
	}
	attempts, err := store.ListDeliveryAttempts(ctx, AlertDeliveryHistoryFilter{AlertKey: "queue-backlog"})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected only initial delivery attempts to be recorded, got %+v", attempts)
	}
}

func TestAlertProviderDeliveryResolverPostsActiveSlackWebhookProvider(t *testing.T) {
	ctx := context.Background()
	var postedPath string
	var postedPayload map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postedPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("expected Slack webhook POST, got %s", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected JSON content type, got %s", contentType)
		}
		if err := json.NewDecoder(r.Body).Decode(&postedPayload); err != nil {
			t.Errorf("decode Slack webhook payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	providerStore := NewInMemoryAlertProviderConfigStore(AlertProviderConfig{
		ID:     "alert_provider_slack_primary",
		Kind:   AlertProviderKindSlackWebhook,
		Name:   "Primary Slack",
		Status: AlertProviderStatusActive,
		Config: map[string]string{
			"webhook_url": upstream.URL + "/slack",
		},
	})
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityWarning: {AlertDeliveryChannelIM},
			},
		},
		SinkResolver: NewAlertProviderDeliverySinkResolver(AlertProviderDeliverySinkResolverOptions{
			ProviderStore: providerStore,
			HTTPClient:    upstream.Client(),
		}),
	})

	results := dispatcher.Deliver(ctx, AlertEvent{
		Key:        "relay-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Relay backlog",
		Message:    "queue depth is high",
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC),
	})

	if len(results) != 1 {
		t.Fatalf("expected one Slack provider result, got %+v", results)
	}
	if result := results[0]; !result.Delivered || result.Channel != AlertDeliveryChannelIM || result.ProviderID != "alert_provider_slack_primary" || result.ProviderKind != AlertProviderKindSlackWebhook || result.Err != nil {
		t.Fatalf("expected successful Slack provider delivery result, got %+v", result)
	}
	if postedPath != "/slack" {
		t.Fatalf("expected Slack webhook path /slack, got %q", postedPath)
	}
	blocks, ok := postedPayload["blocks"].([]any)
	if !ok || len(blocks) == 0 {
		t.Fatalf("expected Slack rich markdown blocks, got %+v", postedPayload)
	}
	section, ok := blocks[0].(map[string]any)
	if !ok || section["type"] != "section" {
		t.Fatalf("expected Slack first block to be a section, got %+v", blocks[0])
	}
	text, ok := section["text"].(map[string]any)
	if !ok || text["type"] != "mrkdwn" {
		t.Fatalf("expected Slack section to use mrkdwn text, got %+v", section)
	}
	markdown := text["text"].(string)
	if !strings.Contains(markdown, "*[WARNING] Relay backlog*") || !strings.Contains(markdown, "queue depth is high") || !strings.Contains(markdown, "`component` relay") {
		t.Fatalf("expected Slack markdown payload with alert title, message, and component, got %+v", postedPayload)
	}
}

func TestAlertProviderConfigRequiresSMTPEnvelopeFields(t *testing.T) {
	_, err := NormalizeAlertProviderConfig(AlertProviderConfig{
		ID:     "alert_provider_smtp_primary",
		Kind:   AlertProviderKindSMTP,
		Name:   "Primary SMTP",
		Status: AlertProviderStatusActive,
		Config: map[string]string{
			"smtp_host": "smtp.example.com",
			"smtp_port": "587",
			"username":  "alerts@example.com",
			"password":  "smtp-secret",
		},
	})

	if err == nil || !strings.Contains(err.Error(), "config.from_email") {
		t.Fatalf("expected smtp provider to require from_email before it can deliver, got %v", err)
	}
}

func TestAlertProviderDeliveryResolverSendsSMTPEmailProvider(t *testing.T) {
	ctx := context.Background()
	address, messages, closeServer := startFakeSMTPServer(t)
	defer closeServer()
	host, port, _ := strings.Cut(address, ":")
	providerStore := NewInMemoryAlertProviderConfigStore(AlertProviderConfig{
		ID:     "alert_provider_smtp_primary",
		Kind:   AlertProviderKindSMTP,
		Name:   "Primary SMTP",
		Status: AlertProviderStatusActive,
		Config: map[string]string{
			"smtp_host":  host,
			"smtp_port":  port,
			"username":   "alerts@example.com",
			"password":   "smtp-secret",
			"from_email": "alerts@example.com",
			"recipients": "ops@example.com, oncall@example.com",
		},
	})
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityWarning: {AlertDeliveryChannelEmail},
			},
		},
		SinkResolver: NewAlertProviderDeliverySinkResolver(AlertProviderDeliverySinkResolverOptions{
			ProviderStore: providerStore,
		}),
	})

	results := dispatcher.Deliver(ctx, AlertEvent{
		Key:        "relay-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Relay backlog",
		Message:    "queue depth is high",
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC),
	})

	if len(results) != 1 {
		t.Fatalf("expected one SMTP provider result, got %+v", results)
	}
	if result := results[0]; !result.Delivered || result.Channel != AlertDeliveryChannelEmail || result.ProviderID != "alert_provider_smtp_primary" || result.ProviderKind != AlertProviderKindSMTP || result.Err != nil {
		t.Fatalf("expected successful SMTP provider delivery result, got %+v", result)
	}
	gotMessages := messages()
	if len(gotMessages) != 1 {
		t.Fatalf("expected one SMTP message, got %+v", gotMessages)
	}
	if gotMessages[0].from != "alerts@example.com" {
		t.Fatalf("expected SMTP from address to be preserved, got %+v", gotMessages[0])
	}
	if strings.Join(gotMessages[0].recipients, ",") != "ops@example.com,oncall@example.com" {
		t.Fatalf("expected SMTP recipients to be parsed, got %+v", gotMessages[0])
	}
	if !strings.Contains(gotMessages[0].data, "Subject: [WARNING] Relay backlog") || !strings.Contains(gotMessages[0].data, "queue depth is high") {
		t.Fatalf("expected SMTP email body with alert content, got %q", gotMessages[0].data)
	}
}

func TestProbeAlertProviderConfigSendsSMTPEmail(t *testing.T) {
	address, messages, closeServer := startFakeSMTPServer(t)
	defer closeServer()
	host, port, _ := strings.Cut(address, ":")
	config, err := NormalizeAlertProviderConfig(AlertProviderConfig{
		ID:     "alert_provider_smtp_primary",
		Kind:   AlertProviderKindSMTP,
		Name:   "Primary SMTP",
		Status: AlertProviderStatusActive,
		Config: map[string]string{
			"smtp_host":  host,
			"smtp_port":  port,
			"username":   "alerts@example.com",
			"password":   "smtp-secret",
			"from_email": "alerts@example.com",
			"recipients": "ops@example.com",
		},
	})
	if err != nil {
		t.Fatalf("normalize smtp config: %v", err)
	}

	result := ProbeAlertProviderConfig(context.Background(), config, time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC), nil)

	if !result.OK || result.ProviderID != config.ID || result.Kind != AlertProviderKindSMTP {
		t.Fatalf("expected successful SMTP probe result, got %+v", result)
	}
	gotMessages := messages()
	if len(gotMessages) != 1 {
		t.Fatalf("expected SMTP provider probe to send a test email, got %+v", gotMessages)
	}
	if !strings.Contains(gotMessages[0].data, "Test alert provider: Primary SMTP") {
		t.Fatalf("expected SMTP probe email to contain test alert title, got %q", gotMessages[0].data)
	}
}

func TestAlertProviderDeliveryResolverFansOutActiveProvidersForSameChannel(t *testing.T) {
	ctx := context.Background()
	postedPaths := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postedPaths = append(postedPaths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	providerStore := NewInMemoryAlertProviderConfigStore(
		AlertProviderConfig{
			ID:     "alert_provider_slack_primary",
			Kind:   AlertProviderKindSlackWebhook,
			Name:   "Primary Slack",
			Status: AlertProviderStatusActive,
			Config: map[string]string{"webhook_url": upstream.URL + "/primary"},
		},
		AlertProviderConfig{
			ID:     "alert_provider_slack_secondary",
			Kind:   AlertProviderKindSlackWebhook,
			Name:   "Secondary Slack",
			Status: AlertProviderStatusActive,
			Config: map[string]string{"webhook_url": upstream.URL + "/secondary"},
		},
		AlertProviderConfig{
			ID:     "alert_provider_slack_disabled",
			Kind:   AlertProviderKindSlackWebhook,
			Name:   "Disabled Slack",
			Status: AlertProviderStatusDisabled,
			Config: map[string]string{"webhook_url": upstream.URL + "/disabled"},
		},
	)
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityWarning: {AlertDeliveryChannelIM},
			},
		},
		SinkResolver: NewAlertProviderDeliverySinkResolver(AlertProviderDeliverySinkResolverOptions{
			ProviderStore: providerStore,
			HTTPClient:    upstream.Client(),
		}),
	})

	results := dispatcher.Deliver(ctx, AlertEvent{
		Key:      "relay-backlog",
		Severity: AlertSeverityWarning,
		Title:    "Relay backlog",
	})

	if len(results) != 2 {
		t.Fatalf("expected one result for each active Slack provider, got %+v", results)
	}
	if got := strings.Join(postedPaths, ","); got != "/primary,/secondary" {
		t.Fatalf("expected both active providers and no disabled provider to be called, got %s", got)
	}
	for _, result := range results {
		if !result.Delivered || result.Err != nil || result.Channel != AlertDeliveryChannelIM || result.ProviderKind != AlertProviderKindSlackWebhook {
			t.Fatalf("expected successful IM provider result, got %+v", result)
		}
	}
}

func TestAlertProviderDeliveryResolverPostsNativeIMWebhookProviders(t *testing.T) {
	cases := []struct {
		name       string
		kind       AlertProviderKind
		wantPath   string
		assertBody func(t *testing.T, payload map[string]any)
	}{
		{
			name:     "feishu",
			kind:     AlertProviderKindFeishuWebhook,
			wantPath: "/feishu",
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				card, ok := payload["card"].(map[string]any)
				if payload["msg_type"] != "interactive" || !ok {
					t.Fatalf("expected Feishu interactive markdown payload, got %+v", payload)
				}
				elements, ok := card["elements"].([]any)
				if !ok || len(elements) == 0 {
					t.Fatalf("expected Feishu card elements, got %+v", payload)
				}
				markdown, ok := elements[0].(map[string]any)
				if !ok || markdown["tag"] != "markdown" || !strings.Contains(markdown["content"].(string), "**[WARNING] Relay backlog**") {
					t.Fatalf("expected Feishu markdown element, got %+v", payload)
				}
			},
		},
		{
			name:     "dingtalk",
			kind:     AlertProviderKindDingTalk,
			wantPath: "/dingtalk",
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				markdown, ok := payload["markdown"].(map[string]any)
				if payload["msgtype"] != "markdown" || !ok || !strings.Contains(markdown["text"].(string), "**[WARNING] Relay backlog**") {
					t.Fatalf("expected DingTalk markdown payload, got %+v", payload)
				}
			},
		},
		{
			name:     "wecom",
			kind:     AlertProviderKindWeComWebhook,
			wantPath: "/wecom",
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				markdown, ok := payload["markdown"].(map[string]any)
				if payload["msgtype"] != "markdown" || !ok || !strings.Contains(markdown["content"].(string), "**[WARNING] Relay backlog**") {
					t.Fatalf("expected WeCom markdown payload, got %+v", payload)
				}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var postedPayload map[string]any
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected IM webhook POST, got %s", r.Method)
				}
				if r.URL.Path != tt.wantPath {
					t.Errorf("expected %s path, got %s", tt.wantPath, r.URL.Path)
				}
				if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
					t.Errorf("expected JSON content type, got %s", contentType)
				}
				if err := json.NewDecoder(r.Body).Decode(&postedPayload); err != nil {
					t.Errorf("decode IM webhook payload: %v", err)
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer upstream.Close()
			providerStore := NewInMemoryAlertProviderConfigStore(AlertProviderConfig{
				ID:     "alert_provider_" + tt.name,
				Kind:   tt.kind,
				Name:   tt.name + " ops",
				Status: AlertProviderStatusActive,
				Config: map[string]string{
					"webhook_url": upstream.URL + tt.wantPath,
				},
			})
			dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
				Policy: DeliveryPolicy{
					Routes: map[AlertSeverity][]AlertDeliveryChannel{
						AlertSeverityWarning: {AlertDeliveryChannelIM},
					},
				},
				SinkResolver: NewAlertProviderDeliverySinkResolver(AlertProviderDeliverySinkResolverOptions{
					ProviderStore: providerStore,
					HTTPClient:    upstream.Client(),
				}),
			})

			results := dispatcher.Deliver(ctx, AlertEvent{
				Key:       "relay-backlog",
				Severity:  AlertSeverityWarning,
				Title:     "Relay backlog",
				Message:   "queue depth is high",
				Component: "relay",
			})

			if len(results) != 1 || !results[0].Delivered || results[0].ProviderKind != tt.kind || results[0].Err != nil {
				t.Fatalf("expected successful %s provider delivery, got %+v", tt.kind, results)
			}
			tt.assertBody(t, postedPayload)
		})
	}
}

func TestAlertProviderDeliveryResolverPostsThirdPartyProviders(t *testing.T) {
	cases := []struct {
		name        string
		kind        AlertProviderKind
		config      func(url string) map[string]string
		assertReq   func(t *testing.T, r *http.Request)
		assertBody  func(t *testing.T, payload map[string]any)
		expectedURL string
	}{
		{
			name: "pagerduty",
			kind: AlertProviderKindPagerDuty,
			config: func(url string) map[string]string {
				return map[string]string{
					"routing_key": "pd-routing-key",
					"api_url":     url + "/pagerduty",
				}
			},
			assertReq: func(t *testing.T, r *http.Request) {
				t.Helper()
				if r.URL.Path != "/pagerduty" {
					t.Fatalf("expected PagerDuty request path /pagerduty, got %s", r.URL.Path)
				}
			},
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				details, ok := payload["payload"].(map[string]any)
				if payload["routing_key"] != "pd-routing-key" || payload["event_action"] != "trigger" || !ok || !strings.Contains(details["summary"].(string), "Relay backlog") {
					t.Fatalf("expected PagerDuty Events API payload, got %+v", payload)
				}
			},
		},
		{
			name: "opsgenie",
			kind: AlertProviderKindOpsgenie,
			config: func(url string) map[string]string {
				return map[string]string{
					"api_key": "opsgenie-key",
					"api_url": url + "/opsgenie",
				}
			},
			assertReq: func(t *testing.T, r *http.Request) {
				t.Helper()
				if r.URL.Path != "/opsgenie" {
					t.Fatalf("expected Opsgenie request path /opsgenie, got %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "GenieKey opsgenie-key" {
					t.Fatalf("expected Opsgenie GenieKey authorization, got %q", r.Header.Get("Authorization"))
				}
			},
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if !strings.Contains(payload["message"].(string), "Relay backlog") || payload["alias"] != "relay-backlog" {
					t.Fatalf("expected Opsgenie alert payload, got %+v", payload)
				}
			},
		},
		{
			name: "aliyun_monitor",
			kind: AlertProviderKindAliyunMonitor,
			config: func(url string) map[string]string {
				return map[string]string{"webhook_url": url + "/aliyun-monitor"}
			},
			assertReq: func(t *testing.T, r *http.Request) {
				t.Helper()
				if r.URL.Path != "/aliyun-monitor" {
					t.Fatalf("expected Aliyun monitor webhook path, got %s", r.URL.Path)
				}
			},
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["alert_key"] != "relay-backlog" || payload["provider"] != string(AlertProviderKindAliyunMonitor) {
					t.Fatalf("expected cloud monitor webhook payload, got %+v", payload)
				}
			},
		},
		{
			name: "tencent_cloud_monitor",
			kind: AlertProviderKindTencentCloud,
			config: func(url string) map[string]string {
				return map[string]string{"webhook_url": url + "/tencent-monitor"}
			},
			assertReq: func(t *testing.T, r *http.Request) {
				t.Helper()
				if r.URL.Path != "/tencent-monitor" {
					t.Fatalf("expected Tencent Cloud monitor webhook path, got %s", r.URL.Path)
				}
			},
			assertBody: func(t *testing.T, payload map[string]any) {
				t.Helper()
				if payload["alert_key"] != "relay-backlog" || payload["provider"] != string(AlertProviderKindTencentCloud) {
					t.Fatalf("expected cloud monitor webhook payload, got %+v", payload)
				}
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var postedPayload map[string]any
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected third-party provider POST, got %s", r.Method)
				}
				if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
					t.Errorf("expected JSON content type, got %s", contentType)
				}
				tt.assertReq(t, r)
				if err := json.NewDecoder(r.Body).Decode(&postedPayload); err != nil {
					t.Errorf("decode third-party payload: %v", err)
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer upstream.Close()
			providerStore := NewInMemoryAlertProviderConfigStore(AlertProviderConfig{
				ID:     "alert_provider_" + tt.name,
				Kind:   tt.kind,
				Name:   tt.name + " ops",
				Status: AlertProviderStatusActive,
				Config: tt.config(upstream.URL),
			})
			dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
				Policy: DeliveryPolicy{
					Routes: map[AlertSeverity][]AlertDeliveryChannel{
						AlertSeverityCritical: {AlertDeliveryChannelThirdParty},
					},
				},
				SinkResolver: NewAlertProviderDeliverySinkResolver(AlertProviderDeliverySinkResolverOptions{
					ProviderStore: providerStore,
					HTTPClient:    upstream.Client(),
				}),
			})

			results := dispatcher.Deliver(ctx, AlertEvent{
				Key:       "relay-backlog",
				Severity:  AlertSeverityCritical,
				Title:     "Relay backlog",
				Message:   "queue depth is high",
				Component: "relay",
			})

			if len(results) != 1 || !results[0].Delivered || results[0].ProviderKind != tt.kind || results[0].Channel != AlertDeliveryChannelThirdParty || results[0].Err != nil {
				t.Fatalf("expected successful %s provider delivery, got %+v", tt.kind, results)
			}
			tt.assertBody(t, postedPayload)
		})
	}
}

func TestAlertProviderDeliveryResolverPostsSMSAndPhoneProvidersWithRecipientLimits(t *testing.T) {
	ctx := context.Background()
	occurredAt := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	var (
		mu          sync.Mutex
		twilioSMS   []url.Values
		aliyunSMS   []map[string]any
		twilioCalls []url.Values
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected provider POST, got %s", r.Method)
		}
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.Contains(r.URL.Path, "/Messages.json"):
			if r.Header.Get("Authorization") == "" {
				t.Errorf("expected Twilio SMS basic auth header")
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse Twilio SMS form: %v", err)
			}
			twilioSMS = append(twilioSMS, cloneURLValues(r.PostForm))
		case strings.Contains(r.URL.Path, "/aliyun-sms"):
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode Aliyun SMS payload: %v", err)
			}
			aliyunSMS = append(aliyunSMS, payload)
		case strings.Contains(r.URL.Path, "/Calls.json"):
			if r.Header.Get("Authorization") == "" {
				t.Errorf("expected Twilio call basic auth header")
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse Twilio call form: %v", err)
			}
			twilioCalls = append(twilioCalls, cloneURLValues(r.PostForm))
		default:
			t.Errorf("unexpected provider path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer upstream.Close()

	providerStore := NewInMemoryAlertProviderConfigStore(
		AlertProviderConfig{
			ID:     "alert_provider_twilio_sms",
			Kind:   AlertProviderKindTwilioSMS,
			Name:   "Twilio SMS",
			Status: AlertProviderStatusActive,
			Config: map[string]string{
				"account_sid": "AC123",
				"auth_token":  "twilio-secret",
				"from_number": "+15550000000",
				"recipients":  "+15550000001,+15550000002",
				"api_url":     upstream.URL,
			},
		},
		AlertProviderConfig{
			ID:     "alert_provider_aliyun_sms",
			Kind:   AlertProviderKindAliyunSMS,
			Name:   "Aliyun SMS",
			Status: AlertProviderStatusActive,
			Config: map[string]string{
				"access_key_id":     "aliyun-key",
				"access_key_secret": "aliyun-secret",
				"sign_name":         "Oblivious",
				"template_code":     "SMS_123",
				"recipients":        "+8613800000000",
				"api_url":           upstream.URL + "/aliyun-sms",
			},
		},
		AlertProviderConfig{
			ID:     "alert_provider_phone",
			Kind:   AlertProviderKindPhone,
			Name:   "On-call phone",
			Status: AlertProviderStatusActive,
			Config: map[string]string{
				"provider":      "twilio",
				"account_sid":   "AC123",
				"auth_token":    "twilio-secret",
				"from_number":   "+15550000000",
				"phone_numbers": "+15550000003",
				"api_url":       upstream.URL,
			},
		},
	)
	resolver := NewAlertProviderDeliverySinkResolver(AlertProviderDeliverySinkResolverOptions{
		ProviderStore: providerStore,
		HTTPClient:    upstream.Client(),
	})
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityCritical: {AlertDeliveryChannelSMS, AlertDeliveryChannelPhone},
			},
		},
		SinkResolver: resolver,
	})

	firstResults := dispatcher.Deliver(ctx, AlertEvent{
		Key:        "database-down",
		Severity:   AlertSeverityCritical,
		Title:      "Database down",
		Message:    "primary database unavailable",
		Component:  "database",
		OccurredAt: occurredAt,
	})
	if len(firstResults) != 3 {
		t.Fatalf("expected Twilio SMS, Aliyun SMS, and phone results, got %+v", firstResults)
	}
	for _, result := range firstResults {
		if result.Err != nil || !result.Delivered {
			t.Fatalf("expected initial SMS/phone delivery to succeed, got %+v", firstResults)
		}
	}

	mu.Lock()
	if len(twilioSMS) != 2 || len(aliyunSMS) != 1 || len(twilioCalls) != 1 {
		t.Fatalf("unexpected initial provider sends, twilioSMS=%+v aliyunSMS=%+v twilioCalls=%+v", twilioSMS, aliyunSMS, twilioCalls)
	}
	if twilioSMS[0].Get("From") != "+15550000000" || twilioSMS[0].Get("To") != "+15550000001" || !strings.Contains(twilioSMS[0].Get("Body"), "Database down") {
		t.Fatalf("unexpected Twilio SMS payload: %+v", twilioSMS[0])
	}
	if aliyunSMS[0]["template_code"] != "SMS_123" || aliyunSMS[0]["phone_numbers"] != "+8613800000000" {
		t.Fatalf("unexpected Aliyun SMS payload: %+v", aliyunSMS[0])
	}
	if twilioCalls[0].Get("To") != "+15550000003" || !strings.Contains(twilioCalls[0].Get("Twiml"), "Database down") {
		t.Fatalf("unexpected Twilio call payload: %+v", twilioCalls[0])
	}
	mu.Unlock()

	for i := 1; i < 5; i++ {
		results := dispatcher.Deliver(ctx, AlertEvent{
			Key:        fmt.Sprintf("database-down-%d", i),
			Severity:   AlertSeverityCritical,
			Title:      "Database down",
			Message:    "primary database unavailable",
			Component:  "database",
			OccurredAt: occurredAt.Add(time.Duration(i) * time.Minute),
		})
		if len(results) != 3 {
			t.Fatalf("expected SMS and phone provider results on send %d, got %+v", i+1, results)
		}
	}

	limitedResults := dispatcher.Deliver(ctx, AlertEvent{
		Key:        "database-down-limit",
		Severity:   AlertSeverityCritical,
		Title:      "Database down",
		Message:    "primary database unavailable",
		Component:  "database",
		OccurredAt: occurredAt.Add(6 * time.Minute),
	})
	if len(limitedResults) != 3 {
		t.Fatalf("expected SMS and phone provider results after limit, got %+v", limitedResults)
	}
	for _, result := range limitedResults {
		if result.Err == nil || !strings.Contains(result.Err.Error(), "recipient hourly limit") {
			t.Fatalf("expected recipient hourly limit result, got %+v", limitedResults)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(twilioSMS) != 10 {
		t.Fatalf("expected Twilio SMS to stop after 5 messages per recipient, got %d sends", len(twilioSMS))
	}
	if len(aliyunSMS) != 5 {
		t.Fatalf("expected Aliyun SMS to stop after 5 messages per recipient, got %d sends", len(aliyunSMS))
	}
	if len(twilioCalls) != 1 {
		t.Fatalf("expected Twilio calls to stop after 1 call per recipient, got %d sends", len(twilioCalls))
	}
}

func cloneURLValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, value := range values {
		cloned[key] = append([]string{}, value...)
	}
	return cloned
}

func TestAlertRouterDeliversInAppNotificationAndRecordsStateAndAttempt(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	notificationStore := &alertNotificationStore{}
	notifier := notification.NewService(notificationStore)
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityWarning: {AlertDeliveryChannelInApp},
			},
		},
		Sinks: []AlertDeliverySink{
			NewInAppAlertDeliverySink(AlertInAppDeliverySinkOptions{
				Notifier:         notifier,
				RecipientUserIDs: []string{"admin_user"},
				ActionURL:        "/admin/alerts",
			}),
		},
		HistoryStore: store,
	})
	router := NewAlertRouter(AlertRouterOptions{
		StateStore: store,
		NotifySink: dispatcher,
		Now:        func() time.Time { return time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC) },
	})

	if err := router.Route(ctx, AlertEvent{
		Key:       "relay-backlog",
		Severity:  AlertSeverityWarning,
		Title:     "Relay backlog",
		Message:   "queue depth is high",
		Component: "relay",
	}); err != nil {
		t.Fatalf("route warning alert: %v", err)
	}

	if len(notificationStore.notifications) != 1 {
		t.Fatalf("expected one in-app notification, got %+v", notificationStore.notifications)
	}
	created := notificationStore.notifications[0]
	if created.UserID != "admin_user" || created.Type != "warning" || created.Category != "system" {
		t.Fatalf("unexpected notification envelope: %+v", created)
	}
	if created.Title != "Relay backlog" || created.Message != "queue depth is high" || created.ActionURL != "/admin/alerts" {
		t.Fatalf("alert notification content was not preserved: %+v", created)
	}
	if created.Metadata["event"] != "observability.alert.fired" ||
		created.Metadata["alertKey"] != "relay-backlog" ||
		created.Metadata["severity"] != string(AlertSeverityWarning) ||
		created.Metadata["component"] != "relay" {
		t.Fatalf("expected alert metadata on notification, got %+v", created.Metadata)
	}

	state, found, err := store.GetAlertState(ctx, "relay-backlog")
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != AlertStatusOpen || state.Severity != AlertSeverityWarning {
		t.Fatalf("expected open warning alert state, got found=%v state=%+v", found, state)
	}

	attempts, err := store.ListDeliveryAttempts(ctx, AlertDeliveryHistoryFilter{AlertKey: "relay-backlog"})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Channel != AlertDeliveryChannelInApp || !attempts[0].Delivered || attempts[0].Error != "" {
		t.Fatalf("expected successful in-app delivery attempt, got %+v", attempts)
	}
}

func TestAlertInAppDeliverySinkRecordsFailedNotificationAttempt(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryAlertStateStore()
	notificationStore := &alertNotificationStore{err: errors.New("insert notification failed")}
	notifier := notification.NewService(notificationStore)
	dispatcher := NewAlertDeliveryDispatcher(AlertDeliveryDispatcherOptions{
		Policy: DeliveryPolicy{
			Routes: map[AlertSeverity][]AlertDeliveryChannel{
				AlertSeverityWarning: {AlertDeliveryChannelInApp},
			},
		},
		Sinks: []AlertDeliverySink{
			NewInAppAlertDeliverySink(AlertInAppDeliverySinkOptions{
				Notifier:         notifier,
				RecipientUserIDs: []string{"admin_user"},
			}),
		},
		HistoryStore: store,
	})
	router := NewAlertRouter(AlertRouterOptions{
		StateStore: store,
		NotifySink: dispatcher,
	})

	err := router.Route(ctx, AlertEvent{
		Key:        "relay-backlog",
		Severity:   AlertSeverityWarning,
		Title:      "Relay backlog",
		Message:    "queue depth is high",
		Component:  "relay",
		OccurredAt: time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC),
	})

	if err == nil || !strings.Contains(err.Error(), "insert notification failed") {
		t.Fatalf("expected notification delivery failure, got %v", err)
	}
	state, found, err := store.GetAlertState(ctx, "relay-backlog")
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != AlertStatusOpen {
		t.Fatalf("expected alert state to be opened despite delivery failure, found=%v state=%+v", found, state)
	}
	attempts, err := store.ListDeliveryAttempts(ctx, AlertDeliveryHistoryFilter{AlertKey: "relay-backlog"})
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Delivered || !strings.Contains(attempts[0].Error, "insert notification failed") {
		t.Fatalf("expected failed in-app delivery attempt, got %+v", attempts)
	}
}

func TestWebhookAlertDeliverySinkPostsSignedAlertPayload(t *testing.T) {
	var postedBody []byte
	var postedSignature string
	var postedKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/ops-alerts" {
			t.Errorf("expected /ops-alerts path, got %s", r.URL.Path)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected JSON content type, got %s", contentType)
		}
		postedSignature = r.Header.Get("X-Oblivious-Alert-Signature")
		postedKey = r.Header.Get("X-Oblivious-Alert-Key")
		var err error
		postedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()
	occurredAt := time.Date(2026, 6, 6, 11, 15, 0, 0, time.UTC)
	sink := NewWebhookAlertDeliverySink(AlertWebhookDeliverySinkOptions{
		EndpointURL: upstream.URL + "/ops-alerts",
		Secret:      "ops-secret",
		HTTPClient:  upstream.Client(),
	})

	if sink.Channel() != AlertDeliveryChannelThirdParty {
		t.Fatalf("expected webhook sink to register as third-party channel, got %s", sink.Channel())
	}
	if err := sink.Deliver(context.Background(), AlertEvent{
		Key:        "relay-backlog",
		Severity:   AlertSeverityCritical,
		Title:      "Relay backlog",
		Message:    "queue depth is above 1000",
		Component:  "relay",
		OccurredAt: occurredAt,
		Fields:     map[string]any{"queueDepth": 1280},
	}); err != nil {
		t.Fatalf("deliver webhook alert: %v", err)
	}

	if postedKey != "relay-backlog" {
		t.Fatalf("expected alert key header, got %q", postedKey)
	}
	mac := hmac.New(sha256.New, []byte("ops-secret"))
	mac.Write(postedBody)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if postedSignature != expectedSignature {
		t.Fatalf("expected HMAC signature %s, got %s", expectedSignature, postedSignature)
	}

	var payload map[string]any
	if err := json.Unmarshal(postedBody, &payload); err != nil {
		t.Fatalf("decode webhook payload: %v\n%s", err, string(postedBody))
	}
	if payload["event"] != "observability.alert.fired" ||
		payload["key"] != "relay-backlog" ||
		payload["severity"] != string(AlertSeverityCritical) ||
		payload["title"] != "Relay backlog" ||
		payload["message"] != "queue depth is above 1000" ||
		payload["component"] != "relay" ||
		payload["occurredAt"] != occurredAt.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected webhook payload: %+v", payload)
	}
	fields, ok := payload["fields"].(map[string]any)
	if !ok || fields["queueDepth"] != float64(1280) {
		t.Fatalf("expected alert fields in webhook payload, got %+v", payload["fields"])
	}
}

func TestMakeAlertDeliveryAttemptIDUsesBatchContext(t *testing.T) {
	firstBatch := makeAlertDeliveryAttemptID("queue-backlog", time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC), AlertDeliveryChannelEmail, 1)
	secondBatch := makeAlertDeliveryAttemptID("queue-backlog", time.Date(2026, 6, 5, 8, 35, 0, 0, time.UTC), AlertDeliveryChannelEmail, 1)
	sameBatchDifferentChannel := makeAlertDeliveryAttemptID("queue-backlog", time.Date(2026, 6, 5, 8, 30, 0, 0, time.UTC), AlertDeliveryChannelInApp, 2)

	if firstBatch == secondBatch {
		t.Fatalf("expected repeated delivery batches for the same alert to use different IDs, got %q", firstBatch)
	}
	if firstBatch == sameBatchDifferentChannel {
		t.Fatalf("expected attempts within a batch to use different IDs, got %q", firstBatch)
	}
}

func sameDeliveryChannels(a, b []AlertDeliveryChannel) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func deliveryResultChannels(results []AlertDeliveryResult) []AlertDeliveryChannel {
	channels := make([]AlertDeliveryChannel, 0, len(results))
	for _, result := range results {
		channels = append(channels, result.Channel)
	}
	return channels
}

func resultForChannel(t *testing.T, results []AlertDeliveryResult, channel AlertDeliveryChannel) AlertDeliveryResult {
	t.Helper()
	for _, result := range results {
		if result.Channel == channel {
			return result
		}
	}
	t.Fatalf("missing result for channel %s in %+v", channel, results)
	return AlertDeliveryResult{}
}
