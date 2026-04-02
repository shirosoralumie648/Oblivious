import { render, screen } from '@testing-library/react';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

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
  it('renders navigation, session context, and child routes', () => {
    const router = createMemoryRouter(
      [
        {
          path: '/console',
          element: <ConsoleLayout />,
          children: [
            { index: true, element: <p>Overview page</p> },
            { path: 'models', element: <p>Models page</p> }
          ]
        }
      ],
      { initialEntries: ['/console/models'] }
    );

    render(<RouterProvider router={router} />);

    expect(screen.getByRole('heading', { name: 'Console' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Overview' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Models' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Usage' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Billing' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Access' })).toBeInTheDocument();
    expect(screen.getByText('user@example.com')).toBeInTheDocument();
    expect(screen.getByText('Default mode: solo')).toBeInTheDocument();
    expect(screen.getByText('Models page')).toBeInTheDocument();
  });
});
