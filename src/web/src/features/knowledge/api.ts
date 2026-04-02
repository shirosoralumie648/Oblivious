import type { HttpClient } from '../../services/http/client';
import type {
  CreateKnowledgeBaseRequest,
  CreateKnowledgeDocumentRequest,
  KnowledgeBaseSummary,
  KnowledgeDocumentSummary
} from '../../types/api';

export interface KnowledgeApi {
  createKnowledgeBase: (payload: CreateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  createKnowledgeDocument: (
    knowledgeBaseId: string,
    payload: CreateKnowledgeDocumentRequest
  ) => Promise<KnowledgeDocumentSummary>;
  getKnowledgeBase: (knowledgeBaseId: string) => Promise<KnowledgeBaseSummary>;
  listKnowledgeDocuments: (knowledgeBaseId: string) => Promise<KnowledgeDocumentSummary[]>;
  listKnowledgeBases: () => Promise<KnowledgeBaseSummary[]>;
}

export function createKnowledgeApi(client: HttpClient): KnowledgeApi {
  return {
    createKnowledgeBase: (payload) => client.post<KnowledgeBaseSummary>('/api/v1/app/knowledge-bases', payload),
    createKnowledgeDocument: (knowledgeBaseId, payload) =>
      client.post<KnowledgeDocumentSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`, payload),
    getKnowledgeBase: (knowledgeBaseId) => client.get<KnowledgeBaseSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`),
    listKnowledgeDocuments: (knowledgeBaseId) =>
      client.get<KnowledgeDocumentSummary[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`),
    listKnowledgeBases: () => client.get<KnowledgeBaseSummary[]>('/api/v1/app/knowledge-bases')
  };
}
