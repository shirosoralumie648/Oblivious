export interface ApiEnvelope<T> {
  data: T | null;
  error: ApiError | null;
  ok: boolean;
}

export interface ApiError {
  code: string;
  message: string;
}

export interface ApiUser {
  email: string;
  id: string;
}

export interface UserPreferences {
  defaultMode: 'chat' | 'solo';
  modelStrategy: string;
  networkEnabledHint: boolean;
  onboardingCompleted: boolean;
}

export interface SessionPayload {
  expiresAt: string;
  id: string;
}

export interface WorkspacePayload {
  id: string;
}

export interface SessionResponse {
  onboardingCompleted: boolean;
  preferences: UserPreferences;
  session: SessionPayload;
  user: ApiUser;
  workspace: WorkspacePayload;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
}

export interface ConversationConfig {
  conversationId: string;
  knowledgeBaseIds: string[];
  maxOutputTokens: number;
  modelId: string;
  systemPromptOverride?: string;
  temperature: number;
  toolsEnabled: boolean;
  updatedAt?: string;
}

export interface ModelOption {
  id: string;
  label: string;
}

export interface UpdateConversationConfigRequest {
  knowledgeBaseIds: string[];
  maxOutputTokens: number;
  modelId: string;
  systemPromptOverride: string;
  temperature: number;
  toolsEnabled: boolean;
}

export interface ConversationSummary {
  createdAt?: string;
  id: string;
  title: string;
  updatedAt?: string;
}

export interface ConversationTaskDraft {
  draftTaskGoal: string;
  relatedKnowledgeBaseIds: string[];
  suggestedBudget: number;
  suggestedExecutionMode: string;
}

export interface TaskSummary {
  authorizationScope: string;
  budgetConsumed?: number;
  budgetLimit: number;
  createdAt?: string;
  executionMode: string;
  finishedAt?: string;
  goal: string;
  id: string;
  resultSummary?: string;
  startedAt?: string;
  status: string;
  title: string;
  updatedAt?: string;
}

export interface TaskStep {
  createdAt?: string;
  finishedAt?: string;
  id: string;
  startedAt?: string;
  status: string;
  stepIndex: number;
  title: string;
  updatedAt?: string;
}

export interface TaskDetail extends TaskSummary {
  knowledgeBaseIds: string[];
  steps: TaskStep[];
  toolAllowList?: string[];
  toolDenyList?: string[];
}

export interface CreateTaskRequest {
  authorizationScope: string;
  budgetLimit: number;
  executionMode: string;
  goal: string;
  knowledgeBaseIds: string[];
  toolAllowList?: string[];
  toolDenyList?: string[];
  title?: string;
}

export interface UpdateTaskBudgetRequest {
  budgetLimit: number;
}

export interface ChatMessage {
  content: string;
  createdAt?: string;
  id: string;
  role: 'assistant' | 'user';
}

export interface CreateConversationRequest {
  title?: string;
}

export interface KnowledgeBaseSummary {
  documentCount: number;
  id: string;
  name: string;
  updatedAt?: string;
}

export interface CreateKnowledgeBaseRequest {
  name: string;
}

export interface UpdateKnowledgeBaseRequest {
  name: string;
}

export interface KnowledgeDocumentSummary {
  content: string;
  id: string;
  title: string;
  updatedAt?: string;
}

export interface CreateKnowledgeDocumentRequest {
  content: string;
  title: string;
}

export interface UpdateKnowledgeDocumentRequest {
  content: string;
  title: string;
}

export interface MessageOverrides {
  maxOutputTokens?: number;
  modelId?: string;
  systemPromptOverride?: string;
  temperature?: number;
  toolsEnabled?: boolean;
}

export interface SendMessageRequest {
  content: string;
  overrides?: MessageOverrides;
}

export interface UsageSummary {
  period: string;
  requests: number;
}

export interface ConsoleModelSummary {
  id: string;
  label: string;
  requests: number;
}

export interface BillingSummary {
  period: string;
  requests: number;
  inputTokens: number;
  outputTokens: number;
  estimatedCostUsd: number;
}

export interface ConsoleAccessSummary {
  defaultMode: string;
  modelStrategy: string;
  networkEnabledHint: boolean;
  onboardingCompleted: boolean;
  sessionExpiresAt: string;
  sessionId: string;
  userEmail: string;
  userId: string;
  workspaceId: string;
}
