import type {
  KnowledgeBaseSummary,
  KnowledgeDocumentSummary,
  KnowledgeRetrievalResult
} from '../../types/api';

type KnowledgePageViewProps = {
  authModelStrategy: string;
  currentDocumentTitle: string;
  currentDocumentContent: string;
  error: string | null;
  hasRetrievedKnowledge: boolean;
  isCreating: boolean;
  isDeletingDocumentId: string | null;
  isDeletingKnowledgeBase: boolean;
  isEditingDocument: boolean;
  isLoading: boolean;
  isRetrievingKnowledge: boolean;
  isSavingDocument: boolean;
  isSavingKnowledgeBase: boolean;
  knowledgeBaseId?: string;
  knowledgeBaseName: string;
  knowledgeBases: KnowledgeBaseSummary[];
  knowledgeDocuments: KnowledgeDocumentSummary[];
  lastRetrievedQuery: string;
  networkEnabledHint: boolean;
  retrievalQuery: string;
  retrievalResults: KnowledgeRetrievalResult[];
  returnTo: string | null;
  selectedKnowledgeBase: KnowledgeBaseSummary | null;
  onChangeDocumentContent: (value: string) => void;
  onChangeDocumentTitle: (value: string) => void;
  onChangeKnowledgeBaseName: (value: string) => void;
  onChangeRetrievalQuery: (value: string) => void;
  onCreateKnowledgeBase: () => void;
  onDeleteKnowledgeBase: () => void;
  onDeleteKnowledgeDocument: (document: KnowledgeDocumentSummary) => void;
  onEditKnowledgeDocument: (document: KnowledgeDocumentSummary) => void;
  onNavigateBackToChat: () => void;
  onNavigateBackToKnowledgeBases: () => void;
  onNavigateChatWorkspace: () => void;
  onNavigateSettings: () => void;
  onOpenKnowledgeBase: (knowledgeBaseID: string) => void;
  onResetDocumentEditor: () => void;
  onRetrieveKnowledge: () => void;
  onSaveKnowledgeBase: () => void;
  onSubmitKnowledgeDocument: () => void;
};

export function KnowledgePageView({
  authModelStrategy,
  currentDocumentContent,
  currentDocumentTitle,
  error,
  hasRetrievedKnowledge,
  isCreating,
  isDeletingDocumentId,
  isDeletingKnowledgeBase,
  isEditingDocument,
  isLoading,
  isRetrievingKnowledge,
  isSavingDocument,
  isSavingKnowledgeBase,
  knowledgeBaseId,
  knowledgeBaseName,
  knowledgeBases,
  knowledgeDocuments,
  lastRetrievedQuery,
  networkEnabledHint,
  onChangeDocumentContent,
  onChangeDocumentTitle,
  onChangeKnowledgeBaseName,
  onChangeRetrievalQuery,
  onCreateKnowledgeBase,
  onDeleteKnowledgeBase,
  onDeleteKnowledgeDocument,
  onEditKnowledgeDocument,
  onNavigateBackToChat,
  onNavigateBackToKnowledgeBases,
  onNavigateChatWorkspace,
  onNavigateSettings,
  onOpenKnowledgeBase,
  onResetDocumentEditor,
  onRetrieveKnowledge,
  onSaveKnowledgeBase,
  onSubmitKnowledgeDocument,
  retrievalQuery,
  retrievalResults,
  returnTo,
  selectedKnowledgeBase
}: KnowledgePageViewProps) {
  return (
    <section>
      <h1>{selectedKnowledgeBase ? selectedKnowledgeBase.name : 'Knowledge'}</h1>
      <p>
        {selectedKnowledgeBase
          ? 'Manage reusable documents in this knowledge base and search indexed snippets for relevant context.'
          : 'Organize reusable workspace context into knowledge bases and search them from each detail view.'}
      </p>
      {isLoading ? <p>{knowledgeBaseId ? 'Loading knowledge base…' : 'Loading knowledge bases…'}</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Model strategy: {authModelStrategy}</p>
      <p>Web suggestions: {networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
      <p>
        {networkEnabledHint
          ? 'Web suggestions are enabled for broader chat context alongside workspace knowledge retrieval.'
          : 'Enable web suggestions in settings if you want broader context beyond your indexed knowledge base.'}
      </p>
      {selectedKnowledgeBase ? (
        <>
          <label>
            Knowledge base name
            <input onChange={(event) => onChangeKnowledgeBaseName(event.target.value)} type="text" value={knowledgeBaseName} />
          </label>
          <button disabled={isSavingKnowledgeBase || knowledgeBaseName.trim() === ''} onClick={onSaveKnowledgeBase} type="button">
            Save knowledge base
          </button>
          <button disabled={isDeletingKnowledgeBase} onClick={onDeleteKnowledgeBase} type="button">
            Delete knowledge base
          </button>
          <p>Knowledge base ID: {selectedKnowledgeBase.id}</p>
          <p>Documents: {selectedKnowledgeBase.documentCount}</p>
          <label>
            Retrieval query
            <input onChange={(event) => onChangeRetrievalQuery(event.target.value)} type="text" value={retrievalQuery} />
          </label>
          <button disabled={isRetrievingKnowledge || retrievalQuery.trim() === ''} onClick={onRetrieveKnowledge} type="button">
            Search knowledge
          </button>
          {hasRetrievedKnowledge ? <h2>Matched snippets</h2> : null}
          {hasRetrievedKnowledge && retrievalResults.length === 0 ? <p>{`No matching snippets found for “${lastRetrievedQuery}”.`}</p> : null}
          {retrievalResults.length > 0 ? (
            <ul>
              {retrievalResults.map((result) => (
                <li key={`${result.documentId}-${result.snippet}`}>
                  <strong>{result.documentTitle}</strong>
                  <p>{result.snippet}</p>
                </li>
              ))}
            </ul>
          ) : null}
          <label>
            Document title
            <input onChange={(event) => onChangeDocumentTitle(event.target.value)} type="text" value={currentDocumentTitle} />
          </label>
          <label>
            Document content
            <textarea onChange={(event) => onChangeDocumentContent(event.target.value)} value={currentDocumentContent} />
          </label>
          <button disabled={isSavingDocument || currentDocumentTitle.trim() === ''} onClick={onSubmitKnowledgeDocument} type="button">
            {isEditingDocument ? 'Save document' : 'Create document'}
          </button>
          {isEditingDocument ? (
            <button disabled={isSavingDocument} onClick={onResetDocumentEditor} type="button">
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
                  <button onClick={() => onEditKnowledgeDocument(document)} type="button">
                    {`Edit document ${document.title}`}
                  </button>
                  <button
                    disabled={isDeletingDocumentId === document.id}
                    onClick={() => onDeleteKnowledgeDocument(document)}
                    type="button"
                  >
                    {`Delete document ${document.title}`}
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
          <button onClick={onNavigateBackToKnowledgeBases} type="button">
            Back to knowledge bases
          </button>
          {returnTo ? (
            <button onClick={onNavigateBackToChat} type="button">
              Back to chat
            </button>
          ) : null}
        </>
      ) : (
        <>
          <label>
            Knowledge base name
            <input onChange={(event) => onChangeKnowledgeBaseName(event.target.value)} type="text" value={knowledgeBaseName} />
          </label>
          <button disabled={isCreating || knowledgeBaseName.trim() === ''} onClick={onCreateKnowledgeBase} type="button">
            Create knowledge base
          </button>
          {!isLoading && knowledgeBases.length === 0 ? <p>No knowledge bases yet. Create one to start collecting workspace context.</p> : null}
          {knowledgeBases.length > 0 ? (
            <ul>
              {knowledgeBases.map((knowledgeBase) => (
                <li key={knowledgeBase.id}>
                  <strong>{knowledgeBase.name}</strong>
                  <p>Documents: {knowledgeBase.documentCount}</p>
                  <button onClick={() => onOpenKnowledgeBase(knowledgeBase.id)} type="button">
                    Open knowledge base
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
        </>
      )}
      <button onClick={onNavigateChatWorkspace} type="button">
        Open chat workspace
      </button>
      <button onClick={onNavigateSettings} type="button">
        Review workspace settings
      </button>
    </section>
  );
}
