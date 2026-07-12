# Oblivious Codex 大规模并发编排设计

> Status: Approved design
>
> Date: 2026-07-12
>
> Decision: Use native Codex SDK threads to run more than one hundred concurrent writing agents against path-isolated work packages.

## 1. Purpose

本规范定义 Oblivious 后续由上百个 Codex Agent 并发完成商业终版建设时的任务边界、执行拓扑、状态模型、隔离规则、合并策略和自动发布门禁。

目标不是把现有 Markdown 计划原样同时交给上百个 Agent，而是将项目转换为机器可调度的工作图，使每个 Agent 都满足以下条件：

- 只负责一个边界明确的纵向业务能力；
- 只写入一个独立稀疏 worktree 中声明的路径；
- 从冻结的契约和依赖摘要开始执行；
- 产生可独立验证、可回滚、可审计的候选提交；
- 不依赖人工分配、人工合并或人工发布；
- 在同一台达标主机上支持至少 110 个真实 Codex writer 同时运行。

本规范是编排控制面的设计，不宣称当前仓库已经具备该并发能力，也不将已有测试数量、计划勾选项或 fixture 成功视为运行证明。

## 2. Relationship To Existing Plans

本规范保留以下既有决定：

- 17 个目标服务及四个 domain unit；
- Relay、Billing、数据库、事件和部署边界；
- OpenAPI、Proto、event envelope 和 migration ownership 的单写者规则；
- 服务必须通过契约、租户隔离、race、恢复和目标环境证据门禁；
- repository-local evidence 与 target commercial release proof 必须分开。

本规范替代以下既有编排限制：

- 每阶段最多 6 个 writer；
- 每个服务只有一个实现 writer；
- 每个服务只使用一个实现 worktree；
- 共享租约只通过 Git 文件或人工 issue 管理；
- 每日一次、无状态机的人工集成；
- 最终 RC 必须等待人工批准。

受影响的现有计划包括：

- docs/superpowers/plans/2026-07-10-commercial-final-program-master-plan.md
- docs/superpowers/plans/2026-07-10-commercial-final-service-delivery-plan.md
- docs/superpowers/plans/2026-07-10-commercial-final-contract-freeze-plan.md
- docs/superpowers/plans/2026-07-10-commercial-final-deployment-profiles-plan.md
- docs/superpowers/plans/2026-07-10-commercial-final-architecture-governance-plan.md

后续实施计划必须显式更新这些冲突点，不能让旧的 6-writer 规则和新的 110-writer 规则同时成为执行真相。

本规范提交后是并发编排的唯一 source of truth。上述旧计划仍可作为产品 requirement 和历史设计来源，但其 writer 数量、worktree、lease、人工审批和执行顺序 checkbox 立即暂停，不能直接交给 worker 执行。

## 3. Binding Decisions

以下决定已经批准并具有约束力：

1. 执行器仅使用原生 Codex SDK，不使用 RuFlo 或 Claude Flow 作为 Agent runtime。
2. 控制面使用 TypeScript 和官方 @openai/codex-sdk。
3. control-plane 决策状态使用本机 PostgreSQL；Git 保存期望状态、schema 和不可变定义，Codex session bytes 保存在隔离 CODEX_HOME。
4. 常规交付阶段运行 17 个 Service Root 和 1 个 Web Platform Root。
5. 每个 Root 最多管理 8 个逻辑 worker slot；worker 是 Scheduler 直接创建的独立顶层 SDK thread，不是 Root 原生子 thread。
6. Root 使用 gpt-5.6-sol 和 ultra reasoning。
7. 所有非 Root Agent 使用 gpt-5.5 和 xhigh reasoning。
8. 不允许自动降低模型或 reasoning effort。
9. 不设置 token、credit 或总费用上限，只记录消耗。
10. 每个活动工作包使用独立稀疏 worktree。
11. 工作按纵向业务能力拆分，公共基础层使用专用单写者。
12. 约 168 个能力单元各拆为 Contract/RED、Implementation、Hardening、Evidence 四个顺序包，目标约 672 个原子工作包。
13. 合并采用 Package、Service、Domain、Global、RC 分层 merge train。
14. 每次失败使用新的 Codex Agent 和新的 attempt worktree；相同失败指纹连续两次后隔离。
15. 并发按 16、32、64、110+ writer 分级放量。
16. worker Agent 使用 workspace-write sandbox；Reviewer 和只读 Evidence Agent 使用 read-only。
17. 普通 Agent 不获得 Docker、Kubernetes、生产凭据或 worktree 管理权限。
18. main、RC、目标部署和回滚全部自动执行，不设置正常流程中的人工批准点。
19. 自动化必须 fail closed；缺少目标授权、凭据、证据或确定性门禁时停止。
20. 操作面同时提供 CLI/TUI 和只绑定本机的 Web Dashboard。

## 4. System Architecture

~~~text
CLI / TUI                    Local Web Dashboard
     \                            /
      \                          /
               Orchestrator API
                       |
                  PostgreSQL
                       |
        Scheduler / Lease / Resource Manager
                       |
          18 Codex SDK Executor Processes
             |                     \
             |              Credential Gateway -> Codex service
             |
       18 Root Threads + up to 144 Worker Threads
                       |
         Independent Sparse Worktrees
                       |
         Package -> Service -> Domain -> Global -> RC
                       |
        Privileged Operation Gateway -> authorized targets
~~~

### 4.1 Control-plane components

控制面位于 tools/codex-orchestrator/，由以下组件组成：

| Component | Responsibility |
|---|---|
| Orchestrator API | 提供 CLI、Dashboard 和内部 executor 使用的唯一写 API |
| Scheduler | 计算 ready frontier，分配 package、Root、Agent 和资源 token |
| DAG Engine | 校验依赖、artifact digest 和后继失效传播 |
| Lease Manager | 原子获取路径租约、维护 heartbeat 和 fencing token |
| Worktree Manager | 创建、配置、冻结、归档和回收稀疏 worktree |
| Codex Executor | 通过 Codex SDK 创建 Root 和独立 worker thread，并取消当前 turn |
| Credential Gateway | 在隔离边界内持有上游 Codex credential，并只接受已登记 transport network identity |
| Privileged Operation Gateway | 作为 target/provider/payment 的唯一 egress，在实际外呼边界线性化 authorization 和 intent fence |
| Event Ingestor | 将 SDK thread、turn、item 和错误事件投影到 PostgreSQL |
| Merge Controller | 创建候选提交并执行分层 merge train |
| Evidence Projector | 固化命令、日志、摘要、环境和产物证据 |
| Release Controller | 执行 Global gate、RC、目标部署和自动回滚 |
| Operator CLI/TUI | 提供脚本化和终端交互操作 |
| Local Dashboard | 展示 DAG、Agent、租约、队列、失败、证据和发布状态 |

### 4.2 Process model

单机运行以下长期进程：

- 1 个 Scheduler/API 进程；
- 1 个 Merge Controller 进程；
- 1 个 Evidence Projector 进程；
- 1 个 Credential Gateway 进程；
- 1 个 Privileged Operation Gateway 进程；
- 18 个 Codex Executor 进程；
- 1 个 PostgreSQL 实例；
- 1 个本地 Dashboard 服务。

每个 Codex Executor 同一时刻只承载一个 Root identity，并维护该 Root 的最多八个逻辑 worker slot。常规服务交付期间，18 个 identity 为 17 个 Service Root 加 1 个 Web Platform Root。Root thread 和 worker thread 都是独立顶层 SDK thread；Root 是组织、审阅和调度分组，不是 SDK parent thread。

PostgreSQL 使用独立的 oblivious_orchestrator database 和最小权限 role，不与任何产品服务数据库共用 schema 或 credential。上游 Codex credential 只由独立本地 Credential Gateway 持有；Executor transport 通过 CodexOptions.baseUrl 连接该 gateway，并由专用 network namespace/cgroup identity 鉴别。Codex CLI 不接收上游 API key、OAuth state 或可兑换 credential；若 CLI 协议要求 API-key 字段，只使用 gateway 接受但对任何上游无权限的 sentinel。credential value 不写入 PostgreSQL、package manifest、worktree、CODEX_HOME、Agent context 或 Agent tool environment。

进程 supervisor 只负责启动、健康检查和重启，不拥有 package 调度权。每个 attempt 另由一个受 supervisor 管理的 worker-runner subprocess 执行，runner 持有 worktree OS lock；runner 退出时，其 Codex CLI descendants 必须被同一 process group 或 cgroup 终止。

八个受保护共享域使用阶段性 protected Root。F0 期间，空闲 executor 被临时分配给 protected Root；进入 17-service 全量交付后，共享契约必须冻结。若交付期间发生共享契约变更，调度器先暂停受影响队列并排空一个 executor，再运行对应 protected Root。共享域 writer 与受影响 service writer 不得同时写入。

### 4.3 Codex thread limits

公开 TypeScript SDK 只提供 startThread、resumeThread、run 和 runStreamed。它不提供原生 child 的确定性创建、独立 cwd、单独取消或 child event identity。因此所有 package worker 都由 Scheduler 直接 startThread，并通过 workingDirectory 绑定唯一 worktree。

每个 Root 使用持久的独立 CODEX_HOME/CODEX_SQLITE_HOME，每个 worker attempt 使用自己的独立 transport state directory 并随 attempt retention 归档。该目录不含 auth state，只挂载给受信任的 Executor transport UID，不出现在 Agent tool mount namespace 中。Credential Gateway 在独立 UID/process/network boundary 内完成上游认证；worker state 不复制 user-wide hooks、plugins、agents.max_threads 或 agents.max_depth。Root client 和 worker client 均强制：

~~~toml
[features]
multi_agent = false
~~~

Root 和 worker 不得自行 spawn subagent。每 Root 八个 worker slot 和全局 144 个 worker 上限由 PostgreSQL semaphore 强制，不依赖 Codex 原生 agents.max_threads。

CodexOptions.env 一旦设置就不会自动继承 process.env。Executor 必须从固定 allowlist 构建 transport 环境，只包含运行所需 PATH、HOME/CODEX_HOME、CODEX_SQLITE_HOME、locale、CA、gateway base URL 和 non-secret sentinel；不得设置上游 apiKey，也不得无筛选透传 repository-controlled 或 host proxy environment variables。每次启动 Agent tool subprocess 时，受信任的 sandbox launcher 必须再次从更小 allowlist 构建 child env，删除 CODEX_HOME、CODEX_SQLITE_HOME、gateway URL、sentinel、proxy setting 和全部 auth variable。

配置值和客户端 turn 数不代表 OpenAI 服务端同时推理 slot。S4 证明的是至少 110 个真实 SDK worker turn 在本机并发运行，而不是无法从公开 SDK 观测的服务端推理并发。

### 4.4 SDK version contract

初始实现以 codex-cli 0.144.1 和对应 @openai/codex-sdk 0.144.1 API 为验证基线，并在 protected dependency package 中锁定版本。升级必须先通过 SDK compatibility suite，验证 startThread、resumeThread、runStreamed、ThreadOptions、AbortSignal、event union 和 session persistence。

设计不得假设公开 SDK 未提供的 child control、event replay、turn ID、create idempotency key 或 in-flight reconnect。后续 SDK 若新增能力，也只能通过新的 plan revision 启用。

## 5. Root And Agent Responsibilities

### 5.1 Root Codex

Root Codex 使用：

~~~toml
model = "gpt-5.6-sol"
model_reasoning_effort = "ultra"
sandbox_mode = "read-only"
~~~

TypeScript SDK 的 ThreadOptions 类型当前不包含 ultra。Root executor 必须通过 CodexOptions.config 注入 model = gpt-5.6-sol、model_reasoning_effort = ultra 和 features.multi_agent = false；不能把 ultra 传给只接受到 xhigh 的 ThreadOptions.modelReasoningEffort。

Root 初始 turn 使用 versioned root prompt template。SDK startThread 没有 agentType 或 developerInstructions 参数，因此 .codex custom-agent file 不能作为 Root 启动机制。

Root workingDirectory 是所属 Service 或 Web Queue head 的只读 coordinator checkout，不是任何 package worktree。Root 只在前一 turn 已完成后使用 resumeThread 接收新的摘要和决策请求。

Root 负责：

- 读取服务能力图、当前 ready packages 和依赖摘要；
- 对 Scheduler 提供的已登记 package 集合给出优先级和风险判断；
- 监控所属 worker 结果并向 Scheduler 返回结构化判断；
- 复核 package evidence 和 reviewer 结论；
- 请求 Merge Controller 入队；
- 对 quarantined package 做只读诊断并创建 repair package 请求。

Root 不得：

- 直接修改产品代码；
- 创建未登记的自由任务；
- 修改 package writeSet、依赖或 acceptance gate；
- 执行 git commit、rebase、merge 或 push；
- 自行批准自己的输出；
- 创建原生 subagent 或绕过 Scheduler 启动 worker。

### 5.2 Non-root Codex Agent

所有非 Root Agent 固定使用：

~~~toml
model = "gpt-5.5"
model_reasoning_effort = "xhigh"
~~~

每个 worker startThread 必须显式设置：

- workingDirectory 为唯一 package worktree；
- sandboxMode 为 workspace-write，或 reviewer/evidence 的 read-only；
- approvalPolicy 为 never；
- networkAccessEnabled 为 false；
- model 为 gpt-5.5；
- modelReasoningEffort 为 xhigh。

每个 worker Codex client 同时注入 features.multi_agent = false。普通 package timeout 由 Orchestrator 的 AbortController 实现，不依赖 agents.job_max_runtime_seconds。

角色包括 Contract、Implementation、Hardening、Evidence、Reviewer、Verifier 和 Repair。角色只改变 versioned prompt、sandbox 和 writeSet，不改变模型。SDK 顶层 thread 不支持 custom-agent type 参数，因此这些角色由 Orchestrator 提供的 prompt template 和 output schema 定义，不依赖 .codex/agents 中的 spawn profile。

实现 Agent 的最终响应必须符合结构化 output schema，至少包含：

- packageId；
- attempt；
- claims；
- filesChanged；
- commandsRun；
- commandResults；
- unresolvedItems；
- requestedFollowups；
- finalStatus。

Agent 不创建 Git commit。Agent 完成后，Worktree Manager 校验实际 diff 是 writeSet 子集，Merge Controller 再创建带审计 trailer 的候选提交。

Contract/RED package 在 immutable dependency ref 固化后，其 contract fragment 和 RED tests 对后续 I/H package 变为只读输入。Implementation 或 Hardening Agent 需要改变这些文件时，必须创建新的 contract revision，不能在实现包中顺带弱化测试。

## 6. Capability Ownership

### 6.1 Domain roots

| Domain | Roots |
|---|---|
| Foundation | identity-access, api-gateway, event-contract-platform, platform-ops, observability-audit |
| AI Runtime | relay, knowledge-rag, tool-mcp, sandbox |
| Product | chat, agent, workflow, task-scheduler, channel |
| Commerce | billing-payment, marketplace, admin-console |
| Web | web-platform |

### 6.2 Capability cells

业务能力单元约 160 个：

| Root | Capability cells |
|---|---|
| identity-access | users, organizations, memberships-rbac, invitations, sessions, api-credentials, mfa, enterprise-federation, service-identity |
| api-gateway | route-registry, identity-context, http-proxy, sse, websocket, idempotency, traffic-policy, origin-csrf, error-mapping |
| event-contract-platform | envelope-registry, schema-registry, topic-registry, outbox-runtime, inbox-runtime, dlq-redrive, ordering-dedup, compatibility-runtime |
| platform-ops | provisioning, secrets, deployment-runtime, migration-orchestration, backup-restore, disaster-recovery, air-gap-runtime, release-evidence-index |
| observability-audit | request-logs, audit-ledger, metrics, traces, logs, alerts, slo, dashboards-retention |
| relay | providers, models, routing, streaming, embeddings, realtime, batch, rate-limit, usage-cost, fallback-cache |
| knowledge-rag | kb-lifecycle, upload, parser, chunking, embedding, indexing, retrieval, rerank-citations, worker-replay, delete-reindex |
| tool-mcp | registry, transport, oauth-secrets, discovery, outbound-policy-integration, approval, invocation, health-audit |
| sandbox | job-lease, runtime-policy, workspace, execution, network-isolation, resource-quota, artifacts, cleanup-recovery |
| chat | conversations, messages, attachments, model-settings, stream-reconnect, tools, knowledge-citations, sharing-export, history-search |
| agent | definitions, runs, planning, memory, tools, approval, sandbox, streaming, resume-retry, budget-evaluation |
| workflow | definitions, graph, triggers, run-state, node-executors, context-secrets, branch-loop, checkpoint-replay, debugger, publish |
| task-scheduler | definitions, cron, queue-lease, agent-dispatch, workflow-dispatch, budget, retry-cancel, history-results |
| channel | configuration, slack, feishu, dingtalk, telegram, webhook-email, format-media, receipts-retry, inbound-events |
| billing-payment | plans, subscriptions, quota, usage, rating, ledger, invoice, checkout-topup, payment-webhooks, refund-reconciliation |
| marketplace | listing, publishing, review, search, install, checkout, entitlement, payout, reconciliation-governance |
| admin-console | admin-rbac, tenant-ops, model-ops, finance-ops, moderation, audit-incidents, configuration, support-privacy, release-evidence |
| web-platform | router, auth-csrf, http-client, generated-client-consumption, design-system, layouts, state-cache, accessibility-test-harness |

### 6.3 Protected shared domains

另外八个能力单元始终使用全局单写者：

| Domain | Protected surfaces |
|---|---|
| openapi-common | API root、common schemas、bundler 和最终 bundle |
| proto-common | shared request/error Proto、descriptor 和 generators |
| event-common | event envelope、topic catalog 和 shared event SDK |
| migration-ownership | table ownership、migration runner 和 checksum policy |
| go-shared-security | shared config、identity propagation、outbound policy 和 common Go packages |
| dependency-locks | go.mod、go.sum、package manifests、pnpm lock 和 tool versions |
| deployment-ci | Compose、Helm shared values、profiles、CI 和 image matrix |
| release-gates | commercial verifier、target evidence、digest lock 和 RC policy |

### 6.4 Overlap resolution

Capability 名称不授予共享路径所有权。以下边界消除语义重叠：

| Capability | May write | Must consume read-only |
|---|---|---|
| tool-mcp/outbound-policy-integration | src/server/internal/mcp/** 下的 MCP-specific adapter/policy wiring | src/server/internal/outboundpolicy/** shared enforcement library |
| go-shared-security | src/server/internal/outboundpolicy/**、shared config/security packages | MCP/Agent/Workflow business packages |
| event-contract-platform/*-registry | 该服务的 runtime registry、private store 和 API | api/proto/oblivious/events/**、contracts/events/** shared source |
| event-common | shared event schema、topic source 和 generated shared SDK | event-contract-platform runtime |
| platform-ops/migration-orchestration | platform-ops runtime and jobs | table ownership、shared migration runner |
| migration-ownership | ownership manifest、runner 和 checksum policy | platform-ops runtime |
| platform-ops/deployment-runtime | deployment operation runtime and evidence index | Compose、Helm shared values、profile sources 和 CI |
| deployment-ci | Compose、Helm shared values、profiles、CI 和 image matrix | platform-ops runtime |
| platform-ops/release-evidence-index | runtime projection/index | release verifier scripts、RC policy 和 digest locks |
| release-gates | verifier scripts、RC policy、release/manifests/** | platform-ops projection |
| web-platform/generated-client-consumption | typed wrappers、query adapters 和 business-neutral client facade | src/web/src/generated/** |
| openapi-common | docs/api/openapi.yaml、api/openapi/**、src/web/src/generated/openapi.ts | Web Platform wrappers |
| proto-common | api/proto/oblivious/common/**、service Proto source 和 declared Go gRPC generated root | service runtime consumers |
| web-platform | business-neutral source files excluding package manifests | root package.json、src/web/package.json、pnpm-lock.yaml |
| dependency-locks | root and Web package manifests、all dependency locks | Web Platform source |

Manifest Compiler 必须把该表展开到 exact path policy，并证明每个 writable path 只有一个 owner。语义 capability 重叠但 exact paths 不重叠时不构成 lease 冲突。

## 7. Frontend Ownership

业务页面和业务组件跟随对应 Service Root。示例：

- Chat 页面和 feature 归 chat；
- Knowledge 页面和 feature 归 knowledge-rag；
- Agent、Memory、Plan Steps 和 SOLO 归 agent；
- Workflow 页面归 workflow；
- Scheduled Tasks 页面归 task-scheduler；
- Publishing Channels 页面归 channel；
- Marketplace 页面归 marketplace；
- Billing 和 Usage 业务页面归 billing-payment；
- Admin 业务页面按权威服务归 observability-audit、billing-payment、marketplace、identity-access 或 admin-console。

Web Platform Root 独占：

- src/web/src/app/**
- router 和 route registry
- protected-route shell 和全局 auth/CSRF transport
- shared HTTP client 和 error mapping
- generated API clients/types
- shared layout、navigation、theme 和 design system
- global state/query cache
- accessibility、i18n、performance 和 route test harness
- src/web/package.json 及前端依赖变更请求

业务 Agent 不得修改 router、generated clients、共享 layout 或依赖锁。需要这些变更时创建 Web Platform 或 protected dependency package。

## 8. Work Package Model

### 8.1 Four-package pipeline

每个 capability cell 固定拆成四个顺序包：

~~~text
<capability>-C  Contract/RED
        |
<capability>-I  Implementation
        |
<capability>-H  Hardening
        |
<capability>-E  Evidence
~~~

职责如下：

| Kind | Required output |
|---|---|
| C | capability contract fragment、失败的 consumer/provider test、依赖摘要和 acceptance claims |
| I | 满足 frozen contract 的最小生产实现和 focused tests |
| H | tenant isolation、security、concurrency、retry、recovery、load 和 failure-path hardening |
| E | 只读执行验收命令并形成 evidence manifest；不通过时创建 repair request |

能力数约 168，因此基线工作包约 672 个。Repair、contract change 和 environment remediation package 是运行时新增节点，不计入基线数量。

### 8.2 Package ID

ID 使用稳定格式：

~~~text
OBL-<DOMAIN>-<SERVICE>-<CAPABILITY>-<KIND>
~~~

例如：

~~~text
OBL-AIR-RELAY-REALTIME-C
OBL-AIR-RELAY-REALTIME-I
OBL-AIR-RELAY-REALTIME-H
OBL-AIR-RELAY-REALTIME-E
~~~

attempt 不进入 package ID，而进入 branch、worktree 和运行态记录。

### 8.3 Machine-readable definition

Git 中的 package definition 至少包含：

~~~yaml
apiVersion: orchestration.oblivious/v1
kind: WorkPackage
metadata:
  id: OBL-AIR-RELAY-REALTIME-I
  revision: 1
  parentId: OBL-AIR-RELAY-REALTIME
  title: Implement Relay realtime capability
  sourceRefs: []
scope:
  phase: delivery
  domain: AI Runtime
  service: relay
  capability: realtime
  packageKind: implementation
  risk: high
  priority: p0
inputs:
  baseSha: immutable-git-sha
  contractDigest: sha256-value
  migrationDigest: sha256-value
  requiredArtifacts: []
dependencies:
  - packageId: OBL-AIR-RELAY-REALTIME-C
    condition: gate_passed
    artifactDigest: sha256-value
ownership:
  writeSet: []
  readSet: []
  forbiddenSet: []
  leaseClass: service-exclusive
execution:
  agentRole: implementation
  rootId: relay
  resourceClass: go-service
  timeoutSeconds: 3600
  maxAttempts: 2
acceptance:
  claims: []
  commands: []
  requiredGates: []
  outputArtifacts: []
  evidenceClass: component
recovery:
  mode: revert
  invalidates: []
~~~

package definition 在一次 program run 内不可原地修改。变更会创建新的 plan revision，并使依赖旧 digest 的 candidate 变为 stale。

上面的 YAML 是字段示例，immutable-git-sha 和 sha256-value 不是可执行默认值。Importer 必须拒绝空 base SHA、空必需 digest、未知 enum 和未解析的 sourceRefs。

evidenceClass 只允许：

| Value | Meaning |
|---|---|
| repository | 静态、单元、contract 或 repository-local verifier |
| component | 独立服务加真实依赖的 component evidence |
| target | 预授权目标环境中的真实运行 evidence |
| release | 与一个 RC digest 绑定的最终 evidence bundle |

普通 coding package 默认 timeout 为 3600 秒，manifest 可提高到最多 14400 秒。更长的 load、soak、chaos 或 target deployment 必须作为受资源池管理的 Verifier job，不能通过延长普通 Agent turn 占用 writer slot。

### 8.4 Effective base materialization

package definition 中的 baseSha 是 declaredBaseSha，只定义允许的最早祖先。每个 attempt 在 dispatch 前必须计算 materializedBaseSha：

1. 读取目标 Service、Protected 或 Web Queue 的 expected integration head；
2. 收集全部直接和传递依赖的已合并 commit 与 artifact digest；
3. 按 deterministic topological order 生成 synthetic materialization commit；
4. 验证 C、I、H、E 前驱内容均已包含；
5. 将 materializedBaseSha、dependency commit vector 和 expected queue head 写入 attempt；
6. 从 materializedBaseSha 创建 worktree。

因此 I 必须看到 C 的 contract/RED tests，H 必须看到 I，E 必须看到 H。materializedBaseSha 在 attempt 内不可变化。dependency vector、contract digest、path policy 或 writeSet 所依赖内容改变时，该 attempt 变为 stale。

expected queue head 变化本身不使 disjoint attempt stale。Merge Controller 把 immutable candidate tree change 重放到当前 head：若 write paths 不冲突、dependency digests 未变且 synthetic gates 通过，则生成新的 proposed merge commit；只有 merge conflict、依赖变化或 policy 变化才要求重新 materialize。这样同一 Service Queue 的并行 package 不会因前一个 disjoint merge 而全部返工。

## 9. Git Source Of Truth

机器可读资产位于：

~~~text
.planning/orchestration/
  program.yaml
  clusters.yaml
  dag.yaml
  path-policy.yaml
  concurrency.yaml
  merge-policy.yaml
  gates.yaml
  migration-map.yaml
  packages/<phase>/<package>.yaml
  schemas/program.schema.json
  schemas/package.schema.json
  schemas/lease.schema.json
  schemas/evidence.schema.json
  schemas/event.schema.json
  templates/package.yaml
  templates/evidence.yaml
  templates/rollback.yaml
  generated/status.md
~~~

Git 保存：

- program 和 package 定义；
- schema；
- path policy；
- gate policy；
- capability ownership；
- immutable baseline digest；
- 人类可读的生成状态快照。

Git 不保存活动 lease、heartbeat、assignment、queue position 或 mutable attempt status。

## 10. PostgreSQL Runtime State

PostgreSQL 是唯一 control-plane decision truth。Codex SDK session files 是外部执行状态，由隔离 CODEX_HOME 管理且只能通过 reference 关联。PostgreSQL 至少包含：

| Table | Purpose |
|---|---|
| program_runs | run、plan revision、base SHA、状态和并发 stage |
| work_packages | package 定义摘要、当前投影状态以及 dependency/authorization/environment revision snapshot |
| package_dependencies | 显式 DAG 边和所需 artifact digest |
| attempts | attempt、state version、declared/materialized base、dependency/authorization/environment revision、dispatch generation、Agent、thread、worktree、candidate 和失败信息 |
| codex_roots | Root identity、executor、thread 和 health |
| codex_threads | SDK thread identity、session location 和 observed event sequence |
| worktrees | path、branch、base、size、lifecycle 和 archive reference |
| path_leases | normalized scope、holder、TTL、heartbeat 和 fencing token |
| lease_acquisition_attempts | assignment/epoch、requested scope digest、result、conflict scope、started/finished time 和 audit digest |
| resource_tokens | build、DB、browser、Docker、Kubernetes 和 load-test token |
| assembly_jobs | owner、C candidate vector、generated outputs、F1 ref/digest 和状态 |
| review_jobs | candidate、reviewer thread、read-only checkout、result digest 和状态 |
| verifier_jobs | command template、authorization/environment revision、resource token、result 和 evidence |
| job_attempts | review/verifier/assembly job、attempt number、dispatch generation、claim heartbeat、runner、input digest 和失败分类 |
| merge_queues | queue、entry、synthetic merge、gate 和 ordering |
| merge_intents | intent revision、expected old ref、proposed new ref、candidate、operation、conflict evidence 和 reconciliation 状态 |
| evidence_intents | read-only E package、base/candidate、evidence digest、expected gate revision 和应用状态 |
| release_candidates | RC artifact/migration digest set、stateVersion、saga state、target/recovery vector 和 barrier digest |
| release_participants | RC/profile、migration/deployment/traffic/recovery intent links、participant state 和 observation digest |
| release_targets | environment/profile、active RC/digest、previous stable digest、stateVersion、health 和 last observation |
| authorization_policies | immutable policy revision、allowed operation set、principal class 和状态 |
| environment_registrations | environment revision、profile、command/credential references、network policy、rollback contract 和 preflight expiry |
| environment_fences | environment/target scope、monotonic fencing token、active operation 和 heartbeat |
| privileged_operation_intents | operation key、profile、target、desired/prior digest、authorization/environment revision、dispatch generation、fence、external idempotency key 和 reconciliation state |
| privileged_operation_attempts | intent、dispatch generation、gateway claim、heartbeat、external operation ID、raw result digest 和 terminal-negative proof |
| concurrency_epochs | run/stage/epoch number、policy revision、slot vector、open/close barrier、state 和 metrics digest |
| epoch_slot_samples | epoch、logical slot、assignment/attempt、sample class、settled disposition、timestamps 和 inclusion reason |
| evidence_manifests | command、exit code、log digest、artifact digest 和 environment |
| failure_fingerprints | normalized failure、count、first/last seen 和 quarantine |
| quarantines | package、attempt、reason、diagnosis 和 repair link |
| scheduler_events | append-only state transition and audit event |
| outbox_events | transactionally published control-plane events |

scheduler_events 使用前一事件摘要构成 hash chain，并定期生成签名 checkpoint artifact。数据库 role 禁止 UPDATE 和 DELETE audit rows，从而使“append-only”成为可验证约束而不是文档约定。

### 10.1 Atomic assignment

Scheduler 在一个 PostgreSQL transaction 中：

1. 使用 SELECT FOR UPDATE SKIP LOCKED 领取一个 ready package；
2. 验证 plan revision、dependency artifact digest、pinned authorization policy revision 和 environment registration revision 仍匹配，且所需 preflight 尚未过期；
3. 创建 lease_acquisition_attempt，使用 INSERT ON CONFLICT DO NOTHING 原子尝试全部 writeSet lease；
4. 为每个 lease 递增 fencing token；
5. 申请 Codex executor 和逻辑 worker slot；
6. 若该 slot 在当前 epoch 尚无 primary ticket，原子创建 epoch_slot_sample 并绑定本次 assignment；其他 assignment 明确标记 unwindowed；
7. 创建带 stateVersion、dispatchGeneration 和可选 epochSampleId 的 attempt；
8. 创建基于 materializedBaseSha 的 worktree request；
9. 写入 outbox 和 append-only audit event。

lease contention 是预期的 committed negative outcome，不作为数据库异常抛出：若第 3 步未取得全部 scope，transaction 删除本次 provisional lease、把 lease_acquisition_attempt 标记 conflict、记录 conflict scope/digest 和 append-only scheduler_event，保持 package 为 ready，然后提交；它不能创建 attempt、slot reservation 或 outbox dispatch。成功领取则在同一 assignment transaction 把 acquisition row 标记 acquired。只有非预期错误才回滚整个 transaction。两条路径都不允许出现“package 已分配但没有租约”或“租约已占用但没有 attempt”的半状态，同时冲突 numerator 不会因 rollback 丢失。

Worktree Manager 完成 sparse checkout、base/digest 校验和 OS lock file 初始化后，使用 stateVersion CAS 将 attempt 标记为 worktree_ready。worktree_ready 之前不得发送 SDK prompt。

Scheduler 随后创建唯一 pending dispatch claim，但 attempt 暂时保持 worktree_ready：

- key 为 attemptId 加 dispatchGeneration；
- unique constraint 保证一个 generation 只有一条 claim；
- transaction 记录 executor、runner request 和 outbox event；
- worker-runner 先取得 session-level PostgreSQL advisory lock，再取得 worktree flock；
- runner 在持有双锁后执行 transaction，重新验证 attempt stateVersion、dispatchGeneration、plan revision、materializedBaseSha、dependency/authorization/environment revision、preflight expiry 和完整未过期 lease vector；
- 同一 transaction 将 claim 从 pending 改为 active、attempt 从 worktree_ready 改为 dispatching，并写入 runner ID、claimHeartbeatAt 和 claimExpiresAt；
- transaction 成功后才允许调用 SDK；
- 重复 outbox delivery 无法取得 advisory lock、flock 或 pending claim，因此不能启动第二个 writer。

runner 调用 SDK startThread 和 runStreamed。thread.started 到达后，Event Ingestor 只允许一次 CAS 绑定 thread ID 并进入 running。由于 SDK 没有 client-generated thread ID 或 create idempotency key，若 runner 在 thread.started 绑定前失联，attempt 进入 indeterminate；supervisor 必须先终止该 runner process group 及全部 descendants，再决定是否创建新 attempt。

active dispatch claim 和 runner 每 30 秒 heartbeat，120 秒过期。过期后 Reconciler 必须取得单独的 attempt recovery advisory lock，检查 cgroup/process group、flock 和 stream；它只能恢复已证明存活的原 runner，或终止全部 descendants 后把 attempt 标记 indeterminate。禁止把过期 claim 直接改回 pending。

Build、database、browser、Docker、Kubernetes 和 target environment token 在 Agent 实际请求相应 command 时按需获取，不在整个 coding turn 中长期占用。

review、verifier、assembly 和 privileged operation 使用各自的原子 claim transaction，不复用 writer assignment。Verifier 或 Release Controller 只有在同一 transaction 中锁定 job/intent、复核 plan/authorization/environment revision 和 credential-reference digest、取得 resource token 或 environment fence、创建新 job_attempt/dispatchGeneration 并写出 outbox 后才能执行。ready-frontier 或 transaction 外 preflight 只用于调度提示，不能作为授权依据。任何 plan/policy/environment revision 变化都会使旧 claim 失效，并由 watcher 把尚未执行的 package/job/intent 改回 blocked 或 stale。

### 10.2 State machine

Package projection、worker attempt、review/verifier/assembly job projection 和 job_attempt 使用不同 state column 和 stateVersion，禁止共用一个 enum。

Package projection：

~~~text
draft
  -> validated
  -> blocked | ready
  -> leased
  -> running
  -> candidate
  -> verifying
  -> reviewed
  -> queued
  -> merged
  -> integrated
  -> evidenced
  -> done
~~~

Package exceptional states：

- stale；
- quarantined；
- superseded；
- rollback_pending；
- rolled_back；
- forward_recovered；
- aborted。

Attempt：

~~~text
reserved
  -> worktree_preparing
  -> worktree_ready
  -> dispatching
  -> running
  -> candidate
~~~

Attempt terminal or exceptional states 为 retryable_failed、stale、authorization_blocked、environment_blocked、indeterminate、aborted。indeterminate 必须先 reconciliation，不能直接重派。

Review job：

~~~text
pending -> running -> passed | changes_required | invalid_candidate
        -> retry_wait -> pending
        -> authorization_blocked -> pending
        -> aborted
~~~

Verifier job：

~~~text
pending -> resource_wait -> running -> passed | failed
        -> retry_wait -> pending
        -> authorization_blocked | environment_blocked -> pending
        -> aborted
~~~

F1 assembly job：

~~~text
pending -> ready -> running -> published
        -> retry_wait -> ready
        -> content_failed | conflict -> superseded
        -> aborted
~~~

每次 job 执行都创建不可变 job_attempt：

~~~text
reserved -> dispatching -> running -> succeeded | domain_failed
                                -> infrastructure_failed
                                -> authorization_blocked | environment_blocked
                                -> indeterminate | aborted
~~~

job_attempt 的 indeterminate 只能在持有 recovery lock 后恢复到 running，或在证明并终止原 process group 后进入 aborted。infrastructure_failed、reconciled aborted 和可恢复的 blocked 结果会让稳定 job projection 进入 retry_wait/blocked；下一次执行必须增加 attempt number 和 dispatchGeneration，不能复用旧 claim。domain_failed 映射为 reviewer changes_required/invalid_candidate、verifier failed 或 assembly content_failed，不作为基础设施重试。

稳定 job projection 的恢复转换为：

| Transition | Guard | Effect |
|---|---|---|
| review/verifier/assembly running -> retry_wait | 当前 job_attempt infrastructure_failed，或 reconciled aborted 且 parent 未取消 | 冻结 attempt evidence并计算 backoff |
| review retry_wait -> pending | backoff 到期且 candidate/revision 仍 current | 创建下一 job_attempt 的资格 |
| verifier retry_wait -> pending/resource_wait | backoff 到期且 authorization/environment revision current | 重新申请 resource token |
| assembly retry_wait -> ready | backoff 到期且 immutable C vector/expected ref 未变 | 以相同 input digest 重试 |
| review authorization_blocked -> pending | current authorization revision 重新满足 | 新 generation claim |
| verifier authorization_blocked/environment_blocked -> pending | current revisions 和 preflight 重新满足 | 新 generation claim |
| assembly content_failed/conflict -> superseded | repair/ref-resolution 产生新 immutable vector/revision | 创建新 assembly_job，旧 job 永不 re-arm |
| any nonterminal job -> aborted | parent package/program 被显式取消 | 终止活动 job_attempt且不自动重派 |

job_attempt aborted 只有在 parent 仍 active 时映射为稳定 job retry_wait；稳定 job 的 aborted 是真正终态。

Package projection 转换：

| Transition | Guard | Effect |
|---|---|---|
| draft -> validated | schema、DAG、ownership 和 plan revision 有效 | 固化 definition digest |
| validated -> blocked | dependency、authorization 或 environment condition 未满足 | 保存结构化 blocking reasons |
| validated -> ready | 所有静态条件满足 | 进入 ready frontier |
| blocked -> ready | 全部 dependency、authorization 和 environment condition 满足，pinned revisions 仍为 current 且 preflight 未过期 | 进入 ready frontier |
| ready -> blocked | authorization/environment condition 失效但 immutable input revision 未变 | 从 frontier 移除并保存 reason |
| validated/blocked/ready -> stale | plan、dependency、contract、path policy、authorization policy 或 environment registration revision 被替换 | 创建 replacement package revision |
| ready -> leased | assignment transaction 成功 | 创建 attempt、lease vector 和 worker reservation |
| leased -> running | 当前 attempt 已进入 running | 投影 active attempt |
| running -> candidate | 当前 attempt 以 candidate 结束 | 绑定 immutable candidate SHA |
| leased/running -> ready | 当前 attempt 以 retryable_failed 或 reconciled aborted 结束，且 maxAttempts 未耗尽 | 原子释放旧 lease/slot；下一次 assignment 创建新 attempt |
| leased/running -> blocked | 当前 attempt 以 authorization_blocked 或 environment_blocked 结束 | 原子释放 lease/slot并保存 blocking condition |
| leased/running -> stale | plan、dependency、contract、path policy、authorization policy 或 environment registration revision 改变 | 终止 runner、释放 lease/slot并生成 replacement revision |
| leased/running -> quarantined | Agent failure 已耗尽 maxAttempts | 释放 lease/slot并创建 diagnosis request |
| candidate -> verifying | review_job 和 required verifier_jobs 已原子创建 | 启动独立 gates |
| verifying -> reviewed | reviewer passed 且全部 verifier passed | 固化 result digests |
| verifying -> ready | reviewer changes_required 或 candidate-attributable verifier failed，且 writer maxAttempts 未耗尽 | 拒绝 candidate，创建下一 attempt |
| verifying -> quarantined | invalid_candidate，或任一 candidate rejection 后 maxAttempts 耗尽 | 阻塞后继并创建 diagnosis request |
| candidate/verifying/reviewed/queued -> stale | plan、immutable dependency、contract、path/policy、authorization 或 environment revision 改变 | 取消未完成 job/queue entry，拒绝 candidate 并创建 replacement revision |
| reviewed -> queued | queue entry CAS 成功 | 创建 immutable queue entry |
| queued -> ready | synthetic gate 的 candidate-attributable failure，且 maxAttempts 未耗尽 | 拒绝 queue entry/candidate并创建下一 writer attempt |
| queued -> quarantined | synthetic gate 的 candidate-attributable failure，且 maxAttempts 已耗尽 | 阻塞后继并创建 diagnosis/repair request |
| queued -> merged | merge_intent applied | 记录 target ref 和 merge SHA |
| merged -> integrated | 对应 Service、Protected 或 Web integration gate 通过 | 允许下一级 queue |
| integrated -> evidenced | package 所需 evidenceClass 全部满足 | 固化 evidence digest |
| evidenced -> done | 所有后继触发事件已写入 outbox | 关闭 package |
| merged/integrated/evidenced/done -> rollback_pending | 上游 rollback、已发布 contract 撤销或 release compensation 要求反向传播 | 阻止后继与新 release，按反向拓扑创建 rollback/forward-fix intent |
| rollback_pending -> rolled_back | rollback intent applied，且 code、traffic、data/schema 均观察到 exact prior-state digest | 固化 exact rollback evidence |
| rollback_pending -> forward_recovered | 不可逆 migration 的预声明 forward-fix 已恢复所有 invariant，但 state digest 不等于 prior | 固化 recovery digest，禁止标记 rolled_back |
| draft/validated/blocked/ready/leased/running/candidate/verifying/reviewed/queued -> aborted | program 或已授权 package abort，且所有 active process 已终止 | 取消 job/queue entry、释放资源并保留 evidence |
| quarantined -> superseded | repair package 通过且产生 replacement revision | 原 package 永久保留，replacement 进入 validated/ready |
| stale -> superseded | 新 dependency vector 或 plan revision 已生成 | 创建 replacement package revision |
| rolled_back/forward_recovered -> superseded | replacement package 已生成 | 保留原 merge/recovery audit chain |

Attempt 转换：

| Transition | Guard | Effect |
|---|---|---|
| reserved -> worktree_preparing | worktree request 被唯一 manager claim | 创建 sparse checkout |
| worktree_preparing -> worktree_ready | materialized base、lease vector 和 checkout 校验成功 | 允许 dispatch claim |
| worktree_ready -> dispatching | runner 已持有 advisory lock、flock 和有效 dispatch claim | 调用 SDK |
| dispatching -> running | thread.started 与 attempt/generation 匹配 | 绑定 SDK thread ID |
| running -> candidate | output schema、diff 和 finalization CAS 成功 | 释放 write leases |
| reserved/worktree_preparing/worktree_ready/dispatching/running -> retryable_failed | 已证明没有 in-flight turn 的 setup/executor failure，或 Agent-attributable execution、output、diff/package gate failure | 冻结 evidence；只有 Agent-attributable 分类消耗 maxAttempts |
| reserved/worktree_preparing/worktree_ready/dispatching/running -> stale | pinned plan/immutable revision 或 lease fence 失效 | 终止 runner并冻结 attempt |
| reserved/worktree_preparing/worktree_ready/dispatching/running -> authorization_blocked/environment_blocked | 当前授权或环境条件失效 | 终止 runner、释放 lease并阻塞 package |
| reserved/worktree_preparing/worktree_ready/dispatching/running -> aborted | program/package abort，且 process group termination 已证明 | 冻结 worktree/evidence并释放资源 |
| dispatching/running -> indeterminate | runner/stream 失联且无法证明 SDK turn 已停止 | 取得 recovery lock并禁止新 dispatch |
| indeterminate -> running | 原 runner、process group、flock 和 stream 均可证明仍存活 | 恢复 observed event ingestion |
| indeterminate -> aborted | 无法证明原 turn 仍安全运行，且 process group termination 已证明 | 冻结 worktree；parent package 仍 active 时才按 retry policy 新建 attempt，parent/program abort 已请求时禁止重派并完成 package abort |

invalid_candidate 和 verifier failed 不直接改写已经 terminal candidate 的 worker attempt；它们是独立 job 的 domain result，并按 Package verifying 转换决定新 attempt 或 quarantine。review/verifier job 处于 retry_wait、authorization_blocked 或 environment_blocked 时，package 保持 verifying 且不能入队。

所有 package、worker attempt、job projection 和 job_attempt 转换都必须使用各自 stateVersion CAS。跨实体副作用和 outbox event 在同一 PostgreSQL transaction 中提交。

每个 Service、Protected domain 和 Web owner 对每个 immutable C candidate vector 都有一个 deterministic F1 assembly_job revision。全部 required C candidates 达到 reviewed 且 immutable dependency refs 存在后，job 进入 ready。Assembler：

1. 按 packageId 排序读取 C contract/ref/digest vector；
2. 生成 OpenAPI/Proto/typed client 或 owner-specific contract bundle；
3. 运行 compatibility 和 deterministic regeneration gates；
4. 先写入 immutable artifact 和 Git object；
5. 使用 expected-ref CAS 发布 refs/oblivious/contracts/f1/{owner}；
6. crash recovery 按 expected old ref、proposed ref 和 deterministic object digest 判断补记 published、安全重试 CAS 或进入 conflict；
7. 只有观察到 published ref 后，才在同一 PostgreSQL transaction 中记录 F1 digest、output digests 和后继唤醒事件。

infrastructure_failed 只终止当前 job_attempt；稳定 assembly_job 经 retry_wait 使用相同 input digest 和新 generation 重试。content_failed 会拒绝该 immutable C vector并创建 contract repair request。expected ref 已被不同 digest 移动时进入 conflict，冻结 proposed object并创建 ref/contract repair request，禁止覆盖当前 ref。repair C candidate 或 ref-resolution package reviewed 后，content_failed/conflict job 进入 superseded，系统基于新的 immutable C vector 和 expected ref 创建全新 pending job，禁止原地修改或 re-arm 旧 vector。I package 在对应 assembly_job published 前保持 blocked，但 repair/supersede DAG 是该 blocked state 的必需出口。assembly_job 是运行态 gate，不计入 672 baseline packages。

### 10.3 SDK session and event boundary

PostgreSQL 是唯一 control-plane decision state，但不是 Codex SDK session bytes 的存储。SDK session 由隔离 CODEX_HOME 下的 Codex session files 保存，codex_threads 只记录其 reference。

SDK event 没有 event ID、Codex turn ID 或 replay cursor。Event Ingestor 为已观察事件分配本地 observedSequence 和 payload digest；该序列只用于审计，不能宣称可补回 crash 期间漏失事件。

resumeThread 只允许对已经完成或已知空闲的 thread 发起后续 turn。它不能重新连接正在运行的 turn。executor crash 后必须进入 indeterminate reconciliation，不能根据 PostgreSQL cursor 假装恢复 in-flight turn。

## 11. Dependency DAG

静态顺序为：

~~~text
Architecture governance
  -> shared F0 contracts
  -> capability C packages
  -> per-service F1 assembly and baseline
  -> capability I packages
  -> capability H packages
  -> capability E packages
  -> service integration gates
  -> domain journeys
  -> web/global journeys
  -> deployment profiles and NFR
  -> RC and automatic target deployment
~~~

Capability C 是 F1 contract freeze 的组成部分，不在 F1 发布之后再次修改 frozen contract。每个 Service/Protected/Web F1 assembler 收集已通过 review 和 red-proof 的 C contract fragments，生成 deterministic bundle、generated clients 和 F1 digest。I package 同时依赖对应 C candidate 和已发布 F1 digest。

现有 contract-freeze plan 中“一次性先发布全部 F1、再开始 capability work”的执行顺序被本规范替代；共享 F0 仍必须先冻结。

必须显式编码以下跨服务依赖：

- identity context -> all protected service operations；
- event envelope -> all event producers and consumers；
- Relay measured usage -> Billing settlement；
- Relay embeddings -> Knowledge indexing；
- Tool MCP and Sandbox -> Agent and Workflow execution；
- Agent and Workflow -> Task Scheduler dispatch；
- Billing -> Marketplace checkout、settlement and refund；
- preceding 16 services -> Admin Console operations；
- service contract bundle -> corresponding business frontend；
- generated clients -> Web Platform integration。

Relay 和 Billing 等存在双向运行时交互的服务不能把彼此的 Implementation package 互设依赖。双方的 C package 先冻结 reservation 和 measured-usage 接口，I/H package 只依赖对方已冻结的 contract digest，并在 Domain Queue 做联合运行验证。Admin 和业务前端同样可以在 provider contract 冻结后实现 client；只有其 integration/evidence gate 等待 provider runtime。该规则避免为了表达运行时调用关系而在 package DAG 中制造环。

路径冲突不是静态 DAG 边。Lease Manager 在 ready frontier 上动态串行化冲突路径。

上述集合标签只用于人类阅读。Manifest Compiler 必须将它们展开为具体 packageId 边，并在导入前证明：

- 每个 dependency packageId 存在；
- 没有 self-edge、cycle 或 dangling edge；
- 每个 baseline node 都可从 governance/F0 roots 到达；
- 每个 baseline node 都能到达 service、protected 或 Web integration terminal；
- 所有 672 个 baseline node 都能继续到 Global 和 RC；
- 对完整图执行 deterministic topological sort 和 ready-frontier simulation。

未完全展开的“all services”“preceding services”或“corresponding frontend”等集合表达不得写入运行态 dependency table。

## 12. Path Ownership And Fencing

### 12.1 Lease classes

| Class | Meaning |
|---|---|
| service-exclusive | 一个 capability writer 独占服务内路径 |
| shared-protected | 全局共享路径，只允许 protected Root |
| append-only | 只允许新建已分配命名空间内文件 |
| generator-owned | 仅 deterministic generator 写入 |
| read-only | Agent 可读但不可修改 |

### 12.2 Lease behavior

- 活动 lease 位于 PostgreSQL，不位于 Git；
- 获取 lease 必须使用原子 compare-and-swap transaction；
- lease 默认 TTL 为 120 秒，holder 每 30 秒 heartbeat；连续四次 heartbeat 缺失即过期；
- Root 和 executor 使用相同的 30 秒 heartbeat、120 秒失联阈值；
- lease 使用单调递增 fencing token；
- attempt 持久化完整 lease vector，每一项为 normalized scope、lease ID 和 fencing token；
- commit 使用可重复的 Lease-Fence trailer 记录每个 scope/token，evidence manifest 保存同一有序 vector；
- lease 过期后生成的 diff 即使测试通过也不得入队；
- actual diff 必须是 writeSet 的子集或与 writeSet 相等，不得超出 writeSet；read-only package 允许空 diff；
- symlink、case-folding、path traversal 和 generated-file aliases 必须规范化后再比较；
- shared-protected lease 在同一 domain 同一时刻最多一个 holder。

Lease Manager 从 assignment transaction 起以 attempt-control holder 身份续租，覆盖 worktree_preparing、worktree_ready 和 dispatch claim 阶段。runner 激活 dispatch claim 的同一 CAS transaction 必须重新校验完整 lease vector，并把 holder 移交给 runner。runner 随后在 dispatching 和 running 期间续租。

Agent turn 完成后，Worktree Manager 先冻结 worktree、计算 diff，再在一个 finalization transaction 中校验 lease vector 仍为当前 holder、创建 candidate commit reference 并释放全部 write lease。Review 和 queue 针对不可变 candidate commit 运行，不继续持有写 lease。

Merge 时不要求已释放 lease 仍 active，但必须验证 candidate 的 materializedBaseSha、dependency vector、path policy revision 和 final lease vector 在 finalization 时有效且未被判定 stale。Protected contract、dependency 或 path policy 变化会使 candidate stale；仅 queue head 变化时按 8.4 的 disjoint replay 规则处理。

### 12.3 Shared hotspots

以下路径不得由普通 service package 直接修改：

~~~text
docs/api/openapi.yaml
api/openapi/common/**
api/proto/oblivious/common/**
api/proto/oblivious/events/**
contracts/**
src/server/migrations/microservices/table-ownership.json
src/server/Makefile
src/server/go.mod
src/server/go.sum
src/server/internal/outboundpolicy/**
src/web/src/generated/**
src/web/src/app/router.tsx
src/web/package.json
package.json
pnpm-lock.yaml
.github/CODEOWNERS
.github/workflows/**
docker-compose.yml
deploy/helm/oblivious/values.yaml
deploy/helm/oblivious/values.schema.json
deploy/profiles/**
release/manifests/**
scripts/check.sh
scripts/verify-commercial-completion.sh
scripts/run-target-release-evidence.sh
scripts/run-target-release-evidence-fixtures.sh
~~~

## 13. Worktree And Branch Lifecycle

每个活动 package 使用一个独立 worktree：

~~~text
/srv/oblivious-agents/<run>/<service>/<package>/a<attempt>
~~~

branch 使用：

~~~text
agent/<run>/<package>/a<attempt>
~~~

流程如下：

1. Worktree Manager 从 attempt 固化的 materializedBaseSha 创建 detached sparse worktree，并验证其 declaredBaseSha ancestry；
2. sparse checkout 包含 writeSet、readSet、module manifest、frozen contracts 和验证脚本；
3. worktree setup 只链接共享只读依赖缓存，不复制 secrets；
4. worker Agent 以该 worktree 为 workingDirectory 启动；
5. Agent 只能修改 writeSet；
6. 完成后冻结 worktree，禁止继续写入；
7. Worktree Manager 计算 diff 和 digest；
8. Merge Controller 只 stage writeSet 并创建 candidate commit；
9. merge 或 quarantine 后归档 evidence；
10. 成功 worktree 自动回收，失败 worktree 按 retention policy 保留。

成功 package 在 evidence 和 merge 均固化后最多保留 1 小时。普通失败 worktree 保留 72 小时。Quarantined worktree 保留到 repair package 完成后 7 天。磁盘压力触发清理时只能缩短普通失败 retention，不能删除尚未固化 evidence 的 worktree。

candidate commit 必须包含 Work-Package、Plan-Revision、Attempt、Contract-Digest、Evidence-Manifest 以及每个 scope 一条 Lease-Fence trailer。

Agent 禁止：

- git push；
- rebase；
- merge；
- reset shared refs；
- worktree add/remove/prune；
- 修改另一个 attempt；
- 使用当前原始 checkout 中的未跟踪文件作为依赖。

Go module cache、pnpm store 和下载缓存可共享。Build output、test cache、临时数据库、端口、Docker project name 和 browser profile 必须按 package 或 resource pool 隔离。

## 14. Merge Trains

### 14.1 Queue hierarchy

~~~text
Package Branch
    -> Service Queue, 17 queues
    -> Domain Queue, 4 queues
    -> Global Integration Queue
    -> RC Queue
    -> main and target deployment
~~~

Web Platform 使用独立 Web Queue，在 Global Integration 汇合。

八个 protected domain 各有一条 Protected Queue。C contract artifacts、I/H code candidates 和 E evidence 都归属该 queue，但按下述 package-kind route 处理；queue 完成后直接进入 Global Integration。release-gates domain 的 E package 同时是 RC Queue 的前置依赖。Protected Queue 具有高优先级，但在运行前必须暂停并排空所有受影响 queue。

Queue routing 按 package kind 细化：

- 带 designated RED tests 的 C candidate 先进入 immutable dependency ref，不单独更新绿色 integration head；
- I candidate 包含 C ancestry，通过后进入 Service、Protected 或 Web Queue；
- H candidate 基于已合并 I 进入同一 queue；
- read-only E package 可以使用 materializedBaseSha 作为 candidateSha 而不创建新 commit，其 evidence result 进入对应 integration/evidence gate；
- 只有实际产生代码或 contract tree change 的 candidate 执行 Git ref update。

对 read-only E，package projection 中 queued -> merged 表示 evidence_intent 已原子应用，不表示创建 Git merge commit。evidence_intent 以 package/candidate/evidence digest 唯一，在一个 PostgreSQL transaction 中复核 expected gate revision、写入 applied intent、推进 package 并发布 outbox；重复投递只能观察同一 applied row。若 E job 包含外部副作用，该副作用必须先通过 14.7 的 privileged_operation_intent 完成，evidence_intent 只能引用其 observed result。

### 14.2 Package gate

candidate commit 创建后，Scheduler 自动创建一个 review_job。review_job 不是第五个 baseline package，不计入 672；它是候选提交的运行态 gate：

- 使用与 writer、Root 不同的 gpt-5.5 xhigh 顶层 SDK thread；
- 占用所属 Root 的一个逻辑 worker slot；
- 使用 candidate SHA 创建只读 review checkout；
- 不获取 write lease；
- 输出 pass、changes_required 或 invalid_candidate 及 result digest；
- 通过 stateVersion CAS 绑定唯一 candidate 和 reviewer identity。

changes_required 会拒绝 candidate、创建新的 writer attempt 并重新获取 lease。Reviewer 不得直接修复代码。Verifier job 由非模型的受控 command runner 执行预登记的 deterministic command/environment contract；需要 Agent 解释结果时，可另起 gpt-5.5 xhigh 只读 Verifier Agent，但其判断不能替代 command exit、digest 或 environment evidence。

candidate 创建后 review_job 和 package verifier_jobs 可以并行运行，package 保持 verifying。只有所有 required verifier 通过且 reviewer 返回 pass，package 才进入 reviewed。reviewer changes_required 计为 writer attempt rejection，并参与 maxAttempts；reviewer 自身或 verifier infrastructure failure 只重派对应 job，不消耗 writer maxAttempts。

Package 入队前必须通过：

- package schema 和 plan revision；
- base SHA 和 dependency digest；
- candidate finalization 时有效、且此后未被标记 stale 的 lease vector；
- diff writeSet；
- forbidden path；
- package-kind-specific tests；
- required evidence manifest；
- independent reviewer result。

Contract/RED package 使用专用 red-proof gate：

- 预先声明 designated RED test IDs 和 expected failure fingerprints；
- red-proof command 必须只让 designated tests 以预期 fingerprint 失败；
- 所有既有测试、contract syntax 和非 designated tests 必须通过；
- C candidate 合并到 immutable package dependency ref，不单独进入绿色 Service/Protected/Web integration head；
- I 的 materializedBaseSha 必须包含 C candidate，并把 designated tests 全部转绿；
- 只有包含 C+I 的 candidate 才进入常规 Service、Protected 或 Web Queue。

因此 RED evidence 是 C package 的成功条件，不会把主 integration train 置红，也不能用任意失败冒充预期 RED。

### 14.3 Service gate

Service Queue 必须通过：

- provider/consumer contract；
- tenant isolation；
- database ownership；
- race；
- service component；
- service-specific recovery；
- no legacy fallback；
- service image and deployment template。

### 14.4 Domain and global gates

Domain Queue 运行跨服务 consumer contract 和 golden journeys。Global Queue 运行：

- complete server and web gates；
- generated contract verification；
- commercial golden journeys；
- profile conformance；
- security；
- race/load/soak/chaos；
- backup/restore/upgrade；
- target-evidence collector、manifest、profile matrix 和 digest-binding machinery 的 deterministic dry-run。

Merge Controller 使用 synthetic merge 验证 queue head。merge queue entry 使用独立 stateVersion 和以下状态机：

~~~text
pending -> synthetic_testing -> gate_passed -> intent_pending -> merged
                          -> candidate_failed | retry_wait | stale | blocked
retry_wait -> pending
blocked -> pending
~~~

candidate_failed 必须按 Package queued -> ready/quarantined 处理。queue head 前进但 write paths/digests 仍可安全 replay 时，以新 synthetic commit 回到 pending；dependency、plan 或 policy 变化时 entry 和 package 进入 stale。确定性 gate/runtime 本身故障进入 retry_wait 并使用新 verifier job_attempt；跨 candidate 的真实 platform/gate 缺陷进入 blocked，自动创建 protected gate-repair package，repair reviewed 后回到 pending。红色 train 只暂停自身和后继 queue，不影响无依赖的其他 service queue，也不能在没有上述结构化 disposition 的情况下无限保持 paused。

### 14.5 Exactly-once ref update

Git ref 和 PostgreSQL 无法组成一个原子 transaction，因此每次 merge 或 rollback 都使用 durable merge_intent：

Merge Controller 在生成 proposed commit 前持有 targetRef 对应的 queue advisory lock，确保同一 ref 的 intent 串行。

1. Merge Controller 以固定 tree、parents、author、committer、timestamp 和 message 生成 proposed commit object，并完成 gates；此时尚不更新 ref；
2. PostgreSQL 创建唯一 intent revision，记录 candidate、targetRef、expectedOldRef、已存在的 proposedNewRef 和 operation；
3. 使用 git update-ref 的 old-value compare-and-swap 更新 targetRef；
4. 更新成功后把 intent 标记 applied；
5. crash recovery 比较 targetRef、expectedOldRef 和 proposedNewRef：已是 proposedNewRef 或可证明 current ref 已包含 proposed commit 则补记 applied，仍是 expectedOldRef 且 commit object 存在则安全重试，其他值则进入 conflict -> reconciling；
6. candidate/targetRef/operation/intentRevision unique constraint、同一三元组最多一个 active intent 的 partial unique constraint 和 commit trailers 阻止重复 merge。

intent 创建前 crash 只会留下不可达 Git object，可由 GC 回收；不得在 proposed commit object 尚不存在时持久化 proposedNewRef。

merge_intent 状态为 prepared -> applying -> applied，异常路径为 conflict -> reconciling -> applied | superseded | blocked，blocked 在 ref observation/repair 恢复后回到 reconciling。Reconciler 持有同一 targetRef lock：

1. current ref 是另一条已 applied 的授权 queue head 且 candidate 可按 8.4 disjoint replay 时，将旧 intent 标记 superseded，基于 current ref 生成新 proposed commit 和递增 intentRevision；
2. dependency、plan、contract 或 path policy 已变化时，将旧 intent 标记 superseded 并把 package/entry 标记 stale；
3. current ref 来源未知、非授权或无法证明 ancestry 时保持 blocked，冻结该 ref 的新 update并创建 protected ref-repair package；repair gate 建立唯一 approved head 后，旧 intent 进入 superseded并创建新 revision；
4. synthetic candidate 已包含在 current ref 时补记 applied，不创建第二次 ref update。

因此 conflict intent 本身不可原地改写 expectedOldRef/proposedNewRef；替代 intent 使用新 revision，原 intent 和 conflict evidence 永久保留。

rollback 使用同一 intent 协议。回滚上游 package 时，DAG Engine 按反向拓扑把未合并后继标记 stale，把 merged/integrated/evidenced/done 后继标记 rollback_pending，并在依赖顺序允许时生成对应 rollback 或 forward-fix intent。intent applied 且恢复 gate 通过后，只有 exact prior-state digest 可进入 rolled_back；不同 digest 的不可逆 migration recovery 进入 forward_recovered。

### 14.6 RC candidate and target evidence

Global Queue 通过后先生成不可变 RC candidate digest，而不是立即发布：

1. 为 global SaaS、China SaaS、self-hosted Kubernetes 和 air-gapped 四个 profile 分别选择预授权隔离 validation slot；
2. 将同一 RC candidate image、chart、contract 和 migration digest set 部署到四个 slot；
3. 分别收集四个 profile 的真实 target evidence，air-gapped slot 额外证明 deny-network；
4. 验证 evidence matrix 每一行均绑定同一 RC candidate digest set；
5. RC Queue 密封同一 artifact digest set、每 profile prior-state digest 和 migration/recovery plan digest；
6. production migration 在 promotion 前分类为 no-op、reversible 或 backward-compatible expand；不可逆 contract migration 不得处于本次 promotion critical path，只能在 rollback window 关闭后作为新的受控 package/RC 执行；
7. 使用 profile-scoped privileged intent 执行允许的 migration step，再自动把已验证 artifact promote 到各自预授权生产目标；
8. 任一 participant 失败时停止并 cancelled 尚未 dispatch 的 intent，先 reconcile 所有已 dispatch intent，再进入全 RC compensation；所有 observed_succeeded 或随后观察到 late success 的 profile 都执行 linked recovery intent；
9. 只有四个 profile 的 code、traffic 和 data/schema 均观察到目标 digest/recovery vector 后，RC 才进入 promoted；失败 RC 只有在所有 participant terminal 且四 profile 达到声明 recovery vector 后才能结束；
10. code、traffic 和 data/schema 全部等于 prior-state digest 时终态为 rolled_back；不可逆 migration 只能经预声明 forward-fix 达到 forward_recovered，禁止把兼容但不同的 schema/data digest 标记为 rolled_back。

因此最终 target proof 在 RC seal 前产生，但使用的 artifact 与 seal 后推广到生产的 artifact 完全相同，不存在“先发布未知 RC 再收集证据”的循环。

RC candidate projection 为：

~~~text
draft -> validating -> validated -> sealed -> promoting -> promoted
                                      -> blocked
promoting -> compensating -> rolled_back | forward_recovered
          -> indeterminate -> promoting | compensating
promoted -> compensating
promoted -> superseded
compensating -> indeterminate -> compensating
~~~

四 profile 的 migration、deployment、traffic switch 和 recovery intent 都是同一 RC saga participant，不允许把部分 profile 成功记录为整体 promoted。blocked 在相同 phase prerequisites 恢复后回到原 phase；indeterminate 只有在全部 participant intents 完成 reconciliation 后才能回到 promoting 或进入 compensating。compensation 未通过全 profile recovery barrier 时保持 indeterminate，冻结后续 RC 和对应 production target，不得盲目重派或宣称回滚完成。

当前 active RC 在 promoted 后若 deterministic post-release health/security gate 触发 rollback policy，或收到已授权的 `release rollback` operator command，Release Controller 在一个 transaction 中执行 promoted -> compensating、固定 previous stable RC/recovery vector并创建 linked recovery intents。它复用相同的 four-profile reconciliation、migration recovery 和 stabilization barrier；exact prior digest 进入 rolled_back，其他已批准 forward-fix 进入 forward_recovered。新 RC promoted 并原子更新 release_targets 后，旧 active RC 进入 superseded。该命令是异常恢复触发器，不是正常 promotion 的人工审批。只有当前 active RC 可以直接改变 production target，superseded RC 不可以。

### 14.7 Durable privileged operation intent

真实 provider/payment、validation deployment、production promotion 和 target rollback 每一步都必须先创建 profile/target-scoped privileged_operation_intent。Intent 至少固定：

- run、RC/package、operation type、profile、environment ID 和 target resource；
- desired artifact/request digest 和 expected prior state digest；
- migration plan、allowed recovery mode 和 expected recovery-state digest；
- command template、authorization policy、environment registration 和 credential-reference digest；
- monotonic environment fence、dispatchGeneration、controller ID、claim heartbeat 和 expiry；
- 稳定 external idempotency key、外部 operation/resource ID、observation digest 和 compensation link。

状态机为：

~~~text
prepared -> dispatching -> running -> observed_succeeded | observed_failed
        -> cancelled          -> indeterminate -> reconciling
reconciling -> observed_succeeded | observed_failed | retry_ready | blocked
blocked -> reconciling
retry_ready -> dispatching
observed_succeeded -> compensating -> compensated | indeterminate
~~~

执行协议固定为：

1. Controller 先创建唯一 prepared intent；该步骤只固化期望状态，不授予 target egress，也不作为最终授权检查；
2. Privileged Operation Gateway 是 target/provider/payment 的唯一网络出口。它取得 environment/target session advisory lock 后，在紧邻外呼的 PostgreSQL transaction 中重新读取 current authorization/environment/credential-reference revision 和 preflight expiry，递增 environment fence，创建不可变 privileged_operation_attempt/dispatchGeneration，将 intent 改为 dispatching 并写出 outbox；
3. 上述 transaction commit 是新外部请求的 authorization linearization point。授权撤销使用同一 target lock，只能在线性化前阻止 dispatch，或等待已经授权的 external in-flight request 进入 observed/indeterminate 后生效；Controller/Verifier 自身没有直达 target 的 route，因而不能在检查后延迟绕过 Gateway；
4. Gateway 在持有同一 target lock 和有效 fence lease 时调用预登记命令或 provider API。target deployment 把 intent key、RC digest 和 fence 写入可查询的 annotation/operation metadata，并使用 Kubernetes resourceVersion、provider idempotency key 或等价 CAS；payment/provider test 使用唯一 test identity 和 provider idempotency key；
5. 外部调用返回后先持久化原始 operation ID/response digest，再通过 target read-after-write probe 观察 desired state。只有 desired state observation 可推进 observed_succeeded；observed_failed 必须同时具有 provider-declared terminal failure、按 idempotency/operation ID 查询到的 terminal-negative proof 以及 desired state absent probe。timeout、connection loss、5xx 或未知状态一律进入 indeterminate；
6. Gateway 在外部副作用后、数据库落账前失联时，intent 进入 indeterminate。Reconciler 持有 target recovery lock，通过稳定 idempotency key、operation ID 和 target marker 查询外部状态；已达 desired state 则补记成功；只有 terminal-negative proof 能进入 observed_failed，只有明确证明请求从未被 target 接受时才能进入 retry_ready；其他情况保持 blocked；
7. blocked 只能在 observation capability 恢复后回到 reconciling。retry_ready 创建新的 privileged_operation_attempt 和 dispatchGeneration，但必须复用相同 external idempotency key；prepared intent 若 pinned revision 在 dispatch 前变化则 cancelled，并以 current revision 创建新 intent，禁止原地换绑授权。重复 outbox、旧 generation 或 stale fence 只能读取既有 intent，不能发起副作用；
8. prepared intent 只有在从未 dispatch 时才能 cancelled。RC compensation 必须等待所有 participant 脱离 dispatching/running/indeterminate/reconciling/retry_ready，然后为每个 observed_succeeded 或 reconciliation 期间发现的 late success 创建更高 fence 的 linked recovery intent；
9. compensation 后按 environment policy 指定的 stabilization window 重查四 profile 的 code、traffic、data/schema 和外部 operation terminal state。仍有 late success 或非终态 operation 时重新进入 compensating/indeterminate；全 barrier 满足前不得进入 rolled_back 或 forward_recovered；
10. compensation 使用新的 linked intent 和更高 fence，不能把原 intent 状态倒写为未执行。

不能提供稳定 idempotency key、可查询 operation/resource marker、terminal-negative proof 或声明 recovery contract 的外部系统不具备自动 privileged operation 资格，environment preflight 必须 fail closed。target credential value 只存在于 Privileged Operation Gateway 的隔离 credential broker，不进入 Controller、intent、Agent、日志或 evidence body。

## 15. Failure, Retry And Quarantine

### 15.1 Attempt policy

第一次失败时：

1. 冻结日志、diff、test output、SDK events 和 environment digest；
2. 在一个 transaction 中终止旧 attempt、释放旧租约/resource token并把 package projection 从 leased/running 改回 ready；
3. 保留旧 worktree snapshot；
4. Scheduler 按正常 atomic assignment 重新领取 ready package并创建 a2；
5. Worktree Manager 创建新的 a2 worktree；
6. 分配新的 Codex Agent，并将前一 attempt 的结构化失败证据作为只读输入。

不允许原 Agent 在同一 worktree 中无限修改和重试。

### 15.2 Failure fingerprint

fingerprint 基于：

- failing command；
- exit class；
- normalized error frames；
- failing test IDs；
- signal or timeout；
- dependency digest；
- environment class。

同一 package 的同一 fingerprint 连续出现两次后：

- package 进入 quarantined；
- Root 只读诊断；
- Scheduler 创建 repair package；
- 原 package 不再自动重试；
- 受其依赖的后继保持 blocked。

maxAttempts 默认为 2。第二个 Agent attempt 无论是否产生相同 fingerprint，只要再次出现 Agent-attributable failure，都进入 quarantined：相同 fingerprint 使用 repeated-fingerprint reason，不同 fingerprint 使用 attempts-exhausted reason。stale、authorization_blocked、environment_blocked、Codex rate limit 和已证明的 executor infrastructure failure 不消耗 maxAttempts。

### 15.3 Non-agent failures

- 上游 digest 变化标记 stale，不计 Agent 失败；
- lease 过期标记 stale，不计业务实现失败；
- authorization revision revoked 标记 authorization_blocked 或 stale，不计 Agent 失败；
- API rate limit 使用带 jitter 的指数退避，从 2 秒开始并封顶 120 秒，同时保留原模型；
- executor health 抖动时，只有原 runner process、flock 和 turn stream 均仍存活才能继续原 attempt；runner 已退出时不能 resume in-flight turn，必须进入 indeterminate 并在终止 descendants 后重建 attempt；
- target environment unavailable 标记 environment_blocked；
- deterministic verifier defect 必须创建 verifier repair package，不能降低 gate。

### 15.4 Rollback

普通 package 使用原子 merge commit 回滚。已经执行的不可逆 migration 不使用 Git revert 回滚数据库，只允许预先声明的 forward-fix；这种恢复终态是 forward_recovered，不是 rolled_back，evidence 必须同时保存 prior digest、actual recovery digest 和恢复后的 invariant results。

Global 或 RC 失败时，Release Controller 自动选择：

- revert package merge；
- forward-fix package；
- feature flag disable；
- blue/green traffic restore；
- database restore only when the approved recovery plan permits it。

## 16. Sandbox And Security

### 16.1 Agent sandbox

- writer 使用 workspace-write；
- reviewer 和只读 evidence 使用 read-only；
- Root 使用 read-only；
- danger-full-access 不允许用于普通 Agent；
- Agent tool subprocess 的 network 默认关闭；
- package dependencies 由预热缓存提供；
- secrets 不进入 prompt、worktree、event log 或 evidence body。

Codex Executor transport 自身访问 Codex service 所需的控制面网络不受 worker tool sandbox 的 network policy 影响，但 transport 和 Agent tool 之间必须存在强制 OS isolation：

- transport 使用专用 UID 和 host PID/network namespace；
- Credential Gateway 只绑定 transport-only network，按已登记 executor cgroup/network identity 放行，拒绝 tool cgroup；sentinel 不是认证因子；
- Agent tool 使用不同的无特权 UID、private PID/mount/network namespace 和独立 cgroup；
- tool namespace 只 bind-mount worktree、声明的只读 inputs 和允许的 dependency cache，不挂载 CODEX_HOME、CODEX_SQLITE_HOME、auth path、Credential Gateway endpoint 或 Privileged Operation Gateway endpoint；
- tool 的 /proc 只能看到自身 namespace，不能读取 transport/parent process environ、fd 或 memory；
- sandbox launcher 从固定 allowlist 重建 tool child env，且 network namespace 无 default route；
- cgroup/namespace 设置完成并由 probe 验证前，SDK turn 不得收到第一个 tool request。

不能实现这些读隔离约束的 host 只能运行 fake executor 和文档检查，不能运行真实 Codex worker。不能为了模型通信而给 Agent shell 开放任意外网。

### 16.2 Privileged operations

Docker、Kubernetes、load/soak/chaos、真实 provider、真实 payment rail 和 target deployment 由专用 Verifier 或 Release Controller 创建 job/intent，但只有 Privileged Operation Gateway 可以实际访问 target/provider/payment endpoint。

Verifier 使用预登记的 command template、environment ID/revision 和 credential reference。Agent 可以请求一个 verifier job，但不能读取 credential value 或自由构造生产命令。所有可能产生外部副作用的命令必须由 Privileged Operation Gateway 按 14.7 intent protocol 执行；只读 verifier 也必须通过原子 job claim 复核 authorization/environment revision。Controller、Verifier 和普通 runner 的 network policy 必须拒绝 target/provider/payment destination，从而使 Gateway 成为可验证的唯一 egress。

### 16.3 Defense in depth

writeSet 在三个位置验证：

1. 调度前 package policy；
2. Agent 完成后实际 diff；
3. merge queue synthetic candidate。

控制面还必须拒绝：

- symlink escape；
- path traversal；
- case-insensitive alias collision；
- nested Git repository injection；
- unauthorized executable or hook；
- package 修改自身 acceptance schema；
- package 删除或弱化 failing tests；
- log 或 evidence 中的 secret；
- 失效 fencing token。

## 17. Model And Usage Policy

模型策略固定，不进行成本路由：

| Role | Model | Reasoning |
|---|---|---|
| Root | gpt-5.6-sol | ultra |
| All non-root roles | gpt-5.5 | xhigh |

不设置以下限制：

- per-package token budget；
- per-service credit budget；
- global run credit budget；
- total monetary budget；
- automatic low-cost fallback。

必须记录：

- input/output token；
- cached token when available；
- reasoning and wall time；
- turn and attempt count；
- rate-limit wait；
- model identity；
- Root、service、capability 和 package 聚合。

可靠性限制仍然有效，包括 timeout、heartbeat、maxAttempts、infinite-loop detection 和 quarantine。它们不能以节省费用为理由改变模型。

## 18. Full Automation And Release

### 18.1 No normal human gate

Package、Service、Domain、Global、main、RC、target deployment 和 rollback 均由控制面自动推进。

没有正常流程中的人工批准。Operator 可以观察、pause、drain 或 abort，但系统不能因为等待人工批准而把一个满足全部 gate 的 queue 保持 pending。

### 18.2 Fail-closed prerequisites

全自动不代表自动扩大权限。目标环境必须预先登记：

- environment ID；
- immutable environment revision and authorization policy revision；
- allowed deployment profile；
- allowed commands；
- credential references；
- payment/provider test identity；
- network destinations；
- rollback strategy；
- evidence requirements。

每次登记变更都生成新 immutable revision。撤销 transaction 使用与 Gateway dispatch 相同的 environment/target lock：它立即撤销旧 revision 的新 claim 权，watcher 将旧 ready package 改为 stale/blocked、取消尚未执行的 job claim，并阻止旧 privileged intent dispatch。已经越过 authorization linearization point 的 external in-flight intent 进入 observation/reconciliation；撤销不抹除已授权请求，也不能丢弃审计或盲目重试。

缺少任一项、preflight 过期或当前 revision 不匹配时，package、job 或 release 保持 blocked。Agent 不得创建新授权、读取 secret 或把 sandbox 环境当作 production evidence。

### 18.3 Independent verification

实现 Agent、Reviewer、Verifier、Merge Controller 和 Release Controller 不能是同一个 thread identity。AI review 不是最终真相；确定性 tests、contracts、digests、environment probes 和 release scripts 是发布依据。

所有自动决策写入 append-only scheduler_events，并绑定：

- plan revision；
- base and candidate SHA；
- contract and migration digest；
- lease fencing tokens；
- reviewer and verifier identity；
- evidence manifest digest；
- target environment ID；
- rollback reference。

## 19. Concurrency And Resource Control

writer frontier 只统计同时满足以下条件的 C、I、H 或 Repair package：

- package projection 为 ready；
- writeSet 非空且 leaseClass 不是 read-only；
- exact writeSet 与 frontier 内其他 package 不重叠；
- dependency digest、authorization 和 environment preflight 已满足；
- 所属 Root 有可用 worker slot；
- 对应 Codex、host 和必要 command resource 可分配。

E package、review_job、只读 Verifier Agent 和 deterministic verifier_job 不计入 writer frontier。Manifest Compiler 必须证明的是至少 110 个 schedulable writer packages，而不是从 672 总节点中简单计数。

### 19.1 Ramp stages

| Stage | Concurrent writers | Primary proof |
|---|---:|---|
| S1 | 16 | worktree、lease、fencing、retry 和 event consistency |
| S2 | 32 | multi-service queues、cache isolation 和 executor recovery |
| S3 | 64 | four domain queues、cross-service contracts 和 merge trains |
| S4 | 110+ | real Codex concurrency、global integration 和 target evidence |

Scheduler 打开 epoch 时在 concurrency_epochs 固化 run、stage、policy revision、递增 epochNumber、完整 logical writer slot vector、opened scheduler-event sequence 和 openedAt。每个 slot 在该 epoch 打开后的第一条 eligible assignment 原子取得唯一 primary epoch_slot_sample；数据库以 (epochId, slotId) unique constraint 保证恰好一个。快 slot 在等待慢 slot settled 期间可以继续工作，但其额外 attempt 标记 unwindowed，不属于当前或未来 epoch 的 primary sample，不能改变 promotion 分子/分母。下一 epoch 只有当前 barrier 关闭后才能打开。

非 candidate primary attempt 在到达 retryable_failed、stale、authorization_blocked、environment_blocked 或 aborted 后 settled。candidate primary 只有在同一 candidate 得到结构化 disposition 后才 settled：review/verifier 使 package 进入 reviewed、ready 或 quarantined 时分别记录 reviewed/rejected disposition；对应 package 从 candidate/verifying/reviewed/queued 进入 stale，或因 program abort 进入 aborted 时记录 stale/aborted disposition。单纯到达 candidate/verifying 不计完整样本。quarantined 是 package projection，不是 attempt state；触发 quarantine 的最后一个 attempt 按实际结果计为 rejected candidate 或 Agent-attributable retryable_failed。stale/aborted candidate sample 可以关闭 slot barrier，但不计入 candidate-disposition sample minimum。

全部 slot 的 primary sample settled 后，Scheduler 在一个 transaction 中写入 closed scheduler-event sequence、closedAt、每个 sample disposition digest 和 epoch barrier digest，再把 epoch 标记 complete。最近两个 epoch 始终按最大两个连续 complete epochNumber 选择，不按 wall-clock 猜测；open、partial、superseded epoch 不进入窗口。每级至少需要两个完整 epoch，因此最小 primary samples 分别为 32、64、128 和 220；实际 stage slot vector 大于表中下限时，required primary samples 为两个 epoch 的 slot 数之和。

一个 writer 计为“真实并发运行”必须同时满足：

- SDK 已观察 thread.started 和 turn.started；
- 已观察至少一个该 turn 的 item.started、item.updated 或 item.completed；
- runner process、worktree flock、worker slot 和 write lease 均 active；
- 当前不处于 approval wait 或 rate-limit backoff；
- 尚未观察 turn.completed、turn.failed、error 或 process exit。

S4 必须存在至少 110 个此类 writer 的时间区间交集，并连续保持至少 60 秒。该证据证明 110 个本地 Codex SDK turns 和工具执行环境并发运行；公开 SDK 无法证明 OpenAI 服务端同时进行 110 路模型推理，规范不得把两者混称。

### 19.2 Promotion gates

- unauthorized path modifications: 0；
- accepted stale fencing tokens: 0；
- duplicate merges: 0；
- lost packages or leases: 0；
- lease conflict rate below 1%；
- infrastructure failure rate below 5%；
- candidate-disposition sample count at least the total primary slot count of the two epochs, with stage floors 32/64/128/220；
- accepted-candidate rate at least 80%；
- quarantine rate below 5%；
- unresolved package verifying、job/queue retry_wait、queue/merge-intent blocked/conflict、worker/job/privileged-operation indeterminate、authorization_blocked、environment_blocked 和 aborted states: 0；
- automatic rollback rate below 5%；
- merge queue P95 below 30 minutes；
- merge queue terminal sample count at least the current stage writer count；
- Codex rate-limit failure rate below 2% after backoff；
- CPU 15 分钟平均利用率低于 85%；
- memory working set 低于 80%；
- worktree volume 使用率低于 75%；
- inode 使用率低于 80%。

失败时停止新 assignment，允许 running Agent 到安全点，随后退回上一 stage。

这些默认阈值只能通过新的 versioned concurrency policy 和 plan revision 修改，运行中的 Root 或 Agent 无权调整。

unauthorized path、accepted stale fence、duplicate merge、lost package/lease 和 unresolved-state 等零容忍门禁扫描该 stage 打开以来的全部 primary、unwindowed、job 和 controller events，不能因不属于 epoch window 而排除失败。

百分比和 latency 指标使用当前 stage 最近两个连续 complete epoch 的 closed event-sequence interval 作为窗口。primary-sample 指标只读 epoch_slot_samples；系统事件指标读取 terminal/finished scheduler-event sequence 位于该 interval 的持久化 rows：

- lease conflict rate = result=conflict 的 lease_acquisition_attempts / result in (conflict, acquired) 的 lease_acquisition_attempts；每行必须绑定 finished scheduler-event sequence；
- infrastructure failure rate = classified infrastructure terminal attempts / 全部 started attempts；
- accepted-candidate rate = reviewed candidate attempts / (reviewed candidate attempts + rejected candidate attempts + Agent-attributable retryable_failed attempts)；
- quarantine rate = newly quarantined packages / 全部 started attempts；
- automatic rollback rate = applied rollback intents / applied merge intents；
- Codex rate-limit failure rate = backoff 后仍以 rate-limit 终止的 turns / 全部 started Codex turns；
- merge queue P95 = queue entry 从 queuedAt 到 merged、candidate_failed 或 stale terminalDispositionAt 的 duration P95；retry_wait/blocked 时间保留在 duration 中。

rejected candidate 指同一 candidate 因 changes_required、invalid_candidate 或 deterministic verifier failed 被拒绝；review/verifier infrastructure retry 不进入分母，直到该 candidate 得到 domain disposition。分类器版本和分母必须写入 promotion evidence。样本未达到本节最小 settled terminal attempts 时不得晋级。

candidate-disposition sample 只包括 reviewed candidate、rejected candidate 和 Agent-attributable retryable_failed；stale、authorization/environment blocked、aborted 或 infrastructure failure 不能填充该最小样本。accepted-candidate rate 分母为零或低于两个 epoch 的 total primary slot count 时门禁失败，Scheduler 必须继续在原 stage 收集有效样本。窗口 close sequence 之前创建的 queue entry 必须在 promotion evaluation 前全部得到 terminal disposition；merge queue terminal sample 低于当前 stage writer count、P95 为空或任一必需 row 缺少 event-sequence binding 时同样失败。任何其他百分比指标若分母为零也按 fail closed 处理，除非 versioned policy 显式定义该指标在“无适用事件”时为 not-applicable。

### 19.3 Resource pools

writer thread 与本地命令资源分开计数。初始资源池为：

| Resource | Initial maximum |
|---|---:|
| Codex writer turns | stage dependent, up to 110+ |
| Static/unit commands | 64 |
| PostgreSQL-backed tests | 24 |
| Browser tests | 8 |
| Docker/Kubernetes tests | 4 |
| Load/soak/chaos | 1 |
| Shared protected writer | 1 per protected domain, 4 globally |

资源上限由真实基准调整，但不得绕过 stage promotion。

### 19.4 Host prerequisite

目标单机基线为：

- 64 physical cores or equivalent 128 hardware threads；
- 256 GiB ECC memory；
- 4 TB high-endurance NVMe worktree and cache volume；
- sufficient network and Codex account/runtime concurrency；
- PostgreSQL on native Linux filesystem。

当前 16-thread、7.5-GiB、fuseblk 工作环境不满足 S4。实现可以在当前机器完成 S1 功能验证，但不得在当前硬件上宣称 110-writer capacity proven。

## 20. Operator Interfaces

### 20.1 CLI/TUI

CLI 提供：

~~~text
program validate|import|start|pause|drain|abort|status
package inspect|retry|quarantine|supersede
root start|stop|resume
queue status|pause|resume
worktree list|inspect|archive|gc
evidence inspect|verify
release status|promote|rollback
~~~

CLI/TUI 只调用 Orchestrator API，不直接写 PostgreSQL。

### 20.2 Local Dashboard

Dashboard 包含：

| View | Content |
|---|---|
| Overview | program progress、stage、health、queue depth 和 resource pressure |
| DAG | package dependency graph、ready frontier 和 stale propagation |
| Agents | Root、worker、thread、turn、model、reasoning 和 heartbeat |
| Worktrees | path、branch、attempt、lease、disk、diff 和 retention |
| Queues | Package、Service、Domain、Web、Global 和 RC train |
| Failures | fingerprint、attempt history、quarantine 和 repair package |
| Evidence | commands、exit codes、logs、SHA 和 artifact digests |
| Usage | token、duration、rate limit 和 model aggregation |
| Release | target gates、deployment、rollback 和 immutable audit |

Dashboard 只绑定 127.0.0.1。所有 mutation 使用本地 authenticated session、CSRF protection 和 explicit confirmation for pause、abort and rollback controls。Confirmation 是操作安全措施，不是正常 merge/release 的人工 gate。

Dashboard 使用独立的 React/TypeScript local SPA，由 Orchestrator API 提供静态资源和 API；它不进入 Oblivious 产品 Web bundle，也不复用产品 session、tenant 或 admin 权限。

## 21. Observability

控制面必须暴露：

- OpenTelemetry traces；
- structured logs with run/package/attempt/thread IDs；
- scheduler latency；
- ready frontier size；
- lease acquisition conflicts；
- heartbeat age；
- Codex thread state；
- queue wait and execution duration；
- worktree disk usage；
- gate pass/fail；
- retry、quarantine、rollback；
- token and rate-limit statistics。

每个 package 的 trace 必须能从 definition 一直关联到 candidate、merge、evidence 和 release。

## 22. Testing Strategy

### 22.1 Deterministic control-plane tests

- package and program schema；
- DAG cycle and missing dependency detection；
- artifact digest invalidation；
- PostgreSQL assignment transaction；
- committed negative lease acquisition preserves conflict audit without partial lease/attempt；
- concurrent lease acquisition；
- fencing token rejection；
- heartbeat expiry；
- retry and quarantine；
- review/verifier/assembly job_attempt generation、infrastructure retry 和 indeterminate reconciliation；
- verifier failed 到 writer retry/quarantine 的 package transition；
- candidate 到 done 全阶段的 stale/rollback propagation；
- plan revision change makes ready/running/candidate/queued packages stale；
- authorization/environment revision change at ready、claim、dispatch and external-call boundaries；
- privileged operation intent crash windows、duplicate delivery 和 stale generation rejection；
- four-profile RC partial-promotion compensation；
- promoted active RC can enter linked post-release compensation；
- external timeout stays indeterminate until terminal-negative proof, and late success re-enters compensation；
- reversible/expand/irreversible migration recovery produces rolled_back versus forward_recovered correctly；
- F1 ref conflict creates repair/supersede job and cannot overwrite current ref；
- synthetic queue gate candidate failure retries/quarantines, and merge-intent ref conflict supersedes through repair/replay；
- all-rejected candidates and unresolved verifying jobs cannot satisfy stage promotion；
- all-stale/zero-denominator samples cannot satisfy stage promotion；
- epoch primary-ticket uniqueness、fast-slot unwindowed attempts 和 consecutive barrier-window reconstruction；
- worktree sparse checkout and cleanup；
- path normalization and symlink escape；
- candidate commit trailers；
- merge train ordering and rollback；
- outbox and event projection idempotency。

### 22.2 Codex executor tests

使用 fake SDK 测试：

- Root start/resume/stop；
- worker assignment；
- structured output validation；
- observed event sequence 和 indeterminate reconciliation；
- executor crash；
- thread timeout；
- cancellation；
- rate-limit backoff；
- forbidden model downgrade。

真实 Codex smoke 从 1 Root、2 worker 开始，再进入 16/32/64/110 stage。

### 22.3 Load and failure tests

在进入真实 110-writer 运行前，fake executor 必须模拟：

- 18 Root；
- 144 worker；
- 672-package DAG；
- 160 simultaneous worktrees；
- concurrent heartbeat；
- executor crash storm；
- PostgreSQL restart；
- queue gate failure；
- disk pressure；
- event duplication and reordering。

### 22.4 Security tests

- worker 写入 writeSet 外路径；
- worker 修改 gate 或 lockfile；
- symlink and case-folding escape；
- secret in log/evidence；
- tool child getenv、auth-path read、parent /proc read 和 Credential Gateway access 均被拒绝；
- stale commit replay；
- forged fencing token；
- unauthorized verifier environment；
- revoked environment revision cannot dispatch queued verifier/release intent；
- revocation and Operation Gateway dispatch serialize on the same target lock；
- Controller/Verifier direct target egress is denied；
- Dashboard CSRF；
- local API authentication bypass。

## 23. Bootstrap And Migration

控制面不能使用尚未完成的自己来实现自己。实施顺序固定为：

### B0 Design and plan

- 提交本规范；
- 创建 master implementation plan 和分模块 subplans；
- 固定 bootstrap branch 和当前未跟踪文件边界。

B0 的第一个 governance package 必须在五份旧计划头部添加 orchestration execution superseded marker，并链接本规范和新 master plan；不得删除仍有效的产品 requirement。program.yaml 的 baseSha 必须等于包含本规范的已提交 Git SHA，并由 bootstrap verifier 从 Git 读取，禁止继续使用旧计划中的固定 3531446。

本设计提交时的未跟踪边界为：

~~~text
scripts/run-target-release-evidence-fixtures.sh
scripts/run-target-release-evidence.sh
src/server/internal/outboundpolicy/
~~~

Bootstrap worker 不得 stage、删除、移动或把这些路径作为自身实现证据。它们将来若由其原所有者正式提交，必须通过新的 base SHA 和 plan revision 显式进入编排。

B0 必须生成 .planning/orchestration/preexisting-untracked.json。每个文件记录 canonical SHA-256；目录记录按相对路径排序后的 file digest tree；同时记录 ownerRef、snapshotAt、resolution 为 commit-with-owner-package 或 exclude-from-program，以及 resolveBeforePhase = B4。

任何 snapshot digest 漂移都会阻止 bootstrap，并要求新的 plan revision。B4 开始前，每个条目必须已由原所有者正式提交并进入明确 protected/service ownership，或被验证为不进入 program 的外部路径；未解决条目会阻止 B4/B5，而不是仅显示 warning。

### B1 Minimal control plane

使用当前受控小并发完成：

- TypeScript package；
- PostgreSQL schema；
- package validator；
- Scheduler；
- Lease Manager；
- Worktree Manager；
- fake Codex executor；
- CLI status and pause。

### B2 Codex and merge integration

- 接入 @openai/codex-sdk；
- 建立 18 executor process supervisor；
- 实现 candidate commit；
- 实现 Service/Domain/Global merge train；
- 实现 evidence projector。

### B3 Manifest compiler

- 导入 17-service ownership；
- 建立 160 business capability cells；
- 建立 8 protected domains；
- 实现约 672 package definitions 的 deterministic compiler；
- 对 planned target paths 生成 draft graph；
- 在不调度 package 的前提下验证 schema、DAG 和 planned writeSet；
- 生成 draft read-only status snapshot。

### B4 Product scaffold

在大量 service Agent 启动前：

- 冻结 shared contracts；
- 将当前扁平 Go package 重组为 capability-owned subtrees；
- 将 service OpenAPI 和 Proto 拆为 capability fragments；
- 建立 deterministic bundle/generator；
- 建立 service test and evidence roots。

B4 本身使用独立 bootstrap catalog，不计入 672 baseline packages：

- 17 个 service scaffold packages；
- 8 个 protected-domain scaffold packages；
- 1 个 Web Platform scaffold package。

这 26 个 package 必须各自声明 owner、writeSet、forbiddenSet、前驱、behavior-parity tests 和 rollback commit。执行顺序为 protected contract/generator roots、service directory skeletons、Web generated-client consumption，当前 bootstrap 并发最多 6 个 writer。

Scaffold package 只允许机械移动、compatibility shim、目录骨架和 generator 接线，不得顺带实现业务能力。退出门禁包括旧行为测试继续通过、目标 import boundary 可编译、每个 target path 唯一 owner、旧 flat path 没有新的生产引用、contract bundle deterministic。

Scaffold 合并后，以新的 immutable base SHA 和 frozen digests 重新运行 Manifest Compiler，生成最终约 672 个 package definitions，验证实际 writeSet 不重叠，并证明 ready frontier 可达到 110。

没有 B4 和 scaffold 后的最终重新编译，多个 Agent 仍会争用同一扁平目录，因此不得进入 S3 或 S4。

### B5 Ramp and delivery

- S1 16 writers；
- S2 32 writers；
- S3 64 writers；
- S4 110+ writers；
- full service/domain/global/RC automation。

## 24. Repository Artifacts

实施完成后至少包含：

~~~text
tools/codex-orchestrator/
  package.json
  src/
  migrations/
  schemas/
    agent-result.schema.json
  prompts/
    root.md
    contract.md
    implementation.md
    hardening.md
    evidence.md
    reviewer.md
    verifier.md
  profiles/
    root.yaml
    worker.yaml
  tests/
  dashboard/

.planning/orchestration/
  program.yaml
  clusters.yaml
  dag.yaml
  path-policy.yaml
  concurrency.yaml
  merge-policy.yaml
  gates.yaml
  migration-map.yaml
  preexisting-untracked.json
  bootstrap-packages/
  packages/
  schemas/
  templates/
  generated/status.md

docs/runbooks/
  codex-orchestrator.md
  codex-orchestrator-recovery.md
~~~

## 25. Definitions Of Done

本设计只有在以下条件全部满足时才算实现完成：

1. 168 capability cells 和约 672 baseline packages 通过 schema 和 DAG 校验。
2. writeSet 冲突审计证明 110 个 writer 的 ready frontier 存在。
3. PostgreSQL assignment、lease 和 fencing 在并发测试中无 lost update；冲突领取持久化 committed negative audit 且无 partial lease/attempt。
4. 18 个 Root executor 可在已完成 turn 之间恢复 Root session；in-flight worker crash 会可靠进入 indeterminate 并创建新 attempt。
5. 110 个真实 non-root Codex turn 同时处于 running，且分别绑定唯一 package、attempt、worktree 和有效 lease。
6. 所有 Root 使用 gpt-5.6-sol ultra，所有非 Root 使用 gpt-5.5 xhigh。
7. 没有自动模型降级、费用预算暂停或递归 worker fan-out。
8. Package、Service、Domain、Global 和 RC merge train 可自动推进；四 profile promotion 及 promoted 后 rollback 使用 durable intent saga，并能从每个 crash window 自动 reconcile、全量补偿或按 migration recovery contract 进入 forward_recovered。
9. path violation、accepted stale fence、duplicate merge 和 lost package 均为零。
10. CLI/TUI 和 Dashboard 显示同一 PostgreSQL state。
11. fake 144-worker load、security、crash recovery 和 real staged ramp 全部通过。
12. 当前原始 checkout 的未跟踪文件未被 bootstrap 或 package worker 意外纳入。
13. repository-local evidence 以及 global SaaS、China SaaS、self-hosted Kubernetes、air-gapped 四个 profile 的 target evidence 均绑定同一 RC digest set。
14. 最终 no-skip commercial verifier 在预授权 target environment 中退出 0。
15. 上游 Codex credential、CODEX_HOME、parent process 和 Credential Gateway 对 Agent tool subprocess 的读取/连接 probe 全部失败。
16. authorization/environment revision 在 assignment、job claim、dispatch 和 privileged operation 前均被原子复核，旧 generation/fence 的副作用执行数为零。
17. RC 只有在四 profile code、traffic、data/schema 和外部 operation 均通过稳定期 barrier 后才能进入 promoted、rolled_back 或 forward_recovered，部分发布与模糊失败不能成为成功终态。
18. concurrency epoch 的 slot tickets、barrier sequence 和 metric rows 可从 PostgreSQL 重建，fast-slot extra attempts 不改变 promotion window。
19. synthetic gate rejection 和 merge-intent ref conflict 都能自动进入 retry、stale、quarantine、repair/replay 或 superseded，任何 queue 不会只有 paused 而没有状态出口。

## 26. Non-goals

本规范不要求：

- 在当前 7.5-GiB 主机上证明 110-writer capacity；
- 自建或本地运行大模型；
- 支持 RuFlo、Claude Flow 或非 Codex Agent runtime；
- 允许 Agent 自由创建任务或扩大权限；
- 用 AI reviewer 代替 deterministic tests；
- 把运行态 heartbeat、lease 或 queue 写入 Git；
- 取消已有 17-service 产品架构；
- 通过跳过 target evidence 提前宣称商业发布完成。

## 27. Primary Risks And Controls

| Risk | Control |
|---|---|
| Codex account/runtime 不提供 110 slots | S4 preflight 真实启动并发 turn；不足时不宣称完成 |
| xhigh Agent 消耗极高 | 仅记录统计；保留 rate-limit backoff 和 loop protection，不自动降级 |
| Agent 修改相同文件 | capability subtree、atomic lease、fencing 和 diff gate |
| 共享文件成为瓶颈 | protected Root、generator-owned output 和 queue drain |
| 扁平旧代码无法隔离 | B4 scaffold 是 S3/S4 硬门禁 |
| 上游变化使证据失真 | artifact digest 和 automatic stale propagation |
| executor crash 丢失状态 | supervised process group、flock、indeterminate reconciliation 和新 attempt |
| 自动发布扩大风险 | preauthorized revision、profile-scoped durable intent、external idempotency、four-profile compensation 和 immutable audit |
| Agent 读取 Codex/target credential | transport/tool 独立 UID 与 PID/mount/network namespace、stripped child env 和 denial probes |
| 单机资源耗尽 | resource pools、stage ramp、NVMe worktrees 和 pressure-based pause |
| 多重计划互相矛盾 | 本规范作为新编排边界，实施时同步更新旧 writer/approval 规则 |

## 28. Final Decision

Oblivious 的大规模完成工作采用 Codex-native、TypeScript SDK 驱动、PostgreSQL 协调、能力纵切、独立稀疏 worktree、分层 merge train 的单机联邦编排。

目标运行形态是 18 个逻辑 Root、110 至 144 个独立 worker thread，其中至少 110 个 writer 可真实同时执行。系统以确定性门禁、证据摘要和自动回滚替代人工合并与发布审批，并在缺少授权、证据或运行能力时严格 fail closed。
