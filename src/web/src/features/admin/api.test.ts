import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createAdminApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    ...overrides,
  };
  return client;
}

describe('createAdminApi', () => {
  it('normalizes list responses from backend collection keys', async () => {
    const client = createClient({
      get: vi
        .fn()
        .mockResolvedValueOnce({ channels: [{ id: 'ch_1', name: 'OpenAI' }], total: 1 })
        .mockResolvedValueOnce({ routes: [{ id: 'rt_1', model: 'gpt-4o' }], total: 1 })
        .mockResolvedValueOnce({ entries: [{ id: 'aud_1', action: 'channel.create' }], total: 1 }),
    });

    const api = createAdminApi(client);

    await expect(api.listChannels({ provider: 'openai', limit: 10 })).resolves.toEqual({
      data: [{ id: 'ch_1', name: 'OpenAI' }],
      total: 1,
    });
    await expect(api.listRoutes()).resolves.toEqual([{ id: 'rt_1', model: 'gpt-4o' }]);
    await expect(api.listAuditLogs({ action: 'channel.create' })).resolves.toEqual({
      data: [{ id: 'aud_1', action: 'channel.create' }],
      total: 1,
    });

    expect(client.get).toHaveBeenNthCalledWith(1, '/api/v1/admin/channels?provider=openai&limit=10');
    expect(client.get).toHaveBeenNthCalledWith(2, '/api/v1/admin/routes');
    expect(client.get).toHaveBeenNthCalledWith(3, '/api/v1/admin/audit-logs?action=channel.create');
  });

  it('uses the backend action payload for channel batch updates', async () => {
    const post = vi.fn().mockResolvedValue(undefined);
    const api = createAdminApi(createClient({ post }));

    await api.batchUpdateChannels(['ch_1', 'ch_2'], 'disable');

    expect(post).toHaveBeenCalledWith('/api/v1/admin/channels/batch', {
      ids: ['ch_1', 'ch_2'],
      action: 'disable',
    });
  });
});
