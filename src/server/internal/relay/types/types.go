package types

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
	return &ProviderResponse{
		StatusCode: 200,
		Content:    content,
		Done:       true,
		Usage:      usage,
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(statusCode int, err *ProviderError) *ProviderResponse {
	return &ProviderResponse{
		StatusCode: statusCode,
		Error:      err,
		Done:       true,
	}
}

// Route 定义（用于 Handler 路由注册）
type Route struct {
	Method    string
	APIType   APIType
	Strategy  HandlerStrategy
	Retryable bool
}
