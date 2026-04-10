package types

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrNoAvailableChannel 无可用渠道
var ErrNoAvailableChannel = errors.New("relay: no available channel")

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
)

func (a APIType) String() string {
	names := [...]string{
		"unknown", "chat", "responses", "realtime", "assistants",
		"threads", "runs", "batch", "batch_files", "fine_tuning",
		"files", "embeddings", "images_generations", "images_edits",
		"images_variations", "videos", "audio_speech", "audio_transcriptions",
		"audio_translations", "moderations", "completions",
	}
	if a < 0 || int(a) >= len(names) {
		return "unknown"
	}
	return names[a]
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
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Provider  string   `json:"provider"` // "openai"
	BaseURL   string   `json:"base_url"`
	APIKey    string   `json:"-"` // 加密存储，不暴露
	Models    []string `json:"models"`
	RPMLimit  int      `json:"rpm_limit"`
	TPMLimit  int      `json:"tpm_limit"`
	CBThreshold int    `json:"cb_threshold"`
	CBTimeout   int    `json:"cb_timeout"`
	HealthCheckStrategy string `json:"health_check_strategy"`
	ProbeModel  string `json:"probe_model"`
	ProbePrompt string `json:"probe_prompt"`
	Strategy   string `json:"strategy"`
	Priority   int    `json:"priority"`
	Enabled    bool    `json:"enabled"`
}

// ModelRoute 模型路由
type ModelRoute struct {
	ID       string `json:"id"`
	Model    string `json:"model"`
	Strategy string `json:"strategy"`
	Channels []RouteChannel `json:"channels"`
}

// RouteChannel 模型-渠道关联
type RouteChannel struct {
	Channel           *Channel `json:"channel"`
	ChannelID        string   `json:"channel_id"`
	Weight           int      `json:"weight"`
	Priority         int      `json:"priority"`
	Enabled          bool     `json:"enabled"`
	Healthy          bool     `json:"healthy"`
	EstimatedCostPer1K float64 `json:"estimated_cost_per_1k"`
}

// ChannelStats 运行时状态（内存）
type ChannelStats struct {
	ChannelID     string    `json:"channel_id"`
	CBState       string    `json:"cb_state"`
	CBFailures    int       `json:"cb_failures"`
	CBLastFailure time.Time `json:"cb_last_failure"`
	CBProbeCount  int       `json:"cb_probe_count"`
	CBHalfOpenReq int       `json:"cb_half_open_req"`
	RPMCurrent    int       `json:"rpm_current"`
	TPMCurrent    int       `json:"tpm_current"`
	RPMLastReset  time.Time `json:"rpm_last_reset"`
	TPMLastReset  time.Time `json:"tpm_last_reset"`
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	LatencySumUs  int64     `json:"latency_sum_us"`
	LatencyCount  int64     `json:"latency_count"`
	LastProbeSuccess time.Time `json:"last_probe_success"`
	LastProbeTime    time.Time `json:"last_probe_time"`
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
	Role      string   `json:"role"`
	Content   string   `json:"content"`
	MediaURLs []string `json:"media_urls,omitempty"`
}

// ProviderRequest 内部标准请求格式
type ProviderRequest struct {
	APIType     APIType   `json:"api_type"`
	Model       string    `json:"model"`
	Headers     http.Header `json:"headers"`
	URL         string    `json:"url"`
	Stream      bool      `json:"stream"`
	Messages    []Message `json:"messages,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Input       string    `json:"input,omitempty"`
	AudioFormat string    `json:"audio_format,omitempty"`
	AudioVoice  string    `json:"audio_voice,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	Prompt      string    `json:"prompt,omitempty"`
	FileURL     string    `json:"file_url,omitempty"`
	Body        []byte    `json:"body,omitempty"`
	RequestID   string    `json:"request_id,omitempty"`
}

// Capabilities 能力声明
type Capabilities struct {
	SupportsChat        bool
	SupportsStreaming   bool
	SupportsEmbeddings  bool
	SupportsImages      bool
	SupportsAudio       bool
	SupportsRealtime    bool
	SupportsAssistants  bool
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
