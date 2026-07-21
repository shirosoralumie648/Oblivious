# Roadmap: Oblivious Commercial Complete & Target Release

## Overview

本路线从当前 brownfield E1/E2 基线出发，先固定发布合同、可信身份和共享安全边界，再完成耐久执行、Relay 权威主链与真实客户旅程，随后关闭资金、观测、恢复和声明部署模式的能力对等，最终以同一 release commit 和 artifact digest 的 E3/E4 no-skip 证据完成商业发布。阶段按依赖关系组织，但每阶段都必须交付可由用户或运营者观察和验证的完整能力。

**Historical evidence boundary:** `.planning/phases/01-*` 至 `30-*` 是已保留的历史 milestone E1/E2 证据；本 milestone 从历史 Phase 30 后连续编号为 Phase 31-39，旧 summary 和 verification 不计入当前进度。

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): planned milestone work
- Decimal phases (2.1, 2.2): urgent insertions marked `INSERTED`

- [x] **Phase 31: 发布合同与可信构建身份** - 固定能力承诺、显式 profile、clean source identity 和统一报告协议。 (completed 2026-07-17)
- [x] **Phase 31.1: 动态 Readiness 与持续 Fail-Closed** (INSERTED) - 用进程内动态授权源约束所有新副作用。 (completed 2026-07-21)
- [ ] **Phase 31.2: 契约表面一致性与聚合门禁** (INSERTED) - 让所有 canonical surface 与 runtime 双向一致并阻断漂移。
- [ ] **Phase 32: 身份、租户与共享出站安全** - 建立可信组织边界和统一 fail-closed 集成安全。
- [ ] **Phase 33: 耐久执行、RAG Worker 与共享对象** - 让知识和自动化任务可恢复、可重放且不依赖本地状态。
- [ ] **Phase 34: Relay、Chat 与证据主链** - 证明唯一 Provider/usage 权威及完整流式 Chat 生命周期。
- [ ] **Phase 35: 真实客户、Builder、Admin 与渠道旅程** - 用真实全栈路径交付工作区、治理和至少一个渠道生命周期。
- [ ] **Phase 36: 财务与 Marketplace 对账闭环** - 关闭支付、账本、交易、结算、payout 和异常治理。
- [ ] **Phase 37: 持久观测、SLO 与全状态恢复** - 让运营者可联查、告警并恢复所有声明状态。
- [ ] **Phase 38: 声明部署模式能力对等** - 仅保留通过同一能力和租户验证的部署形态。
- [ ] **Phase 39: 供应链与目标商业发布** - 生成不可变制品并取得同 commit、同 digest 的 E3/E4 no-skip 证据。

## Phase Details

### Phase 31: 发布合同与可信构建身份

**Goal**: 发布运营者可以从唯一 authored contract 和可信 clean-source identity 准确确认本版本承诺的 capability、deployment profile 与报告身份。
**Depends on**: Nothing (first phase)
**Requirements**: RELS-01
**Success Criteria** (what must be TRUE):

  1. schema-validated authored contract 以领域和 lifecycle slice 描述 commitment、profile、dependencies、state stores、typed operations、catalog bindings 与 readiness requirements，且不 author 自引用 commit/digest。
  2. clean Git commit/tree 与 canonical contract digest 形成不可由 caller 覆盖的 BuildIdentityV1，并可注入 binary 与 OCI label；dirty 或 identity mismatch 明确 fail closed。
  3. 所有后续 surface producer 共用嵌套 SurfaceReportV1、trusted identity resolver 和原子输出合同，environment/mode 不会混入 drift 或 skipped checks。
  4. monolith 是唯一 committed/default profile；microservices、dual、split 具有 profile-bound、无副作用且稳定失败的 migrate/deploy/rollback refs，不会被资产存在隐式晋级。

**Plans**: 7/7 plans complete

Plans:
**Wave 1**

- [x] 31-01-PLAN.md - Establish the strict authored capability/deployment authority and semantic loader.

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 31-02-PLAN.md - Add canonical contract digest, non-vacuous Go test selection, and safe profile operations.

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 31-03-PLAN.md - Derive trusted clean-source BuildIdentityV1 and expose foundation CLI primitives.

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 31-04-PLAN.md - Create nested SurfaceReportV1, typed details registry, build-identity registration, and atomic writer.
- [x] 31-05-PLAN.md - Bind one identity to active binaries, OCI labels, and packaged contract.

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 31-06-PLAN.md - Emit the foundation build report and enforce Stage A plus post-commit clean HEAD Stage B gates.

**Wave 6** *(gap closure; blocked on verification of Plans 31-01 through 31-06)*

- [x] 31-07-PLAN.md - Reconcile deployment-profile timing fields, repair structural proof metadata, and rerun exact clean-HEAD verification.

**Design**: `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md`

**Verification**: `passed` on 2026-07-17 for the E1/E2 repository-local foundation. Fresh Stage A, full Go regression, docs aggregate, 42/42 structural checks, clean review, Nyquist validation, security review, and the zero-skip pre-closeout Stage B tuple pass. Push eligibility additionally requires a zero-skip Stage B run at the exact final clean tracking HEAD with no later commit. See `31-VERIFICATION.md`; `RELS-01` remains pending and no target/live or commercial-readiness claim is made.

### Phase 31.1: 动态 Readiness 与持续 Fail-Closed (INSERTED)

**Goal**: 当前 runtime 使用一个进程内动态 ReadinessManager 持续计算 availability，并在所有新副作用前 fail closed。
**Depends on**: Phase 31
**Requirements**: RELS-01
**Success Criteria** (what must be TRUE):

  1. runtime 按显式 profile 和 trusted build identity 完成 DB ping、migration、bounded bootstrap probes，再发布 generation 1 并周期刷新；旧审计文件不能成为授权源。
  2. 30 秒 refresh、120 秒 max age 与 30 秒 future skew 由 authored profile 固定，单一 Go evaluator 处理 identity、freshness 和 capability verdict。
  3. HTTP/gRPC、worker claim/effect、Provider/model/tool/channel 与财务 dispatch 在每次新副作用前调用同一 guard；expired、disabled、blocked 或 unknown 状态零调用拒绝。
  4. Admin full inventory、app-safe projection、/livez、/readyz、audit export、Docker Compose 与 canonical Kubernetes workload 消费同一 manager 和 identity。
  5. server-derived catalogBindings 为 model/tool 返回只读 capabilityId，并在 mutation 与 execution 重新解析授权，UI 不能成为唯一防线。

**Plans**: 21/22 plans executed

**Original planning envelope exception:** checker-requested closure required complete first-call coverage for 12 real standalone roots plus the committed monolith direct-Tavily/configured-Chain/builtin web-search path. Those disjoint owners and dynamic zero-call suites did not fit the original 6-8 target while retaining the nine-file hard cap, so the executed decomposition used ten focused plans without shrinking coverage or trusting executable/environment identity.

**Gap-closure decomposition:** canonical verification found only 1/5 must-haves verified and the code review identified production fail-closed gaps. Plans 11-22 close those findings in three additional waves while preserving the original 01-10 execution history.

Plans:
**Wave 1** *(blocked on Phase 31 Plan 31-06 completion)*

- [x] 31.1-01-PLAN.md - Establish evaluator/manager, immutable RuntimeAuthorities, authored entrypoint identity, RouterOptions, and the pure readiness report constructor.

**Wave 2** *(blocked on Wave 1 completion; consumer packages are file-disjoint and may run in parallel)*

- [x] 31.1-04-PLAN.md - Guard schedule, channel, batch, and archive worker boundaries and own the workflow ToolExecutor injection seam.
- [x] 31.1-05-PLAN.md - Guard Relay/Chat attempts and deliver exact server-derived `ModelOption.capabilityId` response/mutation contracts.
- [x] 31.1-06-PLAN.md - Guard MCP, the independent Agent tools Registry API, strict mutation DTOs, and the generic configured web-search Chain without depending on the later expected manifest.
- [x] 31.1-07-PLAN.md - Guard checkout, Admin refund, payout, and settlement while preserving signed reconciliation-only completion.

**Wave 3** *(blocked on Plans 01, 04 and 06)*

- [x] 31.1-10-PLAN.md - Build authority-required web-search/live ToolExecutor paths and exact read-only `ToolDefinition.capabilityId` frontend normalization/mutation omission.

**Wave 4** *(blocked on all Wave 2 consumers and Plan 10; sole server.go composition owner)*

- [x] 31.1-02-PLAN.md - Resolve immutable preflight inputs once, wire group-one roots, compose all completed consumers, and enforce listener-before-background lifecycle.

**Wave 5** *(blocked on Wave 4 shared preflight)*

- [x] 31.1-09-PLAN.md - Apply the same immutable source-authored entrypoint handoff to the remaining eight standalone composition roots.

**Wave 6** *(blocked on all runtime consumers and standalone roots)*

- [x] 31.1-03-PLAN.md - Expose control-plane views, exact-join effect coverage, and compile distinct typed readiness/deployment report CLI producers.

**Wave 7** *(blocked on Wave 6 completion)*

- [x] 31.1-08-PLAN.md - Wire canonical deployment assets and prove standalone-build plus aggregate-consume immutable-image readiness/deployment harness modes.

**Gap Wave 1** *(blocked on canonical verification of Plans 01-10; file-disjoint gap owners may run in parallel)*

- [x] 31.1-11-PLAN.md - Serialize generation-one bootstrap and bound context-ignoring probe work without weakening fail-closed observations.
- [x] 31.1-12-PLAN.md - Authorize the current server-owned model before Relay semantic-cache success and retain immediate provider guards.
- [x] 31.1-13-PLAN.md - Require strict Admin and Marketplace readiness composition and deny mutations before reads, probes, writes, or provider calls.
- [x] 31.1-14-PLAN.md - Construct the authority-aware Agent web-search provider before the authorized ToolExecutor and prove exact ID-owner pairs.
- [x] 31.1-15-PLAN.md - Make compatibility ToolExecutor construction deny-only and migrate behavior fixtures to explicit authority-backed construction.
- [x] 31.1-16-PLAN.md - Fail stale or unavailable Admin/app readiness projections closed at the exact evaluator freshness boundary.
- [x] 31.1-17-PLAN.md - Replace harness-only dependency probes with typed deployment-owned DB, Redis, Qdrant, ClickHouse, and Kafka probes.
- [x] 31.1-18-PLAN.md - Complete the structural effect descriptor allowlist, mapping, manifest, authority join, and mutation proof.
- [x] 31.1-21-PLAN.md - Require Chat mutation decoding to reach EOF after exactly one JSON document before any business effect.

**Gap Wave 2** *(blocked on the named Gap Wave 1 owners)*

- [x] 31.1-19-PLAN.md - Exact-join the strict real BuildRuntime descriptor snapshot from the checked-in phase gate after Plans 13, 14, 15, 17, and 18.
- [x] 31.1-22-PLAN.md - Apply and wait for canonical Kafka before the Kubernetes server rollout and `/readyz` proof after Plan 17.

**Gap Wave 3** *(blocked on all Gap Waves 1-2 plans)*

- [x] 31.1-20-PLAN.md - Re-audit code, security, Nyquist, docs, and repository-local gates before handing final tracking-HEAD Stage B and push to a hard manual post-closeout gate.

Cross-cutting constraints:

- Every authorization verdict comes from the current in-process `ReadinessManager`; audit/report files are write-only runtime evidence.
- Every new effect rechecks the current generation through the shared guard, and all runtime descriptors must exact-join the independent expected inventory.
- Every process supplies a source-authored entrypoint ID; generic profile resolution, environment, argv and executable name cannot authorize standalone startup.
- `RunEntrypoint` resolves one complete validated contract, resolver-returned profile and trusted identity into immutable `ResolvedEntrypointInputs`; callbacks cannot reload or replace them through environment, globals, cwd or disk.
- Consumer packages complete against Plan 01 authority types before Plan 02 constructs one immutable `RuntimeAuthorities{CatalogAuthorizer,CapabilityBindings}` value and hands it to router, workers, Provider/tool/web-search, live ToolExecutor and finance consumers.
- Chat response JSON/TypeScript `ModelOption` and live Agent `ToolDefinition`/`AgentToolDefinition` expose exact server-derived read-only `capabilityId`; strict server mutations reject it and frontend mutations omit it.
- Plan 06 descriptor tests are manifest-independent; Plan 03 alone creates `readiness-effect-surface.v1.json` and joins expected/static/runtime sets after every producer is complete.
- Plan 03 owns `NewDeploymentReport` plus `report-readiness`/`report-deployment`; Plan 08 separately persists both reports and its aggregate mode consumes the Phase 31 foundation image bundle/tag/digest without rebuilding.
- Authored monolith timing remains exactly 30s refresh, 120s max age, and 30s future skew with no runtime fallback.
- Effect coverage uses exact descriptor ID-owner pairs, structural guard-to-effect discovery, the frozen manifest, and valid `RuntimeAuthorities`; source text or synthetic registries cannot certify production composition.
- Readiness mutation guards run before reads, probes, local writes, accounting, or provider effects, and denied paths prove zero downstream deltas.
- OpenAPI Envelope parity remains Phase 31.2, Relay Files and multi-provider batch identity remain Phase 34, and MCP Connect lifecycle remains Phase 35.
- Plan 20 owns pre-closeout repository-local audit only. Final Stage B and push remain a hard external/manual gate after every SUMMARY, canonical verifier, phase tracker, and closeout commit has written the final clean tracking HEAD.
- Phase completion is capped at E1/E2 repository-local evidence; non-empty skips and unavailable Docker/DB fail the committed gate.

**Design**: `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md`

### Phase 31.2: 契约表面一致性与聚合门禁 (INSERTED)

**Goal**: OpenAPI、runtime routes、frontend transports、protobuf、migration 与产品呈现对同一 contract identity 双向一致，任何 drift 或 committed skip 都阻断发布。
**Depends on**: Phase 31.1
**Requirements**: RELS-02
**Success Criteria** (what must be TRUE):

  1. `/api/*`、`/v1/*` 与其余明确 disposition 的 public operations 在 OpenAPI、same-source runtime registrar 与 derived compare inputs 上双向一致。
  2. TypeScript AST inventory 覆盖全部 production HttpClient、fetch/SWR、upload、SSE/streamText、EventSource 与 WebSocket caller，并验证 request encoder 和 response decoder；text/markdown 不再走 JSON decoder。
  3. pinned protoc/tool plugins 在 CI 可复现安装，所有 canonical proto source 与 generated output 唯一归属并可确定性再生。
  4. numbered SQL/checksum、runtime ledger 与 committed monolith replay 分别产生 typed evidence；无 DB/Docker 不能被计为通过。
  5. verify-quality-gates.sh 是唯一 direct aggregate owner，聚合 trusted build/readiness/surface reports，拒绝 identity splice、drift、skip、重复 surface 和敏感公开输出。

**Plans**: 3/15 plans executed
**Progress**: 1/15 plans complete

Plans:
**Wave 1** *(blocked on Phase 31.1 Plan 31.1-08 completion; file-disjoint foundations may run in parallel)*

- [x] 31.2-01-PLAN.md - Close public HTTP scope, emit typed projections, and provide the narrow exact-symbol caller preflight.
- [x] 31.2-06-PLAN.md - Pin protobuf tools, disposition every tracked source/output, and prove temp-only byte-clean regeneration.
- [x] 31.2-07-PLAN.md - Produce shared MigrationIdentity rows/digest plus separate static metadata and ledger reports.

**Wave 2** *(blocked on the corresponding Wave 1 foundations; file-disjoint and parallel)*

- [ ] 31.2-02-PLAN.md - Make Go runtime registrations the same source for dispatch/descriptors and compare them to OpenAPI.
- [ ] 31.2-04-PLAN.md - Define shared operation-aware HttpClient/stream/upload/SWR contracts and migrate Chat/transport owners.
- [ ] 31.2-08-PLAN.md - Run one fresh migration session deriving apply/no-op from ledger snapshots and emitting three reports once.

**Wave 3** *(blocked on Plans 01, 02 and 04; caller groups and Group A route migration are file-disjoint and parallel)*

- [ ] 31.2-11-PLAN.md - Close the fixed nine-file Group A expected-owner set with exact generated symbols or compiler-proven non-caller disposition.
- [ ] 31.2-12-PLAN.md - Migrate the fixed eight-file Group B production caller owner set to exact generated operation symbols.
- [ ] 31.2-13-PLAN.md - Migrate the first eight Go route helper owners to same-source runtime registration.

**Wave 4** *(blocked on both caller migration groups and Group A route migration)*

- [ ] 31.2-10-PLAN.md - Wire authenticated projection API/state/provider into router/nav/marketing and Chat/Agents selectors.
- [ ] 31.2-14-PLAN.md - Migrate the remaining eight Go route helper owners and prove Group B dispatch/media parity.

**Wave 5** *(blocked on the route groups and caller foundations)*

- [ ] 31.2-15-PLAN.md - Complete the HTTP runtime producer and migrate the two public marketing auth callers.

**Wave 6** *(blocked on complete caller and route ownership closure)*

- [ ] 31.2-03-PLAN.md - Build the one-Program production sidecar, enforce exact 25-owner closure, and register frontend reports.

**Wave 7** *(blocked on the full sidecar plus exposure/runtime/protobuf foundations)*

- [ ] 31.2-05-PLAN.md - Exact-join one sidecar across transport/exposure and emit two reports over Plan 10 wiring.

**Wave 8** *(blocked on all surface producers)*

- [ ] 31.2-09-PLAN.md - Aggregate exact ten reports with one immutable image build, one migration session, sole quality parent and redaction.

Cross-cutting constraints:

- Phase 31.2 remains at 0/15 until execution; planning/checker language is not implementation or release approval.
- Plan 01 `SchemaIdentityV1`/`OperationContractMetadataV1` is the exact schema identity used by Plans 03-05; TypeScript generics are diagnostics only.
- Plans 04/10/11/12/15 explicitly migrate the exact 25 known production caller/transport owners; Plan 03 runs only after them and fails on a missing owner or newly discovered caller.
- Plan 10 consumes exact Phase 31.1 `ModelOption.capabilityId`, `AgentToolDefinition.capabilityId`, and authenticated identity-bound app projection; caller projection JSON and client maps are forbidden.
- Plan 09 creates one release session, builds the Phase 31 artifact bundle/tag/digest once, shares it with build/readiness/deployment, and invokes the migration session once without a separate static pre-run.
- During tasks only disposable Stage-A callgraph/identity fixtures run. After the atomic commit, only `verify-quality-gates.sh` runs; it is the unique direct clean-head aggregate parent.

**Design**: `docs/superpowers/specs/2026-07-15-phase-31-release-contract-design.md`

### Phase 32: 身份、租户与共享出站安全

**Goal**: 组织成员可以安全完成身份与组织生命周期，所有同步、异步和外部边界都使用可信租户上下文。
**Depends on**: Phase 31.2
**Requirements**: IDEN-01, IDEN-02, IDEN-03, IDEN-04, IDEN-05, IDEN-06, IDEN-07, IDEN-08, IDEN-09, SECU-01, SECU-02, SECU-03
**Success Criteria** (what must be TRUE):

  1. 用户可以注册、登录、保持会话、退出和恢复凭据，失败或过期流程不会留下部分可用身份。
  2. 用户可以创建、加入和切换组织；邀请、成员与角色操作会拒绝过期、跨组织和越权请求。
  3. 可信 actor/organization identity 贯穿 HTTP、gRPC、service、job、retry、vector 和 analytics。
  4. SQL、向量、对象、队列、Admin 和分析路径均拒绝跨租户访问，且不泄露目标对象是否存在。
  5. 所有用户可控出站目标共用 fail-closed policy；credential 可安全轮换，入站 webhook 在改变状态前完成签名、时效和 replay 校验。

**Plans**: TBD
**UI hint**: yes

### Phase 33: 耐久执行、RAG Worker 与共享对象

**Goal**: Knowledge、Agent、Workflow 和 Task 在重启与失败后仍可恢复、重放和审计，生产对象不依赖本地磁盘。
**Depends on**: Phase 32
**Requirements**: KNOW-02, KNOW-03, KNOW-04, KNOW-06, AUTO-02, AUTO-03, AUTO-05, AUTO-07, AUTO-08, AUTO-10, SECU-04, SECU-05, STOR-01, STOR-02
**Success Criteria** (what must be TRUE):

  1. 用户上传文档时可以看到共享对象持久化和 ingestion/index 进度；失败任务可重试、进入 dead letter、审计 replay 并在重启后恢复。
  2. 文档 embedding 只经 Relay 写入声明的向量后端，更新或删除后 stale vector 不再参与正常检索。
  3. Agent 与 Workflow 持久记录结构化运行状态，重启后可以继续或安全回收，retry/replay 不重复已提交的外部副作用。
  4. Scheduled Task 启动真实 Agent 或 Workflow execution，并向用户展示可联查的下游状态、失败和 usage。
  5. 代码执行在 Sandbox 不可用时 fail closed；高风险操作需要授权审计，对象生命周期覆盖校验、扫描、保留、删除、孤儿清理和恢复。

**Plans**: TBD
**UI hint**: yes

### Phase 34: Relay、Chat 与证据主链

**Goal**: 用户获得可取消、可结算、可追踪的真实 Chat 流式响应，所有可计费 AI 调用只有一个权威路径。
**Depends on**: Phase 32, Phase 33
**Requirements**: RLAY-01, RLAY-02, RLAY-03, RLAY-04, RLAY-05, RLAY-06, RLAY-07, RLAY-08, RLAY-09, RLAY-10, CHAT-01, CHAT-02, CHAT-03, CHAT-04, CHAT-05, CHAT-06
**Success Criteria** (what must be TRUE):

  1. Chat、Knowledge、Agent、Workflow、MCP 和受支持 `/v1/*` 的可计费调用只能经 Relay 到达满足能力要求的健康 Provider。
  2. 缺少可信身份、价格、quota 或 Provider 能力时请求在上游调用前失败；retry/fallback 可审计且不产生重复输出。
  3. 用户可以看到顺序稳定的真实流式增量并取消调用，取消会传播到 Provider 且留下可查询终态。
  4. 每次成功、失败、取消或断流调用都有唯一 usage、价格快照、quota settlement/refund 和 request-to-audit 联查证据。
  5. 用户可以创建 Chat、选择模型、绑定 Knowledge、持久化消息、取消或重试、查看引用与可操作错误，并保留 identity 转换为 SOLO 或 Task。

**Plans**: TBD
**UI hint**: yes

### Phase 35: 真实客户、Builder、Admin 与渠道旅程

**Goal**: 客户、Builder 和运营者可以通过真实 Web、Go、数据库、worker 与外部 rail 完成核心工作区和渠道操作。
**Depends on**: Phase 34
**Requirements**: KNOW-01, KNOW-05, KNOW-07, AUTO-01, AUTO-04, AUTO-06, AUTO-09, AUTO-11, CHAN-01, CHAN-02, CHAN-03, CHAN-04, CHAN-05, CHAN-06, ADMN-01, ADMN-02, ADMN-03, ADMN-05, ADMN-06, RELS-04, RELS-05, RELS-06
**Success Criteria** (what must be TRUE):

  1. Release operator 可以在不拦截产品 API 的浏览器中完成真实 Identity、Chat 和 Admin 旅程，并验证持久化 identity。
  2. 用户可以管理 Knowledge、通过真实 worker 检索当前组织的有效版本与引用，并查看 source、version、score、quality 和失败诊断。
  3. Builder 可以版本化 Agent/Workflow、配置 MCP/tool 与审批策略、调度 Task，并通过真实 target 旅程完成审批、运行和失败恢复。
  4. 获授权 Admin 的用户、组织、Provider、route、price、plan 和 channel mutation 会改变真实运行状态；backlog、dead letter 和 remediation 可操作且高风险变更留有审计。
  5. 至少一个 manifest 声明的渠道完成 credential readiness、签名入站、去重、组织映射、出站 receipt、delivery status、重试/dead letter 和 credential 轮换。

**Plans**: TBD
**UI hint**: yes

### Phase 36: 财务与 Marketplace 对账闭环

**Goal**: 客户、Publisher 和 Finance operator 可以完成可重放、可对账且不重复记账的真实资金与 Marketplace 生命周期。
**Depends on**: Phase 34, Phase 35
**Requirements**: BILL-01, BILL-02, BILL-03, BILL-04, BILL-05, BILL-06, BILL-07, BILL-08, BILL-09, MRKT-01, MRKT-02, MRKT-03, MRKT-04, MRKT-05, MRKT-06, MRKT-07, MRKT-08, MRKT-09, MRKT-10, ADMN-04, RELS-07
**Success Criteria** (what must be TRUE):

  1. 客户可以使用真实支付渠道订阅或充值，并查看套餐、quota、usage、invoice、payment 和 credit；签名确认前不会提前授予 entitlement。
  2. Relay usage 只产生一次 ledger 影响，quota 预留、refund、dispute/chargeback 和定时 reconciliation 均可追踪且幂等。
  3. Publisher 可以版本化 listing、提交审核、发布或下架，Buyer 可以检查依赖并完成免费或支付确认后的组织级安装。
  4. Marketplace refund/chargeback、settlement、fee、Publisher revenue 和 external payout 可按 order/payment/settlement identity 联查、重试和对账。
  5. Finance operator 可以通过真实浏览器旅程联查 request 到 payout，并处理 reconciliation 异常、申诉、abuse、takedown 和 restore。

**Plans**: TBD
**UI hint**: yes

### Phase 37: 持久观测、SLO 与全状态恢复

**Goal**: 运营者可以从权威业务 identity 观测系统、接收和恢复告警，并证明所有声明状态可恢复。
**Depends on**: Phase 36
**Requirements**: OPER-01, OPER-02, OPER-03, OPER-04, OPER-07
**Success Criteria** (what must be TRUE):

  1. 运营者可以按 organization 和 request identity 查询持久 request log/audit，并联查 execution、usage、billing、payment 和 settlement。
  2. Metrics 与 traces 携带必要业务 identity 且避免高基数客户 label，真实目标环境报告约定的 API、Relay、RAG、Workflow 和 availability SLO。
  3. 告警可以真实 delivery、acknowledge、escalate 和 recover，每次恢复动作都留下有界、授权的审计。
  4. 每个声明状态存储都通过 backup/restore drill，恢复后核心客户旅程和账务联查继续通过。

**Plans**: TBD

### Phase 38: 声明部署模式能力对等

**Goal**: 运营者只会部署和宣传具备真实 readiness、回滚能力及相同租户语义的运行模式。
**Depends on**: Phase 35, Phase 37
**Requirements**: OPER-05, OPER-06, OPER-08
**Success Criteria** (what must be TRUE):

  1. 每个声明部署模式的 readiness 同时反映必要依赖、worker ownership 和 backlog，而不是只有 health endpoint 成功。
  2. 运营者可以在每个声明模式执行 migration、deploy、target smoke 和 rollback。
  3. monolith、dual 或 split 模式通过相同能力、真实旅程和 tenant-denial 测试；未通过模式从 manifest、默认配置和发布承诺中移除。

**Plans**: TBD

### Phase 39: 供应链与目标商业发布

**Goal**: 发布运营者可以从同一已 push commit 重复制品，并用外部 E3/E4 证据证明目标环境商业发布无 skip 完成。
**Depends on**: Phase 38
**Requirements**: RELS-03, RELS-08, RELS-09, RELS-10, RELS-11, RELS-12
**Success Criteria** (what must be TRUE):

  1. CI 对 production build、unit、integration、contract、race、lint、security、dependency、migration、restore 和必要 load gate 不发生静默 skip。
  2. 同一 release commit 可以重复构建 immutable image 并记录 canonical digest。
  3. 每个制品具有可验证 SBOM、provenance、signature、vulnerability result，且 CI Actions 固定到完整 SHA。
  4. 所有声明的 Provider、支付、数据、对象、观测和 Kubernetes 依赖都有位于 git 外的新鲜 E3 target evidence。
  5. strict verifier 针对同一已 push commit、artifact digest、clean worktree 和外部 artifact body 无 skip 通过，并记录完整 readiness 元数据与 residual risk。

**Plans**: TBD

## Progress

**Execution Order:** Phase 31 -> Phase 31.1 -> Phase 31.2 -> Phase 32 -> Phase 33 -> Phase 34 -> Phase 35 -> Phase 36 -> Phase 37 -> Phase 38 -> Phase 39

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 31. 发布合同与可信构建身份 | 7/7 | Complete   | 2026-07-17 |
| 31.1 动态 Readiness 与持续 Fail-Closed | 22/22 | Complete    | 2026-07-21 |
| 31.2 契约表面一致性与聚合门禁 | 3/15 | In Progress|  |
| 32. 身份、租户与共享出站安全 | 0/TBD | Not started | - |
| 33. 耐久执行、RAG Worker 与共享对象 | 0/TBD | Not started | - |
| 34. Relay、Chat 与证据主链 | 0/TBD | Not started | - |
| 35. 真实客户、Builder、Admin 与渠道旅程 | 0/TBD | Not started | - |
| 36. 财务与 Marketplace 对账闭环 | 0/TBD | Not started | - |
| 37. 持久观测、SLO 与全状态恢复 | 0/TBD | Not started | - |
| 38. 声明部署模式能力对等 | 0/TBD | Not started | - |
| 39. 供应链与目标商业发布 | 0/TBD | Not started | - |
