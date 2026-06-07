package workflow

import (
	"strings"
	"testing"
	"time"
)

func TestApplyNodeFailureDefaultsToPauseForUserDecision(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	execution := Execution{
		ID:         "exec_1",
		WorkflowID: "workflow_1",
		Status:     ExecutionStatusRunning,
		NodeExecutions: []NodeExecution{
			{NodeID: "node_1", Status: NodeStatusRunning, StartedAt: &now},
		},
	}

	next, decision, err := ApplyNodeFailure(execution, Node{
		ID: "node_1",
	}, NodeFailure{
		Message: "provider unavailable",
		Now:     now,
	})

	if err != nil {
		t.Fatalf("ApplyNodeFailure returned error: %v", err)
	}
	if decision.Action != FailureActionPause {
		t.Fatalf("expected pause action, got %+v", decision)
	}
	if next.Status != ExecutionStatusPaused {
		t.Fatalf("expected execution paused, got %+v", next.Status)
	}
	if next.NodeExecutions[0].Status != NodeStatusFailed {
		t.Fatalf("expected failed node status, got %+v", next.NodeExecutions[0])
	}
	if !strings.Contains(next.NodeExecutions[0].Error, "provider unavailable") {
		t.Fatalf("expected node error to be recorded, got %+v", next.NodeExecutions[0])
	}
	if next.PauseReason == "" {
		t.Fatal("expected pause reason")
	}
}

func TestApplyNodeFailureSkipsOptionalNodeAndContinues(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	execution := Execution{
		ID:         "exec_1",
		WorkflowID: "workflow_1",
		Status:     ExecutionStatusRunning,
		NodeExecutions: []NodeExecution{
			{NodeID: "optional_lookup", Status: NodeStatusRunning, StartedAt: &now},
		},
	}

	next, decision, err := ApplyNodeFailure(execution, Node{
		ID: "optional_lookup",
		FailurePolicy: FailurePolicy{
			Strategy: FailureStrategySkipOnFailure,
		},
	}, NodeFailure{
		Message: "knowledge source timed out",
		Now:     now,
	})

	if err != nil {
		t.Fatalf("ApplyNodeFailure returned error: %v", err)
	}
	if decision.Action != FailureActionContinue {
		t.Fatalf("expected continue action, got %+v", decision)
	}
	if next.Status != ExecutionStatusPartialSuccess {
		t.Fatalf("expected partial success execution, got %s", next.Status)
	}
	if next.NodeExecutions[0].Status != NodeStatusSkipped {
		t.Fatalf("expected skipped node, got %+v", next.NodeExecutions[0])
	}
	if got := next.Variables["nodes.optional_lookup.output"]; got != nil {
		t.Fatalf("skipped node output should be nil, got %#v", got)
	}
}

func TestApplyNodeFailureSchedulesAutoRetryUntilBudgetExhausted(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	execution := Execution{
		ID:         "exec_1",
		WorkflowID: "workflow_1",
		Status:     ExecutionStatusRunning,
		NodeExecutions: []NodeExecution{
			{NodeID: "http_call", Status: NodeStatusRunning, Attempt: 1, StartedAt: &now},
		},
	}

	next, decision, err := ApplyNodeFailure(execution, Node{
		ID: "http_call",
		FailurePolicy: FailurePolicy{
			Strategy:    FailureStrategyAutoRetry,
			MaxRetries:  3,
			RetryDelays: []time.Duration{time.Second, 3 * time.Second, 9 * time.Second},
		},
	}, NodeFailure{
		Message: "503",
		Now:     now,
	})

	if err != nil {
		t.Fatalf("ApplyNodeFailure returned error: %v", err)
	}
	if decision.Action != FailureActionRetry || decision.RetryAt == nil || !decision.RetryAt.Equal(now.Add(time.Second)) {
		t.Fatalf("expected retry at +1s, got %+v", decision)
	}
	if next.Status != ExecutionStatusRunning {
		t.Fatalf("expected execution to remain running, got %s", next.Status)
	}
	if next.NodeExecutions[0].Status != NodeStatusRetrying || next.NodeExecutions[0].Attempt != 2 {
		t.Fatalf("expected retrying attempt 2, got %+v", next.NodeExecutions[0])
	}

	next.NodeExecutions[0].Status = NodeStatusRunning
	next.NodeExecutions[0].Attempt = 4
	failed, exhausted, err := ApplyNodeFailure(next, Node{
		ID: "http_call",
		FailurePolicy: FailurePolicy{
			Strategy:   FailureStrategyAutoRetry,
			MaxRetries: 3,
		},
	}, NodeFailure{
		Message: "still failing",
		Now:     now,
	})
	if err != nil {
		t.Fatalf("ApplyNodeFailure returned error after exhausted retries: %v", err)
	}
	if exhausted.Action != FailureActionFail {
		t.Fatalf("expected fail action after exhausted retries, got %+v", exhausted)
	}
	if failed.Status != ExecutionStatusFailed || failed.NodeExecutions[0].Status != NodeStatusFailed {
		t.Fatalf("expected failed execution/node, got execution=%s node=%+v", failed.Status, failed.NodeExecutions[0])
	}
}

func TestApplyNodeFailureUsesExponentialDefaultRetryDelays(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	execution := Execution{
		ID:         "exec_1",
		WorkflowID: "workflow_1",
		Status:     ExecutionStatusRunning,
		NodeExecutions: []NodeExecution{
			{NodeID: "http_call", Status: NodeStatusRunning, Attempt: 1, StartedAt: &now},
		},
	}
	node := Node{
		ID: "http_call",
		FailurePolicy: FailurePolicy{
			Strategy:   FailureStrategyAutoRetry,
			MaxRetries: 3,
		},
	}
	wantDelays := []time.Duration{time.Second, 3 * time.Second, 9 * time.Second}

	for index, wantDelay := range wantDelays {
		execution.NodeExecutions[0].Attempt = index + 1
		execution.NodeExecutions[0].Status = NodeStatusRunning

		next, decision, err := ApplyNodeFailure(execution, node, NodeFailure{
			Message: "503",
			Now:     now,
		})
		if err != nil {
			t.Fatalf("ApplyNodeFailure returned error for attempt %d: %v", index+1, err)
		}
		if decision.Action != FailureActionRetry || decision.RetryAt == nil || !decision.RetryAt.Equal(now.Add(wantDelay)) {
			t.Fatalf("expected attempt %d retry at +%v, got %+v", index+1, wantDelay, decision)
		}

		execution = next
	}
}

func TestApplyNodeFailureBranchesToFailureNodeWithErrorContext(t *testing.T) {
	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	execution := Execution{
		ID:         "exec_1",
		WorkflowID: "workflow_1",
		Status:     ExecutionStatusRunning,
		NodeExecutions: []NodeExecution{
			{NodeID: "charge_card", Status: NodeStatusRunning, StartedAt: &now},
			{NodeID: "notify_ops", Status: NodeStatusPending},
		},
	}

	next, decision, err := ApplyNodeFailure(execution, Node{
		ID: "charge_card",
		FailurePolicy: FailurePolicy{
			Strategy:            FailureStrategyFailureBranch,
			FailureBranchNodeID: "notify_ops",
		},
	}, NodeFailure{
		Message: "payment declined",
		Now:     now,
	})

	if err != nil {
		t.Fatalf("ApplyNodeFailure returned error: %v", err)
	}
	if decision.Action != FailureActionBranch || decision.NextNodeID != "notify_ops" {
		t.Fatalf("expected branch to notify_ops, got %+v", decision)
	}
	if next.Status != ExecutionStatusRunning {
		t.Fatalf("expected execution to remain running, got %s", next.Status)
	}
	if next.NodeExecutions[0].Status != NodeStatusFailed || next.NodeExecutions[1].Status != NodeStatusPending {
		t.Fatalf("unexpected node states: %+v", next.NodeExecutions)
	}
	errCtx, ok := next.Variables["workflow.error"].(map[string]any)
	if !ok {
		t.Fatalf("expected workflow.error context, got %#v", next.Variables["workflow.error"])
	}
	if errCtx["node_id"] != "charge_card" || errCtx["message"] != "payment declined" {
		t.Fatalf("unexpected error context: %#v", errCtx)
	}
}
