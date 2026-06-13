import { useMemo, type ReactNode } from 'react';
import { RiArrowDownLine, RiArrowUpLine, RiErrorWarningLine, RiRefreshLine } from '@remixicon/react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';

import { EmptyState } from './EmptyState';

export type DataTableColumn<T> = {
  key: string;
  header: string;
  render?: (item: T) => ReactNode;
  sortable?: boolean;
  width?: string;
};

export type DataTableProps<T> = {
  columns: DataTableColumn<T>[];
  data: T[];
  loading: boolean;
  error: string | null;
  emptyMessage: string;
  sortKey?: string;
  sortDir?: 'asc' | 'desc';
  onSort?: (key: string) => void;
  onRetry?: () => void;
  renderActions?: (item: T) => ReactNode;
  selectable?: boolean;
  selectedIds?: Set<string>;
  onSelectChange?: (id: string, checked: boolean) => void;
  idKey?: string;
  className?: string;
};

function getValue<T>(item: T, key: string): ReactNode {
  if (typeof item === 'object' && item !== null && key in item) {
    const value = (item as Record<string, unknown>)[key];
    if (value === null || value === undefined) {
      return '';
    }
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      return String(value);
    }
  }
  return '';
}

function rowId<T>(item: T, idKey: string) {
  if (typeof item === 'object' && item !== null && idKey in item) {
    return String((item as Record<string, unknown>)[idKey]);
  }
  return '';
}

export function DataTable<T>({
  columns,
  data,
  loading,
  error,
  emptyMessage,
  sortKey,
  sortDir,
  onSort,
  onRetry,
  renderActions,
  selectable = false,
  selectedIds = new Set<string>(),
  onSelectChange,
  idKey = 'id',
  className,
}: DataTableProps<T>) {
  // ⚡ Bolt Optimization: Memoized selectable rows calculation
  // Impact: Prevents O(n) array operations (map/filter) on every render (e.g. during checkbox toggles).
  const selectableRows = useMemo(() => data.map((item) => rowId(item, idKey)).filter(Boolean), [data, idKey]);
  const selectedCount = selectableRows.filter((id) => selectedIds.has(id)).length;
  const allSelected = selectableRows.length > 0 && selectedCount === selectableRows.length;
  const partiallySelected = selectedCount > 0 && !allSelected;

  const handleSelectAll = (checked: boolean | 'indeterminate') => {
    selectableRows.forEach((id) => onSelectChange?.(id, checked === true));
  };

  if (error) {
    return (
      <div className={cn('rounded-lg border border-destructive/30 bg-card p-6 text-sm text-foreground', className)} role="alert">
        <div className="flex items-start gap-3">
          <RiErrorWarningLine className="mt-1 size-5 shrink-0 text-destructive" aria-hidden="true" />
          <div className="space-y-2">
            <p className="font-medium text-destructive">Something went wrong while loading this data. Please try again or contact support if the issue persists.</p>
            <p className="text-muted-foreground">{error}</p>
            {onRetry ? (
              <Button type="button" variant="outline" className="min-h-[44px]" onClick={onRetry}>
                <RiRefreshLine className="size-4" aria-hidden="true" />
                Try Again
              </Button>
            ) : null}
          </div>
        </div>
      </div>
    );
  }

  if (!loading && data.length === 0) {
    return (
      <div className={cn('rounded-lg border border-border bg-card', className)}>
        <EmptyState title={emptyMessage} />
      </div>
    );
  }

  return (
    <div className={cn('rounded-lg border border-border bg-card', className)} aria-busy={loading || undefined}>
      <div aria-label={loading ? 'Loading table data' : undefined}>
        <Table>
          <TableHeader>
            <TableRow>
              {selectable ? (
                <TableHead className="w-[48px]">
                  <Checkbox
                    aria-label="Select all rows"
                    checked={partiallySelected ? 'indeterminate' : allSelected}
                    onCheckedChange={handleSelectAll}
                  />
                </TableHead>
              ) : null}
              {columns.map((column) => (
                <TableHead key={column.key} style={column.width ? { width: column.width } : undefined}>
                  {column.sortable && onSort ? (
                    <Button
                      type="button"
                      variant="ghost"
                      className="min-h-[44px] px-0 text-foreground hover:bg-transparent"
                      aria-label={`Sort by ${column.header}`}
                      onClick={() => onSort(column.key)}
                    >
                      {column.header}
                      {sortKey === column.key && sortDir === 'asc' ? <RiArrowUpLine className="size-4" aria-hidden="true" /> : null}
                      {sortKey === column.key && sortDir === 'desc' ? <RiArrowDownLine className="size-4" aria-hidden="true" /> : null}
                    </Button>
                  ) : (
                    column.header
                  )}
                </TableHead>
              ))}
              {renderActions ? <TableHead className="w-[1%] text-right">Actions</TableHead> : null}
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading
              ? Array.from({ length: 5 }, (_, rowIndex) => (
                  <TableRow key={`loading-${rowIndex}`}>
                    {selectable ? (
                      <TableCell>
                        <Skeleton className="size-4 rounded-[6px]" />
                      </TableCell>
                    ) : null}
                    {columns.map((column) => (
                      <TableCell key={column.key}>
                        <Skeleton className="h-5 w-full max-w-[180px]" />
                      </TableCell>
                    ))}
                    {renderActions ? (
                      <TableCell>
                        <Skeleton className="ml-auto h-8 w-24" />
                      </TableCell>
                    ) : null}
                  </TableRow>
                ))
              : data.map((item) => {
                  const id = rowId(item, idKey);
                  const label = String(getValue(item, 'name') || id);
                  return (
                    <TableRow key={id || label} data-state={selectedIds.has(id) ? 'selected' : undefined}>
                      {selectable ? (
                        <TableCell>
                          <Checkbox
                            aria-label={`Select row ${label}`}
                            checked={selectedIds.has(id)}
                            onCheckedChange={(checked) => onSelectChange?.(id, checked === true)}
                          />
                        </TableCell>
                      ) : null}
                      {columns.map((column) => (
                        <TableCell key={column.key}>{column.render ? column.render(item) : getValue(item, column.key)}</TableCell>
                      ))}
                      {renderActions ? <TableCell className="text-right">{renderActions(item)}</TableCell> : null}
                    </TableRow>
                  );
                })}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}
