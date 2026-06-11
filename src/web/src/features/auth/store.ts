import { create } from 'zustand';
import type { ApiUser, UserPreferences } from '../../types/api';

export type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'unauthenticated';

export type AuthState = {
  status: AuthStatus;
  user: ApiUser | null;
  preferences: UserPreferences | null;
};

export type AuthStore = {
  getState: () => AuthState;
  subscribe: (listener: () => void) => () => void;
  useStore: () => AuthState;
  startLoading: () => void;
  setAuthenticatedSession: (user: ApiUser, preferences: UserPreferences) => void;
  setAuthenticatedUser: (user: ApiUser) => void;
  clearUser: () => void;
};

export function createAuthStore(
  initialState: AuthState = { status: 'idle', user: null, preferences: null }
): AuthStore {
  const useStore = create<AuthState>(() => initialState);

  return {
    getState: useStore.getState,
    subscribe: useStore.subscribe,
    useStore,
    startLoading: () => {
      useStore.setState((state) => ({ ...state, status: 'loading' }));
    },
    setAuthenticatedSession: (user, preferences) => {
      useStore.setState({ status: 'authenticated', user, preferences });
    },
    setAuthenticatedUser: (user) => {
      useStore.setState((state) => ({ status: 'authenticated', user, preferences: state.preferences }));
    },
    clearUser: () => {
      useStore.setState({ status: 'unauthenticated', user: null, preferences: null });
    }
  };
}
