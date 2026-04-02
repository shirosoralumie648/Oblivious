import { NavLink, Outlet } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createAuthApi } from '../auth/api';
import { createHttpClient } from '../../services/http/client';

export function WorkspaceLayout() {
  const { authState, authStore, refreshAuthState } = useAppContext();

  const handleLogout = async () => {
    const authApi = createAuthApi(createHttpClient());
    await authApi.logout();
    authStore.clearUser();
    refreshAuthState();
  };

  return (
    <div>
      <header>
        <div>
          <strong>Workspace</strong>
          {authState.user ? <span>{authState.user.email}</span> : null}
        </div>
        <button onClick={() => void handleLogout()} type="button">
          Logout
        </button>
      </header>
      <div>
        <aside>
          <h2>Navigate</h2>
          <nav>
            <ul>
              <li>
                <NavLink to="/chat">Chat</NavLink>
              </li>
              <li>
                <NavLink to="/settings">Settings</NavLink>
              </li>
            </ul>
          </nav>
        </aside>
        <section>
          <Outlet />
        </section>
        <aside>
          <h2>Session settings</h2>
          <p>Default mode: {authState.preferences?.defaultMode ?? 'chat'}</p>
          <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
          <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
        </aside>
      </div>
    </div>
  );
}
