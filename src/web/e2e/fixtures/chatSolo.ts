import type { Page, Route, WebSocketRoute } from '@playwright/test';

const now = '2026-06-14T12:00:00Z';
const conversationId = 'conversation_browser_solo';
const taskId = 'task_browser_solo';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_chat_solo',
    expiresAt: '2026-06-15T12:00:00Z',
  },
  user: {
    id: 'user_chat_solo',
    email: 'chat-solo@example.com',
    name: 'Chat SOLO Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_chat_solo',
  },
};

const conversation = {
  id: conversationId,
  title: 'Browser SOLO launch thread',
  createdAt: now,
  updatedAt: now,
};

const forkConversation = {
  id: 'conversation_browser_solo_fork',
  title: 'Branch from Browser action edited launch context.',
  createdAt: now,
  updatedAt: now,
};

const researchKnowledgeBase = {
  id: 'kb_browser_research',
  name: 'Browser Research Vault',
  documentCount: 3,
  updatedAt: now,
};

const runbookKnowledgeBase = {
  id: 'kb_browser_runbooks',
  name: 'Browser Runbooks',
  documentCount: 2,
  updatedAt: now,
};

const savedConversationConfig = {
  conversationId,
  knowledgeBaseIds: [researchKnowledgeBase.id],
  maxOutputTokens: 1400,
  modelId: 'browser-chat-model',
  personaId: '',
  systemPromptOverride: 'Prefer browser SOLO handoff bullets.',
  temperature: 0.7,
  toolsEnabled: true,
  updatedAt: now,
};

const initialConversationConfig = {
  ...savedConversationConfig,
  maxOutputTokens: 900,
  systemPromptOverride: '',
  temperature: 0.2,
  toolsEnabled: false,
};

type ChatSoloMessage = {
  bookmarked?: boolean;
  content: string;
  createdAt: string;
  id: string;
  role: 'assistant' | 'user';
};

const initialMessages: ChatSoloMessage[] = [
  {
    id: 'msg_existing_browser_solo',
    role: 'assistant',
    content: 'Existing launch context from the browser journey.',
    createdAt: now,
  },
];

const streamedMessages: ChatSoloMessage[] = [
  ...initialMessages,
  {
    id: 'msg_user_browser_solo',
    role: 'user',
    content: 'Summarize the launch handoff risk.',
    createdAt: now,
  },
  {
    id: 'msg_assistant_browser_solo',
    role: 'assistant',
    content: 'Browser streamed launch handoff answer with saved settings.',
    createdAt: now,
  },
];

const forkInitialMessages: ChatSoloMessage[] = [
  {
    id: 'msg_fork_user',
    role: 'user',
    content: 'Forked browser action prompt.',
    createdAt: now,
  },
  {
    id: 'msg_fork_assistant',
    role: 'assistant',
    content: 'Forked browser action context.',
    createdAt: now,
  },
];

const forkRegeneratedMessages: ChatSoloMessage[] = [
  forkInitialMessages[0],
  {
    id: 'msg_fork_assistant',
    role: 'assistant',
    content: 'Browser action regenerated launch answer.',
    createdAt: now,
  },
];

const taskSummary = {
  id: taskId,
  title: 'Draft a launch checklist from the browser conversation.',
  goal: 'Draft a launch checklist from the browser conversation.',
  status: 'running',
  executionMode: 'standard',
  authorizationScope: 'full_access',
  budgetLimit: 20,
  budgetConsumed: 2,
  knowledgeBaseIds: [researchKnowledgeBase.id, runbookKnowledgeBase.id],
  toolAllowList: ['browser', 'shell'],
  toolDenyList: ['email'],
  createdAt: now,
  startedAt: now,
  updatedAt: now,
};

const runningTask = {
  ...taskSummary,
  currentStep: 'SOLO browser task started with Chat return context.',
  events: [
    {
      type: 'task_started',
      message: 'SOLO browser task started with Chat return context.',
      createdAt: now,
    },
  ],
  resultArtifacts: [{ label: 'Return path', value: `/chat/${conversationId}` }],
  steps: [
    { id: 'step_context', title: 'Load browser chat context', status: 'completed', stepIndex: 1, createdAt: now },
    { id: 'step_handoff', title: 'Start SOLO launch checklist', status: 'running', stepIndex: 2, createdAt: now },
  ],
};

function envelope(data: unknown) {
  return {
    ok: true,
    data,
    error: null,
  };
}

async function fulfillJSON(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(envelope(data)),
  });
}

async function fulfillError(route: Route, message: string, status = 422) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'fixture_contract_mismatch', message },
    }),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'chat solo fixture route not found' },
    }),
  });
}

function isStringArray(value: unknown, expected: string[]) {
  return (
    Array.isArray(value) &&
    value.length === expected.length &&
    expected.every((expectedValue, index) => value[index] === expectedValue)
  );
}

function configMatchesSavedPayload(payload: Record<string, unknown>) {
  return (
    isStringArray(payload.knowledgeBaseIds, savedConversationConfig.knowledgeBaseIds) &&
    payload.maxOutputTokens === savedConversationConfig.maxOutputTokens &&
    payload.modelId === savedConversationConfig.modelId &&
    payload.personaId === savedConversationConfig.personaId &&
    payload.systemPromptOverride === savedConversationConfig.systemPromptOverride &&
    payload.temperature === savedConversationConfig.temperature &&
    payload.toolsEnabled === savedConversationConfig.toolsEnabled
  );
}

function streamPayloadCarriesSavedOverrides(payload: Record<string, unknown>) {
  const overrides = payload.overrides;
  return (
    payload.content === 'Summarize the launch handoff risk.' &&
    typeof overrides === 'object' &&
    overrides !== null &&
    (overrides as Record<string, unknown>).maxOutputTokens === savedConversationConfig.maxOutputTokens &&
    (overrides as Record<string, unknown>).modelId === savedConversationConfig.modelId &&
    (overrides as Record<string, unknown>).systemPromptOverride === savedConversationConfig.systemPromptOverride &&
    (overrides as Record<string, unknown>).temperature === savedConversationConfig.temperature &&
    (overrides as Record<string, unknown>).toolsEnabled === savedConversationConfig.toolsEnabled
  );
}

function createTaskPayloadMatchesHandoff(payload: Record<string, unknown>) {
  return (
    payload.authorizationScope === 'full_access' &&
    payload.budgetLimit === 20 &&
    payload.executionMode === 'standard' &&
    payload.goal === 'Draft a launch checklist from the browser conversation.' &&
    isStringArray(payload.knowledgeBaseIds, [researchKnowledgeBase.id, runbookKnowledgeBase.id]) &&
    isStringArray(payload.toolAllowList, ['browser', 'shell']) &&
    isStringArray(payload.toolDenyList, ['email'])
  );
}

function updatedMessagePayloadMatchesAction(payload: Record<string, unknown>) {
  return payload.content === 'Browser action edited launch context.';
}

function bookmarkPayloadMatchesAction(payload: Record<string, unknown>) {
  return payload.bookmarked === true;
}

function sharePayloadMatchesAction(payload: Record<string, unknown>) {
  return payload.expiresAt === '2026-06-18T12:00:00Z';
}

function forkPayloadMatchesAction(payload: Record<string, unknown>) {
  return (
    payload.branchFromMessageId === 'msg_existing_browser_solo' &&
    payload.title === 'Branch from Browser action edited launch context.'
  );
}

function regeneratePayloadMatchesFork(payload: Record<string, unknown>) {
  const overrides = payload.overrides;
  return (
    payload.content === 'Forked browser action prompt.' &&
    typeof overrides === 'object' &&
    overrides !== null &&
    (overrides as Record<string, unknown>).maxOutputTokens === initialConversationConfig.maxOutputTokens &&
    (overrides as Record<string, unknown>).modelId === initialConversationConfig.modelId &&
    (overrides as Record<string, unknown>).systemPromptOverride === initialConversationConfig.systemPromptOverride &&
    (overrides as Record<string, unknown>).temperature === initialConversationConfig.temperature &&
    (overrides as Record<string, unknown>).toolsEnabled === initialConversationConfig.toolsEnabled
  );
}

export async function registerChatSoloRoutes(page: Page): Promise<void> {
  let currentConversationConfig = initialConversationConfig;
  let currentConversations = [conversation];
  const messagesByConversation: Record<string, ChatSoloMessage[]> = {
    [conversationId]: initialMessages,
  };
  let currentTask: typeof runningTask | null = null;

  const getMessages = (id: string) => messagesByConversation[id] ?? null;
  const setMessages = (id: string, messages: ChatSoloMessage[]) => {
    messagesByConversation[id] = messages;
  };
  const replaceMessage = (id: string, messageId: string, replacement: ChatSoloMessage) => {
    const messages = getMessages(id);
    if (messages === null) {
      return null;
    }
    setMessages(
      id,
      messages.map((message) => (message.id === messageId ? replacement : message))
    );
    return replacement;
  };

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/models') {
      await fulfillJSON(route, [{ id: 'browser-chat-model', label: 'Browser Chat Model' }]);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/personas') {
      await fulfillJSON(route, []);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/knowledge-bases') {
      await fulfillJSON(route, [researchKnowledgeBase, runbookKnowledgeBase]);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/conversations') {
      await fulfillJSON(route, currentConversations);
      return;
    }

    const conversationMatch = pathname.match(/^\/api\/v1\/app\/conversations\/([^/]+)(?:\/(.*))?$/);
    const requestedConversationId = conversationMatch?.[1];
    const conversationSuffix = conversationMatch?.[2] ?? '';

    if (method === 'GET' && requestedConversationId && conversationSuffix === 'messages') {
      const messages = getMessages(requestedConversationId);
      if (messages === null) {
        await fulfillNotFound(route);
        return;
      }
      await fulfillJSON(route, messages);
      return;
    }

    if (method === 'GET' && requestedConversationId && conversationSuffix === 'config') {
      if (requestedConversationId !== conversationId && requestedConversationId !== forkConversation.id) {
        await fulfillNotFound(route);
        return;
      }
      await fulfillJSON(route, { ...currentConversationConfig, conversationId: requestedConversationId });
      return;
    }

    if (method === 'PUT' && pathname === `/api/v1/app/conversations/${conversationId}/config`) {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!configMatchesSavedPayload(body)) {
        await fulfillError(route, 'chat settings payload did not carry the expected browser handoff configuration');
        return;
      }

      currentConversationConfig = savedConversationConfig;
      await fulfillJSON(route, currentConversationConfig);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/conversations/${conversationId}/messages/stream`) {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!streamPayloadCarriesSavedOverrides(body)) {
        await fulfillError(route, 'stream payload did not include the saved conversation overrides');
        return;
      }

      setMessages(conversationId, streamedMessages);
      await route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: 'data: Browser streamed launch handoff answer with saved settings.\n\ndata: [DONE]\n\n',
      });
      return;
    }

    if (method === 'POST' && requestedConversationId === forkConversation.id && conversationSuffix === 'messages') {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!regeneratePayloadMatchesFork(body)) {
        await fulfillError(route, 'fork regenerate payload did not retry the previous user message with saved overrides');
        return;
      }

      setMessages(forkConversation.id, forkRegeneratedMessages);
      await fulfillJSON(route, forkRegeneratedMessages);
      return;
    }

    const messageMatch = pathname.match(/^\/api\/v1\/app\/conversations\/([^/]+)\/messages\/([^/]+)(?:\/(.*))?$/);
    const messageConversationId = messageMatch?.[1];
    const messageId = messageMatch?.[2];
    const messageSuffix = messageMatch?.[3] ?? '';

    if (method === 'PUT' && messageConversationId && messageId && messageSuffix === '') {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (messageConversationId !== conversationId || messageId !== 'msg_existing_browser_solo') {
        await fulfillNotFound(route);
        return;
      }
      if (!updatedMessagePayloadMatchesAction(body)) {
        await fulfillError(route, 'message edit payload did not carry the expected browser action content');
        return;
      }

      const updatedMessage = replaceMessage(messageConversationId, messageId, {
        id: messageId,
        role: 'assistant',
        content: 'Browser action edited launch context.',
        createdAt: now,
      });
      await fulfillJSON(route, updatedMessage);
      return;
    }

    if (method === 'POST' && messageConversationId === conversationId && messageId === 'msg_existing_browser_solo' && messageSuffix === 'bookmark') {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!bookmarkPayloadMatchesAction(body)) {
        await fulfillError(route, 'bookmark payload did not mark the message as bookmarked');
        return;
      }

      const updatedMessage = replaceMessage(messageConversationId, messageId, {
        id: messageId,
        role: 'assistant',
        content: 'Browser action edited launch context.',
        createdAt: now,
        bookmarked: true,
      });
      await fulfillJSON(route, updatedMessage);
      return;
    }

    if (method === 'POST' && messageConversationId === conversationId && messageId === 'msg_existing_browser_solo' && messageSuffix === 'share') {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!sharePayloadMatchesAction(body)) {
        await fulfillError(route, 'message share payload did not carry the expected expiration');
        return;
      }

      await fulfillJSON(route, {
        id: 'share_message_action',
        url: 'https://share.example.test/message_action',
      }, 201);
      return;
    }

    if (method === 'DELETE' && messageConversationId && messageId && messageSuffix === '') {
      const messages = getMessages(messageConversationId);
      if (messages === null) {
        await fulfillNotFound(route);
        return;
      }

      setMessages(
        messageConversationId,
        messages.filter((message) => message.id !== messageId)
      );
      await fulfillJSON(route, { deleted: true });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/conversations/${conversationId}/convert-to-task`) {
      await fulfillJSON(route, {
        draftTaskGoal: 'Draft a launch checklist from the browser conversation.',
        relatedKnowledgeBaseIds: [researchKnowledgeBase.id],
        suggestedBudget: 20,
        suggestedExecutionMode: 'standard',
      });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/conversations/${conversationId}/fork`) {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!forkPayloadMatchesAction(body)) {
        await fulfillError(route, 'fork payload did not preserve the selected browser action message');
        return;
      }

      currentConversations = [conversation, forkConversation];
      setMessages(forkConversation.id, forkInitialMessages);
      await fulfillJSON(route, forkConversation);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/tasks') {
      await fulfillJSON(route, currentTask === null ? [] : [currentTask]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/app/tasks') {
      const body = (await request.postDataJSON()) as Record<string, unknown>;
      if (!createTaskPayloadMatchesHandoff(body)) {
        await fulfillError(route, 'SOLO create payload did not carry the expected handoff boundaries');
        return;
      }

      currentTask = runningTask;
      await fulfillJSON(route, taskSummary, 201);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/app/tasks/${taskId}/start`) {
      currentTask = runningTask;
      await fulfillJSON(route, runningTask);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/tasks/${taskId}`) {
      await fulfillJSON(route, currentTask ?? runningTask);
      return;
    }

    await fulfillNotFound(route);
  });
}

type SentRealtimeMessage = {
  conversationId?: string;
  isTyping?: boolean;
  type?: string;
};

type ChatSoloRealtime = {
  emit: (event: unknown) => void;
  sentMessages: SentRealtimeMessage[];
  violations: string[];
};

export async function registerChatSoloRealtime(page: Page): Promise<ChatSoloRealtime> {
  const sentMessages: SentRealtimeMessage[] = [];
  const violations: string[] = [];
  let activeSocket: WebSocketRoute | null = null;

  await page.routeWebSocket(/.*/, async (socket) => {
    const url = new URL(socket.url());
    if (url.pathname !== '/api/v1/ws') {
      violations.push(`unexpected websocket url: ${socket.url()}`);
      await socket.close({ code: 1002, reason: 'Unexpected realtime path' });
      return;
    }

    activeSocket = socket;
    socket.onClose(() => {
      if (activeSocket === socket) {
        activeSocket = null;
      }
    });
    socket.onMessage((message) => {
      if (typeof message !== 'string') {
        violations.push('non-string websocket payload');
        return;
      }

      let payload: SentRealtimeMessage;
      try {
        payload = JSON.parse(message) as SentRealtimeMessage;
      } catch {
        violations.push(`non-json websocket payload: ${message}`);
        return;
      }

      if (payload.conversationId !== conversationId) {
        violations.push(`unexpected websocket conversation: ${String(payload.conversationId)}`);
      }
      if (!['chat_join', 'chat_leave', 'chat_typing'].includes(String(payload.type))) {
        violations.push(`unexpected websocket client type: ${String(payload.type)}`);
      }
      if (payload.type === 'chat_typing' && typeof payload.isTyping !== 'boolean') {
        violations.push('chat_typing payload missing boolean isTyping');
      }

      sentMessages.push(payload);
    });
  });

  return {
    emit: (event: unknown) => {
      if (activeSocket === null) {
        violations.push('server event emitted before websocket connection');
        return;
      }
      activeSocket.send(JSON.stringify(event));
    },
    sentMessages,
    violations,
  };
}
