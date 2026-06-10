import { Link, NavLink, Outlet } from 'react-router-dom';
import {
  RiBillLine,
  RiBrainLine,
  RiCalendarScheduleLine,
  RiChat3Line,
  RiDatabase2Line,
  RiMegaphoneLine,
  RiFlowChart,
  RiPlugLine,
  RiRobot2Line,
  RiSettings3Line,
  RiStore2Line,
  RiTerminalBoxLine
} from '@remixicon/react';

const workspaceLinks = [
  { label: 'Chat', to: '/chat', icon: <RiChat3Line className="size-4" aria-hidden="true" /> },
  { label: 'Knowledge', to: '/knowledge', icon: <RiBrainLine className="size-4" aria-hidden="true" /> },
  { label: 'Agents', to: '/agents', icon: <RiRobot2Line className="size-4" aria-hidden="true" /> },
  { label: 'MCP Servers', to: '/mcp-servers', icon: <RiPlugLine className="size-4" aria-hidden="true" /> },
  { label: 'Agent Memories', to: '/memories', icon: <RiDatabase2Line className="size-4" aria-hidden="true" /> },
  { label: 'SOLO', to: '/solo', icon: <RiTerminalBoxLine className="size-4" aria-hidden="true" /> },
  { label: 'Workflows', to: '/workflows', icon: <RiFlowChart className="size-4" aria-hidden="true" /> },
  { label: 'Scheduled Tasks', to: '/scheduled-tasks', icon: <RiCalendarScheduleLine className="size-4" aria-hidden="true" /> },
  { label: 'Publishing', to: '/publishing', icon: <RiMegaphoneLine className="size-4" aria-hidden="true" /> },
  { label: 'Settings', to: '/settings', icon: <RiSettings3Line className="size-4" aria-hidden="true" /> },
  { label: 'Console', to: '/console', icon: <RiBillLine className="size-4" aria-hidden="true" /> },
];

export function WorkspaceLayout() {
  const workspaceLinkClassName = ({ isActive }: { isActive: boolean }) =>
    [
      'flex min-h-[44px] items-center gap-3 rounded-lg px-3 text-sm transition hover:bg-white/10 hover:text-white',
      isActive ? 'bg-white/15 text-white' : 'text-[#d8d3c8]'
    ].join(' ');

  return (
    <div className="min-h-screen bg-[#f4f3ee] text-[#181611] lg:grid lg:grid-cols-[244px_minmax(0,1fr)_280px]" data-gsap-scope="workspace">
      <aside className="border-b border-[#d7d2c4] bg-[#15130f] p-5 text-[#f7f4ea] lg:min-h-screen lg:border-b-0 lg:border-r" data-gsap-item>
        <header className="flex items-center justify-between gap-3" data-gsap-item>
          <div className="flex items-center gap-3">
            <span className="flex size-9 items-center justify-center rounded-lg border border-amber-300/30 bg-amber-300/10 text-amber-200">O</span>
            <span className="font-heading text-lg font-semibold">Workspace</span>
          </div>
        </header>
        <div className="mt-8 rounded-lg border border-white/10 bg-white/[0.04] p-4" data-gsap-item>
          <p className="text-sm font-semibold">Conversations</p>
          <p className="mt-2 text-xs leading-5 text-[#bdb5a6]">Relay-backed Chat, Knowledge, and SOLO share the same commercial context.</p>
        </div>
        <nav aria-label="Workspace navigation" className="mt-6 space-y-1">
          {workspaceLinks.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={workspaceLinkClassName}
              data-gsap-item
            >
              {item.icon}
              {item.label}
            </NavLink>
          ))}
        </nav>
        <NavLink
          to="/marketplace"
          className={({ isActive }) =>
            [
              'mt-6 flex min-h-[44px] items-center gap-3 rounded-lg border border-cyan-200/20 bg-cyan-200/10 px-3 text-sm text-cyan-100 transition hover:bg-cyan-200/15',
              isActive ? 'ring-2 ring-cyan-100/50' : ''
            ].join(' ')
          }
          data-gsap-item
          data-gsap-magnetic
        >
          <RiStore2Line className="size-4" aria-hidden="true" />
          Marketplace
        </NavLink>
      </aside>
      <main className="workspace-canvas min-h-screen overflow-auto p-5 lg:p-8" data-gsap-item>
        <Outlet />
      </main>
      <aside className="hidden border-l border-[#d7d2c4] bg-white/70 p-5 lg:block" data-gsap-item>
        <div className="sticky top-5 space-y-4">
          <div data-gsap-item>
            <p className="text-sm font-semibold">Capability Panel</p>
            <p className="mt-2 text-xs leading-5 text-[#625b4f]">Commercial state remains visible while moving between Chat, Knowledge, SOLO, Console, and Marketplace.</p>
          </div>
          <div className="rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-4" data-gsap-item>
            <p className="text-xs font-semibold uppercase text-[#6d6658]">Relay invariant</p>
            <p className="mt-2 text-sm leading-6">All AI requests route through Relay for quota, billing, audit, and monitoring.</p>
          </div>
          <div className="rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-4" data-gsap-item>
            <p className="text-xs font-semibold uppercase text-[#6d6658]">Journey surfaces</p>
            <p className="mt-2 text-sm leading-6">Draft preservation, source citations, approval gates, settlement copy, and recoverable action errors.</p>
          </div>
        </div>
      </aside>
    </div>
  );
}
