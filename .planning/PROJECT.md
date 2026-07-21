# Oblivious Commercial Complete & Target Release

## What This Is

Oblivious 是一个面向组织工作区的 multi-tenant AI SaaS 平台。它在统一产品中提供 Chat、Knowledge RAG、Agent/SOLO、Workflow、Task、MCP 工具、多渠道发布、Admin 和 Marketplace，并通过 Relay 统一所有可计费 AI 操作的 Provider 路由、鉴权、限流、配额、计价、用量、审计和监控。

本项目不是从零重构，也不是把现有仓库缩减成 MVP 或 RC。目标是在保留当前 brownfield 实现和有效技术决策的基础上，补齐真实运行断点、商业与运维闭环、声明部署模式的能力对等，以及目标环境 no-skip 发布证据，最终形成可直接部署、可收费、可运营、可审计、可恢复的商业产品。

## Core Value

让组织客户能够可靠地构建、运行并商业化 AI 应用，同时让每一次 AI 操作都可隔离、可计费、可追踪、可审计、可恢复。

## Business Context

- **Customer**: 使用 AI 完成知识工作和自动化的组织客户，以及构建 Agent、Workflow、Knowledge 应用并通过 Marketplace 分发的开发者与发行商。
- **Revenue model**: SaaS 套餐、配额与按量计费、充值、Provider 用量结算，以及 Marketplace 平台服务费和分成。
- **Success metric**: 所有已承诺商业旅程在真实目标环境可完成，并由同一发布 commit 的 no-skip 商业验证和外部证据包证明。
- **Strategy notes**: `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md` 定义已批准的上线基线与完整商业终局；`docs/audit/2026-07-10-full-repository-scan.md` 和 `docs/audit/2026-07-10-module-capability-gap-matrix.md` 提供当前差距基线。

## Users and Critical Journeys

### Organization Members

- 注册或登录、加入组织、选择活动组织并在明确的角色边界内工作。
- 在 Chat 中选择模型与参数、绑定 Knowledge、获得真实流式响应、查看引用和配额错误，并将对话转换为 SOLO 或 Task。
- 创建和运行 Agent、Workflow 与 Scheduled Task，查看持久执行状态，处理审批、拒绝、重试、取消和失败恢复。

### Builders and Developers

- 配置 Agent、工具、MCP、Knowledge、Workflow、API、Webhook 和渠道发布能力。
- 调试执行、检查输入输出和 trace、管理版本与分支，并在明确授权和出站策略下运行外部集成或沙箱代码。
- 通过稳定的 OpenAI-compatible `/v1/*` 和产品 API 接入平台，而不绕过 Relay 或租户治理。

### Owners, Admins, and Finance Operators

- 管理组织、成员、权限、Provider、渠道、模型、路由、价格、套餐、配额和安全策略。
- 检查 usage、request log、账单、订阅、充值、发票、退款、支付事件、Marketplace 结算和 payout。
- 审核 Marketplace 内容，处理申诉、滥用、下架、恢复和高风险操作审计。

### Marketplace Publishers

- 创建、版本化、提交审核、发布和下架 Agent、Workflow、工具或模板。
- 完成免费或付费安装、订单、退款影响、收入统计、结算、payout 和申诉治理。

### Platform Operators

- 执行迁移、部署、扩缩容、监控、告警、备份恢复、发布回滚、事故响应和灾难恢复。
- 从 request 到 execution、usage、billing、payment、settlement、audit 和 trace 进行端到端联查。
- 生成与发布 commit 一致、位于仓库外且不含秘密的目标环境证据包。

## Requirements

### Validated

以下能力已在当前 brownfield 生产代码和仓库本地证据中存在。这里的 Validated 表示可作为新工作的既有基线，不代表已经取得最新 target/live 商业发布证明。

- Existing multi-tenant foundation: 组织、成员、会话、安全中间件、租户作用域和审计基础已存在。
- Existing Relay foundation: Provider/渠道路由、生产 fail-closed 策略、配额与结算基础、价格快照、usage 和部分 request-log 路径已存在。
- Existing customer product: Chat、Agent/SOLO、Knowledge RAG、Workflow、Scheduled Task、Admin 和 Marketplace 均有生产代码、持久化模型和前端操作面。
- Existing durable data paths: PostgreSQL migrations、pgvector/Qdrant 检索路径、Agent/Workflow 执行状态、重试/审批及部分后台 job 模型已存在。
- Existing monetization foundation: Stripe、订阅、充值、发票、退款、quota、Marketplace 订单、结算和 payout 状态模型已存在。
- Existing operations foundation: Docker、Kubernetes manifests、Prometheus/Grafana 资产、迁移、备份恢复、发布回滚、事故响应和 DR 文档与本地验证路径已存在。
- Existing verification foundation: Go、Vitest、Playwright、DB evidence、OpenAPI、security、commercial verifier 和 target-evidence 工具链已存在。
- Phase 31.1 dynamic readiness and fail-closed release commitment: 进程内 `ReadinessManager`、当前 generation guard、严格 runtime authority/effect exact-join、部署依赖 probe 和 repository-local E2 Stage B 已在 Phase 31.1 以 E1/E2 证据验证；final tracking-HEAD Stage B、push、target/live 与商业发布证明仍未执行。

### Active

- [ ] 所有发布承诺中的客户旅程使用真实前端、Go 后端、数据库和适用外部服务完成，不以 API fixture、mock Provider 或文档存在代替运行证明。
- [ ] Relay 成为所有可计费 AI 操作的唯一权威路径，完整覆盖鉴权、租户、路由、流式与取消、usage、quota、计价、结算、退款、request log 和审计。
- [ ] 所有客户状态、向量数据、后台 job、重试队列、管理查询和微服务调用都按 organization scope 授权并具有跨租户拒绝证据。
- [ ] RAG ingestion/index workers、SOLO/Task target execution、Agent structured streaming 与 sandbox、Workflow replay/debug，以及声明支持的渠道和工具形成真实耐久运行闭环。
- [ ] 所有用户可控 URL、Provider base URL、Webhook、MCP、工具、Workflow HTTP node、渠道和 payout 出站操作使用统一的 fail-closed outbound security policy。
- [ ] 订阅、充值、usage ledger、quota、发票、支付、退款、Marketplace 订单、结算、payout、chargeback 和 reconciliation 形成幂等且可审计的资金闭环。
- [ ] Gateway、Relay、RAG、Agent、Workflow、Billing、Marketplace、Admin、Channel、Task 和 Observability 在所有对外声明的部署模式中具有能力对等、身份传播、健康检查和 target smoke。
- [ ] Provider、PostgreSQL/pgvector、Qdrant、ClickHouse、Redis、Kafka、对象存储、支付、payout、Kubernetes、可观测性和恢复路径取得新鲜的目标环境证据。
- [ ] CI 和发布门禁覆盖生产构建、单元、集成、契约、真实 E2E、race、lint、安全、依赖、迁移、恢复和必要负载基线；关键商业路径不得静默 skip。
- [ ] 发布制品可复现、不可变、已签名并附带 SBOM/provenance；最终 strict verifier 在目标环境无 skip 通过，证据与发布 commit 一致。

### Out of Scope

- 直接复制 `reference/` 中的源码、身份模型、账本语义或部署假设 - 参考仓库仅用于能力深度和交互模式对照。
- 为追求旧文档中的固定数字而实现 100+ Provider、150+ 工具、10+ 渠道或 20+ 节点 - 支持目录由真实产品需求、生命周期完整度和证据决定。
- 为匹配旧拓扑而一次性重写为固定数量的微服务、Next.js 或指定中间件 - 当前技术栈和渐进迁移优先，领域边界与运行契约比服务数量更重要。
- 在缺少完整身份、计费、治理、补偿和 target proof 时启用 Fine-tuning、Assistants、Threads、Runs 等生命周期 API - 未完成能力保持禁用并 fail closed。
- 把 MAU、GMV 或调用量增长作为代码仓库完成门槛 - 它们是发布后的业务成效指标。
- 将真实 secrets、Kubernetes 凭据、Provider keys、支付密钥、目标 manifest 原始证明或下载 artifact bodies 提交到仓库。

## Platform Invariants

1. **Relay authority**: Chat、Agent、Workflow、Knowledge、MCP 和受支持的 OpenAI-compatible 端点不得绕过 Relay 直连 Provider。
2. **Tenant isolation**: 所有客户拥有的状态必须携带 organization identity，并在 HTTP/gRPC、service、SQL、vector store、后台 job、重试队列和查询层强制授权。
3. **Shared evidence identity**: `organization_id`、`user_id`、`request_id`、`conversation_id`、`run_id`、`execution_id`、`usage_id`、`trace_id`、`payment_id` 和 `settlement_id` 必须可联查。
4. **Fail closed**: 缺少价格、Sandbox、支付渠道、payout、生产观测后端或完整 API 生命周期时必须显式失败，不能降级到 demo、fake、local fallback 或模拟 telemetry。
5. **Evidence hierarchy**: fixture/unit、repository-local runtime、target-environment runtime、final no-skip commercial evidence 是不同证据等级，低等级不能替代高等级。
6. **Current contracts win**: 当前源码、OpenAPI、迁移和运行验证优先于旧设计中的过时路径、技术版本、拓扑和完成描述。

## Completion Definition

项目只有在以下条件全部满足时，才能称为功能完整且可商业发布：

1. 所有承诺上线的模块都有生产装配和真实运行代码，不存在无人消费队列、模拟 target execution 或被当作完整服务发布的 health-only 壳层。
2. Chat、Knowledge、Agent/SOLO、Workflow/Task、Billing 和 Marketplace 的关键浏览器旅程使用真实后端、数据库及适用外部系统完成。
3. Relay 对身份、路由、流式、取消、usage、quota、billing、refund、request log 和 audit 具有唯一且可验证的权威。
4. 组织隔离在路由、服务、数据库、向量存储、队列、审计和管理面均有正向与跨租户拒绝证据。
5. 订阅、支付、退款、Marketplace 交易、结算和 payout 能按 request/payment/settlement identity 对账并安全重放。
6. 所有声明支持的单体、dual-mode 或独立服务部署形态能力对等；未达到的形态从发布承诺和默认配置中移除。
7. 真实 Provider、数据库、向量库、分析库、队列、对象存储、支付、payout、集群、告警和恢复均有 target evidence。
8. CI 和 release gate 对关键路径无静默 skip，并记录环境类别、迁移状态、通过/失败、已跳过检查和残余风险。
9. 发布制品具有可复现构建、不可变 digest、签名、SBOM/provenance、回滚和恢复证明。
10. `scripts/verify-commercial-completion.sh` 在目标环境无 skip 通过，目标 manifest 和 artifact bodies 位于仓库外，工作树与发布 commit 一致，阶段提交已 push。

目标 SLO 在真实目标环境测量：API P95 小于 500 ms，Relay 自身 P95 开销小于 100 ms，RAG 检索 P95 小于 2 s，Workflow 成功率高于 99%，系统可用性目标不低于 99.9%。这些值只能通过明确决策调整，不能因缺少测量而静默删除。

## Delivery Strategy

### Layer 1: Local Runtime Closure

优先修复可由仓库内代码证明的 P0 运行与安全断点，包括统一 outbound policy、耐久 worker 装配、真实 Task target execution、Marketplace payout lifecycle、Gateway 路由权威，以及当前测试和静态检查失败。

### Layer 2: Real End-to-End Journeys

建立不依赖 API fixture 的真实 Chat、RAG、Agent、Workflow、Billing 和 Marketplace 旅程，完成 streaming、cancellation、usage/billing/request-log join、共享对象存储、ClickHouse 和告警恢复闭环。

### Layer 3: Declared Deployment Parity and Target Release

只为实际发布声明的部署形态完成服务能力对等、数据库所有权、gRPC/HTTP 契约、Kubernetes target smoke、供应链证明和外部证据采集，最终执行同一 commit 的 no-skip 商业发布验证。

## Context

- 原始四份 2026-06-04 融合设计定义了产品愿景，但其中固定 Provider/工具/渠道数量、Go/Next.js 版本、12 微服务和部分 API 路径已与当前主线漂移。
- 2026-07-10 批准的 `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md` 是当前能力边界合同，采用“可商用上线基线 + 完整商业终局”的分层路线。
- `README.md` 明确 Phase 30 完成的是仓库状态基线；真实 Provider keys、外部观测部署和 live runtime smoke 仍需要新鲜 target evidence。
- `docs/audit/2026-07-10-full-repository-scan.md` 和模块差距矩阵确认仓库不是空壳，但仍存在运行装配、真实 E2E、微服务 parity、发布供应链和 target proof 缺口。
- 历史 milestone、phase、audit 和 codebase map 保留为证据，不把旧顶层 PROJECT/REQUIREMENTS/ROADMAP/STATE 的完成语言自动继承到新项目。

## Constraints

- **Mainline**: 产品和发布范围是 `src/server`、`src/web`、`api/proto`、`config`、`scripts`、`deploy`、`.github/workflows` 和产品文档；`reference/` 不属于实现或发布证据。
- **Stack**: 延续当前 Go 1.25、React/TypeScript、PostgreSQL/pgvector 及已采用的 Redis、Qdrant、ClickHouse、Kafka、Docker/Kubernetes 集成，除非迁移有可验证收益和回滚路径。
- **Security**: secrets 和原始 target evidence 保持在仓库外；生产配置不得使用 placeholder、sample、fake 或本地 fallback。
- **Compatibility**: 当前 `/v1/*`、产品 API、OpenAPI、数据库迁移和客户端契约不能被旧设计路径覆盖。
- **Delivery**: 按独立可验证模块拆分；并行工作必须保持接口契约一致。每个切片都运行相关测试、更新必要文档、执行 `git diff` 自查、原子 commit，并在验证通过后 push。
- **Claim discipline**: 任何 readiness 结论必须说明 evidence class、环境、命令、迁移状态、通过/失败、skip 和残余风险；缺少 target/live proof 时只能声明 repository-local progress。
- **Change safety**: 保留无关用户改动、历史规划证据和未明确授权的外部环境状态，不通过清理工作树误删证据或秘密。

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| 采用分层商业收口，而非单次大重构 | 先证明核心客户旅程，再完成外部集成与声明部署模式，降低迁移和分布式系统风险 | Pending |
| 以能力合同替代固定功能数量 | Provider、工具、渠道和节点只有在生命周期完整且有证据时才计入支持范围 | Pending |
| 当前运行契约优先于旧拓扑 | 避免为了匹配过时设计而破坏已工作的 Go/React 主线和 `/v1/*` 契约 | Pending |
| 所有未证明能力默认 fail closed | 防止 demo、fixture、fake payment 或不完整生命周期进入商业默认路径 | Pending |
| 只对外声明已完成 parity 的部署模式 | 不把 health-only 或 stub 微服务包装成生产拆分完成 | Pending |
| 采用四级证据模型 | 防止仓库本地绿色测试被误报为 target commercial readiness | Pending |
| 不采用 TDD 流程 | 按实现优先、实现后自动化验证推进；保留回归测试、fixture、静态检查和零匹配防假绿门禁，但不要求先写失败测试 | Active |
| 业务增长指标后置 | MAU、GMV 和调用增长衡量发布后的市场结果，不替代工程和发布验收 | Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `$gsd-transition`):
1. Requirements invalidated? Move them to Out of Scope with a reason.
2. Requirements validated? Move them to Validated with the phase reference and evidence class.
3. New requirements emerged? Add them to Active.
4. Decisions to log? Add them to Key Decisions.
5. Is What This Is still accurate? Update it if reality drifted.

**After each milestone** (via `$gsd-complete-milestone`):
1. Review every section against the live repository and target environment.
2. Recheck whether Core Value is still the right priority.
3. Audit Out of Scope and the reason for each boundary.
4. Update Context with current users, evidence, metrics and residual risks.

---
*Last updated: 2026-07-21 after Phase 31.1 repository-local completion*
