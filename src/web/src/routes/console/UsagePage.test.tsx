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
});
