import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiCheckLine, RiCloseLine, RiEdit2Line } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';

import { ConfirmDialog } from '../../components/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { DrawerForm } from '../../components/shared/DrawerForm';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { PublishedAgent } from '../../types/admin';

type ReviewsState = {
  reviews: PublishedAgent[];
  loading: boolean;
  error: string | null;
  statusFilter: string;
  confirmApprove: PublishedAgent | null;
  changesAgent: PublishedAgent | null;
  rejectAgent: PublishedAgent | null;
  rejectionReason: string;
  actionLoading: boolean;
  formError: string | null;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; reviews: PublishedAgent[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_STATUS'; value: string }
  | { type: 'CONFIRM_APPROVE'; agent: PublishedAgent | null }
  | { type: 'OPEN_CHANGES'; agent: PublishedAgent }
  | { type: 'CLOSE_CHANGES' }
  | { type: 'OPEN_REJECT'; agent: PublishedAgent }
  | { type: 'CLOSE_REJECT' }
  | { type: 'SET_REASON'; value: string }
  | { type: 'ACTION_START' }
  | { type: 'ACTION_DONE' }
  | { type: 'FORM_ERROR'; error: string };

const initialState: ReviewsState = {
  reviews: [],
  loading: true,
  error: null,
  statusFilter: 'pending_review',
  confirmApprove: null,
  changesAgent: null,
  rejectAgent: null,
  rejectionReason: '',
  actionLoading: false,
  formError: null,
};

function reducer(state: ReviewsState, action: Action): ReviewsState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, reviews: action.reviews };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_STATUS':
      return { ...state, statusFilter: action.value };
    case 'CONFIRM_APPROVE':
      return { ...state, confirmApprove: action.agent };
    case 'OPEN_CHANGES':
      return { ...state, changesAgent: action.agent, rejectionReason: '', formError: null };
    case 'CLOSE_CHANGES':
      return { ...state, changesAgent: null, rejectionReason: '', formError: null, actionLoading: false };
    case 'OPEN_REJECT':
      return { ...state, rejectAgent: action.agent, rejectionReason: '', formError: null };
    case 'CLOSE_REJECT':
      return { ...state, rejectAgent: null, rejectionReason: '', formError: null, actionLoading: false };
    case 'SET_REASON':
      return { ...state, rejectionReason: action.value };
    case 'ACTION_START':
      return { ...state, actionLoading: true, formError: null };
    case 'ACTION_DONE':
      return {
        ...state,
        actionLoading: false,
        changesAgent: null,
        confirmApprove: null,
        rejectAgent: null,
        rejectionReason: '',
        formError: null,
      };
    case 'FORM_ERROR':
      return { ...state, actionLoading: false, formError: action.error };
    default:
      return state;
  }
}

function agentStatus(agent: PublishedAgent) {
  if (agent.status === 'pending') {
    return { status: 'pending_review' as const, label: 'Pending Review' };
  }
  if (agent.status === 'draft') {
    return { status: 'pending' as const, label: 'Draft' };
  }
  if (agent.status === 'takedown') {
    return { status: 'rejected' as const, label: 'Takedown' };
  }
  return { status: agent.status, label: undefined };
}

function ratingLabel(agent: PublishedAgent) {
  const rating = agent.ratingAvg ?? agent.rating ?? 0;
  return `${rating.toFixed(1)} (${agent.ratingCount})`;
}

function money(value?: number) {
  return `$${(value ?? 0).toFixed(2)}`;
}

function pricingLabel(agent: PublishedAgent) {
  const pricingType = agent.pricingType ?? (agent.pricingAmount && agent.pricingAmount > 0 ? 'one_time' : 'free');
  return `Pricing: ${pricingType} ${money(agent.pricingAmount)}`;
}

function statusLabel(value?: string) {
  if (!value) {
    return '-';
  }
  const label = value.split('_').filter(Boolean).join(' ');
  return label.charAt(0).toUpperCase() + label.slice(1);
}

function utcDateTime(value?: string) {
  if (!value) {
    return '-';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const year = date.getUTCFullYear();
  const month = String(date.getUTCMonth() + 1).padStart(2, '0');
  const day = String(date.getUTCDate()).padStart(2, '0');
  const hours = String(date.getUTCHours()).padStart(2, '0');
  const minutes = String(date.getUTCMinutes()).padStart(2, '0');
  return `${year}-${month}-${day} ${hours}:${minutes} UTC`;
}

function reviewSLALabel(agent: PublishedAgent) {
  const sla = agent.reviewSLA;
  if (!sla) {
    return <span className="text-muted-foreground">No SLA metadata</span>;
  }
  return (
    <div className="min-w-64 space-y-1 text-xs text-muted-foreground">
      <div>{`Manual SLA: ${statusLabel(sla.manualSlaStatus)} by ${utcDateTime(sla.manualDeadlineAt)}`}</div>
      <div>{`Automated SLA: ${statusLabel(sla.automatedReviewSlaStatus)}`}</div>
      <div>{`Publisher tier: ${sla.publisherTier}`}</div>
    </div>
  );
}

function updatedAt(agent: PublishedAgent) {
  return new Date(agent.updatedAt ?? agent.createdAt).toLocaleDateString();
}

export function AdminReviewsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadReviews = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const result = await api.listReviews({
        status: state.statusFilter === 'all' ? undefined : state.statusFilter,
        limit: 100,
      });
      dispatch({ type: 'LOAD_SUCCESS', reviews: result.data });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.statusFilter]);

  useEffect(() => {
    void loadReviews();
  }, [loadReviews]);

  const handleApprove = async () => {
    if (!state.confirmApprove) {
      return;
    }
    dispatch({ type: 'ACTION_START' });
    await api.approveAgent(state.confirmApprove.id);
    dispatch({ type: 'ACTION_DONE' });
    await loadReviews();
  };

  const handleReject = async () => {
    if (!state.rejectAgent) {
      return;
    }
    const reason = state.rejectionReason.trim();
    if (!reason) {
      dispatch({ type: 'FORM_ERROR', error: 'A rejection reason is required.' });
      return;
    }
    dispatch({ type: 'ACTION_START' });
    try {
      await api.rejectAgent(state.rejectAgent.id, reason);
      dispatch({ type: 'ACTION_DONE' });
      await loadReviews();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to reject agent.' });
    }
  };

  const handleRequestChanges = async () => {
    if (!state.changesAgent) {
      return;
    }
    const reason = state.rejectionReason.trim();
    if (!reason) {
      dispatch({ type: 'FORM_ERROR', error: 'A change request reason is required.' });
      return;
    }
    dispatch({ type: 'ACTION_START' });
    try {
      await api.requestAgentChanges(state.changesAgent.id, reason);
      dispatch({ type: 'ACTION_DONE' });
      await loadReviews();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to request agent changes.' });
    }
  };

  const columns: DataTableColumn<PublishedAgent>[] = [
    { key: 'name', header: 'Agent', sortable: true },
    { key: 'ownerName', header: 'Owner', render: (agent) => agent.ownerName || agent.ownerID || agent.ownerId || '-' },
    { key: 'categoryName', header: 'Category', render: (agent) => agent.categoryName ?? agent.categoryID ?? agent.categoryId ?? '-' },
    { key: 'visibility', header: 'Visibility', render: (agent) => <StatusBadge status={agent.visibility} /> },
    {
      key: 'status',
      header: 'Status',
      render: (agent) => {
        const mappedStatus = agentStatus(agent);
        return <StatusBadge status={mappedStatus.status} label={mappedStatus.label} />;
      },
    },
    {
      key: 'commercialContext',
      header: 'Commercial Context',
      render: (agent) => (
        <div className="min-w-56 space-y-1 text-xs text-muted-foreground">
          <div>{pricingLabel(agent)}</div>
          <div>{`Visibility: ${agent.visibility}`}</div>
          <div>{`Governance status: ${agent.status}`}</div>
        </div>
      ),
    },
    { key: 'reviewSLA', header: 'Review SLA', render: reviewSLALabel },
    { key: 'ratingAvg', header: 'Rating', render: ratingLabel },
    { key: 'installCount', header: 'Installs', render: (agent) => agent.installCount.toLocaleString() },
    { key: 'updatedAt', header: 'Updated', render: updatedAt },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Review Queue</h1>
      </div>

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <select
          aria-label="Review status filter"
          value={state.statusFilter}
          onChange={(event) => dispatch({ type: 'SET_STATUS', value: event.target.value })}
          className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
        >
          <option value="pending_review">Pending review</option>
          <option value="pending">Pending</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
          <option value="all">All statuses</option>
        </select>
      </div>

      <DataTable
        columns={columns}
        data={state.reviews}
        loading={state.loading}
        error={state.error}
        emptyMessage="No agents waiting for review -- Submitted agents will appear here."
        onRetry={loadReviews}
        renderActions={(agent) => (
          <div className="flex justify-end gap-1">
            <Button type="button" variant="ghost" size="icon" aria-label={`Approve agent ${agent.name}`} onClick={() => dispatch({ type: 'CONFIRM_APPROVE', agent })}>
              <RiCheckLine className="size-4" aria-hidden="true" />
            </Button>
            <Button type="button" variant="ghost" size="icon" aria-label={`Request changes for agent ${agent.name}`} onClick={() => dispatch({ type: 'OPEN_CHANGES', agent })}>
              <RiEdit2Line className="size-4" aria-hidden="true" />
            </Button>
            <Button type="button" variant="ghost" size="icon" aria-label={`Reject agent ${agent.name}`} onClick={() => dispatch({ type: 'OPEN_REJECT', agent })}>
              <RiCloseLine className="size-4" aria-hidden="true" />
            </Button>
          </div>
        )}
      />

      <ConfirmDialog
        open={state.confirmApprove !== null}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CONFIRM_APPROVE', agent: null });
          }
        }}
        title="Approve Agent"
        description="Approve this agent for marketplace discovery and installation."
        confirmLabel="Approve Agent"
        onConfirm={() => void handleApprove()}
        variant="default"
        loading={state.actionLoading}
      />

      <DrawerForm
        open={state.changesAgent !== null}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CLOSE_CHANGES' });
          }
        }}
        title={state.changesAgent ? `Request Changes: ${state.changesAgent.name}` : 'Request Changes'}
        submitLabel="Request Changes"
        onSubmit={handleRequestChanges}
        loading={state.actionLoading}
        error={state.formError}
      >
        <div className="space-y-2">
          <label htmlFor="changes-reason" className="text-sm font-medium">Change Request Reason</label>
          <Textarea
            id="changes-reason"
            value={state.rejectionReason}
            onChange={(event) => dispatch({ type: 'SET_REASON', value: event.target.value })}
            placeholder="Describe the supplemental material or fixes required before approval."
          />
        </div>
      </DrawerForm>

      <DrawerForm
        open={state.rejectAgent !== null}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CLOSE_REJECT' });
          }
        }}
        title={state.rejectAgent ? `Reject Agent: ${state.rejectAgent.name}` : 'Reject Agent'}
        submitLabel="Reject Agent"
        onSubmit={handleReject}
        loading={state.actionLoading}
        error={state.formError}
      >
        <div className="space-y-2">
          <label htmlFor="reject-reason" className="text-sm font-medium">Rejection Reason</label>
          <Textarea
            id="reject-reason"
            value={state.rejectionReason}
            onChange={(event) => dispatch({ type: 'SET_REASON', value: event.target.value })}
            placeholder="Explain what the publisher needs to fix."
          />
        </div>
      </DrawerForm>
    </div>
  );
}
