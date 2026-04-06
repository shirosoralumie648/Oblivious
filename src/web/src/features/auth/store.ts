import type { ApiUser, UserPreferences } from '../../types/api';

export type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'unauthenticated';

export type AuthState = {
  status: AuthStatus;
  user: ApiUser | null;
  preferences: UserPreferences | null;
};

type Listener = () => void;

export type AuthStore = {
  getState: () => AuthState;
  subscribe: (listener: Listener) => () => void;
  startLoading: () => void;
  setAuthenticatedSession: (user: ApiUser, preferences: UserPreferences) => void;
  setAuthenticatedUser: (user: ApiUser) => void;
  clearUser: () => void;
};

export function createAuthStore(
  initialState: AuthState = { status: 'idle', user: null, preferences: null }
): AuthStore {
  let state = initialState;
  const listeners = new Set<Listener>();

  const notify = () => {
    for (const listener of listeners) {
      listener();
    }
  };

  const setState = (nextState: AuthState) => {
    state = nextState;
    notify();
  };

  return {
    getState: () => state,
    subscribe: (listener) => {
      listeners.add(listener);

      return () => {
        listeners.delete(listener);
      };
    },
    startLoading: () => {
      setState({
        ...state,
        status: 'loading'
      });
    },
    setAuthenticatedSession: (user, preferences) => {
      setState({
        status: 'authenticated',
        user,
        preferences
      });
    },
    setAuthenticatedUser: (user) => {
      setState({
        status: 'authenticated',
        user,
        preferences: state.preferences
      });
    },
    clearUser: () => {
      setState({
        status: 'unauthenticated',
        user: null,
        preferences: null
      });
    }
  };
}
