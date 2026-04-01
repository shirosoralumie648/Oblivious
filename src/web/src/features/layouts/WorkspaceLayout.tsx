import { Outlet } from 'react-router-dom';

export function WorkspaceLayout() {
  return (
    <div>
      <header>Workspace</header>
      <aside>Conversations</aside>
      <section>
        <Outlet />
      </section>
      <aside>Capability Panel</aside>
    </div>
  );
}
