import { describe, expect, it, vi } from 'vitest';

import type { UserPreferences } from '../../types/api';
import { createAuthStore } from './store';

describe('auth store', () => {
  const user = { id: 'u1', email: 'user@example.com' };
  const preferences: UserPreferences = {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: false,
    onboardingCompleted: true
  };

  it('tracks loading and unauthenticated flows', () => {
    const store = createAuthStore();

    expect(store.getState()).toEqual({ status: 'idle', user: null, preferences: null });

    store.startLoading();
    expect(store.getState().status).toBe('loading');

    store.clearUser();
    expect(store.getState()).toEqual({ status: 'unauthenticated', user: null, preferences: null });
  });

  it('stores authenticated session and preserves preferences when updating user', () => {
    const store = createAuthStore();

    store.setAuthenticatedSession(user, preferences);

    expect(store.getState()).toEqual({ status: 'authenticated', user, preferences });

    const newUser = { id: 'u2', email: 'other@example.com' };
    store.setAuthenticatedUser(newUser);

    expect(store.getState().status).toBe('authenticated');
    expect(store.getState().user).toEqual(newUser);
    expect(store.getState().preferences).toEqual(preferences);
  });

  it('notifies subscribers when the state changes', () => {
    const store = createAuthStore();
    const listener = vi.fn();
    const unsubscribe = store.subscribe(listener);

    expect(listener).not.toHaveBeenCalled();

    store.startLoading();

    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    store.startLoading();

    expect(listener).toHaveBeenCalledTimes(1);
  });
});
