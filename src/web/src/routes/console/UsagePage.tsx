import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { UsageSummary } from '../../types/api';

export function UsagePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadUsage = async () => {
      const summary = await consoleApi.getUsage();
      if (!cancelled) {
        setUsageSummary(summary);
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
      {usageSummary === null ? (
        <p>Loading usage summary…</p>
      ) : (
        <>
          <p>{`Requests: ${usageSummary.requests}`}</p>
          <p>{`Period: ${usageSummary.period}`}</p>
        </>
      )}
    </section>
  );
}
