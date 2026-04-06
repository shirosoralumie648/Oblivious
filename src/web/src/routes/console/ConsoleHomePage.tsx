import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, BillingSummary, ModelSummary, UsageSummary } from '../../types/api';

export function ConsoleHomePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [billingSummary, setBillingSummary] = useState<BillingSummary | null>(null);
  const [loadError, setLoadError] = useState(false);
  const [modelSummaries, setModelSummaries] = useState<ModelSummary[] | null>(null);
  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadDashboard = async () => {
      try {
        const [access, billing, models, usage] = await Promise.all([
          consoleApi.getAccess(),
          consoleApi.getBilling(),
          consoleApi.getModels(),
          consoleApi.getUsage()
        ]);

        if (!cancelled) {
          setAccessSummary(access);
          setBillingSummary(billing);
          setModelSummaries(models);
          setUsageSummary(usage);
          setLoadError(false);
        }
      } catch {
        if (!cancelled) {
          setLoadError(true);
        }
      }
    };

    void loadDashboard();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  const topModel = modelSummaries?.[0]?.label ?? 'n/a';

  return (
    <section>
      <h1>Console Home</h1>
      {loadError ? (
        <p>Unable to load dashboard.</p>
      ) : accessSummary === null || billingSummary === null || modelSummaries === null || usageSummary === null ? (
        <p>Loading dashboard…</p>
      ) : (
        <>
          <p>{`Requests (7d): ${usageSummary.requests}`}</p>
          <p>{`Estimated cost (30d): $${billingSummary.estimatedCostUsd.toFixed(4)}`}</p>
          <p>{`Top model: ${topModel}`}</p>
          <p>{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p>{`User: ${accessSummary.userEmail}`}</p>
        </>
      )}
    </section>
  );
}
