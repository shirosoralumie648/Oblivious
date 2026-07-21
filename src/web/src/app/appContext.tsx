import { createContext, useContext, useEffect, useRef, type ReactNode } from 'react';

import { createAuthApi, type AuthApi } from '../features/auth/api';
import { createAuthStore, type AuthState } from '../features/auth/store';
import { createAuthBootstrapController } from '../features/auth/useAuthBootstrap';
import { updatePreferencesOperationContract } from '@/generated/operation-contracts.generated';
import {
  createHttpClient,
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  type OperationTransportContract
} from '../services/http/client';
import type { UserPreferences } from '../types/api';

export type UpdatePreferencesRequest = (preferences: UserPreferences) => Promise<UserPreferences>;

export type AppContextValue = {
  authState: AuthState;
  bootstrapAuth: () => Promise<void>;
  updatePreferences: UpdatePreferencesRequest;
};

type AppContextProviderProps = {
  children: ReactNode;
  authApi?: Pick<AuthApi, 'me'>;
  updatePreferencesRequest?: UpdatePreferencesRequest;
};

const AppContext = createContext<AppContextValue | null>(null);
const fallbackAppContextValue: AppContextValue = {
  authState: {
    status: 'idle',
    user: null,
    preferences: null
  },
  bootstrapAuth: async () => {},
  updatePreferences: async (preferences) => preferences
};

const updatePreferencesTransport: OperationTransportContract<UserPreferences> = {
  operation: updatePreferencesOperationContract,
  requestEncoder: jsonRequestEncoder(updatePreferencesOperationContract),
  responseDecoder: jsonEnvelopeDecoder<UserPreferences>(updatePreferencesOperationContract, 200)
};

export function AppContextProvider({
  children,
  authApi = createAuthApi(createHttpClient()),
  updatePreferencesRequest = (preferences) =>
    createHttpClient().put<UserPreferences>(
      '/api/v1/app/me/preferences',
      preferences,
      undefined,
      updatePreferencesTransport
    )
}: AppContextProviderProps) {
  const storeRef = useRef(createAuthStore({ status: 'loading', user: null, preferences: null }));
  const bootstrapControllerRef = useRef(createAuthBootstrapController(authApi, storeRef.current));

  const authState = storeRef.current.useStore();

  useEffect(() => {
    void bootstrapControllerRef.current.bootstrap();
  }, []);

  const updatePreferences = async (preferences: UserPreferences) => {
    const nextPreferences = await updatePreferencesRequest(preferences);
    const currentState = storeRef.current.getState();

    if (currentState.user !== null) {
      storeRef.current.setAuthenticatedSession(currentState.user, nextPreferences);
    }

    return nextPreferences;
  };

  return (
    <AppContext.Provider
      value={{
        authState,
        bootstrapAuth: bootstrapControllerRef.current.bootstrap,
        updatePreferences
      }}
    >
      {children}
    </AppContext.Provider>
  );
}

export function useAppContext() {
  const context = useContext(AppContext);
  return context ?? fallbackAppContextValue;
}
