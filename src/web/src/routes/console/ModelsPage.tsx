import { useEffect, useMemo, useState } from 'react';

import { createConsoleApi } from '../../features/console/api';
import { createHttpClient } from '../../services/http/client';
import type { ConsoleModelSummary } from '../../types/api';

export function ModelsPage() {
  const consoleApi = useMemo(() => createConsoleApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [models, setModels] = useState<ConsoleModelSummary[]>([]);

  useEffect(() => {
    let cancelled = false;

    const loadModels = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextModels = await consoleApi.getModels();
        if (!cancelled) {
          setModels(nextModels);
        }
      } catch {
        if (!cancelled) {
          setError('Unable to load model summaries.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
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
      <h1>Models</h1>
      {isLoading ? <p>Loading model summaries…</p> : null}
      {error ? <p>{error}</p> : null}
      {models.length > 0 ? (
        <ul>
          {models.map((model) => (
            <li key={model.id}>
              <strong>{model.label}</strong>
              <p>Requests: {model.requests}</p>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
