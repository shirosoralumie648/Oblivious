import { useMemo } from 'react';
import { RouterProvider } from 'react-router-dom';

import { AppProviders } from './providers';
import { createAppRouter } from './router';
import { routerFuture } from './routerFuture';

export function App() {
  const router = useMemo(() => createAppRouter(), []);

  return (
    <AppProviders>
      <RouterProvider future={routerFuture} router={router} />
    </AppProviders>
  );
}
