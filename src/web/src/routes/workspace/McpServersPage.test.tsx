import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const addServer = vi.fn();
const connectServer = vi.fn();
const deleteServer = vi.fn();
const disconnectServer = vi.fn();
const executeTool = vi.fn();
const getServerStatus = vi.fn();
const listLocalServers = vi.fn();
const listServerTools = vi.fn();
const listServers = vi.fn();

vi.mock('../../features/mcp/mcpServersApi', () => ({
  createMcpServersApi: () => ({
    addServer,
    connectServer,
    deleteServer,
    disconnectServer,
    executeTool,
    getServerStatus,
    listLocalServers,
    listServerTools,
    listServers
  })
}));

import { McpServersPage } from './McpServersPage';

const researchServer = {
  id: 'mcp_1',
  name: 'Research tools',
  status: 'disconnected',
  url: 'https://mcp.example/sse'
};

describe('McpServersPage', () => {
  beforeEach(() => {
    addServer.mockReset();
    connectServer.mockReset();
    deleteServer.mockReset();
    disconnectServer.mockReset();
    executeTool.mockReset();
    getServerStatus.mockReset();
    listLocalServers.mockReset();
    listServerTools.mockReset();
    listServers.mockReset();

    listLocalServers.mockResolvedValue([
      {
        description: 'Tenant-safe local MCP tools exposed by this server',
        id: 'local_search',
        name: 'Local Search Tools',
        toolCount: 2
      }
    ]);
    listServers.mockResolvedValue([researchServer]);
  });

  it('drives MCP server lifecycle and tool execution from the workspace page', async () => {
    addServer.mockResolvedValue({
      hasAuthToken: true,
      id: 'mcp_2',
      name: 'Internal MCP',
      status: 'disconnected',
      url: 'https://mcp.internal/sse'
    });
    connectServer.mockResolvedValue({
      ...researchServer,
      lastConnectedAt: '2026-06-09T00:00:00Z',
      status: 'connected'
    });
    disconnectServer.mockResolvedValue({ status: 'disconnected' });
    getServerStatus.mockResolvedValue({ status: 'connected' });
    listServerTools.mockResolvedValue([
      {
        description: 'Search indexed workspace documents.',
        inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
        name: 'search_docs'
      }
    ]);
    executeTool.mockResolvedValue({ content: 'Found fusion design details.' });
    deleteServer.mockResolvedValue({ status: 'deleted' });

    render(<McpServersPage />);

    expect(await screen.findByRole('heading', { name: 'MCP Servers & Tools' })).toBeInTheDocument();
    expect(screen.getByText('Local Search Tools')).toBeInTheDocument();
    expect(screen.getByText('2 tools')).toBeInTheDocument();
    expect(screen.getByText('Research tools')).toBeInTheDocument();
    expect(screen.getByText('https://mcp.example/sse')).toBeInTheDocument();

    const researchCard = screen.getByText('Research tools').closest('article');
    if (!(researchCard instanceof HTMLElement)) {
      throw new Error('expected Research tools card');
    }

    fireEvent.click(within(researchCard).getByRole('button', { name: 'Connect' }));
    await waitFor(() => {
      expect(connectServer).toHaveBeenCalledWith('mcp_1');
      expect(within(researchCard).getByText('connected')).toBeInTheDocument();
    });
    expect(within(researchCard).getByText('Last connected: 2026-06-09T00:00:00Z')).toBeInTheDocument();

    fireEvent.click(within(researchCard).getByRole('button', { name: 'Disconnect' }));
    await waitFor(() => {
      expect(disconnectServer).toHaveBeenCalledWith('mcp_1');
      expect(within(researchCard).getByText('disconnected')).toBeInTheDocument();
    });

    fireEvent.click(within(researchCard).getByRole('button', { name: 'Diagnose' }));
    await waitFor(() => {
      expect(getServerStatus).toHaveBeenCalledWith('mcp_1');
      expect(within(researchCard).getByText('Diagnostic: connected')).toBeInTheDocument();
    });

    fireEvent.click(within(researchCard).getByRole('button', { name: 'List tools' }));
    await waitFor(() => {
      expect(listServerTools).toHaveBeenCalledWith('mcp_1');
      expect(within(researchCard).getByText('search_docs')).toBeInTheDocument();
    });
    expect(within(researchCard).getByText('Search indexed workspace documents.')).toBeInTheDocument();
    expect(within(researchCard).getByLabelText('Tool name')).toHaveValue('search_docs');

    fireEvent.change(within(researchCard).getByLabelText('Tool arguments JSON'), { target: { value: '{' } });
    fireEvent.click(within(researchCard).getByRole('button', { name: 'Execute test call' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Tool arguments JSON is invalid.');
    expect(executeTool).not.toHaveBeenCalled();

    fireEvent.change(within(researchCard).getByLabelText('Tool arguments JSON'), {
      target: { value: '{"query":"fusion"}' }
    });
    fireEvent.click(within(researchCard).getByRole('button', { name: 'Execute test call' }));
    await waitFor(() => {
      expect(executeTool).toHaveBeenCalledWith('mcp_1', { args: { query: 'fusion' }, toolName: 'search_docs' });
      expect(within(researchCard).getByText('Found fusion design details.')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Server name'), { target: { value: ' Internal MCP ' } });
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: ' https://mcp.internal/sse ' } });
    fireEvent.change(screen.getByLabelText('Auth token'), { target: { value: ' secret-token ' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add MCP server' }));

    await waitFor(() => {
      expect(addServer).toHaveBeenCalledWith({
        authToken: 'secret-token',
        name: 'Internal MCP',
        url: 'https://mcp.internal/sse'
      });
    });
    expect(screen.getByText('Internal MCP')).toBeInTheDocument();
    expect(screen.getByText('Auth token configured')).toBeInTheDocument();
    expect(screen.getByLabelText('Server name')).toHaveValue('');
    expect(screen.getByLabelText('Endpoint URL')).toHaveValue('');
    expect(screen.getByLabelText('Auth token')).toHaveValue('');

    fireEvent.click(screen.getByLabelText('Delete Internal MCP'));
    expect(screen.getByText('Are you sure you want to delete this MCP server? This action cannot be undone.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Delete Server' }));

    await waitFor(() => {
      expect(deleteServer).toHaveBeenCalledWith('mcp_2');
      expect(screen.queryByText('Internal MCP')).not.toBeInTheDocument();
    });
  });

  it('shows load errors from the MCP page surface', async () => {
    listLocalServers.mockRejectedValueOnce(new Error('catalog unavailable'));
    listServers.mockResolvedValueOnce([]);

    render(<McpServersPage />);

    expect(await screen.findByRole('alert')).toHaveTextContent('catalog unavailable');
  });
});
