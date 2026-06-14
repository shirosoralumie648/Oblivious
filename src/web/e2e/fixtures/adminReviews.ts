import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T15:00:00Z';

const adminSession = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_admin_reviews',
    expiresAt: '2026-06-16T15:00:00Z',
  },
  user: {
    id: 'user_admin_reviews',
    email: 'reviews-admin@example.com',
    name: 'Reviews Admin',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_admin_reviews',
  },
};

const reviewAgent = {
  id: 'agent_review_browser',
  name: 'Browser Review Agent',
  description: 'Automates release-review evidence collection for publishers.',
  ownerID: 'publisher_review_browser',
  ownerName: 'Review Publisher',
  status: 'pending_review',
  visibility: 'public',
  pricingType: 'one_time',
  pricingAmount: 29,
  categoryID: 'cat_productivity',
  categoryName: 'Productivity',
  tags: ['review', 'release'],
  ratingAvg: 4.7,
  rating: 4.7,
  ratingCount: 11,
  installCount: 37,
  createdAt: now,
  updatedAt: now,
  reviewSLA: {
    submittedAt: '2026-06-15T12:00:00Z',
    automatedReviewDeadlineAt: '2026-06-15T12:05:00Z',
    automatedReviewSlaMinutes: 5,
    automatedReviewSlaStatus: 'overdue',
    manualDeadlineAt: '2026-06-18T12:00:00Z',
    manualSlaHours: 72,
    manualSlaStatus: 'due_soon',
    minutesUntilDeadline: 180,
    vipPublisher: true,
    publisherTier: 'vip',
    publisherTierSource: 'publisher_revenue_tier',
  },
};

const abuseReport = {
  id: 'report_review_browser',
  reporterOrganizationId: 'org_reporter_browser',
  reporterUserId: 'user_reporter_browser',
  agentId: reviewAgent.id,
  reason: 'policy_violation',
  details: 'The listing contains prompt-injection instructions.',
  status: 'open',
  createdAt: now,
  updatedAt: now,
};

const governanceReasons = {
  takedown: 'policy violation from abuse evidence',
  reinstate: 'appeal accepted with remediation evidence',
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
      error: { code: 'not_found', message: 'admin reviews fixture route not found' },
    }),
  });
}

function reviewQueryMatches(url: URL, expectedStatus?: string) {
  return url.searchParams.get('limit') === '100' && url.searchParams.get('status') === (expectedStatus ?? null);
}

function abuseQueryMatches(url: URL, expectedStatus?: string) {
  return url.searchParams.get('limit') === '50' && url.searchParams.get('status') === (expectedStatus ?? null);
}

function reasonPayloadMatches(payload: Record<string, unknown>, reason: string) {
  return payload.reason === reason;
}

function resolutionPayloadMatches(payload: Record<string, unknown>, resolution: string) {
  return payload.resolution === resolution;
}

function governancePayloadMatches(payload: Record<string, unknown>, reason: string) {
  return payload.reason === reason;
}

export async function registerAdminReviewsRoutes(page: Page): Promise<void> {
  let agentStatus = reviewAgent.status;
  let abuseStatus = abuseReport.status;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, adminSession);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/reviews') {
      if (url.searchParams.has('status')) {
        if (!reviewQueryMatches(url, 'pending_review')) {
          await fulfillError(route, 'review query did not match the pending review filter');
          return;
        }

        const reviews = agentStatus === 'pending_review' ? [{ ...reviewAgent, status: agentStatus }] : [];
        await fulfillJSON(route, { reviews, total: reviews.length });
        return;
      }

      if (!reviewQueryMatches(url)) {
        await fulfillError(route, 'review query did not match the all-status refresh filter');
        return;
      }

      await fulfillJSON(route, { reviews: [{ ...reviewAgent, status: agentStatus }], total: 1 });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/admin/reviews/sla/enforce') {
      if (url.searchParams.get('limit') !== '100') {
        await fulfillError(route, 'review SLA enforcement query did not include limit=100');
        return;
      }
      await fulfillJSON(route, { scanned: 1, alerted: 1 });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/reviews/${reviewAgent.id}/needs-changes`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!reasonPayloadMatches(payload, 'Add screenshots and clarify pricing.')) {
        await fulfillError(route, 'needs-changes payload did not include the operator reason');
        return;
      }
      agentStatus = 'needs_changes';
      await fulfillJSON(route, { status: agentStatus });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/reviews/${reviewAgent.id}/approve`) {
      agentStatus = 'approved';
      await fulfillJSON(route, { status: agentStatus });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/reviews/${reviewAgent.id}/reject`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!reasonPayloadMatches(payload, 'Missing security evidence.')) {
        await fulfillError(route, 'reject payload did not include the operator reason');
        return;
      }
      agentStatus = 'rejected';
      await fulfillJSON(route, { status: agentStatus });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/admin/marketplace/abuse-reports') {
      if (url.searchParams.has('status')) {
        if (!abuseQueryMatches(url, 'open')) {
          await fulfillError(route, 'abuse report query did not match the open filter');
          return;
        }

        const reports = abuseStatus === 'open' ? [{ ...abuseReport, status: abuseStatus }] : [];
        await fulfillJSON(route, { reports, total: reports.length });
        return;
      }

      if (!abuseQueryMatches(url)) {
        await fulfillError(route, 'abuse report query did not match the all-status refresh filter');
        return;
      }

      await fulfillJSON(route, { reports: [{ ...abuseReport, status: abuseStatus, resolution: 'publisher fixed listing' }], total: 1 });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/marketplace/abuse-reports/${abuseReport.id}/resolve`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!resolutionPayloadMatches(payload, 'publisher fixed listing')) {
        await fulfillError(route, 'abuse report resolution payload did not include moderation evidence');
        return;
      }
      abuseStatus = 'resolved';
      await fulfillJSON(route, { status: abuseStatus });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/marketplace/abuse-reports/${abuseReport.id}/dismiss`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!resolutionPayloadMatches(payload, 'not reproducible after review')) {
        await fulfillError(route, 'abuse report dismissal payload did not include moderation evidence');
        return;
      }
      abuseStatus = 'dismissed';
      await fulfillJSON(route, { status: abuseStatus });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/marketplace/agents/${reviewAgent.id}/takedown`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!governancePayloadMatches(payload, governanceReasons.takedown)) {
        await fulfillError(route, 'takedown payload did not include governance evidence');
        return;
      }
      await fulfillJSON(route, { status: 'takedown' });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/admin/marketplace/agents/${reviewAgent.id}/reinstate`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!governancePayloadMatches(payload, governanceReasons.reinstate)) {
        await fulfillError(route, 'reinstate payload did not include governance evidence');
        return;
      }
      await fulfillJSON(route, { status: 'approved' });
      return;
    }

    await fulfillNotFound(route);
  });
}
