import type { HttpClient } from '../../services/http/client';
import type { AgentDetail, AgentSummary } from '../../types/api';
import type { AgentPlanStep, AgentToolRun } from './planStepsApi';

export type AgentToolDefinition = {
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
  return {
    ...tool,
    requiresApproval: tool.requiresApproval ?? false,
    riskLevel: tool.riskLevel ?? 'safe',
    toolType: tool.toolType ?? 'builtin'
  };
}

export function createAgentsApi(client: HttpClient): AgentsApi {
  const path = '/api/v1/app/agents';

  return {
    createAgent: (payload) => client.post<AgentDetail>(path, payload),
    deleteAgent: async (agentId) => {
      await client.delete<unknown>(`${path}/${encodeURIComponent(agentId)}`);
    },
    createRun: async (payload) =>
      createdRunFromPayload(await client.post<AgentRunPayload>('/api/v1/agent/runs', createRunPayload(payload))),
    getAgent: (agentId) => client.get<AgentDetail>(`${path}/${encodeURIComponent(agentId)}`),
    getAgentTools: async (agentId) =>
      (await client.get<Array<Partial<AgentToolDefinition> & { name: string }>>(`${path}/${encodeURIComponent(agentId)}/tools`)).map(normalizeToolDefinition),
    listAgents: () => client.get<AgentSummary[]>(path),
    updateAgent: (agentId, payload) => client.put<AgentDetail>(`${path}/${encodeURIComponent(agentId)}`, payload)
  };
}
