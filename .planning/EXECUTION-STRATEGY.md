# Execution Strategy — 地基先行 + 多模块并发

> 本文件是 `.planning/ROADMAP.md` 的**执行覆盖层**，不改 Phase 编号，只定义：
> 阶段如何归入波段、产品如何切分并发轨道、多 agent 如何锁契约协作、优先级如何排。
> 事实源仍是 PROJECT / REQUIREMENTS / ROADMAP / STATE；本文件回答「怎么并发推进」。

## 1. 波段模型 (Bands)

现有 Phase 32–39 映射到三个波段（映射，非重编号）：

```
Band 0 — 地基（串行，先锁）        映射 Phase 32 + Phase 33
  身份 / 租户 / 可信 actor 上下文 · 共享出站安全（SECU-01..03）
  · 耐久执行原语（job/lease/retry/dead-letter）· 共享对象存储（STOR）· Sandbox 隔离（SECU-04）
  · Relay 权威核心【契约部分】（RLAY client 接口 / usage / quota 快照语义，从 Phase 34 前移「契约冻结」）
        │
        ▼  ── Contract Lock #1（冻结跨轨共享契约，见 §3.1）──
        ▼
Band 1 — 产品轨道（并发，各自端到端竖切）  映射 Phase 34 + 35 + 36
  Track A  Chat + Relay 流式主链         （RLAY-*, CHAT-*）
  Track B  Knowledge / RAG               （KNOW-*）
  Track C  Agent / Workflow / Task       （AUTO-*）
  Track D  Admin / 治理                  （ADMN-*）
  Track E  账务 / Marketplace / payout   （BILL-*, MRKT-*）
  Track F  渠道                          （CHAN-*）
        │
        ▼  ── 集成门（对真实地基跑跨轨 E2E，见 §3.3）──
        ▼
Band 2 — 横切收口（串行）          映射 Phase 37 + 38 + 39
  观测 / SLO / 恢复（OPER-01..04,07）· 部署模式对等（OPER-05,06,08）· 供应链 + 目标发布（RELS-*）
```

**地基必须串行的理由：** Track A–F 全部依赖统一 tenant 上下文与 Relay 契约。地基未锁就并发，各轨会各自发明租户传递与 Provider 调用，导致跨租户泄漏或绕过计费——正是 REQUIREMENTS 要防的。

**产品轨可并发的理由：** 各轨拥有独立领域 package、路由前缀、前端路由、worker 与测试；共享面已在 Contract Lock #1 冻结，故轨间改动不互相阻塞。

## 2. 轨道定义 (Tracks)

每条轨道自持一条端到端竖切：Go 领域 + HTTP/gRPC + 前端 + worker + 测试 + E2 证据。

| Track | 需求归属 | Go 边界（Band 0 后精确回填） | 前端边界（Band 0 后精确回填） | 路由前缀（示意） |
|-------|---------|------------------------------|-------------------------------|-----------------|
| A Chat | RLAY-*, CHAT-* | `internal/chat`, relay stream pkg | `.../chat` | `/api/chat`, `/v1/chat` |
| B Knowledge | KNOW-* | `internal/knowledge` | `.../knowledge` | `/api/knowledge` |
| C Automation | AUTO-* | `internal/agent`, `internal/workflow`, `internal/task` | `.../agents`, `.../workflows` | `/api/agents` … |
| D Admin | ADMN-* | `internal/admin` | `.../admin` | `/api/admin` |
| E Finance | BILL-*, MRKT-* | `internal/billing`, `internal/marketplace` | `.../console`, `.../marketplace` | `/api/billing` … |
| F Channels | CHAN-* | `internal/channel` | `.../channels` | `/api/channels` |

> Go / 前端边界为示意。Band 0 结束时须用 `ls src/server/internal src/web/src` 按代码现状精确划定并回填本表。
> 共享地基代码（tenant、relay 接口、middleware）在 Band 0 后**只读**；需修改走 §3.4 修订协议。

**Phase ↔ Track 正交性：** Phase 是横向能力层，Track 是垂直模块，二者正交。Band 1 并发**以 Track 为单位跨 Phase 聚合**——例如 Track C（Automation）的需求分布在 Phase 33（AUTO 持久化底座）与 Phase 35（AUTO 产品能力），该 track owner 须跨这两个 phase 聚合本域需求。

## 3. 并发协作机制 (Concurrency Protocol)

### 3.1 契约先锁 (Contract Lock)

Band 1 并发启动**前**，必须冻结以下跨轨共享契约，作为 Band 0 显式交付物（清单落在 `contracts-lock.md`）：

1. **HTTP 契约** — OpenAPI operation id / 路径 / 请求响应 schema（沿用 Phase 31.2 surface parity）。
2. **gRPC / proto** — 服务间接口与消息（沿用 31.2 pinned protoc + 唯一归属）。
3. **Tenant 上下文签名** — 可信 actor/organization 在 HTTP / gRPC / job / retry / vector / analytics 的传递类型与函数签名。
4. **Relay client 接口** — 产品轨调用 Relay 的唯一 Go 接口（Complete / Stream / Embed + usage / quota / 价格快照语义）。
5. **事件 schema** — 若使用异步事件，topic 与 payload schema。
6. **DB schema 归属** — 每张表归属哪条轨道；跨轨只读访问走接口，不直连表。

### 3.2 轨道文件边界 (Track Ownership)

把 GSD 现有 plan 级 `file-disjoint may run in parallel` 提升到**轨道级**。每轨独占 §2 声明的 package / 目录；agent 不越界改他轨文件。冲突处（如共享 router 注册）集中在地基，Band 0 后只读。

### 3.3 集成门 (Integration Gate)

每条轨完成一个可验证竖切后，在集成门做：

- 对**真实地基**（非 mock 的 tenant / relay / 存储）集成，而非只跑单轨单测。
- 跑**跨轨 E2E**：如 A+E（Chat 调用产生 usage → 计费 ledger）、C+B（Agent 用 Knowledge 检索）。
- 校验 Phase 31.2 contract surface parity 未漂移。
- 任一轨未过集成门，不进入下一轮并发。

### 3.4 契约修订协议 (Contract Amendment)

某轨须改已锁契约时：

1. 暂停涉及的跨轨依赖。
2. 走轻量 GSD quick 记录 amendment（改什么、影响哪些轨）。
3. 更新 `contracts-lock.md` 与受影响 OpenAPI / proto。
4. 广播到所有相关轨后再继续。

**禁止「先改再说」** — 这是并发下契约漂移的头号来源。

## 4. 优先级护栏 (Priority Guardrails)

- **P1 发布工程元工作时间盒化：** verifier 严格度、digest、surface parity、O_NOFOLLOW 类加固等「证明如何发布」的工作限定投入，不得阻塞产品轨道。达到「足够 fail-closed + 可验证」即止。
- **P2 证据分级推进：** 产品轨按 **E2**（仓库运行时）证据即可推进并标记轨内完成；**E3 / E4** 外部证据统一在 Band 2 收口，不在产品轨内反复追求。
- **P3 完整优先于目录数量：** 生命周期完整度和证据优先于「100+ provider / 150+ tool」的数量堆砌（沿用 REQUIREMENTS Out-of-Scope）。
- **P4 每轨定义「最小可用竖切」：** 先打通一条端到端真实旅程（哪怕只支持 1 provider / 1 种文档），再横向加广度。

## 5. 商业打磨覆盖 triage

| 维度 | 结论 | 说明 |
|------|------|------|
| 首启引导 / 空状态 | 补 v1 | 基本可用性；走 GSD 补需求条目 |
| 统一错误文案体系 | 补 v1 | CHAT-05 / RLAY-03 有可操作错误，但缺跨模块统一语义规范 |
| 无障碍 WCAG | 归 v2（已定） | 用户确认 v1 不做无障碍达标 |
| 响应式 / 移动端 | 归 v2（已定） | 与 CHAT-20 一致，用户确认归 v2 |
| i18n（中英） | 归 v2（已定） | 用户确认 v1 不做 i18n |
| 配额耗尽 / 限流 UX | 已覆盖 | CHAT-05、BILL-05；确认前端呈现闭环即可 |
| 数据导出 / 账户删除（最小版） | 补 v1（已定）→ IDEN-10 + OPER-09 | 用户已定，合规底线；走 GSD 新增需求条目（见 Task 6） |
| 审计可观测运营视图 | 已覆盖 | ADMN-04/05、OPER-01 |

> 「补 v1」项经 GSD 正式加入 REQUIREMENTS 后，此表对应行更新为「已覆盖 + 需求 ID」。
