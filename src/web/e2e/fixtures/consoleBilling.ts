import type { Page, Route } from '@playwright/test';

const now = '2026-06-14T12:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_console_billing',
    expiresAt: '2026-06-15T12:00:00Z',
  },
  user: {
    id: 'user_console_billing',
    email: 'billing-operator@example.com',
    name: 'Billing Operator',
    role: 'user',
  },
  workspace: {
    id: 'workspace_console_billing',
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
  requests: 18,
  inputTokens: 4200,
  outputTokens: 1900,
  estimatedCostUsd: 0.0184,
  balanceUsd: 42.5,
  creditLimitUsd: 100,
  currentSpendUsd: 0.0184,
  paymentProviders: [{ name: 'stripe' }, { name: 'wechatpay' }],
  nextInvoice: {
    id: 'draft-2026-06',
    status: 'draft',
    amountUsd: 29,
    dueAt: '2026-06-30T00:00:00Z',
  },
};

const packages = [
  {
    agentLimit: 10,
    createdAt: now,
    durationDays: 30,
    id: 'pkg_starter',
    isActive: true,
    isPublic: true,
    maxTokensPerRequest: 16000,
    modelAccess: ['gpt-4.1-mini'],
    name: 'Starter',
    price: 12,
    quotaAmount: 50,
    sortOrder: 1,
    tokenQuota: 500000,
    updatedAt: now,
  },
  {
    agentLimit: 25,
    createdAt: now,
    durationDays: 30,
    id: 'pkg_pro',
    isActive: true,
    isPublic: true,
    maxTokensPerRequest: 32000,
    modelAccess: ['gpt-4.1', 'gpt-4.1-mini'],
    name: 'Pro',
    price: 29,
    quotaAmount: 150,
    sortOrder: 2,
    tokenQuota: 1500000,
    updatedAt: now,
  },
];

const invoices = [
  {
    id: 'inv_console_paid',
    status: 'paid',
    amountUsd: 12,
    dueAt: '2026-05-31T00:00:00Z',
    hostedInvoiceUrl: 'https://billing.stripe.test/invoices/inv_console_paid',
    invoicePdf: 'https://billing.stripe.test/invoices/inv_console_paid.pdf',
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
      error: { code: 'not_found', message: 'console billing fixture route not found' },
    }),
  });
}

function checkoutPayloadMatchesSubscription(payload: Record<string, unknown>) {
  return payload.kind === 'subscription' && payload.packageId === 'pkg_pro' && payload.provider === 'wechatpay';
}

function checkoutPayloadMatchesTopUp(payload: Record<string, unknown>) {
  return payload.kind === 'topup' && payload.amount === 37.5 && payload.provider === 'wechatpay';
}

export async function registerConsoleBillingRoutes(page: Page): Promise<void> {
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

    if (method === 'GET' && pathname === '/api/v1/console/invoices') {
      await fulfillJSON(route, invoices);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/packages') {
      await fulfillJSON(route, packages);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/billing/checkout') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (checkoutPayloadMatchesSubscription(payload)) {
        await fulfillJSON(route, {
          checkoutSessionId: 'cs_subscription_browser',
          url: 'https://checkout.wechatpay.test/session/cs_subscription_browser',
        }, 201);
        return;
      }

      if (checkoutPayloadMatchesTopUp(payload)) {
        await fulfillJSON(route, {
          checkoutSessionId: 'cs_topup_browser',
          url: 'https://checkout.wechatpay.test/session/cs_topup_browser',
        }, 201);
        return;
      }

      await fulfillError(route, 'checkout payload did not match selected subscription or top-up provider flow');
      return;
    }

    await fulfillNotFound(route);
  });
}
