import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createScheduledTasksApi } from './scheduledTasksApi';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
  return client;
}

describe('createScheduledTasksApi', () => {
  it('lists scheduled tasks from the scheduled tasks endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
      cronExpression: '0 9 * * 1',
      enabled: true,
      id: 'schedule_1',
      name: 'Weekly workflow',
      targetId: 'workflow_1',
      targetType: 'workflow'
      }
    ]);
    const api = createScheduledTasksApi(createClient({ get }));

    await expect(api.listScheduledTasks()).resolves.toEqual([
      expect.objectContaining({ id: 'schedule_1', name: 'Weekly workflow', targetType: 'workflow' })
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/scheduled-tasks');
  });

  it('creates scheduled tasks on the scheduled tasks endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      cronExpression: '*/15 * * * *',
      enabled: false,
      id: 'schedule_2',
      name: 'Agent pulse',
      targetId: 'agent_1',
      targetType: 'agent'
    });
    const api = createScheduledTasksApi(createClient({ post }));
    const payload = {
      cronExpression: '*/15 * * * *',
      enabled: false,
      name: 'Agent pulse',
      targetId: 'agent_1',
      targetType: 'agent' as const
    };

    await expect(api.createScheduledTask(payload)).resolves.toEqual(
      expect.objectContaining({ id: 'schedule_2', name: 'Agent pulse', targetType: 'agent' })
    );

    expect(post).toHaveBeenCalledWith('/api/v1/scheduled-tasks', payload);
  });

  it('lists scheduled task runs from the task runs endpoint', async () => {
    const get = vi.fn().mockResolvedValue([
      {
        createdAt: '2026-06-04T01:00:00Z',
        finishedAt: '2026-06-04T01:02:00Z',
        id: 'run_1',
        scheduledTaskId: 'schedule_1',
        startedAt: '2026-06-04T01:00:00Z',
        status: 'completed',
        updatedAt: '2026-06-04T01:02:00Z'
      }
    ]);
    const api = createScheduledTasksApi(createClient({ get }));

    await expect(api.listRuns('schedule_1')).resolves.toEqual([
      expect.objectContaining({ id: 'run_1', scheduledTaskId: 'schedule_1', status: 'completed' })
    ]);

    expect(get).toHaveBeenCalledWith('/api/v1/scheduled-tasks/schedule_1/runs');
  });

  it('updates scheduled task enabled state on the status endpoint', async () => {
    const request = vi.fn().mockResolvedValue({
      cronExpression: '0 9 * * 1',
      enabled: false,
      id: 'schedule_1',
      targetId: 'workflow_1',
      targetType: 'workflow'
    });
    const api = createScheduledTasksApi(createClient({ request }));

    await expect(api.updateScheduledTaskEnabled('schedule_1', false)).resolves.toEqual(
      expect.objectContaining({ id: 'schedule_1', enabled: false })
    );

    expect(request).toHaveBeenCalledWith('/api/v1/scheduled-tasks/schedule_1/status', {
      body: JSON.stringify({ enabled: false }),
      method: 'PATCH'
    });
  });

  it('runs a scheduled task immediately from the run endpoint', async () => {
    const post = vi.fn().mockResolvedValue({
      id: 'schedrun_1',
      scheduledTaskId: 'schedule_1',
      status: 'running'
    });
    const api = createScheduledTasksApi(createClient({ post }));

    await expect(api.runScheduledTaskNow('schedule_1')).resolves.toEqual(
      expect.objectContaining({ id: 'schedrun_1', scheduledTaskId: 'schedule_1' })
    );

    expect(post).toHaveBeenCalledWith('/api/v1/scheduled-tasks/schedule_1/run');
  });
});
