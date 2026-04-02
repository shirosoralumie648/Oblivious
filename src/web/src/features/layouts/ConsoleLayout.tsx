import { NavLink, Outlet } from 'react-router-dom';

import { useAppContext } from '../../app/providers';

export function ConsoleLayout() {
  const { authState } = useAppContext();

  return (
    <div>
      <header>
        <div>
          <h1>Console</h1>
          {authState.user ? <p>{authState.user.email}</p> : null}
        </div>
        <NavLink to="/chat">Back to workspace</NavLink>
      </header>
      <div>
        <aside>
          <nav aria-label="Console navigation">
            <ul>
              <li>
                <NavLink end to="/console">
                  Overview
                </NavLink>
              </li>
              <li>
                <NavLink to="/console/models">Models</NavLink>
              </li>
              <li>
                <NavLink to="/console/usage">Usage</NavLink>
              </li>
              <li>
                <NavLink to="/console/billing">Billing</NavLink>
              </li>
              <li>
                <NavLink to="/console/access">Access</NavLink>
              </li>
            </ul>
          </nav>
        </aside>
        <main>
          <Outlet />
        </main>
        <aside>
          <h2>Current session</h2>
          <p>Default mode: {authState.preferences?.defaultMode ?? 'chat'}</p>
          <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
          <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
        </aside>
      </div>
    </div>
  );
}
