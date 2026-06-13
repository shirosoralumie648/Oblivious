import type { Page, Route } from '@playwright/test';

const now = '2026-05-02T12:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin',
    expiresAt: '2026-05-03T12:00:00Z',
  },
  user: {
    id: 'user_admin',
    email: 'admin@example.com',
    name: 'Release Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_release',
  },
};

const channel = {
  id: 'channel_openai_primary',
  name: 'OpenAI Primary',
  provider: 'openai',
  baseURL: 'https://relay.local/v1',
  models: ['gpt-4o-mini', 'gpt-4.1'],
  rpm: 120,
  tpm: 120000,
  priority: 1,
  weight: 100,
  enabled: true,
  status: 'online',
  latency: 86,
  createdAt: now,
  updatedAt: now,
};

const routeInfo = {
  id: 'route_primary',
  model: 'gpt-4o-mini',
  strategy: 'single',
  channels: [
    {
      channelID: channel.id,
      channelName: channel.name,
      priority: 1,
      weight: 100,
      enabled: true,
    },
  ],
  createdAt: now,
};

const plan = {
  id: 'plan_team',
  name: 'Team Release',
  description: 'Release-candidate validation plan',
  quotaAmount: 5000,
  tokenQuota: 1000000,
  price: 49,
  modelAccess: ['gpt-4o-mini'],
  agentLimit: 12,
  durationDays: 30,
  isActive: true,
  isPublic: true,
  sortOrder: 1,
  subscriberCount: 7,
  createdAt: now,
  updatedAt: now,
};

const releaseAgent = {
  id: 'agent_release_helper',
  ownerID: 'user_admin',
  ownerName: 'Release Admin',
  name: 'Release Helper',
  description: 'Guides release owners through quality gates, rollout notes, and smoke checks.',
  iconURL: '',
  categoryID: 'cat_productivity',
  categorySlug: 'productivity',
  categoryName: 'Productivity',
  tags: ['release', 'qa', 'ops'],
  tools: JSON.stringify({ tools: [{ name: 'checklist', description: 'Build a release checklist' }] }),
  exampleConversations: JSON.stringify([{ userMessage: 'Prepare RC', assistantMessage: 'Run the release gate.' }]),
  systemPrompt: 'Help release owners validate candidates.',
  visibility: 'public',
  status: 'approved',
  pricingType: 'free',
  pricingAmount: 0,
  currentVersion: '1.0.0',
  installCount: 42,
  ratingAvg: 4.8,
  rating: 4.8,
  ratingCount: 12,
  createdAt: now,
  updatedAt: now,
};

const submittedAgent = {
  ...releaseAgent,
  id: 'agent_submitted_release_notes',
  name: 'Release Notes Drafter',
  status: 'pending_review',
  installCount: 0,
  ratingAvg: 0,
  rating: 0,
  ratingCount: 0,
};

const paidReleaseAgent = {
  ...releaseAgent,
  id: 'agent_paid_release_helper',
  name: 'Paid Release Operator',
  description: 'Runs paid release operations through checkout-backed Marketplace settlement.',
  pricingType: 'one_time',
  pricingAmount: 75,
  currentVersion: '1.1.0',
  installCount: 9,
  ratingAvg: 4.9,
  rating: 4.9,
  ratingCount: 5,
  paymentProviders: [{ name: 'stripe' }, { name: 'alipay' }],
};

const installedAgent = {
  id: 'install_release_helper',
  agentID: releaseAgent.id,
  agentName: releaseAgent.name,
  userID: 'user_admin',
  version: '1.0.0',
  installedAt: now,
};

function envelope(data: unknown) {
  return {
    ok: true,
    data,
    error: null,
  };
}

async function fulfillJSON(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(envelope(data)),
  });
}

async function fulfillError(route: Route, message: string, status = 422) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'fixture_contract_mismatch', message },
    }),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'fixture route not found' },
    }),
  });
}

export async function registerAdminMarketplaceRoutes(page: Page): Promise<void> {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/stats') {
      await fulfillJSON(route, {
        users: {
          totalUsers: 128,
          activeUsers: 117,
          newUsersToday: 3,
          newUsersWeek: 18,
        },
        quotas: {
          totalBalance: 250000,
          totalUsed: 84000,
          activeTopups: 9,
        },
        conversations: 410,
        agents: 29,
        tasks: 67,
        mcpServers: 6,
        channelsTotal: 2,
        channelsOnline: 2,
        activeAgents: 21,
        apiCalls24h: 9320,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/channels') {
      await fulfillJSON(route, { channels: [channel], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/channels/stats') {
      await fulfillJSON(route, {
        stats: [
          {
            channelID: channel.id,
            rpmCurrent: 7,
            tpmCurrent: 321,
            totalRequests: 84,
            successCount: 80,
            failureCount: 4,
            avgLatencyMs: channel.latency,
            affinityConversationCount: 3,
          },
        ],
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/channel-providers') {
      await fulfillJSON(route, {
        providers: [
          {
            id: 'openai',
            displayName: 'OpenAI',
            kind: 'openai_compatible',
            status: 'supported',
            defaultBaseURL: 'https://api.openai.com/v1',
          },
        ],
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/routes') {
      await fulfillJSON(route, { routes: [routeInfo], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/plans') {
      await fulfillJSON(route, { plans: [plan], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/users') {
      await fulfillJSON(route, {
        users: [
          {
            id: 'user_admin',
            email: 'admin@example.com',
            name: 'Release Admin',
            role: 'admin',
            planID: plan.id,
            planName: plan.name,
            status: 'active',
            lastLoginAt: now,
            createdAt: now,
            usageStats: {
              totalTokens: 12000,
              totalAPICalls: 54,
              totalCost: 2.4,
            },
          },
        ],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/audit-logs') {
      await fulfillJSON(route, {
        entries: [
          {
            id: 'audit_release',
            actorID: 'user_admin',
            actorEmail: 'admin@example.com',
            action: 'agent.approve',
            resourceType: 'agent',
            resourceID: releaseAgent.id,
            changes: JSON.stringify({ status: 'approved' }),
            ipAddress: '127.0.0.1',
            createdAt: now,
          },
        ],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/reviews') {
      await fulfillJSON(route, { reviews: [releaseAgent], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/featured') {
      await fulfillJSON(route, { agents: [releaseAgent, paidReleaseAgent], total: 2 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/categories') {
      await fulfillJSON(route, {
        categories: [
          {
            id: 'cat_productivity',
            name: 'Productivity',
            slug: 'productivity',
            displayOrder: 1,
            agentCount: 1,
          },
        ],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/search') {
      await fulfillJSON(route, { agents: [releaseAgent, paidReleaseAgent], total: 2 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/templates') {
      await fulfillJSON(route, { templates: [], total: 0 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/curated') {
      await fulfillJSON(route, {
        popular: [releaseAgent, paidReleaseAgent],
        topRated: [releaseAgent],
        recent: [releaseAgent],
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/agents/agent_release_helper') {
      await fulfillJSON(route, {
        agent: releaseAgent,
        versions: [{ id: 'version_release_helper_1', agentID: releaseAgent.id, version: '1.0.0', status: 'approved', createdAt: now }],
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/agents/agent_release_helper/versions') {
      await fulfillJSON(route, {
        versions: [{ id: 'version_release_helper_1', agentID: releaseAgent.id, version: '1.0.0', status: 'approved', createdAt: now }],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/agents/agent_release_helper/reviews') {
      await fulfillJSON(route, {
        reviews: [{ id: 'review_release', agentID: releaseAgent.id, userID: 'user_qa', userName: 'QA Owner', rating: 5, body: 'Reliable release workflow.', createdAt: now }],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/agents/agent_paid_release_helper') {
      await fulfillJSON(route, {
        agent: paidReleaseAgent,
        versions: [{ id: 'version_paid_release_1', agentID: paidReleaseAgent.id, version: '1.1.0', status: 'approved', createdAt: now }],
        paymentProviders: paidReleaseAgent.paymentProviders,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/agents/agent_paid_release_helper/versions') {
      await fulfillJSON(route, {
        versions: [{ id: 'version_paid_release_1', agentID: paidReleaseAgent.id, version: '1.1.0', status: 'approved', createdAt: now }],
        total: 1,
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/agents/agent_paid_release_helper/reviews') {
      await fulfillJSON(route, {
        reviews: [{ id: 'review_paid_release', agentID: paidReleaseAgent.id, userID: 'user_admin', userName: 'Release Admin', rating: 5, body: 'Checkout provider evidence is visible.', createdAt: now }],
        total: 1,
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/marketplace/agents/agent_release_helper/install') {
      await fulfillJSON(route, installedAgent, 201);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/marketplace/agents/agent_paid_release_helper/install') {
      const provider = url.searchParams.get('provider');
      const versionID = url.searchParams.get('versionID');
      if (provider !== 'alipay' || versionID !== 'version_paid_release_1') {
        await fulfillError(route, 'paid install did not carry the selected Alipay provider and version');
        return;
      }

      await fulfillJSON(route, {
        checkoutSessionId: 'cs_paid_release_browser',
        url: 'https://checkout.alipay.test/session/cs_paid_release_browser',
      }, 201);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/marketplace/installs/agent_release_helper') {
      await fulfillJSON(route, installedAgent, 201);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/marketplace/agents') {
      await fulfillJSON(route, submittedAgent, 201);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/my-agents') {
      await fulfillJSON(route, { agents: [submittedAgent], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/publisher/stats') {
      await fulfillJSON(route, {
        totalAgents: 1,
        totalInstalls: 1,
        activeUsers: 1,
        totalAPICalls: 0,
        grossRevenue: 0,
        platformFees: 0,
        netRevenue: 0,
        refundedAmount: 0,
        pendingSettlementAmount: 0,
        availableAmount: 0,
        payoutPendingAmount: 0,
        paidOutAmount: 0,
        perAgentStats: [
          {
            agentID: submittedAgent.id,
            agentName: submittedAgent.name,
            installCount: 1,
            activeUsers: 1,
            apiCallCount: 0,
          },
        ],
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/publisher/settlement-preferences') {
      await fulfillJSON(route, {
        cycle: 'monthly',
        label: 'Monthly',
        description: "Settles the previous month's Marketplace income on the first day of each month.",
        payoutBusinessDays: 5,
        processingFeePercent: 1,
        minimumPayoutAmount: 100,
        effectiveFrom: 'next_settlement_cycle',
      });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/marketplace/installs') {
      await fulfillJSON(route, { installs: [installedAgent], total: 1 });
      return;
    }

    await fulfillNotFound(route);
  });
}
