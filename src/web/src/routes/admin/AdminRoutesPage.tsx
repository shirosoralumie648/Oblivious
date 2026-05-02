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
import type { ChannelInfo, RouteCreateRequest, RouteInfo } from '../../types/admin';

type RouteForm = {
  model: string;
  targetChannelId: string;
  priority: string;
  weight: string;
  enabled: boolean;
};

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
  | { type: 'FORM_FIELD'; field: keyof RouteForm; value: string | boolean }
  | { type: 'FORM_START' }
  | { type: 'FORM_ERROR'; error: string }
  | { type: 'FORM_DONE' }
  | { type: 'CONFIRM_DELETE'; route: RouteInfo | null };

const emptyForm: RouteForm = {
  model: '',
  targetChannelId: '',
  priority: '1',
  weight: '100',
  enabled: true,
};

const initialState: RouteState = {
  routes: [],
  channels: [],
  loading: true,
  error: null,
  drawerOpen: false,
  editingRoute: null,
  form: emptyForm,
  formLoading: false,
  formError: null,
  confirmDelete: null,
};

function firstRouteChannel(route: RouteInfo) {
  return route.channels[0];
}

function routeToForm(route: RouteInfo): RouteForm {
  const channel = firstRouteChannel(route);
  return {
    model: route.model ?? route.modelPattern ?? '',
    targetChannelId: channel?.channelID ?? route.targetChannelId ?? '',
    priority: String(channel?.priority ?? route.priority ?? 1),
    weight: String(channel?.weight ?? 100),
    enabled: channel?.enabled ?? route.enabled ?? true,
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
      return { ...state, drawerOpen: true, editingRoute: null, form: emptyForm, formError: null };
    case 'OPEN_EDIT':
      return { ...state, drawerOpen: true, editingRoute: action.route, form: routeToForm(action.route), formError: null };
    case 'CLOSE_DRAWER':
      return { ...state, drawerOpen: false, editingRoute: null, formLoading: false, formError: null };
    case 'FORM_FIELD':
      return { ...state, form: { ...state.form, [action.field]: action.value } };
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
    strategy: 'single',
    channels: [
      {
        channelID: form.targetChannelId,
        priority: Number(form.priority) || 1,
        weight: Number(form.weight) || 100,
        enabled: form.enabled,
      },
    ],
  };
}

function routeTargetName(route: RouteInfo) {
  const channel = firstRouteChannel(route);
  return channel?.channelName ?? route.targetChannelName ?? channel?.channelID ?? '-';
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
    if (!payload.model || !payload.channels[0].channelID) {
      dispatch({ type: 'FORM_ERROR', error: 'Model pattern and target channel are required.' });
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
    { key: 'channels', header: 'Target Channel', render: routeTargetName },
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
          <label htmlFor="route-channel" className="text-sm font-medium">Target Channel</label>
          <select
            id="route-channel"
            value={state.form.targetChannelId}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'targetChannelId', value: event.target.value })}
            className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            <option value="">Select channel</option>
            {state.channels.map((channel) => (
              <option key={channel.id} value={channel.id}>
                {channel.name}
              </option>
            ))}
          </select>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <label htmlFor="route-priority" className="text-sm font-medium">Priority</label>
            <Input id="route-priority" type="number" value={state.form.priority} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'priority', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="route-weight" className="text-sm font-medium">Weight</label>
            <Input id="route-weight" type="number" value={state.form.weight} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'weight', value: event.target.value })} />
          </div>
        </div>
        <label className="flex min-h-[44px] items-center gap-3 text-sm">
          <input
            type="checkbox"
            checked={state.form.enabled}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'enabled', value: event.target.checked })}
            className="size-4"
          />
          Enabled
        </label>
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
