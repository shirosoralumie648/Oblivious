import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T12:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_usage_logs',
    expiresAt: '2026-06-16T12:00:00Z',
  },
  user: {
    id: 'user_admin_usage_logs',
    email: 'usage-admin@example.com',
    name: 'Usage Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_usage_logs',
  },
};

const initialUsageLog = {
  id: 'usage_admin_initial',
  organizationId: 'org_initial',
  userId: 'user_initial',
  apiTokenId: 'tok_initial',
  requestId: 'req_initial',
  apiType: 'responses',
  featureType: 'console_usage',
  quotaMode: 'relay_billing',
  model: 'gpt-4.1-mini',
  channelId: 'channel_initial',
  provider: 'openai',
  status: 'success',
  statusCode: 200,
  latencyMs: 30,
  cost: 0.015,
  channelCost: 0.006,
  promptTokens: 300,
  completionTokens: 80,
  totalTokens: 380,
  createdAt: now,
};

const filteredUsageLog = {
  id: 'usage_admin_filtered',
  organizationId: 'org_browser',
  userId: 'user_browser',
  apiTokenId: 'tok_browser',
  requestId: 'req_browser_filtered',
  apiType: 'chat',
  featureType: 'workspace_chat',
  quotaMode: 'relay_billing',
  model: 'gpt-4o',
  channelId: 'channel_openai_primary',
  provider: 'openai',
  status: 'success',
  statusCode: 200,
  latencyMs: 88,
  cost: 0.1234,
  channelCost: 0.0456,
  promptTokens: 1000,
  completionTokens: 240,
  totalTokens: 1240,
  createdAt: now,
};

const mobileUsageLog = {
  id: 'usage_admin_mobile_without_breaks',
  organizationId: 'orgusagelogsmobilewithoutbreaks20260624',
  userId: 'userusagelogsmobilewithoutbreaks20260624',
  apiTokenId: 'tokusagelogsmobilewithoutbreaks20260624',
  requestId: 'requsagelogsmobilewithoutbreaks20260624',
  apiType: 'chat',
  featureType: 'workspacechatmobilewithoutbreaks20260624',
  quotaMode: 'relaybillingmobilewithoutbreaks20260624',
  model: 'modelusagelogsmobilewithoutbreaks20260624',
  channelId: 'channelusagelogsmobilewithoutbreaks20260624',
  provider: 'providerusagelogsmobilewithoutbreaks20260624',
  status: 'success',
  statusCode: 200,
  latencyMs: 144,
  cost: 0.2222,
  channelCost: 0.1111,
  promptTokens: 2000,
  completionTokens: 440,
  totalTokens: 2440,
  createdAt: now,
};

const initialAnalytics = {
  byModel: [{ dimension: 'model', key: 'gpt-4.1-mini', requestCount: 4, totalTokens: 1520, totalCost: 0.06 }],
  byFeature: [{ dimension: 'feature', key: 'console_usage', requestCount: 4, totalTokens: 1520, totalCost: 0.06 }],
  byUser: [{ dimension: 'user', key: 'user_initial', requestCount: 4, totalTokens: 1520, totalCost: 0.06 }],
  byTime: [{ dimension: 'time', key: '2026-06-15T00:00:00Z', requestCount: 4, totalTokens: 1520, totalCost: 0.06 }],
  byChannel: [{ dimension: 'channel', key: 'channel_initial', requestCount: 4, totalTokens: 1520, totalCost: 0.024 }],
  byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 4, totalTokens: 1520, totalCost: 0.06 }],
  crossDimensions: [],
};

const filteredAnalytics = {
  byModel: [{ dimension: 'model', key: 'gpt-4o', requestCount: 9, totalTokens: 11160, totalCost: 1.1106 }],
  byFeature: [{ dimension: 'feature', key: 'workspace_chat', requestCount: 9, totalTokens: 11160, totalCost: 1.1106 }],
  byUser: [{ dimension: 'user', key: 'user_browser', requestCount: 9, totalTokens: 11160, totalCost: 1.1106 }],
  byTime: [{ dimension: 'time', key: '2026-W25', requestCount: 9, totalTokens: 11160, totalCost: 1.1106 }],
  byChannel: [{ dimension: 'channel', key: 'channel_openai_primary', requestCount: 9, totalTokens: 11160, totalCost: 0.4104 }],
  byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 9, totalTokens: 11160, totalCost: 1.1106 }],
  crossDimensions: [
    {
      dimension: 'model_time',
      key: 'gpt-4o / 2026-W25',
      primary: 'gpt-4o',
      secondary: '2026-W25',
      requestCount: 9,
      totalTokens: 11160,
      totalCost: 1.1106,
    },
    {
      dimension: 'user_feature',
      key: 'user_browser / workspace_chat',
      primary: 'user_browser',
      secondary: 'workspace_chat',
      requestCount: 9,
      totalTokens: 11160,
      totalCost: 1.1106,
    },
  ],
};

const mobileAnalytics = {
  byModel: [
    {
      dimension: 'model',
      key: 'modelusagelogsmobilewithoutbreaks20260624',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 2.2222,
    },
  ],
  byFeature: [
    {
      dimension: 'feature',
      key: 'workspacechatmobilewithoutbreaks20260624',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 2.2222,
    },
  ],
  byUser: [
    {
      dimension: 'user',
      key: 'userusagelogsmobilewithoutbreaks20260624',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 2.2222,
    },
  ],
  byTime: [{ dimension: 'time', key: '2026-W25', requestCount: 11, totalTokens: 26840, totalCost: 2.2222 }],
  byChannel: [
    {
      dimension: 'channel',
      key: 'channelusagelogsmobilewithoutbreaks20260624',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 1.1111,
    },
  ],
  byProvider: [
    {
      dimension: 'provider',
      key: 'providerusagelogsmobilewithoutbreaks20260624',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 2.2222,
    },
  ],
  crossDimensions: [
    {
      dimension: 'model_time',
      key: 'modelusagelogsmobilewithoutbreaks20260624 / 2026-W25',
      primary: 'modelusagelogsmobilewithoutbreaks20260624',
      secondary: '2026-W25',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 2.2222,
    },
    {
      dimension: 'user_feature',
      key: 'userusagelogsmobilewithoutbreaks20260624 / workspacechatmobilewithoutbreaks20260624',
      primary: 'userusagelogsmobilewithoutbreaks20260624',
      secondary: 'workspacechatmobilewithoutbreaks20260624',
      requestCount: 11,
      totalTokens: 26840,
      totalCost: 2.2222,
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

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'admin usage logs fixture route not found' },
    }),
  });
}

function queryHas(url: URL, expected: Record<string, string>) {
  return Object.entries(expected).every(([key, value]) => url.searchParams.get(key) === value);
}

function usageLogQueryMatchesFinal(url: URL) {
  return queryHas(url, {
    organizationID: 'org_browser',
    userID: 'user_browser',
    apiTokenID: 'tok_browser',
    requestID: 'req_browser_filtered',
    apiType: 'chat',
    featureType: 'workspace_chat',
    quotaMode: 'relay_billing',
    channelID: 'channel_openai_primary',
    provider: 'openai',
    status: 'success',
    model: 'gpt-4o',
    limit: '50',
    offset: '0',
  });
}

function usageAnalyticsQueryMatchesFinal(url: URL) {
  return queryHas(url, {
    organizationID: 'org_browser',
    userID: 'user_browser',
    apiType: 'chat',
    featureType: 'workspace_chat',
    quotaMode: 'relay_billing',
    channelID: 'channel_openai_primary',
    provider: 'openai',
    status: 'success',
    model: 'gpt-4o',
    granularity: 'week',
    limit: '8',
  });
}

function usageLogQueryMatchesMobile(url: URL) {
  return queryHas(url, {
    organizationID: 'orgusagelogsmobilewithoutbreaks20260624',
    userID: 'userusagelogsmobilewithoutbreaks20260624',
    apiTokenID: 'tokusagelogsmobilewithoutbreaks20260624',
    requestID: 'requsagelogsmobilewithoutbreaks20260624',
    apiType: 'chat',
    featureType: 'workspacechatmobilewithoutbreaks20260624',
    quotaMode: 'relaybillingmobilewithoutbreaks20260624',
    channelID: 'channelusagelogsmobilewithoutbreaks20260624',
    provider: 'providerusagelogsmobilewithoutbreaks20260624',
    status: 'success',
    model: 'modelusagelogsmobilewithoutbreaks20260624',
    limit: '50',
    offset: '0',
  });
}

function usageAnalyticsQueryMatchesMobile(url: URL) {
  return queryHas(url, {
    organizationID: 'orgusagelogsmobilewithoutbreaks20260624',
    userID: 'userusagelogsmobilewithoutbreaks20260624',
    apiType: 'chat',
    featureType: 'workspacechatmobilewithoutbreaks20260624',
    quotaMode: 'relaybillingmobilewithoutbreaks20260624',
    channelID: 'channelusagelogsmobilewithoutbreaks20260624',
    provider: 'providerusagelogsmobilewithoutbreaks20260624',
    status: 'success',
    model: 'modelusagelogsmobilewithoutbreaks20260624',
    granularity: 'week',
    limit: '8',
  });
}

export async function registerAdminUsageLogsRoutes(page: Page): Promise<void> {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/usage-logs') {
      if (usageLogQueryMatchesMobile(url)) {
        await fulfillJSON(route, { usageLogs: [mobileUsageLog], total: 1 });
        return;
      }
      if (usageLogQueryMatchesFinal(url)) {
        await fulfillJSON(route, { usageLogs: [filteredUsageLog], total: 1 });
        return;
      }
      await fulfillJSON(route, { usageLogs: [initialUsageLog], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/usage-analytics') {
      if (usageAnalyticsQueryMatchesMobile(url)) {
        await fulfillJSON(route, mobileAnalytics);
        return;
      }
      if (usageAnalyticsQueryMatchesFinal(url)) {
        await fulfillJSON(route, filteredAnalytics);
        return;
      }
      await fulfillJSON(route, initialAnalytics);
      return;
    }

    await fulfillNotFound(route);
  });
}
