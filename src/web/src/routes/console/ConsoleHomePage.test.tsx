import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

const getAccess = vi.fn();
const getBilling = vi.fn();
const getModels = vi.fn();
const getUsage = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getBilling,
    getModels,
    getUsage
  })
}));

import { ConsoleHomePage } from './ConsoleHomePage';

describe('ConsoleHomePage', () => {
  afterEach(() => {
    getAccess.mockReset();
    getBilling.mockReset();
    getModels.mockReset();
    getUsage.mockReset();
  });

  it('renders drill-down cards for billing, usage, models, and access', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'solo',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    getUsage.mockResolvedValue({ period: '7d', requests: 3 });
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004
    });
    getModels.mockResolvedValue([
      { id: 'balanced-chat', label: 'balanced-chat', requests: 2 }
    ]);

    render(
      <MemoryRouter>
        <ConsoleHomePage />
      </MemoryRouter>
    );

    expect(await screen.findByRole('link', { name: 'Estimated cost' })).toHaveAttribute('href', '/console/billing');
    expect(screen.getByRole('link', { name: 'Requests' })).toHaveAttribute('href', '/console/usage');
    expect(screen.getByRole('link', { name: 'Top model' })).toHaveAttribute('href', '/console/models');
    expect(screen.getByRole('link', { name: 'Access posture' })).toHaveAttribute('href', '/console/access');
    expect(screen.getByText('Current workspace scope: workspace_1')).toBeInTheDocument();
  });

  it('keeps the dashboard available when one summary fails', async () => {
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
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004
    });
    getModels.mockRejectedValue(new Error('network unavailable'));
    getUsage.mockResolvedValue({ period: '7d', requests: 3 });

    render(
      <MemoryRouter>
        <ConsoleHomePage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Estimated cost')).toBeInTheDocument();
    expect(screen.getByText('Top model unavailable')).toBeInTheDocument();
    expect(screen.queryByText('Unable to load dashboard.')).not.toBeInTheDocument();
  });

  it('renders a fallback message when the dashboard fails to load', async () => {
    getAccess.mockRejectedValue(new Error('network unavailable'));
    getBilling.mockRejectedValue(new Error('network unavailable'));
    getModels.mockRejectedValue(new Error('network unavailable'));
    getUsage.mockRejectedValue(new Error('network unavailable'));

    render(
      <MemoryRouter>
        <ConsoleHomePage />
      </MemoryRouter>
    );

    expect(screen.getByText('Loading dashboard…')).toBeInTheDocument();
    expect(await screen.findByText('Unable to load dashboard.')).toBeInTheDocument();
  });
});
