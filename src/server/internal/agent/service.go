package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/metrics"
	relaytypes "oblivious/server/internal/relay/types"
)

// MemorySearcher Memory 搜索接口
type MemorySearcher interface {
	Search(ctx context.Context, session auth.Session, req *memory.SearchRequest) ([]*memory.SearchResult, error)
}

type MemoryEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type memoryManagementStore interface {
	GetMemory(ctx context.Context, organizationID, id string) (*Memory, error)
	UpdateMemory(ctx context.Context, organizationID, userID, id string, req UpdateMemoryStoreRequest) (*Memory, error)
	DeleteMemory(ctx context.Context, organizationID, userID, id string) error
}

// Service Agent 服务
type Service struct {
	store             Store
	gateway           chat.ChatGateway
	mcpClient         *mcp.Client
	memory            MemorySearcher
	memoryEmbedder    MemoryEmbedder
	webSearchProvider mcp.WebSearchProvider
	runner            *Runner
	planStepExecutor  PlanStepExecutor
}

type PlanStepExecutionResult struct {
	ResultContent string
}

type PlanStepExecutor interface {
	ExecutePlanStep(ctx context.Context, step *PlanStep) (*PlanStepExecutionResult, error)
}

type planStepTokenBudgetOverrideContextKey struct{}

func withPlanStepTokenBudgetOverride(ctx context.Context, tokenBudget int) context.Context {
	return context.WithValue(ctx, planStepTokenBudgetOverrideContextKey{}, tokenBudget)
}

func planStepTokenBudgetOverrideFromContext(ctx context.Context) (int, bool) {
	tokenBudget, ok := ctx.Value(planStepTokenBudgetOverrideContextKey{}).(int)
	return tokenBudget, ok
}

type defaultPlanStepExecutor struct {
	executor *ToolExecutor
	store    Store
	gateway  chat.ChatGateway
}

func (e defaultPlanStepExecutor) ExecutePlanStep(ctx context.Context, step *PlanStep) (*PlanStepExecutionResult, error) {
	if step == nil {
		return nil, fmt.Errorf("plan step not found")
	}
	if strings.TrimSpace(step.ToolName) != "" {
		executor := e.executor
		if executor == nil {
			executor = NewToolExecutor(nil)
		}
		arguments := step.Input
		if arguments == nil {
			arguments = map[string]any{}
		}
		result, err := executor.Execute(ctx, &Agent{
			OrganizationID: step.OrganizationID,
			Tools: []Tool{{
				Name:    step.ToolName,
				Type:    "builtin",
				Enabled: true,
			}},
		}, &ToolCall{
			ID:        "plan_step_" + step.ID,
			Name:      step.ToolName,
			Arguments: arguments,
		})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("tool %s returned no result", step.ToolName)
		}
		if result.IsError {
			return nil, fmt.Errorf("tool %s failed: %s", step.ToolName, result.Content)
		}
		return &PlanStepExecutionResult{ResultContent: result.Content}, nil
	}
	if e.store == nil {
		return nil, fmt.Errorf("plan step executor store is not configured")
	}
	if e.gateway == nil {
		return nil, fmt.Errorf("plan step executor gateway is not configured")
	}
	run, err := e.store.GetRun(ctx, step.OrganizationID, step.RunID)
	if err != nil {
		return nil, fmt.Errorf("get plan step run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("agent run not found")
	}
	agent, err := e.store.GetAgent(ctx, run.AgentID, step.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("get plan step agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	messages, err := e.store.ListMessages(ctx, run.ConversationID, step.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("get plan step messages: %w", err)
	}
	steps, err := e.store.ListPlanSteps(ctx, step.OrganizationID, step.RunID)
	if err != nil {
		return nil, fmt.Errorf("get plan steps: %w", err)
	}
	chatMessages := buildPlanStepExecutionMessages(run, agent, step, messages, steps)
	tokenBudget := normalizeTokenBudget(agent.Config.TokenBudget)
	if override, ok := planStepTokenBudgetOverrideFromContext(ctx); ok {
		tokenBudget = normalizeTokenBudget(override)
	}
	estimatedTokens := estimateChatMessageTokens(chatMessages)
	if tokenBudget > 0 && estimatedTokens > tokenBudget {
		return nil, fmt.Errorf("%w: estimated %d prompt tokens exceeds budget %d", ErrTokenBudgetExceeded, estimatedTokens, tokenBudget)
	}
	reply, err := e.gateway.GenerateReply(ctx, chatMessages, planStepExecutionConversationConfig(run, agent))
	if err != nil {
		return nil, fmt.Errorf("execute plan step with model: %w", err)
	}
	return &PlanStepExecutionResult{ResultContent: strings.TrimSpace(reply)}, nil
}

// NewService 创建 Service
func NewService(store Store, gateway chat.ChatGateway) *Service {
	s := &Service{
		store:   store,
		gateway: gateway,
	}
	s.initRunner()
	return s
}

// NewServiceWithMCP 创建带 MCP 的 Service
func NewServiceWithMCP(store Store, gateway chat.ChatGateway, mcpClient *mcp.Client) *Service {
	s := &Service{
		store:     store,
		gateway:   gateway,
		mcpClient: mcpClient,
	}
	s.initRunner()
	return s
}

// NewServiceWithMemory 创建带 Memory 的 Service
func NewServiceWithMemory(store Store, gateway chat.ChatGateway, mcpClient *mcp.Client, mem MemorySearcher) *Service {
	s := &Service{
		store:     store,
		gateway:   gateway,
		mcpClient: mcpClient,
		memory:    mem,
	}
	s.initRunner()
	return s
}

// initRunner 初始化 Runner
func (s *Service) initRunner() {
	executor := NewToolExecutor(s.mcpClient)
	executor.SetWebSearchProvider(s.webSearchProvider)
	s.runner = NewRunner(s.store, s.gateway, executor, s.memory, DefaultRunnerConfig())
	s.runner.SetMemoryEmbedder(s.memoryEmbedder)
	if s.planStepExecutor == nil {
		s.planStepExecutor = defaultPlanStepExecutor{executor: executor, store: s.store, gateway: s.gateway}
	} else if _, ok := s.planStepExecutor.(defaultPlanStepExecutor); ok {
		s.planStepExecutor = defaultPlanStepExecutor{executor: executor, store: s.store, gateway: s.gateway}
	}
}

// SetMCPClient 设置 MCP 客户端
func (s *Service) SetMCPClient(client *mcp.Client) {
	s.mcpClient = client
	s.initRunner()
}

// SetMemory 设置 Memory
func (s *Service) SetMemory(mem MemorySearcher) {
	s.memory = mem
	s.initRunner()
}

func (s *Service) SetMemoryEmbedder(embedder MemoryEmbedder) {
	s.memoryEmbedder = embedder
	if s.runner != nil {
		s.runner.SetMemoryEmbedder(embedder)
	}
}

func (s *Service) SetWebSearchProvider(provider mcp.WebSearchProvider) {
	s.webSearchProvider = provider
	if s.runner != nil && s.runner.executor != nil {
		s.runner.executor.SetWebSearchProvider(provider)
	}
	if executor, ok := s.planStepExecutor.(defaultPlanStepExecutor); ok && executor.executor != nil {
		executor.executor.SetWebSearchProvider(provider)
		s.planStepExecutor = executor
	}
}

func (s *Service) SetPlanStepExecutor(executor PlanStepExecutor) {
	if executor == nil {
		toolExecutor := NewToolExecutor(s.mcpClient)
		toolExecutor.SetWebSearchProvider(s.webSearchProvider)
		s.planStepExecutor = defaultPlanStepExecutor{executor: toolExecutor, store: s.store, gateway: s.gateway}
		return
	}
	s.planStepExecutor = executor
}

// CreateAgent 创建 Agent
func (s *Service) CreateAgent(ctx context.Context, session auth.Session, req *CreateAgentRequest) (*Agent, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	normalizedConfig, err := NormalizeConfigForWrite(req.Config)
	if err != nil {
		return nil, err
	}
	req.Config = normalizedConfig
	return s.store.CreateAgent(ctx, session.User.ID, session.OrganizationID, req)
}

// GetAgent 获取 Agent
func (s *Service) GetAgent(ctx context.Context, session auth.Session, id string) (*Agent, error) {
	agent, err := s.store.GetAgent(ctx, id, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	// 验证所有权
	if agent.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	return agent, nil
}

// ListAgents 列出用户的 Agent
func (s *Service) ListAgents(ctx context.Context, session auth.Session) ([]*Agent, error) {
	return s.store.ListAgents(ctx, session.User.ID, session.OrganizationID)
}

// UpdateAgent 更新 Agent
func (s *Service) UpdateAgent(ctx context.Context, session auth.Session, id string, req *UpdateAgentRequest) (*Agent, error) {
	// 验证所有权
	agent, err := s.store.GetAgent(ctx, id, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	normalizedReq := *req
	if req.Config != nil {
		normalizedConfig, err := NormalizeConfigForWrite(*req.Config)
		if err != nil {
			return nil, err
		}
		normalizedReq.Config = &normalizedConfig
	}
	return s.store.UpdateAgent(ctx, id, session.OrganizationID, &normalizedReq)
}

// DeleteAgent 删除 Agent
func (s *Service) DeleteAgent(ctx context.Context, session auth.Session, id string) error {
	// 验证所有权
	agent, err := s.store.GetAgent(ctx, id, session.OrganizationID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	return s.store.DeleteAgent(ctx, id, session.OrganizationID)
}

func (s *Service) CreateMemory(ctx context.Context, session auth.Session, req CreateMemoryRequest) (*Memory, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	agentID := strings.TrimSpace(req.AgentID)
	if agentID != "" {
		agent, err := s.store.GetAgent(ctx, agentID, session.OrganizationID)
		if err != nil {
			return nil, err
		}
		if agent == nil {
			return nil, fmt.Errorf("agent not found")
		}
		if agent.UserID != session.User.ID {
			return nil, fmt.Errorf("access denied")
		}
	}

	memoryType := normalizeMemoryType(req.Type)
	metadata := copyMetadata(req.Metadata)
	importance, err := normalizeMemoryImportance(req.Importance)
	if err != nil {
		return nil, err
	}
	embedding := s.embedMemoryContent(ctx, session, content)
	memory, err := s.store.CreateMemory(ctx, &CreateMemoryStoreRequest{
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		AgentID:        agentID,
		Type:           memoryType,
		Content:        content,
		Embedding:      embedding,
		Importance:     importance,
		Metadata:       metadata,
		ExpiresAt:      req.ExpiresAt,
	})
	if err != nil {
		return nil, err
	}
	if memory != nil && memory.Importance == 0 {
		memory.Importance = importance
	}
	return memory, nil
}

func (s *Service) ListMemories(ctx context.Context, session auth.Session, req ListMemoriesRequest) ([]*Memory, error) {
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Type = strings.TrimSpace(req.Type)
	req.Query = strings.TrimSpace(req.Query)
	if req.AgentID != "" {
		agent, err := s.store.GetAgent(ctx, req.AgentID, session.OrganizationID)
		if err != nil {
			return nil, err
		}
		if agent == nil {
			return nil, fmt.Errorf("agent not found")
		}
		if agent.UserID != session.User.ID && !agent.IsPublic {
			return nil, fmt.Errorf("access denied")
		}
	}
	if req.Type != "" {
		req.Type = normalizeMemoryType(req.Type)
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	return s.store.ListMemories(ctx, session.OrganizationID, session.User.ID, req)
}

func (s *Service) UpdateMemory(ctx context.Context, session auth.Session, id string, req UpdateMemoryRequest) (*Memory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("memory not found")
	}

	managementStore, ok := s.store.(memoryManagementStore)
	if !ok {
		return nil, fmt.Errorf("memory management is not configured")
	}

	memory, err := managementStore.GetMemory(ctx, session.OrganizationID, id)
	if err != nil {
		return nil, err
	}
	if memory == nil {
		return nil, fmt.Errorf("memory not found")
	}
	if memory.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	updateReq := UpdateMemoryStoreRequest{}
	if req.Content != nil {
		content := strings.TrimSpace(*req.Content)
		if content == "" {
			return nil, fmt.Errorf("content is required")
		}
		updateReq.Content = &content
		if embedding := s.embedMemoryContent(ctx, session, content); len(embedding) > 0 {
			updateReq.Embedding = embedding
		} else {
			updateReq.ClearEmbedding = true
		}
	}
	if req.Importance != nil {
		importance := *req.Importance
		if importance < 1 || importance > 5 {
			return nil, fmt.Errorf("importance must be between 1 and 5")
		}
		updateReq.Importance = &importance
	}

	return managementStore.UpdateMemory(ctx, session.OrganizationID, session.User.ID, id, updateReq)
}

func (s *Service) DeleteMemory(ctx context.Context, session auth.Session, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("memory not found")
	}

	managementStore, ok := s.store.(memoryManagementStore)
	if !ok {
		return fmt.Errorf("memory management is not configured")
	}

	memory, err := managementStore.GetMemory(ctx, session.OrganizationID, id)
	if err != nil {
		return err
	}
	if memory == nil {
		return fmt.Errorf("memory not found")
	}
	if memory.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	return managementStore.DeleteMemory(ctx, session.OrganizationID, session.User.ID, id)
}

func (s *Service) embedMemoryContent(ctx context.Context, session auth.Session, content string) []float32 {
	if s.memoryEmbedder == nil || strings.TrimSpace(content) == "" {
		return nil
	}
	embedding, err := s.memoryEmbedder.Embed(withAgentMemoryRelayIdentity(ctx, session), content)
	if err != nil || len(embedding) == 0 {
		return nil
	}
	return embedding
}

func withAgentMemoryRelayIdentity(ctx context.Context, session auth.Session) context.Context {
	if session.User.ID != "" {
		ctx = relaytypes.WithTrustedUserID(ctx, session.User.ID)
	}
	if session.OrganizationID != "" {
		ctx = relaytypes.WithTrustedOrganizationID(ctx, session.OrganizationID)
	}
	return ctx
}

// CreateConversation 创建对话
func (s *Service) CreateConversation(ctx context.Context, session auth.Session, agentID string) (*Conversation, error) {
	// 验证 Agent 存在且可访问
	agent, err := s.store.GetAgent(ctx, agentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.CreateConversation(ctx, agentID, session.User.ID, session.OrganizationID, "")
}

// GetConversation 获取对话
func (s *Service) GetConversation(ctx context.Context, session auth.Session, id string) (*Conversation, error) {
	conv, err := s.store.GetConversation(ctx, id, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}

	// 验证所有权
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	return conv, nil
}

// ListConversations 列出对话
func (s *Service) ListConversations(ctx context.Context, session auth.Session, agentID string) ([]*Conversation, error) {
	agent, err := s.store.GetAgent(ctx, agentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.ListConversations(ctx, agentID, session.User.ID, session.OrganizationID)
}

// DeleteConversation 删除对话
func (s *Service) DeleteConversation(ctx context.Context, session auth.Session, id string) error {
	conv, err := s.store.GetConversation(ctx, id, session.OrganizationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	return s.store.DeleteConversation(ctx, id, session.OrganizationID)
}

func (s *Service) DefaultExecutionModeForRun(ctx context.Context, session auth.Session, agentID string) (string, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return ExecutionModeReact, fmt.Errorf("agent not found")
	}
	agent, err := s.store.GetAgent(ctx, agentID, session.OrganizationID)
	if err != nil {
		return "", err
	}
	if agent == nil {
		return "", fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return "", fmt.Errorf("access denied")
	}
	return NormalizeExecutionMode(agent.Config.DefaultExecutionMode), nil
}

type SendMessageOptions struct {
	Mode          string
	MaxIterations *int
	TokenBudget   *int
}

// SendMessage 发送消息
func (s *Service) SendMessage(ctx context.Context, session auth.Session, conversationID string, content string, options ...SendMessageOptions) (*Message, error) {
	// 获取对话
	conv, err := s.store.GetConversation(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	// 获取 Agent
	agent, err := s.store.GetAgent(ctx, conv.AgentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	runAgent := *agent
	mode := NormalizeExecutionMode(runAgent.Config.DefaultExecutionMode)
	if len(options) > 0 {
		option := options[0]
		overrideMode := strings.ToLower(strings.TrimSpace(option.Mode))
		if overrideMode != "" {
			if overrideMode != ExecutionModeReact && overrideMode != ExecutionModePlanning {
				return nil, fmt.Errorf("mode must be react or planning")
			}
			mode = overrideMode
		}
		if option.MaxIterations != nil {
			runAgent.Config.MaxIterations = *option.MaxIterations
		}
		if option.TokenBudget != nil {
			runAgent.Config.TokenBudget = *option.TokenBudget
		}
	}

	if mode == ExecutionModePlanning {
		result, err := s.StartPlanningRun(ctx, session, StartRunRequest{
			AgentID:        runAgent.ID,
			ConversationID: conversationID,
			Input:          content,
			MaxIterations:  firstSendMessageIntPointer(options, func(option SendMessageOptions) *int { return option.MaxIterations }),
			TokenBudget:    firstSendMessageIntPointer(options, func(option SendMessageOptions) *int { return option.TokenBudget }),
		})
		if err != nil {
			return nil, err
		}
		return lastAssistantMessage(result), nil
	}

	if hasEnabledTools(&runAgent) {
		result, err := s.runner.RunWithTools(ctx, session, &runAgent, conversationID, content, nil)
		if err != nil {
			return nil, err
		}
		return result.Message, nil
	}

	result, err := s.runner.Run(ctx, session, &runAgent, conversationID, content)
	if err != nil {
		return nil, err
	}

	return result.Message, nil
}

func firstSendMessageIntPointer(options []SendMessageOptions, pick func(SendMessageOptions) *int) *int {
	if len(options) == 0 {
		return nil
	}
	return pick(options[0])
}

func lastAssistantMessage(result *RunWithMessages) *Message {
	if result == nil {
		return nil
	}
	if result.Run != nil && result.Run.FinalMessageID != "" {
		for _, message := range result.Messages {
			if message != nil && message.ID == result.Run.FinalMessageID {
				return message
			}
		}
	}
	for i := len(result.Messages) - 1; i >= 0; i-- {
		if result.Messages[i] != nil && result.Messages[i].Role == "assistant" {
			return result.Messages[i]
		}
	}
	return nil
}

func (s *Service) StartRun(ctx context.Context, session auth.Session, req StartRunRequest) (*RunWithMessages, error) {
	content := req.Input
	if content == "" {
		return nil, fmt.Errorf("input is required")
	}

	conv, err := s.store.GetConversation(ctx, req.ConversationID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	agent, err := s.store.GetAgent(ctx, req.AgentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}
	if conv.AgentID != "" && conv.AgentID != agent.ID {
		return nil, fmt.Errorf("conversation not found")
	}

	runAgent := *agent
	if req.MaxIterations != nil {
		runAgent.Config.MaxIterations = *req.MaxIterations
	}
	if req.TokenBudget != nil {
		runAgent.Config.TokenBudget = *req.TokenBudget
	}
	runAgent.Config = NormalizeConfig(runAgent.Config)

	var runResult *RunResult
	var run *Run
	if hasEnabledTools(&runAgent) {
		runResult, err = s.runner.RunWithTools(ctx, session, &runAgent, conv.ID, content, nil)
	} else {
		metadata, _ := chat.RelayRequestMetadataFromContext(ctx)
		run, createErr := s.store.CreateRun(ctx, &CreateRunRequest{
			OrganizationID: session.OrganizationID,
			ConversationID: conv.ID,
			AgentID:        runAgent.ID,
			UserID:         session.User.ID,
			RequestID:      metadata.RequestID,
			Mode:           ExecutionModeReact,
			Status:         RunStatusRunning,
		})
		if createErr != nil {
			return nil, createErr
		}
		if run == nil {
			return nil, fmt.Errorf("create agent run: no row created")
		}
		runResult, err = s.runner.Run(ctx, session, &runAgent, conv.ID, content)
		now := time.Now().UTC()
		if err != nil {
			if _, updateErr := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
				Status:         stringPointer(RunStatusFailed),
				IterationCount: intPointer(1),
				ToolCallCount:  intPointer(0),
				Error:          stringPointer(err.Error()),
				CompletedAt:    &now,
			}); updateErr == nil {
				recordAgentRunMetrics(RunStatusFailed, 1)
			}
			return nil, err
		}
		finalMessageID := ""
		if runResult != nil && runResult.Message != nil {
			finalMessageID = runResult.Message.ID
		}
		run, err = s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
			Status:            stringPointer(RunStatusCompleted),
			MemoryEnabled:     boolPointer(runResult != nil && runResult.UsedMemory),
			MemorySearched:    boolPointer(runResult != nil && runResult.MemorySearched),
			MemoryResultCount: intPointer(runResultMemoryResultCount(runResult)),
			IterationCount:    intPointer(1),
			ToolCallCount:     intPointer(0),
			FinalMessageID:    &finalMessageID,
			CompletedAt:       &now,
		})
		if err == nil {
			recordAgentRunMetrics(RunStatusCompleted, 1)
		}
	}
	if err != nil {
		return nil, err
	}

	messages, err := s.store.ListMessages(ctx, conv.ID, session.OrganizationID)
	if err != nil {
		return nil, err
	}

	if run == nil {
		runs, listErr := s.store.ListRuns(ctx, session.OrganizationID, conv.ID)
		if listErr == nil && len(runs) > 0 {
			run = runs[len(runs)-1]
		}
	}
	if run == nil && runResult != nil && runResult.Message != nil {
		run = &Run{
			ID:                runResult.Message.ID,
			OrganizationID:    session.OrganizationID,
			ConversationID:    conv.ID,
			AgentID:           runAgent.ID,
			UserID:            session.User.ID,
			Status:            RunStatusCompleted,
			MemoryEnabled:     runResult.UsedMemory,
			MemorySearched:    runResult.MemorySearched,
			MemoryResultCount: runResult.MemoryResultCount,
			FinalMessageID:    runResult.Message.ID,
			StartedAt:         runResult.Message.CreatedAt,
			CreatedAt:         runResult.Message.CreatedAt,
			UpdatedAt:         runResult.Message.CreatedAt,
		}
	}

	toolRuns := []*ToolRun(nil)
	planSteps := []*PlanStep(nil)
	if run != nil {
		toolRuns, err = s.store.ListToolRuns(ctx, session.OrganizationID, run.ID)
		if err != nil {
			return nil, err
		}
		planSteps, err = s.store.ListPlanSteps(ctx, session.OrganizationID, run.ID)
		if err != nil {
			return nil, err
		}
	}

	return &RunWithMessages{Run: run, ToolRuns: toolRuns, PlanSteps: planSteps, Messages: messages}, nil
}

func runResultMemoryResultCount(result *RunResult) int {
	if result == nil {
		return 0
	}
	return result.MemoryResultCount
}

func (s *Service) StartPlanningRun(ctx context.Context, session auth.Session, req StartRunRequest) (*RunWithMessages, error) {
	content := strings.TrimSpace(req.Input)
	if content == "" {
		return nil, fmt.Errorf("input is required")
	}

	conv, err := s.store.GetConversation(ctx, req.ConversationID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	agent, err := s.store.GetAgent(ctx, req.AgentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}
	if conv.AgentID != "" && conv.AgentID != agent.ID {
		return nil, fmt.Errorf("conversation not found")
	}

	runAgent := *agent
	if req.MaxIterations != nil {
		runAgent.Config.MaxIterations = *req.MaxIterations
	}
	if req.TokenBudget != nil {
		runAgent.Config.TokenBudget = *req.TokenBudget
	}
	runAgent.Config = NormalizeConfig(runAgent.Config)

	ctx = withSessionRelayMetadata(ctx, session)
	metadata, _ := chat.RelayRequestMetadataFromContext(ctx)

	if _, err := s.store.CreateMessage(ctx, conv.ID, session.OrganizationID, "user", content, nil, ""); err != nil {
		return nil, fmt.Errorf("save planning request: %w", err)
	}
	messages, err := s.store.ListMessages(ctx, conv.ID, session.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("get planning messages: %w", err)
	}
	chatMessages, evidence := s.runner.buildChatMessagesWithEvidence(ctx, session, &runAgent, messages, content)

	run, err := s.store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID:    session.OrganizationID,
		ConversationID:    conv.ID,
		AgentID:           runAgent.ID,
		UserID:            session.User.ID,
		RequestID:         metadata.RequestID,
		Mode:              ExecutionModePlanning,
		Status:            RunStatusRunning,
		MemoryEnabled:     evidence.enabled,
		MemorySearched:    evidence.searched,
		MemoryResultCount: evidence.resultCount,
	})
	if err != nil {
		return nil, fmt.Errorf("create planning run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("create planning run: no row created")
	}

	tokenBudget := normalizeTokenBudget(runAgent.Config.TokenBudget)
	estimatedTokens := estimateChatMessageTokens(chatMessages)
	if tokenBudget > 0 && estimatedTokens > tokenBudget {
		now := time.Now().UTC()
		message := fmt.Sprintf("token_budget_exceeded: estimated %d prompt tokens exceeds budget %d", estimatedTokens, tokenBudget)
		if _, updateErr := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
			Status:            stringPointer(RunStatusTokenBudgetExceeded),
			MemoryEnabled:     boolPointer(evidence.enabled),
			MemorySearched:    boolPointer(evidence.searched),
			MemoryResultCount: intPointer(evidence.resultCount),
			IterationCount:    intPointer(1),
			ToolCallCount:     intPointer(0),
			Error:             stringPointer(message),
			CompletedAt:       &now,
		}); updateErr != nil {
			return nil, updateErr
		}
		recordAgentRunMetrics(RunStatusTokenBudgetExceeded, 1)
		return nil, fmt.Errorf("%w: estimated %d prompt tokens exceeds budget %d", ErrTokenBudgetExceeded, estimatedTokens, tokenBudget)
	}

	reply, err := s.gateway.GenerateReply(ctx, chatMessages, planningConversationConfig(&runAgent))
	if err != nil {
		now := time.Now().UTC()
		if _, updateErr := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
			Status:            stringPointer(RunStatusFailed),
			MemoryEnabled:     boolPointer(evidence.enabled),
			MemorySearched:    boolPointer(evidence.searched),
			MemoryResultCount: intPointer(evidence.resultCount),
			IterationCount:    intPointer(1),
			Error:             stringPointer(err.Error()),
			CompletedAt:       &now,
		}); updateErr == nil {
			recordAgentRunMetrics(RunStatusFailed, 1)
		}
		return nil, fmt.Errorf("generate planning reply: %w", err)
	}

	assistantMsg, err := s.store.CreateMessage(ctx, conv.ID, session.OrganizationID, "assistant", reply, nil, "")
	if err != nil {
		now := time.Now().UTC()
		if _, updateErr := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
			Status:            stringPointer(RunStatusFailed),
			MemoryEnabled:     boolPointer(evidence.enabled),
			MemorySearched:    boolPointer(evidence.searched),
			MemoryResultCount: intPointer(evidence.resultCount),
			IterationCount:    intPointer(1),
			Error:             stringPointer(err.Error()),
			CompletedAt:       &now,
		}); updateErr == nil {
			recordAgentRunMetrics(RunStatusFailed, 1)
		}
		return nil, fmt.Errorf("save planning reply: %w", err)
	}

	planSteps, err := s.persistPlanningSteps(ctx, session.OrganizationID, run.ID, reply)
	if err != nil {
		now := time.Now().UTC()
		if _, updateErr := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
			Status:            stringPointer(RunStatusFailed),
			MemoryEnabled:     boolPointer(evidence.enabled),
			MemorySearched:    boolPointer(evidence.searched),
			MemoryResultCount: intPointer(evidence.resultCount),
			IterationCount:    intPointer(1),
			Error:             stringPointer(err.Error()),
			CompletedAt:       &now,
		}); updateErr == nil {
			recordAgentRunMetrics(RunStatusFailed, 1)
		}
		return nil, fmt.Errorf("persist planning steps: %w", err)
	}

	run, err = s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
		Status:            stringPointer(RunStatusPendingApproval),
		MemoryEnabled:     boolPointer(evidence.enabled),
		MemorySearched:    boolPointer(evidence.searched),
		MemoryResultCount: intPointer(evidence.resultCount),
		IterationCount:    intPointer(1),
		ToolCallCount:     intPointer(0),
		FinalMessageID:    stringPointer(assistantMsg.ID),
	})
	if err != nil {
		return nil, err
	}
	recordAgentRunMetrics(RunStatusPendingApproval, 1)

	messages, err = s.store.ListMessages(ctx, conv.ID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	return &RunWithMessages{Run: run, ToolRuns: nil, PlanSteps: planSteps, Messages: messages}, nil
}

var (
	numberedPlanStepPattern = regexp.MustCompile(`^\s*\d+[\.)]\s+(.+?)\s*$`)
	bulletedPlanStepPattern = regexp.MustCompile(`^\s*[-*+]\s+(.+?)\s*$`)
)

func (s *Service) persistPlanningSteps(ctx context.Context, organizationID, runID, reply string) ([]*PlanStep, error) {
	specs := parsePlanStepSpecs(reply)
	return s.createPlanStepsFromSpecs(ctx, organizationID, runID, 1, specs)
}

func (s *Service) createPlanStepsFromSpecs(ctx context.Context, organizationID, runID string, startIndex int, specs []parsedPlanStepSpec) ([]*PlanStep, error) {
	steps := make([]*PlanStep, 0, len(specs))
	for i, spec := range specs {
		step, err := s.store.CreatePlanStep(ctx, &CreatePlanStepRequest{
			OrganizationID: organizationID,
			RunID:          runID,
			Index:          startIndex + i,
			Title:          spec.Title,
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			ToolName:       spec.ToolName,
			Input:          spec.Input,
		})
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

type parsedPlanStepSpec struct {
	Title    string         `json:"title"`
	ToolName string         `json:"toolName"`
	Input    map[string]any `json:"input"`
}

func parsePlanStepSpecs(reply string) []parsedPlanStepSpec {
	var structured []parsedPlanStepSpec
	if err := json.Unmarshal([]byte(strings.TrimSpace(reply)), &structured); err == nil {
		steps := normalizeParsedPlanStepSpecs(structured)
		if len(steps) > 0 {
			return steps
		}
	}
	var wrapped struct {
		Steps []parsedPlanStepSpec `json:"steps"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(reply)), &wrapped); err == nil {
		steps := normalizeParsedPlanStepSpecs(wrapped.Steps)
		if len(steps) > 0 {
			return steps
		}
	}

	titles := parsePlanStepTitles(reply)
	steps := make([]parsedPlanStepSpec, 0, len(titles))
	for _, title := range titles {
		steps = append(steps, parsedPlanStepSpec{
			Title: title,
			Input: map[string]any{},
		})
	}
	return steps
}

func normalizeParsedPlanStepSpecs(structured []parsedPlanStepSpec) []parsedPlanStepSpec {
	steps := make([]parsedPlanStepSpec, 0, len(structured))
	for _, step := range structured {
		step.Title = strings.TrimSpace(step.Title)
		step.ToolName = strings.TrimSpace(step.ToolName)
		if step.Title == "" {
			continue
		}
		if step.Input == nil {
			step.Input = map[string]any{}
		}
		steps = append(steps, step)
	}
	return steps
}

func parsePlanStepTitles(reply string) []string {
	var titles []string
	for _, line := range strings.Split(reply, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		title := ""
		if match := numberedPlanStepPattern.FindStringSubmatch(line); len(match) == 2 {
			title = match[1]
		} else if match := bulletedPlanStepPattern.FindStringSubmatch(line); len(match) == 2 {
			title = match[1]
		}
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		titles = append(titles, title)
	}
	return titles
}

func recordAgentRunMetrics(status string, iterations int) {
	metrics.RecordAgentRun(status)
	metrics.ObserveAgentIterationCount(status, iterations)
}

func (s *Service) GetRunWithMessages(ctx context.Context, session auth.Session, runID string) (*RunWithMessages, error) {
	detail, err := s.GetRunDetail(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	messages, err := s.store.ListMessages(ctx, detail.Run.ConversationID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	return &RunWithMessages{Run: detail.Run, ToolRuns: detail.ToolRuns, PlanSteps: detail.PlanSteps, Messages: messages}, nil
}

// SendMessageStream 流式发送消息
func (s *Service) SendMessageStream(ctx context.Context, session auth.Session, conversationID string, content string, onChunk func(string) error, options ...SendMessageOptions) error {
	// 获取对话
	conv, err := s.store.GetConversation(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	// 获取 Agent
	agent, err := s.store.GetAgent(ctx, conv.AgentID, session.OrganizationID)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}

	runAgent := *agent
	mode := NormalizeExecutionMode(runAgent.Config.DefaultExecutionMode)
	if len(options) > 0 {
		option := options[0]
		overrideMode := strings.ToLower(strings.TrimSpace(option.Mode))
		if overrideMode != "" {
			if overrideMode != ExecutionModeReact && overrideMode != ExecutionModePlanning {
				return fmt.Errorf("mode must be react or planning")
			}
			mode = overrideMode
		}
		if option.MaxIterations != nil {
			runAgent.Config.MaxIterations = *option.MaxIterations
		}
		if option.TokenBudget != nil {
			runAgent.Config.TokenBudget = *option.TokenBudget
		}
	}

	if mode == ExecutionModePlanning {
		result, err := s.StartPlanningRun(ctx, session, StartRunRequest{
			AgentID:        runAgent.ID,
			ConversationID: conversationID,
			Input:          content,
			MaxIterations:  firstSendMessageIntPointer(options, func(option SendMessageOptions) *int { return option.MaxIterations }),
			TokenBudget:    firstSendMessageIntPointer(options, func(option SendMessageOptions) *int { return option.TokenBudget }),
		})
		if err != nil {
			return err
		}
		if onChunk == nil {
			return nil
		}
		reply := ""
		if message := lastAssistantMessage(result); message != nil {
			reply = message.Content
		}
		return streamContent(reply, onChunk)
	}

	if hasEnabledTools(&runAgent) {
		_, err = s.runner.RunWithTools(ctx, session, &runAgent, conversationID, content, onChunk)
		return err
	}

	_, err = s.runner.RunStream(ctx, session, &runAgent, conversationID, content, onChunk)
	return err
}

// ListMessages 列出消息
func (s *Service) ListMessages(ctx context.Context, session auth.Session, conversationID string) ([]*Message, error) {
	// 验证对话所有权
	conv, err := s.store.GetConversation(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.ListMessages(ctx, conversationID, session.OrganizationID)
}

func (s *Service) ListRuns(ctx context.Context, session auth.Session, conversationID string) ([]*Run, error) {
	conv, err := s.store.GetConversation(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}
	return s.store.ListRuns(ctx, session.OrganizationID, conversationID)
}

func (s *Service) GetRunDetail(ctx context.Context, session auth.Session, runID string) (*RunDetail, error) {
	run, err := s.getRunForSession(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	toolRuns, err := s.store.ListToolRuns(ctx, session.OrganizationID, run.ID)
	if err != nil {
		return nil, err
	}
	planSteps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, run.ID)
	if err != nil {
		return nil, err
	}
	return &RunDetail{Run: run, ToolRuns: toolRuns, PlanSteps: planSteps}, nil
}

func (s *Service) AdjustPlanSteps(ctx context.Context, session auth.Session, runID, reason string) (*RunWithMessages, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}

	run, err := s.getRunForSession(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	if NormalizeExecutionMode(run.Mode) != ExecutionModePlanning {
		return nil, fmt.Errorf("agent run is not in planning mode")
	}
	if !isPlanningRunAdjustable(run) {
		return nil, fmt.Errorf("planning run cannot be adjusted from status %s", run.Status)
	}

	agent, err := s.store.GetAgent(ctx, run.AgentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}

	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, run.ID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)

	completedPrefix := make([]*PlanStep, 0, len(steps))
	seenRemaining := false
	for _, step := range steps {
		if step == nil {
			continue
		}
		if isPlanStepDone(step) {
			if seenRemaining {
				return nil, fmt.Errorf("plan cannot be adjusted across completed later step")
			}
			completedPrefix = append(completedPrefix, step)
			continue
		}
		seenRemaining = true
		if !isPlanStepReplaceableForAdjustment(step) {
			return nil, fmt.Errorf("plan step %d cannot be adjusted while %s", step.Index, step.Status)
		}
	}
	if !seenRemaining {
		return nil, fmt.Errorf("no remaining plan steps to adjust")
	}

	ctx = withSessionRelayMetadata(ctx, session)
	prompt := buildAdjustedPlanPrompt(steps, completedPrefix, reason)
	reply, err := s.gateway.GenerateReply(ctx, []chat.Message{{
		Role:    "user",
		Content: prompt,
	}}, planningConversationConfig(agent))
	if err != nil {
		return nil, fmt.Errorf("generate adjusted plan: %w", err)
	}

	specs := parsePlanStepSpecs(reply)
	if len(specs) == 0 {
		return nil, fmt.Errorf("adjusted plan did not include any remaining steps")
	}
	assistantMsg, err := s.store.CreateMessage(ctx, run.ConversationID, session.OrganizationID, "assistant", reply, nil, "")
	if err != nil {
		return nil, fmt.Errorf("save adjusted planning reply: %w", err)
	}

	completedPrefixIDs := make(map[string]struct{}, len(completedPrefix))
	for _, step := range completedPrefix {
		completedPrefixIDs[step.ID] = struct{}{}
	}
	for _, step := range steps {
		if step == nil {
			continue
		}
		if _, ok := completedPrefixIDs[step.ID]; ok {
			continue
		}
		if _, err := s.store.DeletePlanStep(ctx, session.OrganizationID, step.ID); err != nil {
			return nil, err
		}
	}
	if _, err := s.createPlanStepsFromSpecs(ctx, session.OrganizationID, run.ID, len(completedPrefix)+1, specs); err != nil {
		return nil, err
	}
	if _, err := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
		Status:           stringPointer(RunStatusPendingApproval),
		Error:            stringPointer(""),
		FinalMessageID:   stringPointer(assistantMsg.ID),
		ClearCompletedAt: true,
	}); err != nil {
		return nil, err
	}

	return s.GetRunWithMessages(ctx, session, run.ID)
}

func (s *Service) ContinuePlanningRun(ctx context.Context, session auth.Session, runID string) (*RunWithMessages, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	run, err := s.getRunForSession(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	if NormalizeExecutionMode(run.Mode) != ExecutionModePlanning {
		return nil, fmt.Errorf("agent run is not in planning mode")
	}
	if !isPlanningRunContinuable(run) {
		return nil, fmt.Errorf("planning run cannot be continued from status %s", run.Status)
	}

	for {
		detail, err := s.GetRunWithMessages(ctx, session, run.ID)
		if err != nil {
			return nil, err
		}
		steps := append([]*PlanStep(nil), detail.PlanSteps...)
		sortPlanSteps(steps)

		executed := false
		for _, step := range steps {
			if step == nil || isPlanStepDone(step) {
				continue
			}
			if !isPlanStepExecutable(step) {
				return detail, nil
			}
			if _, err := s.ExecutePlanStep(ctx, session, step.ID); err != nil {
				refreshed, refreshErr := s.GetRunWithMessages(ctx, session, run.ID)
				if refreshErr != nil {
					return nil, err
				}
				if isPlanningExecutionStopAfterError(refreshed, step.ID) {
					return refreshed, nil
				}
				return nil, err
			}
			executed = true
			break
		}
		if !executed {
			return detail, nil
		}
	}
}

func (s *Service) ApprovePlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if step.Status != PlanStepStatusPending {
		return nil, fmt.Errorf("plan step is not pending")
	}
	if step.ApprovalStatus == ApprovalStatusRejected {
		return nil, fmt.Errorf("plan step approval was rejected")
	}
	return s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
		Status:         stringPointer(PlanStepStatusApproved),
		ApprovalStatus: stringPointer(ApprovalStatusApproved),
		Error:          stringPointer(""),
	})
}

type UpdatePlanStepDraftRequest struct {
	Title    *string
	ToolName *string
	Input    map[string]any
}

type CreatePlanStepDraftRequest struct {
	AfterPlanStepID *string
	Title           string
	ToolName        string
	Input           map[string]any
}

const (
	MovePlanStepDirectionUp   = "up"
	MovePlanStepDirectionDown = "down"
)

func (s *Service) CreatePlanStepDraft(ctx context.Context, session auth.Session, runID string, req CreatePlanStepDraftRequest) ([]*PlanStep, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	if _, err := s.getPlanningRunForSession(ctx, session, runID); err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("plan step title is required")
	}
	toolName := strings.TrimSpace(req.ToolName)
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}

	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, runID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)

	insertIndex := maxPlanStepIndex(steps) + 1
	afterPlanStepID := ""
	if req.AfterPlanStepID != nil {
		afterPlanStepID = strings.TrimSpace(*req.AfterPlanStepID)
	}
	if afterPlanStepID != "" {
		var anchor *PlanStep
		for _, step := range steps {
			if step != nil && step.ID == afterPlanStepID {
				anchor = step
				break
			}
		}
		if anchor == nil {
			return nil, fmt.Errorf("after plan step not found")
		}
		if !isPlanStepDraftAdjustable(anchor) {
			return nil, fmt.Errorf("plan step cannot be inserted after executed step")
		}
		insertIndex = anchor.Index + 1
	} else if len(steps) > 0 {
		last := steps[len(steps)-1]
		if !isPlanStepDraftAdjustable(last) {
			return nil, fmt.Errorf("plan step cannot be appended after executed step")
		}
	}

	shiftCandidates, originalIndices, err := planStepShiftCandidates(steps, insertIndex, func(index int) bool {
		return index >= insertIndex
	})
	if err != nil {
		return nil, fmt.Errorf("plan step cannot be inserted across executed step")
	}
	if err := s.movePlanStepsToBuffer(ctx, session, steps, shiftCandidates); err != nil {
		return nil, err
	}

	if _, err := s.store.CreatePlanStep(ctx, &CreatePlanStepRequest{
		OrganizationID: session.OrganizationID,
		RunID:          runID,
		Index:          insertIndex,
		Title:          title,
		Status:         PlanStepStatusPending,
		ApprovalStatus: ApprovalStatusPending,
		ToolName:       toolName,
		Input:          copyPlanStepInput(input),
	}); err != nil {
		return nil, err
	}

	for _, candidate := range shiftCandidates {
		nextIndex := originalIndices[candidate.ID] + 1
		if _, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, candidate.ID, planStepMoveUpdateRequest(nextIndex, candidate)); err != nil {
			return nil, err
		}
	}

	return s.listSortedPlanSteps(ctx, session.OrganizationID, runID)
}

func (s *Service) UpdatePlanStepDraft(ctx context.Context, session auth.Session, planStepID string, req UpdatePlanStepDraftRequest) (*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if step.Status != PlanStepStatusPending && step.Status != PlanStepStatusApproved {
		return nil, fmt.Errorf("plan step cannot be adjusted after execution starts")
	}

	updateReq := UpdatePlanStepRequest{
		ResultContent:    stringPointer(""),
		Error:            stringPointer(""),
		ClearCompletedAt: true,
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, fmt.Errorf("plan step title is required")
		}
		updateReq.Title = &title
	}
	if req.ToolName != nil {
		toolName := strings.TrimSpace(*req.ToolName)
		updateReq.ToolName = &toolName
	}
	if req.Input != nil {
		updateReq.Input = copyPlanStepInput(req.Input)
		updateReq.ReplaceInput = true
	}
	if step.ApprovalStatus == ApprovalStatusApproved {
		updateReq.Status = stringPointer(PlanStepStatusPending)
		updateReq.ApprovalStatus = stringPointer(ApprovalStatusPending)
	}
	return s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, updateReq)
}

func (s *Service) MovePlanStep(ctx context.Context, session auth.Session, planStepID, direction string) ([]*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if !isPlanStepDraftAdjustable(step) {
		return nil, fmt.Errorf("plan step cannot be adjusted after execution starts")
	}

	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction != MovePlanStepDirectionUp && direction != MovePlanStepDirectionDown {
		return nil, fmt.Errorf("plan step move direction must be up or down")
	}

	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, step.RunID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)

	currentPosition := -1
	for i, candidate := range steps {
		if candidate.ID == step.ID {
			currentPosition = i
			break
		}
	}
	if currentPosition == -1 {
		return nil, fmt.Errorf("plan step not found")
	}

	targetPosition := currentPosition
	if direction == MovePlanStepDirectionUp {
		targetPosition--
	} else {
		targetPosition++
	}
	if targetPosition < 0 || targetPosition >= len(steps) {
		return nil, fmt.Errorf("plan step cannot move %s", direction)
	}

	target := steps[targetPosition]
	if !isPlanStepDraftAdjustable(target) {
		return nil, fmt.Errorf("plan step cannot move across executed step")
	}
	stepIndex := step.Index
	targetIndex := target.Index
	bufferIndex := maxPlanStepIndex(steps) + 1

	if _, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, step.ID, planStepMoveUpdateRequest(bufferIndex, step)); err != nil {
		return nil, err
	}
	if _, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, target.ID, planStepMoveUpdateRequest(stepIndex, target)); err != nil {
		return nil, err
	}
	if _, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, step.ID, planStepMoveUpdateRequest(targetIndex, step)); err != nil {
		return nil, err
	}

	steps, err = s.store.ListPlanSteps(ctx, session.OrganizationID, step.RunID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)
	return steps, nil
}

func (s *Service) DeletePlanStepDraft(ctx context.Context, session auth.Session, planStepID string) ([]*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if !isPlanStepDraftAdjustable(step) {
		return nil, fmt.Errorf("plan step cannot be deleted after execution starts")
	}

	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, step.RunID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)
	shiftCandidates, originalIndices, err := planStepShiftCandidates(steps, step.Index+1, func(index int) bool {
		return index > step.Index
	})
	if err != nil {
		return nil, fmt.Errorf("plan step cannot be deleted before executed step")
	}

	if _, err := s.store.DeletePlanStep(ctx, session.OrganizationID, step.ID); err != nil {
		return nil, err
	}
	if err := s.movePlanStepsToBuffer(ctx, session, steps, shiftCandidates); err != nil {
		return nil, err
	}
	for _, candidate := range shiftCandidates {
		nextIndex := originalIndices[candidate.ID] - 1
		if _, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, candidate.ID, planStepMoveUpdateRequest(nextIndex, candidate)); err != nil {
			return nil, err
		}
	}

	return s.listSortedPlanSteps(ctx, session.OrganizationID, step.RunID)
}

func (s *Service) ExecutePlanStep(ctx context.Context, session auth.Session, planStepID string) (*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if step.Status != PlanStepStatusApproved && !(step.Status == PlanStepStatusPending && step.ApprovalStatus == ApprovalStatusNotRequired) {
		return nil, fmt.Errorf("plan step is not approved for execution")
	}
	if err := s.ensurePriorPlanStepsDone(ctx, session, step); err != nil {
		return nil, err
	}

	startedAt := time.Now().UTC()
	running, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
		Status:           stringPointer(PlanStepStatusRunning),
		Error:            stringPointer(""),
		StartedAt:        &startedAt,
		ClearCompletedAt: true,
	})
	if err != nil {
		return nil, err
	}

	var toolRun *ToolRun
	if strings.TrimSpace(running.ToolName) != "" {
		run, err := s.getRunForSession(ctx, session, running.RunID)
		if err != nil {
			return nil, err
		}
		toolRun, err = s.store.CreateToolRun(ctx, &CreateToolRunRequest{
			OrganizationID: running.OrganizationID,
			RunID:          running.RunID,
			ConversationID: run.ConversationID,
			AgentID:        run.AgentID,
			ToolCallID:     "plan_step_" + running.ID,
			ToolName:       running.ToolName,
			ToolType:       "builtin",
			RiskLevel:      inferToolRiskLevel(running.ToolName),
			Arguments:      running.Input,
			Status:         ToolRunStatusRunning,
			ApprovalStatus: ApprovalStatusNotRequired,
			AttemptCount:   1,
			StartedAt:      &startedAt,
		})
		if err != nil {
			failedAt := time.Now().UTC()
			failed, updateErr := s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
				Status:      stringPointer(PlanStepStatusFailed),
				Error:       stringPointer(err.Error()),
				CompletedAt: &failedAt,
			})
			if updateErr != nil {
				return nil, updateErr
			}
			return failed, err
		}
	}

	ctx = withSessionRelayMetadata(ctx, session)
	result, execErr := s.planStepExecutor.ExecutePlanStep(ctx, running)
	completedAt := time.Now().UTC()
	if execErr != nil {
		execErrMessage := execErr.Error()
		if errors.Is(execErr, ErrTokenBudgetExceeded) {
			execErrMessage = "token_budget_exceeded: " + execErrMessage
		}
		if toolRun != nil {
			_, _ = s.store.UpdateToolRun(ctx, session.OrganizationID, toolRun.ID, UpdateToolRunRequest{
				Status:       stringPointer(ToolRunStatusFailed),
				Error:        stringPointer(execErrMessage),
				AttemptCount: intPointer(1),
				CompletedAt:  &completedAt,
			})
		}
		failed, updateErr := s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
			Status:      stringPointer(PlanStepStatusFailed),
			Error:       stringPointer(execErrMessage),
			CompletedAt: &completedAt,
		})
		if updateErr != nil {
			return nil, updateErr
		}
		if errors.Is(execErr, ErrTokenBudgetExceeded) {
			_, updateRunErr := s.store.UpdateRun(ctx, session.OrganizationID, running.RunID, UpdateRunRequest{
				Status:         stringPointer(RunStatusTokenBudgetExceeded),
				Error:          stringPointer(execErrMessage),
				IterationCount: intPointer(1),
				CompletedAt:    &completedAt,
			})
			if updateRunErr != nil {
				return nil, updateRunErr
			}
		}
		return failed, execErr
	}

	resultContent := ""
	if result != nil {
		resultContent = result.ResultContent
	}
	if toolRun != nil {
		if _, err := s.store.UpdateToolRun(ctx, session.OrganizationID, toolRun.ID, UpdateToolRunRequest{
			Status:        stringPointer(ToolRunStatusCompleted),
			ResultContent: stringPointer(resultContent),
			Error:         stringPointer(""),
			AttemptCount:  intPointer(1),
			CompletedAt:   &completedAt,
		}); err != nil {
			return nil, err
		}
		metrics.RecordAgentToolCall(running.ToolName, string(ToolRunStatusCompleted))
	}
	completed, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
		Status:        stringPointer(PlanStepStatusCompleted),
		ResultContent: stringPointer(resultContent),
		Error:         stringPointer(""),
		CompletedAt:   &completedAt,
	})
	if err != nil {
		return nil, err
	}
	if err := s.completeRunWhenAllPlanStepsDone(ctx, session, completed.RunID); err != nil {
		return nil, err
	}
	return completed, nil
}

func (s *Service) SkipPlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if !canSkipPlanStep(step) {
		return nil, fmt.Errorf("plan step cannot be skipped after execution starts")
	}
	if err := s.ensurePriorPlanStepsDone(ctx, session, step); err != nil {
		return nil, err
	}

	completedAt := time.Now().UTC()
	skipped, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
		Status:        stringPointer(PlanStepStatusSkipped),
		ResultContent: stringPointer(""),
		Error:         stringPointer(strings.TrimSpace(reason)),
		CompletedAt:   &completedAt,
	})
	if err != nil {
		return nil, err
	}
	if err := s.completeRunWhenAllPlanStepsDone(ctx, session, skipped.RunID); err != nil {
		return nil, err
	}
	return skipped, nil
}

func (s *Service) RetryPlanStep(ctx context.Context, session auth.Session, planStepID string) (*PlanStep, error) {
	step, err := s.getPlanStepForSession(ctx, session, planStepID)
	if err != nil {
		return nil, err
	}
	if step.Status != PlanStepStatusFailed {
		return nil, fmt.Errorf("plan step is not failed")
	}
	retryStatus := PlanStepStatusPending
	retryApprovalStatus := ApprovalStatusPending
	if step.ApprovalStatus == ApprovalStatusApproved {
		retryStatus = PlanStepStatusApproved
		retryApprovalStatus = ApprovalStatusApproved
	} else if step.ApprovalStatus == ApprovalStatusNotRequired {
		retryApprovalStatus = ApprovalStatusNotRequired
	}
	if retryStatus == PlanStepStatusApproved || step.ApprovalStatus == ApprovalStatusNotRequired {
		if err := s.ensurePriorPlanStepsDone(ctx, session, step); err != nil {
			return nil, err
		}
	}
	if _, err := s.store.UpdateRun(ctx, session.OrganizationID, step.RunID, UpdateRunRequest{
		Status:           stringPointer(RunStatusPendingApproval),
		Error:            stringPointer(""),
		ClearCompletedAt: true,
	}); err != nil {
		return nil, err
	}

	reopened, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, planStepID, UpdatePlanStepRequest{
		Status:           stringPointer(retryStatus),
		ApprovalStatus:   stringPointer(retryApprovalStatus),
		ResultContent:    stringPointer(""),
		Error:            stringPointer(""),
		ClearCompletedAt: true,
	})
	if err != nil {
		return nil, err
	}
	if retryStatus != PlanStepStatusApproved && step.ApprovalStatus != ApprovalStatusNotRequired {
		return reopened, nil
	}
	return s.ExecutePlanStep(ctx, session, planStepID)
}

func canSkipPlanStep(step *PlanStep) bool {
	if step == nil {
		return false
	}
	return step.Status == PlanStepStatusPending || step.Status == PlanStepStatusApproved || step.Status == PlanStepStatusFailed
}

func isPlanStepDone(step *PlanStep) bool {
	if step == nil {
		return false
	}
	return step.Status == PlanStepStatusCompleted || step.Status == PlanStepStatusSkipped
}

func isPlanStepExecutable(step *PlanStep) bool {
	if step == nil {
		return false
	}
	return step.Status == PlanStepStatusApproved || (step.Status == PlanStepStatusPending && step.ApprovalStatus == ApprovalStatusNotRequired)
}

func isPlanningRunContinuable(run *Run) bool {
	if run == nil {
		return false
	}
	return run.Status == RunStatusPendingApproval
}

func isPlanningRunAdjustable(run *Run) bool {
	if run == nil {
		return false
	}
	return run.Status == RunStatusPendingApproval
}

func isPlanningExecutionStopAfterError(detail *RunWithMessages, planStepID string) bool {
	if detail == nil {
		return false
	}
	if detail.Run != nil && (detail.Run.Status == RunStatusFailed || detail.Run.Status == RunStatusTokenBudgetExceeded) {
		return true
	}
	for _, step := range detail.PlanSteps {
		if step != nil && step.ID == planStepID && step.Status == PlanStepStatusFailed {
			return true
		}
	}
	return false
}

func isPlanStepReplaceableForAdjustment(step *PlanStep) bool {
	if step == nil {
		return false
	}
	return step.Status == PlanStepStatusPending || step.Status == PlanStepStatusApproved || step.Status == PlanStepStatusFailed
}

type planStepAdjustmentEvidence struct {
	Index          int            `json:"index"`
	Title          string         `json:"title"`
	Status         string         `json:"status"`
	ApprovalStatus string         `json:"approvalStatus"`
	ToolName       string         `json:"toolName,omitempty"`
	Input          map[string]any `json:"input,omitempty"`
	ResultContent  string         `json:"resultContent,omitempty"`
	Error          string         `json:"error,omitempty"`
}

func buildAdjustedPlanPrompt(allSteps, completedSteps []*PlanStep, reason string) string {
	original := planStepsAdjustmentEvidence(allSteps)
	completed := planStepsAdjustmentEvidence(completedSteps)
	return fmt.Sprintf(strings.Join([]string{
		"The original plan has been partially executed and needs adjustment.",
		"Reason: %s",
		"",
		"Original plan: %s",
		"",
		"Completed or skipped steps: %s",
		"",
		"Produce a revised plan for the remaining work only.",
		"Return only a JSON array of remaining steps. Each item must include title, optional toolName, and optional input.",
	}, "\n"), reason, marshalPlanAdjustmentEvidence(original), marshalPlanAdjustmentEvidence(completed))
}

func planStepsAdjustmentEvidence(steps []*PlanStep) []planStepAdjustmentEvidence {
	evidence := make([]planStepAdjustmentEvidence, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			continue
		}
		evidence = append(evidence, planStepAdjustmentEvidence{
			Index:          step.Index,
			Title:          step.Title,
			Status:         step.Status,
			ApprovalStatus: step.ApprovalStatus,
			ToolName:       step.ToolName,
			Input:          step.Input,
			ResultContent:  step.ResultContent,
			Error:          step.Error,
		})
	}
	return evidence
}

func marshalPlanAdjustmentEvidence(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func (s *Service) ensurePriorPlanStepsDone(ctx context.Context, session auth.Session, step *PlanStep) error {
	if step == nil {
		return fmt.Errorf("plan step not found")
	}
	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, step.RunID)
	if err != nil {
		return err
	}
	sortPlanSteps(steps)
	for _, prior := range steps {
		if prior == nil || prior.ID == step.ID || prior.Index >= step.Index {
			continue
		}
		if prior.Status != PlanStepStatusCompleted && prior.Status != PlanStepStatusSkipped {
			return fmt.Errorf("prior plan step %d must be completed or skipped before executing step %d", prior.Index, step.Index)
		}
	}
	return nil
}

func copyPlanStepInput(input map[string]any) map[string]any {
	copied := make(map[string]any, len(input))
	for key, value := range input {
		copied[key] = value
	}
	return copied
}

func isPlanStepDraftAdjustable(step *PlanStep) bool {
	if step == nil {
		return false
	}
	return step.Status == PlanStepStatusPending || step.Status == PlanStepStatusApproved
}

func planStepShiftCandidates(steps []*PlanStep, fromIndex int, match func(int) bool) ([]*PlanStep, map[string]int, error) {
	candidates := make([]*PlanStep, 0)
	originalIndices := make(map[string]int)
	for _, step := range steps {
		if step == nil || !match(step.Index) {
			continue
		}
		if !isPlanStepDraftAdjustable(step) {
			return nil, nil, fmt.Errorf("plan step %d cannot be shifted after execution starts", step.Index)
		}
		if step.Index >= fromIndex {
			candidates = append(candidates, step)
			originalIndices[step.ID] = step.Index
		}
	}
	return candidates, originalIndices, nil
}

func (s *Service) movePlanStepsToBuffer(ctx context.Context, session auth.Session, steps, candidates []*PlanStep) error {
	bufferIndex := maxPlanStepIndex(steps) + 1
	for offset, candidate := range candidates {
		if _, err := s.store.UpdatePlanStep(ctx, session.OrganizationID, candidate.ID, planStepMoveUpdateRequest(bufferIndex+offset, candidate)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) listSortedPlanSteps(ctx context.Context, organizationID, runID string) ([]*PlanStep, error) {
	steps, err := s.store.ListPlanSteps(ctx, organizationID, runID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)
	return steps, nil
}

func planStepMoveUpdateRequest(index int, step *PlanStep) UpdatePlanStepRequest {
	req := UpdatePlanStepRequest{
		Index:            intPointer(index),
		ResultContent:    stringPointer(""),
		Error:            stringPointer(""),
		ClearCompletedAt: true,
	}
	if step != nil && step.ApprovalStatus == ApprovalStatusApproved {
		req.Status = stringPointer(PlanStepStatusPending)
		req.ApprovalStatus = stringPointer(ApprovalStatusPending)
	}
	return req
}

func sortPlanSteps(steps []*PlanStep) {
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].Index == steps[j].Index {
			return steps[i].CreatedAt.Before(steps[j].CreatedAt)
		}
		return steps[i].Index < steps[j].Index
	})
}

func maxPlanStepIndex(steps []*PlanStep) int {
	maxIndex := 0
	for _, step := range steps {
		if step != nil && step.Index > maxIndex {
			maxIndex = step.Index
		}
	}
	return maxIndex
}

func (s *Service) completeRunWhenAllPlanStepsDone(ctx context.Context, session auth.Session, runID string) error {
	run, err := s.getRunForSession(ctx, session, runID)
	if err != nil {
		return err
	}
	if run.Status == RunStatusCompleted {
		return nil
	}
	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, runID)
	if err != nil {
		return err
	}
	if len(steps) == 0 {
		return nil
	}
	for _, step := range steps {
		if step.Status != PlanStepStatusCompleted && step.Status != PlanStepStatusSkipped {
			return nil
		}
	}
	now := time.Now().UTC()
	_, err = s.store.UpdateRun(ctx, session.OrganizationID, runID, UpdateRunRequest{
		Status:      stringPointer(RunStatusCompleted),
		CompletedAt: &now,
	})
	if err == nil {
		recordAgentRunMetrics(RunStatusCompleted, run.IterationCount)
	}
	return err
}

func (s *Service) ApproveToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*ToolRun, error) {
	toolRun, err := s.getToolRunForSession(ctx, session, toolRunID)
	if err != nil {
		return nil, err
	}
	if toolRun.ApprovalStatus != ApprovalStatusPending || toolRun.Status != ToolRunStatusPendingApproval {
		return nil, fmt.Errorf("tool run is not pending approval")
	}
	now := time.Now().UTC()
	toolRun, err = s.store.UpdateToolRun(ctx, session.OrganizationID, toolRunID, UpdateToolRunRequest{
		Status:                 stringPointer(ToolRunStatusRunning),
		ApprovalStatus:         stringPointer(ApprovalStatusApproved),
		ApprovedByUserID:       stringPointer(session.User.ID),
		ApprovalDecisionReason: stringPointer(reason),
		AttemptCount:           intPointer(toolRun.AttemptCount + 1),
		StartedAt:              &now,
		ClearCompletedAt:       true,
	})
	if err != nil {
		return nil, err
	}
	return s.executePersistedToolRun(ctx, session, toolRun)
}

func (s *Service) RejectToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*ToolRun, error) {
	toolRun, err := s.getToolRunForSession(ctx, session, toolRunID)
	if err != nil {
		return nil, err
	}
	if toolRun.ApprovalStatus != ApprovalStatusPending || toolRun.Status != ToolRunStatusPendingApproval {
		return nil, fmt.Errorf("tool run is not pending approval")
	}
	run, err := s.getRunForSession(ctx, session, toolRun.RunID)
	if err != nil {
		return nil, err
	}
	completedAt := time.Now().UTC()
	updated, err := s.store.UpdateToolRun(ctx, session.OrganizationID, toolRunID, UpdateToolRunRequest{
		Status:                 stringPointer(ToolRunStatusRejected),
		ApprovalStatus:         stringPointer(ApprovalStatusRejected),
		ApprovedByUserID:       stringPointer(session.User.ID),
		ApprovalDecisionReason: stringPointer(reason),
		Error:                  stringPointer("tool run rejected: " + reason),
		CompletedAt:            &completedAt,
	})
	if err != nil {
		return nil, err
	}
	metrics.RecordAgentToolCall(toolRun.ToolName, string(ToolRunStatusRejected))
	if _, updateErr := s.store.UpdateRun(ctx, session.OrganizationID, toolRun.RunID, UpdateRunRequest{
		Status:      stringPointer(RunStatusFailed),
		Error:       stringPointer("tool run rejected: " + reason),
		CompletedAt: &completedAt,
	}); updateErr == nil {
		recordAgentRunMetrics(RunStatusFailed, run.IterationCount)
	}
	return updated, nil
}

func (s *Service) RetryToolRun(ctx context.Context, session auth.Session, toolRunID string) (*ToolRun, error) {
	toolRun, err := s.getToolRunForSession(ctx, session, toolRunID)
	if err != nil {
		return nil, err
	}
	if toolRun.Status != ToolRunStatusFailed {
		return nil, fmt.Errorf("tool run is not failed")
	}
	if toolRun.ApprovalStatus == ApprovalStatusPending {
		if _, err := s.store.UpdateRun(ctx, session.OrganizationID, toolRun.RunID, UpdateRunRequest{
			Status:           stringPointer(RunStatusPendingApproval),
			Error:            stringPointer(""),
			ClearCompletedAt: true,
		}); err != nil {
			return nil, err
		}
		empty := ""
		return s.store.UpdateToolRun(ctx, session.OrganizationID, toolRunID, UpdateToolRunRequest{
			Status:           stringPointer(ToolRunStatusPendingApproval),
			ApprovalStatus:   stringPointer(ApprovalStatusPending),
			ResultContent:    &empty,
			Error:            &empty,
			ClearCompletedAt: true,
		})
	}
	now := time.Now().UTC()
	empty := ""
	toolRun, err = s.store.UpdateToolRun(ctx, session.OrganizationID, toolRunID, UpdateToolRunRequest{
		Status:           stringPointer(ToolRunStatusRunning),
		ApprovalStatus:   stringPointer(toolRun.ApprovalStatus),
		AttemptCount:     intPointer(toolRun.AttemptCount + 1),
		ResultContent:    &empty,
		Error:            &empty,
		StartedAt:        &now,
		ClearCompletedAt: true,
	})
	if err != nil {
		return nil, err
	}
	return s.executePersistedToolRun(ctx, session, toolRun)
}

func (s *Service) ContinueRunWithTokenBudget(ctx context.Context, session auth.Session, runID string, tokenBudget int) (*RunResult, error) {
	run, err := s.getRunForSession(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	if run.Status != RunStatusTokenBudgetExceeded {
		return nil, fmt.Errorf("agent run is not token budget exceeded")
	}
	if tokenBudget < 1_000 || tokenBudget > maxTokenBudget {
		return nil, fmt.Errorf("tokenBudget must be between 1000 and 1000000")
	}
	normalizedBudget := normalizeTokenBudget(tokenBudget)
	agent, err := s.store.GetAgent(ctx, run.AgentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}

	if NormalizeExecutionMode(run.Mode) == ExecutionModePlanning {
		return s.continuePlanningRunWithTokenBudget(ctx, session, run, normalizedBudget)
	}

	if _, err := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
		Status:           stringPointer(RunStatusRunning),
		Error:            stringPointer(""),
		ClearCompletedAt: true,
	}); err != nil {
		return nil, err
	}
	run.Status = RunStatusRunning
	run.Error = ""
	run.CompletedAt = nil

	result, err := s.runner.ResumeAfterApprovedToolWithTokenBudget(ctx, session, agent, run, &normalizedBudget)
	if errors.Is(err, ErrToolApprovalRequired) {
		if result != nil {
			return result, nil
		}
		return &RunResult{}, nil
	}
	return result, err
}

func (s *Service) continuePlanningRunWithTokenBudget(ctx context.Context, session auth.Session, run *Run, tokenBudget int) (*RunResult, error) {
	steps, err := s.store.ListPlanSteps(ctx, session.OrganizationID, run.ID)
	if err != nil {
		return nil, err
	}
	sortPlanSteps(steps)
	var target *PlanStep
	for _, step := range steps {
		if step.Status == PlanStepStatusFailed && strings.Contains(step.Error, "token_budget_exceeded") {
			target = step
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("token-budget-exceeded planning run has no failed token-budget plan step")
	}

	if _, err := s.RetryPlanStep(withPlanStepTokenBudgetOverride(ctx, tokenBudget), session, target.ID); err != nil {
		return nil, err
	}
	return &RunResult{}, nil
}

func (s *Service) executePersistedToolRun(ctx context.Context, session auth.Session, toolRun *ToolRun) (*ToolRun, error) {
	if toolRun == nil {
		return nil, fmt.Errorf("tool run not found")
	}
	run, err := s.getRunForSession(ctx, session, toolRun.RunID)
	if err != nil {
		return nil, err
	}
	agent, err := s.store.GetAgent(ctx, toolRun.AgentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}

	toolCall := &ToolCall{
		ID:        toolRun.ToolCallID,
		Name:      toolRun.ToolName,
		Arguments: toolRun.Arguments,
	}
	if toolCall.Arguments == nil {
		toolCall.Arguments = map[string]any{}
	}

	execResult, err := s.runner.ExecuteTool(ctx, agent, toolCall)
	if err != nil {
		_ = s.runner.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolRun.ToolName, err.Error(), toolRun.AttemptCount)
		_ = s.runner.failRun(ctx, session.OrganizationID, run.ID, fmt.Sprintf("execute tool %s: %s", toolRun.ToolName, err.Error()), run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("execute tool %s: %w", toolRun.ToolName, err)
	}
	if execResult == nil {
		message := fmt.Sprintf("tool %s returned no result", toolRun.ToolName)
		_ = s.runner.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolRun.ToolName, message, toolRun.AttemptCount)
		_ = s.runner.failRun(ctx, session.OrganizationID, run.ID, message, run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("%s", message)
	}
	if execResult.IsError {
		_ = s.runner.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolRun.ToolName, execResult.Content, toolRun.AttemptCount)
		_ = s.runner.failRun(ctx, session.OrganizationID, run.ID, fmt.Sprintf("tool %s failed: %s", toolRun.ToolName, execResult.Content), run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("tool %s failed: %s", toolRun.ToolName, execResult.Content)
	}

	if _, err := s.store.CreateMessage(ctx, toolRun.ConversationID, session.OrganizationID, "tool", execResult.Content, nil, toolRun.ToolCallID); err != nil {
		_ = s.runner.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolRun.ToolName, err.Error(), toolRun.AttemptCount)
		_ = s.runner.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("save tool message: %w", err)
	}

	completedAt := time.Now().UTC()
	empty := ""
	updated, err := s.store.UpdateToolRun(ctx, session.OrganizationID, toolRun.ID, UpdateToolRunRequest{
		Status:        stringPointer(ToolRunStatusCompleted),
		ResultContent: stringPointer(execResult.Content),
		Error:         &empty,
		AttemptCount:  intPointer(toolRun.AttemptCount),
		CompletedAt:   &completedAt,
	})
	if err != nil {
		_ = s.runner.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("complete tool run %s: %w", toolRun.ToolName, err)
	}
	metrics.RecordAgentToolCall(toolRun.ToolName, string(ToolRunStatusCompleted))

	if _, err := s.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
		Status:           stringPointer(RunStatusRunning),
		Error:            &empty,
		ClearCompletedAt: true,
	}); err != nil {
		return nil, fmt.Errorf("resume agent run after tool %s: %w", toolRun.ToolName, err)
	}
	if _, err := s.runner.ResumeAfterApprovedTool(ctx, session, agent, run); err != nil {
		if errors.Is(err, ErrToolApprovalRequired) {
			return updated, nil
		}
		return nil, err
	}
	return updated, nil
}

func (s *Service) getToolRunForSession(ctx context.Context, session auth.Session, toolRunID string) (*ToolRun, error) {
	toolRun, err := s.store.GetToolRun(ctx, session.OrganizationID, toolRunID)
	if err != nil {
		return nil, err
	}
	if toolRun == nil {
		return nil, fmt.Errorf("tool run not found")
	}
	run, err := s.getRunForSession(ctx, session, toolRun.RunID)
	if err != nil {
		return nil, err
	}
	if run.ID != toolRun.RunID {
		return nil, fmt.Errorf("tool run not found")
	}
	return toolRun, nil
}

func (s *Service) getPlanStepForSession(ctx context.Context, session auth.Session, planStepID string) (*PlanStep, error) {
	step, err := s.store.GetPlanStep(ctx, session.OrganizationID, planStepID)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, fmt.Errorf("plan step not found")
	}
	run, err := s.getRunForSession(ctx, session, step.RunID)
	if err != nil {
		return nil, err
	}
	if run.ID != step.RunID {
		return nil, fmt.Errorf("plan step not found")
	}
	if NormalizeExecutionMode(run.Mode) != ExecutionModePlanning {
		return nil, fmt.Errorf("agent run is not in planning mode")
	}
	return step, nil
}

func (s *Service) getPlanningRunForSession(ctx context.Context, session auth.Session, runID string) (*Run, error) {
	run, err := s.getRunForSession(ctx, session, runID)
	if err != nil {
		return nil, err
	}
	if NormalizeExecutionMode(run.Mode) != ExecutionModePlanning {
		return nil, fmt.Errorf("agent run is not in planning mode")
	}
	return run, nil
}

func (s *Service) getRunForSession(ctx context.Context, session auth.Session, runID string) (*Run, error) {
	run, err := s.store.GetRun(ctx, session.OrganizationID, runID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}
	if run.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}
	return run, nil
}

func hasEnabledTools(agent *Agent) bool {
	if agent == nil {
		return false
	}
	for _, tool := range agent.Tools {
		if tool.Enabled {
			return true
		}
	}
	return false
}

func planningConversationConfig(agent *Agent) chat.ConversationConfig {
	config := chat.ConversationConfig{}
	if agent != nil {
		config.ModelID = agent.Model
		config.SystemPromptOverride = strings.TrimSpace(strings.Join([]string{
			agent.SystemPrompt,
			"You are in planning mode. Analyze the user's task and produce a concise, ordered execution plan. Do not execute tools or claim that work has been completed.",
		}, "\n\n"))
		config.Temperature = agent.Config.Temperature
		config.MaxOutputTokens = agent.Config.MaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = 1.0
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}
	return config
}

func planStepExecutionConversationConfig(run *Run, agent *Agent) chat.ConversationConfig {
	config := chat.ConversationConfig{}
	if run != nil {
		config.ConversationID = run.ConversationID
	}
	if agent != nil {
		config.ModelID = agent.Model
		config.KnowledgeBaseIDs = append([]string(nil), agent.Config.KnowledgeBaseIDs...)
		config.SystemPromptOverride = strings.TrimSpace(strings.Join([]string{
			agent.SystemPrompt,
			"Execute exactly one approved plan step. Use the provided conversation, current step, and prior completed step results as context. Do not execute other plan steps or claim unrelated work. Return the concrete result for this step only.",
		}, "\n\n"))
		config.Temperature = agent.Config.Temperature
		config.MaxOutputTokens = agent.Config.MaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = 1.0
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}
	return config
}

func estimateChatMessageTokens(messages []chat.Message) int {
	total := 0
	for _, message := range messages {
		total += estimateTextTokens(message.Role)
		total += estimateTextTokens(message.Content)
	}
	return total
}

func buildPlanStepExecutionMessages(run *Run, agent *Agent, currentStep *PlanStep, messages []*Message, steps []*PlanStep) []chat.Message {
	var builder strings.Builder
	builder.WriteString("Execute exactly one approved plan step.\n\n")
	if agent != nil && strings.TrimSpace(agent.SystemPrompt) != "" {
		builder.WriteString("Agent system prompt:\n")
		builder.WriteString(strings.TrimSpace(agent.SystemPrompt))
		builder.WriteString("\n\n")
	}
	if run != nil {
		builder.WriteString("Run context:\n")
		builder.WriteString("- Run ID: ")
		builder.WriteString(run.ID)
		builder.WriteString("\n")
		builder.WriteString("- Conversation ID: ")
		builder.WriteString(run.ConversationID)
		builder.WriteString("\n\n")
	}
	if len(messages) > 0 {
		builder.WriteString("Conversation so far:\n")
		for _, message := range messages {
			if message == nil || strings.TrimSpace(message.Content) == "" {
				continue
			}
			role := strings.TrimSpace(message.Role)
			if role == "" {
				role = "message"
			}
			builder.WriteString("- ")
			builder.WriteString(role)
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(message.Content))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("Current plan step:\n")
	if currentStep != nil {
		builder.WriteString("- Index: ")
		builder.WriteString(fmt.Sprintf("%d", currentStep.Index))
		builder.WriteString("\n")
		builder.WriteString("- Title: ")
		builder.WriteString(strings.TrimSpace(currentStep.Title))
		builder.WriteString("\n")
		if len(currentStep.Input) > 0 {
			builder.WriteString("- Input: ")
			builder.WriteString(formatPlanStepInput(currentStep.Input))
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n")
	priorResults := planStepPriorResults(currentStep, steps)
	if len(priorResults) > 0 {
		builder.WriteString("Prior completed plan steps:\n")
		for _, result := range priorResults {
			builder.WriteString(result)
			builder.WriteString("\n")
		}
	} else {
		builder.WriteString("Prior completed plan steps:\n- None\n")
	}
	builder.WriteString("\nReturn only the execution result for the current plan step.")

	return []chat.Message{{
		Role:    "user",
		Content: builder.String(),
	}}
}

func planStepPriorResults(currentStep *PlanStep, steps []*PlanStep) []string {
	currentIndex := 0
	if currentStep != nil {
		currentIndex = currentStep.Index
	}
	results := make([]string, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			continue
		}
		if currentStep != nil && step.ID == currentStep.ID {
			continue
		}
		if step.Status != PlanStepStatusCompleted && step.Status != PlanStepStatusSkipped {
			continue
		}
		if currentIndex > 0 && step.Index >= currentIndex {
			continue
		}
		title := strings.TrimSpace(step.Title)
		if title == "" {
			title = step.ID
		}
		result := strings.TrimSpace(step.ResultContent)
		if result == "" {
			result = step.Status
		}
		results = append(results, fmt.Sprintf("- Step %d, %s: %s", step.Index, title, result))
	}
	return results
}

func formatPlanStepInput(input map[string]any) string {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Sprintf("%v", input)
	}
	return string(payload)
}

func normalizeMemoryType(memoryType string) string {
	switch strings.TrimSpace(memoryType) {
	case MemoryTypeShortTerm:
		return MemoryTypeShortTerm
	case MemoryTypeUserManaged:
		return MemoryTypeUserManaged
	case MemoryTypeLongTerm, "":
		return MemoryTypeLongTerm
	default:
		return MemoryTypeLongTerm
	}
}

func normalizeMemoryImportance(importance int) (int, error) {
	if importance == 0 {
		return 3, nil
	}
	if importance < 1 || importance > 5 {
		return 0, fmt.Errorf("importance must be between 1 and 5")
	}
	return importance, nil
}

func copyMetadata(metadata map[string]any) map[string]any {
	copied := map[string]any{}
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}

func boolPointer(value bool) *bool {
	return &value
}

// ExecuteTool 执行工具
func (s *Service) ExecuteTool(ctx context.Context, session auth.Session, agentID string, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	// 验证 Agent 存在且可访问
	agent, err := s.store.GetAgent(ctx, agentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	// 查找工具配置
	var targetTool *Tool
	for _, t := range agent.Tools {
		if t.Name == toolName && t.Enabled {
			targetTool = &t
			break
		}
	}
	if targetTool == nil {
		return nil, fmt.Errorf("tool not found or disabled: %s", toolName)
	}

	// 根据工具类型执行
	switch targetTool.Type {
	case "builtin":
		if !s.isCommercialBuiltinEnabled(toolName) {
			return &mcp.ToolResult{
				Content: fmt.Sprintf("builtin tool %s is disabled for default commercial use", toolName),
				IsError: true,
			}, nil
		}
		// 内置工具
		tool, ok := mcp.GetBuiltinTool(toolName)
		if !ok {
			return nil, fmt.Errorf("builtin tool not found: %s", toolName)
		}
		if toolName == "web_search" && s.webSearchProvider != nil {
			restore := mcp.SetWebSearchProviderForTest(s.webSearchProvider)
			defer restore()
		}
		return tool.Execute(ctx, args)

	case "mcp":
		// MCP 工具
		if s.mcpClient == nil {
			return nil, fmt.Errorf("MCP client not configured")
		}
		if targetTool.ServerID == "" {
			return nil, fmt.Errorf("MCP server not specified")
		}
		return s.mcpClient.CallTool(ctx, targetTool.ServerID, session.OrganizationID, toolName, args)

	default:
		return nil, fmt.Errorf("unknown tool type: %s", targetTool.Type)
	}
}

// ListAvailableTools 列出 Agent 可用的工具
func (s *Service) ListAvailableTools(ctx context.Context, session auth.Session, agentID string) ([]ToolDefinition, error) {
	// 验证 Agent 存在且可访问
	agent, err := s.store.GetAgent(ctx, agentID, session.OrganizationID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	toolsByName := make(map[string]ToolDefinition)
	addTool := func(def ToolDefinition) {
		if strings.TrimSpace(def.Name) == "" {
			return
		}
		toolsByName[def.Name] = def
	}

	for _, builtin := range mcp.ListDefaultCommercialBuiltinTools() {
		addTool(ToolDefinition{
			Name:        builtin.Name,
			Description: builtin.Description,
			InputSchema: builtin.InputSchema,
			ToolType:    "builtin",
			RiskLevel:   inferToolRiskLevel(builtin.Name),
		})
	}
	if s.webSearchProvider != nil {
		if builtin, ok := mcp.GetBuiltinTool("web_search"); ok {
			addTool(ToolDefinition{
				Name:        builtin.Name(),
				Description: builtin.Description(),
				InputSchema: builtin.InputSchema(),
				ToolType:    "builtin",
				RiskLevel:   inferToolRiskLevel(builtin.Name()),
			})
		}
	}

	// 添加启用的工具
	for _, t := range agent.Tools {
		if !t.Enabled {
			continue
		}

		def := ToolDefinition{
			Name:             t.Name,
			Description:      t.Description,
			ToolType:         normalizeToolType(t.Type),
			RequiresApproval: t.RequiresApproval,
			RiskLevel:        toolDefinitionRiskLevel(t),
		}

		// 获取 InputSchema
		if t.Type == "builtin" {
			if !s.isCommercialBuiltinEnabled(t.Name) {
				continue
			}
			if builtin, ok := mcp.GetBuiltinTool(t.Name); ok {
				def.InputSchema = builtin.InputSchema()
				if def.Description == "" {
					def.Description = builtin.Description()
				}
			}
		} else if t.Type == "mcp" && s.mcpClient != nil && t.ServerID != "" {
			mcpTools, err := s.mcpClient.ListTools(t.ServerID, session.OrganizationID)
			if err == nil {
				for _, mt := range mcpTools {
					if mt.Name == t.Name {
						def.InputSchema = mt.InputSchema
						if def.Description == "" {
							def.Description = mt.Description
						}
						break
					}
				}
			}
		} else if t.Type == "custom" {
			def.InputSchema = t.InputSchema
		}

		addTool(def)
	}

	tools := make([]ToolDefinition, 0, len(toolsByName))
	for _, def := range toolsByName {
		tools = append(tools, def)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools, nil
}

func (s *Service) isCommercialBuiltinEnabled(name string) bool {
	if mcp.IsDefaultCommercialBuiltin(name) {
		return true
	}
	return name == "web_search" && s.webSearchProvider != nil
}
