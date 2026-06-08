import { useCallback, useEffect, useMemo, useReducer } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

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

type BillingState = {
  summary: BillingSummary;
  rows: BillingInspectionRecord[];
  total: number;
  loading: boolean;
  error: string | null;
  actionError: string | null;
  actioningRecordId: string | null;
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
  surface: 'sessions',
  filters: {
    organizationID: '',
    userID: '',
    status: '',
    kind: '',
    provider: '',
  },
};

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
      return { ...state, actioningRecordId: null };
    case 'ACTION_ERROR':
      return { ...state, actioningRecordId: null, actionError: action.error };
    case 'SET_SURFACE':
      return { ...state, surface: action.surface };
    case 'SET_FILTER':
      return { ...state, filters: { ...state.filters, [action.field]: action.value } };
    case 'OPEN_FAILED_WEBHOOKS':
      return { ...state, surface: 'webhookEvents', filters: { ...state.filters, status: 'failed' } };
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

  const handleMarkPayoutPaid = async (record: BillingInspectionRecord) => {
    const providerPayoutId = record.providerPayoutId || record.id;
    dispatch({ type: 'ACTION_START', recordId: record.id });
    try {
      await api.markMarketplacePayoutPaid(record.id, providerPayoutId);
      dispatch({ type: 'ACTION_DONE' });
      await loadBilling();
    } catch (error) {
      dispatch({ type: 'ACTION_ERROR', error: error instanceof Error ? error.message : 'Unable to mark payout paid.' });
    }
  };

  const handleRefundTopup = async (record: BillingInspectionRecord) => {
    const remainingAmount = Math.max((record.amount ?? record.money ?? 0) - (record.refundedAmount ?? 0), 0);
    if (remainingAmount <= 0) {
      dispatch({ type: 'ACTION_ERROR', error: 'No refundable top-up balance remains.' });
      return;
    }
    dispatch({ type: 'ACTION_START', recordId: record.id });
    try {
      await api.refundTopup(record.id, {
        provider: record.provider || 'manual',
        providerRefundID: record.providerRefundId || `admin-${record.id}`,
        providerPaymentIntentID: record.providerPaymentIntentId,
        amount: remainingAmount,
        currency: record.currency || 'usd',
        reason: 'admin recorded provider refund',
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
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-[36px]"
                    disabled={state.actioningRecordId === record.id}
                    aria-label={`Mark payout ${record.id} paid`}
                    onClick={() => void handleMarkPayoutPaid(record)}
                  >
                    Mark paid
                  </Button>
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
                      onClick={() => void handleRefundTopup(record)}
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
