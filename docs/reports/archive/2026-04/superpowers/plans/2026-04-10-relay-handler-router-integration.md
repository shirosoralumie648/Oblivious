# Relay Handler-Router Integration Plan

**Goal:** Wire Handler layer to Router layer so requests actually flow through load balancing, circuit breakers, rate limiting, and billing.

**Architecture:** Handlers hold a reference to `*relay.Router` via a package-level setter. Each handler's `executeRequest` calls `router.RouteWithBilling()` instead of returning a stub error.

---

## Task 1: Add global Router reference to handler package

**Files:**
- Modify: `src/server/internal/relay/handler/router.go`

- [ ] **Step 1: Add global router variable and setter**

```go
package handler

var globalRouter *Router

func SetRouter(r *Router) {
    globalRouter = r
}

func GetRouter() *Router {
    return globalRouter
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/relay/handler/...`
Expected: PASS

---

## Task 2: Update Relay to create and wire Router

**Files:**
- Modify: `src/server/internal/relay/relay.go`

- [ ] **Step 1: Add Router field and method to Relay struct**

```go
type Relay struct {
    engine   *gin.Engine
    pool     types.ChannelPoolInterface
    handlers map[types.APIType]types.Handler
    router   *Router
}
```

- [ ] **Step 2: Update NewRelay to create Router with full dependency graph**

```go
func NewRelay(cfg *Config) (*Relay, error) {
    r := &Relay{
        handlers: make(map[types.APIType]types.Handler),
    }
    if cfg != nil && cfg.Pool != nil {
        r.pool = cfg.Pool
    } else {
        r.pool = NewChannelPool()
    }

    // Build dependency chain: pool -> lb -> router
    lb := NewLoadBalancer(r.pool, "weighted")

    // Circuit breakers per channel
    cbs := make(map[string]*CircuitBreaker)
    for _, ch := range r.pool.ListChannels() {
        cbs[ch.ID] = NewCircuitBreaker(CircuitBreakerConfig{
            FailureThreshold: ch.CBThreshold,
            Timeout:          time.Duration(ch.CBTimeout) * time.Second,
        })
    }

    // Token bucket for global rate limit
    tb := NewTokenBucket(1000, 60000) // 1K RPM

    // Health checker
    hc := NewHealthChecker(r.pool, 30*time.Second)

    // Pricing store with defaults
    pricing := NewPricingStoreWithDefaults()

    // Billing hook
    seenIdem := make(map[string]bool)
    billingHook := NewBillingHook(pricing, &seenIdem)

    // Create router
    r.router = NewRouterWithBilling(r.pool, lb, cbs, tb, hc, billingHook, "")

    // Register router with handlers
    handler.SetRouter(r.router)

    r.initRouter()
    return r, nil
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/relay/...`
Expected: PASS (will fail on handler executeRequest calls — expected, fix in Task 3)

---

## Task 3: Update executeRequest in all handlers

Each handler's `executeRequest` currently returns `types.ErrNoAvailableChannel`. Replace with actual router call.

### 3a. Chat Handler

**Files:**
- Modify: `src/server/internal/relay/handler/chat.go`

- [ ] **Step 1: Replace executeRequest stub with router call**

```go
func (h *ChatHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
    router := GetRouter()
    if router == nil {
        return nil, types.ErrNoAvailableChannel
    }

    idempotencyKey := c.GetHeader("Idempotency-Key")
    if idempotencyKey == "" {
        idempotencyKey = fmt.Sprintf("chat_%d", time.Now().UnixNano())
    }

    resp, err := router.RouteWithBilling(
        c.Request.Context(),
        req.APIType,
        req.Model,
        req.ChannelID,
        idempotencyKey,
        usage,
        func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
            // Build upstream request
            upstreamURL, _ := h.adapter.BuildURL(req.Model, req.APIType)
            headers, _ := h.adapter.BuildHeaders(c.Request.Context(), req.Model, req.APIType)

            providerReq := &channel.ProviderRequest{
                APIType:    req.APIType,
                Model:      req.Model,
                URL:        upstreamURL,
                Stream:     req.Stream,
                Messages:   req.Messages,
                MaxTokens:  req.MaxTokens,
                Headers:    headers,
                ChannelID:  ch.Channel.ID,
            }

            return h.doUpstreamRequest(providerReq)
        },
    )
    return resp, err
}

func (h *ChatHandler) doUpstreamRequest(req *channel.ProviderRequest) (*types.ProviderResponse, error) {
    body, err := marshalRequest(req)
    if err != nil {
        return nil, err
    }
    upstreamReq, err := http.NewRequest("POST", req.URL, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    upstreamReq.Header = req.Headers.Clone()
    upstreamReq.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 60 * time.Second}
    resp, err := client.Do(upstreamReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    bodyOut, _ := io.ReadAll(resp.Body)
    return &types.ProviderResponse{
        StatusCode: resp.StatusCode,
        Content:    bodyOut,
    }, nil
}
```

Add missing imports: `"bytes"`, `"fmt"`, `"io"`, `"net/http"`, `"time"`

- [ ] **Step 2: Verify build**

Run: `go build ./internal/relay/handler/...`
Expected: PASS

### 3b. Embeddings Handler

**Files:**
- Modify: `src/server/internal/relay/handler/embeddings.go`

- [ ] **Step 1: Replace executeRequest stub**

```go
func (h *EmbeddingsHandler) executeRequest(c *gin.Context, req *channel.ProviderRequest, usage *types.Usage) (*types.ProviderResponse, error) {
    router := GetRouter()
    if router == nil {
        return nil, types.ErrNoAvailableChannel
    }
    idempotencyKey := c.GetHeader("Idempotency-Key")
    if idempotencyKey == "" {
        idempotencyKey = fmt.Sprintf("emb_%d", time.Now().UnixNano())
    }
    resp, err := router.RouteWithBilling(
        c.Request.Context(),
        req.APIType,
        req.Model,
        req.ChannelID,
        idempotencyKey,
        usage,
        func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
            return h.doUpstreamRequest(req, ch)
        },
    )
    return resp, err
}
```

- [ ] **Step 2: Add doUpstreamRequest and wire up to passthroughHelper**

Run: `go build ./internal/relay/handler/...`
Expected: PASS

### 3c. Passthrough Handlers (assistants, batch, files, fine_tuning, images, audio, moderations, completions)

These use `passthroughHelper` directly and don't go through `executeRequest`. For StrategyPassthrough routes, modify `passthroughHelper` to accept a router and route through it.

**Files:**
- Modify: `src/server/internal/relay/handler/common.go`
- Modify: each passthrough handler file

- [ ] **Step 1: Add routing-aware passthroughHelper**

```go
func passthroughHelper(c *gin.Context, adapter *channel.OpenAIAdapter, method, path string, body []byte, apiType types.APIType) {
    router := GetRouter()
    model := "gpt-4o"
    upstreamURL, _ := adapter.BuildURL(model, apiType)
    upstreamURL = upstreamURL + path
    headers, _ := adapter.BuildHeaders(c.Request.Context(), model, apiType)

    if router != nil {
        idempotencyKey := c.GetHeader("Idempotency-Key")
        if idempotencyKey == "" {
            idempotencyKey = fmt.Sprintf("pass_%d", time.Now().UnixNano())
        }
        resp, err := router.RouteWithBilling(
            c.Request.Context(),
            apiType,
            model,
            "", // passthrough doesn't specify channel
            idempotencyKey,
            &types.Usage{},
            func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
                return doPassthroughRequest(method, upstreamURL, headers, body)
            },
        )
        if err != nil {
            c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
            return
        }
        c.Data(resp.StatusCode, "application/json", resp.Content)
        return
    }

    // Fallback: direct passthrough without routing
    req, err := http.NewRequest(method, upstreamURL, bytes.NewReader(body))
    if err != nil {
        c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
        return
    }
    req.Header = headers
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"code": "upstream_error", "message": err.Error()}})
        return
    }
    defer resp.Body.Close()

    bodyOut, _ := io.ReadAll(resp.Body)
    c.Data(resp.StatusCode, "application/json", bodyOut)
}

func doPassthroughRequest(method, url string, headers http.Header, body []byte) (*types.ProviderResponse, error) {
    req, err := http.NewRequest(method, url, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header = headers.Clone()
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    bodyOut, _ := io.ReadAll(resp.Body)
    return &types.ProviderResponse{StatusCode: resp.StatusCode, Content: bodyOut}, nil
}
```

- [ ] **Step 2: Add missing imports to common.go**

Add `"fmt"` to the imports.

- [ ] **Step 3: Verify build**

Run: `go build ./internal/relay/handler/...`
Expected: PASS

### 3d. Realtime Handler

**Files:**
- Modify: `src/server/internal/relay/handler/realtime.go`

- [ ] **Step 1: Wire realtime through router if possible, otherwise keep as-is (WebSocket upgrade is special)**

Run: `go build ./internal/relay/handler/...`
Expected: PASS

---

## Task 4: Run full test suite

- [ ] **Step 1: Run all tests**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 2: Fix any failures**

---

## Task 5: Commit
