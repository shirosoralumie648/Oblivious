import type { Page, Route } from '@playwright/test';

const now = '2026-06-13T09:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_workflows',
    expiresAt: '2026-06-14T09:00:00Z',
  },
  user: {
    id: 'user_workflows',
    email: 'workflow-operator@example.com',
    name: 'Workflow Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_workflows',
  },
};

const workflow = {
  id: 'workflow_release',
  organizationId: 'org_workflows',
  name: 'Release automation',
  description: 'Routes release incidents through trigger matching, webhook intake, and debug snapshots.',
  status: 'published',
  version: 3,
  variables: {
    releaseChannel: 'stable',
    severity: 'sev1',
  },
  definition: {
    max_concurrent_executions: 3,
    max_execution_duration_seconds: 120,
    max_node_executions: 8,
    triggers: {
      conversation: {
        id: 'conversation-release',
        conversationId: 'conversation_release',
      },
      schedule: {
        id: 'daily-release',
        cron: '0 9 * * 1',
        enabled: true,
      },
      semantic: {
        id: 'incident-response',
        keywords: ['release incident', 'rollback'],
        threshold: 0.86,
      },
      webhook: {
        id: 'github-release',
        path: '/api/v1/workflows/webhooks/org_workflows/workflow_release',
        secret: '********',
      },
    },
    nodes: [
      {
        id: 'manual-start',
        type: 'manual',
        position: { x: 80, y: 80 },
        input: { title: 'Collect release incident context' },
      },
      {
        id: 'classify',
        type: 'condition',
        position: { x: 320, y: 80 },
        input: { expression: 'input.severity == "sev1"' },
        failure_strategy: 'pause_on_failure',
        failure_policy: { max_retries: 2, retry_delays: ['1s', '5s'] },
      },
      {
        id: 'notify',
        type: 'notification',
        position: { x: 560, y: 80 },
        input: { channel: 'ops', template: 'Release incident routed' },
      },
    ],
    edges: [
      { id: 'edge-start-classify', source: 'manual-start', target: 'classify' },
      { id: 'edge-classify-notify', source: 'classify', target: 'notify', branch: 'sev1' },
    ],
  },
  createdAt: now,
  updatedAt: now,
};

const workflowVersionOne = {
  ...workflow,
  status: 'draft',
  version: 1,
  definition: {
    ...workflow.definition,
    nodes: workflow.definition.nodes.slice(0, 2),
    edges: workflow.definition.edges.slice(0, 1),
  },
  updatedAt: '2026-06-12T08:00:00Z',
};

const workflowVersionTwo = {
  ...workflow,
  version: 2,
  definition: {
    ...workflow.definition,
    nodes: workflow.definition.nodes.slice(0, 2),
    edges: workflow.definition.edges.slice(0, 1),
  },
  updatedAt: '2026-06-12T18:00:00Z',
};

const rolledBackWorkflow = {
  ...workflowVersionOne,
  id: workflow.id,
  status: 'draft',
  version: 4,
  updatedAt: '2026-06-13T09:05:00Z',
};

const workflowBranch = {
  ...workflowVersionTwo,
  id: 'workflow_release_branch',
  name: 'Release automation branch',
  status: 'draft',
  version: 1,
  definition: {
    ...workflowVersionTwo.definition,
    branch: {
      sourceWorkflowId: workflow.id,
      sourceVersion: 2,
      experimentKey: 'release-routing-v2',
      trafficPercent: 25,
    },
  },
  updatedAt: '2026-06-13T09:10:00Z',
};

const publishedWorkflowBranch = {
  ...workflowBranch,
  status: 'published',
  updatedAt: '2026-06-13T09:11:00Z',
};

const mergedWorkflow = {
  ...workflow,
  version: 5,
  updatedAt: '2026-06-13T09:12:00Z',
};

const scheduledTask = {
  id: 'sched_release_daily',
  organizationId: 'org_workflows',
  name: 'Daily release automation',
  targetType: 'workflow',
  targetId: workflow.id,
  workflowTriggerId: 'daily-release',
  cronExpression: '0 9 * * 1',
  enabled: true,
  lastRunAt: null,
  nextRunAt: '2026-06-15T09:00:00Z',
  createdAt: now,
  updatedAt: now,
};

const executionRun = {
  id: 'exec_release_run',
  workflowId: workflow.id,
  organizationId: 'org_workflows',
  status: 'succeeded',
  input: { release: '2026.06', severity: 'sev1' },
  output: { routed: true, channel: 'ops' },
  nodeExecutions: [
    {
      nodeId: 'manual-start',
      nodeType: 'manual',
      status: 'succeeded',
      durationMs: 12,
      output: { accepted: true },
    },
    {
      nodeId: 'classify',
      nodeType: 'condition',
      status: 'succeeded',
      durationMs: 31,
      output: { branch: 'sev1' },
    },
    {
      nodeId: 'notify',
      nodeType: 'notification',
      status: 'succeeded',
      durationMs: 17,
      output: { delivered: true },
    },
  ],
  startedAt: now,
  completedAt: '2026-06-13T09:00:03Z',
  createdAt: now,
  updatedAt: '2026-06-13T09:00:03Z',
};

const executionWebhook = {
  ...executionRun,
  id: 'exec_release_webhook',
  input: { event: 'release.published', release: '2026.06' },
  output: { webhookAccepted: true, routed: true },
  startedAt: '2026-06-13T09:01:00Z',
  completedAt: '2026-06-13T09:01:02Z',
  createdAt: '2026-06-13T09:01:00Z',
  updatedAt: '2026-06-13T09:01:02Z',
};

const priorExecution = {
  ...executionRun,
  id: 'exec_release_prior',
  input: { release: '2026.05', severity: 'sev2' },
  output: { routed: true, channel: 'release' },
  startedAt: '2026-06-12T09:00:00Z',
  completedAt: '2026-06-12T09:00:04Z',
  createdAt: '2026-06-12T09:00:00Z',
  updatedAt: '2026-06-12T09:00:04Z',
};

const pausedExecution = {
  ...executionRun,
  id: 'exec_release_paused',
  status: 'paused',
  input: { release: '2026.06', severity: 'sev1' },
  output: undefined,
  nodeExecutions: [
    {
      nodeId: 'manual-start',
      nodeType: 'manual',
      status: 'succeeded',
      durationMs: 12,
      output: { accepted: true },
    },
    {
      nodeId: 'classify',
      nodeType: 'condition',
      status: 'failed',
      durationMs: 31,
      input: { severity: 'sev1' },
      error: { message: 'classification model unavailable' },
    },
  ],
  startedAt: '2026-06-13T08:58:00Z',
  completedAt: undefined,
  createdAt: '2026-06-13T08:58:00Z',
  updatedAt: '2026-06-13T08:58:03Z',
};

const resourceLimitedExecution = {
  ...pausedExecution,
  status: 'paused',
  error: { reason: 'workflow_resource_guard', totalTokens: 2048, nodeExecutionCount: 1001 },
  updatedAt: '2026-06-13T09:13:00Z',
};

const branchedPausedExecution = {
  ...pausedExecution,
  status: 'running',
  nodeExecutions: [
    ...pausedExecution.nodeExecutions,
    {
      nodeId: 'notify',
      nodeType: 'notification',
      status: 'pending',
      input: { severity: 'sev1', reviewed: true },
    },
  ],
  updatedAt: '2026-06-13T09:14:00Z',
};

const debugSnapshot = {
  executionId: executionRun.id,
  workflowId: workflow.id,
  status: 'succeeded',
  variableSnapshot: {
    input: executionRun.input,
    context: { releaseChannel: 'stable' },
    nodeOutputs: {
      'manual-start': { accepted: true },
      classify: { branch: 'sev1' },
      notify: { delivered: true },
    },
  },
  trace: [
    {
      nodeId: 'manual-start',
      nodeType: 'manual',
      status: 'succeeded',
      durationMs: 12,
      output: { accepted: true },
    },
    {
      nodeId: 'classify',
      nodeType: 'condition',
      status: 'succeeded',
      durationMs: 31,
      output: { branch: 'sev1' },
    },
    {
      nodeId: 'notify',
      nodeType: 'notification',
      status: 'succeeded',
      durationMs: 17,
      output: { delivered: true },
    },
  ],
  outputs: {
    'manual-start': { accepted: true },
    classify: { branch: 'sev1' },
    notify: { delivered: true },
  },
  performance: {
    totalDurationMs: 60,
    nodeDurationsMs: {
      'manual-start': 12,
      classify: 31,
      notify: 17,
    },
    bottleneckNodeId: 'classify',
  },
  logs: [
    {
      level: 'info',
      message: 'Release incident routed to operations',
      nodeId: 'notify',
      timestamp: '2026-06-13T09:00:03Z',
    },
  ],
};

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

async function fulfillError(route: Route, message: string, status = 422) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'fixture_contract_mismatch', message },
    }),
  });
}

async function fulfillNotFound(route: Route) {
  await route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({
      ok: false,
      data: null,
      error: { code: 'not_found', message: 'workflow fixture route not found' },
    }),
  });
}

function objectMatches(actual: unknown, expected: Record<string, unknown>) {
  if (actual === null || typeof actual !== 'object' || Array.isArray(actual)) {
    return false;
  }
  const actualRecord = actual as Record<string, unknown>;
  return Object.entries(expected).every(([key, expectedValue]) => actualRecord[key] === expectedValue);
}

export async function registerWorkflowRoutes(page: Page): Promise<void> {
  let workflows = [workflow];
  let executions = [pausedExecution, priorExecution];
  let versions = [workflowVersionOne, workflowVersionTwo, workflow];
  let branchCreated = false;
  let branchPublished = false;
  let branchMerged = false;
  let rollbackCalled = false;
  let resourceChecked = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/workflows') {
      await fulfillJSON(route, workflows);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/scheduled-tasks') {
      await fulfillJSON(route, [scheduledTask]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/workflows/conversation-matches') {
      await fulfillJSON(route, [
        {
          workflowId: workflow.id,
          workflowVersion: workflow.version,
          workflowName: workflow.name,
          triggerId: 'conversation-release',
          conversationId: 'conversation_release',
          triggerDefinition: workflow.definition.triggers.conversation,
          workflowDefinition: workflow.definition,
        },
      ]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/workflows/semantic-matches') {
      await fulfillJSON(route, [
        {
          workflowId: workflow.id,
          workflowVersion: workflow.version,
          workflowName: workflow.name,
          triggerId: 'incident-response',
          keyword: 'rollback',
          semanticThreshold: 0.86,
          score: 0.94,
          matchMethod: 'hybrid_rerank',
          triggerDefinition: workflow.definition.triggers.semantic,
          workflowDefinition: workflow.definition,
        },
      ]);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/execute`) {
      executions = [executionRun, ...executions.filter((execution) => execution.id !== executionRun.id)];
      await fulfillJSON(route, executionRun);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/webhook`) {
      executions = [executionWebhook, ...executions.filter((execution) => execution.id !== executionWebhook.id)];
      await fulfillJSON(route, executionWebhook);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/workflows/${workflow.id}/executions`) {
      await fulfillJSON(route, executions);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/workflows/${workflow.id}/versions`) {
      await fulfillJSON(route, versions);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/rollback`) {
      const payload = request.postDataJSON();
      if (!objectMatches(payload, { version: 1 })) {
        await fulfillError(route, 'workflow rollback payload did not request version 1');
        return;
      }
      rollbackCalled = true;
      workflows = [rolledBackWorkflow, ...workflows.filter((currentWorkflow) => currentWorkflow.id !== workflow.id)];
      versions = [...versions, rolledBackWorkflow];
      await fulfillJSON(route, rolledBackWorkflow);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/branches`) {
      if (!rollbackCalled) {
        await fulfillError(route, 'browser tried to create a branch before proving rollback');
        return;
      }
      const payload = request.postDataJSON();
      if (
        !objectMatches(payload, {
          name: 'Release automation branch',
          description: 'Experiment branch',
          version: 2,
          experimentKey: 'release-routing-v2',
          trafficPercent: 25,
        })
      ) {
        await fulfillError(route, 'workflow branch payload did not match browser form selections');
        return;
      }
      branchCreated = true;
      branchPublished = false;
      workflows = [workflowBranch, ...workflows.filter((currentWorkflow) => currentWorkflow.id !== workflowBranch.id)];
      versions = [workflowVersionTwo, workflowBranch, workflow, rolledBackWorkflow];
      await fulfillJSON(route, workflowBranch, 201);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/branches/${workflowBranch.id}/publish`) {
      if (!branchCreated) {
        await fulfillError(route, 'browser tried to publish a branch before creating it');
        return;
      }
      const payload = request.postDataJSON();
      if (!objectMatches(payload, { name: workflowBranch.name })) {
        await fulfillError(route, 'workflow branch publish payload did not preserve branch name');
        return;
      }
      branchPublished = true;
      workflows = [publishedWorkflowBranch, ...workflows.filter((currentWorkflow) => currentWorkflow.id !== workflowBranch.id)];
      versions = [workflowVersionTwo, publishedWorkflowBranch, workflow, rolledBackWorkflow];
      await fulfillJSON(route, publishedWorkflowBranch);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/branches/${workflowBranch.id}/merge`) {
      if (!branchPublished) {
        await fulfillError(route, 'browser tried to merge a branch before publishing it');
        return;
      }
      branchMerged = true;
      workflows = [mergedWorkflow, ...workflows.filter((currentWorkflow) => currentWorkflow.id !== workflow.id)];
      versions = [workflowVersionTwo, publishedWorkflowBranch, mergedWorkflow, rolledBackWorkflow];
      await fulfillJSON(route, mergedWorkflow);
      return;
    }

    if (
      method === 'POST' &&
      pathname === `/api/v1/workflows/${workflow.id}/executions/${pausedExecution.id}/resource-check`
    ) {
      if (!branchMerged) {
        await fulfillError(route, 'browser checked resource limits before branch lifecycle proof');
        return;
      }
      const payload = request.postDataJSON();
      if (!objectMatches(payload, { totalTokens: 2048, nodeExecutionCount: 1001 })) {
        await fulfillError(route, 'workflow resource-check payload did not match browser form selections');
        return;
      }
      resourceChecked = true;
      executions = [
        resourceLimitedExecution,
        ...executions.filter((execution) => execution.id !== resourceLimitedExecution.id),
      ];
      await fulfillJSON(route, resourceLimitedExecution);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/executions/${pausedExecution.id}/decision`) {
      if (!resourceChecked) {
        await fulfillError(route, 'browser resolved paused failure before resource-check proof');
        return;
      }
      const payload = request.postDataJSON();
      const input = (payload as Record<string, unknown>).input as Record<string, unknown> | undefined;
      if (
        !objectMatches(payload, { action: 'retry', nodeId: 'classify' }) ||
        input?.severity !== 'sev1' ||
        input?.retryReason !== 'model-recovered'
      ) {
        await fulfillError(route, 'workflow paused-failure decision payload did not match edited input selections');
        return;
      }
      executions = [
        branchedPausedExecution,
        ...executions.filter((execution) => execution.id !== branchedPausedExecution.id),
      ];
      await fulfillJSON(route, branchedPausedExecution);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/workflows/${workflow.id}/executions/${executionRun.id}`) {
      await fulfillJSON(route, executionRun);
      return;
    }

    if (
      method === 'GET' &&
      pathname === `/api/v1/workflows/${workflow.id}/executions/${executionRun.id}/debug-snapshot`
    ) {
      await fulfillJSON(route, debugSnapshot);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/workflows/${workflow.id}/test-node`) {
      await fulfillJSON(route, {
        workflowId: workflow.id,
        nodeId: 'classify',
        status: 'succeeded',
        input: { severity: 'sev1' },
        output: { branch: 'sev1' },
        durationMs: 31,
        trace: [{ nodeId: 'classify', status: 'succeeded' }],
      });
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/scheduled-tasks/${scheduledTask.id}/run`) {
      await fulfillJSON(route, {
        id: 'sched_release_run',
        scheduledTaskId: scheduledTask.id,
        status: 'succeeded',
        startedAt: now,
        finishedAt: '2026-06-13T09:00:01Z',
        createdAt: now,
        updatedAt: '2026-06-13T09:00:01Z',
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
