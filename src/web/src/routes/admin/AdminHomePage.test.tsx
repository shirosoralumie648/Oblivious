import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getStats = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({ getStats }),
}));

import { AdminHomePage } from './AdminHomePage';

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
    });

    render(<AdminHomePage />);

    expect(await screen.findByText('Channels')).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Dashboard' })).toBeInTheDocument();
    expect(await screen.findByText('Total Users')).toBeInTheDocument();
    expect(await screen.findByText('API Calls (24h)')).toBeInTheDocument();
    expect(await screen.findByText('Active Agents')).toBeInTheDocument();
    expect(await screen.findByText('API Call Volume (7 days)')).toBeInTheDocument();
    expect(await screen.findByText('Channel Uptime')).toBeInTheDocument();
  });

  it('renders retryable error state when stats fail', async () => {
    getStats.mockRejectedValue(new Error('network unavailable'));

    render(<AdminHomePage />);

    expect(await screen.findByText('Something went wrong while loading this data. Please try again or contact support if the issue persists.')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Try Again' })).toBeInTheDocument();
  });
});
