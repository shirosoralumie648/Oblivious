import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createKnowledgeApi } from '../../features/knowledge/api';
import { createHttpClient } from '../../services/http/client';
import type { KnowledgeBaseSummary } from '../../types/api';

export function KnowledgePage() {
  const navigate = useNavigate();
  const { authState } = useAppContext();
  const knowledgeApi = useMemo(() => createKnowledgeApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [knowledgeBaseName, setKnowledgeBaseName] = useState('');
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);

  useEffect(() => {
    let cancelled = false;

    const loadKnowledgeBases = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const nextKnowledgeBases = await knowledgeApi.listKnowledgeBases();
        if (!cancelled) {
          setKnowledgeBases(nextKnowledgeBases);
        }
      } catch {
        if (!cancelled) {
          setKnowledgeBases([]);
          setError('Unable to load knowledge bases.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadKnowledgeBases();

    return () => {
      cancelled = true;
    };
  }, [knowledgeApi]);

  const handleCreateKnowledgeBase = async () => {
    const trimmedName = knowledgeBaseName.trim();
    if (trimmedName === '') {
      return;
    }

    setIsCreating(true);
    setError(null);

    try {
      const createdKnowledgeBase = await knowledgeApi.createKnowledgeBase({ name: trimmedName });
      setKnowledgeBases((current) => [createdKnowledgeBase, ...current]);
      setKnowledgeBaseName('');
    } catch {
      setError('Unable to create knowledge base.');
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <section>
      <h1>Knowledge</h1>
      <p>Organize reusable workspace context into knowledge bases before retrieval and document ingestion land.</p>
      {isLoading ? <p>Loading knowledge bases…</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
      <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
      <p>
        {authState.preferences?.networkEnabledHint
          ? 'Web suggestions are enabled for broader chat context while dedicated knowledge retrieval is still pending.'
          : 'Enable web suggestions in settings if you want broader context before dedicated knowledge bases arrive.'}
      </p>
      <label>
        Knowledge base name
        <input onChange={(event) => setKnowledgeBaseName(event.target.value)} type="text" value={knowledgeBaseName} />
      </label>
      <button disabled={isCreating || knowledgeBaseName.trim() === ''} onClick={() => void handleCreateKnowledgeBase()} type="button">
        Create knowledge base
      </button>
      {!isLoading && knowledgeBases.length === 0 ? (
        <p>No knowledge bases yet. Create one to start collecting workspace context.</p>
      ) : null}
      {knowledgeBases.length > 0 ? (
        <ul>
          {knowledgeBases.map((knowledgeBase) => (
            <li key={knowledgeBase.id}>
              <strong>{knowledgeBase.name}</strong>
              <p>Documents: {knowledgeBase.documentCount}</p>
            </li>
          ))}
        </ul>
      ) : null}
      <button onClick={() => navigate('/chat')} type="button">
        Open chat workspace
      </button>
      <button onClick={() => navigate('/settings')} type="button">
        Review workspace settings
      </button>
    </section>
  );
}
