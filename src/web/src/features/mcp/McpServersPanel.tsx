import { useEffect, useMemo, useState } from 'react';
import { RiAddLine, RiDeleteBinLine, RiLink, RiLinkUnlink, RiListCheck, RiLoader4Line, RiPlayLine, RiSearchLine } from '@remixicon/react';

import { createHttpClient } from '../../services/http/client';
import {
  createMcpServersApi,
  type LocalMcpServer,
  type McpServer,
  type McpServersApi,
  type McpToolDefinition,
  type McpToolResult
} from './mcpServersApi';

type McpServersPanelProps = {
  api?: McpServersApi;
};

type ToolState = {
  diagnostic?: string;
  result?: McpToolResult;
  selectedToolName?: string;
  tools?: McpToolDefinition[];
};

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }
  return fallback;
}

function prettySchema(schema: unknown) {
  if (schema === undefined || schema === null) {
    return '';
  }

  try {
    return JSON.stringify(schema);
  } catch {
    return String(schema);
  }
}

function mergeServer(servers: McpServer[], server: McpServer) {
  if (!servers.some((candidate) => candidate.id === server.id)) {
    return [...servers, server];
  }

  return servers.map((candidate) => (candidate.id === server.id ? server : candidate));
}

export function McpServersPanel({ api }: McpServersPanelProps) {
  const defaultApi = useMemo(() => createMcpServersApi(createHttpClient()), []);
  const mcpApi = api ?? defaultApi;
  const [argsJson, setArgsJson] = useState('{\n  \n}');
  const [authToken, setAuthToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isAdding, setIsAdding] = useState(false);
  const [localServers, setLocalServers] = useState<LocalMcpServer[]>([]);
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [servers, setServers] = useState<McpServer[]>([]);
  const [toolName, setToolName] = useState('');
  const [toolStateByServerId, setToolStateByServerId] = useState<Record<string, ToolState>>({});
  const [url, setUrl] = useState('');

  useEffect(() => {
    let cancelled = false;

    const loadServers = async () => {
      setError(null);
      try {
        const [loadedLocalServers, loaded] = await Promise.all([mcpApi.listLocalServers(), mcpApi.listServers()]);
        if (!cancelled) {
          setLocalServers(loadedLocalServers);
          setServers(loaded);
        }
      } catch (caughtError) {
        if (!cancelled) {
          setError(errorMessage(caughtError, 'Unable to load MCP servers.'));
        }
      }
    };

    void loadServers();

    return () => {
      cancelled = true;
    };
  }, [mcpApi]);

  const updateToolState = (serverId: string, updater: (current: ToolState) => ToolState) => {
    setToolStateByServerId((current) => ({
      ...current,
      [serverId]: updater(current[serverId] ?? {})
    }));
  };

  const addServer = async () => {
    const trimmedName = name.trim();
    const trimmedUrl = url.trim();
    if (trimmedName === '' || trimmedUrl === '') {
      return;
    }

    setIsAdding(true);
    setError(null);

    try {
      const created = await mcpApi.addServer({
        authToken: authToken.trim() || undefined,
        name: trimmedName,
        url: trimmedUrl
      });
      setServers((current) => mergeServer(current, created));
      setAuthToken('');
      setName('');
      setUrl('');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to add MCP server.'));
    } finally {
      setIsAdding(false);
    }
  };

  const connectServer = async (serverId: string) => {
    setLoadingAction(`connect:${serverId}`);
    setError(null);

    try {
      const connected = await mcpApi.connectServer(serverId);
      setServers((current) => mergeServer(current, connected));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to connect MCP server.'));
    } finally {
      setLoadingAction(null);
    }
  };

  const disconnectServer = async (serverId: string) => {
    setLoadingAction(`disconnect:${serverId}`);
    setError(null);

    try {
      const response = await mcpApi.disconnectServer(serverId);
      setServers((current) =>
        current.map((server) => (server.id === serverId ? { ...server, status: response.status } : server))
      );
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to disconnect MCP server.'));
    } finally {
      setLoadingAction(null);
    }
  };

  const deleteServer = async (serverId: string) => {
    setLoadingAction(`delete:${serverId}`);
    setError(null);

    try {
      await mcpApi.deleteServer(serverId);
      setServers((current) => current.filter((server) => server.id !== serverId));
      setToolStateByServerId((current) => {
        const next = { ...current };
        delete next[serverId];
        return next;
      });
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to delete MCP server.'));
    } finally {
      setLoadingAction(null);
    }
  };

  const diagnoseServer = async (serverId: string) => {
    setLoadingAction(`diagnose:${serverId}`);
    setError(null);

    try {
      const response = await mcpApi.getServerStatus(serverId);
      updateToolState(serverId, (current) => ({ ...current, diagnostic: response.status }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to diagnose MCP server.'));
    } finally {
      setLoadingAction(null);
    }
  };

  const listTools = async (serverId: string) => {
    setLoadingAction(`tools:${serverId}`);
    setError(null);

    try {
      const tools = await mcpApi.listServerTools(serverId);
      updateToolState(serverId, (current) => ({
        ...current,
        selectedToolName: tools[0]?.name ?? current.selectedToolName,
        tools
      }));
      if (tools[0]?.name) {
        setToolName(tools[0].name);
      }
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to list MCP tools.'));
    } finally {
      setLoadingAction(null);
    }
  };

  const executeTool = async (serverId: string) => {
    const trimmedToolName = toolName.trim();
    if (trimmedToolName === '') {
      return;
    }

    let args: Record<string, unknown> = {};
    const trimmedArgs = argsJson.trim();
    if (trimmedArgs !== '') {
      try {
        const parsed = JSON.parse(trimmedArgs) as unknown;
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          setError('Tool arguments JSON must be an object.');
          return;
        }
        args = parsed as Record<string, unknown>;
      } catch {
        setError('Tool arguments JSON is invalid.');
        return;
      }
    }

    setLoadingAction(`execute:${serverId}`);
    setError(null);

    try {
      const result = await mcpApi.executeTool(serverId, { args, toolName: trimmedToolName });
      updateToolState(serverId, (current) => ({ ...current, result, selectedToolName: trimmedToolName }));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to execute MCP tool.'));
    } finally {
      setLoadingAction(null);
    }
  };

  return (
    <section className="min-w-0 max-w-full rounded-lg border border-border bg-card p-5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Agent tools</p>
          <h2 className="text-lg font-semibold text-foreground">MCP Servers</h2>
        </div>
      </div>

      {error ? (
        <p className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      <form
        className="mt-5 grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.35fr)_minmax(0,1fr)_auto]"
        onSubmit={(event) => {
          event.preventDefault();
          void addServer();
        }}
      >
        <label className="text-sm font-medium text-foreground">
          Server name
          <input
            className="mt-2 min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 py-2 text-sm text-foreground"
            onChange={(event) => setName(event.target.value)}
            placeholder="Research MCP"
            type="text"
            value={name}
          />
        </label>
        <label className="text-sm font-medium text-foreground">
          Endpoint URL
          <input
            className="mt-2 min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 py-2 text-sm text-foreground"
            onChange={(event) => setUrl(event.target.value)}
            placeholder="https://mcp.example/sse"
            type="url"
            value={url}
          />
        </label>
        <label className="text-sm font-medium text-foreground">
          Auth token
          <input
            className="mt-2 min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 py-2 text-sm text-foreground"
            onChange={(event) => setAuthToken(event.target.value)}
            type="password"
            value={authToken}
          />
        </label>
        <button
          className="inline-flex min-h-[44px] items-center gap-2 self-end rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground disabled:cursor-not-allowed disabled:opacity-50"
          disabled={isAdding || name.trim() === '' || url.trim() === ''}
          type="submit"
        >
          <RiAddLine className="size-4" aria-hidden="true" />
          {isAdding ? 'Adding...' : 'Add MCP server'}
        </button>
      </form>

      <div className="mt-5 space-y-3">
        {localServers.length > 0 ? (
          <section aria-label="Local MCP servers" className="grid min-w-0 gap-3 md:grid-cols-2">
            {localServers.map((server) => (
              <article className="min-w-0 rounded-lg border border-border bg-background/60 p-4" key={server.id}>
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0">
                    <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Local safe server</p>
                    <h3 className="mt-1 break-words text-base font-semibold text-foreground [overflow-wrap:anywhere]">
                      {server.name}
                    </h3>
                    {server.description ? (
                      <p className="mt-1 break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">
                        {server.description}
                      </p>
                    ) : null}
                  </div>
                  <span className="shrink-0 rounded-full border border-border bg-muted/40 px-2 py-1 text-xs font-semibold text-muted-foreground">
                    {server.toolCount} {server.toolCount === 1 ? 'tool' : 'tools'}
                  </span>
                </div>
              </article>
            ))}
          </section>
        ) : null}
        {servers.length === 0 ? <p className="text-sm text-muted-foreground">No remote MCP servers registered.</p> : null}
        {servers.map((server) => {
          const toolState = toolStateByServerId[server.id] ?? {};
          const tools = toolState.tools ?? [];

          return (
            <article className="min-w-0 max-w-full rounded-lg border border-border bg-background/60 p-4" key={server.id}>
              <div className="flex min-w-0 flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0 max-w-full">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="min-w-0 break-words text-base font-semibold text-foreground [overflow-wrap:anywhere]">
                      {server.name}
                    </h3>
                    <span className="rounded-full border border-border bg-muted/40 px-2 py-1 text-xs font-semibold uppercase text-muted-foreground">
                      {server.status}
                    </span>
                  </div>
                  <p className="mt-1 break-all text-sm text-muted-foreground">{server.url}</p>
                  {server.hasAuthToken ? (
                    <p className="mt-1 text-xs font-semibold text-primary">Auth token configured</p>
                  ) : null}
                  {server.lastConnectedAt ? (
                    <p className="mt-1 text-xs text-muted-foreground">Last connected: {server.lastConnectedAt}</p>
                  ) : null}
                  {toolState.diagnostic ? (
                    <p className="mt-2 text-sm font-medium text-primary">Diagnostic: {toolState.diagnostic}</p>
                  ) : null}
                </div>
                <div className="flex min-w-0 max-w-full flex-wrap gap-2">
                  <button
                    className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-input bg-input/30 px-3 py-2 text-sm font-semibold text-foreground"
                    disabled={loadingAction === `connect:${server.id}`}
                    onClick={() => void connectServer(server.id)}
                    type="button"
                  >
                    {loadingAction === `connect:${server.id}` ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiLink className="size-4" aria-hidden="true" />}
                    Connect
                  </button>
                  <button
                    className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-input bg-input/30 px-3 py-2 text-sm font-semibold text-foreground"
                    disabled={loadingAction === `disconnect:${server.id}`}
                    onClick={() => void disconnectServer(server.id)}
                    type="button"
                  >
                    {loadingAction === `disconnect:${server.id}` ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiLinkUnlink className="size-4" aria-hidden="true" />}
                    Disconnect
                  </button>
                  <button
                    className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-input bg-input/30 px-3 py-2 text-sm font-semibold text-foreground"
                    disabled={loadingAction === `diagnose:${server.id}`}
                    onClick={() => void diagnoseServer(server.id)}
                    type="button"
                  >
                    {loadingAction === `diagnose:${server.id}` ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiSearchLine className="size-4" aria-hidden="true" />}
                    Diagnose
                  </button>
                  <button
                    className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-input bg-input/30 px-3 py-2 text-sm font-semibold text-foreground"
                    disabled={loadingAction === `tools:${server.id}`}
                    onClick={() => void listTools(server.id)}
                    type="button"
                  >
                    {loadingAction === `tools:${server.id}` ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiListCheck className="size-4" aria-hidden="true" />}
                    List tools
                  </button>
                  <button
                    aria-label={`Delete ${server.name}`}
                    className="inline-flex min-h-[44px] items-center gap-2 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm font-semibold text-destructive disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={loadingAction === `delete:${server.id}`}
                    onClick={() => void deleteServer(server.id)}
                    type="button"
                  >
                    {loadingAction === `delete:${server.id}` ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiDeleteBinLine className="size-4" aria-hidden="true" />}
                    Delete
                  </button>
                </div>
              </div>

              {tools.length > 0 ? (
                <div className="mt-4 grid min-w-0 max-w-full gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.8fr)]">
                  <section aria-label={`${server.name} tools`} className="min-w-0 space-y-2">
                    {tools.map((tool) => (
                      <div className="min-w-0 rounded-lg border border-border bg-card p-3" key={tool.name}>
                        <p className="break-words text-sm font-semibold text-foreground [overflow-wrap:anywhere]">{tool.name}</p>
                        {tool.description ? (
                          <p className="mt-1 break-words text-sm text-muted-foreground [overflow-wrap:anywhere]">
                            {tool.description}
                          </p>
                        ) : null}
                        {tool.inputSchema ? (
                          <code className="mt-2 block max-w-full whitespace-pre-wrap break-words rounded bg-muted/40 p-2 text-xs text-foreground [overflow-wrap:anywhere]">
                            {prettySchema(tool.inputSchema)}
                          </code>
                        ) : null}
                      </div>
                    ))}
                  </section>
                  <form
                    className="min-w-0 space-y-3 rounded-lg border border-border bg-card p-3"
                    onSubmit={(event) => {
                      event.preventDefault();
                      void executeTool(server.id);
                    }}
                  >
                    <label className="block text-sm font-medium">
                      Tool name
                      <input
                        className="mt-2 min-h-[44px] w-full rounded-lg border border-input bg-input/30 px-3 py-2 text-sm text-foreground"
                        onChange={(event) => setToolName(event.target.value)}
                        type="text"
                        value={toolName || toolState.selectedToolName || ''}
                      />
                    </label>
                    <label className="block text-sm font-medium">
                      Tool arguments JSON
                      <textarea
                        className="mt-2 min-h-24 w-full rounded-lg border border-input bg-input/30 px-3 py-2 font-mono text-sm text-foreground"
                        onChange={(event) => setArgsJson(event.target.value)}
                        value={argsJson}
                      />
                    </label>
                    <button
                      className="inline-flex min-h-[44px] items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground disabled:cursor-not-allowed disabled:opacity-50"
                      disabled={loadingAction === `execute:${server.id}` || (toolName || toolState.selectedToolName || '').trim() === ''}
                      type="submit"
                    >
                      {loadingAction === `execute:${server.id}` ? <RiLoader4Line className="size-4 animate-spin" aria-hidden="true" /> : <RiPlayLine className="size-4" aria-hidden="true" />}
                      Execute test call
                    </button>
                    {toolState.result ? (
                      <output className="block max-w-full whitespace-pre-wrap break-words rounded-lg border border-border bg-muted/30 p-3 text-sm text-foreground [overflow-wrap:anywhere]">
                        {toolState.result.content}
                      </output>
                    ) : null}
                  </form>
                </div>
              ) : null}
            </article>
          );
        })}
      </div>
    </section>
  );
}
