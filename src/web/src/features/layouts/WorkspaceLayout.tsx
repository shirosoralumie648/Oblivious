import { Link, Outlet } from 'react-router-dom';

export function WorkspaceLayout() {
  return (
    <div>
      <header>Workspace</header>
      <aside>
        <p>Conversations</p>
        <nav aria-label="Workspace navigation">
          <Link to="/chat">Chat</Link>
          <Link to="/knowledge">Knowledge</Link>
          <Link to="/solo">SOLO</Link>
          <Link to="/settings">Settings</Link>
          <Link to="/console">Console</Link>
        </nav>
      </aside>
      <section>
        <Outlet />
      </section>
      <aside>Capability Panel</aside>
    </div>
  );
}
