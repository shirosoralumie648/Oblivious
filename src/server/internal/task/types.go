package task

import (
	"time"
)

// ExecutionMode constants define how a task executes.
const (
	ExecutionModeStandard = "standard"
	ExecutionModeSafe     = "safe"
	ExecutionModeAuto     = "auto"
)

// AuthorizationScope constants define the scope of task authorization.
const (
	AuthorizationScopeWorkspaceTools = "workspace_tools"
	AuthorizationScopeKnowledgeOnly = "knowledge_only"
	AuthorizationScopeFullAccess    = "full_access"
)

// TaskStatus constants define the lifecycle states of a task.
const (
	TaskStatusDraft              = "draft"
	TaskStatusRunning            = "running"
	TaskStatusPaused             = "paused"
	TaskStatusAwaitingConfirmation = "awaiting_confirmation"
	TaskStatusCompleted          = "completed"
	TaskStatusCancelled          = "cancelled"
)

// TaskStepStatus constants define the lifecycle states of a task step.
const (
	TaskStepStatusPending              = "pending"
	TaskStepStatusRunning              = "running"
	TaskStepStatusPaused               = "paused"
	TaskStepStatusAwaitingConfirmation = "awaiting_confirmation"
	TaskStepStatusCompleted            = "completed"
	TaskStepStatusCancelled            = "cancelled"
)

// CreateTaskInput contains the parameters for creating a new task.
type CreateTaskInput struct {
	Title              string   `json:"title"`
	Goal               string   `json:"goal"`
	ExecutionMode      string   `json:"executionMode"`
	AuthorizationScope string   `json:"authorizationScope"`
	BudgetLimit        int      `json:"budgetLimit"`
	KnowledgeBaseIDs   []string `json:"knowledgeBaseIds"`
	ToolAllowList      []string `json:"toolAllowList"`
	ToolDenyList       []string `json:"toolDenyList"`
}

// TaskFilter contains optional filters for listing tasks.
type TaskFilter struct {
	Status    string `json:"status,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

// TaskStats contains aggregated statistics about tasks.
type TaskStats struct {
	TotalTasks      int     `json:"totalTasks"`
	RunningTasks    int     `json:"runningTasks"`
	CompletedTasks  int     `json:"completedTasks"`
	FailedTasks     int     `json:"failedTasks"`
	TotalBudgetUsed int     `json:"totalBudgetUsed"`
	AvgStepsPerTask float64 `json:"avgStepsPerTask"`
}

// TaskEvent constants define the types of task events.
const (
	TaskEventTypeStarted              = "started"
	TaskEventTypeRunning              = "running"
	TaskEventTypePaused               = "paused"
	TaskEventTypeAwaitingConfirmation = "awaiting_confirmation"
	TaskEventTypeCompleted            = "completed"
	TaskEventTypeCancelled            = "cancelled"
)

// TaskResult constants define result artifact types.
const (
	TaskResultArtifactTypeSteps    = "steps"
	TaskResultArtifactTypeBudget   = "budget"
	TaskResultArtifactTypeSources  = "sources"
)

// TaskEventWithTimestamp pairs a task event with a specific timestamp.
type TaskEventWithTimestamp struct {
	Event     TaskEvent `json:"Event"`
	Timestamp time.Time `json:"timestamp"`
}

// TaskProgress represents the current progress of a running task.
type TaskProgress struct {
	TaskID         string    `json:"taskId"`
	CurrentStep    int       `json:"currentStep"`
	TotalSteps     int       `json:"totalSteps"`
	ProgressPct    float64   `json:"progressPct"`
	CurrentStepTitle string  `json:"currentStepTitle"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
