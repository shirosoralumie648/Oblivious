import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { ConsoleAccessSummary } from '../../types/api';

export function AccessPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [summary, setSummary] = useState<ConsoleAccessSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadAccess = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextSummary = await consoleApi.getAccess();
        if (!cancelled) {
          setSummary(nextSummary);
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load access summary.');
        }
      } finally {
        if (!cancelled) {
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
    <section>
      <h1>Access</h1>
      {isLoading ? <p>Loading access summary…</p> : null}
      {error ? <p>{error}</p> : null}
      {summary ? (
        <div>
          <p>User: {summary.userEmail}</p>
          <p>Workspace: {summary.workspaceId}</p>
          <p>Session: {summary.sessionId}</p>
          <p>Session expires: {summary.sessionExpiresAt}</p>
          <p>Default mode: {summary.defaultMode}</p>
          <p>Model strategy: {summary.modelStrategy}</p>
        </div>
      ) : null}
    </section>
  );
}
