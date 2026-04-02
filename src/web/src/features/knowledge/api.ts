import type { HttpClient } from '../../services/http/client';
import type {
  CreateKnowledgeBaseRequest,
  CreateKnowledgeDocumentRequest,
  KnowledgeBaseSummary,
  KnowledgeDocumentSummary,
  UpdateKnowledgeBaseRequest,
  UpdateKnowledgeDocumentRequest
} from '../../types/api';

export interface KnowledgeApi {
  createKnowledgeBase: (payload: CreateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  createKnowledgeDocument: (
    knowledgeBaseId: string,
    payload: CreateKnowledgeDocumentRequest
  ) => Promise<KnowledgeDocumentSummary>;
  deleteKnowledgeBase: (knowledgeBaseId: string) => Promise<void>;
  deleteKnowledgeDocument: (knowledgeBaseId: string, documentId: string) => Promise<void>;
  getKnowledgeBase: (knowledgeBaseId: string) => Promise<KnowledgeBaseSummary>;
  listKnowledgeDocuments: (knowledgeBaseId: string) => Promise<KnowledgeDocumentSummary[]>;
  listKnowledgeBases: () => Promise<KnowledgeBaseSummary[]>;
  updateKnowledgeBase: (knowledgeBaseId: string, payload: UpdateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  updateKnowledgeDocument: (
    knowledgeBaseId: string,
    documentId: string,
    payload: UpdateKnowledgeDocumentRequest
  ) => Promise<KnowledgeDocumentSummary>;
}

export function createKnowledgeApi(client: HttpClient): KnowledgeApi {
  return {
    createKnowledgeBase: (payload) => client.post<KnowledgeBaseSummary>('/api/v1/app/knowledge-bases', payload),
    createKnowledgeDocument: (knowledgeBaseId, payload) =>
      client.post<KnowledgeDocumentSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`, payload),
    deleteKnowledgeBase: (knowledgeBaseId) => client.delete<void>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`),
    deleteKnowledgeDocument: (knowledgeBaseId, documentId) =>
      client.delete<void>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`),
    getKnowledgeBase: (knowledgeBaseId) => client.get<KnowledgeBaseSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`),
    listKnowledgeDocuments: (knowledgeBaseId) =>
      client.get<KnowledgeDocumentSummary[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`),
    listKnowledgeBases: () => client.get<KnowledgeBaseSummary[]>('/api/v1/app/knowledge-bases'),
    updateKnowledgeBase: (knowledgeBaseId, payload) =>
      client.put<KnowledgeBaseSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`, payload),
    updateKnowledgeDocument: (knowledgeBaseId, documentId, payload) =>
      client.put<KnowledgeDocumentSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`, payload)
  };
}
