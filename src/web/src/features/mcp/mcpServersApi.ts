import {
  addMcpServerOperationContract,
  connectMcpServerOperationContract,
  deleteMcpServerOperationContract,
  disconnectMcpServerOperationContract,
  executeMcpServerToolOperationContract,
  getMcpServerStatusOperationContract,
  listLocalMcpServersOperationContract,
  listMcpServersOperationContract,
  listMcpServerToolsOperationContract,
  type OperationContractMetadataV1
} from '@/generated/operation-contracts.generated';
import {
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneRequestEncoder,
  type HttpClient,
  type OperationTransportContract
} from '../../services/http/client';

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

function jsonTransport<T>(
  operation: OperationContractMetadataV1,
  status = 200
): OperationTransportContract<T> {
  return {
    operation,
    requestEncoder: operation.request.mediaType === null
      ? noneRequestEncoder(operation)
      : jsonRequestEncoder(operation),
    responseDecoder: jsonEnvelopeDecoder<T>(operation, status)
  };
}

function addServerPayload(payload: AddMcpServerRequest): AddMcpServerRequest {
  return {
    name: payload.name,
    url: payload.url,
    ...(payload.authToken === undefined ? {} : { authToken: payload.authToken })
  };
}

function executeToolPayload(payload: ExecuteMcpToolRequest): ExecuteMcpToolRequest {
  return {
    toolName: payload.toolName,
    ...(payload.args === undefined ? {} : { args: payload.args })
  };
}

const addServerTransport = jsonTransport<McpServer>(addMcpServerOperationContract, 201);
const connectServerTransport = jsonTransport<McpServer>(connectMcpServerOperationContract);
const deleteServerTransport = jsonTransport<McpActionStatus>(deleteMcpServerOperationContract);
const disconnectServerTransport = jsonTransport<McpActionStatus>(disconnectMcpServerOperationContract);
const executeToolTransport = jsonTransport<McpToolResult>(executeMcpServerToolOperationContract);
const getServerStatusTransport = jsonTransport<McpActionStatus>(getMcpServerStatusOperationContract);
const listLocalServersTransport = jsonTransport<LocalMcpServer[]>(listLocalMcpServersOperationContract);
const listServerToolsTransport = jsonTransport<McpToolDefinition[]>(listMcpServerToolsOperationContract);
const listServersTransport = jsonTransport<McpServer[]>(listMcpServersOperationContract);

export function createMcpServersApi(client: HttpClient): McpServersApi {
  const collectionPath = '/api/v1/app/mcp-servers';
  const serverPath = (serverId: string) => `${collectionPath}/${serverId}`;

  return {
    addServer: (payload) => client.post<McpServer>(collectionPath, addServerPayload(payload), undefined, addServerTransport),
    connectServer: (serverId) => client.post<McpServer>(`${serverPath(serverId)}/connect`, undefined, undefined, connectServerTransport),
    deleteServer: (serverId) => client.delete<McpActionStatus>(serverPath(serverId), undefined, deleteServerTransport),
    disconnectServer: (serverId) => client.post<McpActionStatus>(`${serverPath(serverId)}/disconnect`, undefined, undefined, disconnectServerTransport),
    executeTool: (serverId, payload) => client.post<McpToolResult>(`${serverPath(serverId)}/execute`, executeToolPayload(payload), undefined, executeToolTransport),
    getServerStatus: (serverId) => client.get<McpActionStatus>(`${serverPath(serverId)}/status`, undefined, getServerStatusTransport),
    listLocalServers: () => client.get<LocalMcpServer[]>('/api/v1/app/mcp-local-servers', undefined, listLocalServersTransport),
    listServerTools: (serverId) => client.get<McpToolDefinition[]>(`${serverPath(serverId)}/tools`, undefined, listServerToolsTransport),
    listServers: () => client.get<McpServer[]>(collectionPath, undefined, listServersTransport)
  };
}
