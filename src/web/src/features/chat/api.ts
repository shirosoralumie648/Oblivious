import type { HttpClient } from '../../services/http/client';
import type {
  ConversationConfig,
  ConversationMessage,
  ConversationSummary,
  ConvertConversationToTaskResponse,
  CreateConversationRequest,
  ModelOption,
  SendMessageRequest,
  UpdateConversationConfigRequest
} from '../../types/api';

export type ChatApi = {
  createConversation: (payload: CreateConversationRequest) => Promise<ConversationSummary>;
  convertConversationToTask: (conversationId: string) => Promise<ConvertConversationToTaskResponse>;
  getConversationConfig: (conversationId: string) => Promise<ConversationConfig>;
  listConversations: () => Promise<ConversationSummary[]>;
  listMessages: (conversationId: string) => Promise<ConversationMessage[]>;
  listModels: () => Promise<ModelOption[]>;
  sendMessage: (conversationId: string, payload: SendMessageRequest) => Promise<ConversationMessage[]>;
  updateConversationConfig: (conversationId: string, payload: UpdateConversationConfigRequest) => Promise<ConversationConfig>;
};

export function createChatApi(client: HttpClient): ChatApi {
  return {
    createConversation: (payload) => client.post<ConversationSummary>('/api/v1/app/conversations', payload),
    convertConversationToTask: (conversationId) =>
      client.post<ConvertConversationToTaskResponse>(`/api/v1/app/conversations/${conversationId}/convert-to-task`),
    getConversationConfig: (conversationId) =>
      client.get<ConversationConfig>(`/api/v1/app/conversations/${conversationId}/config`),
    listConversations: () => client.get<ConversationSummary[]>('/api/v1/app/conversations'),
    listMessages: (conversationId) => client.get<ConversationMessage[]>(`/api/v1/app/conversations/${conversationId}/messages`),
    listModels: () => client.get<ModelOption[]>('/api/v1/app/models'),
    sendMessage: (conversationId, payload) =>
      client.post<ConversationMessage[]>(`/api/v1/app/conversations/${conversationId}/messages`, payload),
    updateConversationConfig: (conversationId, payload) =>
      client.put<ConversationConfig>(`/api/v1/app/conversations/${conversationId}/config`, payload)
  };
}
