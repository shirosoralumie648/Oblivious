import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createTasksApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: overrides.delete
      ? ((path, init) => init === undefined ? overrides.delete!(path) : overrides.delete!(path, init)) as HttpClient['delete']
      : vi.fn(),
    get: overrides.get
      ? ((path, init) => init === undefined ? overrides.get!(path) : overrides.get!(path, init)) as HttpClient['get']
      : vi.fn(),
    post: overrides.post
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.post!(path) : overrides.post!(path, body)
          : overrides.post!(path, body, init)) as HttpClient['post']
      : vi.fn(),
    put: overrides.put
      ? ((path, body, init) => init === undefined
          ? body === undefined ? overrides.put!(path) : overrides.put!(path, body)
          : overrides.put!(path, body, init)) as HttpClient['put']
      : vi.fn(),
    request: overrides.request
      ? ((path, init) => init === undefined ? overrides.request!(path) : overrides.request!(path, init)) as HttpClient['request']
      : vi.fn(),
  };
  return client;
}

describe('createTasksApi', () => {
  it('binds task collection, lifecycle, and budget operations to their existing routes and payloads', async () => {
    const task = {
      authorizationScope: 'workspace_tools',
      budgetLimit: 25,
      executionMode: 'safe',
      goal: 'Review the workspace backlog',
      id: 'task_1',
      knowledgeBaseIds: [],
      status: 'draft',
      steps: [],
      title: 'Backlog review'
    };
    const get = vi
      .fn()
      .mockResolvedValueOnce([task])
      .mockResolvedValueOnce(task);
    const post = vi.fn().mockResolvedValue(task);
    const api = createTasksApi(createClient({ get, post }));
    const createPayload = {
      authorizationScope: 'workspace_tools',
      budgetLimit: 25,
      executionMode: 'safe',
      goal: 'Review the workspace backlog',
      knowledgeBaseIds: []
    };

    await expect(api.listTasks()).resolves.toEqual([task]);
    await expect(api.getTask('task_1')).resolves.toEqual(task);
    await expect(api.createTask(createPayload)).resolves.toEqual(task);
    await expect(api.approveTask('task_1')).resolves.toEqual(task);
    await expect(api.startTask('task_1')).resolves.toEqual(task);
    await expect(api.pauseTask('task_1')).resolves.toEqual(task);
    await expect(api.resumeTask('task_1')).resolves.toEqual(task);
    await expect(api.cancelTask('task_1')).resolves.toEqual(task);
    await expect(api.updateTaskBudget('task_1', { budgetLimit: 50 })).resolves.toEqual(task);

    expect(get).toHaveBeenNthCalledWith(1, '/api/v1/app/tasks');
    expect(get).toHaveBeenNthCalledWith(2, '/api/v1/app/tasks/task_1');
    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/app/tasks', createPayload);
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/app/tasks/task_1/approve');
    expect(post).toHaveBeenNthCalledWith(3, '/api/v1/app/tasks/task_1/start');
    expect(post).toHaveBeenNthCalledWith(4, '/api/v1/app/tasks/task_1/pause');
    expect(post).toHaveBeenNthCalledWith(5, '/api/v1/app/tasks/task_1/resume');
    expect(post).toHaveBeenNthCalledWith(6, '/api/v1/app/tasks/task_1/cancel');
    expect(post).toHaveBeenNthCalledWith(7, '/api/v1/app/tasks/task_1/budget', { budgetLimit: 50 });
  });
});
