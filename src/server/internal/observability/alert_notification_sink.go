package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"oblivious/server/internal/notification"
)

var ErrAlertInAppRecipientsMissing = errors.New("alert in-app notification recipients missing")

type AlertInAppNotifier interface {
	CreateEvent(ctx context.Context, event notification.NotificationEvent) (*notification.Notification, error)
}

type AlertInAppDeliverySinkOptions struct {
	Notifier         AlertInAppNotifier
	RecipientUserIDs []string
	ActionURL        string
}

type AlertInAppDeliverySink struct {
	notifier         AlertInAppNotifier
	recipientUserIDs []string
	actionURL        string
}

func NewInAppAlertDeliverySink(options AlertInAppDeliverySinkOptions) *AlertInAppDeliverySink {
	return &AlertInAppDeliverySink{
		notifier:         options.Notifier,
		recipientUserIDs: normalizeAlertRecipientUserIDs(options.RecipientUserIDs),
		actionURL:        strings.TrimSpace(options.ActionURL),
	}
}

func (s *AlertInAppDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelInApp
}

func (s *AlertInAppDeliverySink) Deliver(ctx context.Context, event AlertEvent) error {
	if s == nil || s.notifier == nil {
		return ErrAlertDeliverySinkMissing
	}
	if len(s.recipientUserIDs) == 0 {
		return ErrAlertInAppRecipientsMissing
	}

	notificationEvent := alertNotificationEvent(event, s.actionURL)
	var err error
	for _, userID := range s.recipientUserIDs {
		notificationEvent.UserID = userID
		if _, createErr := s.notifier.CreateEvent(ctx, notificationEvent); createErr != nil {
			err = errors.Join(err, fmt.Errorf("deliver in-app alert to %s: %w", userID, createErr))
		}
	}
	return err
}

func alertNotificationEvent(event AlertEvent, actionURL string) notification.NotificationEvent {
	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = alertKey(event)
	}
	if title == "" {
		title = "Operational alert"
	}
	message := strings.TrimSpace(event.Message)
	if message == "" {
		message = "An operational alert requires attention."
	}

	return notification.NotificationEvent{
		Type:      notificationTypeForAlertSeverity(event.Severity),
		Category:  "system",
		Title:     title,
		Message:   message,
		ActionURL: actionURL,
		Metadata:  alertNotificationMetadata(event),
	}
}

func notificationTypeForAlertSeverity(severity AlertSeverity) string {
	switch severity {
	case AlertSeverityCritical:
		return "error"
	case AlertSeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

func alertNotificationMetadata(event AlertEvent) map[string]any {
	metadata := map[string]any{
		"event":    "observability.alert.fired",
		"severity": string(event.Severity),
	}
	if key := alertKey(event); key != "" {
		metadata["alertKey"] = key
	}
	if strings.TrimSpace(event.Component) != "" {
		metadata["component"] = event.Component
	}
	if event.Escalated {
		metadata["escalated"] = true
	}
	if event.OriginalSeverity != "" {
		metadata["originalSeverity"] = string(event.OriginalSeverity)
	}
	if !event.OccurredAt.IsZero() {
		metadata["occurredAt"] = event.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return metadata
}

func normalizeAlertRecipientUserIDs(userIDs []string) []string {
	seen := make(map[string]struct{}, len(userIDs))
	normalized := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		normalized = append(normalized, userID)
	}
	return normalized
}
