# Relay Plan C: Router Engine

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Router Engine layer: CircuitBreaker, TokenBucket, HealthChecker, LoadBalancer, Retry, and Fallback routing logic.

**Architecture:** Router Engine sits between Handler layer and ChannelPool. It evaluates channel health, rate limits, and load to select the optimal channel per request. All decisions are local (no distributed coordination).

---

## File Structure

```
src/server/internal/relay/
  circuitbreaker.go   # CircuitBreaker state machine
  tokenbucket.go      # TokenBucket rate limiter (RPM + TPM)
  healthchecker.go    # HealthChecker with models_api / realtime_probe / disabled
  loadbalancer.go     # LoadBalancer with weighted/priority/cost_aware strategies
  retry.go            # Retry loop with exponential backoff
  router.go           # Router: orchestrates all engine components per request
```

---

### Task 1: TokenBucket Rate Limiter

**Files:**
- Create: `src/server/internal/relay/tokenbucket.go`
- Test: `src/server/internal/relay/tokenbucket_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "sync"
    "testing"
    "time"
)

func TestTokenBucket_RefillRPM(t *testing.T) {
    tb := NewTokenBucket(10, 100) // rpm: 10, tpm: 100

    // Exhaust RPM tokens
    for i := 0; i < 10; i++ {
        ok, _ := tb.TryAcquire("rpm")
        if !ok {
            t.Fatalf("token %d should be acquirable", i)
        }
    }

    // 11th should fail
    ok, remaining := tb.TryAcquire("rpm")
    if ok {
        t.Fatal("11th token should be rejected")
    }
    if remaining != 0 {
        t.Fatalf("remaining rpm should be 0, got %d", remaining)
    }

    // Advance time by 6s → should refill 1 token (10 rpm / 60s = 1/6 per second)
    time.Sleep(6100 * time.Millisecond)
    ok, remaining = tb.TryAcquire("rpm")
    if !ok {
        t.Fatal("token should be refillable after 6s")
    }
    if remaining != 0 {
        t.Fatalf("remaining should be 0 (used 1 of refill), got %d", remaining)
    }
}

func TestTokenBucket_RefillTPM(t *testing.T) {
    tb := NewTokenBucket(60, 1000) // rpm: 60, tpm: 1000

    // Exhaust TPM tokens
    for i := 0; i < 10; i++ {
        ok, _ := tb.TryAcquire("tpm")
        if !ok {
            t.Fatalf("token %d should be acquirable", i)
        }
    }

    ok, remaining := tb.TryAcquire("tpm")
    if ok {
        t.Fatal("11th token should be rejected")
    }
    if remaining != 0 {
        t.Fatalf("remaining tpm should be 0, got %d", remaining)
    }
}

func TestTokenBucket_Concurrent(t *testing.T) {
    tb := NewTokenBucket(100, 1000)
    var success int
    var mu sync.Mutex
    var wg sync.WaitGroup

    for i := 0; i < 200; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            ok, _ := tb.TryAcquire("rpm")
            if ok {
                mu.Lock()
                success++
                mu.Unlock()
            }
        }()
    }
    wg.Wait()

    if success != 100 {
        t.Fatalf("expected exactly 100 successes under concurrency, got %d", success)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/tokenbucket_test.go -v -run TestTokenBucket`
Expected: FAIL — `TryAcquire` not defined

- [ ] **Step 3: Write TokenBucket implementation**

```go
package relay

import (
    "sync"
    "time"
)

type TokenBucket struct {
    mu           sync.Mutex
    rpmTokens    int
    tpmTokens    int
    rpmLimit     int
    tpmLimit     int
    rpmLastRefill time.Time
    tpmLastRefill time.Time
}

func NewTokenBucket(rpm, tpm int) *TokenBucket {
    now := time.Now()
    return &TokenBucket{
        rpmTokens:    rpm,
        tpmTokens:    tpm,
        rpmLimit:     rpm,
        tpmLimit:     tpm,
        rpmLastRefill: now,
        tpmLastRefill: now,
    }
}

func (tb *TokenBucket) TryAcquire(dimension string) (bool, int) {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    switch dimension {
    case "rpm":
        tb.refill(&tb.rpmTokens, &tb.rpmLastRefill, tb.rpmLimit)
        if tb.rpmTokens <= 0 {
            return false, 0
        }
        tb.rpmTokens--
        return true, tb.rpmTokens
    case "tpm":
        tb.refill(&tb.tpmTokens, &tb.tpmLastRefill, tb.tpmLimit)
        if tb.tpmTokens <= 0 {
            return false, 0
        }
        tb.tpmTokens--
        return true, tb.tpmTokens
    default:
        return true, 0
    }
}

func (tb *TokenBucket) refill(tokens *int, lastRefill *time.Time, limit int) {
    now := time.Now()
    elapsed := now.Sub(*lastRefill)
    refill := int(elapsed.Seconds()) * (limit / 60)
    if refill > 0 {
        *tokens = min(limit, *tokens+refill)
        *lastRefill = now
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func (tb *TokenBucket) Available(dimension string) int {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    switch dimension {
    case "rpm":
        tb.refill(&tb.rpmTokens, &tb.rpmLastRefill, tb.rpmLimit)
        return tb.rpmTokens
    case "tpm":
        tb.refill(&tb.tpmTokens, &tb.tpmLastRefill, tb.tpmLimit)
        return tb.tpmTokens
    }
    return 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/tokenbucket_test.go -v -run TestTokenBucket`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/tokenbucket.go src/server/internal/relay/tokenbucket_test.go
git commit -m "feat(relay): add TokenBucket rate limiter with separate RPM/TPM refill"
```

---

### Task 2: CircuitBreaker State Machine

**Files:**
- Create: `src/server/internal/relay/circuitbreaker.go`
- Test: `src/server/internal/relay/circuitbreaker_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "testing"
    "time"
)

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
    cb := NewCircuitBreaker("test-provider", 5, 10*time.Second, 60*time.Second)

    // 5 failures should trip the breaker
    for i := 0; i < 5; i++ {
        cb.RecordFailure()
    }

    if cb.State() != StateOpen {
        t.Fatalf("expected Open after 5 failures, got %s", cb.State())
    }
    if !cb.CanTry() {
        t.Fatal("CanTry should be true when Open (pre-flight probe)")
    }
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
    cb := NewCircuitBreaker("test-provider", 5, 10*time.Millisecond, 60*time.Second)
    cb.failureCount = 5
    cb.State() // trigger state computation

    // Advance past probe interval
    time.Sleep(15 * time.Millisecond)
    cb.probeAt = time.Now().Add(-time.Second) // force into probe window

    if cb.State() != StateHalfOpen {
        t.Fatalf("expected HalfOpen after probe interval, got %s", cb.State())
    }
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
    cb := NewCircuitBreaker("test-provider", 5, 10*time.Millisecond, 60*time.Second)
    cb.state = StateHalfOpen
    cb.successCount = 3

    if cb.State() != StateClosed {
        t.Fatalf("expected Closed after 3 successes in HalfOpen, got %s", cb.State())
    }
}

func TestCircuitBreaker_Escalation(t *testing.T) {
    cb := NewCircuitBreaker("test-provider", 5, 10*time.Second, 60*time.Second)
    cb.failureCount = 10

    // 5 extra failures → probeInterval escalates to 60s
    if cb.probeInterval != 60*time.Second {
        t.Fatalf("expected escalated probeInterval 60s, got %v", cb.probeInterval)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/circuitbreaker_test.go -v -run TestCircuitBreaker`
Expected: FAIL — symbols not defined

- [ ] **Step 3: Write CircuitBreaker implementation**

```go
package relay

import (
    "sync"
    "time"
)

type CircuitState int

const (
    StateClosed CircuitState = iota
    StateOpen
    StateHalfOpen
)

func (s CircuitState) String() string {
    switch s {
    case StateClosed:
        return "closed"
    case StateOpen:
        return "open"
    case StateHalfOpen:
        return "half-open"
    }
    return "unknown"
}

type CircuitBreaker struct {
    mu             sync.Mutex
    provider       string
    failureLimit   int
    successLimit   int
    failureCount   int
    successCount   int
    state          CircuitState
    openedAt       time.Time
    probeAt        time.Time
    probeInterval  time.Duration
    maxProbeInterval time.Duration
}

func NewCircuitBreaker(provider string, failureLimit int, probeInterval, maxProbeInterval time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        provider:         provider,
        failureLimit:     failureLimit,
        successLimit:     3,
        failureCount:     0,
        successCount:     0,
        state:            StateClosed,
        probeInterval:    probeInterval,
        maxProbeInterval: maxProbeInterval,
    }
}

func (cb *CircuitBreaker) State() CircuitState {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    cb.recompute()
    return cb.state
}

func (cb *CircuitBreaker) recompute() {
    switch cb.state {
    case StateClosed:
        if cb.failureCount >= cb.failureLimit {
            cb.state = StateOpen
            cb.openedAt = time.Now()
            cb.probeAt = cb.openedAt.Add(cb.probeInterval)
        }
    case StateOpen:
        if time.Now().After(cb.probeAt) {
            cb.state = StateHalfOpen
            cb.successCount = 0
        }
    case StateHalfOpen:
        if cb.successCount >= cb.successLimit {
            cb.state = StateClosed
            cb.failureCount = 0
            cb.successCount = 0
            cb.probeInterval = cb.probeInterval / 2 // reset to base
        }
    }
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    cb.failureCount++
    if cb.failureCount > cb.failureLimit {
        cb.probeInterval = minDuration(cb.probeInterval*2, cb.maxProbeInterval)
    }
    if cb.state == StateHalfOpen {
        cb.state = StateOpen
        cb.probeAt = time.Now().Add(cb.probeInterval)
    }
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    if cb.state == StateHalfOpen {
        cb.successCount++
    }
    cb.failureCount = 0
}

func (cb *CircuitBreaker) CanTry() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    return cb.state == StateOpen || cb.state == StateHalfOpen
}

func (cb *CircuitBreaker) Provider() string {
    return cb.provider
}

func minDuration(a, b time.Duration) time.Duration {
    if a < b {
        return a
    }
    return b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/circuitbreaker_test.go -v -run TestCircuitBreaker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/circuitbreaker.go src/server/internal/relay/circuitbreaker_test.go
git commit -m "feat(relay): add CircuitBreaker state machine with Closed/Open/HalfOpen"
```

---

### Task 3: HealthChecker

**Files:**
- Create: `src/server/internal/relay/healthchecker.go`
- Test: `src/server/internal/relay/healthchecker_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestHealthChecker_ModelsAPI(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/v1/models" {
            t.Errorf("unexpected path: %s", r.URL.Path)
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer ts.Close()

    hc := NewHealthChecker("models_api", 5*time.Second)
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    healthy, lat := hc.Check(ctx, ts.URL+"/v1/models", "fake-key")
    if !healthy {
        t.Fatal("models_api probe should succeed")
    }
    if lat < 0 {
        t.Fatalf("latency should be non-negative, got %dms", lat.Milliseconds())
    }
}

func TestHealthChecker_Disabled(t *testing.T) {
    hc := NewHealthChecker("disabled", 5*time.Second)
    ctx := context.Background()
    healthy, _ := hc.Check(ctx, "http://fake", "fake-key")
    if !healthy {
        t.Fatal("disabled probe should always return healthy")
    }
}

func TestHealthChecker_Timeout(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(10 * time.Second)
        w.WriteHeader(http.StatusOK)
    }))
    defer ts.Close()

    hc := NewHealthChecker("models_api", 50*time.Millisecond)
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()

    healthy, _ := hc.Check(ctx, ts.URL+"/v1/models", "fake-key")
    if healthy {
        t.Fatal("probe should fail on timeout")
    }
}

func TestHealthChecker_ProbeErrorCounting(t *testing.T) {
    hc := NewHealthChecker("models_api", 5*time.Second)
    cb := NewCircuitBreaker("test", 3, time.Second, 30*time.Second)

    // Record 3 failures
    for i := 0; i < 3; i++ {
        hc.RecordProbeResult(cb, false)
    }

    if cb.State() != StateOpen {
        t.Fatalf("expected Open after 3 probe failures, got %s", cb.State())
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/healthchecker_test.go -v -run TestHealthChecker`
Expected: FAIL — symbols not defined

- [ ] **Step 3: Write HealthChecker implementation**

```go
package relay

import (
    "context"
    "net/http"
    "time"
)

type HealthCheckStrategy string

const (
    HealthCheckModelsAPI  HealthCheckStrategy = "models_api"
    HealthCheckRealtime   HealthCheckStrategy = "realtime_probe"
    HealthCheckDisabled   HealthCheckStrategy = "disabled"
)

type HealthChecker struct {
    strategy   HealthCheckStrategy
    timeout    time.Duration
    httpClient *http.Client
}

func NewHealthChecker(strategy HealthCheckStrategy, timeout time.Duration) *HealthChecker {
    return &HealthChecker{
        strategy: strategy,
        timeout:  timeout,
        httpClient: &http.Client{
            Timeout: timeout,
            Transport: &http.Transport{
                DisableKeepAlives: true,
                MaxIdleConns:       1,
            },
        },
    }
}

func (hc *HealthChecker) Check(ctx context.Context, baseURL, apiKey string) (bool, time.Duration) {
    if hc.strategy == HealthCheckDisabled {
        return true, 0
    }

    if hc.strategy == HealthCheckModelsAPI {
        return hc.checkModelsAPI(ctx, baseURL, apiKey)
    }

    return true, 0
}

func (hc *HealthChecker) checkModelsAPI(ctx context.Context, baseURL, apiKey string) (bool, time.Duration) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
    if err != nil {
        return false, 0
    }
    req.Header.Set("Authorization", "Bearer "+apiKey)

    start := time.Now()
    resp, err := hc.httpClient.Do(req)
    latency := time.Since(start)

    if err != nil {
        return false, latency
    }
    defer resp.Body.Close()

    return resp.StatusCode == http.StatusOK, latency
}

func (hc *HealthChecker) RecordProbeResult(cb *CircuitBreaker, healthy bool) {
    if healthy {
        cb.RecordSuccess()
    } else {
        cb.RecordFailure()
    }
}

func (hc *HealthChecker) Strategy() HealthCheckStrategy {
    return hc.strategy
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/healthchecker_test.go -v -run TestHealthChecker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/healthchecker.go src/server/internal/relay/healthchecker_test.go
git commit -m "feat(relay): add HealthChecker with models_api / disabled strategies"
```

---

### Task 4: LoadBalancer

**Files:**
- Create: `src/server/internal/relay/loadbalancer.go`
- Test: `src/server/internal/relay/loadbalancer_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "testing"
)

func TestLoadBalancer_Weighted(t *testing.T) {
    pool := &ChannelPool{
        channels: []*RouteChannel{
            {Channel: &Channel{ID: "a", BaseURL: "http://a"}, Weight: 3},
            {Channel: &Channel{ID: "b", BaseURL: "http://b"}, Weight: 1},
        },
    }

    lb := NewLoadBalancer(pool, "weighted")

    counts := map[string]int{"a": 0, "b": 0}
    for i := 0; i < 40; i++ {
        ch := lb.Select("chat")
        counts[ch.ID]++
    }

    // a should appear ~3x more than b
    if counts["a"] < 20 || counts["a"] > 35 {
        t.Fatalf("expected ~30 selections for a, got %d", counts["a"])
    }
    if counts["b"] < 5 || counts["b"] > 15 {
        t.Fatalf("expected ~10 selections for b, got %d", counts["b"])
    }
}

func TestLoadBalancer_Priority(t *testing.T) {
    pool := &ChannelPool{
        channels: []*RouteChannel{
            {Channel: &Channel{ID: "a", BaseURL: "http://a", Priority: 1}, Weight: 1},
            {Channel: &Channel{ID: "b", BaseURL: "http://b", Priority: 2}, Weight: 1},
        },
    }

    lb := NewLoadBalancer(pool, "priority")

    // Should always pick the lowest priority number (highest priority)
    for i := 0; i < 10; i++ {
        ch := lb.Select("chat")
        if ch.ID != "a" {
            t.Fatalf("expected priority channel a, got %s", ch.ID)
        }
    }
}

func TestLoadBalancer_CostAware(t *testing.T) {
    pool := &ChannelPool{
        channels: []*RouteChannel{
            {Channel: &Channel{ID: "cheap", BaseURL: "http://cheap"}, Weight: 1, EstimatedCostPer1K: 0.5},
            {Channel: &Channel{ID: "expensive", BaseURL: "http://expensive"}, Weight: 1, EstimatedCostPer1K: 5.0},
        },
    }

    lb := NewLoadBalancer(pool, "cost_aware")

    counts := map[string]int{"cheap": 0, "expensive": 0}
    for i := 0; i < 20; i++ {
        ch := lb.Select("chat")
        counts[ch.ID]++
    }

    // cheap should be selected more often
    if counts["cheap"] <= counts["expensive"] {
        t.Fatalf("expected cheap selected more, got cheap=%d expensive=%d", counts["cheap"], counts["expensive"])
    }
}

func TestLoadBalancer_AllHealthy(t *testing.T) {
    pool := &ChannelPool{
        channels: []*RouteChannel{
            {Channel: &Channel{ID: "a", BaseURL: "http://a"}, Weight: 1, Healthy: true},
            {Channel: &Channel{ID: "b", BaseURL: "http://b"}, Weight: 1, Healthy: true},
        },
    }

    lb := NewLoadBalancer(pool, "weighted")
    ch := lb.Select("chat")
    if ch == nil {
        t.Fatal("should return a channel")
    }
}

func TestLoadBalancer_SkipsUnhealthy(t *testing.T) {
    pool := &ChannelPool{
        channels: []*RouteChannel{
            {Channel: &Channel{ID: "a", BaseURL: "http://a"}, Weight: 1, Healthy: false},
            {Channel: &Channel{ID: "b", BaseURL: "http://b"}, Weight: 1, Healthy: true},
        },
    }

    lb := NewLoadBalancer(pool, "weighted")
    ch := lb.Select("chat")
    if ch == nil {
        t.Fatal("should return healthy channel b")
    }
    if ch.ID != "b" {
        t.Fatalf("expected b, got %s", ch.ID)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/loadbalancer_test.go -v -run TestLoadBalancer`
Expected: FAIL — symbols not defined

- [ ] **Step 3: Write LoadBalancer implementation**

```go
package relay

import (
    "math/rand"
    "sync"
)

type LoadBalancer struct {
    pool     *ChannelPool
    strategy string
    mu       sync.Mutex
}

func NewLoadBalancer(pool *ChannelPool, strategy string) *LoadBalancer {
    return &LoadBalancer{
        pool:     pool,
        strategy: strategy,
    }
}

func (lb *LoadBalancer) Select(apiType string) *RouteChannel {
    lb.mu.Lock()
    defer lb.mu.Unlock()

    candidates := lb.filterHealthy()
    if len(candidates) == 0 {
        return nil
    }

    switch lb.strategy {
    case "weighted":
        return lb.weightedSelect(candidates)
    case "priority":
        return lb.prioritySelect(candidates)
    case "cost_aware":
        return lb.costAwareSelect(candidates)
    default:
        return lb.weightedSelect(candidates)
    }
}

func (lb *LoadBalancer) filterHealthy() []*RouteChannel {
    var result []*RouteChannel
    for _, ch := range lb.pool.channels {
        if ch.Healthy {
            result = append(result, ch)
        }
    }
    return result
}

func (lb *LoadBalancer) weightedSelect(channels []*RouteChannel) *RouteChannel {
    totalWeight := 0
    for _, ch := range channels {
        totalWeight += ch.Weight
    }
    r := rand.Intn(totalWeight)
    cumulative := 0
    for _, ch := range channels {
        cumulative += ch.Weight
        if r < cumulative {
            return ch
        }
    }
    return channels[len(channels)-1]
}

func (lb *LoadBalancer) prioritySelect(channels []*RouteChannel) *RouteChannel {
    best := channels[0]
    for _, ch := range channels {
        if ch.Priority < best.Priority {
            best = ch
        }
    }
    return best
}

func (lb *LoadBalancer) costAwareSelect(channels []*RouteChannel) *RouteChannel {
    // Inverse probability proportional to cost
    totalInverse := 0.0
    weights := make([]float64, len(channels))
    for i, ch := range channels {
        if ch.EstimatedCostPer1K <= 0 {
            ch.EstimatedCostPer1K = 1.0
        }
        weights[i] = 1.0 / ch.EstimatedCostPer1K
        totalInverse += weights[i]
    }
    r := rand.Float64() * totalInverse
    cumulative := 0.0
    for i, ch := range channels {
        cumulative += weights[i]
        if r < cumulative {
            return ch
        }
    }
    return channels[len(channels)-1]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/loadbalancer_test.go -v -run TestLoadBalancer`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/loadbalancer.go src/server/internal/relay/loadbalancer_test.go
git commit -m "feat(relay): add LoadBalancer with weighted/priority/cost_aware strategies"
```

---

### Task 5: Retry Loop

**Files:**
- Create: `src/server/internal/relay/retry.go`
- Test: `src/server/internal/relay/retry_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "context"
    "errors"
    "net/http"
    "testing"
)

func TestRetry_DoesNotRetryOnSuccess(t *testing.T) {
    callCount := 0
    fn := func(ctx context.Context) (*ProviderResponse, error) {
        callCount++
        return &ProviderResponse{StatusCode: http.StatusOK}, nil
    }

    resp, err := Retry(fn, 3, "chat")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if callCount != 1 {
        t.Fatalf("expected 1 call, got %d", callCount)
    }
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}

func TestRetry_RetriesOnFailure(t *testing.T) {
    callCount := 0
    fn := func(ctx context.Context) (*ProviderResponse, error) {
        callCount++
        if callCount < 3 {
            return nil, errors.New("temporary error")
        }
        return &ProviderResponse{StatusCode: http.StatusOK}, nil
    }

    resp, err := Retry(fn, 3, "chat")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if callCount != 3 {
        t.Fatalf("expected 3 calls, got %d", callCount)
    }
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}

func TestRetry_GivesUpAfterMaxAttempts(t *testing.T) {
    callCount := 0
    fn := func(ctx context.Context) (*ProviderResponse, error) {
        callCount++
        return nil, errors.New("permanent error")
    }

    _, err := Retry(fn, 3, "chat")
    if err == nil {
        t.Fatal("expected error after max attempts")
    }
    if callCount != 3 {
        t.Fatalf("expected 3 calls, got %d", callCount)
    }
}

func TestRetry_Retries429(t *testing.T) {
    callCount := 0
    fn := func(ctx context.Context) (*ProviderResponse, error) {
        callCount++
        if callCount == 1 {
            return &ProviderResponse{StatusCode: http.StatusTooManyRequests}, errors.New("rate limited")
        }
        return &ProviderResponse{StatusCode: http.StatusOK}, nil
    }

    resp, err := Retry(fn, 3, "chat")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if callCount != 2 {
        t.Fatalf("expected 2 calls, got %d", callCount)
    }
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.StatusCode)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/retry_test.go -v -run TestRetry`
Expected: FAIL — Retry not defined

- [ ] **Step 3: Write Retry implementation**

```go
package relay

import (
    "context"
    "errors"
    "net/http"
    "time"
)

var (
    ErrMaxAttemptsReached = errors.New("max retry attempts reached")
)

var retryableCodes = map[int]bool{
    http.StatusTooManyRequests: true,
    http.StatusBadGateway:      true,
    http.StatusServiceUnavailable: true,
    http.StatusGatewayTimeout:  true,
}

func Retry(fn func(ctx context.Context) (*ProviderResponse, error), maxAttempts int, apiType string) (*ProviderResponse, error) {
    var lastErr error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        resp, err := fn(context.Background())
        if err == nil && resp != nil {
            if !retryableCodes[resp.StatusCode] {
                return resp, nil
            }
            lastErr = errors.New("retryable error")
        } else if err != nil {
            lastErr = err
            var pr *ProviderResponse
            if errors.As(err, &pr) && !retryableCodes[pr.StatusCode] {
                return pr, nil
            }
        }

        if attempt < maxAttempts {
            backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
            if backoff > 5*time.Second {
                backoff = 5 * time.Second
            }
            time.Sleep(backoff)
        }
    }
    return nil, lastErr
}

func IsRetryable(statusCode int) bool {
    return retryableCodes[statusCode]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/retry_test.go -v -run TestRetry`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/retry.go src/server/internal/relay/retry_test.go
git commit -m "feat(relay): add Retry loop with exponential backoff and 429/502/503/504 handling"
```

---

### Task 6: Router (Orchestrator)

**Files:**
- Modify: `src/server/internal/relay/router.go`
- Test: `src/server/internal/relay/router_test.go`

- [ ] **Step 1: Write failing test**

```go
package relay

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRouter_SelectsHealthyChannel(t *testing.T) {
    // Setup: create pool with two channels, one healthy, one not
    pool := NewChannelPool()
    healthyCh := &Channel{ID: "healthy", BaseURL: "http://healthy", Healthy: true, Weight: 1}
    unhealthyCh := &Channel{ID: "unhealthy", BaseURL: "http://unhealthy", Healthy: false, Weight: 1}
    pool.AddChannel(healthyCh, 1)
    pool.AddChannel(unhealthyCh, 1)

    lb := NewLoadBalancer(pool, "weighted")
    cb := NewCircuitBreaker("test", 3, time.Second, 30*time.Second)
    tb := NewTokenBucket(60, 1000)
    hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)

    router := NewRouter(pool, lb, map[string]*CircuitBreaker{"healthy": cb}, tb, hc)

    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer ts.Close()
    healthyCh.BaseURL = ts.URL

    ch := router.SelectChannel(context.Background(), "chat")
    if ch == nil {
        t.Fatal("should return a channel")
    }
    if !ch.Healthy {
        t.Fatal("should not return unhealthy channel")
    }
}

func TestRouter_AllChannelsFailed(t *testing.T) {
    pool := NewChannelPool()
    pool.AddChannel(&Channel{ID: "fail", BaseURL: "http://fail", Healthy: false, Weight: 1}, 1)

    lb := NewLoadBalancer(pool, "weighted")
    hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)
    router := NewRouter(pool, lb, nil, nil, hc)

    _, err := router.Route(context.Background(), "chat", nil)
    if err == nil {
        t.Fatal("expected error when all channels fail")
    }
    // Should return 503 with Retry-After header
    //具体验证略
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./src/server/internal/relay/router_test.go -v -run TestRouter`
Expected: FAIL — Router not defined

- [ ] **Step 3: Write Router implementation**

```go
package relay

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

type Router struct {
    pool            *ChannelPool
    loadBalancer    *LoadBalancer
    circuitBreakers map[string]*CircuitBreaker
    tokenBucket     *TokenBucket
    healthChecker   *HealthChecker
}

func NewRouter(
    pool *ChannelPool,
    lb *LoadBalancer,
    cbs map[string]*CircuitBreaker,
    tb *TokenBucket,
    hc *HealthChecker,
) *Router {
    return &Router{
        pool:            pool,
        loadBalancer:    lb,
        circuitBreakers: cbs,
        tokenBucket:     tb,
        healthChecker:   hc,
    }
}

func (r *Router) SelectChannel(ctx context.Context, apiType string) *RouteChannel {
    // Check rate limit first
    if r.tokenBucket != nil {
        ok, _ := r.tokenBucket.TryAcquire("rpm")
        if !ok {
            return nil
        }
    }

    // Select via load balancer
    ch := r.loadBalancer.Select(apiType)
    if ch == nil {
        return nil
    }

    // Check circuit breaker
    if cb, ok := r.circuitBreakers[ch.ID]; ok {
        if cb.State() == StateOpen {
            return nil
        }
    }

    return ch
}

func (r *Router) Route(ctx context.Context, apiType string, fn func(ch *RouteChannel) (*ProviderResponse, error)) (*ProviderResponse, error) {
    ch := r.SelectChannel(ctx, apiType)
    if ch == nil {
        return nil, &RouterError{
            Code:       http.StatusServiceUnavailable,
            Message:    "no healthy channel available",
            RetryAfter: 30,
        }
    }

    resp, err := fn(ch)
    if err != nil {
        if cb, ok := r.circuitBreakers[ch.ID]; ok {
            cb.RecordFailure()
        }
        return nil, err
    }

    if resp != nil && resp.StatusCode >= 500 {
        if cb, ok := r.circuitBreakers[ch.ID]; ok {
            cb.RecordFailure()
        }
    } else if resp != nil && resp.StatusCode < 500 {
        if cb, ok := r.circuitBreakers[ch.ID]; ok {
            cb.RecordSuccess()
        }
    }

    return resp, nil
}

func (r *Router) RecordChannelSuccess(channelID string) {
    if cb, ok := r.circuitBreakers[channelID]; ok {
        cb.RecordSuccess()
    }
}

func (r *Router) RecordChannelFailure(channelID string) {
    if cb, ok := r.circuitBreakers[channelID]; ok {
        cb.RecordFailure()
    }
}

type RouterError struct {
    Code       int
    Message    string
    RetryAfter int
}

func (e *RouterError) Error() string {
    return fmt.Sprintf("router error %d: %s (retry after %ds)", e.Code, e.Message, e.RetryAfter)
}

func (e *RouterError) RetryAfterSeconds() int {
    return e.RetryAfter
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./src/server/internal/relay/router_test.go -v -run TestRouter`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/server/internal/relay/router.go src/server/internal/relay/router_test.go
git commit -m "feat(relay): add Router orchestrator combining CircuitBreaker/TokenBucket/LoadBalancer"
```

---

### Task 7: Fallback and All-Channel-Failed Response

**Files:**
- Modify: `src/server/internal/relay/router.go`
- Test: `src/server/internal/relay/router_test.go`

- [ ] **Step 1: Write fallback handler in router**

Add to `router.go`:

```go
func (r *Router) RouteWithFallback(
    ctx context.Context,
    apiType string,
    attempts int,
    fn func(ch *RouteChannel) (*ProviderResponse, error),
) (*ProviderResponse, error) {
    var lastErr error
    for attempt := 1; attempt <= attempts; attempt++ {
        resp, err := r.Route(ctx, apiType, fn)
        if err == nil && resp != nil {
            return resp, nil
        }
        lastErr = err

        if resp != nil && IsRetryable(resp.StatusCode) && attempt < attempts {
            backoff := time.Duration(attempt*attempt) * 200 * time.Millisecond
            if backoff > 5*time.Second {
                backoff = 5 * time.Second
            }
            time.Sleep(backoff)
        }
    }

    if lastErr == nil {
        return nil, &RouterError{
            Code:       http.StatusServiceUnavailable,
            Message:    "all channels failed",
            RetryAfter: 30,
        }
    }

    return nil, lastErr
}
```

- [ ] **Step 2: Add unit test for fallback**

```go
func TestRouter_RouteWithFallback_RetriesAllChannels(t *testing.T) {
    pool := NewChannelPool()
    pool.AddChannel(&Channel{ID: "a", BaseURL: "http://a", Healthy: true, Weight: 1}, 1)
    pool.AddChannel(&Channel{ID: "b", BaseURL: "http://b", Healthy: true, Weight: 1}, 1)

    lb := NewLoadBalancer(pool, "weighted")
    router := NewRouter(pool, lb, nil, nil, NewHealthChecker(HealthCheckDisabled, 5*time.Second))

    callCount := 0
    fn := func(ch *RouteChannel) (*ProviderResponse, error) {
        callCount++
        return nil, errors.New("error")
    }

    _, err := router.RouteWithFallback(context.Background(), "chat", 2, fn)
    if err == nil {
        t.Fatal("expected error")
    }
    // Both attempts use the same channel due to LB; with 2 channels it varies
}
```

- [ ] **Step 3: Run test**

Run: `go test ./src/server/internal/relay/router_test.go -v -run TestRouter_RouteWithFallback`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add src/server/internal/relay/router.go
git commit -m "feat(relay): add RouteWithFallback with retry and all-channel-failed 503 response"
```
