import type { Page, Route } from '@playwright/test';

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

const initialMessages = [
  {
    id: 'msg_existing_browser_solo',
    role: 'assistant',
    content: 'Existing launch context from the browser journey.',
    createdAt: now,
  },
];

const streamedMessages = [
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

export async function registerChatSoloRoutes(page: Page): Promise<void> {
  let currentConversationConfig = initialConversationConfig;
  let currentMessages = initialMessages;
  let currentTask: typeof runningTask | null = null;

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
      await fulfillJSON(route, [conversation]);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/conversations/${conversationId}/messages`) {
      await fulfillJSON(route, currentMessages);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/conversations/${conversationId}/config`) {
      await fulfillJSON(route, currentConversationConfig);
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

      currentMessages = streamedMessages;
      await route.fulfill({
        status: 200,
        contentType: 'text/event-stream',
        body: 'data: Browser streamed launch handoff answer with saved settings.\n\ndata: [DONE]\n\n',
      });
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
