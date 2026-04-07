import { render, screen } from '@testing-library/react';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    },
    status: 'authenticated',
    user: { email: 'user@example.com', id: 'u1' }
  },
  authStore: {
    clearUser: vi.fn()
  },
  refreshAuthState: vi.fn()
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

import { WorkspaceLayout } from './WorkspaceLayout';

describe('WorkspaceLayout', () => {
  it('renders workspace navigation with a console entry', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <WorkspaceLayout />,
          children: [{ index: true, element: <p>Workspace child</p> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByText('Workspace')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Chat' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'SOLO' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Knowledge' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Console' })).toBeInTheDocument();
    expect(screen.getByText('Workspace child')).toBeInTheDocument();
  });

  it('renders workspace navigation in chat-first order', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <WorkspaceLayout />,
          children: [{ index: true, element: <p>Workspace child</p> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    await screen.findByText('Workspace');

    const links = screen.getAllByRole('link').map((link) => link.textContent);

    expect(links).toEqual(['Chat', 'Knowledge', 'SOLO', 'Settings', 'Console']);
  });
});
