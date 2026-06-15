import { useEffect, useMemo, useState } from 'react';
import { RiAddLine, RiPlayCircleLine, RiRefreshLine, RiSave3Line, RiTerminalBoxLine } from '@remixicon/react';
import { Link } from 'react-router-dom';

import { createAgentsApi, type AgentToolDefinition } from '../../features/agents/agentsApi';
import { createHttpClient } from '../../services/http/client';
import type { AgentConfig, AgentModelRoutingRule, AgentSkill, AgentSummary, AgentTool, ToolApprovalOverride } from '../../types/api';

type ApprovalMode = 'tiered' | 'all' | 'none' | 'custom';
type AgentExecutionMode = 'react' | 'planning';
type CustomToolRuntime = 'api' | 'python';
type LongTermMemoryExtractionPolicy = 'deterministic' | 'llm_assisted';
type LongTermMemoryUpdatePolicy = 'exact_refresh' | 'memory_key_consolidate';
type LongTermMemoryWritePolicy = 'interaction_and_explicit' | 'explicit_only' | 'interaction_only' | 'manual_only';
type RiskLevel = 'safe' | 'medium' | 'dangerous';

const approvalModes: ApprovalMode[] = ['tiered', 'all', 'none', 'custom'];
const executionModes: AgentExecutionMode[] = ['react', 'planning'];
const customToolRuntimes: CustomToolRuntime[] = ['api', 'python'];
const longTermMemoryExtractionPolicies: LongTermMemoryExtractionPolicy[] = ['deterministic', 'llm_assisted'];
const longTermMemoryUpdatePolicies: LongTermMemoryUpdatePolicy[] = ['exact_refresh', 'memory_key_consolidate'];
const longTermMemoryWritePolicies: LongTermMemoryWritePolicy[] = ['interaction_and_explicit', 'explicit_only', 'interaction_only', 'manual_only'];
const riskLevels: RiskLevel[] = ['safe', 'medium', 'dangerous'];

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }
  return fallback;
}

function normalizeApprovalMode(value: unknown): ApprovalMode {
  return approvalModes.includes(value as ApprovalMode) ? value as ApprovalMode : 'tiered';
}

function normalizeExecutionMode(value: unknown): AgentExecutionMode {
  return executionModes.includes(value as AgentExecutionMode) ? value as AgentExecutionMode : 'react';
}

function normalizeLongTermMemoryWritePolicy(value: unknown): LongTermMemoryWritePolicy {
  return longTermMemoryWritePolicies.includes(value as LongTermMemoryWritePolicy)
    ? value as LongTermMemoryWritePolicy
    : 'interaction_and_explicit';
}

function normalizeLongTermMemoryExtractionPolicy(value: unknown): LongTermMemoryExtractionPolicy {
  return longTermMemoryExtractionPolicies.includes(value as LongTermMemoryExtractionPolicy)
    ? value as LongTermMemoryExtractionPolicy
    : 'deterministic';
}

function normalizeLongTermMemoryUpdatePolicy(value: unknown): LongTermMemoryUpdatePolicy {
  return longTermMemoryUpdatePolicies.includes(value as LongTermMemoryUpdatePolicy)
    ? value as LongTermMemoryUpdatePolicy
    : 'exact_refresh';
}

function normalizeCustomToolRuntime(value: unknown): CustomToolRuntime {
  return customToolRuntimes.includes(value as CustomToolRuntime) ? value as CustomToolRuntime : 'api';
}

function normalizeRiskLevel(value: unknown): RiskLevel {
  return riskLevels.includes(value as RiskLevel) ? value as RiskLevel : 'safe';
}

function numberFieldValue(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? String(value) : '';
}

function arrayFieldValue(value: unknown) {
  return Array.isArray(value) && value.length > 0 ? readableJSON(value) : '';
}

function optionalPositiveInteger(value: string) {
  if (value.trim() === '') {
    return undefined;
  }

  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return undefined;
  }

  return Math.trunc(parsed);
}

function toolKey(tool: AgentTool) {
  return tool.name;
}

function toolDisplayType(tool: AgentTool) {
  const runtime = tool.runtime === 'python' ? 'python' : tool.serverId;
  return [tool.type, runtime].filter(Boolean).join(' / ') || 'builtin';
}

function readableJSON(value: unknown) {
  if (value === undefined || value === null) {
    return '';
  }

  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function parseConfigArray(value: string, label: string): unknown[] | undefined {
  const trimmed = value.trim();
  if (trimmed === '') {
    return undefined;
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    throw new Error(`${label} must be valid JSON.`);
  }

  if (!Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON array.`);
  }
  return parsed;
}

function parseModelRoutingRules(value: string): AgentModelRoutingRule[] | undefined {
  const parsed = parseConfigArray(value, 'Model routing rules');
  if (parsed === undefined) {
    return undefined;
  }
  if (
    parsed.some((rule) => (
      typeof rule !== 'object' ||
      rule === null ||
      typeof (rule as Partial<AgentModelRoutingRule>).targetModel !== 'string' ||
      (rule as Partial<AgentModelRoutingRule>).targetModel?.trim() === ''
    ))
  ) {
    throw new Error('Model routing rules must include targetModel.');
  }
  return parsed as AgentModelRoutingRule[];
}

function parseAgentSkills(value: string): AgentSkill[] | undefined {
  const parsed = parseConfigArray(value, 'Skills');
  if (parsed === undefined) {
    return undefined;
  }
  if (
    parsed.some((skill) => (
      typeof skill !== 'object' ||
      skill === null ||
      typeof (skill as Partial<AgentSkill>).name !== 'string' ||
      (skill as Partial<AgentSkill>).name?.trim() === ''
    ))
  ) {
    throw new Error('Skills must include name.');
  }
  return parsed as AgentSkill[];
}

function toolFromCatalogDefinition(tool: AgentToolDefinition): AgentTool {
  return {
    enabled: true,
    ...(tool.description ? { description: tool.description } : {}),
    ...(tool.inputSchema !== undefined ? { inputSchema: tool.inputSchema } : {}),
    name: tool.name,
    requiresApproval: tool.requiresApproval ?? false,
    riskLevel: normalizeRiskLevel(tool.riskLevel),
    type: tool.toolType ?? 'builtin'
  };
}

function initialOverrides(agent: AgentSummary | null) {
  const overrides: Record<string, ToolApprovalOverride> = {
    ...(agent?.config?.toolApprovalOverrides ?? {})
  };

  agent?.tools?.forEach((tool) => {
    const configured = agent.config?.toolApprovalOverrides?.[toolKey(tool)];
    overrides[toolKey(tool)] = {
      requiresApproval: configured?.requiresApproval ?? tool.requiresApproval ?? false,
      riskLevel: normalizeRiskLevel(configured?.riskLevel ?? tool.riskLevel)
    };
  });

  return overrides;
}

export function AgentsPage() {
  const api = useMemo(() => createAgentsApi(createHttpClient()), []);
  const [agents, setAgents] = useState<AgentSummary[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDescription, setCreateDescription] = useState('');
  const [createModel, setCreateModel] = useState('gpt-4o-mini');
  const [createSystemPrompt, setCreateSystemPrompt] = useState('');
  const [createApprovalMode, setCreateApprovalMode] = useState<ApprovalMode>('tiered');
  const [createDefaultExecutionMode, setCreateDefaultExecutionMode] = useState<AgentExecutionMode>('react');
  const [createLongTermMemoryExtractionPolicy, setCreateLongTermMemoryExtractionPolicy] = useState<LongTermMemoryExtractionPolicy>('deterministic');
  const [createLongTermMemoryUpdatePolicy, setCreateLongTermMemoryUpdatePolicy] = useState<LongTermMemoryUpdatePolicy>('exact_refresh');
  const [createLongTermMemoryWritePolicy, setCreateLongTermMemoryWritePolicy] = useState<LongTermMemoryWritePolicy>('interaction_and_explicit');
  const [createMaxIterations, setCreateMaxIterations] = useState('');
  const [createTokenBudget, setCreateTokenBudget] = useState('');
  const [createMaxSkills, setCreateMaxSkills] = useState('');
  const [createModelRoutingRules, setCreateModelRoutingRules] = useState('');
  const [createSkills, setCreateSkills] = useState('');
  const [approvalMode, setApprovalMode] = useState<ApprovalMode>('tiered');
  const [defaultExecutionMode, setDefaultExecutionMode] = useState<AgentExecutionMode>('react');
  const [longTermMemoryExtractionPolicy, setLongTermMemoryExtractionPolicy] = useState<LongTermMemoryExtractionPolicy>('deterministic');
  const [longTermMemoryUpdatePolicy, setLongTermMemoryUpdatePolicy] = useState<LongTermMemoryUpdatePolicy>('exact_refresh');
  const [longTermMemoryWritePolicy, setLongTermMemoryWritePolicy] = useState<LongTermMemoryWritePolicy>('interaction_and_explicit');
  const [maxIterations, setMaxIterations] = useState('');
  const [tokenBudget, setTokenBudget] = useState('');
  const [maxSkills, setMaxSkills] = useState('');
  const [modelRoutingRules, setModelRoutingRules] = useState('');
  const [skills, setSkills] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoadingTools, setIsLoadingTools] = useState(false);
  const [isStartingRun, setIsStartingRun] = useState(false);
  const [overrides, setOverrides] = useState<Record<string, ToolApprovalOverride>>({});
  const [runConversationId, setRunConversationId] = useState('');
  const [runGoal, setRunGoal] = useState('');
  const [runLink, setRunLink] = useState('');
  const [runMaxIterations, setRunMaxIterations] = useState('');
  const [runMode, setRunMode] = useState<AgentExecutionMode>('react');
  const [runTokenBudget, setRunTokenBudget] = useState('');
  const [savedMessage, setSavedMessage] = useState('');
  const [toolCatalog, setToolCatalog] = useState<AgentToolDefinition[]>([]);
  const [enablingCatalogToolName, setEnablingCatalogToolName] = useState('');
  const [isCustomToolOpen, setIsCustomToolOpen] = useState(false);
  const [isSavingCustomTool, setIsSavingCustomTool] = useState(false);
  const [customToolName, setCustomToolName] = useState('');
  const [customToolRuntime, setCustomToolRuntime] = useState<CustomToolRuntime>('api');
  const [customToolEndpoint, setCustomToolEndpoint] = useState('');
  const [customToolDescription, setCustomToolDescription] = useState('');
  const [customToolRiskLevel, setCustomToolRiskLevel] = useState<RiskLevel>('safe');
  const [customToolRequiresApproval, setCustomToolRequiresApproval] = useState(false);
  const [customToolInputSchema, setCustomToolInputSchema] = useState('');
  const [customToolSourceCode, setCustomToolSourceCode] = useState('');
  const [customToolTimeoutSeconds, setCustomToolTimeoutSeconds] = useState('');

  const selectedAgent = agents.find((agent) => agent.id === selectedAgentId) ?? agents[0] ?? null;

  const applySelectedAgent = (agent: AgentSummary | null) => {
    setSelectedAgentId(agent?.id ?? '');
    setApprovalMode(normalizeApprovalMode(agent?.config?.approvalMode));
    setDefaultExecutionMode(normalizeExecutionMode(agent?.config?.defaultExecutionMode));
    setLongTermMemoryExtractionPolicy(normalizeLongTermMemoryExtractionPolicy(agent?.config?.longTermMemoryExtractionPolicy));
    setLongTermMemoryUpdatePolicy(normalizeLongTermMemoryUpdatePolicy(agent?.config?.longTermMemoryUpdatePolicy));
    setLongTermMemoryWritePolicy(normalizeLongTermMemoryWritePolicy(agent?.config?.longTermMemoryWritePolicy));
    setMaxIterations(numberFieldValue(agent?.config?.maxIterations));
    setTokenBudget(numberFieldValue(agent?.config?.tokenBudget));
    setMaxSkills(numberFieldValue(agent?.config?.maxSkills));
    setModelRoutingRules(arrayFieldValue(agent?.config?.modelRoutingRules));
    setSkills(arrayFieldValue(agent?.config?.skills));
    setOverrides(initialOverrides(agent));
    setRunLink('');
    setRunMaxIterations(numberFieldValue(agent?.config?.maxIterations));
    setRunMode(normalizeExecutionMode(agent?.config?.defaultExecutionMode));
    setRunTokenBudget(numberFieldValue(agent?.config?.tokenBudget));
    setSavedMessage('');
    setToolCatalog([]);
    setEnablingCatalogToolName('');
    setIsCustomToolOpen(false);
    setCustomToolName('');
    setCustomToolRuntime('api');
    setCustomToolEndpoint('');
    setCustomToolDescription('');
    setCustomToolRiskLevel('safe');
    setCustomToolRequiresApproval(false);
    setCustomToolInputSchema('');
    setCustomToolSourceCode('');
    setCustomToolTimeoutSeconds('');
  };

  const loadAgents = async () => {
    setIsLoading(true);
    setError(null);

    try {
      const loadedAgents = await api.listAgents();
      setAgents(loadedAgents);
      const nextSelected = loadedAgents.find((agent) => agent.id === selectedAgentId) ?? loadedAgents[0] ?? null;
      applySelectedAgent(nextSelected);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load agents.'));
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadAgents();
    // The selected agent is only reconciled when a user refreshes explicitly.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api]);

  const selectAgent = (agent: AgentSummary) => {
    applySelectedAgent(agent);
  };

  const resetCreateForm = () => {
    setCreateName('');
    setCreateDescription('');
    setCreateModel('gpt-4o-mini');
    setCreateSystemPrompt('');
    setCreateApprovalMode('tiered');
    setCreateDefaultExecutionMode('react');
    setCreateLongTermMemoryExtractionPolicy('deterministic');
    setCreateLongTermMemoryUpdatePolicy('exact_refresh');
    setCreateLongTermMemoryWritePolicy('interaction_and_explicit');
    setCreateMaxIterations('');
    setCreateTokenBudget('');
    setCreateMaxSkills('');
    setCreateModelRoutingRules('');
    setCreateSkills('');
  };

  const resetCustomToolForm = () => {
    setCustomToolName('');
    setCustomToolRuntime('api');
    setCustomToolEndpoint('');
    setCustomToolDescription('');
    setCustomToolRiskLevel('safe');
    setCustomToolRequiresApproval(false);
    setCustomToolInputSchema('');
    setCustomToolSourceCode('');
    setCustomToolTimeoutSeconds('');
  };

  const createAgentFromForm = async () => {
    const name = createName.trim();
    const model = createModel.trim() || 'gpt-4o-mini';
    const description = createDescription.trim();
    const systemPrompt = createSystemPrompt.trim();
    const parsedMaxIterations = optionalPositiveInteger(createMaxIterations);
    const parsedTokenBudget = optionalPositiveInteger(createTokenBudget);
    const parsedMaxSkills = optionalPositiveInteger(createMaxSkills);

    if (!name) {
      setError('Agent name is required.');
      return;
    }

    setIsCreating(true);
    setError(null);
    setSavedMessage('');

    try {
      const config: AgentConfig = {
        approvalMode: createApprovalMode,
        defaultExecutionMode: createDefaultExecutionMode,
        longTermMemoryExtractionPolicy: createLongTermMemoryExtractionPolicy,
        longTermMemoryUpdatePolicy: createLongTermMemoryUpdatePolicy,
        longTermMemoryWritePolicy: createLongTermMemoryWritePolicy
      };
      if (parsedMaxIterations !== undefined) {
        config.maxIterations = parsedMaxIterations;
      }
      if (parsedTokenBudget !== undefined) {
        config.tokenBudget = parsedTokenBudget;
      }
      if (parsedMaxSkills !== undefined) {
        config.maxSkills = parsedMaxSkills;
      }
      const parsedModelRoutingRules = parseModelRoutingRules(createModelRoutingRules);
      if (parsedModelRoutingRules !== undefined) {
        config.modelRoutingRules = parsedModelRoutingRules;
      }
      const parsedSkills = parseAgentSkills(createSkills);
      if (parsedSkills !== undefined) {
        config.skills = parsedSkills;
      }

      const created = await api.createAgent({
        config,
        description,
        isPublic: false,
        model,
        name,
        systemPrompt,
        tools: []
      });
      setAgents((current) => [created, ...current.filter((agent) => agent.id !== created.id)]);
      applySelectedAgent(created);
      resetCreateForm();
      setIsCreateOpen(false);
      setSavedMessage('Agent created.');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to create agent.'));
    } finally {
      setIsCreating(false);
    }
  };

  const setToolApprovalRequired = (tool: AgentTool, requiresApproval: boolean) => {
    const key = toolKey(tool);
    setOverrides((current) => ({
      ...current,
      [key]: {
        requiresApproval,
        riskLevel: normalizeRiskLevel(current[key]?.riskLevel ?? tool.riskLevel)
      }
    }));
    setSavedMessage('');
  };

  const setToolRiskLevel = (tool: AgentTool, riskLevel: RiskLevel) => {
    const key = toolKey(tool);
    setOverrides((current) => ({
      ...current,
      [key]: {
        requiresApproval: current[key]?.requiresApproval ?? tool.requiresApproval ?? false,
        riskLevel
      }
    }));
    setSavedMessage('');
  };

  const saveAgentPolicy = async () => {
    if (!selectedAgent) {
      return;
    }

    setIsSaving(true);
    setError(null);
    setSavedMessage('');

    try {
      const nextConfig: AgentConfig = {
        ...(selectedAgent.config ?? {}),
        approvalMode,
        longTermMemoryExtractionPolicy,
        longTermMemoryUpdatePolicy,
        longTermMemoryWritePolicy,
        toolApprovalOverrides: overrides
      };
      const initialExecutionMode = normalizeExecutionMode(selectedAgent.config?.defaultExecutionMode);
      const parsedMaxIterations = optionalPositiveInteger(maxIterations);
      const parsedTokenBudget = optionalPositiveInteger(tokenBudget);
      const parsedMaxSkills = optionalPositiveInteger(maxSkills);
      const parsedModelRoutingRules = parseModelRoutingRules(modelRoutingRules);
      const parsedSkills = parseAgentSkills(skills);

      if (selectedAgent.config?.defaultExecutionMode !== undefined || defaultExecutionMode !== initialExecutionMode) {
        nextConfig.defaultExecutionMode = defaultExecutionMode;
      } else {
        delete nextConfig.defaultExecutionMode;
      }

      if (parsedMaxIterations === undefined) {
        delete nextConfig.maxIterations;
      } else {
        nextConfig.maxIterations = parsedMaxIterations;
      }

      if (parsedTokenBudget === undefined) {
        delete nextConfig.tokenBudget;
      } else {
        nextConfig.tokenBudget = parsedTokenBudget;
      }

      if (parsedMaxSkills === undefined) {
        delete nextConfig.maxSkills;
      } else {
        nextConfig.maxSkills = parsedMaxSkills;
      }

      if (parsedModelRoutingRules === undefined) {
        delete nextConfig.modelRoutingRules;
      } else {
        nextConfig.modelRoutingRules = parsedModelRoutingRules;
      }

      if (parsedSkills === undefined) {
        delete nextConfig.skills;
      } else {
        nextConfig.skills = parsedSkills;
      }

      const updated = await api.updateAgent(selectedAgent.id, {
        config: nextConfig,
        description: selectedAgent.description ?? '',
        model: selectedAgent.model,
        name: selectedAgent.name,
        systemPrompt: selectedAgent.systemPrompt ?? '',
        tools: selectedAgent.tools ?? []
      });
      setAgents((current) => current.map((agent) => (agent.id === updated.id ? updated : agent)));
      applySelectedAgent(updated);
      setSavedMessage('Agent policy saved.');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save agent policy.'));
    } finally {
      setIsSaving(false);
    }
  };

  const loadToolCatalog = async () => {
    if (!selectedAgent) {
      return;
    }

    setIsLoadingTools(true);
    setError(null);

    try {
      setToolCatalog(await api.getAgentTools(selectedAgent.id));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to load tool catalog.'));
    } finally {
      setIsLoadingTools(false);
    }
  };

  const enableCatalogTool = async (tool: AgentToolDefinition) => {
    if (!selectedAgent) {
      return;
    }

    if ((selectedAgent.tools ?? []).some((agentTool) => agentTool.name === tool.name)) {
      setError('Tool already enabled for this agent.');
      return;
    }

    const nextTool = toolFromCatalogDefinition(tool);
    const currentCatalog = toolCatalog;
    setEnablingCatalogToolName(tool.name);
    setError(null);
    setSavedMessage('');

    try {
      const updated = await api.updateAgent(selectedAgent.id, {
        config: selectedAgent.config ?? {},
        description: selectedAgent.description ?? '',
        model: selectedAgent.model,
        name: selectedAgent.name,
        systemPrompt: selectedAgent.systemPrompt ?? '',
        tools: [...(selectedAgent.tools ?? []), nextTool]
      });
      setAgents((current) => current.map((agent) => (agent.id === updated.id ? updated : agent)));
      applySelectedAgent(updated);
      setToolCatalog(currentCatalog);
      setSavedMessage('Tool enabled for agent.');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to enable tool.'));
    } finally {
      setEnablingCatalogToolName('');
    }
  };

  const saveCustomAPITool = async () => {
    if (!selectedAgent) {
      return;
    }

    const name = customToolName.trim();
    const endpoint = customToolEndpoint.trim();
    const description = customToolDescription.trim();
    const sourceCode = customToolSourceCode.trim();

    if (!name || (customToolRuntime === 'api' && !endpoint)) {
      setError('Tool name and endpoint URL are required.');
      return;
    }

    if (customToolRuntime === 'python' && !sourceCode) {
      setError('Python source code is required.');
      return;
    }

    if ((selectedAgent.tools ?? []).some((tool) => tool.name === name)) {
      setError('Tool name already exists for this agent.');
      return;
    }

    let inputSchema: unknown;
    if (customToolInputSchema.trim()) {
      try {
        inputSchema = JSON.parse(customToolInputSchema);
      } catch {
        setError('Input schema JSON is invalid.');
        return;
      }
    }

    const nextTool: AgentTool = {
      description,
      enabled: true,
      ...(inputSchema !== undefined ? { inputSchema } : {}),
      name,
      requiresApproval: customToolRequiresApproval,
      riskLevel: customToolRiskLevel,
      ...(customToolRuntime === 'api' ? { serverId: endpoint } : {}),
      ...(customToolRuntime === 'python' ? { runtime: 'python', sourceCode } : {}),
      ...(customToolRuntime === 'python' && optionalPositiveInteger(customToolTimeoutSeconds) !== undefined
        ? { timeoutSeconds: optionalPositiveInteger(customToolTimeoutSeconds) }
        : {}),
      type: 'custom'
    };

    setIsSavingCustomTool(true);
    setError(null);
    setSavedMessage('');

    try {
      const updated = await api.updateAgent(selectedAgent.id, {
        config: selectedAgent.config ?? {},
        description: selectedAgent.description ?? '',
        model: selectedAgent.model,
        name: selectedAgent.name,
        systemPrompt: selectedAgent.systemPrompt ?? '',
        tools: [...(selectedAgent.tools ?? []), nextTool]
      });
      setAgents((current) => current.map((agent) => (agent.id === updated.id ? updated : agent)));
      applySelectedAgent(updated);
      resetCustomToolForm();
      setIsCustomToolOpen(false);
      setSavedMessage(customToolRuntime === 'python' ? 'Custom Python tool saved.' : 'Custom API tool saved.');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to save custom API tool.'));
    } finally {
      setIsSavingCustomTool(false);
    }
  };

  const startRun = async () => {
    if (!selectedAgent) {
      return;
    }

    const conversationId = runConversationId.trim();
    const input = runGoal.trim();
    if (!conversationId || !input) {
      setError('Run conversation ID and goal are required.');
      return;
    }

    setIsStartingRun(true);
    setError(null);
    setRunLink('');

    try {
      const createdRun = await api.createRun({
        agentId: selectedAgent.id,
        conversationId,
        input,
        maxIterations: optionalPositiveInteger(runMaxIterations),
        mode: runMode,
        tokenBudget: optionalPositiveInteger(runTokenBudget)
      });
      setRunLink(`/agent-runs/${encodeURIComponent(createdRun.id)}/plan-steps`);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to start agent run.'));
    } finally {
      setIsStartingRun(false);
    }
  };

  return (
    <section className="mx-auto max-w-6xl space-y-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Workspace runtime</p>
          <h1 className="font-heading text-3xl font-semibold text-[#181611]">Agents</h1>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            className="inline-flex min-h-10 items-center gap-2 rounded-lg bg-[#181611] px-3 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
            onClick={() => {
              setIsCreateOpen(true);
              setError(null);
              setSavedMessage('');
            }}
            type="button"
          >
            <RiAddLine className="size-4" aria-hidden="true" />
            Create agent
          </button>
          <button
            className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
            disabled={isLoading}
            onClick={() => void loadAgents()}
            type="button"
          >
            <RiRefreshLine className="size-4" aria-hidden="true" />
            Refresh agents
          </button>
        </div>
      </header>

      {error ? (
        <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert">
          {error}
        </p>
      ) : null}

      {isCreateOpen ? (
        <section aria-label="Create agent form" className="rounded-lg border border-[#d7d2c4] bg-white">
          <div className="border-b border-[#d7d2c4] px-5 py-4">
            <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Agent setup</p>
            <h2 className="text-base font-semibold text-[#181611]">Create agent</h2>
          </div>
          <div className="grid gap-4 px-5 py-4 md:grid-cols-2">
            <label className="text-sm font-medium text-[#181611]">
              Agent name
              <input
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateName(event.target.value)}
                type="text"
                value={createName}
              />
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Model
              <input
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateModel(event.target.value)}
                type="text"
                value={createModel}
              />
            </label>
            <label className="text-sm font-medium text-[#181611] md:col-span-2">
              Description
              <input
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateDescription(event.target.value)}
                type="text"
                value={createDescription}
              />
            </label>
            <label className="text-sm font-medium text-[#181611] md:col-span-2">
              System prompt
              <textarea
                className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 text-sm leading-6"
                onChange={(event) => setCreateSystemPrompt(event.target.value)}
                value={createSystemPrompt}
              />
            </label>
          </div>
          <div className="grid gap-4 border-t border-[#d7d2c4] px-5 py-4 md:grid-cols-3">
            <label className="text-sm font-medium text-[#181611]">
              Approval mode
              <select
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateApprovalMode(normalizeApprovalMode(event.target.value))}
                value={createApprovalMode}
              >
                {approvalModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {mode}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Default execution mode
              <select
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateDefaultExecutionMode(normalizeExecutionMode(event.target.value))}
                value={createDefaultExecutionMode}
              >
                {executionModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {mode}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Max iterations
              <input
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                min={1}
                max={100}
                onChange={(event) => setCreateMaxIterations(event.target.value)}
                step={1}
                type="number"
                value={createMaxIterations}
              />
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Token budget
              <input
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                min={1000}
                max={1000000}
                onChange={(event) => setCreateTokenBudget(event.target.value)}
                step={1000}
                type="number"
                value={createTokenBudget}
              />
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Max skills
              <input
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                min={1}
                max={20}
                onChange={(event) => setCreateMaxSkills(event.target.value)}
                step={1}
                type="number"
                value={createMaxSkills}
              />
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Long-term memory writes
              <select
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateLongTermMemoryWritePolicy(normalizeLongTermMemoryWritePolicy(event.target.value))}
                value={createLongTermMemoryWritePolicy}
              >
                {longTermMemoryWritePolicies.map((policy) => (
                  <option key={policy} value={policy}>
                    {policy}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Long-term memory extraction
              <select
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateLongTermMemoryExtractionPolicy(normalizeLongTermMemoryExtractionPolicy(event.target.value))}
                value={createLongTermMemoryExtractionPolicy}
              >
                {longTermMemoryExtractionPolicies.map((policy) => (
                  <option key={policy} value={policy}>
                    {policy}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm font-medium text-[#181611]">
              Long-term memory update
              <select
                className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                onChange={(event) => setCreateLongTermMemoryUpdatePolicy(normalizeLongTermMemoryUpdatePolicy(event.target.value))}
                value={createLongTermMemoryUpdatePolicy}
              >
                {longTermMemoryUpdatePolicies.map((policy) => (
                  <option key={policy} value={policy}>
                    {policy}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm font-medium text-[#181611] md:col-span-3">
              Model routing rules JSON
              <textarea
                className="mt-2 min-h-28 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-xs leading-5 text-[#181611]"
                onChange={(event) => setCreateModelRoutingRules(event.target.value)}
                value={createModelRoutingRules}
              />
            </label>
            <label className="text-sm font-medium text-[#181611] md:col-span-3">
              Skills JSON
              <textarea
                className="mt-2 min-h-28 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-xs leading-5 text-[#181611]"
                onChange={(event) => setCreateSkills(event.target.value)}
                value={createSkills}
              />
            </label>
          </div>
          <div className="flex flex-wrap items-center gap-3 border-t border-[#d7d2c4] px-5 py-4">
            <button
              className="inline-flex min-h-10 items-center gap-2 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isCreating || !createName.trim()}
              onClick={() => void createAgentFromForm()}
              type="button"
            >
              <RiSave3Line className="size-4" aria-hidden="true" />
              Save agent
            </button>
            <button
              className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-[#d7d2c4] px-4 text-sm font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isCreating}
              onClick={() => {
                resetCreateForm();
                setIsCreateOpen(false);
              }}
              type="button"
            >
              Cancel
            </button>
          </div>
        </section>
      ) : null}

      <div className="grid gap-5 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="space-y-3">
          {isLoading && agents.length === 0 ? <p className="text-sm text-[#625b4f]">Loading agents...</p> : null}
          {!isLoading && agents.length === 0 ? <p className="text-sm text-[#625b4f]">No agents configured.</p> : null}
          {agents.map((agent) => (
            <button
              aria-label={agent.name}
              aria-pressed={selectedAgent?.id === agent.id}
              className="block min-h-[52px] w-full rounded-lg border border-[#d7d2c4] bg-white px-4 py-3 text-left text-sm transition hover:border-[#181611] aria-pressed:border-[#181611] aria-pressed:bg-[#f6f1e6]"
              key={agent.id}
              onClick={() => selectAgent(agent)}
              type="button"
            >
              <span className="block font-semibold text-[#181611]">{agent.name}</span>
              <span className="mt-1 block text-xs text-[#625b4f]">{agent.model}</span>
            </button>
          ))}
        </aside>

        {selectedAgent ? (
          <section aria-label={`Agent policy ${selectedAgent.name}`} className="space-y-5">
            <div className="rounded-lg border border-[#d7d2c4] bg-white p-5">
              <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">{selectedAgent.model}</p>
                  <h2 className="mt-1 text-xl font-semibold text-[#181611]">{selectedAgent.name}</h2>
                  {selectedAgent.description ? <p className="mt-2 text-sm leading-6 text-[#625b4f]">{selectedAgent.description}</p> : null}
                </div>
                <label className="text-sm font-medium text-[#181611]">
                  Approval mode
                  <select
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setApprovalMode(normalizeApprovalMode(event.target.value));
                      setSavedMessage('');
                    }}
                    value={approvalMode}
                  >
                    {approvalModes.map((mode) => (
                      <option key={mode} value={mode}>
                        {mode}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            </div>

            <section className="rounded-lg border border-[#d7d2c4] bg-white">
              <div className="border-b border-[#d7d2c4] px-5 py-4">
                <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Run behavior</p>
                <h2 className="text-base font-semibold text-[#181611]">Execution limits</h2>
              </div>
              <div className="grid gap-4 px-5 py-4 md:grid-cols-3">
                <label className="text-sm font-medium text-[#181611]">
                  Default execution mode
                  <select
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setDefaultExecutionMode(normalizeExecutionMode(event.target.value));
                      setSavedMessage('');
                    }}
                    value={defaultExecutionMode}
                  >
                    {executionModes.map((mode) => (
                      <option key={mode} value={mode}>
                        {mode}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Max iterations
                  <input
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    min={1}
                    max={100}
                    onChange={(event) => {
                      setMaxIterations(event.target.value);
                      setSavedMessage('');
                    }}
                    step={1}
                    type="number"
                    value={maxIterations}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Token budget
                  <input
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    min={1000}
                    max={1000000}
                    onChange={(event) => {
                      setTokenBudget(event.target.value);
                      setSavedMessage('');
                    }}
                    step={1000}
                    type="number"
                    value={tokenBudget}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Max skills
                  <input
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    min={1}
                    max={20}
                    onChange={(event) => {
                      setMaxSkills(event.target.value);
                      setSavedMessage('');
                    }}
                    step={1}
                    type="number"
                    value={maxSkills}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Long-term memory writes
                  <select
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setLongTermMemoryWritePolicy(normalizeLongTermMemoryWritePolicy(event.target.value));
                      setSavedMessage('');
                    }}
                    value={longTermMemoryWritePolicy}
                  >
                    {longTermMemoryWritePolicies.map((policy) => (
                      <option key={policy} value={policy}>
                        {policy}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Long-term memory extraction
                  <select
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setLongTermMemoryExtractionPolicy(normalizeLongTermMemoryExtractionPolicy(event.target.value));
                      setSavedMessage('');
                    }}
                    value={longTermMemoryExtractionPolicy}
                  >
                    {longTermMemoryExtractionPolicies.map((policy) => (
                      <option key={policy} value={policy}>
                        {policy}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Long-term memory update
                  <select
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setLongTermMemoryUpdatePolicy(normalizeLongTermMemoryUpdatePolicy(event.target.value));
                      setSavedMessage('');
                    }}
                    value={longTermMemoryUpdatePolicy}
                  >
                    {longTermMemoryUpdatePolicies.map((policy) => (
                      <option key={policy} value={policy}>
                        {policy}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-sm font-medium text-[#181611] md:col-span-3">
                  Model routing rules JSON
                  <textarea
                    className="mt-2 min-h-28 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-xs leading-5 text-[#181611]"
                    onChange={(event) => {
                      setModelRoutingRules(event.target.value);
                      setSavedMessage('');
                    }}
                    value={modelRoutingRules}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611] md:col-span-3">
                  Skills JSON
                  <textarea
                    className="mt-2 min-h-28 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 font-mono text-xs leading-5 text-[#181611]"
                    onChange={(event) => {
                      setSkills(event.target.value);
                      setSavedMessage('');
                    }}
                    value={skills}
                  />
                </label>
              </div>
            </section>

            <section className="rounded-lg border border-[#d7d2c4] bg-white">
              <div className="border-b border-[#d7d2c4] px-5 py-4">
                <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Workspace run</p>
                <h2 className="text-base font-semibold text-[#181611]">Start run</h2>
              </div>
              <div className="grid gap-4 px-5 py-4 md:grid-cols-2">
                <label className="text-sm font-medium text-[#181611]">
                  Run conversation ID
                  <input
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setRunConversationId(event.target.value);
                      setRunLink('');
                    }}
                    type="text"
                    value={runConversationId}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Run mode
                  <select
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    onChange={(event) => {
                      setRunMode(normalizeExecutionMode(event.target.value));
                      setRunLink('');
                    }}
                    value={runMode}
                  >
                    {executionModes.map((mode) => (
                      <option key={mode} value={mode}>
                        {mode}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="text-sm font-medium text-[#181611] md:col-span-2">
                  Run goal
                  <textarea
                    className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 py-2 text-sm leading-6"
                    onChange={(event) => {
                      setRunGoal(event.target.value);
                      setRunLink('');
                    }}
                    value={runGoal}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Run max iterations
                  <input
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    min={1}
                    max={100}
                    onChange={(event) => {
                      setRunMaxIterations(event.target.value);
                      setRunLink('');
                    }}
                    step={1}
                    type="number"
                    value={runMaxIterations}
                  />
                </label>
                <label className="text-sm font-medium text-[#181611]">
                  Run token budget
                  <input
                    className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                    min={1000}
                    max={1000000}
                    onChange={(event) => {
                      setRunTokenBudget(event.target.value);
                      setRunLink('');
                    }}
                    step={1000}
                    type="number"
                    value={runTokenBudget}
                  />
                </label>
              </div>
              <div className="flex flex-wrap items-center gap-3 border-t border-[#d7d2c4] px-5 py-4">
                <button
                  className="inline-flex min-h-10 items-center gap-2 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={isStartingRun || !runConversationId.trim() || !runGoal.trim()}
                  onClick={() => void startRun()}
                  type="button"
                >
                  <RiPlayCircleLine className="size-4" aria-hidden="true" />
                  Start run
                </button>
                {runLink ? (
                  <Link className="text-sm font-semibold text-[#181611] underline" to={runLink}>
                    Open run plan steps
                  </Link>
                ) : null}
              </div>
            </section>

            <section className="rounded-lg border border-[#d7d2c4] bg-white">
              <div className="flex flex-col gap-3 border-b border-[#d7d2c4] px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                <h2 className="text-base font-semibold text-[#181611]">Tool approval policy</h2>
                <button
                  className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                  onClick={() => {
                    setIsCustomToolOpen(true);
                    setError(null);
                    setSavedMessage('');
                  }}
                  type="button"
                >
                  <RiAddLine className="size-4" aria-hidden="true" />
                  Add custom API tool
                </button>
              </div>
              {isCustomToolOpen ? (
                <div className="border-b border-[#e8e2d3] bg-[#fbfaf7] px-5 py-4">
                  <div className="grid gap-4 md:grid-cols-2">
                    <label className="text-sm font-medium text-[#181611]">
                      Tool runtime
                      <select
                        className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm"
                        onChange={(event) => {
                          setCustomToolRuntime(normalizeCustomToolRuntime(event.target.value));
                          setSavedMessage('');
                        }}
                        value={customToolRuntime}
                      >
                        {customToolRuntimes.map((runtime) => (
                          <option key={runtime} value={runtime}>
                            {runtime}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="text-sm font-medium text-[#181611]">
                      Tool name
                      <input
                        className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm"
                        onChange={(event) => {
                          setCustomToolName(event.target.value);
                          setSavedMessage('');
                        }}
                        type="text"
                        value={customToolName}
                      />
                    </label>
                    {customToolRuntime === 'api' ? (
                      <label className="text-sm font-medium text-[#181611] md:col-span-2">
                        Endpoint URL
                        <input
                          className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm"
                          onChange={(event) => {
                            setCustomToolEndpoint(event.target.value);
                            setSavedMessage('');
                          }}
                          type="url"
                          value={customToolEndpoint}
                        />
                      </label>
                    ) : null}
                    <label className="text-sm font-medium text-[#181611]">
                      Tool risk level
                      <select
                        className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm"
                        onChange={(event) => {
                          setCustomToolRiskLevel(normalizeRiskLevel(event.target.value));
                          setSavedMessage('');
                        }}
                        value={customToolRiskLevel}
                      >
                        {riskLevels.map((level) => (
                          <option key={level} value={level}>
                            {level}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="flex min-h-10 items-end gap-3 text-sm font-medium text-[#181611]">
                      <input
                        aria-label="Require approval for custom API tool"
                        checked={customToolRequiresApproval}
                        onChange={(event) => {
                          setCustomToolRequiresApproval(event.target.checked);
                          setSavedMessage('');
                        }}
                        type="checkbox"
                      />
                      Require approval
                    </label>
                    <label className="text-sm font-medium text-[#181611] md:col-span-2">
                      Tool description
                      <input
                        className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm"
                        onChange={(event) => {
                          setCustomToolDescription(event.target.value);
                          setSavedMessage('');
                        }}
                        type="text"
                        value={customToolDescription}
                      />
                    </label>
                    <label className="text-sm font-medium text-[#181611] md:col-span-2">
                      Input schema JSON
                      <textarea
                        className="mt-2 min-h-28 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-xs leading-5 text-[#181611]"
                        onChange={(event) => {
                          setCustomToolInputSchema(event.target.value);
                          setSavedMessage('');
                        }}
                        value={customToolInputSchema}
                      />
                    </label>
                    {customToolRuntime === 'python' ? (
                      <>
                        <label className="text-sm font-medium text-[#181611] md:col-span-2">
                          Python source code
                          <textarea
                            className="mt-2 min-h-36 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 font-mono text-xs leading-5 text-[#181611]"
                            onChange={(event) => {
                              setCustomToolSourceCode(event.target.value);
                              setSavedMessage('');
                            }}
                            value={customToolSourceCode}
                          />
                        </label>
                        <label className="text-sm font-medium text-[#181611]">
                          Timeout seconds
                          <input
                            className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 text-sm"
                            min={1}
                            max={30}
                            onChange={(event) => {
                              setCustomToolTimeoutSeconds(event.target.value);
                              setSavedMessage('');
                            }}
                            step={1}
                            type="number"
                            value={customToolTimeoutSeconds}
                          />
                        </label>
                      </>
                    ) : null}
                  </div>
                  <div className="mt-4 flex flex-wrap items-center gap-3">
                    <button
                      className="inline-flex min-h-10 items-center gap-2 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                      disabled={
                        isSavingCustomTool ||
                        !customToolName.trim() ||
                        (customToolRuntime === 'api' && !customToolEndpoint.trim()) ||
                        (customToolRuntime === 'python' && !customToolSourceCode.trim())
                      }
                      onClick={() => void saveCustomAPITool()}
                      type="button"
                    >
                      <RiSave3Line className="size-4" aria-hidden="true" />
                      Save custom API tool
                    </button>
                    <button
                      className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-[#d7d2c4] px-4 text-sm font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                      disabled={isSavingCustomTool}
                      onClick={() => {
                        resetCustomToolForm();
                        setIsCustomToolOpen(false);
                      }}
                      type="button"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : null}
              {selectedAgent.tools?.length ? (
                <div className="divide-y divide-[#e8e2d3]">
                  {selectedAgent.tools.map((tool) => {
                    const key = toolKey(tool);
                    const override = overrides[key] ?? {};
                    const riskLevel = normalizeRiskLevel(override.riskLevel ?? tool.riskLevel);
                    return (
                      <div
                        aria-label={`Tool policy ${tool.name}`}
                        className="grid gap-4 px-5 py-4 md:grid-cols-[minmax(0,1fr)_150px_180px]"
                        key={key}
                      >
                        <div className="min-w-0">
                          <p className="font-medium text-[#181611]">{tool.name}</p>
                          <p className="mt-1 text-xs text-[#625b4f]">{toolDisplayType(tool)}</p>
                          {tool.description ? <p className="mt-2 text-sm leading-6 text-[#625b4f]">{tool.description}</p> : null}
                        </div>
                        <label className="text-sm font-medium text-[#181611]">
                          {`Risk level for ${tool.name}`}
                          <select
                            aria-label={`Risk level for ${tool.name}`}
                            className="mt-2 min-h-10 w-full rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] px-3 text-sm"
                            onChange={(event) => setToolRiskLevel(tool, normalizeRiskLevel(event.target.value))}
                            value={riskLevel}
                          >
                            {riskLevels.map((level) => (
                              <option key={level} value={level}>
                                {level}
                              </option>
                            ))}
                          </select>
                        </label>
                        <label className="flex min-h-10 items-center gap-3 text-sm font-medium text-[#181611]">
                          <input
                            aria-label={`Require approval for ${tool.name}`}
                            checked={override.requiresApproval ?? tool.requiresApproval ?? false}
                            onChange={(event) => setToolApprovalRequired(tool, event.target.checked)}
                            type="checkbox"
                          />
                          Require approval
                        </label>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="px-5 py-4 text-sm text-[#625b4f]">No tools enabled for this agent.</p>
              )}
            </section>

            <section className="rounded-lg border border-[#d7d2c4] bg-white">
              <div className="flex flex-col gap-3 border-b border-[#d7d2c4] px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Tool registry</p>
                  <h2 className="text-base font-semibold text-[#181611]">Available tool catalog</h2>
                </div>
                <button
                  className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-[#181611] px-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={isLoadingTools}
                  onClick={() => void loadToolCatalog()}
                  type="button"
                >
                  <RiTerminalBoxLine className="size-4" aria-hidden="true" />
                  Load tool catalog
                </button>
              </div>
              {toolCatalog.length === 0 ? (
                <p className="px-5 py-4 text-sm text-[#625b4f]">Tool definitions load on demand for the selected agent.</p>
              ) : (
                <div className="divide-y divide-[#e8e2d3]">
                  {toolCatalog.map((tool) => {
                    const inputSchema = readableJSON(tool.inputSchema);
                    const isEnabled = (selectedAgent?.tools ?? []).some((agentTool) => agentTool.name === tool.name);
                    const isEnabling = enablingCatalogToolName === tool.name;
                    const isCatalogToolSaving = enablingCatalogToolName !== '';
                    return (
                      <article className="grid gap-3 px-5 py-4 lg:grid-cols-[minmax(0,1fr)_minmax(260px,420px)]" key={tool.name}>
                        <div className="min-w-0">
                          <h3 className="text-sm font-semibold text-[#181611]">{tool.name}</h3>
                          <p className="mt-1 text-xs text-[#625b4f]">
                            {tool.toolType ?? 'builtin'} / {normalizeRiskLevel(tool.riskLevel)}
                            {tool.requiresApproval ? ' / approval required' : ' / no approval required'}
                          </p>
                          {tool.description ? <p className="mt-2 text-sm leading-6 text-[#625b4f]">{tool.description}</p> : null}
                          <button
                            className="mt-3 inline-flex min-h-9 items-center gap-2 rounded-lg border border-[#181611] px-3 text-sm font-semibold text-[#181611] disabled:cursor-not-allowed disabled:opacity-50"
                            disabled={isEnabled || isCatalogToolSaving}
                            onClick={() => void enableCatalogTool(tool)}
                            type="button"
                          >
                            {isEnabled
                              ? `Tool ${tool.name} enabled`
                              : isEnabling
                                ? `Enabling tool ${tool.name}`
                                : `Enable tool ${tool.name}`}
                          </button>
                        </div>
                        {inputSchema ? (
                          <pre className="max-h-52 overflow-auto rounded-lg bg-[#f6f1e6] p-3 text-xs leading-5 text-[#3f3a31]">
                            {inputSchema}
                          </pre>
                        ) : (
                          <p className="text-sm text-[#625b4f]">No input schema published.</p>
                        )}
                      </article>
                    );
                  })}
                </div>
              )}
            </section>

            <div className="flex flex-wrap items-center gap-3">
              <button
                className="inline-flex min-h-10 items-center gap-2 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                disabled={isSaving}
                onClick={() => void saveAgentPolicy()}
                type="button"
              >
                <RiSave3Line className="size-4" aria-hidden="true" />
                Save agent policy
              </button>
              {savedMessage ? <p className="text-sm font-medium text-[#2f5f3a]">{savedMessage}</p> : null}
            </div>
          </section>
        ) : null}
      </div>
    </section>
  );
}
