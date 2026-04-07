import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

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
      <MemoryRouter>
        <UsagePage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Back to overview' })).toBeInTheDocument();
    expect(screen.getByText('Unable to load usage summary.')).toBeInTheDocument();
  });
});
