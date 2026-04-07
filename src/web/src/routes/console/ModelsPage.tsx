import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { ConsoleWorkbenchLayout } from '../../features/console/components/ConsoleWorkbenchLayout';
import { createHttpClient } from '../../services/http/client';
import type { AccessSummary, ModelSummary } from '../../types/api';

export function ModelsPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [accessSummary, setAccessSummary] = useState<AccessSummary | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [models, setModels] = useState<ModelSummary[] | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadModels = async () => {
      const [access, modelsResult] = await Promise.allSettled([consoleApi.getAccess(), consoleApi.getModels()]);

      if (cancelled) {
        return;
      }

      setAccessSummary(access.status === 'fulfilled' ? access.value : null);
      if (modelsResult.status === 'fulfilled') {
        setModels(modelsResult.value);
        setLoadError(null);
      } else {
        setModels(null);
        setLoadError('Unable to load model summaries.');
      }
      setIsLoading(false);
    };

    void loadModels();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <ConsoleWorkbenchLayout
      accessSummary={accessSummary}
      description="Review the current workspace model mix and relative request volume."
      errorMessage={loadError}
      siblingLinks={[{ label: 'Open access', to: '/console/access' }]}
      title="Models"
    >
      {isLoading ? (
        <p>Loading model summaries…</p>
      ) : models ? (
        <ul>
          {models.map((model) => (
            <li key={model.id}>
              <p>{model.label}</p>
              <p>{`Requests: ${model.requests}`}</p>
            </li>
          ))}
        </ul>
      ) : (
        <p>Model summaries unavailable.</p>
      )}
    </ConsoleWorkbenchLayout>
  );
}
