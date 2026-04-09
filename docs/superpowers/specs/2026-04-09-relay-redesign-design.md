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
    APITypeBatch
    APITypeFineTuning
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

| Method | Path | APIType | 说明 |
|--------|------|---------|------|
| POST | `/v1/chat/completions` | `APITypeChat` | Streaming |
| POST | `/v1/responses` | `APITypeResponses` | Streaming |
| POST | `/v1/embeddings` | `APITypeEmbeddings` | 向量 |
| POST | `/v1/images/generations` | `APITypeImageGen` | DALL-E |
| POST | `/v1/images/edits` | `APITypeImageEdit` | 编辑 |
| POST | `/v1/images/variations` | `APITypeImageVar` | 变体 |
| POST | `/v1/videos` | `APITypeVideos` | 视频 |
| POST | `/v1/audio/speech` | `APITypeAudioSpeech` | TTS，Streaming |
| POST | `/v1/audio/transcriptions` | `APITypeAudioSTT` | Whisper |
| POST | `/v1/audio/translations` | `APITypeAudioTranslate` | Whisper |
| POST | `/v1/moderations` | `APITypeModeration` | 审核 |
| POST | `/v1/completions` | `APITypeCompletions` | Legacy |
| WS | `/v1/realtime` | `APITypeRealtime` | WebSocket |
| POST | `/v1/batch` | `APITypeBatch` | 异步批处理 |
| POST | `/v1/assistants` | `APITypeAssistants` | 透传 |
| POST | `/v1/fine_tuning/*` | `APITypeFineTuning` | 透传 |

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
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
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
    elapsed := now.Sub(tb.rpmLastRefill)

    if elapsed >= time.Minute {
        tb.rpmTokens = tb.rpmCapacity
        tb.rpmLastRefill = now
    }

    if elapsed >= time.Minute {
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

### 6.1 成本加成定价模型

```
用户账单 = 渠道实际成本 × 溢价系数 × 组折扣
```

| 层级 | 含义 | 示例 |
|------|------|------|
| Base（渠道成本） | 上游实际花费 | OpenAI gpt-4o = $0.005/1K tokens |
| Markup（溢价） | 平台服务费 | 1.5x |
| Group Discount（组折扣） | 用户等级折扣 | VIP = 0.8x |

### 6.2 计费钩子

#### PreBill（请求前）

```go
func PreBill(ctx context.Context, req *ProviderRequest, channel *Channel) error {
    // 1. 估算 Token 数量
    estimatedTokens := channel.Adapter.EstimateTokens(req)

    // 2. 计算预扣费用
    cost := channel.BaseCost * estimatedTokens / 1000 * channel.Markup * user.GroupDiscount

    // 3. 预扣配额
    if err := quota.PreConsume(ctx, userID, cost); err != nil {
        return ErrInsufficientQuota // 配额不足，拒绝
    }

    // 4. 记录预扣会话
    billingSession := &BillingSession{
        UserID:    userID,
        ChannelID: channel.ID,
        Model:     req.Model,
        PreQuota:  cost,
        StartTime: time.Now(),
    }
    billingSessionStore.Save(billingSession)

    return nil
}
```

#### PostBill（请求后）

```go
func PostBill(ctx context.Context, req *ProviderRequest, resp *ProviderResponse, channel *Channel) {
    // 1. 计算实际费用
    if resp.Usage != nil {
        actualTokens := resp.Usage.TotalTokens
        actualCost := channel.BaseCost * actualTokens / 1000 * channel.Markup * user.GroupDiscount
        delta := actualCost - billingSession.PreQuota

        // 2. 结算：多退少补
        quota.Settle(ctx, userID, delta)
    }
}
```

#### Refund（失败时）

```go
func Refund(ctx context.Context, billingSessionID string) {
    session := billingSessionStore.Get(billingSessionID)
    quota.Refund(ctx, userID, session.PreQuota)
}
```

### 6.3 TPM 预扣保护

```go
func PreConsumeTPM(ctx context.Context, userID, channelID string, estimatedTokens int) error {
    channel := pool.GetChannel(channelID)

    // 按最大上下文预扣，不按实际用量
    maxContext := channel.MaxContext
    if maxContext == 0 {
        maxContext = 128000 // 默认最大值
    }

    tokensToCharge := min(estimatedTokens, maxContext)

    // 检查 TPM 限流器
    if !rateLimiter.Allow(userID, tokensToCharge) {
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
// HTTP Server 并发上限
srv := &http.Server{
    Addr:         ":8080",
    Handler:      router,
    ReadTimeout:  120 * time.Second,  // 长时流式请求需要大 timeout
    WriteTimeout: 120 * time.Second,
    MaxConnsPerIP: 100,                 // 单一 IP 最大连接数
    // 注意：Go 1.19+ 支持 http.Server.SetMaxRequests()，可限制全局并发
}

// 结合 Retry-After Header，全挂时快速失败不积压
if allChannelsFailed {
    c.JSON(503, gin.H{"error": "service temporarily unavailable"})
    c.Header("Retry-After", "30")
    return
}
```

## 10. 未纳入第一期的功能

| 功能 | 原因 |
|------|------|
| Per-User 限流 | 第一期先做 Per-Channel |
| Latency-aware 路由 | 第一期不做，依赖监控数据积累 |
| 团队/配额管理 | 属于 Account 层，独立实现 |
| 多语言 i18n | 错误消息国际化 |
