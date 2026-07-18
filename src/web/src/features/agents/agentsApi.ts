import type { HttpClient } from '../../services/http/client';
import type { AgentDetail, AgentSummary } from '../../types/api';
import type { AgentPlanStep, AgentToolRun } from './planStepsApi';

export type AgentToolDefinition = {
  capabilityId: string;
  name: string;
  description?: string;
  inputSchema?: unknown;
  toolType: 'builtin' | 'mcp' | 'custom' | string;
  requiresApproval: boolean;
  riskLevel: 'safe' | 'medium' | 'dangerous' | string;
};

export type UpdateAgentRequest = {
  config?: AgentSummary['config'];
  description?: string;
  isPublic?: boolean;
  model?: string;
  name?: string;
  systemPrompt?: string;
  tools?: AgentSummary['tools'];
};

export type CreateAgentRequest = {
  config?: AgentSummary['config'];
  description?: string;
  isPublic?: boolean;
  model?: string;
  name: string;
  systemPrompt?: string;
  tools?: AgentSummary['tools'];
};

export type CreateAgentRunRequest = {
  agentId: string;
  conversationId: string;
  input?: string;
  prompt?: string;
  mode?: 'react' | 'planning' | string;
  maxIterations?: number;
  tokenBudget?: number;
};

type AgentRunRecord = {
  id?: string;
  status?: string;
};

type AgentRunPayload = {
  id?: string;
  run?: AgentRunRecord | null;
  status?: string;
  planSteps?: AgentPlanStep[];
  toolRuns?: AgentToolRun[];
};

export type CreatedAgentRun = {
  id: string;
  status: string;
  planSteps: AgentPlanStep[];
  toolRuns: AgentToolRun[];
};

export type AgentsApi = {
  createAgent: (payload: CreateAgentRequest) => Promise<AgentDetail>;
  deleteAgent: (agentId: string) => Promise<void>;
  createRun: (payload: CreateAgentRunRequest) => Promise<CreatedAgentRun>;
  getAgent: (agentId: string) => Promise<AgentDetail>;
  getAgentTools: (agentId: string) => Promise<AgentToolDefinition[]>;
  listAgents: () => Promise<AgentSummary[]>;
  updateAgent: (agentId: string, payload: UpdateAgentRequest) => Promise<AgentDetail>;
};

function createdRunFromPayload(payload: AgentRunPayload): CreatedAgentRun {
  return {
    id: payload.id ?? payload.run?.id ?? '',
    planSteps: payload.planSteps ?? [],
    status: payload.status ?? payload.run?.status ?? '',
    toolRuns: payload.toolRuns ?? []
  };
}

function createRunPayload(payload: CreateAgentRunRequest) {
  return {
    agentId: payload.agentId,
    conversationId: payload.conversationId,
    input: payload.input ?? payload.prompt ?? '',
    ...(payload.mode !== undefined ? { mode: payload.mode } : {}),
    ...(payload.maxIterations !== undefined ? { maxIterations: payload.maxIterations } : {}),
    ...(payload.tokenBudget !== undefined ? { tokenBudget: payload.tokenBudget } : {})
  };
}

function normalizeToolDefinition(tool: Partial<AgentToolDefinition> & { name: string }): AgentToolDefinition {
  if (typeof tool.capabilityId !== 'string' || tool.capabilityId.trim() === '') {
    throw new Error('Agent tool definition is missing server capabilityId');
  }
  const normalized: AgentToolDefinition = {
    capabilityId: tool.capabilityId,
    name: tool.name,
    requiresApproval: tool.requiresApproval ?? false,
    riskLevel: tool.riskLevel ?? 'safe',
    toolType: tool.toolType ?? 'builtin'
  };
  if (tool.description !== undefined) normalized.description = tool.description;
  if (tool.inputSchema !== undefined) normalized.inputSchema = tool.inputSchema;
  return normalized;
}

function serializeToolMutation(tool: NonNullable<AgentSummary['tools']>[number]) {
  const result: Record<string, unknown> = { name: tool.name };
  const fields = ['description', 'type', 'serverId', 'enabled', 'requiresApproval', 'riskLevel', 'inputSchema', 'runtime', 'sourceCode', 'timeoutSeconds'] as const;
  for (const field of fields) {
    const value = tool[field];
    if (value !== undefined) result[field] = value;
  }
  return result;
}

function serializeAgentMutation(payload: CreateAgentRequest | UpdateAgentRequest) {
  const result: Record<string, unknown> = {};
  const fields = ['config', 'description', 'isPublic', 'model', 'name', 'systemPrompt'] as const;
  for (const field of fields) {
    const value = payload[field];
    if (value !== undefined) result[field] = value;
  }
  if (payload.tools !== undefined) {
    result.tools = payload.tools.map(serializeToolMutation);
  }
  return result;
}

export function createAgentsApi(client: HttpClient): AgentsApi {
  const path = '/api/v1/app/agents';

  return {
    createAgent: (payload) => client.post<AgentDetail>(path, serializeAgentMutation(payload)),
    deleteAgent: async (agentId) => {
      await client.delete<unknown>(`${path}/${encodeURIComponent(agentId)}`);
    },
    createRun: async (payload) =>
      createdRunFromPayload(await client.post<AgentRunPayload>('/api/v1/agent/runs', createRunPayload(payload))),
    getAgent: (agentId) => client.get<AgentDetail>(`${path}/${encodeURIComponent(agentId)}`),
    getAgentTools: async (agentId) =>
      (await client.get<Array<Partial<AgentToolDefinition> & { name: string }>>(`${path}/${encodeURIComponent(agentId)}/tools`)).map(normalizeToolDefinition),
    listAgents: () => client.get<AgentSummary[]>(path),
    updateAgent: (agentId, payload) => client.put<AgentDetail>(`${path}/${encodeURIComponent(agentId)}`, serializeAgentMutation(payload))
  };
}
