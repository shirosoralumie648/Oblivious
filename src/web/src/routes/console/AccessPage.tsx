import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary } from '../../types/api';

export function AccessPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadAccess = async () => {
      try {
        const summary = await consoleApi.getAccess();
        if (!cancelled) {
          setAccessSummary(summary);
          setLoadError(null);
          setIsLoading(false);
        }
      } catch {
        if (!cancelled) {
          setAccessSummary(null);
          setLoadError('Unable to load access summary.');
          setIsLoading(false);
        }
      }
    };

    void loadAccess();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review the exact scope and session context behind this console."
      errorMessage={loadError}
      siblingLinks={[{ label: 'Open models', to: '/console/models' }]}
      title="Access"
    >
      {isLoading ? (
        <p>Loading access summary…</p>
      ) : accessSummary ? (
        <>
          <p>This console reflects the active workspace and current session.</p>
          <p>{`User: ${accessSummary.userEmail}`}</p>
          <p>{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p>{`Session: ${accessSummary.sessionId}`}</p>
          <p>{`Default mode: ${accessSummary.defaultMode}`}</p>
        </>
      ) : (
        <p>Access summary unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
