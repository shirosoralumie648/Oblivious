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
