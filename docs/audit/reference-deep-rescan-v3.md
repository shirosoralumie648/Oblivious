# Reference Deep Rescan v3

Date: 2026-06-29

Scope: read-only rescan of `reference/` for product capability evidence. This document does not change production code, tests, OpenAPI, completion matrix, Proven status, P0 tickets, or earlier audit conclusions. Reference capability is not proof of Oblivious implementation.

## Executive Summary

The prior reference audit was not a full source-level scan of `reference/`. `docs/audit/reference-coverage-audit.md` already showed that only `new-api` had strong prior evidence, while many projects were only partial, mentioned, or not scanned.

This v3 pass scanned all 30 project roots currently under `reference/`. The requested A1-A11 subagents covered 29 projects. `reference/bifrost` existed in the root manifest but was omitted from the A1-A11 assignment list, so the main agent performed a read-only supplemental scan for it. No reference project root remains completely unscanned at v3 level, but several projects are intentionally narrow references, local bridges, UI-only surfaces, or contain demo/stub-heavy subareas.

The most important correction versus v2 is scope control: P0 should be the evidence spine for Chat + Relay + usage/billing, durable RAG ingestion, minimal agent/tool execution, minimal workflow execution, billing ledger, request observability, and admin enforcement. Realtime, batch, files, fine-tuning/Assistants/Threads/Runs, full visual workflow platforms, full plugin marketplaces, Kubernetes/Envoy CRDs, desktop/PWA/IDE bridge, and full payout settlement should not all be pulled into P0.

## Coverage Summary

| Bucket | Projects |
| --- | --- |
| Deep scanned by A1-A11 | `ai-gateway`, `anything-llm`, `claude-code-api`, `Cli-Proxy-API-Management-Center`, `CLIProxyAPI`, `CliRelay`, `codex-oauth`, `copilot-api`, `coze-studio`, `CPA-Manager`, `dify`, `FastGPT`, `Flowise`, `gateway`, `helicone`, `LibreChat`, `litellm`, `llm-gateway`, `llmgateway`, `lobe-chat`, `lobehub`, `MaxKB`, `new-api`, `NextChat`, `one-api`, `open-webui`, `openai-oauth`, `ragflow`, `sub2api` |
| Supplemental main-agent scan | `bifrost` |
| Still not scanned at project-root level | None |
| Strong P0 product references | `new-api`, `sub2api`, `litellm`, `llmgateway`, `gateway`, `one-api`, `ragflow`, `MaxKB`, `coze-studio`, `LibreChat`, `open-webui`, `helicone`, `CPA-Manager`, `bifrost` |
| Narrow or sketch-only references | `codex-oauth`, `openai-oauth`, `copilot-api`, `claude-code-api`, `Cli-Proxy-API-Management-Center`, `NextChat`, `lobe-chat`, `lobehub`, `ai-gateway`, `CPA-Manager` |

## Per-Project Results

| Project | Scanner | V3 result | Evidence examples | Caveats | Product relevance |
| --- | --- | --- | --- | --- | --- |
| `gateway` | A1 | Strong relay reference | `reference/gateway/src/index.ts:45`, `reference/gateway/src/handlers/handlerUtils.ts:288`, `reference/gateway/src/handlers/streamHandler.ts:392`, `reference/gateway/src/middlewares/requestValidator/index.ts:25`, `reference/gateway/plugins/index.ts:71` | Some provider TODOs; localhost trust defaults must not be copied. | P0 for relay, streaming, hooks, SSRF/custom-host validation. |
| `ai-gateway` | A1 | Strong but domain-narrow | `reference/ai-gateway/api/v1beta1/ai_gateway_route.go:57`, `reference/ai-gateway/internal/controller/ai_gateway_route.go:85`, `reference/ai-gateway/internal/extproc/processor_impl.go:62`, `reference/ai-gateway/internal/metrics/metrics_impl.go:131` | Kubernetes/Envoy extension is enterprise infrastructure, not MVP. | P2/GA for Kubernetes gateway, token rate limit, GenAI metrics. |
| `llmgateway` | A2 | Strong gateway/admin/billing reference | `reference/llmgateway/apps/gateway/src/chat/chat.ts:940`, `reference/llmgateway/packages/actions/src/get-cheapest-from-available-providers.ts:330`, `reference/llmgateway/packages/db/src/schema.ts:471`, `reference/llmgateway/apps/api/src/routes/keys-api.ts:471` | UI demo/mock cost analytics exist. | P0 for provider registry, pricing, API keys, routing. |
| `llm-gateway` | A2 | Useful but narrower | `reference/llm-gateway/backend/app/api/proxy/openai.py:226`, `reference/llm-gateway/backend/app/services/proxy_service.py:99`, `reference/llm-gateway/backend/app/services/strategy.py:392`, `reference/llm-gateway/backend/app/db/models.py:46` | Dev localhost defaults. | P0/P1 for proxy strategy and schema. |
| `one-api` | A2 | Useful with explicit unsupported endpoints | `reference/one-api/router/relay.go:20`, `reference/one-api/router/api.go:71`, `reference/one-api/middleware/distributor.go:47`, `reference/one-api/relay/billing/billing.go:23` | Many relay routes call `RelayNotImplemented`; do not claim broad route completeness. | P0 for supported/unsupported route boundary and billing pattern. |
| `CLIProxyAPI` | A3 | Strong CLI bridge/account-pool reference | `reference/CLIProxyAPI/internal/api/server.go:362`, `reference/CLIProxyAPI/internal/auth/codex/openai_auth.go:63`, `reference/CLIProxyAPI/sdk/cliproxy/auth/scheduler.go:240`, `reference/CLIProxyAPI/internal/runtime/executor/codex_executor.go:253` | Local callbacks and runtime-only account notes. | P1/P2 for CLI bridge, provider auth registry, executor abstraction. |
| `CliRelay` | A3 | Strong CLI/TUI bridge reference | `reference/CliRelay/internal/api/server.go:381`, `reference/CliRelay/sdk/cliproxy/service.go:535`, `reference/CliRelay/sdk/cliproxy/auth/conductor.go:155`, `reference/CliRelay/internal/tui/oauth_tab.go:21` | Unsupported compact streaming and local deployment assumptions. | P1/P2 for TUI/auth/session runtime. |
| `Cli-Proxy-API-Management-Center` | A3 | UI-only admin reference | `reference/Cli-Proxy-API-Management-Center/src/router/MainRoutes.tsx:14`, `reference/Cli-Proxy-API-Management-Center/src/services/api/oauth.ts:35`, `reference/Cli-Proxy-API-Management-Center/src/services/api/providers.ts:364` | No backend enforcement by itself. | Sketch-only/admin UI reference. |
| `helicone` | A4 | Strong observability reference | `reference/helicone/worker/src/lib/dbLogger/DBLoggable.ts:590`, `reference/helicone/clickhouse/migrations/schema_41_request_response_replacing_merge_tree.sql:1`, `reference/helicone/valhalla/jawn/src/managers/MetricsManager.ts:266`, `reference/helicone/worker/src/lib/managers/AlertManager.ts:31` | Dashboard mock paths and fake LLM branch exist; fallback queue not implemented. | P1/P2 for request logs, raw body store, analytics, SLO alerts. |
| `CPA-Manager` | A4 | Useful local usage ledger reference | `reference/CPA-Manager/usage-service/internal/store/store.go:119`, `reference/CPA-Manager/usage-service/internal/collector/collector.go:379`, `reference/CPA-Manager/usage-service/internal/httpapi/server.go:427`, `reference/CPA-Manager/src/pages/MonitoringCenterPage.tsx:2049` | SQLite/local scale; weak SLO reference. | P0/P1 for ledger/pricing UI, not production analytics architecture. |
| `ragflow` | A5 | Strong RAG ingestion/citation reference | `reference/ragflow/api/apps/restful_apis/document_api.py:59`, `reference/ragflow/rag/svr/task_executor.py:270`, `reference/ragflow/rag/nlp/search.py:562`, `reference/ragflow/deepdoc/parser/pdf_parser.py:1673` | Some parser/admin methods are NotImplemented. | P0/P1 for async ingestion, parser, retrieval debug, citations. |
| `MaxKB` | A5 | Strong RAG lifecycle reference | `reference/MaxKB/apps/knowledge/serializers/document.py:996`, `reference/MaxKB/apps/knowledge/task/embedding.py:61`, `reference/MaxKB/apps/knowledge/vector/pg_vector.py:148`, `reference/MaxKB/apps/knowledge/serializers/document.py:1432` | Demo paths in application area; not a gateway/billing reference. | P0 for document state, embedding jobs, vector lifecycle. |
| `anything-llm` | A5 | Useful lightweight RAG/workspace reference | `reference/anything-llm/server/endpoints/workspaces.js:208`, `reference/anything-llm/server/utils/TextSplitter/index.js:157` | TextSplitter TODO; agent CLI plugin stream emulation is not RAG evidence. | P1/sketch for workspace knowledge UX. |
| `Flowise` | A6 | Strong workflow canvas/reference, heavy scope | `reference/Flowise/packages/ui/src/views/canvas/index.jsx:586`, `reference/Flowise/packages/server/src/NodesPool.ts:10`, `reference/Flowise/packages/server/src/utils/buildChatflow.ts:299`, `reference/Flowise/packages/server/src/utils/buildAgentflow.ts:184` | Tool streaming TODO. | P1/P2; do not import full platform into P0. |
| `FastGPT` | A6 | Strong workflow/MCP/RAG reference | `reference/FastGPT/packages/global/core/workflow/node/constant.ts:128`, `reference/FastGPT/packages/service/core/workflow/dispatch/index.ts:159`, `reference/FastGPT/projects/mcp_server/src/index.ts:55`, `reference/FastGPT/projects/app/src/pageComponents/app/detail/WorkflowComponents/context/workflowDebugContext.tsx:94` | Some context defaults throw Function not implemented. | P1 for workflow debugger/node registry/MCP. |
| `dify` | A6 | Strong workflow/plugin reference | `reference/dify/api/core/workflow/workflow_entry.py:185`, `reference/dify/api/core/workflow/node_factory.py:271`, `reference/dify/api/services/workflow_event_snapshot_service.py:79`, `reference/dify/api/core/plugin/plugin_service.py:601` | Fake agent backend and cancellation 501 exist. | P1 for workflow state/plugin lifecycle; P0 only minimal execution evidence. |
| `coze-studio` | A7 | Strong broad product reference | `reference/coze-studio/backend/application/workflow/workflow.go:226`, `reference/coze-studio/backend/domain/workflow/internal/compose/workflow_run.go:46`, `reference/coze-studio/backend/domain/plugin/service/plugin_release.go:67`, `reference/coze-studio/backend/domain/knowledge/service/retrieve.go:54`, `reference/coze-studio/idl/marketplace/public_api.thrift:7` | MCP invocation stub; connector list static; marketplace install mostly duplicate/use contracts. | P0/P1 for workflow/plugin/knowledge/memory, P2 for marketplace. |
| `LibreChat` | A8 | Strong chat/agent/MCP/admin reference | `reference/LibreChat/api/server/routes/convos.js:32`, `reference/LibreChat/api/server/routes/messages.js:13`, `reference/LibreChat/api/server/controllers/agents/openai.js:421`, `reference/LibreChat/packages/api/src/mcp/MCPManager.ts:43`, `reference/LibreChat/api/server/routes/admin/roles.js:34` | MCP OAuth TODO; some import/action UI gaps. | P0/P1 for chat sessions, tools, MCP, RBAC. |
| `open-webui` | A8 | Strong chat/files/tools/admin reference | `reference/open-webui/backend/open_webui/models/chats.py:43`, `reference/open-webui/backend/open_webui/routers/chats.py:52`, `reference/open-webui/backend/open_webui/routers/files.py:218`, `reference/open-webui/backend/open_webui/utils/tools.py:430`, `reference/open-webui/backend/open_webui/routers/auths.py:162` | Tool/MCP/OAuth TODOs and local admin fallback exist. | P0/P1 for chat, files, tools, RBAC, model selector. |
| `lobe-chat` | A9 | Useful chat/provider/plugin reference | `reference/lobe-chat/src/app/api/chat/[provider]/route.ts:12`, `reference/lobe-chat/src/libs/agent-runtime/AgentRuntime.ts:80`, `reference/lobe-chat/src/app/api/plugin/gateway/route.ts:17`, `reference/lobe-chat/src/server/services/dataImporter/index.ts:30` | Data import TODO for TTS/images. | P1 for provider runtime and import/export. |
| `lobehub` | A9 | Useful MCP/desktop/gateway reference | `reference/lobehub/packages/agent-gateway-client/src/client.ts:144`, `reference/lobehub/src/libs/mcp/client.ts:157`, `reference/lobehub/src/store/tool/slices/mcpStore/action.ts:71`, `reference/lobehub/apps/desktop/src/main/services/gatewayConnectionSrv.ts:24` | Desktop stubs under `apps/desktop/stubs`. | P1/P2 for MCP marketplace/desktop bridge. |
| `NextChat` | A9 | Useful client UX/PWA/Tauri reference | `reference/NextChat/app/store/chat.ts:243`, `reference/NextChat/app/utils/chat.ts:224`, `reference/NextChat/app/mcp/actions.ts:22`, `reference/NextChat/src-tauri/src/stream.rs:34` | Browser-local/PWA cache is not backend durability. | P1/sketch for chat UX, sync/export, local bridge. |
| `codex-oauth` | A10 | Narrow OAuth sample | `reference/codex-oauth/codex_oauth.py:93`, `reference/codex-oauth/codex_oauth.py:157`, `reference/codex-oauth/codex_oauth.py:264` | Local browser callback and `auth.json`; not production auth. | Sketch-only/local bridge. |
| `openai-oauth` | A10 | Useful OAuth/SSE adapter reference | `reference/openai-oauth/packages/openai-oauth-core/src/auth.ts:256`, `reference/openai-oauth/packages/openai-oauth-core/src/transport.ts:72`, `reference/openai-oauth/packages/openai-oauth/src/server.ts:53`, `reference/openai-oauth/packages/openai-oauth/src/chat-stream.ts:16` | Local adapter posture, not production gateway security. | P1/P2 for adapter tests, SSE/tool conversion. |
| `copilot-api` | A10 | Useful local Copilot/Anthropic bridge | `reference/copilot-api/src/lib/token.ts:18`, `reference/copilot-api/src/server.ts:19`, `reference/copilot-api/src/routes/messages/non-stream-translation.ts:29`, `reference/copilot-api/src/routes/messages/stream-translation.ts:20` | Token exposure endpoint; local bridge command. | Sketch/local bridge. |
| `claude-code-api` | A10 | Useful Claude Code bridge | `reference/claude-code-api/src/routes/api.ts:17`, `reference/claude-code-api/src/services/claude.ts:131`, `reference/claude-code-api/src/services/claudeAuth.ts:74` | Synthetic long-lived metadata; HTTPS/admin TODOs. | Sketch/local bridge. |
| `new-api` | A11 | Strong relay/admin/billing reference | `reference/new-api/router/relay-router.go:69`, `reference/new-api/router/api-router.go:227`, `reference/new-api/middleware/auth.go:170`, `reference/new-api/service/channel_select.go:14`, `reference/new-api/service/billing_session.go:20` | Explicit unsupported relay endpoints and default credential warnings. | P0 for route/auth/channel/quota/billing boundary. |
| `sub2api` | A11 | Strong auth/admin/gateway/billing reference | `reference/sub2api/backend/internal/server/routes/auth.go:16`, `reference/sub2api/backend/internal/server/routes/admin.go:15`, `reference/sub2api/backend/internal/server/middleware/admin_auth.go:14`, `reference/sub2api/backend/internal/service/gateway_service.go:588`, `reference/sub2api/frontend/src/router/index.ts:381` | Debug/deploy defaults. | P0/P1 for admin auth, sticky sessions, usage ops. |
| `litellm` | A11 | Strong proxy/RBAC/budget reference | `reference/litellm/litellm/proxy/proxy_server.py:1095`, `reference/litellm/litellm/proxy/auth/route_checks.py:67`, `reference/litellm/schema.prisma:12`, `reference/litellm/litellm/proxy/guardrails/init_guardrails.py:19`, `reference/litellm/litellm/litellm_core_utils/get_llm_provider_logic.py:137` | Provider catalog breadth is not equal maturity; request limiter TODO. | P0/P1 for proxy, RBAC, schema, budgets, guardrails. |
| `bifrost` | Main supplement | Strong relay/MCP/observability reference | `reference/bifrost/core/bifrost.go:719`, `reference/bifrost/core/bifrost.go:6474`, `reference/bifrost/core/bifrost.go:6524`, `reference/bifrost/core/mcp/agent.go:35`, `reference/bifrost/plugins/telemetry/main.go:230`, `reference/bifrost/plugins/otel/main.go:203` | Mocker plugin and many test/docs paths excluded; async chat route tests note some converters not implemented. | P0/P1 for gateway streaming, plugin pipeline, MCP/tool execution, metrics. |

## Capability Domain Evidence

| Domain | Strongest references | Evidence paths | Coverage assessment | MVP priority |
| --- | --- | --- | --- | --- |
| Relay / API Gateway | `new-api`, `gateway`, `llmgateway`, `llm-gateway`, `one-api`, `litellm`, `bifrost` | `reference/new-api/router/relay-router.go:69`; `reference/gateway/src/handlers/handlerUtils.ts:288`; `reference/llmgateway/apps/gateway/src/chat/chat.ts:940`; `reference/litellm/litellm/proxy/proxy_server.py:1095`; `reference/bifrost/core/bifrost.go:719` | Sufficient for supported route map, streaming, retry, provider mapping, and explicit unsupported routes. | P0 |
| Chat | `LibreChat`, `open-webui`, `lobe-chat`, `NextChat` | `reference/LibreChat/api/server/routes/convos.js:32`; `reference/open-webui/backend/open_webui/routers/chats.py:52`; `reference/lobe-chat/src/app/api/chat/[provider]/route.ts:12`; `reference/NextChat/app/store/chat.ts:243` | Sufficient for session/message/fork/share/import concepts, but backend durability should outrank client UX. | P0 |
| Knowledge / RAG | `ragflow`, `MaxKB`, `coze-studio`, `open-webui`, `anything-llm` | `reference/ragflow/rag/svr/task_executor.py:270`; `reference/MaxKB/apps/knowledge/task/embedding.py:61`; `reference/coze-studio/backend/domain/knowledge/service/event_handle.go:71`; `reference/open-webui/backend/open_webui/routers/files.py:218` | Sufficient for ingestion job, parse/embed status, delete/reindex, retrieval debug, and citations. | P0/P1 |
| Agent | `LibreChat`, `open-webui`, `coze-studio`, `bifrost`, `dify` | `reference/LibreChat/api/server/controllers/agents/openai.js:421`; `reference/open-webui/backend/open_webui/utils/tools.py:430`; `reference/coze-studio/backend/domain/plugin/service/exec_tool.go:45`; `reference/bifrost/core/mcp/agent.go:35`; `reference/dify/api/core/workflow/nodes/agent_v2/agent_node.py:217` | P0 should be tool-call/approval/trace/budget, not full multi-agent platform. | P0/P1 |
| Workflow | `coze-studio`, `dify`, `FastGPT`, `Flowise` | `reference/coze-studio/backend/domain/workflow/internal/compose/workflow_run.go:46`; `reference/dify/api/core/workflow/workflow_entry.py:185`; `reference/FastGPT/packages/service/core/workflow/dispatch/index.ts:159`; `reference/Flowise/packages/server/src/utils/buildChatflow.ts:299` | Strong enough to define a minimal durable executor and debugger; full visual workflow platform is not P0. | P1 |
| Billing / Quota | `new-api`, `sub2api`, `llmgateway`, `litellm`, `CPA-Manager`, `one-api` | `reference/new-api/service/billing_session.go:20`; `reference/sub2api/backend/ent/schema/user.go:36`; `reference/llmgateway/apps/gateway/src/lib/costs.ts:115`; `reference/litellm/schema.prisma:12`; `reference/CPA-Manager/usage-service/internal/store/store.go:119` | Strong for ledger/reserve/settle/refund/budget patterns. | P0 |
| Marketplace | `coze-studio`, `dify`, `lobehub`, `lobe-chat` | `reference/coze-studio/idl/marketplace/public_api.thrift:7`; `reference/coze-studio/idl/marketplace/product_common.thrift:72`; `reference/dify/api/core/plugin/plugin_service.py:601`; `reference/lobehub/src/routes/(main)/community/(list)/mcp/features/List/Item.tsx:90` | Useful for later product lifecycle; paid install/settlement should not be P0. | P2/later |
| Admin | `new-api`, `sub2api`, `llmgateway`, `litellm`, `open-webui`, `LibreChat` | `reference/new-api/router/api-router.go:227`; `reference/sub2api/backend/internal/server/routes/admin.go:15`; `reference/llmgateway/apps/api/src/routes/keys-api.ts:471`; `reference/litellm/litellm/proxy/auth/route_checks.py:67`; `reference/LibreChat/api/server/routes/admin/roles.js:34` | Sufficient for P0 backend RBAC/admin control plane; UI-only references must not count as enforcement. | P0 |
| Observability | `helicone`, `bifrost`, `CPA-Manager`, `ai-gateway`, `gateway` | `reference/helicone/worker/src/lib/dbLogger/DBLoggable.ts:590`; `reference/bifrost/plugins/telemetry/main.go:230`; `reference/bifrost/plugins/otel/main.go:570`; `reference/ai-gateway/internal/metrics/metrics_impl.go:131`; `reference/gateway/src/middlewares/log/index.ts:7` | Request log/cost/latency/error is P0; alert/SLO loop is P1. | P0/P1 |
| Deployment / Security | `ai-gateway`, `sub2api`, `bifrost`, `gateway`, `litellm` | `reference/ai-gateway/manifests/charts/ai-gateway-helm/templates/deployment.yaml:6`; `reference/sub2api/deploy/docker-compose.yml:14`; `reference/bifrost/core/utils.go:488`; `reference/gateway/src/middlewares/requestValidator/index.ts:25`; `reference/litellm/schema.prisma:12` | Security hardening and deploy defaults are required; Kubernetes/Envoy is later. | P0/P2 |
| Provider / Model / Auth Registry | `new-api`, `llmgateway`, `litellm`, `CLIProxyAPI`, `CliRelay`, `lobe-chat`, `openai-oauth` | `reference/new-api/model/channel.go:23`; `reference/llmgateway/packages/models/src/providers.ts:95`; `reference/litellm/litellm/litellm_core_utils/get_llm_provider_logic.py:137`; `reference/CLIProxyAPI/sdk/cliproxy/auth/conductor.go:3091`; `reference/openai-oauth/packages/openai-oauth-core/src/auth.ts:256` | Strong for P0 provider registry and P1 OAuth/account pool. | P0/P1 |
| Plugin / Hook / Skill System | `gateway`, `bifrost`, `coze-studio`, `dify`, `open-webui`, `LibreChat`, `lobehub` | `reference/gateway/src/middlewares/hooks/index.ts:253`; `reference/bifrost/core/bifrost.go:6524`; `reference/coze-studio/backend/domain/plugin/service/plugin_release.go:67`; `reference/dify/api/core/plugin/plugin_service.py:601`; `reference/open-webui/backend/open_webui/utils/tools.py:430` | P0 should include bounded tool lifecycle, not full marketplace/plugin economy. | P0/P1 |
| Session / Runtime / Event Model | `CLIProxyAPI`, `CliRelay`, `bifrost`, `coze-studio`, `dify`, `LibreChat` | `reference/CLIProxyAPI/internal/wsrelay/http.go:30`; `reference/CLIProxyAPI/internal/watcher/watcher.go:64`; `reference/bifrost/transports/bifrost-http/websocket/session.go`; `reference/coze-studio/backend/domain/workflow/internal/compose/workflow_run.go:46`; `reference/dify/api/services/workflow_event_snapshot_service.py:79` | Runtime events and reconnect/resume are P1 unless needed for P0 tool/relay trace. | P1 |
| Sandbox / Tool Execution | `bifrost`, `open-webui`, `FastGPT`, `coze-studio` | `reference/bifrost/core/mcp/codemode/starlark/utils.go:421`; `reference/open-webui/backend/open_webui/utils/tools.py:430`; `reference/FastGPT/projects/mcp_server/src/index.ts:55`; `reference/coze-studio/backend/domain/plugin/service/tool/invocation_http.go:59` | P0 requires policy/approval/timeout/trace; full code-mode ecosystem is later. | P0/P1 |
| Multi-agent / Team Runtime | `dify`, `LibreChat`, `bifrost`, `lobehub` | `reference/dify/api/core/workflow/nodes/agent_v2/agent_node.py:217`; `reference/LibreChat/api/server/controllers/agents/openai.js:421`; `reference/bifrost/core/mcp/agent.go:140`; `reference/lobehub/packages/agent-gateway-client/src/client.ts:144` | Not P0 except single-agent tool loop. | later |
| IDE / Local Code Editing Workflow | `CLIProxyAPI`, `CliRelay`, `claude-code-api`, `copilot-api`, `openai-oauth`, `lobehub` | `reference/CLIProxyAPI/internal/runtime/executor/codex_executor.go:253`; `reference/CliRelay/internal/tui/oauth_tab.go:21`; `reference/claude-code-api/src/services/claude.ts:131`; `reference/copilot-api/src/start.ts:67`; `reference/lobehub/apps/desktop/src/main/services/gatewayConnectionSrv.ts:24` | Useful but outside core product loop. | later |
| CLI / TUI / SDK / Remote Bridge | `CLIProxyAPI`, `CliRelay`, `codex-oauth`, `openai-oauth`, `copilot-api`, `claude-code-api`, `NextChat` | `reference/CLIProxyAPI/sdk/cliproxy/auth/scheduler.go:240`; `reference/CliRelay/internal/tui/oauth_tab.go:21`; `reference/codex-oauth/codex_oauth.py:93`; `reference/openai-oauth/packages/openai-oauth/src/server.ts:53`; `reference/NextChat/src-tauri/src/stream.rs:34` | P1/P2/later, depending on target market. | later |

## V3 Gaps Newly Made Concrete

- Relay route coverage now needs explicit supported/unsupported endpoint contracts, not broad OpenAI-compatible claims. Evidence: `reference/new-api/router/relay-router.go:153`, `reference/one-api/router/relay.go:20`.
- Gateway security now has concrete SSRF/custom-host validation references. Evidence: `reference/gateway/src/middlewares/requestValidator/index.ts:25`, `reference/bifrost/core/utils.go:488`.
- Provider routing must include key/account pools, model allow/deny lists, sticky sessions, retry classification, failover, and observable failure reasons. Evidence: `reference/sub2api/backend/internal/service/gateway_service.go:588`, `reference/bifrost/core/bifrost.go:7358`.
- Usage billing should be reserve/settle/refund or equivalent ledger semantics, not estimated chat usage. Evidence: `reference/new-api/service/billing_session.go:20`, `reference/one-api/relay/billing/billing.go:23`, `reference/CPA-Manager/usage-service/internal/store/store.go:119`.
- RAG requires async parse/chunk/embed jobs, reparse, cancel, delete/reindex, retrieval debug, and citations. Evidence: `reference/ragflow/rag/svr/task_executor.py:270`, `reference/MaxKB/apps/knowledge/task/embedding.py:61`, `reference/ragflow/rag/nlp/search.py:242`.
- Workflow P0 should be reduced to durable execution/event/trace/log/retry, while Flowise/Dify/FastGPT full canvas surfaces move later. Evidence: `reference/dify/api/services/workflow_event_snapshot_service.py:79`, `reference/Flowise/packages/ui/src/views/canvas/index.jsx:586`.
- Observability now has concrete raw body store, ClickHouse schema, metrics, cost, latency, TTFT, and alert loop references. Evidence: `reference/helicone/clickhouse/migrations/schema_41_request_response_replacing_merge_tree.sql:1`, `reference/bifrost/plugins/telemetry/main.go:422`, `reference/helicone/worker/src/lib/managers/AlertManager.ts:31`.
- MCP/tool systems need lifecycle, auth/OAuth interrupt, execution logs, timeout, user scoping, and fallback boundaries. Evidence: `reference/LibreChat/packages/api/src/mcp/MCPManager.ts:43`, `reference/coze-studio/backend/domain/plugin/service/tool/invocation_http.go:59`, `reference/bifrost/core/mcp/agent.go:35`.

## Not For Oblivious MVP

- Full OpenAI-compatible surface: realtime, batch, files, fine-tuning, Assistants, Threads/Runs.
- Full Flowise/FastGPT/Dify-style visual workflow builder and node marketplace.
- Kubernetes/Envoy `AIGatewayRoute`/ext_proc productization.
- Full desktop/PWA/native bridge and IDE/local code editing workflow.
- Full MCP/plugin marketplace with rankings, reviews, paid installs, and settlement.
- Multi-agent/team runtime beyond a single agent tool loop.
- Advanced OCR/table/figure extraction as a P0 requirement.
- ClickHouse-scale raw body warehouse and alerting suite as a Private Beta blocker.
- Exhaustive LiteLLM-style provider catalog as a completion promise.

## Private Beta / GA Placement

Private Beta must include:

- Chat + Relay + provider registry + request log + usage/billing ledger.
- RAG upload -> job -> parse -> chunk -> embed -> retrieve -> citation with delete/reindex lifecycle.
- Agent run -> tool call -> approval -> trace -> budget with safe tool execution boundaries.
- Workflow trigger -> execution -> logs -> retry with durable status and transition events.
- Billing checkout/webhook/quota/ledger with explicit unsupported/refund/reconcile boundaries.
- Admin control plane with backend-enforced RBAC for users, providers, keys, quota, pricing, and logs.

Commercial Beta should add:

- Broader provider/account pool routing and OAuth account lifecycle.
- Workflow debugger snapshots/replay and richer node registry.
- SLO alerts and dashboard drilldowns.
- Plugin/MCP install/test/uninstall lifecycle.
- Marketplace publish/review/install without full payout settlement.

GA should add:

- Marketplace paid install settlement/refund/payout reconciliation.
- Hardened deployment defaults and production-ready observability stack.
- Advanced RAG parsing, citation anchors, and retrieval debugging.
- Full audit exports, admin operations, and compliance controls.

Later:

- IDE/local code bridge, desktop gateway, PWA sync, full multi-agent/team runtime, Kubernetes/Envoy CRD layer, and exhaustive provider ecosystem.

## Old P0 Ticket Adjustments

| Old P0 area | V3 decision | Reason |
| --- | --- | --- |
| Real gateway proxy + chat streaming | Keep P0, split | Must split into route contract, provider registry, streaming/cancel, request log, usage ledger, and fail-closed chat. Strong references: `gateway`, `new-api`, `llmgateway`, `litellm`, `bifrost`. |
| Realtime API | Downshift | Realtime is not necessary for Private Beta; keep disabled until auth/origin/prebill/abort settlement/usage capture is designed and proven. |
| Batch API | Downshift | Batch lifecycle is complex async settlement; not needed for first core loop. |
| Relay files API | Split | Attachment/file ACL and mapping may be needed for chat/RAG, but generic OpenAI Files API lifecycle is not P0. |
| Fine-tuning/Assistants/Threads/Runs | Downshift | Explicitly unsupported in references; should remain disabled until post-beta. |
| Relay price snapshots and provider-source audit | Keep P0 | SQL catalog and immutable Relay usage price snapshots are in place; provider-source audit/import and reconciliation are still required for every billable relay/chat flow. |
| Durable RAG ingestion/delete/reindex | Keep P0, split | Ingestion job, embedding retry/dead-letter, delete/reindex, and citation payload should be separate closable tickets. |
| Agent sandbox | Keep P0 narrowly | P0 is safe tool execution/approval/timeout/trace, not full code-mode/MCP ecosystem. |
| Workflow durable state/debugger | Split | Durable execution transitions/logs are P0/P1; full visual debugger/replay is P1. |
| Marketplace external payout | Downshift | Full payout/settlement is GA-grade; marketplace should not block Private Beta. |
| Observability request-log/SLO | Split | Request log/cost/latency/error is P0; alert/SLO loop is P1. |
| Secret/plaintext and WebSocket origin | Keep security P0 where route is enabled | Enforce production deny policies and keep disabled surfaces off until hardened. |

## Next Rescan Needs

This v3 pass closes project-root coverage, but future audits should not rely on it as implementation proof. The remaining useful follow-up is a targeted Oblivious-side implementation audit against the v3 product requirements, not another broad reference scan.

The single next implementation ticket recommended from this evidence is:

`P0-Execution-Evidence-Spine`: implement and verify one end-to-end Chat + Relay request path that is fail-closed, provider-routed, truly streamed, request-logged, usage-metered, price-snapshotted, quota-settled, admin-visible, and trace-linked. This is not an implementation in this audit; it is the narrowest next ticket implied by the reference evidence.
