import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary } from '../../types/api';

export function AccessPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadAccess = async () => {
      const summary = await consoleApi.getAccess();
      if (!cancelled) {
        setAccessSummary(summary);
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
      {accessSummary === null ? (
        <p>Loading access summary…</p>
      ) : (
        <>
          <p>{`User: ${accessSummary.userEmail}`}</p>
          <p>{`Workspace: ${accessSummary.workspaceId}`}</p>
          <p>{`Session: ${accessSummary.sessionId}`}</p>
          <p>{`Default mode: ${accessSummary.defaultMode}`}</p>
        </>
      )}
    </section>
  );
}
