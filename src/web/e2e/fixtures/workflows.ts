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

export async function registerWorkflowRoutes(page: Page): Promise<void> {
  let executions = [priorExecution];

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
      await fulfillJSON(route, [workflow]);
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
