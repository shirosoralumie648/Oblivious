# M1 Mainline Runnable Closeout Design

日期：2026-04-07

## 1. Background

`M0 Baseline Freeze Closeout` 已经完成并合入 `main`，当前主线已经具备以下事实：

- root `check` / `test` / CI 入口已冻结
- `src/web` 可构建
- 工作区与控制台关键路由已接入
- 前端 `providers` / router / HTTP client / API types / auth bootstrap 已基本收敛

因此，`docs/reports/2026-04-06-execution-progress-review.md` 等历史材料中“`M0` 未完成”“`M1` 未开始”的判断，已经与当前代码现实不一致。

但当前主线仍缺一类关键收口工作：

- `bash scripts/test.sh` 虽然通过，但仍输出 React Router future flag warning
- 多个页面测试仍输出 `act(...)` warning
- 历史进度文档和路线图尚未追平到当前主线状态

这意味着项目在“技术上已接近 M1 达成”与“治理上尚未形成正式 M1 验收结论”之间，仍存在最后一段断层。

## 2. Goal

完成一个以“主线可运行正式验收”为目标的 `M1 Mainline Runnable Closeout` 里程碑，使当前主线在代码、测试输出和项目文档三层同时满足 `M1` 退出条件。

## 3. Non-Goals

本里程碑明确不做以下内容：

- 不新增 `M2` 或 `M3` 业务能力
- 不新增 retrieval、streaming、runtime 等高级能力
- 不扩展新的页面需求
- 不重构 Chat / Knowledge / SOLO / Console 的业务模型
- 不引入新的状态管理方案
- 不把 warning 清理扩展成大规模前端重构

## 4. Current-State Decision

本里程碑采用以下事实判定规则：

### 4.1 Code Reality Wins

对 `M1` 进度和退出条件的判断，以当前主线代码和最新验证结果为准，而不是以 2026-04-06 之前的审计结论为准。

### 4.2 Warning-Free Is Part Of M1 Closeout

本次 `M1` 收口不接受“测试通过但仍有 warning”的状态。

本里程碑的完成标准包含：

- `bash scripts/test.sh` 通过
- 测试输出为 `0 warning`

### 4.3 History Should Be Updated, Not Frozen Wrong

历史进度文档保留时间语境，但其中与当前主线现实明显冲突的阶段判断，必须追平到当前状态，不能继续保留失真的“当前判断”。

## 5. Scope

本里程碑范围收敛为三组交付。

### 5.1 Router And Test Warning Closeout

需要交付：

- React Router future flag warning 清零
- `act(...)` warning 清零
- 形成可持续的 zero-warning 测试门禁

### 5.2 M1 Acceptance Evidence Closeout

需要交付：

- `src/web build`、root `check/test` 的最新通过证据
- `/chat`、`/knowledge`、`/solo`、`/settings`、`/console` 的关键路由验收口径
- 当前前后端契约一致性的正式说明

### 5.3 Progress Documentation Closeout

需要交付：

- 现有路线图与执行评估文档追平到当前代码现实
- 形成明确的 `M1 已达成` 结论

## 6. Technical Approach

本里程碑采用“warning 根源收敛 + 文档追平”的方式，而不是“测试输出压制”或“只补一份新报告”。

### 6.1 Router Future Compatibility

React Router future flags 将在单一 router 入口集中配置，并同时应用于：

- browser router
- memory router

目标是使运行态与测试态共享同一套 future-compatible 行为，避免：

- 运行态无配置、测试态单独消音
- browser router 与 memory router 行为再次漂移

### 6.2 Test Stability By Real Async Boundaries

`act(...)` warning 的处理方式不是忽略控制台输出，而是把页面测试改成真实匹配异步更新边界的写法：

- 对存在 `useEffect` / data loading 的页面，使用稳定等待的断言方式
- router smoke tests 等待页面进入稳定态再断言
- 保留原有行为验证，不允许为了消 warning 降低测试强度

### 6.3 Zero-Warning Gate

测试基线将把未预期 `console.warn` / `console.error` 视为失败条件。

这样可以把本次里程碑收敛成一个可持续门禁，而不是一次性人工清理：

- React Router future warning 不能回归
- `act(...)` warning 不能回归
- 其他运行时 warning 也不能静默进入主线

### 6.4 Documentation Follows Verified Reality

文档更新必须发生在代码与验证完成之后。

更新顺序固定为：

1. 清理 warning
2. 重新运行 `bash scripts/check.sh`
3. 重新运行 `bash scripts/test.sh`
4. 再更新执行评估与路线图文档

这样文档记录的是已被验证的事实，而不是待验证的假设。

## 7. File Shape

本里程碑预期只改动 `M1` 收口相关入口与文档。

### 7.1 Frontend Runtime And Test Entry

预计涉及：

- `src/web/src/app/router.ts`
- `src/web/src/test/setup.ts`
- `src/web/src/app/router.test.tsx`
- `src/web/src/routes/workspace/*.test.tsx`
- `src/web/src/routes/console/*.test.tsx`
- 其他实际触发 warning 的布局或认证测试文件

### 7.2 Acceptance And Progress Docs

预计涉及：

- `docs/reports/2026-04-06-execution-progress-review.md`
- `docs/reports/2026-04-04-progress-plan.md`

如有必要，可新增一份 `M1 closeout` 验收文档，但前提是已有报告仍需同步追平，而不是把矛盾留在旧文档中。

## 8. Risks And Mitigations

### 8.1 False Silence Risk

风险：

- 为了做到 `0 warning`，把测试改成只会等待、不会验证行为

处理：

- 所有 warning 修复必须保留原始行为断言
- 只允许把同步断言改为稳定异步断言，不允许削弱断言内容

### 8.2 Routing Behavior Drift Risk

风险：

- future flags 启用后，局部相对路径解析语义发生变化

处理：

- 只围绕当前主线路由树验证
- 若行为发生变化，以当前主线可接受行为为准同步更新测试
- 不借机扩展新的路由结构

### 8.3 Historical Narrative Drift Risk

风险：

- 文档追平时把历史阶段判断改写成“从未发生过”

处理：

- 保留原始报告的时间背景
- 只更新其中的当前状态判断、阶段结论和后续时间表依据

## 9. Verification Model

本里程碑采用三层验证。

### 9.1 Root Quality Gates

必须通过：

- `bash scripts/check.sh`
- `bash scripts/test.sh`

### 9.2 Zero-Warning Verification

必须满足：

- `bash scripts/test.sh` 输出不包含 React Router future flag warning
- `bash scripts/test.sh` 输出不包含 `act(...)` warning
- 测试门禁本身可以阻止未来 warning 回归

### 9.3 Documentation Consistency Verification

下列资产必须一致：

- 当前主线验证结果
- `docs/reports/2026-04-06-execution-progress-review.md`
- `docs/reports/2026-04-04-progress-plan.md`
- `docs/architecture/current-system-contracts.md`

## 10. Completion Criteria

当以下条件同时满足时，本里程碑完成：

1. `bash scripts/check.sh` 通过
2. `bash scripts/test.sh` 通过且输出 `0 warning`
3. React Router future flag warning 清零
4. `act(...)` warning 清零
5. `/chat`、`/knowledge`、`/solo`、`/settings`、`/console` 具备明确验收证据
6. 历史进度/路线图文档追平到当前主线现实
7. 可以明确写出“`M1 主线可运行` 已达成”的正式结论

## 11. Recommended Outputs

本设计推荐在实现阶段优先落地以下输出：

1. router future-compatible 配置收敛
2. 页面与路由测试 warning 收敛
3. zero-warning 测试门禁建立
4. `M1` 验收证据整理
5. 历史进度和路线图文档更新

这样可以先把主线验证结果收成稳定门禁，再把项目管理口径追平到当前现实。
