import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createAdminApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    get: overrides.get
      ? ((path, init) => init === undefined ? overrides.get!(path) : overrides.get!(path, init)) as HttpClient['get']
      : vi.fn(),
    post: overrides.post
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.post!(path) : overrides.post!(path, body)
          : overrides.post!(path, body, init)) as HttpClient['post']
      : vi.fn(),
    put: overrides.put
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.put!(path) : overrides.put!(path, body)
          : overrides.put!(path, body, init)) as HttpClient['put']
      : vi.fn(),
    delete: overrides.delete
      ? ((path, init) => init === undefined ? overrides.delete!(path) : overrides.delete!(path, init)) as HttpClient['delete']
      : vi.fn(),
    request: overrides.request
      ? ((path, init) => init === undefined ? overrides.request!(path) : overrides.request!(path, init)) as HttpClient['request']
      : vi.fn(),
  };
  return client;
}

describe('createAdminApi', () => {
  it('normalizes list responses from backend collection keys', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce({ channels: [{ id: 'ch_1', name: 'OpenAI' }], total: 1 })
      .mockResolvedValueOnce({ routes: [{ id: 'rt_1', model: 'gpt-4o' }], total: 1 })
      .mockResolvedValueOnce({ entries: [{ id: 'aud_1', action: 'channel.create' }], total: 1 })
      .mockResolvedValueOnce({ entries: [], total: 0 });
    const client = createClient({ get });

    const api = createAdminApi(client);

    await expect(api.listChannels({ provider: 'openai', limit: 10 })).resolves.toEqual({
      data: [{ id: 'ch_1', name: 'OpenAI' }],
      total: 1,
    });
    await expect(api.listRoutes()).resolves.toEqual([{ id: 'rt_1', model: 'gpt-4o' }]);
    await expect(api.listAuditLogs({ action: 'channel.create' })).resolves.toEqual({
      data: [{ id: 'aud_1', action: 'channel.create' }],
      total: 1,
    });
    await expect(api.listAuditLogs({ organizationId: 'org_audit', action: 'channel.create' })).resolves.toEqual({
      data: [],
      total: 0,
    });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/admin/channels?provider=openai&limit=10');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/admin/routes');
    expect(get).toHaveBeenNthCalledWith(3, '/api/v1/admin/audit-logs?action=channel.create');
    expect(get).toHaveBeenNthCalledWith(4, '/api/v1/admin/audit-logs?organizationID=org_audit&action=channel.create');
  });

  it('preserves marketplace review SLA metadata from the admin review list', async () => {
    const get = vi.fn().mockResolvedValue({
      reviews: [
        {
          id: 'agent_sla',
          name: 'Review me',
          description: 'Pending marketplace submission',
          ownerID: 'owner_1',
          ownerName: 'Publisher',
          status: 'pending_review',
          visibility: 'public',
          tags: [],
          ratingCount: 0,
          installCount: 0,
          createdAt: '2026-06-02T13:00:00Z',
          updatedAt: '2026-06-02T13:00:00Z',
          reviewSLA: {
            submittedAt: '2026-06-02T13:00:00Z',
            manualDeadlineAt: '2026-06-05T13:00:00Z',
            manualSlaHours: 72,
            manualSlaStatus: 'due_soon',
            minutesUntilDeadline: 60,
            automatedReviewDeadlineAt: '2026-06-02T13:05:00Z',
            automatedReviewSlaMinutes: 5,
            automatedReviewSlaStatus: 'overdue',
            vipPublisher: false,
            publisherTier: 'standard',
            publisherTierSource: 'default',
          },
        },
      ],
      total: 1,
    });
    const api = createAdminApi(createClient({ get }));

    await expect(api.listReviews({ status: 'pending_review', limit: 100 })).resolves.toEqual({
      data: [
        expect.objectContaining({
          id: 'agent_sla',
          reviewSLA: expect.objectContaining({
            manualDeadlineAt: '2026-06-05T13:00:00Z',
            manualSlaStatus: 'due_soon',
            automatedReviewSlaStatus: 'overdue',
            publisherTier: 'standard',
          }),
        }),
      ],
      total: 1,
    });
    expect(get).toHaveBeenCalledWith('/api/v1/admin/reviews?status=pending_review&limit=100');
  });

  it('triggers marketplace review SLA enforcement through the admin endpoint', async () => {
    const post = vi.fn().mockResolvedValue({ scanned: 3, alerted: 2 });
    const api = createAdminApi(createClient({ post }));

    await expect(api.enforceReviewSLA({ limit: 50 })).resolves.toEqual({ scanned: 3, alerted: 2 });

    expect(post).toHaveBeenCalledWith('/api/v1/admin/reviews/sla/enforce?limit=50');
  });

  it('requests publisher changes through the admin marketplace review endpoint', async () => {
    const post = vi.fn().mockResolvedValue({ status: 'needs_changes' });
    const api = createAdminApi(createClient({ post }));

    await api.requestAgentChanges('agent_1', 'Add screenshots and clarify pricing.');

    expect(post).toHaveBeenCalledWith('/api/v1/admin/reviews/agent_1/needs-changes', {
      reason: 'Add screenshots and clarify pricing.',
    });
  });

  it('supports admin marketplace abuse report list and resolution routes', async () => {
    const get = vi.fn().mockResolvedValue({
      reports: [
        {
          id: 'report_1',
          reporterOrganizationId: 'org_1',
          reporterUserId: 'user_1',
          agentId: 'agent_1',
          reason: 'malware',
          details: 'attempted credential exfiltration',
          status: 'open',
          createdAt: '2026-01-07T00:00:00Z',
          updatedAt: '2026-01-07T00:00:00Z',
        },
      ],
      total: 1,
    });
    const post = vi
      .fn()
      .mockResolvedValueOnce({ status: 'resolved' })
      .mockResolvedValueOnce({ status: 'dismissed' });
    const api = createAdminApi(createClient({ get, post }));

    await expect(api.listMarketplaceAbuseReports({ status: 'open', limit: 20 })).resolves.toEqual({
      data: [expect.objectContaining({ id: 'report_1', status: 'open' })],
      total: 1,
    });
    await expect(api.resolveMarketplaceAbuseReport('report_1', 'agent removed')).resolves.toEqual({ status: 'resolved' });
    await expect(api.dismissMarketplaceAbuseReport('report_2', 'not reproducible')).resolves.toEqual({ status: 'dismissed' });

    expect(get).toHaveBeenCalledWith('/api/v1/admin/marketplace/abuse-reports?status=open&limit=20');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/marketplace/abuse-reports/report_1/resolve', {
      resolution: 'agent removed',
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/marketplace/abuse-reports/report_2/dismiss', {
      resolution: 'not reproducible',
    });
  });

  it('supports admin marketplace takedown and reinstate routes', async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({ status: 'takedown' })
      .mockResolvedValueOnce({ status: 'approved' });
    const api = createAdminApi(createClient({ post }));

    await expect(api.takedownMarketplaceAgent('agent_1', 'policy violation')).resolves.toEqual({ status: 'takedown' });
    await expect(api.reinstateMarketplaceAgent('agent_1', 'appeal accepted')).resolves.toEqual({ status: 'approved' });

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/marketplace/agents/agent_1/takedown', {
      reason: 'policy violation',
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/marketplace/agents/agent_1/reinstate', {
      reason: 'appeal accepted',
    });
  });

  it('uses the backend action payload for channel batch updates', async () => {
    const post = vi.fn().mockResolvedValue(undefined);
    const api = createAdminApi(createClient({ post }));

    await api.batchUpdateChannels(['ch_1', 'ch_2'], 'disable');

    expect(post).toHaveBeenCalledWith('/api/v1/admin/channels/batch', {
      ids: ['ch_1', 'ch_2'],
      action: 'disable',
    });
  });

  it('syncs channel models through the admin channel endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      channel: { id: 'ch_1', models: ['gpt-4o'] },
      testResult: { success: true, models: ['gpt-4o'] },
    });
    const api = createAdminApi(createClient({ post }));

    await expect(api.syncChannelModels('ch_1')).resolves.toEqual({
      channel: { id: 'ch_1', models: ['gpt-4o'] },
      testResult: { success: true, models: ['gpt-4o'] },
    });
    expect(post).toHaveBeenCalledWith('/api/v1/admin/channels/ch_1/sync-models');
  });

  it('refreshes channel balance through the admin channel endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'ch_1',
      balance: { amount: 12.5, currency: 'USD', source: 'provider_balance' },
      testResult: { success: true, latency: 88 },
    });
    const api = createAdminApi(createClient({ post }));

    await expect(api.refreshChannelBalance('ch_1')).resolves.toEqual({
      id: 'ch_1',
      balance: { amount: 12.5, currency: 'USD', source: 'provider_balance' },
      testResult: { success: true, latency: 88 },
    });
    expect(post).toHaveBeenCalledWith('/api/v1/admin/channels/ch_1/refresh-balance');
  });

  it('detects and applies channel model updates through admin endpoints', async () => {
    const post = vi
      .fn()
      .mockResolvedValueOnce({
        id: 'ch_1',
        added: ['gpt-4.1'],
        removed: ['legacy-model'],
        unchanged: ['gpt-4o'],
      })
      .mockResolvedValueOnce({
        mode: 'replace',
        appliedModels: ['gpt-4o', 'gpt-4.1'],
      });
    const api = createAdminApi(createClient({ post }));

    await expect(api.detectChannelModelUpdates('ch_1')).resolves.toEqual({
      id: 'ch_1',
      added: ['gpt-4.1'],
      removed: ['legacy-model'],
      unchanged: ['gpt-4o'],
    });
    await expect(api.applyChannelModelUpdates('ch_1', { mode: 'replace' })).resolves.toEqual({
      mode: 'replace',
      appliedModels: ['gpt-4o', 'gpt-4.1'],
    });

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/channels/ch_1/model-updates/detect');
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/channels/ch_1/model-updates/apply', { mode: 'replace' });
  });

  it('lists the relay provider catalog from the backend', async () => {
    const get = vi.fn().mockResolvedValue({
      providers: [
        {
          id: 'openai',
          displayName: 'OpenAI',
          kind: 'openai_compatible',
          status: 'supported',
          defaultBaseURL: 'https://api.openai.com',
        },
      ],
    });
    const api = createAdminApi(createClient({ get }));

    await expect(api.listChannelProviders()).resolves.toEqual([
      {
        id: 'openai',
        displayName: 'OpenAI',
        kind: 'openai_compatible',
        status: 'supported',
        defaultBaseURL: 'https://api.openai.com',
      },
    ]);
    expect(get).toHaveBeenCalledWith('/api/v1/admin/channel-providers');
  });

  it('lists channel runtime stats from the backend', async () => {
    const get = vi.fn().mockResolvedValue({
      stats: [
        {
          channelID: 'ch_1',
          rpmCurrent: 7,
          tpmCurrent: 321,
          totalRequests: 12,
          successCount: 10,
          failureCount: 2,
          avgLatencyMs: 300,
          rateLimitedUntil: '2026-06-04T12:30:00Z',
          affinityConversationCount: 4,
        },
      ],
    });
    const api = createAdminApi(createClient({ get }));

    await expect(api.listChannelStats()).resolves.toEqual([
      {
        channelID: 'ch_1',
        rpmCurrent: 7,
        tpmCurrent: 321,
        totalRequests: 12,
        successCount: 10,
        failureCount: 2,
        avgLatencyMs: 300,
        rateLimitedUntil: '2026-06-04T12:30:00Z',
        affinityConversationCount: 4,
      },
    ]);
    expect(get).toHaveBeenCalledWith('/api/v1/admin/channels/stats');
  });

  it('exposes admin billing summary and list surfaces', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce({ billingSessions: { count: 1, settledAmount: 4.5 } })
      .mockResolvedValueOnce({ sessions: [{ id: 'bs_1', status: 'settled' }], total: 1 })
      .mockResolvedValueOnce({ paymentIntents: [{ id: 'pi_1', kind: 'subscription' }], total: 1 })
      .mockResolvedValueOnce({
        topups: [{ id: 'topup_1', provider: 'stripe', providerPaymentIntentId: 'pi_provider_1', currency: 'usd' }],
        total: 1,
      });
    const post = vi
      .fn()
      .mockResolvedValueOnce({ id: 'payout_1', status: 'paid_out', providerPayoutId: 'provider-paid-1' })
      .mockResolvedValueOnce({ id: 'payout_1', status: 'failed', providerPayoutId: 'provider-failed-1' })
      .mockResolvedValueOnce({ payouts: [{ id: 'payout_due_1', status: 'payout_pending', providerPayoutId: 'provider-due-1' }], total: 1 })
      .mockResolvedValueOnce({ id: 'refund_1', status: 'succeeded', providerRefundId: 're_1' });
    const api = createAdminApi(createClient({ get, post }));

    await expect(api.getBillingSummary({ organizationID: 'org_1' })).resolves.toEqual({
      billingSessions: { count: 1, settledAmount: 4.5 },
    });
    await expect(api.listBillingSurface('sessions', { status: 'settled', limit: 10 })).resolves.toEqual({
      data: [{ id: 'bs_1', status: 'settled' }],
      total: 1,
    });
    await expect(api.listBillingSurface('paymentIntents', { kind: 'subscription' })).resolves.toEqual({
      data: [{ id: 'pi_1', kind: 'subscription' }],
      total: 1,
    });
    await expect(api.listBillingSurface('topups', { provider: 'stripe' })).resolves.toEqual({
      data: [{ id: 'topup_1', provider: 'stripe', providerPaymentIntentId: 'pi_provider_1', currency: 'usd' }],
      total: 1,
    });
    await expect(api.markMarketplacePayoutPaid('payout_1', 'provider-paid-1')).resolves.toEqual({
      id: 'payout_1',
      status: 'paid_out',
      providerPayoutId: 'provider-paid-1',
    });
    await expect(api.markMarketplacePayoutFailed('payout_1', 'provider-failed-1', 'bank account closed')).resolves.toEqual({
      id: 'payout_1',
      status: 'failed',
      providerPayoutId: 'provider-failed-1',
    });
    await expect(api.createDueMarketplacePayouts()).resolves.toEqual({
      data: [{ id: 'payout_due_1', status: 'payout_pending', providerPayoutId: 'provider-due-1' }],
      total: 1,
    });
    await expect(
      api.refundTopup('topup_1', {
        provider: 'stripe',
        providerRefundId: 're_1',
        providerChargeId: 'ch_1',
        providerPaymentIntentId: 'pi_provider_1',
        amount: 10,
        currency: 'usd',
        reason: 'duplicate charge',
      })
    ).resolves.toEqual({ id: 'refund_1', status: 'succeeded', providerRefundId: 're_1' });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/admin/billing/summary?organizationID=org_1');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/admin/billing/sessions?status=settled&limit=10');
    expect(get).toHaveBeenNthCalledWith(3, '/api/v1/admin/billing/payment-intents?kind=subscription');
    expect(get).toHaveBeenNthCalledWith(4, '/api/v1/admin/billing/topups?provider=stripe');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/billing/payouts/payout_1/paid', { providerPayoutID: 'provider-paid-1' });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/billing/payouts/payout_1/failed', {
      providerPayoutID: 'provider-failed-1',
      reason: 'bank account closed',
    });
    expect(post).toHaveBeenNthCalledWith(3, '/api/v1/admin/billing/payouts/create-due');
    expect(post).toHaveBeenNthCalledWith(4, '/api/v1/admin/billing/topups/topup_1/refund', {
      provider: 'stripe',
      providerRefundID: 're_1',
      providerChargeID: 'ch_1',
      providerPaymentIntentID: 'pi_provider_1',
      amount: 10,
      currency: 'usd',
      reason: 'duplicate charge',
    });
  });

  it('serializes usage analytics filters with backend query keys', async () => {
    const get = vi.fn().mockResolvedValue({
      byModel: [{ dimension: 'model', key: 'gpt-4o', requestCount: 3, totalTokens: 150, totalCost: 0.0012 }],
      byFeature: [],
      byUser: [],
      byTime: [],
      byChannel: [{ dimension: 'channel', key: 'ch_1', requestCount: 2, totalTokens: 90, totalCost: 0.0006 }],
      byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 2, totalTokens: 90, totalCost: 0.0006 }],
    });
    const api = createAdminApi(createClient({ get }));

    await expect(
      api.getUsageAnalytics({
        organizationId: 'org_1',
        userId: 'user_1',
        from: '2026-06-01T00:00:00Z',
        to: '2026-06-04T00:00:00Z',
        limit: 5,
      })
    ).resolves.toEqual({
      byModel: [{ dimension: 'model', key: 'gpt-4o', requestCount: 3, totalTokens: 150, totalCost: 0.0012 }],
      byFeature: [],
      byUser: [],
      byTime: [],
      byChannel: [{ dimension: 'channel', key: 'ch_1', requestCount: 2, totalTokens: 90, totalCost: 0.0006 }],
      byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 2, totalTokens: 90, totalCost: 0.0006 }],
    });

    expect(get).toHaveBeenCalledWith(
      '/api/v1/admin/usage-analytics?organizationID=org_1&userID=user_1&from=2026-06-01T00%3A00%3A00Z&to=2026-06-04T00%3A00%3A00Z&limit=5'
    );
  });

  it('serializes usage analytics gateway filters with backend query keys', async () => {
    const get = vi.fn().mockResolvedValue({
      byModel: [],
      byFeature: [],
      byUser: [],
      byTime: [],
      byChannel: [],
      byProvider: [],
    });
    const api = createAdminApi(createClient({ get }));

    await api.getUsageAnalytics({
      apiType: 'chat',
      model: 'gpt-4o',
      channelID: 'ch_1',
      provider: 'openai',
      status: 'success',
      granularity: 'week',
      limit: 8,
    });

    expect(get).toHaveBeenCalledWith(
      '/api/v1/admin/usage-analytics?apiType=chat&model=gpt-4o&channelID=ch_1&provider=openai&status=success&granularity=week&limit=8'
    );
  });

  it('preserves usage analytics cross dimension buckets from the backend', async () => {
    const get = vi.fn().mockResolvedValue({
      byModel: [],
      byFeature: [],
      byUser: [],
      byTime: [],
      byChannel: [],
      byProvider: [],
      crossDimensions: [
        {
          dimension: 'model_time',
          key: 'gpt-4o / 2026-06-04T00:00:00Z',
          primary: 'gpt-4o',
          secondary: '2026-06-04T00:00:00Z',
          requestCount: 9,
          totalTokens: 900,
          totalCost: 0.009,
        },
      ],
    });
    const api = createAdminApi(createClient({ get }));

    await expect(api.getUsageAnalytics({ limit: 8 })).resolves.toEqual({
      byModel: [],
      byFeature: [],
      byUser: [],
      byTime: [],
      byChannel: [],
      byProvider: [],
      crossDimensions: [
        {
          dimension: 'model_time',
          key: 'gpt-4o / 2026-06-04T00:00:00Z',
          primary: 'gpt-4o',
          secondary: '2026-06-04T00:00:00Z',
          requestCount: 9,
          totalTokens: 900,
          totalCost: 0.009,
        },
      ],
    });
    expect(get).toHaveBeenCalledWith('/api/v1/admin/usage-analytics?limit=8');
  });

  it('serializes usage log classification filters with backend query keys', async () => {
    const get = vi.fn().mockResolvedValue({
      usageLogs: [
        {
          id: 'usage_1',
          featureType: 'workspace_chat',
          quotaMode: 'relay_billing',
        },
      ],
      total: 1,
    });
    const api = createAdminApi(createClient({ get }));

    await expect(
      api.listUsageLogs({
        organizationId: 'org_1',
        featureType: 'workspace_chat',
        quotaMode: 'relay_billing',
        limit: 25,
      })
    ).resolves.toEqual({
      data: [
        {
          id: 'usage_1',
          featureType: 'workspace_chat',
          quotaMode: 'relay_billing',
        },
      ],
      total: 1,
    });

    expect(get).toHaveBeenCalledWith(
      '/api/v1/admin/usage-logs?organizationID=org_1&featureType=workspace_chat&quotaMode=relay_billing&limit=25'
    );
  });

  it('serializes relay usage price reconciliation filters with backend query keys', async () => {
    const get = vi.fn().mockResolvedValue({
      checkedRecords: 2,
      matchedRecords: 1,
      missingSnapshotRecords: 1,
      mismatchedRecords: 0,
      ledgerTotalCost: 0.42,
      snapshotTotalCost: 0.42,
      deltaCost: 0,
      issues: [
        {
          id: 'usage_missing_snapshot',
          userId: 'user_1',
          model: 'gpt-4o',
          cost: 0.42,
          snapshotTotalCost: 0,
          deltaCost: 0.42,
          issue: 'missing_snapshot',
          createdAt: '2026-07-01T10:00:00Z',
        },
      ],
      limit: 10,
      offset: 0,
    });
    const api = createAdminApi(createClient({ get }));

    await expect(
      api.getRelayUsagePriceReconciliation({
        organizationId: 'org_1',
        userId: 'user_1',
        apiTokenId: 'tok_1',
        requestId: 'req_1',
        apiType: 'chat',
        featureType: 'workspace_chat',
        quotaMode: 'relay_billing',
        model: 'gpt-4o',
        channelId: 'ch_1',
        provider: 'openai',
        status: 'success',
        from: '2026-07-01T00:00:00Z',
        to: '2026-07-02T00:00:00Z',
        limit: 10,
      })
    ).resolves.toMatchObject({
      checkedRecords: 2,
      missingSnapshotRecords: 1,
      issues: [{ id: 'usage_missing_snapshot', issue: 'missing_snapshot' }],
    });

    expect(get).toHaveBeenCalledWith(
      '/api/v1/admin/billing/reconciliation/relay-usage-prices?organizationID=org_1&userID=user_1&apiTokenID=tok_1&requestID=req_1&apiType=chat&featureType=workspace_chat&quotaMode=relay_billing&model=gpt-4o&channelID=ch_1&provider=openai&status=success&from=2026-07-01T00%3A00%3A00Z&to=2026-07-02T00%3A00%3A00Z&limit=10'
    );
  });

  it('serializes usage request-log coverage filters with backend query keys', async () => {
    const get = vi.fn().mockResolvedValue({
      checkedRecords: 3,
      usageRowsWithRequestId: 2,
      usageRowsMissingRequestId: 1,
      matchedRequestLogRecords: 1,
      missingRequestLogRecords: 1,
      issues: [
        {
          id: 'usage_missing_log',
          requestId: 'req_missing_log',
          model: 'gpt-4o',
          issue: 'missing_request_log',
          createdAt: '2026-07-04T10:00:00Z',
        },
      ],
      limit: 10,
      offset: 0,
    });
    const api = createAdminApi(createClient({ get }));

    await expect(
      api.getUsageRequestLogCoverage({
        organizationId: 'org_1',
        userId: 'user_1',
        apiTokenId: 'tok_1',
        requestId: 'req_1',
        apiType: 'chat',
        featureType: 'workspace_chat',
        quotaMode: 'relay_billing',
        model: 'gpt-4o',
        channelId: 'ch_1',
        provider: 'openai',
        status: 'success',
        limit: 10,
      })
    ).resolves.toMatchObject({
      checkedRecords: 3,
      usageRowsMissingRequestId: 1,
      missingRequestLogRecords: 1,
      issues: [{ id: 'usage_missing_log', issue: 'missing_request_log' }],
    });

    expect(get).toHaveBeenCalledWith(
      '/api/v1/admin/billing/reconciliation/usage-request-logs?organizationID=org_1&userID=user_1&apiTokenID=tok_1&requestID=req_1&apiType=chat&featureType=workspace_chat&quotaMode=relay_billing&model=gpt-4o&channelID=ch_1&provider=openai&status=success&limit=10'
    );
  });

  it('serializes API token filters with backend query keys', async () => {
    const get = vi.fn().mockResolvedValue({
      apiTokens: [
        {
          id: 'tok_1',
          name: 'Browser admin key',
          organizationId: 'org_browser_api_tokens',
          userId: 'user_browser_api_tokens',
        },
      ],
      total: 1,
    });
    const api = createAdminApi(createClient({ get }));

    await expect(
      api.listAPITokens({
        organizationId: 'org_browser_api_tokens',
        userId: 'user_browser_api_tokens',
        status: 'active',
        userGroup: 'enterprise',
        search: 'browser admin',
        model: 'gpt-4o',
        limit: 50,
        offset: 0,
      })
    ).resolves.toEqual({
      data: [
        {
          id: 'tok_1',
          name: 'Browser admin key',
          organizationId: 'org_browser_api_tokens',
          userId: 'user_browser_api_tokens',
        },
      ],
      total: 1,
    });

    const [requestPath] = get.mock.calls[0];
    const requestURL = new URL(requestPath, 'http://oblivious.local');
    expect(requestURL.pathname).toBe('/api/v1/admin/api-tokens');
    expect(Object.fromEntries(requestURL.searchParams.entries())).toEqual({
      status: 'active',
      userGroup: 'enterprise',
      search: 'browser admin',
      model: 'gpt-4o',
      limit: '50',
      offset: '0',
      organizationID: 'org_browser_api_tokens',
      userID: 'user_browser_api_tokens',
    });
    expect(requestURL.searchParams.has('organizationId')).toBe(false);
    expect(requestURL.searchParams.has('userId')).toBe(false);
  });

  it('posts API token revocation through the admin endpoint', async () => {
    const post = vi.fn().mockResolvedValue({ status: 'revoked' });
    const api = createAdminApi(createClient({ post }));

    await expect(api.revokeAPIToken('tok_1')).resolves.toBeUndefined();

    expect(post).toHaveBeenCalledWith('/api/v1/admin/api-tokens/tok_1/revoke');
  });

  it('loads and updates usage limit settings through admin settings endpoints', async () => {
    const get = vi.fn().mockResolvedValue({
      usageLimits: [
        {
          organizationId: 'org_1',
          quotaMode: 'organization',
          maxConcurrentRequests: 10,
          windowSeconds: 60,
          maxTokensPerWindow: 1000,
          maxTokensPerRequest: 250,
        },
      ],
    });
    const put = vi.fn().mockResolvedValue({
      organizationId: 'org_1',
      userId: 'user_1',
      quotaMode: 'user',
      maxConcurrentRequests: 3,
      windowSeconds: 30,
      maxTokensPerWindow: 300,
      maxTokensPerRequest: 75,
    });
    const api = createAdminApi(createClient({ get, put }));

    await expect(api.getUsageLimitSettings()).resolves.toEqual([
      {
        organizationId: 'org_1',
        quotaMode: 'organization',
        maxConcurrentRequests: 10,
        windowSeconds: 60,
        maxTokensPerWindow: 1000,
        maxTokensPerRequest: 250,
      },
    ]);
    await expect(
      api.updateUsageLimitSettings({
        userId: 'user_1',
        quotaMode: 'user',
        maxConcurrentRequests: 3,
        windowSeconds: 30,
        maxTokensPerWindow: 300,
        maxTokensPerRequest: 75,
      })
    ).resolves.toEqual({
      organizationId: 'org_1',
      userId: 'user_1',
      quotaMode: 'user',
      maxConcurrentRequests: 3,
      windowSeconds: 30,
      maxTokensPerWindow: 300,
      maxTokensPerRequest: 75,
    });

    expect(get).toHaveBeenCalledWith('/api/v1/admin/settings/usage-limits');
    expect(put).toHaveBeenCalledWith('/api/v1/admin/settings/usage-limits', {
      userId: 'user_1',
      quotaMode: 'user',
      maxConcurrentRequests: 3,
      windowSeconds: 30,
      maxTokensPerWindow: 300,
      maxTokensPerRequest: 75,
    });
  });

  it('serializes relay pricing catalog import approval and rollback endpoints', async () => {
    const createdImport = {
      id: 'rpci_1',
      provider: 'openai',
      source: 'litellm',
      status: 'pending',
      deactivateMissing: true,
      entries: [],
      diff: { added: 1, updated: 0, unchanged: 0, deactivated: 0, entries: [] },
      createdAt: '2026-07-02T00:00:00Z',
    };
    const get = vi
      .fn()
      .mockResolvedValueOnce({ imports: [createdImport], total: 1 })
      .mockResolvedValueOnce({
        runs: [
          {
            id: 'rpcs_1',
            job: 'manual',
            provider: 'openai',
            source: 'litellm',
            status: 'succeeded',
            entryCount: 2,
            startedAt: '2026-07-02T00:00:00Z',
          },
        ],
        total: 1,
      });
    const post = vi
      .fn()
      .mockResolvedValueOnce(createdImport)
      .mockResolvedValueOnce({ ...createdImport, id: 'rpci_sync' })
      .mockResolvedValueOnce({ ...createdImport, status: 'approved' })
      .mockResolvedValueOnce({ ...createdImport, id: 'rpci_reject', status: 'rejected' })
      .mockResolvedValueOnce({ ...createdImport, id: 'rpci_rollback', source: 'rollback:rpci_1' });
    const api = createAdminApi(createClient({ get, post }));

    await expect(api.listRelayPricingCatalogImports({ provider: 'openai', status: 'pending', limit: 20 })).resolves.toEqual({
      data: [createdImport],
      total: 1,
    });
    await expect(
      api.createRelayPricingCatalogImport({
        provider: 'openai',
        source: 'litellm',
        deactivateMissing: true,
        entries: [
          {
            apiType: 'chat',
            model: 'gpt-4o',
            dimension: 'prompt_tokens',
            unitCost: 0.003,
            markup: 1,
            currency: 'quota',
            source: 'litellm',
            active: true,
          },
        ],
      })
    ).resolves.toEqual(createdImport);
    await expect(
      api.syncRelayPricingCatalogImport({
        provider: 'openai',
        source: 'litellm',
        sourceUrl: 'https://example.test/prices.json',
        requiredModels: ['gpt-4o'],
        maxBytes: 1048576,
      })
    ).resolves.toMatchObject({ id: 'rpci_sync' });
    await expect(api.approveRelayPricingCatalogImport('rpci_1')).resolves.toMatchObject({ status: 'approved' });
    await expect(api.rejectRelayPricingCatalogImport('rpci_reject', 'bad source hash')).resolves.toMatchObject({ status: 'rejected' });
    await expect(api.rollbackRelayPricingCatalogImport('rpci_1', { notes: 'restore previous catalog' })).resolves.toMatchObject({
      id: 'rpci_rollback',
      source: 'rollback:rpci_1',
    });
    await expect(api.listRelayPricingCatalogSyncRuns({ provider: 'openai', status: 'succeeded', limit: 10 })).resolves.toEqual({
      data: [
        {
          id: 'rpcs_1',
          job: 'manual',
          provider: 'openai',
          source: 'litellm',
          status: 'succeeded',
          entryCount: 2,
          startedAt: '2026-07-02T00:00:00Z',
        },
      ],
      total: 1,
    });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/admin/pricing/relay-catalog/imports?provider=openai&status=pending&limit=20');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/pricing/relay-catalog/imports', {
      provider: 'openai',
      source: 'litellm',
      deactivateMissing: true,
      entries: [
        {
          apiType: 'chat',
          model: 'gpt-4o',
          dimension: 'prompt_tokens',
          unitCost: 0.003,
          markup: 1,
          currency: 'quota',
          source: 'litellm',
          active: true,
        },
      ],
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/pricing/relay-catalog/sync', {
      provider: 'openai',
      source: 'litellm',
      sourceUrl: 'https://example.test/prices.json',
      requiredModels: ['gpt-4o'],
      maxBytes: 1048576,
    });
    expect(post).toHaveBeenNthCalledWith(3, '/api/v1/admin/pricing/relay-catalog/imports/rpci_1/approve');
    expect(post).toHaveBeenNthCalledWith(4, '/api/v1/admin/pricing/relay-catalog/imports/rpci_reject/reject', {
      reason: 'bad source hash',
    });
    expect(post).toHaveBeenNthCalledWith(5, '/api/v1/admin/pricing/relay-catalog/imports/rpci_1/rollback', {
      notes: 'restore previous catalog',
    });
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/admin/pricing/relay-catalog/sync-runs?provider=openai&status=succeeded&limit=10');
  });

  it('updates admin user quota allocation with backend PATCH payload', async () => {
    const request = vi.fn().mockResolvedValue({
      id: 'user_1',
      email: 'buyer@example.com',
      quotaBalance: 2500,
    });
    const api = createAdminApi(createClient({ request }));

    await expect(api.updateUserQuota('user_1', { balance: 2500 })).resolves.toEqual({
      id: 'user_1',
      email: 'buyer@example.com',
      quotaBalance: 2500,
    });

    expect(request).toHaveBeenCalledWith('/api/v1/admin/users/user_1', {
      method: 'PATCH',
      body: JSON.stringify({ balance: 2500 }),
    });
  });

  it('serializes admin user plan filters with the documented planID query key', async () => {
    const get = vi.fn().mockResolvedValue({
      users: [
        {
          id: 'user_1',
          email: 'plan-user@example.com',
          planID: 'plan_pro',
        },
      ],
      total: 1,
    });
    const api = createAdminApi(createClient({ get }));

    await expect(api.listUsers({ planId: 'plan_pro', role: 'user', status: 'active', limit: 25 })).resolves.toEqual({
      data: [
        {
          id: 'user_1',
          email: 'plan-user@example.com',
          planID: 'plan_pro',
        },
      ],
      total: 1,
    });

    const [requestPath] = get.mock.calls[0];
    const requestURL = new URL(requestPath, 'http://oblivious.local');
    expect(requestURL.pathname).toBe('/api/v1/admin/users');
    expect(requestURL.searchParams.get('planID')).toBe('plan_pro');
    expect(requestURL.searchParams.has('planId')).toBe(false);
    expect(requestURL.searchParams.get('role')).toBe('user');
    expect(requestURL.searchParams.get('status')).toBe('active');
    expect(requestURL.searchParams.get('limit')).toBe('25');
  });

  it('routes observability alert calls through admin endpoints', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce([
        {
          key: 'relay-backlog',
          status: 'open',
          severity: 'critical',
          component: 'relay',
        },
      ])
      .mockResolvedValueOnce({
        key: 'relay-backlog',
        status: 'open',
        severity: 'critical',
      })
      .mockResolvedValueOnce([
        {
          id: 'attempt_1',
          alertKey: 'relay-backlog',
          channel: 'email',
          delivered: true,
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 'restart-relay:relay-backlog:1',
          policyName: 'restart-relay',
          alertKey: 'relay-backlog',
          type: 'restart',
          status: 'recorded',
        },
      ])
      .mockResolvedValueOnce({
        debug: [],
        info: ['email'],
        warning: ['email', 'im'],
        critical: ['email', 'im', 'sms'],
      });
    const post = vi
      .fn()
      .mockResolvedValueOnce({ key: 'relay-backlog', status: 'acknowledged' })
      .mockResolvedValueOnce({ key: 'relay-backlog', status: 'resolved' });
    const put = vi.fn().mockResolvedValue({
      debug: [],
      info: ['email'],
      warning: ['email', 'im', 'sms'],
      critical: ['email', 'im', 'sms', 'third_party'],
    });
    const api = createAdminApi(createClient({ get, post, put }));

    await expect(api.listObservabilityAlerts({ severity: 'critical', status: 'open', component: 'relay', limit: 25 })).resolves.toEqual([
      {
        key: 'relay-backlog',
        status: 'open',
        severity: 'critical',
        component: 'relay',
      },
    ]);
    await expect(api.getObservabilityAlert('relay-backlog')).resolves.toEqual({
      key: 'relay-backlog',
      status: 'open',
      severity: 'critical',
    });
    await expect(api.acknowledgeObservabilityAlert('relay-backlog')).resolves.toEqual({
      key: 'relay-backlog',
      status: 'acknowledged',
    });
    await expect(api.resolveObservabilityAlert('relay-backlog')).resolves.toEqual({
      key: 'relay-backlog',
      status: 'resolved',
    });
    await expect(api.listObservabilityAlertDeliveries('relay-backlog', { limit: 10 })).resolves.toEqual([
      {
        id: 'attempt_1',
        alertKey: 'relay-backlog',
        channel: 'email',
        delivered: true,
      },
    ]);
    await expect(api.listObservabilityRecoveryActions({ alertKey: 'relay-backlog', type: 'restart' })).resolves.toEqual([
      {
        id: 'restart-relay:relay-backlog:1',
        policyName: 'restart-relay',
        alertKey: 'relay-backlog',
        type: 'restart',
        status: 'recorded',
      },
    ]);
    await expect(api.getObservabilityAlertRoutingRules()).resolves.toEqual({
      debug: [],
      info: ['email'],
      warning: ['email', 'im'],
      critical: ['email', 'im', 'sms'],
    });
    await expect(
      api.updateObservabilityAlertRoutingRules({
        debug: [],
        info: ['email'],
        warning: ['email', 'im', 'sms'],
        critical: ['email', 'im', 'sms', 'third_party'],
      })
    ).resolves.toEqual({
      debug: [],
      info: ['email'],
      warning: ['email', 'im', 'sms'],
      critical: ['email', 'im', 'sms', 'third_party'],
    });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/admin/observability/alerts?severity=critical&status=open&component=relay&limit=25');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/admin/observability/alerts/relay-backlog');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/observability/alerts/relay-backlog/acknowledge');
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/observability/alerts/relay-backlog/resolve');
    expect(get).toHaveBeenNthCalledWith(3, '/api/v1/admin/observability/alerts/relay-backlog/deliveries?limit=10');
    expect(get).toHaveBeenNthCalledWith(4, '/api/v1/admin/observability/recovery-actions?alertKey=relay-backlog&type=restart');
    expect(get).toHaveBeenNthCalledWith(5, '/api/v1/admin/observability/alert-routing');
    expect(put).toHaveBeenCalledWith('/api/v1/admin/observability/alert-routing', {
      rules: {
        debug: [],
        info: ['email'],
        warning: ['email', 'im', 'sms'],
        critical: ['email', 'im', 'sms', 'third_party'],
      },
    });
  });

  it('manages observability alert provider configs through admin endpoints', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        id: 'alert_provider_smtp',
        kind: 'smtp',
        channel: 'email',
        name: 'Primary SMTP',
        status: 'active',
        config: {
          smtp_host: 'smtp.example.com',
          smtp_port: '587',
          username: 'alerts@example.com',
          password: '********',
          from_email: 'alerts@example.com',
          recipients: 'ops@example.com,oncall@example.com',
        },
      },
    ]);
    const post = vi
      .fn()
      .mockResolvedValueOnce({
        id: 'alert_provider_slack',
        kind: 'slack_webhook',
        channel: 'im',
        name: 'Slack Ops',
        status: 'active',
        config: {
          webhook_url: '********',
        },
      })
      .mockResolvedValueOnce({
        providerId: 'alert_provider_slack',
        kind: 'slack_webhook',
        channel: 'im',
        ok: true,
        message: 'provider configuration validated',
        testedAt: '2026-06-07T08:00:00Z',
      });
    const put = vi.fn().mockResolvedValue({
      id: 'alert_provider_smtp',
      kind: 'smtp',
      channel: 'email',
      name: 'Primary SMTP EU',
      status: 'active',
      config: {
        smtp_host: 'smtp.eu.example.com',
        smtp_port: '587',
        username: 'alerts-eu@example.com',
        password: '********',
        from_email: 'alerts-eu@example.com',
        recipients: 'ops-eu@example.com',
      },
    });
    const api = createAdminApi(createClient({ get, post, put }));

    await expect(api.listObservabilityAlertProviders()).resolves.toEqual([
      {
        id: 'alert_provider_smtp',
        kind: 'smtp',
        channel: 'email',
        name: 'Primary SMTP',
        status: 'active',
        config: {
          smtp_host: 'smtp.example.com',
          smtp_port: '587',
          username: 'alerts@example.com',
          password: '********',
          from_email: 'alerts@example.com',
          recipients: 'ops@example.com,oncall@example.com',
        },
      },
    ]);
    await expect(
      api.createObservabilityAlertProvider({
        kind: 'slack_webhook',
        name: 'Slack Ops',
        status: 'active',
        config: {
          webhook_url: 'https://hooks.slack.example/ops',
        },
      })
    ).resolves.toEqual({
      id: 'alert_provider_slack',
      kind: 'slack_webhook',
      channel: 'im',
      name: 'Slack Ops',
      status: 'active',
      config: {
        webhook_url: '********',
      },
    });
    await expect(
      api.updateObservabilityAlertProvider('alert_provider_smtp', {
        kind: 'smtp',
        name: 'Primary SMTP EU',
        status: 'active',
        config: {
          smtp_host: 'smtp.eu.example.com',
          smtp_port: '587',
          username: 'alerts-eu@example.com',
          password: '********',
          from_email: 'alerts-eu@example.com',
          recipients: 'ops-eu@example.com',
        },
      })
    ).resolves.toMatchObject({
      id: 'alert_provider_smtp',
      name: 'Primary SMTP EU',
      config: {
        password: '********',
      },
    });
    await expect(api.testObservabilityAlertProvider('alert_provider_slack')).resolves.toEqual({
      providerId: 'alert_provider_slack',
      kind: 'slack_webhook',
      channel: 'im',
      ok: true,
      message: 'provider configuration validated',
      testedAt: '2026-06-07T08:00:00Z',
    });

    expect(get).toHaveBeenCalledWith('/api/v1/admin/observability/alert-providers');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/observability/alert-providers', {
      kind: 'slack_webhook',
      name: 'Slack Ops',
      status: 'active',
      config: {
        webhook_url: 'https://hooks.slack.example/ops',
      },
    });
    expect(put).toHaveBeenCalledWith('/api/v1/admin/observability/alert-providers/alert_provider_smtp', {
      kind: 'smtp',
      name: 'Primary SMTP EU',
      status: 'active',
      config: {
        smtp_host: 'smtp.eu.example.com',
        smtp_port: '587',
        username: 'alerts-eu@example.com',
        password: '********',
        from_email: 'alerts-eu@example.com',
        recipients: 'ops-eu@example.com',
      },
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/observability/alert-providers/alert_provider_slack/test');
  });
});
