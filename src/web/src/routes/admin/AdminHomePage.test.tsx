import { render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SWRConfig } from 'swr';

const getStats = vi.fn();

vi.mock('../../lib/swr', () => ({
  fetcher: () => getStats(),
}));

import { AdminHomePage } from './AdminHomePage';

function renderWithSWR(component: React.ReactElement) {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      {component}
    </SWRConfig>
  );
}

describe('AdminHomePage', () => {
  beforeEach(() => {
    getStats.mockReset();
  });

  it('renders admin dashboard metric cards and charts from stats', async () => {
    getStats.mockResolvedValue({
      users: { totalUsers: 24, activeUsers: 12, newUsersToday: 2, newUsersWeek: 5 },
      quotas: { totalBalance: 100, totalUsed: 25, activeTopups: 3 },
      conversations: 8,
      agents: 11,
      tasks: 4,
      mcpServers: 2,
      channelsTotal: 4,
      channelsOnline: 3,
      activeAgents: 7,
      apiCalls24h: 128,
      dailyStats: [
        { date: '06-05', calls: 100 },
        { date: '06-06', calls: 120 },
        { date: '06-07', calls: 110 },
        { date: '06-08', calls: 130 },
        { date: '06-09', calls: 125 },
        { date: '06-10', calls: 128 },
        { date: '06-11', calls: 135 },
      ],
      modelBreakdown: [
        { model: 'gpt-4', count: 50 },
        { model: 'gpt-3.5-turbo', count: 30 },
        { model: 'claude-3', count: 20 },
      ],
    });

    renderWithSWR(<AdminHomePage />);

    expect((await screen.findAllByText('Channels')).length).toBeGreaterThan(0);
    expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeInTheDocument();
    expect(await screen.findByText('Total Users')).toBeInTheDocument();
    expect(await screen.findByText('API Calls (24h)')).toBeInTheDocument();
    expect(await screen.findByText('Active Agents')).toBeInTheDocument();
    expect(await screen.findByText('API Call Volume (7 days)')).toBeInTheDocument();
    expect(await screen.findByText('Channel Uptime')).toBeInTheDocument();
    expect(await screen.findByText('API 调用趋势（最近 7 天）')).toBeInTheDocument();
    expect(await screen.findByText('模型使用占比')).toBeInTheDocument();
  });

  it('renders commercial operations module coverage', async () => {
    getStats.mockResolvedValue({
      users: { totalUsers: 24, activeUsers: 12, newUsersToday: 2, newUsersWeek: 5 },
      quotas: { totalBalance: 100, totalUsed: 25, activeTopups: 3 },
      conversations: 8,
      agents: 11,
      tasks: 4,
      mcpServers: 2,
      channelsTotal: 4,
      channelsOnline: 3,
      activeAgents: 7,
      apiCalls24h: 128,
      dailyStats: [
        { date: '06-05', calls: 100 },
        { date: '06-06', calls: 120 },
        { date: '06-07', calls: 110 },
        { date: '06-08', calls: 130 },
        { date: '06-09', calls: 125 },
        { date: '06-10', calls: 128 },
        { date: '06-11', calls: 135 },
      ],
      modelBreakdown: [
        { model: 'gpt-4', count: 50 },
        { model: 'gpt-3.5-turbo', count: 30 },
        { model: 'claude-3', count: 20 },
      ],
    });

    renderWithSWR(<AdminHomePage />);

    expect(await screen.findByRole('heading', { name: 'Commercial operations' })).toBeInTheDocument();
    const operations = screen.getByRole('region', { name: 'Commercial operations' });
    expect(within(operations).getByText('Channels')).toBeInTheDocument();
    expect(within(operations).getByText('Routes')).toBeInTheDocument();
    expect(within(operations).getByText('Plans')).toBeInTheDocument();
    expect(within(operations).getByText('Billing')).toBeInTheDocument();
    expect(within(operations).getByText('Users')).toBeInTheDocument();
    expect(within(operations).getByText('Audit Log')).toBeInTheDocument();
    expect(within(operations).getByText('Review Queue')).toBeInTheDocument();
  });

  it('renders retryable error state when stats fail', async () => {
    getStats.mockRejectedValue(new Error('network unavailable'));

    renderWithSWR(<AdminHomePage />);

    expect(await screen.findByText('Something went wrong while loading this data. Please try again or contact support if the issue persists.')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Try Again' })).toBeInTheDocument();
  });
});
