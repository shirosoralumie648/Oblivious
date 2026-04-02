import type { SessionResponse } from '../../types/api';

import type { AuthApi } from './api';
import { createAuthStore, type AuthStore } from './store';

export interface AuthBootstrapController {
  bootstrap: () => Promise<void>;
  getStore: () => AuthStore;
}

export function createAuthBootstrapController(
  authApi: Pick<AuthApi, 'me'>,
  store: AuthStore = createAuthStore()
): AuthBootstrapController {
  const applySession = (session: SessionResponse) => {
    store.setAuthenticatedSession(session.user, session.preferences);
  };

  return {
    bootstrap: async () => {
      store.startLoading();

      try {
        const session = await authApi.me();

        applySession(session);
      } catch {
        store.clearUser();
      }
    },
    getStore: () => store
  };
}
