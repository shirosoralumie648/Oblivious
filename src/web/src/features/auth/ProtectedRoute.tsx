import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { useAppContext } from '../../app/providers';

export function ProtectedRoute() {
  const location = useLocation();
  const { authState } = useAppContext();

  if (authState.status === 'idle') {
    return <Outlet />;
  }

  if (authState.status === 'loading') {
    return <main>Loading session…</main>;
  }

  if (authState.status === 'unauthenticated') {
    const redirectPath = `${location.pathname}${location.search}${location.hash}`;

    return <Navigate replace state={{ from: redirectPath }} to="/login" />;
  }

  if (!authState.preferences?.onboardingCompleted && location.pathname !== '/onboarding') {
    return <Navigate replace to="/onboarding" />;
  }

  if (authState.preferences?.onboardingCompleted && location.pathname === '/onboarding') {
    return <Navigate replace to="/chat" />;
  }

  return <Outlet />;
}
