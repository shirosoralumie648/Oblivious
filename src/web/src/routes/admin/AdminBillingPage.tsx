import { useCallback, useEffect, useMemo, useReducer } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { StatusBadge, type StatusBadgeStatus } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { BillingFilter, BillingInspectionRecord, BillingSummary, BillingSurface, BillingSummaryMetric } from '../../types/admin';

type SurfaceConfig = {
  id: BillingSurface;
  label: string;
  columns: DataTableColumn<BillingInspectionRecord>[];
};

type PayoutPaidDraft = {
  payoutId: string;
  providerPayoutId: string;
};

type PayoutFailedDraft = {
  payoutId: string;
  providerPayoutId: string;
  reason: string;
};

type TopupRefundDraft = {
  topupId: string;
  provider: string;
  providerRefundID: string;
  providerChargeID: string;
  providerPaymentIntentID: string;
  amount: string;
  refundableAmount: number;
  currency: string;
  reason: string;
};

type BillingState = {
  summary: BillingSummary;
  rows: BillingInspectionRecord[];
  total: number;
  loading: boolean;
  error: string | null;
  actionError: string | null;
  actioningRecordId: string | null;
  payoutPaidDraft: PayoutPaidDraft | null;
  payoutFailedDraft: PayoutFailedDraft | null;
  topupRefundDraft: TopupRefundDraft | null;
  surface: BillingSurface;
  filters: {
    organizationID: string;
    userID: string;
    status: string;
    kind: string;
    provider: string;
  };
};

type BillingAction =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; summary: BillingSummary; rows: BillingInspectionRecord[]; total: number }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'ACTION_START'; recordId: string }
  | { type: 'ACTION_DONE' }
  | { type: 'ACTION_ERROR'; error: string }
  | { type: 'OPEN_PAYOUT_PAID'; record: BillingInspectionRecord }
  | { type: 'OPEN_PAYOUT_FAILED'; record: BillingInspectionRecord }
  | { type: 'OPEN_TOPUP_REFUND'; record: BillingInspectionRecord }
  | { type: 'CLOSE_ACTION_FORM' }
  | { type: 'SET_PAYOUT_FIELD'; field: keyof PayoutPaidDraft; value: string }
  | { type: 'SET_PAYOUT_FAILED_FIELD'; field: keyof PayoutFailedDraft; value: string }
  | { type: 'SET_TOPUP_REFUND_FIELD'; field: keyof TopupRefundDraft; value: string }
  | { type: 'SET_SURFACE'; surface: BillingSurface }
  | { type: 'SET_FILTER'; field: keyof BillingState['filters']; value: string }
  | { type: 'OPEN_FAILED_WEBHOOKS' };

const initialState: BillingState = {
  summary: {},
  rows: [],
  total: 0,
  loading: true,
  error: null,
  actionError: null,
  actioningRecordId: null,
  payoutPaidDraft: null,
  payoutFailedDraft: null,
  topupRefundDraft: null,
  surface: 'sessions',
  filters: {
    organizationID: '',
    userID: '',
    status: '',
    kind: '',
    provider: '',
  },
};

function payoutPaidDraftFromRecord(record: BillingInspectionRecord): PayoutPaidDraft {
  return {
    payoutId: record.id,
    providerPayoutId: record.providerPayoutId ?? '',
  };
}

function payoutFailedDraftFromRecord(record: BillingInspectionRecord): PayoutFailedDraft {
  return {
    payoutId: record.id,
    providerPayoutId: record.providerPayoutId ?? '',
    reason: '',
  };
}

function topupRefundDraftFromRecord(record: BillingInspectionRecord): TopupRefundDraft {
  const refundableAmount = Math.max((record.amount ?? record.money ?? 0) - (record.refundedAmount ?? 0), 0);
  return {
    topupId: record.id,
    provider: record.provider ?? '',
    providerRefundID: '',
    providerChargeID: record.providerChargeId ?? '',
    providerPaymentIntentID: record.providerPaymentIntentId ?? '',
    amount: refundableAmount.toFixed(2),
    refundableAmount,
    currency: record.currency ?? '',
    reason: 'admin recorded provider refund',
  };
}

function reducer(state: BillingState, action: BillingAction): BillingState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, summary: action.summary, rows: action.rows, total: action.total };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'ACTION_START':
      return { ...state, actionError: null, actioningRecordId: action.recordId };
    case 'ACTION_DONE':
      return { ...state, actioningRecordId: null, payoutPaidDraft: null, payoutFailedDraft: null, topupRefundDraft: null };
    case 'ACTION_ERROR':
      return { ...state, actioningRecordId: null, actionError: action.error };
    case 'OPEN_PAYOUT_PAID':
      return {
        ...state,
        actionError: null,
        payoutPaidDraft: payoutPaidDraftFromRecord(action.record),
        payoutFailedDraft: null,
        topupRefundDraft: null,
      };
    case 'OPEN_PAYOUT_FAILED':
      return {
        ...state,
        actionError: null,
        payoutPaidDraft: null,
        payoutFailedDraft: payoutFailedDraftFromRecord(action.record),
        topupRefundDraft: null,
      };
    case 'OPEN_TOPUP_REFUND':
      return {
        ...state,
        actionError: null,
        payoutPaidDraft: null,
        payoutFailedDraft: null,
        topupRefundDraft: topupRefundDraftFromRecord(action.record),
      };
    case 'CLOSE_ACTION_FORM':
      return { ...state, actionError: null, payoutPaidDraft: null, payoutFailedDraft: null, topupRefundDraft: null };
    case 'SET_PAYOUT_FIELD':
      return state.payoutPaidDraft
        ? { ...state, payoutPaidDraft: { ...state.payoutPaidDraft, [action.field]: action.value } }
        : state;
    case 'SET_PAYOUT_FAILED_FIELD':
      return state.payoutFailedDraft
        ? { ...state, payoutFailedDraft: { ...state.payoutFailedDraft, [action.field]: action.value } }
        : state;
    case 'SET_TOPUP_REFUND_FIELD':
      return state.topupRefundDraft
        ? { ...state, topupRefundDraft: { ...state.topupRefundDraft, [action.field]: action.value } }
        : state;
    case 'SET_SURFACE':
      return { ...state, surface: action.surface, actionError: null, payoutPaidDraft: null, payoutFailedDraft: null, topupRefundDraft: null };
    case 'SET_FILTER':
      return { ...state, filters: { ...state.filters, [action.field]: action.value }, actionError: null, payoutPaidDraft: null, payoutFailedDraft: null, topupRefundDraft: null };
    case 'OPEN_FAILED_WEBHOOKS':
      return {
        ...state,
        surface: 'webhookEvents',
        filters: { ...state.filters, status: 'failed' },
        actionError: null,
        payoutPaidDraft: null,
        payoutFailedDraft: null,
        topupRefundDraft: null,
      };
    default:
      return state;
  }
}

function money(value?: number) {
  return `$${(value ?? 0).toFixed(2)}`;
}

function numberLabel(value?: number) {
  return (value ?? 0).toLocaleString();
}

function dateLabel(value?: string | null) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleDateString();
}

function idCell(value?: string) {
  return <span className="break-all font-mono text-xs">{value || '-'}</span>;
}

const billingStatusTone: Record<string, StatusBadgeStatus> = {
  active: 'active',
  approved: 'approved',
  completed: 'approved',
  paid: 'approved',
  processed: 'approved',
  settled: 'approved',
  succeeded: 'approved',
  pending: 'pending',
  payout_pending: 'pending',
  processing: 'pending',
  failed: 'rejected',
  canceled: 'disabled',
  cancelled: 'disabled',
  disabled: 'disabled',
  partially_refunded: 'degraded',
  refunded: 'degraded',
};

function billingStatusLabel(status: string) {
  return status
    .split('_')
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join(' ');
}

function statusCell(record: BillingInspectionRecord) {
  if (!record.status) {
    return '-';
  }
  return <StatusBadge status={billingStatusTone[record.status] ?? 'pending'} label={billingStatusLabel(record.status)} />;
}

function amountCell(record: BillingInspectionRecord) {
  const value = record.amount ?? record.money ?? record.settledAmount ?? record.amountPaid ?? record.publisherNetAmount;
  return money(value);
}

const surfaces: SurfaceConfig[] = [
  {
    id: 'sessions',
    label: 'Billing Sessions',
    columns: [
      { key: 'id', header: 'Session', render: (row) => idCell(row.id) },
      { key: 'model', header: 'Model', render: (row) => row.model || '-' },
      { key: 'apiType', header: 'API Type', render: (row) => row.apiType || '-' },
      { key: 'settledAmount', header: 'Settled', render: (row) => money(row.settledAmount) },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
  {
    id: 'paymentIntents',
    label: 'Payment Intents',
    columns: [
      { key: 'id', header: 'Intent', render: (row) => idCell(row.id) },
      { key: 'kind', header: 'Kind', render: (row) => row.kind || '-' },
      { key: 'amount', header: 'Amount', render: amountCell },
      { key: 'refundedAmount', header: 'Refunded', render: (row) => money(row.refundedAmount) },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
  {
    id: 'webhookEvents',
    label: 'Webhooks',
    columns: [
      { key: 'eventId', header: 'Event', render: (row) => idCell(row.eventId ?? row.id) },
      { key: 'eventType', header: 'Type', render: (row) => row.eventType || '-' },
      { key: 'provider', header: 'Provider', render: (row) => row.provider || '-' },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'error', header: 'Error', render: (row) => row.error || '-' },
      { key: 'receivedAt', header: 'Received', render: (row) => dateLabel(row.receivedAt) },
    ],
  },
  {
    id: 'subscriptions',
    label: 'Subscriptions',
    columns: [
      { key: 'id', header: 'Subscription', render: (row) => idCell(row.id) },
      { key: 'packageId', header: 'Package', render: (row) => row.packageId || '-' },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'providerSubscriptionId', header: 'Provider ID', render: (row) => idCell(row.providerSubscriptionId) },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
  {
    id: 'topups',
    label: 'Top-ups',
    columns: [
      { key: 'id', header: 'Top-up', render: (row) => idCell(row.id) },
      { key: 'amount', header: 'Credits', render: (row) => numberLabel(row.amount) },
      { key: 'money', header: 'Money', render: amountCell },
      { key: 'refundedAmount', header: 'Refunded', render: (row) => money(row.refundedAmount) },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
  {
    id: 'invoices',
    label: 'Invoices',
    columns: [
      { key: 'id', header: 'Invoice', render: (row) => idCell(row.id) },
      { key: 'providerInvoiceId', header: 'Provider ID', render: (row) => idCell(row.providerInvoiceId) },
      { key: 'amountDue', header: 'Due', render: (row) => money(row.amountDue) },
      { key: 'amountPaid', header: 'Paid', render: (row) => money(row.amountPaid) },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
  {
    id: 'refunds',
    label: 'Refunds',
    columns: [
      { key: 'id', header: 'Refund', render: (row) => idCell(row.id) },
      { key: 'providerRefundId', header: 'Provider ID', render: (row) => idCell(row.providerRefundId) },
      { key: 'reason', header: 'Reason', render: (row) => row.reason || '-' },
      { key: 'paymentIntentId', header: 'Payment Intent', render: (row) => idCell(row.paymentIntentId) },
      { key: 'topupOrderId', header: 'Top-up Order', render: (row) => idCell(row.topupOrderId) },
      { key: 'amount', header: 'Amount', render: amountCell },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
  {
    id: 'settlements',
    label: 'Settlements',
    columns: [
      { key: 'id', header: 'Settlement', render: (row) => idCell(row.id) },
      { key: 'agentId', header: 'Agent', render: (row) => idCell(row.agentId) },
      { key: 'grossAmount', header: 'Gross', render: (row) => money(row.grossAmount) },
      { key: 'platformFeeAmount', header: 'Fee', render: (row) => money(row.platformFeeAmount) },
      { key: 'publisherNetAmount', header: 'Net', render: (row) => money(row.publisherNetAmount) },
      { key: 'status', header: 'Status', render: statusCell },
    ],
  },
  {
    id: 'payouts',
    label: 'Payouts',
    columns: [
      { key: 'id', header: 'Payout', render: (row) => idCell(row.id) },
      { key: 'provider', header: 'Provider', render: (row) => row.provider || '-' },
      { key: 'providerPayoutId', header: 'Provider ID', render: (row) => idCell(row.providerPayoutId) },
      { key: 'amount', header: 'Amount', render: amountCell },
      { key: 'status', header: 'Status', render: statusCell },
      { key: 'createdAt', header: 'Created', render: (row) => dateLabel(row.createdAt) },
    ],
  },
];

const summaryTiles: Array<{ label: string; metric: keyof BillingSummary; value: (metric?: BillingSummaryMetric) => string; subvalue?: (metric?: BillingSummaryMetric) => string }> = [
  { label: 'Billing Sessions', metric: 'billingSessions', value: (metric) => numberLabel(metric?.count), subvalue: (metric) => money(metric?.settledAmount) },
  { label: 'Payments', metric: 'paymentIntents', value: (metric) => money(metric?.totalAmount), subvalue: (metric) => `${numberLabel(metric?.count)} intents` },
  { label: 'Webhooks', metric: 'webhookEvents', value: (metric) => numberLabel(metric?.count), subvalue: (metric) => `${numberLabel(metric?.failedCount)} failed` },
  { label: 'Marketplace Net', metric: 'settlements', value: (metric) => money(metric?.publisherNetAmount), subvalue: (metric) => money(metric?.grossAmount) },
  { label: 'Payouts', metric: 'payouts', value: (metric) => money(metric?.totalAmount), subvalue: (metric) => `${numberLabel(metric?.count)} records` },
];

function currentSurface(id: BillingSurface) {
  return surfaces.find((surface) => surface.id === id) ?? surfaces[0];
}

export function AdminBillingPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);
  const activeSurface = currentSurface(state.surface);

  const loadBilling = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    const filters: BillingFilter = {
      organizationID: state.filters.organizationID,
      userID: state.filters.userID,
      status: state.filters.status,
      kind: state.filters.kind,
      provider: state.filters.provider,
      limit: 50,
    };

    try {
      const [summary, result] = await Promise.all([
        api.getBillingSummary(filters),
        api.listBillingSurface(state.surface, filters),
      ]);
      dispatch({ type: 'LOAD_SUCCESS', summary, rows: result.data, total: result.total });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [api, state.filters.kind, state.filters.organizationID, state.filters.provider, state.filters.status, state.filters.userID, state.surface]);

  useEffect(() => {
    void loadBilling();
  }, [loadBilling]);

  const handleMarkPayoutPaid = async () => {
    if (!state.payoutPaidDraft) {
      return;
    }
    const providerPayoutId = state.payoutPaidDraft.providerPayoutId.trim();
    if (!providerPayoutId) {
      dispatch({ type: 'ACTION_ERROR', error: 'Provider payout ID is required.' });
      return;
    }
    dispatch({ type: 'ACTION_START', recordId: state.payoutPaidDraft.payoutId });
    try {
      await api.markMarketplacePayoutPaid(state.payoutPaidDraft.payoutId, providerPayoutId);
      dispatch({ type: 'ACTION_DONE' });
      await loadBilling();
    } catch (error) {
      dispatch({ type: 'ACTION_ERROR', error: error instanceof Error ? error.message : 'Unable to mark payout paid.' });
    }
  };

  const handleMarkPayoutFailed = async () => {
    if (!state.payoutFailedDraft) {
      return;
    }
    const providerPayoutId = state.payoutFailedDraft.providerPayoutId.trim();
    const reason = state.payoutFailedDraft.reason.trim();
    if (!providerPayoutId) {
      dispatch({ type: 'ACTION_ERROR', error: 'Provider payout ID is required.' });
      return;
    }
    if (!reason) {
      dispatch({ type: 'ACTION_ERROR', error: 'Failure reason is required.' });
      return;
    }
    dispatch({ type: 'ACTION_START', recordId: state.payoutFailedDraft.payoutId });
    try {
      await api.markMarketplacePayoutFailed(state.payoutFailedDraft.payoutId, providerPayoutId, reason);
      dispatch({ type: 'ACTION_DONE' });
      await loadBilling();
    } catch (error) {
      dispatch({ type: 'ACTION_ERROR', error: error instanceof Error ? error.message : 'Unable to mark payout failed.' });
    }
  };

  const handleRefundTopup = async () => {
    if (!state.topupRefundDraft) {
      return;
    }
    const providerRefundID = state.topupRefundDraft.providerRefundID.trim();
    const provider = state.topupRefundDraft.provider.trim();
    const providerChargeID = state.topupRefundDraft.providerChargeID.trim();
    const providerPaymentIntentID = state.topupRefundDraft.providerPaymentIntentID.trim();
    const currency = state.topupRefundDraft.currency.trim();
    const amount = Number(state.topupRefundDraft.amount);
    if (!provider) {
      dispatch({ type: 'ACTION_ERROR', error: 'Provider is required.' });
      return;
    }
    if (!providerRefundID) {
      dispatch({ type: 'ACTION_ERROR', error: 'Provider refund ID is required.' });
      return;
    }
    if (provider.toLowerCase() === 'stripe' && !providerChargeID && !providerPaymentIntentID) {
      dispatch({ type: 'ACTION_ERROR', error: 'Stripe refunds require a provider charge ID or provider payment intent ID.' });
      return;
    }
    if (!Number.isFinite(amount) || amount <= 0) {
      dispatch({ type: 'ACTION_ERROR', error: 'Refund amount must be greater than zero.' });
      return;
    }
    if (!currency) {
      dispatch({ type: 'ACTION_ERROR', error: 'Currency is required.' });
      return;
    }
    if (amount > state.topupRefundDraft.refundableAmount) {
      dispatch({ type: 'ACTION_ERROR', error: 'Refund amount cannot exceed refundable balance.' });
      return;
    }
    if (state.topupRefundDraft.refundableAmount <= 0) {
      dispatch({ type: 'ACTION_ERROR', error: 'No refundable top-up balance remains.' });
      return;
    }
    dispatch({ type: 'ACTION_START', recordId: state.topupRefundDraft.topupId });
    try {
      await api.refundTopup(state.topupRefundDraft.topupId, {
        provider,
        providerRefundID,
        providerChargeID: providerChargeID || undefined,
        providerPaymentIntentID: providerPaymentIntentID || undefined,
        amount,
        currency,
        reason: state.topupRefundDraft.reason.trim() || undefined,
      });
      dispatch({ type: 'ACTION_DONE' });
      await loadBilling();
    } catch (error) {
      dispatch({ type: 'ACTION_ERROR', error: error instanceof Error ? error.message : 'Unable to record top-up refund.' });
    }
  };

  const failedWebhookCount = state.summary.webhookEvents?.failedCount ?? 0;
  const refundedPaymentAmount = state.summary.paymentIntents?.refundedAmount ?? 0;
  const refundedTopupAmount = state.summary.topups?.refundedAmount ?? 0;
  const refundedRefundAmount = state.summary.refunds?.refundedAmount ?? 0;
  const actionFormOpen = state.payoutPaidDraft !== null || state.payoutFailedDraft !== null || state.topupRefundDraft !== null;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Billing</h1>
        <div className="text-sm text-muted-foreground">{numberLabel(state.total)} records</div>
      </div>

      <div className="grid gap-3 md:grid-cols-5">
        {summaryTiles.map((tile) => {
          const metric = state.summary[tile.metric];
          return (
            <div key={tile.label} className="rounded-lg border border-border bg-card p-4">
              <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{tile.label}</div>
              <div className="mt-2 text-xl font-semibold text-foreground">{tile.value(metric)}</div>
              {tile.subvalue ? <div className="mt-1 text-sm text-muted-foreground">{tile.subvalue(metric)}</div> : null}
            </div>
          );
        })}
      </div>

      <div className="rounded-lg border border-border bg-card p-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Recovery queue</div>
            <div className="mt-1 text-base font-semibold text-foreground">Failure recovery</div>
          </div>
          <div className="grid flex-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <div>
              <div className="text-xs text-muted-foreground">Failed webhooks</div>
              <div className="mt-1 text-lg font-semibold text-foreground">{numberLabel(failedWebhookCount)}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Payment refunds</div>
              <div className="mt-1 text-lg font-semibold text-foreground">{money(refundedPaymentAmount)}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Top-up refunds</div>
              <div className="mt-1 text-lg font-semibold text-foreground">{money(refundedTopupAmount)}</div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">Refund records</div>
              <div className="mt-1 text-lg font-semibold text-foreground">{money(refundedRefundAmount)}</div>
            </div>
          </div>
          <Button type="button" variant="outline" className="min-h-[40px] self-start lg:self-center" onClick={() => dispatch({ type: 'OPEN_FAILED_WEBHOOKS' })}>
            Review failed webhooks
          </Button>
        </div>
      </div>

      <div className="flex flex-wrap gap-2" role="tablist" aria-label="Billing surfaces">
        {surfaces.map((surface) => (
          <Button
            key={surface.id}
            type="button"
            variant={surface.id === state.surface ? 'default' : 'outline'}
            className="min-h-[40px]"
            role="tab"
            aria-selected={surface.id === state.surface}
            onClick={() => dispatch({ type: 'SET_SURFACE', surface: surface.id })}
          >
            {surface.label}
          </Button>
        ))}
      </div>

      <div className="grid gap-3 lg:grid-cols-5">
        <Input
          aria-label="Organization ID filter"
          value={state.filters.organizationID}
          placeholder="Organization ID"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'organizationID', value: event.target.value })}
        />
        <Input
          aria-label="User ID filter"
          value={state.filters.userID}
          placeholder="User ID"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'userID', value: event.target.value })}
        />
        <Input
          aria-label="Status filter"
          value={state.filters.status}
          placeholder="Status"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'status', value: event.target.value })}
        />
        <Input
          aria-label="Kind filter"
          value={state.filters.kind}
          placeholder="Kind"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'kind', value: event.target.value })}
        />
        <Input
          aria-label="Provider filter"
          value={state.filters.provider}
          placeholder="Provider"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'provider', value: event.target.value })}
        />
      </div>

      {actionFormOpen ? (
        <div className="rounded-lg border border-border bg-card p-4">
          {state.payoutPaidDraft ? (
            <div className="space-y-4">
              <div>
                <h2 className="text-base font-semibold text-foreground">Payout confirmation</h2>
                <p className="mt-1 font-mono text-xs text-muted-foreground">{state.payoutPaidDraft.payoutId}</p>
              </div>
              <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto_auto] md:items-end">
                <div className="space-y-2">
                  <label htmlFor="billing-provider-payout-id" className="text-sm font-medium">Provider payout ID</label>
                  <Input
                    id="billing-provider-payout-id"
                    value={state.payoutPaidDraft.providerPayoutId}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_PAYOUT_FIELD', field: 'providerPayoutId', value: event.target.value })}
                  />
                </div>
                <Button
                  type="button"
                  className="min-h-[44px]"
                  disabled={state.actioningRecordId === state.payoutPaidDraft.payoutId}
                  onClick={() => void handleMarkPayoutPaid()}
                >
                  Confirm paid payout
                </Button>
                <Button type="button" variant="outline" className="min-h-[44px]" onClick={() => dispatch({ type: 'CLOSE_ACTION_FORM' })}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : null}
          {state.payoutFailedDraft ? (
            <div className="space-y-4">
              <div>
                <h2 className="text-base font-semibold text-foreground">Payout failure</h2>
                <p className="mt-1 font-mono text-xs text-muted-foreground">{state.payoutFailedDraft.payoutId}</p>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <div className="space-y-2">
                  <label htmlFor="billing-failed-provider-payout-id" className="text-sm font-medium">Provider payout ID</label>
                  <Input
                    id="billing-failed-provider-payout-id"
                    value={state.payoutFailedDraft.providerPayoutId}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_PAYOUT_FAILED_FIELD', field: 'providerPayoutId', value: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="billing-payout-failure-reason" className="text-sm font-medium">Failure reason</label>
                  <Input
                    id="billing-payout-failure-reason"
                    value={state.payoutFailedDraft.reason}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_PAYOUT_FAILED_FIELD', field: 'reason', value: event.target.value })}
                  />
                </div>
              </div>
              <div className="flex flex-wrap justify-end gap-2">
                <Button type="button" variant="outline" className="min-h-[44px]" onClick={() => dispatch({ type: 'CLOSE_ACTION_FORM' })}>
                  Cancel
                </Button>
                <Button
                  type="button"
                  className="min-h-[44px]"
                  disabled={state.actioningRecordId === state.payoutFailedDraft.payoutId}
                  onClick={() => void handleMarkPayoutFailed()}
                >
                  Confirm failed payout
                </Button>
              </div>
            </div>
          ) : null}
          {state.topupRefundDraft ? (
            <div className="space-y-4">
              <div>
                <h2 className="text-base font-semibold text-foreground">Top-up refund</h2>
                <p className="mt-1 font-mono text-xs text-muted-foreground">{state.topupRefundDraft.topupId}</p>
              </div>
              <div className="grid gap-3 lg:grid-cols-3">
                <div className="space-y-2">
                  <label htmlFor="billing-refund-provider" className="text-sm font-medium">Provider</label>
                  <Input
                    id="billing-refund-provider"
                    value={state.topupRefundDraft.provider}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'provider', value: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="billing-provider-refund-id" className="text-sm font-medium">Provider refund ID</label>
                  <Input
                    id="billing-provider-refund-id"
                    value={state.topupRefundDraft.providerRefundID}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'providerRefundID', value: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="billing-provider-charge-id" className="text-sm font-medium">Provider charge ID</label>
                  <Input
                    id="billing-provider-charge-id"
                    value={state.topupRefundDraft.providerChargeID}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'providerChargeID', value: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="billing-provider-payment-intent-id" className="text-sm font-medium">Provider payment intent ID</label>
                  <Input
                    id="billing-provider-payment-intent-id"
                    value={state.topupRefundDraft.providerPaymentIntentID}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'providerPaymentIntentID', value: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="billing-refund-amount" className="text-sm font-medium">Refund amount</label>
                  <Input
                    id="billing-refund-amount"
                    type="number"
                    min="0"
                    step="0.01"
                    value={state.topupRefundDraft.amount}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'amount', value: event.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <label htmlFor="billing-refund-currency" className="text-sm font-medium">Currency</label>
                  <Input
                    id="billing-refund-currency"
                    value={state.topupRefundDraft.currency}
                    className="min-h-[44px]"
                    onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'currency', value: event.target.value })}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <label htmlFor="billing-refund-reason" className="text-sm font-medium">Reason</label>
                <Textarea
                  id="billing-refund-reason"
                  value={state.topupRefundDraft.reason}
                  onChange={(event) => dispatch({ type: 'SET_TOPUP_REFUND_FIELD', field: 'reason', value: event.target.value })}
                />
              </div>
              <div className="flex flex-wrap justify-end gap-2">
                <Button type="button" variant="outline" className="min-h-[44px]" onClick={() => dispatch({ type: 'CLOSE_ACTION_FORM' })}>
                  Cancel
                </Button>
                <Button
                  type="button"
                  className="min-h-[44px]"
                  disabled={state.actioningRecordId === state.topupRefundDraft.topupId}
                  onClick={() => void handleRefundTopup()}
                >
                  Confirm top-up refund
                </Button>
              </div>
            </div>
          ) : null}
        </div>
      ) : null}

      <DataTable
        columns={activeSurface.columns}
        data={state.rows}
        loading={state.loading}
        error={state.error}
        emptyMessage="No billing records found for this commercial surface."
        onRetry={loadBilling}
        renderActions={
          state.surface === 'payouts'
            ? (record) =>
                record.status === 'payout_pending' ? (
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      className="min-h-[36px]"
                      disabled={state.actioningRecordId === record.id}
                      aria-label={`Mark payout ${record.id} paid`}
                      onClick={() => dispatch({ type: 'OPEN_PAYOUT_PAID', record })}
                    >
                      Mark paid
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      className="min-h-[36px]"
                      disabled={state.actioningRecordId === record.id}
                      aria-label={`Mark payout ${record.id} failed`}
                      onClick={() => dispatch({ type: 'OPEN_PAYOUT_FAILED', record })}
                    >
                      Mark failed
                    </Button>
                  </div>
                ) : null
            : state.surface === 'topups'
              ? (record) => {
                  const remainingAmount = Math.max((record.amount ?? record.money ?? 0) - (record.refundedAmount ?? 0), 0);
                  return record.status === 'paid' && remainingAmount > 0 ? (
                    <Button
                      type="button"
                      variant="outline"
                      className="min-h-[36px]"
                      disabled={state.actioningRecordId === record.id}
                      aria-label={`Record refund for top-up ${record.id}`}
                      onClick={() => dispatch({ type: 'OPEN_TOPUP_REFUND', record })}
                    >
                      Record refund
                    </Button>
                  ) : null;
                }
              : undefined
        }
      />
      {state.actionError ? <div role="alert" className="text-sm text-destructive">{state.actionError}</div> : null}
    </div>
  );
}
