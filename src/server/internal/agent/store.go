package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Agent 表示一个 Agent 实例
type Agent struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	UserID         string    `json:"userId"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Model          string    `json:"model"`
	SystemPrompt   string    `json:"systemPrompt,omitempty"`
	Tools          []Tool    `json:"tools,omitempty"`
	Config         Config    `json:"config,omitempty"`
	IsPublic       bool      `json:"isPublic"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Tool 表示 Agent 可用的工具
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // "builtin" | "mcp"
	ServerID    string `json:"serverId,omitempty"` // MCP server ID
	Enabled     bool   `json:"enabled"`
}

// Config 表示 Agent 配置
type Config struct {
	EnableMemory     bool    `json:"enableMemory,omitempty"`
	MaxTokens        int     `json:"maxTokens,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	TopP             float64 `json:"topP,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledgeBaseIds,omitempty"`
}

// Conversation 表示 Agent 对话
type Conversation struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agentId"`
	OrganizationID string    `json:"organizationId"`
	UserID         string    `json:"userId"`
	Title          string    `json:"title,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Message 表示 Agent 消息
type Message struct {
	ID             string      `json:"id"`
	ConversationID string      `json:"conversationId"`
	OrganizationID string      `json:"organizationId"`
	Role           string      `json:"role"` // "user" | "assistant" | "tool"
	Content        string      `json:"content"`
	ToolCalls      []ToolCall  `json:"toolCalls,omitempty"`
	ToolCallID     string      `json:"toolCallId,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
}

// ToolCall 表示工具调用
type ToolCall struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// CreateAgentRequest 创建 Agent 请求
type CreateAgentRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description,omitempty"`
	Model        string  `json:"model,omitempty"`
	SystemPrompt string  `json:"systemPrompt,omitempty"`
	Tools        []Tool  `json:"tools,omitempty"`
	Config       Config  `json:"config,omitempty"`
	IsPublic     bool    `json:"isPublic,omitempty"`
}

// UpdateAgentRequest 更新 Agent 请求
type UpdateAgentRequest struct {
	Name         *string  `json:"name,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Model        *string  `json:"model,omitempty"`
	SystemPrompt *string  `json:"systemPrompt,omitempty"`
	Tools        []Tool   `json:"tools,omitempty"`
	Config       *Config  `json:"config,omitempty"`
	IsPublic     *bool    `json:"isPublic,omitempty"`
}

// Store 接口
type Store interface {
	// Agent CRUD
	CreateAgent(ctx context.Context, userID, organizationID string, req *CreateAgentRequest) (*Agent, error)
	GetAgent(ctx context.Context, id, organizationID string) (*Agent, error)
	ListAgents(ctx context.Context, userID, organizationID string) ([]*Agent, error)
	UpdateAgent(ctx context.Context, id, organizationID string, req *UpdateAgentRequest) (*Agent, error)
	DeleteAgent(ctx context.Context, id, organizationID string) error

	// Conversation
	CreateConversation(ctx context.Context, agentID, userID, organizationID string, title string) (*Conversation, error)
	GetConversation(ctx context.Context, id, organizationID string) (*Conversation, error)
	ListConversations(ctx context.Context, agentID, userID, organizationID string) ([]*Conversation, error)
	DeleteConversation(ctx context.Context, id, organizationID string) error

	// Messages
	CreateMessage(ctx context.Context, conversationID, organizationID, role, content string, toolCalls []ToolCall, toolCallID string) (*Message, error)
	ListMessages(ctx context.Context, conversationID, organizationID string) ([]*Message, error)
}

// SQLStore SQL 实现
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore 创建 SQLStore
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

// CreateAgent 创建 Agent
func (s *SQLStore) CreateAgent(ctx context.Context, userID, organizationID string, req *CreateAgentRequest) (*Agent, error) {
	id := generateID()
	now := time.Now()

	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	toolsJSON, _ := json.Marshal(req.Tools)
	configJSON, _ := json.Marshal(req.Config)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agents (id, user_id, organization_id, name, description, model, system_prompt, tools, config, is_public, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, id, userID, organizationID, req.Name, req.Description, model, req.SystemPrompt, toolsJSON, configJSON, req.IsPublic, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	return &Agent{
		ID:             id,
		OrganizationID: organizationID,
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		Model:          model,
		SystemPrompt:   req.SystemPrompt,
		Tools:          req.Tools,
		Config:         req.Config,
		IsPublic:       req.IsPublic,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// GetAgent 获取 Agent
func (s *SQLStore) GetAgent(ctx context.Context, id, organizationID string) (*Agent, error) {
	var agent Agent
	var toolsJSON, configJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, name, description, model, system_prompt, tools, config, is_public, created_at, updated_at
		FROM agents WHERE id = $1 AND organization_id = $2
	`, id, organizationID).Scan(&agent.ID, &agent.OrganizationID, &agent.UserID, &agent.Name, &agent.Description, &agent.Model,
		&agent.SystemPrompt, &toolsJSON, &configJSON, &agent.IsPublic, &agent.CreatedAt, &agent.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent: %w", err)
	}

	json.Unmarshal(toolsJSON, &agent.Tools)
	json.Unmarshal(configJSON, &agent.Config)

	return &agent, nil
}

// ListAgents 列出用户的 Agent
func (s *SQLStore) ListAgents(ctx context.Context, userID, organizationID string) ([]*Agent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, name, description, model, system_prompt, tools, config, is_public, created_at, updated_at
		FROM agents WHERE user_id = $1 AND organization_id = $2 ORDER BY created_at DESC
	`, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var agent Agent
		var toolsJSON, configJSON []byte

		err := rows.Scan(&agent.ID, &agent.OrganizationID, &agent.UserID, &agent.Name, &agent.Description, &agent.Model,
			&agent.SystemPrompt, &toolsJSON, &configJSON, &agent.IsPublic, &agent.CreatedAt, &agent.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}

		json.Unmarshal(toolsJSON, &agent.Tools)
		json.Unmarshal(configJSON, &agent.Config)
		agents = append(agents, &agent)
	}

	return agents, rows.Err()
}

// UpdateAgent 更新 Agent
func (s *SQLStore) UpdateAgent(ctx context.Context, id, organizationID string, req *UpdateAgentRequest) (*Agent, error) {
	agent, err := s.GetAgent(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Apply updates
	if req.Name != nil {
		agent.Name = *req.Name
	}
	if req.Description != nil {
		agent.Description = *req.Description
	}
	if req.Model != nil {
		agent.Model = *req.Model
	}
	if req.SystemPrompt != nil {
		agent.SystemPrompt = *req.SystemPrompt
	}
	if req.Tools != nil {
		agent.Tools = req.Tools
	}
	if req.Config != nil {
		agent.Config = *req.Config
	}
	if req.IsPublic != nil {
		agent.IsPublic = *req.IsPublic
	}

	now := time.Now()
	toolsJSON, _ := json.Marshal(agent.Tools)
	configJSON, _ := json.Marshal(agent.Config)

	_, err = s.db.ExecContext(ctx, `
		UPDATE agents SET name = $2, description = $3, model = $4, system_prompt = $5, tools = $6, config = $7, is_public = $8, updated_at = $9
		WHERE id = $1 AND organization_id = $10
	`, id, agent.Name, agent.Description, agent.Model, agent.SystemPrompt, toolsJSON, configJSON, agent.IsPublic, now, organizationID)
	if err != nil {
		return nil, fmt.Errorf("update agent: %w", err)
	}

	agent.UpdatedAt = now
	return agent, nil
}

// DeleteAgent 删除 Agent
func (s *SQLStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1 AND organization_id = $2`, id, organizationID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	return nil
}

// CreateConversation 创建对话
func (s *SQLStore) CreateConversation(ctx context.Context, agentID, userID, organizationID string, title string) (*Conversation, error) {
	id := generateID()
	now := time.Now()

	if title == "" {
		title = "New conversation"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_conversations (id, agent_id, user_id, organization_id, title, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, id, agentID, userID, organizationID, title, now)
	if err != nil {
		return nil, fmt.Errorf("insert conversation: %w", err)
	}

	return &Conversation{
		ID:             id,
		AgentID:        agentID,
		OrganizationID: organizationID,
		UserID:         userID,
		Title:          title,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// GetConversation 获取对话
func (s *SQLStore) GetConversation(ctx context.Context, id, organizationID string) (*Conversation, error) {
	var conv Conversation
	err := s.db.QueryRowContext(ctx, `
		SELECT id, agent_id, organization_id, user_id, title, created_at, updated_at
		FROM agent_conversations WHERE id = $1 AND organization_id = $2
	`, id, organizationID).Scan(&conv.ID, &conv.AgentID, &conv.OrganizationID, &conv.UserID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}
	return &conv, nil
}

// ListConversations 列出对话
func (s *SQLStore) ListConversations(ctx context.Context, agentID, userID, organizationID string) ([]*Conversation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, organization_id, user_id, title, created_at, updated_at
		FROM agent_conversations WHERE agent_id = $1 AND user_id = $2 AND organization_id = $3 ORDER BY created_at DESC
	`, agentID, userID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var convs []*Conversation
	for rows.Next() {
		var conv Conversation
		err := rows.Scan(&conv.ID, &conv.AgentID, &conv.OrganizationID, &conv.UserID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		convs = append(convs, &conv)
	}

	return convs, rows.Err()
}

// DeleteConversation 删除对话
func (s *SQLStore) DeleteConversation(ctx context.Context, id, organizationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agent_conversations WHERE id = $1 AND organization_id = $2`, id, organizationID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

// CreateMessage 创建消息
func (s *SQLStore) CreateMessage(ctx context.Context, conversationID, organizationID, role, content string, toolCalls []ToolCall, toolCallID string) (*Message, error) {
	id := generateID()
	now := time.Now()

	toolCallsJSON, _ := json.Marshal(toolCalls)

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_messages (id, conversation_id, organization_id, role, content, tool_calls, tool_call_id, created_at)
		SELECT $1, c.id, c.organization_id, $3, $4, $5, $6, $7
		FROM agent_conversations c
		WHERE c.id = $2 AND c.organization_id = $8
	`, id, conversationID, role, content, toolCallsJSON, toolCallID, now, organizationID)
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert message rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("conversation not found")
	}

	return &Message{
		ID:             id,
		ConversationID: conversationID,
		OrganizationID: organizationID,
		Role:           role,
		Content:        content,
		ToolCalls:      toolCalls,
		ToolCallID:     toolCallID,
		CreatedAt:      now,
	}, nil
}

// ListMessages 列出消息
func (s *SQLStore) ListMessages(ctx context.Context, conversationID, organizationID string) ([]*Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, conversation_id, organization_id, role, content, tool_calls, tool_call_id, created_at
		FROM agent_messages WHERE conversation_id = $1 AND organization_id = $2 ORDER BY created_at ASC
	`, conversationID, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		var msg Message
		var toolCallsJSON []byte
		var toolCallID sql.NullString

		err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.OrganizationID, &msg.Role, &msg.Content, &toolCallsJSON, &toolCallID, &msg.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		json.Unmarshal(toolCallsJSON, &msg.ToolCalls)
		msg.ToolCallID = toolCallID.String
		messages = append(messages, &msg)
	}

	return messages, rows.Err()
}

func generateID() string {
	return fmt.Sprintf("agent_%d", time.Now().UnixNano())
}
