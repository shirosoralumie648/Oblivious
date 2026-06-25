import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listUsageLogs = vi.fn();
const getUsageAnalytics = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    listUsageLogs,
    getUsageAnalytics,
  }),
}));

import { AdminUsageLogsPage } from './AdminUsageLogsPage';

describe('AdminUsageLogsPage', () => {
  beforeEach(() => {
    listUsageLogs.mockReset();
    getUsageAnalytics.mockReset();
    getUsageAnalytics.mockResolvedValue({
      byModel: [],
      byFeature: [],
      byUser: [],
      byTime: [],
      byChannel: [],
      byProvider: [],
      crossDimensions: [],
    });
  });

  it('renders relay usage logs with request, route, cost, and status fields', async () => {
    listUsageLogs.mockResolvedValue({
      data: [
        {
          id: 'usage_1',
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
          statusCode: 200,
          latencyMs: 42,
          cost: 0.42,
          channelCost: 0.21,
          promptTokens: 100,
          completionTokens: 20,
          totalTokens: 120,
          createdAt: '2026-06-01T10:00:00Z',
        },
      ],
      total: 1,
    });

    render(<AdminUsageLogsPage />);

    expect(await screen.findByRole('heading', { name: 'Usage Logs' })).toBeInTheDocument();
    expect(await screen.findByText('req_1')).toBeInTheDocument();
    expect(screen.getByText('user_1')).toBeInTheDocument();
    expect(screen.getByText('tok_1')).toBeInTheDocument();
    expect(screen.getByText('chat')).toBeInTheDocument();
    expect(screen.getByText('workspace_chat')).toBeInTheDocument();
    expect(screen.getByText('relay_billing')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
    expect(screen.getByText('openai / ch_1')).toBeInTheDocument();
    expect(screen.getByLabelText('Success')).toBeInTheDocument();
    expect(screen.getByText('$0.4200')).toBeInTheDocument();
    expect(screen.getByText('42 ms')).toBeInTheDocument();
    expect(screen.getByText('120')).toBeInTheDocument();
  });

  it('renders usage analytics panels for model, feature, user, time, channel, and provider dimensions', async () => {
    listUsageLogs.mockResolvedValue({ data: [], total: 0 });
    getUsageAnalytics.mockResolvedValue({
      byModel: [{ dimension: 'model', key: 'gpt-4o', requestCount: 3, totalTokens: 150, totalCost: 0.0012 }],
      byFeature: [{ dimension: 'feature', key: 'chat', requestCount: 2, totalTokens: 120, totalCost: 0.0009 }],
      byUser: [{ dimension: 'user', key: 'user_1', requestCount: 4, totalTokens: 200, totalCost: 0.0015 }],
      byTime: [
        {
          dimension: 'time',
          key: '2026-06-04T00:00:00Z',
          startedAt: '2026-06-04T00:00:00Z',
          requestCount: 5,
          totalTokens: 300,
          totalCost: 0.002,
        },
      ],
      byChannel: [{ dimension: 'channel', key: 'ch_1', requestCount: 6, totalTokens: 360, totalCost: 0.0025 }],
      byProvider: [{ dimension: 'provider', key: 'openai', requestCount: 7, totalTokens: 420, totalCost: 0.003 }],
    });

    render(<AdminUsageLogsPage />);

    expect(await screen.findByRole('heading', { name: 'Usage Analytics' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By model' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By feature' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By user' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By time' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By channel' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By provider' })).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
    expect(screen.getByText('chat')).toBeInTheDocument();
    expect(screen.getByText('user_1')).toBeInTheDocument();
    expect(screen.getByText('2026-06-04T00:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('ch_1')).toBeInTheDocument();
    expect(screen.getByText('openai')).toBeInTheDocument();
    expect(screen.getByText('$0.0012')).toBeInTheDocument();
  });

  it('renders usage analytics cross dimensions for multidimensional analysis', async () => {
    listUsageLogs.mockResolvedValue({ data: [], total: 0 });
    getUsageAnalytics.mockResolvedValue({
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
        {
          dimension: 'user_feature',
          key: 'user_1 / workspace_chat',
          primary: 'user_1',
          secondary: 'workspace_chat',
          requestCount: 7,
          totalTokens: 700,
          totalCost: 0.007,
        },
        {
          dimension: 'feature_time',
          key: 'agent_run / 2026-06-04T01:00:00Z',
          primary: 'agent_run',
          secondary: '2026-06-04T01:00:00Z',
          requestCount: 5,
          totalTokens: 500,
          totalCost: 0.005,
        },
      ],
    });

    render(<AdminUsageLogsPage />);

    expect(await screen.findByRole('heading', { name: 'Cross dimensions' })).toBeInTheDocument();
    expect(screen.getByText('model_time', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('user_feature', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('feature_time', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('gpt-4o / 2026-06-04T00:00:00Z', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('user_1 / workspace_chat', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('agent_run / 2026-06-04T01:00:00Z', { exact: false })).toBeInTheDocument();
  });

  it('passes filters to listUsageLogs', async () => {
    listUsageLogs.mockResolvedValue({ data: [], total: 0 });

    render(<AdminUsageLogsPage />);

    fireEvent.change(await screen.findByLabelText('Organization ID filter'), { target: { value: 'org_1' } });
    fireEvent.change(screen.getByLabelText('Request ID filter'), { target: { value: 'req_1' } });
    fireEvent.change(screen.getByLabelText('API token ID filter'), { target: { value: 'tok_1' } });
    fireEvent.change(screen.getByLabelText('API type filter'), { target: { value: 'chat' } });
    fireEvent.change(screen.getByLabelText('Feature type filter'), { target: { value: 'workspace_chat' } });
    fireEvent.change(screen.getByLabelText('Quota mode filter'), { target: { value: 'relay_billing' } });
    fireEvent.change(screen.getByLabelText('Channel ID filter'), { target: { value: 'ch_1' } });
    fireEvent.change(screen.getByLabelText('Provider filter'), { target: { value: 'openai' } });
    fireEvent.change(screen.getByLabelText('Status filter'), { target: { value: 'success' } });
    fireEvent.change(screen.getByLabelText('Model filter'), { target: { value: 'gpt-4o' } });
    fireEvent.change(screen.getByLabelText('Analytics granularity filter'), { target: { value: 'month' } });

    await waitFor(() =>
      expect(listUsageLogs).toHaveBeenLastCalledWith(
        expect.objectContaining({
          organizationID: 'org_1',
          requestID: 'req_1',
          apiTokenID: 'tok_1',
          apiType: 'chat',
          featureType: 'workspace_chat',
          quotaMode: 'relay_billing',
          channelID: 'ch_1',
          provider: 'openai',
          status: 'success',
          model: 'gpt-4o',
          limit: 50,
        })
      )
    );
    await waitFor(() =>
      expect(getUsageAnalytics).toHaveBeenLastCalledWith(
        expect.objectContaining({
          organizationID: 'org_1',
          userID: '',
          apiType: 'chat',
          featureType: 'workspace_chat',
          quotaMode: 'relay_billing',
          channelID: 'ch_1',
          provider: 'openai',
          status: 'success',
          model: 'gpt-4o',
          granularity: 'month',
          limit: 8,
        })
      )
    );
  });
});
