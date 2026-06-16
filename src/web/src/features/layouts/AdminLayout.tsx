import { Outlet } from 'react-router-dom';
import { AdminSidebar } from './AdminSidebar';

export function AdminLayout() {
  return (
    <div className="flex h-screen bg-background" data-gsap-scope="admin">
      <AdminSidebar />
      <main className="min-w-0 flex-1 overflow-auto p-4 md:p-8" data-gsap-item>
        <Outlet />
      </main>
    </div>
  );
}
