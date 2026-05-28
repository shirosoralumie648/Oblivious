package agent

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"oblivious/server/internal/chat"
)

func testAgentRunSQLStore(t *testing.T) (*SQLStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed agent run tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed agent run tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock agent run test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock agent run test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS agent_tool_runs CASCADE`,
		`DROP TABLE IF EXISTS agent_runs CASCADE`,
		`DROP TABLE IF EXISTS agent_messages CASCADE`,
		`DROP TABLE IF EXISTS agent_conversations CASCADE`,
		`DROP TABLE IF EXISTS agents CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', name TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agents (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL, description TEXT, model TEXT DEFAULT 'gpt-4o-mini', system_prompt TEXT, tools JSONB DEFAULT '[]', config JSONB DEFAULT '{}', is_public BOOLEAN DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_conversations (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, role TEXT NOT NULL, content TEXT NOT NULL, tool_calls JSONB DEFAULT '[]', tool_call_id TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`INSERT INTO users (id, email, password_hash, name) VALUES ('user_1', 'agent-run-user@example.com', 'hash', 'Agent Run User'), ('user_2', 'agent-run-other@example.com', 'hash', 'Other User')`,
		`INSERT INTO organizations (id, slug, name) VALUES ('org_1', 'agent-run-org-1', 'Agent Run Org 1'), ('org_2', 'agent-run-org-2', 'Agent Run Org 2')`,
		`INSERT INTO agents (id, user_id, organization_id, name, tools) VALUES ('agent_1', 'user_1', 'org_1', 'Durable Agent', '[{"name":"datetime","type":"builtin","enabled":true}]'::jsonb), ('agent_2', 'user_2', 'org_2', 'Other Durable Agent', '[]'::jsonb)`,
		`INSERT INTO agent_conversations (id, agent_id, user_id, organization_id, title) VALUES ('conv_1', 'agent_1', 'user_1', 'org_1', 'Durable Conversation'), ('conv_2', 'agent_2', 'user_2', 'org_2', 'Other Conversation')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare agent run database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0031_agent_workflow_runs.sql")
	if err != nil {
		t.Fatalf("read agent workflow migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply agent workflow migration: %v", err)
	}

	return NewSQLStore(database), context.Background()
}

func TestAgentRunStorePersistsRunLifecycle(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID:    "org_1",
		ConversationID:    "conv_1",
		AgentID:           "agent_1",
		UserID:            "user_1",
		RequestID:         "req_agent_run_lifecycle",
		Status:            RunStatusRunning,
		MemoryEnabled:     true,
		MemorySearched:    true,
		MemoryResultCount: 2,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if run.ID == "" || run.OrganizationID != "org_1" || run.Status != RunStatusRunning {
		t.Fatalf("unexpected created run: %+v", run)
	}
	if !run.MemoryEnabled || !run.MemorySearched || run.MemoryResultCount != 2 {
		t.Fatalf("memory evidence was not persisted on run: %+v", run)
	}

	completedAt := time.Now().UTC()
	updated, err := store.UpdateRun(ctx, "org_1", run.ID, UpdateRunRequest{
		Status:         stringPtr(RunStatusCompleted),
		IterationCount: intPtr(2),
		ToolCallCount:  intPtr(1),
		FinalMessageID: stringPtr("msg_final"),
		CompletedAt:    &completedAt,
	})
	if err != nil {
		t.Fatalf("UpdateRun returned error: %v", err)
	}
	if updated.Status != RunStatusCompleted || updated.FinalMessageID != "msg_final" {
		t.Fatalf("expected completed run with final message, got %+v", updated)
	}
	if updated.IterationCount != 2 || updated.ToolCallCount != 1 {
		t.Fatalf("expected iteration/tool counts to persist, got %+v", updated)
	}
	if updated.CompletedAt == nil {
		t.Fatalf("expected completed_at to be persisted: %+v", updated)
	}
}

func TestAgentToolRunStorePersistsToolLifecycle(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_tool_run_lifecycle",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime_1",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{"format": "rfc3339"},
		Status:         ToolRunStatusPendingApproval,
		ApprovalStatus: ApprovalStatusPending,
		AttemptCount:   0,
	})
	if err != nil {
		t.Fatalf("CreateToolRun returned error: %v", err)
	}
	if toolRun.ID == "" || toolRun.RunID != run.ID || toolRun.Status != ToolRunStatusPendingApproval {
		t.Fatalf("unexpected created tool run: %+v", toolRun)
	}
	if toolRun.Arguments["format"] != "rfc3339" {
		t.Fatalf("expected arguments to round trip, got %+v", toolRun.Arguments)
	}

	startedAt := time.Now().UTC()
	running, err := store.UpdateToolRun(ctx, "org_1", toolRun.ID, UpdateToolRunRequest{
		Status:         stringPtr(ToolRunStatusRunning),
		ApprovalStatus: stringPtr(ApprovalStatusNotRequired),
		AttemptCount:   intPtr(1),
		StartedAt:      &startedAt,
	})
	if err != nil {
		t.Fatalf("UpdateToolRun running returned error: %v", err)
	}
	if running.Status != ToolRunStatusRunning || running.AttemptCount != 1 || running.StartedAt == nil {
		t.Fatalf("expected running tool run with attempt evidence, got %+v", running)
	}

	completedAt := time.Now().UTC()
	completed, err := store.UpdateToolRun(ctx, "org_1", toolRun.ID, UpdateToolRunRequest{
		Status:        stringPtr(ToolRunStatusCompleted),
		ResultContent: stringPtr("2026-05-28T00:00:00Z"),
		CompletedAt:   &completedAt,
	})
	if err != nil {
		t.Fatalf("UpdateToolRun completed returned error: %v", err)
	}
	if completed.Status != ToolRunStatusCompleted || completed.ResultContent != "2026-05-28T00:00:00Z" {
		t.Fatalf("expected completed tool run result, got %+v", completed)
	}
	if completed.CompletedAt == nil {
		t.Fatalf("expected completed_at to be persisted: %+v", completed)
	}
}

func TestAgentRunStoreRejectsCrossTenantAccess(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_cross_tenant",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_cross_tenant",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         ToolRunStatusFailed,
		ApprovalStatus: ApprovalStatusNotRequired,
		AttemptCount:   1,
		Error:          "forced failure",
	})
	if err != nil {
		t.Fatalf("CreateToolRun returned error: %v", err)
	}

	if got, err := store.GetRun(ctx, "org_2", run.ID); err != nil || got != nil {
		t.Fatalf("cross-tenant GetRun got run=%+v err=%v, want nil nil", got, err)
	}
	if got, err := store.ListRuns(ctx, "org_2", "conv_1"); err != nil || len(got) != 0 {
		t.Fatalf("cross-tenant ListRuns got runs=%+v err=%v, want empty nil", got, err)
	}
	if _, err := store.UpdateRun(ctx, "org_2", run.ID, UpdateRunRequest{Status: stringPtr(RunStatusCompleted)}); err == nil {
		t.Fatal("cross-tenant UpdateRun should fail")
	}
	if got, err := store.GetToolRun(ctx, "org_2", toolRun.ID); err != nil || got != nil {
		t.Fatalf("cross-tenant GetToolRun got toolRun=%+v err=%v, want nil nil", got, err)
	}
	if got, err := store.ListToolRuns(ctx, "org_2", run.ID); err != nil || len(got) != 0 {
		t.Fatalf("cross-tenant ListToolRuns got toolRuns=%+v err=%v, want empty nil", got, err)
	}
	if _, err := store.UpdateToolRun(ctx, "org_2", toolRun.ID, UpdateToolRunRequest{Status: stringPtr(ToolRunStatusCompleted)}); err == nil {
		t.Fatal("cross-tenant UpdateToolRun should fail")
	}
}

func TestAgentRunStoreRejectsCrossTenantCreate(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_2",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_wrong_tenant",
		Status:         RunStatusRunning,
	})
	if err == nil || run != nil {
		t.Fatalf("CreateRun with mismatched tenant got run=%+v err=%v, want nil error", run, err)
	}

	validRun, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_valid_tenant",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun valid tenant returned error: %v", err)
	}
	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_2",
		RunID:          validRun.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_wrong_tenant",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         ToolRunStatusRunning,
		ApprovalStatus: ApprovalStatusNotRequired,
		AttemptCount:   1,
	})
	if err == nil || toolRun != nil {
		t.Fatalf("CreateToolRun with mismatched tenant got toolRun=%+v err=%v, want nil error", toolRun, err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func TestMarshalToolCallsRoundTrip(t *testing.T) {
	original := []ToolCall{
		{ID: "call_1", Name: "weather", Arguments: map[string]any{"city": "Beijing"}},
		{ID: "call_2", Name: "datetime", Arguments: map[string]any{}},
	}

	data := MarshalToolCalls(original)
	if len(data) == 0 {
		t.Fatal("MarshalToolCalls should return non-empty JSON for non-empty input")
	}

	result := UnmarshalToolCalls(data)
	if len(result) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Name != "weather" {
		t.Fatalf("unexpected first tool call: %+v", result[0])
	}
	if result[1].ID != "call_2" || result[1].Name != "datetime" {
		t.Fatalf("unexpected second tool call: %+v", result[1])
	}
}

func TestMarshalToolCallsEmpty(t *testing.T) {
	if data := MarshalToolCalls(nil); data != nil {
		t.Fatal("MarshalToolCalls should return nil for nil input")
	}
	if data := MarshalToolCalls([]ToolCall{}); data != nil {
		t.Fatal("MarshalToolCalls should return nil for empty slice")
	}
}

func TestUnmarshalToolCallsEmpty(t *testing.T) {
	if result := UnmarshalToolCalls(nil); result != nil {
		t.Fatal("UnmarshalToolCalls should return nil for nil data")
	}
	if result := UnmarshalToolCalls([]byte{}); result != nil {
		t.Fatal("UnmarshalToolCalls should return nil for empty data")
	}
}

func TestHasEnabledTools(t *testing.T) {
	if hasEnabledTools(nil) {
		t.Fatal("nil agent should not have enabled tools")
	}

	agent := &Agent{Tools: []Tool{}}
	if hasEnabledTools(agent) {
		t.Fatal("agent with no tools should not have enabled tools")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "datetime", Type: "builtin", Enabled: false},
	}}
	if hasEnabledTools(agent) {
		t.Fatal("agent with only disabled tools should not be detected")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "disabled", Type: "builtin", Enabled: false},
		{Name: "enabled", Type: "builtin", Enabled: true},
	}}
	if !hasEnabledTools(agent) {
		t.Fatal("agent with at least one enabled tool should be detected")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "mcp_tool", Type: "mcp", Enabled: true},
	}}
	if !hasEnabledTools(agent) {
		t.Fatal("agent with enabled MCP tool should be detected")
	}
}

func TestStreamContentSplitsWords(t *testing.T) {
	var chunks []string
	err := streamContent("hello world", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 'hello world', got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "hello " || chunks[1] != "world" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestStreamContentSingleWord(t *testing.T) {
	var chunks []string
	err := streamContent("ok", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for 'ok', got %d", len(chunks))
	}
	if chunks[0] != "ok" {
		t.Fatalf("unexpected chunk: %q", chunks[0])
	}
}

func TestStreamContentEmpty(t *testing.T) {
	err := streamContent("", func(chunk string) error {
		t.Fatal("should not call onChunk for empty content")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolCallsToChatToolCalls(t *testing.T) {
	input := []ToolCall{
		{ID: "call_1", Name: "weather.lookup", Arguments: map[string]any{"city": "Paris"}},
		{ID: "call_2", Name: "datetime", Arguments: map[string]any{}},
	}

	result := toolCallsToChatToolCalls(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 chat tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Type != "function" {
		t.Fatalf("unexpected first chat tool call: %+v", result[0])
	}
	if result[0].Function.Name != "weather.lookup" {
		t.Fatalf("expected weather.lookup, got %q", result[0].Function.Name)
	}
	if result[0].Function.Arguments == "" {
		t.Fatal("arguments should be serialized JSON string")
	}
}

func TestToolCallsToChatToolCallsEmpty(t *testing.T) {
	if result := toolCallsToChatToolCalls(nil); result != nil {
		t.Fatal("should return nil for nil input")
	}
	if result := toolCallsToChatToolCalls([]ToolCall{}); result != nil {
		t.Fatal("should return nil for empty input")
	}
}

func TestChatToolCallsToAgent(t *testing.T) {
	input := []chat.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: chat.ToolFunction{
				Name:      "weather.lookup",
				Arguments: `{"city":"Paris"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: chat.ToolFunction{
				Name:      "datetime",
				Arguments: "",
			},
		},
	}

	result := chatToolCallsToAgent(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 agent tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Name != "weather.lookup" {
		t.Fatalf("unexpected first agent tool call: %+v", result[0])
	}
	city, ok := result[0].Arguments["city"].(string)
	if !ok || city != "Paris" {
		t.Fatalf("expected city=Paris in arguments, got %+v", result[0].Arguments)
	}
	if result[1].Arguments == nil {
		t.Fatal("empty arguments should result in empty map, not nil")
	}
}

func TestChatToolCallsToAgentEmpty(t *testing.T) {
	if result := chatToolCallsToAgent(nil); result != nil {
		t.Fatal("should return nil for nil input")
	}
	if result := chatToolCallsToAgent([]chat.ToolCall{}); result != nil {
		t.Fatal("should return nil for empty input")
	}
}

func TestParseToolCallsFromResponse(t *testing.T) {
	// Simulates a raw LLM response map containing tool_calls.
	response := map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id":   "call_abc",
				"type": "function",
				"function": map[string]any{
					"name":      "datetime",
					"arguments": `{}`,
				},
			},
			map[string]any{
				"id":   "call_def",
				"type": "function",
				"function": map[string]any{
					"name":      "web_search",
					"arguments": `{"query":"golang news"}`,
				},
			},
		},
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_abc" || toolCalls[0].Name != "datetime" {
		t.Fatalf("unexpected first tool call: %+v", toolCalls[0])
	}
	if toolCalls[1].ID != "call_def" || toolCalls[1].Name != "web_search" {
		t.Fatalf("unexpected second tool call: %+v", toolCalls[1])
	}
	if toolCalls[1].Arguments["query"] != "golang news" {
		t.Fatalf("unexpected arguments: %+v", toolCalls[1].Arguments)
	}
}

func TestParseToolCallsFromResponseNoToolCalls(t *testing.T) {
	response := map[string]any{
		"content": "simple text response",
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolCalls != nil {
		t.Fatalf("expected nil tool calls, got %+v", toolCalls)
	}
}

func TestParseToolCallsFromResponseNilResponse(t *testing.T) {
	toolCalls, err := ParseToolCallsFromResponse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolCalls != nil {
		t.Fatal("expected nil tool calls for nil response")
	}
}

func TestParseToolCallsFromResponseMalformedEntries(t *testing.T) {
	// Missing function field should be skipped gracefully.
	response := map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id": "call_skip",
			},
			map[string]any{
				"id":   "call_good",
				"type": "function",
				"function": map[string]any{
					"name":      "datetime",
					"arguments": `{}`,
				},
			},
		},
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call after skipping malformed, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_good" {
		t.Fatalf("expected call_good, got %q", toolCalls[0].ID)
	}
}
