package observability

import (
	"context"
	"testing"
	"time"
)

type captureAlertSink struct {
	events []AlertEvent
}

func (s *captureAlertSink) Notify(_ context.Context, event AlertEvent) error {
	s.events = append(s.events, event)
	return nil
}

func TestAlertRouterDoesNotNotifyDebugAlerts(t *testing.T) {
	logSink := &captureAlertSink{}
	notifySink := &captureAlertSink{}
	router := NewAlertRouter(AlertRouterOptions{
		LogSink:    logSink,
		NotifySink: notifySink,
		Now:        func() time.Time { return time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC) },
	})

	if err := router.Route(context.Background(), AlertEvent{
		Key:       "slow-query",
		Severity:  AlertSeverityDebug,
		Title:     "Slow query detected",
		Message:   "query took 1.2s",
		Component: "database",
	}); err != nil {
		t.Fatalf("route debug alert: %v", err)
	}

	if len(logSink.events) != 1 {
		t.Fatalf("expected debug alert to be logged once, got %d log events", len(logSink.events))
	}
	if len(notifySink.events) != 0 {
		t.Fatalf("expected debug alert not to notify, got %d notifications", len(notifySink.events))
	}
}

func TestAlertRouterNotifiesWarningAndCriticalAlerts(t *testing.T) {
	logSink := &captureAlertSink{}
	notifySink := &captureAlertSink{}
	router := NewAlertRouter(AlertRouterOptions{
		LogSink:    logSink,
		NotifySink: notifySink,
		Now:        func() time.Time { return time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC) },
	})

	for _, severity := range []AlertSeverity{AlertSeverityWarning, AlertSeverityCritical} {
		if err := router.Route(context.Background(), AlertEvent{
			Key:      string(severity) + "-alert",
			Severity: severity,
			Title:    "Operational alert",
			Message:  "attention required",
		}); err != nil {
			t.Fatalf("route %s alert: %v", severity, err)
		}
	}

	if len(logSink.events) != 2 {
		t.Fatalf("expected warning and critical alerts to be logged, got %d", len(logSink.events))
	}
	if len(notifySink.events) != 2 {
		t.Fatalf("expected warning and critical alerts to notify, got %d", len(notifySink.events))
	}
	if notifySink.events[0].Severity != AlertSeverityWarning || notifySink.events[1].Severity != AlertSeverityCritical {
		t.Fatalf("unexpected notification severities: %+v", notifySink.events)
	}
}

func TestAlertRouterThrottlesWarningNotificationsButNotCritical(t *testing.T) {
	logSink := &captureAlertSink{}
	notifySink := &captureAlertSink{}
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	router := NewAlertRouter(AlertRouterOptions{
		LogSink:    logSink,
		NotifySink: notifySink,
		Now:        func() time.Time { return now },
	})

	for _, occurredAt := range []time.Time{now, now.Add(5 * time.Minute), now.Add(16 * time.Minute)} {
		if err := router.Route(context.Background(), AlertEvent{
			Key:        "queue-backlog",
			Severity:   AlertSeverityWarning,
			Title:      "Queue backlog",
			Message:    "queue depth is high",
			OccurredAt: occurredAt,
		}); err != nil {
			t.Fatalf("route warning alert at %s: %v", occurredAt, err)
		}
	}

	if len(logSink.events) != 3 {
		t.Fatalf("expected all warning alerts to be logged, got %d", len(logSink.events))
	}
	if len(notifySink.events) != 2 {
		t.Fatalf("expected warning alerts to notify at most once per 15 minutes, got %d", len(notifySink.events))
	}

	for i := 0; i < 2; i++ {
		if err := router.Route(context.Background(), AlertEvent{
			Key:        "database-down",
			Severity:   AlertSeverityCritical,
			Title:      "Database down",
			Message:    "primary database unavailable",
			OccurredAt: now.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("route critical alert: %v", err)
		}
	}

	if len(notifySink.events) != 4 {
		t.Fatalf("expected critical alerts to bypass notification throttle, got %d notifications", len(notifySink.events))
	}
}

func TestAlertEscalatorRaisesSeverityOnThirdRepeatWithinFiveMinutes(t *testing.T) {
	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	escalator := NewAlertEscalator(AlertEscalatorOptions{
		Window:    5 * time.Minute,
		Threshold: 3,
	})

	first := escalator.Apply(AlertEvent{Key: "queue-backlog", Severity: AlertSeverityWarning, OccurredAt: now})
	second := escalator.Apply(AlertEvent{Key: "queue-backlog", Severity: AlertSeverityWarning, OccurredAt: now.Add(2 * time.Minute)})
	third := escalator.Apply(AlertEvent{Key: "queue-backlog", Severity: AlertSeverityWarning, OccurredAt: now.Add(4 * time.Minute)})

	if first.Severity != AlertSeverityWarning || second.Severity != AlertSeverityWarning {
		t.Fatalf("expected first two repeats to stay warning, got %s and %s", first.Severity, second.Severity)
	}
	if third.Severity != AlertSeverityCritical {
		t.Fatalf("expected third repeat within 5 minutes to escalate to critical, got %s", third.Severity)
	}
	if !third.Escalated {
		t.Fatalf("expected third repeat to be marked escalated: %+v", third)
	}
	if third.OriginalSeverity != AlertSeverityWarning {
		t.Fatalf("expected original severity warning, got %s", third.OriginalSeverity)
	}
}
