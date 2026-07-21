import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createAgentsApi } from './agentsApi';

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
      : vi.fn()
  };
  return client;
}

describe('createAgentsApi', () => {
  it('lists workspace agents from the app agent endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        config: { approvalMode: 'tiered' },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      }
    ]);
    const api = createAgentsApi(createClient({ get }));

    await expect(api.listAgents()).resolves.toEqual([
      {
        config: { approvalMode: 'tiered' },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      }
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/app/agents');
  });

  it('creates an agent through the app agent endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
      description: 'Research and summarize workspace materials.',
      id: 'agent_created',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Builder',
      systemPrompt: 'Prefer cited answers.',
      tools: []
    });
    const api = createAgentsApi(createClient({ post }));

    await expect(
      api.createAgent({
        config: {
          approvalMode: 'tiered',
          defaultExecutionMode: 'react',
          maxSkills: 1,
          modelRoutingRules: [{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }],
          skills: [{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }]
        },
        description: 'Research and summarize workspace materials.',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Builder',
        systemPrompt: 'Prefer cited answers.',
        tools: []
      })
    ).resolves.toMatchObject({
      id: 'agent_created',
      name: 'Research Builder'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/app/agents', {
      config: {
        approvalMode: 'tiered',
        defaultExecutionMode: 'react',
        maxSkills: 1,
        modelRoutingRules: [{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }],
        skills: [{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }]
      },
      description: 'Research and summarize workspace materials.',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Builder',
      systemPrompt: 'Prefer cited answers.',
      tools: []
    });
  });

  it('gets and deletes a single agent through the app agent endpoint', async () => {
    const get = vi.fn().mockResolvedValue({
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: []
    });
    const deleteRequest = vi.fn().mockResolvedValue({ status: 'deleted' });
    const api = createAgentsApi(createClient({ delete: deleteRequest, get }));

    await expect(api.getAgent('agent_1')).resolves.toMatchObject({ id: 'agent_1', name: 'Research Agent' });
    await expect(api.deleteAgent('agent_1')).resolves.toBeUndefined();

    expect(get).toHaveBeenCalledWith('/api/v1/app/agents/agent_1');
    expect(deleteRequest).toHaveBeenCalledWith('/api/v1/app/agents/agent_1');
  });

  it('updates approval policy without dropping the existing agent fields', async () => {
    const put = vi.fn().mockResolvedValue({
      config: {
        approvalMode: 'custom',
        maxSkills: 2,
        modelRoutingRules: [{ keywords: ['debug'], targetModel: 'gpt-4o' }],
        skills: [{ instructions: 'Use debugging workflow.', name: 'Debugger', triggers: ['debug'] }],
        toolApprovalOverrides: {
          web_search: { requiresApproval: true, riskLevel: 'medium' }
        }
      },
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
    });
    const api = createAgentsApi(createClient({ put }));

    await expect(
      api.updateAgent('agent_1', {
        config: {
          approvalMode: 'custom',
          maxSkills: 2,
          modelRoutingRules: [{ keywords: ['debug'], targetModel: 'gpt-4o' }],
          skills: [{ instructions: 'Use debugging workflow.', name: 'Debugger', triggers: ['debug'] }],
          toolApprovalOverrides: {
            web_search: { requiresApproval: true, riskLevel: 'medium' }
          }
        },
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      })
    ).resolves.toMatchObject({
      config: {
        approvalMode: 'custom',
        maxSkills: 2,
        modelRoutingRules: [{ keywords: ['debug'], targetModel: 'gpt-4o' }],
        skills: [{ instructions: 'Use debugging workflow.', name: 'Debugger', triggers: ['debug'] }],
        toolApprovalOverrides: {
          web_search: { requiresApproval: true, riskLevel: 'medium' }
        }
      },
      id: 'agent_1'
    });

    expect(put).toHaveBeenCalledWith('/api/v1/app/agents/agent_1', {
      config: {
        approvalMode: 'custom',
        maxSkills: 2,
        modelRoutingRules: [{ keywords: ['debug'], targetModel: 'gpt-4o' }],
        skills: [{ instructions: 'Use debugging workflow.', name: 'Debugger', triggers: ['debug'] }],
        toolApprovalOverrides: {
          web_search: { requiresApproval: true, riskLevel: 'medium' }
        }
      },
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
    });
  });

  it('loads available tool definitions for an agent', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        capabilityId: 'mcp.network_execution',
        description: 'Search workspace and web sources',
        inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
        name: 'web_search'
      }
    ]);
    const api = createAgentsApi(createClient({ get }));

    await expect(api.getAgentTools('agent_1')).resolves.toEqual([
      {
        capabilityId: 'mcp.network_execution',
        description: 'Search workspace and web sources',
        inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
        name: 'web_search',
        requiresApproval: false,
        riskLevel: 'safe',
        toolType: 'builtin'
      }
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/app/agents/agent_1/tools');
  });

  it('normalizes available tool approval metadata with legacy-safe defaults', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        capabilityId: 'mcp.network_execution',
        description: 'Search workspace and web sources',
        inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
        name: 'web_search',
        requiresApproval: true,
        riskLevel: 'medium',
        toolType: 'builtin'
      },
      {
        capabilityId: 'mcp.tool_execution',
        description: 'Legacy MCP response without approval metadata',
        name: 'legacy_mcp_tool'
      }
    ]);
    const api = createAgentsApi(createClient({ get }));

    await expect(api.getAgentTools('agent_1')).resolves.toEqual([
      {
        capabilityId: 'mcp.network_execution',
        description: 'Search workspace and web sources',
        inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
        name: 'web_search',
        requiresApproval: true,
        riskLevel: 'medium',
        toolType: 'builtin'
      },
      {
        capabilityId: 'mcp.tool_execution',
        description: 'Legacy MCP response without approval metadata',
        name: 'legacy_mcp_tool',
        requiresApproval: false,
        riskLevel: 'safe',
        toolType: 'builtin'
      }
    ]);
  });

  it('preserves response capability identity and omits it from tool mutations', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        capabilityId: 'mcp.network_execution',
        name: 'web_search',
        toolType: 'builtin'
      }
    ]);
    const put = vi.fn().mockResolvedValue({ id: 'agent_1' });
    const api = createAgentsApi(createClient({ get, put }));

    await expect(api.getAgentTools('agent_1')).resolves.toMatchObject([
      { capabilityId: 'mcp.network_execution', name: 'web_search' }
    ]);
    await api.updateAgent('agent_1', {
      tools: [
        {
          capabilityId: 'caller.must.not.send',
          enabled: true,
          name: 'web_search',
          type: 'builtin'
        } as never
      ]
    });

    expect(put).toHaveBeenCalledWith('/api/v1/app/agents/agent_1', {
      tools: [{ enabled: true, name: 'web_search', type: 'builtin' }]
    });
  });

  it('starts an agent run from the agent runs endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'run_1',
      planSteps: [{ id: 'step_1', status: 'pending', title: 'Inspect workspace' }],
      status: 'running',
      toolRuns: []
    });
    const api = createAgentsApi(createClient({ post }));

    await expect(
      api.createRun({
        agentId: 'agent_1',
        conversationId: 'conversation_1',
        input: 'Audit the workspace release checklist',
        maxIterations: 20,
        mode: 'planning',
        tokenBudget: 50000
      })
    ).resolves.toEqual({
      id: 'run_1',
      planSteps: [{ id: 'step_1', status: 'pending', title: 'Inspect workspace' }],
      status: 'running',
      toolRuns: []
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs', {
      agentId: 'agent_1',
      conversationId: 'conversation_1',
      input: 'Audit the workspace release checklist',
      maxIterations: 20,
      mode: 'planning',
      tokenBudget: 50000
    });
  });

  it('normalizes prompt to the backend input field when starting a run', async () => {
    const post = vi.fn().mockResolvedValue({ id: 'run_1', status: 'running' });
    const api = createAgentsApi(createClient({ post }));

    await api.createRun({
      agentId: 'agent_1',
      conversationId: 'conversation_1',
      mode: 'react',
      prompt: 'Summarize the open workspace tasks'
    });

    expect(post).toHaveBeenCalledWith('/api/v1/agent/runs', {
      agentId: 'agent_1',
      conversationId: 'conversation_1',
      input: 'Summarize the open workspace tasks',
      mode: 'react'
    });
  });
});
