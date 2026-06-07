import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

vi.mock('../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess: () =>
      Promise.resolve({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: true,
        onboardingCompleted: true,
        sessionExpiresAt: '2026-04-03T00:00:00Z',
        sessionId: 'session_1',
        userEmail: 'user@example.com',
        userId: 'user_1',
        workspaceId: 'workspace_1'
      }),
    getBilling: () =>
      Promise.resolve({
        period: '30d',
        requests: 5,
        inputTokens: 120,
        outputTokens: 80,
        estimatedCostUsd: 0.0004,
        balanceUsd: 42.5,
        creditLimitUsd: 100,
        currentSpendUsd: 0.0004,
        nextInvoice: {
          amountUsd: 0.0004,
          dueAt: '2026-04-30T00:00:00Z',
          id: 'invoice_next',
          status: 'draft'
        }
      }),
    listInvoices: () =>
      Promise.resolve([
        {
          amountUsd: 0.0004,
          dueAt: '2026-04-30T00:00:00Z',
          id: 'invoice_paid',
          status: 'paid'
        }
      ]),
    getModels: () =>
      Promise.resolve([{ id: 'balanced-chat', label: 'balanced-chat', requests: 2 }]),
    getUsage: () => Promise.resolve({ period: '7d', requests: 3 })
  })
}));

vi.mock('../features/notifications/notificationsApi', () => ({
  createNotificationsApi: () => ({
    listNotifications: () =>
      Promise.resolve([
        {
          category: 'system',
          createdAt: '2026-06-06T08:00:00Z',
          id: 'notif_1',
          isRead: false,
          message: 'Database connection failed',
          title: 'Database down',
          type: 'critical',
          userId: 'user_1'
        }
      ]),
    markRead: () => Promise.resolve({ id: 'notif_1', isRead: true })
  })
}));

vi.mock('../app/providers', () => ({
  useAppContext: () => ({
    authState: {
      status: 'authenticated',
      user: { id: 'admin_1', email: 'admin@example.com', role: 'admin' },
      preferences: { onboardingCompleted: false }
    },
    bootstrapAuth: () => Promise.resolve(),
    updatePreferences: (preferences: unknown) => Promise.resolve(preferences)
  })
}));

vi.mock('../features/admin/api', () => ({
  createAdminApi: () => ({
    getBillingSummary: () =>
      Promise.resolve({
        billingSessions: { count: 1, settledAmount: 4.5 },
        paymentIntents: { count: 1, totalAmount: 29 },
        webhookEvents: { count: 1 },
        settlements: { count: 1, grossAmount: 50 },
        payouts: { count: 1, totalAmount: 40 }
      }),
    listBillingSurface: () => Promise.resolve({ data: [], total: 0 }),
    listUsageLogs: () => Promise.resolve({ data: [], total: 0 }),
    listAPITokens: () => Promise.resolve({ data: [], total: 0 }),
    listModelInventory: () => Promise.resolve({ data: [], total: 0 }),
    revokeAPIToken: () => Promise.resolve()
  })
}));

vi.mock('../features/scheduledTasks/scheduledTasksApi', () => ({
  createScheduledTasksApi: () => ({
    createScheduledTask: () =>
      Promise.resolve({
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_2',
        targetId: 'workflow_2',
        targetType: 'workflow'
      }),
    listScheduledTasks: () =>
      Promise.resolve([
        {
          cronExpression: '0 9 * * 1',
          enabled: true,
          id: 'schedule_1',
          targetId: 'workflow_1',
          targetType: 'workflow'
        }
      ])
  })
}));

vi.mock('../features/publishingChannels/publishingChannelsApi', () => ({
  createPublishingChannelsApi: () => ({
    createChannel: () =>
      Promise.resolve({
        config: { endpointUrl: 'https://hooks.example/ops' },
        id: 'channel_1',
        name: 'Ops Webhook',
        status: 'active',
        type: 'webhook'
      }),
    listChannels: () =>
      Promise.resolve([
        {
          config: { endpointUrl: 'https://hooks.example/ops' },
          id: 'channel_1',
          name: 'Ops Webhook',
          status: 'active',
          type: 'webhook'
        }
      ]),
    sendChannelMessage: () => Promise.resolve({ id: 'log_1', status: 'recorded', transform_success: true }),
    testChannel: () => Promise.resolve({ message: 'channel adapter is available', status: 'success' }),
    updateChannelStatus: (channel: unknown) => Promise.resolve(channel)
  })
}));

vi.mock('../features/agents/planStepsApi', () => ({
  createAgentPlanStepsApi: () => ({
    approvePlanStep: () => Promise.resolve([]),
    approveToolRun: () => Promise.resolve([]),
    getRunDetail: () => Promise.resolve({ id: 'run_1', planSteps: [], status: 'running', toolRuns: [] }),
    retryToolRun: () => Promise.resolve([]),
    executePlanStep: () => Promise.resolve([])
  })
}));

vi.mock('../features/agents/agentsApi', () => ({
  createAgentsApi: () => ({
    listAgents: () =>
      Promise.resolve([
        {
          config: { approvalMode: 'tiered' },
          id: 'agent_1',
          isPublic: false,
          model: 'gpt-4o-mini',
          name: 'Research Agent',
          tools: []
        }
      ]),
    updateAgent: (agent: unknown) => Promise.resolve(agent)
  })
}));

vi.mock('../features/mcp/mcpServersApi', () => ({
  createMcpServersApi: () => ({
    addServer: () =>
      Promise.resolve({
        id: 'mcp_remote',
        name: 'Remote MCP',
        status: 'disconnected',
        url: 'https://mcp.example/sse'
      }),
    connectServer: (serverId: string) =>
      Promise.resolve({
        id: serverId,
        name: 'Remote MCP',
        status: 'connected',
        url: 'https://mcp.example/sse'
      }),
    disconnectServer: () => Promise.resolve({ status: 'disconnected' }),
    executeTool: () => Promise.resolve({ content: '{"ok":true}', isError: false }),
    getServerStatus: () => Promise.resolve({ status: 'connected' }),
    listLocalServers: () =>
      Promise.resolve([
        {
          description: 'Tenant-safe local MCP tools exposed by this server',
          id: 'local_builtin_safe',
          name: 'Oblivious Safe Builtins',
          toolCount: 2
        }
      ]),
    listServerTools: () => Promise.resolve([]),
    listServers: () => Promise.resolve([])
  })
}));

import { createAppRouter } from './router';
import { routerFuture } from './routerFuture';

describe('app router', () => {
  it('renders home content on /', async () => {
    const router = createAppRouter(['/']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Oblivious')).toBeInTheDocument();
    expect(await screen.findByText('AI workspace framework')).toBeInTheDocument();
  });

  it('renders knowledge route inside the workspace shell', async () => {
    const router = createAppRouter(['/knowledge']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Knowledge' })).toBeInTheDocument();
  });

  it('renders memories route inside the workspace shell', async () => {
    const router = createAppRouter(['/memories']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agent Memories' })).toBeInTheDocument();
  });

  it('renders agent plan steps route inside the workspace shell', async () => {
    const router = createAppRouter(['/agent-runs/run_1/plan-steps']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agent Plan Steps' })).toBeInTheDocument();
    expect(await screen.findByText('Run run_1')).toBeInTheDocument();
  });

  it('renders agents route inside the workspace shell', async () => {
    const router = createAppRouter(['/agents']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Agents' })).toBeInTheDocument();
  });

  it('renders MCP servers route inside the workspace shell', async () => {
    const router = createAppRouter(['/mcp-servers']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'MCP Servers & Tools' })).toBeInTheDocument();
    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
  });

  it('renders onboarding inside the workspace shell', async () => {
    const router = createAppRouter(['/onboarding']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
  });

  it('renders solo route inside the workspace shell', async () => {
    const router = createAppRouter(['/solo']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'SOLO' })).toBeInTheDocument();
  });

  it('renders workflows route inside the workspace shell', async () => {
    const router = createAppRouter(['/workflows']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Workflows' })).toBeInTheDocument();
  });

  it('renders scheduled tasks route inside the workspace shell', async () => {
    const router = createAppRouter(['/scheduled-tasks']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Scheduled Tasks' })).toBeInTheDocument();
  });

  it('renders publishing channels route inside the workspace shell', async () => {
    const router = createAppRouter(['/publishing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Publishing Channels' })).toBeInTheDocument();
  });

  it('renders billing route inside the console shell', async () => {
    const router = createAppRouter(['/console/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
  });

  it('renders notifications route inside the console shell', async () => {
    const router = createAppRouter(['/console/notifications']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Notifications' })).toBeInTheDocument();
  });

  it('renders admin billing route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
  });

  it('renders admin usage logs route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/usage-logs']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Usage Logs' })).toBeInTheDocument();
  });

  it('renders admin API tokens route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/api-tokens']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'API Tokens' })).toBeInTheDocument();
  });

  it('renders admin models route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/models']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Models' })).toBeInTheDocument();
  });
});
