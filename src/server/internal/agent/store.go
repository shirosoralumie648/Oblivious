package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
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
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Type             string `json:"type"`               // "builtin" | "mcp"
	ServerID         string `json:"serverId,omitempty"` // MCP server ID
	Enabled          bool   `json:"enabled"`
	RequiresApproval bool   `json:"requiresApproval,omitempty"`
	RiskLevel        string `json:"riskLevel,omitempty"`
	InputSchema      any    `json:"inputSchema,omitempty"`
	Runtime          string `json:"runtime,omitempty"`
	SourceCode       string `json:"sourceCode,omitempty"`
	TimeoutSeconds   int    `json:"timeoutSeconds,omitempty"`
}

// Config 表示 Agent 配置
type Config struct {
	EnableMemory                   bool                            `json:"enableMemory,omitempty"`
	MaxTokens                      int                             `json:"maxTokens,omitempty"`
	MaxIterations                  int                             `json:"maxIterations,omitempty"`
	TokenBudget                    int                             `json:"tokenBudget,omitempty"`
	DefaultExecutionMode           string                          `json:"defaultExecutionMode,omitempty"`
	LongTermMemoryExtractionPolicy string                          `json:"longTermMemoryExtractionPolicy,omitempty"`
	LongTermMemoryUpdatePolicy     string                          `json:"longTermMemoryUpdatePolicy,omitempty"`
	LongTermMemoryWritePolicy      string                          `json:"longTermMemoryWritePolicy,omitempty"`
	Temperature                    float64                         `json:"temperature,omitempty"`
	TopP                           float64                         `json:"topP,omitempty"`
	KnowledgeBaseIDs               []string                        `json:"knowledgeBaseIds,omitempty"`
	ApprovalMode                   string                          `json:"approvalMode,omitempty"`
	ToolApprovalOverrides          map[string]ToolApprovalOverride `json:"toolApprovalOverrides,omitempty"`
	// ModelRoutingRules selects the model per iteration based on task
	// signals. When empty the agent's static Model is always used.
	ModelRoutingRules []ModelRoutingRule `json:"modelRoutingRules,omitempty"`
	// Skills are named instruction+tool bundles. When set, the skill
	// selector scores them against the user input and injects the
	// top-scoring skills' instructions and tools into the run.
	Skills []Skill `json:"skills,omitempty"`
	// MaxSkills caps how many skills the selector activates per run
	// (default 3 when Skills is non-empty).
	MaxSkills int `json:"maxSkills,omitempty"`
	// SubAgentMaxDepth bounds nested call_agent invocations (default 3).
	SubAgentMaxDepth int `json:"subAgentMaxDepth,omitempty"`
}

// ModelRoutingRule maps task signals to a target model. The first rule
// whose conditions all match wins; unmatched runs fall back to the static
// agent model.
type ModelRoutingRule struct {
	// TargetModel is the model id selected when this rule matches.
	TargetModel string `json:"targetModel"`
	// MinInputChars matches when the iteration input length is >= this.
	MinInputChars int `json:"minInputChars,omitempty"`
	// MaxInputChars matches when the iteration input length is <= this
	// (0 means no upper bound).
	MaxInputChars int `json:"maxInputChars,omitempty"`
	// MinIteration matches when the 1-based iteration index is >= this.
	MinIteration int `json:"minIteration,omitempty"`
	// RequiresToolResult matches only when a prior tool result exists in
	// the iteration context.
	RequiresToolResult bool `json:"requiresToolResult,omitempty"`
	// Keywords matches when any keyword (case-insensitive) is present in
	// the input.
	Keywords []string `json:"keywords,omitempty"`
}

// Skill is a named instruction+tool bundle the skill selector can activate.
type Skill struct {
	Name string `json:"name"`
	// Instructions are appended to the system prompt when the skill is
	// selected.
	Instructions string `json:"instructions,omitempty"`
	// Triggers are keywords/phrases used for lexical scoring against the
	// user input.
	Triggers []string `json:"triggers,omitempty"`
	// ToolNames lists tools this skill enables when active.
	ToolNames []string `json:"toolNames,omitempty"`
}

type ToolApprovalOverride struct {
	RiskLevel        string `json:"riskLevel,omitempty"`
	RequiresApproval *bool  `json:"requiresApproval,omitempty"`
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
	ID             string     `json:"id"`
	ConversationID string     `json:"conversationId"`
	OrganizationID string     `json:"organizationId"`
	Role           string     `json:"role"` // "user" | "assistant" | "tool"
	Content        string     `json:"content"`
	ToolCalls      []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID     string     `json:"toolCallId,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// ToolCall 表示工具调用
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	RunID     string         `json:"runId,omitempty"`
	ToolRunID string         `json:"toolRunId,omitempty"`
	RequestID string         `json:"requestId,omitempty"`
}

const (
	RunStatusRunning              = "running"
	RunStatusPendingApproval      = "pending_approval"
	RunStatusCompleted            = "completed"
	RunStatusFailed               = "failed"
	RunStatusMaxIterationsReached = "max_iterations_reached"
	RunStatusTokenBudgetExceeded  = "token_budget_exceeded"

	ToolRunStatusPendingApproval = "pending_approval"
	ToolRunStatusRunning         = "running"
	ToolRunStatusCompleted       = "completed"
	ToolRunStatusFailed          = "failed"
	ToolRunStatusRejected        = "rejected"

	PlanStepStatusPending   = "pending"
	PlanStepStatusApproved  = "approved"
	PlanStepStatusRunning   = "running"
	PlanStepStatusCompleted = "completed"
	PlanStepStatusFailed    = "failed"
	PlanStepStatusSkipped   = "skipped"

	ApprovalStatusNotRequired = "not_required"
	ApprovalStatusPending     = "pending"
	ApprovalStatusApproved    = "approved"
	ApprovalStatusRejected    = "rejected"
)

// Run is durable Agent workflow state for one user request.
type Run struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	ConversationID    string     `json:"conversationId"`
	AgentID           string     `json:"agentId"`
	UserID            string     `json:"userId"`
	RequestID         string     `json:"requestId,omitempty"`
	Mode              string     `json:"mode"`
	Status            string     `json:"status"`
	MemoryEnabled     bool       `json:"memoryEnabled"`
	MemorySearched    bool       `json:"memorySearched"`
	MemoryResultCount int        `json:"memoryResultCount"`
	IterationCount    int        `json:"iterationCount"`
	ToolCallCount     int        `json:"toolCallCount"`
	FinalMessageID    string     `json:"finalMessageId,omitempty"`
	Error             string     `json:"error,omitempty"`
	StartedAt         time.Time  `json:"startedAt"`
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// ToolRun is durable state for one model-requested tool call.
type ToolRun struct {
	ID                     string         `json:"id"`
	OrganizationID         string         `json:"organizationId"`
	RunID                  string         `json:"runId"`
	ConversationID         string         `json:"conversationId"`
	AgentID                string         `json:"agentId"`
	ToolCallID             string         `json:"toolCallId"`
	ToolName               string         `json:"toolName"`
	ToolType               string         `json:"toolType"`
	ServerID               string         `json:"serverId,omitempty"`
	RiskLevel              string         `json:"riskLevel,omitempty"`
	Arguments              map[string]any `json:"arguments"`
	Status                 string         `json:"status"`
	ApprovalStatus         string         `json:"approvalStatus"`
	ApprovedByUserID       string         `json:"approvedByUserId,omitempty"`
	ApprovalDecisionReason string         `json:"approvalDecisionReason,omitempty"`
	AttemptCount           int            `json:"attemptCount"`
	ResultContent          string         `json:"resultContent,omitempty"`
	Error                  string         `json:"error,omitempty"`
	StartedAt              *time.Time     `json:"startedAt,omitempty"`
	CompletedAt            *time.Time     `json:"completedAt,omitempty"`
	CreatedAt              time.Time      `json:"createdAt"`
	UpdatedAt              time.Time      `json:"updatedAt"`
}

type PlanStep struct {
	ID             string         `json:"id"`
	RunID          string         `json:"runId"`
	OrganizationID string         `json:"organizationId"`
	Index          int            `json:"index"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	Status         string         `json:"status"`
	ApprovalStatus string         `json:"approvalStatus"`
	ToolName       string         `json:"toolName,omitempty"`
	Input          map[string]any `json:"input,omitempty"`
	DependsOn      []int          `json:"dependsOn,omitempty"`
	ResultContent  string         `json:"resultContent,omitempty"`
	Error          string         `json:"error,omitempty"`
	StartedAt      *time.Time     `json:"startedAt,omitempty"`
	CompletedAt    *time.Time     `json:"completedAt,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

const (
	MemoryTypeShortTerm   = "short_term"
	MemoryTypeLongTerm    = "long_term"
	MemoryTypeUserManaged = "user_managed"

	memoryMetadataImportanceKey = "_importance"
)

type Memory struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	UserID         string         `json:"userId"`
	AgentID        string         `json:"agentId,omitempty"`
	Type           string         `json:"type"`
	Content        string         `json:"content"`
	Importance     int            `json:"importance"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	ExpiresAt      *time.Time     `json:"expiresAt,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type RunDetail struct {
	Run       *Run        `json:"run"`
	ToolRuns  []*ToolRun  `json:"toolRuns"`
	PlanSteps []*PlanStep `json:"planSteps,omitempty"`
}

type RunWithMessages struct {
	Run       *Run        `json:"run"`
	ToolRuns  []*ToolRun  `json:"toolRuns"`
	PlanSteps []*PlanStep `json:"planSteps,omitempty"`
	Messages  []*Message  `json:"messages"`
}

type StartRunRequest struct {
	AgentID        string
	ConversationID string
	Input          string
	MaxIterations  *int
	TokenBudget    *int
}

type CreateRunRequest struct {
	OrganizationID    string
	ConversationID    string
	AgentID           string
	UserID            string
	RequestID         string
	Mode              string
	Status            string
	MemoryEnabled     bool
	MemorySearched    bool
	MemoryResultCount int
	RecursionDepth    int
	MaxDepth          int
	StartedAt         time.Time
}

type UpdateRunRequest struct {
	Status            *string
	MemoryEnabled     *bool
	MemorySearched    *bool
	MemoryResultCount *int
	IterationCount    *int
	ToolCallCount     *int
	FinalMessageID    *string
	Error             *string
	CompletedAt       *time.Time
	ClearCompletedAt  bool
}

type CreateToolRunRequest struct {
	OrganizationID         string
	RunID                  string
	ConversationID         string
	AgentID                string
	ToolCallID             string
	ToolName               string
	ToolType               string
	ServerID               string
	RiskLevel              string
	Arguments              map[string]any
	Status                 string
	ApprovalStatus         string
	ApprovedByUserID       string
	ApprovalDecisionReason string
	AttemptCount           int
	ResultContent          string
	Error                  string
	StartedAt              *time.Time
	CompletedAt            *time.Time
}

type CreatePlanStepRequest struct {
	OrganizationID string
	RunID          string
	Index          int
	Title          string
	Description    string
	Status         string
	ApprovalStatus string
	ToolName       string
	Input          map[string]any
	DependsOn      []int
}

type UpdateToolRunRequest struct {
	Status                 *string
	ApprovalStatus         *string
	ApprovedByUserID       *string
	ApprovalDecisionReason *string
	AttemptCount           *int
	ResultContent          *string
	Error                  *string
	StartedAt              *time.Time
	CompletedAt            *time.Time
	ClearCompletedAt       bool
}

type UpdatePlanStepRequest struct {
	Index            *int
	Title            *string
	Description      *string
	Status           *string
	ApprovalStatus   *string
	ToolName         *string
	Input            map[string]any
	ReplaceInput     bool
	DependsOn        []int
	ReplaceDependsOn bool
	ResultContent    *string
	Error            *string
	StartedAt        *time.Time
	CompletedAt      *time.Time
	ClearCompletedAt bool
}

func normalizePlanStepDependsOn(dependsOn []int) []int {
	if len(dependsOn) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(dependsOn))
	normalized := make([]int, 0, len(dependsOn))
	for _, index := range dependsOn {
		if index <= 0 {
			continue
		}
		if _, ok := seen[index]; ok {
			continue
		}
		seen[index] = struct{}{}
		normalized = append(normalized, index)
	}
	sort.Ints(normalized)
	return normalized
}

func marshalPlanStepDependsOn(dependsOn []int) ([]byte, error) {
	normalized := normalizePlanStepDependsOn(dependsOn)
	if normalized == nil {
		normalized = []int{}
	}
	return json.Marshal(normalized)
}

type CreateMemoryRequest struct {
	AgentID    string
	Type       string
	Content    string
	Importance int
	Metadata   map[string]any
	ExpiresAt  *time.Time
}

type ListMemoriesRequest struct {
	AgentID string
	Type    string
	Query   string
	Limit   int
	Offset  int
}

type CreateMemoryStoreRequest struct {
	OrganizationID string
	UserID         string
	AgentID        string
	Type           string
	Content        string
	Embedding      []float32
	Importance     int
	Metadata       map[string]any
	ExpiresAt      *time.Time
}

type SearchMemoriesRequest struct {
	AgentID   string
	Type      string
	Embedding []float32
	Limit     int
	MinScore  float64
}

type MemorySearchResult struct {
	Memory Memory
	Score  float64
}

type UpdateMemoryRequest struct {
	Content    *string
	Importance *int
}

type UpdateMemoryStoreRequest struct {
	Content        *string
	Embedding      []float32
	ClearEmbedding bool
	Importance     *int
	Metadata       map[string]any
}

// CreateAgentRequest 创建 Agent 请求
type CreateAgentRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Model        string `json:"model,omitempty"`
	SystemPrompt string `json:"systemPrompt,omitempty"`
	Tools        []Tool `json:"tools,omitempty"`
	Config       Config `json:"config,omitempty"`
	IsPublic     bool   `json:"isPublic,omitempty"`
}

// UpdateAgentRequest 更新 Agent 请求
type UpdateAgentRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	Model        *string `json:"model,omitempty"`
	SystemPrompt *string `json:"systemPrompt,omitempty"`
	Tools        []Tool  `json:"tools,omitempty"`
	Config       *Config `json:"config,omitempty"`
	IsPublic     *bool   `json:"isPublic,omitempty"`
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

	// Durable Agent runs
	CreateRun(ctx context.Context, req *CreateRunRequest) (*Run, error)
	GetRun(ctx context.Context, organizationID, id string) (*Run, error)
	ListRuns(ctx context.Context, organizationID, conversationID string) ([]*Run, error)
	UpdateRun(ctx context.Context, organizationID, id string, req UpdateRunRequest) (*Run, error)
	CreateToolRun(ctx context.Context, req *CreateToolRunRequest) (*ToolRun, error)
	GetToolRun(ctx context.Context, organizationID, id string) (*ToolRun, error)
	ListToolRuns(ctx context.Context, organizationID, runID string) ([]*ToolRun, error)
	UpdateToolRun(ctx context.Context, organizationID, id string, req UpdateToolRunRequest) (*ToolRun, error)
	CreatePlanStep(ctx context.Context, req *CreatePlanStepRequest) (*PlanStep, error)
	GetPlanStep(ctx context.Context, organizationID, id string) (*PlanStep, error)
	ListPlanSteps(ctx context.Context, organizationID, runID string) ([]*PlanStep, error)
	UpdatePlanStep(ctx context.Context, organizationID, id string, req UpdatePlanStepRequest) (*PlanStep, error)
	DeletePlanStep(ctx context.Context, organizationID, id string) (*PlanStep, error)

	// Agent memories
	CreateMemory(ctx context.Context, req *CreateMemoryStoreRequest) (*Memory, error)
	ListMemories(ctx context.Context, organizationID, userID string, req ListMemoriesRequest) ([]*Memory, error)
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

	normalizedConfig, err := NormalizeConfigForWrite(req.Config)
	if err != nil {
		return nil, err
	}
	req.Config = normalizedConfig
	toolsJSON, _ := json.Marshal(req.Tools)
	configJSON, _ := json.Marshal(req.Config)

	_, err = s.db.ExecContext(ctx, `
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
		normalizedConfig, err := NormalizeConfigForWrite(*req.Config)
		if err != nil {
			return nil, err
		}
		agent.Config = normalizedConfig
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

func (s *SQLStore) CreateRun(ctx context.Context, req *CreateRunRequest) (*Run, error) {
	id := generateID()
	now := time.Now()
	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	status := req.Status
	if status == "" {
		status = RunStatusRunning
	}
	mode := NormalizeExecutionMode(req.Mode)

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_runs (
			id, organization_id, conversation_id, agent_id, user_id, request_id, mode, status,
			memory_enabled, memory_searched, memory_result_count, started_at, created_at, updated_at
		)
		SELECT $1, c.organization_id, c.id, c.agent_id, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13
		FROM agent_conversations c
		WHERE c.id = $2 AND c.organization_id = $3 AND c.agent_id = $4
	`, id, req.ConversationID, req.OrganizationID, req.AgentID, req.UserID, req.RequestID, mode, status,
		req.MemoryEnabled, req.MemorySearched, req.MemoryResultCount, startedAt, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent run: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert agent run rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("conversation not found")
	}

	return s.GetRun(ctx, req.OrganizationID, id)
}

func (s *SQLStore) GetRun(ctx context.Context, organizationID, id string) (*Run, error) {
	run, err := scanRun(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, conversation_id, agent_id, user_id, request_id, mode, status,
			memory_enabled, memory_searched, memory_result_count, iteration_count, tool_call_count,
			final_message_id, error, started_at, completed_at, created_at, updated_at
		FROM agent_runs
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent run: %w", err)
	}
	return run, nil
}

func (s *SQLStore) ListRuns(ctx context.Context, organizationID, conversationID string) ([]*Run, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, conversation_id, agent_id, user_id, request_id, mode, status,
			memory_enabled, memory_searched, memory_result_count, iteration_count, tool_call_count,
			final_message_id, error, started_at, completed_at, created_at, updated_at
		FROM agent_runs
		WHERE organization_id = $1 AND conversation_id = $2
		ORDER BY created_at DESC
	`, organizationID, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}
	defer rows.Close()

	var runs []*Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *SQLStore) UpdateRun(ctx context.Context, organizationID, id string, req UpdateRunRequest) (*Run, error) {
	run, err := s.GetRun(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}

	if req.Status != nil {
		run.Status = *req.Status
	}
	if req.MemoryEnabled != nil {
		run.MemoryEnabled = *req.MemoryEnabled
	}
	if req.MemorySearched != nil {
		run.MemorySearched = *req.MemorySearched
	}
	if req.MemoryResultCount != nil {
		run.MemoryResultCount = *req.MemoryResultCount
	}
	if req.IterationCount != nil {
		run.IterationCount = *req.IterationCount
	}
	if req.ToolCallCount != nil {
		run.ToolCallCount = *req.ToolCallCount
	}
	if req.FinalMessageID != nil {
		run.FinalMessageID = *req.FinalMessageID
	}
	if req.Error != nil {
		run.Error = *req.Error
	}
	if req.ClearCompletedAt {
		run.CompletedAt = nil
	} else if req.CompletedAt != nil {
		run.CompletedAt = req.CompletedAt
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = $3,
			memory_enabled = $4,
			memory_searched = $5,
			memory_result_count = $6,
			iteration_count = $7,
			tool_call_count = $8,
			final_message_id = NULLIF($9, ''),
			error = $10,
			completed_at = CASE WHEN $11 THEN NULL::timestamptz ELSE $12::timestamptz END,
			updated_at = $13
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID, run.Status, run.MemoryEnabled, run.MemorySearched, run.MemoryResultCount,
		run.IterationCount, run.ToolCallCount, run.FinalMessageID, run.Error, req.ClearCompletedAt, run.CompletedAt, time.Now())
	if err != nil {
		return nil, fmt.Errorf("update agent run: %w", err)
	}
	return s.GetRun(ctx, organizationID, id)
}

func (s *SQLStore) CreateToolRun(ctx context.Context, req *CreateToolRunRequest) (*ToolRun, error) {
	id := generateID()
	now := time.Now()
	status := req.Status
	if status == "" {
		status = ToolRunStatusRunning
	}
	approvalStatus := req.ApprovalStatus
	if approvalStatus == "" {
		approvalStatus = ApprovalStatusNotRequired
	}
	arguments := req.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	argumentsJSON, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal tool run arguments: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_tool_runs (
			id, organization_id, run_id, conversation_id, agent_id, tool_call_id, tool_name,
			tool_type, server_id, risk_level, arguments, status, approval_status,
			approved_by_user_id, approval_decision_reason, attempt_count, result_content,
			error, started_at, completed_at, created_at, updated_at
		)
		SELECT $1, r.organization_id, r.id, r.conversation_id, r.agent_id, $5, $6,
			$7, $8, $9, $10, $11, $12, NULLIF($13, ''), $14, $15, $16, $17, $18, $19, $20, $20
		FROM agent_runs r
		WHERE r.id = $2 AND r.organization_id = $3 AND r.conversation_id = $4
	`, id, req.RunID, req.OrganizationID, req.ConversationID, req.ToolCallID, req.ToolName,
		req.ToolType, req.ServerID, req.RiskLevel, argumentsJSON, status, approvalStatus, req.ApprovedByUserID,
		req.ApprovalDecisionReason, req.AttemptCount, req.ResultContent, req.Error,
		req.StartedAt, req.CompletedAt, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent tool run: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert agent tool run rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("agent run not found")
	}
	return s.GetToolRun(ctx, req.OrganizationID, id)
}

func (s *SQLStore) GetToolRun(ctx context.Context, organizationID, id string) (*ToolRun, error) {
	toolRun, err := scanToolRun(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, run_id, conversation_id, agent_id, tool_call_id, tool_name,
			tool_type, server_id, risk_level, arguments, status, approval_status, approved_by_user_id,
			approval_decision_reason, attempt_count, result_content, error, started_at,
			completed_at, created_at, updated_at
		FROM agent_tool_runs
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent tool run: %w", err)
	}
	return toolRun, nil
}

func (s *SQLStore) ListToolRuns(ctx context.Context, organizationID, runID string) ([]*ToolRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, run_id, conversation_id, agent_id, tool_call_id, tool_name,
			tool_type, server_id, risk_level, arguments, status, approval_status, approved_by_user_id,
			approval_decision_reason, attempt_count, result_content, error, started_at,
			completed_at, created_at, updated_at
		FROM agent_tool_runs
		WHERE organization_id = $1 AND run_id = $2
		ORDER BY created_at ASC
	`, organizationID, runID)
	if err != nil {
		return nil, fmt.Errorf("list agent tool runs: %w", err)
	}
	defer rows.Close()

	var toolRuns []*ToolRun
	for rows.Next() {
		toolRun, err := scanToolRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent tool run: %w", err)
		}
		toolRuns = append(toolRuns, toolRun)
	}
	return toolRuns, rows.Err()
}

func (s *SQLStore) CreatePlanStep(ctx context.Context, req *CreatePlanStepRequest) (*PlanStep, error) {
	id := generateID()
	now := time.Now()
	status := req.Status
	if status == "" {
		status = PlanStepStatusPending
	}
	approvalStatus := req.ApprovalStatus
	if approvalStatus == "" {
		approvalStatus = ApprovalStatusNotRequired
	}
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal plan step input: %w", err)
	}
	dependsOn := normalizePlanStepDependsOn(req.DependsOn)
	dependsOnJSON, err := marshalPlanStepDependsOn(dependsOn)
	if err != nil {
		return nil, fmt.Errorf("marshal plan step dependencies: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_plan_steps (
			id, organization_id, run_id, step_index, title, status, approval_status,
			tool_name, input, description, depends_on, created_at, updated_at
		)
		SELECT $1, r.organization_id, r.id, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12
		FROM agent_runs r
		WHERE r.id = $2 AND r.organization_id = $3
	`, id, req.RunID, req.OrganizationID, req.Index, req.Title, status, approvalStatus, req.ToolName, inputJSON, req.Description, dependsOnJSON, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent plan step: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("insert agent plan step rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("agent run not found")
	}

	steps, err := s.ListPlanSteps(ctx, req.OrganizationID, req.RunID)
	if err != nil {
		return nil, err
	}
	for _, step := range steps {
		if step.ID == id {
			return step, nil
		}
	}
	return nil, fmt.Errorf("agent plan step not found")
}

func (s *SQLStore) ListPlanSteps(ctx context.Context, organizationID, runID string) ([]*PlanStep, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, run_id, step_index, title, status, approval_status,
			tool_name, input, description, depends_on, result_content, error, started_at, completed_at, created_at, updated_at
		FROM agent_plan_steps
		WHERE organization_id = $1 AND run_id = $2
		ORDER BY step_index ASC, created_at ASC
	`, organizationID, runID)
	if err != nil {
		return nil, fmt.Errorf("list agent plan steps: %w", err)
	}
	defer rows.Close()

	var steps []*PlanStep
	for rows.Next() {
		step, err := scanPlanStep(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent plan step: %w", err)
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *SQLStore) GetPlanStep(ctx context.Context, organizationID, id string) (*PlanStep, error) {
	step, err := scanPlanStep(s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, run_id, step_index, title, status, approval_status,
			tool_name, input, description, depends_on, result_content, error, started_at, completed_at, created_at, updated_at
		FROM agent_plan_steps
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent plan step: %w", err)
	}
	return step, nil
}

func (s *SQLStore) UpdatePlanStep(ctx context.Context, organizationID, id string, req UpdatePlanStepRequest) (*PlanStep, error) {
	step, err := s.GetPlanStep(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("agent plan step not found")
	}

	if req.Index != nil {
		step.Index = *req.Index
	}
	if req.Title != nil {
		step.Title = *req.Title
	}
	if req.Description != nil {
		step.Description = *req.Description
	}
	if req.Status != nil {
		step.Status = *req.Status
	}
	if req.ApprovalStatus != nil {
		step.ApprovalStatus = *req.ApprovalStatus
	}
	if req.ToolName != nil {
		step.ToolName = *req.ToolName
	}
	if req.ReplaceInput {
		step.Input = map[string]any{}
		if req.Input != nil {
			step.Input = req.Input
		}
	}
	if req.ReplaceDependsOn {
		step.DependsOn = normalizePlanStepDependsOn(req.DependsOn)
	}
	if req.ResultContent != nil {
		step.ResultContent = *req.ResultContent
	}
	if req.Error != nil {
		step.Error = *req.Error
	}
	if req.StartedAt != nil {
		step.StartedAt = req.StartedAt
	}
	if req.ClearCompletedAt {
		step.CompletedAt = nil
	} else if req.CompletedAt != nil {
		step.CompletedAt = req.CompletedAt
	}
	inputJSON, err := json.Marshal(step.Input)
	if err != nil {
		return nil, fmt.Errorf("marshal plan step input: %w", err)
	}
	dependsOnJSON, err := marshalPlanStepDependsOn(step.DependsOn)
	if err != nil {
		return nil, fmt.Errorf("marshal plan step dependencies: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE agent_plan_steps
		SET step_index = $3,
			title = $4,
			description = $5,
			status = $6,
			approval_status = $7,
			tool_name = $8,
			input = $9,
			depends_on = $10,
			result_content = $11,
			error = $12,
			started_at = $13,
			completed_at = CASE WHEN $14 THEN NULL::timestamptz ELSE $15::timestamptz END,
			updated_at = $16
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID, step.Index, step.Title, step.Description, step.Status, step.ApprovalStatus, step.ToolName, inputJSON,
		dependsOnJSON, step.ResultContent, step.Error, step.StartedAt, req.ClearCompletedAt, step.CompletedAt, time.Now())
	if err != nil {
		return nil, fmt.Errorf("update agent plan step: %w", err)
	}
	return s.GetPlanStep(ctx, organizationID, id)
}

func (s *SQLStore) DeletePlanStep(ctx context.Context, organizationID, id string) (*PlanStep, error) {
	step, err := s.GetPlanStep(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("agent plan step not found")
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM agent_plan_steps
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID)
	if err != nil {
		return nil, fmt.Errorf("delete agent plan step: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("delete agent plan step rows: %w", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("agent plan step not found")
	}
	return step, nil
}

func (s *SQLStore) UpdateToolRun(ctx context.Context, organizationID, id string, req UpdateToolRunRequest) (*ToolRun, error) {
	toolRun, err := s.GetToolRun(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if toolRun == nil {
		return nil, fmt.Errorf("agent tool run not found")
	}

	if req.Status != nil {
		toolRun.Status = *req.Status
	}
	if req.ApprovalStatus != nil {
		toolRun.ApprovalStatus = *req.ApprovalStatus
	}
	if req.ApprovedByUserID != nil {
		toolRun.ApprovedByUserID = *req.ApprovedByUserID
	}
	if req.ApprovalDecisionReason != nil {
		toolRun.ApprovalDecisionReason = *req.ApprovalDecisionReason
	}
	if req.AttemptCount != nil {
		toolRun.AttemptCount = *req.AttemptCount
	}
	if req.ResultContent != nil {
		toolRun.ResultContent = *req.ResultContent
	}
	if req.Error != nil {
		toolRun.Error = *req.Error
	}
	if req.StartedAt != nil {
		toolRun.StartedAt = req.StartedAt
	}
	if req.ClearCompletedAt {
		toolRun.CompletedAt = nil
	} else if req.CompletedAt != nil {
		toolRun.CompletedAt = req.CompletedAt
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE agent_tool_runs
		SET status = $3,
			approval_status = $4,
			approved_by_user_id = NULLIF($5, ''),
			approval_decision_reason = $6,
			attempt_count = $7,
			result_content = $8,
			error = $9,
			started_at = $10,
			completed_at = CASE WHEN $11 THEN NULL::timestamptz ELSE $12::timestamptz END,
			updated_at = $13
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID, toolRun.Status, toolRun.ApprovalStatus, toolRun.ApprovedByUserID,
		toolRun.ApprovalDecisionReason, toolRun.AttemptCount, toolRun.ResultContent, toolRun.Error,
		toolRun.StartedAt, req.ClearCompletedAt, toolRun.CompletedAt, time.Now())
	if err != nil {
		return nil, fmt.Errorf("update agent tool run: %w", err)
	}
	return s.GetToolRun(ctx, organizationID, id)
}

func (s *SQLStore) CreateMemory(ctx context.Context, req *CreateMemoryStoreRequest) (*Memory, error) {
	id := fmt.Sprintf("agentmem_%d", time.Now().UnixNano())
	now := time.Now()
	metadata := copyMetadata(req.Metadata)
	importance, err := normalizeMemoryImportance(req.Importance)
	if err != nil {
		return nil, err
	}
	metadata[memoryMetadataImportanceKey] = importance
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal agent memory metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agent_memories (
			id, organization_id, user_id, agent_id, type, content, embedding, metadata, expires_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NULLIF($7, '')::vector, $8, $9, $10, $10)
	`, id, req.OrganizationID, req.UserID, req.AgentID, req.Type, req.Content, agentMemoryEmbeddingToVector(req.Embedding), metadataJSON, req.ExpiresAt, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent memory: %w", err)
	}

	return &Memory{
		ID:             id,
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		AgentID:        req.AgentID,
		Type:           req.Type,
		Content:        req.Content,
		Importance:     importance,
		Metadata:       publicMemoryMetadata(metadata),
		ExpiresAt:      req.ExpiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (s *SQLStore) GetMemory(ctx context.Context, organizationID, id string) (*Memory, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, organization_id, user_id, agent_id, type, content, metadata, expires_at, created_at, updated_at
		FROM agent_memories
		WHERE id = $1 AND organization_id = $2
	`, id, organizationID)
	memory, err := scanMemory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent memory: %w", err)
	}
	return memory, nil
}

func (s *SQLStore) ListMemories(ctx context.Context, organizationID, userID string, req ListMemoriesRequest) ([]*Memory, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, agent_id, type, content, metadata, expires_at, created_at, updated_at
		FROM agent_memories
		WHERE organization_id = $1
			AND user_id = $2
			AND ($3 = '' OR agent_id = $3)
			AND ($4 = '' OR type = $4)
			AND ($5 = '' OR content ILIKE '%' || $5 || '%' OR metadata::text ILIKE '%' || $5 || '%')
			AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
		LIMIT $6 OFFSET $7
	`, organizationID, userID, req.AgentID, req.Type, req.Query, limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("list agent memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		memory, err := scanMemory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agent memory: %w", err)
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

func (s *SQLStore) SearchMemories(ctx context.Context, organizationID, userID string, req SearchMemoriesRequest) ([]*MemorySearchResult, error) {
	if len(req.Embedding) == 0 {
		return []*MemorySearchResult{}, nil
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	minScore := req.MinScore
	if minScore <= 0 {
		minScore = 0.5
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, organization_id, user_id, agent_id, type, content, metadata, expires_at, created_at, updated_at,
			1 - (embedding <=> $5::vector) AS similarity
		FROM agent_memories
		WHERE organization_id = $1
			AND user_id = $2
			AND ($3 = '' OR agent_id = $3)
			AND ($4 = '' OR type = $4)
			AND embedding IS NOT NULL
			AND (expires_at IS NULL OR expires_at > NOW())
			AND (1 - (embedding <=> $5::vector)) >= $6
		ORDER BY embedding <=> $5::vector, updated_at DESC
		LIMIT $7
	`, organizationID, userID, req.AgentID, req.Type, agentMemoryEmbeddingToVector(req.Embedding), minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("search agent memories: %w", err)
	}
	defer rows.Close()

	results := make([]*MemorySearchResult, 0, limit)
	for rows.Next() {
		var memory Memory
		var agentID sql.NullString
		var metadataJSON []byte
		var expiresAt sql.NullTime
		var score float64
		if err := rows.Scan(
			&memory.ID,
			&memory.OrganizationID,
			&memory.UserID,
			&agentID,
			&memory.Type,
			&memory.Content,
			&metadataJSON,
			&expiresAt,
			&memory.CreatedAt,
			&memory.UpdatedAt,
			&score,
		); err != nil {
			return nil, fmt.Errorf("scan agent memory search result: %w", err)
		}
		memory.AgentID = agentID.String
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &memory.Metadata)
		}
		if memory.Metadata == nil {
			memory.Metadata = map[string]any{}
		}
		memory.Importance = memoryImportanceFromMetadata(memory.Metadata)
		memory.Metadata = publicMemoryMetadata(memory.Metadata)
		if expiresAt.Valid {
			memory.ExpiresAt = &expiresAt.Time
		}
		results = append(results, &MemorySearchResult{Memory: memory, Score: score})
	}
	return results, rows.Err()
}

func (s *SQLStore) UpdateMemory(ctx context.Context, organizationID, userID, id string, req UpdateMemoryStoreRequest) (*Memory, error) {
	memory, err := s.GetMemory(ctx, organizationID, id)
	if err != nil {
		return nil, err
	}
	if memory == nil || memory.UserID != userID {
		return nil, fmt.Errorf("memory not found")
	}

	if req.Content != nil {
		memory.Content = *req.Content
	}
	if req.Importance != nil {
		memory.Importance = *req.Importance
	}
	metadata := copyMetadata(memory.Metadata)
	for key, value := range req.Metadata {
		if strings.TrimSpace(key) == "" {
			continue
		}
		metadata[key] = value
	}
	metadata[memoryMetadataImportanceKey] = memory.Importance
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal agent memory metadata: %w", err)
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx, `
		UPDATE agent_memories
		SET content = $4,
			metadata = $5,
			embedding = CASE
				WHEN $6 <> '' THEN $6::vector
				WHEN $7 THEN NULL
				ELSE embedding
			END,
			updated_at = $8
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
	`, id, organizationID, userID, memory.Content, metadataJSON, agentMemoryEmbeddingToVector(req.Embedding), req.ClearEmbedding, now)
	if err != nil {
		return nil, fmt.Errorf("update agent memory: %w", err)
	}

	memory.Metadata = publicMemoryMetadata(metadata)
	memory.UpdatedAt = now
	return memory, nil
}

func (s *SQLStore) DeleteMemory(ctx context.Context, organizationID, userID, id string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM agent_memories
		WHERE id = $1 AND organization_id = $2 AND user_id = $3
	`, id, organizationID, userID)
	if err != nil {
		return fmt.Errorf("delete agent memory: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return fmt.Errorf("memory not found")
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (*Run, error) {
	var run Run
	var finalMessageID sql.NullString
	var completedAt sql.NullTime
	err := row.Scan(
		&run.ID,
		&run.OrganizationID,
		&run.ConversationID,
		&run.AgentID,
		&run.UserID,
		&run.RequestID,
		&run.Mode,
		&run.Status,
		&run.MemoryEnabled,
		&run.MemorySearched,
		&run.MemoryResultCount,
		&run.IterationCount,
		&run.ToolCallCount,
		&finalMessageID,
		&run.Error,
		&run.StartedAt,
		&completedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	run.Mode = NormalizeExecutionMode(run.Mode)
	run.FinalMessageID = finalMessageID.String
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return &run, nil
}

func scanToolRun(row scanner) (*ToolRun, error) {
	var toolRun ToolRun
	var argumentsJSON []byte
	var approvedByUserID sql.NullString
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	err := row.Scan(
		&toolRun.ID,
		&toolRun.OrganizationID,
		&toolRun.RunID,
		&toolRun.ConversationID,
		&toolRun.AgentID,
		&toolRun.ToolCallID,
		&toolRun.ToolName,
		&toolRun.ToolType,
		&toolRun.ServerID,
		&toolRun.RiskLevel,
		&argumentsJSON,
		&toolRun.Status,
		&toolRun.ApprovalStatus,
		&approvedByUserID,
		&toolRun.ApprovalDecisionReason,
		&toolRun.AttemptCount,
		&toolRun.ResultContent,
		&toolRun.Error,
		&startedAt,
		&completedAt,
		&toolRun.CreatedAt,
		&toolRun.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	toolRun.ApprovedByUserID = approvedByUserID.String
	if len(argumentsJSON) > 0 {
		_ = json.Unmarshal(argumentsJSON, &toolRun.Arguments)
	}
	if toolRun.Arguments == nil {
		toolRun.Arguments = map[string]any{}
	}
	if startedAt.Valid {
		toolRun.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		toolRun.CompletedAt = &completedAt.Time
	}
	return &toolRun, nil
}

func scanPlanStep(row scanner) (*PlanStep, error) {
	var step PlanStep
	var inputJSON []byte
	var dependsOnJSON []byte
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	err := row.Scan(
		&step.ID,
		&step.OrganizationID,
		&step.RunID,
		&step.Index,
		&step.Title,
		&step.Status,
		&step.ApprovalStatus,
		&step.ToolName,
		&inputJSON,
		&step.Description,
		&dependsOnJSON,
		&step.ResultContent,
		&step.Error,
		&startedAt,
		&completedAt,
		&step.CreatedAt,
		&step.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(inputJSON) > 0 {
		_ = json.Unmarshal(inputJSON, &step.Input)
	}
	if step.Input == nil {
		step.Input = map[string]any{}
	}
	if len(dependsOnJSON) > 0 {
		_ = json.Unmarshal(dependsOnJSON, &step.DependsOn)
	}
	step.DependsOn = normalizePlanStepDependsOn(step.DependsOn)
	if startedAt.Valid {
		step.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		step.CompletedAt = &completedAt.Time
	}
	return &step, nil
}

func scanMemory(row scanner) (*Memory, error) {
	var memory Memory
	var agentID sql.NullString
	var metadataJSON []byte
	var expiresAt sql.NullTime
	err := row.Scan(
		&memory.ID,
		&memory.OrganizationID,
		&memory.UserID,
		&agentID,
		&memory.Type,
		&memory.Content,
		&metadataJSON,
		&expiresAt,
		&memory.CreatedAt,
		&memory.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	memory.AgentID = agentID.String
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &memory.Metadata)
	}
	if memory.Metadata == nil {
		memory.Metadata = map[string]any{}
	}
	memory.Importance = memoryImportanceFromMetadata(memory.Metadata)
	memory.Metadata = publicMemoryMetadata(memory.Metadata)
	if expiresAt.Valid {
		memory.ExpiresAt = &expiresAt.Time
	}
	return &memory, nil
}

func memoryImportanceFromMetadata(metadata map[string]any) int {
	value, ok := metadata[memoryMetadataImportanceKey]
	if !ok {
		return 3
	}
	switch typed := value.(type) {
	case int:
		if typed >= 1 && typed <= 5 {
			return typed
		}
	case int64:
		if typed >= 1 && typed <= 5 {
			return int(typed)
		}
	case float64:
		if typed >= 1 && typed <= 5 && math.Trunc(typed) == typed {
			return int(typed)
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed >= 1 && parsed <= 5 {
			return int(parsed)
		}
	}
	return 3
}

func publicMemoryMetadata(metadata map[string]any) map[string]any {
	copied := copyMetadata(metadata)
	delete(copied, memoryMetadataImportanceKey)
	return copied
}

func agentMemoryEmbeddingToVector(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}
	parts := make([]string, len(embedding))
	for i, value := range embedding {
		parts[i] = fmt.Sprintf("%f", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func generateID() string {
	return fmt.Sprintf("agent_%d", time.Now().UnixNano())
}
