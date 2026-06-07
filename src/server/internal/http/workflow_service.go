package http

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/notification"
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
	return workflow.NewService(store, configuredWorkflowServiceOptions(cfg, notifier)...)
}

func configuredWorkflowServiceOptions(cfg config.Config, notifier workflowFailurePauseNotifier) []workflow.ServiceOption {
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
	if notifier != nil {
		options = append(options, workflow.WithFailurePauseNotificationSink(workflowFailurePauseNotificationAdapter{notifier: notifier}))
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
	notifier workflowFailurePauseNotifier
}

func (a workflowFailurePauseNotificationAdapter) NotifyWorkflowFailurePaused(ctx context.Context, event workflow.WorkflowFailurePauseNotification) error {
	if a.notifier == nil {
		return nil
	}
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
	_, err := a.notifier.CreateEvent(ctx, notification.NotificationEvent{
		UserID:    event.UserID,
		Type:      "warning",
		Category:  "workflow",
		Title:     "Workflow paused: " + title,
		Message:   "Node " + event.NodeID + " failed: " + message,
		ActionURL: event.ActionURL,
		Metadata:  event.Metadata,
	})
	return err
}
