import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

vi.mock('../features/console/api', () => ({
  createConsoleApi: () => ({
    getAccess: () =>
      Promise.resolve({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: true,
        onboardingCompleted: true,
        sessionExpiresAt: '2026-04-03T00:00:00Z',
        sessionId: 'session_1',
        userEmail: 'user@example.com',
        userId: 'user_1',
        workspaceId: 'workspace_1'
      }),
    getBilling: () =>
      Promise.resolve({
        period: '30d',
        requests: 5,
        inputTokens: 120,
        outputTokens: 80,
        estimatedCostUsd: 0.0004
      }),
    getModels: () =>
      Promise.resolve([{ id: 'balanced-chat', label: 'balanced-chat', requests: 2 }]),
    getUsage: () => Promise.resolve({ period: '7d', requests: 3 })
  })
}));

import { createAppRouter } from './router';
import { routerFuture } from './routerFuture';

describe('app router', () => {
  it('renders home content on /', async () => {
    const router = createAppRouter(['/']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Oblivious')).toBeInTheDocument();
    expect(await screen.findByText('AI workspace framework')).toBeInTheDocument();
  });

  it('renders knowledge route inside the workspace shell', async () => {
    const router = createAppRouter(['/knowledge']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Knowledge' })).toBeInTheDocument();
  });

  it('renders onboarding inside the workspace shell', async () => {
    const router = createAppRouter(['/onboarding']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Onboarding' })).toBeInTheDocument();
  });

  it('renders solo route inside the workspace shell', async () => {
    const router = createAppRouter(['/solo']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'SOLO' })).toBeInTheDocument();
  });

  it('renders billing route inside the console shell', async () => {
    const router = createAppRouter(['/console/billing']);

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Console')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Billing' })).toBeInTheDocument();
  });
});
