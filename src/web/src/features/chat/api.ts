import {
  bookmarkMessageOperationContract,
  convertConversationToTaskOperationContract,
  connectWorkspaceWebSocketOperationContract,
  createConversationOperationContract,
  createConversationShareOperationContract,
  createMessageShareOperationContract,
  createPersonaOperationContract,
  deleteConversationOperationContract,
  deleteMessageOperationContract,
  deletePersonaOperationContract,
  exportConversationMarkdownOperationContract,
  forkConversationOperationContract,
  getConversationConfigOperationContract,
  getConversationOperationContract,
  listConversationsOperationContract,
  listMessagesOperationContract,
  listModelsOperationContract,
  listPersonasOperationContract,
  sendMessageOperationContract,
  streamMessageOperationContract,
  updateConversationConfigOperationContract,
  updateConversationOperationContract,
  updateMessageOperationContract,
  updatePersonaOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';

import {
  noneRequestEncoder,
  jsonRequestEncoder,
  jsonEnvelopeDecoder,
  rawResponseDecoder,
  textResponseDecoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';
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

export type ChatRealtimePayload = {
  conversationId?: string;
  isTyping?: boolean;
  message?: ConversationMessage;
  messageId?: string;
  messages?: ConversationMessage[];
  userId?: string;
};

export type ChatRealtimeEvent = {
  category?: string;
  payload?: ChatRealtimePayload;
  timestamp?: string;
  type: 'chat_messages_synced' | 'chat_message_deleted' | 'chat_message_updated' | 'chat_typing' | string;
};

export type ConversationRealtimeHandlers = {
  onEvent: (event: ChatRealtimeEvent) => void;
};

export type ConversationRealtimeSocket = {
  close: () => void;
  sendTyping: (isTyping: boolean) => void;
};

type RawPersonaSummary = Omit<PersonaSummary, 'suggestedQuestions'> & {
  suggestedQuestions?: string[] | string;
};

function jsonTransport<T>(operation: OperationContractMetadataV1, status = 200): OperationTransportContract<T> {
  return {
    operation,
    requestEncoder: operation.request.mediaType === null ? noneRequestEncoder(operation) : jsonRequestEncoder(operation),
    responseDecoder: jsonEnvelopeDecoder<T>(operation, status)
  };
}

const bookmarkMessageTransport = jsonTransport<ConversationMessage>(bookmarkMessageOperationContract);
const convertConversationToTaskTransport = jsonTransport<ConvertConversationToTaskResponse>(convertConversationToTaskOperationContract);
const createConversationTransport = jsonTransport<ConversationSummary>(createConversationOperationContract);
const createConversationShareTransport = jsonTransport<ConversationShareResponse>(createConversationShareOperationContract, 201);
const createMessageShareTransport = jsonTransport<MessageShareResponse>(createMessageShareOperationContract, 201);
const createPersonaTransport = jsonTransport<RawPersonaSummary>(createPersonaOperationContract);
const deleteConversationTransport = jsonTransport<void>(deleteConversationOperationContract);
const deleteMessageTransport = jsonTransport<void>(deleteMessageOperationContract);
const deletePersonaTransport = jsonTransport<unknown>(deletePersonaOperationContract);
const forkConversationTransport = jsonTransport<ConversationSummary>(forkConversationOperationContract);
const getConversationConfigTransport = jsonTransport<ConversationConfig>(getConversationConfigOperationContract);
const getConversationTransport = jsonTransport<ConversationSummary>(getConversationOperationContract);
const listConversationsTransport = jsonTransport<ConversationSummary[]>(listConversationsOperationContract);
const listMessagesTransport = jsonTransport<ConversationMessage[]>(listMessagesOperationContract);
const listModelsTransport = jsonTransport<ModelOption[]>(listModelsOperationContract);
const listPersonasTransport = jsonTransport<RawPersonaSummary[]>(listPersonasOperationContract);
const sendMessageTransport = jsonTransport<ConversationMessage[]>(sendMessageOperationContract);
const updateConversationConfigTransport = jsonTransport<ConversationConfig>(updateConversationConfigOperationContract);
const updateConversationTransport = jsonTransport<ConversationSummary>(updateConversationOperationContract);
const updateMessageTransport = jsonTransport<ConversationMessage>(updateMessageOperationContract);
const updatePersonaTransport = jsonTransport<RawPersonaSummary>(updatePersonaOperationContract);

const exportConversationMarkdownTransport: OperationTransportContract<string> = {
  operation: exportConversationMarkdownOperationContract,
  requestEncoder: noneRequestEncoder(exportConversationMarkdownOperationContract),
  responseDecoder: textResponseDecoder(exportConversationMarkdownOperationContract, 200)
};

const streamMessageTransport: OperationTransportContract<Response> = {
  operation: streamMessageOperationContract,
  requestEncoder: jsonRequestEncoder(streamMessageOperationContract),
  responseDecoder: rawResponseDecoder(streamMessageOperationContract, 200)
};

function websocketURL(path: string) {
  if (typeof window === 'undefined' || !window.location?.host) {
    return path;
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${protocol}//${window.location.host}${path}`;
}

function noopConversationRealtimeSocket(): ConversationRealtimeSocket {
  return {
    close: () => undefined,
    sendTyping: () => undefined
  };
}

export function createConversationRealtimeSocket(
  conversationId: string,
  handlers: ConversationRealtimeHandlers
): ConversationRealtimeSocket {
  const normalizedConversationId = conversationId.trim();
  if (normalizedConversationId === '' || typeof WebSocket === 'undefined') {
    return noopConversationRealtimeSocket();
  }

  const socket = new WebSocket(websocketURL(connectWorkspaceWebSocketOperationContract.normalizedPath));
  const sendClientMessage = (message: { conversationId: string; isTyping?: boolean; type: string }) => {
    if (socket.readyState !== WebSocket.OPEN) {
      return;
    }
    socket.send(JSON.stringify(message));
  };

  socket.onopen = () => {
    sendClientMessage({ conversationId: normalizedConversationId, type: 'chat_join' });
  };
  socket.onmessage = (event) => {
    if (typeof event.data !== 'string') {
      return;
    }
    event.data
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
      .forEach((line) => {
        try {
          handlers.onEvent(JSON.parse(line) as ChatRealtimeEvent);
        } catch {
          // Ignore malformed frames so one bad realtime payload does not tear down the chat page.
        }
      });
  };

  return {
    close: () => {
      sendClientMessage({ conversationId: normalizedConversationId, type: 'chat_leave' });
      socket.close();
    },
    sendTyping: (isTyping) => {
      sendClientMessage({ conversationId: normalizedConversationId, isTyping, type: 'chat_typing' });
    }
  };
}

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
    createConversation: (payload) =>
      client.post<ConversationSummary>('/api/v1/app/conversations', payload, undefined, createConversationTransport),
    createPersona: async (payload) =>
      normalizePersona(await client.post<RawPersonaSummary>('/api/v1/app/personas', payload, undefined, createPersonaTransport)),
    createConversationShare: async (conversationId, options) => {
      const path = `/api/v1/app/conversations/${conversationId}/share`;
      const share =
        options === undefined
          ? await client.post<ConversationShareResponse>(path, undefined, undefined, createConversationShareTransport)
          : await client.post<ConversationShareResponse>(path, options, undefined, createConversationShareTransport);
      return normalizeConversationShare(share);
    },
    createMessageShare: async (conversationId, messageId, options) => {
      const path = `${messagePath(conversationId, messageId)}/share`;
      const share =
        options === undefined
          ? await client.post<MessageShareResponse>(path, undefined, undefined, createMessageShareTransport)
          : await client.post<MessageShareResponse>(path, options, undefined, createMessageShareTransport);
      return normalizeMessageShare(share);
    },
    convertConversationToTask: (conversationId) =>
      client.post<ConvertConversationToTaskResponse>(
        `/api/v1/app/conversations/${conversationId}/convert-to-task`,
        undefined,
        undefined,
        convertConversationToTaskTransport
      ),
    deleteConversation: (conversationId) =>
      client.delete<void>(`/api/v1/app/conversations/${conversationId}`, undefined, deleteConversationTransport),
    deleteMessage: (conversationId, messageId) =>
      client.delete<void>(messagePath(conversationId, messageId), undefined, deleteMessageTransport),
    deletePersona: async (personaId) => {
      await client.delete<unknown>(
        `/api/v1/app/personas/${encodeURIComponent(personaId)}`,
        undefined,
        deletePersonaTransport
      );
    },
    exportConversationMarkdown: (conversationId) =>
      client.get<string>(
        `/api/v1/app/conversations/${encodeURIComponent(conversationId)}/export.md`,
        undefined,
        exportConversationMarkdownTransport
      ),
    forkConversation: async (conversationId, payload) => {
      const { branchFromMessageId, messageId, ...rest } = payload;
      return normalizeConversationSummary(
        await client.post<ConversationSummary>(
          `/api/v1/app/conversations/${conversationId}/fork`,
          {
            ...rest,
            branchFromMessageId: branchFromMessageId ?? messageId
          },
          undefined,
          forkConversationTransport
        )
      );
    },
    bookmarkMessage: (conversationId, messageId, payload) =>
      client.post<ConversationMessage>(
        `${messagePath(conversationId, messageId)}/bookmark`,
        payload,
        undefined,
        bookmarkMessageTransport
      ),
    getConversation: async (conversationId) =>
      normalizeConversationSummary(
        await client.get<ConversationSummary>(
          `/api/v1/app/conversations/${conversationId}`,
          undefined,
          getConversationTransport
        )
      ),
    getConversationConfig: (conversationId) =>
      client.get<ConversationConfig>(
        `/api/v1/app/conversations/${conversationId}/config`,
        undefined,
        getConversationConfigTransport
      ),
    listConversations: () =>
      client.get<ConversationSummary[]>('/api/v1/app/conversations', undefined, listConversationsTransport),
    listMessages: (conversationId) =>
      client.get<ConversationMessage[]>(
        `/api/v1/app/conversations/${conversationId}/messages`,
        undefined,
        listMessagesTransport
      ),
    listModels: () => client.get<ModelOption[]>('/api/v1/app/models', undefined, listModelsTransport),
    listPersonas: async () =>
      (await client.get<RawPersonaSummary[]>('/api/v1/app/personas', undefined, listPersonasTransport)).map(normalizePersona),
    sendMessage: (conversationId, payload) =>
      client.post<ConversationMessage[]>(
        `/api/v1/app/conversations/${conversationId}/messages`,
        payload,
        undefined,
        sendMessageTransport
      ),
    sendMessageStream: (conversationId, payload, handlers) =>
      streamText(
        `/api/v1/app/conversations/${conversationId}/messages/stream`,
        handlers.onChunk,
        streamMessageOperationContract,
        streamMessageTransport,
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
    updateMessage: (conversationId, messageId, payload) =>
      client.put<ConversationMessage>(messagePath(conversationId, messageId), payload, undefined, updateMessageTransport),
    updateConversation: async (conversationId, payload) =>
      normalizeConversationSummary(
        await client.put<ConversationSummary>(
          `/api/v1/app/conversations/${conversationId}`,
          payload,
          undefined,
          updateConversationTransport
        )
      ),
    updateConversationConfig: (conversationId, payload) =>
      client.put<ConversationConfig>(
        `/api/v1/app/conversations/${conversationId}/config`,
        payload,
        undefined,
        updateConversationConfigTransport
      ),
    updatePersona: async (personaId, payload) =>
      normalizePersona(
        await client.put<RawPersonaSummary>(
          `/api/v1/app/personas/${encodeURIComponent(personaId)}`,
          payload,
          undefined,
          updatePersonaTransport
        )
      )
  };
}
