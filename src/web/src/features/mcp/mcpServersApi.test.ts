import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createMcpServersApi } from './mcpServersApi';

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

describe('createMcpServersApi', () => {
  it('wraps MCP server lifecycle, diagnostics, tools, and execution endpoints', async () => {
    const get = vi
      .fn()
      .mockResolvedValueOnce([{ id: 'mcp_1', name: 'Research tools', status: 'disconnected', url: 'https://mcp.example/sse' }])
      .mockResolvedValueOnce({ status: 'connected' })
      .mockResolvedValueOnce([{ description: 'Search indexed docs', inputSchema: { type: 'object' }, name: 'search_docs' }]);
    const post = vi
      .fn()
      .mockResolvedValueOnce({ id: 'mcp_2', name: 'Internal MCP', status: 'disconnected', url: 'https://mcp.internal/sse' })
      .mockResolvedValueOnce({ id: 'mcp_1', name: 'Research tools', status: 'connected', url: 'https://mcp.example/sse' })
      .mockResolvedValueOnce({ status: 'disconnected' })
      .mockResolvedValueOnce({ content: '{"ok":true}', isError: false });
    const del = vi.fn().mockResolvedValue({ status: 'deleted' });
    const api = createMcpServersApi(createClient({ delete: del, get, post }));

    await expect(api.listServers()).resolves.toEqual([
      expect.objectContaining({ id: 'mcp_1', name: 'Research tools' })
    ]);
    await expect(api.addServer({ authToken: 'secret', name: 'Internal MCP', url: 'https://mcp.internal/sse' })).resolves.toEqual(
      expect.objectContaining({ id: 'mcp_2' })
    );
    await expect(api.connectServer('mcp_1')).resolves.toEqual(expect.objectContaining({ status: 'connected' }));
    await expect(api.disconnectServer('mcp_1')).resolves.toEqual({ status: 'disconnected' });
    await expect(api.getServerStatus('mcp_1')).resolves.toEqual({ status: 'connected' });
    await expect(api.listServerTools('mcp_1')).resolves.toEqual([
      expect.objectContaining({ name: 'search_docs' })
    ]);
    await expect(api.executeTool('mcp_1', { args: { query: 'fusion' }, toolName: 'search_docs' })).resolves.toEqual({
      content: '{"ok":true}',
      isError: false
    });
    await expect(api.deleteServer('mcp_1')).resolves.toEqual({ status: 'deleted' });

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/app/mcp-servers');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/app/mcp-servers', {
      authToken: 'secret',
      name: 'Internal MCP',
      url: 'https://mcp.internal/sse'
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/app/mcp-servers/mcp_1/connect');
    expect(post).toHaveBeenNthCalledWith(3, '/api/v1/app/mcp-servers/mcp_1/disconnect');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/app/mcp-servers/mcp_1/status');
    expect(get).toHaveBeenNthCalledWith(3, '/api/v1/app/mcp-servers/mcp_1/tools');
    expect(post).toHaveBeenNthCalledWith(4, '/api/v1/app/mcp-servers/mcp_1/execute', {
      args: { query: 'fusion' },
      toolName: 'search_docs'
    });
    expect(del).toHaveBeenCalledWith('/api/v1/app/mcp-servers/mcp_1');
  });

  it('lists local tenant-safe MCP servers from the local catalog endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        description: 'Tenant-safe local MCP tools exposed by this server',
        id: 'local_builtin_safe',
        name: 'Oblivious Safe Builtins',
        toolCount: 2
      }
    ]);
    const api = createMcpServersApi(createClient({ get }));

    await expect(api.listLocalServers()).resolves.toEqual([
      expect.objectContaining({
        id: 'local_builtin_safe',
        name: 'Oblivious Safe Builtins',
        toolCount: 2
      })
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/app/mcp-local-servers');
  });
});
