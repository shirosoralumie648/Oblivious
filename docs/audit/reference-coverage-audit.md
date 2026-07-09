# Reference Coverage Audit v2

Generated: 2026-06-29

Scope: this audit checks coverage of `reference/` only. It does not update production code, tests, OpenAPI, completion matrix, previous report conclusions, Proven status, or P0 implementation state.

## Summary

The previous round did not fully scan `reference/`.

The strongest previous evidence is concentrated in `new-api` and a smaller set of partially scanned projects: `bifrost`, `coze-studio`, `CPA-Manager`, `dify`, `FastGPT`, `LibreChat`, `litellm`, `lobe-chat`, `NextChat`, `open-webui`, and `sub2api`.

Many projects were only named, summarized, or mapped by broad/stale directories. Several gateway and CLI bridge projects had no effective project-specific source evidence in the requested prior audit reports.

Status meaning in this file:

- `Covered`: multiple valid source-file paths and sufficient capability extraction in prior reports.
- `Partially Covered`: some valid source-file evidence exists, but domains/directories are incomplete.
- `Mentioned Only`: project was named or summarized, but source evidence is insufficient.
- `Not Scanned`: no effective project-specific scan evidence was found in the requested prior audit reports.
- `Not Useful / Stub-heavy`: reference itself is too stub-heavy or docs-only for capability extraction.

The v2 subagent paths below are newly observed evidence used to define rescan targets. They do not mean the previous round had covered those paths.

## Reference Project Inventory

| Project | Language / framework | Source | Useful as capability reference | Product sketch only | Prior coverage status |
| --- | --- | --- | --- | --- | --- |
| `ai-gateway` | Go, Kubernetes controller-runtime, Envoy ext_proc, Helm | yes | yes | no | Not Scanned |
| `anything-llm` | Node/Express, React/Vite, Prisma | yes | yes | no | Mentioned Only |
| `bifrost` | Go core, React/TypeScript UI | yes | yes | no | Partially Covered |
| `claude-code-api` | TypeScript, Node/Express | yes | yes, narrow | no | Not Scanned |
| `Cli-Proxy-API-Management-Center` | React/TypeScript/Vite | yes | yes, admin UI | no | Not Scanned |
| `CLIProxyAPI` | Go, Gin, WebSocket, TUI | yes | yes | no | Not Scanned |
| `CliRelay` | Go, Gin, TUI, SQLite | yes | yes | no | Not Scanned |
| `codex-oauth` | Python OAuth CLI | yes | yes, narrow | no | Not Scanned |
| `copilot-api` | TypeScript, Bun, Hono | yes | yes, narrow | no | Not Scanned |
| `coze-studio` | Go/Hertz/Eino, React/TypeScript, Thrift | yes | yes | no | Partially Covered |
| `CPA-Manager` | React/TypeScript plus Go usage-service | yes | yes | no | Partially Covered |
| `dify` | Python Flask/Celery, Next.js/React | yes | yes | no | Partially Covered |
| `FastGPT` | TypeScript/Node monorepo, Next.js, MCP, sandbox | yes | yes | no | Partially Covered |
| `Flowise` | TypeScript monorepo, Express/TypeORM, React/ReactFlow | yes | yes | no | Mentioned Only |
| `gateway` | TypeScript, Hono, Cloudflare Workers | yes | yes | no | Not Scanned |
| `helicone` | TypeScript monorepo, Workers, Next.js, ClickHouse | yes | yes | no | Not Scanned |
| `LibreChat` | Node/Express, React/Vite, MongoDB | yes | yes | no | Partially Covered |
| `litellm` | Python SDK/FastAPI proxy, Next.js dashboard | yes | yes | no | Partially Covered |
| `llm-gateway` | Python FastAPI, Next.js dashboard | yes | yes | no | Not Scanned |
| `llmgateway` | TypeScript Hono/OpenAPI, Next.js, Drizzle | yes | yes | no | Not Scanned |
| `lobe-chat` | TypeScript, Next.js, Zustand, Drizzle | yes | yes | no | Partially Covered |
| `lobehub` | TypeScript monorepo, Next.js, Electron | yes | yes | no | Not Scanned |
| `MaxKB` | Python Django/DRF, pgvector, Vue | yes | yes | no | Mentioned Only |
| `new-api` | Go Gin/GORM/Redis, React/TypeScript | yes | yes | no | Covered |
| `NextChat` | TypeScript, Next.js, Zustand, Tauri | yes | yes | no | Partially Covered |
| `one-api` | Go Gin/GORM/Redis, React | yes | yes | no | Not Scanned |
| `open-webui` | Python/FastAPI, SvelteKit/Svelte | yes | yes | no | Partially Covered |
| `openai-oauth` | TypeScript/Node ESM monorepo | yes | yes, narrow | no | Not Scanned |
| `ragflow` | Python Quart/RAG, React, Go services | yes | yes | no | Mentioned Only |
| `sub2api` | Go Gin/Ent/Redis, Vue 3/TypeScript | yes | yes | no | Partially Covered |

No project in this pass was downgraded to `Not Useful / Stub-heavy`. Some projects are narrow references, especially `codex-oauth`, `openai-oauth`, `claude-code-api`, and `copilot-api`, but they still contain usable source.

## Project Coverage Status

### Covered

- `new-api`: prior reports cite multiple concrete source paths for channel management, routing/fallback, pre-consume/refund, streaming, pricing, relay info, subscriptions, webhooks, and billing sessions. It is covered for relay/billing/provider-management purposes, not for RAG/workflow/agent/marketplace.

### Partially Covered

- `bifrost`: provider/MCP/API compatibility evidence exists, but observability, UI, deployment, and plugin directories need deeper scans. One prior `core/cache` path is stale.
- `coze-studio`: prior evidence covers workflow/IDL/marketplace slices. Plugin implementation, knowledge, memory, connectors, model manager, frontend agent IDE, workflow playground, and deployment are missing.
- `CPA-Manager`: source evidence exists mainly for usage ledger/model pricing. Monitoring UI, API clients, collector/import/export, and current React/Go layout were undercovered.
- `dify`: file service, workflow run, billing/quota, plugin install, and recommendations were cited. Web UI, `dify-agent`, VDB providers, trace providers, SDKs, and e2e were not sufficiently scanned.
- `FastGPT`: one concrete agent-node UI path and several broad directories were used. MCP, sandbox, billing, permissions, marketplace, deploy, and actual workflow UI need rescans.
- `LibreChat`: prior evidence focuses on agents/tools/planning. Chat routes, MCP lifecycle, files/speech, auth/admin/RBAC, balance, frontend, deployment, and e2e are incomplete.
- `litellm`: prior evidence covers budget/cost and broad API compatibility. Proxy route map, provider taxonomy, auth/RBAC, guardrails, MCP/A2A, dashboard, schema/migrations, deployment, and tests are missing.
- `lobe-chat`: prior evidence covers topic actions, upload, agent runtime, traces, and market page. Auth, sync/import/export, DB schema, API routes, settings, media, PWA/mobile/i18n, and plugin layout are incomplete.
- `NextChat`: prior evidence covers chat store, tool loop, model config, and basic chat utilities. MCP, sync, artifacts, realtime, Stable Diffusion, PWA, Tauri, and tests need rescans.
- `open-webui`: prior evidence covers chat/message models, files router, tools utility, and model selector. Stale `backend/apps/...` paths exist; auth/users/groups, media, admin analytics, channels, automations, terminal, code interpreter, and storage need rescans.
- `sub2api`: prior source evidence is mainly payment/subscription/webhook/refund/reconciliation. Gateway compatibility, auth/OAuth, account scheduler, API keys, concurrency/rate-limit, channel monitoring, risk control, and frontend ops need rescans.

### Mentioned Only

- `anything-llm`: generic RAG/upload mention only; no concrete source-file evidence.
- `Flowise`: workflow/node UI was described, but evidence was broad directory or summary-level, not source-file level.
- `MaxKB`: broad/stale `apps/dataset` style paths were used; current RAG/workflow source is under `apps/knowledge`, `apps/application/flow`, `apps/models_provider`, and `ui/src/workflow`.
- `ragflow`: broad `rag/` directory references exist, but API services, DeepDoc parser, task executors, retrieval backends, agent, MCP, UI, Go service, SDK, and deployment were not file-level scanned.

### Not Scanned

- `ai-gateway`
- `claude-code-api`
- `Cli-Proxy-API-Management-Center`
- `CLIProxyAPI`
- `CliRelay`
- `codex-oauth`
- `copilot-api`
- `gateway`
- `helicone`
- `llm-gateway`
- `llmgateway`
- `lobehub`
- `one-api`
- `openai-oauth`

These projects may have been present in broader inventories or design-side notes, but the requested prior audit reports did not provide effective project-specific source evidence for them.

## Capability Domain Coverage

| Domain | Should reference | Prior scanned | Evidence level | Coverage |
| --- | --- | --- | --- | --- |
| Relay / API Gateway | `new-api`, `litellm`, `bifrost`, `ai-gateway`, `gateway`, `one-api`, `llm-gateway`, `llmgateway`, `sub2api`, `CLIProxyAPI`, `CliRelay`, `helicone` | mostly `new-api`, `litellm`, `bifrost`, `sub2api` billing slice | strong for `new-api`; weak elsewhere | Insufficient |
| Chat | `lobe-chat`, `NextChat`, `open-webui`, `LibreChat`, `anything-llm`, `lobehub`, `MaxKB` | `lobe-chat`, `NextChat`, `open-webui`, partial `LibreChat` | source paths exist but narrow | Partial |
| Knowledge / RAG | `ragflow`, `MaxKB`, `dify`, `FastGPT`, `open-webui`, `anything-llm`, `coze-studio`, `Flowise` | `dify`, `open-webui`, one `FastGPT` path | strongest RAG projects undercovered | Insufficient |
| Agent | `LibreChat`, `dify`, `coze-studio`, `FastGPT`, `Flowise`, `anything-llm`, `lobehub`, `ragflow`, `MaxKB` | `LibreChat`, `dify`, `coze-studio`, `FastGPT` slices | scattered | Partial |
| Workflow | `coze-studio`, `dify`, `FastGPT`, `Flowise`, `MaxKB`, `ragflow`, `anything-llm` | `coze-studio`, `dify`, `FastGPT` slices | missing Flowise/MaxKB/RAGFlow file evidence | Insufficient |
| Billing / Quota | `new-api`, `sub2api`, `CPA-Manager`, `litellm`, `dify`, `one-api`, `llmgateway`, `helicone` | `new-api`, `sub2api`, `CPA-Manager`, `litellm`, `dify` | decent but incomplete | Partial |
| Marketplace | `coze-studio`, `dify`, `lobe-chat`, `Flowise`, `FastGPT`, `lobehub`, `LibreChat` | `coze-studio`, `dify`, `lobe-chat` | narrow | Partial |
| Admin | `new-api`, `open-webui`, `sub2api`, `CPA-Manager`, `llmgateway`, `Cli-Proxy-API-Management-Center`, `LibreChat` | `new-api`, `open-webui`, `sub2api`, `CPA-Manager` slices | many admin UIs missed | Insufficient |
| Observability | `helicone`, `bifrost`, `litellm`, `CPA-Manager`, `gateway`, `llmgateway`, `open-webui` | `CPA-Manager`, `litellm`, partial `bifrost` | Helicone absent | Insufficient |
| Deployment / Security | `ai-gateway`, `litellm`, `helicone`, `llmgateway`, `ragflow`, `bifrost`, `open-webui`, `sub2api` | scattered | no systematic scan | Insufficient |
| Provider / Model / Auth Registry | `new-api`, `litellm`, `lobe-chat`, `NextChat`, `open-webui`, `one-api`, `CLIProxyAPI`, `CliRelay`, OAuth projects | `new-api`, `litellm`, chat UIs | OAuth/account-pool projects missed | Partial |
| Plugin / Hook / Skill System | `dify`, `open-webui`, `LibreChat`, `lobe-chat`, `coze-studio`, `Flowise`, `anything-llm`, `FastGPT`, `bifrost`, `gateway`, `lobehub` | `dify`, `open-webui`, `LibreChat`, `lobe-chat` | incomplete MCP/hook coverage | Partial |
| Session / Runtime / Event Model | `lobe-chat`, `NextChat`, `LibreChat`, `dify`, `CLIProxyAPI`, `CliRelay`, `coze-studio`, `lobehub`, `ragflow` | chat stores and agent client slices | CLI/runtime and durable event models missed | Partial |
| Sandbox / Tool Execution | `open-webui`, `FastGPT`, `MaxKB`, `Flowise`, `LibreChat` | `open-webui`, `FastGPT` mentions | missing implementation details | Insufficient |
| Multi-agent / Team Runtime | `LibreChat`, `dify`, `Flowise`, `coze-studio`, `lobehub`, `ragflow`, `anything-llm` | `LibreChat`, `dify` slices | no systematic team runtime scan | Insufficient |
| IDE / Local Code Editing Workflow | `lobehub`, `claude-code-api`, `CLIProxyAPI`, `CliRelay`, `openai-oauth`, `codex-oauth` | none | not established | Insufficient |
| CLI / TUI / SDK / Remote Bridge | `CLIProxyAPI`, `CliRelay`, `codex-oauth`, `openai-oauth`, `copilot-api`, `NextChat`, `ragflow`, `litellm` | `NextChat` Tauri mention only | most CLI/TUI/SDK bridge projects missed | Insufficient |

## Missing Evidence

The previous round over-generalized in these areas:

- It treated project names or category labels as evidence for capability coverage.
- It used broad directory mappings where file-level evidence was needed.
- It cited stale paths, including `reference/CLIProxyAPI/pkg/oauth`, `reference/CLIProxyAPI/pkg/router`, `reference/bifrost/core/cache`, `reference/MaxKB/apps/dataset`, `reference/open-webui/backend/apps/...`, `reference/sub2api/backend/internal/auth`, and `reference/lobe-chat/src/plugins`.
- It blurred similarly named projects: `llm-gateway` vs `llmgateway`, `lobe-chat` vs `lobehub`, and `one-api` vs `new-api`.
- It mapped reference capabilities to Oblivious gaps without proving all relevant reference source directories had been read.

## Top 20 Rescan Areas

1. `reference/gateway/src/handlers`, `src/providers`, `src/middlewares`, `plugins`
2. `reference/ai-gateway/api`, `internal/controller`, `internal/extproc`, `internal/translator`, `internal/mcpproxy`
3. `reference/llmgateway/apps/gateway/src`, `apps/api/src`, `packages/db/src`, `ee`
4. `reference/one-api/router`, `controller`, `relay`, `model`, `monitor`, `web/*/src`
5. `reference/llm-gateway/backend/app/api`, `backend/app/services`, `llm_api_converter`
6. `reference/CLIProxyAPI/internal/api`, `internal/auth`, `internal/runtime/executor`, `sdk`
7. `reference/CliRelay/internal/api`, `internal/runtime/executor`, `internal/translator`, `sdk`, `internal/tui`
8. `reference/helicone/worker/src`, `valhalla/jawn/src`, `web`, `packages`, `supabase`, `helicone-mcp`
9. `reference/ragflow/api`, `deepdoc`, `rag/svr`, `rag/nlp`, `agent`, `mcp`, `web`, `cmd`, `internal`
10. `reference/MaxKB/apps/knowledge`, `apps/application/flow`, `apps/models_provider`, `apps/tools`, `ui/src/workflow`
11. `reference/Flowise/packages/server/src`, `packages/components/nodes`, `packages/ui/src/views`, `packages/agentflow`, `packages/observe`
12. `reference/anything-llm/server/endpoints`, `server/utils/agents`, `server/utils/agentFlows`, `server/utils/MCP`, `collector`, `frontend/src`
13. `reference/coze-studio/backend/application`, `backend/domain`, `frontend/packages/workflow`, `frontend/packages/agent-ide`
14. `reference/dify/api/core/workflow`, `api/core/plugin`, `api/providers`, `dify-agent`, `web/app/components`
15. `reference/sub2api/backend/internal/service`, `backend/internal/server/routes`, `backend/ent/schema`, `frontend/src`
16. `reference/open-webui/backend/open_webui/routers`, `models`, `retrieval`, `utils`, `src/lib/components/admin`
17. `reference/LibreChat/api/server/routes`, `api/server/services`, `client/src`, `packages/data-schemas`
18. `reference/lobehub/src/app`, `src/server/modules`, `packages/model-runtime`, `packages/agent-runtime`, `apps/desktop`
19. `reference/openai-oauth/packages/openai-oauth-core`, `packages/openai-oauth`, `packages/openai-oauth-provider`
20. `reference/copilot-api/src/routes`, `src/services`, `src/lib`, `tests`

## Next-Round Subagent Rescan Plan

Use one subagent per project or tightly paired slice. Each subagent must return concrete source paths and line-level notes for capability extraction. Concept summaries alone should be rejected.

Required return format for every subagent:

- Project and exact directories scanned.
- 10 to 20 concrete source-file paths.
- Capability domains proven by those paths.
- Explicit non-goals and stub/not-implemented boundaries.
- Which prior evidence was stale, broad, or missing.
- Clear statement that reference capability is not Oblivious implementation proof.

Suggested allocation:

| Agent | Projects / directories | Focus |
| --- | --- | --- |
| A1 | `reference/gateway`, `reference/ai-gateway` | Relay, provider routing, hooks/plugins, streaming, deployment |
| A2 | `reference/llmgateway`, `reference/llm-gateway`, `reference/one-api` | Gateway, admin APIs, pricing, DB schema, provider/model registry |
| A3 | `reference/CLIProxyAPI`, `reference/CliRelay`, `reference/Cli-Proxy-API-Management-Center` | CLI bridge, account pools, OAuth, runtime executors, TUI/admin UI |
| A4 | `reference/helicone`, `reference/CPA-Manager` | Observability, usage ledger, ClickHouse/SQLite analytics, dashboards |
| A5 | `reference/ragflow`, `reference/MaxKB`, `reference/anything-llm` | RAG ingestion, parsing, retrieval, vector backends, knowledge UI |
| A6 | `reference/Flowise`, `reference/FastGPT`, `reference/dify` | Workflow canvas, node registry, executions, agent/workflow runtime |
| A7 | `reference/coze-studio` | Workflow, plugin, knowledge, memory, connectors, agent IDE, marketplace |
| A8 | `reference/LibreChat`, `reference/open-webui` | Chat, agents/tools, MCP, files, auth/admin, frontend settings |
| A9 | `reference/lobe-chat`, `reference/lobehub`, `reference/NextChat` | Chat UX, provider runtime, plugins/MCP, sync, desktop/PWA |
| A10 | `reference/codex-oauth`, `reference/openai-oauth`, `reference/copilot-api`, `reference/claude-code-api` | OAuth/token lifecycle, local proxy, SSE/tool conversion, CLI/IDE bridge |
| A11 | `reference/new-api`, `reference/sub2api`, `reference/litellm` | Validate prior covered slices and fill missing route/auth/schema/deploy evidence |

Guardrails for the next round:

- Do not summarize a capability without at least one concrete source path.
- Do not count README claims as implementation evidence unless paired with source.
- Do not count broad directories as file-level proof.
- Mark stale or missing paths explicitly.
- Keep reference evidence separate from Oblivious implementation status.
- Do not mark any Oblivious module Proven based on reference-only evidence.
- Do not produce implementation tickets from this rescan; produce only coverage deltas and evidence.

## Final Coverage Answer

Previous full-reference scan: no. `reference/` has 30 projects, and only one project (`new-api`) had broad enough prior source evidence for its main reference role.

Covered: `new-api`.

Partially Covered: `bifrost`, `coze-studio`, `CPA-Manager`, `dify`, `FastGPT`, `LibreChat`, `litellm`, `lobe-chat`, `NextChat`, `open-webui`, `sub2api`.

Mentioned Only: `anything-llm`, `Flowise`, `MaxKB`, `ragflow`.

Not Scanned: `ai-gateway`, `claude-code-api`, `Cli-Proxy-API-Management-Center`, `CLIProxyAPI`, `CliRelay`, `codex-oauth`, `copilot-api`, `gateway`, `helicone`, `llm-gateway`, `llmgateway`, `lobehub`, `one-api`, `openai-oauth`.

Not Useful / Stub-heavy: none found in this pass.
