import type { ReactNode } from 'react';
import { createContext, useContext, useEffect, useMemo, useState } from 'react';

import { createAuthApi } from '../features/auth/api';
import { createAuthBootstrapController } from '../features/auth/useAuthBootstrap';
import { createAuthStore, type AuthState, type AuthStore } from '../features/auth/store';
import { createHttpClient } from '../services/http/client';
import type { UserPreferences } from '../types/api';

interface AppContextValue {
  authState: AuthState;
  authStore: AuthStore;
  bootstrapAuth: () => Promise<void>;
  refreshAuthState: () => void;
  updatePreferences: (preferences: UserPreferences) => Promise<UserPreferences>;
}

const AppContext = createContext<AppContextValue | null>(null);

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  const authStore = useMemo(() => createAuthStore(), []);
  const authApi = useMemo(() => createAuthApi(createHttpClient()), []);
  const bootstrapController = useMemo(() => createAuthBootstrapController(authApi, authStore), [authApi, authStore]);
  const [authState, setAuthState] = useState(() => authStore.getState());

  const refreshAuthState = () => {
    setAuthState(authStore.getState());
  };

  const bootstrapAuth = async () => {
    await bootstrapController.bootstrap();
    refreshAuthState();
  };

  const updatePreferences = async (preferences: UserPreferences) => {
    const nextPreferences = await authApi.updatePreferences(preferences);
    authStore.updatePreferences(nextPreferences);
    refreshAuthState();
    return nextPreferences;
  };

  useEffect(() => {
    refreshAuthState();
  }, [authStore]);

  const value = useMemo(
    () => ({
      authState,
      authStore,
      bootstrapAuth,
      refreshAuthState,
      updatePreferences
    }),
    [authState, authStore]
  );

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useAppContext() {
  const context = useContext(AppContext);

  if (!context) {
    throw new Error('useAppContext must be used within AppProviders');
  }

  return context;
}
