import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const expandedSupportedUploadAccept =
  '.txt,.text,.md,.markdown,.pdf,.docx,.html,.htm,.csv,.xlsx,.xls,.pptx,.json,.xml,text/plain,text/markdown,text/x-markdown,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,text/html,application/xhtml+xml,text/csv,application/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.openxmlformats-officedocument.presentationml.presentation,application/json,text/json,application/xml,text/xml';

const createKnowledgeBase = vi.fn();
const createKnowledgeDocument = vi.fn();
const createRetrievalTestCase = vi.fn();
const deleteKnowledgeBase = vi.fn();
const deleteKnowledgeDocument = vi.fn();
const getKnowledgeBase = vi.fn();
const listKnowledgeDocumentChunks = vi.fn();
const listKnowledgeDocumentIngestionJobs = vi.fn();
const listKnowledgeDocumentVersions = vi.fn();
const listKnowledgeDocuments = vi.fn();
const listKnowledgeBases = vi.fn();
const listRetrievalTestCases = vi.fn();
const navigate = vi.fn();
const retrieveKnowledge = vi.fn();
const runRetrievalTestCases = vi.fn();
const mergeKnowledgeDocumentChunks = vi.fn();
const splitKnowledgeDocumentChunk = vi.fn();
const updateKnowledgeBase = vi.fn();
const updateKnowledgeDocumentChunk = vi.fn();
const updateKnowledgeDocument = vi.fn();
const uploadKnowledgeDocument = vi.fn();
const routeState = vi.hoisted(() => ({
  knowledgeBaseId: undefined as string | undefined
}));

const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat' as const,
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    },
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  }
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');

  return {
    ...actual,
    useNavigate: () => navigate,
    useParams: () => ({ knowledgeBaseId: routeState.knowledgeBaseId })
  };
});

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

vi.mock('../../features/knowledge/api', () => ({
  createKnowledgeApi: () => ({
    createKnowledgeBase,
    createKnowledgeDocument,
    createRetrievalTestCase,
    deleteKnowledgeBase,
    deleteKnowledgeDocument,
    getKnowledgeBase,
    listKnowledgeDocumentChunks,
    listKnowledgeDocumentIngestionJobs,
    listKnowledgeDocumentVersions,
    listKnowledgeDocuments,
    listKnowledgeBases,
    listRetrievalTestCases,
    retrieveKnowledge,
    runRetrievalTestCases,
    mergeKnowledgeDocumentChunks,
    splitKnowledgeDocumentChunk,
    updateKnowledgeBase,
    updateKnowledgeDocumentChunk,
    updateKnowledgeDocument,
    uploadKnowledgeDocument
  })
}));

import { KnowledgePage } from './KnowledgePage';

describe('KnowledgePage', () => {
  beforeEach(() => {
    appContext.authState.preferences = {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    };
    createKnowledgeBase.mockReset();
    createKnowledgeDocument.mockReset();
    createRetrievalTestCase.mockReset();
    deleteKnowledgeBase.mockReset();
    deleteKnowledgeDocument.mockReset();
    getKnowledgeBase.mockReset();
    listKnowledgeDocumentChunks.mockReset();
    listKnowledgeDocumentIngestionJobs.mockReset();
    listKnowledgeDocumentIngestionJobs.mockResolvedValue([]);
    listKnowledgeDocumentVersions.mockReset();
    listKnowledgeDocuments.mockReset();
    listKnowledgeBases.mockReset();
    listRetrievalTestCases.mockReset();
    listRetrievalTestCases.mockResolvedValue([]);
    navigate.mockReset();
    routeState.knowledgeBaseId = undefined;
    retrieveKnowledge.mockReset();
    runRetrievalTestCases.mockReset();
    mergeKnowledgeDocumentChunks.mockReset();
    splitKnowledgeDocumentChunk.mockReset();
    updateKnowledgeBase.mockReset();
    updateKnowledgeDocumentChunk.mockReset();
    updateKnowledgeDocument.mockReset();
    uploadKnowledgeDocument.mockReset();
  });

  it('loads and renders knowledge bases with workspace context', async () => {
    listKnowledgeBases.mockResolvedValue([
      {
        documentCount: 4,
        id: 'kb_1',
        name: 'Product Docs',
        updatedAt: '2026-04-03T09:00:00Z'
      },
      {
        documentCount: 1,
        id: 'kb_2',
        name: 'Runbooks',
        updatedAt: '2026-04-02T12:00:00Z'
      }
    ]);

    render(<KnowledgePage />);

    expect(screen.getByText('Loading knowledge bases…')).toBeInTheDocument();
    expect(await screen.findByText('Product Docs')).toBeInTheDocument();
    expect(screen.getByText('Documents: 4')).toBeInTheDocument();
    expect(screen.getByText('Model strategy: balanced')).toBeInTheDocument();
    expect(screen.getByText('Web suggestions: Disabled')).toBeInTheDocument();
  });

  it('creates a knowledge base from the page', async () => {
    listKnowledgeBases.mockResolvedValue([]);
    createKnowledgeBase.mockResolvedValue({
      documentCount: 0,
      id: 'kb_3',
      name: 'Research Vault',
      updatedAt: '2026-04-03T10:00:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByText('No knowledge bases yet. Create one to start collecting workspace context.');
    fireEvent.change(screen.getByLabelText('Knowledge base name'), { target: { value: 'Research Vault' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create knowledge base' }));

    await waitFor(() => {
      expect(createKnowledgeBase).toHaveBeenCalledWith({ name: 'Research Vault' });
    });
    expect(screen.getByText('Research Vault')).toBeInTheDocument();
  });

  it('routes users to settings from the knowledge page', async () => {
    listKnowledgeBases.mockResolvedValue([]);

    render(<KnowledgePage />);

    await screen.findByRole('button', { name: 'Review workspace settings' });
    fireEvent.click(screen.getByRole('button', { name: 'Review workspace settings' }));

    expect(navigate).toHaveBeenCalledWith('/settings');
  });

  it('renders a single knowledge-base detail view when the route includes an id', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 9,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);

    render(<KnowledgePage />);

    expect(screen.getByText('Loading knowledge base…')).toBeInTheDocument();
    expect(await screen.findByRole('heading', { name: 'Architecture Notes' })).toBeInTheDocument();
    expect(screen.getByText('Knowledge base ID: kb_9')).toBeInTheDocument();
    expect(screen.getByText('Documents: 9')).toBeInTheDocument();
    expect(screen.getByText('Overview')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back to knowledge bases' })).toBeInTheDocument();
  });

  it('renders knowledge document ingestion job status without counting pending uploads as documents', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    listKnowledgeDocumentIngestionJobs.mockResolvedValue([
      {
        attempts: 5,
        availableAt: '2026-04-03T12:45:00Z',
        completedAt: '2026-04-03T12:50:00Z',
        createdAt: '2026-04-03T12:40:00Z',
        error: 'parser failed',
        id: 'kig_dead',
        knowledgeBaseId: 'kb_9',
        maxAttempts: 5,
        status: 'dead_letter',
        title: 'Broken PDF',
        updatedAt: '2026-04-03T12:50:00Z'
      }
    ]);

    render(<KnowledgePage />);

    expect(await screen.findByRole('heading', { name: 'Architecture Notes' })).toBeInTheDocument();
    expect(listKnowledgeDocumentIngestionJobs).toHaveBeenCalledWith('kb_9');
    expect(screen.getByRole('heading', { name: 'Ingestion jobs' })).toBeInTheDocument();
    expect(screen.getByText('Broken PDF')).toBeInTheDocument();
    expect(screen.getByText('Status: dead letter')).toBeInTheDocument();
    expect(screen.getByText('Attempts: 5/5')).toBeInTheDocument();
    expect(screen.getByText('Error: parser failed')).toBeInTheDocument();
    expect(screen.getByText('Documents: 1')).toBeInTheDocument();
    expect(screen.getByText('No documents yet. Add one to seed this knowledge base.')).toBeInTheDocument();
  });

  it('shows a back-to-chat action when returnTo is present on the knowledge route', async () => {
    window.history.replaceState({}, '', '/knowledge/kb_9?returnTo=%2Fchat%2Fconversation_1');
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);

    render(<KnowledgePage />);

    expect(await screen.findByRole('button', { name: 'Back to chat' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Back to chat' }));

    expect(navigate).toHaveBeenCalledWith('/chat/conversation_1');
  });

  it('creates a document inside the selected knowledge base', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    createKnowledgeDocument.mockResolvedValue({
      content: 'Initial architecture draft',
      id: 'doc_9',
      title: 'Draft',
      updatedAt: '2026-04-03T12:00:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Document title'), { target: { value: 'Draft' } });
    fireEvent.change(screen.getByLabelText('Document content'), { target: { value: 'Initial architecture draft' } });
    fireEvent.change(screen.getByLabelText('Document source URL'), { target: { value: ' https://docs.example/draft.md ' } });
    fireEvent.change(screen.getByLabelText('Document source page'), { target: { value: '5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create document' }));

    await waitFor(() => {
      expect(createKnowledgeDocument).toHaveBeenCalledWith('kb_9', {
        content: 'Initial architecture draft',
        pageNumber: 5,
        sourceUrl: 'https://docs.example/draft.md',
        title: 'Draft'
      });
    });
    expect(screen.getByText('Draft')).toBeInTheDocument();
  });

  it('creates versioned knowledge documents with an update strategy', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    createKnowledgeDocument.mockResolvedValue({
      content: 'Initial architecture draft',
      documentVersion: 'v3',
      id: 'doc_9',
      title: 'Draft',
      updateStrategy: 'versioned',
      updatedAt: '2026-04-03T12:00:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Document title'), { target: { value: 'Draft' } });
    fireEvent.change(screen.getByLabelText('Document content'), { target: { value: 'Initial architecture draft' } });
    fireEvent.change(screen.getByLabelText('Document version'), { target: { value: 'v3' } });
    fireEvent.change(screen.getByLabelText('Update strategy'), { target: { value: 'versioned' } });
    fireEvent.click(screen.getByRole('button', { name: 'Create document' }));

    await waitFor(() => {
      expect(createKnowledgeDocument).toHaveBeenCalledWith('kb_9', {
        content: 'Initial architecture draft',
        documentVersion: 'v3',
        title: 'Draft',
        updateStrategy: 'versioned'
      });
    });
    expect(screen.getByText('Version: v3')).toBeInTheDocument();
  });

  it('uploads a document file and clears stale retrieval and chunk details', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 24,
        chunkId: 'kdc_1',
        chunkIndex: 0,
        content: 'First architecture chunk.',
        documentVersion: 'v2',
        estimatedTokenCount: 6,
        metadata: {
          documentVersion: 'v2'
        }
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_1',
        chunkIndex: 0,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'embedding_rag',
        similarity: 0.88,
        source: {
          chunkId: 'kdc_1',
          chunkIndex: 0,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'System boundaries include deployment controls.'
      }
    ]);
    uploadKnowledgeDocument.mockResolvedValue({
      attempts: 0,
      availableAt: '2026-04-03T12:45:00Z',
      createdAt: '2026-04-03T12:45:00Z',
      id: 'kig_upload',
      knowledgeBaseId: 'kb_9',
      maxAttempts: 5,
      status: 'pending',
      title: 'Uploaded Runbook',
      updatedAt: '2026-04-03T12:45:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));
    expect(await screen.findByText('System boundaries include deployment controls.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View chunks for Overview' }));
    expect(await screen.findByRole('heading', { name: 'Chunks for Overview' })).toBeInTheDocument();
    expect(screen.getAllByText('First architecture chunk.').length).toBeGreaterThan(0);

    const file = new File(['%PDF-1.4 uploaded body'], 'runbook.pdf', { type: 'application/pdf' });
    fireEvent.change(screen.getByLabelText('Document file'), { target: { files: [file] } });
    fireEvent.change(screen.getByLabelText('Upload title'), { target: { value: 'Uploaded Runbook' } });
    fireEvent.change(screen.getByLabelText('Upload document version'), { target: { value: 'v4' } });
    fireEvent.change(screen.getByLabelText('Upload source URL'), { target: { value: ' https://docs.example/runbook.pdf ' } });
    fireEvent.change(screen.getByLabelText('Upload source page'), { target: { value: '12' } });
    fireEvent.change(screen.getByLabelText('Upload update strategy'), { target: { value: 'versioned' } });
    fireEvent.click(screen.getByRole('button', { name: 'Upload document' }));

    await waitFor(() => {
      expect(uploadKnowledgeDocument).toHaveBeenCalledWith('kb_9', {
        documentVersion: 'v4',
        file,
        pageNumber: 12,
        sourceUrl: 'https://docs.example/runbook.pdf',
        title: 'Uploaded Runbook',
        updateStrategy: 'versioned'
      });
    });

    const uploadedTitle = screen.getByText('Uploaded Runbook');
    const existingTitle = screen.getByText('Overview');
    expect(uploadedTitle.compareDocumentPosition(existingTitle) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByText('Status: pending')).toBeInTheDocument();
    expect(screen.getByText('Attempts: 0/5')).toBeInTheDocument();
    expect(screen.queryByText('System boundaries include deployment controls.')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Chunks for Overview' })).not.toBeInTheDocument();
    expect(screen.getByText('Documents: 1')).toBeInTheDocument();
  });

  it('uploads a DOCX document file now that the backend parser supports it', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    uploadKnowledgeDocument.mockResolvedValue({
      attempts: 0,
      availableAt: '2026-04-03T13:00:00Z',
      createdAt: '2026-04-03T13:00:00Z',
      id: 'kig_docx',
      knowledgeBaseId: 'kb_9',
      maxAttempts: 5,
      status: 'pending',
      title: 'DOCX Runbook',
      updatedAt: '2026-04-03T13:00:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    const fileInput = screen.getByLabelText('Document file');
    expect(fileInput).toHaveAttribute('accept', expandedSupportedUploadAccept);

    const file = new File(['docx body'], 'runbook.docx', {
      type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
    });
    fireEvent.change(fileInput, { target: { files: [file] } });
    fireEvent.change(screen.getByLabelText('Upload title'), { target: { value: 'DOCX Runbook' } });
    fireEvent.change(screen.getByLabelText('Upload document version'), { target: { value: 'v5' } });
    fireEvent.change(screen.getByLabelText('Upload update strategy'), { target: { value: 'versioned' } });
    fireEvent.click(screen.getByRole('button', { name: 'Upload document' }));

    await waitFor(() => {
      expect(uploadKnowledgeDocument).toHaveBeenCalledWith('kb_9', {
        documentVersion: 'v5',
        file,
        title: 'DOCX Runbook',
        updateStrategy: 'versioned'
      });
    });
    expect(screen.queryByText(/DOCX parsing is not available yet/)).not.toBeInTheDocument();
    expect(screen.getByText('DOCX Runbook')).toBeInTheDocument();
    expect(screen.getByText('Status: pending')).toBeInTheDocument();
  });

  it('uploads a CSV document file through the expanded parser path', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    uploadKnowledgeDocument.mockResolvedValue({
      attempts: 0,
      availableAt: '2026-04-03T13:30:00Z',
      createdAt: '2026-04-03T13:30:00Z',
      id: 'kig_csv',
      knowledgeBaseId: 'kb_9',
      maxAttempts: 5,
      status: 'pending',
      title: 'CSV Matrix',
      updatedAt: '2026-04-03T13:30:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    const file = new File(['title,owner\nDeploy,Ops'], 'matrix.csv', {
      type: 'text/csv'
    });
    fireEvent.change(screen.getByLabelText('Document file'), { target: { files: [file] } });
    fireEvent.change(screen.getByLabelText('Upload title'), { target: { value: 'CSV Matrix' } });
    fireEvent.change(screen.getByLabelText('Upload document version'), { target: { value: 'v6' } });
    fireEvent.change(screen.getByLabelText('Upload update strategy'), { target: { value: 'incremental' } });
    fireEvent.click(screen.getByRole('button', { name: 'Upload document' }));

    await waitFor(() => {
      expect(uploadKnowledgeDocument).toHaveBeenCalledWith('kb_9', {
        documentVersion: 'v6',
        file,
        title: 'CSV Matrix',
        updateStrategy: 'incremental'
      });
    });
    expect(screen.queryByText(/Legacy \.doc parsing is not available yet/)).not.toBeInTheDocument();
    expect(screen.getByText('CSV Matrix')).toBeInTheDocument();
    expect(screen.getByText('Status: pending')).toBeInTheDocument();
  });

  it('blocks unsupported document upload formats before calling the API', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    const fileInput = screen.getByLabelText('Document file');
    expect(fileInput).toHaveAttribute('accept', expandedSupportedUploadAccept);

    fireEvent.change(fileInput, {
      target: {
        files: [
          new File(['doc body'], 'runbook.doc', {
            type: 'application/msword'
          })
        ]
      }
    });

    expect(
      await screen.findByText(
        'Knowledge document uploads currently support .txt, .md, PDF, DOCX, HTML, CSV, XLSX/XLS, PPTX, JSON, and XML files. Legacy .doc parsing is not available yet.'
      )
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Upload document' })).toBeDisabled();
    expect(uploadKnowledgeDocument).not.toHaveBeenCalled();
  });

  it('renames the selected knowledge base', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    updateKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Decisions',
      updatedAt: '2026-04-03T12:30:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Knowledge base name'), { target: { value: 'Architecture Decisions' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save knowledge base' }));

    await waitFor(() => {
      expect(updateKnowledgeBase).toHaveBeenCalledWith('kb_9', { name: 'Architecture Decisions' });
    });
    expect(screen.getByRole('heading', { name: 'Architecture Decisions' })).toBeInTheDocument();
  });

  it('saves knowledge-base level RAG retrieval and chunking configuration', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      chunkOverlap: 80,
      chunkSize: 900,
      chunkStrategy: 'semantic',
      documentCount: 1,
      embeddingModel: 'text-embedding-3-small',
      id: 'kb_9',
      name: 'Architecture Notes',
      rerankTopK: 8,
      rerankerModel: 'bge-reranker-base',
      retrievalMode: 'hybrid',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    updateKnowledgeBase.mockResolvedValue({
      chunkOverlap: 120,
      chunkSize: 1200,
      chunkStrategy: 'qa_split',
      documentCount: 1,
      embeddingModel: 'text-embedding-3-large',
      id: 'kb_9',
      name: 'Architecture Notes',
      rerankTopK: 10,
      rerankerModel: 'bge-reranker-large',
      retrievalMode: 'hybrid_rerank',
      updatedAt: '2026-04-03T12:30:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    expect(screen.getByText('Retrieval strategy: hybrid')).toBeInTheDocument();
    expect(screen.getByText('Chunking: semantic · 900 chars · 80 overlap')).toBeInTheDocument();
    expect(screen.getByText('Reranking: bge-reranker-base · top 8')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Default retrieval strategy'), { target: { value: 'hybrid_rerank' } });
    fireEvent.change(screen.getByLabelText('Chunking strategy'), { target: { value: 'qa_split' } });
    fireEvent.change(screen.getByLabelText('Chunk size'), { target: { value: '1200' } });
    fireEvent.change(screen.getByLabelText('Chunk overlap'), { target: { value: '120' } });
    fireEvent.change(screen.getByLabelText('Embedding model'), { target: { value: 'text-embedding-3-large' } });
    fireEvent.change(screen.getByLabelText('Reranker model'), { target: { value: 'bge-reranker-large' } });
    fireEvent.change(screen.getByLabelText('Rerank top K'), { target: { value: '10' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save knowledge base' }));

    await waitFor(() => {
      expect(updateKnowledgeBase).toHaveBeenCalledWith('kb_9', {
        chunkOverlap: 120,
        chunkSize: 1200,
        chunkStrategy: 'qa_split',
        embeddingModel: 'text-embedding-3-large',
        name: 'Architecture Notes',
        rerankTopK: 10,
        rerankerModel: 'bge-reranker-large',
        retrievalMode: 'hybrid_rerank'
      });
    });
    expect(screen.getByText('Retrieval strategy: hybrid_rerank')).toBeInTheDocument();
    expect(screen.getByText('Chunking: qa_split · 1200 chars · 120 overlap')).toBeInTheDocument();
    expect(screen.getByText('Reranking: bge-reranker-large · top 10')).toBeInTheDocument();
  });

  it('updates and deletes documents inside the selected knowledge base', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    updateKnowledgeDocument.mockResolvedValue({
      content: 'Updated boundaries',
      id: 'doc_1',
      title: 'Overview v2',
      updatedAt: '2026-04-03T12:15:00Z'
    });
    deleteKnowledgeDocument.mockResolvedValue(undefined);

    render(<KnowledgePage />);

    await screen.findByText('Overview');
    fireEvent.click(screen.getByRole('button', { name: 'Edit document Overview' }));
    fireEvent.change(screen.getByLabelText('Document title'), { target: { value: 'Overview v2' } });
    fireEvent.change(screen.getByLabelText('Document content'), { target: { value: 'Updated boundaries' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save document' }));

    await waitFor(() => {
      expect(updateKnowledgeDocument).toHaveBeenCalledWith('kb_9', 'doc_1', {
        content: 'Updated boundaries',
        title: 'Overview v2'
      });
    });
    expect(screen.getByText('Overview v2')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Delete document Overview v2' }));

    await waitFor(() => {
      expect(deleteKnowledgeDocument).toHaveBeenCalledWith('kb_9', 'doc_1');
    });
    expect(screen.queryByText('Overview v2')).not.toBeInTheDocument();
  });

  it('deletes the selected knowledge base and returns to the list route', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    deleteKnowledgeBase.mockResolvedValue(undefined);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.click(screen.getByRole('button', { name: 'Delete knowledge base' }));

    await waitFor(() => {
      expect(deleteKnowledgeBase).toHaveBeenCalledWith('kb_9');
    });
    expect(navigate).toHaveBeenCalledWith('/knowledge');
  });

  it('retrieves matching snippets inside the selected knowledge base', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_1',
        chunkIndex: 2,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'embedding_rag',
        similarity: 0.93,
        source: {
          chunkId: 'kdc_1',
          chunkIndex: 2,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'System boundaries include deployment controls.'
      },
      {
        chunkId: 'kdc_2',
        chunkIndex: 0,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'keyword',
        similarity: 0.79,
        source: {
          chunkId: 'kdc_2',
          chunkIndex: 0,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'Deployment controls cover release approvals.'
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    await waitFor(() => {
      expect(retrieveKnowledge).toHaveBeenCalledWith('kb_9', {
        query: 'deployment'
      });
    });
    expect(screen.getByText('System boundaries include deployment controls.')).toBeInTheDocument();
    expect(screen.getByText('Score: 93%')).toBeInTheDocument();
    expect(screen.getByText('Method: embedding_rag')).toBeInTheDocument();
    expect(screen.getByText('Source: Overview · chunk 3 · kdc_1')).toBeInTheDocument();
    expect(screen.getByText(/Retrieval metrics: 2 hits · avg 86% · \d+ ms/)).toBeInTheDocument();
  });

  it('adds a retrieval result to the knowledge test set', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    const result = {
      chunkId: 'kdc_1',
      chunkIndex: 2,
      documentId: 'doc_1',
      documentTitle: 'Overview',
      retrievalMethod: 'hybrid',
      similarity: 0.93,
      source: {
        chunkId: 'kdc_1',
        chunkIndex: 2,
        documentId: 'doc_1',
        documentTitle: 'Overview'
      },
      snippet: 'System boundaries include deployment controls.'
    };
    retrieveKnowledge.mockResolvedValue([result]);
    createRetrievalTestCase.mockResolvedValue({
      expectedChunkId: 'kdc_1',
      expectedChunkIndex: 2,
      expectedDocumentId: 'doc_1',
      id: 'krtc_1',
      knowledgeBaseId: 'kb_9',
      query: 'deployment'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));
    await screen.findByText('System boundaries include deployment controls.');
    fireEvent.click(screen.getByRole('button', { name: 'Add result kdc_1 to test set' }));

    await waitFor(() => {
      expect(createRetrievalTestCase).toHaveBeenCalledWith('kb_9', {
        expectedResult: result,
        query: 'deployment'
      });
    });
    expect(screen.getByText('Saved retrieval test case krtc_1')).toBeInTheDocument();
  });

  it('lists saved retrieval test cases and runs the retrieval evaluation report', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    listRetrievalTestCases.mockResolvedValue([
      {
        expectedChunkId: 'kdc_1',
        expectedChunkIndex: 2,
        expectedDocumentId: 'doc_1',
        expectedDocumentTitle: 'Overview',
        id: 'krtc_1',
        knowledgeBaseId: 'kb_9',
        query: 'deployment rollback'
      },
      {
        expectedChunkId: 'kdc_missing',
        expectedChunkIndex: 0,
        expectedDocumentId: 'doc_missing',
        expectedDocumentTitle: 'Missing',
        id: 'krtc_2',
        knowledgeBaseId: 'kb_9',
        query: 'billing controls'
      }
    ]);
    runRetrievalTestCases.mockResolvedValue({
      failed: 1,
      knowledgeBaseId: 'kb_9',
      passed: 1,
      ranAt: '2026-06-05T12:00:00Z',
      results: [
        {
          actualResult: {
            chunkId: 'kdc_1',
            chunkIndex: 2,
            documentId: 'doc_1',
            documentTitle: 'Overview',
            retrievalMethod: 'hybrid',
            similarity: 0.92,
            snippet: 'Deployment rollback plans belong in the release runbook.',
            source: {
              chunkId: 'kdc_1',
              chunkIndex: 2,
              documentId: 'doc_1',
              documentTitle: 'Overview'
            }
          },
          expectedResult: {
            chunkId: 'kdc_1',
            chunkIndex: 2,
            documentId: 'doc_1',
            documentTitle: 'Overview',
            retrievalMethod: 'hybrid',
            similarity: 0.92,
            snippet: 'Deployment rollback plans belong in the release runbook.',
            source: {
              chunkId: 'kdc_1',
              chunkIndex: 2,
              documentId: 'doc_1',
              documentTitle: 'Overview'
            }
          },
          passed: true,
          query: 'deployment rollback',
          rank: 1,
          testCaseId: 'krtc_1'
        },
        {
          expectedResult: {
            chunkId: 'kdc_missing',
            chunkIndex: 0,
            documentId: 'doc_missing',
            documentTitle: 'Missing',
            retrievalMethod: 'hybrid',
            similarity: 0,
            snippet: '',
            source: {
              chunkId: 'kdc_missing',
              chunkIndex: 0,
              documentId: 'doc_missing',
              documentTitle: 'Missing'
            }
          },
          passed: false,
          query: 'billing controls',
          rank: 0,
          reason: 'expected retrieval result was not returned',
          testCaseId: 'krtc_2'
        }
      ],
      total: 2
    });

    render(<KnowledgePage />);

    await waitFor(() => {
      expect(listRetrievalTestCases).toHaveBeenCalledWith('kb_9');
    });
    expect(await screen.findByText('Retrieval test set')).toBeInTheDocument();
    expect(screen.getByText('Saved cases: 2')).toBeInTheDocument();
    expect(screen.getByText('deployment rollback')).toBeInTheDocument();
    expect(screen.getByText('Expected: Overview · chunk 3 · kdc_1')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Retrieval mode'), { target: { value: 'hybrid' } });
    fireEvent.change(screen.getByLabelText('Retrieval limit'), { target: { value: '3' } });
    fireEvent.click(screen.getByRole('button', { name: 'Run retrieval tests' }));

    await waitFor(() => {
      expect(runRetrievalTestCases).toHaveBeenCalledWith('kb_9', {
        limit: 3,
        mode: 'hybrid'
      });
    });
    expect(screen.getByText('Evaluation: 1 passed / 1 failed / 2 total')).toBeInTheDocument();
    expect(screen.getByText('krtc_1 passed at rank 1')).toBeInTheDocument();
    expect(screen.getByText('krtc_2 failed: expected retrieval result was not returned')).toBeInTheDocument();
    expect(screen.getByText('Actual top: Overview · chunk 3 · kdc_1')).toBeInTheDocument();
  });

  it('runs curated retrieval benchmarks across configured RAG modes', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    listRetrievalTestCases.mockResolvedValue([
      {
        expectedChunkId: 'kdc_1',
        expectedChunkIndex: 2,
        expectedDocumentId: 'doc_1',
        expectedDocumentTitle: 'Overview',
        id: 'krtc_1',
        knowledgeBaseId: 'kb_9',
        query: 'deployment rollback'
      }
    ]);
    runRetrievalTestCases.mockResolvedValue({
      benchmarks: [
        {
          averageRank: 2,
          failed: 1,
          mode: 'vector_only',
          passRate: 0.5,
          passed: 1,
          results: [],
          total: 2
        },
        {
          averageRank: 1,
          failed: 0,
          mode: 'hybrid',
          passRate: 1,
          passed: 2,
          results: [],
          total: 2
        },
        {
          averageRank: 1.5,
          failed: 0,
          mode: 'hybrid_rerank',
          passRate: 1,
          passed: 2,
          results: [],
          total: 2
        }
      ],
      failed: 1,
      knowledgeBaseId: 'kb_9',
      passed: 1,
      results: [],
      total: 2
    });

    render(<KnowledgePage />);

    await screen.findByText('Saved cases: 1');
    fireEvent.click(screen.getByRole('button', { name: 'Run RAG mode benchmark' }));

    await waitFor(() => {
      expect(runRetrievalTestCases).toHaveBeenCalledWith('kb_9', {
        benchmarkModes: ['vector_only', 'hybrid', 'hybrid_rerank']
      });
    });
    expect(screen.getByText('RAG mode benchmark')).toBeInTheDocument();
    expect(screen.getByText('vector_only: 1/2 passed · 50% · average rank 2')).toBeInTheDocument();
    expect(screen.getByText('hybrid: 2/2 passed · 100% · average rank 1')).toBeInTheDocument();
    expect(screen.getByText('hybrid_rerank: 2/2 passed · 100% · average rank 1.5')).toBeInTheDocument();
  });

  it('runs retrieval tests with knowledge-base default tuning when controls are untouched', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    listRetrievalTestCases.mockResolvedValue([
      {
        expectedChunkId: 'kdc_1',
        expectedChunkIndex: 2,
        expectedDocumentId: 'doc_1',
        expectedDocumentTitle: 'Overview',
        id: 'krtc_1',
        knowledgeBaseId: 'kb_9',
        query: 'deployment rollback'
      }
    ]);
    runRetrievalTestCases.mockResolvedValue({
      failed: 0,
      knowledgeBaseId: 'kb_9',
      passed: 1,
      ranAt: '2026-06-05T12:00:00Z',
      results: [],
      total: 1
    });

    render(<KnowledgePage />);

    await screen.findByText('Saved cases: 1');
    fireEvent.click(screen.getByRole('button', { name: 'Run retrieval tests' }));

    await waitFor(() => {
      expect(runRetrievalTestCases).toHaveBeenCalledWith('kb_9', {});
    });
  });

  it('retrieves a selected document version or all versions', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        documentVersion: 'v2',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_1',
        chunkIndex: 0,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        documentVersion: 'v2',
        retrievalMethod: 'hybrid',
        similarity: 0.91,
        source: {
          chunkId: 'kdc_1',
          chunkIndex: 0,
          documentId: 'doc_1',
          documentTitle: 'Overview',
          documentVersion: 'v2'
        },
        snippet: 'Versioned deployment boundaries.'
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.change(screen.getByLabelText('Retrieval document version'), { target: { value: 'v2' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    await waitFor(() => {
      expect(retrieveKnowledge).toHaveBeenLastCalledWith('kb_9', {
        documentVersion: 'v2',
        query: 'deployment'
      });
    });
    expect(screen.getAllByText('Version: v2').length).toBeGreaterThan(0);

    fireEvent.click(screen.getByLabelText('Search all document versions'));
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    await waitFor(() => {
      expect(retrieveKnowledge).toHaveBeenLastCalledWith('kb_9', {
        allVersions: true,
        query: 'deployment'
      });
    });
  });

  it('loads document chunks and renders the selected chunk details', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 24,
        chunkId: 'kdc_1',
        chunkIndex: 0,
        content: 'First architecture chunk.',
        documentVersion: 'v2',
        estimatedTokenCount: 6,
        metadata: {
          documentVersion: 'v2',
          endRune: 44,
          pageNumber: 12,
          sourceUrl: 'https://docs.example/runbook.pdf',
          startRune: 0
        }
      },
      {
        charCount: 36,
        chunkId: 'kdc_2',
        chunkIndex: 1,
        content: 'Second architecture chunk with details.',
        documentVersion: 'v2',
        estimatedTokenCount: 9,
        metadata: {
          documentVersion: 'v2'
        }
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.click(screen.getByRole('button', { name: 'View chunks for Overview' }));

    await waitFor(() => {
      expect(listKnowledgeDocumentChunks).toHaveBeenCalledWith('kb_9', 'doc_1');
    });
    expect(screen.getByRole('heading', { name: 'Chunks for Overview' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Chunk 1 kdc_1 selected' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Chunk 2 kdc_2' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getAllByText('First architecture chunk.').length).toBeGreaterThan(0);
    expect(screen.getByText('chunk_id: kdc_1')).toBeInTheDocument();
    expect(screen.getByText('Characters: 24')).toBeInTheDocument();
    expect(screen.getByText('Estimated tokens: 6')).toBeInTheDocument();
    expect(screen.getByText('documentVersion: v2')).toBeInTheDocument();
    expect(screen.getByText('Page: 12')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open chunk source' })).toHaveAttribute('href', 'https://docs.example/runbook.pdf');

    fireEvent.click(screen.getByRole('button', { name: 'Chunk 2 kdc_2' }));

    expect(screen.getByRole('button', { name: 'Chunk 2 kdc_2 selected' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getAllByText('Second architecture chunk with details.').length).toBeGreaterThan(0);
    expect(screen.getByText('chunk_id: kdc_2')).toBeInTheDocument();
  });

  it('loads document version history and restores a selected version into the editor', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'Current version content.',
        documentVersion: 'v3',
        id: 'doc_1',
        title: 'Runbook',
        updateStrategy: 'versioned',
        updatedAt: '2026-06-07T10:30:00Z'
      }
    ]);
    listKnowledgeDocumentVersions.mockResolvedValue([
      {
        chunkCount: 2,
        content: 'Current version content.',
        documentId: 'doc_1',
        documentVersion: 'v3',
        knowledgeBaseId: 'kb_9',
        title: 'Runbook',
        updateStrategy: 'versioned',
        updatedAt: '2026-06-07T10:30:00Z'
      },
      {
        chunkCount: 1,
        content: 'Previous version content.',
        documentId: 'doc_1',
        documentVersion: 'v2',
        knowledgeBaseId: 'kb_9',
        title: 'Runbook',
        updateStrategy: 'versioned',
        updatedAt: '2026-06-07T09:30:00Z'
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.click(screen.getByRole('button', { name: 'View version history for Runbook' }));

    await waitFor(() => {
      expect(listKnowledgeDocumentVersions).toHaveBeenCalledWith('kb_9', 'doc_1');
    });
    expect(screen.getByRole('heading', { name: 'Version history for Runbook' })).toBeInTheDocument();
    expect(screen.getByText('Version v3 · chunks 2')).toBeInTheDocument();
    expect(screen.getByText('Version v2 · chunks 1')).toBeInTheDocument();
    expect(screen.getByText('Previous version content.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Restore version v2 for Runbook' }));

    expect(screen.getByLabelText('Document title')).toHaveValue('Runbook');
    expect(screen.getByLabelText('Document content')).toHaveValue('Previous version content.');
    expect(screen.getByLabelText('Document version')).toHaveValue('v2');
    expect(screen.getByLabelText('Update strategy')).toHaveValue('versioned');
  });

  it('renders an original document preview with source paging and chunk boundaries', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'Uploaded runbook extracted text',
        id: 'doc_1',
        title: 'Uploaded Runbook',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 44,
        chunkId: 'kdc_1',
        chunkIndex: 0,
        content: 'First runbook chunk from the original PDF.',
        documentVersion: 'v2',
        estimatedTokenCount: 8,
        metadata: {
          documentVersion: 'v2',
          endRune: 44,
          pageNumber: 12,
          sourceUrl: 'https://docs.example/runbook.pdf',
          startRune: 0
        }
      },
      {
        charCount: 47,
        chunkId: 'kdc_2',
        chunkIndex: 1,
        content: 'Second runbook chunk with escalation steps.',
        documentVersion: 'v2',
        estimatedTokenCount: 9,
        metadata: {
          documentVersion: 'v2',
          endRune: 92,
          pageNumber: 13,
          sourceUrl: 'https://docs.example/runbook.pdf',
          startRune: 45
        }
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.click(screen.getByRole('button', { name: 'View chunks for Uploaded Runbook' }));

    const preview = await screen.findByLabelText('Original document preview');
    expect(preview).toHaveTextContent('Original preview: Uploaded Runbook');
    expect(preview).toHaveTextContent('PDF source');
    expect(preview).toHaveTextContent('Page 12');
    expect(screen.getByRole('link', { name: 'Open PDF page 12' })).toHaveAttribute(
      'href',
      'https://docs.example/runbook.pdf#page=12'
    );
    expect(screen.getByRole('button', { name: 'Preview chunk boundary 1 kdc_1 selected' })).toHaveAttribute(
      'aria-pressed',
      'true'
    );
    expect(screen.getByRole('button', { name: 'Preview chunk boundary 2 kdc_2' })).toHaveAttribute('aria-pressed', 'false');
    const firstOverlayButton = screen.getByRole('button', { name: 'Preview chunk boundary 1 kdc_1 selected' });
    const secondOverlayButton = screen.getByRole('button', { name: 'Preview chunk boundary 2 kdc_2' });
    expect(firstOverlayButton).toHaveTextContent('Range 0-44');
    expect(secondOverlayButton).toHaveTextContent('Range 45-92');
    expect(screen.getByLabelText('Overlay color for kdc_1')).toHaveStyle({ backgroundColor: '#2563eb' });
    expect(screen.getByLabelText('Overlay color for kdc_2')).toHaveStyle({ backgroundColor: '#16a34a' });

    fireEvent.click(screen.getByRole('button', { name: 'Preview chunk boundary 2 kdc_2' }));

    expect(screen.getByRole('button', { name: 'Preview chunk boundary 2 kdc_2 selected' })).toHaveAttribute(
      'aria-pressed',
      'true'
    );
    expect(screen.getByText('chunk_id: kdc_2')).toBeInTheDocument();
    expect(preview).toHaveTextContent('Second runbook chunk with escalation steps.');
  });

  it('opens the matching chunk directly from a retrieval result', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_2',
        chunkIndex: 1,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'hybrid',
        similarity: 0.9,
        source: {
          chunkId: 'kdc_2',
          chunkIndex: 1,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'Second architecture chunk with details.'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 24,
        chunkId: 'kdc_1',
        chunkIndex: 0,
        content: 'First architecture chunk.',
        documentVersion: 'v1',
        estimatedTokenCount: 6,
        metadata: {}
      },
      {
        charCount: 36,
        chunkId: 'kdc_2',
        chunkIndex: 1,
        content: 'Second architecture chunk with details.',
        documentVersion: 'v1',
        estimatedTokenCount: 9,
        metadata: {}
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    expect(await screen.findByText('Second architecture chunk with details.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'View chunk kdc_2' }));

    await waitFor(() => {
      expect(listKnowledgeDocumentChunks).toHaveBeenCalledWith('kb_9', 'doc_1');
    });
    expect(await screen.findByRole('heading', { name: 'Chunks for Overview' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Chunk 2 kdc_2 selected' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByText('chunk_id: kdc_2')).toBeInTheDocument();
  });

  it('previews the selected retrieval chunk with highlighted query text', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_2',
        chunkIndex: 1,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'hybrid',
        similarity: 0.9,
        source: {
          chunkId: 'kdc_2',
          chunkIndex: 1,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'Deployment controls are part of the selected chunk.'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 94,
        chunkId: 'kdc_2',
        chunkIndex: 1,
        content: 'Full source chunk: Deployment controls require staged approval before production rollout.',
        documentVersion: 'v1',
        estimatedTokenCount: 14,
        metadata: {}
      }
    ]);

    const { container } = render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment controls' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));
    await screen.findByText('Deployment controls are part of the selected chunk.');
    fireEvent.click(screen.getByRole('button', { name: 'View chunk kdc_2' }));

    const selectedChunkPanel = await screen.findByLabelText('Selected chunk details');
    expect(screen.getByRole('heading', { name: 'Selected chunk' })).toBeInTheDocument();
    expect(selectedChunkPanel).toHaveTextContent('Full source chunk: Deployment controls require staged approval before production rollout.');
    const highlightedText = selectedChunkPanel.querySelector('mark')?.textContent;
    expect(highlightedText).toBe('Deployment controls');
    expect(container.querySelector('mark')).toBeInTheDocument();
  });

  it('edits a selected document chunk and updates the local preview', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 24,
        chunkId: 'kdc_1',
        chunkIndex: 0,
        content: 'First architecture chunk.',
        documentVersion: 'v2',
        estimatedTokenCount: 6,
        metadata: {
          documentVersion: 'v2'
        }
      }
    ]);
    updateKnowledgeDocumentChunk.mockResolvedValue({
      charCount: 35,
      chunkId: 'kdc_1',
      chunkIndex: 0,
      content: 'Edited architecture chunk preview.',
      documentVersion: 'v2',
      estimatedTokenCount: 8,
      metadata: {
        documentVersion: 'v2'
      }
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.click(screen.getByRole('button', { name: 'View chunks for Overview' }));
    expect(await screen.findByLabelText('Chunk content editor')).toHaveValue('First architecture chunk.');

    fireEvent.change(screen.getByLabelText('Chunk content editor'), {
      target: { value: 'Edited architecture chunk preview.' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save chunk' }));

    await waitFor(() => {
      expect(updateKnowledgeDocumentChunk).toHaveBeenCalledWith('kb_9', 'doc_1', 'kdc_1', {
        content: 'Edited architecture chunk preview.'
      });
    });
    expect(screen.getAllByText('Edited architecture chunk preview.').length).toBeGreaterThan(0);
    expect(screen.getByText('Characters: 35')).toBeInTheDocument();
    expect(screen.getByText('Estimated tokens: 8')).toBeInTheDocument();
  });

  it('splits and merges a selected document chunk from the chunk editor', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    listKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 31,
        chunkId: 'kdc_1',
        chunkIndex: 0,
        content: 'First architecture chunk. Second.',
        documentVersion: 'v2',
        estimatedTokenCount: 7,
        metadata: {
          documentVersion: 'v2'
        }
      }
    ]);
    splitKnowledgeDocumentChunk.mockResolvedValue([
      {
        charCount: 24,
        chunkId: 'kdc_left',
        chunkIndex: 0,
        content: 'First architecture chunk.',
        documentVersion: 'v2',
        estimatedTokenCount: 6,
        metadata: {
          documentVersion: 'v2'
        }
      },
      {
        charCount: 7,
        chunkId: 'kdc_right',
        chunkIndex: 1,
        content: 'Second.',
        documentVersion: 'v2',
        estimatedTokenCount: 2,
        metadata: {
          documentVersion: 'v2'
        }
      }
    ]);
    mergeKnowledgeDocumentChunks.mockResolvedValue([
      {
        charCount: 33,
        chunkId: 'kdc_left',
        chunkIndex: 0,
        content: 'First architecture chunk.\n\nSecond.',
        documentVersion: 'v2',
        estimatedTokenCount: 8,
        metadata: {
          documentVersion: 'v2'
        }
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.click(screen.getByRole('button', { name: 'View chunks for Overview' }));
    expect(await screen.findByLabelText('Chunk split position')).toHaveValue(31);

    fireEvent.change(screen.getByLabelText('Chunk split position'), { target: { value: '24' } });
    fireEvent.click(screen.getByRole('button', { name: 'Split chunk' }));

    await waitFor(() => {
      expect(splitKnowledgeDocumentChunk).toHaveBeenCalledWith('kb_9', 'doc_1', 'kdc_1', { splitAt: 24 });
    });
    expect(screen.getByText('chunk_id: kdc_left')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Preview chunk boundary 2 kdc_right' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Merge with next chunk' }));

    await waitFor(() => {
      expect(mergeKnowledgeDocumentChunks).toHaveBeenCalledWith('kb_9', 'doc_1', 'kdc_left', { direction: 'next' });
    });
    expect(screen.getByLabelText('Selected chunk details')).toHaveTextContent('First architecture chunk. Second.');
    expect(screen.queryByRole('button', { name: 'Preview chunk boundary 2 kdc_right' })).not.toBeInTheDocument();
  });

  it('submits retrieval tuning parameters and renders scored source details', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 2,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_7',
        chunkIndex: 4,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'hybrid',
        similarity: 0.87,
        source: {
          chunkId: 'kdc_7',
          chunkIndex: 4,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'Deployment rollback plans belong in the release runbook.'
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment rollback' } });
    fireEvent.change(screen.getByLabelText('Retrieval limit'), { target: { value: '7' } });
    fireEvent.change(screen.getByLabelText('Similarity threshold'), { target: { value: '0.42' } });
    fireEvent.change(screen.getByLabelText('Retrieval mode'), { target: { value: 'hybrid' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    await waitFor(() => {
      expect(retrieveKnowledge).toHaveBeenCalledWith('kb_9', {
        limit: 7,
        minScore: 0.42,
        mode: 'hybrid',
        query: 'deployment rollback'
      });
    });
    expect(screen.getByText('Score: 87%')).toBeInTheDocument();
    expect(screen.getByText('Method: hybrid')).toBeInTheDocument();
    expect(screen.getByText('Source: Overview · chunk 5 · kdc_7')).toBeInTheDocument();
    expect(screen.getByText('Deployment rollback plans belong in the release runbook.')).toBeInTheDocument();
  });

  it('renders citation page, source URL, original text, and highlight spans', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_trace',
        chunkIndex: 3,
        documentId: 'doc_trace',
        documentTitle: 'User Manual.pdf',
        retrievalMethod: 'hybrid_rerank',
        similarity: 0.91,
        source: {
          chunkId: 'kdc_trace',
          chunkIndex: 3,
          documentId: 'doc_trace',
          documentTitle: 'User Manual.pdf',
          pageNumber: 15,
          sourceUrl: 'https://docs.example/manual.pdf',
          originalText: 'Deployment controls require approval before production rollout.',
          matchedSnippet: 'Deployment controls',
          highlightPositions: [{ start: 0, end: 19 }]
        },
        snippet: 'Deployment controls require approval before production rollout.'
      }
    ]);

    const { container } = render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment controls' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    expect(await screen.findByText('Page: 15')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open source document' })).toHaveAttribute('href', 'https://docs.example/manual.pdf');
    expect(screen.getByText('Original text: Deployment controls require approval before production rollout.')).toBeInTheDocument();
    expect(screen.getByText((_, element) => element?.textContent === 'Matched snippet: Deployment controls')).toBeInTheDocument();
    expect(container.querySelector('mark')?.textContent).toBe('Deployment controls');
    expect(screen.getByText('Highlight: 0-19')).toBeInTheDocument();
  });

  it('shows query-specific empty feedback when retrieval returns no snippets', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([]);
    retrieveKnowledge.mockResolvedValue([]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment rollback' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    expect(await screen.findByText('No matching snippets found for “deployment rollback”.')).toBeInTheDocument();
  });

  it('clears stale retrieval results after saving a document', async () => {
    routeState.knowledgeBaseId = 'kb_9';
    getKnowledgeBase.mockResolvedValue({
      documentCount: 1,
      id: 'kb_9',
      name: 'Architecture Notes',
      updatedAt: '2026-04-03T11:30:00Z'
    });
    listKnowledgeDocuments.mockResolvedValue([
      {
        content: 'System boundaries',
        id: 'doc_1',
        title: 'Overview',
        updatedAt: '2026-04-03T11:45:00Z'
      }
    ]);
    retrieveKnowledge.mockResolvedValue([
      {
        chunkId: 'kdc_1',
        chunkIndex: 0,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'embedding_rag',
        similarity: 0.88,
        source: {
          chunkId: 'kdc_1',
          chunkIndex: 0,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        },
        snippet: 'System boundaries include deployment controls.'
      }
    ]);
    updateKnowledgeDocument.mockResolvedValue({
      content: 'Updated boundaries',
      id: 'doc_1',
      title: 'Overview v2',
      updatedAt: '2026-04-03T12:15:00Z'
    });

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));
    expect(await screen.findByText('System boundaries include deployment controls.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Edit document Overview' }));
    fireEvent.change(screen.getByLabelText('Document title'), { target: { value: 'Overview v2' } });
    fireEvent.change(screen.getByLabelText('Document content'), { target: { value: 'Updated boundaries' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save document' }));

    await waitFor(() => {
      expect(updateKnowledgeDocument).toHaveBeenCalledWith('kb_9', 'doc_1', {
        content: 'Updated boundaries',
        title: 'Overview v2'
      });
    });

    expect(screen.queryByText('System boundaries include deployment controls.')).not.toBeInTheDocument();
  });
});
