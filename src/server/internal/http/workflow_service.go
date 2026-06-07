package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/workflow"
)

func newConfiguredWorkflowService(cfg config.Config, database *sql.DB, notifier ...workflowFailurePauseNotifier) *workflow.Service {
	var failureNotifier workflowFailurePauseNotifier
	if len(notifier) > 0 {
		failureNotifier = notifier[0]
	}
	return newConfiguredWorkflowServiceWithStoreAndNotifier(cfg, workflow.NewSQLStore(database), failureNotifier)
}

func newConfiguredWorkflowServiceWithStore(cfg config.Config, store workflow.Store) *workflow.Service {
	return newConfiguredWorkflowServiceWithStoreAndNotifier(cfg, store, nil)
}

func newConfiguredWorkflowServiceWithStoreAndNotifier(cfg config.Config, store workflow.Store, notifier workflowFailurePauseNotifier) *workflow.Service {
	return newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, store, notifier, nil)
}

func configuredWorkflowServiceOptions(cfg config.Config, notifier workflowFailurePauseNotifier) []workflow.ServiceOption {
	return configuredWorkflowServiceOptionsWithAlerts(cfg, notifier, nil)
}

func newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg config.Config, store workflow.Store, notifier workflowFailurePauseNotifier, alertSink observability.AlertSink) *workflow.Service {
	return workflow.NewService(store, configuredWorkflowServiceOptionsWithAlerts(cfg, notifier, alertSink)...)
}

func configuredWorkflowServiceOptionsWithAlerts(cfg config.Config, notifier workflowFailurePauseNotifier, alertSink observability.AlertSink) []workflow.ServiceOption {
	options := []workflow.ServiceOption{}
	if cfg.WorkflowSystemMaxConcurrent > 0 || cfg.WorkflowGlobalMaxExecutionsPerMinute > 0 {
		options = append(options, workflow.WithSystemWorkflowLimits(workflow.SystemWorkflowLimits{
			MaxConcurrentWorkflows: cfg.WorkflowSystemMaxConcurrent,
			MaxExecutionsPerMinute: cfg.WorkflowGlobalMaxExecutionsPerMinute,
		}))
	}
	if cfg.RelayEnabled {
		options = append(options, workflow.WithSemanticTriggerMatcher(
			workflow.NewEmbeddingSemanticTriggerMatcher(
				memory.NewRelayEmbedder(workflowRelayBaseURL(cfg), "text-embedding-3-small"),
			),
		))
	}
	if notifier != nil || alertSink != nil {
		options = append(options, workflow.WithFailurePauseNotificationSink(workflowFailurePauseNotificationAdapter{notifier: notifier, alertSink: alertSink}))
	}
	return options
}

func workflowRelayBaseURL(cfg config.Config) string {
	if baseURL := strings.TrimSpace(cfg.WorkflowRelayBaseURL); baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	return "http://localhost:" + fmt.Sprintf("%d", cfg.Port) + "/v1"
}

type workflowFailurePauseNotifier interface {
	CreateEvent(ctx context.Context, event notification.NotificationEvent) (*notification.Notification, error)
}

type workflowFailurePauseNotificationAdapter struct {
	notifier  workflowFailurePauseNotifier
	alertSink observability.AlertSink
}

func (a workflowFailurePauseNotificationAdapter) NotifyWorkflowFailurePaused(ctx context.Context, event workflow.WorkflowFailurePauseNotification) error {
	title := strings.TrimSpace(event.WorkflowName)
	if title == "" {
		title = strings.TrimSpace(event.WorkflowID)
	}
	if title == "" {
		title = "Workflow"
	}
	message := strings.TrimSpace(event.Message)
	if message == "" {
		message = "node execution failed"
	}
	notificationTitle := "Workflow paused: " + title
	notificationMessage := "Node " + event.NodeID + " failed: " + message

	var err error
	if a.notifier != nil {
		_, err = a.notifier.CreateEvent(ctx, notification.NotificationEvent{
			UserID:    event.UserID,
			Type:      "warning",
			Category:  "workflow",
			Title:     notificationTitle,
			Message:   notificationMessage,
			ActionURL: event.ActionURL,
			Metadata:  event.Metadata,
		})
	}
	if a.alertSink != nil {
		err = errors.Join(err, a.alertSink.Notify(ctx, workflowFailurePauseAlertEvent(event, notificationTitle, notificationMessage)))
	}
	return err
}

func workflowFailurePauseAlertEvent(event workflow.WorkflowFailurePauseNotification, title string, message string) observability.AlertEvent {
	fields := map[string]any{}
	for key, value := range event.Metadata {
		fields[key] = value
	}
	if strings.TrimSpace(event.ActionURL) != "" {
		fields["actionUrl"] = strings.TrimSpace(event.ActionURL)
	}
	if strings.TrimSpace(event.UserID) != "" {
		fields["userId"] = strings.TrimSpace(event.UserID)
	}
	if strings.TrimSpace(event.OrganizationID) != "" {
		fields["organizationId"] = strings.TrimSpace(event.OrganizationID)
	}
	if strings.TrimSpace(event.WorkflowID) != "" {
		fields["workflowId"] = strings.TrimSpace(event.WorkflowID)
	}
	if strings.TrimSpace(event.ExecutionID) != "" {
		fields["executionId"] = strings.TrimSpace(event.ExecutionID)
	}
	if strings.TrimSpace(event.NodeID) != "" {
		fields["nodeId"] = strings.TrimSpace(event.NodeID)
	}
	keyParts := []string{"workflow", "failure_paused", strings.TrimSpace(event.ExecutionID), strings.TrimSpace(event.NodeID)}
	return observability.AlertEvent{
		Key:       strings.Join(keyParts, ":"),
		Severity:  observability.AlertSeverityWarning,
		Title:     strings.TrimSpace(title),
		Message:   strings.TrimSpace(message),
		Component: "workflow",
		Fields:    fields,
	}
}
