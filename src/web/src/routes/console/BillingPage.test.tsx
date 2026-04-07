import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

const getAccess = vi.fn();
const getBilling = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getBilling
  })
}));

import { BillingPage } from './BillingPage';

describe('BillingPage', () => {
  afterEach(() => {
    getAccess.mockReset();
    getBilling.mockReset();
  });

  it('renders billing inside a workbench layout with context rail and sibling links', async () => {
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
    getBilling.mockResolvedValue({
      period: '30d',
      requests: 5,
      inputTokens: 120,
      outputTokens: 80,
      estimatedCostUsd: 0.0004
    });

    render(
      <MemoryRouter>
        <BillingPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(screen.getByText('Workspace: workspace_1')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Back to overview' })).toHaveAttribute('href', '/console');
    expect(screen.getByRole('link', { name: 'Open usage' })).toHaveAttribute('href', '/console/usage');
    expect(screen.getByText('Estimated cost: $0.0004')).toBeInTheDocument();
  });
});
