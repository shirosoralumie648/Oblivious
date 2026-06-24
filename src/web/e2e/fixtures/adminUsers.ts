import type { Page, Route } from '@playwright/test';

const now = '2026-06-24T12:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_users',
    expiresAt: '2026-06-25T12:00:00Z',
  },
  user: {
    id: 'user_admin_users',
    email: 'admin-users@example.com',
    name: 'Admin Users',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_users',
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
      error: { code: 'not_found', message: 'admin users fixture route not found' },
    }),
  });
}

const initialUser = {
  id: 'user_initial',
  email: 'initial-user@example.com',
  name: 'Initial User',
  role: 'user',
  planID: 'plan_basic',
  planName: 'Basic',
  quotaBalance: 250,
  status: 'active',
  lastLoginAt: now,
  createdAt: now,
  usageStats: {
    totalTokens: 1000,
    totalAPICalls: 10,
    totalCost: 0.5,
  },
};

const filteredUser = {
  id: 'user_plan_filtered',
  email: 'plan-filtered@example.com',
  name: 'Plan Filtered',
  role: 'user',
  planID: 'plan_enterprise_browser',
  planName: 'Enterprise Browser',
  quotaBalance: 9500,
  status: 'active',
  lastLoginAt: now,
  createdAt: now,
  usageStats: {
    totalTokens: 64000,
    totalAPICalls: 128,
    totalCost: 9.75,
  },
};

const mobilePlanID = 'plan_browser_users_mobile_without_breaks_20260624_primary';
const mobileUser = {
  id: 'user_browser_users_mobile_without_breaks',
  email: 'browserusersmobilewithoutbreaks20260624primary@example.com',
  name: 'browserusersmobilewithoutbreaks20260624primarytenant',
  role: 'user',
  planID: mobilePlanID,
  planName: 'browserusersmobilewithoutbreaks20260624primaryplan',
  quotaBalance: 987654.32,
  status: 'active',
  lastLoginAt: now,
  createdAt: now,
  usageStats: {
    totalTokens: 1234567,
    totalAPICalls: 98765,
    totalCost: 4321.09,
  },
};

export async function registerAdminUsersRoutes(page: Page): Promise<void> {
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

    if (method === 'GET' && pathname === '/api/v1/admin/users') {
      if (url.searchParams.has('planId')) {
        await fulfillError(route, 'admin user queries must use planID, not planId');
        return;
      }

      if (url.searchParams.get('planID') === mobilePlanID) {
        await fulfillJSON(route, { users: [mobileUser], total: 1 });
        return;
      }

      if (url.searchParams.get('planID') === 'plan_enterprise_browser') {
        await fulfillJSON(route, { users: [filteredUser], total: 1 });
        return;
      }

      await fulfillJSON(route, { users: [initialUser], total: 1 });
      return;
    }

    await fulfillNotFound(route);
  });
}
