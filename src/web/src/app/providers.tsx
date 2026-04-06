import type { ReactNode } from 'react';

import { AppContextProvider, useAppContext } from './appContext';

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return <AppContextProvider>{children}</AppContextProvider>;
}

export { useAppContext };
