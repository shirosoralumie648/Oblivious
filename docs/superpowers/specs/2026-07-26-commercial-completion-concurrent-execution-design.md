# Commercial Completion & Concurrent Execution Design

> Date: 2026-07-26
> Status: Draft (awaiting user spec review)
> Scope: 规划层完善 —— 让 `.planning/` 体系能指引 agent 以「地基先行 + 多模块并发」方式完成一个功能完整、可商业化的 Oblivious。
> 不改产品代码;只改/新增规划与治理文档。

## 0. TL;DR

本规格解决的不是「需求缺失」（`REQUIREMENTS.md` 已有 101 条 v1 需求、100% 映射、0 遗漏），
而是四个「plan 无法指引 agent 做出完整商用产品」的结构性问题：

1. **入口脱节** —— 根目录 `goal.md` 指向 2026-06 的废弃微服务计划。
2. **结构纯线性** —— ROADMAP Phase 32→39 强依赖串行、每阶段横切所有模块，机制上不支持模块级并发。
3. **优先级失衡** —— Phase 31/31.1/31.2 把大量精力投入发布工程元工作，产品主体停在 E2 基线。
4. **缺并发协作规则** —— 没有「多 agent 如何锁契约、分文件边界、集成对接」的机制。

解决办法：修入口 + 波段并发执行模型（Band 0 地基 → Band 1 并发轨道 → Band 2 收口）
+ 并发协作机制（契约先锁 / 轨道边界 / 集成门 / 修订协议）+ 优先级再平衡 + 商业打磨覆盖复查。

## 1. Context / 现状

### 1.1 两套并存的规划

| 文件 | 角色 | 处置 |
|------|------|------|
| 根 `goal.md` | 旧入口，25 行，只说「按 4 份 fusion 设计文档实现」 | **重写为薄治理宪章** |
| 根 `IMPLEMENTATION_PLAN.md` | 2026-06-11 微服务改造周计划，已与现实脱节 | **标记 DEPRECATED** |
| `.planning/PROJECT.md` | 业务定义、用户与关键旅程、既有基线 | 保留，唯一「是什么」 |
| `.planning/REQUIREMENTS.md` | 101 条 v1 需求 + E1–E4 证据模型 + DoD | 保留，唯一「做到什么算完成」 |
| `.planning/ROADMAP.md` | Phase 31–39，含 Goal/依赖/成功标准 | 加 band/track 标注（经 GSD） |
| `.planning/STATE.md` | 当前进度（18%，正在 Phase 31.2） | GSD 自动演进，不手改 |

### 1.2 GSD 约束（硬约束）

`.planning/` 由 GSD 工具链管理。`AGENTS.md` 规定：改文件前必须走 GSD 命令（`/gsd-quick`、
`/gsd-execute-phase` 等），不在 GSD 工作流外直接编辑，除非用户显式要求 bypass。

推论：Phase 32–39 的 `Plans: TBD` **不是缺陷**，是 GSD 的 just-in-time 设计 —— 每个 phase 执行前
用 `/gsd-plan-phase` 展开细粒度 plan。因此本规格**不预先撑满各 phase 的 plan**，而是提供
**执行策略覆盖层**，指导 GSD 在展开 plan 时如何切分并发轨道、锁契约、设集成门。

### 1.3 依赖现状

ROADMAP 现有执行序为严格线性：`31 → 31.1 → 31.2 → 32 → 33 → 34 → 35 → 36 → 37 → 38 → 39`。
其中 32（身份/租户/出站安全）与 34（Relay 权威）是所有产品模块共用的地基；33 提供耐久执行原语。
35/36 是真正的「产品旅程」，但被排在地基之后且彼此串行。

## 2. Goals / Non-Goals

**Goals**

- G1 让任意 agent 从单一入口即可理解「做什么、做到什么算完整商用、当前在哪、下一步怎么并发推进」。
- G2 在**不破坏 GSD 自动化**的前提下，把线性 phase 重排为「地基先行 + 多模块并发 + 串行收口」。
- G3 定义可落地的多 agent 并发协作机制，保证并发下的接口契约一致（呼应原 goal.md 原则）。
- G4 建立优先级护栏，防止发布工程元工作阻塞产品轨道。
- G5 对「完善可商用」的产品打磨维度做一次覆盖复查，产出 triage 结论。

**Non-Goals**

- N1 不改产品源码（`src/**`）；本规格只动规划/治理文档。
- N2 不重排 GSD phase 编号，不改 REQUIREMENTS 的需求条目与 traceability。
- N3 不新增 v2 之外的能力范围；YAGNI，不扩张 `Out of Scope` 已排除项。
- N4 不做 big-bang 微服务或前端重写（REQUIREMENTS 已明确排除）。

## 3. 入口治理（修「顶层脱节」）

### 3.1 `goal.md` 重写为薄治理宪章

新 `goal.md` 只承担「导航 + 不变量」，不重复 PROJECT/REQUIREMENTS 内容。结构：

- **唯一事实源**：`.planning/PROJECT.md`（是什么）、`REQUIREMENTS.md`（做到什么算完成）、
  `ROADMAP.md`（阶段）、`STATE.md`（当前在哪）、本执行策略（怎么并发推进）。
- **商业完成定义**：直接引用 REQUIREMENTS 的 E1–E4 证据模型与 Definition of Done，
  强调「route/page/schema/proto/health/docs 的存在都不等于旅程完成」。
- **核心不变量**：Relay 是唯一可计费 AI 权威路径；tenant 上下文贯穿所有边界；未证明能力 fail closed。
- **执行入口**：所有改动走 GSD 命令；并发推进遵循本执行策略与波段模型。
- **停止条件**：`no-final-readiness` —— 缺 E3/E4 证据不得声称最终商业 readiness。

### 3.2 `IMPLEMENTATION_PLAN.md` 弃用

顶部加横幅：`> DEPRECATED (2026-07-26)：本文件为 2026-06 微服务计划的历史存档，已被
.planning/ROADMAP.md + EXECUTION-STRATEGY.md 取代。请勿据此执行。` 正文保留供追溯。
同步更新 `docs/design/FUSION_GAP_CLOSURE_PLAN.md`、`docs/reports/2026-06-13-repo-rescan.md`
中指向它的引用（加同样的弃用说明，不删历史）。

## 4. 波段执行模型（修「结构纯线性」）

把现有 phase **映射**（非重编号）到三个波段。这是覆盖层语义，物理上仍是 GSD 的 Phase 32–39。

```
Band 0 — 地基（串行，先锁）        映射 Phase 32 + Phase 33
  身份/租户/可信 actor 上下文 · 共享出站安全(SECU-01..03)
  · 耐久执行原语(job/lease/retry/dead-letter) · 共享对象存储 · Sandbox 隔离
  · Relay 权威核心契约(RLAY client 接口/usage/quota 快照 —— 从 Phase 34 前移「契约」部分)
        │
        ▼  ── Contract Lock #1（冻结跨轨共享契约）──
        ▼
Band 1 — 产品轨道（并发，各自端到端竖切）  映射 Phase 34 + 35 + 36
  Track A  Chat + Relay 流式主链         (RLAY-*, CHAT-*)
  Track B  Knowledge / RAG               (KNOW-*)
  Track C  Agent / Workflow / Task       (AUTO-*)
  Track D  Admin / 治理                  (ADMN-*)
  Track E  账务 / Marketplace / payout   (BILL-*, MRKT-*)
  Track F  渠道                          (CHAN-*)
        │
        ▼  ── 集成门（对真实地基跑跨轨 E2E）──
        ▼
Band 2 — 横切收口（串行）          映射 Phase 37 + 38 + 39
  观测/SLO/恢复(OPER-01..04,07) · 部署模式对等(OPER-05,06,08) · 供应链+目标发布(RELS-*)
```

**为什么地基必须串行**：Track A–F 全部依赖统一的 tenant 上下文与 Relay 契约；若地基未锁就并发，
各轨会各自发明租户传递与 Provider 调用方式，导致跨租户泄漏或绕过计费 —— 正是 REQUIREMENTS 要防的。

**为什么产品轨可并发**：各轨拥有独立的领域 package、路由前缀、前端路由、worker 与测试；
共享面已在 Contract Lock #1 冻结，故轨间改动不互相阻塞。

## 5. 并发协作机制（修「缺并发规则」）

这是「多模块并发」能成立的前提，也是当前 plan 完全缺失的部分。写入 `EXECUTION-STRATEGY.md`。

### 5.1 契约先锁（Contract Lock）

Band 1 并发启动**前**，必须先冻结跨轨共享契约，作为 Band 0 的显式交付物。冻结清单：

1. **HTTP 契约** —— OpenAPI operation id、路径、请求/响应 schema（沿用 Phase 31.2 的 surface parity 机制）。
2. **gRPC/proto** —— 服务间接口与消息（沿用 31.2 的 pinned protoc + 唯一归属）。
3. **Tenant 上下文签名** —— 可信 actor/organization 在 HTTP、gRPC、job、retry、vector、analytics 的传递类型与函数签名。
4. **Relay client 接口** —— 产品轨调用 Relay 的唯一 Go 接口（Complete/Stream/Embed + usage/quota/价格快照语义）。
5. **事件 schema** —— 若使用异步事件，其 topic 与 payload schema。
6. **DB schema 归属** —— 每张表归属哪条轨道，跨轨只读访问走接口不直连表。

冻结产物：一份 `contracts-lock.md`（或复用 31.2 已有的 contract 报告），列出上述条目与其 owner track。

### 5.2 轨道文件边界（Track Ownership）

把 GSD 现有的 plan 级 `file-disjoint may run in parallel` 提升到**轨道级**。每条轨道声明独占边界，例如：

| Track | Go package（示意） | 前端路由（示意） | 路由前缀 |
|-------|-------------------|-----------------|---------|
| A Chat | `internal/chat`, `pkg/relay/stream` | `src/web/.../chat` | `/api/chat`, `/v1/chat` |
| B Knowledge | `internal/knowledge` | `.../knowledge` | `/api/knowledge` |
| C Automation | `internal/agent`, `internal/workflow`, `internal/task` | `.../agents`,`.../workflows` | `/api/agents` … |
| D Admin | `internal/admin` | `.../admin` | `/api/admin` |
| E Finance | `internal/billing`, `internal/marketplace` | `.../console`,`.../marketplace` | `/api/billing` … |
| F Channels | `internal/channel` | `.../channels` | `/api/channels` |

（实际边界在 Band 0 结束时按代码现状精确划定并写入 EXECUTION-STRATEGY。）
共享地基代码（tenant、relay 接口、middleware）在 Band 0 后**只读**；需改动走 5.4 修订协议。

### 5.3 集成门（Integration Gate）

并发不等于各跑各的。设固定合流点：每条轨完成一个可验证竖切后，在集成门做：

- 对**真实地基**（非 mock 的 tenant/relay/存储）做集成，而非只跑单轨单测。
- 跑**跨轨 E2E**：如 A+E（Chat 调用产生 usage → 计费 ledger）、C+B（Agent 用 Knowledge 检索）。
- 校验 Phase 31.2 的 contract surface parity 未漂移。
- 任一轨未过集成门，不进入下一轮并发。

### 5.4 契约修订协议（Contract Amendment）

某轨发现必须改已锁契约时：

1. 暂停该改动涉及的跨轨依赖；2. 走一个轻量 GSD quick 记录 amendment（改什么、影响哪些轨）；
3. 更新 `contracts-lock.md` 与受影响 OpenAPI/proto；4. 广播到所有相关轨后再继续。
禁止「先改再说」——这是并发下契约漂移的头号来源。

## 6. 优先级再平衡（修「钻牛角尖」）

写入 EXECUTION-STRATEGY 的护栏原则：

- **P1 发布工程元工作时间盒化**：verifier 严格度、digest、surface parity、O_NOFOLLOW 类加固等
  「证明如何发布」的工作必须限定投入，不得阻塞产品轨道推进。达到「足够 fail-closed + 可验证」即止。
- **P2 证据分级推进**：产品轨按 **E2（仓库运行时）** 证据即可推进并标记轨内完成；
  **E3/E4（目标环境/商业发布）** 外部证据统一在 Band 2 收口，不在产品轨内反复追求。
- **P3 完整优先于目录数量**：沿用 REQUIREMENTS Out-of-Scope —— 生命周期完整度和证据优先于
  「100+ provider / 150+ tool」这类数量堆砌。
- **P4 每轨定义「最小可用竖切」**：先打通一条端到端真实旅程（哪怕只支持 1 个 provider/1 种文档），
  再横向加广度。避免在单一维度过度打磨而迟迟没有可用旅程。

## 7. 商业打磨覆盖复查（修「覆盖缺口」）

REQUIREMENTS 强在正确性/证据，但「完善可商用」还需复查产品打磨维度。对每项给出 triage 结论：
**已覆盖**（指出对应需求）/ **补 v1**（进当前里程碑）/ **归 v2**（记入 v2 需求）。

| 维度 | 初判 | 依据/说明 |
|------|------|-----------|
| 首启引导 / 空状态 | 复查 | PROJECT 提到 onboarding，但空状态/首次体验未见显式需求 |
| 统一错误文案体系 | 复查 | CHAT-05/RLAY-03 有「可操作错误」，但缺跨模块统一错误语义规范 |
| 无障碍 WCAG | **归 v2（已定）** | 用户确认 v1 不做无障碍达标 |
| 响应式 / 移动端 | **归 v2（已定）** | 与 CHAT-20（多端）一致，用户确认归 v2 |
| i18n（中英） | **归 v2（已定）** | 产品中英混用，用户确认 v1 不做 i18n |
| 配额耗尽 / 限流 UX | 已覆盖倾向 | CHAT-05、BILL-05 覆盖配额错误；确认前端呈现闭环即可 |
| 数据导出 / 账户删除 | **补 v1 最小版（已定）** | 用户确认：最小可用的组织数据导出 + 账户/组织删除进 v1（合规底线）|
| 审计可观测的运营视图 | 已覆盖 | ADMN-04/05、OPER-01 已覆盖 |

**复查动作**：本规格不擅自改需求；产出 `docs/superpowers/specs/...-coverage-triage.md`（或在 EXECUTION-STRATEGY 内附表），
把「补 v1」项走 GSD 正式加入 REQUIREMENTS，「归 v2」项记入 v2 段。已定结论：无障碍/移动端/i18n 归 v2；
数据导出 + 账户/组织删除最小版补 v1（合规底线，走 GSD 新增需求条目并映射 phase）；错误文案体系与首启引导/空状态补 v1（基本可用性）。

## 8. 交付物与文件改动

| 动作 | 文件 | 说明 |
|------|------|------|
| 重写 | `goal.md` | 薄治理宪章（§3.1）|
| 弃用 | `IMPLEMENTATION_PLAN.md` | 顶部加 DEPRECATED 横幅（§3.2）|
| 更新引用 | `docs/design/FUSION_GAP_CLOSURE_PLAN.md`、`docs/reports/2026-06-13-repo-rescan.md` | 加弃用指向 |
| 新增 | `.planning/EXECUTION-STRATEGY.md` | 波段模型 §4 + 并发机制 §5 + 优先级护栏 §6 |
| 新增 | `.planning/contracts-lock.md`（Band 0 交付） | 契约冻结清单 §5.1（Band 0 时填实）|
| 新增/附表 | 覆盖 triage（§7） | 复查结论；「补 v1」走 GSD 入 REQUIREMENTS |
| 标注 | `.planning/ROADMAP.md` | 经 GSD 给 Phase 32–39 加 Band/Track 标注，不重编号 |

> ROADMAP/REQUIREMENTS 的改动通过 GSD 命令进行；`goal.md`、`IMPLEMENTATION_PLAN.md`、
> `EXECUTION-STRATEGY.md` 属根级治理/新增文档，按用户对本规格的批准落地。

## 9. 如何验证「plan 完善」本身

规划文档无单测，验收标准定义为：

- V1 从 `goal.md` 出发，可在 3 跳内到达「做什么/何为完成/当前在哪/下一步并发怎么走」。
- V2 EXECUTION-STRATEGY 中每条 Track 都能指出：拥有哪些需求、独占哪些文件边界、依赖哪些已锁契约。
- V3 波段映射覆盖 Phase 32–39 全部，且与 REQUIREMENTS traceability 无冲突（101 条仍 100% 映射）。
- V4 无占位符/TODO/自相矛盾；与 GSD 约束（不绕过、不重编号）一致。
- V5 覆盖 triage 每项都有明确结论（已覆盖/补 v1/归 v2），无「待定」。

## 10. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 与 GSD 自动化耦合，手改 planning 文件引发漂移 | 只手改根级治理文档；ROADMAP/REQUIREMENTS 走 GSD；STATE 不碰 |
| 并发轨道契约漂移 | Contract Lock（§5.1）+ 修订协议（§5.4）+ 集成门（§5.3）三重约束 |
| 地基迟迟锁不完导致并发无法开始 | Band 0 定义明确的「契约冻结」交付物作为并发准入 |
| 优先级护栏被忽视，仍钻发布工程 | P1–P4 写入 EXECUTION-STRATEGY，并作为 phase 展开时的检查项 |
| 覆盖 triage 擅自扩张范围 | 不自动改需求；「补 v1」必须用户确认后走 GSD |

## 11. 锁定默认值与待确认

**已锁定（用户可在规格复审时推翻）**

- 轨道切分：A–F 六轨（§4）。
- ROADMAP：不破坏性重写，仅加 Band/Track 标注 + 独立 EXECUTION-STRATEGY 覆盖层。
- 覆盖 triage：以清单形式产出结论，不强行全塞 v1。

**已确认（2026-07-26，用户拍板）**

- 无障碍 / 移动端响应式 / i18n：**归 v2**（v1 先证明核心商业旅程）。
- 数据导出 / 账户删除最小版：**进 v1**（合规底线，做最小可用版）—— 走 GSD 新增对应需求条目并映射 phase。

## 12. 后续（brainstorming 之后）

本规格获批并经用户复审后，调用 writing-plans skill，把 §8 交付物转成可执行的实施计划
（逐文件编辑步骤 + 顺序 + 每步验证），再按计划执行。
