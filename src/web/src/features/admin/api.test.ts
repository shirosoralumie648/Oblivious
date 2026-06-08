import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createAdminApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    request: vi.fn(),
    ...overrides,
  };
  return client;
}

describe('createAdminApi', () => {
  it('normalizes list responses from backend collection keys', async () => {
    const client = createClient({
      get: vi
        .fn()
        .mockResolvedValueOnce({ channels: [{ id: 'ch_1', name: 'OpenAI' }], total: 1 })
        .mockResolvedValueOnce({ routes: [{ id: 'rt_1', model: 'gpt-4o' }], total: 1 })
        .mockResolvedValueOnce({ entries: [{ id: 'aud_1', action: 'channel.create' }], total: 1 }),
    });

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

    expect(client.get).toHaveBeenNthCalledWith(1, '/api/v1/admin/channels?provider=openai&limit=10');
    expect(client.get).toHaveBeenNthCalledWith(2, '/api/v1/admin/routes');
    expect(client.get).toHaveBeenNthCalledWith(3, '/api/v1/admin/audit-logs?action=channel.create');
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

  it('requests publisher changes through the admin marketplace review endpoint', async () => {
    const post = vi.fn().mockResolvedValue({ status: 'needs_changes' });
    const api = createAdminApi(createClient({ post }));

    await api.requestAgentChanges('agent_1', 'Add screenshots and clarify pricing.');

    expect(post).toHaveBeenCalledWith('/api/v1/admin/reviews/agent_1/needs-changes', {
      reason: 'Add screenshots and clarify pricing.',
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
      .mockResolvedValueOnce({ paymentIntents: [{ id: 'pi_1', kind: 'subscription' }], total: 1 });
    const post = vi.fn().mockResolvedValue({ id: 'payout_1', status: 'paid_out', providerPayoutId: 'provider-paid-1' });
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
    await expect(api.markMarketplacePayoutPaid('payout_1', 'provider-paid-1')).resolves.toEqual({
      id: 'payout_1',
      status: 'paid_out',
      providerPayoutId: 'provider-paid-1',
    });
    post.mockResolvedValueOnce({ id: 'refund_1', status: 'succeeded', providerRefundId: 're_1' });
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
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/admin/billing/payouts/payout_1/paid', { providerPayoutID: 'provider-paid-1' });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/admin/billing/topups/topup_1/refund', {
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
