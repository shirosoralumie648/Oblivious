import type { HttpClient } from '../../services/http/client';

export type McpServerStatus = 'connected' | 'disconnected' | 'error' | string;

export type McpServer = {
  id: string;
  organizationId?: string;
  userId?: string;
  name: string;
  url: string;
  authToken?: string;
  hasAuthToken?: boolean;
  status: McpServerStatus;
  lastConnectedAt?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type AddMcpServerRequest = {
  name: string;
  url: string;
  authToken?: string;
};

export type McpToolDefinition = {
  name: string;
  description?: string;
  inputSchema?: unknown;
};

export type ExecuteMcpToolRequest = {
  toolName: string;
  args?: Record<string, unknown>;
};

export type McpToolResult = {
  content: string;
  isError?: boolean;
};

export type McpActionStatus = {
  status: string;
};

export type LocalMcpServer = {
  id: string;
  name: string;
  description?: string;
  toolCount: number;
};

export type McpServersApi = {
  addServer: (payload: AddMcpServerRequest) => Promise<McpServer>;
  connectServer: (serverId: string) => Promise<McpServer>;
  deleteServer: (serverId: string) => Promise<McpActionStatus>;
  disconnectServer: (serverId: string) => Promise<McpActionStatus>;
  executeTool: (serverId: string, payload: ExecuteMcpToolRequest) => Promise<McpToolResult>;
  getServerStatus: (serverId: string) => Promise<McpActionStatus>;
  listLocalServers: () => Promise<LocalMcpServer[]>;
  listServerTools: (serverId: string) => Promise<McpToolDefinition[]>;
  listServers: () => Promise<McpServer[]>;
};

export function createMcpServersApi(client: HttpClient): McpServersApi {
  const collectionPath = '/api/v1/app/mcp-servers';
  const serverPath = (serverId: string) => `${collectionPath}/${serverId}`;

  return {
    addServer: (payload) => client.post<McpServer>(collectionPath, payload),
    connectServer: (serverId) => client.post<McpServer>(`${serverPath(serverId)}/connect`),
    deleteServer: (serverId) => client.delete<McpActionStatus>(serverPath(serverId)),
    disconnectServer: (serverId) => client.post<McpActionStatus>(`${serverPath(serverId)}/disconnect`),
    executeTool: (serverId, payload) => client.post<McpToolResult>(`${serverPath(serverId)}/execute`, payload),
    getServerStatus: (serverId) => client.get<McpActionStatus>(`${serverPath(serverId)}/status`),
    listLocalServers: () => client.get<LocalMcpServer[]>('/api/v1/app/mcp-local-servers'),
    listServerTools: (serverId) => client.get<McpToolDefinition[]>(`${serverPath(serverId)}/tools`),
    listServers: () => client.get<McpServer[]>(collectionPath)
  };
}
