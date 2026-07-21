import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createMcpServersApi } from './mcpServersApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: overrides.delete
      ? ((path, init) => init === undefined ? overrides.delete!(path) : overrides.delete!(path, init)) as HttpClient['delete']
      : vi.fn(),
    get: overrides.get
      ? ((path, init) => init === undefined ? overrides.get!(path) : overrides.get!(path, init)) as HttpClient['get']
      : vi.fn(),
    post: overrides.post
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.post!(path) : overrides.post!(path, body)
          : overrides.post!(path, body, init)) as HttpClient['post']
      : vi.fn(),
    put: overrides.put
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.put!(path) : overrides.put!(path, body)
          : overrides.put!(path, body, init)) as HttpClient['put']
      : vi.fn(),
    request: overrides.request
      ? ((path, init) => init === undefined ? overrides.request!(path) : overrides.request!(path, init)) as HttpClient['request']
      : vi.fn(),
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
    await expect(api.addServer({
      authToken: 'secret',
      capabilityId: 'caller.mcp.authority',
      name: 'Internal MCP',
      url: 'https://mcp.internal/sse'
    } as Parameters<typeof api.addServer>[0] & { capabilityId: string })).resolves.toEqual(
      expect.objectContaining({ id: 'mcp_2' })
    );
    await expect(api.connectServer('mcp_1')).resolves.toEqual(expect.objectContaining({ status: 'connected' }));
    await expect(api.disconnectServer('mcp_1')).resolves.toEqual({ status: 'disconnected' });
    await expect(api.getServerStatus('mcp_1')).resolves.toEqual({ status: 'connected' });
    await expect(api.listServerTools('mcp_1')).resolves.toEqual([
      expect.objectContaining({ name: 'search_docs' })
    ]);
    await expect(api.executeTool('mcp_1', {
      args: { query: 'fusion' },
      capabilityId: 'caller.mcp.execution',
      toolName: 'search_docs'
    } as Parameters<typeof api.executeTool>[1] & { capabilityId: string })).resolves.toEqual({
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
