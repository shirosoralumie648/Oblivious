import { useMemo } from 'react';
import { RouterProvider } from 'react-router-dom';

import { AppProviders } from './providers';
import { createAppRouter } from './router';
import { GsapMotionProvider } from '../features/motion/GsapMotionProvider';

export function App() {
  const router = useMemo(() => createAppRouter(), []);

  return (
    <AppProviders>
      <div className="theme min-h-screen">
        <GsapMotionProvider>
          <RouterProvider  router={router} />
        </GsapMotionProvider>
      </div>
    </AppProviders>
  );
}
