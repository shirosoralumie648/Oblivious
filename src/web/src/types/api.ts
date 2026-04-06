export type ApiUser = {
  id: string;
  email: string;
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
  createdAt?: string;
  updatedAt?: string;
};

export type ConversationMessage = {
  id: string;
  role: string;
  content: string;
  createdAt?: string;
};

export type ConversationConfig = {
  conversationId: string;
  knowledgeBaseIds: string[];
  modelId: string;
  systemPromptOverride: string;
  temperature: number;
  maxOutputTokens: number;
  toolsEnabled: boolean;
  updatedAt?: string;
};

export type UpdateConversationConfigRequest = {
  knowledgeBaseIds: string[];
  modelId: string;
  systemPromptOverride: string;
  temperature: number;
  maxOutputTokens: number;
  toolsEnabled: boolean;
};

export type CreateConversationRequest = {
  title?: string;
};

export type SendMessageRequest = {
  content: string;
  overrides?: {
    modelId?: string;
    systemPromptOverride?: string;
    temperature?: number;
    maxOutputTokens?: number;
    toolsEnabled?: boolean;
  };
};

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

export type KnowledgeBaseSummary = {
  id: string;
  name: string;
  documentCount: number;
  updatedAt?: string;
};

export type KnowledgeDocumentSummary = {
  id: string;
  title: string;
  content: string;
  updatedAt?: string;
};

export type KnowledgeRetrievalResult = {
  documentId: string;
  documentTitle: string;
  snippet: string;
};

export type CreateKnowledgeBaseRequest = {
  name: string;
};

export type UpdateKnowledgeBaseRequest = {
  name: string;
};

export type CreateKnowledgeDocumentRequest = {
  title: string;
  content: string;
};

export type UpdateKnowledgeDocumentRequest = {
  title: string;
  content: string;
};

export type RetrieveKnowledgeRequest = {
  query: string;
};

export type TaskStatus =
  | 'draft'
  | 'running'
  | 'paused'
  | 'completed'
  | 'cancelled'
  | 'awaiting_confirmation';

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

export type UsageSummary = {
  period: string;
  requests: number;
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
