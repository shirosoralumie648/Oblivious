import {
  eventSourceOperationContract,
  getAppReadinessCapabilitiesOperationContract,
  listUsersOperationContract,
  noContentOperationContract,
  rawOperationContract,
  socketOperationContract,
  streamOperationContract,
  textOperationContract,
  uploadOperationContract
} from './generated/client.generated';

type Transport = { operation: unknown; requestEncoder: unknown; responseDecoder: unknown };
type HttpClient = { get: (...args: unknown[]) => Promise<unknown>; post: (...args: unknown[]) => Promise<unknown>; delete: (...args: unknown[]) => Promise<unknown> };
declare const client: HttpClient;
declare function noneRequestEncoder(operation: unknown): unknown;
declare function jsonRequestEncoder(operation: unknown): unknown;
declare function formDataRequestEncoder(operation: unknown): unknown;
declare function rawRequestEncoder(operation: unknown): unknown;
declare function jsonEnvelopeDecoder<T>(operation: unknown, status: number): unknown;
declare function rawResponseDecoder(operation: unknown, status: number): unknown;
declare function textResponseDecoder(operation: unknown, status: number): unknown;
declare function noneResponseDecoder(operation: unknown, status: number): unknown;
declare function streamText(path: string, onChunk: (chunk: string) => void, operation: unknown, contract: Transport): Promise<void>;
declare function uploadFile(path: string, file: File, operation: unknown, contract: Transport): Promise<Response>;
declare function useSWR<T>(key: readonly unknown[], fetcher: unknown): unknown;
declare function fetchFn(path: string, init: RequestInit, contract: Transport): Promise<Response>;
declare class EventSource { constructor(path: string); }
declare class WebSocket { constructor(path: string); }

const listTransport: Transport = {
  operation: listUsersOperationContract,
  requestEncoder: noneRequestEncoder(listUsersOperationContract),
  responseDecoder: jsonEnvelopeDecoder(listUsersOperationContract, 200)
};
const projectionTransport: Transport = {
  operation: getAppReadinessCapabilitiesOperationContract,
  requestEncoder: noneRequestEncoder(getAppReadinessCapabilitiesOperationContract),
  responseDecoder: jsonEnvelopeDecoder(getAppReadinessCapabilitiesOperationContract, 200)
};
const uploadTransport: Transport = {
  operation: uploadOperationContract,
  requestEncoder: formDataRequestEncoder(uploadOperationContract),
  responseDecoder: jsonEnvelopeDecoder(uploadOperationContract, 200)
};
const rawTransport: Transport = {
  operation: rawOperationContract,
  requestEncoder: rawRequestEncoder(rawOperationContract),
  responseDecoder: rawResponseDecoder(rawOperationContract, 200)
};
const textTransport: Transport = {
  operation: textOperationContract,
  requestEncoder: noneRequestEncoder(textOperationContract),
  responseDecoder: textResponseDecoder(textOperationContract, 200)
};
const noContentTransport: Transport = {
  operation: noContentOperationContract,
  requestEncoder: noneRequestEncoder(noContentOperationContract),
  responseDecoder: noneResponseDecoder(noContentOperationContract, 204)
};
const streamTransport: Transport = {
  operation: streamOperationContract,
  requestEncoder: jsonRequestEncoder(streamOperationContract),
  responseDecoder: rawResponseDecoder(streamOperationContract, 200)
};
const socketTransport: Transport = {
  operation: socketOperationContract,
  requestEncoder: noneRequestEncoder(socketOperationContract),
  responseDecoder: rawResponseDecoder(socketOperationContract, 101)
};

client.get('/fixture/users', undefined, listTransport);
client.get('/fixture/users', undefined, listTransport);
streamText('/fixture/stream', () => undefined, streamOperationContract, streamTransport);
uploadFile('/fixture/upload', new File([], 'fixture.txt'), uploadOperationContract, uploadTransport);
client.post('/fixture/raw', new Uint8Array(), undefined, rawTransport);
client.get('/fixture/text', undefined, textTransport);
client.delete('/fixture/items/item_1', undefined, noContentTransport);
fetchFn('/fixture/users', { method: 'GET' }, listTransport);
useSWR([' /fixture/users', listUsersOperationContract, listTransport], () => undefined);
new EventSource(eventSourceOperationContract.normalizedPath);
new WebSocket(socketOperationContract.normalizedPath);

export const fixtureExposure = { path: '/fixture/users', capabilityId: 'fixture.users' };
export const fixtureConditionalExposure = { path: '/fixture/conditional', capabilityId: 'fixture.conditional' };

export type ModelOption = {
  id: string;
  label: string;
  capabilityId: string;
};

export type AgentToolDefinition = {
  capabilityId: string;
  name: string;
};

type UpdateConversationConfigRequest = {
  modelId: string;
};

type AppCapabilityProjectionResponse = {
  capabilities: readonly { capabilityId: string }[];
};

type AgentTool = {
  enabled: boolean;
  name: string;
};

type CreateAgentRequest = {
  name: string;
  tools?: AgentTool[];
};

type UpdateAgentRequest = {
  name?: string;
  tools?: AgentTool[];
};

declare const releaseProjection: { isCapabilityEnabled(capabilityId: string): boolean };
declare const modelOption: ModelOption;
declare const agentToolDefinition: AgentToolDefinition;
declare function useAppContext(): { authState: { status: string } };

function createReleaseProjectionApi(): { load(): Promise<AppCapabilityProjectionResponse> } {
  return {
    load: () => client.get('/fixture/app-projection', undefined, projectionTransport) as Promise<AppCapabilityProjectionResponse>
  };
}

releaseProjection.isCapabilityEnabled(modelOption.capabilityId);
releaseProjection.isCapabilityEnabled(agentToolDefinition.capabilityId);

export function ReleaseProjectionProvider({ children }: { children: unknown }) {
  const { authState } = useAppContext();
  if (authState.status !== 'authenticated') return null;
  void createReleaseProjectionApi().load();
  return children;
}

function conversationConfigRequest(model: ModelOption): UpdateConversationConfigRequest {
  return { modelId: model.id };
}

function toolFromCatalogDefinition(tool: AgentToolDefinition): AgentTool {
  return { enabled: true, name: tool.name };
}

function serializeToolMutation(tool: AgentTool): Record<string, unknown> {
  const fields = ['enabled', 'name'] as const;
  const result: Record<string, unknown> = {};
  for (const field of fields) result[field] = tool[field];
  return result;
}

function serializeAgentMutation(payload: CreateAgentRequest | UpdateAgentRequest): Record<string, unknown> {
  const fields = ['name'] as const;
  const result: Record<string, unknown> = {};
  for (const field of fields) {
    if (payload[field] !== undefined) result[field] = payload[field];
  }
  if (payload.tools !== undefined) result.tools = payload.tools.map(serializeToolMutation);
  return result;
}

void conversationConfigRequest;
void toolFromCatalogDefinition;
void serializeAgentMutation;
