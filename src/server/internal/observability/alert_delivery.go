package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type AlertDeliveryChannel string

const (
	AlertDeliveryChannelEmail      AlertDeliveryChannel = "email"
	AlertDeliveryChannelIM         AlertDeliveryChannel = "im"
	AlertDeliveryChannelSMS        AlertDeliveryChannel = "sms"
	AlertDeliveryChannelThirdParty AlertDeliveryChannel = "third_party"
	AlertDeliveryChannelPhone      AlertDeliveryChannel = "phone"
	AlertDeliveryChannelInApp      AlertDeliveryChannel = "in_app"
)

var ErrAlertDeliverySinkMissing = errors.New("alert delivery sink missing")

type AlertDeliverySink interface {
	Channel() AlertDeliveryChannel
	Deliver(ctx context.Context, event AlertEvent) error
}

type AlertDeliverySinkResolver interface {
	SinksForChannel(ctx context.Context, channel AlertDeliveryChannel) ([]AlertDeliverySink, error)
}

type AlertProviderDeliveryMetadata interface {
	ProviderID() string
	ProviderKind() AlertProviderKind
}

type DeliveryPolicy struct {
	Routes                  map[AlertSeverity][]AlertDeliveryChannel
	UnknownSeverityFallback AlertSeverity
}

func DefaultAlertDeliveryPolicy() DeliveryPolicy {
	return DeliveryPolicy{
		Routes:                  map[AlertSeverity][]AlertDeliveryChannel(DefaultAlertRoutingRules()),
		UnknownSeverityFallback: AlertSeverityWarning,
	}
}

func (p DeliveryPolicy) ChannelsForSeverity(severity AlertSeverity) []AlertDeliveryChannel {
	p = normalizeDeliveryPolicy(p)

	channels, ok := p.Routes[severity]
	if !ok {
		channels = p.Routes[p.UnknownSeverityFallback]
	}
	return copyDeliveryChannels(channels)
}

type AlertDeliveryDispatcherOptions struct {
	Policy            DeliveryPolicy
	RoutingRules      AlertRoutingRuleStore
	Sinks             []AlertDeliverySink
	SinkResolver      AlertDeliverySinkResolver
	HistoryStore      AlertDeliveryHistoryStore
	NotificationStore AlertNotificationStateStore
	NotifyWindows     map[AlertSeverity]time.Duration
	InfoDigestWindow  time.Duration
}

type AlertDeliveryDispatcher struct {
	policy            DeliveryPolicy
	routingRules      AlertRoutingRuleStore
	sinks             map[AlertDeliveryChannel][]AlertDeliverySink
	sinkResolver      AlertDeliverySinkResolver
	historyStore      AlertDeliveryHistoryStore
	notificationStore AlertNotificationStateStore
	notifyWindows     map[AlertSeverity]time.Duration
	infoDigestWindow  time.Duration
	infoDigestMu      sync.Mutex
	infoEmailDigest   *alertInfoEmailDigest
}

type AlertDeliveryHistoryStore interface {
	RecordDeliveryAttempts(ctx context.Context, event AlertEvent, results []AlertDeliveryResult) error
}

type AlertNotificationStateStore interface {
	RecordNotification(ctx context.Context, event AlertEvent, window time.Duration) (bool, error)
}

func NewAlertDeliveryDispatcher(options AlertDeliveryDispatcherOptions) *AlertDeliveryDispatcher {
	dispatcher := &AlertDeliveryDispatcher{
		policy:            normalizeDeliveryPolicy(options.Policy),
		routingRules:      options.RoutingRules,
		sinks:             make(map[AlertDeliveryChannel][]AlertDeliverySink),
		sinkResolver:      options.SinkResolver,
		historyStore:      options.HistoryStore,
		notificationStore: options.NotificationStore,
		notifyWindows:     normalizeAlertNotifyWindows(options.NotifyWindows),
		infoDigestWindow:  normalizeInfoEmailDigestWindow(options.InfoDigestWindow),
	}
	for _, sink := range options.Sinks {
		if sink == nil {
			continue
		}
		channel := sink.Channel()
		if channel == "" {
			continue
		}
		dispatcher.sinks[channel] = append(dispatcher.sinks[channel], sink)
	}
	return dispatcher
}

type AlertDeliveryResult struct {
	Channel      AlertDeliveryChannel
	ProviderID   string
	ProviderKind AlertProviderKind
	Delivered    bool
	Err          error
}

type alertInfoEmailDigest struct {
	StartedAt time.Time
	Events    []AlertEvent
}

func (d *AlertDeliveryDispatcher) Deliver(ctx context.Context, event AlertEvent) []AlertDeliveryResult {
	if d == nil {
		return nil
	}
	results := d.FlushInfoEmailDigests(ctx, eventTime(event))
	if !d.shouldNotify(ctx, event) {
		return results
	}

	channels := d.channelsForSeverity(ctx, event.Severity)
	for _, channel := range channels {
		if d.shouldQueueInfoEmailDigest(event, channel) {
			d.queueInfoEmailDigest(event)
			continue
		}
		results = append(results, d.deliverToChannel(ctx, channel, event)...)
	}
	return results
}

func (d *AlertDeliveryDispatcher) FlushInfoEmailDigests(ctx context.Context, now time.Time) []AlertDeliveryResult {
	if d == nil || d.infoDigestWindow <= 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	d.infoDigestMu.Lock()
	digest := d.infoEmailDigest
	if digest == nil || len(digest.Events) == 0 || now.Sub(digest.StartedAt) < d.infoDigestWindow {
		d.infoDigestMu.Unlock()
		return nil
	}
	d.infoEmailDigest = nil
	d.infoDigestMu.Unlock()

	event := infoEmailDigestEvent(*digest, now)
	return d.deliverToChannel(ctx, AlertDeliveryChannelEmail, event)
}

func (d *AlertDeliveryDispatcher) deliverToChannel(ctx context.Context, channel AlertDeliveryChannel, event AlertEvent) []AlertDeliveryResult {
	sinks, err := d.sinksForChannel(ctx, channel)
	if err != nil {
		return []AlertDeliveryResult{{Channel: channel, Err: err}}
	}
	if len(sinks) == 0 {
		return []AlertDeliveryResult{{
			Channel: channel,
			Err:     fmt.Errorf("%w: %s", ErrAlertDeliverySinkMissing, channel),
		}}
	}
	results := make([]AlertDeliveryResult, 0, len(sinks))
	for _, sink := range sinks {
		result := newAlertDeliveryResult(channel, sink)
		if err := sink.Deliver(ctx, event); err != nil {
			result.Err = err
		} else {
			result.Delivered = true
		}
		results = append(results, result)
	}
	if d.historyStore != nil && len(results) > 0 {
		_ = d.historyStore.RecordDeliveryAttempts(ctx, event, results)
	}
	return results
}

func (d *AlertDeliveryDispatcher) sinksForChannel(ctx context.Context, channel AlertDeliveryChannel) ([]AlertDeliverySink, error) {
	sinks := append([]AlertDeliverySink{}, d.sinks[channel]...)
	if d.sinkResolver == nil {
		return sinks, nil
	}
	resolved, err := d.sinkResolver.SinksForChannel(ctx, channel)
	if err != nil {
		return sinks, err
	}
	for _, sink := range resolved {
		if sink != nil {
			sinks = append(sinks, sink)
		}
	}
	return sinks, nil
}

func (d *AlertDeliveryDispatcher) shouldQueueInfoEmailDigest(event AlertEvent, channel AlertDeliveryChannel) bool {
	return d != nil && d.infoDigestWindow > 0 && event.Severity == AlertSeverityInfo && channel == AlertDeliveryChannelEmail
}

func (d *AlertDeliveryDispatcher) queueInfoEmailDigest(event AlertEvent) {
	if d == nil || d.infoDigestWindow <= 0 {
		return
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	d.infoDigestMu.Lock()
	defer d.infoDigestMu.Unlock()

	if d.infoEmailDigest == nil {
		d.infoEmailDigest = &alertInfoEmailDigest{StartedAt: event.OccurredAt}
	}
	d.infoEmailDigest.Events = append(d.infoEmailDigest.Events, cloneAlertEvent(event))
}

func newAlertDeliveryResult(channel AlertDeliveryChannel, sink AlertDeliverySink) AlertDeliveryResult {
	result := AlertDeliveryResult{Channel: channel}
	if metadata, ok := sink.(AlertProviderDeliveryMetadata); ok {
		result.ProviderID = strings.TrimSpace(metadata.ProviderID())
		result.ProviderKind = metadata.ProviderKind()
	}
	return result
}

func (d *AlertDeliveryDispatcher) channelsForSeverity(ctx context.Context, severity AlertSeverity) []AlertDeliveryChannel {
	if d.routingRules != nil {
		rules, err := d.routingRules.GetRoutingRules(ctx)
		if err == nil {
			policy := DeliveryPolicy{
				Routes:                  map[AlertSeverity][]AlertDeliveryChannel(rules),
				UnknownSeverityFallback: d.policy.UnknownSeverityFallback,
			}
			return policy.ChannelsForSeverity(severity)
		}
	}
	return d.policy.ChannelsForSeverity(severity)
}

func (d *AlertDeliveryDispatcher) shouldNotify(ctx context.Context, event AlertEvent) bool {
	if d == nil || d.notificationStore == nil {
		return true
	}
	window := d.notifyWindows[event.Severity]
	allowed, err := d.notificationStore.RecordNotification(ctx, event, window)
	return err == nil && allowed
}

func (d *AlertDeliveryDispatcher) Notify(ctx context.Context, event AlertEvent) error {
	var err error
	for _, result := range d.Deliver(ctx, event) {
		err = errors.Join(err, result.Err)
	}
	return err
}

func normalizeDeliveryPolicy(policy DeliveryPolicy) DeliveryPolicy {
	defaultPolicy := DefaultAlertDeliveryPolicy()
	if policy.Routes == nil {
		policy.Routes = defaultPolicy.Routes
	}
	if policy.UnknownSeverityFallback == "" {
		policy.UnknownSeverityFallback = defaultPolicy.UnknownSeverityFallback
	}
	if _, ok := policy.Routes[policy.UnknownSeverityFallback]; !ok {
		policy.UnknownSeverityFallback = defaultPolicy.UnknownSeverityFallback
	}
	return policy
}

func copyDeliveryChannels(channels []AlertDeliveryChannel) []AlertDeliveryChannel {
	if channels == nil {
		return []AlertDeliveryChannel{}
	}
	copied := make([]AlertDeliveryChannel, len(channels))
	copy(copied, channels)
	return copied
}

func alertDeliveryResultError(result AlertDeliveryResult) string {
	if result.Err == nil {
		return ""
	}
	return result.Err.Error()
}

func normalizeInfoEmailDigestWindow(window time.Duration) time.Duration {
	if window < 0 {
		return 0
	}
	if window == 0 {
		return time.Hour
	}
	return window
}

func infoEmailDigestEvent(digest alertInfoEmailDigest, now time.Time) AlertEvent {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return AlertEvent{
		Key:        "info-email-digest:" + digest.StartedAt.UTC().Format("2006010215"),
		Severity:   AlertSeverityInfo,
		Title:      fmt.Sprintf("Info alert digest (%d events)", len(digest.Events)),
		Message:    infoEmailDigestMessage(digest),
		Component:  "observability",
		OccurredAt: now.UTC(),
		Fields: map[string]any{
			"digest_started_at":  digest.StartedAt.UTC().Format(time.RFC3339Nano),
			"digest_event_count": len(digest.Events),
		},
	}
}

func infoEmailDigestMessage(digest alertInfoEmailDigest) string {
	lines := []string{
		"Hourly info alert digest",
		fmt.Sprintf("Window started at %s", digest.StartedAt.UTC().Format(time.RFC3339Nano)),
		"",
	}
	for _, event := range digest.Events {
		title := strings.TrimSpace(event.Title)
		if title == "" {
			title = alertKey(event)
		}
		if title == "" {
			title = "Info alert"
		}
		line := "- "
		if !event.OccurredAt.IsZero() {
			line += event.OccurredAt.UTC().Format(time.RFC3339) + " "
		}
		if component := strings.TrimSpace(event.Component); component != "" {
			line += "[" + component + "] "
		}
		line += title
		if message := strings.TrimSpace(event.Message); message != "" {
			line += ": " + message
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func eventTime(event AlertEvent) time.Time {
	if event.OccurredAt.IsZero() {
		return time.Now().UTC()
	}
	return event.OccurredAt
}

func cloneAlertEvent(event AlertEvent) AlertEvent {
	if event.Fields != nil {
		fields := make(map[string]any, len(event.Fields))
		for key, value := range event.Fields {
			fields[key] = value
		}
		event.Fields = fields
	}
	return event
}

func makeAlertDeliveryAttemptID(alertKey string, occurredAt time.Time, channel AlertDeliveryChannel, sequence int) string {
	key := strings.TrimSpace(alertKey)
	if key == "" {
		key = "alert"
	}
	deliveryChannel := strings.TrimSpace(string(channel))
	if deliveryChannel == "" {
		deliveryChannel = "unknown"
	}
	return fmt.Sprintf("%s:%d:%s:%d", key, occurredAt.UTC().UnixNano(), deliveryChannel, sequence)
}
