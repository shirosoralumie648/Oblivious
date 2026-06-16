import { useMemo, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import {
  RiDashboardLine,
  RiFileListLine,
  RiGitBranchLine,
  RiKey2Line,
  RiAlarmWarningLine,
  RiMenuFoldLine,
  RiMenuLine,
  RiMoneyDollarCircleLine,
  RiRouterLine,
  RiSearchLine,
  RiSettings3Line,
  RiShieldCheckLine,
  RiUserLine,
} from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';

type SidebarItem = {
  label: string;
  path: string;
  icon: React.ReactNode;
  keywords: string[];
};

type SidebarGroup = {
  label: string;
  items: SidebarItem[];
};

const sidebarGroups: SidebarGroup[] = [
  {
    label: 'Overview',
    items: [{ label: 'Dashboard', path: '/admin', icon: <RiDashboardLine />, keywords: ['home', 'stats'] }],
  },
  {
    label: 'Channels',
    items: [{ label: 'Channels', path: '/admin/channels', icon: <RiRouterLine />, keywords: ['provider', 'llm', 'api'] }],
  },
  {
    label: 'Models',
    items: [{ label: 'Models', path: '/admin/models', icon: <RiFileListLine />, keywords: ['model', 'provider', 'channel', 'cost', 'inventory'] }],
  },
  {
    label: 'Routes',
    items: [{ label: 'Model Routes', path: '/admin/routes', icon: <RiGitBranchLine />, keywords: ['routing', 'model'] }],
  },
  {
    label: 'Plans',
    items: [{ label: 'Plans', path: '/admin/plans', icon: <RiMoneyDollarCircleLine />, keywords: ['pricing', 'subscription', 'tier'] }],
  },
  {
    label: 'Billing',
    items: [{ label: 'Billing', path: '/admin/billing', icon: <RiMoneyDollarCircleLine />, keywords: ['billing', 'payment', 'invoice', 'refund', 'stripe', 'settlement', 'payout'] }],
  },
  {
    label: 'Logs',
    items: [
      { label: 'API Tokens', path: '/admin/api-tokens', icon: <RiKey2Line />, keywords: ['token', 'key', 'access', 'quota', 'relay'] },
      { label: 'Usage Logs', path: '/admin/usage-logs', icon: <RiFileListLine />, keywords: ['usage', 'request', 'logs', 'latency', 'cost', 'relay'] },
      { label: 'Alerts', path: '/admin/alerts', icon: <RiAlarmWarningLine />, keywords: ['alert', 'observability', 'incident', 'health'] },
    ],
  },
  {
    label: 'Settings',
    items: [{ label: 'Settings', path: '/admin/settings', icon: <RiSettings3Line />, keywords: ['settings', 'pricing', 'ratio', 'multiplier'] }],
  },
  {
    label: 'Users',
    items: [{ label: 'Users', path: '/admin/users', icon: <RiUserLine />, keywords: ['member', 'account'] }],
  },
  {
    label: 'Audit',
    items: [{ label: 'Audit Log', path: '/admin/audit-log', icon: <RiFileListLine />, keywords: ['history', 'activity'] }],
  },
  {
    label: 'Reviews',
    items: [{ label: 'Review Queue', path: '/admin/reviews', icon: <RiShieldCheckLine />, keywords: ['agent', 'approval', 'moderation'] }],
  },
];

function matchesItem(item: SidebarItem, query: string) {
  const normalized = query.trim().toLowerCase();
  if (!normalized) {
    return true;
  }
  return item.label.toLowerCase().includes(normalized) || item.keywords.some((keyword) => keyword.includes(normalized));
}

function defaultCollapsed() {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
    ? window.matchMedia('(max-width: 767px)').matches
    : false;
}

export function AdminSidebar() {
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState('');
  const [collapsed, setCollapsed] = useState(defaultCollapsed);

  const filteredGroups = useMemo(
    () =>
      sidebarGroups
        .map((group) => ({
          ...group,
          items: group.items.filter((item) => matchesItem(item, searchQuery)),
        }))
        .filter((group) => group.items.length > 0),
    [searchQuery]
  );

  const noResults = filteredGroups.length === 0;

  return (
    <aside
      aria-label="Admin navigation"
      className={cn('flex h-screen shrink-0 flex-col border-r border-sidebar-border bg-card transition-[width]', collapsed ? 'w-16' : 'w-64')}
      data-gsap-item
    >
      <div className="flex min-h-16 items-center gap-2 border-b border-sidebar-border p-3" data-gsap-item>
        {!collapsed ? (
          <div className="relative flex-1">
            <RiSearchLine className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <Input
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="Search modules..."
              className="min-h-[44px] rounded-lg pl-9"
            />
          </div>
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="min-h-[44px] min-w-[44px]"
          aria-label={collapsed ? 'Expand admin sidebar' : 'Collapse admin sidebar'}
          onClick={() => setCollapsed((value) => !value)}
        >
          {collapsed ? <RiMenuLine className="size-5" aria-hidden="true" /> : <RiMenuFoldLine className="size-5" aria-hidden="true" />}
        </Button>
      </div>

      <ScrollArea className="flex-1">
        <nav className="space-y-5 p-3">
          {filteredGroups.map((group) => (
            <section key={group.label} className="space-y-2" data-gsap-item>
              {!collapsed ? (
                <div className="space-y-2">
                  <Separator />
                  <p className="px-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">{group.label}</p>
                </div>
              ) : null}
              {group.items.map((item) => {
                const active = location.pathname === item.path;
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    title={collapsed ? item.label : undefined}
                    aria-label={collapsed ? item.label : undefined}
                    aria-current={active ? 'page' : undefined}
                    className={cn(
                      'flex min-h-[44px] items-center gap-3 rounded-lg border-l-2 border-transparent px-3 text-sm text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground',
                      collapsed && 'justify-center px-0',
                      active && 'border-sidebar-primary bg-sidebar-primary text-sidebar-primary-foreground hover:bg-sidebar-primary hover:text-sidebar-primary-foreground'
                    )}
                    data-gsap-item
                  >
                    <span className="[&_svg]:size-5">{item.icon}</span>
                    {!collapsed ? <span className="truncate">{item.label}</span> : null}
                  </Link>
                );
              })}
            </section>
          ))}
          {noResults ? <p className="px-2 py-8 text-center text-sm text-muted-foreground">No modules found</p> : null}
        </nav>
      </ScrollArea>
    </aside>
  );
}
