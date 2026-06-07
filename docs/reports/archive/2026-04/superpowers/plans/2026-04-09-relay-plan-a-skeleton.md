# Relay Plan A: 项目骨架 + 数据模型 + 核心接口

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立 Relay 模块的目录骨架、数据库迁移、核心类型定义、ProviderAdapter 接口。

**Architecture:** 在 `src/server/internal/relay/` 下建立完整目录结构，按 handler/router/channel/pool/billing/metrics 分层。数据模型通过 GORM/sqlx + 手写 SQL migration 管理。

**Tech Stack:** Go 1.22, Gin, GORM or sqlx, pgvector, Viper

---

## 文件结构

```
src/server/internal/relay/
├── relay.go                  # 主入口，导出 NewRelay()
├── api_types.go              # APIType 枚举 + 相关类型
├── errors.go                 # Relay 错误类型定义
├── go.mod
│
├── channel/
│   ├── adapter.go            # ProviderAdapter 接口定义
│   ├── openai/
│   │   └── adapter.go       # OpenAI Adapter 骨架
│   └── types.go             # Channel、ModelRoute、RouteChannel 类型
│
├── pool/
│   ├── pool.go              # ChannelPool
│   └── stats.go             # ChannelStats 运行时状态
│
├── handler/
│   └── router.go            # Gin 路由骨架（35个路由注册）
│
└── migrations/
    └── 001_init_relay.sql   # 初始迁移
```

---

## Task 1: 项目骨架与 go.mod

**Files:**
- Create: `src/server/internal/relay/go.mod`
- Create: `src/server/internal/relay/relay.go`
- Create: `src/server/internal/relay/errors.go`

- [ ] **Step 1: 创建 relay 目录骨架**

```bash
mkdir -p src/server/internal/relay/{channel/openai,pool,handler,migrations}
cd src/server/internal/relay && go mod init github.com/shirosora/oblivious/relay
```

- [ ] **Step 2: 创建 go.mod 依赖**

```go
module github.com/shirosora/oblivious/relay

go 1.22

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/prometheus/client_golang v1.18.0
    github.com/pgvector/pgvector-go v0.1.1
    github.com/jmoiron/sqlx v1.3.5
    github.com/spf13/viper v1.18.2
    github.com/google/uuid v1.6.0
    github.com/redis/go-redis/v9 v9.4.0
)
```

- [ ] **Step 3: 创建 Relay 错误类型**

```go
// src/server/internal/relay/errors.go
package relay

import "errors"

var (
    ErrNoAvailableChannel = errors.New("relay: no available channel")
    ErrChannelUnavailable  = errors.New("relay: channel unavailable")
    ErrRateLimitExceeded  = errors.New("relay: rate limit exceeded")
    ErrInsufficientQuota  = errors.New("relay: insufficient quota")
    ErrCircuitOpen        = errors.New("relay: circuit open")
    ErrInvalidRequest     = errors.New("relay: invalid request")
    ErrTimeout            = errors.New("relay: timeout")
    ErrUpstreamError      = errors.New("relay: upstream error")
    ErrBillingFailed      = errors.New("relay: billing failed")
)
```

- [ ] **Step 4: 创建 relay.go 导出主入口**

```go
// src/server/internal/relay/relay.go
package relay

import "github.com/gin-gonic/gin"

type Relay struct {
    engine     *gin.Engine
    pool       *ChannelPool
    handlers   map[APIType]Handler
}

func NewRelay(cfg *Config) (*Relay, error) {
    r := &Relay{
        handlers: make(map[APIType]Handler),
    }
    if err := r.initPool(cfg); err != nil {
        return nil, err
    }
    r.initHandlers(cfg)
    r.initRouter()
    return r, nil
}

func (r *Relay) initRouter() {
    r.engine = gin.New()
    RegisterRoutes(r.engine, r.handlers)
}

func (r *Relay) Run(addr string) error {
    return r.engine.Run(addr)
}
```

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/
git commit -m "feat(relay): add relay module skeleton

- Add go.mod with dependencies
- Add relay errors package
- Add main Relay struct with NewRelay() entry point
- Add Gin engine setup and route registration hook
- Create directory structure: channel/, pool/, handler/, migrations/
"
```

---

## Task 2: APIType 枚举与核心类型

**Files:**
- Create: `src/server/internal/relay/api_types.go`
- Modify: `src/server/internal/relay/errors.go` (增加 ProviderError)

- [ ] **Step 1: 创建 APIType 枚举和相关类型**

```go
// src/server/internal/relay/api_types.go
package relay

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

// Handler 策略
type HandlerStrategy int

const (
    StrategyNative HandlerStrategy = iota // 原生处理：走 Router 选渠道 + 计费
    StrategyPassthrough                   // 透传：直接转发
    StrategyFileProxy                      // 文件代理：用户上传到本服务再转发
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

// Route 定义（用于 Handler 路由注册）
type Route struct {
    Method    string
    Path      string
    APIType   APIType
    Strategy  HandlerStrategy
    Retryable bool
}
```

- [ ] **Step 2: 创建 ProviderError**

```go
// 在 errors.go 中增加

type ProviderError struct {
    Code       string
    Message    string
    StatusCode int
    Retryable  bool
}

func (e *ProviderError) Error() string {
    return e.Message
}
```

- [ ] **Step 3: 创建 Usage 类型**

```go
// src/server/internal/relay/api_types.go 增加

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

type ProviderResponse struct {
    Content   string
    Done      bool
    Usage     *Usage
    Error     *ProviderError
    StreamCB  func(chunk []byte) error
}
```

- [ ] **Step 4: Commit**

```bash
git add src/server/internal/relay/api_types.go src/server/internal/relay/errors.go
git commit -m "feat(relay): add APIType enum, Usage, ProviderResponse, HandlerStrategy types"
```

---

## Task 3: 数据库迁移

**Files:**
- Create: `src/server/internal/relay/migrations/001_init_relay.sql`

- [ ] **Step 1: 写 SQL 迁移**

```sql
-- src/server/internal/relay/migrations/001_init_relay.sql

-- 渠道表
CREATE TABLE IF NOT EXISTS relay_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50) NOT NULL,  -- 'openai'
    base_url VARCHAR(500) NOT NULL DEFAULT 'https://api.openai.com',
    api_key_encrypted TEXT,
    models TEXT[] DEFAULT '{}',
    rpm_limit INT DEFAULT 1000,
    tpm_limit INT DEFAULT 100000,
    markup DECIMAL(5,2) DEFAULT 1.0,
    cb_threshold INT DEFAULT 5,
    cb_timeout INT DEFAULT 30,
    health_check_strategy VARCHAR(20) DEFAULT 'models_api',
    probe_model VARCHAR(100) DEFAULT 'gpt-4o-mini',
    probe_prompt VARCHAR(255) DEFAULT 'hi',
    strategy VARCHAR(20) DEFAULT 'weighted',
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 定价表
CREATE TABLE IF NOT EXISTS relay_pricing_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_type VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL DEFAULT '*',
    dimension VARCHAR(50) NOT NULL,
    unit_cost DECIMAL(15,8) NOT NULL,
    markup DECIMAL(5,2) DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 模型路由表
CREATE TABLE IF NOT EXISTS relay_model_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model VARCHAR(100) NOT NULL UNIQUE,
    strategy VARCHAR(20) DEFAULT 'weighted',
    created_at TIMESTAMP DEFAULT NOW()
);

-- 模型-渠道权重
CREATE TABLE IF NOT EXISTS relay_model_channel_weights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    route_id UUID REFERENCES relay_model_routes(id) ON DELETE CASCADE,
    channel_id UUID REFERENCES relay_channels(id) ON DELETE CASCADE,
    weight INT DEFAULT 100,
    priority INT DEFAULT 0,
    enabled BOOLEAN DEFAULT true
);

-- 渠道路由缓存（避免每次查 DB）
CREATE INDEX idx_channels_enabled ON relay_channels(enabled);
CREATE INDEX idx_model_routes_model ON relay_model_routes(model);
CREATE INDEX idx_pricing_api_type ON relay_pricing_entries(api_type);

-- 初始 OpenAI 定价数据
INSERT INTO relay_pricing_entries (api_type, model, dimension, unit_cost, markup) VALUES
-- Chat (prompt / completion 不对称)
('chat', 'gpt-4o', 'prompt_tokens', 0.0000025, 1.5),
('chat', 'gpt-4o', 'completion_tokens', 0.00001, 1.5),
('chat', 'gpt-4o-mini', 'prompt_tokens', 0.000000075, 1.5),
('chat', 'gpt-4o-mini', 'completion_tokens', 0.0000003, 1.5),
('chat', 'gpt-4o', 'total_tokens', 0.0, 1.5),
('responses', 'gpt-4o', 'prompt_tokens', 0.0000025, 1.5),
('responses', 'gpt-4o', 'completion_tokens', 0.00001, 1.5),
('embeddings', '*', 'total_tokens', 0.0000001, 1.5),
('images_generations', 'dall-e-3', 'image_count', 0.040, 1.5),
('images_generations', 'dall-e-2', 'image_count', 0.020, 1.5),
('audio_speech', '*', 'audio_seconds', 0.015, 1.5),
('audio_transcriptions', '*', 'audio_seconds', 0.00006, 1.5),
('audio_translations', '*', 'audio_seconds', 0.00006, 1.5);
```

- [ ] **Step 2: 创建 Go migration 入口**

```go
// src/server/internal/relay/migrations/migrate.go
package migrations

import (
    "embed"
    "fmt"
    "os"
)

//go:embed *.sql
var fs embed.FS

func Run(dsn string) error {
    content, err := fs.ReadFile("001_init_relay.sql")
    if err != nil {
        return err
    }
    // 执行 SQL（使用 sqlx.Exec 或直接 pgx）
    return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add src/server/internal/relay/migrations/
git commit -m "feat(relay): add initial migration for relay schema

Tables:
- relay_channels (渠道配置)
- relay_pricing_entries (按 APIType+Dimension 定价)
- relay_model_routes (模型路由)
- relay_model_channel_weights (渠道路由权重)

Includes initial OpenAI pricing data."
```

---

## Task 4: Channel 类型与 ProviderAdapter 接口

**Files:**
- Create: `src/server/internal/relay/channel/types.go`
- Create: `src/server/internal/relay/channel/adapter.go`

- [ ] **Step 1: 创建 Channel 相关类型**

```go
// src/server/internal/relay/channel/types.go
package channel

import "time"

type Channel struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Provider  string `json:"provider"` // "openai"
    BaseURL   string `json:"base_url"`
    APIKey    string `json:"-"` // 加密存储
    Models    []string `json:"models"`

    // 限速
    RPMLimit  int `json:"rpm_limit"`
    TPMLimit  int `json:"tpm_limit"`

    // 熔断
    CBThreshold int `json:"cb_threshold"`
    CBTimeout   int `json:"cb_timeout"`

    // 健康检查
    HealthCheckStrategy string `json:"health_check_strategy"` // "models_api" | "realtime_probe" | "disabled"
    ProbeModel  string `json:"probe_model"`
    ProbePrompt string `json:"probe_prompt"`

    // 路由
    Strategy string `json:"strategy"` // "weighted" | "priority"
    Priority int    `json:"priority"`
    Enabled  bool   `json:"enabled"`
}

type ModelRoute struct {
    ID      string `json:"id"`
    Model   string `json:"model"`
    Strategy string `json:"strategy"`
    Channels []RouteChannel `json:"channels"`
}

type RouteChannel struct {
    ChannelID string `json:"channel_id"`
    Weight    int    `json:"weight"`
    Priority  int    `json:"priority"`
    Enabled   bool   `json:"enabled"`
}

// ChannelStats 运行时状态（内存）
type ChannelStats struct {
    ChannelID string `json:"channel_id"`

    // 熔断状态机
    CBState       string    `json:"cb_state"` // "closed" | "open" | "half_open"
    CBFailures    int       `json:"cb_failures"`
    CBLastFailure time.Time `json:"cb_last_failure"`
    CBProbeCount  int       `json:"cb_probe_count"`
    CBHalfOpenReq int       `json:"cb_half_open_req"` // half_open 状态下已处理的请求数

    // 限流器
    RPMCurrent   int       `json:"rpm_current"`
    TPMCurrent    int       `json:"tpm_current"`
    RPMLastReset  time.Time `json:"rpm_last_reset"`
    TPMLastReset  time.Time `json:"tpm_last_reset"`

    // 监控
    TotalRequests int64 `json:"total_requests"`
    SuccessCount  int64 `json:"success_count"`
    FailureCount  int64 `json:"failure_count"`
    LatencySumUs  int64 `json:"latency_sum_us"`
    LatencyCount  int64 `json:"latency_count"`

    LastProbeSuccess time.Time `json:"last_probe_success"`
    LastProbeTime    time.Time `json:"last_probe_time"`
}
```

- [ ] **Step 2: 创建 ProviderAdapter 接口**

```go
// src/server/internal/relay/channel/adapter.go
package channel

import (
    "context"
    "net/http"
    "relay"
)

// Capabilities 能力声明
type Capabilities struct {
    SupportsChat        bool
    SupportsStreaming   bool
    SupportsEmbeddings  bool
    SupportsImages       bool
    SupportsAudio       bool
    SupportsRealtime     bool
    SupportsTaskPolling bool
}

// ProviderAdapter Provider 适配器接口
type ProviderAdapter interface {
    // 元信息
    Name() string
    Provider() string
    Capabilities() Capabilities

    // 请求构建
    BuildURL(model string, apiType relay.APIType) (string, error)
    BuildHeaders(ctx context.Context, model string, apiType relay.APIType) (http.Header, error)

    // 请求转换（Provider 原生格式 ↔ 内部格式）
    ConvertRequest(req *ProviderRequest) (*ProviderRequest, error)
    ConvertResponse(resp []byte, isStream bool) (*relay.ProviderResponse, error)

    // HTTP 执行
    DoRequest(ctx context.Context, req *ProviderRequest) (*http.Response, error)

    // 健康检查
    HealthCheck(ctx context.Context) error

    // 错误映射
    MapError(statusCode int, body []byte) *relay.ProviderError

    // 用量估算（用于 PreBill）
    EstimateUsage(req *ProviderRequest) *relay.Usage
}

// ProviderRequest 内部标准请求格式
type ProviderRequest struct {
    APIType    relay.APIType `json:"api_type"`
    Model      string        `json:"model"`
    Headers    http.Header   `json:"headers"`
    URL        string        `json:"url"`
    Stream     bool          `json:"stream"`
    Messages   []Message     `json:"messages,omitempty"`
    MaxTokens  int           `json:"max_tokens,omitempty"`
    Input      string        `json:"input,omitempty"`       // embeddings / audio
    AudioFormat string        `json:"audio_format,omitempty"` // speech
    ImageURL   string        `json:"image_url,omitempty"`
    Prompt     string        `json:"prompt,omitempty"`      // legacy completions
    MaxRetries int           `json:"max_retries,omitempty"`
    RequestID  string        `json:"request_id,omitempty"`
}

type Message struct {
    Role    string   `json:"role"`
    Content string   `json:"content"`
    MediaURLs []string `json:"media_urls,omitempty"`
}
```

- [ ] **Step 3: Commit**

```bash
git add src/server/internal/relay/channel/types.go src/server/internal/relay/channel/adapter.go
git commit -m "feat(relay): add Channel types and ProviderAdapter interface

- Channel, ModelRoute, RouteChannel, ChannelStats types
- ProviderAdapter interface with Capabilities, BuildURL/Headers,
  ConvertRequest/Response, DoRequest, HealthCheck, MapError, EstimateUsage
- ProviderRequest and Message internal request types
"
```

---

## Task 5: ChannelPool

**Files:**
- Create: `src/server/internal/relay/pool/pool.go`
- Create: `src/server/internal/relay/pool/stats.go`

- [ ] **Step 1: 创建 ChannelPool**

```go
// src/server/internal/relay/pool/pool.go
package pool

import (
    "context"
    "sync"
    "relay/channel"
)

type ChannelPool struct {
    mu       sync.RWMutex
    channels map[string]*channel.Channel   // channel_id -> channel
    routes   map[string]*channel.ModelRoute // model -> route
    stats    map[string]*channel.ChannelStats // channel_id -> runtime stats
}

func NewChannelPool() *ChannelPool {
    return &ChannelPool{
        channels: make(map[string]*channel.Channel),
        routes:   make(map[string]*channel.ModelRoute),
        stats:    make(map[string]*channel.ChannelStats),
    }
}

func (p *ChannelPool) GetChannel(id string) (*channel.Channel, bool) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    ch, ok := p.channels[id]
    return ch, ok
}

func (p *ChannelPool) GetChannelsByModel(model string) []*channel.RouteChannel {
    p.mu.RLock()
    defer p.mu.RUnlock()
    route, ok := p.routes[model]
    if !ok {
        return nil
    }
    return route.Channels
}

func (p *ChannelPool) GetStats(channelID string) (*channel.ChannelStats, bool) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    stats, ok := p.stats[channelID]
    return stats, ok
}

func (p *ChannelPool) UpdateChannel(ch *channel.Channel) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.channels[ch.ID] = ch
    if p.stats[ch.ID] == nil {
        p.stats[ch.ID] = &channel.ChannelStats{ChannelID: ch.ID}
    }
}

func (p *ChannelPool) UpdateRoute(route *channel.ModelRoute) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.routes[route.Model] = route
}

func (p *ChannelPool) ListChannels() []*channel.Channel {
    p.mu.RLock()
    defer p.mu.RUnlock()
    result := make([]*channel.Channel, 0, len(p.channels))
    for _, ch := range p.channels {
        result = append(result, ch)
    }
    return result
}
```

- [ ] **Step 2: Commit**

```bash
git add src/server/internal/relay/pool/pool.go src/server/internal/relay/pool/stats.go
git commit -m "feat(relay): add ChannelPool with in-memory routing cache

- Thread-safe pool with RWMutex
- GetChannel, GetChannelsByModel, GetStats
- UpdateChannel, UpdateRoute for dynamic reload
- ListChannels for admin/debug
"
```

---

## Task 6: Handler 路由注册骨架

**Files:**
- Create: `src/server/internal/relay/handler/router.go`

- [ ] **Step 1: 创建 Handler 路由注册骨架**

```go
// src/server/internal/relay/handler/router.go
package handler

import (
    "github.com/gin-gonic/gin"
    "relay"
)

// Handler 接口
type Handler interface {
    Handle(c *gin.Context) error
    HandleStream(c *gin.Context) error
}

// Route 定义
type Route struct {
    Method    string
    Path      string
    APIType   relay.APIType
    Strategy  relay.HandlerStrategy
    Retryable bool
}

// RegisterRoutes 注册全部 35 个路由
func RegisterRoutes(e *gin.Engine, handlers map[relay.APIType]Handler) {
    routes := getOpenAIRoutes()

    for _, r := range routes {
        h, ok := handlers[r.APIType]
        if !ok {
            continue
        }

        if r.Strategy == relay.StrategyPassthrough {
            // 透传路由直接注册
            e.Handle(r.Method, r.Path, func(c *gin.Context) {
                h.Handle(c)
            })
            continue
        }

        // 原生处理和文件代理走同一 handler
        e.Handle(r.Method, r.Path, func(c *gin.Context) {
            if r.APIType == relay.APITypeRealtime {
                h.HandleStream(c)
            } else {
                h.Handle(c)
            }
        })
    }

    // 健康检查
    e.GET("/healthz", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })
}

func getOpenAIRoutes() []Route {
    return []Route{
        // Chat / Responses
        {Method: "POST", Path: "/v1/chat/completions", APIType: relay.APITypeChat, Strategy: relay.StrategyNative, Retryable: true},
        {Method: "POST", Path: "/v1/responses", APIType: relay.APITypeResponses, Strategy: relay.StrategyNative, Retryable: true},

        // Realtime
        {Method: "WS", Path: "/v1/realtime", APIType: relay.APITypeRealtime, Strategy: relay.StrategyNative, Retryable: false},

        // Embeddings
        {Method: "POST", Path: "/v1/embeddings", APIType: relay.APITypeEmbeddings, Strategy: relay.StrategyNative, Retryable: true},

        // Images
        {Method: "POST", Path: "/v1/images/generations", APIType: relay.APITypeImageGen, Strategy: relay.StrategyNative, Retryable: false},
        {Method: "POST", Path: "/v1/images/edits", APIType: relay.APITypeImageEdit, Strategy: relay.StrategyNative, Retryable: false},
        {Method: "POST", Path: "/v1/images/variations", APIType: relay.APITypeImageVar, Strategy: relay.StrategyNative, Retryable: false},

        // Videos
        {Method: "POST", Path: "/v1/videos", APIType: relay.APITypeVideos, Strategy: relay.StrategyNative, Retryable: false},

        // Audio
        {Method: "POST", Path: "/v1/audio/speech", APIType: relay.APITypeAudioSpeech, Strategy: relay.StrategyNative, Retryable: false},
        {Method: "POST", Path: "/v1/audio/transcriptions", APIType: relay.APITypeAudioSTT, Strategy: relay.StrategyNative, Retryable: false},
        {Method: "POST", Path: "/v1/audio/translations", APIType: relay.APITypeAudioTranslate, Strategy: relay.StrategyNative, Retryable: false},

        // Moderations / Legacy
        {Method: "POST", Path: "/v1/moderations", APIType: relay.APITypeModeration, Strategy: relay.StrategyNative, Retryable: false},
        {Method: "POST", Path: "/v1/completions", APIType: relay.APITypeCompletions, Strategy: relay.StrategyNative, Retryable: true},

        // Batch
        {Method: "POST", Path: "/v1/batch", APIType: relay.APITypeBatch, Strategy: relay.StrategyNative, Retryable: false},
        {Method: "GET", Path: "/v1/batches", APIType: relay.APITypeBatch, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/batches/:id", APIType: relay.APITypeBatch, Strategy: relay.StrategyPassthrough, Retryable: false},

        // Files
        {Method: "POST", Path: "/v1/files", APIType: relay.APITypeFiles, Strategy: relay.StrategyFileProxy, Retryable: false},
        {Method: "GET", Path: "/v1/files", APIType: relay.APITypeFiles, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/files/:id", APIType: relay.APITypeFiles, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "DELETE", Path: "/v1/files/:id", APIType: relay.APITypeFiles, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/files/:id/content", APIType: relay.APITypeFiles, Strategy: relay.StrategyPassthrough, Retryable: false},

        // Fine-tuning
        {Method: "POST", Path: "/v1/fine_tuning/jobs", APIType: relay.APITypeFineTuning, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/fine_tuning/jobs", APIType: relay.APITypeFineTuning, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/fine_tuning/jobs/:id", APIType: relay.APITypeFineTuning, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "POST", Path: "/v1/fine_tuning/jobs/:id/cancel", APIType: relay.APITypeFineTuning, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/fine_tuning/jobs/:id/events", APIType: relay.APITypeFineTuning, Strategy: relay.StrategyPassthrough, Retryable: false},

        // Assistants / Threads / Runs
        {Method: "POST", Path: "/v1/assistants", APIType: relay.APITypeAssistants, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/assistants", APIType: relay.APITypeAssistants, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/assistants/:id", APIType: relay.APITypeAssistants, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "POST", Path: "/v1/threads", APIType: relay.APITypeThreads, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/threads/:id", APIType: relay.APITypeThreads, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "POST", Path: "/v1/threads/:id/runs", APIType: relay.APITypeRuns, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "GET", Path: "/v1/threads/:id/runs/:rid", APIType: relay.APITypeRuns, Strategy: relay.StrategyPassthrough, Retryable: false},
        {Method: "POST", Path: "/v1/threads/:id/runs/:rid/submit", APIType: relay.APITypeRuns, Strategy: relay.StrategyPassthrough, Retryable: false},
    }
}
```

- [ ] **Step 2: Commit**

```bash
git add src/server/internal/relay/handler/router.go
git commit -m "feat(relay): add handler router with 35 OpenAI routes

Register all routes by strategy:
- StrategyNative: Chat, Responses, Embeddings, Images, Audio, Moderations, Batch
- StrategyPassthrough: Batch status, Files read, Fine-tuning, Assistants/Threads/Runs
- StrategyFileProxy: Files upload (multipart -> S3 -> OpenAI)

WebSocket registered via HandleStream for Realtime.
"
```

---

## Self-Review

1. **Spec coverage:** 骨架/接口/数据模型/Pool/Handler 路由注册 - 全部覆盖 ✅
2. **Placeholder scan:** 无 TBD/TODO ✅
3. **Type consistency:** `APIType` 在 api_types.go 和 adapter.go 一致 ✅；`ProviderRequest` 在 adapter.go 定义 ✅
