export type ApiUser = {
  id: string;
  email: string;
  name?: string;
  role?: string;
};

export type ApiEnvelopeError = {
  code: string;
  message: string;
};

export type ApiEnvelope<T> = {
  ok: boolean;
  data: T | null;
  error: ApiEnvelopeError | null;
};

export type UserPreferences = {
  defaultMode: 'chat' | 'solo';
  modelStrategy: string;
  networkEnabledHint: boolean;
  onboardingCompleted: boolean;
};

export type ApiSession = {
  id: string;
  expiresAt: string;
};

export type ApiWorkspace = {
  id: string;
};

export type SessionResponse = {
  onboardingCompleted: boolean;
  preferences: UserPreferences;
  session: ApiSession;
  user: ApiUser;
  workspace: ApiWorkspace;
};

export type ConversationSummary = {
  id: string;
  title: string;
  archivedAt?: string;
  createdAt?: string;
  hasBookmarkedMessages?: boolean;
  parentId?: string;
  parent_id?: string;
  updatedAt?: string;
};

export type ConversationMessage = {
  attachments?: MessageAttachment[];
  id: string;
  role: string;
  content: string;
  bookmarked?: boolean;
  createdAt?: string;
  knowledgeCitations?: KnowledgeCitation[];
};

export type KnowledgeCitation = {
  chunkId?: string;
  chunkIndex?: number;
  documentId?: string;
  documentTitle?: string;
  documentVersion?: string;
  highlightPositions?: Array<{ end: number; start: number }>;
  knowledgeBaseId?: string;
  knowledgeBaseName?: string;
  originalText?: string;
  pageNumber?: number;
  retrievalMethod?: string;
  score?: number;
  snippet: string;
  sourceUrl?: string;
};

export type MessageAttachment = {
  contentType: string;
  id: string;
  name: string;
  providerFileId?: string;
  sizeBytes: number;
  type: 'file' | 'image';
  url?: string;
};

export type ConversationConfig = {
  conversationId: string;
  knowledgeBaseIds: string[];
  modelId: string;
  personaId?: string;
  personaRole?: string;
  personaStyle?: string;
  personaTone?: string;
  personaConstraints?: string;
  systemPromptOverride: string;
  temperature: number;
  maxOutputTokens: number;
  toolsEnabled: boolean;
  updatedAt?: string;
};

export type PersonaSummary = {
  createdAt?: string;
  id: string;
  name: string;
  workspaceId?: string;
  role?: string;
  style?: string;
  tone?: string;
  constraints?: string;
  openingMessage?: string;
  suggestedQuestions?: string[];
};

export type PersonaRequest = {
  constraints?: string;
  name: string;
  openingMessage?: string;
  role?: string;
  style?: string;
  suggestedQuestions?: string[];
  tone?: string;
};

export type UpdateConversationConfigRequest = {
  knowledgeBaseIds: string[];
  modelId: string;
  personaId?: string;
  systemPromptOverride: string;
  temperature: number;
  maxOutputTokens: number;
  toolsEnabled: boolean;
};

export type CreateConversationRequest = {
  title?: string;
};

export type ForkConversationRequest = {
  messageId?: string;
  title?: string;
};

export type SendMessageRequest = {
  attachments?: MessageAttachment[];
  content: string;
  overrides?: {
    modelId?: string;
    systemPromptOverride?: string;
    temperature?: number;
    maxOutputTokens?: number;
    toolsEnabled?: boolean;
  };
};

export type UpdateConversationMessageRequest = {
  content: string;
};

export type BookmarkConversationMessageRequest = {
  bookmarked: boolean;
};

export type MessageShareResponse = {
  id?: string;
  shareId?: string;
  shareUrl?: string;
  url?: string;
};

export type ConversationShareResponse = MessageShareResponse;

export type ConvertConversationToTaskResponse = {
  draftTaskGoal: string;
  relatedKnowledgeBaseIds: string[];
  suggestedBudget: number;
  suggestedExecutionMode: string;
};

export type ModelOption = {
  id: string;
  label: string;
};

export type KnowledgeRetrievalMode = 'vector_only' | 'hybrid' | 'hybrid_rerank';

export type KnowledgeChunkStrategy = 'fixed_size' | 'semantic' | 'qa_split' | 'template_based';

export type KnowledgeBaseRagConfig = {
  chunkOverlap?: number;
  chunkSize?: number;
  chunkStrategy?: KnowledgeChunkStrategy;
  embeddingModel?: string;
  rerankTopK?: number;
  rerankerModel?: string;
  retrievalMode?: KnowledgeRetrievalMode;
};

export type KnowledgeBaseSummary = KnowledgeBaseRagConfig & {
  id: string;
  name: string;
  documentCount: number;
  updatedAt?: string;
};

export type KnowledgeDocumentSummary = {
  id: string;
  title: string;
  content: string;
  documentVersion?: string;
  updateStrategy?: KnowledgeUpdateStrategy;
  updatedAt?: string;
};

export type KnowledgeDocumentVersion = {
  chunkCount: number;
  content: string;
  documentId: string;
  documentVersion: string;
  knowledgeBaseId: string;
  title: string;
  updateStrategy?: KnowledgeUpdateStrategy;
  updatedAt?: string;
};

export type KnowledgeDocumentChunk = {
  chunkId: string;
  chunkIndex: number;
  content: string;
  documentVersion: string;
  metadata: {
    documentVersion?: string;
    pageNumber?: number;
    sourceUrl?: string;
    startRune?: number;
    endRune?: number;
  };
  charCount: number;
  estimatedTokenCount: number;
};

export type SplitKnowledgeDocumentChunkRequest = {
  splitAt: number;
};

export type MergeKnowledgeDocumentChunksRequest = {
  direction: 'previous' | 'next';
};

export type KnowledgeRetrievalResult = {
  chunkId: string;
  chunkIndex: number;
  documentId: string;
  documentTitle: string;
  documentVersion?: string;
  retrievalMethod: string;
  similarity: number;
  snippet: string;
  source: KnowledgeRetrievalSource;
};

export type CreateKnowledgeRetrievalTestCaseRequest = {
  expectedResult: KnowledgeRetrievalResult;
  query: string;
};

export type KnowledgeRetrievalTestCase = {
  createdAt?: string;
  expectedChunkId: string;
  expectedChunkIndex: number;
  expectedDocumentId: string;
  expectedDocumentTitle?: string;
  expectedDocumentVersion?: string;
  expectedResult?: KnowledgeRetrievalResult;
  expectedSnippet?: string;
  id: string;
  knowledgeBaseId: string;
  organizationId?: string;
  query: string;
  updatedAt?: string;
};

export type KnowledgeRetrievalTestRunRequest = {
  allVersions?: boolean;
  benchmarkModes?: KnowledgeRetrievalMode[];
  documentVersion?: string;
  keywordWeight?: number;
  limit?: number;
  minScore?: number;
  mode?: KnowledgeRetrievalMode;
  vectorWeight?: number;
};

export type KnowledgeRetrievalTestRunResult = {
  actualResult?: KnowledgeRetrievalResult;
  expectedResult: KnowledgeRetrievalResult;
  matchedResult?: KnowledgeRetrievalResult;
  passed: boolean;
  query: string;
  rank: number;
  reason?: string;
  testCaseId: string;
};

export type KnowledgeRetrievalBenchmarkReport = {
  averageRank?: number;
  failed: number;
  mode: KnowledgeRetrievalMode;
  passed: number;
  passRate: number;
  results: KnowledgeRetrievalTestRunResult[];
  total: number;
};

export type KnowledgeRetrievalTestRunReport = {
  benchmarks?: KnowledgeRetrievalBenchmarkReport[];
  failed: number;
  knowledgeBaseId: string;
  passed: number;
  ranAt?: string;
  results: KnowledgeRetrievalTestRunResult[];
  total: number;
};

export type KnowledgeHighlightPosition = {
  start: number;
  end: number;
};

export type KnowledgeRetrievalSource = {
  chunkId: string;
  chunkIndex: number;
  documentId: string;
  documentTitle: string;
  documentVersion?: string;
  pageNumber?: number;
  sourceUrl?: string;
  originalText?: string;
  matchedSnippet?: string;
  highlightPositions?: KnowledgeHighlightPosition[];
};

export type CreateKnowledgeBaseRequest = KnowledgeBaseRagConfig & {
  name: string;
};

export type UpdateKnowledgeBaseRequest = KnowledgeBaseRagConfig & {
  name: string;
};

export type KnowledgeUpdateStrategy = 'full_replace' | 'incremental' | 'versioned';

export type CreateKnowledgeDocumentRequest = {
  title: string;
  content: string;
  documentVersion?: string;
  pageNumber?: number;
  sourceUrl?: string;
  updateStrategy?: KnowledgeUpdateStrategy;
};

export type UploadKnowledgeDocumentRequest = {
  file: File;
  title?: string;
  documentVersion?: string;
  pageNumber?: number;
  sourceUrl?: string;
  updateStrategy?: KnowledgeUpdateStrategy;
};

export type UpdateKnowledgeDocumentRequest = {
  title: string;
  content: string;
  documentVersion?: string;
  updateStrategy?: KnowledgeUpdateStrategy;
};

export type RetrieveKnowledgeRequest = {
  allVersions?: boolean;
  documentVersion?: string;
  keywordWeight?: number;
  limit?: number;
  minScore?: number;
  mode?: KnowledgeRetrievalMode;
  query: string;
  vectorWeight?: number;
};

export type TaskStatus =
  | 'draft'
  | 'running'
  | 'paused'
  | 'completed'
  | 'cancelled'
  | 'awaiting_confirmation'
  | 'failed'
  | 'stopped';

export type ExecutionMode = 'safe' | 'standard' | 'auto';

export type AuthorizationScope = 'knowledge_only' | 'workspace_tools' | 'full_access';

export type TaskStep = {
  id: string;
  title: string;
  status: string;
  stepIndex: number;
  createdAt?: string;
  startedAt?: string;
  finishedAt?: string;
  updatedAt?: string;
};

export type TaskEvent = {
  type: string;
  message: string;
  createdAt?: string;
};

export type TaskResultArtifact = {
  label: string;
  value: string;
};

export type TaskSummary = {
  id: string;
  title: string;
  goal: string;
  status: TaskStatus;
  executionMode: ExecutionMode;
  budgetLimit: number;
  budgetConsumed?: number;
  authorizationScope?: AuthorizationScope;
  createdAt?: string;
  startedAt?: string;
  finishedAt?: string;
  updatedAt?: string;
};

export type TaskDetail = TaskSummary & {
  authorizationScope: AuthorizationScope;
  currentStep?: string;
  events?: TaskEvent[];
  knowledgeBaseIds: string[];
  resultArtifacts?: TaskResultArtifact[];
  toolAllowList?: string[];
  toolDenyList?: string[];
  resultSummary?: string;
  steps: TaskStep[];
};

export type CreateTaskRequest = {
  title?: string;
  goal: string;
  executionMode: ExecutionMode | string;
  authorizationScope: AuthorizationScope | string;
  budgetLimit: number;
  knowledgeBaseIds: string[];
  toolAllowList?: string[];
  toolDenyList?: string[];
};

export type UpdateTaskBudgetRequest = {
  budgetLimit: number;
};

export type ScheduledTaskRun = {
  id: string;
  scheduledTaskId: string;
  status: string;
  startedAt?: string | null;
  finishedAt?: string | null;
  error?: string | null;
  createdAt?: string;
  updatedAt?: string;
};

export type UsageDimensionSummary = {
  key: string;
  requestCount: number;
  totalTokens: number;
  totalCost: number;
};

export type UsageTimeSeriesSummary = {
  bucket: string;
  requestCount: number;
  totalTokens: number;
  totalCost: number;
};

export type UsageSummary = {
  period: string;
  byFeature?: UsageDimensionSummary[];
  byModel?: UsageDimensionSummary[];
  byUser?: UsageDimensionSummary[];
  recent?: RelayApiTokenUsageItem[];
  requests: number;
  timeSeries?: UsageTimeSeriesSummary[];
};

export type ModelSummary = {
  id: string;
  label: string;
  requests: number;
};

export type BillingSummary = {
  period: string;
  requests: number;
  inputTokens: number;
  outputTokens: number;
  estimatedCostUsd: number;
};

export type AccessSummary = {
  defaultMode: string;
  modelStrategy: string;
  networkEnabledHint: boolean;
  onboardingCompleted: boolean;
  sessionExpiresAt: string;
  sessionId: string;
  userEmail: string;
  userId: string;
  workspaceId: string;
};

export type RelayApiToken = {
  id: string;
  name: string;
  tokenPrefix: string;
  status: string;
  userGroup?: string;
  modelLimitsEnabled: boolean;
  modelLimits: string[];
  quotaLimit?: number;
  usedQuota: number;
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt: string;
  revokedAt?: string;
};

export type RelayApiTokenUsageItem = {
  id: string;
  apiTokenId: string;
  requestId: string;
  apiType: string;
  model: string;
  channelId: string;
  provider: string;
  status: string;
  statusCode: number;
  errorCode?: string;
  latencyMs: number;
  cost: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  createdAt: string;
};

export type CreateRelayApiTokenRequest = {
  name: string;
  modelLimitsEnabled: boolean;
  modelLimits: string[];
  userGroup?: string;
  quotaLimit?: number;
  expiresAt?: string;
};

export type CreatedRelayApiToken = {
  rawToken: string;
  token: RelayApiToken;
};

export type AgentTool = {
  name: string;
  description?: string;
  type?: 'builtin' | 'mcp' | string;
  serverId?: string;
  enabled?: boolean;
  requiresApproval?: boolean;
  riskLevel?: 'safe' | 'medium' | 'dangerous' | string;
  inputSchema?: unknown;
  runtime?: 'api' | 'python' | string;
  sourceCode?: string;
  timeoutSeconds?: number;
};

export type ToolApprovalOverride = {
  requiresApproval?: boolean;
  riskLevel?: 'safe' | 'medium' | 'dangerous' | string;
};

export type AgentConfig = {
  enableMemory?: boolean;
  maxIterations?: number;
  maxTokens?: number;
  tokenBudget?: number;
  defaultExecutionMode?: string;
  temperature?: number;
  topP?: number;
  knowledgeBaseIds?: string[];
  approvalMode?: 'tiered' | 'all' | 'none' | 'custom' | string;
  toolApprovalOverrides?: Record<string, ToolApprovalOverride>;
};

export type AgentSummary = {
  id: string;
  userId: string;
  name: string;
  description?: string;
  model: string;
  systemPrompt?: string;
  tools?: AgentTool[];
  config?: AgentConfig;
  isPublic: boolean;
  createdAt: string;
  updatedAt: string;
};

export type AgentDetail = AgentSummary;

export type AgentConversationSummary = {
  id: string;
  agentId: string;
  userId: string;
  title?: string;
  createdAt: string;
  updatedAt: string;
};

export type AgentToolCall = {
  id: string;
  name: string;
  arguments: Record<string, unknown>;
};

export type AgentMessage = {
  id: string;
  conversationId: string;
  role: 'user' | 'assistant' | 'tool' | string;
  content: string;
  toolCalls?: AgentToolCall[];
  toolCallId?: string;
  createdAt: string;
};

export type MemoryDocumentSummary = {
  id: string;
  userId: string;
  title?: string;
  content: string;
  sourceType: string;
  sourceUrl?: string;
  metadata?: Record<string, unknown>;
  totalChunks: number;
  embeddingModel: string;
  createdAt: string;
  updatedAt: string;
};

export type MemoryChunk = {
  id: string;
  documentId: string;
  userId: string;
  content: string;
  chunkIndex: number;
  embedding?: number[];
  metadata?: Record<string, unknown>;
  createdAt: string;
};

export type MemorySearchRequest = {
  query: string;
  topK?: number;
  minScore?: number;
};

export type MemorySearchResult = {
  documentId: string;
  documentTitle: string;
  chunkContent: string;
  chunkIndex: number;
  score: number;
};

export type McpServer = {
  id: string;
  userId: string;
  name: string;
  url: string;
  authToken?: string;
  hasAuthToken?: boolean;
  status: 'connected' | 'disconnected' | 'error' | string;
  lastConnectedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type McpServerRequest = {
  name: string;
  url: string;
  authToken?: string;
  command?: string;
  args?: string[];
  description?: string;
};

export type McpTool = {
  name: string;
  description?: string;
  inputSchema: unknown;
};

export type Notification = {
  id: string;
  userId: string;
  type: 'info' | 'warning' | 'error' | 'success' | string;
  category: 'billing' | 'agent' | 'system' | 'mcp' | string;
  title: string;
  message: string;
  isRead: boolean;
  actionUrl?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  readAt?: string;
};

export type QuotaSnapshot = {
  id: string;
  userId: string;
  balance: number;
  used: number;
  createdAt: string;
  updatedAt: string;
};

export type PackageOption = {
  id: string;
  name: string;
  description?: string;
  quotaAmount: number;
  price: number;
  durationDays?: number;
  isActive: boolean;
  sortOrder: number;
  createdAt: string;
};

export type QuotaTopupRequest = {
  amount: number;
};
