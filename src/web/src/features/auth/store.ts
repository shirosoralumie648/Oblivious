import type { ApiUser } from '../../types/api';

export type AuthStatus = 'idle' | 'authenticated' | 'unauthenticated';

export type AuthState = {
  status: AuthStatus;
  user: ApiUser | null;
};

export type AuthStore = {
  getState: () => AuthState;
  setAuthenticatedUser: (user: ApiUser) => void;
  clearUser: () => void;
};

export function createAuthStore(initialState: AuthState = { status: 'idle', user: null }): AuthStore {
  let state = initialState;

  return {
    getState: () => state,
    setAuthenticatedUser: (user) => {
      state = {
        status: 'authenticated',
        user
      };
    },
    clearUser: () => {
      state = {
        status: 'unauthenticated',
        user: null
      };
    }
  };
}
