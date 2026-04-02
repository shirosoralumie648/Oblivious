import { render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AppProviders } from './providers';
import { createAppRouter } from './router';

describe('app router', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders home content on /', () => {
    const router = createAppRouter(['/']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Oblivious')).toBeInTheDocument();
    expect(screen.getByText('AI workspace framework')).toBeInTheDocument();
  });

  it('shows session loading for protected workspace routes', () => {
    const router = createAppRouter(['/chat']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('shows session loading for protected console routes', () => {
    const router = createAppRouter(['/console']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('renders onboarding route content', () => {
    const router = createAppRouter(['/onboarding']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });
});
