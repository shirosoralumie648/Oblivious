import { Link, NavLink, Outlet } from 'react-router-dom';
import { RiArrowLeftLine, RiBillLine, RiKey2Line, RiLineChartLine, RiNotification3Line, RiRobot2Line, RiSettings3Line } from '@remixicon/react';

import { useAppContext } from '../../app/providers';

const consoleLinks = [
  { label: 'Overview', to: '/console', icon: <RiLineChartLine className="size-4" aria-hidden="true" /> },
  { label: 'Billing', to: '/console/billing', icon: <RiBillLine className="size-4" aria-hidden="true" /> },
  { label: 'Usage', to: '/console/usage', icon: <RiLineChartLine className="size-4" aria-hidden="true" /> },
  { label: 'Models', to: '/console/models', icon: <RiRobot2Line className="size-4" aria-hidden="true" /> },
  { label: 'Access', to: '/console/access', icon: <RiKey2Line className="size-4" aria-hidden="true" /> },
  { label: 'Notifications', to: '/console/notifications', icon: <RiNotification3Line className="size-4" aria-hidden="true" /> },
];

export function ConsoleLayout() {
  const { authState } = useAppContext();
  const consoleLinkClassName = ({ isActive }: { isActive: boolean }) =>
    [
      'inline-flex min-h-[44px] shrink-0 items-center gap-2 rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm transition hover:border-[#1a614f]/40 hover:bg-[#e9f2ee]',
      isActive ? 'border-[#1a614f] bg-[#dcece6] font-semibold text-[#17483d]' : ''
    ].join(' ');

  return (
    <div className="min-h-screen bg-[#f4f3ee] text-[#181611]" data-gsap-scope="console">
      <header className="border-b border-[#d7d2c4] bg-[#15130f] px-5 py-5 text-[#f7f4ea] lg:px-8" data-gsap-item>
        <div className="mx-auto flex max-w-7xl flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
          <div data-gsap-item>
            <h2 className="font-heading text-3xl font-semibold">Console</h2>
            <p className="mt-2 text-sm text-[#bdb5a6]">Current workspace scope</p>
            <div className="mt-3 flex flex-wrap gap-2 text-sm">
              <span className="rounded-lg border border-white/10 bg-white/10 px-3 py-2" data-gsap-item>{authState.user?.email ?? 'anonymous'}</span>
              <span className="rounded-lg border border-white/10 bg-white/10 px-3 py-2" data-gsap-item>{`Default mode: ${authState.preferences?.defaultMode ?? 'chat'}`}</span>
            </div>
          </div>
          <nav aria-label="Console shortcuts" className="flex flex-wrap gap-2" data-gsap-item>
            <Link className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-white/15 px-3 text-sm transition hover:bg-white/10" data-gsap-magnetic to="/settings">
              <RiSettings3Line className="size-4" aria-hidden="true" />
              Workspace settings
            </Link>
            <Link className="inline-flex min-h-[44px] items-center gap-2 rounded-lg bg-[#f0c36a] px-3 text-sm font-semibold text-[#17110a] transition hover:bg-[#ffd98a]" data-gsap-magnetic to="/chat">
              <RiArrowLeftLine className="size-4" aria-hidden="true" />
              Return to workspace
            </Link>
          </nav>
        </div>
      </header>
      <div className="mx-auto grid max-w-7xl gap-6 px-5 py-6 lg:grid-cols-[220px_minmax(0,1fr)] lg:px-8" data-gsap-item>
        <nav aria-label="Console navigation" className="flex gap-2 overflow-x-auto lg:flex-col lg:overflow-visible" data-gsap-item>
          {consoleLinks.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/console'}
              className={consoleLinkClassName}
              data-gsap-item
            >
              {item.icon}
              {item.label}
            </NavLink>
          ))}
        </nav>
        <main className="console-canvas min-w-0 rounded-lg border border-[#d7d2c4] bg-white p-5 shadow-sm" data-gsap-item>
          <Outlet />
        </main>
      </div>
    </div>
  );
}
