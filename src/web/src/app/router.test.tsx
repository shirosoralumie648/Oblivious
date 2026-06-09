import { fireEvent, render, screen, within } from '@testing-library/react';
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
    createApiToken: () =>
      Promise.resolve({
        rawToken: 'obv_router_secret',
        token: {
          id: 'tok_router_created',
          modelLimits: ['gpt-4o-mini'],
          modelLimitsEnabled: true,
          name: 'Router key',
          status: 'active',
          tokenPrefix: 'obv_router_created',
          usedQuota: 0,
          createdAt: '2026-06-09T00:00:00Z'
        }
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
        paymentProviders: [{ name: 'stripe' }, { name: 'alipay' }],
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
    getUsage: () =>
      Promise.resolve({
        period: '7d',
        requests: 5,
        byModel: [{ key: 'gpt-4o', requestCount: 3, totalTokens: 300, totalCost: 0.09 }],
        byFeature: [{ key: 'workflow', requestCount: 2, totalTokens: 180, totalCost: 0.04 }],
        byUser: [{ key: 'user_router', requestCount: 2, totalTokens: 240, totalCost: 0.12 }],
        timeSeries: [{ bucket: '2026-06-09', requestCount: 3, totalTokens: 280, totalCost: 0.09 }],
        recent: [
          {
            id: 'usage_router_console',
            apiTokenId: 'tok_router_1',
            requestId: 'req_router_console',
            apiType: 'chat',
            model: 'gpt-4o',
            channelId: 'ch_router_1',
            provider: 'openai',
            status: 'success',
            statusCode: 200,
            latencyMs: 42,
            cost: 0.004,
            promptTokens: 100,
            completionTokens: 50,
            totalTokens: 150,
            createdAt: '2026-06-09T00:00:00Z'
          }
        ]
      }),
    listApiTokenUsage: () =>
      Promise.resolve([
        {
          id: 'usage_router_1',
          apiTokenId: 'tok_router_1',
          requestId: 'req_router_1',
          apiType: 'chat',
          model: 'gpt-4o',
          channelId: 'ch_router_1',
          provider: 'openai',
          status: 'success',
          statusCode: 200,
          latencyMs: 42,
          cost: 0.004,
          promptTokens: 100,
          completionTokens: 50,
          totalTokens: 150,
          createdAt: '2026-06-09T00:00:00Z'
        }
      ]),
    listApiTokens: () =>
      Promise.resolve([
        {
          id: 'tok_router_1',
          modelLimits: ['gpt-4o'],
          modelLimitsEnabled: true,
          name: 'Router gateway key',
          status: 'active',
          tokenPrefix: 'obv_router',
          usedQuota: 2.5,
          createdAt: '2026-06-09T00:00:00Z'
        }
      ]),
    revokeApiToken: () => Promise.resolve()
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
        paymentIntents: { count: 1, refundedAmount: 10, totalAmount: 29 },
        webhookEvents: { count: 1, failedCount: 1 },
        settlements: { count: 1, grossAmount: 50 },
        payouts: { count: 1, totalAmount: 40 }
      }),
    listBillingSurface: (surface: string) =>
      Promise.resolve({
        data:
          surface === 'sessions'
            ? [
                {
                  id: 'bs_router_phase28',
                  model: 'gpt-4o',
                  settledAmount: 4.5,
                  status: 'settled',
                  createdAt: '2026-06-09T00:00:00Z'
                }
              ]
            : [],
        total: surface === 'sessions' ? 1 : 0
      }),
    listUsageLogs: () =>
      Promise.resolve({
        data: [
          {
            id: 'usage_admin_router',
            organizationId: 'org_router',
            userId: 'user_router',
            apiTokenId: 'tok_router_1',
            requestId: 'req_admin_router',
            apiType: 'chat',
            featureType: 'workspace_chat',
            quotaMode: 'relay_billing',
            model: 'gpt-4o',
            channelId: 'ch_router_1',
            provider: 'openai',
            status: 'success',
            statusCode: 200,
            latencyMs: 42,
            cost: 0.42,
            channelCost: 0.21,
            promptTokens: 100,
            completionTokens: 20,
            totalTokens: 120,
            createdAt: '2026-06-09T00:00:00Z'
          }
        ],
        total: 1
      }),
    getUsageAnalytics: () =>
      Promise.resolve({
        byModel: [{ dimension: 'model', key: 'gpt-4o', requestCount: 3, totalTokens: 150, totalCost: 0.0012 }],
        byFeature: [{ dimension: 'feature', key: 'chat', requestCount: 2, totalTokens: 120, totalCost: 0.0009 }],
        byUser: [{ dimension: 'user', key: 'user_router', requestCount: 4, totalTokens: 200, totalCost: 0.0015 }],
        byTime: [{ dimension: 'time', key: '2026-06-09T00:00:00Z', requestCount: 5, totalTokens: 300, totalCost: 0.002 }],
        byChannel: [{ dimension: 'channel', key: 'ch_router_1', requestCount: 6, totalTokens: 360, totalCost: 0.0025 }],
        byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 7, totalTokens: 420, totalCost: 0.003 }],
        crossDimensions: [
          {
            dimension: 'model_time',
            key: 'gpt-4o / 2026-06-09T00:00:00Z',
            primary: 'gpt-4o',
            secondary: '2026-06-09T00:00:00Z',
            requestCount: 9,
            totalTokens: 900,
            totalCost: 0.009
          }
        ]
      }),
    listReviews: () =>
      Promise.resolve({
        data: [
          {
            id: 'agent_review_router',
            name: 'Research Agent',
            description: 'Helps with research',
            ownerID: 'owner_1',
            ownerName: 'Publisher',
            status: 'pending_review',
            visibility: 'public',
            pricingType: 'one_time',
            pricingAmount: 19,
            categoryID: 'cat_1',
            categoryName: 'Productivity',
            tags: ['research'],
            ratingAvg: 4.5,
            ratingCount: 8,
            installCount: 120,
            reviewSLA: {
              submittedAt: '2026-06-02T13:00:00Z',
              manualDeadlineAt: '2026-06-05T13:00:00Z',
              manualSlaHours: 72,
              manualSlaStatus: 'due_soon',
              minutesUntilDeadline: 60,
              automatedReviewDeadlineAt: '2026-06-02T13:05:00Z',
              automatedReviewSlaMinutes: 5,
              automatedReviewSlaStatus: 'overdue',
              vipPublisher: true,
              publisherTier: 'vip',
              publisherTierSource: 'organization_metadata'
            },
            createdAt: '2026-01-01T00:00:00Z',
            updatedAt: '2026-01-02T00:00:00Z'
          }
        ],
        total: 1
      }),
    listAPITokens: () => Promise.resolve({ data: [], total: 0 }),
    listModelInventory: () => Promise.resolve({ data: [], total: 0 }),
    approveAgent: () => Promise.resolve(),
    rejectAgent: () => Promise.resolve(),
    requestAgentChanges: () => Promise.resolve(),
    revokeAPIToken: () => Promise.resolve()
  })
}));

vi.mock('../features/marketplace/api', () => ({
  createMarketplaceApi: () => ({
    getCategories: () => Promise.resolve([{ id: 'cat_1', name: 'Productivity', slug: 'productivity', agentCount: 1 }]),
    getAgent: () =>
      Promise.resolve({
        id: 'agent_1',
        ownerID: 'owner_1',
        ownerName: 'Publisher',
        name: 'Research Agent',
        description: 'Helps with research workflows',
        categoryName: 'Productivity',
        tags: ['research', 'writing'],
        tools: '[{"name":"search"}]',
        exampleConversations: 'User asks for a market scan.',
        visibility: 'public',
        status: 'approved',
        pricingType: 'one_time',
        pricingAmount: 19,
        currentVersion: '1.0.0',
        installCount: 120,
        ratingAvg: 4.5,
        ratingCount: 8,
        paymentProviders: [{ name: 'stripe' }, { name: 'alipay' }],
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-02T00:00:00Z'
      }),
    getReviews: () =>
      Promise.resolve([
        {
          id: 'review_1',
          agentID: 'agent_1',
          userID: 'user_1',
          rating: 5,
          body: 'Great for launch research.',
          createdAt: '2026-01-03T00:00:00Z'
        }
      ]),
    getVersions: () => Promise.resolve([{ id: 'ver_1', version: '1.0.0', createdAt: '2026-01-01T00:00:00Z' }]),
    installAgent: () => Promise.resolve({ checkoutSessionId: 'cs_marketplace_1', url: 'https://checkout.example/session' }),
    publishAgent: () =>
      Promise.resolve({
        id: 'agent_published_router',
        name: 'Published Router Agent',
        status: 'pending_review'
      }),
    submitReview: () => Promise.resolve({ id: 'review_2', agentID: 'agent_1', userID: 'user_1', rating: 5, body: 'Useful' })
  }),
  getMarketplaceCheckoutUrl: (value: { url?: string; checkoutUrl?: string; checkoutURL?: string }) =>
    value.url ?? value.checkoutUrl ?? value.checkoutURL ?? '',
  isMarketplaceCheckoutResponse: (value: { checkoutSessionId?: string; checkoutSessionID?: string }) =>
    Boolean(value.checkoutSessionId ?? value.checkoutSessionID)
}));

vi.mock('../features/scheduledTasks/scheduledTasksApi', () => ({
  createScheduledTasksApi: () => ({
    createScheduledTask: () =>
      Promise.resolve({
        cronExpression: '0 9 * * 1',
        enabled: true,
        id: 'schedule_2',
        name: 'Created workflow schedule',
        targetId: 'workflow_2',
        targetType: 'workflow'
      }),
    listRuns: () =>
      Promise.resolve([
        {
          createdAt: '2026-06-09T09:00:00Z',
          finishedAt: '2026-06-09T09:02:00Z',
          id: 'run_schedule_router_list',
          scheduledTaskId: 'schedule_1',
          startedAt: '2026-06-09T09:00:00Z',
          status: 'completed',
          updatedAt: '2026-06-09T09:02:00Z'
        }
      ]),
    listScheduledTasks: () =>
      Promise.resolve([
        {
          cronExpression: '0 9 * * 1',
          enabled: true,
          id: 'schedule_1',
          lastRunAt: '2026-06-02T09:00:00Z',
          name: 'Weekly workflow schedule',
          nextRunAt: '2026-06-09T09:00:00Z',
          targetId: 'workflow_1',
          targetType: 'workflow'
        }
      ]),
    runScheduledTaskNow: () =>
      Promise.resolve({
        createdAt: '2026-06-09T10:00:00Z',
        finishedAt: null,
        id: 'run_schedule_router_now',
        scheduledTaskId: 'schedule_1',
        startedAt: '2026-06-09T10:00:00Z',
        status: 'running',
        updatedAt: '2026-06-09T10:00:00Z'
      }),
    updateScheduledTaskEnabled: () =>
      Promise.resolve({
        cronExpression: '0 9 * * 1',
        enabled: false,
        id: 'schedule_1',
        lastRunAt: '2026-06-02T09:00:00Z',
        name: 'Weekly workflow schedule',
        nextRunAt: null,
        targetId: 'workflow_1',
        targetType: 'workflow'
      })
  })
}));

vi.mock('../features/agents/memoriesApi', () => ({
  createAgentMemoriesApi: () => ({
    createMemory: () =>
      Promise.resolve({
        agentId: 'agent_1',
        content: 'Remember launch checklist owner.',
        id: 'memory_created_router',
        importance: 4,
        metadata: { managedBy: 'workspace' },
        type: 'user_managed'
      }),
    deleteMemory: () => Promise.resolve(),
    exportMemories: () =>
      Promise.resolve({
        data: [
          {
            agentId: 'agent_1',
            content: 'Router export memory.',
            id: 'memory_export_router',
            importance: 5,
            metadata: { source: 'router-test' },
            type: 'user_managed'
          }
        ],
        total: 1
      }),
    importMemories: () =>
      Promise.resolve([
        {
          agentId: 'agent_1',
          content: 'Imported router memory.',
          id: 'memory_import_router',
          importance: 3,
          metadata: { imported: true },
          type: 'user_managed'
        }
      ]),
    searchMemories: () =>
      Promise.resolve({
        data: [
          {
            agentId: 'agent_1',
            content: 'Router search memory.',
            id: 'memory_search_router',
            importance: 5,
            metadata: { source: 'router-test' },
            type: 'user_managed'
          }
        ],
        total: 1
      }),
    updateMemory: () =>
      Promise.resolve({
        agentId: 'agent_1',
        content: 'Updated router memory.',
        id: 'memory_search_router',
        importance: 4,
        metadata: { source: 'router-test' },
        type: 'user_managed'
      })
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
    listChannelMessages: () =>
      Promise.resolve([
        {
          created_at: '2026-06-09T00:00:00Z',
          direction: 'outbound',
          id: 'log_router_recent',
          status: 'delivered',
          transform_success: true
        }
      ]),
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
    listFailedChannelMessages: () =>
      Promise.resolve([
        {
          created_at: '2026-06-09T00:01:00Z',
          failure_reason: 'adapter timeout',
          id: 'log_router_failed',
          next_retry_at: '2026-06-09T00:05:00Z',
          retry_count: 2,
          status: 'failed',
          transform_success: true
        }
      ]),
    retryFailedChannelMessages: () => Promise.resolve({ claimed: 1, failed: 0, permanentFailures: 0, succeeded: 1 }),
    sendChannelMessage: () => Promise.resolve({ id: 'log_1', status: 'recorded', transform_success: true }),
    testChannel: () => Promise.resolve({ message: 'channel adapter is available', status: 'success' }),
    updateChannelStatus: (channel: unknown) => Promise.resolve(channel)
  })
}));

vi.mock('../features/agents/planStepsApi', () => ({
  createAgentPlanStepsApi: () => ({
    approvePlanStep: () => Promise.resolve([]),
    approveToolRun: () => Promise.resolve({ id: 'run_1', planSteps: [], status: 'running', toolRuns: [] }),
    getRunDetail: () => Promise.resolve({ id: 'run_1', planSteps: [], status: 'running', toolRuns: [] }),
    rejectToolRun: () => Promise.resolve({ id: 'run_1', planSteps: [], status: 'failed', toolRuns: [] }),
    retryToolRun: () => Promise.resolve({ id: 'run_1', planSteps: [], status: 'running', toolRuns: [] }),
    executePlanStep: () => Promise.resolve([])
  })
}));

vi.mock('../features/agents/agentsApi', () => ({
  createAgentsApi: () => ({
    createAgent: (agent: unknown) => Promise.resolve(agent),
    createRun: () =>
      Promise.resolve({
        id: 'run_router_agent',
        planSteps: [],
        status: 'pending_approval',
        toolRuns: []
      }),
    deleteAgent: () => Promise.resolve(),
    getAgent: () =>
      Promise.resolve({
        config: { approvalMode: 'tiered', defaultExecutionMode: 'planning' },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: []
      }),
    getAgentTools: () =>
      Promise.resolve([
        {
          description: 'Search the web with tenant policy controls.',
          inputSchema: { type: 'object' },
          name: 'web_search',
          requiresApproval: true,
          riskLevel: 'medium',
          toolType: 'builtin'
        }
      ]),
    listAgents: () =>
      Promise.resolve([
        {
          config: {
            approvalMode: 'tiered',
            defaultExecutionMode: 'planning',
            longTermMemoryExtractionPolicy: 'deterministic',
            longTermMemoryUpdatePolicy: 'exact_refresh',
            longTermMemoryWritePolicy: 'interaction_and_explicit',
            maxIterations: 8,
            tokenBudget: 30000
          },
          description: 'Router-level research automation.',
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
    listServerTools: () =>
      Promise.resolve([
        {
          description: 'Search tenant-safe docs',
          inputSchema: { type: 'object' },
          name: 'search_docs'
        }
      ]),
    listServers: () =>
      Promise.resolve([
        {
          hasAuthToken: true,
          id: 'mcp_remote',
          name: 'Remote MCP',
          status: 'disconnected',
          url: 'https://mcp.example/sse'
        }
      ])
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

  it('keeps agent memories route-level CRUD and import-export controls reachable', async () => {
    const router = createAppRouter(['/memories']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Agent Memories' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Optional agent ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory content')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory importance')).toHaveValue('3');
    expect(screen.getByRole('button', { name: 'Create memory' })).toBeDisabled();
    expect(screen.getByLabelText('Search query')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory type')).toHaveValue('');
    expect(screen.getByLabelText('Result limit')).toHaveValue(10);
    expect(screen.getByRole('button', { name: 'Export memories' })).toBeDisabled();
    expect(screen.getByLabelText('Import memories JSON')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Search query'), { target: { value: 'router' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search memories' }));

    expect(await screen.findByText('Router search memory.')).toBeInTheDocument();
    expect(screen.getByText('Agent: agent_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Importance 5 of 5')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit memory' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete memory' })).toBeInTheDocument();
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

  it('keeps agents route-level policy, run, and tool catalog controls reachable', async () => {
    const router = createAppRouter(['/agents']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Agents' })).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Research Agent' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getAllByText('gpt-4o-mini').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Router-level research automation.')).toBeInTheDocument();
    expect(screen.getByLabelText('Approval mode')).toHaveValue('tiered');
    expect(screen.getByRole('heading', { name: 'Execution limits' })).toBeInTheDocument();
    expect(screen.getByLabelText('Default execution mode')).toHaveValue('planning');
    expect(screen.getByLabelText('Max iterations')).toHaveValue(8);
    expect(screen.getByLabelText('Token budget')).toHaveValue(30000);
    expect(screen.getByLabelText('Long-term memory writes')).toHaveValue('interaction_and_explicit');
    expect(screen.getByLabelText('Long-term memory extraction')).toHaveValue('deterministic');
    expect(screen.getByLabelText('Long-term memory update')).toHaveValue('exact_refresh');
    expect(screen.getByRole('heading', { name: 'Start run' })).toBeInTheDocument();
    expect(screen.getByLabelText('Run conversation ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Run mode')).toHaveValue('planning');
    expect(screen.getByLabelText('Run goal')).toBeInTheDocument();
    expect(screen.getByLabelText('Run max iterations')).toHaveValue(8);
    expect(screen.getByLabelText('Run token budget')).toHaveValue(30000);
    expect(screen.getByRole('button', { name: 'Start run' })).toBeDisabled();
    expect(screen.getByRole('heading', { name: 'Tool approval policy' })).toBeInTheDocument();
    expect(screen.getByText('No tools enabled for this agent.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add custom API tool' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Available tool catalog' })).toBeInTheDocument();
    expect(screen.getByText('Tool definitions load on demand for the selected agent.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Load tool catalog' }));
    expect(await screen.findByText('web_search')).toBeInTheDocument();
    expect(screen.getByText('builtin / medium / approval required')).toBeInTheDocument();
    expect(screen.getByText('Search the web with tenant policy controls.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Enable tool web_search' })).toBeEnabled();

    fireEvent.change(screen.getByLabelText('Run conversation ID'), { target: { value: 'conv_router_agent' } });
    fireEvent.change(screen.getByLabelText('Run goal'), { target: { value: 'Plan a release readiness review.' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start run' }));

    expect(await screen.findByRole('link', { name: 'Open run plan steps' })).toHaveAttribute(
      'href',
      '/agent-runs/run_router_agent/plan-steps'
    );
  });

  it('renders MCP servers route inside the workspace shell', async () => {
    const router = createAppRouter(['/mcp-servers']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'MCP Servers & Tools' })).toBeInTheDocument();
    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
  });

  it('keeps MCP route-level server and tool controls reachable', async () => {
    const router = createAppRouter(['/mcp-servers']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'MCP Servers & Tools' })).toBeInTheDocument();
    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
    expect(await screen.findByLabelText('Local MCP servers')).toBeInTheDocument();
    expect(await screen.findByLabelText('Server name')).toBeInTheDocument();
    expect(screen.getByLabelText('Endpoint URL')).toBeInTheDocument();
    expect(screen.getByLabelText('Auth token')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add MCP server' })).toBeDisabled();
    expect(await screen.findByText('Remote MCP')).toBeInTheDocument();
    expect(screen.getByText('Auth token configured')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Connect' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disconnect' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Diagnose' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'List tools' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete Remote MCP' })).toBeInTheDocument();
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

  it('keeps scheduled tasks route-level creation and run controls reachable', async () => {
    const router = createAppRouter(['/scheduled-tasks']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Workflows' })).toHaveAttribute('href', '/workflows');
    expect(await screen.findByRole('heading', { name: 'Scheduled Tasks' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Create scheduled task')).toBeInTheDocument();
    expect(screen.getByLabelText('Schedule name')).toBeInTheDocument();
    expect(screen.getByLabelText('Target type')).toHaveValue('workflow');
    expect(screen.getByLabelText('Target ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Cron expression')).toBeInTheDocument();
    expect(screen.getByLabelText('Enabled')).toBeChecked();
    expect(screen.getByRole('button', { name: 'Create schedule' })).toBeDisabled();
    expect(await screen.findByLabelText('Scheduled task list')).toBeInTheDocument();
    expect(await screen.findByText('Weekly workflow schedule')).toBeInTheDocument();
    expect(screen.getByText('workflow_1')).toBeInTheDocument();
    expect(screen.getByText('0 9 * * 1')).toBeInTheDocument();
    expect(screen.getByText('Next: 2026-06-09 09:00')).toBeInTheDocument();
    expect(screen.getByText('Last: 2026-06-02 09:00')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disable workflow_1 schedule' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Run workflow_1 schedule now' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Show recent runs for workflow_1' })).toHaveAttribute(
      'aria-expanded',
      'false'
    );

    fireEvent.click(screen.getByRole('button', { name: 'Show recent runs for workflow_1' }));

    expect(await screen.findByLabelText('Recent runs for workflow_1')).toBeInTheDocument();
    expect(await screen.findByText('run_schedule_router_list')).toBeInTheDocument();
    expect(screen.getByText('Started: 2026-06-09 09:00')).toBeInTheDocument();
    expect(screen.getByText('Finished: 2026-06-09 09:02')).toBeInTheDocument();
    expect(screen.getByText('Error: None')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refresh recent runs for workflow_1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Hide recent runs for workflow_1' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Run workflow_1 schedule now' }));

    expect(await screen.findByText('run_schedule_router_now')).toBeInTheDocument();
    expect(screen.getByText('Started: 2026-06-09 10:00')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Disable workflow_1 schedule' }));

    expect(await screen.findByText('Disabled')).toBeInTheDocument();
  });

  it('renders publishing channels route inside the workspace shell', async () => {
    const router = createAppRouter(['/publishing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Publishing Channels' })).toBeInTheDocument();
  });

  it('keeps publishing route-level delivery and failed-queue recovery controls reachable', async () => {
    const router = createAppRouter(['/publishing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Workflows' })).toHaveAttribute('href', '/workflows');
    expect(await screen.findByRole('heading', { name: 'Publishing Channels' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Create publishing channel')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel name')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel type')).toHaveValue('webhook');
    expect(screen.getByLabelText('Endpoint URL')).toBeInTheDocument();
    expect(screen.getByLabelText('Shared secret')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create channel' })).toBeDisabled();
    expect(await screen.findByLabelText('Publishing channel send test')).toBeInTheDocument();
    expect(screen.getByLabelText('Channel')).toHaveValue('channel_1');
    expect(screen.getByLabelText('Conversation ID')).toBeInTheDocument();
    expect(screen.getByLabelText('Message text')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
    expect(await screen.findByLabelText('Publishing channel message visibility')).toBeInTheDocument();
    expect(await screen.findByText('1 recent / 1 failed')).toBeInTheDocument();
    expect(screen.getAllByText('log_router_recent').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('log_router_failed').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('adapter timeout')).toBeInTheDocument();
    expect(screen.getByLabelText('Failed retry queue controls')).toBeInTheDocument();
    expect(screen.getByLabelText('Fallback channel')).toHaveValue('');
    expect(screen.getByLabelText('Retry limit')).toHaveValue(null);
    expect(screen.getByRole('button', { name: 'Retry failed messages' })).toBeEnabled();
    expect(screen.getByLabelText('Publishing channel list')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Test channel Ops Webhook' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Disable Ops Webhook' })).toBeInTheDocument();
  });

  it('renders billing route inside the console shell', async () => {
    const router = createAppRouter(['/console/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
  });

  it('keeps console billing route-level landmarks and top-up controls reachable', async () => {
    const router = createAppRouter(['/console/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Back to overview' })).toHaveAttribute('href', '/console');
    expect(await screen.findByRole('link', { name: 'Open usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByLabelText('Top-up amount USD')).toHaveValue(25);
    expect(await screen.findByLabelText('Payment provider')).toHaveValue('stripe');
    expect(screen.getByRole('option', { name: 'Alipay' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start top-up checkout' })).toBeEnabled();
    expect(screen.getByText('Invoice history')).toBeInTheDocument();
  });

  it('renders notifications route inside the console shell', async () => {
    const router = createAppRouter(['/console/notifications']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Notifications' })).toBeInTheDocument();
  });

  it('keeps console access route-level API token controls reachable', async () => {
    const router = createAppRouter(['/console/access']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Access' })).toHaveAttribute('href', '/console/access');
    expect(await screen.findByRole('heading', { name: 'Access' })).toBeInTheDocument();
    expect(await screen.findByRole('navigation', { name: 'Access sibling navigation' })).toBeInTheDocument();
    expect(await screen.findByText('API tokens')).toBeInTheDocument();
    expect(await screen.findByText('Router gateway key')).toBeInTheDocument();
    expect(await screen.findByLabelText('Token name')).toBeInTheDocument();
    expect(screen.getByLabelText('Allowed models')).toHaveValue('gpt-4o,gpt-4o-mini');
    expect(screen.getByRole('button', { name: 'Create API token' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'View usage for Router gateway key' })).toBeInTheDocument();
  });

  it('keeps console usage route-level analytics and recent relay evidence reachable', async () => {
    const router = createAppRouter(['/console/usage']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const consoleNavigation = await screen.findByRole('navigation', { name: 'Console navigation' });
    expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    expect(within(consoleNavigation).getByRole('link', { name: 'Usage' })).toHaveAttribute('href', '/console/usage');
    expect(await screen.findByRole('heading', { name: 'Usage' })).toBeInTheDocument();
    expect(await screen.findByRole('navigation', { name: 'Usage sibling navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'By model' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By feature' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Top users' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Daily trend' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Recent relay requests' })).toBeInTheDocument();
    expect(screen.getAllByText('gpt-4o').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('workflow')).toBeInTheDocument();
    expect(screen.getByText('user_router')).toBeInTheDocument();
    expect(screen.getByText('req_router_console')).toBeInTheDocument();
    expect(screen.getByText('openai / ch_router_1')).toBeInTheDocument();
  });

  it('renders admin billing route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
  });

  it('keeps admin billing route-level recovery controls and filters reachable', async () => {
    const router = createAppRouter(['/admin/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
    expect(await screen.findByText('bs_router_phase28')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Review failed webhooks' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Payment Intents' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Payouts' })).toBeInTheDocument();
    expect(screen.getByLabelText('Organization ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Provider filter')).toBeInTheDocument();
  });

  it('keeps marketplace agent detail route-level install and review controls reachable', async () => {
    const router = createAppRouter(['/marketplace/agents/agent_1']);

    render(<RouterProvider future={routerFuture} router={router} />);

    const workspaceNavigation = await screen.findByRole('navigation', { name: 'Workspace navigation' });
    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(within(workspaceNavigation).getByRole('link', { name: 'Agents' })).toHaveAttribute('href', '/agents');
    expect(await screen.findByRole('heading', { name: 'Research Agent' })).toBeInTheDocument();
    expect(await screen.findByText('Paid installs create a checkout-backed marketplace order before workspace installation.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Agent version')).toHaveValue('ver_1');
    expect(await screen.findByLabelText('Payment provider')).toHaveValue('stripe');
    expect(screen.getByRole('option', { name: 'Alipay' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Install Agent' })).toBeEnabled();
    expect(screen.getByText('Reviews')).toBeInTheDocument();
    expect(screen.getByLabelText('Review text')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Submit Review' })).toBeEnabled();
  });

  it('keeps marketplace publish route-level submission controls reachable', async () => {
    const router = createAppRouter(['/marketplace/publish']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Publish Agent' })).toBeInTheDocument();
    expect(await screen.findByText('Submit an agent for marketplace review.')).toBeInTheDocument();
    expect(await screen.findByLabelText('Name')).toBeInTheDocument();
    expect(await screen.findByLabelText('Category')).toHaveTextContent('Productivity');
    expect(screen.getByLabelText('Pricing')).toHaveValue('free');
    expect(screen.getByLabelText('Version')).toHaveValue('1.0.0');
    expect(screen.getByRole('button', { name: 'Publish Agent' })).toBeEnabled();
  });

  it('keeps admin review route-level SLA and decision controls reachable', async () => {
    const router = createAppRouter(['/admin/reviews']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Review Queue' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Review status filter')).toHaveValue('pending_review');
    expect(await screen.findByText('Research Agent')).toBeInTheDocument();
    expect(screen.getByText('Manual SLA: Due soon by 2026-06-05 13:00 UTC')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Approve agent Research Agent' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reject agent Research Agent' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Request changes for agent Research Agent' })).toBeInTheDocument();
  });

  it('renders admin usage logs route inside the admin shell', async () => {
    const router = createAppRouter(['/admin/usage-logs']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Usage Logs' })).toBeInTheDocument();
  });

  it('keeps admin usage logs route-level filters, analytics, and table evidence reachable', async () => {
    const router = createAppRouter(['/admin/usage-logs']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('complementary', { name: 'Admin navigation' })).toBeInTheDocument();
    expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Usage Logs' })).toBeInTheDocument();
    expect(await screen.findByLabelText('Organization ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('API token ID filter')).toBeInTheDocument();
    expect(screen.getByLabelText('Analytics granularity filter')).toHaveValue('day');
    expect(await screen.findByRole('heading', { name: 'Usage Analytics' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By model' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Cross dimensions' })).toBeInTheDocument();
    expect(screen.getByText('req_admin_router')).toBeInTheDocument();
    expect(screen.getByText('tok_router_1')).toBeInTheDocument();
    expect(screen.getAllByText('gpt-4o').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('openai / ch_router_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Success')).toBeInTheDocument();
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
