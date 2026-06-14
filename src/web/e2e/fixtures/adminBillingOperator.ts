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
    id: 'session_admin_billing_operator',
    expiresAt: '2026-06-16T14:00:00Z',
  },
  user: {
    id: 'user_billing_operator',
    email: 'billing-admin@example.com',
    name: 'Billing Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_billing_operator',
  },
};

const billingSummary = {
  billingSessions: { count: 1, settledAmount: 8.5 },
  paymentIntents: { count: 2, totalAmount: 65, refundedAmount: 10 },
  webhookEvents: { count: 3, failedCount: 0 },
  topups: { count: 1, refundedAmount: 10 },
  refunds: { count: 1, refundedAmount: 10 },
  settlements: { count: 2, grossAmount: 90, publisherNetAmount: 72 },
  payouts: { count: 2, totalAmount: 72 },
};

const initialSession = {
  id: 'session_admin_billing_initial',
  organizationId: 'org_billing_operator',
  userId: 'user_billing_operator',
  model: 'gpt-4.1-mini',
  apiType: 'chat',
  settledAmount: 8.5,
  status: 'settled',
  createdAt: now,
};

const payoutToMarkPaid = {
  id: 'payout_browser_paid',
  organizationId: 'org_billing_operator',
  publisherOrganizationId: 'publisher_org_browser_paid',
  provider: 'stripe_connect',
  providerPayoutId: 'po_browser_paid_pending',
  amount: 40,
  status: 'payout_pending',
  kind: 'marketplace_install',
  createdAt: now,
};

const payoutToMarkFailed = {
  id: 'payout_browser_failed',
  organizationId: 'org_billing_operator',
  publisherOrganizationId: 'publisher_org_browser_failed',
  provider: 'stripe_connect',
  providerPayoutId: 'po_browser_failed_pending',
  amount: 32,
  status: 'payout_pending',
  kind: 'marketplace_install',
  createdAt: now,
};

const topupToRefund = {
  id: 'topup_browser_refund',
  organizationId: 'org_billing_operator',
  userId: 'user_billing_operator',
  provider: 'stripe',
  providerChargeId: 'ch_browser_refund_1',
  providerPaymentIntentId: 'pi_browser_refund_1',
  amount: 25,
  money: 25,
  refundedAmount: 10,
  status: 'paid',
  kind: 'topup',
  currency: 'usd',
  createdAt: now,
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
      error: { code: 'not_found', message: 'admin billing operator fixture route not found' },
    }),
  });
}

function queryHas(url: URL, expected: Record<string, string>) {
  return Object.entries(expected).every(([key, value]) => url.searchParams.get(key) === value);
}

function payoutQueryMatches(url: URL) {
  return queryHas(url, {
    organizationID: 'org_billing_operator',
    status: 'payout_pending',
    kind: 'marketplace_install',
    provider: 'stripe_connect',
    limit: '50',
  });
}

function topupQueryMatches(url: URL) {
  return queryHas(url, {
    organizationID: 'org_billing_operator',
    userID: 'user_billing_operator',
    status: 'paid',
    kind: 'topup',
    provider: 'stripe',
    limit: '50',
  });
}

function paidPayoutPayloadMatches(payload: Record<string, unknown>) {
  return payload.providerPayoutID === 'po_browser_paid_confirmed';
}

function failedPayoutPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.providerPayoutID === 'po_browser_failed_confirmed' &&
    payload.reason === 'bank account closed by publisher'
  );
}

function refundPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.provider === 'stripe' &&
    payload.providerRefundID === 're_browser_refund_1' &&
    payload.providerChargeID === 'ch_browser_refund_1' &&
    payload.providerPaymentIntentID === 'pi_browser_refund_1' &&
    payload.amount === 12.5 &&
    payload.currency === 'usd' &&
    payload.reason === 'duplicate provider capture'
  );
}

export async function registerAdminBillingOperatorRoutes(page: Page): Promise<void> {
  let paidPayoutConfirmed = false;
  let failedPayoutConfirmed = false;
  let topupRefundConfirmed = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/billing/summary') {
      await fulfillJSON(route, billingSummary);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/billing/sessions') {
      await fulfillJSON(route, { sessions: [initialSession], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/billing/payouts') {
      if (!payoutQueryMatches(url)) {
        await fulfillError(route, 'payout query did not match Billing operator filters');
        return;
      }

      await fulfillJSON(route, {
        payouts: [
          {
            ...payoutToMarkPaid,
            providerPayoutId: paidPayoutConfirmed ? 'po_browser_paid_confirmed' : payoutToMarkPaid.providerPayoutId,
            status: paidPayoutConfirmed ? 'paid_out' : 'payout_pending',
          },
          {
            ...payoutToMarkFailed,
            providerPayoutId: failedPayoutConfirmed ? 'po_browser_failed_confirmed' : payoutToMarkFailed.providerPayoutId,
            status: failedPayoutConfirmed ? 'failed' : 'payout_pending',
          },
        ],
        total: 2,
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/billing/payouts/payout_browser_paid/paid') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!paidPayoutPayloadMatches(payload)) {
        await fulfillError(route, 'paid payout payload did not include the confirmed provider payout ID');
        return;
      }
      paidPayoutConfirmed = true;
      await fulfillJSON(route, { ...payoutToMarkPaid, providerPayoutId: 'po_browser_paid_confirmed', status: 'paid_out' });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/billing/payouts/payout_browser_failed/failed') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!failedPayoutPayloadMatches(payload)) {
        await fulfillError(route, 'failed payout payload did not include provider evidence and reason');
        return;
      }
      failedPayoutConfirmed = true;
      await fulfillJSON(route, { ...payoutToMarkFailed, providerPayoutId: 'po_browser_failed_confirmed', status: 'failed' });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/billing/topups') {
      if (!topupQueryMatches(url)) {
        await fulfillError(route, 'top-up query did not match Billing operator filters');
        return;
      }

      await fulfillJSON(route, {
        topups: [
          {
            ...topupToRefund,
            refundedAmount: topupRefundConfirmed ? 22.5 : topupToRefund.refundedAmount,
          },
        ],
        total: 1,
      });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/billing/topups/topup_browser_refund/refund') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!refundPayloadMatches(payload)) {
        await fulfillError(route, 'top-up refund payload did not include provider refund evidence');
        return;
      }
      topupRefundConfirmed = true;
      await fulfillJSON(route, {
        id: 'refund_browser_topup',
        providerRefundId: 're_browser_refund_1',
        topupOrderId: 'topup_browser_refund',
        amount: 12.5,
        status: 'processed',
        reason: 'duplicate provider capture',
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
