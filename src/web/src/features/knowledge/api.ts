import type { HttpClient } from '../../services/http/client';
import type { CreateKnowledgeBaseRequest, KnowledgeBaseSummary } from '../../types/api';

export interface KnowledgeApi {
  createKnowledgeBase: (payload: CreateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  listKnowledgeBases: () => Promise<KnowledgeBaseSummary[]>;
}

export function createKnowledgeApi(client: HttpClient): KnowledgeApi {
  return {
    createKnowledgeBase: (payload) => client.post<KnowledgeBaseSummary>('/api/v1/app/knowledge-bases', payload),
    listKnowledgeBases: () => client.get<KnowledgeBaseSummary[]>('/api/v1/app/knowledge-bases')
  };
}
