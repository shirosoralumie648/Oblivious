package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"oblivious/server/internal/config"
	"oblivious/server/internal/notification"
	relaytypes "oblivious/server/internal/relay/types"
	"oblivious/server/internal/workflow"
)

func TestNewConfiguredWorkflowServiceWiresRelaySemanticTriggerMatcher(t *testing.T) {
	var received struct {
		userID         string
		organizationID string
		inputs         []string
	}
	relayServer := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost || r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected relay embedding request %s %s", r.Method, r.URL.Path)
		}
		received.userID = r.Header.Get(relaytypes.HeaderInternalUserID)
		received.organizationID = r.Header.Get(relaytypes.HeaderInternalOrganization)

		var payload struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode relay embedding request: %v", err)
		}
		received.inputs = append(received.inputs, payload.Input...)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","model":"text-embedding-3-small","data":[{"object":"embedding","index":0,"embedding":[1,0]}],"usage":{"prompt_tokens":1,"total_tokens":1}}`))
	}))
	defer relayServer.Close()

	store := newWorkflowServiceMemoryStore()
	service := newConfiguredWorkflowServiceWithStore(config.Config{
		Port:                 8080,
		RelayEnabled:         true,
		WorkflowRelayBaseURL: relayServer.URL + "/v1",
	}, store)

	created, err := service.CreateWorkflow(context.Background(), workflow.CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Semantic billing triage",
		Status:         workflow.WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{map[string]any{"id": "start", "type": "manual"}},
			"triggers": map[string]any{
				"semantic": []any{
					map[string]any{
						"id":                "semantic_billing",
						"keywords":          []any{"payment failure"},
						"semanticThreshold": 0.9,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	matches, err := service.MatchSemanticTriggers(context.Background(), workflow.MatchSemanticTriggersRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Message:        "客户说扣款一直失败，需要人工介入",
	})
	if err != nil {
		t.Fatalf("MatchSemanticTriggers returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("MatchSemanticTriggers returned %d matches, want 1: %+v", len(matches), matches)
	}
	match := matches[0]
	if match.WorkflowID != created.ID || match.TriggerID != "semantic_billing" {
		t.Fatalf("unexpected semantic match: %+v", match)
	}
	if match.Keyword != "payment failure" || match.MatchMethod != "embedding" || match.Score < 0.9 {
		t.Fatalf("expected relay-backed embedding match metadata, got %+v", match)
	}
	if received.userID != "user_1" || received.organizationID != "org_1" {
		t.Fatalf("expected trusted relay identity user_1/org_1, got user=%q org=%q", received.userID, received.organizationID)
	}
	if len(received.inputs) != 2 || received.inputs[0] != "客户说扣款一直失败，需要人工介入" || received.inputs[1] != "payment failure" {
		t.Fatalf("expected query and keyword embedding inputs, got %+v", received.inputs)
	}
}

func TestNewConfiguredWorkflowServiceWiresFailurePauseNotification(t *testing.T) {
	notificationStore := &workflowServiceNotificationStore{}
	store := newWorkflowServiceMemoryStore()
	service := newConfiguredWorkflowServiceWithStoreAndNotifier(config.Config{
		RelayEnabled: false,
	}, store, notification.NewService(notificationStore))

	created, err := service.CreateWorkflow(context.Background(), workflow.CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Incident triage",
		Status:         workflow.WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{map[string]any{"id": "call_agent", "type": "agent"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(context.Background(), workflow.StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Context: map[string]any{
			"userId":      "user_1",
			"workspaceId": "workspace_1",
		},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if _, err := service.RecordNodeStatus(context.Background(), "org_1", execution.ID, workflow.RecordNodeStatusRequest{
		NodeID:   "call_agent",
		NodeType: "agent",
		Status:   workflow.NodeStatusFailed,
		Error:    map[string]any{"message": "agent timeout"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus returned error: %v", err)
	}

	if len(notificationStore.created) != 1 {
		t.Fatalf("expected one in-app notification, got %+v", notificationStore.created)
	}
	got := notificationStore.created[0]
	if got.UserID != "user_1" || got.Type != "warning" || got.Category != "workflow" {
		t.Fatalf("unexpected notification envelope: %+v", got)
	}
	if got.Title != "Workflow paused: Incident triage" || got.Message != "Node call_agent failed: agent timeout" {
		t.Fatalf("unexpected notification content: %+v", got)
	}
	if got.ActionURL != "/workspace/workflows/"+created.ID+"/executions/"+execution.ID {
		t.Fatalf("unexpected action URL: %q", got.ActionURL)
	}
	if got.Metadata["event"] != "workflow.failure_paused" || got.Metadata["nodeType"] != "agent" || got.Metadata["workspaceId"] != "workspace_1" {
		t.Fatalf("unexpected notification metadata: %+v", got.Metadata)
	}
}

type workflowServiceMemoryStore struct {
	workflows  map[string]*workflow.WorkflowDefinition
	versions   map[string][]*workflow.WorkflowDefinition
	executions map[string]*workflow.WorkflowExecution
	nodes      map[string][]workflow.WorkflowNodeExecution
	nextID     int
}

func newWorkflowServiceMemoryStore() *workflowServiceMemoryStore {
	return &workflowServiceMemoryStore{
		workflows:  map[string]*workflow.WorkflowDefinition{},
		versions:   map[string][]*workflow.WorkflowDefinition{},
		executions: map[string]*workflow.WorkflowExecution{},
		nodes:      map[string][]workflow.WorkflowNodeExecution{},
		nextID:     1,
	}
}

func (s *workflowServiceMemoryStore) CreateWorkflow(ctx context.Context, req workflow.CreateWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	id := "workflow_default_wiring_1"
	if s.nextID > 1 {
		id = "workflow_default_wiring_2"
	}
	s.nextID++
	now := time.Now().UTC()
	created := &workflow.WorkflowDefinition{
		ID:             id,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Description:    req.Description,
		Status:         req.Status,
		Version:        req.Version,
		Definition:     req.Definition,
		Variables:      req.Variables,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if created.Status == "" {
		created.Status = workflow.WorkflowStatusDraft
	}
	if created.Version <= 0 {
		created.Version = 1
	}
	s.workflows[id] = cloneWorkflowServiceDefinition(created)
	s.versions[id] = append(s.versions[id], cloneWorkflowServiceDefinition(created))
	return cloneWorkflowServiceDefinition(created), nil
}

func (s *workflowServiceMemoryStore) GetWorkflow(ctx context.Context, organizationID, id string) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	workflowDefinition := s.workflows[id]
	if workflowDefinition == nil || workflowDefinition.OrganizationID != organizationID {
		return nil, nil
	}
	return cloneWorkflowServiceDefinition(workflowDefinition), nil
}

func (s *workflowServiceMemoryStore) ListWorkflows(ctx context.Context, organizationID string) ([]*workflow.WorkflowDefinition, error) {
	_ = ctx
	workflows := []*workflow.WorkflowDefinition{}
	for _, workflowDefinition := range s.workflows {
		if workflowDefinition.OrganizationID == organizationID {
			workflows = append(workflows, cloneWorkflowServiceDefinition(workflowDefinition))
		}
	}
	return workflows, nil
}

func (s *workflowServiceMemoryStore) ListWorkflowVersions(ctx context.Context, organizationID, workflowID string) ([]*workflow.WorkflowDefinition, error) {
	_ = ctx
	versions := []*workflow.WorkflowDefinition{}
	for _, version := range s.versions[workflowID] {
		if version.OrganizationID == organizationID {
			versions = append(versions, cloneWorkflowServiceDefinition(version))
		}
	}
	return versions, nil
}

func (s *workflowServiceMemoryStore) GetWorkflowVersion(ctx context.Context, organizationID, workflowID string, version int) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	for _, workflowVersion := range s.versions[workflowID] {
		if workflowVersion.OrganizationID == organizationID && workflowVersion.Version == version {
			return cloneWorkflowServiceDefinition(workflowVersion), nil
		}
	}
	return nil, nil
}

func (s *workflowServiceMemoryStore) UpdateWorkflow(ctx context.Context, req workflow.UpdateWorkflowStoreRequest) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	return nil, nil
}

func (s *workflowServiceMemoryStore) CreateExecution(ctx context.Context, req workflow.CreateExecutionRequest) (*workflow.WorkflowExecution, error) {
	_ = ctx
	id := s.nextIDValue("wexec")
	now := time.Now().UTC()
	execution := &workflow.WorkflowExecution{
		ID:               id,
		WorkflowID:       req.WorkflowID,
		WorkflowVersion:  req.WorkflowVersion,
		OrganizationID:   req.OrganizationID,
		Status:           req.Status,
		Input:            req.Input,
		Output:           req.Output,
		Error:            req.Error,
		Context:          req.Context,
		WorkflowSnapshot: req.WorkflowSnapshot,
		StartedAt:        req.StartedAt,
		CompletedAt:      req.CompletedAt,
		DurationMS:       req.DurationMS,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if execution.Status == "" {
		execution.Status = workflow.ExecutionStatusRunning
	}
	if execution.StartedAt.IsZero() {
		execution.StartedAt = now
	}
	s.executions[id] = cloneWorkflowServiceExecution(execution)
	for _, nodeReq := range req.NodeExecutions {
		node, _ := s.CreateNodeExecution(context.Background(), req.OrganizationID, id, nodeReq)
		if node != nil {
			execution.NodeExecutions = append(execution.NodeExecutions, *node)
		}
	}
	return cloneWorkflowServiceExecution(execution), nil
}

func (s *workflowServiceMemoryStore) ListExecutions(ctx context.Context, organizationID, workflowID string) ([]*workflow.WorkflowExecution, error) {
	_ = ctx
	executions := []*workflow.WorkflowExecution{}
	for _, execution := range s.executions {
		if execution.OrganizationID == organizationID && execution.WorkflowID == workflowID {
			executions = append(executions, cloneWorkflowServiceExecution(execution))
		}
	}
	return executions, nil
}

func (s *workflowServiceMemoryStore) GetExecution(ctx context.Context, organizationID, id string) (*workflow.WorkflowExecution, error) {
	_ = ctx
	execution := s.executions[id]
	if execution == nil || execution.OrganizationID != organizationID {
		return nil, nil
	}
	cloned := cloneWorkflowServiceExecution(execution)
	cloned.NodeExecutions = append([]workflow.WorkflowNodeExecution(nil), s.nodes[id]...)
	return cloned, nil
}

func (s *workflowServiceMemoryStore) ListActiveExecutionHealth(ctx context.Context, organizationID string, statuses []workflow.ExecutionStatus) ([]workflow.WorkflowExecutionHealthSummary, error) {
	_ = ctx
	return nil, nil
}

func (s *workflowServiceMemoryStore) CountRunningExecutions(ctx context.Context, organizationID, workflowID string) (int, error) {
	_ = ctx
	return 0, nil
}

func (s *workflowServiceMemoryStore) CountRunningExecutionsForOrganization(ctx context.Context, organizationID string) (int, error) {
	_ = ctx
	return 0, nil
}

func (s *workflowServiceMemoryStore) UpdateExecutionStatus(ctx context.Context, organizationID, id string, status workflow.ExecutionStatus, completedAt *time.Time) (*workflow.WorkflowExecution, error) {
	_ = ctx
	execution := s.executions[id]
	if execution == nil || execution.OrganizationID != organizationID {
		return nil, nil
	}
	execution.Status = status
	execution.CompletedAt = completedAt
	execution.UpdatedAt = time.Now().UTC()
	return cloneWorkflowServiceExecution(execution), nil
}

func (s *workflowServiceMemoryStore) CreateNodeExecution(ctx context.Context, organizationID, executionID string, req workflow.CreateNodeExecutionRequest) (*workflow.WorkflowNodeExecution, error) {
	_ = ctx
	execution := s.executions[executionID]
	if execution == nil || execution.OrganizationID != organizationID {
		return nil, nil
	}
	now := time.Now().UTC()
	node := workflow.WorkflowNodeExecution{
		ID:             s.nextIDValue("wnode"),
		ExecutionID:    executionID,
		OrganizationID: organizationID,
		NodeID:         req.NodeID,
		NodeType:       req.NodeType,
		Status:         req.Status,
		Attempt:        req.Attempt,
		Input:          req.Input,
		Output:         req.Output,
		Error:          req.Error,
		Context:        req.Context,
		StartedAt:      req.StartedAt,
		CompletedAt:    req.CompletedAt,
		DurationMS:     req.DurationMS,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if node.Status == "" {
		node.Status = workflow.NodeStatusPending
	}
	if node.StartedAt.IsZero() {
		node.StartedAt = now
	}
	s.nodes[executionID] = append(s.nodes[executionID], node)
	return &node, nil
}

func cloneWorkflowServiceDefinition(input *workflow.WorkflowDefinition) *workflow.WorkflowDefinition {
	if input == nil {
		return nil
	}
	cloned := *input
	return &cloned
}

func cloneWorkflowServiceExecution(input *workflow.WorkflowExecution) *workflow.WorkflowExecution {
	if input == nil {
		return nil
	}
	cloned := *input
	cloned.NodeExecutions = append([]workflow.WorkflowNodeExecution(nil), input.NodeExecutions...)
	return &cloned
}

func (s *workflowServiceMemoryStore) nextIDValue(prefix string) string {
	id := prefix + "_default_wiring_1"
	if s.nextID > 1 {
		id = prefix + "_default_wiring_2"
	}
	s.nextID++
	return id
}

type workflowServiceNotificationStore struct {
	created []*notification.Notification
}

func (s *workflowServiceNotificationStore) Create(ctx context.Context, notification *notification.Notification) (*notification.Notification, error) {
	s.created = append(s.created, notification)
	return notification, nil
}

func (s *workflowServiceNotificationStore) Get(ctx context.Context, id string) (*notification.Notification, error) {
	return nil, nil
}

func (s *workflowServiceNotificationStore) List(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*notification.Notification, error) {
	return nil, nil
}

func (s *workflowServiceNotificationStore) MarkRead(ctx context.Context, id string) error {
	return nil
}

func (s *workflowServiceNotificationStore) MarkAllRead(ctx context.Context, userID string) error {
	return nil
}

func (s *workflowServiceNotificationStore) Delete(ctx context.Context, id string) error {
	return nil
}

func (s *workflowServiceNotificationStore) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	return 0, nil
}
