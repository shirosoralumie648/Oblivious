import type { HttpClient } from '../../services/http/client';

export type AgentMemory = {
  id: string;
  organizationId?: string;
  userId?: string;
  agentId?: string;
  type: string;
  content: string;
  importance?: number;
  metadata?: Record<string, unknown>;
  expiresAt?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type CreateAgentMemoryRequest = {
  agentId?: string;
  content: string;
  importance?: number;
  metadata?: Record<string, unknown>;
  type?: string;
};

export type UpdateAgentMemoryRequest = {
  content?: string;
  importance?: number;
};

export type SearchAgentMemoriesParams = {
  agentId?: string;
  limit?: number;
  offset?: number;
  query: string;
  topK?: number;
  type?: string;
};

export type AgentMemoriesResponse = {
  data: AgentMemory[];
  total: number;
};

type AgentMemoriesPayload = AgentMemory[] | {
  data?: AgentMemory[];
  memories?: AgentMemory[];
  total?: number;
};

type QueryValue = string | number | undefined | null;

function buildQuery(params: Record<string, QueryValue>) {
  const query = new URLSearchParams();

  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value));
    }
  });

  const serialized = query.toString();
  return serialized ? `?${serialized}` : '';
}

function toMemoriesResponse(payload: AgentMemoriesPayload): AgentMemoriesResponse {
  if (Array.isArray(payload)) {
    return {
      data: payload,
      total: payload.length
    };
  }

  const memories = payload.memories ?? payload.data ?? [];
  return {
    data: memories,
    total: payload.total ?? memories.length
  };
}

export type AgentMemoriesApi = {
  createMemory: (payload: CreateAgentMemoryRequest) => Promise<AgentMemory>;
  deleteMemory: (memoryId: string) => Promise<void>;
  importMemories: (payload: CreateAgentMemoryRequest[]) => Promise<AgentMemory[]>;
  searchMemories: (params: SearchAgentMemoriesParams) => Promise<AgentMemoriesResponse>;
  updateMemory: (memoryId: string, payload: UpdateAgentMemoryRequest) => Promise<AgentMemory>;
};

export function createAgentMemoriesApi(client: HttpClient): AgentMemoriesApi {
  const path = '/api/v1/agent/memories';

  return {
    createMemory: (payload) => client.post<AgentMemory>(path, payload),
    deleteMemory: (memoryId) => client.delete<void>(`${path}/${encodeURIComponent(memoryId)}`),
    importMemories: (payload) => client.post<AgentMemory[]>(`${path}/import`, { memories: payload }),
    searchMemories: async (params) =>
      toMemoriesResponse(
        await client.get<AgentMemoriesPayload>(
          `${path}${buildQuery({
            agentId: params.agentId,
            limit: params.limit,
            offset: params.offset,
            query: params.query,
            topK: params.topK,
            type: params.type
          })}`
        )
      ),
    updateMemory: (memoryId, payload) =>
      client.request<AgentMemory>(`${path}/${encodeURIComponent(memoryId)}`, {
        body: JSON.stringify(payload),
        method: 'PATCH'
      })
  };
}
