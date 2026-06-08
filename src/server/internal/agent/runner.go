package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/metrics"
)

// ErrMaxIterationsExceeded is returned when the tool-calling loop exhausts its
// iteration budget without the model producing a final non-tool response.
var ErrMaxIterationsExceeded = errors.New("tool loop exceeded max iterations")

// ErrTokenBudgetExceeded is returned when model usage exceeds a configured run
// token budget before another tool or model step can safely continue.
var ErrTokenBudgetExceeded = errors.New("token budget exceeded")

// ErrToolApprovalRequired is returned when a tool call has been persisted and
// the workflow is waiting for explicit approval before execution.
var ErrToolApprovalRequired = errors.New("tool execution requires approval")

// RunnerConfig Agent 运行配置
type RunnerConfig struct {
	MaxIterations int // 最大工具调用迭代次数
}

const maxTokenBudget = 1_000_000

const (
	shortTermMessageLimit = 50
	shortTermTokenLimit   = 10_000
)

const (
	ExecutionModeReact    = "react"
	ExecutionModePlanning = "planning"

	ApprovalModeTiered = "tiered"
	ApprovalModeAll    = "all"
	ApprovalModeNone   = "none"
	ApprovalModeCustom = "custom"

	ToolRiskSafe      = "safe"
	ToolRiskMedium    = "medium"
	ToolRiskDangerous = "dangerous"
)

// DefaultRunnerConfig 默认运行配置
func DefaultRunnerConfig() RunnerConfig {
	return RunnerConfig{
		MaxIterations: 10,
	}
}

// NormalizeConfig applies runtime defaults and limits for user-configurable
// Agent execution controls.
func NormalizeConfig(config Config) Config {
	config.MaxIterations = normalizeMaxIterations(config.MaxIterations, DefaultRunnerConfig().MaxIterations)
	config.TokenBudget = normalizeTokenBudget(config.TokenBudget)
	config.DefaultExecutionMode = NormalizeExecutionMode(config.DefaultExecutionMode)
	config.ApprovalMode = normalizeApprovalMode(config.ApprovalMode)
	return config
}

func NormalizeConfigForWrite(config Config) (Config, error) {
	normalizedMode, err := NormalizeExecutionModeForWrite(config.DefaultExecutionMode)
	if err != nil {
		return Config{}, err
	}
	config = NormalizeConfig(config)
	config.DefaultExecutionMode = normalizedMode
	return config, nil
}

func NormalizeExecutionMode(value string) string {
	normalized, err := NormalizeExecutionModeForWrite(value)
	if err != nil {
		return ExecutionModeReact
	}
	return normalized
}

func NormalizeExecutionModeForWrite(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ExecutionModeReact:
		return ExecutionModeReact, nil
	case ExecutionModePlanning:
		return ExecutionModePlanning, nil
	default:
		return "", fmt.Errorf("defaultExecutionMode must be react or planning")
	}
}

func normalizeApprovalMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ApprovalModeAll:
		return ApprovalModeAll
	case ApprovalModeNone:
		return ApprovalModeNone
	case ApprovalModeCustom:
		return ApprovalModeCustom
	default:
		return ApprovalModeTiered
	}
}

func normalizeMaxIterations(value, defaultValue int) int {
	if defaultValue <= 0 {
		defaultValue = 10
	}
	if value <= 0 {
		return defaultValue
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeTokenBudget(value int) int {
	if value <= 0 {
		return 0
	}
	if value < 1_000 {
		return 1_000
	}
	if value > maxTokenBudget {
		return maxTokenBudget
	}
	return value
}

type toolApprovalDecision struct {
	RiskLevel        string
	RequiresApproval bool
}

func (r *Runner) decideToolApproval(ctx context.Context, organizationID, conversationID string, agent *Agent, tool *Tool, toolCall ToolCall) toolApprovalDecision {
	riskLevel := normalizeToolRiskLevel("")
	if tool != nil {
		riskLevel = normalizeToolRiskLevel(tool.RiskLevel)
	}
	if riskLevel == "" {
		riskLevel = inferToolRiskLevel(toolCall.Name)
	}

	requiresApproval := false
	mode := ApprovalModeTiered
	if agent != nil {
		mode = normalizeApprovalMode(agent.Config.ApprovalMode)
	}
	switch mode {
	case ApprovalModeAll:
		requiresApproval = true
	case ApprovalModeNone:
		requiresApproval = false
	case ApprovalModeCustom:
		if override, ok := toolApprovalOverride(agent, toolCall.Name); ok {
			if normalized := normalizeToolRiskLevel(override.RiskLevel); normalized != "" {
				riskLevel = normalized
			}
			if override.RequiresApproval != nil {
				requiresApproval = *override.RequiresApproval
			} else {
				requiresApproval = r.tieredToolRequiresApproval(ctx, organizationID, conversationID, toolCall.Name, riskLevel)
			}
		} else {
			requiresApproval = r.tieredToolRequiresApproval(ctx, organizationID, conversationID, toolCall.Name, riskLevel)
		}
	default:
		requiresApproval = r.tieredToolRequiresApproval(ctx, organizationID, conversationID, toolCall.Name, riskLevel)
	}
	if tool != nil && tool.RequiresApproval {
		requiresApproval = true
	}
	return toolApprovalDecision{RiskLevel: riskLevel, RequiresApproval: requiresApproval}
}

func toolApprovalOverride(agent *Agent, toolName string) (ToolApprovalOverride, bool) {
	if agent == nil || len(agent.Config.ToolApprovalOverrides) == 0 {
		return ToolApprovalOverride{}, false
	}
	override, ok := agent.Config.ToolApprovalOverrides[toolName]
	return override, ok
}

func (r *Runner) tieredToolRequiresApproval(ctx context.Context, organizationID, conversationID, toolName, riskLevel string) bool {
	switch normalizeToolRiskLevel(riskLevel) {
	case ToolRiskSafe:
		return false
	case ToolRiskMedium:
		return !r.hasApprovedMediumTool(ctx, organizationID, conversationID, toolName)
	case ToolRiskDangerous:
		return true
	default:
		return true
	}
}

func (r *Runner) hasApprovedMediumTool(ctx context.Context, organizationID, conversationID, toolName string) bool {
	if r == nil || r.store == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}
	runs, err := r.store.ListRuns(ctx, organizationID, conversationID)
	if err != nil {
		return false
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		toolRuns, err := r.store.ListToolRuns(ctx, organizationID, run.ID)
		if err != nil {
			continue
		}
		for _, toolRun := range toolRuns {
			if toolRun != nil &&
				toolRun.ConversationID == conversationID &&
				toolRun.ToolName == toolName &&
				normalizeToolRiskLevel(toolRun.RiskLevel) == ToolRiskMedium &&
				toolRun.ApprovalStatus == ApprovalStatusApproved {
				return true
			}
		}
	}
	return false
}

func normalizeToolRiskLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ToolRiskSafe:
		return ToolRiskSafe
	case ToolRiskMedium:
		return ToolRiskMedium
	case ToolRiskDangerous:
		return ToolRiskDangerous
	default:
		return ""
	}
}

func inferToolRiskLevel(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(normalized, "delete"),
		strings.Contains(normalized, "drop"),
		strings.Contains(normalized, "pay"),
		strings.Contains(normalized, "transfer"),
		strings.Contains(normalized, "execute_code"):
		return ToolRiskDangerous
	case strings.Contains(normalized, "write"),
		strings.Contains(normalized, "create"),
		strings.Contains(normalized, "update"),
		strings.Contains(normalized, "post"):
		return ToolRiskMedium
	default:
		return ToolRiskSafe
	}
}

// Runner Agent 执行器
type Runner struct {
	store          Store
	gateway        chat.ChatGateway
	executor       *ToolExecutor
	memory         MemorySearcher
	memoryEmbedder MemoryEmbedder
	config         RunnerConfig
}

type memoryVectorSearchStore interface {
	SearchMemories(ctx context.Context, organizationID, userID string, req SearchMemoriesRequest) ([]*MemorySearchResult, error)
}

type memoryUpdateStore interface {
	ListMemories(ctx context.Context, organizationID, userID string, req ListMemoriesRequest) ([]*Memory, error)
	UpdateMemory(ctx context.Context, organizationID, userID, id string, req UpdateMemoryStoreRequest) (*Memory, error)
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

func (r *Runner) SetMemoryEmbedder(embedder MemoryEmbedder) {
	r.memoryEmbedder = embedder
}

// RunResult 运行结果
type RunResult struct {
	Message           *Message `json:"message"`
	ToolCalls         int      `json:"toolCalls"`
	UsedMemory        bool     `json:"usedMemory"`
	MemorySearched    bool     `json:"memorySearched"`
	MemoryResultCount int      `json:"memoryResultCount"`
}

type memoryEvidence struct {
	enabled     bool
	searched    bool
	resultCount int
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

	ctx = withSessionRelayMetadata(ctx, session)

	// 构建对话上下文
	chatMessages, evidence := r.buildChatMessagesWithEvidence(ctx, session, agent, messages, userContent)
	result.UsedMemory = evidence.enabled
	result.MemorySearched = evidence.searched
	result.MemoryResultCount = evidence.resultCount

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
		r.storeLongTermInteractionMemory(ctx, session, agent, conversationID, userContent, reply)
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

	ctx = withSessionRelayMetadata(ctx, session)

	// 构建对话上下文
	chatMessages, evidence := r.buildChatMessagesWithEvidence(ctx, session, agent, messages, userContent)
	result.UsedMemory = evidence.enabled
	result.MemorySearched = evidence.searched
	result.MemoryResultCount = evidence.resultCount

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
	chatMessages, _ := r.buildChatMessagesWithEvidence(ctx, session, agent, messages, userContent)
	return chatMessages
}

func (r *Runner) buildChatMessagesWithEvidence(ctx context.Context, session auth.Session, agent *Agent, messages []*Message, userContent string) ([]chat.Message, memoryEvidence) {
	messages = limitShortTermMessages(messages)
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

	evidence := memoryEvidence{enabled: agent.Config.EnableMemory && (r.memory != nil || r.store != nil)}

	// 如果启用 Memory，注入相关记忆
	if evidence.enabled {
		results, searched := r.searchMemory(ctx, session, agent, userContent)
		evidence.searched = searched
		evidence.resultCount = len(results)
		if len(results) > 0 {
			chatMessages = injectMemoryResults(results, chatMessages)
		}
	}

	return chatMessages, evidence
}

func limitShortTermMessages(messages []*Message) []*Message {
	if len(messages) <= 1 {
		return messages
	}
	start := len(messages)
	estimatedTokens := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if len(messages)-i > shortTermMessageLimit {
			break
		}
		messageTokens := estimateAgentMessageTokens(messages[i])
		if estimatedTokens > 0 && estimatedTokens+messageTokens > shortTermTokenLimit {
			break
		}
		estimatedTokens += messageTokens
		start = i
	}
	if start == 0 {
		return messages
	}
	for start < len(messages) && isOrphanedToolResult(messages[start]) {
		start++
	}
	return messages[start:]
}

func estimateAgentMessageTokens(message *Message) int {
	if message == nil {
		return 0
	}
	tokens := estimateTextTokens(message.Role) + estimateTextTokens(message.Content) + estimateTextTokens(message.ToolCallID)
	for _, toolCall := range message.ToolCalls {
		tokens += estimateTextTokens(toolCall.ID) + estimateTextTokens(toolCall.Name)
		if len(toolCall.Arguments) > 0 {
			if payload, err := json.Marshal(toolCall.Arguments); err == nil {
				tokens += estimateTextTokens(string(payload))
			}
		}
	}
	return tokens
}

func estimateTextTokens(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	runeCount := utf8.RuneCountInString(trimmed)
	tokens := runeCount / 4
	if runeCount%4 != 0 {
		tokens++
	}
	if tokens < 1 {
		return 1
	}
	return tokens
}

func isOrphanedToolResult(message *Message) bool {
	return message != nil && message.Role == "tool" && strings.TrimSpace(message.ToolCallID) != ""
}

func (r *Runner) searchMemory(ctx context.Context, session auth.Session, agent *Agent, query string) ([]*memory.SearchResult, bool) {
	results := make([]*memory.SearchResult, 0, 5)
	if r.memory != nil {
		internalResults, err := r.memory.Search(ctx, session, &memory.SearchRequest{
			Query:    query,
			TopK:     5,
			MinScore: 0.5,
		})
		if err == nil {
			results = append(results, internalResults...)
		}
	}

	if r.store != nil && agent != nil && agent.ID != "" {
		if vectorResults, ok := r.searchUserManagedMemoriesByVector(ctx, session, agent, query); ok {
			results = append(results, vectorResults...)
			return results, true
		}
		managedMemories, err := r.store.ListMemories(ctx, session.OrganizationID, session.User.ID, ListMemoriesRequest{
			AgentID: agent.ID,
			Type:    MemoryTypeUserManaged,
			Query:   query,
			Limit:   5,
		})
		if err == nil {
			for _, managedMemory := range managedMemories {
				if managedMemory == nil {
					continue
				}
				results = append(results, &memory.SearchResult{
					DocumentID:    "agent_memory:" + managedMemory.ID,
					DocumentTitle: "Agent memory",
					ChunkContent:  managedMemory.Content,
					ChunkIndex:    0,
					Score:         1,
				})
			}
		}
	}
	return results, true
}

func (r *Runner) searchUserManagedMemoriesByVector(ctx context.Context, session auth.Session, agent *Agent, query string) ([]*memory.SearchResult, bool) {
	if r.memoryEmbedder == nil || r.store == nil || agent == nil || strings.TrimSpace(query) == "" {
		return nil, false
	}
	vectorStore, ok := r.store.(memoryVectorSearchStore)
	if !ok {
		return nil, false
	}
	embedding, err := r.memoryEmbedder.Embed(withAgentMemoryRelayIdentity(ctx, session), query)
	if err != nil || len(embedding) == 0 {
		return nil, false
	}
	matches, err := vectorStore.SearchMemories(ctx, session.OrganizationID, session.User.ID, SearchMemoriesRequest{
		AgentID:   agent.ID,
		Type:      MemoryTypeUserManaged,
		Embedding: embedding,
		Limit:     5,
		MinScore:  0.5,
	})
	if err != nil || len(matches) == 0 {
		return nil, false
	}
	results := make([]*memory.SearchResult, 0, len(matches))
	for _, match := range matches {
		if match == nil {
			continue
		}
		results = append(results, &memory.SearchResult{
			DocumentID:    "agent_memory:" + match.Memory.ID,
			DocumentTitle: "Agent memory",
			ChunkContent:  match.Memory.Content,
			ChunkIndex:    0,
			Score:         match.Score,
		})
	}
	return results, len(results) > 0
}

// injectMemoryResults 注入相关记忆到消息上下文
func injectMemoryResults(results []*memory.SearchResult, messages []chat.Message) []chat.Message {
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

	ctx = withSessionRelayMetadata(ctx, session)

	// 构建对话上下文
	chatMessages, evidence := r.buildChatMessagesWithEvidence(ctx, session, agent, messages, userContent)
	result.UsedMemory = evidence.enabled
	result.MemorySearched = evidence.searched
	result.MemoryResultCount = evidence.resultCount

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

	metadata, _ := chat.RelayRequestMetadataFromContext(ctx)
	run, err := r.store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID:    session.OrganizationID,
		ConversationID:    conversationID,
		AgentID:           agent.ID,
		UserID:            session.User.ID,
		RequestID:         metadata.RequestID,
		Mode:              ExecutionModeReact,
		Status:            RunStatusRunning,
		MemoryEnabled:     evidence.enabled,
		MemorySearched:    evidence.searched,
		MemoryResultCount: evidence.resultCount,
	})
	if err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}
	if run == nil {
		return nil, fmt.Errorf("create agent run: no row created")
	}

	structuredGateway, ok := r.gateway.(chat.StructuredReplyGenerator)
	if !ok {
		// Gateway does not support structured replies (tool calls).
		// Fall back to a plain-text path but still honor the streaming
		// callback so SendMessageStream consumers are not starved.
		reply, err := r.gateway.GenerateReply(ctx, chatMessages, config)
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), 1, result.ToolCalls)
			return nil, fmt.Errorf("generate reply: %w", err)
		}
		if onChunk != nil {
			if err := onChunk(reply); err != nil {
				_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), 1, result.ToolCalls)
				return nil, err
			}
		}
		assistantMsg, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply, nil, "")
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), 1, result.ToolCalls)
			return nil, fmt.Errorf("save assistant message: %w", err)
		}
		if err := r.completeRun(ctx, session.OrganizationID, run.ID, 1, result.ToolCalls, assistantMsg.ID); err != nil {
			return nil, err
		}
		result.Message = assistantMsg
		r.storeLongTermInteractionMemory(ctx, session, agent, conversationID, userContent, reply)
		return result, nil
	}

	tools, err := r.BuildOpenAITools(ctx, agent)
	if err != nil {
		_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), 0, result.ToolCalls)
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
	maxIterations := r.maxIterationsFor(agent)
	tokenBudget := normalizeTokenBudget(agent.Config.TokenBudget)
	usedTokens := 0
	for iteration := 0; iteration < maxIterations; iteration++ {
		reply, err := structuredGateway.GenerateStructuredReply(ctx, chatMessages, config, tools)
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
			return nil, fmt.Errorf("generate structured reply: %w", err)
		}
		usedTokens += completionTotalTokens(reply)
		if tokenBudget > 0 && usedTokens > tokenBudget {
			message := fmt.Sprintf("token_budget_exceeded: used %d tokens exceeds budget %d", usedTokens, tokenBudget)
			_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusTokenBudgetExceeded, message, iteration+1, result.ToolCalls)
			return nil, fmt.Errorf("%w: used %d tokens exceeds budget %d", ErrTokenBudgetExceeded, usedTokens, tokenBudget)
		}

		toolCalls := chatToolCallsToAgent(reply.ToolCalls)
		if len(toolCalls) > 0 {
			result.ToolCalls, err = r.handleStructuredToolCalls(ctx, session, agent, run, conversationID, reply.Content, toolCalls, iteration+1, result.ToolCalls)
			if err != nil {
				return nil, err
			}

			// Refresh the message view so the next iteration includes
			// the tool-call and tool-result messages.
			messages, err = r.store.ListMessages(ctx, conversationID, session.OrganizationID)
			if err != nil {
				_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
				return nil, fmt.Errorf("refresh messages: %w", err)
			}
			chatMessages = r.buildChatMessages(ctx, session, agent, messages, userContent)
			continue
		}

		// No tool calls — this is the final assistant answer.
		assistantMsg, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", reply.Content, nil, "")
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
			return nil, fmt.Errorf("save assistant message: %w", err)
		}
		result.Message = assistantMsg
		if err := r.completeRun(ctx, session.OrganizationID, run.ID, iteration+1, result.ToolCalls, assistantMsg.ID); err != nil {
			return nil, err
		}
		r.storeLongTermInteractionMemory(ctx, session, agent, conversationID, userContent, reply.Content)

		// Stream the final answer in word-level chunks when the caller
		// provided a streaming callback.  We already have the full
		// content from GenerateStructuredReply; chunking it avoids
		// a second (duplicate) API call while still delivering a
		// streaming UX.
		if onChunk != nil && reply.Content != "" {
			if err := streamContent(reply.Content, onChunk); err != nil {
				_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
				return nil, err
			}
		}
		return result, nil
	}

	_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusMaxIterationsReached, ErrMaxIterationsExceeded.Error(), maxIterations, result.ToolCalls)
	return nil, fmt.Errorf("%w (%d)", ErrMaxIterationsExceeded, maxIterations)
}

func (r *Runner) ResumeAfterApprovedTool(ctx context.Context, session auth.Session, agent *Agent, run *Run) (*RunResult, error) {
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}
	result := &RunResult{}
	messages, err := r.store.ListMessages(ctx, run.ConversationID, session.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("refresh messages: %w", err)
	}
	chatMessages := r.buildChatMessages(ctx, session, agent, messages, "")

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
	maxIterations := r.maxIterationsFor(agent)
	if run.IterationCount >= maxIterations {
		_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusMaxIterationsReached, ErrMaxIterationsExceeded.Error(), run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("%w (%d)", ErrMaxIterationsExceeded, maxIterations)
	}
	tokenBudget := normalizeTokenBudget(agent.Config.TokenBudget)
	structuredGateway, ok := r.gateway.(chat.StructuredReplyGenerator)
	if !ok {
		reply, err := r.gateway.GenerateReply(ctx, chatMessages, config)
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount+1, run.ToolCallCount)
			return nil, fmt.Errorf("generate reply: %w", err)
		}
		estimatedTokens := estimateChatMessageTokens(append(chatMessages, chat.Message{Role: "assistant", Content: reply}))
		if tokenBudget > 0 && estimatedTokens > tokenBudget {
			message := fmt.Sprintf("token_budget_exceeded: estimated %d tokens exceeds budget %d", estimatedTokens, tokenBudget)
			_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusTokenBudgetExceeded, message, run.IterationCount+1, run.ToolCallCount)
			return nil, fmt.Errorf("%w: estimated %d tokens exceeds budget %d", ErrTokenBudgetExceeded, estimatedTokens, tokenBudget)
		}
		assistantMsg, err := r.store.CreateMessage(ctx, run.ConversationID, session.OrganizationID, "assistant", reply, nil, "")
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount+1, run.ToolCallCount)
			return nil, fmt.Errorf("save assistant message: %w", err)
		}
		result.Message = assistantMsg
		if err := r.completeRun(ctx, session.OrganizationID, run.ID, run.IterationCount+1, run.ToolCallCount, assistantMsg.ID); err != nil {
			return nil, err
		}
		r.storeLongTermInteractionMemory(ctx, session, agent, run.ConversationID, lastUserMessageContent(messages), reply)
		return result, nil
	}

	tools, err := r.BuildOpenAITools(ctx, agent)
	if err != nil {
		_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount, run.ToolCallCount)
		return nil, fmt.Errorf("build tools: %w", err)
	}
	reply, err := structuredGateway.GenerateStructuredReply(ctx, chatMessages, config, tools)
	if err != nil {
		_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount+1, run.ToolCallCount)
		return nil, fmt.Errorf("generate structured reply: %w", err)
	}
	usedTokens := completionTotalTokens(reply)
	if tokenBudget > 0 && usedTokens > tokenBudget {
		message := fmt.Sprintf("token_budget_exceeded: used %d tokens exceeds budget %d", usedTokens, tokenBudget)
		_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusTokenBudgetExceeded, message, run.IterationCount+1, run.ToolCallCount)
		return nil, fmt.Errorf("%w: used %d tokens exceeds budget %d", ErrTokenBudgetExceeded, usedTokens, tokenBudget)
	}
	toolCalls := chatToolCallsToAgent(reply.ToolCalls)
	if len(toolCalls) > 0 {
		result.ToolCalls, err = r.handleStructuredToolCalls(ctx, session, agent, run, run.ConversationID, reply.Content, toolCalls, run.IterationCount+1, run.ToolCallCount)
		if err != nil {
			return nil, err
		}
		messages, err = r.store.ListMessages(ctx, run.ConversationID, session.OrganizationID)
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount+1, result.ToolCalls)
			return nil, fmt.Errorf("refresh messages: %w", err)
		}
		chatMessages = r.buildChatMessages(ctx, session, agent, messages, "")
		run.IterationCount++
		run.ToolCallCount = result.ToolCalls
		return r.resumeToolLoop(ctx, session, agent, run, chatMessages, config, tools, structuredGateway, tokenBudget, usedTokens)
	}
	assistantMsg, err := r.store.CreateMessage(ctx, run.ConversationID, session.OrganizationID, "assistant", reply.Content, nil, "")
	if err != nil {
		_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), run.IterationCount+1, run.ToolCallCount)
		return nil, fmt.Errorf("save assistant message: %w", err)
	}
	result.Message = assistantMsg
	if err := r.completeRun(ctx, session.OrganizationID, run.ID, run.IterationCount+1, run.ToolCallCount, assistantMsg.ID); err != nil {
		return nil, err
	}
	r.storeLongTermInteractionMemory(ctx, session, agent, run.ConversationID, lastUserMessageContent(messages), reply.Content)
	return result, nil
}

func (r *Runner) resumeToolLoop(ctx context.Context, session auth.Session, agent *Agent, run *Run, chatMessages []chat.Message, config chat.ConversationConfig, tools []map[string]any, structuredGateway chat.StructuredReplyGenerator, tokenBudget int, usedTokens int) (*RunResult, error) {
	result := &RunResult{ToolCalls: run.ToolCallCount}
	maxIterations := r.maxIterationsFor(agent)
	for iteration := run.IterationCount; iteration < maxIterations; iteration++ {
		reply, err := structuredGateway.GenerateStructuredReply(ctx, chatMessages, config, tools)
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
			return nil, fmt.Errorf("generate structured reply: %w", err)
		}
		usedTokens += completionTotalTokens(reply)
		if tokenBudget > 0 && usedTokens > tokenBudget {
			message := fmt.Sprintf("token_budget_exceeded: used %d tokens exceeds budget %d", usedTokens, tokenBudget)
			_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusTokenBudgetExceeded, message, iteration+1, result.ToolCalls)
			return nil, fmt.Errorf("%w: used %d tokens exceeds budget %d", ErrTokenBudgetExceeded, usedTokens, tokenBudget)
		}

		toolCalls := chatToolCallsToAgent(reply.ToolCalls)
		if len(toolCalls) > 0 {
			result.ToolCalls, err = r.handleStructuredToolCalls(ctx, session, agent, run, run.ConversationID, reply.Content, toolCalls, iteration+1, result.ToolCalls)
			if err != nil {
				return nil, err
			}
			messages, err := r.store.ListMessages(ctx, run.ConversationID, session.OrganizationID)
			if err != nil {
				_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
				return nil, fmt.Errorf("refresh messages: %w", err)
			}
			chatMessages = r.buildChatMessages(ctx, session, agent, messages, "")
			continue
		}

		assistantMsg, err := r.store.CreateMessage(ctx, run.ConversationID, session.OrganizationID, "assistant", reply.Content, nil, "")
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iteration+1, result.ToolCalls)
			return nil, fmt.Errorf("save assistant message: %w", err)
		}
		result.Message = assistantMsg
		if err := r.completeRun(ctx, session.OrganizationID, run.ID, iteration+1, result.ToolCalls, assistantMsg.ID); err != nil {
			return nil, err
		}
		messages, _ := r.store.ListMessages(ctx, run.ConversationID, session.OrganizationID)
		r.storeLongTermInteractionMemory(ctx, session, agent, run.ConversationID, lastUserMessageContent(messages), reply.Content)
		return result, nil
	}

	_ = r.failRunWithStatus(ctx, session.OrganizationID, run.ID, RunStatusMaxIterationsReached, ErrMaxIterationsExceeded.Error(), maxIterations, result.ToolCalls)
	return nil, fmt.Errorf("%w (%d)", ErrMaxIterationsExceeded, maxIterations)
}

func (r *Runner) handleStructuredToolCalls(ctx context.Context, session auth.Session, agent *Agent, run *Run, conversationID, assistantContent string, toolCalls []ToolCall, iterationCount, previousToolCallCount int) (int, error) {
	_, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "assistant", assistantContent, toolCalls, "")
	if err != nil {
		return previousToolCallCount, fmt.Errorf("save assistant tool call message: %w", err)
	}

	totalToolCallCount := previousToolCallCount + len(toolCalls)
	for _, toolCall := range toolCalls {
		targetTool := findEnabledTool(agent, toolCall.Name)
		toolType := ""
		serverID := ""
		if targetTool != nil {
			toolType = targetTool.Type
			serverID = targetTool.ServerID
		}
		approvalDecision := r.decideToolApproval(ctx, session.OrganizationID, conversationID, agent, targetTool, toolCall)
		status := ToolRunStatusRunning
		approvalStatus := ApprovalStatusNotRequired
		attemptCount := 1
		var startedAt *time.Time
		if approvalDecision.RequiresApproval {
			status = ToolRunStatusPendingApproval
			approvalStatus = ApprovalStatusPending
			attemptCount = 0
		} else {
			now := time.Now().UTC()
			startedAt = &now
		}
		toolRun, err := r.store.CreateToolRun(ctx, &CreateToolRunRequest{
			OrganizationID: session.OrganizationID,
			RunID:          run.ID,
			ConversationID: conversationID,
			AgentID:        agent.ID,
			ToolCallID:     toolCall.ID,
			ToolName:       toolCall.Name,
			ToolType:       toolType,
			ServerID:       serverID,
			RiskLevel:      approvalDecision.RiskLevel,
			Arguments:      toolCall.Arguments,
			Status:         status,
			ApprovalStatus: approvalStatus,
			AttemptCount:   attemptCount,
			StartedAt:      startedAt,
		})
		if err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iterationCount, previousToolCallCount)
			return previousToolCallCount, fmt.Errorf("create tool run %s: %w", toolCall.Name, err)
		}
		if approvalDecision.RequiresApproval {
			if _, updateErr := r.store.UpdateRun(ctx, session.OrganizationID, run.ID, UpdateRunRequest{
				Status:         stringPointer(RunStatusPendingApproval),
				IterationCount: intPointer(iterationCount),
				ToolCallCount:  intPointer(totalToolCallCount),
				Error:          stringPointer(""),
			}); updateErr == nil {
				recordAgentRunMetrics(RunStatusPendingApproval, iterationCount)
				metrics.RecordAgentToolCall(toolCall.Name, string(ToolRunStatusPendingApproval))
			}
			return totalToolCallCount, ErrToolApprovalRequired
		}
		execResult, err := r.ExecuteTool(ctx, agent, &toolCall)
		if err != nil {
			_ = r.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolCall.Name, err.Error(), attemptCount)
			_ = r.failRun(ctx, session.OrganizationID, run.ID, fmt.Sprintf("execute tool %s: %s", toolCall.Name, err.Error()), iterationCount, previousToolCallCount)
			return previousToolCallCount, fmt.Errorf("execute tool %s: %w", toolCall.Name, err)
		}
		if execResult == nil {
			toolErr := fmt.Sprintf("tool %s returned no result", toolCall.Name)
			_ = r.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolCall.Name, toolErr, attemptCount)
			_ = r.failRun(ctx, session.OrganizationID, run.ID, toolErr, iterationCount, previousToolCallCount)
			return previousToolCallCount, fmt.Errorf("%s", toolErr)
		}
		if execResult.IsError {
			_ = r.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolCall.Name, execResult.Content, attemptCount)
			_ = r.failRun(ctx, session.OrganizationID, run.ID, fmt.Sprintf("tool %s failed: %s", toolCall.Name, execResult.Content), iterationCount, previousToolCallCount)
			return previousToolCallCount, fmt.Errorf("tool %s failed: %s", toolCall.Name, execResult.Content)
		}
		if _, err := r.store.CreateMessage(ctx, conversationID, session.OrganizationID, "tool", execResult.Content, nil, toolCall.ID); err != nil {
			_ = r.failToolRun(ctx, session.OrganizationID, toolRun.ID, toolCall.Name, err.Error(), attemptCount)
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iterationCount, previousToolCallCount)
			return previousToolCallCount, fmt.Errorf("save tool message: %w", err)
		}
		completedAt := time.Now().UTC()
		if _, err := r.store.UpdateToolRun(ctx, session.OrganizationID, toolRun.ID, UpdateToolRunRequest{
			Status:        stringPointer(ToolRunStatusCompleted),
			ResultContent: stringPointer(execResult.Content),
			AttemptCount:  intPointer(attemptCount),
			CompletedAt:   &completedAt,
		}); err != nil {
			_ = r.failRun(ctx, session.OrganizationID, run.ID, err.Error(), iterationCount, previousToolCallCount)
			return previousToolCallCount, fmt.Errorf("complete tool run %s: %w", toolCall.Name, err)
		}
		metrics.RecordAgentToolCall(toolCall.Name, string(ToolRunStatusCompleted))
	}
	return totalToolCallCount, nil
}

func (r *Runner) maxIterationsFor(agent *Agent) int {
	defaultValue := r.config.MaxIterations
	if agent == nil {
		return normalizeMaxIterations(0, defaultValue)
	}
	return normalizeMaxIterations(agent.Config.MaxIterations, defaultValue)
}

func completionTotalTokens(reply *chat.CompletionResponse) int {
	if reply == nil || reply.Usage == nil || reply.Usage.TotalTokens <= 0 {
		return 0
	}
	return reply.Usage.TotalTokens
}

func (r *Runner) completeRun(ctx context.Context, organizationID, runID string, iterationCount, toolCallCount int, finalMessageID string) error {
	completedAt := time.Now().UTC()
	_, err := r.store.UpdateRun(ctx, organizationID, runID, UpdateRunRequest{
		Status:         stringPointer(RunStatusCompleted),
		IterationCount: intPointer(iterationCount),
		ToolCallCount:  intPointer(toolCallCount),
		FinalMessageID: stringPointer(finalMessageID),
		CompletedAt:    &completedAt,
	})
	if err != nil {
		return fmt.Errorf("complete agent run: %w", err)
	}
	recordAgentRunMetrics(RunStatusCompleted, iterationCount)
	return nil
}

func (r *Runner) storeLongTermInteractionMemory(ctx context.Context, session auth.Session, agent *Agent, conversationID, userContent, assistantContent string) {
	if r == nil || r.store == nil || agent == nil || !agent.Config.EnableMemory {
		return
	}
	userContent = strings.TrimSpace(userContent)
	assistantContent = strings.TrimSpace(assistantContent)
	if userContent == "" || assistantContent == "" {
		return
	}
	content := fmt.Sprintf("User: %s\nAssistant: %s", userContent, assistantContent)
	req := &CreateMemoryStoreRequest{
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		AgentID:        agent.ID,
		Type:           MemoryTypeLongTerm,
		Content:        content,
		Importance:     3,
		Metadata: map[string]any{
			"source":          "agent_run",
			"conversation_id": conversationID,
		},
	}
	if r.memoryEmbedder != nil {
		embedding, err := r.memoryEmbedder.Embed(withSessionRelayMetadata(ctx, session), content)
		if err != nil {
			return
		}
		req.Embedding = embedding
	}
	if r.refreshDuplicateLongTermInteractionMemory(ctx, session, agent, content, req.Embedding) {
		return
	}
	_, _ = r.store.CreateMemory(ctx, req)
}

func (r *Runner) refreshDuplicateLongTermInteractionMemory(ctx context.Context, session auth.Session, agent *Agent, content string, embedding []float32) bool {
	store, ok := r.store.(memoryUpdateStore)
	if !ok {
		return false
	}
	memories, err := store.ListMemories(ctx, session.OrganizationID, session.User.ID, ListMemoriesRequest{
		AgentID: agent.ID,
		Type:    MemoryTypeLongTerm,
		Limit:   20,
	})
	if err != nil {
		return false
	}
	normalizedContent := normalizeMemoryContent(content)
	for _, memory := range memories {
		if memory == nil || normalizeMemoryContent(memory.Content) != normalizedContent {
			continue
		}
		importance := memory.Importance
		if importance <= 0 {
			importance = 3
		}
		_, err := store.UpdateMemory(ctx, session.OrganizationID, session.User.ID, memory.ID, UpdateMemoryStoreRequest{
			Content:    stringPointer(content),
			Embedding:  append([]float32(nil), embedding...),
			Importance: intPointer(importance),
		})
		return err == nil
	}
	return false
}

func normalizeMemoryContent(content string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(content))), " ")
}

func lastUserMessageContent(messages []*Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] != nil && messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func (r *Runner) failRun(ctx context.Context, organizationID, runID, message string, iterationCount, toolCallCount int) error {
	return r.failRunWithStatus(ctx, organizationID, runID, RunStatusFailed, message, iterationCount, toolCallCount)
}

func (r *Runner) failRunWithStatus(ctx context.Context, organizationID, runID, status, message string, iterationCount, toolCallCount int) error {
	if status == "" {
		status = RunStatusFailed
	}
	completedAt := time.Now().UTC()
	_, err := r.store.UpdateRun(ctx, organizationID, runID, UpdateRunRequest{
		Status:         stringPointer(status),
		IterationCount: intPointer(iterationCount),
		ToolCallCount:  intPointer(toolCallCount),
		Error:          stringPointer(message),
		CompletedAt:    &completedAt,
	})
	if err == nil {
		recordAgentRunMetrics(status, iterationCount)
	}
	return err
}

func (r *Runner) failToolRun(ctx context.Context, organizationID, toolRunID, toolName, message string, attemptCount int) error {
	completedAt := time.Now().UTC()
	_, err := r.store.UpdateToolRun(ctx, organizationID, toolRunID, UpdateToolRunRequest{
		Status:       stringPointer(ToolRunStatusFailed),
		Error:        stringPointer(message),
		AttemptCount: intPointer(attemptCount),
		CompletedAt:  &completedAt,
	})
	if err == nil {
		metrics.RecordAgentToolCall(toolName, string(ToolRunStatusFailed))
	}
	return err
}

func findEnabledTool(agent *Agent, name string) *Tool {
	if agent == nil {
		return nil
	}
	for i := range agent.Tools {
		if agent.Tools[i].Name == name && agent.Tools[i].Enabled {
			return &agent.Tools[i]
		}
	}
	return nil
}

func stringPointer(value string) *string {
	return &value
}

func intPointer(value int) *int {
	return &value
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
