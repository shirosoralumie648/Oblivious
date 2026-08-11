# Commercial Completion Plan Refinement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `.planning/` 规划体系完善成能指引 agent 以「地基先行 + 多模块并发」方式做出功能完整、可商业化 Oblivious 的形态——通过修入口、加执行策略覆盖层、定并发协作机制。

**Architecture:** 纯规划/治理文档改动。根级文件(`goal.md`、`IMPLEMENTATION_PLAN.md`)与新增覆盖层文档(`.planning/EXECUTION-STRATEGY.md`、`.planning/contracts-lock.md`)可直接编辑;GSD 核心文件(`ROADMAP.md`、`REQUIREMENTS.md`)的改动必须走 GSD 命令。不触碰 `src/**` 产品代码。

**Tech Stack:** Markdown 文档;GSD 工具链(`/gsd-quick` 等);Git;grep/wc 做验证。

## Global Constraints

> 以下为项目级约束,每个任务都隐含适用。逐字来自 spec 与 AGENTS.md。

- **不动产品代码**:本计划只改/新增规划与治理文档,`src/**` 零改动。
- **GSD 边界**:`AGENTS.md` 规定「改文件前走 GSD 命令,不在 GSD 工作流外直接编辑」。据此——根级 `goal.md`/`IMPLEMENTATION_PLAN.md`、`docs/**` 引用、以及 `.planning/` 下**新增**的覆盖层文档(EXECUTION-STRATEGY、contracts-lock)按用户对本工作的授权直接编辑;对 GSD 核心文件 `ROADMAP.md`、`REQUIREMENTS.md`、`STATE.md`、`PROJECT.md` 的改动**必须走 GSD 命令**(Task 5、Task 6 已标注)。
- **不重编号**:不改 GSD 的 Phase 编号(31–39),波段/轨道以**标注**形式叠加。
- **不改需求语义**:除用户已拍板新增的「数据导出/账户删除最小版」外,不新增/删除/改写 REQUIREMENTS 条目;101 条现有需求与 traceability 保持不变。
- **提交授权**:commit/push 步骤为计划蓝图的一部分;实际执行时 commit 需用户显式授权,GSD 任务的提交由 GSD 流程处理。
- **唯一事实源不重复**:治理宪章与执行策略只做导航与规则,不复制 PROJECT/REQUIREMENTS 的内容。

## File Structure

| 文件 | 责任 | 处置 | 归属 |
|------|------|------|------|
| `goal.md` | 项目唯一入口:导航 + 不变量 | 重写为薄治理宪章 | 根级,直接编辑 |
| `IMPLEMENTATION_PLAN.md` | 历史微服务计划 | 加 DEPRECATED 横幅 | 根级,直接编辑 |
| `docs/design/FUSION_GAP_CLOSURE_PLAN.md` | 引用旧计划处 | 加弃用指向 | docs,直接编辑 |
| `docs/reports/2026-06-13-repo-rescan.md` | 引用旧计划处 | 加弃用指向 | docs,直接编辑 |
| `.planning/EXECUTION-STRATEGY.md` | 波段模型 + 并发机制 + 优先级护栏 + 覆盖 triage | 新增 | 覆盖层,直接编辑 |
| `.planning/contracts-lock.md` | Band 0 契约冻结清单(模板) | 新增骨架 | 覆盖层,直接编辑 |
| `.planning/ROADMAP.md` | Phase 32–39 加 Band/Track 标注 | 走 GSD | GSD 核心 |
| `.planning/REQUIREMENTS.md` | 新增数据导出/账户删除 v1 需求 + traceability | 走 GSD | GSD 核心 |

**任务依赖:** Task 1–4 相互独立(不同文件),可并发。Task 5、6 走 GSD、依赖 Task 3(需引用 EXECUTION-STRATEGY 的 band/track 定义与 contracts-lock)。

---

### Task 1: 重写 goal.md 为治理宪章

**Files:**
- Modify(重写): `goal.md`

**Interfaces:**
- Consumes: 无(独立)。
- Produces: 项目唯一入口;后续文档与 agent 从此导航到 `.planning/` 各文件与 `EXECUTION-STRATEGY.md`。

- [ ] **Step 1: 用以下完整内容覆盖 `goal.md`**

```markdown
# Oblivious 项目宪章 (Governance Charter)

> 本文件是进入本项目的唯一入口。它不重复需求与阶段细节,只提供导航与不可协商的不变量。
> 历史实现计划见 IMPLEMENTATION_PLAN.md(已弃用)。

## 唯一事实源 (Source of Truth)

| 问题 | 文件 |
|------|------|
| 这是什么产品、给谁用、关键旅程 | `.planning/PROJECT.md` |
| 做到什么才算商业化完成(含证据标准) | `.planning/REQUIREMENTS.md` |
| 分几个阶段、依赖与成功标准 | `.planning/ROADMAP.md` |
| 现在进行到哪 | `.planning/STATE.md` |
| 如何并发推进、如何锁契约 | `.planning/EXECUTION-STRATEGY.md` |

## 什么才算「商业化完成」

完成的唯一定义见 `.planning/REQUIREMENTS.md` 的 Definition of Done 与 E1–E4 证据模型:

- E1 单元/契约/fixture → E2 仓库运行时 → E3 目标环境 → E4 商业发布(同 commit/digest、无 skip)。
- route、page、schema、proto、health endpoint、测试数量或文档存在,都**不**单独证明旅程完成。
- 低等级证据不能关闭要求更高等级证据的需求。

## 核心不变量 (Invariants)

1. **Relay 唯一权威**:所有可计费 AI 调用只能经 Relay 到达 Provider;不得有第二套 usage/billing 权威。
2. **租户贯穿**:可信 actor/organization 上下文贯穿 HTTP、gRPC、job、retry、vector、analytics;跨租户访问一律拒绝。
3. **Fail closed**:未声明或未证明的能力必须禁用、隐藏并安全失败。
4. **证据优先于数量**:生命周期完整度和证据优先于 Provider/tool/channel 的目录数量。

## 如何推进 (Execution)

- 所有改动通过 GSD 命令进行(`/gsd-quick`、`/gsd-execute-phase` 等),不绕过 GSD 直接编辑 planning 文件。
- 并发推进遵循 `.planning/EXECUTION-STRATEGY.md` 的波段模型(地基先行 → 多轨并发 → 串行收口)与并发协作机制(契约先锁 / 轨道边界 / 集成门 / 修订协议)。

## 停止条件 (no-final-readiness)

在缺少每个商业 gate 的当前证据、自动化验证和适用 runtime smoke 之前,不得声称最终商业 readiness。
```

- [ ] **Step 2: 验证内容完整**

Run: `grep -c "唯一事实源\|Definition of Done\|Relay 唯一权威\|EXECUTION-STRATEGY\|no-final-readiness" goal.md`
Expected: ≥ 5(五个关键锚点都在)

- [ ] **Step 3: 验证无残留旧内容**

Run: `grep -c "complete-fusion-design\|4 份设计文档\|四个设计文档" goal.md`
Expected: 0(旧的「按 4 份设计文档实现」表述已清除)

- [ ] **Step 4: Commit(需授权)**

```bash
git add goal.md
git commit -m "docs(goal): rewrite goal.md as thin governance charter"
```

---

### Task 2: 弃用 IMPLEMENTATION_PLAN.md 并更新引用

**Files:**
- Modify: `IMPLEMENTATION_PLAN.md`(顶部加横幅)
- Modify: `docs/design/FUSION_GAP_CLOSURE_PLAN.md`(引用处加注)
- Modify: `docs/reports/2026-06-13-repo-rescan.md`(引用处加注)

**Interfaces:**
- Consumes: Task 1 产出的新 `goal.md`(横幅指向它)。
- Produces: 无下游依赖。

- [ ] **Step 1: 在 `IMPLEMENTATION_PLAN.md` 第一行前插入弃用横幅**

在文件最顶部插入(原有内容全部保留在横幅之后):

```markdown
> **⚠️ DEPRECATED (2026-07-26)**
> 本文件是 2026-06 微服务改造计划的历史存档,已与现状脱节。
> 当前规划权威见 `.planning/ROADMAP.md` + `.planning/EXECUTION-STRATEGY.md`,入口见 `goal.md`。
> **请勿据此执行。** 正文保留仅供历史追溯。

---

```

- [ ] **Step 2: 定位 docs 中对旧计划的引用**

Run: `grep -n "IMPLEMENTATION_PLAN\|goal.md" docs/design/FUSION_GAP_CLOSURE_PLAN.md docs/reports/2026-06-13-repo-rescan.md`
Expected: 列出具体行号(用于下一步精确加注)

- [ ] **Step 3: 在每处引用旁插入一行弃用注记**

在上一步定位的每个引用行的下一行插入:

```markdown
> 注:上述 `IMPLEMENTATION_PLAN.md` 已于 2026-07-26 弃用,当前权威见 `.planning/ROADMAP.md` + `EXECUTION-STRATEGY.md`。
```

- [ ] **Step 4: 验证横幅与注记就位**

Run: `grep -rl "DEPRECATED (2026-07-26)\|已于 2026-07-26 弃用" IMPLEMENTATION_PLAN.md docs/design/FUSION_GAP_CLOSURE_PLAN.md docs/reports/2026-06-13-repo-rescan.md`
Expected: 三个文件都命中

- [ ] **Step 5: Commit(需授权)**

```bash
git add IMPLEMENTATION_PLAN.md docs/design/FUSION_GAP_CLOSURE_PLAN.md docs/reports/2026-06-13-repo-rescan.md
git commit -m "docs: deprecate stale IMPLEMENTATION_PLAN and update references"
```

---

### Task 3: 新增 `.planning/EXECUTION-STRATEGY.md`

**Files:**
- Create: `.planning/EXECUTION-STRATEGY.md`

**Interfaces:**
- Consumes: 无代码依赖;引用 REQUIREMENTS 的需求 ID 与 ROADMAP 的 Phase 32–39。
- Produces: 波段/轨道/机制定义,被 `goal.md`、`contracts-lock.md`、Task 5 的 ROADMAP 标注共同引用。

- [ ] **Step 1: 用以下完整内容创建 `.planning/EXECUTION-STRATEGY.md`**

````markdown
# Execution Strategy — 地基先行 + 多模块并发

> 本文件是 `.planning/ROADMAP.md` 的**执行覆盖层**,不改 Phase 编号,只定义:
> 阶段如何归入波段、产品如何切分并发轨道、多 agent 如何锁契约协作、优先级如何排。
> 事实源仍是 PROJECT/REQUIREMENTS/ROADMAP/STATE;本文件回答「怎么并发推进」。

## 1. 波段模型 (Bands)

现有 Phase 32–39 映射到三个波段(映射,非重编号):

```
Band 0 — 地基(串行,先锁)        映射 Phase 32 + Phase 33
  身份/租户/可信 actor 上下文 · 共享出站安全(SECU-01..03)
  · 耐久执行原语(job/lease/retry/dead-letter) · 共享对象存储(STOR) · Sandbox 隔离(SECU-04)
  · Relay 权威核心【契约部分】(RLAY client 接口/usage/quota 快照语义,从 Phase 34 前移「契约冻结」)
        │
        ▼  ── Contract Lock #1(冻结跨轨共享契约,见 §3.1)──
        ▼
Band 1 — 产品轨道(并发,各自端到端竖切)  映射 Phase 34 + 35 + 36
  Track A  Chat + Relay 流式主链         (RLAY-*, CHAT-*)
  Track B  Knowledge / RAG               (KNOW-*)
  Track C  Agent / Workflow / Task       (AUTO-*)
  Track D  Admin / 治理                  (ADMN-*)
  Track E  账务 / Marketplace / payout   (BILL-*, MRKT-*)
  Track F  渠道                          (CHAN-*)
        │
        ▼  ── 集成门(对真实地基跑跨轨 E2E,见 §3.3)──
        ▼
Band 2 — 横切收口(串行)          映射 Phase 37 + 38 + 39
  观测/SLO/恢复(OPER-01..04,07) · 部署模式对等(OPER-05,06,08) · 供应链+目标发布(RELS-*)
```

**地基必须串行的理由:** Track A–F 全部依赖统一 tenant 上下文与 Relay 契约。地基未锁就并发,各轨会各自发明租户传递与 Provider 调用,导致跨租户泄漏或绕过计费——正是 REQUIREMENTS 要防的。

**产品轨可并发的理由:** 各轨拥有独立领域 package、路由前缀、前端路由、worker 与测试;共享面已在 Contract Lock #1 冻结,故轨间改动不互相阻塞。

## 2. 轨道定义 (Tracks)

每条轨道自持一条端到端竖切:Go 领域 + HTTP/gRPC + 前端 + worker + 测试 + E2 证据。

| Track | 需求归属 | Go 边界(示意,Band 0 精确化) | 前端边界(示意) | 路由前缀(示意) |
|-------|---------|------------------------------|-----------------|-----------------|
| A Chat | RLAY-*, CHAT-* | `internal/chat`, `pkg/relay/stream` | `.../chat` | `/api/chat`, `/v1/chat` |
| B Knowledge | KNOW-* | `internal/knowledge` | `.../knowledge` | `/api/knowledge` |
| C Automation | AUTO-* | `internal/agent`,`internal/workflow`,`internal/task` | `.../agents`,`.../workflows` | `/api/agents` … |
| D Admin | ADMN-* | `internal/admin` | `.../admin` | `/api/admin` |
| E Finance | BILL-*, MRKT-* | `internal/billing`,`internal/marketplace` | `.../console`,`.../marketplace` | `/api/billing` … |
| F Channels | CHAN-* | `internal/channel` | `.../channels` | `/api/channels` |

> 上表 Go/前端边界为示意。Band 0 结束时须用 `ls src/server/internal src/web/src` 按代码现状精确划定并回填本表。共享地基代码(tenant、relay 接口、middleware)在 Band 0 后**只读**,改动走 §3.4。

## 3. 并发协作机制 (Concurrency Protocol)

### 3.1 契约先锁 (Contract Lock)

Band 1 并发启动**前**,必须冻结以下跨轨共享契约,作为 Band 0 显式交付物(清单落在 `contracts-lock.md`):

1. **HTTP 契约** — OpenAPI operation id/路径/请求响应 schema(沿用 Phase 31.2 surface parity)。
2. **gRPC/proto** — 服务间接口与消息(沿用 31.2 pinned protoc + 唯一归属)。
3. **Tenant 上下文签名** — 可信 actor/organization 在 HTTP/gRPC/job/retry/vector/analytics 的传递类型与函数签名。
4. **Relay client 接口** — 产品轨调用 Relay 的唯一 Go 接口(Complete/Stream/Embed + usage/quota/价格快照语义)。
5. **事件 schema** — 若用异步事件,topic 与 payload schema。
6. **DB schema 归属** — 每张表归属哪条轨道;跨轨只读访问走接口,不直连表。

### 3.2 轨道文件边界 (Track Ownership)

把 GSD 现有 plan 级 `file-disjoint may run in parallel` 提升到**轨道级**。每轨独占 §2 声明的 package/目录;agent 不越界改他轨文件。冲突处(如共享 router 注册)集中在地基,Band 0 后只读。

### 3.3 集成门 (Integration Gate)

每条轨完成一个可验证竖切后,在集成门做:
- 对**真实地基**(非 mock 的 tenant/relay/存储)集成,而非只跑单轨单测。
- 跑**跨轨 E2E**:如 A+E(Chat 调用产生 usage → 计费 ledger)、C+B(Agent 用 Knowledge 检索)。
- 校验 Phase 31.2 contract surface parity 未漂移。
- 任一轨未过集成门,不进入下一轮并发。

### 3.4 契约修订协议 (Contract Amendment)

某轨须改已锁契约时:①暂停涉及的跨轨依赖;②走轻量 GSD quick 记录 amendment(改什么、影响哪些轨);③更新 `contracts-lock.md` 与受影响 OpenAPI/proto;④广播到所有相关轨后再继续。**禁止「先改再说」**——这是并发下契约漂移的头号来源。

## 4. 优先级护栏 (Priority Guardrails)

- **P1 发布工程元工作时间盒化**:verifier 严格度、digest、surface parity、O_NOFOLLOW 类加固等「证明如何发布」的工作限定投入,不得阻塞产品轨道。达到「足够 fail-closed + 可验证」即止。
- **P2 证据分级推进**:产品轨按 **E2** 证据即可推进并标记轨内完成;**E3/E4** 外部证据统一在 Band 2 收口,不在产品轨内反复追求。
- **P3 完整优先于目录数量**:生命周期完整度和证据优先于「100+ provider / 150+ tool」的数量堆砌(沿用 REQUIREMENTS Out-of-Scope)。
- **P4 每轨定义「最小可用竖切」**:先打通一条端到端真实旅程(哪怕只支持 1 provider / 1 种文档),再横向加广度。

## 5. 商业打磨覆盖 triage

| 维度 | 结论 | 说明 |
|------|------|------|
| 首启引导 / 空状态 | 补 v1 | 基本可用性;走 GSD 补需求 |
| 统一错误文案体系 | 补 v1 | CHAT-05/RLAY-03 有可操作错误,但缺跨模块统一语义 |
| 无障碍 WCAG | 归 v2 | 用户已定 |
| 响应式 / 移动端 | 归 v2 | 用户已定,与 CHAT-20 一致 |
| i18n(中英) | 归 v2 | 用户已定 |
| 配额耗尽 / 限流 UX | 已覆盖 | CHAT-05、BILL-05;确认前端呈现闭环 |
| 数据导出 / 账户删除(最小版) | 补 v1 | 用户已定,合规底线;走 GSD 新增条目(见 ROADMAP/REQUIREMENTS 更新) |
| 审计可观测运营视图 | 已覆盖 | ADMN-04/05、OPER-01 |

> 「补 v1」项经 GSD 正式加入 REQUIREMENTS 后,此表对应行更新为「已覆盖 + 需求 ID」。
````

- [ ] **Step 2: 验证结构完整(五节齐全)**

Run: `grep -c "## 1. 波段模型\|## 2. 轨道定义\|## 3. 并发协作机制\|## 4. 优先级护栏\|## 5. 商业打磨覆盖" .planning/EXECUTION-STRATEGY.md`
Expected: 5

- [ ] **Step 3: 验证六轨与四机制齐全**

Run: `grep -c "Track A\|Track B\|Track C\|Track D\|Track E\|Track F" .planning/EXECUTION-STRATEGY.md && grep -c "Contract Lock\|轨道文件边界\|集成门\|契约修订协议" .planning/EXECUTION-STRATEGY.md`
Expected: 第一条 ≥ 6,第二条 ≥ 4

- [ ] **Step 4: 无占位符扫描**

Run: `grep -n "TODO\|TBD\|FIXME\|待填\|xxx" .planning/EXECUTION-STRATEGY.md || echo "clean"`
Expected: `clean`(「Band 0 精确化」「Band 0 回填」是有意的执行时动作,非占位符)

- [ ] **Step 5: Commit(需授权)**

```bash
git add .planning/EXECUTION-STRATEGY.md
git commit -m "docs(planning): add EXECUTION-STRATEGY band/track concurrency model"
```

---

### Task 4: 新增 `.planning/contracts-lock.md` 骨架

**Files:**
- Create: `.planning/contracts-lock.md`

**Interfaces:**
- Consumes: EXECUTION-STRATEGY §3.1 的六类契约定义。
- Produces: Band 0 的契约冻结交付物;Band 1 各 track 启动的准入依据。

- [ ] **Step 1: 用以下内容创建 `.planning/contracts-lock.md`**

````markdown
# Contracts Lock — Band 0 冻结清单

> 这是 Band 1 并发的**准入门**。每类契约在 Band 0 完成时由地基 owner 填实并置为 `LOCKED`。
> 全部 LOCKED 前,Band 1 产品轨不得启动。修改已 LOCKED 项须走 EXECUTION-STRATEGY §3.4 修订协议。

| # | 契约类别 | Owner | 冻结引用(文件/符号) | 状态 |
|---|---------|-------|---------------------|------|
| 1 | HTTP 契约(OpenAPI operation/schema) | 地基 | 待填(沿用 31.2 surface) | ☐ PENDING |
| 2 | gRPC / proto 接口与消息 | 地基 | 待填(沿用 31.2 protoc) | ☐ PENDING |
| 3 | Tenant 上下文签名(actor/org 传递) | 地基 | 待填(Go 类型+签名) | ☐ PENDING |
| 4 | Relay client 接口(Complete/Stream/Embed + usage/quota/价格快照) | 地基 | 待填(Go interface) | ☐ PENDING |
| 5 | 事件 schema(topic/payload,如使用) | 地基 | 待填 | ☐ PENDING |
| 6 | DB schema 归属(表 → track) | 地基 | 待填(表清单+归属) | ☐ PENDING |

## 冻结判据

- 每项须指向具体文件/符号(不是描述),并有对应 E1/E2 证据。
- 第 3、4 项须被至少一条产品轨的最小竖切实际消费一次,证明接口可用。
- 全部 6 项 `LOCKED` 后,在本文件顶部记 `Contract Lock #1 achieved: <日期/commit>`。
````

- [ ] **Step 2: 验证六类契约齐全**

Run: `grep -c "☐ PENDING" .planning/contracts-lock.md`
Expected: 6

- [ ] **Step 3: Commit(需授权)**

```bash
git add .planning/contracts-lock.md
git commit -m "docs(planning): add Band 0 contracts-lock template"
```

---

### Task 5: 给 ROADMAP 加 Band/Track 标注 【走 GSD】

**Files:**
- Modify(经 GSD): `.planning/ROADMAP.md`

**Interfaces:**
- Consumes: EXECUTION-STRATEGY 的 band/track 定义。
- Produces: ROADMAP 上每个 Phase 32–39 可见其波段与 track 归属。

> **执行方式:** 这是 GSD 核心文件。通过 `/gsd-quick`(描述:「给 ROADMAP Phase 32–39 各加一行 Band/Track 标注,不改编号与内容」)执行,或经用户显式授权后直接编辑。标注内容如下。

- [ ] **Step 1: 在 ROADMAP 每个 Phase 标题块下插入一行标注**

按下表,在每个 `### Phase NN:` 的 `**Goal**:` 上方插入一行:

| Phase | 插入行 |
|-------|-------|
| 32 | `**Band/Track**: Band 0 地基 · 横切(IDEN/SECU),不分轨` |
| 33 | `**Band/Track**: Band 0 地基 · 横切耐久执行原语(STOR/SECU-04,05 + AUTO 持久化底座),不分轨` |
| 34 | `**Band/Track**: Band 1 并发 · Track A(RLAY-*, CHAT-*)` |
| 35 | `**Band/Track**: Band 1 并发 · Track B(KNOW-01/05/07)+ C(AUTO-01/04/06/09/11)+ D(ADMN-01/02/03/05/06)+ F(CHAN-*)` |
| 36 | `**Band/Track**: Band 1 并发 · Track E(BILL-*, MRKT-*, ADMN-04)` |
| 37 | `**Band/Track**: Band 2 收口 · 观测/恢复(OPER-01..04,07)` |
| 38 | `**Band/Track**: Band 2 收口 · 部署对等(OPER-05,06,08)` |
| 39 | `**Band/Track**: Band 2 收口 · 供应链/发布(RELS-*)` |

> **正交性说明(写入该标注段前的一句注解):** Phase 是横向能力层,Track 是垂直模块;二者正交。Band 1 实际并发**以 Track 为单位跨 Phase 聚合**——例如 Track C(Automation)的需求分布在 Phase 33(AUTO 持久化底座)与 Phase 35(AUTO 产品能力),该 track 的 owner 需跨这两个 phase 聚合本域需求。

- [ ] **Step 2: 验证八个 phase 均已标注**

Run: `grep -c "\*\*Band/Track\*\*:" .planning/ROADMAP.md`
Expected: 8

- [ ] **Step 3: 验证未改编号/需求映射**

Run: `grep -c "Coverage:\|101 total" .planning/ROADMAP.md; git diff --stat .planning/ROADMAP.md`
Expected: 只有新增标注行,无 Phase 编号或 Requirements 行删改

- [ ] **Step 4: 提交(由 GSD 流程处理或授权后)**

```bash
git add .planning/ROADMAP.md
git commit -m "docs(roadmap): annotate phases with band/track overlay (no renumber)"
```

---

### Task 6: REQUIREMENTS 新增数据权利 v1 需求 【走 GSD】

**Files:**
- Modify(经 GSD): `.planning/REQUIREMENTS.md`

**Interfaces:**
- Consumes: 用户已拍板的「数据导出 + 账户/组织删除最小版进 v1」。
- Produces: 两条新 v1 需求 + traceability 映射;EXECUTION-STRATEGY §5 triage 表对应行更新为「已覆盖 + ID」。

> **执行方式:** GSD 核心文件,通过 `/gsd-quick`(描述:「新增两条 v1 数据权利需求并更新 traceability 与 coverage 计数」)执行,或经用户显式授权后直接编辑。

- [ ] **Step 1: 在 REQUIREMENTS「Identity And Tenancy」段末尾新增账户删除需求**

```markdown
- [ ] **IDEN-10**: 用户可以删除自己的账户,Owner 可以删除组织;删除会移除或匿名化关联数据、使会话与令牌立即失效,且不留部分可用身份。
```

- [ ] **Step 2: 在「Operations And Deployment」段末尾新增数据导出需求**

```markdown
- [ ] **OPER-09**: 组织 Owner/Admin 可以把本组织核心数据(成员、会话、知识库元数据、账单记录)导出为可移植格式,导出仅含本组织租户数据。
```

- [ ] **Step 3: 更新 Traceability 表(新增两行)**

在 Traceability 表相应位置插入:

```markdown
| IDEN-10 | Phase 32 | Pending |
| OPER-09 | Phase 37 | Pending |
```

- [ ] **Step 4: 更新 Coverage 计数**

将 `v1 requirements: 101 total` → `103 total`,`Mapped to phases: 101` → `103`。

- [ ] **Step 5: 同步 EXECUTION-STRATEGY §5 triage 表**

把「数据导出 / 账户删除(最小版)」行的说明改为:`已覆盖 → IDEN-10(删除)、OPER-09(导出)`。

- [ ] **Step 6: 验证**

Run: `grep -c "IDEN-10\|OPER-09" .planning/REQUIREMENTS.md`
Expected: ≥ 4(各出现在需求段与 traceability 段)
Run: `grep -c "103 total" .planning/REQUIREMENTS.md`
Expected: 1

- [ ] **Step 7: 提交(由 GSD 流程处理或授权后)**

```bash
git add .planning/REQUIREMENTS.md .planning/EXECUTION-STRATEGY.md
git commit -m "docs(requirements): add v1 data-rights needs (IDEN-10, OPER-09)"
```

---

## Self-Review(计划作者自查结果)

**1. Spec coverage:** spec §3→Task 1/2;§4 波段→Task 3(EXECUTION §1)+Task 5;§5 并发机制→Task 3(EXECUTION §3);§6 优先级→Task 3(EXECUTION §4);§7 triage→Task 3(EXECUTION §5)+Task 6;§8 交付物→全部 Task 覆盖。无遗漏。

**2. Placeholder scan:** 计划内 code 步骤均给实际内容;contracts-lock 的「待填」是有意的 Band 0 执行时动作(该文件本就是模板),非计划占位符。

**3. Type/naming consistency:** `Band 0/1/2`、`Track A–F`、六类契约、`IDEN-10`/`OPER-09`、文件路径在 Task 3/4/5/6 间一致;phase↔track 正交性在 Task 5 显式说明,避免执行者误按 phase=track。

**4. GSD 边界一致:** Task 1–4 直接编辑(根级/新增覆盖层);Task 5–6 标注为【走 GSD】。与 Global Constraints 一致。

## Execution Handoff

见对话中的执行方式选择。
