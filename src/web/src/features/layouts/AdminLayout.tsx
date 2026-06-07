import { Outlet } from 'react-router-dom';
import { AdminSidebar } from './AdminSidebar';

export function AdminLayout() {
  return (
    <div className="flex h-screen bg-background" data-gsap-scope="admin">
      <AdminSidebar />
      <main className="flex-1 overflow-auto p-8" data-gsap-item>
        <Outlet />
      </main>
    </div>
  );
}
