import type { Page, Route } from '@playwright/test';

const now = '2026-06-14T10:00:00Z';
const runId = 'run_browser_agent';
const expectedAdjustReason = 'Browser scope changed after operator review.';

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
      type: 'builtin',
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
    toolType: 'builtin',
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
      toolType: 'builtin',
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

export async function registerAgentPlanningRoutes(page: Page): Promise<void> {
  let currentRunDetail = runDetail();
  let currentPlanVariant: PlanVariant = 'initial';

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
      await fulfillJSON(route, [agent]);
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

    await fulfillNotFound(route);
  });
}
