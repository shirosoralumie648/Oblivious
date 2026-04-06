import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { BillingSummary } from '../../types/api';

export function BillingPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [billingSummary, setBillingSummary] = useState<BillingSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadBilling = async () => {
      const summary = await consoleApi.getBilling();
      if (!cancelled) {
        setBillingSummary(summary);
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
      {billingSummary === null ? (
        <p>Loading billing summary…</p>
      ) : (
        <>
          <p>{`Requests: ${billingSummary.requests}`}</p>
          <p>{`Input tokens: ${billingSummary.inputTokens}`}</p>
          <p>{`Output tokens: ${billingSummary.outputTokens}`}</p>
          <p>{`Estimated cost: $${billingSummary.estimatedCostUsd.toFixed(4)}`}</p>
        </>
      )}
    </section>
  );
}
