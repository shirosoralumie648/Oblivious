import type { Page, Route } from '@playwright/test';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'quality',
    networkEnabledHint: true,
    onboardingCompleted: true,
    defaultAgentModel: 'gpt-4.1',
    sidebarCollapsed: true,
    notifications: {
      desktop: true,
      email: false,
    },
  },
  session: {
    id: 'session_settings_browser',
    expiresAt: '2026-06-25T12:00:00Z',
  },
  user: {
    id: 'user_settings_browser',
    email: 'settings-operator@example.com',
    name: 'Settings Operator',
    role: 'user',
  },
  workspace: {
    id: 'workspace_settings_browser',
  },
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
      error: { code: 'not_found', message: 'settings fixture route not found' },
    }),
  });
}

function preferencesPayloadMatches(payload: Record<string, unknown>) {
  const notifications = payload.notifications as Record<string, unknown> | undefined;

  return (
    payload.defaultMode === 'solo' &&
    payload.modelStrategy === 'cost' &&
    payload.networkEnabledHint === false &&
    payload.onboardingCompleted === true &&
    payload.defaultAgentModel === 'gpt-4.1' &&
    payload.sidebarCollapsed === true &&
    notifications?.desktop === true &&
    notifications?.email === false
  );
}

export async function registerSettingsRoutes(page: Page): Promise<void> {
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/mcp-local-servers') {
      await fulfillJSON(route, [
        {
          description: 'Tenant-safe local MCP tools exposed by this server',
          id: 'local_builtin_safe',
          name: 'Oblivious Safe Builtins',
          toolCount: 2,
        },
      ]);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/mcp-servers') {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'PUT' && pathname === '/api/v1/app/me/preferences') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!preferencesPayloadMatches(payload)) {
        await fulfillError(route, 'Settings save dropped extended preference fields');
        return;
      }

      await fulfillJSON(route, {
        ...session.preferences,
        defaultMode: 'solo',
        modelStrategy: 'cost',
        networkEnabledHint: false,
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
