# Requirements: Oblivious Commercial Complete & Target Release

**Defined:** 2026-07-14
**Core Value:** 让组织客户能够可靠地构建、运行并商业化 AI 应用，同时让每一次 AI 操作都可隔离、可计费、可追踪、可审计、可恢复。

## Evidence Model

所有 v1 需求从未完成状态开始。现有代码和历史 `[x]` 只能作为实现基线，不能继承为当前商业完成证明。

- **E1 - Fixture/unit：** 隔离的单元、契约、schema、fixture 或 mock 行为。
- **E2 - Repository runtime：** 真实本地产品代码、数据库、worker 和无 fixture 的全栈行为。
- **E3 - Target runtime：** 真实目标基础设施、Provider、支付/payout rail、存储、可观测性、集群和恢复证据。
- **E4 - Commercial release：** 同一 commit、同一 digest、无 skip strict verifier 和不可变外部证据。

低等级证据不能关闭要求更高等级证据的需求。route、page、schema、proto、health endpoint、测试数量或文档存在，均不能单独证明客户旅程完成。

## v1 Requirements

商业完整版本的承诺范围。每条需求必须且只能映射到一个 Roadmap 阶段。

### Identity And Tenancy

- [ ] **IDEN-01**: 用户可以注册账户，失败注册不会留下部分可用的身份。
- [ ] **IDEN-02**: 用户可以登录和退出；会话跨浏览器刷新保持，退出后立即失效。
- [ ] **IDEN-03**: 用户可以通过生产可用的恢复渠道重置凭据，旧恢复令牌随后失效。
- [ ] **IDEN-04**: 用户可以创建组织并成为其 Owner。
- [ ] **IDEN-05**: 受邀用户可以加入组织，过期、撤销或跨组织邀请会被拒绝。
- [ ] **IDEN-06**: 多组织用户可以切换活动组织，后续操作只作用于所选组织。
- [ ] **IDEN-07**: Owner 可以增删成员和修改角色，越权角色变更会被拒绝。
- [ ] **IDEN-08**: 可信 actor 和 organization identity 一致贯穿 HTTP、gRPC、service、job、retry、vector 和 analytics。
- [ ] **IDEN-09**: 对 SQL、向量、对象、队列、Admin 和分析数据的跨租户访问均被拒绝，且不泄露目标对象是否存在。

### Relay Authority

- [ ] **RLAY-01**: Chat、Knowledge、Agent、Workflow、MCP 和受支持 `/v1/*` 的所有可计费 AI 调用只能经 Relay 到达 Provider。
- [ ] **RLAY-02**: Relay 只把请求路由到满足所需 endpoint、streaming、tool、usage 和 lifecycle 能力的健康渠道。
- [ ] **RLAY-03**: 缺少可信身份、价格、quota 或所需 Provider 能力时，Relay 在发起上游调用前明确失败。
- [ ] **RLAY-04**: 受支持的流式调用以稳定事件顺序把真实 Provider 增量传给客户端。
- [ ] **RLAY-05**: 客户端取消会传播到 Provider，并留下可查询的取消终态。
- [ ] **RLAY-06**: retry 和 fallback 只按已配置策略执行，每次尝试可审计且不产生重复输出。
- [ ] **RLAY-07**: 成功、失败、取消和断流调用均记录一次权威 usage，或明确标记为 partial/pending reconciliation。
- [ ] **RLAY-08**: 每次调用使用不可变价格快照预留 quota，并且只结算或退款一次。
- [ ] **RLAY-09**: 同一调用可按 request、trace、upstream request、route decision、usage、billing、request log 和 audit identity 联查。
- [ ] **RLAY-10**: 当前受支持 `/v1/*` 契约保持兼容；商业生命周期不完整的 API 默认禁用并 fail closed。

### Chat

- [ ] **CHAT-01**: 用户可以创建会话并选择当前可用的模型和受支持参数。
- [ ] **CHAT-02**: 用户可以为会话绑定或移除活动组织拥有的 Knowledge。
- [ ] **CHAT-03**: 用户可以收到真实流式响应，用户消息和已生成内容均被持久化。
- [ ] **CHAT-04**: 用户可以取消或重试响应，刷新页面不会产生重复消息或虚假完成状态。
- [ ] **CHAT-05**: 用户可以查看引用、quota 错误和可操作的 Provider 失败信息。
- [ ] **CHAT-06**: 用户可以把会话转换为 SOLO 或 Task，并保留 organization、conversation 和 request identity。

### Knowledge And RAG

- [ ] **KNOW-01**: 用户可以在活动组织内创建、更新和删除知识库。
- [ ] **KNOW-02**: 用户可以上传受支持文档，并看到校验、共享对象持久化进度和失败原因。
- [ ] **KNOW-03**: ingestion/index job 具有持久状态、lease、retry、dead letter、可审计 replay 和重启恢复。
- [ ] **KNOW-04**: 文档 embedding 只能经 Relay 生成，并写入当前声明支持的向量后端。
- [ ] **KNOW-05**: 检索只返回当前组织拥有的有效版本，并提供可定位引用。
- [ ] **KNOW-06**: 文档更新或删除会重建或移除相应向量，旧版本和 stale vector 不再参与正常检索。
- [ ] **KNOW-07**: Builder 或运营者可以运行检索检查，并查看 source、version、score、quality 和失败诊断。

### Automation

- [ ] **AUTO-01**: Builder 可以创建和版本化 Agent 的模型、Knowledge、工具、预算和执行策略。
- [ ] **AUTO-02**: Agent run 持久记录 plan、tool call、memory、output、status 和实际 usage。
- [ ] **AUTO-03**: 用户可以通过稳定的 run/tool identity 查看 Agent 结构化实时事件。
- [ ] **AUTO-04**: 获授权用户可以审批、拒绝、重试或取消 Agent 操作。
- [ ] **AUTO-05**: Agent 执行在重启后可以继续或被安全回收，且不重复已提交的外部副作用。
- [ ] **AUTO-06**: Builder 可以创建、验证和版本化 Workflow graph。
- [ ] **AUTO-07**: Workflow execution 持久记录每个节点的状态、变量、输入、输出和错误。
- [ ] **AUTO-08**: 用户可以从失败点 retry/replay Workflow，并查看相应 debug trace。
- [ ] **AUTO-09**: 用户可以创建、暂停、恢复和取消 Scheduled Task。
- [ ] **AUTO-10**: Task 必须启动真实 Agent 或 Workflow execution，并展示下游 status、failure 和 usage。
- [ ] **AUTO-11**: Builder 可以注册 MCP server 或 tool，配置 credential、risk 和 approval policy，并查看调用审计。

### Shared Security

- [ ] **SECU-01**: Provider URL、MCP、tool、Workflow HTTP node、Webhook、Channel 和 payout 共用同一 fail-closed outbound policy，覆盖 DNS、IP、redirect 和 rebinding。
- [ ] **SECU-02**: 外部 credential 加密保存、可轮换和撤销，并且不出现在日志、API 响应或证据包中。
- [ ] **SECU-03**: 入站 webhook 在改变状态前验证签名、时间窗口和 replay/idempotency identity。
- [ ] **SECU-04**: 代码只能在声明的隔离 Sandbox 中执行；隔离或容量不可用时不得回退到 host process。
- [ ] **SECU-05**: 高风险 tool、outbound 和 payout 操作必须经过授权审批并生成不可抵赖审计。

### Shared Storage

- [ ] **STOR-01**: 文档原件、Relay Files 和自动化 artifact 使用 tenant-scoped 共享对象存储，不以本地磁盘或数据库 raw blob 作为生产权威。
- [ ] **STOR-02**: 对象生命周期覆盖 checksum、恶意文件扫描、retention、删除传播、孤儿清理、备份和恢复证明。

### Billing And Payments

- [ ] **BILL-01**: 客户可以查看当前套餐、价格、quota、已用量和剩余额度。
- [ ] **BILL-02**: 客户可以通过至少一个已声明的真实支付渠道订阅，签名确认前不授予 entitlement。
- [ ] **BILL-03**: 客户可以充值；重复 webhook 或重试不会重复增加余额。
- [ ] **BILL-04**: 每条 Relay 权威 usage 只产生一条可追踪的不可变 ledger 影响。
- [ ] **BILL-05**: quota 在 Provider 调用前预留；额度不足时客户收到明确错误且不会产生上游费用。
- [ ] **BILL-06**: 客户可以查看 invoice、payment、credit 和 usage history。
- [ ] **BILL-07**: refund 可幂等执行，并正确更新 ledger、quota、invoice 和 entitlement。
- [ ] **BILL-08**: dispute/chargeback 具有明确状态并产生可追踪的补偿性财务影响。
- [ ] **BILL-09**: 定时 reconciliation 能发现 Provider 与内部 ledger 差异，并把异常交给运营工作流处理。

### Marketplace

- [ ] **MRKT-01**: Publisher 可以创建和版本化 Marketplace listing 草稿。
- [ ] **MRKT-02**: Publisher 可以提交审核并查看 review decision 和修改意见。
- [ ] **MRKT-03**: 仅审核通过的版本可以发布；Publisher 可以下架自己的已发布版本。
- [ ] **MRKT-04**: Buyer 可以浏览、搜索并检查 listing 的版本、价格、权限和依赖。
- [ ] **MRKT-05**: Buyer 可以安装免费 listing，并获得 organization-scoped entitlement。
- [ ] **MRKT-06**: Buyer 可以购买付费 listing，支付确认后才获得 entitlement。
- [ ] **MRKT-07**: refund 或 chargeback 会正确影响 Buyer entitlement 和 Publisher revenue。
- [ ] **MRKT-08**: settlement 可以追溯到 order、payment、refund、fee 和 Publisher。
- [ ] **MRKT-09**: payout 对外幂等派发，失败可重试并可与 Provider 结果对账。
- [ ] **MRKT-10**: Publisher 可以申诉；运营者可以处理 abuse、takedown 和 restore，所有决定均留有完整审计。

### Channels

当前项目保留多渠道发布承诺，因此 v1 必须至少声明一个真实渠道并满足以下全部需求。否则必须在发布前移除相应承诺、默认配置和 UI 入口。

- [ ] **CHAN-01**: Release manifest 明确列出全部受支持渠道，未声明渠道不会显示为可用。
- [ ] **CHAN-02**: Admin 可以配置渠道 credential 并查看真实 readiness。
- [ ] **CHAN-03**: 渠道入站消息验证签名、去重，并映射到正确 organization 和 conversation。
- [ ] **CHAN-04**: 渠道出站消息记录 Provider receipt 和最终 delivery status。
- [ ] **CHAN-05**: 渠道失败执行有界 retry，并把耗尽任务送入可操作 dead-letter queue。
- [ ] **CHAN-06**: Admin 可以轮换或撤销渠道 credential，已有日志不会泄露旧 credential。

### Admin And Governance

- [ ] **ADMN-01**: 只有具备对应角色的用户可以访问 Admin、Finance 或 Moderation 操作。
- [ ] **ADMN-02**: Admin 对用户、组织和角色的修改作用于真实后端状态。
- [ ] **ADMN-03**: Admin 对 Provider、model、route、price、plan 和 channel 的配置会影响实际运行，并显示 readiness。
- [ ] **ADMN-04**: Finance operator 可以联查 request、usage、bill、payment、refund、settlement 和 payout。
- [ ] **ADMN-05**: Platform operator 可以查看 worker backlog、dead letter、依赖健康和 release blocker，并执行授权 replay/remediation。
- [ ] **ADMN-06**: 每项高风险 mutation 记录 actor、reason、before/after、organization 和 request identity。

### Operations And Deployment

- [ ] **OPER-01**: 生产 request log 和 audit 持久保存，并可按 organization 和 request identity 查询。
- [ ] **OPER-02**: Metrics 和 traces 携带必要业务 identity，且不使用高基数客户对象作为 metric label。
- [ ] **OPER-03**: 目标环境实际测量并报告 API P95 小于 500 ms、Relay 自身 P95 开销小于 100 ms、RAG 检索 P95 小于 2 秒、Workflow 成功率高于 99%、可用性不低于 99.9%。
- [ ] **OPER-04**: 告警可真实投递、acknowledge、escalate 和 recover，并留下 recovery audit。
- [ ] **OPER-05**: readiness 同时反映进程、必要依赖、worker ownership 和 backlog，而非只返回 health-only 成功。
- [ ] **OPER-06**: 运营者可以对每个声明部署模式执行 migration、deploy 和 rollback。
- [ ] **OPER-07**: 每个声明状态存储都完成 backup/restore drill，恢复后关键旅程和账务联查仍通过。
- [ ] **OPER-08**: 每个公开声明的 monolith、dual 或 split 模式通过相同能力与 tenant-denial 测试；否则从发布承诺和默认配置中移除。

### Release And Evidence

- [ ] **RELS-01**: capability/deployment manifest 明确列出承诺的模块、集成和运行模式，未承诺能力保持禁用。
- [ ] **RELS-02**: OpenAPI、protobuf、migration、前端 client 和当前 runtime 一致，contract drift 会阻断发布。
- [ ] **RELS-03**: CI 运行 production build、unit、integration、contract、race、lint、security、dependency、migration、restore 和必要 load gate，关键项不得静默 skip。
- [ ] **RELS-04**: Release operator 可以运行不拦截产品 API 的真实 Identity、Chat 和 Admin 浏览器旅程。
- [ ] **RELS-05**: Release operator 可以运行真实 Knowledge 上传、worker、检索和引用浏览器旅程。
- [ ] **RELS-06**: Release operator 可以运行真实 Agent、Workflow 和 Task target 浏览器旅程。
- [ ] **RELS-07**: Release operator 可以运行真实 subscription、payment、refund、Marketplace order、settlement 和 payout 旅程；声明渠道时同时覆盖渠道旅程。
- [ ] **RELS-08**: 同一 release commit 可以重复构建 immutable image 并记录其 digest。
- [ ] **RELS-09**: 每个发布制品附有可验证 SBOM、provenance、signature 和 vulnerability result，CI Actions 固定到完整 SHA。
- [ ] **RELS-10**: 每个声明的 Provider、支付渠道、PostgreSQL/vector backend、ClickHouse、Redis/Kafka、对象存储、观测后端和 Kubernetes 依赖都有新鲜 E3 target evidence，原始证据和 secrets 位于 git 外。
- [ ] **RELS-11**: 最终 strict verifier 对同一已 push commit 和 artifact digest、clean worktree 及已验证外部 artifact body 无 skip 通过。
- [ ] **RELS-12**: 每个 readiness 结论记录 evidence class、环境、commit、命令、migration state、pass/fail、skip 和 residual risk。

## v2 Requirements

延后到商业上线基线被证明之后。这些需求不映射到当前 Roadmap。

### Enterprise Identity And Governance

- **IDEN-20**: 用户可以使用 MFA 和 recovery codes。
- **IDEN-21**: 组织可以配置 OIDC/SAML SSO 和 verified domains。
- **IDEN-22**: 组织可以通过 SCIM provisioning/deprovisioning 用户。
- **IDEN-23**: 组织可以使用细粒度角色、定期 access review 和 dormant-account policy。
- **OPER-20**: 组织可以配置 retention、legal hold 和数据导出。
- **SECU-20**: 组织可以应用 model、tool 和 network policy pack/allowlist。

### Advanced AI And Automation

- **AUTO-20**: 获授权用户可以在 supervisor control 下接管运行中的 Agent。
- **AUTO-21**: Builder 可以创建受治理的 multi-Agent team。
- **AUTO-22**: Workflow 可以使用 distributed worker、checkpoint replay 和 Saga compensation。
- **KNOW-20**: Knowledge 可以使用 entity extraction 和 tenant-safe knowledge graph。
- **KNOW-21**: 运营者可以运行高级离线和在线 retrieval evaluation。
- **RLAY-20**: Fine-tuning、Assistants、Threads 和 Runs 具备完整身份、计费、补偿、治理和审计生命周期。

### Ecosystem And Expansion

- **MRKT-20**: Marketplace 支持 recommendation、creator analytics、risk scoring 和可配置 fee tier。
- **CHAN-20**: 首个渠道契约被证明后，平台支持更多真实渠道。
- **OPER-21**: 平台支持 multi-region routing、residency、active-active 和区域灾难恢复。
- **RELS-20**: 发布证据支持广泛 compliance program 和正式 penetration-testing result。
- **CHAT-20**: 用户可以使用受支持的 Desktop/PWA 和其他 client。

## Out of Scope

| Feature | Reason |
|---------|--------|
| 固定 100+ Provider、150+ tool、10+ channel 或 20+ node 数量 | 生命周期完整度和证据优先于目录数量。 |
| 复制 `reference/` 源码或业务语义 | 参考项目是设计输入，不是 Oblivious 实现证据。 |
| Big-bang 微服务或前端框架重写 | 当前运行契约和渐进商业收口优先。 |
| Relay 之外的第二套 Provider、usage 或 billing authority | 这会分裂 quota、账务、审计和事故证据。 |
| Production fake Provider、local payout、simulated telemetry 或 host Sandbox fallback | 缺少生产依赖时必须 fail closed。 |
| 把 MAU、GMV 或调用量作为仓库完成门槛 | 它们属于发布后的业务结果，不是工程完成证据。 |
| 把 secrets、kubeconfig、原始 target manifest 或 artifact body 提交到 git | 敏感信息和原始证据必须保留在受控外部 workdir。 |
| 为未声明部署模式完成固定 independent-service parity | 未声明模式必须从发布承诺中移除。 |
| 未经审批的高影响自动 remediation | 初始运营操作必须有界、可观察、已授权且可审计。 |

## Definition of Done

一条 v1 需求只有在所有适用条件均满足时才能完成：

1. 生产行为已经实现、验证、提交，并映射到唯一 Roadmap 阶段。
2. 仓库行为具有 E2 证据；涉及外部系统的需求还具有新鲜 E3 证据。
3. 最终发布具有来自外部 workdir 的 E4 同 commit、同 digest、无 skip 证据。
4. 不使用低等级证据、fixture、page、route、schema、proto、health endpoint 或文档替代所需运行证据。
5. 验证记录环境、commit、命令、migration state、pass/fail、skip 和 residual risk。
6. 任何未支持或未证明的能力均被禁用、从发布承诺中隐藏并 fail closed。

## Traceability

每条需求由哪个阶段覆盖。创建 Roadmap 时填充。

| Requirement | Phase | Status |
|-------------|-------|--------|
| 所有 v1 需求 | Unmapped | Pending |

**Coverage:**
- v1 requirements: 101 total
- Mapped to phases: 0
- Unmapped: 101

---
*Requirements defined: 2026-07-14*
*Last updated: 2026-07-14 after initial definition*
