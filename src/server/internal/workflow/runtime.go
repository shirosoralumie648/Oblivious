package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNodeExecutionNotFound = errors.New("workflow node execution not found")

func ApplyNodeFailure(execution Execution, node Node, failure NodeFailure) (Execution, FailureDecision, error) {
	nodeID := strings.TrimSpace(node.ID)
	if nodeID == "" {
		return Execution{}, FailureDecision{}, ErrNodeExecutionNotFound
	}
	now := failure.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	next := cloneExecution(execution)
	next.UpdatedAt = now
	nodeIndex := findNodeExecution(next.NodeExecutions, nodeID)
	if nodeIndex == -1 {
		return Execution{}, FailureDecision{}, ErrNodeExecutionNotFound
	}

	policy := normalizeFailurePolicy(node.FailurePolicy)
	message := strings.TrimSpace(failure.Message)
	if message == "" {
		message = "node execution failed"
	}

	switch policy.Strategy {
	case FailureStrategyAutoRetry:
		next = applyAutoRetryFailure(next, nodeIndex, policy, message, now)
		if next.NodeExecutions[nodeIndex].Status == NodeStatusFailed {
			return next, FailureDecision{Action: FailureActionFail}, nil
		}
		return next, FailureDecision{Action: FailureActionRetry, RetryAt: next.NodeExecutions[nodeIndex].RetryAt}, nil
	case FailureStrategySkipOnFailure:
		next.NodeExecutions[nodeIndex].Status = NodeStatusSkipped
		next.NodeExecutions[nodeIndex].Error = message
		next.NodeExecutions[nodeIndex].FinishedAt = &now
		next.Status = ExecutionStatusPartialSuccess
		if next.Variables == nil {
			next.Variables = map[string]any{}
		}
		next.Variables["nodes."+nodeID+".output"] = nil
		return next, FailureDecision{Action: FailureActionContinue}, nil
	case FailureStrategyFailureBranch:
		next.NodeExecutions[nodeIndex].Status = NodeStatusFailed
		next.NodeExecutions[nodeIndex].Error = message
		next.NodeExecutions[nodeIndex].FinishedAt = &now
		if next.Variables == nil {
			next.Variables = map[string]any{}
		}
		next.Variables["workflow.error"] = map[string]any{
			"node_id": nodeID,
			"message": message,
		}
		return next, FailureDecision{Action: FailureActionBranch, NextNodeID: policy.FailureBranchNodeID}, nil
	default:
		next.NodeExecutions[nodeIndex].Status = NodeStatusFailed
		next.NodeExecutions[nodeIndex].Error = message
		next.NodeExecutions[nodeIndex].FinishedAt = &now
		next.Status = ExecutionStatusPaused
		next.PauseReason = fmt.Sprintf("Node %s failed: %s", nodeID, message)
		return next, FailureDecision{Action: FailureActionPause}, nil
	}
}

func applyAutoRetryFailure(execution Execution, nodeIndex int, policy FailurePolicy, message string, now time.Time) Execution {
	attempt := execution.NodeExecutions[nodeIndex].Attempt
	if attempt <= 0 {
		attempt = 1
	}
	if attempt > policy.MaxRetries {
		execution.NodeExecutions[nodeIndex].Status = NodeStatusFailed
		execution.NodeExecutions[nodeIndex].Error = message
		execution.NodeExecutions[nodeIndex].FinishedAt = &now
		execution.Status = ExecutionStatusFailed
		execution.FinishedAt = &now
		return execution
	}

	retryAt := now.Add(retryDelayForAttempt(policy, attempt))
	execution.NodeExecutions[nodeIndex].Status = NodeStatusRetrying
	execution.NodeExecutions[nodeIndex].Attempt = attempt + 1
	execution.NodeExecutions[nodeIndex].Error = message
	execution.NodeExecutions[nodeIndex].RetryAt = &retryAt
	execution.NodeExecutions[nodeIndex].FinishedAt = nil
	execution.Status = ExecutionStatusRunning
	return execution
}

func normalizeFailurePolicy(policy FailurePolicy) FailurePolicy {
	if policy.Strategy == "" {
		policy.Strategy = FailureStrategyPauseOnFailure
	}
	if policy.Strategy == FailureStrategyAutoRetry && policy.MaxRetries <= 0 {
		policy.MaxRetries = 3
	}
	return policy
}

func retryDelayForAttempt(policy FailurePolicy, attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	index := attempt - 1
	if index >= 0 && index < len(policy.RetryDelays) && policy.RetryDelays[index] > 0 {
		return policy.RetryDelays[index]
	}
	delay := time.Second
	for i := 1; i < attempt; i++ {
		delay *= 3
	}
	return delay
}

func findNodeExecution(nodes []NodeExecution, nodeID string) int {
	for index, node := range nodes {
		if node.NodeID == nodeID {
			return index
		}
	}
	return -1
}

func cloneExecution(execution Execution) Execution {
	cloned := execution
	if execution.Variables != nil {
		cloned.Variables = make(map[string]any, len(execution.Variables))
		for key, value := range execution.Variables {
			cloned.Variables[key] = value
		}
	}
	if execution.NodeExecutions != nil {
		cloned.NodeExecutions = append([]NodeExecution(nil), execution.NodeExecutions...)
	}
	return cloned
}
