import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, UsageSummary } from '../../types/api';

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
        </>
      ) : (
        <p>Usage summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
