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
  const [editingDocumentId, setEditingDocumentId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isDeletingKnowledgeBase, setIsDeletingKnowledgeBase] = useState(false);
  const [isDeletingDocumentId, setIsDeletingDocumentId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSavingDocument, setIsSavingDocument] = useState(false);
  const [isSavingKnowledgeBase, setIsSavingKnowledgeBase] = useState(false);
  const [knowledgeDocumentContent, setKnowledgeDocumentContent] = useState('');
  const [knowledgeDocumentTitle, setKnowledgeDocumentTitle] = useState('');
  const [knowledgeBaseName, setKnowledgeBaseName] = useState('');
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<KnowledgeDocumentSummary[]>([]);
  const [selectedKnowledgeBase, setSelectedKnowledgeBase] = useState<KnowledgeBaseSummary | null>(null);

  const resetDocumentEditor = () => {
    setEditingDocumentId(null);
    setKnowledgeDocumentTitle('');
    setKnowledgeDocumentContent('');
  };

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
            setKnowledgeBaseName(nextKnowledgeBase.name);
            setKnowledgeDocuments(nextKnowledgeDocuments);
            setKnowledgeBases([]);
            resetDocumentEditor();
          }
        } else {
          const nextKnowledgeBases = await knowledgeApi.listKnowledgeBases();
          if (!cancelled) {
            setKnowledgeBases(nextKnowledgeBases);
            setKnowledgeDocuments([]);
            setSelectedKnowledgeBase(null);
            setKnowledgeBaseName('');
            resetDocumentEditor();
          }
        }
      } catch {
        if (!cancelled) {
          setKnowledgeBases([]);
          setKnowledgeDocuments([]);
          setSelectedKnowledgeBase(null);
          setKnowledgeBaseName('');
          resetDocumentEditor();
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

  const handleSaveKnowledgeBase = async () => {
    if (!knowledgeBaseId || !selectedKnowledgeBase) {
      return;
    }

    const trimmedName = knowledgeBaseName.trim();
    if (trimmedName === '') {
      return;
    }

    setIsSavingKnowledgeBase(true);
    setError(null);

    try {
      const updatedKnowledgeBase = await knowledgeApi.updateKnowledgeBase(knowledgeBaseId, { name: trimmedName });
      setSelectedKnowledgeBase(updatedKnowledgeBase);
      setKnowledgeBaseName(updatedKnowledgeBase.name);
      setKnowledgeBases((current) =>
        current.map((knowledgeBase) => (knowledgeBase.id === updatedKnowledgeBase.id ? updatedKnowledgeBase : knowledgeBase))
      );
    } catch {
      setError('Unable to update knowledge base.');
    } finally {
      setIsSavingKnowledgeBase(false);
    }
  };

  const handleDeleteKnowledgeBase = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    setIsDeletingKnowledgeBase(true);
    setError(null);

    try {
      await knowledgeApi.deleteKnowledgeBase(knowledgeBaseId);
      navigate('/knowledge');
    } catch {
      setError('Unable to delete knowledge base.');
    } finally {
      setIsDeletingKnowledgeBase(false);
    }
  };

  const handleSubmitKnowledgeDocument = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    const trimmedTitle = knowledgeDocumentTitle.trim();
    const trimmedContent = knowledgeDocumentContent.trim();
    if (trimmedTitle === '') {
      return;
    }

    setIsSavingDocument(true);
    setError(null);

    try {
      if (editingDocumentId) {
        const updatedDocument = await knowledgeApi.updateKnowledgeDocument(knowledgeBaseId, editingDocumentId, {
          content: trimmedContent,
          title: trimmedTitle
        });
        setKnowledgeDocuments((current) =>
          current.map((document) => (document.id === editingDocumentId ? updatedDocument : document))
        );
      } else {
        const createdDocument = await knowledgeApi.createKnowledgeDocument(knowledgeBaseId, {
          content: trimmedContent,
          title: trimmedTitle
        });
        setKnowledgeDocuments((current) => [createdDocument, ...current]);
        setSelectedKnowledgeBase((current) =>
          current
            ? {
                ...current,
                documentCount: current.documentCount + 1
              }
            : current
        );
      }

      resetDocumentEditor();
    } catch {
      setError(editingDocumentId ? 'Unable to update knowledge document.' : 'Unable to create knowledge document.');
    } finally {
      setIsSavingDocument(false);
    }
  };

  const handleEditKnowledgeDocument = (document: KnowledgeDocumentSummary) => {
    setEditingDocumentId(document.id);
    setKnowledgeDocumentTitle(document.title);
    setKnowledgeDocumentContent(document.content);
  };

  const handleDeleteKnowledgeDocument = async (document: KnowledgeDocumentSummary) => {
    if (!knowledgeBaseId) {
      return;
    }

    setIsDeletingDocumentId(document.id);
    setError(null);

    try {
      await knowledgeApi.deleteKnowledgeDocument(knowledgeBaseId, document.id);
      setKnowledgeDocuments((current) => current.filter((currentDocument) => currentDocument.id !== document.id));
      setSelectedKnowledgeBase((current) =>
        current
          ? {
              ...current,
              documentCount: Math.max(current.documentCount - 1, 0)
            }
          : current
      );
      if (editingDocumentId === document.id) {
        resetDocumentEditor();
      }
    } catch {
      setError('Unable to delete knowledge document.');
    } finally {
      setIsDeletingDocumentId(null);
    }
  };

  const isEditingDocument = editingDocumentId !== null;

  return (
    <section>
      <h1>{selectedKnowledgeBase ? selectedKnowledgeBase.name : 'Knowledge'}</h1>
      <p>
        {selectedKnowledgeBase
          ? 'Manage reusable documents in this knowledge base while retrieval and indexing continue to mature.'
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
          <label>
            Knowledge base name
            <input onChange={(event) => setKnowledgeBaseName(event.target.value)} type="text" value={knowledgeBaseName} />
          </label>
          <button
            disabled={isSavingKnowledgeBase || knowledgeBaseName.trim() === ''}
            onClick={() => void handleSaveKnowledgeBase()}
            type="button"
          >
            Save knowledge base
          </button>
          <button disabled={isDeletingKnowledgeBase} onClick={() => void handleDeleteKnowledgeBase()} type="button">
            Delete knowledge base
          </button>
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
            disabled={isSavingDocument || knowledgeDocumentTitle.trim() === ''}
            onClick={() => void handleSubmitKnowledgeDocument()}
            type="button"
          >
            {isEditingDocument ? 'Save document' : 'Create document'}
          </button>
          {isEditingDocument ? (
            <button disabled={isSavingDocument} onClick={resetDocumentEditor} type="button">
              Cancel document edit
            </button>
          ) : null}
          {knowledgeDocuments.length === 0 ? <p>No documents yet. Add one to seed this knowledge base.</p> : null}
          {knowledgeDocuments.length > 0 ? (
            <ul>
              {knowledgeDocuments.map((document) => (
                <li key={document.id}>
                  <strong>{document.title}</strong>
                  <p>{document.content}</p>
                  <button onClick={() => handleEditKnowledgeDocument(document)} type="button">
                    {`Edit document ${document.title}`}
                  </button>
                  <button
                    disabled={isDeletingDocumentId === document.id}
                    onClick={() => void handleDeleteKnowledgeDocument(document)}
                    type="button"
                  >
                    {`Delete document ${document.title}`}
                  </button>
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
