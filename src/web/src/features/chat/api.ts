import type { HttpClient } from '../../services/http/client';
import { streamText } from '../../services/http/stream';
import type {
  ConversationConfig,
  ConversationMessage,
  ConversationShareResponse,
  ConversationSummary,
  ConvertConversationToTaskResponse,
  CreateConversationRequest,
  BookmarkConversationMessageRequest,
  ForkConversationRequest,
  MessageShareResponse,
  ModelOption,
  PersonaRequest,
  PersonaSummary,
  SendMessageRequest,
  UpdateConversationMessageRequest,
  UpdateConversationConfigRequest
} from '../../types/api';

export type CreateMessageShareOptions = {
  expiresAt?: string;
};

export type CreateConversationShareOptions = {
  endMessageId?: string;
  expiresAt?: string;
  startMessageId?: string;
};

export type ChatApi = {
  createConversation: (payload: CreateConversationRequest) => Promise<ConversationSummary>;
  createPersona: (payload: PersonaRequest) => Promise<PersonaSummary>;
  createConversationShare: (
    conversationId: string,
    options?: CreateConversationShareOptions
  ) => Promise<ConversationShareResponse>;
  createMessageShare: (
    conversationId: string,
    messageId: string,
    options?: CreateMessageShareOptions
  ) => Promise<MessageShareResponse>;
  convertConversationToTask: (conversationId: string) => Promise<ConvertConversationToTaskResponse>;
  deleteConversation: (conversationId: string) => Promise<void>;
  deleteMessage: (conversationId: string, messageId: string) => Promise<void>;
  deletePersona: (personaId: string) => Promise<void>;
  exportConversationMarkdown: (conversationId: string) => Promise<string>;
  forkConversation: (conversationId: string, payload: ForkConversationRequest) => Promise<ConversationSummary>;
  bookmarkMessage: (
    conversationId: string,
    messageId: string,
    payload: BookmarkConversationMessageRequest
  ) => Promise<ConversationMessage>;
  getConversation: (conversationId: string) => Promise<ConversationSummary>;
  getConversationConfig: (conversationId: string) => Promise<ConversationConfig>;
  listConversations: () => Promise<ConversationSummary[]>;
  listMessages: (conversationId: string) => Promise<ConversationMessage[]>;
  listModels: () => Promise<ModelOption[]>;
  listPersonas: () => Promise<PersonaSummary[]>;
  updatePersona: (personaId: string, payload: PersonaRequest) => Promise<PersonaSummary>;
  sendMessage: (conversationId: string, payload: SendMessageRequest) => Promise<ConversationMessage[]>;
  sendMessageStream: (
    conversationId: string,
    payload: SendMessageRequest,
    handlers: { onChunk: (chunk: string) => void; signal?: AbortSignal }
  ) => Promise<void>;
  updateMessage: (
    conversationId: string,
    messageId: string,
    payload: UpdateConversationMessageRequest
  ) => Promise<ConversationMessage>;
  updateConversation: (conversationId: string, payload: CreateConversationRequest) => Promise<ConversationSummary>;
  updateConversationConfig: (conversationId: string, payload: UpdateConversationConfigRequest) => Promise<ConversationConfig>;
};

export type ChatApiOptions = {
  fetchFn?: typeof fetch;
};

type RawPersonaSummary = Omit<PersonaSummary, 'suggestedQuestions'> & {
  suggestedQuestions?: string[] | string;
};

export function createChatApi(client: HttpClient, options: ChatApiOptions = {}): ChatApi {
  const fetchFn = options.fetchFn ?? fetch;
  const messagePath = (conversationId: string, messageId: string) =>
    `/api/v1/app/conversations/${conversationId}/messages/${messageId}`;
  const normalizeMessageShare = (share: MessageShareResponse): MessageShareResponse => ({
    ...share,
    id: share.id ?? share.shareId,
    url: share.url ?? share.shareUrl
  });
  const normalizeConversationShare = (share: ConversationShareResponse): ConversationShareResponse => ({
    ...share,
    id: share.id ?? share.shareId,
    url: share.url ?? share.shareUrl
  });
  const normalizeConversationSummary = (conversation: ConversationSummary): ConversationSummary => ({
    ...conversation,
    parentId: conversation.parentId ?? conversation.parent_id
  });
  const normalizePersona = (persona: RawPersonaSummary): PersonaSummary => ({
    ...persona,
    suggestedQuestions:
      typeof persona.suggestedQuestions === 'string'
        ? persona.suggestedQuestions
            .split('\n')
            .map((question) => question.trim())
            .filter(Boolean)
        : persona.suggestedQuestions
  });

  return {
    createConversation: (payload) => client.post<ConversationSummary>('/api/v1/app/conversations', payload),
    createPersona: async (payload) => normalizePersona(await client.post<RawPersonaSummary>('/api/v1/app/personas', payload)),
    createConversationShare: async (conversationId, options) => {
      const path = `/api/v1/app/conversations/${conversationId}/share`;
      const share =
        options === undefined
          ? await client.post<ConversationShareResponse>(path)
          : await client.post<ConversationShareResponse>(path, options);
      return normalizeConversationShare(share);
    },
    createMessageShare: async (conversationId, messageId, options) => {
      const path = `${messagePath(conversationId, messageId)}/share`;
      const share =
        options === undefined
          ? await client.post<MessageShareResponse>(path)
          : await client.post<MessageShareResponse>(path, options);
      return normalizeMessageShare(share);
    },
    convertConversationToTask: (conversationId) =>
      client.post<ConvertConversationToTaskResponse>(`/api/v1/app/conversations/${conversationId}/convert-to-task`),
    deleteConversation: (conversationId) => client.delete<void>(`/api/v1/app/conversations/${conversationId}`),
    deleteMessage: (conversationId, messageId) => client.delete<void>(messagePath(conversationId, messageId)),
    deletePersona: async (personaId) => {
      await client.delete<unknown>(`/api/v1/app/personas/${encodeURIComponent(personaId)}`);
    },
    exportConversationMarkdown: (conversationId) =>
      client.get<string>(`/api/v1/app/conversations/${encodeURIComponent(conversationId)}/export.md`),
    forkConversation: async (conversationId, payload) =>
      normalizeConversationSummary(await client.post<ConversationSummary>(`/api/v1/app/conversations/${conversationId}/fork`, payload)),
    bookmarkMessage: (conversationId, messageId, payload) =>
      client.post<ConversationMessage>(`${messagePath(conversationId, messageId)}/bookmark`, payload),
    getConversation: async (conversationId) =>
      normalizeConversationSummary(await client.get<ConversationSummary>(`/api/v1/app/conversations/${conversationId}`)),
    getConversationConfig: (conversationId) =>
      client.get<ConversationConfig>(`/api/v1/app/conversations/${conversationId}/config`),
    listConversations: () => client.get<ConversationSummary[]>('/api/v1/app/conversations'),
    listMessages: (conversationId) => client.get<ConversationMessage[]>(`/api/v1/app/conversations/${conversationId}/messages`),
    listModels: () => client.get<ModelOption[]>('/api/v1/app/models'),
    listPersonas: async () => (await client.get<RawPersonaSummary[]>('/api/v1/app/personas')).map(normalizePersona),
    sendMessage: (conversationId, payload) =>
      client.post<ConversationMessage[]>(`/api/v1/app/conversations/${conversationId}/messages`, payload),
    sendMessageStream: (conversationId, payload, handlers) =>
      streamText(
        `/api/v1/app/conversations/${conversationId}/messages/stream`,
        handlers.onChunk,
        fetchFn,
        {
          body: JSON.stringify(payload),
          headers: {
            Accept: 'text/event-stream',
            'Content-Type': 'application/json'
          },
          method: 'POST',
          signal: handlers.signal
        }
      ),
    updateMessage: (conversationId, messageId, payload) => client.put<ConversationMessage>(messagePath(conversationId, messageId), payload),
    updateConversation: async (conversationId, payload) =>
      normalizeConversationSummary(await client.put<ConversationSummary>(`/api/v1/app/conversations/${conversationId}`, payload)),
    updateConversationConfig: (conversationId, payload) =>
      client.put<ConversationConfig>(`/api/v1/app/conversations/${conversationId}/config`, payload),
    updatePersona: async (personaId, payload) =>
      normalizePersona(await client.put<RawPersonaSummary>(`/api/v1/app/personas/${encodeURIComponent(personaId)}`, payload))
  };
}
