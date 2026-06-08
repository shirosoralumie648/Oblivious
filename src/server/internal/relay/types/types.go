package types

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	relaycache "oblivious/server/internal/relay/cache"
)

// ErrNoAvailableChannel 无可用渠道
var ErrNoAvailableChannel = errors.New("relay: no available channel")

var (
	ErrRelayAPITokenInvalid       = errors.New("relay API token invalid")
	ErrRelayAPITokenRevoked       = errors.New("relay API token revoked")
	ErrRelayAPITokenExpired       = errors.New("relay API token expired")
	ErrRelayAPITokenModelDenied   = errors.New("relay API token model denied")
	ErrRelayAPITokenQuotaExceeded = errors.New("relay API token quota exceeded")
)

const (
	HeaderInternalAuth         = "X-Oblivious-Internal-Auth"
	HeaderInternalUserID       = "X-Oblivious-User-ID"
	HeaderInternalOrganization = "X-Oblivious-Organization-ID"
	HeaderInternalUserGroup    = "X-Oblivious-User-Group"
	HeaderInternalConversation = "X-Oblivious-Conversation-ID"
	HeaderInternalFeatureType  = "X-Oblivious-Feature-Type"
	HeaderRequestID            = "X-Request-ID"

	SharedInternalToken = "oblivious-internal"
)

type trustedContextKey string

const (
	trustedUserIDKey         trustedContextKey = "relay_trusted_user_id"
	trustedOrganizationIDKey trustedContextKey = "relay_trusted_organization_id"
	trustedAPITokenIDKey     trustedContextKey = "relay_trusted_api_token_id"
	trustedRequestIDKey      trustedContextKey = "relay_trusted_request_id"
	trustedUserGroupKey      trustedContextKey = "relay_trusted_user_group"
	trustedConversationIDKey trustedContextKey = "relay_trusted_conversation_id"
	trustedFeatureTypeKey    trustedContextKey = "relay_trusted_feature_type"
	semanticCacheRequestKey  trustedContextKey = "relay_semantic_cache_request"
)

// APIType 枚举（22 种 OpenAI API 类型）
type APIType int

const (
	APITypeUnknown APIType = iota
	APITypeChat
	APITypeResponses
	APITypeRealtime
	APITypeAssistants
	APITypeThreads
	APITypeRuns
	APITypeBatch
	APITypeBatchFiles
	APITypeFineTuning
	APITypeFiles
	APITypeEmbeddings
	APITypeImageGen
	APITypeImageEdit
	APITypeImageVar
	APITypeVideos
	APITypeAudioSpeech
	APITypeAudioSTT
	APITypeAudioTranslate
	APITypeModeration
	APITypeCompletions
	APITypeModels
)

func (a APIType) String() string {
	names := [...]string{
		"unknown", "chat", "responses", "realtime", "assistants",
		"threads", "runs", "batch", "batch_files", "fine_tuning",
		"files", "embeddings", "images_generations", "images_edits",
		"images_variations", "videos", "audio_speech", "audio_transcriptions",
		"audio_translations", "moderations", "completions", "models",
	}
	if a < 0 || int(a) >= len(names) {
		return "unknown"
	}
	return names[a]
}

func WithTrustedUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, trustedUserIDKey, userID)
}

func TrustedUserIDFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedUserIDKey)
}

func WithTrustedOrganizationID(ctx context.Context, organizationID string) context.Context {
	return context.WithValue(ctx, trustedOrganizationIDKey, organizationID)
}

func TrustedOrganizationIDFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedOrganizationIDKey)
}

func WithTrustedAPITokenID(ctx context.Context, tokenID string) context.Context {
	return context.WithValue(ctx, trustedAPITokenIDKey, tokenID)
}

func TrustedAPITokenIDFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedAPITokenIDKey)
}

func WithTrustedRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, trustedRequestIDKey, requestID)
}

func TrustedRequestIDFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedRequestIDKey)
}

func WithTrustedUserGroup(ctx context.Context, userGroup string) context.Context {
	return context.WithValue(ctx, trustedUserGroupKey, userGroup)
}

func TrustedUserGroupFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedUserGroupKey)
}

func WithTrustedConversationID(ctx context.Context, conversationID string) context.Context {
	return context.WithValue(ctx, trustedConversationIDKey, conversationID)
}

func TrustedConversationIDFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedConversationIDKey)
}

func WithTrustedFeatureType(ctx context.Context, featureType string) context.Context {
	return context.WithValue(ctx, trustedFeatureTypeKey, featureType)
}

func TrustedFeatureTypeFromContext(ctx context.Context) (string, bool) {
	return trustedStringFromContext(ctx, trustedFeatureTypeKey)
}

type SemanticCacheRequest = relaycache.SemanticCacheRequest

func WithSemanticCacheRequest(ctx context.Context, req SemanticCacheRequest) context.Context {
	return context.WithValue(ctx, semanticCacheRequestKey, req)
}

func SemanticCacheRequestFromContext(ctx context.Context) (SemanticCacheRequest, bool) {
	req, ok := ctx.Value(semanticCacheRequestKey).(SemanticCacheRequest)
	return req, ok
}

type RelayAPITokenIdentity struct {
	TokenID        string
	UserID         string
	OrganizationID string
	UserGroup      string
}

type RelayAPITokenAuthenticator interface {
	AuthenticateRelayAPIToken(ctx context.Context, rawToken, model string, apiType APIType) (RelayAPITokenIdentity, error)
}

func trustedStringFromContext(ctx context.Context, key trustedContextKey) (string, bool) {
	value, ok := ctx.Value(key).(string)
	return value, ok && value != ""
}

// HandlerStrategy 处理器策略
type HandlerStrategy int

const (
	StrategyNative HandlerStrategy = iota
	StrategyPassthrough
	StrategyFileProxy
)

// UsageDimension 计费维度
type UsageDimension string

const (
	DimPromptTokens     UsageDimension = "prompt_tokens"
	DimCompletionTokens UsageDimension = "completion_tokens"
	DimTotalTokens      UsageDimension = "total_tokens"
	DimImageCount       UsageDimension = "image_count"
	DimVideoCount       UsageDimension = "video_count"
	DimAudioSeconds     UsageDimension = "audio_seconds"
	DimStorageBytes     UsageDimension = "storage_bytes"
	DimTrainingTokens   UsageDimension = "training_tokens"
)

// Usage 用量结构
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	ImageCount       int     `json:"image_count,omitempty"`
	VideoCount       int     `json:"video_count,omitempty"`
	AudioSeconds     float64 `json:"audio_seconds,omitempty"`
	StorageBytes     int64   `json:"storage_bytes,omitempty"`
	TrainingTokens   int     `json:"training_tokens,omitempty"`
}

// ProviderError Provider 错误
type ProviderError struct {
	Code       string
	Message    string
	StatusCode int
	Retryable  bool
}

func (e *ProviderError) Error() string {
	return e.Message
}

// ProviderResponse Provider 原生响应
type ProviderResponse struct {
	StatusCode int
	Headers    http.Header
	Content    []byte
	Done       bool
	Usage      *Usage
	Error      *ProviderError
	StreamCB   func(chunk []byte) error
}

// NewOKResponse 创建成功响应
func NewOKResponse(content []byte, usage *Usage) *ProviderResponse {
	return &ProviderResponse{StatusCode: 200, Content: content, Done: true, Usage: usage}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(statusCode int, err *ProviderError) *ProviderResponse {
	return &ProviderResponse{StatusCode: statusCode, Error: err, Done: true}
}

// Route 定义（用于 Handler 路由注册）
type Route struct {
	Method    string
	APIType   APIType
	Strategy  HandlerStrategy
	Retryable bool
}

// Handler 接口（由 handler 包实现）
type Handler interface {
	Handle(c *gin.Context) error
	HandleStream(c *gin.Context) error
}

// Channel 渠道配置
type Channel struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Provider            string   `json:"provider"` // "openai"
	BaseURL             string   `json:"base_url"`
	APIKey              string   `json:"-"` // 加密存储，不暴露
	Models              []string `json:"models"`
	Groups              []string `json:"groups,omitempty"`
	RPMLimit            int      `json:"rpm_limit"`
	TPMLimit            int      `json:"tpm_limit"`
	CBThreshold         int      `json:"cb_threshold"`
	CBTimeout           int      `json:"cb_timeout"`
	HealthCheckStrategy string   `json:"health_check_strategy"`
	ProbeModel          string   `json:"probe_model"`
	ProbePrompt         string   `json:"probe_prompt"`
	Strategy            string   `json:"strategy"`
	Priority            int      `json:"priority"`
	Weight              int      `json:"weight"`
	EstimatedCostPer1K  float64  `json:"estimated_cost_per_1k"`
	CostMultiplier      float64  `json:"cost_multiplier"`
	Enabled             bool     `json:"enabled"`
}

// ModelRoute 模型路由
type ModelRoute struct {
	ID       string         `json:"id"`
	Model    string         `json:"model"`
	Strategy string         `json:"strategy"`
	Channels []RouteChannel `json:"channels"`
}

// RouteChannel 模型-渠道关联
type RouteChannel struct {
	Channel            *Channel `json:"channel"`
	ChannelID          string   `json:"channel_id"`
	Weight             int      `json:"weight"`
	Priority           int      `json:"priority"`
	Enabled            bool     `json:"enabled"`
	Healthy            bool     `json:"healthy"`
	EstimatedCostPer1K float64  `json:"estimated_cost_per_1k"`
	CostMultiplier     float64  `json:"cost_multiplier"`
}

// ChannelStats 运行时状态（内存）
type ChannelStats struct {
	ChannelID        string    `json:"channel_id"`
	CBState          string    `json:"cb_state"`
	CBFailures       int       `json:"cb_failures"`
	CBLastFailure    time.Time `json:"cb_last_failure"`
	CBProbeCount     int       `json:"cb_probe_count"`
	CBHalfOpenReq    int       `json:"cb_half_open_req"`
	RPMCurrent       int       `json:"rpm_current"`
	TPMCurrent       int       `json:"tpm_current"`
	RPMLastReset     time.Time `json:"rpm_last_reset"`
	TPMLastReset     time.Time `json:"tpm_last_reset"`
	TotalRequests    int64     `json:"total_requests"`
	SuccessCount     int64     `json:"success_count"`
	FailureCount     int64     `json:"failure_count"`
	LatencySumUs     int64     `json:"latency_sum_us"`
	LatencyCount     int64     `json:"latency_count"`
	Invalid          bool      `json:"invalid"`
	Forbidden        bool      `json:"forbidden"`
	LastProbeSuccess time.Time `json:"last_probe_success"`
	LastProbeTime    time.Time `json:"last_probe_time"`
	RateLimitedUntil time.Time `json:"rate_limited_until,omitempty"`

	AffinityConversationCount int `json:"affinity_conversation_count"`

	RuntimeSamples []ChannelRuntimeSample `json:"runtime_samples,omitempty"`
}

type ChannelRuntimeSample struct {
	At        time.Time `json:"at"`
	Success   bool      `json:"success"`
	LatencyUs int64     `json:"latency_us"`
}

// ChannelPoolInterface 渠道池接口（由 relay.ChannelPool 实现）
type ChannelPoolInterface interface {
	GetChannel(id string) (*Channel, bool)
	GetChannelsByModel(model string) []*RouteChannel
	GetStats(channelID string) (*ChannelStats, bool)
	UpdateChannel(ch *Channel)
	UpdateRoute(route *ModelRoute)
	ListChannels() []*Channel
	SetChannelHealthy(channelID string, healthy bool)
	GetAllStats() map[string]*ChannelStats
}

// RouterInterface is the minimal interface handlers need to route requests.
// Defined in types package to avoid import cycles between relay and handler packages.
type RouterInterface interface {
	Route(ctx context.Context, apiType string, fn func(ch *RouteChannel) (*ProviderResponse, error)) (*ProviderResponse, error)
	RouteWithBilling(ctx context.Context, apiType APIType, model, channelID, idempotencyKey string, usage *Usage, fn func(ch *RouteChannel) (*ProviderResponse, error)) (*ProviderResponse, error)
	RecordChannelSuccess(channelID string)
	RecordChannelFailure(channelID string)
}

// Message 内部标准消息格式
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	MediaURLs  []string   `json:"media_urls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall is the OpenAI-compatible function call shape carried through Relay.
type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function ToolFunction `json:"function,omitempty"`
}

type ToolFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ProviderRequest 内部标准请求格式
type ProviderRequest struct {
	APIType     APIType          `json:"api_type"`
	Model       string           `json:"model"`
	Headers     http.Header      `json:"headers"`
	URL         string           `json:"url"`
	Stream      bool             `json:"stream"`
	Messages    []Message        `json:"messages,omitempty"`
	Tools       []map[string]any `json:"tools,omitempty"`
	ToolChoice  any              `json:"tool_choice,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Input       string           `json:"input,omitempty"`
	AudioFormat string           `json:"audio_format,omitempty"`
	AudioVoice  string           `json:"audio_voice,omitempty"`
	ImageURL    string           `json:"image_url,omitempty"`
	Prompt      string           `json:"prompt,omitempty"`
	FileURL     string           `json:"file_url,omitempty"`
	Body        []byte           `json:"body,omitempty"`
	RequestID   string           `json:"request_id,omitempty"`
}

// Capabilities 能力声明
type Capabilities struct {
	SupportsChat       bool
	SupportsStreaming  bool
	SupportsEmbeddings bool
	SupportsImages     bool
	SupportsAudio      bool
	SupportsRealtime   bool
	SupportsAssistants bool
}

// HealthScore 健康分（0-100）
type HealthScore struct {
	ChannelID     string    `json:"channel_id"`
	Score         float64   `json:"score"`          // 0-100
	AvgLatencyMs  float64   `json:"avg_latency_ms"` // 移动平均延迟
	ErrorRate     float64   `json:"error_rate"`     // 0-1 错误率
	TotalProbes   int64     `json:"total_probes"`
	FailedProbes  int64     `json:"failed_probes"`
	LastProbeTime time.Time `json:"last_probe_time"`
	LastHealthy   bool      `json:"last_healthy"`
	RemovedAt     time.Time `json:"removed_at,omitempty"` // 自动摘除时间
}

// WeightedChannel 带动态权重的渠道
type WeightedChannel struct {
	Channel       *Channel `json:"channel"`
	ChannelID     string   `json:"channel_id"`
	StaticWeight  int      `json:"static_weight"`  // 配置权重
	DynamicWeight float64  `json:"dynamic_weight"` // 动态计算权重
	Healthy       bool     `json:"healthy"`
	Enabled       bool     `json:"enabled"`
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key       string    `json:"key"` // 向量 hash 或 query hash
	Query     string    `json:"query"`
	Response  []byte    `json:"response"`
	IsPublic  bool      `json:"is_public"` // true=全局共享，false=组织私有
	OrgID     string    `json:"org_id"`    // 组织隔离
	Model     string    `json:"model"`
	HitCount  int64     `json:"hit_count"`
	Embedding []float32 `json:"embedding"` // 语义向量
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AffinityMapping 渠道亲和性映射
type AffinityMapping struct {
	ConversationID string    `json:"conversation_id"`
	ChannelID      string    `json:"channel_id"`
	OrgID          string    `json:"org_id"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsedAt     time.Time `json:"last_used_at"`
	FailoverCount  int       `json:"failover_count"` // 故障切换次数
}

// RateLimitInfo 限流信息
type RateLimitInfo struct {
	ChannelID     string    `json:"channel_id"`
	RPMUsed       int       `json:"rpm_used"`
	RPMLimit      int       `json:"rpm_limit"`
	TPMUsed       int       `json:"tpm_used"`
	TPMLimit      int       `json:"tpm_limit"`
	WindowStart   time.Time `json:"window_start"`
	RetryAfterSec int       `json:"retry_after_sec"` // 从 429 解析
}

// CacheLevel 缓存级别
type CacheLevel int

const (
	CacheLevelNone    CacheLevel = iota
	CacheLevelPrivate            // 组织私有缓存
	CacheLevelPublic             // 全局公共缓存（无敏感信息）
)

func (cl CacheLevel) String() string {
	switch cl {
	case CacheLevelPrivate:
		return "private"
	case CacheLevelPublic:
		return "public"
	default:
		return "none"
	}
}

// ProviderAdapter Provider 适配器接口
type ProviderAdapter interface {
	// 元信息
	Name() string
	Provider() string
	Capabilities() Capabilities

	// 请求构建
	BuildURL(model string, apiType APIType) (string, error)
	BuildHeaders(ctx context.Context, model string, apiType APIType) (http.Header, error)

	// 请求转换（外部格式 → 内部格式）
	ConvertRequest(req *ProviderRequest) (*ProviderRequest, error)
	// 响应转换（Provider 响应 → 内部格式）
	ConvertResponse(resp []byte, isStream bool) (*ProviderResponse, error)

	// HTTP 执行
	DoRequest(ctx context.Context, req *ProviderRequest) (*http.Response, error)

	// 健康检查
	HealthCheck(ctx context.Context) error

	// 错误映射
	MapError(statusCode int, body []byte) *ProviderError

	// 用量估算（用于 PreBill）
	EstimateUsage(req *ProviderRequest) *Usage
}
