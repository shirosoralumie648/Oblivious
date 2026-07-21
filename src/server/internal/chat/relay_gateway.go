package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	relaytypes "oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

// RelayGateway 通过 Relay 调用 LLM
type RelayGateway struct {
	httpClient   *http.Client
	relayURL     string
	defaultModel string
	readiness    *relayGatewayReadiness
}

const maxRelayStreamLineBytes = 4 * 1024 * 1024

// RelayGatewayOption 配置选项
type RelayGatewayOption func(*RelayGateway)

// RelayGatewayRuntimeOptions is the exact startup-built readiness carrier for
// provider dispatch. It is intentionally separate from HTTP/client settings.
type RelayGatewayRuntimeOptions struct {
	Guard                  releasecontract.Guard
	Authorities            releasecontract.RuntimeAuthorities
	Effects                releasecontract.EffectRegistrar
	SkipEffectRegistration bool
}

type relayGatewayReadiness struct {
	guard       releasecontract.Guard
	authorities releasecontract.RuntimeAuthorities
	chatEffect  releasecontract.CapabilityID
}

func newRelayGatewayReadiness(options RelayGatewayRuntimeOptions, descriptorID, owner string) (*relayGatewayReadiness, error) {
	if options.Guard == nil || options.Effects == nil || !options.Authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "chat.relayGateway"}
	}
	chatEffect, err := options.Authorities.CapabilityBindings.Resolve(releasecontract.EffectChatProvider)
	if err != nil {
		return nil, err
	}
	if !options.SkipEffectRegistration {
		if err := options.Effects.Register(releasecontract.EffectDescriptor{
			ID:           descriptorID,
			CapabilityID: string(chatEffect),
			Boundary:     releasecontract.BoundaryOutbound,
			Owner:        owner,
		}); err != nil {
			return nil, err
		}
	}
	return &relayGatewayReadiness{guard: options.Guard, authorities: options.Authorities, chatEffect: chatEffect}, nil
}

func (r *relayGatewayReadiness) requireDispatch(ctx context.Context, model string) error {
	if r == nil {
		return nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "chat.model"}
	}
	capabilityID, err := r.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, releasecontract.CatalogSubject{
		Kind:    releasecontract.CatalogSubjectModel,
		ID:      model,
		Runtime: releasecontract.CatalogRuntimeServerModel,
	}, releasecontract.BoundaryOutbound)
	if err != nil {
		return err
	}
	if capabilityID != r.chatEffect {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "chat.modelCapability"}
	}
	return r.guard.Require(ctx, string(r.chatEffect), releasecontract.BoundaryOutbound)
}

// WithRelayURL 设置 Relay URL
func WithRelayURL(url string) RelayGatewayOption {
	return func(g *RelayGateway) {
		g.relayURL = strings.TrimRight(url, "/")
	}
}

// WithDefaultModel 设置默认模型
func WithDefaultModel(model string) RelayGatewayOption {
	return func(g *RelayGateway) {
		g.defaultModel = model
	}
}

// WithHTTPClient 设置 HTTP 客户端
func WithHTTPClient(client *http.Client) RelayGatewayOption {
	return func(g *RelayGateway) {
		g.httpClient = client
	}
}

// NewRelayGateway 创建 RelayGateway
func NewRelayGateway(opts ...RelayGatewayOption) *RelayGateway {
	g := &RelayGateway{
		relayURL:     "http://localhost:8080/v1",
		defaultModel: "gpt-4o-mini",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// NewRelayGatewayWithOptions constructs an authority-bound Relay gateway.
func NewRelayGatewayWithOptions(runtime RelayGatewayRuntimeOptions, opts ...RelayGatewayOption) (*RelayGateway, error) {
	readiness, err := newRelayGatewayReadiness(runtime, "chat.provider.dispatch", "chat.RelayGateway")
	if err != nil {
		return nil, err
	}
	gateway := NewRelayGateway(opts...)
	gateway.readiness = readiness
	return gateway, nil
}

// NewReadinessRelayGateway is an explicit alias for runtime composition.
func NewReadinessRelayGateway(runtime RelayGatewayRuntimeOptions, opts ...RelayGatewayOption) (*RelayGateway, error) {
	return NewRelayGatewayWithOptions(runtime, opts...)
}

// GenerateReply 实现 ReplyGenerator 接口
func (g *RelayGateway) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	req := &chatCompletionRequest{
		Model:          selectModelID(config.ModelID, g.defaultModel),
		Messages:       toOpenAIMessages(messages, config),
		Temperature:    config.Temperature,
		ConversationID: config.ConversationID,
	}
	if config.MaxOutputTokens > 0 {
		req.MaxTokens = config.MaxOutputTokens
	}

	resp, err := g.complete(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", ErrModelGatewayUnavailable
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateReplyStream 流式生成回复
func (g *RelayGateway) GenerateReplyStream(ctx context.Context, messages []Message, config ConversationConfig, onChunk func(string) error) error {
	req := &chatCompletionRequest{
		Model:          selectModelID(config.ModelID, g.defaultModel),
		Messages:       toOpenAIMessages(messages, config),
		Temperature:    config.Temperature,
		Stream:         true,
		ConversationID: config.ConversationID,
	}
	if config.MaxOutputTokens > 0 {
		req.MaxTokens = config.MaxOutputTokens
	}

	return g.completeStream(ctx, req, onChunk)
}

// GenerateStructuredReply returns the full assistant payload needed for tool-calling loops.
func (g *RelayGateway) GenerateStructuredReply(ctx context.Context, messages []Message, config ConversationConfig, tools []map[string]any) (*CompletionResponse, error) {
	req := &chatCompletionRequest{
		Model:          selectModelID(config.ModelID, g.defaultModel),
		Messages:       toOpenAIMessages(messages, config),
		Temperature:    config.Temperature,
		Tools:          tools,
		ConversationID: config.ConversationID,
	}
	if config.MaxOutputTokens > 0 {
		req.MaxTokens = config.MaxOutputTokens
	}

	resp, err := g.complete(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, ErrModelGatewayUnavailable
	}

	choice := resp.Choices[0]
	if strings.TrimSpace(choice.Message.Content) == "" && len(choice.Message.ToolCalls) == 0 {
		return nil, ErrModelGatewayUnavailable
	}

	result := &CompletionResponse{
		Content:      choice.Message.Content,
		ToolCalls:    append([]ToolCall(nil), choice.Message.ToolCalls...),
		FinishReason: choice.FinishReason,
	}
	if resp.Usage != nil {
		result.Usage = &CompletionUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
			RecordedByRelay:  true,
		}
	}

	return result, nil
}

// chatCompletionRequest OpenAI Chat Completion 请求
type chatCompletionRequest struct {
	Model          string           `json:"model"`
	Messages       []openAIMessage  `json:"messages"`
	Temperature    float64          `json:"temperature,omitempty"`
	MaxTokens      int              `json:"max_tokens,omitempty"`
	Stream         bool             `json:"stream,omitempty"`
	Tools          []map[string]any `json:"tools,omitempty"`
	ConversationID string           `json:"-"`
}

// chatCompletionResponse OpenAI Chat Completion 响应
type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
		Delta        *struct {
			Content string `json:"content"`
		} `json:"delta,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// complete 发送非流式请求
func (g *RelayGateway) complete(ctx context.Context, req *chatCompletionRequest) (*chatCompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.relayURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyRelayRequestMetadata(httpReq)
	applyRelayConversationMetadata(httpReq, req.ConversationID)

	if g.readiness != nil {
		if err := g.readiness.requireDispatch(ctx, req.Model); err != nil {
			return nil, err
		}
	}
	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("relay returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// completeStream 发送流式请求
func (g *RelayGateway) completeStream(ctx context.Context, req *chatCompletionRequest, onChunk func(string) error) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.relayURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	applyRelayRequestMetadata(httpReq)
	applyRelayConversationMetadata(httpReq, req.ConversationID)

	if g.readiness != nil {
		if err := g.readiness.requireDispatch(ctx, req.Model); err != nil {
			return err
		}
	}
	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("relay returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), maxRelayStreamLineBytes)
	for scanner.Scan() {
		line := scanner.Text()

		// SSE 格式: "data: {...}"
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// 结束标记
		if data == "[DONE]" {
			break
		}

		var chunk chatCompletionResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			content := chunk.Choices[0].Delta.Content
			if content != "" {
				if err := onChunk(content); err != nil {
					return err
				}
			}
		}
	}

	return scanner.Err()
}

// ChatGateway 组合接口，支持流式和非流式
type ChatGateway interface {
	ReplyGenerator
	GenerateReplyStream(ctx context.Context, messages []Message, config ConversationConfig, onChunk func(string) error) error
}

// Ensure RelayGateway implements ChatGateway
var _ ChatGateway = (*RelayGateway)(nil)

// Ensure RelayGateway implements StructuredReplyGenerator
var _ StructuredReplyGenerator = (*RelayGateway)(nil)

// CompositeGateway 组合多个 Gateway，支持回退
type CompositeGateway struct {
	primary   ChatGateway
	fallback  ReplyGenerator
	readiness *relayGatewayReadiness
	mu        sync.Mutex
	lastError error
}

// NewCompositeGateway 创建组合 Gateway
func NewCompositeGateway(primary ChatGateway, fallback ReplyGenerator) *CompositeGateway {
	return &CompositeGateway{
		primary:  primary,
		fallback: fallback,
	}
}

// NewCompositeGatewayWithOptions binds both primary and fallback dispatch to
// the same current startup authority. The primary gateway may already carry a
// readiness instance; this constructor still owns the fallback boundary.
func NewCompositeGatewayWithOptions(primary ChatGateway, fallback ReplyGenerator, runtime RelayGatewayRuntimeOptions) (*CompositeGateway, error) {
	readiness, err := newRelayGatewayReadiness(runtime, "chat.provider.fallback", "chat.CompositeGateway")
	if err != nil {
		return nil, err
	}
	return &CompositeGateway{primary: primary, fallback: fallback, readiness: readiness}, nil
}

// GenerateReply 实现 ReplyGenerator 接口
func (g *CompositeGateway) GenerateReply(ctx context.Context, messages []Message, config ConversationConfig) (string, error) {
	reply, err := g.primary.GenerateReply(ctx, messages, config)
	if err != nil {
		g.mu.Lock()
		g.lastError = err
		g.mu.Unlock()

		// 如果 Relay 不可用，尝试回退
		if g.fallback != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return "", err
			}
			// 检查是否是可回退的错误类型
			if errors.Is(err, ErrModelGatewayUnavailable) ||
				strings.Contains(err.Error(), "relay returned status") {
				if g.readiness != nil {
					if authErr := g.readiness.requireDispatch(ctx, selectModelID(config.ModelID, "")); authErr != nil {
						return "", authErr
					}
				}
				return g.fallback.GenerateReply(ctx, messages, config)
			}
		}
		return "", err
	}
	return reply, nil
}

// GenerateReplyStream 流式生成
func (g *CompositeGateway) GenerateReplyStream(ctx context.Context, messages []Message, config ConversationConfig, onChunk func(string) error) error {
	return g.primary.GenerateReplyStream(ctx, messages, config, onChunk)
}

// GenerateStructuredReply delegates to the primary gateway if it supports
// structured replies; otherwise it wraps a plain-text reply as a fallback.
func (g *CompositeGateway) GenerateStructuredReply(ctx context.Context, messages []Message, config ConversationConfig, tools []map[string]any) (*CompletionResponse, error) {
	if sg, ok := g.primary.(StructuredReplyGenerator); ok {
		return sg.GenerateStructuredReply(ctx, messages, config, tools)
	}
	content, err := g.primary.GenerateReply(ctx, messages, config)
	if err != nil {
		return nil, err
	}
	return &CompletionResponse{
		Content:      content,
		FinishReason: "stop",
	}, nil
}

// LastError 返回最后一次错误
func (g *CompositeGateway) LastError() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.lastError
}

func applyRelayRequestMetadata(req *http.Request) {
	metadata, ok := RelayRequestMetadataFromContext(req.Context())
	if !ok {
		return
	}
	if metadata.UserID != "" {
		req.Header.Set(relaytypes.HeaderInternalUserID, metadata.UserID)
		req.Header.Set("X-Oblivious-Internal-User-ID", metadata.UserID)
	}
	if metadata.WorkspaceID != "" {
		req.Header.Set("X-Oblivious-Internal-Workspace-ID", metadata.WorkspaceID)
	}
	if metadata.OrganizationID != "" {
		req.Header.Set(relaytypes.HeaderInternalOrganization, metadata.OrganizationID)
		req.Header.Set("X-Oblivious-Internal-Organization-ID", metadata.OrganizationID)
	}
	if metadata.UserGroup != "" {
		req.Header.Set(relaytypes.HeaderInternalUserGroup, metadata.UserGroup)
		req.Header.Set("X-Oblivious-Internal-User-Group", metadata.UserGroup)
	}
	if metadata.RequestID != "" {
		req.Header.Set("X-Request-ID", metadata.RequestID)
	}
	if metadata.FeatureType != "" {
		req.Header.Set(relaytypes.HeaderInternalFeatureType, metadata.FeatureType)
		req.Header.Set("X-Oblivious-Internal-Feature-Type", metadata.FeatureType)
	}

	// Set internal auth token so the Relay handler can verify this is
	// trusted server-to-server traffic and not a spoofed external request.
	internalAuth := os.Getenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN")
	if internalAuth == "" {
		internalAuth = "oblivious-internal-v1"
	}
	req.Header.Set("X-Oblivious-Internal-Auth", internalAuth)
}

func applyRelayConversationMetadata(req *http.Request, conversationID string) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return
	}
	req.Header.Set(relaytypes.HeaderInternalConversation, conversationID)
	req.Header.Set("X-Oblivious-Internal-Conversation-ID", conversationID)
}
