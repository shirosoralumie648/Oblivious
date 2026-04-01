import { RouterProvider } from 'react-router-dom';

import { AppProviders } from './providers';
import { createAppRouter } from './router';

export function App() {
  const router = createAppRouter();

  return (
    <AppProviders>
      <RouterProvider router={router} />
    </AppProviders>
  );
}
