# Relay Fallback Channel Exclusion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Relay fallback retries advance to another eligible channel after a retryable upstream failure instead of repeatedly selecting the same failed channel.

**Architecture:** Keep the existing load balancer behavior unchanged for normal selection. Add private router selection helpers that can skip channel IDs for one fallback sequence, and use them only inside `RouteWithFallback`.

**Tech Stack:** Go 1.22, existing `relay.Router`, existing `LoadBalancer` and `ChannelPool`.

---

### Task 1: Retryable Fallback Excludes Failed Channel

**Files:**
- Modify: `src/server/internal/relay/router_test.go`
- Modify: `src/server/internal/relay/router.go`

- [ ] **Step 1: Write the failing test**

Add a test where priority routing chooses `primary`; `primary` returns HTTP 500; retry must skip it and call `secondary`, which returns 200.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/relay -run TestRouter_RouteWithFallback_SkipsFailedChannelOnRetry -count=1`

Expected: FAIL because current fallback retries `primary` twice.

- [ ] **Step 3: Write minimal implementation**

Add private helpers:
- `selectChannelSkipping(ctx, routeKeys, excluded)`
- `routeSkipping(ctx, apiType, excluded, fn)`

Update `RouteWithFallback` to remember the channel ID used for retryable failures and exclude it on the next attempt.

- [ ] **Step 4: Run target test**

Run: `go test ./internal/relay -run TestRouter_RouteWithFallback_SkipsFailedChannelOnRetry -count=1`

Expected: PASS.

- [ ] **Step 5: Run relay regression**

Run: `go test ./internal/relay/... -count=1`

Expected: PASS.
