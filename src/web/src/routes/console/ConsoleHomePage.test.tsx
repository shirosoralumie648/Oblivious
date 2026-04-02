import { render, screen } from '@testing-library/react';
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

  it('loads and renders dashboard cards from console summaries', async () => {
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

    render(<ConsoleHomePage />);

    expect(screen.getByText('Loading dashboard…')).toBeInTheDocument();
    expect(await screen.findByText('Requests (7d): 3')).toBeInTheDocument();
    expect(screen.getByText('Estimated cost (30d): $0.0004')).toBeInTheDocument();
    expect(screen.getByText('Top model: balanced-chat')).toBeInTheDocument();
    expect(screen.getByText('Workspace: workspace_1')).toBeInTheDocument();
    expect(screen.getByText('User: user@example.com')).toBeInTheDocument();
  });
});
