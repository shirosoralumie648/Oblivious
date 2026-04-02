import { describe, expect, it } from 'vitest';

import type { SessionResponse, UserPreferences } from '../../types/api';

import { createAuthStore } from './store';
import { createAuthBootstrapController } from './useAuthBootstrap';

const onboardingRequiredPreferences: UserPreferences = {
  defaultMode: 'chat',
  modelStrategy: 'balanced',
  networkEnabledHint: false,
  onboardingCompleted: false
};

const completedPreferences: UserPreferences = {
  ...onboardingRequiredPreferences,
  onboardingCompleted: true
};

const makeSession = (preferences: UserPreferences): SessionResponse => ({
  onboardingCompleted: preferences.onboardingCompleted,
  preferences,
  session: { expiresAt: '2026-04-02T00:00:00Z', id: 's1' },
  user: { id: 'u1', email: 'user@example.com' },
  workspace: { id: 'w1' }
});

describe('auth store', () => {
  it('starts loading and stores authenticated session', () => {
    const store = createAuthStore();

    expect(store.getState().status).toBe('loading');
    expect(store.getState().user).toBeNull();

    store.setAuthenticatedSession({ id: 'u1', email: 'user@example.com' }, completedPreferences);

    expect(store.getState().status).toBe('authenticated');
    expect(store.getState().user).toEqual({
      id: 'u1',
      email: 'user@example.com'
    });
    expect(store.getState().preferences).toEqual(completedPreferences);
  });

  it('transitions to unauthenticated when loading finishes without a user', () => {
    const store = createAuthStore();

    store.finishLoading();

    expect(store.getState()).toEqual({
      preferences: null,
      status: 'unauthenticated',
      user: null
    });
  });

  it('returns to loading and clears to unauthenticated', () => {
    const store = createAuthStore({
      preferences: completedPreferences,
      status: 'authenticated',
      user: { id: 'u1', email: 'user@example.com' }
    });

    store.startLoading();
    expect(store.getState().status).toBe('loading');

    store.clearUser();
    expect(store.getState()).toEqual({
      preferences: null,
      status: 'unauthenticated',
      user: null
    });
  });

  it('stores session preferences during bootstrap', async () => {
    const store = createAuthStore();
    const controller = createAuthBootstrapController(
      {
        me: async () => makeSession(onboardingRequiredPreferences)
      },
      store
    );

    await controller.bootstrap();

    expect(store.getState()).toEqual({
      preferences: onboardingRequiredPreferences,
      status: 'authenticated',
      user: { id: 'u1', email: 'user@example.com' }
    });
  });
});
