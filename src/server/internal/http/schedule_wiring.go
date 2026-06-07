package http

import (
	"oblivious/server/internal/agent"
	"oblivious/server/internal/schedule"
	"oblivious/server/internal/workflow"
)

func newScheduleService(store schedule.Store, workflowService *workflow.Service, agentStarter schedule.AgentStarter) *schedule.Service {
	options := []schedule.ServiceOption{}
	if workflowService != nil {
		options = append(options, schedule.WithWorkflowStarter(workflowService))
	}
	if agentStarter != nil {
		options = append(options, schedule.WithAgentStarter(agentStarter))
	}
	service := schedule.NewService(store, options...)
	if workflowService != nil {
		workflowService.SetScheduleSyncer(service)
	}
	return service
}

var _ schedule.AgentStarter = (*agent.Service)(nil)
