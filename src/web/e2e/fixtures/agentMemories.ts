import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T13:00:00Z';
const agentId = 'agent_browser_memory';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_agent_memories',
    expiresAt: '2026-06-16T13:00:00Z',
  },
  user: {
    id: 'user_agent_memory_operator',
    email: 'memory-operator@example.com',
    name: 'Memory Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_agent_memories',
  },
};

const searchedMemory = {
  id: 'memory_release_preference',
  userId: session.user.id,
  agentId,
  type: 'long_term',
  content: 'Prefer concise rollout notes.',
  importance: 4,
  metadata: { source: 'workflow', topic: 'release' },
  createdAt: now,
  updatedAt: now,
};

const createdMemory = {
  id: 'memory_created_browser',
  userId: session.user.id,
  agentId,
  type: 'user_managed',
  content: 'Always include rollback notes.',
  importance: 5,
  metadata: { managedBy: 'workspace' },
  createdAt: now,
  updatedAt: now,
};

const updatedMemory = {
  ...createdMemory,
  content: 'Always include rollback notes and owner evidence.',
  importance: 4,
  updatedAt: '2026-06-15T13:05:00Z',
};

const importedMemory = {
  id: 'memory_imported_browser',
  userId: session.user.id,
  agentId: 'agent_imported',
  type: 'user_managed',
  content: 'Imported escalation preference.',
  importance: 5,
  metadata: { imported: true },
  createdAt: '2026-06-15T13:10:00Z',
  updatedAt: '2026-06-15T13:10:00Z',
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
      error: { code: 'not_found', message: 'agent memories fixture route not found' },
    }),
  });
}

function searchQueryMatches(url: URL) {
  return (
    url.searchParams.get('agentId') === agentId &&
    url.searchParams.get('limit') === '5' &&
    url.searchParams.get('query') === 'rollout' &&
    url.searchParams.get('topK') === '5' &&
    url.searchParams.get('type') === 'long_term'
  );
}

function exportQueryMatches(url: URL) {
  return (
    url.searchParams.get('agentId') === agentId &&
    url.searchParams.get('limit') === '5' &&
    url.searchParams.get('query') === 'rollout' &&
    url.searchParams.get('type') === 'long_term' &&
    !url.searchParams.has('topK')
  );
}

function createPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.agentId === agentId &&
    payload.content === createdMemory.content &&
    payload.importance === createdMemory.importance &&
    payload.type === createdMemory.type &&
    JSON.stringify(payload.metadata) === JSON.stringify(createdMemory.metadata)
  );
}

function updatePayloadMatches(payload: Record<string, unknown>) {
  return payload.content === updatedMemory.content && payload.importance === updatedMemory.importance;
}

function importPayloadMatches(payload: Record<string, unknown>) {
  const memories = payload.memories;
  if (!Array.isArray(memories) || memories.length !== 1) {
    return false;
  }
  const [memory] = memories as Record<string, unknown>[];
  return (
    memory.agentId === importedMemory.agentId &&
    memory.content === importedMemory.content &&
    memory.importance === importedMemory.importance &&
    memory.type === importedMemory.type &&
    JSON.stringify(memory.metadata) === JSON.stringify(importedMemory.metadata)
  );
}

export async function registerAgentMemoriesRoutes(page: Page): Promise<void> {
  let createdVisible = false;
  let updatedVisible = false;
  let importedVisible = false;
  let deletedCreatedMemory = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/agent/memories') {
      if (!searchQueryMatches(url)) {
        await fulfillError(route, 'memory search query did not match browser filter selections');
        return;
      }
      await fulfillJSON(route, { memories: [searchedMemory], total: 1 });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/agent/memories/export') {
      if (!exportQueryMatches(url)) {
        await fulfillError(route, 'memory export query did not match browser filter selections');
        return;
      }
      await fulfillJSON(route, { memories: [searchedMemory], total: 1 });
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/agent/memories') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!createPayloadMatches(payload)) {
        await fulfillError(route, 'memory create payload did not match browser form selections');
        return;
      }
      createdVisible = true;
      deletedCreatedMemory = false;
      await fulfillJSON(route, createdMemory, 201);
      return;
    }

    if (method === 'PATCH' && pathname === `/api/v1/agent/memories/${createdMemory.id}`) {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!updatePayloadMatches(payload)) {
        await fulfillError(route, 'memory update payload did not match browser edit selections');
        return;
      }
      updatedVisible = true;
      await fulfillJSON(route, updatedMemory);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/agent/memories/import') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!importPayloadMatches(payload)) {
        await fulfillError(route, 'memory import payload did not match selected JSON file');
        return;
      }
      importedVisible = true;
      await fulfillJSON(route, [importedMemory], 201);
      return;
    }

    if (method === 'DELETE' && pathname === `/api/v1/agent/memories/${createdMemory.id}`) {
      if (!createdVisible || !updatedVisible || deletedCreatedMemory) {
        await fulfillError(route, 'memory delete did not target the updated browser-created memory');
        return;
      }
      deletedCreatedMemory = true;
      await fulfillJSON(route, { status: 'deleted' });
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/agent/memories/state') {
      await fulfillJSON(route, {
        createdVisible,
        deletedCreatedMemory,
        importedVisible,
        updatedVisible,
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
