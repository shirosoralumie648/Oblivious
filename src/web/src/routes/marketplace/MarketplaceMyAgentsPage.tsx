import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { Link } from 'react-router-dom';
import { RiDeleteBinLine, RiExternalLinkLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { RatingStars } from '../../components/shared/RatingStars';
import { StatusBadge } from '../../components/shared/StatusBadge';
import {
  createMarketplaceApi,
  type AgentInstall,
  type MarketplaceAgent,
  type MarketplaceSettlementPreferences,
  type PublisherStats,
  type SettlementCycle,
} from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';
import type { StatusBadgeStatus } from '../../components/shared/StatusBadge';

type MyAgentsState = {
  myAgents: MarketplaceAgent[];
  installs: AgentInstall[];
  settlementPreferences: MarketplaceSettlementPreferences | null;
  publisherStats: PublisherStats | null;
  settlementCycleDraft: SettlementCycle;
  savingSettlementCycle: boolean;
  loading: boolean;
  error: string | null;
  actionError: { title: string; message?: string } | null;
  uninstallingID: string | null;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; myAgents: MarketplaceAgent[]; installs: AgentInstall[]; settlementPreferences: MarketplaceSettlementPreferences; publisherStats: PublisherStats }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_ACTION_ERROR'; title: string; message?: string }
  | { type: 'SET_UNINSTALLING'; installID: string | null }
  | { type: 'SET_SETTLEMENT_CYCLE_DRAFT'; cycle: SettlementCycle }
  | { type: 'SAVE_SETTLEMENT_CYCLE_START' }
  | { type: 'SAVE_SETTLEMENT_CYCLE_SUCCESS'; settlementPreferences: MarketplaceSettlementPreferences };

const initialState: MyAgentsState = {
  myAgents: [],
  installs: [],
  settlementPreferences: null,
  publisherStats: null,
  settlementCycleDraft: 'monthly',
  savingSettlementCycle: false,
  loading: true,
  error: null,
  actionError: null,
  uninstallingID: null,
};

function reducer(state: MyAgentsState, action: Action): MyAgentsState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null, actionError: null };
    case 'LOAD_SUCCESS':
      return {
        ...state,
        actionError: null,
        loading: false,
        error: null,
        myAgents: action.myAgents,
        installs: action.installs,
        settlementPreferences: action.settlementPreferences,
        publisherStats: action.publisherStats,
        settlementCycleDraft: action.settlementPreferences.cycle,
      };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_ACTION_ERROR':
      return { ...state, savingSettlementCycle: false, actionError: { title: action.title, message: action.message } };
    case 'SET_UNINSTALLING':
      return { ...state, actionError: action.installID ? null : state.actionError, uninstallingID: action.installID };
    case 'SET_SETTLEMENT_CYCLE_DRAFT':
      return { ...state, actionError: null, settlementCycleDraft: action.cycle };
    case 'SAVE_SETTLEMENT_CYCLE_START':
      return { ...state, actionError: null, savingSettlementCycle: true };
    case 'SAVE_SETTLEMENT_CYCLE_SUCCESS':
      return {
        ...state,
        actionError: null,
        savingSettlementCycle: false,
        settlementPreferences: action.settlementPreferences,
        settlementCycleDraft: action.settlementPreferences.cycle,
      };
    default:
      return state;
  }
}

function getErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }

  return fallback;
}

function publishedStatus(agent: MarketplaceAgent): { status: StatusBadgeStatus; label?: string } {
  if (agent.status === 'draft') {
    return { status: 'pending', label: 'Draft' };
  }
  if (agent.status === 'pending') {
    return { status: 'pending_review', label: 'Pending Review' };
  }
  if (agent.status === 'approved' || agent.status === 'rejected' || agent.status === 'pending_review') {
    return { status: agent.status };
  }
  return { status: 'pending', label: agent.status };
}

const settlementCycleLabels: Record<SettlementCycle, string> = {
  weekly: 'Weekly',
  monthly: 'Monthly',
  quarterly: 'Quarterly',
};

function formatUSD(amount: number | undefined) {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', maximumFractionDigits: 0 }).format(amount ?? 0);
}

function formatNumber(value: number | undefined) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value ?? 0);
}

export function MarketplaceMyAgentsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createMarketplaceApi(createHttpClient()), []);

  const loadAgents = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const [myAgents, installs, settlementPreferences, publisherStats] = await Promise.all([
        api.getMyAgents(),
        api.getInstalledAgents(),
        api.getSettlementPreferences(),
        api.getPublisherStats(),
      ]);
      dispatch({ type: 'LOAD_SUCCESS', myAgents, installs, settlementPreferences, publisherStats });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Unable to load agents.' });
    }
  }, [api]);

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  const handleUninstall = async (install: AgentInstall) => {
    dispatch({ type: 'SET_UNINSTALLING', installID: install.id });
    try {
      await api.uninstallAgent(install.id);
      await loadAgents();
    } catch (error) {
      dispatch({
        type: 'SET_ACTION_ERROR',
        title: 'Unable to uninstall agent.',
        message: getErrorMessage(error, 'Retry after pending marketplace settlement state clears.'),
      });
    } finally {
      dispatch({ type: 'SET_UNINSTALLING', installID: null });
    }
  };

  const handleSettlementCycleSave = async () => {
    dispatch({ type: 'SAVE_SETTLEMENT_CYCLE_START' });
    try {
      const settlementPreferences = await api.updateSettlementPreferences(state.settlementCycleDraft);
      dispatch({ type: 'SAVE_SETTLEMENT_CYCLE_SUCCESS', settlementPreferences });
    } catch (error) {
      dispatch({
        type: 'SET_ACTION_ERROR',
        title: 'Unable to update settlement cycle.',
        message: getErrorMessage(error, 'Choose weekly, monthly, or quarterly and try again.'),
      });
    }
  };

  const publishedColumns: DataTableColumn<MarketplaceAgent>[] = [
    { key: 'name', header: 'Agent' },
    {
      key: 'status',
      header: 'Status',
      render: (agent) => {
        const mappedStatus = publishedStatus(agent);
        return <StatusBadge status={mappedStatus.status} label={mappedStatus.label} />;
      },
    },
    { key: 'ratingAvg', header: 'Rating', render: (agent) => <RatingStars value={agent.ratingAvg ?? agent.rating ?? 0} readonly showValue count={agent.ratingCount} /> },
    { key: 'installCount', header: 'Installs', render: (agent) => agent.installCount.toLocaleString() },
  ];

  const installColumns: DataTableColumn<AgentInstall>[] = [
    { key: 'agentName', header: 'Agent', render: (install) => install.agentName ?? install.agentID ?? install.agentId },
    { key: 'version', header: 'Version', render: (install) => install.version ?? '-' },
    { key: 'installedAt', header: 'Installed', render: (install) => new Date(install.installedAt).toLocaleDateString() },
  ];
  const revenueTier = state.publisherStats?.revenueTier;
  const settlementSummary = [
    { label: 'Gross revenue', value: formatUSD(state.publisherStats?.grossRevenue) },
    { label: 'Platform fees', value: formatUSD(state.publisherStats?.platformFees) },
    { label: 'Net revenue', value: formatUSD(state.publisherStats?.netRevenue) },
    { label: 'Refunded', value: formatUSD(state.publisherStats?.refundedAmount) },
    { label: 'Pending settlement', value: formatUSD(state.publisherStats?.pendingSettlementAmount) },
    { label: 'Available payout', value: formatUSD(state.publisherStats?.availableAmount) },
    { label: 'Payout pending', value: formatUSD(state.publisherStats?.payoutPendingAmount) },
    { label: 'Paid out', value: formatUSD(state.publisherStats?.paidOutAmount) },
  ];

  const perAgentStatsColumns: DataTableColumn<NonNullable<PublisherStats['perAgentStats']>[number]>[] = [
    { key: 'agentName', header: 'Agent', render: (agent) => agent.agentName },
    { key: 'installCount', header: 'Installs', render: (agent) => formatNumber(agent.installCount) },
    { key: 'activeUsers', header: 'Active Users', render: (agent) => formatNumber(agent.activeUsers) },
    { key: 'apiCallCount', header: 'API Calls', render: (agent) => formatNumber(agent.apiCallCount) },
  ];

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="font-heading text-2xl font-semibold text-foreground">My Agents</h1>
          <p className="text-sm text-muted-foreground">Manage published and installed marketplace agents.</p>
        </div>
        <Button type="button" className="min-h-[44px]" asChild>
          <Link to="/marketplace/publish">Publish Agent</Link>
        </Button>
      </div>

      {state.error ? <div role="alert" className="rounded-lg border border-destructive/30 bg-card p-6 text-sm text-destructive">{state.error}</div> : null}
      {state.actionError ? (
        <div role="alert" className="rounded-lg border border-destructive/30 bg-card p-6 text-sm text-destructive">
          <p>{state.actionError.title}</p>
          {state.actionError.message ? <p>{state.actionError.message}</p> : null}
        </div>
      ) : null}

      <Card className="rounded-lg">
        <CardHeader>
          <h2 className="font-heading text-base font-medium">Settlement Cycle</h2>
        </CardHeader>
        <CardContent className="grid gap-4 lg:grid-cols-[1fr_auto] lg:items-end">
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <p className="text-xs font-medium uppercase text-muted-foreground">Current</p>
              <p className="mt-1 text-sm font-medium text-foreground">
                Current cycle: {state.settlementPreferences?.label ?? settlementCycleLabels[state.settlementCycleDraft]}
              </p>
            </div>
            <div>
              <p className="text-xs font-medium uppercase text-muted-foreground">Arrival</p>
              <p className="mt-1 text-sm text-foreground">{state.settlementPreferences?.payoutBusinessDays ?? 5} business days</p>
            </div>
            <div>
              <p className="text-xs font-medium uppercase text-muted-foreground">Fees</p>
              <p className="mt-1 text-sm text-foreground">{state.settlementPreferences?.processingFeePercent ?? 1}% processing fee</p>
              <p className="text-xs text-muted-foreground">${state.settlementPreferences?.minimumPayoutAmount ?? 100} minimum payout</p>
            </div>
            {revenueTier ? (
              <div>
                <p className="text-xs font-medium uppercase text-muted-foreground">Revenue Tier</p>
                <p className="mt-1 text-sm font-medium text-foreground">{revenueTier.label}</p>
                <p className="text-xs text-muted-foreground">{revenueTier.platformFeePercent}% current platform fee</p>
                <p className="text-xs text-muted-foreground">{formatUSD(revenueTier.salesToNextTier)} to next tier</p>
                <p className="text-xs text-muted-foreground">{formatUSD(revenueTier.estimatedPublisherNetIncreaseAtNextTier)} projected net increase</p>
              </div>
            ) : null}
          </div>
          <div className="grid gap-2 sm:grid-cols-[minmax(180px,1fr)_auto]">
            <label className="sr-only" htmlFor="settlement-cycle">Settlement cycle</label>
            <select
              id="settlement-cycle"
              aria-label="Settlement cycle"
              className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              disabled={state.loading || state.savingSettlementCycle}
              value={state.settlementCycleDraft}
              onChange={(event) => dispatch({ type: 'SET_SETTLEMENT_CYCLE_DRAFT', cycle: event.target.value as SettlementCycle })}
            >
              <option value="weekly">Weekly</option>
              <option value="monthly">Monthly</option>
              <option value="quarterly">Quarterly</option>
            </select>
            <Button type="button" className="min-h-[44px]" disabled={state.loading || state.savingSettlementCycle} onClick={() => void handleSettlementCycleSave()}>
              Save Settlement Cycle
            </Button>
          </div>
        </CardContent>
      </Card>

      <section className="space-y-4" aria-label="Settlement summary">
        <div>
          <h2 className="font-heading text-xl font-semibold text-foreground">Settlement Summary</h2>
          <p className="text-sm text-muted-foreground">Track publisher revenue, payout availability, and per-agent usage signals.</p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {settlementSummary.map((item) => (
            <div key={item.label} className="rounded-lg border border-border bg-card p-4">
              <p className="text-xs font-medium uppercase text-muted-foreground">{item.label}</p>
              <p className="mt-2 text-lg font-semibold text-foreground">{item.value}</p>
            </div>
          ))}
        </div>
        <DataTable
          columns={perAgentStatsColumns}
          data={state.publisherStats?.perAgentStats ?? []}
          loading={state.loading}
          error={null}
          emptyMessage="No per-agent settlement activity yet."
        />
      </section>

      <section className="space-y-4">
        <h2 className="font-heading text-xl font-semibold text-foreground">Published Agents</h2>
        <DataTable
          columns={publishedColumns}
          data={state.myAgents}
          loading={state.loading}
          error={null}
          emptyMessage="No published agents -- Publish your first agent to start the review process."
          renderActions={(agent) => (
            <Button type="button" variant="ghost" size="icon" aria-label={`Open agent ${agent.name}`} asChild>
              <Link to={`/marketplace/agents/${agent.id}`}>
                <RiExternalLinkLine className="size-4" aria-hidden="true" />
              </Link>
            </Button>
          )}
        />
      </section>

      <section className="space-y-4">
        <h2 className="font-heading text-xl font-semibold text-foreground">Installed Agents</h2>
        <DataTable
          columns={installColumns}
          data={state.installs}
          loading={state.loading}
          error={null}
          emptyMessage="No installed agents -- Install agents from the marketplace to use them in your workspace."
          renderActions={(install) => (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`Uninstall ${install.agentName ?? install.agentID ?? install.id}`}
              disabled={state.uninstallingID === install.id}
              onClick={() => void handleUninstall(install)}
            >
              <RiDeleteBinLine className="size-4" aria-hidden="true" />
            </Button>
          )}
        />
      </section>
    </div>
  );
}
