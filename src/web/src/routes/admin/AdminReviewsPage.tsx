import { useCallback, useEffect, useMemo, useReducer } from 'react';
import { RiAlarmWarningLine, RiCheckLine, RiCloseLine, RiEdit2Line, RiShieldCheckLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

import { ConfirmDialog } from '../../components/shared/ConfirmDialog';
import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { DrawerForm } from '../../components/shared/DrawerForm';
import { StatusBadge } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { MarketplaceAbuseReport, PublishedAgent } from '../../types/admin';

type AbuseReportAction = {
  action: 'dismiss' | 'resolve';
  report: MarketplaceAbuseReport;
};

type GovernanceAction = 'reinstate' | 'reject_appeal' | 'takedown';

type ReviewsState = {
  reviews: PublishedAgent[];
  loading: boolean;
  error: string | null;
  statusFilter: string;
  slaLoading: boolean;
  slaMessage: string | null;
  slaError: string | null;
  abuseReports: MarketplaceAbuseReport[];
  abuseLoading: boolean;
  abuseError: string | null;
  abuseStatusFilter: string;
  abuseAction: AbuseReportAction | null;
  abuseResolution: string;
  abuseActionLoading: boolean;
  abuseFormError: string | null;
  abuseActionMessage: string | null;
  governanceAgentID: string;
  governanceAction: GovernanceAction;
  governanceReason: string;
  governanceLoading: boolean;
  governanceMessage: string | null;
  governanceError: string | null;
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
  | { type: 'SLA_START' }
  | { type: 'SLA_DONE'; message: string }
  | { type: 'SLA_ERROR'; error: string }
  | { type: 'ABUSE_LOAD_START' }
  | { type: 'ABUSE_LOAD_SUCCESS'; reports: MarketplaceAbuseReport[] }
  | { type: 'ABUSE_LOAD_ERROR'; error: string }
  | { type: 'SET_ABUSE_STATUS'; value: string }
  | { type: 'OPEN_ABUSE_ACTION'; report: MarketplaceAbuseReport; action: 'dismiss' | 'resolve' }
  | { type: 'CLOSE_ABUSE_ACTION' }
  | { type: 'SET_ABUSE_RESOLUTION'; value: string }
  | { type: 'ABUSE_ACTION_START' }
  | { type: 'ABUSE_ACTION_DONE'; message: string }
  | { type: 'ABUSE_FORM_ERROR'; error: string }
  | { type: 'SET_GOVERNANCE_AGENT_ID'; value: string }
  | { type: 'SET_GOVERNANCE_ACTION'; value: GovernanceAction }
  | { type: 'SET_GOVERNANCE_REASON'; value: string }
  | { type: 'GOVERNANCE_START' }
  | { type: 'GOVERNANCE_DONE'; message: string }
  | { type: 'GOVERNANCE_ERROR'; error: string }
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
  slaLoading: false,
  slaMessage: null,
  slaError: null,
  abuseReports: [],
  abuseLoading: true,
  abuseError: null,
  abuseStatusFilter: 'open',
  abuseAction: null,
  abuseResolution: '',
  abuseActionLoading: false,
  abuseFormError: null,
  abuseActionMessage: null,
  governanceAgentID: '',
  governanceAction: 'takedown',
  governanceReason: '',
  governanceLoading: false,
  governanceMessage: null,
  governanceError: null,
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
    case 'SLA_START':
      return { ...state, slaLoading: true, slaMessage: null, slaError: null };
    case 'SLA_DONE':
      return { ...state, slaLoading: false, slaMessage: action.message, slaError: null };
    case 'SLA_ERROR':
      return { ...state, slaLoading: false, slaError: action.error };
    case 'ABUSE_LOAD_START':
      return { ...state, abuseLoading: true, abuseError: null };
    case 'ABUSE_LOAD_SUCCESS':
      return { ...state, abuseLoading: false, abuseError: null, abuseReports: action.reports };
    case 'ABUSE_LOAD_ERROR':
      return { ...state, abuseLoading: false, abuseError: action.error };
    case 'SET_ABUSE_STATUS':
      return { ...state, abuseStatusFilter: action.value };
    case 'OPEN_ABUSE_ACTION':
      return { ...state, abuseAction: { action: action.action, report: action.report }, abuseResolution: '', abuseFormError: null };
    case 'CLOSE_ABUSE_ACTION':
      return { ...state, abuseAction: null, abuseResolution: '', abuseFormError: null, abuseActionLoading: false };
    case 'SET_ABUSE_RESOLUTION':
      return { ...state, abuseResolution: action.value };
    case 'ABUSE_ACTION_START':
      return { ...state, abuseActionLoading: true, abuseFormError: null, abuseActionMessage: null };
    case 'ABUSE_ACTION_DONE':
      return {
        ...state,
        abuseAction: null,
        abuseResolution: '',
        abuseActionLoading: false,
        abuseFormError: null,
        abuseActionMessage: action.message,
      };
    case 'ABUSE_FORM_ERROR':
      return { ...state, abuseActionLoading: false, abuseFormError: action.error };
    case 'SET_GOVERNANCE_AGENT_ID':
      return { ...state, governanceAgentID: action.value, governanceError: null, governanceMessage: null };
    case 'SET_GOVERNANCE_ACTION':
      return { ...state, governanceAction: action.value, governanceError: null, governanceMessage: null };
    case 'SET_GOVERNANCE_REASON':
      return { ...state, governanceReason: action.value, governanceError: null, governanceMessage: null };
    case 'GOVERNANCE_START':
      return { ...state, governanceLoading: true, governanceError: null, governanceMessage: null };
    case 'GOVERNANCE_DONE':
      return {
        ...state,
        governanceLoading: false,
        governanceReason: '',
        governanceMessage: action.message,
        governanceError: null,
      };
    case 'GOVERNANCE_ERROR':
      return { ...state, governanceLoading: false, governanceError: action.error };
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
  if (agent.status === 'appeal_pending') {
    return { status: 'appeal_pending' as const, label: 'Appeal Pending' };
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

  const loadAbuseReports = useCallback(async () => {
    dispatch({ type: 'ABUSE_LOAD_START' });
    try {
      const result = await api.listMarketplaceAbuseReports({
        status: state.abuseStatusFilter === 'all' ? undefined : state.abuseStatusFilter,
        limit: 50,
      });
      dispatch({ type: 'ABUSE_LOAD_SUCCESS', reports: result.data });
    } catch (error) {
      dispatch({ type: 'ABUSE_LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.abuseStatusFilter]);

  useEffect(() => {
    void loadAbuseReports();
  }, [loadAbuseReports]);

  const handleApprove = async () => {
    if (!state.confirmApprove) {
      return;
    }
    dispatch({ type: 'ACTION_START' });
    await api.approveAgent(state.confirmApprove.id);
    dispatch({ type: 'ACTION_DONE' });
    await loadReviews();
  };

  const handleClaimReview = async (agent: PublishedAgent) => {
    dispatch({ type: 'ACTION_START' });
    try {
      await api.claimReview(agent.id);
      dispatch({ type: 'ACTION_DONE' });
      await loadReviews();
    } catch (error) {
      dispatch({ type: 'FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to claim marketplace review.' });
    }
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

  const handleEnforceSLA = async () => {
    dispatch({ type: 'SLA_START' });
    try {
      const result = await api.enforceReviewSLA({ limit: 100 });
      dispatch({ type: 'SLA_DONE', message: `Review SLA scan complete: ${result.scanned} scanned, ${result.alerted} alerted.` });
      await loadReviews();
    } catch (error) {
      dispatch({ type: 'SLA_ERROR', error: error instanceof Error ? error.message : 'Unable to enforce review SLA.' });
    }
  };

  const handleAbuseReportAction = async () => {
    if (!state.abuseAction) {
      return;
    }
    const resolution = state.abuseResolution.trim();
    if (!resolution) {
      dispatch({ type: 'ABUSE_FORM_ERROR', error: 'Resolution is required.' });
      return;
    }
    dispatch({ type: 'ABUSE_ACTION_START' });
    try {
      if (state.abuseAction.action === 'resolve') {
        await api.resolveMarketplaceAbuseReport(state.abuseAction.report.id, resolution);
        dispatch({ type: 'ABUSE_ACTION_DONE', message: 'Abuse report resolved.' });
      } else {
        await api.dismissMarketplaceAbuseReport(state.abuseAction.report.id, resolution);
        dispatch({ type: 'ABUSE_ACTION_DONE', message: 'Abuse report dismissed.' });
      }
      await loadAbuseReports();
    } catch (error) {
      dispatch({ type: 'ABUSE_FORM_ERROR', error: error instanceof Error ? error.message : 'Unable to update abuse report.' });
    }
  };

  const handleGovernanceAction = async () => {
    const agentID = state.governanceAgentID.trim();
    const reason = state.governanceReason.trim();
    if (!agentID) {
      dispatch({ type: 'GOVERNANCE_ERROR', error: 'Agent ID is required.' });
      return;
    }
    if (!reason) {
      dispatch({ type: 'GOVERNANCE_ERROR', error: 'Governance reason is required.' });
      return;
    }

    dispatch({ type: 'GOVERNANCE_START' });
    try {
      if (state.governanceAction === 'takedown') {
        await api.takedownMarketplaceAgent(agentID, reason);
        dispatch({ type: 'GOVERNANCE_DONE', message: 'Marketplace agent taken down.' });
      } else if (state.governanceAction === 'reinstate') {
        await api.reinstateMarketplaceAgent(agentID, reason);
        dispatch({ type: 'GOVERNANCE_DONE', message: 'Marketplace agent reinstated.' });
      } else {
        await api.rejectMarketplaceAgentAppeal(agentID, reason);
        dispatch({ type: 'GOVERNANCE_DONE', message: 'Marketplace agent appeal rejected.' });
      }
    } catch (error) {
      dispatch({ type: 'GOVERNANCE_ERROR', error: error instanceof Error ? error.message : 'Unable to update marketplace governance.' });
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
            <div>{`Reviewer: ${agent.reviewerUserId || 'Unassigned'}`}</div>
          </div>
        ),
    },
    { key: 'reviewSLA', header: 'Review SLA', render: reviewSLALabel },
    { key: 'ratingAvg', header: 'Rating', render: ratingLabel },
    { key: 'installCount', header: 'Installs', render: (agent) => agent.installCount.toLocaleString() },
    { key: 'updatedAt', header: 'Updated', render: updatedAt },
  ];

  const abuseColumns: DataTableColumn<MarketplaceAbuseReport>[] = [
    { key: 'agentId', header: 'Agent', render: (report) => report.agentId },
    {
      key: 'reason',
      header: 'Reason',
      render: (report) => (
        <div className="min-w-64 space-y-1">
          <div>{report.reason}</div>
          {report.details ? <div className="text-xs text-muted-foreground">{report.details}</div> : null}
        </div>
      ),
    },
    {
      key: 'reporterUserId',
      header: 'Reporter',
      render: (report) => (
        <div className="min-w-48 space-y-1 text-xs text-muted-foreground">
          <div>{`User: ${report.reporterUserId}`}</div>
          <div>{`Org: ${report.reporterOrganizationId}`}</div>
        </div>
      ),
    },
    { key: 'status', header: 'Status', render: (report) => <StatusBadge status={report.status} /> },
    { key: 'updatedAt', header: 'Updated', render: (report) => utcDateTime(report.updatedAt) },
  ];

  const abuseActionTitle = state.abuseAction
    ? `${state.abuseAction.action === 'resolve' ? 'Resolve' : 'Dismiss'} Abuse Report: ${state.abuseAction.report.id}`
    : 'Update Abuse Report';
  const abuseSubmitLabel = state.abuseAction?.action === 'resolve' ? 'Resolve Report' : 'Dismiss Report';

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Review Queue</h1>
        <Button type="button" variant="outline" className="min-h-[44px]" disabled={state.slaLoading} onClick={() => void handleEnforceSLA()}>
          <RiAlarmWarningLine className="size-4" aria-hidden="true" />
          {state.slaLoading ? 'Enforcing SLA' : 'Enforce SLA'}
        </Button>
      </div>

      {state.slaMessage ? (
        <p className="rounded-lg border border-border bg-card px-4 py-3 text-sm text-foreground">{state.slaMessage}</p>
      ) : null}
      {state.slaError ? (
        <p className="rounded-lg border border-destructive/30 bg-card px-4 py-3 text-sm text-destructive" role="alert">{state.slaError}</p>
      ) : null}

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <select
          aria-label="Review status filter"
          value={state.statusFilter}
          onChange={(event) => dispatch({ type: 'SET_STATUS', value: event.target.value })}
          className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
        >
          <option value="pending_review">Pending review</option>
          <option value="appeal_pending">Appeal pending</option>
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
            <Button type="button" variant="ghost" size="icon" aria-label={`Claim review ${agent.name}`} onClick={() => void handleClaimReview(agent)}>
              <RiShieldCheckLine className="size-4" aria-hidden="true" />
            </Button>
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

      <section className="space-y-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <h2 className="font-heading text-xl font-semibold text-foreground">Marketplace Abuse Reports</h2>
          <select
            aria-label="Abuse report status filter"
            value={state.abuseStatusFilter}
            onChange={(event) => dispatch({ type: 'SET_ABUSE_STATUS', value: event.target.value })}
            className="min-h-[44px] rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
          >
            <option value="open">Open</option>
            <option value="resolved">Resolved</option>
            <option value="dismissed">Dismissed</option>
            <option value="all">All statuses</option>
          </select>
        </div>

        {state.abuseActionMessage ? (
          <p className="rounded-lg border border-border bg-card px-4 py-3 text-sm text-foreground">{state.abuseActionMessage}</p>
        ) : null}

        <DataTable
          columns={abuseColumns}
          data={state.abuseReports}
          loading={state.abuseLoading}
          error={state.abuseError}
          emptyMessage="No marketplace abuse reports match this filter."
          onRetry={loadAbuseReports}
          renderActions={(report) =>
            report.status === 'open' ? (
              <div className="flex justify-end gap-1">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={`Resolve abuse report ${report.id}`}
                  onClick={() => dispatch({ type: 'OPEN_ABUSE_ACTION', report, action: 'resolve' })}
                >
                  <RiCheckLine className="size-4" aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={`Dismiss abuse report ${report.id}`}
                  onClick={() => dispatch({ type: 'OPEN_ABUSE_ACTION', report, action: 'dismiss' })}
                >
                  <RiCloseLine className="size-4" aria-hidden="true" />
                </Button>
              </div>
            ) : null
          }
        />
      </section>

      <section className="space-y-4 rounded-lg border border-border bg-card p-4">
        <h2 className="font-heading text-xl font-semibold text-foreground">Marketplace Governance</h2>

        {state.governanceMessage ? (
          <p className="rounded-lg border border-border bg-background px-4 py-3 text-sm text-foreground">{state.governanceMessage}</p>
        ) : null}
        {state.governanceError ? (
          <p className="rounded-lg border border-destructive/30 bg-background px-4 py-3 text-sm text-destructive" role="alert">{state.governanceError}</p>
        ) : null}

        <div className="grid gap-4 lg:grid-cols-[minmax(180px,1fr)_180px]">
          <div className="space-y-2">
            <label htmlFor="governance-agent-id" className="text-sm font-medium">Agent ID</label>
            <Input
              id="governance-agent-id"
              className="min-h-[44px] rounded-lg"
              value={state.governanceAgentID}
              onChange={(event) => dispatch({ type: 'SET_GOVERNANCE_AGENT_ID', value: event.target.value })}
            />
          </div>
          <div className="space-y-2">
            <label htmlFor="governance-action" className="text-sm font-medium">Action</label>
            <select
              id="governance-action"
              aria-label="Governance action"
              className="min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 text-sm text-foreground"
              value={state.governanceAction}
              onChange={(event) => dispatch({ type: 'SET_GOVERNANCE_ACTION', value: event.target.value as GovernanceAction })}
            >
              <option value="takedown">Takedown</option>
              <option value="reinstate">Reinstate</option>
              <option value="reject_appeal">Reject appeal</option>
            </select>
          </div>
          <div className="space-y-2 lg:col-span-2">
            <label htmlFor="governance-reason" className="text-sm font-medium">Reason</label>
            <Textarea
              id="governance-reason"
              value={state.governanceReason}
              onChange={(event) => dispatch({ type: 'SET_GOVERNANCE_REASON', value: event.target.value })}
            />
          </div>
        </div>

        <Button type="button" className="min-h-[44px]" disabled={state.governanceLoading} onClick={() => void handleGovernanceAction()}>
          <RiShieldCheckLine className="size-4" aria-hidden="true" />
          {state.governanceLoading ? 'Applying Governance' : 'Apply Governance'}
        </Button>
      </section>

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

      <DrawerForm
        open={state.abuseAction !== null}
        onOpenChange={(open) => {
          if (!open) {
            dispatch({ type: 'CLOSE_ABUSE_ACTION' });
          }
        }}
        title={abuseActionTitle}
        submitLabel={abuseSubmitLabel}
        onSubmit={handleAbuseReportAction}
        loading={state.abuseActionLoading}
        error={state.abuseFormError}
      >
        <div className="space-y-2">
          <label htmlFor="abuse-resolution" className="text-sm font-medium">Resolution</label>
          <Textarea
            id="abuse-resolution"
            value={state.abuseResolution}
            onChange={(event) => dispatch({ type: 'SET_ABUSE_RESOLUTION', value: event.target.value })}
            placeholder="Record the moderation decision and follow-up evidence."
          />
        </div>
      </DrawerForm>
    </div>
  );
}
