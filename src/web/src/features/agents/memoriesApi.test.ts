import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createAgentMemoriesApi } from './memoriesApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
  return client;
}

describe('createAgentMemoriesApi', () => {
  it('creates a user-managed memory through the agent memories endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      content: 'Remember concise release notes.',
      id: 'memory_1',
      type: 'user_managed'
    });
    const api = createAgentMemoriesApi(createClient({ post }));

    await expect(
      api.createMemory({
        agentId: 'agent_1',
        content: 'Remember concise release notes.',
        importance: 4,
        metadata: { source: 'workspace' },
        type: 'user_managed'
      })
    ).resolves.toEqual({
      content: 'Remember concise release notes.',
      id: 'memory_1',
      type: 'user_managed'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/memories', {
      agentId: 'agent_1',
      content: 'Remember concise release notes.',
      importance: 4,
      metadata: { source: 'workspace' },
      type: 'user_managed'
    });
  });

  it('searches memories with query and topK parameters', async () => {
    const get = vi.fn().mockResolvedValue({
      data: [{ content: 'Use dry-run before migrations.', id: 'memory_2', type: 'long_term' }]
    });
    const api = createAgentMemoriesApi(createClient({ get }));

    await expect(api.searchMemories({ agentId: 'agent_1', query: 'migrations', topK: 5 })).resolves.toEqual({
      data: [{ content: 'Use dry-run before migrations.', id: 'memory_2', type: 'long_term' }],
      total: 1
    });

    expect(get).toHaveBeenCalledWith('/api/v1/agent/memories?agentId=agent_1&query=migrations&topK=5');
  });

  it('imports memories through the bulk import endpoint', async () => {
    const post = vi.fn().mockResolvedValue([
      { content: 'Imported one.', id: 'memory_import_1', type: 'user_managed' },
      { content: 'Imported two.', id: 'memory_import_2', type: 'user_managed' }
    ]);
    const api = createAgentMemoriesApi(createClient({ post }));
    const payload = [
      {
        agentId: 'agent_1',
        content: 'Imported one.',
        importance: 5,
        metadata: { imported: true },
        type: 'user_managed'
      },
      {
        content: 'Imported two.',
        importance: 3,
        type: 'user_managed'
      }
    ];

    await expect(api.importMemories(payload)).resolves.toEqual([
      { content: 'Imported one.', id: 'memory_import_1', type: 'user_managed' },
      { content: 'Imported two.', id: 'memory_import_2', type: 'user_managed' }
    ]);

    expect(post).toHaveBeenCalledWith('/api/v1/agent/memories/import', { memories: payload });
  });

  it('exports memories through the scoped export endpoint', async () => {
    const get = vi.fn().mockResolvedValue({
      memories: [{ content: 'Exported memory.', id: 'memory_export_1', type: 'user_managed' }],
      total: 1
    });
    const api = createAgentMemoriesApi(createClient({ get }));

    await expect(api.exportMemories({ agentId: 'agent_1', query: 'release', type: 'user_managed' })).resolves.toEqual({
      data: [{ content: 'Exported memory.', id: 'memory_export_1', type: 'user_managed' }],
      total: 1
    });

    expect(get).toHaveBeenCalledWith('/api/v1/agent/memories/export?agentId=agent_1&query=release&type=user_managed');
  });

  it('searches memories with type and limit filters', async () => {
    const get = vi.fn().mockResolvedValue({
      data: [
        {
          content: 'Use profile-aware retrieval.',
          id: 'memory_3',
          type: 'user_managed'
        }
      ],
      total: 1
    });
    const api = createAgentMemoriesApi(createClient({ get }));

    await api.searchMemories({
      agentId: 'agent_2',
      limit: 3,
      query: 'retrieval',
      type: 'user_managed'
    });

    expect(get).toHaveBeenCalledWith(
      '/api/v1/agent/memories?agentId=agent_2&limit=3&query=retrieval&type=user_managed'
    );
  });

  it('updates memory content and importance through PATCH', async () => {
    const request = vi.fn().mockResolvedValue({
      content: 'Use validation gates before release notes.',
      id: 'memory_4',
      importance: 5,
      type: 'user_managed'
    });
    const api = createAgentMemoriesApi(createClient({ request }));

    await expect(
      api.updateMemory('memory_4', {
        content: 'Use validation gates before release notes.',
        importance: 5
      })
    ).resolves.toEqual({
      content: 'Use validation gates before release notes.',
      id: 'memory_4',
      importance: 5,
      type: 'user_managed'
    });

    expect(request).toHaveBeenCalledWith('/api/v1/agent/memories/memory_4', {
      body: JSON.stringify({
        content: 'Use validation gates before release notes.',
        importance: 5
      }),
      method: 'PATCH'
    });
  });

  it('deletes a memory by id', async () => {
    const deleteRequest = vi.fn().mockResolvedValue(undefined);
    const api = createAgentMemoriesApi(createClient({ delete: deleteRequest }));

    await expect(api.deleteMemory('memory_5')).resolves.toBeUndefined();

    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/agent/memories/memory_5');
  });
});
