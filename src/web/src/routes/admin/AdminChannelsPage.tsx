import { useCallback, useEffect, useMemo, useReducer, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { RiAddLine, RiDeleteBinLine, RiDownloadCloudLine, RiFlashlightLine, RiLoader4Line, RiPencilLine, RiRefreshLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';

import { ConfirmDialog } from '../../components/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { DrawerForm } from '../../components/shared/DrawerForm';
import { SearchBar } from '../../components/shared/SearchBar';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import { channelFormSchema } from '../../lib/formSchemas';
import type { ChannelCreateRequest, ChannelInfo, ChannelModelUpdatePreview, ChannelProviderInfo, ChannelRuntimeStats, ChannelTestResult, ChannelUpdateRequest } from '../../types/admin';

type ChannelForm = {
  name: string;
  provider: string;
  apiKey: string;
  baseURL: string;
  models: string;
  groups: string;
  rpmLimit: string;
  tpmLimit: string;
  priority: string;
  estimatedCostPer1K: string;
  costMultiplier: string;
  weight: string;
};

type RelayProviderOption = {
  value: string;
  label: string;
  defaultBaseURL?: string;
};

const defaultRelayProviderOptions: RelayProviderOption[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'claude', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'vertex', label: 'Vertex AI' },
  { value: 'bedrock', label: 'Amazon Bedrock' },
];

function providerCatalogToOptions(providers: ChannelProviderInfo[]): RelayProviderOption[] {
  const options = providers
    .filter((provider) => provider.status === 'supported')
    .map((provider) => ({
      value: canonicalProvider(provider.id),
      label: provider.displayName || provider.id,
      defaultBaseURL: provider.defaultBaseURL,
    }))
    .filter((provider) => provider.value && provider.label);
  return options.length > 0 ? options : defaultRelayProviderOptions;
}

function canonicalProvider(provider: string) {
  switch (provider.trim().toLowerCase()) {
    case 'anthropic':
      return 'claude';
    case 'google':
    case 'google-gemini':
      return 'gemini';
    case 'open-router':
      return 'openrouter';
    case 'vertex-ai':
      return 'vertex';
    case 'amazon-bedrock':
    case 'aws-bedrock':
      return 'bedrock';
    default:
      return provider.trim().toLowerCase() || 'openai';
  }
}

type ChannelState = {
  channels: ChannelInfo[];
  loading: boolean;
  error: string | null;
  sortKey: string;
  sortDir: 'asc' | 'desc';
  search: string;
  providerFilter: string;
  selectedIds: Set<string>;
  drawerOpen: boolean;
  editingChannel: ChannelInfo | null;
  formLoading: boolean;
  formError: string | null;
  confirmDelete: ChannelInfo | null;
  runtimeStats: Record<string, ChannelRuntimeStats>;
  runtimeStatsError: string | null;
  testResults: Record<string, ChannelTestResult | null>;
  testLoading: Record<string, boolean>;
  modelUpdatePreviews: Record<string, ChannelModelUpdatePreview | null>;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; channels: ChannelInfo[]; runtimeStats: Record<string, ChannelRuntimeStats>; runtimeStatsError: string | null }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_SEARCH'; value: string }
  | { type: 'SET_PROVIDER'; value: string }
  | { type: 'SORT'; key: string }
  | { type: 'SELECT'; id: string; checked: boolean }
  | { type: 'CLEAR_SELECTION' }
  | { type: 'OPEN_ADD' }
  | { type: 'OPEN_EDIT'; channel: ChannelInfo }
  | { type: 'CLOSE_DRAWER' }
  | { type: 'FORM_START' }
  | { type: 'FORM_ERROR'; error: string }
  | { type: 'FORM_DONE' }
  | { type: 'CONFIRM_DELETE'; channel: ChannelInfo | null }
  | { type: 'TEST_START'; id: string }
  | { type: 'TEST_DONE'; id: string; result: ChannelTestResult | null }
  | { type: 'MODEL_UPDATE_PREVIEW'; id: string; preview: ChannelModelUpdatePreview | null }
  | { type: 'HEALTH'; id: string; status: ChannelInfo['status']; latency: number | null };

const emptyForm: ChannelForm = {
  name: '',
  provider: 'openai',
  apiKey: '',
  baseURL: '',
  models: '',
  groups: '',
  rpmLimit: '60',
  tpmLimit: '60000',
  priority: '1',
  estimatedCostPer1K: '0',
  costMultiplier: '1',
  weight: '100',
};

const initialState: ChannelState = {
  channels: [],
  loading: true,
  error: null,
  sortKey: 'name',
  sortDir: 'asc',
  search: '',
  providerFilter: 'all',
  selectedIds: new Set(),
  drawerOpen: false,
  editingChannel: null,
  formLoading: false,
  formError: null,
  confirmDelete: null,
  runtimeStats: {},
  runtimeStatsError: null,
  testResults: {},
  testLoading: {},
  modelUpdatePreviews: {},
};

function channelToForm(channel: ChannelInfo): ChannelForm {
  return {
    name: channel.name,
    provider: canonicalProvider(channel.provider),
    apiKey: '',
    baseURL: channel.baseURL ?? channel.baseUrl ?? '',
    models: channel.models.join(', '),
    groups: (channel.groups ?? []).join(', '),
    rpmLimit: String(channel.rpm ?? 60),
    tpmLimit: String(channel.tpm ?? 60000),
    priority: String(channel.priority ?? 1),
    estimatedCostPer1K: String(channel.estimatedCostPer1K ?? 0),
    costMultiplier: String(channel.costMultiplier ?? 1),
    weight: String(channel.weight ?? 100),
  };
}

function reducer(state: ChannelState, action: Action): ChannelState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return {
        ...state,
        channels: action.channels,
        runtimeStats: action.runtimeStats,
        runtimeStatsError: action.runtimeStatsError,
        loading: false,
        error: null,
      };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_SEARCH':
      return { ...state, search: action.value };
    case 'SET_PROVIDER':
      return { ...state, providerFilter: action.value };
    case 'SORT':
      return {
        ...state,
        sortKey: action.key,
        sortDir: state.sortKey === action.key && state.sortDir === 'asc' ? 'desc' : 'asc',
      };
    case 'SELECT': {
      const selectedIds = new Set(state.selectedIds);
      if (action.checked) {
        selectedIds.add(action.id);
      } else {
        selectedIds.delete(action.id);
      }
      return { ...state, selectedIds };
    }
    case 'CLEAR_SELECTION':
      return { ...state, selectedIds: new Set() };
    case 'OPEN_ADD':
      return { ...state, drawerOpen: true, editingChannel: null, formError: null };
    case 'OPEN_EDIT':
      return { ...state, drawerOpen: true, editingChannel: action.channel, formError: null };
    case 'CLOSE_DRAWER':
      return { ...state, drawerOpen: false, editingChannel: null, formLoading: false, formError: null };
    case 'FORM_START':
      return { ...state, formLoading: true, formError: null };
    case 'FORM_ERROR':
      return { ...state, formLoading: false, formError: action.error };
    case 'FORM_DONE':
      return { ...state, formLoading: false, drawerOpen: false, editingChannel: null, formError: null };
    case 'CONFIRM_DELETE':
      return { ...state, confirmDelete: action.channel };
    case 'TEST_START':
      return { ...state, testLoading: { ...state.testLoading, [action.id]: true } };
    case 'TEST_DONE':
      return {
        ...state,
        testLoading: { ...state.testLoading, [action.id]: false },
        testResults: { ...state.testResults, [action.id]: action.result },
      };
    case 'MODEL_UPDATE_PREVIEW':
      return {
        ...state,
        testLoading: { ...state.testLoading, [action.id]: false },
        modelUpdatePreviews: { ...state.modelUpdatePreviews, [action.id]: action.preview },
      };
    case 'HEALTH':
      return {
        ...state,
        channels: state.channels.map((channel) =>
          channel.id === action.id ? { ...channel, status: action.status, latency: action.latency } : channel
        ),
      };
    default:
      return state;
  }
}

function channelPayload(form: ChannelForm): ChannelCreateRequest {
  return {
    name: form.name.trim(),
    provider: form.provider,
    apiKey: form.apiKey.trim(),
    baseURL: form.baseURL.trim(),
    models: form.models.split(',').map((model) => model.trim()).filter(Boolean),
    groups: form.groups.split(',').map((group) => group.trim()).filter(Boolean),
    rpmLimit: Number(form.rpmLimit) || 0,
    tpmLimit: Number(form.tpmLimit) || 0,
    priority: Number(form.priority) || 1,
    estimatedCostPer1K: Number(form.estimatedCostPer1K) || 0,
    costMultiplier: Number(form.costMultiplier) || 1,
    weight: Number(form.weight) || 100,
  };
}

function channelUpdatePayload(form: ChannelForm): ChannelUpdateRequest {
  const payload = channelPayload(form);

  if (payload.apiKey) {
    return payload;
  }

  return {
    name: payload.name,
    provider: payload.provider,
    baseURL: payload.baseURL,
    models: payload.models,
    groups: payload.groups,
    rpmLimit: payload.rpmLimit,
    tpmLimit: payload.tpmLimit,
    priority: payload.priority,
    estimatedCostPer1K: payload.estimatedCostPer1K,
    costMultiplier: payload.costMultiplier,
    weight: payload.weight,
  };
}

function normalizeLatency(result?: ChannelTestResult | null, latency?: number | null) {
  const value = result?.latencyMs ?? result?.latency ?? latency;
  return typeof value === 'number' && value > 0 ? `${value}ms` : '-';
}

function formatBalance(result?: ChannelTestResult | null) {
  if (!result?.balance) {
    return 'Unavailable';
  }
  const currency = result.balance.currency || 'USD';
  return `${currency} ${result.balance.amount.toFixed(2)}`;
}

function runtimeStatsByChannel(stats: ChannelRuntimeStats[]) {
  return stats.reduce<Record<string, ChannelRuntimeStats>>((acc, stat) => {
    const channelId = stat.channelID || stat.channelId;
    if (channelId) {
      acc[channelId] = stat;
    }
    return acc;
  }, {});
}

function formatRuntimeRate(stats?: ChannelRuntimeStats) {
  if (!stats) {
    return '-';
  }
  return `${stats.rpmCurrent.toLocaleString()} RPM`;
}

function formatRuntimeTokens(stats?: ChannelRuntimeStats) {
  if (!stats) {
    return '-';
  }
  return `${stats.tpmCurrent.toLocaleString()} TPM`;
}

function formatRuntimeLatency(stats?: ChannelRuntimeStats) {
  if (!stats || stats.avgLatencyMs <= 0) {
    return '-';
  }
  return `${Math.round(stats.avgLatencyMs).toLocaleString()}ms avg`;
}

function formatRateLimitedUntil(value?: string) {
  if (!value) {
    return 'Not limited';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return 'Limited';
  }
  return `Limited until ${date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: false,
  })}`;
}

export function AdminChannelsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const [providerOptions, setProviderOptions] = useState<RelayProviderOption[]>(defaultRelayProviderOptions);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const { register, handleSubmit, reset, formState: { errors } } = useForm<ChannelForm>({
    resolver: zodResolver(channelFormSchema),
    defaultValues: emptyForm,
  });

  useEffect(() => {
    if (state.editingChannel) {
      reset(channelToForm(state.editingChannel));
    } else if (state.drawerOpen) {
      reset(emptyForm);
    }
  }, [state.editingChannel, state.drawerOpen, reset]);

  const loadChannels = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const [channelsResult, runtimeStatsResult] = await Promise.allSettled([
        api.listChannels({
          search: state.search,
          provider: state.providerFilter === 'all' ? undefined : state.providerFilter,
          sort: `${state.sortKey}:${state.sortDir}`,
          limit: 50,
        }),
        api.listChannelStats(),
      ]);
      if (channelsResult.status === 'rejected') {
        throw channelsResult.reason;
      }
      dispatch({
        type: 'LOAD_SUCCESS',
        channels: channelsResult.value.data,
        runtimeStats: runtimeStatsResult.status === 'fulfilled' ? runtimeStatsByChannel(runtimeStatsResult.value) : {},
        runtimeStatsError: runtimeStatsResult.status === 'rejected'
          ? runtimeStatsResult.reason instanceof Error
            ? runtimeStatsResult.reason.message
            : 'Runtime stats are unavailable.'
          : null,
      });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.providerFilter, state.search, state.sortDir, state.sortKey]);

  useEffect(() => {
    void loadChannels();
  }, [loadChannels]);

  useEffect(() => {
    void api.listChannelProviders()
      .then((providers) => setProviderOptions(providerCatalogToOptions(providers)))
      .catch(() => setProviderOptions(defaultRelayProviderOptions));
  }, [api]);

  useEffect(() => {
    if (state.channels.length === 0) {
      return undefined;
    }

    const interval = window.setInterval(() => {
      state.channels.forEach((channel) => {
        void api.getChannelHealth(channel.id).then((health) => {
          dispatch({ type: 'HEALTH', id: channel.id, status: health.status, latency: health.latency });
        }).catch(() => undefined);
      });
    }, 30000);

    return () => window.clearInterval(interval);
  }, [api, state.channels]);

  const onSubmit = async (formData: ChannelForm) => {
    const payload = channelPayload(formData);

    dispatch({ type: 'FORM_START' });
    try {
      if (state.editingChannel) {
        await api.updateChannel(state.editingChannel.id, channelUpdatePayload(formData));
      } else {
        await api.createChannel(payload);
      }
      dispatch({ type: 'FORM_DONE' });
      await loadChannels();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to save channel.' });
    }
  };

  const handleTest = async (channel: ChannelInfo) => {
    dispatch({ type: 'TEST_START', id: channel.id });
    try {
      const result = await api.testChannel(channel.id);
      dispatch({ type: 'TEST_DONE', id: channel.id, result });
    } catch (error) {
      dispatch({
        type: 'TEST_DONE',
        id: channel.id,
        result: { success: false, latency: 0, error: error instanceof Error ? error.message : 'Connection test failed.' },
      });
    }
  };

  const handleSyncModels = async (channel: ChannelInfo) => {
    dispatch({ type: 'TEST_START', id: channel.id });
    try {
      const result = await api.syncChannelModels(channel.id);
      dispatch({ type: 'TEST_DONE', id: channel.id, result: result.testResult });
      await loadChannels();
    } catch (error) {
      dispatch({
        type: 'TEST_DONE',
        id: channel.id,
        result: { success: false, latency: 0, error: error instanceof Error ? error.message : 'Model sync failed.' },
      });
    }
  };

  const handleRefreshBalance = async (channel: ChannelInfo) => {
    dispatch({ type: 'TEST_START', id: channel.id });
    try {
      const result = await api.refreshChannelBalance(channel.id);
      dispatch({ type: 'TEST_DONE', id: channel.id, result: result.testResult });
      await loadChannels();
    } catch (error) {
      dispatch({
        type: 'TEST_DONE',
        id: channel.id,
        result: { success: false, latency: 0, error: error instanceof Error ? error.message : 'Balance refresh failed.' },
      });
    }
  };

  const handleDetectModelUpdates = async (channel: ChannelInfo) => {
    dispatch({ type: 'TEST_START', id: channel.id });
    try {
      const preview = await api.detectChannelModelUpdates(channel.id);
      dispatch({ type: 'MODEL_UPDATE_PREVIEW', id: channel.id, preview });
      dispatch({ type: 'TEST_DONE', id: channel.id, result: preview.testResult ?? null });
    } catch (error) {
      dispatch({ type: 'MODEL_UPDATE_PREVIEW', id: channel.id, preview: null });
      dispatch({
        type: 'TEST_DONE',
        id: channel.id,
        result: { success: false, latency: 0, error: error instanceof Error ? error.message : 'Model update detection failed.' },
      });
    }
  };

  const handleApplyModelUpdates = async (channel: ChannelInfo) => {
    dispatch({ type: 'TEST_START', id: channel.id });
    try {
      const result = await api.applyChannelModelUpdates(channel.id, { mode: 'merge' });
      if (result.preview) {
        dispatch({ type: 'MODEL_UPDATE_PREVIEW', id: channel.id, preview: result.preview });
        dispatch({ type: 'TEST_DONE', id: channel.id, result: result.preview.testResult ?? null });
      } else {
        dispatch({ type: 'TEST_DONE', id: channel.id, result: null });
      }
      await loadChannels();
    } catch (error) {
      dispatch({
        type: 'TEST_DONE',
        id: channel.id,
        result: { success: false, latency: 0, error: error instanceof Error ? error.message : 'Model update apply failed.' },
      });
    }
  };

  const handleBatch = async (action: 'enable' | 'disable') => {
    const ids = Array.from(state.selectedIds);
    await api.batchUpdateChannels(ids, action);
    dispatch({ type: 'CLEAR_SELECTION' });
    await loadChannels();
  };

  const handleDeleteConfirm = async () => {
    if (!state.confirmDelete) {
      return;
    }
    await api.deleteChannel(state.confirmDelete.id);
    dispatch({ type: 'CONFIRM_DELETE', channel: null });
    await loadChannels();
  };

  const columns: DataTableColumn<ChannelInfo>[] = [
    { key: 'name', header: 'Name', sortable: true },
    { key: 'provider', header: 'Provider', sortable: true },
    { key: 'status', header: 'Status', render: (channel) => <StatusBadge status={channel.status} /> },
    {
      key: 'models',
      header: 'Models',
      render: (channel) => (
        <div className="flex flex-wrap gap-1">
          {channel.models.slice(0, 3).map((model) => <Badge key={model} variant="outline">{model}</Badge>)}
          {channel.models.length > 3 ? <Badge variant="secondary">+{channel.models.length - 3}</Badge> : null}
        </div>
      ),
    },
    {
      key: 'estimatedCostPer1K',
      header: 'Cost/1K',
      render: (channel) => {
        const value = channel.estimatedCostPer1K ?? 0;
        return value > 0 ? value.toFixed(4) : '-';
      },
    },
    {
      key: 'runtimeRate',
      header: 'Runtime',
      render: (channel) => {
        const stats = state.runtimeStats[channel.id];
        return (
          <div className="space-y-0.5 text-sm">
            <div className="font-medium text-foreground">{formatRuntimeRate(stats)}</div>
            <div className="text-xs text-muted-foreground">{formatRuntimeTokens(stats)}</div>
          </div>
        );
      },
    },
    {
      key: 'avgLatency',
      header: 'Avg Latency',
      render: (channel) => formatRuntimeLatency(state.runtimeStats[channel.id]),
    },
    {
      key: 'rateLimitedUntil',
      header: 'Rate Limit',
      render: (channel) => {
        const stats = state.runtimeStats[channel.id];
        const isLimited = Boolean(stats?.rateLimitedUntil);
        return (
          <span className={isLimited ? 'font-medium text-destructive' : 'text-muted-foreground'}>
            {formatRateLimitedUntil(stats?.rateLimitedUntil)}
          </span>
        );
      },
    },
    { key: 'latency', header: 'Latency', render: (channel) => normalizeLatency(state.testResults[channel.id], channel.latency) },
  ];

  const diagnosticEntries = state.channels
    .map((channel) => ({ channel, result: state.testResults[channel.id] }))
    .filter((entry): entry is { channel: ChannelInfo; result: ChannelTestResult } => Boolean(entry.result));
  const runtimeDiagnosticEntries = state.channels
    .map((channel) => ({ channel, stats: state.runtimeStats[channel.id] }))
    .filter((entry): entry is { channel: ChannelInfo; stats: ChannelRuntimeStats } => Boolean(entry.stats));

  const selectedCount = state.selectedIds.size;

  return (
    <TooltipProvider>
      <div className="space-y-6">
        <div className="flex items-center justify-between gap-4">
          <h1 className="font-heading text-2xl font-semibold text-foreground">Channels</h1>
          <Button type="button" className="min-h-[44px]" onClick={() => dispatch({ type: 'OPEN_ADD' })}>
            <RiAddLine className="size-4" aria-hidden="true" />
            Add Channel
          </Button>
        </div>

        <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
          <SearchBar value={state.search} onChange={(value) => dispatch({ type: 'SET_SEARCH', value })} placeholder="Search channels..." />
          <select
            aria-label="Provider filter"
            value={state.providerFilter}
            onChange={(event) => dispatch({ type: 'SET_PROVIDER', value: event.target.value })}
            className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            <option value="all">All providers</option>
            {providerOptions.map((provider) => (
              <option key={provider.value} value={provider.value}>{provider.label}</option>
            ))}
          </select>
          {selectedCount > 0 ? (
            <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
              <span>{selectedCount} selected</span>
              <Button type="button" variant="outline" className="min-h-[44px]" onClick={() => void handleBatch('enable')}>
                Batch Enable
              </Button>
              <Button type="button" variant="outline" className="min-h-[44px]" onClick={() => void handleBatch('disable')}>
                Batch Disable
              </Button>
            </div>
          ) : null}
        </div>

        <DataTable
          columns={columns}
          data={state.channels}
          loading={state.loading}
          error={state.error}
          emptyMessage="No channels configured -- Add your first LLM provider channel to get started."
          sortKey={state.sortKey}
          sortDir={state.sortDir}
          onSort={(key) => dispatch({ type: 'SORT', key })}
          onRetry={loadChannels}
          selectable
          selectedIds={state.selectedIds}
          onSelectChange={(id, checked) => dispatch({ type: 'SELECT', id, checked })}
          renderActions={(channel) => (
            <div className="flex justify-end gap-1">
              <Button type="button" variant="ghost" size="icon" aria-label={`Edit channel ${channel.name}`} onClick={() => dispatch({ type: 'OPEN_EDIT', channel })}>
                <RiPencilLine className="size-4" aria-hidden="true" />
              </Button>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" variant="ghost" size="icon" aria-label={`Test connection for ${channel.name}`} disabled={state.testLoading[channel.id]} onClick={() => void handleTest(channel)}>
                    {state.testLoading[channel.id] ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiFlashlightLine className="size-4" aria-hidden="true" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {state.testResults[channel.id]?.success
                    ? `Connection OK: ${normalizeLatency(state.testResults[channel.id])}`
                    : state.testResults[channel.id]?.error ?? 'Test connection'}
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" variant="ghost" size="icon" aria-label={`Sync models for ${channel.name}`} disabled={state.testLoading[channel.id]} onClick={() => void handleSyncModels(channel)}>
                    {state.testLoading[channel.id] ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiDownloadCloudLine className="size-4" aria-hidden="true" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  Sync models from upstream
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" variant="ghost" size="icon" aria-label={`Detect model updates for ${channel.name}`} disabled={state.testLoading[channel.id]} onClick={() => void handleDetectModelUpdates(channel)}>
                    {state.testLoading[channel.id] ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiDownloadCloudLine className="size-4" aria-hidden="true" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  Detect upstream model changes
                </TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button type="button" variant="ghost" size="icon" aria-label={`Refresh balance for ${channel.name}`} disabled={state.testLoading[channel.id]} onClick={() => void handleRefreshBalance(channel)}>
                    {state.testLoading[channel.id] ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiRefreshLine className="size-4" aria-hidden="true" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  Refresh provider balance
                </TooltipContent>
              </Tooltip>
              <Button type="button" variant="ghost" size="icon" aria-label={`Delete channel ${channel.name}`} onClick={() => dispatch({ type: 'CONFIRM_DELETE', channel })}>
                <RiDeleteBinLine className="size-4" aria-hidden="true" />
              </Button>
            </div>
          )}
        />

        {runtimeDiagnosticEntries.length > 0 || state.runtimeStatsError ? (
          <section aria-label="Runtime diagnostics" className="rounded-lg border border-border bg-card p-4">
            <div className="flex flex-col gap-1 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h2 className="text-sm font-semibold text-foreground">Runtime diagnostics</h2>
                <p className="text-xs text-muted-foreground">In-memory relay counters reset when the relay process restarts.</p>
              </div>
              {state.runtimeStatsError ? <p className="text-xs text-destructive">{state.runtimeStatsError}</p> : null}
            </div>
            {runtimeDiagnosticEntries.length > 0 ? (
              <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {runtimeDiagnosticEntries.map(({ channel, stats }) => (
                  <div key={channel.id} className="rounded-md border border-border/80 p-3">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-medium text-foreground">{channel.name}</p>
                        <p className="text-xs text-muted-foreground">{formatRateLimitedUntil(stats.rateLimitedUntil)}</p>
                      </div>
                      <Badge variant="outline">{formatRuntimeLatency(stats)}</Badge>
                    </div>
                    <dl className="mt-3 grid grid-cols-2 gap-3 text-sm">
                      <div>
                        <dt className="text-xs text-muted-foreground">Requests</dt>
                        <dd className="font-medium text-foreground">{stats.totalRequests.toLocaleString()} requests</dd>
                      </div>
                      <div>
                        <dt className="text-xs text-muted-foreground">Results</dt>
                        <dd className="font-medium text-foreground">{stats.successCount.toLocaleString()} ok / {stats.failureCount.toLocaleString()} failed</dd>
                      </div>
                      <div>
                        <dt className="text-xs text-muted-foreground">RPM</dt>
                        <dd className="font-medium text-foreground">{stats.rpmCurrent.toLocaleString()}</dd>
                      </div>
	                      <div>
	                        <dt className="text-xs text-muted-foreground">TPM</dt>
	                        <dd className="font-medium text-foreground">{stats.tpmCurrent.toLocaleString()}</dd>
	                      </div>
	                      <div>
	                        <dt className="text-xs text-muted-foreground">Sticky conversations</dt>
	                        <dd className="font-medium text-foreground">{(stats.affinityConversationCount ?? 0).toLocaleString()} active</dd>
	                      </div>
	                    </dl>
	                  </div>
                ))}
              </div>
            ) : null}
          </section>
        ) : null}

        {diagnosticEntries.length > 0 ? (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {diagnosticEntries.map(({ channel, result }) => (
              <section key={channel.id} aria-label={`${channel.name} diagnostics`} className="rounded-lg border border-border bg-card p-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h2 className="text-sm font-semibold text-foreground">{channel.name} diagnostics</h2>
                    <p className="text-xs text-muted-foreground">{result.provider ?? channel.provider}</p>
                  </div>
                  <StatusBadge status={result.success ? 'online' : 'offline'} />
                </div>
                <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
                  <div>
                    <dt className="text-xs text-muted-foreground">Latency</dt>
                    <dd className="font-medium text-foreground">{normalizeLatency(result)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-muted-foreground">Balance</dt>
                    <dd className="font-medium text-foreground">{formatBalance(result)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-muted-foreground">Health</dt>
                    <dd className="font-medium text-foreground">Health {result.health?.status ?? (result.success ? 'online' : 'offline')}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-muted-foreground">Models</dt>
                    <dd className="font-medium text-foreground">{result.models?.length ?? 0}</dd>
                  </div>
                </dl>
                {result.models && result.models.length > 0 ? (
                  <div className="mt-3 flex flex-wrap gap-1">
                    {result.models.slice(0, 6).map((model) => <Badge key={model} variant="outline">{model}</Badge>)}
                    {result.models.length > 6 ? <Badge variant="secondary">+{result.models.length - 6}</Badge> : null}
                  </div>
                ) : null}
                {result.balanceError ? (
                  <p className="mt-3 text-xs text-muted-foreground">{result.balanceError}</p>
                ) : null}
                {result.error ? (
                  <p className="mt-3 text-xs text-destructive">{result.error}</p>
                ) : null}
              </section>
            ))}
          </div>
        ) : null}

        {Object.entries(state.modelUpdatePreviews).filter(([, preview]) => Boolean(preview)).length > 0 ? (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {Object.entries(state.modelUpdatePreviews).map(([channelId, preview]) => {
              if (!preview) {
                return null;
              }
              const channel = state.channels.find((item) => item.id === channelId);
              const channelName = channel?.name ?? channelId;
              return (
                <section key={channelId} aria-label={`Model updates for ${channelName}`} className="rounded-lg border border-border bg-card p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <h2 className="text-sm font-semibold text-foreground">Model updates for {channelName}</h2>
                      <p className="text-xs text-muted-foreground">{preview.upstreamModels.length} upstream models checked</p>
                    </div>
                    <Button type="button" variant="outline" className="min-h-[36px]" aria-label={`Apply model updates for ${channelName}`} onClick={() => channel ? void handleApplyModelUpdates(channel) : undefined}>
                      Apply
                    </Button>
                  </div>
                  <dl className="mt-4 grid grid-cols-3 gap-3 text-sm">
                    <div>
                      <dt className="text-xs text-muted-foreground">Added</dt>
                      <dd className="font-medium text-foreground">Added {preview.added.length}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-muted-foreground">Removed</dt>
                      <dd className="font-medium text-foreground">Removed {preview.removed.length}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-muted-foreground">Unchanged</dt>
                      <dd className="font-medium text-foreground">{preview.unchanged.length}</dd>
                    </div>
                  </dl>
                  {preview.added.length > 0 ? (
                    <div className="mt-3 flex flex-wrap gap-1">
                      {preview.added.slice(0, 6).map((model) => <Badge key={model} variant="outline">{model}</Badge>)}
                      {preview.added.length > 6 ? <Badge variant="secondary">+{preview.added.length - 6}</Badge> : null}
                    </div>
                  ) : null}
                  {preview.removed.length > 0 ? (
                    <p className="mt-3 text-xs text-muted-foreground">{preview.removed.join(', ')}</p>
                  ) : null}
                </section>
              );
            })}
          </div>
        ) : null}

        <DrawerForm
          open={state.drawerOpen}
          onOpenChange={(open) => {
            if (!open) {
              dispatch({ type: 'CLOSE_DRAWER' });
            }
          }}
          title={state.editingChannel ? 'Edit Channel' : 'Add Channel'}
          submitLabel={state.editingChannel ? 'Save Changes' : 'Create Channel'}
          onSubmit={() => void handleSubmit(onSubmit)()}
          loading={state.formLoading}
          error={state.formError}
        >
          <div className="space-y-2">
            <label htmlFor="channel-name" className="text-sm font-medium">Name</label>
            <Input id="channel-name" {...register('name')} />
            {errors.name && <p className="text-xs text-destructive">{errors.name.message}</p>}
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-provider" className="text-sm font-medium">Provider</label>
            <select
              id="channel-provider"
              {...register('provider')}
              className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
            >
              {providerOptions.map((provider) => (
                <option key={provider.value} value={provider.value}>{provider.label}</option>
              ))}
            </select>
            {errors.provider && <p className="text-xs text-destructive">{errors.provider.message}</p>}
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-api-key" className="text-sm font-medium">API Key</label>
            <Input id="channel-api-key" type="password" placeholder={state.editingChannel ? '********' : ''} {...register('apiKey')} />
            {errors.apiKey && <p className="text-xs text-destructive">{errors.apiKey.message}</p>}
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-base-url" className="text-sm font-medium">Base URL</label>
            <Input id="channel-base-url" {...register('baseURL')} />
            {errors.baseURL && <p className="text-xs text-destructive">{errors.baseURL.message}</p>}
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-models" className="text-sm font-medium">Models</label>
            <Input id="channel-models" placeholder="gpt-4o, gpt-4o-mini" {...register('models')} />
            {errors.models && <p className="text-xs text-destructive">{errors.models.message}</p>}
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-groups" className="text-sm font-medium">Groups</label>
            <Input id="channel-groups" placeholder="default, vip, enterprise" {...register('groups')} />
            {errors.groups && <p className="text-xs text-destructive">{errors.groups.message}</p>}
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label htmlFor="channel-rpm-limit" className="text-sm font-medium">RPM Limit</label>
              <Input id="channel-rpm-limit" type="number" min="0" {...register('rpmLimit')} />
              {errors.rpmLimit && <p className="text-xs text-destructive">{errors.rpmLimit.message}</p>}
            </div>
            <div className="space-y-2">
              <label htmlFor="channel-tpm-limit" className="text-sm font-medium">TPM Limit</label>
              <Input id="channel-tpm-limit" type="number" min="0" {...register('tpmLimit')} />
              {errors.tpmLimit && <p className="text-xs text-destructive">{errors.tpmLimit.message}</p>}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label htmlFor="channel-estimated-cost" className="text-sm font-medium">Estimated Cost per 1K</label>
              <Input id="channel-estimated-cost" type="number" step="0.0001" min="0" {...register('estimatedCostPer1K')} />
              {errors.estimatedCostPer1K && <p className="text-xs text-destructive">{errors.estimatedCostPer1K.message}</p>}
            </div>
            <div className="space-y-2">
              <label htmlFor="channel-cost-multiplier" className="text-sm font-medium">Cost Multiplier</label>
              <Input id="channel-cost-multiplier" type="number" step="0.01" min="0" {...register('costMultiplier')} />
              {errors.costMultiplier && <p className="text-xs text-destructive">{errors.costMultiplier.message}</p>}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label htmlFor="channel-priority" className="text-sm font-medium">Priority</label>
              <Input id="channel-priority" type="number" {...register('priority')} />
              {errors.priority && <p className="text-xs text-destructive">{errors.priority.message}</p>}
            </div>
            <div className="space-y-2">
              <label htmlFor="channel-weight" className="text-sm font-medium">Weight</label>
              <Input id="channel-weight" type="number" {...register('weight')} />
              {errors.weight && <p className="text-xs text-destructive">{errors.weight.message}</p>}
            </div>
          </div>
        </DrawerForm>

        <ConfirmDialog
          open={state.confirmDelete !== null}
          onOpenChange={(open) => {
            if (!open) {
              dispatch({ type: 'CONFIRM_DELETE', channel: null });
            }
          }}
          title="Delete Channel"
          description="Are you sure you want to delete this channel? This action cannot be undone."
          confirmLabel="Delete Channel"
          onConfirm={() => void handleDeleteConfirm()}
          variant="destructive"
        />
      </div>
    </TooltipProvider>
  );
}
