package observability

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type AlertSeverity string

const (
	AlertSeverityDebug    AlertSeverity = "debug"
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

type AlertEvent struct {
	Key              string
	Severity         AlertSeverity
	OriginalSeverity AlertSeverity
	Escalated        bool
	Title            string
	Message          string
	Component        string
	OccurredAt       time.Time
	Fields           map[string]any
}

type AlertSink interface {
	Notify(ctx context.Context, event AlertEvent) error
}

type AlertRouterOptions struct {
	LogSink       AlertSink
	NotifySink    AlertSink
	Escalator     *AlertEscalator
	StateStore    AlertStateStore
	Now           func() time.Time
	NotifyWindows map[AlertSeverity]time.Duration
}

type AlertRouter struct {
	mu            sync.Mutex
	logSink       AlertSink
	notifySink    AlertSink
	escalator     *AlertEscalator
	stateStore    AlertStateStore
	now           func() time.Time
	notifyWindows map[AlertSeverity]time.Duration
	lastNotified  map[string]time.Time
}

func NewAlertRouter(options AlertRouterOptions) *AlertRouter {
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	escalator := options.Escalator
	if escalator == nil {
		escalator = NewAlertEscalator(AlertEscalatorOptions{})
	}
	return &AlertRouter{
		logSink:       options.LogSink,
		notifySink:    options.NotifySink,
		escalator:     escalator,
		stateStore:    options.StateStore,
		now:           now,
		notifyWindows: normalizeAlertNotifyWindows(options.NotifyWindows),
		lastNotified:  make(map[string]time.Time),
	}
}

func (r *AlertRouter) Route(ctx context.Context, event AlertEvent) error {
	if event.Severity == "" {
		event.Severity = AlertSeverityInfo
	}
	if event.OccurredAt.IsZero() {
		if r != nil && r.now != nil {
			event.OccurredAt = r.now()
		} else {
			event.OccurredAt = time.Now().UTC()
		}
	}
	if r != nil && r.stateStore != nil {
		state, err := r.stateStore.RecordAlertOpen(ctx, event)
		if err != nil {
			return err
		}
		event.Severity = state.Severity
		event.OriginalSeverity = state.OriginalSeverity
		event.Escalated = state.Escalated
	} else if r != nil && r.escalator != nil {
		event = r.escalator.Apply(event)
	}

	var err error
	if r != nil && r.logSink != nil {
		err = errors.Join(err, r.logSink.Notify(ctx, event))
	}
	if event.Severity != AlertSeverityDebug && r != nil && r.notifySink != nil && r.shouldNotify(ctx, event) {
		err = errors.Join(err, r.notifySink.Notify(ctx, event))
	}
	return err
}

func (r *AlertRouter) Notify(ctx context.Context, event AlertEvent) error {
	return r.Route(ctx, event)
}

func normalizeAlertNotifyWindows(overrides map[AlertSeverity]time.Duration) map[AlertSeverity]time.Duration {
	windows := map[AlertSeverity]time.Duration{
		AlertSeverityInfo:     time.Hour,
		AlertSeverityWarning:  15 * time.Minute,
		AlertSeverityCritical: 0,
	}
	for severity, window := range overrides {
		windows[severity] = window
	}
	return windows
}

func (r *AlertRouter) shouldNotify(ctx context.Context, event AlertEvent) bool {
	if r == nil {
		return false
	}
	window := r.notifyWindows[event.Severity]
	if window <= 0 {
		if r.stateStore != nil {
			_, _ = r.stateStore.RecordNotification(ctx, event, window)
		}
		return true
	}
	key := alertKey(event)
	if key == "" {
		return true
	}
	if r.stateStore != nil {
		allowed, err := r.stateStore.RecordNotification(ctx, event, window)
		return err == nil && allowed
	}
	notifyKey := string(event.Severity) + ":" + key

	r.mu.Lock()
	defer r.mu.Unlock()

	last, ok := r.lastNotified[notifyKey]
	if ok && event.OccurredAt.Sub(last) < window {
		return false
	}
	r.lastNotified[notifyKey] = event.OccurredAt
	return true
}

type AlertEscalatorOptions struct {
	Window    time.Duration
	Threshold int
}

type AlertEscalator struct {
	mu        sync.Mutex
	window    time.Duration
	threshold int
	seen      map[string][]time.Time
}

func NewAlertEscalator(options AlertEscalatorOptions) *AlertEscalator {
	window := options.Window
	if window <= 0 {
		window = 5 * time.Minute
	}
	threshold := options.Threshold
	if threshold <= 0 {
		threshold = 3
	}
	return &AlertEscalator{
		window:    window,
		threshold: threshold,
		seen:      make(map[string][]time.Time),
	}
}

func (e *AlertEscalator) Apply(event AlertEvent) AlertEvent {
	if e == nil {
		return event
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	key := alertKey(event)
	if key == "" {
		return event
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := event.OccurredAt.Add(-e.window)
	timestamps := e.seen[key]
	kept := timestamps[:0]
	for _, seenAt := range timestamps {
		if !seenAt.Before(cutoff) {
			kept = append(kept, seenAt)
		}
	}
	kept = append(kept, event.OccurredAt)
	e.seen[key] = kept

	if len(kept) < e.threshold {
		return event
	}

	escalated := escalateSeverity(event.Severity)
	if escalated == event.Severity {
		return event
	}
	event.OriginalSeverity = event.Severity
	event.Severity = escalated
	event.Escalated = true
	return event
}

func alertKey(event AlertEvent) string {
	for _, candidate := range []string{event.Key, event.Component + ":" + event.Title, event.Title} {
		if strings.TrimSpace(candidate) != "" && candidate != ":" {
			return candidate
		}
	}
	return ""
}

func escalateSeverity(severity AlertSeverity) AlertSeverity {
	switch severity {
	case AlertSeverityDebug:
		return AlertSeverityInfo
	case AlertSeverityInfo:
		return AlertSeverityWarning
	case AlertSeverityWarning:
		return AlertSeverityCritical
	default:
		return severity
	}
}
