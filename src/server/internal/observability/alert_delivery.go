package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
}

type AlertDeliveryDispatcher struct {
	policy            DeliveryPolicy
	routingRules      AlertRoutingRuleStore
	sinks             map[AlertDeliveryChannel][]AlertDeliverySink
	sinkResolver      AlertDeliverySinkResolver
	historyStore      AlertDeliveryHistoryStore
	notificationStore AlertNotificationStateStore
	notifyWindows     map[AlertSeverity]time.Duration
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

func (d *AlertDeliveryDispatcher) Deliver(ctx context.Context, event AlertEvent) []AlertDeliveryResult {
	if d == nil {
		return nil
	}
	if !d.shouldNotify(ctx, event) {
		return []AlertDeliveryResult{}
	}

	channels := d.channelsForSeverity(ctx, event.Severity)
	results := make([]AlertDeliveryResult, 0, len(channels))
	for _, channel := range channels {
		sinks, err := d.sinksForChannel(ctx, channel)
		if err != nil {
			results = append(results, AlertDeliveryResult{Channel: channel, Err: err})
			continue
		}
		if len(sinks) == 0 {
			results = append(results, AlertDeliveryResult{
				Channel: channel,
				Err:     fmt.Errorf("%w: %s", ErrAlertDeliverySinkMissing, channel),
			})
			continue
		}
		for _, sink := range sinks {
			result := newAlertDeliveryResult(channel, sink)
			if err := sink.Deliver(ctx, event); err != nil {
				result.Err = err
			} else {
				result.Delivered = true
			}
			results = append(results, result)
		}
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
