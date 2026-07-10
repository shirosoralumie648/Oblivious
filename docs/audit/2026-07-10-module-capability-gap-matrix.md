# Oblivious 模块能力差距矩阵

日期：2026-07-10

依据：

- `docs/superpowers/specs/2026-07-10-module-capability-reference-design.md`
- 当前生产源码、测试、迁移、部署清单和 2026-07-10 本地验证日志
- `docs/audit/current-implementation-depth.md`
- `docs/audit/oblivious-gap-matrix.md`
- `docs/audit/stub-hardcoded-todo-report.md`
- `docs/audit/vertical-slice-gap-report.md`

本矩阵区分：

- **实现存在**：生产源码中存在真实路径；
- **本地闭环**：源码、测试和本地运行证据能够形成闭环；
- **目标环境闭环**：真实目标数据库、Provider、支付、ClickHouse、Kubernetes 和外部系统中已有证据；
- **商业完成**：最终 no-skip verifier 通过。

评分含义：

| 分数 | 含义 |
|---:|---|
| 0 | 不存在 |
| 1 | 仅文档、类型、接口或壳层 |
| 2 | 有局部生产实现，但主流程不闭环 |
| 3 | 主体实现存在并有本地测试，缺关键运行或外部证据 |
| 4 | 本地端到端闭环，主要缺目标环境证明或商业成熟度 |
| 5 | 目标环境和商业门禁已证明 |

当前没有模块达到 5/5。

## 1. 总览

| 模块 | 当前分数 | 上线基线状态 | 首要阻塞 | 推荐首个切片 |
|---|---:|---|---|---|
| Identity / Tenant | 3.0 | 主体存在 | 细粒度授权和企业身份不足 | 组织角色权限矩阵和跨路由授权证明 |
| API Gateway | 2.0 | 单体入口可用，独立服务偏壳 | 独立 Gateway 没有真实服务代理路由 | 注册真实服务目标、认证传播和路由健康 |
| Relay | 3.5 | 主体较深 | Realtime/Batch/Files 目标生命周期未证明 | Realtime 和 Batch 的真实 request-log/usage/settlement 证明 |
| Chat | 3.0 | 主体存在 | 真实浏览器到 Provider 的流式链路未证明 | 实际后端 SSE、取消和 usage join E2E |
| Knowledge / RAG | 3.0 | 作业和存储存在 | Ingestion/Index Worker 未在生产入口启动 | Worker 装配、优雅关闭和 drain 证明 |
| Agent / SOLO | 3.0 | 主体存在 | 外部 URL 安全、真实流式和沙箱目标证据 | 统一 outbound policy 和结构化流式 |
| Workflow | 3.0 | 持久状态较深 | HTTP 节点安全和目标重放证明不足 | HTTP node outbound policy 和 DB restart replay |
| Task / Scheduler | 1.5 | 状态模型存在 | Task 只模拟步骤和预算，不启动真实执行 | Task target executor 调用 Agent/Workflow |
| MCP / Tools | 3.5 | 工具面较广 | 远程工具授权、网络策略和生命周期不足 | 统一 MCP 凭证、OAuth 和 outbound policy |
| Sandbox | 2.5 | 可选 Docker 路径存在 | 默认未成为所有代码执行的生产权威路径 | 统一 SandboxRunner 和 artifact retention |
| Billing / Quota | 3.0 | Stripe 和账本主体存在 | 国内支付、完整对账和独立服务能力不足 | 支付 Provider 统一 ledger/reconciliation |
| Marketplace | 3.0 | 主体存在 | 外部 payout 和对账未证明 | payout endpoint 安全和目标 provider 生命周期 |
| Publishing Channels | 2.5 | 配置和页面存在 | 真实渠道适配器和消息闭环不足 | 一个真实渠道的签名、重试和回执闭环 |
| Admin / Console | 3.0 | 页面和单体 API 较广 | 独立 Admin 服务偏薄，权限深度不足 | Admin service parity 和操作审计 |
| Observability | 2.5 | 本地能力存在 | 默认 no-op/内存路径，目标 ClickHouse join 未证明 | 目标 request-log、usage、billing 联查 |
| Deployment / Recovery | 3.5 | 本地 Compose/K8s/恢复通过 | 缺真实目标环境和外部 Secret 证明 | target evidence runner 正式接入 |
| Microservices / gRPC | 2.5 | 入口和清单存在 | 多个服务只有健康接口，功能不对等 | 逐服务 parity 证明和数据库所有权门禁 |

## 2. Identity、Tenant 与 Access

### 当前证据

- `src/server/internal/auth/service.go`
- `src/server/internal/auth/store.go`
- `src/server/internal/http/auth_middleware.go`
- `src/server/internal/http/security_middleware.go`
- `src/server/internal/tenant/**`
- `src/server/internal/http/tenant_handler.go`
- `src/web/src/features/auth/**`
- `src/web/src/routes/workspace/OnboardingPage.tsx`
- `src/web/src/routes/console/AccessPage.tsx`

已具备密码哈希、会话、会话轮换、CSRF、密码重置、组织和成员作用域。

### 上线缺口

- 将角色和权限从粗粒度 admin/member 判断升级为明确权限表；
- 对 Admin、Marketplace review、Billing inspection、API token 管理形成统一授权矩阵；
- 增加跨组织访问的路由级、服务级和 SQL 级统一证明；
- 统一独立服务和单体模式的身份传播语义。

### 商业增强

- MFA；
- OIDC/SAML；
- SCIM；
- 企业域名验证；
- Access Review 和 dormant account policy。

### 参考项目

- `open-webui`
- `LibreChat`
- `dify`
- `sub2api`

### 首个可验证切片

创建统一 `Permission` 常量、角色权限映射和中间件断言，先覆盖 Admin billing、API token、Marketplace review 三个高风险操作面。

## 3. API Gateway

### 当前证据

- `src/server/cmd/gateway/main.go`
- `src/server/internal/gateway/gateway.go`
- `src/server/internal/gateway/service.go`
- `src/server/internal/http/server.go`

独立 Gateway 当前仅注册 `/health`、`/healthz` 和 `NoRoute`，`HealthCheckTargets` 为空。完整外部路由权威仍主要位于聚合 server。

### 上线缺口

- 独立 Gateway 注册真实服务路由；
- 验证 Session、API token、组织身份和 request ID 向下游传播；
- 对目标服务执行健康检查、超时、熔断和失败返回；
- 建立单体和独立服务模式的路由一致性测试；
- 防止 Gateway 与 Relay 重复承担 Provider 路由和计费逻辑。

### 商业增强

- 动态服务发现；
- 灰度和 canary；
- 多区域入口；
- WAF 和高级滥用检测。

### 参考项目

- `ai-gateway`
- `gateway`
- `llmgateway`
- `Bifrost`

### 首个可验证切片

为 Chat、Relay、RAG、Agent、Workflow 注册显式 upstream target，加入身份头白名单和路由表测试。

## 4. Relay 与 Provider Runtime

### 当前证据

- `src/server/internal/relay/**`
- `src/server/internal/relay/handler/policy.go`
- `src/server/internal/relay/handler/chat.go`
- `src/server/internal/relay/handler/realtime.go`
- `src/server/internal/relay/handler/batch.go`
- `src/server/internal/relay/store.go`
- `src/server/internal/relay/pricing.go`
- `src/server/cmd/relay/main.go`

已有渠道选择、重试、usage、价格快照、配额、文件映射、Batch polling 结构、请求日志和部分 Realtime 生命周期。

### 上线缺口

- Realtime 的认证、预扣、断连结算和目标 request-log 证明；
- Batch 的真实 polling worker、Provider usage、失败退款和目标审计；
- Files delete/tombstone 的目标生命周期证明；
- Provider 价格来源、审批、freshness SLO 和跨系统 reconciliation；
- 真实 Provider SSE、取消、路由回退和 usage 证明。

### 商业增强

- 多区域路由；
- 语义缓存治理；
- 更多生命周期 API；
- Provider 质量评分和自动路由。

### 参考项目

- `new-api`
- `LiteLLM`
- `Bifrost`
- `one-api`
- `Helicone`

### 首个可验证切片

建立一个真实 Chat SSE target test：Provider 请求、首块响应、取消、usage、quota、request-log 和 billing 通过同一 `request_id` 联查。

## 5. Chat

### 当前证据

- `src/server/internal/chat/service.go`
- `src/server/internal/chat/gateway.go`
- `src/server/internal/chat/relay_gateway.go`
- `src/server/internal/http/routes_chat.go`
- `src/web/src/routes/workspace/ChatPage.tsx`

非流式主路径优先使用 Relay `CompletionUsage`，缺失时回退估算；生产模式已有 fail-closed 保护。

### 上线缺口

- 真实浏览器连接实际 Go 后端，而不是 Playwright API fixture；
- SSE 首块时延、断流、取消和重试；
- Chat message、Relay usage、request log、quota 和 billing 的 join；
- 明确禁止生产环境触发 demo fallback；
- 流式完成后的最终 usage 和异常补偿。

### 商业增强

- 多模态；
- 多模型比较；
- Artifacts；
- 桌面/PWA；
- 语音和跨设备同步。

### 参考项目

- `lobe-chat`
- `lobehub`
- `NextChat`
- `open-webui`
- `LibreChat`

### 首个可验证切片

新增启动真实 Go server 和测试 PostgreSQL 的 Playwright Chat E2E，不使用 `page.route` 拦截消息 API。

## 6. Knowledge / RAG

### 当前证据

- `src/server/internal/http/knowledge_handler.go`
- `src/server/internal/knowledge/ingestion_jobs.go`
- `src/server/internal/knowledge/ingestion_job_store.go`
- `src/server/internal/knowledge/index_jobs.go`
- `src/server/internal/knowledge/index_job_store.go`
- `src/server/internal/knowledge/qdrant_store.go`
- `src/server/internal/knowledge/service.go`

已有 durable ingestion/index jobs、租约、重试、死信、raw upload replay、Qdrant 生命周期和引用元数据。

### 已确认的核心缺口

仓库中存在 `NewIngestionWorker` 和 `NewIndexWorker`，但在 `src/server/cmd/**` 和聚合 server 中未搜索到生产装配。作业可能被持久化但无人消费。

### 上线缺口

- 在 server 和 rag service 中装配两个 worker；
- 配置 interval、batch size、lease、max attempts 和 worker ID；
- 共享生命周期 context，支持优雅关闭；
- readiness 暴露 worker 状态；
- 增加目标 Postgres/Qdrant drain、重试、dead-letter 和 stale-vector 证明。

### 商业增强

- OCR、DeepDoc、大文件和对象存储；
- 知识图谱；
- 检索质量评估；
- 多向量库迁移。

### 参考项目

- `ragflow`
- `MaxKB`
- `FastGPT`
- `dify`
- `anything-llm`

### 首个可验证切片

装配 IngestionWorker 与 IndexWorker，并新增生产启动测试：enqueue 后最终创建文档、写入向量、更新状态且关闭时停止 claim。

## 7. Agent / SOLO

### 当前证据

- `src/server/internal/agent/runner.go`
- `src/server/internal/agent/service.go`
- `src/server/internal/agent/executor.go`
- `src/server/internal/agent/store.go`
- `src/server/cmd/agent/main.go`
- `src/web/src/routes/workspace/AgentsPage.tsx`
- `src/web/src/routes/workspace/SoloPage.tsx`

Agent、Run、ToolRun、PlanStep、Memory、审批和预算均有持久化实现。

### 上线缺口

- 自定义 API 工具的 URL 当前直接进入 `http.NewRequestWithContext`；
- 未发现统一 DNS/IP/重定向级 SSRF 防护；
- structured streaming 仍有完成后按块模拟的路径；
- Sandbox target 部署、容量、取消和 artifact retention 未证明；
- Agent、Workflow、Task 的 execution identity 尚未完全统一。

### 商业增强

- 子 Agent；
- Team/Supervisor；
- 动态模型路由；
- 技能选择；
- restart resume；
- 人工接管。

### 参考项目

- `coze-studio`
- `LibreChat`
- `dify`
- `FastGPT`
- `open-webui`

### 首个可验证切片

建立共享 `outboundpolicy.Validator`，Agent custom API 在请求前解析、DNS 校验，并在每次重定向时重新校验目标地址。

## 8. Workflow

### 当前证据

- `src/server/internal/workflow/service.go`
- `src/server/internal/workflow/node_executor.go`
- `src/server/internal/workflow/executor/state_machine.go`
- `src/server/internal/workflow/sandbox/sandbox.go`
- `src/server/internal/http/routes_workflow.go`

已有持久执行、事件、trace、snapshot、replay、版本、触发器和状态机。

### 上线缺口

- HTTP node 只做 URL 解析，没有共享 DNS/IP/重定向 SSRF 防护；
- 目标 Postgres restart replay 和 retention prune 未证明；
- 所有 transition surface 未完全统一到 durable StateMachine sink；
- trigger listener、scheduled run 和 retry 的统一幂等证据不足；
- Docker sandbox 不是所有 code node 的默认生产权威路径。

### 商业增强

- 版本化节点适配器；
- 分支实验；
- 分布式 worker；
- checkpoint replay；
- Saga 和补偿。

### 参考项目

- `dify`
- `coze-studio`
- `Flowise`
- `FastGPT`
- `MaxKB`

### 首个可验证切片

Workflow HTTP node 复用统一 outbound policy，并增加重定向至 loopback、DNS rebinding 和私网 IPv6 的失败测试。

## 9. Task / Scheduler

### 当前证据

- `src/server/internal/task/runtime.go`
- `src/server/internal/task/service.go`
- `src/server/internal/schedule/**`
- `src/server/cmd/task/main.go`

Schedule 有 SQL claim、幂等和运行历史；Task runtime 只把当前步骤设为 completed、推进下一步骤并按比例更新预算。

### 已确认的核心缺口

Task `continueRuntimeTask` 不调用 Agent、Workflow 或真实工具。完成结果由步骤数量和预算字段合成，属于状态模拟器。

### 上线缺口

- 定义 `TargetExecutor` 接口；
- 支持 Agent 和 Workflow 两类 target；
- 保存下游 run/execution ID；
- 下游失败映射为 Task step failure；
- 支持取消和幂等重试；
- budget 从真实 usage 汇总，而不是按比例增加。

### 商业增强

- 分布式 scheduler；
- 依赖图；
- 资源预留；
- leader election；
- backlog SLO。

### 参考项目

- `coze-studio`
- `dify`
- `FastGPT`

### 首个可验证切片

新增 `TargetExecutor.Start`，先支持 Workflow target；Task start 后创建真实 workflow execution，并在 Task 中持久化 execution ID。

## 10. MCP / Tools

### 当前证据

- `src/server/internal/mcp/**`
- `src/server/internal/agent/executor.go`
- `src/web/src/features/mcp/**`
- `src/web/src/routes/workspace/McpServersPage.tsx`

内置工具、MCP、商业 enablement policy 和风险元数据较丰富。

### 上线缺口

- 网络类工具需要统一 outbound policy；
- MCP Server 凭证和 token rotation；
- MCP OAuth 和租户授权；
- Server 健康、超时和版本管理；
- 工具调用 cost、latency 和 error telemetry。

### 商业增强

- Tool/Skill Marketplace；
- Remote Tool Gateway；
- policy packs；
- 版本迁移。

### 参考项目

- `open-webui`
- `LibreChat`
- `dify`
- `FastGPT`
- `Bifrost`

### 首个可验证切片

将 MCP HTTP/SSE transport 接入共享 outbound policy，并记录 server ID、tool ID、request ID、latency 和结果大小。

## 11. Sandbox

### 当前证据

- `src/server/internal/workflow/sandbox/sandbox.go`
- `src/server/internal/agent/executor.go`
- `src/server/cmd/agent/main.go`

已有 network-none、只读 FS、资源限制、非 root 和 context metadata。

### 上线缺口

- 所有生产 Python/code tool 必须统一走 SandboxRunner；
- Sandbox 不可用时显式 fail closed；
- artifact 和日志持久化；
- queue、容量和并发治理；
- 目标 Docker/Kubernetes 隔离证据；
- cancellation、timeout、OOM 和 output truncation 的统一状态。

### 商业增强

- 多语言池；
- 依赖缓存；
- 网络白名单；
- malware scan；
- notebook session。

### 参考项目

- `FastGPT`
- `open-webui`
- `Flowise`
- `MaxKB`

### 首个可验证切片

统一 Agent 与 Workflow 的 SandboxRunner 返回类型，并将 artifact/log retention metadata 持久化到 tool run 或 node execution。

## 12. Billing / Quota / Payments

### 当前证据

- `src/server/internal/quota/service.go`
- `src/server/internal/stripe/**`
- `src/server/internal/payment/**`
- `src/server/internal/billing/**`
- `src/server/internal/http/billing_handler.go`

Stripe、quota 和 Relay price snapshot 主体存在。

### 上线缺口

- 独立 Billing 服务需要达到单体 billing handler 的功能对等；
- Alipay/WeChat Pay 目前主要是托管 checkout URL 和 webhook secret 配置；
- Provider checkout、webhook、refund、reconciliation 的统一 ledger；
- request ID 与 payment、usage、settlement 的联查；
- 定时 reconciliation 和异常告警。

### 商业增强

- chargeback；
- 税务和电子发票；
- margin analytics；
- Provider 价格同步。

### 参考项目

- `new-api`
- `sub2api`
- `LiteLLM`
- `CPA-Manager`

### 首个可验证切片

定义 PaymentProvider reconciliation contract，先实现 Stripe checkout/webhook/refund 的日终 reconciliation report。

## 13. Marketplace

### 当前证据

- `src/server/internal/marketplace/**`
- `src/server/internal/http/marketplace_handler.go`
- `src/server/internal/http/marketplace_payout_webhook_handler.go`
- `src/server/cmd/marketplace/main.go`

发布、审核、安装、订单、结算、payout outbox、申诉和举报主体存在。

### 上线缺口

- 独立 Marketplace 服务当前主要是健康接口；
- payout endpoint 只做 URL 语法和 HTTPS 约束，没有统一 DNS/IP/重定向 SSRF 防护；
- 真实 provider dispatch、webhook、retry 和 reconciliation 目标证据；
- refund/chargeback 对 publisher settlement 的一致性；
- 人工补单和 operator remediation。

### 商业增强

- 推荐和排名；
- 风险评分；
- reviewer SLA；
- creator analytics；
- 分成层级。

### 参考项目

- `coze-studio`
- `dify`
- `lobe-chat`
- `Flowise`
- `FastGPT`

### 首个可验证切片

payout provider 复用 outbound policy，并增加 provider idempotency key、dispatch attempt 和 webhook reconciliation 测试。

## 14. Multi-Channel Publishing

### 当前证据

- `src/server/internal/channel/**`
- `src/server/cmd/channel/main.go`
- `src/web/src/routes/workspace/PublishingChannelsPage.tsx`

存在渠道配置、日志和 UI，但独立 Channel 服务主要是健康接口。

### 上线缺口

- 至少一个真实渠道适配器；
- outbound signature、inbound webhook verification；
- delivery receipt、retry 和 idempotency；
- Chat/Agent/Workflow 与渠道消息 identity 关联；
- credential encryption 和 rotation。

### 商业增强

- 多渠道适配器；
- 双向会话；
- 媒体转换；
- 渠道故障切换。

### 参考项目

- `coze-studio`
- `dify`
- 各渠道官方 SDK

### 首个可验证切片

选择 Slack 或 Feishu，完成发送、签名 webhook、回执、重试和重复事件拒绝的单渠道闭环。

## 15. Admin / Console

### 当前证据

- `src/server/internal/admin/**`
- `src/server/internal/http/admin_handler.go`
- `src/server/cmd/admin/main.go`
- `src/web/src/routes/admin/**`
- `src/web/src/routes/console/**`

单体 Admin API 和前端页面较广，独立 Admin 服务的功能需要继续与单体对齐。

### 上线缺口

- 独立 Admin 服务 parity；
- 统一权限矩阵；
- 每个写操作的审计、旧值/新值和 request ID；
- Provider、价格、Billing、Marketplace 和 Observability 的真实操作状态；
- support 操作必须可追踪且不可静默越权。

### 商业增强

- 批量操作；
- tenant risk；
- 成本利润；
- impersonation；
- reconciliation control center。

### 参考项目

- `new-api`
- `CPA-Manager`
- `llmgateway`
- `open-webui`
- `Cli-Proxy-API-Management-Center`

### 首个可验证切片

为 Admin pricing、channel 和 payout remediation 建立统一 mutation audit envelope。

## 16. Observability / Audit

### 当前证据

- `src/server/internal/observability/**`
- `src/server/internal/metrics/**`
- `src/server/internal/http/middleware.go`
- `src/server/internal/http/observability_alert_handler.go`
- `src/server/cmd/observability/main.go`

单体生产配置可 fail closed 到 ClickHouse，但独立 Observability 使用内存 Reporter，`/subscribe` 仅返回 ready。

### 上线缺口

- 独立 Observability 持久化；
- request log、usage、billing、route decision 的目标 ClickHouse join；
- alert delivery 和 recovery audit 目标证据；
- trace backend；
- retention 和访问权限；
- 去重、抑制和升级。

### 商业增强

- anomaly detection；
- Provider quality；
- multi-region SLO；
- 自动但可审核的 remediation。

### 参考项目

- `Helicone`
- `Bifrost`
- `LiteLLM`
- `CPA-Manager`

### 首个可验证切片

用真实 ClickHouse 启动独立 Observability service，写入 request log 后通过 request ID 联查 Relay usage 和 billing snapshot。

## 17. Deployment / Security / Recovery

### 当前证据

- `docker-compose.yml`
- `Dockerfile.server`
- `Dockerfile.web`
- `deploy/kubernetes/**`
- `scripts/deploy-validate.sh`
- `scripts/k8s-validate.sh`
- `scripts/backup-restore-smoke.sh`
- `scripts/verify-commercial-completion.sh`

近期本地日志证明 Compose、Kubernetes、112 个迁移和备份恢复通过。

### 上线缺口

- 外部目标 DB；
- 仓库外 Kubernetes Secret；
- target evidence manifest；
- artifact body directory；
- 真实 Provider、支付、payout、ClickHouse、RAG、gRPC 和微服务 DB 证据；
- 最终 no-skip verifier。

### 商业增强

- 多区域；
- HA；
- artifact signing；
- Secret rotation；
- penetration testing；
- load/soak/DR drills。

### 参考项目

- `ai-gateway`
- `Helicone`
- `ragflow`
- `dify`
- `Bifrost`

### 首个可验证切片

将当前未跟踪 `scripts/run-target-release-evidence.sh` 和 fixture 纳入正式门禁，完成外部 workdir 的 prepare-only 流程。

## 18. Microservices / gRPC / Events

### 当前证据

- `src/server/cmd/{gateway,relay,chat,workflow,rag,agent,billing,marketplace,admin,channel,task,observability}`
- `api/proto/**`
- `src/server/internal/grpc/**`
- `src/server/pkg/**/grpc_server.go`
- `src/server/migrations/microservices/**`
- `deploy/kubernetes/*-deployment.yaml`

12 个业务入口和 K8s 清单存在，但多个入口仍只有健康接口或薄层。

### 上线缺口

- 每个服务与单体实现的功能 parity；
- proto 唯一来源和生成物治理；
- migration prefix、checksum 和 ownership；
- 禁止跨服务数据库访问；
- gRPC timeout、retry、auth、tenant 和 trace metadata；
- Kafka/outbox 事件幂等和补偿；
- target gRPC smoke 和 database-per-service 证明。

### 商业增强

- Saga；
- 多区域事件复制；
- 服务级 autoscaling；
- chaos 和故障隔离验证。

### 参考项目

- `coze-studio`
- `dify`
- `ragflow`
- `ai-gateway`

### 首个可验证切片

选择 RAG 服务完成 parity pilot：独立 DB、gRPC API、worker、K8s 部署、target smoke，并禁止聚合 server 直接访问 RAG 表。

## 19. 测试与证据横向缺口

### 已有

- 264 个 Go 测试文件；
- 前端 Vitest 和 25 个 Playwright spec；
- DB evidence profiles；
- Compose、Kubernetes、backup/restore smoke；
- target evidence assembler、collector、digest 和 verifier。

### 缺口

- 25 个 Playwright spec 均使用 API fixture；
- `src/server/test/integration` 没有进入常规门禁；
- Kafka 和 Agent E2E 存在显式 skip；
- 没有全局 no-skip；
- 没有覆盖率门槛；
- 没有 `go test -race`；
- 没有稳定的 `go vet`、Staticcheck、golangci-lint、ESLint 门禁；
- 没有 fuzz、benchmark、load、soak；
- 浏览器只有 Chromium。

### 首个可验证切片

建立 `scripts/verify-real-fullstack-e2e.sh`：

1. 启动 PostgreSQL、Qdrant、ClickHouse、Redis 和 Go server；
2. 运行迁移；
3. 启动真实 Web；
4. 运行不使用 `page.route` 的 Chat、RAG、Agent、Workflow、Billing 五条 Playwright 流程；
5. 失败时保留服务日志和 trace ID。

## 20. 子计划拆分

完整规格不能由一个巨型计划执行。后续应建立以下独立计划：

| 顺序 | 子计划 | 依赖 | 完成产物 |
|---:|---|---|---|
| 1 | Shared Outbound Security Policy | 无 | Agent、Workflow、MCP、Payout 共享 SSRF 防护 |
| 2 | RAG Worker Production Wiring | 数据库和 Qdrant | Worker 装配、readiness、drain 和恢复证明 |
| 3 | Task Real Target Execution | Workflow 或 Agent service | Task 创建真实 execution 并汇总 usage |
| 4 | Real Full-Stack Browser E2E | 稳定本地运行栈 | 无 API fixture 的五条商业旅程 |
| 5 | Relay and Chat Live Evidence | Provider 和 ClickHouse | SSE、取消、usage、quota、billing、request log join |
| 6 | Payment and Marketplace Reconciliation | 支付 Provider | checkout、webhook、refund、payout 和 reconciliation |
| 7 | Observability Persistence and SLO | ClickHouse 和 alert sink | target request-log、join、delivery、recovery 证明 |
| 8 | Microservice Parity Pilot: RAG | RAG worker | 独立 RAG DB、gRPC、K8s 和 target smoke |
| 9 | Gateway Service Routing | 独立服务可运行 | 真实 upstream route、auth 和 health-aware dispatch |
| 10 | Target Release Evidence Runner | 外部目标环境 | prepare-only、artifact collection、final no-skip run |

## 21. 推荐执行顺序

### 第一批：本地可完成且阻塞安全或真实功能

1. Shared Outbound Security Policy；
2. RAG Worker Production Wiring；
3. Task Real Target Execution；
4. Real Full-Stack Browser E2E。

### 第二批：需要真实外部系统

5. Relay and Chat Live Evidence；
6. Payment and Marketplace Reconciliation；
7. Observability Persistence and SLO。

### 第三批：架构收口

8. RAG Microservice Parity Pilot；
9. Gateway Service Routing；
10. 全服务 parity 和 Target Release Evidence Runner。

## 22. 全项目完成标准

仅当以下条件全部满足时，才能把状态提升为完整商业发布：

- 上线基线功能都有生产运行代码；
- 不支持能力明确 fail closed；
- Task、RAG worker、Agent、Workflow 不再依赖模拟或无人消费队列；
- 所有外部 URL 通过统一 outbound policy；
- 浏览器 E2E 使用真实后端和数据库；
- integration tests 进入门禁且无关键 skip；
- 单体和独立服务模式功能对等；
- target DB、Kubernetes、Provider、支付、payout、ClickHouse、RAG、gRPC 和微服务数据库证据齐全；
- target artifact body 和 digest 校验通过；
- 最终 `scripts/verify-commercial-completion.sh` 无环境跳过并成功。
