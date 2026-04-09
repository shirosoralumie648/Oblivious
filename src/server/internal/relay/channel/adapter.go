package channel

import (
	"context"
	"net/http"

	"oblivious/server/internal/relay/types"
)

// Capabilities 能力声明
type Capabilities struct {
	SupportsChat        bool
	SupportsStreaming   bool
	SupportsEmbeddings  bool
	SupportsImages      bool
	SupportsAudio      bool
	SupportsRealtime   bool
	SupportsAssistants bool
}

// Message 内部标准消息格式
type Message struct {
	Role     string   `json:"role"`
	Content  string   `json:"content"`
	MediaURLs []string `json:"media_urls,omitempty"`
}

// ProviderRequest 内部标准请求格式
type ProviderRequest struct {
	APIType     types.APIType `json:"api_type"`
	Model       string        `json:"model"`
	Headers     http.Header   `json:"headers"`
	URL         string        `json:"url"`
	Stream      bool          `json:"stream"`
	Messages    []Message     `json:"messages,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Input       string        `json:"input,omitempty"`        // embeddings / audio
	AudioFormat string        `json:"audio_format,omitempty"`  // speech
	AudioVoice  string        `json:"audio_voice,omitempty"`   // speech
	ImageURL    string        `json:"image_url,omitempty"`     // images edit
	Prompt      string        `json:"prompt,omitempty"`       // legacy completions
	FileURL     string        `json:"file_url,omitempty"`      // fine-tuning / batch
	Body        []byte        `json:"body,omitempty"`         // 原始 body 用于透传
	RequestID   string        `json:"request_id,omitempty"`
}

// ProviderAdapter Provider 适配器接口
type ProviderAdapter interface {
	// 元信息
	Name() string
	Provider() string
	Capabilities() Capabilities

	// 请求构建
	BuildURL(model string, apiType types.APIType) (string, error)
	BuildHeaders(ctx context.Context, model string, apiType types.APIType) (http.Header, error)

	// 请求转换（外部格式 → 内部格式）
	ConvertRequest(req *ProviderRequest) (*ProviderRequest, error)
	// 响应转换（Provider 响应 → 内部格式）
	ConvertResponse(resp []byte, isStream bool) (*types.ProviderResponse, error)

	// HTTP 执行
	DoRequest(ctx context.Context, req *ProviderRequest) (*http.Response, error)

	// 健康检查
	HealthCheck(ctx context.Context) error

	// 错误映射
	MapError(statusCode int, body []byte) *types.ProviderError

	// 用量估算（用于 PreBill）
	EstimateUsage(req *ProviderRequest) *types.Usage
}
