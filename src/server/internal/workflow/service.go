package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"oblivious/server/internal/metrics"
)

var (
	ErrNotFound                 = errors.New("workflow resource not found")
	ErrInvalidInput             = errors.New("invalid workflow input")
	ErrInvalidTransition        = errors.New("invalid workflow transition")
	ErrWorkflowConcurrencyLimit = errors.New("workflow concurrency limit reached")
	ErrWorkflowResourceLimit    = errors.New("workflow resource limit reached")
)

type Service struct {
	store                     Store
	orgMaxConcurrentWorkflows int
	nodeExecutors             *NodeExecutorRegistry
	semanticTriggerMatcher    SemanticTriggerMatcher
	scheduleSyncer            ScheduleSyncer
	systemLimits              SystemWorkflowLimits
	systemLimitWindow         []time.Time
	systemLimitMu             sync.Mutex
}

type ServiceOption func(*Service)

type SystemWorkflowLimits struct {
	MaxConcurrentWorkflows int
	MaxExecutionsPerMinute int
}

type ScheduleSyncer interface {
	SyncWorkflowScheduleTriggers(ctx context.Context, req WorkflowScheduleSyncRequest) error
}

type WorkflowScheduleSyncRequest struct {
	OrganizationID string
	WorkflowID     string
	Triggers       []WorkflowScheduleTrigger
	Now            time.Time
}

type WorkflowScheduleTrigger struct {
	ID             string
	Name           string
	CronExpression string
	Enabled        bool
	Definition     map[string]any
}

type SemanticTriggerMatcher interface {
	MatchSemanticTrigger(ctx context.Context, req SemanticTriggerMatchRequest) (SemanticTriggerMatchDecision, error)
}

type agentApprovalNodeExecutor interface {
	ApproveToolRun(ctx context.Context, input NodeExecutorInput, pending WorkflowNodeExecution, submitted map[string]any) (map[string]any, error)
}

type SemanticTriggerMatchRequest struct {
	OrganizationID string
	UserID         string
	Message        string
	TriggerID      string
	Keywords       []string
	Threshold      float64
	Definition     map[string]any
}

type SemanticTriggerMatchDecision struct {
	Matched     bool
	Keyword     string
	Score       float64
	MatchMethod string
}

func WithOrgMaxConcurrentWorkflows(limit int) ServiceOption {
	return func(service *Service) {
		if limit > 0 {
			service.orgMaxConcurrentWorkflows = limit
		}
	}
}

func WithSystemWorkflowLimits(limits SystemWorkflowLimits) ServiceOption {
	return func(service *Service) {
		if limits.MaxConcurrentWorkflows > 0 {
			service.systemLimits.MaxConcurrentWorkflows = limits.MaxConcurrentWorkflows
		}
		if limits.MaxExecutionsPerMinute > 0 {
			service.systemLimits.MaxExecutionsPerMinute = limits.MaxExecutionsPerMinute
		}
	}
}

func WithNodeExecutors(registry *NodeExecutorRegistry) ServiceOption {
	return func(service *Service) {
		if registry != nil {
			service.nodeExecutors = registry
		}
	}
}

func WithSemanticTriggerMatcher(matcher SemanticTriggerMatcher) ServiceOption {
	return func(service *Service) {
		service.semanticTriggerMatcher = matcher
	}
}

func WithScheduleSyncer(syncer ScheduleSyncer) ServiceOption {
	return func(service *Service) {
		service.scheduleSyncer = syncer
	}
}

func NewService(store Store, options ...ServiceOption) *Service {
	service := &Service{
		store:                     store,
		orgMaxConcurrentWorkflows: defaultOrgMaxConcurrentWorkflows,
		nodeExecutors:             defaultNodeExecutorRegistry(),
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) RegisterNodeExecutors(executors ...NodeExecutor) {
	if s == nil {
		return
	}
	if s.nodeExecutors == nil {
		s.nodeExecutors = defaultNodeExecutorRegistry()
	}
	for _, executor := range executors {
		s.nodeExecutors.Register(executor)
	}
}

func (s *Service) SetScheduleSyncer(syncer ScheduleSyncer) {
	if s == nil {
		return
	}
	s.scheduleSyncer = syncer
}

type StartExecutionRequest struct {
	OrganizationID string
	WorkflowID     string
	TriggerType    WorkflowTriggerType
	TriggerPayload map[string]any
	Input          map[string]any
	Context        map[string]any
}

type MatchSemanticTriggersRequest struct {
	OrganizationID string
	UserID         string
	Message        string
}

type MatchConversationTriggersRequest struct {
	OrganizationID string
	ConversationID string
}

type ConversationTriggerMatch struct {
	WorkflowID         string         `json:"workflowId"`
	WorkflowVersion    int            `json:"workflowVersion"`
	WorkflowName       string         `json:"workflowName"`
	TriggerID          string         `json:"triggerId,omitempty"`
	ConversationID     string         `json:"conversationId"`
	TriggerDefinition  map[string]any `json:"triggerDefinition,omitempty"`
	WorkflowDefinition map[string]any `json:"workflowDefinition,omitempty"`
}

type SemanticTriggerMatch struct {
	WorkflowID         string         `json:"workflowId"`
	WorkflowVersion    int            `json:"workflowVersion"`
	WorkflowName       string         `json:"workflowName"`
	TriggerID          string         `json:"triggerId,omitempty"`
	Keyword            string         `json:"keyword"`
	SemanticThreshold  float64        `json:"semanticThreshold,omitempty"`
	Score              float64        `json:"score,omitempty"`
	MatchMethod        string         `json:"matchMethod,omitempty"`
	TriggerDefinition  map[string]any `json:"triggerDefinition,omitempty"`
	WorkflowDefinition map[string]any `json:"workflowDefinition,omitempty"`
}

type RollbackWorkflowRequest struct {
	OrganizationID string
	WorkflowID     string
	Version        int
}

type CreateWorkflowBranchRequest struct {
	OrganizationID string
	WorkflowID     string
	Version        int
	Name           string
	Description    string
	ExperimentKey  string
	TrafficPercent int
}

type UpdateWorkflowRequest struct {
	OrganizationID string
	WorkflowID     string
	Name           *string
	Description    *string
	Status         *WorkflowStatus
	Definition     map[string]any
	Variables      map[string]any
}

type TestNodeRequest struct {
	OrganizationID string
	WorkflowID     string
	NodeID         string
	Input          map[string]any
}

type TestNodeResult struct {
	WorkflowID string           `json:"workflowId"`
	NodeID     string           `json:"nodeId"`
	Status     ExecutionStatus  `json:"status"`
	Input      map[string]any   `json:"input,omitempty"`
	Output     map[string]any   `json:"output,omitempty"`
	Error      map[string]any   `json:"error,omitempty"`
	DurationMS int              `json:"durationMs,omitempty"`
	Trace      []map[string]any `json:"trace,omitempty"`
}

type RecordNodeStatusRequest struct {
	NodeID      string
	NodeType    string
	Status      NodeStatus
	Attempt     int
	Input       map[string]any
	Output      map[string]any
	Error       map[string]any
	Context     map[string]any
	StartedAt   time.Time
	CompletedAt *time.Time
	DurationMS  int
}

type ResolveFailureDecisionRequest struct {
	Action     FailureAction
	Input      map[string]any
	NextNodeID string
	NodeID     string
}

type ResumeExecutionRequest struct {
	NodeID string
	Input  map[string]any
}

func (s *Service) CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*WorkflowDefinition, error) {
	if err := validateCreateWorkflowRequest(req); err != nil {
		return nil, err
	}
	if err := validateScheduleTriggersForStatus(req.Status, req.Definition); err != nil {
		return nil, err
	}
	created, err := s.store.CreateWorkflow(ctx, req)
	if err != nil {
		return nil, err
	}
	if created.Status == WorkflowStatusPublished {
		if err := s.syncWorkflowScheduleTriggers(ctx, created); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (s *Service) ListWorkflows(ctx context.Context, organizationID string) ([]*WorkflowDefinition, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	return s.store.ListWorkflows(ctx, organizationID)
}

func (s *Service) GetWorkflow(ctx context.Context, organizationID, id string) (*WorkflowDefinition, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}
	workflow, err := s.store.GetWorkflow(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if workflow == nil {
		return nil, fmt.Errorf("%w: workflow %s", ErrNotFound, id)
	}
	return workflow, nil
}

func (s *Service) ListWorkflowVersions(ctx context.Context, organizationID, workflowID string) ([]*WorkflowDefinition, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(workflowID) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}
	if _, err := s.GetWorkflow(ctx, organizationID, workflowID); err != nil {
		return nil, err
	}
	return s.store.ListWorkflowVersions(ctx, organizationID, workflowID)
}

func (s *Service) UpdateWorkflow(ctx context.Context, req UpdateWorkflowRequest) (*WorkflowDefinition, error) {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.WorkflowID) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}

	existing, err := s.GetWorkflow(ctx, req.OrganizationID, req.WorkflowID)
	if err != nil {
		return nil, err
	}

	name := existing.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	description := existing.Description
	if req.Description != nil {
		description = strings.TrimSpace(*req.Description)
	}
	status := existing.Status
	if req.Status != nil {
		status = *req.Status
	}
	definition := existing.Definition
	if req.Definition != nil {
		definition = req.Definition
	}
	variables := existing.Variables
	if req.Variables != nil {
		variables = req.Variables
	}

	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: workflow name is required", ErrInvalidInput)
	}
	if err := validateWorkflowDefinition(definition); err != nil {
		return nil, err
	}
	if err := validateScheduleTriggersForStatus(status, definition); err != nil {
		return nil, err
	}

	updated, err := s.store.UpdateWorkflow(ctx, UpdateWorkflowStoreRequest{
		OrganizationID: req.OrganizationID,
		WorkflowID:     req.WorkflowID,
		Name:           name,
		Description:    description,
		Status:         status,
		Definition:     definition,
		Variables:      variables,
	})
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, fmt.Errorf("%w: workflow %s", ErrNotFound, req.WorkflowID)
	}
	if err := s.syncWorkflowScheduleTriggers(ctx, updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) RollbackWorkflow(ctx context.Context, req RollbackWorkflowRequest) (*WorkflowDefinition, error) {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.WorkflowID) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}
	if req.Version <= 0 {
		return nil, fmt.Errorf("%w: workflow version is required", ErrInvalidInput)
	}
	version, err := s.store.GetWorkflowVersion(ctx, req.OrganizationID, req.WorkflowID, req.Version)
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrNotFound, req.WorkflowID, req.Version)
	}
	return s.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: req.OrganizationID,
		WorkflowID:     req.WorkflowID,
		Name:           &version.Name,
		Description:    &version.Description,
		Status:         &version.Status,
		Definition:     version.Definition,
		Variables:      version.Variables,
	})
}

func (s *Service) CreateWorkflowBranch(ctx context.Context, req CreateWorkflowBranchRequest) (*WorkflowDefinition, error) {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.WorkflowID) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}
	if req.Version <= 0 {
		return nil, fmt.Errorf("%w: workflow version is required", ErrInvalidInput)
	}
	if req.TrafficPercent < 0 || req.TrafficPercent > 100 {
		return nil, fmt.Errorf("%w: branch traffic percent must be between 0 and 100", ErrInvalidInput)
	}

	source, err := s.store.GetWorkflowVersion(ctx, req.OrganizationID, req.WorkflowID, req.Version)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrNotFound, req.WorkflowID, req.Version)
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("%s branch", source.Name)
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = source.Description
	}

	return s.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: req.OrganizationID,
		Name:           name,
		Description:    description,
		Status:         WorkflowStatusDraft,
		Definition: workflowDefinitionWithBranchMetadata(source.Definition, workflowBranchMetadata{
			ExperimentKey:  strings.TrimSpace(req.ExperimentKey),
			SourceVersion:  source.Version,
			SourceWorkflow: source.ID,
			TrafficPercent: req.TrafficPercent,
		}),
		Variables: cloneWorkflowMap(source.Variables),
	})
}

func (s *Service) DeleteWorkflow(ctx context.Context, organizationID, workflowID string) (*WorkflowDefinition, error) {
	archived := WorkflowStatusArchived
	return s.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: organizationID,
		WorkflowID:     workflowID,
		Status:         &archived,
	})
}

func (s *Service) TestNode(ctx context.Context, req TestNodeRequest) (*TestNodeResult, error) {
	if strings.TrimSpace(req.NodeID) == "" {
		return nil, fmt.Errorf("%w: node ID is required", ErrInvalidInput)
	}
	workflow, err := s.GetWorkflow(ctx, req.OrganizationID, req.WorkflowID)
	if err != nil {
		return nil, err
	}
	nodeID := strings.TrimSpace(req.NodeID)
	if !definitionHasNodeID(workflow.Definition, nodeID) {
		return nil, fmt.Errorf("%w: workflow node %s", ErrNotFound, nodeID)
	}
	node, err := nodeDefinitionByID(workflow.Definition, nodeID)
	if err != nil {
		return nil, err
	}
	nodeType := node.Type
	if strings.TrimSpace(nodeType) == "" {
		nodeType = "start"
	}
	executor, ok := s.nodeExecutors.Get(nodeType)
	if !ok {
		return nil, fmt.Errorf("%w: executor for node type %s", ErrNodeExecutorNotFound, nodeType)
	}
	execution := &WorkflowExecution{
		ID:               "test_node",
		WorkflowID:       workflow.ID,
		WorkflowVersion:  workflow.Version,
		OrganizationID:   req.OrganizationID,
		Status:           ExecutionStatusRunning,
		Input:            req.Input,
		WorkflowSnapshot: workflow.Definition,
	}
	input, err := interpolateNodeInput(node.Input, workflow, execution)
	if err != nil {
		return nil, err
	}
	startedAt := time.Now().UTC()
	output, err := executor.Execute(ctx, NodeExecutorInput{
		Workflow:  workflow,
		Execution: execution,
		Node:      node,
		Input:     input,
	})
	durationMS := int(time.Since(startedAt) / time.Millisecond)
	if durationMS <= 0 {
		durationMS = 1
	}
	if err != nil {
		errorPayload := map[string]any{"message": err.Error()}
		return &TestNodeResult{
			WorkflowID: workflow.ID,
			NodeID:     nodeID,
			Status:     ExecutionStatusFailed,
			Input:      input,
			Output:     output,
			Error:      errorPayload,
			DurationMS: durationMS,
			Trace: []map[string]any{{
				"nodeId":     nodeID,
				"nodeType":   nodeType,
				"status":     string(ExecutionStatusFailed),
				"durationMs": durationMS,
				"error":      errorPayload,
			}},
		}, nil
	}
	return &TestNodeResult{
		WorkflowID: workflow.ID,
		NodeID:     nodeID,
		Status:     ExecutionStatusSucceeded,
		Input:      input,
		Output:     output,
		DurationMS: durationMS,
		Trace: []map[string]any{{
			"nodeId":     nodeID,
			"nodeType":   nodeType,
			"status":     string(ExecutionStatusSucceeded),
			"durationMs": durationMS,
		}},
	}, nil
}

func (s *Service) StartExecution(ctx context.Context, req StartExecutionRequest) (*WorkflowExecution, error) {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.WorkflowID) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}
	triggerType, err := normalizeWorkflowTriggerType(req.TriggerType)
	if err != nil {
		return nil, err
	}
	workflow, err := s.GetWorkflow(ctx, req.OrganizationID, req.WorkflowID)
	if err != nil {
		return nil, err
	}
	runtimeWorkflow, err := s.latestPublishedWorkflowVersion(ctx, workflow)
	if err != nil {
		return nil, err
	}
	startNodes, err := startNodeExecutionsForDefinition(runtimeWorkflow.Definition, req.Input)
	if err != nil {
		return nil, err
	}
	policy := concurrencyPolicyForTrigger(runtimeWorkflow, triggerType)
	if err := s.checkSystemWorkflowLimits(ctx, time.Now().UTC()); err != nil {
		return nil, err
	}
	reservedGlobalStart, err := s.reserveGlobalExecutionStart(time.Now().UTC())
	if err != nil {
		return nil, err
	}
	status := ExecutionStatusRunning
	if s.orgMaxConcurrentWorkflows > 0 {
		runningForOrg, err := s.store.CountRunningExecutionsForOrganization(ctx, req.OrganizationID)
		if err != nil {
			return nil, err
		}
		if runningForOrg >= s.orgMaxConcurrentWorkflows {
			status = ExecutionStatusQueued
		}
	}
	if status == ExecutionStatusRunning && policy.MaxConcurrentExecutions > 0 {
		runningForWorkflow, err := s.store.CountRunningExecutions(ctx, req.OrganizationID, req.WorkflowID)
		if err != nil {
			return nil, err
		}
		if runningForWorkflow >= policy.MaxConcurrentExecutions {
			if policy.ConcurrencyOverflow == workflowConcurrencyReject {
				return nil, fmt.Errorf("%w: workflow %s has %d running executions", ErrWorkflowConcurrencyLimit, req.WorkflowID, runningForWorkflow)
			}
			status = ExecutionStatusQueued
		}
	}
	execution, err := s.store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID:   req.OrganizationID,
		WorkflowID:       req.WorkflowID,
		WorkflowVersion:  runtimeWorkflow.Version,
		Status:           status,
		Input:            req.Input,
		Context:          executionContextWithTrigger(req.Context, triggerType, req.TriggerPayload),
		WorkflowSnapshot: runtimeWorkflow.Definition,
		NodeExecutions:   startNodes,
	})
	if err != nil {
		if reservedGlobalStart {
			s.releaseGlobalExecutionStart()
		}
		return nil, err
	}
	if execution == nil {
		if reservedGlobalStart {
			s.releaseGlobalExecutionStart()
		}
		return nil, fmt.Errorf("%w: workflow %s", ErrNotFound, req.WorkflowID)
	}
	return execution, nil
}

func (s *Service) checkSystemWorkflowLimits(ctx context.Context, now time.Time) error {
	if s == nil {
		return nil
	}
	if s.systemLimits.MaxConcurrentWorkflows > 0 {
		summaries, err := s.store.ListActiveExecutionHealth(ctx, "", []ExecutionStatus{ExecutionStatusRunning})
		if err != nil {
			return err
		}
		running := 0
		for _, summary := range summaries {
			running += summary.Count
		}
		if running >= s.systemLimits.MaxConcurrentWorkflows {
			return fmt.Errorf("%w: system has %d running workflow executions", ErrWorkflowConcurrencyLimit, running)
		}
	}
	return nil
}

func (s *Service) reserveGlobalExecutionStart(now time.Time) (bool, error) {
	if s == nil || s.systemLimits.MaxExecutionsPerMinute <= 0 {
		return false, nil
	}
	s.systemLimitMu.Lock()
	defer s.systemLimitMu.Unlock()
	s.pruneSystemLimitWindowLocked(now)
	if len(s.systemLimitWindow) >= s.systemLimits.MaxExecutionsPerMinute {
		return false, fmt.Errorf("%w: system exceeded %d workflow executions per minute", ErrWorkflowConcurrencyLimit, s.systemLimits.MaxExecutionsPerMinute)
	}
	s.systemLimitWindow = append(s.systemLimitWindow, now)
	return true, nil
}

func (s *Service) releaseGlobalExecutionStart() {
	if s == nil || s.systemLimits.MaxExecutionsPerMinute <= 0 {
		return
	}
	s.systemLimitMu.Lock()
	defer s.systemLimitMu.Unlock()
	if len(s.systemLimitWindow) > 0 {
		s.systemLimitWindow = s.systemLimitWindow[:len(s.systemLimitWindow)-1]
	}
}

func (s *Service) pruneSystemLimitWindowLocked(now time.Time) {
	cutoff := now.Add(-time.Minute)
	window := s.systemLimitWindow[:0]
	for _, startedAt := range s.systemLimitWindow {
		if startedAt.After(cutoff) {
			window = append(window, startedAt)
		}
	}
	s.systemLimitWindow = window
}

func (s *Service) MatchSemanticTriggers(ctx context.Context, req MatchSemanticTriggersRequest) ([]SemanticTriggerMatch, error) {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return []SemanticTriggerMatch{}, nil
	}
	workflows, err := s.ListWorkflows(ctx, req.OrganizationID)
	if err != nil {
		return nil, err
	}

	matches := []SemanticTriggerMatch{}
	for _, workflow := range workflows {
		if workflow == nil || workflow.Status == WorkflowStatusArchived {
			continue
		}
		runtimeWorkflow, err := s.latestPublishedWorkflowVersion(ctx, workflow)
		if err != nil {
			return nil, err
		}
		if runtimeWorkflow == nil || runtimeWorkflow.Status != WorkflowStatusPublished {
			continue
		}
		for _, trigger := range semanticTriggersFromDefinition(runtimeWorkflow.Definition) {
			decision, ok, err := s.matchSemanticTrigger(ctx, req, message, trigger)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			matches = append(matches, SemanticTriggerMatch{
				WorkflowID:         runtimeWorkflow.ID,
				WorkflowVersion:    runtimeWorkflow.Version,
				WorkflowName:       runtimeWorkflow.Name,
				TriggerID:          trigger.ID,
				Keyword:            decision.Keyword,
				SemanticThreshold:  trigger.SemanticThreshold,
				Score:              decision.Score,
				MatchMethod:        decision.MatchMethod,
				TriggerDefinition:  trigger.Definition,
				WorkflowDefinition: runtimeWorkflow.Definition,
			})
		}
	}
	return matches, nil
}

func (s *Service) matchSemanticTrigger(ctx context.Context, req MatchSemanticTriggersRequest, message string, trigger semanticTriggerDefinition) (SemanticTriggerMatchDecision, bool, error) {
	if keyword, ok := firstMatchingSemanticKeyword(message, trigger.Keywords); ok {
		return SemanticTriggerMatchDecision{
			Matched:     true,
			Keyword:     keyword,
			Score:       1,
			MatchMethod: "keyword",
		}, true, nil
	}
	if trigger.SemanticThreshold <= 0 || s.semanticTriggerMatcher == nil {
		return SemanticTriggerMatchDecision{}, false, nil
	}
	decision, err := s.semanticTriggerMatcher.MatchSemanticTrigger(ctx, SemanticTriggerMatchRequest{
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		Message:        message,
		TriggerID:      trigger.ID,
		Keywords:       append([]string(nil), trigger.Keywords...),
		Threshold:      trigger.SemanticThreshold,
		Definition:     trigger.Definition,
	})
	if err != nil {
		return SemanticTriggerMatchDecision{}, false, err
	}
	if !decision.Matched {
		return SemanticTriggerMatchDecision{}, false, nil
	}
	if strings.TrimSpace(decision.Keyword) == "" {
		decision.Keyword = firstSemanticTriggerKeyword(trigger.Keywords)
	}
	if strings.TrimSpace(decision.MatchMethod) == "" {
		decision.MatchMethod = "embedding"
	}
	return decision, true, nil
}

func (s *Service) MatchConversationTriggers(ctx context.Context, req MatchConversationTriggersRequest) ([]ConversationTriggerMatch, error) {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		return []ConversationTriggerMatch{}, nil
	}
	workflows, err := s.ListWorkflows(ctx, req.OrganizationID)
	if err != nil {
		return nil, err
	}

	matches := []ConversationTriggerMatch{}
	for _, workflow := range workflows {
		if workflow == nil || workflow.Status == WorkflowStatusArchived {
			continue
		}
		runtimeWorkflow, err := s.latestPublishedWorkflowVersion(ctx, workflow)
		if err != nil {
			return nil, err
		}
		if runtimeWorkflow == nil || runtimeWorkflow.Status != WorkflowStatusPublished {
			continue
		}
		for _, trigger := range conversationTriggersFromDefinition(runtimeWorkflow.Definition) {
			if !strings.EqualFold(trigger.ConversationID, conversationID) {
				continue
			}
			matches = append(matches, ConversationTriggerMatch{
				WorkflowID:         runtimeWorkflow.ID,
				WorkflowVersion:    runtimeWorkflow.Version,
				WorkflowName:       runtimeWorkflow.Name,
				TriggerID:          trigger.ID,
				ConversationID:     trigger.ConversationID,
				TriggerDefinition:  trigger.Definition,
				WorkflowDefinition: runtimeWorkflow.Definition,
			})
		}
	}
	return matches, nil
}

func (s *Service) latestPublishedWorkflowVersion(ctx context.Context, workflow *WorkflowDefinition) (*WorkflowDefinition, error) {
	versions, err := s.store.ListWorkflowVersions(ctx, workflow.OrganizationID, workflow.ID)
	if err != nil {
		return nil, err
	}
	var latest *WorkflowDefinition
	for _, version := range versions {
		if version.Status != WorkflowStatusPublished {
			continue
		}
		if latest == nil || version.Version > latest.Version {
			latest = version
		}
	}
	if latest != nil {
		return latest, nil
	}
	return workflow, nil
}

func normalizeWorkflowTriggerType(triggerType WorkflowTriggerType) (WorkflowTriggerType, error) {
	if triggerType == "" {
		return WorkflowTriggerManual, nil
	}
	switch triggerType {
	case WorkflowTriggerManual, WorkflowTriggerConversation, WorkflowTriggerSchedule, WorkflowTriggerWebhook, WorkflowTriggerSemantic:
		return triggerType, nil
	default:
		return "", fmt.Errorf("%w: unsupported workflow trigger type %s", ErrInvalidInput, triggerType)
	}
}

func executionContextWithTrigger(contextValue map[string]any, triggerType WorkflowTriggerType, triggerPayload map[string]any) map[string]any {
	next := mergeWorkflowMaps(contextValue, nil)
	trigger := mergeWorkflowMaps(triggerPayload, nil)
	trigger["type"] = string(triggerType)
	next["trigger"] = trigger
	return next
}

func (s *Service) ListExecutions(ctx context.Context, organizationID, workflowID string) ([]*WorkflowExecution, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(workflowID) == "" {
		return nil, fmt.Errorf("%w: workflow ID is required", ErrInvalidInput)
	}
	return s.store.ListExecutions(ctx, organizationID, workflowID)
}

func (s *Service) RefreshExecutionHealthMetrics(ctx context.Context, organizationID string, now time.Time) error {
	if s == nil || s.store == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	summaries, err := s.store.ListActiveExecutionHealth(ctx, strings.TrimSpace(organizationID), activeExecutionHealthStatuses())
	if err != nil {
		return err
	}
	healthByStatus := map[string]metrics.WorkflowExecutionActiveHealth{}
	for _, summary := range summaries {
		if summary.Count <= 0 {
			continue
		}
		ageSeconds := 0.0
		if !summary.OldestStartedAt.IsZero() {
			ageSeconds = now.Sub(summary.OldestStartedAt).Seconds()
		}
		healthByStatus[string(summary.Status)] = metrics.WorkflowExecutionActiveHealth{
			Count:            summary.Count,
			OldestAgeSeconds: ageSeconds,
		}
	}
	metrics.SetWorkflowExecutionActiveHealth(healthByStatus)
	return nil
}

func (s *Service) PromoteQueuedExecutions(ctx context.Context, organizationID string) ([]*WorkflowExecution, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	queued, err := s.queuedExecutionsForOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(queued, func(i, j int) bool {
		if queued[i].CreatedAt.Equal(queued[j].CreatedAt) {
			return queued[i].ID < queued[j].ID
		}
		return queued[i].CreatedAt.Before(queued[j].CreatedAt)
	})

	promoted := []*WorkflowExecution{}
	for _, execution := range queued {
		canPromote, err := s.canPromoteQueuedExecution(ctx, execution)
		if err != nil {
			return promoted, err
		}
		if !canPromote {
			continue
		}
		updated, err := s.store.UpdateExecutionStatus(ctx, organizationID, execution.ID, ExecutionStatusRunning, nil)
		if err != nil {
			return promoted, err
		}
		if updated == nil {
			continue
		}
		promoted = append(promoted, updated)
	}
	return promoted, nil
}

func (s *Service) GetExecution(ctx context.Context, organizationID, executionID string) (*WorkflowExecution, error) {
	return s.getExecutionForTransition(ctx, organizationID, executionID)
}

func (s *Service) BuildExecutionDebugSnapshot(ctx context.Context, organizationID, executionID string) (*ExecutionDebugSnapshot, error) {
	execution, err := s.GetExecution(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	trace := make([]ExecutionDebugTraceEntry, 0, len(execution.NodeExecutions))
	outputs := map[string]map[string]any{}
	nodeDurations := map[string]int{}
	logs := buildExecutionDebugLogs(execution.NodeExecutions)
	bottleneckNodeID := ""
	bottleneckDuration := 0

	for _, node := range execution.NodeExecutions {
		trace = append(trace, ExecutionDebugTraceEntry{
			NodeID:      node.NodeID,
			NodeType:    node.NodeType,
			Status:      node.Status,
			Attempt:     node.Attempt,
			Input:       mergeWorkflowMaps(node.Input, nil),
			Output:      mergeWorkflowMaps(node.Output, nil),
			Error:       mergeWorkflowMaps(node.Error, nil),
			Context:     mergeWorkflowMaps(node.Context, nil),
			StartedAt:   node.StartedAt,
			CompletedAt: node.CompletedAt,
			DurationMS:  node.DurationMS,
		})
		if isSuccessfulNodeStatus(node.Status) {
			outputs[node.NodeID] = mergeWorkflowMaps(node.Output, nil)
		}
		if node.DurationMS > 0 {
			nodeDurations[node.NodeID] += node.DurationMS
			if node.DurationMS > bottleneckDuration {
				bottleneckDuration = node.DurationMS
				bottleneckNodeID = node.NodeID
			}
		}
	}

	totalDuration := execution.DurationMS
	if totalDuration <= 0 && execution.CompletedAt != nil && !execution.StartedAt.IsZero() {
		totalDuration = int(execution.CompletedAt.Sub(execution.StartedAt).Milliseconds())
	}
	if totalDuration < 0 {
		totalDuration = 0
	}

	return &ExecutionDebugSnapshot{
		ExecutionID: execution.ID,
		WorkflowID:  execution.WorkflowID,
		Status:      execution.Status,
		VariableSnapshot: ExecutionVariableSnapshot{
			Input:       mergeWorkflowMaps(execution.Input, nil),
			Context:     mergeWorkflowMaps(execution.Context, nil),
			NodeOutputs: outputs,
		},
		Trace:   trace,
		Outputs: outputs,
		Performance: ExecutionDebugPerformance{
			TotalDurationMS:  totalDuration,
			NodeDurationsMS:  nodeDurations,
			BottleneckNodeID: bottleneckNodeID,
		},
		Logs: logs,
	}, nil
}

func buildExecutionDebugLogs(nodeExecutions []WorkflowNodeExecution) []ExecutionDebugLogEntry {
	logs := make([]ExecutionDebugLogEntry, 0, len(nodeExecutions))
	for _, node := range nodeExecutions {
		logs = append(logs, ExecutionDebugLogEntry{
			Level:     debugLogLevelForNodeStatus(node.Status),
			Message:   debugLogMessage(node),
			Timestamp: debugLogTimestamp(node),
			NodeID:    node.NodeID,
		})
	}
	return logs
}

func debugLogLevelForNodeStatus(status NodeStatus) string {
	switch status {
	case NodeStatusFailed:
		return "error"
	case NodeStatusRetrying:
		return "warning"
	default:
		return "info"
	}
}

func debugLogTimestamp(node WorkflowNodeExecution) time.Time {
	if node.CompletedAt != nil {
		return *node.CompletedAt
	}
	if !node.StartedAt.IsZero() {
		return node.StartedAt
	}
	return node.CreatedAt
}

func debugLogMessage(node WorkflowNodeExecution) string {
	nodeID := strings.TrimSpace(node.NodeID)
	if nodeID == "" {
		nodeID = "node"
	}
	message := fmt.Sprintf("Node %s %s", nodeID, node.Status)
	if node.DurationMS > 0 {
		message = fmt.Sprintf("%s in %dms", message, node.DurationMS)
	}
	if node.Status == NodeStatusFailed {
		if failureMessage := debugNodeErrorMessage(node.Error); failureMessage != "" {
			message = fmt.Sprintf("%s: %s", message, failureMessage)
		}
	}
	return message
}

func debugNodeErrorMessage(errorPayload map[string]any) string {
	for _, key := range []string{"message", "error", "reason"} {
		if value, ok := errorPayload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *Service) RunExecutionUntilBlocked(ctx context.Context, organizationID, executionID string) (*WorkflowExecution, error) {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if !isRunnableExecutionStatus(execution.Status) {
		return execution, nil
	}

	policy := resourcePolicyForWorkflow(workflowDefinitionForExecutionPolicy(execution))
	maxNodeExecutions := policy.MaxNodeExecutions
	if maxNodeExecutions <= 0 {
		maxNodeExecutions = defaultWorkflowMaxNodeExecutions
	}

	var lastErr error
	for executedNodes := 0; ; executedNodes++ {
		if !isRunnableExecutionStatus(execution.Status) {
			return execution, lastErr
		}

		nodeID := nextRunnableWorkflowNodeID(execution.NodeExecutions, time.Now().UTC())
		if nodeID == "" {
			if lastErr != nil {
				return execution, lastErr
			}
			if latestWorkflowNodesComplete(execution.NodeExecutions) {
				now := time.Now().UTC()
				updated, updateErr := s.setExecutionStatus(ctx, organizationID, executionID, completedWorkflowExecutionStatus(execution.NodeExecutions), &now)
				if updateErr != nil {
					return nil, updateErr
				}
				refreshed, refreshErr := s.getExecutionForTransition(ctx, organizationID, executionID)
				if refreshErr == nil && refreshed != nil {
					updated = refreshed
				}
				return updated, nil
			}
			return execution, nil
		}
		if executedNodes >= maxNodeExecutions {
			now := time.Now().UTC()
			updated, updateErr := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusMaxIterations, &now)
			if updateErr != nil {
				return nil, updateErr
			}
			refreshed, refreshErr := s.getExecutionForTransition(ctx, organizationID, executionID)
			if refreshErr == nil && refreshed != nil {
				updated = refreshed
			}
			return updated, fmt.Errorf("%w: execution exceeded node execution limit", ErrWorkflowResourceLimit)
		}

		beforeNodeCount := len(execution.NodeExecutions)
		runErr := s.RunReadyNode(ctx, organizationID, execution.ID, nodeID)
		updated, getErr := s.getExecutionForTransition(ctx, organizationID, executionID)
		if getErr != nil {
			return nil, getErr
		}
		execution = updated
		if runErr == nil && isRunnableExecutionStatus(execution.Status) {
			limited, limitErr := s.CheckResourceLimits(ctx, organizationID, executionID, resourceUsageForExecution(execution))
			if limited != nil {
				execution = limited
			}
			if limitErr != nil {
				return execution, limitErr
			}
		}
		if runErr != nil {
			if latestNode, ok := latestWorkflowNodeExecutionsByID(execution.NodeExecutions)[nodeID]; ok && latestNode.Status == NodeStatusRetrying {
				lastErr = nil
				continue
			} else if ok && latestNode.Status == NodeStatusSkipped {
				lastErr = nil
				continue
			}
			if !isRunnableExecutionStatus(execution.Status) || len(execution.NodeExecutions) <= beforeNodeCount {
				return execution, runErr
			}
			lastErr = runErr
		}
	}
}

func (s *Service) CheckResourceLimits(ctx context.Context, organizationID, executionID string, usage WorkflowResourceUsage) (*WorkflowExecution, error) {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if execution.Status != ExecutionStatusRunning {
		return execution, nil
	}
	policy := resourcePolicyForWorkflow(workflowDefinitionForExecutionPolicy(execution))
	now := usage.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	if policy.MaxExecutionDuration > 0 && !execution.StartedAt.IsZero() && now.Sub(execution.StartedAt) > policy.MaxExecutionDuration {
		updated, updateErr := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusTimedOut, &now)
		if updateErr != nil {
			return nil, updateErr
		}
		return updated, fmt.Errorf("%w: execution exceeded max duration", ErrWorkflowResourceLimit)
	}
	if policy.MaxTokensBudget > 0 && usage.TotalTokens > policy.MaxTokensBudget {
		_, nodeErr := s.store.CreateNodeExecution(ctx, organizationID, executionID, CreateNodeExecutionRequest{
			NodeID:   "workflow_resource_guard",
			NodeType: "resource_guard",
			Status:   NodeStatusFailed,
			Attempt:  1,
			Error: map[string]any{
				"code":            "token_budget_exceeded",
				"message":         "workflow token budget exceeded",
				"maxTokensBudget": policy.MaxTokensBudget,
				"totalTokens":     usage.TotalTokens,
			},
			Context: map[string]any{
				"pauseReason": "token_budget_exceeded",
			},
			StartedAt:   now,
			CompletedAt: &now,
		})
		if nodeErr != nil {
			return nil, nodeErr
		}
		updated, updateErr := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusPaused, nil)
		if updateErr != nil {
			return nil, updateErr
		}
		return updated, fmt.Errorf("%w: execution exceeded token budget", ErrWorkflowResourceLimit)
	}
	if policy.MaxNodeExecutions > 0 && usage.NodeExecutionCount > policy.MaxNodeExecutions {
		updated, updateErr := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusMaxIterations, &now)
		if updateErr != nil {
			return nil, updateErr
		}
		return updated, fmt.Errorf("%w: execution exceeded node execution limit", ErrWorkflowResourceLimit)
	}
	return execution, nil
}

func (s *Service) PauseExecution(ctx context.Context, organizationID, executionID string) (*WorkflowExecution, error) {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if execution.Status != ExecutionStatusRunning {
		return nil, fmt.Errorf("%w: pause requires running execution, got %s", ErrInvalidTransition, execution.Status)
	}
	return s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusPaused, nil)
}

func (s *Service) ResumeExecution(ctx context.Context, organizationID, executionID string, requests ...ResumeExecutionRequest) (*WorkflowExecution, error) {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if execution.Status != ExecutionStatusPaused {
		return nil, fmt.Errorf("%w: resume requires paused execution, got %s", ErrInvalidTransition, execution.Status)
	}
	if len(requests) > 0 {
		req := requests[0]
		if req.Input != nil {
			if err := s.completePausedResumeNode(ctx, organizationID, execution, req); err != nil {
				return nil, err
			}
		}
	}
	updated, err := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusRunning, nil)
	if err != nil {
		return nil, err
	}
	if refreshed, refreshErr := s.getExecutionForTransition(ctx, organizationID, executionID); refreshErr == nil && refreshed != nil {
		updated = refreshed
	}
	return updated, nil
}

func (s *Service) completePausedResumeNode(ctx context.Context, organizationID string, execution *WorkflowExecution, req ResumeExecutionRequest) error {
	node, ok := latestPendingResumeWorkflowNode(execution.NodeExecutions, strings.TrimSpace(req.NodeID))
	if !ok {
		return fmt.Errorf("%w: pending resume node is required", ErrInvalidTransition)
	}
	waitReason := strings.TrimSpace(stringFromWorkflowValue(node.Context["waitReason"]))
	resumeDecision := "input_submitted"
	switch waitReason {
	case "agent_approval_required":
		if _, hasStatus := req.Input["status"]; !hasStatus {
			output, err := s.approvePausedAgentToolRun(ctx, execution, node, req.Input)
			if err != nil {
				return err
			}
			req.Input = output
		} else if err := validateAgentApprovalSubmission(req.Input); err != nil {
			return err
		}
		resumeDecision = "agent_approval_completed"
	case "approval_required", "user_input_required":
		if err := validateUserInputSubmission(node.Input, req.Input); err != nil {
			return err
		}
		resumeDecision = "user_input_submitted"
	default:
		return fmt.Errorf("%w: pending resume node is required", ErrInvalidTransition)
	}
	now := time.Now().UTC()
	_, err := s.RecordNodeStatus(ctx, organizationID, execution.ID, RecordNodeStatusRequest{
		NodeID:      node.NodeID,
		NodeType:    node.NodeType,
		Status:      NodeStatusSucceeded,
		Attempt:     node.Attempt,
		Input:       mergeWorkflowMaps(node.Input, nil),
		Output:      mergeWorkflowMaps(req.Input, nil),
		StartedAt:   now,
		CompletedAt: &now,
		Context:     map[string]any{"resumeDecision": resumeDecision},
	})
	return err
}

func (s *Service) approvePausedAgentToolRun(ctx context.Context, execution *WorkflowExecution, node WorkflowNodeExecution, submitted map[string]any) (map[string]any, error) {
	if s == nil || s.nodeExecutors == nil {
		return nil, fmt.Errorf("%w: agent node executor is required", ErrInvalidInput)
	}
	executor, ok := s.nodeExecutors.Get("agent")
	if !ok {
		return nil, fmt.Errorf("%w: executor for node type agent", ErrNodeExecutorNotFound)
	}
	approvalExecutor, ok := executor.(agentApprovalNodeExecutor)
	if !ok {
		return nil, fmt.Errorf("%w: agent node approval executor is required", ErrInvalidInput)
	}
	output, err := approvalExecutor.ApproveToolRun(ctx, NodeExecutorInput{
		Execution: execution,
		Input:     node.Input,
	}, node, submitted)
	if err != nil {
		return nil, err
	}
	if err := validateAgentApprovalSubmission(output); err != nil {
		return nil, err
	}
	return output, nil
}

func (s *Service) completePausedUserInputNode(ctx context.Context, organizationID string, execution *WorkflowExecution, req ResumeExecutionRequest) error {
	node, ok := latestPendingUserInputWorkflowNode(execution.NodeExecutions, strings.TrimSpace(req.NodeID))
	if !ok {
		return fmt.Errorf("%w: pending user input node is required", ErrInvalidTransition)
	}
	if err := validateUserInputSubmission(node.Input, req.Input); err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err := s.RecordNodeStatus(ctx, organizationID, execution.ID, RecordNodeStatusRequest{
		NodeID:      node.NodeID,
		NodeType:    node.NodeType,
		Status:      NodeStatusSucceeded,
		Attempt:     node.Attempt,
		Input:       mergeWorkflowMaps(node.Input, nil),
		Output:      mergeWorkflowMaps(req.Input, nil),
		StartedAt:   now,
		CompletedAt: &now,
		Context:     map[string]any{"resumeDecision": "user_input_submitted"},
	})
	return err
}

func validateUserInputSubmission(nodeInput map[string]any, submitted map[string]any) error {
	for _, field := range userInputRequiredFields(nodeInput) {
		value, ok := submitted[field]
		if !ok {
			return fmt.Errorf("%w: user input field %s is required", ErrInvalidInput, field)
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return fmt.Errorf("%w: user input field %s is required", ErrInvalidInput, field)
		}
	}
	return nil
}

func validateAgentApprovalSubmission(submitted map[string]any) error {
	status := strings.TrimSpace(stringFromWorkflowValue(submitted["status"]))
	if status == "" {
		return fmt.Errorf("%w: agent run status is required", ErrInvalidInput)
	}
	if strings.EqualFold(status, "pending_approval") {
		return fmt.Errorf("%w: agent run is still pending approval", ErrInvalidInput)
	}
	return nil
}

func (s *Service) ResolvePausedFailure(ctx context.Context, organizationID, executionID string, req ResolveFailureDecisionRequest) (*WorkflowExecution, error) {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if execution.Status != ExecutionStatusPaused {
		return nil, fmt.Errorf("%w: failure decision requires paused execution, got %s", ErrInvalidTransition, execution.Status)
	}

	nodeID := strings.TrimSpace(req.NodeID)
	if nodeID == "" {
		nodeID = latestFailedWorkflowNodeID(execution.NodeExecutions)
	}
	if nodeID == "" {
		return nil, fmt.Errorf("%w: failed node ID is required", ErrInvalidInput)
	}
	failedNode, ok := latestWorkflowNodeExecutionsByID(execution.NodeExecutions)[nodeID]
	if !ok || failedNode.Status != NodeStatusFailed {
		return nil, fmt.Errorf("%w: node %s is not the latest failed node", ErrInvalidTransition, nodeID)
	}

	switch req.Action {
	case FailureActionRetry:
		nodeInput := failedNode.Input
		if req.Input != nil {
			nodeInput = req.Input
		}
		if _, err := s.store.CreateNodeExecution(ctx, organizationID, executionID, CreateNodeExecutionRequest{
			NodeID:   failedNode.NodeID,
			NodeType: failedNode.NodeType,
			Status:   NodeStatusPending,
			Attempt:  failedNode.Attempt + 1,
			Input:    nodeInput,
			Context:  map[string]any{"failureDecision": "retry"},
		}); err != nil {
			return nil, err
		}
		if _, err := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusRunning, nil); err != nil {
			return nil, err
		}
	case FailureActionContinue:
		if _, err := s.store.CreateNodeExecution(ctx, organizationID, executionID, CreateNodeExecutionRequest{
			NodeID:   failedNode.NodeID,
			NodeType: failedNode.NodeType,
			Status:   NodeStatusSkipped,
			Attempt:  failedNode.Attempt,
			Input:    failedNode.Input,
			Error:    failedNode.Error,
			Context:  map[string]any{"failureDecision": "skip"},
		}); err != nil {
			return nil, err
		}
		if _, err := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusRunning, nil); err != nil {
			return nil, err
		}
		if err := s.seedReadyDownstreamNodes(ctx, organizationID, executionID, failedNode.NodeID); err != nil {
			return nil, err
		}
	case FailureActionFail:
		now := time.Now().UTC()
		if _, err := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusFailed, &now); err != nil {
			return nil, err
		}
	case FailureActionBranch:
		nextNodeID := strings.TrimSpace(req.NextNodeID)
		if nextNodeID == "" {
			return nil, fmt.Errorf("%w: next node ID is required for branch decision", ErrInvalidInput)
		}
		if err := s.seedFailureBranchNode(ctx, organizationID, execution, nextNodeID, failedNode.NodeID, failureMessage(failedNode.Error)); err != nil {
			return nil, err
		}
		if _, err := s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusRunning, nil); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("%w: unsupported failure decision action %q", ErrInvalidInput, req.Action)
	}

	return s.getExecutionForTransition(ctx, organizationID, executionID)
}

func (s *Service) CancelExecution(ctx context.Context, organizationID, executionID string) (*WorkflowExecution, error) {
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if isTerminalExecutionStatus(execution.Status) {
		return nil, fmt.Errorf("%w: cancel cannot be applied to terminal execution %s", ErrInvalidTransition, execution.Status)
	}
	now := time.Now().UTC()
	return s.setExecutionStatus(ctx, organizationID, executionID, ExecutionStatusCancelled, &now)
}

func (s *Service) RecordNodeStatus(ctx context.Context, organizationID, executionID string, req RecordNodeStatusRequest) (*WorkflowNodeExecution, error) {
	if strings.TrimSpace(req.NodeID) == "" {
		return nil, fmt.Errorf("%w: node ID is required", ErrInvalidInput)
	}
	if req.Status == "" {
		return nil, fmt.Errorf("%w: node status is required", ErrInvalidInput)
	}
	execution, err := s.getExecutionForTransition(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if req.Status == NodeStatusFailed {
		return s.recordFailedNodeStatus(ctx, organizationID, execution, req)
	}
	node, err := s.store.CreateNodeExecution(ctx, organizationID, executionID, CreateNodeExecutionRequest{
		NodeID:      strings.TrimSpace(req.NodeID),
		NodeType:    req.NodeType,
		Status:      req.Status,
		Attempt:     req.Attempt,
		Input:       req.Input,
		Output:      req.Output,
		Error:       req.Error,
		Context:     req.Context,
		StartedAt:   req.StartedAt,
		CompletedAt: req.CompletedAt,
		DurationMS:  req.DurationMS,
	})
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("%w: execution %s", ErrNotFound, executionID)
	}
	if req.Status == NodeStatusFailed || isSuccessfulNodeStatus(req.Status) {
		metrics.RecordWorkflowNodeExecutionResult(req.NodeType, req.Status == NodeStatusFailed)
	}
	if isSuccessfulNodeStatus(req.Status) {
		if err := s.seedReadyDownstreamNodes(ctx, organizationID, executionID, strings.TrimSpace(req.NodeID)); err != nil {
			return nil, err
		}
	}
	return node, nil
}

func (s *Service) recordFailedNodeStatus(ctx context.Context, organizationID string, execution *WorkflowExecution, req RecordNodeStatusRequest) (*WorkflowNodeExecution, error) {
	nodeID := strings.TrimSpace(req.NodeID)
	nodeDefinition, err := s.nodeDefinitionForExecution(ctx, organizationID, execution, nodeID)
	if err != nil {
		return nil, err
	}
	policy := normalizeFailurePolicy(nodeDefinition.FailurePolicy)
	message := failureMessage(req.Error)
	now := time.Now().UTC()
	metrics.RecordWorkflowNodeExecutionResult(req.NodeType, true)

	switch policy.Strategy {
	case FailureStrategyAutoRetry:
		attempt := req.Attempt
		if attempt <= 0 {
			attempt = latestNodeAttempt(execution.NodeExecutions, nodeID)
		}
		if attempt <= 0 {
			attempt = 1
		}
		if attempt > policy.MaxRetries {
			node, err := s.createPolicyNodeExecution(ctx, organizationID, execution.ID, req, NodeStatusFailed, attempt, map[string]any{"message": message}, nil)
			if err != nil {
				return nil, err
			}
			if _, err := s.setExecutionStatus(ctx, organizationID, execution.ID, ExecutionStatusFailed, &now); err != nil {
				return nil, err
			}
			return node, nil
		}
		retryAt := now.Add(retryDelayForAttempt(policy, attempt))
		node, err := s.createPolicyNodeExecution(ctx, organizationID, execution.ID, req, NodeStatusRetrying, attempt+1, map[string]any{"message": message}, map[string]any{
			"retryAt": retryAt.Format(time.RFC3339),
		})
		if err != nil {
			return nil, err
		}
		if execution.Status != ExecutionStatusRunning {
			if _, err := s.setExecutionStatus(ctx, organizationID, execution.ID, ExecutionStatusRunning, nil); err != nil {
				return nil, err
			}
		}
		return node, nil
	case FailureStrategySkipOnFailure:
		node, err := s.createPolicyNodeExecution(ctx, organizationID, execution.ID, req, NodeStatusSkipped, req.Attempt, map[string]any{"message": message}, nil)
		if err != nil {
			return nil, err
		}
		if _, err := s.setExecutionStatus(ctx, organizationID, execution.ID, ExecutionStatusPartialSuccess, nil); err != nil {
			return nil, err
		}
		if err := s.seedReadyDownstreamNodes(ctx, organizationID, execution.ID, nodeID); err != nil {
			return nil, err
		}
		return node, nil
	case FailureStrategyFailureBranch:
		node, err := s.createPolicyNodeExecution(ctx, organizationID, execution.ID, req, NodeStatusFailed, req.Attempt, map[string]any{"message": message}, nil)
		if err != nil {
			return nil, err
		}
		if policy.FailureBranchNodeID != "" {
			if err := s.seedFailureBranchNode(ctx, organizationID, execution, policy.FailureBranchNodeID, nodeID, message); err != nil {
				return nil, err
			}
		}
		return node, nil
	default:
		node, err := s.createPolicyNodeExecution(ctx, organizationID, execution.ID, req, NodeStatusFailed, req.Attempt, map[string]any{"message": message}, nil)
		if err != nil {
			return nil, err
		}
		if _, err := s.setExecutionStatus(ctx, organizationID, execution.ID, ExecutionStatusPaused, nil); err != nil {
			return nil, err
		}
		return node, nil
	}
}

func (s *Service) createPolicyNodeExecution(ctx context.Context, organizationID, executionID string, req RecordNodeStatusRequest, status NodeStatus, attempt int, errorValue map[string]any, contextValue map[string]any) (*WorkflowNodeExecution, error) {
	if attempt <= 0 {
		attempt = 1
	}
	if req.Error != nil && errorValue == nil {
		errorValue = req.Error
	}
	if req.Context != nil {
		contextValue = mergeWorkflowMaps(req.Context, contextValue)
	}
	return s.store.CreateNodeExecution(ctx, organizationID, executionID, CreateNodeExecutionRequest{
		NodeID:      strings.TrimSpace(req.NodeID),
		NodeType:    req.NodeType,
		Status:      status,
		Attempt:     attempt,
		Input:       req.Input,
		Output:      req.Output,
		Error:       errorValue,
		Context:     contextValue,
		StartedAt:   req.StartedAt,
		CompletedAt: req.CompletedAt,
		DurationMS:  req.DurationMS,
	})
}

func (s *Service) nodeDefinitionForExecution(ctx context.Context, organizationID string, execution *WorkflowExecution, nodeID string) (Node, error) {
	definition := execution.WorkflowSnapshot
	if len(definition) == 0 {
		workflow, err := s.GetWorkflow(ctx, organizationID, execution.WorkflowID)
		if err != nil {
			return Node{}, err
		}
		definition = workflow.Definition
	}
	nodes, err := definitionNodes(definition)
	if err != nil {
		return Node{}, err
	}
	for _, node := range nodes {
		if node.ID == nodeID {
			return node, nil
		}
	}
	return Node{}, fmt.Errorf("%w: workflow node %s", ErrNotFound, nodeID)
}

func (s *Service) seedFailureBranchNode(ctx context.Context, organizationID string, execution *WorkflowExecution, branchNodeID, failedNodeID, message string) error {
	branchNode, err := s.nodeDefinitionForExecution(ctx, organizationID, execution, branchNodeID)
	if err != nil {
		return err
	}
	if existingNodeIDs(execution.NodeExecutions)[branchNodeID] {
		return nil
	}
	errorContext := map[string]any{
		"workflow.error": map[string]any{
			"nodeId":  failedNodeID,
			"message": message,
		},
	}
	_, err = s.store.CreateNodeExecution(ctx, organizationID, execution.ID, CreateNodeExecutionRequest{
		NodeID:   branchNode.ID,
		NodeType: branchNode.Type,
		Status:   NodeStatusPending,
		Attempt:  1,
		Input:    mergeWorkflowMaps(execution.Input, errorContext),
		Context:  errorContext,
	})
	return err
}

func failureMessage(errorValue map[string]any) string {
	for _, key := range []string{"message", "error", "reason"} {
		if value, ok := errorValue[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if len(errorValue) == 0 {
		return "node execution failed"
	}
	return fmt.Sprint(errorValue)
}

func latestNodeAttempt(nodes []WorkflowNodeExecution, nodeID string) int {
	attempt := 0
	for _, node := range nodes {
		if node.NodeID == nodeID && node.Attempt > attempt {
			attempt = node.Attempt
		}
	}
	return attempt
}

func mergeWorkflowMaps(left map[string]any, right map[string]any) map[string]any {
	next := map[string]any{}
	for key, value := range left {
		next[key] = value
	}
	for key, value := range right {
		next[key] = value
	}
	return next
}

func (s *Service) seedReadyDownstreamNodes(ctx context.Context, organizationID, executionID, completedNodeID string) error {
	execution, err := s.store.GetExecution(ctx, organizationID, executionID)
	if err != nil {
		return err
	}
	if execution == nil {
		return fmt.Errorf("%w: execution %s", ErrNotFound, executionID)
	}

	definition := execution.WorkflowSnapshot
	if len(definition) == 0 {
		workflow, err := s.GetWorkflow(ctx, organizationID, execution.WorkflowID)
		if err != nil {
			return err
		}
		definition = workflow.Definition
	}
	nodes, err := definitionNodes(definition)
	if err != nil {
		return err
	}
	nodeByID := make(map[string]Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.ID] = node
	}

	edges := definitionEdges(definition)
	readyNodes := downstreamReadyNodeExecutionsByID(execution.NodeExecutions)
	existingNodes := existingNodeIDs(execution.NodeExecutions)
	for _, edge := range downstreamEdges(edges, completedNodeID) {
		childID := edge.To
		if childID == "" || existingNodes[childID] {
			continue
		}
		if !edgeMatchesParentBranch(edge, readyNodes[completedNodeID]) {
			continue
		}
		if !allParentEdgesReady(edges, childID, readyNodes) {
			continue
		}
		child := nodeByID[childID]
		if child.ID == "" {
			continue
		}
		if _, err := s.store.CreateNodeExecution(ctx, organizationID, executionID, CreateNodeExecutionRequest{
			NodeID:   child.ID,
			NodeType: child.Type,
			Status:   NodeStatusPending,
			Attempt:  1,
			Input:    execution.Input,
		}); err != nil {
			return err
		}
		existingNodes[childID] = true
	}

	return nil
}

func (s *Service) getExecutionForTransition(ctx context.Context, organizationID, executionID string) (*WorkflowExecution, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(executionID) == "" {
		return nil, fmt.Errorf("%w: execution ID is required", ErrInvalidInput)
	}
	execution, err := s.store.GetExecution(ctx, organizationID, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, fmt.Errorf("%w: execution %s", ErrNotFound, executionID)
	}
	return execution, nil
}

func workflowExecutionDurationSeconds(execution *WorkflowExecution) float64 {
	if execution == nil {
		return 0
	}
	if execution.DurationMS > 0 {
		return float64(execution.DurationMS) / 1000
	}
	if execution.CompletedAt != nil && !execution.StartedAt.IsZero() {
		return execution.CompletedAt.Sub(execution.StartedAt).Seconds()
	}
	return 0
}

func (s *Service) setExecutionStatus(ctx context.Context, organizationID, executionID string, status ExecutionStatus, completedAt *time.Time) (*WorkflowExecution, error) {
	execution, err := s.store.UpdateExecutionStatus(ctx, organizationID, executionID, status, completedAt)
	if err != nil {
		return nil, err
	}
	if execution == nil {
		return nil, fmt.Errorf("%w: execution %s", ErrNotFound, executionID)
	}
	if refreshed, refreshErr := s.store.GetExecution(ctx, organizationID, executionID); refreshErr != nil {
		return nil, refreshErr
	} else if refreshed != nil {
		execution = refreshed
	}
	if isTerminalExecutionStatus(status) {
		metrics.RecordWorkflowExecution(string(status), workflowExecutionDurationSeconds(execution))
		if _, promoteErr := s.PromoteQueuedExecutions(ctx, organizationID); promoteErr != nil {
			return nil, promoteErr
		}
	}
	return execution, nil
}

func (s *Service) queuedExecutionsForOrganization(ctx context.Context, organizationID string) ([]*WorkflowExecution, error) {
	workflows, err := s.store.ListWorkflows(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	queued := []*WorkflowExecution{}
	seen := map[string]struct{}{}
	for _, workflow := range workflows {
		if workflow == nil {
			continue
		}
		executions, err := s.store.ListExecutions(ctx, organizationID, workflow.ID)
		if err != nil {
			return nil, err
		}
		for _, execution := range executions {
			if execution == nil || execution.Status != ExecutionStatusQueued {
				continue
			}
			if _, ok := seen[execution.ID]; ok {
				continue
			}
			seen[execution.ID] = struct{}{}
			queued = append(queued, execution)
		}
	}
	return queued, nil
}

func (s *Service) canPromoteQueuedExecution(ctx context.Context, execution *WorkflowExecution) (bool, error) {
	if execution == nil {
		return false, nil
	}
	if s.orgMaxConcurrentWorkflows > 0 {
		runningForOrg, err := s.store.CountRunningExecutionsForOrganization(ctx, execution.OrganizationID)
		if err != nil {
			return false, err
		}
		if runningForOrg >= s.orgMaxConcurrentWorkflows {
			return false, nil
		}
	}
	policy := concurrencyPolicyForTrigger(workflowDefinitionForExecutionPolicy(execution), triggerTypeForExecution(execution))
	if policy.MaxConcurrentExecutions > 0 {
		runningForWorkflow, err := s.store.CountRunningExecutions(ctx, execution.OrganizationID, execution.WorkflowID)
		if err != nil {
			return false, err
		}
		if runningForWorkflow >= policy.MaxConcurrentExecutions {
			return false, nil
		}
	}
	return true, nil
}

func triggerTypeForExecution(execution *WorkflowExecution) WorkflowTriggerType {
	if execution == nil || execution.Context == nil {
		return WorkflowTriggerManual
	}
	trigger, ok := execution.Context["trigger"].(map[string]any)
	if !ok {
		return WorkflowTriggerManual
	}
	triggerType, err := normalizeWorkflowTriggerType(WorkflowTriggerType(strings.TrimSpace(stringValue(trigger["type"]))))
	if err != nil {
		return WorkflowTriggerManual
	}
	return triggerType
}

func validateCreateWorkflowRequest(req CreateWorkflowRequest) error {
	if strings.TrimSpace(req.OrganizationID) == "" {
		return fmt.Errorf("%w: organization ID is required", ErrInvalidInput)
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("%w: workflow name is required", ErrInvalidInput)
	}
	return validateWorkflowDefinition(req.Definition)
}

func validateWorkflowDefinition(definition map[string]any) error {
	nodes, err := definitionNodes(definition)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("%w: workflow must include at least one node", ErrInvalidInput)
	}
	nodeIDs := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.ID)
		if nodeID == "" {
			return fmt.Errorf("%w: workflow node ID is required", ErrInvalidInput)
		}
		if nodeIDs[nodeID] {
			return fmt.Errorf("%w: duplicate workflow node ID %s", ErrInvalidInput, nodeID)
		}
		nodeIDs[nodeID] = true
		nodeType := strings.TrimSpace(node.Type)
		if nodeType == "" {
			nodeType = "start"
		}
		if !isKnownWorkflowNodeType(nodeType) {
			return fmt.Errorf("%w: unsupported workflow node type %s", ErrInvalidInput, nodeType)
		}
	}

	edges, err := validDefinitionEdges(definition)
	if err != nil {
		return err
	}
	incoming := map[string]bool{}
	graph := make(map[string][]string, len(nodes))
	for _, edge := range edges {
		if !nodeIDs[edge.From] {
			return fmt.Errorf("%w: workflow edge references unknown source node %s", ErrInvalidInput, edge.From)
		}
		if !nodeIDs[edge.To] {
			return fmt.Errorf("%w: workflow edge references unknown target node %s", ErrInvalidInput, edge.To)
		}
		graph[edge.From] = append(graph[edge.From], edge.To)
		incoming[edge.To] = true
	}
	if len(nodes) > 0 && len(incoming) == len(nodes) {
		return fmt.Errorf("%w: workflow must include at least one runnable root node", ErrInvalidInput)
	}
	if workflowDefinitionHasCycle(nodes, graph) {
		return fmt.Errorf("%w: workflow graph must be acyclic", ErrInvalidInput)
	}
	return nil
}

func isKnownWorkflowNodeType(nodeType string) bool {
	switch strings.TrimSpace(nodeType) {
	case "start", "end", "manual", "trigger", "condition", "loop", "join", "code", "http", "llm", "knowledge", "user_input", "approval", "agent", "tool", "database", "rpa":
		return true
	default:
		return false
	}
}

func validDefinitionEdges(definition map[string]any) ([]definitionEdge, error) {
	values := definition["edges"]
	if values == nil {
		return nil, nil
	}
	switch typed := values.(type) {
	case []any:
		edges := make([]definitionEdge, 0, len(typed))
		for _, value := range typed {
			edge, ok := definitionEdgeFromAny(value)
			if !ok {
				return nil, fmt.Errorf("%w: workflow edge is invalid", ErrInvalidInput)
			}
			if edge.From == "" || edge.To == "" {
				return nil, fmt.Errorf("%w: workflow edge endpoints are required", ErrInvalidInput)
			}
			edges = append(edges, edge)
		}
		return edges, nil
	case []map[string]any:
		edges := make([]definitionEdge, 0, len(typed))
		for _, value := range typed {
			edge := definitionEdgeFromMap(value)
			if edge.From == "" || edge.To == "" {
				return nil, fmt.Errorf("%w: workflow edge endpoints are required", ErrInvalidInput)
			}
			edges = append(edges, edge)
		}
		return edges, nil
	default:
		return nil, fmt.Errorf("%w: workflow edges are invalid", ErrInvalidInput)
	}
}

func workflowDefinitionHasCycle(nodes []Node, graph map[string][]string) bool {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := map[string]int{}
	var visit func(string) bool
	visit = func(nodeID string) bool {
		switch state[nodeID] {
		case visiting:
			return true
		case visited:
			return false
		}
		state[nodeID] = visiting
		for _, childID := range graph[nodeID] {
			if visit(childID) {
				return true
			}
		}
		state[nodeID] = visited
		return false
	}
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.ID)
		if state[nodeID] == unvisited && visit(nodeID) {
			return true
		}
	}
	return false
}

func validateScheduleTriggersForStatus(status WorkflowStatus, definition map[string]any) error {
	if status != WorkflowStatusPublished {
		return nil
	}
	for _, trigger := range scheduleTriggersFromDefinition(definition) {
		if strings.TrimSpace(trigger.ID) == "" {
			return fmt.Errorf("%w: schedule trigger id is required", ErrInvalidInput)
		}
		if strings.TrimSpace(trigger.CronExpression) == "" {
			return fmt.Errorf("%w: schedule trigger cron expression is required", ErrInvalidInput)
		}
	}
	return nil
}

func (s *Service) syncWorkflowScheduleTriggers(ctx context.Context, workflow *WorkflowDefinition) error {
	if s == nil || s.scheduleSyncer == nil || workflow == nil {
		return nil
	}
	triggers := []WorkflowScheduleTrigger{}
	if workflow.Status == WorkflowStatusPublished {
		triggers = scheduleTriggersFromDefinition(workflow.Definition)
	}
	return s.scheduleSyncer.SyncWorkflowScheduleTriggers(ctx, WorkflowScheduleSyncRequest{
		OrganizationID: workflow.OrganizationID,
		WorkflowID:     workflow.ID,
		Triggers:       triggers,
		Now:            time.Now().UTC(),
	})
}

func workflowDefinitionForExecutionPolicy(execution *WorkflowExecution) *WorkflowDefinition {
	if execution == nil {
		return &WorkflowDefinition{}
	}
	return &WorkflowDefinition{
		ID:             execution.WorkflowID,
		OrganizationID: execution.OrganizationID,
		Version:        execution.WorkflowVersion,
		Definition:     execution.WorkflowSnapshot,
	}
}

func resourceUsageForExecution(execution *WorkflowExecution) WorkflowResourceUsage {
	if execution == nil {
		return WorkflowResourceUsage{}
	}
	usage := WorkflowResourceUsage{
		NodeExecutionCount: completedWorkflowNodeExecutionCount(execution.NodeExecutions),
	}
	for _, node := range execution.NodeExecutions {
		if !isSuccessfulNodeStatus(node.Status) {
			continue
		}
		usage.TotalTokens += tokenUsageFromOutput(node.Output)
	}
	return usage
}

func completedWorkflowNodeExecutionCount(nodes []WorkflowNodeExecution) int {
	count := 0
	for _, node := range nodes {
		switch node.Status {
		case NodeStatusSucceeded, NodeStatusCompleted, NodeStatusFailed, NodeStatusSkipped:
			count++
		}
	}
	return count
}

func tokenUsageFromOutput(output map[string]any) int {
	usage, ok := mapStringAnyFromAny(output["usage"])
	if !ok {
		return 0
	}
	for _, key := range []string{"totalTokens", "total_tokens", "tokens", "tokenCount", "token_count"} {
		if value, ok := intFromAny(usage[key]); ok && value > 0 {
			return value
		}
	}
	total := 0
	for _, key := range []string{"promptTokens", "prompt_tokens", "inputTokens", "input_tokens", "completionTokens", "completion_tokens", "outputTokens", "output_tokens"} {
		if value, ok := intFromAny(usage[key]); ok && value > 0 {
			total += value
		}
	}
	return total
}

func definitionHasAtLeastOneNode(definition map[string]any) bool {
	nodes, ok := definition["nodes"]
	if !ok || nodes == nil {
		return false
	}
	switch typed := nodes.(type) {
	case []any:
		return len(typed) > 0
	case []map[string]any:
		return len(typed) > 0
	case []Node:
		return len(typed) > 0
	default:
		return false
	}
}

func definitionHasNodeID(definition map[string]any, nodeID string) bool {
	nodes, ok := definition["nodes"]
	if !ok || nodes == nil {
		return false
	}
	switch typed := nodes.(type) {
	case []any:
		for _, node := range typed {
			if mapNodeHasID(node, nodeID) {
				return true
			}
		}
	case []map[string]any:
		for _, node := range typed {
			if value, ok := node["id"].(string); ok && value == nodeID {
				return true
			}
		}
	case []Node:
		for _, node := range typed {
			if node.ID == nodeID {
				return true
			}
		}
	}
	return false
}

func startNodeExecutionsForDefinition(definition map[string]any, input map[string]any) ([]CreateNodeExecutionRequest, error) {
	nodes, err := definitionNodes(definition)
	if err != nil {
		return nil, err
	}
	incoming := map[string]bool{}
	for _, edge := range definitionEdges(definition) {
		if edge.To != "" {
			incoming[edge.To] = true
		}
	}

	startNodes := make([]CreateNodeExecutionRequest, 0, len(nodes))
	for _, node := range nodes {
		if node.ID == "" {
			return nil, fmt.Errorf("%w: workflow node ID is required", ErrInvalidInput)
		}
		if incoming[node.ID] {
			continue
		}
		startNodes = append(startNodes, CreateNodeExecutionRequest{
			NodeID:   node.ID,
			NodeType: node.Type,
			Status:   NodeStatusPending,
			Attempt:  1,
			Input:    input,
		})
	}
	if len(startNodes) == 0 {
		return nil, fmt.Errorf("%w: workflow must include at least one runnable root node", ErrInvalidInput)
	}
	return startNodes, nil
}

type semanticTriggerDefinition struct {
	ID                string
	Keywords          []string
	SemanticThreshold float64
	Definition        map[string]any
}

type conversationTriggerDefinition struct {
	ID             string
	ConversationID string
	Definition     map[string]any
}

func conversationTriggersFromDefinition(definition map[string]any) []conversationTriggerDefinition {
	triggers, ok := mapStringAnyFromAny(definition["triggers"])
	if !ok {
		return nil
	}
	return conversationTriggersFromAny(triggers["conversation"])
}

func conversationTriggersFromAny(value any) []conversationTriggerDefinition {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		triggers := make([]conversationTriggerDefinition, 0, len(typed))
		for _, item := range typed {
			if trigger, ok := conversationTriggerFromAny(item); ok {
				triggers = append(triggers, trigger)
			}
		}
		return triggers
	case []map[string]any:
		triggers := make([]conversationTriggerDefinition, 0, len(typed))
		for _, item := range typed {
			if trigger, ok := conversationTriggerFromAny(item); ok {
				triggers = append(triggers, trigger)
			}
		}
		return triggers
	default:
		if trigger, ok := conversationTriggerFromAny(value); ok {
			return []conversationTriggerDefinition{trigger}
		}
		return nil
	}
}

func conversationTriggerFromAny(value any) (conversationTriggerDefinition, bool) {
	triggerMap, ok := mapStringAnyFromAny(value)
	if !ok {
		return conversationTriggerDefinition{}, false
	}
	trigger := conversationTriggerDefinition{Definition: triggerMap}
	if id, ok := triggerMap["id"].(string); ok {
		trigger.ID = strings.TrimSpace(id)
	}
	for _, key := range []string{"conversationId", "conversation_id", "conversation"} {
		if conversationID, ok := triggerMap[key].(string); ok {
			trigger.ConversationID = strings.TrimSpace(conversationID)
			break
		}
	}
	return trigger, trigger.ConversationID != ""
}

func semanticTriggersFromDefinition(definition map[string]any) []semanticTriggerDefinition {
	triggers, ok := mapStringAnyFromAny(definition["triggers"])
	if !ok {
		return nil
	}
	return semanticTriggersFromAny(triggers["semantic"])
}

func semanticTriggersFromAny(value any) []semanticTriggerDefinition {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		triggers := make([]semanticTriggerDefinition, 0, len(typed))
		for _, item := range typed {
			if trigger, ok := semanticTriggerFromAny(item); ok {
				triggers = append(triggers, trigger)
			}
		}
		return triggers
	case []map[string]any:
		triggers := make([]semanticTriggerDefinition, 0, len(typed))
		for _, item := range typed {
			if trigger, ok := semanticTriggerFromAny(item); ok {
				triggers = append(triggers, trigger)
			}
		}
		return triggers
	default:
		if trigger, ok := semanticTriggerFromAny(value); ok {
			return []semanticTriggerDefinition{trigger}
		}
		return nil
	}
}

func semanticTriggerFromAny(value any) (semanticTriggerDefinition, bool) {
	triggerMap, ok := mapStringAnyFromAny(value)
	if !ok {
		return semanticTriggerDefinition{}, false
	}
	trigger := semanticTriggerDefinition{Definition: triggerMap}
	if id, ok := triggerMap["id"].(string); ok {
		trigger.ID = strings.TrimSpace(id)
	}
	trigger.Keywords = stringsFromAny(triggerMap["keywords"])
	if threshold, ok := float64FromAny(triggerMap["semanticThreshold"]); ok {
		trigger.SemanticThreshold = threshold
	}
	if threshold, ok := float64FromAny(triggerMap["semantic_threshold"]); ok {
		trigger.SemanticThreshold = threshold
	}
	return trigger, len(trigger.Keywords) > 0
}

func scheduleTriggersFromDefinition(definition map[string]any) []WorkflowScheduleTrigger {
	triggers, ok := mapStringAnyFromAny(definition["triggers"])
	if !ok {
		return nil
	}
	return scheduleTriggersFromAny(triggers["schedule"])
}

func scheduleTriggersFromAny(value any) []WorkflowScheduleTrigger {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		triggers := make([]WorkflowScheduleTrigger, 0, len(typed))
		for _, item := range typed {
			if trigger, ok := scheduleTriggerFromAny(item); ok {
				triggers = append(triggers, trigger)
			}
		}
		return triggers
	case []map[string]any:
		triggers := make([]WorkflowScheduleTrigger, 0, len(typed))
		for _, item := range typed {
			if trigger, ok := scheduleTriggerFromAny(item); ok {
				triggers = append(triggers, trigger)
			}
		}
		return triggers
	default:
		if trigger, ok := scheduleTriggerFromAny(value); ok {
			return []WorkflowScheduleTrigger{trigger}
		}
		return nil
	}
}

func scheduleTriggerFromAny(value any) (WorkflowScheduleTrigger, bool) {
	triggerMap, ok := mapStringAnyFromAny(value)
	if !ok {
		return WorkflowScheduleTrigger{}, false
	}
	trigger := WorkflowScheduleTrigger{Definition: triggerMap, Enabled: true}
	if id, ok := triggerMap["id"].(string); ok {
		trigger.ID = strings.TrimSpace(id)
	}
	for _, key := range []string{"name", "title", "label"} {
		if name, ok := triggerMap[key].(string); ok {
			trigger.Name = strings.TrimSpace(name)
			break
		}
	}
	for _, key := range []string{"cron", "cronExpression", "cron_expression", "expression"} {
		if expression, ok := triggerMap[key].(string); ok {
			trigger.CronExpression = strings.TrimSpace(expression)
			break
		}
	}
	if enabled, ok := boolFromAny(triggerMap["enabled"]); ok {
		trigger.Enabled = enabled
	}
	return trigger, trigger.ID != "" || trigger.CronExpression != ""
}

func stringsFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		keywords := make([]string, 0, len(typed))
		for _, item := range typed {
			if keyword := strings.TrimSpace(item); keyword != "" {
				keywords = append(keywords, keyword)
			}
		}
		return keywords
	case []any:
		keywords := make([]string, 0, len(typed))
		for _, item := range typed {
			keyword, ok := item.(string)
			if !ok {
				continue
			}
			if keyword = strings.TrimSpace(keyword); keyword != "" {
				keywords = append(keywords, keyword)
			}
		}
		return keywords
	default:
		return nil
	}
}

func boolFromAny(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func firstMatchingSemanticKeyword(message string, keywords []string) (string, bool) {
	normalizedMessage := strings.ToLower(message)
	for _, keyword := range keywords {
		normalizedKeyword := strings.ToLower(strings.TrimSpace(keyword))
		if normalizedKeyword == "" {
			continue
		}
		if strings.Contains(normalizedMessage, normalizedKeyword) {
			return keyword, true
		}
	}
	return "", false
}

func firstSemanticTriggerKeyword(keywords []string) string {
	for _, keyword := range keywords {
		if strings.TrimSpace(keyword) != "" {
			return keyword
		}
	}
	return ""
}

func definitionNodes(definition map[string]any) ([]Node, error) {
	values, ok := definition["nodes"]
	if !ok || values == nil {
		return nil, fmt.Errorf("%w: workflow must include at least one node", ErrInvalidInput)
	}
	switch typed := values.(type) {
	case []any:
		nodes := make([]Node, 0, len(typed))
		for _, value := range typed {
			node, ok := definitionNode(value)
			if !ok {
				return nil, fmt.Errorf("%w: workflow node is invalid", ErrInvalidInput)
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	case []map[string]any:
		nodes := make([]Node, 0, len(typed))
		for _, value := range typed {
			nodes = append(nodes, nodeFromMap(value))
		}
		return nodes, nil
	case []Node:
		return append([]Node(nil), typed...), nil
	default:
		return nil, fmt.Errorf("%w: workflow nodes are invalid", ErrInvalidInput)
	}
}

func nodeDefinitionByID(definition map[string]any, nodeID string) (Node, error) {
	nodes, err := definitionNodes(definition)
	if err != nil {
		return Node{}, err
	}
	for _, node := range nodes {
		if node.ID == nodeID {
			return node, nil
		}
	}
	return Node{}, fmt.Errorf("%w: workflow node %s", ErrNotFound, nodeID)
}

func definitionNode(value any) (Node, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return nodeFromMap(typed), true
	case Node:
		return typed, true
	default:
		return Node{}, false
	}
}

func nodeFromMap(value map[string]any) Node {
	node := Node{}
	if id, ok := value["id"].(string); ok {
		node.ID = strings.TrimSpace(id)
	}
	if nodeType, ok := value["type"].(string); ok {
		node.Type = strings.TrimSpace(nodeType)
	}
	if input, ok := mapStringAnyFromAny(value["input"]); ok {
		node.Input = input
	}
	if policy, ok := failurePolicyFromMapValue(value["failurePolicy"]); ok {
		node.FailurePolicy = policy
	}
	if policy, ok := failurePolicyFromMapValue(value["failure_policy"]); ok {
		node.FailurePolicy = policy
	}
	return node
}

func mapStringAnyFromAny(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		mapped := make(map[string]any, len(typed))
		for key, item := range typed {
			mapped[key] = item
		}
		return mapped, true
	default:
		return nil, false
	}
}

func failurePolicyFromMapValue(value any) (FailurePolicy, bool) {
	if value == nil {
		return FailurePolicy{}, false
	}
	switch typed := value.(type) {
	case FailurePolicy:
		return typed, true
	case map[string]any:
		policy := FailurePolicy{}
		if strategy, ok := typed["strategy"].(string); ok {
			policy.Strategy = FailureStrategy(strings.TrimSpace(strategy))
		}
		if maxRetries, ok := intFromAny(typed["maxRetries"]); ok {
			policy.MaxRetries = maxRetries
		}
		if maxRetries, ok := intFromAny(typed["max_retries"]); ok {
			policy.MaxRetries = maxRetries
		}
		if branch, ok := typed["failureBranchNodeId"].(string); ok {
			policy.FailureBranchNodeID = strings.TrimSpace(branch)
		}
		if branch, ok := typed["failure_branch_node_id"].(string); ok {
			policy.FailureBranchNodeID = strings.TrimSpace(branch)
		}
		policy.RetryDelays = retryDelaysFromAny(typed["retryDelays"])
		if len(policy.RetryDelays) == 0 {
			policy.RetryDelays = retryDelaysFromAny(typed["retry_delays"])
		}
		return policy, true
	default:
		return FailurePolicy{}, false
	}
}

func intFromAny(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	default:
		return 0, false
	}
}

func float64FromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func retryDelaysFromAny(value any) []time.Duration {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	delays := make([]time.Duration, 0, len(values))
	for _, item := range values {
		switch typed := item.(type) {
		case time.Duration:
			if typed > 0 {
				delays = append(delays, typed)
			}
		case string:
			delay, err := time.ParseDuration(typed)
			if err == nil && delay > 0 {
				delays = append(delays, delay)
			}
		case int:
			if typed > 0 {
				delays = append(delays, time.Duration(typed)*time.Second)
			}
		case float64:
			if typed > 0 {
				delays = append(delays, time.Duration(typed*float64(time.Second)))
			}
		}
	}
	return delays
}

type definitionEdge struct {
	From string
	To   string
	Raw  map[string]any
}

func definitionEdges(definition map[string]any) []definitionEdge {
	values := definition["edges"]
	switch typed := values.(type) {
	case []any:
		edges := make([]definitionEdge, 0, len(typed))
		for _, value := range typed {
			if edge, ok := definitionEdgeFromAny(value); ok {
				edges = append(edges, edge)
			}
		}
		return edges
	case []map[string]any:
		edges := make([]definitionEdge, 0, len(typed))
		for _, value := range typed {
			edges = append(edges, definitionEdgeFromMap(value))
		}
		return edges
	default:
		return nil
	}
}

func definitionEdgeFromAny(value any) (definitionEdge, bool) {
	typed, ok := value.(map[string]any)
	if !ok {
		return definitionEdge{}, false
	}
	return definitionEdgeFromMap(typed), true
}

func definitionEdgeFromMap(value map[string]any) definitionEdge {
	return definitionEdge{
		From: edgeEndpoint(value, "from", "source", "sourceId", "sourceNodeId"),
		To:   edgeEndpoint(value, "to", "target", "targetId", "targetNodeId"),
		Raw:  value,
	}
}

func edgeEndpoint(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if endpoint, ok := value[key].(string); ok {
			return strings.TrimSpace(endpoint)
		}
	}
	return ""
}

func isSuccessfulNodeStatus(status NodeStatus) bool {
	switch status {
	case NodeStatusSucceeded, NodeStatusCompleted:
		return true
	default:
		return false
	}
}

func isDownstreamReadyNodeStatus(status NodeStatus) bool {
	return isSuccessfulNodeStatus(status) || status == NodeStatusSkipped
}

func nextPendingWorkflowNodeID(nodes []WorkflowNodeExecution) string {
	latest := latestWorkflowNodeExecutionsByID(nodes)
	for _, node := range nodes {
		if latestNode, ok := latest[node.NodeID]; ok && latestNode.ID == node.ID && latestNode.Status == NodeStatusPending {
			return node.NodeID
		}
	}
	return ""
}

func nextRunnableWorkflowNodeID(nodes []WorkflowNodeExecution, now time.Time) string {
	if nodeID := nextPendingWorkflowNodeID(nodes); nodeID != "" {
		return nodeID
	}
	latest := latestWorkflowNodeExecutionsByID(nodes)
	for _, node := range nodes {
		latestNode, ok := latest[node.NodeID]
		if !ok || latestNode.ID != node.ID || latestNode.Status != NodeStatusRetrying {
			continue
		}
		if retryingWorkflowNodeReady(latestNode, now) {
			return node.NodeID
		}
	}
	return ""
}

func retryingWorkflowNodeReady(node WorkflowNodeExecution, now time.Time) bool {
	if node.Context == nil {
		return true
	}
	value, ok := node.Context["retryAt"]
	if !ok {
		return true
	}
	switch typed := value.(type) {
	case time.Time:
		return !typed.After(now)
	case string:
		retryAt, err := time.Parse(time.RFC3339, strings.TrimSpace(typed))
		if err != nil {
			return true
		}
		return !retryAt.After(now)
	default:
		return true
	}
}

func latestWorkflowNodesComplete(nodes []WorkflowNodeExecution) bool {
	if len(nodes) == 0 {
		return false
	}
	latest := latestWorkflowNodeExecutionsByID(nodes)
	for _, node := range latest {
		if !isDownstreamReadyNodeStatus(node.Status) {
			return false
		}
	}
	return true
}

func completedWorkflowExecutionStatus(nodes []WorkflowNodeExecution) ExecutionStatus {
	latest := latestWorkflowNodeExecutionsByID(nodes)
	for _, node := range latest {
		if node.Status == NodeStatusSkipped {
			return ExecutionStatusPartialSuccess
		}
	}
	return ExecutionStatusSucceeded
}

func latestWorkflowNodeExecutionsByID(nodes []WorkflowNodeExecution) map[string]WorkflowNodeExecution {
	latest := make(map[string]WorkflowNodeExecution, len(nodes))
	for _, node := range nodes {
		latest[node.NodeID] = node
	}
	return latest
}

func latestFailedWorkflowNodeID(nodes []WorkflowNodeExecution) string {
	for index := len(nodes) - 1; index >= 0; index-- {
		if nodes[index].Status == NodeStatusFailed {
			return nodes[index].NodeID
		}
	}
	return ""
}

func latestPendingWorkflowNodeInput(nodes []WorkflowNodeExecution, nodeID string) (map[string]any, bool) {
	for index := len(nodes) - 1; index >= 0; index-- {
		node := nodes[index]
		if node.NodeID == nodeID && node.Status == NodeStatusPending {
			return node.Input, true
		}
	}
	return nil, false
}

func latestPendingUserInputWorkflowNode(nodes []WorkflowNodeExecution, nodeID string) (WorkflowNodeExecution, bool) {
	node, ok := latestPendingResumeWorkflowNode(nodes, nodeID)
	if !ok || node.Context["waitReason"] != "user_input_required" {
		return WorkflowNodeExecution{}, false
	}
	return node, true
}

func latestPendingResumeWorkflowNode(nodes []WorkflowNodeExecution, nodeID string) (WorkflowNodeExecution, bool) {
	for index := len(nodes) - 1; index >= 0; index-- {
		node := nodes[index]
		if node.Status != NodeStatusPending {
			continue
		}
		if nodeID != "" && node.NodeID != nodeID {
			continue
		}
		switch node.Context["waitReason"] {
		case "user_input_required", "approval_required", "agent_approval_required":
			return node, true
		}
	}
	return WorkflowNodeExecution{}, false
}

func userInputRequiredFields(input map[string]any) []string {
	if len(input) == 0 {
		return nil
	}
	fields := []string{}
	switch typed := input["required"].(type) {
	case []any:
		for _, item := range typed {
			field := strings.TrimSpace(stringFromWorkflowValue(item))
			if field != "" {
				fields = append(fields, field)
			}
		}
	case []string:
		for _, item := range typed {
			field := strings.TrimSpace(item)
			if field != "" {
				fields = append(fields, field)
			}
		}
	}
	return fields
}

func successfulNodeIDs(nodes []WorkflowNodeExecution) map[string]bool {
	successful := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		if isDownstreamReadyNodeStatus(node.Status) {
			successful[node.NodeID] = true
		}
	}
	return successful
}

func existingNodeIDs(nodes []WorkflowNodeExecution) map[string]bool {
	existing := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		existing[node.NodeID] = true
	}
	return existing
}

func downstreamReadyNodeExecutionsByID(nodes []WorkflowNodeExecution) map[string]WorkflowNodeExecution {
	ready := make(map[string]WorkflowNodeExecution, len(nodes))
	for _, node := range nodes {
		if isDownstreamReadyNodeStatus(node.Status) {
			ready[node.NodeID] = node
		}
	}
	return ready
}

func downstreamNodeIDs(edges []definitionEdge, nodeID string) []string {
	downstream := []string{}
	seen := map[string]bool{}
	for _, edge := range edges {
		if edge.From != nodeID || edge.To == "" || seen[edge.To] {
			continue
		}
		seen[edge.To] = true
		downstream = append(downstream, edge.To)
	}
	return downstream
}

func downstreamEdges(edges []definitionEdge, nodeID string) []definitionEdge {
	downstream := []definitionEdge{}
	seen := map[string]bool{}
	for _, edge := range edges {
		if edge.From != nodeID || edge.To == "" || seen[edge.To] {
			continue
		}
		seen[edge.To] = true
		downstream = append(downstream, edge)
	}
	return downstream
}

func allParentNodesSucceeded(edges []definitionEdge, nodeID string, successfulNodes map[string]bool) bool {
	parentCount := 0
	for _, edge := range edges {
		if edge.To != nodeID || edge.From == "" {
			continue
		}
		parentCount++
		if !successfulNodes[edge.From] {
			return false
		}
	}
	return parentCount > 0
}

func allParentEdgesReady(edges []definitionEdge, nodeID string, readyNodes map[string]WorkflowNodeExecution) bool {
	parentCount := 0
	for _, edge := range edges {
		if edge.To != nodeID || edge.From == "" {
			continue
		}
		parentCount++
		parent := readyNodes[edge.From]
		if parent.NodeID == "" || !edgeMatchesParentBranch(edge, parent) {
			return false
		}
	}
	return parentCount > 0
}

func edgeMatchesParentBranch(edge definitionEdge, parent WorkflowNodeExecution) bool {
	expected, ok := edgeBranchValue(edge)
	if !ok {
		return true
	}
	actual, ok := nodeExecutionBranchValue(parent)
	if !ok {
		return false
	}
	return actual == expected
}

func edgeBranchValue(edge definitionEdge) (string, bool) {
	if branch, ok := edgeMetadataString(edge.Raw, "branch", "condition", "when"); ok {
		return normalizeBranchValue(branch), true
	}
	if condition, ok := edgeMetadataMap(edge.Raw, "condition", "when"); ok {
		if branch, ok := edgeMetadataString(condition, "branch", "matched"); ok {
			return normalizeBranchValue(branch), true
		}
	}
	return "", false
}

func edgeMetadataString(raw map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return stringFromWorkflowValue(value), true
		}
	}
	if data, ok := mapStringAnyFromAny(raw["data"]); ok {
		for _, key := range keys {
			if value, ok := data[key]; ok {
				return stringFromWorkflowValue(value), true
			}
		}
	}
	return "", false
}

func edgeMetadataMap(raw map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		if value, ok := mapStringAnyFromAny(raw[key]); ok {
			return value, true
		}
	}
	if data, ok := mapStringAnyFromAny(raw["data"]); ok {
		for _, key := range keys {
			if value, ok := mapStringAnyFromAny(data[key]); ok {
				return value, true
			}
		}
	}
	return nil, false
}

func nodeExecutionBranchValue(node WorkflowNodeExecution) (string, bool) {
	if node.Output == nil {
		return "", false
	}
	if branch, ok := node.Output["branch"]; ok {
		return normalizeBranchValue(stringFromWorkflowValue(branch)), true
	}
	if matched, ok := boolFromWorkflowValue(node.Output["matched"]); ok {
		if matched {
			return "true", true
		}
		return "false", true
	}
	return "", false
}

func normalizeBranchValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

type workflowBranchMetadata struct {
	ExperimentKey  string
	SourceVersion  int
	SourceWorkflow string
	TrafficPercent int
}

func workflowDefinitionWithBranchMetadata(definition map[string]any, metadata workflowBranchMetadata) map[string]any {
	next := cloneWorkflowMap(definition)
	next["branch"] = map[string]any{
		"experimentKey":    metadata.ExperimentKey,
		"sourceVersion":    metadata.SourceVersion,
		"sourceWorkflowId": metadata.SourceWorkflow,
		"trafficPercent":   metadata.TrafficPercent,
	}
	return next
}

func cloneWorkflowMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil || cloned == nil {
		return map[string]any{}
	}
	return cloned
}

func mapNodeHasID(node any, nodeID string) bool {
	switch typed := node.(type) {
	case map[string]any:
		value, ok := typed["id"].(string)
		return ok && value == nodeID
	case Node:
		return typed.ID == nodeID
	default:
		return false
	}
}

func isTerminalExecutionStatus(status ExecutionStatus) bool {
	switch status {
	case ExecutionStatusSucceeded, ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusCancelled, ExecutionStatusPartialSuccess, ExecutionStatusTimedOut, ExecutionStatusMaxIterations:
		return true
	default:
		return false
	}
}

func activeExecutionHealthStatuses() []ExecutionStatus {
	return []ExecutionStatus{ExecutionStatusRunning, ExecutionStatusQueued, ExecutionStatusPaused}
}

func isRunnableExecutionStatus(status ExecutionStatus) bool {
	return status == ExecutionStatusRunning || status == ExecutionStatusPartialSuccess
}
