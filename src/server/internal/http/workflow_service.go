package http

import (
	"database/sql"

	"oblivious/server/internal/config"
	"oblivious/server/internal/workflow"
)

func newConfiguredWorkflowService(cfg config.Config, database *sql.DB) *workflow.Service {
	options := []workflow.ServiceOption{}
	if cfg.WorkflowSystemMaxConcurrent > 0 || cfg.WorkflowGlobalMaxExecutionsPerMinute > 0 {
		options = append(options, workflow.WithSystemWorkflowLimits(workflow.SystemWorkflowLimits{
			MaxConcurrentWorkflows: cfg.WorkflowSystemMaxConcurrent,
			MaxExecutionsPerMinute: cfg.WorkflowGlobalMaxExecutionsPerMinute,
		}))
	}
	return workflow.NewService(workflow.NewSQLStore(database), options...)
}
