import {
  createKnowledgeBaseOperationContract,
  createKnowledgeDocumentOperationContract,
  createKnowledgeRetrievalTestCaseOperationContract,
  deleteKnowledgeBaseOperationContract,
  deleteKnowledgeDocumentOperationContract,
  getKnowledgeBaseOperationContract,
  listKnowledgeBasesOperationContract,
  listKnowledgeDocumentChunksOperationContract,
  listKnowledgeDocumentIngestionJobsOperationContract,
  listKnowledgeDocumentsOperationContract,
  listKnowledgeDocumentVersionsOperationContract,
  listKnowledgeRetrievalTestCasesOperationContract,
  mergeKnowledgeDocumentChunksOperationContract,
  retrieveKnowledgeOperationContract,
  runKnowledgeRetrievalTestCasesOperationContract,
  splitKnowledgeDocumentChunkOperationContract,
  updateKnowledgeBaseOperationContract,
  updateKnowledgeDocumentChunkOperationContract,
  updateKnowledgeDocumentOperationContract,
  uploadKnowledgeDocumentOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';

import {
  formDataRequestEncoder,
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  noneResponseDecoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
import type {
  CreateKnowledgeBaseRequest,
  CreateKnowledgeDocumentRequest,
  CreateKnowledgeRetrievalTestCaseRequest,
  KnowledgeBaseSummary,
  KnowledgeDocumentChunk,
  KnowledgeDocumentIngestionJob,
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
  listKnowledgeDocumentIngestionJobs: (knowledgeBaseId: string) => Promise<KnowledgeDocumentIngestionJob[]>;
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
  ) => Promise<KnowledgeDocumentIngestionJob>;
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

function jsonTransport<T>(
  operation: OperationContractMetadataV1,
  status = 200
): OperationTransportContract<T> {
  return {
    operation,
    requestEncoder: operation.request.mediaType === null
      ? noneRequestEncoder(operation)
      : jsonRequestEncoder(operation),
    responseDecoder: jsonEnvelopeDecoder<T>(operation, status)
  };
}

function noContentTransport(operation: OperationContractMetadataV1): OperationTransportContract<void> {
  return {
    operation,
    requestEncoder: noneRequestEncoder(operation),
    responseDecoder: noneResponseDecoder(operation, 204)
  };
}

const createKnowledgeBaseTransport = jsonTransport<KnowledgeBaseSummary>(createKnowledgeBaseOperationContract);
const createKnowledgeDocumentTransport = jsonTransport<KnowledgeDocumentSummary>(createKnowledgeDocumentOperationContract);
const createRetrievalTestCaseTransport = jsonTransport<KnowledgeRetrievalTestCase>(
  createKnowledgeRetrievalTestCaseOperationContract,
  201
);
const deleteKnowledgeBaseTransport = noContentTransport(deleteKnowledgeBaseOperationContract);
const deleteKnowledgeDocumentTransport = noContentTransport(deleteKnowledgeDocumentOperationContract);
const getKnowledgeBaseTransport = jsonTransport<KnowledgeBaseSummary>(getKnowledgeBaseOperationContract);
const listKnowledgeBasesTransport = jsonTransport<KnowledgeBaseSummary[]>(listKnowledgeBasesOperationContract);
const listKnowledgeDocumentChunksTransport = jsonTransport<KnowledgeDocumentChunk[]>(
  listKnowledgeDocumentChunksOperationContract
);
const listKnowledgeDocumentIngestionJobsTransport = jsonTransport<KnowledgeDocumentIngestionJob[]>(
  listKnowledgeDocumentIngestionJobsOperationContract
);
const listKnowledgeDocumentsTransport = jsonTransport<KnowledgeDocumentSummary[]>(listKnowledgeDocumentsOperationContract);
const listKnowledgeDocumentVersionsTransport = jsonTransport<KnowledgeDocumentVersion[]>(
  listKnowledgeDocumentVersionsOperationContract
);
const listRetrievalTestCasesTransport = jsonTransport<KnowledgeRetrievalTestCase[]>(
  listKnowledgeRetrievalTestCasesOperationContract
);
const mergeKnowledgeDocumentChunksTransport = jsonTransport<KnowledgeDocumentChunk[]>(
  mergeKnowledgeDocumentChunksOperationContract
);
const retrieveKnowledgeTransport = jsonTransport<KnowledgeRetrievalResult[]>(retrieveKnowledgeOperationContract);
const runRetrievalTestCasesTransport = jsonTransport<KnowledgeRetrievalTestRunReport>(
  runKnowledgeRetrievalTestCasesOperationContract
);
const splitKnowledgeDocumentChunkTransport = jsonTransport<KnowledgeDocumentChunk[]>(
  splitKnowledgeDocumentChunkOperationContract
);
const updateKnowledgeBaseTransport = jsonTransport<KnowledgeBaseSummary>(updateKnowledgeBaseOperationContract);
const updateKnowledgeDocumentChunkTransport = jsonTransport<KnowledgeDocumentChunk>(
  updateKnowledgeDocumentChunkOperationContract
);
const updateKnowledgeDocumentTransport = jsonTransport<KnowledgeDocumentSummary>(updateKnowledgeDocumentOperationContract);
const uploadKnowledgeDocumentTransport: OperationTransportContract<KnowledgeDocumentIngestionJob> = {
  operation: uploadKnowledgeDocumentOperationContract,
  requestEncoder: formDataRequestEncoder(uploadKnowledgeDocumentOperationContract),
  responseDecoder: jsonEnvelopeDecoder<KnowledgeDocumentIngestionJob>(uploadKnowledgeDocumentOperationContract, 202)
};

export function createKnowledgeApi(client: HttpClient): KnowledgeApi {
  return {
    createKnowledgeBase: (payload) => client.post<KnowledgeBaseSummary>(
      '/api/v1/app/knowledge-bases',
      payload,
      undefined,
      createKnowledgeBaseTransport
    ),
    createKnowledgeDocument: (knowledgeBaseId, payload) =>
      client.post<KnowledgeDocumentSummary>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`,
        payload,
        undefined,
        createKnowledgeDocumentTransport
      ),
    createRetrievalTestCase: (knowledgeBaseId, payload) =>
      client.post<KnowledgeRetrievalTestCase>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieval-test-cases`,
        payload,
        undefined,
        createRetrievalTestCaseTransport
      ),
    deleteKnowledgeBase: (knowledgeBaseId) => client.delete<void>(
      `/api/v1/app/knowledge-bases/${knowledgeBaseId}`,
      undefined,
      deleteKnowledgeBaseTransport
    ),
    deleteKnowledgeDocument: (knowledgeBaseId, documentId) =>
      client.delete<void>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`,
        undefined,
        deleteKnowledgeDocumentTransport
      ),
    getKnowledgeBase: (knowledgeBaseId) => client.get<KnowledgeBaseSummary>(
      `/api/v1/app/knowledge-bases/${knowledgeBaseId}`,
      undefined,
      getKnowledgeBaseTransport
    ),
    listKnowledgeDocumentChunks: (knowledgeBaseId, documentId) =>
      client.get<KnowledgeDocumentChunk[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks`,
        undefined,
        listKnowledgeDocumentChunksTransport
      ),
    listKnowledgeDocumentIngestionJobs: (knowledgeBaseId) =>
      client.get<KnowledgeDocumentIngestionJob[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/ingestion-jobs`,
        undefined,
        listKnowledgeDocumentIngestionJobsTransport
      ),
    listKnowledgeDocumentVersions: (knowledgeBaseId, documentId) =>
      client.get<KnowledgeDocumentVersion[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/versions`,
        undefined,
        listKnowledgeDocumentVersionsTransport
      ),
    listKnowledgeDocuments: (knowledgeBaseId) =>
      client.get<KnowledgeDocumentSummary[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents`,
        undefined,
        listKnowledgeDocumentsTransport
      ),
    listKnowledgeBases: () => client.get<KnowledgeBaseSummary[]>(
      '/api/v1/app/knowledge-bases',
      undefined,
      listKnowledgeBasesTransport
    ),
    listRetrievalTestCases: (knowledgeBaseId) =>
      client.get<KnowledgeRetrievalTestCase[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieval-test-cases`,
        undefined,
        listRetrievalTestCasesTransport
      ),
    retrieveKnowledge: (knowledgeBaseId, payload) =>
      client.post<KnowledgeRetrievalResult[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieve`,
        payload,
        undefined,
        retrieveKnowledgeTransport
      ),
    runRetrievalTestCases: (knowledgeBaseId, payload) =>
      client.post<KnowledgeRetrievalTestRunReport>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/retrieval-test-cases/run`,
        payload,
        undefined,
        runRetrievalTestCasesTransport
      ),
    mergeKnowledgeDocumentChunks: (knowledgeBaseId, documentId, chunkId, payload) =>
      client.post<KnowledgeDocumentChunk[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks/${chunkId}/merge`,
        payload,
        undefined,
        mergeKnowledgeDocumentChunksTransport
      ),
    splitKnowledgeDocumentChunk: (knowledgeBaseId, documentId, chunkId, payload) =>
      client.post<KnowledgeDocumentChunk[]>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks/${chunkId}/split`,
        payload,
        undefined,
        splitKnowledgeDocumentChunkTransport
      ),
    updateKnowledgeBase: (knowledgeBaseId, payload) =>
      client.put<KnowledgeBaseSummary>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}`,
        payload,
        undefined,
        updateKnowledgeBaseTransport
      ),
    updateKnowledgeDocument: (knowledgeBaseId, documentId, payload) =>
      client.put<KnowledgeDocumentSummary>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}`,
        payload,
        undefined,
        updateKnowledgeDocumentTransport
      ),
    updateKnowledgeDocumentChunk: (knowledgeBaseId, documentId, chunkId, payload) =>
      client.put<KnowledgeDocumentChunk>(
        `/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/${documentId}/chunks/${chunkId}`,
        payload,
        undefined,
        updateKnowledgeDocumentChunkTransport
      ),
    uploadKnowledgeDocument: (knowledgeBaseId, payload) =>
      client.request<KnowledgeDocumentIngestionJob>(`/api/v1/app/knowledge-bases/${knowledgeBaseId}/documents/upload`, {
        body: buildUploadKnowledgeDocumentFormData(payload),
        method: 'POST'
      }, uploadKnowledgeDocumentTransport)
  };
}
