package agent

import (
	"context"
	"fmt"

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

	return s.store.CreateAgent(ctx, session.User.ID, req)
}

// GetAgent 获取 Agent
func (s *Service) GetAgent(ctx context.Context, session auth.Session, id string) (*Agent, error) {
	agent, err := s.store.GetAgent(ctx, id)
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
	return s.store.ListAgents(ctx, session.User.ID)
}

// UpdateAgent 更新 Agent
func (s *Service) UpdateAgent(ctx context.Context, session auth.Session, id string, req *UpdateAgentRequest) (*Agent, error) {
	// 验证所有权
	agent, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.UpdateAgent(ctx, id, req)
}

// DeleteAgent 删除 Agent
func (s *Service) DeleteAgent(ctx context.Context, session auth.Session, id string) error {
	// 验证所有权
	agent, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	return s.store.DeleteAgent(ctx, id)
}

// CreateConversation 创建对话
func (s *Service) CreateConversation(ctx context.Context, session auth.Session, agentID string) (*Conversation, error) {
	// 验证 Agent 存在且可访问
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}
	if agent.UserID != session.User.ID && !agent.IsPublic {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.CreateConversation(ctx, agentID, session.User.ID, "")
}

// GetConversation 获取对话
func (s *Service) GetConversation(ctx context.Context, session auth.Session, id string) (*Conversation, error) {
	conv, err := s.store.GetConversation(ctx, id)
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
	return s.store.ListConversations(ctx, agentID, session.User.ID)
}

// DeleteConversation 删除对话
func (s *Service) DeleteConversation(ctx context.Context, session auth.Session, id string) error {
	conv, err := s.store.GetConversation(ctx, id)
	if err != nil {
		return err
	}
	if conv == nil {
		return fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return fmt.Errorf("access denied")
	}

	return s.store.DeleteConversation(ctx, id)
}

// SendMessage 发送消息
func (s *Service) SendMessage(ctx context.Context, session auth.Session, conversationID string, content string) (*Message, error) {
	// 获取对话
	conv, err := s.store.GetConversation(ctx, conversationID)
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
	agent, err := s.store.GetAgent(ctx, conv.AgentID)
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
	conv, err := s.store.GetConversation(ctx, conversationID)
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
	agent, err := s.store.GetAgent(ctx, conv.AgentID)
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
	conv, err := s.store.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, fmt.Errorf("conversation not found")
	}
	if conv.UserID != session.User.ID {
		return nil, fmt.Errorf("access denied")
	}

	return s.store.ListMessages(ctx, conversationID)
}

func hasEnabledTools(agent *Agent) bool {
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
	agent, err := s.store.GetAgent(ctx, agentID)
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
		return s.mcpClient.CallTool(ctx, targetTool.ServerID, toolName, args)

	default:
		return nil, fmt.Errorf("unknown tool type: %s", targetTool.Type)
	}
}

// ListAvailableTools 列出 Agent 可用的工具
func (s *Service) ListAvailableTools(ctx context.Context, session auth.Session, agentID string) ([]ToolDefinition, error) {
	// 验证 Agent 存在且可访问
	agent, err := s.store.GetAgent(ctx, agentID)
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
			if builtin, ok := mcp.GetBuiltinTool(t.Name); ok {
				def.InputSchema = builtin.InputSchema()
				if def.Description == "" {
					def.Description = builtin.Description()
				}
			}
		} else if t.Type == "mcp" && s.mcpClient != nil && t.ServerID != "" {
			mcpTools, err := s.mcpClient.ListTools(t.ServerID)
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
