# Vertex Relay Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first real Vertex AI provider adapter so Oblivious can create and route Vertex channels instead of keeping Vertex as a planned catalog item.

**Architecture:** Implement Vertex as a native Relay provider adapter under `src/server/internal/relay/channel`. The first slice supports API-key mode for Gemini-style chat generation, using Vertex publisher model URLs and the existing Gemini request/usage shape.

**Tech Stack:** Go 1.22, standard `net/http`, existing Relay `types.ProviderAdapter`, existing channel provider registry.

---

### Task 1: Vertex Gemini Chat Adapter

**Files:**
- Modify: `src/server/internal/relay/channel/provider_registry_test.go`
- Create: `src/server/internal/relay/channel/vertex_adapter_test.go`
- Create: `src/server/internal/relay/channel/vertex_adapter.go`
- Modify: `src/server/internal/relay/channel/provider_registry.go`

- [ ] **Step 1: Write the failing tests**

Add tests proving:
- `AdapterForChannel` returns a supported `vertex` adapter.
- Vertex API-key mode builds `/v1/projects/{project}/locations/{region}/publishers/google/models/{model}:generateContent?key={key}`.
- Chat messages become Vertex/Gemini `contents` JSON.
- Vertex `usageMetadata` maps into `types.Usage`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/relay/channel -run 'TestAdapterForChannelSupportsVertexProvider|TestVertexAdapter' -count=1`

Expected: FAIL because `vertex` is still planned/unsupported and `NewVertexAdapter` does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `VertexAdapter` with:
- API-key parsing from `key|project|region`, with `key|region` also accepted.
- `BuildURL(model, APITypeChat)` -> Vertex publisher model generateContent URL.
- `BuildHeaders` -> JSON headers.
- `DoRequest` -> POST the existing Gemini-shaped body.
- `ConvertResponse` -> usage from `usageMetadata.promptTokenCount`, `candidatesTokenCount`, `totalTokenCount`.
- `MapError` -> parse Google-style `error.status` and `error.message`.

- [ ] **Step 4: Register provider support**

Change Vertex status from `planned` to `supported` in `provider_registry.go` and return `NewVertexAdapter(...)` from `AdapterForChannel`.

- [ ] **Step 5: Run target tests**

Run: `go test ./internal/relay/channel -run 'TestAdapterForChannelSupportsVertexProvider|TestVertexAdapter' -count=1`

Expected: PASS.

- [ ] **Step 6: Run package and relay regression**

Run:
- `go test ./internal/relay/channel -count=1`
- `go test ./internal/relay/... -count=1`
- `go test ./internal/admin -run 'Test.*Channel|TestServiceCreateChannel' -count=1`

Expected: PASS.
