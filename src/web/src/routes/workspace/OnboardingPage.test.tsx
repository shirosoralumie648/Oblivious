import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';

vi.mock('../../app/providers', () => ({
  useAppContext: () => ({
    authState: {
      preferences: null,
      status: 'authenticated',
      user: { email: 'user@example.com', id: 'u1' }
    },
    updatePreferences: vi.fn(async () => ({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    }))
  })
}));

import { OnboardingPage } from './OnboardingPage';

describe('OnboardingPage', () => {
  it('offers chat and solo choices for first-run mode selection', () => {
    render(
      <MemoryRouter>
        <OnboardingPage />
      </MemoryRouter>
    );

    expect(screen.getByRole('button', { name: 'Start with Chat' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Start with SOLO' })).toBeInTheDocument();
  });

  it('lets users skip onboarding from the page shell', () => {
    render(
      <MemoryRouter>
        <OnboardingPage />
      </MemoryRouter>
    );

    expect(screen.getByRole('button', { name: 'Skip for now' })).toBeInTheDocument();
  });

  it('shows preference options after choosing a mode', () => {
    render(
      <MemoryRouter>
        <OnboardingPage />
      </MemoryRouter>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Start with Chat' }));

    expect(screen.getByText('Default model strategy')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Continue to workspace' })).toBeInTheDocument();
  });
});
