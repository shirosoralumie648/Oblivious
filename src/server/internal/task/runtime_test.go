package task

import (
	"strings"
	"testing"
	"time"
)

func TestResumeRuntimeTaskReactivatesPausedStep(t *testing.T) {
	now := time.Date(2026, time.April, 4, 9, 30, 0, 0, time.UTC)
	detail := TaskDetail{
		Task: Task{
			BudgetConsumed: 4,
			BudgetLimit:    12,
			ExecutionMode:  "standard",
			Goal:           "Review launch plan",
			ID:             "task_1",
			Status:         "paused",
			Title:          "Review launch plan",
		},
		Steps: []TaskStep{
			{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
			{ID: "step_2", Status: "paused", StepIndex: 2, Title: "Review workspace context"},
			{ID: "step_3", Status: "pending", StepIndex: 3, Title: "Deliver runtime result"},
		},
	}

	resumed, err := resumeRuntimeTask(detail, now)
	if err != nil {
		t.Fatalf("resume runtime task: %v", err)
	}

	if resumed.Status != "running" {
		t.Fatalf("expected resumed status running, got %+v", resumed)
	}
	if resumed.Steps[1].Status != "running" {
		t.Fatalf("expected paused step to resume running, got %+v", resumed.Steps)
	}
	if resumed.FinishedAt != nil {
		t.Fatalf("expected resumed task to remain unfinished, got %+v", resumed)
	}
}

func TestContinueRuntimeTaskCompletesFinalStepWithStructuredSummary(t *testing.T) {
	now := time.Date(2026, time.April, 4, 10, 0, 0, 0, time.UTC)
	detail := TaskDetail{
		Task: Task{
			AuthorizationScope: "workspace_tools",
			BudgetConsumed:     8,
			BudgetLimit:        12,
			ExecutionMode:      "standard",
			Goal:               "Review launch plan",
			ID:                 "task_1",
			Status:             "running",
			Title:              "Review launch plan",
		},
		KnowledgeBaseIDs: []string{"kb_1"},
		Steps: []TaskStep{
			{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
			{ID: "step_2", Status: "completed", StepIndex: 2, Title: "Review workspace context"},
			{ID: "step_3", Status: "running", StepIndex: 3, Title: "Deliver runtime result"},
		},
	}

	completed, err := continueRuntimeTask(detail, now)
	if err != nil {
		t.Fatalf("continue runtime task: %v", err)
	}

	if completed.Status != "completed" {
		t.Fatalf("expected completed task, got %+v", completed)
	}
	if completed.FinishedAt == nil || !completed.FinishedAt.Equal(now) {
		t.Fatalf("expected finished timestamp %s, got %+v", now, completed)
	}
	if !strings.Contains(completed.ResultSummary, "Completed steps: 3 / 3") {
		t.Fatalf("expected structured result summary, got %q", completed.ResultSummary)
	}
	if strings.Contains(strings.ToLower(completed.ResultSummary), "starter solo") {
		t.Fatalf("expected runtime summary instead of starter placeholder, got %q", completed.ResultSummary)
	}
}
