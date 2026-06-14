import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T16:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_commercial_config',
    expiresAt: '2026-06-16T16:00:00Z',
  },
  user: {
    id: 'user_admin_commercial_config',
    email: 'commercial-admin@example.com',
    name: 'Commercial Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_commercial_config',
  },
};

const starterPlan = {
  id: 'plan_browser_starter',
  name: 'Starter Browser',
  description: 'Initial browser fixture plan',
  quotaAmount: 1000,
  tokenQuota: 250000,
  price: 19,
  modelAccess: ['gpt-4.1-mini'],
  agentLimit: 2,
  maxTokensPerRequest: 8000,
  durationDays: 30,
  isActive: true,
  isPublic: true,
  sortOrder: 1,
  subscriberCount: 4,
  createdAt: now,
  updatedAt: now,
};

const growthPlan = {
  id: 'plan_browser_growth',
  name: 'Browser Growth',
  description: 'Filtered plan with request-token cap evidence',
  quotaAmount: 60000,
  tokenQuota: 1000000,
  price: 99,
  modelAccess: ['gpt-4o', 'claude-3-5-sonnet'],
  agentLimit: 10,
  maxTokensPerRequest: 32000,
  durationDays: 30,
  isActive: true,
  isPublic: true,
  sortOrder: 2,
  subscriberCount: 16,
  createdAt: now,
  updatedAt: now,
};

const browserUser = {
  id: 'user_browser_entitlement',
  email: 'buyer-browser@example.com',
  name: 'Browser Buyer',
  role: 'user',
  planID: 'plan_browser_growth',
  planName: 'Browser Growth',
  status: 'active',
  lastLoginAt: now,
  createdAt: now,
  usageStats: {
    totalTokens: 18000,
    totalAPICalls: 240,
    totalCost: 12.5,
  },
};

const usageLimit = {
  id: 'usage_limit_browser_request_cap',
  scopeType: 'organization',
  scopeId: 'org_browser_settings',
  organizationId: 'org_browser_settings',
  quotaMode: 'organization',
  limitType: 'request_tokens',
  period: 'minute',
  limitValue: 4096,
  maxConcurrentRequests: 8,
  windowSeconds: 60,
  maxTokensPerWindow: 120000,
  maxTokensPerRequest: 4096,
  enabled: true,
  updatedAt: now,
};

const rateLimitedLog = {
  id: 'usage_browser_limit_hit',
  organizationId: 'org_browser_settings',
  userId: 'user_browser_settings',
  requestId: 'req_browser_limited',
  apiType: 'chat',
  featureType: 'workspace_chat',
  quotaMode: 'organization',
  model: 'gpt-4o',
  channelId: 'channel_browser_primary',
  provider: 'openai',
  status: 'error',
  statusCode: 429,
  errorCode: 'relay_rate_limited',
  latencyMs: 18,
  cost: 0,
  channelCost: 0,
  promptTokens: 0,
  completionTokens: 0,
  totalTokens: 0,
  createdAt: now,
};

const recoveredLog = {
  id: 'usage_browser_recovery',
  organizationId: 'org_browser_settings',
  userId: 'user_browser_settings',
  requestId: 'req_browser_recovered',
  apiType: 'chat',
  featureType: 'workspace_chat',
  quotaMode: 'organization',
  model: 'gpt-4o',
  channelId: 'channel_browser_primary',
  provider: 'openai',
  status: 'success',
  statusCode: 200,
  latencyMs: 42,
  cost: 0.04,
  channelCost: 0.01,
  promptTokens: 800,
  completionTokens: 120,
  totalTokens: 920,
  createdAt: '2026-06-15T15:59:00Z',
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
      error: { code: 'not_found', message: 'admin commercial config fixture route not found' },
    }),
  });
}

function queryHas(url: URL, expected: Record<string, string>) {
  return Object.entries(expected).every(([key, value]) => url.searchParams.get(key) === value);
}

function finalPlanQuery(url: URL) {
  return queryHas(url, {
    search: 'browser plan',
    status: 'active',
    isPublic: 'true',
    limit: '100',
  });
}

function finalUserQuery(url: URL) {
  return queryHas(url, {
    search: 'buyer_browser',
    role: 'user',
    status: 'active',
    limit: '100',
  });
}

function usageLimitErrorQuery(url: URL) {
  return queryHas(url, {
    organizationID: 'org_browser_settings',
    status: 'error',
    limit: '50',
  });
}

function usageLimitSuccessQuery(url: URL) {
  return queryHas(url, {
    organizationID: 'org_browser_settings',
    status: 'success',
    limit: '1',
  });
}

function planPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.name === 'Browser Enterprise' &&
    payload.description === 'Browser proof plan with per-request token cap' &&
    payload.price === 199 &&
    payload.quotaAmount === 120000 &&
    payload.tokenQuota === 2000000 &&
    payload.agentLimit === 25 &&
    payload.maxTokensPerRequest === 64000 &&
    Array.isArray(payload.modelAccess) &&
    payload.modelAccess.join(',') === 'gpt-4o,claude-3-5-sonnet' &&
    payload.durationDays === 30 &&
    payload.isPublic === true &&
    payload.sortOrder === 7
  );
}

function userUpdatePayloadMatches(payload: Record<string, unknown>) {
  return payload.role === 'admin' && payload.planID === 'plan_browser_enterprise' && payload.status === 'disabled';
}

function pricingPayloadMatches(payload: Record<string, unknown>) {
  const modelMultipliers = payload.modelMultipliers as Record<string, unknown> | undefined;
  const groupMultipliers = payload.groupMultipliers as Record<string, unknown> | undefined;
  return (
    modelMultipliers?.['gpt-4o'] === 1.35 &&
    modelMultipliers?.['claude-3-5-sonnet'] === 1.6 &&
    groupMultipliers?.enterprise === 0.85 &&
    groupMultipliers?.vip === 0.75
  );
}

function usageLimitPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.id === 'usage_limit_browser_request_cap' &&
    payload.scopeType === 'organization' &&
    payload.scopeId === 'org_browser_settings' &&
    payload.limitType === 'request_tokens' &&
    payload.period === 'minute' &&
    payload.limitValue === 8192 &&
    payload.enabled === true &&
    payload.quotaMode === 'organization' &&
    payload.maxTokensPerRequest === 8192 &&
    payload.windowSeconds === 60
  );
}

export async function registerAdminCommercialConfigRoutes(page: Page): Promise<void> {
  let createdPlan: typeof growthPlan | null = null;
  let userState = { ...browserUser };
  let usageLimitState = { ...usageLimit };
  let pricingState = {
    modelMultipliers: { 'gpt-4.1-mini': 1, 'gpt-4o': 1.2 },
    groupMultipliers: { default: 1, enterprise: 0.9 },
  };

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/plans') {
      if (finalPlanQuery(url)) {
        await fulfillJSON(route, { plans: [growthPlan, ...(createdPlan ? [createdPlan] : [])], total: createdPlan ? 2 : 1 });
        return;
      }
      await fulfillJSON(route, { plans: [starterPlan], total: 1 });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/plans') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!planPayloadMatches(payload)) {
        await fulfillError(route, 'plan payload did not preserve commercial quota fields');
        return;
      }
      createdPlan = {
        ...growthPlan,
        id: 'plan_browser_enterprise',
        name: 'Browser Enterprise',
        description: 'Browser proof plan with per-request token cap',
        price: 199,
        quotaAmount: 120000,
        tokenQuota: 2000000,
        agentLimit: 25,
        maxTokensPerRequest: 64000,
        modelAccess: ['gpt-4o', 'claude-3-5-sonnet'],
        sortOrder: 7,
        subscriberCount: 0,
      };
      await fulfillJSON(route, createdPlan, 201);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/users') {
      if (!finalUserQuery(url) && url.searchParams.get('search')) {
        await fulfillError(route, 'user query did not include expected commercial entitlement filters');
        return;
      }
      await fulfillJSON(route, { users: [userState], total: 1 });
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/admin/users/user_browser_entitlement') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!userUpdatePayloadMatches(payload)) {
        await fulfillError(route, 'user update payload did not include role, planID, and disabled status');
        return;
      }
      userState = {
        ...userState,
        role: 'admin',
        planID: 'plan_browser_enterprise',
        planName: 'Browser Enterprise',
        status: 'disabled',
      };
      await fulfillJSON(route, userState);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/users/user_browser_entitlement/enable') {
      userState = { ...userState, status: 'active' };
      await fulfillJSON(route, { status: 'enabled' });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/settings/relay-pricing') {
      await fulfillJSON(route, pricingState);
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/admin/settings/relay-pricing') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!pricingPayloadMatches(payload)) {
        await fulfillError(route, 'relay pricing payload did not preserve model and group multipliers');
        return;
      }
      pricingState = payload as typeof pricingState;
      await fulfillJSON(route, pricingState);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/settings/usage-limits') {
      await fulfillJSON(route, { usageLimits: [usageLimitState] });
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/admin/settings/usage-limits') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!usageLimitPayloadMatches(payload)) {
        await fulfillError(route, 'usage-limit payload did not preserve request-token cap runtime fields');
        return;
      }
      usageLimitState = { ...usageLimitState, limitValue: 8192, maxTokensPerRequest: 8192 };
      await fulfillJSON(route, usageLimitState);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/usage-logs') {
      if (usageLimitErrorQuery(url)) {
        await fulfillJSON(route, { usageLogs: [rateLimitedLog], total: 1 });
        return;
      }
      if (usageLimitSuccessQuery(url)) {
        await fulfillJSON(route, { usageLogs: [recoveredLog], total: 1 });
        return;
      }
      await fulfillError(route, 'usage-limit runtime signal query did not match expected filters');
      return;
    }

    await fulfillNotFound(route);
  });
}
