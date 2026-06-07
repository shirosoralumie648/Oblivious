# Console Operations Overview Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把当前 Console 收敛成一个“首页总览 + drill-down 工作台”的前端只读运营视图，不改后端 `/api/v1/console/*` 契约。

**Architecture:** 保持现有 console 路由树不变，优先升级 `ConsoleLayout` 成为带管理员壳层与快捷跳转的稳定框架。`/console` 使用 executive overview 风格聚合四个 console 接口，`/console/billing`、`/console/usage`、`/console/models`、`/console/access` 统一切换为 workbench 风格，并通过共享展示组件复用上下文侧栏与明细布局。首页使用 `Promise.allSettled` 处理局部降级，drill-down 页面以主接口为核心并按需补 `access` 作为上下文。

**Tech Stack:** React 18, React Router 6, Vite 5, Vitest, Testing Library, existing `createConsoleApi`, `createHttpClient`, `types/api.ts`

---

## File Structure

- Create: `src/web/src/features/console/components/ConsoleOverviewCard.tsx`
  - 总览页 KPI 卡片，统一 title/value/note/link 结构
- Create: `src/web/src/features/console/components/ConsoleSnapshotPanel.tsx`
  - 总览页右侧辅助摘要区与主叙事区的轻量容器
- Create: `src/web/src/features/console/components/ConsoleContextRail.tsx`
  - workbench 页面共享的 scope、session、shortcut 侧栏
- Create: `src/web/src/features/console/components/ConsoleWorkbenchLayout.tsx`
  - drill-down 页面共享的两栏布局与 sibling navigation
- Modify: `src/web/src/features/layouts/ConsoleLayout.tsx`
  - 升级 console shell，调整导航顺序并增加管理员壳层提示与快捷链接
- Modify: `src/web/src/features/layouts/ConsoleLayout.test.tsx`
  - 锁定 shell 的 scope 提示、shortcut、导航顺序
- Modify: `src/web/src/routes/console/ConsoleHomePage.tsx`
  - 聚合四个 console 接口，渲染 overview 卡片、主叙事区与局部降级
- Modify: `src/web/src/routes/console/ConsoleHomePage.test.tsx`
  - 锁定首页 KPI、drill-down 入口与 all-settled 降级行为
- Modify: `src/web/src/routes/console/BillingPage.tsx`
  - 切换为 workbench 风格，补 access 侧栏与 sibling navigation
- Modify: `src/web/src/routes/console/BillingPage.test.tsx`
  - 锁定 billing 主内容、context rail、overview/usage 跳转
- Modify: `src/web/src/routes/console/UsagePage.tsx`
  - 切换为 workbench 风格，补 access 侧栏与 sibling navigation
- Modify: `src/web/src/routes/console/UsagePage.test.tsx`
  - 锁定 usage 主内容、context rail、overview/billing 跳转、失败降级
- Modify: `src/web/src/routes/console/ModelsPage.tsx`
  - 切换为 supporting drill-down workbench
- Modify: `src/web/src/routes/console/ModelsPage.test.tsx`
  - 锁定 supporting drill-down 结构与失败降级
- Modify: `src/web/src/routes/console/AccessPage.tsx`
  - 切换为 supporting drill-down workbench 并明确当前 scope
- Modify: `src/web/src/routes/console/AccessPage.test.tsx`
  - 锁定 scope 文案、shortcut、supporting drill-down 布局
- Modify: `src/web/src/app/router.test.tsx`
  - 扩展 console route smoke tests
- Modify: `docs/architecture/current-system-contracts.md`
  - 更新 console 路由状态为已实现运营总览链路

## Task 1: Upgrade The Console Shell

**Files:**
- Modify: `src/web/src/features/layouts/ConsoleLayout.tsx`
- Modify: `src/web/src/features/layouts/ConsoleLayout.test.tsx`

- [ ] **Step 1: Write the failing console shell tests**

```tsx
it('renders an admin shell with scope messaging and workspace shortcuts', () => {
  const router = createMemoryRouter(
    [
      {
        path: '/console',
        element: <ConsoleLayout />,
        children: [{ index: true, element: <p>Overview page</p> }]
      }
    ],
    { initialEntries: ['/console'] }
  );

  render(<RouterProvider router={router} />);

  expect(screen.getByText('Current workspace scope')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Workspace settings' })).toHaveAttribute('href', '/settings');
  expect(screen.getByRole('link', { name: 'Return to workspace' })).toHaveAttribute('href', '/chat');
});

it('renders console navigation in overview-billing-usage-models-access order', () => {
  const router = createMemoryRouter(
    [
      {
        path: '/console',
        element: <ConsoleLayout />,
        children: [{ index: true, element: <p>Overview page</p> }]
      }
    ],
    { initialEntries: ['/console'] }
  );

  render(<RouterProvider router={router} />);

  const links = screen
    .getAllByRole('link')
    .map((link) => link.textContent)
    .filter((text): text is string => text !== null);

  expect(links).toEqual([
    'Workspace settings',
    'Return to workspace',
    'Overview',
    'Billing',
    'Usage',
    'Models',
    'Access'
  ]);
});
```

- [ ] **Step 2: Run the layout tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run src/features/layouts/ConsoleLayout.test.tsx
```

Expected: FAIL because `ConsoleLayout` does not yet render `Current workspace scope`, shortcut links, or the new nav order.

- [ ] **Step 3: Implement the admin shell and nav order**

```tsx
import { Link, Outlet } from 'react-router-dom';

import { useAppContext } from '../../app/providers';

export function ConsoleLayout() {
  const { authState } = useAppContext();

  return (
    <div>
      <h1>Console</h1>
      <p>Current workspace scope</p>
      <p>{authState.user?.email ?? 'anonymous'}</p>
      <p>{`Default mode: ${authState.preferences?.defaultMode ?? 'chat'}`}</p>
      <nav aria-label="Console shortcuts">
        <Link to="/settings">Workspace settings</Link>
        <Link to="/chat">Return to workspace</Link>
      </nav>
      <nav aria-label="Console navigation">
        <Link to="/console">Overview</Link>
        <Link to="/console/billing">Billing</Link>
        <Link to="/console/usage">Usage</Link>
        <Link to="/console/models">Models</Link>
        <Link to="/console/access">Access</Link>
      </nav>
      <Outlet />
    </div>
  );
}
```

- [ ] **Step 4: Re-run the layout tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run src/features/layouts/ConsoleLayout.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  src/web/src/features/layouts/ConsoleLayout.tsx \
  src/web/src/features/layouts/ConsoleLayout.test.tsx
git commit -m "feat(web): add console admin shell"
```

## Task 2: Build The Overview Dashboard

**Files:**
- Create: `src/web/src/features/console/components/ConsoleOverviewCard.tsx`
- Create: `src/web/src/features/console/components/ConsoleSnapshotPanel.tsx`
- Modify: `src/web/src/routes/console/ConsoleHomePage.tsx`
- Modify: `src/web/src/routes/console/ConsoleHomePage.test.tsx`

- [ ] **Step 1: Write the failing overview tests**

```tsx
import { MemoryRouter } from 'react-router-dom';

it('renders drill-down cards for billing, usage, models, and access', async () => {
  getAccess.mockResolvedValue({
    defaultMode: 'solo',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
    sessionExpiresAt: '2026-04-03T00:00:00Z',
    sessionId: 'session_1',
    userEmail: 'user@example.com',
    userId: 'user_1',
    workspaceId: 'workspace_1'
  });
  getUsage.mockResolvedValue({ period: '7d', requests: 3 });
  getBilling.mockResolvedValue({
    period: '30d',
    requests: 5,
    inputTokens: 120,
    outputTokens: 80,
    estimatedCostUsd: 0.0004
  });
  getModels.mockResolvedValue([{ id: 'balanced-chat', label: 'balanced-chat', requests: 2 }]);

  render(
    <MemoryRouter>
      <ConsoleHomePage />
    </MemoryRouter>
  );

  expect(await screen.findByRole('link', { name: 'Estimated cost' })).toHaveAttribute('href', '/console/billing');
  expect(screen.getByRole('link', { name: 'Requests' })).toHaveAttribute('href', '/console/usage');
  expect(screen.getByRole('link', { name: 'Top model' })).toHaveAttribute('href', '/console/models');
  expect(screen.getByRole('link', { name: 'Access posture' })).toHaveAttribute('href', '/console/access');
  expect(screen.getByText('Current workspace scope: workspace_1')).toBeInTheDocument();
});

it('keeps the dashboard available when one summary fails', async () => {
  getAccess.mockResolvedValue({
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: false,
    onboardingCompleted: true,
    sessionExpiresAt: '2026-04-03T00:00:00Z',
    sessionId: 'session_1',
    userEmail: 'user@example.com',
    userId: 'user_1',
    workspaceId: 'workspace_1'
  });
  getUsage.mockResolvedValue({ period: '7d', requests: 3 });
  getBilling.mockResolvedValue({
    period: '30d',
    requests: 5,
    inputTokens: 120,
    outputTokens: 80,
    estimatedCostUsd: 0.0004
  });
  getModels.mockRejectedValue(new Error('network unavailable'));

  render(
    <MemoryRouter>
      <ConsoleHomePage />
    </MemoryRouter>
  );

  expect(await screen.findByText('Estimated cost')).toBeInTheDocument();
  expect(screen.getByText('Top model unavailable')).toBeInTheDocument();
  expect(screen.queryByText('Unable to load dashboard.')).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run the overview tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/console/ConsoleHomePage.test.tsx
```

Expected: FAIL because the home page does not yet render drill-down links or partial all-settled degradation.

- [ ] **Step 3: Create the overview presentation components**

```tsx
// src/web/src/features/console/components/ConsoleOverviewCard.tsx
import { Link } from 'react-router-dom';

type ConsoleOverviewCardProps = {
  title: string;
  value: string;
  note: string;
  to: string;
};

export function ConsoleOverviewCard({ title, value, note, to }: ConsoleOverviewCardProps) {
  return (
    <Link aria-label={title} to={to}>
      <h2>{title}</h2>
      <p>{value}</p>
      <p>{note}</p>
    </Link>
  );
}

// src/web/src/features/console/components/ConsoleSnapshotPanel.tsx
import type { ReactNode } from 'react';

type ConsoleSnapshotPanelProps = {
  title: string;
  children: ReactNode;
};

export function ConsoleSnapshotPanel({ title, children }: ConsoleSnapshotPanelProps) {
  return (
    <section aria-label={title}>
      <h3>{title}</h3>
      {children}
    </section>
  );
}
```

- [ ] **Step 4: Rebuild `ConsoleHomePage` around all-settled overview cards**

```tsx
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleOverviewCard } from '../../features/console/components/ConsoleOverviewCard';
import { ConsoleSnapshotPanel } from '../../features/console/components/ConsoleSnapshotPanel';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, BillingSummary, ModelSummary, UsageSummary } from '../../types/api';

export function ConsoleHomePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [billingSummary, setBillingSummary] = useState<BillingSummary | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [modelSummaries, setModelSummaries] = useState<ModelSummary[] | null>(null);
  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadDashboard = async () => {
      const [access, billing, models, usage] = await Promise.allSettled([
        consoleApi.getAccess(),
        consoleApi.getBilling(),
        consoleApi.getModels(),
        consoleApi.getUsage()
      ]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      setBillingSummary(billing.status === 'fulfilled' ? billing.value : null);
      setModelSummaries(models.status === 'fulfilled' ? models.value : null);
      setUsageSummary(usage.status === 'fulfilled' ? usage.value : null);
      setLoadError([access, billing, models, usage].every((result) => result.status === 'rejected'));
    };

    void loadDashboard();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  if (loadError) {
    return (
      <section>
        <h1>Console Home</h1>
        <p>Unable to load dashboard.</p>
      </section>
    );
  }

  if (accessSummary === null && billingSummary === null && modelSummaries === null && usageSummary === null) {
    return (
      <section>
        <h1>Console Home</h1>
        <p>Loading dashboard…</p>
      </section>
    );
  }

  const topModel = modelSummaries?.[0]?.label ?? 'Top model unavailable';
  const accessPosture = accessSummary ? `Session ${accessSummary.sessionId}` : 'Access posture unavailable';

  return (
    <section>
      <h1>Console Home</h1>
      <p>{`Current workspace scope: ${accessSummary?.workspaceId ?? 'unavailable'}`}</p>
      <section aria-label="Key performance indicators">
        <ConsoleOverviewCard
          note={billingSummary?.period ?? 'Billing summary unavailable'}
          title="Estimated cost"
          to="/console/billing"
          value={billingSummary ? `$${billingSummary.estimatedCostUsd.toFixed(4)}` : 'Estimated cost unavailable'}
        />
        <ConsoleOverviewCard
          note={usageSummary?.period ?? 'Usage summary unavailable'}
          title="Requests"
          to="/console/usage"
          value={usageSummary ? String(usageSummary.requests) : 'Requests unavailable'}
        />
        <ConsoleOverviewCard note="Primary model in current workspace" title="Top model" to="/console/models" value={topModel} />
        <ConsoleOverviewCard note="Current session and workspace context" title="Access posture" to="/console/access" value={accessPosture} />
      </section>
      <ConsoleSnapshotPanel title="Cost and usage focus">
        <p>{`Billing requests: ${billingSummary?.requests ?? 'unavailable'}`}</p>
        <p>{`Usage requests: ${usageSummary?.requests ?? 'unavailable'}`}</p>
        <Link to="/console/billing">Open billing drill-down</Link>
        <Link to="/console/usage">Open usage drill-down</Link>
      </ConsoleSnapshotPanel>
      <ConsoleSnapshotPanel title="Supporting summaries">
        <p>{`Top model: ${topModel}`}</p>
        <p>{`User: ${accessSummary?.userEmail ?? 'unavailable'}`}</p>
      </ConsoleSnapshotPanel>
    </section>
  );
}
```

- [ ] **Step 5: Re-run the overview tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run src/routes/console/ConsoleHomePage.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add \
  src/web/src/features/console/components/ConsoleOverviewCard.tsx \
  src/web/src/features/console/components/ConsoleSnapshotPanel.tsx \
  src/web/src/routes/console/ConsoleHomePage.tsx \
  src/web/src/routes/console/ConsoleHomePage.test.tsx
git commit -m "feat(web): add console overview dashboard"
```

## Task 3: Convert Billing And Usage Into Workbench Pages

**Files:**
- Create: `src/web/src/features/console/components/ConsoleContextRail.tsx`
- Create: `src/web/src/features/console/components/ConsoleWorkbenchLayout.tsx`
- Modify: `src/web/src/routes/console/BillingPage.tsx`
- Modify: `src/web/src/routes/console/BillingPage.test.tsx`
- Modify: `src/web/src/routes/console/UsagePage.tsx`
- Modify: `src/web/src/routes/console/UsagePage.test.tsx`

- [ ] **Step 1: Write the failing billing and usage workbench tests**

```tsx
// BillingPage.test.tsx
const getAccess = vi.fn();
const getBilling = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getBilling
  })
}));

it('renders billing inside a workbench layout with context rail and sibling links', async () => {
  getAccess.mockResolvedValue({
    defaultMode: 'solo',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
    sessionExpiresAt: '2026-04-03T00:00:00Z',
    sessionId: 'session_1',
    userEmail: 'user@example.com',
    userId: 'user_1',
    workspaceId: 'workspace_1'
  });
  getBilling.mockResolvedValue({
    period: '30d',
    requests: 5,
    inputTokens: 120,
    outputTokens: 80,
    estimatedCostUsd: 0.0004
  });

  render(
    <MemoryRouter>
      <BillingPage />
    </MemoryRouter>
  );

  expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
  expect(screen.getByText('Workspace: workspace_1')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Back to overview' })).toHaveAttribute('href', '/console');
  expect(screen.getByRole('link', { name: 'Open usage' })).toHaveAttribute('href', '/console/usage');
});

// UsagePage.test.tsx
const getAccess = vi.fn();
const getUsage = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getUsage
  })
}));

it('keeps the usage workbench frame available when the summary fails', async () => {
  getAccess.mockResolvedValue({
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: false,
    onboardingCompleted: true,
    sessionExpiresAt: '2026-04-03T00:00:00Z',
    sessionId: 'session_1',
    userEmail: 'user@example.com',
    userId: 'user_1',
    workspaceId: 'workspace_1'
  });
  getUsage.mockRejectedValue(new Error('usage unavailable'));

  render(
    <MemoryRouter>
      <UsagePage />
    </MemoryRouter>
  );

  expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Back to overview' })).toBeInTheDocument();
  expect(screen.getByText('Unable to load usage summary.')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the billing and usage tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/console/BillingPage.test.tsx \
  src/routes/console/UsagePage.test.tsx
```

Expected: FAIL because these pages do not yet fetch access context or render workbench structure.

- [ ] **Step 3: Create the shared workbench components**

```tsx
// src/web/src/features/console/components/ConsoleContextRail.tsx
import { Link } from 'react-router-dom';

import type { AccessSummary } from '../../../types/api';

type ConsoleContextRailProps = {
  accessSummary: AccessSummary | null;
};

export function ConsoleContextRail({ accessSummary }: ConsoleContextRailProps) {
  return (
    <aside>
      <h2>Current workspace scope</h2>
      {accessSummary ? (
        <>
          <p>{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p>{`Session: ${accessSummary.sessionId}`}</p>
          <p>{`Default mode: ${accessSummary.defaultMode}`}</p>
          <p>{`Model strategy: ${accessSummary.modelStrategy}`}</p>
        </>
      ) : (
        <p>Access context unavailable.</p>
      )}
      <nav aria-label="Console workbench shortcuts">
        <Link to="/console">Back to overview</Link>
        <Link to="/settings">Workspace settings</Link>
        <Link to="/chat">Return to workspace</Link>
      </nav>
    </aside>
  );
}

// src/web/src/features/console/components/ConsoleWorkbenchLayout.tsx
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

import type { AccessSummary } from '../../../types/api';
import { ConsoleContextRail } from './ConsoleContextRail';

type ConsoleWorkbenchLayoutProps = {
  accessSummary: AccessSummary | null;
  children: ReactNode;
  description: string;
  errorMessage: string | null;
  siblingLinks: Array<{ label: string; to: string }>;
  title: string;
};

export function ConsoleWorkbenchLayout({
  accessSummary,
  children,
  description,
  errorMessage,
  siblingLinks,
  title
}: ConsoleWorkbenchLayoutProps) {
  return (
    <section>
      <h1>{title}</h1>
      <p>{description}</p>
      <nav aria-label={`${title} sibling navigation`}>
        {siblingLinks.map((link) => (
          <Link key={link.to} to={link.to}>
            {link.label}
          </Link>
        ))}
      </nav>
      <div>
        <ConsoleContextRail accessSummary={accessSummary} />
        <section>{errorMessage ? <p>{errorMessage}</p> : children}</section>
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Convert `BillingPage` and `UsagePage` to workbench layout**

```tsx
// BillingPage.tsx
import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, BillingSummary } from '../../types/api';

export function BillingPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [billingSummary, setBillingSummary] = useState<BillingSummary | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadBilling = async () => {
      const [access, billing] = await Promise.allSettled([consoleApi.getAccess(), consoleApi.getBilling()]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (billing.status === 'fulfilled') {
        setBillingSummary(billing.value);
        setErrorMessage(null);
      } else {
        setBillingSummary(null);
        setErrorMessage('Unable to load billing summary.');
      }
    };

    void loadBilling();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review current workspace cost and billing activity."
      errorMessage={errorMessage}
      siblingLinks={[
        { label: 'Back to overview', to: '/console' },
        { label: 'Open usage', to: '/console/usage' }
      ]}
      title="Billing"
    >
      {billingSummary ? (
        <>
          <p>{`Requests: ${billingSummary.requests}`}</p>
          <p>{`Input tokens: ${billingSummary.inputTokens}`}</p>
          <p>{`Output tokens: ${billingSummary.outputTokens}`}</p>
          <p>{`Estimated cost: $${billingSummary.estimatedCostUsd.toFixed(4)}`}</p>
        </>
      ) : (
        <p>Billing summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}

// UsagePage.tsx
import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, UsageSummary } from '../../types/api';

export function UsagePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadUsage = async () => {
      const [access, usage] = await Promise.allSettled([consoleApi.getAccess(), consoleApi.getUsage()]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (usage.status === 'fulfilled') {
        setUsageSummary(usage.value);
        setErrorMessage(null);
      } else {
        setUsageSummary(null);
        setErrorMessage('Unable to load usage summary.');
      }
    };

    void loadUsage();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review current workspace request volume and operating context."
      errorMessage={errorMessage}
      siblingLinks={[
        { label: 'Back to overview', to: '/console' },
        { label: 'Open billing', to: '/console/billing' }
      ]}
      title="Usage"
    >
      {usageSummary ? (
        <>
          <p>{`Requests: ${usageSummary.requests}`}</p>
          <p>{`Period: ${usageSummary.period}`}</p>
        </>
      ) : (
        <p>Usage summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
```

- [ ] **Step 5: Re-run the billing and usage tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/console/BillingPage.test.tsx \
  src/routes/console/UsagePage.test.tsx
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add \
  src/web/src/features/console/components/ConsoleContextRail.tsx \
  src/web/src/features/console/components/ConsoleWorkbenchLayout.tsx \
  src/web/src/routes/console/BillingPage.tsx \
  src/web/src/routes/console/BillingPage.test.tsx \
  src/web/src/routes/console/UsagePage.tsx \
  src/web/src/routes/console/UsagePage.test.tsx
git commit -m "feat(web): add console billing and usage workbench"
```

## Task 4: Convert Models And Access Into Supporting Drill-Down Pages

**Files:**
- Modify: `src/web/src/routes/console/ModelsPage.tsx`
- Modify: `src/web/src/routes/console/ModelsPage.test.tsx`
- Modify: `src/web/src/routes/console/AccessPage.tsx`
- Modify: `src/web/src/routes/console/AccessPage.test.tsx`

- [ ] **Step 1: Write the failing supporting-page tests**

```tsx
// ModelsPage.test.tsx
const getAccess = vi.fn();
const getModels = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getModels
  })
}));

it('renders models as a supporting drill-down with context rail', async () => {
  getAccess.mockResolvedValue({
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
    sessionExpiresAt: '2026-04-03T00:00:00Z',
    sessionId: 'session_1',
    userEmail: 'user@example.com',
    userId: 'user_1',
    workspaceId: 'workspace_1'
  });
  getModels.mockResolvedValue([
    { id: 'balanced-chat', label: 'balanced-chat', requests: 2 },
    { id: 'quality-chat', label: 'quality-chat', requests: 1 }
  ]);

  render(
    <MemoryRouter>
      <ModelsPage />
    </MemoryRouter>
  );

  expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Back to overview' })).toBeInTheDocument();
  expect(screen.getByText('balanced-chat')).toBeInTheDocument();
});

// AccessPage.test.tsx
const getAccess = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess
  })
}));

it('renders the access page as a scope explanation workbench', async () => {
  getAccess.mockResolvedValue({
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
    sessionExpiresAt: '2026-04-03T00:00:00Z',
    sessionId: 'session_1',
    userEmail: 'user@example.com',
    userId: 'user_1',
    workspaceId: 'workspace_1'
  });

  render(
    <MemoryRouter>
      <AccessPage />
    </MemoryRouter>
  );

  expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
  expect(screen.getByText('This console reflects the active workspace and current session.')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Workspace settings' })).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the models and access tests to verify they fail**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/console/ModelsPage.test.tsx \
  src/routes/console/AccessPage.test.tsx
```

Expected: FAIL because these pages still render simple summaries instead of the shared workbench structure.

- [ ] **Step 3: Convert `ModelsPage` and `AccessPage` to supporting workbench pages**

```tsx
// ModelsPage.tsx
import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, ModelSummary } from '../../types/api';

export function ModelsPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [models, setModels] = useState<ModelSummary[] | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadModels = async () => {
      const [access, modelsResult] = await Promise.allSettled([consoleApi.getAccess(), consoleApi.getModels()]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (modelsResult.status === 'fulfilled') {
        setModels(modelsResult.value);
        setLoadError(null);
      } else {
        setModels(null);
        setLoadError('Unable to load model summaries.');
      }
    };

    void loadModels();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review the current workspace model mix and relative request volume."
      errorMessage={loadError}
      siblingLinks={[
        { label: 'Back to overview', to: '/console' },
        { label: 'Open access', to: '/console/access' }
      ]}
      title="Models"
    >
      {models ? (
        <ul>
          {models.map((model) => (
            <li key={model.id}>
              <p>{model.label}</p>
              <p>{`Requests: ${model.requests}`}</p>
            </li>
          ))}
        </ul>
      ) : (
        <p>Model summaries unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}

// AccessPage.tsx
import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary } from '../../types/api';

export function AccessPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadAccess = async () => {
      try {
        const summary = await consoleApi.getAccess();
        if (!cancelled) {
          setAccessSummary(summary);
          setLoadError(null);
        }
      } catch {
        if (!cancelled) {
          setAccessSummary(null);
          setLoadError('Unable to load access summary.');
        }
      }
    };

    void loadAccess();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review the exact scope and session context behind this console."
      errorMessage={loadError}
      siblingLinks={[
        { label: 'Back to overview', to: '/console' },
        { label: 'Open models', to: '/console/models' }
      ]}
      title="Access"
    >
      {accessSummary ? (
        <>
          <p>This console reflects the active workspace and current session.</p>
          <p>{`User: ${accessSummary.userEmail}`}</p>
          <p>{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p>{`Session: ${accessSummary.sessionId}`}</p>
          <p>{`Default mode: ${accessSummary.defaultMode}`}</p>
        </>
      ) : (
        <p>Access summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
```

- [ ] **Step 4: Re-run the models and access tests to verify they pass**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/routes/console/ModelsPage.test.tsx \
  src/routes/console/AccessPage.test.tsx
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  src/web/src/routes/console/ModelsPage.tsx \
  src/web/src/routes/console/ModelsPage.test.tsx \
  src/web/src/routes/console/AccessPage.tsx \
  src/web/src/routes/console/AccessPage.test.tsx
git commit -m "feat(web): add console supporting drill-down pages"
```

## Task 5: Update Contracts And Run Final Regression

**Files:**
- Modify: `docs/architecture/current-system-contracts.md`
- Modify: `src/web/src/app/router.test.tsx`

- [ ] **Step 1: Update the console route contract**

```md
| Console | `/console` | 已接入，运营总览页可用 |
| Console | `/console/models` | 已接入，supporting drill-down 可用 |
| Console | `/console/usage` | 已接入，请求量 workbench drill-down 可用 |
| Console | `/console/billing` | 已接入，成本 workbench drill-down 可用 |
| Console | `/console/access` | 已接入，scope / session workbench drill-down 可用 |
```

- [ ] **Step 2: Extend router smoke tests for the stabilized console flow**

```tsx
it('renders billing route inside the console shell', () => {
  const router = createAppRouter(['/console/billing']);

  render(<RouterProvider router={router} />);

  expect(screen.getByText('Console')).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: 'Billing' })).toBeInTheDocument();
});
```

- [ ] **Step 3: Run the focused console regression suite**

Run:

```bash
pnpm --dir src/web exec vitest run \
  src/features/layouts/ConsoleLayout.test.tsx \
  src/routes/console/ConsoleHomePage.test.tsx \
  src/routes/console/BillingPage.test.tsx \
  src/routes/console/UsagePage.test.tsx \
  src/routes/console/ModelsPage.test.tsx \
  src/routes/console/AccessPage.test.tsx \
  src/app/router.test.tsx
```

Expected: PASS

- [ ] **Step 4: Run the full web verification**

Run:

```bash
pnpm --dir src/web test
pnpm --dir src/web build
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add \
  docs/architecture/current-system-contracts.md \
  src/web/src/app/router.test.tsx
git commit -m "docs(web): record console operations overview"
```
