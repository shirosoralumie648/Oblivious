package task

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidTaskTransition = errors.New("invalid task transition")

func continueRuntimeTask(detail TaskDetail, now time.Time) (TaskDetail, error) {
	if detail.Status != "running" {
		return TaskDetail{}, ErrInvalidTaskTransition
	}

	next := cloneTaskDetail(detail)
	runningIndex := findStepIndexByStatus(next.Steps, "running")
	if runningIndex == -1 {
		return TaskDetail{}, ErrInvalidTaskTransition
	}

	next.Steps[runningIndex].Status = "completed"
	next.Steps[runningIndex].UpdatedAt = now
	if next.Steps[runningIndex].StartedAt == nil {
		value := now
		next.Steps[runningIndex].StartedAt = &value
	}
	value := now
	next.Steps[runningIndex].FinishedAt = &value

	nextPendingIndex := findNextPendingStepIndex(next.Steps, runningIndex+1)
	if nextPendingIndex == -1 {
		next.Status = "completed"
		next.UpdatedAt = now
		next.FinishedAt = &value
		next.BudgetConsumed = nextRuntimeBudget(next.BudgetLimit, next.BudgetConsumed, true)
		next.ResultSummary = buildRuntimeResultSummary(next)
		return next, nil
	}

	next.Steps[nextPendingIndex].Status = "running"
	next.Steps[nextPendingIndex].UpdatedAt = now
	if next.Steps[nextPendingIndex].StartedAt == nil {
		value := now
		next.Steps[nextPendingIndex].StartedAt = &value
	}
	next.Steps[nextPendingIndex].FinishedAt = nil

	next.Status = "running"
	next.UpdatedAt = now
	next.FinishedAt = nil
	next.BudgetConsumed = nextRuntimeBudget(next.BudgetLimit, next.BudgetConsumed, false)
	next.ResultSummary = ""
	return next, nil
}

func pauseRuntimeTask(detail TaskDetail, now time.Time) (TaskDetail, error) {
	if detail.Status != "running" {
		return TaskDetail{}, ErrInvalidTaskTransition
	}

	next := cloneTaskDetail(detail)
	runningIndex := findStepIndexByStatus(next.Steps, "running")
	if runningIndex == -1 {
		return TaskDetail{}, ErrInvalidTaskTransition
	}

	next.Steps[runningIndex].Status = "paused"
	next.Steps[runningIndex].UpdatedAt = now
	next.Status = "paused"
	next.UpdatedAt = now
	next.FinishedAt = nil
	return next, nil
}

func resumeRuntimeTask(detail TaskDetail, now time.Time) (TaskDetail, error) {
	if detail.Status != "paused" {
		return TaskDetail{}, ErrInvalidTaskTransition
	}

	next := cloneTaskDetail(detail)
	pausedIndex := findStepIndexByStatus(next.Steps, "paused")
	if pausedIndex == -1 {
		pausedIndex = findStepIndexByStatus(next.Steps, "running")
	}
	if pausedIndex == -1 {
		pausedIndex = findStepIndexByStatus(next.Steps, "pending")
	}
	if pausedIndex == -1 {
		return TaskDetail{}, ErrInvalidTaskTransition
	}

	next.Steps[pausedIndex].Status = "running"
	next.Steps[pausedIndex].UpdatedAt = now
	if next.Steps[pausedIndex].StartedAt == nil {
		value := now
		next.Steps[pausedIndex].StartedAt = &value
	}
	next.Steps[pausedIndex].FinishedAt = nil

	next.Status = "running"
	next.UpdatedAt = now
	next.FinishedAt = nil
	next.ResultSummary = ""
	return next, nil
}

func deriveCurrentStep(task Task, steps []TaskStep) string {
	switch task.Status {
	case "running", "paused", "awaiting_confirmation":
	default:
		return ""
	}

	for _, status := range []string{"running", "paused", "awaiting_confirmation"} {
		if index := findStepIndexByStatus(steps, status); index != -1 {
			return steps[index].Title
		}
	}

	if index := findStepIndexByStatus(steps, "pending"); index != -1 {
		return steps[index].Title
	}

	return ""
}

func buildRuntimeEvents(detail TaskDetail) []TaskEvent {
	events := []TaskEvent{}
	if detail.StartedAt != nil {
		events = append(events, TaskEvent{
			CreatedAt: *detail.StartedAt,
			Message:   "Task execution started",
			Type:      "started",
		})
	}

	currentStep := deriveCurrentStep(detail.Task, detail.Steps)
	switch detail.Status {
	case "awaiting_confirmation":
		events = append(events, TaskEvent{
			CreatedAt: detail.UpdatedAt,
			Message:   fmt.Sprintf("Awaiting approval for %s", currentStep),
			Type:      "awaiting_confirmation",
		})
	case "running":
		events = append(events, TaskEvent{
			CreatedAt: detail.UpdatedAt,
			Message:   fmt.Sprintf("Executing %s", currentStep),
			Type:      "running",
		})
	case "paused":
		events = append(events, TaskEvent{
			CreatedAt: detail.UpdatedAt,
			Message:   fmt.Sprintf("Execution paused at %s", currentStep),
			Type:      "paused",
		})
	case "completed":
		createdAt := detail.UpdatedAt
		if detail.FinishedAt != nil {
			createdAt = *detail.FinishedAt
		}
		events = append(events, TaskEvent{
			CreatedAt: createdAt,
			Message:   "Runtime execution completed",
			Type:      "completed",
		})
	case "cancelled":
		createdAt := detail.UpdatedAt
		if detail.FinishedAt != nil {
			createdAt = *detail.FinishedAt
		}
		events = append(events, TaskEvent{
			CreatedAt: createdAt,
			Message:   "Runtime execution cancelled",
			Type:      "cancelled",
		})
	}

	return events
}

func buildTaskResultArtifacts(detail TaskDetail) []TaskResultArtifact {
	if detail.Status != "completed" {
		return []TaskResultArtifact{}
	}

	completedSteps := 0
	for _, step := range detail.Steps {
		if step.Status == "completed" {
			completedSteps++
		}
	}

	return []TaskResultArtifact{
		{Label: "Completed steps", Value: fmt.Sprintf("%d / %d", completedSteps, len(detail.Steps))},
		{Label: "Budget usage", Value: fmt.Sprintf("%d / %d", detail.BudgetConsumed, detail.BudgetLimit)},
		{Label: "Knowledge sources", Value: fmt.Sprintf("%d", len(detail.KnowledgeBaseIDs))},
	}
}

func buildRuntimeResultSummary(detail TaskDetail) string {
	completedSteps := 0
	for _, step := range detail.Steps {
		if step.Status == "completed" {
			completedSteps++
		}
	}

	title := strings.TrimSpace(detail.Title)
	if title == "" {
		title = detail.ID
	}

	lines := []string{
		fmt.Sprintf("Runtime result for %q", title),
		fmt.Sprintf("Completed steps: %d / %d", completedSteps, len(detail.Steps)),
		fmt.Sprintf("Budget usage: %d / %d", detail.BudgetConsumed, detail.BudgetLimit),
	}

	if goal := strings.TrimSpace(detail.Goal); goal != "" {
		lines = append(lines, fmt.Sprintf("Goal: %s", goal))
	}

	return strings.Join(lines, "\n")
}

func nextRuntimeBudget(limit, current int, finalize bool) int {
	if limit <= 0 {
		return current
	}
	if finalize {
		return limit
	}

	increment := (limit + 3) / 4
	if increment < 1 {
		increment = 1
	}

	next := current + increment
	if next >= limit {
		next = limit - 1
	}
	if next < 1 {
		next = 1
	}
	return next
}

func cloneTaskDetail(detail TaskDetail) TaskDetail {
	cloned := detail
	if detail.KnowledgeBaseIDs != nil {
		cloned.KnowledgeBaseIDs = append([]string(nil), detail.KnowledgeBaseIDs...)
	}
	if detail.Steps != nil {
		cloned.Steps = append([]TaskStep(nil), detail.Steps...)
	}
	return cloned
}

func findStepIndexByStatus(steps []TaskStep, status string) int {
	for index, step := range steps {
		if step.Status == status {
			return index
		}
	}

	return -1
}

func findNextPendingStepIndex(steps []TaskStep, startIndex int) int {
	for index := startIndex; index < len(steps); index++ {
		if steps[index].Status == "pending" {
			return index
		}
	}

	return -1
}
