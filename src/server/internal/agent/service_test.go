package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

type fakeGateway struct {
	plainReply      string
	structured      []*chat.CompletionResponse
	plainCalls      int
	streamCalls     int
	structuredCalls int
	structIndex     int
}

func (g *fakeGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	g.plainCalls++
	return g.plainReply, nil
}

func (g *fakeGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	g.streamCalls++
	return onChunk(g.plainReply)
}

func (g *fakeGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	g.structuredCalls++
	if g.structIndex >= len(g.structured) {
		return &chat.CompletionResponse{Content: g.plainReply}, nil
	}
	resp := g.structured[g.structIndex]
	g.structIndex++
	return resp, nil
}

type fakeStore struct {
	agent        *Agent
	conversation *Conversation
	messages     []*Message
}

func (s *fakeStore) CreateAgent(ctx context.Context, userID string, req *CreateAgentRequest) (*Agent, error) {
	panic("not used")
}

func (s *fakeStore) GetAgent(ctx context.Context, id string) (*Agent, error) {
	if s.agent != nil && s.agent.ID == id {
		return s.agent, nil
	}
	return nil, nil
}

func (s *fakeStore) ListAgents(ctx context.Context, userID string) ([]*Agent, error) {
	panic("not used")
}

func (s *fakeStore) UpdateAgent(ctx context.Context, id string, req *UpdateAgentRequest) (*Agent, error) {
	panic("not used")
}

func (s *fakeStore) DeleteAgent(ctx context.Context, id string) error {
	panic("not used")
}

func (s *fakeStore) CreateConversation(ctx context.Context, agentID, userID string, title string) (*Conversation, error) {
	panic("not used")
}

func (s *fakeStore) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	if s.conversation != nil && s.conversation.ID == id {
		return s.conversation, nil
	}
	return nil, nil
}

func (s *fakeStore) ListConversations(ctx context.Context, agentID, userID string) ([]*Conversation, error) {
	panic("not used")
}

func (s *fakeStore) DeleteConversation(ctx context.Context, id string) error {
	panic("not used")
}

func (s *fakeStore) CreateMessage(ctx context.Context, conversationID, role, content string, toolCalls []ToolCall, toolCallID string) (*Message, error) {
	msg := &Message{
		ID:             role + "-" + time.Now().UTC().Format("150405.000000"),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		ToolCalls:      append([]ToolCall(nil), toolCalls...),
		ToolCallID:     toolCallID,
		CreatedAt:      time.Now().UTC(),
	}
	s.messages = append(s.messages, msg)
	return msg, nil
}

func (s *fakeStore) ListMessages(ctx context.Context, conversationID string) ([]*Message, error) {
	result := make([]*Message, len(s.messages))
	copy(result, s.messages)
	return result, nil
}

func TestServiceSendMessageUsesRunnerForToolEnabledAgents(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "plain path used",
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{
						ID:   "call_datetime",
						Type: "function",
						Function: chat.ToolFunction{
							Name:      "datetime",
							Arguments: "{}",
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "final answer after tool",
				FinishReason: "stop",
			},
		},
	}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	msg, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "What time is it?")
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	if msg.Content != "final answer after tool" {
		t.Fatalf("expected final tool-assisted answer, got %q", msg.Content)
	}
	if gateway.structuredCalls != 2 {
		t.Fatalf("expected tool path to use configured gateway twice, got %d calls", gateway.structuredCalls)
	}
	if gateway.plainCalls != 0 {
		t.Fatalf("expected structured tool path not to bypass into plain gateway calls, got %d", gateway.plainCalls)
	}

	var sawToolMessage bool
	for _, message := range store.messages {
		if message.Role == "tool" && message.ToolCallID == "call_datetime" {
			sawToolMessage = true
			break
		}
	}
	if !sawToolMessage {
		t.Fatalf("expected tool result message to be persisted, got %+v", store.messages)
	}
}

// TestRunnerExhaustsIterationCap verifies that the Runner returns
// ErrMaxIterationsExceeded when the model never produces a final
// non-tool response within the iteration budget.
func TestRunnerExhaustsIterationCap(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	// Gateway that always returns tool calls (never a final answer).
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{ToolCalls: []chat.ToolCall{{ID: "c1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}}}, FinishReason: "tool_calls"},
			{ToolCalls: []chat.ToolCall{{ID: "c2", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}}}, FinishReason: "tool_calls"},
			{ToolCalls: []chat.ToolCall{{ID: "c3", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}}}, FinishReason: "tool_calls"},
		},
	}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 2

	_, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "loop forever")
	if err == nil {
		t.Fatal("expected iteration cap error, got nil")
	}
	if !errors.Is(err, ErrMaxIterationsExceeded) {
		t.Fatalf("expected ErrMaxIterationsExceeded, got %v", err)
	}
}

// TestRunWithToolsStreaming verifies that the final assistant answer is
// streamed via the onChunk callback in word-level chunks.
func TestRunWithToolsStreaming(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "c1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "hello world",
				FinishReason: "stop",
			},
		},
	}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "hi", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected streamed chunks, got none")
	}
	// Word-level chunking should produce at least 2 chunks for "hello world".
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 streamed chunks for 'hello world', got %d: %v", len(chunks), chunks)
	}
	combined := ""
	for _, c := range chunks {
		combined += c
	}
	if combined != "hello world" {
		t.Fatalf("expected streamed content 'hello world', got %q", combined)
	}

	var sawToolMessage bool
	for _, msg := range store.messages {
		if msg.Role == "tool" {
			sawToolMessage = true
			break
		}
	}
	if !sawToolMessage {
		t.Fatal("expected tool result message in streaming path")
	}
}

// TestRunWithToolsFallbackStreaming verifies that the non-structured
// gateway fallback path still calls onChunk when provided.
func TestRunWithToolsFallbackStreaming(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	// Gateway that does NOT implement StructuredReplyGenerator
	gateway := &plainOnlyGateway{reply: "fallback reply"}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "hi", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk in fallback path, got none")
	}
	if chunks[0] != "fallback reply" {
		t.Fatalf("expected 'fallback reply', got %q", chunks[0])
	}
}

// TestServiceSendMessagePlainPath verifies the non-tool plain chat path.
func TestServiceSendMessagePlainPath(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_plain",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools:  nil, // No tools.
		},
		conversation: &Conversation{
			ID:      "conv_plain",
			AgentID: "agent_plain",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "plain response with no tools",
	}

	service := NewService(store, gateway)
	msg, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_plain", "hello")
	if err != nil {
		t.Fatalf("SendMessage plain path returned error: %v", err)
	}
	if msg.Content != "plain response with no tools" {
		t.Fatalf("expected plain response, got %q", msg.Content)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected plain path to use configured Relay-compatible gateway once, got %d calls", gateway.plainCalls)
	}
}

// TestServiceSendMessageAllDisabledTools verifies that agents with all tools
// disabled take the plain path.
func TestServiceSendMessageAllDisabledTools(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_disabled",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: false},
				{Name: "web_search", Type: "builtin", Enabled: false},
			},
		},
		conversation: &Conversation{
			ID:      "conv_disabled",
			AgentID: "agent_disabled",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "plain path for disabled tools",
	}

	service := NewService(store, gateway)
	msg, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_disabled", "test")
	if err != nil {
		t.Fatalf("SendMessage with disabled tools returned error: %v", err)
	}
	if msg.Content != "plain path for disabled tools" {
		t.Fatalf("expected plain path reply, got %q", msg.Content)
	}
}

// TestSendMessageStreamPlainPath verifies streaming for non-tool agents.
func TestSendMessageStreamPlainPath(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_stream",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:      "conv_stream",
			AgentID: "agent_stream",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "streaming plain reply",
	}

	service := NewService(store, gateway)
	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_stream", "hello", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream plain path returned error: %v", err)
	}
	if len(chunks) == 0 || chunks[0] != "streaming plain reply" {
		t.Fatalf("expected 'streaming plain reply' chunk, got %v", chunks)
	}
}

// plainOnlyGateway implements ChatGateway but NOT StructuredReplyGenerator.
type plainOnlyGateway struct {
	reply string
}

func (g *plainOnlyGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return g.reply, nil
}

func (g *plainOnlyGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	return onChunk(g.reply)
}
