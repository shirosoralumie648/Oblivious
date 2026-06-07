package debug

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrTraceNotFound = errors.New("execution trace not found")
	ErrNodeNotFound  = errors.New("node not found in trace")
)

// Tracer captures and analyzes workflow execution traces for debugging.
type Tracer struct {
	mu       sync.RWMutex
	traces   map[string]*ExecutionTrace
	variables map[string]*VariableStore
}

// ExecutionTrace records the full execution trace for a workflow execution.
type ExecutionTrace struct {
	ExecutionID string          `json:"executionId"`
	WorkflowID  string          `json:"workflowId"`
	StartedAt   time.Time       `json:"startedAt"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	Entries     []TraceEntry    `json:"entries"`
	Status      string          `json:"status"`
}

// TraceEntry records a single event in an execution trace.
type TraceEntry struct {
	Timestamp   time.Time      `json:"timestamp"`
	NodeID      string         `json:"nodeId"`
	NodeType    string         `json:"nodeType"`
	Status      string         `json:"status"`
	DurationMS  int            `json:"durationMs,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       map[string]any `json:"error,omitempty"`
	Attempt     int            `json:"attempt,omitempty"`
	Level       string         `json:"level"`
	Message     string         `json:"message"`
}

// VariableStore tracks variables at each point in execution for inspection.
type VariableStore struct {
	ExecutionID string                  `json:"executionId"`
	Snapshots   []VariableSnapshot      `json:"snapshots"`
}

// VariableSnapshot captures all variables at a specific point in execution.
type VariableSnapshot struct {
	Timestamp   time.Time              `json:"timestamp"`
	NodeID      string                 `json:"nodeId"`
	Variables   map[string]any         `json:"variables"`
	NodeOutputs map[string]map[string]any `json:"nodeOutputs"`
}

// VariableInspectResult holds the result of inspecting a specific variable.
type VariableInspectResult struct {
	Path     string `json:"path"`
	Value    any    `json:"value"`
	Type     string `json:"type"`
	Source   string `json:"source"`
	NodeID   string `json:"nodeId,omitempty"`
}

// NodeTestResult holds the result of a single-node test execution.
type NodeTestResult struct {
	WorkflowID string         `json:"workflowId"`
	NodeID     string         `json:"nodeId"`
	NodeType   string         `json:"nodeType"`
	Status     string         `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      map[string]any `json:"error,omitempty"`
	DurationMS int            `json:"durationMs,omitempty"`
	Trace      []TraceEntry   `json:"trace,omitempty"`
	Variables  []VariableInspectResult `json:"variables,omitempty"`
}

// NewTracer creates a new debug tracer.
func NewTracer() *Tracer {
	return &Tracer{
		traces:    make(map[string]*ExecutionTrace),
		variables: make(map[string]*VariableStore),
	}
}

// StartTrace begins a new execution trace.
func (t *Tracer) StartTrace(executionID, workflowID string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.traces[executionID] = &ExecutionTrace{
		ExecutionID: executionID,
		WorkflowID:  workflowID,
		StartedAt:   time.Now().UTC(),
		Entries:     []TraceEntry{},
		Status:      "running",
	}
	t.variables[executionID] = &VariableStore{
		ExecutionID: executionID,
		Snapshots:   []VariableSnapshot{},
	}
}

// RecordEntry adds a trace entry for a node event.
func (t *Tracer) RecordEntry(executionID string, entry TraceEntry) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	trace, ok := t.traces[executionID]
	if !ok {
		return
	}
	entry.Timestamp = time.Now().UTC()
	if entry.Level == "" {
		entry.Level = levelForStatus(entry.Status)
	}
	if entry.Message == "" {
		entry.Message = formatTraceMessage(entry)
	}
	trace.Entries = append(trace.Entries, entry)
}

// RecordVariables captures a variable snapshot at a node.
func (t *Tracer) RecordVariables(executionID, nodeID string, variables map[string]any, nodeOutputs map[string]map[string]any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	store, ok := t.variables[executionID]
	if !ok {
		return
	}
	store.Snapshots = append(store.Snapshots, VariableSnapshot{
		Timestamp:   time.Now().UTC(),
		NodeID:      nodeID,
		Variables:   cloneMap(variables),
		NodeOutputs: cloneNestedMap(nodeOutputs),
	})
}

// CompleteTrace marks a trace as completed.
func (t *Tracer) CompleteTrace(executionID, status string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	trace, ok := t.traces[executionID]
	if !ok {
		return
	}
	now := time.Now().UTC()
	trace.CompletedAt = &now
	trace.Status = status
}

// GetTrace returns the full execution trace.
func (t *Tracer) GetTrace(executionID string) (*ExecutionTrace, error) {
	if t == nil {
		return nil, fmt.Errorf("tracer is nil")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	trace, ok := t.traces[executionID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTraceNotFound, executionID)
	}
	return cloneTrace(trace), nil
}

// GetNodeEntries returns all trace entries for a specific node.
func (t *Tracer) GetNodeEntries(executionID, nodeID string) ([]TraceEntry, error) {
	if t == nil {
		return nil, fmt.Errorf("tracer is nil")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	trace, ok := t.traces[executionID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTraceNotFound, executionID)
	}

	entries := []TraceEntry{}
	for _, entry := range trace.Entries {
		if entry.NodeID == nodeID {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: node %s in execution %s", ErrNodeNotFound, nodeID, executionID)
	}
	return entries, nil
}

// InspectVariable inspects a variable value at a specific point in execution.
func (t *Tracer) InspectVariable(executionID, nodeID, varPath string) (*VariableInspectResult, error) {
	if t == nil {
		return nil, fmt.Errorf("tracer is nil")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	store, ok := t.variables[executionID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTraceNotFound, executionID)
	}

	// Search snapshots from newest to oldest, preferring the requested node.
	var bestSnapshot *VariableSnapshot
	for i := len(store.Snapshots) - 1; i >= 0; i-- {
		snap := &store.Snapshots[i]
		if snap.NodeID == nodeID {
			bestSnapshot = snap
			break
		}
		if bestSnapshot == nil {
			bestSnapshot = snap
		}
	}
	if bestSnapshot == nil {
		return nil, fmt.Errorf("no variable snapshots found for execution %s", executionID)
	}

	value, source, err := resolveVariablePath(varPath, bestSnapshot.Variables, bestSnapshot.NodeOutputs)
	if err != nil {
		return nil, err
	}

	return &VariableInspectResult{
		Path:   varPath,
		Value:  value,
		Type:   typeOfValue(value),
		Source: source,
		NodeID: bestSnapshot.NodeID,
	}, nil
}

// InspectAllVariables returns all variables available at a given node.
func (t *Tracer) InspectAllVariables(executionID, nodeID string) ([]VariableInspectResult, error) {
	if t == nil {
		return nil, fmt.Errorf("tracer is nil")
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	store, ok := t.variables[executionID]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTraceNotFound, executionID)
	}

	var bestSnapshot *VariableSnapshot
	for i := len(store.Snapshots) - 1; i >= 0; i-- {
		snap := &store.Snapshots[i]
		if snap.NodeID == nodeID {
			bestSnapshot = snap
			break
		}
		if bestSnapshot == nil {
			bestSnapshot = snap
		}
	}
	if bestSnapshot == nil {
		return []VariableInspectResult{}, nil
	}

	results := []VariableInspectResult{}
	for key, value := range bestSnapshot.Variables {
		results = append(results, VariableInspectResult{
			Path:   key,
			Value:  value,
			Type:   typeOfValue(value),
			Source: "variables",
		})
	}
	for nodeIDKey, outputs := range bestSnapshot.NodeOutputs {
		for key, value := range outputs {
			path := fmt.Sprintf("nodes.%s.%s", nodeIDKey, key)
			results = append(results, VariableInspectResult{
				Path:   path,
				Value:  value,
				Type:   typeOfValue(value),
				Source: "nodeOutput",
				NodeID: nodeIDKey,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return results, nil
}

// GetTimeline returns a sorted timeline of all trace entries.
func (t *Tracer) GetTimeline(executionID string) ([]TraceEntry, error) {
	trace, err := t.GetTrace(executionID)
	if err != nil {
		return nil, err
	}
	entries := make([]TraceEntry, len(trace.Entries))
	copy(entries, trace.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}

// GetSummary returns a summary of the execution trace.
func (t *Tracer) GetSummary(executionID string) (map[string]any, error) {
	trace, err := t.GetTrace(executionID)
	if err != nil {
		return nil, err
	}

	nodeCounts := map[string]int{}
	statusCounts := map[string]int{}
	totalDuration := 0
	bottleneckNode := ""
	bottleneckDuration := 0

	for _, entry := range trace.Entries {
		nodeCounts[entry.NodeID]++
		statusCounts[entry.Status]++
		totalDuration += entry.DurationMS
		if entry.DurationMS > bottleneckDuration {
			bottleneckDuration = entry.DurationMS
			bottleneckNode = entry.NodeID
		}
	}

	durationMS := 0
	if trace.CompletedAt != nil {
		durationMS = int(trace.CompletedAt.Sub(trace.StartedAt).Milliseconds())
	}

	return map[string]any{
		"executionId":       trace.ExecutionID,
		"workflowId":        trace.WorkflowID,
		"status":            trace.Status,
		"startedAt":         trace.StartedAt,
		"completedAt":       trace.CompletedAt,
		"durationMs":        durationMS,
		"totalEntries":      len(trace.Entries),
		"uniqueNodes":       len(nodeCounts),
		"statusCounts":      statusCounts,
		"bottleneckNode":    bottleneckNode,
		"bottleneckMs":      bottleneckDuration,
		"totalNodeDuration": totalDuration,
	}, nil
}

func resolveVariablePath(path string, variables map[string]any, nodeOutputs map[string]map[string]any) (any, string, error) {
	parts := strings.SplitN(path, ".", 2)
	if len(parts) == 0 {
		return nil, "", fmt.Errorf("empty variable path")
	}

	if parts[0] == "nodes" && len(parts) == 2 {
		nodeParts := strings.SplitN(parts[1], ".", 2)
		if len(nodeParts) == 2 {
			if outputs, ok := nodeOutputs[nodeParts[0]]; ok {
				if value, ok := outputs[nodeParts[1]]; ok {
					return value, "nodeOutput", nil
				}
			}
		}
	}

	if value, ok := variables[path]; ok {
		return value, "variables", nil
	}

	if len(parts) == 1 {
		if value, ok := variables[parts[0]]; ok {
			return value, "variables", nil
		}
	}

	return nil, "", fmt.Errorf("variable %s not found", path)
}

func typeOfValue(value any) string {
	if value == nil {
		return "nil"
	}
	switch value.(type) {
	case string:
		return "string"
	case float64, float32, int, int64, int32:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func levelForStatus(status string) string {
	switch strings.ToLower(status) {
	case "failed", "error":
		return "error"
	case "retrying", "warning":
		return "warning"
	default:
		return "info"
	}
}

func formatTraceMessage(entry TraceEntry) string {
	nodeID := strings.TrimSpace(entry.NodeID)
	if nodeID == "" {
		nodeID = "node"
	}
	message := fmt.Sprintf("Node %s %s", nodeID, entry.Status)
	if entry.DurationMS > 0 {
		message = fmt.Sprintf("%s in %dms", message, entry.DurationMS)
	}
	return message
}

func cloneTrace(trace *ExecutionTrace) *ExecutionTrace {
	if trace == nil {
		return nil
	}
	cloned := *trace
	cloned.Entries = make([]TraceEntry, len(trace.Entries))
	copy(cloned.Entries, trace.Entries)
	return &cloned
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func cloneNestedMap(m map[string]map[string]any) map[string]map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]map[string]any, len(m))
	for k, v := range m {
		result[k] = cloneMap(v)
	}
	return result
}
