package workflow

import "time"

type WorkflowStatus string

const (
	WorkflowStatusDraft     WorkflowStatus = "draft"
	WorkflowStatusPublished WorkflowStatus = "published"
	WorkflowStatusArchived  WorkflowStatus = "archived"
)

type ExecutionStatus string

const (
	ExecutionStatusQueued         ExecutionStatus = "queued"
	ExecutionStatusRunning        ExecutionStatus = "running"
	ExecutionStatusPaused         ExecutionStatus = "paused"
	ExecutionStatusSucceeded      ExecutionStatus = "succeeded"
	ExecutionStatusCompleted      ExecutionStatus = "completed"
	ExecutionStatusFailed         ExecutionStatus = "failed"
	ExecutionStatusCancelled      ExecutionStatus = "cancelled"
	ExecutionStatusPartialSuccess ExecutionStatus = "partial_success"
	ExecutionStatusTimedOut       ExecutionStatus = "timeout"
	ExecutionStatusMaxIterations  ExecutionStatus = "max_iterations"
)

type NodeStatus string

const (
	NodeStatusPending   NodeStatus = "pending"
	NodeStatusRunning   NodeStatus = "running"
	NodeStatusWaiting   NodeStatus = "waiting"
	NodeStatusRetrying  NodeStatus = "retrying"
	NodeStatusSucceeded NodeStatus = "succeeded"
	NodeStatusCompleted NodeStatus = "completed"
	NodeStatusFailed    NodeStatus = "failed"
	NodeStatusSkipped   NodeStatus = "skipped"
)

type WorkflowTriggerType string

const (
	WorkflowTriggerManual       WorkflowTriggerType = "manual"
	WorkflowTriggerConversation WorkflowTriggerType = "conversation"
	WorkflowTriggerSchedule     WorkflowTriggerType = "schedule"
	WorkflowTriggerWebhook      WorkflowTriggerType = "webhook"
	WorkflowTriggerSemantic     WorkflowTriggerType = "semantic"
)

type FailureStrategy string

const (
	FailureStrategyPauseOnFailure FailureStrategy = "pause_on_failure"
	FailureStrategyAutoRetry      FailureStrategy = "auto_retry"
	FailureStrategySkipOnFailure  FailureStrategy = "skip_on_failure"
	FailureStrategyFailureBranch  FailureStrategy = "failure_branch"
)

type FailureAction string

const (
	FailureActionPause    FailureAction = "pause"
	FailureActionRetry    FailureAction = "retry"
	FailureActionContinue FailureAction = "continue"
	FailureActionBranch   FailureAction = "branch"
	FailureActionFail     FailureAction = "fail"
)

type FailurePolicy struct {
	Strategy            FailureStrategy `json:"strategy"`
	MaxRetries          int             `json:"maxRetries,omitempty"`
	RetryDelays         []time.Duration `json:"retryDelays,omitempty"`
	FailureBranchNodeID string          `json:"failureBranchNodeId,omitempty"`
}

type Node struct {
	ID            string         `json:"id"`
	Type          string         `json:"type,omitempty"`
	Input         map[string]any `json:"input,omitempty"`
	FailurePolicy FailurePolicy  `json:"failurePolicy,omitempty"`
}

type Execution struct {
	ID             string          `json:"id"`
	WorkflowID     string          `json:"workflowId"`
	Status         ExecutionStatus `json:"status"`
	PauseReason    string          `json:"pauseReason,omitempty"`
	Variables      map[string]any  `json:"variables,omitempty"`
	NodeExecutions []NodeExecution `json:"nodeExecutions,omitempty"`
	UpdatedAt      time.Time       `json:"updatedAt,omitempty"`
	FinishedAt     *time.Time      `json:"finishedAt,omitempty"`
}

type NodeExecution struct {
	NodeID     string     `json:"nodeId"`
	Status     NodeStatus `json:"status"`
	Attempt    int        `json:"attempt,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	RetryAt    *time.Time `json:"retryAt,omitempty"`
}

type WorkflowDefinition struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	Description    string         `json:"description,omitempty"`
	Status         WorkflowStatus `json:"status"`
	Version        int            `json:"version"`
	Definition     map[string]any `json:"definition"`
	Variables      map[string]any `json:"variables,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type WorkflowExecution struct {
	ID               string                  `json:"id"`
	WorkflowID       string                  `json:"workflowId"`
	WorkflowVersion  int                     `json:"workflowVersion"`
	OrganizationID   string                  `json:"organizationId"`
	Status           ExecutionStatus         `json:"status"`
	Input            map[string]any          `json:"input,omitempty"`
	Output           map[string]any          `json:"output,omitempty"`
	Error            map[string]any          `json:"error,omitempty"`
	Context          map[string]any          `json:"context,omitempty"`
	WorkflowSnapshot map[string]any          `json:"workflowSnapshot,omitempty"`
	StartedAt        time.Time               `json:"startedAt"`
	CompletedAt      *time.Time              `json:"completedAt,omitempty"`
	DurationMS       int                     `json:"durationMs,omitempty"`
	CreatedAt        time.Time               `json:"createdAt"`
	UpdatedAt        time.Time               `json:"updatedAt"`
	NodeExecutions   []WorkflowNodeExecution `json:"nodeExecutions,omitempty"`
}

type WorkflowExecutionHealthSummary struct {
	Status          ExecutionStatus `json:"status"`
	Count           int             `json:"count"`
	OldestStartedAt time.Time       `json:"oldestStartedAt,omitempty"`
}

type WorkflowNodeExecution struct {
	ID             string         `json:"id"`
	ExecutionID    string         `json:"executionId"`
	OrganizationID string         `json:"organizationId"`
	NodeID         string         `json:"nodeId"`
	NodeType       string         `json:"nodeType"`
	Status         NodeStatus     `json:"status"`
	Attempt        int            `json:"attempt,omitempty"`
	Input          map[string]any `json:"input,omitempty"`
	Output         map[string]any `json:"output,omitempty"`
	Error          map[string]any `json:"error,omitempty"`
	Context        map[string]any `json:"context,omitempty"`
	StartedAt      time.Time      `json:"startedAt"`
	CompletedAt    *time.Time     `json:"completedAt,omitempty"`
	DurationMS     int            `json:"durationMs,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type ExecutionVariableSnapshot struct {
	Input       map[string]any            `json:"input"`
	Context     map[string]any            `json:"context"`
	NodeOutputs map[string]map[string]any `json:"nodeOutputs"`
}

type ExecutionDebugTraceEntry struct {
	NodeID      string         `json:"nodeId"`
	NodeType    string         `json:"nodeType"`
	Status      NodeStatus     `json:"status"`
	Attempt     int            `json:"attempt,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       map[string]any `json:"error,omitempty"`
	Context     map[string]any `json:"context,omitempty"`
	StartedAt   time.Time      `json:"startedAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
	DurationMS  int            `json:"durationMs,omitempty"`
}

type ExecutionDebugPerformance struct {
	TotalDurationMS  int            `json:"totalDurationMs"`
	NodeDurationsMS  map[string]int `json:"nodeDurationsMs"`
	BottleneckNodeID string         `json:"bottleneckNodeId,omitempty"`
}

type ExecutionDebugLogEntry struct {
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"nodeId,omitempty"`
}

type ExecutionDebugSnapshot struct {
	ExecutionID      string                     `json:"executionId"`
	WorkflowID       string                     `json:"workflowId"`
	Status           ExecutionStatus            `json:"status"`
	VariableSnapshot ExecutionVariableSnapshot  `json:"variableSnapshot"`
	Trace            []ExecutionDebugTraceEntry `json:"trace"`
	Outputs          map[string]map[string]any  `json:"outputs"`
	Performance      ExecutionDebugPerformance  `json:"performance"`
	Logs             []ExecutionDebugLogEntry   `json:"logs"`
}

type NodeFailure struct {
	Message string
	Now     time.Time
}

type FailureDecision struct {
	Action     FailureAction `json:"action"`
	NextNodeID string        `json:"nextNodeId,omitempty"`
	RetryAt    *time.Time    `json:"retryAt,omitempty"`
}

// --- New types for enhanced workflow engine ---

// DAGAnalysis holds the result of DAG parsing and analysis.
type DAGAnalysis struct {
	TopologicalOrder []string   `json:"topologicalOrder"`
	ParallelGroups   [][]string `json:"parallelGroups"`
	RootNodes        []string   `json:"rootNodes"`
	LeafNodes        []string   `json:"leafNodes"`
	Levels           []string   `json:"levels"`
	HasCycle         bool       `json:"hasCycle"`
	CycleNodes       []string   `json:"cycleNodes,omitempty"`
}

// WorkflowStateMachineEvent represents an event that triggers a state transition.
type WorkflowStateMachineEvent string

const (
	StateEventStart    WorkflowStateMachineEvent = "start"
	StateEventPause    WorkflowStateMachineEvent = "pause"
	StateEventResume   WorkflowStateMachineEvent = "resume"
	StateEventComplete WorkflowStateMachineEvent = "complete"
	StateEventFail     WorkflowStateMachineEvent = "fail"
	StateEventTimeout  WorkflowStateMachineEvent = "timeout"
	StateEventCancel   WorkflowStateMachineEvent = "cancel"
)

// WorkflowStateMachineTransition defines a valid state transition.
type WorkflowStateMachineTransition struct {
	From  ExecutionStatus
	To    ExecutionStatus
	Event WorkflowStateMachineEvent
}

// WebhookTriggerConfig defines configuration for a webhook trigger.
type WebhookTriggerConfig struct {
	ID             string         `json:"id"`
	URL            string         `json:"url"`
	Secret         string         `json:"secret,omitempty"`
	AllowedMethods []string       `json:"allowedMethods,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Definition     map[string]any `json:"definition,omitempty"`
}

// WebhookVerificationResult contains the result of webhook payload verification.
type WebhookVerificationResult struct {
	Valid    bool   `json:"valid"`
	Error    string `json:"error,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
}

// ScheduleTriggerConfig defines configuration for a schedule trigger.
type ScheduleTriggerConfig struct {
	ID             string         `json:"id"`
	Name           string         `json:"name,omitempty"`
	CronExpression string         `json:"cronExpression"`
	Timezone       string         `json:"timezone,omitempty"`
	Enabled        bool           `json:"enabled"`
	NextRunAt      *time.Time     `json:"nextRunAt,omitempty"`
	LastRunAt      *time.Time     `json:"lastRunAt,omitempty"`
	Definition     map[string]any `json:"definition,omitempty"`
}

// ScheduleNextRunResult contains the next scheduled run time.
type ScheduleNextRunResult struct {
	NextRunAt   time.Time `json:"nextRunAt"`
	Description string    `json:"description,omitempty"`
}

// FailureRetryConfig defines retry behavior for node failures.
type FailureRetryConfig struct {
	MaxRetries      int             `json:"maxRetries"`
	InitialDelay    time.Duration   `json:"initialDelay"`
	MaxDelay        time.Duration   `json:"maxDelay"`
	BackoffFactor   float64         `json:"backoffFactor"`
	RetryableErrors []string        `json:"retryableErrors,omitempty"`
}

// FailureRetryState tracks the current retry state for a node execution.
type FailureRetryState struct {
	NodeID        string        `json:"nodeId"`
	Attempt       int           `json:"attempt"`
	MaxRetries    int           `json:"maxRetries"`
	LastError     string        `json:"lastError,omitempty"`
	NextRetryAt   *time.Time    `json:"nextRetryAt,omitempty"`
	BackoffDelay  time.Duration `json:"backoffDelay"`
	TotalWaitTime time.Duration `json:"totalWaitTime"`
}

// VersionMetadata holds metadata for a workflow version.
type VersionMetadata struct {
	Version     int       `json:"version"`
	WorkflowID  string    `json:"workflowId"`
	BranchName  string    `json:"branchName,omitempty"`
	IsPublished bool      `json:"isPublished"`
	CreatedAt   time.Time `json:"createdAt"`
	Changelog   string    `json:"changelog,omitempty"`
}

// ExecutionTraceEntry represents a single entry in an execution trace.
type ExecutionTraceEntry struct {
	Timestamp   time.Time      `json:"timestamp"`
	NodeID      string         `json:"nodeId"`
	NodeType    string         `json:"nodeType"`
	Status      NodeStatus     `json:"status"`
	DurationMS  int            `json:"durationMs,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       map[string]any `json:"error,omitempty"`
	Attempt     int            `json:"attempt,omitempty"`
}

// VariableInspectResult holds the result of variable inspection for debugging.
type VariableInspectResult struct {
	Path      string `json:"path"`
	Value     any    `json:"value"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	NodeID    string `json:"nodeId,omitempty"`
}

// NodeTestResult holds the result of a single-node test execution.
type NodeTestResult struct {
	WorkflowID string         `json:"workflowId"`
	NodeID     string         `json:"nodeId"`
	NodeType   string         `json:"nodeType"`
	Status     ExecutionStatus `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      map[string]any `json:"error,omitempty"`
	DurationMS int            `json:"durationMs,omitempty"`
	Trace      []ExecutionTraceEntry `json:"trace,omitempty"`
	Variables  []VariableInspectResult `json:"variables,omitempty"`
}
