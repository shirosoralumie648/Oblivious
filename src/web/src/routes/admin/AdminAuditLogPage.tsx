import { useCallback, useEffect, useMemo, useReducer, useRef } from 'react';

import { Input } from '@/components/ui/input';

import { DataTable, type DataTableColumn } from '../../components/shared/DataTable';
import { SearchBar } from '../../components/shared/SearchBar';
import { createAdminApi } from '../../features/admin/api';
import { createHttpClient } from '../../services/http/client';
import type { AuditEntry } from '../../types/admin';

type AuditState = {
  entries: AuditEntry[];
  loading: boolean;
  error: string | null;
  organizationID: string;
  actorSearch: string;
  action: string;
  resourceType: string;
  startDate: string;
  endDate: string;
  offset: number;
};

type Action =
  | { type: 'LOAD_START' }
  | { type: 'LOAD_SUCCESS'; entries: AuditEntry[] }
  | { type: 'LOAD_ERROR'; error: string }
  | { type: 'SET_FILTER'; field: 'organizationID' | 'actorSearch' | 'action' | 'resourceType' | 'startDate' | 'endDate'; value: string };

const initialState: AuditState = {
  entries: [],
  loading: true,
  error: null,
  organizationID: '',
  actorSearch: '',
  action: '',
  resourceType: '',
  startDate: '',
  endDate: '',
  offset: 0,
};

function reducer(state: AuditState, action: Action): AuditState {
  switch (action.type) {
    case 'LOAD_START':
      return { ...state, loading: true, error: null };
    case 'LOAD_SUCCESS':
      return { ...state, loading: false, error: null, entries: action.entries };
    case 'LOAD_ERROR':
      return { ...state, loading: false, error: action.error };
    case 'SET_FILTER':
      return { ...state, [action.field]: action.value, offset: 0 };
    default:
      return state;
  }
}

function resourceLabel(entry: AuditEntry) {
  const id = entry.resourceID ?? entry.resourceId;
  return id ? `${entry.resourceType} / ${id}` : entry.resourceType;
}

function changesSummary(changes?: string) {
  if (!changes) {
    return '-';
  }

  try {
    const parsed = JSON.parse(changes) as Record<string, unknown>;
    return Object.keys(parsed).slice(0, 4).join(', ') || changes;
  } catch {
    return changes.length > 80 ? `${changes.slice(0, 77)}...` : changes;
  }
}

function formatDate(value: string) {
  return new Date(value).toLocaleString();
}

export function AdminAuditLogPage() {
  const [state, dispatch] = useReducer(reducer, initialState);
  const api = useMemo(() => createAdminApi(createHttpClient()), []);
  const latestRequestRef = useRef(0);

  const loadEntries = useCallback(async () => {
    const requestID = latestRequestRef.current + 1;
    latestRequestRef.current = requestID;
    dispatch({ type: 'LOAD_START' });
    try {
      const result = await api.listAuditLogs({
        organizationID: state.organizationID,
        actorID: state.actorSearch,
        action: state.action,
        resourceType: state.resourceType,
        startDate: state.startDate,
        endDate: state.endDate,
        limit: 50,
        offset: state.offset,
      });
      if (requestID === latestRequestRef.current) {
        dispatch({ type: 'LOAD_SUCCESS', entries: result.data });
      }
    } catch (error) {
      if (requestID === latestRequestRef.current) {
        dispatch({ type: 'LOAD_ERROR', error: error instanceof Error ? error.message : 'Something went wrong while loading this data.' });
      }
    }
  }, [api, state.action, state.actorSearch, state.endDate, state.offset, state.organizationID, state.resourceType, state.startDate]);

  useEffect(() => {
    void loadEntries();
  }, [loadEntries]);

  const columns: DataTableColumn<AuditEntry>[] = [
    { key: 'actorEmail', header: 'Actor', sortable: true },
    { key: 'action', header: 'Action', render: (entry) => <span className="font-mono text-xs">{entry.action}</span> },
    { key: 'resourceType', header: 'Resource', render: resourceLabel },
    { key: 'changes', header: 'Changes', render: (entry) => changesSummary(entry.changes) },
    { key: 'ipAddress', header: 'IP Address', render: (entry) => entry.ipAddress ?? '-' },
    { key: 'createdAt', header: 'Timestamp', render: (entry) => formatDate(entry.createdAt) },
  ];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="font-heading text-2xl font-semibold text-foreground">Audit Log</h1>
      </div>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(220px,1fr)_180px_180px_180px_160px_160px]">
        <SearchBar
          value={state.actorSearch}
          onChange={(value) => dispatch({ type: 'SET_FILTER', field: 'actorSearch', value })}
          placeholder="Search actor..."
        />
        <Input
          aria-label="Organization ID filter"
          value={state.organizationID}
          placeholder="Organization ID"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'organizationID', value: event.target.value })}
        />
        <Input
          aria-label="Action filter"
          value={state.action}
          placeholder="Action"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'action', value: event.target.value })}
        />
        <Input
          aria-label="Resource type filter"
          value={state.resourceType}
          placeholder="Resource type"
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'resourceType', value: event.target.value })}
        />
        <Input
          aria-label="Start date"
          type="date"
          value={state.startDate}
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'startDate', value: event.target.value })}
        />
        <Input
          aria-label="End date"
          type="date"
          value={state.endDate}
          className="min-h-[44px]"
          onChange={(event) => dispatch({ type: 'SET_FILTER', field: 'endDate', value: event.target.value })}
        />
      </div>

      <DataTable
        columns={columns}
        data={state.entries}
        loading={state.loading}
        error={state.error}
        emptyMessage="No audit entries found -- Administrative actions will appear here."
        onRetry={loadEntries}
      />
    </div>
  );
}
