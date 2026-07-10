# Oblivious 完整仓库扫描总报告

日期：2026-07-10

## 1. 扫描范围与方法

本报告覆盖根仓库主线：

- `src/server`：单体运行时、独立服务入口、领域服务、存储、迁移和测试；
- `src/web`：用户工作台、管理控制台、API client、状态管理和测试；
- `api/proto`：服务契约；
- `deploy`、`.github/workflows`、`scripts`：构建、部署、恢复、验证和发布证据；
- `docs/audit`、`docs/release`、`docs/superpowers/specs`：设计目标、差距和商业完成边界；
- `reference`：31 个参考项目，仅用于能力和交互模式对照，不作为 Oblivious 实现证据。

扫描采用四条并行只读审计线：

1. 核心平台：Gateway、Relay、Billing、Auth/Tenant、Observability；
2. 用户产品与编排：Chat、Agent、Workflow、Task、Channel、Admin、Web；
3. 知识与生态：Knowledge/RAG、Marketplace、Provider、Payment、Payout、File；
4. 工程与发布：CI、测试、Docker、Kubernetes、迁移、安全、恢复和目标发布证据。

所有结论区分四种证据：生产代码存在、仓库本地验证、目标环境验证、最终无跳过商业发布验证。

## 2. 项目的完整目标

Oblivious 的目标不是单一聊天应用，而是一个面向工作区的 multi-tenant AI SaaS 平台：

- 用 Chat、Knowledge、Agent、Workflow、Task 和多渠道发布形成客户工作台；
- 用 Relay 统一所有可计费 AI 调用的 Provider 路由、限流、配额、计价、usage、request log 和审计；
- 用 Billing、Marketplace、Payment、Payout 和 Admin 形成商业运营与资金闭环；
- 用 PostgreSQL、Qdrant、ClickHouse、Redis、Kafka、对象存储和后台 worker 形成耐久执行与数据闭环；
- 同时支持可发布的单体/双模基线，以及能力对等的独立微服务目标；
- 最终必须在真实目标环境完成 Provider、支付、Payout、ClickHouse、Kubernetes、恢复和 no-skip verifier 证明。

全局不可破坏的约束：

- 所有 billable AI operation 必须经过 Relay；
- 所有客户状态必须 tenant scoped；
- request、execution、usage、billing、audit 和 trace identity 必须可联查；
- 未完整实现或未证明的能力必须 fail closed；
- fixture、mock 和本地证据不能替代目标环境和最终发布证据。

## 3. 当前总体结论

### 3.1 综合判断

- **产品与领域代码成熟度：约 55%-60%。** 大部分核心模块已有真实 SQL、HTTP、业务状态机和前端页面，不是空壳。
- **仓库本地工程化成熟度：约 65%-70%。** 测试、迁移、OpenAPI、发布验证器、Docker/Kubernetes 静态资产和运行手册较丰富。
- **独立微服务能力对等度：约 35%-40%。** Relay 较深，但 Gateway、Billing、RAG、Marketplace、Observability 等独立入口明显弱于单体实现或仅提供 health/stub。
- **真实商业闭环证明度：约 20%-30%。** 真实 Provider、支付、Payout、ClickHouse、对象存储、集群恢复和 no-skip target evidence 尚未形成当前证明。
- **当前状态：不可声明完整商业发布。** 这是一个中后期产品化仓库，不是 MVP 空架子，但仍有多个 P0 运行和装配断点。

### 3.2 本次实际验证基线

- `bash scripts/check.sh docs`：通过；覆盖发布资产、商业 preflight/target evidence fixtures、OpenAPI、迁移、schema、Kubernetes 恢复策略、部署契约、工作流成功率证据和文档一致性。
- `pnpm test`：68 个测试文件、640 个测试全部通过。
- `go test ./...`：除新建但未跟踪的 `src/server/internal/outboundpolicy` 外，其余包通过或无测试；唯一失败为跨域重定向测试依赖真实 DNS，当前环境把 `example.com` 解析到保留网段 `198.18.0.15`，先触发 `ErrBlocked`。
- Go 仓库包含 103 个 package、266 个 `_test.go` 文件；Web 包含 67 个源码测试文件，Vitest 实际收集 68 个测试文件。
- 工作树不是 release-clean：`scripts/run-target-release-evidence*.sh` 和 `src/server/internal/outboundpolicy/` 未跟踪。
- 两个 target release runner shell 脚本通过 `bash -n`，但尚未纳入版本控制、正式门禁和目标环境运行。

## 4. 模块能力、参考项目和完整度

评分衡量生产实现、装配、测试和商业闭环，不按代码量评分。5/5 仅表示目标环境和最终商业门禁已证明；当前没有模块达到 5/5。

| 模块 | 完整版本必须具备 | 当前进展与主要断点 | 参考项目 | 综合完整度 |
|---|---|---|---|---:|
| Identity / Tenant / Access | 注册登录、密码重置、组织/成员/邀请、资源权限、session 安全、MFA、OIDC/SAML、SCIM、审计 | 基础注册、SQL session、组织成员和 CSRF 已有；缺生产密码投递、细粒度 RBAC/ABAC、企业身份、可信代理和设备/session 管理 | `open-webui`、`LibreChat`、`dify`、`sub2api` | 3.0/5 |
| API Gateway | 服务发现、真实代理、可信身份传播、限流、熔断、流式/WebSocket、动态路由、mTLS/WAF | 中间件、Redis 限流和熔断存在；独立 Gateway 没有生产 `RegisterRoute`，健康检查目标为空，非健康请求返回 404 | `ai-gateway`、`gateway`、`llm-gateway`、`bifrost` | 1.5/5 |
| Relay / Provider Runtime | 多 Provider 协议、路由、重试、健康、定价、quota、usage、request log、Realtime/Batch/File 生命周期 | 当前最深模块；Chat/Responses/Embedding/Image/Audio 等主链真实，Batch worker 已有；Realtime/Batch/File 仍缺目标生命周期、对象存储、断连结算和真实对账证明 | `new-api`、`litellm`、`bifrost`、`helicone`、`one-api` | 3.5/5 |
| Chat | 多轮、分支、模型/参数、附件、多模态、知识绑定、分享、真实 SSE、精确 usage、失败恢复 | CRUD、分享、导出、分支、Relay 调用真实；流式 usage 仍估算，失败可能留下单边消息，缺真实浏览器到 Provider 的无 mock E2E | `lobe-chat`、`lobehub`、`LibreChat`、`open-webui`、`NextChat` | 3.0/5 |
| Knowledge / RAG | 对象存储、解析/OCR、异步 job、重试/DLQ、chunk/embed/index、混合检索、引用、评测、版本和删除传播 | Parser、SQL job、lease、dead-letter、Qdrant 路径较深；生产入口未启动 ingestion/index worker，raw bytes 存 PostgreSQL，独立 RAG gRPC 返回合成数据 | `ragflow`、`FastGPT`、`MaxKB`、`dify`、`anything-llm` | 2.5/5 |
| Agent / SOLO | Agent version、run/tool/plan/memory、审批、预算、真实 structured streaming、sandbox、restart resume、人工接管 | SQL Run/Tool/Plan/Memory 和审批较深；Custom API 无统一 SSRF 策略，最终输出是完成后按词回放，sandbox 与 restart resume 缺目标证明 | `coze-studio`、`LibreChat`、`dify`、`FastGPT` | 3.0/5 |
| Workflow | CRUD/version、typed node、触发器、持久执行、retry/pause/resume、trace/snapshot/replay、分布式 worker、Saga | 版本、分支、持久事件、trace/snapshot/replay 已有；HTTP node 未接统一 outbound policy，目标数据库重启恢复和分布式 lease 未证明 | `dify`、`coze-studio`、`Flowise`、`FastGPT` | 3.0/5 |
| Task / Scheduler | 真实调用 Agent/Workflow、cron、claim、幂等、预算、审批、run history、重试、取消和恢复 | Scheduled Task 能启动 Agent/Workflow；SOLO Task runtime 只推进步骤和预算，属于状态模拟，两套模型未统一 | `coze-studio`、`dify`、`FastGPT` | 1.5/5 |
| MCP / Tool Platform | server/tool registry、schema、凭证/OAuth、授权、审计、超时、网络策略、生命周期和使用计费 | 内置和远程工具面较广、测试较多；远程授权、统一 secret、outbound policy、生产生命周期和治理仍不足 | `open-webui`、`LibreChat`、`dify`、`Flowise`、`bifrost` | 3.5/5 |
| Sandbox / Custom Execution | 强制隔离、资源限制、取消、网络策略、artifact/log、retention、审计、容量管理 | Docker runner 路径存在并传递执行上下文；未成为所有代码执行的生产权威路径，目标部署、artifact/log 持久化和容量证明不足 | `FastGPT`、`open-webui`、`Flowise` | 2.5/5 |
| Billing / Quota / Payments | usage、预留/结算、不可变 ledger、subscription、invoice、refund、dispute、税务、provider reconciliation | 单体 quota、Stripe lifecycle 和事务更新较深；独立 Billing 只暴露 health，缺双重记账、完整对账、国内支付真实接入和争议处理 | `new-api`、`sub2api`、`litellm`、`CPA-Manager` | 3.0/5 |
| Marketplace | 发布、审核、版本、安装、entitlement、订单、退款、治理、publisher analytics、结算和 payout | 单体发布/审核/安装/订单/settlement 存在；独立 Marketplace 仅 health，商品/许可证/税务不足，真实 payout 和 reconciliation 未闭环 | `coze-studio`、`dify`、`lobe-chat`、`Flowise` | 2.5/5 |
| Multi-Channel Publishing | OAuth/secret、签名 webhook、发送/回执、重试、DLQ、限流、审计、渠道配额 | 多个 adapter、发送、失败重试和归档路径存在；独立 Channel 能力不对等，未证明任何真实平台收发、回执、限流和密钥轮换闭环 | `coze-studio`、`dify`、官方渠道 SDK | 2.5/5 |
| Admin / Console | 用户、模型、路由、渠道、套餐、账单、usage、审计、Marketplace、告警、细粒度权限和双人审批 | 单体 Admin SQL/API/UI 较广；独立 Admin 装配偏薄，权限粗粒度，高风险操作、不可篡改审计和真实跨系统对账不足 | `new-api`、`CPA-Manager`、`open-webui`、`sub2api` | 3.0/5 |
| Observability / Audit | 标准 metrics、trace、ClickHouse request log、SLO/error budget、alert delivery、on-call、审计保留和恢复自动化 | 单体 request-log/alert/recovery 组件存在；独立服务使用内存 reporter，`/metrics` 非 Prometheus，consumer 未启动，recovery 为 audit-only | `helicone`、`bifrost`、`litellm`、`CPA-Manager` | 2.5/5 |
| Deployment / Security / Recovery | 可复现构建、非 root、镜像签名/SBOM、K8s、Secret、迁移、备份、DR、PDB、目标 smoke | 静态部署资产、PostgreSQL 恢复和 verifier 较强；缺镜像供应链、非 PostgreSQL 状态恢复、真实集群、目标 Secret 和 no-skip 结果 | `ai-gateway`、`helicone`、`ragflow`、`dify` | 3.0/5 |
| Microservices / gRPC / Events | 单体/独立服务 parity、服务数据库所有权、完整 proto、auth/tenant/trace metadata、outbox/idempotency、target smoke | Relay 独立服务最接近可用；Gateway、Billing、RAG、Marketplace、Observability 多为壳层/stub，proto 与 HTTP 能力不对称 | `coze-studio`、`dify`、`ragflow`、`ai-gateway` | 2.0/5 |

## 5. 已确认的 P0 阻塞

1. **统一 Outbound Security Policy 未完成。** 新包未跟踪、未接入 Agent/Workflow/MCP/Payout，且单测依赖真实 DNS；现有用户可控 URL 仍存在 SSRF 和 redirect 风险。
2. **Knowledge 后台 worker 未生产装配。** Ingestion/Index job 能入队但没有启动点，上传可能永久停留 `pending`。
3. **SOLO Task 不执行真实目标。** 当前 runtime 仅修改步骤状态和预算，不能算真实 Agent/Workflow 执行。
4. **Gateway 独立服务没有真实路由。** 部署存在，但服务本身不能承担入口代理职责。
5. **Marketplace payout 启动链断裂。** Provider 未注入 `RouterOptions`，webhook secret 为空，缺 dispatcher、重试、人工重放和逐笔 reconciliation。
6. **独立 RAG/Marketplace/Billing/Observability 与单体不对等。** 当前 Kubernetes 微服务形态会暴露 health/stub 服务，不能视为生产拆分完成。
7. **文件存储不适合多副本。** Knowledge raw bytes 进入 PostgreSQL，Relay Files 使用本地目录，缺共享对象存储、AV、checksum、retention 和孤儿清理。
8. **真实 full-stack E2E 缺失。** 前端 unit tests 通过，但大量路由测试依赖 API mock，Playwright 未证明真实 Go + PostgreSQL + Provider 链路。
9. **目标环境证据缺失。** 没有当前真实 Provider、支付、Payout、ClickHouse、Kubernetes、备份恢复和 no-skip commercial verifier 结果。
10. **发布供应链不完整。** 缺正式镜像 build/push/sign、SBOM/provenance、Action SHA 固定和全状态面 DR 演练。

## 6. 推荐执行顺序

### 第一阶段：修复本地真实功能和安全断点

1. 完成并接入 Shared Outbound Security Policy，修复确定性测试；
2. 装配 RAG ingestion/index workers，加入 readiness、积压、失败和 drain 指标；
3. 将 SOLO Task 改为真实 Agent/Workflow target execution；
4. 接通 Marketplace payout provider、webhook secret、dispatcher 和 reconciliation；
5. 为独立 Gateway 注册真实 upstream route、可信身份传播和健康路由。

### 第二阶段：证明真实端到端链路

6. 建立不使用 API fixture 的 Chat、RAG、Agent、Workflow、Billing 浏览器 E2E；
7. 将 Knowledge/Relay Files 迁移到共享对象存储并增加安全扫描；
8. 补 Relay/Chat 的 streaming cancellation、usage、quota、billing 和 request-log join；
9. 补 ClickHouse ingest/query、alert delivery 和 SLO recovery 目标证明。

### 第三阶段：微服务与商业收口

10. 以 RAG 为 parity pilot，完成独立 DB、gRPC、worker、K8s 和 target smoke；
11. 逐步补齐 Billing、Marketplace、Observability、Gateway 的单体/独立服务能力对等；
12. 将 target release runner 纳入版本控制和正式 CI，生成真实 artifact body、digest 和 no-skip 最终结果。

## 7. 完整完成标准

只有以下条件全部满足，项目才能称为“功能完整且可商业发布”：

- 17 个模块的上线基线均有生产装配和运行代码；
- Task、RAG worker、Agent、Workflow 不依赖模拟执行或无人消费队列；
- 所有用户可控出站 URL 使用统一 fail-closed policy；
- 单体与宣称部署的独立服务模式能力对等；
- 关键浏览器旅程使用真实后端、数据库和外部服务；
- integration tests 进入常规门禁，关键路径无 skip，并增加 race、lint、security 和 load 基线；
- Provider、支付、Payout、ClickHouse、Qdrant、对象存储、Kafka、Kubernetes 和恢复均有目标环境证据；
- request、execution、usage、billing、payment、settlement、audit 和 trace 能够端到端联查；
- 发布制品可复现、不可变、已签名并附带 SBOM/provenance；
- 最终 `scripts/verify-commercial-completion.sh` 在目标环境无跳过通过，工作树和发布 commit 一致。

## 8. 关联文档

- 完整模块功能定义：`docs/superpowers/specs/2026-07-10-module-capability-reference-design.md`
- 逐模块详细差距与首个切片：`docs/audit/2026-07-10-module-capability-gap-matrix.md`
- 生产代码深度基线：`docs/audit/current-implementation-depth.md`
- 商业完成证据边界：`docs/release/commercial-completion-audit.md`
- 当前发布门禁：`docs/release/commercial-gates.md`
