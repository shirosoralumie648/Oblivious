import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, BillingSummary } from '../../types/api';

export function BillingPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [billingSummary, setBillingSummary] = useState<BillingSummary | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const loadBilling = async () => {
      const [access, billing] = await Promise.allSettled([consoleApi.getAccess(), consoleApi.getBilling()]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (billing.status === 'fulfilled') {
        setBillingSummary(billing.value);
        setErrorMessage(null);
      } else {
        setBillingSummary(null);
        setErrorMessage('Unable to load billing summary.');
      }
      setIsLoading(false);
    };

    void loadBilling();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review current workspace cost and billing activity."
      errorMessage={errorMessage}
      siblingLinks={[{ label: 'Open usage', to: '/console/usage' }]}
      title="Billing"
    >
      {isLoading ? (
        <p>Loading billing summary…</p>
      ) : billingSummary ? (
        <>
          <p>{`Requests: ${billingSummary.requests}`}</p>
          <p>{`Input tokens: ${billingSummary.inputTokens}`}</p>
          <p>{`Output tokens: ${billingSummary.outputTokens}`}</p>
          <p>{`Estimated cost: $${billingSummary.estimatedCostUsd.toFixed(4)}`}</p>
        </>
      ) : (
        <p>Billing summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
