import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const getAccess = vi.fn();
const getUsage = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getUsage
  })
}));

import { UsagePage } from './UsagePage';

describe('UsagePage', () => {
  afterEach(() => {
    getAccess.mockReset();
    getUsage.mockReset();
  });

  it('keeps the usage workbench frame available when the summary fails', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getUsage.mockRejectedValue(new Error('usage unavailable'));

    render(
      <MemoryRouter future={routerFuture}>
        <UsagePage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Back to overview' })).toBeInTheDocument();
    expect(await screen.findByText('Unable to load usage summary.')).toBeInTheDocument();
  });

  it('renders recent relay usage details for the current user', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getUsage.mockResolvedValue({
      period: '7d',
      requests: 1,
      recent: [
        {
          id: 'usage_1',
          apiTokenId: 'tok_1',
          requestId: 'req_1',
          apiType: 'chat',
          model: 'gpt-4o',
          status: 'success',
          statusCode: 200,
          latencyMs: 42,
          cost: 0.42,
          promptTokens: 100,
          completionTokens: 20,
          totalTokens: 120,
          createdAt: '2026-06-01T10:00:00Z'
        }
      ]
    });

    render(
      <MemoryRouter future={routerFuture}>
        <UsagePage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Requests: 1')).toBeInTheDocument();
    expect(await screen.findByText('req_1')).toBeInTheDocument();
    expect(screen.getByText('tok_1')).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
    expect(screen.queryByText('openai / ch_1')).not.toBeInTheDocument();
    expect(screen.getByText('success')).toBeInTheDocument();
    expect(screen.getByText('120 tokens')).toBeInTheDocument();
    expect(screen.getByText('$0.4200')).toBeInTheDocument();
    expect(screen.getByText('42 ms')).toBeInTheDocument();
  });

  it('renders model, feature, user, and time usage aggregations', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getUsage.mockResolvedValue({
      period: '7d',
      requests: 5,
      byModel: [{ key: 'gpt-4o', requestCount: 3, totalTokens: 300, totalCost: 0.09 }],
      byFeature: [{ key: 'workflow', requestCount: 2, totalTokens: 180, totalCost: 0.04 }],
      byUser: [{ key: 'user_2', requestCount: 2, totalTokens: 240, totalCost: 0.12 }],
      timeSeries: [{ bucket: '2026-06-05', requestCount: 3, totalTokens: 280, totalCost: 0.09 }],
      recent: []
    });

    render(
      <MemoryRouter future={routerFuture}>
        <UsagePage />
      </MemoryRouter>
    );

    expect(await screen.findByRole('heading', { name: 'By model' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'By feature' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Top users' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Daily trend' })).toBeInTheDocument();
    expect(screen.getByText('gpt-4o')).toBeInTheDocument();
    expect(screen.getByText('workflow')).toBeInTheDocument();
    expect(screen.getByText('user_2')).toBeInTheDocument();
    expect(screen.getByText('2026-06-05')).toBeInTheDocument();
    expect(screen.getAllByText('3 req')).toHaveLength(2);
    expect(screen.getByText('240 tokens')).toBeInTheDocument();
    expect(screen.getByText('$0.1200')).toBeInTheDocument();
  });
});
