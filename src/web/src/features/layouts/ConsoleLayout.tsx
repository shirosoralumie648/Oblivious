import { Outlet } from 'react-router-dom';

export function ConsoleLayout() {
  return (
    <div>
      <h1>Console</h1>
      <Outlet />
    </div>
  );
}
