import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createKnowledgeApi } from '../../features/knowledge/api';
import { createHttpClient } from '../../services/http/client';
import type { KnowledgeBaseSummary, KnowledgeDocumentSummary } from '../../types/api';

export function KnowledgePage() {
  const navigate = useNavigate();
  const { knowledgeBaseId } = useParams<{ knowledgeBaseId?: string }>();
  const { authState } = useAppContext();
  const knowledgeApi = useMemo(() => createKnowledgeApi(createHttpClient()), []);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isCreatingDocument, setIsCreatingDocument] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [knowledgeDocumentContent, setKnowledgeDocumentContent] = useState('');
  const [knowledgeDocumentTitle, setKnowledgeDocumentTitle] = useState('');
  const [knowledgeBaseName, setKnowledgeBaseName] = useState('');
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<KnowledgeDocumentSummary[]>([]);
  const [selectedKnowledgeBase, setSelectedKnowledgeBase] = useState<KnowledgeBaseSummary | null>(null);

  useEffect(() => {
    let cancelled = false;

    const loadKnowledge = async () => {
      setIsLoading(true);
      setError(null);

      try {
        if (knowledgeBaseId) {
          const [nextKnowledgeBase, nextKnowledgeDocuments] = await Promise.all([
            knowledgeApi.getKnowledgeBase(knowledgeBaseId),
            knowledgeApi.listKnowledgeDocuments(knowledgeBaseId)
          ]);
          if (!cancelled) {
            setSelectedKnowledgeBase(nextKnowledgeBase);
            setKnowledgeDocuments(nextKnowledgeDocuments);
            setKnowledgeBases([]);
          }
        } else {
          const nextKnowledgeBases = await knowledgeApi.listKnowledgeBases();
          if (!cancelled) {
            setKnowledgeBases(nextKnowledgeBases);
            setKnowledgeDocuments([]);
            setSelectedKnowledgeBase(null);
          }
        }
      } catch {
        if (!cancelled) {
          setKnowledgeBases([]);
          setKnowledgeDocuments([]);
          setSelectedKnowledgeBase(null);
          setError('Unable to load knowledge bases.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadKnowledge();

    return () => {
      cancelled = true;
    };
  }, [knowledgeApi, knowledgeBaseId]);

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

  const handleCreateKnowledgeDocument = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    const trimmedTitle = knowledgeDocumentTitle.trim();
    const trimmedContent = knowledgeDocumentContent.trim();
    if (trimmedTitle === '') {
      return;
    }

    setIsCreatingDocument(true);
    setError(null);

    try {
      const createdDocument = await knowledgeApi.createKnowledgeDocument(knowledgeBaseId, {
        content: trimmedContent,
        title: trimmedTitle
      });
      setKnowledgeDocuments((current) => [createdDocument, ...current]);
      setKnowledgeDocumentTitle('');
      setKnowledgeDocumentContent('');
    } catch {
      setError('Unable to create knowledge document.');
    } finally {
      setIsCreatingDocument(false);
    }
  };

  return (
    <section>
      <h1>{selectedKnowledgeBase ? selectedKnowledgeBase.name : 'Knowledge'}</h1>
      <p>
        {selectedKnowledgeBase
          ? 'Review the current knowledge-base shell while document ingestion is still pending.'
          : 'Organize reusable workspace context into knowledge bases before retrieval and document ingestion land.'}
      </p>
      {isLoading ? <p>{knowledgeBaseId ? 'Loading knowledge base…' : 'Loading knowledge bases…'}</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
      <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
      <p>
        {authState.preferences?.networkEnabledHint
          ? 'Web suggestions are enabled for broader chat context while dedicated knowledge retrieval is still pending.'
          : 'Enable web suggestions in settings if you want broader context before dedicated knowledge bases arrive.'}
      </p>
      {selectedKnowledgeBase ? (
        <>
          <p>Knowledge base ID: {selectedKnowledgeBase.id}</p>
          <p>Documents: {selectedKnowledgeBase.documentCount}</p>
          <label>
            Document title
            <input onChange={(event) => setKnowledgeDocumentTitle(event.target.value)} type="text" value={knowledgeDocumentTitle} />
          </label>
          <label>
            Document content
            <textarea onChange={(event) => setKnowledgeDocumentContent(event.target.value)} value={knowledgeDocumentContent} />
          </label>
          <button
            disabled={isCreatingDocument || knowledgeDocumentTitle.trim() === ''}
            onClick={() => void handleCreateKnowledgeDocument()}
            type="button"
          >
            Create document
          </button>
          {knowledgeDocuments.length === 0 ? <p>No documents yet. Add one to seed this knowledge base.</p> : null}
          {knowledgeDocuments.length > 0 ? (
            <ul>
              {knowledgeDocuments.map((document) => (
                <li key={document.id}>
                  <strong>{document.title}</strong>
                  <p>{document.content}</p>
                </li>
              ))}
            </ul>
          ) : null}
          <button onClick={() => navigate('/knowledge')} type="button">
            Back to knowledge bases
          </button>
        </>
      ) : (
        <>
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
                  <button onClick={() => navigate(`/knowledge/${knowledgeBase.id}`)} type="button">
                    Open knowledge base
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
        </>
      )}
      <button onClick={() => navigate('/chat')} type="button">
        Open chat workspace
      </button>
      <button onClick={() => navigate('/settings')} type="button">
        Review workspace settings
      </button>
    </section>
  );
}
