import type { HttpClient } from '../../services/http/client';
import type { CreateTaskRequest, TaskDetail, TaskSummary } from '../../types/api';

export interface TasksApi {
  cancelTask: (taskId: string) => Promise<TaskDetail>;
  createTask: (payload: CreateTaskRequest) => Promise<TaskSummary>;
  getTask: (taskId: string) => Promise<TaskDetail>;
  listTasks: () => Promise<TaskSummary[]>;
  pauseTask: (taskId: string) => Promise<TaskDetail>;
  resumeTask: (taskId: string) => Promise<TaskDetail>;
  startTask: (taskId: string) => Promise<TaskDetail>;
}

export function createTasksApi(client: HttpClient): TasksApi {
  return {
    cancelTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/cancel`),
    createTask: (payload) => client.post<TaskSummary>('/api/v1/app/tasks', payload),
    getTask: (taskId) => client.get<TaskDetail>(`/api/v1/app/tasks/${taskId}`),
    listTasks: () => client.get<TaskSummary[]>('/api/v1/app/tasks'),
    pauseTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/pause`),
    resumeTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/resume`),
    startTask: (taskId) => client.post<TaskDetail>(`/api/v1/app/tasks/${taskId}/start`)
  };
}
