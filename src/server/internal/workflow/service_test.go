package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"oblivious/server/internal/metrics"
	relaytypes "oblivious/server/internal/relay/types"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestServiceCreateWorkflowValidatesRequiredFields(t *testing.T) {
	service := NewService(newMemoryWorkflowStore())
	ctx := context.Background()

	if _, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Definition:     workflowDefinitionWithNodes("start"),
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateWorkflow without name err=%v, want ErrInvalidInput", err)
	}

	if _, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Empty Workflow",
		Definition:     map[string]any{"nodes": []any{}},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateWorkflow without nodes err=%v, want ErrInvalidInput", err)
	}
}

func TestServiceCreateWorkflowRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		definition map[string]any
	}{
		{
			name: "duplicate node ID",
			definition: workflowDefinitionDAG(
				[]map[string]any{
					{"id": "start", "type": "manual"},
					{"id": "start", "type": "http"},
				},
				nil,
			),
		},
		{
			name: "unknown node type",
			definition: workflowDefinitionDAG(
				[]map[string]any{{"id": "start", "type": "teleport"}},
				nil,
			),
		},
		{
			name: "edge references missing node",
			definition: workflowDefinitionDAG(
				[]map[string]any{{"id": "start", "type": "manual"}},
				[]map[string]any{{"from": "start", "to": "missing"}},
			),
		},
		{
			name: "cyclic graph",
			definition: workflowDefinitionDAG(
				[]map[string]any{
					{"id": "first", "type": "manual"},
					{"id": "second", "type": "agent"},
				},
				[]map[string]any{
					{"from": "first", "to": "second"},
					{"from": "second", "to": "first"},
				},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(newMemoryWorkflowStore())
			_, err := service.CreateWorkflow(context.Background(), CreateWorkflowRequest{
				OrganizationID: "org_1",
				Name:           "Invalid Workflow",
				Definition:     tt.definition,
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("CreateWorkflow err=%v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceCreatesAndReadsWorkflow(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Description:    "Provision account",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithNodes("start", "notify"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if created.ID == "" || created.Name != "Customer Onboarding" {
		t.Fatalf("unexpected created workflow: %+v", created)
	}

	got, err := service.GetWorkflow(ctx, "org_1", created.ID)
	if err != nil {
		t.Fatalf("GetWorkflow returned error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetWorkflow ID=%q, want %q", got.ID, created.ID)
	}

	workflows, err := service.ListWorkflows(ctx, "org_1")
	if err != nil {
		t.Fatalf("ListWorkflows returned error: %v", err)
	}
	if len(workflows) != 1 || workflows[0].ID != created.ID {
		t.Fatalf("ListWorkflows got %+v, want created workflow", workflows)
	}
}

func TestServiceUpdateWorkflowMergesAndValidatesFinalDefinition(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Description:    "Provision account",
		Status:         WorkflowStatusDraft,
		Definition:     workflowDefinitionWithNodes("start"),
		Variables:      map[string]any{"priority": "normal"},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	updated, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Name:           stringPtr(" Escalation Flow "),
		Status:         workflowStatusPtr(WorkflowStatusPublished),
		Definition:     workflowDefinitionWithNodes("start", "notify"),
		Variables:      map[string]any{"priority": "high"},
	})
	if err != nil {
		t.Fatalf("UpdateWorkflow returned error: %v", err)
	}
	if updated.Name != "Escalation Flow" || updated.Description != "Provision account" || updated.Status != WorkflowStatusPublished {
		t.Fatalf("unexpected updated workflow fields: %+v", updated)
	}
	if len(updated.Definition["nodes"].([]any)) != 2 || updated.Variables["priority"] != "high" {
		t.Fatalf("unexpected updated workflow payload: definition=%+v variables=%+v", updated.Definition, updated.Variables)
	}

	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Name:           stringPtr("  "),
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateWorkflow empty final name err=%v, want ErrInvalidInput", err)
	}

	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Definition:     map[string]any{"nodes": []any{}},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateWorkflow empty final definition err=%v, want ErrInvalidInput", err)
	}

	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Definition: workflowDefinitionDAG(
			[]map[string]any{{"id": "start", "type": "manual"}},
			[]map[string]any{{"from": "start", "to": "missing"}},
		),
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("UpdateWorkflow invalid final definition err=%v, want ErrInvalidInput", err)
	}
}

func TestServiceUpdateWorkflowCreatesVersionHistory(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	updated, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Definition:     workflowDefinitionWithNodes("start", "notify"),
	})
	if err != nil {
		t.Fatalf("UpdateWorkflow returned error: %v", err)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("UpdateWorkflow version=%d, want %d", updated.Version, created.Version+1)
	}

	versions, err := service.ListWorkflowVersions(ctx, "org_1", created.ID)
	if err != nil {
		t.Fatalf("ListWorkflowVersions returned error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected two workflow versions, got %+v", versions)
	}
	if versions[0].Version != 1 || len(versions[0].Definition["nodes"].([]any)) != 1 {
		t.Fatalf("expected v1 to preserve original definition, got %+v", versions[0])
	}
	if versions[1].Version != 2 || len(versions[1].Definition["nodes"].([]any)) != 2 {
		t.Fatalf("expected v2 to preserve updated definition, got %+v", versions[1])
	}
}

func TestServiceDeleteWorkflowArchivesWorkflow(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	archived, err := service.DeleteWorkflow(ctx, "org_1", created.ID)
	if err != nil {
		t.Fatalf("DeleteWorkflow returned error: %v", err)
	}
	if archived.Status != WorkflowStatusArchived {
		t.Fatalf("DeleteWorkflow status=%s, want archived", archived.Status)
	}

	got, err := service.GetWorkflow(ctx, "org_1", created.ID)
	if err != nil {
		t.Fatalf("GetWorkflow after delete returned error: %v", err)
	}
	if got.Status != WorkflowStatusArchived {
		t.Fatalf("expected soft archived workflow to remain readable, got %+v", got)
	}
}

func TestServiceSyncsScheduleTriggersForPublishedWorkflowLifecycle(t *testing.T) {
	store := newMemoryWorkflowStore()
	syncer := &recordingWorkflowScheduleSyncer{}
	service := NewService(store, WithScheduleSyncer(syncer))
	ctx := context.Background()

	draft, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Draft Scheduled Workflow",
		Status:         WorkflowStatusDraft,
		Definition:     workflowDefinitionWithScheduleTrigger("daily-report", "0 9 * * 1", true, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow draft returned error: %v", err)
	}
	if len(syncer.requests) != 0 {
		t.Fatalf("draft workflow should not sync schedule triggers, got %+v", syncer.requests)
	}

	publishedStatus := WorkflowStatusPublished
	publishedDefinition := workflowDefinitionWithScheduleTrigger("daily-report", "30 9 * * 1", true, "start")
	published, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     draft.ID,
		Status:         &publishedStatus,
		Definition:     publishedDefinition,
	})
	if err != nil {
		t.Fatalf("UpdateWorkflow publish returned error: %v", err)
	}
	if published.Status != WorkflowStatusPublished {
		t.Fatalf("expected published workflow, got %+v", published)
	}
	if len(syncer.requests) != 1 {
		t.Fatalf("expected one schedule sync after publish, got %+v", syncer.requests)
	}
	publishReq := syncer.requests[0]
	if publishReq.OrganizationID != "org_1" || publishReq.WorkflowID != draft.ID {
		t.Fatalf("unexpected publish sync scope: %+v", publishReq)
	}
	if len(publishReq.Triggers) != 1 || publishReq.Triggers[0].ID != "daily-report" || publishReq.Triggers[0].CronExpression != "30 9 * * 1" || !publishReq.Triggers[0].Enabled {
		t.Fatalf("unexpected published schedule triggers: %+v", publishReq.Triggers)
	}

	archived, err := service.DeleteWorkflow(ctx, "org_1", draft.ID)
	if err != nil {
		t.Fatalf("DeleteWorkflow returned error: %v", err)
	}
	if archived.Status != WorkflowStatusArchived {
		t.Fatalf("expected archived workflow, got %+v", archived)
	}
	if len(syncer.requests) != 2 {
		t.Fatalf("expected archive to reconcile empty schedule triggers, got %+v", syncer.requests)
	}
	archiveReq := syncer.requests[1]
	if archiveReq.WorkflowID != draft.ID || len(archiveReq.Triggers) != 0 {
		t.Fatalf("expected archive sync with empty trigger list, got %+v", archiveReq)
	}
}

func TestServiceCreatePublishedWorkflowSyncsScheduleTriggers(t *testing.T) {
	store := newMemoryWorkflowStore()
	syncer := &recordingWorkflowScheduleSyncer{}
	service := NewService(store, WithScheduleSyncer(syncer))

	created, err := service.CreateWorkflow(context.Background(), CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Published Scheduled Workflow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionWithScheduleTriggers([]map[string]any{
			{"id": "hourly", "name": "Hourly sync", "cron": "0 * * * *"},
			{"id": "disabled-maintenance", "cronExpression": "30 2 * * *", "enabled": false},
		}, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow published returned error: %v", err)
	}

	if len(syncer.requests) != 1 {
		t.Fatalf("expected one schedule sync, got %+v", syncer.requests)
	}
	req := syncer.requests[0]
	if req.WorkflowID != created.ID || req.OrganizationID != "org_1" {
		t.Fatalf("unexpected schedule sync scope: %+v", req)
	}
	if len(req.Triggers) != 2 {
		t.Fatalf("expected two schedule triggers, got %+v", req.Triggers)
	}
	if req.Triggers[0].ID != "hourly" || req.Triggers[0].Name != "Hourly sync" || req.Triggers[0].CronExpression != "0 * * * *" || !req.Triggers[0].Enabled {
		t.Fatalf("unexpected first schedule trigger: %+v", req.Triggers[0])
	}
	if req.Triggers[1].ID != "disabled-maintenance" || req.Triggers[1].Enabled {
		t.Fatalf("unexpected disabled schedule trigger: %+v", req.Triggers[1])
	}
}

func TestServiceRejectsPublishedWorkflowScheduleTriggerWithoutStableIdentityOrCron(t *testing.T) {
	service := NewService(newMemoryWorkflowStore(), WithScheduleSyncer(&recordingWorkflowScheduleSyncer{}))

	tests := []struct {
		name       string
		definition map[string]any
	}{
		{
			name:       "missing trigger id",
			definition: workflowDefinitionWithScheduleTrigger("", "0 * * * *", true, "start"),
		},
		{
			name:       "missing cron",
			definition: workflowDefinitionWithScheduleTrigger("daily-report", "", true, "start"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.CreateWorkflow(context.Background(), CreateWorkflowRequest{
				OrganizationID: "org_1",
				Name:           "Invalid Published Schedule",
				Status:         WorkflowStatusPublished,
				Definition:     tt.definition,
			}); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("CreateWorkflow err=%v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestServiceStartExecutionBindsLatestPublishedVersion(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	published, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Published Runtime",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithNodes("published_start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	draft := WorkflowStatusDraft
	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     published.ID,
		Status:         &draft,
		Definition:     workflowDefinitionWithNodes("draft_start"),
	}); err != nil {
		t.Fatalf("UpdateWorkflow draft returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     published.ID,
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if execution.WorkflowVersion != published.Version {
		t.Fatalf("StartExecution workflow version=%d, want published version %d", execution.WorkflowVersion, published.Version)
	}
	if len(execution.NodeExecutions) != 1 || execution.NodeExecutions[0].NodeID != "published_start" {
		t.Fatalf("StartExecution should seed nodes from latest published version, got %+v", execution.NodeExecutions)
	}
}

func TestServiceRollbackWorkflowCreatesNewVersionFromHistory(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Rollback Runtime",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithNodes("stable_start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Definition:     workflowDefinitionWithNodes("broken_start"),
	}); err != nil {
		t.Fatalf("UpdateWorkflow returned error: %v", err)
	}

	rolledBack, err := service.RollbackWorkflow(ctx, RollbackWorkflowRequest{
		OrganizationID: created.OrganizationID,
		WorkflowID:     created.ID,
		Version:        created.Version,
	})
	if err != nil {
		t.Fatalf("RollbackWorkflow returned error: %v", err)
	}
	if rolledBack.Version != 3 {
		t.Fatalf("RollbackWorkflow version=%d, want 3", rolledBack.Version)
	}
	if got := rolledBack.Definition["nodes"].([]any)[0].(map[string]any)["id"]; got != "stable_start" {
		t.Fatalf("RollbackWorkflow definition node=%v, want stable_start", got)
	}

	versions, err := service.ListWorkflowVersions(ctx, "org_1", created.ID)
	if err != nil {
		t.Fatalf("ListWorkflowVersions returned error: %v", err)
	}
	if len(versions) != 3 || versions[2].Version != rolledBack.Version {
		t.Fatalf("expected rollback to append v3 to history, got %+v", versions)
	}
}

func TestServiceCreateWorkflowBranchCopiesVersionAsDraftExperiment(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Incident Router",
		Description:    "Stable production routing",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "manual-start", "type": "manual"},
				{"id": "notify-team", "type": "http"},
			},
			[]map[string]any{{"from": "manual-start", "to": "notify-team"}},
		),
		Variables: map[string]any{"owner": "ops", "priority": "high"},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Status:         workflowStatusPtr(WorkflowStatusDraft),
		Definition:     workflowDefinitionWithNodes("draft-only"),
		Variables:      map[string]any{"owner": "draft"},
	}); err != nil {
		t.Fatalf("UpdateWorkflow draft returned error: %v", err)
	}

	branch, err := service.CreateWorkflowBranch(ctx, CreateWorkflowBranchRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Version:        created.Version,
		Name:           "Incident Router Experiment B",
		ExperimentKey:  "routing-copy-v2",
		TrafficPercent: 25,
	})
	if err != nil {
		t.Fatalf("CreateWorkflowBranch returned error: %v", err)
	}
	if branch.ID == created.ID {
		t.Fatalf("expected branch to be a new workflow, got source id %q", branch.ID)
	}
	if branch.Status != WorkflowStatusDraft || branch.Version != 1 {
		t.Fatalf("expected draft branch v1, got status=%s version=%d", branch.Status, branch.Version)
	}
	if branch.Name != "Incident Router Experiment B" || branch.Description != "Stable production routing" {
		t.Fatalf("unexpected branch identity: %+v", branch)
	}
	if branch.Variables["owner"] != "ops" || branch.Variables["priority"] != "high" {
		t.Fatalf("expected branch variables from source version, got %+v", branch.Variables)
	}
	nodes := branch.Definition["nodes"].([]any)
	if nodes[0].(map[string]any)["id"] != "manual-start" {
		t.Fatalf("expected branch definition from source version, got %+v", branch.Definition)
	}
	metadata, ok := branch.Definition["branch"].(map[string]any)
	if !ok {
		t.Fatalf("expected branch metadata in definition, got %+v", branch.Definition["branch"])
	}
	if metadata["sourceWorkflowId"] != created.ID || metadata["sourceVersion"].(int) != created.Version {
		t.Fatalf("unexpected source metadata: %+v", metadata)
	}
	if metadata["experimentKey"] != "routing-copy-v2" || metadata["trafficPercent"].(int) != 25 {
		t.Fatalf("unexpected experiment metadata: %+v", metadata)
	}
}

func TestServiceTestNodeValidatesNodeAndEchoesInput(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Definition:     workflowDefinitionWithNodes("start", "notify"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	result, err := service.TestNode(ctx, TestNodeRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		NodeID:         "notify",
		Input:          map[string]any{"ticket": "INC-1"},
	})
	if err != nil {
		t.Fatalf("TestNode returned error: %v", err)
	}
	if result.WorkflowID != created.ID || result.NodeID != "notify" || result.Status != ExecutionStatusSucceeded {
		t.Fatalf("unexpected test node result: %+v", result)
	}
	if result.Input["ticket"] != "INC-1" || result.Output["input"].(map[string]any)["ticket"] != "INC-1" {
		t.Fatalf("expected input to be echoed, got input=%+v output=%+v", result.Input, result.Output)
	}

	if _, err := service.TestNode(ctx, TestNodeRequest{OrganizationID: "org_1", WorkflowID: created.ID, NodeID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("TestNode missing node err=%v, want ErrNotFound", err)
	}
}

func TestServiceTestNodeReturnsFailedResultForExecutorError(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(functionNodeExecutor{
		nodeType: "http",
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			return map[string]any{"statusCode": 500}, errors.New("upstream timeout")
		},
	})))
	ctx := context.Background()

	created, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Incident triage",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "notify", "type": "http", "input": map[string]any{"ticket": "{{input.ticket}}"}},
			},
			nil,
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	result, err := service.TestNode(ctx, TestNodeRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		NodeID:         "notify",
		Input:          map[string]any{"ticket": "INC-1"},
	})
	if err != nil {
		t.Fatalf("TestNode executor failure err=%v, want structured failed result", err)
	}
	if result.WorkflowID != created.ID || result.NodeID != "notify" || result.Status != ExecutionStatusFailed {
		t.Fatalf("unexpected failed test node result identity/status: %+v", result)
	}
	if result.Input["ticket"] != "INC-1" {
		t.Fatalf("expected resolved input to be preserved, got %+v", result.Input)
	}
	if result.Output["statusCode"] != 500 {
		t.Fatalf("expected partial executor output to be preserved, got %+v", result.Output)
	}

	payload := map[string]any{}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed test node result: %v", err)
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode failed test node result: %v", err)
	}
	errorPayload, ok := payload["error"].(map[string]any)
	if !ok || errorPayload["message"] != "upstream timeout" {
		t.Fatalf("expected executor error message in result, got %+v", payload["error"])
	}
	if duration, ok := payload["durationMs"].(float64); !ok || duration <= 0 {
		t.Fatalf("expected failed test node duration to be recorded, got %+v", payload["durationMs"])
	}
	trace, ok := payload["trace"].([]any)
	if !ok || len(trace) == 0 {
		t.Fatalf("expected failed node trace entry, got %+v", payload["trace"])
	}
	traceEntry, ok := trace[0].(map[string]any)
	if !ok || traceEntry["nodeId"] != "notify" || traceEntry["status"] != string(ExecutionStatusFailed) {
		t.Fatalf("expected failed node trace entry, got %+v", payload["trace"])
	}
}

func TestServiceGetWorkflowMapsMissingWorkflowToNotFound(t *testing.T) {
	service := NewService(newMemoryWorkflowStore())

	if _, err := service.GetWorkflow(context.Background(), "org_1", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetWorkflow missing err=%v, want ErrNotFound", err)
	}
}

func TestServiceStartExecutionCreatesRunningExecutionFromWorkflow(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"customer_id": "cus_123"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if execution.WorkflowID != workflow.ID || execution.Status != ExecutionStatusRunning {
		t.Fatalf("unexpected started execution: %+v", execution)
	}
	if execution.Input["customer_id"] != "cus_123" {
		t.Fatalf("expected execution input to be preserved, got %#v", execution.Input)
	}
	if len(store.executions) != 1 {
		t.Fatalf("expected one persisted execution, got %d", len(store.executions))
	}
}

func TestServiceStartExecutionSeedsStartNodesFromDefinition(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Customer Onboarding",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "manual"},
				{"id": "notify", "type": "agent"},
			},
			[]map[string]any{
				{"from": "start", "to": "notify"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"customer_id": "cus_123"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if len(execution.NodeExecutions) != 1 {
		t.Fatalf("expected one seeded node execution, got %+v", execution.NodeExecutions)
	}
	node := execution.NodeExecutions[0]
	if node.NodeID != "start" || node.NodeType != "manual" || node.Status != NodeStatusPending || node.Attempt != 1 {
		t.Fatalf("unexpected seeded node execution: %+v", node)
	}
	if node.Input["customer_id"] != "cus_123" {
		t.Fatalf("expected start node input to match execution input, got %#v", node.Input)
	}
}

func TestServiceStartExecutionSeedsAllRootNodes(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Parallel Intake",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "manual_review", "type": "manual"},
				{"id": "agent_check", "type": "agent"},
				{"id": "join", "type": "join"},
			},
			[]map[string]any{
				{"from": "manual_review", "to": "join"},
				{"from": "agent_check", "to": "join"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if len(execution.NodeExecutions) != 2 {
		t.Fatalf("expected two seeded root node executions, got %+v", execution.NodeExecutions)
	}
	if execution.NodeExecutions[0].NodeID != "manual_review" || execution.NodeExecutions[1].NodeID != "agent_check" {
		t.Fatalf("expected roots in definition order, got %+v", execution.NodeExecutions)
	}
	if execution.NodeExecutions[0].Status != NodeStatusPending || execution.NodeExecutions[1].Status != NodeStatusPending {
		t.Fatalf("expected pending root nodes, got %+v", execution.NodeExecutions)
	}
}

func TestServiceStartExecutionRecordsSupportedTriggerTypesInContext(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Triggered Workflow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	for _, triggerType := range []WorkflowTriggerType{
		WorkflowTriggerManual,
		WorkflowTriggerConversation,
		WorkflowTriggerSchedule,
		WorkflowTriggerWebhook,
		WorkflowTriggerSemantic,
	} {
		execution, err := service.StartExecution(ctx, StartExecutionRequest{
			OrganizationID: "org_1",
			WorkflowID:     workflow.ID,
			TriggerType:    triggerType,
			TriggerPayload: map[string]any{"source": string(triggerType)},
		})
		if err != nil {
			t.Fatalf("StartExecution trigger %s returned error: %v", triggerType, err)
		}
		trigger, ok := execution.Context["trigger"].(map[string]any)
		if !ok {
			t.Fatalf("expected trigger context for %s, got %#v", triggerType, execution.Context["trigger"])
		}
		if trigger["type"] != string(triggerType) || trigger["source"] != string(triggerType) {
			t.Fatalf("unexpected trigger context for %s: %#v", triggerType, trigger)
		}
	}

	if _, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerType("email"),
	}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("StartExecution unknown trigger err=%v, want ErrInvalidInput", err)
	}
}

func TestServiceMatchSemanticTriggersReturnsPublishedKeywordMatches(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	incidentWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Incident Router",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionWithSemanticTrigger(
			"incident_trigger",
			[]string{"urgent billing", "账单异常"},
			0.85,
			"start",
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow incident returned error: %v", err)
	}
	if _, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Sales Router",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionWithSemanticTrigger(
			"sales_trigger",
			[]string{"pricing question", "购买咨询"},
			0.8,
			"start",
		),
	}); err != nil {
		t.Fatalf("CreateWorkflow sales returned error: %v", err)
	}
	draftOnlyWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Draft Only",
		Status:         WorkflowStatusDraft,
		Definition: workflowDefinitionWithSemanticTrigger(
			"draft_trigger",
			[]string{"urgent billing"},
			0.9,
			"start",
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow draft returned error: %v", err)
	}
	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     incidentWorkflow.ID,
		Status:         workflowStatusPtr(WorkflowStatusDraft),
		Definition: workflowDefinitionWithSemanticTrigger(
			"draft_revision_trigger",
			[]string{"pricing question"},
			0.7,
			"start",
		),
	}); err != nil {
		t.Fatalf("UpdateWorkflow draft revision returned error: %v", err)
	}

	matches, err := service.MatchSemanticTriggers(ctx, MatchSemanticTriggersRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Message:        "请帮我处理这个账单异常，URGENT BILLING escalation",
	})
	if err != nil {
		t.Fatalf("MatchSemanticTriggers returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("MatchSemanticTriggers returned %d matches, want 1: %+v", len(matches), matches)
	}
	match := matches[0]
	if match.WorkflowID != incidentWorkflow.ID {
		t.Fatalf("matched workflow ID=%q, want %q", match.WorkflowID, incidentWorkflow.ID)
	}
	if match.WorkflowVersion != incidentWorkflow.Version {
		t.Fatalf("matched workflow version=%d, want latest published version %d", match.WorkflowVersion, incidentWorkflow.Version)
	}
	if match.TriggerID != "incident_trigger" || match.Keyword != "urgent billing" {
		t.Fatalf("unexpected semantic trigger match: %+v", match)
	}
	if match.SemanticThreshold != 0.85 {
		t.Fatalf("semantic threshold=%v, want 0.85", match.SemanticThreshold)
	}
	if match.WorkflowID == draftOnlyWorkflow.ID {
		t.Fatalf("draft workflow should not match: %+v", match)
	}
}

func TestServiceMatchSemanticTriggersUsesInjectedMatcherForThresholdTriggers(t *testing.T) {
	store := newMemoryWorkflowStore()
	matcher := &recordingSemanticTriggerMatcher{
		decision: SemanticTriggerMatchDecision{
			Matched:     true,
			Keyword:     "payment failure",
			Score:       0.91,
			MatchMethod: "embedding",
		},
	}
	service := NewService(store, WithSemanticTriggerMatcher(matcher))
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Billing Escalation",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionWithSemanticTrigger(
			"semantic_billing",
			[]string{"payment failure"},
			0.88,
			"start",
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	matches, err := service.MatchSemanticTriggers(ctx, MatchSemanticTriggersRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Message:        "客户说扣款一直失败，需要人工介入",
	})
	if err != nil {
		t.Fatalf("MatchSemanticTriggers returned error: %v", err)
	}
	if matcher.calls != 1 {
		t.Fatalf("expected matcher to be called once, got %d", matcher.calls)
	}
	if matcher.lastRequest.OrganizationID != "org_1" || matcher.lastRequest.UserID != "user_1" || matcher.lastRequest.Threshold != 0.88 {
		t.Fatalf("unexpected matcher request: %+v", matcher.lastRequest)
	}
	if len(matches) != 1 {
		t.Fatalf("MatchSemanticTriggers returned %d matches, want 1: %+v", len(matches), matches)
	}
	match := matches[0]
	if match.WorkflowID != workflow.ID || match.TriggerID != "semantic_billing" {
		t.Fatalf("unexpected workflow match: %+v", match)
	}
	if match.Keyword != "payment failure" || match.Score != 0.91 || match.MatchMethod != "embedding" {
		t.Fatalf("unexpected matcher metadata: %+v", match)
	}
}

func TestServiceMatchSemanticTriggersSkipsMatcherWithoutThreshold(t *testing.T) {
	store := newMemoryWorkflowStore()
	matcher := &recordingSemanticTriggerMatcher{
		decision: SemanticTriggerMatchDecision{
			Matched:     true,
			Keyword:     "payment failure",
			Score:       0.99,
			MatchMethod: "embedding",
		},
	}
	service := NewService(store, WithSemanticTriggerMatcher(matcher))
	ctx := context.Background()

	if _, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Billing Escalation",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionWithSemanticTrigger(
			"semantic_billing",
			[]string{"payment failure"},
			0,
			"start",
		),
	}); err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	matches, err := service.MatchSemanticTriggers(ctx, MatchSemanticTriggersRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Message:        "客户说扣款一直失败，需要人工介入",
	})
	if err != nil {
		t.Fatalf("MatchSemanticTriggers returned error: %v", err)
	}
	if matcher.calls != 0 {
		t.Fatalf("expected matcher not to be called without semanticThreshold, got %d", matcher.calls)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches without keyword or threshold matcher, got %+v", matches)
	}
}

func TestEmbeddingSemanticTriggerMatcherChoosesBestKeywordAndPropagatesIdentity(t *testing.T) {
	embedder := &recordingWorkflowEmbedder{
		embeddings: map[string][]float32{
			"客户说扣款一直失败，需要人工介入": {1, 0},
			"payment failure":  {0.9, 0.1},
			"pricing question": {0, 1},
		},
	}
	matcher := NewEmbeddingSemanticTriggerMatcher(embedder)

	decision, err := matcher.MatchSemanticTrigger(context.Background(), SemanticTriggerMatchRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Message:        "客户说扣款一直失败，需要人工介入",
		Keywords:       []string{"pricing question", "payment failure"},
		Threshold:      0.8,
	})
	if err != nil {
		t.Fatalf("MatchSemanticTrigger returned error: %v", err)
	}
	if !decision.Matched || decision.Keyword != "payment failure" || decision.MatchMethod != "embedding" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.Score < 0.99 {
		t.Fatalf("expected high cosine score, got %.4f", decision.Score)
	}
	if embedder.userID != "user_1" || embedder.organizationID != "org_1" {
		t.Fatalf("expected trusted identity user_1/org_1, got user=%q org=%q", embedder.userID, embedder.organizationID)
	}
}

func TestEmbeddingSemanticTriggerMatcherRejectsBelowThreshold(t *testing.T) {
	embedder := &recordingWorkflowEmbedder{
		embeddings: map[string][]float32{
			"客户说扣款一直失败，需要人工介入": {1, 0},
			"pricing question": {0, 1},
		},
	}
	matcher := NewEmbeddingSemanticTriggerMatcher(embedder)

	decision, err := matcher.MatchSemanticTrigger(context.Background(), SemanticTriggerMatchRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		Message:        "客户说扣款一直失败，需要人工介入",
		Keywords:       []string{"pricing question"},
		Threshold:      0.8,
	})
	if err != nil {
		t.Fatalf("MatchSemanticTrigger returned error: %v", err)
	}
	if decision.Matched {
		t.Fatalf("expected no match below threshold, got %+v", decision)
	}
}

func TestServiceMatchConversationTriggersReturnsPublishedConversationBindings(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	boundWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Conversation Router",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithConversationTrigger("conversation_incident", "conversation_trigger", "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow bound returned error: %v", err)
	}
	if _, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Other Conversation",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithConversationTrigger("conversation_sales", "sales_trigger", "start"),
	}); err != nil {
		t.Fatalf("CreateWorkflow other returned error: %v", err)
	}
	draftOnlyWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Draft Conversation",
		Status:         WorkflowStatusDraft,
		Definition:     workflowDefinitionWithConversationTrigger("conversation_incident", "draft_trigger", "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow draft returned error: %v", err)
	}
	if _, err := service.UpdateWorkflow(ctx, UpdateWorkflowRequest{
		OrganizationID: "org_1",
		WorkflowID:     boundWorkflow.ID,
		Status:         workflowStatusPtr(WorkflowStatusDraft),
		Definition:     workflowDefinitionWithConversationTrigger("conversation_sales", "draft_revision_trigger", "start"),
	}); err != nil {
		t.Fatalf("UpdateWorkflow draft revision returned error: %v", err)
	}

	matches, err := service.MatchConversationTriggers(ctx, MatchConversationTriggersRequest{
		OrganizationID: "org_1",
		ConversationID: " conversation_incident ",
	})
	if err != nil {
		t.Fatalf("MatchConversationTriggers returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("MatchConversationTriggers returned %d matches, want 1: %+v", len(matches), matches)
	}
	match := matches[0]
	if match.WorkflowID != boundWorkflow.ID {
		t.Fatalf("matched workflow ID=%q, want %q", match.WorkflowID, boundWorkflow.ID)
	}
	if match.WorkflowVersion != boundWorkflow.Version {
		t.Fatalf("matched workflow version=%d, want latest published version %d", match.WorkflowVersion, boundWorkflow.Version)
	}
	if match.TriggerID != "conversation_trigger" || match.ConversationID != "conversation_incident" {
		t.Fatalf("unexpected conversation trigger match: %+v", match)
	}
	if match.WorkflowID == draftOnlyWorkflow.ID {
		t.Fatalf("draft workflow should not match: %+v", match)
	}
}

func TestServiceCreateWorkflowRejectsWorkflowWithoutRunnableRoot(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	_, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Cyclic Workflow",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "first", "type": "manual"},
				{"id": "second", "type": "agent"},
			},
			[]map[string]any{
				{"from": "first", "to": "second"},
				{"from": "second", "to": "first"},
			},
		),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateWorkflow cyclic workflow err=%v, want ErrInvalidInput", err)
	}
	if len(store.executions) != 0 {
		t.Fatalf("expected no persisted execution for cyclic workflow, got %d", len(store.executions))
	}
	if len(store.workflows) != 0 {
		t.Fatalf("expected no persisted workflow for cyclic workflow, got %d", len(store.workflows))
	}
}

func TestServiceStartExecutionRequiresExistingWorkflow(t *testing.T) {
	service := NewService(newMemoryWorkflowStore())

	_, err := service.StartExecution(context.Background(), StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     "workflow_missing",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("StartExecution missing workflow err=%v, want ErrNotFound", err)
	}
}

func TestServiceStartExecutionRejectsWhenWorkflowConcurrencyLimitIsReached(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Serial Intake",
		Definition: workflowDefinitionWithLimits(map[string]any{
			"max_concurrent_executions": float64(1),
			"concurrency_overflow":      "reject",
		}, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if _, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID}); err != nil {
		t.Fatalf("StartExecution first run returned error: %v", err)
	}

	_, err = service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if !errors.Is(err, ErrWorkflowConcurrencyLimit) {
		t.Fatalf("StartExecution over workflow concurrency err=%v, want ErrWorkflowConcurrencyLimit", err)
	}
	if got := len(store.executions); got != 1 {
		t.Fatalf("expected rejected execution not to persist, got %d executions", got)
	}
}

func TestServiceStartExecutionQueuesWhenOrganizationConcurrencyLimitIsReached(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithOrgMaxConcurrentWorkflows(1))
	ctx := context.Background()

	firstWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "First Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow first returned error: %v", err)
	}
	secondWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Second Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow second returned error: %v", err)
	}
	if _, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: firstWorkflow.ID}); err != nil {
		t.Fatalf("StartExecution first returned error: %v", err)
	}

	queued, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: secondWorkflow.ID})
	if err != nil {
		t.Fatalf("StartExecution over org concurrency returned error: %v", err)
	}
	if queued.Status != ExecutionStatusQueued {
		t.Fatalf("StartExecution over org concurrency status=%s, want queued", queued.Status)
	}
	if len(queued.NodeExecutions) != 1 || queued.NodeExecutions[0].Status != NodeStatusPending {
		t.Fatalf("expected queued execution to retain pending root node, got %+v", queued.NodeExecutions)
	}
}

func TestServiceStartExecutionQueuesScheduleTriggersSeriallyByDefault(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Scheduled Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	first, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution first schedule returned error: %v", err)
	}
	if first.Status != ExecutionStatusRunning {
		t.Fatalf("first schedule execution status=%s, want running", first.Status)
	}

	second, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution second schedule returned error: %v", err)
	}
	if second.Status != ExecutionStatusQueued {
		t.Fatalf("second schedule execution status=%s, want queued", second.Status)
	}
}

func TestServiceStartExecutionUsesTriggerAwareDefaultConcurrency(t *testing.T) {
	tests := []struct {
		name          string
		triggerType   WorkflowTriggerType
		runningCount  int
		wantStatus    ExecutionStatus
		wantPersisted int
	}{
		{
			name:          "webhook medium concurrency allows tenth run",
			triggerType:   WorkflowTriggerWebhook,
			runningCount:  9,
			wantStatus:    ExecutionStatusRunning,
			wantPersisted: 10,
		},
		{
			name:          "webhook medium concurrency queues eleventh run",
			triggerType:   WorkflowTriggerWebhook,
			runningCount:  10,
			wantStatus:    ExecutionStatusQueued,
			wantPersisted: 11,
		},
		{
			name:          "conversation high concurrency allows eleventh run",
			triggerType:   WorkflowTriggerConversation,
			runningCount:  10,
			wantStatus:    ExecutionStatusRunning,
			wantPersisted: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryWorkflowStore()
			service := NewService(store)
			ctx := context.Background()

			workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
				OrganizationID: "org_1",
				Name:           "Triggered Flow",
				Definition:     workflowDefinitionWithNodes("start"),
			})
			if err != nil {
				t.Fatalf("CreateWorkflow returned error: %v", err)
			}
			for i := 0; i < tt.runningCount; i++ {
				store.addExecution("org_1", workflow.ID, ExecutionStatusRunning)
			}

			execution, err := service.StartExecution(ctx, StartExecutionRequest{
				OrganizationID: "org_1",
				WorkflowID:     workflow.ID,
				TriggerType:    tt.triggerType,
			})
			if err != nil {
				t.Fatalf("StartExecution returned error: %v", err)
			}
			if execution.Status != tt.wantStatus {
				t.Fatalf("StartExecution status=%s, want %s", execution.Status, tt.wantStatus)
			}
			if got := len(store.executions); got != tt.wantPersisted {
				t.Fatalf("persisted executions=%d, want %d", got, tt.wantPersisted)
			}
		})
	}
}

func TestServiceStartExecutionExplicitConcurrencyOverridesTriggerDefaults(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Configured Schedule Flow",
		Definition: workflowDefinitionWithLimits(map[string]any{
			"max_concurrent_executions": float64(2),
		}, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	first, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution first schedule returned error: %v", err)
	}
	second, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution second schedule returned error: %v", err)
	}
	third, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution third schedule returned error: %v", err)
	}
	if first.Status != ExecutionStatusRunning || second.Status != ExecutionStatusRunning || third.Status != ExecutionStatusQueued {
		t.Fatalf("expected configured schedule concurrency 2, got first=%s second=%s third=%s", first.Status, second.Status, third.Status)
	}
}

func TestServicePromotesOldestQueuedWorkflowExecutionAfterCompletion(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"ok": true}),
	)))
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Serial Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"max_concurrent_executions": float64(1),
			"nodes": []map[string]any{
				{"id": "start", "type": "start"},
			},
			"edges": []map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	first, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution first returned error: %v", err)
	}
	second, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution second returned error: %v", err)
	}
	if second.Status != ExecutionStatusQueued {
		t.Fatalf("second execution status=%s, want queued", second.Status)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", first.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked first returned error: %v", err)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("first execution status=%s, want succeeded", completed.Status)
	}

	promoted, err := service.GetExecution(ctx, "org_1", second.ID)
	if err != nil {
		t.Fatalf("GetExecution promoted returned error: %v", err)
	}
	if promoted.Status != ExecutionStatusRunning {
		t.Fatalf("expected second execution promoted to running, got %+v", promoted)
	}

	rerun, err := service.RunExecutionUntilBlocked(ctx, "org_1", second.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked promoted returned error: %v", err)
	}
	if rerun.Status != ExecutionStatusSucceeded || rerun.CompletedAt == nil {
		t.Fatalf("promoted execution did not run to success: %+v", rerun)
	}
}

func TestServicePromoteQueuedExecutionsKeepsScheduleTriggersSerial(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Queued Schedule Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	running, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution running schedule returned error: %v", err)
	}
	queued, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		TriggerType:    WorkflowTriggerSchedule,
	})
	if err != nil {
		t.Fatalf("StartExecution queued schedule returned error: %v", err)
	}
	if running.Status != ExecutionStatusRunning || queued.Status != ExecutionStatusQueued {
		t.Fatalf("expected running then queued schedule executions, got running=%s queued=%s", running.Status, queued.Status)
	}

	promoted, err := service.PromoteQueuedExecutions(ctx, "org_1")
	if err != nil {
		t.Fatalf("PromoteQueuedExecutions returned error: %v", err)
	}
	if len(promoted) != 0 {
		t.Fatalf("expected no schedule promotion while schedule execution is running, got %+v", promoted)
	}
	stillQueued, err := service.GetExecution(ctx, "org_1", queued.ID)
	if err != nil {
		t.Fatalf("GetExecution queued returned error: %v", err)
	}
	if stillQueued.Status != ExecutionStatusQueued {
		t.Fatalf("queued schedule status=%s, want queued", stillQueued.Status)
	}
}

func TestServiceRefreshExecutionHealthMetricsRecordsActiveCountsAndOldestAge(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	now := time.Date(2026, 6, 6, 8, 30, 0, 0, time.UTC)

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Execution Health",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	createExecutionAt := func(orgID string, status ExecutionStatus, startedAt time.Time) {
		t.Helper()
		if _, err := store.CreateExecution(ctx, CreateExecutionRequest{
			OrganizationID: orgID,
			WorkflowID:     workflow.ID,
			Status:         status,
			StartedAt:      startedAt,
		}); err != nil {
			t.Fatalf("CreateExecution(%s, %s) returned error: %v", orgID, status, err)
		}
	}
	createExecutionAt("org_1", ExecutionStatusRunning, now.Add(-15*time.Minute))
	createExecutionAt("org_1", ExecutionStatusRunning, now.Add(-45*time.Minute))
	createExecutionAt("org_1", ExecutionStatusQueued, now.Add(-5*time.Minute))
	createExecutionAt("org_1", ExecutionStatusPaused, now.Add(-10*time.Minute))
	createExecutionAt("org_1", ExecutionStatusSucceeded, now.Add(-2*time.Hour))
	createExecutionAt("org_2", ExecutionStatusRunning, now.Add(-3*time.Hour))

	if err := service.RefreshExecutionHealthMetrics(ctx, "org_1", now); err != nil {
		t.Fatalf("RefreshExecutionHealthMetrics returned error: %v", err)
	}

	if got := testutil.ToFloat64(metrics.WorkflowExecutionActive.WithLabelValues("running")); got != 2 {
		t.Fatalf("expected running active count 2, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.WorkflowExecutionActiveAgeSeconds.WithLabelValues("running")); got != 2700 {
		t.Fatalf("expected running oldest age 2700 seconds, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.WorkflowExecutionActive.WithLabelValues("queued")); got != 1 {
		t.Fatalf("expected queued active count 1, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.WorkflowExecutionActiveAgeSeconds.WithLabelValues("queued")); got != 300 {
		t.Fatalf("expected queued oldest age 300 seconds, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.WorkflowExecutionActive.WithLabelValues("paused")); got != 1 {
		t.Fatalf("expected paused active count 1, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.WorkflowExecutionActiveAgeSeconds.WithLabelValues("paused")); got != 600 {
		t.Fatalf("expected paused oldest age 600 seconds, got %v", got)
	}
}

func TestServiceBuildExecutionDebugSnapshotDerivesTraceVariablesOutputsPerformanceAndLogs(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	startedAt := time.Date(2026, time.June, 4, 9, 0, 0, 0, time.UTC)
	firstCompletedAt := startedAt.Add(20 * time.Millisecond)
	secondStartedAt := startedAt.Add(25 * time.Millisecond)
	secondCompletedAt := startedAt.Add(55 * time.Millisecond)
	thirdStartedAt := startedAt.Add(60 * time.Millisecond)
	thirdCompletedAt := startedAt.Add(90 * time.Millisecond)
	workflowSnapshot := workflowDefinitionWithNodes("start", "notify", "archive")
	execution, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID:   "org_1",
		WorkflowID:       "workflow_1",
		WorkflowVersion:  3,
		Status:           ExecutionStatusFailed,
		Input:            map[string]any{"ticket": "INC-9"},
		Context:          map[string]any{"trigger": map[string]any{"type": "manual"}},
		WorkflowSnapshot: workflowSnapshot,
		StartedAt:        startedAt,
		CompletedAt:      &thirdCompletedAt,
		NodeExecutions: []CreateNodeExecutionRequest{
			{
				NodeID:      "start",
				NodeType:    "start",
				Status:      NodeStatusSucceeded,
				Attempt:     1,
				Input:       map[string]any{"ticket": "INC-9"},
				Output:      map[string]any{"ticket": "INC-9"},
				StartedAt:   startedAt,
				CompletedAt: &firstCompletedAt,
				DurationMS:  20,
			},
			{
				NodeID:      "notify",
				NodeType:    "http",
				Status:      NodeStatusSucceeded,
				Attempt:     1,
				Input:       map[string]any{"url": "https://tickets.example/INC-9"},
				Output:      map[string]any{"statusCode": float64(200)},
				StartedAt:   secondStartedAt,
				CompletedAt: &secondCompletedAt,
				DurationMS:  30,
			},
			{
				NodeID:      "archive",
				NodeType:    "http",
				Status:      NodeStatusFailed,
				Attempt:     1,
				Input:       map[string]any{"ticket": "INC-9"},
				Error:       map[string]any{"message": "archive endpoint unavailable"},
				StartedAt:   thirdStartedAt,
				CompletedAt: &thirdCompletedAt,
				DurationMS:  30,
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateExecution returned error: %v", err)
	}

	snapshot, err := service.BuildExecutionDebugSnapshot(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("BuildExecutionDebugSnapshot returned error: %v", err)
	}
	if snapshot.ExecutionID != execution.ID || snapshot.WorkflowID != "workflow_1" || snapshot.Status != ExecutionStatusFailed {
		t.Fatalf("unexpected snapshot identity: %+v", snapshot)
	}
	if snapshot.VariableSnapshot.Input["ticket"] != "INC-9" || snapshot.VariableSnapshot.Context["trigger"] == nil {
		t.Fatalf("expected input/context variable snapshot, got %+v", snapshot.VariableSnapshot)
	}
	if snapshot.VariableSnapshot.NodeOutputs["notify"]["statusCode"] != float64(200) || snapshot.Outputs["start"]["ticket"] != "INC-9" {
		t.Fatalf("expected node outputs snapshot, got variables=%+v outputs=%+v", snapshot.VariableSnapshot.NodeOutputs, snapshot.Outputs)
	}
	if len(snapshot.Trace) != 3 || snapshot.Trace[1].NodeID != "notify" || snapshot.Trace[1].Input["url"] != "https://tickets.example/INC-9" {
		t.Fatalf("expected trace entries from node executions, got %+v", snapshot.Trace)
	}
	if snapshot.Performance.TotalDurationMS != 90 || snapshot.Performance.NodeDurationsMS["notify"] != 30 || snapshot.Performance.BottleneckNodeID != "notify" {
		t.Fatalf("unexpected performance snapshot: %+v", snapshot.Performance)
	}
	if len(snapshot.Logs) != 3 {
		t.Fatalf("expected one timeline log per node execution, got %+v", snapshot.Logs)
	}
	if snapshot.Logs[0].Level != "info" || snapshot.Logs[0].NodeID != "start" || snapshot.Logs[0].Message != "Node start succeeded in 20ms" || !snapshot.Logs[0].Timestamp.Equal(firstCompletedAt) {
		t.Fatalf("unexpected successful node log: %+v", snapshot.Logs[0])
	}
	if snapshot.Logs[2].Level != "error" || snapshot.Logs[2].NodeID != "archive" || snapshot.Logs[2].Message != "Node archive failed in 30ms: archive endpoint unavailable" || !snapshot.Logs[2].Timestamp.Equal(thirdCompletedAt) {
		t.Fatalf("unexpected failed node log: %+v", snapshot.Logs[2])
	}
}

func TestServiceStartExecutionUsesDefaultOrganizationConcurrencyLimit(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Default Org Limit Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	for i := 0; i < defaultOrgMaxConcurrentWorkflows; i++ {
		store.addExecution("org_1", "other_workflow", ExecutionStatusRunning)
	}

	queued, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution at default org limit returned error: %v", err)
	}
	if queued.Status != ExecutionStatusQueued {
		t.Fatalf("StartExecution at default org limit status=%s, want queued", queued.Status)
	}
}

func TestServiceStartExecutionRejectsWhenSystemConcurrencyLimitIsReached(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithSystemWorkflowLimits(SystemWorkflowLimits{
		MaxConcurrentWorkflows: 2,
	}))
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_3",
		Name:           "System Limit Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	store.addExecution("org_1", "other_workflow_a", ExecutionStatusRunning)
	store.addExecution("org_2", "other_workflow_b", ExecutionStatusRunning)

	_, err = service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_3", WorkflowID: workflow.ID})
	if !errors.Is(err, ErrWorkflowConcurrencyLimit) {
		t.Fatalf("StartExecution over system concurrency err=%v, want ErrWorkflowConcurrencyLimit", err)
	}
	if got := len(store.executions); got != 2 {
		t.Fatalf("expected system concurrency rejection not to persist a new execution, got %d executions", got)
	}
}

func TestServiceStartExecutionRejectsWhenGlobalExecutionsPerMinuteLimitIsReached(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithSystemWorkflowLimits(SystemWorkflowLimits{
		MaxExecutionsPerMinute: 1,
	}))
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Minute Limit Flow",
		Definition:     workflowDefinitionWithNodes("start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if _, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID}); err != nil {
		t.Fatalf("StartExecution first returned error: %v", err)
	}

	_, err = service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if !errors.Is(err, ErrWorkflowConcurrencyLimit) {
		t.Fatalf("StartExecution over global executions/minute err=%v, want ErrWorkflowConcurrencyLimit", err)
	}
	if got := len(store.executions); got != 1 {
		t.Fatalf("expected global minute rejection not to persist a new execution, got %d executions", got)
	}
}

func TestServiceCheckResourceLimitsTimesOutLongRunningExecution(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Timed Flow",
		Definition: workflowDefinitionWithLimits(map[string]any{
			"max_execution_duration_seconds": float64(60),
		}, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	startedAt := time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC)
	store.executions[execution.ID].StartedAt = startedAt

	checked, err := service.CheckResourceLimits(ctx, "org_1", execution.ID, WorkflowResourceUsage{
		Now: startedAt.Add(61 * time.Second),
	})
	if !errors.Is(err, ErrWorkflowResourceLimit) {
		t.Fatalf("CheckResourceLimits timeout err=%v, want ErrWorkflowResourceLimit", err)
	}
	if checked.Status != ExecutionStatusTimedOut || checked.CompletedAt == nil {
		t.Fatalf("CheckResourceLimits timeout got %+v, want timeout with completion time", checked)
	}
}

func TestServiceCheckResourceLimitsPausesWhenTokenBudgetIsExceeded(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Budgeted Flow",
		Definition: workflowDefinitionWithLimits(map[string]any{
			"max_tokens_budget": float64(100),
		}, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	checked, err := service.CheckResourceLimits(ctx, "org_1", execution.ID, WorkflowResourceUsage{TotalTokens: 101})
	if !errors.Is(err, ErrWorkflowResourceLimit) {
		t.Fatalf("CheckResourceLimits token budget err=%v, want ErrWorkflowResourceLimit", err)
	}
	if checked.Status != ExecutionStatusPaused || checked.CompletedAt != nil {
		t.Fatalf("CheckResourceLimits token budget got %+v, want paused without completion time", checked)
	}
	guardNodes := workflowNodeExecutionsByID(checked.NodeExecutions, "workflow_resource_guard")
	if len(guardNodes) != 1 {
		t.Fatalf("expected resource guard node execution, got %+v", guardNodes)
	}
	guard := guardNodes[0]
	if guard.Status != NodeStatusFailed || guard.NodeType != "resource_guard" {
		t.Fatalf("expected failed resource guard node, got %+v", guard)
	}
	if guard.Error["code"] != "token_budget_exceeded" || guard.Error["maxTokensBudget"] != 100 || guard.Error["totalTokens"] != 101 {
		t.Fatalf("expected token budget error payload, got %+v", guard.Error)
	}
	if guard.Context["pauseReason"] != "token_budget_exceeded" {
		t.Fatalf("expected pause reason context, got %+v", guard.Context)
	}
}

func TestServiceCheckResourceLimitsStopsWhenNodeExecutionLimitIsExceeded(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Loop Guard Flow",
		Definition: workflowDefinitionWithLimits(map[string]any{
			"max_node_executions": float64(1),
		}, "start"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	checked, err := service.CheckResourceLimits(ctx, "org_1", execution.ID, WorkflowResourceUsage{NodeExecutionCount: 2})
	if !errors.Is(err, ErrWorkflowResourceLimit) {
		t.Fatalf("CheckResourceLimits node count err=%v, want ErrWorkflowResourceLimit", err)
	}
	if checked.Status != ExecutionStatusMaxIterations || checked.CompletedAt == nil {
		t.Fatalf("CheckResourceLimits node count got %+v, want max_iterations with completion time", checked)
	}
}

func TestServiceListsAndGetsExecutions(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	execution := store.addExecution("org_1", "workflow_1", ExecutionStatusRunning)
	store.addExecution("org_other", "workflow_1", ExecutionStatusRunning)
	store.addExecution("org_1", "workflow_2", ExecutionStatusRunning)

	executions, err := service.ListExecutions(ctx, "org_1", "workflow_1")
	if err != nil {
		t.Fatalf("ListExecutions returned error: %v", err)
	}
	if len(executions) != 1 || executions[0].ID != execution.ID {
		t.Fatalf("ListExecutions got %+v, want only %s", executions, execution.ID)
	}

	got, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	if got.ID != execution.ID {
		t.Fatalf("GetExecution ID=%q, want %q", got.ID, execution.ID)
	}
	if _, err := service.GetExecution(ctx, "org_1", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetExecution missing err=%v, want ErrNotFound", err)
	}
}

func TestServiceExecutionLifecycleTransitions(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	execution := store.addExecution("org_1", "workflow_1", ExecutionStatusRunning)

	paused, err := service.PauseExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("PauseExecution returned error: %v", err)
	}
	if paused.Status != ExecutionStatusPaused {
		t.Fatalf("PauseExecution status=%s, want paused", paused.Status)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if resumed.Status != ExecutionStatusRunning {
		t.Fatalf("ResumeExecution status=%s, want running", resumed.Status)
	}

	cancelled, err := service.CancelExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("CancelExecution returned error: %v", err)
	}
	if cancelled.Status != ExecutionStatusCancelled || cancelled.CompletedAt == nil {
		t.Fatalf("CancelExecution got %+v, want cancelled with completion time", cancelled)
	}
}

func TestServiceRecordsWorkflowExecutionMetricsWhenExecutionReachesTerminalStatus(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	execution := store.addExecution("org_1", "workflow_1", ExecutionStatusRunning)

	before := testutil.ToFloat64(metrics.WorkflowExecutionTotal.WithLabelValues(string(ExecutionStatusCancelled)))
	if _, err := service.CancelExecution(ctx, "org_1", execution.ID); err != nil {
		t.Fatalf("CancelExecution returned error: %v", err)
	}
	after := testutil.ToFloat64(metrics.WorkflowExecutionTotal.WithLabelValues(string(ExecutionStatusCancelled)))
	if after != before+1 {
		t.Fatalf("expected workflow execution metric increment, before=%v after=%v", before, after)
	}
	if count := testutil.CollectAndCount(metrics.WorkflowExecutionDurationSeconds, "workflow_execution_duration_seconds"); count == 0 {
		t.Fatal("expected workflow execution duration metric to be collectable")
	}
}

func TestServiceRejectsInvalidLifecycleTransitions(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	if _, err := service.PauseExecution(ctx, "org_1", store.addExecution("org_1", "workflow_1", ExecutionStatusPaused).ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("PauseExecution from paused err=%v, want ErrInvalidTransition", err)
	}
	if _, err := service.ResumeExecution(ctx, "org_1", store.addExecution("org_1", "workflow_1", ExecutionStatusRunning).ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("ResumeExecution from running err=%v, want ErrInvalidTransition", err)
	}
	if _, err := service.CancelExecution(ctx, "org_1", store.addExecution("org_1", "workflow_1", ExecutionStatusFailed).ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("CancelExecution from failed err=%v, want ErrInvalidTransition", err)
	}
	if _, err := service.CancelExecution(ctx, "org_1", store.addExecution("org_1", "workflow_1", ExecutionStatusSucceeded).ID); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("CancelExecution from succeeded err=%v, want ErrInvalidTransition", err)
	}
}

func TestServiceRecordNodeStatusValidatesExecutionAndNode(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	execution := store.addExecution("org_1", "workflow_1", ExecutionStatusRunning)

	node, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID:   "call_agent",
		NodeType: "agent",
		Status:   NodeStatusRunning,
		Attempt:  1,
		Input:    map[string]any{"prompt": "hello"},
	})
	if err != nil {
		t.Fatalf("RecordNodeStatus returned error: %v", err)
	}
	if node.ExecutionID != execution.ID || node.NodeID != "call_agent" || node.Status != NodeStatusRunning {
		t.Fatalf("unexpected node execution: %+v", node)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{Status: NodeStatusRunning}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RecordNodeStatus without node ID err=%v, want ErrInvalidInput", err)
	}
	if _, err := service.RecordNodeStatus(ctx, "org_1", "missing", RecordNodeStatusRequest{NodeID: "call_agent", Status: NodeStatusRunning}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RecordNodeStatus missing execution err=%v, want ErrNotFound", err)
	}
}

func TestServiceRecordsWorkflowNodeErrorRateOnFailedNodeStatus(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Error Rate Flow",
		Definition:     workflowDefinitionWithNodes("call_agent", "call_agent_retry"),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution := store.addExecution("org_1", workflow.ID, ExecutionStatusRunning)

	metricNodeType := "agent_error_rate_regression"
	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID:   "call_agent",
		NodeType: metricNodeType,
		Status:   NodeStatusFailed,
		Attempt:  1,
	}); err != nil {
		t.Fatalf("RecordNodeStatus returned error: %v", err)
	}

	if got := testutil.ToFloat64(metrics.WorkflowNodeErrorRate.WithLabelValues(metricNodeType)); got != 1 {
		t.Fatalf("expected workflow node error rate for failed agent node to be 1, got %v", got)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID:   "call_agent_retry",
		NodeType: metricNodeType,
		Status:   NodeStatus("succeeded"),
		Attempt:  1,
	}); err != nil {
		t.Fatalf("RecordNodeStatus retry returned error: %v", err)
	}

	if got := testutil.ToFloat64(metrics.WorkflowNodeErrorRate.WithLabelValues(metricNodeType)); got != 0.5 {
		t.Fatalf("expected workflow node error rate to aggregate failures over attempts, got %v", got)
	}
}

func TestServiceRecordNodeStatusSeedsReadyDownstreamNodes(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Serial Workflow",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "manual"},
				{"id": "notify", "type": "agent"},
			},
			[]map[string]any{
				{"from": "start", "to": "notify"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID:   "start",
		NodeType: "manual",
		Status:   NodeStatus("succeeded"),
		Attempt:  1,
		Output:   map[string]any{"ticket": "INC-1"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus returned error: %v", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	notifyNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "notify")
	if len(notifyNodes) != 1 {
		t.Fatalf("expected one notify node to be seeded, got execution nodes %+v", updated.NodeExecutions)
	}
	if notifyNodes[0].Status != NodeStatusPending || notifyNodes[0].Attempt != 1 || notifyNodes[0].NodeType != "agent" {
		t.Fatalf("unexpected seeded notify node: %+v", notifyNodes[0])
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "start",
		Status: NodeStatus("succeeded"),
	}); err != nil {
		t.Fatalf("RecordNodeStatus second success returned error: %v", err)
	}
	updated, err = service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution after duplicate success returned error: %v", err)
	}
	if notifyNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "notify"); len(notifyNodes) != 1 {
		t.Fatalf("expected notify node not to be duplicated, got %+v", notifyNodes)
	}
}

func TestServiceRecordNodeStatusWaitsForAllParentsBeforeSeedingJoin(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Join Workflow",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "manual_review", "type": "manual"},
				{"id": "agent_check", "type": "agent"},
				{"id": "join", "type": "join"},
			},
			[]map[string]any{
				{"from": "manual_review", "to": "join"},
				{"from": "agent_check", "to": "join"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-2"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "manual_review",
		Status: NodeStatus("succeeded"),
	}); err != nil {
		t.Fatalf("RecordNodeStatus manual returned error: %v", err)
	}
	afterFirstParent, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution after first parent returned error: %v", err)
	}
	if joinNodes := workflowNodeExecutionsByID(afterFirstParent.NodeExecutions, "join"); len(joinNodes) != 0 {
		t.Fatalf("expected join not to seed before all parents, got %+v", joinNodes)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "agent_check",
		Status: NodeStatus("succeeded"),
	}); err != nil {
		t.Fatalf("RecordNodeStatus agent returned error: %v", err)
	}
	afterSecondParent, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution after second parent returned error: %v", err)
	}
	joinNodes := workflowNodeExecutionsByID(afterSecondParent.NodeExecutions, "join")
	if len(joinNodes) != 1 || joinNodes[0].Status != NodeStatusPending || joinNodes[0].NodeType != "join" {
		t.Fatalf("expected one pending join node after all parents, got %+v", joinNodes)
	}
}

func TestServiceRecordNodeStatusSeedsOnlyMatchedConditionBranch(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Conditional Branch Workflow",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "check_priority", "type": "condition"},
				{"id": "escalate", "type": "http"},
				{"id": "archive", "type": "manual"},
			},
			[]map[string]any{
				{"from": "check_priority", "to": "escalate", "branch": "true"},
				{"from": "check_priority", "to": "archive", "branch": "false"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"priority": "high"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID:   "check_priority",
		NodeType: "condition",
		Status:   NodeStatusSucceeded,
		Attempt:  1,
		Output:   map[string]any{"matched": true, "branch": "true"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus condition returned error: %v", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	if nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "escalate"); len(nodes) != 1 || nodes[0].Status != NodeStatusPending {
		t.Fatalf("expected true branch escalate node to be seeded, got %+v", nodes)
	}
	if nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "archive"); len(nodes) != 0 {
		t.Fatalf("expected false branch archive node not to be seeded, got %+v", nodes)
	}
}

func TestServiceRecordNodeStatusAppliesFailurePolicies(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Failure Policies",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "must_review", "type": "manual"},
				{"id": "optional_lookup", "type": "knowledge", "failurePolicy": map[string]any{"strategy": "skip_on_failure"}},
				{"id": "charge_card", "type": "http", "failurePolicy": map[string]any{"strategy": "failure_branch", "failureBranchNodeId": "notify_ops"}},
				{"id": "notify_ops", "type": "agent"},
			},
			[]map[string]any{
				{"from": "charge_card", "to": "notify_ops"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "must_review",
		Status: NodeStatusFailed,
		Error:  map[string]any{"message": "manual decision required"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus pause policy returned error: %v", err)
	}
	paused, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution paused returned error: %v", err)
	}
	if paused.Status != ExecutionStatusPaused {
		t.Fatalf("expected pause_on_failure to pause execution, got %s", paused.Status)
	}

	execution = store.addWorkflowExecution(ctx, workflow.ID, "org_1", workflow.Definition, ExecutionStatusRunning)
	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "optional_lookup",
		Status: NodeStatusFailed,
		Error:  map[string]any{"message": "knowledge timeout"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus skip policy returned error: %v", err)
	}
	skipped, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution skipped returned error: %v", err)
	}
	if skipped.Status != ExecutionStatusPartialSuccess {
		t.Fatalf("expected skip_on_failure to mark partial success, got %s", skipped.Status)
	}
	if nodes := workflowNodeExecutionsByID(skipped.NodeExecutions, "optional_lookup"); len(nodes) == 0 || nodes[len(nodes)-1].Status != NodeStatusSkipped {
		t.Fatalf("expected skipped node record, got %+v", nodes)
	}

	execution = store.addWorkflowExecution(ctx, workflow.ID, "org_1", workflow.Definition, ExecutionStatusRunning)
	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "charge_card",
		Status: NodeStatusFailed,
		Error:  map[string]any{"message": "payment declined"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus branch policy returned error: %v", err)
	}
	branched, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution branch returned error: %v", err)
	}
	if branched.Status != ExecutionStatusRunning {
		t.Fatalf("expected failure_branch to keep execution running, got %s", branched.Status)
	}
	if nodes := workflowNodeExecutionsByID(branched.NodeExecutions, "notify_ops"); len(nodes) == 0 || nodes[len(nodes)-1].Status != NodeStatusPending {
		t.Fatalf("expected failure branch node to be seeded, got %+v", nodes)
	}
}

func TestServiceResolvePausedFailureDecisionRetriesSkipsAndTerminates(t *testing.T) {
	ctx := context.Background()

	t.Run("retry with edited input", func(t *testing.T) {
		store := newMemoryWorkflowStore()
		service := NewService(store)
		_, execution := createPausedFailureExecution(t, ctx, service, store)

		resolved, err := service.ResolvePausedFailure(ctx, "org_1", execution.ID, ResolveFailureDecisionRequest{
			Action: FailureActionRetry,
			Input:  map[string]any{"ticket": "INC-9", "priority": "urgent"},
			NodeID: "must_review",
		})
		if err != nil {
			t.Fatalf("ResolvePausedFailure retry returned error: %v", err)
		}
		if resolved.Status != ExecutionStatusRunning {
			t.Fatalf("expected retry decision to resume execution, got %s", resolved.Status)
		}
		nodes := workflowNodeExecutionsByID(resolved.NodeExecutions, "must_review")
		if len(nodes) < 2 || nodes[len(nodes)-1].Status != NodeStatusPending {
			t.Fatalf("expected pending retry node after failed node, got %+v", nodes)
		}
		if nodes[len(nodes)-1].Attempt != 2 {
			t.Fatalf("expected retry attempt 2, got %+v", nodes[len(nodes)-1])
		}
		if nodes[len(nodes)-1].Input["priority"] != "urgent" {
			t.Fatalf("expected edited retry input, got %+v", nodes[len(nodes)-1].Input)
		}
	})

	t.Run("skip failed node", func(t *testing.T) {
		store := newMemoryWorkflowStore()
		service := NewService(store)
		_, execution := createPausedFailureExecution(t, ctx, service, store)

		resolved, err := service.ResolvePausedFailure(ctx, "org_1", execution.ID, ResolveFailureDecisionRequest{
			Action: FailureActionContinue,
			NodeID: "must_review",
		})
		if err != nil {
			t.Fatalf("ResolvePausedFailure skip returned error: %v", err)
		}
		if resolved.Status != ExecutionStatusRunning {
			t.Fatalf("expected skip decision to continue execution, got %s", resolved.Status)
		}
		nodes := workflowNodeExecutionsByID(resolved.NodeExecutions, "must_review")
		if len(nodes) < 2 || nodes[len(nodes)-1].Status != NodeStatusSkipped {
			t.Fatalf("expected skipped node after failed node, got %+v", nodes)
		}
		notifyNodes := workflowNodeExecutionsByID(resolved.NodeExecutions, "notify")
		if len(notifyNodes) != 1 || notifyNodes[0].Status != NodeStatusPending {
			t.Fatalf("expected downstream notify to be seeded, got %+v", notifyNodes)
		}
	})

	t.Run("terminate workflow", func(t *testing.T) {
		store := newMemoryWorkflowStore()
		service := NewService(store)
		_, execution := createPausedFailureExecution(t, ctx, service, store)

		resolved, err := service.ResolvePausedFailure(ctx, "org_1", execution.ID, ResolveFailureDecisionRequest{
			Action: FailureActionFail,
			NodeID: "must_review",
		})
		if err != nil {
			t.Fatalf("ResolvePausedFailure terminate returned error: %v", err)
		}
		if resolved.Status != ExecutionStatusFailed || resolved.CompletedAt == nil {
			t.Fatalf("expected terminated workflow to be failed with completion time, got %+v", resolved)
		}
	})
}

func createPausedFailureExecution(t *testing.T, ctx context.Context, service *Service, store *memoryWorkflowStore) (*WorkflowDefinition, *WorkflowExecution) {
	t.Helper()

	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Paused failure workflow",
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "must_review", "type": "manual"},
				{"id": "notify", "type": "manual"},
			},
			[]map[string]any{
				{"from": "must_review", "to": "notify"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution := store.addWorkflowExecution(ctx, workflow.ID, "org_1", workflow.Definition, ExecutionStatusRunning)
	if _, err := service.RecordNodeStatus(ctx, "org_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "must_review",
		Status: NodeStatusFailed,
		Input:  map[string]any{"ticket": "INC-9"},
		Error:  map[string]any{"message": "manual decision required"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus pause policy returned error: %v", err)
	}
	paused, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	if paused.Status != ExecutionStatusPaused {
		t.Fatalf("expected paused execution fixture, got %s", paused.Status)
	}
	return workflow, paused
}

func workflowNodeExecutionsByID(nodes []WorkflowNodeExecution, nodeID string) []WorkflowNodeExecution {
	matches := []WorkflowNodeExecution{}
	for _, node := range nodes {
		if node.NodeID == nodeID {
			matches = append(matches, node)
		}
	}
	return matches
}

func assertWorkflowErrorContext(t *testing.T, value any, wantNodeID, wantMessage string) {
	t.Helper()
	errCtx, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected workflow.error context map, got %#v", value)
	}
	if errCtx["nodeId"] != wantNodeID && errCtx["node_id"] != wantNodeID {
		t.Fatalf("expected workflow.error node ID %q, got %#v", wantNodeID, errCtx)
	}
	if errCtx["message"] != wantMessage {
		t.Fatalf("expected workflow.error message %q, got %#v", wantMessage, errCtx)
	}
}

type recordingSemanticTriggerMatcher struct {
	calls       int
	decision    SemanticTriggerMatchDecision
	lastRequest SemanticTriggerMatchRequest
}

type recordingWorkflowScheduleSyncer struct {
	requests []WorkflowScheduleSyncRequest
	err      error
}

func (s *recordingWorkflowScheduleSyncer) SyncWorkflowScheduleTriggers(ctx context.Context, req WorkflowScheduleSyncRequest) error {
	s.requests = append(s.requests, req)
	return s.err
}

func (m *recordingSemanticTriggerMatcher) MatchSemanticTrigger(ctx context.Context, req SemanticTriggerMatchRequest) (SemanticTriggerMatchDecision, error) {
	m.calls++
	m.lastRequest = req
	return m.decision, nil
}

type recordingWorkflowEmbedder struct {
	embeddings     map[string][]float32
	userID         string
	organizationID string
}

func (e *recordingWorkflowEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.userID, _ = relaytypes.TrustedUserIDFromContext(ctx)
	e.organizationID, _ = relaytypes.TrustedOrganizationIDFromContext(ctx)
	if embedding, ok := e.embeddings[text]; ok {
		return embedding, nil
	}
	return []float32{}, nil
}

func (e *recordingWorkflowEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.userID, _ = relaytypes.TrustedUserIDFromContext(ctx)
	e.organizationID, _ = relaytypes.TrustedOrganizationIDFromContext(ctx)
	embeddings := make([][]float32, 0, len(texts))
	for _, text := range texts {
		if embedding, ok := e.embeddings[text]; ok {
			embeddings = append(embeddings, embedding)
			continue
		}
		embeddings = append(embeddings, []float32{})
	}
	return embeddings, nil
}

func workflowDefinitionWithNodes(ids ...string) map[string]any {
	nodes := make([]any, 0, len(ids))
	for _, id := range ids {
		nodes = append(nodes, map[string]any{"id": id, "type": "manual"})
	}
	return map[string]any{"nodes": nodes}
}

func workflowDefinitionWithSemanticTrigger(triggerID string, keywords []string, threshold float64, ids ...string) map[string]any {
	definition := workflowDefinitionWithNodes(ids...)
	keywordValues := make([]any, 0, len(keywords))
	for _, keyword := range keywords {
		keywordValues = append(keywordValues, keyword)
	}
	definition["triggers"] = map[string]any{
		"semantic": []any{
			map[string]any{
				"id":                triggerID,
				"keywords":          keywordValues,
				"semanticThreshold": threshold,
			},
		},
	}
	return definition
}

func workflowDefinitionWithScheduleTrigger(triggerID, cronExpression string, enabled bool, ids ...string) map[string]any {
	trigger := map[string]any{
		"id":      triggerID,
		"enabled": enabled,
	}
	if cronExpression != "" {
		trigger["cron"] = cronExpression
	}
	return workflowDefinitionWithScheduleTriggers([]map[string]any{trigger}, ids...)
}

func workflowDefinitionWithScheduleTriggers(triggers []map[string]any, ids ...string) map[string]any {
	definition := workflowDefinitionWithNodes(ids...)
	triggerValues := make([]any, 0, len(triggers))
	for _, trigger := range triggers {
		triggerValues = append(triggerValues, trigger)
	}
	definition["triggers"] = map[string]any{"schedule": triggerValues}
	return definition
}

func workflowDefinitionWithConversationTrigger(conversationID, triggerID string, ids ...string) map[string]any {
	definition := workflowDefinitionWithNodes(ids...)
	definition["triggers"] = map[string]any{
		"conversation": map[string]any{
			"id":             triggerID,
			"conversationId": conversationID,
		},
	}
	return definition
}

func workflowDefinitionWithLimits(limits map[string]any, ids ...string) map[string]any {
	definition := workflowDefinitionWithNodes(ids...)
	for key, value := range limits {
		definition[key] = value
	}
	return definition
}

func workflowDefinitionDAG(nodes []map[string]any, edges []map[string]any) map[string]any {
	nodeValues := make([]any, 0, len(nodes))
	for _, node := range nodes {
		nodeValues = append(nodeValues, node)
	}
	edgeValues := make([]any, 0, len(edges))
	for _, edge := range edges {
		edgeValues = append(edgeValues, edge)
	}
	return map[string]any{"nodes": nodeValues, "edges": edgeValues}
}

func stringPtr(value string) *string { return &value }

func workflowStatusPtr(value WorkflowStatus) *WorkflowStatus { return &value }

type memoryWorkflowStore struct {
	workflows  map[string]*WorkflowDefinition
	versions   map[string][]*WorkflowDefinition
	executions map[string]*WorkflowExecution
	nodes      map[string][]WorkflowNodeExecution
	nextID     int
}

func newMemoryWorkflowStore() *memoryWorkflowStore {
	return &memoryWorkflowStore{
		workflows:  map[string]*WorkflowDefinition{},
		versions:   map[string][]*WorkflowDefinition{},
		executions: map[string]*WorkflowExecution{},
		nodes:      map[string][]WorkflowNodeExecution{},
		nextID:     1,
	}
}

func (s *memoryWorkflowStore) CreateWorkflow(_ context.Context, req CreateWorkflowRequest) (*WorkflowDefinition, error) {
	id := s.newID("workflow")
	now := time.Now().UTC()
	workflow := &WorkflowDefinition{
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
	if workflow.Status == "" {
		workflow.Status = WorkflowStatusDraft
	}
	if workflow.Version <= 0 {
		workflow.Version = 1
	}
	s.workflows[id] = cloneWorkflowDefinition(workflow)
	s.versions[id] = append(s.versions[id], cloneWorkflowDefinition(workflow))
	return cloneWorkflowDefinition(workflow), nil
}

func (s *memoryWorkflowStore) GetWorkflow(_ context.Context, organizationID, id string) (*WorkflowDefinition, error) {
	workflow := s.workflows[id]
	if workflow == nil || workflow.OrganizationID != organizationID {
		return nil, nil
	}
	return cloneWorkflowDefinition(workflow), nil
}

func (s *memoryWorkflowStore) UpdateWorkflow(_ context.Context, req UpdateWorkflowStoreRequest) (*WorkflowDefinition, error) {
	workflow := s.workflows[req.WorkflowID]
	if workflow == nil || workflow.OrganizationID != req.OrganizationID {
		return nil, nil
	}
	workflow.Version = s.nextWorkflowVersion(req.WorkflowID)
	workflow.Name = req.Name
	workflow.Description = req.Description
	workflow.Status = req.Status
	workflow.Definition = req.Definition
	workflow.Variables = req.Variables
	workflow.UpdatedAt = time.Now().UTC()
	s.versions[req.WorkflowID] = append(s.versions[req.WorkflowID], cloneWorkflowDefinition(workflow))
	return cloneWorkflowDefinition(workflow), nil
}

func (s *memoryWorkflowStore) ListWorkflows(_ context.Context, organizationID string) ([]*WorkflowDefinition, error) {
	workflows := []*WorkflowDefinition{}
	for _, workflow := range s.workflows {
		if workflow.OrganizationID == organizationID {
			workflows = append(workflows, cloneWorkflowDefinition(workflow))
		}
	}
	return workflows, nil
}

func (s *memoryWorkflowStore) ListWorkflowVersions(_ context.Context, organizationID, workflowID string) ([]*WorkflowDefinition, error) {
	versions := []*WorkflowDefinition{}
	for _, version := range s.versions[workflowID] {
		if version.OrganizationID == organizationID {
			versions = append(versions, cloneWorkflowDefinition(version))
		}
	}
	return versions, nil
}

func (s *memoryWorkflowStore) GetWorkflowVersion(_ context.Context, organizationID, workflowID string, versionNumber int) (*WorkflowDefinition, error) {
	for _, version := range s.versions[workflowID] {
		if version.OrganizationID == organizationID && version.Version == versionNumber {
			return cloneWorkflowDefinition(version), nil
		}
	}
	return nil, nil
}

func (s *memoryWorkflowStore) CreateExecution(_ context.Context, req CreateExecutionRequest) (*WorkflowExecution, error) {
	id := s.newID("wexec")
	now := time.Now().UTC()
	execution := &WorkflowExecution{
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
	if execution.WorkflowVersion <= 0 {
		execution.WorkflowVersion = 1
	}
	if execution.Status == "" {
		execution.Status = ExecutionStatusRunning
	}
	if execution.StartedAt.IsZero() {
		execution.StartedAt = now
	}
	s.executions[id] = cloneWorkflowExecution(execution)
	for _, nodeReq := range req.NodeExecutions {
		node, _ := s.CreateNodeExecution(context.Background(), req.OrganizationID, id, nodeReq)
		if node != nil {
			execution.NodeExecutions = append(execution.NodeExecutions, *node)
		}
	}
	return cloneWorkflowExecution(execution), nil
}

func (s *memoryWorkflowStore) ListExecutions(_ context.Context, organizationID, workflowID string) ([]*WorkflowExecution, error) {
	executions := []*WorkflowExecution{}
	for _, execution := range s.executions {
		if execution.OrganizationID == organizationID && execution.WorkflowID == workflowID {
			executions = append(executions, cloneWorkflowExecution(execution))
		}
	}
	return executions, nil
}

func (s *memoryWorkflowStore) GetExecution(_ context.Context, organizationID, id string) (*WorkflowExecution, error) {
	execution := s.executions[id]
	if execution == nil || execution.OrganizationID != organizationID {
		return nil, nil
	}
	cloned := cloneWorkflowExecution(execution)
	cloned.NodeExecutions = append([]WorkflowNodeExecution(nil), s.nodes[id]...)
	return cloned, nil
}

func (s *memoryWorkflowStore) ListActiveExecutionHealth(_ context.Context, organizationID string, statuses []ExecutionStatus) ([]WorkflowExecutionHealthSummary, error) {
	statusSet := map[ExecutionStatus]struct{}{}
	for _, status := range statuses {
		statusSet[status] = struct{}{}
	}
	summariesByStatus := map[ExecutionStatus]WorkflowExecutionHealthSummary{}
	for _, execution := range s.executions {
		if organizationID != "" && execution.OrganizationID != organizationID {
			continue
		}
		if _, ok := statusSet[execution.Status]; !ok {
			continue
		}
		summary := summariesByStatus[execution.Status]
		summary.Status = execution.Status
		summary.Count++
		if summary.OldestStartedAt.IsZero() || execution.StartedAt.Before(summary.OldestStartedAt) {
			summary.OldestStartedAt = execution.StartedAt
		}
		summariesByStatus[execution.Status] = summary
	}
	summaries := []WorkflowExecutionHealthSummary{}
	for _, summary := range summariesByStatus {
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *memoryWorkflowStore) CountRunningExecutions(_ context.Context, organizationID, workflowID string) (int, error) {
	count := 0
	for _, execution := range s.executions {
		if execution.OrganizationID == organizationID && execution.WorkflowID == workflowID && execution.Status == ExecutionStatusRunning {
			count++
		}
	}
	return count, nil
}

func (s *memoryWorkflowStore) CountRunningExecutionsForOrganization(_ context.Context, organizationID string) (int, error) {
	count := 0
	for _, execution := range s.executions {
		if execution.OrganizationID == organizationID && execution.Status == ExecutionStatusRunning {
			count++
		}
	}
	return count, nil
}

func (s *memoryWorkflowStore) UpdateExecutionStatus(_ context.Context, organizationID, id string, status ExecutionStatus, completedAt *time.Time) (*WorkflowExecution, error) {
	execution := s.executions[id]
	if execution == nil || execution.OrganizationID != organizationID {
		return nil, nil
	}
	execution.Status = status
	execution.CompletedAt = completedAt
	execution.UpdatedAt = time.Now().UTC()
	return cloneWorkflowExecution(execution), nil
}

func (s *memoryWorkflowStore) CreateNodeExecution(_ context.Context, organizationID, executionID string, req CreateNodeExecutionRequest) (*WorkflowNodeExecution, error) {
	execution := s.executions[executionID]
	if execution == nil || execution.OrganizationID != organizationID {
		return nil, nil
	}
	now := time.Now().UTC()
	node := WorkflowNodeExecution{
		ID:             s.newID("wnode"),
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
		node.Status = NodeStatusPending
	}
	if node.StartedAt.IsZero() {
		node.StartedAt = now
	}
	s.nodes[executionID] = append(s.nodes[executionID], node)
	return &node, nil
}

func (s *memoryWorkflowStore) addExecution(organizationID, workflowID string, status ExecutionStatus) *WorkflowExecution {
	execution, _ := s.CreateExecution(context.Background(), CreateExecutionRequest{
		OrganizationID: organizationID,
		WorkflowID:     workflowID,
		Status:         status,
	})
	return execution
}

func (s *memoryWorkflowStore) addWorkflowExecution(ctx context.Context, workflowID, organizationID string, definition map[string]any, status ExecutionStatus) *WorkflowExecution {
	nodes, _ := startNodeExecutionsForDefinition(definition, nil)
	execution, _ := s.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID:   organizationID,
		WorkflowID:       workflowID,
		Status:           status,
		WorkflowSnapshot: definition,
		NodeExecutions:   nodes,
	})
	return execution
}

func (s *memoryWorkflowStore) newID(prefix string) string {
	id := prefix + "_test_" + string(rune('a'+s.nextID))
	s.nextID++
	return id
}

func (s *memoryWorkflowStore) nextWorkflowVersion(workflowID string) int {
	nextVersion := 1
	for _, version := range s.versions[workflowID] {
		if version.Version >= nextVersion {
			nextVersion = version.Version + 1
		}
	}
	return nextVersion
}

func cloneWorkflowDefinition(workflow *WorkflowDefinition) *WorkflowDefinition {
	cloned := *workflow
	return &cloned
}

func cloneWorkflowExecution(execution *WorkflowExecution) *WorkflowExecution {
	cloned := *execution
	cloned.NodeExecutions = append([]WorkflowNodeExecution(nil), execution.NodeExecutions...)
	return &cloned
}
