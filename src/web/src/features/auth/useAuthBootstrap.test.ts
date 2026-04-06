import { describe, expect, it, vi } from 'vitest';

import type { SessionResponse, UserPreferences } from '../../types/api';
import { createAuthStore } from './store';
import { createAuthBootstrapController } from './useAuthBootstrap';

describe('createAuthBootstrapController', () => {
  it('bootstraps authenticated session into store', async () => {
    const preferences: UserPreferences = {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    };
    const session: SessionResponse = {
      onboardingCompleted: true,
      preferences,
      session: {
        expiresAt: '2026-04-06T00:00:00Z',
        id: 'session_1'
      },
      user: {
        email: 'user@example.com',
        id: 'u1'
      },
      workspace: {
        id: 'workspace_1'
      }
    };
    const authApi = {
      me: vi.fn(async () => session)
    };
    const store = createAuthStore();
    const controller = createAuthBootstrapController(authApi, store);

    await controller.bootstrap();

    expect(authApi.me).toHaveBeenCalledTimes(1);
    expect(store.getState()).toEqual({
      status: 'authenticated',
      user: session.user,
      preferences
    });
  });

  it('clears the user when bootstrap fails', async () => {
    const store = createAuthStore({
      status: 'authenticated',
      user: {
        email: 'user@example.com',
        id: 'u1'
      },
      preferences: {
        defaultMode: 'chat',
        modelStrategy: 'balanced',
        networkEnabledHint: false,
        onboardingCompleted: true
      }
    });
    const controller = createAuthBootstrapController(
      {
        me: vi.fn(async () => {
          throw new Error('unauthorized');
        })
      },
      store
    );

    await controller.bootstrap();

    expect(store.getState()).toEqual({
      status: 'unauthenticated',
      user: null,
      preferences: null
    });
  });
});
