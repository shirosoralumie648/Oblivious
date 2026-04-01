import type { HttpClient } from '../../services/http/client';
import type { ConversationSummary } from '../../types/api';

export type ChatApi = {
  listConversations: () => Promise<ConversationSummary[]>;
};

export function createChatApi(client: HttpClient): ChatApi {
  return {
    listConversations: () => client.get<ConversationSummary[]>('/api/v1/app/conversations')
  };
}
