import type { HttpClient } from '../../services/http/client';
import type { CreateKnowledgeBaseRequest, KnowledgeBaseSummary } from '../../types/api';

export interface KnowledgeApi {
  createKnowledgeBase: (payload: CreateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  getKnowledgeBase: (knowledgeBaseId: string) => Promise<KnowledgeBaseSummary>;
  listKnowledgeBases: () => Promise<KnowledgeBaseSummary[]>;
}

export function createKnowledgeApi(client: HttpClient): KnowledgeApi {
  return {
    createKnowledgeBase: (payload) => client.post<KnowledgeBaseSummary>('/api/v1/app/knowledge-bases', payload),
    getKnowledgeBase: (knowledgeBaseId) => client.get<KnowledgeBaseSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`),
    listKnowledgeBases: () => client.get<KnowledgeBaseSummary[]>('/api/v1/app/knowledge-bases')
  };
}
