# Oblivious 项目执行进度评估与后续推进方案

日期：2026-04-06
状态更新：2026-04-07

参考基线：

- `docs/reports/2026-04-04-progress-plan.md`
- `docs/superpowers/specs/2026-04-06-m0-baseline-freeze-closeout-design.md`
- `docs/superpowers/specs/2026-04-07-m1-mainline-runnable-closeout-design.md`
- `docs/architecture/current-system-contracts.md`

## 0. 结论摘要

当前项目已经完成 `M0 基线冻结`，并进入 `M1 Mainline Runnable Closeout` 的最终收口阶段。

更具体地说，项目当前所处阶段可以定义为：

- **产品阶段**：后端 MVP 已成型，前端主线已进入可运行收口阶段
- **执行阶段**：`M0` 已完成，`M1` 收口进行中
- **交付阶段**：主线已跨过“不可构建、不可访问”的阶段，剩余工作集中在 warning 清零与正式验收结论

综合判断：

- **代码资产沉淀度**：约 `65%-70%`
- **可交付准备度**：约 `55%-60%`
- **既定计划执行完成度**：约 `45%-50%`

当前阶段的核心工作已经从“主线不可构建、不可交付”收敛为：

- 固化 `M1` 验收证据
- 把历史执行评估与路线图文档追平到当前代码现实
- 为进入 `M2` 做范围与资源准备

## 1. 评估依据与校验结果

### 1.1 评估口径

本次评估采用三层口径：

1. **计划执行口径**：对照 `2026-04-04-progress-plan` 的 Phase 0 至 Phase 4 里程碑
2. **实现落地口径**：对照 `2026-04-04-project-functional-completion` 的 Task 1 至 Task 12
3. **动态验证口径**：以 2026-04-06 当日实际构建/测试结果确认审计结论是否仍成立

### 1.2 当日动态验证

| 检查项 | 命令 | 结果 | 结论 |
| --- | --- | --- | --- |
| 前端构建 | `pnpm --dir src/web build` | 通过 | 主线前端已可稳定构建 |
| 前端测试 | `pnpm --dir src/web test` | 76 项全部通过 | 当前主要收口目标转为 `0 warning` 验收与测试门禁固定 |
| 后端测试 | `bash scripts/check.sh` / `bash scripts/test.sh` | 通过 | server unit tests 通过；integration 在未提供 `TEST_DATABASE_URL` 时显式 skip |

### 1.3 核心静态证据

| 范围 | 当前事实 | 对应计划任务 |
| --- | --- | --- |
| `docs/architecture/current-system-contracts.md` | 已存在并对齐当前主线 | `M0` 已完成 |
| root `scripts/check.sh` / `scripts/test.sh` | 已形成主线统一验证入口 | `M0` 已完成 |
| `src/web/src/app/providers.tsx` | 已接入真实 `AppContextProvider` | `M1` 关键契约已收敛 |
| `src/web/src/app/router.tsx` | 已接入 `/knowledge`、`/solo` 与 `ProtectedRoute` | `M1` 关键路由已打通 |
| `src/web/src/services/http/client.ts` | 已提供 `get/post/put/delete` 与 envelope 解包 | 前端 API 契约已统一 |
| `src/web/src/routes/workspace/*` | Chat / Onboarding / Settings / Knowledge / SOLO 已进入可运行态 | `M1` 路由验收进入收口阶段 |
| `src/web/src/routes/console/*` | Console 首页与子页已具备可运行 workbench 形态 | 主线控制台已从占位态提升为可验收态 |
| `config/.env.example` | 已按当前真实消费字段更新 | 环境变量契约已冻结 |

## 2. 当前实际执行进度

### 2.1 按里程碑评估

| 里程碑 | 原计划日期 | 当前状态 | 完成度 | 判断 |
| --- | --- | --- | ---: | --- |
| M0 基线冻结 | 2026-04-10 | 已完成 | 100% | workspace、root 入口、契约文档与治理模板已冻结并合入主线 |
| M1 主线可运行 | 2026-04-24 | 已完成 | 100% | `src/web build`、关键路由、root `check/test` 与前端 `0 warning` 验收已经闭环 |
| M2 工作区 Beta | 2026-05-15 | 进行中 | 25% | Knowledge 已完成首个 Beta 子项目收口，其余 Chat / SOLO / Console 闭环仍待推进 |
| M3 能力 Beta | 2026-06-05 | 未开始 | 5% | advanced retrieval、SOLO runtime、Chat streaming 仍未展开 |
| M4 RC 候选版 | 2026-06-19 | 未开始 | 0% | CI、质量门禁、安全基线、发布清单尚未建立 |

说明：

- 上表的“完成度”是按**里程碑退出条件**计算，而不是按仓库已有代码量计算。
- 当前代码中虽存在部分后续阶段的原型页面或模型，但由于前置里程碑未满足，不应被误判为“已进入下阶段执行”。

### 2.2 按能力域评估

| 能力域 | 当前状态 | 估算完成度 | 说明 |
| --- | --- | ---: | --- |
| 后端核心业务壳 | 已实现 MVP | 70%-75% | auth、preferences、chat、knowledge CRUD、SOLO starter、console 已有真实代码 |
| 前端基础契约 | 已收敛 | 90%-95% | AppContext、auth bootstrap、API 类型与 HTTP envelope 已对齐当前主线 |
| 前端工作区页面 | 主线可运行 | 75%-80% | Chat / Knowledge / SOLO / Settings / Onboarding 已具备 M1 验收所需可访问与可回归能力 |
| 前端控制台 | 主线可运行 | 70%-75% | Console 首页与子页已形成可验收 workbench 结构 |
| 文档与契约治理 | 已冻结并收口 | 75%-80% | 系统契约、环境变量、M0/M1 进度文档已追平到当前现实 |
| 测试、CI、工程门禁 | 基线可用 | 55%-60% | root `check/test` 稳定可跑，前端 warning 已清零，后续重点转向 M2/M4 工程加固 |

### 2.3 按实施计划 Task 评估

| Task | 名称 | 状态 | 完成度 | 判断 |
| --- | --- | --- | ---: | --- |
| Task 1 | Freeze Contracts And Documentation | 已完成 | 100% | 契约文档、环境变量文档、治理模板与 root 入口已冻结 |
| Task 2 | Rebuild Frontend App Context And Auth Bootstrap | 已完成 | 95% | AppContext、auth bootstrap 与相关测试已进入稳定可回归状态 |
| Task 3 | Normalize HTTP Client And Frontend API Contracts | 已完成 | 90% | `HttpClient` 与前端类型已支撑当前主线接口 |
| Task 4 | Wire Protected Workspace Routes And Layouts | 已完成 | 90% | 关键工作区路由与权限守卫已接通 |
| Task 5 | Implement Workspace Chat / Onboarding / Settings | 已完成 | 80% | 已达到 `M1` 的可访问、可构建、可回归标准 |
| Task 6 | Finish Knowledge Workspace CRUD Integration | 已完成 Beta 收口 | 100% | Knowledge CRUD、retrieval 结果质量、页面回归与文档说明已进入 Beta 可依赖状态 |
| Task 7 | Finish SOLO Starter Workspace Integration | 已完成 M1 范围 | 75% | SOLO starter 已进入主线可运行态，后续 runtime 能力转入 `M2/M3` |
| Task 8 | Implement Console Dashboard And Child Pages | 已完成 M1 范围 | 75% | Console 首页与子页已可访问、可测试、可验收 |
| Task 9 | Harden Backend Runtime Configuration And Testability | 部分完成 | 60% | root 验证入口已稳定，后续重点是 `TEST_DATABASE_URL` 场景的环境复现 |
| Task 10 | Deliver Knowledge Retrieval MVP | 已完成 M2-A 范围 | 40% | 当前文本 retrieval 已稳定进入 Beta，但更重的 ingestion / indexing / M3 检索能力尚未展开 |
| Task 11 | Replace SOLO Starter With Runtime MVP | 未启动 | 5% | 只有 starter 状态机原型 |
| Task 12 | Add Quality Gates, CI, And Release Checks | 未启动 | 5% | 无 CI 基线，脚本契约仍错误 |

## 3. 项目所处具体阶段判断

### 3.1 阶段定义

当前项目更准确的定义是：

> **`M0` 已完成、`M1 主线可运行` 已达成、正准备进入 `M2 工作区 Beta` 的 Phase 1.5 阶段**

这一阶段的特征是：

- 主线路由、契约与 root 验证入口已经稳定
- 前端测试已可在 warning-free 口径下回归
- 工程与文档基线已足以支撑后续 `M2` 开发
- 下一阶段重点从“基础收敛”切换到“用户流闭环”

### 3.2 阶段退出条件

当前主线已经满足进入 `Phase 2 / M2` 的前置条件：

1. `M0` 的 workspace、contracts、governance 已冻结
2. `src/web build` 与 root `check/test` 均通过
3. 关键工作区与控制台路由可访问
4. 前端测试在当前主线下已达到 `0 warning`

因此，后续工作不应再回到 `M1` 基础收敛，而应转入 `M2` 的用户流闭环与 `M4` 之前的工程加固。

## 4. 当前偏差与关键阻塞

### 4.1 与既定计划的偏差

| 计划要求 | 当前事实 | 偏差性质 |
| --- | --- | --- |
| M0 先冻结文档、接口、环境变量与 owner | 只完成了分析报告和 backlog，缺正式契约文档与 owner 落位 | 执行偏差 |
| Phase 1 之前先收敛前端基础契约 | 当前前端仍同时存在占位页、类型缺失、路由缺口 | 关键路径未启动 |
| 测试环境要逐步可复现 | 后端 `internal/http` 仍绑本地 PostgreSQL | 工程化偏差 |
| 文档要替代过期 Task 5 基线 | 仍无 Task 6/7 正式设计补档 | 架构基线偏差 |

### 4.2 当前关键阻塞项

| 阻塞项 | 对应 backlog | 影响面 | 优先级 |
| --- | --- | --- | --- |
| root workspace 漂移与 lockfile 策略未收敛 | `K-01` | 安装、CI、多人协作 | 关键 |
| AppContext / AuthStore / bootstrap 未收敛 | `K-02`、`K-03` | 所有受保护页面 | 关键 |
| API 类型与 HTTP envelope 不一致 | `K-04`、`K-05`、`K-06` | Chat / Knowledge / SOLO / Console | 关键 |
| 路由与权限守卫未完成 | `K-07` | 页面不可达、伪进展 | 关键 |
| 构建脚本与 check 基线损坏 | `K-10` | 无法建立统一验证入口 | 关键 |
| 后端集成测试依赖本地 DB | `K-11` | 无法建立稳定回归门禁 | 关键 |
| 设计文档与环境变量契约过期 | `K-12`、`K-13` | 所有开发、联调、部署活动 | 关键 |

## 5. 后续推进计划

## 5.1 推进总原则

未来推进必须遵循四条铁律：

1. **先收敛关键路径，再扩展高级能力**
2. **以后端现有接口为事实来源，前端先适配而不是反向重构**
3. **所有接口变更必须同步更新文档、类型和测试**
4. **任何超出 M1/M2 的范围变更，必须经过 TL 明确批准**

## 5.2 关键里程碑时间表

在维持原目标日期不变的前提下，建议采用以下执行排期：

| 阶段 | 日期 | 负责人 | 核心 backlog | 退出条件 |
| --- | --- | --- | --- | --- |
| Phase 0 | 2026-04-06 至 2026-04-10 | TL | `K-01`、`K-10`、`K-12`、`K-13` | contract matrix、env matrix、owner、workspace 策略冻结 |
| Phase 1 | 2026-04-13 至 2026-04-24 | FE | `K-02` 至 `K-07` | `src/web build` 通过，关键工作区路由可访问 |
| Phase 2 | 2026-04-27 至 2026-05-15 | FE + BE | `K-08`、`K-09`、`K-11` | Chat / Knowledge CRUD / SOLO starter / Settings / Console 跑通 |
| Phase 3 | 2026-05-18 至 2026-06-05 | BE | `I-04`、`I-05`、`I-06` | retrieval MVP、runtime MVP、chat gateway 增强可演示 |
| Phase 4 | 2026-06-08 至 2026-06-19 | TL + QA + OPS | `I-01` 至 `I-03`、`G-*` | CI、文档、安全、发布流程齐备 |

### 5.3 未来两周的详细执行清单

#### 2026-04-06 至 2026-04-10：M0 基线冻结周

必须完成：

- 明确 `new-api` / `lobehub` 的 workspace 角色，关闭主线安装漂移
- 输出 `current-system-contracts` 文档
- 补 Task 6/7 设计说明或历史说明，停止继续引用过期 Task 5 作为当前基线
- 统一 `.env.example` 与 `config.Load()` 的真实契约
- 建立 owner 映射与周报模板

禁止事项：

- 不启动 retrieval
- 不启动 SOLO runtime
- 不扩展新的页面需求
- 不新增未纳入 backlog 的控制台功能

#### 2026-04-13 至 2026-04-17：M1 第一周

必须完成：

- `AppProviders` / `useAppContext`
- `AuthStore`、`useAuthBootstrap` 契约收敛
- `types/api.ts` 主线类型补齐
- `HttpClient` 的 `put/delete` 与 envelope 解包

阶段验收：

- auth/store 相关测试由红转绿
- `src/web build` 中与 store/context/client/types 相关错误清零

#### 2026-04-20 至 2026-04-24：M1 第二周

必须完成：

- 路由接入 `/knowledge`、`/solo`、`ProtectedRoute`
- 收敛 `ChatApi`、`ConsoleApi`、`AuthApi`
- layout 测试与 router 测试通过

阶段验收：

- `/chat`、`/knowledge`、`/solo`、`/settings`、`/console` 可访问
- 前端构建通过
- 前端失败测试数下降到 `0-3` 以内，且不得再出现类型级系统性失败

### 5.4 里程碑滑移预案

为防止“计划仍写原日期、执行已经失控”，建议加入三条滑移规则：

1. 若 `M0` 在 **2026-04-14** 前未完成，则 `M1` 与 `M2` 各顺延 1 周
2. 若 `M1` 在 **2026-04-28** 前未完成，则 `M3` 必须缩 scope，只保留 retrieval 或 runtime 之一
3. 若 `K-11` 在 **2026-05-15** 前未完成，则禁止宣称项目进入 RC 准备阶段

## 6. 资源分配方案

### 6.1 最小可执行编制

建议继续采用原计划的人力假设：

| 角色 | 数量 | 核心职责 |
| --- | ---: | --- |
| TL | 1 | 范围控制、架构裁决、里程碑验收、风险升级 |
| FE | 1 | `src/web` 状态层、API 层、路由、页面收敛 |
| BE | 1 | `src/server` 测试环境、接口稳定性、后续 retrieval/runtime |
| QA | 1 | 用例、回归、缺陷门禁、发布签署 |
| OPS | 0.5 | CI、测试环境、日志与发布流水线 |

如果实际只有 2 名开发者，则整体排期需要**保守顺延 2-4 周**。

### 6.2 按阶段投入比例

| 阶段 | TL | FE | BE | QA | OPS |
| --- | ---: | ---: | ---: | ---: | ---: |
| Phase 0 | 70% | 40% | 40% | 30% | 20% |
| Phase 1 | 35% | 100% | 30% | 50% | 10% |
| Phase 2 | 30% | 100% | 60% | 60% | 10% |
| Phase 3 | 40% | 70% | 100% | 60% | 20% |
| Phase 4 | 50% | 50% | 60% | 100% | 50% |

### 6.3 工作包 owner 映射

| 工作包 | owner | 协作 |
| --- | --- | --- |
| workspace 策略与文档基线 | TL | BE、FE |
| 前端状态层与契约层 | FE | TL |
| 前端页面与布局收敛 | FE | BE、QA |
| 后端测试与配置基线 | BE | TL、OPS |
| 能力升级（retrieval/runtime/gateway） | BE | FE、TL |
| 回归、缺陷、发布门禁 | QA | FE、BE、OPS |

## 7. 风险防控措施

| 风险 | 级别 | 预警信号 | 防控措施 | owner |
| --- | --- | --- | --- | --- |
| workspace 边界不清继续拖慢主线 | 高 | `pnpm install` 仍受嵌入仓阻断 | Phase 0 内完成隔离或正式纳管策略 | TL |
| 前端契约继续扩散 | 高 | build 报错类型继续增加 | 把 `K-02` 至 `K-07` 设为绝对阻塞项，禁止插入新功能 | FE |
| 文档继续落后于实现 | 高 | 接口变更后未更新文档/类型 | 增加“接口变更三联动”门禁：文档 + 类型 + 测试 | TL |
| SOLO / Knowledge 范围膨胀 | 高 | 开始讨论多 agent、向量平台、复杂 orchestration | 严格限定为 MVP：文本 retrieval + 受限 runtime | TL、BE |
| 后端测试环境不可复现 | 高 | `internal/http` 长期依赖本地 DB | `K-11` 前置到 M2 前半段，缺环境时显式 skip | BE |
| 安全项后置到发布前 | 中高 | 登录、Cookie、CSRF、限流长期无人处理 | 从 M2 起建立安全清单，每周 review 一次 | TL、BE |
| 人员单点瓶颈 | 中 | 某模块只有一个人理解 | 关键模块至少双人 review，文档先行 | TL |

## 8. 质量保障机制

### 8.1 质量门禁分层

#### 开发门禁

- 修改前端代码必须通过对应测试与 `pnpm --dir src/web build`
- 修改后端代码必须通过对应 package `go test`
- API 变更必须同步更新：
  - contract 文档
  - `src/web/src/types/api.ts`
  - 至少一条对应测试

#### 合并门禁

- 不允许引入新的 TypeScript 编译错误
- 不允许增加新的失败测试
- 不允许绕过根脚本与约定的验证命令

#### 里程碑门禁

| 里程碑 | 最低质量标准 |
| --- | --- |
| M0 | contract matrix、env matrix、owner、risk list 全部存在且可执行 |
| M1 | `src/web build` 通过，关键路由可访问，前端失败测试数归零 |
| M2 | 核心用户流全跑通，后端测试不再硬依赖本地固定 DB |
| M3 | retrieval/runtime/gateway 的 MVP 有可重复演示脚本 |
| M4 | CI、文档、安全清单、发布清单全部就位，P0/P1 缺陷清零 |

### 8.2 覆盖率与回归策略

现状覆盖率基线来自 2026-04-05 审计：

- `internal/config`: `87.8%`
- `internal/chat`: `40.8%`
- `internal/task`: `30.9%`
- `internal/http`: `22.0%`
- `internal/console`: `18.5%`
- `internal/knowledge`: `11.8%`

建议采用“增量提升”策略，而不是立即强设全仓统一阈值：

| 包 | 当前基线 | M2 建议门槛 | M4 建议门槛 |
| --- | ---: | ---: | ---: |
| `internal/config` | 87.8% | >= 85% | >= 85% |
| `internal/chat` | 40.8% | >= 45% | >= 50% |
| `internal/task` | 30.9% | >= 35% | >= 45% |
| `internal/http` | 22.0% | >= 30% | >= 40% |
| `internal/console` | 18.5% | >= 25% | >= 35% |
| `internal/knowledge` | 11.8% | >= 20% | >= 30% |

### 8.3 性能与稳定性保障

当前没有可靠性能基线，因此建议在 M2 结束前建立最小 benchmark 集：

- 聊天会话列表
- 消息发送接口
- 知识库与文档 CRUD
- SOLO 任务列表与详情
- Console usage 汇总

每次 milestone 验收至少输出一次：

- 接口耗时样本
- 慢查询列表
- 前端关键页面加载错误数

### 8.4 安全保障

M2 起至少纳入以下清单：

- Cookie `secure` / `sameSite` / `httpOnly` 复核
- CSRF 防护策略
- 登录限速
- 密码策略
- 输入校验与错误码映射
- 审计日志关键字段

## 9. 定期进度跟踪与报告机制

### 9.1 看板字段

所有 backlog 事项统一维护以下字段：

- ID
- 标题
- owner
- 优先级
- 状态：`未开始` / `进行中` / `联调中` / `待验收` / `已完成` / `阻塞`
- 截止日期
- 依赖项
- 验收标准
- 风险等级

### 9.2 固定会议节奏

| 节奏 | 时间 | 时长 | 目的 |
| --- | --- | ---: | --- |
| 每日站会 | 每天 10:00 | 15 分钟 | 同步昨日完成、今日计划、阻塞项 |
| 周计划会 | 每周一 14:00 | 45 分钟 | 校准本周目标、调整优先级 |
| 契约评审会 | 每周三 16:00 | 30 分钟 | 处理跨端接口和文档变更 |
| 周演示与缺陷分级会 | 每周五 17:00 | 60 分钟 | 演示里程碑增量，确认 P0/P1/P2 |

### 9.3 周报模板

建议固定使用以下周报结构：

```md
# 项目周报（YYYY-MM-DD ~ YYYY-MM-DD）

## 1. 里程碑状态
- M0/M1/M2/M3/M4：绿 / 黄 / 红

## 2. 本周计划 vs 实际
- 计划完成事项
- 延迟事项
- 未启动事项

## 3. 关键指标
- web build：通过 / 失败
- web test：通过率
- go test：通过包 / 失败包
- 阻塞项数量
- P0/P1 缺陷数量

## 4. 风险与偏差
- 新增风险
- 已解除风险
- 需要升级的问题

## 5. 下周计划
- 关键交付物
- owner
- 依赖
```

### 9.4 升级规则

满足任一条件，必须升级到 TL 或项目负责人处理：

1. 关键路径任务阻塞超过 `1` 个工作日
2. 里程碑偏差超过 `2` 个工作日
3. 新增 P0 或连续两天 build 红灯
4. 接口变更未同步文档和类型
5. 范围新增影响到当前 milestone 退出条件

## 10. 最终判断与执行建议

当前最重要的结论不是“功能还差多少”，而是：

> **项目已经完成 `M0` 基线冻结，并具备 `M1 主线可运行` 的实际代码基础；剩余工作是把测试输出、验收证据和历史文档全部收口到同一结论。**

因此后续推进必须按以下顺序执行：

1. 完成 `M1` 收口：warning 清零、验收证据固化、历史文档追平
2. 再进入 `M2`：Chat / Knowledge CRUD / SOLO starter / Settings / Console 用户流闭环
3. `M3` 能力升级继续保持在 `M2` 关闭后再展开
4. CI、安全、文档、发布清单仍需在 `M4` 前闭环

若本方案按原资源假设执行，并且 **M0 于 2026-04-10 前完成、M1 于 2026-04-24 前完成**，则原 `2026-06-19` 的 RC 候选目标仍然可守。  
若 M0 或 M1 继续滑移，则后续所有节点都应按本报告的滑移规则整体顺延，不应继续维持名义时间表。
