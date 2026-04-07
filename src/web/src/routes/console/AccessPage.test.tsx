import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

const getAccess = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess
  })
}));

import { AccessPage } from './AccessPage';

describe('AccessPage', () => {
  afterEach(() => {
    getAccess.mockReset();
  });

  it('renders the access page as a scope explanation workbench', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });

    render(
      <MemoryRouter>
        <AccessPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(screen.getByText('This console reflects the active workspace and current session.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Workspace settings' })).toBeInTheDocument();
  });
});
