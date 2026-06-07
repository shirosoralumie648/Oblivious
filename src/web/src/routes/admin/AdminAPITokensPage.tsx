import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiCloseCircleLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { StatusBadge, type StatusBadgeStatus } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { APITokenEntry, APITokenFilter } from '../../types/admin';

type APITokenState = {
  tokens: APITokenEntry[];
  total: number;
  loading: boolean;
  error: string | null;
  revokingID: string | null;
  filters: {
    organizationID: string;
    userID: string;
    status: string;
    userGroup: string;
    search: string;
    model: string;
  };
};

type APITokenAction =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; tokens: APITokenEntry[]; total: number }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_FILTER'; field: keyof APITokenState['filters']; value: string }
  | { type: 'REVOKE_START'; tokenID: string }
  | { type: 'REVOKE_END' };

const initialState: APITokenState = {
  tokens: [],
  total: 0,
  loading: true,
  error: null,
  revokingID: null,
  filters: {
    organizationID: '',
    userID: '',
    status: '',
    userGroup: '',
    search: '',
    model: '',
  },
};

function reducer(state: APITokenState, action: APITokenAction): APITokenState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, tokens: action.tokens, total: action.total };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_FILTER':
      return { ...state, filters: { ...state.filters, [action.field]: action.value } };
    case 'REVOKE_START':
      return { ...state, revokingID: action.tokenID, error: null };
    case 'REVOKE_END':
      return { ...state, revokingID: null };
    default:
      return state;
  }
}

function money(value?: number | null) {
  return `$${(value ?? 0).toFixed(4)}`;
}

function quotaCell(token: APITokenEntry) {
  const limit = token.quotaLimit === null || token.quotaLimit === undefined ? 'Unlimited' : money(token.quotaLimit);
  return `${money(token.usedQuota)} / ${limit}`;
}

function dateLabel(value?: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

function idCell(value?: string) {
  return <span className="break-all font-mono text-xs">{value || '-'}</span>;
}

function modelLimitsCell(token: APITokenEntry) {
  if (!token.modelLimitsEnabled) {
    return 'All models';
  }
  if (token.modelLimits.length === 0) {
    return '-';
  }
  return token.modelLimits.join(', ');
}

const tokenStatusTone: Record<string, StatusBadgeStatus> = {
  active: 'active',
  revoked: 'disabled',
  expired: 'disabled',
  pending: 'pending',
};

function tokenStatusLabel(status: string) {
  return status
    .split('_')
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join(' ');
}

function statusCell(token: APITokenEntry) {
  return (
    <StatusBadge
      status={tokenStatusTone[token.status] ?? 'pending'}
      label={tokenStatusLabel(token.status || 'unknown')}
    />
  );
}

export function AdminAPITokensPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadAPITokens = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const filters: APITokenFilter = {
        organizationID: state.filters.organizationID,
        userID: state.filters.userID,
        status: state.filters.status,
        userGroup: state.filters.userGroup,
        search: state.filters.search,
        model: state.filters.model,
        limit: 50,
        offset: 0,
      };
      const result = await api.listAPITokens(filters);
      dispatch({ type: 'LOAD_SUCCESS', tokens: result.data, total: result.total });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.filters.model, state.filters.organizationID, state.filters.search, state.filters.status, state.filters.userGroup, state.filters.userID]);

  useEffect(() => {
    void loadAPITokens();
  }, [loadAPITokens]);

  const revokeToken = useCallback(
    async (token: APITokenEntry) => {
      if (!window.confirm(`Revoke API token "${token.name}"?`)) {
        return;
      }
      dispatch({ type: 'REVOKE_START', tokenID: token.id });
      try {
        await api.revokeAPIToken(token.id);
        await loadAPITokens();
      } catch (error) {
        dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while revoking this token.' });
      } finally {
        dispatch({ type: 'REVOKE_END' });
      }
    },
    [api, loadAPITokens]
  );

  const columns: DataTableColumn<APITokenEntry>[] = [
    { key: 'name', header: 'Token', render: (token) => token.name || '-' },
    { key: 'tokenPrefix', header: 'Prefix', render: (token) => idCell(token.tokenPrefix), width: '120px' },
    { key: 'userEmail', header: 'User', render: (token) => token.userEmail || token.userId },
    { key: 'userGroup', header: 'Group', render: (token) => token.userGroup || '-' },
    { key: 'organizationId', header: 'Organization', render: (token) => idCell(token.organizationId), width: '160px' },
    { key: 'status', header: 'Status', render: statusCell },
    { key: 'modelLimits', header: 'Models', render: modelLimitsCell },
    { key: 'quota', header: 'Quota', render: quotaCell },
    { key: 'requestCount', header: 'Requests', render: (token) => token.requestCount.toLocaleString() },
    { key: 'totalCost', header: 'Cost', render: (token) => money(token.totalCost) },
    { key: 'lastUsedAt', header: 'Last Used', render: (token) => dateLabel(token.lastUsedAt) },
  ];

  const setFilter = (field: keyof APITokenState['filters']) => (event: React.ChangeEvent<HTMLInputElement>) => {
    dispatch({ type: 'SET_FILTER', field, value: event.target.value });
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-heading text-2xl font-semibold text-foreground">API Tokens</h1>
          <p className="mt-1 text-sm text-muted-foreground">{state.total.toLocaleString()} relay keys matched</p>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
        <Input aria-label="Organization ID filter" value={state.filters.organizationID} placeholder="Organization ID" className="min-h-[44px]" onChange={setFilter('organizationID')} />
        <Input aria-label="User ID filter" value={state.filters.userID} placeholder="User ID" className="min-h-[44px]" onChange={setFilter('userID')} />
        <Input aria-label="Status filter" value={state.filters.status} placeholder="Status" className="min-h-[44px]" onChange={setFilter('status')} />
        <Input aria-label="User group filter" value={state.filters.userGroup} placeholder="User group" className="min-h-[44px]" onChange={setFilter('userGroup')} />
        <Input aria-label="Search tokens" value={state.filters.search} placeholder="Search name, prefix, or email" className="min-h-[44px]" onChange={setFilter('search')} />
        <Input aria-label="Model filter" value={state.filters.model} placeholder="Model" className="min-h-[44px]" onChange={setFilter('model')} />
      </div>

      <DataTable
        columns={columns}
        data={state.tokens}
        loading={state.loading}
        error={state.error}
        emptyMessage="No API tokens found for these filters."
        onRetry={loadAPITokens}
        renderActions={(token) => (
          <Button
            type="button"
            variant="destructive"
            size="sm"
            aria-label={`Revoke ${token.name}`}
            disabled={token.status !== 'active' || state.revokingID === token.id}
            onClick={() => void revokeToken(token)}
          >
            <RiCloseCircleLine className="size-4" aria-hidden="true" />
            Revoke
          </Button>
        )}
      />
    </div>
  );
}
