import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { ModelSummary } from '../../types/api';

export function ModelsPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [loadError, setLoadError] = useState(false);
  const [models, setModels] = useState<ModelSummary[] | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadModels = async () => {
      try {
        const nextModels = await consoleApi.getModels();
        if (!cancelled) {
          setModels(nextModels);
          setLoadError(false);
        }
      } catch {
        if (!cancelled) {
          setModels(null);
          setLoadError(true);
        }
      }
    };

    void loadModels();

    return () => {
      cancelled = true;
    };
  }, [consoleApi]);

  return (
    <section>
      <h1>Model summaries</h1>
      {loadError ? (
        <p>Unable to load model summaries.</p>
      ) : models === null ? (
        <p>Loading model summaries…</p>
      ) : (
        <ul>
          {models.map((model) => (
            <li key={model.id}>
              <p>{model.label}</p>
              <p>{`Requests: ${model.requests}`}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
