import { useMemo, type ReactNode } from 'react';
import {
  RiAlarmWarningLine,
  RiBarChartLine,
  RiFileListLine,
  RiGitBranchLine,
  RiMoneyDollarCircleLine,
  RiRobot2Line,
  RiRouterLine,
  RiShieldCheckLine,
  RiUserLine,
} from '@remixicon/react';
import useSWR from 'swr';
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip, LineChart, Line, XAxis, YAxis, CartesianGrid } from 'recharts';

import { createAdminApi } from '../../features/admin/api';
import { EmptyState } from '../../components/shared/EmptyState';
import { MetricCard } from '../../components/shared/MetricCard';
import { StatChart } from '../../components/shared/StatChart';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card';
import { createHttpClient } from '../../services/http/client';
import type { AdminStats } from '../../types/admin';

type ChartPoint = {
  label: string;
  value: number;
};

type CommercialOperation = {
  label: string;
  href: string;
  description: string;
  icon: ReactNode;
};

const commercialOperations: CommercialOperation[] = [
  {
    label: 'Channels',
    href: '/admin/channels',
    description: 'Relay provider health and failover inputs.',
    icon: <RiRouterLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Routes',
    href: '/admin/routes',
    description: 'Model routing policy and channel weights.',
    icon: <RiGitBranchLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Plans',
    href: '/admin/plans',
    description: 'Commercial package, quota, and access rules.',
    icon: <RiMoneyDollarCircleLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Billing',
    href: '/admin/billing',
    description: 'Sessions, payments, invoices, refunds, settlements, and payouts.',
    icon: <RiMoneyDollarCircleLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Users',
    href: '/admin/users',
    description: 'Accounts, roles, status, and package assignment.',
    icon: <RiUserLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Audit Log',
    href: '/admin/audit-log',
    description: 'Administrative and commercial governance events.',
    icon: <RiFileListLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Alerts',
    href: '/admin/alerts',
    description: 'Observability alert state and incident review.',
    icon: <RiAlarmWarningLine className="size-4" aria-hidden="true" />,
  },
  {
    label: 'Review Queue',
    href: '/admin/reviews',
    description: 'Marketplace approval, rejection, and governance workflow.',
    icon: <RiShieldCheckLine className="size-4" aria-hidden="true" />,
  },
];

function buildApiCallsData(stats: AdminStats): ChartPoint[] {
  return [
    { label: 'Calls', value: stats.apiCalls24h },
    { label: 'Chats', value: stats.conversations },
    { label: 'Tasks', value: stats.tasks },
    { label: 'Agents', value: stats.activeAgents },
  ];
}

function buildUptimeData(stats: AdminStats): ChartPoint[] {
  const offline = Math.max(stats.channelsTotal - stats.channelsOnline, 0);
  return [
    { label: 'Online', value: stats.channelsOnline },
    { label: 'Offline', value: offline },
  ];
}

function hasNoActivity(stats: AdminStats) {
  return stats.channelsTotal === 0 && stats.users.totalUsers === 0 && stats.apiCalls24h === 0 && stats.activeAgents === 0;
}

const COLORS = ['hsl(var(--primary))', 'hsl(var(--chart-2))', 'hsl(var(--chart-3))', 'hsl(var(--chart-4))', 'hsl(var(--chart-5))'];

export function AdminHomePage() {
  const api = useMemo(() => createAdminApi(createHttpClient()), []);
  const { data: stats, error, isLoading, mutate } = useSWR('/api/v1/admin/stats', () => api.getStats());

  const apiCallsData = useMemo(() => (stats ? buildApiCallsData(stats) : []), [stats]);
  const uptimeData = useMemo(() => (stats ? buildUptimeData(stats) : []), [stats]);
  const dailyTrendData = useMemo(() => stats?.dailyStats?.map(d => ({ label: d.date, value: d.calls })) || [], [stats]);
  const modelData = useMemo(() => stats?.modelBreakdown?.map(m => ({ name: m.model, value: m.count })) || [], [stats]);

  if (isLoading) {
    return (
      <div aria-busy="true" className="space-y-8">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Dashboard</h1>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }, (_, index) => (
            <MetricCard key={index} label="Loading" value={0} loading />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {Array.from({ length: 4 }, (_, index) => (
            <StatChart key={index} title="Loading" data={[]} loading />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <EmptyState
        title="Something went wrong while loading this data. Please try again or contact support if the issue persists."
        description={error instanceof Error ? error.message : 'Unable to load dashboard.'}
        action={{ label: 'Try Again', onClick: () => mutate() }}
      />
    );
  }

  if (!stats || hasNoActivity(stats)) {
    return (
      <EmptyState
        title="No activity yet -- System metrics will appear here once users start interacting."
        action={{ label: 'Refresh', onClick: () => mutate() }}
      />
    );
  }

  const onlinePercent = stats.channelsTotal > 0 ? Math.round((stats.channelsOnline / stats.channelsTotal) * 100) : 0;

  return (
    <div className="space-y-8">
      <div className="space-y-2">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Dashboard</h1>
        <p className="text-sm text-muted-foreground">System health, usage, and marketplace operations.</p>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Channels"
          value={stats.channelsTotal}
          trend={stats.channelsTotal > 0 ? { direction: 'up', value: `${onlinePercent}% online` } : undefined}
          icon={<RiRouterLine className="size-5" aria-hidden="true" />}
        />
        <MetricCard
          label="Total Users"
          value={stats.users.totalUsers}
          trend={{ direction: 'up', value: `+${stats.users.newUsersWeek} this week` }}
          icon={<RiUserLine className="size-5" aria-hidden="true" />}
        />
        <MetricCard
          label="API Calls (24h)"
          value={stats.apiCalls24h}
          format="number"
          icon={<RiBarChartLine className="size-5" aria-hidden="true" />}
        />
        <MetricCard
          label="Active Agents"
          value={stats.activeAgents}
          icon={<RiRobot2Line className="size-5" aria-hidden="true" />}
        />
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <StatChart title="API Call Volume (7 days)" data={apiCallsData} type="bar" />
        <StatChart title="Channel Uptime" data={uptimeData} type="bar" />
        <Card>
          <CardHeader>
            <CardTitle>API 调用趋势（最近 7 天）</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={200}>
              <LineChart data={dailyTrendData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="label" />
                <YAxis />
                <Tooltip />
                <Line type="monotone" dataKey="value" stroke="hsl(var(--primary))" strokeWidth={2} />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>模型使用占比</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={200}>
              <PieChart>
                <Pie data={modelData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={80} label>
                  {modelData.map((_, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      <section className="space-y-4" aria-labelledby="commercial-operations-heading">
        <div className="space-y-1">
          <h2 id="commercial-operations-heading" className="font-heading text-xl font-semibold text-foreground">Commercial operations</h2>
          <p className="text-sm text-muted-foreground">Routed modules required to operate Relay, billing, users, audit, and Marketplace review.</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          {commercialOperations.map((operation) => (
            <a
              key={operation.href}
              href={operation.href}
              className="flex min-h-[92px] gap-3 rounded-lg border border-border bg-card p-4 text-left transition-colors hover:border-primary/50 hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground">{operation.icon}</span>
              <span className="min-w-0 space-y-1">
                <span className="block text-sm font-medium text-foreground">{operation.label}</span>
                <span className="block text-xs leading-5 text-muted-foreground">{operation.description}</span>
              </span>
            </a>
          ))}
        </div>
      </section>
    </div>
  );
}
