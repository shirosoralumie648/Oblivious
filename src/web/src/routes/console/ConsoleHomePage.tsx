import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleOverviewCard } from '../../features/console/components/ConsoleOverviewCard';
import { ConsoleSnapshotPanel } from '../../features/console/components/ConsoleSnapshotPanel';
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
      const [access, billing, models, usage] = await Promise.allSettled([
        consoleApi.getAccess(),
        consoleApi.getBilling(),
        consoleApi.getModels(),
        consoleApi.getUsage()
      ]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      setBillingSummary(billing.status === 'fulfilled' ? billing.value : null);
      setModelSummaries(models.status === 'fulfilled' ? models.value : null);
      setUsageSummary(usage.status === 'fulfilled' ? usage.value : null);
      setLoadError([access, billing, models, usage].every((result) => result.status === 'rejected'));
    };

    void loadDashboard();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  if (loadError) {
    return (
      <section>
        <h1>Console Home</h1>
        <p>Unable to load dashboard.</p>
      </section>
    );
  }

  if (accessSummary === null && billingSummary === null && modelSummaries === null && usageSummary === null) {
    return (
      <section>
        <h1>Console Home</h1>
        <p>Loading dashboard…</p>
      </section>
    );
  }

  const topModel = modelSummaries?.[0]?.label ?? 'Top model unavailable';
  const accessPosture = accessSummary ? `Session ${accessSummary.sessionId}` : 'Access posture unavailable';

  return (
    <section>
      <h1>Console Home</h1>
      <p>{`Current workspace scope: ${accessSummary?.workspaceId ?? 'unavailable'}`}</p>
      <section aria-label="Key performance indicators">
        <ConsoleOverviewCard
          note={billingSummary?.period ?? 'Billing summary unavailable'}
          title="Estimated cost"
          to="/console/billing"
          value={billingSummary ? `$${billingSummary.estimatedCostUsd.toFixed(4)}` : 'Estimated cost unavailable'}
        />
        <ConsoleOverviewCard
          note={usageSummary?.period ?? 'Usage summary unavailable'}
          title="Requests"
          to="/console/usage"
          value={usageSummary ? String(usageSummary.requests) : 'Requests unavailable'}
        />
        <ConsoleOverviewCard
          note="Primary model in current workspace"
          title="Top model"
          to="/console/models"
          value={topModel}
        />
        <ConsoleOverviewCard
          note="Current session and workspace context"
          title="Access posture"
          to="/console/access"
          value={accessPosture}
        />
      </section>
      <ConsoleSnapshotPanel title="Cost and usage focus">
        <p>{`Billing requests: ${billingSummary?.requests ?? 'unavailable'}`}</p>
        <p>{`Usage requests: ${usageSummary?.requests ?? 'unavailable'}`}</p>
        <Link to="/console/billing">Open billing drill-down</Link>
        <Link to="/console/usage">Open usage drill-down</Link>
      </ConsoleSnapshotPanel>
      <ConsoleSnapshotPanel title="Supporting summaries">
        <p>{`Active user: ${accessSummary?.userEmail ?? 'unavailable'}`}</p>
        <p>{accessSummary?.networkEnabledHint ? 'Network access hint enabled' : 'Network access hint unavailable'}</p>
      </ConsoleSnapshotPanel>
    </section>
  );
}
