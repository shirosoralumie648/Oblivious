package http

import (
	"database/sql"
	"fmt"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/workflow"
)

func newConfiguredWorkflowService(cfg config.Config, database *sql.DB) *workflow.Service {
	return newConfiguredWorkflowServiceWithStore(cfg, workflow.NewSQLStore(database))
}

func newConfiguredWorkflowServiceWithStore(cfg config.Config, store workflow.Store) *workflow.Service {
	return workflow.NewService(store, configuredWorkflowServiceOptions(cfg)...)
}

func configuredWorkflowServiceOptions(cfg config.Config) []workflow.ServiceOption {
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
	return options
}

func workflowRelayBaseURL(cfg config.Config) string {
	if baseURL := strings.TrimSpace(cfg.WorkflowRelayBaseURL); baseURL != "" {
		return strings.TrimRight(baseURL, "/")
	}
	return "http://localhost:" + fmt.Sprintf("%d", cfg.Port) + "/v1"
}
