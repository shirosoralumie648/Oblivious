import { useEffect, useMemo } from 'react';
import { RouterProvider } from 'react-router-dom';

import { AppProviders, useAppContext } from './providers';
import { createAppRouter } from './router';

function AppRouter() {
  const { bootstrapAuth } = useAppContext();
  const router = useMemo(() => createAppRouter(), []);

  useEffect(() => {
    void bootstrapAuth();
  }, [bootstrapAuth]);

  return <RouterProvider router={router} />;
}

export function App() {
  return (
    <AppProviders>
      <AppRouter />
    </AppProviders>
  );
}
