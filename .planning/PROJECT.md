# Oblivious

## What This Is

Oblivious 是一个多租户 AI 平台，整合 LobeHub（C 端体验）和 New-API（B 端运营能力）。目标能力包括 Chat、Agent 编排、知识库、多渠道 LLM Relay、Admin 管理和 Agent Marketplace。

**技术栈**: Go 后端 (Gin) + React 前端 (Vite) + PostgreSQL + Redis

## Core Value

**统一的多渠道 LLM 调用层** — 所有 AI 调用必须经过 Relay，确保计费、限流、监控统一。

## Current Milestone: v08 Product Completeness (Complete)

**Goal:** 去掉面向客户的 MVP/placeholder 行为，使 Chat、Agent、Knowledge、MCP、Admin、Marketplace、公共文档、onboarding、pricing 和端到端商业旅程与“可直接部署商用”的承诺一致。

**Target features:**
- Built-in MCP tools either use real providers/parsers with tenant-safe configuration or are disabled from default commercial use.
- Durable Agent workflows with persisted tool runs, approval points, memory injection, observable execution state, and failure/retry evidence.
- Knowledge behavior and product copy aligned, including embedding-backed RAG and source citation if the product claims RAG.
- Chat, Agent, Knowledge, Admin, and Marketplace UX free of enabled customer-facing placeholders.
- Public docs, onboarding, pricing, and operator guides that match implemented commercial behavior.
- End-to-end commercial journeys that prove signup, organization setup, provider/channel configuration, subscription/top-up, Chat, Agent, Knowledge, Marketplace publish/install, billing inspection, deploy, backup, and restore.

## Current State: Commercial Complete After v08 Phase 30

The commercial complete target is defined in `docs/superpowers/specs/2026-05-27-commercial-complete-program-design.md`. That spec explicitly says the prior release-candidate state is not the final product.

v04 Commercial Foundation is complete: Phase 9 completed first-class organization tenants and the migration ledger, Phase 10 completed memberships/auth security, Phase 11 completed tenant scope across core domains, and Phase 12 completed reproducible DB-backed CI and commercial gate evidence.

v05 Relay Billing Completeness is complete. Phase 13 completed Relay endpoint classification and production fail-closed behavior. Phase 14 completed provider-bypass CI checks, supported endpoint auth/tenant identity policy, rate-limit policy, route-decision audit semantics, and trusted Relay metadata for Chat, Agent, and Knowledge embedding paths. Phase 15 completed quota preauthorization, exactly-once settlement, refund behavior, provider usage parsing, explicit route billing policy, and streaming/async production-disablement evidence. Phase 16 completed Relay route-table evidence, commercial gate closeout, DB-backed verification, and v05 milestone snapshots.

v06 Billing And Marketplace Operations is complete. Phase 17 completed the first money-movement slice by replacing the existing unmounted/partial Stripe code with running route authority, tenant-aware checkout metadata, signature-verified webhooks, and an idempotent webhook ledger. Phase 18 completed subscription, invoice, top-up, refund, failed-payment, and plan-change state transitions through an auditable lifecycle service. Phase 19 completed Marketplace settlement, payout-state modeling, refund impact, takedown, appeal, abuse reporting, and governance event evidence. Phase 20 completed Admin billing inspection APIs/UI and v06 closeout evidence.

v07 Production Operations is complete. Phase 21 completed equivalent production orchestration proof through migration-aware Docker compose validation, a local pgvector fallback image, configurable host ports, app/Relay smoke coverage, restricted-network evidence, and explicit missing-`kubectl` behavior. Phase 22 completed PostgreSQL backup/restore smoke with migration ledger checksum verification and commercial tenant fixture recovery. Phase 23 completed structured logs, Prometheus metrics, OpenTelemetry hooks, error-reporting primitives, alert rules, a Grafana dashboard artifact, SLO docs, and quality-gate coverage for OPS-04/OPS-05. Phase 24 completed release/rollback, incident response, disaster recovery, default-command runtime smoke, restricted-network/fallback runtime smoke, docs gates, and v07 evidence artifacts. Fresh Docker Hub daemon pulls and Kubernetes cluster proof remain environment-specific; the current repository proof records those boundaries instead of hiding them.

Phase 25 MCP Tool Commercial Behavior is complete. It closed `PROD-01`: `calculator` now uses a bounded arithmetic parser, `datetime` remains a real RFC3339 built-in, and `web_search`/`http_request` are disabled from default commercial Agent use unless real provider or tenant-safe outbound policy configuration exists. Agent tool-definition, available-tool, and executor paths enforce the same policy, with focused tests, API docs, commercial gate docs, and quality gates.

Phase 26 Durable Agent Workflows is complete. It closed `PROD-02`: Agent runs and tool runs are persisted with organization scope, approval-required tools pause before execution, rejection and retry state transitions are visible through tenant-scoped APIs, failed tool executions persist error/attempt evidence, and memory search evidence is recorded on Agent runs. Focused DB-backed Agent/HTTP tests, API docs, commercial gate docs, and quality gates cover the behavior.

Phase 27 Knowledge Product Promise Alignment is complete. It closed `PROD-03` only by upgrading Knowledge from text/snippet search to Relay embedding-backed ingestion, pgvector chunk retrieval, and source citations, while updating the UI, API docs, and quality gates to prevent customer-facing RAG overclaims.

Phase 28 Commercial UX and Journey Hardening is complete. It closed `PROD-04` only: Chat, Agent/SOLO, Knowledge, Admin, and Marketplace journeys now expose quota/budget/Relay/settlement boundaries, recoverable errors, commercial empty/action states, and no enabled fake commercial behavior. Focused frontend tests, docs gates, and diff hygiene are recorded in `.planning/phases/28-commercial-ux-and-journey-hardening/28-VERIFICATION.md`.

Phase 29 Public Docs Onboarding Pricing and Operator Guides is complete. It closed `PROD-05` only: README, public overview, onboarding, pricing, operator guide, API docs, architecture contracts, commercial gates, and quality gates now align with implemented tenant, Relay, billing, Marketplace, operations, and product behavior. Docs verification and stale-wording scans are recorded in `.planning/phases/29-public-docs-onboarding-pricing-and-operator-guides/29-VERIFICATION.md`.

The overall commercial-complete SaaS objective is complete for the current repository-local evidence model after Phase 30 strict verification.

Phase 30 End-to-End Commercial Journey and Final Audit is complete. It added DB-backed backend journey evidence, browser journey evidence, a strict commercial completion verifier script, final commercial completion audit, deploy validation, backup/restore smoke, docs gates, and diff hygiene. `PROD-06`, `AUDIT-01`, the Product Completeness Gate, and final commercial readiness are closed in `.planning/phases/30-end-to-end-commercial-journey-and-final-audit/30-VERIFICATION.md`.

## Requirements

### Validated

- ✓ 用户认证与会话管理 — Auth/Session 模块
- ✓ Chat 会话与消息管理 — Chat 模块
- ✓ 知识库基础存储 — Knowledge 模块
- ✓ 用户偏好设置 — Preferences 模块
- ✓ 控制台使用量展示 — Console 模块
- ✓ Relay 独立模块 — Handler + Router + Billing + Metrics
- ✓ 前端骨架 — 营销页、工作区、控制台页面
- ✓ Relay 挂载、Chat RelayGateway、Agent Runtime、MCP Client — Phase 1
- ✓ Agent 工具循环、Memory HNSW、Quota-Billing 串联 — Phase 2
- ✓ Admin 与 Marketplace 后端 API — Phase 3a
- ✓ Admin 管理面板与 Agent Marketplace UI — v03.1
- ✓ v03.2 Quality and Release — backend integration, E2E, API/RC docs, and Docker compose runtime validation
- ✓ v03.3 Mainline Consolidation — commit-boundary triage, backend hardening, frontend/E2E/deployment alignment, contract docs, release verification, and accepted cleanup debt closeout
- ✓ v04 Commercial Foundation — tenant model, membership/auth security, tenant-scoped domains, DB-backed CI, and commercial gate evidence
- ✓ RELAY-08 — Every registered `/v1/*` route is classified as commercial-supported and billed, internal/admin-only, or disabled in production
- ✓ RELAY-09 — Unsupported or partially implemented `/v1/*` endpoints fail closed in production before any upstream provider call
- ✓ RELAY-10 — CI proves app services do not import provider SDKs or call direct provider URLs outside Relay/channel adapters
- ✓ RELAY-11 — Supported Relay endpoints enforce tenant identity, auth policy, rate-limit policy, and audit semantics
- ✓ BILL-01 — Supported Relay calls pre-authorize quota, settle exactly once per idempotency key, and refund failed or partial calls
- ✓ BILL-02 — Streaming/realtime, file, batch, and async flows have explicit settlement models or are production-disabled
- ✓ PAY-01 — Stripe checkout routes are mounted, authenticated, tenant-aware, and testable without live Stripe keys
- ✓ PAY-02 — Stripe webhook route verifies raw-body signatures, records provider events idempotently, and rejects invalid signatures
- ✓ PAY-03 — Subscription lifecycle, invoices, refunds, failed-payment states, plan changes, and top-ups are auditable/idempotent lifecycle transitions
- ✓ MARKET-03 — Marketplace publisher revenue, platform fee, payout state, and refund impact are modeled before paid operation
- ✓ MARKET-04 — Marketplace moderation and abuse workflows cover takedown, appeal, reinstate, abuse, and audit paths
- ✓ ADMIN-BILL-01 — Admin can inspect billing sessions, webhook events, subscriptions, top-ups, invoices, refunds, settlements, and payout state
- ✓ DOC-05 — v06 evidence maps money-movement and Marketplace governance requirements to files, tests, runtime/database proof, and residual v07/v08 work
- ✓ OPS-01 — Equivalent production orchestration validation starts the actual stack, applies migrations, and proves health, metrics, app, and Relay paths without live provider secrets
- ✓ OPS-03 — Backup and restore smoke proves PostgreSQL tenant-commercial data can be backed up and restored with migration ledger integrity
- ✓ OPS-04 — Structured logs, Prometheus metrics, OpenTelemetry tracing hooks, and error-tracking integration points cover HTTP, Relay, billing, jobs, and provider failures
- ✓ OPS-05 — Alert rules, dashboards, and SLO definitions exist for Relay outage, quota settlement failure, webhook failure, migration failure, high provider error rate, and tenant isolation incidents
- ✓ OPS-02 — Runtime smoke covers default-command and restricted-network/fallback deployment paths with skipped Kubernetes proof recorded explicitly
- ✓ OPS-06 — Release, rollback, incident response, and disaster recovery runbooks are documented and verified against deployment, restore, alert, and evidence commands
- ✓ DOC-06 — v07 evidence maps production-operations requirements to scripts, manifests, runbooks, smoke output, skipped checks, residual v08 work, and no-final-readiness boundary
- ✓ PROD-01 — Built-in MCP tools are real by default or disabled from default commercial use
- ✓ PROD-02 — Agent workflows are durable and observable instead of relying on placeholder tool output
- ✓ PROD-03 — Knowledge behavior matches product copy through Relay embedding-backed RAG, pgvector chunk retrieval, and source citations
- ✓ PROD-04 — Chat, Agent, Knowledge, Admin, and Marketplace customer journeys expose commercial error, quota, budget, review, settlement, and operation boundaries without enabled fake behavior
- ✓ PROD-05 — Public docs, onboarding, pricing, and operator guides align with implemented tenant, Relay, billing, Marketplace, operations, and product behavior
- ✓ PROD-06 — End-to-end commercial journeys pass across signup, organization, provider, subscription, Chat, Agent, Knowledge, Marketplace, billing, deploy, backup, and restore
- ✓ AUDIT-01 — Final commercial completion audit maps every commercial gate to evidence before final readiness is claimed

### Active

- None for v08 Product Completeness.

### Out of Scope For v08

- Mobile-specific experience; web remains the primary control plane.

## Context

**整合背景**: 项目是 LobeHub (B2C) + New-API (B2B) 整合到同一主线的中间产物。当前商业目标不是继续打磨 RC，而是把它推进到可直接部署商用的 SaaS 完全体。

**当前架构**:
```
React Frontend
    │ HTTP/WebSocket
    ▼
Go Backend (Gin)
    ├── API Gateway (Auth/CORS/Recovery)
    ├── Service Layer (Auth/Chat/Agent/Knowledge/Task/Memory/Admin/Marketplace)
    ├── Relay Layer (所有 AI 调用的统一入口)
    └── Data Layer (PostgreSQL + MongoDB)
```

**当前状态**:
- v03.1 已交付可用 Admin UI 与 Marketplace UI。
- v03.2 已完成质量、E2E、文档和 Docker 部署 smoke 收口。
- v03.3 已完成主线整合、文档对齐、发布验证和两个历史 cleanup backlog。
- v04 Commercial Foundation 已完成。
- v05 Relay Billing Completeness 已完成；v06 Billing And Marketplace Operations 已完成，Phase 17 已完成 Stripe route authority 和 webhook ledger，Phase 18 已完成支付生命周期状态机，Phase 19 已完成 Marketplace settlement and governance，Phase 20 已完成 Admin billing inspection 和 v06 closeout。
- v07 Production Operations 已完成；Phase 21 Production Orchestration Runtime Proof、Phase 22 Backup Restore and Migration Recovery、Phase 23 Observability Alerts Dashboards and SLOs、Phase 24 Release Rollback Incident DR and v07 Closeout 均已有验证证据。
- v08 Product Completeness 已完成 Phase 25 至 Phase 30；最终商业 readiness 已由严格无跳过 verifier、deploy validation、backup/restore smoke、docs gate 和 diff hygiene 记录。
- 直接 Docker Hub fresh daemon pull 和默认 Go proxy 在本机网络仍不稳定；Phase 24 修复了 Dockerfile module cache 复用问题，并通过默认命令 smoke 与受限网络/fallback smoke 记录 OPS-02。
- 当前环境只有 Docker 可用，`kubectl`、`kind`、`minikube` 未安装；Kubernetes 验证脚本在缺少集群工具时明确失败并记录为非成功证据。

## Constraints

- **技术栈**: Go 1.22+, Node.js 20+, PostgreSQL 14+, Redis
- **架构**: Relay 必须作为所有 LLM 调用的统一入口
- **计费**: Chat/Agent/Relay 消耗必须最终进入统一 quota/billing 账本
- **隔离**: 组织/租户隔离必须继续作为所有商业功能的安全边界
- **工作树安全**: 当前根工作树存在 unrelated dirty/untracked 文件；商业规划和后续实现提交必须保持窄范围并继续在 active worktree 中推进
- **商业完成定义**: 任何 milestone 只能在对应 gate 有当前仓库证据、自动化验证和必要 runtime smoke 后才可宣称完成

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go 统一后端 | Agent Runtime、MCP Tools、Relay 全部 Go 重写 | ✓ Good — Phase 1/2/3 APIs built on Go |
| Relay 作为统一入口 | 计费/限流/监控统一 | Active — v05 now proves this for every commercial AI surface |
| pgvector 向量检索 | PostgreSQL 原生支持，运维简单 | ✓ Good — HNSW migration shipped in Phase 2 |
| Admin/Marketplace UI on shared primitives | Keep page work consistent and testable | ✓ Good — v03.1 closed with focused Vitest coverage |
| Docker compose runtime path satisfies DEPLOY-01 | Requirement accepted one real Docker or Kubernetes runtime path | ✓ Good — compose build/start/smoke passed; Kubernetes remains later |
| Preserve living REQUIREMENTS.md | This repo uses it for cross-phase context and archives milestone snapshots separately | ✓ Good — Phase 999.2 recorded this policy |
| Commercial target is a milestone program, not one giant phase | Tenant/security, Relay billing, money movement, operations, and product completeness have hard dependencies | Complete — v04 through v08 are complete, including Phase 30 final journey and audit |
| v04 starts with tenant/security foundation | Billing, Marketplace payouts, and production ops need tenant identity and isolation first | ✓ Good — v04 completed tenant/security/migration/CI foundation |
| v05 starts with route policy and fail-closed enforcement | Billing semantics are unsafe until every `/v1/*` route has an explicit commercial class and production behavior | ✓ Good — Phase 13 through Phase 16 closed v05 Relay Billing Completeness |
| Phase 13 disables partial Relay endpoints in production first | Passthrough/file/async endpoints must not reach providers before billing/audit/settlement semantics exist | ✓ Good — `3b9d4dd` and `docs/release/relay-route-table.md` |
| Phase 14 makes provider bypass and supported-route identity testable before settlement | Billing cannot be trusted if app services can still bypass Relay or supported routes lack tenant identity/audit policy | ✓ Good — `scripts/verify-relay-security.sh`, trusted internal identity guard, route-decision audit sink, and Memory Relay metadata tests |
| Phase 15 makes settlement/refund explicit before v05 closeout | Relay Authority Gate cannot close while supported calls can strand quota or streaming/async flows bypass settlement | ✓ Good — `RouteWithBilling` lifecycle tests, `BillingPolicy` route coverage, provider usage parsing, and route-table evidence |
| Phase 16 closes v05 with evidence rather than new runtime behavior | Relay behavior changed in Phases 13-15; v05 closeout needs reproducible proof and must not imply final commercial readiness | ✓ Good — `16-VERIFICATION.md`, route table/gate docs, docs gate assertions, DB-backed script verification, and milestone snapshots |
| v06 starts with Stripe route authority and webhook ledger | Subscription and Marketplace settlement cannot be safe until payment provider events are signature-verified, idempotent, tenant-aware, and inspectable | ✓ Good — Phase 17 mounted checkout/webhook routes, `payment_intents`, `stripe_webhook_events`, and DB-backed route tests |
| Phase 18 applies verified provider events through a lifecycle service | Commercial money movement needs subscriptions, invoices, top-ups, failed-payment state, plan changes, and refunds to be auditable and idempotent before Marketplace settlement | ✓ Good — `billing_lifecycle_events`, `billing_invoices`, `billing_refunds`, lifecycle retry/idempotency tests, and DB-backed route tests close PAY-03 |
| Phase 19 starts from paid-install settlement rather than admin UI | Marketplace paid operation is unsafe until orders, settlements, fees, payout state, refund impact, and governance trails exist | ✓ Good — orders, settlements, payout-state modeling, refund impact, governance routes/events, abuse workflow, publisher financial stats, and DB-backed route tests close MARKET-03/MARKET-04 |
| Phase 20 closes v06 with inspection and evidence rather than new money movement | v06 money state existed after Phases 17-19, but operators needed read-only Admin inspection and reproducible closeout evidence before the Billing gate could close | ✓ Good — Admin billing routes/UI, `20-VERIFICATION.md`, docs updates, and v06 snapshots close ADMIN-BILL-01/DOC-05 |
| v07 starts with runtime proof before runbooks | Operators cannot trust backup, alert, or incident runbooks until the deployment path can start the stack, apply migrations, and smoke app/Relay routes reproducibly | ✓ Good — Phase 21 through Phase 24 closed v07 Operations Gate evidence |
| Phase 22 treats recovery as executable smoke, not prose | Commercial operations readiness needs proof that tenant, billing, Marketplace, audit, and migration-ledger data survive restore into a fresh database | ✓ Good — `backup-restore-smoke.sh`, `22-VERIFICATION.md`, and `backup-restore-runbook.md` close OPS-03 |
| Phase 23 starts from code-level observability before runbook closeout | Phase 24 incident/runbook evidence needs real logs, metrics, tracing hooks, error-reporting integration points, alerts, dashboards, and SLO definitions to reference | ✓ Good — `23-VERIFICATION.md`, alert/dashboard/SLO artifacts, docs gate, and focused package tests close OPS-04/OPS-05 |
| Phase 24 fixes Dockerfile module-cache reuse before v07 closeout | Bare default deployment validation failed after image metadata because `go build` did not mount the `/go/pkg/mod` cache populated by `go mod download`; fixing the Dockerfile made default-command runtime smoke reproducible without lowering evidence standards | ✓ Good — `Dockerfile.server`, docs gate regression check, and `24-VERIFICATION.md` close OPS-02/OPS-06/DOC-06 |
| Phase 25 disables unsafe/fake MCP built-ins by default | Commercial Agents must not expose fake search output or raw outbound HTTP without provider/egress policy, while safe built-ins should remain useful | ✓ Good — real `calculator`/`datetime`, default-disabled `web_search`/`http_request`, Agent enforcement, docs gates, and focused tests close PROD-01 |
| Phase 26 makes Agent tool workflows durable before Knowledge/UX closeout | Product completeness requires observable Agent execution state before broader customer journey hardening | ✓ Good — `agent_runs`, `agent_tool_runs`, approval/reject/retry APIs, memory/failure evidence, tenant-scoped tests, docs, and gates close PROD-02 |
| Phase 27 chooses real Knowledge RAG instead of copy downgrade | The final product promise needs Knowledge to support commercial RAG behavior through Relay embeddings, pgvector retrieval, and source citations rather than preserving text search under narrower wording | ✓ Good — Relay embeddings, pgvector chunk retrieval, source citations, UI rendering, docs, gates, and focused tests close PROD-03 |
| Phase 28 starts from active customer journeys, not marketing copy | PROD-04 requires the routed Chat, Agent/SOLO, Knowledge, Admin, and Marketplace experiences to show commercial boundaries and recoverable errors before docs or final E2E closeout can be trusted | ✓ Good — Chat, SOLO/Agent, Knowledge, Marketplace, and Admin focused frontend tests, docs gates, and `28-VERIFICATION.md` close only PROD-04 |
| Phase 29 treats docs as product contract, not marketing copy | Public docs, onboarding, pricing, and operator guides must match implemented tenant, Relay, billing, Marketplace, operations, and product behavior before final E2E closeout can be trusted | ✓ Good — README, product docs, API/architecture contracts, commercial gates, quality gates, stale scan, and `29-VERIFICATION.md` close only PROD-05 |
| Phase 30 requires fresh strict evidence rather than inherited phase prose | Final commercial readiness needs one current evidence chain spanning product, billing, Relay, operations, recovery, and audit; environment skips cannot count as readiness | Complete — strict verifier, deploy validation, backup/restore smoke, docs gate, diff hygiene, `30-VERIFICATION.md`, and `30-01-SUMMARY.md` close the final gate |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition**:
1. Requirements invalidated? Move to Out of Scope with reason.
2. Requirements validated? Move to Validated with phase reference.
3. New requirements emerged? Add to Active.
4. Decisions to log? Add to Key Decisions.
5. "What This Is" still accurate? Update if drifted.

**After each milestone**:
1. Full review of all sections.
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state.

---
*Last updated: 2026-05-29 after completing Phase 30 End-to-End Commercial Journey and Final Audit*
