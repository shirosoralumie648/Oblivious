import { useCallback, useEffect, useMemo, useReducer } from 'react';

import { Input } from '@/components/ui/input';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { StatusBadge, type StatusBadgeStatus } from '../../components/shared/StatusBadge';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type {
  UsageAnalyticsBucket,
  UsageAnalyticsCrossDimensionBucket,
  UsageAnalyticsFilter,
  UsageAnalyticsResponse,
  UsageLogEntry,
  UsageLogFilter,
} from '../../types/admin';

type UsageLogState = {
  logs: UsageLogEntry[];
  analytics: UsageAnalyticsResponse;
  total: number;
  loading: boolean;
  error: string | null;
  filters: {
    organizationID: string;
    userID: string;
    apiTokenID: string;
    requestID: string;
    apiType: string;
    featureType: string;
    quotaMode: string;
    channelID: string;
    provider: string;
    status: string;
    model: string;
    analyticsGranularity: UsageAnalyticsFilter['granularity'];
  };
};

type UsageLogAction =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; logs: UsageLogEntry[]; total: number; analytics: UsageAnalyticsResponse }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_FILTER'; field: keyof UsageLogState['filters']; value: string };

const emptyAnalytics: UsageAnalyticsResponse = {
  byModel: [],
  byFeature: [],
  byUser: [],
  byTime: [],
  byChannel: [],
  byProvider: [],
  crossDimensions: [],
};

const initialState: UsageLogState = {
  logs: [],
  analytics: emptyAnalytics,
  total: 0,
  loading: true,
  error: null,
  filters: {
    organizationID: '',
    userID: '',
    apiTokenID: '',
    requestID: '',
    apiType: '',
    featureType: '',
    quotaMode: '',
    channelID: '',
    provider: '',
    status: '',
    model: '',
    analyticsGranularity: 'day',
  },
};

function reducer(state: UsageLogState, action: UsageLogAction): UsageLogState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, logs: action.logs, total: action.total, analytics: action.analytics };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_FILTER':
      return { ...state, filters: { ...state.filters, [action.field]: action.value } };
    default:
      return state;
  }
}

function money(value?: number) {
  return `$${(value ?? 0).toFixed(4)}`;
}

function dateLabel(value?: string) {
  if (!value) {
    return '-';
  }
  return new Date(value).toLocaleString();
}

function idCell(value?: string) {
  return <span className="break-all font-mono text-xs">{value || '-'}</span>;
}

function providerCell(log: UsageLogEntry) {
  if (!log.provider && !log.channelId) {
    return '-';
  }
  return `${log.provider || '-'} / ${log.channelId || '-'}`;
}

function requestLogEvidenceCell(log: UsageLogEntry) {
  const evidence = log.requestLogEvidence;
  if (!evidence) {
    return <span className="text-muted-foreground">No ClickHouse row</span>;
  }

  return (
    <div className="min-w-0 space-y-1 text-xs">
      <div className="break-all font-mono text-foreground">{evidence.requestLogId}</div>
      <div className="break-words text-muted-foreground [overflow-wrap:anywhere]">
        {evidence.service} {evidence.method} {evidence.endpoint}
      </div>
      <div className="font-mono text-muted-foreground">
        CH {money(evidence.costUsd)} / {evidence.durationMs} ms
      </div>
    </div>
  );
}

const usageStatusTone: Record<string, StatusBadgeStatus> = {
  success: 'approved',
  settled: 'approved',
  error: 'rejected',
  failed: 'rejected',
  timeout: 'rejected',
  rejected: 'rejected',
  pending: 'pending',
};

function usageStatusLabel(status?: string, statusCode?: number) {
  if (!status) {
    return statusCode ? String(statusCode) : 'Unknown';
  }
  return status
    .split('_')
    .map((part) => `${part.charAt(0).toUpperCase()}${part.slice(1)}`)
    .join(' ');
}

function statusCell(log: UsageLogEntry) {
  return (
    <StatusBadge
      status={usageStatusTone[log.status ?? ''] ?? 'pending'}
      label={usageStatusLabel(log.status, log.statusCode)}
    />
  );
}

function AnalyticsPanel({ title, rows }: { title: string; rows: UsageAnalyticsBucket[] }) {
  const visibleRows = rows.slice(0, 4);

  return (
    <section className="min-w-0 rounded-lg border border-border bg-card p-4" aria-label={title}>
      <div className="mb-3 flex min-w-0 items-center justify-between gap-3">
        <h3 className="min-w-0 break-words text-sm font-semibold text-foreground [overflow-wrap:anywhere]">{title}</h3>
        <span className="text-xs text-muted-foreground">{visibleRows.length.toLocaleString()} rows</span>
      </div>
      {visibleRows.length === 0 ? (
        <p className="text-sm text-muted-foreground">No analytics data</p>
      ) : (
        <div className="min-w-0 space-y-3">
          {visibleRows.map((row) => (
            <div key={row.key} className="min-w-0 space-y-1.5">
              <div className="flex min-w-0 flex-wrap items-start justify-between gap-2 text-sm">
                <span className="min-w-0 break-words font-medium text-foreground [overflow-wrap:anywhere]" title={row.key}>
                  {row.key}
                </span>
                <span className="shrink-0 font-mono text-xs text-muted-foreground">{money(row.totalCost)}</span>
              </div>
              <div className="grid min-w-0 grid-cols-3 gap-2 text-xs text-muted-foreground">
                <span className="min-w-0 break-words [overflow-wrap:anywhere]">{row.requestCount.toLocaleString()} req</span>
                <span className="min-w-0 break-words [overflow-wrap:anywhere]">{row.totalTokens.toLocaleString()} tok</span>
                <span className="min-w-0 break-words [overflow-wrap:anywhere]">{row.dimension}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function CrossDimensionsPanel({ rows }: { rows: UsageAnalyticsCrossDimensionBucket[] }) {
  const visibleRows = rows.slice(0, 6);

  return (
    <section className="min-w-0 rounded-lg border border-border bg-card p-4" aria-label="Cross dimensions">
      <div className="mb-3 flex min-w-0 items-center justify-between gap-3">
        <h3 className="min-w-0 break-words text-sm font-semibold text-foreground [overflow-wrap:anywhere]">Cross dimensions</h3>
        <span className="text-xs text-muted-foreground">{visibleRows.length.toLocaleString()} rows</span>
      </div>
      {visibleRows.length === 0 ? (
        <p className="text-sm text-muted-foreground">No cross dimension data</p>
      ) : (
        <div className="min-w-0 space-y-3">
          {visibleRows.map((row) => (
            <div key={`${row.dimension}:${row.key}`} className="min-w-0 space-y-1.5">
              <div className="flex min-w-0 flex-wrap items-start justify-between gap-2 text-sm">
                <span className="max-w-full break-words rounded-md border border-border px-2 py-0.5 font-mono text-xs text-muted-foreground [overflow-wrap:anywhere]">
                  {row.dimension}
                </span>
                <span className="shrink-0 font-mono text-xs text-muted-foreground">{money(row.totalCost)}</span>
              </div>
              <div className="min-w-0 break-words text-sm font-medium text-foreground [overflow-wrap:anywhere]" title={row.key}>
                {row.key}
              </div>
              <div className="grid min-w-0 grid-cols-2 gap-2 text-xs text-muted-foreground sm:grid-cols-4">
                <span className="min-w-0 break-words [overflow-wrap:anywhere]">{row.requestCount.toLocaleString()} req</span>
                <span className="min-w-0 break-words [overflow-wrap:anywhere]">{row.totalTokens.toLocaleString()} tok</span>
                <span className="min-w-0 break-words [overflow-wrap:anywhere]" title={row.primary}>
                  {row.primary || '-'}
                </span>
                <span className="min-w-0 break-words [overflow-wrap:anywhere]" title={row.secondary}>
                  {row.secondary || '-'}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

export function AdminUsageLogsPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);

  const loadUsageLogs = useCallback(async () => {
    dispatch({ type: 'LOAD_START' });
    try {
      const filters: UsageLogFilter = {
        organizationID: state.filters.organizationID,
        userID: state.filters.userID,
        apiTokenID: state.filters.apiTokenID,
        requestID: state.filters.requestID,
        apiType: state.filters.apiType,
        featureType: state.filters.featureType,
        quotaMode: state.filters.quotaMode,
        channelID: state.filters.channelID,
        provider: state.filters.provider,
        status: state.filters.status,
        model: state.filters.model,
        limit: 50,
        offset: 0,
      };
      const analyticsFilters: UsageAnalyticsFilter = {
        organizationID: state.filters.organizationID,
        userID: state.filters.userID,
        apiType: state.filters.apiType,
        featureType: state.filters.featureType,
        quotaMode: state.filters.quotaMode,
        channelID: state.filters.channelID,
        provider: state.filters.provider,
        status: state.filters.status,
        model: state.filters.model,
        granularity: state.filters.analyticsGranularity,
        limit: 8,
      };
      const [result, analytics] = await Promise.all([api.listUsageLogs(filters), api.getUsageAnalytics(analyticsFilters)]);
      dispatch({ type: 'LOAD_SUCCESS', logs: result.data, total: result.total, analytics });
    } catch (error) {
      dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
    }
  }, [
    api,
    state.filters.apiTokenID,
    state.filters.apiType,
    state.filters.analyticsGranularity,
    state.filters.channelID,
    state.filters.featureType,
    state.filters.model,
    state.filters.organizationID,
    state.filters.provider,
    state.filters.quotaMode,
    state.filters.requestID,
    state.filters.status,
    state.filters.userID,
  ]);

  useEffect(() => {
    void loadUsageLogs();
  }, [loadUsageLogs]);

  const columns: DataTableColumn<UsageLogEntry>[] = [
    { key: 'requestId', header: 'Request', render: (log) => idCell(log.requestId ?? log.id), width: '180px' },
    { key: 'requestLogEvidence', header: 'Request Log Evidence', render: requestLogEvidenceCell, width: '260px' },
    { key: 'userId', header: 'User', render: (log) => idCell(log.userId), width: '160px' },
    { key: 'apiTokenId', header: 'API Token', render: (log) => idCell(log.apiTokenId), width: '160px' },
    { key: 'apiType', header: 'API Type', render: (log) => log.apiType || '-' },
    { key: 'featureType', header: 'Feature', render: (log) => log.featureType || '-' },
    { key: 'quotaMode', header: 'Quota Mode', render: (log) => log.quotaMode || '-' },
    { key: 'model', header: 'Model', render: (log) => log.model || '-' },
    { key: 'provider', header: 'Provider / Channel', render: providerCell },
    { key: 'status', header: 'Status', render: statusCell },
    { key: 'cost', header: 'Cost', render: (log) => money(log.cost) },
    { key: 'channelCost', header: 'Channel Cost', render: (log) => money(log.channelCost) },
    { key: 'totalTokens', header: 'Tokens', render: (log) => log.totalTokens.toLocaleString() },
    { key: 'latencyMs', header: 'Latency', render: (log) => `${log.latencyMs ?? 0} ms` },
    { key: 'createdAt', header: 'Timestamp', render: (log) => dateLabel(log.createdAt) },
  ];

  const setFilter = (field: keyof UsageLogState['filters']) => (event: React.ChangeEvent<HTMLInputElement>) => {
    dispatch({ type: 'SET_FILTER', field, value: event.target.value });
  };
  const setSelectFilter = (field: keyof UsageLogState['filters']) => (event: React.ChangeEvent<HTMLSelectElement>) => {
    dispatch({ type: 'SET_FILTER', field, value: event.target.value });
  };

  return (
    <div className="min-w-0 space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="font-heading text-2xl font-semibold text-foreground">Usage Logs</h1>
          <p className="mt-1 text-sm text-muted-foreground">{state.total.toLocaleString()} relay requests matched</p>
        </div>
      </div>

      <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(180px,1fr)_minmax(160px,1fr)_minmax(160px,1fr)_minmax(160px,1fr)] xl:grid-cols-12">
        <Input aria-label="Organization ID filter" value={state.filters.organizationID} placeholder="Organization ID" className="min-h-[44px]" onChange={setFilter('organizationID')} />
        <Input aria-label="User ID filter" value={state.filters.userID} placeholder="User ID" className="min-h-[44px]" onChange={setFilter('userID')} />
        <Input aria-label="Request ID filter" value={state.filters.requestID} placeholder="Request ID" className="min-h-[44px]" onChange={setFilter('requestID')} />
        <Input aria-label="API token ID filter" value={state.filters.apiTokenID} placeholder="API token ID" className="min-h-[44px]" onChange={setFilter('apiTokenID')} />
        <Input aria-label="API type filter" value={state.filters.apiType} placeholder="API type" className="min-h-[44px]" onChange={setFilter('apiType')} />
        <Input aria-label="Feature type filter" value={state.filters.featureType} placeholder="Feature type" className="min-h-[44px]" onChange={setFilter('featureType')} />
        <Input aria-label="Quota mode filter" value={state.filters.quotaMode} placeholder="Quota mode" className="min-h-[44px]" onChange={setFilter('quotaMode')} />
        <Input aria-label="Channel ID filter" value={state.filters.channelID} placeholder="Channel ID" className="min-h-[44px]" onChange={setFilter('channelID')} />
        <Input aria-label="Provider filter" value={state.filters.provider} placeholder="Provider" className="min-h-[44px]" onChange={setFilter('provider')} />
        <Input aria-label="Status filter" value={state.filters.status} placeholder="Status" className="min-h-[44px]" onChange={setFilter('status')} />
        <Input aria-label="Model filter" value={state.filters.model} placeholder="Model" className="min-h-[44px]" onChange={setFilter('model')} />
        <select
          aria-label="Analytics granularity filter"
          value={state.filters.analyticsGranularity}
          className="h-9 w-full min-w-0 rounded-4xl border border-input bg-input/30 px-3 py-1 text-base text-foreground transition-colors outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm min-h-[44px]"
          onChange={setSelectFilter('analyticsGranularity')}
        >
          <option value="second">Second</option>
          <option value="minute">Minute</option>
          <option value="hour">Hour</option>
          <option value="day">Day</option>
          <option value="week">Week</option>
          <option value="month">Month</option>
        </select>
      </div>

      <section className="min-w-0 space-y-3" aria-labelledby="usage-analytics-heading">
        <div>
          <h2 id="usage-analytics-heading" className="font-heading text-lg font-semibold text-foreground">
            Usage Analytics
          </h2>
        </div>
        <div className="grid min-w-0 gap-3 md:grid-cols-2 xl:grid-cols-3">
          <AnalyticsPanel title="By model" rows={state.analytics.byModel} />
          <AnalyticsPanel title="By feature" rows={state.analytics.byFeature} />
          <AnalyticsPanel title="By user" rows={state.analytics.byUser} />
          <AnalyticsPanel title="By time" rows={state.analytics.byTime} />
          <AnalyticsPanel title="By channel" rows={state.analytics.byChannel} />
          <AnalyticsPanel title="By provider" rows={state.analytics.byProvider} />
        </div>
        <CrossDimensionsPanel rows={state.analytics.crossDimensions ?? []} />
      </section>

      <DataTable
        columns={columns}
        data={state.logs}
        loading={state.loading}
        error={state.error}
        emptyMessage="No relay usage logs found for these filters."
        onRetry={loadUsageLogs}
      />
    </div>
  );
}
