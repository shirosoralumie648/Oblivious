import { create } from 'zustand';

interface KnowledgeBase {
  id: string;
  name: string;
}

interface Document {
  id: string;
  title: string;
  content: string;
}

interface RetrievalResult {
  id: string;
  score: number;
  document: Document;
}

interface KnowledgeState {
  knowledgeBases: KnowledgeBase[];
  currentKB: string | null;
  documents: Document[];
  retrievalResults: RetrievalResult[];
  setKnowledgeBases: (kbs: KnowledgeBase[]) => void;
  setCurrentKB: (id: string | null) => void;
  setDocuments: (docs: Document[]) => void;
  setRetrievalResults: (results: RetrievalResult[]) => void;
}

export const useKnowledgeStore = create<KnowledgeState>((set) => ({
  knowledgeBases: [],
  currentKB: null,
  documents: [],
  retrievalResults: [],
  setKnowledgeBases: (kbs) => set({ knowledgeBases: kbs }),
  setCurrentKB: (id) => set({ currentKB: id }),
  setDocuments: (docs) => set({ documents: docs }),
  setRetrievalResults: (results) => set({ retrievalResults: results }),
}));
