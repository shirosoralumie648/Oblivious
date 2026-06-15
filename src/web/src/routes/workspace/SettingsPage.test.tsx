import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import type { UserPreferences } from '../../types/api';

const navigate = vi.fn();
const listServers = vi.fn();
const addServer = vi.fn();
const connectServer = vi.fn();
const deleteServer = vi.fn();
const disconnectServer = vi.fn();
const getServerStatus = vi.fn();
const listLocalServers = vi.fn();
const listServerTools = vi.fn();
const executeTool = vi.fn();
const appContext = vi.hoisted(() => ({
  authState: {
    preferences: {
      defaultMode: 'chat' as const,
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    } as UserPreferences,
    status: 'authenticated' as const,
    user: { email: 'user@example.com', id: 'u1' }
  },
  updatePreferences: vi.fn(async (preferences) => preferences)
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual<typeof import('react-router-dom')>('react-router-dom');

  return {
    ...actual,
    useNavigate: () => navigate
  };
});

vi.mock('../../app/providers', () => ({
  useAppContext: () => appContext
}));

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

import { SettingsPage } from './SettingsPage';

describe('SettingsPage', () => {
  beforeEach(() => {
    navigate.mockReset();
    listServers.mockReset();
    addServer.mockReset();
    connectServer.mockReset();
    deleteServer.mockReset();
    disconnectServer.mockReset();
    getServerStatus.mockReset();
    listLocalServers.mockReset();
    listServerTools.mockReset();
    executeTool.mockReset();
    appContext.authState.preferences = {
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: false,
      onboardingCompleted: true
    } as UserPreferences;
    appContext.updatePreferences.mockClear();
    listServers.mockResolvedValue([]);
    deleteServer.mockResolvedValue({ status: 'deleted' });
    listLocalServers.mockResolvedValue([
      {
        description: 'Tenant-safe local MCP tools exposed by this server',
        id: 'local_builtin_safe',
        name: 'Oblivious Safe Builtins',
        toolCount: 2
      }
    ]);
  });

  it('renders the current workspace preferences', async () => {
    appContext.authState.preferences = {
      defaultMode: 'solo',
      modelStrategy: 'quality',
      networkEnabledHint: true,
      onboardingCompleted: true
    } as UserPreferences;

    render(<SettingsPage />);

    expect(screen.getByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByLabelText('Default mode')).toHaveValue('solo');
    expect(screen.getByLabelText('Model strategy')).toHaveValue('quality');
    expect(screen.getByLabelText('Enable web suggestions')).toBeChecked();
    expect(screen.getByText('Onboarding complete')).toBeInTheDocument();
    expect(await screen.findByText('No remote MCP servers registered.')).toBeInTheDocument();
  });

  it('saves updated preferences', async () => {
    render(<SettingsPage />);

    fireEvent.change(screen.getByLabelText('Default mode'), { target: { value: 'solo' } });
    fireEvent.change(screen.getByLabelText('Model strategy'), { target: { value: 'cost' } });
    fireEvent.click(screen.getByLabelText('Enable web suggestions'));
    fireEvent.click(screen.getByRole('button', { name: 'Save preferences' }));

    await waitFor(() => {
      expect(appContext.updatePreferences).toHaveBeenCalledWith({
        defaultMode: 'solo',
        modelStrategy: 'cost',
        networkEnabledHint: true,
        onboardingCompleted: true
      });
    });

    expect(screen.getByText('Preferences saved.')).toBeInTheDocument();
  });

  it('offers a return path back to chat', async () => {
    render(<SettingsPage />);

    expect(screen.getByRole('button', { name: 'Return to chat' })).toBeInTheDocument();
    expect(await screen.findByText('No remote MCP servers registered.')).toBeInTheDocument();
  });

  it('shows tenant-safe local MCP servers even when no remote servers are registered', async () => {
    render(<SettingsPage />);

    expect(await screen.findByText('Oblivious Safe Builtins')).toBeInTheDocument();
    expect(screen.getByText('Tenant-safe local MCP tools exposed by this server')).toBeInTheDocument();
    expect(screen.getByText('2 tools')).toBeInTheDocument();
    expect(listLocalServers).toHaveBeenCalled();
    expect(screen.getByText('No remote MCP servers registered.')).toBeInTheDocument();
  });

  it('manages MCP servers, diagnoses connection, lists tools, executes a test call, and shows errors', async () => {
    listServers.mockResolvedValueOnce([
      {
        hasAuthToken: true,
        id: 'mcp_1',
        lastConnectedAt: '2026-06-04T08:00:00Z',
        name: 'Research MCP',
        status: 'disconnected',
        url: 'https://mcp.example/sse'
      }
    ]);
    addServer.mockResolvedValueOnce({
      id: 'mcp_2',
      name: 'Internal MCP',
      status: 'disconnected',
      url: 'https://mcp.internal/sse'
    });
    connectServer.mockResolvedValueOnce({
      id: 'mcp_1',
      name: 'Research MCP',
      status: 'connected',
      url: 'https://mcp.example/sse'
    });
    getServerStatus.mockResolvedValueOnce({ status: 'connected' }).mockRejectedValueOnce(new Error('diagnostic timeout'));
    listServerTools.mockResolvedValueOnce([
      {
        description: 'Search workspace docs',
        inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
        name: 'search_docs'
      }
    ]);
    executeTool.mockResolvedValueOnce({ content: '{"matches":2}', isError: false });
    disconnectServer.mockResolvedValueOnce({ status: 'disconnected' });

    render(<SettingsPage />);

    expect(await screen.findByText('Research MCP')).toBeInTheDocument();
    expect(screen.getByText('https://mcp.example/sse')).toBeInTheDocument();
    expect(screen.getByText('Auth token configured')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Server name'), { target: { value: 'Internal MCP' } });
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: 'https://mcp.internal/sse' } });
    fireEvent.change(screen.getByLabelText('Auth token'), { target: { value: 'secret-token' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add MCP server' }));

    await waitFor(() => {
      expect(addServer).toHaveBeenCalledWith({
        authToken: 'secret-token',
        name: 'Internal MCP',
        url: 'https://mcp.internal/sse'
      });
    });
    expect(screen.getByText('Internal MCP')).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: /Connect to/i })[0]);
    await waitFor(() => expect(connectServer).toHaveBeenCalledWith('mcp_1'));

    fireEvent.click(screen.getAllByRole('button', { name: /Diagnose/i })[0]);
    await waitFor(() => expect(getServerStatus).toHaveBeenCalledWith('mcp_1'));
    expect(screen.getByText('Diagnostic: connected')).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: /List tools for/i })[0]);
    expect(await screen.findByText('search_docs')).toBeInTheDocument();
    expect(screen.getByText('Search workspace docs')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Tool name'), { target: { value: 'search_docs' } });
    fireEvent.change(screen.getByLabelText('Tool arguments JSON'), { target: { value: '{"query":"fusion"}' } });
    fireEvent.click(screen.getByRole('button', { name: 'Execute test call' }));

    await waitFor(() => {
      expect(executeTool).toHaveBeenCalledWith('mcp_1', {
        args: { query: 'fusion' },
        toolName: 'search_docs'
      });
    });
    expect(screen.getByText('{"matches":2}')).toBeInTheDocument();

    fireEvent.click(screen.getAllByRole('button', { name: /Disconnect from/i })[0]);
    await waitFor(() => expect(disconnectServer).toHaveBeenCalledWith('mcp_1'));

    fireEvent.click(screen.getAllByRole('button', { name: /Diagnose/i })[0]);
    expect(await screen.findByRole('alert')).toHaveTextContent('diagnostic timeout');
  });

  it('deletes a remote MCP server from the settings list', async () => {
    listServers.mockResolvedValueOnce([
      {
        id: 'mcp_1',
        name: 'Research MCP',
        status: 'disconnected',
        url: 'https://mcp.example/sse'
      }
    ]);

    render(<SettingsPage />);

    expect(await screen.findByText('Research MCP')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Delete Research MCP' }));

    await waitFor(() => expect(deleteServer).toHaveBeenCalledWith('mcp_1'));
    expect(screen.queryByText('Research MCP')).not.toBeInTheDocument();
    expect(screen.getByText('No remote MCP servers registered.')).toBeInTheDocument();
  });
});
