import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const navigate = vi.fn();
const updatePreferences = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');

  return {
    ...actual,
    useNavigate: () => navigate
  };
});

vi.mock('../../app/providers', () => ({
  useAppContext: () => ({
    authState: {
      preferences: null,
      status: 'authenticated',
      user: { email: 'user@example.com', id: 'u1' }
    },
    updatePreferences
  })
}));

import { OnboardingPage } from './OnboardingPage';

describe('OnboardingPage', () => {
  beforeEach(() => {
    navigate.mockReset();
    updatePreferences.mockReset();
    updatePreferences.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    });
  });

  it('offers chat and solo choices for first-run mode selection', () => {
    render(
      <MemoryRouter future={routerFuture}>
        <OnboardingPage />
      </MemoryRouter>
    );

    expect(screen.getByRole('button', { name: 'Start with Chat' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start with SOLO' })).toBeInTheDocument();
  });

  it('lets users skip onboarding from the page shell', () => {
    render(
      <MemoryRouter future={routerFuture}>
        <OnboardingPage />
      </MemoryRouter>
    );

    expect(screen.getByRole('button', { name: 'Skip for now' })).toBeInTheDocument();
  });

  it('shows preference options after choosing a mode', () => {
    render(
      <MemoryRouter future={routerFuture}>
        <OnboardingPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Start with Chat' }));

    expect(screen.getByText('Default model strategy')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Continue to workspace' })).toBeInTheDocument();
  });

  it('saves preferences and routes to /chat after choosing chat', async () => {
    render(
      <MemoryRouter future={routerFuture}>
        <OnboardingPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Start with Chat' }));
    fireEvent.click(screen.getByRole('button', { name: 'Continue to workspace' }));

    await waitFor(() => {
      expect(updatePreferences).toHaveBeenCalledWith({
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: true
      });
    });

    expect(navigate).toHaveBeenCalledWith('/chat');
  });

  it('saves preferences and routes to /solo/new after choosing solo', async () => {
    render(
      <MemoryRouter future={routerFuture}>
        <OnboardingPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Start with SOLO' }));
    fireEvent.click(screen.getByRole('button', { name: 'Continue to workspace' }));

    await waitFor(() => {
      expect(updatePreferences).toHaveBeenCalledWith({
        defaultMode: 'solo',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: true
      });
    });

    expect(navigate).toHaveBeenCalledWith('/solo/new');
  });

  it('routes to /chat without saving when the user skips onboarding', () => {
    render(
      <MemoryRouter future={routerFuture}>
        <OnboardingPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Skip for now' }));

    expect(updatePreferences).not.toHaveBeenCalled();
    expect(navigate).toHaveBeenCalledWith('/chat');
  });
});
