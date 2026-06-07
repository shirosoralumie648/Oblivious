import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';

import { useAppContext } from '../../app/providers';
import { createKnowledgeApi } from '../../features/knowledge/api';
import { createHttpClient } from '../../services/http/client';
import type {
  CreateKnowledgeDocumentRequest,
  KnowledgeBaseSummary,
  KnowledgeChunkStrategy,
  KnowledgeDocumentChunk,
  KnowledgeDocumentSummary,
  KnowledgeRetrievalResult,
  KnowledgeRetrievalMode,
  KnowledgeRetrievalTestCase,
  KnowledgeRetrievalTestRunReport,
  KnowledgeRetrievalTestRunRequest,
  KnowledgeUpdateStrategy,
  RetrieveKnowledgeRequest,
  UploadKnowledgeDocumentRequest
} from '../../types/api';

const defaultRetrievalLimit = 5;
const defaultRetrievalMinScore = 0;
const defaultRetrievalModeSelection = 'knowledge_base_default';
const defaultDocumentUpdateStrategy: KnowledgeUpdateStrategy = 'full_replace';
const defaultKnowledgeRetrievalMode = 'hybrid';
const defaultChunkStrategy = 'template_based';
const defaultChunkSize = 500;
const defaultChunkOverlap = 50;
const defaultEmbeddingModel = 'text-embedding-3-small';
const defaultRerankerModel = 'bge-reranker-large';
const defaultRerankTopK = 5;
const supportedUploadAccept = '.txt,.text,.md,.markdown,.pdf,.docx,text/plain,text/markdown,text/x-markdown,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document';
const unsupportedUploadFormatMessage = 'Knowledge document uploads currently support .txt, .md, PDF, and DOCX files. Legacy .doc parsing is not available yet.';

type KnowledgeBaseWithRagConfig = KnowledgeBaseSummary & {
  chunkOverlap?: number;
  chunkSize?: number;
  chunkStrategy?: KnowledgeChunkStrategy;
  embeddingModel?: string;
  rerankTopK?: number;
  rerankerModel?: string;
  retrievalMode?: KnowledgeRetrievalMode;
};

type RetrievalModeSelection = typeof defaultRetrievalModeSelection | KnowledgeRetrievalMode;

type RetrievalMetrics = {
  averageSimilarity: number;
  elapsedMs: number;
  hitCount: number;
};

function clampNumber(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) {
    return min;
  }
  return Math.min(Math.max(value, min), max);
}

function parseRetrievalLimit(value: string) {
  return Math.trunc(clampNumber(Number(value), 1, 20));
}

function parseRetrievalMinScore(value: string) {
  return clampNumber(Number(value), 0, 1);
}

function parseRerankTopK(value: string) {
  return Math.trunc(clampNumber(Number(value), 1, 50));
}

function formatRetrievalSimilarity(similarity: number) {
  if (!Number.isFinite(similarity)) {
    return '0%';
  }
  return `${Math.round(similarity * 100)}%`;
}

function buildRetrievalMetrics(results: KnowledgeRetrievalResult[], elapsedMs: number): RetrievalMetrics {
  const validSimilarities = results
    .map((result) => result.similarity)
    .filter((similarity) => Number.isFinite(similarity));
  const averageSimilarity = validSimilarities.length > 0
    ? validSimilarities.reduce((total, similarity) => total + similarity, 0) / validSimilarities.length
    : 0;

  return {
    averageSimilarity,
    elapsedMs: Math.max(Math.round(elapsedMs), 0),
    hitCount: results.length
  };
}

function formatRetrievalMetrics(metrics: RetrievalMetrics) {
  return `Retrieval metrics: ${metrics.hitCount} hits · avg ${formatRetrievalSimilarity(metrics.averageSimilarity)} · ${metrics.elapsedMs} ms`;
}

function formatRetrievalSource(result: KnowledgeRetrievalResult) {
  const source = result.source ?? {
    chunkId: result.chunkId,
    chunkIndex: result.chunkIndex,
    documentTitle: result.documentTitle
  };
  return `Source: ${source.documentTitle} · chunk ${source.chunkIndex + 1} · ${source.chunkId}`;
}

function formatExpectedRetrievalTestCase(testCase: KnowledgeRetrievalTestCase) {
  const title = testCase.expectedDocumentTitle?.trim() || testCase.expectedResult?.documentTitle || testCase.expectedDocumentId;
  return `Expected: ${title} · chunk ${testCase.expectedChunkIndex + 1} · ${testCase.expectedChunkId}`;
}

function formatRunResultSource(result: KnowledgeRetrievalResult) {
  return `${result.documentTitle} · chunk ${result.chunkIndex + 1} · ${result.chunkId}`;
}

function formatDocumentVersion(version: string | undefined) {
  return version && version.trim() !== '' ? version : 'v1';
}

function addSourceMetadataToPayload<T extends { pageNumber?: number; sourceUrl?: string }>(payload: T, sourceUrl: string, pageNumber: string): T {
  const trimmedSourceUrl = sourceUrl.trim();
  const parsedPageNumber = Number.parseInt(pageNumber.trim(), 10);
  if (trimmedSourceUrl !== '') {
    payload.sourceUrl = trimmedSourceUrl;
  }
  if (Number.isFinite(parsedPageNumber) && parsedPageNumber > 0) {
    payload.pageNumber = parsedPageNumber;
  }
  return payload;
}

function buildDocumentPayload(
  title: string,
  content: string,
  documentVersion: string,
  updateStrategy: KnowledgeUpdateStrategy,
  sourceUrl: string,
  pageNumber: string
): CreateKnowledgeDocumentRequest {
  const payload: CreateKnowledgeDocumentRequest = { content, title };
  const trimmedVersion = documentVersion.trim();
  if (trimmedVersion !== '') {
    payload.documentVersion = trimmedVersion;
  }
  if (updateStrategy !== defaultDocumentUpdateStrategy) {
    payload.updateStrategy = updateStrategy;
  }
  return addSourceMetadataToPayload(payload, sourceUrl, pageNumber);
}

function buildUploadDocumentPayload(
  file: File,
  title: string,
  documentVersion: string,
  updateStrategy: KnowledgeUpdateStrategy,
  sourceUrl: string,
  pageNumber: string
): UploadKnowledgeDocumentRequest {
  const payload: UploadKnowledgeDocumentRequest = { file };
  const trimmedTitle = title.trim();
  const trimmedVersion = documentVersion.trim();

  if (trimmedTitle !== '') {
    payload.title = trimmedTitle;
  }
  if (trimmedVersion !== '') {
    payload.documentVersion = trimmedVersion;
  }
  if (updateStrategy !== defaultDocumentUpdateStrategy) {
    payload.updateStrategy = updateStrategy;
  }

  return addSourceMetadataToPayload(payload, sourceUrl, pageNumber);
}

function addExplicitRetrievalTuning(
  payload: KnowledgeRetrievalTestRunRequest,
  retrievalModeSelection: RetrievalModeSelection,
  retrievalLimit: number,
  retrievalMinScore: number
) {
  if (retrievalModeSelection !== defaultRetrievalModeSelection) {
    payload.mode = retrievalModeSelection;
  }
  if (retrievalLimit !== defaultRetrievalLimit) {
    payload.limit = retrievalLimit;
  }
  if (retrievalMinScore !== defaultRetrievalMinScore) {
    payload.minScore = retrievalMinScore;
  }
}

function addRetrievalVersionFilter(
  payload: KnowledgeRetrievalTestRunRequest,
  retrievalAllVersions: boolean,
  retrievalDocumentVersion: string
) {
  const trimmedDocumentVersion = retrievalDocumentVersion.trim();
  if (retrievalAllVersions) {
    payload.allVersions = true;
  } else if (trimmedDocumentVersion !== '') {
    payload.documentVersion = trimmedDocumentVersion;
  }
}

function buildRetrieveKnowledgePayload(
  query: string,
  retrievalModeSelection: RetrievalModeSelection,
  retrievalLimit: number,
  retrievalMinScore: number,
  retrievalAllVersions: boolean,
  retrievalDocumentVersion: string
): RetrieveKnowledgeRequest {
  const payload: RetrieveKnowledgeRequest = { query };
  addExplicitRetrievalTuning(payload, retrievalModeSelection, retrievalLimit, retrievalMinScore);
  addRetrievalVersionFilter(payload, retrievalAllVersions, retrievalDocumentVersion);
  return payload;
}

function buildRetrievalTestRunPayload(
  retrievalModeSelection: RetrievalModeSelection,
  retrievalLimit: number,
  retrievalMinScore: number
): KnowledgeRetrievalTestRunRequest {
  const payload: KnowledgeRetrievalTestRunRequest = {};
  addExplicitRetrievalTuning(payload, retrievalModeSelection, retrievalLimit, retrievalMinScore);
  return payload;
}

function isSupportedUploadDocumentFile(file: File) {
  const name = file.name.trim().toLowerCase();
  const type = file.type.trim().toLowerCase();
  return (
    name.endsWith('.txt') ||
    name.endsWith('.text') ||
    name.endsWith('.md') ||
    name.endsWith('.markdown') ||
    name.endsWith('.pdf') ||
    name.endsWith('.docx') ||
    type === 'text/plain' ||
    type === 'text/markdown' ||
    type === 'text/x-markdown' ||
    type === 'application/pdf' ||
    type === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  );
}

function formatHighlightPosition(position: { start: number; end: number }) {
  return `${position.start}-${position.end}`;
}

function findQueryHighlightRange(content: string, query: string) {
  const trimmedQuery = query.trim();
  if (trimmedQuery === '') {
    return null;
  }

  const phraseIndex = content.toLowerCase().indexOf(trimmedQuery.toLowerCase());
  if (phraseIndex >= 0) {
    return {
      end: phraseIndex + trimmedQuery.length,
      start: phraseIndex
    };
  }

  const firstMatchingTerm = trimmedQuery
    .split(/\s+/)
    .filter((term) => term.length > 0)
    .find((term) => content.toLowerCase().includes(term.toLowerCase()));
  if (!firstMatchingTerm) {
    return null;
  }

  const termIndex = content.toLowerCase().indexOf(firstMatchingTerm.toLowerCase());
  return {
    end: termIndex + firstMatchingTerm.length,
    start: termIndex
  };
}

function renderHighlightedChunkContent(content: string, query: string) {
  const highlightRange = findQueryHighlightRange(content, query);
  if (!highlightRange) {
    return content;
  }

  return (
    <>
      {content.slice(0, highlightRange.start)}
      <mark>{content.slice(highlightRange.start, highlightRange.end)}</mark>
      {content.slice(highlightRange.end)}
    </>
  );
}

function renderMatchedSnippet(matchedSnippet: string) {
  return (
    <>
      Matched snippet: <mark>{matchedSnippet}</mark>
    </>
  );
}

type DocumentPreviewKind = 'docx' | 'pdf' | 'text';

function inferDocumentPreviewKind(title: string, sourceUrl: string | undefined): DocumentPreviewKind {
  const descriptor = `${title} ${sourceUrl ?? ''}`.toLowerCase();
  if (descriptor.includes('.pdf') || descriptor.includes('application/pdf')) {
    return 'pdf';
  }
  if (
    descriptor.includes('.docx') ||
    descriptor.includes('wordprocessingml.document') ||
    descriptor.includes('application/vnd.openxmlformats-officedocument')
  ) {
    return 'docx';
  }
  return 'text';
}

function formatDocumentPreviewKind(kind: DocumentPreviewKind) {
  if (kind === 'pdf') {
    return 'PDF source';
  }
  if (kind === 'docx') {
    return 'Word source';
  }
  return 'Text source';
}

function isPositivePageNumber(pageNumber: number | undefined): pageNumber is number {
  return pageNumber !== undefined && Number.isFinite(pageNumber) && pageNumber > 0;
}

function buildDocumentPreviewHref(sourceUrl: string, pageNumber: number | undefined, kind: DocumentPreviewKind) {
  const trimmedSourceUrl = sourceUrl.trim();
  if (trimmedSourceUrl === '') {
    return '';
  }
  if (kind !== 'pdf' || !isPositivePageNumber(pageNumber)) {
    return trimmedSourceUrl;
  }
  return `${trimmedSourceUrl.replace(/#.*$/, '')}#page=${Math.trunc(pageNumber)}`;
}

function formatDocumentPreviewLinkLabel(kind: DocumentPreviewKind, pageNumber: number | undefined) {
  if (kind === 'pdf' && isPositivePageNumber(pageNumber)) {
    return `Open PDF page ${Math.trunc(pageNumber)}`;
  }
  if (kind === 'docx') {
    return 'Open Word source';
  }
  return 'Open original source';
}

function hasKnowledgeBaseRagConfig(knowledgeBase: KnowledgeBaseWithRagConfig) {
  return (
    knowledgeBase.chunkOverlap !== undefined ||
    knowledgeBase.chunkSize !== undefined ||
    knowledgeBase.chunkStrategy !== undefined ||
    knowledgeBase.embeddingModel !== undefined ||
    knowledgeBase.rerankTopK !== undefined ||
    knowledgeBase.rerankerModel !== undefined ||
    knowledgeBase.retrievalMode !== undefined
  );
}

export function KnowledgePage() {
  const navigate = useNavigate();
  const { knowledgeBaseId } = useParams<{ knowledgeBaseId?: string }>();
  const { authState } = useAppContext();
  const returnTo = new URLSearchParams(window.location.search).get('returnTo');
  const knowledgeApi = useMemo(() => createKnowledgeApi(createHttpClient()), []);
  const [editingDocumentId, setEditingDocumentId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [hasRetrievedKnowledge, setHasRetrievedKnowledge] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isDeletingKnowledgeBase, setIsDeletingKnowledgeBase] = useState(false);
  const [isDeletingDocumentId, setIsDeletingDocumentId] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingDocumentChunks, setIsLoadingDocumentChunks] = useState(false);
  const [isRetrievingKnowledge, setIsRetrievingKnowledge] = useState(false);
  const [isEditingChunkStructure, setIsEditingChunkStructure] = useState(false);
  const [isRunningRetrievalTests, setIsRunningRetrievalTests] = useState(false);
  const [isSavingChunk, setIsSavingChunk] = useState(false);
  const [isSavingRetrievalTestCaseChunkId, setIsSavingRetrievalTestCaseChunkId] = useState<string | null>(null);
  const [isSavingDocument, setIsSavingDocument] = useState(false);
  const [isSavingKnowledgeBase, setIsSavingKnowledgeBase] = useState(false);
  const [isUploadingDocument, setIsUploadingDocument] = useState(false);
  const [knowledgeDocumentContent, setKnowledgeDocumentContent] = useState('');
  const [knowledgeDocumentVersion, setKnowledgeDocumentVersion] = useState('');
  const [knowledgeDocumentUpdateStrategy, setKnowledgeDocumentUpdateStrategy] = useState<KnowledgeUpdateStrategy>(defaultDocumentUpdateStrategy);
  const [knowledgeDocumentSourceUrl, setKnowledgeDocumentSourceUrl] = useState('');
  const [knowledgeDocumentSourcePage, setKnowledgeDocumentSourcePage] = useState('');
  const [knowledgeDocumentTitle, setKnowledgeDocumentTitle] = useState('');
  const [knowledgeBaseName, setKnowledgeBaseName] = useState('');
  const [knowledgeBaseChunkOverlap, setKnowledgeBaseChunkOverlap] = useState(defaultChunkOverlap);
  const [knowledgeBaseChunkSize, setKnowledgeBaseChunkSize] = useState(defaultChunkSize);
  const [knowledgeBaseChunkStrategy, setKnowledgeBaseChunkStrategy] = useState<KnowledgeChunkStrategy>(defaultChunkStrategy);
  const [knowledgeBaseEmbeddingModel, setKnowledgeBaseEmbeddingModel] = useState(defaultEmbeddingModel);
  const [knowledgeBaseHasRagConfig, setKnowledgeBaseHasRagConfig] = useState(false);
  const [knowledgeBaseRagConfigDirty, setKnowledgeBaseRagConfigDirty] = useState(false);
  const [knowledgeBaseRerankerModel, setKnowledgeBaseRerankerModel] = useState(defaultRerankerModel);
  const [knowledgeBaseRerankTopK, setKnowledgeBaseRerankTopK] = useState(defaultRerankTopK);
  const [knowledgeBaseRetrievalMode, setKnowledgeBaseRetrievalMode] = useState<KnowledgeRetrievalMode>(defaultKnowledgeRetrievalMode);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseSummary[]>([]);
  const [knowledgeChunkContentDraft, setKnowledgeChunkContentDraft] = useState('');
  const [knowledgeChunkSplitAt, setKnowledgeChunkSplitAt] = useState(0);
  const [knowledgeDocumentChunks, setKnowledgeDocumentChunks] = useState<KnowledgeDocumentChunk[]>([]);
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<KnowledgeDocumentSummary[]>([]);
  const [lastRetrievedQuery, setLastRetrievedQuery] = useState('');
  const [retrievalLimit, setRetrievalLimit] = useState(defaultRetrievalLimit);
  const [retrievalMinScore, setRetrievalMinScore] = useState(defaultRetrievalMinScore);
  const [retrievalModeSelection, setRetrievalModeSelection] = useState<RetrievalModeSelection>(defaultRetrievalModeSelection);
  const [retrievalDocumentVersion, setRetrievalDocumentVersion] = useState('');
  const [retrievalAllVersions, setRetrievalAllVersions] = useState(false);
  const [retrievalMetrics, setRetrievalMetrics] = useState<RetrievalMetrics | null>(null);
  const [retrievalQuery, setRetrievalQuery] = useState('');
  const [retrievalResults, setRetrievalResults] = useState<KnowledgeRetrievalResult[]>([]);
  const [retrievalTestCases, setRetrievalTestCases] = useState<KnowledgeRetrievalTestCase[]>([]);
  const [retrievalTestRunReport, setRetrievalTestRunReport] = useState<KnowledgeRetrievalTestRunReport | null>(null);
  const [savedRetrievalTestCaseId, setSavedRetrievalTestCaseId] = useState<string | null>(null);
  const [selectedKnowledgeBase, setSelectedKnowledgeBase] = useState<KnowledgeBaseSummary | null>(null);
  const [selectedChunkId, setSelectedChunkId] = useState<string | null>(null);
  const [selectedChunkDocument, setSelectedChunkDocument] = useState<KnowledgeDocumentSummary | null>(null);
  const [uploadDocumentFile, setUploadDocumentFile] = useState<File | null>(null);
  const [uploadDocumentFileInputKey, setUploadDocumentFileInputKey] = useState(0);
  const [uploadDocumentTitle, setUploadDocumentTitle] = useState('');
  const [uploadDocumentVersion, setUploadDocumentVersion] = useState('');
  const [uploadDocumentUpdateStrategy, setUploadDocumentUpdateStrategy] = useState<KnowledgeUpdateStrategy>(defaultDocumentUpdateStrategy);
  const [uploadDocumentSourceUrl, setUploadDocumentSourceUrl] = useState('');
  const [uploadDocumentSourcePage, setUploadDocumentSourcePage] = useState('');

  const resetDocumentEditor = () => {
    setEditingDocumentId(null);
    setKnowledgeDocumentTitle('');
    setKnowledgeDocumentContent('');
    setKnowledgeDocumentVersion('');
    setKnowledgeDocumentUpdateStrategy(defaultDocumentUpdateStrategy);
    setKnowledgeDocumentSourceUrl('');
    setKnowledgeDocumentSourcePage('');
  };

  const resetKnowledgeRetrieval = () => {
    setHasRetrievedKnowledge(false);
    setLastRetrievedQuery('');
    setRetrievalLimit(defaultRetrievalLimit);
    setRetrievalMinScore(defaultRetrievalMinScore);
    setRetrievalModeSelection(defaultRetrievalModeSelection);
    setRetrievalDocumentVersion('');
    setRetrievalAllVersions(false);
    setRetrievalMetrics(null);
    setRetrievalQuery('');
    setRetrievalResults([]);
    setRetrievalTestRunReport(null);
    setIsSavingRetrievalTestCaseChunkId(null);
    setSavedRetrievalTestCaseId(null);
  };

  const resetRetrievalTestSet = () => {
    setRetrievalTestCases([]);
    setRetrievalTestRunReport(null);
  };

  const resetKnowledgeBaseRagConfig = () => {
    setKnowledgeBaseChunkOverlap(defaultChunkOverlap);
    setKnowledgeBaseChunkSize(defaultChunkSize);
    setKnowledgeBaseChunkStrategy(defaultChunkStrategy);
    setKnowledgeBaseEmbeddingModel(defaultEmbeddingModel);
    setKnowledgeBaseHasRagConfig(false);
    setKnowledgeBaseRagConfigDirty(false);
    setKnowledgeBaseRerankerModel(defaultRerankerModel);
    setKnowledgeBaseRerankTopK(defaultRerankTopK);
    setKnowledgeBaseRetrievalMode(defaultKnowledgeRetrievalMode);
  };

  const setKnowledgeBaseRagConfig = (knowledgeBase: KnowledgeBaseWithRagConfig) => {
    setKnowledgeBaseChunkOverlap(knowledgeBase.chunkOverlap ?? defaultChunkOverlap);
    setKnowledgeBaseChunkSize(knowledgeBase.chunkSize ?? defaultChunkSize);
    setKnowledgeBaseChunkStrategy(knowledgeBase.chunkStrategy ?? defaultChunkStrategy);
    setKnowledgeBaseEmbeddingModel(knowledgeBase.embeddingModel ?? defaultEmbeddingModel);
    setKnowledgeBaseHasRagConfig(hasKnowledgeBaseRagConfig(knowledgeBase));
    setKnowledgeBaseRagConfigDirty(false);
    setKnowledgeBaseRerankerModel(knowledgeBase.rerankerModel?.trim() || defaultRerankerModel);
    setKnowledgeBaseRerankTopK(knowledgeBase.rerankTopK ?? defaultRerankTopK);
    setKnowledgeBaseRetrievalMode(knowledgeBase.retrievalMode ?? defaultKnowledgeRetrievalMode);
  };

  const resetDocumentChunks = () => {
    setIsLoadingDocumentChunks(false);
    setIsEditingChunkStructure(false);
    setIsSavingChunk(false);
    setKnowledgeChunkContentDraft('');
    setKnowledgeChunkSplitAt(0);
    setKnowledgeDocumentChunks([]);
    setSelectedChunkDocument(null);
    setSelectedChunkId(null);
  };

  const resetUploadDocumentForm = () => {
    setUploadDocumentFile(null);
    setUploadDocumentFileInputKey((current) => current + 1);
    setUploadDocumentTitle('');
    setUploadDocumentVersion('');
    setUploadDocumentUpdateStrategy(defaultDocumentUpdateStrategy);
    setUploadDocumentSourceUrl('');
    setUploadDocumentSourcePage('');
  };

  const handleUploadDocumentFileChange = (file: File | null) => {
    if (!file) {
      setUploadDocumentFile(null);
      return;
    }
    if (!isSupportedUploadDocumentFile(file)) {
      setUploadDocumentFile(null);
      setUploadDocumentFileInputKey((current) => current + 1);
      setError(unsupportedUploadFormatMessage);
      return;
    }
    setError(null);
    setUploadDocumentFile(file);
  };

  const selectedChunk = knowledgeDocumentChunks.find((chunk) => chunk.chunkId === selectedChunkId) ?? knowledgeDocumentChunks[0] ?? null;
  const selectedChunkListIndex = selectedChunk
    ? knowledgeDocumentChunks.findIndex((chunk) => chunk.chunkId === selectedChunk.chunkId)
    : -1;
  const chunkPreviewSourceUrl =
    selectedChunk?.metadata.sourceUrl ?? knowledgeDocumentChunks.find((chunk) => chunk.metadata.sourceUrl)?.metadata.sourceUrl;
  const chunkPreviewPageNumber =
    selectedChunk?.metadata.pageNumber ?? knowledgeDocumentChunks.find((chunk) => chunk.metadata.pageNumber)?.metadata.pageNumber;
  const chunkPreviewKind = inferDocumentPreviewKind(selectedChunkDocument?.title ?? '', chunkPreviewSourceUrl);
  const chunkPreviewHref = chunkPreviewSourceUrl
    ? buildDocumentPreviewHref(chunkPreviewSourceUrl, chunkPreviewPageNumber, chunkPreviewKind)
    : '';

  useEffect(() => {
    setKnowledgeChunkContentDraft(selectedChunk?.content ?? '');
    setKnowledgeChunkSplitAt(selectedChunk?.charCount ?? selectedChunk?.content.length ?? 0);
  }, [selectedChunk?.chunkId, selectedChunk?.content]);

  useEffect(() => {
    let cancelled = false;

    const loadKnowledge = async () => {
      setIsLoading(true);
      setError(null);

      try {
        if (knowledgeBaseId) {
          const [nextKnowledgeBase, nextKnowledgeDocuments, nextRetrievalTestCases] = await Promise.all([
            knowledgeApi.getKnowledgeBase(knowledgeBaseId),
            knowledgeApi.listKnowledgeDocuments(knowledgeBaseId),
            knowledgeApi.listRetrievalTestCases(knowledgeBaseId)
          ]);
          if (!cancelled) {
            setSelectedKnowledgeBase(nextKnowledgeBase);
            setKnowledgeBaseRagConfig(nextKnowledgeBase as KnowledgeBaseWithRagConfig);
            setKnowledgeBaseName(nextKnowledgeBase.name);
            setKnowledgeDocuments(nextKnowledgeDocuments);
            setRetrievalTestCases(nextRetrievalTestCases);
            setKnowledgeBases([]);
            resetDocumentEditor();
            resetDocumentChunks();
            resetKnowledgeRetrieval();
          }
        } else {
          const nextKnowledgeBases = await knowledgeApi.listKnowledgeBases();
          if (!cancelled) {
            setKnowledgeBases(nextKnowledgeBases);
            setKnowledgeDocuments([]);
            setSelectedKnowledgeBase(null);
            setKnowledgeBaseName('');
            resetRetrievalTestSet();
            resetKnowledgeBaseRagConfig();
            resetDocumentEditor();
            resetDocumentChunks();
            resetKnowledgeRetrieval();
          }
        }
      } catch {
        if (!cancelled) {
          setKnowledgeBases([]);
          setKnowledgeDocuments([]);
          setSelectedKnowledgeBase(null);
          setKnowledgeBaseName('');
          resetRetrievalTestSet();
          resetKnowledgeBaseRagConfig();
          resetDocumentEditor();
          resetDocumentChunks();
          resetKnowledgeRetrieval();
          setError('Unable to load workspace data. Retry the request or check the backend session.');
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    };

    void loadKnowledge();

    return () => {
      cancelled = true;
    };
  }, [knowledgeApi, knowledgeBaseId]);

  const handleCreateKnowledgeBase = async () => {
    const trimmedName = knowledgeBaseName.trim();
    if (trimmedName === '') {
      return;
    }

    setIsCreating(true);
    setError(null);

    try {
      const createdKnowledgeBase = await knowledgeApi.createKnowledgeBase({ name: trimmedName });
      setKnowledgeBases((current) => [createdKnowledgeBase, ...current]);
      setKnowledgeBaseName('');
    } catch {
      setError('Unable to create knowledge base. Retry the request or check the backend session.');
    } finally {
      setIsCreating(false);
    }
  };

  const handleSaveKnowledgeBase = async () => {
    if (!knowledgeBaseId || !selectedKnowledgeBase) {
      return;
    }

    const trimmedName = knowledgeBaseName.trim();
    if (trimmedName === '') {
      return;
    }

    setIsSavingKnowledgeBase(true);
    setError(null);

    try {
      const knowledgeBasePayload = knowledgeBaseHasRagConfig || knowledgeBaseRagConfigDirty
        ? {
            chunkOverlap: knowledgeBaseChunkOverlap,
            chunkSize: knowledgeBaseChunkSize,
            chunkStrategy: knowledgeBaseChunkStrategy,
            embeddingModel: knowledgeBaseEmbeddingModel.trim(),
            name: trimmedName,
            rerankTopK: knowledgeBaseRerankTopK,
            rerankerModel: knowledgeBaseRerankerModel.trim(),
            retrievalMode: knowledgeBaseRetrievalMode
          }
        : { name: trimmedName };
      const updatedKnowledgeBase = await knowledgeApi.updateKnowledgeBase(knowledgeBaseId, knowledgeBasePayload);
      setSelectedKnowledgeBase(updatedKnowledgeBase);
      setKnowledgeBaseRagConfig(updatedKnowledgeBase as KnowledgeBaseWithRagConfig);
      setKnowledgeBaseName(updatedKnowledgeBase.name);
      setKnowledgeBases((current) =>
        current.map((knowledgeBase) => (knowledgeBase.id === updatedKnowledgeBase.id ? updatedKnowledgeBase : knowledgeBase))
      );
    } catch {
      setError('Unable to update knowledge base. Retry the request or check the backend session.');
    } finally {
      setIsSavingKnowledgeBase(false);
    }
  };

  const handleDeleteKnowledgeBase = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    setIsDeletingKnowledgeBase(true);
    setError(null);

    try {
      await knowledgeApi.deleteKnowledgeBase(knowledgeBaseId);
      navigate('/knowledge');
    } catch {
      setError('Unable to delete knowledge base. Retry the request or check the backend session.');
    } finally {
      setIsDeletingKnowledgeBase(false);
    }
  };

  const handleRetrieveKnowledge = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    const trimmedQuery = retrievalQuery.trim();
    if (trimmedQuery === '') {
      return;
    }

    setIsRetrievingKnowledge(true);
    setError(null);

    try {
      const retrievalPayload = buildRetrieveKnowledgePayload(
        trimmedQuery,
        retrievalModeSelection,
        retrievalLimit,
        retrievalMinScore,
        retrievalAllVersions,
        retrievalDocumentVersion
      );
      const startedAt = Date.now();
      const nextResults = await knowledgeApi.retrieveKnowledge(knowledgeBaseId, retrievalPayload);
      setRetrievalMetrics(buildRetrievalMetrics(nextResults, Date.now() - startedAt));
      setLastRetrievedQuery(trimmedQuery);
      setRetrievalResults(nextResults);
      setHasRetrievedKnowledge(true);
      setSavedRetrievalTestCaseId(null);
    } catch {
      setError('Unable to retrieve knowledge. Retry the request or check the backend session.');
    } finally {
      setIsRetrievingKnowledge(false);
    }
  };

  const handleCreateRetrievalTestCase = async (result: KnowledgeRetrievalResult) => {
    if (!knowledgeBaseId) {
      return;
    }

    const query = (lastRetrievedQuery || retrievalQuery).trim();
    if (query === '') {
      return;
    }

    setIsSavingRetrievalTestCaseChunkId(result.chunkId);
    setSavedRetrievalTestCaseId(null);
    setError(null);

    try {
      const testCase = await knowledgeApi.createRetrievalTestCase(knowledgeBaseId, {
        expectedResult: result,
        query
      });
      setRetrievalTestCases((current) => [testCase, ...current.filter((currentTestCase) => currentTestCase.id !== testCase.id)]);
      setSavedRetrievalTestCaseId(testCase.id);
    } catch {
      setError('Unable to save retrieval test case. Retry the request or check the backend session.');
    } finally {
      setIsSavingRetrievalTestCaseChunkId(null);
    }
  };

  const handleRunRetrievalTests = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    setIsRunningRetrievalTests(true);
    setError(null);

    try {
      const runPayload = buildRetrievalTestRunPayload(retrievalModeSelection, retrievalLimit, retrievalMinScore);
      const report = await knowledgeApi.runRetrievalTestCases(knowledgeBaseId, runPayload);
      setRetrievalTestRunReport(report);
    } catch {
      setError('Unable to run retrieval tests. Retry the request or check the backend session.');
    } finally {
      setIsRunningRetrievalTests(false);
    }
  };

  const handleViewDocumentChunks = async (document: KnowledgeDocumentSummary, preferredChunkId?: string) => {
    if (!knowledgeBaseId) {
      return;
    }

    setIsLoadingDocumentChunks(true);
    setError(null);
    setSelectedChunkDocument(document);
    setSelectedChunkId(null);
    setKnowledgeChunkContentDraft('');

    try {
      const chunks = await knowledgeApi.listKnowledgeDocumentChunks(knowledgeBaseId, document.id);
      setKnowledgeDocumentChunks(chunks);
      setSelectedChunkId(chunks.some((chunk) => chunk.chunkId === preferredChunkId) ? preferredChunkId ?? null : chunks[0]?.chunkId ?? null);
    } catch {
      setKnowledgeDocumentChunks([]);
      setSelectedChunkId(null);
      setError('Unable to load document chunks. Retry the request or check the backend session.');
    } finally {
      setIsLoadingDocumentChunks(false);
    }
  };

  const documentSummaryForRetrievalResult = (result: KnowledgeRetrievalResult): KnowledgeDocumentSummary => {
    const existingDocument = knowledgeDocuments.find((document) => document.id === result.documentId);
    if (existingDocument) {
      return existingDocument;
    }
    return {
      content: result.snippet,
      documentVersion: result.documentVersion,
      id: result.documentId,
      title: result.documentTitle
    };
  };

  const handleSaveKnowledgeDocumentChunk = async () => {
    if (!knowledgeBaseId || !selectedChunkDocument || !selectedChunk) {
      return;
    }

    const trimmedContent = knowledgeChunkContentDraft.trim();
    if (trimmedContent === '') {
      return;
    }

    setIsSavingChunk(true);
    setError(null);

    try {
      const updatedChunk = await knowledgeApi.updateKnowledgeDocumentChunk(
        knowledgeBaseId,
        selectedChunkDocument.id,
        selectedChunk.chunkId,
        { content: trimmedContent }
      );
      setKnowledgeDocumentChunks((current) =>
        current.map((chunk) => (chunk.chunkId === updatedChunk.chunkId ? updatedChunk : chunk))
      );
      setSelectedChunkId(updatedChunk.chunkId);
      setKnowledgeChunkContentDraft(updatedChunk.content);
    } catch {
      setError('Unable to update document chunk. Retry the request or check the backend session.');
    } finally {
      setIsSavingChunk(false);
    }
  };

  const applyChunkListUpdate = (chunks: KnowledgeDocumentChunk[], preferredChunkId?: string) => {
    setKnowledgeDocumentChunks(chunks);
    const nextSelectedChunkId = chunks.some((chunk) => chunk.chunkId === preferredChunkId)
      ? preferredChunkId ?? null
      : chunks[0]?.chunkId ?? null;
    setSelectedChunkId(nextSelectedChunkId);
  };

  const handleSplitKnowledgeDocumentChunk = async () => {
    if (!knowledgeBaseId || !selectedChunkDocument || !selectedChunk) {
      return;
    }
    const splitAt = Math.trunc(knowledgeChunkSplitAt);
    if (!Number.isFinite(splitAt) || splitAt <= 0 || splitAt >= selectedChunk.charCount) {
      return;
    }

    setIsEditingChunkStructure(true);
    setError(null);

    try {
      const chunks = await knowledgeApi.splitKnowledgeDocumentChunk(
        knowledgeBaseId,
        selectedChunkDocument.id,
        selectedChunk.chunkId,
        { splitAt }
      );
      applyChunkListUpdate(chunks, selectedChunk.chunkId);
    } catch {
      setError('Unable to split document chunk. Retry the request or check the backend session.');
    } finally {
      setIsEditingChunkStructure(false);
    }
  };

  const handleMergeKnowledgeDocumentChunks = async (direction: 'previous' | 'next') => {
    if (!knowledgeBaseId || !selectedChunkDocument || !selectedChunk) {
      return;
    }

    setIsEditingChunkStructure(true);
    setError(null);

    try {
      const chunks = await knowledgeApi.mergeKnowledgeDocumentChunks(
        knowledgeBaseId,
        selectedChunkDocument.id,
        selectedChunk.chunkId,
        { direction }
      );
      applyChunkListUpdate(chunks, selectedChunk.chunkId);
    } catch {
      setError('Unable to merge document chunks. Retry the request or check the backend session.');
    } finally {
      setIsEditingChunkStructure(false);
    }
  };

  const handleSubmitKnowledgeDocument = async () => {
    if (!knowledgeBaseId) {
      return;
    }

    const trimmedTitle = knowledgeDocumentTitle.trim();
    const trimmedContent = knowledgeDocumentContent.trim();
    if (trimmedTitle === '') {
      return;
    }

    setIsSavingDocument(true);
    setError(null);

    try {
      const documentPayload = buildDocumentPayload(
        trimmedTitle,
        trimmedContent,
        knowledgeDocumentVersion,
        knowledgeDocumentUpdateStrategy,
        knowledgeDocumentSourceUrl,
        knowledgeDocumentSourcePage
      );
      if (editingDocumentId) {
        const updatedDocument = await knowledgeApi.updateKnowledgeDocument(knowledgeBaseId, editingDocumentId, documentPayload);
        setKnowledgeDocuments((current) =>
          current.map((document) => (document.id === editingDocumentId ? updatedDocument : document))
        );
      } else {
        const createdDocument = await knowledgeApi.createKnowledgeDocument(knowledgeBaseId, documentPayload);
        setKnowledgeDocuments((current) => [createdDocument, ...current]);
        setSelectedKnowledgeBase((current) =>
          current
            ? {
                ...current,
                documentCount: current.documentCount + 1
              }
            : current
        );
      }

      resetKnowledgeRetrieval();
      resetDocumentChunks();
      resetDocumentEditor();
    } catch {
      setError(
        editingDocumentId
          ? 'Unable to update knowledge document. Retry the request or check the backend session.'
          : 'Unable to create knowledge document. Retry the request or check the backend session.'
      );
    } finally {
      setIsSavingDocument(false);
    }
  };

  const handleUploadKnowledgeDocument = async () => {
    if (!knowledgeBaseId || !uploadDocumentFile) {
      return;
    }

    setIsUploadingDocument(true);
    setError(null);

    try {
      const uploadedDocument = await knowledgeApi.uploadKnowledgeDocument(
        knowledgeBaseId,
        buildUploadDocumentPayload(
          uploadDocumentFile,
          uploadDocumentTitle,
          uploadDocumentVersion,
          uploadDocumentUpdateStrategy,
          uploadDocumentSourceUrl,
          uploadDocumentSourcePage
        )
      );
      setKnowledgeDocuments((current) => [uploadedDocument, ...current]);
      setSelectedKnowledgeBase((current) =>
        current
          ? {
              ...current,
              documentCount: current.documentCount + 1
            }
          : current
      );
      resetKnowledgeRetrieval();
      resetDocumentChunks();
      resetUploadDocumentForm();
    } catch {
      setError('Unable to upload knowledge document. Retry the request or check the backend session.');
    } finally {
      setIsUploadingDocument(false);
    }
  };

  const handleEditKnowledgeDocument = (document: KnowledgeDocumentSummary) => {
    setEditingDocumentId(document.id);
    setKnowledgeDocumentTitle(document.title);
    setKnowledgeDocumentContent(document.content);
    setKnowledgeDocumentVersion(document.documentVersion ?? '');
    setKnowledgeDocumentUpdateStrategy(document.updateStrategy ?? defaultDocumentUpdateStrategy);
    setKnowledgeDocumentSourceUrl('');
    setKnowledgeDocumentSourcePage('');
  };

  const handleDeleteKnowledgeDocument = async (document: KnowledgeDocumentSummary) => {
    if (!knowledgeBaseId) {
      return;
    }

    setIsDeletingDocumentId(document.id);
    setError(null);

    try {
      await knowledgeApi.deleteKnowledgeDocument(knowledgeBaseId, document.id);
      setKnowledgeDocuments((current) => current.filter((currentDocument) => currentDocument.id !== document.id));
      setSelectedKnowledgeBase((current) =>
        current
          ? {
              ...current,
              documentCount: Math.max(current.documentCount - 1, 0)
            }
            : current
      );
      resetKnowledgeRetrieval();
      if (editingDocumentId === document.id) {
        resetDocumentEditor();
      }
      if (selectedChunkDocument?.id === document.id) {
        resetDocumentChunks();
      }
    } catch {
      setError('Unable to delete knowledge document. Retry the request or check the backend session.');
    } finally {
      setIsDeletingDocumentId(null);
    }
  };

  const isEditingDocument = editingDocumentId !== null;

  return (
    <section>
      <h1>{selectedKnowledgeBase ? selectedKnowledgeBase.name : 'Knowledge'}</h1>
      <p>
        {selectedKnowledgeBase
          ? 'Manage reusable documents in this knowledge base and retrieve embedding-backed RAG citations for relevant context.'
          : 'Organize reusable workspace context into knowledge bases and retrieve source-cited RAG context from each detail view.'}
      </p>
      {isLoading ? <p>{knowledgeBaseId ? 'Loading knowledge base…' : 'Loading knowledge bases…'}</p> : null}
      {error ? <p>{error}</p> : null}
      <p>Model strategy: {authState.preferences?.modelStrategy ?? 'balanced'}</p>
      <p>Web suggestions: {authState.preferences?.networkEnabledHint ? 'Enabled' : 'Disabled'}</p>
      <p>
        {authState.preferences?.networkEnabledHint
          ? 'Web suggestions are enabled for broader chat context alongside workspace knowledge retrieval.'
          : 'Enable web suggestions in settings if you want broader context beyond your indexed knowledge base.'}
      </p>
      {selectedKnowledgeBase ? (
        <>
          <label>
            Knowledge base name
            <input onChange={(event) => setKnowledgeBaseName(event.target.value)} type="text" value={knowledgeBaseName} />
          </label>
          <button
            disabled={isSavingKnowledgeBase || knowledgeBaseName.trim() === ''}
            onClick={() => void handleSaveKnowledgeBase()}
            type="button"
          >
            Save knowledge base
          </button>
          <button disabled={isDeletingKnowledgeBase} onClick={() => void handleDeleteKnowledgeBase()} type="button">
            Delete knowledge base
          </button>
          <p>Knowledge base ID: {selectedKnowledgeBase.id}</p>
          <p>Documents: {selectedKnowledgeBase.documentCount}</p>
          <p>{`Retrieval strategy: ${knowledgeBaseRetrievalMode}`}</p>
          <p>{`Chunking: ${knowledgeBaseChunkStrategy} · ${knowledgeBaseChunkSize} chars · ${knowledgeBaseChunkOverlap} overlap`}</p>
          <p>{`Reranking: ${knowledgeBaseRerankerModel} · top ${knowledgeBaseRerankTopK}`}</p>
          <label>
            Default retrieval strategy
            <select
              onChange={(event) => {
                setKnowledgeBaseRetrievalMode(event.target.value as KnowledgeRetrievalMode);
                setKnowledgeBaseRagConfigDirty(true);
              }}
              value={knowledgeBaseRetrievalMode}
            >
              <option value="vector_only">vector_only</option>
              <option value="hybrid">hybrid</option>
              <option value="hybrid_rerank">hybrid_rerank</option>
            </select>
          </label>
          <label>
            Chunking strategy
            <select
              onChange={(event) => {
                setKnowledgeBaseChunkStrategy(event.target.value as KnowledgeChunkStrategy);
                setKnowledgeBaseRagConfigDirty(true);
              }}
              value={knowledgeBaseChunkStrategy}
            >
              <option value="fixed_size">fixed_size</option>
              <option value="semantic">semantic</option>
              <option value="qa_split">qa_split</option>
              <option value="template_based">template_based</option>
            </select>
          </label>
          <label>
            Chunk size
            <input
              max="4000"
              min="100"
              onChange={(event) => {
                setKnowledgeBaseChunkSize(Math.trunc(clampNumber(Number(event.target.value), 100, 4000)));
                setKnowledgeBaseRagConfigDirty(true);
              }}
              type="number"
              value={knowledgeBaseChunkSize}
            />
          </label>
          <label>
            Chunk overlap
            <input
              max="1000"
              min="0"
              onChange={(event) => {
                setKnowledgeBaseChunkOverlap(Math.trunc(clampNumber(Number(event.target.value), 0, 1000)));
                setKnowledgeBaseRagConfigDirty(true);
              }}
              type="number"
              value={knowledgeBaseChunkOverlap}
            />
          </label>
          <label>
            Embedding model
            <input
              onChange={(event) => {
                setKnowledgeBaseEmbeddingModel(event.target.value);
                setKnowledgeBaseRagConfigDirty(true);
              }}
              type="text"
              value={knowledgeBaseEmbeddingModel}
            />
          </label>
          <label>
            Reranker model
            <input
              onChange={(event) => {
                setKnowledgeBaseRerankerModel(event.target.value);
                setKnowledgeBaseRagConfigDirty(true);
              }}
              type="text"
              value={knowledgeBaseRerankerModel}
            />
          </label>
          <label>
            Rerank top K
            <input
              max="50"
              min="1"
              onChange={(event) => {
                setKnowledgeBaseRerankTopK(parseRerankTopK(event.target.value));
                setKnowledgeBaseRagConfigDirty(true);
              }}
              type="number"
              value={knowledgeBaseRerankTopK}
            />
          </label>
          <label>
            Retrieval query
            <input onChange={(event) => setRetrievalQuery(event.target.value)} type="text" value={retrievalQuery} />
          </label>
          <label>
            Retrieval limit
            <input
              max="20"
              min="1"
              onChange={(event) => setRetrievalLimit(parseRetrievalLimit(event.target.value))}
              type="number"
              value={retrievalLimit}
            />
          </label>
          <label>
            Similarity threshold
            <input
              max="1"
              min="0"
              onChange={(event) => setRetrievalMinScore(parseRetrievalMinScore(event.target.value))}
              step="0.01"
              type="number"
              value={retrievalMinScore}
            />
          </label>
          <label>
            Retrieval mode
            <select
              onChange={(event) => setRetrievalModeSelection(event.target.value as RetrievalModeSelection)}
              value={retrievalModeSelection}
            >
              <option value="knowledge_base_default">knowledge_base_default</option>
              <option value="vector_only">vector_only</option>
              <option value="hybrid">hybrid</option>
              <option value="hybrid_rerank">hybrid_rerank</option>
            </select>
          </label>
          <label>
            Retrieval document version
            <input
              disabled={retrievalAllVersions}
              onChange={(event) => setRetrievalDocumentVersion(event.target.value)}
              type="text"
              value={retrievalDocumentVersion}
            />
          </label>
          <label>
            <input
              checked={retrievalAllVersions}
              onChange={(event) => {
                setRetrievalAllVersions(event.target.checked);
                if (event.target.checked) {
                  setRetrievalDocumentVersion('');
                }
              }}
              type="checkbox"
            />
            Search all document versions
          </label>
          <button
            disabled={isRetrievingKnowledge || retrievalQuery.trim() === ''}
            onClick={() => void handleRetrieveKnowledge()}
            type="button"
          >
            Search knowledge
          </button>
          {hasRetrievedKnowledge ? <h2>RAG citations</h2> : null}
          {retrievalMetrics ? <p>{formatRetrievalMetrics(retrievalMetrics)}</p> : null}
          {hasRetrievedKnowledge && retrievalResults.length === 0 ? (
            <p>{`No matching snippets found for “${lastRetrievedQuery}”.`}</p>
          ) : null}
          {retrievalResults.length > 0 ? (
            <ul>
              {retrievalResults.map((result) => (
                <li key={`${result.documentId}-${result.snippet}`}>
                  {(() => {
                    const source = result.source;
                    return (
                      <>
                        <strong>{result.documentTitle}</strong>
                        <p>{result.snippet}</p>
                        <p>{`Score: ${formatRetrievalSimilarity(result.similarity)}`}</p>
                        <progress
                          aria-label={`Similarity score for ${result.chunkId}`}
                          max="1"
                          value={Number.isFinite(result.similarity) ? result.similarity : 0}
                        />
                        <p>{`Method: ${result.retrievalMethod}`}</p>
                        <p>{`Version: ${formatDocumentVersion(result.documentVersion ?? source?.documentVersion)}`}</p>
                        <p>{formatRetrievalSource(result)}</p>
                        {source?.pageNumber ? <p>{`Page: ${source.pageNumber}`}</p> : null}
                        {source?.sourceUrl ? (
                          <p>
                            <a href={source.sourceUrl} rel="noreferrer" target="_blank">
                              Open source document
                            </a>
                          </p>
                        ) : null}
                        {source?.originalText ? <p>{`Original text: ${source.originalText}`}</p> : null}
                        {source?.matchedSnippet ? <p>{renderMatchedSnippet(source.matchedSnippet)}</p> : null}
                        <button
                          onClick={() => void handleViewDocumentChunks(documentSummaryForRetrievalResult(result), result.chunkId)}
                          type="button"
                        >
                          {`View chunk ${result.chunkId}`}
                        </button>
                        <button
                          disabled={isSavingRetrievalTestCaseChunkId === result.chunkId}
                          onClick={() => void handleCreateRetrievalTestCase(result)}
                          type="button"
                        >
                          {`Add result ${result.chunkId} to test set`}
                        </button>
                        {source?.highlightPositions?.length ? (
                          <ul aria-label={`Highlight positions for ${result.chunkId}`}>
                            {source.highlightPositions.map((position) => (
                              <li key={`${position.start}-${position.end}`}>{`Highlight: ${formatHighlightPosition(position)}`}</li>
                            ))}
                          </ul>
                        ) : null}
                      </>
                    );
                  })()}
                </li>
              ))}
            </ul>
          ) : null}
          {savedRetrievalTestCaseId ? <p>{`Saved retrieval test case ${savedRetrievalTestCaseId}`}</p> : null}
          <section aria-label="Retrieval test set">
            <h2>Retrieval test set</h2>
            <p>{`Saved cases: ${retrievalTestCases.length}`}</p>
            <button
              disabled={isRunningRetrievalTests || retrievalTestCases.length === 0}
              onClick={() => void handleRunRetrievalTests()}
              type="button"
            >
              Run retrieval tests
            </button>
            {retrievalTestCases.length === 0 ? <p>No retrieval test cases saved yet.</p> : null}
            {retrievalTestCases.length > 0 ? (
              <ul>
                {retrievalTestCases.map((testCase) => (
                  <li key={testCase.id}>
                    <strong>{testCase.query}</strong>
                    <p>{formatExpectedRetrievalTestCase(testCase)}</p>
                  </li>
                ))}
              </ul>
            ) : null}
            {retrievalTestRunReport ? (
              <section aria-label="Retrieval test run report">
                <h3>{`Evaluation: ${retrievalTestRunReport.passed} passed / ${retrievalTestRunReport.failed} failed / ${retrievalTestRunReport.total} total`}</h3>
                <ul>
                  {retrievalTestRunReport.results.map((result) => (
                    <li key={result.testCaseId}>
                      <p>
                        {result.passed
                          ? `${result.testCaseId} passed at rank ${result.rank}`
                          : `${result.testCaseId} failed: ${result.reason || 'expected retrieval result was not returned'}`}
                      </p>
                      {result.actualResult ? <p>{`Actual top: ${formatRunResultSource(result.actualResult)}`}</p> : null}
                    </li>
                  ))}
                </ul>
              </section>
            ) : null}
          </section>
          <label>
            Document title
            <input onChange={(event) => setKnowledgeDocumentTitle(event.target.value)} type="text" value={knowledgeDocumentTitle} />
          </label>
          <label>
            Document content
            <textarea onChange={(event) => setKnowledgeDocumentContent(event.target.value)} value={knowledgeDocumentContent} />
          </label>
          <label>
            Document version
            <input onChange={(event) => setKnowledgeDocumentVersion(event.target.value)} type="text" value={knowledgeDocumentVersion} />
          </label>
          <label>
            Document source URL
            <input onChange={(event) => setKnowledgeDocumentSourceUrl(event.target.value)} type="url" value={knowledgeDocumentSourceUrl} />
          </label>
          <label>
            Document source page
            <input min="1" onChange={(event) => setKnowledgeDocumentSourcePage(event.target.value)} type="number" value={knowledgeDocumentSourcePage} />
          </label>
          <label>
            Update strategy
            <select
              onChange={(event) => setKnowledgeDocumentUpdateStrategy(event.target.value as KnowledgeUpdateStrategy)}
              value={knowledgeDocumentUpdateStrategy}
            >
              <option value="full_replace">full_replace</option>
              <option value="incremental">incremental</option>
              <option value="versioned">versioned</option>
            </select>
          </label>
          <button
            disabled={isSavingDocument || knowledgeDocumentTitle.trim() === ''}
            onClick={() => void handleSubmitKnowledgeDocument()}
            type="button"
          >
            {isEditingDocument ? 'Save document' : 'Create document'}
          </button>
          {isEditingDocument ? (
            <button disabled={isSavingDocument} onClick={resetDocumentEditor} type="button">
              Cancel document edit
            </button>
          ) : null}
          <label>
            Document file
            <input
              accept={supportedUploadAccept}
              key={uploadDocumentFileInputKey}
              onChange={(event) => handleUploadDocumentFileChange(event.target.files?.[0] ?? null)}
              type="file"
            />
          </label>
          <label>
            Upload title
            <input onChange={(event) => setUploadDocumentTitle(event.target.value)} type="text" value={uploadDocumentTitle} />
          </label>
          <label>
            Upload document version
            <input onChange={(event) => setUploadDocumentVersion(event.target.value)} type="text" value={uploadDocumentVersion} />
          </label>
          <label>
            Upload source URL
            <input onChange={(event) => setUploadDocumentSourceUrl(event.target.value)} type="url" value={uploadDocumentSourceUrl} />
          </label>
          <label>
            Upload source page
            <input min="1" onChange={(event) => setUploadDocumentSourcePage(event.target.value)} type="number" value={uploadDocumentSourcePage} />
          </label>
          <label>
            Upload update strategy
            <select
              onChange={(event) => setUploadDocumentUpdateStrategy(event.target.value as KnowledgeUpdateStrategy)}
              value={uploadDocumentUpdateStrategy}
            >
              <option value="full_replace">full_replace</option>
              <option value="incremental">incremental</option>
              <option value="versioned">versioned</option>
            </select>
          </label>
          <button
            disabled={isUploadingDocument || !uploadDocumentFile}
            onClick={() => void handleUploadKnowledgeDocument()}
            type="button"
          >
            Upload document
          </button>
          {knowledgeDocuments.length === 0 ? <p>No documents yet. Add one to seed this knowledge base.</p> : null}
          {knowledgeDocuments.length > 0 ? (
            <ul>
              {knowledgeDocuments.map((document) => (
                <li key={document.id}>
                  <strong>{document.title}</strong>
                  <p>{document.content}</p>
                  <p>{`Version: ${formatDocumentVersion(document.documentVersion)}`}</p>
                  {document.updateStrategy ? <p>{`Update strategy: ${document.updateStrategy}`}</p> : null}
                  <button
                    disabled={isLoadingDocumentChunks && selectedChunkDocument?.id === document.id}
                    onClick={() => void handleViewDocumentChunks(document)}
                    type="button"
                  >
                    {`View chunks for ${document.title}`}
                  </button>
                  <button onClick={() => handleEditKnowledgeDocument(document)} type="button">
                    {`Edit document ${document.title}`}
                  </button>
                  <button
                    disabled={isDeletingDocumentId === document.id}
                    onClick={() => void handleDeleteKnowledgeDocument(document)}
                    type="button"
                  >
                    {`Delete document ${document.title}`}
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
          {selectedChunkDocument ? (
            <section aria-label="Document chunks">
              <h2>{`Chunks for ${selectedChunkDocument.title}`}</h2>
              {isLoadingDocumentChunks ? <p>Loading document chunks…</p> : null}
              {!isLoadingDocumentChunks && knowledgeDocumentChunks.length === 0 ? <p>No chunks indexed for this document.</p> : null}
              {knowledgeDocumentChunks.length > 0 ? (
                <>
                  <ul>
                    {knowledgeDocumentChunks.map((chunk) => {
                      const isSelected = selectedChunk?.chunkId === chunk.chunkId;
                      return (
                        <li key={chunk.chunkId}>
                          <button
                            aria-pressed={isSelected}
                            onClick={() => setSelectedChunkId(chunk.chunkId)}
                            type="button"
                          >
                            {`Chunk ${chunk.chunkIndex + 1} ${chunk.chunkId}${isSelected ? ' selected' : ''}`}
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                  <section aria-label="Original document preview">
                    <h3>{`Original preview: ${selectedChunkDocument.title}`}</h3>
                    <p>{formatDocumentPreviewKind(chunkPreviewKind)}</p>
                    {isPositivePageNumber(chunkPreviewPageNumber) ? <p>{`Page ${Math.trunc(chunkPreviewPageNumber)}`}</p> : null}
                    {chunkPreviewHref ? (
                      <p>
                        <a href={chunkPreviewHref} rel="noreferrer" target="_blank">
                          {formatDocumentPreviewLinkLabel(chunkPreviewKind, chunkPreviewPageNumber)}
                        </a>
                      </p>
                    ) : null}
                    <ul aria-label="Chunk boundary overlay">
                      {knowledgeDocumentChunks.map((chunk) => {
                        const isSelected = selectedChunk?.chunkId === chunk.chunkId;
                        return (
                          <li key={`preview-${chunk.chunkId}`}>
                            <button
                              aria-pressed={isSelected}
                              onClick={() => setSelectedChunkId(chunk.chunkId)}
                              type="button"
                            >
                              {`Preview chunk boundary ${chunk.chunkIndex + 1} ${chunk.chunkId}${isSelected ? ' selected' : ''}`}
                            </button>
                          </li>
                        );
                      })}
                    </ul>
                    {selectedChunk ? (
                      <p>{renderHighlightedChunkContent(selectedChunk.content, lastRetrievedQuery || retrievalQuery)}</p>
                    ) : null}
                  </section>
                  {selectedChunk ? (
                    <section aria-label="Selected chunk details">
                      <h3>Selected chunk</h3>
                      <p>{`Chunk ${selectedChunk.chunkIndex + 1}`}</p>
                      <p>{renderHighlightedChunkContent(selectedChunk.content, lastRetrievedQuery || retrievalQuery)}</p>
                      <p>{`chunk_id: ${selectedChunk.chunkId}`}</p>
                      <p>{`Characters: ${selectedChunk.charCount}`}</p>
                      <p>{`Estimated tokens: ${selectedChunk.estimatedTokenCount}`}</p>
                      <p>{`Document version: ${selectedChunk.documentVersion}`}</p>
                      <p>{`documentVersion: ${selectedChunk.metadata.documentVersion ?? selectedChunk.documentVersion}`}</p>
                      {selectedChunk.metadata.pageNumber ? <p>{`Page: ${selectedChunk.metadata.pageNumber}`}</p> : null}
                      {selectedChunk.metadata.sourceUrl ? (
                        <p>
                          <a href={selectedChunk.metadata.sourceUrl} rel="noreferrer" target="_blank">
                            Open chunk source
                          </a>
                        </p>
                      ) : null}
                      <label>
                        Chunk content editor
                        <textarea
                          onChange={(event) => setKnowledgeChunkContentDraft(event.target.value)}
                          value={knowledgeChunkContentDraft}
                        />
                      </label>
                      <button
                        disabled={isSavingChunk || knowledgeChunkContentDraft.trim() === ''}
                        onClick={() => void handleSaveKnowledgeDocumentChunk()}
                        type="button"
                      >
                        Save chunk
                      </button>
                      <label>
                        Chunk split position
                        <input
                          max={Math.max(selectedChunk.charCount - 1, 1)}
                          min={1}
                          onChange={(event) => setKnowledgeChunkSplitAt(Number(event.target.value))}
                          type="number"
                          value={knowledgeChunkSplitAt}
                        />
                      </label>
                      <button
                        disabled={
                          isEditingChunkStructure ||
                          knowledgeChunkSplitAt <= 0 ||
                          knowledgeChunkSplitAt >= selectedChunk.charCount
                        }
                        onClick={() => void handleSplitKnowledgeDocumentChunk()}
                        type="button"
                      >
                        Split chunk
                      </button>
                      <button
                        disabled={isEditingChunkStructure || selectedChunkListIndex <= 0}
                        onClick={() => void handleMergeKnowledgeDocumentChunks('previous')}
                        type="button"
                      >
                        Merge with previous chunk
                      </button>
                      <button
                        disabled={isEditingChunkStructure || selectedChunkListIndex < 0 || selectedChunkListIndex >= knowledgeDocumentChunks.length - 1}
                        onClick={() => void handleMergeKnowledgeDocumentChunks('next')}
                        type="button"
                      >
                        Merge with next chunk
                      </button>
                    </section>
                  ) : null}
                </>
              ) : null}
            </section>
          ) : null}
          <button onClick={() => navigate('/knowledge')} type="button">
            Back to knowledge bases
          </button>
          {returnTo ? (
            <button onClick={() => navigate(returnTo)} type="button">
              Back to chat
            </button>
          ) : null}
        </>
      ) : (
        <>
          <label>
            Knowledge base name
            <input onChange={(event) => setKnowledgeBaseName(event.target.value)} type="text" value={knowledgeBaseName} />
          </label>
          <button disabled={isCreating || knowledgeBaseName.trim() === ''} onClick={() => void handleCreateKnowledgeBase()} type="button">
            Create knowledge base
          </button>
          {!isLoading && knowledgeBases.length === 0 ? (
            <p>No knowledge bases yet. Create one to start collecting workspace context.</p>
          ) : null}
          {knowledgeBases.length > 0 ? (
            <ul>
              {knowledgeBases.map((knowledgeBase) => (
                <li key={knowledgeBase.id}>
                  <strong>{knowledgeBase.name}</strong>
                  <p>Documents: {knowledgeBase.documentCount}</p>
                  <button onClick={() => navigate(`/knowledge/${knowledgeBase.id}`)} type="button">
                    Open knowledge base
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
        </>
      )}
      <button onClick={() => navigate('/chat')} type="button">
        Open chat workspace
      </button>
      <button onClick={() => navigate('/settings')} type="button">
        Review workspace settings
      </button>
    </section>
  );
}
