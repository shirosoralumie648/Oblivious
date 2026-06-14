import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T12:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_console_access',
    expiresAt: '2026-06-16T12:00:00Z',
  },
  user: {
    id: 'user_console_access',
    email: 'access-operator@example.com',
    name: 'Access Operator',
    role: 'user',
  },
  workspace: {
    id: 'workspace_console_access',
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

const initialToken = {
  id: 'tok_console_ci',
  name: 'CI gateway key',
  tokenPrefix: 'obv_ci_123',
  status: 'active',
  userGroup: 'ci',
  modelLimitsEnabled: true,
  modelLimits: ['gpt-4o'],
  quotaLimit: 10,
  usedQuota: 2.5,
  createdAt: now,
};

const createdToken = {
  id: 'tok_console_browser',
  name: 'Browser key',
  tokenPrefix: 'obv_browser',
  status: 'active',
  userGroup: 'vip',
  modelLimitsEnabled: true,
  modelLimits: ['gpt-4o-mini', 'gpt-4.1-mini'],
  quotaLimit: 25.5,
  usedQuota: 0,
  expiresAt: '2026-06-30T00:00:00Z',
  createdAt: now,
};

const tokenUsage = [
  {
    id: 'usage_console_ci',
    apiTokenId: initialToken.id,
    apiType: 'chat',
    completionTokens: 100,
    cost: 0.004,
    createdAt: now,
    latencyMs: 42,
    model: 'gpt-4o',
    promptTokens: 1000,
    requestId: 'req_console_ci',
    status: 'success',
    statusCode: 200,
    totalTokens: 1100,
  },
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
      error: { code: 'not_found', message: 'console access fixture route not found' },
    }),
  });
}

function createTokenPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.name === 'Browser key' &&
    payload.modelLimitsEnabled === true &&
    Array.isArray(payload.modelLimits) &&
    payload.modelLimits.length === 2 &&
    payload.modelLimits[0] === 'gpt-4o-mini' &&
    payload.modelLimits[1] === 'gpt-4.1-mini' &&
    payload.userGroup === 'vip' &&
    payload.quotaLimit === 25.5 &&
    payload.expiresAt === '2026-06-30T00:00:00Z'
  );
}

export async function registerConsoleAccessRoutes(page: Page): Promise<void> {
  let createdTokenVisible = false;
  let tokenRevoked = false;

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

    if (method === 'GET' && pathname === '/api/v1/console/api-tokens') {
      const currentInitialToken = tokenRevoked ? { ...initialToken, status: 'revoked' } : initialToken;
      await fulfillJSON(route, createdTokenVisible ? [createdToken, currentInitialToken] : [currentInitialToken]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/console/api-tokens') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!createTokenPayloadMatches(payload)) {
        await fulfillError(route, 'API token creation payload did not match browser form selections');
        return;
      }
      createdTokenVisible = true;
      await fulfillJSON(route, {
        rawToken: 'obv_browser_raw_secret',
        token: createdToken,
      }, 201);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/console/api-tokens/${initialToken.id}/usage`) {
      await fulfillJSON(route, tokenUsage);
      return;
    }

    if (method === 'DELETE' && pathname === `/api/v1/console/api-tokens/${initialToken.id}`) {
      tokenRevoked = true;
      await fulfillJSON(route, { status: 'revoked' });
      return;
    }

    await fulfillNotFound(route);
  });
}
