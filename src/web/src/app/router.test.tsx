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

  it('renders pricing content on /pricing', () => {
    const router = createAppRouter(['/pricing']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Pricing')).toBeInTheDocument();
    expect(screen.getByText('Choose a plan that matches your workload.')).toBeInTheDocument();
  });

  it('renders download content on /download', () => {
    const router = createAppRouter(['/download']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Download')).toBeInTheDocument();
    expect(screen.getByText('Pick the client that fits your device and workflow.')).toBeInTheDocument();
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

  it('shows session loading for the protected solo route', () => {
    const router = createAppRouter(['/solo']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('shows session loading for the protected solo task creation route', () => {
    const router = createAppRouter(['/solo/new']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('shows session loading for the protected knowledge route', () => {
    const router = createAppRouter(['/knowledge']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });

  it('shows session loading for the protected knowledge detail route', () => {
    const router = createAppRouter(['/knowledge/kb_1']);

    render(
      <AppProviders>
        <RouterProvider router={router} />
      </AppProviders>
    );

    expect(screen.getByText('Loading session…')).toBeInTheDocument();
  });
});
