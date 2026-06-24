import type { Page, Route } from '@playwright/test';

const now = '2026-06-24T10:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_audit',
    expiresAt: '2026-06-25T10:00:00Z',
  },
  user: {
    id: 'user_admin_audit',
    email: 'browser-audit@example.com',
    name: 'Browser Audit Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_audit',
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

const browserOrgID = 'org_audit_browser';
const mobileOrgID = 'org_audit_mobile_without_breaks_20260624_primary';

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
      error: { code: 'not_found', message: 'admin audit log fixture route not found' },
    }),
  });
}

export async function registerAdminAuditLogRoutes(page: Page): Promise<void> {
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

    if (method === 'GET' && pathname === '/api/v1/admin/audit-logs') {
      const organizationID = url.searchParams.get('organizationID');

      if (organizationID !== browserOrgID && organizationID !== mobileOrgID) {
        await fulfillError(route, 'admin audit log queries must include a recognized organizationID');
        return;
      }

      if (url.searchParams.has('organizationId')) {
        await fulfillError(route, 'admin audit log queries must use organizationID, not organizationId');
        return;
      }

      if (organizationID === mobileOrgID) {
        await fulfillJSON(route, {
          entries: [
            {
              id: 'audit_mobile_without_breaks',
              actorID: 'auditlogmobileoperatorwithoutbreaks20260624',
              actorEmail: 'auditlogmobileoperatorwithoutbreaks20260624@example.com',
              action: 'billing.refund.providerreconciliationmobilewithoutbreaks20260624',
              resourceType: 'billingproviderrailwithoutbreaks',
              resourceID: 'refundtopupmobileevidencewithoutbreaks20260624',
              changes: JSON.stringify({
                changeevidencemobilewithoutbreaks20260624: mobileOrgID,
                providerrailreconciliationwithoutbreaks20260624: 'alipayrefundledgeridempotency',
              }),
              ipAddress: '198.51.100.240',
              createdAt: now,
            },
          ],
          total: 1,
        });
        return;
      }

      await fulfillJSON(route, {
        entries: [
          {
            id: 'audit_browser',
            actorID: session.user.id,
            actorEmail: session.user.email,
            action: 'agent.approve',
            resourceType: 'agent',
            resourceID: 'agent_browser_audit',
            changes: JSON.stringify({ organizationID: 'org_audit_browser', status: 'approved' }),
            ipAddress: '203.0.113.24',
            createdAt: now,
          },
        ],
        total: 1,
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
