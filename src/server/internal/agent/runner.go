package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/memory"
)

// ErrMaxIterationsExceeded is returned when the tool-calling loop exhausts its
// iteration budget without the model producing a final non-tool response.
var ErrMaxIterationsExceeded = errors.New("tool loop exceeded max iterations")

// RunnerConfig Agent 运行配置
type RunnerConfig struct {
	MaxIterations int // 最大工具调用迭代次数
}

// DefaultRunnerConfig 默认运行配置
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		MaxIterations: 10,
	}
}

// Runner Agent 执行器
type Runner struct {
	store    Store
	gateway  chat.ChatGateway
	executor *ToolExecutor
	memory   MemorySearcher
	config   RunnerConfig
}

// NewRunner 创建 Agent Runner
func NewRunner(store Store, gateway chat.ChatGateway, executor *ToolExecutor, memory MemorySearcher, config RunnerConfig) *Runner {
	return &Runner{
		store:    store,
		gateway:  gateway,
		executor: executor,
		memory:   memory,
		config:   config,
	}
}

// RunResult 运行结果
type RunResult struct {
	Message    *Message `json:"message"`
	ToolCalls  int      `json:"toolCalls"`
	UsedMemory bool     `json:"usedMemory"`
}

// Run 执行 Agent 对话（轻量路径，无工具调用支持）。
// 始终通过 Runner 路由以确保消息持久化发生在正确的位置且仅发生一次。
func (r *Runner) Run(ctx context.Context, session auth.Session, agent *Agent, conversationID string, userContent string) (*RunResult, error) {
	result := &RunResult{}

	// 保存用户消息
	_, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "user", userContent, nil, "")
	if err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// 获取历史消息
	messages, err := r.store.ListMessages(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	// 构建对话上下文
	chatMessages := r.buildChatMessages(ctx, session, agent, messages, userContent)
	result.UsedMemory = agent.Config.EnableMemory && r.memory != nil

	// 构建 config
	config := chat.ConversationConfig{
		ModelID:              agent.Model,
		SystemPromptOverride: agent.SystemPrompt,
		Temperature:          agent.Config.Temperature,
		MaxOutputTokens:      agent.Config.MaxTokens,
	}
	if config.Temperature == 0 {
		config.Temperature = 1.0
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}
	ctx = withSessionRelayMetadata(ctx, session)

	// 执行循环
	iteration := 0
	for iteration < r.config.MaxIterations {
		iteration++

		// 调用 LLM
		reply, err := r.gateway.GenerateReply(ctx, chatMessages, config)
		if err != nil {
			return nil, fmt.Errorf("generate reply: %w", err)
		}

		// 检查是否有工具调用（简化版：暂不支持自动工具调用）
		// 实际实现需要 LLM 返回 tool_calls，这里先保存普通回复
		assistantMsg, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply, nil, "")
		if err != nil {
			return nil, fmt.Errorf("save assistant message: %w", err)
		}

		result.Message = assistantMsg
		break
	}

	return result, nil
}

// RunStream 流式执行 Agent 对话
func (r *Runner) RunStream(ctx context.Context, session auth.Session, agent *Agent, conversationID string, userContent string, onChunk func(string) error) (*RunResult, error) {
	result := &RunResult{}

	// 保存用户消息
	_, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "user", userContent, nil, "")
	if err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// 获取历史消息
	messages, err := r.store.ListMessages(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	// 构建对话上下文
	chatMessages := r.buildChatMessages(ctx, session, agent, messages, userContent)
	result.UsedMemory = agent.Config.EnableMemory && r.memory != nil

	// 构建 config
	config := chat.ConversationConfig{
		ModelID:              agent.Model,
		SystemPromptOverride: agent.SystemPrompt,
		Temperature:          agent.Config.Temperature,
		MaxOutputTokens:      agent.Config.MaxTokens,
	}
	if config.Temperature == 0 {
		config.Temperature = 1.0
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}
	ctx = withSessionRelayMetadata(ctx, session)

	// 流式生成
	var replyContent string
	err = r.gateway.GenerateReplyStream(ctx, chatMessages, config, func(chunk string) error {
		replyContent += chunk
		return onChunk(chunk)
	})
	if err != nil {
		return nil, fmt.Errorf("generate reply stream: %w", err)
	}

	// 保存完整回复
	assistantMsg, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", replyContent, nil, "")
	if err != nil {
		return nil, fmt.Errorf("save assistant message: %w", err)
	}

	result.Message = assistantMsg
	return result, nil
}

// buildChatMessages 构建对话消息列表
func (r *Runner) buildChatMessages(ctx context.Context, session auth.Session, agent *Agent, messages []*Message, userContent string) []chat.Message {
	// 转换为 chat.Message 格式
	chatMessages := make([]chat.Message, len(messages))
	for i, m := range messages {
		chatMessages[i] = chat.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			ToolCalls:  toolCallsToChatToolCalls(m.ToolCalls),
		}
	}

	// 如果启用 Memory，注入相关记忆
	if agent.Config.EnableMemory && r.memory != nil {
		chatMessages = r.injectMemory(ctx, session, userContent, chatMessages)
	}

	return chatMessages
}

// injectMemory 注入相关记忆到消息上下文
func (r *Runner) injectMemory(ctx context.Context, session auth.Session, query string, messages []chat.Message) []chat.Message {
	results, err := r.memory.Search(ctx, session, &memory.SearchRequest{
		Query:    query,
		TopK:     5,
		MinScore: 0.5,
	})
	if err != nil || len(results) == 0 {
		return messages
	}

	// 构建记忆上下文
	var memoryBuilder strings.Builder
	memoryBuilder.WriteString("Relevant information from memory:\n\n")
	for i, res := range results {
		memoryBuilder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, res.ChunkContent))
		if i < len(results)-1 {
			memoryBuilder.WriteString("\n")
		}
	}

	// 在系统提示后插入记忆上下文
	result := make([]chat.Message, 0, len(messages)+1)
	inserted := false
	for _, m := range messages {
		result = append(result, m)
		if !inserted && m.Role == "system" {
			result = append(result, chat.Message{
				Role:    "system",
				Content: memoryBuilder.String(),
			})
			inserted = true
		}
	}

	// 如果没有系统消息，在开头插入
	if !inserted {
		result = append([]chat.Message{{
			Role:    "system",
			Content: memoryBuilder.String(),
		}}, result...)
	}

	return result
}

// ExecuteTool 执行单个工具调用
func (r *Runner) ExecuteTool(ctx context.Context, agent *Agent, toolCall *ToolCall) (*ExecuteResult, error) {
	if r.executor == nil {
		return nil, fmt.Errorf("tool executor not configured")
	}
	return r.executor.Execute(ctx, agent, toolCall)
}

// GetToolDefinitions 获取工具定义
func (r *Runner) GetToolDefinitions(ctx context.Context, agent *Agent) ([]ToolDefinition, error) {
	if r.executor == nil {
		return nil, nil
	}
	return r.executor.GetToolDefinitions(ctx, agent)
}

// BuildOpenAITools 构建 OpenAI 工具格式
func (r *Runner) BuildOpenAITools(ctx context.Context, agent *Agent) ([]map[string]any, error) {
	definitions, err := r.GetToolDefinitions(ctx, agent)
	if err != nil {
		return nil, err
	}

	tools := make([]map[string]any, len(definitions))
	for i, def := range definitions {
		tools[i] = def.ToOpenAITool()
	}
	return tools, nil
}

// RunWithTools 执行带工具调用的对话（完整版）
// 注意：这需要 LLM 支持 function calling
func (r *Runner) RunWithTools(ctx context.Context, session auth.Session, agent *Agent, conversationID string, userContent string, onChunk func(string) error) (*RunResult, error) {
	result := &RunResult{}

	// 保存用户消息
	_, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "user", userContent, nil, "")
	if err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// 获取历史消息
	messages, err := r.store.ListMessages(ctx, conversationID, session.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	// 构建对话上下文
	chatMessages := r.buildChatMessages(ctx, session, agent, messages, userContent)
	result.UsedMemory = agent.Config.EnableMemory && r.memory != nil

	// 构建 config
	config := chat.ConversationConfig{
		ModelID:              agent.Model,
		SystemPromptOverride: agent.SystemPrompt,
		Temperature:          agent.Config.Temperature,
		MaxOutputTokens:      agent.Config.MaxTokens,
		ToolsEnabled:         true,
	}
	if config.Temperature == 0 {
		config.Temperature = 1.0
	}
	if config.MaxOutputTokens == 0 {
		config.MaxOutputTokens = 2048
	}
	ctx = withSessionRelayMetadata(ctx, session)

	structuredGateway, ok := r.gateway.(chat.StructuredReplyGenerator)
	if !ok {
		// Gateway does not support structured replies (tool calls).
		// Fall back to a plain-text path but still honor the streaming
		// callback so SendMessageStream consumers are not starved.
		reply, err := r.gateway.GenerateReply(ctx, chatMessages, config)
		if err != nil {
			return nil, fmt.Errorf("generate reply: %w", err)
		}
		if onChunk != nil {
			if err := onChunk(reply); err != nil {
				return nil, err
			}
		}
		assistantMsg, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply, nil, "")
		if err != nil {
			return nil, fmt.Errorf("save assistant message: %w", err)
		}
		result.Message = assistantMsg
		return result, nil
	}

	tools, err := r.BuildOpenAITools(ctx, agent)
	if err != nil {
		return nil, fmt.Errorf("build tools: %w", err)
	}

	// Tool-calling loop strategy:
	//
	// Each iteration uses GenerateStructuredReply (non-streamed) so the
	// caller can inspect tool_calls reliably.  Streaming function-call
	// responses requires streaming infrastructure that is not yet
	// implemented end-to-end.  The final assistant turn streams its
	// content to the caller in word-level chunks, preserving the
	// streaming UX without a second API call.
	for iteration := 0; iteration < r.config.MaxIterations; iteration++ {
		reply, err := structuredGateway.GenerateStructuredReply(ctx, chatMessages, config, tools)
		if err != nil {
			return nil, fmt.Errorf("generate structured reply: %w", err)
		}

		toolCalls := chatToolCallsToAgent(reply.ToolCalls)
		if len(toolCalls) > 0 {
			// Save the assistant message that requested tool calls.
			_, err = r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply.Content, toolCalls, "")
			if err != nil {
				return nil, fmt.Errorf("save assistant tool call message: %w", err)
			}

			// Execute each requested tool and persist the result as a
			// role=tool message linked by tool_call_id so the model can
			// correlate results to requests on the next iteration.
			for _, toolCall := range toolCalls {
				execResult, err := r.ExecuteTool(ctx, agent, &toolCall)
				if err != nil {
					return nil, fmt.Errorf("execute tool %s: %w", toolCall.Name, err)
				}
				if _, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "tool", execResult.Content, nil, toolCall.ID); err != nil {
					return nil, fmt.Errorf("save tool message: %w", err)
				}
			}

			result.ToolCalls += len(toolCalls)

			// Refresh the message view so the next iteration includes
			// the tool-call and tool-result messages.
			messages, err = r.store.ListMessages(ctx, conversationID, session.OrganizationID)
			if err != nil {
				return nil, fmt.Errorf("refresh messages: %w", err)
			}
			chatMessages = r.buildChatMessages(ctx, session, agent, messages, userContent)
			continue
		}

		// No tool calls — this is the final assistant answer.
		assistantMsg, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply.Content, nil, "")
		if err != nil {
			return nil, fmt.Errorf("save assistant message: %w", err)
		}
		result.Message = assistantMsg

		// Stream the final answer in word-level chunks when the caller
		// provided a streaming callback.  We already have the full
		// content from GenerateStructuredReply; chunking it avoids
		// a second (duplicate) API call while still delivering a
		// streaming UX.
		if onChunk != nil && reply.Content != "" {
			if err := streamContent(reply.Content, onChunk); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	return nil, fmt.Errorf("%w (%d)", ErrMaxIterationsExceeded, r.config.MaxIterations)
}

func withSessionRelayMetadata(ctx context.Context, session auth.Session) context.Context {
	metadata, _ := chat.RelayRequestMetadataFromContext(ctx)
	if strings.TrimSpace(metadata.UserID) == "" {
		metadata.UserID = session.User.ID
	}
	if strings.TrimSpace(metadata.WorkspaceID) == "" {
		metadata.WorkspaceID = session.WorkspaceID
	}
	if strings.TrimSpace(metadata.OrganizationID) == "" {
		metadata.OrganizationID = session.OrganizationID
	}
	return chat.WithRelayRequestMetadata(ctx, metadata)
}

// MarshalToolCalls 序列化工具调用
func MarshalToolCalls(toolCalls []ToolCall) []byte {
	if len(toolCalls) == 0 {
		return nil
	}
	data, _ := json.Marshal(toolCalls)
	return data
}

// UnmarshalToolCalls 反序列化工具调用
func UnmarshalToolCalls(data []byte) []ToolCall {
	if len(data) == 0 {
		return nil
	}
	var toolCalls []ToolCall
	json.Unmarshal(data, &toolCalls)
	return toolCalls
}

func toolCallsToChatToolCalls(toolCalls []ToolCall) []chat.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]chat.ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		argumentsJSON, _ := json.Marshal(toolCall.Arguments)
		result = append(result, chat.ToolCall{
			ID:   toolCall.ID,
			Type: "function",
			Function: chat.ToolFunction{
				Name:      toolCall.Name,
				Arguments: string(argumentsJSON),
			},
		})
	}
	return result
}

func chatToolCallsToAgent(toolCalls []chat.ToolCall) []ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}

	result := make([]ToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		var arguments map[string]any
		if strings.TrimSpace(toolCall.Function.Arguments) != "" {
			_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments)
		}
		if arguments == nil {
			arguments = map[string]any{}
		}
		result = append(result, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: arguments,
		})
	}
	return result
}

// streamContent sends content to onChunk in word-level chunks to simulate
// streaming without a second API call.  This is used for the final assistant
// turn in a tool-calling loop where the full content was already obtained
// from GenerateStructuredReply.
func streamContent(content string, onChunk func(string) error) error {
	words := strings.Fields(content)
	for i, word := range words {
		chunk := word
		if i < len(words)-1 {
			chunk += " "
		}
		if err := onChunk(chunk); err != nil {
			return err
		}
	}
	return nil
}
