import type { Page, Route } from '@playwright/test';

const now = '2026-06-14T10:00:00Z';
const runId = 'run_browser_agent';
const budgetRunId = 'run_browser_agent_budget';
const configAgentId = 'agent_browser_config';
const expectedAdjustReason = 'Browser scope changed after operator review.';
const expectedContinueBudget = 45000;

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_agent_planning',
    expiresAt: '2026-06-15T10:00:00Z',
  },
  user: {
    id: 'user_agent_operator',
    email: 'agent-operator@example.com',
    name: 'Agent Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_agent_planning',
  },
};

const agent = {
  id: 'agent_browser_planner',
  userId: session.user.id,
  name: 'Browser Planning Agent',
  description: 'Exercises planning-mode approval and plan-step controls through the browser router.',
  model: 'gpt-4o-mini',
  systemPrompt: 'Plan browser route proof with explicit approval boundaries.',
  tools: [
    {
      description: 'Search the web with tenant policy controls.',
      enabled: true,
      name: 'web_search',
      requiresApproval: true,
      riskLevel: 'medium',
      serverId: 'custom-api-browser-search',
      type: 'custom',
    },
  ],
  config: {
    approvalMode: 'tiered',
    defaultExecutionMode: 'planning',
    longTermMemoryExtractionPolicy: 'deterministic',
    longTermMemoryUpdatePolicy: 'exact_refresh',
    longTermMemoryWritePolicy: 'interaction_and_explicit',
    maxIterations: 8,
    tokenBudget: 30000,
  },
  isPublic: false,
  createdAt: now,
  updatedAt: now,
};

const expectedCreateAgentPayload = {
  config: {
    approvalMode: 'all',
    defaultExecutionMode: 'planning',
    longTermMemoryExtractionPolicy: 'llm_assisted',
    longTermMemoryUpdatePolicy: 'memory_key_consolidate',
    longTermMemoryWritePolicy: 'explicit_only',
    maxIterations: 30,
    maxSkills: 1,
    modelRoutingRules: [{ minIteration: 2, requiresToolResult: true, targetModel: 'gpt-4o' }],
    skills: [{ instructions: 'Check weather sources.', name: 'Weather', toolNames: ['web_search'], triggers: ['weather'] }],
    tokenBudget: 60000,
  },
  description: 'Exercises the browser create and update agent config flow.',
  isPublic: false,
  model: 'gpt-4o-mini',
  name: 'Browser Config Agent',
  systemPrompt: 'Prefer explicit browser evidence.',
  tools: [],
};

const createdConfigAgent = {
  config: expectedCreateAgentPayload.config,
  createdAt: now,
  description: expectedCreateAgentPayload.description,
  id: configAgentId,
  isPublic: false,
  model: expectedCreateAgentPayload.model,
  name: expectedCreateAgentPayload.name,
  systemPrompt: expectedCreateAgentPayload.systemPrompt,
  tools: [],
  updatedAt: now,
  userId: session.user.id,
};

const expectedUpdateAgentPayload = {
  config: {
    approvalMode: 'custom',
    defaultExecutionMode: 'react',
    longTermMemoryExtractionPolicy: 'deterministic',
    longTermMemoryUpdatePolicy: 'exact_refresh',
    longTermMemoryWritePolicy: 'manual_only',
    maxIterations: 40,
    maxSkills: 3,
    modelRoutingRules: [{ minInputChars: 2000, targetModel: 'gpt-4.1' }],
    skills: [{ instructions: 'Summarize with citations.', name: 'Summarizer', toolNames: ['web_search'] }],
    tokenBudget: 75000,
    toolApprovalOverrides: {},
  },
  description: expectedCreateAgentPayload.description,
  model: expectedCreateAgentPayload.model,
  name: expectedCreateAgentPayload.name,
  systemPrompt: expectedCreateAgentPayload.systemPrompt,
  tools: [],
};

const updatedConfigAgent = {
  ...createdConfigAgent,
  config: expectedUpdateAgentPayload.config,
};

const toolCatalog = [
  {
    name: 'web_search',
    description: 'Search the web with tenant policy controls.',
    inputSchema: {
      type: 'object',
      properties: {
        query: { type: 'string' },
      },
      required: ['query'],
    },
    serverId: 'custom-api-browser-search',
    toolType: 'custom',
    requiresApproval: true,
    riskLevel: 'medium',
  },
];

type PlanVariant = 'initial' | 'adjusted';
type PlanStepStatus = 'pending' | 'approved' | 'completed';

function planSteps(stepStatus: PlanStepStatus, resultContent = '', variant: PlanVariant = 'initial') {
  if (variant === 'adjusted') {
    return [
      {
        id: 'step_scope',
        runId,
        index: 1,
        title: 'Inspect browser route scope',
        description: 'Confirm the workspace shell keeps Agent navigation active before editing plan steps.',
        status: 'completed',
        approvalStatus: 'not_required',
        resultContent: 'Workspace Agent route scope inspected.',
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'step_adjusted',
        runId,
        index: 2,
        title: 'Run adjusted browser checks',
        description: 'Run the adjusted browser checks after the route scope changed.',
        status: stepStatus,
        approvalStatus: 'not_required',
        dependsOn: [1],
        ...(resultContent ? { resultContent } : {}),
        createdAt: now,
        updatedAt: now,
      },
    ];
  }

  return [
    {
      id: 'step_scope',
      runId,
      index: 1,
      title: 'Inspect browser route scope',
      description: 'Confirm the workspace shell keeps Agent navigation active before editing plan steps.',
      status: 'completed',
      approvalStatus: 'not_required',
      resultContent: 'Workspace Agent route scope inspected.',
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'step_patch',
      runId,
      index: 2,
      title: 'Patch browser route proof',
      description: 'Patch the browser route proof after the release scope is known.',
      status: stepStatus,
      approvalStatus: stepStatus === 'pending' ? 'pending' : 'approved',
      toolName: 'web_search',
      input: {
        query: 'agent planning browser route proof',
      },
      dependsOn: [1],
      ...(resultContent ? { resultContent } : {}),
      createdAt: now,
      updatedAt: now,
    },
  ];
}

function toolRuns(status: 'pending_approval' | 'completed') {
  return [
    {
      id: 'tool_search',
      runId,
      toolCallId: 'call_search',
      toolName: 'web_search',
      toolType: 'custom',
      serverId: 'custom-api-browser-search',
      riskLevel: 'medium',
      arguments: {
        query: 'agent planning browser route proof',
      },
      status,
      approvalStatus: status === 'pending_approval' ? 'pending' : 'approved',
      attemptCount: 1,
      ...(status === 'completed'
        ? {
            approvalDecisionReason: 'Browser route operator approval.',
            resultContent: 'Search approved.',
          }
        : {}),
      createdAt: now,
      updatedAt: now,
    },
  ];
}

function runDetail(options: {
  iterationCount?: number;
  planVariant?: PlanVariant;
  status?: string;
  stepStatus?: PlanStepStatus;
  stepResult?: string;
  toolCallCount?: number;
  toolStatus?: 'pending_approval' | 'completed';
  toolRunsVisible?: boolean;
} = {}) {
  const stepStatus = options.stepStatus ?? 'pending';
  const toolStatus = options.toolStatus ?? 'pending_approval';

  return {
    id: runId,
    iterationCount: options.iterationCount ?? 2,
    mode: 'planning',
    status: options.status ?? 'pending_approval',
    toolCallCount: options.toolCallCount ?? 1,
    planSteps: planSteps(stepStatus, options.stepResult, options.planVariant),
    toolRuns: options.toolRunsVisible === false ? [] : toolRuns(toolStatus),
  };
}

function budgetRunDetail(status: 'token_budget_exceeded' | 'completed' = 'token_budget_exceeded') {
  const exceeded = status === 'token_budget_exceeded';
  const stopReason = 'token_budget_exceeded: used 32500 tokens exceeds budget 30000';

  return {
    id: budgetRunId,
    error: exceeded ? stopReason : undefined,
    iterationCount: exceeded ? 2 : 3,
    mode: 'planning',
    status,
    toolCallCount: 1,
    planSteps: [
      {
        id: 'step_budget_scope',
        runId: budgetRunId,
        index: 1,
        title: 'Inspect token budget stop',
        description: 'Capture the browser-visible budget stop before retrying the failed plan step.',
        status: 'completed',
        approvalStatus: 'not_required',
        resultContent: 'Token budget stop captured.',
        createdAt: now,
        updatedAt: now,
      },
      {
        id: 'step_budget_retry',
        runId: budgetRunId,
        index: 2,
        title: 'Retry after increased budget',
        description: 'Retry the stopped plan step after the operator increases the token budget.',
        status: exceeded ? 'failed' : 'completed',
        approvalStatus: 'not_required',
        toolName: 'web_search',
        input: {
          query: 'agent planning token budget browser recovery',
        },
        dependsOn: [1],
        ...(exceeded
          ? { error: stopReason }
          : { resultContent: 'Token-budget recovery completed in the browser.' }),
        createdAt: now,
        updatedAt: now,
      },
    ],
    toolRuns: [],
  };
}

function envelope(data: unknown) {
  return {
    ok: true,
    data,
    error: null,
  };
}

async function fulfillJSON(route: Route, data: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(envelope(data)),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'agent planning fixture route not found' },
    }),
  });
}

async function fulfillInvalidRequest(route: Route, message: string) {
  await route.fulfill({
    status: 400,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'invalid_request', message },
    }),
  });
}

function sortedJSON(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortedJSON);
  }

  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, nestedValue]) => [key, sortedJSON(nestedValue)])
    );
  }

  return value;
}

function payloadMatches(payload: unknown, expected: unknown) {
  return JSON.stringify(sortedJSON(payload)) === JSON.stringify(sortedJSON(expected));
}

export async function registerAgentPlanningRoutes(page: Page): Promise<void> {
  let currentAgents = [agent];
  let currentRunDetail = runDetail();
  let currentPlanVariant: PlanVariant = 'initial';
  let currentBudgetRunDetail = budgetRunDetail();

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/app/agents') {
      await fulfillJSON(route, currentAgents);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/app/agents') {
      const payload = request.postDataJSON();
      if (!payloadMatches(payload, expectedCreateAgentPayload)) {
        await fulfillInvalidRequest(route, 'unexpected create agent payload');
        return;
      }

      currentAgents = [createdConfigAgent, ...currentAgents.filter((currentAgent) => currentAgent.id !== configAgentId)];
      await fulfillJSON(route, createdConfigAgent, 201);
      return;
    }

    if (method === 'PUT' && pathname === `/api/v1/app/agents/${configAgentId}`) {
      const payload = request.postDataJSON();
      if (!payloadMatches(payload, expectedUpdateAgentPayload)) {
        await fulfillInvalidRequest(route, 'unexpected update agent payload');
        return;
      }

      currentAgents = currentAgents.map((currentAgent) => (
        currentAgent.id === configAgentId ? updatedConfigAgent : currentAgent
      ));
      await fulfillJSON(route, updatedConfigAgent);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/app/agents/${agent.id}/tools`) {
      await fulfillJSON(route, toolCatalog);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/agent/runs') {
      currentPlanVariant = 'initial';
      currentRunDetail = runDetail();
      await fulfillJSON(route, {
        id: runId,
        status: currentRunDetail.status,
        planSteps: currentRunDetail.planSteps,
        toolRuns: currentRunDetail.toolRuns,
      }, 201);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/agent/runs/${runId}/adjust-plan`) {
      const payload = request.postDataJSON() as { reason?: string };
      if (payload.reason !== expectedAdjustReason) {
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            ok: false,
            data: null,
            error: { code: 'invalid_request', message: 'unexpected adjust-plan reason' },
          }),
        });
        return;
      }

      currentPlanVariant = 'adjusted';
      currentRunDetail = runDetail({
        iterationCount: 3,
        planVariant: currentPlanVariant,
        toolRunsVisible: false,
      });
      await fulfillJSON(route, currentRunDetail);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/agent/runs/${runId}`) {
      await fulfillJSON(route, currentRunDetail);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/agent/runs/${budgetRunId}`) {
      await fulfillJSON(route, currentBudgetRunDetail);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/agent/runs/${runId}/approve-tool`) {
      currentRunDetail = runDetail({ toolStatus: 'completed' });
      await fulfillJSON(route, currentRunDetail);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/agent/runs/${runId}/approve-plan-step`) {
      currentRunDetail = runDetail({ stepStatus: 'approved', toolStatus: 'completed' });
      await fulfillJSON(route, currentRunDetail);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/agent/runs/${runId}/execute-plan-step`) {
      currentRunDetail = runDetail({
        stepStatus: 'completed',
        stepResult: 'Browser route proof patched.',
        toolStatus: 'completed',
      });
      await fulfillJSON(route, currentRunDetail);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/agent/runs/${runId}/continue-plan`) {
      if (currentPlanVariant === 'adjusted') {
        currentRunDetail = runDetail({
          iterationCount: 4,
          planVariant: currentPlanVariant,
          status: 'completed',
          stepStatus: 'completed',
          stepResult: 'Adjusted browser checks completed.',
          toolRunsVisible: false,
        });
      } else {
        currentRunDetail = runDetail({
          iterationCount: 5,
          status: 'completed',
          stepStatus: 'completed',
          stepResult: 'Browser route proof patched.',
          toolStatus: 'completed',
        });
      }
      await fulfillJSON(route, currentRunDetail);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/agent/runs/${budgetRunId}/continue-budget`) {
      const payload = request.postDataJSON() as { tokenBudget?: number };
      if (payload.tokenBudget !== expectedContinueBudget) {
        await route.fulfill({
          status: 400,
          contentType: 'application/json',
          body: JSON.stringify({
            ok: false,
            data: null,
            error: { code: 'invalid_request', message: 'unexpected continue-budget payload' },
          }),
        });
        return;
      }

      currentBudgetRunDetail = budgetRunDetail('completed');
      await fulfillJSON(route, currentBudgetRunDetail);
      return;
    }

    await fulfillNotFound(route);
  });
}
