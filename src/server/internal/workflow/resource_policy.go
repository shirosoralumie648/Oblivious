package workflow

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultWorkflowMaxConcurrentExecutions     = 10
	defaultConversationMaxConcurrentExecutions = 50
	defaultScheduleMaxConcurrentExecutions     = 1
	defaultWebhookMaxConcurrentExecutions      = 10
	defaultWorkflowMaxExecutionDuration        = time.Hour
	defaultWorkflowMaxNodeExecutions           = 1000
	defaultOrgMaxConcurrentWorkflows           = 50
)

type workflowConcurrencyOverflow string

const (
	workflowConcurrencyQueue  workflowConcurrencyOverflow = "queue"
	workflowConcurrencyReject workflowConcurrencyOverflow = "reject"
)

type WorkflowResourceUsage struct {
	Now                time.Time
	TotalTokens        int
	NodeExecutionCount int
}

type workflowResourcePolicy struct {
	MaxConcurrentExecutions      int
	maxConcurrentExecutionsSet   bool
	ConcurrencyOverflow          workflowConcurrencyOverflow
	MaxExecutionDuration         time.Duration
	MaxTokensBudget              int
	MaxNodeExecutions            int
	OrgMaxConcurrentWorkflows    int
	SystemMaxConcurrentWorkflows int
}

func resourcePolicyForWorkflow(workflow *WorkflowDefinition) workflowResourcePolicy {
	policy := workflowResourcePolicy{
		MaxConcurrentExecutions: defaultWorkflowMaxConcurrentExecutions,
		ConcurrencyOverflow:     workflowConcurrencyQueue,
		MaxExecutionDuration:    defaultWorkflowMaxExecutionDuration,
		MaxNodeExecutions:       defaultWorkflowMaxNodeExecutions,
	}
	if workflow == nil {
		return policy
	}
	definition := workflow.Definition
	if definition == nil {
		return policy
	}

	if value, ok := lookupDefinitionValue(definition, "max_concurrent_executions", "maxConcurrentExecutions", "workflow.max_concurrent_executions", "workflow.maxConcurrentExecutions"); ok {
		if limit, ok := positiveInt(value); ok {
			policy.MaxConcurrentExecutions = limit
			policy.maxConcurrentExecutionsSet = true
		}
	}
	if value, ok := lookupDefinitionValue(definition, "concurrency_overflow", "concurrencyOverflow", "workflow.concurrency_overflow", "workflow.concurrencyOverflow"); ok {
		switch strings.ToLower(strings.TrimSpace(stringValue(value))) {
		case string(workflowConcurrencyReject):
			policy.ConcurrencyOverflow = workflowConcurrencyReject
		case string(workflowConcurrencyQueue):
			policy.ConcurrencyOverflow = workflowConcurrencyQueue
		}
	}
	if value, ok := lookupDefinitionValue(definition, "max_execution_duration_seconds", "maxExecutionDurationSeconds", "workflow.max_execution_duration_seconds", "workflow.maxExecutionDurationSeconds"); ok {
		if seconds, ok := positiveInt(value); ok {
			policy.MaxExecutionDuration = time.Duration(seconds) * time.Second
		}
	}
	if value, ok := lookupDefinitionValue(definition, "max_tokens_budget", "maxTokensBudget", "workflow.max_tokens_budget", "workflow.maxTokensBudget"); ok {
		if budget, ok := positiveInt(value); ok {
			policy.MaxTokensBudget = budget
		}
	}
	if value, ok := lookupDefinitionValue(definition, "max_node_executions", "maxNodeExecutions", "workflow.max_node_executions", "workflow.maxNodeExecutions"); ok {
		if limit, ok := positiveInt(value); ok {
			policy.MaxNodeExecutions = limit
		}
	}
	return policy
}

func concurrencyPolicyForTrigger(workflow *WorkflowDefinition, triggerType WorkflowTriggerType) workflowResourcePolicy {
	policy := resourcePolicyForWorkflow(workflow)
	if policy.maxConcurrentExecutionsSet {
		return policy
	}
	switch triggerType {
	case WorkflowTriggerConversation:
		policy.MaxConcurrentExecutions = defaultConversationMaxConcurrentExecutions
	case WorkflowTriggerSchedule:
		policy.MaxConcurrentExecutions = defaultScheduleMaxConcurrentExecutions
	case WorkflowTriggerWebhook:
		policy.MaxConcurrentExecutions = defaultWebhookMaxConcurrentExecutions
	}
	return policy
}

func lookupDefinitionValue(definition map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := definition[key]; ok {
			return value, true
		}
	}
	if workflowValue, ok := nestedDefinitionMap(definition["workflow"]); ok {
		for _, key := range keys {
			trimmed := strings.TrimPrefix(strings.TrimPrefix(key, "workflow."), "workflow_")
			if value, ok := workflowValue[trimmed]; ok {
				return value, true
			}
		}
	}
	if limitsValue, ok := nestedDefinitionMap(definition["limits"]); ok {
		for _, key := range keys {
			if value, ok := limitsValue[key]; ok {
				return value, true
			}
		}
	}
	return nil, false
}

func nestedDefinitionMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func positiveInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, typed > 0
	case int32:
		return int(typed), typed > 0
	case int64:
		return int(typed), typed > 0
	case float64:
		if typed <= 0 {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed > 0 {
			return int(parsed), true
		}
		floatValue, err := typed.Float64()
		if err != nil || floatValue <= 0 {
			return 0, false
		}
		return int(floatValue), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil && parsed > 0
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
