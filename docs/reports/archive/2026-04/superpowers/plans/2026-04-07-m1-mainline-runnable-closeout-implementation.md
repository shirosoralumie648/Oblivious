# M1 Mainline Runnable Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让当前主线正式达成 `M1 主线可运行`：root `check/test` 通过、前端测试输出 `0 warning`、关键路由验收证据齐备、历史进度文档追平到当前代码现实。

**Architecture:** 以 React Router future flags 的单一配置源为入口，把测试 warning 从“运行时噪音”提升为“测试失败”，再按 router smoke tests、console tests、workspace tests 三层收敛异步断言方式。文档层只纳入并更新两份已有的 `docs/reports` 基线文件，不重写整套历史报告，只修改与当前阶段判断、里程碑状态和验收标准直接相关的段落。

**Tech Stack:** React 18, React Router 6.30, Vitest, Testing Library, Bash root quality gates, Markdown project reports

---

## File Structure

- Create: `src/web/src/app/routerFuture.ts`
  - 统一 browser router、memory router 和测试侧 `MemoryRouter` 的 future flags
- Modify: `src/web/src/app/router.tsx`
  - 使用共享 router future config
- Modify: `src/web/src/test/setup.ts`
  - 建立 zero-warning 测试门禁，把未预期 `console.warn/error` 视为失败
- Modify: `src/web/src/features/auth/ProtectedRoute.test.tsx`
  - 给 `MemoryRouter` 补 future config
- Modify: `src/web/src/app/router.test.tsx`
  - router smoke tests 改为等待稳定 UI 后断言
- Modify: `src/web/src/features/layouts/ConsoleLayout.test.tsx`
  - `createMemoryRouter` 补 future config，并等待控制台壳层稳定
- Modify: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
  - `createMemoryRouter` 补 future config，并等待工作区壳层稳定
- Modify: `src/web/src/routes/workspace/ChatPage.test.tsx`
  - router smoke tests 改为等待工作区和 console 页面稳定
- Modify: `src/web/src/routes/console/AccessPage.test.tsx`
  - `MemoryRouter` 补 future config，并把数据加载断言改成异步稳定写法
- Modify: `src/web/src/routes/console/BillingPage.test.tsx`
  - 同上
- Modify: `src/web/src/routes/console/ConsoleHomePage.test.tsx`
  - 同上
- Modify: `src/web/src/routes/console/ModelsPage.test.tsx`
  - 同上
- Modify: `src/web/src/routes/console/UsagePage.test.tsx`
  - 同上
- Modify: `src/web/src/routes/workspace/OnboardingPage.test.tsx`
  - `MemoryRouter` 补 future config，并保持行为断言不降级
- Modify: `src/web/src/routes/workspace/SoloPage.test.tsx`
  - 让任务加载和状态跳转断言等待稳定 UI
- Create: `docs/reports/2026-04-04-progress-plan.md`
  - 从当前主工作区未跟踪参考副本导入，并追平 `M0/M1` 状态与验收标准
- Create: `docs/reports/2026-04-06-execution-progress-review.md`
  - 从当前主工作区未跟踪参考副本导入，并追平阶段判断、里程碑表与最终结论

## Task 1: Add A Single Router Future Source And Zero-Warning Gate

**Files:**
- Create: `src/web/src/app/routerFuture.ts`
- Modify: `src/web/src/app/router.tsx`
- Modify: `src/web/src/test/setup.ts`
- Modify: `src/web/src/features/auth/ProtectedRoute.test.tsx`

- [ ] **Step 1: Write the failing zero-warning gate**

```ts
// src/web/src/test/setup.ts
import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach, vi } from 'vitest';

let consoleWarnSpy: ReturnType<typeof vi.spyOn>;
let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

function formatConsoleArgs(args: unknown[]) {
  return args
    .map((value) => (value instanceof Error ? value.stack ?? value.message : String(value)))
    .join(' ');
}

function failUnexpectedConsole(method: 'warn' | 'error', args: unknown[]) {
  throw new Error(`[unexpected console.${method}] ${formatConsoleArgs(args)}`);
}

beforeEach(() => {
  consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation((...args) => {
    failUnexpectedConsole('warn', args);
  });
  consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation((...args) => {
    failUnexpectedConsole('error', args);
  });
});

afterEach(() => {
  consoleWarnSpy.mockRestore();
  consoleErrorSpy.mockRestore();
});
```

- [ ] **Step 2: Run a focused warning check and verify it fails**

Run:

```bash
pnpm --dir src/web exec vitest run src/features/auth/ProtectedRoute.test.tsx src/app/router.test.tsx
```

Expected: FAIL with `unexpected console.warn` caused by React Router future flag warnings.

- [ ] **Step 3: Add a single future-config source and wire it into the router**

```ts
// src/web/src/app/routerFuture.ts
import type { FutureConfig } from 'react-router-dom';

export const routerFuture: FutureConfig = {
  v7_relativeSplatPath: true,
  v7_startTransition: true
};
```

```tsx
// src/web/src/app/router.tsx
import {
  createBrowserRouter,
  createMemoryRouter,
  type RouteObject
} from 'react-router-dom';

import { routerFuture } from './routerFuture';

// ...

export function createAppRouter(initialEntries?: string[]) {
  if (initialEntries && initialEntries.length > 0) {
    return createMemoryRouter(routes, { initialEntries, future: routerFuture });
  }

  return createBrowserRouter(routes, { future: routerFuture });
}
```

```tsx
// src/web/src/features/auth/ProtectedRoute.test.tsx
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { routerFuture } from '../../app/routerFuture';

// ...

render(
  <MemoryRouter future={routerFuture} initialEntries={['/chat']}>
    <Routes>
      <Route element={<ProtectedRoute />}>
        <Route element={<p>workspace</p>} path="/chat" />
      </Route>
      <Route element={<p>login</p>} path="/login" />
    </Routes>
  </MemoryRouter>
);
```

- [ ] **Step 4: Re-run the focused warning check**

Run:

```bash
pnpm --dir src/web exec vitest run src/features/auth/ProtectedRoute.test.tsx src/app/router.test.tsx
```

Expected: `ProtectedRoute.test.tsx` no longer emits React Router future warnings, but `router.test.tsx` still fails because route/page tests are too synchronous under the new console gate.

- [ ] **Step 5: Commit**

```bash
git add \
  src/web/src/app/routerFuture.ts \
  src/web/src/app/router.tsx \
  src/web/src/test/setup.ts \
  src/web/src/features/auth/ProtectedRoute.test.tsx
git commit -m "test(web): gate unexpected warnings in router tests"
```

## Task 2: Stabilize Router, Layout, And Chat Route Smoke Tests

**Files:**
- Modify: `src/web/src/app/router.test.tsx`
- Modify: `src/web/src/features/layouts/ConsoleLayout.test.tsx`
- Modify: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
- Modify: `src/web/src/routes/workspace/ChatPage.test.tsx`

- [ ] **Step 1: Run the route and layout smoke tests under the warning gate**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/app/router.test.tsx \
  src/features/layouts/ConsoleLayout.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx \
  src/routes/workspace/ChatPage.test.tsx
```

Expected: FAIL because `createMemoryRouter` calls still omit `future`, and synchronous `getBy*` assertions race page hydration and data-loading updates.

- [ ] **Step 2: Update `createMemoryRouter` tests to use shared future config and wait for stable UI**

```tsx
// src/web/src/features/layouts/ConsoleLayout.test.tsx
import { RouterProvider, createMemoryRouter } from 'react-router-dom';

import { routerFuture } from '../../app/routerFuture';

const router = createMemoryRouter(
  [
    {
      element: <ConsoleLayout />,
      children: [{ index: true, element: <p>console body</p> }]
    }
  ],
  { future: routerFuture, initialEntries: ['/console'] }
);

render(<RouterProvider router={router} />);

expect(await screen.findByText('Console')).toBeInTheDocument();
expect(screen.getByText('console body')).toBeInTheDocument();
```

```tsx
// src/web/src/features/layouts/WorkspaceLayout.test.tsx
import { RouterProvider, createMemoryRouter } from 'react-router-dom';

import { routerFuture } from '../../app/routerFuture';

const router = createMemoryRouter(
  [
    {
      element: <WorkspaceLayout />,
      children: [{ index: true, element: <p>workspace body</p> }]
    }
  ],
  { future: routerFuture, initialEntries: ['/chat'] }
);

render(<RouterProvider router={router} />);

expect(await screen.findByText('Workspace')).toBeInTheDocument();
expect(screen.getByText('workspace body')).toBeInTheDocument();
```

```tsx
// src/web/src/app/router.test.tsx
it('renders knowledge route inside the workspace shell', async () => {
  const router = createAppRouter(['/knowledge']);

  render(<RouterProvider router={router} />);

  expect(await screen.findByText('Workspace')).toBeInTheDocument();
  expect(await screen.findByRole('heading', { name: 'Knowledge' })).toBeInTheDocument();
});
```

```tsx
// src/web/src/routes/workspace/ChatPage.test.tsx
it('renders console index route on /console', async () => {
  const router = createAppRouter(['/console']);

  render(<RouterProvider router={router} />);

  expect(await screen.findByText('Console')).toBeInTheDocument();
  expect(await screen.findByText('Console Home')).toBeInTheDocument();
});
```

- [ ] **Step 3: Re-run the route and layout smoke tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/app/router.test.tsx \
  src/features/layouts/ConsoleLayout.test.tsx \
  src/features/layouts/WorkspaceLayout.test.tsx \
  src/routes/workspace/ChatPage.test.tsx
```

Expected: PASS with no React Router future warnings and no `act(...)` warnings from the covered files.

- [ ] **Step 4: Commit**

```bash
git add \
  src/web/src/app/router.test.tsx \
  src/web/src/features/layouts/ConsoleLayout.test.tsx \
  src/web/src/features/layouts/WorkspaceLayout.test.tsx \
  src/web/src/routes/workspace/ChatPage.test.tsx
git commit -m "test(web): stabilize route smoke tests under future router"
```

## Task 3: Stabilize Console Page Tests Under Future Routing

**Files:**
- Modify: `src/web/src/routes/console/AccessPage.test.tsx`
- Modify: `src/web/src/routes/console/BillingPage.test.tsx`
- Modify: `src/web/src/routes/console/ConsoleHomePage.test.tsx`
- Modify: `src/web/src/routes/console/ModelsPage.test.tsx`
- Modify: `src/web/src/routes/console/UsagePage.test.tsx`

- [ ] **Step 1: Run the console page tests and verify they fail under the warning gate**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/console/AccessPage.test.tsx \
  src/routes/console/BillingPage.test.tsx \
  src/routes/console/ConsoleHomePage.test.tsx \
  src/routes/console/ModelsPage.test.tsx \
  src/routes/console/UsagePage.test.tsx
```

Expected: FAIL because each file mounts `MemoryRouter` without `future`, and data-loading assertions still allow asynchronous updates to escape the test boundary.

- [ ] **Step 2: Add `future={routerFuture}` and make first visible assertions asynchronous**

```tsx
// src/web/src/routes/console/AccessPage.test.tsx
import { MemoryRouter } from 'react-router-dom';

import { routerFuture } from '../../app/routerFuture';

render(
  <MemoryRouter future={routerFuture}>
    <AccessPage />
  </MemoryRouter>
);

expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
expect(screen.getByText('Workspace: workspace_1')).toBeInTheDocument();
```

```tsx
// src/web/src/routes/console/BillingPage.test.tsx
render(
  <MemoryRouter future={routerFuture}>
    <BillingPage />
  </MemoryRouter>
);

expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
expect(screen.getByText('Estimated cost: $0.0004')).toBeInTheDocument();
```

```tsx
// src/web/src/routes/console/ConsoleHomePage.test.tsx
render(
  <MemoryRouter future={routerFuture}>
    <ConsoleHomePage />
  </MemoryRouter>
);

expect(await screen.findByRole('heading', { name: 'Console Home' })).toBeInTheDocument();
expect(screen.getByRole('link', { name: 'Open billing drill-down' })).toHaveAttribute('href', '/console/billing');
```

```tsx
// src/web/src/routes/console/ModelsPage.test.tsx
render(
  <MemoryRouter future={routerFuture}>
    <ModelsPage />
  </MemoryRouter>
);

expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
expect(screen.getByText('Primary model in current workspace')).toBeInTheDocument();
```

```tsx
// src/web/src/routes/console/UsagePage.test.tsx
render(
  <MemoryRouter future={routerFuture}>
    <UsagePage />
  </MemoryRouter>
);

expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
expect(screen.getByText('Requests this period: 34')).toBeInTheDocument();
```

- [ ] **Step 3: Re-run the console page tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/console/AccessPage.test.tsx \
  src/routes/console/BillingPage.test.tsx \
  src/routes/console/ConsoleHomePage.test.tsx \
  src/routes/console/ModelsPage.test.tsx \
  src/routes/console/UsagePage.test.tsx
```

Expected: PASS with no React Router future warnings and no `act(...)` warnings from console tests.

- [ ] **Step 4: Commit**

```bash
git add \
  src/web/src/routes/console/AccessPage.test.tsx \
  src/web/src/routes/console/BillingPage.test.tsx \
  src/web/src/routes/console/ConsoleHomePage.test.tsx \
  src/web/src/routes/console/ModelsPage.test.tsx \
  src/web/src/routes/console/UsagePage.test.tsx
git commit -m "test(web): remove console route warnings"
```

## Task 4: Stabilize Workspace Page Tests And Achieve A Clean Web Test Run

**Files:**
- Modify: `src/web/src/routes/workspace/OnboardingPage.test.tsx`
- Modify: `src/web/src/routes/workspace/SoloPage.test.tsx`

- [ ] **Step 1: Run the remaining warning-producing workspace tests**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/SoloPage.test.tsx
```

Expected: FAIL because `OnboardingPage.test.tsx` still mounts `MemoryRouter` without `future`, and `SoloPage.test.tsx` still allows asynchronous task-loading updates to escape the test boundary.

- [ ] **Step 2: Make the workspace tests future-compatible and wait for stable execution state**

```tsx
// src/web/src/routes/workspace/OnboardingPage.test.tsx
import { MemoryRouter } from 'react-router-dom';

import { routerFuture } from '../../app/routerFuture';

render(
  <MemoryRouter future={routerFuture}>
    <OnboardingPage />
  </MemoryRouter>
);

expect(await screen.findByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
```

```tsx
// src/web/src/routes/workspace/SoloPage.test.tsx
render(<SoloPage />);

expect(await screen.findByRole('button', { name: 'Start solo run' })).toBeInTheDocument();
fireEvent.change(screen.getByLabelText('Task goal'), { target: { value: 'Draft launch checklist' } });
fireEvent.click(screen.getByRole('button', { name: 'Start solo run' }));

await waitFor(() => {
  expect(screen.getByText('Current step')).toBeInTheDocument();
  expect(screen.getByText('Review workspace context')).toBeInTheDocument();
});
```

- [ ] **Step 3: Re-run the targeted workspace tests and then the full web suite**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/workspace/OnboardingPage.test.tsx \
  src/routes/workspace/SoloPage.test.tsx
pnpm --dir src/web test
```

Expected: Both targeted tests pass, then the full web suite passes with `0 warning`.

- [ ] **Step 4: Commit**

```bash
git add \
  src/web/src/routes/workspace/OnboardingPage.test.tsx \
  src/web/src/routes/workspace/SoloPage.test.tsx
git commit -m "test(web): finish workspace zero-warning closeout"
```

## Task 5: Import And Update The M1 Progress Reports

**Files:**
- Create: `docs/reports/2026-04-04-progress-plan.md`
- Create: `docs/reports/2026-04-06-execution-progress-review.md`

- [ ] **Step 1: Verify the clean worktree does not yet contain the report files**

Run:

```bash
test -f docs/reports/2026-04-04-progress-plan.md
test -f docs/reports/2026-04-06-execution-progress-review.md
```

Expected: FAIL because `docs/reports/*` currently exists only as untracked reference copies in the main workspace, not in this isolated branch.

- [ ] **Step 2: Seed tracked report files from the current main-workspace reference copies**

Run:

```bash
mkdir -p docs/reports
cp ../../docs/reports/2026-04-04-progress-plan.md docs/reports/2026-04-04-progress-plan.md
cp ../../docs/reports/2026-04-06-execution-progress-review.md docs/reports/2026-04-06-execution-progress-review.md
```

Expected: PASS, and both report files now exist inside the worktree as tracked candidates.

- [ ] **Step 3: Update the roadmap report to reflect completed M0 and M1 closeout criteria**

````md
<!-- docs/reports/2026-04-04-progress-plan.md -->
| 阶段 | 日期 | 目标 | 里程碑 |
| --- | --- | --- | --- |
| Phase 0 | 2026-04-06 至 2026-04-10 | 对齐基线，冻结契约，补齐设计和任务拆分 | M0 项目基线冻结（已完成） |
| Phase 1 | 2026-04-13 至 2026-04-24 | 清理测试 warning、固定关键路由验收、追平里程碑文档 | M1 主线可运行收口 |
| Phase 2 | 2026-04-27 至 2026-05-15 | 交付工作区 Beta：Chat / Knowledge CRUD / SOLO starter / Settings / Console | M2 工作区 Beta |
| Phase 3 | 2026-05-18 至 2026-06-05 | 交付能力 Beta：Knowledge 检索 MVP + SOLO runtime MVP + Chat 网关增强 | M3 能力 Beta |
| Phase 4 | 2026-06-08 至 2026-06-19 | CI、性能、安全、文档与发布加固 | M4 RC 候选版 |

| 里程碑 | 验收标准 |
| --- | --- |
| M0 | 文档、契约、owner、范围冻结完成 |
| M1 | 前端可构建，关键路由可访问，前后端契约统一，root `check/test` 通过且 `0 warning` |
| M2 | Chat / Knowledge CRUD / SOLO starter / Settings / Console 完整跑通 |
| M3 | Knowledge retrieval MVP、SOLO runtime MVP、Chat streaming 或网关增强可演示 |
| M4 | CI、测试、文档、安全、发布流程齐备，可进入 RC |
````

```md
## 11. 最终判断

项目当前最适合的推进方式不是继续平铺新功能，而是：

- `M0` 已完成并可作为后续里程碑基线
- 当前主线通过 `M1` 收口后，应正式以 `M2` 作为下一开发战场
- 所有新增能力继续受 `M2/M3` 范围约束，不能回到基础契约反复漂移
```

- [ ] **Step 4: Update the execution-review report to reflect current reality and M1 achievement**

```md
## 0. 结论摘要

当前项目已经完成 `M0 基线冻结`，并正式进入 `M1 Mainline Runnable Closeout` 的最后验收阶段。

更具体地说，项目当前所处阶段可以定义为：

- **产品阶段**：后端 MVP 已成型，前端主线已进入可运行收口阶段
- **执行阶段**：`M0` 已完成，`M1` 收口进行中
- **交付阶段**：主线已经跨过“不可构建、不可访问”的阶段，剩余工作集中在 warning 清零与正式验收结论
```

```md
| 里程碑 | 原计划日期 | 当前状态 | 完成度 | 判断 |
| --- | --- | --- | ---: | --- |
| M0 基线冻结 | 2026-04-10 | 已完成 | 100% | workspace、root entry、契约文档与治理模板已冻结并合入主线 |
| M1 主线可运行 | 2026-04-24 | 收口中 | 85% | `src/web build`、关键路由与 root `check/test` 已打通，剩余 warning 清零与正式验收结论 |
| M2 工作区 Beta | 2026-05-15 | 未开始 | 10% | 仍应在 `M1` 关闭后再进入主战场 |
| M3 能力 Beta | 2026-06-05 | 未开始 | 5% | 继续保持冻结，不前置展开 |
| M4 RC 候选版 | 2026-06-19 | 未开始 | 0% | 仍以后续 CI、安全与发布加固为主 |
```

```md
## 10. 最终判断与执行建议

当前最重要的结论不是“功能还差多少”，而是：

> **项目已经完成 `M0` 基线冻结，并具备 `M1 主线可运行` 的实际代码基础；剩余工作是把测试输出、验收证据和历史文档全部收口到同一结论。**

因此后续推进必须按以下顺序执行：

1. 完成 `M1` 收口：warning 清零、验收证据固化、历史文档追平
2. 再进入 `M2`：Chat / Knowledge CRUD / SOLO starter / Settings / Console 用户流闭环
3. `M3` 能力升级仍在 `M2` 关闭后再展开
4. CI、安全、文档、发布清单继续保留在 `M4` 前闭环
```

- [ ] **Step 5: Verify the report updates reflect the new milestone reality**

Run:

```bash
rg -n --fixed-strings "M0 项目基线冻结（已完成）" docs/reports/2026-04-04-progress-plan.md
rg -n --fixed-strings 'root `check/test` 通过且 `0 warning`' docs/reports/2026-04-04-progress-plan.md
rg -n --fixed-strings "M1 主线可运行收口" docs/reports/2026-04-04-progress-plan.md
rg -n --fixed-strings "| M0 基线冻结 | 2026-04-10 | 已完成 |" docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings "| M1 主线可运行 | 2026-04-24 | 收口中 |" docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings '项目已经完成 `M0` 基线冻结' docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings "尚未进入 Phase 1（主线收敛）" docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings "M0 基线冻结进行中，尚未达成" docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings "| M1 主线可运行 | 2026-04-24 | 未开始 |" docs/reports/2026-04-06-execution-progress-review.md
rg -n --fixed-strings "仍未跨过“主线可运行”这道门槛" docs/reports/2026-04-06-execution-progress-review.md
```

Expected:

- the first six `rg` commands return matches
- the last four `rg` commands return no matches

- [ ] **Step 6: Commit**

```bash
git add \
  docs/reports/2026-04-04-progress-plan.md \
  docs/reports/2026-04-06-execution-progress-review.md
git commit -m "docs: close out M1 mainline runnable status"
```

## Task 6: Run The Full M1 Closeout Verification

**Files:**
- Verify: `src/web/src/app/routerFuture.ts`
- Verify: `src/web/src/app/router.tsx`
- Verify: `src/web/src/test/setup.ts`
- Verify: `src/web/src/app/router.test.tsx`
- Verify: `src/web/src/features/auth/ProtectedRoute.test.tsx`
- Verify: `src/web/src/features/layouts/ConsoleLayout.test.tsx`
- Verify: `src/web/src/features/layouts/WorkspaceLayout.test.tsx`
- Verify: `src/web/src/routes/workspace/ChatPage.test.tsx`
- Verify: `src/web/src/routes/workspace/OnboardingPage.test.tsx`
- Verify: `src/web/src/routes/workspace/SoloPage.test.tsx`
- Verify: `src/web/src/routes/console/AccessPage.test.tsx`
- Verify: `src/web/src/routes/console/BillingPage.test.tsx`
- Verify: `src/web/src/routes/console/ConsoleHomePage.test.tsx`
- Verify: `src/web/src/routes/console/ModelsPage.test.tsx`
- Verify: `src/web/src/routes/console/UsagePage.test.tsx`
- Verify: `docs/reports/2026-04-04-progress-plan.md`
- Verify: `docs/reports/2026-04-06-execution-progress-review.md`

- [ ] **Step 1: Run the root quality gate**

Run:

```bash
bash scripts/check.sh
```

Expected: PASS

- [ ] **Step 2: Run the full root test entry and capture the log**

Run:

```bash
bash scripts/test.sh 2>&1 | tee /tmp/m1-mainline-runnable-closeout-test.log
```

Expected: PASS. If `TEST_DATABASE_URL` is not set, accept the explicit skip message for server integration tests.

- [ ] **Step 3: Verify the test log is warning-free**

Run:

```bash
rg -n "Future Flag Warning|Warning: An update to|unexpected console\\.warn|unexpected console\\.error" /tmp/m1-mainline-runnable-closeout-test.log
```

Expected: no matches

- [ ] **Step 4: Verify diff hygiene and intended file scope**

Run:

```bash
git diff --check
git status --short
```

Expected:

- `git diff --check` returns no whitespace / conflict-marker errors
- `git status --short` only shows the intended M1 closeout implementation files

- [ ] **Step 5: Commit**

```bash
git add \
  src/web/src/app/routerFuture.ts \
  src/web/src/app/router.tsx \
  src/web/src/test/setup.ts \
  src/web/src/app/router.test.tsx \
  src/web/src/features/auth/ProtectedRoute.test.tsx \
  src/web/src/features/layouts/ConsoleLayout.test.tsx \
  src/web/src/features/layouts/WorkspaceLayout.test.tsx \
  src/web/src/routes/workspace/ChatPage.test.tsx \
  src/web/src/routes/workspace/OnboardingPage.test.tsx \
  src/web/src/routes/workspace/SoloPage.test.tsx \
  src/web/src/routes/console/AccessPage.test.tsx \
  src/web/src/routes/console/BillingPage.test.tsx \
  src/web/src/routes/console/ConsoleHomePage.test.tsx \
  src/web/src/routes/console/ModelsPage.test.tsx \
  src/web/src/routes/console/UsagePage.test.tsx \
  docs/reports/2026-04-04-progress-plan.md \
  docs/reports/2026-04-06-execution-progress-review.md
git commit -m "feat: close out M1 mainline runnable milestone"
```
