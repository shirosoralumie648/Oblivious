import { useCallback, useEffect, useMemo, useReducer } from 'react';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { ModelInventoryEntry, ModelInventoryFilter } from '../../types/admin';

type ModelsState = {
  models: ModelInventoryEntry[];
  total: number;
  loading: boolean;
  error: string | null;
  sortKey: string;
  sortDir: 'asc' | 'desc';
  filters: {
    provider: string;
    group: string;
    status: string;
    search: string;
  };
};

type ModelsAction =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; models: ModelInventoryEntry[]; total: number }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_FILTER'; field: keyof ModelsState['filters']; value: string }
  | { type: 'SORT'; key: string };

const initialState: ModelsState = {
  models: [],
  total: 0,
  loading: true,
  error: null,
  sortKey: 'model',
  sortDir: 'asc',
  filters: {
    provider: '',
    group: '',
    status: '',
    search: '',
  },
};

function reducer(state: ModelsState, action: ModelsAction): ModelsState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, models: action.models, total: action.total };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_FILTER':
      return { ...state, filters: { ...state.filters, [action.field]: action.value } };
    case 'SORT': {
      if (state.sortKey === action.key) {
        return { ...state, sortDir: state.sortDir === 'asc' ? 'desc' : 'asc' };
      }
      return {
        ...state,
        sortKey: action.key,
        sortDir: action.key === 'model' ? 'asc' : 'desc',
      };
    }
    default:
      return state;
  }
}

function money(value?: number) {
  return `$${(value ?? 0).toFixed(4)}`;
}

function costRange(model: ModelInventoryEntry) {
  if (model.minEstimatedCostPer1K === model.maxEstimatedCostPer1K) {
    return money(model.minEstimatedCostPer1K);
  }
  return `${money(model.minEstimatedCostPer1K)} - ${money(model.maxEstimatedCostPer1K)}`;
}

function grossMargin(model: ModelInventoryEntry) {
  return model.totalCost - model.totalChannelCost;
}

function badgeList(values: string[], variant: 'outline' | 'secondary' = 'outline') {
  if (values.length === 0) {
    return '-';
  }
  return (
    <div className="flex max-w-[260px] flex-wrap gap-1.5">
      {values.slice(0, 4).map((value) => (
        <Badge key={value} variant={variant}>
          {value}
        </Badge>
      ))}
      {values.length > 4 ? <Badge variant="secondary">+{values.length - 4}</Badge> : null}
    </div>
  );
}

function channelsCell(model: ModelInventoryEntry) {
  if (model.channels.length === 0) {
    return '-';
  }
  return (
    <div className="max-w-[260px] space-y-1">
      {model.channels.slice(0, 2).map((channel) => (
        <div key={channel.id} className="truncate text-sm text-foreground">
          {channel.name}
        </div>
      ))}
      {model.channels.length > 2 ? <div className="text-xs text-muted-foreground">+{model.channels.length - 2} more</div> : null}
    </div>
  );
}

export function AdminModelsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadModels = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const filters: ModelInventoryFilter = {
        provider: state.filters.provider,
        group: state.filters.group,
        status: state.filters.status,
        search: state.filters.search,
        sort: `${state.sortKey}:${state.sortDir}`,
        limit: 50,
        offset: 0,
      };
      const result = await api.listModelInventory(filters);
      dispatch({ type: 'LOAD_SUCCESS', models: result.data, total: result.total });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.filters.group, state.filters.provider, state.filters.search, state.filters.status, state.sortDir, state.sortKey]);

  useEffect(() => {
    void loadModels();
  }, [loadModels]);

  const columns: DataTableColumn<ModelInventoryEntry>[] = [
    { key: 'model', header: 'Model', render: (model) => <span className="font-mono text-sm">{model.model}</span>, sortable: true, width: '180px' },
    { key: 'providers', header: 'Providers', render: (model) => badgeList(model.providers) },
    { key: 'groups', header: 'Groups', render: (model) => badgeList(model.groups, 'secondary') },
    { key: 'channels', header: 'Channels', render: channelsCell },
    { key: 'enabledChannelCount', header: 'Availability', render: (model) => `${model.enabledChannelCount} / ${model.channelCount} enabled` },
    { key: 'estimatedCost', header: 'Cost / 1K', render: costRange },
    { key: 'avgCostMultiplier', header: 'Multiplier', render: (model) => `${model.avgCostMultiplier.toFixed(2)}x` },
    { key: 'requestCount', header: 'Requests', render: (model) => model.requestCount.toLocaleString(), sortable: true },
    { key: 'totalCost', header: 'Spend', render: (model) => money(model.totalCost), sortable: true },
    { key: 'totalChannelCost', header: 'Channel Cost', render: (model) => money(model.totalChannelCost), sortable: true },
    { key: 'grossMargin', header: 'Gross Margin', render: (model) => money(grossMargin(model)), sortable: true },
  ];

  const sortModels = (key: string) => {
    dispatch({ type: 'SORT', key });
  };

  const setFilter = (field: keyof ModelsState['filters']) => (event: React.ChangeEvent<HTMLInputElement>) => {
    dispatch({ type: 'SET_FILTER', field, value: event.target.value });
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-heading text-2xl font-semibold text-foreground">Models</h1>
          <p className="mt-1 text-sm text-muted-foreground">{state.total.toLocaleString()} relay models matched</p>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <Input aria-label="Provider filter" value={state.filters.provider} placeholder="Provider" className="min-h-[44px]" onChange={setFilter('provider')} />
        <Input aria-label="Group filter" value={state.filters.group} placeholder="Group" className="min-h-[44px]" onChange={setFilter('group')} />
        <Input aria-label="Status filter" value={state.filters.status} placeholder="Status" className="min-h-[44px]" onChange={setFilter('status')} />
        <Input aria-label="Search models" value={state.filters.search} placeholder="Search model or channel" className="min-h-[44px]" onChange={setFilter('search')} />
      </div>

      <DataTable
        columns={columns}
        data={state.models}
        loading={state.loading}
        error={state.error}
        emptyMessage="No relay models found for these filters."
        sortKey={state.sortKey}
        sortDir={state.sortDir}
        onSort={sortModels}
        onRetry={loadModels}
      />
    </div>
  );
}
