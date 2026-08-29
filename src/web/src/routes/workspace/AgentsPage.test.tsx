import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const listAgents = vi.fn();
const createAgent = vi.fn();
const getAgentTools = vi.fn();
const createRun = vi.fn();
const updateAgent = vi.fn();

vi.mock('../../features/agents/agentsApi', () => ({
  createAgentsApi: () => ({
    createAgent,
    createRun,
    getAgentTools,
    listAgents,
    updateAgent
  })
}));

vi.mock('../../features/releaseProjection/releaseProjection', () => ({
  useReleaseProjection: () => ({
    isCapabilityEnabled: (capabilityId: string) => capabilityId === 'mcp.network_execution'
  })
}));

import { AgentsPage } from './AgentsPage';

function renderAgentsPage() {
  return render(
    <MemoryRouter >
      <AgentsPage />
    </MemoryRouter>
  );
}

describe('AgentsPage', () => {
  beforeEach(() => {
    createAgent.mockReset();
    createRun.mockReset();
    getAgentTools.mockReset();
    listAgents.mockReset();
    updateAgent.mockReset();
  });

  it('loads agents and renders their approval policy controls', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: {
          approvalMode: 'tiered',
          toolApprovalOverrides: {
            web_search: { requiresApproval: true, riskLevel: 'medium' }
          }
        },
        description: 'Answers research requests.',
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [
          { enabled: true, name: 'web_search', requiresApproval: true, riskLevel: 'medium', type: 'builtin' },
          { enabled: true, name: 'write_file', riskLevel: 'dangerous', type: 'builtin' }
        ]
      }
    ]);

    renderAgentsPage();

    expect(await screen.findByRole('heading', { name: 'Agents' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Research Agent' })).toBeInTheDocument();
    expect(screen.getByLabelText('Approval mode')).toHaveValue('tiered');
    expect(screen.getByText('web_search')).toBeInTheDocument();
    expect(screen.getByText('write_file')).toBeInTheDocument();

    const webSearchRow = screen.getByLabelText('Tool policy web_search');
    expect(within(webSearchRow).getByLabelText('Require approval for web_search')).toBeChecked();
    expect(within(webSearchRow).getByLabelText('Risk level for web_search')).toHaveValue('medium');
  });

  it('creates a new agent from the workspace page', async () => {
    listAgents.mockResolvedValueOnce([]);
    createAgent.mockResolvedValueOnce({
      config: {
        approvalMode: 'all',
        defaultExecutionMode: 'planning',
        longTermMemoryExtractionPolicy: 'llm_assisted',
        longTermMemoryUpdatePolicy: 'memory_key_consolidate',
        longTermMemoryWritePolicy: 'explicit_only',
        maxSkills: 1,
        maxIterations: 30,
        modelRoutingRules: [{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }],
        skills: [{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }],
        tokenBudget: 60000
      },
      description: 'Research and summarize workspace materials.',
      id: 'agent_created',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Builder',
      systemPrompt: 'Prefer cited answers.',
      tools: []
    });

    renderAgentsPage();

    expect(await screen.findByText('No agents configured.')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Create agent' }));
    fireEvent.change(screen.getByLabelText('Agent name'), { target: { value: 'Research Builder' } });
    fireEvent.change(screen.getByLabelText('Model'), { target: { value: 'gpt-4o-mini' } });
    fireEvent.change(screen.getByLabelText('Description'), { target: { value: 'Research and summarize workspace materials.' } });
    fireEvent.change(screen.getByLabelText('System prompt'), { target: { value: 'Prefer cited answers.' } });
    fireEvent.change(screen.getByLabelText('Approval mode'), { target: { value: 'all' } });
    fireEvent.change(screen.getByLabelText('Default execution mode'), { target: { value: 'planning' } });
    fireEvent.change(screen.getByLabelText('Long-term memory writes'), { target: { value: 'explicit_only' } });
    fireEvent.change(screen.getByLabelText('Long-term memory extraction'), { target: { value: 'llm_assisted' } });
    fireEvent.change(screen.getByLabelText('Long-term memory update'), { target: { value: 'memory_key_consolidate' } });
    fireEvent.change(screen.getByLabelText('Max iterations'), { target: { value: '30' } });
    fireEvent.change(screen.getByLabelText('Token budget'), { target: { value: '60000' } });
    fireEvent.change(screen.getByLabelText('Max skills'), { target: { value: '1' } });
    fireEvent.change(screen.getByLabelText('Model routing rules JSON'), {
      target: { value: JSON.stringify([{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }]) }
    });
    fireEvent.change(screen.getByLabelText('Skills JSON'), {
      target: {
        value: JSON.stringify([
          { instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }
        ])
      }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save agent' }));

    await waitFor(() => {
      expect(createAgent).toHaveBeenCalledWith({
        config: {
          approvalMode: 'all',
          defaultExecutionMode: 'planning',
          longTermMemoryExtractionPolicy: 'llm_assisted',
          longTermMemoryUpdatePolicy: 'memory_key_consolidate',
          longTermMemoryWritePolicy: 'explicit_only',
          maxSkills: 1,
          maxIterations: 30,
          modelRoutingRules: [{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }],
          skills: [{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }],
          tokenBudget: 60000
        },
        description: 'Research and summarize workspace materials.',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Builder',
        systemPrompt: 'Prefer cited answers.',
        tools: []
      });
    });

    expect(await screen.findByRole('button', { name: 'Research Builder' })).toBeInTheDocument();
    expect(screen.getByText('Agent created.')).toBeInTheDocument();
  });

  it('saves custom approval overrides for the selected agent', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: { approvalMode: 'tiered' },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [
          { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
          { enabled: true, name: 'write_file', riskLevel: 'dangerous', type: 'builtin' }
        ]
      }
    ]);
    updateAgent.mockResolvedValueOnce({
      config: {
        approvalMode: 'custom',
        toolApprovalOverrides: {
          web_search: { requiresApproval: true, riskLevel: 'medium' },
          write_file: { requiresApproval: true, riskLevel: 'dangerous' }
        }
      },
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: [
        { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
        { enabled: true, name: 'write_file', riskLevel: 'dangerous', type: 'builtin' }
      ]
    });

    renderAgentsPage();

    expect(await screen.findByLabelText('Approval mode')).toHaveValue('tiered');

    fireEvent.change(screen.getByLabelText('Approval mode'), { target: { value: 'custom' } });
    fireEvent.click(screen.getByLabelText('Require approval for web_search'));
    fireEvent.click(screen.getByLabelText('Require approval for write_file'));
    fireEvent.click(screen.getByRole('button', { name: 'Save agent policy' }));

    await waitFor(() => {
      expect(updateAgent).toHaveBeenCalledWith('agent_1', {
        config: {
          approvalMode: 'custom',
          longTermMemoryExtractionPolicy: 'deterministic',
          longTermMemoryUpdatePolicy: 'exact_refresh',
          longTermMemoryWritePolicy: 'interaction_and_explicit',
          toolApprovalOverrides: {
            web_search: { requiresApproval: true, riskLevel: 'medium' },
            write_file: { requiresApproval: true, riskLevel: 'dangerous' }
          }
        },
        description: '',
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: '',
        tools: [
          { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
          { enabled: true, name: 'write_file', riskLevel: 'dangerous', type: 'builtin' }
        ]
      });
    });
    expect(screen.getByText('Agent policy saved.')).toBeInTheDocument();
  });

  it('saves execution limits for the selected agent', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: {
          approvalMode: 'tiered',
          defaultExecutionMode: 'planning',
          longTermMemoryExtractionPolicy: 'deterministic',
          longTermMemoryUpdatePolicy: 'exact_refresh',
          longTermMemoryWritePolicy: 'interaction_and_explicit',
          maxSkills: 2,
          maxIterations: 25,
          modelRoutingRules: [{ keywords: ['debug'], targetModel: 'gpt-4o' }],
          skills: [{ instructions: 'Use debugging workflow.', name: 'Debugger', triggers: ['debug'] }],
          tokenBudget: 50000,
          toolApprovalOverrides: {
            internal_tool: { requiresApproval: false, riskLevel: 'safe' },
            web_search: { requiresApproval: true, riskLevel: 'medium' }
          }
        },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      }
    ]);
    updateAgent.mockResolvedValueOnce({
      config: {
        approvalMode: 'tiered',
        defaultExecutionMode: 'react',
        longTermMemoryExtractionPolicy: 'llm_assisted',
        longTermMemoryUpdatePolicy: 'memory_key_consolidate',
        longTermMemoryWritePolicy: 'explicit_only',
        maxSkills: 3,
        maxIterations: 40,
        modelRoutingRules: [{ minInputChars: 2000, targetModel: 'gpt-4.1' }],
        skills: [{ instructions: 'Summarize with citations.', name: 'Summarizer', toolNames: ['web_search'] }],
        tokenBudget: 75000,
        toolApprovalOverrides: {
          internal_tool: { requiresApproval: false, riskLevel: 'safe' },
          web_search: { requiresApproval: true, riskLevel: 'medium' }
        }
      },
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
    });

    renderAgentsPage();

    expect(await screen.findByLabelText('Default execution mode')).toHaveValue('planning');
    expect(screen.getByLabelText('Long-term memory writes')).toHaveValue('interaction_and_explicit');
    expect(screen.getByLabelText('Long-term memory extraction')).toHaveValue('deterministic');
    expect(screen.getByLabelText('Long-term memory update')).toHaveValue('exact_refresh');
    expect(screen.getByLabelText('Max iterations')).toHaveValue(25);
    expect(screen.getByLabelText('Token budget')).toHaveValue(50000);
    expect(screen.getByLabelText('Max skills')).toHaveValue(2);
    expect(screen.getByLabelText('Model routing rules JSON')).toHaveValue(JSON.stringify([{ keywords: ['debug'], targetModel: 'gpt-4o' }], null, 2));
    expect(screen.getByLabelText('Skills JSON')).toHaveValue(JSON.stringify([{ instructions: 'Use debugging workflow.', name: 'Debugger', triggers: ['debug'] }], null, 2));

    fireEvent.change(screen.getByLabelText('Default execution mode'), { target: { value: 'react' } });
    fireEvent.change(screen.getByLabelText('Long-term memory writes'), { target: { value: 'explicit_only' } });
    fireEvent.change(screen.getByLabelText('Long-term memory extraction'), { target: { value: 'llm_assisted' } });
    fireEvent.change(screen.getByLabelText('Long-term memory update'), { target: { value: 'memory_key_consolidate' } });
    fireEvent.change(screen.getByLabelText('Max iterations'), { target: { value: '40' } });
    fireEvent.change(screen.getByLabelText('Token budget'), { target: { value: '75000' } });
    fireEvent.change(screen.getByLabelText('Max skills'), { target: { value: '3' } });
    fireEvent.change(screen.getByLabelText('Model routing rules JSON'), {
      target: { value: JSON.stringify([{ minInputChars: 2000, targetModel: 'gpt-4.1' }]) }
    });
    fireEvent.change(screen.getByLabelText('Skills JSON'), {
      target: { value: JSON.stringify([{ instructions: 'Summarize with citations.', name: 'Summarizer', toolNames: ['web_search'] }]) }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save agent policy' }));

    await waitFor(() => {
      expect(updateAgent).toHaveBeenCalledWith('agent_1', {
        config: {
          approvalMode: 'tiered',
          defaultExecutionMode: 'react',
          longTermMemoryExtractionPolicy: 'llm_assisted',
          longTermMemoryUpdatePolicy: 'memory_key_consolidate',
          longTermMemoryWritePolicy: 'explicit_only',
          maxSkills: 3,
          maxIterations: 40,
          modelRoutingRules: [{ minInputChars: 2000, targetModel: 'gpt-4.1' }],
          skills: [{ instructions: 'Summarize with citations.', name: 'Summarizer', toolNames: ['web_search'] }],
          tokenBudget: 75000,
          toolApprovalOverrides: {
            internal_tool: { requiresApproval: false, riskLevel: 'safe' },
            web_search: { requiresApproval: true, riskLevel: 'medium' }
          }
        },
        description: '',
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: '',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      });
    });
    expect(screen.getByText('Agent policy saved.')).toBeInTheDocument();
  });

  it('adds a custom API tool to the selected agent', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
        description: 'Research and summarize workspace materials.',
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: 'Prefer cited answers.',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      }
    ]);
    updateAgent.mockResolvedValueOnce({
      config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
      description: 'Research and summarize workspace materials.',
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      systemPrompt: 'Prefer cited answers.',
      tools: [
        { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
        {
          description: 'Lookup customer records',
          enabled: true,
          inputSchema: {
            properties: { customer_id: { type: 'string' } },
            required: ['customer_id'],
            type: 'object'
          },
          name: 'crm_lookup',
          requiresApproval: true,
          riskLevel: 'medium',
          serverId: 'https://tools.example.com/crm_lookup',
          type: 'custom'
        }
      ]
    });

    renderAgentsPage();

    expect(await screen.findByRole('button', { name: 'Research Agent' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add custom API tool' }));
    fireEvent.change(screen.getByLabelText('Tool name'), { target: { value: 'crm_lookup' } });
    fireEvent.change(screen.getByLabelText('Endpoint URL'), { target: { value: 'https://tools.example.com/crm_lookup' } });
    fireEvent.change(screen.getByLabelText('Tool description'), { target: { value: 'Lookup customer records' } });
    fireEvent.change(screen.getByLabelText('Tool risk level'), { target: { value: 'medium' } });
    fireEvent.change(screen.getByLabelText('Input schema JSON'), {
      target: {
        value: JSON.stringify({
          properties: { customer_id: { type: 'string' } },
          required: ['customer_id'],
          type: 'object'
        })
      }
    });
    fireEvent.click(screen.getByLabelText('Require approval for custom API tool'));
    fireEvent.click(screen.getByRole('button', { name: 'Save custom API tool' }));

    await waitFor(() => {
      expect(updateAgent).toHaveBeenCalledWith('agent_1', {
        config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
        description: 'Research and summarize workspace materials.',
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: 'Prefer cited answers.',
        tools: [
          { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
          {
            description: 'Lookup customer records',
            enabled: true,
            inputSchema: {
              properties: { customer_id: { type: 'string' } },
              required: ['customer_id'],
              type: 'object'
            },
            name: 'crm_lookup',
            requiresApproval: true,
            riskLevel: 'medium',
            serverId: 'https://tools.example.com/crm_lookup',
            type: 'custom'
          }
        ]
      });
    });
    expect(await screen.findByText('Custom API tool saved.')).toBeInTheDocument();
    expect(screen.getByText('crm_lookup')).toBeInTheDocument();
    expect(screen.getByText('custom / https://tools.example.com/crm_lookup')).toBeInTheDocument();
  });

  it('adds a custom Python tool to the selected agent', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: []
      }
    ]);
    updateAgent.mockResolvedValueOnce({
      config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      tools: [
        {
          description: 'Score inbound leads',
          enabled: true,
          inputSchema: {
            properties: { lead_score: { type: 'number' } },
            type: 'object'
          },
          name: 'lead_score',
          requiresApproval: true,
          riskLevel: 'dangerous',
          runtime: 'python',
          sourceCode: 'def main(args):\n    return {"score": args["lead_score"] * 2}',
          timeoutSeconds: 3,
          type: 'custom'
        }
      ]
    });

    renderAgentsPage();

    expect(await screen.findByRole('button', { name: 'Research Agent' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Add custom API tool' }));
    fireEvent.change(screen.getByLabelText('Tool runtime'), { target: { value: 'python' } });
    fireEvent.change(screen.getByLabelText('Tool name'), { target: { value: 'lead_score' } });
    fireEvent.change(screen.getByLabelText('Tool description'), { target: { value: 'Score inbound leads' } });
    fireEvent.change(screen.getByLabelText('Tool risk level'), { target: { value: 'dangerous' } });
    fireEvent.change(screen.getByLabelText('Input schema JSON'), {
      target: { value: JSON.stringify({ properties: { lead_score: { type: 'number' } }, type: 'object' }) }
    });
    fireEvent.change(screen.getByLabelText('Python source code'), {
      target: { value: 'def main(args):\n    return {"score": args["lead_score"] * 2}' }
    });
    fireEvent.change(screen.getByLabelText('Timeout seconds'), { target: { value: '3' } });
    fireEvent.click(screen.getByLabelText('Require approval for custom API tool'));
    fireEvent.click(screen.getByRole('button', { name: 'Save custom API tool' }));

    await waitFor(() => {
      expect(updateAgent).toHaveBeenCalledWith('agent_1', {
        config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
        description: '',
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: '',
        tools: [
          {
            description: 'Score inbound leads',
            enabled: true,
            inputSchema: {
              properties: { lead_score: { type: 'number' } },
              type: 'object'
            },
            name: 'lead_score',
            requiresApproval: true,
            riskLevel: 'dangerous',
            runtime: 'python',
            sourceCode: 'def main(args):\n    return {"score": args["lead_score"] * 2}',
            timeoutSeconds: 3,
            type: 'custom'
          }
        ]
      });
    });
    expect(await screen.findByText('Custom Python tool saved.')).toBeInTheDocument();
    expect(screen.getByText('custom / python')).toBeInTheDocument();
  });

  it('keeps rendered agents visible when refresh fails', async () => {
    listAgents
      .mockResolvedValueOnce([
        {
          config: { approvalMode: 'none' },
          id: 'agent_1',
          isPublic: false,
          model: 'gpt-4o-mini',
          name: 'Support Agent',
          tools: []
        }
      ])
      .mockRejectedValueOnce(new Error('agent API unavailable'));

    renderAgentsPage();

    expect(await screen.findByRole('button', { name: 'Support Agent' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Refresh agents' }));

    expect(await screen.findByText('agent API unavailable')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Support Agent' })).toBeInTheDocument();
  });

  it('loads available tool definitions for the selected agent and preserves them when refresh fails', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: { approvalMode: 'tiered' },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      }
    ]);
    getAgentTools
      .mockResolvedValueOnce([
        {
          capabilityId: 'mcp.network_execution',
          description: 'Search workspace and web sources',
          inputSchema: { properties: { query: { type: 'string' } }, type: 'object' },
          name: 'web_search'
        }
      ])
      .mockRejectedValueOnce(new Error('tool catalog unavailable'));

    renderAgentsPage();

    expect(await screen.findByRole('button', { name: 'Research Agent' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Load tool catalog' }));

    await waitFor(() => expect(getAgentTools).toHaveBeenCalledWith('agent_1'));
    expect(await screen.findByText('Search workspace and web sources')).toBeInTheDocument();
    expect(screen.getByText('Search workspace and web sources').closest('section')).toHaveTextContent('"query"');

    fireEvent.click(screen.getByRole('button', { name: 'Load tool catalog' }));

    expect(await screen.findByText('tool catalog unavailable')).toBeInTheDocument();
    expect(screen.getByText('Search workspace and web sources')).toBeInTheDocument();
  });

  it('enables a catalog tool for the selected agent', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
        description: 'Research and summarize workspace materials.',
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: 'Prefer cited answers.',
        tools: [{ enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' }]
      }
    ]);
    getAgentTools.mockResolvedValueOnce([
      {
        capabilityId: 'mcp.network_execution',
        description: 'Lookup customer records',
        inputSchema: { properties: { customer_id: { type: 'string' } }, type: 'object' },
        name: 'crm_lookup',
        requiresApproval: true,
        riskLevel: 'medium',
        toolType: 'mcp'
      }
    ]);
    updateAgent.mockResolvedValueOnce({
      config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
      description: 'Research and summarize workspace materials.',
      id: 'agent_1',
      isPublic: false,
      model: 'gpt-4o-mini',
      name: 'Research Agent',
      systemPrompt: 'Prefer cited answers.',
      tools: [
        { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
        {
          description: 'Lookup customer records',
          enabled: true,
          inputSchema: { properties: { customer_id: { type: 'string' } }, type: 'object' },
          name: 'crm_lookup',
          requiresApproval: true,
          riskLevel: 'medium',
          type: 'mcp'
        }
      ]
    });

    renderAgentsPage();

    expect(await screen.findByRole('button', { name: 'Research Agent' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Load tool catalog' }));
    expect(await screen.findByRole('button', { name: 'Enable tool crm_lookup' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Enable tool crm_lookup' }));

    await waitFor(() => {
      expect(updateAgent).toHaveBeenCalledWith('agent_1', {
        config: { approvalMode: 'tiered', defaultExecutionMode: 'react' },
        description: 'Research and summarize workspace materials.',
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        systemPrompt: 'Prefer cited answers.',
        tools: [
          { enabled: true, name: 'web_search', riskLevel: 'medium', type: 'builtin' },
          {
            description: 'Lookup customer records',
            enabled: true,
            inputSchema: { properties: { customer_id: { type: 'string' } }, type: 'object' },
            name: 'crm_lookup',
            requiresApproval: true,
            riskLevel: 'medium',
            type: 'mcp'
          }
        ]
      });
    });

    expect(await screen.findByText('Tool enabled for agent.')).toBeInTheDocument();
    expect(screen.getByLabelText('Tool policy crm_lookup')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Tool crm_lookup enabled' })).toBeDisabled();
  });

  it('starts a planning run for the selected agent and links to plan steps', async () => {
    listAgents.mockResolvedValueOnce([
      {
        config: {
          approvalMode: 'tiered',
          defaultExecutionMode: 'planning',
          maxIterations: 15,
          tokenBudget: 30000
        },
        id: 'agent_1',
        isPublic: false,
        model: 'gpt-4o-mini',
        name: 'Research Agent',
        tools: []
      }
    ]);
    createRun.mockResolvedValueOnce({
      id: 'run_1',
      planSteps: [{ id: 'step_1', status: 'pending', title: 'Inspect workspace' }],
      status: 'running',
      toolRuns: []
    });

    renderAgentsPage();

    expect(await screen.findByRole('button', { name: 'Research Agent' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Run conversation ID'), { target: { value: 'conversation_1' } });
    fireEvent.change(screen.getByLabelText('Run goal'), { target: { value: 'Audit the workspace release checklist' } });
    fireEvent.click(screen.getByRole('button', { name: 'Start run' }));

    await waitFor(() => {
      expect(createRun).toHaveBeenCalledWith({
        agentId: 'agent_1',
        conversationId: 'conversation_1',
        input: 'Audit the workspace release checklist',
        maxIterations: 15,
        mode: 'planning',
        tokenBudget: 30000
      });
    });
    expect(await screen.findByRole('link', { name: 'Open run plan steps' })).toHaveAttribute(
      'href',
      '/agent-runs/run_1/plan-steps'
    );
  });
});
