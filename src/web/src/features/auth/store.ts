import type { ApiUser, UserPreferences } from '../../types/api';

export type AuthStatus = 'authenticated' | 'loading' | 'unauthenticated';

export interface AuthState {
  preferences: UserPreferences | null;
  status: AuthStatus;
  user: ApiUser | null;
}

export interface AuthStore {
  clearUser: () => void;
  finishLoading: () => void;
  getState: () => AuthState;
  setAuthenticatedSession: (user: ApiUser, preferences: UserPreferences) => void;
  startLoading: () => void;
  updatePreferences: (preferences: UserPreferences) => void;
}

export function createAuthStore(
  initialState: AuthState = { preferences: null, status: 'loading', user: null }
): AuthStore {
  let state = initialState;

  return {
    clearUser: () => {
      state = {
        preferences: null,
        status: 'unauthenticated',
        user: null
      };
    },
    finishLoading: () => {
      state = {
        ...state,
        status: state.user ? 'authenticated' : 'unauthenticated'
      };
    },
    getState: () => state,
    setAuthenticatedSession: (user, preferences) => {
      state = {
        preferences,
        status: 'authenticated',
        user
      };
    },
    startLoading: () => {
      state = {
        ...state,
        status: 'loading'
      };
    },
    updatePreferences: (preferences) => {
      state = {
        ...state,
        preferences
      };
    }
  };
}
