import type { Page, Route } from '@playwright/test';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_console_overview',
    expiresAt: '2026-06-18T08:00:00Z',
  },
  user: {
    id: 'user_console_overview',
    email: 'overview-operator@example.com',
    name: 'Overview Operator',
    role: 'user',
  },
  workspace: {
    id: 'workspace_console_overview',
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

const billingSummary = {
  period: '30d',
  requests: 11,
  inputTokens: 18000,
  outputTokens: 9000,
  estimatedCostUsd: 12.3456,
};

const usageSummary = {
  period: '7d',
  requests: 9,
};

const modelSummaries = [
  { id: 'model_balanced', label: 'balanced-chat', requests: 6 },
  { id: 'model_quality', label: 'quality-chat', requests: 3 },
  { id: 'model_long_context', label: 'providerresearchclusterultralongcontextmodel20260624previewwithunbrokenidentifier', requests: 1 },
];

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
      error: { code: 'not_found', message: 'console overview fixture route not found' },
    }),
  });
}

export async function registerConsoleOverviewRoutes(page: Page): Promise<void> {
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

    if (method === 'GET' && pathname === '/api/v1/console/billing') {
      await fulfillJSON(route, billingSummary);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/console/usage') {
      await fulfillJSON(route, usageSummary);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/console/models') {
      if (url.searchParams.size > 0) {
        await fulfillError(route, 'console models query params must be empty');
        return;
      }

      await fulfillJSON(route, modelSummaries);
      return;
    }

    await fulfillNotFound(route);
  });
}
