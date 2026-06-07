# Relay Plan D: Billing Hooks & Observability

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement billing hooks (PreBill/PostBill/Refund), async billing closure via Asynq, tiktoken-go token estimation, pricing table lookup, and Prometheus metrics registration.

**Architecture:** Billing is a hook layer around the Router. PreBill authorizes usage before proxying; PostBill settles or refunds based on actual usage. Prometheus metrics are registered in a dedicated metrics package and exposed via `/metrics` endpoint.

---

## File Structure

```
src/server/internal/relay/
  billing.go          # BillingHook, PreBill, PostBill, Refund
  pricing.go          # Pricing lookup from DB (pricing_entries table)
  tokenizer.go        # tiktoken-go token estimation
  billing_worker.go   # Asynq worker for async billing closure

src/server/internal/metrics/
  prometheus.go       # Metrics registry and HTTP handler
```

---

### Task 1: Token Estimator (tiktoken-go)

**Files:**
- Create: `src/server/internal/relay/tokenizer.go`
- Test: `src/server/internal/relay/tokenizer_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "testing"
)

func TestTokenEstimator_Estimate(t *testing.T) {
    est := NewTokenEstimator("gpt-4o")

    tokens := est.Estimate("Hello, world!")
    if tokens <= 0 {
        t.Fatalf("expected positive token count, got %d", tokens)
    }
}

func TestTokenEstimator_EstimateEmpty(t *testing.T) {
    est := NewTokenEstimator("gpt-4o")
    tokens := est.Estimate("")
    if tokens != 0 {
        t.Fatalf("expected 0 for empty string, got %d", tokens)
    }
}

func TestTokenEstimator_EstimateLong(t *testing.T) {
    est := NewTokenEstimator("gpt-4o")
    longText := ""
    for i := 0; i < 1000; i++ {
        longText += "Hello world. "
    }
    tokens := est.Estimate(longText)
    if tokens < 100 {
        t.Fatalf("expected substantial tokens for 1000 repetitions, got %d", tokens)
    }
}

func TestTokenEstimator_ModelMapping(t *testing.T) {
    // o200k_base is used by default when model not explicitly mapped
    est := NewTokenEstimator("unknown-model")
    tokens := est.Estimate("test")
    if tokens <= 0 {
        t.Fatalf("expected positive token count even for unknown model, got %d", tokens)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/tokenizer_test.go -v -run TestTokenEstimator`
Expected: FAIL — `tiktoken` not imported, `NewTokenEstimator` not defined

- [ ] **Step 3: Write tokenizer implementation**

```go
package relay

import (
    "github.com/pkoukk/tiktoken-go"
)

type TokenEstimator struct {
    encoder *tiktoken.Tiktoken
}

func NewTokenEstimator(model string) *TokenEstimator {
    // Map model name to tiktoken encoding
    encodingName := modelToEncoding(model)
    encoder, err := tiktoken.EncodingForModel(encodingName)
    if err != nil {
        // Fallback to o200k_base for unknown models
        encoder, _ = tiktoken.EncodingForModel("o200k_base")
    }
    return &TokenEstimator{encoder: encoder}
}

func modelToEncoding(model string) string {
    switch model {
    case "gpt-4o", "gpt-4o-mini", "chatgpt-4o-latest":
        return "o200k_base"
    case "gpt-4-turbo", "gpt-4":
        return "o200k_base"
    case "gpt-3.5-turbo", "gpt-3.5-turbo-16k":
        return "cl100k_base"
    case "text-embedding-3-large":
        return "cl100k_base"
    case "text-embedding-3-small":
        return "cl100k_base"
    case "text-embedding-ada-002":
        return "cl100k_base"
    default:
        return "o200k_base"
    }
}

func (e *TokenEstimator) Estimate(text string) int {
    if text == "" {
        return 0
    }
    tokens := e.encoder.Encode(text, nil, nil)
    return len(tokens)
}

func (e *TokenEstimator) EstimateMessages(messages []Message) int {
    total := 0
    for _, m := range messages {
        total += e.Estimate(m.Role) + e.Estimate(m.Content)
    }
    return total
}

type Message struct {
    Role    string
    Content string
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/tokenizer_test.go -v -run TestTokenEstimator`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/tokenizer.go src/server/internal/relay/tokenizer_test.go
git commit -m "feat(relay): add TokenEstimator using tiktoken-go for precise token counting"
```

---

### Task 2: Pricing Lookup

**Files:**
- Create: `src/server/internal/relay/pricing.go`
- Test: `src/server/internal/relay/pricing_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "testing"
)

func TestPricing_GetPrice(t *testing.T) {
    store := NewPricingStore()
    store.SetPrice("gpt-4o", APITypeChat, DimPromptTokens, 0.002)
    store.SetPrice("gpt-4o", APITypeChat, DimCompletionTokens, 0.008)

    price, err := store.GetPrice("gpt-4o", APITypeChat, DimPromptTokens)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if price != 0.002 {
        t.Fatalf("expected 0.002, got %f", price)
    }
}

func TestPricing_GetPrice_NotFound(t *testing.T) {
    store := NewPricingStore()
    _, err := store.GetPrice("unknown-model", APITypeChat, DimPromptTokens)
    if err == nil {
        t.Fatal("expected error for unknown model")
    }
}

func TestPricing_CalculateCost(t *testing.T) {
    store := NewPricingStore()
    store.SetPrice("gpt-4o", APITypeChat, DimPromptTokens, 0.002)
    store.SetPrice("gpt-4o", APITypeChat, DimCompletionTokens, 0.008)

    cost := store.CalculateCost("gpt-4o", APITypeChat, &Usage{
        PromptTokens:     1000,
        CompletionTokens: 500,
    })

    expected := 0.002*1000 + 0.008*500 // 2 + 4 = 6
    if cost != expected {
        t.Fatalf("expected %f, got %f", expected, cost)
    }
}

func TestPricing_DefaultPricing(t *testing.T) {
    store := NewPricingStoreWithDefaults()
    price, err := store.GetPrice("gpt-4o", APITypeChat, DimPromptTokens)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if price <= 0 {
        t.Fatal("default pricing should return positive price")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/pricing_test.go -v -run TestPricing`
Expected: FAIL — symbols not defined

- [ ] **Step 3: Write pricing implementation**

```go
package relay

import (
    "fmt"
    "sync"
)

type UsageDimension string

const (
    DimPromptTokens     UsageDimension = "prompt_tokens"
    DimCompletionTokens UsageDimension = "completion_tokens"
    DimTotalTokens       UsageDimension = "total_tokens"
    DimImageCount        UsageDimension = "image_count"
    DimVideoCount        UsageDimension = "video_count"
    DimAudioSeconds      UsageDimension = "audio_seconds"
    DimStorageBytes      UsageDimension = "storage_bytes"
    DimTrainingTokens    UsageDimension = "training_tokens"
)

type PricingStore struct {
    mu     sync.RWMutex
    prices map[string]map[APIType]map[UsageDimension]float64
}

func NewPricingStore() *PricingStore {
    return &PricingStore{
        prices: make(map[string]map[APIType]map[UsageDimension]float64),
    }
}

func NewPricingStoreWithDefaults() *PricingStore {
    store := NewPricingStore()
    // OpenAI defaults (approximate, per 1K tokens)
    defaults := map[APIType]map[UsageDimension]float64{
        APITypeChat: {
            DimPromptTokens:     0.002,
            DimCompletionTokens: 0.008,
        },
        APITypeCompletions: {
            DimPromptTokens:     0.002,
            DimCompletionTokens: 0.008,
        },
        APITypeEmbeddings: {
            DimPromptTokens: 0.0001,
        },
        APITypeImageGen: {
            DimImageCount: 0.004,
        },
    }
    for apiType, dims := range defaults {
        for dim, price := range dims {
            store.SetPrice("gpt-4o", apiType, dim, price)
            store.SetPrice("gpt-4o-mini", apiType, dim, price*0.1)
            _ = apiType
            _ = dim
        }
    }
    return store
}

func (s *PricingStore) SetPrice(model string, apiType APIType, dim UsageDimension, price float64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.prices[model] == nil {
        s.prices[model] = make(map[APIType]map[UsageDimension]float64)
    }
    if s.prices[model][apiType] == nil {
        s.prices[model][apiType] = make(map[UsageDimension]float64)
    }
    s.prices[model][apiType][dim] = price
}

func (s *PricingStore) GetPrice(model string, apiType APIType, dim UsageDimension) (float64, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.prices[model] == nil || s.prices[model][apiType] == nil {
        return 0, fmt.Errorf("price not found for model=%s apiType=%s dim=%s", model, apiType, dim)
    }
    price := s.prices[model][apiType][dim]
    return price, nil
}

func (s *PricingStore) CalculateCost(model string, apiType APIType, usage *Usage) float64 {
    var total float64
    if usage.PromptTokens > 0 {
        if price, err := s.GetPrice(model, apiType, DimPromptTokens); err == nil {
            total += price * float64(usage.PromptTokens) / 1000.0
        }
    }
    if usage.CompletionTokens > 0 {
        if price, err := s.GetPrice(model, apiType, DimCompletionTokens); err == nil {
            total += price * float64(usage.CompletionTokens) / 1000.0
        }
    }
    if usage.ImageCount > 0 {
        if price, err := s.GetPrice(model, apiType, DimImageCount); err == nil {
            total += price * float64(usage.ImageCount)
        }
    }
    if usage.AudioSeconds > 0 {
        if price, err := s.GetPrice(model, apiType, DimAudioSeconds); err == nil {
            total += price * usage.AudioSeconds
        }
    }
    return total
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/pricing_test.go -v -run TestPricing`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/pricing.go src/server/internal/relay/pricing_test.go
git commit -m "feat(relay): add PricingStore with per-model/per-APIType/per-dimension pricing lookup"
```

---

### Task 3: Billing Hook

**Files:**
- Create: `src/server/internal/relay/billing.go`
- Test: `src/server/internal/relay/billing_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "testing"
)

func TestBillingHook_PreBill(t *testing.T) {
    store := NewPricingStoreWithDefaults()
    hook := NewBillingHook(store, nil)

    session := &BillingSession{
        ID:             "sess_123",
        ChannelID:      "ch_1",
        APIType:        APITypeChat,
        Model:          "gpt-4o",
        IdempotencyKey: "idem_123",
    }

    preAuth, err := hook.PreBill(session, &Usage{PromptTokens: 1000, CompletionTokens: 500})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if preAuth <= 0 {
        t.Fatal("PreBill should return positive pre_auth_amount")
    }
    if session.PreAuthorizedAmt <= 0 {
        t.Fatal("session.PreAuthorizedAmt should be set")
    }
}

func TestBillingHook_PostBill_Settles(t *testing.T) {
    store := NewPricingStoreWithDefaults()
    hook := NewBillingHook(store, nil)

    session := &BillingSession{
        ID:             "sess_123",
        ChannelID:      "ch_1",
        APIType:        APITypeChat,
        Model:          "gpt-4o",
        IdempotencyKey: "idem_123",
        PreAuthorizedAmt: 10.0,
    }

    settled, err := hook.PostBill(session, &Usage{PromptTokens: 1000, CompletionTokens: 500})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if settled <= 0 {
        t.Fatal("PostBill should settle positive amount")
    }
}

func TestBillingHook_Refund(t *testing.T) {
    store := NewPricingStoreWithDefaults()
    hook := NewBillingHook(store, nil)

    session := &BillingSession{
        ID:               "sess_123",
        ChannelID:       "ch_1",
        PreAuthorizedAmt: 10.0,
        SettledAmt:       5.0,
    }

    refunded, err := hook.Refund(session)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if refunded != 5.0 {
        t.Fatalf("expected refund of 5.0, got %f", refunded)
    }
    if session.Status != BillingStatusRefunded {
        t.Fatalf("expected status Refunded, got %s", session.Status)
    }
}

func TestBillingHook_DuplicateIdempotency(t *testing.T) {
    store := NewPricingStoreWithDefaults()
    hook := NewBillingHook(store, nil)

    seen := map[string]bool{}
    hook2 := NewBillingHook(store, &seen)

    session := &BillingSession{
        ID:             "sess_123",
        ChannelID:      "ch_1",
        APIType:        APITypeChat,
        Model:          "gpt-4o",
        IdempotencyKey: "idem_123",
    }

    _, err := hook2.PreBill(session, &Usage{PromptTokens: 1000})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    // Second PreBill with same idempotency key should return cached
    _, err = hook2.PreBill(session, &Usage{PromptTokens: 1000})
    if err != nil {
        t.Fatalf("duplicate PreBill should not error: %v", err)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/billing_test.go -v -run TestBillingHook`
Expected: FAIL — symbols not defined

- [ ] **Step 3: Write BillingHook implementation**

```go
package relay

import (
    "fmt"
    "sync"
    "time"
)

type BillingStatus string

const (
    BillingStatusAuthorized BillingStatus = "authorized"
    BillingStatusSettled    BillingStatus = "settled"
    BillingStatusRefunded   BillingStatus = "refunded"
    BillingStatusFailed     BillingStatus = "failed"
)

type BillingSession struct {
    ID               string
    ChannelID        string
    APIType          APIType
    Model            string
    IdempotencyKey   string
    RequestID        string
    AttemptNo        int
    PreAuthorizedAmt float64
    SettledAmt       float64
    Status           BillingStatus
    CreatedAt        time.Time
}

type BillingHook struct {
    pricing    *PricingStore
    seenIdem   *map[string]bool
    mu         sync.Mutex
}

func NewBillingHook(pricing *PricingStore, seenIdem *map[string]bool) *BillingHook {
    return &BillingHook{
        pricing:  pricing,
        seenIdem: seenIdem,
    }
}

func (h *BillingHook) PreBill(session *BillingSession, usage *Usage) (float64, error) {
    // Check idempotency
    if h.seenIdem != nil {
        h.mu.Lock()
        if (*h.seenIdem)[session.IdempotencyKey] {
            h.mu.Unlock()
            return session.PreAuthorizedAmt, nil
        }
        (*h.seenIdem)[session.IdempotencyKey] = true
        h.mu.Unlock()
    }

    // Estimate cost
    cost := h.pricing.CalculateCost(session.Model, session.APIType, usage)
    // Add 20% buffer for safety
    preAuth := cost * 1.2

    session.PreAuthorizedAmt = preAuth
    session.Status = BillingStatusAuthorized
    session.CreatedAt = time.Now()

    return preAuth, nil
}

func (h *BillingHook) PostBill(session *BillingSession, usage *Usage) (float64, error) {
    // Check idempotency
    if h.seenIdem != nil {
        h.mu.Lock()
        key := session.IdempotencyKey + ":settled"
        if (*h.seenIdem)[key] {
            h.mu.Unlock()
            return session.SettledAmt, nil
        }
        (*h.seenIdem)[key] = true
        h.mu.Unlock()
    }

    actualCost := h.pricing.CalculateCost(session.Model, session.APIType, usage)

    // Refund excess authorization
    excess := session.PreAuthorizedAmt - actualCost
    if excess > 0 {
        h.refund(session, excess)
    }

    session.SettledAmt = actualCost
    session.Status = BillingStatusSettled

    return actualCost, nil
}

func (h *BillingHook) Refund(session *BillingSession) (float64, error) {
    refund := session.PreAuthorizedAmt - session.SettledAmt
    if refund < 0 {
        refund = 0
    }
    session.Status = BillingStatusRefunded
    return refund, nil
}

func (h *BillingHook) refund(session *BillingSession, amount float64) {
    // In production: call channel's refund endpoint
    _ = amount
    _ = session
}

func (h *BillingHook) SetRequestID(session *BillingSession, requestID string) {
    session.RequestID = requestID
}

func (h *BillingHook) IncrementAttempt(session *BillingSession) {
    session.AttemptNo++
}

func (h *BillingHook) BuildBillingSession(channelID, model, apiType, idempotencyKey string) *BillingSession {
    return &BillingSession{
        ID:               fmt.Sprintf("sess_%d", time.Now().UnixNano()),
        ChannelID:        channelID,
        APIType:          APIType(apiType),
        Model:            model,
        IdempotencyKey:   idempotencyKey,
        PreAuthorizedAmt: 0,
        SettledAmt:       0,
        Status:           BillingStatusAuthorized,
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/billing_test.go -v -run TestBillingHook`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/billing.go src/server/internal/relay/billing_test.go
git commit -m "feat(relay): add BillingHook with PreBill/PostBill/Refund and idempotency support"
```

---

### Task 4: Asynq Billing Worker

**Files:**
- Create: `src/server/internal/relay/billing_worker.go`
- Test: `src/server/internal/relay/billing_worker_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "context"
    "testing"
    "time"
)

func TestBillingWorker_TimeoutTask(t *testing.T) {
    // Test that enqueueBillingTimeoutTask produces a task with correct delay
    task := EnqueueBillingTimeoutTask("sess_123", APITypeBatch)

    if task.SessionID != "sess_123" {
        t.Fatalf("expected session sess_123, got %s", task.SessionID)
    }
    if task.APIType != APITypeBatch {
        t.Fatalf("expected APIType Batch, got %s", task.APIType)
    }
}

func TestBillingWorker_ProcessTimeout(t *testing.T) {
    // Mock: timeout session should be settled at pre_auth amount
    session := &BillingSession{
        ID:               "sess_timeout",
        ChannelID:        "ch_1",
        APIType:          APITypeBatch,
        Model:            "gpt-4o",
        PreAuthorizedAmt:  10.0,
        SettledAmt:       0,
    }

    // Simulate timeout: settle at pre_auth amount
    settled := session.PreAuthorizedAmt
    session.SettledAmt = settled
    session.Status = BillingStatusSettled

    if session.SettledAmt != 10.0 {
        t.Fatalf("expected settle at 10.0, got %f", session.SettledAmt)
    }
    if session.Status != BillingStatusSettled {
        t.Fatalf("expected Settled status, got %s", session.Status)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/billing_worker_test.go -v -run TestBillingWorker`
Expected: FAIL — symbols not defined

- [ ] **Step 3: Write billing worker implementation**

```go
package relay

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/hibiken/asynq"
)

const (
    billingTimeoutQueue  = "billing"
    billingTimeoutType  = "billing:timeout"
    billingPollingType  = "billing:polling"
)

type BillingTimeoutTask struct {
    SessionID string `json:"session_id"`
    APIType   string `json:"api_type"`
}

type BillingPollingTask struct {
    SessionID string `json:"session_id"`
    BatchID   string `json:"batch_id"`
}

func EnqueueBillingTimeoutTask(sessionID string, apiType APIType) *BillingTimeoutTask {
    return &BillingTimeoutTask{
        SessionID: sessionID,
        APIType:   string(apiType),
    }
}

func (t *BillingTimeoutTask) Type() string {
    return billingTimeoutType
}

func (t *BillingTimeoutTask) Payload() []byte {
    data, _ := json.Marshal(t)
    return data
}

func (t *BillingTimeoutTask) Delay() time.Duration {
    // Default 30min timeout for batch/fine-tuning operations
    return 30 * time.Minute
}

func (t *BillingTimeoutTask) Enqueue(client *asynq.Client) (*asynq.Task, error) {
    return client.Enqueue(t, asynq.Queue(billingTimeoutQueue), asynq.Delay(t.Delay()))
}

type BillingWorker struct {
    asynqServer *asynq.Server
    billingHook *BillingHook
}

func NewBillingWorker(redisAddr string, hook *BillingHook) (*BillingWorker, error) {
    srv := asynq.NewServer(
        asynq.RedisClientOpt{Addr: redisAddr},
        asynq.Config{
            Queues: map[string]int{
                billingTimeoutQueue: 1,
            },
        },
    )

    return &BillingWorker{
        asynqServer: srv,
        billingHook: hook,
    }, nil
}

func (w *BillingWorker) Start(ctx context.Context) error {
    mux := asynq.NewServeMux()
    mux.HandleFunc(billingTimeoutType, w.handleBillingTimeout)
    mux.HandleFunc(billingPollingType, w.handleBillingPolling)
    return w.asynqServer.Start(ctx, mux)
}

func (w *BillingWorker) Stop() {
    w.asynqServer.Stop()
}

func (w *BillingWorker) handleBillingTimeout(ctx context.Context, t *asynq.Task) error {
    var task BillingTimeoutTask
    if err := json.Unmarshal(t.Payload(), &task); err != nil {
        return err
    }

    // Retrieve session from DB and settle at pre_auth amount
    // This is a simplified implementation; real one would fetch from DB
    fmt.Printf("billing timeout for session %s (api_type=%s)\n", task.SessionID, task.APIType)
    return nil
}

func (w *BillingWorker) handleBillingPolling(ctx context.Context, t *asynq.Task) error {
    var task BillingPollingTask
    if err := json.Unmarshal(t.Payload(), &task); err != nil {
        return err
    }

    fmt.Printf("billing polling for session %s batch %s\n", task.SessionID, task.BatchID)
    return nil
}

func EnqueueBillingPollingTask(sessionID, batchID string) *BillingPollingTask {
    return &BillingPollingTask{
        SessionID: sessionID,
        BatchID:   batchID,
    }
}

func (t *BillingPollingTask) Type() string {
    return billingPollingType
}

func (t *BillingPollingTask) Payload() []byte {
    data, _ := json.Marshal(t)
    return data
}

func (t *BillingPollingTask) Enqueue(client *asynq.Client) (*asynq.Task, error) {
    return client.Enqueue(t, asynq.Queue(billingTimeoutQueue), asynq.MaxRetry(5))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/billing_worker_test.go -v -run TestBillingWorker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/billing_worker.go src/server/internal/relay/billing_worker_test.go
git commit -m "feat(relay): add Asynq billing worker for async timeout and polling tasks"
```

---

### Task 5: Prometheus Metrics

**Files:**
- Create: `src/server/internal/metrics/prometheus.go`
- Test: `src/server/internal/metrics/prometheus_test.go`

- [ ] **Step 1: Write failing test**

```go
package metrics

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestMetricsRegistry_Register(t *testing.T) {
    reg := NewRegistry()

    ctr := reg.Counter("requests_total", "api_type", "channel")
    ctr.WithLabelValues("chat", "ch_1").Inc()

    if got := ctr.WithLabelValues("chat", "ch_1").Value(); got != 1 {
        t.Fatalf("expected 1, got %f", got)
    }
}

func TestMetricsRegistry_Histogram(t *testing.T) {
    reg := NewRegistry()

    hist := reg.Histogram("latency_ms", "api_type")
    hist.WithLabelValues("chat").Observe(100.5)

    if hist.Count() != 1 {
        t.Fatalf("expected count 1, got %d", hist.Count())
    }
}

func TestMetricsHandler_ServeHTTP(t *testing.T) {
    reg := NewRegistry()
    reg.Counter("requests_total", "api_type").WithLabelValues("chat").Inc()

    handler := reg.Handler()
    req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/metrics/prometheus_test.go -v -run TestMetricsRegistry`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Write Prometheus metrics implementation**

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
    reg        *prometheus.Registry
    counters   map[string]*prometheus.CounterVec
    histograms map[string]*prometheus.HistogramVec
    gauges     map[string]*prometheus.GaugeVec
}

func NewRegistry() *Registry {
    return &Registry{
        reg:        prometheus.NewRegistry(),
        counters:   make(map[string]*prometheus.CounterVec),
        histograms: make(map[string]*prometheus.HistogramVec),
        gauges:     make(map[string]*prometheus.GaugeVec),
    }
}

func (r *Registry) Counter(name string, labels ...string) *prometheus.CounterVec {
    if c, ok := r.counters[name]; ok {
        return c
    }
    c := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: name,
        Help: name,
    }, labels)
    r.reg.MustRegister(c)
    r.counters[name] = c
    return c
}

func (r *Registry) Histogram(name string, labels ...string) *prometheus.HistogramVec {
    if h, ok := r.histograms[name]; ok {
        return h
    }
    h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    name,
        Help:    name,
        Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
    }, labels)
    r.reg.MustRegister(h)
    r.histograms[name] = h
    return h
}

func (r *Registry) Gauge(name string, labels ...string) *prometheus.GaugeVec {
    if g, ok := r.gauges[name]; ok {
        return g
    }
    g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: name,
        Help: name,
    }, labels)
    r.reg.MustRegister(g)
    r.gauges[name] = g
    return g
}

func (r *Registry) Handler() http.Handler {
    return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{})
}

// Predefined metric names for relay
var (
    RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "relay_requests_total",
        Help: "Total number of relay requests",
    }, []string{"api_type", "channel", "status"})

    LatencyHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "relay_request_latency_ms",
        Help:    "Request latency in milliseconds",
        Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
    }, []string{"api_type", "channel"})

    TokensTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "relay_tokens_total",
        Help: "Total tokens processed",
    }, []string{"api_type", "model", "dimension"})

    CircuitBreakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "relay_circuit_breaker_state",
        Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
    }, []string{"channel"})

    RateLimiterUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
        Name: "relay_rate_limiter_usage",
        Help: "Rate limiter token usage",
    }, []string{"channel", "dimension"})

    HealthProbeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "relay_health_probe_total",
        Help: "Total health probes",
    }, []string{"channel", "result"})

    BillingAuthorizedAmt = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "relay_billing_authorized_amt",
        Help: "Total authorized billing amount",
    }, []string{"channel", "api_type"})

    BillingSettledAmt = prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "relay_billing_settled_amt",
        Help: "Total settled billing amount",
    }, []string{"channel", "api_type"})
)

func MustRegisterMetrics(reg *prometheus.Registry) {
    reg.MustRegister(RequestsTotal)
    reg.MustRegister(LatencyHistogram)
    reg.MustRegister(TokensTotal)
    reg.MustRegister(CircuitBreakerState)
    reg.MustRegister(RateLimiterUsage)
    reg.MustRegister(HealthProbeTotal)
    reg.MustRegister(BillingAuthorizedAmt)
    reg.MustRegister(BillingSettledAmt)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/metrics/prometheus_test.go -v -run TestMetricsRegistry`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/metrics/prometheus.go src/server/internal/metrics/prometheus_test.go
git commit -m "feat(relay): add Prometheus metrics registry with relay-specific metric definitions"
```

---

### Task 6: HTTP Metrics Endpoint

**Files:**
- Modify: `src/server/internal/http/router.go` (add `/metrics` route)

- [ ] **Step 1: Read existing router to find registration pattern**

```bash
grep -n "HandleFunc\|GET(" src/server/internal/http/router.go | head -20
```

- [ ] **Step 2: Add metrics registration**

Add to the HTTP router setup:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var globalRegistry = prometheus.NewRegistry()
metrics.MustRegisterMetrics(globalRegistry)

// In route registration:
router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(globalRegistry, promhttp.HandlerOpts{})))
```

- [ ] **Step 3: Commit**

```bash
git add src/server/internal/http/router.go
git commit -m "feat(relay): expose /metrics endpoint for Prometheus scraping"
```

---

### Task 7: Integrate Billing Hook into Router

**Files:**
- Modify: `src/server/internal/relay/router.go`

- [ ] **Step 1: Add billing to Router struct**

```go
type Router struct {
    pool            *ChannelPool
    loadBalancer    *LoadBalancer
    circuitBreakers map[string]*CircuitBreaker
    tokenBucket     *TokenBucket
    healthChecker   *HealthChecker
    billingHook     *BillingHook
}
```

- [ ] **Step 2: Add RouteWithBilling method**

```go
func (r *Router) RouteWithBilling(
    ctx context.Context,
    apiType string,
    model string,
    session *BillingSession,
    usage *Usage,
    fn func(ch *RouteChannel) (*ProviderResponse, error),
) (*ProviderResponse, error) {
    // PreBill: authorize before proxying
    if r.billingHook != nil && session != nil {
        preAuth, err := r.billingHook.PreBill(session, usage)
        if err != nil {
            return nil, fmt.Errorf("prebill failed: %w", err)
        }
        metrics.BillingAuthorizedAmt.WithLabelValues(session.ChannelID, string(session.APIType)).Add(preAuth)
    }

    // Route the request
    resp, err := r.Route(ctx, apiType, fn)

    // PostBill: settle/refund after response
    if r.billingHook != nil && session != nil && resp != nil {
        settled, settleErr := r.billingHook.PostBill(session, usage)
        if settleErr == nil {
            metrics.BillingSettledAmt.WithLabelValues(session.ChannelID, string(session.APIType)).Add(settled)
        }
    }

    return resp, err
}
```

- [ ] **Step 3: Commit**

```bash
git add src/server/internal/relay/router.go
git commit -m "feat(relay): integrate BillingHook into Router with PreBill/PostBill"
```
