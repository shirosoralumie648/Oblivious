import type { Page, Route } from '@playwright/test';

const now = '2026-06-15T09:00:00Z';

const session = {
  onboardingCompleted: true,
  preferences: {
    defaultMode: 'chat',
    modelStrategy: 'balanced',
    networkEnabledHint: true,
    onboardingCompleted: true,
  },
  session: {
    id: 'session_scheduled_tasks',
    expiresAt: '2026-06-16T09:00:00Z',
  },
  user: {
    id: 'user_schedule_operator',
    email: 'schedule-operator@example.com',
    name: 'Schedule Operator',
    role: 'admin',
  },
  workspace: {
    id: 'workspace_scheduled_tasks',
  },
};

const workflowTask = {
  id: 'schedule_release_digest',
  organizationId: 'org_scheduled_tasks',
  name: 'Release readiness digest',
  targetType: 'workflow',
  targetId: 'workflow_release_digest',
  workflowTriggerId: 'weekday-digest',
  cronExpression: '0 9 * * 1-5',
  enabled: true,
  lastRunAt: '2026-06-14T09:00:00Z',
  nextRunAt: '2026-06-15T09:00:00Z',
  createdAt: now,
  updatedAt: now,
};

const disabledAgentTask = {
  id: 'schedule_agent_summary',
  organizationId: 'org_scheduled_tasks',
  name: 'Agent summary pulse',
  targetType: 'agent',
  targetId: 'agent_summary',
  cronExpression: '*/30 * * * *',
  enabled: false,
  lastRunAt: null,
  nextRunAt: null,
  createdAt: now,
  updatedAt: now,
};

const createdTask = {
  id: 'schedule_agent_browser',
  organizationId: 'org_scheduled_tasks',
  name: 'Agent browser pulse',
  targetType: 'agent',
  targetId: 'agent_browser_operator',
  cronExpression: '*/20 * * * *',
  enabled: false,
  lastRunAt: null,
  nextRunAt: null,
  createdAt: '2026-06-15T09:05:00Z',
  updatedAt: '2026-06-15T09:05:00Z',
};

const enabledCreatedTask = {
  ...createdTask,
  enabled: true,
  nextRunAt: '2026-06-15T09:20:00Z',
  updatedAt: '2026-06-15T09:06:00Z',
};

const manualRun = {
  id: 'scheduled_run_browser_now',
  scheduledTaskId: createdTask.id,
  status: 'running',
  startedAt: '2026-06-15T09:07:00Z',
  finishedAt: null,
  error: null,
  createdAt: '2026-06-15T09:07:00Z',
  updatedAt: '2026-06-15T09:07:00Z',
};

const workflowRuns = [
  {
    id: 'scheduled_run_provider_research_cluster_mobile_without_breaks_20260624',
    scheduledTaskId: workflowTask.id,
    status: 'failed',
    startedAt: '2026-06-15T09:00:00Z',
    finishedAt: '2026-06-15T09:01:00Z',
    error:
      'providerresearchclusterscheduledtaskruntimeerrorwithoutbreaks20260624 rejected stale deployment evidence before workflow execution',
    createdAt: '2026-06-15T09:00:00Z',
    updatedAt: '2026-06-15T09:01:00Z',
  },
  {
    id: 'scheduled_run_digest_success',
    scheduledTaskId: workflowTask.id,
    status: 'completed',
    startedAt: '2026-06-14T09:00:00Z',
    finishedAt: '2026-06-14T09:01:00Z',
    error: null,
    createdAt: '2026-06-14T09:00:00Z',
    updatedAt: '2026-06-14T09:01:00Z',
  },
  {
    id: 'scheduled_run_digest_failed',
    scheduledTaskId: workflowTask.id,
    status: 'failed',
    startedAt: '2026-06-13T09:00:00Z',
    finishedAt: '2026-06-13T09:01:00Z',
    error: 'workflow guard rejected stale deployment input',
    createdAt: '2026-06-13T09:00:00Z',
    updatedAt: '2026-06-13T09:01:00Z',
  },
];

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
      error: { code: 'not_found', message: 'scheduled tasks fixture route not found' },
    }),
  });
}

function createPayloadMatches(payload: Record<string, unknown>) {
  return (
    payload.cronExpression === createdTask.cronExpression &&
    payload.enabled === createdTask.enabled &&
    payload.name === createdTask.name &&
    payload.targetId === createdTask.targetId &&
    payload.targetType === createdTask.targetType
  );
}

function statusPayloadMatches(payload: Record<string, unknown>) {
  return payload.enabled === true;
}

export async function registerScheduledTasksRoutes(page: Page): Promise<void> {
  let createdVisible = false;
  let enabledCreatedVisible = false;
  let manualRunVisible = false;

  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const { pathname } = url;
    const method = request.method();

    if (method === 'GET' && pathname === '/api/v1/auth/me') {
      await fulfillJSON(route, session);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/scheduled-tasks') {
      await fulfillJSON(route, [
        ...(createdVisible ? [enabledCreatedVisible ? enabledCreatedTask : createdTask] : []),
        workflowTask,
        disabledAgentTask,
      ]);
      return;
    }

    if (method === 'POST' && pathname === '/api/v1/scheduled-tasks') {
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!createPayloadMatches(payload)) {
        await fulfillError(route, 'scheduled task create payload did not match browser form selections');
        return;
      }
      createdVisible = true;
      await fulfillJSON(route, createdTask, 201);
      return;
    }

    if (method === 'PATCH' && pathname === `/api/v1/scheduled-tasks/${createdTask.id}/status`) {
      if (!createdVisible) {
        await fulfillError(route, 'browser tried to enable a scheduled task before creating it');
        return;
      }
      const payload = request.postDataJSON() as Record<string, unknown>;
      if (!statusPayloadMatches(payload)) {
        await fulfillError(route, 'scheduled task status payload did not enable the created task');
        return;
      }
      enabledCreatedVisible = true;
      await fulfillJSON(route, enabledCreatedTask);
      return;
    }

    if (method === 'POST' && pathname === `/api/v1/scheduled-tasks/${createdTask.id}/run`) {
      if (!enabledCreatedVisible) {
        await fulfillError(route, 'browser tried to run a scheduled task before enabling it');
        return;
      }
      manualRunVisible = true;
      await fulfillJSON(route, manualRun, 201);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/scheduled-tasks/${workflowTask.id}/runs`) {
      await fulfillJSON(route, workflowRuns);
      return;
    }

    if (method === 'GET' && pathname === `/api/v1/scheduled-tasks/${createdTask.id}/runs`) {
      await fulfillJSON(route, manualRunVisible ? [manualRun] : []);
      return;
    }

    if (method === 'GET' && pathname === '/api/v1/scheduled-tasks/state') {
      await fulfillJSON(route, {
        createdVisible,
        enabledCreatedVisible,
        manualRunVisible,
      });
      return;
    }

    await fulfillNotFound(route);
  });
}
