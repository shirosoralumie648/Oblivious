import type { HttpClient } from '../../services/http/client';
import type { ScheduledTaskRun } from '../../types/api';

export type ScheduledTaskTargetType = 'workflow' | 'agent';

export type ScheduledTask = {
  id: string;
  organizationId?: string;
  name: string;
  targetType: ScheduledTaskTargetType;
  targetId: string;
  workflowTriggerId?: string;
  cronExpression: string;
  enabled: boolean;
  lastRunAt?: string | null;
  nextRunAt?: string | null;
  createdAt?: string;
  updatedAt?: string;
};

export type CreateScheduledTaskRequest = {
  name: string;
  targetType: ScheduledTaskTargetType;
  targetId: string;
  cronExpression: string;
  enabled: boolean;
};

export type ScheduledTasksApi = {
  createScheduledTask: (payload: CreateScheduledTaskRequest) => Promise<ScheduledTask>;
  listRuns: (taskId: string) => Promise<ScheduledTaskRun[]>;
  listScheduledTasks: () => Promise<ScheduledTask[]>;
  runScheduledTaskNow: (taskId: string) => Promise<ScheduledTaskRun>;
  updateScheduledTaskEnabled: (taskId: string, enabled: boolean) => Promise<ScheduledTask>;
};

export function createScheduledTasksApi(client: HttpClient): ScheduledTasksApi {
  const path = '/api/v1/scheduled-tasks';

  return {
    createScheduledTask: (payload) => client.post<ScheduledTask>(path, payload),
    listRuns: (taskId) => client.get<ScheduledTaskRun[]>(`${path}/${encodeURIComponent(taskId)}/runs`),
    listScheduledTasks: () => client.get<ScheduledTask[]>(path),
    runScheduledTaskNow: (taskId) => client.post<ScheduledTaskRun>(`${path}/${encodeURIComponent(taskId)}/run`),
    updateScheduledTaskEnabled: (taskId, enabled) =>
      client.request<ScheduledTask>(`${path}/${encodeURIComponent(taskId)}/status`, {
        body: JSON.stringify({ enabled }),
        method: 'PATCH'
      })
  };
}
