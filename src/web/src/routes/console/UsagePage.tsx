import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { UsageSummary } from '../../types/api';

export function UsagePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [summary, setSummary] = useState<UsageSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadUsage = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextSummary = await consoleApi.getUsage();
        if (!cancelled) {
          setSummary(nextSummary);
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load usage summary.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadUsage();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <section>
      <h1>Usage</h1>
      {isLoading ? <p>Loading usage summary…</p> : null}
      {error ? <p>{error}</p> : null}
      {summary ? (
        <div>
          <p>Period: {summary.period}</p>
          <p>Requests: {summary.requests}</p>
        </div>
      ) : null}
    </section>
  );
}
