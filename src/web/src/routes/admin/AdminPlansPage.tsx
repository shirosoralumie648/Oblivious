import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiAddLine, RiDeleteBinLine, RiPencilLine } from '@remixicon/react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

import { ConfirmDialog } from '../../components/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { DrawerForm } from '../../components/shared/DrawerForm';
import { SearchBar } from '../../components/shared/SearchBar';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { PlanCreateRequest, PlanInfo } from '../../types/admin';

type PlanForm = {
  name: string;
  description: string;
  quotaAmount: string;
  tokenQuota: string;
  price: string;
  modelAccess: string;
  agentLimit: string;
  maxTokensPerRequest: string;
  durationDays: string;
  isPublic: boolean;
  sortOrder: string;
};

type PlanState = {
  plans: PlanInfo[];
  loading: boolean;
  error: string | null;
  search: string;
  statusFilter: string;
  publicFilter: string;
  drawerOpen: boolean;
  editingPlan: PlanInfo | null;
  form: PlanForm;
  formLoading: boolean;
  formError: string | null;
  confirmDeactivate: PlanInfo | null;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; plans: PlanInfo[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_SEARCH'; value: string }
  | { type: 'SET_STATUS'; value: string }
  | { type: 'SET_PUBLIC'; value: string }
  | { type: 'OPEN_ADD' }
  | { type: 'OPEN_EDIT'; plan: PlanInfo }
  | { type: 'CLOSE_DRAWER' }
  | { type: 'FORM_FIELD'; field: keyof PlanForm; value: string | boolean }
  | { type: 'FORM_START' }
  | { type: 'FORM_ERROR'; error: string }
  | { type: 'FORM_DONE' }
  | { type: 'CONFIRM_DEACTIVATE'; plan: PlanInfo | null };

const emptyForm: PlanForm = {
  name: '',
  description: '',
  quotaAmount: '0',
  tokenQuota: '0',
  price: '0',
  modelAccess: '',
  agentLimit: '1',
  maxTokensPerRequest: '0',
  durationDays: '',
  isPublic: true,
  sortOrder: '0',
};

const initialState: PlanState = {
  plans: [],
  loading: true,
  error: null,
  search: '',
  statusFilter: 'all',
  publicFilter: 'all',
  drawerOpen: false,
  editingPlan: null,
  form: emptyForm,
  formLoading: false,
  formError: null,
  confirmDeactivate: null,
};

function planToForm(plan: PlanInfo): PlanForm {
  return {
    name: plan.name,
    description: plan.description,
    quotaAmount: String(plan.quotaAmount),
    tokenQuota: String(plan.tokenQuota),
    price: String(plan.price),
    modelAccess: plan.modelAccess.join(', '),
    agentLimit: String(plan.agentLimit),
    maxTokensPerRequest: String(plan.maxTokensPerRequest ?? 0),
    durationDays: plan.durationDays == null ? '' : String(plan.durationDays),
    isPublic: plan.isPublic,
    sortOrder: String(plan.sortOrder),
  };
}

function reducer(state: PlanState, action: Action): PlanState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, plans: action.plans };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_SEARCH':
      return { ...state, search: action.value };
    case 'SET_STATUS':
      return { ...state, statusFilter: action.value };
    case 'SET_PUBLIC':
      return { ...state, publicFilter: action.value };
    case 'OPEN_ADD':
      return { ...state, drawerOpen: true, editingPlan: null, form: emptyForm, formError: null };
    case 'OPEN_EDIT':
      return { ...state, drawerOpen: true, editingPlan: action.plan, form: planToForm(action.plan), formError: null };
    case 'CLOSE_DRAWER':
      return { ...state, drawerOpen: false, editingPlan: null, formLoading: false, formError: null };
    case 'FORM_FIELD':
      return { ...state, form: { ...state.form, [action.field]: action.value } };
    case 'FORM_START':
      return { ...state, formLoading: true, formError: null };
    case 'FORM_ERROR':
      return { ...state, formLoading: false, formError: action.error };
    case 'FORM_DONE':
      return { ...state, formLoading: false, drawerOpen: false, editingPlan: null, formError: null };
    case 'CONFIRM_DEACTIVATE':
      return { ...state, confirmDeactivate: action.plan };
    default:
      return state;
  }
}

function numericValue(value: string, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function planPayload(form: PlanForm): PlanCreateRequest {
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    quotaAmount: numericValue(form.quotaAmount),
    tokenQuota: numericValue(form.tokenQuota),
    price: numericValue(form.price),
    modelAccess: form.modelAccess.split(',').map((model) => model.trim()).filter(Boolean),
    agentLimit: numericValue(form.agentLimit, 1),
    maxTokensPerRequest: numericValue(form.maxTokensPerRequest),
    durationDays: form.durationDays.trim() ? numericValue(form.durationDays) : null,
    isPublic: form.isPublic,
    sortOrder: numericValue(form.sortOrder),
  };
}

function money(value: number) {
  return `$${value.toFixed(2)}`;
}

export function AdminPlansPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadPlans = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const plans = await api.listPlans({
        search: state.search,
        status: state.statusFilter === 'all' ? undefined : state.statusFilter,
        isPublic: state.publicFilter === 'all' ? undefined : state.publicFilter === 'public',
        limit: 100,
      });
      dispatch({ type: 'LOAD_SUCCESS', plans });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.publicFilter, state.search, state.statusFilter]);

  useEffect(() => {
    void loadPlans();
  }, [loadPlans]);

  const handleSubmit = async () => {
    const payload = planPayload(state.form);
    if (!payload.name || !payload.description) {
      dispatch({ type: 'FORM_ERROR', error: 'Name and description are required.' });
      return;
    }

    dispatch({ type: 'FORM_START' });
    try {
      if (state.editingPlan) {
        await api.updatePlan(state.editingPlan.id, payload);
      } else {
        await api.createPlan(payload);
      }
      dispatch({ type: 'FORM_DONE' });
      await loadPlans();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to save plan.' });
    }
  };

  const handleDeactivate = async () => {
    if (!state.confirmDeactivate) {
      return;
    }
    await api.deactivatePlan(state.confirmDeactivate.id);
    dispatch({ type: 'CONFIRM_DEACTIVATE', plan: null });
    await loadPlans();
  };

  const columns: DataTableColumn<PlanInfo>[] = [
    { key: 'name', header: 'Name', sortable: true },
    { key: 'price', header: 'Price', render: (plan) => money(plan.price) },
    { key: 'quotaAmount', header: 'Quota', render: (plan) => `${plan.quotaAmount.toLocaleString()} credits / ${plan.tokenQuota.toLocaleString()} tokens` },
    { key: 'maxTokensPerRequest', header: 'Request Cap', render: (plan) => `${(plan.maxTokensPerRequest ?? 0).toLocaleString()} tokens` },
    {
      key: 'modelAccess',
      header: 'Model Access',
      render: (plan) => (
        <div className="flex flex-wrap gap-1">
          {plan.modelAccess.slice(0, 3).map((model) => <Badge key={model} variant="outline">{model}</Badge>)}
          {plan.modelAccess.length > 3 ? <Badge variant="secondary">+{plan.modelAccess.length - 3}</Badge> : null}
        </div>
      ),
    },
    { key: 'subscriberCount', header: 'Subscribers', render: (plan) => plan.subscriberCount ?? 0 },
    {
      key: 'isActive',
      header: 'Status',
      render: (plan) => (
        <div className="flex flex-col gap-1">
          <StatusBadge status={plan.isActive ? 'active' : 'disabled'} />
          <StatusBadge status={plan.isPublic ? 'public' : 'private'} />
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Plans</h1>
        <Button type="button" className="min-h-[44px]" onClick={() => dispatch({ type: 'OPEN_ADD' })}>
          <RiAddLine className="size-4" aria-hidden="true" />
          Add Plan
        </Button>
      </div>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <SearchBar value={state.search} onChange={(value) => dispatch({ type: 'SET_SEARCH', value })} placeholder="Search plans..." />
        <select
          aria-label="Plan status filter"
          value={state.statusFilter}
          onChange={(event) => dispatch({ type: 'SET_STATUS', value: event.target.value })}
          className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
        >
          <option value="all">All statuses</option>
          <option value="active">Active</option>
          <option value="inactive">Inactive</option>
        </select>
        <select
          aria-label="Plan visibility filter"
          value={state.publicFilter}
          onChange={(event) => dispatch({ type: 'SET_PUBLIC', value: event.target.value })}
          className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
        >
          <option value="all">All visibility</option>
          <option value="public">Public</option>
          <option value="private">Private</option>
        </select>
      </div>

      <DataTable
        columns={columns}
        data={state.plans}
        loading={state.loading}
        error={state.error}
        emptyMessage="No plans configured -- Create a pricing plan to make quota packages available."
        onRetry={loadPlans}
        renderActions={(plan) => (
          <div className="flex justify-end gap-1">
            <Button type="button" variant="ghost" size="icon" aria-label={`Edit plan ${plan.name}`} onClick={() => dispatch({ type: 'OPEN_EDIT', plan })}>
              <RiPencilLine className="size-4" aria-hidden="true" />
            </Button>
            <Button type="button" variant="ghost" size="icon" aria-label={`Deactivate plan ${plan.name}`} onClick={() => dispatch({ type: 'CONFIRM_DEACTIVATE', plan })}>
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
        title={state.editingPlan ? 'Edit Plan' : 'Add Plan'}
        submitLabel={state.editingPlan ? 'Save Changes' : 'Create Plan'}
        onSubmit={handleSubmit}
        loading={state.formLoading}
        error={state.formError}
      >
        <div className="space-y-2">
          <label htmlFor="plan-name" className="text-sm font-medium">Name</label>
          <Input id="plan-name" value={state.form.name} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'name', value: event.target.value })} />
        </div>
        <div className="space-y-2">
          <label htmlFor="plan-description" className="text-sm font-medium">Description</label>
          <Input id="plan-description" value={state.form.description} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'description', value: event.target.value })} />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <label htmlFor="plan-price" className="text-sm font-medium">Price</label>
            <Input id="plan-price" type="number" value={state.form.price} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'price', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="plan-quota" className="text-sm font-medium">Quota Amount</label>
            <Input id="plan-quota" type="number" value={state.form.quotaAmount} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'quotaAmount', value: event.target.value })} />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <label htmlFor="plan-token-quota" className="text-sm font-medium">Token Quota</label>
            <Input id="plan-token-quota" type="number" value={state.form.tokenQuota} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'tokenQuota', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="plan-agent-limit" className="text-sm font-medium">Agent Limit</label>
            <Input id="plan-agent-limit" type="number" value={state.form.agentLimit} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'agentLimit', value: event.target.value })} />
          </div>
        </div>
        <div className="space-y-2">
          <label htmlFor="plan-request-token-cap" className="text-sm font-medium">Request Token Cap</label>
          <Input id="plan-request-token-cap" type="number" value={state.form.maxTokensPerRequest} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'maxTokensPerRequest', value: event.target.value })} />
        </div>
        <div className="space-y-2">
          <label htmlFor="plan-model-access" className="text-sm font-medium">Model Access</label>
          <Input id="plan-model-access" placeholder="gpt-4o, claude-3.5" value={state.form.modelAccess} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'modelAccess', value: event.target.value })} />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <label htmlFor="plan-duration" className="text-sm font-medium">Duration Days</label>
            <Input id="plan-duration" type="number" value={state.form.durationDays} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'durationDays', value: event.target.value })} />
          </div>
          <div className="space-y-2">
            <label htmlFor="plan-sort-order" className="text-sm font-medium">Sort Order</label>
            <Input id="plan-sort-order" type="number" value={state.form.sortOrder} onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'sortOrder', value: event.target.value })} />
          </div>
        </div>
        <label className="flex min-h-[44px] items-center gap-3 text-sm">
          <input
            type="checkbox"
            checked={state.form.isPublic}
            onChange={(event) => dispatch({ type: 'FORM_FIELD', field: 'isPublic', value: event.target.checked })}
            className="size-4"
          />
          Public plan
        </label>
      </DrawerForm>

      <ConfirmDialog
        open={state.confirmDeactivate !== null}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CONFIRM_DEACTIVATE', plan: null });
          }
        }}
        title="Deactivate Plan"
        description="Are you sure you want to deactivate this plan? Existing subscribers may keep their current entitlement, but new subscribers will not see it."
        confirmLabel="Deactivate Plan"
        onConfirm={() => void handleDeactivate()}
        variant="destructive"
      />
    </div>
  );
}
