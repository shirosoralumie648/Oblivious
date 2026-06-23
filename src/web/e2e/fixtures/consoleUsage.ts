import type { Page, Route } from '@playwright/test';

const now = '2026-06-17T08:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_console_usage',
    expiresAt: '2026-06-18T08:00:00Z',
  },
  user: {
    id: 'user_console_usage',
    email: 'usage-operator@example.com',
    name: 'Usage Operator',
    role: 'user',
  },
  workspace: {
    id: 'workspace_console_usage',
  },
};

const accessSummary = {
  defaultMode: 'chat',
  modelStrategy: 'balanced',
  networkEnabledHint: true,
  onboardingCompleted: true,
  sessionExpiresAt: session.session.expiresAt,
  sessionId: session.session.id,
  userEmail: session.user.email,
  userId: session.user.id,
  workspaceId: session.workspace.id,
};

const usageSummary = {
  period: '7d',
  requests: 7,
  byModel: [{ key: 'gpt-4o', requestCount: 4, totalTokens: 4200, totalCost: 0.84 }],
  byFeature: [{ key: 'chat', requestCount: 3, totalTokens: 2400, totalCost: 0.48 }],
  byUser: [{ key: 'usage-operator@example.com', requestCount: 7, totalTokens: 6600, totalCost: 1.32 }],
  timeSeries: [{ bucket: '2026-06-17', requestCount: 7, totalTokens: 6600, totalCost: 1.32 }],
  recent: [
    {
      id: 'usage_console_recent',
      apiTokenId: 'tok_console_usage',
      apiType: 'chat',
      completionTokens: 300,
      cost: 0.42,
      createdAt: now,
      latencyMs: 95,
      model: 'gpt-4o',
      promptTokens: 1200,
      requestId: 'req_console_usage',
      status: 'success',
      statusCode: 200,
      totalTokens: 1500,
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

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'console usage fixture route not found' },
    }),
  });
}

export async function registerConsoleUsageRoutes(page: Page): Promise<void> {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/console/access') {
      await fulfillJSON(route, accessSummary);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/console/usage') {
      if (url.searchParams.size > 0) {
        await fulfillError(route, 'console usage query params must be empty');
        return;
      }

      await fulfillJSON(route, usageSummary);
      return;
    }

    await fulfillNotFound(route);
  });
}
