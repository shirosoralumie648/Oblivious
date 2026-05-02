import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiBarChartLine, RiRobot2Line, RiRouterLine, RiUserLine } from '@remixicon/react';

import { createAdminApi } from '../../features/admin/api';
import { EmptyState } from '../../components/shared/EmptyState';
import { MetricCard } from '../../components/shared/MetricCard';
import { StatChart } from '../../components/shared/StatChart';
import { createHttpClient } from '../../services/http/client';
import type { AdminStats } from '../../types/admin';

type ChartPoint = {
  label: string;
  value: number;
};

type State = {
  stats: AdminStats | null;
  apiCallsData: ChartPoint[];
  uptimeData: ChartPoint[];
  loading: boolean;
  error: string | null;
};

type Action =
  | { type: 'LOADING' }
  | { type: 'SUCCESS'; stats: AdminStats; apiCallsData: ChartPoint[]; uptimeData: ChartPoint[] }
  | { type: 'ERROR'; error: string };

const initialState: State = {
  stats: null,
  apiCallsData: [],
  uptimeData: [],
  loading: true,
  error: null,
};

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'LOADING':
      return { ...state, loading: true, error: null };
    case 'SUCCESS':
      return {
        stats: action.stats,
        apiCallsData: action.apiCallsData,
        uptimeData: action.uptimeData,
        loading: false,
        error: null,
      };
    case 'ERROR':
      return { ...state, loading: false, error: action.error };
    default:
      return state;
  }
}

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

export function AdminHomePage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadStats = useCallback(async () => {
    dispatch({ type: 'LOADING' });

    try {
      const stats = await api.getStats();
      dispatch({
        type: 'SUCCESS',
        stats,
        apiCallsData: buildApiCallsData(stats),
        uptimeData: buildUptimeData(stats),
      });
    } catch (error) {
      dispatch({
        type: 'ERROR',
        error: error instanceof Error ? error.message : 'Unable to load dashboard.',
      });
    }
  }, [api]);

  useEffect(() => {
    void loadStats();
  }, [loadStats]);

  if (state.loading) {
    return (
      <div aria-busy="true" className="space-y-8">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Dashboard</h1>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }, (_, index) => (
            <MetricCard key={index} label="Loading" value={0} loading />
          ))}
        </div>
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <StatChart title="API Call Volume (7 days)" data={[]} loading />
          <StatChart title="Channel Uptime" data={[]} loading />
        </div>
      </div>
    );
  }

  if (state.error) {
    return (
      <EmptyState
        title="Something went wrong while loading this data. Please try again or contact support if the issue persists."
        description={state.error}
        action={{ label: 'Try Again', onClick: loadStats }}
      />
    );
  }

  if (!state.stats || hasNoActivity(state.stats)) {
    return (
      <EmptyState
        title="No activity yet -- System metrics will appear here once users start interacting."
        action={{ label: 'Refresh', onClick: loadStats }}
      />
    );
  }

  const onlinePercent = state.stats.channelsTotal > 0 ? Math.round((state.stats.channelsOnline / state.stats.channelsTotal) * 100) : 0;

  return (
    <div className="space-y-8">
      <div className="space-y-2">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Dashboard</h1>
        <p className="text-sm text-muted-foreground">System health, usage, and marketplace operations.</p>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          label="Channels"
          value={state.stats.channelsTotal}
          trend={state.stats.channelsTotal > 0 ? { direction: 'up', value: `${onlinePercent}% online` } : undefined}
          icon={<RiRouterLine className="size-5" aria-hidden="true" />}
        />
        <MetricCard
          label="Total Users"
          value={state.stats.users.totalUsers}
          trend={{ direction: 'up', value: `+${state.stats.users.newUsersWeek} this week` }}
          icon={<RiUserLine className="size-5" aria-hidden="true" />}
        />
        <MetricCard
          label="API Calls (24h)"
          value={state.stats.apiCalls24h}
          format="number"
          icon={<RiBarChartLine className="size-5" aria-hidden="true" />}
        />
        <MetricCard
          label="Active Agents"
          value={state.stats.activeAgents}
          icon={<RiRobot2Line className="size-5" aria-hidden="true" />}
        />
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <StatChart title="API Call Volume (7 days)" data={state.apiCallsData} type="bar" />
        <StatChart title="Channel Uptime" data={state.uptimeData} type="bar" />
      </div>
    </div>
  );
}
