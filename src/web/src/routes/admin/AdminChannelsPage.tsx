import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiAddLine, RiDeleteBinLine, RiFlashlightLine, RiLoader4Line, RiPencilLine } from '@remixicon/react';

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
import type { ChannelCreateRequest, ChannelInfo, ChannelTestResult, ChannelUpdateRequest } from '../../types/admin';

type ChannelForm = {
  name: string;
  provider: string;
  apiKey: string;
  baseURL: string;
  models: string;
  rpmLimit: string;
  tpmLimit: string;
  priority: string;
  weight: string;
};

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
  form: ChannelForm;
  formLoading: boolean;
  formError: string | null;
  confirmDelete: ChannelInfo | null;
  testResults: Record<string, ChannelTestResult | null>;
  testLoading: Record<string, boolean>;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; channels: ChannelInfo[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_SEARCH'; value: string }
  | { type: 'SET_PROVIDER'; value: string }
  | { type: 'SORT'; key: string }
  | { type: 'SELECT'; id: string; checked: boolean }
  | { type: 'CLEAR_SELECTION' }
  | { type: 'OPEN_ADD' }
  | { type: 'OPEN_EDIT'; channel: ChannelInfo }
  | { type: 'CLOSE_DRAWER' }
  | { type: 'FORM_FIELD'; field: keyof ChannelForm; value: string }
  | { type: 'FORM_START' }
  | { type: 'FORM_ERROR'; error: string }
  | { type: 'FORM_DONE' }
  | { type: 'CONFIRM_DELETE'; channel: ChannelInfo | null }
  | { type: 'TEST_START'; id: string }
  | { type: 'TEST_DONE'; id: string; result: ChannelTestResult | null }
  | { type: 'HEALTH'; id: string; status: ChannelInfo['status']; latency: number | null };

const emptyForm: ChannelForm = {
  name: '',
  provider: 'openai',
  apiKey: '',
  baseURL: '',
  models: '',
  rpmLimit: '60',
  tpmLimit: '60000',
  priority: '1',
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
  form: emptyForm,
  formLoading: false,
  formError: null,
  confirmDelete: null,
  testResults: {},
  testLoading: {},
};

function channelToForm(channel: ChannelInfo): ChannelForm {
  return {
    name: channel.name,
    provider: channel.provider,
    apiKey: '',
    baseURL: channel.baseURL ?? channel.baseUrl ?? '',
    models: channel.models.join(', '),
    rpmLimit: String(channel.rpm ?? 60),
    tpmLimit: String(channel.tpm ?? 60000),
    priority: String(channel.priority ?? 1),
    weight: String(channel.weight ?? 100),
  };
}

function reducer(state: ChannelState, action: Action): ChannelState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, channels: action.channels, loading: false, error: null };
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
      return { ...state, drawerOpen: true, editingChannel: null, form: emptyForm, formError: null };
    case 'OPEN_EDIT':
      return { ...state, drawerOpen: true, editingChannel: action.channel, form: channelToForm(action.channel), formError: null };
    case 'CLOSE_DRAWER':
      return { ...state, drawerOpen: false, editingChannel: null, formLoading: false, formError: null };
    case 'FORM_FIELD':
      return { ...state, form: { ...state.form, [action.field]: action.value } };
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
    rpmLimit: Number(form.rpmLimit) || 0,
    tpmLimit: Number(form.tpmLimit) || 0,
    priority: Number(form.priority) || 1,
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
    rpmLimit: payload.rpmLimit,
    tpmLimit: payload.tpmLimit,
    priority: payload.priority,
    weight: payload.weight,
  };
}

function normalizeLatency(result?: ChannelTestResult | null, latency?: number | null) {
  const value = result?.latencyMs ?? result?.latency ?? latency;
  return typeof value === 'number' && value > 0 ? `${value}ms` : '-';
}

export function AdminChannelsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadChannels = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const result = await api.listChannels({
        search: state.search,
        provider: state.providerFilter === 'all' ? undefined : state.providerFilter,
        sort: `${state.sortKey}:${state.sortDir}`,
        limit: 50,
      });
      dispatch({ type: 'LOAD_SUCCESS', channels: result.data });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.providerFilter, state.search, state.sortDir, state.sortKey]);

  useEffect(() => {
    void loadChannels();
  }, [loadChannels]);

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

  const handleSubmit = async () => {
    const payload = channelPayload(state.form);
    if (!payload.name || !payload.baseURL) {
      dispatch({ type: 'FORM_ERROR', error: 'Name and Base URL are required.' });
      return;
    }

    dispatch({ type: 'FORM_START' });
    try {
      if (state.editingChannel) {
        await api.updateChannel(state.editingChannel.id, channelUpdatePayload(state.form));
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
    { key: 'latency', header: 'Latency', render: (channel) => normalizeLatency(state.testResults[channel.id], channel.latency) },
  ];

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
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="google">Google</option>
            <option value="azure">Azure</option>
            <option value="custom">Custom</option>
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
                  <Button type="button" variant="ghost" size="icon" aria-label={`Test connection for ${channel.name}`} onClick={() => void handleTest(channel)}>
                    {state.testLoading[channel.id] ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiFlashlightLine className="size-4" aria-hidden="true" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {state.testResults[channel.id]?.success
                    ? `Connection OK: ${normalizeLatency(state.testResults[channel.id])}`
                    : state.testResults[channel.id]?.error ?? 'Test connection'}
                </TooltipContent>
              </Tooltip>
              <Button type="button" variant="ghost" size="icon" aria-label={`Delete channel ${channel.name}`} onClick={() => dispatch({ type: 'CONFIRM_DELETE', channel })}>
                <RiDeleteBinLine className="size-4" aria-hidden="true" />
              </Button>
            </div>
          )}
        />

        <DrawerForm
          open={state.drawerOpen}
          onOpenChange={(open) => {
            if (!open) {
              dispatch({ type: 'CLOSE_DRAWER' });
            }
          }}
          title={state.editingChannel ? 'Edit Channel' : 'Add Channel'}
          submitLabel={state.editingChannel ? 'Save Changes' : 'Create Channel'}
          onSubmit={handleSubmit}
          loading={state.formLoading}
          error={state.formError}
        >
          <div className="space-y-2">
            <label htmlFor="channel-name" className="text-sm font-medium">Name</label>
            <Input id="channel-name" value={state.form.name} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'name', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-provider" className="text-sm font-medium">Provider</label>
            <select
              id="channel-provider"
              value={state.form.provider}
              onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'provider', value: event.target.value })}
              className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
            >
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="google">Google</option>
              <option value="azure">Azure</option>
              <option value="custom">Custom</option>
            </select>
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-api-key" className="text-sm font-medium">API Key</label>
            <Input id="channel-api-key" type="password" value={state.form.apiKey} placeholder={state.editingChannel ? '********' : ''} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'apiKey', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-base-url" className="text-sm font-medium">Base URL</label>
            <Input id="channel-base-url" value={state.form.baseURL} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'baseURL', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="channel-models" className="text-sm font-medium">Models</label>
            <Input id="channel-models" value={state.form.models} placeholder="gpt-4o, gpt-4o-mini" onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'models', value: event.target.value })} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <label htmlFor="channel-priority" className="text-sm font-medium">Priority</label>
              <Input id="channel-priority" type="number" value={state.form.priority} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'priority', value: event.target.value })} />
            </div>
            <div className="space-y-2">
              <label htmlFor="channel-weight" className="text-sm font-medium">Weight</label>
              <Input id="channel-weight" type="number" value={state.form.weight} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'weight', value: event.target.value })} />
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
