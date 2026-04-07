import { render, screen } from '@testing-library/react';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    },
    status: 'authenticated',
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

import { ConsoleLayout } from './ConsoleLayout';

describe('ConsoleLayout', () => {
  it('renders an admin shell with scope messaging and workspace shortcuts', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/console',
          element: <ConsoleLayout />,
          children: [{ index: true, element: <p>Overview page</p> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/console'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    expect(await screen.findByRole('heading', { name: 'Console' })).toBeInTheDocument();
    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(await screen.findByText('user@example.com')).toBeInTheDocument();
    expect(await screen.findByText('Default mode: solo')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Workspace settings' })).toHaveAttribute('href', '/settings');
    expect(await screen.findByRole('link', { name: 'Return to workspace' })).toHaveAttribute('href', '/chat');
    expect(await screen.findByText('Overview page')).toBeInTheDocument();
  });

  it('renders console navigation in overview-billing-usage-models-access order', async () => {
    const router = createMemoryRouter(
      [
        {
          path: '/console',
          element: <ConsoleLayout />,
          children: [{ index: true, element: <p>Overview page</p> }]
        }
      ],
      { future: routerFuture, initialEntries: ['/console'] }
    );

    render(<RouterProvider future={routerFuture} router={router} />);

    await screen.findByRole('heading', { name: 'Console' });

    const links = (await screen.findAllByRole('link'))
      .map((link) => link.textContent)
      .filter((text): text is string => text !== null);

    expect(links).toEqual([
      'Workspace settings',
      'Return to workspace',
      'Overview',
      'Billing',
      'Usage',
      'Models',
      'Access'
    ]);
  });
});
