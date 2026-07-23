import { describe, it, expect, beforeEach } from 'vitest';
import { useKnowledgeStore } from './knowledge';

describe('useKnowledgeStore', () => {
  beforeEach(() => {
    useKnowledgeStore.setState({
      knowledgeBases: [],
      currentKB: null,
      documents: [],
      retrievalResults: [],
    });
  });

  it('initializes with empty state', () => {
    const state = useKnowledgeStore.getState();
    expect(state.knowledgeBases).toEqual([]);
    expect(state.currentKB).toBeNull();
    expect(state.documents).toEqual([]);
    expect(state.retrievalResults).toEqual([]);
  });

  it('sets knowledge bases', () => {
    const kbs = [
      { id: '1', name: 'KB1' },
      { id: '2', name: 'KB2' },
    ];
    useKnowledgeStore.getState().setKnowledgeBases(kbs);
    expect(useKnowledgeStore.getState().knowledgeBases).toEqual(kbs);
  });

  it('sets current KB', () => {
    useKnowledgeStore.getState().setCurrentKB('kb-123');
    expect(useKnowledgeStore.getState().currentKB).toBe('kb-123');
  });

  it('sets documents', () => {
    const docs = [
      { id: '1', title: 'Doc1', content: 'Content1' },
      { id: '2', title: 'Doc2', content: 'Content2' },
    ];
    useKnowledgeStore.getState().setDocuments(docs);
    expect(useKnowledgeStore.getState().documents).toEqual(docs);
  });

  it('sets retrieval results', () => {
    const results = [
      { id: '1', score: 0.9, document: { id: '1', title: 'Doc1', content: 'Content1' } },
      { id: '2', score: 0.7, document: { id: '2', title: 'Doc2', content: 'Content2' } },
    ];
    useKnowledgeStore.getState().setRetrievalResults(results);
    expect(useKnowledgeStore.getState().retrievalResults).toEqual(results);
  });
});
