import type { Page, Route } from '@playwright/test';

const now = '2026-06-16T00:30:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_console_notifications',
    expiresAt: '2026-06-17T00:30:00Z',
  },
  user: {
    id: 'user_console_notifications',
    email: 'notifications-operator@example.com',
    name: 'Notifications Operator',
    role: 'user',
  },
  workspace: {
    id: 'workspace_console_notifications',
  },
};

const initialNotifications = [
  {
    id: 'notif_critical',
    userId: session.user.id,
    type: 'error',
    category: 'system',
    title: 'Database down',
    message: 'Primary database heartbeat failed for the production cluster.',
    isRead: false,
    metadata: { incidentId: 'inc_database_down' },
    createdAt: now,
  },
  {
    id: 'notif_warning',
    userId: session.user.id,
    type: 'warning',
    category: 'billing',
    title: 'Quota near limit',
    message: 'Workspace usage reached 90% of the monthly quota.',
    isRead: false,
    metadata: { quotaPercent: 90 },
    createdAt: '2026-06-15T23:45:00Z',
  },
  {
    id: 'notif_report',
    userId: session.user.id,
    type: 'success',
    category: 'reports',
    title: 'Usage report ready',
    message: 'The June usage report is ready for review.',
    isRead: true,
    readAt: '2026-06-16T00:00:00Z',
    metadata: { reportId: 'usage_2026_06' },
    createdAt: '2026-06-15T22:00:00Z',
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
      error: { code: 'not_found', message: 'console notifications fixture route not found' },
    }),
  });
}

function requestHasBody(route: Route) {
  const body = route.request().postData();
  return body !== null && body.trim().length > 0;
}

export async function registerConsoleNotificationsRoutes(page: Page): Promise<void> {
  let notifications = initialNotifications.map((notification) => ({ ...notification }));

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/notifications') {
      if (
        url.searchParams.get('limit') !== '50' ||
        url.searchParams.has('offset') ||
        url.searchParams.has('unread')
      ) {
        await fulfillError(route, 'notification list query did not preserve the page limit contract');
        return;
      }
      await fulfillJSON(route, notifications);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/notifications/unread-count') {
      await fulfillJSON(route, { count: notifications.filter((notification) => !notification.isRead).length });
      return;
    }

    if (method === 'PATCH' && pathname === '/api/v1/app/notifications/notif_critical') {
      if (requestHasBody(route)) {
        await fulfillError(route, 'mark-read request should not send a JSON body');
        return;
      }
      notifications = notifications.map((notification) =>
        notification.id === 'notif_critical'
          ? { ...notification, isRead: true, readAt: '2026-06-16T00:31:00Z' }
          : notification
      );
      await fulfillJSON(route, { status: 'ok' });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/app/notifications/mark-all-read') {
      if (requestHasBody(route)) {
        await fulfillError(route, 'mark-all-read request should not send a JSON body');
        return;
      }
      notifications = notifications.map((notification) => ({
        ...notification,
        isRead: true,
        readAt: notification.readAt ?? '2026-06-16T00:32:00Z',
      }));
      await fulfillJSON(route, { status: 'ok' });
      return;
    }

    if (method === 'DELETE' && pathname === '/api/v1/app/notifications/notif_warning') {
      if (requestHasBody(route)) {
        await fulfillError(route, 'delete request should not send a JSON body');
        return;
      }
      notifications = notifications.filter((notification) => notification.id !== 'notif_warning');
      await fulfillJSON(route, { status: 'deleted' });
      return;
    }

    await fulfillNotFound(route);
  });
}
