package agent

import (
	"context"
	"fmt"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
)

// MemorySearcher Memory 搜索接口
type MemorySearcher interface {
	Search(ctx context.Context, session auth.Session, req *memory.SearchRequest) ([]*memory.SearchResult, error)
}

// Service Agent 服务
type Service struct {
	store     Store
	gateway   chat.ChatGateway
	mcpClient *mcp.Client
	memory    MemorySearcher
	runner    *Runner
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
	s.runner = NewRunner(s.store, s.gateway, executor, s.memory, DefaultRunnerConfig())
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

// CreateAgent 创建 Agent
func (s *Service) CreateAgent(ctx context.Context, session auth.Session, req *CreateAgentRequest) (*Agent, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

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

	return s.store.UpdateAgent(ctx, id, session.OrganizationID, req)
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

// SendMessage 发送消息
func (s *Service) SendMessage(ctx context.Context, session auth.Session, conversationID string, content string) (*Message, error) {
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

	if hasEnabledTools(agent) {
		result, err := s.runner.RunWithTools(ctx, session, agent, conversationID, content, nil)
		if err != nil {
			return nil, err
		}
		return result.Message, nil
	}

	result, err := s.runner.Run(ctx, session, agent, conversationID, content)
	if err != nil {
		return nil, err
	}

	return result.Message, nil
}

// SendMessageStream 流式发送消息
func (s *Service) SendMessageStream(ctx context.Context, session auth.Session, conversationID string, content string, onChunk func(string) error) error {
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

	if hasEnabledTools(agent) {
		_, err = s.runner.RunWithTools(ctx, session, agent, conversationID, content, onChunk)
		return err
	}

	_, err = s.runner.RunStream(ctx, session, agent, conversationID, content, onChunk)
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
	return &RunDetail{Run: run, ToolRuns: toolRuns}, nil
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
	return s.store.UpdateToolRun(ctx, session.OrganizationID, toolRunID, UpdateToolRunRequest{
		Status:                 stringPointer(ToolRunStatusRunning),
		ApprovalStatus:         stringPointer(ApprovalStatusApproved),
		ApprovedByUserID:       stringPointer(session.User.ID),
		ApprovalDecisionReason: stringPointer(reason),
		AttemptCount:           intPointer(toolRun.AttemptCount + 1),
		StartedAt:              &now,
		ClearCompletedAt:       true,
	})
}

func (s *Service) RejectToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*ToolRun, error) {
	toolRun, err := s.getToolRunForSession(ctx, session, toolRunID)
	if err != nil {
		return nil, err
	}
	if toolRun.ApprovalStatus != ApprovalStatusPending || toolRun.Status != ToolRunStatusPendingApproval {
		return nil, fmt.Errorf("tool run is not pending approval")
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
	_, _ = s.store.UpdateRun(ctx, session.OrganizationID, toolRun.RunID, UpdateRunRequest{
		Status:      stringPointer(RunStatusFailed),
		Error:       stringPointer("tool run rejected: " + reason),
		CompletedAt: &completedAt,
	})
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
	now := time.Now().UTC()
	empty := ""
	return s.store.UpdateToolRun(ctx, session.OrganizationID, toolRunID, UpdateToolRunRequest{
		Status:           stringPointer(ToolRunStatusRunning),
		ApprovalStatus:   stringPointer(toolRun.ApprovalStatus),
		AttemptCount:     intPointer(toolRun.AttemptCount + 1),
		ResultContent:    &empty,
		Error:            &empty,
		StartedAt:        &now,
		ClearCompletedAt: true,
	})
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
		if !mcp.IsDefaultCommercialBuiltin(toolName) {
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

	var tools []ToolDefinition

	// 添加启用的工具
	for _, t := range agent.Tools {
		if !t.Enabled {
			continue
		}

		def := ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
		}

		// 获取 InputSchema
		if t.Type == "builtin" {
			if !mcp.IsDefaultCommercialBuiltin(t.Name) {
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
		}

		tools = append(tools, def)
	}

	return tools, nil
}
