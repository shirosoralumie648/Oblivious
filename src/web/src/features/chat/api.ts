import type { HttpClient } from '../../services/http/client';
import type {
  ChatMessage,
  ConversationConfig,
  ConversationTaskDraft,
  ConversationSummary,
  CreateConversationRequest,
  ModelOption,
  SendMessageRequest,
  UpdateConversationConfigRequest
} from '../../types/api';

export interface ChatApi {
  convertConversationToTask: (conversationId: string) => Promise<ConversationTaskDraft>;
  createConversation: (payload?: CreateConversationRequest) => Promise<ConversationSummary>;
  getConversationConfig: (conversationId: string) => Promise<ConversationConfig>;
  listConversations: () => Promise<ConversationSummary[]>;
  listMessages: (conversationId: string) => Promise<ChatMessage[]>;
  listModels: () => Promise<ModelOption[]>;
  sendMessage: (conversationId: string, payload: SendMessageRequest) => Promise<ChatMessage[]>;
  updateConversationConfig: (conversationId: string, payload: UpdateConversationConfigRequest) => Promise<ConversationConfig>;
}

export function createChatApi(client: HttpClient): ChatApi {
  return {
    convertConversationToTask: (conversationId) =>
      client.post<ConversationTaskDraft>(`/api/v1/app/conversations/${conversationId}/convert-to-task`),
    createConversation: (payload) => client.post<ConversationSummary>('/api/v1/app/conversations', payload),
    getConversationConfig: (conversationId) => client.get<ConversationConfig>(`/api/v1/app/conversations/${conversationId}/config`),
    listConversations: () => client.get<ConversationSummary[]>('/api/v1/app/conversations'),
    listMessages: (conversationId) => client.get<ChatMessage[]>(`/api/v1/app/conversations/${conversationId}/messages`),
    listModels: () => client.get<ModelOption[]>('/api/v1/app/models'),
    sendMessage: (conversationId, payload) =>
      client.post<ChatMessage[]>(`/api/v1/app/conversations/${conversationId}/messages`, payload),
    updateConversationConfig: (conversationId, payload) =>
      client.put<ConversationConfig>(`/api/v1/app/conversations/${conversationId}/config`, payload)
  };
}
