import { Link, Outlet } from 'react-router-dom';

import { useAppContext } from '../../app/providers';

export function ConsoleLayout() {
  const { authState } = useAppContext();

  return (
    <div>
      <h1>Console</h1>
      <p>{authState.user?.email ?? 'anonymous'}</p>
      <p>{`Default mode: ${authState.preferences?.defaultMode ?? 'chat'}`}</p>
      <nav aria-label="Console navigation">
        <Link to="/console">Overview</Link>
        <Link to="/console/models">Models</Link>
        <Link to="/console/usage">Usage</Link>
        <Link to="/console/billing">Billing</Link>
        <Link to="/console/access">Access</Link>
      </nav>
      <Outlet />
    </div>
  );
}
