# Roadmap: Oblivious Commercial Complete & Target Release

## Overview

本路线从当前 brownfield E1/E2 基线出发，先固定发布合同、可信身份和共享安全边界，再完成耐久执行、Relay 权威主链与真实客户旅程，随后关闭资金、观测、恢复和声明部署模式的能力对等，最终以同一 release commit 和 artifact digest 的 E3/E4 no-skip 证据完成商业发布。阶段按依赖关系组织，但每阶段都必须交付可由用户或运营者观察和验证的完整能力。

**Historical evidence boundary:** `.planning/phases/01-*` 至 `30-*` 是已保留的历史 milestone E1/E2 证据；本 milestone 从历史 Phase 30 后连续编号为 Phase 31-39，旧 summary 和 verification 不计入当前进度。

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): planned milestone work
- Decimal phases (2.1, 2.2): urgent insertions marked `INSERTED`

- [ ] **Phase 31: 发布合同与当前基线** - 明确承诺能力、部署形态和当前 runtime 契约。
- [ ] **Phase 32: 身份、租户与共享出站安全** - 建立可信组织边界和统一 fail-closed 集成安全。
- [ ] **Phase 33: 耐久执行、RAG Worker 与共享对象** - 让知识和自动化任务可恢复、可重放且不依赖本地状态。
- [ ] **Phase 34: Relay、Chat 与证据主链** - 证明唯一 Provider/usage 权威及完整流式 Chat 生命周期。
- [ ] **Phase 35: 真实客户、Builder、Admin 与渠道旅程** - 用真实全栈路径交付工作区、治理和至少一个渠道生命周期。
- [ ] **Phase 36: 财务与 Marketplace 对账闭环** - 关闭支付、账本、交易、结算、payout 和异常治理。
- [ ] **Phase 37: 持久观测、SLO 与全状态恢复** - 让运营者可联查、告警并恢复所有声明状态。
- [ ] **Phase 38: 声明部署模式能力对等** - 仅保留通过同一能力和租户验证的部署形态。
- [ ] **Phase 39: 供应链与目标商业发布** - 生成不可变制品并取得同 commit、同 digest 的 E3/E4 no-skip 证据。

## Phase Details

### Phase 31: 发布合同与当前基线
**Goal**: 发布运营者可以从机器可读合同确认承诺能力、部署形态和当前 runtime 契约。
**Depends on**: Nothing (first phase)
**Requirements**: RELS-01, RELS-02
**Success Criteria** (what must be TRUE):
  1. 发布运营者可以检查 capability/deployment manifest，并准确识别本次发布承诺的模块、集成和运行模式。
  2. 未承诺或生命周期不完整的能力不会出现在默认可用范围内，并会显式 fail closed。
  3. OpenAPI、protobuf、migration 和前端 client 与当前 runtime 一致，任何 contract drift 都会阻断发布。
**Plans**: TBD

### Phase 32: 身份、租户与共享出站安全
**Goal**: 组织成员可以安全完成身份与组织生命周期，所有同步、异步和外部边界都使用可信租户上下文。
**Depends on**: Phase 31
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

**Execution Order:** Phase 31 -> Phase 32 -> Phase 33 -> Phase 34 -> Phase 35 -> Phase 36 -> Phase 37 -> Phase 38 -> Phase 39

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 31. 发布合同与当前基线 | 0/TBD | Not started | - |
| 32. 身份、租户与共享出站安全 | 0/TBD | Not started | - |
| 33. 耐久执行、RAG Worker 与共享对象 | 0/TBD | Not started | - |
| 34. Relay、Chat 与证据主链 | 0/TBD | Not started | - |
| 35. 真实客户、Builder、Admin 与渠道旅程 | 0/TBD | Not started | - |
| 36. 财务与 Marketplace 对账闭环 | 0/TBD | Not started | - |
| 37. 持久观测、SLO 与全状态恢复 | 0/TBD | Not started | - |
| 38. 声明部署模式能力对等 | 0/TBD | Not started | - |
| 39. 供应链与目标商业发布 | 0/TBD | Not started | - |
