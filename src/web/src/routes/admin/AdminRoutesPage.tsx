import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiAddLine, RiDeleteBinLine, RiPencilLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

import { ConfirmDialog } from '../../components/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { DrawerForm } from '../../components/shared/DrawerForm';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { ChannelInfo, RouteCreateRequest, RouteInfo, RouteStrategy } from '../../types/admin';

type RouteFormChannel = {
  channelID: string;
  priority: string;
  weight: string;
  enabled: boolean;
};

type RouteForm = {
  model: string;
  strategy: RouteStrategy;
  channels: RouteFormChannel[];
};

const routeStrategyOptions: { value: RouteStrategy; label: string }[] = [
  { value: 'adaptive', label: 'Adaptive' },
  { value: 'weighted', label: 'Weighted' },
  { value: 'priority', label: 'Priority' },
  { value: 'cost_aware', label: 'Cost aware' },
];

type RouteState = {
  routes: RouteInfo[];
  channels: ChannelInfo[];
  loading: boolean;
  error: string | null;
  drawerOpen: boolean;
  editingRoute: RouteInfo | null;
  form: RouteForm;
  formLoading: boolean;
  formError: string | null;
  confirmDelete: RouteInfo | null;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; routes: RouteInfo[]; channels: ChannelInfo[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'OPEN_ADD' }
  | { type: 'OPEN_EDIT'; route: RouteInfo }
  | { type: 'CLOSE_DRAWER' }
  | { type: 'FORM_FIELD'; field: 'model' | 'strategy'; value: string }
  | { type: 'CHANNEL_FIELD'; index: number; field: keyof RouteFormChannel; value: string | boolean }
  | { type: 'ADD_CHANNEL' }
  | { type: 'REMOVE_CHANNEL'; index: number }
  | { type: 'FORM_START' }
  | { type: 'FORM_ERROR'; error: string }
  | { type: 'FORM_DONE' }
  | { type: 'CONFIRM_DELETE'; route: RouteInfo | null };

function emptyRouteChannel(): RouteFormChannel {
  return {
    channelID: '',
    priority: '1',
    weight: '100',
    enabled: true,
  };
}

function createEmptyForm(): RouteForm {
  return {
    model: '',
    strategy: 'adaptive',
    channels: [emptyRouteChannel()],
  };
}

const initialState: RouteState = {
  routes: [],
  channels: [],
  loading: true,
  error: null,
  drawerOpen: false,
  editingRoute: null,
  form: createEmptyForm(),
  formLoading: false,
  formError: null,
  confirmDelete: null,
};

function firstRouteChannel(route: RouteInfo) {
  return route.channels?.[0];
}

function routeStrategy(value?: string): RouteStrategy {
  return routeStrategyOptions.some((option) => option.value === value) ? value as RouteStrategy : 'adaptive';
}

function routeToForm(route: RouteInfo): RouteForm {
  const existingChannels = route.channels ?? [];
  const routeChannels = existingChannels.length > 0
    ? existingChannels.map((channel) => ({
        channelID: channel.channelID,
        priority: String(channel.priority ?? 1),
        weight: String(channel.weight ?? 100),
        enabled: channel.enabled ?? true,
      }))
    : [{
        channelID: route.targetChannelId ?? '',
        priority: String(route.priority ?? 1),
        weight: '100',
        enabled: route.enabled ?? true,
      }];

  return {
    model: route.model ?? route.modelPattern ?? '',
    strategy: routeStrategy(route.strategy),
    channels: routeChannels,
  };
}

function reducer(state: RouteState, action: Action): RouteState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, routes: action.routes, channels: action.channels, loading: false, error: null };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'OPEN_ADD':
      return { ...state, drawerOpen: true, editingRoute: null, form: createEmptyForm(), formError: null };
    case 'OPEN_EDIT':
      return { ...state, drawerOpen: true, editingRoute: action.route, form: routeToForm(action.route), formError: null };
    case 'CLOSE_DRAWER':
      return { ...state, drawerOpen: false, editingRoute: null, formLoading: false, formError: null };
    case 'FORM_FIELD':
      if (action.field === 'strategy') {
        return { ...state, form: { ...state.form, strategy: routeStrategy(action.value) } };
      }
      return { ...state, form: { ...state.form, model: action.value } };
    case 'CHANNEL_FIELD':
      return {
        ...state,
        form: {
          ...state.form,
          channels: state.form.channels.map((channel, index) => {
            if (index !== action.index) {
              return channel;
            }
            if (action.field === 'enabled') {
              return { ...channel, enabled: action.value === true };
            }
            return { ...channel, [action.field]: String(action.value) };
          }),
        },
      };
    case 'ADD_CHANNEL':
      return { ...state, form: { ...state.form, channels: [...state.form.channels, emptyRouteChannel()] } };
    case 'REMOVE_CHANNEL': {
      const channels = state.form.channels.filter((_, index) => index !== action.index);
      return {
        ...state,
        form: {
          ...state.form,
          channels: channels.length > 0 ? channels : [emptyRouteChannel()],
        },
      };
    }
    case 'FORM_START':
      return { ...state, formLoading: true, formError: null };
    case 'FORM_ERROR':
      return { ...state, formLoading: false, formError: action.error };
    case 'FORM_DONE':
      return { ...state, formLoading: false, drawerOpen: false, editingRoute: null, formError: null };
    case 'CONFIRM_DELETE':
      return { ...state, confirmDelete: action.route };
    default:
      return state;
  }
}

function routePayload(form: RouteForm): RouteCreateRequest {
  return {
    model: form.model.trim(),
    strategy: form.strategy,
    channels: form.channels.map((channel) => ({
      channelID: channel.channelID,
      priority: Number(channel.priority) || 1,
      weight: Number(channel.weight) || 100,
      enabled: channel.enabled,
    })),
  };
}

function routeTargetName(route: RouteInfo) {
  const channels = route.channels ?? [];
  if (channels.length === 0) {
    return route.targetChannelName ?? route.targetChannelId ?? '-';
  }
  return (
    <div className="flex max-w-xs flex-wrap gap-1.5">
      {channels.map((channel) => (
        <span key={channel.channelID} className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs text-foreground">
          <span>{channel.channelName ?? channel.channelID}</span>
          <span className="text-muted-foreground">p{channel.priority} w{channel.weight}</span>
        </span>
      ))}
    </div>
  );
}

function routeStrategyLabel(route: RouteInfo) {
  return route.strategy || 'adaptive';
}

function routePriority(route: RouteInfo) {
  return firstRouteChannel(route)?.priority ?? route.priority ?? 1;
}

function routeEnabled(route: RouteInfo) {
  return firstRouteChannel(route)?.enabled ?? route.enabled ?? true;
}

export function AdminRoutesPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadRoutes = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const [routes, channels] = await Promise.all([
        api.listRoutes(),
        api.listChannels({ limit: 100 }),
      ]);
      dispatch({ type: 'LOAD_SUCCESS', routes, channels: channels.data });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api]);

  useEffect(() => {
    void loadRoutes();
  }, [loadRoutes]);

  const handleSubmit = async () => {
    const payload = routePayload(state.form);
    if (!payload.model || payload.channels.length === 0 || payload.channels.some((channel) => !channel.channelID)) {
      dispatch({ type: 'FORM_ERROR', error: 'Model pattern and target channels are required.' });
      return;
    }

    dispatch({ type: 'FORM_START' });
    try {
      if (state.editingRoute) {
        await api.updateRoute(state.editingRoute.id, payload);
      } else {
        await api.createRoute(payload);
      }
      dispatch({ type: 'FORM_DONE' });
      await loadRoutes();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to save route.' });
    }
  };

  const handleDeleteConfirm = async () => {
    if (!state.confirmDelete) {
      return;
    }
    await api.deleteRoute(state.confirmDelete.id);
    dispatch({ type: 'CONFIRM_DELETE', route: null });
    await loadRoutes();
  };

  const columns: DataTableColumn<RouteInfo>[] = [
    { key: 'model', header: 'Model Pattern', sortable: true, render: (route) => route.model ?? route.modelPattern },
    { key: 'strategy', header: 'Strategy', sortable: true, render: routeStrategyLabel },
    { key: 'channels', header: 'Target Channels', render: routeTargetName },
    { key: 'priority', header: 'Priority', sortable: true, render: (route) => routePriority(route) },
    { key: 'enabled', header: 'Enabled', render: (route) => <StatusBadge status={routeEnabled(route) ? 'active' : 'disabled'} /> },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Model Routes</h1>
        <Button type="button" className="min-h-[44px]" onClick={() => dispatch({ type: 'OPEN_ADD' })}>
          <RiAddLine className="size-4" aria-hidden="true" />
          Add Route
        </Button>
      </div>

      <DataTable
        columns={columns}
        data={state.routes}
        loading={state.loading}
        error={state.error}
        emptyMessage="No model routes defined -- Create a route mapping to direct model requests to specific channels."
        onRetry={loadRoutes}
        renderActions={(route) => {
          const model = route.model ?? route.modelPattern ?? route.id;
          return (
            <div className="flex justify-end gap-1">
              <Button type="button" variant="ghost" size="icon" aria-label={`Edit route ${model}`} onClick={() => dispatch({ type: 'OPEN_EDIT', route })}>
                <RiPencilLine className="size-4" aria-hidden="true" />
              </Button>
              <Button type="button" variant="ghost" size="icon" aria-label={`Delete route ${model}`} onClick={() => dispatch({ type: 'CONFIRM_DELETE', route })}>
                <RiDeleteBinLine className="size-4" aria-hidden="true" />
              </Button>
            </div>
          );
        }}
      />

      <DrawerForm
        open={state.drawerOpen}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CLOSE_DRAWER' });
          }
        }}
        title={state.editingRoute ? 'Edit Route' : 'Add Route'}
        submitLabel={state.editingRoute ? 'Save Changes' : 'Create Route'}
        onSubmit={handleSubmit}
        loading={state.formLoading}
        error={state.formError}
      >
        <div className="space-y-2">
          <label htmlFor="route-model" className="text-sm font-medium">Model Pattern</label>
          <Input id="route-model" placeholder="gpt-4*, claude-*" value={state.form.model} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'model', value: event.target.value })} />
        </div>
        <div className="space-y-2">
          <label htmlFor="route-strategy" className="text-sm font-medium">Strategy</label>
          <select
            id="route-strategy"
            value={state.form.strategy}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'strategy', value: routeStrategy(event.target.value) })}
            className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            {routeStrategyOptions.map((strategy) => (
              <option key={strategy.value} value={strategy.value}>
                {strategy.label}
              </option>
            ))}
          </select>
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <h3 className="text-sm font-medium">Channel targets</h3>
            <Button type="button" size="xs" variant="outline" aria-label="Add channel target" onClick={() => dispatch({ type: 'ADD_CHANNEL' })}>
              <RiAddLine className="size-3" aria-hidden="true" />
              Add channel
            </Button>
          </div>
          {state.form.channels.map((channel, index) => {
            const number = index + 1;
            return (
              <div key={index} className="space-y-3 rounded-lg border border-border bg-muted/20 p-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Target {number}</span>
                  {state.form.channels.length > 1 ? (
                    <Button
                      type="button"
                      size="icon-xs"
                      variant="ghost"
                      aria-label={`Remove channel target ${number}`}
                      onClick={() => dispatch({ type: 'REMOVE_CHANNEL', index })}
                    >
                      <RiDeleteBinLine className="size-3" aria-hidden="true" />
                    </Button>
                  ) : null}
                </div>
                <div className="space-y-2">
                  <label htmlFor={`route-channel-${index}`} className="text-sm font-medium">Target Channel {number}</label>
                  <select
                    id={`route-channel-${index}`}
                    value={channel.channelID}
                    onChange={(event) => dispatch({ type: 'CHANNEL_FIELD', index, field: 'channelID', value: event.target.value })}
                    className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
                  >
                    <option value="">Select channel</option>
                    {state.channels.map((availableChannel) => (
                      <option key={availableChannel.id} value={availableChannel.id}>
                        {availableChannel.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label htmlFor={`route-priority-${index}`} className="text-sm font-medium">Priority {number}</label>
                    <Input
                      id={`route-priority-${index}`}
                      type="number"
                      value={channel.priority}
                      onChange={(event) => dispatch({ type: 'CHANNEL_FIELD', index, field: 'priority', value: event.target.value })}
                    />
                  </div>
                  <div className="space-y-2">
                    <label htmlFor={`route-weight-${index}`} className="text-sm font-medium">Weight {number}</label>
                    <Input
                      id={`route-weight-${index}`}
                      type="number"
                      value={channel.weight}
                      onChange={(event) => dispatch({ type: 'CHANNEL_FIELD', index, field: 'weight', value: event.target.value })}
                    />
                  </div>
                </div>
                <label className="flex min-h-[44px] items-center gap-3 text-sm">
                  <input
                    type="checkbox"
                    checked={channel.enabled}
                    onChange={(event) => dispatch({ type: 'CHANNEL_FIELD', index, field: 'enabled', value: event.target.checked })}
                    className="size-4"
                  />
                  Enabled {number}
                </label>
              </div>
            );
          })}
        </div>
      </DrawerForm>

      <ConfirmDialog
        open={state.confirmDelete !== null}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CONFIRM_DELETE', route: null });
          }
        }}
        title="Delete Route"
        description="Are you sure you want to delete this route? This action cannot be undone."
        confirmLabel="Delete Route"
        onConfirm={() => void handleDeleteConfirm()}
        variant="destructive"
      />
    </div>
  );
}
