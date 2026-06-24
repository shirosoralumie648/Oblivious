import type { Page, Route } from '@playwright/test';

const now = '2026-06-17T08:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_models',
    expiresAt: '2026-06-18T08:00:00Z',
  },
  user: {
    id: 'user_admin_models',
    email: 'admin-models@example.com',
    name: 'Models Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_models',
  },
};

const initialModels = [
  {
    model: 'gpt-4o',
    providers: ['openai', 'azure'],
    groups: ['default', 'enterprise'],
    channelCount: 3,
    enabledChannelCount: 2,
    disabledChannelCount: 1,
    minEstimatedCostPer1K: 0.002,
    maxEstimatedCostPer1K: 0.006,
    avgCostMultiplier: 1.18,
    requestCount: 1280,
    totalCost: 12.48,
    totalChannelCost: 7.1,
    channels: [
      {
        id: 'channel_openai_browser_primary',
        name: 'OpenAI Browser Primary',
        provider: 'openai',
        groups: ['default', 'enterprise'],
        enabled: true,
        priority: 1,
        estimatedCostPer1K: 0.002,
        costMultiplier: 1,
      },
      {
        id: 'channel_azure_browser_fallback',
        name: 'Azure Browser Fallback',
        provider: 'azure',
        groups: ['enterprise'],
        enabled: true,
        priority: 2,
        estimatedCostPer1K: 0.006,
        costMultiplier: 1.35,
      },
      {
        id: 'channel_openai_browser_disabled',
        name: 'OpenAI Browser Disabled',
        provider: 'openai',
        groups: ['default'],
        enabled: false,
        priority: 9,
        estimatedCostPer1K: 0.003,
        costMultiplier: 1.2,
      },
    ],
  },
  {
    model: 'claude-3-5-sonnet',
    providers: ['anthropic'],
    groups: ['default'],
    channelCount: 1,
    enabledChannelCount: 1,
    disabledChannelCount: 0,
    minEstimatedCostPer1K: 0.004,
    maxEstimatedCostPer1K: 0.004,
    avgCostMultiplier: 1,
    requestCount: 640,
    totalCost: 6.4,
    totalChannelCost: 4.8,
    channels: [
      {
        id: 'channel_claude_browser_primary',
        name: 'Claude Browser Primary',
        provider: 'anthropic',
        groups: ['default'],
        enabled: true,
        priority: 1,
        estimatedCostPer1K: 0.004,
        costMultiplier: 1,
      },
    ],
  },
];

const filteredModel = {
  model: 'gpt-4o',
  providers: ['openai'],
  groups: ['enterprise'],
  channelCount: 1,
  enabledChannelCount: 1,
  disabledChannelCount: 0,
  minEstimatedCostPer1K: 0.002,
  maxEstimatedCostPer1K: 0.002,
  avgCostMultiplier: 1.05,
  requestCount: 2560,
  totalCost: 18.2,
  totalChannelCost: 10.5,
  channels: [
    {
      id: 'channel_openai_enterprise_browser',
      name: 'OpenAI Enterprise Browser',
      provider: 'openai',
      groups: ['enterprise'],
      enabled: true,
      priority: 1,
      estimatedCostPer1K: 0.002,
      costMultiplier: 1.05,
    },
  ],
};

const mobileModel = {
  model: 'modelinventorymobilewithoutbreaks20260624',
  providers: ['providermodelsmobilewithoutbreaks20260624'],
  groups: ['groupmodelsmobilewithoutbreaks20260624'],
  channelCount: 1,
  enabledChannelCount: 1,
  disabledChannelCount: 0,
  minEstimatedCostPer1K: 0.0099,
  maxEstimatedCostPer1K: 0.0099,
  avgCostMultiplier: 1.42,
  requestCount: 9876,
  totalCost: 98.76,
  totalChannelCost: 54.32,
  channels: [
    {
      id: 'channel_models_mobile_without_breaks_primary',
      name: 'channelmodelsmobilewithoutbreaks20260624primary',
      provider: 'providermodelsmobilewithoutbreaks20260624',
      groups: ['groupmodelsmobilewithoutbreaks20260624'],
      enabled: true,
      priority: 1,
      estimatedCostPer1K: 0.0099,
      costMultiplier: 1.42,
    },
  ],
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

function hasInitialQuery(url: URL) {
  return url.searchParams.get('sort') === 'model:asc' && url.searchParams.get('limit') === '50' && url.searchParams.get('offset') === '0';
}

function hasFilteredQuery(url: URL) {
  return (
    url.searchParams.get('provider') === 'openai' &&
    url.searchParams.get('group') === 'enterprise' &&
    url.searchParams.get('status') === 'enabled' &&
    url.searchParams.get('search') === 'gpt-4' &&
    url.searchParams.get('limit') === '50' &&
    url.searchParams.get('offset') === '0'
  );
}

function hasMobileQuery(url: URL) {
  return (
    url.searchParams.get('provider') === 'providermodelsmobilewithoutbreaks20260624' &&
    url.searchParams.get('group') === 'groupmodelsmobilewithoutbreaks20260624' &&
    url.searchParams.get('status') === 'enabled' &&
    url.searchParams.get('search') === 'modelinventorymobilewithoutbreaks20260624' &&
    url.searchParams.get('sort') === 'model:asc' &&
    url.searchParams.get('limit') === '50' &&
    url.searchParams.get('offset') === '0'
  );
}

function onlyKnownParams(url: URL) {
  const allowed = new Set(['provider', 'group', 'status', 'search', 'sort', 'limit', 'offset']);
  return [...url.searchParams.keys()].every((key) => allowed.has(key));
}

export async function registerAdminModelsRoutes(page: Page): Promise<void> {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/models') {
      if (!onlyKnownParams(url)) {
        await fulfillError(route, 'admin models list sent an unexpected query parameter');
        return;
      }

      if (hasMobileQuery(url)) {
        await fulfillJSON(route, { models: [mobileModel], total: 1 });
        return;
      }

      if (hasFilteredQuery(url)) {
        if (url.searchParams.get('sort') === 'requestCount:desc') {
          await fulfillJSON(route, { models: [filteredModel], total: 1 });
          return;
        }
        if (url.searchParams.get('sort') === 'model:asc') {
          await fulfillJSON(route, { models: [filteredModel], total: 1 });
          return;
        }
        await fulfillError(route, 'filtered admin models query did not preserve sort=model:asc or sort=requestCount:desc');
        return;
      }

      if (!hasInitialQuery(url)) {
        await fulfillError(route, 'initial admin models query did not preserve sort=model:asc, limit=50, and offset=0');
        return;
      }

      await fulfillJSON(route, { models: initialModels, total: initialModels.length });
      return;
    }

    await fulfillError(route, `fixture route not implemented for ${method} ${pathname}`, 404);
  });
}
