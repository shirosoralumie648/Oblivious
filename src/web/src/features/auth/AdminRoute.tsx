import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useAppContext } from '../../app/providers';

export function AdminRoute() {
  const { authState } = useAppContext();
  const location = useLocation();

  if (!authState.user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (authState.user.role !== 'admin') {
    return <Navigate to="/" replace />;
  }

  return <Outlet />;
}
