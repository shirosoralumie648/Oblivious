import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const createKnowledgeBase = vi.fn();
const createKnowledgeDocument = vi.fn();
const deleteKnowledgeBase = vi.fn();
const deleteKnowledgeDocument = vi.fn();
const getKnowledgeBase = vi.fn();
const listKnowledgeDocuments = vi.fn();
const listKnowledgeBases = vi.fn();
const navigate = vi.fn();
const retrieveKnowledge = vi.fn();
const updateKnowledgeBase = vi.fn();
const updateKnowledgeDocument = vi.fn();
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
    deleteKnowledgeBase,
    deleteKnowledgeDocument,
    getKnowledgeBase,
    listKnowledgeDocuments,
    listKnowledgeBases,
    retrieveKnowledge,
    updateKnowledgeBase,
    updateKnowledgeDocument
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
    deleteKnowledgeBase.mockReset();
    deleteKnowledgeDocument.mockReset();
    getKnowledgeBase.mockReset();
    listKnowledgeDocuments.mockReset();
    listKnowledgeBases.mockReset();
    navigate.mockReset();
    routeState.knowledgeBaseId = undefined;
    retrieveKnowledge.mockReset();
    updateKnowledgeBase.mockReset();
    updateKnowledgeDocument.mockReset();
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
    fireEvent.click(screen.getByRole('button', { name: 'Create document' }));

    await waitFor(() => {
      expect(createKnowledgeDocument).toHaveBeenCalledWith('kb_9', {
        content: 'Initial architecture draft',
        title: 'Draft'
      });
    });
    expect(screen.getByText('Draft')).toBeInTheDocument();
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
        documentId: 'doc_1',
        documentTitle: 'Overview',
        snippet: 'System boundaries include deployment controls.'
      }
    ]);

    render(<KnowledgePage />);

    await screen.findByRole('heading', { name: 'Architecture Notes' });
    fireEvent.change(screen.getByLabelText('Retrieval query'), { target: { value: 'deployment' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search knowledge' }));

    await waitFor(() => {
      expect(retrieveKnowledge).toHaveBeenCalledWith('kb_9', { query: 'deployment' });
    });
    expect(screen.getByText('System boundaries include deployment controls.')).toBeInTheDocument();
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
        documentId: 'doc_1',
        documentTitle: 'Overview',
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
