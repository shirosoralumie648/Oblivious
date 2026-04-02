import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { BillingSummary, ConsoleAccessSummary, ConsoleModelSummary, UsageSummary } from '../../types/api';

type DashboardState = {
  access: ConsoleAccessSummary | null;
  billing: BillingSummary | null;
  models: ConsoleModelSummary[];
  usage: UsageSummary | null;
};

export function ConsoleHomePage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [dashboard, setDashboard] = useState<DashboardState>({
    access: null,
    billing: null,
    models: [],
    usage: null
  });
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const loadDashboard = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const [access, usage, billing, models] = await Promise.all([
          consoleApi.getAccess(),
          consoleApi.getUsage(),
          consoleApi.getBilling(),
          consoleApi.getModels()
        ]);

        if (!cancelled) {
          setDashboard({
            access,
            billing,
            models,
            usage
          });
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load dashboard.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadDashboard();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  const topModel = dashboard.models[0]?.label ?? 'No model activity yet';

  return (
    <section>
      <h1>Console Home</h1>
      {isLoading ? <p>Loading dashboard…</p> : null}
      {error ? <p>{error}</p> : null}
      {dashboard.access ? <p>User: {dashboard.access.userEmail}</p> : null}
      {dashboard.access ? <p>Workspace: {dashboard.access.workspaceId}</p> : null}
      {dashboard.usage ? <p>Requests (7d): {dashboard.usage.requests}</p> : null}
      {dashboard.billing ? <p>Estimated cost (30d): ${dashboard.billing.estimatedCostUsd.toFixed(4)}</p> : null}
      {!isLoading && !error ? <p>Top model: {topModel}</p> : null}
    </section>
  );
}
