import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createMarketplaceApi } from './api';

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

describe('createMarketplaceApi', () => {
  it('normalizes marketplace collection envelopes', async () => {
    const client = createClient({
      get: vi
        .fn()
        .mockResolvedValueOnce({ agents: [{ id: 'agent_1', name: 'Researcher' }], total: 1 })
        .mockResolvedValueOnce({ categories: [{ id: 'cat_1', slug: 'research' }], total: 1 })
        .mockResolvedValueOnce({ agent: { id: 'agent_1' }, versions: [{ version: '1.0.0' }] }),
    });

    const api = createMarketplaceApi(client);

    await expect(api.getFeatured()).resolves.toEqual([{ id: 'agent_1', name: 'Researcher' }]);
    await expect(api.getCategories()).resolves.toEqual([{ id: 'cat_1', slug: 'research' }]);
    await expect(api.getAgent('agent_1')).resolves.toEqual({ id: 'agent_1' });

    expect(client.get).toHaveBeenNthCalledWith(1, '/api/v1/marketplace/featured');
    expect(client.get).toHaveBeenNthCalledWith(2, '/api/v1/marketplace/categories');
    expect(client.get).toHaveBeenNthCalledWith(3, '/api/v1/marketplace/agents/agent_1');
  });

  it('matches backend install and uninstall routes', async () => {
    const post = vi.fn().mockResolvedValue({ id: 'install_1' });
    const del = vi.fn().mockResolvedValue(undefined);
    const api = createMarketplaceApi(createClient({ post, delete: del }));

    await api.installAgent('agent_1', 'ver_1');
    await api.uninstallAgent('agent_1');

    expect(post).toHaveBeenCalledWith('/api/v1/marketplace/agents/agent_1/install?versionID=ver_1');
    expect(del).toHaveBeenCalledWith('/api/v1/marketplace/installs/agent_1');
  });
});
