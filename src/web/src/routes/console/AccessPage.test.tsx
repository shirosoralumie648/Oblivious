import { render, screen } from '@testing-library/react';
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

  it('loads and renders the current access context', async () => {
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

    render(<AccessPage />);

    expect(screen.getByText('Loading access summary…')).toBeInTheDocument();
    expect(await screen.findByText('User: user@example.com')).toBeInTheDocument();
    expect(screen.getByText('Workspace: workspace_1')).toBeInTheDocument();
    expect(screen.getByText('Session: session_1')).toBeInTheDocument();
    expect(screen.getByText('Default mode: chat')).toBeInTheDocument();
  });
});
