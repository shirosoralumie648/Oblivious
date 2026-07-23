import type { ReactNode } from 'react';
import { SWRConfig } from 'swr';

import { AppContextProvider, useAppContext } from './appContext';
import { swrConfig } from '@/lib/swr';

type AppProvidersProps = {
  children: ReactNode;
};

export function AppProviders({ children }: AppProvidersProps) {
  return (
    <AppContextProvider>
      <SWRConfig value={swrConfig}>{children}</SWRConfig>
    </AppContextProvider>
  );
}

export { useAppContext };
