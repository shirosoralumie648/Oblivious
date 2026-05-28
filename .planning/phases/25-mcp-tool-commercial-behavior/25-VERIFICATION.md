# Phase 25 Verification: MCP Tool Commercial Behavior

## Result

`PROD-01` is complete for Phase 25.

Built-in Agent/MCP tools now have a default commercial policy:

- `calculator` is enabled by default and evaluates a bounded arithmetic grammar.
- `datetime` remains enabled by default and returns RFC3339 time output.
- `web_search` is disabled by default until a real search provider is configured.
- `http_request` is disabled by default until a tenant-safe outbound HTTP policy is configured.

Disabled built-ins are filtered from Agent tool definitions and `ListAvailableTools`, and executor-level calls reject disabled built-ins before invoking tool implementation. This prevents stale Agent JSON from surfacing or executing disabled commercial built-ins.

## Code Evidence

- `src/server/internal/mcp/builtin.go`: default commercial registry, calculator parser, disabled default `web_search`/`http_request`.
- `src/server/internal/mcp/builtin_test.go`: registry, calculator, datetime, disabled built-in, placeholder, and no-outbound tests.
- `src/server/internal/agent/executor.go`: model-facing tool definitions filter disabled built-ins and execution rejects disabled built-ins before calling tool code.
- `src/server/internal/agent/service.go`: available-tool and direct execution paths enforce the same policy.
- `src/server/internal/agent/service_test.go`: Agent policy tests for filtering, rejection, and enabled calculator behavior.
- `docs/API.md`, `docs/release/commercial-gates.md`, and `scripts/verify-quality-gates.sh`: product/API contract and docs gate coverage.

## RED Evidence

Before implementation:

```bash
cd src/server && go test ./internal/mcp -run 'Builtin|Calculator|Disabled|Datetime' -count=1
```

Result: failed because `ListDefaultCommercialBuiltinTools` did not exist.

```bash
cd src/server && go test ./internal/agent -run 'Disabled|Builtin|ToolDefinitions|ListAvailable|AllowsEnabledCommercial' -count=1
```

Result: failed because `web_search` and `http_request` were still exposed through Agent tool definitions and available-tool APIs, and disabled built-ins were called by the executor.

## GREEN Evidence

Focused MCP/Agent verification:

```bash
cd src/server && go test ./internal/mcp ./internal/agent -run 'Builtin|Commercial|Tool|Calculator|WebSearch|HTTPRequest|Disabled' -count=1
```

Result:

```text
ok  	oblivious/server/internal/mcp	0.008s
ok  	oblivious/server/internal/agent	0.003s
```

Docs and quality gates:

```bash
bash scripts/check.sh docs
```

Result:

```text
[check] Verifying release assets.
[quality-gates] quality gate assets look complete.
[check] Verifying docs and env consistency.
[check] Verifying mainline workspace boundary.
```

Diff hygiene:

```bash
git diff --check
```

Result: passed with no output.

## Residual Work

Phase 25 closes only `PROD-01`.

Still required for v08 and final commercial readiness:

- Phase 26: durable Agent workflows with persisted tool runs, approval points, observable execution state, memory injection, and retry/failure evidence.
- Phase 27: Knowledge behavior/product-copy alignment, including embedding-backed RAG and source citation if marketed as RAG.
- Phase 28: Chat, Agent, Knowledge, Admin, and Marketplace customer-journey hardening with no enabled placeholder pages or fake commercial behavior.
- Phase 29: public docs, onboarding, pricing, and operator guides aligned with implemented behavior.
- Phase 30: end-to-end commercial journeys and final commercial completion audit.

Final commercial readiness remains unclaimed.
