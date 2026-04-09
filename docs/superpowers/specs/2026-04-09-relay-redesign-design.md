# Relay 重新设计文档

日期：2026-04-09
版本：v1.0

## 1. 背景与目标

### 1.1 现状问题

New-API 的 Relay 架构存在以下核心问题：

| 问题 | 表现 |
|------|------|
| RelayInfo God Object | 100+ 字段混合计费、转换、流式、渠道关注点 |
| Adaptor 接口渗漏厂商细节 | `ConvertGeminiRequest()`、`ConvertClaudeRequest()` 等不应在接口层 |
| Response 处理重复 | 每个 channel 的 `DoResponse()` 有数百行 switch 分支 |
| 无 Middleware 能力 | 无法在 relay 层统一添加日志/监控/追踪 |
| Task 系统平行不统一 | `TaskAdaptor` 和 `Adaptor` 是两套并列结构 |
| 计费与 relay 紧耦合 | billing 逻辑散落在 service 层各处 |

### 1.2 设计目标

1. **完全重写 Relay**，新接口设计清晰，Provider 扩展口完善
2. **高可用路由**：Circuit Breaker + Token Bucket 限流 + 主动健康检查 + 重试降级
3. **成本加成计费**：渠道实际成本 × 溢价系数 × 组折扣，定价透明
4. **第一期只做 OpenAI Provider**，完整实现全部 15 个接口，跑稳后再扩展 Anthropic/Gemini

### 1.3 整合背景

本 Relay 是 Oblivious 项目整合 LobeHub（C 端）+ New-API（B 端）架构的核心组件：

```
LobeHub UI (C 端体验)
    ↓ Agent Runtime 调用
Relay (本设计) → OpenAI Provider
    ↓ 计费
Quota/Account System
    ↑
Admin UI (管控台) → Channel Manager / Subscription Manager
```

## 2. 整体架构

### 2.1 四层架构

```
┌─────────────────────────────────────────────────────────────┐
│                      API Handler                            │
│          (HTTP/Gin 入口，按 path 分发到 Provider Handler)    │
├─────────────────────────────────────────────────────────────┤
│                 Provider Handler Layer                      │
│                                                              │
│   OpenAIHandler  │ AnthropicHandler  │  GeminiHandler      │
│   (OpenAI 全部15接口)                                        │
├─────────────────────────────────────────────────────────────┤
│                    Router Engine                            │
│         (熔断 / 权重路由 / 限流 / 重试 / 降级)              │
├─────────────────────────────────────────────────────────────┤
│                   Provider Adapters                         │
│                                                              │
│   OpenAIAdapter  │ AnthropicAdapter  │  GeminiAdapter        │
│   (HTTP 执行 + 错误映射 + 健康检查)                          │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
                  ┌─────────────────────┐
                  │   Billing Hooks     │
                  │  (预扣 / 结算 / 回填)│
                  └─────────────────────┘
```

### 2.2 目录结构

```
src/
├── relay/
│   ├── handler/                    # HTTP 入口
│   │   ├── router.go              # 主路由，按 path 分发
│   │   ├── chat.go                 # Chat Completions
│   │   ├── responses.go            # Responses API
│   │   ├── realtime.go             # Realtime WebSocket
│   │   ├── embeddings.go           # Embeddings
│   │   ├── images.go               # Images (generations/edits/variations)
│   │   ├── audio.go                # Audio (speech/transcriptions/translations)
│   │   ├── moderations.go          # Moderations
│   │   ├── completions.go          # Legacy Completions
│   │   ├── batch.go                # Batch 异步
│   │   ├── assistants.go           # Assistants (透传)
│   │   ├── fine_tuning.go          # Fine-tuning (透传)
│   │   └── videos.go               # Videos
│   │
│   ├── router/                    # 高可用路由引擎
│   │   ├── router.go               # 主路由器
│   │   ├── circuit_breaker.go      # 熔断器
│   │   ├── health_checker.go       # 主动健康检查
│   │   ├── load_balancer.go        # 负载均衡
│   │   ├── rate_limiter.go         # Token Bucket 限流
│   │   └── fallback.go             # 降级策略
│   │
│   ├── channel/                   # Provider 适配器
│   │   ├── adapter.go             # Adapter 接口定义
│   │   ├── openai/
│   │   │   ├── adapter.go         # OpenAI 主适配器
│   │   │   ├── models.go          # 模型映射
│   │   │   ├── stream.go          # SSE 流处理
│   │   │   └── errors.go          # 错误映射
│   │   ├── anthropic/              # (Phase 2)
│   │   └── gemini/                # (Phase 3)
│   │
│   ├── pool/                      # Channel Pool
│   │   ├── pool.go                # 池化管理
│   │   └── stats.go               # 运行时状态
│   │
│   ├── billing/                   # 计费接入
│   │   ├── hooks.go               # PreBill / PostBill
│   │   └── estimator.go           # 费用估算
│   │
│   └── metrics/                   # 监控指标
│       └── prometheus.go          # Prometheus 导出
```

## 3. Provider Handler 接口设计

### 3.1 统一 Handler 接口

每个 Provider 有一个 Handler，实现以下接口：

```go
type ProviderHandler interface {
    // Provider 标识
    Provider() string

    // 支持的接口列表
    SupportedRoutes() []Route

    // 请求处理
    Handle(c *gin.Context) error

    // 流式处理（如果支持）
    HandleStream(c *gin.Context) error
}

type Route struct {
    Method string
    Path   string
    // 对应 OpenAI 接口
    APIType APIType
}
```

### 3.2 APIType 枚举

```go
type APIType int

const (
    APITypeUnknown APIType = iota
    APITypeChat
    APITypeResponses
    APITypeRealtime
    APITypeAssistants
    APITypeThreads       // /v1/threads/*
    APITypeRuns          // /v1/threads/*/runs/*
    APITypeBatch
    APITypeBatchFiles    // /v1/batches/* (Batch 本身是 Job，文件走 /v1/files)
    APITypeFineTuning
    APITypeFiles         // /v1/files (对象存储，Batch/Fine-tuning/Assistants 共用)
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
```

### 3.3 OpenAI Handler 路由表

OpenAI API 按"资源族"组织，分为三类处理策略：

| 策略 | 说明 |
|------|------|
| **原生处理** | Adapter 解析请求、转换格式、计费、走 Router 选渠道 |
| **透传** | 直接转发给 OpenAI，不走 Router 选渠道路由（因为是管理类接口） |
| **文件代理** | 用户上传到本服务，再转发到 OpenAI；OpenAI 返回的 file_id 映射回本地 |

**路由表：**

| Method | Path | APIType | 策略 | 计费维度 | 重试 |
|--------|------|---------|------|---------|------|
| POST | `/v1/chat/completions` | `APITypeChat` | 原生处理 | `prompt_tokens + completion_tokens` | ✅ |
| POST | `/v1/responses` | `APITypeResponses` | 原生处理 | `prompt_tokens + completion_tokens` | ✅ |
| WS | `/v1/realtime` | `APITypeRealtime` | 原生处理 | 按 token 计费 | ✅（连接级） |
| POST | `/v1/embeddings` | `APITypeEmbeddings` | 原生处理 | `total_tokens` | ✅ |
| POST | `/v1/images/generations` | `APITypeImageGen` | 原生处理 | `image_count` | ❌（幂等） |
| POST | `/v1/images/edits` | `APITypeImageEdit` | 原生处理 | `image_count` | ❌ |
| POST | `/v1/images/variations` | `APITypeImageVar` | 原生处理 | `image_count` | ❌ |
| POST | `/v1/videos` | `APITypeVideos` | 原生处理 | `video_count` | ❌ |
| POST | `/v1/audio/speech` | `APITypeAudioSpeech` | 原生处理 | `audio_seconds` | ❌ |
| POST | `/v1/audio/transcriptions` | `APITypeAudioSTT` | 原生处理 | `audio_seconds` | ❌ |
| POST | `/v1/audio/translations` | `APITypeAudioTranslate` | 原生处理 | `audio_seconds` | ❌ |
| POST | `/v1/moderations` | `APITypeModeration` | 原生处理 | `input_tokens` | ❌ |
| POST | `/v1/completions` | `APITypeCompletions` | 原生处理 | `prompt_tokens + completion_tokens` | ✅ |
| POST | `/v1/batch` | `APITypeBatch` | 原生处理 | Batch 完成前预付，成功后结算 | ✅（Job 级） |
| GET | `/v1/batches` | `APITypeBatch` | 透传 | 无（只读） | ❌ |
| GET | `/v1/batches/:id` | `APITypeBatch` | 透传 | 无（只读） | ❌ |
| POST | `/v1/files` | `APITypeFiles` | 文件代理 | `storage_bytes` | ❌ |
| GET | `/v1/files` | `APITypeFiles` | 文件代理 | 无 | ❌ |
| GET | `/v1/files/:id` | `APITypeFiles` | 文件代理 | 无 | ❌ |
| DELETE | `/v1/files/:id` | `APITypeFiles` | 文件代理 | 无 | ❌ |
| GET | `/v1/files/:id/content` | `APITypeFiles` | 文件代理 | 无 | ❌ |
| POST | `/v1/fine_tuning/jobs` | `APITypeFineTuning` | 透传 | 按训练 token 计费 | ❌ |
| GET | `/v1/fine_tuning/jobs` | `APITypeFineTuning` | 透传 | 无 | ❌ |
| GET | `/v1/fine_tuning/jobs/:id` | `APITypeFineTuning` | 透传 | 无 | ❌ |
| POST | `/v1/fine_tuning/jobs/:id/cancel` | `APITypeFineTuning` | 透传 | 无 | ❌ |
| GET | `/v1/fine_tuning/jobs/:id/events` | `APITypeFineTuning` | 透传 | 无 | ❌ |
| POST | `/v1/assistants` | `APITypeAssistants` | 透传 | 按 token 计费 | ❌ |
| GET | `/v1/assistants` | `APITypeAssistants` | 透传 | 无 | ❌ |
| GET | `/v1/assistants/:id` | `APITypeAssistants` | 透传 | 无 | ❌ |
| POST | `/v1/assistants/:id` | `APITypeAssistants` | 透传 | 按 token 计费 | ❌ |
| DELETE | `/v1/assistants/:id` | `APITypeAssistants` | 透传 | 无 | ❌ |
| POST | `/v1/threads` | `APITypeThreads` | 透传 | 按 token 计费 | ❌ |
| GET | `/v1/threads/:id` | `APITypeThreads` | 透传 | 无 | ❌ |
| POST | `/v1/threads/:id/runs` | `APITypeRuns` | 透传 | 按 token 计费 | ❌ |
| GET | `/v1/threads/:id/runs/:rid` | `APITypeRuns` | 透传 | 无 | ❌ |
| POST | `/v1/threads/:id/runs/:rid/submit` | `APITypeRuns` | 透传 | 无 | ❌ |

**说明：**
- **原生处理**：走 Router 选渠道 + 计费系统
- **透传**：直接转发（因为是管理接口不走渠道路由）
- **文件代理**：`/v1/files` 的文件内容先存本地 S3，再转发给 OpenAI；本服务存储 `storage_path` 与 `openai_file_id` 的映射

### 3.4 统一请求/响应格式

Handler 层不统一格式，每个 Handler 负责：
1. 解析 Provider 原生请求格式
2. 提取关键字段（model、messages、stream 等）
3. 调用 Router
4. 将 Router 返回的响应转回 Provider 格式

**透传接口**（Assistants、Fine-tuning）：直接透传，不做解析。

## 4. Provider Adapter 接口设计

### 4.1 核心接口

```go
// 能力声明
type Capabilities struct {
    SupportsChat        bool
    SupportsStreaming   bool
    SupportsEmbeddings  bool
    SupportsImages      bool
    SupportsAudio       bool
    SupportsRealtime    bool
    SupportsTaskPolling bool // Batch 等异步任务
}

// Provider Adapter 接口
type ProviderAdapter interface {
    // 元信息
    Name() string
    Provider() string
    Capabilities() Capabilities

    // 请求构建
    BuildURL(model string, apiType APIType) (string, error)
    BuildHeaders(ctx context.Context, model string, apiType APIType) (http.Header, error)

    // 请求转换（Provider 原生 ↔ 内部格式）
    ConvertRequest(req *ProviderRequest) (*ProviderRequest, error)
    ConvertResponse(resp []byte, isStream bool) (*ProviderResponse, error)

    // HTTP 执行
    DoRequest(ctx context.Context, req *ProviderRequest) (*http.Response, error)

    // 健康检查（零成本）
    HealthCheck(ctx context.Context) error

    // 错误映射
    MapError(statusCode int, body []byte) *ProviderError

    // Token 估算（用于预扣费）
    EstimateTokens(req *ProviderRequest) int
}
```

### 4.2 内部请求/响应格式

```go
type ProviderRequest struct {
    APIType APIType
    Model   string
    Headers http.Header
    URL     string

    // 各接口通用字段
    Stream       bool
    Messages     []Message
    MaxTokens    int
    Temperature  float64

    // 接口特有字段（按需填充）
    Input        string           // embeddings / audio
    AudioFormat  string           // speech TTS
    ImageURL     string           // images
    Prompt       string           // legacy completions
    // ... 其他字段
}

type Message struct {
    Role    string
    Content string
    // 多模态
    MediaURLs []string
}

type ProviderResponse struct {
    Content   string
    Done      bool
    Usage     *Usage
    Error     *ProviderError
    // 流式回调
    StreamCB  func(chunk []byte) error
}

type Usage struct {
    PromptTokens     int     `json:"prompt_tokens"`
    CompletionTokens int     `json:"completion_tokens"`
    TotalTokens      int     `json:"total_tokens"`
    // 非 token 维度
    ImageCount       int     `json:"image_count"`
    VideoCount       int     `json:"video_count"`
    AudioSeconds     float64 `json:"audio_seconds"`
    StorageBytes     int64   `json:"storage_bytes"`
    TrainingTokens   int     `json:"training_tokens"`
}
```

### 4.3 错误类型

```go
type ProviderError struct {
    Code       string // "rate_limit" | "invalid_api_key" | "circuit_open" | ...
    Message    string
    StatusCode int
    Retryable  bool // 是否可重试
}
```

## 5. 高可用路由引擎

### 5.1 Router 主流程

```
请求进来 (model, apiType)
    │
    ▼
从 Pool 获取该模型的渠道列表
    │
    ▼
根据 Strategy 选择渠道：
    ├─ Weighted:   按 Weight 权重随机
    ├─ Priority:   按 Priority 找第一个可用的
    └─ CostAware:  找成本最低且可用的
    │
    ▼
对候选渠道排序（优先级 + 权重）
    │
    ▼
遍历渠道列表：
    │
    ├─ CircuitBreaker.IsAvailable()?
    │       ├─ Open → 跳过
    │       └─ Closed → 继续
    │
    ├─ RateLimiter.Allow()?
    │       └─ 超限 → 跳过
    │
    └─ Enabled == true?
            └─ 否 → 跳过
    │
    ▼
执行请求
    │
    ├─ 成功 → 更新延迟记录 → 返回
    │
    ├─ 5xx / 429 错误
    │       ├─ 记录失败 → CircuitBreaker.RecordFailure()
    │       ├─ RetryCount < MaxRetries → 试下一个渠道
    │       └─ RetryCount >= MaxRetries → 继续遍历
    │
    └─ 其他错误 → 直接返回
    │
    ▼
所有渠道失败 → Fallback 检查
    │
    ├─ 有 Fallback 配置 → 走 Fallback 渠道
    └─ 无 Fallback 或全挂 → 503 + Retry-After Header
```

### 5.2 Circuit Breaker

#### 状态机

```
Closed ──(UserError >= 5)──→ Open ──(10s 后探)──→ HalfOpen
  ↑                              │                      │
  │                              │                      │
  │                              │              ├─ 成功 >= 3 → Closed
  │                              │              └─ 失败 → Open
  └────(连续成功 >= 3)───────────┴────(任何错误)─────────┘
```

#### 关键参数

| 参数 | 值 | 说明 |
|------|-----|------|
| `failure_threshold` | 5 | Closed→Open 的失败次数 |
| `recovery_timeout` | 30s | Open→HalfOpen 的探活间隔 |
| `success_threshold` | 3 | HalfOpen→Closed 的成功次数 |
| `probe_interval_escalation` | 连续 5 次失败后从 10s 升到 60s | 节省配额 |

#### 错误分类

| 错误类型 | 来源 | 行为 |
|---------|------|------|
| User Error | 真实用户请求失败 | 计入 Closed→Open 计数 |
| Probe Error | 健康检查探测失败 | 仅决定 Open→Open 或 HalfOpen→Open，不加重 |

### 5.3 Token Bucket Rate Limiter

#### 实现

```go
type TokenBucket struct {
    mu sync.Mutex

    // RPM
    rpmCapacity   int
    rpmTokens     int
    rpmLastRefill time.Time

    // TPM
    tpmCapacity   int
    tpmTokens     int
    tpmLastRefill time.Time
}

func (tb *TokenBucket) Allow(rpmTokens, tpmTokens int) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    tb.refill()

    rpmOk := tb.rpmTokens >= rpmTokens
    tpmOk := tb.tpmTokens >= tpmTokens

    if rpmOk && tpmOk {
        tb.rpmTokens -= rpmTokens
        tb.tpmTokens -= tpmTokens
        return true
    }
    return false
}

func (tb *TokenBucket) refill() {
    now := time.Now()

    // RPM refill
    rpmElapsed := now.Sub(tb.rpmLastRefill)
    if rpmElapsed >= time.Minute {
        tb.rpmTokens = tb.rpmCapacity
        tb.rpmLastRefill = now
    }

    // TPM refill（独立计算，不能复用 rpmElapsed）
    tpmElapsed := now.Sub(tb.tpmLastRefill)
    if tpmElapsed >= time.Minute {
        tb.tpmTokens = tb.tpmCapacity
        tb.tpmLastRefill = now
    }
}
```

#### 限流维度

| 维度 | 说明 |
|------|------|
| Per-Channel RPM | 每个渠道每分钟最大请求数 |
| Per-Channel TPM | 每个渠道每分钟最大 Token 数 |
| Per-User RPM | 每个用户每分钟最大请求数（可选） |
| Per-User TPM | 每个用户每分钟最大 Token 数（可选） |

### 5.4 主动健康检查

#### 策略

| 渠道状态 | 策略 |
|---------|------|
| Closed（正常） | 不探测，靠用户真实请求被动监测 |
| Open（断路） | 每 10s 探测一次，连续 5 次失败后降到 60s |
| HalfOpen（半开） | 允许真实用户请求进入，成功即恢复 |

#### 探测源优先级

1. **第一优先**：`GET /v1/models`（零成本，只消耗 RPM）
2. **备选**：极低 Token 的 Chat 探测（如 `{"model":"gpt-4o-mini","max_tokens":5,"messages":[{"role":"user","content":"hi"}]}`)
3. **辅助**：渠道余额 API（如果可用）

### 5.5 降级策略

#### Fallback 配置（可选，用户开启）

```go
type FallbackConfig struct {
    Enabled     bool
    Model       string       // 兜底模型，如 gpt-4o-mini
    ChannelID   string       // 兜底渠道
    StrictMatch bool        // true=只有原模型全挂才降级
}
```

#### 全挂处理

```
所有渠道均不可用：
    │
    ├─ 返回 503 Service Unavailable
    └─ Header: Retry-After: 30
```

**禁止排队**：LLM 请求耗时长，排队会导致连接积压引发雪崩。

## 6. 计费接入

### 6.1 定价模型：按 APIType + UsageDimension 计费

**旧模型的问题**：单一 `base_cost × (prompt+completion)_tokens` 无法覆盖：
- Images/Audio/Videos：按次数计费，不按 token
- Files storage：按存储字节计费
- Fine-tuning：按训练 token 计费
- Prompt/Completion 价格通常不对称

**新模型：按资源族 + 使用量维度计费**

```go
// 定价维度枚举
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

// 定价表：APIType × Model × Dimension → 单价(USD)
type PricingEntry struct {
    APIType       APIType
    Model         string  // "*" 表示该 APIType 下所有模型
    Dimension     UsageDimension
    UnitCost      float64 // USD per unit
    Markup        float64 // 平台溢价系数
}

var defaultPricingTable = []PricingEntry{
    // Chat / Responses / Completions：按 token 计费，prompt/completion 不对称
    {APITypeChat, "gpt-4o", DimPromptTokens, 0.0000025, 1.5},      // $2.5/1M
    {APITypeChat, "gpt-4o", DimCompletionTokens, 0.00001, 1.5},       // $10/1M
    {APITypeChat, "gpt-4o-mini", DimPromptTokens, 0.000000075, 1.5},
    {APITypeChat, "gpt-4o-mini", DimCompletionTokens, 0.0000003, 1.5},

    // Embeddings
    {APITypeEmbeddings, "*", DimTotalTokens, 0.0000001, 1.5},       // $0.1/1M

    // Images（按张数，不按 token）
    {APITypeImageGen, "dall-e-3", DimImageCount, 0.040, 1.5},       // $0.04/张
    {APITypeImageEdit, "dall-e-3", DimImageCount, 0.080, 1.5},
    {APITypeImageVar, "dall-e-3", DimImageCount, 0.080, 1.5},

    // Videos（按生成秒数）
    {APITypeVideos, "*", DimVideoCount, 0.050, 1.5},               // $0.05/秒

    // Audio（按音频秒数）
    {APITypeAudioSpeech, "*", DimAudioSeconds, 0.015, 1.5},        // TTS $15/1K 秒
    {APITypeAudioSTT, "*", DimAudioSeconds, 0.00006, 1.5},         // Whisper $0.06/分钟
    {APITypeAudioTranslate, "*", DimAudioSeconds, 0.00006, 1.5},

    // Files storage（按存储字节·天）
    {APITypeFiles, "*", DimStorageBytes, 0.0000001, 1.0},          // $0.1/1M bytes/day

    // Fine-tuning training（按训练 token）
    {APITypeFineTuning, "*", DimTrainingTokens, 0.008, 1.5},      // $8/1M
}

// 计算单次请求费用
func CalculateCost(apiType APIType, model string, usage *Usage, groupDiscount float64) float64 {
    entries := getPricingEntries(apiType, model)

    var totalCost float64
    for _, entry := range entries {
        unit := getUsageUnit(usage, entry.Dimension)
        cost := entry.UnitCost * unit * entry.Markup * groupDiscount
        totalCost += cost
    }
    return totalCost
}

func getUsageUnit(usage *Usage, dim UsageDimension) float64 {
    switch dim {
    case DimPromptTokens:     return float64(usage.PromptTokens)
    case DimCompletionTokens: return float64(usage.CompletionTokens)
    case DimTotalTokens:      return float64(usage.TotalTokens)
    case DimImageCount:       return float64(usage.ImageCount)
    case DimVideoCount:       return float64(usage.VideoCount)
    case DimAudioSeconds:     return float64(usage.AudioSeconds)
    case DimStorageBytes:     return float64(usage.StorageBytes)
    case DimTrainingTokens:   return float64(usage.TrainingTokens)
    }
    return 0
}
```

### 6.2 计费钩子

```go
type BillingSession struct {
    ID                string    `json:"id"`           // UUID，幂等键
    UserID            string    `json:"user_id"`
    RequestID         string    `json:"request_id"`  // 来自 X-Request-ID header，幂等
    IdempotencyKey    string    `json:"idempotency_key"` // 客户端传入的幂等键
    ChannelID         string    `json:"channel_id"`
    Model             string    `json:"model"`
    APIType           APIType   `json:"api_type"`
    PreAuthorizedAmt  float64   `json:"pre_authorized_amt"` // 预扣金额
    SettledAmt        float64   `json:"settled_amt"`       // 结算金额
    Status            string    `json:"status"`   // "preauthorized" | "settled" | "refunded"
    AttemptNo         int       `json:"attempt_no"`        // 重试次数
    CreatedAt         time.Time `json:"created_at"`
}
```

#### PreBill（请求前）

```go
func PreBill(ctx context.Context, req *ProviderRequest, channel *Channel, idempotencyKey string) (*BillingSession, error) {
    // 1. 幂等检查：同一 IdempotencyKey 已存在，直接返回已有 session
    existing := billingSessionStore.FindByIdempotencyKey(idempotencyKey)
    if existing != nil {
        return existing, nil
    }

    // 2. 估算用量（使用 tiktoken-go）
    estimatedUsage := channel.Adapter.EstimateUsage(req) // 返回 *Usage 估算

    // 3. 计算预扣金额
    cost := CalculateCost(req.APIType, req.Model, estimatedUsage, user.GroupDiscount)

    // 4. 预扣配额
    if err := quota.PreConsume(ctx, userID, cost); err != nil {
        return nil, ErrInsufficientQuota
    }

    // 5. 创建会话（持久化！）
    session := &BillingSession{
        ID:               uuid.New().String(),
        UserID:           userID,
        RequestID:        req.RequestID,
        IdempotencyKey:   idempotencyKey,
        ChannelID:        channel.ID,
        Model:            req.Model,
        APIType:          req.APIType,
        PreAuthorizedAmt: cost,
        Status:           "preauthorized",
        AttemptNo:        1,
        CreatedAt:        time.Now(),
    }
    billingSessionStore.Save(session)

    return session, nil
}
```

#### PostBill（请求后）

```go
func PostBill(ctx context.Context, session *BillingSession, resp *ProviderResponse, channel *Channel) {
    if resp.Usage == nil {
        // 无 usage 响应（某些接口如 Images/Videos 直接返回 URL）
        // 按固定估算值结算
        quota.Settle(ctx, session.UserID, 0) // 差额已在 PreBill 全额预扣
        return
    }

    // 计算实际费用
    actualCost := CalculateCost(session.APIType, session.Model, resp.Usage, user.GroupDiscount)
    delta := actualCost - session.PreAuthorizedAmt

    session.SettledAmt = actualCost
    session.Status = "settled"
    billingSessionStore.Update(session)

    quota.Settle(ctx, session.UserID, delta) // 多退少补
}
```

#### Refund（失败时）

```go
func Refund(ctx context.Context, sessionID string) {
    session := billingSessionStore.Get(sessionID)
    if session == nil || session.Status == "refunded" {
        return
    }
    quota.Refund(ctx, session.UserID, session.PreAuthorizedAmt)
    session.Status = "refunded"
    billingSessionStore.Update(session)
}
```

### 6.3 账单幂等与重试语义

**核心问题**：Router 支持渠道失败后切换下一渠道重试，这意味着同一请求可能到达多个渠道。需要明确：
- 同一 `X-Request-ID` / `IdempotencyKey` 是否使用同一 `BillingSession`？
- 重复请求是否会导致重复预扣？

**策略：同一 IdempotencyKey 复用 BillingSession，AttemptNo 递增**

```
请求 1（X-Request-ID=req-1, IdempotencyKey=idem-1）
    → BillingSession(attempt_no=1, status=preauthorized)
    → 渠道 A 执行
    → 失败（5xx）

请求 1 重试（X-Request-ID=req-1, IdempotencyKey=idem-1）
    → FindByIdempotencyKey("idem-1") → 找到已有 Session
    → session.AttemptNo++ (变成 2)
    → 复用已有 Session，不重复预扣
    → 渠道 B 执行
    → 成功 → PostBill(session)

极端：渠道 A 预扣成功，渠道 B 也预扣成功（两次独立 PreBill）
    → A 失败，Refund(A session)
    → B 成功，PostBill(B session)
    → 用户只被扣一次（第二次的 Refund 会被执行）
```

**幂等键优先级**：
1. 客户端传入的 `X-Idempotency-Key` header（最高）
2. `X-Request-ID` header
3. 自动生成 UUID（仅用于内部追踪）

### 6.4 TPM 预扣保护

```go
func PreConsumeTPM(ctx context.Context, userID, channelID string, estimatedTokens int) error {
    channel := pool.GetChannel(channelID)

    // 使用 tiktoken 精确估算 prompt tokens
    promptTokens := tiktoken.EstimatePromptTokens(req.InputText, req.Model)

    // Completion tokens：按 max_tokens 或默认 2000（不是 maxContext）
    completionTokens := req.MaxTokens
    if completionTokens == 0 {
        completionTokens = 2000
    }

    totalTokens := promptTokens + completionTokens

    if !rateLimiter.AllowTPM(userID, totalTokens) {
        return ErrTPMLimitExceeded
    }
    return nil
}
```

## 7. 监测数据

### 7.1 Prometheus Metrics

```go
var (
    // 请求量
    RequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "relay_requests_total",
            Help: "Total requests",
        },
        []string{"channel_id", "model", "api_type", "status"},
    )

    // 延迟分布
    LatencyHistogram = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "relay_latency_seconds",
            Help:    "Request latency",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
        },
        []string{"channel_id", "model", "api_type"},
    )

    // Token 用量
    TokensTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "relay_tokens_total",
            Help: "Total tokens",
        },
        []string{"channel_id", "model", "type"}, // type: prompt/completion
    )

    // 熔断状态
    CircuitBreakerState = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "relay_circuit_breaker_state",
            Help: "CB state: 0=closed, 1=open, 0.5=half_open",
        },
        []string{"channel_id"},
    )

    // 限流器使用率
    RateLimiterUsage = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "relay_rate_limiter_usage",
            Help: "Rate limiter usage ratio",
        },
        []string{"channel_id", "type"}, // type: rpm/tpm
    )

    // 健康检查探测
    HealthProbeTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "relay_health_probe_total",
            Help: "Health probe results",
        },
        []string{"channel_id", "result"}, // result: success/failure
    )
)
```

### 7.2 导出端点

| 端点 | 格式 | 说明 |
|------|------|------|
| `GET /metrics` | Prometheus | Prometheus 抓取端点 |
| `GET /debug/channels` | JSON | 所有渠道实时状态 |
| `GET /debug/channels/:id` | JSON | 单个渠道详情 |

## 8. 数据模型

### 8.1 Channel

```go
type Channel struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Provider  string `json:"provider"` // "openai"

    // 连接配置
    BaseURL   string `json:"base_url"`  // 默认 https://api.openai.com
    APIKey    string `json:"-"`         // 加密存储

    // 模型配置
    Models    []string `json:"models"`
    MaxContext int     `json:"max_context"` // 最大上下文限制

    // 限速配置
    RPMLimit  int `json:"rpm_limit"`
    TPMLimit  int `json:"tpm_limit"`

    // 熔断配置
    CBThreshold    int `json:"cb_threshold"`     // 熔断阈值，默认 5
    CBTimeout      int `json:"cb_timeout"`       // 恢复超时(秒)，默认 30

    // 成本配置
    BaseCost float64 `json:"base_cost"` // 每 1K token 的基础成本(USD)
    Markup   float64 `json:"markup"`    // 溢价系数，默认 1.0

    // 路由配置
    Strategy string `json:"strategy"` // "weighted" | "priority" | "cost_aware"
    Priority int    `json:"priority"` // 基础优先级

    Enabled  bool `json:"enabled"`
}
```

### 8.2 ModelRoute

```go
type ModelRoute struct {
    Model    string         `json:"model"`
    Strategy string         `json:"strategy"`
    Channels []RouteChannel `json:"channels"`
}

type RouteChannel struct {
    ChannelID string `json:"channel_id"`
    Weight    int    `json:"weight"`    // 权重（weighted 模式）
    Priority  int    `json:"priority"`  // 优先级（priority 模式）
    Enabled   bool   `json:"enabled"`
}
```

### 8.3 ChannelStats（运行时）

```go
type ChannelStats struct {
    ChannelID string `json:"channel_id"`

    // 熔断状态
    CBState        string    `json:"cb_state"` // "closed" | "open" | "half_open"
    CBFailures     int       `json:"cb_failures"`
    CBLastFailure  time.Time `json:"cb_last_failure"`
    CBProbeCount   int       `json:"cb_probe_count"`  // 连续探测失败次数

    // 限流器状态
    RPMCurrent    int `json:"rpm_current"`
    TPMCurrent    int `json:"tpm_current"`
    RPMLastReset  time.Time `json:"rpm_last_reset"`
    TPMLastReset  time.Time `json:"tpm_last_reset"`

    // 监控数据
    TotalRequests  int64 `json:"total_requests"`
    SuccessCount   int64 `json:"success_count"`
    FailureCount   int64 `json:"failure_count"`
    LatencySumUs   int64 `json:"latency_sum_us"`   // 微秒累计
    LatencyCount   int64 `json:"latency_count"`

    // LastProbe 用于计算下次探测间隔
    LastProbeSuccess time.Time `json:"last_probe_success"`
    LastProbeTime    time.Time `json:"last_probe_time"`
}
```

## 9. 实施计划

### Phase 1: OpenAI Provider 完整实现

| 步骤 | 内容 |
|------|------|
| 1.1 | 项目骨架 + Provider Adapter 接口定义 |
| 1.2 | Channel Pool + 路由基础 |
| 1.3 | Circuit Breaker + Token Bucket |
| 1.4 | Health Checker |
| 1.5 | Retry + Fallback |
| 1.6 | Chat Completions 接口 |
| 1.7 | Embeddings 接口 |
| 1.8 | Images 接口（generations/edits/variations） |
| 1.9 | Audio 接口（speech/transcriptions/translations） |
| 1.10 | Responses 接口 |
| 1.11 | Realtime WebSocket 接口 |
| 1.12 | Batch 异步接口 |
| 1.13 | Assistants 透传 |
| 1.14 | Fine-tuning 透传 |
| 1.15 | Moderations + Legacy Completions |
| 1.16 | Videos 接口 |
| 1.17 | Prometheus Metrics 集成 |
| 1.18 | Billing Hooks 集成 |

### Phase 2: Anthropic Provider

（第一期跑稳后启动）

### Phase 3: Gemini Provider

（第二期跑稳后启动）

## 10. 生产风险与缓解措施

### 风险 1：预扣费与 TPM 限流的粗暴估算

**问题**：若按 `maxContext`（128k）粗暴预扣，用户发起简单短对话（实际消耗 500 tokens），系统却按 128,000 预扣，配额或 TPM 限制会在几秒内被假性耗尽，导致后续请求全部被 `ErrTPMLimitExceeded` 拒绝。

**缓解措施**：

```go
// 使用 tiktoken-go 进行精准 Token 估算
import "github.com/pkoukk/tiktoken-go"

func EstimateTokens(text string, model string) int {
    encoding, _ := tiktoken.EncodingForModel(model)
    tokens := encoding.Encode(text, nil, nil)
    return len(tokens)
}

func PreBill(ctx context.Context, req *ProviderRequest, channel *Channel) error {
    // 1. 精准估算 prompt tokens
    promptText := extractPromptText(req)
    promptTokens := EstimateTokens(promptText, req.Model)

    // 2. Completion tokens：前端必须传 max_tokens，或系统给默认缓冲值
    completionTokens := req.MaxTokens
    if completionTokens == 0 {
        completionTokens = 2000 // 默认缓冲，避免按 maxContext 全扣
    }

    totalTokens := promptTokens + completionTokens

    // 3. 检查 TPM 限流器
    if !rateLimiter.AllowTPM(channel.ID, totalTokens) {
        return ErrTPMLimitExceeded
    }

    // 4. 预扣费（按估算值，不是 maxContext）
    cost := calculateCost(totalTokens, channel)
    return quota.PreConsume(ctx, userID, cost)
}
```

### 风险 2：零成本健康检查的可靠性

**问题**：`GET /v1/models` 在很多渠道中由 Nginx/Cloudflare 静态缓存，即使大模型推理集群全部宕机，`/models` 依然秒回 200，导致熔断器无法触发，引发大规模超时。

**缓解措施**：

```go
type Channel struct {
    // ... 原有字段 ...

    // 健康检查策略配置
    HealthCheckStrategy string `json:"health_check_strategy"`
    // "models_api"     - 零成本，GET /v1/models（低权重备用渠道）
    // "realtime_probe" - 真实推理探活（推荐高权重主渠道）
    // "disabled"        - 不探活，纯被动（极少使用的渠道）

    ProbeModel  string `json:"probe_model"`  // 探活用模型，默认 gpt-4o-mini
    ProbePrompt string `json:"probe_prompt"` // 探活 prompt，默认 "hi"
}

func (hc *HealthChecker) HealthCheck(ctx context.Context, channel *Channel) error {
    switch channel.HealthCheckStrategy {
    case "disabled":
        return nil // 不探活

    case "models_api":
        // 零成本，但可能被 CDN 缓存，有漏报风险
        resp, err := http.Get(channel.BaseURL + "/v1/models")
        return err

    case "realtime_probe":
        // 真实推理探活，推荐高权重渠道使用
        body := map[string]any{
            "model": channel.ProbeModel,
            "max_tokens": 5,
            "messages": []map[string]string{{
                "role":    "user",
                "content": channel.ProbePrompt,
            }},
        }
        jsonBody, _ := json.Marshal(body)
        req, _ := http.NewRequest("POST", channel.BaseURL+"/v1/chat/completions", bytes.NewBuffer(jsonBody))
        req.Header.Set("Authorization", "Bearer "+channel.APIKey)
        req.Header.Set("Content-Type", "application/json")

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            hc.recordFailure(channel.ID)
        } else {
            hc.recordSuccess(channel.ID)
        }
        return err
    }
}
```

### 风险 3：HTTP Server 层雪崩

**问题**：全渠道路由失败时，快速涌入的请求会在 Go HTTP Server 层积压，引发 OOM。

**缓解措施**：

```go
// HTTP Server 配置
srv := &http.Server{
    Addr:         ":8080",
    Handler:      router,
    ReadTimeout:  120 * time.Second,  // 长时流式请求需要大 timeout
    WriteTimeout: 120 * time.Second,
    // 注意：net/http.Server 没有 MaxConnsPerIP，需通过 middleware 实现
}

// IP 级连接限制（通过 Gin middleware）
func IPConnLimiter(limit int) gin.HandlerFunc {
    ips := make(map[string]int)
    return func(c *gin.Context) {
        ip := c.ClientIP()
        ips[ip]++
        if ips[ip] > limit {
            c.AbortWithStatus(429)
            return
        }
        c.Next()
        ips[ip]--
    }
}

// 全局并发限制（通过 semaphore）
var globalLimiter = make(chan struct{}, 10000) // 全局最多 10000 并发

func GlobalConcurrencyLimiter() gin.HandlerFunc {
    return func(c *gin.Context) {
        select {
        case globalLimiter <- struct{}{}:
            defer func() { <-globalLimiter }()
            c.Next()
        default:
            c.JSON(503, gin.H{"error": "too many requests"})
            c.Header("Retry-After", "5")
        }
    }
}

// 结合 Retry-After Header，全挂时快速失败不积压
if allChannelsFailed {
    c.JSON(503, gin.H{"error": "service temporarily unavailable"})
    c.Header("Retry-After", "30")
    return
}
```

## 11. Realtime WebSocket 设计

### 11.1 架构问题

当前 `ProviderAdapter` 接口设计是**单次 HTTP 请求-响应**模式：
- `DoRequest()` 返回 `*http.Response`
- 适合 Unary HTTP 和 SSE Streaming

但 Realtime API 使用 **WebSocket 双向流**，与 HTTP 模式不兼容：
- 连接建立：`GET /v1/realtime` → Upgrade to WebSocket
- 双向消息：客户端发送 `session.update`、`conversation.item.create`；服务端推送 `session.created`、`response.done`
- 连接生命周期：不是单次请求，而是长连接
- Usage 回填：整个对话的 token 消耗在连接关闭时才知晓

### 11.2 三类协议抽象

```
┌─────────────────────────────────────────────────────────────┐
│                    ProtocolHandler                           │
│                                                              │
│  ┌────────────┐  ┌────────────┐  ┌────────────────────────┐  │
│  │ UnaryHTTP  │  │ SSEStream │  │ WebSocketTunnel        │  │
│  │ Handler    │  │ Handler   │  │ Handler                │  │
│  │            │  │           │  │                        │  │
│  │ 单次请求   │  │ Server    │  │ 双向消息流              │  │
│  │ 单次响应   │  │ Push +    │  │ 生命周期=连接时长       │  │
│  │            │  │ Client Req│  │                        │  │
│  └────────────┘  └────────────┘  └────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 11.3 WebSocket Tunnel Handler 设计

```go
// RealtimeHandler 独立于 ProviderAdapter 之外
type RealtimeHandler struct {
    channelPool *Pool
    billingHook BillingHook
}

func (h *RealtimeHandler) HandleWebSocket(c *gin.Context) error {
    // 1. 鉴权：从 cookie/query param 取 token，验证用户
    userID, err := auth(c)
    if err != nil {
        return err
    }

    // 2. 解析 model 参数
    model := c.Query("model")
    if model == "" {
        return c.JSON(400, gin.H{"error": "model required"})
    }

    // 3. 幂等键（同一路由多次连接应有同一 session 语义）
    // Realtime 不像 Unary HTTP 有 X-Request-ID，用 connection_id 代替
    connectionID := c.GetHeader("OpenAI-Realtime-Connection-ID")
    if connectionID == "" {
        connectionID = uuid.New().String()
    }

    // 4. PreBill（按连接预扣，而不是按请求）
    // Realtime 预估：按连接时长预扣 1 分钟，后续按 Usage 调整
    estimatedCostPerMinute := getRealtimeEstimate(model)
    if err := quota.PreConsumeMinute(c, userID, estimatedCostPerMinute); err != nil {
        return c.JSON(403, gin.H{"error": "insufficient quota"})
    }

    // 5. 从 Pool 选择渠道（走 Router）
    channel, err := h.channelPool.Select(model)
    if err != nil {
        return c.JSON(503, gin.H{"error": "no channel available"})
    }

    // 6. 建立到上游的 WebSocket 连接
    upstreamURL := channel.BaseURL + "/v1/realtime?model=" + model
    upstreamReq, _ := http.NewRequest("GET", upstreamURL, nil)
    channel.Adapter.AddAuthHeader(upstreamReq, channel.APIKey)
    // 注意：实际的 WebSocket 升级由 gorilla/websocket 处理

    upstreamConn, resp, err := websocket.DefaultDialer.Dial(upstreamURL, upstreamReq.Header)
    if err != nil {
        billingHook.Refund(connectionID)
        return c.JSON(502, gin.H{"error": "upstream connection failed"})
    }

    // 7. 代理双向消息
    clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return err
    }

    // 8. 并发代理：client ↔ relay ↔ upstream
    var wg sync.WaitGroup
    var totalUsage atomic.Value // 累计 usage

    // 8a. 客户端 → 上游
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            _, msg, err := clientConn.ReadMessage()
            if err != nil {
                upstreamConn.Close()
                break
            }
            // 记录客户端发送的 token（客户端消息）
            // 可选：tiktoken 估算后计入 usage
            if err := upstreamConn.WriteMessage websocket.TextMessage, msg); err != nil {
                break
            }
        }
    }()

    // 8b. 上游 → 客户端
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            _, msg, err := upstreamConn.ReadMessage()
            if err != nil {
                clientConn.Close()
                break
            }
            // 解析 usage（OpenAI Realtime 在 response.done event 中发送 usage）
            if parseAndAccumulateUsage(msg, totalUsage) {
                // 收到 response.done，更新 usage
            }
            if err := clientConn.WriteMessage(websocket.TextMessage, msg); err != nil {
                break
            }
        }
    }()

    wg.Wait()

    // 9. 连接关闭后，按实际 Usage 结算
    actualUsage := totalUsage.Load().(*Usage)
    h.billingHook.SettleWebRTC(userID, connectionID, actualUsage)

    return nil
}
```

### 11.4 Realtime 计费

```go
// Realtime 的 Usage 来自 OpenAI 发送的 response.done event
// OpenAI Realtime 协议中，usage 包含：
// { "type": "response.done", "response": { "usage": { "totalTokens": 1234 } } }

func parseAndAccumulateUsage(msg []byte, acc atomic.Value) bool {
    var event map[string]any
    if err := json.Unmarshal(msg, &event); err != nil {
        return false
    }
    if event["type"] != "response.done" {
        return false
    }
    resp := event["response"].(map[string]any)
    usageMap := resp["usage"].(map[string]any)
    acc.Store(&Usage{
        TotalTokens: int(usageMap["totalTokens"].(float64)),
    })
    return true
}

// 连接关闭时结算（按实际 Usage，不按预扣）
func (h *RealtimeHandler) SettleWebRTC(ctx context.Context, userID, connectionID string, usage *Usage) {
    session := billingSessionStore.FindByConnectionID(connectionID)
    if session == nil {
        return
    }

    actualCost := CalculateCost(APITypeRealtime, session.Model, usage, user.GroupDiscount)
    delta := actualCost - session.PreAuthorizedAmt
    quota.Settle(ctx, userID, delta)
    session.Status = "settled"
    billingSessionStore.Update(session)
}
```

### 11.5 断线重连与续活

Realtime 连接断开后，客户端通常会尝试重连。需支持：

```go
// 同一 connection_id 的重连使用同一 BillingSession
func (h *RealtimeHandler) HandleWebSocket(c *gin.Context) error {
    connectionID := c.GetHeader("OpenAI-Realtime-Connection-ID")

    // 查找已有 session
    existingSession := billingSessionStore.FindByConnectionID(connectionID)
    if existingSession != nil {
        // 重连：复用已有 session，AttemptNo++，不重复预扣
        existingSession.AttemptNo++
        billingSessionStore.Update(existingSession)
        // 继续代理...
    } else {
        // 新建连接
        session, _ := h.PreBillRealtime(c, connectionID)
        // ...
    }
}
```

## 12. 未纳入第一期的功能

| 功能 | 原因 |
|------|------|
| Per-User 限流 | 第一期先做 Per-Channel |
| Latency-aware 路由 | 第一期不做，依赖监控数据积累 |
| 团队/配额管理 | 属于 Account 层，独立实现 |
| 多语言 i18n | 错误消息国际化 |
