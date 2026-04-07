import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const getAccess = vi.fn();
const getModels = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess,
    getModels
  })
}));

import { ModelsPage } from './ModelsPage';

describe('ModelsPage', () => {
  afterEach(() => {
    getAccess.mockReset();
    getModels.mockReset();
  });

  it('renders models as a supporting drill-down with context rail', async () => {
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
    getModels.mockResolvedValue([
      { id: 'balanced-chat', label: 'balanced-chat', requests: 2 },
      { id: 'quality-chat', label: 'quality-chat', requests: 1 }
    ]);

    render(
      <MemoryRouter future={routerFuture}>
        <ModelsPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Back to overview' })).toBeInTheDocument();
    expect(await screen.findByText('balanced-chat')).toBeInTheDocument();
    expect(await screen.findByText('Requests: 2')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Open access' })).toHaveAttribute('href', '/console/access');
  });

  it('renders a fallback message when model summaries fail to load', async () => {
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
    getModels.mockRejectedValue(new Error('network unavailable'));

    render(
      <MemoryRouter future={routerFuture}>
        <ModelsPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(await screen.findByText('Unable to load model summaries.')).toBeInTheDocument();
  });
});
