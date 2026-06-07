import type { HttpClient } from '../../services/http/client';
import type {
  CreateKnowledgeBaseRequest,
  CreateKnowledgeDocumentRequest,
  CreateKnowledgeRetrievalTestCaseRequest,
  KnowledgeBaseSummary,
  KnowledgeDocumentChunk,
  KnowledgeDocumentVersion,
  KnowledgeDocumentSummary,
  KnowledgeRetrievalResult,
  KnowledgeRetrievalTestCase,
  KnowledgeRetrievalTestRunReport,
  KnowledgeRetrievalTestRunRequest,
  MergeKnowledgeDocumentChunksRequest,
  RetrieveKnowledgeRequest,
  SplitKnowledgeDocumentChunkRequest,
  UpdateKnowledgeBaseRequest,
  UpdateKnowledgeDocumentRequest,
  UploadKnowledgeDocumentRequest
} from '../../types/api';

export interface KnowledgeApi {
  createKnowledgeBase: (payload: CreateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  createKnowledgeDocument: (
    knowledgeBaseId: string,
    payload: CreateKnowledgeDocumentRequest
  ) => Promise<KnowledgeDocumentSummary>;
  createRetrievalTestCase: (
    knowledgeBaseId: string,
    payload: CreateKnowledgeRetrievalTestCaseRequest
  ) => Promise<KnowledgeRetrievalTestCase>;
  deleteKnowledgeBase: (knowledgeBaseId: string) => Promise<void>;
  deleteKnowledgeDocument: (knowledgeBaseId: string, documentId: string) => Promise<void>;
  getKnowledgeBase: (knowledgeBaseId: string) => Promise<KnowledgeBaseSummary>;
  listKnowledgeDocumentChunks: (knowledgeBaseId: string, documentId: string) => Promise<KnowledgeDocumentChunk[]>;
  listKnowledgeDocumentVersions: (knowledgeBaseId: string, documentId: string) => Promise<KnowledgeDocumentVersion[]>;
  listKnowledgeDocuments: (knowledgeBaseId: string) => Promise<KnowledgeDocumentSummary[]>;
  listKnowledgeBases: () => Promise<KnowledgeBaseSummary[]>;
  listRetrievalTestCases: (knowledgeBaseId: string) => Promise<KnowledgeRetrievalTestCase[]>;
  retrieveKnowledge: (knowledgeBaseId: string, payload: RetrieveKnowledgeRequest) => Promise<KnowledgeRetrievalResult[]>;
  runRetrievalTestCases: (
    knowledgeBaseId: string,
    payload: KnowledgeRetrievalTestRunRequest
  ) => Promise<KnowledgeRetrievalTestRunReport>;
  mergeKnowledgeDocumentChunks: (
    knowledgeBaseId: string,
    documentId: string,
    chunkId: string,
    payload: MergeKnowledgeDocumentChunksRequest
  ) => Promise<KnowledgeDocumentChunk[]>;
  splitKnowledgeDocumentChunk: (
    knowledgeBaseId: string,
    documentId: string,
    chunkId: string,
    payload: SplitKnowledgeDocumentChunkRequest
  ) => Promise<KnowledgeDocumentChunk[]>;
  updateKnowledgeBase: (knowledgeBaseId: string, payload: UpdateKnowledgeBaseRequest) => Promise<KnowledgeBaseSummary>;
  updateKnowledgeDocument: (
    knowledgeBaseId: string,
    documentId: string,
    payload: UpdateKnowledgeDocumentRequest
  ) => Promise<KnowledgeDocumentSummary>;
  updateKnowledgeDocumentChunk: (
    knowledgeBaseId: string,
    documentId: string,
    chunkId: string,
    payload: { content: string }
  ) => Promise<KnowledgeDocumentChunk>;
  uploadKnowledgeDocument: (
    knowledgeBaseId: string,
    payload: UploadKnowledgeDocumentRequest
  ) => Promise<KnowledgeDocumentSummary>;
}

function buildUploadKnowledgeDocumentFormData(payload: UploadKnowledgeDocumentRequest) {
  const formData = new FormData();
  formData.append('file', payload.file);

  const trimmedTitle = payload.title?.trim();
  if (trimmedTitle) {
    formData.append('title', trimmedTitle);
  }

  const trimmedVersion = payload.documentVersion?.trim();
  if (trimmedVersion) {
    formData.append('documentVersion', trimmedVersion);
  }

  if (payload.pageNumber !== undefined && Number.isFinite(payload.pageNumber) && payload.pageNumber > 0) {
    formData.append('pageNumber', String(Math.trunc(payload.pageNumber)));
  }

  const trimmedSourceUrl = payload.sourceUrl?.trim();
  if (trimmedSourceUrl) {
    formData.append('sourceUrl', trimmedSourceUrl);
  }

  if (payload.updateStrategy) {
    formData.append('updateStrategy', payload.updateStrategy);
  }

  return formData;
}

export function createKnowledgeApi(client: HttpClient): KnowledgeApi {
  return {
    createKnowledgeBase: (payload) => client.post<KnowledgeBaseSummary>('/api/v1/app/knowledge-bases', payload),
    createKnowledgeDocument: (knowledgeBaseId, payload) =>
      client.post<KnowledgeDocumentSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`, payload),
    createRetrievalTestCase: (knowledgeBaseId, payload) =>
      client.post<KnowledgeRetrievalTestCase>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieval-test-cases`, payload),
    deleteKnowledgeBase: (knowledgeBaseId) => client.delete<void>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`),
    deleteKnowledgeDocument: (knowledgeBaseId, documentId) =>
      client.delete<void>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`),
    getKnowledgeBase: (knowledgeBaseId) => client.get<KnowledgeBaseSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`),
    listKnowledgeDocumentChunks: (knowledgeBaseId, documentId) =>
      client.get<KnowledgeDocumentChunk[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks`),
    listKnowledgeDocumentVersions: (knowledgeBaseId, documentId) =>
      client.get<KnowledgeDocumentVersion[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/versions`),
    listKnowledgeDocuments: (knowledgeBaseId) =>
      client.get<KnowledgeDocumentSummary[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`),
    listKnowledgeBases: () => client.get<KnowledgeBaseSummary[]>('/api/v1/app/knowledge-bases'),
    listRetrievalTestCases: (knowledgeBaseId) =>
      client.get<KnowledgeRetrievalTestCase[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieval-test-cases`),
    retrieveKnowledge: (knowledgeBaseId, payload) =>
      client.post<KnowledgeRetrievalResult[]>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieve`, payload),
    runRetrievalTestCases: (knowledgeBaseId, payload) =>
      client.post<KnowledgeRetrievalTestRunReport>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieval-test-cases/run`,
        payload
      ),
    mergeKnowledgeDocumentChunks: (knowledgeBaseId, documentId, chunkId, payload) =>
      client.post<KnowledgeDocumentChunk[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks/${chunkId}/merge`,
        payload
      ),
    splitKnowledgeDocumentChunk: (knowledgeBaseId, documentId, chunkId, payload) =>
      client.post<KnowledgeDocumentChunk[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks/${chunkId}/split`,
        payload
      ),
    updateKnowledgeBase: (knowledgeBaseId, payload) =>
      client.put<KnowledgeBaseSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}`, payload),
    updateKnowledgeDocument: (knowledgeBaseId, documentId, payload) =>
      client.put<KnowledgeDocumentSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`, payload),
    updateKnowledgeDocumentChunk: (knowledgeBaseId, documentId, chunkId, payload) =>
      client.put<KnowledgeDocumentChunk>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks/${chunkId}`,
        payload
      ),
    uploadKnowledgeDocument: (knowledgeBaseId, payload) =>
      client.request<KnowledgeDocumentSummary>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/upload`, {
        body: buildUploadKnowledgeDocumentFormData(payload),
        method: 'POST'
      })
  };
}
