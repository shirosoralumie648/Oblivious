import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { BillingSummary } from '../../types/api';

export function BillingPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [summary, setSummary] = useState<BillingSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadBilling = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextSummary = await consoleApi.getBilling();
        if (!cancelled) {
          setSummary(nextSummary);
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load billing summary.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadBilling();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <section>
      <h1>Billing</h1>
      {isLoading ? <p>Loading billing summary…</p> : null}
      {error ? <p>{error}</p> : null}
      {summary ? (
        <div>
          <p>Period: {summary.period}</p>
          <p>Requests: {summary.requests}</p>
          <p>Input tokens: {summary.inputTokens}</p>
          <p>Output tokens: {summary.outputTokens}</p>
          <p>Estimated cost: ${summary.estimatedCostUsd.toFixed(4)}</p>
        </div>
      ) : null}
    </section>
  );
}
