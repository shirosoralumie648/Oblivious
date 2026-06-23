import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type {
  AccessSummary,
  ConsoleApiTokenUsageItem,
  UsageDimensionSummary,
  UsageSummary,
  UsageTimeSeriesSummary
} from '../../types/api';

function formatCurrency(value: number) {
  return `$${value.toFixed(4)}`;
}

function requestLabel(item: ConsoleApiTokenUsageItem) {
  return item.requestId || item.id;
}

function metricRow(row: UsageDimensionSummary | UsageTimeSeriesSummary) {
  return (
    <>
      <td>{'key' in row ? row.key : row.bucket}</td>
      <td>{`${row.requestCount.toLocaleString()} req`}</td>
      <td>{`${row.totalTokens.toLocaleString()} tokens`}</td>
      <td>{formatCurrency(row.totalCost)}</td>
    </>
  );
}

function UsageAggregationTable({
  emptyText,
  rows,
  title
}: {
  emptyText: string;
  rows: Array<UsageDimensionSummary | UsageTimeSeriesSummary>;
  title: string;
}) {
  return (
    <section aria-label={title}>
      <h2>{title}</h2>
      {rows.length > 0 ? (
        <div className="min-w-0 max-w-full overflow-x-auto">
          <table className="min-w-[560px] border-collapse text-left text-sm">
            <thead>
              <tr>
                <th scope="col">Segment</th>
                <th scope="col">Requests</th>
                <th scope="col">Tokens</th>
                <th scope="col">Cost</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={'key' in row ? row.key : row.bucket}>{metricRow(row)}</tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p>{emptyText}</p>
      )}
    </section>
  );
}

export function UsagePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadUsage = async () => {
      const [access, usage] = await Promise.allSettled([consoleApi.getAccess(), consoleApi.getUsage()]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (usage.status === 'fulfilled') {
        setUsageSummary(usage.value);
        setErrorMessage(null);
      } else {
        setUsageSummary(null);
        setErrorMessage('Unable to load usage summary.');
      }
      setIsLoading(false);
    };

    void loadUsage();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review current workspace request volume and operating context."
      errorMessage={errorMessage}
      siblingLinks={[{ label: 'Open billing', to: '/console/billing' }]}
      title="Usage"
    >
      {isLoading ? (
        <p>Loading usage summary…</p>
      ) : usageSummary ? (
        <>
          <p>{`Requests: ${usageSummary.requests}`}</p>
          <p>{`Period: ${usageSummary.period}`}</p>
          <UsageAggregationTable
            emptyText="No model usage recorded for this period."
            rows={usageSummary.byModel ?? []}
            title="By model"
          />
          <UsageAggregationTable
            emptyText="No feature usage recorded for this period."
            rows={usageSummary.byFeature ?? []}
            title="By feature"
          />
          <UsageAggregationTable
            emptyText="No user usage recorded for this period."
            rows={usageSummary.byUser ?? []}
            title="Top users"
          />
          <UsageAggregationTable
            emptyText="No daily usage trend recorded for this period."
            rows={usageSummary.timeSeries ?? []}
            title="Daily trend"
          />
          <section>
            <h2>Recent relay requests</h2>
            {usageSummary.recent && usageSummary.recent.length > 0 ? (
              <ul>
                {usageSummary.recent.map((item) => (
                  <li className="min-w-0 break-words" key={item.id}>
                    <span>{requestLabel(item)}</span>
                    <span>{item.apiTokenId || '-'}</span>
                    <span>{item.model || '-'}</span>
                    <span>{item.status || 'unknown'}</span>
                    <span>{`${item.totalTokens} tokens`}</span>
                    <span>{formatCurrency(item.cost)}</span>
                    <span>{`${item.latencyMs ?? 0} ms`}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p>No recent relay usage recorded for this user.</p>
            )}
          </section>
        </>
      ) : (
        <p>Usage summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
