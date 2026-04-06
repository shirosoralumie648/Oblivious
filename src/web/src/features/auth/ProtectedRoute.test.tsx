import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { UserPreferences } from '../../types/api';

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat' as const,
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: false
    } as UserPreferences,
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

import { ProtectedRoute } from './ProtectedRoute';

describe('ProtectedRoute', () => {
  beforeEach(() => {
    appContext.authState.preferences = {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: false
    } as UserPreferences;
  });

  it('allows authenticated users with incomplete onboarding to remain on /chat', () => {
    render(
      <MemoryRouter initialEntries={['/chat']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route element={<div>chat shell</div>} path="/chat" />
            <Route element={<div>onboarding shell</div>} path="/onboarding" />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('chat shell')).toBeInTheDocument();
  });

  it('redirects completed onboarding users away from /onboarding using the default mode', () => {
    appContext.authState.preferences = {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    } as UserPreferences;

    render(
      <MemoryRouter initialEntries={['/onboarding']}>
        <Routes>
          <Route element={<ProtectedRoute />}>
            <Route element={<div>solo shell</div>} path="/solo/new" />
            <Route element={<div>onboarding shell</div>} path="/onboarding" />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('solo shell')).toBeInTheDocument();
  });
});
