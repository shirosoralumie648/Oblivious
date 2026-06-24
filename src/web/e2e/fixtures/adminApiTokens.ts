import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T14:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_api_tokens',
    expiresAt: '2026-06-16T14:00:00Z',
  },
  user: {
    id: 'user_admin_api_tokens',
    email: 'api-token-admin@example.com',
    name: 'API Token Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_api_tokens',
  },
};

const initialToken = {
  id: 'tok_admin_initial',
  organizationId: 'org_initial',
  userId: 'user_initial',
  userEmail: 'initial-admin@example.com',
  name: 'Initial admin key',
  tokenPrefix: 'obv_admin_initial',
  status: 'active',
  userGroup: 'ops',
  modelLimitsEnabled: false,
  modelLimits: [],
  quotaLimit: null,
  usedQuota: 1.5,
  requestCount: 42,
  totalCost: 0.0123,
  createdAt: now,
  lastUsedAt: now,
};

const filteredToken = {
  id: 'tok_admin_browser',
  organizationId: 'org_browser_api_tokens',
  userId: 'user_browser_api_tokens',
  userEmail: 'browser-admin@example.com',
  name: 'Browser admin key',
  tokenPrefix: 'obv_admin_browser',
  status: 'active',
  userGroup: 'enterprise',
  modelLimitsEnabled: true,
  modelLimits: ['gpt-4o', 'gpt-4.1-mini'],
  quotaLimit: 25.5,
  usedQuota: 3.25,
  requestCount: 1234,
  totalCost: 0.1288,
  createdAt: now,
  lastUsedAt: now,
};

const mobileToken = {
  id: 'tok_admin_mobile_without_breaks',
  organizationId: 'orgapitokensmobilewithoutbreaks20260624',
  userId: 'userapitokensmobilewithoutbreaks20260624',
  userEmail: 'mobileapitokensuserwithoutbreaks20260624@example.com',
  name: 'mobileapitokennamewithoutbreaks20260624',
  tokenPrefix: 'obvadminmobileprefixwithoutbreaks20260624',
  status: 'active',
  userGroup: 'enterpriseapitokensmobilewithoutbreaks20260624',
  modelLimitsEnabled: true,
  modelLimits: ['modelapitokensmobilewithoutbreaks20260624', 'backupmodelapitokensmobilewithoutbreaks20260624'],
  quotaLimit: 99.5,
  usedQuota: 9.75,
  requestCount: 9876,
  totalCost: 4.5678,
  createdAt: now,
  lastUsedAt: now,
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
      error: { code: 'not_found', message: 'admin API tokens fixture route not found' },
    }),
  });
}

function queryHas(url: URL, expected: Record<string, string>) {
  return Object.entries(expected).every(([key, value]) => url.searchParams.get(key) === value);
}

function finalAPITokensQuery(url: URL) {
  return queryHas(url, {
    organizationID: 'org_browser_api_tokens',
    userID: 'user_browser_api_tokens',
    status: 'active',
    userGroup: 'enterprise',
    search: 'browser admin',
    model: 'gpt-4o',
    limit: '50',
    offset: '0',
  });
}

function mobileAPITokensQuery(url: URL) {
  return queryHas(url, {
    organizationID: 'orgapitokensmobilewithoutbreaks20260624',
    userID: 'userapitokensmobilewithoutbreaks20260624',
    status: 'active',
    userGroup: 'enterpriseapitokensmobilewithoutbreaks20260624',
    search: 'mobileadmintokenwithoutbreaks20260624',
    model: 'modelapitokensmobilewithoutbreaks20260624',
    limit: '50',
    offset: '0',
  });
}

export async function registerAdminAPITokensRoutes(page: Page): Promise<void> {
  let tokenRevoked = false;
  let finalFilterLoaded = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/api-tokens') {
      if (mobileAPITokensQuery(url)) {
        await fulfillJSON(route, { apiTokens: [mobileToken], total: 1 });
        return;
      }
      if (finalAPITokensQuery(url)) {
        finalFilterLoaded = true;
        const currentFilteredToken = tokenRevoked
          ? { ...filteredToken, status: 'revoked', revokedAt: '2026-06-15T14:10:00Z' }
          : filteredToken;
        await fulfillJSON(route, { apiTokens: [currentFilteredToken], total: 1 });
        return;
      }
      await fulfillJSON(route, { apiTokens: [initialToken], total: 1 });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/api-tokens/${filteredToken.id}/revoke`) {
      if (!finalFilterLoaded) {
        await fulfillError(route, 'API token revoke was attempted outside the filtered Admin scope');
        return;
      }
      tokenRevoked = true;
      await fulfillJSON(route, { status: 'revoked' });
      return;
    }

    await fulfillNotFound(route);
  });
}
