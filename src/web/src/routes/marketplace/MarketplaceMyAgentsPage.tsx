import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { Link } from 'react-router-dom';
import { RiDeleteBinLine, RiExternalLinkLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { RatingStars } from '../../components/shared/RatingStars';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createMarketplaceApi, type AgentInstall, type MarketplaceAgent } from '../../features/marketplace/api';
import { createHttpClient } from '../../services/http/client';
import type { StatusBadgeStatus } from '../../components/shared/StatusBadge';

type MyAgentsState = {
  myAgents: MarketplaceAgent[];
  installs: AgentInstall[];
  loading: boolean;
  error: string | null;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; myAgents: MarketplaceAgent[]; installs: AgentInstall[] }
  | { type: 'LOAD_ERROR'; error: string };

const initialState: MyAgentsState = {
  myAgents: [],
  installs: [],
  loading: true,
  error: null,
};

function reducer(state: MyAgentsState, action: Action): MyAgentsState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, myAgents: action.myAgents, installs: action.installs };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    default:
      return state;
  }
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

export function MarketplaceMyAgentsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createMarketplaceApi(createHttpClient()), []);

  const loadAgents = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const [myAgents, installs] = await Promise.all([api.getMyAgents(), api.getInstalledAgents()]);
      dispatch({ type: 'LOAD_SUCCESS', myAgents, installs });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Unable to load agents.' });
    }
  }, [api]);

  useEffect(() => {
    void loadAgents();
  }, [loadAgents]);

  const handleUninstall = async (install: AgentInstall) => {
    await api.uninstallAgent(install.id);
    await loadAgents();
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
            <Button type="button" variant="ghost" size="icon" aria-label={`Uninstall ${install.agentName ?? install.agentID ?? install.id}`} onClick={() => void handleUninstall(install)}>
              <RiDeleteBinLine className="size-4" aria-hidden="true" />
            </Button>
          )}
        />
      </section>
    </div>
  );
}
