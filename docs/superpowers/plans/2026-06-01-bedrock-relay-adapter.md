# Bedrock Relay Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first real Amazon Bedrock provider adapter so Oblivious can create and route Bedrock channels instead of keeping Bedrock as a planned catalog item.

**Architecture:** Implement Bedrock as a native Relay provider adapter under `src/server/internal/relay/channel`. The first slice supports Bedrock Runtime API-key mode (`<api-key>|<region>`) and maps OpenAI-style chat requests into Bedrock Converse payloads, with normalized usage extraction for billing.

**Tech Stack:** Go 1.22, standard `net/http`, existing Relay `types.ProviderAdapter`, existing channel provider registry.

---

### Task 1: Bedrock Adapter Contract

**Files:**
- Modify: `src/server/internal/relay/channel/provider_registry_test.go`
- Create: `src/server/internal/relay/channel/bedrock_adapter_test.go`
- Create: `src/server/internal/relay/channel/bedrock_adapter.go`
- Modify: `src/server/internal/relay/channel/provider_registry.go`

- [ ] **Step 1: Write the failing tests**

Add tests proving:
- `AdapterForChannel` returns a supported `bedrock` adapter.
- Bedrock API-key mode builds `/model/{modelId}/converse`.
- Chat messages and system prompts become Bedrock Converse JSON.
- Bedrock response usage maps into `types.Usage`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/relay/channel -run 'TestAdapterForChannelSupportsBedrockProvider|TestBedrockAdapter' -count=1`

Expected: FAIL because `bedrock` is still planned/unsupported and `NewBedrockAdapter` does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `BedrockAdapter` with:
- `BuildURL(model, APITypeChat)` -> `${base}/model/${normalizedModel}/converse`
- `BuildHeaders` -> `Authorization: Bearer <api-key>` plus JSON headers
- `DoRequest` -> POST a marshaled Converse payload
- `ConvertResponse` -> usage from `usage.inputTokens`, `usage.outputTokens`, `usage.totalTokens`
- `MapError` -> parse AWS-style `message`, `Message`, `__type`, or fallback status text

- [ ] **Step 4: Register provider support**

Change Bedrock status from `planned` to `supported` in `provider_registry.go` and return `NewBedrockAdapter(...)` from `AdapterForChannel`.

- [ ] **Step 5: Run target tests**

Run: `go test ./internal/relay/channel -run 'TestAdapterForChannelSupportsBedrockProvider|TestBedrockAdapter' -count=1`

Expected: PASS.

- [ ] **Step 6: Run package regression**

Run: `go test ./internal/relay/channel -count=1`

Expected: PASS.
